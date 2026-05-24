package config

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultProviderUpdateInterval = 3600
	defaultProviderHTTPTimeout    = 45 * time.Second
)

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type ProviderRefreshResult struct {
	ID        string `json:"id"`
	Count     int    `json:"count"`
	Path      string `json:"path"`
	UpdatedAt int64  `json:"updated_at"`
}

func UpsertProvider(dir string, provider ProviderAddon, publicBaseURL string) (ProviderAddon, error) {
	providers, err := LoadProviders(dir)
	if err != nil {
		return ProviderAddon{}, err
	}
	var existing *ProviderAddon
	for i := range providers {
		if providers[i].ID == provider.ID {
			existing = &providers[i]
			break
		}
	}
	provider, err = PrepareProvider(provider, publicBaseURL, existing)
	if err != nil {
		return ProviderAddon{}, err
	}

	found := false
	for i := range providers {
		if providers[i].ID == provider.ID {
			providers[i] = provider
			found = true
			break
		}
	}
	if !found {
		providers = append(providers, provider)
	}
	if err := SaveProviders(dir, providers); err != nil {
		return ProviderAddon{}, err
	}
	return provider, nil
}

func PrepareProvider(provider ProviderAddon, publicBaseURL string, existing *ProviderAddon) (ProviderAddon, error) {
	provider.ID = strings.TrimSpace(provider.ID)
	provider.Name = strings.TrimSpace(provider.Name)
	provider.SourceURL = strings.TrimSpace(provider.SourceURL)
	if provider.ID == "" {
		return ProviderAddon{}, fmt.Errorf("provider id is required")
	}
	if !providerIDPattern.MatchString(provider.ID) {
		return ProviderAddon{}, fmt.Errorf("provider id can only contain letters, numbers, _ and -")
	}
	provider.Name = provider.ID
	if provider.SourceURL == "" {
		return ProviderAddon{}, fmt.Errorf("source url is required")
	}
	if _, err := url.ParseRequestURI(provider.SourceURL); err != nil {
		return ProviderAddon{}, fmt.Errorf("invalid source url: %w", err)
	}
	if provider.UpdateIntervalSeconds <= 0 {
		provider.UpdateIntervalSeconds = defaultProviderUpdateInterval
	}
	provider.ProxyProviders = defaultProviderMap(provider, publicBaseURL)
	if len(provider.ProxyGroups) == 0 {
		provider.ProxyGroups = defaultProviderGroups(provider)
	}
	if existing != nil {
		provider.LastUpdatedAt = existing.LastUpdatedAt
		provider.LastError = existing.LastError
		provider.ProxyCount = existing.ProxyCount
	}
	return provider, nil
}

func DeleteProvider(dir, id string) error {
	providers, err := LoadProviders(dir)
	if err != nil {
		return err
	}
	next := providers[:0]
	for _, provider := range providers {
		if provider.ID != id {
			next = append(next, provider)
		}
	}
	if err := SaveProviders(dir, next); err != nil {
		return err
	}
	if err := removeProviderFromAppConfig(dir, id); err != nil {
		return err
	}
	if err := os.Remove(providerFilePath(dir, id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func RefreshProvider(dir, id string) (ProviderRefreshResult, error) {
	providers, err := LoadProviders(dir)
	if err != nil {
		return ProviderRefreshResult{}, err
	}
	for i := range providers {
		if providers[i].ID != id {
			continue
		}
		result, refreshErr := refreshProvider(dir, &providers[i])
		if refreshErr != nil {
			providers[i].LastError = refreshErr.Error()
			_ = SaveProviders(dir, providers)
			return ProviderRefreshResult{}, refreshErr
		}
		providers[i].LastError = ""
		providers[i].LastUpdatedAt = result.UpdatedAt
		providers[i].ProxyCount = result.Count
		if err := SaveProviders(dir, providers); err != nil {
			return ProviderRefreshResult{}, err
		}
		return result, nil
	}
	return ProviderRefreshResult{}, fmt.Errorf("provider %q not found", id)
}

func RefreshProviderAddon(dir string, provider ProviderAddon) (ProviderAddon, ProviderRefreshResult, error) {
	result, err := refreshProvider(dir, &provider)
	if err != nil {
		provider.LastError = err.Error()
		return provider, ProviderRefreshResult{}, err
	}
	provider.LastError = ""
	provider.LastUpdatedAt = result.UpdatedAt
	provider.ProxyCount = result.Count
	return provider, result, nil
}

func RefreshDueProviders(dir string) {
	providers, err := LoadProviders(dir)
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
			interval = defaultProviderUpdateInterval
		}
		if provider.LastUpdatedAt > 0 && now-provider.LastUpdatedAt < int64(interval) {
			continue
		}
		_, _ = RefreshProvider(dir, provider.ID)
	}
}

func ReadProviderFile(dir, id string) ([]byte, error) {
	if !providerIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid provider id")
	}
	path := providerFilePath(dir, id)
	base := filepath.Join(dir, "proxy_providers")
	absBase, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if filepath.Dir(absPath) != absBase {
		return nil, fmt.Errorf("invalid provider path")
	}
	return os.ReadFile(absPath)
}

