package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"subforme/backend/internal/config"
	"subforme/backend/internal/db"
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
		if deps.DBService == nil {
			http.Error(w, "db service unavailable", http.StatusNotImplemented)
			return
		}
		switch r.Method {
		case http.MethodGet:
			dbNodes, err := deps.DBService.DBListNodeDB()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(dbNodes)
		case http.MethodPut, http.MethodPost:
			var next []db.Node2
			if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if err := deps.DBService.DBReplaceNodes(next); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			// Return saved nodes with generated IDs
			saved, _ := deps.DBService.DBListNodeDB()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(saved)
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

	mux.HandleFunc("/api/provider-converters", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if deps.ConfigService == nil {
			http.Error(w, "config service unavailable", http.StatusNotImplemented)
			return
		}
		switch r.Method {
		case http.MethodGet:
			providers, err := deps.ConfigService.ReadProviders()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(providers)
		case http.MethodPost:
			var provider config.ProviderAddon
			if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			saved, err := deps.ConfigService.UpsertProvider(provider, publicBaseURLForRequest(r))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(saved)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/provider-converters/refresh/", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/provider-converters/refresh/")
		result, err := deps.ConfigService.RefreshProvider(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}))

	mux.HandleFunc("/api/provider-converters/delete/", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/provider-converters/delete/")
		if err := deps.ConfigService.DeleteProvider(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

type AppConfigPayload config.AppConfig
