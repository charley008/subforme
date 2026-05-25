package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func registerPublicRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/proxy-providers/", func(w http.ResponseWriter, r *http.Request) {
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/proxy-providers/")
		if id == r.URL.Path || !strings.HasSuffix(id, ".yaml") {
			http.Error(w, "invalid provider path", http.StatusBadRequest)
			return
		}
		id = strings.TrimSuffix(id, ".yaml")
		if id == "" || strings.Contains(id, "/") || strings.Contains(id, ".") {
			http.Error(w, "invalid provider id", http.StatusBadRequest)
			return
		}
		raw, err := deps.ConfigService.ReadProviderFile(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(raw)
	})

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

		raw, err := generateForRequest(deps.SubscriptionService, r, user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(raw)
	})

	registerFrontendRoutes(mux, deps.FrontendDir)
}

func generateForRequest(service SubscriptionService, r *http.Request, user string) ([]byte, error) {
	if svc, ok := service.(PublicBaseSubscriptionService); ok {
		return svc.GenerateWithBaseURL(user, publicBaseURLForRequest(r))
	}
	return service.Generate(user)
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
