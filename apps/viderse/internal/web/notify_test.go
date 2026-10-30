package web

import (
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/events"
)

// fakeClock đồng hồ + hẹn giờ tiêm được cho test notifier.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	fn    func()
	timer *time.Timer
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1000, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *fakeClock) After(_ time.Duration, fn func()) *time.Timer {
	t := time.NewTimer(time.Hour)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timers = append(c.timers, &fakeTimer{fn: fn, timer: t})
	return t
}

// Fire hẹn giờ gần nhất còn hiệu lực (giống một time.AfterFunc tới hạn).
func (c *fakeClock) Fire(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	if len(c.timers) == 0 {
		c.mu.Unlock()
		t.Fatal("không có hẹn giờ nào để bắn")
	}
	ft := c.timers[len(c.timers)-1]
	c.timers = c.timers[:len(c.timers)-1]
	c.mu.Unlock()
	if !ft.timer.Stop() {
		t.Fatal("hẹn giờ đã bị huỷ nhưng vẫn bị bắn")
	}
	ft.fn()
}

// frames ghi các frame đã phát từ hub.
type frames struct {
	mu   sync.Mutex
	data []statusData
}

func recordFrames(hub *events.Hub) *frames {
	f := &frames{}
	hub.OnPublish(func(ev events.Event) {
		d, ok := ev.Data.(statusData)
		if !ok {
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.data = append(f.data, d)
	})
	return f
}

func (f *frames) snapshot() []statusData {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]statusData(nil), f.data...)
}

// TestNotifierBurstCoalescesToLeadingAndTrailing: 20 lần đổi trong 100 ms chỉ
// phát 2 frame (leading + trailing), rev tăng dần, frame cuối mang lại ref.
func TestNotifierBurstCoalescesToLeadingAndTrailing(t *testing.T) {
	hub := events.New(nil)
	clock := newFakeClock()
	frames := recordFrames(hub)
	notify := newNotifier(hub, "activity", clock.Now, clock.After)

	for range 20 {
		notify("task-1")
		clock.Advance(5 * time.Millisecond)
	}
	if got := frames.snapshot(); len(got) != 1 || got[0].Rev != 1 || got[0].Ref != "task-1" {
		t.Fatalf("trước trailing = %+v, muốn một frame leading rev=1 ref=task-1", got)
	}
	clock.Advance(150 * time.Millisecond) // tổng 250 ms kể từ frame đầu
	clock.Fire(t)
	got := frames.snapshot()
	if len(got) != 2 {
		t.Fatalf("bùng 20 lần phát %d frame, muốn 2 (leading + trailing)", len(got))
	}
	if got[1].Rev != 2 || got[1].Ref != "task-1" {
		t.Fatalf("trailing = %+v, muốn rev=2 ref=task-1", got[1])
	}
}

// TestNotifierMixedRefsTrailingEmpty: các ref khác nhau trong cùng cửa sổ thì
// frame trailing mang ref rỗng (client làm mới mọi khoá).
func TestNotifierMixedRefsTrailingEmpty(t *testing.T) {
	hub := events.New(nil)
	clock := newFakeClock()
	frames := recordFrames(hub)
	notify := newNotifier(hub, "activity", clock.Now, clock.After)

	for i := range 20 {
		ref := "task-1"
		if i%2 == 1 {
			ref = "notice-1"
		}
		notify(ref)
		clock.Advance(5 * time.Millisecond)
	}
	clock.Advance(150 * time.Millisecond)
	clock.Fire(t)
	got := frames.snapshot()
	if len(got) != 2 {
		t.Fatalf("phát %d frame, muốn 2", len(got))
	}
	if got[1].Ref != "" {
		t.Fatalf("trailing ref = %q, muốn rỗng khi ref trộn", got[1].Ref)
	}
}

// TestNotifierSpacedCallsPublishImmediately: các lần đổi cách nhau hơn cửa sổ
// đều phát ngay, không cần trailing.
func TestNotifierSpacedCallsPublishImmediately(t *testing.T) {
	hub := events.New(nil)
	clock := newFakeClock()
	frames := recordFrames(hub)
	notify := newNotifier(hub, "activity", clock.Now, clock.After)

	notify("a")
	clock.Advance(251 * time.Millisecond)
	notify("b")
	clock.Advance(251 * time.Millisecond)
	notify("c")
	got := frames.snapshot()
	if len(got) != 3 {
		t.Fatalf("phát %d frame, muốn 3", len(got))
	}
	for i, want := range []string{"a", "b", "c"} {
		if got[i].Ref != want || got[i].Rev != uint64(i+1) {
			t.Fatalf("frame %d = %+v, muốn rev=%d ref=%q", i, got[i], i+1, want)
		}
	}
	if clock.timers != nil && len(clock.timers) != 0 {
		t.Fatalf("không được hẹn giờ khi các lần gọi cách xa nhau: %d timer", len(clock.timers))
	}
}
