package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/platform/events"
	"github.com/datnt1998/verse/platform/identity"
)

// 续传回归：手机/代理掐断 SSE 后带 Last-Event-ID 重连，必须做到不丢不重。
// 全局 /api/events 的通用续传契约已随 hub 迁到 platform/events
// （hub_test.go 的 TestEventsResumeAfterDisconnectNoLossNoDuplicate）；本文件
// 只留 Noverse 私有路径的回归：worker 直连流、实况帧可见性与关注集补发。

var eventFrameID = regexp.MustCompile(`(?m)^id: (\d+)\nevent: [a-z.]+$`)

func lastEventFrameID(t *testing.T, body string) uint64 {
	t.Helper()
	all := eventFrameID.FindAllStringSubmatch(body, -1)
	if len(all) == 0 {
		t.Fatalf("no event frame with id in %q", body)
	}
	id, err := strconv.ParseUint(all[len(all)-1][1], 10, 64)
	if err != nil {
		t.Fatalf("bad event id %q: %v", all[len(all)-1][1], err)
	}
	return id
}

func readFrame(t *testing.T, ch chan events.Event) events.Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("no live frame within 2s")
		return events.Event{}
	}
}

// 全局 /api/events：断开期间发布的事件在重连后补发一次，且不重复已确认的事件。
// 覆盖整条 Noverse 链路（subscribe → Serve → Meta 还原），通用部分的矩阵在
// platform/events。
func TestGlobalEventsResumeAfterDisconnectNoLossNoDuplicate(t *testing.T) {
	tg := newTestGateway(t)
	hub := tg.g.events()
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}

	open := func(last uint64) (*streamWriter, context.CancelFunc, chan struct{}) {
		ctx, cancel := context.WithCancel(context.Background())
		w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
		r := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
		if last > 0 {
			r.Header.Set("Last-Event-ID", strconv.FormatUint(last, 10))
		}
		done := make(chan struct{})
		go func() { tg.g.serveEvents(w, r, admin); close(done) }()
		return w, cancel, done
	}

	w, cancel, done := open(0)
	w.until(t, "event: hello")
	first := hub.Publish(Event{Type: "story.changed", Project: "proj", Story: "s", Data: map[string]any{"project": "proj", "story": "s", "parts": []string{"snapshot"}}})
	second := hub.Publish(Event{Type: "story.changed", Project: "proj", Story: "s", Data: map[string]any{"project": "proj", "story": "s", "parts": []string{"snapshot"}}})
	body := w.until(t, fmt.Sprintf("id: %d\nevent: story.changed", second))
	if got := lastEventFrameID(t, body); got != second {
		t.Fatalf("last confirmed id = %d, want %d", got, second)
	}
	cancel()
	<-done

	// 掉线期间发生两件事。
	third := hub.Publish(Event{Type: "story.changed", Project: "proj", Story: "s", Data: map[string]any{"project": "proj", "story": "s", "parts": []string{"chapters"}}})
	fourth := hub.Publish(Event{Type: "story.changed", Project: "proj", Story: "s", Data: map[string]any{"project": "proj", "story": "s", "parts": []string{"reviews"}}})

	w2, cancel2, done2 := open(second)
	replay := w2.until(t, fmt.Sprintf("id: %d\nevent: story.changed", fourth))
	cancel2()
	<-done2

	if strings.Contains(replay, "event: reset") {
		t.Fatalf("resume must not reset a live cursor: %q", replay)
	}
	for _, id := range []uint64{first, second} {
		if strings.Contains(replay, fmt.Sprintf("id: %d\n", id)) {
			t.Fatalf("replayed already-confirmed event %d: %q", id, replay)
		}
	}
	for _, id := range []uint64{third, fourth} {
		if got := strings.Count(replay, fmt.Sprintf("id: %d\nevent: story.changed", id)); got != 1 {
			t.Fatalf("event %d delivered %d times: %q", id, got, replay)
		}
	}
}

