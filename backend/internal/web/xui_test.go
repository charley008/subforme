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
