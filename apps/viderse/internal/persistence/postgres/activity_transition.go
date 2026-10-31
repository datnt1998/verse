package postgres

import (
 "context"
 "fmt"
 "time"
 "github.com/datnt1998/verse/apps/viderse/internal/activity"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
 "github.com/jackc/pgx/v5"
)

// recordStageNotification writes the notice on the same transaction that
// updates the workbench stage. A rollback therefore loses neither half.
func recordStageNotification(ctx context.Context, job domain.CloneJob, next string) error {
 tx,ok:=ctx.Value(mediaTxKey{}).(pgx.Tx);if !ok{return fmt.Errorf("chuyển trạng thái cần giao dịch")}
 if job.StageGroup == next { return nil }
 spec := activity.NotificationSpec{Audience:activity.AudienceAll, Href:"#/?video="+job.ID, ActionLabel:"Mở"}
 switch next {
 case domain.StageReview:
  var awaiting bool
  if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clone_steps WHERE job_id=$1 AND step IN ('images','build') AND status='waiting' AND error->>'code'=$2)`, job.ID, domain.CloneErrAwaitingReview).Scan(&awaiting); err != nil { return err }
  if !awaiting { return nil }
  spec.Kind,spec.Text,spec.ActionLabel="review_ready","Video chờ duyệt","Duyệt ảnh"
 case domain.StageAttention:
  spec.Kind,spec.Text="needs_attention","Cần xử lý: "+job.Source.Title
  spec.Object=&domain.ObjectRef{Type:"video",ID:job.ID,Label:job.Source.Title}
 case domain.StageReady:
  spec.Kind,spec.Text="ready_to_post","Video sẵn sàng đăng"
  spec.Object=&domain.ObjectRef{Type:"video",ID:job.ID,Label:job.Source.Title}
 default: return nil
 }
 prefs:=activity.DefaultNotificationSettings()
 if _,err:=NewSettingsStore(tx).Get(ctx,"notifications.preferences",&prefs);err!=nil{return fmt.Errorf("đọc cài đặt thông báo: %w",err)}
 window:=activity.NotifyMergeWindow;if !prefs.Merge {window=time.Duration(0)}
 _,err:=upsertScopedNotificationTx(ctx,tx,gen.New(tx),spec,window)
 return err
}
