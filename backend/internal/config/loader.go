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

func LoadTemplateBundle(dir string) (Bundle, error) {
	var bundle Bundle
	if err := ensureTemplateLayout(dir); err != nil {
		return Bundle{}, fmt.Errorf("prepare template layout: %w", err)
	}
	if err := readYAML(filepath.Join(dir, "templates", "whitelist.yaml"), &bundle.BaseWhitelist); err != nil {
		return Bundle{}, fmt.Errorf("load whitelist base: %w", err)
	}
	if err := readYAML(filepath.Join(dir, "templates", "blacklist.yaml"), &bundle.BaseBlacklist); err != nil {
		return Bundle{}, fmt.Errorf("load blacklist base: %w", err)
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

func LoadGroupsConfig(dir string) (GroupConfig, error) {
	var cfg GroupConfig
	if err := readYAML(filepath.Join(dir, "groups.yaml"), &cfg); err != nil {
		return GroupConfig{}, err
	}
	return cfg, nil
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
