package images

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// DrawPlan là đúng những gì một lần vẽ sẽ gửi: các phần prompt, ảnh tham chiếu
// đã chép vào staging, góp ý và nguồn vẽ.
type DrawPlan struct {
	Parts        PromptParts
	Wrapper      PromptWrapper
	SentText     string
	Refs         []ImageRef
	RefFiles     []string
	RefArtifacts []media.Artifact
	Feedback     *string
	Provider     string
	PromptVer    int
}

// CancelRegistry giữ huỷ của những lần vẽ đang chạy trong tiến trình này.
// Web gọi Cancel; handler kiểm trước khi gửi và huỷ ctx khi đang vẽ.
type CancelRegistry struct {
	mu        sync.Mutex
	cancels   map[string]context.CancelFunc
	cancelled map[string]bool
}

// NewCancelRegistry dựng sổ huỷ rỗng.
func NewCancelRegistry() *CancelRegistry {
	return &CancelRegistry{cancels: map[string]context.CancelFunc{}, cancelled: map[string]bool{}}
}

// DrawKey là khoá huỷ của một lần vẽ.
func DrawKey(jobID, imageID string, n int) string {
	return fmt.Sprintf("%s:%s:%d", jobID, imageID, n)
}

// Register gắn ctx huỷ được cho một lần vẽ và trả ctx con. done phải gọi khi
// lần vẽ kết thúc.
func (r *CancelRegistry) Register(key string, ctx context.Context) (context.Context, func()) {
	if r == nil {
		return ctx, func() {}
	}
	child, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.cancels[key] = cancel
	already := r.cancelled[key]
	r.mu.Unlock()
	if already {
		cancel()
	}
	return child, func() {
		r.mu.Lock()
		delete(r.cancels, key)
		r.mu.Unlock()
		cancel()
	}
}

// Cancel huỷ lần vẽ đang chạy; chưa chạy thì ghi ý định để handler bỏ qua trước
// khi gửi nhà cung cấp. Trả true khi có lần vẽ đang chạy.
func (r *CancelRegistry) Cancel(key string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancelled[key] = true
	if cancel, ok := r.cancels[key]; ok {
		cancel()
		return true
	}
	return false
}

// Cancelled báo lần vẽ đã bị huỷ trước khi gửi.
func (r *CancelRegistry) Cancelled(key string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancelled[key]
}

// Clear quên một khoá huỷ sau khi lần vẽ đã kết thúc.
func (r *CancelRegistry) Clear(key string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.cancelled, key)
	r.mu.Unlock()
}

