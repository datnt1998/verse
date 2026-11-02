package preview

// Regression thật cho lease đọc: repository giả mô phỏng đúng khoá advisory
// của PostgreSQL (lease đọc giữ khoá chia sẻ, công bố giữ khoá độc quyền trên
// cùng khoá nội dung) trên media.Service + blobstore local thật. Chuẩn bị
// (preflight) phải nhả hết lease, và ảnh/âm thanh nguồn phải nhập được thật
// qua ImportFileOnce thay vì chỉ "không ném lỗi".

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// errLeaseHeld mirrors PostgreSQL: công bố cần khoá độc quyền trên khoá nội
// dung nhưng một lease đọc vẫn giữ khoá chia sẻ. Trả lỗi để test tất định,
// không treo test binary.
var errLeaseHeld = errors.New("preview lease regression: exclusive content lock blocked by an open read lease")

// leaseRepository là media.Repository tối thiểu: manifest trong bộ nhớ, blob
// local thật. Interface nhúng cố ý để trống — thao tác ngoài phạm vi sẽ panic
// thay vì âm thầm thành công.
type leaseRepository struct {
	media.Repository
	mu         sync.Mutex
	leaseMu    sync.Mutex
	descriptor media.Descriptor
	refs       map[string]media.Ref
	leases     map[string]int
}

func (r *leaseRepository) State(context.Context) (media.State, error) {
	return media.State{Active: "local", Revision: 1, Generation: 1}, nil
}

func (r *leaseRepository) Descriptor(_ context.Context, id string) (media.Descriptor, error) {
	if id != r.descriptor.ID {
		return media.Descriptor{}, blobstore.ErrNotFound
	}
	return r.descriptor, nil
}

func (r *leaseRepository) Publication(ctx context.Context, callback func(context.Context, media.State) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	before := make(map[string]media.Ref, len(r.refs))
	for path, ref := range r.refs {
		before[path] = ref
	}
	state, err := r.State(ctx)
	if err != nil {
		return err
	}
	if err := callback(ctx, state); err != nil {
		r.refs = before
		return err
	}
	return nil
}

func (r *leaseRepository) Lock(_ context.Context, store, key string) error {
	if store != r.descriptor.ID || blobstore.ValidateKey(key) != nil {
		return blobstore.ErrInvalidKey
	}
	r.leaseMu.Lock()
	defer r.leaseMu.Unlock()
	if r.leases[store+"/"+key] > 0 {
		return errLeaseHeld
	}
	return nil
}

func (r *leaseRepository) Bind(_ context.Context, ref media.Ref) error {
	if ref.StoreID != r.descriptor.ID || ref.Path == "" {
		return blobstore.ErrInvalidKey
	}
	r.refs[ref.Path] = ref
	return nil
}

func (r *leaseRepository) Resolve(_ context.Context, store, path string) (media.Ref, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ref, ok := r.refs[path]
	if !ok || store != r.descriptor.ID {
		return media.Ref{}, blobstore.ErrNotFound
	}
	return ref, nil
}

func (r *leaseRepository) ReadLease(_ context.Context, store, key string) (func(), error) {
	if store != r.descriptor.ID || blobstore.ValidateKey(key) != nil {
		return nil, blobstore.ErrInvalidKey
	}
	lease := store + "/" + key
	r.leaseMu.Lock()
	r.leases[lease]++
	r.leaseMu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			r.leaseMu.Lock()
			r.leases[lease]--
			r.leaseMu.Unlock()
		})
	}, nil
}

// openLeases là số lease đọc còn mở trên mọi khoá nội dung.
func (r *leaseRepository) openLeases() int {
	r.leaseMu.Lock()
	defer r.leaseMu.Unlock()
	total := 0
	for _, count := range r.leases {
		total += count
	}
	return total
}

// newLeaseMedia dựng media.Service thật trên blobstore local thật, đúng cách
// serve gắn kho media.
func newLeaseMedia(t *testing.T) (*media.Service, *leaseRepository) {
	t.Helper()
	root := t.TempDir()
	local, err := blobstore.NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := local.Close(); err != nil {
			t.Error(err)
		}
	})
	repository := &leaseRepository{
		descriptor: media.Descriptor{ID: "local", Config: blobstore.Config{Driver: "local", LocalRoot: root}},
		refs:       map[string]media.Ref{},
		leases:     map[string]int{},
	}
	service := media.New(repository, nil, func(media.Descriptor) (blobstore.BlobStore, error) { return local, nil })
	return service, repository
}

