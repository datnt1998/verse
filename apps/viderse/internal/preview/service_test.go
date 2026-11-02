package preview

// Test hợp đồng bản xem thử ở ranh giới nghiệp vụ: nguồn chỉ là video đã xong
// và còn hiệu lực, số cảnh đúng bằng số ảnh recipe tạo ra (đọc từ project được
// đổ, không phải đếm lại công thức), cận cảnh dùng framing của recipe, lỗi
// dựng giữ câu hành động và không mất bản xem thử cũ, và Start/Cancel không
// giẫm lên dòng việc của lượt khác.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// ── fake store/service ─────────────────────────────────────────────────────

type fakeTemplates struct {
	mu       sync.Mutex
	detail   *library.Detail
	begun    []string
	cleared  []string
	record   *library.TemplatePreviewRecord
	failure  *library.PreviewError
	projects []string
	forgot   []string
	rows     []library.PreviewProjectRow
	recorded chan library.TemplatePreviewRecord
	failed   chan *library.PreviewError
}

func newFakeTemplates(detail *library.Detail) *fakeTemplates {
	return &fakeTemplates{detail: detail, recorded: make(chan library.TemplatePreviewRecord, 4), failed: make(chan *library.PreviewError, 4)}
}

// Get trả snapshot như library.Get trên Postgres: mỗi lần gọi một DTO riêng,
// không chia sẻ con trỏ với build đang chạy (test race ở bản trước do fake sửa
// f.detail trong RecordTemplatePreview, không phải service).
func (f *fakeTemplates) Get(context.Context, string) (*library.Detail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.detail == nil {
		return nil, library.ErrNotFound
	}
	snapshot := *f.detail
	snapshot.Project = append([]byte(nil), f.detail.Project...)
	return &snapshot, nil
}

func (f *fakeTemplates) BeginTemplatePreview(_ context.Context, _, runID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.begun = append(f.begun, runID)
	return nil
}

func (f *fakeTemplates) RecordTemplatePreview(_ context.Context, _ string, record library.TemplatePreviewRecord) (string, error) {
	f.mu.Lock()
	f.record = &record
	f.mu.Unlock()
	select {
	case f.recorded <- record:
	default:
	}
	return "", nil
}

func (f *fakeTemplates) FailTemplatePreview(_ context.Context, _, _ string, failure *library.PreviewError) error {
	f.mu.Lock()
	f.failure = failure
	f.mu.Unlock()
	select {
	case f.failed <- failure:
	default:
	}
	return nil
}

func (f *fakeTemplates) ClearTemplatePreviewRun(_ context.Context, _, runID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleared = append(f.cleared, runID)
	return nil
}

func (f *fakeTemplates) RegisterTemplatePreviewProject(_ context.Context, _, projectID, _ string, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projects = append(f.projects, projectID)
	return nil
}

func (f *fakeTemplates) PreviewProjectIDsForTemplate(context.Context, string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.projects...), nil
}

func (f *fakeTemplates) ForgetPreviewProject(_ context.Context, projectID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgot = append(f.forgot, projectID)
	return nil
}

func (f *fakeTemplates) PreviewProjectRows(context.Context) ([]library.PreviewProjectRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]library.PreviewProjectRow{}, f.rows...), nil
}

type fakeJobs struct {
	list domain.CloneJobList
	// supersedeOnGet > 0: lần Get thứ n trở đi trả job đã bị thay.
	supersedeOnGet int
	gets           int
	mu             sync.Mutex
}

func (f *fakeJobs) List(context.Context, *string, int) (domain.CloneJobList, error) {
	return f.list, nil
}

func (f *fakeJobs) Get(_ context.Context, id string) (*domain.CloneJobView, error) {
	f.mu.Lock()
	f.gets++
	supersede := f.supersedeOnGet > 0 && f.gets >= f.supersedeOnGet
	f.mu.Unlock()
	for i := range f.list.Jobs {
		job := f.list.Jobs[i]
		if job.ID != id {
			continue
		}
		if supersede {
			now := time.Now()
			job.SupersededAt = &now
		}
		return &job, nil
	}
	return nil, nil
}

type fakeMedia struct {
	mu    sync.Mutex
	files map[string]string
	// fail maps logical path → lỗi đọc (mô phỏng blob hỏng/mất).
	fail map[string]bool
}