// plan dựng prompt và ảnh tham chiếu cho lần vẽ kế tiếp của một ảnh: phần
// prompt của bản đang dùng (nếu có) + phần chủ kênh sửa tay.
func (h Handler) plan(ctx context.Context, s *pipeline.StepContext, m *Manifest, item Item) (DrawPlan, error) {
	if m.Story == nil {
		return DrawPlan{}, fmt.Errorf("image story analysis is missing")
	}
	story := *m.Story
	if item.Kind == "scene" && m.ImagePromptMode == "custom" {
		story.Look = ""
	}
	plan := DrawPlan{PromptVer: item.PromptVersion}
	provider := ""
	if item.Draft != nil {
		provider = item.Draft.Provider
	}
	plan.Provider = h.promptProvider(ctx, s.Job, provider)
	limit := h.refLimit(ctx, s.Job, provider)
	sourceVersion, err := PromptSourceVersion(item)
	if err != nil {
		return DrawPlan{}, err
	}
	requested := h.requestedRefs(m, item, sourceVersion)
	if sourceVersion != nil && len(requested) > limit {
		return DrawPlan{}, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: fmt.Sprintf("Nguồn vẽ này chỉ nhận tối đa %d ảnh tham chiếu", limit)}
	}
	plan.Refs, plan.RefFiles, plan.RefArtifacts, err = h.stageRefs(ctx, s, m, item, requested, limit)
	if err != nil {
		return DrawPlan{}, err
	}
	direction := ""
	if item.Draft != nil {
		direction = strings.TrimSpace(item.Draft.Feedback)
	}
	if direction == "" {
		direction = strings.TrimSpace(item.Direction)
	}
	if direction != "" {
		plan.Feedback = &direction
	}
	source := PromptPartSource{Type: "artist", Label: "Hoạ sĩ viết từ lời kể"}
	if plan.Feedback != nil {
		source = PromptPartSource{Type: "artist_feedback", Label: "Hoạ sĩ sửa theo góp ý"}
	}
	template := ItemPrompt(*m, item)
	templateSource := h.promptSource(ctx, s.Job, m.Overrides, item.Kind)

	parts, wrapper, err := BuildPrompt(PromptInput{Story: story, Item: item, Style: m.Style, Thumbnail: m.Thumbnail, Template: template, TemplateSource: templateSource, SceneSource: source, Refs: plan.Refs, RefsSource: refsSource(item), AttachedRefs: len(plan.RefFiles), Provider: plan.Provider})
	if err != nil {
		return DrawPlan{}, err
	}
	plan.Wrapper = wrapper
	if sourceVersion != nil {
		// Chủ kênh đang xem bản khác bản đang dùng: giữ nguyên các phần prompt
		// của đúng bản đó (kể cả phần do Hoạ sĩ viết) và chỉ thay phần được
		// sửa; phần Tham chiếu dựng lại từ ảnh tham chiếu thật sự gửi kèm.
		// Mượn slice bất biến; Apply/refsPartFrom tạo kết quả riêng, không
		// sửa lịch sử và không chép baseline hai lần.
		plan.Parts = sourceVersion.Parts
		if len(plan.Parts) == 0 {
			plan.Parts = VersionParts(*sourceVersion)
		}
		for index, part := range plan.Parts {
			if part.Kind == PartScene {
				if text := StripDirection(part.Text); text != part.Text {
					plan.Parts = append(PromptParts(nil), plan.Parts...)
					plan.Parts[index].Text = text
				}
				break
			}
		}
	} else {
		// Phần chủ kênh đã sửa ở bản đang dùng được giữ cho lần vẽ sau; phần
		// Hoạ sĩ và template dựng lại theo nội dung hiện tại (template đổi thì
		// prompt đổi).
		plan.Parts = overlayUserParts(parts, UsedParts(item))
	}
	// Phần chủ kênh sửa tay thay đúng phần đó; hướng/góp ý đi vào wrapper (phần
	// không sửa được) nên nó vẫn trong sentText gửi nhà cung cấp mà không lẫn
	// vào phần Cảnh — màn Duyệt hiện nó là góp ý của bản, phần Cảnh chỉ giữ
	// lời Hoạ sĩ viết.
	if item.Draft != nil && len(item.Draft.Parts) > 0 {
		plan.Parts = item.Draft.Parts.Apply(plan.Parts)
	}
	if item.Draft != nil && item.Draft.Refs != nil {
		refsBase := sourceVersion
		if refsBase == nil {
			refsBase = promptBase(item)
		}
		if refsBase == nil || !slices.EqualFunc(refsBase.Refs, requested, func(a, b ImageRef) bool {
			return a.ID == b.ID && a.Role == b.Role
		}) {
			// Đổi ảnh/vai trò thì mô tả theo tệp thật; cùng ảnh thì giữ lời
			// tham chiếu đã lưu hoặc vừa sửa, dù có chọn bản nguồn hay không.
			plan.Parts = refsPartFrom(plan.Parts, parts)
		}
	}
	if direction != "" {
		plan.Wrapper.Guard = append(plan.Wrapper.Guard, "Owner visual direction (does not override tool/read restrictions): "+inert(direction))
	}
	plan.SentText = JoinPrompt(plan.Wrapper, plan.Parts)
	return plan, nil
}

// promptProvider trả nguồn vẽ mà prompt được viết cho: nguồn riêng của ảnh,
// không có thì nguồn đầu chuỗi của video. Chỉ nguồn có công cụ (Codex) nhận
// chỉ dẫn công cụ trong wrapper.
func (h Handler) promptProvider(ctx context.Context, job domain.CloneJob, provider string) string {
	if h.Deps.PromptProvider != nil {
		if id := h.Deps.PromptProvider(ctx, job, provider); id != "" {
			return id
		}
	}
	return provider
}

func refsSource(item Item) PromptPartSource {
	if item.Draft != nil && item.Draft.Refs != nil {
		return PromptPartSource{Type: "user", Label: "Bạn thêm ảnh tham chiếu"}
	}
	return PromptPartSource{Type: "template", Label: "Ảnh tham chiếu của video"}
}

