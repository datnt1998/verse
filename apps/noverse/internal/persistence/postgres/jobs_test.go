package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/noverse/internal/persistence/postgres"
	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
)

// registerTestWorker creates an active worker credential (hash only; the
// plaintext token is the HTTP layer's business).
func registerTestWorker(t *testing.T, jobs *postgres.Jobs, name string) postgres.Worker {
	t.Helper()
	sum := sha256.Sum256([]byte("token-" + name))
	w, err := jobs.RegisterWorker(context.Background(), name, sum[:], []string{"tts"})
	if err != nil {
		t.Fatalf("register worker %s: %v", name, err)
	}
	return w
}

func enqueueTestJob(t *testing.T, jobs *postgres.Jobs, scope postgres.ScopeRef, key string) postgres.Job {
	t.Helper()
	job, err := jobs.Enqueue(context.Background(), scope, postgres.JobTTS, key, map[string]any{
		"chapter": 1, "kind": "create", "segments": 3,
	}, nil, pgtype.UUID{})
	if err != nil {
		t.Fatalf("enqueue %s: %v", key, err)
	}
	return job
}

// TestJobsEnqueueIdempotent: re-locking a chapter must resolve the existing
// job instead of queueing a second render of the same work.
func TestJobsEnqueueIdempotent(t *testing.T) {
	f := provisionNarrative(t, "jobs-idem-proj", "jobs-idem-story")
	jobs := postgres.NewJobs(f.pool)

	first := enqueueTestJob(t, jobs, f.scope, "chapter-1-create")
	second := enqueueTestJob(t, jobs, f.scope, "chapter-1-create")
	if first.ID != second.ID {
		t.Fatalf("idempotent enqueue produced two jobs: %v vs %v", first.ID, second.ID)
	}
	if second.State != postgres.JobQueued {
		t.Fatalf("state = %q, want queued", second.State)
	}
	list, err := jobs.List(context.Background(), f.scope)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("job count = %d, want 1", len(list))
	}

	// A different kind is a different queue entry.
	exportJob, err := jobs.Enqueue(context.Background(), f.scope, postgres.JobAudioExport, "chapter-1-create", map[string]any{}, nil, pgtype.UUID{})
	if err != nil {
		t.Fatalf("enqueue export: %v", err)
	}
	if exportJob.ID == first.ID {
		t.Fatal("export job reused the TTS job id")
	}
}

// TestJobsClaimSkipLocked: two workers claiming concurrently never get the
// same job, and a third claim reports the queue empty.
func TestJobsClaimSkipLocked(t *testing.T) {
	f := provisionNarrative(t, "jobs-claim-proj", "jobs-claim-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()

	w1 := registerTestWorker(t, jobs, "mac-1")
	w2 := registerTestWorker(t, jobs, "mac-2")

	a := enqueueTestJob(t, jobs, f.scope, "a")
	b := enqueueTestJob(t, jobs, f.scope, "b")

	type result struct {
		job postgres.Job
		err error
	}
	results := make(chan result, 2)
	claim := func(worker postgres.Worker) {
		job, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
		results <- result{job: job, err: err}
	}
	go claim(w1)
	go claim(w2)

	claimed := map[pgtype.UUID]bool{}
	for i := range 2 {
		r := <-results
		if r.err != nil {
			t.Fatalf("concurrent claim %d: %v", i, r.err)
		}
		if claimed[r.job.ID] {
			t.Fatalf("two workers claimed the same job %v", r.job.ID)
		}
		claimed[r.job.ID] = true
	}
	if !claimed[a.ID] || !claimed[b.ID] {
		t.Fatalf("claims = %v, want both %v and %v", claimed, a.ID, b.ID)
	}
	if _, err := jobs.Claim(ctx, postgres.JobTTS, w1.ID, postgres.DefaultLease, pgtype.UUID{}); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("third claim = %v, want ErrNoClaimableJob", err)
	}
}

