package scan

// Test đơn vị của mặt service + vài ca worker cần dựng cảnh cụ thể: rào giờ chờ
// nền tảng là not-before, lỗi đọc rào chặn Add, Add ghi 429 bền, adoption giữ
// tiến độ, và mất liên kết việc nền không để dòng mồ côi.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

// gateStub là PlatformGate điều khiển được.
type gateStub struct {
	mu      sync.Mutex
	err     error
	until   *time.Time
	blocked []time.Duration
}

func (g *gateStub) Until(context.Context, domain.PlatformID) (*time.Time, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.until, g.err
}

func (g *gateStub) RecordBlocked(_ context.Context, id domain.PlatformID, retryAfter time.Duration) error {
	if id != domain.PlatformYouTube {
		panic("chặn nhầm nền tảng: " + string(id))
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.blocked = append(g.blocked, retryAfter)
	return nil
}

// countProvider đếm lời gọi mạng và trả lỗi cố định.
type countProvider struct {
	Provider
	mu    sync.Mutex
	calls int
	err   error
}

func (p *countProvider) FirstChannelPage(context.Context, string) (*youtube.ChannelVideoPage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return nil, p.err
}

func (p *countProvider) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// stubStore chỉ cài những gì test chạm tới; phần còn lại panic nếu dùng.
type stubStore struct {
	Store
	channel    domain.RivalChannel
	state      State
	syncErr    error
	syncResult TaskSync
	syncCalls  int
	syncSpec   domain.TaskSpec
	syncs      []domain.TaskSpec
}

func (s *stubStore) Channel(context.Context, string) (domain.RivalChannel, error) {
	return s.channel, nil
}
func (s *stubStore) EnsureState(context.Context, string) (State, error) { return s.state, nil }
func (s *stubStore) SyncScanTask(_ context.Context, _, _ string, spec domain.TaskSpec) (TaskSync, error) {
	s.syncCalls++
	s.syncSpec = spec
	s.syncs = append(s.syncs, spec)
	if s.syncErr != nil {
		return TaskSync{}, s.syncErr
	}
	res := s.syncResult
	if res.TaskID == "" {
		res.TaskID = "task-1"
	}
	return res, nil
}
func (s *stubStore) ClearTaskLink(context.Context, string) error { return nil }

// tasksStub ghi lại mọi lần Finish/Changed; Update/Start/Notify cố tình không
// còn trong interface — giữ bản ghi để test chứng minh không lần ghi rời nào.
type tasksStub struct {
	mu       sync.Mutex
	starts   []domain.TaskSpec
	updates  []domain.TaskUpdate
	finished []string
	changed  []string
	nextID   string
}

func (t *tasksStub) Start(_ context.Context, spec domain.TaskSpec) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.starts = append(t.starts, spec)
	if t.nextID == "" {
		return "task-1", nil
	}
	return t.nextID, nil
}
func (t *tasksStub) Update(_ context.Context, _ string, u domain.TaskUpdate) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.updates = append(t.updates, u)
	return nil
}
func (t *tasksStub) Finish(_ context.Context, id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.finished = append(t.finished, id)
	return nil
}
func (t *tasksStub) Notify(context.Context, string, string, string, string) (string, error) {
	panic("scan không còn gọi Notify")
}
func (t *tasksStub) Changed(ref string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.changed = append(t.changed, ref)
}

func TestAddPauseReadFailureStopsRequest(t *testing.T) {
	gate := &gateStub{err: errors.New("pause store hỏng")}
	provider := &countProvider{err: &youtube.FetchError{Kind: youtube.KindRateLimited}}
	svc := New(Deps{Store: &stubStore{}, YouTube: provider, Platform: gate})
	if _, err := svc.Add(context.Background(), "@SomeHandle"); err == nil {
		t.Fatal("Add phải trả lỗi khi không đọc được rào nền tảng")
	}
	if got := provider.count(); got != 0 {
		t.Fatalf("Add đã gửi %d request dù rào nền tảng không đọc được", got)
	}
}

