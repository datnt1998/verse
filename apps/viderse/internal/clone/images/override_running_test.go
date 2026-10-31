package images

import (
 "context"
 "fmt"
 "testing"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

func TestOverrideAcceptedWhileImageWorkerRuns(t *testing.T) {
 s, store := testService()
 store.m.Items = nil
 for n := range 6 {
  item:=model.Item{ID:fmt.Sprintf("scene-%d",n), Kind:"scene", Status:"pending"}
  if n<4 { item.Status="review"; item.Versions=[]model.Version{{Version:1}} }
  if n==4 { item.Status="running"; item.InFlight=&model.Flight{GenerationID:"ongoing"} }
  store.m.Items=append(store.m.Items,item)
 }
 s.Edits.Store=runningEditStore{store}
 s.Edits.Media=&stepstest.FakeMedia{Root:t.TempDir()}
 s.Prompts=promptCatalog{}
 s.QuotaLeft=func(context.Context)(int,error){return 20,nil}
 prompt:="new-scene"
 revision:=3
 count:=4
 noImages := false
 plan,err:=s.ApplyOverrides(context.Background(),"job",OverrideInput{ExpectedRevision:&revision,ScenePromptID:OptionalString{Set:true,Value:&prompt},NoImages:&noImages,ConfirmCount:&count})
 if err!=nil { t.Fatalf("active image override rejected: %v",err) }
 if plan.Redraw.Scenes!=4 || plan.Cost.Images!=4 { t.Fatalf("incorrect cost: %+v",plan) }
 if store.m.Items[4].Status!="running" || store.m.Items[4].InFlight==nil { t.Fatal("lost active flight") }
 if store.m.Items[5].Status!="pending" { t.Fatal("lost pending image") }
 if store.change.InvalidateFrom!=domain.CloneImages { t.Fatalf("override was not queued: %+v",store.change) }
}

func TestNoImagesAcceptedWhileWorkerRunsWithoutCharge(t *testing.T) {
 s, store := testService()
 store.m.Items[0].Status = "running"
 store.m.Items[0].InFlight = &model.Flight{GenerationID:"ongoing"}
 s.Edits.Store = runningEditStore{store}
 s.Prompts = promptCatalog{}
 s.QuotaLeft = func(context.Context)(int,error){ return 20,nil }
 noImages, revision := true, 3
 plan, err := s.ApplyOverrides(context.Background(),"job",OverrideInput{ExpectedRevision:&revision,NoImages:&noImages})
 if err != nil { t.Fatalf("no-images override rejected: %v",err) }
 if plan.Cost.Images != 0 || store.m.ImageMode != "none" || !store.change.ImageOverride { t.Fatalf("wrong no-images plan: %+v %+v",plan,store.m) }
}

func TestCoverOverrideRedrawsOnlyCoverWhileWorkerRuns(t *testing.T) {
 s, store := testService()
 store.m.Items[0].Status = "running"
 store.m.Items[0].InFlight = &model.Flight{GenerationID:"ongoing"}
 s.Edits.Store = runningEditStore{store}
 s.Prompts = coverPromptCatalog{}
 s.QuotaLeft = func(context.Context)(int,error){ return 20,nil }
 revision, count, prompt := 3, 1, "cover-new"
 plan, err := s.ApplyOverrides(context.Background(),"job",OverrideInput{ExpectedRevision:&revision,CoverPromptID:OptionalString{Set:true,Value:&prompt},ConfirmCount:&count})
 if err != nil { t.Fatalf("cover override rejected: %v",err) }
 if plan.Cost.Images != 1 || plan.Redraw.Scenes != 0 || !plan.Redraw.Cover { t.Fatalf("wrong cover plan: %+v",plan) }
 if len(store.m.QueueOnly) != 1 || store.m.QueueOnly[0] != "thumbnail" || store.m.Items[0].InFlight == nil { t.Fatalf("wrong cover redraw: %+v",store.m) }
}