// TestJobsLeaseExpireReclaim is the crash/partition gate: a worker that dies
// mid-job loses the lease, a second worker reclaims the same job with a higher
// fence, and the dead worker's heartbeat/ACK can no longer publish.
func TestJobsLeaseExpireReclaim(t *testing.T) {
	f := provisionNarrative(t, "jobs-lease-proj", "jobs-lease-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	dead := registerTestWorker(t, jobs, "mac-dead")
	live := registerTestWorker(t, jobs, "mac-live")

	job := enqueueTestJob(t, jobs, f.scope, "crash")
	claimed, err := jobs.Claim(ctx, postgres.JobTTS, dead.ID, time.Second, pgtype.UUID{})
	if err != nil {
		t.Fatalf("claim by dead worker: %v", err)
	}
	deadLease := postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: claimed.ProjectID, ID: claimed.ID}, WorkerID: dead.ID, Fence: claimed.FencingToken}

	// A live worker cannot steal a live lease.
	if _, err := jobs.Claim(ctx, postgres.JobTTS, live.ID, postgres.DefaultLease, pgtype.UUID{}); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("claim while leased = %v, want ErrNoClaimableJob", err)
	}

	// Simulate the dead worker's lease running out (server-side expiry).
	expireLease(t, f.pool, claimed.ProjectID, claimed.ID)

	reclaimed, err := jobs.Claim(ctx, postgres.JobTTS, live.ID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatalf("reclaim after expiry: %v", err)
	}
	if reclaimed.ID != job.ID {
		t.Fatalf("reclaimed job %v, want the original %v", reclaimed.ID, job.ID)
	}
	if reclaimed.FencingToken != claimed.FencingToken+1 {
		t.Fatalf("fence = %d, want %d", reclaimed.FencingToken, claimed.FencingToken+1)
	}
	if reclaimed.ProgressDone != claimed.ProgressDone {
		t.Fatalf("reclaim reset progress: %d -> %d", claimed.ProgressDone, reclaimed.ProgressDone)
	}

	// The stale worker is fenced out of every publish path.
	if _, err := jobs.Heartbeat(ctx, deadLease, postgres.DefaultLease); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("stale heartbeat = %v, want ErrJobLeaseLost", err)
	}
	if _, err := jobs.Complete(ctx, deadLease, map[string]any{"ok": true}, 3, nil); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("stale complete = %v, want ErrJobLeaseLost", err)
	}
	if _, err := jobs.Fail(ctx, deadLease, map[string]any{"error": "boom"}, nil); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("stale fail = %v, want ErrJobLeaseLost", err)
	}
	if _, err := jobs.Release(ctx, deadLease); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("stale release = %v, want ErrJobLeaseLost", err)
	}

	// The new owner still holds the job and can publish.
	liveLease := postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: reclaimed.ProjectID, ID: reclaimed.ID}, WorkerID: live.ID, Fence: reclaimed.FencingToken}
	done, err := jobs.Complete(ctx, liveLease, map[string]any{"segments": 3}, 3, nil)
	if err != nil {
		t.Fatalf("new owner complete: %v", err)
	}
	if done.State != postgres.JobCompleted || done.FinishedAt == nil {
		t.Fatalf("completed job = state %q finished %v", done.State, done.FinishedAt)
	}
}

// expireLease forces a running job's lease into the past while keeping the row
// consistent with the version guard.
func expireLease(t *testing.T, p *postgres.Pool, projectID, jobID pgtype.UUID) {
	t.Helper()
	if _, err := p.Pool().Exec(context.Background(), `
		UPDATE jobs
		SET lease_until = now() - interval '1 minute',
		    updated_at = clock_timestamp(), version = version + 1
		WHERE project_id = $1 AND id = $2`, projectID, jobID); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
}

// TestJobsDuplicateACK: replaying a completion is rejected (no second effect)
// and leaves exactly one terminal row/event set.
func TestJobsDuplicateACK(t *testing.T) {
	f := provisionNarrative(t, "jobs-ack-proj", "jobs-ack-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	w := registerTestWorker(t, jobs, "mac-ack")

	job := enqueueTestJob(t, jobs, f.scope, "ack")
	claimed, err := jobs.Claim(ctx, postgres.JobTTS, w.ID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	lease := postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: claimed.ProjectID, ID: claimed.ID}, WorkerID: w.ID, Fence: claimed.FencingToken}

	if _, err := jobs.AppendEvent(ctx, lease.JobKey, "claimed", map[string]any{"worker": "mac-ack"}); err != nil {
		t.Fatalf("append claimed event: %v", err)
	}
	if _, err := jobs.Complete(ctx, lease, map[string]any{"file": "chapter-1.mp3"}, 3, nil); err != nil {
		t.Fatalf("first complete: %v", err)
	}
	if _, err := jobs.Complete(ctx, lease, map[string]any{"file": "chapter-1.mp3"}, 3, nil); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("duplicate ACK = %v, want ErrJobLeaseLost", err)
	}

	stored, err := jobs.Get(ctx, lease.JobKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.ID != job.ID || stored.State != postgres.JobCompleted {
		t.Fatalf("stored job = %v/%q", stored.ID, stored.State)
	}
	events, err := jobs.Events(ctx, lease.JobKey, 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1 (duplicate ACK must not append)", len(events))
	}
}

