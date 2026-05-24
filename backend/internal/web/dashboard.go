package web

import (
	"encoding/json"
	"net/http"
	"sort"
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
	mux.HandleFunc("/api/dashboard/traffic", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		type barItem struct {
			ServerID   int64  `json:"server_id"`
			ServerName string `json:"server_name"`
			Total      int64  `json:"total"`
		}
		type userBars struct {
			Email string    `json:"email"`
			Bars  []barItem `json:"bars"`
			Total int64     `json:"total"`
		}
		all := deps.DBService.LoadTraffic(r.Context())
		result := make([]userBars, 0)
		for email, traffic := range all {
			u := userBars{Email: email}
			for _, t := range traffic {
				u.Bars = append(u.Bars, barItem{
					ServerID:   t.ServerID,
					ServerName: t.ServerName,
					Total:      t.Up + t.Down,
				})
				u.Total += t.Up + t.Down
			}
			if u.Total > 0 {
				result = append(result, u)
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].Total > result[j].Total })
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}))
}
