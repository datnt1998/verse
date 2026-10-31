package postgres_test

import (
 "context"
 "encoding/json"
 "fmt"
 "testing"
 "github.com/datnt1998/verse/apps/viderse/internal/activity"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)
func TestStageNotificationCommitsWithTransition(t *testing.T) {
 ctx:=context.Background();pool:=provision(t);store:=postgres.NewCloneStore(pool)
 _,err:=pool.Exec(ctx,`INSERT INTO owner_channels(id,name) VALUES('11111111-1111-4111-8111-111111111111','test')`);if err!=nil{t.Fatal(err)}
 makeJob:=func(i int) string {
  input:=postgres.StartJobInput{ChannelID:"11111111-1111-4111-8111-111111111111",SourceKey:fmt.Sprint(i),Source:domain.CloneSourceSnapshot{ID:fmt.Sprint(i),Title:fmt.Sprintf("Video %d",i)},TemplateSnapshot:json.RawMessage(`{}`),ModelRefs:json.RawMessage(`{}`),Priority:domain.ClonePick,Origin:domain.OriginManual}
  result,err:=store.StartJob(ctx,input);if err!=nil{t.Fatal(err)}
  _,err=pool.Exec(ctx,`UPDATE clone_steps SET status='running',attempt=1,fence=1 WHERE job_id=$1 AND step='images'`,result.Job.ID);if err!=nil{t.Fatal(err)}
  return result.Job.ID
 }
 for i:=range 3 { id:=makeJob(i);if err=store.ApplyStepWaiting(ctx,id,domain.CloneImages,1,1,nil,&domain.CloneError{Code:domain.CloneErrAwaitingReview});err!=nil{t.Fatal(err)} }
 // A newly constructed service models process restart after commit, with no callback.
 history,err:=activity.New(postgres.NewActivityStore(pool),nil).Notifications(ctx);if err!=nil{t.Fatal(err)}
 if len(history)!=1 || history[0].Count!=3 || history[0].Kind!="review_ready" {t.Fatalf("history=%+v",history)}
 bad:=makeJob(9)
 if err=store.ApplyStepWaiting(ctx,bad,domain.CloneImages,2,1,nil,&domain.CloneError{Code:domain.CloneErrAwaitingReview});err==nil{t.Fatal("stale fence should rollback")}
 step,err:=store.GetStep(ctx,bad,domain.CloneImages);if err!=nil{t.Fatal(err)};if step.Status!=domain.CloneRunning {t.Fatalf("transition not rolled back: %s",step.Status)}
 history,err=activity.New(postgres.NewActivityStore(pool),nil).Notifications(ctx);if err!=nil{t.Fatal(err)};if len(history)!=1 || history[0].Count!=3 {t.Fatalf("rollback changed history: %+v",history)}
}

func TestRedrawMarkerDoesNotNotifyReviewReady(t *testing.T) {
 ctx:=context.Background();pool:=provision(t);store:=postgres.NewCloneStore(pool)
 var job string
 err:=pool.QueryRow(ctx,`WITH c AS (INSERT INTO owner_channels(id,name) VALUES(gen_random_uuid(),'test') RETURNING id) INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,status,stage_group) SELECT gen_random_uuid(),id,'source','{}'::jsonb,'running','working' FROM c RETURNING id::text`).Scan(&job);if err!=nil{t.Fatal(err)}
 _,err=pool.Exec(ctx,`INSERT INTO clone_steps(job_id,step,status,checkpoint) VALUES($1,'images','running','{"reviewOrigin":true,"items":[{"id":"one","status":"running"}]}'::jsonb)`,job);if err!=nil{t.Fatal(err)}
 if err=store.SetJobStatus(ctx,job,domain.CloneRunning);err!=nil{t.Fatal(err)}
 notes,err:=activity.New(postgres.NewActivityStore(pool),nil).Notifications(ctx);if err!=nil{t.Fatal(err)}
 if len(notes)!=0 {t.Fatalf("stage progress notified: %+v",notes)}
}
