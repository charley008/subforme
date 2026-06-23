package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"subforme/backend/internal/db"
	"subforme/backend/internal/xui"
)

func registerDBRoutes(mux *http.ServeMux, deps Dependencies) {
	// Servers CRUD
	mux.HandleFunc("/api/servers", requireSession(deps.SessionSecret, handleServersList(deps)))
	mux.HandleFunc("/api/servers/add", requireSession(deps.SessionSecret, handleServerAdd(deps)))
	mux.HandleFunc("/api/servers/update/", requireSession(deps.SessionSecret, handleServerUpdate(deps)))
	mux.HandleFunc("/api/servers/delete/", requireSession(deps.SessionSecret, handleServerDelete(deps)))

	// Import & Sync & Traffic
	mux.HandleFunc("/api/servers/import/", requireSession(deps.SessionSecret, handleServerImport(deps)))
	mux.HandleFunc("/api/sync", requireSession(deps.SessionSecret, handleSync(deps)))
	mux.HandleFunc("/api/traffic/refresh", requireSession(deps.SessionSecret, handleTrafficRefresh(deps)))
	mux.HandleFunc("/api/traffic/load", requireSession(deps.SessionSecret, handleTrafficLoad(deps)))
	mux.HandleFunc("/api/traffic/reset-server/", requireSession(deps.SessionSecret, handleTrafficResetServer(deps)))

	// Server test connection
	mux.HandleFunc("/api/servers/test/", requireSession(deps.SessionSecret, handleServerTest(deps)))

	// DB Users
	mux.HandleFunc("/api/db/users", requireSession(deps.SessionSecret, handleDBUsersList(deps)))
	mux.HandleFunc("/api/db/users/search", requireSession(deps.SessionSecret, handleDBUsersSearch(deps)))
	mux.HandleFunc("/api/db/users/add", requireSession(deps.SessionSecret, handleDBUserAdd(deps)))
	mux.HandleFunc("/api/db/users/update/", requireSession(deps.SessionSecret, handleDBUserUpdate(deps)))
	mux.HandleFunc("/api/db/users/delete/", requireSession(deps.SessionSecret, handleDBUserDelete(deps)))
}

// ─── Servers CRUD ──────────────────────────────────────────────────

func handleServersList(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		servers, err := deps.DBService.DBListServers()
		if err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, servers)
	}
}

func handleServerAdd(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var sv db.Server
		if err := json.NewDecoder(r.Body).Decode(&sv); err != nil {
			writeError(w, "invalid request body")
			return
		}
		if sv.Name == "" || sv.Host == "" {
			writeError(w, "name and host are required")
			return
		}
		if sv.Scheme == "" {
			sv.Scheme = "https"
		}
		if sv.Port == 0 {
			sv.Port = 2053
		}
		if sv.BasePath == "" {
			sv.BasePath = "/xui/"
		}
		if err := deps.DBService.DBCreateServer(&sv); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, sv)
	}
}

func handleServerUpdate(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := extractID(r.URL.Path, "/api/servers/update/")
		if err != nil {
			writeError(w, "invalid server id")
			return
		}
		var sv db.Server
		if err := json.NewDecoder(r.Body).Decode(&sv); err != nil {
			writeError(w, "invalid request body")
			return
		}
		sv.ID = id
		if err := deps.DBService.DBUpdateServer(&sv); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, sv)
	}
}

func handleServerDelete(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := extractID(r.URL.Path, "/api/servers/delete/")
		if err != nil {
			writeError(w, "invalid server id")
			return
		}
		if err := deps.DBService.DBDeleteServer(id); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}

// ─── Import & Sync ────────────────────────────────────────────────

func handleServerImport(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := extractID(r.URL.Path, "/api/servers/import/")
		if err != nil {
			writeError(w, "invalid server id")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()
		result, err := deps.DBService.ImportFromServer(ctx, id)
		if err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, result)
	}
}

func handleSync(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		result, err := deps.DBService.SyncToServers(ctx)
		if err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, result)
	}
}

// ─── Server Test Connection ───────────────────────────────────────

func handleServerTest(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := extractID(r.URL.Path, "/api/servers/test/")
		if err != nil {
			writeError(w, "invalid server id")
			return
		}

		servers, err := deps.DBService.DBListServers()
		if err != nil {
			writeError(w, err.Error())
			return
		}
		var target *db.Server
		for _, sv := range servers {
			if sv.ID == id {
				target = &sv
				break
			}
		}
		if target == nil {
			writeError(w, "server not found")
			return
		}

		baseURL := xuiBaseURL(target)
		cli := xui.NewClient(baseURL, target.APIKey, "", "")
		resolver := xui.NewResolver(cli)
		status, err := resolver.TestConnection(context.Background())
		if err != nil {
			writeJSON(w, status)
			return
		}
		writeJSON(w, status)
	}
}

// ─── DB Users ─────────────────────────────────────────────────────

func handleDBUsersList(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := deps.DBService.DBUserList()
		if err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, users)
	}
}

func handleDBUsersSearch(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		users, err := deps.DBService.DBUserSearch(q)
		if err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, users)
	}
}

func handleDBUserAdd(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var u db.User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			writeError(w, "invalid request body")
			return
		}
		if err := deps.DBService.DBCreateUser(&u); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, u)
	}
}

func handleDBUserUpdate(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := extractID(r.URL.Path, "/api/db/users/update/")
		if err != nil {
			writeError(w, "invalid user id")
			return
		}
		var u db.User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			writeError(w, "invalid request body")
			return
		}
		u.ID = id
		if err := deps.DBService.DBUpdateUser(&u); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, u)
	}
}

func handleDBUserDelete(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := extractID(r.URL.Path, "/api/db/users/delete/")
		if err != nil {
			writeError(w, "invalid user id")
			return
		}
		if err := deps.DBService.DBDeleteUser(id); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}

// ─── Helpers ──────────────────────────────────────────────────────

func extractID(path, prefix string) (int64, error) {
	idStr := strings.TrimPrefix(path, prefix)
	idStr = strings.TrimSuffix(idStr, "/")
	return strconv.ParseInt(idStr, 10, 64)
}

func handleTrafficLoad(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		traffic := deps.DBService.LoadTraffic(r.Context())
		writeJSON(w, traffic)
	}
}

func handleTrafficRefresh(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		traffic := deps.DBService.RefreshTraffic(r.Context())
		writeJSON(w, traffic)
	}
}

func handleTrafficResetServer(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := extractID(r.URL.Path, "/api/traffic/reset-server/")
		if err != nil {
			writeError(w, "invalid server id")
			return
		}
		result, err := deps.DBService.ResetServerUserTraffic(r.Context(), id)
		if err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, result)
	}
}

func xuiBaseURL(sv *db.Server) string {
	base := strings.TrimRight(sv.BasePath, "/")
	return sv.Scheme + "://" + sv.Host + ":" + strconv.Itoa(sv.Port) + base
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
