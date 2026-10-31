package main

import (
 "context"
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "sync"

 "github.com/jackc/pgx/v5/pgxpool"
 "github.com/datnt1998/verse/apps/viderse/internal/backdrop"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/library"
 "github.com/datnt1998/verse/apps/viderse/internal/imagegen"
 "github.com/datnt1998/verse/apps/viderse/internal/media"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
 stepbackdrop "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/backdrop"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/build"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/download"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/metadata"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
 "github.com/datnt1998/verse/apps/viderse/internal/status"
 "github.com/datnt1998/verse/apps/viderse/internal/tools"
 "github.com/datnt1998/verse/apps/viderse/internal/web"
 "github.com/datnt1998/verse/apps/viderse/internal/youtube"
 "github.com/datnt1998/verse/core/llm"
 "github.com/datnt1998/verse/platform/events"
)

type cloneHandler func(context.Context,*pipeline.StepContext)(pipeline.StepResult,error)
func(f cloneHandler)Run(ctx context.Context,s *pipeline.StepContext)(pipeline.StepResult,error){return f(ctx,s)}
type cloneSettings struct{store *postgres.SettingsStore}
func(s cloneSettings)read(key string,def int)int{var n int;found,err:=s.store.Get(context.Background(),key,&n);if err!=nil||!found{return def};return n}
func(s cloneSettings)JobConcurrency()int{return s.read(domain.KeyJobConcurrency,domain.JobConcurrencyDefault)}
func(s cloneSettings)ImageConcurrency()int{return s.read(domain.KeyImageConcurrency,domain.ImageConcurrencyDefault)}
type cloneQueueBoard struct{store *postgres.QueueStore}
func(q cloneQueueBoard)BoardItems(ctx context.Context,channelID *string)([]domain.CloneQueueItem,error){items,err:=q.store.ListQueuedItems(ctx);if err!=nil{return nil,err};if channelID==nil||*channelID==""||*channelID=="all"{return items,nil};out:=make([]domain.CloneQueueItem,0);for _,item:=range items{if item.ChannelID==*channelID{out=append(out,item)}};return out,nil}

type cloneText struct{models []*llm.ClientModel}
func(g cloneText)Generate(ctx context.Context,m []llm.Message,t []llm.ToolSpec,o ...llm.CallOption)(*llm.LLMResponse,error){var last error;for _,model:=range g.models{result,err:=model.Generate(ctx,m,t,o...);if err==nil{return result,nil};last=err;if ctx.Err()!=nil{return nil,ctx.Err()}};if last==nil{last=&domain.CloneError{Code:domain.CloneErrToolMissing,Message:"Chưa chọn model Đạo diễn"}};return nil,last}

