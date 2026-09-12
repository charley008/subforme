package db

import "testing"

func TestNodeOptionsPersistAndInvalidSavePreservesData(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	options := "reality-opts:\n  support-x25519mlkem768: false"
	if err := s.ReplaceNodes([]Node2{{NodeID: "test", Name: "test", MihomoOptions: options}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceNodes([]Node2{{NodeID: "test", Name: "test", MihomoOptions: "name: forbidden"}}); err == nil {
		t.Fatal("expected validation failure")
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	nodes, err := s.ListNodeDB()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].MihomoOptions != options {
		t.Fatalf("options lost: %#v", nodes)
	}
}

func TestV12MigrationPreservesExistingNodes(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ReplaceNodes([]Node2{{NodeID: "old", Name: "old"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("ALTER TABLE nodes DROP COLUMN mihomo_options"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("DELETE FROM schema_version WHERE version = 12"); err != nil {
		t.Fatal(err)
	}
	if err := s.migrate(); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodeDB()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].NodeID != "old" || nodes[0].MihomoOptions != "" {
		t.Fatalf("migration changed data: %#v", nodes)
	}
}
