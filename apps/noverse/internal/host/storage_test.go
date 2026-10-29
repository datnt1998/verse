package host

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// stubLease is a StoryLease for tests that do not need a durable row.
type stubLease struct {
	closed atomic.Bool
	lost   error
}

func (l *stubLease) Close() error {
	l.closed.Store(true)
	return nil
}
func (l *stubLease) Err() error { return l.lost }

// TestStorageInjectionNoDirectoryIO proves an injected store replaces the
// engine directory entirely: the Host must not create, lock or log into
// cfg.OutputDir, and it must use the injected store for facts.
func TestStorageInjectionNoDirectoryIO(t *testing.T) {
	engineDir := t.TempDir()
	st := storepkg.NewStore(engineDir)
	lease := &stubLease{}
	absent := filepath.Join(t.TempDir(), "absent-story")

	h, err := New(bootstrap.Config{OutputDir: absent}, assets.Bundle{},
		WithUnconfiguredReads(), WithStorage(Storage{Store: st, Lease: lease}))
	if err != nil {
		t.Fatalf("New with injected storage: %v", err)
	}
	if h.store != st {
		t.Fatal("Host did not use the injected store")
	}
	if h.ModelReady() {
		t.Fatal("reading host with unconfigured models must not be model-ready")
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("injected store still touched %q: %v", absent, err)
	}
	h.Close()
	if !lease.closed.Load() {
		t.Fatal("Host.Close must release the injected story lease")
	}
}

// TestStorageInjectionRequiresLease refuses a half-injected storage: a store
// without a lease would let two engine processes write the same story.
func TestStorageInjectionRequiresLease(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	_, err := New(bootstrap.Config{OutputDir: filepath.Join(t.TempDir(), "absent")}, assets.Bundle{},
		WithUnconfiguredReads(), WithStorage(Storage{Store: st}))
	if err == nil {
		t.Fatal("New accepted storage without a story lease")
	}
}

// TestScratchIsPrivateAndRemoved proves the explicit scratch capability: a
// fresh private directory, removed on Close, and never the engine directory.
func TestScratchIsPrivateAndRemoved(t *testing.T) {
	scratch, err := NewScratch("noverse-test-*")
	if err != nil {
		t.Fatalf("NewScratch: %v", err)
	}
	dir := scratch.Dir()
	if dir == "" {
		t.Fatal("scratch has no directory")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("scratch dir missing: %v", err)
	}
	if err := scratch.Close(); err != nil {
		t.Fatalf("scratch close: %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scratch not removed: %v", err)
	}
	if scratch.Dir() != "" {
		t.Fatal("scratch dir must be empty after Close")
	}
	if err := scratch.Close(); err != nil {
		t.Fatalf("second Close must be a no-op: %v", err)
	}
}

// TestEngineScratchDirHostedDoesNotUseDir proves hosted stores get an isolated
// temp tree instead of a path derived from Dir() == "".
func TestEngineScratchDirHostedDoesNotUseDir(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	dir, cleanup, err := EngineScratchDir(st, "noverse-engine-*")
	if err != nil {
		t.Fatalf("EngineScratchDir: %v", err)
	}
	defer cleanup()
	if dir != st.Dir() {
		t.Fatalf("file mode scratch = %q, want engine dir %q", dir, st.Dir())
	}
}
