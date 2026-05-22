package web

import (
	"encoding/json"
	"net/http"
)

type dashboardSummary struct {
	Mode        string `json:"mode"`
	Service     string `json:"service"`
	XUIStatus   string `json:"xui_status"`
	UniqueUsers int    `json:"unique_users"`
	NodeCount   int    `json:"node_count"`
}

func registerDashboardRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/dashboard/summary", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if deps.ConfigService == nil || deps.XUIService == nil || deps.UserService == nil {
			http.Error(w, "dashboard dependencies unavailable", http.StatusNotImplemented)
			return
		}

		appConfig, err := deps.ConfigService.ReadAppConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		xuiStatus, err := deps.XUIService.TestConnection(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		users, err := deps.UserService.SearchUsers("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		nodes, err := deps.ConfigService.ReadManagedNodes()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		modeLabel := "默认直连"
		if appConfig.Mode == "blacklist" {
			modeLabel = "默认代理"
		}

		summary := dashboardSummary{
			Mode:        modeLabel,
			Service:     "ok",
			XUIStatus:   map[bool]string{true: "connected", false: "failed"}[xuiStatus.OK],
			UniqueUsers: len(users),
			NodeCount:   len(nodes),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(summary)
	}))
}
