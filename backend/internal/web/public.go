package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func registerPublicRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/sub", func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		if user == "" {
			http.Error(w, "missing user query", http.StatusBadRequest)
			return
		}
		if deps.SubscriptionService == nil {
			http.Error(w, "subscription service unavailable", http.StatusNotImplemented)
			return
		}

		raw, err := deps.SubscriptionService.Generate(user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(raw)
	})

	registerFrontendRoutes(mux, deps.FrontendDir)
}

func registerFrontendRoutes(mux *http.ServeMux, frontendDir string) {
	if frontendDir == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(frontendDir, "index.html")); err != nil {
		return
	}

	fileServer := http.FileServer(http.Dir(frontendDir))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
			return
		}
		target := filepath.Join(frontendDir, p)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
	}))
}
