package logs

import (
 "context"
 "testing"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
)
type taskSpy struct { started,finished int }
func (s *taskSpy) Start(_ context.Context, spec domain.TaskSpec)(string,error){if spec.Kind!=domain.TaskLogs { panic("wrong kind") };s.started++;return "id",nil}
func (s *taskSpy) Finish(context.Context,string)error{s.finished++;return nil}
func TestBundleReportsBackgroundTask(t *testing.T) {
 s:=New(t.TempDir(),""); tasks:=&taskSpy{};s.SetTasks(tasks)
 if _,err:=s.BuildBundle(context.Background(),BundleOptions{});err!=nil{t.Fatal(err)}
 if tasks.started!=1 || tasks.finished!=1 {t.Fatalf("start=%d finish=%d",tasks.started,tasks.finished)}
}
