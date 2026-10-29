// Package events là hub SSE dùng chung của các app: mỗi sự kiện có id tăng
// dần, một ring giữ lại RingSize sự kiện gần nhất để kết nối nối lại bằng
// Last-Event-ID, và quá hạn đệm thì phát reset để client tải lại toàn bộ.
//
// App quyết định hai thứ khi dựng hub: kiểu principal (thông tin đăng nhập gắn
// với mỗi kết nối — user, session…) và luật lọc sự kiện theo principal đó.
// Meta của Event chở ngữ cảnh app cần cho luật lọc; hub không đọc nó. Tag của
// Sub chở dữ liệu phụ trợ app gắn theo kết nối, đọc/ghi qua WithSub và
// ForEachSub dưới khoá của hub nên không thể đấu với vòng đời subscriber.
package events

import (
	"crypto/rand"
	"sync"
	"time"
)

// RingSize là dung lượng bộ đệm phát lại: tối đa bao nhiêu sự kiện gần nhất một
// kết nối có thể xin lại bằng Last-Event-ID.
const RingSize = 500

// PingInterval là nhịp giữ kết nối mặc định. iOS Safari và proxy gia đình cắt
// luồng HTTP lâu không có byte; một comment mỗi 15 giây sống qua được cả hai.
// App cần phát hiện mất kết nối nhanh hơn truyền Option PingEvery cho New.
const PingInterval = 15 * time.Second

// subBuffer là số sự kiện xếp chờ mỗi kết nối trước khi hub coi kết nối là chậm
// và cắt nó (client sẽ nối lại bằng Last-Event-ID).
const subBuffer = 256

// Event là một sự kiện trên luồng. ID do Publish gán (Broadcast và các frame
// app tự viết để ID = 0 → ghi không dòng id:). Meta là ngữ cảnh riêng của app
// cho luật lọc visibility; hub không diễn giải nó.
type Event struct {
	ID   uint64
	Type string
	Data any
	Meta any
}

// Visible quyết định principal p có được nhận sự kiện ev không (khi phát trực
// tiếp lẫn khi phát lại từ ring). Nil nghĩa là mọi sự kiện đều hiển thị.
type Visible func(p any, ev Event) bool

// Sub là một kết nối đang đăng ký. Hub ghi vào C và đóng nó khi subscriber bị
// gỡ (Unsubscribe, buffer đầy, Update trả keep=false, Close); phía đọc không
// được ghi vào C.
//
// Tag là dữ liệu phụ trợ của app gắn theo kết nối (ví dụ tập khoá theo dõi);
// chỉ đọc/ghi trong callback của WithSub/ForEachSub khi hub đang giữ khoá.
type Sub struct {
	ID  string
	C   chan Event
	Tag any

	p any
}

// Principal trả principal app đưa vào lúc Subscribe. Chỉ được đọc trong
// callback của WithSub/ForEachSub (hub giữ khoá) — đọc ngoài có thể đấu với
// Update.
func (s *Sub) Principal() any { return s.p }

// Hub là trung tâm sự kiện của một app. Mọi phương thức an toàn cho goroutine;
// callback của WithSub/ForEachSub chạy dưới khoá hub nên không được gọi ngược
// vào phương thức của hub.
type Hub struct {
	mu           sync.Mutex
	visible      Visible
	pingInterval time.Duration
	seq          uint64
	ring         []Event
	ringStart    int // vị trí sự kiện cũ nhất còn giữ
	subs         map[string]*Sub
	onPublish    []func(Event)
}

// Option tuỳ chỉnh hub lúc dựng.
type Option func(*Hub)

// PingEvery đặt nhịp comment giữ kết nối của app này (mặc định PingInterval).
// Chọn nhịp ngắn hơn khi client cần phát hiện mất kết nối sớm — comment ping
// là tín hiệu sống duy nhất trên luồng.
func PingEvery(d time.Duration) Option {
	return func(h *Hub) {
		if d > 0 {
			h.pingInterval = d
		}
	}
}

// New dựng hub rỗng với luật lọc visible và các Option của app.
func New(visible Visible, opts ...Option) *Hub {
	if visible == nil {
		visible = func(any, Event) bool { return true }
	}
	h := &Hub{visible: visible, pingInterval: PingInterval, subs: map[string]*Sub{}, ring: make([]Event, 0, RingSize)}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Seq trả id sự kiện mới nhất đã gán (0 khi chưa phát gì).
func (h *Hub) Seq() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seq
}

// Publish gán id, ghi vào ring phát lại và chuyển cho mọi subscriber được thấy
// sự kiện; trả về id đã gán. Subscriber chậm (buffer đầy) bị cắt — client nối
// lại bằng Last-Event-ID và nhận phần còn thiếu từ ring.
func (h *Hub) Publish(ev Event) uint64 {
	h.mu.Lock()
	h.seq++
	ev.ID = h.seq
	if len(h.ring) < RingSize {
		h.ring = append(h.ring, ev)
	} else {
		h.ring[h.ringStart] = ev
		h.ringStart = (h.ringStart + 1) % RingSize
	}
	h.fanOutLocked(ev)
	hooks := h.onPublish
	h.mu.Unlock()
	for _, fn := range hooks {
		fn(ev)
	}
	return ev.ID
}

