// Package library là mảng Thư viện: template (recipe + revision), prompt ảnh
// (7 prompt có sẵn + prompt của bạn) và lớp phủ dùng chung. Mọi ghi đi qua
// Service; store là mặt lưu trữ (postgres.LibraryStore), web chỉ là hình wire.
//
// Bất biến quan trọng:
//   - revision tăng mỗi lần lưu recipe; ghi kèm revision cũ bị từ chối (409).
//   - recipe kiểm bằng đúng một đường (ParseRecipe → ValidateRecipe →
//     CheckRefs) cho API và CLI.
//   - xoá template bị chặn khi còn kênh dùng; xoá lớp phủ là tombstone để
//     template đang dùng vẫn giữ tệp.
package library

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// Chủ đề sự kiện SSE của mảng Thư viện.
const (
	TopicTemplate = "library.templates"
	TopicPrompt   = "library.prompts"
	TopicOverlay  = "library.overlays"
)

// MaxOverlayBytes là trần tệp lớp phủ tải lên (256 MiB) — cùng bậc với kho
// media của yt-factory, đủ cho một vòng hiệu ứng 1080p dài.
const MaxOverlayBytes = 256 << 20

// TemplateRef là một template (id + tên) dùng cho danh sách "đang dùng".
type TemplateRef struct {
	ID   string
	Name string
}

// Store là mặt lưu trữ Service cần — hiện thực là postgres.LibraryStore.
type Store interface {
	RefChecker
	ListTemplates(ctx context.Context) ([]domain.TemplateSummary, error)
	GetTemplate(ctx context.Context, id string) (*domain.Template, error)
	InsertTemplate(ctx context.Context, t domain.Template) error
	// UpdateTemplate ghi recipe + tên với điều kiện revision khớp; trả revision
	// mới và ok=false khi revision đã cũ (hoặc hàng đã mất).
	UpdateTemplate(ctx context.Context, id string, name *string, recipe domain.Recipe, expectRevision int64, project json.RawMessage) (revision int64, ok bool, err error)
	// InitializeTemplateProject only fills a missing project at this revision.
	// Concurrent backfills never replace an authored or already generated project.
	InitializeTemplateProject(ctx context.Context, id string, project json.RawMessage, expectRevision int64) (bool, error)
	// RenameTemplate đổi tên, không đổi revision (tên không nằm trong bản xem
	// thử); expectRevision nil = không kiểm.
	RenameTemplate(ctx context.Context, id, name string, expectRevision *int64) (ok bool, err error)
	// NameChannelStandardTemplates đổi tên trần plain → name cho template chỉ
	// kênh channelID dùng và còn đúng tên plain; trả id đã đổi.
	NameChannelStandardTemplates(ctx context.Context, channelID, plain, name string) ([]string, error)
	DeleteTemplate(ctx context.Context, id string) error
	TemplateChannels(ctx context.Context, templateID string) (domain.ChannelRefList, error)
	TemplatesUsingPrompt(ctx context.Context, promptID string) ([]TemplateRef, error)

	ListPrompts(ctx context.Context, kind domain.PromptKind, q string) ([]domain.ImagePrompt, error)
	GetPrompt(ctx context.Context, id string) (*domain.ImagePrompt, error)
	NameTaken(ctx context.Context, name, exceptID string) (bool, error)
	InsertPrompt(ctx context.Context, p domain.ImagePrompt) error
	UpdatePrompt(ctx context.Context, p domain.ImagePrompt) error
	DeletePrompt(ctx context.Context, id string) error

	ListOverlays(ctx context.Context) ([]domain.Overlay, error)
	OverlayUsage(ctx context.Context) (map[string][]TemplateRef, error)
	OverlayByID(ctx context.Context, id string) (*domain.Overlay, error)
	// OverlayBySHA tìm theo nội dung, kể cả hàng đã bỏ khỏi thư viện;
	// overlay nil = chưa từng có, removed = dòng đã bị bỏ khỏi thư viện.
	OverlayBySHA(ctx context.Context, sha string) (overlay *domain.Overlay, removed bool, err error)
	InsertOverlay(ctx context.Context, o domain.Overlay) error
	RestoreOverlay(ctx context.Context, id, name string) error
	RemoveOverlay(ctx context.Context, id string) error
	// ExistingBuiltins trả id hiệu ứng có sẵn đã từng có dòng (kể cả đã bỏ).
	ExistingBuiltins(ctx context.Context) (map[string]bool, error)
}

