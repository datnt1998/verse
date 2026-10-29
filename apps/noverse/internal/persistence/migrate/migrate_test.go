package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/datnt1998/noverse/internal/persistence/migrate"
	"github.com/datnt1998/noverse/internal/persistence/pgtest"
)

// TestMigrateFoundationUpIsIdempotent runs the migration twice in one schema;
// the second run must be a no-op rather than an error or a duplicate.
func TestMigrateFoundationUpIsIdempotent(t *testing.T) {
	dsn := pgtest.Provision(t)
	ctx := context.Background()

	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	defer db.Close()

	res, err := migrate.Up(ctx, db)
	if err != nil {
		t.Fatalf("first migrate up: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("first migrate up applied no migrations")
	}

	res2, err := migrate.Up(ctx, db)
	if err != nil {
		t.Fatalf("second migrate up: %v", err)
	}
	if len(res2) != 0 {
		t.Fatalf("second migrate up applied %d migrations, want 0", len(res2))
	}

	var foundation bool
	if err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM noverse_meta WHERE key = 'schema.format')").Scan(&foundation); err != nil {
		t.Fatalf("check foundation marker: %v", err)
	}
	if !foundation {
		t.Fatal("foundation marker noverse_meta[schema.format] missing")
	}
}

// TestMigrateVersion reports the last applied version.
func TestMigrateVersion(t *testing.T) {
	dsn := pgtest.Provision(t)
	ctx := context.Background()

	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	defer db.Close()

	if v, err := migrate.Version(ctx, db); err != nil {
		t.Fatalf("version before up: %v", err)
	} else if v != 0 {
		t.Fatalf("version before up = %d, want 0", v)
	}

	if _, err := migrate.Up(ctx, db); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	v, err := migrate.Version(ctx, db)
	if err != nil {
		t.Fatalf("version after up: %v", err)
	}
	if v < 1 {
		t.Fatalf("version after up = %d, want >= 1", v)
	}
}

// TestMigrateStatus distinguishes applied and pending migrations.
func TestMigrateStatus(t *testing.T) {
	dsn := pgtest.Provision(t)
	ctx := context.Background()

	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	defer db.Close()

	before, err := migrate.Status(ctx, db)
	if err != nil {
		t.Fatalf("status before up: %v", err)
	}
	pending := 0
	for _, st := range before {
		if !st.Applied {
			pending++
		}
	}
	if pending == 0 {
		t.Fatal("status before up shows no pending migrations")
	}

	if _, err := migrate.Up(ctx, db); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	after, err := migrate.Status(ctx, db)
	if err != nil {
		t.Fatalf("status after up: %v", err)
	}
	for _, st := range after {
		if !st.Applied {
			t.Fatalf("migration %d still pending after up", st.Version)
		}
	}
}

// TestMigrateConcurrentLocks runs two migrators on separate connections into
// the same schema. The session locker must serialise them: exactly one schema
// progression, no duplicate version rows, no deadlock error.
func TestMigrateConcurrentLocks(t *testing.T) {
	dsn := pgtest.Provision(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db1, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open first pool: %v", err)
	}
	defer db1.Close()
	db2, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open second pool: %v", err)
	}
	defer db2.Close()

	errsCh := make(chan error, 2)
	go func() { _, err := migrate.Up(ctx, db1); errsCh <- err }()
	go func() { _, err := migrate.Up(ctx, db2); errsCh <- err }()
	for range 2 {
		if err := <-errsCh; err != nil {
			t.Fatalf("concurrent migrate up: %v", err)
		}
	}

	v, err := migrate.Version(ctx, db1)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if v < 1 {
		t.Fatalf("version = %d, want >= 1", v)
	}
}

// TestMigrateApplicationRoleCannotDDL connects as a fresh unprivileged role
// and proves the application role model: it can read/write rows but cannot
// run DDL inside the migrated schema.
func TestMigrateApplicationRoleCannotDDL(t *testing.T) {
	adminDSN := pgtest.RequireURL(t)
	schemaDSN := pgtest.Provision(t)
	ctx := context.Background()

	db, err := migrate.OpenDB(schemaDSN)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(ctx, db); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()

	// create an application-like role and grant usage, never DDL rights
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("open admin conn: %v", err)
	}
	defer admin.Close()
	schema := pgtest.SchemaOf(schemaDSN) // provisioned schema, embedded in schemaDSN via search_path
	role := strings.TrimPrefix(schema, "pgtest_")
	role = "pgtest_app_" + role
	if len(role) > 60 {
		role = role[:60]
	}
	if _, err := admin.ExecContext(ctx,
		"CREATE ROLE "+role+" LOGIN PASSWORD 'apptest'"); err != nil {
		t.Fatalf("create app role: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP ROLE "+role)
	})
	if _, err := admin.ExecContext(ctx,
		"GRANT USAGE ON SCHEMA "+schema+" TO "+role); err != nil {
		t.Fatalf("grant schema usage: %v", err)
	}
	if _, err := admin.ExecContext(ctx,
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA "+schema+" TO "+role); err != nil {
		t.Fatalf("grant table DML: %v", err)
	}

	// swap credentials in the DSN text — pgx.ConnString() drops runtime
	// params (search_path), so build the app DSN from the provisioned one.
	appDSN := strings.Replace(schemaDSN, "://postgres:test@", "://"+role+":apptest@", 1)
	if appDSN == schemaDSN {
		t.Fatal("app DSN substitution failed; check TEST_DATABASE_URL userinfo")
	}
	appDB, err := migrate.OpenDB(appDSN)
	if err != nil {
		t.Fatalf("open app conn: %v", err)
	}
	defer appDB.Close()

	// DML works for the application role.
	if _, err := appDB.ExecContext(ctx,
		"INSERT INTO noverse_meta(key, value) VALUES ('role.probe', 'ok') ON CONFLICT (key) DO UPDATE SET value = 'ok'"); err != nil {
		t.Fatalf("app role DML failed: %v", err)
	}

	// DDL must be refused — this is the migration-role boundary.
	if _, err := appDB.ExecContext(ctx, "CREATE TABLE app_ddl_probe(id int)"); err == nil {
		t.Fatal("application role executed DDL; migration boundary missing")
	} else {
		var pgerr interface{ SQLState() string }
		if !errors.As(err, &pgerr) || pgerr.SQLState() != "42501" {
			t.Fatalf("DDL refusal expected insufficient_privilege (42501), got %v", err)
		}
	}
}
