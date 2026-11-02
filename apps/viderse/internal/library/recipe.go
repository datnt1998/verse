package library

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Recipe của template: mặc định, phân tích JSON và kiểm hai lớp — cú pháp
// (miền giá trị, tập giá trị) và tham chiếu (prompt/lớp phủ/kênh nền còn
// tồn tại). API và CLI dùng chung đúng một đường kiểm này.
//
// Port giới hạn từ yt-factory src/editor/engine/features/editing-templates/
// template-limits.ts + template-schemas.ts; hình JSON do màn Thư viện định.

// Các tập giá trị hợp lệ.
const (
	StyleFull   = "full"
	StyleCutout = "cutout"

	FramingClose = "close"

	ImagesFixed      = "fixed"
	ImagesPerMinutes = "per-minutes"
	ImagesNone       = "none"

	MotionKenBurns = "ken-burns"
	MotionWander   = "wander"

	CoverSourceUpload = "upload"
	CoverSourceAI     = "ai"

	ReferenceComposition = "composition"
	ReferenceMaster      = "master"
)

// backdropChannelRe giữ đúng biểu thức của migration: id kênh nền hợp lệ.
var backdropChannelRe = regexp.MustCompile(`^(twitch:[a-z0-9_]{1,25}|bilibili:[0-9]{1,20})$`)

// fixedVideoIDRe giữ đúng biểu thức của migration: "<platform>:<videoId>".
var fixedVideoIDRe = regexp.MustCompile(`^[a-z]+:.+$`)

// backdropKinds trả loại video mỗi nền tảng cho phép (yt-factory backdrop.ts).
func backdropKinds(platform string) []string {
	switch platform {
	case "bilibili":
		return []string{"video"}
	default:
		return []string{"clip", "highlight", "vod"}
	}
}

// DefaultAppearance là "Chỉnh nền" không đổi gì.
func DefaultAppearance() domain.BackgroundAppearance {
	return domain.BackgroundAppearance{
		Opacity: 1, BlurRadius: 0, Brightness: 0, Contrast: 1,
		Saturation: 1, HueShift: 0, Speed: 1, Zoom: 1,
	}
}

// DefaultRecipe là recipe khi chưa đặt gì: toàn khung, 5 ảnh, Ken Burns,
// bìa "Ảnh sạch", xuất có nén — giữ nguyên hành vi của yt-factory.
func DefaultRecipe() domain.Recipe {
	return domain.Recipe{
		Scenes: domain.ScenesRecipe{
			Style:           StyleFull,
			Images:          domain.ImagesMode{Mode: ImagesFixed, Count: 5},
			Motion:          domain.MotionRecipe{Kind: MotionKenBurns, Intensity: 1, Speed: 1},
			CoverPromptID:   BuiltinCoverClean,
			CoverSource:     CoverSourceUpload,
			ReferenceImages: true,
			ReferenceStyle:  ReferenceComposition,
			SceneOpacity:    1,
		},
		Overlays:             []string{},
		BackgroundAppearance: DefaultAppearance(),
		Output:               domain.OutputRecipe{CompressExport: true},
	}
}

// StandardRecipe resolves the untouched schema-v15 standard layout, matching
// yt-factory's resolveTemplateRecipe (the layout has no templateScenes override).
func StandardRecipe() domain.Recipe {
	r := DefaultRecipe()
	r.Scenes.ScenePromptID = BuiltinScene
	return r
}

// ── phân tích JSON ─────────────────────────────────────────────────────────

// rawRecipe giữ con trỏ để biết trường nào có mặt; trường vắng giữ giá trị
// của base (PATCH thay cả recipe, trường thiếu nghĩa là mặc định).
type rawRecipe struct {
	Scenes     *rawScenes      `json:"scenes"`
	Backdrop   json.RawMessage `json:"backdrop"`
	Overlays   *[]string       `json:"overlays"`
	Appearance *rawAppearance  `json:"appearance"`
	Output     *rawOutput      `json:"output"`
}

type rawScenes struct {
	Style           *string    `json:"style"`
	Framing         *string    `json:"framing"`
	Images          *rawImages `json:"images"`
	Motion          *rawMotion `json:"motion"`
	ScenePromptID   *string    `json:"scenePromptId"`
	CoverPromptID   *string    `json:"coverPromptId"`
	CoverSource     *string    `json:"coverSource"`
	CoverMediaID    *string    `json:"coverMediaId"`
	ReferenceImages *bool      `json:"referenceImages"`
	ReferenceStyle  *string    `json:"referenceStyle"`
	SceneOpacity    *float64   `json:"sceneOpacity"`
}

