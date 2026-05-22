package yamlx

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	target := normalizeDocumentValue(v)
	if err := enc.Encode(target); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func normalizeDocumentValue(v any) any {
	switch node := v.(type) {
	case *yaml.Node:
		if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
			return node.Content[0]
		}
	case yaml.Node:
		if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
			return node.Content[0]
		}
	}
	return v
}

func Parse(raw string) (*yaml.Node, error) {
	var node yaml.Node
	dec := yaml.NewDecoder(strings.NewReader(raw))
	if err := dec.Decode(&node); err != nil {
		return nil, err
	}
	return &node, nil
}

func ToNode(v any) (*yaml.Node, error) {
	raw, err := Marshal(v)
	if err != nil {
		return nil, err
	}
	return Parse(string(raw))
}

func SetMappingValue(doc *yaml.Node, key string, value *yaml.Node) error {
	root, err := mappingNode(doc)
	if err != nil {
		return err
	}
	value = normalizeNode(value)
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == key {
			root.Content[i+1] = cloneNode(value)
			return nil
		}
	}
	root.Content = append(root.Content, scalarNode(key), cloneNode(value))
	return nil
}

func DeleteMappingValue(doc *yaml.Node, key string) error {
	root, err := mappingNode(doc)
	if err != nil {
		return err
	}
	next := make([]*yaml.Node, 0, len(root.Content))
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == key {
			continue
		}
		next = append(next, root.Content[i], root.Content[i+1])
	}
	root.Content = next
	return nil
}

func GetMappingValue(doc *yaml.Node, key string) (*yaml.Node, bool, error) {
	root, err := mappingNode(doc)
	if err != nil {
		return nil, false, err
	}
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1], true, nil
		}
	}
	return nil, false, nil
}

func mappingNode(doc *yaml.Node) (*yaml.Node, error) {
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

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	copyNode := *node
	if len(node.Content) > 0 {
		copyNode.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			copyNode.Content[i] = cloneNode(child)
		}
	}
	return &copyNode
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func normalizeNode(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}
