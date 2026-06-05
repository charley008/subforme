package db

import (
	"testing"

	"subforme/backend/internal/config"
)

func TestOpenAppliesLatestSchemaVersion(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	var got int
	if err := store.DB.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&got); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if got != schemaVersion {
		t.Fatalf("expected schema version %d, got %d", schemaVersion, got)
	}

	var tableName string
	if err := store.DB.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'nodes'").Scan(&tableName); err != nil {
		t.Fatalf("expected nodes table from latest migration: %v", err)
	}
}

func TestUpdateUserPreservesSubscriptionPrefs(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	u := &User{
		Email:           "alice",
		UUID:            "uuid-1",
		Enable:          true,
		Mode:            "blacklist",
		NodeIDsJSON:     `["los"]`,
		ProviderIDsJSON: `["airport"]`,
		GroupNodesJSON:  `{"PROXY":["los","airport"]}`,
		GroupModesJSON:  `{"PROXY":"fallback"}`,
	}
	if err := store.CreateUser(u); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	u.UUID = "uuid-2"
	u.Mode = ""
	u.NodeIDsJSON = ""
	u.ProviderIDsJSON = ""
	u.GroupNodesJSON = ""
	u.GroupModesJSON = ""
	if err := store.UpdateUser(u); err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}

	got, err := store.GetUser(u.ID)
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if got.UUID != "uuid-2" || got.Mode != "blacklist" || got.NodeIDsJSON == "" || got.ProviderIDsJSON == "" || got.GroupNodesJSON == "" || got.GroupModesJSON == "" {
		t.Fatalf("unexpected user after update: %#v", got)
	}
}

func TestAppConfigPrefsOnlyApplyToExistingUsers(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if err := store.CreateUser(&User{Email: "alice", UUID: "uuid", Enable: true}); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	err = store.SaveAppConfigToDB(config.AppConfig{
		Mode: "whitelist",
		UserNodes: map[string][]string{
			"alice": {"node-a"},
			"ghost": {"node-b"},
		},
		UserProviders: map[string][]string{
			"alice": {"airport"},
			"ghost": {"stale"},
		},
		UserGroupModes: map[string]map[string]string{
			"alice": {"PROXY": "fallback"},
			"ghost": {"PROXY": "url-test"},
		},
	})
	if err != nil {
		t.Fatalf("SaveAppConfigToDB returned error: %v", err)
	}

	got, found, err := store.LoadAppConfigFromDB(config.AppConfig{})
	if err != nil {
		t.Fatalf("LoadAppConfigFromDB returned error: %v", err)
	}
	if !found {
		t.Fatal("expected app config to be found")
	}
	if _, ok := got.UserNodes["ghost"]; ok {
		t.Fatalf("stale user was imported: %#v", got.UserNodes)
	}
	if got.UserNodes["alice"][0] != "node-a" || got.UserProviders["alice"][0] != "airport" {
		t.Fatalf("expected alice prefs, got %#v %#v", got.UserNodes, got.UserProviders)
	}
	if got.UserGroupModes["alice"]["PROXY"] != "fallback" {
		t.Fatalf("expected alice group mode override, got %#v", got.UserGroupModes)
	}
}

func TestEmptyProvidersRemainDBOwned(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if providers, found, err := store.LoadProvidersFromDB(); err != nil || found || len(providers) != 0 {
		t.Fatalf("expected providers to be unseeded, found=%v providers=%#v err=%v", found, providers, err)
	}
	if err := store.SaveProvidersToDB(nil); err != nil {
		t.Fatalf("SaveProvidersToDB returned error: %v", err)
	}
	if providers, found, err := store.LoadProvidersFromDB(); err != nil || !found || len(providers) != 0 {
		t.Fatalf("expected empty providers to be DB-owned, found=%v providers=%#v err=%v", found, providers, err)
	}
}

func TestEnsureServerInboundsPreservesInboundRowIDs(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if err := store.EnsureServerInbounds(1, []Inbound{{
		InboundID:    7,
		Remark:       "vless",
		Port:         443,
		Protocol:     "vless",
		SettingsJSON: `{"clients":[]}`,
		Enable:       true,
	}}); err != nil {
		t.Fatalf("EnsureServerInbounds returned error: %v", err)
	}
	first, err := store.ListInboundsByServer(1)
	if err != nil {
		t.Fatalf("ListInboundsByServer returned error: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected one inbound, got %#v", first)
	}
	if err := store.EnsureServerInbounds(1, []Inbound{{
		InboundID:    7,
		Remark:       "renamed",
		Port:         8443,
		Protocol:     "vless",
		SettingsJSON: `{"clients":[{"email":"alice"}]}`,
		Enable:       true,
	}}); err != nil {
		t.Fatalf("EnsureServerInbounds returned error: %v", err)
	}
	second, err := store.ListInboundsByServer(1)
	if err != nil {
		t.Fatalf("ListInboundsByServer returned error: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected one inbound, got %#v", second)
	}
	if second[0].ID != first[0].ID {
		t.Fatalf("expected stable local row id %d, got %d", first[0].ID, second[0].ID)
	}
	if second[0].Remark != "renamed" || second[0].Port != 8443 {
		t.Fatalf("expected inbound update, got %#v", second[0])
	}
}

func TestServerScheduleFieldsPersistAcrossRoundTrip(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	sv := &Server{
		Name:                       "hk",
		Scheme:                     "https",
		Host:                       "panel.example.com",
		Port:                       2053,
		BasePath:                   "/xui/",
		APIKey:                     "token",
		Enabled:                    true,
		TrafficSyncIntervalMinutes: 15,
		AutoResetTrafficEnabled:    true,
		AutoResetDay:               5,
		AutoResetHour:              2,
		AutoResetMinute:            30,
		AutoResetTimezone:          "Asia/Shanghai",
		LastTrafficSyncAt:          123,
		LastTrafficResetKey:        "2026-06",
	}
	if err := store.CreateServer(sv); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	got, err := store.GetServer(sv.ID)
	if err != nil {
		t.Fatalf("GetServer returned error: %v", err)
	}
	if got.TrafficSyncIntervalMinutes != 15 || !got.AutoResetTrafficEnabled || got.AutoResetDay != 5 || got.AutoResetHour != 2 || got.AutoResetMinute != 30 || got.AutoResetTimezone != "Asia/Shanghai" || got.LastTrafficSyncAt != 123 || got.LastTrafficResetKey != "2026-06" {
		t.Fatalf("unexpected server schedule fields: %#v", got)
	}
}