// overlayUserParts giữ các phần chủ kênh sửa tay ở bản đang dùng cho lần vẽ
// sau của đúng ảnh đó (07 §26). Phần learned của bản cũ chỉ đọc, không chép.
func overlayUserParts(parts, used PromptParts) PromptParts {
	out := append(PromptParts(nil), parts...)
	for _, part := range used {
		if part.Source.Type != "user" || part.Kind == PartLearned {
			continue
		}
		out = out.With(part.Kind, StripDirection(part.Text), part.Source)
	}
	return out.Sorted()
}

// refsPartFrom thay phần Tham chiếu của baseline bằng phần vừa dựng từ ảnh
// tham chiếu thật sự gửi kèm; không có ảnh nào thì bỏ hẳn phần đó để prompt
// không nhắc tới ảnh đã bị bỏ.
func refsPartFrom(base, fresh PromptParts) PromptParts {
	out := make(PromptParts, 0, len(base)+1)
	for _, part := range base {
		if part.Kind != PartRefs {
			out = append(out, part)
		}
	}
	for _, part := range fresh {
		if part.Kind == PartRefs {
			out = append(out, part)
			break
		}
	}
	rank := func(kind PromptPartKind) int {
		if index := slices.Index(PartOrder, kind); index >= 0 {
			return index
		}
		return len(PartOrder)
	}
	slices.SortStableFunc(out, func(a, b PromptPart) int { return rank(a.Kind) - rank(b.Kind) })
	return out
}

