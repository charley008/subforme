package app

import (
	"bytes"
	"testing"

	"subforme/backend/internal/config"
	"subforme/backend/internal/generator"
	"subforme/backend/internal/xui"
)

func TestDraftPreviewMatchesManagedNodeAndDoesNotSave(t *testing.T) {
	templates := []xui.Node{{Name: "template", Type: "vless", Network: "raw", UUID: "uuid", RealityPublicKey: "pk", RealityShortID: "sid"}}
	s := Service{ConfigDir: t.TempDir(), Loader: func(string) (config.Bundle, error) { return config.Bundle{}, nil }, ResolverFactory: func(config.XUIConfig) XUIResolver { return fakeResolver{nodes: templates} }}
	draft := config.ManagedNode{ID: "n", Name: "draft", Address: "example.com", Port: 443, Protocol: "vless", Network: "raw", MihomoOptions: "reality-opts:\n  support-x25519mlkem768: false"}
	preview, err := s.PreviewManagedNode("alice", draft)
	if err != nil {
		t.Fatal(err)
	}
	nodes := applyManagedNodes(templates, []config.ManagedNode{draft}, nil)
	expected, err := generator.BuildProxyYAML(nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(preview, expected) {
		t.Fatalf("preview mismatch: %s / %s", preview, expected)
	}
	saved, err := s.ReadManagedNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 0 {
		t.Fatal("preview persisted draft")
	}
	if _, err := s.PreviewManagedNode("", draft); err == nil {
		t.Fatal("expected missing user error")
	}
	draft.MihomoOptions = "name: invalid"
	if _, err := s.PreviewManagedNode("alice", draft); err == nil {
		t.Fatal("expected invalid override error")
	}
}