func (m *fakeMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail[path] {
		return nil, errors.New("blob hỏng")
	}
	file, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &media.Handle{Path: file}, nil
}

type fakeEditor struct {
	mu           sync.Mutex
	commands     []editor.Command
	deleted      []string
	imports      []editor.ImportFileOnceOptions
	nextID       int
	failCommands bool
}

func (e *fakeEditor) LoadProject(context.Context, string) (editor.Project, string, error) {
	return nil, "", errors.New("không dùng")
}

func (e *fakeEditor) DuplicateProjectAs(_ context.Context, _, _, newID string) (editor.Project, string, error) {
	return editor.Project{"id": newID}, "sha256:" + newID, nil
}

func (e *fakeEditor) RunCommands(_ context.Context, _ string, commands []editor.Command, options editor.ApplyCommandsOptions) (editor.Object, error) {
	if options.DryRun {
		return editor.Object{}, nil
	}
	if e.failCommands {
		return nil, &library.Error{Code: library.CodeConflict, Message: "Không đổ được project xem thử", Recovery: "Dựng lại video nguồn rồi thử lại"}
	}
	e.mu.Lock()
	e.commands = append([]editor.Command{}, commands...)
	e.mu.Unlock()
	return editor.Object{}, nil
}

func (e *fakeEditor) DeleteProject(_ context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.deleted = append(e.deleted, id)
	return nil
}

func (e *fakeEditor) ImportFileOnce(_ context.Context, options editor.ImportFileOnceOptions) (editor.MediaRecord, bool, error) {
	e.mu.Lock()
	e.nextID++
	id := fmt.Sprintf("media-%d", e.nextID)
	e.imports = append(e.imports, options)
	e.mu.Unlock()
	record := editor.MediaRecord{ID: id, FileName: options.FileName, MIME: options.DeclaredMIME, ContentHash: options.ContentHash, CreatedAt: 1}
	if options.MIMEPrefix == "audio/" {
		duration := 180.0
		record.Probe = editor.ProbeResult{Kind: "audio", DurationSec: &duration}
	} else {
		width, height := 10.0, 10.0
		record.Probe = editor.ProbeResult{Kind: "image", Width: &width, Height: &height}
	}
	return record, true, nil
}

type fakeRunner struct {
	mu      sync.Mutex
	block   chan struct{} // non-nil: chờ đóng/gửi trước khi chạy
	calls   int
	onBlock func()
}

func (r *fakeRunner) Run(ctx context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
	r.mu.Lock()
	r.calls++
	block := r.block
	if block != nil && r.onBlock != nil {
		r.onBlock()
	}
	r.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return tools.Result{}, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return tools.Result{}, err
	}
	output := args[len(args)-1]
	if len(args) >= 2 && args[len(args)-2] == "f32le" {
		// 48 000 khung stereo f32 = 8 byte/khung.
		return tools.Result{}, os.WriteFile(output, make([]byte, 48000*8), 0o644)
	}
	input := ""
	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			input = args[i+1]
		}
	}
	if input == "" {
		return tools.Result{}, errors.New("thiếu input")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return tools.Result{}, err
	}
	return tools.Result{}, os.WriteFile(output, data, 0o644)
}

type fakeTasks struct {
	mu       sync.Mutex
	active   map[string]*activity.Task
	nextID   int
	started  []string
	finished []string
	updates  []domain.TaskUpdate
}

func newFakeTasks() *fakeTasks { return &fakeTasks{active: map[string]*activity.Task{}} }

func (t *fakeTasks) Start(_ context.Context, spec domain.TaskSpec) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if row, ok := t.active[spec.Object.ID]; ok {
		return row.ID, nil
	}
	t.nextID++
	id := fmt.Sprintf("task-%d", t.nextID)
	t.active[spec.Object.ID] = &activity.Task{ID: id, Kind: spec.Kind, Object: spec.Object, Phase: spec.Phase, State: domain.TaskRunning, Action: spec.Action}
	t.started = append(t.started, id)
	return id, nil
}

func (t *fakeTasks) Update(_ context.Context, id string, update domain.TaskUpdate) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, row := range t.active {
		if row.ID != id {
			continue
		}
		if update.Phase != nil {
			row.Phase = *update.Phase
		}
		if update.State != "" {
			row.State = update.State
		}
		if update.Progress != nil {
			row.Progress = update.Progress
		}
		t.updates = append(t.updates, update)
	}
	return nil
}

