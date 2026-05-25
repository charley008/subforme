package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"subforme/backend/internal/auth"
	"subforme/backend/internal/db"
)

const testSessionSecret = "test-session-secret"

func addSessionCookie(req *http.Request) {
	rec := httptest.NewRecorder()
	auth.SetSession(rec, testSessionSecret)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
}

type stubAuthService struct {
	valid bool
}

func (s stubAuthService) Check(username, password string) bool {
	return s.valid
}

func (s stubAuthService) UpdatePassword(newPassword string) {}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	refreshCh := make(chan struct{}, 1)
	router := NewRouter(Dependencies{
		AuthService:   stubAuthService{valid: false},
		SessionSecret: testSessionSecret,
		DBService:     loginRefreshDBService{refreshCh: refreshCh},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"bad","password":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	select {
	case <-refreshCh:
		t.Fatal("traffic refresh should not run after failed login")
	default:
	}
}

func TestLoginRefreshesTrafficAfterSuccess(t *testing.T) {
	refreshCh := make(chan struct{}, 1)
	router := NewRouter(Dependencies{
		AuthService:   stubAuthService{valid: true},
		SessionSecret: testSessionSecret,
		DBService:     loginRefreshDBService{refreshCh: refreshCh},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	select {
	case <-refreshCh:
	case <-time.After(time.Second):
		t.Fatal("expected login to trigger traffic refresh")
	}
}

func TestAuthMeReturnsAuthenticatedWhenCookiePresent(t *testing.T) {
	router := NewRouter(Dependencies{SessionSecret: testSessionSecret})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"authenticated":true`)) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

type loginRefreshDBService struct {
	stubDBService
	refreshCh chan struct{}
}

func (s loginRefreshDBService) RefreshTraffic(ctx context.Context) map[string][]db.ServerTraffic {
	select {
	case s.refreshCh <- struct{}{}:
	default:
	}
	return map[string][]db.ServerTraffic{"alice": nil}
}
