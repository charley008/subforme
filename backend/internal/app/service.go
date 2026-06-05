package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"subforme/backend/internal/auth"
	"subforme/backend/internal/config"
	"subforme/backend/internal/db"
	"subforme/backend/internal/generator"
	"subforme/backend/internal/groups"
	"subforme/backend/internal/xui"
)

const trafficRefreshRequestTimeout = 5 * time.Second

type XUIResolver interface {
	ResolveUserNodes(ctx context.Context, query string) ([]xui.Node, error)
	SearchUsers(ctx context.Context, query string) ([]xui.UserSummary, error)
	ListAvailableNodes(ctx context.Context) ([]xui.AvailableNode, error)
	TestConnection(ctx context.Context) (xui.ConnectionStatus, error)
}

type ResolverFactory func(cfg config.XUIConfig) XUIResolver
type ModeTemplateLoader func(dir, mode string) (string, error)

type Service struct {
	ConfigDir       string
	Loader          func(string) (config.Bundle, error)
	DefaultXUI      config.XUIConfig
	ResolverFactory ResolverFactory
	TemplateLoader  ModeTemplateLoader
	DB              *db.Store
}

func NewService(configDir string, defaultXUI config.XUIConfig) Service {
	return Service{
		ConfigDir:      configDir,
		Loader:         config.LoadBundle,
		DefaultXUI:     defaultXUI,
		TemplateLoader: config.LoadModeTemplateYAML,
		ResolverFactory: func(cfg config.XUIConfig) XUIResolver {
			return xui.NewResolver(xui.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Username, cfg.Password))
		},
	}
}

func NewServiceWithDB(configDir string, defaultXUI config.XUIConfig, store *db.Store) Service {
	svc := NewService(configDir, defaultXUI)
	svc.DB = store
	return svc
}

func (s Service) Generate(user string) ([]byte, error) {
	return s.GenerateWithBaseURL(user, "")
}

func (s Service) GenerateWithBaseURL(user, publicBaseURL string) ([]byte, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return nil, err
	}

	var nodes []xui.Node
	if s.DB != nil {
		nodes, err = s.dbResolveUserNodes(user)
	}
	if len(nodes) == 0 {
		nodes, err = s.resolver(bundle.App).ResolveUserNodes(context.Background(), user)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve user nodes: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("user %q not found", user)
	}
	managedNodes := s.loadManagedNodes()
	nodes = applyManagedNodes(nodes, managedNodes, bundle.App.UserNodes[user])
	if len(nodes) == 0 {
		return nil, fmt.Errorf("user %q has no selected nodes", user)
	}

	mode := s.resolveMode(bundle.App, user)
	providers, err := s.ReadProviders()
	if err != nil {
		return nil, fmt.Errorf("load providers: %w", err)
	}
	providers = withProviderPublicBaseURL(providers, publicBaseURL)
	templateLoader := s.TemplateLoader
	if templateLoader == nil {
		templateLoader = config.LoadModeTemplateYAML
	}
	templateRaw, err := templateLoader(s.ConfigDir, mode)
	if err != nil {
		return nil, fmt.Errorf("load %s template: %w", mode, err)
	}

	groupList := groups.Build(bundle.Groups, nodes, bundle.App.UserGroupNodes[user], bundle.App.UserGroupModes[user], bundle.App.UserProviders[user])
	raw, err := generator.BuildFinalYAML(templateRaw, nodes, groupList, providers, bundle.App.UserProviders[user], mainProxyGroupName(bundle.Groups, groupList))
	if err != nil {
		return nil, fmt.Errorf("build final yaml: %w", err)
	}

	validation := generator.ValidateConfig(raw)
	if validation.HasErrors() {
		warning := "# 配置警告：\n"
		for _, e := range validation.Errors {
			warning += "#   - " + e + "\n"
		}
		raw = append([]byte(warning+"\n"), raw...)
	}

	return raw, nil
}

func withProviderPublicBaseURL(providers []config.ProviderAddon, publicBaseURL string) []config.ProviderAddon {
	publicBaseURL = strings.TrimRight(publicBaseURL, "/")
	if publicBaseURL == "" {
		return providers
	}
	out := make([]config.ProviderAddon, len(providers))
	copy(out, providers)
	for i := range out {
		if len(out[i].ProxyProviders) == 0 {
			continue
		}
		copied := make(map[string]any, len(out[i].ProxyProviders))
		for key, value := range out[i].ProxyProviders {
			copied[key] = rewriteProviderURLValue(value, publicBaseURL, key)
		}
		out[i].ProxyProviders = copied
	}
	return out
}

func rewriteProviderURLValue(value any, publicBaseURL, id string) any {
	entry, ok := value.(map[string]any)
	if !ok {
		return value
	}
	copied := make(map[string]any, len(entry))
	for k, v := range entry {
		copied[k] = v
	}
	copied["url"] = publicBaseURL + "/api/proxy-providers/" + id + ".yaml"
	return copied
}

func (s Service) ReadAppConfig() (config.AppConfig, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return config.AppConfig{}, err
	}
	bundle.App.XUI = s.mergeXUI(bundle.App.XUI)
	return bundle.App, nil
}

func (s Service) UpdateAppConfig(next config.AppConfig) error {
	if s.Loader == nil {
		return fmt.Errorf("config loader is not configured")
	}
	if s.DB != nil {
		return s.DB.SaveAppConfigToDB(next)
	}
	return config.SaveAppConfig(s.ConfigDir, next)
}

func (s Service) ReadGroupsConfig() (config.GroupConfig, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return config.GroupConfig{}, err
	}
	return bundle.Groups, nil
}

func (s Service) UpdateGroupsConfig(next config.GroupConfig) error {
	if s.DB != nil {
		if err := s.DB.SaveGroupsConfigToDB(next); err != nil {
			return err
		}
		return s.DB.CleanupGroupPrefs(groupConfigNames(next))
	}
	if err := config.SaveGroupsConfig(s.ConfigDir, next); err != nil {
		return err
	}
	return s.cleanupAppConfig()
}

func (s Service) SearchUsers(query string) ([]xui.UserSummary, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return nil, err
	}
	return s.resolver(bundle.App).SearchUsers(context.Background(), query)
}

func (s Service) PreviewUser(query string) ([]xui.Node, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return nil, err
	}
	var nodes []xui.Node
	if s.DB != nil {
		nodes, err = s.dbResolveUserNodes(query)
	}
	if len(nodes) == 0 {
		nodes, err = s.resolver(bundle.App).ResolveUserNodes(context.Background(), query)
	}
	if err != nil {
		return nil, err
	}
	return applyManagedNodes(nodes, s.loadManagedNodes(), bundle.App.UserNodes[query]), nil
}

func (s Service) loadManagedNodes() []config.ManagedNode {
	if s.DB != nil {
		dbNodes, err := s.DB.ListNodeDB()
		if err == nil {
			out := make([]config.ManagedNode, len(dbNodes))
			for i, n := range dbNodes {
				out[i] = config.ManagedNode{
					ID:       n.NodeID,
					Name:     n.Name,
					Address:  n.Address,
					Port:     n.Port,
					ServerID: n.ServerID,
				}
			}
			return out
		}
	}
	// Fallback to YAML
	nodes, _ := config.LoadManagedNodes(s.ConfigDir)
	return nodes
}

