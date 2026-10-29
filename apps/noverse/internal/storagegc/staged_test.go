package storagegc_test

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/blobkey"
	"github.com/datnt1998/noverse/internal/persistence/postgres"
	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/noverse/internal/storagegc"
)

func TestGCReclaimsStaleStagedBlob(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now()
	key, err := f.keys.AudioSegment(testProject, testStory, hexOf("abandoned"), "wav")
	if err != nil {
		t.Fatal(err)
	}
	f.put(t, key, "abandoned")
	id := "00000000-0000-4000-8000-000000000031"
	f.insertBlob(t, id, key, "staged", f.now.Add(-48*time.Hour))

	before := f.blobRows(t)
	f.run(t, true)
	var state string
	if err := f.pool.Pool().QueryRow(context.Background(), `SELECT state FROM blobs WHERE id=$1`, id).Scan(&state); err != nil || state != "staged" {
		t.Fatalf("dry-run mutated staged row: state=%s err=%v", state, err)
	}
	if len(f.blobRows(t)) != len(before) || !f.exists(key) {
		t.Fatal("dry-run changed registry or object")
	}

	rep := f.run(t, false)
	if len(f.blobRows(t)) != 0 || f.exists(key) || rep.ObjectsDeleted != 1 {
		t.Fatalf("stale staged blob was not reclaimed: %+v", rep)
	}
}

func TestGCStagedAgeAndReferences(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now()
	for i, age := range []time.Duration{time.Hour, 48 * time.Hour, 48 * time.Hour} {
		key, _ := f.keys.AudioSegment(testProject, testStory, hexOf(string(rune('a'+i))), "wav")
		id := []string{
			"00000000-0000-4000-8000-000000000041",
			"00000000-0000-4000-8000-000000000042",
			"00000000-0000-4000-8000-000000000043",
		}[i]
		// No object is required: death before Put must still release the row.
		f.insertBlob(t, id, key, "staged", f.now.Add(-age))
		if i == 2 {
			f.insertCache(t, "staged-reference", "txt", id, f.now.Add(time.Hour))
		}
	}
	// A custom staged age is independent from the ready/orphan grace.
	rep := f.run(t, false, func(o *storagegc.Options) { o.StagedAge = 72 * time.Hour })
	if rep.StagedTombstoned != 0 || len(f.blobRows(t)) != 3 {
		t.Fatalf("custom staged grace ignored: %+v", rep)
	}
	rep = f.run(t, false)
	if rep.StagedTombstoned != 1 || len(f.blobRows(t)) != 2 {
		t.Fatalf("fresh/referenced staged protection: %+v", rep)
	}
}

func TestGCStagedJobLeaseProtection(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		lease       *time.Time
		keep        bool
	}{
		{"leased", "running", timePtr(time.Now().Add(time.Hour)), true},
		{"expired", "running", timePtr(time.Now().Add(-time.Hour)), false},
		{"queued", "queued", nil, true},
		{"paused", "paused", nil, true},
		{"waiting", "waiting", nil, true},
		{"internal-active", "running", nil, true},
		{"failed", "failed", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, blobkey.EnvDev)
			f.now = time.Now()
			ctx := context.Background()
			if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO jobs
				(project_id,story_id,id,kind,state,idempotency_key,input,lease_until)
				VALUES ($1,$2,$3,'audio_export',$4,'upload','{}',$5)`,
				testProject, testStory, testUpload, tc.state, tc.lease); err != nil {
				t.Fatal(err)
			}
			if tc.name == "expired" {
				// An explicitly linked upload is not pinned by an unrelated
				// queued job in the same project (only legacy rows need that).
				if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO jobs
					(project_id,story_id,id,kind,state,idempotency_key,input)
					VALUES ($1,$2,'33333333-3333-4333-8333-333333333333','audio_export','queued','unrelated','{}')`,
					testProject, testStory); err != nil {
					t.Fatal(err)
				}
			}
			key, _ := f.keys.AudioSegment(testProject, testStory, hexOf(tc.name), "wav")
			id := "00000000-0000-4000-8000-000000000051"
			f.put(t, key, tc.name)
			f.insertBlob(t, id, key, "staged", f.now.Add(-48*time.Hour))
			if _, err := f.pool.Pool().Exec(ctx, `UPDATE blobs SET upload_job_id=$2,
				updated_at=clock_timestamp(),version=version+1 WHERE id=$1`, id, testUpload); err != nil {
				t.Fatal(err)
			}
			// Test the locked decision directly, not only its advisory SQL.
			blobID, _ := postgres.ParseUUID(id)
			moved, err := postgres.NewBlobGC(f.pool).TombstoneStaged(ctx, blobID, f.now.Add(-24*time.Hour))
			if err != nil || moved == tc.keep {
				t.Fatalf("locked lease decision: moved=%v keep=%v err=%v", moved, tc.keep, err)
			}
			f.run(t, false)
			if got := len(f.blobRows(t)) == 1 && f.exists(key); got != tc.keep {
				t.Fatalf("staged job %s: kept=%v want=%v", tc.name, got, tc.keep)
			}
		})
	}
}