type rawImages struct {
	Mode    *string `json:"mode"`
	Count   *int    `json:"count"`
	Minutes *int    `json:"minutes"`
}

type rawMotion struct {
	Kind      *string  `json:"kind"`
	Intensity *float64 `json:"intensity"`
	Speed     *float64 `json:"speed"`
}

type rawBackdrop struct {
	Channels      *[]rawChannelLink `json:"channels"`
	ClipCount     *int              `json:"clipCount"` // null = tự phủ kín
	ClipSec       *int              `json:"clipSec"`
	FixedVideoIDs *[]string         `json:"fixedVideoIds"`
}

type rawChannelLink struct {
	ChannelID *string   `json:"channelId"`
	Kinds     *[]string `json:"kinds"`
	Pinned    *[]string `json:"pinned"`
}

type rawAppearance struct {
	Opacity    *float64 `json:"opacity"`
	BlurRadius *float64 `json:"blurRadius"`
	Brightness *float64 `json:"brightness"`
	Contrast   *float64 `json:"contrast"`
	Saturation *float64 `json:"saturation"`
	HueShift   *float64 `json:"hueShift"`
	Speed      *float64 `json:"speed"`
	Zoom       *float64 `json:"zoom"`
	FlipX      *bool    `json:"flipX"`
	FlipY      *bool    `json:"flipY"`
}

type rawOutput struct {
	CompressExport *bool `json:"compressExport"`
}

// ParseRecipe đọc JSON recipe đè lên base, không kiểm miền giá trị (bước
// ValidateRecipe làm việc đó và trả lỗi theo từng trường). JSON hỏng hoặc
// field lạ là lỗi invalid_input ngay.
func ParseRecipe(data []byte, base domain.Recipe) (domain.Recipe, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var raw rawRecipe
	if err := dec.Decode(&raw); err != nil {
		return domain.Recipe{}, invalidInput("Recipe không đọc được", "Kiểm tra lại các mục Ảnh cảnh, Ảnh bìa, Nền, Xuất")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return domain.Recipe{}, invalidInput("Recipe không đọc được", "Chỉ gửi một đối tượng JSON duy nhất")
	}
	next := base
	if raw.Scenes != nil {
		applyScenes(&next.Scenes, raw.Scenes)
	}
	if raw.Appearance != nil {
		applyAppearance(&next.BackgroundAppearance, raw.Appearance)
	}
	if raw.Output != nil && raw.Output.CompressExport != nil {
		next.Output.CompressExport = *raw.Output.CompressExport
	}
	if raw.Overlays != nil {
		next.Overlays = dedupeStrings(*raw.Overlays)
	}
	if len(raw.Backdrop) > 0 {
		b, err := parseBackdrop(raw.Backdrop)
		if err != nil {
			return domain.Recipe{}, err
		}
		next.Backdrop = b
	}
	return next, nil
}

// applyScenes đè các trường có mặt của scenes.
func applyScenes(dst *domain.ScenesRecipe, raw *rawScenes) {
	if raw.Style != nil {
		dst.Style = *raw.Style
	}
	if raw.Framing != nil {
		dst.Framing = *raw.Framing
	}
	if raw.Images != nil {
		if raw.Images.Mode != nil {
			dst.Images = domain.ImagesMode{Mode: *raw.Images.Mode}
		}
		if raw.Images.Count != nil {
			dst.Images.Count = *raw.Images.Count
		}
		if raw.Images.Minutes != nil {
			dst.Images.Minutes = *raw.Images.Minutes
		}
	}
	if raw.Motion != nil {
		if raw.Motion.Kind != nil {
			dst.Motion.Kind = *raw.Motion.Kind
		}
		if raw.Motion.Intensity != nil {
			dst.Motion.Intensity = *raw.Motion.Intensity
		}
		if raw.Motion.Speed != nil {
			dst.Motion.Speed = *raw.Motion.Speed
		}
	}
	if raw.ScenePromptID != nil {
		dst.ScenePromptID = *raw.ScenePromptID
	}
	if raw.CoverPromptID != nil {
		dst.CoverPromptID = *raw.CoverPromptID
	}
	if raw.CoverSource != nil {
		dst.CoverSource = *raw.CoverSource
	}
	if raw.CoverMediaID != nil {
		dst.CoverMediaID = *raw.CoverMediaID
	}
	if raw.ReferenceImages != nil {
		dst.ReferenceImages = *raw.ReferenceImages
	}
	if raw.ReferenceStyle != nil {
		dst.ReferenceStyle = *raw.ReferenceStyle
	}
	if raw.SceneOpacity != nil {
		dst.SceneOpacity = *raw.SceneOpacity
	}
}

