package storage

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

// fakeSettings là settings trong bộ nhớ (JSON như postgres.SettingsStore).
type fakeSettings struct {
	mu     sync.Mutex
	values map[string]json.RawMessage
}

func newFakeSettings() *fakeSettings {
	return &fakeSettings{values: map[string]json.RawMessage{}}
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

func (f *fakeSettings) Clear(_ context.Context, key string) error {
	f.mu.Lock()
	delete(f.values, key)
	f.mu.Unlock()
	return nil
}

// fakeVolumes trả ổ theo mount dài nhất khớp tiền tố.
type fakeVolumes struct {
	mu    sync.Mutex
	vols  []Volume
	probe func(path string) Volume
}

func (f *fakeVolumes) List(context.Context) ([]Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Volume(nil), f.vols...), nil
}

func (f *fakeVolumes) Info(_ context.Context, path string) (Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.probe != nil {
		return f.probe(path), nil
	}
	best := Volume{}
	bestLen := -1
	for _, v := range f.vols {
		if onMount(path, v.Mount) && len(v.Mount) > bestLen {
			best, bestLen = v, len(v.Mount)
		}
	}
	if bestLen < 0 {
		return Volume{}, codeError(ErrNotFound, "không tìm thấy ổ cho %s", path)
	}
	return best, nil
}

type fakeQueue struct {
	mu     sync.Mutex
	paused bool
}

func (q *fakeQueue) QueuePaused(context.Context) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paused, nil
}

func (q *fakeQueue) SetQueuePaused(_ context.Context, paused bool) error {
	q.mu.Lock()
	q.paused = paused
	q.mu.Unlock()
	return nil
}

func (q *fakeQueue) Paused() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paused
}

// storagePaused đọc cờ tạm dừng do hold storage bật (storage.queuePaused).
func (f *fakeSettings) storagePaused(t testingT) bool {
	t.Helper()
	var paused bool
	found, _ := f.Get(context.Background(), domain.KeyStoragePaused, &paused)
	return found && paused
}

// userPaused đọc cờ tạm dừng của người dùng (clone.queuePaused).
func (f *fakeSettings) userPaused(t testingT) bool {
	t.Helper()
	var paused bool
	found, _ := f.Get(context.Background(), domain.KeyQueuePaused, &paused)
	return found && paused
}

// fakeTasks ghi lại các lần gọi; block (nếu có) chặn ở Update để test huỷ.
type fakeTasks struct {
	mu       sync.Mutex
	started  []domain.TaskSpec
	updates  []domain.TaskUpdate
	finished []string
	block    chan struct{} // nhận yêu cầu Update đầu tiên
	release  chan struct{} // mở để Update đi tiếp
}

func (t *fakeTasks) Start(_ context.Context, spec domain.TaskSpec) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.started = append(t.started, spec)
	return "task-1", nil
}

func (t *fakeTasks) Update(_ context.Context, id string, u domain.TaskUpdate) error {
	t.mu.Lock()
	t.updates = append(t.updates, u)
	block, release := t.block, t.release
	t.block = nil
	t.mu.Unlock()
	if block != nil {
		select {
		case block <- struct{}{}:
		default:
		}
		<-release
	}
	return nil
}

func (t *fakeTasks) Finish(_ context.Context, id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.finished = append(t.finished, id)
	return nil
}

// blockOnce trả kênh chặn Update đầu tiên (đọc dưới khoá — test khác không
// đọc trực tiếp trường để tránh đua).
func (t *fakeTasks) blockOnce() chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.block
}

func (t *fakeTasks) lastUpdate() (domain.TaskUpdate, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.updates) == 0 {
		return domain.TaskUpdate{}, false
	}
	return t.updates[len(t.updates)-1], true
}

type fakeNotify struct {
	mu    sync.Mutex
	calls []string
}

func (n *fakeNotify) Notify(_ context.Context, kind, text, href, actionLabel string) (string, error) {
	n.mu.Lock()
	n.calls = append(n.calls, kind+"|"+text+"|"+href+"|"+actionLabel)
	n.mu.Unlock()
	return "notif-1", nil
}

func (n *fakeNotify) all() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.calls...)
}

type fakeOpener struct {
	mu   sync.Mutex
	path string
	err  error
}

