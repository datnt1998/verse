// Package pgmigrate applies embedded goose migrations through a dedicated
// migration-role connection. The application pool never runs migrations and
// the application role never has DDL grants — schema changes are owned by
// this package alone (one migration owner, explicit version progression).
//
// Apps pass their embedded migration FS (goose *.sql files); each app owns
// its own database, so goose_db_version never collides.
package pgmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// OpenDB opens a dedicated migration connection using the pgx stdlib adapter.
// It is the ONLY sql.DB in the codebase: the application path uses pgxpool
// natively and this pool must be closed by the caller once migrations finish.
// The DSN is parsed by pgx so search_path and other runtime params survive.
func OpenDB(dsn string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid migration database config: %w", err)
	}
	db := stdlib.OpenDB(*cfg)
	// migrations are serial; one connection is both sufficient and honest
	db.SetMaxOpenConns(1)
	return db, nil
}

func newProvider(db *sql.DB, fsys fs.FS) (*goose.Provider, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("build session locker: %w", err)
	}
	return goose.NewProvider(goose.DialectPostgres, db, fsys,
		goose.WithSessionLocker(locker),
		goose.WithVerbose(false))
}

// Up applies every pending migration and returns the applied results in
// order. A second Up is a no-op (empty result, nil error) — idempotent by
// construction, and the session locker serialises concurrent migrators.
func Up(ctx context.Context, db *sql.DB, fsys fs.FS) ([]*goose.MigrationResult, error) {
	p, err := newProvider(db, fsys)
	if err != nil {
		return nil, err
	}
	res, err := p.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate up: %w", err)
	}
	for _, r := range res {
		if r.Error != nil {
			return res, fmt.Errorf("migration %s failed: %w", r.Source.Path, r.Error)
		}
	}
	return res, nil
}

// StatusEntry reports one migration's applied state.
type StatusEntry struct {
	Version int64
	Applied bool
	Source  string
}

// Status returns all migrations merged from filesystem and database state.
func Status(ctx context.Context, db *sql.DB, fsys fs.FS) ([]StatusEntry, error) {
	p, err := newProvider(db, fsys)
	if err != nil {
		return nil, err
	}
	st, err := p.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate status: %w", err)
	}
	out := make([]StatusEntry, 0, len(st))
	for _, s := range st {
		if s.Source == nil {
			continue
		}
		out = append(out, StatusEntry{
			Version: s.Source.Version,
			Applied: s.State == goose.StateApplied,
			Source:  s.Source.Path,
		})
	}
	return out, nil
}

// Version returns the highest applied migration version (0 = unmigrated).
func Version(ctx context.Context, db *sql.DB, fsys fs.FS) (int64, error) {
	p, err := newProvider(db, fsys)
	if err != nil {
		return 0, err
	}
	v, err := p.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("migrate version: %w", err)
	}
	return v, nil
}
