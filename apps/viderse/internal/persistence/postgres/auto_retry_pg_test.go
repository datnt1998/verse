package postgres_test

import (
 "context"
 "encoding/json"
 "testing"
 "time"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)
func TestClaimRecordsAutomaticStepRetry(t *testing.T) {
 ctx:=context.Background();pool:=provision(t)
 _,err:=pool.Exec(ctx,`INSERT INTO owner_channels(id,name) VALUES('11111111-1111-4111-8111-111111111111','test')`);if err!=nil{t.Fatal(err)}
 store:=postgres.NewCloneStore(pool)
 start,err:=store.StartJob(ctx,postgres.StartJobInput{ChannelID:"11111111-1111-4111-8111-111111111111",SourceKey:"retry",Source:domain.CloneSourceSnapshot{ID:"retry",Title:"retry"},TemplateSnapshot:json.RawMessage(`{}`),ModelRefs:json.RawMessage(`{}`),Priority:domain.ClonePick,Origin:domain.OriginManual});if err!=nil{t.Fatal(err)}
 _,err=pool.Exec(ctx,`UPDATE clone_steps SET status='waiting',attempt=1,error=jsonb_build_object('code','quota','retryAt',$2::text) WHERE job_id=$1 AND step='download'`,start.Job.ID,time.Now().Add(-time.Minute).Format(time.RFC3339));if err!=nil{t.Fatal(err)}
 if _,err=store.ClaimJobLease(ctx,start.Job.ID,"worker",time.Minute);err!=nil{t.Fatal(err)}
 claim,err:=store.ClaimNextStep(ctx,start.Job.ID,"worker");if err!=nil || claim==nil{t.Fatalf("claim=%v err=%v",claim,err)}
 var count int
 if err=pool.QueryRow(ctx,`SELECT count(*) FROM clone_auto_actions WHERE job_id=$1 AND kind='step_retry'`,start.Job.ID).Scan(&count);err!=nil{t.Fatal(err)}
 if count!=1{t.Fatalf("retry records=%d",count)}
}
