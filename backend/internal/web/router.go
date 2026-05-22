package web

import (
	"context"
	"encoding/json"
	"net/http"

	"subforme/backend/internal/config"
	"subforme/backend/internal/xui"
)

type SubscriptionService interface {
	Generate(user string) ([]byte, error)
}

type AuthService interface {
	Check(username, password string) bool
}

type ConfigService interface {
	ReadAppConfig() (config.AppConfig, error)
	UpdateAppConfig(next config.AppConfig) error
	ReadGroupsConfig() (config.GroupConfig, error)
	UpdateGroupsConfig(next config.GroupConfig) error
	ReadBaseYAML(mode string) (string, error)
	UpdateBaseYAML(mode, raw string) error
	ReadTemplateSectionYAML(section string) (string, error)
	UpdateTemplateSectionYAML(section, raw string) error
	ReadManagedNodes() ([]config.ManagedNode, error)
	UpdateManagedNodes(next []config.ManagedNode) error
	ReadProviders() ([]config.ProviderAddon, error)
}

type XUIService interface {
	TestConnection(ctx context.Context) (xui.ConnectionStatus, error)
	DetectAvailableNodes() ([]xui.AvailableNode, error)
}

type UserService interface {
	SearchUsers(query string) ([]xui.UserSummary, error)
	PreviewUser(query string) ([]xui.Node, error)
}

type Dependencies struct {
	SubscriptionService SubscriptionService
	AuthService         AuthService
	SessionSecret       string
	FrontendDir         string
	ConfigService       ConfigService
	XUIService          XUIService
	UserService         UserService
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

	return mux
}
