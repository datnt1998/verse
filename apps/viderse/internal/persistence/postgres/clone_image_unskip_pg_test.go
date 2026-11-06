package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/platform/blobstore"
)

func TestOwnerUnskipSurvivesPublicationAndPreservesImageStep(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	seedCloneChannel(t, pool)
	store := postgres.NewCloneStore(pool)
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(ctx, media.Descriptor{ID: "unskip-test", Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.NewLocal(d.Config.LocalRoot) })
	edits := &jobedit.Service{Store: store, Media: files}
	job := cloneTestJob(t, store, "owner-unskip")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	one, two := 1, 2
	manifest := images.Manifest{Version: 1, Revision: 1, SceneRevision: 3, Items: []images.Item{
		{ID: "scene-1", Kind: "scene", Status: "review", CurrentVersion: &one, ReviewVersion: &two, Versions: []images.Version{{Version: 1}, {Version: 2}}},
		{ID: "scene-2", Kind: "scene", Status: "review"},
	}}
	original := manifest.Items[0]
	for {
		claim, err := store.ClaimNextStep(ctx, job.ID, "worker")
		if err != nil || claim == nil {
			t.Fatalf("claim: %v %v", claim, err)
		}
		output := domain.CloneStepOutput{}
		if claim.Step.Step == domain.CloneImages {
			output.Data = map[string]any{"manifest": manifest}
		}
		if err := store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, output); err != nil {
			t.Fatal(err)
		}
		if claim.Step.Step == domain.CloneImages {
			break
		}
	}
	if err := store.ReleaseJobLease(ctx, job.ID, "worker"); err != nil {
		t.Fatal(err)
	}
	before, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := images.Review(manifest, "scene-1", "skip", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := edits.CommitImageReview(ctx, before, skipped, domain.CloneBuild, false, []string{"scene-1"}); err != nil {
		t.Fatal(err)
	}
	afterSkip, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	var persisted images.Manifest
	if err := json.Unmarshal(afterSkip.Step.Checkpoint, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Items[0].Status != "skipped" || persisted.Items[0].BeforeSkip == nil || persisted.Items[0].BeforeSkip.Status != "review" {
		t.Fatalf("skip receipt was not durable: %+v", persisted.Items[0])
	}
	restored, err := images.Review(persisted, "scene-1", "unskip", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := edits.CommitImageReview(ctx, afterSkip, restored, domain.CloneBuild, false, []string{"scene-1"}); err != nil {
		t.Fatal(err)
	}
	after, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint images.Manifest
	if err := json.Unmarshal(after.Step.Checkpoint, &checkpoint); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(checkpoint.Items[0], original) {
		t.Fatalf("undo changed the selected versions or approved a review: before=%+v after=%+v", original, checkpoint.Items[0])
	}
	if after.Step.Status != domain.CloneSucceeded || after.Step.Attempt != before.Step.Attempt || after.Step.Fence != before.Step.Fence {
		t.Fatalf("undo queued image work: before=%+v after=%+v", before.Step, after.Step)
	}
	handle, err := edits.Acquire(ctx, after, "images/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(handle.Path)
	handle.Release()
	if err != nil {
		t.Fatal(err)
	}
	var published images.Manifest
	if err := json.Unmarshal(raw, &published); err != nil || !reflect.DeepEqual(published, checkpoint) {
		t.Fatalf("published undo disagrees with checkpoint: %v", err)
	}
	editConflict(t, edits.CommitImageReview(ctx, afterSkip, restored, domain.CloneBuild, false, []string{"scene-1"}))
	rejected, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil || !reflect.DeepEqual(rejected.Step.Checkpoint, after.Step.Checkpoint) {
		t.Fatalf("stale undo changed persisted state: %v", err)
	}
}
