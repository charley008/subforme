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
