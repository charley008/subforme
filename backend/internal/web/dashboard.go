package web

import (
	"encoding/json"
	"net/http"
)

type dashboardSummary struct {
	Mode        string `json:"mode"`
	Service     string `json:"service"`
	ServerCount int    `json:"server_count"`
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

		serverCount := 0
		if deps.DBService != nil {
			if servers, err := deps.DBService.DBListServers(); err == nil {
				serverCount = len(servers)
			}
		}

		uniqueUsers := 0
		if deps.DBService != nil {
			if users, err := deps.DBService.DBUserList(); err == nil {
				uniqueUsers = len(users)
			}
		} else {
			if users, err := deps.UserService.SearchUsers(""); err == nil {
				uniqueUsers = len(users)
			}
		}

		nodeCount := 0
		if deps.DBService != nil {
			if nodes, err := deps.DBService.DBListNodeDB(); err == nil {
				nodeCount = len(nodes)
			}
		} else {
			if nodes, err := deps.ConfigService.ReadManagedNodes(); err == nil {
				nodeCount = len(nodes)
			}
		}

		modeLabel := "默认直连"
		if appConfig.Mode == "blacklist" {
			modeLabel = "默认代理"
		}

		summary := dashboardSummary{
			Mode:        modeLabel,
			Service:     "ok",
			ServerCount: serverCount,
			UniqueUsers: uniqueUsers,
			NodeCount:   nodeCount,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(summary)
	}))
}
