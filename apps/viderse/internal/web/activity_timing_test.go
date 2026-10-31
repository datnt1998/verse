package web

import (
 "testing"
 "time"
 "github.com/datnt1998/verse/apps/viderse/internal/activity"
)
func TestTaskWireIncludesCreationTime(t *testing.T) {
 at:=time.Date(2026,1,2,3,4,5,0,time.UTC)
 got:=taskToWire(activity.Task{CreatedAt:at})
 if got.CreatedAt==nil || !got.CreatedAt.Equal(at) {t.Fatalf("createdAt=%v",got.CreatedAt)}
 if taskToWire(activity.Task{}).CreatedAt!=nil {t.Fatal("zero time should be absent")}
}
