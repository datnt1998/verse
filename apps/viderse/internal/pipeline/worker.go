package pipeline

import (
 "context"
 "errors"
 "sync"
 "time"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// WorkerLock pins the advisory-lock connection for the worker's lifetime.
type WorkerLock interface { Check(context.Context) error; Close(context.Context) error }
type WorkerStore interface {
 Store
 AcquireWorkerLock(context.Context) (WorkerLock,error)
 ListClaimableJobs(context.Context,int)([]domain.CloneJob,error)
 ListInterruptedJobs(context.Context,int)([]domain.CloneJob,error)
 ResumeJob(context.Context,string)(*domain.CloneJob,error)
}

// Worker admits at most jobConcurrency jobs; paused/review jobs release a lane.
// A process without the PostgreSQL advisory lock remains read-only.
type Worker struct {
 Store WorkerStore
 Runner *Runner
 Settings Settings
 ID string
 OnPrimary func(context.Context) error
 OnStopped func() error
 mu sync.Mutex
 running map[string]bool
 wake chan struct{}
 wg sync.WaitGroup
 primary bool
}

func (w *Worker) Wake() { w.mu.Lock(); ch:=w.wake; w.mu.Unlock(); if ch!=nil {select {case ch<-struct{}{}:default:}} }
func (w *Worker) IsPrimary() bool { w.mu.Lock(); defer w.mu.Unlock(); return w.primary }
func (w *Worker) Run(ctx context.Context) error {
 lock,err:=w.Store.AcquireWorkerLock(ctx)
 if err!=nil {return err}
 if lock==nil {<-ctx.Done(); return nil}
 defer lock.Close(context.Background())
 ctx,cancel:=context.WithCancel(ctx); defer cancel()
 w.mu.Lock(); w.primary=true; w.running=map[string]bool{}; w.wake=make(chan struct{},1); w.mu.Unlock()
 defer func(){w.mu.Lock();w.primary=false;w.mu.Unlock()}()
 // Recovery is automatic after the old process lease expires; checkpoints stay.
 recoverJobs:=func() error {
  jobs,err:=w.Store.ListInterruptedJobs(ctx,1000); if err!=nil{return err}
  for _,job:=range jobs {
   if err:=w.Runner.ReconcileInterrupted(ctx,job.ID);err!=nil{return err}
   if _,err:=w.Store.ResumeJob(ctx,job.ID);err!=nil{return err}
  };return nil
 }
 if err:=recoverJobs();err!=nil{return err}
 if w.OnPrimary!=nil {if err:=w.OnPrimary(ctx);err!=nil{return err}}
 defer func(){if w.OnStopped!=nil {_=w.OnStopped()}}()
 tick:=time.NewTicker(500*time.Millisecond); defer tick.Stop()
 defer func(){cancel();w.wg.Wait()}()
 for {
  if ctx.Err()!=nil{return nil}
  if err:=lock.Check(ctx);err!=nil{return err}
  if err:=recoverJobs();err!=nil{return err}
  limit:=domain.JobConcurrencyDefault; if w.Settings!=nil {limit=w.Settings.JobConcurrency()}
  if limit<domain.JobConcurrencyMin {limit=domain.JobConcurrencyMin};if limit>domain.JobConcurrencyMax {limit=domain.JobConcurrencyMax}
  w.mu.Lock();free:=limit-len(w.running);w.mu.Unlock()
  if free>0 {
   jobs,err:=w.Store.ListClaimableJobs(ctx,limit*2);if err!=nil{return err}
   for _,job:=range jobs {
    w.mu.Lock();if w.running[job.ID]||len(w.running)>=limit {w.mu.Unlock();continue};w.running[job.ID]=true;w.mu.Unlock()
    w.wg.Add(1)
    go func(id string){defer w.wg.Done();defer func(){w.mu.Lock();delete(w.running,id);w.mu.Unlock();w.Wake()}();_,err:=w.Runner.RunJob(ctx,id,w.ID);if err!=nil&&!errors.Is(err,context.Canceled){w.Runner.publish(domain.CloneEventUpdated,map[string]any{"jobId":id})}}(job.ID)
   }
  }
  select {case <-ctx.Done():return nil;case <-tick.C:case <-w.wake:}
 }
}

// ImageLimiter is resized by reading current settings on every acquisition;
// settings changes never cancel a paid draw already in flight.
type ImageLimiter struct {mu sync.Mutex;used int;changed chan struct{};Limit func()int}
func (l *ImageLimiter) WithSlot(ctx context.Context,run func(context.Context)error)error {
 for {l.mu.Lock();if l.changed==nil {l.changed=make(chan struct{})};limit:=domain.ImageConcurrencyDefault;if l.Limit!=nil{limit=l.Limit()};if limit<1{limit=1};if limit>domain.ImageConcurrencyMax{limit=domain.ImageConcurrencyMax}
 if l.used<limit {l.used++;l.mu.Unlock();defer func(){l.mu.Lock();l.used--;close(l.changed);l.changed=make(chan struct{});l.mu.Unlock()}();return run(ctx)}
 ch:=l.changed;l.mu.Unlock();select{case <-ctx.Done():return ctx.Err();case <-ch:case <-time.After(250*time.Millisecond):}
 }
}
