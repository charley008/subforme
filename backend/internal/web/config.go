package web

import (
	"encoding/json"
	"io"
	"net/http"

	"subforme/backend/internal/config"
)

func registerConfigRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/config/app", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		if r.Method == http.MethodPut {
			var next AppConfigPayload
			if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if err := deps.ConfigService.UpdateAppConfig(config.AppConfig(next)); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		appConfig, err := deps.ConfigService.ReadAppConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(appConfig)
	}))

	mux.HandleFunc("/api/config/groups", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		switch r.Method {
		case http.MethodGet:
			cfg, err := deps.ConfigService.ReadGroupsConfig()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cfg)
		case http.MethodPut:
			var cfg config.GroupConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if err := deps.ConfigService.UpdateGroupsConfig(cfg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/config/base", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			http.Error(w, "missing mode query", http.StatusBadRequest)
			return
		}
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		switch r.Method {
		case http.MethodGet:
			raw, err := deps.ConfigService.ReadBaseYAML(mode)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			_, _ = w.Write([]byte(raw))
		case http.MethodPut:
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read request body failed", http.StatusBadRequest)
				return
			}
			if err := deps.ConfigService.UpdateBaseYAML(mode, string(raw)); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/config/template", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		section := r.URL.Query().Get("section")
		if section == "" {
			http.Error(w, "missing section query", http.StatusBadRequest)
			return
		}
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		switch r.Method {
		case http.MethodGet:
			raw, err := deps.ConfigService.ReadTemplateSectionYAML(section)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			_, _ = w.Write([]byte(raw))
		case http.MethodPut:
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read request body failed", http.StatusBadRequest)
				return
			}
			if err := deps.ConfigService.UpdateTemplateSectionYAML(section, string(raw)); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/nodes", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		switch r.Method {
		case http.MethodGet:
			nodes, err := deps.ConfigService.ReadManagedNodes()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(nodes)
		case http.MethodPut:
			var next []config.ManagedNode
			if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if err := deps.ConfigService.UpdateManagedNodes(next); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/providers", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		providers, err := deps.ConfigService.ReadProviders()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(providers)
	}))
}

type AppConfigPayload config.AppConfig