func (s Service) cleanupAppConfig() error {
	state := config.AppCleanupState{}

	if s.DB != nil {
		if users, err := s.DB.ListUsers(); err == nil {
			state.UsersKnown = true
			for _, u := range users {
				state.Users = append(state.Users, u.Email)
			}
		}
		if nodes, err := s.DB.ListNodeDB(); err == nil {
			state.NodeIDsKnown = true
			state.NodeNamesKnown = true
			for _, n := range nodes {
				state.NodeIDs = append(state.NodeIDs, n.NodeID)
				state.NodeNames = append(state.NodeNames, n.Name)
			}
		}
	}
	if !state.NodeIDsKnown || !state.NodeNamesKnown {
		if nodes, err := config.LoadManagedNodes(s.ConfigDir); err == nil {
			state.NodeIDsKnown = true
			state.NodeNamesKnown = true
			for _, n := range nodes {
				state.NodeIDs = append(state.NodeIDs, n.ID)
				state.NodeNames = append(state.NodeNames, n.Name)
			}
		}
	}
	if providers, err := config.LoadProviders(s.ConfigDir); err == nil {
		state.ProvidersKnown = true
		for _, p := range providers {
			state.Providers = append(state.Providers, p.ID)
		}
	}
	if groupCfg, err := config.LoadGroupsConfig(s.ConfigDir); err == nil {
		state.GroupsKnown = true
		state.Groups = groupConfigNames(groupCfg)
	}

	return config.CleanupAppConfigFile(s.ConfigDir, state)
}

func groupConfigNames(cfg config.GroupConfig) []string {
	if len(cfg.Groups) > 0 {
		names := make([]string, 0, len(cfg.Groups))
		for _, g := range cfg.Groups {
			names = append(names, g.Name)
		}
		return names
	}

	names := []string{cfg.GroupNames.Proxy, cfg.GroupNames.Auto}
	for region := range cfg.Regions {
		names = append(names, region)
	}
	if cfg.GroupNames.Other != "" {
		names = append(names, cfg.GroupNames.Other)
	}
	return names
}

func mainProxyGroupName(cfg config.GroupConfig, groupList []groups.ProxyGroup) string {
	if cfg.GroupNames.Proxy != "" {
		return cfg.GroupNames.Proxy
	}
	for _, group := range groupList {
		if len(group.Use) == 0 && group.Type == "select" {
			return group.Name
		}
	}
	for _, group := range groupList {
		if len(group.Use) == 0 {
			return group.Name
		}
	}
	return ""
}

func (s Service) ReadManagedNodes() ([]config.ManagedNode, error) {
	if s.DB != nil {
		dbNodes, err := s.DB.ListNodeDB()
		if err == nil {
			out := make([]config.ManagedNode, len(dbNodes))
			for i, n := range dbNodes {
				out[i] = config.ManagedNode{ID: n.NodeID, Name: n.Name, Address: n.Address, Port: n.Port, ServerID: n.ServerID}
			}
			return out, nil
		}
	}
	return config.LoadManagedNodes(s.ConfigDir)
}

func (s Service) UpdateManagedNodes(next []config.ManagedNode) error {
	if s.DB != nil {
		dbNodes := make([]db.Node2, len(next))
		validIDs := make([]string, 0, len(next))
		for i, n := range next {
			dbNodes[i] = db.Node2{NodeID: n.ID, Name: n.Name, Address: n.Address, Port: n.Port, ServerID: n.ServerID}
			validIDs = append(validIDs, n.ID)
		}
		if err := s.DB.ReplaceNodes(dbNodes); err != nil {
			return err
		}
		return s.DB.CleanupNodePrefs(validIDs)
	}
	return config.SaveManagedNodes(s.ConfigDir, next)
}

func (s Service) ReadProviders() ([]config.ProviderAddon, error) {
	if s.DB != nil {
		if providers, _, err := s.DB.LoadProvidersFromDB(); err != nil {
			return nil, err
		} else {
			return providers, nil
		}
	}
	return config.LoadProviders(s.ConfigDir)
}

func (s Service) UpsertProvider(provider config.ProviderAddon, publicBaseURL string) (config.ProviderAddon, error) {
	if s.DB != nil {
		providers, _, err := s.DB.LoadProvidersFromDB()
		if err != nil {
			return config.ProviderAddon{}, err
		}
		var existing *config.ProviderAddon
		for i := range providers {
			if providers[i].ID == provider.ID {
				existing = &providers[i]
				break
			}
		}
		saved, err := config.PrepareProvider(provider, publicBaseURL, existing)
		if err != nil {
			return config.ProviderAddon{}, err
		}
		if err := s.DB.UpsertProviderToDB(saved); err != nil {
			return config.ProviderAddon{}, err
		}
		return saved, nil
	}
	return config.UpsertProvider(s.ConfigDir, provider, publicBaseURL)
}

func (s Service) DeleteProvider(id string) error {
	if s.DB != nil {
		if err := s.DB.DeleteProviderFromDB(id); err != nil {
			return err
		}
		if err := s.DB.CleanupProviderPrefs(id); err != nil {
			return err
		}
		_ = config.RemoveProviderFile(s.ConfigDir, id)
		return nil
	}
	if err := config.DeleteProvider(s.ConfigDir, id); err != nil {
		return err
	}
	return s.cleanupAppConfig()
}

func (s Service) RefreshProvider(id string) (config.ProviderRefreshResult, error) {
	if s.DB != nil {
		providers, _, err := s.DB.LoadProvidersFromDB()
		if err != nil {
			return config.ProviderRefreshResult{}, err
		}
		for _, provider := range providers {
			if provider.ID != id {
				continue
			}
			updated, result, refreshErr := config.RefreshProviderAddon(s.ConfigDir, provider)
			_ = s.DB.UpsertProviderToDB(updated)
			if refreshErr != nil {
				log.Printf("[provider] refresh %s failed: %v", id, refreshErr)
				return result, refreshErr
			}
			log.Printf("[provider] refresh %s ok: %d proxies", id, result.Count)
			return result, nil
		}
		return config.ProviderRefreshResult{}, fmt.Errorf("provider %q not found", id)
	}
	result, err := config.RefreshProvider(s.ConfigDir, id)
	if err != nil {
		log.Printf("[provider] refresh %s failed: %v", id, err)
		return result, err
	}
	log.Printf("[provider] refresh %s ok: %d proxies", id, result.Count)
	return result, nil
}

func (s Service) ReadProviderFile(id string) ([]byte, error) {
	return config.ReadProviderFile(s.ConfigDir, id)
}

func (s Service) StartProviderUpdater(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		s.refreshDueProviders(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.refreshDueProviders(ctx)
			}
		}
	}()
}

func (s Service) StartTrafficRefresher(ctx context.Context) {
	go startTrafficSchedulerLoop(ctx, time.Minute, func() {
		refreshed, resets := s.RunTrafficMaintenance(ctx)
		if refreshed > 0 || resets > 0 {
			log.Printf("[traffic] maintenance refreshed=%d reset=%d", refreshed, resets)
		}
	})
}

func startTrafficSchedulerLoop(ctx context.Context, interval time.Duration, refresh func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}

