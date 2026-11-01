package images

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

type fakeSteps struct {
	snapshots map[domain.CloneStep]jobedit.Snapshot
	committed []media.Artifact
	commits   int
}

func (f *fakeSteps) Read(_ context.Context, _ string, step domain.CloneStep) (jobedit.Snapshot, error) {
	return f.snapshots[step], nil
}
func (f *fakeSteps) Acquire(_ context.Context, _ jobedit.Snapshot, path string) (*media.Handle, error) {
	return &media.Handle{Path: path}, nil
}
func (f *fakeSteps) commit(artifacts []media.Artifact) error {
	f.commits++
	// Tệp tạm bị xoá ngay sau Commit: đọc nội dung lúc commit.
	for _, artifact := range artifacts {
		raw, err := os.ReadFile(artifact.File)
		if err != nil {
			return err
		}
		artifact.File = string(raw)
		f.committed = append(f.committed, artifact)
	}
	return nil
}
func (f *fakeSteps) Commit(_ context.Context, _ jobedit.Snapshot, _ any, _ domain.CloneStep, _ bool, artifacts []media.Artifact) error {
	return f.commit(artifacts)
}
func (f *fakeSteps) CommitBuildMedia(_ context.Context, _ jobedit.Snapshot, _ any, artifacts []media.Artifact) error {
	return f.commit(artifacts)
}

type fakeProject struct {
	project      editor.Project
	revision     string
	commands     [][]editor.Command
	imports      int
	importFiles  []string
	importBodies [][]byte
	mediaByID    map[string]editor.MediaRecord
}

func (f *fakeProject) ImportFileOnce(_ context.Context, opts editor.ImportFileOnceOptions) (editor.MediaRecord, bool, error) {
	f.imports++
	body, err := os.ReadFile(opts.File)
	if err != nil {
		return editor.MediaRecord{}, false, err
	}
	f.importFiles = append(f.importFiles, opts.File)
	f.importBodies = append(f.importBodies, body)
	record := editor.MediaRecord{ID: "media-new", ContentHash: opts.ContentHash, MIME: opts.DeclaredMIME}
	f.mediaByID[record.ID] = record
	return record, true, nil
}
func (f *fakeProject) LoadProject(context.Context, string) (editor.Project, string, error) {
	return f.project, f.revision, nil
}
func (f *fakeProject) RunCommands(_ context.Context, _ string, commands []editor.Command, _ editor.ApplyCommandsOptions) (editor.Object, error) {
	f.commands = append(f.commands, commands)
	return editor.Object{}, nil
}