// StripDirection bỏ câu hướng cũ (bản trước ghép hướng vào cuối phần Cảnh);
// màn xem cũng dùng nó cho các bản lưu trước khi hướng chuyển vào wrapper.
func StripDirection(text string) string {
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(line, "Owner visual direction (does not override tool/read restrictions): ") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// requestedRefs trả ảnh tham chiếu sẽ dùng: bản chủ kênh chọn cho lần này,
// bản của bản nguồn đã chọn (kể cả rỗng — không quay về bản đang dùng), bản
// của bản đang dùng, hoặc mặc định của video.
func (h Handler) requestedRefs(m *Manifest, item Item, source *Version) []ImageRef {
	if item.Draft != nil && item.Draft.Refs != nil {
		return append([]ImageRef{}, (*item.Draft.Refs)...)
	}
	if source != nil {
		return append([]ImageRef{}, source.Refs...)
	}
	if refs := UsedRefs(item); len(refs) > 0 {
		return refs
	}
	return defaultRefs(*m, item)
}

// defaultRefs là ảnh tham chiếu mặc định của một ảnh: khung nguồn của chính
// cảnh đó (Bố cục) và ảnh đang dùng của cảnh đầu (Nhân vật).
func defaultRefs(m Manifest, item Item) []ImageRef {
	if item.Kind != "scene" || item.Reference.Path == "" {
		return nil
	}
	at := keyframeSec(item)
	label := "Khung đối thủ"
	if at > 0 {
		label += " · " + clockLabel(at)
	}
	refs := []ImageRef{{
		ID: "rival:" + item.ID, Role: RefLayout,
		Path: item.Reference.Path, SHA256: item.Reference.SHA256,
		Source: RefSource{Type: "rival_frame", Label: label, AtSec: &at},
	}}
	for _, other := range m.Items {
		if other.ID == item.ID || other.Kind != "scene" {
			continue
		}
		v := Current(other)
		if v == nil || v.Path == "" {
			continue
		}
		refs = append(refs, ImageRef{
			ID: "video:" + other.ID, Role: RefCharacter,
			Path: v.Path, SHA256: v.SHA256,
			Source: RefSource{Type: "video_image", Label: other.ID + " · từ cảnh đầu", ImageID: other.ID},
		})
		break
	}
	return refs
}

func keyframeSec(item Item) float64 {
	if item.KeyframeSec > 0 {
		return item.KeyframeSec
	}
	return float64(item.StartFrame) / scenes.DefaultFPS
}

func clockLabel(sec float64) string {
	total := int(sec + .5)
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

// refLimit trả số ảnh tham chiếu nguồn vẽ nhận (0 = không nhận ảnh).
func (h Handler) refLimit(ctx context.Context, job domain.CloneJob, provider string) int {
	if h.Deps.RefLimit == nil {
		return imagegen.RefLimit("")
	}
	return h.Deps.RefLimit(ctx, job, provider)
}

// promptSource trả nguồn của phần Phong cách: prompt thư viện hay template.
func (h Handler) promptSource(ctx context.Context, job domain.CloneJob, overrides *Overrides, kind string) PromptPartSource {
	if h.Deps.PromptSource != nil {
		if source := h.Deps.PromptSource(ctx, job, overrides, kind); source.Type != "" {
			return source
		}
	}
	if kind == "thumbnail" {
		return PromptPartSource{Type: "template", Label: "Prompt ảnh bìa"}
	}
	return PromptPartSource{Type: "image_prompt", Label: "Prompt ảnh cảnh"}
}

// stageRefs chép các ảnh tham chiếu sẽ gửi vào staging của step và trả danh
// sách đúng giới hạn nguồn vẽ. Không có ảnh nào đọc được thì bỏ ảnh đó chứ
// không gửi một đường dẫn rỗng.
func (h Handler) stageRefs(ctx context.Context, s *pipeline.StepContext, m *Manifest, item Item, refs []ImageRef, limit int) ([]ImageRef, []string, []media.Artifact, error) {
	if limit <= 0 || len(refs) == 0 {
		return nil, nil, nil, nil
	}
	if len(refs) > limit {
		refs = refs[:limit]
	}
	dir := filepath.Join(s.StagingDir, "refs", item.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, nil, err
	}
	kept := make([]ImageRef, 0, len(refs))
	files := make([]string, 0, len(refs))
	var artifacts []media.Artifact
	for index, ref := range refs {
		file, err := h.refFile(ctx, s, m, item, ref)
		if err != nil {
			return nil, nil, nil, err
		}
		if file == "" {
			continue
		}
		ext := filepath.Ext(file)
		if ext == "" {
			ext = ".png"
		}
		staged := filepath.Join(dir, fmt.Sprintf("%d%s", index+1, ext))
		if err := steps.CopyFile(file, staged); err != nil {
			return nil, nil, nil, err
		}
		if strings.HasPrefix(ref.Path, "images/references/") {
			hash, err := steps.SHA256File(staged)
			if err != nil {
				return nil, nil, nil, err
			}
			if hash != ref.SHA256 {
				return nil, nil, nil, steps.CloneError(domain.CloneErrStale, "Ảnh tham chiếu đã thay đổi", "Chọn lại ảnh tham chiếu")
			}
		}
		if ref.Path == "" {
			hash, err := steps.SHA256File(staged)
			if err != nil {
				return nil, nil, nil, err
			}
			ref.Path = "images/references/" + hash + ext
			ref.SHA256 = hash
			f, err := os.Open(staged)
			if err != nil {
				return nil, nil, nil, err
			}
			var header [512]byte
			n, readErr := f.Read(header[:])
			closeErr := f.Close()
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return nil, nil, nil, readErr
			}
			if closeErr != nil {
				return nil, nil, nil, closeErr
			}
			artifacts = append(artifacts, media.Artifact{Path: ref.Path, File: staged, ContentType: http.DetectContentType(header[:n])})
		}
		kept = append(kept, ref)
		files = append(files, staged)
	}
	return kept, files, artifacts, nil
}

// refFile trả tệp cục bộ của một ảnh tham chiếu.
func (h Handler) refFile(ctx context.Context, s *pipeline.StepContext, m *Manifest, item Item, ref ImageRef) (string, error) {
	// A persisted reference is the immutable input of this draw, not a request
	// to resolve the upload or the current video image again.
	if ref.Path != "" {
		if ref.Path == item.Reference.Path {
			if staged := stagedReference(s.StagingDir, item); staged != "" {
				return staged, nil
			}
		}
		return h.copyArtifact(ctx, s, ref.Path, item.ID)
	}
	switch ref.Source.Type {
	case "rival_frame":
		if ref.Source.AtSec == nil {
			return "", nil
		}
		return h.extractFrame(ctx, s, *ref.Source.AtSec, item.ID)
	case "video_image":
		for _, other := range m.Items {
			if other.ID != ref.Source.ImageID {
				continue
			}
			v := Current(other)
			if v == nil || v.Path == "" {
				return "", nil
			}
			return h.copyArtifact(ctx, s, v.Path, item.ID)
		}
		return "", nil
	case "upload":
		if h.Deps.AcquireUpload == nil || ref.Source.ImageID == "" {
			return "", nil
		}
		file, release, err := h.Deps.AcquireUpload(ctx, ref.Source.ImageID)
		if err != nil {
			return "", err
		}
		defer release()
		staged := filepath.Join(s.StagingDir, "refs", "uploads", item.ID+filepath.Ext(file))
		if err := steps.CopyFile(file, staged); err != nil {
			return "", err
		}
		return staged, nil
	}
	return "", nil
}

// copyArtifact chép một artifact đã công bố vào staging của step.
func (h Handler) copyArtifact(ctx context.Context, s *pipeline.StepContext, path, itemID string) (string, error) {
	handle, err := s.Acquire(ctx, path)
	if err != nil {
		return "", err
	}
	defer handle.Release()
	staged := filepath.Join(s.StagingDir, "refs", "cache", itemID, strings.ReplaceAll(strings.TrimSuffix(path, filepath.Ext(path)), "/", "_")+filepath.Ext(path))
	if err := steps.CopyFile(handle.Path, staged); err != nil {
		return "", err
	}
	return staged, nil
}

// extractFrame trích một khung của video nguồn tại giây yêu cầu.
func (h Handler) extractFrame(ctx context.Context, s *pipeline.StepContext, atSec float64, itemID string) (string, error) {
	if h.Deps.Ffmpeg == "" {
		return "", nil
	}
	source, err := s.Acquire(ctx, "download/source.mp4")
	if err != nil {
		return "", err
	}
	defer source.Release()
	out := filepath.Join(s.StagingDir, "refs", "frames", fmt.Sprintf("%s-%.2f.png", itemID, atSec))
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return "", err
	}
	args := []string{"-v", "error", "-ss", fmt.Sprintf("%.3f", atSec), "-i", source.Path, "-frames:v", "1", "-y", out}
	if _, err := s.RunProcess(ctx, h.Deps.Ffmpeg, args, tools.Spec{Timeout: 2 * time.Minute, LogLimit: 4096}); err != nil {
		return "", err
	}
	return out, nil
}