func (s Service) RunTrafficMaintenance(ctx context.Context) (int, int) {
	if s.DB == nil {
		refreshed := s.RefreshTraffic(ctx)
		return len(refreshed), 0
	}

	servers, err := s.DB.ListServers()
	if err != nil {
		return 0, 0
	}

	now := time.Now()
	refreshed := 0
	resets := 0
	for _, sv := range servers {
		if !sv.Enabled {
			continue
		}

		if shouldResetServerTraffic(now, sv) {
			result, err := s.resetAllServerTraffic(ctx, sv)
			if err != nil {
				log.Printf("[traffic] auto_reset server=%s error=%v", sv.Name, err)
			} else {
				log.Printf("[traffic] auto_reset server=%s reset=%d", sv.Name, result.Reset)
				resets++
			}
		}

		if shouldSyncServerTraffic(now.Unix(), sv) {
			if _, err := s.refreshTrafficForServer(ctx, sv); err != nil {
				log.Printf("[traffic] auto_sync server=%s error=%v", sv.Name, err)
				continue
			}
			refreshed++
		}
	}
	return refreshed, resets
}

func (s Service) refreshDueProviders(ctx context.Context) {
	if s.DB == nil {
		config.RefreshDueProviders(s.ConfigDir)
		return
	}
	providers, err := s.ReadProviders()
	if err != nil {
		return
	}
	now := time.Now().Unix()
	for _, provider := range providers {
		if provider.SourceURL == "" {
			continue
		}
		interval := provider.UpdateIntervalSeconds
		if interval <= 0 {
			interval = 3600
		}
		if provider.LastUpdatedAt > 0 && now-provider.LastUpdatedAt < int64(interval) {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
			_, _ = s.RefreshProvider(provider.ID)
		}
	}
}

func (s Service) DetectAvailableNodes() ([]xui.AvailableNode, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return nil, err
	}
	return s.resolver(bundle.App).ListAvailableNodes(context.Background())
}

func (s Service) ReadTemplateSectionYAML(section string) (string, error) {
	return config.LoadTemplateSectionYAML(s.ConfigDir, section)
}

func (s Service) UpdateTemplateSectionYAML(section, raw string) error {
	return config.SaveTemplateSectionYAML(s.ConfigDir, section, raw)
}

func (s Service) TestConnection(ctx context.Context) (xui.ConnectionStatus, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return xui.ConnectionStatus{OK: false, Message: err.Error()}, err
	}
	return s.resolver(bundle.App).TestConnection(ctx)
}

func (s Service) loadBundle() (config.Bundle, error) {
	if s.Loader == nil {
		return config.Bundle{}, fmt.Errorf("config loader is not configured")
	}
	if s.DB != nil {
		var bundle config.Bundle
		appCfg, _, err := s.DB.LoadAppConfigFromDB(defaultAppConfig())
		if err != nil {
			return config.Bundle{}, err
		}
		groupCfg, _, err := s.DB.LoadGroupsConfigFromDB()
		if err != nil {
			return config.Bundle{}, err
		}
		bundle.App = appCfg
		bundle.Groups = groupCfg
		bundle.App.XUI = s.mergeXUI(bundle.App.XUI)
		return bundle, nil
	}
	bundle, err := s.Loader(s.ConfigDir)
	if err != nil {
		return config.Bundle{}, fmt.Errorf("load config bundle: %w", err)
	}
	bundle.App.XUI = s.mergeXUI(bundle.App.XUI)
	return bundle, nil
}

func defaultAppConfig() config.AppConfig {
	return config.AppConfig{
		Mode:                       "whitelist",
		CacheTTLSeconds:            60,
		HealthcheckURL:             "https://www.gstatic.com/generate_204",
		HealthcheckIntervalSeconds: 300,
		UserModes:                  map[string]string{},
		UserNodes:                  map[string][]string{},
		UserProviders:              map[string][]string{},
		UserGroupNodes:             map[string]map[string][]string{},
		UserGroupModes:             map[string]map[string]string{},
	}
}

func (s Service) resolver(appConfig config.AppConfig) XUIResolver {
	factory := s.ResolverFactory
	if factory == nil {
		factory = func(cfg config.XUIConfig) XUIResolver {
			return xui.NewResolver(xui.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Username, cfg.Password))
		}
	}
	return factory(s.mergeXUI(appConfig.XUI))
}

func (s Service) mergeXUI(current config.XUIConfig) config.XUIConfig {
	merged := current
	if merged.BaseURL == "" {
		merged.BaseURL = s.DefaultXUI.BaseURL
	}
	if merged.APIKey == "" {
		merged.APIKey = s.DefaultXUI.APIKey
	}
	if merged.Username == "" {
		merged.Username = s.DefaultXUI.Username
	}
	if merged.Password == "" {
		merged.Password = s.DefaultXUI.Password
	}
	return merged
}

func (s Service) resolveMode(appConfig config.AppConfig, user string) string {
	if appConfig.UserModes != nil {
		if mode, ok := appConfig.UserModes[user]; ok && (mode == "whitelist" || mode == "blacklist") {
			return mode
		}
	}
	if appConfig.Mode == "blacklist" {
		return "blacklist"
	}
	return "whitelist"
}

func applyManagedNodes(templateNodes []xui.Node, managedNodes []config.ManagedNode, selected []string) []xui.Node {
	selectedSet := map[string]struct{}{}
	for _, id := range selected {
		selectedSet[id] = struct{}{}
	}
	if len(selected) == 0 {
		for _, node := range managedNodes {
			selectedSet[node.ID] = struct{}{}
		}
	}

	var expanded []xui.Node
	for _, managed := range managedNodes {
		if _, ok := selectedSet[managed.ID]; !ok {
			continue
		}
		if len(templateNodes) == 0 {
			continue
		}
		proxy := templateNodes[0]
		proxy.Name = managed.Name
		proxy.Server = managed.Address
		if managed.Port > 0 {
			proxy.Port = managed.Port
		}
		proxy.ID = managed.ID
		expanded = append(expanded, proxy)
	}

	if len(expanded) > 0 {
		return expanded
	}
	return templateNodes
}

// ─── Traffic ────────────────────────────────────────────────────────

// RefreshTraffic fetches live traffic from all servers, stores in DB, and returns per-user data.
func (s Service) RefreshTraffic(ctx context.Context) map[string][]db.ServerTraffic {
	if s.DB == nil {
		return map[string][]db.ServerTraffic{}
	}

	servers, err := s.DB.ListServers()
	if err != nil {
		return map[string][]db.ServerTraffic{}
	}

	var wg sync.WaitGroup
	for _, sv := range servers {
		if !sv.Enabled {
			continue
		}
		sv := sv
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.refreshTrafficForServer(ctx, sv); err != nil {
				log.Printf("[traffic] %s: %v", sv.Name, err)
			}
		}()
	}
	wg.Wait()

	return s.LoadTraffic(ctx)
}

