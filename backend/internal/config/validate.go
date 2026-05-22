package config

import "fmt"

func ValidateBundle(bundle Bundle) error {
	if bundle.App.Mode != "whitelist" && bundle.App.Mode != "blacklist" {
		return fmt.Errorf("unsupported mode %q", bundle.App.Mode)
	}
	for user, mode := range bundle.App.UserModes {
		if mode != "whitelist" && mode != "blacklist" {
			return fmt.Errorf("unsupported user mode %q for %s", mode, user)
		}
	}
	for user, nodes := range bundle.App.UserNodes {
		for _, nodeID := range nodes {
			if nodeID == "" {
				return fmt.Errorf("user %s contains empty node id", user)
			}
		}
	}
	for user, providers := range bundle.App.UserProviders {
		for _, providerID := range providers {
			if providerID == "" {
				return fmt.Errorf("user %s contains empty provider id", user)
			}
		}
	}
	if len(bundle.BaseWhitelist.Rules) == 0 {
		return fmt.Errorf("whitelist base requires at least one rule")
	}
	if len(bundle.BaseBlacklist.Rules) == 0 {
		return fmt.Errorf("blacklist base requires at least one rule")
	}
	if len(bundle.Groups.Groups) == 0 {
		if bundle.Groups.GroupNames.Proxy == "" || bundle.Groups.GroupNames.Auto == "" || bundle.Groups.GroupNames.Other == "" {
			return fmt.Errorf("group_names must define proxy, auto, and other")
		}
	}
	if bundle.App.XUI.BaseURL == "" && (bundle.App.XUI.APIKey != "" || bundle.App.XUI.Username != "" || bundle.App.XUI.Password != "") {
		return fmt.Errorf("xui.base_url is required when xui auth is configured")
	}
	return nil
}
