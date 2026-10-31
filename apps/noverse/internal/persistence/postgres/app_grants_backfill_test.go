package postgres_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrations"
	"github.com/datnt1998/verse/platform/pgtest"
)

func gooseTo(t *testing.T, db *sql.DB, version int64) {
	t.Helper()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.UpToContext(context.Background(), db, ".", version); err != nil {
		t.Fatalf("migrate to %d: %v", version, err)
	}
}

// TestAppGrantBackfillKeepsExistingAccounts applies only the migrations that
// precede the grant table, seeds live/locked/soft-deleted accounts, then
// applies the grant migration: every live account (locked included) keeps a
// Noverse grant, tombstones get none, and no Viderse grant is manufactured.
func TestAppGrantBackfillKeepsExistingAccounts(t *testing.T) {
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	gooseTo(t, db, 11)

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users (id, username, display_name, global_role, status, deleted_at)
		VALUES
			('00000000-0000-0000-0000-000000000101', 'old-active', 'Active', 'member', 'active', NULL),
			('00000000-0000-0000-0000-000000000102', 'old-locked', 'Locked', 'member', 'locked', NULL),
			('00000000-0000-0000-0000-000000000103', 'old-deleted', 'Deleted', 'member', 'active', now())`); err != nil {
		t.Fatalf("seed pre-grant accounts: %v", err)
	}

	gooseTo(t, db, 12)

	rows, err := db.QueryContext(ctx, `
		SELECT u.username, g.app, g.role
		FROM users u
		LEFT JOIN app_grants g ON g.user_id = u.id
		ORDER BY u.username, g.app`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string][]string{}
	for rows.Next() {
		var username, app, role sql.NullString
		if err := rows.Scan(&username, &app, &role); err != nil {
			t.Fatal(err)
		}
		if app.Valid {
			got[username.String] = append(got[username.String], app.String+":"+role.String)
		} else {
			got[username.String] = nil
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got["old-active"]) != 1 || got["old-active"][0] != "noverse:member" {
		t.Fatalf("live account backfill = %v", got["old-active"])
	}
	if len(got["old-locked"]) != 1 || got["old-locked"][0] != "noverse:member" {
		t.Fatalf("locked account must keep its access: %v", got["old-locked"])
	}
	if len(got["old-deleted"]) != 0 {
		t.Fatalf("tombstone received a grant: %v", got["old-deleted"])
	}
	var viderse int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM app_grants WHERE app = 'viderse'`).Scan(&viderse); err != nil {
		t.Fatal(err)
	}
	if viderse != 0 {
		t.Fatalf("migration manufactured %d Viderse grants", viderse)
	}
	// The grant table is anchored to users with ON DELETE RESTRICT, and the
	// primary key is (user_id, app).
	var restricted bool
	if err := db.QueryRowContext(ctx, `
		SELECT confdeltype = 'r' FROM pg_constraint
		WHERE conrelid = 'app_grants'::regclass AND contype = 'f'`).Scan(&restricted); err != nil {
		t.Fatal(err)
	}
	if !restricted {
		t.Fatal("app_grants.user_id must be ON DELETE RESTRICT")
	}
}
