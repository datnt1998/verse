package main

import (
 "context"
 "encoding/json"
 "strings"

 cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/library"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
 imagepipeline "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

// queuedPromptOperations dispatches queued IDs without changing job-only image operations.
type queuedPromptOperations struct {
 *cloneimages.Service
 queue *postgres.QueueStore
 publish func(string, any)
}

func queuedPrompt(ctx context.Context, catalog cloneimages.PromptCatalog, value *string, kind domain.PromptKind) (*domain.ImagePrompt, error) {
 if value == nil { return nil, nil }
 id := strings.TrimSpace(*value)
 if id == "" { return nil, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message:"Prompt ảnh không được để trống"} }
 p, err := catalog.GetPrompt(ctx, id)
 if err != nil {
  if library.CodeOf(err) == library.CodeNotFound { return nil, &domain.CloneError{Code:domain.CloneErrNotFound, Message:"Không tìm thấy prompt ảnh đã chọn"} }
  return nil, &domain.CloneError{Code:domain.CloneErrInternal, Message:"Không đọc được thư viện prompt ảnh"}
 }
 if p == nil || p.Kind != kind { return nil, &domain.CloneError{Code:domain.CloneErrInvalidRequest, Message:"Loại prompt không khớp ảnh cảnh hoặc ảnh bìa"} }
 return p, nil
}

func (s *queuedPromptOperations) prepare(ctx context.Context, item *domain.CloneQueueItem, in cloneimages.OverrideInput) (cloneimages.OverridePlan, error) {
 var plan cloneimages.OverridePlan
 if item.State != domain.QueueQueued || item.RemovedAt != nil { return plan, &domain.CloneError{Code:domain.CloneErrConflict, Message:"Mục hàng chờ đã bị bỏ hoặc đã chạy"} }
 if in.ScenePromptID.Set {
  p, err := queuedPrompt(ctx, s.Prompts, in.ScenePromptID.Value, domain.PromptScene); if err != nil { return plan, err }
  item.ScenePromptID = nil; if p != nil { item.ScenePromptID = &p.ID }
 }
 if in.CoverPromptID.Set {
  p, err := queuedPrompt(ctx, s.Prompts, in.CoverPromptID.Value, domain.PromptCover); if err != nil { return plan, err }
  item.CoverPromptID = nil; if p != nil { item.CoverPromptID = &p.ID }
 }
 if in.NoImages != nil { item.NoImages = *in.NoImages; item.NoImagesOverride = in.NoImages }
 if s.QuotaLeft == nil { return plan, &domain.CloneError{Code:domain.CloneErrInternal, Message:"Thiếu nguồn hạn mức tạo ảnh"} }
 var err error
 plan.QuotaLeft, err = s.QuotaLeft(ctx)
 return plan, err
}
func (s *queuedPromptOperations) PlanOverrides(ctx context.Context, id string, in cloneimages.OverrideInput) (cloneimages.OverridePlan, error) {
 item, err := s.queue.GetQueueItem(ctx, id)
 if err != nil { return cloneimages.OverridePlan{}, err }
 if item == nil { return s.Service.PlanOverrides(ctx, id, in) }
 return s.prepare(ctx, item, in)
}
func (s *queuedPromptOperations) ApplyOverrides(ctx context.Context, id string, in cloneimages.OverrideInput) (cloneimages.OverridePlan, error) {
 item, err := s.queue.GetQueueItem(ctx, id)
 if err != nil { return cloneimages.OverridePlan{}, err }
 if item == nil { return s.Service.ApplyOverrides(ctx, id, in) }
 if in.ExpectedRevision == nil || int64(*in.ExpectedRevision) != item.OverrideRevision { return cloneimages.OverridePlan{}, &domain.CloneError{Code:domain.CloneErrConflict, Message:"Mục hàng chờ đã thay đổi, vui lòng tải lại"} }
 plan, err := s.prepare(ctx, item, in); if err != nil { return plan, err }
 ok, err := s.queue.UpdateQueueOverrides(ctx, *item, item.OverrideRevision)
 if err != nil { return plan, err }; if !ok { return plan, &domain.CloneError{Code:domain.CloneErrConflict, Message:"Mục hàng chờ đã thay đổi, vui lòng tải lại"} }
 if s.publish != nil { s.publish(domain.CloneEventQueue, map[string]any{"id":id}) }
 return plan, nil
}

// queuedImageCheckpoint holds video overrides separately from the immutable template snapshot.
// A scene revision of -1 forces the images step to reconcile with the first real scene manifest.
func queuedImageCheckpoint(ctx context.Context, catalog cloneimages.PromptCatalog, item domain.CloneQueueItem) (json.RawMessage, error) {
 if item.ScenePromptID == nil && item.CoverPromptID == nil && item.NoImagesOverride == nil { return nil, nil }
 m := imagepipeline.Manifest{Version:1, SceneRevision:-1, Overrides:&imagepipeline.Overrides{NoImages:item.NoImagesOverride}}
 if item.ScenePromptID != nil {
  p, err := queuedPrompt(ctx, catalog, item.ScenePromptID, domain.PromptScene); if err != nil { return nil, err }
  m.Overrides.ScenePromptSet, m.Overrides.ScenePromptID = true, p.ID
  m.TemplatePrompt, m.ImagePromptMode = p.Prompt, "custom"; if p.BuiltIn { m.ImagePromptMode = "blend" }
 }
 if item.CoverPromptID != nil {
  p, err := queuedPrompt(ctx, catalog, item.CoverPromptID, domain.PromptCover); if err != nil { return nil, err }
  m.Overrides.CoverPromptSet, m.Overrides.CoverPromptID = true, p.ID
  m.CoverPrompt, m.Thumbnail = p.Prompt, p.Composition
 }
 return json.Marshal(m)
}
