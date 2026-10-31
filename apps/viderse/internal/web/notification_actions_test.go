package web

import (
	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"testing"
)

func TestNotificationWireRecoveryActions(t *testing.T) {
	n := activity.Notification{Kind: "needs_attention", Href: "#/?video=1", ActionLabel: "Mở", Detail: &activity.NotificationDetail{Recovery: &activity.NotificationAction{Label: "Đăng nhập lại", Href: "#/settings?section=connect"}}}
	wire := notificationToWire(n)
	if len(wire.Actions) != 2 || wire.Actions[0].Label != "Đăng nhập lại" || wire.Actions[1].Label != "Mở video" || wire.Actions[1].Href != "#/?video=1" {
		t.Fatalf("actions=%+v", wire.Actions)
	}
}
