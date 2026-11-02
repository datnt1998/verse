package covertrial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

// ── fakes ──────────────────────────────────────────────────────────────────

type fakeStore struct {
	mu      sync.Mutex
	trials  []library.CoverTrial
	samples []library.PromptSample
	unbind  []string

	// insertEntered/insertGate giữ lần ghi hàng đầu tiên để test hai cú bấm
	// chồng nhau thấy đúng cửa sổ tranh chấp.
	insertEntered chan struct{}
	insertGate    chan struct{}
	insertOnce    sync.Once
}

func (f *fakeStore) ListCoverTrials(_ context.Context, templateID string, limit int) ([]library.CoverTrial, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []library.CoverTrial{}
	for i := len(f.trials) - 1; i >= 0 && len(out) < limit; i-- {
		if f.trials[i].TemplateID == templateID {
			out = append(out, f.trials[i])
		}
	}
	return out, nil
}

func (f *fakeStore) GetCoverTrial(_ context.Context, templateID, id string) (*library.CoverTrial, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.trials {
		if f.trials[i].ID == id && f.trials[i].TemplateID == templateID {
			row := f.trials[i]
			return &row, nil
		}
	}
	return nil, library.ErrNotFound
}

func (f *fakeStore) InsertCoverTrial(ctx context.Context, trial library.CoverTrial) error {
	if f.insertEntered != nil {
		f.insertOnce.Do(func() { close(f.insertEntered) })
		select {
		case <-f.insertGate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trials = append(f.trials, trial)
	return nil
}

func (f *fakeStore) UpdateCoverTrial(_ context.Context, trial library.CoverTrial) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.trials {
		if f.trials[i].ID == trial.ID {
			f.trials[i] = trial
			return nil
		}
	}
	return library.ErrNotFound
}

func (f *fakeStore) CoverTrialsBeyond(_ context.Context, templateID string, keep int) ([]library.CoverTrial, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rows := []library.CoverTrial{}
	for _, row := range f.trials {
		if row.TemplateID == templateID {
			rows = append(rows, row)
		}
	}
	if len(rows) <= keep {
		return nil, nil
	}
	return rows[:len(rows)-keep], nil
}

func (f *fakeStore) DeleteCoverTrials(_ context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	kept := f.trials[:0]
	for _, row := range f.trials {
		if !drop[row.ID] {
			kept = append(kept, row)
		}
	}
	f.trials = kept
	return nil
}

func (f *fakeStore) CoverTrialMediaPaths(_ context.Context, templateID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []string{}
	for _, row := range f.trials {
		if row.TemplateID == templateID && row.MediaPath != "" {
			out = append(out, row.MediaPath)
		}
	}
	return out, nil
}

func (f *fakeStore) FailRunningCoverTrials(context.Context, time.Time, string, string) (int64, error) {
	return 0, nil
}

func (f *fakeStore) ListPromptSamples(_ context.Context, promptID string, limit int) ([]library.PromptSample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []library.PromptSample{}
	for _, row := range f.samples {
		if row.PromptID == promptID && len(out) < limit {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeStore) GetPromptSample(_ context.Context, promptID, id string) (*library.PromptSample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.samples {
		if f.samples[i].ID == id && f.samples[i].PromptID == promptID {
			row := f.samples[i]
			return &row, nil
		}
	}
	return nil, library.ErrNotFound
}

func (f *fakeStore) InsertPromptSample(_ context.Context, sample library.PromptSample) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.samples = append(f.samples, sample)
	return nil
}

func (f *fakeStore) UpdatePromptSample(_ context.Context, sample library.PromptSample) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.samples {
		if f.samples[i].ID == sample.ID {
			f.samples[i] = sample
			return nil
		}
	}
	return library.ErrNotFound
}

func (f *fakeStore) PromptSamplesBeyond(context.Context, string, int) ([]library.PromptSample, error) {
	return nil, nil
}
func (f *fakeStore) DeletePromptSamples(context.Context, []string) error { return nil }
func (f *fakeStore) PromptSampleMediaPaths(context.Context, string) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) FailRunningPromptSamples(context.Context, time.Time, string, string) (int64, error) {
	return 0, nil
}
func (f *fakeStore) ListPromptSampleIDs(context.Context, []string) (map[string][]string, error) {
	return map[string][]string{}, nil
}

