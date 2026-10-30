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
	const home = "IP-CIDR,192.168.1.4/32,家庭-WireGuard,no-resolve"
	const office = "IP-CIDR,192.168.1.4/32,办公室-WireGuard,no-resolve"
	const ordinary = "GEOIP,CN,DIRECT"
	template := "rules:\n  # @provider:home\n  - " + home + "\n\n  # @provider:office\n  - " + office + "\n\n  - " + ordinary + "\n"
	for _, section := range []string{"whitelist", "blacklist", "ios"} {
		if err := config.SaveTemplateSectionYAML(dir, section, template); err != nil {
			t.Fatal(err)
		}
	}
	addons := []config.ProviderAddon{
		{ID: "home", ProxyProviders: map[string]any{"home": map[string]any{"type": "http"}}, ProxyGroups: []map[string]any{{"name": "家庭-WireGuard", "type": "select", "use": []string{"home"}}}},
		{ID: "office", ProxyProviders: map[string]any{"office": map[string]any{"type": "http"}}, ProxyGroups: []map[string]any{{"name": "办公室-WireGuard", "type": "select", "use": []string{"office"}}}},
	}
	if err := config.SaveProviders(dir, addons); err != nil {
		t.Fatal(err)
	}
	svc := NewService(dir, config.XUIConfig{})
	assignments := map[string][]string{"alice": {"home"}, "bob": {"office"}, "both": {"home", "office"}}
	svc.Loader = func(string) (config.Bundle, error) {
		return config.Bundle{App: config.AppConfig{
			UserProviders: assignments,
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
		{user: "alice", mode: "home", want: []string{home, ordinary}},
		{user: "bob", mode: "office", want: []string{office, ordinary}},
		{user: "charlie", mode: "home", want: []string{ordinary}},
		{user: "alice", mode: "office", want: []string{ordinary}},
		{user: "alice", mode: " home ", want: []string{home, ordinary}},
		{user: "both", want: []string{ordinary}},
		{user: "both", mode: "home", want: []string{home, ordinary}},
		{user: "both", mode: "office", want: []string{office, ordinary}},
		{user: "both", mode: "unknown", want: []string{ordinary}},
		{user: "both", mode: "wg", want: []string{ordinary}},
		{user: "both", mode: "HOME", want: []string{ordinary}},
		{user: "both", mode: "家庭-WireGuard", want: []string{ordinary}},
		{user: "alice", variant: "ios", want: []string{ordinary}},
		{user: "charlie", variant: "ios", want: []string{ordinary}},
		{user: "both", variant: "ios", mode: "home", want: []string{home, ordinary}},
		{user: "both", variant: "ios", mode: "office", want: []string{office, ordinary}},
		{user: "charlie", variant: "ios", mode: "home", want: []string{ordinary}},
	} {
		t.Run(tt.user+"/"+tt.variant+"/"+tt.mode, func(t *testing.T) {
			raw, err := svc.GenerateWithOptions(tt.user, "", SubscriptionOptions{Variant: tt.variant, Mode: tt.mode})
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(string(raw), "# 配置警告") {
				t.Fatalf("valid subscription produced warnings:\n%s", raw)
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
			wantProviders := map[string]bool{}
			for _, id := range assignments[tt.user] {
				wantProviders[id] = true
			}
			for _, addon := range addons {
				if _, ok := doc.Providers[addon.ID]; ok != wantProviders[addon.ID] {
					t.Fatalf("unexpected provider selection: %#v", doc.Providers)
				}
			}
			foundGroups := map[string]bool{}
			for _, group := range doc.Groups {
				if len(group.Use) == 0 {
					continue
				}
				expectedNames := map[string]string{"home": "家庭-WireGuard", "office": "办公室-WireGuard"}
				if len(group.Use) != 1 || !wantProviders[group.Use[0]] || group.Name != expectedNames[group.Use[0]] {
					t.Fatalf("unexpected provider group: %+v", group)
				}
				foundGroups[group.Use[0]] = true
			}
			if !reflect.DeepEqual(foundGroups, wantProviders) {
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
