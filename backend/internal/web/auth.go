package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"subforme/backend/internal/auth"
	"subforme/backend/internal/config"
)

func requireSession(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.HasSession(r, secret) {
			log.Printf("[auth] denied method=%s path=%s remote=%s", r.Method, safeRequestURI(r), remoteAddr(r))
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Printf("[admin] method=%s path=%s status=%d duration=%s remote=%s", r.Method, safeRequestURI(r), status, time.Since(start).Round(time.Millisecond), remoteAddr(r))
	}
}

func registerAuthRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req auth.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[auth] login invalid_json remote=%s", remoteAddr(r))
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if deps.AuthService == nil || !deps.AuthService.Check(req.Username, req.Password) {
			log.Printf("[auth] login failed username=%s remote=%s", req.Username, remoteAddr(r))
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		auth.SetSession(w, deps.SessionSecret)
		log.Printf("[auth] login success username=%s remote=%s", req.Username, remoteAddr(r))
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if auth.HasSession(r, deps.SessionSecret) {
			log.Printf("[auth] logout remote=%s", remoteAddr(r))
		}
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(data)
}

func safeRequestURI(r *http.Request) string {
	if r.URL == nil {
		return ""
	}
	return r.URL.RequestURI()
}

func remoteAddr(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}
