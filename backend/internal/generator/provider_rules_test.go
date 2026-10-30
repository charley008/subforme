package generator

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"subforme/backend/internal/config"
)

func TestBuildFinalYAMLProviderRules(t *testing.T) {
	const home = "IP-CIDR,192.168.1.0/24,家庭-WireGuard,no-resolve"
	const office = "IP-CIDR,10.10.0.0/16,办公室-WireGuard,no-resolve"
	const ordinary = "GEOIP,CN,DIRECT"
	template := "rules:\n  # @provider:home-wg\n  - " + home + "\n\n  # @provider:office-wg\n  - " + office + "\n\n  # ordinary comment\n  - " + ordinary + "\n"
	addons := []config.ProviderAddon{
		{ID: "home-wg", Name: "家庭-WireGuard"},
		{ID: "office-wg", Name: "办公室-WireGuard"},
	}
	for _, tt := range []struct {
		name     string
		selected []string
		addons   []config.ProviderAddon
		disabled bool
		want     []string
	}{
		{name: "home only", selected: []string{"home-wg"}, addons: addons, want: []string{home, ordinary}},
		{name: "office only", selected: []string{"office-wg"}, addons: addons, want: []string{office, ordinary}},
		{name: "both", selected: []string{"home-wg", "office-wg"}, addons: addons, want: []string{home, office, ordinary}},
		{name: "none", addons: addons, want: []string{ordinary}},
		{name: "deleted provider", selected: []string{"home-wg"}, want: []string{ordinary}},
		{name: "display name is not ID", selected: []string{"家庭-WireGuard"}, addons: addons, want: []string{ordinary}},
		{name: "default mode skips assigned provider rules", selected: []string{"home-wg", "office-wg"}, addons: addons, disabled: true, want: []string{ordinary}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := BuildFinalYAML(template, nil, nil, tt.addons, tt.selected, "", !tt.disabled)
			if err != nil {
				t.Fatal(err)
			}
			assertProviderRules(t, raw, tt.want)
			if strings.Contains(string(raw), "@provider:") || !strings.Contains(string(raw), "# ordinary comment") {
				t.Fatalf("expected only directive comments removed, got:\n%s", raw)
			}
		})
	}
}

func TestBuildFinalYAMLProviderRuleBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name     string
		template string
		selected []string
		want     []string
	}{
		{name: "single rule only", template: "rules:\n  # @provider:home-wg\n  - MATCH,home-wg\n  - GEOIP,CN,DIRECT\n", want: []string{"GEOIP,CN,DIRECT"}},
		{name: "all removed", template: "rules:\n# @provider:home-wg\n- MATCH,home-wg\n", want: []string{}},
		{name: "CRLF and whitespace", template: "rules:\r\n  # @provider: home-wg  \r\n  - MATCH,home-wg\r\n", selected: []string{"home-wg"}, want: []string{"MATCH,home-wg"}},
		{name: "ordinary rules", template: "rules:\n  # regular comment mentioning @provider:home-wg\n  - DOMAIN,example.com,DIRECT\n  - GEOIP,CN,DIRECT\n", want: []string{"DOMAIN,example.com,DIRECT", "GEOIP,CN,DIRECT"}},
		{name: "other sections unaffected", template: "hosts:\n  # @provider:home-wg\n  example.com: 127.0.0.1\nrules:\n  - MATCH,DIRECT\n", want: []string{"MATCH,DIRECT"}},
		{name: "inline comment unaffected", template: "rules:\n  - MATCH,DIRECT # @provider:home-wg\n", want: []string{"MATCH,DIRECT"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := BuildFinalYAML(tt.template, nil, nil, []config.ProviderAddon{{ID: "home-wg"}}, tt.selected, "", true)
			if err != nil {
				t.Fatal(err)
			}
			assertProviderRules(t, raw, tt.want)
			if tt.name == "other sections unaffected" && !strings.Contains(string(raw), "# @provider:home-wg") {
				t.Fatalf("comment outside rules was changed:\n%s", raw)
			}
		})
	}
}

func assertProviderRules(t *testing.T, raw []byte, want []string) {
	t.Helper()
	var doc struct {
		Rules []string `yaml:"rules"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid generated YAML: %v", err)
	}
	if !reflect.DeepEqual(doc.Rules, want) {
		t.Fatalf("rules = %#v, want %#v; YAML:\n%s", doc.Rules, want, raw)
	}
}
