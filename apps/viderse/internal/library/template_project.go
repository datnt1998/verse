package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

type standardAsset struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func standardAssets() ([]standardAsset, error) {
	data, err := builtinAssets.ReadFile("assets/standard-template/manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Files []standardAsset `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if len(manifest.Files) != 4 {
		return nil, fmt.Errorf("standard template must contain four assets")
	}
	return manifest.Files, nil
}

func standardAssetID(name string) string {
	return "standard-" + name[:len(name)-len(filepath.Ext(name))]
}
func standardAssetPath(name string) string { return "library/standard-template/" + name }

// publishStandardAssets and the row write share the media publication's DB
// transaction. A failed backfill never exposes a project without its assets.
func (s *Service) publishStandardAssets(ctx context.Context, commit func(context.Context) error) error {
	// Metadata-only stores (including CLI unit stores) still own the same real
	// project. A configured production media service publishes all source bytes.
	if s.media == nil {
		return commit(ctx)
	}
	assets, err := standardAssets()
	if err != nil {
		return err
	}
	return s.publishStandardAssetFiles(ctx, assets, commit)
}

func (s *Service) publishStandardAssetFiles(ctx context.Context, assets []standardAsset, commit func(context.Context) error) error {
	dir, err := os.MkdirTemp("", "viderse-standard-template-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	artifacts := make([]media.Artifact, 0, len(assets))
	for _, asset := range assets {
		data, err := builtinAssets.ReadFile("assets/standard-template/" + asset.Name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != asset.SHA256 || int64(len(data)) != asset.Size {
			return fmt.Errorf("bundled standard template asset changed: %s", asset.Name)
		}
		file := filepath.Join(dir, asset.Name)
		if err := os.WriteFile(file, data, 0600); err != nil {
			return err
		}
		mime := "image/webp"
		if filepath.Ext(asset.Name) == ".wav" {
			mime = "audio/wav"
		}
		if filepath.Ext(asset.Name) == ".mp4" {
			mime = "video/mp4"
		}
		artifacts = append(artifacts, media.Artifact{Path: standardAssetPath(asset.Name), File: file, Filename: asset.Name, ContentType: mime})
	}
	return s.media.Publish(ctx, artifacts, commit)
}

func (s *Service) standardProject(ctx context.Context, id, name string, created time.Time, recipe domain.Recipe) (json.RawMessage, error) {
	project, err := editor.BuildStandardTemplateProject(id, name, created)
	if err != nil {
		return nil, err
	}
	data, err := editor.Marshal(project)
	if err != nil {
		return nil, err
	}
	// Untouched standard templates retain the byte-for-byte canonical layout;
	// older rows' settings become track overrides without losing that layout.
	applyRecipe := !reflect.DeepEqual(recipe.Scenes, StandardRecipe().Scenes) || recipe.Backdrop != nil || recipe.BackgroundAppearance != DefaultAppearance() || !recipe.Output.CompressExport
	cover, err := s.resolvePrompt(ctx, recipe.Scenes.CoverPromptID, BuiltinCoverClean, "scenes.coverPromptId")
	if err != nil {
		return nil, err
	}
	return updateTemplateProject(data, name, recipe, cover.Composition, applyRecipe, created)
}

func (s *Service) ensureTemplateProject(ctx context.Context, tpl *domain.Template) (*domain.Template, error) {
	for len(tpl.Project) == 0 {
		project, err := s.standardProject(ctx, tpl.ID, tpl.Name, tpl.CreatedAt, tpl.Recipe)
		if err != nil {
			return nil, err
		}
		var saved bool
		err = s.publishStandardAssets(ctx, func(txctx context.Context) error {
			var err error
			saved, err = s.store.InitializeTemplateProject(txctx, tpl.ID, project, tpl.Revision)
			return err
		})
		if err != nil {
			return nil, err
		}
		if saved {
			tpl.Project = project
			break
		}
		// A concurrent save/backfill won. Read its authored data rather than
		// returning our candidate or overwriting it on a later snapshot.
		tpl, err = s.store.GetTemplate(ctx, tpl.ID)
		if err != nil {
			return nil, err
		}
		if tpl == nil {
			return nil, notFound("Mẫu video")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	recipe, err := s.projectTemplateRecipe(ctx, tpl.Project, tpl.Recipe)
	if err != nil {
		return nil, err
	}
	tpl.Recipe = recipe
	return tpl, nil
}

func validateTemplateProject(raw json.RawMessage, id string, created time.Time) (json.RawMessage, error) {
	project, err := editor.Parse(raw)
	if err != nil {
		return nil, invalidInput("Project mẫu video không hợp lệ", "Mở lại mẫu video rồi lưu")
	}
	version, _ := project["schemaVersion"].(float64)
	metadata, timeline := project.Metadata(), project.Timeline()
	validNumber := func(key string) bool { value, ok := metadata[key].(float64); return ok && value > 0 }
	_, tracks := timeline["tracks"].([]any)
	_, items := timeline["items"].([]any)
	if version != editor.SchemaVersion || !validNumber("width") || !validNumber("height") || !validNumber("fps") || !tracks || !items {
		return nil, invalidInput("Project mẫu video không đúng schema 15", "Mở lại mẫu video rồi lưu")
	}
	project["id"], project["createdAt"] = id, created.UnixMilli()
	return editor.Marshal(project)
}

func duplicateTemplateProject(raw json.RawMessage, id, name string, now time.Time) (json.RawMessage, error) {
	project, err := editor.Parse(raw)
	if err != nil {
		return nil, err
	}
	project["id"], project["name"] = id, name
	project["createdAt"], project["updatedAt"] = now.UnixMilli(), now.UnixMilli()
	return editor.Marshal(project)
}

// updateTemplateProject changes only the settings-owned fields; timeline
// edits, nested compositions, extra tracks and future engine fields survive.
func updateTemplateProject(raw json.RawMessage, name string, recipe domain.Recipe, thumbnail string, applyRecipe bool, now time.Time) (json.RawMessage, error) {
	project, err := editor.Parse(raw)
	if err != nil {
		return nil, err
	}
	project["name"], project["updatedAt"] = name, now.UnixMilli()
	if !applyRecipe {
		return editor.Marshal(project)
	}
	timeline := project.Timeline()
	tracks, _ := timeline["tracks"].([]any)
	var scenes, background map[string]any
	for _, value := range tracks {
		track, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if track["templateSlot"] == "scenes" && scenes == nil {
			scenes = track
		}
		if track["templateSlot"] == "background-video" && background == nil {
			background = track
		}
	}
	if scenes == nil {
		return nil, invalidInput("Mẫu video chưa có track Ảnh cảnh", "Mở trình chỉnh sửa và gán vai trò Ảnh cảnh")
	}
	settings, _ := scenes["templateScenes"].(map[string]any)
	if settings == nil {
		settings = map[string]any{}
	}
	style := recipe.Scenes.Style
	if style == StyleCutout && recipe.Scenes.Framing == "close" {
		style = "cutout-close"
	}
	// Keep engine settings as JSON objects, like the parsed authored project.
	// Struct-valued settings serialize in declaration order, then reorder on
	// the next parse/marshal and change the persisted project's byte identity.
	images := map[string]any{"mode": recipe.Scenes.Images.Mode}
	if recipe.Scenes.Images.Count != 0 {
		images["count"] = recipe.Scenes.Images.Count
	}
	if recipe.Scenes.Images.Minutes != 0 {
		images["minutes"] = recipe.Scenes.Images.Minutes
	}
	motion := recipe.Scenes.Motion
	settings["style"], settings["images"], settings["motion"] = style, images, map[string]any{"kind": motion.Kind, "intensity": motion.Intensity, "speed": motion.Speed}
	settings["thumbnail"] = thumbnail
	settings["opacity"] = recipe.Scenes.SceneOpacity
	if recipe.Scenes.CoverMediaID == "" {
		delete(settings, "coverMediaId")
	} else {
		settings["coverMediaId"] = recipe.Scenes.CoverMediaID
	}
	if recipe.Output.CompressExport {
		delete(settings, "compress")
	} else {
		settings["compress"] = false
	}
	scenes["templateScenes"] = settings
	if background == nil && (recipe.Backdrop != nil || recipe.BackgroundAppearance != DefaultAppearance()) {
		audioIndex := -1
		used := make(map[string]bool, len(tracks))
		for i, value := range tracks {
			if track, ok := value.(map[string]any); ok {
				if id, ok := track["id"].(string); ok {
					used[id] = true
				}
				if track["templateSlot"] == "source-audio" && audioIndex < 0 {
					audioIndex = i
				}
			}
		}
		if audioIndex < 0 {
			return nil, invalidInput("Mẫu video chưa có track Âm thanh gốc", "Mở trình chỉnh sửa và gán vai trò Âm thanh gốc cho một track")
		}
		id := "standard-background"
		for n := 1; used[id]; n++ {
			id = fmt.Sprintf("standard-background-%d", n)
		}
		background = map[string]any{"id": id, "name": "Nền video", "kind": "video", "templateSlot": "background-video", "visible": true, "muted": false, "locked": false, "solo": false, "height": 100, "items": []any{}}
		tracks = append(tracks, nil)
		copy(tracks[audioIndex+1:], tracks[audioIndex:])
		tracks[audioIndex] = background
		for i, value := range tracks {
			if track, ok := value.(map[string]any); ok {
				track["order"] = i
			}
		}
		timeline["tracks"] = tracks
	}
	if background != nil {
		if recipe.Backdrop == nil {
			delete(background, "templateBackdrop")
		} else {
			backdrop := recipe.Backdrop
			channels := make([]map[string]any, len(backdrop.Channels))
			for i, channel := range backdrop.Channels {
				channels[i] = map[string]any{"channelId": channel.ChannelID, "kinds": channel.Kinds, "pinned": channel.Pinned}
			}
			background["templateBackdrop"] = map[string]any{"channels": channels, "clipCount": backdrop.ClipCount, "clipSec": backdrop.ClipSec, "fixedVideoIds": backdrop.FixedVideoIDs}
		}
		appearance := recipe.BackgroundAppearance
		background["templateBackgroundAppearance"] = map[string]any{
			"opacity": appearance.Opacity, "blurRadius": appearance.BlurRadius,
			"brightness": appearance.Brightness, "contrast": appearance.Contrast,
			"saturation": appearance.Saturation, "hueShift": appearance.HueShift,
			"speed": appearance.Speed, "zoom": appearance.Zoom,
			"flipX": appearance.FlipX, "flipY": appearance.FlipY,
		}
	}
	return editor.Marshal(project)
}
