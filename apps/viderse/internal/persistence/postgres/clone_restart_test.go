package postgres_test

import (
 "context"
 "encoding/json"
 "errors"
 "sync"
 "testing"
 "time"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

func TestCloneRestartGeneration(t *testing.T) {
 ctx := context.Background()
 store := cloneStore(t)
 old := cloneTestJob(t, store, "restart-source")
 svc := &pipeline.Service{Store: store, ResolveTemplate: func(context.Context, domain.CloneQueueItem)(string,json.RawMessage,json.RawMessage,error){ return "",json.RawMessage(`{"style":"new"}`),json.RawMessage(`{}`),nil }}
 if _, err := svc.Restart(ctx, old.ID); err == nil { t.Fatal("pending job restarted") }
 if _, err := store.ClaimJobLease(ctx, old.ID, "worker", time.Minute); err != nil { t.Fatal(err) }
 if err := store.SetJobStatus(ctx, old.ID, domain.CloneFailed); err != nil { t.Fatal(err) }
 if _, err := svc.Restart(ctx, old.ID); err == nil { t.Fatal("live lease restarted") }
 if err := store.ReleaseJobLease(ctx, old.ID, "worker"); err != nil { t.Fatal(err) }
 var wg sync.WaitGroup
 results := make(chan *pipeline.StartJobResult, 2)
 errs := make(chan error,2)
 for range 2 { wg.Add(1); go func(){ defer wg.Done(); r,e:=svc.Restart(ctx,old.ID); results<-r; errs<-e }() }
 wg.Wait(); close(results); close(errs)
 successes:=0
 var next *domain.CloneJob
 for r:=range results { if r!=nil { successes++; next=r.Job } }
 conflicts:=0
 for err:=range errs { if err!=nil { var ce *domain.CloneError; if !errors.As(err,&ce)||ce.Code!=domain.CloneErrConflict {t.Fatalf("unexpected error: %v",err)}; conflicts++ } }
 if successes!=1 || conflicts!=1 { t.Fatalf("successes %d conflicts %d",successes,conflicts) }
 if next.ID==old.ID || next.Generation!=old.Generation+1 || next.SourceKey!=old.SourceKey || next.Source.ID!=old.Source.ID {t.Fatalf("new generation: %+v",next)}
 previous,err:=store.GetJob(ctx,old.ID); if err!=nil || previous.SupersededAt==nil || previous.Status!=domain.CloneCancelled {t.Fatalf("old job: %+v %v",previous,err)}
 steps,err:=store.ListSteps(ctx,next.ID); if err!=nil || len(steps)!=len(domain.CloneSteps) {t.Fatalf("steps: %+v %v",steps,err)}
 for _,step:=range steps { if step.Status!=domain.ClonePending || step.Attempt!=0 || len(step.Checkpoint)>0 {t.Fatalf("not fresh: %+v",step)} }
 if _,err=svc.Restart(ctx,"00000000-0000-4000-8000-000000000000"); !errors.Is(err,pipeline.ErrNotFound) {t.Fatalf("unknown: %v",err)}
}
