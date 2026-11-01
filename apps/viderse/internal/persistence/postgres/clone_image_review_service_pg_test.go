package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/platform/blobstore"
)

// UseVersion commits without invalidating a step. Its review identity must
// survive the Service -> media publication -> Store path while the worker still
// holds the job lease immediately after images succeeds.
func TestCommitImageReviewPublishesVersionUnderLiveLeaseAfterImagesSuccess(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name,language,auto_pick) VALUES($1,'Clone fixture','en',false)`, autonomyChannel); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(ctx, media.Descriptor{ID: "image-review-test", Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) {
		return blobstore.NewLocal(d.Config.LocalRoot)
	})
	edits := &jobedit.Service{Store: store, Media: files}
	job := cloneTestJob(t, store, "review-version-live-lease")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	one, two := 1, 2
	manifest := images.Manifest{Version: 1, Revision: 1, SceneRevision: 3, Items: []images.Item{{
		ID: "scene-1", Kind: "scene", Status: "ready", CurrentVersion: &one,
		Versions: []images.Version{{Version: 1}, {Version: 2}},
	}}}
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
	before, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil || before.Step.Status != domain.CloneSucceeded {
		t.Fatalf("images did not succeed: %+v %v", before.Step, err)
	}
	var leaseActive bool
	if err := pool.QueryRow(ctx, `SELECT lease_owner='worker' AND lease_expires_at>now() FROM clone_jobs WHERE id=$1`, job.ID).Scan(&leaseActive); err != nil || !leaseActive {
		t.Fatalf("worker lease must remain live: %v %v", leaseActive, err)
	}
	stepsBefore, err := store.ListSteps(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	next := manifest
	next.Revision++
	next.Items = append([]images.Item(nil), manifest.Items...)
	next.Items[0].CurrentVersion = &two
	if err := edits.CommitImageReview(ctx, before, next, "", false, []string{"scene-1"}); err != nil {
		t.Fatalf("version selection after images success must publish under live lease: %v", err)
	}
	after, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil || after.Step.Status != domain.CloneSucceeded {
		t.Fatalf("review changed images status: %+v %v", after.Step, err)
	}
	var checkpoint images.Manifest
	if err := json.Unmarshal(after.Step.Checkpoint, &checkpoint); err != nil || checkpoint.Revision != 2 || len(checkpoint.Items) != 1 || checkpoint.Items[0].CurrentVersion == nil || *checkpoint.Items[0].CurrentVersion != two {
		t.Fatalf("selected version not persisted: %s %v", after.Step.Checkpoint, err)
	}
	if after.Step.Output == nil || len(after.Step.Output.Artifacts) != 1 {
		t.Fatalf("review publication not recorded in producer output: %+v", after.Step.Output)
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
		t.Fatalf("published manifest disagrees with checkpoint: %s %v", raw, err)
	}
	stepsAfter, err := store.ListSteps(ctx, job.ID)
	if err != nil || len(stepsAfter) != len(stepsBefore) {
		t.Fatalf("steps changed: %v %v", stepsAfter, err)
	}
	for i, step := range stepsBefore {
		if stepsAfter[i].Step != step.Step || stepsAfter[i].Status != step.Status || stepsAfter[i].Attempt != step.Attempt || stepsAfter[i].Fence != step.Fence {
			t.Fatalf("review invalidated or reran a step: before=%+v after=%+v", step, stepsAfter[i])
		}
	}
	if err := pool.QueryRow(ctx, `SELECT lease_owner='worker' AND lease_expires_at>now() FROM clone_jobs WHERE id=$1`, job.ID).Scan(&leaseActive); err != nil || !leaseActive {
		t.Fatalf("review changed the worker lease: %v %v", leaseActive, err)
	}
	// The review marker does not grant rerun/invalidation rights or remove CAS.
	editConflict(t, edits.CommitImageReview(ctx, before, next, "", false, []string{"scene-1"}))
	next.Revision++
	editConflict(t, edits.CommitImageReview(ctx, after, next, domain.CloneBuild, false, []string{"scene-1"}))
	editConflict(t, edits.CommitImageReview(ctx, after, next, "", true, []string{"scene-1"}))
	editConflict(t, edits.Commit(ctx, after, next, "", false, nil))
	current, err := store.Read(ctx, job.ID, domain.CloneImages)
	if err != nil || !reflect.DeepEqual(current.Step.Checkpoint, after.Step.Checkpoint) {
		t.Fatalf("rejected edit changed checkpoint: %s %v", current.Step.Checkpoint, err)
	}
	var bindings int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blob_refs WHERE left(logical_path,length($1))=$1`, "clone/"+job.ID+"/").Scan(&bindings); err != nil || bindings != 1 {
		t.Fatalf("rejected publication leaked bindings: %d %v", bindings, err)
	}
}

