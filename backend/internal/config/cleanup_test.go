package config

import (
	"reflect"
	"testing"
)

func TestCleanupAppConfigPrunesDeletedUsersAndRefs(t *testing.T) {
	app := AppConfig{
		Mode:                       "whitelist",
		CacheTTLSeconds:            60,
		HealthcheckURL:             "https://example.com/204",
		HealthcheckIntervalSeconds: 300,
		XUI:                        XUIConfig{BaseURL: "http://127.0.0.1"},
		UserModes: map[string]string{
			"alice":   "whitelist",
			"deleted": "blacklist",
		},
		UserNodes: map[string][]string{
			"alice":   {"node-a", "node-missing"},
			"deleted": {"node-a"},
		},
		UserProviders: map[string][]string{
			"alice":   {"provider-a", "provider-missing"},
			"deleted": {"provider-a"},
		},
		UserGroupNodes: map[string]map[string][]string{
			"alice": {
				"PROXY": {"Los Angeles", "provider-a", "Auto", "deleted-ref"},
				"Old":   {"Los Angeles"},
			},
			"deleted": {
				"PROXY": {"Los Angeles"},
			},
		},
	}

	cleaned := CleanupAppConfig(app, AppCleanupState{
		Users:          []string{"alice"},
		UsersKnown:     true,
		NodeIDs:        []string{"node-a"},
		NodeIDsKnown:   true,
		NodeNames:      []string{"Los Angeles"},
		NodeNamesKnown: true,
		Providers:      []string{"provider-a"},
		ProvidersKnown: true,
		Groups:         []string{"PROXY", "Auto"},
		GroupsKnown:    true,
	})

	if _, ok := cleaned.UserModes["deleted"]; ok {
		t.Fatal("deleted user mode was not removed")
	}
	if got := cleaned.UserNodes["alice"]; !reflect.DeepEqual(got, []string{"node-a"}) {
		t.Fatalf("unexpected user nodes: %#v", got)
	}
	if got := cleaned.UserProviders["alice"]; !reflect.DeepEqual(got, []string{"provider-a"}) {
		t.Fatalf("unexpected user providers: %#v", got)
	}
	wantGroupRefs := []string{"Los Angeles", "provider-a", "Auto"}
	if got := cleaned.UserGroupNodes["alice"]["PROXY"]; !reflect.DeepEqual(got, wantGroupRefs) {
		t.Fatalf("unexpected group refs: %#v", got)
	}
	if _, ok := cleaned.UserGroupNodes["alice"]["Old"]; ok {
		t.Fatal("deleted group was not removed")
	}
}

func TestCleanupAppConfigCanRemoveLastKnownProvider(t *testing.T) {
	app := AppConfig{
		Mode:                       "whitelist",
		CacheTTLSeconds:            60,
		HealthcheckURL:             "https://example.com/204",
		HealthcheckIntervalSeconds: 300,
		XUI:                        XUIConfig{BaseURL: "http://127.0.0.1"},
		UserProviders:              map[string][]string{"alice": {"old-provider"}},
		UserGroupNodes: map[string]map[string][]string{
			"alice": {"PROXY": {"old-provider"}},
		},
	}

	cleaned := CleanupAppConfig(app, AppCleanupState{
		ProvidersKnown: true,
		Groups:         []string{"PROXY"},
		GroupsKnown:    true,
	})

	if len(cleaned.UserProviders) != 0 {
		t.Fatalf("expected all providers removed, got %#v", cleaned.UserProviders)
	}
	if len(cleaned.UserGroupNodes) != 0 {
		t.Fatalf("expected provider group refs removed, got %#v", cleaned.UserGroupNodes)
	}
}