// 故事实况流：重连带 Last-Event-ID → 不 reset、只补断开期间的事件、不重复。
func TestStoryLiveResumeAfterDisconnectNoLossNoDuplicate(t *testing.T) {
	h := newFakeHost()
	handler := NewWorkerHandler(h, "secret")

	open := func(last uint64) (*streamWriter, context.CancelFunc, chan struct{}) {
		ctx, cancel := context.WithCancel(context.Background())
		w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
		r := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
		r.Header.Set("X-Ainovel-Worker-Token", "secret")
		if last > 0 {
			r.Header.Set("Last-Event-ID", strconv.FormatUint(last, 10))
		}
		done := make(chan struct{})
		go func() { handler.ServeHTTP(w, r); close(done) }()
		return w, cancel, done
	}

	w, cancel, done := open(0)
	w.until(t, "event: snapshot")
	h.events <- host.Event{ID: "a", Summary: "A"}
	h.events <- host.Event{ID: "b", Summary: "B"}
	body := w.until(t, `"id":"b"`)
	confirmed := lastEventFrameID(t, body)
	cancel()
	<-done

	// 掉线期间写完两步。
	h.events <- host.Event{ID: "c", Summary: "C"}
	h.events <- host.Event{ID: "d", Summary: "D"}

	w2, cancel2, done2 := open(confirmed)
	replay := w2.until(t, `"id":"d"`)
	cancel2()
	<-done2

	if !strings.Contains(replay, "event: snapshot") {
		t.Fatalf("resume must resend a snapshot: %q", replay)
	}
	if strings.Contains(replay, "event: reset") {
		t.Fatalf("resume must not wipe the client log: %q", replay)
	}
	for _, id := range []string{`"id":"a"`, `"id":"b"`} {
		if strings.Contains(replay, id) {
			t.Fatalf("replayed already-delivered %s: %q", id, replay)
		}
	}
	for _, id := range []string{`"id":"c"`, `"id":"d"`} {
		if strings.Count(replay, id) != 1 {
			t.Fatalf("event %s delivered %d times: %q", id, strings.Count(replay, id), replay)
		}
	}
}

// 实况帧：只发给能读该故事的人，且不进续传环（delta 每秒几十帧，不能把 500 条
// 全局事件挤掉）。
func TestLiveFramesRespectVisibilityAndStayOutOfResumeRing(t *testing.T) {
	hub := newEventHub()
	reader := identity.User{Username: "r", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"p": identity.ProjectView}}
	other := identity.User{Username: "o", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"x": identity.ProjectView}}
	mine, _, _ := hub.subscribe(reader, 0, []string{"p/s"})
	defer hub.unsubscribe(mine)
	theirs, _, _ := hub.subscribe(other, 0, nil)
	defer hub.unsubscribe(theirs)

	g := &Gateway{eventHub: hub}
	mux := newLiveMux(g, "p", "s")
	mux.frame("event", []byte(`{"id":"e1","summary":"xong"}`))
	mux.frame("delta", []byte(`{"text":"một đoạn"}`))

	first := readFrame(t, mine.C)
	if first.Type != liveDataType {
		t.Fatalf("frame type = %q", first.Type)
	}
	select {
	case ev := <-theirs.C:
		t.Fatalf("live frame leaked to a reader without the story: %+v", ev)
	default:
	}

	backlog := hub.Backlog([]string{"p/s"})
	kinds := make([]string, 0, len(backlog))
	for _, ev := range backlog {
		frame, ok := ev.Data.(liveFrame)
		if !ok {
			t.Fatalf("backlog frame type %T", ev.Data)
		}
		kinds = append(kinds, frame.Kind)
	}
	want := strings.Join(kinds, ",")
	if want != "events,clear,delta" {
		t.Fatalf("backlog = %s", want)
	}
	// 瞬时帧不占全局序号：新连接从 lastID=0 开始拿不到任何“历史”。
	if hub.inner.Seq() != 0 {
		t.Fatalf("live frames consumed the resume sequence: seq=%d", hub.inner.Seq())
	}
}

// 新增关注范围时立刻补发实况尾部（切到故事页不必等下一次重连）。
func TestSetWatchBackfillsLiveTail(t *testing.T) {
	hub := newEventHub()
	user := identity.User{Username: "r", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"p": identity.ProjectView}}
	hub.AppendLiveEvent("p/s", json.RawMessage(`{"id":"e1"}`))
	hub.SetLiveStream("p/s", "đang viết")
	sub, _, _ := hub.subscribe(user, 0, nil)
	defer hub.unsubscribe(sub)

	if !hub.SetWatch(sub.ID, "r", []string{"p/s"}) {
		t.Fatal("SetWatch must accept the live subscriber")
	}
	var kinds []string
	for len(kinds) < 3 {
		ev := readFrame(t, sub.C)
		frame, ok := ev.Data.(liveFrame)
		if !ok {
			t.Fatalf("frame data %T", ev.Data)
		}
		kinds = append(kinds, frame.Kind)
	}
	if got := strings.Join(kinds, ","); got != "events,clear,delta" {
		t.Fatalf("backfill = %s", got)
	}
}

