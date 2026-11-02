package library

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// Real local publication/acquisition with an in-memory manifest. The embedded
// interface is deliberately unset: an unexpected repository operation panics
// rather than silently succeeding in these focused media-binding regressions.
// Publication rolls back manifest entries when the callback fails.
type templateMediaRepository struct {
	media.Repository
	mu           sync.Mutex
	leaseMu      sync.Mutex
	descriptor   media.Descriptor
	refs         map[string]media.Ref
	leases       map[string]int
	publications int
	failure      error
}

// errTemplateMediaLeaseHeld mirrors PostgreSQL: a publication's exclusive
// advisory lock on a content key cannot be taken while a read lease still holds
// the shared lock for that key. Returning an error keeps the regression
// deterministic instead of blocking the test binary forever.
var errTemplateMediaLeaseHeld = errors.New("template media repository: exclusive content lock blocked by an open read lease")

func templateLocalMedia(t *testing.T) (*media.Service, *templateMediaRepository, string) {
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
	repository := &templateMediaRepository{
		descriptor: media.Descriptor{ID: "local", Config: blobstore.Config{Driver: "local", LocalRoot: root}},
		refs:       map[string]media.Ref{},
		leases:     map[string]int{},
	}
	service := media.New(repository, nil, func(media.Descriptor) (blobstore.BlobStore, error) { return local, nil })
	return service, repository, root
}

func (r *templateMediaRepository) State(context.Context) (media.State, error) {
	return media.State{Active: "local", Revision: 1, Generation: 1}, nil
}
func (r *templateMediaRepository) Descriptor(_ context.Context, id string) (media.Descriptor, error) {
	if id != r.descriptor.ID {
		return media.Descriptor{}, blobstore.ErrNotFound
	}
	return r.descriptor, nil
}
func (r *templateMediaRepository) Publication(ctx context.Context, callback func(context.Context, media.State) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failure != nil {
		return r.failure
	}
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
	r.publications++
	return nil
}
func (r *templateMediaRepository) Lock(_ context.Context, store, key string) error {
	if store != r.descriptor.ID || blobstore.ValidateKey(key) != nil {
		return blobstore.ErrInvalidKey
	}
	r.leaseMu.Lock()
	defer r.leaseMu.Unlock()
	if r.leases[store+"/"+key] > 0 {
		return errTemplateMediaLeaseHeld
	}
	return nil
}
func (r *templateMediaRepository) Bind(_ context.Context, ref media.Ref) error {
	if ref.StoreID != r.descriptor.ID || ref.Path == "" {
		return blobstore.ErrInvalidKey
	}
	r.refs[ref.Path] = ref
	return nil
}
func (r *templateMediaRepository) Resolve(_ context.Context, store, path string) (media.Ref, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ref, ok := r.refs[path]
	if !ok || store != r.descriptor.ID {
		return media.Ref{}, blobstore.ErrNotFound
	}
	return ref, nil
}
func (r *templateMediaRepository) openLeases() int {
	r.leaseMu.Lock()
	defer r.leaseMu.Unlock()
	total := 0
	for _, count := range r.leases {
		total += count
	}
	return total
}

