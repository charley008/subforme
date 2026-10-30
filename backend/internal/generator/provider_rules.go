package generator

import (
	"strings"

	"gopkg.in/yaml.v3"

	"subforme/backend/internal/config"
	"subforme/backend/pkg/yamlx"
)

// filterProviderRules applies standalone @provider comments to the following
// rule only. Use the same resolved providers that supply groups and providers.
func filterProviderRules(doc *yaml.Node, selected map[string]config.ProviderAddon) {
	rules, _, err := yamlx.GetMappingValue(doc, "rules")
	if err != nil || rules == nil || rules.Kind != yaml.SequenceNode {
		return
	}
	filtered := rules.Content[:0]
	for _, rule := range rules.Content {
		if rule.Kind != yaml.ScalarNode || rule.Tag != "!!str" {
			filtered = append(filtered, rule)
			continue
		}
		keep := true
		comments := strings.Split(rule.HeadComment, "\n")
		remaining := make([]string, 0, len(comments))
		for _, comment := range comments {
			text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment), "#"))
			if !strings.HasPrefix(text, "@provider:") {
				remaining = append(remaining, comment)
				continue
			}
			id := strings.TrimSpace(strings.TrimPrefix(text, "@provider:"))
			if _, ok := selected[id]; !ok {
				keep = false
			}
		}
		if keep {
			rule.HeadComment = strings.Join(remaining, "\n")
			filtered = append(filtered, rule)
		}
	}
	rules.Content = filtered
}
