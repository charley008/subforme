package db

// Server represents a 3x-ui panel (VPS) connection.
type Server struct {
	ID                         int64  `json:"id"`
	Name                       string `json:"name"`
	Scheme                     string `json:"scheme"`
	Host                       string `json:"host"`
	Port                       int    `json:"port"`
	BasePath                   string `json:"base_path"`
	APIKey                     string `json:"api_key,omitempty"`
	SubAddress                 string `json:"sub_address,omitempty"`
	SubPort                    int    `json:"sub_port,omitempty"`
	IsMain                     bool   `json:"is_main"`
	Remark                     string `json:"remark,omitempty"`
	Enabled                    bool   `json:"enabled"`
	TrafficSyncIntervalMinutes int    `json:"traffic_sync_interval_minutes"`
	AutoResetTrafficEnabled    bool   `json:"auto_reset_traffic_enabled"`
	AutoResetDay               int    `json:"auto_reset_day"`
	AutoResetHour              int    `json:"auto_reset_hour"`
	AutoResetMinute            int    `json:"auto_reset_minute"`
	AutoResetTimezone          string `json:"auto_reset_timezone,omitempty"`
	LastTrafficSyncAt          int64  `json:"last_traffic_sync_at"`
	LastTrafficResetKey        string `json:"last_traffic_reset_key,omitempty"`
	CreatedAt                  int64  `json:"created_at"`
	UpdatedAt                  int64  `json:"updated_at"`
}

// Inbound is a cached inbound config from a 3x-ui panel.
type Inbound struct {
	ID                 int64  `json:"id"`
	ServerID           int64  `json:"server_id"`
	InboundID          int    `json:"inbound_id"`
	Remark             string `json:"remark"`
	Listen             string `json:"listen,omitempty"`
	Port               int    `json:"port"`
	Protocol           string `json:"protocol"`
	Total              int64  `json:"total"`
	ExpiryTime         int64  `json:"expiry_time"`
	TrafficReset       string `json:"traffic_reset,omitempty"`
	SettingsJSON       string `json:"settings_json"`
	StreamSettingsJSON string `json:"stream_settings_json,omitempty"`
	SniffingJSON       string `json:"sniffing_json,omitempty"`
	Tag                string `json:"tag,omitempty"`
	Enable             bool   `json:"enable"`
	TrafficJSON        string `json:"traffic_json,omitempty"`
	UpdatedAt          int64  `json:"updated_at"`
}

// Node represents a managed proxy node.
type Node2 struct {
	ID         int64  `json:"-"`
	NodeID     string `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol,omitempty"`
	Network    string `json:"network,omitempty"`
	Flow       string `json:"flow,omitempty"`
	ServerName string `json:"server_name,omitempty"`
	ServerID   int64  `json:"server_id,omitempty"`
}

// UserTraffic is a stored traffic record.
type UserTraffic struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	ServerID  int64  `json:"server_id"`
	Up        int64  `json:"up"`
	Down      int64  `json:"down"`
	UpdatedAt int64  `json:"updated_at"`
}

// ServerTraffic holds per-server traffic data for a user.
type ServerTraffic struct {
	ServerID      int64  `json:"server_id"`
	ServerName    string `json:"server_name"`
	ServerAddress string `json:"server_address"`
	Up            int64  `json:"up"`
	Down          int64  `json:"down"`
}

// User is a local user registry entry.
type User struct {
	ID              int64  `json:"id"`
	Email           string `json:"email"`
	UUID            string `json:"uuid"`
	Password        string `json:"password,omitempty"`
	Auth            string `json:"auth,omitempty"`
	Flow            string `json:"flow,omitempty"`
	Security        string `json:"security,omitempty"`
	Remark          string `json:"remark,omitempty"`
	TotalGB         int64  `json:"total_gb"`
	ExpiryTime      int64  `json:"expiry_time"`
	LimitIP         int    `json:"limit_ip"`
	SubID           string `json:"sub_id,omitempty"`
	TgID            int64  `json:"tg_id,omitempty"`
	Reset           int    `json:"reset,omitempty"`
	Comment         string `json:"comment,omitempty"`
	Enable          bool   `json:"enable"`
	Mode            string `json:"mode,omitempty"`
	NodeIDsJSON     string `json:"node_ids_json,omitempty"`
	ProviderIDsJSON string `json:"provider_ids_json,omitempty"`
	GroupNodesJSON  string `json:"group_nodes_json,omitempty"`
	GroupModesJSON  string `json:"group_modes_json,omitempty"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

// UserAssignment links a user to a specific inbound on a specific server.
type UserAssignment struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	ServerID      int64  `json:"server_id"`
	InboundID     int64  `json:"inbound_id"`
	EmailOnServer string `json:"email_on_server"`
	Enable        bool   `json:"enable"`
	CreatedAt     int64  `json:"created_at"`
}