// Files là mặt thư mục dự án — hiện thực là storage.Service.
type Files interface {
	Root() string
	Resolve(rel string) (string, error)
}

// PriceOf đọc giá một ảnh của model vẽ chính (optional: ok=false khi chưa biết).
type PriceOf interface {
	PerImageUSD(ctx context.Context) (usd float64, ok bool, err error)
}

// Deps là phụ thuộc của Service; trường optional để test chỉ cần phần dùng.
type Deps struct {
	Store  Store
	Files  Files
	Media  *media.Service
	Runner Runner
	Bins   Binaries
	Price  PriceOf
	// Importer nhập media của template cũ; nil = chỉ đọc/ghi metadata (test),
	// nhập template báo 400 thay vì tạo template trỏ tới media không có.
	Importer MediaImporter
	// Notify phát sự kiện SSE (chủ đề, ref). nil = không phát (CLI/test).
	Notify func(topic, ref string)
	Now    func() time.Time
	NewID  func() string
}

// Service là nghiệp vụ Thư viện. An toàn cho goroutine (không giữ trạng thái).
type Service struct {
	store    Store
	files    Files
	media    *media.Service
	runner   Runner
	bins     Binaries
	price    PriceOf
	importer MediaImporter
	notify   func(topic, ref string)
	now      func() time.Time
	newID    func() string
}