// applyAppearance đè các trường có mặt của "Chỉnh nền".
func applyAppearance(dst *domain.BackgroundAppearance, raw *rawAppearance) {
	set := func(d *float64, v *float64) {
		if v != nil {
			*d = *v
		}
	}
	set(&dst.Opacity, raw.Opacity)
	set(&dst.BlurRadius, raw.BlurRadius)
	set(&dst.Brightness, raw.Brightness)
	set(&dst.Contrast, raw.Contrast)
	set(&dst.Saturation, raw.Saturation)
	set(&dst.HueShift, raw.HueShift)
	set(&dst.Speed, raw.Speed)
	set(&dst.Zoom, raw.Zoom)
	if raw.FlipX != nil {
		dst.FlipX = *raw.FlipX
	}
	if raw.FlipY != nil {
		dst.FlipY = *raw.FlipY
	}
}

// parseBackdrop đọc phần backdrop (null → nil).
func parseBackdrop(data json.RawMessage) (*domain.BackdropRecipe, error) {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) || len(trimmed) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var raw rawBackdrop
	if err := dec.Decode(&raw); err != nil {
		return nil, invalidInput("Cấu hình nền không đọc được", "Kiểm tra lại danh sách kênh nền và số clip")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, invalidInput("Cấu hình nền không đọc được", "Chỉ gửi một đối tượng JSON duy nhất")
	}
	out := &domain.BackdropRecipe{ClipSec: 60, FixedVideoIDs: []string{}}
	if raw.ClipSec != nil {
		out.ClipSec = *raw.ClipSec
	}
	out.ClipCount = raw.ClipCount
	if raw.FixedVideoIDs != nil {
		out.FixedVideoIDs = dedupeStrings(*raw.FixedVideoIDs)
	}
	if raw.Channels != nil {
		for _, link := range *raw.Channels {
			l := domain.BackdropChannelLink{Pinned: []string{}}
			if link.ChannelID != nil {
				l.ChannelID = *link.ChannelID
			}
			if link.Kinds != nil {
				l.Kinds = dedupeStrings(*link.Kinds)
			}
			if link.Pinned != nil {
				l.Pinned = dedupeStrings(*link.Pinned)
			}
			out.Channels = append(out.Channels, l)
		}
	}
	return out, nil
}

// ── kiểm cú pháp ───────────────────────────────────────────────────────────

// ValidateRecipe kiểm miền giá trị và tập giá trị; không chạm DB.
func ValidateRecipe(r domain.Recipe) error {
	fields := append(validateScenes(r.Scenes), validateAppearance(r.BackgroundAppearance)...)
	fields = append(fields, validateBackdrop(r.Backdrop)...)
	if len(fields) > 0 {
		return invalidFields("Cấu hình mẫu video không hợp lệ", fields)
	}
	return nil
}

