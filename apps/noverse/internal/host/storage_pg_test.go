package host

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// pgAnchorUUID derives a stable UUID for a test row (same shape as phase 04).
func pgAnchorUUID(t *testing.T, ns, name string) pgtype.UUID {
	t.Helper()
	sum := sha256.Sum256([]byte("ainovel|test|" + ns + "|" + name))
	var id pgtype.UUID
	copy(id.Bytes[:], sum[:16])
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x50
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id
}

// pgEngineStore provisions a migrated schema with one story, claims its durable
// lease and returns the scoped facade plus the lease. Every write through the
// facade is fenced on the lease, exactly like a hosted engine run.
func pgEngineStore(t *testing.T, chapters int) (*storepkg.Store, *postgres.StoryLease) {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	_ = db.Close()

	ctx := context.Background()
	pool, err := postgres.Open(ctx, postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	q := gen.New(pool.Pool())
	if _, err := q.EnsureProject(ctx, gen.EnsureProjectParams{
		ID: pgAnchorUUID(t, "project", "pg-engine"), Slug: "pg-engine", Name: "PG Engine",
	}); err != nil {
		t.Fatalf("anchor project: %v", err)
	}
	nar := postgres.NewNarrative(pool)
	if _, err := nar.CreateStory(ctx, "pg-engine", nil, postgres.StoryInfo{
		Slug: "pg-story", Title: "Truyện Postgres", Status: "writing",
	}); err != nil {
		t.Fatalf("create story: %v", err)
	}
	scope, err := nar.Resolve(ctx, "pg-engine", "pg-story", nil, false)
	if err != nil {
		t.Fatalf("resolve scope: %v", err)
	}
	lease, err := postgres.NewStoryLease(ctx, nar, scope, 2*time.Minute)
	if err != nil {
		t.Fatalf("claim lease: %v", err)
	}
	t.Cleanup(func() { _ = lease.Close() })

	backend := postgres.NewBackend(ctx, pool, nar, scope, postgres.WithLeaseFence(lease))
	st, err := storepkg.NewFromBackend(backend)
	if err != nil {
		t.Fatalf("store from backend: %v", err)
	}
	if err := st.Init(); err != nil {
		t.Fatalf("store init: %v", err)
	}
	if err := st.Progress.Init(chapters); err != nil {
		t.Fatalf("progress init: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	outline := make([]domain.OutlineEntry, 0, chapters)
	for i := 1; i <= chapters; i++ {
		outline = append(outline, domain.OutlineEntry{Chapter: i, Title: "Chương", CoreEvent: "việc"})
	}
	if err := st.Outline.SaveOutline(outline); err != nil {
		t.Fatalf("outline: %v", err)
	}
	return st, lease
}

// isolateCwd moves the process into an empty directory for the duration of the
// test. A hosted run that derives a path from Dir() == "" would write here; the
// assertions below require it to stay empty.
func isolateCwd(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	t.Setenv("HOME", dir)
	return dir
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("hosted run wrote %d file(s) into %q: %v", len(entries), dir, names)
	}
}

// TestPostgresHostNoLibrary proves a postgres-backed Host starts with no
// library directory, no engine directory and no lock/log file: the injected
// store is the only fact source.
func TestPostgresHostNoLibrary(t *testing.T) {
	work := isolateCwd(t)
	st, lease := pgEngineStore(t, 2)
	absent := filepath.Join(work, "library", "story")
	cfg := bootstrap.Config{OutputDir: absent}

	h, err := New(cfg, assets.Bundle{},
		WithUnconfiguredReads(),
		WithFileLog("tui.log", false),
		WithStorage(Storage{Store: st, Lease: lease}))
	if err != nil {
		t.Fatalf("New with postgres store: %v", err)
	}
	if h.store != st {
		t.Fatal("Host did not use the injected postgres store")
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hosted Host touched engine directory %q: %v", absent, err)
	}
	if h.fileLogErr != nil {
		t.Fatalf("hosted Host must not write a file log: %v", h.fileLogErr)
	}
	h.Close()
	assertEmptyDir(t, work)
}

// TestScriptedEngineRunPostgres runs the real engine tools (plan/draft/check/
// commit) against the postgres store with a scripted model and an empty working
// directory: the chapter must land in rows, not files.
func TestScriptedEngineRunPostgres(t *testing.T) {
	work := isolateCwd(t)
	st, _ := pgEngineStore(t, 2)
	var runs atomic.Int32
	model := &countFirstCalls{base: scriptedWriterModel(), runs: &runs}
	h, e := onceTestHost(t, st, model)

	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatalf("RunInstructionOnce: %v", err)
	}
	waitHostDone(t, h)
	e.wait()

	progress, err := st.Progress.Load()
	if err != nil {
		t.Fatalf("progress: %v", err)
	}
	if len(progress.CompletedChapters) != 1 || progress.CompletedChapters[0] != 1 {
		t.Fatalf("chapter 1 must be completed in rows: %v", progress.CompletedChapters)
	}
	text, err := st.Drafts.LoadChapterText(1)
	if err != nil {
		t.Fatalf("load committed chapter: %v", err)
	}
	if text == "" {
		t.Fatal("committed chapter text missing from the store")
	}
	reader := NewStoryReaderWithStore(st)
	if reader.EngineDir() != "" {
		t.Fatalf("hosted reader EngineDir = %q, want empty", reader.EngineDir())
	}
	chapters, err := reader.Chapters()
	if err != nil {
		t.Fatalf("reader chapters: %v", err)
	}
	if len(chapters) == 0 || !chapters[0].Completed {
		t.Fatalf("reader must report chapter 1 completed: %+v", chapters)
	}
	if review, err := reader.LatestReview(); err != nil || review != nil {
		t.Fatalf("LatestReview on empty review rows = (%v, %v)", review, err)
	}
	assertEmptyDir(t, work)
}

// TestRevisionCheckPostgres keeps the external-edit check on stored rows: a
// hosted chapter has no chapters/NN.md, so the check must read the stored
// chapter text instead of resolving a path against the process directory.
func TestRevisionCheckPostgres(t *testing.T) {
	work := isolateCwd(t)
	st, _ := pgEngineStore(t, 2)
	var runs atomic.Int32
	model := &countFirstCalls{base: scriptedWriterModel(), runs: &runs}
	h, e := onceTestHost(t, st, model)

	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatalf("RunInstructionOnce: %v", err)
	}
	waitHostDone(t, h)
	e.wait()

	if err := h.requireCleanChapters(); err != nil {
		t.Fatalf("accepted hosted chapter must count as clean: %v", err)
	}
	accepted, err := st.Drafts.LoadChapterText(1)
	if err != nil {
		t.Fatalf("load committed chapter: %v", err)
	}
	edited := accepted + "\n\nMột đoạn người đọc tự thêm."
	if err := st.Drafts.SaveFinalChapter(1, edited); err != nil {
		t.Fatalf("edit stored chapter: %v", err)
	}
	changed, err := h.CheckChapterRevisions()
	if err != nil {
		t.Fatalf("CheckChapterRevisions: %v", err)
	}
	if len(changed) != 1 || changed[0] != 1 {
		t.Fatalf("edited chapter 1 must be reported: %v", changed)
	}
	diff, err := h.RevisionDiffFor(1)
	if err != nil {
		t.Fatalf("RevisionDiffFor: %v", err)
	}
	if strings.Contains(diff.Before, "người đọc tự thêm") || !strings.Contains(diff.After, "người đọc tự thêm") {
		t.Fatalf("diff must expose the stored edit: %+v", diff)
	}
	assertEmptyDir(t, work)
}

// TestScriptedEngineRunFile keeps the file backend as the parity reference for
// the same scripted run: the chapter lands in the engine directory.
func TestScriptedEngineRunFile(t *testing.T) {
	st := onceTestWriterStore(t)
	var runs atomic.Int32
	model := &countFirstCalls{base: scriptedWriterModel(), runs: &runs}
	h, e := onceTestHost(t, st, model)

	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatalf("RunInstructionOnce: %v", err)
	}
	waitHostDone(t, h)
	e.wait()

	progress, err := st.Progress.Load()
	if err != nil {
		t.Fatalf("progress: %v", err)
	}
	if len(progress.CompletedChapters) != 1 || progress.CompletedChapters[0] != 1 {
		t.Fatalf("chapter 1 must be completed: %v", progress.CompletedChapters)
	}
	if _, err := os.Stat(filepath.Join(st.Dir(), "chapters", "01.md")); err != nil {
		t.Fatalf("file backend must still write the chapter document: %v", err)
	}
}
