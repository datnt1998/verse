package host

import (
	"fmt"
	"os"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// StoryLease is the exclusive write right for one story run: an OS file lock in
// file mode, a renewable fencing row in postgres. The Host holds it for its
// whole lifetime; losing it cancels the run instead of letting a stale writer
// commit. Err reports the loss reason and is nil while the lease is held.
type StoryLease interface {
	Close() error
	Err() error
}

// Storage injects the backend-scoped store and its lease into the Host. It is
// how a hosted run (STORE_DRIVER=postgres) gets a store without an engine
// directory: the Host must not create, lock or log into cfg.OutputDir once a
// store has been injected.
type Storage struct {
	Store *storepkg.Store
	Lease StoryLease
}

// Validate refuses a half-provided injection: a store without a lease would let
// two engine processes write the same story.
func (s Storage) Validate() error {
	if s.Store == nil {
		return fmt.Errorf("storage được tiêm thiếu store: %w", errs.ErrConfig)
	}
	if s.Lease == nil {
		return fmt.Errorf("storage được tiêm thiếu lease của truyện: %w", errs.ErrConfig)
	}
	return nil
}

// WithStorage injects an already-scoped store plus its story lease. The Host
// then skips the file lease, the on-disk runtime log and store construction —
// all business IO must go through the injected Store.
func WithStorage(s Storage) NewOption {
	return func(opts *newOptions) { opts.storage = &s }
}

// closeLease releases a lease that may be absent (read host without injection).
func closeLease(l StoryLease) error {
	if l == nil {
		return nil
	}
	return l.Close()
}

// leaseErr reports a lost lease (nil when held or absent).
func leaseErr(l StoryLease) error {
	if l == nil {
		return nil
	}
	return l.Err()
}

// Scratch is explicit, private, bounded temporary space for one operation
// (ffmpeg input, import analysis). It is never a content authority and never a
// watcher source; Close removes it. Hosted code uses it instead of writing
// beside the engine documents.
type Scratch struct {
	dir string
}

// NewScratch creates a private temporary directory for one operation.
func NewScratch(pattern string) (*Scratch, error) {
	if pattern == "" {
		pattern = "noverse-scratch-*"
	}
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return nil, fmt.Errorf("tạo thư mục tạm: %w", err)
	}
	return &Scratch{dir: dir}, nil
}

// Dir is the scratch directory; empty after Close.
func (s *Scratch) Dir() string {
	if s == nil {
		return ""
	}
	return s.dir
}

// Close removes the scratch tree. It is idempotent.
func (s *Scratch) Close() error {
	if s == nil || s.dir == "" {
		return nil
	}
	dir := s.dir
	s.dir = ""
	return os.RemoveAll(dir)
}

// EngineScratchDir returns a bounded scratch directory for one engine
// operation. File mode keeps using the engine directory when the caller already
// owns it; hosted mode always gets a fresh private tree instead of a path
// derived from Dir() == "".
func EngineScratchDir(s *storepkg.Store, pattern string) (string, func(), error) {
	if dir, ok := s.FileRoot(); ok && dir != "" {
		return dir, func() {}, nil
	}
	scratch, err := NewScratch(pattern)
	if err != nil {
		return "", nil, err
	}
	return scratch.Dir(), func() { _ = scratch.Close() }, nil
}
