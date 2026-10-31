package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestStorageProbeConfigApplicationDeleteBoundary(t *testing.T) {
	m, dsn := publicationFixture(t, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var exists bool
	if err := m.pool.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT FROM pg_roles WHERE rolname='noverse_app' AND NOT rolsuper)`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("test stack requires a non-superuser noverse_app role")
	}
	// Production grants schema usage during provisioning. Apply that grant only
	// to this test's isolated schema, never modify the shared role or public data.
	if _, err := m.pool.Pool().Exec(ctx, `GRANT USAGE ON SCHEMA `+pgx.Identifier{pgtest.SchemaOf(dsn)}.Sanitize()+` TO noverse_app`); err != nil {
		t.Fatal(err)
	}
	if err := reviewPublish(ctx, m, 7); err != nil {
		t.Fatal(err)
	}
	active, err := m.active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := m.pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	ids := map[string]string{"active": active}
	for _, kind := range []string{"pending", "migration source", "migration target", "blob", "probe", "orphan"} {
		id, err := m.saveConfig(ctx, tx, m.configs[active])
		if err != nil {
			t.Fatal(err)
		}
		ids[kind] = id
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_settings SET pending_store=$1`, ids["pending"]); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO storage_migrations(id,source_store,target_store,state) VALUES('retained-migration',$1,$2,'completed')`, ids["migration source"], ids["migration target"]); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE blobs SET store_id=$1,version=version+1,updated_at=clock_timestamp()`, ids["blob"]); err != nil {
		t.Fatal(err)
	}
	key, err := m.keys.Staging("b1111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO storage_probes(store_id,object_key,expires_at) VALUES($1,$2,now()+interval '1 minute')`, ids["probe"], key); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	app, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close(context.Background())
	if _, err = app.Exec(ctx, `SET ROLE noverse_app`); err != nil {
		t.Fatal(err)
	}
	// Positive capability check: an expired probe's now-unreferenced config can
	// really be deleted by the serving role, not just a maintenance test DSN.
	tag, err := app.Exec(ctx, `DELETE FROM blob_stores WHERE id=$1`, ids["orphan"])
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("app cannot reclaim unreferenced probe config: %v", err)
	}
	for _, kind := range []string{"active", "pending", "migration source", "migration target", "blob", "probe"} {
		t.Run(kind, func(t *testing.T) {
			_, err := app.Exec(ctx, `DELETE FROM blob_stores WHERE id=$1`, ids[kind])
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
				t.Fatalf("app must be stopped by reference FK for %s: %v", kind, err)
			}
		})
	}
	_, err = app.Exec(ctx, `DELETE FROM blobs WHERE false`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("app gained registry DELETE privilege: %v", err)
	}
	if _, err = app.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatal(err)
	}
	// Reaper filtering must independently respect all of the same references.
	if err = m.pruneUnusedStoreConfigs(ctx); err != nil {
		t.Fatal(err)
	}
	var retained int
	if err = m.pool.Pool().QueryRow(ctx, `SELECT count(*) FROM blob_stores`).Scan(&retained); err != nil || retained != 6 {
		t.Fatalf("referenced configs lost: count=%d err=%v", retained, err)
	}
}

// Production provisioning gives the migrator's future tables app DML by
// default. Fresh migrations must explicitly remove DELETE from durable storage
// settings and history, rather than relying on a superuser fixture's defaults.
func TestStorageSettingsApplicationDeleteBoundary(t *testing.T) {
	pgtest.ProvisionRoles(t, pgtest.RoleSpec{MigrateRole: "noverse_migrate", AppRole: "noverse_app", LockKey1: 2710, LockKey2: 11, InitScriptRef: "deploy/postgres-init/01-roles.sh"})
	dsn := pgtest.Provision(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	schema := pgx.Identifier{pgtest.SchemaOf(dsn)}.Sanitize()
	if _, err := conn.Exec(ctx, `GRANT USAGE ON SCHEMA `+schema+` TO noverse_app;
		ALTER DEFAULT PRIVILEGES IN SCHEMA `+schema+` GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO noverse_app`); err != nil {
		t.Fatal(err)
	}
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, err = migrate.Up(ctx, db)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SET ROLE noverse_app`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"storage_settings", "storage_migrations", "storage_migration_items"} {
		t.Run(table, func(t *testing.T) {
			_, err := conn.Exec(ctx, "DELETE FROM "+pgx.Identifier{table}.Sanitize()+" WHERE false")
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Fatalf("app must not have DELETE on durable %s: %v", table, err)
			}
		})
	}
}
