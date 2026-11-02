package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// Hosted preparation regressions: a queued chapter without a bản đọc is an
// ordinary kind='tts' row whose input carries needsScript=true. The remote
// worker must never see it, the web compose runner claims it per exact story,
// cancellation/pause win by moving the fence, and a composed script + prepared
// job land atomically on the same row.

// enqueuePreparationJob seeds the hosted preparation row the web queue writes.
func enqueuePreparationJob(t *testing.T, jobs *postgres.Jobs, scope postgres.ScopeRef, key string, chapter int) postgres.Job {
	t.Helper()
	job, err := jobs.Enqueue(context.Background(), scope, postgres.JobTTS, key, map[string]any{
		"kind": "create", "volume": 1, "chapter": chapter,
		"quality": "high", "needsScript": true, "segments": []any{},
	}, nil, pgtype.UUID{})
	if err != nil {
		t.Fatalf("enqueue preparation %s: %v", key, err)
	}
	return job
}

func jobInputMap(t *testing.T, job postgres.Job) map[string]any {
	t.Helper()
	var in map[string]any
	if err := json.Unmarshal(job.Input, &in); err != nil {
		t.Fatalf("job input is not JSON: %v (%s)", err, job.Input)
	}
	return in
}

func jobKey(scope postgres.ScopeRef, id pgtype.UUID) postgres.JobKey {
	return postgres.JobKey{ProjectID: scope.ProjectID, ID: id}
}

