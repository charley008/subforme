package xui

import (
	"encoding/json"
	"strconv"
	"strings"
)

type UserRecord struct {
	ID     int64  `json:"id"`
	Email  string `json:"email"`
	Remark string `json:"remark"`
	UUID   string `json:"uuid"`
}

type Node struct {
	ID                string   `json:"id,omitempty"`
	Name              string   `json:"name"`
	Type              string   `json:"type"`
	Server            string   `json:"server"`
	Port              int      `json:"port"`
	UUID              string   `json:"uuid"`
	Password          string   `json:"password,omitempty"`
	Flow              string   `json:"flow,omitempty"`
	Network           string   `json:"network,omitempty"`
	TLS               bool     `json:"tls"`
	UDP               bool     `json:"udp"`
	ALPN              []string `json:"alpn,omitempty"`
	Encryption        string   `json:"encryption,omitempty"`
	ServerName        string   `json:"server_name,omitempty"`
	ClientFingerprint string   `json:"client_fingerprint,omitempty"`
	RealityPublicKey  string   `json:"reality_public_key,omitempty"`
	RealityShortID    string   `json:"reality_short_id,omitempty"`
	XHTTPPath         string   `json:"xhttp_path,omitempty"`
	XHTTPHost         string   `json:"xhttp_host,omitempty"`
	XHTTPMode         string   `json:"xhttp_mode,omitempty"`
}

type AvailableNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Network  string `json:"network"`
	Security string `json:"security"`
}

type ConnectionStatus struct {
	OK           bool   `json:"ok"`
	BaseURL      string `json:"base_url"`
	DetectedPath string `json:"detected_path,omitempty"`
	InboundCount int    `json:"inbound_count"`
	EnabledCount int    `json:"enabled_count"`
	Message      string `json:"message,omitempty"`
}

type UserSummary struct {
	Email      string `json:"email"`
	Remark     string `json:"remark"`
	Protocol   string `json:"protocol"`
	NodeCount  int    `json:"node_count"`
	Server     string `json:"server"`
	Port       int    `json:"port"`
	LastRemark string `json:"last_remark,omitempty"`
	UUID       string `json:"uuid,omitempty"`
	Password   string `json:"password,omitempty"`
}

type inboundListResponse struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	RawObj  json.RawMessage `json:"obj"`
	RawData json.RawMessage `json:"data"`
}

type ClientTraffic struct {
	ID         int64  `json:"id"`
	InboundID  int64  `json:"inboundId"`
	Enable     bool   `json:"enable"`
	Email      string `json:"email"`
	UUID       string `json:"uuid"`
	SubID      string `json:"subId"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	AllTime    int64  `json:"allTime"`
	ExpiryTime int64  `json:"expiryTime"`
	Total      int64  `json:"total"`
	Reset      int    `json:"reset"`
	LastOnline int64  `json:"lastOnline"`
}

type ClientListRecord struct {
	ID         int64  `json:"id"`
	Email      string `json:"email"`
	SubID      string `json:"subId"`
	UUID       string `json:"uuid"`
	Password   string `json:"password"`
	Auth       string `json:"auth"`
	Flow       string `json:"flow"`
	Security   string `json:"security"`
	LimitIP    int    `json:"limitIp"`
	TotalGB    int64  `json:"totalGB"`
	ExpiryTime int64  `json:"expiryTime"`
	Enable     bool   `json:"enable"`
	TgID       int64  `json:"tgId"`
	Reset      int    `json:"reset"`
	Comment    string `json:"comment"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
	InboundIDs []int  `json:"inboundIds"`
}

type clientListResponse struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	RawObj  json.RawMessage `json:"obj"`
	RawData json.RawMessage `json:"data"`
}

type InboundRecord struct {
	ID             int64           `json:"id"`
	Up             int64           `json:"up"`
	Down           int64           `json:"down"`
	Total          int64           `json:"total"`
	Remark         string          `json:"remark"`
	Enable         bool            `json:"enable"`
	ExpiryTime     int64           `json:"expiryTime"`
	TrafficReset   string          `json:"trafficReset"`
	Listen         string          `json:"listen"`
	Port           int             `json:"port"`
	Protocol       string          `json:"protocol"`
	Settings       string          `json:"settings"`
	StreamSettings string          `json:"streamSettings"`
	Tag            string          `json:"tag"`
	Sniffing       string          `json:"sniffing"`
	ClientStats    []ClientTraffic `json:"clientStats,omitempty"`
}

