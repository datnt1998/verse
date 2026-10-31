package web

import (
 "encoding/json"
 "strings"
 "testing"
 "github.com/datnt1998/verse/apps/viderse/internal/activity"
)
func TestNotificationWireIncludesDetail(t *testing.T) {
 wire:=notificationToWire(activity.Notification{Detail:&activity.NotificationDetail{Cause:"Cần đăng nhập"}})
 raw,err:=json.Marshal(wire);if err!=nil{t.Fatal(err)}
 if !strings.Contains(string(raw),`"detail":{"cause":"Cần đăng nhập"}`){t.Fatalf("wire=%s",raw)}
}