type fakeMedia struct {
	mu      sync.Mutex
	files   map[string]string
	blobs   map[string]string
	unbound []string

	// publishEntered/publishGate giữ transaction bind để test xoá template
	// trong lúc lần vẽ đang publish.
	publishEntered chan struct{}
	publishGate    chan struct{}
	publishOnce    sync.Once
}

func (f *fakeMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	local, ok := f.files[path]
	if !ok {
		return nil, errors.New("thiếu artifact " + path)
	}
	return &media.Handle{Path: local}, nil
}

func (f *fakeMedia) Publish(ctx context.Context, artifacts []media.Artifact, commit func(context.Context) error) error {
	if f.publishEntered != nil {
		f.publishOnce.Do(func() { close(f.publishEntered) })
		select {
		case <-f.publishGate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	for _, artifact := range artifacts {
		data, err := os.ReadFile(artifact.File)
		if err != nil {
			f.mu.Unlock()
			return err
		}
		blob := filepath.Join(os.TempDir(), "covertrial-blob-"+filepath.Base(artifact.Path))
		if err := os.WriteFile(blob, data, 0o600); err != nil {
			f.mu.Unlock()
			return err
		}
		f.blobs[artifact.Path] = blob
	}
	f.mu.Unlock()
	return commit(ctx)
}

func (f *fakeMedia) Unbind(_ context.Context, paths []string, commit func(context.Context) error) error {
	f.mu.Lock()
	f.unbound = append(f.unbound, paths...)
	f.mu.Unlock()
	if commit != nil {
		return commit(context.Background())
	}
	return nil
}

type fakeDrawer struct {
	mu       sync.Mutex
	requests []imagegen.Request
	block    bool
	// hold chờ release rồi vẫn trả THÀNH CÔNG dù ctx đã bị huỷ — provider
	// không tôn trọng huỷ, đúng ca sinh "late success".
	hold    bool
	release chan struct{}
	started chan struct{}
	once    sync.Once
}

func (f *fakeDrawer) signalStarted() {
	if f.started == nil {
		return
	}
	f.once.Do(func() { close(f.started) })
}

func (f *fakeDrawer) Draw(ctx context.Context, req imagegen.Request) (imagegen.Receipt, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	if f.block {
		f.signalStarted()
		<-ctx.Done()
		return imagegen.Receipt{}, ctx.Err()
	}
	if f.hold {
		f.signalStarted()
		<-f.release
	}
	if err := writePNG(req.Output); err != nil {
		return imagegen.Receipt{}, err
	}
	if req.OnPhase != nil {
		req.OnPhase("sending")
		req.OnPhase("drawing")
	}
	return imagegen.Receipt{Provider: "chatgpt", Model: "gpt-image-1.5"}, nil
}

func (f *fakeDrawer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

type fakeTasks struct{}

func (fakeTasks) Start(context.Context, domain.TaskSpec) (string, error)  { return "task-1", nil }
func (fakeTasks) Update(context.Context, string, domain.TaskUpdate) error { return nil }
func (fakeTasks) Finish(context.Context, string) error                    { return nil }
func (fakeTasks) FindTaskByObject(context.Context, domain.TaskKind, domain.ObjectRef) (*activity.Task, error) {
	return nil, nil
}

type fakeTemplates struct{ detail *library.Detail }

func (f fakeTemplates) Get(context.Context, string) (*library.Detail, error) { return f.detail, nil }

type fakePrompts struct{ prompt *domain.ImagePrompt }

func (f fakePrompts) GetPrompt(context.Context, string) (*domain.ImagePrompt, error) {
	return f.prompt, nil
}

type fakeJobs struct{ job *domain.CloneJobView }

func (f fakeJobs) Get(context.Context, string) (*domain.CloneJobView, error) { return f.job, nil }

// ── helpers ────────────────────────────────────────────────────────────────

func writePNG(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func itemWithImage(id, kind, path string, width, height int) images.Item {
	item := images.Item{ID: id, Kind: kind, SceneID: id, Width: width, Height: height, Status: "approved", CurrentVersion: new(1)}
	if kind == "thumbnail" {
		item.SceneID = ""
	}
	item.Versions = []images.Version{{
		Version: 1, Path: path, SourceFingerprint: images.SourceFingerprint(item),
	}}
	return item
}

// fixture dựng job nguồn + media giả + service thật với drawer giả.
func fixture(t *testing.T) (*Service, *fakeStore, *fakeMedia, *fakeDrawer) {
	t.Helper()
	dir := t.TempDir()
	cover := filepath.Join(dir, "cover.png")
	scene := filepath.Join(dir, "scene.png")
	if err := writePNG(cover); err != nil {
		t.Fatal(err)
	}
	if err := writePNG(scene); err != nil {
		t.Fatal(err)
	}
	coverSHA, _ := steps.SHA256File(cover)
	sceneSHA, _ := steps.SHA256File(scene)
	manifest := images.Manifest{
		Version: 1, Style: "full", Thumbnail: "clean",
		Story: &images.Story{
			Look: "ảnh tả thực",
			Shots: map[string]string{
				"thumbnail": "Một người đàn ông đứng trước cánh đồng lúa.",
				"s1":        "Một người phụ nữ bước qua cầu tre.",
			},
			Cast: []images.Cast{{Name: "Lan", Look: "áo bà ba"}},
		},
		Items: []images.Item{
			itemWithImage("thumbnail", "thumbnail", "job/images/cover-v1.png", 1280, 720),
			itemWithImage("s1", "scene", "job/images/scene-v1.png", 1920, 1080),
		},
	}
	manifest.Items[0].Versions[0].SHA256 = coverSHA
	manifest.Items[1].Versions[0].SHA256 = sceneSHA
	manifestFile := filepath.Join(dir, "manifest.json")
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(manifestFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	job := &domain.CloneJobView{
		CloneJob: domain.CloneJob{ID: "job-1", Status: domain.CloneSucceeded, Source: domain.CloneSourceSnapshot{Title: "Video xong"}},
		Steps: []domain.CloneStepRecord{{
			Step: domain.CloneImages,
			Output: &domain.CloneStepOutput{Artifacts: []domain.CloneArtifact{
				{Path: "job/images/manifest.json"},
				{Path: "job/images/cover-v1.png"},
				{Path: "job/images/scene-v1.png"},
			}},
		}},
	}
	recipe := library.StandardRecipe()
	recipe.Scenes.CoverPromptID = library.BuiltinCoverClean
	detail := &library.Detail{Summary: library.Summary{TemplateSummary: domain.TemplateSummary{
		Template: domain.Template{ID: "tpl-1", Name: "Mẫu", Recipe: recipe},
	}}}
	prompt := &domain.ImagePrompt{ID: library.BuiltinCoverClean, Name: "Ảnh sạch", Kind: "cover", BuiltIn: true, Composition: "clean"}
	store := &fakeStore{}
	blobs := &fakeMedia{
		files: map[string]string{
			"job/images/manifest.json": manifestFile,
			"job/images/cover-v1.png":  cover,
			"job/images/scene-v1.png":  scene,
		},
		blobs: map[string]string{},
	}
	drawer := &fakeDrawer{}
	svc := New(Options{
		Templates: fakeTemplates{detail: detail},
		Prompts:   fakePrompts{prompt: prompt},
		Jobs:      fakeJobs{job: job},
		Sources: func(context.Context) ([]library.PreviewSource, error) {
			return []library.PreviewSource{{JobID: "job-1", Title: "Video xong"}}, nil
		},
		Store: store,
		Media: blobs,
		Tasks: fakeTasks{},
		ResolveDrawer: func(context.Context) (DrawerResolution, error) {
			return DrawerResolution{Drawer: drawer, Provider: "chatgpt", Model: "gpt-image-1.5", Label: "ChatGPT", RefLimit: 4}, nil
		},
		Quota: func(context.Context) (library.TrialQuota, error) {
			return library.TrialQuota{Provider: "chatgpt", Label: "ChatGPT", Paid: true, Left: 5, Total: 50}, nil
		},
		Now:   func() time.Time { return time.Unix(1_700_000_000, 0) },
		NewID: func() string { return "trial-1" },
	})
	return svc, store, blobs, drawer
}

func waitState(t *testing.T, store *fakeStore, id string) library.CoverTrial {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		for _, row := range store.trials {
			if row.ID == id && row.State != library.TrialRunning {
				store.mu.Unlock()
				return row
			}
		}
		store.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("lần vẽ %s chưa kết thúc", id)
	return library.CoverTrial{}
}

// TestCoverTrialDrawsOneImageAndPersistsHistory kiểm đường thật: prompt ghép
// từ câu chuyện nguồn, đúng một lời gọi vẽ, ảnh được publish kèm hàng lịch sử.
func TestCoverTrialDrawsOneImageAndPersistsHistory(t *testing.T) {
	svc, store, mediaStore, drawer := fixture(t)
	status, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready || status.Cost.Images != 1 || status.Cost.Provider != "chatgpt" || status.Cost.Label != "ChatGPT" {
		t.Fatalf("status = %+v", status)
	}
	row := waitState(t, store, "trial-1")
	if row.State != library.TrialReady || row.CostImages != 1 {
		t.Fatalf("row = %+v", row)
	}
	if row.MediaPath != "library/cover-trials/tpl-1/trial-1.png" {
		t.Fatalf("media path = %q", row.MediaPath)
	}
	if drawer.count() != 1 {
		t.Fatalf("draw count = %d", drawer.count())
	}
	req := drawer.requests[0]
	if req.Width != 1280 || req.Height != 720 {
		t.Fatalf("canvas = %dx%d", req.Width, req.Height)
	}
	if !strings.Contains(req.Prompt, "Một người đàn ông") {
		t.Fatalf("prompt thiếu câu chuyện nguồn: %s", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "keep the camera angle") {
		t.Fatalf("prompt thiếu ảnh tham chiếu bìa nguồn")
	}
	if len(req.References) != 2 {
		t.Fatalf("refs = %d, muốn 2 (bìa nguồn + cảnh đầu)", len(req.References))
	}
	if _, ok := mediaStore.blobs[row.MediaPath]; !ok {
		t.Fatalf("media chưa được publish")
	}
	if row.Width != 4 || row.Height != 3 {
		t.Fatalf("kích thước đọc từ ảnh thật = %dx%d", row.Width, row.Height)
	}
}

// TestCoverTrialRequiresConfirmAndQuota chặn trước khi tiêu ảnh.
func TestCoverTrialRequiresConfirmAndQuota(t *testing.T) {
	svc, store, _, drawer := fixture(t)
	_, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1"})
	if library.CodeOf(err) != library.CodeConfirmRequired {
		t.Fatalf("err = %v", err)
	}
	svc.opts.Quota = func(context.Context) (library.TrialQuota, error) {
		return library.TrialQuota{Provider: "chatgpt", Label: "ChatGPT", Paid: true, Left: 0, Total: 50}, nil
	}
	_, err = svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true})
	if library.CodeOf(err) != library.CodeQuota {
		t.Fatalf("err = %v", err)
	}
	if len(store.trials) != 0 || drawer.count() != 0 {
		t.Fatalf("đã ghi hàng/gọi vẽ khi chưa đủ điều kiện")
	}
}

// TestCoverTrialKeepsTwelveAndUnbindsOldMedia giữ đúng 12 và nhả media cũ.
func TestCoverTrialKeepsTwelveAndUnbindsOldMedia(t *testing.T) {
	svc, store, mediaStore, _ := fixture(t)
	for i := range 12 {
		store.trials = append(store.trials, library.CoverTrial{
			ID: "old-" + string(rune('a'+i)), TemplateID: "tpl-1", State: library.TrialReady,
			MediaPath: "library/cover-trials/tpl-1/old.png", CreatedAt: time.Unix(int64(i), 0),
		})
	}
	if _, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	waitState(t, store, "trial-1")
	store.mu.Lock()
	count := len(store.trials)
	store.mu.Unlock()
	if count != 12 {
		t.Fatalf("lịch sử = %d hàng, muốn 12", count)
	}
	if len(mediaStore.unbound) == 0 {
		t.Fatalf("chưa nhả media của hàng bị bỏ")
	}
}

// TestCoverTrialCancelKeepsCostZero huỷ trước khi gửi thì không tính ảnh.
func TestCoverTrialCancelKeepsCostZero(t *testing.T) {
	svc, store, _, _ := fixture(t)
	blocking := &fakeDrawer{block: true, started: make(chan struct{})}
	svc.opts.ResolveDrawer = func(context.Context) (DrawerResolution, error) {
		return DrawerResolution{Drawer: blocking, Provider: "chatgpt", Model: "gpt-image-1.5", Label: "ChatGPT", RefLimit: 4}, nil
	}
	if _, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocking.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider chưa bắt đầu")
	}
	if err := svc.Cancel(context.Background(), "tpl-1", "trial-1"); err != nil {
		t.Fatal(err)
	}
	row := waitState(t, store, "trial-1")
	if row.State != library.TrialCancelled || row.CostImages != 0 {
		t.Fatalf("row = %+v", row)
	}
}

func waitSample(t *testing.T, store *fakeStore, id string) library.PromptSample {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		for _, row := range store.samples {
			if row.ID == id && row.State != library.TrialRunning {
				store.mu.Unlock()
				return row
			}
		}
		store.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("ảnh mẫu %s chưa kết thúc", id)
	return library.PromptSample{}
}

// TestPromptSampleUsesChosenScene kiểm prompt cảnh dùng đúng cảnh nguồn và
// lưu ảnh mẫu thật (không phải stub).
func TestPromptSampleUsesChosenScene(t *testing.T) {
	svc, store, mediaStore, drawer := fixture(t)
	svc.opts.Prompts = fakePrompts{prompt: &domain.ImagePrompt{
		ID: "p-scene", Name: "Cảnh mẫu", Kind: "scene", BuiltIn: true, Prompt: "",
	}}
	if _, err := svc.StartPromptSample(context.Background(), "p-scene", library.PromptSampleRequest{
		VideoID: "job-1", SceneID: "s1", Confirm: true,
	}); err != nil {
		t.Fatal(err)
	}
	row := waitSample(t, store, "trial-1")
	if row.State != library.TrialReady || row.CostImages != 1 || row.SceneID != "s1" {
		t.Fatalf("row = %+v", row)
	}
	if drawer.count() != 1 {
		t.Fatalf("draw count = %d", drawer.count())
	}
	if !strings.Contains(drawer.requests[0].Prompt, "người phụ nữ bước qua cầu tre") {
		t.Fatalf("prompt không dùng cảnh đã chọn: %s", drawer.requests[0].Prompt)
	}
	if _, ok := mediaStore.blobs[row.MediaPath]; !ok {
		t.Fatalf("ảnh mẫu chưa publish")
	}
	if row.PromptKind != "scene" || row.Composition != "" {
		t.Fatalf("kind/composition = %q/%q", row.PromptKind, row.Composition)
	}
}

// waitTrialsTerminal chờ mọi hàng lịch sử đã rời running và trả bản sao.
func waitTrialsTerminal(t *testing.T, store *fakeStore) []library.CoverTrial {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		rows := append([]library.CoverTrial{}, store.trials...)
		running := false
		for _, row := range rows {
			if row.State == library.TrialRunning {
				running = true
			}
		}
		store.mu.Unlock()
		if len(rows) > 0 && !running {
			return rows
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("lịch sử chưa chốt trạng thái cuối")
	return nil
}

// TestCoverTrialDoubleStartDrawsOnePaidImage: hai cú bấm chồng nhau chỉ được
// sinh MỘT lời gọi trả phí; cú thứ hai nhận 409 thay vì vẽ thêm.
func TestCoverTrialDoubleStartDrawsOnePaidImage(t *testing.T) {
	svc, store, _, _ := fixture(t)
	var seq atomic.Int64
	svc.opts.NewID = func() string { return fmt.Sprintf("trial-%d", seq.Add(1)) }
	holding := &fakeDrawer{hold: true, started: make(chan struct{}), release: make(chan struct{})}
	svc.opts.ResolveDrawer = func(context.Context) (DrawerResolution, error) {
		return DrawerResolution{Drawer: holding, Provider: "chatgpt", Model: "gpt-image-1.5", Label: "ChatGPT", RefLimit: 4}, nil
	}
	store.insertEntered = make(chan struct{})
	store.insertGate = make(chan struct{})

	results := make(chan error, 2)
	start := func() {
		_, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true})
		results <- err
	}
	go start()
	select {
	case <-store.insertEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("lần ghi hàng đầu chưa vào")
	}
	go start()
	time.Sleep(20 * time.Millisecond) // cho cú bấm thứ hai tới cổng kiểm
	close(store.insertGate)

	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("hai lần bấm đều %v/%v, muốn đúng một lần nhận", first, second)
	}
	conflictErr := first
	if conflictErr == nil {
		conflictErr = second
	}
	if library.CodeOf(conflictErr) != library.CodeConflict {
		t.Fatalf("lỗi cú bấm thứ hai = %v, muốn 409", conflictErr)
	}
	close(holding.release)
	rows := waitTrialsTerminal(t, store)
	if len(rows) != 1 {
		t.Fatalf("lịch sử = %d hàng, muốn 1", len(rows))
	}
	if rows[0].State != library.TrialReady {
		t.Fatalf("hàng = %+v", rows[0])
	}
	if holding.count() != 1 {
		t.Fatalf("draw count = %d, muốn 1", holding.count())
	}
}

// TestCoverTrialCancelAcceptedBeforeDrawSuccessNeverWritesReady: huỷ nhận
// trước khi provider trả ảnh thì kể cả provider trả THÀNH CÔNG sau đó cũng
// không được ghi ready/publish ảnh; cost tính đúng lời gọi đã gửi.
func TestCoverTrialCancelAcceptedBeforeDrawSuccessNeverWritesReady(t *testing.T) {
	svc, store, mediaStore, _ := fixture(t)
	holding := &fakeDrawer{hold: true, started: make(chan struct{}), release: make(chan struct{})}
	svc.opts.ResolveDrawer = func(context.Context) (DrawerResolution, error) {
		return DrawerResolution{Drawer: holding, Provider: "chatgpt", Model: "gpt-image-1.5", Label: "ChatGPT", RefLimit: 4}, nil
	}
	if _, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-holding.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider chưa bắt đầu")
	}
	if err := svc.Cancel(context.Background(), "tpl-1", "trial-1"); err != nil {
		t.Fatalf("huỷ bị từ chối: %v", err)
	}
	close(holding.release)
	row := waitState(t, store, "trial-1")
	if row.State != library.TrialCancelled {
		t.Fatalf("hàng = %+v, muốn cancelled sau khi huỷ đã nhận", row)
	}
	if row.CostImages != 1 {
		t.Fatalf("cost = %d, muốn 1 vì lời gọi đã gửi", row.CostImages)
	}
	if _, ok := mediaStore.blobs["library/cover-trials/tpl-1/trial-1.png"]; ok {
		t.Fatal("ảnh bị publish sau khi huỷ đã nhận")
	}
}

