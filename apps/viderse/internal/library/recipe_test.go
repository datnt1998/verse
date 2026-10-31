package library

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Bảng giới hạn của recipe: mọi biên port từ template-limits.ts. Bảng test
// soi đúng trường lỗi và câu phục hồi để API/CLI không lệch nhau.

func TestDefaultRecipeMatchesYtFactory(t *testing.T) {
	r := DefaultRecipe()
	if r.Scenes.Style != StyleFull || r.Scenes.Images.Mode != ImagesFixed || r.Scenes.Images.Count != 5 {
		t.Fatalf("mặc định = %+v", r.Scenes)
	}
	if r.Scenes.Motion != (domain.MotionRecipe{Kind: MotionKenBurns, Intensity: 1, Speed: 1}) {
		t.Fatalf("motion = %+v", r.Scenes.Motion)
	}
	if r.Scenes.CoverPromptID != BuiltinCoverClean || r.Scenes.CoverSource != CoverSourceUpload {
		t.Fatalf("bìa = %+v", r.Scenes)
	}
	if !r.Scenes.ReferenceImages || r.Scenes.ReferenceStyle != ReferenceComposition {
		t.Fatalf("tham chiếu = %+v", r.Scenes)
	}
	if r.Scenes.SceneOpacity != 1 || r.Backdrop != nil || len(r.Overlays) != 0 || !r.Output.CompressExport {
		t.Fatalf("phần còn lại = %+v", r)
	}
	if r.BackgroundAppearance != DefaultAppearance() {
		t.Fatalf("appearance = %+v", r.BackgroundAppearance)
	}
	if err := ValidateRecipe(r); err != nil {
		t.Fatalf("mặc định phải hợp lệ: %v", err)
	}
}

func TestStandardRecipeUsesCanonicalUntouchedLayoutDefaults(t *testing.T) {
	r := StandardRecipe()
	if r.Scenes.Style != StyleFull || r.Scenes.ScenePromptID != BuiltinScene || r.Scenes.CoverPromptID != BuiltinCoverClean {
		t.Fatalf("Mẫu chuẩn = %+v", r.Scenes)
	}
	if err := ValidateRecipe(r); err != nil {
		t.Fatalf("Mẫu chuẩn phải hợp lệ: %v", err)
	}
}

