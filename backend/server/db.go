package server

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

func ensureDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	return os.MkdirAll(dir, 0o755)
}

func openDB(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout=5000&_pragma=foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func OpenDB(dbPath string) (*sql.DB, error) {
	return openDB(dbPath)
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS endpoints (
			id TEXT PRIMARY KEY,
			name TEXT,
			method TEXT NOT NULL,
			path_pattern TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			tags_json TEXT NOT NULL,
			description TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS rules (
			id TEXT PRIMARY KEY,
			endpoint_id TEXT NOT NULL,
			name TEXT,
			enabled INTEGER NOT NULL,
			priority INTEGER NOT NULL,
			weight INTEGER NOT NULL,
			matchers_json TEXT NOT NULL,
			response_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_rules_endpoint_id ON rules(endpoint_id);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
