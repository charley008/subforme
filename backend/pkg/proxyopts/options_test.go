package proxyopts

import "testing"

func TestParseRejectsInvalidOverrides(t *testing.T) {
	for _, raw := range []string{"name: renamed", "- tls: true", "null", "tls: [", "tls: true\ntls: false", "tls: true\n---\nnetwork: ws", "opts:\n  1: value", "opts: &ref {a: 1}\nother: *ref"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Parse(raw); err == nil {
				t.Fatal("expected invalid override to be rejected")
			}
		})
	}
}

func TestMergePreservesNestedValuesAndExplicitFalse(t *testing.T) {
	base := map[string]any{"reality-opts": map[string]any{"public-key": "pk", "short-id": "sid", "support-x25519mlkem768": true}, "alpn": []any{"h2", "http/1.1"}}
	override, err := Parse("reality-opts:\n  support-x25519mlkem768: false\nalpn: []\nfuture-option:\n  enabled: true")
	if err != nil {
		t.Fatal(err)
	}
	Merge(base, override)
	reality := base["reality-opts"].(map[string]any)
	if reality["public-key"] != "pk" || reality["short-id"] != "sid" || reality["support-x25519mlkem768"] != false {
		t.Fatalf("unexpected merge: %#v", base)
	}
	if len(base["alpn"].([]any)) != 0 || base["future-option"] == nil {
		t.Fatalf("array or unknown option lost: %#v", base)
	}
}
