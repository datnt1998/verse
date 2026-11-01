package build

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/platform/blobstore"
)

func plainDigest(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func artifactPath(value string) bool {
	return value != "" && value != "." && !strings.ContainsAny(value, "\\\x00") && !path.IsAbs(value) && path.Clean(value) == value && value != ".." && !strings.HasPrefix(value, "../")
}

func templateMediaPath(id, file string) bool {
	return artifactPath(file) && (strings.HasPrefix(file, "editor/media/"+id+"/") || strings.HasPrefix(file, "library/standard-template/") || strings.HasPrefix(file, "library/overlays/"))
}

func templateStale(message string) error {
	return steps.CloneError(domain.CloneErrStale, message, "Media trong template đã bị thay — làm lại job để dùng bản mới")
}

func verifyTemplateMedia(ctx context.Context, s *pipeline.StepContext, pin library.TemplateMediaBinding, file string) error {
	// Uploaded media may be replaced under the same id while its old source
	// file still exists. Check the live index as well as the pinned bytes.
	// Built-in template assets are immutable library files, without an index.
	if strings.HasPrefix(file, "editor/media/"+pin.MediaID+"/") {
		metadata, err := s.Acquire(ctx, "editor/media/"+pin.MediaID+"/metadata.json")
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, blobstore.ErrNotFound) {
				return steps.CloneError(domain.CloneErrStale, "Media trong template đã bị xoá: "+pin.MediaID, "Media trong template đã bị xoá — thêm lại media hoặc sửa template")
			}
			return err
		}
		data, readErr := os.ReadFile(metadata.Path)
		metadata.Release()
		if readErr != nil {
			return readErr
		}
		var record struct {
			ContentHash string `json:"contentHash"`
		}
		if err := json.Unmarshal(data, &record); err != nil {
			return templateStale("Metadata media của template không hợp lệ: " + pin.MediaID)
		}
		if record.ContentHash != pin.ContentHash {
			return templateStale("Media trong template đã bị thay đổi: " + pin.MediaID)
		}
	}
	handle, err := s.Acquire(ctx, file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, blobstore.ErrNotFound) {
			return steps.CloneError(domain.CloneErrStale, "Media trong template đã bị xoá: "+pin.MediaID, "Media trong template đã bị xoá — thêm lại media hoặc sửa template")
		}
		return err
	}
	defer handle.Release()
	hash, err := steps.SHA256File(handle.Path)
	if err != nil {
		return err
	}
	if hash != strings.TrimPrefix(pin.ContentHash, "sha256:") {
		return templateStale("Media trong template đã bị thay đổi: " + pin.MediaID)
	}
	return nil
}

