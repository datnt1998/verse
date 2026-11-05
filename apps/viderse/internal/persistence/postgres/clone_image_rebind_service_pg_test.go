package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/platform/blobstore"
)

// Rebinding an existing project owns the build step, not the unrelated images
// worker's job fence. A retry from the same snapshot is idempotent only for the
// exact checkpoint and artifact set already committed by another publication.
func TestCommitBuildMediaConcurrentIdenticalPublicationUnderRedrawLease(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	seedCloneChannel(t, pool)
	store := postgres.NewCloneStore(pool)
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(ctx, media.Descriptor{ID: "image-rebind-test", Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) {
		return blobstore.NewLocal(d.Config.LocalRoot)
	})
	edits := &jobedit.Service{Store: store, Media: files}
	job := cloneTestJob(t, store, "concurrent-build-media")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	one := 1
	imageManifest := images.Manifest{Version: 1, Revision: 1, Items: []images.Item{{
		ID: "scene-1", Kind: "scene", Status: "ready", CurrentVersion: &one, Versions: []images.Version{{Version: 1}},
	}}}
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
			output.Data = map[string]any{"manifest": imageManifest}
		}
		if err := store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, output); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ReleaseJobLease(ctx, job.ID, "worker"); err != nil {
		t.Fatal(err)
	}
	beforeBuild, err := edits.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil || beforeBuild.Step.Status != domain.CloneSucceeded {
		t.Fatalf("build did not succeed: %+v %v", beforeBuild.Step, err)
	}
	beforeImages, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	imageManifest.Revision++
	imageManifest.Items[0].Status = "pending"
	imageManifest.QueueOnly = []string{"scene-1"}
	if err := edits.CommitImageRedraw(ctx, beforeImages, imageManifest, []string{"scene-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	claim, err := store.ClaimNextStep(ctx, job.ID, "worker")
	if err != nil || claim == nil || claim.Step.Step != domain.CloneImages {
		t.Fatalf("redraw did not claim images: %+v %v", claim, err)
	}
	duringRedraw, err := edits.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	if duringRedraw.Job.Fence == beforeBuild.Job.Fence || duringRedraw.Step.Fence != beforeBuild.Step.Fence || duringRedraw.Step.Attempt != beforeBuild.Step.Attempt || duringRedraw.Step.Status != beforeBuild.Step.Status || !reflect.DeepEqual(duringRedraw.Step.Checkpoint, beforeBuild.Step.Checkpoint) {
		t.Fatalf("fixture must advance only the unrelated job fence: before=%+v after=%+v", beforeBuild, duringRedraw)
	}
	var leaseActive bool
	if err := pool.QueryRow(ctx, `SELECT lease_owner='worker' AND lease_expires_at>now() FROM clone_jobs WHERE id=$1`, job.ID).Scan(&leaseActive); err != nil || !leaseActive {
		t.Fatalf("images lease must remain live: %v %v", leaseActive, err)
	}
	desired := map[string]any{"revision": 2, "projectId": "saved-workspace", "mediaId": "redrawn-scene"}
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outcomes <- edits.CommitBuildMedia(ctx, beforeBuild, desired, nil)
		}()
	}
	wg.Wait()
	close(outcomes)
	for err := range outcomes {
		if err != nil {
			t.Fatalf("identical build rebind must survive unrelated job fence and concurrent exact retry: %v", err)
		}
	}
	if err := edits.CommitBuildMedia(ctx, beforeBuild, desired, nil); err != nil {
		t.Fatalf("identical publication from the same old snapshot must be idempotent: %v", err)
	}
	accepted, err := edits.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil || accepted.Step.Output == nil || len(accepted.Step.Output.Artifacts) != 1 {
		t.Fatalf("build publication missing: %+v %v", accepted.Step, err)
	}
	handle, err := edits.Acquire(ctx, accepted, "build/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(handle.Path)
	handle.Release()
	if err != nil {
		t.Fatal(err)
	}
	var published map[string]any
	if err := json.Unmarshal(raw, &published); err != nil || published["revision"] != float64(2) || published["mediaId"] != desired["mediaId"] {
		t.Fatalf("wrong published build: %s %v", raw, err)
	}
	var bindings int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blob_refs WHERE left(logical_path,length($1))=$1`, "clone/"+job.ID+"/").Scan(&bindings); err != nil {
		t.Fatal(err)
	}
	assertRejectedPublicationUnchanged := func(t *testing.T) {
		t.Helper()
		current, err := store.Read(ctx, job.ID, domain.CloneBuild)
		if err != nil || current.Step.Output == nil || !reflect.DeepEqual(current.Step.Checkpoint, accepted.Step.Checkpoint) || !reflect.DeepEqual(current.Step.Output.Artifacts, accepted.Step.Output.Artifacts) {
			t.Fatalf("rejected publication changed build checkpoint or artifact identity: %+v %v", current.Step, err)
		}
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM blob_refs WHERE left(logical_path,length($1))=$1`, "clone/"+job.ID+"/").Scan(&count); err != nil || count != bindings {
			t.Fatalf("rejected publication leaked bindings: %d, want %d: %v", count, bindings, err)
		}
	}
	divergent := map[string]any{"revision": 2, "projectId": "saved-workspace", "mediaId": "different-scene"}
	editConflict(t, edits.CommitBuildMedia(ctx, beforeBuild, divergent, nil))
	assertRejectedPublicationUnchanged(t)
	editConflict(t, store.Commit(ctx, beforeBuild, jobedit.Change{Checkpoint: accepted.Step.Checkpoint, ImageRebind: true}))
	assertRejectedPublicationUnchanged(t)
	for _, kind := range []string{"foreign-path", "foreign-step"} {
		t.Run(kind, func(t *testing.T) {
			artifact := accepted.Step.Output.Artifacts[0]
			if kind == "foreign-path" {
				artifact.Path = "clone/00000000-0000-4000-8000-000000000099/build/manifest.json"
			} else {
				artifact.Step = domain.CloneImages
			}
			err := store.Commit(ctx, beforeBuild, jobedit.Change{Checkpoint: accepted.Step.Checkpoint, Artifacts: []domain.CloneArtifact{artifact}, ImageRebind: true})
			var invalid *domain.CloneError
			if !errors.As(err, &invalid) || invalid.Code != domain.CloneErrInvalidRequest {
				t.Fatalf("foreign artifact must fail ownership validation: %v", err)
			}
			assertRejectedPublicationUnchanged(t)
		})
	}
	// A matching checkpoint alone is insufficient: both an additional artifact
	// and different bytes for the same manifest identity must remain conflicts.
	extraFile := filepath.Join(t.TempDir(), "extra.json")
	if err := os.WriteFile(extraFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	editConflict(t, edits.CommitBuildMedia(ctx, beforeBuild, desired, []media.Artifact{{Path: "build/extra.json", File: extraFile, ContentType: "application/json"}}))
	assertRejectedPublicationUnchanged(t)
	editConflict(t, edits.CommitBuildMedia(ctx, beforeBuild, desired, []media.Artifact{{Path: "build/manifest.json", File: extraFile, ContentType: "application/json", Filename: "different.json"}}))
	assertRejectedPublicationUnchanged(t)
	differentFile := filepath.Join(t.TempDir(), "different.json")
	if err := os.WriteFile(differentFile, []byte(`{"revision":99,"mediaId":"different-scene"}`), 0600); err != nil {
		t.Fatal(err)
	}
	editConflict(t, edits.CommitBuildMedia(ctx, beforeBuild, desired, []media.Artifact{{Path: "build/manifest.json", File: differentFile, ContentType: "application/json"}}))
	assertRejectedPublicationUnchanged(t)
	for _, state := range []struct {
		name    string
		change  string
		restore string
		value   any
	}{
		{name: "step-fence", change: `UPDATE clone_steps SET fence=fence+1 WHERE job_id=$1 AND step='build'`, restore: `UPDATE clone_steps SET fence=$2 WHERE job_id=$1 AND step='build'`, value: accepted.Step.Fence},
		{name: "step-attempt", change: `UPDATE clone_steps SET attempt=attempt+1 WHERE job_id=$1 AND step='build'`, restore: `UPDATE clone_steps SET attempt=$2 WHERE job_id=$1 AND step='build'`, value: accepted.Step.Attempt},
		{name: "step-status", change: `UPDATE clone_steps SET status='stale' WHERE job_id=$1 AND step='build'`, restore: `UPDATE clone_steps SET status=$2 WHERE job_id=$1 AND step='build'`, value: string(accepted.Step.Status)},
	} {
		t.Run(state.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, state.change, job.ID); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := pool.Exec(ctx, state.restore, job.ID, state.value); err != nil {
					t.Error(err)
				}
			})
			editConflict(t, edits.CommitBuildMedia(ctx, beforeBuild, desired, nil))
			assertRejectedPublicationUnchanged(t)
		})
	}
	// A real subsequent build edit is not an exact retry, even though the old
	// desired publication once committed successfully under this same lease.
	changed := map[string]any{"revision": 3, "projectId": "saved-workspace", "mediaId": "newer-scene"}
	if err := edits.CommitBuildMedia(ctx, accepted, changed, nil); err != nil {
		t.Fatalf("fresh build snapshot could not publish changed checkpoint: %v", err)
	}
	editConflict(t, edits.CommitBuildMedia(ctx, beforeBuild, desired, nil))
	latest, err := store.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint map[string]any
	if err := json.Unmarshal(latest.Step.Checkpoint, &checkpoint); err != nil || checkpoint["revision"] != float64(3) || checkpoint["mediaId"] != "newer-scene" {
		t.Fatalf("stale retry overwrote changed real build checkpoint: %s %v", latest.Step.Checkpoint, err)
	}
}