// New dựng Service trên store.
func New(deps Deps) *Service {
	s := &Service{
		store: deps.Store, files: deps.Files, runner: deps.Runner, bins: deps.Bins,
		media: deps.Media, importer: deps.Importer,
		price: deps.Price, notify: deps.Notify, now: deps.Now, newID: deps.NewID,
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.newID == nil {
		s.newID = newUUID
	}
	return s
}

// Media returns the shared backend; nil is valid for metadata-only services.
func (s *Service) Media() *media.Service { return s.media }

// newUUID sinh uuid v4 (không thêm phụ thuộc ngoài).
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// emit phát sự kiện nếu có notifier (không chặn).
func (s *Service) emit(topic, ref string) {
	if s.notify != nil {
		s.notify(topic, ref)
	}
}

// refChecker ghép prompt có sẵn (hằng số) với mặt đọc của store để kiểm
// tham chiếu recipe.
type refChecker struct{ store Store }

func (c refChecker) PromptExists(ctx context.Context, id string, kind domain.PromptKind) (bool, error) {
	if p, ok := BuiltinPrompt(id); ok {
		return p.Kind == kind, nil
	}
	return c.store.PromptExists(ctx, id, kind)
}

func (c refChecker) OverlayExists(ctx context.Context, id string) (bool, error) {
	return c.store.OverlayExists(ctx, id)
}

func (c refChecker) BackdropChannels(ctx context.Context, ids []string) ([]string, error) {
	return c.store.BackdropChannels(ctx, ids)
}

func (c refChecker) BackdropVideos(ctx context.Context, channelID string, ids []string) ([]string, error) {
	return c.store.BackdropVideos(ctx, channelID, ids)
}

func (c refChecker) BackdropVideoExists(ctx context.Context, id string) (bool, error) {
	return c.store.BackdropVideoExists(ctx, id)
}

// ── giá ────────────────────────────────────────────────────────────────────

// ImageCountOf trả số ảnh cảnh mỗi video; ok=false với chế độ mỗi N phút
// (chỉ biết sau khi có thời lượng video nguồn).
func ImageCountOf(m domain.ImagesMode) (int, bool) {
	switch m.Mode {
	case ImagesFixed:
		return m.Count, true
	case ImagesNone:
		return 0, true
	default:
		return 0, false
	}
}

// ImageCountFor là số ảnh cảnh cho một thời lượng cụ thể (port
// recipeImageCount của yt-factory): none → 0, fixed → count, per-minutes →
// clamp(ceil(durationSec/60/minutes), 1, 60).
func ImageCountFor(m domain.ImagesMode, durationSec float64) int {
	switch m.Mode {
	case ImagesNone:
		return 0
	case ImagesFixed:
		return m.Count
	default:
		if m.Minutes <= 0 {
			return 1
		}
		count := int(math.Ceil(durationSec / 60 / float64(m.Minutes)))
		if float64(count) < domain.LimitFixedImagesMin {
			return int(domain.LimitFixedImagesMin)
		}
		if float64(count) > domain.LimitFixedImagesMax {
			return int(domain.LimitFixedImagesMax)
		}
		return count
	}
}

// PriceEstimate là ước lượng giá ảnh mỗi video của một template.
//
// Quyết định: dùng số ảnh của template (không dùng hằng 7 của design) — cộng
// một ảnh bìa. Chế độ "mỗi N phút một ảnh" không biết số ảnh khi chưa có
// video nên TotalUSD nil, UI ghi cách tính thay vì bịa giá.
type PriceEstimate struct {
	PerImageUSD    float64  `json:"perImageUSD"`
	Known          bool     `json:"known"`
	ImagesPerVideo *int     `json:"imagesPerVideo"`
	IncludesCover  bool     `json:"includesCover"`
	Mode           string   `json:"mode"`
	PerMinutes     int      `json:"perMinutes,omitempty"`
	TotalUSD       *float64 `json:"totalUSD"`
}

// Estimate tính giá theo recipe; lỗi đọc giá không làm hỏng GET (Known=false).
func (s *Service) Estimate(ctx context.Context, recipe domain.Recipe) PriceEstimate {
	out := PriceEstimate{Mode: recipe.Scenes.Images.Mode, PerMinutes: recipe.Scenes.Images.Minutes, IncludesCover: true}
	if s.price != nil {
		if usd, ok, err := s.price.PerImageUSD(ctx); err == nil && ok {
			out.PerImageUSD, out.Known = usd, true
		}
	}
	if count, ok := ImageCountOf(recipe.Scenes.Images); ok {
		out.ImagesPerVideo = &count
		if out.Known {
			total := float64(count+1) * out.PerImageUSD
			out.TotalUSD = &total
		}
	}
	return out
}

// ── template ───────────────────────────────────────────────────────────────

// Summary là một dòng của GET /api/templates: template + kênh dùng + tên
// prompt đã phân giải + số đếm mà thẻ Thư viện cần (ảnh, nền, lớp phủ).
type Summary struct {
	domain.TemplateSummary
	ScenePromptName  string
	CoverPromptName  string
	ImageCount       int
	ImageCountKnown  bool
	BackdropChannels int
	BackdropVideos   int
	OverlayCount     int
}

// Detail là GET /api/templates/:id — template + kênh dùng + ước lượng giá.
type Detail struct {
	Summary
	Price PriceEstimate
}

// List trả mọi template kèm kênh đang dùng và tên prompt.
func (s *Service) List(ctx context.Context) ([]Summary, error) {
	rows, err := s.store.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	names, err := s.promptNames(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Summary, 0, len(rows))
	for _, row := range rows {
		if len(row.Project) > 0 {
			row.Recipe, err = s.projectTemplateRecipe(ctx, row.Project, row.Recipe)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, summarize(row, names))
	}
	return out, nil
}

// summarize dựng hình tóm tắt từ một hàng template + bảng tên prompt.
func summarize(row domain.TemplateSummary, names map[string]string) Summary {
	summary := Summary{TemplateSummary: row}
	summary.ScenePromptName = names[row.Recipe.Scenes.ScenePromptID]
	summary.CoverPromptName = names[row.Recipe.Scenes.CoverPromptID]
	summary.ImageCount, summary.ImageCountKnown = ImageCountOf(row.Recipe.Scenes.Images)
	summary.OverlayCount = len(row.Recipe.Overlays)
	if backdrop := row.Recipe.Backdrop; backdrop != nil {
		summary.BackdropChannels = len(backdrop.Channels)
		summary.BackdropVideos = len(backdrop.FixedVideoIDs)
		for _, link := range backdrop.Channels {
			summary.BackdropVideos += len(link.Pinned)
		}
	}
	return summary
}

// promptNames trả bảng id → tên của mọi prompt (có sẵn + của bạn).
func (s *Service) promptNames(ctx context.Context) (map[string]string, error) {
	names := make(map[string]string)
	for _, p := range BuiltinPrompts() {
		names[p.ID] = p.Name
	}
	mine, err := s.store.ListPrompts(ctx, "", "")
	if err != nil {
		return nil, err
	}
	for _, p := range mine {
		names[p.ID] = p.Name
	}
	return names, nil
}

// Get trả chi tiết một template.
func (s *Service) Get(ctx context.Context, id string) (*Detail, error) {
	tpl, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound("Template")
		}
		return nil, err
	}
	if tpl == nil {
		return nil, notFound("Template")
	}
	tpl, err = s.ensureTemplateProject(ctx, tpl)
	if err != nil {
		return nil, err
	}
	channels, err := s.store.TemplateChannels(ctx, id)
	if err != nil {
		return nil, err
	}
	if channels == nil {
		channels = domain.ChannelRefList{}
	}
	names, err := s.promptNames(ctx)
	if err != nil {
		return nil, err
	}
	return &Detail{
		Summary: summarize(domain.TemplateSummary{Template: *tpl, Channels: channels}, names),
		Price:   s.Estimate(ctx, tpl.Recipe),
	}, nil
}