// leaseEditorRepository là kho JSON trong bộ nhớ của Biên tập, đúng hợp đồng
// CAS/Get/List/Paths mà ImportFileOnce dùng.
type leaseEditorRepository struct {
	mu        sync.Mutex
	resources map[string]editor.Resource
	media     *leaseRepository
}

func newLeaseEditorRepository(media *leaseRepository) *leaseEditorRepository {
	return &leaseEditorRepository{resources: map[string]editor.Resource{}, media: media}
}

func (r *leaseEditorRepository) Get(_ context.Context, key string) (editor.Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	resource, ok := r.resources[key]
	if !ok {
		return editor.Resource{}, blobstore.ErrNotFound
	}
	return resource, nil
}

func (r *leaseEditorRepository) List(_ context.Context, prefix string) ([]editor.Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []editor.Resource{}
	for key, resource := range r.resources {
		if strings.HasPrefix(key, prefix) {
			out = append(out, resource)
		}
	}
	return out, nil
}

func (r *leaseEditorRepository) CAS(_ context.Context, key, expected string, next editor.Resource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resources[key].Revision != expected {
		return editor.ErrConflict
	}
	if next.Key == "" {
		next.Key = key
	}
	r.resources[key] = next
	return nil
}

func (r *leaseEditorRepository) Delete(_ context.Context, key, expected string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resources[key].Revision != expected {
		return editor.ErrConflict
	}
	delete(r.resources, key)
	return nil
}

