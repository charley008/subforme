package web

import (
	"encoding/json"
	"net/http"

	"subforme/backend/internal/auth"
	"subforme/backend/internal/config"
)

func requireSession(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.HasSession(r, secret) {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func registerAuthRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req auth.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if deps.AuthService == nil || !deps.AuthService.Check(req.Username, req.Password) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		auth.SetSession(w, deps.SessionSecret)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		auth.ClearSession(w)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authenticated":  auth.HasSession(r, deps.SessionSecret),
			"admin_username": deps.AdminUsername,
		})
	})

	mux.HandleFunc("/api/auth/password", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if req.Password == "" {
			http.Error(w, "password cannot be empty", http.StatusBadRequest)
			return
		}
		if err := config.SaveRuntimePassword(deps.RuntimePath, req.Password); err != nil {
			http.Error(w, "save password failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if deps.AuthService != nil {
			deps.AuthService.UpdatePassword(req.Password)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}