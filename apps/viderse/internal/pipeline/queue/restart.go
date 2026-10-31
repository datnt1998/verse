package queue

import (
 "context"
 "errors"
 "time"

 "github.com/jackc/pgx/v5"
 "encoding/json"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// RestartPlan uses the same template rotation and queue simulation as admission.
func (s *Service) RestartPlan(ctx context.Context, job *domain.CloneJob) (*domain.CloneRestartPlan, error) {
 var status string
 var superseded,lease *time.Time
 var queued bool
 err:=s.d.Pool.QueryRow(ctx,`SELECT status,superseded_at,lease_expires_at,EXISTS(SELECT 1 FROM clone_queue q WHERE q.source_key=clone_jobs.source_key AND q.state='queued' AND q.removed_at IS NULL) FROM clone_jobs WHERE id=$1`,job.ID).Scan(&status,&superseded,&lease,&queued)
 if errors.Is(err,pgx.ErrNoRows){return nil,notFound("Video không còn tồn tại")};if err!=nil{return nil,err}
 if queued || superseded!=nil || status=="pending" || status=="running" || (lease!=nil && lease.After(s.d.Now())) {return nil,&domain.CloneError{Code:domain.CloneErrConflict,Message:"Video vẫn đang làm hoặc đã có bản làm lại"}}
 owner, err := s.store.Owner(ctx,job.ChannelID); if err!=nil{return nil,err}; if owner==nil{return nil,notFound("Kênh không tồn tại")}
 templates, err := s.store.Templates(ctx,job.ChannelID); if err!=nil{return nil,err}; if len(templates)==0{return nil,invalid("Kênh chưa có template")}
 cursor:=owner.TemplateCursor%len(templates); if cursor<0 {cursor=0}; tpl:=templates[cursor]
 raw,err:=s.store.TemplateRecipe(ctx,tpl.ID); if err!=nil{return nil,err}
 var recipe domain.Recipe
 var imageEstimate *int
 if json.Unmarshal(raw,&recipe)==nil { imageEstimate,_=estimateImages(&recipe,job.Source.DurationSec,job.NoImages) }
 lanes,_,_,_,err:=s.loadLanes(ctx); if err!=nil{return nil,err}
 index:=-1; position:=0.0
 for i:=range lanes { if lanes[i].ChannelID==job.ChannelID {index=i}; for _,item:=range lanes[i].Items {if item.Priority=="pick" && item.Position<=position {position=item.Position-1}} }
 if index<0 {lanes=append(lanes,Lane{ChannelID:job.ChannelID,Pace:owner.PacePerDay,TemplateOK:true});index=len(lanes)-1}
 seconds:=int64(0); if job.Source.DurationSec!=nil {seconds=int64(*job.Source.DurationSec)}
 const id="__restart_preview__"
 lanes[index].Items=append(lanes[index].Items,Item{ID:id,ChannelID:job.ChannelID,Priority:"pick",Position:position,DurationSec:seconds,CreatedAt:s.d.Now()})
 rules,err:=s.Rules(ctx);if err!=nil{return nil,err}
 basis,err:=s.basis(ctx);if err!=nil{return nil,err}
 active,err:=s.store.ListActiveJobs(ctx);if err!=nil{return nil,err}
 eta:=Estimate(lanes,EstimateInput{Now:s.d.Now(),Workers:rules.Workers,Basis:basis,Active:activeETA(active),Paused:rules.Paused})[id]
 return &domain.CloneRestartPlan{Template:domain.ClonePlanTemplate{ID:tpl.ID,Name:tpl.Name,FromRotation:true},ImageEstimate:imageEstimate,Position:"front",ETAStart:eta.Start},nil
}


// EnqueueRestart reserves a regular pick at the head, leaving the old job
// readable until the scheduler atomically supersedes it at admission.
func (s *Service) EnqueueRestart(ctx context.Context, job *domain.CloneJob) (*domain.CloneQueueItem, error) {
	tx,err:=s.d.Pool.Begin(ctx); if err!=nil{return nil,err}; defer tx.Rollback(ctx)
	repo:=s.store.With(tx)
	if err=repo.LockOwnerChannel(ctx,job.ChannelID);err!=nil{return nil,err}
	var status string
	var superseded, lease *time.Time
	err=tx.QueryRow(ctx,`SELECT status,superseded_at,lease_expires_at FROM clone_jobs WHERE id=$1 FOR UPDATE`,job.ID).Scan(&status,&superseded,&lease)
	if errors.Is(err,pgx.ErrNoRows){return nil,notFound("Video không còn tồn tại")};if err!=nil{return nil,err}
	if superseded!=nil || status=="pending" || status=="running" || (lease!=nil && lease.After(s.d.Now())) {return nil,&domain.CloneError{Code:domain.CloneErrConflict,Message:"Video vẫn đang làm hoặc đã có bản làm lại"}}
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(27900503)`);err!=nil{return nil,err}
	owner,err:=repo.Owner(ctx,job.ChannelID);if err!=nil{return nil,err};if owner==nil{return nil,notFound("Kênh không tồn tại")}
	templates,err:=repo.Templates(ctx,job.ChannelID);if err!=nil{return nil,err};if len(templates)==0{return nil,invalid("Kênh chưa có template")}
	cursor:=owner.TemplateCursor%len(templates);if cursor<0{cursor=0};templateID:=templates[cursor].ID
	queued,err:=repo.ListQueuedItems(ctx);if err!=nil{return nil,err}
	position:=float64(s.d.Now().UnixNano())/1e9
	for _,item:=range queued {if item.SourceKey==job.SourceKey{return nil,&domain.CloneError{Code:domain.CloneErrConflict,Message:"Video đã có bản làm lại trong hàng chờ"}};if item.Priority==domain.ClonePick && item.Position<=position {position=item.Position-1}}
	raw,err:=json.Marshal(job.Source);if err!=nil{return nil,err}
	item:=domain.CloneQueueItem{ID:newID(),ChannelID:job.ChannelID,SourceVideoID:job.Source.ID,SourceKey:job.SourceKey,Source:job.Source,Priority:domain.ClonePick,Origin:domain.OriginManual,Position:position,State:domain.QueueQueued,NoImages:job.NoImages,TemplateID:&templateID,RestartOf:&job.ID,CreatedAt:s.d.Now(),OverrideRevision:1}
	_,err=tx.Exec(ctx,`INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,source_snapshot,priority,origin,position,no_images,template_id,created_at,restart_of) VALUES($1,$2,$3,$4,$5,'pick','manual',$6,$7,$8,$9,$10)`,item.ID,item.ChannelID,item.SourceVideoID,item.SourceKey,raw,item.Position,item.NoImages,templateID,item.CreatedAt,job.ID)
	if err!=nil{return nil,err}
	if err=tx.Commit(ctx);err!=nil{return nil,err}
	s.emit();s.RequestDrain()
	return &item,nil
}