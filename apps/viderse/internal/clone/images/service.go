package images

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
)

func cloneError(code, message string) *domain.CloneError {
	return &domain.CloneError{Code: code, Message: message}
}
func (s *Service) load(ctx context.Context, id string, revision *int, mutation bool) (loaded, error) {
	if s.Edits == nil {
		return loaded{}, cloneError(domain.CloneErrInternal, "Thiếu kho chỉnh sửa ảnh")
	}
	snap, err := s.Edits.Read(ctx, id, domain.CloneImages)
	if err != nil {
		return loaded{}, err
	}
	raw := snap.Manifest
	if len(raw) == 0 {
		raw = snap.Step.Checkpoint
	}
	if len(raw) == 0 {
		return loaded{snapshot: snap}, cloneError(domain.CloneErrNotFound, "Chưa có manifest ảnh — chạy bước images trước")
	}
	var m model.Manifest
	if err = json.Unmarshal(raw, &m); err != nil {
		return loaded{}, cloneError(domain.CloneErrInternal, "Manifest ảnh không đọc được")
	}
	if m.Version != 1 || m.Revision < 1 {
		return loaded{}, cloneError(domain.CloneErrInternal, "Manifest ảnh không hợp lệ")
	}
	if mutation && (revision == nil || *revision != m.Revision) {
		return loaded{}, cloneError(domain.CloneErrConflict, "Danh sách ảnh đã thay đổi; tải lại trước khi sửa")
	}
	return loaded{snapshot: snap, manifest: m}, nil
}
func (s *Service) mutable(l loaded) error {
	if l.snapshot.Job.SupersededAt != nil {
		return cloneError(domain.CloneErrNotFound, "Job đã bị thay thế bởi generation mới")
	}
	if jobedit.Busy(l.snapshot, domain.CloneImages) {
		return cloneError(domain.CloneErrConflict, "Ảnh hoặc bước phụ thuộc đang chạy")
	}
	for _, i := range l.manifest.Items {
		if i.Status == "running" || i.InFlight != nil {
			return cloneError(domain.CloneErrConflict, "Ảnh đang tạo hoặc cần phục hồi output đã lưu")
		}
	}
	return nil
}
func (s *Service) currentScenes(ctx context.Context, l loaded) error {
	snap, err := s.Edits.Read(ctx, l.snapshot.Job.ID, domain.CloneScenes)
	if err != nil {
		return err
	}
	if snap.Step.Status == domain.CloneRunning {
		return cloneError(domain.CloneErrConflict, "Cảnh đang được cập nhật — chờ xong rồi duyệt ảnh")
	}
	if snap.Step.Status == domain.CloneStale {
		return cloneError(domain.CloneErrStale, "Cảnh đã thay đổi — chạy lại bước images trước khi duyệt")
	}
	raw := snap.Manifest
	if len(raw) == 0 {
		raw = snap.Step.Checkpoint
	}
	var scene scenes.Manifest
	if len(raw) == 0 || json.Unmarshal(raw, &scene) != nil || scene.Revision != l.manifest.SceneRevision {
		return cloneError(domain.CloneErrStale, "Manifest ảnh thuộc revision cảnh cũ — chạy lại bước images trước khi duyệt")
	}
	if l.manifest.SceneGeneration != 0 && l.manifest.SceneGeneration != l.snapshot.Job.Generation {
		return cloneError(domain.CloneErrStale, "Manifest ảnh thuộc generation cũ")
	}
	return nil
}
func hasPending(m model.Manifest) bool {
	for _, i := range m.Items {
		if i.Status == "pending" || i.Status == "failed" || i.InFlight != nil {
			return true
		}
	}
	return false
}
func artifactURL(snapshot jobedit.Snapshot, path, hash string) string {
	if path == "" {
		return ""
	}
	found := ""
	for _, artifact := range snapshot.Artifacts {
		if artifact.Path != path && !strings.HasSuffix(artifact.Path, "/"+path) {
			continue
		}
		if hash != "" && artifact.SHA256 != hash {
			continue
		}
		if found != "" && found != artifact.ID {
			return ""
		}
		found = artifact.ID
	}
	if found == "" {
		return ""
	}
	return "/api/clone/" + url.PathEscape(snapshot.Job.ID) + "/artifacts/" + url.PathEscape(found)
}

