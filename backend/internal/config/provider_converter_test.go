package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProviderConverterRefreshExtractsProxies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(`
proxies:
  - name: 🇭🇰 HK|Special:节点
    type: vmess
    server: hk.example.com
    port: 443
proxy-groups:
  - name: AUTO
    type: select
rules:
  - MATCH,DIRECT
`))
	}))
	defer server.Close()

	dir := t.TempDir()
	provider, err := UpsertProvider(dir, ProviderAddon{
		ID:                    "airport",
		Name:                  "Airport",
		SourceURL:             server.URL,
		UpdateIntervalSeconds: 3600,
	}, "https://sub.example.com")
	if err != nil {
		t.Fatalf("UpsertProvider returned error: %v", err)
	}
	if provider.ProxyProviders["airport"] == nil {
		t.Fatalf("expected generated proxy provider config")
	}
	if len(provider.ProxyGroups) != 1 {
		t.Fatalf("expected default proxy group, got %#v", provider.ProxyGroups)
	}
	if provider.ProxyGroups[0]["name"] != "airport" {
		t.Fatalf("expected provider group name, got %#v", provider.ProxyGroups[0])
	}
	if provider.ProxyGroups[0]["type"] != "url-test" {
		t.Fatalf("expected url-test provider group, got %#v", provider.ProxyGroups[0])
	}

	result, err := RefreshProvider(dir, "airport")
	if err != nil {
		t.Fatalf("RefreshProvider returned error: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("expected 1 proxy, got %d", result.Count)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "proxy_providers", "airport.yaml"))
	if err != nil {
		t.Fatalf("read generated provider file: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "proxies:") {
		t.Fatalf("expected proxies in output, got %s", got)
	}
	if strings.Contains(got, "proxy-groups") || strings.Contains(got, "rules:") {
		t.Fatalf("expected output to contain only proxies, got %s", got)
	}

	var parsed map[string][]map[string]any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse generated provider file: %v", err)
	}
	if parsed["proxies"][0]["name"] != "🇭🇰 HK|Special:节点" {
		t.Fatalf("expected special characters to round-trip, got %#v", parsed["proxies"][0]["name"])
	}
}

func TestProviderConverterStoresRefreshErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
		want string
	}{
		{name: "not found", code: http.StatusNotFound, body: "not found", want: "download returned 404"},
		{name: "invalid yaml", code: http.StatusOK, body: "not: [valid", want: "parse source yaml"},
		{name: "missing proxies", code: http.StatusOK, body: "rules:\n  - MATCH,DIRECT\n", want: "has no proxies"},
		{name: "empty proxies", code: http.StatusOK, body: "proxies: []\n", want: "contains no proxies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			dir := t.TempDir()
			if _, err := UpsertProvider(dir, ProviderAddon{
				ID:        "bad",
				SourceURL: server.URL,
			}, ""); err != nil {
				t.Fatalf("UpsertProvider returned error: %v", err)
			}

			if _, err := RefreshProvider(dir, "bad"); err == nil {
				t.Fatal("expected refresh error")
			}

			providers, err := LoadProviders(dir)
			if err != nil {
				t.Fatalf("LoadProviders returned error: %v", err)
			}
			if len(providers) != 1 || !strings.Contains(providers[0].LastError, tt.want) {
				t.Fatalf("expected last_error containing %q, got %#v", tt.want, providers)
			}
			if _, err := os.Stat(filepath.Join(dir, "proxy_providers", "bad.yaml")); !os.IsNotExist(err) {
				t.Fatalf("expected failed refresh not to write provider file, got err=%v", err)
			}
		})
	}
}
