package config

import (
	"fmt"
	"os"
	"path/filepath"

	"subforme/backend/pkg/yamlx"

	"gopkg.in/yaml.v3"
)

var templateKeyOrder = []string{
	"mixed-port",
	"port",
	"socks-port",
	"redir-port",
	"tproxy-port",
	"allow-lan",
	"bind-address",
	"authentication",
	"skip-auth-prefixes",
	"lan-allowed-ips",
	"mode",
	"log-level",
	"external-controller",
	"external-ui",
	"external-ui-name",
	"external-ui-url",
	"secret",
	"ipv6",
	"sniffer",
	"dns",
	"tun",
	"proxies",
	"proxy-groups",
	"proxy-providers",
	"rule-providers",
	"rules",
}

var modeSectionKeys = []string{"proxies", "proxy-groups", "rule-providers", "rules"}

func ensureTemplateLayout(dir string) error {
	templatesDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		return err
	}

	basePath := filepath.Join(templatesDir, "base.yaml")
	whitelistPath := filepath.Join(templatesDir, "whitelist.yaml")
	blacklistPath := filepath.Join(templatesDir, "blacklist.yaml")

	baseExists := fileExists(basePath)
	whiteExists := fileExists(whitelistPath)
	blackExists := fileExists(blacklistPath)

	if !baseExists || !whiteExists || !blackExists {
		whiteRaw, err := readYAMLText(filepath.Join(dir, "base", "whitelist.yaml"))
		if err != nil {
			return err
		}
		blackRaw, err := readYAMLText(filepath.Join(dir, "base", "blacklist.yaml"))
		if err != nil {
			return err
		}
		baseRaw, whiteSectionRaw, err := splitTemplateSections(whiteRaw)
		if err != nil {
			return err
		}
		_, blackSectionRaw, err := splitTemplateSections(blackRaw)
		if err != nil {
			return err
		}
		if !baseExists {
			if err := os.WriteFile(basePath, []byte(baseRaw), 0o644); err != nil {
				return err
			}
		}
		if !whiteExists {
			if err := os.WriteFile(whitelistPath, []byte(whiteSectionRaw), 0o644); err != nil {
				return err
			}
		}
		if !blackExists {
			if err := os.WriteFile(blacklistPath, []byte(blackSectionRaw), 0o644); err != nil {
				return err
			}
		}
	}

	for _, mode := range []string{"whitelist", "blacklist"} {
		baseRaw, err := readYAMLText(basePath)
		if err != nil {
			return err
		}
		modeRaw, err := readYAMLText(filepath.Join(templatesDir, mode+".yaml"))
		if err != nil {
			return err
		}
		raw, err := mergeTemplateSections(baseRaw, modeRaw)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "base", mode+".yaml"), []byte(raw), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func LoadTemplateSectionYAML(dir, section string) (string, error) {
	if err := ensureTemplateLayout(dir); err != nil {
		return "", err
	}
	path, err := templateSectionPath(dir, section)
	if err != nil {
		return "", err
	}
	return readYAMLText(path)
}

func SaveTemplateSectionYAML(dir, section, raw string) error {
	if err := ensureTemplateLayout(dir); err != nil {
		return err
	}
	if _, err := yamlx.Parse(raw); err != nil {
		return fmt.Errorf("解析 %s.yaml 失败: %w", section, err)
	}
	path, err := templateSectionPath(dir, section)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		return err
	}
	return ensureTemplateLayout(dir)
}

func LoadModeTemplateYAML(dir, mode string) (string, error) {
	if err := ensureTemplateLayout(dir); err != nil {
		return "", err
	}
	baseRaw, err := readYAMLText(filepath.Join(dir, "templates", "base.yaml"))
	if err != nil {
		return "", err
	}
	modeRaw, err := readYAMLText(filepath.Join(dir, "templates", mode+".yaml"))
	if err != nil {
		return "", err
	}
	return mergeTemplateSections(baseRaw, modeRaw)
}