// TestPreparationRemoteClaimExcludesPrepared: the Python worker claims real
// render work only. A preparation row (or a stringly-typed flag) stays
// invisible, while a malformed flag neither throws nor hides a normal job.
func TestPreparationRemoteClaimExcludesPrepared(t *testing.T) {
	f := provisionNarrative(t, "prep-claim-proj", "prep-claim-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	worker := registerTestWorker(t, jobs, "prep-claim-mac")

	normal := enqueueTestJob(t, jobs, f.scope, "render")
	prepared := enqueuePreparationJob(t, jobs, f.scope, "prepare", 2)
	stringFlag, err := jobs.Enqueue(ctx, f.scope, postgres.JobTTS, "prepare-string", map[string]any{
		"kind": "create", "volume": 1, "chapter": 3, "needsScript": "true",
	}, nil, pgtype.UUID{})
	if err != nil {
		t.Fatalf("enqueue string-flag job: %v", err)
	}
	malformed, err := jobs.Enqueue(ctx, f.scope, postgres.JobTTS, "render-malformed", map[string]any{
		"kind": "create", "volume": 1, "chapter": 4,
		"needsScript": map[string]any{"oops": true},
	}, nil, pgtype.UUID{})
	if err != nil {
		t.Fatalf("enqueue malformed-flag job: %v", err)
	}

	claimed := map[pgtype.UUID]bool{}
	for {
		job, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
		if errors.Is(err, postgres.ErrNoClaimableJob) {
			break
		}
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		claimed[job.ID] = true
	}
	if claimed[prepared.ID] || claimed[stringFlag.ID] {
		t.Fatalf("worker claimed a preparation row: prepared=%v stringFlag=%v", claimed[prepared.ID], claimed[stringFlag.ID])
	}
	if !claimed[normal.ID] {
		t.Fatal("worker did not claim the ordinary render job")
	}
	if !claimed[malformed.ID] {
		t.Fatal("a malformed needsScript value must not hide a normal job from the worker")
	}

	row, err := jobs.Get(ctx, jobKey(f.scope, prepared.ID))
	if err != nil {
		t.Fatalf("read preparation row: %v", err)
	}
	if row.State != postgres.JobQueued || row.WorkerID.Valid || row.LeaseUntil != nil {
		t.Fatalf("preparation row changed: state=%s worker=%v lease=%v", row.State, row.WorkerID.Valid, row.LeaseUntil)
	}
}

// TestPreparationClaimIsolationAndSiblings: a claim sees the oldest preparation
// of its exact story only — never a sibling story, never another project — and
// publishing one story leaves every other preparation row byte-identical.
func TestPreparationClaimIsolationAndSiblings(t *testing.T) {
	f := provisionNarrative(t, "prep-iso-proj", "prep-iso-a")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()

	if _, err := f.nar.CreateStory(ctx, "prep-iso-proj", nil, postgres.StoryInfo{
		Slug: "prep-iso-b", Title: "B", Status: "writing",
	}); err != nil {
		t.Fatalf("create sibling story: %v", err)
	}
	scopeB, err := f.nar.Resolve(ctx, "prep-iso-proj", "prep-iso-b", nil, false)
	if err != nil {
		t.Fatalf("resolve sibling story: %v", err)
	}
	if _, err := gen.New(f.pool.Pool()).EnsureProject(ctx, gen.EnsureProjectParams{
		ID: deterministicUUID("project", "prep-iso-proj-2"), Slug: "prep-iso-proj-2", Name: "prep-iso-proj-2",
	}); err != nil {
		t.Fatalf("anchor second project: %v", err)
	}
	if _, err := f.nar.CreateStory(ctx, "prep-iso-proj-2", nil, postgres.StoryInfo{
		Slug: "prep-iso-c", Title: "C", Status: "writing",
	}); err != nil {
		t.Fatalf("create other-project story: %v", err)
	}
	scopeC, err := f.nar.Resolve(ctx, "prep-iso-proj-2", "prep-iso-c", nil, false)
	if err != nil {
		t.Fatalf("resolve other-project story: %v", err)
	}

	jobA := enqueuePreparationJob(t, jobs, f.scope, "a-1", 1)
	jobB := enqueuePreparationJob(t, jobs, scopeB, "b-1", 2)
	jobC := enqueuePreparationJob(t, jobs, scopeC, "c-1", 3)

	claimA, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim story A: %v", err)
	}
	if claimA.ID != jobA.ID {
		t.Fatalf("claim story A got %v, want %v", claimA.ID, jobA.ID)
	}
	// The live lease on A blocks a second compose, and story B/C rows must not
	// leak into A's claim.
	if _, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("second claim of the same story = %v, want ErrNoClaimableJob", err)
	}
	claimB, err := jobs.ClaimPreparation(ctx, scopeB, time.Minute)
	if err != nil {
		t.Fatalf("claim story B: %v", err)
	}
	if claimB.ID != jobB.ID {
		t.Fatalf("claim story B got %v, want %v", claimB.ID, jobB.ID)
	}
	claimC, err := jobs.ClaimPreparation(ctx, scopeC, time.Minute)
	if err != nil {
		t.Fatalf("claim story C: %v", err)
	}
	if claimC.ID != jobC.ID {
		t.Fatalf("claim story C got %v, want %v", claimC.ID, jobC.ID)
	}

	keyB := jobKey(scopeB, jobB.ID)
	keyC := jobKey(scopeC, jobC.ID)
	beforeB, err := jobs.Get(ctx, keyB)
	if err != nil {
		t.Fatalf("read story B before publish: %v", err)
	}
	beforeC, err := jobs.Get(ctx, keyC)
	if err != nil {
		t.Fatalf("read story C before publish: %v", err)
	}

	completedA, err := jobs.CompletePreparationWith(ctx, f.scope, claimA.ID, claimA.FencingToken,
		func(ctx context.Context, tx pgx.Tx, job postgres.Job) (any, int32, error) {
			return map[string]any{"kind": "create", "volume": 1, "chapter": 1, "segments": []any{}}, 1, nil
		})
	if err != nil {
		t.Fatalf("complete story A preparation: %v", err)
	}
	if completedA.State != postgres.JobQueued || completedA.FencingToken != claimA.FencingToken+1 {
		t.Fatalf("prepared row = state %s fence %d, want queued fence %d", completedA.State, completedA.FencingToken, claimA.FencingToken+1)
	}

	afterB, err := jobs.Get(ctx, keyB)
	if err != nil {
		t.Fatalf("read story B after publish: %v", err)
	}
	afterC, err := jobs.Get(ctx, keyC)
	if err != nil {
		t.Fatalf("read story C after publish: %v", err)
	}
	for name, pair := range map[string][2]postgres.Job{"B": {beforeB, afterB}, "C": {beforeC, afterC}} {
		before, after := pair[0], pair[1]
		if after.State != postgres.JobQueued || after.FencingToken != before.FencingToken ||
			string(after.Input) != string(before.Input) || after.ProgressTotal != nil {
			t.Fatalf("story %s preparation changed: state=%s fence=%d/%d input=%s",
				name, after.State, after.FencingToken, before.FencingToken, after.Input)
		}
		if after.LeaseUntil == nil || before.LeaseUntil == nil || !after.LeaseUntil.Equal(*before.LeaseUntil) {
			t.Fatalf("story %s lease changed: %v -> %v", name, before.LeaseUntil, after.LeaseUntil)
		}
	}
}

