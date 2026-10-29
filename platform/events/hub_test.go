// Kiểm tra hợp đồng nối lại của hub SSE. Ca đầu được chuyển nguyên từ
// apps/noverse/internal/entry/web/events_resume_test.go (phần generic); các ca
// khác bao phần hub đã tách (visibility, buffer chậm, Broadcast, Update).
package events

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var eventFrameID = regexp.MustCompile(`(?m)^id: (\d+)\nevent: [a-z.]+$`)

func lastEventFrameID(t *testing.T, body string) uint64 {
	t.Helper()
	all := eventFrameID.FindAllStringSubmatch(body, -1)
	if len(all) == 0 {
		t.Fatalf("không có frame nào mang id trong %q", body)
	}
	id, err := strconv.ParseUint(all[len(all)-1][1], 10, 64)
	if err != nil {
		t.Fatalf("id sự kiện lỗi %q: %v", all[len(all)-1][1], err)
	}
	return id
}

// streamWriter là ResponseWriter giả giữ toàn bộ body và báo hiệu mỗi lần ghi.
type streamWriter struct {
	mu     sync.Mutex
	body   strings.Builder
	signal chan struct{}
	header http.Header
}

func newStreamWriter() *streamWriter {
	return &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
}

func (w *streamWriter) Header() http.Header { return w.header }
func (w *streamWriter) WriteHeader(int)     {}
func (w *streamWriter) Flush()              {}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, _ := w.body.Write(p)
	select {
	case w.signal <- struct{}{}:
	default:
	}
	return n, nil
}

func (w *streamWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

// until chờ tới khi body chứa phrase rồi trả toàn bộ body.
func (w *streamWriter) until(t *testing.T, phrase string) string {
	t.Helper()
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	for {
		if strings.Contains(w.String(), phrase) {
			return w.String()
		}
		select {
		case <-w.signal:
		case <-timeout.C:
			t.Fatalf("hết giờ chờ %q; body hiện tại: %q", phrase, w.String())
		}
	}
}

func readSub(t *testing.T, ch chan Event) Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("không có frame trong 2s")
		return Event{}
	}
}

// serveStream mở một kết nối SSE thật qua Subscribe + Serve, trả writer, hàm
// đóng và kênh báo Serve đã thoát.
func serveStream(t *testing.T, hub *Hub, p any, last uint64) (*streamWriter, context.CancelFunc, chan struct{}) {
	t.Helper()
	sub, replay, stale := hub.Subscribe(p, last)
	ctx, cancel := context.WithCancel(context.Background())
	w := newStreamWriter()
	r := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		_ = hub.Serve(w, r, sub, replay, stale, Hooks{})
		hub.Unsubscribe(sub)
		close(done)
	}()
	return w, cancel, done
}

// 断开期间发布的事件在重连后补发一次，且不重复已确认的事件 — chuyển nguyên từ
// TestGlobalEventsResumeAfterDisconnectNoLossNoDuplicate của Noverse.
func TestEventsResumeAfterDisconnectNoLossNoDuplicate(t *testing.T) {
	hub := New(nil)

	w, cancel, done := serveStream(t, hub, "u1", 0)
	w.until(t, "event: hello")
	first := hub.Publish(Event{Type: "story.changed", Data: map[string]any{"parts": []string{"snapshot"}}})
	second := hub.Publish(Event{Type: "story.changed", Data: map[string]any{"parts": []string{"snapshot"}}})
	body := w.until(t, fmt.Sprintf("id: %d\nevent: story.changed", second))
	if got := lastEventFrameID(t, body); got != second {
		t.Fatalf("id xác nhận cuối = %d, muốn %d", got, second)
	}
	cancel()
	<-done

	// Hai sự kiện phát khi kết nối đã đứt.
	third := hub.Publish(Event{Type: "story.changed", Data: map[string]any{"parts": []string{"chapters"}}})
	fourth := hub.Publish(Event{Type: "story.changed", Data: map[string]any{"parts": []string{"reviews"}}})

	w2, cancel2, done2 := serveStream(t, hub, "u1", second)
	replay := w2.until(t, fmt.Sprintf("id: %d\nevent: story.changed", fourth))
	cancel2()
	<-done2

	if strings.Contains(replay, "event: reset") {
		t.Fatalf("con trỏ còn sống không được reset: %q", replay)
	}
	for _, id := range []uint64{first, second} {
		if strings.Contains(replay, fmt.Sprintf("id: %d\n", id)) {
			t.Fatalf("phát lại sự kiện đã xác nhận %d: %q", id, replay)
		}
	}
	for _, id := range []uint64{third, fourth} {
		if got := strings.Count(replay, fmt.Sprintf("id: %d\nevent: story.changed", id)); got != 1 {
			t.Fatalf("sự kiện %d được gửi %d lần: %q", id, got, replay)
		}
	}
}