func (r *leaseEditorRepository) Paths(_ context.Context, prefix string) ([]string, error) {
	r.media.mu.Lock()
	defer r.media.mu.Unlock()
	out := []string{}
	for path := range r.media.refs {
		if strings.HasPrefix(path, prefix) {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out, nil
}

// leaseProber nhận dạng bằng magic bytes của chính tệp nhập, không gọi
// ffprobe: ảnh fixture là PNG thật, âm thanh còn lại là bản sao của nguồn.
type leaseProber struct{}

func (leaseProber) Probe(_ context.Context, file string) (editor.ProbeResult, error) {
	handle, err := os.Open(file)
	if err != nil {
		return editor.ProbeResult{}, err
	}
	defer handle.Close()
	head := make([]byte, 8)
	if _, err := io.ReadFull(handle, head); err != nil {
		return editor.ProbeResult{}, err
	}
	if string(head) == "\x89PNG\r\n\x1a\n" {
		width, height := 24.0, 24.0
		return editor.ProbeResult{Kind: "image", MIME: "image/png", Width: &width, Height: &height}, nil
	}
	return editor.ProbeResult{Kind: "audio", MIME: "audio/mp4"}, nil
}

// leaseImportEditor dùng editor.Service thật cho ImportFileOnce (đi qua
// media.Publish + khoá nội dung thật) và fakeEditor cho vòng đời project, vì
// regression này chỉ đo lease ở ranh giới nhập media.
type leaseImportEditor struct {
	*editor.Service
	project *fakeEditor
	mu      sync.Mutex
	imports []editor.MediaRecord
}

func (e *leaseImportEditor) LoadProject(ctx context.Context, id string) (editor.Project, string, error) {
	return e.project.LoadProject(ctx, id)
}

func (e *leaseImportEditor) DuplicateProjectAs(ctx context.Context, id, name, newID string) (editor.Project, string, error) {
	return e.project.DuplicateProjectAs(ctx, id, name, newID)
}

func (e *leaseImportEditor) RunCommands(ctx context.Context, id string, commands []editor.Command, options editor.ApplyCommandsOptions) (editor.Object, error) {
	return e.project.RunCommands(ctx, id, commands, options)
}

func (e *leaseImportEditor) DeleteProject(ctx context.Context, id string) error {
	return e.project.DeleteProject(ctx, id)
}

func (e *leaseImportEditor) ImportFileOnce(ctx context.Context, opts editor.ImportFileOnceOptions) (editor.MediaRecord, bool, error) {
	record, created, err := e.Service.ImportFileOnce(ctx, opts)
	if err == nil {
		e.mu.Lock()
		e.imports = append(e.imports, record)
		e.mu.Unlock()
	}
	return record, created, err
}

func newLeaseEditor(t *testing.T, backend *media.Service, repository *leaseRepository) *editor.Service {
	t.Helper()
	now := func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) }
	workspace := newLeaseEditorRepository(repository)
	staging := t.TempDir()
	return &editor.Service{
		Store:      editor.Store{Media: backend, Repository: workspace, StagingDir: staging, Now: now},
		Media:      backend,
		Repository: workspace,
		StagingDir: staging,
		Now:        now,
		Prober:     leaseProber{},
	}
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestPreviewReleasesSourceLeasesBeforePublishingMedia chạy chuỗi thật: nguồn
// đã duyệt → preflight → nhập ảnh/âm thanh qua media.Publish → project xem thử
// đạt ready. Trước bản sửa, preflight giữ lease ảnh và build giữ lease ảnh +
// âm thanh qua ImportFileOnce, nên khoá độc quyền bị chặn và bản xem thử hỏng.
func TestPreviewReleasesSourceLeasesBeforePublishingMedia(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixtureOptions{recipe: recipeFixed(1), durationSec: 180, minutes: 3})
	defer f.cleanup()

	backend, repository := newLeaseMedia(t)
	artifacts := make([]media.Artifact, 0, len(f.media.files))
	for logical, file := range f.media.files {
		artifacts = append(artifacts, media.Artifact{Path: logical, File: file})
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	if err := backend.Publish(ctx, artifacts, nil); err != nil {
		t.Fatalf("công bố nguồn vào kho media thật: %v", err)
	}

	workspace := newLeaseEditor(t, backend, repository)
	importer := &leaseImportEditor{Service: workspace, project: f.editor}
	svc := New(Options{
		Templates: f.tmpl,
		Jobs:      f.jobs,
		Editor:    importer,
		Media:     backend,
		Runner:    f.runner,
		Ffmpeg:    func(context.Context) (string, bool) { return "/usr/bin/ffmpeg", true },
		Tasks:     f.tasks,
		Now:       func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) },
	})

	job, err := f.jobs.Get(ctx, "job-1")
	if err != nil || job == nil {
		t.Fatalf("job nguồn: %v", err)
	}
	if plan, err := svc.prepare(ctx, f.detail, job, 3); err != nil || plan == nil {
		t.Fatalf("preflight: plan=%v err=%v", plan, err)
	}
	if leases := repository.openLeases(); leases != 0 {
		t.Fatalf("preflight còn %d lease đọc sau khi trả 202", leases)
	}

	if _, err := svc.Start(ctx, "tpl-1", library.PreviewRequest{Minutes: 3}); err != nil {
		t.Fatalf("start: %v", err)
	}
	record := waitRecord(t, f)
	if record.JobID != "job-1" || record.Minutes != 3 || record.ProjectID == "" {
		t.Fatalf("bản xem thử công bố sai: %+v", record)
	}
	if leases := repository.openLeases(); leases != 0 {
		t.Fatalf("build còn %d lease đọc sau khi công bố", leases)
	}

	importer.mu.Lock()
	imports := append([]editor.MediaRecord{}, importer.imports...)
	importer.mu.Unlock()
	if len(imports) != 2 {
		t.Fatalf("nhập %d media, muốn 1 ảnh + 1 âm thanh: %+v", len(imports), imports)
	}

	// Nguồn thật của đúng hai tệp sẽ được nhập: ảnh cảnh đã duyệt và âm thanh
	// nguồn. Đối chiếu mã nội dung, rồi mở lại bytes đã công bố trong kho.
	var sourceScene []byte
	for logical, file := range f.media.files {
		if strings.HasSuffix(logical, "/images/scene.png") {
			sourceScene, err = os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(sourceScene) == 0 {
		t.Fatal("fixture thiếu ảnh cảnh nguồn")
	}
	sourceAudio, err := os.ReadFile(f.media.files["clone/job-1/x/build/source-audio.m4a"])
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{
		hashBytes(sourceScene): sourceScene,
		hashBytes(sourceAudio): sourceAudio,
	}

	mediaIDs := map[string]bool{}
	for _, imported := range imports {
		metadata, err := workspace.GetMediaMetadata(ctx, imported.ID)
		if err != nil {
			t.Fatalf("metadata media %s: %v", imported.ID, err)
		}
		want, ok := sources[metadata.ContentHash]
		if !ok {
			t.Fatalf("media %s giữ mã nội dung lạ %s", imported.ID, metadata.ContentHash)
		}
		path := "editor/media/" + imported.ID + "/sources/" +
			strings.TrimPrefix(metadata.ContentHash, "sha256:") + "/" + metadata.FileName
		handle, err := backend.Acquire(ctx, path)
		if err != nil {
			t.Fatalf("mở bytes đã nhập của %s: %v", imported.ID, err)
		}
		got, readErr := os.ReadFile(handle.Path)
		handle.Release()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("bytes của %s không khớp nguồn đã duyệt", imported.ID)
		}
		mediaIDs[imported.ID] = true
	}

	// Project đổ ra là kết quả thật của editor.BuildTemplateFill: cảnh và âm
	// thanh phải trỏ đúng media vừa nhập.
	f.editor.mu.Lock()
	commands := append([]editor.Command{}, f.editor.commands...)
	f.editor.mu.Unlock()
	if got := sceneImageItems(t, commands); got != 1 {
		t.Fatalf("project xem thử có %d cảnh, muốn 1", got)
	}
	foundImage, foundAudio := false, false
	for _, command := range commands {
		if command.Type != "item.add" {
			continue
		}
		item, _ := command.Params["item"].(editor.Object)
		if item == nil {
			continue
		}
		id, _ := item["mediaId"].(string)
		if id != "" && !mediaIDs[id] {
			t.Fatalf("project trỏ tới media không được nhập thật: %q", id)
		}
		switch item["type"] {
		case "image":
			foundImage = true
		case "audio":
			foundAudio = true
		}
	}
	if !foundImage || !foundAudio {
		t.Fatalf("project thiếu cảnh/âm thanh: image=%v audio=%v", foundImage, foundAudio)
	}
}

