package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadBundle(dir string) (Bundle, error) {
	var bundle Bundle
	if err := ensureTemplateLayout(dir); err != nil {
		return Bundle{}, fmt.Errorf("prepare template layout: %w", err)
	}

	if err := readYAML(filepath.Join(dir, "app.yaml"), &bundle.App); err != nil {
		return Bundle{}, fmt.Errorf("load app config: %w", err)
	}
	if err := readYAML(filepath.Join(dir, "templates", "whitelist.yaml"), &bundle.BaseWhitelist); err != nil {
		return Bundle{}, fmt.Errorf("load whitelist base: %w", err)
	}
	if err := readYAML(filepath.Join(dir, "templates", "blacklist.yaml"), &bundle.BaseBlacklist); err != nil {
		return Bundle{}, fmt.Errorf("load blacklist base: %w", err)
	}
	if err := readYAML(filepath.Join(dir, "groups.yaml"), &bundle.Groups); err != nil {
		return Bundle{}, fmt.Errorf("load groups config: %w", err)
	}

	if err := ValidateBundle(bundle); err != nil {
		return Bundle{}, err
	}

	return bundle, nil
}

func readYAML(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(raw, out)
}

func readYAMLText(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func SaveAppConfig(dir string, cfg AppConfig) error {
	if err := ValidateBundle(Bundle{
		App:           cfg,
		BaseWhitelist: BaseConfig{Rules: []string{"MATCH,DIRECT"}},
		BaseBlacklist: BaseConfig{Rules: []string{"MATCH,PROXY"}},
		Groups:        GroupConfig{GroupNames: GroupNames{Proxy: "proxy", Auto: "auto", Other: "other"}},
	}); err != nil {
		return err
	}
	return writeYAML(filepath.Join(dir, "app.yaml"), cfg)
}

func SaveGroupsConfig(dir string, cfg GroupConfig) error {
	return writeYAML(filepath.Join(dir, "groups.yaml"), cfg)
}

func LoadProviders(dir string) ([]ProviderAddon, error) {
	path := filepath.Join(dir, "providers.yaml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return []ProviderAddon{}, nil
		}
		return nil, err
	}
	var providers []ProviderAddon
	if err := readYAML(path, &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

func SaveProviders(dir string, providers []ProviderAddon) error {
	return writeYAML(filepath.Join(dir, "providers.yaml"), providers)
}

func LoadProvidersYAML(dir string) (string, error) {
	path := filepath.Join(dir, "providers.yaml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "[]\n", nil
		}
		return "", err
	}
	return readYAMLText(path)
}

func SaveProvidersYAML(dir, raw string) error {
	var providers []ProviderAddon
	if err := yaml.Unmarshal([]byte(raw), &providers); err == nil {
		return SaveProviders(dir, providers)
	}

	// Try parsing as a raw proxy-providers map and auto-convert to list format
	var rawMap map[string]any
	if err := yaml.Unmarshal([]byte(raw), &rawMap); err != nil {
		return fmt.Errorf("无法解析 providers.yaml，请确认格式正确")
	}

	pp, ok := rawMap["proxy-providers"]
	if !ok {
		return fmt.Errorf("需要 proxy-providers 或列表格式")
	}
	ppMap, ok := pp.(map[string]any)
	if !ok {
		return fmt.Errorf("proxy-providers 必须是映射")
	}

	var converted []ProviderAddon
	for name := range ppMap {
		converted = append(converted, ProviderAddon{
			ID:   name,
			Name: name,
			ProxyProviders: map[string]any{
				name: ppMap[name],
			},
			ProxyGroups: []map[string]any{},
		})
	}
	return SaveProviders(dir, converted)
}

func LoadBaseYAML(dir, mode string) (string, error) {
	path, err := baseConfigPath(dir, mode)
	if err != nil {
		return "", err
	}
	return readYAMLText(path)
}

func SaveBaseConfig(dir, mode string, cfg BaseConfig) error {
	if len(cfg.Rules) == 0 {
		return fmt.Errorf("base config requires at least one rule")
	}
	path, err := baseConfigPath(dir, mode)
	if err != nil {
		return err
	}
	return writeYAML(path, cfg)
}

func SaveBaseYAML(dir, mode, raw string) error {
	var cfg BaseConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return fmt.Errorf("解析 %s.yaml 失败: %w", mode, err)
	}
	return SaveBaseConfig(dir, mode, cfg)
}

func writeYAML(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func baseConfigPath(dir, mode string) (string, error) {
	switch mode {
	case "whitelist", "blacklist":
		return filepath.Join(dir, "templates", mode+".yaml"), nil
	default:
		return "", fmt.Errorf("unsupported mode %q", mode)
	}
}
