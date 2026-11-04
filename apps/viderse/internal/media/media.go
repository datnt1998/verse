// Package media owns durable artifacts and their working-file lifetimes.
// Resource authorization belongs to callers, database visibility to Repository.
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/platform/blobstore"
)

var ErrConflict = errors.New("dữ liệu lưu trữ đã thay đổi")
var ErrBusy = errors.New("đang có tác vụ lưu trữ khác")

func ContentKey(sum string) (string, error) {
	if len(sum) != 64 || strings.ToLower(sum) != sum {
		return "", blobstore.ErrInvalidKey
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return "", blobstore.ErrInvalidKey
	}
	return "v1/sha256/" + sum[:2] + "/" + sum, nil
}
func ValidatePath(path string) error { return blobstore.ValidateKey(path) }

type Descriptor struct {
	ID          string
	Config      blobstore.Config
	Credentials []byte
}
type State struct {
	Active, Pending, Previous string
	Revision, Generation      int64
}
type Ref struct {
	Path, Key, SHA256, ContentType, Filename, SourcePath, StoreID string
	Bytes                                                         int64
	VerifiedAt                                                    time.Time
}
type Root struct{ Path, ContentType, Group string }
type Artifact struct{ Path, File, ContentType, Filename string }
type Handle struct {
	Path    string
	Ref     Ref
	release func()
	once    sync.Once
}

func (h *Handle) Release() {
	if h != nil {
		h.once.Do(func() {
			if h.release != nil {
				h.release()
			}
		})
	}
}

// Repository callbacks run in one transaction. The context carries that
// transaction to domain stores; none of the callbacks may commit separately.
type Repository interface {
	Initialize(context.Context, Descriptor) (State, error)
	State(context.Context) (State, error)
	Descriptor(context.Context, string) (Descriptor, error)
	Configure(context.Context, Descriptor, int64, bool) (State, error)
	Publication(context.Context, func(context.Context, State) error) error
	Lock(context.Context, string, string) error
	Bind(context.Context, Ref) error
	Resolve(context.Context, string, string) (Ref, error)
	// Unbind gỡ binding logical_path → object_key trong publication tx;
	// idempotent (vắng mặt = nil). Content chỉ thành unreferenced khi mọi
	// binding và live ref biến mất; GC thu hồi byte sau ân hạn.
	Unbind(context.Context, string, string) error
	Roots(context.Context) ([]Root, error)
	Protect(context.Context, string, string, time.Time) error
	ReadLease(context.Context, string, string) (func(), error)
}

type Service struct {
	repo   Repository
	cache  *Cache
	open   func(Descriptor) (blobstore.BlobStore, error)
	mu     sync.Mutex
	stores map[string]blobstore.BlobStore
}

func New(repo Repository, cache *Cache, open func(Descriptor) (blobstore.BlobStore, error)) *Service {
	return &Service{repo: repo, cache: cache, open: open, stores: make(map[string]blobstore.BlobStore)}
}
func (s *Service) Repository() Repository { return s.repo }
func (s *Service) Cache() *Cache          { return s.cache }
func (s *Service) Store(ctx context.Context, id string) (blobstore.BlobStore, Descriptor, error) {
	d, err := s.repo.Descriptor(ctx, id)
	if err != nil {
		return nil, d, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if store := s.stores[id]; store != nil {
		return store, d, nil
	}
	store, err := s.open(d)
	if err != nil {
		return nil, d, err
	}
	s.stores[id] = store
	return store, d, nil
}
func (s *Service) Stores() []blobstore.BlobStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]blobstore.BlobStore, 0, len(s.stores))
	for _, st := range s.stores {
		out = append(out, st)
	}
	return out
}
func (s *Service) Active(ctx context.Context) (blobstore.BlobStore, Descriptor, State, error) {
	state, err := s.repo.State(ctx)
	if err != nil {
		return nil, Descriptor{}, state, err
	}
	st, d, err := s.Store(ctx, state.Active)
	return st, d, state, err
}

// stagedSuffix marks a local publication whose bytes are already durable but
// whose logical path must not exist yet. Inventory skips these files, so a
// crash before the reference commits can never leave an unexplained durable
// path behind.
const stagedSuffix = ".part"