// TestPreparationCancelBeatsCompose: the fence a runner holds is invalidated the
// moment the user cancels, so a compose that returns later can neither run its
// publish callback nor record anything.
func TestPreparationCancelBeatsCompose(t *testing.T) {
	f := provisionNarrative(t, "prep-cancel-proj", "prep-cancel-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()

	prepared := enqueuePreparationJob(t, jobs, f.scope, "prepare", 1)
	claimed, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	cancelled, err := jobs.Cancel(ctx, jobKey(f.scope, prepared.ID))
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.State != postgres.JobCancelled || cancelled.FencingToken != claimed.FencingToken+1 {
		t.Fatalf("cancel = state %s fence %d, want cancelled fence %d", cancelled.State, cancelled.FencingToken, claimed.FencingToken+1)
	}

	publishRan := false
	_, err = jobs.CompletePreparationWith(ctx, f.scope, prepared.ID, claimed.FencingToken,
		func(ctx context.Context, tx pgx.Tx, job postgres.Job) (any, int32, error) {
			publishRan = true
			return map[string]any{"ok": true}, 1, nil
		})
	if !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("complete after cancel = %v, want ErrJobLeaseLost", err)
	}
	if publishRan {
		t.Fatal("publish callback ran after the user cancelled")
	}
	if _, err := jobs.FailPreparation(ctx, f.scope, prepared.ID, claimed.FencingToken, map[string]any{"why": "late"}); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("fail after cancel = %v, want ErrJobLeaseLost", err)
	}
	if _, err := jobs.ReleasePreparation(ctx, f.scope, prepared.ID, claimed.FencingToken); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("release after cancel = %v, want ErrJobLeaseLost", err)
	}

	row, err := jobs.Get(ctx, jobKey(f.scope, prepared.ID))
	if err != nil {
		t.Fatalf("read cancelled row: %v", err)
	}
	if row.State != postgres.JobCancelled || jobInputMap(t, row)["needsScript"] != true {
		t.Fatalf("late compose changed the cancelled row: state=%s input=%s", row.State, row.Input)
	}
}