func TestValidateRecipeLimits(t *testing.T) {
	valid := DefaultRecipe()
	cases := []struct {
		name   string
		mutate func(*domain.Recipe)
		field  string
	}{
		{"style lạ", func(r *domain.Recipe) { r.Scenes.Style = "green" }, "scenes.style"},
		{"framing lạ", func(r *domain.Recipe) { r.Scenes.Framing = "wide" }, "scenes.framing"},
		{"cận cảnh không tách nền", func(r *domain.Recipe) { r.Scenes.Framing = FramingClose }, "scenes.framing"},
		{"mode ảnh lạ", func(r *domain.Recipe) { r.Scenes.Images = domain.ImagesMode{Mode: "sometimes"} }, "scenes.images.mode"},
		{"0 ảnh", func(r *domain.Recipe) { r.Scenes.Images.Count = 0 }, "scenes.images.count"},
		{"61 ảnh", func(r *domain.Recipe) { r.Scenes.Images.Count = 61 }, "scenes.images.count"},
		{"0 phút", func(r *domain.Recipe) {
			r.Scenes.Images = domain.ImagesMode{Mode: ImagesPerMinutes, Minutes: 0}
		}, "scenes.images.minutes"},
		{"61 phút", func(r *domain.Recipe) {
			r.Scenes.Images = domain.ImagesMode{Mode: ImagesPerMinutes, Minutes: 61}
		}, "scenes.images.minutes"},
		{"motion lạ", func(r *domain.Recipe) { r.Scenes.Motion.Kind = "spin" }, "scenes.motion.kind"},
		{"cường độ 0.2", func(r *domain.Recipe) { r.Scenes.Motion.Intensity = 0.2 }, "scenes.motion.intensity"},
		{"cường độ 2.1", func(r *domain.Recipe) { r.Scenes.Motion.Intensity = 2.1 }, "scenes.motion.intensity"},
		{"tốc độ 0", func(r *domain.Recipe) { r.Scenes.Motion.Speed = 0 }, "scenes.motion.speed"},
		{"nguồn bìa lạ", func(r *domain.Recipe) { r.Scenes.CoverSource = "magic" }, "scenes.coverSource"},
		{"tham chiếu lạ", func(r *domain.Recipe) { r.Scenes.ReferenceStyle = "copy" }, "scenes.referenceStyle"},
		{"trong suốt âm", func(r *domain.Recipe) { r.Scenes.SceneOpacity = -0.1 }, "scenes.sceneOpacity"},
		{"appearance opacity 2", func(r *domain.Recipe) { r.BackgroundAppearance.Opacity = 2 }, "appearance.opacity"},
		{"blur 51", func(r *domain.Recipe) { r.BackgroundAppearance.BlurRadius = 51 }, "appearance.blurRadius"},
		{"brightness -1.1", func(r *domain.Recipe) { r.BackgroundAppearance.Brightness = -1.1 }, "appearance.brightness"},
		{"contrast âm", func(r *domain.Recipe) { r.BackgroundAppearance.Contrast = -0.1 }, "appearance.contrast"},
		{"saturation 4", func(r *domain.Recipe) { r.BackgroundAppearance.Saturation = 4 }, "appearance.saturation"},
		{"hue 1.2", func(r *domain.Recipe) { r.BackgroundAppearance.HueShift = 1.2 }, "appearance.hueShift"},
		{"speed 0.2", func(r *domain.Recipe) { r.BackgroundAppearance.Speed = 0.2 }, "appearance.speed"},
		{"zoom 0.9", func(r *domain.Recipe) { r.BackgroundAppearance.Zoom = 0.9 }, "appearance.zoom"},
		{"clipSec 9", func(r *domain.Recipe) { r.Backdrop = &domain.BackdropRecipe{ClipSec: 9} }, "backdrop.clipSec"},
		{"clipSec 601", func(r *domain.Recipe) { r.Backdrop = &domain.BackdropRecipe{ClipSec: 601} }, "backdrop.clipSec"},
		{"clips 0", func(r *domain.Recipe) {
			zero := 0
			r.Backdrop = &domain.BackdropRecipe{ClipSec: 60, ClipCount: &zero}
		}, "backdrop.clipCount"},
		{"clips 41", func(r *domain.Recipe) {
			v := 41
			r.Backdrop = &domain.BackdropRecipe{ClipSec: 60, ClipCount: &v}
		}, "backdrop.clipCount"},
		{"kênh nền rỗng", func(r *domain.Recipe) {
			r.Backdrop = &domain.BackdropRecipe{ClipSec: 60}
		}, "backdrop.channels"},
		{"kênh nền sai mã", func(r *domain.Recipe) {
			r.Backdrop = &domain.BackdropRecipe{ClipSec: 60, Channels: []domain.BackdropChannelLink{{ChannelID: "youtube:x", Kinds: []string{"vod"}}}}
		}, "backdrop.channels[0].channelId"},
		{"kênh nền trùng", func(r *domain.Recipe) {
			r.Backdrop = &domain.BackdropRecipe{ClipSec: 60, Channels: []domain.BackdropChannelLink{
				{ChannelID: "twitch:a", Kinds: []string{"vod"}},
				{ChannelID: "twitch:a", Kinds: []string{"clip"}},
			}}
		}, "backdrop.channels[1].channelId"},
		{"kinds rỗng", func(r *domain.Recipe) {
			r.Backdrop = &domain.BackdropRecipe{ClipSec: 60, Channels: []domain.BackdropChannelLink{{ChannelID: "twitch:a"}}}
		}, "backdrop.channels[0].kinds"},
		{"kinds sai nền tảng", func(r *domain.Recipe) {
			r.Backdrop = &domain.BackdropRecipe{ClipSec: 60, Channels: []domain.BackdropChannelLink{{ChannelID: "bilibili:1", Kinds: []string{"clip"}}}}
		}, "backdrop.channels[0].kinds"},
		{"video lẻ sai mã", func(r *domain.Recipe) {
			r.Backdrop = &domain.BackdropRecipe{ClipSec: 60, Channels: []domain.BackdropChannelLink{{ChannelID: "twitch:a", Kinds: []string{"vod"}}}, FixedVideoIDs: []string{"x"}}
		}, "backdrop.fixedVideoIds[0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := valid
			tc.mutate(&r)
			err := ValidateRecipe(r)
			if err == nil {
				t.Fatalf("phải báo lỗi %s", tc.field)
			}
			var apiErr *Error
			if !asError(err, &apiErr) {
				t.Fatalf("loại lỗi = %T", err)
			}
			if apiErr.Code != CodeInvalidInput {
				t.Fatalf("code = %s", apiErr.Code)
			}
			found := false
			for _, f := range apiErr.Fields {
				if f.Field == tc.field {
					found = true
					if f.Message == "" || f.Recovery == "" {
						t.Fatalf("%s thiếu câu lỗi/phục hồi: %+v", f.Field, f)
					}
				}
			}
			if !found {
				t.Fatalf("thiếu trường %s trong %+v", tc.field, apiErr.Fields)
			}
		})
	}
}