// Con trỏ đã rơi ra ngoài ring (mất hơn RingSize sự kiện) phải phát reset.
func TestResumeGapEmitsReset(t *testing.T) {
	hub := New(nil)
	first := hub.Publish(Event{Type: "story.changed", Data: map[string]any{}})
	for range RingSize + 1 {
		hub.Publish(Event{Type: "story.changed", Data: map[string]any{}})
	}

	// Tầng API Subscribe: con trỏ cũ phải báo stale, không còn gì để replay.
	_, replay, stale := hub.Subscribe("u", first)
	if !stale || len(replay) != 0 {
		t.Fatalf("stale=%v replay=%d, muốn stale=true", stale, len(replay))
	}

	// Tầng dây: kết nối nối lại nhận hello rồi reset, không frame id nào cũ.
	w, cancel, done := serveStream(t, hub, "u", first)
	body := w.until(t, "event: reset")
	cancel()
	<-done
	if !strings.Contains(body, "event: reset") {
		t.Fatalf("thiếu frame reset: %q", body)
	}
	if strings.Contains(body, fmt.Sprintf("id: %d\n", first)) {
		t.Fatalf("reset còn kèm sự kiện cũ: %q", body)
	}
}

// Con trỏ lớn hơn seq hiện tại cũng là stale — client đang cầm con trỏ của hub
// khác (sau khi restart) nên phải reset thay vì im lặng chờ id không tồn tại.
func TestResumeBeyondSequenceEmitsReset(t *testing.T) {
	hub := New(nil)
	hub.Publish(Event{Type: "a", Data: map[string]any{}})
	w, cancel, done := serveStream(t, hub, "u", 99)
	body := w.until(t, "event: reset")
	cancel()
	<-done
	if !strings.Contains(body, "event: reset") {
		t.Fatalf("con trỏ quá seq phải reset: %q", body)
	}
}

// Luật lọc áp cả lúc phát trực tiếp lẫn lúc phát lại từ ring.
func TestVisibilityFiltersLiveAndReplay(t *testing.T) {
	hub := New(func(p any, ev Event) bool {
		meta, _ := ev.Meta.(string)
		return meta == "" || meta == p
	})
	pub := Event{Type: "pub", Data: "x"}
	priv := Event{Type: "priv", Data: "y", Meta: "alice"}
	sub, replay, _ := hub.Subscribe("alice", 0)
	defer hub.Unsubscribe(sub)
	pubID := hub.Publish(pub)
	hub.Publish(priv)
	if ev := readSub(t, sub.C); ev.Type != "pub" {
		t.Fatalf("frame 1 = %v", ev)
	}
	if ev := readSub(t, sub.C); ev.Type != "priv" {
		t.Fatalf("frame 2 = %v", ev)
	}

	bob, replay, _ := hub.Subscribe("bob", 0)
	defer hub.Unsubscribe(bob)
	if len(replay) != 0 {
		t.Fatalf("kết nối mới (con trỏ 0) không được replay lịch sử: %v", replay)
	}
	// Con trỏ hợp lệ: replay chỉ trả phần sau con trỏ mà bob được thấy.
	hub.Publish(Event{Type: "pub2", Data: "z"})
	bob2, replay, _ := hub.Subscribe("bob", pubID)
	defer hub.Unsubscribe(bob2)
	if len(replay) != 1 || replay[0].Type != "pub2" {
		t.Fatalf("replay cho bob = %v", replay)
	}
}

// Subscriber chậm (buffer đầy) bị cắt kênh nhưng ring vẫn giữ sự kiện để nối
// lại không mất gì.
func TestSlowSubscriberDroppedThenResumes(t *testing.T) {
	hub := New(nil)
	sub, _, _ := hub.Subscribe("u", 0)
	// Lấp đầy buffer 256 + một: hub phải đóng kênh thay vì chặn Publish.
	first := hub.Publish(Event{Type: "e", Data: 0})
	for i := 1; i <= subBuffer; i++ {
		hub.Publish(Event{Type: "e", Data: i})
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, ok := <-sub.C; !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("kênh subscriber chậm không bị đóng")
		}
	}
	// Nối lại từ con trỏ đã xác nhận: ring trả đúng phần còn thiếu.
	sub2, replay, stale := hub.Subscribe("u", first)
	defer hub.Unsubscribe(sub2)
	if stale || len(replay) != subBuffer {
		t.Fatalf("stale=%v replay=%d", stale, len(replay))
	}
}

