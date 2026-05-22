package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"subforme/backend/internal/auth"
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
	router := NewRouter(Dependencies{
		AuthService:   stubAuthService{valid: false},
		SessionSecret: testSessionSecret,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"bad","password":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
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
