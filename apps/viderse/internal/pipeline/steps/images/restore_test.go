package images

import (
 "context"
 "encoding/json"
 "errors"
 "os"
 "testing"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestHandlerRestoresPublishedVideoOverrideWithoutDrawing(t *testing.T){
 s,_:=fixture(t);var old Manifest;if err:=json.Unmarshal(s.Checkpoint,&old);err!=nil{t.Fatal(err)}
 old.SceneRevision=0;old.ImageMode="none";old.TemplatePrompt="video-specific art";none:=true
 old.Overrides=&Overrides{ScenePromptID:"custom",ScenePromptSet:true,NoImages:&none}
 item:=&old.Items[0];n:=1;item.Status="approved";item.CurrentVersion=&n;item.Versions=[]Version{{Version:1,Source:"upload",SourceFingerprint:SourceFingerprint(*item),Path:"cover.webp"}}
 s.Checkpoint=nil;called:=0
 h:=Handler{Deps:Deps{ReadManifest:func(context.Context,domain.CloneJob)(*Manifest,error){called++;return &old,nil}}}
 result,err:=h.Run(context.Background(),s);if err!=nil{t.Fatal(err)}
 raw,err:=os.ReadFile(result.Artifacts[0].File);if err!=nil{t.Fatal(err)};var got Manifest;if err=json.Unmarshal(raw,&got);err!=nil{t.Fatal(err)}
 if called!=1||got.Overrides==nil||got.Overrides.ScenePromptID!="custom"||got.TemplatePrompt!="video-specific art"||len(got.Items[0].Versions)!=1{t.Fatalf("lost restored video state: %+v",got)}
}
func TestHandlerPropagatesRestoreFailureInsteadOfRedrawing(t *testing.T){
 s,_:=fixture(t);s.Checkpoint=nil;failure:=errors.New("published manifest unavailable")
 h:=Handler{Deps:Deps{ReadManifest:func(context.Context,domain.CloneJob)(*Manifest,error){return nil,failure}}}
 if _,err:=h.Run(context.Background(),s);!errors.Is(err,failure){t.Fatalf("got %v",err)}
}