// Broadcast chuyển frame tức thời không cấp id và không chiếm chỗ ring.
func TestBroadcastStaysOutOfRing(t *testing.T) {
	hub := New(nil)
	sub, _, _ := hub.Subscribe("u", 0)
	defer hub.Unsubscribe(sub)
	hub.Broadcast(Event{Type: "live", Data: map[string]any{}})
	if ev := readSub(t, sub.C); ev.ID != 0 || ev.Type != "live" {
		t.Fatalf("broadcast frame = %+v", ev)
	}
	if hub.Seq() != 0 {
		t.Fatalf("broadcast đốt số seq: %d", hub.Seq())
	}
	// Ring rỗng: nối lại từ 0 không replay gì.
	_, replay, stale := hub.Subscribe("u", 0)
	if stale || len(replay) != 0 {
		t.Fatalf("broadcast lọt vào ring: replay=%d stale=%v", len(replay), stale)
	}
}

// Update đổi principal của kết nối đang sống và cắt những kết nối keep=false.
func TestUpdateSwapsPrincipalAndDrops(t *testing.T) {
	hub := New(func(p any, ev Event) bool { return ev.Meta == p })
	live, _, _ := hub.Subscribe("alice", 0)
	defer hub.Unsubscribe(live)
	dead, _, _ := hub.Subscribe("mallory", 0)

	hub.Update(func(p any) (any, bool) {
		if p == "mallory" {
			return p, false // thu hồi phiên
		}
		return "bob", true // principal mới của alice
	})

	if _, ok := <-dead.C; ok {
		t.Fatal("kết nối keep=false phải đóng")
	}
	hub.Publish(Event{Type: "e", Data: 1, Meta: "bob"})
	if ev := readSub(t, live.C); ev.Type != "e" {
		t.Fatalf("principal mới không nhận sự kiện: %+v", ev)
	}
	ok := hub.WithSub(live.ID, func(s *Sub) bool { return s.Principal() == "bob" })
	if !ok {
		t.Fatal("principal của alice không được đổi sang bob")
	}
}

// TrySend chỉ đẩy được tới subscriber còn sống và không chặn khi buffer đầy.
func TestTrySendBackfill(t *testing.T) {
	hub := New(nil)
	sub, _, _ := hub.Subscribe("u", 0)
	defer hub.Unsubscribe(sub)
	if !hub.TrySend(sub.ID, Event{Type: "backfill", Data: "x"}) {
		t.Fatal("TrySend tới subscriber sống phải thành công")
	}
	if ev := readSub(t, sub.C); ev.Type != "backfill" {
		t.Fatalf("frame = %+v", ev)
	}
	if hub.TrySend("không-tồn-tại", Event{Type: "x"}) {
		t.Fatal("TrySend tới subscriber không tồn tại phải false")
	}
}

// ResumeID đọc header trước rồi mới tới query (EventSource chỉ gửi header).
func TestResumeIDHeaderThenQuery(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("Last-Event-ID", "12")
	if got := ResumeID(r); got != 12 {
		t.Fatalf("header = %d", got)
	}
	r = httptest.NewRequest(http.MethodGet, "/x?lastEventId=34", nil)
	if got := ResumeID(r); got != 34 {
		t.Fatalf("query = %d", got)
	}
	r = httptest.NewRequest(http.MethodGet, "/x?lastEventId=khoẻ", nil)
	if got := ResumeID(r); got != 0 {
		t.Fatalf("rác = %d", got)
	}
}

