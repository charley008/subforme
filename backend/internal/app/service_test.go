package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"subforme/backend/internal/config"
	"subforme/backend/internal/xui"
)

func TestGenerateReturnsErrorWhenUserMissing(t *testing.T) {
	svc := Service{}
	_, err := svc.Generate("charley")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpsertClientInSettingsPreservesInboundSettings(t *testing.T) {
	raw := `{"clients":[{"email":"alice","id":"uuid-1"}],"decryption":"none","testseed":[900,500]}`
	got := upsertClientInSettings(raw, xui.InboundClient{Email: "bob", ID: "uuid-2", Enable: true})

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("settings json is invalid: %v", err)
	}
	if decoded["decryption"] != "none" {
		t.Fatalf("expected decryption to be preserved, got %#v", decoded)
	}
	clients, ok := decoded["clients"].([]any)
	if !ok || len(clients) != 2 {
		t.Fatalf("expected two clients, got %#v", decoded["clients"])
	}
}

func TestRemoveClientFromSettingsPreservesInboundSettings(t *testing.T) {
	raw := `{"clients":[{"email":"alice","id":"uuid-1"},{"email":"bob","id":"uuid-2"}],"encryption":"none"}`
	got := removeClientFromSettings(raw, "alice")

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("settings json is invalid: %v", err)
	}
	if decoded["encryption"] != "none" {
		t.Fatalf("expected encryption to be preserved, got %#v", decoded)
	}
	clients, ok := decoded["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("expected one remaining client, got %#v", decoded["clients"])
	}
	remaining := clients[0].(map[string]any)
	if remaining["email"] != "bob" {
		t.Fatalf("expected bob to remain, got %#v", remaining)
	}
}

func TestGenerateBuildsYAMLFromBundleAndNodes(t *testing.T) {
	svc := Service{
		ConfigDir: "ignored",
		Loader: func(dir string) (config.Bundle, error) {
			return config.Bundle{
				App: config.AppConfig{
					Mode: "whitelist",
					XUI:  config.XUIConfig{BaseURL: "https://panel.example.com/xui", APIKey: "token"},
				},
				BaseWhitelist: config.BaseConfig{
					Mode:        "rule",
					LogLevel:    "info",
					Proxies:     []any{},
					ProxyGroups: []any{},
					Rules:       []string{"MATCH,DIRECT"},
				},
				BaseBlacklist: config.BaseConfig{
					Mode:        "rule",
					LogLevel:    "info",
					Proxies:     []any{},
					ProxyGroups: []any{},
					Rules:       []string{"MATCH,PROXY"},
				},
				Groups: config.GroupConfig{
					Regions: map[string][]string{
						"HK": {"HK"},
					},
					GroupNames: config.GroupNames{
						Proxy: "节点选择",
						Auto:  "自动选择",
						Other: "其他节点",
					},
					Healthcheck: config.GroupHealthcheck{
						URL:             "https://www.gstatic.com/generate_204",
						IntervalSeconds: 300,
					},
				},
			}, nil
		},
		ResolverFactory: func(cfg config.XUIConfig) XUIResolver {
			return fakeResolver{
				nodes: []xui.Node{
					{
						Name:              "HK-01",
						Type:              "vless",
						Server:            "hk.example.com",
						Port:              443,
						UUID:              "uuid-1",
						TLS:               true,
						ClientFingerprint: "chrome",
					},
				},
			}
		},
		TemplateLoader: func(dir, mode string) (string, error) {
			return "mixed-port: 10801\nallow-lan: true\nmode: rule\nproxies: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n", nil
		},
	}

	raw, err := svc.Generate("charley")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	got := string(raw)
	if !strings.Contains(got, "name: HK-01") {
		t.Fatalf("expected generated proxy in yaml, got %s", got)
	}
	if !strings.Contains(got, "name: 节点选择") {
		t.Fatalf("expected generated group in yaml, got %s", got)
	}
}

func TestGenerateUsesFirstSelectGroupWhenGroupNameProxyIsEmpty(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveProviders(tmp, []config.ProviderAddon{{
		ID:             "airport",
		Name:           "airport",
		ProxyProviders: map[string]any{"airport": map[string]any{"type": "http", "url": "https://example.com/provider.yaml"}},
	}}); err != nil {
		t.Fatalf("save providers: %v", err)
	}

	svc := Service{
		ConfigDir: tmp,
		Loader: func(dir string) (config.Bundle, error) {
			return config.Bundle{
				App: config.AppConfig{
					Mode:          "whitelist",
					UserProviders: map[string][]string{"alice": {"airport"}},
				},
				BaseWhitelist: config.BaseConfig{
					Rules: []string{"MATCH,DIRECT"},
				},
				BaseBlacklist: config.BaseConfig{
					Rules: []string{"MATCH,PROXY"},
				},
				Groups: config.GroupConfig{
					Groups: []config.GroupDef{
						{Name: "PROXY", Type: "select"},
						{Name: "GPT", Type: "url-test", URL: "https://www.gstatic.com/generate_204", Interval: 300},
					},
				},
			}, nil
		},
		ResolverFactory: func(cfg config.XUIConfig) XUIResolver {
			return fakeResolver{
				nodes: []xui.Node{{Name: "node-a", Type: "vless", Server: "example.com", Port: 443, UUID: "uuid"}},
			}
		},
		TemplateLoader: func(dir, mode string) (string, error) {
			return "proxies: []\nproxy-groups: []\nproxy-providers: {}\nrules:\n  - MATCH,DIRECT\n", nil
		},
	}

	raw, err := svc.Generate("alice")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "name: PROXY") || !strings.Contains(got, "- airport") {
		t.Fatalf("expected provider to be attached to PROXY, got %s", got)
	}
}

type fakeResolver struct {
	nodes  []xui.Node
	users  []xui.UserSummary
	avail  []xui.AvailableNode
	status xui.ConnectionStatus
	err    error
}

func (f fakeResolver) ResolveUserNodes(ctx context.Context, query string) ([]xui.Node, error) {
	return f.nodes, f.err
}

func (f fakeResolver) SearchUsers(ctx context.Context, query string) ([]xui.UserSummary, error) {
	return f.users, f.err
}

func (f fakeResolver) ListAvailableNodes(ctx context.Context) ([]xui.AvailableNode, error) {
	return f.avail, f.err
}

func (f fakeResolver) TestConnection(ctx context.Context) (xui.ConnectionStatus, error) {
	return f.status, f.err
}
