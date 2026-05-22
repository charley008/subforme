package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"subforme/backend/internal/config"
	"subforme/backend/internal/xui"
)

func TestDashboardSummaryReturnsCombinedStatus(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		ConfigService: stubConfigService{
			app: config.AppConfig{
				Mode:                       "whitelist",
				CacheTTLSeconds:            60,
				HealthcheckURL:             "https://www.gstatic.com/generate_204",
				HealthcheckIntervalSeconds: 300,
			},
			nodes: []config.ManagedNode{
				{ID: "1", Name: "hk", Address: "hk.sample.com"},
			},
		},
		XUIService: stubXUIService{
			status: xui.ConnectionStatus{
				OK:           true,
				BaseURL:      "https://panel.example.com/xui",
				InboundCount: 2,
				EnabledCount: 2,
			},
		},
		UserService: stubUserService{
			users: []xui.UserSummary{
				{Email: "alpha@example.com"},
				{Email: "beta@example.com"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/summary", nil)
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
	if got["mode"] != "默认直连" {
		t.Fatalf("expected whitelist mode, got %#v", got)
	}
	if got["unique_users"] != float64(2) {
		t.Fatalf("expected 2 unique users, got %#v", got)
	}
}