// SnapshotTemplate signs the editor's canonical JSON, not JSONB's formatting.
// Reconstruct those bytes before checking either project or media-links pins.
func validateTemplateSnapshot(snap library.Snapshot, project editor.Project, templateID string) (string, error) {
	binding := snap.Binding
	if binding.ProjectID == "" || snap.TemplateID != binding.ProjectID || project["id"] != binding.ProjectID || (templateID != "" && templateID != binding.ProjectID) {
		return "", steps.CloneError(domain.CloneErrTemplateChanged, "Template không khớp binding đã chốt", "Chọn lại template để dựng")
	}
	projectBytes, err := editor.Marshal(project)
	if err != nil {
		return "", err
	}
	digest := strings.TrimPrefix(editor.Revision(projectBytes), "sha256:")
	if !plainDigest(binding.SHA256) || binding.SHA256 != digest || binding.Revision != "sha256:"+digest {
		return "", templateStale("Snapshot template đã thay đổi sau khi chốt")
	}
	linksBytes, err := json.MarshalIndent(snap.MediaLinks, "", "  ")
	if err != nil {
		return "", err
	}
	if snap.MediaLinks.Version != "1.0" || snap.MediaLinks.MediaIDs == nil || !plainDigest(binding.LinksSHA256) || editor.Revision(append(linksBytes, '\n')) != "sha256:"+binding.LinksSHA256 {
		return "", templateStale("Liên kết media của snapshot template đã thay đổi")
	}
	pins := make(map[string]bool, len(binding.Media))
	for _, pin := range binding.Media {
		if _, err := editor.Path(pin.MediaID); err != nil {
			return "", templateStale("Binding media của template không hợp lệ")
		}
		if pins[pin.MediaID] || !strings.HasPrefix(pin.ContentHash, "sha256:") || !plainDigest(strings.TrimPrefix(pin.ContentHash, "sha256:")) || !templateMediaPath(pin.MediaID, snap.MediaFiles[pin.MediaID]) {
			return "", templateStale("Binding media của template không hợp lệ")
		}
		pins[pin.MediaID] = true
	}
	if len(snap.MediaFiles) != len(pins) || len(snap.MediaLinks.MediaIDs) != len(pins) {
		return "", templateStale("Snapshot template thiếu media đã chốt")
	}
	linked := make(map[string]bool, len(pins))
	for _, link := range snap.MediaLinks.MediaIDs {
		if !pins[link.ID] || linked[link.ID] {
			return "", templateStale("Liên kết media không khớp binding template")
		}
		linked[link.ID] = true
	}
	refs := map[string]bool{}
	var collect func(any)
	collect = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				if key == "mediaId" || key == "coverMediaId" {
					if id, ok := child.(string); ok && id != "" {
						refs[id] = true
					}
				} else {
					collect(child)
				}
			}
		case []any:
			for _, child := range v {
				collect(child)
			}
		}
	}
	collect(project["timeline"])
	if media, ok := project["mediaItems"].([]any); ok {
		for _, value := range media {
			if ref, ok := value.(map[string]any); ok {
				if id, ok := ref["id"].(string); ok && id != "" {
					refs[id] = true
				}
			}
		}
	}
	if snap.CoverMediaID != "" {
		refs[snap.CoverMediaID] = true
	}
	for id := range refs {
		if !pins[id] {
			return "", templateStale("Media của project chưa được chốt trong binding template")
		}
	}
	return digest, nil
}

// validateBuildRecipe compares the recorded scene recipe with the template. A
// video-level images choice may already be baked into the recorded recipe (the
// scenes step resolves it), so the override-resolved recipe is accepted too.
func validateBuildRecipe(s *pipeline.StepContext, manifest scenes.Manifest, override *bool) error {
	expected, err := scenes.ResolveRecipe(s.Job.TemplateSnapshot, s.Job.NoImages, manifest.DurationSec)
	if err != nil {
		return err
	}
	recorded := manifest.Recipe
	if override != nil {
		overridden, err := scenes.ResolveRecipeWithOverride(s.Job.TemplateSnapshot, s.Job.NoImages, override, manifest.DurationSec)
		if err != nil {
			return err
		}
		if overridden.ImagesMode == recorded.ImagesMode && overridden.ImageCount == recorded.ImageCount {
			expected = overridden
		}
	}
	coverChanged := expected.ImagesMode == library.ImagesNone && recorded.CoverMediaID != expected.CoverMediaID
	if expected.Style != recorded.Style || expected.ImagesMode != recorded.ImagesMode || expected.ImageCount != recorded.ImageCount || expected.Thumbnail != recorded.Thumbnail || expected.ImagePrompt != recorded.ImagePrompt || coverChanged {
		return steps.CloneError(domain.CloneErrTemplateChanged, "Template đã đổi số ảnh hoặc kiểu ảnh", "Template đã thay đổi — chạy lại bước Cảnh")
	}
	if math.IsInf(manifest.DurationSec, 0) {
		return fmt.Errorf("invalid scene duration")
	}
	if manifest.Pending != nil {
		return steps.CloneError(domain.CloneErrAwaitingReview, "Cảnh đang chờ duyệt", "Duyệt cảnh trước khi dựng")
	}
	return nil
}

