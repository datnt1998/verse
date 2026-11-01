package editor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// Ports media-gc.test.ts ownership, grace, dry-run, reuse and import-race cases.
type mediaTestWorkspace struct {
	mu    sync.Mutex
	root  string
	paths map[string]string
}

func (m *mediaTestWorkspace) Acquire(_ context.Context, p string) (*media.Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.paths[p]
	if !ok {
		return nil, blobstore.ErrNotFound
	}
	return &media.Handle{Path: f}, nil
}
func (m *mediaTestWorkspace) Publish(ctx context.Context, as []media.Artifact, commit func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range as {
		b, e := os.ReadFile(a.File)
		if e != nil {
			return e
		}
		f := filepath.Join(m.root, filepath.FromSlash(a.Path))
		if e = os.MkdirAll(filepath.Dir(f), 0755); e != nil {
			return e
		}
		if e = os.WriteFile(f, b, 0600); e != nil {
			return e
		}
		m.paths[a.Path] = f
	}
	if commit != nil {
		return commit(ctx)
	}
	return nil
}
func (m *mediaTestWorkspace) Unbind(ctx context.Context, ps []string, commit func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if commit != nil {
		if e := commit(ctx); e != nil {
			return e
		}
	}
	for _, p := range ps {
		delete(m.paths, p)
	}
	return nil
}

type mediaTestRepository struct {
	mu        sync.Mutex
	resources map[string]Resource
	workspace *mediaTestWorkspace
	FailCAS   map[string]error
}

