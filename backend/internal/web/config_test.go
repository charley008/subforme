package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"subforme/backend/internal/config"
)

type stubConfigService struct {
	app       config.AppConfig
	groups    config.GroupConfig
	templates map[string]string
	nodes     []config.ManagedNode
	providers []config.ProviderAddon
	err       error
}

func (s stubConfigService) ReadAppConfig() (config.AppConfig, error) {
	return s.app, s.err
}

func (s stubConfigService) UpdateAppConfig(next config.AppConfig) error {
	s.app = next
	return s.err
}

func (s stubConfigService) ReadGroupsConfig() (config.GroupConfig, error) {
	return s.groups, s.err
}

func (s stubConfigService) UpdateGroupsConfig(next config.GroupConfig) error {
	s.groups = next
	return s.err
}

func (s stubConfigService) ReadTemplateSectionYAML(section string) (string, error) {
	if s.templates == nil {
		return "", s.err
	}
	return s.templates[section], s.err
}

func (s stubConfigService) UpdateTemplateSectionYAML(section, raw string) error {
	return s.err
}

func (s stubConfigService) ReadManagedNodes() ([]config.ManagedNode, error) {
	return s.nodes, s.err
}

func (s stubConfigService) UpdateManagedNodes(next []config.ManagedNode) error {
	return s.err
}

func (s stubConfigService) ReadProviders() ([]config.ProviderAddon, error) {
	return s.providers, s.err
}

func (s stubConfigService) UpsertProvider(provider config.ProviderAddon, publicBaseURL string) (config.ProviderAddon, error) {
	return provider, s.err
}

func (s stubConfigService) DeleteProvider(id string) error {
	return s.err
}

func (s stubConfigService) RefreshProvider(id string) (config.ProviderRefreshResult, error) {
	return config.ProviderRefreshResult{ID: id}, s.err
}

func (s stubConfigService) ReadProviderFile(id string) ([]byte, error) {
	return []byte("proxies: []\n"), s.err
}

func TestGetAppConfigReturnsJSON(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		ConfigService: stubConfigService{
			app: config.AppConfig{
				Mode:                       "whitelist",
				CacheTTLSeconds:            60,
				HealthcheckURL:             "https://www.gstatic.com/generate_204",
				HealthcheckIntervalSeconds: 300,
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/config/app", nil)
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
	if got["mode"] != "whitelist" {
		t.Fatalf("expected whitelist mode, got %#v", got)
	}
}

func TestGetGroupsConfigReturnsJSON(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		ConfigService: stubConfigService{
			groups: config.GroupConfig{
				GroupNames: config.GroupNames{Proxy: "节点选择", Auto: "自动选择", Other: "其他"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/config/groups", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "节点选择") {
		t.Fatalf("expected group data in response")
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected json content type")
	}
}

func TestConfigRoutesRequireSession(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionSecret: testSessionSecret,
		ConfigService: stubConfigService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/config/app", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