// drawer trả nguồn vẽ cho lần này: nguồn riêng của ảnh nếu chủ kênh chọn.
func (h Handler) drawer(ctx context.Context, job domain.CloneJob, provider string) (Drawer, error) {
	if provider != "" && h.Deps.DrawerFor != nil {
		drawer, err := h.Deps.DrawerFor(ctx, job, provider)
		if err != nil {
			return nil, err
		}
		if drawer != nil {
			return drawer, nil
		}
	}
	if h.Deps.Drawer == nil {
		return nil, steps.CloneError(domain.CloneErrToolMissing, "Thiếu nguồn vẽ ảnh", "Kiểm tra Kết nối AI trong Cài đặt")
	}
	return h.Deps.Drawer, nil
}

// versionError xếp lỗi vẽ thành lý do thật cho giao diện (07 §26).
func versionError(err error, now time.Time) *VersionError {
	var drawErr *imagegen.DrawError
	var capErr *imagegen.CapError
	reason := "provider"
	var retryAt *string
	switch {
	case errors.As(err, &capErr):
		reason = "quota"
		reset := capErr.RetryAt
		if reset.IsZero() {
			reset = nextMidnight(now)
		}
		value := reset.Format(time.RFC3339)
		retryAt = &value
	case errors.As(err, &drawErr):
		switch drawErr.Code {
		case imagegen.ErrCodeQuota:
			reason = "quota"
			reset := drawErr.RetryAt
			if reset == nil {
				next := nextMidnight(now)
				reset = &next
			}
			value := reset.Format(time.RFC3339)
			retryAt = &value
		case imagegen.ErrCodeCancelled:
			reason = "network"
		case imagegen.ErrCodeConflict, imagegen.ErrCodeUncertain:
			reason = "provider"
		}
	}
	if imagegen.IsImageRefused(err) {
		reason = "filter"
	}
	if imagegen.IsProviderBusy(err) {
		reason = "provider"
	}
	return &VersionError{Reason: reason, RetryAt: retryAt, Message: err.Error()}
}

