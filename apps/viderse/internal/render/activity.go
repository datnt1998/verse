package render

import (
 "context"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func (s *Supervisor) beginActivity(ctx context.Context, objectType, objectID, title string) func(error) {
 if s.Tasks==nil { return func(error){} }
 id,err:=s.Tasks.Start(ctx,domain.TaskSpec{Kind:domain.TaskEditorExport,Title:title,Object:domain.ObjectRef{Type:objectType,ID:objectID,Label:title},Phase:"Đang thực hiện"})
 if err!=nil { return func(error){} }
 return func(cause error) {
  finishCtx:=context.WithoutCancel(ctx)
  if cause==nil { _=s.Tasks.Finish(finishCtx,id); return }
  phase:="Không hoàn tất, hãy thử lại"
  action:=domain.TaskAction{Label:"Thử lại",Op:domain.TaskOpRetry}
  _=s.Tasks.Update(finishCtx,id,domain.TaskUpdate{State:domain.TaskFailed,Phase:&phase,Action:&action})
 }
}
