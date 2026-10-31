package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func editConflict(t *testing.T, err error) {
	t.Helper()
	var conflict *domain.CloneError
	if !errors.As(err, &conflict) || conflict.Code != domain.CloneErrConflict {
		t.Fatalf("want conflict, got %v", err)
	}
}

func TestCloneEditCompareAndSwapPersistsAndRejectsStale(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	job := cloneTestJob(t, store, "image-edit")
	before, err := store.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	cp := json.RawMessage(`{"revision":2,"items":[]}`)
	change := jobedit.Change{Checkpoint: cp, InvalidateFrom: domain.CloneImages, Rerun: true}
	if err := store.Commit(ctx, before, change); err != nil {
		t.Fatal(err)
	}
	after, err := store.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Revision int `json:"revision"`
	}
	if err := json.Unmarshal(after.Step.Checkpoint, &decoded); err != nil || decoded.Revision != 2 {
		t.Fatalf("checkpoint not persisted: %s %v", after.Step.Checkpoint, err)
	}
	editConflict(t, store.Commit(ctx, before, jobedit.Change{Checkpoint: json.RawMessage(`{"revision":3}`), InvalidateFrom: domain.CloneImages, Rerun: true}))
	if after.Job.Status != domain.ClonePending {
		t.Fatalf("job not requeued: %s", after.Job.Status)
	}
}

func TestCloneEditRefusesRunningDependentAndSuperseded(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	job := cloneTestJob(t, store, "running-edit")
	before, err := store.Read(ctx, job.ID, domain.CloneDownload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	if claim, err := store.ClaimNextStep(ctx, job.ID, "worker"); err != nil || claim == nil {
		t.Fatalf("claim: %v %v", claim, err)
	}
	editConflict(t, store.Commit(ctx, before, jobedit.Change{Checkpoint: json.RawMessage(`{"revision":2}`), InvalidateFrom: domain.CloneDownload, Rerun: true}))
}

func TestCloneEditPublishesArtifactIntoProducerOutput(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	job := cloneTestJob(t, store, "published-edit")
	before, err := store.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	artifact := domain.CloneArtifact{ID: "edit-manifest", Path: "clone/" + job.ID + "/hash/images/manifest.json", Step: domain.CloneImages, SHA256: strings.Repeat("a", 64), ContentType: "application/json"}
	cp := json.RawMessage(`{"revision":4,"items":[]}`)
	if err := store.Commit(ctx, before, jobedit.Change{Checkpoint: cp, InvalidateFrom: domain.CloneBuild, Artifacts: []domain.CloneArtifact{artifact}}); err != nil {
		t.Fatal(err)
	}
	after, err := store.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	if after.Step.Output == nil || len(after.Step.Output.Artifacts) != 1 || after.Step.Output.Artifacts[0].Path != artifact.Path {
		t.Fatalf("producer output does not point to edit: %+v", after.Step.Output)
	}
	if len(after.Artifacts) != 1 {
		t.Fatalf("published artifacts not recorded: %+v", after.Artifacts)
	}
}