// TestCoverTrialCancelAfterReadyKeepsResult: huỷ quá muộn trả 409 và không
// được đè lên ảnh đã xong.
func TestCoverTrialCancelAfterReadyKeepsResult(t *testing.T) {
	svc, store, mediaStore, _ := fixture(t)
	if _, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	row := waitState(t, store, "trial-1")
	if row.State != library.TrialReady {
		t.Fatalf("row = %+v", row)
	}
	if err := svc.Cancel(context.Background(), "tpl-1", "trial-1"); library.CodeOf(err) != library.CodeConflict {
		t.Fatalf("huỷ sau ready = %v, muốn 409", err)
	}
	again, err := store.GetCoverTrial(context.Background(), "tpl-1", "trial-1")
	if err != nil {
		t.Fatal(err)
	}
	if again.State != library.TrialReady || again.MediaPath == "" {
		t.Fatalf("kết quả bị đè: %+v", again)
	}
	if _, ok := mediaStore.blobs[again.MediaPath]; !ok {
		t.Fatal("media của kết quả ready bị mất")
	}
}

// TestCoverTrialCancelOrphanRowDoesNotPanic: hàng running mồ côi (không có
// goroutine trong tiến trình này) đóng sổ được, cost 0, không panic.
func TestCoverTrialCancelOrphanRowDoesNotPanic(t *testing.T) {
	svc, store, _, _ := fixture(t)
	store.mu.Lock()
	store.trials = append(store.trials, library.CoverTrial{
		ID: "trial-orphan", TemplateID: "tpl-1", State: library.TrialRunning, CreatedAt: time.Unix(1, 0),
	})
	store.mu.Unlock()
	if err := svc.Cancel(context.Background(), "tpl-1", "trial-orphan"); err != nil {
		t.Fatal(err)
	}
	row, err := store.GetCoverTrial(context.Background(), "tpl-1", "trial-orphan")
	if err != nil {
		t.Fatal(err)
	}
	if row.State != library.TrialCancelled || row.CostImages != 0 || row.FinishedAt == nil {
		t.Fatalf("row = %+v", row)
	}
	if _, err := store.GetCoverTrial(context.Background(), "tpl-other", "trial-orphan"); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("hàng đọc được ở template khác: %v", err)
	}
}

