package generator

import (
	"strings"
	"testing"

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
