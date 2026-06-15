package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveTemplateSectionYAMLNormalizesLineEndings(t *testing.T) {
	dir := t.TempDir()
	raw := "proxies: []\r\nproxy-groups: []\r\nrule-providers:\r\n  direct:\r\n    type: http\r\n#  proxy:\r\n#    type: http\r\nrules:\r\n  - MATCH,DIRECT\r\n"

	if err := SaveTemplateSectionYAML(dir, "blacklist", raw); err != nil {
		t.Fatalf("SaveTemplateSectionYAML returned error: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(dir, "templates", "blacklist.yaml"))
	if err != nil {
		t.Fatalf("read saved template: %v", err)
	}
	if strings.Contains(string(saved), "\r") {
		t.Fatalf("expected saved template to use LF only, got %q", string(saved))
	}
	if !strings.Contains(string(saved), "#  proxy:\n#    type: http\n") {
		t.Fatalf("expected comments to be preserved, got %q", string(saved))
	}
}

func TestLoadTemplateSectionYAMLSupportsIOS(t *testing.T) {
	dir := t.TempDir()
	if err := SaveTemplateSectionYAML(dir, "whitelist", "proxies: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n"); err != nil {
		t.Fatalf("SaveTemplateSectionYAML returned error: %v", err)
	}

	raw, err := LoadTemplateSectionYAML(dir, "ios")
	if err != nil {
		t.Fatalf("LoadTemplateSectionYAML returned error: %v", err)
	}
	if raw == "" {
		t.Fatal("expected ios template content")
	}
}

func TestLoadModeTemplateYAMLPreservesBaseTemplateOrder(t *testing.T) {
	dir := t.TempDir()

	base := `mixed-port: 10801
allow-lan: true
bind-address: "*"
unified-delay: true
mode: rule
geox-url:
  geoip: https://example.com/geoip.dat
geo-auto-update: false
geo-update-interval: 24
log-level: info
`
	mode := `proxies: []
proxy-groups: []
rule-providers: {}
rules:
  - MATCH,DIRECT
`

	if err := SaveTemplateSectionYAML(dir, "base", base); err != nil {
		t.Fatalf("SaveTemplateSectionYAML(base) returned error: %v", err)
	}
	if err := SaveTemplateSectionYAML(dir, "whitelist", mode); err != nil {
		t.Fatalf("SaveTemplateSectionYAML(whitelist) returned error: %v", err)
	}

	raw, err := LoadModeTemplateYAML(dir, "whitelist")
	if err != nil {
		t.Fatalf("LoadModeTemplateYAML returned error: %v", err)
	}

	order := []string{
		"bind-address:",
		"unified-delay:",
		"mode:",
		"geox-url:",
		"geo-auto-update:",
		"geo-update-interval:",
		"log-level:",
		"proxies:",
	}

	last := -1
	for _, marker := range order {
		idx := strings.Index(raw, marker)
		if idx < 0 {
			t.Fatalf("expected %q in merged template, got %q", marker, raw)
		}
		if idx <= last {
			t.Fatalf("expected %q after previous marker, got %q", marker, raw)
		}
		last = idx
	}
}

func TestLoadModeTemplateYAMLPreservesCustomInsertedKeyPosition(t *testing.T) {
	dir := t.TempDir()

	base := `mixed-port: 10801
allow-lan: true
bind-address: "*"
custom-before-mode: keep-me-here
mode: rule
log-level: info
`
	mode := `proxies: []
proxy-groups: []
rule-providers: {}
rules:
  - MATCH,DIRECT
`

	if err := SaveTemplateSectionYAML(dir, "base", base); err != nil {
		t.Fatalf("SaveTemplateSectionYAML(base) returned error: %v", err)
	}
	if err := SaveTemplateSectionYAML(dir, "whitelist", mode); err != nil {
		t.Fatalf("SaveTemplateSectionYAML(whitelist) returned error: %v", err)
	}

	raw, err := LoadModeTemplateYAML(dir, "whitelist")
	if err != nil {
		t.Fatalf("LoadModeTemplateYAML returned error: %v", err)
	}

	beforeMode := strings.Index(raw, "custom-before-mode:")
	modeIdx := strings.Index(raw, "mode:")
	logLevelIdx := strings.Index(raw, "log-level:")
	proxiesIdx := strings.Index(raw, "proxies:")
	if beforeMode < 0 || modeIdx < 0 || logLevelIdx < 0 || proxiesIdx < 0 {
		t.Fatalf("expected merged keys in output, got %q", raw)
	}
	if !(beforeMode < modeIdx && modeIdx < logLevelIdx && logLevelIdx < proxiesIdx) {
		t.Fatalf("expected custom key order to be preserved, got %q", raw)
	}
}