// TestCancelTemplateWaitsForInflightPublish: xoá template phải chờ lần vẽ đang
// publish chốt xong để media_path vừa bind còn dấu mà nhả — không rò media.
func TestCancelTemplateWaitsForInflightPublish(t *testing.T) {
	svc, store, mediaStore, _ := fixture(t)
	mediaStore.publishEntered = make(chan struct{})
	mediaStore.publishGate = make(chan struct{})
	if _, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-mediaStore.publishEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("media chưa vào transaction publish")
	}
	done := make(chan struct{})
	go func() {
		svc.CancelTemplate(context.Background(), "tpl-1")
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("CancelTemplate trả về khi lần vẽ chưa chốt xong")
	case <-time.After(50 * time.Millisecond):
	}
	close(mediaStore.publishGate)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("CancelTemplate không chờ lần vẽ chốt xong")
	}
	paths, err := store.CoverTrialMediaPaths(context.Background(), "tpl-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "library/cover-trials/tpl-1/trial-1.png" {
		t.Fatalf("media_path sau khi chờ = %v, muốn đường dẫn vừa publish", paths)
	}
}

// TestCloseCancelsFlightAndRefusesNewStart: đóng ứng dụng huỷ lần vẽ, chờ nó
// dừng, và lần bấm sau nhận lỗi thay vì treo.
func TestCloseCancelsFlightAndRefusesNewStart(t *testing.T) {
	svc, store, _, _ := fixture(t)
	blocking := &fakeDrawer{block: true, started: make(chan struct{})}
	svc.opts.ResolveDrawer = func(context.Context) (DrawerResolution, error) {
		return DrawerResolution{Drawer: blocking, Provider: "chatgpt", Model: "gpt-image-1.5", Label: "ChatGPT", RefLimit: 4}, nil
	}
	if _, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocking.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider chưa bắt đầu")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Close(ctx); err != nil {
		t.Fatalf("Close = %v", err)
	}
	row := waitState(t, store, "trial-1")
	if row.State != library.TrialCancelled {
		t.Fatalf("row = %+v, muốn cancelled", row)
	}
	if _, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true}); library.CodeOf(err) != library.CodeToolMissing {
		t.Fatalf("Start sau Close = %v, muốn tool_missing", err)
	}
	if err := svc.Close(ctx); err != nil {
		t.Fatalf("Close lần hai = %v", err)
	}
}

