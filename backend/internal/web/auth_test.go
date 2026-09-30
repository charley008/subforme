package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"subforme/backend/internal/auth"
	"subforme/backend/internal/db"
)

var testSessions = auth.NewSessions()

func addSessionCookie(req *http.Request) {
	rec := httptest.NewRecorder()
	if err := testSessions.Set(rec, req); err != nil {
		panic(err)
	}
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
		AuthService: stubAuthService{valid: false},
		Sessions:    testSessions,
		DBService:   loginRefreshDBService{refreshCh: refreshCh},
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
		AuthService: stubAuthService{valid: true},
		Sessions:    testSessions,
		DBService:   loginRefreshDBService{refreshCh: refreshCh},
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

func TestLoginSetsPersistentSessionCookie(t *testing.T) {
	router := NewRouter(Dependencies{
		AuthService: stubAuthService{valid: true},
		Sessions:    testSessions,
		DBService:   stubDBService{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one session cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge <= 0 {
		t.Fatalf("expected persistent cookie MaxAge, got %d", cookies[0].MaxAge)
	}
	if cookies[0].Expires.IsZero() {
		t.Fatal("expected persistent cookie Expires")
	}
}

func TestAuthMeReturnsAuthenticatedWhenCookiePresent(t *testing.T) {
	router := NewRouter(Dependencies{Sessions: testSessions})
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

func TestLogoutRejectsCookieReplay(t *testing.T) {
	sessions := auth.NewSessions()
	router := NewRouter(Dependencies{Sessions: sessions})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()
	if err := sessions.Set(rec, req); err != nil {
		t.Fatal(err)
	}
	cookie := rec.Result().Cookies()[0]
	req.AddCookie(cookie)
	router.ServeHTTP(httptest.NewRecorder(), req)
	check := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	check.AddCookie(cookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, check)
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"authenticated":false`)) {
		t.Fatalf("logout cookie replay accepted: %s", rec.Body.String())
	}
}

func TestPasswordChangeRevokesOldSessionsAndKeepsCurrentLogin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"admin_password":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessions()
	authService := &auth.Service{Username: "admin", Password: "old"}
	router := NewRouter(Dependencies{Sessions: sessions, AuthService: authService, RuntimePath: path})
	login := func(password string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"`+password+`"}`))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	first, second := login("old"), login("old")
	if first.Code != http.StatusNoContent || second.Code != http.StatusNoContent {
		t.Fatal("login failed")
	}
	req := httptest.NewRequest(http.MethodPut, "/api/auth/password", bytes.NewBufferString(`{"password":"new"}`))
	req.AddCookie(first.Result().Cookies()[0])
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("password change failed: %d %s", rec.Code, rec.Body.String())
	}
	for _, cookie := range []*http.Cookie{first.Result().Cookies()[0], second.Result().Cookies()[0]} {
		check := httptest.NewRequest(http.MethodGet, "/", nil)
		check.AddCookie(cookie)
		if sessions.Has(check) {
			t.Fatal("old session accepted after password change")
		}
	}
	check := httptest.NewRequest(http.MethodGet, "/", nil)
	check.AddCookie(rec.Result().Cookies()[0])
	if !sessions.Has(check) {
		t.Fatal("replacement current session rejected")
	}
	if login("old").Code != http.StatusUnauthorized || login("new").Code != http.StatusNoContent {
		t.Fatal("new password not applied")
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(raw, []byte(`"admin_password": "new"`)) {
		t.Fatalf("password was not persisted: %s %v", raw, err)
	}
}

func TestFailedPasswordSavePreservesSessions(t *testing.T) {
	sessions := auth.NewSessions()
	authService := &auth.Service{Username: "admin", Password: "old"}
	router := NewRouter(Dependencies{Sessions: sessions, AuthService: authService, RuntimePath: filepath.Join(t.TempDir(), "missing.json")})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/password", bytes.NewBufferString(`{"password":"new"}`))
	rec := httptest.NewRecorder()
	if err := sessions.Set(rec, req); err != nil {
		t.Fatal(err)
	}
	req.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || !sessions.Has(req) || !authService.Check("admin", "old") {
		t.Fatal("failed password save must preserve credentials and session")
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
