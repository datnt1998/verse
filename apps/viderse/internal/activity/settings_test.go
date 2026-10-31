package activity

import (
 "testing"
 "time"
)

func TestNotificationDeliveryQuietHours(t *testing.T) {
 s := DefaultNotificationSettings()
 s.DoNotDisturb = QuietHours{Enabled:true, Start:"22:00", End:"07:00"}
 for _, tc := range []struct{ hour int; want bool }{{23,false},{6,false},{12,true}} {
  at := time.Date(2026, 1, 1, tc.hour, 0, 0, 0, time.UTC)
  if got := s.SystemAllowed("review_ready", at); got != tc.want { t.Fatalf("hour %d: got %v want %v", tc.hour, got, tc.want) }
 }
 s.Kinds["review_ready"] = NotificationChannels{InApp:true, System:false}
 if s.SystemAllowed("review_ready", time.Date(2026,1,1,12,0,0,0,time.UTC)) { t.Fatal("disabled system delivery") }
}
