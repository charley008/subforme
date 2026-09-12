package web

import (
	"encoding/json"
	"net/http"

	"subforme/backend/internal/config"
)

type nodePreviewService interface {
	PreviewManagedNode(string, config.ManagedNode) ([]byte, error)
}

func registerNodePreviewRoute(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/nodes/preview", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, ok := deps.ConfigService.(nodePreviewService)
		if !ok {
			http.Error(w, "node preview unavailable", http.StatusNotImplemented)
			return
		}
		var input struct {
			User string             `json:"user"`
			Node config.ManagedNode `json:"node"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 262144)).Decode(&input); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		raw, err := svc.PreviewManagedNode(input.User, input.Node)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]string{"yaml": string(raw)})
	}))
}
