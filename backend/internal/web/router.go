package web

import (
	"context"
	"encoding/json"
	"net/http"

	"subforme/backend/internal/app"
	"subforme/backend/internal/config"
	"subforme/backend/internal/db"
	"subforme/backend/internal/xui"
)

type SubscriptionService interface {
	Generate(user string) ([]byte, error)
}

type PublicBaseSubscriptionService interface {
	GenerateWithBaseURL(user, publicBaseURL string) ([]byte, error)
}

type VariantSubscriptionService interface {
	GenerateWithBaseURLAndVariant(user, publicBaseURL, variant string) ([]byte, error)
}

type AuthService interface {
	Check(username, password string) bool
	UpdatePassword(newPassword string)
}

type ConfigService interface {
	ReadAppConfig() (config.AppConfig, error)
	UpdateAppConfig(next config.AppConfig) error
	ReadGroupsConfig() (config.GroupConfig, error)
	UpdateGroupsConfig(next config.GroupConfig) error
	ReadTemplateSectionYAML(section string) (string, error)
	UpdateTemplateSectionYAML(section, raw string) error
	ReadManagedNodes() ([]config.ManagedNode, error)
	UpdateManagedNodes(next []config.ManagedNode) error
	ReadProviders() ([]config.ProviderAddon, error)
	UpsertProvider(provider config.ProviderAddon, publicBaseURL string) (config.ProviderAddon, error)
	DeleteProvider(id string) error
	RefreshProvider(id string) (config.ProviderRefreshResult, error)
	ReadProviderFile(id string) ([]byte, error)
}

type XUIService interface {
	TestConnection(ctx context.Context) (xui.ConnectionStatus, error)
	DetectAvailableNodes() ([]xui.AvailableNode, error)
}

type UserService interface {
	SearchUsers(query string) ([]xui.UserSummary, error)
	PreviewUser(query string) ([]xui.Node, error)
}

type DBService interface {
	DBUserSearch(query string) ([]db.User, error)
	DBUserList() ([]db.User, error)
	DBCreateUser(u *db.User) error
	DBUpdateUser(u *db.User) error
	DBDeleteUser(id int64) error
	DBGetUserProtocols(userID int64) (string, int)
	DBGetUserTraffic(userID int64) []db.ServerTraffic
	RefreshTraffic(ctx context.Context) map[string][]db.ServerTraffic
	LoadTraffic(ctx context.Context) map[string][]db.ServerTraffic
	ResetServerUserTraffic(ctx context.Context, serverID int64) (*app.TrafficResetResult, error)
	ImportFromServer(ctx context.Context, serverID int64) (*app.ImportResult, error)
	SyncToServers(ctx context.Context) (*app.SyncResult, error)
	DBListServers() ([]db.Server, error)
	DBCreateServer(sv *db.Server) error
	DBUpdateServer(sv *db.Server) error
	DBDeleteServer(id int64) error
	DBListNodeDB() ([]db.Node2, error)
	DBReplaceNodes(nodes []db.Node2) error
}

type Dependencies struct {
	SubscriptionService SubscriptionService
	AuthService         AuthService
	SessionSecret       string
	FrontendDir         string
	ConfigService       ConfigService
	XUIService          XUIService
	UserService         UserService
	DBService           DBService
	AdminUsername       string
	RuntimePath         string
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": config.Version})
	})

	registerPublicRoutes(mux, deps)
	registerAuthRoutes(mux, deps)
	registerDashboardRoutes(mux, deps)
	registerConfigRoutes(mux, deps)
	registerPreviewRoutes(mux, deps)
	registerDBRoutes(mux, deps)

	return mux
}