func (s Service) refreshTrafficForServer(ctx context.Context, sv db.Server) ([]db.UserTraffic, error) {
	start := time.Now()
	requestCtx, cancel := context.WithTimeout(ctx, trafficRefreshRequestTimeout)
	defer cancel()

	cli := xui.NewClient(s.xuiURL(&sv), sv.APIKey, "", "")
	clients, err := cli.ListClients(requestCtx)
	if err != nil {
		return nil, fmt.Errorf("refresh timeout_or_fetch_error after %s: %w", time.Since(start).Round(time.Millisecond), err)
	}

	entries := make([]db.UserTraffic, 0, len(clients))
	for _, client := range clients {
		if client.Email == "" {
			continue
		}
		entries = append(entries, db.UserTraffic{
			Email:    client.Email,
			ServerID: sv.ID,
			Up:       client.Traffic.Up,
			Down:     client.Traffic.Down,
		})
	}

	if err := s.DB.ReplaceUserTrafficForServer(sv.ID, entries); err != nil {
		return nil, fmt.Errorf("store traffic: %w", err)
	}
	if err := s.DB.UpdateServerTrafficSyncAt(sv.ID, time.Now().Unix()); err != nil {
		log.Printf("[traffic] update sync marker server=%s: %v", sv.Name, err)
	}
	log.Printf("[traffic] refreshed server=%s clients=%d duration=%s", sv.Name, len(entries), time.Since(start).Round(time.Millisecond))
	return entries, nil
}

// LoadTraffic returns stored traffic data (without fetching from servers).
func (s Service) LoadTraffic(ctx context.Context) map[string][]db.ServerTraffic {
	result := map[string][]db.ServerTraffic{}
	all, err := s.DB.GetAllUserTraffic()
	if err != nil {
		return result
	}
	for _, t := range all {
		sv, err := s.DB.GetServer(t.ServerID)
		if err != nil {
			continue
		}
		key := t.Email
		result[key] = append(result[key], db.ServerTraffic{
			ServerID:      sv.ID,
			ServerName:    sv.Name,
			ServerAddress: sv.Host,
			Up:            t.Up,
			Down:          t.Down,
		})
	}
	return result
}

// ─── DB-backed user management ──────────────────────────────────────

