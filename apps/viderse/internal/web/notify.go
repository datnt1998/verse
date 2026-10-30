package web

import (
	"sync"
	"time"

	"github.com/datnt1998/verse/platform/events"
)

// notifyWindow là cửa sổ gộp sự kiện SSE của một loại: tối đa 4 frame/giây,
// frame cuối luôn được phát sau cơn bùng (trailing).
const notifyWindow = 250 * time.Millisecond

// Notifier trả hàm phát sự kiện SSE kiểu typ cho mọi phiên đăng nhập, data
// {"rev":N,"ref":"<id>"?}. rev tăng dần theo từng kiểu sự kiện trong tiến
// trình; ref (có thể rỗng) là id của đối tượng vừa đổi để client chỉ làm mới
// đúng khoá. Gói nghiệp vụ nhận hàm này qua constructor (kiểu func(ref
// string)), không import web hay platform/events.
//
// Mỗi notifier gộp nhịp cho CẢ luồng sự kiện của loại: frame đầu phát ngay,
// các thay đổi trong 250 ms kế tiếp gộp vào đúng một frame trailing (ref =
// ref cuối nếu mọi ref gộp bằng nhau, ngược lại rỗng) nên stream ≤ 4 frame/s
// mà trạng thái cuối vẫn tới client. Chỉ dùng một time.AfterFunc một lần cho
// mỗi cửa sổ, không có goroutine thường trực.
func Notifier(hub *events.Hub, typ string) func(ref string) {
	return newNotifier(hub, typ, time.Now, time.AfterFunc)
}

// newNotifier dựng notifier với đồng hồ/hẹn giờ tiêm được cho test.
func newNotifier(hub *events.Hub, typ string, now func() time.Time, after func(time.Duration, func()) *time.Timer) func(ref string) {
	n := &notifier{hub: hub, typ: typ, now: now, after: after}
	return n.publish
}

type notifier struct {
	hub   *events.Hub
	typ   string
	now   func() time.Time
	after func(time.Duration, func()) *time.Timer

	mu      sync.Mutex
	rev     uint64
	last    time.Time
	hasLast bool
	pending bool
	lastRef string
	mixed   bool
	timer   *time.Timer
}

// publish phát ngay khi đã đủ cửa sổ; nếu chưa thì gộp vào frame trailing.
func (n *notifier) publish(ref string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := n.now()
	if !n.hasLast || now.Sub(n.last) >= notifyWindow {
		n.sendLocked(ref, now)
		n.clearPendingLocked()
		return
	}
	if !n.pending {
		n.pending = true
		n.lastRef = ref
		n.mixed = false
	} else if ref != n.lastRef {
		n.mixed = true
	}
	if n.timer == nil {
		n.timer = n.after(notifyWindow-now.Sub(n.last), n.flush)
	}
}

// flush phát frame trailing cho các thay đổi đã gộp rồi hẹn cửa sổ kế nếu có.
func (n *notifier) flush() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.timer = nil
	if !n.pending {
		return
	}
	now := n.now()
	if now.Sub(n.last) < notifyWindow {
		// Hẹn giờ bắn sớm: chờ đủ cửa sổ thay vì vượt trần 4 frame/s.
		n.timer = n.after(notifyWindow-now.Sub(n.last), n.flush)
		return
	}
	ref := n.lastRef
	if n.mixed {
		ref = ""
	}
	n.sendLocked(ref, now)
	n.clearPendingLocked()
}

func (n *notifier) sendLocked(ref string, now time.Time) {
	n.rev++
	n.last = now
	n.hasLast = true
	n.hub.Publish(events.Event{
		Type: n.typ,
		Data: statusData{Rev: n.rev, Ref: ref},
		Meta: eventScope{},
	})
}

func (n *notifier) clearPendingLocked() {
	n.pending = false
	n.lastRef = ""
	n.mixed = false
	if n.timer != nil {
		n.timer.Stop()
		n.timer = nil
	}
}
