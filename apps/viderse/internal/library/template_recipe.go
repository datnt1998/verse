package library

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"unicode/utf16"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

// The project owns engine settings; the row retains library-only choices such
// as prompt references, reference-image policy and shared overlay selections.
// This is resolveTemplateRecipe from yt-factory 2c201e6, projected onto the
// library wire contract (cutout-close becomes cutout plus close framing).
func (s *Service) projectTemplateRecipe(ctx context.Context, raw json.RawMessage, base domain.Recipe) (domain.Recipe, error) {
	project, err := editor.Parse(raw)
	if err != nil {
		return domain.Recipe{}, err
	}
	var tracks []struct {
		Slot       string          `json:"templateSlot"`
		Scenes     json.RawMessage `json:"templateScenes"`
		Backdrop   json.RawMessage `json:"templateBackdrop"`
		Appearance json.RawMessage `json:"templateBackgroundAppearance"`
	}
	data, err := json.Marshal(project.Timeline()["tracks"])
	if err != nil {
		return domain.Recipe{}, err
	}
	if err := json.Unmarshal(data, &tracks); err != nil {
		return domain.Recipe{}, invalidTemplateSettings()
	}
	defaults := StandardRecipe()
	next := base
	next.Scenes.Style, next.Scenes.Framing = defaults.Scenes.Style, ""
	next.Scenes.Images, next.Scenes.Motion = defaults.Scenes.Images, defaults.Scenes.Motion
	next.Scenes.SceneOpacity, next.Scenes.CoverMediaID = 1, ""
	next.Output.CompressExport = true
	next.Backdrop, next.BackgroundAppearance = nil, DefaultAppearance()
	thumbnail := "clean"
	seenScenes, seenBackground := false, false
	for _, track := range tracks {
		if track.Slot == "scenes" && !seenScenes {
			seenScenes = true
			if len(track.Scenes) == 0 || bytes.Equal(track.Scenes, []byte("null")) {
				continue
			}
			var settings struct {
				Style        string               `json:"style"`
				Images       *domain.ImagesMode   `json:"images"`
				Motion       *domain.MotionRecipe `json:"motion"`
				Thumbnail    *string              `json:"thumbnail"`
				ImagePrompt  *string              `json:"imagePrompt"`
				Opacity      *float64             `json:"opacity"`
				CoverMediaID *string              `json:"coverMediaId"`
				Compress     *bool                `json:"compress"`
			}
			decoder := json.NewDecoder(bytes.NewReader(track.Scenes))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&settings); err != nil || settings.Images == nil || settings.Motion == nil {
				return domain.Recipe{}, invalidTemplateSettings()
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(track.Scenes, &fields); err != nil {
				return domain.Recipe{}, invalidTemplateSettings()
			}
			for _, value := range fields {
				if bytes.Equal(value, []byte("null")) {
					return domain.Recipe{}, invalidTemplateSettings()
				}
			}
			var imageFields map[string]json.RawMessage
			if err := json.Unmarshal(fields["images"], &imageFields); err != nil {
				return domain.Recipe{}, invalidTemplateSettings()
			}
			expectedImageFields := 2
			if settings.Images.Mode == ImagesNone {
				expectedImageFields = 1
			}
			if len(imageFields) != expectedImageFields {
				return domain.Recipe{}, invalidTemplateSettings()
			}
			if _, present := imageFields["minutes"]; present && settings.Images.Mode != ImagesPerMinutes {
				return domain.Recipe{}, invalidTemplateSettings()
			}
			if _, present := imageFields["count"]; present && settings.Images.Mode != ImagesFixed {
				return domain.Recipe{}, invalidTemplateSettings()
			}
			next.Scenes.Style = settings.Style
			if settings.Style == "cutout-close" {
				next.Scenes.Style, next.Scenes.Framing = StyleCutout, FramingClose
			}
			next.Scenes.Images, next.Scenes.Motion = *settings.Images, *settings.Motion
			if settings.Thumbnail != nil {
				thumbnail = *settings.Thumbnail
			}
			if settings.Opacity != nil {
				next.Scenes.SceneOpacity = *settings.Opacity
			}
			if settings.CoverMediaID != nil {
				if *settings.CoverMediaID == "" {
					return domain.Recipe{}, invalidTemplateSettings()
				}
				next.Scenes.CoverMediaID = *settings.CoverMediaID
			}
			if settings.Compress != nil {
				if *settings.Compress {
					return domain.Recipe{}, invalidTemplateSettings()
				}
				next.Output.CompressExport = false
			}
			if settings.ImagePrompt != nil {
				length := len(utf16.Encode([]rune(*settings.ImagePrompt)))
				if strings.TrimSpace(*settings.ImagePrompt) != *settings.ImagePrompt || length == 0 || length > 2000 {
					return domain.Recipe{}, invalidTemplateSettings()
				}
			}
		}
		if track.Slot == "background-video" && !seenBackground {
			seenBackground = true
			if len(track.Backdrop) > 0 {
				backdrop, err := parseBackdrop(track.Backdrop)
				if err != nil {
					return domain.Recipe{}, err
				}
				next.Backdrop = backdrop
			}
			if len(track.Appearance) > 0 && !bytes.Equal(track.Appearance, []byte("null")) {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(track.Appearance, &fields); err != nil || len(fields) != 10 {
					return domain.Recipe{}, invalidTemplateSettings()
				}
				for _, value := range fields {
					if bytes.Equal(value, []byte("null")) {
						return domain.Recipe{}, invalidTemplateSettings()
					}
				}
				decoder := json.NewDecoder(bytes.NewReader(track.Appearance))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&next.BackgroundAppearance); err != nil {
					return domain.Recipe{}, invalidTemplateSettings()
				}
			}
		}
	}
	// Keep a selected custom prompt when its composition matches the project.
	// An editor thumbnail change instead selects the corresponding built-in.
	cover, ok := BuiltinPrompt(base.Scenes.CoverPromptID)
	if !ok && base.Scenes.CoverPromptID != "" {
		custom, err := s.store.GetPrompt(ctx, base.Scenes.CoverPromptID)
		if err != nil {
			return domain.Recipe{}, err
		}
		if custom == nil {
			return domain.Recipe{}, invalidFields("Template trỏ tới prompt ảnh không còn", []FieldError{{"scenes.coverPromptId", "Prompt ảnh không còn trong thư viện", "Chọn prompt khác cho template rồi lưu lại"}})
		}
		cover = *custom
	}
	if cover.Composition != thumbnail {
		found := false
		for _, prompt := range BuiltinPrompts() {
			if prompt.Kind == domain.PromptCover && prompt.Composition == thumbnail {
				next.Scenes.CoverPromptID, found = prompt.ID, true
				break
			}
		}
		if !found {
			return domain.Recipe{}, invalidTemplateSettings()
		}
	}
	if err := ValidateRecipe(next); err != nil {
		return domain.Recipe{}, err
	}
	return next, nil
}

func invalidTemplateSettings() error {
	return invalidInput("Cấu hình track template không hợp lệ", "Mở lại template và kiểm tra cấu hình ảnh cùng video nền")
}
