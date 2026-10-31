package images

import (
 "context"
 "encoding/json"
 "strings"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/library"
 model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
)

func(s *Service) PlanOverrides(ctx context.Context,id string,input OverrideInput)(OverridePlan,error){
 l,err:=s.load(ctx,id,nil,false);if err!=nil{return OverridePlan{},err}
 _,plan,err:=s.prepareOverrides(ctx,l,input);return plan,err
}
func(s *Service) ApplyOverrides(ctx context.Context,id string,input OverrideInput)(OverridePlan,error){
 l,err:=s.load(ctx,id,input.ExpectedRevision,true);if err!=nil{return OverridePlan{},err};if err=s.mutable(l);err!=nil{return OverridePlan{},err}
 next,plan,err:=s.prepareOverrides(ctx,l,input);if err!=nil{return OverridePlan{},err}
 from:=domain.CloneStep("");rerun:=len(next.QueueOnly)>0
 if rerun{from=domain.CloneImages}else if plan.Rebuild{from=domain.CloneBuild}
 if err=s.Edits.Commit(ctx,l.snapshot,next,from,rerun,nil);err!=nil{return OverridePlan{},err};return plan,nil
}
func(s *Service) resolvePrompt(ctx context.Context,value *string,kind domain.PromptKind)(*domain.ImagePrompt,error){
 if value==nil{return nil,nil};id:=strings.TrimSpace(*value);if id==""{return nil,cloneError(domain.CloneErrInvalidRequest,"Prompt ảnh không được để trống")}
 if s.Prompts==nil{return nil,cloneError(domain.CloneErrInternal,"Thiếu thư viện prompt ảnh")}
 p,err:=s.Prompts.GetPrompt(ctx,id);if err!=nil{if library.CodeOf(err)==library.CodeNotFound{return nil,cloneError(domain.CloneErrNotFound,"Không tìm thấy prompt ảnh đã chọn")};return nil,cloneError(domain.CloneErrInternal,"Không đọc được thư viện prompt ảnh")};if p==nil||p.Kind!=kind{return nil,cloneError(domain.CloneErrInvalidRequest,"Loại prompt không khớp ảnh cảnh hoặc ảnh bìa")};return p,nil
}
func(s *Service) prepareOverrides(ctx context.Context,l loaded,input OverrideInput)(model.Manifest,OverridePlan,error){
 old:=l.manifest
 next,err:=model.CloneManifest(old);if err!=nil{return model.Manifest{},OverridePlan{},err}
 var template library.Snapshot
 if len(l.snapshot.Job.TemplateSnapshot)>0 {if err=json.Unmarshal(l.snapshot.Job.TemplateSnapshot,&template);err!=nil{return model.Manifest{},OverridePlan{},cloneError(domain.CloneErrInternal,"Template snapshot không đọc được")}}
 if next.Overrides==nil{next.Overrides=&model.Overrides{}}
 o:=next.Overrides
 if input.ScenePromptID.Set{
  p,err:=s.resolvePrompt(ctx,input.ScenePromptID.Value,domain.PromptScene);if err!=nil{return model.Manifest{},OverridePlan{},err}
  o.ScenePromptSet=p!=nil;o.ScenePromptID=""
  if p==nil{next.TemplatePrompt=template.ImagePrompt;next.ImagePromptMode=template.ImagePromptMode}else{o.ScenePromptID=p.ID;next.TemplatePrompt=p.Prompt;next.ImagePromptMode="custom";if p.BuiltIn{next.ImagePromptMode="blend"}}
 }
 if input.CoverPromptID.Set{
  p,err:=s.resolvePrompt(ctx,input.CoverPromptID.Value,domain.PromptCover);if err!=nil{return model.Manifest{},OverridePlan{},err}
  o.CoverPromptSet=p!=nil;o.CoverPromptID=""
  if p==nil{next.CoverPrompt=template.CoverPrompt.Prompt;next.Thumbnail=template.Thumbnail}else{o.CoverPromptID=p.ID;next.CoverPrompt=p.Prompt;next.Thumbnail=p.Composition}
 }
 if input.NoImages!=nil{o.NoImages=input.NoImages}
 none:=l.snapshot.Job.NoImages||template.Images.Mode=="none"
 if o.NoImages!=nil{none=*o.NoImages}
 if none{next.ImageMode="none"}else{next.ImageMode=""}
 sceneChanged:=old.TemplatePrompt!=next.TemplatePrompt||old.ImagePromptMode!=next.ImagePromptMode
 coverChanged:=old.CoverPrompt!=next.CoverPrompt||old.Thumbnail!=next.Thumbnail
 modeChanged:=old.ImageMode!=next.ImageMode
 plan:=OverridePlan{}
 if s.QuotaLeft==nil{return model.Manifest{},plan,cloneError(domain.CloneErrInternal,"Thiếu nguồn hạn mức tạo ảnh")}
 plan.QuotaLeft,err=s.QuotaLeft(ctx);if err!=nil{return model.Manifest{},plan,err}
 // Turning scenes back on uses current scene identities, never a synthetic id.
 if modeChanged&&!none{
  snapshot,err:=s.Edits.Read(ctx,l.snapshot.Job.ID,domain.CloneScenes);if err!=nil{return model.Manifest{},plan,err};var doc scenes.Manifest;raw:=snapshot.Manifest;if len(raw)==0{raw=snapshot.Step.Checkpoint};if err=json.Unmarshal(raw,&doc);err!=nil{return model.Manifest{},plan,err}
  doc.Recipe.ImagesMode="auto";fresh:=model.CreateManifest(doc,scenes.FileRef{},l.snapshot.Job.Source.Title)
  sceneItems:=[]model.Item{};for _,i:=range fresh.Items{if i.Kind=="scene"{sceneItems=append(sceneItems,i)}};for _,i:=range next.Items{if i.Kind=="thumbnail"{sceneItems=append(sceneItems,i)}};next.Items=sceneItems
 }
 next.QueueOnly=nil
 kept:=make([]model.Item,0,len(next.Items))
 for _,item:=range next.Items{
  if item.Kind=="scene"&&none{continue}
  affected:=(item.Kind=="scene"&&(sceneChanged||modeChanged))||(item.Kind=="thumbnail"&&coverChanged)
  if affected{
   redraw:=len(item.Versions)>0||modeChanged&&!none&&item.Kind=="scene"
   if redraw&&!none {if item.Kind=="scene"{plan.Redraw.Scenes++}else{plan.Redraw.Cover=true}}
   item.Status="pending";item.ReviewVersion=nil;item.SkipReason="";item.Error="";item.PromptVersion++
   // A new video recipe retracts old approvals; unlike feedback redraws, they no longer express the selected recipe.
   item.CurrentVersion=nil
   if !none&&redraw{next.QueueOnly=append(next.QueueOnly,item.ID)}
  }
  kept=append(kept,item)
 }
 next.Items=kept
 plan.Cost.Images=plan.Redraw.Scenes;if plan.Redraw.Cover{plan.Cost.Images++}
 plan.Rebuild=modeChanged||sceneChanged&&next.ImageMode!="none"
 if sceneChanged||coverChanged||modeChanged{next.Story=nil}
 next.Revision++
 return next,plan,nil
}
