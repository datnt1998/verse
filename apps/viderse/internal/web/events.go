package web

import (
	"time"

	"github.com/datnt1998/verse/platform/events"
	"github.com/datnt1998/verse/platform/identity"
)

// eventScope là ngữ cảnh lọc của một sự kiện Viderse, đặt trong Meta của
// events.Event. User rỗng nghĩa là mọi phiên đăng nhập đều thấy; User khác rỗng
// chỉ phiên của tài khoản đó thấy (thông báo riêng của từng tài khoản).
type eventScope struct {
	User string
}

// EventTypeStatus là tên sự kiện phát khi trạng thái đầu trang đổi revision.
// Client vô hiệu cache status khi nhận nó; ref rỗng cho sự kiện toàn cục này.
const EventTypeStatus = "status"

// statusData là hình data của frame status: {"rev":N,"ref":"<id>"?}.
type statusData struct {
	Rev uint64 `json:"rev"`
	Ref string `json:"ref,omitempty"`
}

// pingInterval là nhịp giữ kết nối của app: web coi luồng là mất khi không có
// byte nào quá ~3.5 s, nên server phải đảm bảo ping dưới ngưỡng đó — 2 s để dư
// đường truyền. Noverse giữ nhịp 15 s mặc định của hub.
const pingInterval = 2 * time.Second

// NewEventHub dựng hub SSE của app: principal là identity.User của phiên, sự
// kiện lọc theo eventScope (mọi phiên hoặc theo tài khoản).
func NewEventHub() *events.Hub {
	return events.New(func(p any, pe events.Event) bool {
		u, _ := p.(identity.User)
		if u.Disabled {
			return false
		}
		scope, _ := pe.Meta.(eventScope)
		return scope.User == "" || scope.User == u.Username
	}, events.PingEvery(pingInterval))
}

// PublishStatus trả con trỏ publish của status.Service: mỗi Bump phát một
// frame "status" mang rev mới cho mọi phiên đăng nhập.
func PublishStatus(hub *events.Hub) func(rev uint64) {
	return func(rev uint64) {
		hub.Publish(events.Event{
			Type: EventTypeStatus,
			Data: statusData{Rev: rev},
			Meta: eventScope{},
		})
	}
}
