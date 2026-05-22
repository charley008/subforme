package app

import (
	"context"
	"fmt"

	"subforme/backend/internal/auth"
	"subforme/backend/internal/config"
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

func (s Service) Generate(user string) ([]byte, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return nil, err
	}

	nodes, err := s.resolver(bundle.App).ResolveUserNodes(context.Background(), user)
	if err != nil {
		return nil, fmt.Errorf("resolve user nodes: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("user %q not found", user)
	}
	managedNodes, err := s.ReadManagedNodes()
	if err != nil {
		return nil, fmt.Errorf("read managed nodes: %w", err)
	}
	nodes = applyManagedNodes(nodes, managedNodes, bundle.App.UserNodes[user])
	if len(nodes) == 0 {
		return nil, fmt.Errorf("user %q has no selected nodes", user)
	}

	mode := s.resolveMode(bundle.App, user)
	providers, err := config.LoadProviders(s.ConfigDir)
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

	groupList := groups.Build(bundle.Groups, nodes, bundle.App.UserGroupNodes[user])
	raw, err := generator.BuildFinalYAML(templateRaw, nodes, groupList, providers, bundle.App.UserProviders[user], bundle.Groups.GroupNames.Proxy)
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
	return config.SaveGroupsConfig(s.ConfigDir, next)
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
	nodes, err := s.resolver(bundle.App).ResolveUserNodes(context.Background(), query)
	if err != nil {
		return nil, err
	}
	managedNodes, err := s.ReadManagedNodes()
	if err != nil {
		return nil, err
	}
	return applyManagedNodes(nodes, managedNodes, bundle.App.UserNodes[query]), nil
}

func (s Service) ReadManagedNodes() ([]config.ManagedNode, error) {
	return config.LoadManagedNodes(s.ConfigDir)
}

func (s Service) UpdateManagedNodes(next []config.ManagedNode) error {
	return config.SaveManagedNodes(s.ConfigDir, next)
}

func (s Service) ReadProviders() ([]config.ProviderAddon, error) {
	return config.LoadProviders(s.ConfigDir)
}

func (s Service) DetectAvailableNodes() ([]xui.AvailableNode, error) {
	bundle, err := s.loadBundle()
	if err != nil {
		return nil, err
	}
	return s.resolver(bundle.App).ListAvailableNodes(context.Background())
}

func (s Service) ReadTemplateSectionYAML(section string) (string, error) {
	if section == "providers" {
		return config.LoadProvidersYAML(s.ConfigDir)
	}
	return config.LoadTemplateSectionYAML(s.ConfigDir, section)
}

func (s Service) UpdateTemplateSectionYAML(section, raw string) error {
	if section == "providers" {
		return config.SaveProvidersYAML(s.ConfigDir, raw)
	}
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
	bundle, err := s.Loader(s.ConfigDir)
	if err != nil {
		return config.Bundle{}, fmt.Errorf("load config bundle: %w", err)
	}
	bundle.App.XUI = s.mergeXUI(bundle.App.XUI)
	return bundle, nil
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

type StaticAuthService = auth.Service