func (t *fakeTasks) Finish(_ context.Context, id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, row := range t.active {
		if row.ID == id {
			delete(t.active, key)
			t.finished = append(t.finished, id)
		}
	}
	return nil
}

func (t *fakeTasks) FindTaskByObject(_ context.Context, _ domain.TaskKind, object domain.ObjectRef) (*activity.Task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.active[object.ID]
	if !ok {
		return nil, nil
	}
	copy := *row
	return &copy, nil
}

// ── fixture ────────────────────────────────────────────────────────────────

type fixture struct {
	detail  *library.Detail
	jobs    *fakeJobs
	media   *fakeMedia
	editor  *fakeEditor
	runner  *fakeRunner
	tasks   *fakeTasks
	tmpl    *fakeTemplates
	cleanup func()
}

func sha256Of(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func pngBytes(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if x >= size/4 && x < size*3/4 && y >= size/4 && y < size*3/4 {
				img.Set(x, y, color.NRGBA{A: 255})
			}
		}
	}
	var buf []byte
	writer := &sliceWriter{data: &buf}
	if err := png.Encode(writer, img); err != nil {
		t.Fatal(err)
	}
	return buf
}

type sliceWriter struct{ data *[]byte }

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}

func templateProject(t *testing.T) editor.Project {
	t.Helper()
	raw, err := os.ReadFile("../library/testdata/ytf-project-standard.json")
	if err != nil {
		t.Fatal(err)
	}
	project, err := editor.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return project
}

type fixtureOptions struct {
	recipe       domain.Recipe
	closeUp      bool
	durationSec  float64
	minutes      int
	blockRunner  bool
	failCommands bool
	corruptImage bool
	supersedeGet int
}

