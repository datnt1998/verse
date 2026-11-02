package scan

// Test đơn vị của luồng quét: phân loại lỗi, câu chẩn đoán an toàn, backoff,
// ánh xạ việc nền theo trạng thái, và nhịp thức của worker khi rảnh (không
// poll DB trong lúc ngủ).

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

func TestClassifyFailureMapsProviderErrors(t *testing.T) {
	delay := 2 * time.Minute
	cases := []struct {
		name string
		err  error
		kind FailureKind
	}{
		{"hết giờ", context.DeadlineExceeded, FailureAbort},
		{"fence mất", &StoreError{Code: StoreLeaseLost}, FailureStopped},
		{"ô lịch chung", &youtube.DeferredError{RetryAt: time.Now().Add(time.Minute)}, FailureDeferred},
		{"rate limit", &youtube.FetchError{Kind: youtube.KindRateLimited, RetryDelay: &delay}, FailureRateLimited},
		{"chập chờn", &youtube.FetchError{Kind: youtube.KindTransient}, FailureTransient},
		{"bị chặn", &youtube.FetchError{Kind: youtube.KindBlocked}, FailureBlocked},
		{"cursor hỏng", &youtube.FetchError{Kind: youtube.KindCursorInvalid}, FailureCursor},
		{"cursor kho", &StoreError{Code: StoreCursorInvalid}, FailureCursor},
		{"trang lạ", &youtube.FetchError{Kind: youtube.KindUnexpectedPage}, FailureFatal},
		{"kênh mất", &youtube.ChannelNotFoundError{}, FailureFatal},
		{"khác", errors.New("mạng đứt"), FailureTransient},
	}
	for _, c := range cases {
		if got := ClassifyFailure(c.err); got.Kind != c.kind {
			t.Errorf("%s: kind=%v, muốn %v", c.name, got.Kind, c.kind)
		}
	}
	if got := ClassifyFailure(nil); got.Kind != FailureAbort {
		t.Fatalf("nil phải là abort, được %v", got.Kind)
	}
}

func TestSanitizeErrorHidesURLsAndTokens(t *testing.T) {
	raw := fmt.Errorf("lỗi https://www.youtube.com/watch?v=abc token=%s", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJ")
	got := SanitizeError(raw)
	if len(got) == 0 || got == raw.Error() {
		t.Fatalf("chẩn đoán phải được làm sạch: %q", got)
	}
	if len(got) > errorTextMax {
		t.Fatalf("vượt trần %d: %d", errorTextMax, len(got))
	}
	if SanitizeError(nil) != "" {
		t.Fatal("nil phải cho chuỗi rỗng")
	}
}

func TestBackoffDelayGrowsWithJitterAndCap(t *testing.T) {
	mid := func() float64 { return 0.5 }
	first := scanBackoffDelay(5*time.Second, 0, mid)
	if first != 5*time.Second {
		t.Fatalf("attempt 0 = %v, muốn 5s", first)
	}
	third := scanBackoffDelay(5*time.Second, 2, mid)
	if third != 20*time.Second {
		t.Fatalf("attempt 2 = %v, muốn 20s", third)
	}
	huge := scanBackoffDelay(5*time.Second, 40, mid)
	if huge > BackoffMax {
		t.Fatalf("trần backoff bị vượt: %v", huge)
	}
	low := scanBackoffDelay(5*time.Second, 0, func() float64 { return 0 })
	high := scanBackoffDelay(5*time.Second, 0, func() float64 { return 1 })
	if low >= first || high <= first {
		t.Fatalf("jitter phải hai phía: low=%v mid=%v high=%v", low, first, high)
	}
}

func TestTaskSpecReflectsJobState(t *testing.T) {
	base := &Job{ID: "j", ChannelID: "c", Kind: domain.ScanHistory, State: domain.ScanRunning}
	view, live := TaskPresentation(*base)
	if !live || view.Action.Op != domain.TaskOpCancel {
		t.Fatalf("đang chạy phải huỷ được: %+v live=%v", view, live)
	}
	paused := *base
	paused.State = domain.ScanPaused
	if view, _ := TaskPresentation(paused); view.Action.Op != domain.TaskOpResume {
		t.Fatalf("tạm dừng phải Tiếp tục, op=%v", view.Action.Op)
	}
	failed := *base
	failed.State = domain.ScanError
	failed.LastError = "YouTube chặn"
	if view, _ := TaskPresentation(failed); view.Action.Op != domain.TaskOpRetry {
		t.Fatalf("lỗi phải có thao tác Thử lại: %+v", view)
	}
}

// fakeStore chỉ cài các phương thức worker dùng khi rảnh; phần còn lại của
// scan.Store để nil (gọi tới sẽ panic — test không chạm).
type fakeStore struct {
	Store
	runnable  atomic.Int32
	selects   atomic.Int32
	wakeReads atomic.Int32
	// wakeup được trả cho NextWakeup (mặc định: không có việc, không có hạn).
	mu     sync.Mutex
	wakeup Wakeup
	// idleEntered đóng khi lần đọc lịch rảnh đầu tiên xong: rào đồng bộ cho
	// test đếm, tránh chụp trước khi worker thực sự vào rảnh.
	idleEntered chan struct{}
	once        sync.Once
}

func (f *fakeStore) markIdleRead() {
	if f.idleEntered == nil {
		return
	}
	f.once.Do(func() { close(f.idleEntered) })
}

func (f *fakeStore) RenewWorkerLease(context.Context, WorkerLease, time.Duration) (*WorkerLease, error) {
	return &WorkerLease{Owner: "o", Until: time.Now().Add(time.Minute)}, nil
}
func (f *fakeStore) ReleaseWorkerLease(context.Context, WorkerLease) error { return nil }
func (f *fakeStore) DueChannels(context.Context, int, time.Time) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) NextRunnableJob(context.Context, map[string]bool) (*Job, error) {
	f.runnable.Add(1)
	return nil, nil
}
func (f *fakeStore) NextWakeup(context.Context, int, time.Time) (Wakeup, error) {
	f.wakeReads.Add(1)
	f.mu.Lock()
	w := f.wakeup
	f.mu.Unlock()
	f.markIdleRead()
	return w, nil
}

