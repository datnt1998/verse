package media_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// memRepository is the smallest Repository that can publish: references live in
// memory, and each Publication callback runs inside one "transaction" the test
// controls.
type memRepository struct {
	cfg    blobstore.Config
	refs   map[string]media.Ref
	fail   error
	commit func(context.Context, func(context.Context, media.State) error) error
}

func (r *memRepository) State(context.Context) (media.State, error) {
	return media.State{Active: "local"}, nil
}
func (r *memRepository) Descriptor(context.Context, string) (media.Descriptor, error) {
	return media.Descriptor{ID: "local", Config: r.cfg}, nil
}
func (r *memRepository) Publication(ctx context.Context, fn func(context.Context, media.State) error) error {
	if r.commit != nil {
		return r.commit(ctx, fn)
	}
	if err := fn(ctx, media.State{Active: "local"}); err != nil {
		return err
	}
	return r.fail
}
func (r *memRepository) Lock(context.Context, string, string) error { return nil }
func (r *memRepository) Bind(_ context.Context, ref media.Ref) error {
	if r.refs == nil {
		r.refs = map[string]media.Ref{}
	}
	r.refs[ref.Path] = ref
	return nil
}
func (r *memRepository) Resolve(_ context.Context, _, path string) (media.Ref, error) {
	ref, ok := r.refs[path]
	if !ok {
		return media.Ref{}, blobstore.ErrNotFound
	}
	return ref, nil
}
func (r *memRepository) Unbind(_ context.Context, _, path string) error {
	delete(r.refs, path)
	return nil
}
func (r *memRepository) Initialize(context.Context, media.Descriptor) (media.State, error) {
	return media.State{Active: "local"}, nil
}
func (r *memRepository) Configure(context.Context, media.Descriptor, int64, bool) (media.State, error) {
	return media.State{Active: "local"}, nil
}
func (r *memRepository) Roots(context.Context) ([]media.Root, error) { return nil, nil }
func (r *memRepository) Protect(context.Context, string, string, time.Time) error {
	return nil
}
func (r *memRepository) ReadLease(context.Context, string, string) (func(), error) {
	return func() {}, nil
}

func newPublisher(t *testing.T) (*media.Service, *memRepository, string) {
	t.Helper()
	root := t.TempDir()
	repo := &memRepository{cfg: blobstore.Config{Driver: "local", LocalRoot: root}, refs: map[string]media.Ref{}}
	store := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) {
		return blobstore.Open(d.Config)
	})
	return store, repo, root
}

func sourceFile(t *testing.T, payload string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.bin")
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A crash after the bytes are written but before the reference commits must
// leave no durable logical path: the tree would otherwise hold a file that no
// root adapter can explain, which is what kept the server from starting.
func TestPublishHidesLogicalPathUntilReferenceCommits(t *testing.T) {
	store, repo, root := newPublisher(t)
	const rel = "clone/job/abc/editor/media/media-1/metadata.json"
	repo.commit = func(ctx context.Context, fn func(context.Context, media.State) error) error {
		if err := fn(ctx, media.State{Active: "local"}); err != nil {
			return err
		}
		// The process dies here: no commit, no deferred cleanup.
		panic("killed")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("crash simulation did not run")
			}
		}()
		_ = store.Publish(context.Background(), []media.Artifact{{Path: rel, File: sourceFile(t, "bytes")}}, nil)
	}()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("logical path exists before the reference committed: %v", err)
	}
	staged, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(rel)) + ".part")
	if err != nil || len(staged) != 1 {
		t.Fatalf("staged bytes missing: %v %v", staged, err)
	}
}

func TestPublishRevealsLogicalPathAfterCommit(t *testing.T) {
	store, _, root := newPublisher(t)
	const rel = "library/overlays/one.mp4"
	if err := store.Publish(context.Background(), []media.Artifact{{Path: rel, File: sourceFile(t, "bytes")}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("committed path missing: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(rel)) + ".part"); len(left) != 0 {
		t.Fatalf("staged bytes left behind: %v", left)
	}
}

func TestPublishDropsStagedBytesWhenTheTransactionRollsBack(t *testing.T) {
	store, repo, root := newPublisher(t)
	repo.fail = errors.New("rollback")
	const rel = "library/overlays/two.mp4"
	if err := store.Publish(context.Background(), []media.Artifact{{Path: rel, File: sourceFile(t, "bytes")}}, nil); err == nil {
		t.Fatal("rollback was reported as success")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rolled back publication left a path: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(rel)) + ".part"); len(left) != 0 {
		t.Fatalf("rolled back publication left staged bytes: %v", left)
	}
}

// Start-up repair finishes the link the crash interrupted, and never promotes
// bytes whose reference does not exist.
func TestRepairLocalPublicationsCompletesCommittedLinkAndDropsUncommitted(t *testing.T) {
	store, repo, root := newPublisher(t)
	committed := "clone/job/abc/abc/editor/media/media-1/metadata.json"
	uncommitted := "clone/job/abc/def/editor/media/media-2/metadata.json"
	for _, rel := range []string{committed, uncommitted} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+".part", []byte(rel), 0600); err != nil {
			t.Fatal(err)
		}
	}
	payload := []byte(committed)
	repo.refs[committed] = media.Ref{Path: committed, StoreID: "local", SHA256: digestOf(t, payload), Bytes: int64(len(payload))}
	report, err := store.RepairLocalPublications(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Linked != 1 || report.Dropped != 1 {
		t.Fatalf("unexpected repair report: %+v", report)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(committed)))
	if err != nil || string(raw) != committed {
		t.Fatalf("committed reference was not completed: %q %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(uncommitted))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("uncommitted bytes became a durable path: %v", err)
	}
}

func TestRepairLocalPublicationsKeepsMismatchedBytes(t *testing.T) {
	store, repo, root := newPublisher(t)
	const rel = "library/overlays/three.mp4"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".part", []byte("other bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	repo.refs[rel] = media.Ref{Path: rel, StoreID: "local", SHA256: digestOf(t, []byte("expected"))}
	report, err := store.RepairLocalPublications(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Linked != 0 || len(report.Skipped) != 1 {
		t.Fatalf("mismatched bytes were promoted: %+v", report)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mismatched bytes reached the logical path: %v", err)
	}
}

func digestOf(t *testing.T, payload []byte) string {
	t.Helper()
	sum, _, err := blobstore.DigestAll(strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	return sum
}
