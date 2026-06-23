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

func TestBuildFinalYAMLMakesXHTTPNodesMihomoCompatible(t *testing.T) {
	template := `proxies: []
proxy-groups: []
rules:
  - MATCH,DIRECT
`

	nodes := []xui.Node{
		{
			Name:      "hk",
			Type:      "vless",
			Server:    "hk.4738.org",
			Port:      443,
			UUID:      "036898f6-8f62-46b2-9924-c1b884d6e75d",
			Network:   "xhttp",
			Flow:      "xtls-rprx-vision",
			UDP:       true,
			XHTTPPath: "/xhttp",
			XHTTPMode: "auto",
		},
	}

	raw, err := BuildFinalYAML(template, nodes, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	got := string(raw)
	for _, want := range []string{
		"name: hk",
		"network: xhttp",
		"tls: true",
		"client-fingerprint: chrome",
		"xhttp-opts:",
		"path: /xhttp",
		"mode: stream-one",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in yaml, got %s", want, got)
		}
	}
	if strings.Contains(got, "flow:") {
		t.Fatalf("xhttp node should not emit flow, got %s", got)
	}
	if strings.Contains(got, "reality-opts:") {
		t.Fatalf("xhttp node should not emit reality opts, got %s", got)
	}
}

func TestBuildFinalYAMLDoesNotEmitXHTTPOptsForNonXHTTPNodes(t *testing.T) {
	template := `proxies: []
proxy-groups: []
rules:
  - MATCH,DIRECT
`

	nodes := []xui.Node{
		{
			Name:             "hk",
			Type:             "vless",
			Server:           "hk.4738.org",
			Port:             443,
			UUID:             "036898f6-8f62-46b2-9924-c1b884d6e75d",
			Network:          "raw",
			TLS:              true,
			RealityPublicKey: "pk-1",
			RealityShortID:   "sid-1",
		},
	}

	raw, err := BuildFinalYAML(template, nodes, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	got := string(raw)
	if strings.Contains(got, "xhttp-opts:") {
		t.Fatalf("non-xhttp node should not contain xhttp opts, got %s", got)
	}
	if !strings.Contains(got, "reality-opts:") {
		t.Fatalf("expected reality opts to remain for reality node, got %s", got)
	}
}

func TestBuildFinalYAMLUsesProtocolCredentials(t *testing.T) {
	template := `proxies: []
proxy-groups: []
rules:
  - MATCH,DIRECT
`

	nodes := []xui.Node{
		{Name: "vmess", Type: "vmess", Server: "vmess.example.com", Port: 443, UUID: "uuid-1", Security: "chacha20-poly1305"},
		{Name: "trojan", Type: "trojan", Server: "trojan.example.com", Port: 443, Password: "pass-1"},
		{Name: "ss", Type: "shadowsocks", Server: "ss.example.com", Port: 8388, Password: "pass-2"},
	}

	raw, err := BuildFinalYAML(template, nodes, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	got := string(raw)
	for _, want := range []string{
		"type: vmess",
		"uuid: uuid-1",
		"cipher: chacha20-poly1305",
		"type: trojan",
		"password: pass-1",
		"type: shadowsocks",
		"password: pass-2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in yaml, got %s", want, got)
		}
	}
}

func TestBuildFinalYAMLEmitsFlow(t *testing.T) {
	template := `proxies: []
proxy-groups: []
rules:
  - MATCH,DIRECT
`

	nodes := []xui.Node{
		{Name: "vision", Type: "vless", Server: "vision.example.com", Port: 443, UUID: "uuid-1", Network: "raw", TLS: true, Flow: "xtls-rprx-vision"},
	}

	raw, err := BuildFinalYAML(template, nodes, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	if got := string(raw); !strings.Contains(got, "flow: xtls-rprx-vision") {
		t.Fatalf("expected flow in yaml, got %s", got)
	}
}

func TestBuildFinalYAMLCompactsBlankLinesBeforeComments(t *testing.T) {
	raw := "rules:\n  - MATCH,DIRECT\n\n# comment\nproxies: []\n"
	got := compactCommentSpacing(raw)
	if strings.Contains(got, "\n\n# comment") {
		t.Fatalf("expected blank line before comment to be compacted, got %s", got)
	}
	if !strings.Contains(got, "\n# comment") {
		t.Fatalf("expected comment to be preserved, got %s", got)
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
	if strings.Contains(got, "proxies:\n      - provider-main") {
		t.Fatalf("provider should not be auto-attached to PROXY, got %s", got)
	}
}

func TestBuildFinalYAMLDoesNotDuplicateExistingProviderGroup(t *testing.T) {
	template := `proxies: []
proxy-groups: []
proxy-providers: {}
rules:
  - MATCH,DIRECT
`
	groupList := []groups.ProxyGroup{
		{Name: "PROXY", Type: "select", Proxies: []string{"node-a"}},
		{Name: "airport", Type: "url-test", URL: "https://www.gstatic.com/generate_204", Interval: 300, Use: []string{"airport"}},
	}
	addons := []config.ProviderAddon{
		{ID: "airport", Name: "airport", ProxyProviders: map[string]any{"airport": map[string]any{"type": "http"}}},
	}

	raw, err := BuildFinalYAML(template, nil, groupList, addons, []string{"airport"}, "PROXY")
	if err != nil {
		t.Fatalf("BuildFinalYAML returned error: %v", err)
	}

	got := string(raw)
	if strings.Count(got, "name: airport") != 1 {
		t.Fatalf("expected one provider group, got %s", got)
	}
	if strings.Count(got, "use:\n      - airport") != 1 {
		t.Fatalf("expected provider use to remain, got %s", got)
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
