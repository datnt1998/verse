package pipeline

import (
 "encoding/json"
 "os"
 "reflect"
 "testing"
 "time"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestCloneGoldenReadModels(t *testing.T){
 data,err:=os.ReadFile("testdata/clone-views.golden.json");if err!=nil{t.Fatal(err)}
 var fixtures []domain.CloneJobView;if err:=json.Unmarshal(data,&fixtures);err!=nil{t.Fatal(err)}
 now:=time.Date(2026,10,31,12,0,0,0,time.UTC)
 for _,fixture:=range fixtures{t.Run(fixture.ID,func(t *testing.T){
  if got:=domain.StageGroupOf(fixture.CloneJob,fixture.Steps);got!=fixture.StageGroup{t.Fatalf("group %q want %q",got,fixture.StageGroup)}
  if got:=domain.Subline(fixture.CloneJob,fixture.Steps,now);got!=fixture.Subline{t.Fatalf("subline %q want %q",got,fixture.Subline)}
  if got:=domain.AgentViews(fixture.CloneJob,fixture.Steps);!reflect.DeepEqual(got,fixture.AgentView){t.Fatalf("agents %+v want %+v",got,fixture.AgentView)}
 })}
}

func TestProviderResetDoesNotBecomeWorkerBackoff(t *testing.T){
 now:=time.Now();at:=now.Add(time.Minute)
 step:=domain.CloneStepRecord{Step:domain.CloneDownload,Attempt:1,Error:&domain.CloneError{Code:domain.CloneErrQuota,RetryAt:&at,RetryAtSource:domain.RetryAtProvider}}
 if got:=domain.AutoRetryAt(step,now);got!=nil{t.Fatalf("provider reset auto-retried: %v",got)}
 step.Error.RetryAtSource=domain.RetryAtWorker
 if got:=domain.AutoRetryAt(step,now);got==nil||!got.Equal(at){t.Fatalf("worker backoff lost: %v",got)}
 step.Step=domain.CloneImages;if got:=domain.AutoRetryAt(step,now);got!=nil{t.Fatal("image quota holds lane")}
}
func TestImageQuotaIsWorkingNotAttention(t *testing.T){
 job:=domain.CloneJob{Status:domain.CloneWaiting};steps:=[]domain.CloneStepRecord{{Step:domain.CloneImages,Status:domain.CloneWaiting,Error:&domain.CloneError{Code:domain.CloneErrQuota}}}
 if got:=domain.StageGroupOf(job,steps);got!=domain.StageWorking{t.Fatalf("quota group %q",got)}
}
func TestChainOrderAndMissingStep(t *testing.T){
 rows:=[]domain.CloneStepRecord{{Step:domain.CloneImages,Status:domain.ClonePending},{Step:domain.CloneDownload,Status:domain.CloneSucceeded}}
 if got:=domain.NextOpenStep(rows);got==nil||got.Step!=domain.CloneImages{t.Fatalf("next %+v",got)}
 if domain.ChainDone(rows){t.Fatal("incomplete chain marked done")}
}