func TestValidateRecipeBoundariesPass(t *testing.T) {
	r := DefaultRecipe()
	r.Scenes.Images.Count = 60
	r.Scenes.Motion.Intensity = 0.25
	r.Scenes.Motion.Speed = 3
	r.Scenes.SceneOpacity = 0
	r.Scenes.Framing = FramingClose
	r.Scenes.Style = StyleCutout
	r.BackgroundAppearance.BlurRadius = 50
	r.BackgroundAppearance.Brightness = -1
	r.BackgroundAppearance.Contrast = 3
	r.Backdrop = &domain.BackdropRecipe{ClipSec: 10}
	r.Backdrop.Channels = make([]domain.BackdropChannelLink, 20)
	for i := range r.Backdrop.Channels {
		r.Backdrop.Channels[i] = domain.BackdropChannelLink{ChannelID: "twitch:chan" + string(rune('a'+i)), Kinds: []string{"clip"}}
	}
	if err := ValidateRecipe(r); err != nil {
		t.Fatalf("biên phải hợp lệ: %v", err)
	}
}

// TestParseRecipeAppliesDefaults kiểm JSON vắng trường giữ mặc định và field
// lạ bị từ chối.
func TestParseRecipeAppliesDefaults(t *testing.T) {
	base := DefaultRecipe()
	parsed, err := ParseRecipe([]byte(`{"scenes":{"style":"cutout","images":{"mode":"fixed","count":9}}}`), base)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scenes.Style != StyleCutout || parsed.Scenes.Images.Count != 9 {
		t.Fatalf("scenes = %+v", parsed.Scenes)
	}
	if parsed.Scenes.Motion.Kind != MotionKenBurns || parsed.Output.CompressExport != true {
		t.Fatalf("mặc định bị mất: %+v", parsed)
	}
	if _, err := ParseRecipe([]byte(`{"scenes":{"thumbnail":"clean"}}`), base); err == nil {
		t.Fatal("field lạ phải bị từ chối")
	}
	if _, err := ParseRecipe([]byte(`{`), base); err == nil {
		t.Fatal("JSON hỏng phải bị từ chối")
	}
}

func TestParseRecipeBackdropNullAndDedupe(t *testing.T) {
	base := DefaultRecipe()
	parsed, err := ParseRecipe([]byte(`{"backdrop":null,"overlays":["a","a","b"]}`), base)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Backdrop != nil || len(parsed.Overlays) != 2 {
		t.Fatalf("backdrop/overlays = %+v %+v", parsed.Backdrop, parsed.Overlays)
	}
	parsed, err = ParseRecipe([]byte(`{"backdrop":{"channels":[{"channelId":"twitch:a","kinds":["vod","vod"]}],"clipSec":30}}`), base)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Backdrop == nil || parsed.Backdrop.ClipSec != 30 || len(parsed.Backdrop.Channels[0].Kinds) != 1 {
		t.Fatalf("backdrop = %+v", parsed.Backdrop)
	}
}

func TestCheckRefs(t *testing.T) {
	store := newMemStore()
	store.prompts["11111111-1111-4111-8111-111111111111"] = domain.ImagePrompt{
		ID: "11111111-1111-4111-8111-111111111111", Name: "Của tôi", Kind: domain.PromptCover, Composition: "hook",
	}
	store.overlays["22222222-2222-4222-8222-222222222222"] = domain.Overlay{ID: "22222222-2222-4222-8222-222222222222", SHA256: strings.Repeat("a", 64)}
	store.backdrop["twitch:abc"] = true
	store.bdVideos["twitch:abc"] = map[string]bool{"twitch:1": true}
	store.fixed["twitch:9"] = true
	ctx := context.Background()
	checker := refChecker{store: store}

	ok := DefaultRecipe()
	ok.Scenes.ScenePromptID = BuiltinScene
	ok.Scenes.CoverPromptID = BuiltinCoverHook
	ok.Overlays = []string{"22222222-2222-4222-8222-222222222222"}
	ok.Backdrop = &domain.BackdropRecipe{ClipSec: 60, Channels: []domain.BackdropChannelLink{{ChannelID: "twitch:abc", Kinds: []string{"vod"}, Pinned: []string{"twitch:1"}}}}
	if err := CheckRefs(ctx, checker, ok); err != nil {
		t.Fatalf("hợp lệ: %v", err)
	}

	bad := ok
	bad.Scenes.ScenePromptID = BuiltinCoverHook // sai loại
	bad.Scenes.CoverPromptID = "33333333-3333-4333-8333-333333333333"
	bad.Overlays = []string{"44444444-4444-4444-8444-444444444444"}
	bad.Backdrop = &domain.BackdropRecipe{ClipSec: 60, Channels: []domain.BackdropChannelLink{
		{ChannelID: "twitch:nope", Kinds: []string{"vod"}, Pinned: []string{"twitch:2"}},
	}, FixedVideoIDs: []string{"twitch:404"}}
	err := CheckRefs(ctx, checker, bad)
	if err == nil {
		t.Fatal("phải báo lỗi tham chiếu")
	}
	var apiErr *Error
	if !asError(err, &apiErr) || len(apiErr.Fields) < 5 {
		t.Fatalf("lỗi = %+v", err)
	}
	fields := make(map[string]bool)
	for _, f := range apiErr.Fields {
		fields[f.Field] = true
	}
	for _, want := range []string{"scenes.scenePromptId", "scenes.coverPromptId", "overlays[0]", "backdrop.channels", "backdrop.channels[0].pinned", "backdrop.fixedVideoIds[0]"} {
		if !fields[want] {
			t.Fatalf("thiếu %s trong %+v", want, apiErr.Fields)
		}
	}
}

