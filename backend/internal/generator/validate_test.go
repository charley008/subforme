package generator

import (
	"strings"
	"testing"
)

func TestValidateConfigRuleNoResolve(t *testing.T) {
	for _, tt := range []struct {
		name    string
		rule    string
		missing string
	}{
		{name: "IPv4", rule: "IP-CIDR,192.168.1.4/32,home,no-resolve"},
		{name: "IPv6", rule: "IP-CIDR6,fd00::/64,home,no-resolve"},
		{name: "GEOIP", rule: "GEOIP,CN,home,no-resolve"},
		{name: "DIRECT", rule: "IP-CIDR,192.168.1.4/32,DIRECT,no-resolve"},
		{name: "REJECT", rule: "IP-CIDR,192.168.1.4/32,REJECT,no-resolve"},
		{name: "whitespace", rule: "IP-CIDR, 192.168.1.4/32, home, no-resolve "},
		{name: "without option", rule: "IP-CIDR,192.168.1.4/32,home"},
		{name: "ordinary domain rule", rule: "DOMAIN,example.com,home"},
		{name: "option name is group name", rule: "DOMAIN,example.com,no-resolve"},
		{name: "missing group with option", rule: "IP-CIDR,192.168.1.4/32,missing,no-resolve", missing: "missing"},
		{name: "missing group without option", rule: "IP-CIDR,192.168.1.4/32,missing", missing: "missing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := "proxy-groups:\n  - name: home\n    type: select\n    proxies: [DIRECT]\n  - name: no-resolve\n    type: select\n    proxies: [DIRECT]\nrules:\n  - " + tt.rule + "\n"
			// Do not define an option-named group except when explicitly testing it.
			if tt.name != "option name is group name" {
				raw = strings.ReplaceAll(raw, "  - name: no-resolve\n    type: select\n    proxies: [DIRECT]\n", "")
			}
			result := ValidateConfig([]byte(raw))
			if tt.missing == "" {
				if result.HasErrors() {
					t.Fatalf("valid rule produced warnings: %v", result)
				}
				return
			}
			if len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "代理组 \""+tt.missing+"\"") {
				t.Fatalf("expected warning for actual missing target %q, got %v", tt.missing, result)
			}
		})
	}
}