// Selecting a version during a redraw changes an owner intent, not the worker
// checkpoint or the already-built downstream steps.
func TestCommitImageReviewDuringRedrawPreservesBuiltStepsAndWorkerFence(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name,language,auto_pick) VALUES($1,'Clone fixture','en',false)`, autonomyChannel); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(ctx, media.Descriptor{ID: "running-image-review-test", Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) {
		return blobstore.NewLocal(d.Config.LocalRoot)
	})
	edits := &jobedit.Service{Store: store, Media: files}
	job := cloneTestJob(t, store, "review-version-during-redraw")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	one, two := 1, 2
	manifest := images.Manifest{Version: 1, Revision: 1, Items: []images.Item{
		{ID: "selected", Kind: "scene", Status: "ready", CurrentVersion: &one, Versions: []images.Version{{Version: 1}, {Version: 2}}},
		{ID: "redrawing", Kind: "scene", Status: "ready", CurrentVersion: &one, Versions: []images.Version{{Version: 1}}},
	}}
	// First finish the video through the normal worker store so the build,
	// score and export are genuinely succeeded before the isolated redraw.
	for {
		claim, err := store.ClaimNextStep(ctx, job.ID, "worker")
		if err != nil {
			t.Fatal(err)
		}
		if claim == nil {
			break
		}
		output := domain.CloneStepOutput{}
		if claim.Step.Step == domain.CloneImages {
			output.Data = map[string]any{"manifest": manifest}
		}
		if err := store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, output); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ReleaseJobLease(ctx, job.ID, "worker"); err != nil {
		t.Fatal(err)
	}
	completed, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Revision++
	manifest.QueueOnly = []string{"redrawing"}
	manifest.Items[1].Status = "pending"
	if err := edits.CommitImageRedraw(ctx, completed, manifest, []string{"redrawing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	claim, err := store.ClaimNextStep(ctx, job.ID, "worker")
	if err != nil || claim == nil || claim.Step.Step != domain.CloneImages {
		t.Fatalf("redraw did not claim images: %+v %v", claim, err)
	}
	manifest.Revision++
	manifest.Items[1].Status = "running"
	manifest.Items[1].InFlight = &images.Flight{GenerationID: "redraw-result"}
	marshal := func(value any) json.RawMessage {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, marshal(manifest)); err != nil {
		t.Fatal(err)
	}
	before, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil || before.Step.Status != domain.CloneRunning {
		t.Fatalf("images must be running: %+v %v", before.Step, err)
	}
	var leaseActive bool
	if err := pool.QueryRow(ctx, `SELECT lease_owner='worker' AND lease_expires_at>now() FROM clone_jobs WHERE id=$1`, job.ID).Scan(&leaseActive); err != nil || !leaseActive {
		t.Fatalf("redraw lease must remain live: %v %v", leaseActive, err)
	}
	built := map[domain.CloneStep]domain.CloneStepRecord{}
	for _, step := range before.Steps {
		if step.Step == domain.CloneBuild || step.Step == domain.CloneScore || step.Step == domain.CloneExport {
			if step.Status != domain.CloneSucceeded {
				t.Fatalf("fixture downstream step was not built: %+v", step)
			}
			built[step.Step] = step
		}
	}
	if len(built) != 3 {
		t.Fatalf("fixture missing built steps: %+v", built)
	}
	assertWorkerState := func(wantCheckpoint json.RawMessage, intents int) {
		t.Helper()
		step, err := store.GetStep(ctx, job.ID, domain.CloneImages)
		if err != nil || step.Status != domain.CloneRunning || step.Fence != before.Step.Fence || step.Attempt != before.Step.Attempt || !reflect.DeepEqual(step.Checkpoint, wantCheckpoint) {
			t.Fatalf("owner selection changed worker checkpoint or fence: %+v %v", step, err)
		}
		steps, err := store.ListSteps(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, step := range steps {
			if previous, ok := built[step.Step]; ok {
				if step.Status != previous.Status || step.Fence != previous.Fence || step.Attempt != previous.Attempt {
					t.Fatalf("selection invalidated built step: before=%+v after=%+v", previous, step)
				}
			}
		}
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM clone_image_review_intents WHERE job_id=$1`, job.ID).Scan(&count); err != nil || count != intents {
			t.Fatalf("review intents=%d, want %d: %v", count, intents, err)
		}
	}
	next := manifest
	next.Revision++
	next.Items = append([]images.Item(nil), manifest.Items...)
	next.Items[0].CurrentVersion = &two
	if err := edits.CommitImageReview(ctx, before, next, "", false, []string{"selected"}); err != nil {
		t.Fatal(err)
	}
	// This is the fail-before assertion: the old running-image review branch
	// marked every dependent stale despite this being a selection-only edit.
	assertWorkerState(before.Step.Checkpoint, 1)
	visible, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	var projected images.Manifest
	if err := json.Unmarshal(visible.Manifest, &projected); err != nil || projected.Items[0].CurrentVersion == nil || *projected.Items[0].CurrentVersion != two {
		t.Fatalf("selected version absent from owner projection: %s %v", visible.Manifest, err)
	}
	// A worker finishing a different image must merge the accepted selection.
	worker := manifest
	worker.Revision++
	worker.Items = append([]images.Item(nil), manifest.Items...)
	worker.Items[1].Status = "ready"
	worker.Items[1].InFlight = nil
	worker.Items[1].CurrentVersion = &two
	worker.Items[1].Versions = append([]images.Version(nil), manifest.Items[1].Versions...)
	worker.Items[1].Versions = append(worker.Items[1].Versions, images.Version{Version: 2, GenerationID: "redraw-result"})
	workerRaw := marshal(worker)
	if _, err := store.DrainImageIntents(ctx, job.ID, claim.Fence+1, claim.Step.Attempt, workerRaw); !errors.Is(err, pipeline.ErrFence) {
		t.Fatalf("stale worker drained owner intent: %v", err)
	}
	assertWorkerState(before.Step.Checkpoint, 1)
	drained, err := store.DrainImageIntents(ctx, job.ID, claim.Fence, claim.Step.Attempt, workerRaw)
	if err != nil {
		t.Fatal(err)
	}
	var merged images.Manifest
	if err := json.Unmarshal(drained, &merged); err != nil || merged.Items[0].CurrentVersion == nil || *merged.Items[0].CurrentVersion != two || len(merged.Items[1].Versions) != 2 || merged.Items[1].Versions[1].GenerationID != "redraw-result" {
		t.Fatalf("drain lost selection or concurrent worker result: %s %v", drained, err)
	}
	persisted, err := store.GetStep(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	var persistedManifest images.Manifest
	if err := json.Unmarshal(persisted.Checkpoint, &persistedManifest); err != nil || !reflect.DeepEqual(persistedManifest, merged) {
		t.Fatalf("drain did not persist merged checkpoint: %s %v", persisted.Checkpoint, err)
	}
	assertWorkerState(persisted.Checkpoint, 0)
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt+1, workerRaw); !errors.Is(err, pipeline.ErrFence) {
		t.Fatalf("wrong-attempt worker overwrote selected version: %v", err)
	}
	assertWorkerState(persisted.Checkpoint, 0)
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, drained); err != nil {
		t.Fatalf("current worker could not save merged checkpoint: %v", err)
	}
	assertWorkerState(persisted.Checkpoint, 0)
}
