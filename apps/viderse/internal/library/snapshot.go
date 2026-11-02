package library

import (
	"context"
	"encoding/json"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

// SnapshotTemplate là bước "thiết lập chốt lúc bắt đầu": phân giải recipe
// của template thành đúng các trường manifest của một video, để job start giữ
// bản chốt và sửa template sau đó không đổi video đã bắt đầu.
//
// Hình này giữ tên trường cũ của yt-factory nơi nó chép sang manifest:
//   imagePrompt      ← prompt của prompt ảnh cảnh
//   imagePromptMode  ← prompt có sẵn ? "blend" : "custom"
//   thumbnail        ← bố cục của prompt ảnh bìa
//   coverPrompt      ← nội dung prompt ảnh bìa
//
// `style` lấy thẳng từ recipe nên chữ ký buildStoryPrompt/thumbnailPrompt
// không đổi.

// PromptSnapshot là một prompt đã phân giải tại thời điểm chốt.
type PromptSnapshot struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	BuiltIn     bool   `json:"builtIn"`
	Composition string `json:"composition,omitempty"`
	Prompt      string `json:"prompt"`
}

// OverlaySnapshot là một lớp phủ đã phân giải (tệp có thể đã bị tombstone;
// khi đó job bỏ lớp đó đi — template vẫn giữ id).
type OverlaySnapshot struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FilePath string `json:"filePath"`
	SHA256   string `json:"sha256"`
}

// Snapshot là đầu vào chốt của một job start.
type Snapshot struct {
	TemplateID      string                      `json:"templateId"`
	Project         json.RawMessage             `json:"project"`
	Binding         TemplateBinding             `json:"binding"`
	MediaLinks      TemplateMediaLinks          `json:"mediaLinks"`
	MediaFiles      map[string]string           `json:"mediaFiles"`
	TemplateName    string                      `json:"templateName"`
	Revision        int64                       `json:"revision"`
	Style           string                      `json:"style"`
	Framing         string                      `json:"framing,omitempty"`
	ScenePrompt     PromptSnapshot              `json:"scenePrompt"`
	CoverPrompt     PromptSnapshot              `json:"coverPrompt"`
	ImagePrompt     string                      `json:"imagePrompt"`
	ImagePromptMode string                      `json:"imagePromptMode"`
	Thumbnail       string                      `json:"thumbnail"`
	CoverSource     string                      `json:"coverSource"`
	CoverMediaID    string                      `json:"coverMediaId,omitempty"`
	ReferenceImages bool                        `json:"referenceImages"`
	ReferenceStyle  string                      `json:"referenceStyle"`
	Images          domain.ImagesMode           `json:"images"`
	Motion          domain.MotionRecipe         `json:"motion"`
	SceneOpacity    float64                     `json:"sceneOpacity"`
	Overlays        []OverlaySnapshot           `json:"overlays"`
	Backdrop        *domain.BackdropRecipe      `json:"backdrop"`
	Appearance      domain.BackgroundAppearance `json:"appearance"`
	CompressExport  bool                        `json:"compressExport"`
}

