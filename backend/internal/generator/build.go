package generator

import (
	"strings"

	"subforme/backend/internal/config"
	"subforme/backend/internal/groups"
	"subforme/backend/internal/xui"
	"subforme/backend/pkg/yamlx"
)

type groupEntry struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies,omitempty"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
	Use      []string `yaml:"use,omitempty"`
}

func BuildFinalYAML(templateRaw string, nodes []xui.Node, groupList []groups.ProxyGroup, addons []config.ProviderAddon, selectedProviders []string, mainGroupName string) ([]byte, error) {
	doc, err := yamlx.Parse(templateRaw)
	if err != nil {
		return nil, err
	}

	proxies := make([]Proxy, 0, len(nodes))
	for _, node := range nodes {
		proxies = append(proxies, buildProxy(node))
	}

	selectedSet := map[string]config.ProviderAddon{}
	for _, id := range selectedProviders {
		for _, addon := range addons {
			if addon.ID == id {
				selectedSet[id] = addon
			}
		}
	}

	groupEntries := convertGroups(groupList)
	for _, addon := range selectedSet {
		for _, g := range providerGroups(addon) {
			groupEntries = upsertProviderGroup(groupEntries, groupEntryFromMap(g))
		}
	}
	groupEntries = filterInvalidGroupRefs(groupEntries, nodes)

	proxyNode, err := yamlx.ToNode(proxies)
	if err != nil {
		return nil, err
	}
	groupNode, err := yamlx.ToNode(groupEntries)
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

	raw, err := yamlx.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return []byte(compactCommentSpacing(string(raw))), nil
}

func buildProxy(node xui.Node) Proxy {
	isXHTTP := node.Network == "xhttp"
	proxy := Proxy{
		Name:    node.Name,
		Type:    node.Type,
		Server:  node.Server,
		Port:    node.Port,
		Network: node.Network,
		TLS:     node.TLS,
		UDP:     node.UDP,
		ALPN:    node.ALPN,
		ServerName:        node.ServerName,
		ClientFingerprint: node.ClientFingerprint,
	}

	switch node.Type {
	case "vless", "vmess":
		proxy.UUID = node.UUID
	case "trojan", "shadowsocks":
		proxy.Password = node.Password
	}

	switch node.Type {
	case "vless":
		applyVLESSTemplate(&proxy, node, isXHTTP)
	case "vmess":
		cipher := node.Security
		if cipher == "" {
			cipher = "auto"
		}
		proxy.Cipher = cipher
	case "trojan", "shadowsocks":
		// protocol-specific credential already applied above
	}

	return proxy
}

func applyVLESSTemplate(proxy *Proxy, node xui.Node, isXHTTP bool) {
	if isXHTTP {
		empty := ""
		proxy.Encryption = &empty
		proxy.TLS = true
		if len(proxy.ALPN) == 0 {
			proxy.ALPN = []string{"h2"}
		}
		if proxy.ClientFingerprint == "" {
			proxy.ClientFingerprint = "chrome"
		}
		path := node.XHTTPPath
		if path == "" {
			path = "/"
		}
		proxy.XHTTPOpts = &XHTTPOpts{
			Path: path,
			Host: node.XHTTPHost,
			Mode: normalizeXHTTPMode(node.XHTTPMode),
		}
		return
	}

	proxy.Flow = node.Flow
	if node.RealityPublicKey != "" || node.RealityShortID != "" {
		proxy.RealityOpts = &RealityOpts{
			PublicKey: node.RealityPublicKey,
			ShortID:   node.RealityShortID,
		}
	}
}

func normalizeXHTTPMode(mode string) string {
	switch mode {
	case "", "auto":
		return "stream-one"
	default:
		return mode
	}
}

func compactCommentSpacing(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" && i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "#") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func filterInvalidGroupRefs(entries []groupEntry, nodes []xui.Node) []groupEntry {
	valid := map[string]struct{}{
		"DIRECT": {},
		"REJECT": {},
		"GLOBAL": {},
		"PASS":   {},
	}
	for _, node := range nodes {
		valid[node.Name] = struct{}{}
	}
	for _, entry := range entries {
		valid[entry.Name] = struct{}{}
	}
	for i := range entries {
		if len(entries[i].Proxies) == 0 {
			continue
		}
		filtered := entries[i].Proxies[:0]
		for _, proxy := range entries[i].Proxies {
			if _, ok := valid[proxy]; ok {
				filtered = append(filtered, proxy)
			}
		}
		entries[i].Proxies = filtered
	}
	return entries
}

func providerGroups(addon config.ProviderAddon) []map[string]any {
	if len(addon.ProxyGroups) > 0 {
		return addon.ProxyGroups
	}
	return []map[string]any{
		{
			"name":     addon.ID,
			"type":     "url-test",
			"url":      "http://www.gstatic.com/generate_204",
			"interval": 300,
			"use":      []string{addon.ID},
		},
	}
}

func convertGroups(groupList []groups.ProxyGroup) []groupEntry {
	out := make([]groupEntry, 0, len(groupList))
	for _, group := range groupList {
		entry := groupEntry{
			Name: group.Name,
			Type: group.Type,
		}
		if len(group.Proxies) > 0 {
			entry.Proxies = group.Proxies
		}
		if len(group.Use) > 0 {
			entry.Use = group.Use
		}
		if group.URL != "" {
			entry.URL = group.URL
		}
		if group.Interval > 0 {
			entry.Interval = group.Interval
		}
		out = append(out, entry)
	}
	return out
}

func upsertProviderGroup(entries []groupEntry, next groupEntry) []groupEntry {
	if next.Name == "" {
		return entries
	}
	for i, existing := range entries {
		if existing.Name != next.Name {
			continue
		}
		entries[i] = mergeProviderGroup(existing, next)
		return entries
	}
	return append(entries, next)
}

func mergeProviderGroup(existing, next groupEntry) groupEntry {
	if existing.Type == "" {
		existing.Type = next.Type
	}
	if existing.URL == "" {
		existing.URL = next.URL
	}
	if existing.Interval == 0 {
		existing.Interval = next.Interval
	}
	if len(existing.Use) == 0 {
		existing.Use = next.Use
	}
	if len(existing.Proxies) == 0 {
		existing.Proxies = next.Proxies
	}
	return existing
}

func groupEntryFromMap(m map[string]any) groupEntry {
	e := groupEntry{}
	if v, ok := m["name"].(string); ok {
		e.Name = v
	}
	if v, ok := m["type"].(string); ok {
		e.Type = v
	}
	if v, ok := m["url"].(string); ok {
		e.URL = v
	}
	if v, ok := m["interval"].(int); ok {
		e.Interval = v
	}
	if v, ok := m["proxies"].([]string); ok {
		e.Proxies = v
	}
	if v, ok := m["use"].([]string); ok {
		e.Use = v
	}
	// Handle []any to []string conversion for proxies/use
	if v, ok := m["proxies"].([]any); ok {
		for _, p := range v {
			if s, ok := p.(string); ok {
				e.Proxies = append(e.Proxies, s)
			}
		}
	}
	if v, ok := m["use"].([]any); ok {
		for _, p := range v {
			if s, ok := p.(string); ok {
				e.Use = append(e.Use, s)
			}
		}
	}
	return e
}