func validateScenes(s domain.ScenesRecipe) []FieldError {
	fields := make([]FieldError, 0, 8)
	if s.Style != StyleFull && s.Style != StyleCutout {
		fields = append(fields, FieldError{"scenes.style", "Cách ghép ảnh vào video không hợp lệ", "Chọn Toàn khung hoặc Tách nền"})
	}
	if s.Framing != "" && s.Framing != FramingClose {
		fields = append(fields, FieldError{"scenes.framing", "Khung ảnh cảnh không hợp lệ", `Chỉ dùng "close" cho Cận cảnh`})
	}
	if s.Framing == FramingClose && s.Style != StyleCutout {
		fields = append(fields, FieldError{"scenes.framing", "Cận cảnh phải đi với Tách nền", "Chọn cách ghép Tách nền rồi chọn Cận cảnh"})
	}
	switch s.Images.Mode {
	case ImagesFixed:
		if !inRange(float64(s.Images.Count), domain.Range{Min: domain.LimitFixedImagesMin, Max: domain.LimitFixedImagesMax}) {
			fields = append(fields, FieldError{"scenes.images.count", "Số ảnh cảnh không hợp lệ", "Nhập số " + rangeText(domain.LimitFixedImagesMin, domain.LimitFixedImagesMax)})
		}
	case ImagesPerMinutes:
		if !inRange(float64(s.Images.Minutes), domain.Range{Min: domain.LimitPerMinutesMin, Max: domain.LimitPerMinutesMax}) {
			fields = append(fields, FieldError{"scenes.images.minutes", "Số phút mỗi ảnh không hợp lệ", "Nhập số " + rangeText(domain.LimitPerMinutesMin, domain.LimitPerMinutesMax)})
		}
	case ImagesNone:
	default:
		fields = append(fields, FieldError{"scenes.images.mode", "Cách tính số ảnh cảnh không hợp lệ", "Chọn Số ảnh cố định, Mỗi N phút một ảnh, hoặc Không vẽ ảnh"})
	}
	if s.Motion.Kind != MotionKenBurns && s.Motion.Kind != MotionWander {
		fields = append(fields, FieldError{"scenes.motion.kind", "Chuyển động không hợp lệ", "Chọn Ken Burns hoặc Trôi"})
	}
	if !inRange(s.Motion.Intensity, domain.Range{Min: domain.LimitMotionIntensityMin, Max: domain.LimitMotionIntensityMax}) {
		fields = append(fields, FieldError{"scenes.motion.intensity", "Độ mạnh chuyển động không hợp lệ", "Nhập số " + rangeText(domain.LimitMotionIntensityMin, domain.LimitMotionIntensityMax)})
	}
	if !inRange(s.Motion.Speed, domain.Range{Min: domain.LimitMotionSpeedMin, Max: domain.LimitMotionSpeedMax}) {
		fields = append(fields, FieldError{"scenes.motion.speed", "Tốc độ chuyển động không hợp lệ", "Nhập số " + rangeText(domain.LimitMotionSpeedMin, domain.LimitMotionSpeedMax)})
	}
	if s.CoverSource != CoverSourceUpload && s.CoverSource != CoverSourceAI {
		fields = append(fields, FieldError{"scenes.coverSource", "Nguồn ảnh bìa không hợp lệ", "Chọn Ảnh tải lên hoặc Ảnh vẽ"})
	}
	if s.ReferenceStyle != ReferenceComposition && s.ReferenceStyle != ReferenceMaster {
		fields = append(fields, FieldError{"scenes.referenceStyle", "Kiểu tham chiếu ảnh đối thủ không hợp lệ", "Chọn Giữ bố cục hoặc Làm lại hoàn toàn"})
	}
	if !inRange(s.SceneOpacity, domain.Range{Min: domain.LimitSceneOpacityMin, Max: domain.LimitSceneOpacityMax}) {
		fields = append(fields, FieldError{"scenes.sceneOpacity", "Độ trong suốt ảnh cảnh không hợp lệ", "Nhập số " + rangeText(domain.LimitSceneOpacityMin, domain.LimitSceneOpacityMax)})
	}
	if len([]rune(s.ScenePromptID)) > 200 || len([]rune(s.CoverPromptID)) > 200 {
		fields = append(fields, FieldError{"scenes.scenePromptId", "Mã chỉ dẫn ảnh quá dài", "Chọn chỉ dẫn trong Thư viện"})
	}
	return fields
}

func validateAppearance(a domain.BackgroundAppearance) []FieldError {
	fields := make([]FieldError, 0, 4)
	check := func(field, label string, v float64, r domain.Range) {
		if !inRange(v, r) {
			fields = append(fields, FieldError{field, label + " không hợp lệ", "Nhập số " + rangeText(r.Min, r.Max)})
		}
	}
	check("appearance.opacity", "Độ mờ nền", a.Opacity, domain.Range{Min: domain.LimitAppearanceOpacityMin, Max: domain.LimitAppearanceOpacityMax})
	check("appearance.blurRadius", "Độ mờ nhoè nền", a.BlurRadius, domain.Range{Min: domain.LimitBlurRadiusMin, Max: domain.LimitBlurRadiusMax})
	check("appearance.brightness", "Độ sáng nền", a.Brightness, domain.Range{Min: domain.LimitBrightnessMin, Max: domain.LimitBrightnessMax})
	check("appearance.contrast", "Độ tương phản nền", a.Contrast, domain.Range{Min: domain.LimitContrastMin, Max: domain.LimitContrastMax})
	check("appearance.saturation", "Độ bão hoà nền", a.Saturation, domain.Range{Min: domain.LimitSaturationMin, Max: domain.LimitSaturationMax})
	check("appearance.hueShift", "Dịch màu nền", a.HueShift, domain.Range{Min: domain.LimitHueShiftMin, Max: domain.LimitHueShiftMax})
	check("appearance.speed", "Tốc độ nền", a.Speed, domain.Range{Min: domain.LimitAppearanceSpeedMin, Max: domain.LimitAppearanceSpeedMax})
	check("appearance.zoom", "Độ phóng nền", a.Zoom, domain.Range{Min: domain.LimitZoomMin, Max: domain.LimitZoomMax})
	return fields
}