func newFixture(t *testing.T, options fixtureOptions) *fixture {
	t.Helper()
	dir := t.TempDir()
	mediaFiles := map[string]string{}
	steps := []domain.CloneStepRecord{}
	addArtifact := func(step domain.CloneStep, rel string, data []byte) {
		path := "clone/job-1/x/" + rel
		file := writeFile(t, dir, rel, data)
		mediaFiles[path] = file
		hash := sha256Of(data)
		art := domain.CloneArtifact{ID: rel, Path: path, SHA256: hash, Bytes: int64(len(data)), Step: step, ContentType: "application/octet-stream", Filename: filepath.Base(rel)}
		found := false
		for i := range steps {
			if steps[i].Step == step {
				steps[i].Output.Artifacts = append(steps[i].Output.Artifacts, art)
				found = true
			}
		}
		if !found {
			steps = append(steps, domain.CloneStepRecord{JobID: "job-1", Step: step, Status: domain.CloneSucceeded, Output: &domain.CloneStepOutput{Artifacts: []domain.CloneArtifact{art}}})
		}
	}
	// scenes step: duration trong Data (build đọc nhanh, không mở manifest).
	steps = append(steps, domain.CloneStepRecord{
		JobID: "job-1", Step: domain.CloneScenes, Status: domain.CloneSucceeded,
		Output: &domain.CloneStepOutput{Data: map[string]any{"durationSec": options.durationSec}},
	})
	scenesManifest, err := json.Marshal(scenes.Manifest{DurationSec: options.durationSec})
	if err != nil {
		t.Fatal(err)
	}
	addArtifact(domain.CloneScenes, "scenes/manifest.json", scenesManifest)
	scenePNG := pngBytes(t, 24)
	cutPNG := pngBytes(t, 24)
	scenePath := writeFile(t, dir, "source-scene.png", scenePNG)
	cutPath := writeFile(t, dir, "source-cut.png", cutPNG)
	sceneHash := sha256Of(scenePNG)
	cutHash := sha256Of(cutPNG)
	mediaFiles["clone/job-1/"+sceneHash+"/images/scene.png"] = scenePath
	mediaFiles["clone/job-1/"+cutHash+"/images/scene-cut.png"] = cutPath

	manifest := images.Manifest{Version: 1, Revision: 1}
	if options.closeUp {
		manifest.Style = "cutout"
	}
	sceneItem := images.Item{ID: "scene-1", Kind: "scene", Width: 24, Height: 24, Reference: scenes.FileRef{Path: "images/ref.png", SHA256: sceneHash}, Narration: "kể", Prompt: "vẽ", PromptVersion: 1, Status: "approved"}
	sceneItem.Versions = []images.Version{{Version: 1, SourceFingerprint: images.SourceFingerprint(sceneItem), Path: "images/scene.png", SHA256: sceneHash, Mime: "image/png", Width: 24, Height: 24}}
	if options.closeUp {
		sceneItem.Versions[0].Cutout = &scenes.FileRef{Path: "images/scene-cut.png", SHA256: cutHash}
	}
	sceneItem.CurrentVersion = new(1)
	coverItem := images.Item{ID: "cover", Kind: "thumbnail", Width: 24, Height: 24, Reference: scenes.FileRef{Path: "images/cover.png", SHA256: sceneHash}, Narration: "bìa", Prompt: "bìa", PromptVersion: 1, Status: "approved"}
	coverItem.Versions = []images.Version{{Version: 1, SourceFingerprint: images.SourceFingerprint(coverItem), Path: "images/scene.png", SHA256: sceneHash, Mime: "image/png", Width: 24, Height: 24}}
	coverItem.CurrentVersion = new(1)
	manifest.Items = []images.Item{coverItem, sceneItem}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if options.corruptImage {
		// Artifact ghi hash khác nội dung: preflight phải báo lỗi, không bỏ qua.
		addArtifact(domain.CloneImages, "images/manifest.json", manifestData)
		for i := range steps {
			if steps[i].Step == domain.CloneImages {
				steps[i].Output.Artifacts[0].SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
			}
		}
	} else {
		addArtifact(domain.CloneImages, "images/manifest.json", manifestData)
	}
	captions, err := json.Marshal(transcript.CaptionManifest{Cues: []transcript.Cue{{ID: "c1", StartMs: 1000, EndMs: new(2000.0), Text: "xin chào"}}})
	if err != nil {
		t.Fatal(err)
	}
	addArtifact(domain.CloneTranscript, "transcript/captions.json", captions)
	audio := []byte("nguồn âm thanh")
	addArtifact(domain.CloneBuild, "build/source-audio.m4a", audio)

	project := templateProject(t)
	if options.closeUp {
		for _, value := range project.Timeline()["tracks"].([]any) {
			track, _ := value.(map[string]any)
			if track["templateSlot"] == "scenes" {
				track["templateScenes"] = map[string]any{
					"style":  "cutout-close",
					"images": map[string]any{"mode": options.recipe.Scenes.Images.Mode, "count": options.recipe.Scenes.Images.Count, "minutes": options.recipe.Scenes.Images.Minutes},
					"motion": map[string]any{"kind": "ken-burns", "intensity": 1, "speed": 1},
				}
			}
		}
	}
	rawProject, err := json.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	detail := &library.Detail{}
	detail.Template = domain.Template{ID: "tpl-1", Name: "Mẫu thử", Revision: 3, Project: rawProject, Recipe: options.recipe, PreviewProjectID: "old-project"}
	job := domain.CloneJobView{CloneJob: domain.CloneJob{ID: "job-1", Status: domain.CloneSucceeded, Source: domain.CloneSourceSnapshot{Title: "Video nguồn", DurationSec: &options.durationSec}}, Steps: steps}

	runner := &fakeRunner{}
	if options.blockRunner {
		runner.block = make(chan struct{})
	}
	editorFake := &fakeEditor{failCommands: options.failCommands}
	return &fixture{
		detail: detail,
		jobs:   &fakeJobs{list: domain.CloneJobList{Jobs: []domain.CloneJobView{job}}, supersedeOnGet: options.supersedeGet},
		media:  &fakeMedia{files: mediaFiles, fail: map[string]bool{}},
		editor: editorFake,
		runner: runner,
		tasks:  newFakeTasks(),
		tmpl:   newFakeTemplates(detail),
		cleanup: func() {
			if runner.block != nil {
				close(runner.block)
			}
		},
	}
}

func (f *fixture) service() *Service {
	return New(Options{
		Templates: f.tmpl,
		Jobs:      f.jobs,
		Editor:    f.editor,
		Media:     f.media,
		Runner:    f.runner,
		Ffmpeg:    func(context.Context) (string, bool) { return "/usr/bin/ffmpeg", true },
		Tasks:     f.tasks,
		Now:       func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) },
	})
}

