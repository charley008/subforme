package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func issueSession(t *testing.T, sessions *Sessions) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	if err := sessions.Set(rec, req); err != nil {
		t.Fatal(err)
	}
	req.AddCookie(rec.Result().Cookies()[0])
	return req
}

func TestRejectsForgedAndLegacySessions(t *testing.T) {
	sessions := NewSessions()
	for _, value := range []string{"ok", "ok.known-signature", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: value})
		if sessions.Has(req) {
			t.Errorf("accepted unissued cookie %q", value)
		}
	}
}

func TestSessionExpiresOnServer(t *testing.T) {
	now := time.Unix(1700000000, 0)
	sessions := NewSessions()
	sessions.now = func() time.Time { return now }
	req := issueSession(t, sessions)
	if !sessions.Has(req) {
		t.Fatal("fresh session rejected")
	}
	now = now.Add(time.Duration(sessionMaxAge) * time.Second)
	if sessions.Has(req) {
		t.Fatal("accepted expired cookie replay")
	}
}

func TestLogoutRevokesOnlyCurrentSession(t *testing.T) {
	sessions := NewSessions()
	first, second := issueSession(t, sessions), issueSession(t, sessions)
	rec := httptest.NewRecorder()
	sessions.Clear(rec, first)
	if sessions.Has(first) || !sessions.Has(second) {
		t.Fatal("logout must revoke current session and preserve other sessions")
	}
	if rec.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout must delete browser cookie")
	}
}

func TestRevokeAllAndRestartInvalidateSessions(t *testing.T) {
	sessions := NewSessions()
	first, second := issueSession(t, sessions), issueSession(t, sessions)
	if NewSessions().Has(first) {
		t.Fatal("another server instance accepted session")
	}
	sessions.RevokeAll()
	if sessions.Has(first) || sessions.Has(second) {
		t.Fatal("revoked session accepted")
	}
	if !sessions.Has(issueSession(t, sessions)) {
		t.Fatal("new session after revocation rejected")
	}
}

func TestCookieFlagsForTLS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	if err := NewSessions().Set(rec, req); err != nil {
		t.Fatal(err)
	}
	cookie := rec.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != sessionMaxAge {
		t.Fatalf("unexpected cookie flags: %#v", cookie)
	}
}

func TestConcurrentSessionLifecycle(t *testing.T) {
	sessions := NewSessions()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			if err := sessions.Set(rec, req); err != nil {
				t.Error(err)
				return
			}
			req.AddCookie(rec.Result().Cookies()[0])
			_ = sessions.Has(req)
			sessions.Clear(httptest.NewRecorder(), req)
		})
	}
	wg.Wait()
	if len(sessions.tokens) != 0 {
		t.Fatal("sessions remain after logout")
	}
}
