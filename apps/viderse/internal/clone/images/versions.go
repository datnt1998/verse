package images

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
)

// providerAlias chuẩn hoá tên nguồn vẽ của giao diện về tên provider thật.
func providerAlias(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		return ""
	case "chatgpt", "codex":
		return "codex"
	case "grok":
		return "grok"
	}
	return ""
}

// ProviderID là tên nguồn vẽ theo 07 §26 (codex hiện là "chatgpt").
func ProviderID(name string) string {
	if strings.EqualFold(name, "codex") {
		return "chatgpt"
	}
	return name
}

// imagesView dựng danh sách ảnh theo 07 §26 từ manifest và artifact đã công bố.
func (s *Service) imagesView(ctx context.Context, l loaded, refLimit int) []ImageView {
	out := make([]ImageView, 0, len(l.manifest.Items))
	sceneNumber := 0
	for _, i := range l.manifest.Items {
		index := 1
		label := "Ảnh bìa"
		kind := "cover"
		if i.Kind == "scene" {
			sceneNumber++
			index, label, kind = sceneNumber, fmt.Sprintf("Cảnh %d", sceneNumber), "scene"
		}
		used := 0
		if i.CurrentVersion != nil {
			used = *i.CurrentVersion
		}
		image := ImageView{ID: i.ID, Kind: kind, Index: index, Label: label, UsedVersion: used, NeedsReview: i.NeedsReview, Status: i.Status, RefLimit: refLimit, Versions: make([]ImageVersionView, 0, len(i.Versions))}
		if i.InFlight != nil {
			image.Status = "running"
		}
		for _, v := range i.Versions {
			image.Versions = append(image.Versions, s.versionView(ctx, l, i, v))
		}
		out = append(out, image)
	}
	return out
}

// versionView dựng một phiên bản: URL tệp, các phần prompt, ảnh tham chiếu và
// lỗi thật. Bản cũ chưa tách phần thì cả prompt là phần Cảnh.
func (s *Service) versionView(ctx context.Context, l loaded, item model.Item, v model.Version) ImageVersionView {
	state := v.State()
	var url *string
	if v.Path != "" {
		if value := artifactURL(l.snapshot, v.Path, v.SHA256); value != "" {
			url = &value
		}
	}
	parts := append([]model.PromptPart(nil), v.Parts...)
	legacy := false
	if len(parts) == 0 && v.Prompt != "" {
		parts = []model.PromptPart{{Kind: model.PartScene, Text: v.Prompt, Source: model.PromptPartSource{Type: "artist", Label: "Bản cũ · chưa tách phần"}}}
		legacy = true
	}
	// Bản lưu trước khi góp ý chuyển vào wrapper có câu "Owner visual
	// direction…" dính cuối phần Cảnh — hiển thị phải bỏ câu đó (sentText
	// giữ nguyên văn đã gửi nhà cung cấp).
	for i := range parts {
		if parts[i].Kind == model.PartScene {
			parts[i].Text = strings.TrimRight(model.StripDirection(parts[i].Text), "\n")
		}
	}
	refs := make([]model.ImageRef, 0, len(v.Refs))
	for _, ref := range v.Refs {
		ref.URL = ""
		if ref.Path != "" {
			ref.URL = artifactURL(l.snapshot, ref.Path, ref.SHA256)
		}
		refs = append(refs, ref)
	}
	provider := v.Provider
	if provider == "" && v.Receipt != nil {
		provider = v.Receipt.Provider
	}
	provider = ProviderID(provider)
	modelName := v.Model
	if modelName == "" && v.Receipt != nil {
		modelName = v.Receipt.Model
	}
	params := model.PromptParams{Size: fmt.Sprintf("%dx%d", item.Width, item.Height)}
	if v.Params != nil && v.Params.Size != "" {
		params = *v.Params
	}
	started := v.StartedAt
	if started == "" {
		started = v.CreatedAt
	}
	return ImageVersionView{
		N: v.Version, State: state, URL: url, PreviewURL: nil, Width: v.Width, Height: v.Height,
		Parts: parts, Wrapper: v.Wrapper, SentText: v.Prompt, Refs: refs, Provider: provider, Model: modelName, Params: params,
		Feedback: v.Feedback, DiffFromPrev: v.DiffFromPrev, StartedAt: started, FinishedAt: v.FinishedAt,
		DurationSec: v.DurationSec, CostImages: v.CostImages, Error: v.VersionError, PartsLegacy: legacy,
	}
}

