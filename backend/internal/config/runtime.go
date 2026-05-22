package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func LoadRuntimeConfig(path string) (RuntimeConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return RuntimeConfig{}, err
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return RuntimeConfig{}, err
	}
	var cfg RuntimeConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return RuntimeConfig{}, fmt.Errorf("parse runtime config: %w", err)
	}
	baseDir := filepath.Dir(absPath)

	cfg.Listen = env("SUBFORME_LISTEN", cfg.Listen)
	cfg.AdminUsername = env("SUBFORME_ADMIN_USERNAME", cfg.AdminUsername)
	cfg.AdminPassword = env("SUBFORME_ADMIN_PASSWORD", cfg.AdminPassword)
	cfg.SessionSecret = env("SUBFORME_SESSION_SECRET", cfg.SessionSecret)
	cfg.ConfigDir = env("SUBFORME_CONFIG_DIR", cfg.ConfigDir)
	cfg.FrontendDir = env("SUBFORME_FRONTEND_DIR", cfg.FrontendDir)

	if cfg.XUI.BaseURL == "" {
		cfg.XUI.BaseURL = env("SUBFORME_XUI_BASE_URL", "")
	}
	if cfg.XUI.APIKey == "" {
		cfg.XUI.APIKey = env("SUBFORME_XUI_API_KEY", "")
	}
	if cfg.XUI.Username == "" {
		cfg.XUI.Username = env("SUBFORME_XUI_USERNAME", "")
	}
	if cfg.XUI.Password == "" {
		cfg.XUI.Password = env("SUBFORME_XUI_PASSWORD", "")
	}

	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = "."
	}
	if cfg.FrontendDir == "" {
		cfg.FrontendDir = "../frontend/dist"
	}
	if !filepath.IsAbs(cfg.ConfigDir) {
		cfg.ConfigDir = filepath.Clean(filepath.Join(baseDir, cfg.ConfigDir))
	}
	if !filepath.IsAbs(cfg.FrontendDir) {
		cfg.FrontendDir = filepath.Clean(filepath.Join(baseDir, cfg.FrontendDir))
	}
	cfg.RuntimePath = absPath
	return cfg, nil
}

func SaveRuntimePassword(path, newPassword string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	cfg["admin_password"] = newPassword
	raw, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