// stagedPublication is one local artifact waiting for its reference to commit.
type stagedPublication struct{ root, staged, target, sum string }

// Publish selects the active generation only after all preprocessing is done.
// Bytes are verified before manifest + domain pointers commit atomically.
// A local logical path is linked only after that commit: the tree can be
// missing bytes for a committed reference (repaired at start-up), but it can
// never hold bytes without a reference.
func (s *Service) Publish(ctx context.Context, artifacts []Artifact, commit func(context.Context) error) error {
	refs := make([]Ref, len(artifacts))
	for i, a := range artifacts {
		if err := ValidatePath(a.Path); err != nil {
			return err
		}
		f, err := os.Open(a.File)
		if err != nil {
			return err
		}
		sum, n, err := blobstore.DigestAll(f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		key, err := ContentKey(sum)
		if err != nil {
			return err
		}
		refs[i] = Ref{Path: a.Path, Key: key, SHA256: sum, Bytes: n, ContentType: a.ContentType, Filename: a.Filename}
	}
	staged := make([]stagedPublication, 0, len(artifacts))
	err := s.repo.Publication(ctx, func(txctx context.Context, state State) error {
		store, d, err := s.Store(txctx, state.Active)
		if err != nil {
			return err
		}
		keys := make([]string, len(refs))
		for i, ref := range refs {
			keys[i] = ref.Key
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := s.repo.Lock(txctx, state.Active, key); err != nil {
				return err
			}
		}
		// One writer per logical path: a staged file is named after its target,
		// so two concurrent publications of the same path must not interleave.
		for _, a := range artifacts {
			if err := s.repo.Lock(txctx, state.Active, "path/"+a.Path); err != nil {
				return err
			}
		}
		for i, a := range artifacts {
			ref := refs[i]
			ref.StoreID = state.Active
			if d.Config.Driver == "local" {
				entry, err := stageLocal(d.Config.LocalRoot, a.File, a.Path, ref.SHA256)
				if err != nil {
					return err
				}
				if entry != nil {
					staged = append(staged, *entry)
				}
				ref.SourcePath = a.Path
			} else {
				f, err := os.Open(a.File)
				if err != nil {
					return err
				}
				_, err = store.Put(txctx, ref.Key, f, blobstore.PutOptions{ExpectedSHA256: ref.SHA256, ContentType: ref.ContentType})
				closeErr := f.Close()
				if err != nil {
					return err
				}
				if closeErr != nil {
					return closeErr
				}
				if _, err := blobstore.Verify(txctx, store, ref.Key, ref.SHA256, ref.Bytes); err != nil {
					return err
				}
			}
			ref.VerifiedAt = time.Now().UTC()
			if err := s.repo.Bind(txctx, ref); err != nil {
				return err
			}
		}
		if commit != nil {
			return commit(txctx)
		}
		return nil
	})
	if err != nil {
		discardStaged(staged)
		return err
	}
	// The references are durable now, so the logical paths may appear. A crash
	// in between is repaired by RepairLocalPublications at the next start.
	for _, entry := range staged {
		if err := linkStaged(entry); err != nil {
			return err
		}
	}
	return nil
}

// RepairLocalPublications finishes local publications that were interrupted
// after the reference commit and drops staged bytes whose reference never
// committed. It never invents a path: uncommitted bytes are removed, and a
// committed reference without bytes is completed only when the staged digest
// still matches the reference.
func (s *Service) RepairLocalPublications(ctx context.Context) (RepairReport, error) {
	report := RepairReport{}
	state, err := s.repo.State(ctx)
	if err != nil {
		return report, err
	}
	d, err := s.repo.Descriptor(ctx, state.Active)
	if err != nil {
		return report, err
	}
	if d.Config.Driver != "local" {
		return report, nil
	}
	root, err := os.OpenRoot(d.Config.LocalRoot)
	if errors.Is(err, os.ErrNotExist) {
		return report, nil
	}
	if err != nil {
		return report, err
	}
	defer root.Close()
	staged := []string{}
	err = fs.WalkDir(root.FS(), ".", func(rel string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(entry.Name(), stagedSuffix) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("tệp tạm không thường trong dữ liệu bền vững: %s", rel)
		}
		staged = append(staged, rel)
		return nil
	})
	if err != nil {
		return report, err
	}
	for _, rel := range staged {
		target := strings.TrimSuffix(rel, stagedSuffix)
		if ValidatePath(target) != nil {
			report.Skipped = append(report.Skipped, rel)
			continue
		}
		if _, err := root.Stat(target); err == nil {
			// The link already happened; the staged copy is a leftover.
			if err := root.Remove(rel); err != nil && !errors.Is(err, os.ErrNotExist) {
				return report, err
			}
			report.Dropped++
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return report, err
		}
		ref, err := s.repo.Resolve(ctx, state.Active, target)
		if errors.Is(err, blobstore.ErrNotFound) {
			if err := root.Remove(rel); err != nil && !errors.Is(err, os.ErrNotExist) {
				return report, err
			}
			report.Dropped++
			continue
		}
		if err != nil {
			return report, err
		}
		file, err := root.Open(rel)
		if err != nil {
			return report, err
		}
		sum, _, err := blobstore.DigestAll(file)
		file.Close()
		if err != nil {
			return report, err
		}
		if sum != ref.SHA256 {
			// Never place bytes that no reference describes.
			report.Skipped = append(report.Skipped, rel)
			continue
		}
		if err := linkStaged(stagedPublication{root: d.Config.LocalRoot, staged: rel, target: target, sum: ref.SHA256}); err != nil {
			return report, err
		}
		report.Linked++
	}
	return report, nil
}