// quotaView trả hạn mức ảnh còn lại của các nguồn vẽ của video này.
func (s *Service) quotaView(ctx context.Context) *QuotaView {
	if s.Quotas == nil {
		return nil
	}
	providers, err := s.Quotas(ctx)
	if err != nil {
		return nil
	}
	out := &QuotaView{Providers: providers}
	for _, provider := range providers {
		out.Left += provider.Left
	}
	if len(providers) > 1 {
		fallback := providers[1]
		out.Fallback = &fallback
	}
	return out
}

// refLimitFor trả trần ảnh tham chiếu của nguồn vẽ đang dùng cho video.
func (s *Service) refLimitFor(ctx context.Context, provider string) int {
	if s.RefLimits != nil {
		if limit, ok := s.RefLimits(ctx, providerAlias(provider)); ok {
			return limit
		}
	}
	return imagegen.RefLimit(providerAlias(provider))
}

// RedrawVersions vẽ lại một hoặc nhiều ảnh với góp ý chung, phần prompt và ảnh
// tham chiếu chủ kênh sửa (07 §26). Trả số bản kế tiếp của từng ảnh.
func (s *Service) RedrawVersions(ctx context.Context, id string, input RedrawVersionsInput) (RedrawVersionsResult, error) {
	l, err := s.load(ctx, id, nil, false)
	if err != nil {
		return RedrawVersionsResult{}, err
	}
	if err = s.mutableReview(l); err != nil {
		return RedrawVersionsResult{}, err
	}
	if err = s.currentScenes(ctx, l); err != nil {
		return RedrawVersionsResult{}, err
	}
	if l.manifest.ImageMode == "none" {
		return RedrawVersionsResult{}, cloneError(domain.CloneErrInvalidRequest, "Template này không tạo ảnh — đổi ảnh bìa bằng tải lên")
	}
	ids := make([]string, 0, len(input.ImageIDs))
	seen := map[string]bool{}
	for _, imageID := range input.ImageIDs {
		if imageID == "" || seen[imageID] {
			continue
		}
		if _, ok := model.FindItem(l.manifest, imageID); !ok {
			return RedrawVersionsResult{}, cloneError(domain.CloneErrNotFound, "Không tìm thấy ảnh")
		}
		seen[imageID] = true
		ids = append(ids, imageID)
	}
	if len(ids) == 0 {
		return RedrawVersionsResult{}, cloneError(domain.CloneErrInvalidRequest, "Cần chọn ít nhất một ảnh để vẽ lại")
	}
	if len(input.Parts) > 0 && len(ids) > 1 {
		return RedrawVersionsResult{}, cloneError(domain.CloneErrInvalidRequest, "Sửa prompt chỉ áp cho một ảnh")
	}
	if len([]rune(input.Feedback)) > 8000 {
		return RedrawVersionsResult{}, cloneError(domain.CloneErrInvalidRequest, "Góp ý quá dài")
	}
	provider := providerAlias(input.Provider)
	if input.Provider != "" && provider == "" {
		return RedrawVersionsResult{}, cloneError(domain.CloneErrInvalidRequest, "Nguồn vẽ không hợp lệ")
	}
	parts := model.PartOverrides{}
	for kind, text := range input.Parts {
		key := model.PromptPartKind(kind)
		valid := false
		for _, known := range model.PartOrder {
			if known == key {
				valid = true
				break
			}
		}
		if !valid {
			return RedrawVersionsResult{}, cloneError(domain.CloneErrInvalidRequest, "Phần prompt không hợp lệ")
		}
		parts[key] = text
	}
	limit := s.refLimitFor(ctx, provider)
	refs, err := s.resolveRefs(l, input.Refs)
	if err != nil {
		return RedrawVersionsResult{}, err
	}
	if len(refs) > limit {
		return RedrawVersionsResult{}, &TooManyRefsError{Limit: limit}
	}
	feedback := strings.TrimSpace(input.Feedback)
	out := RedrawVersionsResult{Versions: make([]PlannedVersion, 0, len(ids))}
	for index := range l.manifest.Items {
		item := &l.manifest.Items[index]
		if !seen[item.ID] {
			continue
		}
		draft := &model.PromptDraft{Parts: parts, Feedback: feedback, Provider: provider, BaseStatus: item.Status}
		draft.Project = input.Project
		if input.Refs != nil {
			chosen := append([]model.ImageRef(nil), refs...)
			draft.Refs = &chosen
		}
		item.Draft = draft
		item.Status = "pending"
		item.ReviewVersion = nil
		item.Error = ""
		item.SkipReason = ""
		out.Versions = append(out.Versions, PlannedVersion{ImageID: item.ID, N: model.NextVersion(*item)})
	}
	l.manifest.QueueOnly = append([]string(nil), ids...)
	if l.snapshot.Job.StageGroup == domain.StageReview {
		l.manifest.ReviewOrigin = true
	}
	l.manifest.Revision++
	if err = s.Edits.CommitImageRedraw(ctx, l.snapshot, l.manifest, ids); err != nil {
		return RedrawVersionsResult{}, err
	}
	return out, nil
}

