package main

import (
 "context"
 "encoding/json"
 "errors"
 "testing"

 cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/library"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline"
 imagepipeline "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

type queuePromptCatalog struct{}
func (queuePromptCatalog) GetPrompt(_ context.Context, id string) (*domain.ImagePrompt, error) {
 for _, p := range library.BuiltinPrompts() { if p.ID == id { return &p, nil } }
 return nil, &domain.CloneError{Code:domain.CloneErrNotFound}
}

func TestQueuedPromptLifecyclePG(t *testing.T) {
 pool, _ := provisionCLI(t); ctx := context.Background()
 _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name) VALUES ('11111111-1111-4111-8111-111111111111','Kênh');
 INSERT INTO templates(id,name,recipe) VALUES ('aaaaaaaa-1111-4111-8111-111111111111','Mẫu','{}');
 INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,source_snapshot,position) VALUES ('33333333-3333-4333-8333-333333333333','11111111-1111-4111-8111-111111111111','aaaaaaaaaaa','queued-override','{"title":"Video"}',1)`)
 if err != nil { t.Fatal(err) }
 catalog := queuePromptCatalog{}
 svc := &queuedPromptOperations{Service:&cloneimages.Service{Prompts:catalog, QuotaLeft:func(context.Context)(int,error){return 7,nil}}, queue:postgres.NewQueueStore(pool)}
 scene, cover, noImages, rev := library.BuiltinScene, library.BuiltinCoverHook, true, 1
 input := cloneimages.OverrideInput{ScenePromptID:cloneimages.OptionalString{Set:true,Value:&scene}, CoverPromptID:cloneimages.OptionalString{Set:true,Value:&cover}, NoImages:&noImages, ExpectedRevision:&rev}
 plan, err := svc.PlanOverrides(ctx, "33333333-3333-4333-8333-333333333333", input)
 if err != nil || plan.Cost.Images != 0 || plan.Redraw.Scenes != 0 || plan.Redraw.Cover || plan.Rebuild || plan.QuotaLeft != 7 { t.Fatalf("plan=%+v err=%v", plan, err) }
 if _, err = svc.ApplyOverrides(ctx, "33333333-3333-4333-8333-333333333333", input); err != nil { t.Fatal(err) }
 if _, err = svc.ApplyOverrides(ctx, "33333333-3333-4333-8333-333333333333", input); err == nil { t.Fatal("stale revision accepted") }
 item, err := postgres.NewQueueStore(pool).GetQueueItem(ctx, "33333333-3333-4333-8333-333333333333")
 if err != nil || item == nil || item.OverrideRevision != 2 || item.ScenePromptID == nil || *item.ScenePromptID != scene || item.CoverPromptID == nil || *item.CoverPromptID != cover || !item.NoImages { t.Fatalf("restart row=%+v err=%v", item, err) }
 cloneStore := postgres.NewCloneStore(pool)
 runner := &pipeline.Service{Store:cloneStore, ResolveTemplate:func(context.Context,domain.CloneQueueItem)(string,json.RawMessage,json.RawMessage,error){return "aaaaaaaa-1111-4111-8111-111111111111",json.RawMessage(`{"imagePrompt":"template"}`),json.RawMessage(`{}`),nil}, ResolveQueuedImageOverrides:func(ctx context.Context, item domain.CloneQueueItem)(json.RawMessage,error){return queuedImageCheckpoint(ctx,catalog,item)}}
 stale := *item; stale.OverrideRevision = 1
 if _, err = runner.Start(ctx, stale); err == nil { t.Fatal("stale queue snapshot admitted") } else { var ce *domain.CloneError; if !errors.As(err,&ce) || ce.Code != domain.CloneErrConflict { t.Fatal(err) } }
 job, err := runner.Start(ctx, *item); if err != nil { t.Fatal(err) }
 var snapshot map[string]any
 if err := json.Unmarshal(job.TemplateSnapshot, &snapshot); err != nil || len(snapshot) != 1 || snapshot["imagePrompt"] != "template" { t.Fatalf("template changed: %s %v", job.TemplateSnapshot, err) }
 step, err := cloneStore.GetStep(ctx, job.ID, domain.CloneImages); if err != nil { t.Fatal(err) }
 var manifest imagepipeline.Manifest; if err = json.Unmarshal(step.Checkpoint, &manifest); err != nil { t.Fatal(err) }
 if manifest.SceneRevision != -1 || manifest.Overrides == nil || manifest.Overrides.ScenePromptID != scene || manifest.Overrides.CoverPromptID != cover || manifest.Overrides.NoImages == nil || !*manifest.Overrides.NoImages || manifest.TemplatePrompt == "" || manifest.CoverPrompt == "" { t.Fatalf("checkpoint=%+v",manifest) }
 fresh := imagepipeline.Manifest{SceneRevision:1, TemplatePrompt:"template", CoverPrompt:"template", Items:[]imagepipeline.Item{{ID:"cover",Kind:"thumbnail"},{ID:"scene",Kind:"scene"}}}
 got := imagepipeline.Reconcile(manifest, fresh)
 if got.TemplatePrompt != manifest.TemplatePrompt || got.CoverPrompt != manifest.CoverPrompt || got.ImageMode != "none" || len(got.Items) != 1 { t.Fatalf("resolved manifest=%+v", got) }
}
