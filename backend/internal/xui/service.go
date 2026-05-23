package xui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type Resolver struct {
	Client *Client
}

func NewResolver(client *Client) Resolver {
	return Resolver{Client: client}
}

func ResolveUser(users []UserRecord, query string) (UserRecord, bool) {
	for _, user := range users {
		if strings.EqualFold(user.Email, query) {
			return user, true
		}
	}
	for _, user := range users {
		if strings.EqualFold(user.Remark, query) {
			return user, true
		}
	}
	return UserRecord{}, false
}

func (r Resolver) ResolveUserNodes(ctx context.Context, query string) ([]Node, error) {
	if r.Client == nil {
		return nil, fmt.Errorf("xui client is not configured")
	}
	if r.Client.BaseURL == "" {
		return nil, fmt.Errorf("xui base url is not configured")
	}
	if r.Client.APIKey == "" && (r.Client.Username == "" || r.Client.Password == "") {
		return nil, fmt.Errorf("xui auth is not configured")
	}

	inbounds, err := r.Client.ListInbounds(ctx)
	if err != nil {
		return nil, err
	}

	host, _ := panelHost(r.Client.BaseURL)
	nodes := collectNodes(inbounds, host, func(client InboundClient, inbound InboundRecord) bool {
		return strings.EqualFold(client.Email, query) || strings.EqualFold(firstNonEmpty(inbound.Remark, client.Email), query)
	})

	if len(nodes) == 0 {
		return nil, fmt.Errorf("user %q not found", query)
	}

	return nodes, nil
}

func (r Resolver) SearchUsers(ctx context.Context, query string) ([]UserSummary, error) {
	if r.Client == nil {
		return nil, fmt.Errorf("xui client is not configured")
	}
	inbounds, err := r.Client.ListInbounds(ctx)
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	host, _ := panelHost(r.Client.BaseURL)
	summaries := map[string]*UserSummary{}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		settings, ok := parseInboundSettings(inbound.Settings)
		if !ok {
			continue
		}

		for _, client := range settings.Clients {
			if !client.Enable {
				continue
			}

			displayRemark := firstNonEmpty(inbound.Remark, client.Email)
			if needle != "" &&
				!strings.Contains(strings.ToLower(client.Email), needle) &&
				!strings.Contains(strings.ToLower(displayRemark), needle) {
				continue
			}

			existing, found := summaries[client.Email]
			if !found {
				existing = &UserSummary{
					Email:    client.Email,
					Remark:   displayRemark,
					Protocol: inbound.Protocol,
					Server:   pickServerHost(inbound.Listen, host),
					Port:     inbound.Port,
					UUID:     client.ID,
					Password: client.Password,
				}
				summaries[client.Email] = existing
			}
			existing.NodeCount++
			existing.LastRemark = inbound.Remark
		}
	}

	out := make([]UserSummary, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Email < out[j].Email
	})
	return out, nil
}

func (r Resolver) ListAvailableNodes(ctx context.Context) ([]AvailableNode, error) {
	if r.Client == nil {
		return nil, fmt.Errorf("xui client is not configured")
	}
	inbounds, err := r.Client.ListInbounds(ctx)
	if err != nil {
		return nil, err
	}

	host, _ := panelHost(r.Client.BaseURL)
	seen := map[string]AvailableNode{}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		stream := parseStreamSettings(inbound.StreamSettings)
		server := pickServerHost(inbound.Listen, host)
		id := buildNodeID(inbound.Protocol, inbound.Remark, server, inbound.Port)
		seen[id] = AvailableNode{
			ID:       id,
			Name:     firstNonEmpty(inbound.Remark, inbound.Protocol),
			Address:  server,
			Protocol: inbound.Protocol,
			Port:     inbound.Port,
			Network:  stream.Network,
			Security: stream.Security,
		}
	}

	out := make([]AvailableNode, 0, len(seen))
	for _, node := range seen {
		out = append(out, node)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Address < out[j].Address
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (r Resolver) TestConnection(ctx context.Context) (ConnectionStatus, error) {
	if r.Client == nil {
		return ConnectionStatus{OK: false, Message: "xui client is not configured"}, fmt.Errorf("xui client is not configured")
	}
	detected, inbounds, err := r.Client.DetectInboundEndpoint(ctx)
	if err != nil {
		return ConnectionStatus{
			OK:      false,
			BaseURL: r.Client.BaseURL,
			Message: err.Error(),
		}, err
	}

	enabled := 0
	for _, inbound := range inbounds {
		if inbound.Enable {
			enabled++
		}
	}

	return ConnectionStatus{
		OK:           true,
		BaseURL:      r.Client.BaseURL,
		DetectedPath: detected,
		InboundCount: len(inbounds),
		EnabledCount: enabled,
		Message:      "3x-ui connection successful",
	}, nil
}

func firstOrEmpty(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func parseInboundSettings(raw string) (inboundSettings, bool) {
	settings := inboundSettings{}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return inboundSettings{}, false
	}
	return settings, true
}

// ParseInboundSettings is the exported version of parseInboundSettings.
func ParseInboundSettings(raw string) (inboundSettings, bool) {
	return parseInboundSettings(raw)
}

// ClientParams holds the fields needed to create/update a client on a 3x-ui inbound.
type ClientParams struct {
	Email      string
	UUID       string
	Password   string
	Flow       string
	TotalGB    int64
	ExpiryTime int64
	LimitIP    int
	Enable     bool
}

// BuildClientConfig creates an InboundClient from ClientParams for the given protocol.
func BuildClientConfig(protocol string, p ClientParams) InboundClient {
	c := InboundClient{
		Email:      p.Email,
		Enable:     p.Enable,
		TotalGB:    p.TotalGB,
		ExpiryTime: p.ExpiryTime,
		LimitIP:    p.LimitIP,
	}
	switch protocol {
	case "vless", "vmess":
		c.ID = p.UUID
		c.Flow = p.Flow
	case "trojan":
		c.Password = p.Password
	case "shadowsocks":
		c.Password = p.Password
	case "hysteria2", "hysteria":
		c.Password = p.Password
	default:
		c.ID = p.UUID
	}
	return c
}

func parseStreamSettings(raw string) InboundStreamSettings {
	stream := InboundStreamSettings{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &stream)
	}
	return stream
}

