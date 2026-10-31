package library

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

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
	descriptor   media.Descriptor
	refs         map[string]media.Ref
	publications int
	failure      error
}

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
	repository := &templateMediaRepository{descriptor: media.Descriptor{ID: "local", Config: blobstore.Config{Driver: "local", LocalRoot: root}}, refs: map[string]media.Ref{}}
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
func (r *templateMediaRepository) ReadLease(_ context.Context, store, key string) (func(), error) {
	if store != r.descriptor.ID || blobstore.ValidateKey(key) != nil {
		return nil, blobstore.ErrInvalidKey
	}
	// No collector runs in these tests; the immutable local file remains live.
	return func() {}, nil
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