func (r *templateMediaRepository) ReadLease(_ context.Context, store, key string) (func(), error) {
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

func TestTemplateSnapshotRepairsMissingBundledAssetsWithoutRewritingProject(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	// Reproduce a row created by the old metadata-only CLI dependency wiring.
	tpl, err := svc.Create(ctx, "Đã tạo bằng CLI")
	if err != nil {
		t.Fatal(err)
	}
	backend, repository, root := templateLocalMedia(t)
	svc = testService(t, store, func(deps *Deps) { deps.Media = backend })
	snapshot, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.refs) != 4 || len(snapshot.Binding.Media) != 4 {
		t.Fatal("missing bundled files were not republished")
	}
	for _, binding := range snapshot.Binding.Media {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(snapshot.MediaFiles[binding.MediaID])))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("sha256:%x", sha256.Sum256(data)) != binding.ContentHash {
			t.Fatal("repaired file differs from canonical asset")
		}
	}
	stored, err := store.GetTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 1 || !bytes.Equal(stored.Project, tpl.Project) || !bytes.Equal(snapshot.Project, tpl.Project) {
		t.Fatal("asset repair rewrote the authored project or revision")
	}
	publications := repository.publications
	if _, err := svc.SnapshotTemplate(ctx, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if repository.publications != publications {
		t.Fatal("healthy snapshots republish bundled bytes")
	}
	// A stale manifest entry with a missing local file is repaired as well.
	path := snapshot.MediaFiles["standard-scene-cool"]
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SnapshotTemplate(ctx, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if repository.publications != publications+1 {
		t.Fatal("repair did not publish only the absent asset")
	}
}

func TestTemplateSnapshotRejectsDriftInsteadOfRepairingPresentBytes(t *testing.T) {
	store := newMemStore()
	backend, repository, root := templateLocalMedia(t)
	svc := testService(t, store, func(deps *Deps) { deps.Media = backend })
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Không ghi đè")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "library", "standard-template", "scene-cool.webp")
	changed := []byte("out-of-band replacement")
	if err := os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	publications := repository.publications
	if _, err := svc.SnapshotTemplate(ctx, tpl.ID); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("drift accepted: %v", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, changed) || repository.publications != publications {
		t.Fatal("drift was hidden by overwriting present bytes")
	}
}

func TestTemplateMissingAssetRepairPropagatesPublicationFailure(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Chưa có tệp")
	if err != nil {
		t.Fatal(err)
	}
	backend, repository, _ := templateLocalMedia(t)
	repository.failure = errors.New("publication unavailable")
	svc = testService(t, store, func(deps *Deps) { deps.Media = backend })
	if _, err := svc.SnapshotTemplate(ctx, tpl.ID); !errors.Is(err, repository.failure) {
		t.Fatalf("repair failure hidden: %v", err)
	}
	stored, err := store.GetTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored.Project, tpl.Project) || stored.Revision != 1 || len(repository.refs) != 0 {
		t.Fatal("failed repair changed template or exposed bindings")
	}
}

// ── người dùng thật: Thư viện ↔ kho Biên tập ───────────────────────────────

// templateEditorRepository là kho JSON trong bộ nhớ của Biên tập. Nó giữ
// đúng hợp đồng CAS/Get/List/Paths mà editor.Service dùng khi nhập media, nên
// test chạy qua chính ImportFileOnce thay vì bắt chước kết quả của nó.
type templateEditorRepository struct {
	mu        sync.Mutex
	resources map[string]editor.Resource
	media     *templateMediaRepository
}

func newTemplateEditorRepository(media *templateMediaRepository) *templateEditorRepository {
	return &templateEditorRepository{resources: map[string]editor.Resource{}, media: media}
}

func (r *templateEditorRepository) Get(_ context.Context, key string) (editor.Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	resource, ok := r.resources[key]
	if !ok {
		return editor.Resource{}, blobstore.ErrNotFound
	}
	return resource, nil
}

