package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
)

// TestDraftSaveRefusesTombstonedChapter is the M6 regression: after a chapter
// is soft-deleted, an ensure-based draft save must surface an explicit
// conflict, never a successful no-op. Tombstone number uniqueness is kept.
func TestDraftSaveRefusesTombstonedChapter(t *testing.T) {
	f := provisionNarrative(t, "tomb-proj", "tomb-story")
	ctx := context.Background()

	if err := f.nar.SaveDraft(ctx, f.scope, 1, "bản đầu"); err != nil {
		t.Fatalf("save live draft: %v", err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `UPDATE chapters
		SET deleted_at = clock_timestamp(), updated_at = clock_timestamp(),
		    version = version + 1
		WHERE project_id = $1 AND story_id = $2 AND number = 1`,
		f.scope.ProjectID, f.scope.StoryID); err != nil {
		t.Fatalf("soft-delete chapter: %v", err)
	}

	if err := f.nar.SaveDraft(ctx, f.scope, 1, "bản sau"); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("save over tombstone = %v, want ErrStorageConflict", err)
	}
	if err := f.nar.SaveDraftCAS(ctx, f.scope, 1, "bản cas", 1); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("CAS save over tombstone = %v, want ErrStorageConflict", err)
	}
	if got, err := f.nar.LoadDraft(ctx, f.scope, 1); err != nil || got != "" {
		t.Fatalf("tombstoned draft = %q, %v; want empty", got, err)
	}
}

// TestMutableGuardsRequireVersionBump is the S1 regression: every UPDATE on a
// mutable business row must bump version once and change updated_at. The
// trigger is a backstop; a plain UPDATE is rejected.
func TestMutableGuardsRequireVersionBump(t *testing.T) {
	f := provisionNarrative(t, "guard-proj", "guard-story")
	ctx := context.Background()
	userID := deterministicUUID("user", "guard-user")
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO users
		(id, username, display_name, global_role, status)
		VALUES ($1, 'guard-user', 'Guard', 'member', 'active')`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if _, err := f.pool.Pool().Exec(ctx,
		`UPDATE users SET display_name = 'Mất chặn' WHERE id = $1`, userID); err == nil {
		t.Fatal("users UPDATE without version bump was accepted")
	}
	if _, err := f.pool.Pool().Exec(ctx, `UPDATE users
		SET display_name = 'Có chặn', updated_at = clock_timestamp(),
		    version = version + 1
		WHERE id = $1`, userID); err != nil {
		t.Fatalf("guarded users UPDATE: %v", err)
	}
}

// TestSessionTouchBumpsVersion is the S1 auth-caller fix: the opportunistic
// last-seen touch participates in the same row contract instead of bypassing
// it.
func TestSessionTouchBumpsVersion(t *testing.T) {
	f := provisionNarrative(t, "touch-proj", "touch-story")
	ctx := context.Background()
	userID := deterministicUUID("user", "touch-user")
	deviceID := deterministicUUID("device", "touch-device")
	sessionID := deterministicUUID("session", "touch-session")
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO users
		(id, username, display_name, global_role, status)
		VALUES ($1, 'touch-user', 'Touch', 'member', 'active')`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO devices
		(id, user_id, name) VALUES ($1, $2, 'máy thử')`, deviceID, userID); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO sessions
		(id, user_id, device_id, token_hash, credential_kind, last_seen_at, expires_at)
		VALUES ($1, $2, $3, $4, 'password', clock_timestamp(), clock_timestamp() + interval '1 hour')`,
		sessionID, userID, deviceID, make([]byte, 32)); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `UPDATE sessions
		SET last_seen_at = $2, updated_at = clock_timestamp(), version = version + 1
		WHERE id = $1 AND revoked_at IS NULL`,
		sessionID, pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}); err != nil {
		t.Fatalf("touch session: %v", err)
	}
	var version int64
	if err := f.pool.Pool().QueryRow(ctx,
		`SELECT version FROM sessions WHERE id = $1`, sessionID).Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != 2 {
		t.Fatalf("session version after touch = %d, want 2", version)
	}
}

// TestAuditRetentionIsNotAReadSideEffect is the S13 regression: reading the
// audit page must never delete history; only an explicit maintenance purge
// (privileged principal + marker) may.
func TestAuditRetentionIsNotAReadSideEffect(t *testing.T) {
	f := provisionNarrative(t, "audit-proj", "audit-story")
	ctx := context.Background()
	accounts := identityRepo(f.pool)
	id := deterministicUUID("audit", "old-entry")
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO audit_logs
		(id, actor_kind, action, target_kind, target_id, occurred_at)
		VALUES ($1, 'system', 'thử', 'acct', 'x', clock_timestamp() - interval '400 days')`,
		id); err != nil {
		t.Fatalf("insert old audit: %v", err)
	}

	page, err := accounts.Audit(identity.AuditQuery{Limit: 10})
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if len(page.Items) == 0 {
		t.Fatal("audit read returned no rows")
	}

	purged, err := accounts.PurgeAuditBefore(ctx, time.Now().UTC().Add(-365*24*time.Hour), false)
	if err != nil {
		t.Fatalf("maintenance purge: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purged %d rows, want 1", purged)
	}
	if page, err = accounts.Audit(identity.AuditQuery{Limit: 10}); err != nil || len(page.Items) != 0 {
		t.Fatalf("audit after purge = %d, %v; want 0", len(page.Items), err)
	}
}

// TestCheckMigrationsRejectsOlderSchema is the M3 regression: format
// compatibility alone is not readiness; every embedded migration must be
// applied (newer additive schemas stay acceptable).
func TestCheckMigrationsRejectsOlderSchema(t *testing.T) {
	f := provisionNarrative(t, "mig-proj", "mig-story")
	ctx := context.Background()

	if err := f.pool.CheckMigrations(ctx); err != nil {
		t.Fatalf("fully migrated schema refused: %v", err)
	}
	required := postgres.RequiredMigrations()
	if len(required) == 0 {
		t.Fatal("embedded migration set is empty")
	}
	last := required[len(required)-1]
	if _, err := f.pool.Pool().Exec(ctx,
		`DELETE FROM goose_db_version WHERE version_id = $1`, last); err != nil {
		t.Fatalf("simulate older schema: %v", err)
	}
	if err := f.pool.CheckMigrations(ctx); !errors.Is(err, errs.ErrStorageUnsupported) {
		t.Fatalf("partially migrated schema = %v, want ErrStorageUnsupported", err)
	}
}