func (f *fakeStore) setWakeup(w Wakeup) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wakeup = w
}
func (f *fakeStore) NextUploadCandidate(context.Context, time.Time) (*UploadCandidate, error) {
	f.selects.Add(1)
	return nil, nil
}

// fakeProvider chỉ để thoả Provider; test rảnh không gọi mạng.
type fakeProvider struct{ Provider }

// waitFor chờ tới khi điều kiện đúng hoặc hết hạn (test thời gian).
func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("hết giờ chờ %s", what)
}

// TestWorkerIdleDoesNotPoll: hai đối thủ, không việc nào đến hạn — chạy xong
// vòng đầu, worker ngủ; không lượt chọn việc / thăm dò nào lặp lại. Nhịp
// heartbeat (15 s) chỉ đọc lại hạn, không chọn việc.
func TestWorkerIdleDoesNotPoll(t *testing.T) {
	idle := make(chan struct{})
	store := &fakeStore{idleEntered: idle}
	w := NewWorker(WorkerDeps{Store: store, YouTube: fakeProvider{}})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = w.loop(ctx, "")
	}()
	// Rào: worker đã hoàn tất lần đọc lịch rảnh đầu tiên (nghĩa là đã đi qua
	// lượt chọn việc + thăm dò khởi đầu) rồi mới chụp bộ đếm.
	select {
	case <-idle:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("worker không vào nhịp rảnh")
	}
	t.Cleanup(func() { cancel(); <-done })
	first := store.runnable.Load()
	firstSelects := store.selects.Load()
	firstWakeReads := store.wakeReads.Load()

	// Mô phỏng 1 phút nhịp heartbeat: mỗi tick chỉ đọc lại hạn một lần.
	for i := 0; i < 4; i++ {
		w.tick <- struct{}{}
		time.Sleep(10 * time.Millisecond)
	}
	if got := store.runnable.Load(); got != first {
		t.Fatalf("worker chọn việc khi rảnh: %d → %d", first, got)
	}
	if got := store.selects.Load(); got != firstSelects {
		t.Fatalf("worker thăm dò khi rảnh: %d → %d", firstSelects, got)
	}
	if got := store.wakeReads.Load(); got <= firstWakeReads {
		t.Fatalf("nhịp heartbeat phải đọc lại lịch: %d → %d", firstWakeReads, got)
	}
}

// TestWorkerWakesAtNextDeadline: hạn kế tiếp ở tương lai đánh thức worker đúng
// lúc, không sớm hơn (không poll).
func TestWorkerWakesAtNextDeadline(t *testing.T) {
	store := &fakeStore{}
	due := time.Now().Add(80 * time.Millisecond)
	store.setWakeup(Wakeup{Next: &due})
	w := NewWorker(WorkerDeps{Store: store, YouTube: fakeProvider{}})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() { _ = w.loop(ctx, "") }()
	waitFor(t, "vòng chọn việc đầu", func() bool { return store.runnable.Load() >= 1 })
	first := store.runnable.Load()
	// Việc do process khác tạo chỉ lộ ra qua lần đọc lịch kế tiếp (nhịp
	// heartbeat): không có wake, không có timer mới — worker phải rời rảnh.
	store.setWakeup(Wakeup{Actionable: true})
	waitFor(t, "thức dậy khi có việc mới", func() bool { return store.runnable.Load() > first })
	cancel()
}

// TestWorkerIgnoresPastDeadline: hạn đã qua không được biến thành một vòng quay
// nhanh; chỉ nhịp heartbeat mới đọc lại lịch.
func TestWorkerIgnoresPastDeadline(t *testing.T) {
	store := &fakeStore{}
	past := time.Now().Add(-time.Minute)
	store.setWakeup(Wakeup{Next: &past})
	w := NewWorker(WorkerDeps{Store: store, YouTube: fakeProvider{}})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	go func() { _ = w.loop(ctx, "") }()
	waitFor(t, "vòng chọn việc đầu", func() bool { return store.runnable.Load() >= 1 })
	before := store.wakeReads.Load()
	time.Sleep(120 * time.Millisecond)
	if got := store.wakeReads.Load(); got > before+1 {
		t.Fatalf("hạn đã qua làm quay vòng: %d → %d lần đọc lịch", before, got)
	}
	cancel()
}

// TestWorkerStopsWhenLeaseExpired: đồng hồ cục bộ vượt hạn lease thì vòng lặp
// tự dừng (kể cả khi heartbeat goroutine đã chết).
func TestWorkerStopsWhenLeaseExpired(t *testing.T) {
	store := &fakeStore{}
	w := NewWorker(WorkerDeps{Store: store, YouTube: fakeProvider{}})
	w.leaseUntil.Store(time.Now().Add(-time.Second).UnixNano())
	err := w.loop(context.Background(), "")
	if !errors.Is(err, errWorkerLeaseExpired) {
		t.Fatalf("loop = %v, muốn errWorkerLeaseExpired", err)
	}
}
