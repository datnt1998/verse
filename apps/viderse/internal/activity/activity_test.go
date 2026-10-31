// Hợp đồng việc nền §11 chép từ src/server/db/{tasks,notifications}-store của
// yt-factory: một đối tượng một việc, kết thúc thì xoá, thông báo gộp 10 giây.
package activity

import (
	"context"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type memStore struct {
	mu     sync.Mutex
	tasks  map[string]Task
	notes  []Notification
	errors []ErrorEvent
	nextID int
}

func newMemStore() *memStore { return &memStore{tasks: map[string]Task{}} }

func (m *memStore) FindTaskByObject(_ context.Context, kind domain.TaskKind, object domain.ObjectRef) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tasks {
		if t.Kind == kind && t.Object == object {
			copied := t
			return &copied, nil
		}
	}
	return nil, ErrNotFound
}

func (m *memStore) InsertTask(_ context.Context, spec domain.TaskSpec) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := string(rune('a' + m.nextID))
	t := Task{
		ID: id, Kind: spec.Kind, Agent: spec.Agent, Title: spec.Title,
		Object: spec.Object, Phase: spec.Phase, State: domain.TaskRunning,
		Action: spec.Action, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	m.tasks[id] = t
	copied := t
	return &copied, nil
}

func (m *memStore) ResetTask(_ context.Context, id string, spec domain.TaskSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return ErrNotFound
	}
	t.Title, t.Object, t.Phase, t.State, t.Action = spec.Title, spec.Object, spec.Phase, domain.TaskRunning, spec.Action
	t.Progress = nil
	t.UpdatedAt = time.Now()
	m.tasks[id] = t
	return nil
}

func (m *memStore) AdoptTask(_ context.Context, id string, spec domain.TaskSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return ErrNotFound
	}
	t.Title, t.Object, t.Phase, t.State, t.Action = spec.Title, spec.Object, spec.Phase, domain.TaskRunning, spec.Action
	if spec.Progress != nil {
		t.Progress = spec.Progress
	}
	t.UpdatedAt = time.Now()
	m.tasks[id] = t
	return nil
}

func (m *memStore) UpdateTask(_ context.Context, id string, u domain.TaskUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return ErrNotFound
	}
	if u.Phase != nil {
		t.Phase = *u.Phase
	}
	if u.Progress != nil {
		t.Progress = u.Progress
	}
	if u.State != "" {
		t.State = u.State
	}
	if u.Action != nil {
		t.Action = *u.Action
	}
	t.UpdatedAt = time.Now()
	m.tasks[id] = t
	return nil
}

func (m *memStore) DeleteTask(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, id)
	return nil
}

func (m *memStore) ListTasks(context.Context) ([]Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (m *memStore) CountTasks(_ context.Context, kind domain.TaskKind, state domain.TaskState) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for _, t := range m.tasks {
		if t.Kind == kind && t.State == state {
			n++
		}
	}
	return n, nil
}

