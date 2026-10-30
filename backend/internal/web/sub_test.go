package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"subforme/backend/internal/app"
	"subforme/backend/internal/config"
)

type stubSubscriptionService struct {
	payload []byte
	err     error
}

func (s stubSubscriptionService) Generate(user string) ([]byte, error) {
	return s.payload, s.err
}

type stubVariantSubscriptionService struct {
	user    string
	variant string
	payload []byte
	err     error
}

func (s *stubVariantSubscriptionService) Generate(user string) ([]byte, error) {
	s.user = user
	return s.payload, s.err
}

func (s *stubVariantSubscriptionService) GenerateWithBaseURLAndVariant(user, publicBaseURL, variant string) ([]byte, error) {
	s.user = user
	s.variant = variant
	return s.payload, s.err
}

type stubOptionsSubscriptionService struct {
	stubVariantSubscriptionService
	options app.SubscriptionOptions
	baseURL string
}

func (s *stubOptionsSubscriptionService) GenerateWithOptions(user, publicBaseURL string, options app.SubscriptionOptions) ([]byte, error) {
	s.user = user
	s.options = options
	s.baseURL = publicBaseURL
	return s.payload, s.err
}

func TestSubAndPreviewPassSubscriptionOptions(t *testing.T) {
	for _, path := range []string{"/api/sub", "/api/sub/preview"} {
		for _, tt := range []struct {
			query string
			want  app.SubscriptionOptions
		}{
			{query: ""},
			{query: "&mode=home", want: app.SubscriptionOptions{Mode: "home"}},
			{query: "&mode=office", want: app.SubscriptionOptions{Mode: "office"}},
			{query: "&mode=Home", want: app.SubscriptionOptions{Mode: "Home"}},
			{query: "&type=ios", want: app.SubscriptionOptions{Variant: "ios"}},
			{query: "&type=ios&mode=office", want: app.SubscriptionOptions{Variant: "ios", Mode: "office"}},
		} {
			t.Run(path+tt.query, func(t *testing.T) {
				svc := &stubOptionsSubscriptionService{stubVariantSubscriptionService: stubVariantSubscriptionService{payload: []byte("rules: []\n")}}
				router := NewRouter(Dependencies{Sessions: testSessions, SubscriptionService: svc})
				req := httptest.NewRequest(http.MethodGet, "https://sub.example.com"+path+"?user=charley"+tt.query, nil)
				if path == "/api/sub/preview" {
					addSessionCookie(req)
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK || rec.Body.String() != "rules: []\n" {
					t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
				}
				if svc.user != "charley" || svc.options != tt.want || svc.baseURL != "https://sub.example.com" {
					t.Fatalf("unexpected request: user=%q options=%+v baseURL=%q", svc.user, svc.options, svc.baseURL)
				}
			})
		}
	}
}

func TestSubRequiresUserQuery(t *testing.T) {
	router := NewRouter(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/sub", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPreviewReturnsGeneratedYAML(t *testing.T) {
	router := NewRouter(Dependencies{
		Sessions: testSessions,
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

func TestSubPassesIOSVariantFromTypeQuery(t *testing.T) {
	svc := &stubVariantSubscriptionService{payload: []byte("ios: true\n")}
	router := NewRouter(Dependencies{SubscriptionService: svc})
	req := httptest.NewRequest(http.MethodGet, "/api/sub?user=charley&type=ios", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.user != "charley" || svc.variant != "ios" {
		t.Fatalf("unexpected generated request: user=%q variant=%q", svc.user, svc.variant)
	}
}

func TestSubIgnoresModelQuery(t *testing.T) {
	svc := &stubVariantSubscriptionService{payload: []byte("ios: true\n")}
	router := NewRouter(Dependencies{SubscriptionService: svc})
	req := httptest.NewRequest(http.MethodGet, "/api/sub?user=charley&model=ios", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.variant != "" {
		t.Fatalf("expected empty variant, got %q", svc.variant)
	}
}

func TestProxyProviderRouteRejectsTraversal(t *testing.T) {
	router := NewRouter(Dependencies{
		ConfigService: stubConfigService{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/proxy-providers/%2e%2e/config.yaml", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestProxyProviderRouteServesProviderFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "proxy_providers"), 0o755); err != nil {
		t.Fatalf("mkdir proxy providers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proxy_providers", "airport.yaml"), []byte("proxies: []\n"), 0o644); err != nil {
		t.Fatalf("write provider file: %v", err)
	}
	router := NewRouter(Dependencies{
		ConfigService: providerFileConfigService{dir: dir},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/proxy-providers/airport.yaml", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "proxies: []\n" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

type providerFileConfigService struct {
	stubConfigService
	dir string
}

func (s providerFileConfigService) ReadProviderFile(id string) ([]byte, error) {
	return config.ReadProviderFile(s.dir, id)
}
