package editor_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

// Only the build manifest seam is a fixture. Acquire and editor publication use
// the real PostgreSQL shared/exclusive digest leases that previously deadlocked.
type rebindLeaseSteps struct {
	media     *media.Service
	projectID string
}

func (s rebindLeaseSteps) Read(_ context.Context, _ string, step domain.CloneStep) (jobedit.Snapshot, error) {
	return jobedit.Snapshot{Step: domain.CloneStepRecord{Step: step, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": s.projectID}}}, Manifest: json.RawMessage(`{"scenes":[],"thumbnail":{}}`)}, nil
}
func (s rebindLeaseSteps) Acquire(ctx context.Context, _ jobedit.Snapshot, path string) (*media.Handle, error) {
	return s.media.Acquire(ctx, path)
}
func (s rebindLeaseSteps) Commit(context.Context, jobedit.Snapshot, any, domain.CloneStep, bool, []media.Artifact) error {
	return nil
}
func (s rebindLeaseSteps) CommitBuildMedia(_ context.Context, _ jobedit.Snapshot, _ any, artifacts []media.Artifact) error {
	_, err := os.ReadFile(artifacts[0].File)
	return err
}

func TestCloneRebindReleasesReadLeaseBeforeEditorPublication(t *testing.T) {
	store, repo, transport, _, _ := editorPersistenceStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	project := persistenceProject(t, "rebind-lease")
	if _, err := store.Create(ctx, project); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "second-version.png")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 9))
	img.Set(0, 0, color.RGBA{R: 220, G: 80, B: 30, A: 255})
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	sha := hex.EncodeToString(digest[:])
	path := "clone/rebind-lease/images/second-version.png"
	if err := transport.Publish(ctx, []media.Artifact{{Path: path, File: file, ContentType: "image/png"}}, nil); err != nil {
		t.Fatal(err)
	}
	service := &editor.Service{Store: store, Repository: repo, Media: transport, StagingDir: store.StagingDir}
	binder := cloneimages.ProjectBinder{Edits: rebindLeaseSteps{media: transport, projectID: "rebind-lease"}, Editor: service}
	result, err := binder.Rebind(ctx, cloneimages.RebindRequest{JobID: "rebind-lease", ImageID: "thumbnail", Kind: "thumbnail", Version: 2, Path: path, SHA256: sha, Mime: "image/png"})
	if err != nil {
		t.Fatalf("rebind must finish before deadline, not wait on its own read lease: %v", err)
	}
	if result == nil || result.MediaID == "" {
		t.Fatalf("missing imported image: %+v", result)
	}
	metadata, err := service.GetMediaMetadata(ctx, result.MediaID)
	if err != nil || metadata.ContentHash != "sha256:"+sha {
		t.Fatalf("imported metadata=%+v err=%v", metadata, err)
	}
}

// Synchronize only the read boundary, not publication: both real binders start
// from the same PostgreSQL snapshot and then race through real Editor saves.
type concurrentRebindSteps struct {
	*jobedit.Service
	reads   atomic.Int32
	commits atomic.Int32
	ready   chan struct{}
}

func (s *concurrentRebindSteps) Read(ctx context.Context, id string, step domain.CloneStep) (jobedit.Snapshot, error) {
	snapshot, err := s.Service.Read(ctx, id, step)
	if err != nil || step != domain.CloneBuild {
		return snapshot, err
	}
	n := s.reads.Add(1)
	if n == 2 {
		close(s.ready)
	}
	if n <= 2 {
		select {
		case <-s.ready:
		case <-ctx.Done():
			return jobedit.Snapshot{}, ctx.Err()
		}
	}
	return snapshot, nil
}

func (s *concurrentRebindSteps) CommitBuildMedia(ctx context.Context, snapshot jobedit.Snapshot, checkpoint any, artifacts []media.Artifact) error {
	s.commits.Add(1)
	return s.Service.CommitBuildMedia(ctx, snapshot, checkpoint, artifacts)
}

