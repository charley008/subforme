package db

import (
	"testing"

	"subforme/backend/internal/config"
)

func TestUpsertProviderCreatesProxyGroup(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	provider := config.ProviderAddon{
		ID:                    "airport",
		Name:                  "Airport",
		UpdateIntervalSeconds: 3600,
		ProxyProviders:        map[string]any{"airport": map[string]any{"type": "http"}},
	}
	if err := store.UpsertProviderToDB(provider); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}

	groups, found, err := store.LoadGroupsConfigFromDB()
	if err != nil {
		t.Fatalf("load groups: %v", err)
	}
	if !found {
		t.Fatal("expected groups config to exist")
	}
	if len(groups.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups.Groups))
	}
	group := groups.Groups[0]
	if group.Name != "airport" || group.Type != "url-test" || group.Provider != "airport" {
		t.Fatalf("unexpected provider group: %+v", group)
	}
	if group.URL != "http://www.gstatic.com/generate_204" || group.Interval != 300 {
		t.Fatalf("unexpected provider group healthcheck: %+v", group)
	}
}

func TestDeleteProviderRemovesProxyGroup(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	if err := store.UpsertProviderToDB(config.ProviderAddon{ID: "airport", Name: "Airport"}); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	if err := store.DeleteProviderFromDB("airport"); err != nil {
		t.Fatalf("delete provider: %v", err)
	}

	groups, _, err := store.LoadGroupsConfigFromDB()
	if err != nil {
		t.Fatalf("load groups: %v", err)
	}
	if len(groups.Groups) != 0 {
		t.Fatalf("expected provider group to be deleted, got %+v", groups.Groups)
	}
}

func TestSaveGroupsConfigPreservesProviderGroups(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	if err := store.UpsertProviderToDB(config.ProviderAddon{ID: "airport", Name: "Airport"}); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	if err := store.SaveGroupsConfigToDB(config.GroupConfig{
		Groups: []config.GroupDef{{Name: "PROXY", Type: "select"}},
	}); err != nil {
		t.Fatalf("save groups: %v", err)
	}

	groups, _, err := store.LoadGroupsConfigFromDB()
	if err != nil {
		t.Fatalf("load groups: %v", err)
	}
	if len(groups.Groups) != 2 {
		t.Fatalf("expected normal group plus provider group, got %+v", groups.Groups)
	}
	gotProvider := false
	for _, group := range groups.Groups {
		if group.Name == "airport" && group.Provider == "airport" {
			gotProvider = true
		}
	}
	if !gotProvider {
		t.Fatalf("expected provider group to be preserved, got %+v", groups.Groups)
	}
}

func TestSaveGroupsConfigClearsHealthcheckFieldsForNormalGroups(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	if err := store.SaveGroupsConfigToDB(config.GroupConfig{
		Groups: []config.GroupDef{{
			Name:     "PROXY",
			Type:     "select",
			URL:      "https://www.gstatic.com/generate_204",
			Interval: 300,
		}},
	}); err != nil {
		t.Fatalf("save groups: %v", err)
	}

	groups, _, err := store.LoadGroupsConfigFromDB()
	if err != nil {
		t.Fatalf("load groups: %v", err)
	}
	if len(groups.Groups) != 1 || groups.Groups[0].URL != "" || groups.Groups[0].Interval != 0 {
		t.Fatalf("expected normal group healthcheck fields to be cleared, got %+v", groups.Groups)
	}
}
