package generator

import (
	"subforme/backend/internal/config"
	"subforme/backend/internal/groups"
	"subforme/backend/internal/xui"
	"subforme/backend/pkg/yamlx"

	"gopkg.in/yaml.v3"
)

func BuildFinalYAML(templateRaw string, nodes []xui.Node, groupList []groups.ProxyGroup, addons []config.ProviderAddon, selectedProviders []string, mainGroupName string) ([]byte, error) {
	doc, err := yamlx.Parse(templateRaw)
	if err != nil {
		return nil, err
	}

	proxies := make([]Proxy, 0, len(nodes))
	for _, node := range nodes {
		var encryption *string
		if node.Network == "xhttp" {
			empty := ""
			encryption = &empty
		}
		alpn := node.ALPN
		if node.Network == "xhttp" && len(alpn) == 0 {
			alpn = []string{"h2"}
		}
		proxies = append(proxies, Proxy{
			Name:              node.Name,
			Type:              node.Type,
			Server:            node.Server,
			Port:              node.Port,
			UUID:              node.UUID,
			Flow:              node.Flow,
			Network:           node.Network,
			TLS:               node.TLS,
			UDP:               node.UDP,
			ALPN:              alpn,
			ServerName:        node.ServerName,
			ClientFingerprint: node.ClientFingerprint,
			Encryption:        encryption,
			RealityOpts: RealityOpts{
				PublicKey: node.RealityPublicKey,
				ShortID:   node.RealityShortID,
			},
			XHTTPOpts: XHTTPOpts{
				Path: node.XHTTPPath,
				Host: node.XHTTPHost,
				Mode: node.XHTTPMode,
			},
		})
	}

	selectedSet := map[string]config.ProviderAddon{}
	for _, id := range selectedProviders {
		for _, addon := range addons {
			if addon.ID == id {
				selectedSet[id] = addon
			}
		}
	}

	groupMaps := convertGroups(groupList)
	for _, addon := range selectedSet {
		attach := addon.AttachToGroup
		if attach == "" {
			attach = mainGroupName
		}
		groupMaps = appendProviderGroupEntry(groupMaps, attach, addon.Name)
		groupMaps = append(groupMaps, addon.ProxyGroups...)
	}

	proxyNode, err := yamlx.ToNode(proxies)
	if err != nil {
		return nil, err
	}
	groupNode, err := yamlx.ToNode(groupMaps)
	if err != nil {
		return nil, err
	}

	if err := yamlx.SetMappingValue(doc, "proxies", proxyNode); err != nil {
		return nil, err
	}
	if err := yamlx.SetMappingValue(doc, "proxy-groups", groupNode); err != nil {
		return nil, err
	}

	if len(selectedSet) > 0 {
		currentProvidersNode, _, _ := yamlx.GetMappingValue(doc, "proxy-providers")
		mergedProviders := map[string]any{}
		if currentProvidersNode != nil {
			_ = currentProvidersNode.Decode(&mergedProviders)
		}
		for _, addon := range selectedSet {
			for key, value := range addon.ProxyProviders {
				mergedProviders[key] = value
			}
		}
		providerNode, err := yamlx.ToNode(mergedProviders)
		if err != nil {
			return nil, err
		}
		if err := yamlx.SetMappingValue(doc, "proxy-providers", providerNode); err != nil {
			return nil, err
		}
	}

	return yamlx.Marshal(doc)
}

func convertGroups(groupList []groups.ProxyGroup) []map[string]any {
	out := make([]map[string]any, 0, len(groupList))
	for _, group := range groupList {
		item := map[string]any{
			"name": group.Name,
			"type": group.Type,
		}
		if len(group.Proxies) > 0 {
			item["proxies"] = group.Proxies
		}
		if len(group.Use) > 0 {
			item["use"] = group.Use
		}
		if group.URL != "" {
			item["url"] = group.URL
		}
		if group.Interval > 0 {
			item["interval"] = group.Interval
		}
		out = append(out, item)
	}
	return out
}

func appendProviderGroupEntry(groupsList []map[string]any, targetGroup string, entry string) []map[string]any {
	for _, group := range groupsList {
		if group["name"] != targetGroup {
			continue
		}
		current, _ := group["proxies"].([]string)
		if current == nil {
			if generic, ok := group["proxies"].([]any); ok {
				for _, item := range generic {
					if text, ok := item.(string); ok {
						current = append(current, text)
					}
				}
			}
		}
		for _, existing := range current {
			if existing == entry {
				group["proxies"] = current
				return groupsList
			}
		}
		group["proxies"] = append(current, entry)
		return groupsList
	}
	return groupsList
}

func nodeToMap(node *yaml.Node) map[string]any {
	out := map[string]any{}
	_ = node.Decode(&out)
	return out
}
