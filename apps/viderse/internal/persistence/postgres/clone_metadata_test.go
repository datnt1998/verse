package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

func TestCloneMetadataFailedRetryPersistsAndRequeues(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name) VALUES('11111111-1111-4111-8111-111111111111','Metadata')`); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	job := cloneTestJob(t, store, "metadata-recovery")
	if _, err := pool.Exec(ctx, `UPDATE clone_steps SET status='failed', error='{"code":"internal_error","message":"failed"}' WHERE job_id=$1 AND step='metadata'`, job.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetJobStatus(ctx, job.ID, domain.CloneFailed); err != nil {
		t.Fatal(err)
	}
	before, err := store.Read(ctx, job.ID, domain.CloneMetadata)
	if err != nil {
		t.Fatal(err)
	}
	cp := json.RawMessage(`{"revision":1,"current":{"title":"recovered","description":"text","tags":[]},"history":[]}`)
	if err = store.Commit(ctx, before, jobedit.Change{Checkpoint: cp, InvalidateFrom: domain.CloneMetadata}); err != nil {
		t.Fatal(err)
	}
	after, err := store.Read(ctx, job.ID, domain.CloneMetadata)
	if err != nil {
		t.Fatal(err)
	}
	if after.Step.Status != domain.CloneSucceeded || after.Step.Error != nil || after.Job.Status != domain.ClonePending {
		t.Fatalf("retry not recovered: step=%s error=%+v job=%s", after.Step.Status, after.Step.Error, after.Job.Status)
	}
	var state []byte
	if err = pool.QueryRow(ctx, `SELECT state FROM clone_metadata_states WHERE job_id=$1`, job.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Revision int `json:"revision"`
	}
	if err = json.Unmarshal(state, &got); err != nil || got.Revision != 1 {
		t.Fatalf("state=%s err=%v", state, err)
	}
	editConflict(t, store.Commit(ctx, before, jobedit.Change{Checkpoint: cp, InvalidateFrom: domain.CloneMetadata}))
}

func TestCloneMetadataEditWithActiveBuildAndMetadataWrite(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name) VALUES('11111111-1111-4111-8111-111111111111','Metadata')`); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	job := cloneTestJob(t, store, "metadata-active")
	if _, err := pool.Exec(ctx, `UPDATE clone_jobs SET status='running',active_step='build',lease_owner='worker',lease_expires_at=now()+interval '5 minutes' WHERE id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE clone_steps SET status=CASE step WHEN 'build' THEN 'running' ELSE 'succeeded' END WHERE job_id=$1 AND step IN ('build','metadata')`, job.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.Read(ctx, job.ID, domain.CloneMetadata)
	if err != nil {
		t.Fatal(err)
	}
	// A worker acquiring a later pipeline step changes the job fence without
	// changing metadata; it must not make this metadata edit stale.
	if _, err := pool.Exec(ctx, `UPDATE clone_jobs SET fence=fence+1 WHERE id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	change := jobedit.Change{Checkpoint: json.RawMessage(`{"revision":1,"current":{"title":"edited","description":"text","tags":[]},"history":[]}`)}
	if err = store.Commit(ctx, before, change); err != nil {
		t.Fatalf("metadata edit while build active: %v", err)
	}
	after, err := store.Read(ctx, job.ID, domain.CloneMetadata)
	if err != nil {
		t.Fatal(err)
	}
	if after.Job.Status != domain.CloneRunning || after.Job.ActiveStep == nil || *after.Job.ActiveStep != domain.CloneBuild {
		t.Fatalf("running build lost: %+v", after.Job)
	}
	editConflict(t, store.Commit(ctx, before, change))
	if _, err := pool.Exec(ctx, `UPDATE clone_steps SET status='running' WHERE job_id=$1 AND step='metadata'`, job.ID); err != nil {
		t.Fatal(err)
	}
	writing, err := store.Read(ctx, job.ID, domain.CloneMetadata)
	if err != nil {
		t.Fatal(err)
	}
	editConflict(t, store.Commit(ctx, writing, change))
}