// TestTrialCostOnlyPromisesRealImages: chưa có nguồn vẽ thì không báo giá ảnh;
// provider là id thật, label là nhãn hiển thị, quotaLeft chỉ khi tính phí.
func TestTrialCostOnlyPromisesRealImages(t *testing.T) {
	svc, _, _, _ := fixture(t)
	status, err := svc.Status(context.Background(), "tpl-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Cost.Images != 1 || status.Cost.Provider != "chatgpt" || status.Cost.Label != "ChatGPT" || status.Cost.QuotaLeft != 5 {
		t.Fatalf("cost = %+v", status.Cost)
	}
	svc.opts.ResolveDrawer = func(context.Context) (DrawerResolution, error) {
		return DrawerResolution{}, errors.New("chưa cấu hình nguồn vẽ")
	}
	status, err = svc.Status(context.Background(), "tpl-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Cost.Images != 0 || status.Cost.Provider != "" || status.Cost.Label != "" {
		t.Fatalf("cost khi chưa có nguồn vẽ = %+v, muốn không hứa ảnh nào", status.Cost)
	}
	if status.Blocked == "" || status.Ready {
		t.Fatalf("status = ready %v blocked %q, muốn bị khoá kèm lý do", status.Ready, status.Blocked)
	}
}

// TestScenesRequiresVideoID: thiếu videoId là lỗi nhập (400), không phải 500.
func TestScenesRequiresVideoID(t *testing.T) {
	svc, _, _, _ := fixture(t)
	if _, err := svc.Scenes(context.Background(), library.BuiltinCoverClean, "  "); library.CodeOf(err) != library.CodeInvalidInput {
		t.Fatalf("Scenes thiếu videoId = %v, muốn invalid_input", err)
	}
}