// TestPreparationCASConflictRollsBackScriptAndJob: a script write that loses its
// compare-and-swap aborts the whole transaction — including writes that already
// ran inside the callback — and leaves the preparation claim intact for a retry.
func TestPreparationCASConflictRollsBackScriptAndJob(t *testing.T) {
	f := provisionNarrative(t, "prep-cas-proj", "prep-cas-story")
	repo := postgres.NewAudio(f.pool)
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	seed, err := repo.SaveScript(ctx, f.scope, postgres.Script{
		ChapterID: chapterID, RevisionNo: 1, SourceRevision: int32(record.Revision),
		SourceSHA256: record.ContentSHA256, Status: "locked", Payload: []byte(`{"v":1}`),
	})
	if err != nil {
		t.Fatalf("seed script: %v", err)
	}
	prepared := enqueuePreparationJob(t, jobs, f.scope, "prepare", 1)
	claimed, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	newRevision := postgres.Script{
		ChapterID: chapterID, RevisionNo: 2, SourceRevision: int32(record.Revision),
		SourceSHA256: record.ContentSHA256, Status: "locked", Payload: []byte(`{"v":"rev2"}`),
	}
	staleEdit := postgres.Script{
		ChapterID: chapterID, RevisionNo: 1, SourceRevision: int32(record.Revision),
		SourceSHA256: record.ContentSHA256, Status: "locked", Payload: []byte(`{"v":"stale"}`),
	}
	_, err = jobs.CompletePreparationWith(ctx, f.scope, prepared.ID, claimed.FencingToken,
		func(ctx context.Context, tx pgx.Tx, job postgres.Job) (any, int32, error) {
			// A write that must disappear with the failed CAS below.
			if _, err := repo.SaveScriptBundleTx(ctx, tx, f.scope, newRevision, nil, nil, 0); err != nil {
				return nil, 0, err
			}
			if _, err := repo.SaveScriptBundleTx(ctx, tx, f.scope, staleEdit, nil, nil, seed.Version+5); err != nil {
				return nil, 0, err
			}
			return map[string]any{"should": "not land"}, 1, nil
		})
	if !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("stale CAS publish = %v, want ErrStorageConflict", err)
	}

	if _, err := repo.Script(ctx, f.scope.ProjectID, chapterID, 2); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("revision written before the failed CAS survived: %v", err)
	}
	latest, err := repo.LatestScript(ctx, f.scope.ProjectID, chapterID)
	if err != nil {
		t.Fatalf("latest script: %v", err)
	}
	var storedScript map[string]any
	if err := json.Unmarshal(latest.Payload, &storedScript); err != nil {
		t.Fatalf("stored script payload is not JSON: %v (%s)", err, latest.Payload)
	}
	if latest.RevisionNo != 1 || latest.Version != seed.Version || storedScript["v"] != float64(1) {
		t.Fatalf("stored script changed: revision=%d version=%d payload=%s", latest.RevisionNo, latest.Version, latest.Payload)
	}
	row, err := jobs.Get(ctx, jobKey(f.scope, prepared.ID))
	if err != nil {
		t.Fatalf("read preparation after rollback: %v", err)
	}
	if row.State != postgres.JobQueued || row.FencingToken != claimed.FencingToken || row.ProgressTotal != nil {
		t.Fatalf("job changed after rollback: state=%s fence=%d/%d total=%v", row.State, row.FencingToken, claimed.FencingToken, row.ProgressTotal)
	}
	if row.LeaseUntil == nil || !row.LeaseUntil.Equal(*claimed.LeaseUntil) {
		t.Fatalf("claim lease changed after rollback: %v -> %v", claimed.LeaseUntil, row.LeaseUntil)
	}
	if jobInputMap(t, row)["needsScript"] != true {
		t.Fatalf("preparation seed input changed after rollback: %s", row.Input)
	}

	// The same claim can retry and land the script + prepared job atomically.
	retried, err := jobs.CompletePreparationWith(ctx, f.scope, prepared.ID, claimed.FencingToken,
		func(ctx context.Context, tx pgx.Tx, job postgres.Job) (any, int32, error) {
			if _, err := repo.SaveScriptBundleTx(ctx, tx, f.scope, newRevision, nil, nil, 0); err != nil {
				return nil, 0, err
			}
			return map[string]any{"kind": "create", "volume": 1, "chapter": 1, "segments": []any{}}, 1, nil
		})
	if err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}
	if retried.State != postgres.JobQueued || retried.FencingToken != claimed.FencingToken+1 {
		t.Fatalf("retried row = state %s fence %d", retried.State, retried.FencingToken)
	}
	if _, err := repo.Script(ctx, f.scope.ProjectID, chapterID, 2); err != nil {
		t.Fatalf("retried script missing: %v", err)
	}
}