func (r *mediaTestRepository) Get(_ context.Context, k string) (Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.resources[k]
	if !ok {
		return v, blobstore.ErrNotFound
	}
	return v, nil
}
func (r *mediaTestRepository) List(_ context.Context, p string) ([]Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Resource{}
	for k, v := range r.resources {
		if strings.HasPrefix(k, p) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *mediaTestRepository) CAS(_ context.Context, k, expected string, v Resource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.FailCAS[k]; e != nil {
		return e
	}
	old := r.resources[k]
	if old.Revision != expected {
		return ErrConflict
	}
	r.resources[k] = v
	return nil
}
func (r *mediaTestRepository) Delete(_ context.Context, k, expected string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resources[k].Revision != expected {
		return ErrConflict
	}
	delete(r.resources, k)
	return nil
}
func (r *mediaTestRepository) Paths(_ context.Context, p string) ([]string, error) {
	r.workspace.mu.Lock()
	defer r.workspace.mu.Unlock()
	out := []string{}
	for k := range r.workspace.paths {
		if strings.HasPrefix(k, p) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}
func newMediaTestService(t *testing.T) *Service {
	t.Helper()
	w := &mediaTestWorkspace{root: t.TempDir(), paths: map[string]string{}}
	r := &mediaTestRepository{resources: map[string]Resource{}, workspace: w}
	s := &Service{Media: w, Repository: r, StagingDir: t.TempDir(), Now: func() time.Time { return time.Unix(1800000000, 0) }}
	s.Store = Store{Media: w, Repository: r, StagingDir: s.StagingDir, Now: s.Now}
	return s
}
func mediaTestImport(t *testing.T, s *Service, file string) MediaRecord {
	t.Helper()
	f, e := os.Open(filepath.Join("testdata", "__fixtures__", file))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	m, e := s.ImportMedia(context.Background(), ImportMediaOptions{FileName: file, DeclaredMIME: "image/png", Body: f})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func ageMediaTest(t *testing.T, s *Service, id string) {
	t.Helper()
	o, rev, e := s.readJSON(context.Background(), "media/"+id, "editor/media/"+id+"/metadata.json")
	if e != nil {
		t.Fatal(e)
	}
	o["createdAt"] = s.now().Add(-MediaClaimGrace - time.Second).UnixMilli()
	o["lastUsedAt"] = s.now().Add(-MediaClaimGrace - time.Second).UnixMilli()
	if _, e = s.writeJSON(context.Background(), "media/"+id, "editor/media/"+id+"/metadata.json", o, rev); e != nil {
		t.Fatal(e)
	}
}
func reclaimIDs(t *testing.T, o Object) []string {
	t.Helper()
	b, _ := json.Marshal(o["mediaIds"])
	var ids []string
	if e := json.Unmarshal(b, &ids); e != nil {
		t.Fatal(e)
	}
	return ids
}
func TestMediaGCGraceDryRunAndDeletion(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	m := mediaTestImport(t, s, "image.png")
	o, e := s.ReclaimUnreferencedMedia(ctx, false)
	if e != nil || len(reclaimIDs(t, o)) != 0 {
		t.Fatalf("fresh import: %v %v", o, e)
	}
	ageMediaTest(t, s, m.ID)
	dry, e := s.ReclaimUnreferencedMedia(ctx, true)
	if e != nil || !reflect.DeepEqual(reclaimIDs(t, dry), []string{m.ID}) {
		t.Fatalf("dry: %v %v", dry, e)
	}
	if _, e = s.GetMediaMetadata(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
	real, e := s.ReclaimUnreferencedMedia(ctx, false)
	if e != nil || !reflect.DeepEqual(real, dry) {
		t.Fatalf("real: %v %v", real, e)
	}
	if _, e = s.GetMediaMetadata(ctx, m.ID); e == nil {
		t.Fatal("orphan retained")
	}
}
func TestMediaGCKeepsSharedOverlayAndJobReferences(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"project", "overlay", "download", "backdrop", "template"} {
		t.Run(kind, func(t *testing.T) {
			s := newMediaTestService(t)
			m := mediaTestImport(t, s, "image.png")
			ageMediaTest(t, s, m.ID)
			switch kind {
			case "project":
				_, e := s.writeJSON(ctx, "projects/holder", "editor/projects/holder/project.json", Object{"id": "holder", "timeline": Object{"items": []any{Object{"mediaId": m.ID}}}}, "")
				if e != nil {
					t.Fatal(e)
				}
			case "overlay":
				if _, e := s.UpdateMediaMetadata(ctx, m.ID, Object{"overlay": true}); e != nil {
					t.Fatal(e)
				}
			default:
				s.MediaReferences = func(context.Context) ([]string, error) { return []string{m.ID}, nil }
			}
			o, e := s.ReclaimMediaOf(ctx, []string{m.ID})
			if e != nil || len(reclaimIDs(t, o)) != 0 {
				t.Fatalf("held: %v %v", o, e)
			}
			if kind != "project" && kind != "overlay" {
				s.MediaReferences = func(context.Context) ([]string, error) { return nil, nil }
				o, e = s.ReclaimMediaOf(ctx, []string{m.ID})
				if e != nil || len(reclaimIDs(t, o)) != 1 {
					t.Fatalf("released: %v %v", o, e)
				}
			}
		})
	}
}
func TestMediaGCUnReadableReferencesFailClosed(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	m := mediaTestImport(t, s, "image.png")
	ageMediaTest(t, s, m.ID)
	s.MediaReferences = func(context.Context) ([]string, error) { return nil, errors.New("database unavailable") }
	if _, e := s.ReclaimMediaOf(ctx, []string{m.ID}); e == nil {
		t.Fatal("reference error ignored")
	}
	if _, e := s.GetMediaMetadata(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
	s.MediaReferences = nil
	w := s.Media.(*mediaTestWorkspace)
	f := filepath.Join(w.root, "bad.json")
	if e := os.WriteFile(f, []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	w.paths["editor/projects/holder/project.json"] = f
	if _, e := s.ReclaimMediaOf(ctx, []string{m.ID}); e == nil {
		t.Fatal("unreadable project JSON ignored")
	}
}
func TestMediaGCTouchDeferredAndRelinked(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	m := mediaTestImport(t, s, "image.png")
	ageMediaTest(t, s, m.ID)
	if e := s.TouchMedia(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
	o, e := s.ReclaimMediaOf(ctx, []string{m.ID})
	if e != nil || len(reclaimIDs(t, o)) != 0 {
		t.Fatalf("claim: %v %v", o, e)
	}
	ageMediaTest(t, s, m.ID)
	o, e = s.ReclaimPendingMedia(ctx)
	if e != nil || !reflect.DeepEqual(reclaimIDs(t, o), []string{m.ID}) {
		t.Fatalf("deferred: %v %v", o, e)
	}
	o, e = s.ReclaimPendingMedia(ctx)
	if e != nil || len(reclaimIDs(t, o)) != 0 {
		t.Fatalf("settled: %v %v", o, e)
	}
}
func TestMediaGCConcurrentDeduplicatingImport(t *testing.T) {
	s := newMediaTestService(t)
	ctx := context.Background()
	m := mediaTestImport(t, s, "image.png")
	ageMediaTest(t, s, m.ID)
	var wg sync.WaitGroup
	wg.Add(2)
	var record MediaRecord
	var importErr, gcErr error
	go func() { defer wg.Done(); _, gcErr = s.ReclaimUnreferencedMedia(ctx, false) }()
	go func() {
		defer wg.Done()
		record, _, importErr = s.ImportFileOnce(ctx, ImportFileOnceOptions{File: filepath.Join("testdata", "__fixtures__", "image.png"), ContentHash: m.ContentHash, FileName: "image.png", DeclaredMIME: "image/png", MIMEPrefix: "image/"})
	}()
	wg.Wait()
	if gcErr != nil || importErr != nil {
		t.Fatalf("gc=%v import=%v", gcErr, importErr)
	}
	h, e := s.AcquireMedia(ctx, record.ID)
	if e != nil {
		t.Fatal(e)
	}
	h.Release()
}

func TestMediaGCDeferredReuseRelinkAndProjectSidecars(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	mediaTestProject(t, s, "gone")
	mediaTestProject(t, s, "keeper")
	own := mediaTestImport(t, s, "image.png")
	shared := mediaTestImport(t, s, "image.jpg")
	for _, id := range []string{own.ID, shared.ID} {
		if _, e := s.LinkMedia(ctx, "gone", id); e != nil {
			t.Fatal(e)
		}
		if e := s.TouchMedia(ctx, id); e != nil {
			t.Fatal(e)
		}
	}
	sidecar := Object{"mediaId": shared.ID}
	if _, e := s.writeJSON(ctx, "projects/gone/sidecars/animation-presets", "editor/projects/gone/animation-presets.json", sidecar, ""); e != nil {
		t.Fatal(e)
	}
	ids, e := s.ProjectMediaIDs(ctx, "gone")
	if e != nil || len(ids) != 2 {
		t.Fatalf("project ownership: %v %v", ids, e)
	}
	for _, resource := range []struct{ key, path string }{{"projects/gone/sidecars/animation-presets", "editor/projects/gone/animation-presets.json"}, {"projects/gone/links", mediaLinksPath("gone")}, {"projects/gone", "editor/projects/gone/project.json"}} {
		if e = s.removeResource(ctx, resource.key, resource.path); e != nil {
			t.Fatal(e)
		}
	}
	o, e := s.ReclaimMediaOf(ctx, ids)
	if e != nil || len(reclaimIDs(t, o)) != 0 {
		t.Fatalf("reuse lost claim: %v %v", o, e)
	}
	if _, e = s.LinkMedia(ctx, "keeper", shared.ID); e != nil {
		t.Fatal(e)
	}
	ageMediaTest(t, s, own.ID)
	ageMediaTest(t, s, shared.ID)
	o, e = s.ReclaimPendingMedia(ctx)
	if e != nil || !reflect.DeepEqual(reclaimIDs(t, o), []string{own.ID}) {
		t.Fatalf("relinked media reclaimed: %v %v", o, e)
	}
	if _, e = s.GetMediaMetadata(ctx, shared.ID); e != nil {
		t.Fatal(e)
	}
}

func TestMediaGCUnfinishedImportGraceAndExplicitLists(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	id := ID()
	logical := "editor/media/" + id + "/source.png"
	if e := s.Media.Publish(ctx, []media.Artifact{{Path: logical, File: filepath.Join("testdata", "__fixtures__", "image.png")}}, nil); e != nil {
		t.Fatal(e)
	}
	w := s.Media.(*mediaTestWorkspace)
	birth := s.now()
	if e := os.Chtimes(w.paths[logical], birth, birth); e != nil {
		t.Fatal(e)
	}
	o, e := s.ReclaimUnreferencedMedia(ctx, false)
	if e != nil || len(reclaimIDs(t, o)) != 0 {
		t.Fatalf("unfinished fresh import reclaimed: %v %v", o, e)
	}
	birth = s.now().Add(-MediaClaimGrace - time.Second)
	if e = os.Chtimes(w.paths[logical], birth, birth); e != nil {
		t.Fatal(e)
	}
	o, e = s.ReclaimMediaOf(ctx, []string{id})
	if e != nil || len(reclaimIDs(t, o)) != 0 {
		t.Fatalf("explicit list reclaimed metadata-less import: %v %v", o, e)
	}
	o, e = s.ReclaimUnreferencedMedia(ctx, true)
	if e != nil || !reflect.DeepEqual(reclaimIDs(t, o), []string{id}) {
		t.Fatalf("unfinished dry run: %v %v", o, e)
	}
	o, e = s.ReclaimUnreferencedMedia(ctx, false)
	if e != nil || !reflect.DeepEqual(reclaimIDs(t, o), []string{id}) {
		t.Fatalf("unfinished reclaim: %v %v", o, e)
	}
}

func TestProjectMediaTokensRecognizesPortableStructuralOwnership(t *testing.T) {
	ctx := context.Background()
	id := "media-0123456789abcdef0123456789abcdef"
	for _, test := range []struct {
		name string
		body Object
		held bool
	}{
		{"saved-clip", Object{"timeline": Object{"items": []any{Object{"mediaId": id}}}}, true},
		{"saved-link", Object{"mediaIds": []any{Object{"id": id, "addedAt": 1000}}}, true},
		{"portable-library-id", Object{"mediaIds": []any{"library-image_1"}}, true},
		{"cover", Object{"coverMediaId": id}, true},
		{"source", Object{"sourceMediaId": id}, true},
		{"backdrop", Object{"backdropMediaId": id}, true},
		{"unrelated-label", Object{"label": id}, false},
		{"unrelated-id", Object{"id": id}, false},
		{"invalid-path-reference", Object{"mediaId": "../" + id}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newMediaTestService(t)
			if _, err := s.writeJSON(ctx, "projects/owner", "editor/projects/owner/project.json", test.body, ""); err != nil {
				t.Fatal(err)
			}
			ids, err := s.ProjectMediaIDs(ctx, "owner")
			if err != nil {
				t.Fatal(err)
			}
			want := []string{}
			if test.held {
				want = []string{id}
				if test.name == "portable-library-id" {
					want = []string{"library-image_1"}
				}
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("structural ownership: got %v want %v", ids, want)
			}
		})
	}
}
