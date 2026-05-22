package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubSubscriptionService struct {
	payload []byte
	err     error
}

func (s stubSubscriptionService) Generate(user string) ([]byte, error) {
	return s.payload, s.err
}

func TestSubRequiresUserQuery(t *testing.T) {
	router := NewRouter(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/sub", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPreviewReturnsGeneratedYAML(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		SubscriptionService: stubSubscriptionService{
			payload: []byte("proxies:\n  - name: HK-01\n"),
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/sub/preview?user=charley", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "proxies:\n  - name: HK-01\n" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}
