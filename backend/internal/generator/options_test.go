package generator

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
	"subforme/backend/internal/xui"
)

func TestSubscriptionAndPreviewUseSameOverrides(t *testing.T) {
	node := xui.Node{Name: "test", Type: "vless", Server: "example.com", Port: 443, UUID: "uuid", TLS: true, RealityPublicKey: "pk", RealityShortID: "sid", MihomoOptions: "reality-opts:\n  support-x25519mlkem768: false\nfuture-key: [one, two]"}
	raw, err := BuildFinalYAML("proxies: []\nproxy-groups: []", []xui.Node{node}, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	preview, err := BuildProxyYAML(node)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(preview, &got); err != nil {
		t.Fatal(err)
	}
	if len(doc.Proxies) != 1 || !reflect.DeepEqual(got, doc.Proxies[0]) {
		t.Fatalf("preview differs: %s / %s", preview, raw)
	}
	reality := got["reality-opts"].(map[string]any)
	if reality["public-key"] != "pk" || reality["short-id"] != "sid" || reality["support-x25519mlkem768"] != false {
		t.Fatalf("unexpected reality options: %#v", reality)
	}
	if got["name"] != "test" || got["future-key"] == nil {
		t.Fatalf("unexpected proxy: %#v", got)
	}
	node.MihomoOptions = "name: broken-reference"
	if _, err := BuildFinalYAML("proxies: []", []xui.Node{node}, nil, nil, nil, ""); err == nil {
		t.Fatal("expected protected name rejection")
	}
}