func validateBackdrop(b *domain.BackdropRecipe) []FieldError {
	if b == nil {
		return nil
	}
	fields := make([]FieldError, 0, 4)
	backdropClipSec := domain.Range{Min: domain.LimitBackdropClipSecMin, Max: domain.LimitBackdropClipSecMax}
	backdropClips := domain.Range{Min: domain.LimitBackdropClipsMin, Max: domain.LimitBackdropClipsMax}
	if len(b.Channels) < domain.LimitBackdropChannelsMin || len(b.Channels) > domain.LimitBackdropChannelsMax {
		fields = append(fields, FieldError{"backdrop.channels", "Danh sách kênh nền không hợp lệ", fmt.Sprintf("Gắn %d–%d kênh nền, hoặc bỏ trống mục nền", domain.LimitBackdropChannelsMin, domain.LimitBackdropChannelsMax)})
	}
	seen := make(map[string]bool, len(b.Channels))
	for i, link := range b.Channels {
		where := fmt.Sprintf("backdrop.channels[%d]", i)
		if !backdropChannelRe.MatchString(link.ChannelID) {
			fields = append(fields, FieldError{where + ".channelId", "Kênh nền không hợp lệ", "Chọn kênh trong Thư viện › Video nền"})
			continue
		}
		if seen[link.ChannelID] {
			fields = append(fields, FieldError{where + ".channelId", "Kênh nền bị gắn hai lần", "Bỏ dòng trùng rồi lưu lại"})
		}
		seen[link.ChannelID] = true
		allowed := backdropKinds(platformOf(link.ChannelID))
		if len(link.Kinds) < int(domain.LimitKindsPerChannelMin) || len(link.Kinds) > int(domain.LimitKindsPerChannelMax) {
			fields = append(fields, FieldError{where + ".kinds", "Loại video của kênh nền không hợp lệ", "Chọn 1–3 loại: " + strings.Join(allowed, ", ")})
		}
		for _, kind := range link.Kinds {
			if !contains(allowed, kind) {
				fields = append(fields, FieldError{where + ".kinds", "Loại video \"" + kind + "\" không dùng được cho kênh " + platformOf(link.ChannelID), "Chọn trong: " + strings.Join(allowed, ", ")})
			}
		}
		if len(link.Pinned) > int(domain.LimitPinnedPerChannelMax) {
			fields = append(fields, FieldError{where + ".pinned", "Quá nhiều video ghim", fmt.Sprintf("Chọn tối đa %d video", int(domain.LimitPinnedPerChannelMax))})
		}
		for _, id := range link.Pinned {
			if id == "" || len([]rune(id)) > domain.LimitPinnedIDMaxChars {
				fields = append(fields, FieldError{where + ".pinned", "Mã video ghim không hợp lệ", fmt.Sprintf("Mã video %d–%d ký tự", 1, domain.LimitPinnedIDMaxChars)})
			}
		}
	}
	if b.ClipCount != nil && !inRange(float64(*b.ClipCount), backdropClips) {
		fields = append(fields, FieldError{"backdrop.clipCount", "Số clip nền không hợp lệ", "Để trống (tự phủ kín) hoặc nhập số " + rangeText(backdropClips.Min, backdropClips.Max)})
	}
	if !inRange(float64(b.ClipSec), backdropClipSec) {
		fields = append(fields, FieldError{"backdrop.clipSec", "Độ dài mỗi clip nền không hợp lệ", "Nhập số " + rangeText(backdropClipSec.Min, backdropClipSec.Max) + " giây"})
	}
	for i, id := range b.FixedVideoIDs {
		if !fixedVideoIDRe.MatchString(id) {
			fields = append(fields, FieldError{fmt.Sprintf("backdrop.fixedVideoIds[%d]", i), "Video lẻ không hợp lệ", "Chọn video trong Thư viện › Video nền › Video lẻ"})
		}
	}
	return fields
}