// TestPreparationCompletesSameRowClaimable: publish saves the locked script and
// the SAME job row becomes the worker's render job — same id, attempt metadata
// preserved, needsScript stripped, fence moved, input claimable.
func TestPreparationCompletesSameRowClaimable(t *testing.T) {
	f := provisionNarrative(t, "prep-done-proj", "prep-done-story")
	repo := postgres.NewAudio(f.pool)
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	prepared := enqueuePreparationJob(t, jobs, f.scope, "prepare-success", 1)
	claimed, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	published := map[string]any{
		"kind": "create", "volume": 1, "chapter": 1,
		"chapterId": postgres.UUIDString(chapterID), "scriptRevision": 1,
		"sourceRevision": record.Revision, "quality": "high",
		"segments": []map[string]any{{"id": "s0001", "text": "Trời đã tối."}},
	}
	done, err := jobs.CompletePreparationWith(ctx, f.scope, prepared.ID, claimed.FencingToken,
		func(ctx context.Context, tx pgx.Tx, job postgres.Job) (any, int32, error) {
			if job.ID != prepared.ID || job.FencingToken != claimed.FencingToken {
				t.Fatalf("publish saw job %v fence %d, want %v fence %d", job.ID, job.FencingToken, prepared.ID, claimed.FencingToken)
			}
			if _, err := repo.SaveScriptBundleTx(ctx, tx, f.scope, postgres.Script{
				ChapterID: chapterID, RevisionNo: 1, SourceRevision: int32(record.Revision),
				SourceSHA256: record.ContentSHA256, Status: "locked", Payload: []byte(`{"segments":1}`),
			}, []postgres.Segment{{
				SegmentID: "s0001", Ordinal: 0, Speaker: "nguoi-ke", Text: "Trời đã tối.", Payload: []byte(`{}`),
			}}, nil, 0); err != nil {
				return nil, 0, err
			}
			return published, 1, nil
		})
	if err != nil {
		t.Fatalf("complete preparation: %v", err)
	}
	if done.ID != prepared.ID {
		t.Fatalf("prepared row id changed: %v -> %v", prepared.ID, done.ID)
	}
	if done.State != postgres.JobQueued || done.WorkerID.Valid || done.LeaseUntil != nil {
		t.Fatalf("prepared row = state %s worker %v lease %v, want queued/unleased", done.State, done.WorkerID.Valid, done.LeaseUntil)
	}
	if done.FencingToken != claimed.FencingToken+1 {
		t.Fatalf("fence = %d, want %d", done.FencingToken, claimed.FencingToken+1)
	}
	if done.ProgressTotal == nil || *done.ProgressTotal != 1 {
		t.Fatalf("progress_total = %v, want 1", done.ProgressTotal)
	}
	if done.IdempotencyKey != prepared.IdempotencyKey || !done.CreatedAt.Equal(prepared.CreatedAt) {
		t.Fatalf("attempt metadata changed: key=%s created=%v", done.IdempotencyKey, done.CreatedAt)
	}
	stored := jobInputMap(t, done)
	if _, ok := stored["needsScript"]; ok {
		t.Fatalf("needsScript survived preparation: %s", done.Input)
	}
	if stored["chapter"] != float64(1) {
		t.Fatalf("prepared input = %v", stored)
	}
	latest, err := repo.LatestScript(ctx, f.scope.ProjectID, chapterID)
	if err != nil {
		t.Fatalf("latest script: %v", err)
	}
	if latest.Status != "locked" || latest.RevisionNo != 1 {
		t.Fatalf("script = status %s revision %d", latest.Status, latest.RevisionNo)
	}
	segments, err := repo.Segments(ctx, f.scope.ProjectID, chapterID, 1)
	if err != nil {
		t.Fatalf("segments: %v", err)
	}
	if len(segments) != 1 || segments[0].SegmentID != "s0001" {
		t.Fatalf("segments = %+v", segments)
	}

	worker := registerTestWorker(t, jobs, "prep-done-mac")
	claimedJob, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatalf("worker claim after preparation: %v", err)
	}
	if claimedJob.ID != prepared.ID || claimedJob.State != postgres.JobRunning {
		t.Fatalf("worker claimed %v state %s, want the prepared row running", claimedJob.ID, claimedJob.State)
	}
	workerInput := jobInputMap(t, claimedJob)
	if _, ok := workerInput["needsScript"]; ok {
		t.Fatalf("worker payload still carries needsScript: %s", claimedJob.Input)
	}
	if segs, ok := workerInput["segments"].([]any); !ok || len(segs) != 1 {
		t.Fatalf("worker payload segments = %v", workerInput["segments"])
	}
	if _, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("prepared row is still claimable as preparation: %v", err)
	}
}