func TestAddRateLimitIsPersisted(t *testing.T) {
	gate := &gateStub{}
	delay := time.Minute
	provider := &countProvider{err: &youtube.FetchError{Kind: youtube.KindRateLimited, RetryDelay: &delay}}
	svc := New(Deps{Store: &stubStore{}, YouTube: provider, Platform: gate})
	_, err := svc.Add(context.Background(), "@SomeHandle")
	if CodeOf(err) != CodeRateLimited {
		t.Fatalf("Add lỗi = %v, muốn rate_limited", err)
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if len(gate.blocked) != 1 || gate.blocked[0] != time.Minute {
		t.Fatalf("RecordBlocked = %v, muốn một lần đúng 1 phút Retry-After", gate.blocked)
	}
}

func TestAddBlockedIsPersisted(t *testing.T) {
	gate := &gateStub{}
	provider := &countProvider{err: &youtube.FetchError{Kind: youtube.KindBlocked, Message: "bot"}}
	svc := New(Deps{Store: &stubStore{}, YouTube: provider, Platform: gate})
	if _, err := svc.Add(context.Background(), "@SomeHandle"); CodeOf(err) != CodeBlocked {
		t.Fatalf("Add lỗi = %v, muốn blocked", err)
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if len(gate.blocked) != 1 {
		t.Fatalf("RecordBlocked = %v, muốn một lần", gate.blocked)
	}
}

// TestWorkerIdleHonoursPlatformPause: đang chờ nền tảng thì việc actionable
// không được kéo worker ra khỏi giấc ngủ trước hạn.
func TestWorkerIdleHonoursPlatformPause(t *testing.T) {
	store := &fakeStore{}
	store.setWakeup(Wakeup{Actionable: true})
	until := time.Now().Add(120 * time.Millisecond)
	w := NewWorker(WorkerDeps{Store: store, YouTube: fakeProvider{}, Platform: &gateStub{until: &until}})
	start := time.Now()
	if err := w.idle(context.Background(), 1); err != nil {
		t.Fatalf("idle = %v", err)
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("idle trở về sau %v khi nền tảng còn chờ tới %v", elapsed, until)
	}
}

// TestSyncTaskUsesAtomicSyncForAdoption: tạo/nhận dòng + nối việc đi qua đúng
// một lời gọi kho nguyên tử; dòng mới nhận tiến độ khởi tạo một lần, còn dòng
// đã có giữ nguyên con số đang hiện (spec.Progress nil). Không còn lần ghi rời
// nào mang state/phase/action.
func TestSyncTaskUsesAtomicSyncForAdoption(t *testing.T) {
	store := &stubStore{syncResult: TaskSync{TaskID: "task-1"}, state: State{ChannelID: "c", HistoryCap: 150, IndexedVideos: 3}}
	tasks := &tasksStub{}
	svc := New(Deps{Store: store, YouTube: &countProvider{}, Tasks: tasks})

	// Dòng mới (chưa có task): tiến độ khởi tạo đi kèm spec.
	svc.syncTask(context.Background(), &Job{ID: "job", ChannelID: "c", Kind: domain.ScanHistory, State: domain.ScanRunning}, taskSync{})
	if store.syncCalls != 1 {
		t.Fatalf("SyncScanTask = %d lần, muốn 1", store.syncCalls)
	}
	if p := store.syncSpec.Progress; p == nil || p.Done != 3 || p.Total != 150 {
		t.Fatalf("progress khởi tạo trong spec = %+v, muốn {3,150}", p)
	}
	if store.syncSpec.Kind != domain.TaskChannelScan {
		t.Fatalf("kind = %q", store.syncSpec.Kind)
	}

	// Dòng đã có (Job.TaskID != ""): giữ tiến độ đang hiện (nil).
	store.syncCalls, store.syncSpec = 0, domain.TaskSpec{}
	svc.syncTask(context.Background(), &Job{ID: "job", ChannelID: "c", TaskID: "task-1", Kind: domain.ScanHistory, State: domain.ScanRunning}, taskSync{})
	if store.syncCalls != 1 || store.syncSpec.Progress != nil {
		t.Fatalf("adoption dòng cũ = %d lần, progress %+v; muốn 1 lần, nil", store.syncCalls, store.syncSpec.Progress)
	}

	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if len(tasks.starts) != 0 || len(tasks.updates) != 0 {
		t.Fatalf("còn lần ghi việc nền rời (start=%d update=%d), muốn 0", len(tasks.starts), len(tasks.updates))
	}
}

// TestSyncTaskWritesOnlyThroughStore: mọi đường ghi (kể cả tiến độ của một
// việc đang chạy) đều đi qua SyncScanTask; service không ghi state/phase/
// action lần thứ hai bằng ảnh chụp cũ.
func TestSyncTaskWritesOnlyThroughStore(t *testing.T) {
	store := &stubStore{state: State{ChannelID: "c", HistoryCap: 150, IndexedVideos: 9}}
	tasks := &tasksStub{}
	svc := New(Deps{Store: store, YouTube: &countProvider{}, Tasks: tasks})
	svc.syncTask(context.Background(), &Job{ID: "job", ChannelID: "c", TaskID: "task-1", Kind: domain.ScanHistory, State: domain.ScanRunning}, taskSync{progress: true})
	if store.syncCalls != 1 {
		t.Fatalf("SyncScanTask = %d lần, muốn 1", store.syncCalls)
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if len(tasks.updates) != 0 {
		t.Fatalf("còn Update rời mang ảnh chụp cũ: %+v", tasks.updates)
	}
}

// TestAdoptionHonoursProgressBudget: nhiều lần đồng bộ cùng một mốc thời gian
// chỉ được gửi tiến độ một lần qua SyncScanTask (kho giữ nguyên khi nil).
func TestAdoptionHonoursProgressBudget(t *testing.T) {
	store := &stubStore{syncResult: TaskSync{TaskID: "task-1"}, state: State{ChannelID: "c", HistoryCap: 150, IndexedVideos: 7}}
	tasks := &tasksStub{}
	now := time.Now()
	svc := New(Deps{Store: store, YouTube: &countProvider{}, Tasks: tasks, Now: func() time.Time { return now }})
	job := &Job{ID: "job", ChannelID: "c", TaskID: "task-1", Kind: domain.ScanHistory, State: domain.ScanRunning}
	for range 10 {
		svc.syncTask(context.Background(), job, taskSync{progress: true})
	}
	writes := 0
	for _, spec := range store.syncs {
		if spec.Progress != nil {
			writes++
		}
	}
	if writes > 1 {
		t.Fatalf("10 lần đồng bộ tại một mốc ghi tiến độ %d lần, muốn ≤ 1", writes)
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if len(tasks.updates) != 0 {
		t.Fatalf("còn lần ghi tiến độ rời: %+v", tasks.updates)
	}
}

// TestProgressBudgetAcrossPageSyncs: các lần đồng bộ trang (kể cả trang dư đã
// lưu) dùng chung một ngân sách ≤4/s/kênh; qua 250ms mới có lượt ghi kế tiếp.
func TestProgressBudgetAcrossPageSyncs(t *testing.T) {
	store := &stubStore{state: State{ChannelID: "c", HistoryCap: 150, IndexedVideos: 5}}
	tasks := &tasksStub{}
	now := time.Now()
	svc := New(Deps{Store: store, YouTube: &countProvider{}, Tasks: tasks, Now: func() time.Time { return now }})
	job := &Job{ID: "job", ChannelID: "c", TaskID: "task-1", Kind: domain.ScanHistory, State: domain.ScanRunning}

	countWrites := func() int {
		n := 0
		for _, spec := range store.syncs {
			if spec.Progress != nil {
				n++
			}
		}
		return n
	}
	for range 3 {
		svc.syncTask(context.Background(), job, taskSync{progress: true})
	}
	if got := countWrites(); got != 1 {
		t.Fatalf("3 lần đồng bộ cùng mốc ghi tiến độ %d lần, muốn 1", got)
	}
	now = now.Add(ProgressInterval)
	svc.syncTask(context.Background(), job, taskSync{progress: true})
	if got := countWrites(); got != 2 {
		t.Fatalf("sau 250ms phải được ghi thêm một lần, tổng %d", got)
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if len(tasks.updates) != 0 {
		t.Fatalf("còn Update rời mang tiến độ: %+v", tasks.updates)
	}
}

// TestSyncTaskStoreErrorLeavesNoOrphan: việc bị gỡ giữa chừng thì giao dịch kho
// tự cuốn dòng vừa tạo; service không Update/Finish dòng không tồn tại.
func TestSyncTaskStoreErrorLeavesNoOrphan(t *testing.T) {
	store := &stubStore{syncErr: &StoreError{Code: StoreJobStopped}}
	tasks := &tasksStub{}
	svc := New(Deps{Store: store, YouTube: &countProvider{}, Tasks: tasks})
	job := &Job{ID: "job", ChannelID: "c", Kind: domain.ScanDiscovery, State: domain.ScanRunning}
	svc.syncTask(context.Background(), job, taskSync{})

	if store.syncCalls != 1 {
		t.Fatalf("SyncScanTask = %d lần, muốn 1", store.syncCalls)
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if len(tasks.updates) != 0 || len(tasks.finished) != 0 || len(tasks.starts) != 0 || len(tasks.changed) != 0 {
		t.Fatalf("sau lỗi kho vẫn ghi/phát dòng: updates=%d finished=%d starts=%d changed=%d",
			len(tasks.updates), len(tasks.finished), len(tasks.starts), len(tasks.changed))
	}
}

// TestTaskPresentationKeepsAcceptedActionMapping: state/phase/action là hợp
// đồng UI đã chốt; TaskPresentation là nguồn duy nhất cho cả service và kho.
func TestTaskPresentationKeepsAcceptedActionMapping(t *testing.T) {
	paused := Job{ID: "j", ChannelID: "c", Kind: domain.ScanHistory, State: domain.ScanPaused}
	view, live := TaskPresentation(paused)
	if !live || view.State != domain.TaskPaused || view.Phase != "Đã tạm dừng" ||
		view.Action.Op != domain.TaskOpResume || view.Action.Label != "Tiếp tục" {
		t.Fatalf("paused = %+v live=%v", view, live)
	}
	failed := paused
	failed.State = domain.ScanError
	failed.LastError = "YouTube chặn"
	view, live = TaskPresentation(failed)
	if !live || view.State != domain.TaskFailed || view.Phase != "YouTube chặn" ||
		view.Action.Op != domain.TaskOpRetry || view.Action.Label != "Thử lại" {
		t.Fatalf("error = %+v live=%v", view, live)
	}
	empty := failed
	empty.LastError = "  "
	if view, _ := TaskPresentation(empty); view.Phase != "Lỗi khi quét" {
		t.Fatalf("error rỗng = %+v", view)
	}
	waiting := paused
	waiting.State = domain.ScanBackoff
	view, live = TaskPresentation(waiting)
	if !live || view.State != domain.TaskWaiting || view.Phase != "Đang chờ thử lại" ||
		view.Action.Op != domain.TaskOpCancel {
		t.Fatalf("backoff = %+v live=%v", view, live)
	}
	for _, state := range []domain.ScanJobState{domain.ScanComplete, domain.ScanCancelled, domain.ScanCapped} {
		if _, live := TaskPresentation(Job{State: state}); live {
			t.Fatalf("việc đã dừng %q vẫn tạo dòng việc", state)
		}
	}
}

// TestTaskPresentationDiscoveryAndHistory: discovery có nút Mở trỏ đúng kênh;
// lịch sử đang chạy có nút Huỷ và pha đọc tab Videos.
func TestTaskPresentationDiscoveryAndHistory(t *testing.T) {
	discovery := Job{ID: "j", ChannelID: "UC123", Kind: domain.ScanDiscovery, State: domain.ScanRunning}
	view, live := TaskPresentation(discovery)
	if !live || view.State != domain.TaskRunning || view.Phase != "Đang kiểm tra video mới" {
		t.Fatalf("discovery = %+v live=%v", view, live)
	}
	if view.Action.Label != "Mở" || view.Action.Href != "#/explore?rival=UC123" || view.Action.Op != "" {
		t.Fatalf("discovery action = %+v", view.Action)
	}
	history := Job{ID: "j", ChannelID: "UC123", Kind: domain.ScanHistory, State: domain.ScanRunning}
	view, live = TaskPresentation(history)
	if !live || view.State != domain.TaskRunning || view.Phase != "Đang đọc tab Videos" ||
		view.Action.Label != "Huỷ" || view.Action.Op != domain.TaskOpCancel {
		t.Fatalf("history = %+v live=%v", view, live)
	}
	// Discovery đang chờ thử lại: giữ waiting nhưng nút là Mở, không mượn op
	// huỷ của lịch sử (endpoint huỷ chỉ tác động lịch sử).
	discovery.State = domain.ScanBackoff
	view, live = TaskPresentation(discovery)
	if !live || view.State != domain.TaskWaiting || view.Phase != "Đang chờ thử lại" {
		t.Fatalf("discovery backoff = %+v live=%v", view, live)
	}
	if view.Action.Label != "Mở" || view.Action.Href != "#/explore?rival=UC123" || view.Action.Op != "" {
		t.Fatalf("discovery backoff action = %+v, muốn Mở không op", view.Action)
	}
	history.State = domain.ScanBackoff
	if view, _ := TaskPresentation(history); view.Action.Op != domain.TaskOpCancel {
		t.Fatalf("history backoff phải còn Huỷ: %+v", view.Action)
	}
}

// TestFailedSyncPublishesTaskAndNotification: đường failed chỉ ghi qua
// SyncScanTask; service phát đúng hai sự kiện khi kho báo lần ghi này đã đưa
// việc sang failed (dòng việc + thông báo), không còn lần Notify/Update rời.
func TestFailedSyncPublishesTaskAndNotification(t *testing.T) {
	store := &stubStore{syncResult: TaskSync{TaskID: "task-1", NotificationID: "notice-1"}}
	tasks := &tasksStub{}
	svc := New(Deps{Store: store, YouTube: &countProvider{}, Tasks: tasks})
	svc.syncTask(context.Background(), &Job{ID: "job", ChannelID: "c", Kind: domain.ScanHistory, State: domain.ScanError, LastError: "YouTube chặn"}, taskSync{})
	if store.syncCalls != 1 {
		t.Fatalf("SyncScanTask = %d lần, muốn 1", store.syncCalls)
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if len(tasks.updates) != 0 {
		t.Fatalf("còn Update rời cho failed: %+v", tasks.updates)
	}
	if len(tasks.changed) != 2 || tasks.changed[0] != "task-1" || tasks.changed[1] != "notice-1" {
		t.Fatalf("changed = %v, muốn [task-1 notice-1]", tasks.changed)
	}
}

// TestStaleErrorSnapshotPublishesNoNotification: ảnh chụp lỗi cũ không tự sinh
// thông báo; chỉ id thông báo do kho trả về (nếu có chuyển failed thật) mới
// được phát. Việc đã được tiếp tục/đổi trạng thái thì lần sync thành công trả
// NotificationID rỗng.
func TestStaleErrorSnapshotPublishesNoNotification(t *testing.T) {
	store := &stubStore{syncResult: TaskSync{TaskID: "task-1"}}
	tasks := &tasksStub{}
	svc := New(Deps{Store: store, YouTube: &countProvider{}, Tasks: tasks})
	svc.syncTask(context.Background(), &Job{ID: "job", ChannelID: "c", TaskID: "task-1", Kind: domain.ScanHistory, State: domain.ScanError, LastError: "cũ"}, taskSync{})
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if len(tasks.changed) != 1 || tasks.changed[0] != "task-1" {
		t.Fatalf("changed = %v, muốn [task-1] (không thông báo từ ảnh chụp cũ)", tasks.changed)
	}
}

// TestFailureNoticeText: thông báo lỗi thuần giữ đúng câu chữ/nhãn của hợp
// đồng "Cần xử lý" trước đây.
func TestFailureNoticeText(t *testing.T) {
	notice := FailureNotice("Quét lịch sử", "UC1", TaskView{Phase: "YouTube chặn"})
	want := Notice{Kind: "needs_attention", Text: "Quét lịch sử · UC1: YouTube chặn", Href: "#/explore", ActionLabel: ""}
	if notice != want {
		t.Fatalf("FailureNotice = %+v, muốn %+v", notice, want)
	}
}

// TestTerminalJobFinishesTaskWithoutSync: việc đã dừng hẳn chỉ đóng dòng cũ,
// không ghi/tạo dòng mới.
func TestTerminalJobFinishesTaskWithoutSync(t *testing.T) {
	store := &stubStore{}
	tasks := &tasksStub{}
	svc := New(Deps{Store: store, YouTube: &countProvider{}, Tasks: tasks})
	svc.syncTask(context.Background(), &Job{ID: "job", ChannelID: "c", TaskID: "task-1", Kind: domain.ScanHistory, State: domain.ScanCapped}, taskSync{})
	if store.syncCalls != 0 {
		t.Fatalf("SyncScanTask = %d lần cho việc đã dừng, muốn 0", store.syncCalls)
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if len(tasks.finished) != 1 || tasks.finished[0] != "task-1" || len(tasks.updates) != 0 {
		t.Fatalf("finish=%v updates=%v", tasks.finished, tasks.updates)
	}
}
