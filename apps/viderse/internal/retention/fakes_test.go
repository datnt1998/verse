package retention

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// fakeRoots trả một thư mục dự án cố định; alt mô phỏng các root cũ còn
// dữ liệu (multi-root cho heldMedia).
type fakeRoots struct {
	mu   sync.Mutex
	root string
	alt  []string
}

func (f *fakeRoots) Root() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.root
}

func (f *fakeRoots) Roots() (string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.root, append([]string{f.root}, f.alt...)
}

func (f *fakeRoots) set(root string, alt []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.root, f.alt = root, alt
}

// fakeSettings giữ JSON như postgres.SettingsStore.
type fakeSettings struct {
	mu     sync.Mutex
	values map[string]json.RawMessage
}

func (f *fakeSettings) Get(_ context.Context, key string, out any) (bool, error) {
	f.mu.Lock()
	raw, ok := f.values[key]
	f.mu.Unlock()
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, out)
}

func (f *fakeSettings) Set(_ context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.values[key] = raw
	f.mu.Unlock()
	return nil
}

// fakeReaper trả nhãn cố định, tách dry run và thật.
type fakeReaper struct {
	mu     sync.Mutex
	dead   []string
	dried  bool
	reaped int
}

func (r *fakeReaper) ReapOrphaned(_ context.Context, dryRun bool) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if dryRun {
		r.dried = true
		return append([]string(nil), r.dead...), nil
	}
	r.reaped++
	return append([]string(nil), r.dead...), nil
}

// fakeReceipts trả các thư mục scratch cần bảo vệ; err mô phỏng DB lỗi.
type fakeReceipts struct {
	dirs []string
	err  error
}

func (f fakeReceipts) ScratchDirs(context.Context) ([]string, error) {
	return f.dirs, f.err
}

// fakeVideos cấu hình trạng thái video.
type fakeVideos struct {
	finished []SourceVideo
	posted   []PostedVideo
	held     map[string]bool
}

func (f *fakeVideos) Finished(context.Context) ([]SourceVideo, error) { return f.finished, nil }
func (f *fakeVideos) Posted(context.Context) ([]PostedVideo, error)   { return f.posted, nil }
func (f *fakeVideos) HeldMedia(context.Context) (map[string]bool, error) {
	return f.held, nil
}

// fakeTasks ghi lại việc nền của lần dọn.
type fakeTasks struct {
	mu       sync.Mutex
	started  int
	finished int
}

func (t *fakeTasks) Start(_ context.Context, spec domain.TaskSpec) (string, error) {
	t.mu.Lock()
	t.started++
	t.mu.Unlock()
	return "clean-task", nil
}

func (t *fakeTasks) Update(context.Context, string, domain.TaskUpdate) error { return nil }

func (t *fakeTasks) Finish(_ context.Context, id string) error {
	t.mu.Lock()
	t.finished++
	t.mu.Unlock()
	return nil
}

/* ------------------------------- tiện ích ------------------------------- */

type retentionRig struct {
	svc      *Service
	root     string
	config   string
	tmp      string
	now      time.Time
	settings *fakeSettings
	videos   *fakeVideos
	reaper   *fakeReaper
	tasks    *fakeTasks
	roots    *fakeRoots
	receipts *fakeReceipts
}

func newRetentionRig(t *testing.T) *retentionRig {
	t.Helper()
	base := canonicalOr(t, t.TempDir())
	rig := &retentionRig{
		root: filepath.Join(base, "projects"), config: filepath.Join(base, "config"),
		tmp: filepath.Join(base, "tmp"), now: time.Now(),
		settings: &fakeSettings{values: map[string]json.RawMessage{}},
		videos:   &fakeVideos{}, reaper: &fakeReaper{}, tasks: &fakeTasks{},
		roots: &fakeRoots{root: filepath.Join(base, "projects")}, receipts: &fakeReceipts{},
	}
	for _, dir := range []string{rig.root, rig.config, rig.tmp} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	var clock GenClock = clockFunc(func() time.Time { return rig.now })
	rig.svc = New(Deps{
		Roots: rig.roots, ConfigDir: rig.config, TmpDir: rig.tmp,
		Reaper: rig.reaper, Receipts: rig.receipts, Videos: rig.videos,
		Tasks: rig.tasks, Settings: rig.settings, Clock: clock,
	})
	return rig
}

// touchOld đặt mtime của một tệp/thư mục về quá khứ.
func touchOld(t *testing.T, path string, age time.Duration) {
	t.Helper()
	at := time.Now().Add(-age)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func canonicalOr(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks %s: %v", path, err)
	}
	return resolved
}

func writeFileT(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("ghi %s: %v", path, err)
	}
}

func itemByLabel(items []PreviewItem, label string) (PreviewItem, bool) {
	for _, it := range items {
		if it.Label == label {
			return it, true
		}
	}
	return PreviewItem{}, false
}