func TestCloneRebindConcurrentDifferentScenesPreservesBothBindings(t *testing.T) {
	store, repo, files, pool, _ := editorPersistenceStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	channel, job, pid := "00000000-0000-4000-8000-0000000000e1", "00000000-0000-4000-8000-0000000000e2", "rebind-concurrent"
	seedEditorCloneChannel(t, pool, channel)
	if _, err := pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,generation) VALUES($1,$2,'rebind-concurrent','{}',1)`, job, channel); err != nil {
		t.Fatal(err)
	}
	encode := func(value any) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	output := domain.CloneStepOutput{Data: map[string]any{"projectId": pid}, Artifacts: []domain.CloneArtifact{}}
	if _, err := pool.Exec(ctx, `INSERT INTO clone_steps(job_id,step,status,checkpoint,output_manifest) VALUES($1,'build','succeeded','{}',$2)`, job, encode(output)); err != nil {
		t.Fatal(err)
	}
	requests := []cloneimages.RebindRequest{}
	imageOutput := domain.CloneStepOutput{Artifacts: []domain.CloneArtifact{}}
	for index, scene := range []string{"a", "b"} {
		file := filepath.Join(t.TempDir(), scene+".png")
		f, err := os.Create(file)
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 16, 9))
		img.Set(0, 0, color.RGBA{R: uint8(120 + index), G: 80, B: 30, A: 255})
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		digest := hex.EncodeToString(sum[:])
		path := "clone/" + job + "/" + digest + "/images/" + scene + ".png"
		if err := files.Publish(ctx, []media.Artifact{{Path: path, File: file, ContentType: "image/png"}}, nil); err != nil {
			t.Fatal(err)
		}
		imageOutput.Artifacts = append(imageOutput.Artifacts, domain.CloneArtifact{ID: scene, Path: path, SHA256: digest, Step: domain.CloneImages, Bytes: int64(len(body)), ContentType: "image/png"})
		requests = append(requests, cloneimages.RebindRequest{JobID: job, ImageID: scene, SceneID: scene, Kind: "scene", Version: 2, Path: path, SHA256: digest, Mime: "image/png", ApplyProject: true})
	}
	if _, err := pool.Exec(ctx, `INSERT INTO clone_steps(job_id,step,status,checkpoint,output_manifest) VALUES($1,'images','succeeded','{}',$2)`, job, encode(imageOutput)); err != nil {
		t.Fatal(err)
	}
	service := &editor.Service{Store: store, Repository: repo, Media: files, StagingDir: store.StagingDir, MediaReferences: repo.MediaReferences}
	old, _, err := service.ImportFileOnce(ctx, editor.ImportFileOnceOptions{File: filepath.Join("testdata", "__fixtures__", "image.png"), FileName: "old.png", DeclaredMIME: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	project := persistenceProject(t, pid)
	project["timeline"] = editor.Object{
		"tracks": []any{editor.Object{"id": "scenes", "kind": "video"}},
		"items": []any{
			editor.Object{"id": pid + "-scene-a", "type": "image", "trackId": "scenes", "label": "Scene A", "mediaId": old.ID, "from": 0, "durationInFrames": 30},
			editor.Object{"id": pid + "-scene-b", "type": "image", "trackId": "scenes", "label": "Scene B", "mediaId": old.ID, "from": 30, "durationInFrames": 30},
		},
	}
	if _, err := store.Create(ctx, project); err != nil {
		t.Fatal(err)
	}
	edits := &jobedit.Service{Store: postgres.NewCloneStore(pool), Media: files}
	build, err := edits.Read(ctx, job, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	manifest := encode(editor.Object{"scenes": []any{editor.Object{"sceneId": "a", "mediaId": old.ID}, editor.Object{"sceneId": "b", "mediaId": old.ID}}, "thumbnail": editor.Object{}})
	file := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(file, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err := edits.CommitBuildMedia(ctx, build, json.RawMessage(build.Step.Checkpoint), []media.Artifact{{Path: "build/manifest.json", File: file, ContentType: "application/json"}}); err != nil {
		t.Fatal(err)
	}
	steps := &concurrentRebindSteps{Service: edits, ready: make(chan struct{})}
	binder := &cloneimages.ProjectBinder{Edits: steps, Editor: service}
	type outcome struct {
		result *cloneimages.RebindResult
		err    error
	}
	done := make(chan outcome, 2)
	for _, request := range requests {
		go func() {
			result, err := binder.Rebind(ctx, request)
			done <- outcome{result, err}
		}()
	}
	ids := map[string]bool{}
	expected := map[string]string{}
	for range requests {
		out := <-done
		if out.err != nil || out.result == nil {
			t.Fatalf("independent rebind failed: %+v %v", out.result, out.err)
		}
		ids[out.result.MediaID] = true
		expected[out.result.ClipID] = out.result.MediaID
	}
	if len(ids) != 2 || len(expected) != 2 {
		t.Fatalf("expected two distinct successful clip bindings: ids=%v clips=%v", ids, expected)
	}
	current, err := edits.Read(ctx, job, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	var published struct {
		Scenes []struct {
			SceneID string `json:"sceneId"`
			MediaID string `json:"mediaId"`
		} `json:"scenes"`
	}
	if err := json.Unmarshal(current.Manifest, &published); err != nil || len(published.Scenes) != 2 {
		t.Fatalf("invalid merged manifest: %s %v", current.Manifest, err)
	}
	manifestIDs := map[string]bool{}
	for _, scene := range published.Scenes {
		if scene.MediaID != expected[pid+"-scene-"+scene.SceneID] {
			t.Fatalf("successful scene binding lost: %s", current.Manifest)
		}
		manifestIDs[scene.MediaID] = true
	}
	if len(manifestIDs) != 2 {
		t.Fatalf("merged manifest duplicated a successful binding: %s", current.Manifest)
	}
	saved, _, err := service.LoadProject(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range saved.Timeline()["items"].([]any) {
		clip := item.(map[string]any)
		id, _ := clip["mediaId"].(string)
		if id != expected[clip["id"].(string)] || clip["durationInFrames"] != float64(30) {
			t.Fatalf("saved clip lost binding or editorial state: %+v", clip)
		}
		handle, err := service.AcquireMedia(ctx, id)
		if err != nil {
			t.Fatalf("saved clip source unavailable: %v", err)
		}
		handle.Release()
	}
	if steps.commits.Load() != 3 {
		t.Fatalf("expected two publications and one real CAS retry, got %d", steps.commits.Load())
	}
}