func (r *InboundRecord) UnmarshalJSON(raw []byte) error {
	type inboundAlias InboundRecord
	var aux struct {
		inboundAlias
		Settings       json.RawMessage `json:"settings"`
		StreamSettings json.RawMessage `json:"streamSettings"`
		Sniffing       json.RawMessage `json:"sniffing"`
	}
	if err := json.Unmarshal(raw, &aux); err != nil {
		return err
	}
	*r = InboundRecord(aux.inboundAlias)
	r.Settings = rawJSONString(aux.Settings)
	r.StreamSettings = rawJSONString(aux.StreamSettings)
	r.Sniffing = rawJSONString(aux.Sniffing)
	return nil
}

func rawJSONString(raw json.RawMessage) string {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return ""
	}
	if strings.HasPrefix(text, `"`) {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err == nil {
			return decoded
		}
	}
	return text
}

type inboundSettings struct {
	Clients []InboundClient `json:"clients"`
}

type InboundClient struct {
	Email      string        `json:"email"`
	ID         string        `json:"id,omitempty"`
	Security   string        `json:"security,omitempty"`
	Password   string        `json:"password,omitempty"`
	Flow       string        `json:"flow,omitempty"`
	Reverse    any           `json:"reverse,omitempty"`
	Auth       string        `json:"auth,omitempty"`
	LimitIP    int           `json:"limitIp"`
	TotalGB    int64         `json:"totalGB"`
	ExpiryTime int64         `json:"expiryTime"`
	Enable     bool          `json:"enable"`
	TgID       FlexibleInt64 `json:"tgId,omitempty"`
	SubID      string        `json:"subId,omitempty"`
	Comment    string        `json:"comment,omitempty"`
	Reset      int           `json:"reset,omitempty"`
	CreatedAt  int64         `json:"created_at,omitempty"`
	UpdatedAt  int64         `json:"updated_at,omitempty"`
}

type FlexibleInt64 int64

func (v *FlexibleInt64) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" || text == `""` {
		*v = 0
		return nil
	}
	if strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		if s == "" {
			*v = 0
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			*v = 0
			return nil
		}
		*v = FlexibleInt64(n)
		return nil
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*v = FlexibleInt64(n)
	return nil
}

func (v FlexibleInt64) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(v), 10)), nil
}

type InboundStreamSettings struct {
	Network         string          `json:"network"`
	Security        string          `json:"security"`
	ALPN            []string        `json:"alpn"`
	RealitySettings realitySettings `json:"realitySettings"`
	TLSSettings     tlsSettings     `json:"tlsSettings"`
	TCPSettings     tcpSettings     `json:"tcpSettings"`
	WSSettings      wsSettings      `json:"wsSettings"`
	GRPCSettings    grpcSettings    `json:"grpcSettings"`
	XHTTPSettings   xhttpSettings   `json:"xhttpSettings"`
}

type realitySettings struct {
	ServerNames []string `json:"serverNames"`
	ShortIds    []string `json:"shortIds"`
	Settings    struct {
		PublicKey   string `json:"publicKey"`
		Fingerprint string `json:"fingerprint"`
		ServerName  string `json:"serverName"`
	} `json:"settings"`
}

type tlsSettings struct {
	ServerName  string   `json:"serverName"`
	ALPN        []string `json:"alpn"`
	Fingerprint string   `json:"fingerprint"`
}

type tcpSettings struct {
	Header struct {
		Type string `json:"type"`
	} `json:"header"`
}

type wsSettings struct {
	Path string `json:"path"`
}

type grpcSettings struct {
	ServiceName string `json:"serviceName"`
}

type xhttpSettings struct {
	Path string `json:"path"`
	Host string `json:"host"`
	Mode string `json:"mode"`
}
