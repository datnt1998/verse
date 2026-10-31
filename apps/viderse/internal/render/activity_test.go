package render

import (
 "context"
 "testing"
 "time"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
)
type activitySpy struct { started,failed,finished int }
func (s *activitySpy) Start(_ context.Context, spec domain.TaskSpec)(string,error){if spec.Kind!=domain.TaskEditorExport {panic("wrong task kind")};s.started++;return "task",nil}
func (s *activitySpy) Update(_ context.Context,_ string,u domain.TaskUpdate)error{if u.State==domain.TaskFailed{s.failed++};return nil}
func (s *activitySpy) Finish(context.Context,string)error{s.finished++;return nil}
func TestExportReportsFailedBackgroundTask(t *testing.T){
 supervisor,_,_:=fakeSupervisor(t,"exit");tasks:=&activitySpy{};supervisor.Tasks=tasks
 ctx,cancel:=context.WithTimeout(context.Background(),time.Second);defer cancel()
 _,err:=supervisor.LaunchExport(ctx,"project","",nil);if err==nil{t.Fatal("expected failure")}
 if tasks.started!=1 || tasks.failed!=1 || tasks.finished!=0{t.Fatalf("task=%+v",tasks)}
}
