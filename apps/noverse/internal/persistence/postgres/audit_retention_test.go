package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

func TestAuditRetentionPreviewBoundaryAndChunkRecovery(t *testing.T) {
	f := provisionNarrative(t, "retention-proj", "retention-story")
	ctx := context.Background()
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO audit_logs(id,actor_kind,action,target_kind,target_id,occurred_at)
 SELECT md5('retention-'||n)::uuid,'system','test','test',n::text,$1::timestamptz-interval '1 second' FROM generate_series(1,1001)n;
 `, cutoff); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO audit_logs(id,actor_kind,action,target_kind,target_id,occurred_at)
 VALUES('00000000-0000-4000-8000-000000000099','system','test','test','boundary',$1)`, cutoff); err != nil {
		t.Fatal(err)
	}
	accounts := identityRepo(f.pool)
	// Reject all DELETE statements while preview executes: dry-run must not even
	// issue an empty DELETE or maintenance UPDATE.
	if _, err := f.pool.Pool().Exec(ctx, `CREATE FUNCTION reject_retention_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'unexpected retention write'; END $$;
 CREATE TRIGGER reject_retention_write BEFORE UPDATE OR DELETE ON audit_logs FOR EACH STATEMENT EXECUTE FUNCTION reject_retention_write();`); err != nil {
		t.Fatal(err)
	}
	n, err := accounts.PurgeAuditBefore(ctx, cutoff, true)
	if err != nil || n != 1001 {
		t.Fatalf("dry preview: count=%d err=%v", n, err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `DROP TRIGGER reject_retention_write ON audit_logs; DROP FUNCTION reject_retention_write();
 CREATE FUNCTION interrupt_retention() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (SELECT count(*) FROM audit_logs WHERE occurred_at<'2026-01-01')<=1 THEN RAISE EXCEPTION 'injected retention interruption'; END IF; RETURN NULL; END $$;
 CREATE TRIGGER interrupt_retention BEFORE DELETE ON audit_logs FOR EACH STATEMENT EXECUTE FUNCTION interrupt_retention();`); err != nil {
		t.Fatal(err)
	}
	n, err = accounts.PurgeAuditBefore(ctx, cutoff, false)
	if err == nil || n != 1000 {
		t.Fatalf("must commit first bounded chunk before interruption: count=%d err=%v", n, err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `DROP TRIGGER interrupt_retention ON audit_logs; DROP FUNCTION interrupt_retention()`); err != nil {
		t.Fatal(err)
	}
	n, err = accounts.PurgeAuditBefore(ctx, cutoff, false)
	if err != nil || n != 1 {
		t.Fatalf("retention restart: count=%d err=%v", n, err)
	}
	var boundary int
	if err := f.pool.Pool().QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE occurred_at=$1`, cutoff).Scan(&boundary); err != nil || boundary != 1 {
		t.Fatalf("boundary removed: count=%d err=%v", boundary, err)
	}
}

func TestAuditRetentionPreviewRejectsAppRole(t *testing.T) {
	appDSN, maintenanceDSN := pgtest.ProvisionRoles(t, pgtest.RoleSpec{MigrateRole: "noverse_migrate", AppRole: "noverse_app", LockKey1: 2710, LockKey2: 11, InitScriptRef: "deploy/postgres-init/01-roles.sh"})
	db, err := migrate.OpenDB(maintenanceDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := migrate.Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	app, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: appDSN})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	for _, dry := range []bool{true, false} {
		_, err := identityRepo(app).PurgeAuditBefore(context.Background(), time.Now(), dry)
		if err == nil || !strings.Contains(err.Error(), "noverse_migrate") {
			t.Fatalf("app accepted dry=%v: %v", dry, err)
		}
	}
}