// RepairReport counts the start-up repair of interrupted local publications.
type RepairReport struct {
	Linked  int      `json:"linked"`
	Dropped int      `json:"dropped"`
	Skipped []string `json:"skipped,omitempty"`
}

func (s *Service) Resolve(ctx context.Context, path string) (blobstore.BlobStore, Descriptor, Ref, func(), error) {
	state, err := s.repo.State(ctx)
	if err != nil {
		return nil, Descriptor{}, Ref{}, nil, err
	}
	return s.ResolveAt(ctx, state.Active, path)
}
func (s *Service) ResolveAt(ctx context.Context, storeID, path string) (blobstore.BlobStore, Descriptor, Ref, func(), error) {
	if err := ValidatePath(path); err != nil {
		return nil, Descriptor{}, Ref{}, nil, err
	}
	ref, err := s.repo.Resolve(ctx, storeID, path)
	if err != nil {
		return nil, Descriptor{}, ref, nil, err
	}
	store, d, err := s.Store(ctx, storeID)
	if err != nil {
		return nil, d, ref, nil, err
	}
	release, err := s.repo.ReadLease(ctx, storeID, ref.Key)
	if err != nil {
		return nil, d, ref, nil, err
	}
	if d.Config.Driver == "local" {
		// Local mode preserves logical layout even when two paths deduplicate
		// to the same digest. A path must never depend on another owner's file.
		ref.SourcePath = ref.Path
		store = &locatedStore{BlobStore: store, key: ref.Key, path: ref.SourcePath}
	}
	return store, d, ref, release, nil
}
func (s *Service) Acquire(ctx context.Context, path string) (*Handle, error) {
	store, d, ref, release, err := s.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	if d.Config.Driver == "local" {
		root, err := os.OpenRoot(d.Config.LocalRoot)
		if err != nil {
			release()
			return nil, err
		}
		f, err := root.Open(ref.SourcePath)
		root.Close()
		if err != nil {
			release()
			return nil, err
		}
		info, err := f.Stat()
		f.Close()
		if err != nil || !info.Mode().IsRegular() {
			release()
			return nil, blobstore.ErrNotFound
		}
		return &Handle{Path: filepath.Join(d.Config.LocalRoot, filepath.FromSlash(ref.SourcePath)), Ref: ref, release: release}, nil
	}
	handle, err := s.cache.Acquire(ctx, store, ref)
	if err != nil {
		release()
		return nil, err
	}
	unpin := handle.release
	handle.release = func() {
		if unpin != nil {
			unpin()
		}
		release()
	}
	return handle, nil
}
func (s *Service) Protect(ctx context.Context, ref Ref, until time.Time) error {
	return s.repo.Protect(ctx, ref.StoreID, ref.Key, until)
}