func (s *Service) Read(ctx context.Context, id string) (View, error) {
	l, err := s.load(ctx, id, nil, false)
	if err != nil {
		var ce *domain.CloneError
		if errors.As(err, &ce) && ce.Code == domain.CloneErrNotFound && l.snapshot.Job.ID != "" {
			return View{Items: []ItemView{}, Reason: ce.Message}, nil
		}
		return View{}, err
	}
	m := l.manifest
	out := View{Manifest: &ManifestView{Revision: m.Revision, SceneRevision: m.SceneRevision, SceneGeneration: m.SceneGeneration, Waiting: m.Waiting}, Items: make([]ItemView, 0, len(m.Items)), Approved: model.ImagesApproved(m)}
	var template library.Snapshot
	if len(l.snapshot.Job.TemplateSnapshot) > 0 {
		if err := json.Unmarshal(l.snapshot.Job.TemplateSnapshot, &template); err != nil {
			return View{}, cloneError(domain.CloneErrInternal, "Template snapshot không đọc được")
		}
	}
	out.Manifest.ScenePrompt = EffectivePrompt{ID: template.ScenePrompt.ID, Name: template.ScenePrompt.Name}
	out.Manifest.CoverPrompt = EffectivePrompt{ID: template.CoverPrompt.ID, Name: template.CoverPrompt.Name}
	out.Manifest.Overrides = m.Overrides
	if m.Overrides != nil {
		for _, selection := range []struct {
			set bool
			id string
			target *EffectivePrompt
		}{{m.Overrides.ScenePromptSet, m.Overrides.ScenePromptID, &out.Manifest.ScenePrompt}, {m.Overrides.CoverPromptSet, m.Overrides.CoverPromptID, &out.Manifest.CoverPrompt}} {
			if !selection.set {
				continue
			}
			*selection.target = EffectivePrompt{ID: selection.id, Overridden: true}
			if s.Prompts != nil {
				prompt, err := s.Prompts.GetPrompt(ctx, selection.id)
				if err == nil && prompt != nil {
					selection.target.Name = prompt.Name
				}
			}
		}
	}
	if m.ImageMode != "" {
		mode := m.ImageMode
		out.Manifest.ImageMode = &mode
	}
	queue := 0
	for _, i := range m.Items {
		item := ItemView{ID: i.ID, Kind: i.Kind, Status: i.Status, SkipReason: i.SkipReason, Prompt: i.Prompt, PromptVersion: i.PromptVersion, CurrentVersion: i.CurrentVersion, ReviewVersion: i.ReviewVersion, Reference: i.Reference.Path, Width: i.Width, Height: i.Height, Versions: make([]VersionView, 0, len(i.Versions))}
		item.ReferenceURL = artifactURL(l.snapshot, i.Reference.Path, i.Reference.SHA256)
		if i.InFlight != nil {
			item.Status = "running"
		}
		if i.Status == "pending" || i.InFlight != nil {
			queue++
			position := queue
			item.QueuePosition = &position
		}
		for _, v := range i.Versions {
			source := v.Source
			if source == "" && v.Receipt != nil {
				source = v.Receipt.Provider
			}
			if source == "" {
				source = "codex"
			}
			version := VersionView{Version: v.Version, Path: v.Path, SHA256: v.SHA256, Width: v.Width, Height: v.Height, Bytes: v.Bytes, CreatedAt: v.CreatedAt, PromptVersion: v.PromptVersion, SourceFingerprint: v.SourceFingerprint, Source: source, Cutout: v.Cutout}
			version.OriginalURL = artifactURL(l.snapshot, v.Path, v.SHA256)
			version.CutoutState = "unavailable"
			if runtime.GOOS == "darwin" && s.Runner != nil && version.OriginalURL != "" {
				version.CutoutState = "available"
				version.CutoutURL = "/api/clone/" + url.PathEscape(id) + "/images/" + url.PathEscape(i.ID) + "?variant=cutout&version=" + strconv.Itoa(v.Version)
			}
			if v.Cutout != nil {
				version.CutoutURL = artifactURL(l.snapshot, v.Cutout.Path, v.Cutout.SHA256)
				if version.CutoutURL != "" {
					version.CutoutState = "ready"
				}
			}
			item.Versions = append(item.Versions, version)
		}
		item.SceneID = i.SceneID
		item.StartFrame = i.StartFrame
		item.EndFrame = i.EndFrame
		item.Narration = i.Narration
		item.Direction = i.Direction
		if v := model.Displayed(i); v != nil {
			hash := v.SHA256
			if m.Style == "cutout" && i.Kind == "scene" && v.Cutout != nil {
				hash = v.Cutout.SHA256
			}
			if len(hash) >= 32 {
				item.MediaID = "media-" + hash[:32]
			}
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}
func (s *Service) Review(ctx context.Context, id string, input ReviewInput) (ReviewResult, error) {
	l, err := s.load(ctx, id, input.ExpectedRevision, true)
	if err != nil {
		return ReviewResult{}, err
	}
	if err = s.mutable(l); err != nil {
		return ReviewResult{}, err
	}
	if err = s.currentScenes(ctx, l); err != nil {
		return ReviewResult{}, err
	}
	if len(input.Actions) == 0 {
		return ReviewResult{}, cloneError(domain.CloneErrInvalidRequest, "Cần ít nhất một hành động duyệt ảnh")
	}
	wasApproved := model.ImagesApproved(l.manifest)
	changed, sceneChanged := false, false
	pending := []string{}
	seen := map[string]bool{}
	for _, a := range input.Actions {
		if a.Prompt != nil && strings.TrimSpace(*a.Prompt) == "" {
			return ReviewResult{}, cloneError(domain.CloneErrInvalidRequest, "Prompt không được để trống")
		}
		before := model.Item{}
		for _, i := range l.manifest.Items {
			if i.ID == a.ID {
				before = i
				break
			}
		}
		prompt := ""
		if a.Prompt != nil {
			prompt = *a.Prompt
		}
		l.manifest, err = model.Review(l.manifest, a.ID, a.Op, prompt, a.Version)
		if err != nil {
			return ReviewResult{}, err
		}
		for _, i := range l.manifest.Items {
			if i.ID == a.ID {
				if !sameChoice(before.CurrentVersion, i.CurrentVersion) {
					changed = true
					if i.Kind == "scene" {
						sceneChanged = true
					}
				}
				if (a.Op == "regenerate" || a.Op == "editPrompt" || a.Op == "restore") && !seen[i.ID] {
					pending = append(pending, i.ID)
					seen[i.ID] = true
				}
			}
		}
	}
	var from domain.CloneStep
	rerun := len(pending) > 0
	if rerun {
		from = domain.CloneImages
		l.manifest.QueueOnly = pending
	} else if sceneChanged {
		from = domain.CloneBuild
	}
	if !rerun && from == "" && firstReviewCompleted(l.snapshot, wasApproved, l.manifest) {
		from = domain.CloneBuild
	}
	err = s.Edits.Commit(ctx, l.snapshot, l.manifest, from, rerun, nil)
	if err != nil {
		return ReviewResult{}, err
	}
	out := ReviewResult{Approved: model.ImagesApproved(l.manifest), ChosenChanged: changed, Invalidated: from != "", Kicked: from != "" && !rerun}
	out.Manifest.Revision = l.manifest.Revision
	if rerun {
		out.Generation = &GenerateResult{Accepted: true, JobID: id, Pending: pending, Count: len(pending)}
	}
	return out, nil
}
func sameChoice(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
func firstReviewCompleted(snapshot jobedit.Snapshot, wasApproved bool, next model.Manifest) bool {
	if wasApproved || !model.ImagesApproved(next) {
		return false
	}
	for _, step := range snapshot.Steps {
		if step.Step == domain.CloneBuild && step.Status == domain.CloneSucceeded {
			return false
		}
	}
	return true
}
func (s *Service) Redraw(ctx context.Context, id, itemID string, input RedrawInput) (GenerateResult, error) {
	result, err := s.Review(ctx, id, ReviewInput{ExpectedRevision: input.ExpectedRevision, Actions: []Action{{ID: itemID, Op: "regenerate", Prompt: input.Prompt}}})
	if err != nil {
		return GenerateResult{}, err
	}
	out := *result.Generation
	out.ItemID = itemID
	return out, nil
}
func (s *Service) RedrawAll(ctx context.Context, id string, input ResetInput) (GenerateResult, error) {
	l, err := s.load(ctx, id, input.ExpectedRevision, true)
	if err != nil {
		return GenerateResult{}, err
	}
	if err = s.mutable(l); err != nil {
		return GenerateResult{}, err
	}
	if input.Kind != "" && input.Kind != "scene" && input.Kind != "thumbnail" {
		return GenerateResult{}, cloneError(domain.CloneErrInvalidRequest, "kind phải là scene hoặc thumbnail")
	}
	if len([]rune(input.Feedback)) > 8000 {
		return GenerateResult{}, cloneError(domain.CloneErrInvalidRequest, "Góp ý quá dài")
	}
	var feedback *string
	if strings.TrimSpace(input.Feedback) != "" {
		feedback = &input.Feedback
	}
	if l.manifest.ImageMode == "none" {
		return GenerateResult{}, cloneError(domain.CloneErrInvalidRequest, "Template này không tạo ảnh — đổi ảnh bìa bằng tải lên")
	}
	actions := []Action{}
	for _, i := range l.manifest.Items {
		if input.Kind == "" || input.Kind == i.Kind {
			actions = append(actions, Action{ID: i.ID, Op: "regenerate", Prompt: feedback})
		}
	}
	if input.ConfirmCount != len(actions) {
		return GenerateResult{}, cloneError(domain.CloneErrConflict, "Số ảnh xác nhận không khớp danh sách hiện tại")
	}
	if len(actions) == 0 {
		return GenerateResult{JobID: id, Pending: []string{}}, nil
	}
	result, err := s.Review(ctx, id, ReviewInput{ExpectedRevision: input.ExpectedRevision, Actions: actions})
	if err != nil {
		return GenerateResult{}, err
	}
	return *result.Generation, nil
}
func (s *Service) ApproveAll(ctx context.Context, id string, input RevisionInput) (MutationResult, error) {
	l, err := s.load(ctx, id, input.ExpectedRevision, true)
	if err != nil {
		return MutationResult{}, err
	}
	if err = s.mutable(l); err != nil {
		return MutationResult{}, err
	}
	if err = s.currentScenes(ctx, l); err != nil {
		return MutationResult{}, err
	}
	if !model.ImagesReady(l.manifest) {
		return MutationResult{}, cloneError(domain.CloneErrInvalidRequest, "Chưa có đủ ảnh hợp lệ để duyệt")
	}
	for _, i := range l.manifest.Items {
		if i.Status == "review" {
			l.manifest, err = model.Review(l.manifest, i.ID, "approve", "", nil)
			if err != nil {
				return MutationResult{}, err
			}
		}
	}
	if !model.ImagesApproved(l.manifest) {
		return MutationResult{}, cloneError(domain.CloneErrInvalidRequest, "Chưa có đủ ảnh để duyệt")
	}
	if input.ApproveExport {
		err = s.Edits.CommitOwnerApproval(ctx, l.snapshot, l.manifest)
	} else {
		err = s.Edits.Commit(ctx, l.snapshot, l.manifest, domain.CloneBuild, false, nil)
	}
	if err != nil {
		return MutationResult{}, err
	}
	return MutationResult{JobID: id, Revision: l.manifest.Revision}, nil
}
func (s *Service) DisplayedCutout(ctx context.Context, id, itemID string) (string, error) {
	l, err := s.load(ctx, id, nil, false)
	if err != nil {
		return "", err
	}
	for _, i := range l.manifest.Items {
		if i.ID != itemID {
			continue
		}
		v := model.Displayed(i)
		if v == nil || v.Cutout == nil || v.Cutout.Path == "" {
			return "", cloneError(domain.CloneErrNotFound, "Ảnh chưa có bản tách nền")
		}
		return v.Cutout.Path, nil
	}
	return "", cloneError(domain.CloneErrNotFound, "Không tìm thấy ảnh")
}
