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
	}
	if err := store.CreateUser(u); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	u.UUID = "uuid-2"
	u.Mode = ""
	u.NodeIDsJSON = ""
	u.ProviderIDsJSON = ""
	u.GroupNodesJSON = ""
	if err := store.UpdateUser(u); err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}

	got, err := store.GetUser(u.ID)
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if got.UUID != "uuid-2" || got.Mode != "blacklist" || got.NodeIDsJSON == "" || got.ProviderIDsJSON == "" || got.GroupNodesJSON == "" {
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
