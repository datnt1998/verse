package domain

import (
	"encoding/json"
	"time"
)

// Kiểu dùng chung của Thư viện: recipe template (một JSONB duy nhất), prompt
// ảnh và lớp phủ. Chỉ kiểu giá trị + hằng giới hạn — không I/O. Trình kiểm
// recipe ở internal/library dùng chung một bảng giới hạn này cho API và CLI.

// Range là khoảng số đóng cho một trường recipe.
type Range struct {
	Min float64
	Max float64
}

// Bố cục ảnh bìa (thumbnail) — giữ nguyên tập của yt-factory.
type ThumbnailStyle = string

const (
	ThumbClean       ThumbnailStyle = "clean"
	ThumbHook        ThumbnailStyle = "hook"
	ThumbCircleArrow ThumbnailStyle = "circle-arrow"
	ThumbHookArrow   ThumbnailStyle = "hook-arrow"
	ThumbSplit       ThumbnailStyle = "split"
	ThumbCloseup     ThumbnailStyle = "closeup"
)

// thumbnailStyles là tập bố cục hợp lệ (thứ tự hiển thị); đọc qua accessor
// để không bên ngoài nào sửa được bản tham chiếu.
var thumbnailStyles = []ThumbnailStyle{
	ThumbClean, ThumbHook, ThumbCircleArrow, ThumbHookArrow, ThumbSplit, ThumbCloseup,
}

// ThumbnailStyles trả bản sao của tập bố cục hợp lệ.
func ThumbnailStyles() []ThumbnailStyle {
	return append([]ThumbnailStyle(nil), thumbnailStyles...)
}

// Giới hạn recipe + video nền, giữ nguyên con số của yt-factory
// (template-limits.ts). Toàn bộ là hằng — validator và câu lỗi phục hồi đọc
// cùng chỗ, không bên ngoài nào sửa được.
const (
	LimitFixedImagesMin, LimitFixedImagesMax             = 1.0, 60.0
	LimitPerMinutesMin, LimitPerMinutesMax               = 1.0, 60.0
	LimitMotionIntensityMin, LimitMotionIntensityMax     = 0.25, 2.0
	LimitMotionSpeedMin, LimitMotionSpeedMax             = 0.25, 3.0
	LimitSceneOpacityMin, LimitSceneOpacityMax           = 0.0, 1.0
	LimitAppearanceOpacityMin, LimitAppearanceOpacityMax = 0.0, 1.0
	LimitBlurRadiusMin, LimitBlurRadiusMax               = 0.0, 50.0
	LimitBrightnessMin, LimitBrightnessMax               = -1.0, 1.0
	LimitContrastMin, LimitContrastMax                   = 0.0, 3.0
	LimitSaturationMin, LimitSaturationMax               = 0.0, 3.0
	LimitHueShiftMin, LimitHueShiftMax                   = 0.0, 1.0
	LimitAppearanceSpeedMin, LimitAppearanceSpeedMax     = 0.25, 3.0
	LimitZoomMin, LimitZoomMax                           = 1.0, 3.0
	LimitBackdropChannelsMin, LimitBackdropChannelsMax   = 1, 20
	LimitKindsPerChannelMin, LimitKindsPerChannelMax     = 1.0, 3.0
	LimitPinnedPerChannelMin, LimitPinnedPerChannelMax   = 0.0, 200.0
	LimitBackdropClipsMin, LimitBackdropClipsMax         = 1.0, 40.0
	LimitBackdropClipSecMin, LimitBackdropClipSecMax     = 10.0, 600.0
	LimitImagePromptMaxChars                             = 2000
	LimitPinnedIDMaxChars                                = 64
	LimitTemplateNameMaxChars                            = 200
	LimitPromptNameMaxChars                              = 60
	LimitOverlayNameMaxChars                             = 200
)

// ── recipe ─────────────────────────────────────────────────────────────────

// Recipe là toàn bộ cài đặt của một template (cột templates.recipe jsonb).
// Khoá JSON giữ hình đã chốt: scenes/backdrop/overlays/…
type Recipe struct {
	Scenes               ScenesRecipe         `json:"scenes"`
	Backdrop             *BackdropRecipe      `json:"backdrop"`
	Overlays             []string             `json:"overlays"`
	BackgroundAppearance BackgroundAppearance `json:"appearance"`
	Output               OutputRecipe         `json:"output"`
}

// ScenesRecipe là mục Ảnh cảnh + Ảnh bìa của template.
type ScenesRecipe struct {
	Style           string       `json:"style"`             // full | cutout
	Framing         string       `json:"framing,omitempty"` // "close" = Cận cảnh
	Images          ImagesMode   `json:"images"`
	Motion          MotionRecipe `json:"motion"`
	ScenePromptID   string       `json:"scenePromptId,omitempty"`
	CoverPromptID   string       `json:"coverPromptId,omitempty"`
	CoverSource     string       `json:"coverSource"` // upload | ai
	CoverMediaID    string       `json:"coverMediaId,omitempty"`
	ReferenceImages bool         `json:"referenceImages"`
	ReferenceStyle  string       `json:"referenceStyle"` // composition | master
	SceneOpacity    float64      `json:"sceneOpacity"`   // 1 = không giảm
}

// ImagesMode là cách tính số ảnh cảnh: fixed (count) | per-minutes (minutes) | none.
type ImagesMode struct {
	Mode    string `json:"mode"`
	Count   int    `json:"count,omitempty"`
	Minutes int    `json:"minutes,omitempty"`
}