func (m *memStore) FailTasksByIDs(_ context.Context, ids []string) ([]Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []Task
	for id, t := range m.tasks {
		if want[id] && (t.State == domain.TaskRunning || t.State == domain.TaskPaused || t.State == domain.TaskWaiting) {
			t.State = domain.TaskFailed
			t.Phase = "Bị gián đoạn khi app khởi động lại"
			t.Action = domain.TaskAction{Label: "Thử lại", Op: domain.TaskOpRetry}
			m.tasks[id] = t
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *memStore) UpsertScopedNotification(_ context.Context, spec NotificationSpec, window time.Duration) (*Notification, error) {
	kind, text, href, actionLabel := spec.Kind, spec.Text, spec.Href, spec.ActionLabel
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.notes) - 1; i >= 0; i-- {
		if m.notes[i].Kind != kind || m.notes[i].Audience != spec.Audience || !sameObject(m.notes[i].Object, spec.Object) {
			continue
		}
		if time.Since(m.notes[i].CreatedAt) <= window {
			m.notes[i].Count++
			m.notes[i].Text, m.notes[i].Href, m.notes[i].ActionLabel = text, href, actionLabel
			copied := m.notes[i]
			return &copied, nil
		}
		break
	}
	m.nextID++
	n := Notification{
		ID: string(rune('A' + m.nextID)), Kind: kind, Count: 1,
		Text: text, Href: href, ActionLabel: actionLabel, CreatedAt: time.Now(),
		Audience: spec.Audience, Object: spec.Object,
	}
	m.notes = append(m.notes, n)
	copied := n
	return &copied, nil
}

func (m *memStore) ListUnreadNotifications(ctx context.Context, limit int) ([]Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Notification, 0, len(m.notes))
	for _, n := range m.notes {
		if n.ReadAt == nil && (IsAdminViewer(ctx) || EffectiveAudience(n.Audience, n.Object) == AudienceAll) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memStore) MarkRead(ctx context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for i := range m.notes {
		if !IsAdminViewer(ctx) && EffectiveAudience(m.notes[i].Audience, m.notes[i].Object) != AudienceAll {
			continue
		}
		if len(ids) == 0 || contains(ids, m.notes[i].ID) {
			if m.notes[i].ReadAt == nil {
				m.notes[i].ReadAt = &now
			}
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func TestStartIsOneTaskPerObjectAndRetryReusesRow(t *testing.T) {
	store := newMemStore()
	var events []string
	svc := New(store, func(ref string) { events = append(events, ref) })
	ctx := context.Background()
	object := domain.ObjectRef{Type: "tool", ID: "ffmpeg", Label: "Xử lý hình và tiếng"}
	spec := domain.TaskSpec{Kind: domain.TaskSetup, Title: "Xử lý hình và tiếng", Object: object, Phase: "Đang cài"}

	id1, err := svc.Start(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := svc.Start(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("Start lặp tạo hai việc: %s / %s", id1, id2)
	}
	failed := domain.TaskFailed
	if err := svc.Update(ctx, id1, domain.TaskUpdate{State: failed}); err != nil {
		t.Fatal(err)
	}
	// Retry sau failed: cùng dòng, quay về running và sạch tiến độ cũ.
	spec.Phase = "Đang thử lại"
	if id3, err := svc.Start(ctx, spec); err != nil || id3 != id1 {
		t.Fatalf("retry id = %q err = %v", id3, err)
	}
	snap, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tasks) != 1 || snap.Tasks[0].State != domain.TaskRunning || snap.Tasks[0].Phase != "Đang thử lại" {
		t.Fatalf("tasks = %+v", snap.Tasks)
	}
	if len(events) == 0 {
		t.Fatal("mọi thay đổi phải phát sự kiện")
	}
}

func TestStartAdoptionKeepsProgressUntilFailed(t *testing.T) {
	store := newMemStore()
	svc := New(store, nil)
	ctx := context.Background()
	spec := domain.TaskSpec{
		Kind: domain.TaskSetup, Title: "Xử lý hình và tiếng",
		Object: domain.ObjectRef{Type: "tool", ID: "ffmpeg"}, Phase: "Đang cài",
	}
	id, err := svc.Start(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	paused := domain.TaskPaused
	if err := svc.Update(ctx, id, domain.TaskUpdate{
		State: paused, Progress: &domain.TaskProgress{Done: 3, Total: 150},
	}); err != nil {
		t.Fatal(err)
	}

	// Nhận lại một việc đang sống: nội dung đổi, tiến độ còn nguyên.
	spec.Phase = "Đang cài lại"
	if id2, err := svc.Start(ctx, spec); err != nil || id2 != id {
		t.Fatalf("adopt id = %q err = %v", id2, err)
	}
	task, err := svc.FindTaskByObject(ctx, spec.Kind, spec.Object)
	if err != nil {
		t.Fatal(err)
	}
	if task.Progress == nil || task.Progress.Done != 3 || task.Progress.Total != 150 {
		t.Fatalf("nhận lại xoá mất tiến độ: %+v", task.Progress)
	}
	if task.State != domain.TaskRunning || task.Phase != "Đang cài lại" {
		t.Fatalf("nhận lại phải viết lại nội dung: %+v", task)
	}

	// Retry sau failed vẫn là đường xoá sạch tiến độ.
	failed := domain.TaskFailed
	if err := svc.Update(ctx, id, domain.TaskUpdate{State: failed}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(ctx, spec); err != nil {
		t.Fatal(err)
	}
	task, err = svc.FindTaskByObject(ctx, spec.Kind, spec.Object)
	if err != nil {
		t.Fatal(err)
	}
	if task.Progress != nil {
		t.Fatalf("retry sau failed phải xoá tiến độ cũ: %+v", task.Progress)
	}
}

func TestFinishRemovesTaskAndCountsShape(t *testing.T) {
	store := newMemStore()
	svc := New(store, nil)
	ctx := context.Background()
	video := domain.TaskSpec{
		Kind: domain.TaskVideo, Title: "Tải video", Phase: "Đang tải",
		Object: domain.ObjectRef{Type: "video", ID: "v1"},
	}
	id, err := svc.Start(ctx, video)
	if err != nil {
		t.Fatal(err)
	}
	waiting := video
	waiting.Object = domain.ObjectRef{Type: "video", ID: "v2"}
	wid, err := svc.Start(ctx, waiting)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(ctx, wid, domain.TaskUpdate{State: domain.TaskWaiting}); err != nil {
		t.Fatal(err)
	}
	snap, _ := svc.Snapshot(ctx)
	if snap.VideoJobs != 1 || snap.Queued != 1 {
		t.Fatalf("video=%d queued=%d", snap.VideoJobs, snap.Queued)
	}
	if err := svc.Finish(ctx, id); err != nil {
		t.Fatal(err)
	}
	snap, _ = svc.Snapshot(ctx)
	if snap.VideoJobs != 0 || len(snap.Tasks) != 1 {
		t.Fatalf("sau Finish: video=%d tasks=%d", snap.VideoJobs, len(snap.Tasks))
	}
}

func TestReconcileFailsOrphansWithRetryAction(t *testing.T) {
	store := newMemStore()
	svc := New(store, nil)
	ctx := context.Background()
	id, err := svc.Start(ctx, domain.TaskSpec{
		Kind: domain.TaskSetup, Title: "Dựng và kiểm tra video",
		Object: domain.ObjectRef{Type: "tool", ID: "chrome"},
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := svc.Reconcile(ctx)
	if err != nil || n != 1 {
		t.Fatalf("reconcile = %d, %v", n, err)
	}
	snap, _ := svc.Snapshot(ctx)
	task := snap.Tasks[0]
	if task.ID != id || task.State != domain.TaskFailed {
		t.Fatalf("task = %+v", task)
	}
	if task.Action.Op != domain.TaskOpRetry || task.Action.Label != "Thử lại" {
		t.Fatalf("action = %+v", task.Action)
	}
	// Chạy lại Reconcile không đụng việc đã failed.
	if n, _ := svc.Reconcile(ctx); n != 0 {
		t.Fatalf("reconcile lần hai = %d", n)
	}
}

func TestNotifyMergesWithinWindow(t *testing.T) {
	store := newMemStore()
	var refs []string
	svc := New(store, func(ref string) { refs = append(refs, ref) })
	ctx := context.Background()
	first, err := svc.Notify(ctx, "action_result", "Đã xuất video", "#/videos/1", "Mở")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Notify(ctx, "action_result", "Đã xuất video xong", "#/videos/1", "Mở")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("không gộp: %s / %s", first, second)
	}
	other, err := svc.Notify(ctx, "needs_attention", "Không thấy ổ T7", "#/settings?section=storage", "")
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatal("kind khác phải là thông báo riêng")
	}
	snap, _ := svc.Snapshot(ctx)
	if len(snap.Unread) != 2 {
		t.Fatalf("unread = %+v", snap.Unread)
	}
	if err := svc.MarkRead(ctx, nil); err != nil {
		t.Fatal(err)
	}
	snap, _ = svc.Snapshot(ctx)
	if len(snap.Unread) != 0 {
		t.Fatalf("MarkRead(nil) không xoá hết: %+v", snap.Unread)
	}
}

func TestStartRequiresTitleAndKind(t *testing.T) {
	svc := New(newMemStore(), nil)
	if _, err := svc.Start(context.Background(), domain.TaskSpec{Kind: domain.TaskSetup}); err == nil {
		t.Fatal("thiếu title phải lỗi")
	}
	if _, err := svc.Start(context.Background(), domain.TaskSpec{Title: "x"}); err == nil {
		t.Fatal("thiếu kind phải lỗi")
	}
}

func TestErrorsListsFailedTasksAndAttention(t *testing.T) {
	store := newMemStore()
	svc := New(store, nil)
	ctx := context.Background()
	id, _ := svc.Start(ctx, domain.TaskSpec{
		Kind: domain.TaskSetup, Title: "Xử lý hình và tiếng",
		Object: domain.ObjectRef{Type: "tool", ID: "ffmpeg", Label: "Xử lý hình và tiếng"},
	})
	phase := "Mạng bị ngắt"
	action := domain.TaskAction{Label: "Thử lại", Op: domain.TaskOpRetry}
	_ = svc.Update(ctx, id, domain.TaskUpdate{State: domain.TaskFailed, Phase: &phase, Action: &action})
	if _, err := svc.Notify(ctx, "needs_attention", "Không thấy ổ T7", "#/settings?section=storage", ""); err != nil {
		t.Fatal(err)
	}
	items, err := svc.Errors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	var sawTask, sawNote bool
	for _, it := range items {
		switch it.Kind {
		case "task_failed":
			sawTask = it.What == "Xử lý hình và tiếng" && it.Why == phase && it.Action.Op == domain.TaskOpRetry
		case "needs_attention":
			sawNote = it.Status == "Cần xử lý"
		}
	}
	if !sawTask || !sawNote {
		t.Fatalf("task=%v note=%v items=%+v", sawTask, sawNote, items)
	}
	if _, err := svc.ErrorsCount(ctx); err != nil {
		t.Fatal(err)
	}
}

func (m *memStore) InsertErrorEvent(_ context.Context, e ErrorEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	e.ID = "err-" + string(rune('0'+m.nextID%10))
	if e.At.IsZero() {
		e.At = time.Now()
	}
	m.errors = append(m.errors, e)
	return nil
}

func (m *memStore) ListErrorEvents(ctx context.Context, since time.Time, limit int) ([]ErrorEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ErrorEvent
	for i := len(m.errors) - 1; i >= 0; i-- {
		if !m.errors[i].At.Before(since) && (IsAdminViewer(ctx) || EffectiveAudience(m.errors[i].Audience, m.errors[i].Object) == AudienceAll) {
			out = append(out, m.errors[i])
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memStore) CountErrorEvents(ctx context.Context, since time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, e := range m.errors {
		if !e.At.Before(since) && (IsAdminViewer(ctx) || EffectiveAudience(e.Audience, e.Object) == AudienceAll) {
			n++
		}
	}
	return n, nil
}

// Chỉ việc của process đã chết mới bị đánh dấu failed; việc của process
// đang sống (và dòng cũ chưa có owner) không bị đụng.
func TestReconcileOnlyDeadOwners(t *testing.T) {
	store := newMemStore()
	store.tasks["live"] = Task{ID: "live", State: domain.TaskRunning, OwnerPID: os.Getpid()}
	store.tasks["dead"] = Task{ID: "dead", State: domain.TaskRunning, OwnerPID: 999999}
	store.tasks["legacy"] = Task{ID: "legacy", State: domain.TaskWaiting, OwnerPID: 0}
	svc := New(store, nil)
	n, err := svc.Reconcile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("reconciled = %d, muốn 2", n)
	}
	if got := store.tasks["live"].State; got != domain.TaskRunning {
		t.Fatalf("việc của process sống bị đổi: %q", got)
	}
	for _, id := range []string{"dead", "legacy"} {
		if got := store.tasks[id].State; got != domain.TaskFailed {
			t.Fatalf("%s = %q, muốn failed", id, got)
		}
	}
}

// Lịch sử lỗi 7 ngày bền: đọc/đánh dấu đã đọc hay thử lại không làm mất mục,
// và số đếm không bị cắt theo trang hiển thị.
func TestErrorHistorySurvivesReadAndRetry(t *testing.T) {
	store := newMemStore()
	for i := 0; i < 25; i++ {
		if err := store.InsertErrorEvent(context.Background(), ErrorEvent{Kind: "needs_attention", What: "lỗi"}); err != nil {
			t.Fatal(err)
		}
	}
	svc := New(store, nil)
	items, err := svc.Errors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 20 {
		t.Fatalf("trang = %d, muốn 20", len(items))
	}
	count, err := svc.ErrorsCount(context.Background())
	if err != nil || count != 25 {
		t.Fatalf("count = %d, %v; muốn 25", count, err)
	}
}

// TestChangedPublishesWithoutMutating: Changed chỉ phát ref cho SSE, không tạo
// hay sửa dòng việc/thông báo nào.
func TestChangedPublishesWithoutMutating(t *testing.T) {
	store := newMemStore()
	var refs []string
	svc := New(store, func(ref string) { refs = append(refs, ref) })
	svc.Changed("task-1")
	if len(refs) != 1 || refs[0] != "task-1" {
		t.Fatalf("refs = %v, muốn [task-1]", refs)
	}
	tasks, err := store.ListTasks(context.Background())
	if err != nil || len(tasks) != 0 {
		t.Fatalf("Changed đã sửa dữ liệu: tasks=%v err=%v", tasks, err)
	}
}

func sameObject(a, b *domain.ObjectRef) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestStorageActivityIsAdminOnly(t *testing.T) {
	store := newMemStore()
	store.tasks["storage"] = Task{Kind: "storage_migrate", Object: domain.ObjectRef{Type: "storage"}}
	store.tasks["other"] = Task{Kind: domain.TaskSetup, Object: domain.ObjectRef{Type: "tool"}}
	svc := New(store, nil)
	ctx := context.Background()
	adminCtx := WithViewer(ctx, true)
	privateID, err := svc.NotifyScoped(ctx, NotificationSpec{
		Kind: "needs_attention", Text: "private", Object: &domain.ObjectRef{Type: "storage", ID: "active"},
	})
	if err != nil {
		t.Fatal(err)
	}
	publicID, err := svc.Notify(ctx, "needs_attention", "public", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if privateID == publicID {
		t.Fatal("different objects/audiences merged")
	}
	member, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(member.Tasks) != 1 || len(member.Unread) != 1 || member.Unread[0].ID != publicID {
		t.Fatalf("member visibility: %+v", member)
	}
	if err := svc.MarkRead(ctx, nil); err != nil {
		t.Fatal(err)
	}
	admin, err := svc.Snapshot(adminCtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(admin.Tasks) != 2 || len(admin.Unread) != 1 || admin.Unread[0].ID != privateID {
		t.Fatalf("admin visibility or member consumed private notification: %+v", admin)
	}
	items, err := svc.Errors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].What != "public" {
		t.Fatalf("private errors leaked: %+v", items)
	}
}