// resolveRefs chuyển ảnh tham chiếu của giao diện thành ảnh tham chiếu của lần
// vẽ: ảnh trong video, khung video gốc, hoặc tệp chủ kênh tải lên.
func (s *Service) resolveRefs(l loaded, refs []RedrawRef) ([]model.ImageRef, error) {
	out := make([]model.ImageRef, 0, len(refs))
	for _, ref := range refs {
		role := model.RefRole(ref.Role)
		if !model.ValidRefRole(role) {
			return nil, cloneError(domain.CloneErrInvalidRequest, "Vai trò ảnh tham chiếu không hợp lệ")
		}
		switch {
		case ref.Upload != "":
			out = append(out, model.ImageRef{ID: "upload:" + ref.Upload, Role: role, Source: model.RefSource{Type: "upload", Label: "Bạn tải lên", ImageID: ref.Upload}})
		case ref.VideoImageID != "":
			item, ok := model.FindItem(l.manifest, ref.VideoImageID)
			if !ok {
				return nil, cloneError(domain.CloneErrInvalidRequest, "Ảnh trong video không tồn tại")
			}
			version := model.Current(item)
			if version == nil || version.Path == "" {
				return nil, cloneError(domain.CloneErrInvalidRequest, "Ảnh đó chưa có bản để làm tham chiếu")
			}
			out = append(out, model.ImageRef{ID: "video:" + item.ID, Role: role, Path: version.Path, SHA256: version.SHA256, Source: model.RefSource{Type: "video_image", Label: "Ảnh trong video · " + item.ID, ImageID: item.ID}})
		case ref.RivalFrame != nil:
			at := ref.RivalFrame.AtSec
			label := "Khung đối thủ · " + clock(ref.RivalFrame.AtSec)
			out = append(out, model.ImageRef{ID: "rival:" + ref.RivalFrame.VideoID + ":" + fmt.Sprintf("%.2f", at), Role: role, Source: model.RefSource{Type: "rival_frame", Label: label, VideoID: ref.RivalFrame.VideoID, AtSec: &at}})
		case ref.ID != "":
			existing, ok := findRef(l.manifest, ref.ID)
			if !ok {
				return nil, cloneError(domain.CloneErrInvalidRequest, "Ảnh tham chiếu không tồn tại")
			}
			existing.Role = role
			out = append(out, existing)
		default:
			return nil, cloneError(domain.CloneErrInvalidRequest, "Ảnh tham chiếu thiếu nguồn")
		}
	}
	return out, nil
}

// findRef tìm một ảnh tham chiếu đã gửi ở bản đang dùng của video.
func findRef(m model.Manifest, id string) (model.ImageRef, bool) {
	for _, item := range m.Items {
		for _, version := range item.Versions {
			for _, ref := range version.Refs {
				if ref.ID == id {
					return ref, true
				}
			}
		}
	}
	return model.ImageRef{}, false
}