func (r *templateEditorRepository) List(_ context.Context, prefix string) ([]editor.Resource, error) {
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

func (r *templateEditorRepository) CAS(_ context.Context, key, expected string, next editor.Resource) error {
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

func (r *templateEditorRepository) Delete(_ context.Context, key, expected string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resources[key].Revision != expected {
		return editor.ErrConflict
	}
	delete(r.resources, key)
	return nil
}

// Paths trả các binding đã công bố, đúng thứ mediaIDs của Biên tập dùng để
// dựng lại chỉ mục khi chỉ mục mất.
func (r *templateEditorRepository) Paths(_ context.Context, prefix string) ([]string, error) {
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

// templateProber nhận dạng media bằng magic bytes của chính tệp nhập, không
// gọi ffprobe: test không dùng công cụ ngoài hay mạng.
type templateProber struct{}

func (templateProber) Probe(_ context.Context, file string) (editor.ProbeResult, error) {
	handle, err := os.Open(file)
	if err != nil {
		return editor.ProbeResult{}, err
	}
	defer handle.Close()
	head := make([]byte, 12)
	if _, err := io.ReadFull(handle, head); err != nil {
		return editor.ProbeResult{}, err
	}
	switch {
	case string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return editor.ProbeResult{Kind: "image", MIME: "image/webp"}, nil
	case string(head[0:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return editor.ProbeResult{Kind: "audio", MIME: "audio/wav"}, nil
	case string(head[4:8]) == "ftyp":
		return editor.ProbeResult{Kind: "video", MIME: "video/mp4"}, nil
	}
	return editor.ProbeResult{}, fmt.Errorf("media không nhận dạng được: %s", filepath.Base(file))
}

// templateRealEditor dựng service Biên tập thật trên cùng backend media với
// Thư viện, đúng cách serve gắn hai service với nhau.
func templateRealEditor(t *testing.T, backend *media.Service, repository *templateMediaRepository) *editor.Service {
	t.Helper()
	now := func() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) }
	workspace := newTemplateEditorRepository(repository)
	staging := t.TempDir()
	return &editor.Service{
		Store:      editor.Store{Media: backend, Repository: workspace, StagingDir: staging, Now: now},
		Media:      backend,
		Repository: workspace,
		StagingDir: staging,
		Now:        now,
		Prober:     templateProber{},
	}
}

// TestTemplateSnapshotBindsRealEditorMediaAndExportsRoundTrip chạy đúng chuỗi
// người dùng: mở mẫu chuẩn → ảnh có sẵn được nhập vào kho Biên tập bằng
// ImportFileOnce → project đổi sang mã media thật → bản chốt có đủ
// Binding.Media/MediaFiles mở được → xuất rồi nhập lại vẫn giữ media.
func TestTemplateSnapshotBindsRealEditorMediaAndExportsRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	backend, repository, _ := templateLocalMedia(t)
	workspace := templateRealEditor(t, backend, repository)
	svc := testService(t, store, func(deps *Deps) {
		deps.Media = backend
		deps.Importer = workspace
	})
	svc.BindMediaReader(workspace)

	tpl, err := svc.Create(ctx, "Mẫu media thật")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("chốt mẫu video sau khi nhập ảnh có sẵn: %v", err)
	}
	assets, err := standardAssets()
	if err != nil {
		t.Fatal(err)
	}
	hashes := make(map[string]bool, len(assets))
	for _, asset := range assets {
		hashes["sha256:"+asset.SHA256] = true
	}
	if len(snapshot.Binding.Media) != len(assets) || len(snapshot.MediaFiles) != len(assets) {
		t.Fatalf("bản chốt có %d media và %d tệp, muốn %d", len(snapshot.Binding.Media), len(snapshot.MediaFiles), len(assets))
	}
	project, err := editor.Parse(snapshot.Project)
	if err != nil {
		t.Fatal(err)
	}
	projectIDs := projectMediaIDs(project)
	if len(projectIDs) != len(assets) {
		t.Fatalf("project trỏ tới %d media, muốn %d: %v", len(projectIDs), len(assets), projectIDs)
	}
	bound := make(map[string]bool, len(assets))
	for _, binding := range snapshot.Binding.Media {
		bound[binding.MediaID] = true
		if !hashes[binding.ContentHash] {
			t.Fatalf("bản chốt giữ mã nội dung lạ cho %s: %s", binding.MediaID, binding.ContentHash)
		}
		record, err := workspace.GetMediaMetadata(ctx, binding.MediaID)
		if err != nil {
			t.Fatalf("media %s không có trong kho Biên tập: %v", binding.MediaID, err)
		}
		if record.ContentHash != binding.ContentHash {
			t.Fatalf("bản chốt %s dùng %s, kho Biên tập có %s", binding.MediaID, binding.ContentHash, record.ContentHash)
		}
		path := snapshot.MediaFiles[binding.MediaID]
		want := "editor/media/" + binding.MediaID + "/sources/" + strings.TrimPrefix(binding.ContentHash, "sha256:") + "/" + record.FileName
		if path != want {
			t.Fatalf("MediaFiles[%s] = %q, muốn tệp nguồn chuẩn %q", binding.MediaID, path, want)
		}
		handle, err := backend.Acquire(ctx, path)
		if err != nil {
			t.Fatalf("không mở được media %s: %v", binding.MediaID, err)
		}
		data, err := os.ReadFile(handle.Path)
		handle.Release()
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("sha256:%x", sha256.Sum256(data)) != binding.ContentHash {
			t.Fatalf("tệp của %s không khớp mã nội dung đã chốt", binding.MediaID)
		}
	}
	for _, id := range projectIDs {
		if strings.HasPrefix(id, "standard-") {
			t.Fatalf("project còn mã ảnh có sẵn %q thay vì media Biên tập", id)
		}
		if !bound[id] {
			t.Fatalf("project trỏ tới %q nhưng bản chốt không có media đó", id)
		}
	}
	// Chốt lần hai không nhập lại media cũng không đổi revision.
	again, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("chốt lần hai: %v", err)
	}
	if again.Revision != snapshot.Revision || len(again.Binding.Media) != len(assets) {
		t.Fatalf("chốt lần hai đổi bản chốt: revision %d, %d media", again.Revision, len(again.Binding.Media))
	}
	records, err := workspace.ListMedia(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != len(assets) {
		t.Fatalf("kho Biên tập có %d media, muốn %d (không nhập trùng)", len(records), len(assets))
	}

	// Xuất rồi nhập lại gói vừa xuất: media đi theo gói và dùng lại theo nội dung.
	var archive bytes.Buffer
	info, err := svc.ExportTemplate(ctx, tpl.ID, &archive)
	if err != nil {
		t.Fatalf("xuất mẫu video: %v", err)
	}
	if info.MediaCount != len(assets) {
		t.Fatalf("gói xuất có %d media, muốn %d", info.MediaCount, len(assets))
	}
	zipPath := filepath.Join(t.TempDir(), info.FileName)
	if err := os.WriteFile(zipPath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle, err := OpenTemplateBundle(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Close()
	if len(bundle.Media) != len(assets) {
		t.Fatalf("gói mở lại có %d media, muốn %d", len(bundle.Media), len(assets))
	}
	exported := make(map[string]bool, len(bundle.Media))
	for _, item := range bundle.Media {
		if !hashes[item.ContentHash] {
			t.Fatalf("gói xuất mang mã nội dung lạ cho %s: %s", item.ID, item.ContentHash)
		}
		exported[item.ID] = true
	}
	imported, err := svc.ImportTemplate(ctx, bundle, ImportOptions{})
	if err != nil {
		t.Fatalf("nhập lại gói vừa xuất: %v", err)
	}
	if !imported.OK || imported.MediaImported != 0 || imported.MediaReused != len(assets) {
		t.Fatalf("nhập lại = %+v", imported)
	}
	stored, err := store.GetTemplate(ctx, imported.TemplateID)
	if err != nil || stored == nil {
		t.Fatalf("mẫu nhập lại: %v", err)
	}
	reparsed, err := editor.Parse(stored.Project)
	if err != nil {
		t.Fatal(err)
	}
	reimported := projectMediaIDs(reparsed)
	if len(reimported) != len(assets) {
		t.Fatalf("project nhập lại trỏ tới %d media: %v", len(reimported), reimported)
	}
	for _, id := range reimported {
		if !exported[id] {
			t.Fatalf("project nhập lại trỏ tới media lạ %q", id)
		}
		if _, err := workspace.GetMediaMetadata(ctx, id); err != nil {
			t.Fatalf("media %s của project nhập lại không có trong kho Biên tập: %v", id, err)
		}
	}
}

// TestTemplateGetReleasesLibraryLeaseBeforeEditorImport reproduces the real
// serve binding: the editor's project store reads templates through
// library.ReadEditorProject while the library imports the bundled media into
// that same editor service. A template GET must terminate even though the
// library holds a read lease on each asset's content key and the editor's
// publication takes the exclusive lock for that key.
func TestTemplateGetReleasesLibraryLeaseBeforeEditorImport(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	backend, repository, _ := templateLocalMedia(t)
	workspace := templateRealEditor(t, backend, repository)
	svc := testService(t, store, func(deps *Deps) {
		deps.Media = backend
		deps.Importer = workspace
	})
	svc.BindMediaReader(workspace)
	// Same binding as serve: editor.Store.Read asks the library for a template
	// before falling back to its own workspace blobs.
	workspace.Store.Templates = svc

	tpl, err := svc.Create(ctx, "Mẫu GET không khoá chéo")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.Get(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("GET mẫu video: %v", err)
	}
	project, err := editor.Parse(detail.Project)
	if err != nil {
		t.Fatal(err)
	}
	ids := projectMediaIDs(project)
	assets, err := standardAssets()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != len(assets) {
		t.Fatalf("project trỏ tới %d media, muốn %d: %v", len(ids), len(assets), ids)
	}
	for _, id := range ids {
		if strings.HasPrefix(id, "standard-") {
			t.Fatalf("project còn mã ảnh có sẵn %q sau GET", id)
		}
		if _, err := workspace.GetMediaMetadata(ctx, id); err != nil {
			t.Fatalf("media %s của project không có trong kho Biên tập: %v", id, err)
		}
	}
	records, err := workspace.ListMedia(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != len(assets) {
		t.Fatalf("kho Biên tập có %d media, muốn %d", len(records), len(assets))
	}
	if open := repository.openLeases(); open != 0 {
		t.Fatalf("GET mẫu video còn giữ %d read lease", open)
	}
	again, err := svc.Get(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("GET mẫu video lần hai: %v", err)
	}
	if again.Revision != detail.Revision {
		t.Fatalf("GET lần hai đổi revision: %d → %d", detail.Revision, again.Revision)
	}

}