// TestCancelPromptScopedToItsPrompt: huỷ prompt khác không được dừng ảnh mẫu
// đang chạy; huỷ đúng prompt mới dừng nó.
func TestCancelPromptScopedToItsPrompt(t *testing.T) {
	svc, store, _, _ := fixture(t)
	svc.opts.Prompts = fakePrompts{prompt: &domain.ImagePrompt{ID: "p-scene", Name: "Cảnh mẫu", Kind: "scene"}}
	blocking := &fakeDrawer{block: true, started: make(chan struct{})}
	svc.opts.ResolveDrawer = func(context.Context) (DrawerResolution, error) {
		return DrawerResolution{Drawer: blocking, Provider: "chatgpt", Model: "gpt-image-1.5", Label: "ChatGPT", RefLimit: 4}, nil
	}
	if _, err := svc.StartPromptSample(context.Background(), "p-scene", library.PromptSampleRequest{
		VideoID: "job-1", SceneID: "s1", Confirm: true,
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocking.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider chưa bắt đầu")
	}
	svc.CancelPrompt(context.Background(), "p-other")
	if !svc.inFlight(flightKey("s", "p-scene", "trial-1")) {
		t.Fatal("huỷ prompt khác đã dừng ảnh mẫu của p-scene")
	}
	svc.CancelPrompt(context.Background(), "p-scene")
	row := waitSample(t, store, "trial-1")
	if row.State != library.TrialCancelled || row.CostImages != 0 {
		t.Fatalf("row = %+v", row)
	}
}

// TestCloseReportsTimeoutUntilFlightSettles: Close chỉ trả nil khi mọi lần vẽ
// đã chốt xong; provider không tôn trọng huỷ thì trả deadline để nơi gọi không
// tháo media/pool trong im lặng, và lần Close sau (đã chốt) trả nil.
func TestCloseReportsTimeoutUntilFlightSettles(t *testing.T) {
	svc, store, _, _ := fixture(t)
	holding := &fakeDrawer{hold: true, started: make(chan struct{}), release: make(chan struct{})}
	svc.opts.ResolveDrawer = func(context.Context) (DrawerResolution, error) {
		return DrawerResolution{Drawer: holding, Provider: "chatgpt", Model: "gpt-image-1.5", Label: "ChatGPT", RefLimit: 4}, nil
	}
	if _, err := svc.Start(context.Background(), "tpl-1", library.CoverTrialRequest{JobID: "job-1", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-holding.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider chưa bắt đầu")
	}
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer shortCancel()
	if err := svc.Close(shortCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close khi provider treo = %v, muốn deadline", err)
	}
	close(holding.release)
	waitState(t, store, "trial-1")
	lateCtx, lateCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer lateCancel()
	if err := svc.Close(lateCtx); err != nil {
		t.Fatalf("Close sau khi chốt = %v, muốn nil", err)
	}
}