type TrafficResetResult struct {
	Server  string   `json:"server"`
	Reset   int      `json:"reset"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

func (s Service) ResetServerUserTraffic(ctx context.Context, serverID int64) (*TrafficResetResult, error) {
	sv, err := s.DB.GetServer(serverID)
	if err != nil {
		return nil, fmt.Errorf("get server: %w", err)
	}
	return s.resetAllServerTraffic(ctx, *sv)
}

func (s Service) resetAllServerTraffic(ctx context.Context, sv db.Server) (*TrafficResetResult, error) {
	cli := xui.NewClient(s.xuiURL(&sv), sv.APIKey, "", "")
	clients, err := cli.ListClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	result := &TrafficResetResult{Server: sv.Name}
	for _, client := range clients {
		if client.Email == "" {
			result.Skipped++
			continue
		}
		result.Reset++
	}
	if err := cli.ResetAllClientTraffics(ctx); err != nil {
		return nil, fmt.Errorf("reset all client traffic: %w", err)
	}
	if err := s.DB.ZeroAllUserTrafficForServer(sv.ID); err != nil {
		return nil, fmt.Errorf("zero cached traffic: %w", err)
	}
	if err := s.DB.UpdateServerTrafficSyncAt(sv.ID, time.Now().Unix()); err != nil {
		log.Printf("[traffic] update sync marker after reset server=%s: %v", sv.Name, err)
	}
	if sv.AutoResetTrafficEnabled {
		if err := s.DB.UpdateServerTrafficResetKey(sv.ID, scheduledResetKey(time.Now(), sv.AutoResetTimezone)); err != nil {
			log.Printf("[traffic] update reset marker server=%s: %v", sv.Name, err)
		}
	}
	return result, nil
}

func shouldSyncServerTraffic(nowUnix int64, sv db.Server) bool {
	interval := sv.TrafficSyncIntervalMinutes
	if interval <= 0 {
		interval = 60
	}
	if sv.LastTrafficSyncAt <= 0 {
		return true
	}
	return nowUnix-sv.LastTrafficSyncAt >= int64(interval*60)
}

func shouldResetServerTraffic(now time.Time, sv db.Server) bool {
	if !sv.AutoResetTrafficEnabled {
		return false
	}
	loc, err := time.LoadLocation(sv.AutoResetTimezone)
	if err != nil {
		loc = time.Local
	}
	localNow := now.In(loc)
	scheduledDay := clampDay(localNow.Year(), localNow.Month(), sv.AutoResetDay)
	scheduledAt := time.Date(localNow.Year(), localNow.Month(), scheduledDay, sv.AutoResetHour, sv.AutoResetMinute, 0, 0, loc)
	if localNow.Before(scheduledAt) {
		return false
	}
	return sv.LastTrafficResetKey != scheduledResetKey(now, sv.AutoResetTimezone)
}

func scheduledResetKey(now time.Time, timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}
	localNow := now.In(loc)
	return fmt.Sprintf("%04d-%02d", localNow.Year(), int(localNow.Month()))
}

func clampDay(year int, month time.Month, day int) int {
	if day <= 0 {
		day = 1
	}
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		return lastDay
	}
	return day
}

func (s Service) DBUserSearch(query string) ([]db.User, error) {
	if query == "" {
		return s.DB.ListUsers()
	}
	return s.DB.SearchUsers(query)
}

func (s Service) DBUserList() ([]db.User, error) {
	return s.DB.ListUsers()
}

func (s Service) DBCreateUser(u *db.User) error {
	return s.DB.CreateUser(u)
}

func (s Service) DBUpdateUser(u *db.User) error {
	return s.DB.UpdateUser(u)
}

func (s Service) DBDeleteUser(id int64) error {
	if err := s.DB.DeleteUser(id); err != nil {
		return err
	}
	return nil
}

func (s Service) DBListServers() ([]db.Server, error) {
	return s.DB.ListServers()
}

func (s Service) DBListNodeDB() ([]db.Node2, error) {
	return s.DB.ListNodeDB()
}

func (s Service) DBReplaceNodes(nodes []db.Node2) error {
	if err := s.DB.ReplaceNodes(nodes); err != nil {
		return err
	}
	validIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		validIDs = append(validIDs, n.NodeID)
	}
	return s.DB.CleanupNodePrefs(validIDs)
}

func (s Service) DBCreateServer(sv *db.Server) error {
	return s.DB.CreateServer(sv)
}

func (s Service) DBUpdateServer(sv *db.Server) error {
	return s.DB.UpdateServer(sv)
}

func (s Service) DBDeleteServer(id int64) error {
	return s.DB.DeleteServer(id)
}

func (s Service) DBGetUserProtocols(userID int64) (string, int) {
	assignments, err := s.DB.ListAssignmentsByUser(userID)
	if err != nil || len(assignments) == 0 {
		return "", 0
	}
	seen := map[string]bool{}
	labels := make([]string, 0, len(assignments))
	for _, a := range assignments {
		inb, err := s.DB.GetInbound(a.InboundID)
		if err != nil {
			continue
		}
		label := protocolLabel(inb.Protocol, inb.StreamSettingsJSON)
		if !seen[label] {
			seen[label] = true
			labels = append(labels, label)
		}
	}
	count := len(assignments)
	joined := ""
	if len(labels) > 0 {
		joined = strings.Join(labels, ", ")
	}
	return joined, count
}

func (s Service) xuiURL(sv *db.Server) string {
	base := strings.TrimRight(sv.BasePath, "/")
	return fmt.Sprintf("%s://%s:%d%s", sv.Scheme, sv.Host, sv.Port, base)
}

// ─── Import ─────────────────────────────────────────────────────────

func (s Service) ImportFromServer(ctx context.Context, serverID int64) (*ImportResult, error) {
	sv, err := s.DB.GetServer(serverID)
	if err != nil {
		return nil, fmt.Errorf("get server: %w", err)
	}

	// Save all DB users BEFORE import to detect deletions
	allDBUsers, _ := s.DB.ListUsers()
	beforeEmails := map[string]bool{}
	for _, u := range allDBUsers {
		beforeEmails[u.Email] = true
	}

	cli := xui.NewClient(s.xuiURL(sv), sv.APIKey, "", "")
	log.Printf("[import] fetching inbounds from %s (%s)", sv.Name, sv.Host)
	inbounds, err := cli.ListInbounds(ctx)
	if err != nil {
		return nil, fmt.Errorf("list inbounds: %w", err)
	}
	log.Printf("[import] got %d inbounds, %d users in DB before import", len(inbounds), len(beforeEmails))

	cachedList, err := s.cacheServerInbounds(serverID, inbounds)
	if err != nil {
		return nil, fmt.Errorf("cache inbounds: %w", err)
	}
	cachedByAPIID := inboundsByAPIID(cachedList)
	if err := s.DB.DeleteAssignmentsByServer(serverID); err != nil {
		return nil, fmt.Errorf("clear assignments: %w", err)
	}

	seen := map[string]*db.User{}
	imported := 0
	updated := 0

	for _, inb := range inbounds {
		settings, ok := xui.ParseInboundSettings(inb.Settings)
		if !ok {
			continue
		}
		for _, cl := range settings.Clients {
			if cl.Email == "" {
				continue
			}
			existing, err := s.DB.GetUserByEmail(cl.Email)
			if err != nil {
				u := &db.User{
					Email:      cl.Email,
					UUID:       cl.ID,
					Password:   cl.Password,
					Auth:       cl.Auth,
					Flow:       cl.Flow,
					Security:   cl.Security,
					TotalGB:    cl.TotalGB,
					ExpiryTime: cl.ExpiryTime,
					LimitIP:    cl.LimitIP,
					SubID:      cl.SubID,
					TgID:       int64(cl.TgID),
					Reset:      cl.Reset,
					Comment:    cl.Comment,
					Enable:     true,
				}
				if err := s.DB.CreateUser(u); err != nil {
					return nil, fmt.Errorf("create user %s: %w", cl.Email, err)
				}
				seen[cl.Email] = u
				imported++
			} else {
				if applyImportedClient(existing, cl) {
					if err := s.DB.UpdateUser(existing); err != nil {
						return nil, fmt.Errorf("update user %s: %w", cl.Email, err)
					}
					updated++
				}
				seen[cl.Email] = existing
			}
		}
	}

	for email, u := range seen {
		for _, inb := range inbounds {
			settings, ok := xui.ParseInboundSettings(inb.Settings)
			if !ok {
				continue
			}
			for _, cl := range settings.Clients {
				if cl.Email == email {
					if ci, ok := cachedByAPIID[int(inb.ID)]; ok {
						s.DB.CreateAssignment(&db.UserAssignment{
							UserID:        u.ID,
							ServerID:      serverID,
							InboundID:     ci.ID,
							EmailOnServer: email,
							Enable:        cl.Enable,
						})
					}
				}
			}
		}
	}

	// Remove users that exist in DB but no longer on the main server
	alive := map[string]bool{}
	for e := range seen {
		alive[e] = true
	}
	var removed []string
	for email := range beforeEmails {
		if alive[email] {
			continue
		}
		u, err := s.DB.GetUserByEmail(email)
		if err != nil {
			continue
		}
		log.Printf("[import] user %s not found on main server, removing from DB", email)
		s.DB.DeleteUser(u.ID)
		removed = append(removed, email)
	}
	if len(removed) > 0 {
		log.Printf("[import] removed %d users from local database", len(removed))
	}

	log.Printf("[import] done: %d new, %d updated, %d removed", imported, updated, len(removed))
	return &ImportResult{Imported: imported, Updated: updated, Total: len(seen), Removed: removed}, nil
}

// ─── Sync ───────────────────────────────────────────────────────────

func applyImportedClient(user *db.User, client xui.InboundClient) bool {
	changed := false
	setString := func(target *string, value string) {
		if *target != value {
			*target = value
			changed = true
		}
	}
	setInt := func(target *int, value int) {
		if *target != value {
			*target = value
			changed = true
		}
	}
	setInt64 := func(target *int64, value int64) {
		if *target != value {
			*target = value
			changed = true
		}
	}

	setString(&user.UUID, client.ID)
	setString(&user.Password, client.Password)
	setString(&user.Auth, client.Auth)
	setString(&user.Flow, client.Flow)
	setString(&user.Security, client.Security)
	setInt64(&user.TotalGB, client.TotalGB)
	setInt64(&user.ExpiryTime, client.ExpiryTime)
	setInt(&user.LimitIP, client.LimitIP)
	setString(&user.SubID, client.SubID)
	setInt64(&user.TgID, int64(client.TgID))
	setInt(&user.Reset, client.Reset)
	setString(&user.Comment, client.Comment)

	return changed
}

func (s Service) SyncToServers(ctx context.Context) (*SyncResult, error) {
	servers, err := s.DB.ListServers()
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	result := &SyncResult{}
	log.Printf("[sync] found %d servers", len(servers))

	var mainServer *db.Server

	for i := range servers {
		sv := servers[i]
		if sv.IsMain && sv.Enabled {
			mainServer = &servers[i]
			log.Printf("[sync] auto-import from main server %s", sv.Name)
			imported, err := s.ImportFromServer(ctx, sv.ID)
			if err != nil {
				log.Printf("[sync] import error: %v", err)
			} else {
				log.Printf("[sync] import: %d new, %d updated, %d total, %d removed",
					imported.Imported, imported.Updated, imported.Total, len(imported.Removed))
			}
			break
		}
	}
	if mainServer == nil {
		return result, fmt.Errorf("no enabled main server configured")
	}

	users, err := s.DB.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	log.Printf("[sync] %d users in DB", len(users))

	mainInbounds, err := s.DB.ListInboundsByServer(mainServer.ID)
	if err != nil {
		return nil, fmt.Errorf("list main inbounds: %w", err)
	}
	mainByLocalID := inboundsByLocalID(mainInbounds)
	mainAssignments, err := s.DB.ListAssignmentsByServer(mainServer.ID)
	if err != nil {
		return nil, fmt.Errorf("list main assignments: %w", err)
	}
	desiredByEmail := desiredInboundTagsByEmail(mainAssignments, mainByLocalID)
	localByEmail := usersByEmail(users)

	for _, sv := range servers {
		if sv.IsMain || !sv.Enabled {
			continue
		}
		log.Printf("[sync] processing %s (%s)", sv.Name, sv.Host)

		cli := xui.NewClient(s.xuiURL(&sv), sv.APIKey, "", "")
		existingInbounds, err := cli.ListInbounds(ctx)
		if err != nil {
			log.Printf("[sync] list inbounds failed for %s: %v", sv.Name, err)
			result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: err.Error()})
			continue
		}
		log.Printf("[sync] %s: %d inbounds", sv.Name, len(existingInbounds))
		syncedInbounds, err := s.syncServerInbounds(ctx, cli, sv, mainInbounds, existingInbounds)
		if err != nil {
			log.Printf("[sync] sync inbounds for %s: %v", sv.Name, err)
			result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: err.Error()})
			continue
		}

		existingClients, existingAttachments, err := s.collectServerClients(ctx, cli, syncedInbounds)
		if err != nil {
			log.Printf("[sync] list clients for %s failed, falling back to inbound clients: %v", sv.Name, err)
		}
		log.Printf("[sync] %s: %d existing users", sv.Name, len(existingClients))
		targetByTag := dbInboundsByTag(syncedInbounds)

		for _, u := range users {
			tags := desiredByEmail[u.Email]
			if len(tags) == 0 {
				continue
			}
			desiredInboundIDs := inboundIDsForTags(tags, targetByTag)
			if len(desiredInboundIDs) == 0 {
				log.Printf("[sync] %s: no matching target inbounds for %s", sv.Name, u.Email)
				continue
			}
			protocol := protocolForFirstTag(tags, targetByTag)
			client := buildXUIClient(protocol, u)
			if existing, ok := existingClients[u.Email]; ok {
				client.Enable = existing.Enable
				if !sameRemoteClient(existing, client) {
					if err := cli.UpdateClientByEmail(ctx, u.Email, client); err != nil {
						result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("update %s: %v", u.Email, err)})
						continue
					}
					result.Updated++
				}
				add, remove := diffInboundAttachments(existingAttachments[u.Email], desiredInboundIDs)
				if err := cli.AttachClient(ctx, u.Email, add); err != nil {
					result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("attach %s: %v", u.Email, err)})
					continue
				}
				result.Attached += len(add)
				if err := cli.DetachClient(ctx, u.Email, remove); err != nil {
					result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("detach %s: %v", u.Email, err)})
					continue
				}
				result.Detached += len(remove)
			} else if err := cli.CreateClient(ctx, client, desiredInboundIDs); err != nil {
				if !isEmailAlreadyInUse(err) {
					result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("create %s: %v", u.Email, err)})
					continue
				}
				log.Printf("[sync] %s already exists on %s, updating and attaching", u.Email, sv.Name)
				if err := cli.UpdateClientByEmail(ctx, u.Email, client); err != nil {
					result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("update existing %s: %v", u.Email, err)})
					continue
				}
				result.Updated++
				if err := cli.AttachClient(ctx, u.Email, desiredInboundIDs); err != nil {
					result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("attach existing %s: %v", u.Email, err)})
					continue
				}
				result.Attached += len(desiredInboundIDs)
			} else {
				result.Added++
			}
			result.Synced++
		}

		for email := range existingClients {
			if _, ok := localByEmail[email]; ok {
				if len(desiredByEmail[email]) > 0 {
					continue
				}
			}
			log.Printf("[sync] deleting %s from %s", email, sv.Name)
			if err := cli.DeleteClientByEmailV2(ctx, email); err != nil {
				result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("delete %s: %v", email, err)})
				continue
			}
			result.Deleted++
		}

		refetched, err := cli.ListInbounds(ctx)
		if err == nil {
			_, _ = s.cacheServerInbounds(sv.ID, refetched)
			_ = s.rebuildAssignmentsFromRemote(sv.ID, refetched, localByEmail)
		}
	}

	return result, nil
}

// ─── Preview & Generate helpers ─────────────────────────────────────

func (s Service) cacheServerInbounds(serverID int64, inbounds []xui.InboundRecord) ([]db.Inbound, error) {
	cached := make([]db.Inbound, 0, len(inbounds))
	for _, inb := range inbounds {
		trafficJSON := ""
		if len(inb.ClientStats) > 0 {
			if b, err := json.Marshal(inb.ClientStats); err == nil {
				trafficJSON = string(b)
			}
		}
		cached = append(cached, db.Inbound{
			InboundID:          int(inb.ID),
			Remark:             inb.Remark,
			Listen:             inb.Listen,
			Port:               inb.Port,
			Protocol:           inb.Protocol,
			Total:              inb.Total,
			ExpiryTime:         inb.ExpiryTime,
			TrafficReset:       inb.TrafficReset,
			SettingsJSON:       inb.Settings,
			StreamSettingsJSON: inb.StreamSettings,
			SniffingJSON:       inb.Sniffing,
			Tag:                inb.Tag,
			Enable:             inb.Enable,
			TrafficJSON:        trafficJSON,
		})
	}
	if err := s.DB.EnsureServerInbounds(serverID, cached); err != nil {
		return nil, err
	}
	return s.DB.ListInboundsByServer(serverID)
}

func inboundsByAPIID(inbounds []db.Inbound) map[int]db.Inbound {
	out := make(map[int]db.Inbound, len(inbounds))
	for _, inb := range inbounds {
		out[inb.InboundID] = inb
	}
	return out
}

func inboundsByLocalID(inbounds []db.Inbound) map[int64]db.Inbound {
	out := make(map[int64]db.Inbound, len(inbounds))
	for _, inb := range inbounds {
		out[inb.ID] = inb
	}
	return out
}

func dbInboundsByTag(inbounds []db.Inbound) map[string]db.Inbound {
	out := make(map[string]db.Inbound, len(inbounds))
	for _, inb := range inbounds {
		key := inboundSyncKey(inb)
		if key != "" {
			out[key] = inb
		}
	}
	return out
}

func desiredInboundTagsByEmail(assignments []db.UserAssignment, inbounds map[int64]db.Inbound) map[string][]string {
	out := map[string][]string{}
	for _, a := range assignments {
		if !a.Enable {
			continue
		}
		inb, ok := inbounds[a.InboundID]
		if !ok {
			continue
		}
		key := inboundSyncKey(inb)
		if key == "" {
			continue
		}
		out[a.EmailOnServer] = appendUniqueString(out[a.EmailOnServer], key)
	}
	return out
}

func usersByEmail(users []db.User) map[string]db.User {
	out := make(map[string]db.User, len(users))
	for _, u := range users {
		out[u.Email] = u
	}
	return out
}

func inboundSyncKey(inb db.Inbound) string {
	if strings.TrimSpace(inb.Tag) != "" {
		return strings.TrimSpace(inb.Tag)
	}
	return fmt.Sprintf("%s|%s|%d|%s", inb.Protocol, inb.Listen, inb.Port, inb.Remark)
}

func inboundRecordSyncKey(inb xui.InboundRecord) string {
	if strings.TrimSpace(inb.Tag) != "" {
		return strings.TrimSpace(inb.Tag)
	}
	return fmt.Sprintf("%s|%s|%d|%s", inb.Protocol, inb.Listen, inb.Port, inb.Remark)
}

func appendUniqueString(list []string, value string) []string {
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}

func inboundIDsForTags(tags []string, inbounds map[string]db.Inbound) []int {
	out := []int{}
	for _, tag := range tags {
		if inb, ok := inbounds[tag]; ok && inb.Enable {
			out = append(out, inb.InboundID)
		}
	}
	return out
}

func protocolForFirstTag(tags []string, inbounds map[string]db.Inbound) string {
	for _, tag := range tags {
		if inb, ok := inbounds[tag]; ok {
			return inb.Protocol
		}
	}
	return ""
}

func buildXUIClient(protocol string, u db.User) xui.InboundClient {
	return xui.BuildClientConfig(protocol, xui.ClientParams{
		Email:      u.Email,
		UUID:       u.UUID,
		Password:   u.Password,
		Auth:       u.Auth,
		Flow:       u.Flow,
		Security:   u.Security,
		TotalGB:    u.TotalGB,
		ExpiryTime: u.ExpiryTime,
		LimitIP:    u.LimitIP,
		SubID:      u.SubID,
		TgID:       u.TgID,
		Reset:      u.Reset,
		Comment:    u.Comment,
		Enable:     u.Enable,
	})
}

func collectRemoteClients(inbounds []db.Inbound) (map[string]xui.ClientListRecord, map[string][]int) {
	clients := map[string]xui.ClientListRecord{}
	attachments := map[string][]int{}
	for _, inb := range inbounds {
		settings, ok := xui.ParseInboundSettings(inb.SettingsJSON)
		if !ok {
			continue
		}
		for _, cl := range settings.Clients {
			if cl.Email == "" {
				continue
			}
			clients[cl.Email] = xui.ClientListRecord{
				Email:      cl.Email,
				UUID:       cl.ID,
				Password:   cl.Password,
				Auth:       cl.Auth,
				Flow:       cl.Flow,
				Security:   cl.Security,
				LimitIP:    cl.LimitIP,
				TotalGB:    cl.TotalGB,
				ExpiryTime: cl.ExpiryTime,
				Enable:     cl.Enable,
				TgID:       int64(cl.TgID),
				SubID:      cl.SubID,
				Reset:      cl.Reset,
				Comment:    cl.Comment,
			}
			attachments[cl.Email] = appendUniqueInt(attachments[cl.Email], inb.InboundID)
		}
	}
	return clients, attachments
}

func (s Service) collectServerClients(ctx context.Context, cli *xui.Client, inbounds []db.Inbound) (map[string]xui.ClientListRecord, map[string][]int, error) {
	rows, err := cli.ListClients(ctx)
	if err != nil {
		clients, attachments := collectRemoteClients(inbounds)
		return clients, attachments, err
	}
	clients := map[string]xui.ClientListRecord{}
	attachments := map[string][]int{}
	for _, row := range rows {
		if row.Email == "" {
			continue
		}
		clients[row.Email] = row
		for _, inboundID := range row.InboundIDs {
			attachments[row.Email] = appendUniqueInt(attachments[row.Email], inboundID)
		}
	}
	return clients, attachments, nil
}

func appendUniqueInt(list []int, value int) []int {
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}

func diffInboundAttachments(current, desired []int) ([]int, []int) {
	currentSet := map[int]bool{}
	desiredSet := map[int]bool{}
	for _, id := range current {
		currentSet[id] = true
	}
	for _, id := range desired {
		desiredSet[id] = true
	}
	add := []int{}
	remove := []int{}
	for _, id := range desired {
		if !currentSet[id] {
			add = append(add, id)
		}
	}
	for _, id := range current {
		if !desiredSet[id] {
			remove = append(remove, id)
		}
	}
	return add, remove
}

func sameRemoteClient(existing xui.ClientListRecord, desired xui.InboundClient) bool {
	return existing.Email == desired.Email &&
		existing.UUID == desired.ID &&
		existing.Password == desired.Password &&
		existing.Auth == desired.Auth &&
		existing.Flow == desired.Flow &&
		existing.Security == desired.Security &&
		existing.SubID == desired.SubID &&
		existing.LimitIP == desired.LimitIP &&
		existing.TotalGB == desired.TotalGB &&
		existing.ExpiryTime == desired.ExpiryTime &&
		existing.TgID == int64(desired.TgID) &&
		existing.Reset == desired.Reset &&
		existing.Comment == desired.Comment
}

func isEmailAlreadyInUse(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "email already in use")
}

func dbInboundToXUI(inb db.Inbound) xui.InboundRecord {
	return xui.InboundRecord{
		ID:             int64(inb.InboundID),
		Total:          inb.Total,
		Remark:         inb.Remark,
		Enable:         inb.Enable,
		ExpiryTime:     inb.ExpiryTime,
		TrafficReset:   inb.TrafficReset,
		Listen:         inb.Listen,
		Port:           inb.Port,
		Protocol:       inb.Protocol,
		Settings:       inb.SettingsJSON,
		StreamSettings: inb.StreamSettingsJSON,
		Sniffing:       inb.SniffingJSON,
		Tag:            inb.Tag,
	}
}

func inboundEquivalent(a db.Inbound, b xui.InboundRecord) bool {
	return a.Remark == b.Remark &&
		a.Listen == b.Listen &&
		a.Port == b.Port &&
		a.Protocol == b.Protocol &&
		a.Enable == b.Enable &&
		a.Total == b.Total &&
		a.ExpiryTime == b.ExpiryTime &&
		a.TrafficReset == b.TrafficReset &&
		settingsWithoutClients(a.SettingsJSON) == settingsWithoutClients(b.Settings) &&
		a.StreamSettingsJSON == b.StreamSettings &&
		a.SniffingJSON == b.Sniffing
}

func settingsWithoutClients(raw string) string {
	settings := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return raw
	}
	if _, ok := settings["clients"]; ok {
		settings["clients"] = []any{}
	}
	b, err := json.Marshal(settings)
	if err != nil {
		return raw
	}
	return string(b)
}

func (s Service) syncServerInbounds(ctx context.Context, cli *xui.Client, sv db.Server, mainInbounds []db.Inbound, remote []xui.InboundRecord) ([]db.Inbound, error) {
	remoteByKey := map[string]xui.InboundRecord{}
	for _, inb := range remote {
		key := inboundRecordSyncKey(inb)
		if key != "" {
			remoteByKey[key] = inb
		}
	}
	mainKeys := map[string]bool{}
	for _, main := range mainInbounds {
		key := inboundSyncKey(main)
		if key == "" {
			continue
		}
		mainKeys[key] = true
		remoteInb, ok := remoteByKey[key]
		record := dbInboundToXUI(main)
		if !ok {
			log.Printf("[sync] %s: creating inbound %s", sv.Name, main.Remark)
			if err := cli.AddInbound(ctx, record); err != nil {
				return nil, err
			}
			continue
		}
		if !inboundEquivalent(main, remoteInb) {
			log.Printf("[sync] %s: updating inbound %s", sv.Name, main.Remark)
			if err := cli.UpdateInbound(ctx, int(remoteInb.ID), record); err != nil {
				return nil, err
			}
		}
	}
	for key, remoteInb := range remoteByKey {
		if mainKeys[key] {
			continue
		}
		log.Printf("[sync] %s: deleting stale inbound %s", sv.Name, remoteInb.Remark)
		if err := cli.DeleteInbound(ctx, int(remoteInb.ID)); err != nil {
			return nil, err
		}
	}
	refetched, err := cli.ListInbounds(ctx)
	if err != nil {
		return nil, err
	}
	return s.cacheServerInbounds(sv.ID, refetched)
}

func (s Service) rebuildAssignmentsFromRemote(serverID int64, inbounds []xui.InboundRecord, localByEmail map[string]db.User) error {
	cached, err := s.cacheServerInbounds(serverID, inbounds)
	if err != nil {
		return err
	}
	byAPI := inboundsByAPIID(cached)
	if err := s.DB.DeleteAssignmentsByServer(serverID); err != nil {
		return err
	}
	for _, inb := range inbounds {
		cachedInb, ok := byAPI[int(inb.ID)]
		if !ok {
			continue
		}
		settings, ok := xui.ParseInboundSettings(inb.Settings)
		if !ok {
			continue
		}
		for _, cl := range settings.Clients {
			u, ok := localByEmail[cl.Email]
			if !ok {
				continue
			}
			_ = s.DB.CreateAssignment(&db.UserAssignment{
				UserID:        u.ID,
				ServerID:      serverID,
				InboundID:     cachedInb.ID,
				EmailOnServer: cl.Email,
				Enable:        cl.Enable,
			})
		}
	}
	return nil
}

func upsertClientInSettings(raw string, client xui.InboundClient) string {
	settings := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &settings)
	clients := decodeInboundClients(settings["clients"])
	replaced := false
	for i := range clients {
		if sameClient(clients[i], client) {
			clients[i] = client
			replaced = true
			break
		}
	}
	if !replaced {
		clients = append(clients, client)
	}
	settings["clients"] = clients
	if b, err := json.Marshal(settings); err == nil {
		return string(b)
	}
	return raw
}

func removeClientFromSettings(raw, email string) string {
	settings := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return raw
	}
	clients := decodeInboundClients(settings["clients"])
	filtered := clients[:0]
	for _, client := range clients {
		if !strings.EqualFold(client.Email, email) {
			filtered = append(filtered, client)
		}
	}
	settings["clients"] = filtered
	if b, err := json.Marshal(settings); err == nil {
		return string(b)
	}
	return raw
}

func decodeInboundClients(raw any) []xui.InboundClient {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var clients []xui.InboundClient
	_ = json.Unmarshal(data, &clients)
	return clients
}

func sameClient(a, b xui.InboundClient) bool {
	if a.Email != "" && b.Email != "" && strings.EqualFold(a.Email, b.Email) {
		return true
	}
	if a.ID != "" && b.ID != "" && a.ID == b.ID {
		return true
	}
	if a.Password != "" && b.Password != "" && a.Password == b.Password {
		return true
	}
	return false
}

func (s Service) dbResolveUserNodes(email string) ([]xui.Node, error) {
	user, err := s.DB.GetUserByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("user %q not found in DB", email)
	}
	assignments, err := s.DB.ListAssignmentsByUser(user.ID)
	if err != nil || len(assignments) == 0 {
		return nil, fmt.Errorf("user %q has no server assignments", email)
	}
	seen := map[int64]bool{}
	var nodes []xui.Node
	for _, a := range assignments {
		if !a.Enable || seen[a.InboundID] {
			continue
		}
		seen[a.InboundID] = true
		inb, err := s.DB.GetInbound(a.InboundID)
		if err != nil || !inb.Enable {
			continue
		}
		sv, _ := s.DB.GetServer(a.ServerID)
		host := ""
		if sv != nil {
			host = sv.Host
		}
		nodes = append(nodes, buildNodeFromCache(*inb, *user, host))
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no usable nodes for user %q", email)
	}
	return nodes, nil
}

func buildNodeFromCache(inb db.Inbound, u db.User, host string) xui.Node {
	stream := xui.ParseStreamSettings(inb.StreamSettingsJSON)
	server := host
	if server == "" {
		server = inb.Remark
	}
	node := xui.Node{
		ID:                fmt.Sprintf("%s|%s|%s|%d", inb.Protocol, inb.Remark, server, inb.Port),
		Name:              inb.Remark,
		Type:              inb.Protocol,
		Server:            server,
		Port:              inb.Port,
		UUID:              u.UUID,
		Password:          u.Password,
		Flow:              u.Flow,
		Network:           normalizeNet(stream.Network),
		TLS:               stream.Security == "tls" || stream.Security == "reality",
		UDP:               inb.Protocol == "vless",
		ServerName:        pickSNI(stream),
		ClientFingerprint: firstNonEmpty(stream.RealitySettings.Settings.Fingerprint, stream.TLSSettings.Fingerprint),
		RealityPublicKey:  stream.RealitySettings.Settings.PublicKey,
		RealityShortID:    firstOrEmpty(stream.RealitySettings.ShortIds),
		XHTTPPath:         stream.XHTTPSettings.Path,
		XHTTPHost:         stream.XHTTPSettings.Host,
		XHTTPMode:         stream.XHTTPSettings.Mode,
	}
	if stream.ALPN != nil {
		node.ALPN = stream.ALPN
	}
	return node
}

func pickHost(listen, fallback string) string {
	switch listen {
	case "", "0.0.0.0", "::", "127.0.0.1", "localhost":
		return fallback
	default:
		return listen
	}
}

func normalizeNet(network string) string {
	if network == "" || network == "tcp" {
		return "raw"
	}
	return network
}

func pickSNI(stream xui.InboundStreamSettings) string {
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (s Service) DBGetUserTraffic(userID int64) []db.ServerTraffic {
	assigns, err := s.DB.ListAssignmentsByUser(userID)
	if err != nil {
		return nil
	}
	serverCache := map[int64]*db.Server{} // cache lookups
	seen := map[string]bool{}
	var out []db.ServerTraffic
	for _, a := range assigns {
		inb, err := s.DB.GetInbound(a.InboundID)
		if err != nil || inb.TrafficJSON == "" {
			continue
		}
		sv, ok := serverCache[a.ServerID]
		if !ok {
			sv, err = s.DB.GetServer(a.ServerID)
			if err != nil {
				continue
			}
			serverCache[a.ServerID] = sv
		}
		key := sv.Name
		if seen[key] {
			continue // already have traffic for this server
		}
		seen[key] = true
		var stats []xui.ClientTraffic
		if err := json.Unmarshal([]byte(inb.TrafficJSON), &stats); err != nil {
			continue
		}
		for _, ct := range stats {
			if ct.Email == a.EmailOnServer {
				out = append(out, db.ServerTraffic{
					ServerName: sv.Name,
					Up:         ct.Up,
					Down:       ct.Down,
				})
				break
			}
		}
	}
	return out
}

func firstOrEmpty(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

type streamInfo struct {
	Network  string `json:"network"`
	Security string `json:"security"`
}

func protocolLabel(protocol, streamSettingsJSON string) string {
	if streamSettingsJSON == "" {
		return protocol
	}
	var si streamInfo
	if err := json.Unmarshal([]byte(streamSettingsJSON), &si); err != nil {
		return protocol
	}
	transport := ""
	switch si.Security {
	case "tls":
		transport = "tls"
	case "reality":
		transport = "reality"
	}
	network := si.Network
	if network == "" || network == "tcp" {
		network = ""
	}
	label := protocol
	if transport != "" && network != "" {
		return label + "+" + network + "+" + transport
	}
	if transport != "" {
		return label + "+" + transport
	}
	if network != "" {
		return label + "+" + network
	}
	return label
}

// ─── Types ──────────────────────────────────────────────────────────

type ImportResult struct {
	Imported int      `json:"imported"`
	Updated  int      `json:"updated"`
	Total    int      `json:"total"`
	Removed  []string `json:"removed,omitempty"`
}

type SyncResult struct {
	Synced       int           `json:"synced"`
	Added        int           `json:"added"`
	Updated      int           `json:"updated"`
	Attached     int           `json:"attached"`
	Detached     int           `json:"detached"`
	Deleted      int           `json:"deleted"`
	ServerErrors []ServerError `json:"server_errors,omitempty"`
}

type ServerError struct {
	Server string `json:"server"`
	Error  string `json:"error"`
}

type StaticAuthService = auth.Service