func LoadManagedNodes(dir string) ([]ManagedNode, error) {
	path := filepath.Join(dir, "nodes.yaml")
	if !fileExists(path) {
		return []ManagedNode{}, nil
	}
	var nodes []ManagedNode
	if err := readYAML(path, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func SaveManagedNodes(dir string, nodes []ManagedNode) error {
	for i := range nodes {
		nodes[i].ID = fmt.Sprintf("%d", i+1)
	}
	return writeYAML(filepath.Join(dir, "nodes.yaml"), nodes)
}

func mergeTemplateSections(baseRaw, modeRaw string) (string, error) {
	baseDoc, err := yamlx.Parse(baseRaw)
	if err != nil {
		return "", err
	}
	modeDoc, err := yamlx.Parse(modeRaw)
	if err != nil {
		return "", err
	}

	baseRoot, err := rootMapping(baseDoc)
	if err != nil {
		return "", err
	}
	modeRoot, err := rootMapping(modeDoc)
	if err != nil {
		return "", err
	}

	mergedRoot := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	used := map[string]bool{}

	for _, key := range templateKeyOrder {
		if value, ok := mappingValue(modeRoot, key); ok {
			appendPair(mergedRoot, key, value)
			used[key] = true
			continue
		}
		if value, ok := mappingValue(baseRoot, key); ok {
			appendPair(mergedRoot, key, value)
			used[key] = true
		}
	}

	appendUnknownKeys(mergedRoot, baseRoot, used)
	appendUnknownKeys(mergedRoot, modeRoot, used)

	return stringMust(yamlx.Marshal(mergedRoot))
}

func splitTemplateSections(raw string) (string, string, error) {
	baseDoc, err := yamlx.Parse(raw)
	if err != nil {
		return "", "", err
	}
	modeDoc, err := yamlx.Parse(raw)
	if err != nil {
		return "", "", err
	}

	for _, key := range modeSectionKeys {
		_ = yamlx.DeleteMappingValue(baseDoc, key)
	}

	baseRaw, err := yamlx.Marshal(baseDoc)
	if err != nil {
		return "", "", err
	}

	modeRoot, err := rootMapping(modeDoc)
	if err != nil {
		return "", "", err
	}
	modeOnly := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, key := range modeSectionKeys {
		if value, ok := mappingValue(modeRoot, key); ok {
			appendPair(modeOnly, key, value)
			continue
		}
		switch key {
		case "rule-providers":
			appendPair(modeOnly, key, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
		default:
			appendPair(modeOnly, key, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
		}
	}
	modeRaw, err := yamlx.Marshal(modeOnly)
	if err != nil {
		return "", "", err
	}
	return string(baseRaw), string(modeRaw), nil
}

func templateSectionPath(dir, section string) (string, error) {
	switch section {
	case "base", "whitelist", "blacklist":
		return filepath.Join(dir, "templates", section+".yaml"), nil
	default:
		return "", fmt.Errorf("unsupported template section %q", section)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func rootMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil {
		return nil, fmt.Errorf("yaml document is nil")
	}
	root := doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil, fmt.Errorf("yaml document is empty")
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("yaml root is not a mapping")
	}
	return root, nil
}

func mappingValue(root *yaml.Node, key string) (*yaml.Node, bool) {
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1], true
		}
	}
	return nil, false
}

func appendPair(root *yaml.Node, key string, value *yaml.Node) {
	root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, cloneYAMLNode(value))
}

func appendUnknownKeys(target, source *yaml.Node, used map[string]bool) {
	for i := 0; i < len(source.Content)-1; i += 2 {
		key := source.Content[i].Value
		if used[key] {
			continue
		}
		appendPair(target, key, source.Content[i+1])
		used[key] = true
	}
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	copyNode := *node
	if len(node.Content) > 0 {
		copyNode.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			copyNode.Content[i] = cloneYAMLNode(child)
		}
	}
	return &copyNode
}

func stringMust(raw []byte, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
