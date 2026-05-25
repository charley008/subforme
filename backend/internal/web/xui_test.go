package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"subforme/backend/internal/xui"
)

type stubXUIService struct {
	status xui.ConnectionStatus
	nodes  []xui.AvailableNode
	err    error
}

func TestPublicBaseURLForRequestUsesForwardedPrefix(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/sub?user=alice", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "www.4738.org")
	req.Header.Set("X-Forwarded-Prefix", "/sub")

	if got := publicBaseURLForRequest(req); got != "https://www.4738.org/sub" {
		t.Fatalf("unexpected public base URL: %s", got)
	}
}

func (s stubXUIService) TestConnection(ctx context.Context) (xui.ConnectionStatus, error) {
	return s.status, s.err
}

func (s stubXUIService) DetectAvailableNodes() ([]xui.AvailableNode, error) {
	return s.nodes, s.err
}

func TestXUITestRouteReturnsStatus(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		XUIService: stubXUIService{
			status: xui.ConnectionStatus{
				OK:           true,
				BaseURL:      "https://panel.example.com/xui",
				InboundCount: 3,
				EnabledCount: 2,
				DetectedPath: "/xui/panel/api/inbounds/list",
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/xui/test", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["ok"] != true {
		t.Fatalf("expected ok response, got %#v", got)
	}
}
