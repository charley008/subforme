package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"subforme/backend/internal/app"
	"subforme/backend/internal/config"
	"subforme/backend/internal/db"
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

func TestDashboardTrafficReturnsSortedRows(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		DBService: stubDBService{
			traffic: map[string][]db.ServerTraffic{
				"light@example.com": {
					{ServerID: 1, ServerName: "HK", Up: 1, Down: 1},
				},
				"heavy@example.com": {
					{ServerID: 2, ServerName: "JP", Up: 10, Down: 5},
					{ServerID: 3, ServerName: "US", Up: 5, Down: 5},
				},
				"zero@example.com": {
					{ServerID: 4, ServerName: "Zero", Up: 0, Down: 0},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/traffic", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got []struct {
		Email string `json:"email"`
		Total int64  `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected zero-traffic user to be omitted, got %#v", got)
	}
	if got[0].Email != "heavy@example.com" || got[0].Total != 25 {
		t.Fatalf("expected heavy user first, got %#v", got)
	}
}

type stubDBService struct {
	servers []db.Server
	users   []db.User
	nodes   []db.Node2
	traffic map[string][]db.ServerTraffic
}

func (s stubDBService) DBUserSearch(query string) ([]db.User, error) {
	return s.users, nil
}

func (s stubDBService) DBUserList() ([]db.User, error) {
	return s.users, nil
}

func (s stubDBService) DBCreateUser(u *db.User) error {
	return nil
}

func (s stubDBService) DBUpdateUser(u *db.User) error {
	return nil
}

func (s stubDBService) DBDeleteUser(id int64) error {
	return nil
}

func (s stubDBService) DBGetUserProtocols(userID int64) (string, int) {
	return "", 0
}

func (s stubDBService) DBGetUserTraffic(userID int64) []db.ServerTraffic {
	return nil
}

func (s stubDBService) RefreshTraffic(ctx context.Context) map[string][]db.ServerTraffic {
	return s.traffic
}

func (s stubDBService) LoadTraffic(ctx context.Context) map[string][]db.ServerTraffic {
	return s.traffic
}

func (s stubDBService) ImportFromServer(ctx context.Context, serverID int64) (*app.ImportResult, error) {
	return &app.ImportResult{}, nil
}

func (s stubDBService) SyncToServers(ctx context.Context) (*app.SyncResult, error) {
	return &app.SyncResult{}, nil
}

func (s stubDBService) DBListServers() ([]db.Server, error) {
	return s.servers, nil
}

func (s stubDBService) DBCreateServer(sv *db.Server) error {
	return nil
}

func (s stubDBService) DBUpdateServer(sv *db.Server) error {
	return nil
}

func (s stubDBService) DBDeleteServer(id int64) error {
	return nil
}

func (s stubDBService) DBListNodeDB() ([]db.Node2, error) {
	return s.nodes, nil
}

func (s stubDBService) DBReplaceNodes(nodes []db.Node2) error {
	return nil
}
