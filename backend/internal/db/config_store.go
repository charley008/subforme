package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"subforme/backend/internal/config"
)

const (
	appConfigKey = "app_config"
	groupMetaKey = "group_meta"
	providersKey = "providers_config"
	nodesKey     = "nodes_config"
)

type groupMeta struct {
	Healthcheck config.GroupHealthcheck `json:"healthcheck"`
	Regions     map[string][]string     `json:"regions"`
	GroupNames  config.GroupNames       `json:"group_names"`
}

func (s *Store) HasSetting(key string) (bool, error) {
	var one int
	err := s.DB.QueryRow("SELECT 1 FROM app_settings WHERE key = ? LIMIT 1", key).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) CountProxyGroups() (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM proxy_groups").Scan(&n)
	return n, err
}

func (s *Store) CountProviders() (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM provider_addons").Scan(&n)
	return n, err
}

func (s *Store) SaveAppBaseConfig(cfg config.AppConfig) error {
	cfg.UserModes = nil
	cfg.UserNodes = nil
	cfg.UserProviders = nil
	cfg.UserGroupNodes = nil
	cfg.UserGroupModes = nil
	return s.saveJSONSetting(appConfigKey, cfg)
}

func (s *Store) LoadAppConfigFromDB(fallback config.AppConfig) (config.AppConfig, bool, error) {
	cfg := fallback
	found, err := s.loadJSONSetting(appConfigKey, &cfg)
	if err != nil {
		return config.AppConfig{}, false, err
	}
	users, err := s.ListUsers()
	if err != nil {
		return config.AppConfig{}, false, err
	}
	cfg.UserModes = map[string]string{}
	cfg.UserNodes = map[string][]string{}
	cfg.UserProviders = map[string][]string{}
	cfg.UserGroupNodes = map[string]map[string][]string{}
	cfg.UserGroupModes = map[string]map[string]string{}
	for _, u := range users {
		if u.Mode != "" {
			cfg.UserModes[u.Email] = u.Mode
		}
		if values := decodeStringSlice(u.NodeIDsJSON); len(values) > 0 {
			cfg.UserNodes[u.Email] = values
		}
		if values := decodeStringSlice(u.ProviderIDsJSON); len(values) > 0 {
			cfg.UserProviders[u.Email] = values
		}
		if values := decodeGroupNodes(u.GroupNodesJSON); len(values) > 0 {
			cfg.UserGroupNodes[u.Email] = values
		}
		if values := decodeStringMap(u.GroupModesJSON); len(values) > 0 {
			cfg.UserGroupModes[u.Email] = values
		}
	}
	return cfg, found, nil
}

