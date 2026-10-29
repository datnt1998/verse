package store

import (
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

// fakeIncompleteBackend is the phase-02 stand-in for the Postgres backend:
// it cannot supply any domain, so the facade must refuse to wrap it instead
// of leaving nil substores that panic later.
type fakeIncompleteBackend struct{ dir string }

func (b *fakeIncompleteBackend) Kind() BackendKind { return BackendPostgres }
func (b *fakeIncompleteBackend) Dir() string       { return b.dir }
func (b *fakeIncompleteBackend) Close() error      { return nil }
func (b *fakeIncompleteBackend) Docs() *IO         { return NewIO(b.dir) }
func (b *fakeIncompleteBackend) Comments() *comments.Store {
	return nil // no comment surface on the incomplete backend
}
func (b *fakeIncompleteBackend) Substores() Substores {
	return Substores{} // no domains implemented yet
}
func (b *fakeIncompleteBackend) LoadProjectFormatVersion() (int, error) {
	return 0, &CapabilityError{Backend: BackendPostgres, Op: "LoadProjectFormatVersion"}
}
func (b *fakeIncompleteBackend) SaveProjectFormatVersion(int) error {
	return &CapabilityError{Backend: BackendPostgres, Op: "SaveProjectFormatVersion"}
}
func (b *fakeIncompleteBackend) Init() error {
	return &CapabilityError{Backend: BackendPostgres, Op: "Init"}
}
func (b *fakeIncompleteBackend) FoundationFingerprint() (string, error) {
	return "", &CapabilityError{Backend: BackendPostgres, Op: "FoundationFingerprint"}
}
func (b *fakeIncompleteBackend) ExpandArc(int, int, domain.ArcExpansion) error {
	return &CapabilityError{Backend: BackendPostgres, Op: "ExpandArc"}
}
func (b *fakeIncompleteBackend) AppendVolume(domain.VolumeOutline) error {
	return &CapabilityError{Backend: BackendPostgres, Op: "AppendVolume"}
}
func (b *fakeIncompleteBackend) ReviseOutline(int, []domain.OutlineEntry) (int, error) {
	return 0, &CapabilityError{Backend: BackendPostgres, Op: "ReviseOutline"}
}
func (b *fakeIncompleteBackend) ClearHandledSteer() error {
	return &CapabilityError{Backend: BackendPostgres, Op: "ClearHandledSteer"}
}
func (b *fakeIncompleteBackend) InvalidateChapterAggregates(int) error {
	return &CapabilityError{Backend: BackendPostgres, Op: "InvalidateChapterAggregates"}
}

// TestBackendFileDefaults locks the file backend as the default constructor:
// same dir identity, real substores, and the os.IsNotExist distinction that
// empty-state loads rely on (nil result, not an error).
func TestBackendFileDefaults(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
	if s.Dir() != dir {
		t.Fatalf("Dir() = %q, want %q", s.Dir(), dir)
	}
	if s.BackendKind() != BackendFile {
		t.Fatalf("BackendKind() = %q, want file", s.BackendKind())
	}
	// empty-state loads: missing artifacts are (nil, nil), not errors — the
	// file adapter must preserve the not-found distinction callers depend on.
	if book, err := s.Book.Load(); err != nil || book != nil {
		t.Fatalf("Book.Load on empty dir = (%v, %v); want (nil, nil)", book, err)
	}
	if outline, err := s.Outline.LoadOutline(); err != nil || len(outline) != 0 {
		t.Fatalf("LoadOutline on empty dir = (%v, %v); want (empty, nil)", outline, err)
	}
}

// TestBackendCapabilityGate proves NewFromBackend refuses a backend that
// cannot supply all 19 domains — the startup-fail requirement, not stubs.
func TestBackendCapabilityGate(t *testing.T) {
	_, err := NewFromBackend(&fakeIncompleteBackend{})
	if err == nil {
		t.Fatal("NewFromBackend accepted backend with no domains")
	}
	var capErr *CapabilityError
	if !errors.As(err, &capErr) {
		t.Fatalf("NewFromBackend error = %v, want *CapabilityError", err)
	}
}

// TestBackendFileFromConstructor proves the backend path and NewStore produce
// the same working facade — the two construction roots stay consistent.
func TestBackendFileFromConstructor(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFromBackend(newFileBackend(dir))
	if err != nil {
		t.Fatalf("NewFromBackend(file): %v", err)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(4); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	p, err := s.Progress.Load()
	if err != nil {
		t.Fatalf("Progress.Load: %v", err)
	}
	if p == nil || p.TotalChapters != 4 {
		t.Fatalf("progress = %+v, want TotalChapters=4", p)
	}
}

// TestCompoundIdempotentReplay proves the compound coordination path kept its
// semantics through the backend split: re-running the same AppendVolume is a
// no-op, not a duplicate.
func TestCompoundIdempotentReplay(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	vol := domain.VolumeOutline{
		Index: 1,
		Title: "tập một",
		Arcs: []domain.ArcOutline{{
			Index: 1,
			Title: "cung một",
			Goal:  "mở đầu",
			Chapters: []domain.OutlineEntry{
				{Chapter: 1, Title: "chương 1", CoreEvent: "sự kiện 1"},
				{Chapter: 2, Title: "chương 2", CoreEvent: "sự kiện 2"},
			},
		}},
	}
	if err := s.Outline.SaveLayeredOutline(nil); err != nil {
		t.Fatalf("seed layered outline: %v", err)
	}
	if err := s.AppendVolume(vol); err != nil {
		t.Fatalf("AppendVolume: %v", err)
	}
	if err := s.AppendVolume(vol); err != nil {
		t.Fatalf("AppendVolume replay: %v", err)
	}
	volumes, err := s.Outline.LoadLayeredOutline()
	if err != nil {
		t.Fatalf("LoadLayeredOutline: %v", err)
	}
	if len(volumes) != 1 {
		t.Fatalf("replay duplicated volume: %d volumes", len(volumes))
	}
}

// TestStoreErrorsSurfaceTyped proves domain-level sentinels from errs are
// preserved through the facade, not swallowed into opaque strings.
func TestStoreErrorsSurfaceTyped(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// ReviseOutline on an uninitialised outline must surface the typed
	// precondition error, not a raw filesystem error.
	_, err := s.ReviseOutline(1, []domain.OutlineEntry{{Chapter: 1, Title: "a"}})
	if err == nil {
		t.Fatal("ReviseOutline on empty progress must fail")
	}
	if !errors.Is(err, errs.ErrToolPrecondition) && !errors.Is(err, errs.ErrStoreRead) {
		t.Fatalf("ReviseOutline error = %v, want typed store/tool error", err)
	}
}
