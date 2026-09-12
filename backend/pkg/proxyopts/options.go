package proxyopts

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse accepts a single mapping of Mihomo proxy options, including future keys.
func Parse(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	if len(raw) > 65536 {
		return nil, fmt.Errorf("高级配置不能超过 64 KiB")
	}
	d := yaml.NewDecoder(strings.NewReader(raw))
	var doc yaml.Node
	if err := d.Decode(&doc); err != nil {
		return nil, fmt.Errorf("高级配置 YAML 无效: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("高级配置必须是 YAML 对象，不能是列表或纯文本")
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("高级配置只能包含一个 YAML 文档")
	}
	if err := validate(doc.Content[0]); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := doc.Decode(&out); err != nil {
		return nil, fmt.Errorf("高级配置 YAML 无效: %w", err)
	}
	if _, ok := out["name"]; ok {
		return nil, fmt.Errorf("高级配置不能覆盖 name，请使用节点名称输入框")
	}
	return out, nil
}

func validate(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode {
		return fmt.Errorf("高级配置不支持 YAML 别名，请直接填写参数")
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			if n.Content[i].Tag != "!!str" {
				return fmt.Errorf("高级配置的参数名必须是字符串")
			}
		}
	}
	for _, c := range n.Content {
		if err := validate(c); err != nil {
			return err
		}
	}
	return nil
}

// Merge preserves unspecified nested keys and replaces arrays and scalar values.
func Merge(base, override map[string]any) {
	for key, value := range override {
		if nested, ok := value.(map[string]any); ok {
			if current, ok := base[key].(map[string]any); ok {
				Merge(current, nested)
				continue
			}
		}
		base[key] = value
	}
}