func (s *Store) SaveAppConfigToDB(cfg config.AppConfig) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	base := cfg
	base.UserModes = nil
	base.UserNodes = nil
	base.UserProviders = nil
	base.UserGroupNodes = nil
	base.UserGroupModes = nil
	if err := saveJSONSettingTx(tx, appConfigKey, base); err != nil {
		return err
	}

	rows, err := tx.Query("SELECT id, email FROM users")
	if err != nil {
		return err
	}
	defer rows.Close()
	type userRef struct {
		id    int64
		email string
	}
	var users []userRef
	for rows.Next() {
		var u userRef
		if err := rows.Scan(&u.id, &u.email); err != nil {
			return err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, u := range users {
		mode := cfg.UserModes[u.email]
		nodeIDs := encodeJSON(cfg.UserNodes[u.email])
		providerIDs := encodeJSON(cfg.UserProviders[u.email])
		groupNodes := encodeJSON(cfg.UserGroupNodes[u.email])
		groupModes := encodeJSON(cfg.UserGroupModes[u.email])
		if _, err := tx.Exec(`
			UPDATE users
			SET mode = ?, node_ids_json = ?, provider_ids_json = ?, group_nodes_json = ?, group_modes_json = ?, updated_at = ?
			WHERE id = ?
		`, mode, nodeIDs, providerIDs, groupNodes, groupModes, time.Now().Unix(), u.id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) SaveGroupsConfigToDB(cfg config.GroupConfig) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	meta := groupMeta{Healthcheck: cfg.Healthcheck, Regions: cfg.Regions, GroupNames: cfg.GroupNames}
	if err := saveJSONSettingTx(tx, groupMetaKey, meta); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM proxy_groups"); err != nil {
		return err
	}
	for i, g := range cfg.Groups {
		g = normalizeProxyGroupForStorage(g)
		if _, err := tx.Exec(`
			INSERT INTO proxy_groups (name, type, url, interval, provider, sort_order)
			VALUES (?, ?, ?, ?, ?, ?)
		`, g.Name, g.Type, g.URL, g.Interval, g.Provider, i); err != nil {
			return err
		}
	}
	providers, err := loadProvidersTx(tx)
	if err != nil {
		return err
	}
	for _, p := range providers {
		if err := upsertProviderGroupTx(tx, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LoadGroupsConfigFromDB() (config.GroupConfig, bool, error) {
	n, err := s.CountProxyGroups()
	if err != nil {
		return config.GroupConfig{}, false, err
	}
	var meta groupMeta
	foundMeta, err := s.loadJSONSetting(groupMetaKey, &meta)
	if err != nil {
		return config.GroupConfig{}, false, err
	}
	if n == 0 && !foundMeta {
		return config.GroupConfig{}, false, nil
	}

	rows, err := s.DB.Query(`
		SELECT name, type, COALESCE(url,''), COALESCE(interval,0), COALESCE(provider,'')
		FROM proxy_groups ORDER BY sort_order, id
	`)
	if err != nil {
		return config.GroupConfig{}, false, err
	}
	defer rows.Close()

	cfg := config.GroupConfig{
		Healthcheck: meta.Healthcheck,
		Regions:     meta.Regions,
		GroupNames:  meta.GroupNames,
		Groups:      []config.GroupDef{},
	}
	if cfg.Regions == nil {
		cfg.Regions = map[string][]string{}
	}
	for rows.Next() {
		var g config.GroupDef
		if err := rows.Scan(&g.Name, &g.Type, &g.URL, &g.Interval, &g.Provider); err != nil {
			return config.GroupConfig{}, false, err
		}
		g = normalizeProxyGroupForStorage(g)
		cfg.Groups = append(cfg.Groups, g)
	}
	return cfg, true, rows.Err()
}

func normalizeProxyGroupForStorage(g config.GroupDef) config.GroupDef {
	if g.Provider == "" {
		g.URL = ""
		g.Interval = 0
	}
	return g
}

func (s *Store) SaveProvidersToDB(providers []config.ProviderAddon) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM provider_addons"); err != nil {
		return err
	}
	if err := saveJSONSettingTx(tx, providersKey, map[string]bool{"seeded": true}); err != nil {
		return err
	}
	for _, p := range providers {
		if err := upsertProviderTx(tx, p); err != nil {
			return err
		}
		if err := upsertProviderGroupTx(tx, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertProviderToDB(p config.ProviderAddon) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := saveJSONSettingTx(tx, providersKey, map[string]bool{"seeded": true}); err != nil {
		return err
	}
	if err := upsertProviderTx(tx, p); err != nil {
		return err
	}
	if err := upsertProviderGroupTx(tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteProviderFromDB(id string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := saveJSONSettingTx(tx, providersKey, map[string]bool{"seeded": true}); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM provider_addons WHERE id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM proxy_groups WHERE provider = ? OR name = ?", id, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LoadProvidersFromDB() ([]config.ProviderAddon, bool, error) {
	if found, err := s.HasSetting(providersKey); err != nil {
		return nil, false, err
	} else if !found {
		return []config.ProviderAddon{}, false, nil
	}
	n, err := s.CountProviders()
	if err != nil {
		return nil, false, err
	}
	if n == 0 {
		return []config.ProviderAddon{}, true, nil
	}
	rows, err := s.DB.Query(`
		SELECT id, name, COALESCE(source_url,''), COALESCE(update_interval_seconds,0),
		       COALESCE(insecure_skip_verify,0), COALESCE(last_updated_at,0),
		       COALESCE(last_error,''), COALESCE(proxy_count,0),
		       proxy_providers_json, proxy_groups_json
		FROM provider_addons ORDER BY id
	`)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var out []config.ProviderAddon
	for rows.Next() {
		var p config.ProviderAddon
		var insecure int
		var providersJSON, groupsJSON string
		if err := rows.Scan(&p.ID, &p.Name, &p.SourceURL, &p.UpdateIntervalSeconds,
			&insecure, &p.LastUpdatedAt, &p.LastError, &p.ProxyCount,
			&providersJSON, &groupsJSON); err != nil {
			return nil, false, err
		}
		p.InsecureSkipVerify = insecure != 0
		_ = json.Unmarshal([]byte(providersJSON), &p.ProxyProviders)
		_ = json.Unmarshal([]byte(groupsJSON), &p.ProxyGroups)
		out = append(out, p)
	}
	return out, true, rows.Err()
}

func (s *Store) CleanupProviderPrefs(providerID string) error {
	users, err := s.ListUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		providers := filterString(decodeStringSlice(u.ProviderIDsJSON), providerID)
		groups := decodeGroupNodes(u.GroupNodesJSON)
		for name, refs := range groups {
			groups[name] = filterString(refs, providerID)
			if len(groups[name]) == 0 {
				delete(groups, name)
			}
		}
		if _, err := s.DB.Exec(`
			UPDATE users SET provider_ids_json = ?, group_nodes_json = ?, updated_at = ? WHERE id = ?
		`, encodeJSON(providers), encodeJSON(groups), time.Now().Unix(), u.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CleanupGroupPrefs(validGroups []string) error {
	valid := map[string]struct{}{}
	for _, name := range validGroups {
		if name != "" {
			valid[name] = struct{}{}
		}
	}
	users, err := s.ListUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		groups := decodeGroupNodes(u.GroupNodesJSON)
		groupModes := decodeStringMap(u.GroupModesJSON)
		changed := false
		for name := range groups {
			if _, ok := valid[name]; !ok {
				delete(groups, name)
				changed = true
			}
		}
		for name := range groupModes {
			if _, ok := valid[name]; !ok {
				delete(groupModes, name)
				changed = true
			}
		}
		if changed {
			if _, err := s.DB.Exec(`
				UPDATE users SET group_nodes_json = ?, group_modes_json = ?, updated_at = ? WHERE id = ?
			`, encodeJSON(groups), encodeJSON(groupModes), time.Now().Unix(), u.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) CleanupNodePrefs(validNodeIDs []string) error {
	valid := map[string]struct{}{}
	for _, id := range validNodeIDs {
		if id != "" {
			valid[id] = struct{}{}
		}
	}
	users, err := s.ListUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		nodes := decodeStringSlice(u.NodeIDsJSON)
		filtered := nodes[:0]
		for _, id := range nodes {
			if _, ok := valid[id]; ok {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) != len(nodes) {
			if _, err := s.DB.Exec(`
				UPDATE users SET node_ids_json = ?, updated_at = ? WHERE id = ?
			`, encodeJSON(filtered), time.Now().Unix(), u.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func upsertProviderTx(tx *sql.Tx, p config.ProviderAddon) error {
	_, err := tx.Exec(`
		INSERT INTO provider_addons (
			id, name, source_url, update_interval_seconds, insecure_skip_verify,
			last_updated_at, last_error, proxy_count, proxy_providers_json,
			proxy_groups_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			source_url = excluded.source_url,
			update_interval_seconds = excluded.update_interval_seconds,
			insecure_skip_verify = excluded.insecure_skip_verify,
			last_updated_at = excluded.last_updated_at,
			last_error = excluded.last_error,
			proxy_count = excluded.proxy_count,
			proxy_providers_json = excluded.proxy_providers_json,
			proxy_groups_json = excluded.proxy_groups_json,
			updated_at = excluded.updated_at
	`, p.ID, p.Name, p.SourceURL, p.UpdateIntervalSeconds, boolToInt(p.InsecureSkipVerify),
		p.LastUpdatedAt, p.LastError, p.ProxyCount, encodeJSON(p.ProxyProviders),
		encodeJSON(p.ProxyGroups), time.Now().Unix())
	return err
}

func upsertProviderGroupTx(tx *sql.Tx, p config.ProviderAddon) error {
	if p.ID == "" {
		return nil
	}
	group := providerGroupDef(p)
	sortOrder, err := nextProxyGroupSortOrderTx(tx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO proxy_groups (name, type, url, interval, provider, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			type = excluded.type,
			url = excluded.url,
			interval = excluded.interval,
			provider = excluded.provider
	`, group.Name, group.Type, group.URL, group.Interval, group.Provider, sortOrder)
	return err
}

func providerGroupDef(p config.ProviderAddon) config.GroupDef {
	def := config.GroupDef{
		Name:     p.ID,
		Type:     "url-test",
		URL:      "http://www.gstatic.com/generate_204",
		Interval: 300,
		Provider: p.ID,
	}
	for _, raw := range p.ProxyGroups {
		name, _ := raw["name"].(string)
		if name != p.ID {
			continue
		}
		if typ, _ := raw["type"].(string); typ != "" {
			def.Type = typ
		}
		if url, _ := raw["url"].(string); url != "" {
			def.URL = url
		}
		if interval := anyToInt(raw["interval"]); interval > 0 {
			def.Interval = interval
		}
		return def
	}
	return def
}

func nextProxyGroupSortOrderTx(tx *sql.Tx) (int, error) {
	var n int
	err := tx.QueryRow("SELECT COALESCE(MAX(sort_order), -1) + 1 FROM proxy_groups").Scan(&n)
	return n, err
}

func loadProvidersTx(tx *sql.Tx) ([]config.ProviderAddon, error) {
	rows, err := tx.Query(`
		SELECT id, name, COALESCE(source_url,''), COALESCE(update_interval_seconds,0),
		       COALESCE(insecure_skip_verify,0), COALESCE(last_updated_at,0),
		       COALESCE(last_error,''), COALESCE(proxy_count,0),
		       proxy_providers_json, proxy_groups_json
		FROM provider_addons ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []config.ProviderAddon
	for rows.Next() {
		var p config.ProviderAddon
		var insecure int
		var providersJSON, groupsJSON string
		if err := rows.Scan(&p.ID, &p.Name, &p.SourceURL, &p.UpdateIntervalSeconds,
			&insecure, &p.LastUpdatedAt, &p.LastError, &p.ProxyCount,
			&providersJSON, &groupsJSON); err != nil {
			return nil, err
		}
		p.InsecureSkipVerify = insecure != 0
		_ = json.Unmarshal([]byte(providersJSON), &p.ProxyProviders)
		_ = json.Unmarshal([]byte(groupsJSON), &p.ProxyGroups)
		out = append(out, p)
	}
	return out, rows.Err()
}

func anyToInt(v any) int {
	switch value := v.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case int32:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	default:
		return 0
	}
}

func (s *Store) saveJSONSetting(key string, value any) error {
	_, err := s.DB.Exec(`
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, encodeJSON(value), time.Now().Unix())
	return err
}

func saveJSONSettingTx(tx *sql.Tx, key string, value any) error {
	_, err := tx.Exec(`
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, encodeJSON(value), time.Now().Unix())
	return err
}

func (s *Store) loadJSONSetting(key string, out any) (bool, error) {
	var raw string
	err := s.DB.QueryRow("SELECT value FROM app_settings WHERE key = ?", key).Scan(&raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return false, fmt.Errorf("decode %s: %w", key, err)
	}
	return true, nil
}

func encodeJSON(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeStringSlice(raw string) []string {
	var out []string
	if raw == "" {
		return nil
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func decodeGroupNodes(raw string) map[string][]string {
	var out map[string][]string
	if raw == "" {
		return nil
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func decodeStringMap(raw string) map[string]string {
	var out map[string]string
	if raw == "" {
		return nil
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func filterString(values []string, remove string) []string {
	filtered := values[:0]
	for _, value := range values {
		if value != remove {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