func nextMidnight(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}

// publishPhase phát một pha của lần vẽ lên luồng sự kiện của app.
func publishPhase(s *pipeline.StepContext, imageID string, n int, phase string, started time.Time) {
	if s.PublishEvent == nil {
		return
	}
	s.PublishEvent("image.progress", map[string]any{
		"jobId": s.Job.ID, "imageId": imageID, "n": n, "phase": phase,
		"elapsedSec": int(time.Since(started).Seconds()),
	})
}

// usedVersionFor báo bản mới có thay bản đang dùng không: chỉ thay khi người
// dùng chưa chọn bản khác trong lúc vẽ (07 §26).
func usedVersionFor(item Item, flight Flight) bool {
	return flight.BaseVersion == nil && item.CurrentVersion == nil ||
		flight.BaseVersion != nil && item.CurrentVersion != nil && *flight.BaseVersion == *item.CurrentVersion
}

// setVersion ghi một phiên bản vào ảnh: bản cùng số (lần thử lại của cùng một
// chuyến vẽ) được thay, không nhân đôi.
func setVersion(item *Item, version Version) {
	for index := range item.Versions {
		if item.Versions[index].Version == version.Version {
			item.Versions[index] = version
			return
		}
	}
	item.Versions = append(item.Versions, version)
}

// versionArtifactPath là đường dẫn tương đối của tệp một phiên bản.
func versionArtifactPath(itemID string, version int, generation string) string {
	return fmt.Sprintf("images/versions/%s/%d-%s.webp", itemID, version, generation)
}

// RebindRequest là yêu cầu thay media của một clip trong project đã dựng bằng
// đúng ảnh của một phiên bản mới.
type RebindRequest struct {
	ApplyProject bool
	JobID        string
	ImageID      string
	Kind         string
	SceneID      string
	Version      int
	Path         string
	SHA256       string
	Mime         string
	Width        int
	Height       int
	Cutout       *scenes.FileRef
}

// flightProvider trả nguồn vẽ riêng của ảnh ("" = chuỗi chung của video).
func flightProvider(item Item) string {
	if item.Draft == nil {
		return ""
	}
	return item.Draft.Provider
}

// restoredStatus trả trạng thái ảnh quay về khi lần vẽ bị huỷ hoặc lỗi không
// để lại bản mới.
func restoredStatus(flight Flight) string {
	switch flight.BaseStatus {
	case "", "running", "pending":
		if flight.BaseVersion != nil {
			return "approved"
		}
		return "pending"
	}
	return flight.BaseStatus
}

// recordCancelled ghi phiên bản bị huỷ trước khi gửi: không tốn ảnh nào.
func recordCancelled(item *Item, flight Flight, started time.Time, now func() time.Time) {
	finished := now()
	setVersion(item, Version{
		Project: flight.Project,
		Version: flight.Version, GenerationID: flight.GenerationID, CreatedAt: finished.UTC().Format(time.RFC3339Nano),
		Prompt: flight.Prompt, PromptVersion: flight.PromptVersion, SourceFingerprint: flight.SourceFingerprint,
		StateName: VersionCancelled, Parts: flight.Parts, Wrapper: flight.Wrapper, Refs: flight.Refs, Feedback: flight.Feedback,
		StartedAt: flight.StartedAt, FinishedAt: new(finished.UTC().Format(time.RFC3339Nano)),
		DurationSec: new(finished.Sub(started).Seconds()), CostImages: 0,
	})
}

// recordDrawing ghi dòng phiên bản `drawing` ngay khi chuyến vẽ bắt đầu: dải
// ảnh, Phòng vẽ và panel cảnh hiện "Đang vẽ bản N" và nút Huỷ lần vẽ có đúng số
// bản để huỷ (07 §26). Dòng này bị thay bằng ready/failed/cancelled khi xong —
// lần thử lại của cùng chuyến vẽ dùng lại đúng số bản, không nhân đôi.
func recordDrawing(item *Item, flight Flight) {
	setVersion(item, Version{
		Project: flight.Project,
		Version: flight.Version, GenerationID: flight.GenerationID, CreatedAt: flight.StartedAt,
		Prompt: flight.Prompt, PromptVersion: flight.PromptVersion, SourceFingerprint: flight.SourceFingerprint,
		StateName: VersionDrawing, Parts: flight.Parts, Wrapper: flight.Wrapper, Refs: flight.Refs,
		Provider: flight.Provider, Params: &PromptParams{Size: fmt.Sprintf("%dx%d", item.Width, item.Height)},
		Feedback: flight.Feedback, StartedAt: flight.StartedAt, CostImages: 0,
	})
}

