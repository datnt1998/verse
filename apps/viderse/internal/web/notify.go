package web

import (
	"sync/atomic"

	"github.com/datnt1998/verse/platform/events"
)

// Notifier trả hàm phát sự kiện SSE kiểu typ cho mọi phiên đăng nhập, data
// {"rev":N,"ref":"<id>"?}. rev tăng dần theo từng kiểu sự kiện trong tiến trình;
// ref (có thể rỗng) là id của đối tượng vừa đổi để client chỉ làm mới đúng
// khoá. Gói nghiệp vụ nhận hàm này qua constructor (kiểu func(ref string)),
// không import web hay platform/events.
func Notifier(hub *events.Hub, typ string) func(ref string) {
	var rev atomic.Uint64
	return func(ref string) {
		hub.Publish(events.Event{
			Type: typ,
			Data: statusData{Rev: rev.Add(1), Ref: ref},
			Meta: eventScope{},
		})
	}
}
