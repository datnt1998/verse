package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// writeTimeout là thời hạn ghi một frame: Wi-Fi điện thoại rớt kết nối sẽ để
// TCP nửa mở không báo lỗi; không có deadline thì Write treo mãi và subscriber
// nằm lại trong map.
const writeTimeout = 15 * time.Second

// pingComment là dòng comment SSE — client và proxy đều bỏ qua nhưng nó làm
// mới đồng hồ rảnh của kết nối.
const pingComment = ": ping\n\n"

// ErrNotFlushable trả từ Serve khi writer không hỗ trợ Flush (không thể phát
// SSE); caller chuyển thành lỗi HTTP của app.
var ErrNotFlushable = errors.New("writer không hỗ trợ luồng sự kiện")

// ResumeID đọc con trỏ nối lại: EventSource tự gửi lại header Last-Event-ID;
// client dùng fetch stream còn đặt thêm ?lastEventId= vì có proxy nuốt header
// tuỳ ý.
func ResumeID(r *http.Request) uint64 {
	if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		id, _ := strconv.ParseUint(raw, 10, 64)
		return id
	}
	if raw := r.URL.Query().Get("lastEventId"); raw != "" {
		id, _ := strconv.ParseUint(raw, 10, 64)
		return id
	}
	return 0
}

// Hooks là các điểm cắm của app quanh vòng sống một kết nối SSE; mọi field đều
// tuỳ chọn. Trả false để đóng kết nối sớm.
type Hooks struct {
	// Preamble ghi các frame đầu phiên sau reset/replay và trước vòng sự kiện
	// trực tiếp (ví dụ bù backlog của kênh con). write(ev) chuyển hoá và ghi đúng
	// định dạng khung; trả false khi ghi lỗi.
	Preamble func(write func(ev Event) bool)
	// FilterFrame returns false to skip a frame already represented by a
	// snapshot. It runs for replay, preamble and live delivery in writer order.
	FilterFrame func(ev Event) bool
	// AfterFrame chạy sau mỗi frame sự kiện trực tiếp đã ghi (vd kiểm phiên còn
	// hiệu lực sau frame session.revoked).
	AfterFrame func(ev Event) bool
	// OnTick chạy trước mỗi comment giữ kết nối (dọn tài nguyên, kiểm phiên).
	OnTick func() bool
}

// deadlineWriter gắn deadline ghi vào từng lần Write. Writer giả trong test
// không hỗ trợ ResponseController nên lỗi SetWriteDeadline được bỏ qua.
type deadlineWriter struct {
	w  io.Writer
	rc *http.ResponseController
}

func newDeadlineWriter(w http.ResponseWriter) deadlineWriter {
	return deadlineWriter{w: w, rc: http.NewResponseController(w)}
}

// NewDeadlineWriter trả writer gắn deadline ghi vào từng Write cho các luồng
// SSE riêng của app (xem deadlineWriter).
func NewDeadlineWriter(w http.ResponseWriter) io.Writer {
	return newDeadlineWriter(w)
}

func (d deadlineWriter) Write(p []byte) (int, error) {
	_ = d.rc.SetWriteDeadline(time.Now().Add(writeTimeout))
	n, err := d.w.Write(p)
	if err != nil {
		return n, err
	}
	_ = d.rc.SetWriteDeadline(time.Time{})
	return n, nil
}

// Serve phục vụ một kết nối SSE cho sub đã Subscribe: header, frame hello
// (không id, data {"subscriber": sub.ID}), reset khi con trỏ quá hạn, phát lại
// replay, preamble của app, rồi vòng sự kiện trực tiếp với comment giữ kết nối
// mỗi nhịp ping của hub (Option PingEvery). Trả khi kết nối đóng (context huỷ, ghi lỗi, hook trả false
// hay kênh sub bị đóng).
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, sub *Sub, replay []Event, stale bool, hooks Hooks) error {
	f, ok := w.(http.Flusher)
	if !ok {
		return ErrNotFlushable
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	out := newDeadlineWriter(w)

	write := func(id uint64, typ string, data any) bool {
		b, err := json.Marshal(data)
		if err != nil {
			b = []byte("{}")
		}
		if id > 0 {
			_, err = fmt.Fprintf(out, "id: %d\nevent: %s\ndata: %s\n\n", id, typ, b)
		} else {
			_, err = fmt.Fprintf(out, "event: %s\ndata: %s\n\n", typ, b)
		}
		f.Flush()
		return err == nil
	}
	writeEvent := func(ev Event) bool {
		if hooks.FilterFrame != nil && !hooks.FilterFrame(ev) {
			return true
		}
		return write(ev.ID, ev.Type, ev.Data)
	}

	if !write(0, "hello", map[string]any{"subscriber": sub.ID}) {
		return nil
	}
	if stale && !write(0, "reset", map[string]any{}) {
		return nil
	}
	for _, ev := range replay {
		if !writeEvent(ev) {
			return nil
		}
	}
	if hooks.Preamble != nil {
		hooks.Preamble(writeEvent)
	}
	ping := time.NewTicker(h.pingInterval)
	defer ping.Stop()
	for {
		select {
		case ev, ok := <-sub.C:
			if !ok || !writeEvent(ev) {
				return nil
			}
			if hooks.AfterFrame != nil && !hooks.AfterFrame(ev) {
				return nil
			}
		case <-ping.C:
			if hooks.OnTick != nil && !hooks.OnTick() {
				return nil
			}
			if _, err := io.WriteString(out, pingComment); err != nil {
				return nil
			}
			f.Flush()
		case <-r.Context().Done():
			return nil
		}
	}
}