func sameSceneContext(a, b *scenes.TextContext) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// Draft-before-review changes only the approval gate: a completed review
// candidate counts, but pending/running/failed items and in-flight files do not.
func buildImageVersions(manifest images.Manifest, sceneDoc scenes.Manifest, title string) (map[string]images.Version, images.Version, error) {
	stale := func(message string) (map[string]images.Version, images.Version, error) {
		return nil, images.Version{}, steps.CloneError(domain.CloneErrStale, message, "Vẽ lại ảnh theo bản cảnh hiện tại")
	}
	waiting := func() (map[string]images.Version, images.Version, error) {
		return nil, images.Version{}, steps.CloneError(domain.CloneErrAwaitingReview, "Ảnh chưa hoàn tất", "Hoàn tất ảnh trước khi dựng")
	}
	if manifest.Version != 1 || manifest.Revision < 1 {
		return stale("Manifest ảnh không hợp lệ")
	}
	sceneOverride := manifest.Overrides != nil && manifest.Overrides.ScenePromptSet
	coverOverride := manifest.Overrides != nil && manifest.Overrides.CoverPromptSet
	if manifest.SceneRevision != sceneDoc.Revision || manifest.Style != sceneDoc.Recipe.Style || !coverOverride && manifest.Thumbnail != sceneDoc.Recipe.Thumbnail || !sceneOverride && manifest.TemplatePrompt != sceneDoc.Recipe.ImagePrompt {
		return stale("Ảnh không thuộc bản cảnh và kiểu ảnh hiện tại")
	}
	none := sceneDoc.Recipe.ImagesMode == library.ImagesNone
	if manifest.Overrides != nil && manifest.Overrides.NoImages != nil {
		none = *manifest.Overrides.NoImages
	}
	if none != (manifest.ImageMode == "none") && manifest.Overrides != nil && manifest.Overrides.NoImages != nil {
		return stale("Ngoại lệ ảnh không khớp chế độ tạo ảnh của video")
	}
	expected := make(map[string]scenes.Scene, len(sceneDoc.Scenes))
	for _, scene := range sceneDoc.Scenes {
		expected[scene.SceneID] = scene
	}
	seen := make(map[string]bool, len(manifest.Items))
	byScene := make(map[string]images.Version, len(sceneDoc.Scenes))
	thumbnail := images.Version{}
	thumbnails, sceneCount := 0, 0
	for _, item := range manifest.Items {
		if item.ID == "" || seen[item.ID] {
			return stale("Manifest ảnh trùng hoặc thiếu định danh")
		}
		seen[item.ID] = true
		switch item.Kind {
		case "scene":
			scene, ok := expected[item.SceneID]
			if none || !ok || item.ID != item.SceneID || item.StartFrame != scene.StartFrame || item.EndFrame != scene.EndFrame || item.Reference != scene.Keyframe || item.Narration != scene.Text || !sameSceneContext(item.Context, scene.Context) || item.Width != 1920 || item.Height != 1080 {
				return stale("Ảnh không khớp định danh, nội dung, tham chiếu hoặc dải cảnh")
			}
			sceneCount++
		case "thumbnail":
			thumbnails++
			if item.ID != "thumbnail" || item.Narration != title || item.Context != nil || item.Width != 1280 || item.Height != 720 || item.Status == "skipped" {
				return stale("Manifest ảnh bìa không hợp lệ")
			}
		default:
			return stale("Manifest có loại ảnh không hợp lệ")
		}
		if item.InFlight != nil {
			return waiting()
		}
		if item.Kind == "scene" && item.Status == "skipped" {
			continue
		}
		var number *int
		switch item.Status {
		case "review":
			number = item.ReviewVersion
		case "approved":
			number = item.CurrentVersion
		case "pending", "running", "failed":
			return waiting()
		default:
			return stale("Trạng thái ảnh không hợp lệ")
		}
		if number == nil {
			return waiting()
		}
		var chosen *images.Version
		versions := make(map[int]bool, len(item.Versions))
		for index := range item.Versions {
			version := &item.Versions[index]
			if version.Version < 1 || versions[version.Version] {
				return stale("Manifest có version ảnh trùng hoặc không hợp lệ")
			}
			versions[version.Version] = true
			if version.Version == *number {
				chosen = version
			}
		}
		if chosen == nil || chosen.SourceFingerprint != images.SourceFingerprint(item) || chosen.RefHash != item.Reference.SHA256 {
			return stale("Version ảnh không còn khớp đầu vào cảnh")
		}
		if chosen.GenerationID == "" || chosen.CreatedAt == "" || chosen.Mime != "image/webp" || chosen.Width != item.Width || chosen.Height != item.Height || chosen.Bytes <= 0 || !artifactPath(chosen.Path) || !plainDigest(chosen.SHA256) {
			return stale("Version ảnh chưa hoàn tất hoặc sai kích thước/định dạng")
		}
		if _, err := time.Parse(time.RFC3339Nano, chosen.CreatedAt); err != nil {
			return stale("Version ảnh thiếu thời điểm tạo hợp lệ")
		}
		if item.Kind == "scene" {
			if strings.HasPrefix(sceneDoc.Recipe.Style, "cutout") && (chosen.Cutout == nil || !artifactPath(chosen.Cutout.Path) || !plainDigest(chosen.Cutout.SHA256)) {
				return stale("Ảnh cảnh thiếu bản tách nền hợp lệ")
			}
			byScene[item.SceneID] = *chosen
		} else {
			thumbnail = *chosen
		}
	}
	if thumbnails != 1 || (!none && sceneCount != len(expected)) || (none && sceneCount != 0) {
		return stale("Manifest ảnh thiếu ảnh cảnh hoặc ảnh bìa")
	}
	if !none && len(byScene) == 0 {
		return stale("Không thể bỏ qua mọi ảnh cảnh")
	}
	return byScene, thumbnail, nil
}