// ── kiểm tham chiếu ────────────────────────────────────────────────────────

// RefChecker là mặt đọc để kiểm tham chiếu của recipe: prompt/lớp phủ trong
// Thư viện, và kênh/video của mảng Video nền (chỉ đọc bảng của mảng đó).
type RefChecker interface {
	PromptExists(ctx context.Context, id string, kind domain.PromptKind) (bool, error)
	OverlayExists(ctx context.Context, id string) (bool, error)
	BackdropChannels(ctx context.Context, ids []string) (missing []string, err error)
	BackdropVideos(ctx context.Context, channelID string, ids []string) (missing []string, err error)
	BackdropVideoExists(ctx context.Context, id string) (bool, error)
}

// CheckRefs kiểm mọi tham chiếu của recipe còn tồn tại và đúng loại.
func CheckRefs(ctx context.Context, refs RefChecker, r domain.Recipe) error {
	fields := make([]FieldError, 0, 4)
	for _, ref := range []struct {
		field string
		id    string
		kind  domain.PromptKind
		label string
	}{
		{"scenes.scenePromptId", r.Scenes.ScenePromptID, domain.PromptScene, "Chỉ dẫn ảnh cảnh"},
		{"scenes.coverPromptId", r.Scenes.CoverPromptID, domain.PromptCover, "Chỉ dẫn ảnh bìa"},
	} {
		if ref.id == "" {
			continue
		}
		ok, err := refs.PromptExists(ctx, ref.id, ref.kind)
		if err != nil {
			return err
		}
		if !ok {
			fields = append(fields, FieldError{ref.field, ref.label + " không tồn tại hoặc sai loại", "Chọn một chỉ dẫn ảnh " + kindWord(ref.kind) + " trong Thư viện"})
		}
	}
	for i, id := range r.Overlays {
		ok, err := refs.OverlayExists(ctx, id)
		if err != nil {
			return err
		}
		if !ok {
			fields = append(fields, FieldError{fmt.Sprintf("overlays[%d]", i), "Lớp phủ không còn trong thư viện", "Bỏ lớp phủ này rồi lưu lại"})
		}
	}
	if b := r.Backdrop; b != nil {
		ids := make([]string, 0, len(b.Channels))
		for _, link := range b.Channels {
			ids = append(ids, link.ChannelID)
		}
		missing, err := refs.BackdropChannels(ctx, ids)
		if err != nil {
			return err
		}
		for _, id := range missing {
			fields = append(fields, FieldError{"backdrop.channels", "Kênh nền " + id + " chưa có trong thư viện", "Thêm kênh ở Thư viện › Video nền rồi chọn lại"})
		}
		for i, link := range b.Channels {
			if len(link.Pinned) == 0 {
				continue
			}
			missing, err := refs.BackdropVideos(ctx, link.ChannelID, link.Pinned)
			if err != nil {
				return err
			}
			for _, id := range missing {
				fields = append(fields, FieldError{fmt.Sprintf("backdrop.channels[%d].pinned", i), "Video \"" + id + "\" chưa có trong kênh nền", "Làm mới kênh trong Thư viện rồi chọn lại video"})
			}
		}
		for i, id := range b.FixedVideoIDs {
			ok, err := refs.BackdropVideoExists(ctx, id)
			if err != nil {
				return err
			}
			if !ok {
				fields = append(fields, FieldError{fmt.Sprintf("backdrop.fixedVideoIds[%d]", i), "Video lẻ " + id + " không còn trong thư viện", "Chọn video lẻ khác rồi lưu lại"})
			}
		}
	}
	if len(fields) > 0 {
		return invalidFields("Cấu hình mẫu video trỏ tới thứ không còn", fields)
	}
	return nil
}

// kindWord là danh từ loại prompt dùng trong câu lỗi.
func kindWord(kind domain.PromptKind) string {
	if kind == domain.PromptCover {
		return "bìa"
	}
	return "cảnh"
}

// platformOf suy nền tảng từ id kênh nền.
func platformOf(channelID string) string {
	if strings.HasPrefix(channelID, "bilibili:") {
		return "bilibili"
	}
	return "twitch"
}

// inRange báo v nằm trong [Min, Max].
func inRange(v float64, r domain.Range) bool { return v >= r.Min && v <= r.Max }

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// dedupeStrings bỏ trùng, giữ thứ tự.
func dedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, v := range in {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