// CreateTemplate tạo template mới với recipe Mẫu chuẩn (không gắn kênh) —
// POST /api/templates và `viderse template create`.
func (s *Service) Create(ctx context.Context, name string) (*domain.Template, error) {
	trimmed, err := validTemplateName(name)
	if err != nil {
		return nil, err
	}
	return s.insert(ctx, trimmed)
}

// standardTemplateName là tên trần của Mẫu chuẩn khi kênh chưa có tên.
const standardTemplateName = "Mẫu chuẩn"

// standardName: "Mẫu chuẩn · <tên kênh>", hoặc tên trần khi kênh chưa có tên.
func standardName(channelName string) string {
	trimmed := strings.TrimSpace(channelName)
	if trimmed == "" {
		return standardTemplateName
	}
	return truncateRunes(standardTemplateName+" · "+trimmed, domain.LimitTemplateNameMaxChars)
}

// CreateStandardTemplate tạo một "Mẫu chuẩn" mới cho một kênh (tên theo
// standardName), mỗi lần gọi một template.
func (s *Service) CreateStandardTemplate(ctx context.Context, channelName string) (string, error) {
	tpl, err := s.insert(ctx, standardName(channelName))
	if err != nil {
		return "", err
	}
	return tpl.ID, nil
}

// NameStandardTemplates đặt tên kèm kênh cho Mẫu chuẩn còn tên trần của kênh
// vừa có tên (Đạo diễn đặt sau khi tạo, hoặc chủ tự đặt). Template đã đổi tên
// tay hay kênh khác cũng dùng thì giữ nguyên; tên trống thì không làm gì.
func (s *Service) NameStandardTemplates(ctx context.Context, channelID, channelName string) error {
	if strings.TrimSpace(channelName) == "" {
		return nil
	}
	ids, err := s.store.NameChannelStandardTemplates(ctx, channelID, standardTemplateName, standardName(channelName))
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.emit(TopicTemplate, id)
	}
	return nil
}