// TestJobsCancelDuringUpload: cancelling a running job fences the worker out;
// its late upload cannot become the published result.
func TestJobsCancelDuringUpload(t *testing.T) {
	f := provisionNarrative(t, "jobs-cancel-proj", "jobs-cancel-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	w := registerTestWorker(t, jobs, "mac-cancel")

	enqueueTestJob(t, jobs, f.scope, "cancel")
	claimed, err := jobs.Claim(ctx, postgres.JobTTS, w.ID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	lease := postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: claimed.ProjectID, ID: claimed.ID}, WorkerID: w.ID, Fence: claimed.FencingToken}

	cancelled, err := jobs.Cancel(ctx, lease.JobKey)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.State != postgres.JobCancelled || cancelled.CancelledAt == nil || cancelled.FinishedAt == nil {
		t.Fatalf("cancelled job = %+v", cancelled)
	}
	if cancelled.FencingToken <= claimed.FencingToken {
		t.Fatalf("cancel did not move the fence: %d -> %d", claimed.FencingToken, cancelled.FencingToken)
	}

	for name, publish := range map[string]func() error{
		"complete": func() error {
			_, err := jobs.Complete(ctx, lease, map[string]any{"file": "late.mp3"}, 3, nil)
			return err
		},
		"heartbeat": func() error {
			_, err := jobs.Heartbeat(ctx, lease, postgres.DefaultLease)
			return err
		},
	} {
		if err := publish(); !errors.Is(err, postgres.ErrJobLeaseLost) {
			t.Fatalf("%s after cancel = %v, want ErrJobLeaseLost", name, err)
		}
	}

	// Nothing was claimed again: a cancelled job is terminal.
	if _, err := jobs.Claim(ctx, postgres.JobTTS, w.ID, postgres.DefaultLease, pgtype.UUID{}); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("claim after cancel = %v, want ErrNoClaimableJob", err)
	}
}

// TestJobsRestartPersistence: jobs survive a process restart (a fresh pool
// sees the same rows) and an interrupted running job becomes reclaimable
// without being reported as done.
func TestJobsRestartPersistence(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	poolA := openPool(t, dsn)
	f := provisionStory(t, poolA, "jobs-restart-proj", "jobs-restart-story")
	ctx := context.Background()

	jobsA := postgres.NewJobs(poolA)
	worker := registerTestWorker(t, jobsA, "mac-restart")
	enqueueTestJob(t, jobsA, f.scope, "survive")
	claimed, err := jobsA.Claim(ctx, postgres.JobTTS, worker.ID, time.Second, pgtype.UUID{})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	// The worker is killed: no release, no fail. Its lease lapses.

	expireLease(t, poolA, claimed.ProjectID, claimed.ID)

	// "Restart": a brand-new pool and repository (the web process comes back).
	poolB := openPool(t, dsn)
	jobsB := postgres.NewJobs(poolB)
	list, err := jobsB.List(ctx, f.scope)
	if err != nil {
		t.Fatalf("list after restart: %v", err)
	}
	if len(list) != 1 || list[0].ID != claimed.ID {
		t.Fatalf("jobs after restart = %v, want the persisted job %v", list, claimed.ID)
	}
	if list[0].State != postgres.JobRunning {
		t.Fatalf("interrupted job state = %q, want running until normalised", list[0].State)
	}

	n, err := jobsB.RequeueExpired(ctx)
	if err != nil {
		t.Fatalf("requeue expired: %v", err)
	}
	if n != 1 {
		t.Fatalf("requeued %d jobs, want 1", n)
	}
	recovered, err := jobsB.Get(ctx, postgres.JobKey{ProjectID: claimed.ProjectID, ID: claimed.ID})
	if err != nil {
		t.Fatalf("get recovered: %v", err)
	}
	if recovered.State != postgres.JobQueued {
		t.Fatalf("recovered state = %q, want queued (never completed)", recovered.State)
	}
	if recovered.Result != nil {
		t.Fatalf("recovered job has a result: %s", recovered.Result)
	}

	// A live worker is not disturbed by boot-time normalisation.
	if _, err := jobsB.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{}); err != nil {
		t.Fatalf("claim recovered: %v", err)
	}
	if n, err := jobsB.RequeueExpired(ctx); err != nil || n != 0 {
		t.Fatalf("requeue with a live lease = %d, %v; want 0", n, err)
	}
}