// TestRebindKeepsKeyframesAndMotion: thay media của clip bằng ảnh bản mới chỉ
// đổi mediaId/src — chuyển động, keyframe, transform và thời lượng giữ nguyên
// — và manifest bước dựng theo đúng ảnh mới.
func TestRebindKeepsKeyframesAndMotion(t *testing.T) {
	dir := t.TempDir()
	imageFile := dir + "/2-x.webp"
	if err := os.WriteFile(imageFile, []byte("webp"), 0644); err != nil {
		t.Fatal(err)
	}
	buildManifest := map[string]any{
		"scenes":    []any{map[string]any{"sceneId": "sc-0", "mediaId": "media-old", "src": "/api/editor/media/media-old?v=sha256:old", "startFrame": 0, "endFrame": 30}},
		"thumbnail": map[string]any{"mediaId": "media-cover", "sha256": "cover"},
	}
	raw, _ := json.Marshal(buildManifest)
	steps := &fakeSteps{snapshots: map[domain.CloneStep]jobedit.Snapshot{
		domain.CloneBuild:  {Job: domain.CloneJob{ID: "job-1", Generation: 1}, Step: domain.CloneStepRecord{Step: domain.CloneBuild, Status: domain.CloneSucceeded, Checkpoint: json.RawMessage(`{"version":1}`), Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "clone-job-1-g1"}}}, Manifest: raw},
		domain.CloneImages: {Job: domain.CloneJob{ID: "job-1"}, Step: domain.CloneStepRecord{Step: domain.CloneImages, Checkpoint: json.RawMessage(`{}`)}},
	}}
	project := &fakeProject{revision: "r1", mediaByID: map[string]editor.MediaRecord{}, project: editor.Project{
		"timeline": map[string]any{"items": []any{
			map[string]any{"id": "clone-job-1-g1-scene-sc-0", "type": "image", "mediaId": "media-old", "src": "/api/editor/media/media-old?v=sha256:old", "from": 0, "durationInFrames": 30,
				"transform": map[string]any{"x": 0.5}, "motionModifiers": []any{map[string]any{"type": "zoom", "amount": 0.1}},
				"keyframes": []any{map[string]any{"property": "scale", "frame": 0, "value": 1}}},
		}},
	}}
	measured := 0
	binder := &ProjectBinder{Edits: steps, Editor: project, Measure: func(context.Context, string) error { measured++; return nil }}
	out, err := binder.Rebind(context.Background(), RebindRequest{JobID: "job-1", ImageID: "sc-0", SceneID: "sc-0", Kind: "scene", Version: 2, Path: imageFile, SHA256: "new", Mime: "image/webp", ApplyProject: true})
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.MediaID != "media-new" || out.ClipID != "clone-job-1-g1-scene-sc-0" || out.PreviousMediaID != "media-old" {
		t.Fatalf("out=%+v", out)
	}
	if len(project.importFiles) != 1 || project.importFiles[0] == imageFile || string(project.importBodies[0]) != "webp" {
		t.Fatalf("import must use a private copy, not the leased source: files=%v bodies=%q", project.importFiles, project.importBodies)
	}
	if _, err := os.Stat(project.importFiles[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged import copy leaked: %v", err)
	}
	if len(project.commands) != 1 {
		t.Fatalf("commands=%v", project.commands)
	}
	command := project.commands[0][0]
	if command.Type != "item.update" || command.Params["id"] != "clone-job-1-g1-scene-sc-0" {
		t.Fatalf("command=%+v", command)
	}
	updates := command.Params["updates"].(editor.Object)
	if updates["mediaId"] != "media-new" || updates["src"] != out.Src {
		t.Fatalf("updates=%+v", updates)
	}
	for _, key := range []string{"transform", "motionModifiers", "keyframes", "from", "durationInFrames"} {
		if _, exists := updates[key]; exists {
			t.Fatalf("lệnh thay media không được đụng %q", key)
		}
	}
	if steps.commits != 1 || len(steps.committed) != 1 || steps.committed[0].Path != "build/manifest.json" {
		t.Fatalf("commit=%v", steps.committed)
	}
	updated := []byte(steps.committed[0].File)
	var manifest struct {
		Scenes []struct {
			SceneID string `json:"sceneId"`
			MediaID string `json:"mediaId"`
			Src     string `json:"src"`
			Start   int    `json:"startFrame"`
			End     int    `json:"endFrame"`
		} `json:"scenes"`
	}
	if err := json.Unmarshal(updated, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Scenes[0].MediaID != "media-new" || manifest.Scenes[0].Src != out.Src || manifest.Scenes[0].Start != 0 || manifest.Scenes[0].End != 30 {
		t.Fatalf("manifest=%+v", manifest.Scenes[0])
	}
	if measured != 1 || out.Quality != "measuring" {
		t.Fatalf("measure=%d quality=%q", measured, out.Quality)
	}
}

// TestRebindWithoutProjectWriteLeavesProjectAlone: project=false thì không ghi
// project, nhưng vẫn nạp media và cập nhật manifest bước dựng.
func TestRebindWithoutProjectWriteLeavesProjectAlone(t *testing.T) {
	dir := t.TempDir()
	imageFile := dir + "/2-x.webp"
	if err := os.WriteFile(imageFile, []byte("webp"), 0644); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"scenes": []any{map[string]any{"sceneId": "sc-0", "mediaId": "media-old"}}})
	steps := &fakeSteps{snapshots: map[domain.CloneStep]jobedit.Snapshot{
		domain.CloneBuild:  {Job: domain.CloneJob{ID: "job-1"}, Step: domain.CloneStepRecord{Step: domain.CloneBuild, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "clone-job-1-g1"}}}, Manifest: raw},
		domain.CloneImages: {Job: domain.CloneJob{ID: "job-1"}, Step: domain.CloneStepRecord{Step: domain.CloneImages}},
	}}
	project := &fakeProject{revision: "r1", mediaByID: map[string]editor.MediaRecord{}, project: editor.Project{"timeline": map[string]any{"items": []any{map[string]any{"id": "clone-job-1-g1-scene-sc-0", "mediaId": "media-old"}}}}}
	binder := &ProjectBinder{Edits: steps, Editor: project}
	out, err := binder.Rebind(context.Background(), RebindRequest{JobID: "job-1", SceneID: "sc-0", Kind: "scene", Version: 2, Path: imageFile, SHA256: "new", ApplyProject: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(project.commands) != 0 {
		t.Fatalf("project bị ghi dù project=false: %v", project.commands)
	}
	if project.imports != 1 || out == nil || out.MediaID != "media-new" || steps.commits != 1 {
		t.Fatalf("imports=%d out=%+v commits=%d", project.imports, out, steps.commits)
	}
}

// rebindWithMeasure chạy Rebind với Measure giả trên project một cảnh.
func rebindWithMeasure(t *testing.T, measure func(context.Context, string) error, grace time.Duration) (*RebindResult, error, *fakeSteps) {
	t.Helper()
	imageFile := t.TempDir() + "/2-x.webp"
	if err := os.WriteFile(imageFile, []byte("webp"), 0644); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"scenes": []any{map[string]any{"sceneId": "sc-0", "mediaId": "media-old"}}})
	steps := &fakeSteps{snapshots: map[domain.CloneStep]jobedit.Snapshot{
		domain.CloneBuild:  {Job: domain.CloneJob{ID: "job-1"}, Step: domain.CloneStepRecord{Step: domain.CloneBuild, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "clone-job-1-g1"}}}, Manifest: raw},
		domain.CloneImages: {Job: domain.CloneJob{ID: "job-1"}, Step: domain.CloneStepRecord{Step: domain.CloneImages}},
	}}
	project := &fakeProject{revision: "r1", mediaByID: map[string]editor.MediaRecord{}, project: editor.Project{"timeline": map[string]any{"items": []any{map[string]any{"id": "clone-job-1-g1-scene-sc-0", "mediaId": "media-old"}}}}}
	binder := &ProjectBinder{Edits: steps, Editor: project, Measure: measure, MeasureGrace: grace}
	out, err := binder.Rebind(context.Background(), RebindRequest{JobID: "job-1", SceneID: "sc-0", Kind: "scene", Version: 2, Path: imageFile, SHA256: "new", ApplyProject: true})
	return out, err, steps
}

