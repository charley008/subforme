package db

import "testing"

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
