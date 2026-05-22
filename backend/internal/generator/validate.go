package generator

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type ValidationError struct {
	Errors []string
}

func (e ValidationError) Error() string {
	return strings.Join(e.Errors, "; ")
}

func (e ValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

func ValidateConfig(raw []byte) ValidationError {
	var result ValidationError

	doc := map[string]any{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("YAML 语法错误: %v", err))
		return result
	}

	proxies := collectNames(doc, "proxies")
	proxyGroups := collectNames(doc, "proxy-groups")
	proxyProviders := collectKeys(doc, "proxy-providers")
	ruleProviders := collectKeys(doc, "rule-providers")
	rules := collectRules(doc)

	allGroupNames := map[string]bool{}
	for _, name := range proxyGroups {
		allGroupNames[name] = true
	}

	allProxyNames := map[string]bool{}
	for _, name := range proxies {
		allProxyNames[name] = true
	}

	if groupsRaw, ok := doc["proxy-groups"]; ok {
		if groupsList, ok := groupsRaw.([]any); ok {
			for _, gRaw := range groupsList {
				if g, ok := gRaw.(map[string]any); ok {
					name, _ := g["name"].(string)

					if proxiesList, ok := g["proxies"].([]any); ok {
						for _, p := range proxiesList {
							pStr, _ := p.(string)
							if pStr == "" {
								continue
							}
							if !allProxyNames[pStr] && !allGroupNames[pStr] && pStr != "DIRECT" && pStr != "REJECT" && pStr != "PASS" {
								result.Errors = append(result.Errors,
									fmt.Sprintf("proxy-group %q 引用了不存在的节点或分组 %q", name, pStr))
							}
						}
					}

					if useList, ok := g["use"].([]any); ok {
						for _, u := range useList {
							uStr, _ := u.(string)
							if uStr != "" && !proxyProviders[uStr] {
								result.Errors = append(result.Errors,
									fmt.Sprintf("proxy-group %q 的 use 引用了不存在的 proxy-provider %q", name, uStr))
							}
						}
					}
				}
			}
		}
	}

	for _, rule := range rules {
		parts := splitRule(rule)
		if len(parts) < 3 {
			continue
		}
		target := parts[len(parts)-1]
		if target == "DIRECT" || target == "REJECT" || target == "PASS" {
			continue
		}
		if strings.HasPrefix(target, "rule-set:") {
			rsName := strings.TrimPrefix(target, "rule-set:")
			if !ruleProviders[rsName] {
				result.Errors = append(result.Errors,
					fmt.Sprintf("规则 %q 引用了不存在的 rule-provider %q", rule, rsName))
			}
			continue
		}
		if !allGroupNames[target] {
			result.Errors = append(result.Errors,
				fmt.Sprintf("规则 %q 引用了不存在的代理组 %q", rule, target))
		}
	}

	return result
}

func collectNames(doc map[string]any, key string) []string {
	var out []string
	if raw, ok := doc[key]; ok {
		if list, ok := raw.([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					if name, ok := m["name"].(string); ok {
						out = append(out, name)
					}
				}
			}
		}
	}
	return out
}

func collectKeys(doc map[string]any, key string) map[string]bool {
	out := map[string]bool{}
	if raw, ok := doc[key]; ok {
		if m, ok := raw.(map[string]any); ok {
			for k := range m {
				out[k] = true
			}
		}
	}
	return out
}

func collectRules(doc map[string]any) []string {
	if raw, ok := doc["rules"]; ok {
		if list, ok := raw.([]any); ok {
			out := make([]string, 0, len(list))
			for _, item := range list {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

func splitRule(rule string) []string {
	var parts []string
	start := 0
	inQuotes := false
	for i := 0; i < len(rule); i++ {
		if rule[i] == '"' || rule[i] == '\'' {
			inQuotes = !inQuotes
		}
		if rule[i] == ',' && !inQuotes {
			parts = append(parts, strings.TrimSpace(rule[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, strings.TrimSpace(rule[start:]))
	return parts
}
