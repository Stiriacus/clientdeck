// Package store implements board.Store on top of SQLite, storing each
// customer's products as a JSON blob in a single row.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"

	sqlite "modernc.org/sqlite"
)

// maxSlugAttempts bounds how many times Upsert retries newSlug on a slug
// collision before giving up with board.ErrSlugExhausted.
const maxSlugAttempts = 5

// sqliteConstraintUnique is the SQLite result code for a UNIQUE constraint
// violation (SQLITE_CONSTRAINT_UNIQUE = SQLITE_CONSTRAINT | (SQLITE_UNIQUE << 8)).
const sqliteConstraintUnique = 2067

// SQLiteStore persists board.CustomerViews in a SQLite database. It
// implements board.Store.
type SQLiteStore struct {
	db *sql.DB
}

var _ board.Store = (*SQLiteStore)(nil)

// Open opens (creating if necessary) the SQLite database at path, applies
// the required connection pragmas and runs pending migrations.
func Open(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}

	if err := applyPragmas(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("store: apply pragma %q: %w", p, err)
		}
	}
	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Upsert creates or updates the row for v.CustomerID. On update the stored
// slug is kept and newSlug is never called. On insert, newSlug is called
// (and retried on a slug collision) up to maxSlugAttempts times.
func (s *SQLiteStore) Upsert(ctx context.Context, v board.CustomerView, newSlug func() (string, error)) (slug string, created bool, err error) {
	productsJSON, err := json.Marshal(v.Products)
	if err != nil {
		return "", false, fmt.Errorf("store: marshal products: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("store: begin upsert: %w", err)
	}
	defer tx.Rollback()

	var existingSlug string
	err = tx.QueryRowContext(ctx, `SELECT slug FROM customer_views WHERE customer_id = ?`, v.CustomerID).Scan(&existingSlug)
	switch {
	case err == nil:
		if err := update(ctx, tx, v, productsJSON); err != nil {
			return "", false, err
		}
		if err := tx.Commit(); err != nil {
			return "", false, fmt.Errorf("store: commit update: %w", err)
		}
		return existingSlug, false, nil

	case errors.Is(err, sql.ErrNoRows):
		slug, err := insertWithRetry(ctx, tx, v, productsJSON, newSlug)
		if err != nil {
			return "", false, err
		}
		if err := tx.Commit(); err != nil {
			return "", false, fmt.Errorf("store: commit insert: %w", err)
		}
		return slug, true, nil

	default:
		return "", false, fmt.Errorf("store: lookup customer: %w", err)
	}
}

func update(ctx context.Context, tx *sql.Tx, v board.CustomerView, productsJSON []byte) error {
	lang := v.Language
	if lang == "" {
		lang = "en"
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE customer_views
		SET client_name = ?, intro = ?, language = ?, products = ?, updated_at = ?
		WHERE customer_id = ?`,
		v.ClientName, v.Intro, lang, string(productsJSON), now(), v.CustomerID)
	if err != nil {
		return fmt.Errorf("store: update customer: %w", err)
	}
	return nil
}

func insertWithRetry(ctx context.Context, tx *sql.Tx, v board.CustomerView, productsJSON []byte, newSlug func() (string, error)) (string, error) {
	ts := now()
	lang := v.Language
	if lang == "" {
		lang = "en"
	}
	for attempt := 0; attempt < maxSlugAttempts; attempt++ {
		slug, err := newSlug()
		if err != nil {
			return "", fmt.Errorf("store: generate slug: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO customer_views (customer_id, slug, client_name, intro, language, products, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.CustomerID, slug, v.ClientName, v.Intro, lang, string(productsJSON), ts, ts)
		if err == nil {
			return slug, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return "", fmt.Errorf("store: insert customer: %w", err)
	}
	return "", board.ErrSlugExhausted
}

func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqliteConstraintUnique
}

// BySlug looks up a CustomerView by its public slug. It returns
// board.ErrUnknownSlug if no customer has that slug.
func (s *SQLiteStore) BySlug(ctx context.Context, slug string) (board.CustomerView, error) {
	var (
		v            board.CustomerView
		productsJSON string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT customer_id, slug, client_name, intro, language, products
		FROM customer_views WHERE slug = ?`, slug,
	).Scan(&v.CustomerID, &v.Slug, &v.ClientName, &v.Intro, &v.Language, &productsJSON)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return board.CustomerView{}, board.ErrUnknownSlug
	case err != nil:
		return board.CustomerView{}, fmt.Errorf("store: lookup slug: %w", err)
	}

	if err := json.Unmarshal([]byte(productsJSON), &v.Products); err != nil {
		return board.CustomerView{}, fmt.Errorf("store: unmarshal products: %w", err)
	}
	return v, nil
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