func TestImageCountFor(t *testing.T) {
	if got := ImageCountFor(domain.ImagesMode{Mode: ImagesNone}, 3600); got != 0 {
		t.Fatalf("none = %d", got)
	}
	if got := ImageCountFor(domain.ImagesMode{Mode: ImagesFixed, Count: 5}, 3600); got != 5 {
		t.Fatalf("fixed = %d", got)
	}
	if got := ImageCountFor(domain.ImagesMode{Mode: ImagesPerMinutes, Minutes: 2}, 600); got != 5 {
		t.Fatalf("10 phút / 2 = %d", got)
	}
	if got := ImageCountFor(domain.ImagesMode{Mode: ImagesPerMinutes, Minutes: 1}, 1); got != 1 {
		t.Fatalf("tối thiểu = %d", got)
	}
	if got := ImageCountFor(domain.ImagesMode{Mode: ImagesPerMinutes, Minutes: 1}, 10*3600); got != 60 {
		t.Fatalf("trần = %d", got)
	}
}

func TestBuiltinPromptsShape(t *testing.T) {
	list := BuiltinPrompts()
	if len(list) != 7 {
		t.Fatalf("số prompt có sẵn = %d", len(list))
	}
	if list[0].ID != BuiltinScene || list[0].Kind != domain.PromptScene || list[0].Composition != "" {
		t.Fatalf("scene = %+v", list[0])
	}
	seen := map[string]bool{BuiltinScene: true}
	for _, p := range list[1:] {
		if !p.BuiltIn || p.Kind != domain.PromptCover {
			t.Fatalf("bìa = %+v", p)
		}
		if !contains(domain.ThumbnailStyles(), p.Composition) {
			t.Fatalf("bố cục lạ %q", p.Composition)
		}
		if p.ID != "builtin:cover-"+p.Composition {
			t.Fatalf("id %q không khớp bố cục %q", p.ID, p.Composition)
		}
		if seen[p.ID] || p.Name == "" || p.Prompt == "" {
			t.Fatalf("prompt có sẵn trùng/rỗng: %+v", p)
		}
		seen[p.ID] = true
	}
}

// TestBuiltinAssetsMatchManifest đối chiếu manifest với tệp nhúng: sha256 và
// danh sách 12 hiệu ứng.
func TestBuiltinAssetsMatchManifest(t *testing.T) {
	effects, err := BuiltinEffects()
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 12 {
		t.Fatalf("số hiệu ứng = %d", len(effects))
	}
	for _, effect := range effects {
		if len(effect.SHA) != 64 {
			t.Fatalf("sha lạ: %+v", effect)
		}
		if _, err := builtinOverlayBytes(effect); err != nil {
			t.Fatalf("%s: %v", effect.ID, err)
		}
		if effect.Name == "" || strings.HasSuffix(effect.Name, ".mp4") {
			t.Fatalf("tên hiển thị = %q", effect.Name)
		}
	}
	files, err := builtinAssetFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 12 {
		t.Fatalf("tệp mp4 nhúng = %v", files)
	}
}

// asError là errors.As gọn cho *Error trong test.
func asError(err error, target **Error) bool {
	if e, ok := err.(*Error); ok {
		*target = e
		return true
	}
	return false
}

// ensure json import used (marshal recipe round-trip).
var _ = json.Marshal