// Sau khi tiến trình khởi động lại, hub mới có seq nhỏ hơn Last-Event-ID mà
// trình duyệt còn giữ: client phải nhận reset ngay, không được im lặng chờ một
// id không bao giờ tới. Giữ nguyên hành vi của hub trước khi tách sang
// platform/events (lastID > seq → stale).
func TestGlobalEventsResumeAfterRestartEmitsReset(t *testing.T) {
	tg := newTestGateway(t)
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}
	tg.g.events().Publish(Event{Type: "story.changed", Project: "proj", Story: "s", Data: map[string]any{"parts": []string{"snapshot"}}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	r.Header.Set("Last-Event-ID", "99")
	w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	done := make(chan struct{})
	go func() { tg.g.serveEvents(w, r, admin); close(done) }()

	body := w.until(t, "event: reset")
	cancel()
	<-done
	if strings.Contains(body, "event: story.changed") {
		t.Fatalf("sau restart không được phát lại sự kiện cũ: %q", body)
	}
	if !strings.Contains(body, "event: hello") {
		t.Fatalf("thiếu hello trước reset: %q", body)
	}
}

// 降权回归（R1）：RefreshUser 在 watch 补发前后换掉 principal 但不断线时，
// 不允许任何被禁故事的实况帧进 channel。旧的 TrySend 补发不查可见性回调，
// 检查与发送之间的窗口会把 backlog 漏过去；现在整个批次在 hub 锁内以当前
// principal 复查可见性。
func TestSetWatchBackfillRespectsRefreshedPermissions(t *testing.T) {
	hub := newEventHub()
	reader := identity.User{Username: "r", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"p": identity.ProjectView}}
	hub.SetLiveStream("p/s", "đang viết")
	hub.AppendLiveEvent("p/s", json.RawMessage(`{"id":"e1"}`))
	sub, _, _ := hub.subscribe(reader, 0, nil)
	defer hub.unsubscribe(sub)

	// Thu hồi quyền đọc "p" nhưng giữ kết nối sống (không khoá tài khoản).
	hub.RefreshUser(identity.User{Username: "r", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{}})

	if !hub.SetWatch(sub.ID, "r", []string{"p/s"}) {
		t.Fatal("SetWatch vẫn phải chấp nhận — chỉ batch bị chặn, watch vẫn được đổi")
	}
	select {
	case ev := <-sub.C:
		t.Fatalf("frame bị cấm vẫn lọt sau khi hạ quyền: %+v", ev)
	case <-time.After(200 * time.Millisecond):
	}
}

// 时序回归（R2）：worker 的瞬时 delta 与 watch 补发并发时，客户端按帧序
// 重建的正文必须正好是 "AB" —— 无论补发抓在 delta B 之前还是之后。
// 两种交错都合法（B 先广播则尾巴抓成 "AB"；后广播则尾巴 "A" 再由 live B
// 补上），但任一帧插进批次内部都会得到 "BA" 或丢帧。
func TestSetWatchBackfillAndLiveDeltaReconstructInOrder(t *testing.T) {
	for attempt := range 16 {
		hub := newEventHub()
		user := identity.User{Username: "r", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"p": identity.ProjectView}}

		g := &Gateway{eventHub: hub}
		mux := newLiveMux(g, "p", "s")
		// 尾巴 "A" 在订阅者出现前就流完：channel 里没有它的 live 帧。
		mux.frame("delta", []byte(`{"text":"A"}`))

		sub, _, _ := hub.subscribe(user, 0, nil)

		live := make(chan struct{})
		go func() {
			defer close(live)
			mux.frame("delta", []byte(`{"text":"B"}`))
			mux.flush(true) // bỏ qua gom 80 ms để frame ra ngay trong test
		}()
		if !hub.SetWatch(sub.ID, "r", []string{"p/s"}) {
			t.Fatalf("lần %d: SetWatch thất bại", attempt)
		}
		<-live

		// Đúng 3 frame tới: clear + delta(backfill) + delta(live), thứ tự tuỳ
		// 交错 —— mô phỏng client append/clear để kiểm thứ tự hợp lệ.
		var text string
		for range 3 {
			ev := readFrame(t, sub.C)
			frame, ok := ev.Data.(liveFrame)
			if !ok {
				t.Fatalf("lần %d: frame %T", attempt, ev.Data)
			}
			switch frame.Kind {
			case "clear":
				text = ""
			case "delta":
				text += frameText(frame.Data)
			}
		}
		if text != "AB" {
			t.Fatalf("lần %d: text rebuilt = %q, muốn %q", attempt, text, "AB")
		}
		hub.unsubscribe(sub)
	}
}