// SnapshotTemplate phân giải template thành bản chốt cho job start. Template
// lạ → 404; trỏ tới prompt đã mất → invalid_input (dữ liệu bị sửa tay).
func (s *Service) SnapshotTemplate(ctx context.Context, id string) (*Snapshot, error) {
	tpl, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, notFound("Mẫu video")
	}
	tpl, err = s.ensureTemplateMedia(ctx, tpl)
	if err != nil {
		return nil, err
	}
	project, err := editor.Parse(tpl.Project)
	if err != nil {
		return nil, err
	}
	projectBytes, err := editor.Marshal(project)
	if err != nil {
		return nil, err
	}
	binding, links, files, err := s.bindTemplateMedia(ctx, tpl, project, projectBytes)
	if err != nil {
		return nil, err
	}
	scene, err := s.resolvePrompt(ctx, tpl.Recipe.Scenes.ScenePromptID, BuiltinScene, "scenes.scenePromptId")
	if err != nil {
		return nil, err
	}
	cover, err := s.resolvePrompt(ctx, tpl.Recipe.Scenes.CoverPromptID, BuiltinCoverClean, "scenes.coverPromptId")
	if err != nil {
		return nil, err
	}
	overlays := make([]OverlaySnapshot, 0, len(tpl.Recipe.Overlays))
	for _, overlayID := range tpl.Recipe.Overlays {
		overlay, err := s.store.OverlayByID(ctx, overlayID)
		if err != nil {
			return nil, err
		}
		if overlay == nil {
			// Lớp phủ đã bỏ khỏi thư viện: template vẫn giữ id cho bản sau,
			// job này không có tệp để phủ.
			continue
		}
		overlays = append(overlays, OverlaySnapshot{ID: overlay.ID, Name: overlay.Name, FilePath: overlay.FilePath, SHA256: overlay.SHA256})
	}
	snap := &Snapshot{
		TemplateID:      tpl.ID,
		Project:         append(json.RawMessage(nil), projectBytes...),
		Binding:         binding,
		MediaLinks:      links,
		MediaFiles:      files,
		TemplateName:    tpl.Name,
		Revision:        tpl.Revision,
		Style:           tpl.Recipe.Scenes.Style,
		Framing:         tpl.Recipe.Scenes.Framing,
		ScenePrompt:     promptSnapshot(scene),
		CoverPrompt:     promptSnapshot(cover),
		ImagePrompt:     scene.Prompt,
		CoverSource:     tpl.Recipe.Scenes.CoverSource,
		CoverMediaID:    tpl.Recipe.Scenes.CoverMediaID,
		ReferenceImages: tpl.Recipe.Scenes.ReferenceImages,
		ReferenceStyle:  tpl.Recipe.Scenes.ReferenceStyle,
		Images:          tpl.Recipe.Scenes.Images,
		Motion:          tpl.Recipe.Scenes.Motion,
		SceneOpacity:    tpl.Recipe.Scenes.SceneOpacity,
		Overlays:        overlays,
		Backdrop:        tpl.Recipe.Backdrop,
		Appearance:      tpl.Recipe.BackgroundAppearance,
		CompressExport:  tpl.Recipe.Output.CompressExport,
	}
	// Prompt có sẵn dùng house style của kênh (blend); prompt của bạn thay
	// toàn bộ cách vẽ (custom).
	snap.ImagePromptMode = "custom"
	if scene.BuiltIn {
		snap.ImagePromptMode = "blend"
	}
	// The editor stores owner direction on the scenes track, independently of
	// the library's selected prompt reference. Honour that authored override.
	if tracks, ok := project.Timeline()["tracks"].([]any); ok {
		for _, value := range tracks {
			track, ok := value.(map[string]any)
			if !ok || track["templateSlot"] != "scenes" {
				continue
			}
			if settings, ok := track["templateScenes"].(map[string]any); ok {
				if prompt, ok := settings["imagePrompt"].(string); ok {
					snap.ImagePrompt = prompt
				}
			}
			break
		}
	}
	snap.Thumbnail = cover.Composition
	return snap, nil
}

// resolvePrompt phân giải một id prompt, rỗng thì dùng id có sẵn mặc định.
func (s *Service) resolvePrompt(ctx context.Context, id, fallback, field string) (domain.ImagePrompt, error) {
	if id == "" {
		id = fallback
	}
	p, ok := BuiltinPrompt(id)
	if ok {
		return p, nil
	}
	row, err := s.store.GetPrompt(ctx, id)
	if err != nil {
		return domain.ImagePrompt{}, err
	}
	if row == nil {
		return domain.ImagePrompt{}, invalidFields("Mẫu video trỏ tới chỉ dẫn ảnh không còn", []FieldError{
			{field, "Chỉ dẫn ảnh không còn trong thư viện", "Chọn chỉ dẫn khác cho mẫu video rồi lưu lại"},
		})
	}
	return *row, nil
}

// promptSnapshot đổi prompt đã phân giải sang hình chốt.
func promptSnapshot(p domain.ImagePrompt) PromptSnapshot {
	return PromptSnapshot{
		ID: p.ID, Name: p.Name, Kind: p.Kind, BuiltIn: p.BuiltIn,
		Composition: p.Composition, Prompt: p.Prompt,
	}
}