// Broadcast chuyển một sự kiện tức thời không vào ring, không cấp id: dùng cho
// frame tần suất cao không đáng chiếm chỗ của bộ đệm phát lại.
func (h *Hub) Broadcast(ev Event) {
	h.mu.Lock()
	h.fanOutLocked(ev)
	h.mu.Unlock()
}

// fanOutLocked gửi ev cho các subscriber được thấy; subscriber buffer đầy bị
// cắt khỏi map. Gọi với h.mu đang giữ.
func (h *Hub) fanOutLocked(ev Event) {
	for id, sub := range h.subs {
		if !h.visible(sub.p, ev) {
			continue
		}
		select {
		case sub.C <- ev:
		default:
			delete(h.subs, id)
			close(sub.C)
		}
	}
}

// Subscribe đăng ký một kết nối mới cho principal p. lastID là con trỏ
// Last-Event-ID của client: các sự kiện ring còn giữ sau con trỏ (và p được
// thấy) trả trong replay để caller ghi lại; stale=true báo con trỏ đã rơi ra
// ngoài ring (hoặc lớn hơn seq) — caller phải phát reset.
func (h *Hub) Subscribe(p any, lastID uint64) (sub *Sub, replay []Event, stale bool) {
	sub = &Sub{ID: rand.Text(), p: p, C: make(chan Event, subBuffer)}
	h.mu.Lock()
	if lastID > 0 {
		if lastID > h.seq || (len(h.ring) > 0 && lastID < h.ring[h.ringStart].ID-1) || (len(h.ring) == 0 && lastID < h.seq) {
			stale = true
		} else {
			for i := range len(h.ring) {
				ev := h.ring[(h.ringStart+i)%len(h.ring)]
				if ev.ID > lastID && h.visible(p, ev) {
					replay = append(replay, ev)
				}
			}
		}
	}
	h.subs[sub.ID] = sub
	h.mu.Unlock()
	return sub, replay, stale
}

// Unsubscribe gỡ sub (idempotent); an toàn gọi với cả sub đã bị hub cắt.
func (h *Hub) Unsubscribe(sub *Sub) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[sub.ID]; ok {
		delete(h.subs, sub.ID)
		close(sub.C)
	}
}

// WithSub chạy fn dưới khoá hub khi subscriber còn đăng ký; fn được đọc
// Principal và thay Tag. Trả false khi subscriber đã đi hoặc fn trả false.
func (h *Hub) WithSub(id string, fn func(s *Sub) bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub, ok := h.subs[id]
	if !ok {
		return false
	}
	return fn(sub)
}

// ForEachSub chạy fn cho từng subscriber đang sống dưới khoá hub; fn được đọc
// Principal và Tag (và sửa Tag khi cần quét-toàn-bộ).
func (h *Hub) ForEachSub(fn func(s *Sub)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, sub := range h.subs {
		fn(sub)
	}
}

// Update duyệt mọi subscriber: fn nhận principal hiện tại và trả principal mới
// cùng keep — keep=false cắt kết nối đó (đổi mật khẩu, thu hồi phiên, khoá tài
// khoản).
func (h *Hub) Update(fn func(p any) (np any, keep bool)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subs {
		np, keep := fn(sub.p)
		if !keep {
			delete(h.subs, id)
			close(sub.C)
			continue
		}
		sub.p = np
	}
}

// Backfill chạy một pha bù backlog nguyên khối cho một subscriber: fill chạy
// dưới khoá hub — được đọc Principal hiện hành (kể cả principal vừa được
// Update đổi) và sửa Tag — rồi trả về các sự kiện cần đẩy. Mỗi sự kiện của
// batch được kiểm lại qua visible với principal đó rồi mới enqueue; batch
// không bị Publish/Broadcast chen giữa, nên thứ tự "clear → delta" của app
// không bị phá. Trả false khi subscriber không còn hoặc fill báo ok=false
// (không đủ quyền); gửi vẫn không chặn — buffer đầy bỏ phần còn lại.
func (h *Hub) Backfill(id string, fill func(s *Sub) (batch []Event, ok bool)) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub, ok := h.subs[id]
	if !ok {
		return false
	}
	batch, ok := fill(sub)
	if !ok {
		return false
	}
	for _, ev := range batch {
		if !h.visible(sub.p, ev) {
			continue
		}
		select {
		case sub.C <- ev:
		default:
			return true // còn đăng ký nhưng buffer đầy: không cắt kết nối
		}
	}
	return true
}

// TrySend đẩy một sự kiện thẳng tới một subscriber (bù backlog ngoài ring);
// không chặn — buffer đầy hay subscriber đã đi đều trả false thay vì cắt nó.
func (h *Hub) TrySend(id string, ev Event) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub, ok := h.subs[id]
	if !ok {
		return false
	}
	select {
	case sub.C <- ev:
		return true
	default:
		return false
	}
}

// OnPublish đăng ký hook chạy sau mỗi Publish (ngoài khoá hub).
func (h *Hub) OnPublish(fn func(Event)) {
	h.mu.Lock()
	h.onPublish = append(h.onPublish, fn)
	h.mu.Unlock()
}

// Close cắt mọi kết nối (server đang tắt).
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subs {
		delete(h.subs, id)
		close(sub.C)
	}
}
