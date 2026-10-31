package postgres_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	metadataedit "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
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

func TestCloneMetadataPatchAndRestoreWhileBuildRuns(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name) VALUES('11111111-1111-4111-8111-111111111111','Metadata')`); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	job := cloneTestJob(t, store, "metadata-patch-active")
	files := &stepstest.FakeMedia{Root: t.TempDir()}
	path := "clone/" + job.ID + "/seed/metadata/metadata.json"
	files.Put(path, []byte(`{"version":1,"title":"before","description":"text","tags":[]}`))
	artifact := domain.CloneArtifact{ID: "seed", Path: path, SHA256: strings.Repeat("a", 64), Step: domain.CloneMetadata, ContentType: "application/json"}
	output, err := json.Marshal(domain.CloneStepOutput{Artifacts: []domain.CloneArtifact{artifact}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clone_artifacts(id,job_id,step,path,sha256,bytes,content_type) VALUES('seed',$1,'metadata',$2,$3,1,'application/json')`, job.ID, path, artifact.SHA256); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE clone_steps SET status='succeeded',output_manifest=$2::jsonb WHERE job_id=$1 AND step='metadata'`, job.ID, output); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE clone_steps SET status='running' WHERE job_id=$1 AND step='build'`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE clone_jobs SET status='running',active_step='build',lease_owner='worker',lease_expires_at=now()+interval '5 minutes' WHERE id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	svc := &metadataedit.Service{Edits: &jobedit.Service{Store: store, Media: files}}
	rev, title := 0, "after"
	view, err := svc.Patch(ctx, job.ID, metadataedit.Patch{ExpectedRevision: &rev, Title: &title})
	if err != nil || view.Metadata == nil || view.Metadata.Title != "after" {
		t.Fatalf("patch while building: %+v %v", view, err)
	}
	history, err := svc.History(ctx, job.ID)
	if err != nil || len(history) != 1 || history[0].Title != "before" {
		t.Fatalf("history: %+v %v", history, err)
	}
	rev = 1
	view, err = svc.Restore(ctx, job.ID, metadataedit.Restore{ExpectedRevision: &rev, At: history[0].At})
	if err != nil || view.Metadata.Title != "before" {
		t.Fatalf("restore while building: %+v %v", view, err)
	}
	rev = 2
	if _, err = pool.Exec(ctx, `UPDATE clone_steps SET status='running' WHERE job_id=$1 AND step='metadata'`, job.ID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Patch(ctx, job.ID, metadataedit.Patch{ExpectedRevision: &rev, Title: &title})
	editConflict(t, err)
}