func clock(sec float64) string {
	total := int(sec + .5)
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

// UseVersion chọn bản đang dùng cho video và thay media của clip trong project
// đã dựng (07 §26). project=false để Editor tự áp thay đổi bằng một bước hoàn
// tác của nó.
func (s *Service) UseVersion(ctx context.Context, id, imageID string, input UseVersionInput) (UseVersionResult, error) {
	l, err := s.load(ctx, id, nil, false)
	if err != nil {
		return UseVersionResult{}, err
	}
	if err = s.mutableReview(l); err != nil {
		return UseVersionResult{}, err
	}
	index := -1
	for n := range l.manifest.Items {
		if l.manifest.Items[n].ID == imageID {
			index = n
			break
		}
	}
	if index < 0 {
		return UseVersionResult{}, cloneError(domain.CloneErrNotFound, "Không tìm thấy ảnh")
	}
	item := &l.manifest.Items[index]
	chosen := model.Current(*item)
	for n := range item.Versions {
		if item.Versions[n].Version == input.N {
			candidate := item.Versions[n]
			chosen = &candidate
			break
		}
	}
	if chosen == nil || chosen.Version != input.N {
		return UseVersionResult{}, cloneError(domain.CloneErrNotFound, "Không tìm thấy bản ảnh này")
	}
	if chosen.State() != model.VersionReady {
		return UseVersionResult{}, cloneError(domain.CloneErrConflict, "Bản ảnh này chưa vẽ xong")
	}
	if l.manifest.Style == "cutout" && item.Kind == "scene" && (chosen.Cutout == nil || chosen.Cutout.Path == "") {
		return UseVersionResult{}, cloneError(domain.CloneErrInvalidRequest, "Bản ảnh này chưa có bản tách nền")
	}
	result := UseVersionResult{ImageID: item.ID, N: input.N, PreviousVersion: item.CurrentVersion}
	previous := 0
	if item.CurrentVersion != nil {
		previous = *item.CurrentVersion
	}
	n := input.N
	item.CurrentVersion = &n
	item.ReviewVersion = nil
	item.NeedsReview = false
	item.Status = "approved"
	// Bản đã là bản đang dùng (ví dụ worker vừa tự gán bản vừa vẽ xong): vẫn
	// phải Rebind — media của clip trong project có thể chưa thay, và Editor
	// cần clipId/mediaId để tự thay bằng một lệnh hoàn tác được.
	if previous != input.N {
		l.manifest.Revision++
	}
	if err = s.Edits.CommitImageReview(ctx, l.snapshot, l.manifest, "", false, []string{item.ID}); err != nil {
		return UseVersionResult{}, err
	}
	result.UsedVersion = n
	result.Quality = "none"
	if s.Binder == nil {
		return result, nil
	}
	applyProject := input.Project == nil || *input.Project
	bound, err := s.Binder.Rebind(ctx, RebindRequest{JobID: id, ImageID: item.ID, SceneID: item.SceneID, Kind: item.Kind, Version: chosen.Version, Path: chosen.Path, SHA256: chosen.SHA256, Mime: chosen.Mime, Width: chosen.Width, Height: chosen.Height, Cutout: chosen.Cutout, ApplyProject: applyProject})
	if err != nil {
		result.ProjectError = err.Error()
		return result, nil
	}
	if bound != nil {
		result.ProjectID = &bound.ProjectID
		result.ClipID = &bound.ClipID
		result.MediaID = &bound.MediaID
		result.Src = &bound.Src
		if bound.PreviousMediaID != "" {
			result.PreviousMediaID = &bound.PreviousMediaID
		}
		result.Quality = bound.Quality
	}
	return result, nil
}

// CancelVersion huỷ một lần vẽ đang chạy (07 §26): chưa gửi nhà cung cấp thì
// không tốn ảnh nào, bản đang dùng giữ nguyên.
func (s *Service) CancelVersion(ctx context.Context, id, imageID string, n int) (CancelVersionResult, error) {
	l, err := s.load(ctx, id, nil, false)
	if err != nil {
		return CancelVersionResult{}, err
	}
	index := -1
	for i := range l.manifest.Items {
		if l.manifest.Items[i].ID == imageID {
			index = i
			break
		}
	}
	if index < 0 {
		return CancelVersionResult{}, cloneError(domain.CloneErrNotFound, "Không tìm thấy ảnh")
	}
	item := &l.manifest.Items[index]
	used := 0
	if item.CurrentVersion != nil {
		used = *item.CurrentVersion
	}
	result := CancelVersionResult{ImageID: item.ID, N: n, UsedVersion: used}
	for _, version := range item.Versions {
		// Dòng `drawing` là lần vẽ đang chạy nên huỷ được; bản đã xong thì không.
		if state := version.State(); version.Version == n && state != model.VersionCancelled && state != model.VersionDrawing {
			return CancelVersionResult{}, cloneError(domain.CloneErrConflict, "Bản ảnh này đã vẽ xong — không huỷ được")
		}
	}
	if item.InFlight != nil && item.InFlight.Version == n {
		if s.Cancels != nil {
			s.Cancels.Cancel(model.DrawKey(id, item.ID, n))
		}
		result.Cancelled = true
		return result, nil
	}
	// Dòng `drawing` còn trong manifest nhưng chuyến vẽ đã mất (job dừng, tiến
	// trình chết hẳn): huỷ được, không tốn ảnh, ảnh về bản đang dùng.
	for at := range item.Versions {
		if item.Versions[at].Version != n || item.Versions[at].State() != model.VersionDrawing {
			continue
		}
		finished := time.Now().UTC().Format(time.RFC3339Nano)
		item.Versions[at].StateName = model.VersionCancelled
		item.Versions[at].FinishedAt = &finished
		item.Versions[at].CostImages = 0
		item.InFlight = nil
		item.Error = ""
		if item.CurrentVersion != nil {
			item.Status = "approved"
		} else {
			item.Status = "review"
		}
		l.manifest.Revision++
		if err = s.Edits.CommitImageRedraw(ctx, l.snapshot, l.manifest, []string{item.ID}); err != nil {
			return CancelVersionResult{}, err
		}
		result.Cancelled = true
		result.CostImages = 0
		return result, nil
	}
	if item.Status != "pending" || !inQueue(l.manifest.QueueOnly, item.ID) {
		return CancelVersionResult{}, cloneError(domain.CloneErrConflict, "Không có lần vẽ nào đang chạy cho ảnh này")
	}
	// Chưa chạy: bỏ khỏi hàng chờ và trả ảnh về trạng thái trước khi vẽ lại.
	status := "approved"
	if item.CurrentVersion == nil {
		status = "review"
	}
	if item.Draft != nil && item.Draft.BaseStatus != "" {
		status = item.Draft.BaseStatus
	}
	item.Status = status
	item.Draft = nil
	item.Error = ""
	queue := make([]string, 0, len(l.manifest.QueueOnly))
	for _, queued := range l.manifest.QueueOnly {
		if queued != item.ID {
			queue = append(queue, queued)
		}
	}
	l.manifest.QueueOnly = queue
	l.manifest.Revision++
	if err = s.Edits.CommitImageRedraw(ctx, l.snapshot, l.manifest, []string{item.ID}); err != nil {
		return CancelVersionResult{}, err
	}
	result.Cancelled = true
	result.CostImages = 0
	return result, nil
}

func inQueue(queue []string, id string) bool {
	for _, queued := range queue {
		if queued == id {
			return true
		}
	}
	return false
}

// RebindRequest là yêu cầu thay media của một clip trong project đã dựng.
type RebindRequest struct {
	JobID        string
	ImageID      string
	SceneID      string
	Kind         string
	Version      int
	Path         string
	SHA256       string
	Mime         string
	Width        int
	Height       int
	Cutout       *scenes.FileRef
	ApplyProject bool
}

// RebindResult là thay đổi Editor cần áp khi project=false, và là kết quả thật
// khi máy chủ tự thay.
type RebindResult struct {
	ProjectID       string
	ClipID          string
	MediaID         string
	Src             string
	PreviousMediaID string
	Quality         string
}

// Binder thay media của clip trong project đã dựng và đo lại Độ hoàn thiện.
type Binder interface {
	Rebind(context.Context, RebindRequest) (*RebindResult, error)
}

// projectClipID là id clip của một cảnh trong project dựng từ video này.
func projectClipID(projectID, sceneID string) string {
	return projectID + "-scene-" + sceneID
}

// editorProjectID là id project editor của một video.
func editorProjectID(jobID string, generation int) string {
	return fmt.Sprintf("clone-%s-g%d", jobID, generation)
}

// measureTimeout giới hạn lần đo lại Độ hoàn thiện sau khi thay ảnh.
const measureTimeout = 30 * time.Second

var _ = errors.Is
var _ = editor.Object{}
var _ = jobedit.Snapshot{}