func RemoveProviderFile(dir, id string) error {
	if !providerIDPattern.MatchString(id) {
		return fmt.Errorf("invalid provider id")
	}
	if err := os.Remove(providerFilePath(dir, id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func refreshProvider(dir string, provider *ProviderAddon) (ProviderRefreshResult, error) {
	if provider.SourceURL == "" {
		return ProviderRefreshResult{}, fmt.Errorf("source url is required")
	}
	raw, err := downloadProvider(provider.SourceURL, provider.InsecureSkipVerify)
	if err != nil {
		return ProviderRefreshResult{}, err
	}
	out, count, err := extractProxiesYAML(raw)
	if err != nil {
		return ProviderRefreshResult{}, err
	}
	if count == 0 {
		return ProviderRefreshResult{}, fmt.Errorf("source contains no proxies")
	}
	path := providerFilePath(dir, provider.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ProviderRefreshResult{}, err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return ProviderRefreshResult{}, err
	}
	return ProviderRefreshResult{
		ID:        provider.ID,
		Count:     count,
		Path:      filepath.ToSlash(filepath.Join("proxy_providers", provider.ID+".yaml")),
		UpdatedAt: time.Now().Unix(),
	}, nil
}

func downloadProvider(sourceURL string, insecureSkipVerify bool) ([]byte, error) {
	client := &http.Client{Timeout: defaultProviderHTTPTimeout}
	if insecureSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "clash")
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func extractProxiesYAML(raw []byte) ([]byte, int, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, 0, fmt.Errorf("parse source yaml: %w", err)
	}
	proxies, ok := doc["proxies"]
	if !ok {
		return nil, 0, fmt.Errorf("source yaml has no proxies")
	}
	list, ok := proxies.([]any)
	if !ok {
		return nil, 0, fmt.Errorf("source proxies is not a list")
	}
	out := map[string]any{"proxies": list}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(out); err != nil {
		return nil, 0, err
	}
	if err := enc.Close(); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), len(list), nil
}

func defaultProviderMap(provider ProviderAddon, publicBaseURL string) map[string]any {
	publicBaseURL = strings.TrimRight(publicBaseURL, "/")
	providerURL := "/api/proxy-providers/" + provider.ID + ".yaml"
	if publicBaseURL != "" {
		providerURL = publicBaseURL + providerURL
	}
	return map[string]any{
		provider.ID: map[string]any{
			"type":     "http",
			"url":      providerURL,
			"path":     "./proxy_providers/" + provider.ID + ".yaml",
			"interval": provider.UpdateIntervalSeconds,
			"proxy":    "DIRECT",
			"health-check": map[string]any{
				"enable":          true,
				"url":             "https://www.gstatic.com/generate_204",
				"interval":        300,
				"timeout":         5000,
				"lazy":            true,
				"expected-status": 204,
			},
		},
	}
}

func defaultProviderGroups(provider ProviderAddon) []map[string]any {
	return []map[string]any{
		{
			"name":     provider.ID,
			"type":     "url-test",
			"url":      "http://www.gstatic.com/generate_204",
			"interval": 300,
			"use":      []string{provider.ID},
		},
	}
}

func providerFilePath(dir, id string) string {
	return filepath.Join(dir, "proxy_providers", id+".yaml")
}

func removeProviderFromAppConfig(dir, id string) error {
	var app AppConfig
	path := filepath.Join(dir, "app.yaml")
	if err := readYAML(path, &app); err != nil {
		return err
	}
	for user, providers := range app.UserProviders {
		filtered := providers[:0]
		for _, providerID := range providers {
			if providerID != id {
				filtered = append(filtered, providerID)
			}
		}
		if len(filtered) == 0 {
			delete(app.UserProviders, user)
		} else {
			app.UserProviders[user] = filtered
		}
	}
	for user, groups := range app.UserGroupNodes {
		for groupName, refs := range groups {
			filtered := refs[:0]
			for _, ref := range refs {
				if ref != id {
					filtered = append(filtered, ref)
				}
			}
			if len(filtered) == 0 {
				delete(groups, groupName)
			} else {
				groups[groupName] = filtered
			}
		}
		if len(groups) == 0 {
			delete(app.UserGroupNodes, user)
		}
	}
	return SaveAppConfig(dir, app)
}
