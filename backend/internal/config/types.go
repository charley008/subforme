package config

const Version = "v1.3.5"

type XUIConfig struct {
	BaseURL  string `yaml:"base_url" json:"base_url"`
	APIKey   string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

type AppConfig struct {
	Mode                       string                         `yaml:"mode" json:"mode"`
	CacheTTLSeconds            int                            `yaml:"cache_ttl_seconds" json:"cache_ttl_seconds"`
	HealthcheckURL             string                         `yaml:"healthcheck_url" json:"healthcheck_url"`
	HealthcheckIntervalSeconds int                            `yaml:"healthcheck_interval_seconds" json:"healthcheck_interval_seconds"`
	XUI                        XUIConfig                      `yaml:"xui" json:"xui"`
	UserModes                  map[string]string              `yaml:"user_modes,omitempty" json:"user_modes,omitempty"`
	UserNodes                  map[string][]string            `yaml:"user_nodes,omitempty" json:"user_nodes,omitempty"`
	UserProviders              map[string][]string            `yaml:"user_providers,omitempty" json:"user_providers,omitempty"`
	UserGroupNodes             map[string]map[string][]string `yaml:"user_group_nodes,omitempty" json:"user_group_nodes,omitempty"`
	UserGroupModes             map[string]map[string]string   `yaml:"user_group_modes,omitempty" json:"user_group_modes,omitempty"`
}

type ManagedNode struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Address  string `yaml:"address" json:"address"`
	Port     int    `yaml:"port,omitempty" json:"port,omitempty"`
	ServerID int64  `yaml:"server_id,omitempty" json:"server_id,omitempty"`
}

type RuntimeConfig struct {
	Listen        string    `json:"listen"`
	AdminUsername string    `json:"admin_username"`
	AdminPassword string    `json:"admin_password"`
	SessionSecret string    `json:"session_secret"`
	ConfigDir     string    `json:"config_dir"`
	FrontendDir   string    `json:"frontend_dir"`
	XUI           XUIConfig `json:"xui"`
	RuntimePath   string    `json:"-"`
}

type ProviderAddon struct {
	ID                    string           `yaml:"id" json:"id"`
	Name                  string           `yaml:"name" json:"name"`
	SourceURL             string           `yaml:"source_url,omitempty" json:"source_url,omitempty"`
	UpdateIntervalSeconds int              `yaml:"update_interval_seconds,omitempty" json:"update_interval_seconds,omitempty"`
	InsecureSkipVerify    bool             `yaml:"insecure_skip_verify,omitempty" json:"insecure_skip_verify,omitempty"`
	LastUpdatedAt         int64            `yaml:"last_updated_at,omitempty" json:"last_updated_at,omitempty"`
	LastError             string           `yaml:"last_error,omitempty" json:"last_error,omitempty"`
	ProxyCount            int              `yaml:"proxy_count,omitempty" json:"proxy_count,omitempty"`
	ProxyProviders        map[string]any   `yaml:"proxy_providers" json:"proxy_providers"`
	ProxyGroups           []map[string]any `yaml:"proxy_groups" json:"proxy_groups"`
}

type BaseConfig struct {
	MixedPort   int            `yaml:"mixed-port,omitempty" json:"mixed-port,omitempty"`
	AllowLAN    bool           `yaml:"allow-lan,omitempty" json:"allow-lan,omitempty"`
	Mode        string         `yaml:"mode,omitempty" json:"mode,omitempty"`
	LogLevel    string         `yaml:"log-level,omitempty" json:"log-level,omitempty"`
	Proxies     []any          `yaml:"proxies" json:"proxies"`
	ProxyGroups []any          `yaml:"proxy-groups" json:"proxy-groups"`
	Rules       []string       `yaml:"rules" json:"rules"`
	Extra       map[string]any `yaml:",inline" json:"-"`
}

type GroupHealthcheck struct {
	URL             string `yaml:"url" json:"url"`
	IntervalSeconds int    `yaml:"interval_seconds" json:"interval_seconds"`
}

type GroupNames struct {
	Proxy string `yaml:"proxy" json:"proxy"`
	Auto  string `yaml:"auto" json:"auto"`
	Other string `yaml:"other" json:"other"`
}

type GroupDef struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	URL      string `yaml:"url,omitempty" json:"url,omitempty"`
	Interval int    `yaml:"interval,omitempty" json:"interval,omitempty"`
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"`
}

type GroupConfig struct {
	Healthcheck GroupHealthcheck    `yaml:"healthcheck" json:"healthcheck"`
	Regions     map[string][]string `yaml:"regions" json:"regions"`
	GroupNames  GroupNames          `yaml:"group_names" json:"group_names"`
	Groups      []GroupDef          `yaml:"groups,omitempty" json:"groups,omitempty"`
}

type Bundle struct {
	App           AppConfig
	BaseWhitelist BaseConfig
	BaseBlacklist BaseConfig
	Groups        GroupConfig
}
