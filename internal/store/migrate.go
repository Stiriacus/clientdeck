package store

import (
	"database/sql"
	"fmt"
)

// schemaVersion is tracked via PRAGMA user_version so migrate is a no-op on
// an already-initialized database.
const schemaVersion = 2

const schemaV1 = `
CREATE TABLE IF NOT EXISTS customer_views (
    customer_id TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    client_name TEXT NOT NULL,
    intro       TEXT NOT NULL DEFAULT '',
    products    TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_customer_views_slug ON customer_views(slug);
`

const schemaV2 = `
ALTER TABLE customer_views ADD COLUMN language TEXT NOT NULL DEFAULT 'en';
`

// migrate applies the schema if the database's user_version is behind
// schemaVersion. Migrations are applied sequentially from the current version.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("store: read schema version: %w", err)
	}
	if version >= schemaVersion {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin migration: %w", err)
	}
	defer tx.Rollback()

	// v0 → v1: initial schema.
	if version < 1 {
		if _, err := tx.Exec(schemaV1); err != nil {
			return fmt.Errorf("store: apply v1 schema: %w", err)
		}
	}
	// v1 → v2: add language column.
	if version < 2 {
		if _, err := tx.Exec(schemaV2); err != nil {
			return fmt.Errorf("store: apply v2 schema: %w", err)
		}
	}

	// user_version does not accept bind parameters; schemaVersion is a
	// package constant, never user input.
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("store: set schema version: %w", err)
	}

	return tx.Commit()
}
