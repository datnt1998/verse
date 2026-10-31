package postgres_test

import (
 "context"
 "encoding/json"
 "errors"
 "testing"
 "time"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

func TestCloneRestartGeneration(t *testing.T) {
 ctx:=context.Background(); store:=cloneStore(t); old:=cloneTestJob(t,store,"restart-source")
 in:=postgres.StartJobInput{ChannelID:old.ChannelID,SourceKey:old.SourceKey,Source:old.Source,TemplateSnapshot:json.RawMessage(`{}`),ModelRefs:json.RawMessage(`{}`),Replaces:old.ID}
 if _,err:=store.StartJob(ctx,in);err==nil {t.Fatal("pending job restarted")}
 if _,err:=store.ClaimJobLease(ctx,old.ID,"worker",time.Minute);err!=nil{t.Fatal(err)}
 if err:=store.SetJobStatus(ctx,old.ID,domain.CloneFailed);err!=nil{t.Fatal(err)}
 if _,err:=store.StartJob(ctx,in);err==nil{t.Fatal("live lease restarted")}
 if err:=store.ReleaseJobLease(ctx,old.ID,"worker");err!=nil{t.Fatal(err)}
 result,err:=store.StartJob(ctx,in);if err!=nil{t.Fatal(err)}
 if result.Job.ID==old.ID || result.Job.Generation!=old.Generation+1 || result.Job.SourceKey!=old.SourceKey {t.Fatalf("generation: %+v",result.Job)}
 previous,err:=store.GetJob(ctx,old.ID);if err!=nil || previous.SupersededAt==nil || previous.Status!=domain.CloneCancelled {t.Fatalf("old job: %+v %v",previous,err)}
 svc:=&pipeline.Service{Store:store};view,err:=svc.Get(ctx,result.Job.ID);if err!=nil || view.PreviousJobID==nil || *view.PreviousJobID!=old.ID {t.Fatalf("previous link: %+v %v",view,err)}
 if _,err=store.StartJob(ctx,in);err==nil {t.Fatal("superseded generation restarted twice")}
 if _,err=store.GetJob(ctx,"00000000-0000-4000-8000-000000000000");!errors.Is(err,pipeline.ErrNotFound){t.Fatalf("missing: %v",err)}
}