// TestPreparationZeroWorkCompletes: when the refreshed script needs no new
// synthesis the same row is terminal, not a zero-segment render job the worker
// would fail forever.
func TestPreparationZeroWorkCompletes(t *testing.T) {
	f := provisionNarrative(t, "prep-zero-proj", "prep-zero-story")
	repo := postgres.NewAudio(f.pool)
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	prepared := enqueuePreparationJob(t, jobs, f.scope, "prepare-zero", 1)
	claimed, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	done, err := jobs.CompletePreparationWith(ctx, f.scope, prepared.ID, claimed.FencingToken,
		func(ctx context.Context, tx pgx.Tx, job postgres.Job) (any, int32, error) {
			if _, err := repo.SaveScriptBundleTx(ctx, tx, f.scope, postgres.Script{
				ChapterID: chapterID, RevisionNo: 1, SourceRevision: int32(record.Revision),
				SourceSHA256: record.ContentSHA256, Status: "locked", Payload: []byte(`{"segments":0}`),
			}, nil, nil, 0); err != nil {
				return nil, 0, err
			}
			return map[string]any{"kind": "update", "volume": 1, "chapter": 1, "segments": []any{}}, 0, nil
		})
	if err != nil {
		t.Fatalf("complete zero-work preparation: %v", err)
	}
	if done.State != postgres.JobCompleted || done.FinishedAt == nil {
		t.Fatalf("zero-work row = state %s finished %v, want completed", done.State, done.FinishedAt)
	}
	if done.FencingToken != claimed.FencingToken+1 || done.ProgressTotal == nil || *done.ProgressTotal != 0 {
		t.Fatalf("zero-work row fence=%d total=%v", done.FencingToken, done.ProgressTotal)
	}
	if done.WorkerID.Valid || done.LeaseUntil != nil {
		t.Fatalf("zero-work row kept a worker/lease: %v %v", done.WorkerID.Valid, done.LeaseUntil)
	}
	if _, ok := jobInputMap(t, done)["needsScript"]; ok {
		t.Fatalf("needsScript survived zero-work completion: %s", done.Input)
	}
	if _, err := repo.LatestScript(ctx, f.scope.ProjectID, chapterID); err != nil {
		t.Fatalf("zero-work script was not saved: %v", err)
	}

	worker := registerTestWorker(t, jobs, "prep-zero-mac")
	if _, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{}); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("worker claimed a zero-segment preparation: %v", err)
	}
	if _, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("completed preparation still claimable: %v", err)
	}
}