// TestJobsEventsAppendOrdered: concurrent appenders get a dense sequence with
// no duplicate keys (the job row lock serialises them).
func TestJobsEventsAppendOrdered(t *testing.T) {
	f := provisionNarrative(t, "jobs-events-proj", "jobs-events-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()

	job := enqueueTestJob(t, jobs, f.scope, "events")
	key := postgres.JobKey{ProjectID: job.ProjectID, ID: job.ID}

	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := jobs.AppendEvent(ctx, key, "progress", map[string]any{"done": i})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("append event: %v", err)
		}
	}

	events, err := jobs.Events(ctx, key, 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) != n {
		t.Fatalf("event count = %d, want %d", len(events), n)
	}
	for i, ev := range events {
		if ev.Seq != int64(i+1) {
			t.Fatalf("event %d seq = %d, want %d", i, ev.Seq, i+1)
		}
	}
	if after, err := jobs.Events(ctx, key, 4); err != nil || len(after) != n-4 {
		t.Fatalf("events after seq 4 = %d, %v; want %d", len(after), err, n-4)
	}
}

// TestJobsWorkerCredentials: tokens authenticate by hash, revocation stops
// further use, and the plaintext token is never stored.
func TestJobsWorkerCredentials(t *testing.T) {
	f := provisionNarrative(t, "jobs-worker-proj", "jobs-worker-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()

	worker := registerTestWorker(t, jobs, "mac-auth")
	sum := sha256.Sum256([]byte("token-mac-auth"))
	got, err := jobs.AuthenticateWorker(ctx, sum[:])
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.ID != worker.ID || len(got.Capabilities) != 1 || got.Capabilities[0] != "tts" {
		t.Fatalf("authenticated worker = %+v", got)
	}
	wrong := sha256.Sum256([]byte("wrong"))
	if _, err := jobs.AuthenticateWorker(ctx, wrong[:]); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("unknown token = %v, want ErrJobNotFound", err)
	}

	if err := jobs.RevokeWorker(ctx, worker.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := jobs.AuthenticateWorker(ctx, sum[:]); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("revoked token = %v, want ErrJobLeaseLost", err)
	}
	if err := jobs.RevokeWorker(ctx, worker.ID); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("second revoke = %v, want ErrJobNotFound", err)
	}
}

// provisionStory anchors a project+story on an already-migrated pool.
func provisionStory(t *testing.T, pool *postgres.Pool, projectSlug, storySlug string) pgFixture {
	t.Helper()
	ctx := context.Background()
	q := gen.New(pool.Pool())
	if _, err := q.EnsureProject(ctx, gen.EnsureProjectParams{
		ID:   deterministicUUID("project", projectSlug),
		Slug: projectSlug, Name: projectSlug,
	}); err != nil {
		t.Fatalf("anchor project: %v", err)
	}
	nar := postgres.NewNarrative(pool)
	if _, err := nar.CreateStory(ctx, projectSlug, nil, postgres.StoryInfo{
		Slug: storySlug, Title: "Truyện thử", Status: "writing",
	}); err != nil {
		t.Fatalf("create story: %v", err)
	}
	scope, err := nar.Resolve(ctx, projectSlug, storySlug, nil, false)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return pgFixture{nar: nar, pool: pool, scope: scope}
}

