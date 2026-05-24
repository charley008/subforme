package generator

import (
	"strings"
	"testing"

	"subforme/backend/internal/config"
	"subforme/backend/internal/groups"
	"subforme/backend/internal/xui"
)

func TestBuildFinalYAMLInjectsProxiesAndGroupsWithoutReorderingTemplate(t *testing.T) {
	template := `mixed-port: 10801
allow-lan: true
mode: rule
proxies: []
proxy-groups: []
rules:
  - MATCH,DIRECT
`

	nodes := []xui.Node{
		{Name: "HK-01", Type: "vless", Server: "hk.example.com", Port: 443, UUID: "u-1", TLS: true, RealityPublicKey: "pk-1", RealityShortID: "short-1"},
	}

	groupList := []groups.ProxyGroup{
		{Name: "节点选择", Type: "select", Proxies: []string{"HK-01"}},
	}

	raw, err := BuildFinalYAML(template, nodes, groupList, nil, nil, "节点选择")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	got := string(raw)
	if !strings.Contains(got, "name: HK-01") {
		t.Fatalf("expected proxy entry, got %s", got)
	}
	if !strings.Contains(got, "name: 节点选择") {
		t.Fatalf("expected group entry, got %s", got)
	}
	if !strings.Contains(got, "mixed-port: 10801\nallow-lan: true\nmode: rule") {
		t.Fatalf("expected original key order to be preserved, got %s", got)
	}
}

func TestBuildFinalYAMLAddsDefaultProviderGroup(t *testing.T) {
	template := `proxies: []
proxy-groups: []
proxy-providers: {}
rules:
  - MATCH,DIRECT
`
	groupList := []groups.ProxyGroup{
		{Name: "PROXY", Type: "select", Proxies: []string{"HK-01"}},
	}
	addons := []config.ProviderAddon{
		{
			ID:   "provider-main",
			Name: "provider-main",
			ProxyProviders: map[string]any{
				"provider-main": map[string]any{
					"type": "http",
					"url":  "https://sub.example.com/api/proxy-providers/provider-main.yaml",
				},
			},
		},
	}

	raw, err := BuildFinalYAML(template, nil, groupList, addons, []string{"provider-main"}, "PROXY")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	got := string(raw)
	if !strings.Contains(got, "name: provider-main") || !strings.Contains(got, "type: url-test") || !strings.Contains(got, "use:\n      - provider-main") {
		t.Fatalf("expected default provider group, got %s", got)
	}
	if !strings.Contains(got, "proxy-providers:\n  provider-main:") {
		t.Fatalf("expected proxy provider entry, got %s", got)
	}
}

func TestBuildFinalYAMLRemovesUnselectedProviderRefs(t *testing.T) {
	template := `proxies: []
proxy-groups: []
proxy-providers: {}
rules:
  - MATCH,DIRECT
`
	groupList := []groups.ProxyGroup{
		{Name: "PROXY", Type: "select", Proxies: []string{"los", "provider-main", "test"}},
	}
	addons := []config.ProviderAddon{
		{ID: "provider-main", Name: "provider-main", ProxyProviders: map[string]any{"provider-main": map[string]any{"type": "http"}}},
		{ID: "test", Name: "test", ProxyProviders: map[string]any{"test": map[string]any{"type": "http"}}},
	}

	raw, err := BuildFinalYAML(template, nil, groupList, addons, []string{"test"}, "PROXY")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	got := string(raw)
	if strings.Contains(got, "- provider-main") {
		t.Fatalf("expected unselected provider ref to be removed, got %s", got)
	}
	if !strings.Contains(got, "- test") {
		t.Fatalf("expected selected provider ref to remain, got %s", got)
	}
	if !strings.Contains(got, "proxy-providers:\n  test:") || strings.Contains(got, "proxy-providers:\n  provider-main:") {
		t.Fatalf("expected only selected provider entry, got %s", got)
	}
}

func TestBuildFinalYAMLRemovesDeletedProviderRefs(t *testing.T) {
	template := `proxies: []
proxy-groups: []
proxy-providers: {}
rules:
  - MATCH,DIRECT
`
	groupList := []groups.ProxyGroup{
		{Name: "PROXY", Type: "select", Proxies: []string{"los", "test", "111"}},
	}
	addons := []config.ProviderAddon{
		{ID: "111", Name: "111", ProxyProviders: map[string]any{"111": map[string]any{"type": "http"}}},
	}

	raw, err := BuildFinalYAML(template, nil, groupList, addons, []string{"111"}, "PROXY")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	got := string(raw)
	if strings.Contains(got, "- test") {
		t.Fatalf("expected deleted provider ref to be removed, got %s", got)
	}
	if !strings.Contains(got, "- \"111\"") && !strings.Contains(got, "- 111") {
		t.Fatalf("expected selected provider ref to remain, got %s", got)
	}
}