func sceneImageItems(t *testing.T, commands []editor.Command) int {
	t.Helper()
	trackID := ""
	project := templateProject(t)
	for _, value := range project.Timeline()["tracks"].([]any) {
		track, _ := value.(map[string]any)
		if track["templateSlot"] == "scenes" {
			trackID, _ = track["id"].(string)
		}
	}
	count := 0
	for _, command := range commands {
		if command.Type != "item.add" {
			continue
		}
		item, _ := command.Params["item"].(editor.Object)
		if item == nil {
			continue
		}
		if item["type"] == "image" && item["trackId"] == trackID {
			count++
		}
	}
	return count
}

func waitRecord(t *testing.T, f *fixture) library.TemplatePreviewRecord {
	t.Helper()
	select {
	case record := <-f.tmpl.recorded:
		return record
	case failure := <-f.tmpl.failed:
		t.Fatalf("dựng hỏng: %+v", failure)
	case <-time.After(10 * time.Second):
		t.Fatal("hết giờ chờ dựng xong")
	}
	return library.TemplatePreviewRecord{}
}

func waitFailure(t *testing.T, f *fixture) *library.PreviewError {
	t.Helper()
	select {
	case failure := <-f.tmpl.failed:
		return failure
	case record := <-f.tmpl.recorded:
		t.Fatalf("đã công bố: %+v", record)
	case <-time.After(10 * time.Second):
		t.Fatal("hết giờ chờ lỗi")
	}
	return nil
}

// ── sources ────────────────────────────────────────────────────────────────

func fullyBuiltSteps(jobID string) []domain.CloneStepRecord {
	artifacts := func(step domain.CloneStep, paths ...string) domain.CloneStepRecord {
		out := domain.CloneStepRecord{JobID: jobID, Step: step, Status: domain.CloneSucceeded, Output: &domain.CloneStepOutput{}}
		for _, path := range paths {
			out.Output.Artifacts = append(out.Output.Artifacts, domain.CloneArtifact{Path: "clone/" + jobID + "/x/" + path})
		}
		return out
	}
	scenesStep := artifacts(domain.CloneScenes, "scenes/manifest.json")
	scenesStep.Output.Data = map[string]any{"durationSec": 90.0}
	return []domain.CloneStepRecord{
		scenesStep,
		artifacts(domain.CloneImages, "images/manifest.json"),
		artifacts(domain.CloneTranscript, "transcript/captions.json"),
		artifacts(domain.CloneBuild, "build/source-audio.m4a"),
	}
}

func TestSourcesOnlyFinishedCurrentJobsWithArtifacts(t *testing.T) {
	superseded := time.Now()
	jobs := &fakeJobs{list: domain.CloneJobList{Jobs: []domain.CloneJobView{
		{CloneJob: domain.CloneJob{ID: "done", Status: domain.CloneSucceeded, Source: domain.CloneSourceSnapshot{Title: "Video xong"}}, Steps: fullyBuiltSteps("done")},
		{CloneJob: domain.CloneJob{ID: "old", Status: domain.CloneSucceeded, SupersededAt: &superseded, Source: domain.CloneSourceSnapshot{Title: "Video cũ"}}, Steps: fullyBuiltSteps("old")},
		{CloneJob: domain.CloneJob{ID: "failed", Status: domain.CloneFailed, Source: domain.CloneSourceSnapshot{Title: "Video lỗi"}}, Steps: fullyBuiltSteps("failed")},
		{CloneJob: domain.CloneJob{ID: "partial", Status: domain.CloneSucceeded, Source: domain.CloneSourceSnapshot{Title: "Thiếu artifact"}}, Steps: fullyBuiltSteps("partial")[:2]},
	}}}
	svc := New(Options{Jobs: jobs})
	sources, err := svc.Sources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].JobID != "done" || sources[0].DurationSec != 90 {
		t.Fatalf("sources = %+v", sources)
	}
}

func TestSourcesDisablesNoImagesJobForImageRecipe(t *testing.T) {
	jobs := &fakeJobs{list: domain.CloneJobList{Jobs: []domain.CloneJobView{
		{CloneJob: domain.CloneJob{ID: "done", Status: domain.CloneSucceeded, NoImages: true, Source: domain.CloneSourceSnapshot{Title: "Không ảnh"}}, Steps: fullyBuiltSteps("done")},
	}}}
	svc := New(Options{Jobs: jobs})
	recipe := domain.Recipe{Scenes: domain.ScenesRecipe{Images: domain.ImagesMode{Mode: library.ImagesFixed, Count: 5}}}
	sources, err := svc.sources(context.Background(), &recipe)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || !sources[0].Disabled || sources[0].Reason == "" {
		t.Fatalf("sources = %+v", sources)
	}
}