// Frame hello là frame đầu tiên, không id, mang subscriber id của kết nối.
func TestHelloFrameNamesSubscriber(t *testing.T) {
	hub := New(nil)
	sub, replay, stale := hub.Subscribe("u", 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := newStreamWriter()
	r := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() { _ = hub.Serve(w, r, sub, replay, stale, Hooks{}); close(done) }()
	body := w.until(t, "event: hello")
	cancel()
	<-done
	want := fmt.Sprintf("event: hello\ndata: {\"subscriber\":\"%s\"}", sub.ID)
	if !strings.Contains(body, want) {
		t.Fatalf("hello = %q, muốn %q", body, want)
	}
	if strings.HasPrefix(body, "id:") {
		t.Fatalf("hello không được mang id: %q", body)
	}
}

// Writer không hỗ trợ Flush báo ErrNotFlushable thay vì panic.
func TestServeRejectsNonFlusher(t *testing.T) {
	hub := New(nil)
	sub, replay, stale := hub.Subscribe("u", 0)
	defer hub.Unsubscribe(sub)
	r := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	err := hub.Serve(struct{ http.ResponseWriter }{httptest.NewRecorder()}, r, sub, replay, stale, Hooks{})
	if err != ErrNotFlushable {
		t.Fatalf("err = %v", err)
	}
}

// Backfill đọc principal hiện hành và lọc lại qua visible tại thời điểm enqueue:
// principal bị Update đổi (hạ quyền, không cắt kết nối) giữa subscribe và
// backfill phải khiến batch không gửi frame bị cấm — như enqueue trực tiếp
// TrySend của bản cũ.
func TestBackfillUsesCurrentPrincipalVisibility(t *testing.T) {
	hub := New(func(p any, ev Event) bool {
		meta, _ := ev.Meta.(string)
		return meta == "" || meta == p
	})
	sub, _, _ := hub.Subscribe("alice", 0)
	defer hub.Unsubscribe(sub)

	// Hạ quyền: alice không còn thấy scope "alice" mà chỉ thấy "public".
	hub.Update(func(p any) (any, bool) {
		if p == "alice" {
			return "public", true
		}
		return p, true
	})

	ok := hub.Backfill(sub.ID, func(s *Sub) ([]Event, bool) {
		if s.Principal() != "public" {
			t.Fatalf("fill phải thấy principal hiện hành, được %v", s.Principal())
		}
		return []Event{
			{Type: "delta", Meta: "alice"}, // bị cấm sau hạ quyền
			{Type: "delta", Meta: "public"},
		}, true
	})
	if !ok {
		t.Fatal("backfill bị từ chối dù subscriber còn và fill ok")
	}
	ev := readSub(t, sub.C)
	if meta, _ := ev.Meta.(string); meta != "public" {
		t.Fatalf("frame đầu = %+v, chỉ frame còn quyền mới được gửi", ev)
	}
	select {
	case ev := <-sub.C:
		t.Fatalf("lọt frame bị cấm sau hạ quyền: %+v", ev)
	default:
	}
}

// Batch của Backfill là nguyên khối so với Broadcast: frame live không được
// chen vào giữa các frame backfill (bảo vệ thứ tự clear→delta của app).
func TestBackfillBatchIsAtomicAgainstBroadcast(t *testing.T) {
	for attempt := range 32 {
		hub := New(nil)
		sub, _, _ := hub.Subscribe("u", 0)

		broadcast := make(chan struct{})
		go func() {
			<-broadcast
			hub.Broadcast(Event{Type: "live"})
		}()

		ok := hub.Backfill(sub.ID, func(s *Sub) ([]Event, bool) {
			close(broadcast)
			// Nới lỏng cho broadcast cố chen vào giữa batch.
			for i := 0; i < 50; i++ {
				// Vòng quay nhỏ — chỉ hub lock mới ngăn được xen kẽ.
			}
			return []Event{{Type: "a"}, {Type: "b"}, {Type: "c"}}, true
		})
		if !ok {
			t.Fatalf("lần %d: backfill thất bại", attempt)
		}
		defer hub.Unsubscribe(sub)

		// Broadcast bị giữ ngoài khoá trong lúc fill chạy xong batch, nên frame
		// "live" chắc chắn tới: đọc đủ 4 frame rồi mới kiểm thứ tự.
		var types []string
		for range 4 {
			types = append(types, readSub(t, sub.C).Type)
		}
		got := strings.Join(types, ",")
		if got != "a,b,c,live" {
			t.Fatalf("lần %d: batch không nguyên khối: %s", attempt, got)
		}
	}
}

// fill trả ok=false từ chối cả batch — cơ chế phía app dùng để kiểm quyền.
func TestBackfillFillRejects(t *testing.T) {
	hub := New(nil)
	sub, _, _ := hub.Subscribe("u", 0)
	defer hub.Unsubscribe(sub)
	if hub.Backfill(sub.ID, func(s *Sub) ([]Event, bool) {
		return []Event{{Type: "x"}}, false
	}) {
		t.Fatal("fill ok=false phải khiến Backfill trả false")
	}
	select {
	case ev := <-sub.C:
		t.Fatalf("batch bị từ chối vẫn được gửi: %+v", ev)
	default:
	}
}