func ParseStreamSettings(raw string) InboundStreamSettings {
	return parseStreamSettings(raw)
}

func collectNodes(inbounds []InboundRecord, host string, include func(client InboundClient, inbound InboundRecord) bool) []Node {
	nodes := make([]Node, 0)
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}

		settings, ok := parseInboundSettings(inbound.Settings)
		if !ok {
			continue
		}
		stream := parseStreamSettings(inbound.StreamSettings)

		for _, client := range settings.Clients {
			if !client.Enable || !include(client, inbound) {
				continue
			}
			nodes = append(nodes, Node{
				ID:                buildNodeID(inbound.Protocol, inbound.Remark, pickServerHost(inbound.Listen, host), inbound.Port),
				Name:              firstNonEmpty(inbound.Remark, client.Email, inbound.Protocol),
				Type:              inbound.Protocol,
				Server:            pickServerHost(inbound.Listen, host),
				Port:              inbound.Port,
				UUID:              client.ID,
				Password:          client.Password,
				Flow:              client.Flow,
				Network:           normalizeNetwork(stream.Network, stream.TCPSettings.Header.Type),
				TLS:               stream.Security == "tls" || stream.Security == "reality",
				UDP:               inbound.Protocol == "vless",
				ALPN:              pickALPN(stream),
				Encryption:        pickEncryption(inbound.Protocol, stream),
				ServerName:        pickServerName(stream),
				ClientFingerprint: firstNonEmpty(stream.RealitySettings.Settings.Fingerprint, stream.TLSSettings.Fingerprint),
				RealityPublicKey:  stream.RealitySettings.Settings.PublicKey,
				RealityShortID:    firstOrEmpty(stream.RealitySettings.ShortIds),
				XHTTPPath:         stream.XHTTPSettings.Path,
				XHTTPHost:         stream.XHTTPSettings.Host,
				XHTTPMode:         stream.XHTTPSettings.Mode,
			})
		}
	}
	return nodes
}

func buildNodeID(protocol, remark, server string, port int) string {
	return fmt.Sprintf("%s|%s|%s|%d", protocol, firstNonEmpty(remark, protocol), server, port)
}

func pickServerName(stream InboundStreamSettings) string {
	if stream.RealitySettings.Settings.ServerName != "" {
		return stream.RealitySettings.Settings.ServerName
	}
	if stream.TLSSettings.ServerName != "" {
		return stream.TLSSettings.ServerName
	}
	if len(stream.RealitySettings.ServerNames) > 0 {
		return stream.RealitySettings.ServerNames[0]
	}
	return ""
}

func pickALPN(stream InboundStreamSettings) []string {
	if len(stream.ALPN) > 0 {
		return stream.ALPN
	}
	if len(stream.TLSSettings.ALPN) > 0 {
		return stream.TLSSettings.ALPN
	}
	return nil
}

func pickEncryption(protocol string, stream InboundStreamSettings) string {
	if protocol == "vless" && stream.Network == "xhttp" {
		return ""
	}
	return ""
}

func normalizeNetwork(network, headerType string) string {
	if network == "tcp" && (headerType == "" || headerType == "none") {
		return "raw"
	}
	if network == "" {
		return "raw"
	}
	return network
}

func pickServerHost(listen, fallback string) string {
	switch listen {
	case "", "0.0.0.0", "::", "127.0.0.1", "localhost":
		return fallback
	default:
		return listen
	}
}

func panelHost(base string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	return parsed.Hostname(), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