// MotionRecipe là chuyển động của ảnh cảnh.
type MotionRecipe struct {
	Kind      string  `json:"kind"` // ken-burns | wander
	Intensity float64 `json:"intensity"`
	Speed     float64 `json:"speed"`
}

// BackdropRecipe là mục Video nền của template (tham chiếu sang bảng của
// đọc phần này khi chia clip).
type BackdropRecipe struct {
	Channels      []BackdropChannelLink `json:"channels"`
	ClipCount     *int                  `json:"clipCount"` // null = tự phủ kín
	ClipSec       int                   `json:"clipSec"`
	FixedVideoIDs []string              `json:"fixedVideoIds"`
}

// BackdropChannelLink là một kênh nền gắn vào template.
type BackdropChannelLink struct {
	ChannelID string   `json:"channelId"`
	Kinds     []string `json:"kinds"`
	Pinned    []string `json:"pinned"` // rỗng = Ngẫu nhiên mọi video của kinds
}

// BackgroundAppearance là mục Chỉnh nền (10 trường, nhận diện khi mặc định).
type BackgroundAppearance struct {
	Opacity    float64 `json:"opacity"`
	BlurRadius float64 `json:"blurRadius"`
	Brightness float64 `json:"brightness"`
	Contrast   float64 `json:"contrast"`
	Saturation float64 `json:"saturation"`
	HueShift   float64 `json:"hueShift"`
	Speed      float64 `json:"speed"`
	Zoom       float64 `json:"zoom"`
	FlipX      bool    `json:"flipX"`
	FlipY      bool    `json:"flipY"`
}

// OutputRecipe là mục Xuất & kênh.
type OutputRecipe struct {
	CompressExport bool `json:"compressExport"`
}

// ── template ───────────────────────────────────────────────────────────────

// PreviewState là trạng thái bản xem thử: none (chưa có) | fresh | stale.
type PreviewState = string

const (
	PreviewNone  PreviewState = "none"
	PreviewFresh PreviewState = "fresh"
	PreviewStale PreviewState = "stale"
)

// PreviewError là lần dựng bản xem thử hỏng gần nhất, giữ nguyên câu hành
// động của lỗi dựng (library.Error) để UI không phải thay bằng câu chung.
// Bản xem thử cũ vẫn còn; lỗi chỉ được xoá khi lần dựng sau thành công hoặc
// khi bắt đầu lần dựng mới.
type PreviewError struct {
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
	Recovery string `json:"recovery,omitempty"`
}

// Template là một dòng templates.
type Template struct {
	ID              string
	Name            string
	Recipe          Recipe
	Project         json.RawMessage // canonical schema-v15 editor project; nil only before durable backfill
	Revision        int64
	PreviewRevision *int64
	PreviewBuiltAt  *time.Time
	// PreviewProjectID là project xem thử đang dùng (bản sao của project
	// template, đổ nội dung từ một video đã xong); rỗng khi chưa có.
	PreviewProjectID string
	PreviewJobID     string
	PreviewMinutes   int
	// PreviewError là lỗi của lần dựng gần nhất; nil khi lần gần nhất thành
	// công. Không làm mất bản xem thử cũ (PreviewProjectID giữ nguyên).
	PreviewError *PreviewError
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PreviewStateOf trả trạng thái xem thử từ cặp revision/preview_revision.
func (t *Template) PreviewState() PreviewState {
	switch {
	case t.PreviewRevision == nil:
		return PreviewNone
	case *t.PreviewRevision != t.Revision:
		return PreviewStale
	default:
		return PreviewFresh
	}
}

// ChannelRef là một kênh đang dùng template (đọc chéo owner_channel_templates).
type ChannelRef struct {
	ID       string
	Name     string
	Position int
}

// TemplateSummary là một dòng của GET /api/templates.
type TemplateSummary struct {
	Template
	Channels ChannelRefList
}

// ChannelRefList là danh sách kênh theo tên.
type ChannelRefList []ChannelRef

// ── prompt ảnh ─────────────────────────────────────────────────────────────

// PromptKind là loại prompt ảnh.
type PromptKind = string

const (
	PromptScene PromptKind = "scene"
	PromptCover PromptKind = "cover"
)

// ImagePrompt là một prompt trong kho Prompt ảnh.
type ImagePrompt struct {
	ID          string // uuid, hoặc "builtin:scene" / "builtin:cover-<style>"
	Name        string
	Kind        PromptKind
	BuiltIn     bool
	Composition ThumbnailStyle // rỗng với kind scene
	Prompt      string
	UpdatedAt   time.Time
}

// ── lớp phủ ────────────────────────────────────────────────────────────────

// OverlayBackground là nền phát hiện của một lớp phủ.
type OverlayBackground = string

const (
	BgBlack OverlayBackground = "black"
	BgGreen OverlayBackground = "green"
	BgNone  OverlayBackground = "none"
)

// Overlay là một dòng overlays đang còn trong thư viện (removed_at null).
type Overlay struct {
	ID          string
	SHA256      string
	Name        string
	FilePath    string // tương đối dưới thư mục dự án
	Mime        string
	Bytes       int64
	DurationSec *float64
	Width       *int
	Height      *int
	Background  OverlayBackground
	Builtin     string // "" hoặc id hiệu ứng có sẵn (rain, fog, snow…)
	CreatedAt   time.Time
}