// TestRebindKeepsResultWhenMeasureFails: media đã thay và manifest đã ghi thì
// kết quả thay phải về tới Editor dù đo Độ hoàn thiện lỗi (thiếu Chrome…) —
// nếu không clip không đổi ảnh, không có thông báo Hoàn tác.
func TestRebindKeepsResultWhenMeasureFails(t *testing.T) {
	out, err, steps := rebindWithMeasure(t, func(context.Context, string) error {
		return errors.New("Cài Chrome for Testing trong Cài đặt → Công cụ")
	}, time.Second)
	if err != nil {
		t.Fatalf("đo lỗi không được làm hỏng việc thay media: %v", err)
	}
	if out == nil || out.ClipID != "clone-job-1-g1-scene-sc-0" || out.MediaID != "media-new" || out.PreviousMediaID != "media-old" || out.Src == "" {
		t.Fatalf("out=%+v", out)
	}
	if out.Quality != "unavailable" || steps.commits != 1 {
		t.Fatalf("quality=%q commits=%d", out.Quality, steps.commits)
	}
}

// TestRebindDoesNotWaitForSlowMeasure: đo chạy nền; request thay ảnh trả về
// sau khoảng chờ ngắn, không đợi Chrome đo xong.
func TestRebindDoesNotWaitForSlowMeasure(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{}, 1)
	begin := time.Now()
	out, err, _ := rebindWithMeasure(t, func(ctx context.Context, _ string) error {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Fatalf("Rebind chờ đo %s", elapsed)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("đo chưa được khởi động")
	}
	if out == nil || out.MediaID != "media-new" || out.Quality != "measuring" {
		t.Fatalf("out=%+v", out)
	}
}

