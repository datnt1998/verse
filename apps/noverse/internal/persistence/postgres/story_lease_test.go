package postgres_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

func storeFromBackend(t *testing.T, b *postgres.Backend) *storepkg.Store {
	t.Helper()
	s, err := storepkg.NewFromBackend(b)
	if err != nil {
		t.Fatalf("store from backend: %v", err)
	}
	return s
}

// TestStoryLeaseFence proves the durable write fence: a claimed lease allows
// writes, a takeover by another holder turns the old writer's next transaction
// into ErrLeaseLost, and the new holder's writes commit.
func TestStoryLeaseFence(t *testing.T) {
	f := provisionNarrative(t, "lease-proj", "lease-story")
	ctx := context.Background()

	lease, err := postgres.NewStoryLease(ctx, f.nar, f.scope, 2*time.Minute)
	if err != nil {
		t.Fatalf("claim lease: %v", err)
	}
	defer lease.Close()

	st := storeFromBackend(t, postgres.NewBackend(ctx, f.pool, f.nar, f.scope, postgres.WithLeaseFence(lease)))
	if err := st.Progress.Init(3); err != nil {
		t.Fatalf("progress init under lease: %v", err)
	}

	// A second claim while the first is live must fail — no steal path.
	if other, err := postgres.NewStoryLease(ctx, f.nar, f.scope, 2*time.Minute); err == nil {
		_ = other.Close()
		t.Fatal("second live claim succeeded; lease is not exclusive")
	} else if !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("second claim error = %v, want ErrLeaseLost", err)
	}

	// Simulate an external takeover: another holder owns the row with a higher
	// fencing token (what a restarted owner would see).
	var foreign pgtype.UUID
	if _, err := rand.Read(foreign.Bytes[:]); err != nil {
		t.Fatalf("holder id: %v", err)
	}
	foreign.Valid = true
	if _, err := f.pool.Pool().Exec(ctx, `
		UPDATE story_leases SET holder_id = $3, fencing_token = fencing_token + 1,
			lease_until = now() + interval '10 minutes', updated_at = clock_timestamp(),
			version = version + 1
		WHERE project_id = $1 AND story_id = $2`,
		f.scope.ProjectID, f.scope.StoryID, foreign); err != nil {
		t.Fatalf("takeover: %v", err)
	}

	if err := st.Progress.Init(4); !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("write after takeover = %v, want ErrLeaseLost", err)
	}

	// A new holder with its own fence proceeds once the foreign lease is no
	// longer live (what happens after the dead owner's TTL passes).
	if _, err := f.pool.Pool().Exec(ctx, `
		UPDATE story_leases SET lease_until = now() - interval '1 minute',
			updated_at = clock_timestamp(), version = version + 1
		WHERE project_id = $1 AND story_id = $2`,
		f.scope.ProjectID, f.scope.StoryID); err != nil {
		t.Fatalf("expire foreign lease: %v", err)
	}
	newLease, err := postgres.NewStoryLease(ctx, f.nar, f.scope, 2*time.Minute)
	if err != nil {
		t.Fatalf("claim after takeover: %v", err)
	}
	defer newLease.Close()
	newStore := storeFromBackend(t, postgres.NewBackend(ctx, f.pool, f.nar, f.scope, postgres.WithLeaseFence(newLease)))
	if err := newStore.Progress.Init(4); err != nil {
		t.Fatalf("progress init under new lease: %v", err)
	}
}

// TestStoryLeaseReleasedCannotWrite proves a released lease cannot write.
func TestStoryLeaseReleasedCannotWrite(t *testing.T) {
	f := provisionNarrative(t, "lease-proj-2", "lease-story-2")
	ctx := context.Background()
	lease, err := postgres.NewStoryLease(ctx, f.nar, f.scope, 2*time.Minute)
	if err != nil {
		t.Fatalf("claim lease: %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatalf("release lease: %v", err)
	}
	st := storeFromBackend(t, postgres.NewBackend(ctx, f.pool, f.nar, f.scope, postgres.WithLeaseFence(lease)))
	if err := st.Progress.Init(2); !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("write after release = %v, want ErrLeaseLost", err)
	}
}

func foreignHolder(t *testing.T) pgtype.UUID {
	t.Helper()
	var holder pgtype.UUID
	if _, err := rand.Read(holder.Bytes[:]); err != nil {
		t.Fatalf("holder id: %v", err)
	}
	holder.Valid = true
	return holder
}

// takeOverLease hands the story to another holder the way a restarted owner
// claims it: new holder, next fencing token, fresh lifetime.
func takeOverLease(ctx context.Context, f pgFixture, holder pgtype.UUID) error {
	_, err := f.pool.Pool().Exec(ctx, `
		UPDATE story_leases SET holder_id = $3, fencing_token = fencing_token + 1,
			lease_until = now() + interval '10 minutes', updated_at = clock_timestamp(),
			version = version + 1
		WHERE project_id = $1 AND story_id = $2`,
		f.scope.ProjectID, f.scope.StoryID, holder)
	return err
}

