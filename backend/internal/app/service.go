package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"subforme/backend/internal/auth"
	"subforme/backend/internal/config"
	"subforme/backend/internal/db"
	"subforme/backend/internal/generator"
	"subforme/backend/internal/groups"
	"subforme/backend/internal/xui"
)

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
	templateLoader := s.TemplateLoader
	if templateLoader == nil {
		templateLoader = config.LoadModeTemplateYAML
	}
	templateRaw, err := templateLoader(s.ConfigDir, mode)
	if err != nil {
		return nil, fmt.Errorf("load %s template: %w", mode, err)
	}

	groupList := groups.Build(bundle.Groups, nodes, bundle.App.UserGroupNodes[user], bundle.App.UserProviders[user])
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

func (s Service) ReadBaseYAML(mode string) (string, error) {
	return config.LoadBaseYAML(s.ConfigDir, mode)
}

func (s Service) UpdateBaseYAML(mode, raw string) error {
	return config.SaveBaseYAML(s.ConfigDir, mode, raw)
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
	result := map[string][]db.ServerTraffic{}
	servers, err := s.DB.ListServers()
	if err != nil {
		return result
	}

	var trafficEntries []db.UserTraffic
	for _, sv := range servers {
		if !sv.Enabled {
			continue
		}
		cli := xui.NewClient(s.xuiURL(&sv), sv.APIKey, "", "")
		inbounds, err := cli.ListInbounds(ctx)
		if err != nil {
			log.Printf("[traffic] %s: %v", sv.Name, err)
			continue
		}
		for _, inb := range inbounds {
			for _, cs := range inb.ClientStats {
				if cs.Email == "" {
					continue
				}
				st := db.ServerTraffic{
					ServerID:      sv.ID,
					ServerName:    sv.Name,
					ServerAddress: sv.Host,
					Up:            cs.Up,
					Down:          cs.Down,
				}
				result[cs.Email] = append(result[cs.Email], st)
				trafficEntries = append(trafficEntries, db.UserTraffic{
					Email:    cs.Email,
					ServerID: sv.ID,
					Up:       cs.Up,
					Down:     cs.Down,
				})
			}
		}
	}

	// Store in DB (delete old, insert fresh)
	if err := s.DB.ReplaceUserTraffic(trafficEntries); err != nil {
		log.Printf("[traffic] store error: %v", err)
	}

	return result
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

	var cachedInbounds []db.Inbound
	for _, inb := range inbounds {
		trafficJSON := ""
		if len(inb.ClientStats) > 0 {
			if b, err := json.Marshal(inb.ClientStats); err == nil {
				trafficJSON = string(b)
			}
		}
		cachedInbounds = append(cachedInbounds, db.Inbound{
			InboundID:          int(inb.ID),
			Remark:             inb.Remark,
			Port:               inb.Port,
			Protocol:           inb.Protocol,
			SettingsJSON:       inb.Settings,
			StreamSettingsJSON: inb.StreamSettings,
			SniffingJSON:       inb.Sniffing,
			Enable:             inb.Enable,
			TrafficJSON:        trafficJSON,
		})
	}
	if err := s.DB.EnsureServerInbounds(serverID, cachedInbounds); err != nil {
		return nil, fmt.Errorf("cache inbounds: %w", err)
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
					Flow:       cl.Flow,
					TotalGB:    inb.Total,
					ExpiryTime: inb.ExpiryTime,
					Enable:     cl.Enable,
				}
				if err := s.DB.CreateUser(u); err != nil {
					return nil, fmt.Errorf("create user %s: %w", cl.Email, err)
				}
				seen[cl.Email] = u
				imported++
			} else {
				existing.UUID = cl.ID
				if cl.Password != "" {
					existing.Password = cl.Password
				}
				if cl.Flow != "" {
					existing.Flow = cl.Flow
				}
				if inb.Total > 0 {
					existing.TotalGB = inb.Total
				}
				existing.ExpiryTime = inb.ExpiryTime
				existing.Enable = cl.Enable
				if err := s.DB.UpdateUser(existing); err != nil {
					return nil, fmt.Errorf("update user %s: %w", cl.Email, err)
				}
				seen[cl.Email] = existing
				updated++
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
					cachedList, err := s.DB.ListInboundsByServer(serverID)
					if err != nil {
						continue
					}
					for _, ci := range cachedList {
						if ci.InboundID == int(inb.ID) {
							s.DB.CreateAssignment(&db.UserAssignment{
								UserID:        u.ID,
								ServerID:      serverID,
								InboundID:     ci.ID,
								EmailOnServer: email,
								Enable:        cl.Enable,
							})
							break
						}
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

func (s Service) SyncToServers(ctx context.Context) (*SyncResult, error) {
	servers, err := s.DB.ListServers()
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	result := &SyncResult{}
	log.Printf("[sync] found %d servers", len(servers))

	// Track users removed from main server for cross-server deletion
	removedFromMain := map[string]bool{}

	for _, sv := range servers {
		if sv.IsMain && sv.Enabled {
			log.Printf("[sync] auto-import from main server %s", sv.Name)
			imported, err := s.ImportFromServer(ctx, sv.ID)
			if err != nil {
				log.Printf("[sync] import error: %v", err)
			} else {
				log.Printf("[sync] import: %d new, %d updated, %d total, %d removed",
					imported.Imported, imported.Updated, imported.Total, len(imported.Removed))
				for _, email := range imported.Removed {
					removedFromMain[email] = true
				}
			}
			break
		}
	}

	users, err := s.DB.ListEnabledUsers()
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	log.Printf("[sync] %d enabled users in DB", len(users))

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

		existingEmails := map[string]bool{}
		for _, inb := range existingInbounds {
			if settings, ok := xui.ParseInboundSettings(inb.Settings); ok {
				for _, cl := range settings.Clients {
					if cl.Email != "" {
						existingEmails[cl.Email] = true
					}
				}
			}
		}
		log.Printf("[sync] %s: %d existing users", sv.Name, len(existingEmails))

		// Get or fetch inbounds for this server
		inbounds, err := s.DB.ListInboundsByServer(sv.ID)
		if err != nil || len(inbounds) == 0 {
			fresh, err := cli.ListInbounds(ctx)
			if err != nil {
				log.Printf("[sync] fetch inbounds for %s: %v", sv.Name, err)
				continue
			}
			var cached []db.Inbound
			for _, inb := range fresh {
				cached = append(cached, db.Inbound{
					InboundID:          int(inb.ID),
					Remark:             inb.Remark,
					Port:               inb.Port,
					Protocol:           inb.Protocol,
					SettingsJSON:       inb.Settings,
					StreamSettingsJSON: inb.StreamSettings,
					SniffingJSON:       inb.Sniffing,
					Enable:             inb.Enable,
				})
			}
			s.DB.EnsureServerInbounds(sv.ID, cached)
			inbounds = cached
		}

		for _, u := range users {
			if existingEmails[u.Email] {
				continue
			}
			log.Printf("[sync] adding %s to %s", u.Email, sv.Name)

			matched := false
			assignments, _ := s.DB.ListAssignmentsByUser(u.ID)
			for _, a := range assignments {
				if a.ServerID == sv.ID {
					inb, err := s.DB.GetInbound(a.InboundID)
					if err != nil {
						continue
					}
					cc := xui.BuildClientConfig(inb.Protocol, xui.ClientParams{
						Email: u.Email, UUID: u.UUID, Password: u.Password, Flow: u.Flow,
						TotalGB: u.TotalGB, ExpiryTime: u.ExpiryTime, LimitIP: u.LimitIP, Enable: u.Enable,
					})
					raw, _ := json.Marshal(map[string]any{"clients": []xui.InboundClient{cc}})
					if err := cli.AddClient(ctx, inb.InboundID, string(raw)); err != nil {
						result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("add %s: %v", u.Email, err)})
						continue
					}
					log.Printf("[sync] %s added to %s inbound %d", u.Email, sv.Name, inb.InboundID)
					result.Synced++
					matched = true
					break
				}
			}
			if matched {
				continue
			}

			for _, inb := range inbounds {
				if !inb.Enable {
					continue
				}
				cc := xui.BuildClientConfig(inb.Protocol, xui.ClientParams{
					Email: u.Email, UUID: u.UUID, Password: u.Password, Flow: u.Flow,
					TotalGB: u.TotalGB, ExpiryTime: u.ExpiryTime, LimitIP: u.LimitIP, Enable: u.Enable,
				})
				raw, _ := json.Marshal(map[string]any{"clients": []xui.InboundClient{cc}})
				if err := cli.AddClient(ctx, inb.InboundID, string(raw)); err != nil {
					result.ServerErrors = append(result.ServerErrors, ServerError{Server: sv.Name, Error: fmt.Sprintf("add %s: %v", u.Email, err)})
					continue
				}
				log.Printf("[sync] %s added to %s inbound %d", u.Email, sv.Name, inb.InboundID)
				s.DB.UpdateInboundClientsJSON(inb.ID, string(raw))
				if assignee, _ := s.DB.GetUserByEmail(u.Email); assignee != nil {
					s.DB.CreateAssignment(&db.UserAssignment{
						UserID: assignee.ID, ServerID: sv.ID, InboundID: inb.ID,
						EmailOnServer: u.Email, Enable: true,
					})
				}
				result.Synced++
				break
			}
		}

		// Delete users on this server that don't exist in local DB
		localEmails := map[string]bool{}
		for _, u := range users {
			localEmails[u.Email] = true
		}
		for email := range existingEmails {
			if localEmails[email] && !removedFromMain[email] {
				continue // exists in DB and wasn't removed from main, keep it
			}
			log.Printf("[sync] deleting %s from %s", email, sv.Name)
			result.Deleted++
			for _, inb := range existingInbounds {
				cli.DeleteClientByEmail(ctx, int(inb.ID), email)
				s.DB.UpdateInboundClientsJSON(inb.ID, inb.Settings)
			}
		}
	}

	// Disable removed users in local DB (those with no assignments left)
	for email := range removedFromMain {
		u, err := s.DB.GetUserByEmail(email)
		if err != nil {
			continue
		}
		rem, _ := s.DB.ListAssignmentsByUser(u.ID)
		if len(rem) == 0 {
			u.Enable = false
			s.DB.UpdateUser(u)
			log.Printf("[sync] disabled %s in local DB (no remaining assignments)", email)
		}
	}

	return result, nil
}

// ─── Preview & Generate helpers ─────────────────────────────────────

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
	Deleted      int           `json:"deleted"`
	ServerErrors []ServerError `json:"server_errors,omitempty"`
}

type ServerError struct {
	Server string `json:"server"`
	Error  string `json:"error"`
}

type StaticAuthService = auth.Service