// ── start / preflight ──────────────────────────────────────────────────────

func TestStartWithoutFinishedVideoIsRealMissingSampleConflict(t *testing.T) {
	detail := &library.Detail{}
	detail.Template = domain.Template{ID: "tpl", Name: "Mẫu"}
	svc := New(Options{Templates: newFakeTemplates(detail), Jobs: &fakeJobs{}})
	_, err := svc.Start(context.Background(), "tpl", library.PreviewRequest{})
	if library.CodeOf(err) != library.CodeConflict {
		t.Fatalf("err = %v", err)
	}
}

func TestStartRejectsMinutesOutsideRange(t *testing.T) {
	svc := New(Options{Templates: newFakeTemplates(&library.Detail{})})
	if _, err := svc.Start(context.Background(), "tpl", library.PreviewRequest{Minutes: 11}); library.CodeOf(err) != library.CodeInvalidInput {
		t.Fatalf("11 phút: %v", err)
	}
	if _, err := svc.Start(context.Background(), "tpl", library.PreviewRequest{Minutes: -1}); library.CodeOf(err) != library.CodeInvalidInput {
		t.Fatalf("-1 phút: %v", err)
	}
}

func TestStartRejectsCorruptSourceBlobBefore202(t *testing.T) {
	f := newFixture(t, fixtureOptions{recipe: recipeFixed(1), durationSec: 180, corruptImage: true})
	defer f.cleanup()
	svc := f.service()
	_, err := svc.Start(context.Background(), "tpl-1", library.PreviewRequest{})
	if library.CodeOf(err) != library.CodeConflict {
		t.Fatalf("err = %v", err)
	}
	if len(f.tmpl.begun) != 0 || len(f.tasks.started) != 0 {
		t.Fatalf("đã nhận lượt dựng dù preflight hỏng: begun=%v tasks=%v", f.tmpl.begun, f.tasks.started)
	}
}

// ── build: số cảnh theo recipe ─────────────────────────────────────────────

func recipeFixed(count int) domain.Recipe {
	return domain.Recipe{Scenes: domain.ScenesRecipe{Style: library.StyleFull, Images: domain.ImagesMode{Mode: library.ImagesFixed, Count: count}}}
}

func recipePerMinutes(minutes int) domain.Recipe {
	return domain.Recipe{Scenes: domain.ScenesRecipe{Style: library.StyleFull, Images: domain.ImagesMode{Mode: library.ImagesPerMinutes, Minutes: minutes}}}
}

func recipeNone() domain.Recipe {
	return domain.Recipe{Scenes: domain.ScenesRecipe{Style: library.StyleFull, Images: domain.ImagesMode{Mode: library.ImagesNone}}}
}

func TestPreviewProjectHasRecipeSceneCount(t *testing.T) {
	cases := []struct {
		name    string
		recipe  domain.Recipe
		minutes int
		seconds float64
		want    int
	}{
		// 10 phút nguồn, "mỗi 2 phút 1 ảnh" → 5 cảnh (trước đây nhân thành 20).
		{"per-minutes", recipePerMinutes(2), 10, 600, 5},
		// fixed 1 không bị chặn sàn 3 cảnh.
		{"fixed-one", recipeFixed(1), 3, 180, 1},
		{"fixed-two", recipeFixed(2), 3, 180, 2},
		// không vẽ ảnh → 0 cảnh, không 409 và không ảnh.
		{"none", recipeNone(), 3, 180, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, fixtureOptions{recipe: tc.recipe, durationSec: tc.seconds, minutes: tc.minutes})
			defer f.cleanup()
			svc := f.service()
			if _, err := svc.Start(context.Background(), "tpl-1", library.PreviewRequest{Minutes: tc.minutes}); err != nil {
				t.Fatalf("start: %v", err)
			}
			waitRecord(t, f)
			editorFake := f.editor
			editorFake.mu.Lock()
			commands := append([]editor.Command{}, editorFake.commands...)
			editorFake.mu.Unlock()
			// Đếm scene item THẬT trong project được đổ: đây là số cảnh người
			// dùng thấy, không phải chép lại công thức ImageCountFor.
			if got := sceneImageItems(t, commands); got != tc.want {
				t.Fatalf("số cảnh trong project = %d, muốn %d", got, tc.want)
			}
			if tc.want == 0 {
				for _, imported := range f.editor.imports {
					if imported.MIMEPrefix == "image/" {
						t.Fatalf("recipe không vẽ ảnh nhưng đã nhập ảnh: %+v", imported)
					}
				}
			}
		})
	}
}