// TestStoryLeaseFencesTypedWrites proves chapter acceptance and reviews are
// fenced like documents: after a takeover the stale engine appends no version.
func TestStoryLeaseFencesTypedWrites(t *testing.T) {
	f := provisionNarrative(t, "lease-typed", "lease-typed-story")
	ctx := context.Background()
	lease, err := postgres.NewStoryLease(ctx, f.nar, f.scope, 2*time.Minute)
	if err != nil {
		t.Fatalf("claim lease: %v", err)
	}
	defer lease.Close()
	st := storeFromBackend(t, postgres.NewBackend(ctx, f.pool, f.nar, f.scope, postgres.WithLeaseFence(lease)))
	facts := domain.ChapterFacts{Title: "Đêm"}
	if _, err := st.ChapterRecords.Accept(1, domain.ChapterOriginGenerated, "Bản một.", facts, domain.StyleDelta{}); err != nil {
		t.Fatalf("accept under lease: %v", err)
	}

	if err := takeOverLease(ctx, f, foreignHolder(t)); err != nil {
		t.Fatalf("takeover: %v", err)
	}
	if _, err := st.ChapterRecords.Accept(1, domain.ChapterOriginGenerated, "Bản hai của writer cũ.", facts, domain.StyleDelta{}); !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("accept after takeover = %v, want ErrLeaseLost", err)
	}
	if err := st.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "rewrite", Summary: "cũ"}); !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("review after takeover = %v, want ErrLeaseLost", err)
	}

	reader := storeFromBackend(t, postgres.NewBackend(ctx, f.pool, f.nar, f.scope))
	record, err := reader.ChapterRecords.Load(1)
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	if record == nil || record.Revision != 1 || record.Content != "Bản một." {
		t.Fatalf("stale writer changed the accepted chapter: %+v", record)
	}
}

// TestStoryLeaseTakeoverWaitsForFencedWrite proves the fence holds the lease
// row until the write commits: a takeover cannot land between the fence check
// and the commit of the old holder's transaction.
func TestStoryLeaseTakeoverWaitsForFencedWrite(t *testing.T) {
	f := provisionNarrative(t, "lease-race", "lease-race-story")
	ctx := context.Background()
	lease, err := postgres.NewStoryLease(ctx, f.nar, f.scope, 2*time.Minute)
	if err != nil {
		t.Fatalf("claim lease: %v", err)
	}
	defer lease.Close()

	tx, err := f.pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lease.Checker()(ctx, tx); err != nil {
		t.Fatalf("fence under live lease: %v", err)
	}

	done := make(chan error, 1)
	holder := foreignHolder(t)
	go func() { done <- takeOverLease(ctx, f, holder) }()
	select {
	case err := <-done:
		t.Fatalf("takeover finished while a fenced write was still open (err=%v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit fenced write: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("takeover after commit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("takeover still blocked after the fenced write committed")
	}
}

// TestStoryLeaseRenewalNeverReclaims proves renewal only extends a lease the
// holder still owns: once lost, a free row is not silently taken back, so the
// stale holder's next write fails instead of overwriting the other holder's work.
func TestStoryLeaseRenewalNeverReclaims(t *testing.T) {
	f := provisionNarrative(t, "lease-renew", "lease-renew-story")
	ctx := context.Background()
	// A 3s lifetime renews every second.
	lease, err := postgres.NewStoryLease(ctx, f.nar, f.scope, 3*time.Second)
	if err != nil {
		t.Fatalf("claim lease: %v", err)
	}
	defer lease.Close()
	st := storeFromBackend(t, postgres.NewBackend(ctx, f.pool, f.nar, f.scope, postgres.WithLeaseFence(lease)))

	if err := takeOverLease(ctx, f, foreignHolder(t)); err != nil {
		t.Fatalf("takeover: %v", err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `
		UPDATE story_leases SET holder_id = NULL, lease_until = NULL,
			updated_at = clock_timestamp(), version = version + 1
		WHERE project_id = $1 AND story_id = $2`,
		f.scope.ProjectID, f.scope.StoryID); err != nil {
		t.Fatalf("foreign release: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for lease.Err() == nil && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if lease.Err() == nil {
		t.Fatal("renewal took a lost lease back instead of reporting it lost")
	}
	if err := st.Progress.Init(2); !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("write after lost renewal = %v, want ErrLeaseLost", err)
	}
}

// TestDocumentWriteReportsCommitFailure proves a document write returns the
// failure of its own commit instead of reporting success and surfacing the
// error on some later operation.
func TestDocumentWriteReportsCommitFailure(t *testing.T) {
	f := provisionNarrative(t, "doc-commit", "doc-commit-story")
	ctx := context.Background()
	st := storeFromBackend(t, postgres.NewBackend(ctx, f.pool, f.nar, f.scope))
	if err := st.Progress.Init(2); err != nil {
		t.Fatalf("progress init: %v", err)
	}
	// A deferred constraint trigger only fires at COMMIT.
	for _, stmt := range []string{
		`CREATE FUNCTION reject_at_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'commit rejected'; END $$`,
		`CREATE CONSTRAINT TRIGGER reject_at_commit AFTER INSERT OR UPDATE ON story_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_at_commit()`,
	} {
		if _, err := f.pool.Pool().Exec(ctx, stmt); err != nil {
			t.Fatalf("install commit trigger: %v", err)
		}
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err == nil {
		t.Fatal("write reported success although its commit was rejected")
	}
	if _, err := f.pool.Pool().Exec(ctx, `DROP TRIGGER reject_at_commit ON story_state`); err != nil {
		t.Fatalf("drop commit trigger: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("write after a failed commit must succeed: %v", err)
	}
	progress, err := st.Progress.Load()
	if err != nil {
		t.Fatalf("load progress: %v", err)
	}
	if progress.Phase != domain.PhaseWriting {
		t.Fatalf("phase = %q, want %q", progress.Phase, domain.PhaseWriting)
	}
}