// insert dựng hàng mới với recipe Mẫu chuẩn (thêm tuyết nếu hiệu ứng có sẵn
// đã nằm trong thư viện).
func (s *Service) insert(ctx context.Context, name string) (*domain.Template, error) {
	recipe := StandardRecipe()
	if overlay, ok, err := s.findBuiltinOverlay(ctx, "snow"); err != nil {
		return nil, err
	} else if ok {
		recipe.Overlays = []string{overlay.ID}
	}
	now := s.now().UTC()
	tpl := domain.Template{
		ID: s.newID(), Name: name, Recipe: recipe,
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	project, err := s.standardProject(ctx, tpl.ID, tpl.Name, tpl.CreatedAt, tpl.Recipe)
	if err != nil {
		return nil, err
	}
	tpl.Project = project
	if err := s.publishStandardAssets(ctx, func(txctx context.Context) error {
		return s.store.InsertTemplate(txctx, tpl)
	}); err != nil {
		return nil, err
	}
	s.emit(TopicTemplate, tpl.ID)
	return &tpl, nil
}

// DuplicateTemplate sao chép một template (không kênh, không bản xem thử):
// tên mặc định "<tên> — Bản sao" như yt-factory.
func (s *Service) Duplicate(ctx context.Context, fromID, name string) (*domain.Template, error) {
	src, err := s.store.GetTemplate(ctx, fromID)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, notFound("Template")
	}
	src, err = s.ensureTemplateProject(ctx, src)
	if err != nil {
		return nil, err
	}
	next := strings.TrimSpace(name)
	if next == "" {
		next = src.Name + " — Bản sao"
	}
	next, err = validTemplateName(next)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	tpl := domain.Template{ID: s.newID(), Name: next, Recipe: src.Recipe, Revision: 1, CreatedAt: now, UpdatedAt: now}
	tpl.Project, err = duplicateTemplateProject(src.Project, tpl.ID, tpl.Name, now)
	if err != nil {
		return nil, err
	}
	if err := s.store.InsertTemplate(ctx, tpl); err != nil {
		return nil, err
	}
	s.emit(TopicTemplate, tpl.ID)
	return &tpl, nil
}

// Patch là body PATCH /api/templates/:id: revision bắt buộc, tên và/hoặc
// recipe tuỳ chọn (recipe thay cả khối, trường thiếu lấy mặc định).
type Patch struct {
	Revision int64
	Name     *string
	Recipe   []byte          // nil = không đổi recipe
	Project  json.RawMessage // nil preserves the authored project
}

// PatchTemplate lưu thay đổi với điều kiện revision; lệch → 409 conflict.
func (s *Service) Patch(ctx context.Context, id string, patch Patch) (*domain.Template, error) {
	current, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, notFound("Template")
	}
	current, err = s.ensureTemplateProject(ctx, current)
	if err != nil {
		return nil, err
	}
	name := current.Name
	if patch.Name != nil {
		name, err = validTemplateName(*patch.Name)
		if err != nil {
			return nil, err
		}
	}
	project := current.Project
	recipe := current.Recipe
	if patch.Project != nil {
		project, err = validateTemplateProject(patch.Project, id, current.CreatedAt)
		if err != nil {
			return nil, err
		}
		recipe, err = s.projectTemplateRecipe(ctx, project, recipe)
		if err != nil {
			return nil, err
		}
	}
	if patch.Recipe != nil {
		recipe, err = ParseRecipe(patch.Recipe, recipe)
		if err != nil {
			return nil, err
		}
		if err := ValidateRecipe(recipe); err != nil {
			return nil, err
		}
	}
	if patch.Recipe != nil || patch.Project != nil {
		if err := CheckRefs(ctx, refChecker{store: s.store}, recipe); err != nil {
			return nil, err
		}
	}
	cover, err := s.resolvePrompt(ctx, recipe.Scenes.CoverPromptID, BuiltinCoverClean, "scenes.coverPromptId")
	if err != nil {
		return nil, err
	}
	project, err = updateTemplateProject(project, name, recipe, cover.Composition, patch.Recipe != nil, s.now().UTC())
	if err != nil {
		return nil, err
	}
	var writeName *string
	if patch.Name != nil {
		writeName = &name
	}
	revision, ok, err := s.store.UpdateTemplate(ctx, id, writeName, recipe, patch.Revision, project)
	if errors.Is(err, ErrPromptRef) || errors.Is(err, ErrBackdropRef) {
		// Tham chiếu đổi/mất giữa lúc kiểm và lúc ghi: kiểm lại để trả lỗi đúng trường.
		if refErr := CheckRefs(ctx, refChecker{store: s.store}, recipe); refErr != nil {
			return nil, refErr
		}
		return nil, invalidInput("Một mục trong recipe không còn hợp lệ", "Kiểm tra lại prompt và video nền rồi lưu lại")
	}
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, conflictRevision()
	}
	s.emit(TopicTemplate, id)
	current.Name, current.Recipe, current.Revision, current.UpdatedAt = name, recipe, revision, s.now().UTC()
	current.Project = project
	return current, nil
}