// TestPreparationLeaseExpiryFailureAndRetry: an exhausted lease loses the fence,
// a failure stays retryable through the same preparation path, and every stale
// fence is rejected.
func TestPreparationLeaseExpiryFailureAndRetry(t *testing.T) {
	f := provisionNarrative(t, "prep-lease-proj", "prep-lease-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	worker := registerTestWorker(t, jobs, "prep-lease-mac")

	prepared := enqueuePreparationJob(t, jobs, f.scope, "prepare", 1)
	first, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `UPDATE jobs
		SET lease_until = now() - interval '1 second',
		    updated_at = clock_timestamp(), version = version + 1
		WHERE project_id = $1 AND id = $2`, f.scope.ProjectID, prepared.ID); err != nil {
		t.Fatalf("expire preparation lease: %v", err)
	}

	publishRan := false
	if _, err := jobs.CompletePreparationWith(ctx, f.scope, prepared.ID, first.FencingToken,
		func(ctx context.Context, tx pgx.Tx, job postgres.Job) (any, int32, error) {
			publishRan = true
			return map[string]any{"ok": true}, 1, nil
		}); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("complete with exhausted lease = %v, want ErrJobLeaseLost", err)
	}
	if publishRan {
		t.Fatal("publish callback ran for an exhausted lease")
	}
	if _, err := jobs.ReleasePreparation(ctx, f.scope, prepared.ID, first.FencingToken); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("release with exhausted lease = %v, want ErrJobLeaseLost", err)
	}

	second, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("reclaim after expiry: %v", err)
	}
	if second.FencingToken != first.FencingToken+1 {
		t.Fatalf("reclaim fence = %d, want %d", second.FencingToken, first.FencingToken+1)
	}

	failed, err := jobs.FailPreparation(ctx, f.scope, prepared.ID, second.FencingToken, map[string]any{"error": "model"})
	if err != nil {
		t.Fatalf("fail preparation: %v", err)
	}
	if failed.State != postgres.JobFailed || failed.LeaseUntil != nil || failed.WorkerID.Valid {
		t.Fatalf("failed row = state %s lease %v worker %v", failed.State, failed.LeaseUntil, failed.WorkerID.Valid)
	}
	var failure map[string]any
	if err := json.Unmarshal(failed.Error, &failure); err != nil || failure["error"] != "model" {
		t.Fatalf("job error = %s (%v)", failed.Error, err)
	}
	if jobInputMap(t, failed)["needsScript"] != true {
		t.Fatalf("failure dropped needsScript: %s", failed.Input)
	}
	if _, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{}); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("worker claimed a failed preparation: %v", err)
	}
	if _, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("failed preparation is claimable: %v", err)
	}

	retried, err := jobs.Retry(ctx, jobKey(f.scope, prepared.ID), nil)
	if err != nil {
		t.Fatalf("retry preparation: %v", err)
	}
	if retried.State != postgres.JobQueued {
		t.Fatalf("retried state = %s, want queued", retried.State)
	}
	third, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim after retry: %v", err)
	}
	if third.FencingToken != retried.FencingToken+1 {
		t.Fatalf("claim after retry fence = %d, want %d", third.FencingToken, retried.FencingToken+1)
	}
	if _, err := jobs.FailPreparation(ctx, f.scope, prepared.ID, retried.FencingToken, map[string]any{"error": "stale"}); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("fail with pre-claim fence = %v, want ErrJobLeaseLost", err)
	}
}