// TestRebindMeasureOutlivesRequest: lần đo chạy nền không bị huỷ khi request kết thúc.
func TestRebindMeasureOutlivesRequest(t *testing.T) {
	imageFile := t.TempDir() + "/2-x.webp"
	if err := os.WriteFile(imageFile, []byte("webp"), 0644); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"scenes": []any{map[string]any{"sceneId": "sc-0", "mediaId": "media-old"}}})
	steps := &fakeSteps{snapshots: map[domain.CloneStep]jobedit.Snapshot{
		domain.CloneBuild:  {Job: domain.CloneJob{ID: "job-1"}, Step: domain.CloneStepRecord{Step: domain.CloneBuild, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "clone-job-1-g1"}}}, Manifest: raw},
		domain.CloneImages: {Job: domain.CloneJob{ID: "job-1"}, Step: domain.CloneStepRecord{Step: domain.CloneImages}},
	}}
	project := &fakeProject{revision: "r1", mediaByID: map[string]editor.MediaRecord{}, project: editor.Project{"timeline": map[string]any{"items": []any{map[string]any{"id": "clone-job-1-g1-scene-sc-0", "mediaId": "media-old"}}}}}
	alive := make(chan error, 1)
	release := make(chan struct{})
	binder := &ProjectBinder{Edits: steps, Editor: project, MeasureGrace: 10 * time.Millisecond, Measure: func(ctx context.Context, _ string) error {
		<-release
		alive <- ctx.Err()
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := binder.Rebind(ctx, RebindRequest{JobID: "job-1", SceneID: "sc-0", Kind: "scene", Version: 2, Path: imageFile, SHA256: "new", ApplyProject: true}); err != nil {
		t.Fatal(err)
	}
	cancel()
	close(release)
	select {
	case err := <-alive:
		if err != nil {
			t.Fatalf("đo nền bị huỷ theo request: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("đo nền không chạy tiếp")
	}
}

type conflictingRebindSteps struct {
	*fakeSteps
	latest jobedit.Snapshot
	calls  int
	always bool
}

func (s *conflictingRebindSteps) CommitBuildMedia(_ context.Context, _ jobedit.Snapshot, _ any, artifacts []media.Artifact) error {
	s.calls++
	if s.calls == 1 || s.always {
		s.snapshots[domain.CloneBuild] = s.latest
		return jobedit.Conflict("concurrent build publication")
	}
	return s.commit(artifacts)
}

func TestRebindRetriesOnlyIndependentManifestPublication(t *testing.T) {
	for _, name := range []string{"other-scene", "same-scene-newer-version", "worker-fence", "generation", "bounded-conflicts"} {
		t.Run(name, func(t *testing.T) {
			file := t.TempDir() + "/version.png"
			if err := os.WriteFile(file, []byte("image"), 0600); err != nil {
				t.Fatal(err)
			}
			build := jobedit.Snapshot{
				Job: domain.CloneJob{ID: "job", Generation: 1},
				Step: domain.CloneStepRecord{Step: domain.CloneBuild, Status: domain.CloneSucceeded, Checkpoint: json.RawMessage(`{}`),
					Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "project"}}},
				Manifest: json.RawMessage(`{"scenes":[{"sceneId":"a","mediaId":"old-a"},{"sceneId":"b","mediaId":"old-b"}]}`),
			}
			latest := build
			latest.Manifest = json.RawMessage(`{"scenes":[{"sceneId":"a","mediaId":"old-a"},{"sceneId":"b","mediaId":"new-b"}]}`)
			switch name {
			case "same-scene-newer-version":
				latest.Manifest = json.RawMessage(`{"scenes":[{"sceneId":"a","mediaId":"newer-a"},{"sceneId":"b","mediaId":"old-b"}]}`)
			case "worker-fence":
				latest.Step.Fence++
			case "generation":
				latest.Job.Generation++
			}
			steps := &conflictingRebindSteps{fakeSteps: &fakeSteps{snapshots: map[domain.CloneStep]jobedit.Snapshot{
				domain.CloneBuild: build, domain.CloneImages: {},
			}}, latest: latest, always: name == "bounded-conflicts"}
			project := &fakeProject{mediaByID: map[string]editor.MediaRecord{}, project: editor.Project{
				"timeline": editor.Object{"items": []any{editor.Object{"id": "project-scene-a", "mediaId": "old-a"}}},
			}}
			binder := &ProjectBinder{Edits: steps, Editor: project}
			_, err := binder.Rebind(context.Background(), RebindRequest{JobID: "job", SceneID: "a", Kind: "scene", Version: 2, Path: file, SHA256: "new", ApplyProject: true})
			if name == "other-scene" {
				if err != nil || steps.calls != 2 || steps.commits != 1 {
					t.Fatalf("independent publication was not merged: calls=%d commits=%d %v", steps.calls, steps.commits, err)
				}
				var manifest struct {
					Scenes []struct {
						MediaID string `json:"mediaId"`
					} `json:"scenes"`
				}
				if err := json.Unmarshal([]byte(steps.committed[0].File), &manifest); err != nil || len(manifest.Scenes) != 2 || manifest.Scenes[0].MediaID != "media-new" || manifest.Scenes[1].MediaID != "new-b" {
					t.Fatalf("independent binding lost: %+v %v", manifest, err)
				}
			} else {
				requireCode(t, err, domain.CloneErrConflict)
				wantCalls := 1
				if name == "bounded-conflicts" {
					wantCalls = 3
				}
				if steps.calls != wantCalls || steps.commits != 0 {
					t.Fatalf("stale publication retried incorrectly: calls=%d commits=%d", steps.calls, steps.commits)
				}
			}
			if project.imports != 1 || len(project.commands) != 1 {
				t.Fatalf("manifest retry repeated side effects: imports=%d commands=%d", project.imports, len(project.commands))
			}
		})
	}
}
