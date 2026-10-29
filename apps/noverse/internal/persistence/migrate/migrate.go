// Package migrate binds the app's embedded migrations to the shared runner in
// platform/pgmigrate. The application pool never runs migrations and the
// application role never has DDL grants — schema changes are owned by this
// package alone (one migration owner, explicit version progression).
package migrate

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrations"
	"github.com/datnt1998/verse/platform/pgmigrate"
)

// OpenDB opens a dedicated migration connection using the pgx stdlib adapter.
// It is the ONLY sql.DB in the codebase: the application path uses pgxpool
// natively and this pool must be closed by the caller once migrations finish.
func OpenDB(dsn string) (*sql.DB, error) {
	return pgmigrate.OpenDB(dsn)
}

// Up applies every pending migration and returns the applied results in
// order. A second Up is a no-op (empty result, nil error).
func Up(ctx context.Context, db *sql.DB) ([]*goose.MigrationResult, error) {
	return pgmigrate.Up(ctx, db, migrations.FS)
}

// StatusEntry reports one migration's applied state.
type StatusEntry = pgmigrate.StatusEntry

// Status returns all migrations merged from filesystem and database state.
func Status(ctx context.Context, db *sql.DB) ([]StatusEntry, error) {
	return pgmigrate.Status(ctx, db, migrations.FS)
}

// Version returns the highest applied migration version (0 = unmigrated).
func Version(ctx context.Context, db *sql.DB) (int64, error) {
	return pgmigrate.Version(ctx, db, migrations.FS)
}