func (o *fakeOpener) Open(_ context.Context, path string) error {
	o.mu.Lock()
	o.path = path
	o.mu.Unlock()
	return o.err
}

type fakeClock struct {
	mu sync.Mutex
	at time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{at: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.at = c.at.Add(d)
	c.mu.Unlock()
}

// manualTicker để test đẩy từng nhịp watch.
type manualTicker struct{ ch chan time.Time }

func newManualTicker() *manualTicker { return &manualTicker{ch: make(chan time.Time, 1)} }

func (m *manualTicker) C() <-chan time.Time { return m.ch }
func (m *manualTicker) Stop()               {}
func (m *manualTicker) Push()               { m.ch <- time.Now() }

/* ------------------------------- tiện ích ------------------------------- */

// canonicalOr resolve symlink của một đường dẫn đã tồn tại (macOS đổi
// /var → /private/var, nên mount giả phải cùng dạng với path đã canonical).
func canonicalOr(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks %s: %v", path, err)
	}
	return resolved
}

// writeFile ghi tệp và tạo thư mục cha.
func writeFile(t testingT, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("ghi %s: %v", path, err)
	}
}

type testingT interface {
	Helper()
	Fatalf(format string, args ...any)
	Errorf(format string, args ...any)
}

// newTestService dựng Service với các phụ thuộc giả và thư mục gốc trong
// t.TempDir().
type testRig struct {
	svc      *Service
	settings *fakeSettings
	volumes  *fakeVolumes
	tasks    *fakeTasks
	notify   *fakeNotify
	opener   *fakeOpener
	clock    *fakeClock
	home     string
	root     string
}

func newTestRig(t *testing.T, opts ...func(*Deps)) *testRig {
	t.Helper()
	base := canonicalOr(t, t.TempDir())
	home := filepath.Join(base, "home")
	config := filepath.Join(home, ".viderse")
	root := filepath.Join(base, "projects")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	rig := &testRig{
		settings: newFakeSettings(),
		volumes: &fakeVolumes{vols: []Volume{{
			Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
			FreeBytes: 500 << 30, Mount: "/", id: [2]int32{1, 1},
		}}},
		tasks: &fakeTasks{}, notify: &fakeNotify{},
		opener: &fakeOpener{}, clock: newFakeClock(), home: home, root: root,
	}
	deps := Deps{
		Home: home, ConfigDir: config, DefaultDir: root,
		Settings: rig.settings, Volumes: rig.volumes,
		Tasks: rig.tasks, Notify: rig.notify, Opener: rig.opener,
		Clock: rig.clock,
	}
	for _, opt := range opts {
		opt(&deps)
	}
	svc, err := New(context.Background(), deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rig.svc = svc
	return rig
}

// restart dựng lại Service trên cùng settings/thư mục — mô phỏng một lần mở
// app mới để test phục hồi nhật ký.
func (r *testRig) restart(t *testing.T) *Service {
	t.Helper()
	deps := Deps{
		Home: r.home, ConfigDir: filepath.Join(r.home, ".viderse"), DefaultDir: r.root,
		Settings: r.settings, Volumes: r.volumes, Tasks: r.tasks,
		Notify: r.notify, Opener: r.opener, Clock: r.clock,
	}
	svc, err := New(context.Background(), deps)
	if err != nil {
		t.Fatalf("restart New: %v", err)
	}
	return svc
}

// readFile đọc toàn bộ một tệp.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc %s: %v", path, err)
	}
	return string(data)
}

// waitIdle chờ lần chuyển nền kết thúc.
func (r *testRig) waitIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		r.svc.mu.Lock()
		moving := r.svc.moving
		r.svc.mu.Unlock()
		if moving == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("lần chuyển không kết thúc trong 15s")
}

// mkTarget dựng thư mục đích và ổ giả cho nó.
func (r *testRig) mkTarget(t *testing.T, name string, vol Volume) string {
	t.Helper()
	target := filepath.Join(canonicalOr(t, t.TempDir()), name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	target = canonicalOr(t, target)
	vol.Mount = target
	r.volumes.mu.Lock()
	r.volumes.vols = append(r.volumes.vols, vol)
	r.volumes.mu.Unlock()
	return target
}
