package app

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"subforme/backend/internal/config"
	"subforme/backend/internal/xui"
)

func TestGenerateProviderRulesAfterTemplateMerge(t *testing.T) {
	dir := t.TempDir()
	const home = "IP-CIDR,192.168.1.0/24,家庭-WireGuard,no-resolve"
	const office = "IP-CIDR,10.10.0.0/16,办公室-WireGuard,no-resolve"
	const ordinary = "GEOIP,CN,DIRECT"
	template := "rules:\n  # @provider:home-wg\n  - " + home + "\n\n  # @provider:office-wg\n  - " + office + "\n\n  - " + ordinary + "\n"
	for _, section := range []string{"whitelist", "blacklist", "ios"} {
		if err := config.SaveTemplateSectionYAML(dir, section, template); err != nil {
			t.Fatal(err)
		}
	}
	addons := []config.ProviderAddon{
		{ID: "home-wg", ProxyProviders: map[string]any{"home-wg": map[string]any{"type": "http"}}, ProxyGroups: []map[string]any{{"name": "家庭-WireGuard", "type": "select", "use": []string{"home-wg"}}}},
		{ID: "office-wg", ProxyProviders: map[string]any{"office-wg": map[string]any{"type": "http"}}, ProxyGroups: []map[string]any{{"name": "办公室-WireGuard", "type": "select", "use": []string{"office-wg"}}}},
	}
	if err := config.SaveProviders(dir, addons); err != nil {
		t.Fatal(err)
	}
	svc := NewService(dir, config.XUIConfig{})
	svc.Loader = func(string) (config.Bundle, error) {
		return config.Bundle{App: config.AppConfig{
			UserProviders: map[string][]string{"alice": {"home-wg"}, "bob": {"office-wg"}},
			UserModes:     map[string]string{"bob": "blacklist"},
		}}, nil
	}
	svc.ResolverFactory = func(config.XUIConfig) XUIResolver {
		return fakeResolver{nodes: []xui.Node{{Name: "node-a", Type: "vless", Server: "example.com", Port: 443, UUID: "uuid-1"}}}
	}
	for _, tt := range []struct {
		user    string
		variant string
		mode    string
		want    []string
	}{
		{user: "alice", want: []string{ordinary}},
		{user: "bob", want: []string{ordinary}},
		{user: "charlie", want: []string{ordinary}},
		{user: "alice", mode: "wg", want: []string{home, ordinary}},
		{user: "bob", mode: "wg", want: []string{office, ordinary}},
		{user: "charlie", mode: "wg", want: []string{ordinary}},
		{user: "alice", mode: " WG ", want: []string{home, ordinary}},
		{user: "alice", variant: "ios", want: []string{ordinary}},
		{user: "charlie", variant: "ios", want: []string{ordinary}},
		{user: "alice", variant: "ios", mode: "wg", want: []string{home, ordinary}},
		{user: "charlie", variant: "ios", mode: "wg", want: []string{ordinary}},
	} {
		t.Run(tt.user+"/"+tt.variant+"/"+tt.mode, func(t *testing.T) {
			raw, err := svc.GenerateWithOptions(tt.user, "", SubscriptionOptions{Variant: tt.variant, Mode: tt.mode})
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Rules     []string       `yaml:"rules"`
				Providers map[string]any `yaml:"proxy-providers"`
				Groups    []struct {
					Name string   `yaml:"name"`
					Use  []string `yaml:"use"`
				} `yaml:"proxy-groups"`
				Proxies []struct {
					Name string `yaml:"name"`
				} `yaml:"proxies"`
			}
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(doc.Rules, tt.want) {
				t.Fatalf("rules = %#v, want %#v; YAML:\n%s", doc.Rules, tt.want, raw)
			}
			if len(doc.Proxies) != 1 || doc.Proxies[0].Name != "node-a" {
				t.Fatalf("3x-ui node changed: %+v", doc.Proxies)
			}
			wantProvider, wantGroup := "", ""
			switch tt.user {
			case "alice":
				wantProvider, wantGroup = "home-wg", "家庭-WireGuard"
			case "bob":
				wantProvider, wantGroup = "office-wg", "办公室-WireGuard"
			}
			for _, addon := range addons {
				if _, ok := doc.Providers[addon.ID]; ok != (addon.ID == wantProvider) {
					t.Fatalf("unexpected provider selection: %#v", doc.Providers)
				}
			}
			foundGroup := false
			for _, group := range doc.Groups {
				if len(group.Use) == 0 {
					continue
				}
				if group.Name != wantGroup || !reflect.DeepEqual(group.Use, []string{wantProvider}) {
					t.Fatalf("unexpected provider group: %+v", group)
				}
				foundGroup = true
			}
			if foundGroup != (wantProvider != "") {
				t.Fatalf("provider group missing or leaked: %+v", doc.Groups)
			}
			if strings.Contains(string(raw), "@provider:") {
				t.Fatalf("directive leaked into subscription:\n%s", raw)
			}
			if tt.mode == "" {
				legacy, err := svc.GenerateWithBaseURLAndVariant(tt.user, "", tt.variant)
				if err != nil {
					t.Fatal(err)
				}
				var legacyDoc struct {
					Rules []string `yaml:"rules"`
				}
				if err := yaml.Unmarshal(legacy, &legacyDoc); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(legacyDoc.Rules, tt.want) {
					t.Fatalf("default generation enabled conditional rules: %#v", legacyDoc.Rules)
				}
			}
		})
	}
}

func TestGenerateRejectsUnknownSubscriptionMode(t *testing.T) {
	_, err := (Service{}).GenerateWithOptions("alice", "", SubscriptionOptions{Mode: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unsupported subscription mode") {
		t.Fatalf("expected unsupported mode error, got %v", err)
	}
}