func TestPreviewCloseUpUsesRecipeFraming(t *testing.T) {
	recipe := recipeFixed(1)
	recipe.Scenes.Style = library.StyleCutout
	recipe.Scenes.Framing = library.FramingClose
	f := newFixture(t, fixtureOptions{recipe: recipe, closeUp: true, durationSec: 180, minutes: 3})
	defer f.cleanup()
	svc := f.service()
	if _, err := svc.Start(context.Background(), "tpl-1", library.PreviewRequest{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitRecord(t, f)
	f.editor.mu.Lock()
	commands := append([]editor.Command{}, f.editor.commands...)
	f.editor.mu.Unlock()
	if got := sceneImageItems(t, commands); got != 1 {
		t.Fatalf("số cảnh = %d", got)
	}
	// Cận cảnh phải có transform đã phóng (khác rect gốc 0/0/1920/1080).
	for _, command := range commands {
		if command.Type != "item.add" {
			continue
		}
		item, _ := command.Params["item"].(editor.Object)
		if item == nil || item["type"] != "image" {
			continue
		}
		transform, _ := item["transform"].(map[string]any)
		if transform == nil {
			t.Fatalf("thiếu transform: %+v", item)
		}
		if width, _ := transform["width"].(float64); width <= 1920 {
			t.Fatalf("cận cảnh chưa phóng khung (width=%v): %+v", width, transform)
		}
		return
	}
	t.Fatal("không có scene item")
}

// ── build failure: lỗi hành động, giữ bản cũ ───────────────────────────────

func TestBuildFailureStoresActionableErrorAndKeepsOldPreview(t *testing.T) {
	f := newFixture(t, fixtureOptions{recipe: recipeFixed(1), durationSec: 180, minutes: 3, failCommands: true})
	defer f.cleanup()
	svc := f.service()
	if _, err := svc.Start(context.Background(), "tpl-1", library.PreviewRequest{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	failure := waitFailure(t, f)
	if failure.Code != library.CodeConflict || failure.Message != "Không đổ được project xem thử" || failure.Recovery == "" {
		t.Fatalf("failure = %+v", failure)
	}
	if f.tmpl.record != nil {
		t.Fatalf("đã công bố dù dựng hỏng: %+v", f.tmpl.record)
	}
	if f.detail.PreviewProjectID != "old-project" {
		t.Fatalf("mất bản xem thử cũ: %q", f.detail.PreviewProjectID)
	}
	f.tasks.mu.Lock()
	foundFailed := false
	for _, update := range f.tasks.updates {
		if update.State == domain.TaskFailed && update.Phase != nil && *update.Phase == failure.Message {
			foundFailed = true
		}
	}
	f.tasks.mu.Unlock()
	if !foundFailed {
		t.Fatal("dòng việc không giữ câu lỗi hành động")
	}
}

func TestSupersededSourceDuringBuildDoesNotPublish(t *testing.T) {
	f := newFixture(t, fixtureOptions{recipe: recipeFixed(1), durationSec: 180, minutes: 3, supersedeGet: 2})
	defer f.cleanup()
	svc := f.service()
	if _, err := svc.Start(context.Background(), "tpl-1", library.PreviewRequest{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	failure := waitFailure(t, f)
	if failure.Code != library.CodeConflict || failure.Recovery == "" {
		t.Fatalf("failure = %+v", failure)
	}
	if f.tmpl.record != nil || f.detail.PreviewProjectID != "old-project" {
		t.Fatalf("công bố nhầm: record=%+v pointer=%q", f.tmpl.record, f.detail.PreviewProjectID)
	}
}

// ── lifecycle ──────────────────────────────────────────────────────────────

func TestStartIsSingleFlightPerTemplate(t *testing.T) {
	f := newFixture(t, fixtureOptions{recipe: recipeFixed(1), durationSec: 180, minutes: 3, blockRunner: true})
	defer f.cleanup()
	started := make(chan struct{}, 2)
	f.runner.onBlock = func() { started <- struct{}{} }
	svc := f.service()
	if _, err := svc.Start(context.Background(), "tpl-1", library.PreviewRequest{}); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	<-started
	status, err := svc.Start(context.Background(), "tpl-1", library.PreviewRequest{})
	if err != nil {
		t.Fatalf("start 2: %v", err)
	}
	if status.State != library.PreviewBuilding {
		t.Fatalf("state = %s", status.State)
	}
	if len(f.tmpl.begun) != 1 || len(f.tasks.started) != 1 {
		t.Fatalf("hai lượt dựng đã nhận: begun=%v tasks=%v", f.tmpl.begun, f.tasks.started)
	}
	if err := svc.Cancel(context.Background(), "tpl-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	// Cancel chờ lượt cũ dừng và nhả token + dòng việc trước khi trả.
	f.tmpl.mu.Lock()
	cleared := len(f.tmpl.cleared) == 1
	f.tmpl.mu.Unlock()
	f.tasks.mu.Lock()
	finished := len(f.tasks.finished) == 1
	f.tasks.mu.Unlock()
	if !cleared || !finished {
		t.Fatalf("cancel chưa dọn xong: cleared=%v finished=%v", cleared, finished)
	}
}

func TestCloseCancelsAndWaitsForBuild(t *testing.T) {
	f := newFixture(t, fixtureOptions{recipe: recipeFixed(1), durationSec: 180, minutes: 3, blockRunner: true})
	defer f.cleanup()
	started := make(chan struct{}, 1)
	f.runner.onBlock = func() { started <- struct{}{} }
	svc := f.service()
	if _, err := svc.Start(context.Background(), "tpl-1", library.PreviewRequest{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-f.tmpl.failed:
		t.Fatal("đóng service không phải lỗi dựng")
	default:
	}
}

// Reconcile sau crash: project của con trỏ hiện tại được giữ, project mồ côi
// (lượt dựng dở hoặc template đã xoá) bị xoá và bỏ dấu; dấu chưa dọn xong được
// giữ lại để lần khởi động sau thử tiếp.
func TestReconcileDropsOnlyOrphanPreviewProjects(t *testing.T) {
	f := newFixture(t, fixtureOptions{recipe: recipeFixed(1), durationSec: 180})
	defer f.cleanup()
	f.tmpl.rows = []library.PreviewProjectRow{
		{ProjectID: "old-project", TemplateID: "tpl-1", Current: true},
		{ProjectID: "crash-project", TemplateID: "tpl-1"},
		{ProjectID: "deleted-template-project"},
	}
	svc := f.service()
	if err := svc.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	f.editor.mu.Lock()
	deleted := append([]string{}, f.editor.deleted...)
	f.editor.mu.Unlock()
	f.tmpl.mu.Lock()
	forgot := append([]string{}, f.tmpl.forgot...)
	f.tmpl.mu.Unlock()
	if len(deleted) != 2 || deleted[0] != "crash-project" || deleted[1] != "deleted-template-project" {
		t.Fatalf("xoá = %v", deleted)
	}
	if len(forgot) != 2 {
		t.Fatalf("bỏ dấu = %v", forgot)
	}
}

// ── helpers cũ ─────────────────────────────────────────────────────────────

func TestClippedCuesDropAfterWindowAndClipWords(t *testing.T) {
	end := 10_000.0
	cues := []transcript.Cue{
		{ID: "a", StartMs: 1_000, EndMs: &end, Text: "trong cửa sổ", Words: []transcript.Word{{StartMs: 9_500, EndMs: 12_000, Text: "cuối"}}},
		{ID: "b", StartMs: 11_000, EndMs: &end, Text: "ngoài cửa sổ"},
		{ID: "c", StartMs: 2_000, Text: "không có end"},
	}
	out := clippedCues(cues, 10)
	if len(out) != 1 || out[0].ID != "a" {
		t.Fatalf("cues = %+v", out)
	}
	if out[0].EndSec != 10 || len(out[0].Words) != 1 || out[0].Words[0].EndMs != 10_000 {
		t.Fatalf("clipped = %+v", out[0])
	}
}

// ensure pipeline import used (PreviewSourceArtifacts list is the source of
// truth for fallback listing).
var _ = pipeline.PreviewSourceArtifacts