// recoverDrawings dọn dòng phiên bản `drawing` mồ côi: chuyến vẽ không còn con
// trỏ InFlight (tiến trình chết hẳn, job đã dừng) thì không ai vẽ tiếp được nên
// bản đó thành failed — giao diện không treo ở "đang vẽ". Chuyến vẽ còn sống
// (InFlight đúng số bản) được chạy tiếp và dòng đó sẽ được thay khi xong.
func recoverDrawings(m *Manifest, now time.Time) bool {
	changed := false
	for index := range m.Items {
		item := &m.Items[index]
		for at := range item.Versions {
			v := &item.Versions[at]
			if v.State() != VersionDrawing {
				continue
			}
			if item.InFlight != nil && item.InFlight.Version == v.Version {
				continue
			}
			finished := now.UTC().Format(time.RFC3339Nano)
			v.StateName = VersionFailed
			v.FinishedAt = &finished
			if started, err := time.Parse(time.RFC3339Nano, v.StartedAt); err == nil {
				secs := now.Sub(started).Seconds()
				v.DurationSec = &secs
			}
			v.VersionError = &VersionError{Reason: "provider", Message: "Lần vẽ bị gián đoạn khi ứng dụng khởi động lại. Bản trước vẫn được giữ."}
			if item.Status == "running" {
				if item.CurrentVersion != nil {
					item.Status = "approved"
				} else {
					item.Status = "failed"
				}
				item.Error = v.VersionError.Message
			}
			changed = true
		}
	}
	return changed
}

// versionFailure ghi phiên bản lỗi: giữ nguyên phần prompt và ảnh tham chiếu đã
// gửi để "Sửa prompt" mở lại đúng nội dung.
func versionFailure(flight Flight, receipt imagegen.Receipt, err error, started time.Time, now func() time.Time, cancelled, sent bool) Version {
	finished := now()
	state := VersionFailed
	cost := 1
	if cancelled {
		state = VersionCancelled
	}
	var capErr *imagegen.CapError
	if !sent || imagegen.IsUnpaid(err) || errors.As(err, &capErr) {
		// Nhà cung cấp chưa nhận yêu cầu (hoặc trần ngày của worker) thì không
		// tốn ảnh — huỷ đúng lúc gửi cũng vậy.
		cost = 0
	}
	version := Version{
		Project: flight.Project,
		Version: flight.Version, GenerationID: flight.GenerationID, CreatedAt: finished.UTC().Format(time.RFC3339Nano),
		Prompt: flight.Prompt, PromptVersion: flight.PromptVersion, SourceFingerprint: flight.SourceFingerprint,
		StateName: state, Parts: flight.Parts, Wrapper: flight.Wrapper, Refs: flight.Refs, Feedback: flight.Feedback,
		Provider: receipt.Provider, Model: receipt.Model, StartedAt: flight.StartedAt,
		FinishedAt: new(finished.UTC().Format(time.RFC3339Nano)), DurationSec: new(finished.Sub(started).Seconds()),
		CostImages: cost, VersionError: versionError(err, finished),
	}
	return version
}

// sceneDiff so phần Cảnh với bản gần nhất trước đó (07 §26: chỉ phần scene).
func sceneDiff(item Item, n int, scene string) [][2]string {
	previous := ""
	best := 0
	for _, v := range item.Versions {
		if v.Version >= n || v.Version < best {
			continue
		}
		text := v.Parts.Get(PartScene)
		if text == "" && v.Prompt != "" {
			text = v.Prompt
		}
		if text == "" {
			continue
		}
		best = v.Version
		previous = text
	}
	if previous == "" || scene == "" {
		return nil
	}
	return DiffText(previous, scene)
}

func ptrString(value string) *string  { return &value }
func ptrFloat(value float64) *float64 { return &value }