// Rename đổi tên template; revision tuỳ chọn (lệch → 409). Không đổi revision
// vì tên không đi vào bản xem thử.
func (s *Service) Rename(ctx context.Context, id, name string, expectRevision *int64) (*domain.Template, error) {
	trimmed, err := validTemplateName(name)
	if err != nil {
		return nil, err
	}
	ok, err := s.store.RenameTemplate(ctx, id, trimmed, expectRevision)
	if err != nil {
		return nil, err
	}
	if !ok {
		if expectRevision != nil {
			if current, gerr := s.store.GetTemplate(ctx, id); gerr == nil && current == nil {
				return nil, notFound("Template")
			}
			return nil, conflictRevision()
		}
		return nil, notFound("Template")
	}
	tpl, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, notFound("Template")
	}
	s.emit(TopicTemplate, id)
	return tpl, nil
}

// DeleteTemplate xoá template; bị chặn khi kênh còn dùng.
func (s *Service) DeleteTemplate(ctx context.Context, id string) error {
	channels, err := s.store.TemplateChannels(ctx, id)
	if err != nil {
		return err
	}
	if len(channels) > 0 {
		return inUseByChannels(channels)
	}
	tpl, err := s.store.GetTemplate(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return notFound("Template")
	}
	if err != nil {
		return err
	}
	if tpl == nil {
		return notFound("Template")
	}
	if err := s.store.DeleteTemplate(ctx, id); err != nil {
		if errors.Is(err, ErrInUse) {
			return &Error{
				Code:     CodeConflict,
				Message:  "Template vừa được gắn vào một kênh. Bỏ khỏi kênh trước rồi xoá.",
				Recovery: "Tải lại trang rồi thử lại",
			}
		}
		return err
	}
	s.emit(TopicTemplate, id)
	return nil
}

// inUseByChannels dựng lỗi 409 theo câu của 06-copy.
func inUseByChannels(channels domain.ChannelRefList) *Error {
	names := make([]string, 0, len(channels))
	for _, ch := range channels {
		names = append(names, ch.Name)
	}
	return &Error{
		Code:     CodeConflict,
		Message:  fmt.Sprintf("Đang dùng ở %d kênh. Bỏ khỏi kênh trước rồi xoá.", len(channels)),
		Recovery: "Mở kênh đang dùng, bỏ template khỏi danh sách luân phiên rồi xoá lại",
		Channels: names,
	}
}

// validTemplateName kiểm tên template (1–200 ký tự sau khi cắt khoảng trắng).
func validTemplateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", invalidInput("Tên template đang trống", "Nhập tên template")
	}
	if len([]rune(trimmed)) > domain.LimitTemplateNameMaxChars {
		return "", invalidInput("Tên template quá dài", fmt.Sprintf("Rút gọn còn tối đa %d ký tự", domain.LimitTemplateNameMaxChars))
	}
	return trimmed, nil
}

// truncateRunes cắt chuỗi theo ký tự (không cắt giữa byte UTF-8).
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