// ProjectBinder.commitBuild publishes new manifest bytes while keeping the
// original build checkpoint. The output itself must therefore be a CAS token.
func TestCommitBuildMediaUnchangedCheckpointRejectsLostManifest(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	seedCloneChannel(t, pool)
	store := postgres.NewCloneStore(pool)
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(ctx, media.Descriptor{ID: "unchanged-checkpoint", Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) {
		return blobstore.NewLocal(d.Config.LocalRoot)
	})
	edits := &jobedit.Service{Store: store, Media: files}
	job := cloneTestJob(t, store, "unchanged-build-checkpoint")
	if _, err := pool.Exec(ctx, `UPDATE clone_steps SET status='succeeded',checkpoint='{"version":1}' WHERE job_id=$1 AND step='build'`, job.ID); err != nil {
		t.Fatal(err)
	}
	publish := func(snapshot jobedit.Snapshot, manifest, other string) error {
		t.Helper()
		artifacts := []media.Artifact{}
		for _, entry := range []struct{ path, body string }{{"build/manifest.json", manifest}, {"build/other.json", other}} {
			if entry.body == "" {
				continue
			}
			file := filepath.Join(t.TempDir(), filepath.Base(entry.path))
			if err := os.WriteFile(file, []byte(entry.body), 0600); err != nil {
				t.Fatal(err)
			}
			artifacts = append(artifacts, media.Artifact{Path: entry.path, File: file, ContentType: "application/json"})
		}
		return edits.CommitBuildMedia(ctx, snapshot, json.RawMessage(snapshot.Step.Checkpoint), artifacts)
	}
	seed, err := store.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	if err := publish(seed, `{"scenes":[{"sceneId":"a","mediaId":"a0"},{"sceneId":"b","mediaId":"b0"}]}`, `{"value":0}`); err != nil {
		t.Fatal(err)
	}
	before, err := store.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	desired := `{"scenes":[{"sceneId":"a","mediaId":"a1"},{"sceneId":"b","mediaId":"b0"}]}`
	if err := publish(before, desired, ""); err != nil {
		t.Fatal(err)
	}
	accepted, err := store.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil || !reflect.DeepEqual(accepted.Step.Checkpoint, before.Step.Checkpoint) {
		t.Fatalf("production rebind must keep checkpoint: %+v %v", accepted.Step, err)
	}
	var bindings, artifacts int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM blob_refs),(SELECT count(*) FROM clone_artifacts WHERE job_id=$1)`, job.ID).Scan(&bindings, &artifacts); err != nil {
		t.Fatal(err)
	}
	editConflict(t, publish(before, `{"scenes":[{"sceneId":"a","mediaId":"a0"},{"sceneId":"b","mediaId":"b1"}]}`, ""))
	if err := publish(before, desired, ""); err != nil {
		t.Fatalf("exact unchanged-checkpoint retry failed: %v", err)
	}
	after, err := store.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil || !reflect.DeepEqual(after.Step, accepted.Step) {
		t.Fatalf("stale publication mutated build: %+v %v", after.Step, err)
	}
	var currentBindings, currentArtifacts int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM blob_refs),(SELECT count(*) FROM clone_artifacts WHERE job_id=$1)`, job.ID).Scan(&currentBindings, &currentArtifacts); err != nil || currentBindings != bindings || currentArtifacts != artifacts {
		t.Fatalf("rejected publication leaked rows: bindings=%d artifacts=%d: %v", currentBindings, currentArtifacts, err)
	}
	handle, err := edits.Acquire(ctx, after, "build/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(handle.Path)
	handle.Release()
	if err != nil || string(raw) != desired {
		t.Fatalf("accepted manifest lost: %s %v", raw, err)
	}
	// Even if the requested manifest still matches, an unrelated output edit
	// means this old publication is no longer the complete desired state.
	if err := publish(accepted, desired, `{"value":1}`); err != nil {
		t.Fatal(err)
	}
	editConflict(t, publish(before, desired, ""))
	latest, err := store.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	editConflict(t, publish(before, `{"scenes":[]}`, ""))
	current, err := store.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil || !reflect.DeepEqual(current.Step, latest.Step) {
		t.Fatalf("old retry replaced newer complete output: %+v %v", current.Step, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE clone_steps SET output_manifest=jsonb_set(output_manifest,'{data,independentEdit}','true'::jsonb) WHERE job_id=$1 AND step='build'`, job.ID); err != nil {
		t.Fatal(err)
	}
	editConflict(t, publish(latest, desired, ""))
	current, err = store.Read(ctx, job.ID, domain.CloneBuild)
	if err != nil || current.Step.Output == nil || current.Step.Output.Data["independentEdit"] != true {
		t.Fatalf("stale retry lost unrelated output data: %+v %v", current.Step, err)
	}
}