func TestGCLegacyStagedBlobProtectsActiveProjectJobs(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now()
	ctx := context.Background()
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO jobs
		(project_id,story_id,id,kind,state,idempotency_key,input,lease_until)
		VALUES ($1,$2,$3,'audio_export','running','legacy-upload','{}',now()+interval '1 hour')`,
		testProject, testStory, testUpload); err != nil {
		t.Fatal(err)
	}
	key, _ := f.keys.AudioSegment(testProject, testStory, hexOf("legacy-upload"), "wav")
	id := "00000000-0000-4000-8000-000000000071"
	f.put(t, key, "legacy-upload")
	f.insertBlob(t, id, key, "staged", f.now.Add(-48*time.Hour))
	blobID, _ := postgres.ParseUUID(id)
	moved, err := postgres.NewBlobGC(f.pool).TombstoneStaged(ctx, blobID, f.now.Add(-24*time.Hour))
	if err != nil || moved {
		t.Fatalf("unlinked legacy stage lost active-job protection: moved=%v err=%v", moved, err)
	}
	f.run(t, true)
	f.run(t, false)
	if len(f.blobRows(t)) != 1 || !f.exists(key) {
		t.Fatal("legacy stage collected while project still has an active job")
	}
	if _, err := f.pool.Pool().Exec(ctx, `UPDATE jobs SET state='failed',lease_until=NULL,
		updated_at=clock_timestamp(),version=version+1 WHERE project_id=$1 AND id=$2`,
		testProject, testUpload); err != nil {
		t.Fatal(err)
	}
	f.run(t, false)
	if len(f.blobRows(t)) != 0 || f.exists(key) {
		t.Fatal("legacy stage not collected after project jobs became inactive")
	}
}

func timePtr(t time.Time) *time.Time { return &t }

func TestGCStagedReadyRaceKeepsBlob(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	key, _ := f.keys.AudioSegment(testProject, testStory, hexOf("ready-race"), "wav")
	id := "00000000-0000-4000-8000-000000000061"
	f.put(t, key, "ready-race")
	f.insertBlob(t, id, key, "staged", f.now.Add(-48*time.Hour))
	blobID, _ := postgres.ParseUUID(id)
	projectID, _ := postgres.ParseUUID(testProject)
	repo := postgres.NewBlobGC(f.pool)
	// Capture the advisory candidate before the worker starts publishing it.
	candidates, err := repo.StaleStaged(ctx, f.now.Add(-24*time.Hour))
	if err != nil || len(candidates) != 1 {
		t.Fatalf("staged candidates: %v %v", candidates, err)
	}
	tx, err := f.pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := gen.New(tx).MarkBlobReady(ctx, gen.MarkBlobReadyParams{
		ProjectID: projectID, ID: blobID, Sha256: hexOf("ready-race"), SizeBytes: 10,
	}); err != nil {
		t.Fatal(err)
	}
	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	type result struct {
		moved bool
		err   error
	}
	done := make(chan result, 1)
	go func() {
		moved, err := repo.TombstoneStaged(ctx, blobID, f.now.Add(-24*time.Hour))
		done <- result{moved, err}
	}()
	waitForBlockedGC(t, ctx, f, pid)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil || got.moved {
		t.Fatalf("worker ready transition lost race: moved=%v err=%v", got.moved, got.err)
	}
	// Even another complete GC pass must honor the new ready grace, not the
	// old stage timestamp. Use wall clock after the committed ready update.
	f.now = time.Now()
	f.run(t, false)
	if len(f.blobRows(t)) != 1 || !f.exists(key) {
		t.Fatal("concurrently readied blob or object was collected")
	}
}

func waitForBlockedGC(t *testing.T, ctx context.Context, f *fixture, blocker int) {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var blocked bool
		if err := f.pool.Pool().QueryRow(ctx, `SELECT EXISTS (
			SELECT FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`, blocker).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("GC never waited on the worker's row lock")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-tick.C:
		}
	}
}
