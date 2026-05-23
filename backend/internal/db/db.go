package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

type Store struct {
	DB *sql.DB
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	path := filepath.Join(dir, "subforme.db")

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_cache_size=-20000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{DB: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrate() error {
	// Create schema_version table
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	var current int
	err = s.DB.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&current)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if current < 1 {
		if err := s.migrateV1(); err != nil {
			return err
		}
	}
	if current < 2 {
		if err := s.migrateV2(); err != nil {
			return err
		}
	}
	if current < 3 {
		if err := s.migrateV3(); err != nil {
			return err
		}
	}
	if current < 4 {
		if err := s.migrateV4(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) migrateV4() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id   TEXT NOT NULL UNIQUE,
			name      TEXT NOT NULL,
			address   TEXT NOT NULL,
			port      INTEGER DEFAULT 443,
			server_id INTEGER DEFAULT 0 REFERENCES servers(id)
		)
	`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT INTO schema_version (version, applied_at) VALUES (4, ?)", time.Now().Unix())
	return err
}

func (s *Store) migrateV3() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_traffic (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			email     TEXT NOT NULL,
			server_id INTEGER NOT NULL,
			up        INTEGER DEFAULT 0,
			down      INTEGER DEFAULT 0,
			updated_at INTEGER NOT NULL,
			UNIQUE(email, server_id)
		)
	`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT INTO schema_version (version, applied_at) VALUES (3, ?)", time.Now().Unix())
	return err
}

func (s *Store) migrateV2() error {
	_, err := s.DB.Exec("ALTER TABLE inbounds ADD COLUMN traffic_json TEXT DEFAULT ''")
	if err != nil {
		// Ignore error if column already exists
	}
	_, err = s.DB.Exec("INSERT INTO schema_version (version, applied_at) VALUES (2, ?)", time.Now().Unix())
	return err
}

func (s *Store) migrateV1() error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS servers (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL,
			scheme      TEXT NOT NULL DEFAULT 'https',
			host        TEXT NOT NULL,
			port        INTEGER NOT NULL DEFAULT 2053,
			base_path   TEXT NOT NULL DEFAULT '/xui/',
			api_key     TEXT NOT NULL DEFAULT '',
			sub_address TEXT DEFAULT '',
			sub_port    INTEGER DEFAULT 0,
			is_main     INTEGER DEFAULT 0,
			remark      TEXT DEFAULT '',
			enabled     INTEGER DEFAULT 1,
			created_at  INTEGER NOT NULL,
			updated_at  INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS inbounds (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			server_id           INTEGER NOT NULL REFERENCES servers(id),
			inbound_id          INTEGER NOT NULL,
			remark              TEXT NOT NULL,
			port                INTEGER NOT NULL,
			protocol            TEXT NOT NULL,
			settings_json       TEXT NOT NULL,
			stream_settings_json TEXT DEFAULT '',
			sniffing_json       TEXT DEFAULT '',
			tag                 TEXT DEFAULT '',
			enable              INTEGER DEFAULT 1,
			updated_at          INTEGER NOT NULL,
			UNIQUE(server_id, inbound_id)
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			email       TEXT NOT NULL UNIQUE,
			uuid        TEXT NOT NULL,
			password    TEXT DEFAULT '',
			auth        TEXT DEFAULT '',
			flow        TEXT DEFAULT '',
			security    TEXT DEFAULT '',
			remark      TEXT DEFAULT '',
			total_gb    INTEGER DEFAULT 0,
			expiry_time INTEGER DEFAULT 0,
			limit_ip    INTEGER DEFAULT 0,
			sub_id      TEXT DEFAULT '',
			tg_id       INTEGER DEFAULT 0,
			reset       INTEGER DEFAULT 0,
			comment     TEXT DEFAULT '',
			enable      INTEGER DEFAULT 1,
			created_at  INTEGER NOT NULL,
			updated_at  INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS user_assignments (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id         INTEGER NOT NULL REFERENCES users(id),
			server_id       INTEGER NOT NULL REFERENCES servers(id),
			inbound_id      INTEGER NOT NULL REFERENCES inbounds(id),
			email_on_server TEXT NOT NULL,
			enable          INTEGER DEFAULT 1,
			created_at      INTEGER NOT NULL,
			UNIQUE(user_id, server_id, inbound_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_inbounds_server ON inbounds(server_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_user ON user_assignments(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_server ON user_assignments(server_id)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec migration: %w\nStatement: %s", err, stmt)
		}
	}

	_, err = tx.Exec("INSERT INTO schema_version (version, applied_at) VALUES (1, ?)", time.Now().Unix())
	if err != nil {
		return err
	}

	return tx.Commit()
}