func mountClonePipeline(ctx context.Context,g *web.Gateway,pool *pgxpool.Pool,settings *postgres.SettingsStore,hub *events.Hub,statusSvc *status.Service,cfgDir string,toolKit *tools.Tools,toolDeps tools.Deps,ai *aiWiring,librarySvc *library.Service,backdropSvc *backdrop.Service,files *media.Service)(func()error,error){
 store:=postgres.NewCloneStore(pool);queueStore:=postgres.NewQueueStore(pool);rules:=cloneSettings{settings}
 publish:=func(name string,data any){hub.Publish(events.Event{Type:name,Data:data});statusSvc.Bump()}
 service:=&pipeline.Service{Store:store,Media:files,Publish:publish,Queue:cloneQueueBoard{queueStore}}
 service.ResolveTemplate=func(ctx context.Context,item domain.CloneQueueItem)(string,json.RawMessage,json.RawMessage,error){
  owner,err:=queueStore.Owner(ctx,item.ChannelID);if err!=nil{return "",nil,nil,err};if owner==nil{return "",nil,nil,pipeline.ErrNotFound}
  templates,err:=queueStore.Templates(ctx,item.ChannelID);if err!=nil{return "",nil,nil,err};if len(templates)==0{return "",nil,nil,&domain.CloneError{Code:domain.CloneErrInvalidRequest,Message:"Kênh chưa có template"}}
  cursor:=owner.TemplateCursor%len(templates);if cursor<0{cursor=0};id:=templates[cursor].ID;if item.TemplateID!=nil{id=*item.TemplateID}
  snap,err:=librarySvc.SnapshotTemplate(ctx,id);if err!=nil{return "",nil,nil,err};raw,err:=json.Marshal(snap);if err!=nil{return "",nil,nil,err};models,err:=ai.Service.Agents(ctx);if err!=nil{return "",nil,nil,err};modelJSON,err:=json.Marshal(models);return id,raw,modelJSON,err
 }
 generator:=func(s *pipeline.StepContext)(cloneText,error){var refs domain.AgentModels;if err:=json.Unmarshal(s.Job.ModelRefs,&refs);err!=nil{return cloneText{},err};chain:=append([]domain.ModelRef{refs.Director.Model},refs.Director.Fallback...);if s.Step.Step==domain.CloneImages{chain=[]domain.ModelRef{refs.Artist.PromptModel}};return cloneText{ai.Executors.textModels(ctx,chain)},nil}
 ff:=func(ctx context.Context)(*tools.FfmpegResolved,error){r:=toolKit.Ffmpeg.Resolve(ctx);if r==nil{return nil,&domain.CloneError{Code:domain.CloneErrToolMissing,Message:"Thiếu FFmpeg"}};return r,nil}
 tr:=tools.NewTranscription(toolDeps,toolKit.Uv,nil)
 registry:=pipeline.Registry{
 domain.CloneDownload:download.New(&download.Deps{Tools:toolDeps,ResolveFfmpeg:toolKit.Ffmpeg.Resolve}),
 domain.CloneBackdrop:stepbackdrop.New(stepbackdrop.ProductionDeps(postgres.NewBackdropStore(pool),backdropSvc)),
 domain.CloneTranscript:transcript.New(&transcript.Deps{ResolveFfmpeg:toolKit.Ffmpeg.Resolve,ResolveUv:toolKit.Uv.Resolve,TranscriptionStatus:tr.Status,RunEnv:tr.RunEnv()}),
 domain.CloneMetadata:cloneHandler(func(ctx context.Context,s *pipeline.StepContext)(pipeline.StepResult,error){model,err:=generator(s);if err!=nil{return pipeline.StepResult{},err};return (metadata.Handler{Deps:metadata.Deps{Model:model,OwnerChannel:func(ctx context.Context,job domain.CloneJob)(*metadata.ChannelView,error){owner,err:=queueStore.Owner(ctx,job.ChannelID);if err!=nil||owner==nil{return nil,err};note:="";if owner.StyleNote!=nil{note=*owner.StyleNote};return &metadata.ChannelView{ID:owner.ID,Name:owner.Name,StyleNote:note},nil}}}).Run(ctx,s)}),
 domain.CloneScenes:cloneHandler(func(ctx context.Context,s *pipeline.StepContext)(pipeline.StepResult,error){resolved,err:=ff(ctx);if err!=nil{return pipeline.StepResult{},err};return (scenes.Handler{Deps:scenes.Deps{Ffmpeg:resolved.Ffmpeg,FFprobe:resolved.Ffprobe}}).Run(ctx,s)}),
 domain.CloneImages:cloneHandler(func(ctx context.Context,s *pipeline.StepContext)(pipeline.StepResult,error){
  resolved,err:=ff(ctx);if err!=nil{return pipeline.StepResult{},err};model,err:=generator(s);if err!=nil{return pipeline.StepResult{},err}
  var refs domain.AgentModels;if err:=json.Unmarshal(s.Job.ModelRefs,&refs);err!=nil{return pipeline.StepResult{},err};chain:=[]domain.ModelRef{refs.Artist.Draw};if refs.Artist.FallbackOn{chain=append(chain,refs.Artist.DrawFallback...)}
  providers,_:=ai.Executors.imageProviders(ctx,chain);limit:=ai.Executors.limit;fallback:=refs.Artist.FallbackOn
  drawer:=imagegen.NewService(imagegen.Options{Providers:providers,Fallback:&fallback,Journal:ai.Executors.journal,DailyPaidImageLimit:&limit})
  return (images.Handler{Deps:images.Deps{Drawer:drawer,Model:model,Ffmpeg:resolved.Ffmpeg,FFprobe:resolved.Ffprobe,Look:func(ctx context.Context,job domain.CloneJob)(string,error){owner,err:=queueStore.Owner(ctx,job.ChannelID);if err!=nil||owner==nil{return "",err};return owner.Branding.Look,nil}}}).Run(ctx,s)
 }),
 domain.CloneBuild:cloneHandler(func(ctx context.Context,s *pipeline.StepContext)(pipeline.StepResult,error){resolved,err:=ff(ctx);if err!=nil{return pipeline.StepResult{},err};return (build.Handler{Deps:build.Deps{Ffmpeg:resolved.Ffmpeg,FFprobe:resolved.Ffprobe}}).Run(ctx,s)}),
 }
 runner:=&pipeline.Runner{Store:store,Handlers:registry,Media:service.Media,Runner:toolDeps.Runner,Publish:publish,StagingRoot:filepath.Join(cfgDir,"clone-staging"),Settings:rules,ImageLimiter:&pipeline.ImageLimiter{Limit:rules.ImageConcurrency}}
 worker:=&pipeline.Worker{Store:store,Runner:runner,Settings:rules,ID:fmt.Sprintf("%d",os.Getpid())};service.Worker=worker
 q:=queue.New(queue.Options{Pool:pool,Runner:service,Settings:settings,Probe:queue.YouTubeProbe(youtube.NewClient(youtube.Options{})),Notify:func(name string){publish(name,map[string]any{})}})
 runner.Queue=q;worker.OnPrimary=q.Start;worker.OnStopped=q.Stop
 web.MountQueueDiag(g,q,func(err error){fmt.Fprintln(os.Stderr,"Hàng chờ:",err)})
 web.MountClone(g,service,q,service.Media)
 workerCtx,stop:=context.WithCancel(ctx);done:=make(chan error,1);go func(){done<-worker.Run(workerCtx)}()
 var once sync.Once;var stopped error
 return func()error{once.Do(func(){stop();stopped=<-done});return stopped},nil
}