// TestCancelReleasesSourceLeaseWhileAudioRuns: huỷ giữa lúc ffmpeg đang đọc
// âm thanh phải chờ lượt dựng dừng và nhả lease đọc, không để lượt sau kế thừa.
func TestCancelReleasesSourceLeaseWhileAudioRuns(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixtureOptions{recipe: recipeFixed(1), durationSec: 180, minutes: 3, blockRunner: true})
	defer f.cleanup()
	started := make(chan struct{}, 1)
	f.runner.onBlock = func() { started <- struct{}{} }

	backend, repository := newLeaseMedia(t)
	artifacts := make([]media.Artifact, 0, len(f.media.files))
	for logical, file := range f.media.files {
		artifacts = append(artifacts, media.Artifact{Path: logical, File: file})
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	if err := backend.Publish(ctx, artifacts, nil); err != nil {
		t.Fatalf("công bố nguồn: %v", err)
	}
	workspace := newLeaseEditor(t, backend, repository)
	importer := &leaseImportEditor{Service: workspace, project: f.editor}
	svc := New(Options{
		Templates: f.tmpl,
		Jobs:      f.jobs,
		Editor:    importer,
		Media:     backend,
		Runner:    f.runner,
		Ffmpeg:    func(context.Context) (string, bool) { return "/usr/bin/ffmpeg", true },
		Tasks:     f.tasks,
		Now:       func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) },
	})
	if _, err := svc.Start(ctx, "tpl-1", library.PreviewRequest{Minutes: 3}); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-started:
	case failure := <-f.tmpl.failed:
		t.Fatalf("dựng hỏng trước khi đọc âm thanh: %+v", failure)
	case <-time.After(10 * time.Second):
		t.Fatal("hết giờ chờ ffmpeg đọc âm thanh")
	}
	if leases := repository.openLeases(); leases == 0 {
		t.Fatal("ffmpeg đang đọc âm thanh nhưng không còn lease đọc")
	}
	cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Cancel(cancelCtx, "tpl-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if leases := repository.openLeases(); leases != 0 {
		t.Fatalf("huỷ lượt dựng còn %d lease đọc", leases)
	}
	f.tmpl.mu.Lock()
	published := f.tmpl.record
	f.tmpl.mu.Unlock()
	if published != nil {
		t.Fatalf("huỷ lượt dựng vẫn công bố: %+v", published)
	}
}

// TestStageSourceCleansStagedCopyOnCancel: huỷ giữa chừng không để lại tệp
// chờ nhập trong thư mục tạm.
func TestStageSourceCleansStagedCopyOnCancel(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, t.TempDir(), "scene.png", pngBytes(t, 8))
	handle := &media.Handle{Path: file}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := stageSource(ctx, dir, handle); !errors.Is(err, context.Canceled) {
		t.Fatalf("stage với ctx đã huỷ = %v, muốn context.Canceled", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("còn %d tệp tạm sau khi huỷ: %v", len(entries), entries)
	}
}