// TestJobsGoiAKSemantics: the Gói AK audio queue contract (kind
// create|update|redo, redo flag, position ordering and updateEtaSec) survives
// a durable round trip unchanged. The file queue keeps these fields in
// queue.json; here they live in the frozen job input and the row order.
func TestJobsGoiAKSemantics(t *testing.T) {
	f := provisionNarrative(t, "jobs-ak-proj", "jobs-ak-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	worker := registerTestWorker(t, jobs, "mac-ak")

	inputs := []map[string]any{
		{"kind": "create", "chapter": 1, "redo": false, "segments": 3},
		{"kind": "update", "chapter": 2, "redo": false, "segments": 4, "updateEtaSec": 150},
		{"kind": "redo", "chapter": 3, "redo": true, "segments": 2, "updateEtaSec": 40},
	}
	ids := make([]pgtype.UUID, 0, len(inputs))
	for i, input := range inputs {
		job, err := jobs.Enqueue(ctx, f.scope, postgres.JobTTS, "ak-"+string(rune('a'+i)), input, nil, pgtype.UUID{})
		if err != nil {
			t.Fatalf("enqueue %v: %v", input["kind"], err)
		}
		ids = append(ids, job.ID)
	}

	// Position follows enqueue order (the UI numbers queued jobs from 1).
	list, err := jobs.List(ctx, f.scope)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != len(inputs) {
		t.Fatalf("jobs = %d, want %d", len(list), len(inputs))
	}
	for i, job := range list {
		if job.ID != ids[i] {
			t.Fatalf("position %d = %v, want %v", i+1, job.ID, ids[i])
		}
	}

	// Claim order mirrors position and each snapshot keeps its AK fields.
	for i, want := range inputs {
		claimed, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
		if err != nil {
			t.Fatalf("claim %d: %v", i, err)
		}
		var got map[string]any
		if err := json.Unmarshal(claimed.Input, &got); err != nil {
			t.Fatalf("input not JSON: %v", err)
		}
		for k, v := range want {
			if fmt.Sprint(got[k]) != fmt.Sprint(v) {
				t.Fatalf("job %d input[%s] = %v, want %v", i, k, got[k], v)
			}
		}
		lease := postgres.WorkerLease{
			JobKey:   postgres.JobKey{ProjectID: claimed.ProjectID, ID: claimed.ID},
			WorkerID: worker.ID, Fence: claimed.FencingToken,
		}
		done, err := jobs.Complete(ctx, lease, map[string]any{"kind": want["kind"]}, 1, nil)
		if err != nil {
			t.Fatalf("complete %d: %v", i, err)
		}
		if done.State != postgres.JobCompleted {
			t.Fatalf("job %d state = %q", i, done.State)
		}
	}
	if _, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{}); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("claim after all done = %v, want ErrNoClaimableJob", err)
	}
}

// TestJobsFenceSameWorkerReclaim is the split-brain gate the fence exists for:
// a worker restarts, reclaims its own interrupted job (same worker id, higher
// fence), while a late goroutine from the first attempt still holds the old
// lease. The old attempt must not be able to publish.
func TestJobsFenceSameWorkerReclaim(t *testing.T) {
	f := provisionNarrative(t, "jobs-same-proj", "jobs-same-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	worker := registerTestWorker(t, jobs, "mac-same")

	enqueueTestJob(t, jobs, f.scope, "same")
	first, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, time.Second, pgtype.UUID{})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	oldLease := postgres.WorkerLease{
		JobKey:   postgres.JobKey{ProjectID: first.ProjectID, ID: first.ID},
		WorkerID: worker.ID, Fence: first.FencingToken,
	}
	expireLease(t, f.pool, first.ProjectID, first.ID)

	second, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatalf("reclaim by same worker: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("reclaimed %v, want %v", second.ID, first.ID)
	}
	if second.FencingToken <= first.FencingToken {
		t.Fatalf("fence did not advance: %d -> %d", first.FencingToken, second.FencingToken)
	}

	if _, err := jobs.Complete(ctx, oldLease, map[string]any{"file": "late.mp3"}, 1, nil); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("old attempt complete = %v, want ErrJobLeaseLost", err)
	}
	if _, err := jobs.Heartbeat(ctx, oldLease, postgres.DefaultLease); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("old attempt heartbeat = %v, want ErrJobLeaseLost", err)
	}

	newLease := postgres.WorkerLease{
		JobKey:   postgres.JobKey{ProjectID: second.ProjectID, ID: second.ID},
		WorkerID: worker.ID, Fence: second.FencingToken,
	}
	done, err := jobs.Complete(ctx, newLease, map[string]any{"file": "current.mp3"}, 1, nil)
	if err != nil {
		t.Fatalf("new attempt complete: %v", err)
	}
	if done.State != postgres.JobCompleted {
		t.Fatalf("state = %q, want completed", done.State)
	}
}
