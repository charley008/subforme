package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"subforme/backend/internal/config"
	"subforme/backend/internal/xui"
)

type stubUserService struct {
	users []xui.UserSummary
	nodes []xui.Node
	err   error
}

func (s stubUserService) SearchUsers(query string) ([]xui.UserSummary, error) {
	return s.users, s.err
}

func (s stubUserService) PreviewUser(query string) ([]xui.Node, error) {
	return s.nodes, s.err
}

func TestUsersSearchReturnsResults(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		ConfigService: stubConfigService{
			app: config.AppConfig{Mode: "whitelist"},
		},
		UserService: stubUserService{
			users: []xui.UserSummary{{Email: "charley", Remark: "vless", NodeCount: 1}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/search?q=char", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %#v", got)
	}
}

func TestUsersPreviewReturnsNodes(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		ConfigService: stubConfigService{
			app: config.AppConfig{Mode: "whitelist"},
		},
		UserService: stubUserService{
			nodes: []xui.Node{{Name: "vless", Server: "example.com", Port: 6443}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/preview?user=charley", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("expected valid json, got %s", rec.Body.String())
	}
}
