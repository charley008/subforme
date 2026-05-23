export type HealthResponse = {
  status: string;
};

export type XuiConfig = {
  base_url: string;
  api_key?: string;
  username?: string;
  password?: string;
};

export type AppConfig = {
  mode: string;
  cache_ttl_seconds: number;
  healthcheck_url: string;
  healthcheck_interval_seconds: number;
  xui: XuiConfig;
  user_nodes?: Record<string, string[]>;
};

export type XUIConnectionStatus = {
  ok: boolean;
  base_url: string;
  detected_path?: string;
  inbound_count: number;
  enabled_count: number;
  message?: string;
};

export type AuthStatus = {
  authenticated: boolean;
  admin_username?: string;
};

export type BaseConfig = {
  "mixed-port"?: number;
  "allow-lan"?: boolean;
  mode?: string;
  "log-level"?: string;
  proxies: unknown[];
  "proxy-groups": unknown[];
  rules: string[];
};

export type GroupConfig = {
  healthcheck: {
    url: string;
    interval_seconds: number;
  };
  regions: Record<string, string[]>;
  group_names: {
    proxy: string;
    auto: string;
    other: string;
  };
};

export type UserSummary = {
  email: string;
  remark: string;
  protocol: string;
  node_count: number;
  server: string;
  port: number;
  last_remark?: string;
  mode: "whitelist" | "blacklist";
  share_url: string;
  selected_nodes: string[];
  selected_providers: string[];
  group_nodes?: Record<string, string[]>;
  uuid?: string;
  password?: string;
  server_traffic?: { server_name: string; up: number; down: number }[];
};

export type ManagedNode = {
  id: string;
  name: string;
  address: string;
  port?: number;
  server_id?: number;
};

export type AvailableNode = {
  id: string;
  name: string;
  address: string;
  protocol: string;
  port: number;
  network: string;
  security: string;
};

export type ProviderAddon = {
  id: string;
  name: string;
  attach_to_group?: string;
};

export type NodePreview = {
  id: string;
  name: string;
  type: string;
  server: string;
  port: number;
  uuid: string;
  flow: string;
  network: string;
  tls: boolean;
  server_name: string;
  client_fingerprint: string;
  reality_public_key: string;
  reality_short_id: string;
};

export type DashboardSummary = {
  mode: string;
  service: string;
  server_count: number;
  unique_users: number;
  node_count: number;
};