// Unbind gỡ các logical path khỏi manifest trong một publication transaction
// (cùng fence với Bind/GC). Commit chạy trước vòng gỡ binding vì nó xoá hàng
// domain sở hữu path (clone_artifacts, editor head…): binding còn bị FK RESTRICT
// giữ cho tới khi chủ của nó biến mất. Path vẫn còn chủ sống (head editor của
// project khác, artifact/cutout của job khác) thì repository giữ binding lại —
// đường nhả canonical của chủ cuối mới xoá nó. Lỗi ở commit để nguyên binding;
// lỗi khi gỡ binding rollback luôn commit. Idempotent: path vắng mặt không lỗi.
func (s *Service) Unbind(ctx context.Context, paths []string, commit func(context.Context) error) error {
	return s.repo.Publication(ctx, func(txctx context.Context, state State) error {
		for _, path := range paths {
			if err := ValidatePath(path); err != nil {
				return err
			}
		}
		if commit != nil {
			if err := commit(txctx); err != nil {
				return err
			}
		}
		for _, path := range paths {
			if err := s.repo.Unbind(txctx, state.Active, path); err != nil {
				return err
			}
		}
		return nil
	})
}

type locatedStore struct {
	blobstore.BlobStore
	key, path string
}

func (s *locatedStore) Open(ctx context.Context, key string, r blobstore.ByteRange) (io.ReadCloser, blobstore.BlobInfo, error) {
	if key != s.key {
		return nil, blobstore.BlobInfo{}, blobstore.ErrInvalidKey
	}
	return s.BlobStore.Open(ctx, s.path, r)
}
func (s *locatedStore) Stat(ctx context.Context, key string) (blobstore.BlobInfo, error) {
	if key != s.key {
		return blobstore.BlobInfo{}, blobstore.ErrInvalidKey
	}
	return s.BlobStore.Stat(ctx, s.path)
}
func (s *locatedStore) DriverName() string { return "local" }

// stageLocal copies bytes next to their logical path without making that path
// visible. A nil result means the path already holds the same bytes.
func stageLocal(root, from, rel, sum string) (*stagedPublication, error) {
	if err := ValidatePath(rel); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	parent := filepath.ToSlash(filepath.Dir(rel))
	if err := r.MkdirAll(parent, 0755); err != nil {
		return nil, err
	}
	if existing, err := r.Open(rel); err == nil {
		digest, _, readErr := blobstore.DigestAll(existing)
		existing.Close()
		if readErr != nil {
			return nil, readErr
		}
		if digest == sum {
			return nil, nil
		}
		return nil, blobstore.ErrDigestMismatch
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	staged := rel + stagedSuffix
	// A staged file for this path belongs to an interrupted publication: its
	// bytes were never referenced, and the path lock serialises writers.
	if err := r.Remove(staged); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	out, err := r.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	src, err := os.Open(from)
	if err != nil {
		out.Close()
		_ = r.Remove(staged)
		return nil, err
	}
	_, err = io.Copy(out, src)
	src.Close()
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = r.Remove(staged)
		return nil, err
	}
	return &stagedPublication{root: root, staged: staged, target: rel, sum: sum}, nil
}

// linkStaged makes a committed logical path visible. Bytes already there must
// match, so a stale staged copy can never overwrite another writer's file.
func linkStaged(entry stagedPublication) error {
	r, err := os.OpenRoot(entry.root)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := r.Link(entry.staged, entry.target); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		existing, err := r.Open(entry.target)
		if err != nil {
			return err
		}
		digest, _, err := blobstore.DigestAll(existing)
		existing.Close()
		if err != nil {
			return err
		}
		if digest != entry.sum {
			return blobstore.ErrDigestMismatch
		}
	}
	if err := r.Remove(entry.staged); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// discardStaged drops bytes whose references never committed.
func discardStaged(entries []stagedPublication) {
	for _, entry := range entries {
		r, err := os.OpenRoot(entry.root)
		if err != nil {
			continue
		}
		_ = r.Remove(entry.staged)
		r.Close()
	}
}

// NamespaceID contains no credentials, so key rotation preserves store identity.
func NamespaceID(config blobstore.Config) string {
	value := config.Driver + "\x00" + config.LocalRoot
	if config.Driver == "s3" {
		value = config.Driver + "\x00" + config.S3.Endpoint + "\x00" + config.S3.Region + "\x00" + config.S3.Bucket
	}
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:16])
}