// TestListPreparationTargetsActiveStories: restart recovery sees exactly the
// active stories still waiting for a bản đọc — once per story, never a draft or
// a deleted story, never a story whose jobs need no preparation.
func TestListPreparationTargetsActiveStories(t *testing.T) {
	f := provisionNarrative(t, "prep-target-proj", "prep-target-a")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	q := gen.New(f.pool.Pool())

	if _, err := f.nar.CreateStory(ctx, "prep-target-proj", nil, postgres.StoryInfo{
		Slug: "prep-target-b", Title: "B", Status: "writing",
	}); err != nil {
		t.Fatalf("create target story B: %v", err)
	}
	scopeB, err := f.nar.Resolve(ctx, "prep-target-proj", "prep-target-b", nil, false)
	if err != nil {
		t.Fatalf("resolve story B: %v", err)
	}
	if _, err := f.nar.CreateStory(ctx, "prep-target-proj", nil, postgres.StoryInfo{
		Slug: "prep-target-render", Title: "R", Status: "writing",
	}); err != nil {
		t.Fatalf("create render-only story: %v", err)
	}
	scopeRender, err := f.nar.Resolve(ctx, "prep-target-proj", "prep-target-render", nil, false)
	if err != nil {
		t.Fatalf("resolve render-only story: %v", err)
	}
	if _, err := f.nar.CreateStory(ctx, "prep-target-proj", nil, postgres.StoryInfo{
		Slug: "prep-target-gone", Title: "G", Status: "writing",
	}); err != nil {
		t.Fatalf("create deleted story: %v", err)
	}
	scopeGone, err := f.nar.Resolve(ctx, "prep-target-proj", "prep-target-gone", nil, false)
	if err != nil {
		t.Fatalf("resolve deleted story: %v", err)
	}
	draftID := deterministicUUID("story", "antiq-draft-hidden")
	if _, err := q.InsertStory(ctx, gen.InsertStoryParams{
		ProjectID: f.scope.ProjectID, ID: draftID, Slug: "draft-hidden", Title: "Nháp",
		Status: "draft", Tags: []string{},
	}); err != nil {
		t.Fatalf("insert draft story: %v", err)
	}

	prepA1 := enqueuePreparationJob(t, jobs, f.scope, "a-ch1", 1)
	enqueuePreparationJob(t, jobs, f.scope, "a-ch2", 2)
	// Two preparations for the same story: the oldest (chapter 1) must be the
	// one a claim returns. Pin the order instead of relying on wall-clock luck.
	if _, err := f.pool.Pool().Exec(ctx, `UPDATE jobs
		SET created_at = now() - interval '1 minute',
		    updated_at = clock_timestamp(), version = version + 1
		WHERE project_id = $1 AND id = $2`, f.scope.ProjectID, prepA1.ID); err != nil {
		t.Fatalf("age the first preparation: %v", err)
	}
	enqueuePreparationJob(t, jobs, scopeB, "b-ch1", 1)
	enqueuePreparationJob(t, jobs, scopeGone, "gone-ch1", 1)
	enqueuePreparationJob(t, jobs, postgres.ScopeRef{ProjectID: f.scope.ProjectID, StoryID: draftID}, "draft-ch1", 1)
	if _, err := jobs.Enqueue(ctx, scopeRender, postgres.JobTTS, "render-only", map[string]any{
		"kind": "create", "volume": 1, "chapter": 1,
	}, nil, pgtype.UUID{}); err != nil {
		t.Fatalf("enqueue render-only job: %v", err)
	}
	if _, err := q.SoftDeleteStory(ctx, gen.SoftDeleteStoryParams{
		ProjectID: scopeGone.ProjectID, ID: scopeGone.StoryID,
	}); err != nil {
		t.Fatalf("soft delete story: %v", err)
	}

	targets, err := jobs.ListPreparationTargets(ctx)
	if err != nil {
		t.Fatalf("list preparation targets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %+v, want story A and story B only", targets)
	}
	byStory := map[pgtype.UUID]postgres.PreparationTarget{}
	for _, target := range targets {
		byStory[target.Scope.StoryID] = target
	}
	for name, want := range map[string]struct {
		scope  postgres.ScopeRef
		slug   string
		exists bool
	}{
		"A":      {scope: f.scope, slug: "prep-target-a", exists: true},
		"B":      {scope: scopeB, slug: "prep-target-b", exists: true},
		"render": {scope: scopeRender, slug: "prep-target-render"},
		"gone":   {scope: scopeGone, slug: "prep-target-gone"},
		"draft":  {scope: postgres.ScopeRef{StoryID: draftID}, slug: "draft-hidden"},
	} {
		target, ok := byStory[want.scope.StoryID]
		if ok != want.exists {
			t.Fatalf("story %s target present = %v, want %v (targets %+v)", name, ok, want.exists, targets)
		}
		if !want.exists {
			continue
		}
		if target.ProjectSlug != "prep-target-proj" || target.StorySlug != want.slug || target.Scope.ProjectID != want.scope.ProjectID {
			t.Fatalf("story %s target = %+v", name, target)
		}
	}

	// A story whose only preparation was completed stops being a target.
	claimed, err := jobs.ClaimPreparation(ctx, scopeB, time.Minute)
	if err != nil {
		t.Fatalf("claim story B: %v", err)
	}
	if _, err := jobs.CompletePreparationWith(ctx, scopeB, claimed.ID, claimed.FencingToken,
		func(ctx context.Context, tx pgx.Tx, job postgres.Job) (any, int32, error) {
			return map[string]any{"kind": "create", "volume": 1, "chapter": 1, "segments": []any{}}, 1, nil
		}); err != nil {
		t.Fatalf("complete story B preparation: %v", err)
	}
	targets, err = jobs.ListPreparationTargets(ctx)
	if err != nil {
		t.Fatalf("list preparation targets after completion: %v", err)
	}
	if len(targets) != 1 || targets[0].Scope.StoryID != f.scope.StoryID {
		t.Fatalf("targets after completion = %+v, want only story A", targets)
	}
	// A claim still sees story A's oldest preparation row.
	first, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim story A: %v", err)
	}
	if first.ID != prepA1.ID {
		t.Fatalf("claimed %v, want the oldest preparation %v", first.ID, prepA1.ID)
	}
}
