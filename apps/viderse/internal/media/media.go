// Package media owns durable artifacts and their working-file lifetimes.
// Resource authorization belongs to callers, database visibility to Repository.
package media

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

// Publish selects the active generation only after all preprocessing is done.
// Bytes are verified before manifest + domain pointers commit atomically.
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
	return s.repo.Publication(ctx, func(txctx context.Context, state State) error {
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
		for i, a := range artifacts {
			ref := refs[i]
			ref.StoreID = state.Active
			if d.Config.Driver == "local" {
				if err := publishLocal(d.Config.LocalRoot, a.File, a.Path, ref.SHA256); err != nil {
					return err
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

func publishLocal(root, from, rel, sum string) error {
	if err := ValidatePath(rel); err != nil {
		return err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer r.Close()
	parent := filepath.ToSlash(filepath.Dir(rel))
	if err := r.MkdirAll(parent, 0755); err != nil {
		return err
	}
	if existing, err := r.Open(rel); err == nil {
		digest, _, readErr := blobstore.DigestAll(existing)
		existing.Close()
		if readErr != nil {
			return readErr
		}
		if digest == sum {
			return nil
		}
		return blobstore.ErrDigestMismatch
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	tmp := parent + fmt.Sprintf("/.publish-%x.part", token)
	out, err := r.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer r.Remove(tmp)
	src, err := os.Open(from)
	if err != nil {
		out.Close()
		return err
	}
	_, err = io.Copy(out, src)
	src.Close()
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := r.Link(tmp, rel); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		existing, err := r.Open(rel)
		if err != nil {
			return err
		}
		digest, _, err := blobstore.DigestAll(existing)
		existing.Close()
		if err != nil {
			return err
		}
		if digest != sum {
			return blobstore.ErrDigestMismatch
		}
	}
	return nil
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