// Verify every selected file, including skipped neighbours' sources, before
// extracting audio or staging editor imports. Cutout refs have no dimensions in
// the Go manifest, so read their PNG header rather than inventing metadata.
func verifyBuildImage(ctx context.Context, s *pipeline.StepContext, version images.Version, cutout bool) error {
	verify := func(ref scenes.FileRef, size int64, png bool) error {
		handle, err := s.Acquire(ctx, ref.Path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, blobstore.ErrNotFound) {
				return steps.CloneError(domain.CloneErrStale, "Tệp ảnh đã bị xoá: "+ref.Path, "Vẽ lại ảnh")
			}
			return err
		}
		defer handle.Release()
		hash, err := steps.SHA256File(handle.Path)
		if err != nil {
			return err
		}
		if hash != ref.SHA256 {
			return steps.CloneError(domain.CloneErrStale, "Tệp ảnh đã thay đổi: "+ref.Path, "Vẽ lại ảnh")
		}
		info, err := os.Stat(handle.Path)
		if err != nil {
			return err
		}
		if size > 0 && info.Size() != size {
			return steps.CloneError(domain.CloneErrStale, "Kích thước tệp ảnh không khớp manifest", "Vẽ lại ảnh")
		}
		if png {
			file, err := os.Open(handle.Path)
			if err != nil {
				return err
			}
			config, format, decodeErr := image.DecodeConfig(file)
			closeErr := file.Close()
			if decodeErr != nil || format != "png" || config.Width != version.Width || config.Height != version.Height {
				return steps.CloneError(domain.CloneErrStale, "Bản tách nền sai kích thước hoặc định dạng", "Vẽ lại ảnh")
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}
	if err := verify(scenes.FileRef{Path: version.Path, SHA256: version.SHA256}, version.Bytes, false); err != nil {
		return err
	}
	if cutout {
		return verify(*version.Cutout, 0, true)
	}
	return nil
}
