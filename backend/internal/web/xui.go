package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"subforme/backend/internal/config"
	"subforme/backend/internal/db"
)

type userModeRequest struct {
	User       string              `json:"user"`
	Mode       string              `json:"mode"`
	Nodes      []string            `json:"nodes,omitempty"`
	Providers  []string            `json:"providers,omitempty"`
	GroupNodes map[string][]string `json:"group_nodes,omitempty"`
}

type userSummaryResponse struct {
	Email             string              `json:"email"`
	Remark            string              `json:"remark"`
	Protocol          string              `json:"protocol"`
	NodeCount         int                 `json:"node_count"`
	Server            string              `json:"server"`
	Port              int                 `json:"port"`
	LastRemark        string              `json:"last_remark,omitempty"`
	Mode              string              `json:"mode"`
	ShareURL          string              `json:"share_url"`
	SelectedNodes     []string            `json:"selected_nodes,omitempty"`
	SelectedProviders []string            `json:"selected_providers,omitempty"`
	GroupNodes        map[string][]string `json:"group_nodes,omitempty"`
	UUID              string              `json:"uuid,omitempty"`
	Password          string              `json:"password,omitempty"`
	ServerTraffic     []db.ServerTraffic  `json:"server_traffic,omitempty"`
}

func registerPreviewRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/sub/preview", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		if user == "" {
			http.Error(w, "missing user query", http.StatusBadRequest)
			return
		}
		if deps.SubscriptionService == nil {
			http.Error(w, "subscription service unavailable", http.StatusNotImplemented)
			return
		}
		raw, err := generateForRequest(deps.SubscriptionService, r, user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(raw)
	}))
	mux.HandleFunc("/api/xui/test", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if deps.XUIService == nil {
			http.Error(w, "xui service unavailable", http.StatusNotImplemented)
			return
		}
		status, err := deps.XUIService.TestConnection(r.Context())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}))
	mux.HandleFunc("/api/users/search", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if deps.UserService == nil || deps.ConfigService == nil {
			http.Error(w, "user service unavailable", http.StatusNotImplemented)
			return
		}
		query := r.URL.Query().Get("q")

		// Try DB first if available (new system)
		if deps.DBService != nil {
			dbUsers, err := deps.DBService.DBUserSearch(query)
			if err == nil {
				ac, _ := deps.ConfigService.ReadAppConfig()
				out := make([]userSummaryResponse, 0, len(dbUsers))
				for _, u := range dbUsers {
					mode := resolveUserMode(ac, u.Email)
					protocols, nodeCount := deps.DBService.DBGetUserProtocols(u.ID)
					shareURL := shareURLForRequest(r, u.Email)
					traffic := deps.DBService.DBGetUserTraffic(u.ID)
					out = append(out, userSummaryResponse{
						Email:             u.Email,
						Protocol:          protocols,
						Remark:            u.Remark,
						Mode:              mode,
						UUID:              u.UUID,
						NodeCount:         nodeCount,
						ShareURL:          shareURL,
						SelectedNodes:     ac.UserNodes[u.Email],
						SelectedProviders: ac.UserProviders[u.Email],
						GroupNodes:        ac.UserGroupNodes[u.Email],
						ServerTraffic:     traffic,
					})
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(out)
				return
			}
		}

		// Fallback to old xui-based search
		users, err := deps.UserService.SearchUsers(query)
		if err != nil {
			// Return empty rather than 502
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]userSummaryResponse{})
			return
		}
		appConfig, err := deps.ConfigService.ReadAppConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		out := make([]userSummaryResponse, 0, len(users))
		for _, user := range users {
			mode := resolveUserMode(appConfig, user.Email)
			out = append(out, userSummaryResponse{
				Email:             user.Email,
				Remark:            user.Remark,
				Protocol:          user.Protocol,
				NodeCount:         user.NodeCount,
				Server:            user.Server,
				Port:              user.Port,
				LastRemark:        user.LastRemark,
				Mode:              mode,
				ShareURL:          shareURLForRequest(r, user.Email),
				SelectedNodes:     appConfig.UserNodes[user.Email],
				SelectedProviders: appConfig.UserProviders[user.Email],
				GroupNodes:        appConfig.UserGroupNodes[user.Email],
				UUID:              user.UUID,
				Password:          user.Password,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	mux.HandleFunc("/api/users/policy", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		var req userModeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if req.User == "" {
			http.Error(w, "missing user", http.StatusBadRequest)
			return
		}
		if req.Mode != "whitelist" && req.Mode != "blacklist" {
			http.Error(w, "invalid mode", http.StatusBadRequest)
			return
		}
		appConfig, err := deps.ConfigService.ReadAppConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if appConfig.UserModes == nil {
			appConfig.UserModes = map[string]string{}
		}
		if appConfig.UserNodes == nil {
			appConfig.UserNodes = map[string][]string{}
		}
		if appConfig.UserProviders == nil {
			appConfig.UserProviders = map[string][]string{}
		}
		appConfig.UserModes[req.User] = req.Mode
		appConfig.UserNodes[req.User] = req.Nodes
		appConfig.UserProviders[req.User] = req.Providers
		if req.GroupNodes != nil {
			if appConfig.UserGroupNodes == nil {
				appConfig.UserGroupNodes = map[string]map[string][]string{}
			}
			appConfig.UserGroupNodes[req.User] = req.GroupNodes
		}
		if err := deps.ConfigService.UpdateAppConfig(appConfig); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("/api/nodes/detect", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if deps.XUIService == nil {
			http.Error(w, "xui service unavailable", http.StatusNotImplemented)
			return
		}
		nodes, err := deps.XUIService.DetectAvailableNodes()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(nodes)
	}))
	mux.HandleFunc("/api/users/preview", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if deps.UserService == nil {
			http.Error(w, "user service unavailable", http.StatusNotImplemented)
			return
		}
		user := r.URL.Query().Get("user")
		if user == "" {
			http.Error(w, "missing user query", http.StatusBadRequest)
			return
		}
		nodes, err := deps.UserService.PreviewUser(user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(nodes)
	}))
}

func resolveUserMode(appConfig config.AppConfig, user string) string {
	if appConfig.UserModes != nil {
		if mode, ok := appConfig.UserModes[user]; ok && (mode == "whitelist" || mode == "blacklist") {
			return mode
		}
	}
	if appConfig.Mode == "blacklist" {
		return "blacklist"
	}
	return "whitelist"
}

func shareURLForRequest(r *http.Request, user string) string {
	return fmt.Sprintf("%s/api/sub?user=%s", publicBaseURLForRequest(r), user)
}

func publicBaseURLForRequest(r *http.Request) string {
	scheme := "http"
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = strings.Split(forwarded, ",")[0]
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	prefix := ""
	if forwarded := r.Header.Get("X-Forwarded-Prefix"); forwarded != "" {
		prefix = "/" + strings.Trim(strings.Split(forwarded, ",")[0], "/")
		if prefix == "/" {
			prefix = ""
		}
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, prefix)
}
