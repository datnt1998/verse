package library

// Nhập template cũ: đọc gói `.ytf-template.zip` (hoặc thư mục editor cũ) rồi
// dựng một template Viderse — media vào kho media dùng chung, project giữ
// nguyên timeline schema 15, recipe suy ra từ chính project bằng đúng hàm của
// màn Cài đặt template.
//
// Bảng quy đổi (field của bản xuất cũ → field Viderse):
//
//	project (schema 15)            → templates.project, chỉ đổi id/name/thời gian
//	                                 và mã media; mọi track, keyframe, mốc giữ nguyên
//	media[].id                     → id media mới trong kho; project đổi theo
//	media[].tags/attribution/overlay → metadata của media (editor/media/<id>)
//	media[].fileName/mime/size/sha256 → kiểm nội dung rồi mới nhập
//	templateScenes.style           → scenes.style (cutout-close → cutout + framing close)
//	templateScenes.images/motion   → scenes.images / scenes.motion
//	templateScenes.thumbnail       → scenes.coverPromptId (prompt ảnh bìa có sẵn)
//	templateScenes.imagePrompt     → prompt ảnh cảnh "<tên template> · ảnh"
//	templateScenes.opacity         → scenes.sceneOpacity
//	templateScenes.coverMediaId    → scenes.coverMediaId (media đã nhập)
//	templateScenes.compress        → output.compressExport
//	templateBackdrop               → backdrop (kênh/loại/clip); kênh chưa có ở
//	                                 Viderse bị bỏ và ghi vào warnings
//	templateBackgroundAppearance   → appearance
//
// Trường không có tương đương, ghi vào warnings của kết quả (không bao giờ bỏ
// im lặng): kênh nền / video ghim / video lẻ chưa có trong thư viện Viderse, và
// lớp phủ đi vào project dưới dạng track (không thành mục Thư viện › Lớp phủ).
// `thumbnailId` của project bị bỏ: Viderse không có ảnh thu nhỏ template
// (hỏi đáp #46), thẻ dùng khối màu trung tính.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

// MediaImporter là mặt nhập media mà template cần — hiện thực là
// *editor.Service (cùng ImportFileOnce/UpdateMediaMetadata của editor).
type MediaImporter interface {
	ImportFileOnce(ctx context.Context, opts editor.ImportFileOnceOptions) (editor.MediaRecord, bool, error)
	UpdateMediaMetadata(ctx context.Context, id string, patch editor.Object) (editor.MediaRecord, error)
}

// BindMediaImporter gắn mặt nhập media sau khi dựng Service. Serve dựng service
// Thư viện trước service Editor (editor cần Thư viện để đọc project template),
// nên import template nhận mặt này muộn; CLI gắn ngay khi mở.
func (s *Service) BindMediaImporter(importer MediaImporter) { s.importer = importer }

// ImportResult là kết quả nhập một template. OK=false kèm Error để danh sách
// nhập hàng loạt hiện được từng dòng, không dừng cả lô vì một tệp hỏng.
type ImportResult struct {
	// Source là tên tệp/thư mục đã nhập (rỗng khi nhập từ request body).
	Source string
	// Name là tên template đã đặt trong Viderse (đã tránh trùng tên).
	Name string
	// TemplateID rỗng khi OK=false.
	TemplateID string
	OK         bool
	// Warnings là những gì không quy đổi được sang Viderse, kèm cách xử lý.
	Warnings []string
	// MediaImported/MediaReused đếm media mới nhập và media dùng lại theo nội dung.
	MediaImported int
	MediaReused   int
	// Error là câu lỗi tiếng Việt khi OK=false.
	Error string
}

// ImportOptions là tuỳ chọn nhập một template.
type ImportOptions struct {
	// Name đè tên template trong tệp (rỗng = dùng tên đã xuất).
	Name string
}

// importMediaDir là tên thư mục tạm cho một lần nhập.
const importMediaDirPrefix = "viderse-template-import-"

// ImportTemplateFile nhập một tệp `.ytf-template.zip` hoặc một thư mục project
// cũ. Dùng chung cho route HTTP và `viderse template import`.
func (s *Service) ImportTemplateFile(ctx context.Context, file string, opts ImportOptions) (*ImportResult, error) {
	bundle, err := OpenTemplateBundle(file)
	if err != nil {
		return nil, err
	}
	defer bundle.Close()
	result, err := s.ImportTemplate(ctx, bundle, opts)
	if result != nil {
		result.Source = filepath.Base(file)
	}
	return result, err
}

// ImportTemplateFiles nhập một lô tệp/thư mục. Tệp hỏng không làm hỏng lô: kết
// quả của nó có OK=false và Error. Lỗi trả về chỉ khi không đọc được danh sách.
func (s *Service) ImportTemplateFiles(ctx context.Context, files []string, opts ImportOptions) ([]ImportResult, error) {
	if len(files) == 0 {
		return nil, invalidInput("Chưa chọn tệp mẫu video", "Chọn tệp .ytf-template.zip rồi nhập lại")
	}
	results := make([]ImportResult, 0, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		result, err := s.ImportTemplateFile(ctx, file, opts)
		if err != nil {
			name := filepath.Base(file)
			results = append(results, ImportResult{Source: name, Name: name, Error: errorText(err)})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// errorText trả câu lỗi tiếng Việt của lỗi nghiệp vụ (hoặc lỗi thường).
func errorText(err error) string {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Message
	}
	return err.Error()
}

// ImportTemplate nhập một template đã đọc: nhập media, đổi mã media trong
// project, suy recipe, đặt tên không trùng rồi ghi hàng mới.
//
// Media đã nhập xong mà bước sau hỏng thì để lại trong kho (lần nhập sau dùng
// lại theo nội dung; bộ dọn media thu hồi tệp không còn ai trỏ tới) — đúng
// cách bản xuất cũ xử lý.
func (s *Service) ImportTemplate(ctx context.Context, bundle *TemplateBundle, opts ImportOptions) (*ImportResult, error) {
	if bundle == nil {
		return nil, invalidInput("Không đọc được tệp mẫu video", bundleRecovery())
	}
	if s.importer == nil {
		return nil, invalidInput("Máy chủ chưa sẵn sàng nhập mẫu video", "Khởi động lại Viderse rồi thử lại")
	}
	project, err := editor.Parse(mustMarshalProject(bundle.Project))
	if err != nil {
		return nil, notTemplateFile("Nội dung mẫu video không hợp lệ", bundleRecovery())
	}
	result := &ImportResult{Name: strings.TrimSpace(opts.Name)}
	if result.Name == "" {
		result.Name = bundle.Name
	}

	dir, err := os.MkdirTemp("", importMediaDirPrefix)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	mapping, warnings, imported, reused, err := s.importBundleMedia(ctx, bundle, dir)
	if err != nil {
		return nil, err
	}
	result.Warnings = append(result.Warnings, warnings...)
	result.MediaImported, result.MediaReused = imported, reused

	remapEditorTemplateMedia(project, mapping)
	remapped := project
	now := s.now().UTC()
	templateID := s.newID()
	remapped["id"] = templateID
	remapped["createdAt"] = now.UnixMilli()
	remapped["updatedAt"] = now.UnixMilli()
	delete(remapped, "thumbnailId")

	name, err := s.freeTemplateName(ctx, result.Name)
	if err != nil {
		return nil, err
	}
	remapped["name"] = name
	result.Name = name

	base := StandardRecipe()
	promptID, warning, err := s.importScenePrompt(ctx, name, project)
	if err != nil {
		return nil, err
	}
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}
	if promptID != "" {
		base.Scenes.ScenePromptID = promptID
	}

	raw, err := editor.Marshal(remapped)
	if err != nil {
		return nil, err
	}
	recipe, err := s.projectTemplateRecipe(ctx, raw, base)
	if err != nil {
		return nil, err
	}
	raw, recipe, backdropWarnings, err := s.pruneMissingBackdrop(ctx, remapped, recipe)
	if err != nil {
		return nil, err
	}
	result.Warnings = append(result.Warnings, backdropWarnings...)
	if err := ValidateRecipe(recipe); err != nil {
		return nil, err
	}
	if err := CheckRefs(ctx, refChecker{store: s.store}, recipe); err != nil {
		return nil, err
	}

	tpl := domain.Template{
		ID: templateID, Name: name, Recipe: recipe, Project: raw,
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.InsertTemplate(ctx, tpl); err != nil {
		return nil, err
	}
	s.emit(TopicTemplate, tpl.ID)
	result.TemplateID = tpl.ID
	result.OK = true
	if overlays := overlayTrackCount(bundle.Project); overlays > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%d lớp phủ nằm trong project dưới dạng track (giữ nguyên khi dựng video), không thành mục Thư viện › Lớp phủ", overlays))
	}
	return result, nil
}

// importBundleMedia nhập mọi media của gói và trả bảng đổi mã cũ → mã mới.
func (s *Service) importBundleMedia(ctx context.Context, bundle *TemplateBundle, dir string) (map[string]string, []string, int, int, error) {
	mapping := make(map[string]string, len(bundle.Media))
	var warnings []string
	imported, reused := 0, 0
	for index, media := range bundle.Media {
		if err := ctx.Err(); err != nil {
			return nil, nil, imported, reused, err
		}
		file := filepath.Join(dir, fmt.Sprintf("media-%d%s", index, safeExtension(media.FileName)))
		if err := copyBundleMedia(bundle, index, file); err != nil {
			return nil, nil, imported, reused, err
		}
		record, wasReused, err := s.importer.ImportFileOnce(ctx, editor.ImportFileOnceOptions{
			File:         file,
			FileName:     media.FileName,
			DeclaredMIME: baseMIME(media.MIME),
			ContentHash:  media.ContentHash,
			MIMEPrefix:   mimePrefix(media.MIME),
		})
		_ = os.Remove(file)
		if err != nil {
			return nil, nil, imported, reused, err
		}
		if wasReused {
			reused++
		} else {
			imported++
			if patch := mediaMetadataPatch(media); patch != nil {
				if _, err := s.importer.UpdateMediaMetadata(ctx, record.ID, patch); err != nil {
					warnings = append(warnings, fmt.Sprintf("Không ghi được thẻ của media %s: %s", media.FileName, errorText(err)))
				}
			}
		}
		mapping[media.ID] = record.ID
	}
	return mapping, warnings, imported, reused, nil
}

// mediaMetadataPatch dựng bản vá metadata media từ manifest (tags, nguồn,
// dấu lớp phủ); nil khi manifest không khai gì thêm.
func mediaMetadataPatch(media BundleMedia) editor.Object {
	patch := editor.Object{}
	if len(media.Tags) > 0 {
		patch["tags"] = media.Tags
	}
	if media.Attribution != nil {
		attribution := editor.Object{}
		for key, value := range map[string]*string{
			"provider": &media.Attribution.Provider, "author": media.Attribution.Author,
			"authorUrl": media.Attribution.AuthorURL, "sourceUrl": media.Attribution.SourceURL,
			"sourceId": media.Attribution.SourceID, "license": media.Attribution.License,
		} {
			if value != nil && *value != "" {
				attribution[key] = *value
			}
		}
		if len(attribution) > 0 {
			patch["attribution"] = attribution
		}
	}
	if media.Overlay {
		patch["overlay"] = true
	}
	if len(patch) == 0 {
		return nil
	}
	return patch
}

// freeTemplateName trả tên chưa có template nào dùng, theo luật của hỏi đáp
// #45: giữ nguyên tên, trùng thì "<tên> (nhập)", "(nhập 2)"… Template không có
// ràng buộc tên duy nhất trong DB nên chỗ này là chỗ duy nhất giữ tên sạch.
func (s *Service) freeTemplateName(ctx context.Context, wanted string) (string, error) {
	existing, err := s.store.ListTemplates(ctx)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(existing))
	for _, row := range existing {
		taken[strings.ToLower(strings.TrimSpace(row.Name))] = true
	}
	trimmed, err := validTemplateName(wanted)
	if err != nil {
		return "", err
	}
	if !taken[strings.ToLower(trimmed)] {
		return trimmed, nil
	}
	for attempt := 1; attempt <= 99; attempt++ {
		suffix := " (nhập)"
		if attempt > 1 {
			suffix = fmt.Sprintf(" (nhập %d)", attempt)
		}
		candidate := truncateRunes(trimmed, 200-len([]rune(suffix))) + suffix
		if !taken[strings.ToLower(candidate)] {
			return candidate, nil
		}
	}
	return "", invalidInput("Đã có quá nhiều mẫu video cùng tên", "Đổi tên mẫu video cũ rồi nhập lại")
}

// importScenePrompt tạo (hoặc dùng lại) prompt ảnh cảnh cho imagePrompt của
// template cũ — luật migration của Gói V15: mỗi template cũ sinh một prompt
// "<tên template> · ảnh"; prompt trùng nội dung thì dùng lại.
func (s *Service) importScenePrompt(ctx context.Context, templateName string, project editor.Project) (string, string, error) {
	text := projectScenePrompt(project)
	if text == "" {
		return "", "", nil
	}
	existing, err := s.store.ListPrompts(ctx, domain.PromptScene, "")
	if err != nil {
		return "", "", err
	}
	for _, prompt := range existing {
		if strings.TrimSpace(prompt.Prompt) == text {
			return prompt.ID, "", nil
		}
	}
	name := truncateRunes(templateName+" · ảnh", domain.LimitPromptNameMaxChars)
	prompt, err := s.CreatePrompt(ctx, PromptInput{Name: name, Kind: domain.PromptScene, Prompt: &text})
	if err != nil {
		if CodeOf(err) != CodeConflict {
			return "", "", err
		}
		// Tên prompt đã có nhưng nội dung khác: dùng hậu tố của prompt import.
		prompt, err = s.CreatePrompt(ctx, PromptInput{
			Name: truncateRunes(templateName+" · ảnh (nhập)", domain.LimitPromptNameMaxChars),
			Kind: domain.PromptScene, Prompt: &text,
		})
		if err != nil {
			return "", "", err
		}
	}
	return prompt.ID, fmt.Sprintf("Chỉ dẫn ảnh của mẫu video cũ đã thành chỉ dẫn “%s” trong Thư viện › Chỉ dẫn ảnh", prompt.Name), nil
}

// projectScenePrompt đọc `templateScenes.imagePrompt` của track cảnh.
func projectScenePrompt(project editor.Project) string {
	tracks, ok := project.Timeline()["tracks"].([]any)
	if !ok {
		return ""
	}
	for _, value := range tracks {
		track, ok := value.(map[string]any)
		if !ok || track["templateSlot"] != "scenes" {
			continue
		}
		settings, ok := track["templateScenes"].(map[string]any)
		if !ok {
			return ""
		}
		text, _ := settings["imagePrompt"].(string)
		return strings.TrimSpace(text)
	}
	return ""
}

// overlayTrackCount đếm item toàn khung nằm trên track không có vai trò template
// — đúng phép đếm overlayCount của bản xuất cũ.
func overlayTrackCount(project editor.Project) int {
	tracks, ok := project.Timeline()["tracks"].([]any)
	if !ok {
		return 0
	}
	free := make(map[string]bool, len(tracks))
	for _, value := range tracks {
		track, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if slot, _ := track["templateSlot"].(string); slot == "" {
			if id, ok := track["id"].(string); ok {
				free[id] = true
			}
		}
	}
	items, ok := project.Timeline()["items"].([]any)
	if !ok {
		return 0
	}
	count := 0
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		trackID, _ := item["trackId"].(string)
		kind, _ := item["type"].(string)
		anchor, _ := item["templateAnchor"].(string)
		if free[trackID] && (kind == "video" || kind == "image") && anchor == "full" {
			count++
		}
	}
	return count
}

// pruneMissingBackdrop bỏ khỏi recipe (và khỏi project) những kênh nền, video
// ghim và video lẻ chưa có trong thư viện Viderse, kèm cảnh báo. Giữ lại trong
// recipe thì lần lưu recipe sau của người dùng sẽ bị 400 (CheckRefs), nên bỏ
// ngay từ lúc nhập — không bao giờ bỏ im lặng.
func (s *Service) pruneMissingBackdrop(ctx context.Context, project editor.Project, recipe domain.Recipe) (json.RawMessage, domain.Recipe, []string, error) {
	if recipe.Backdrop == nil {
		raw, err := editor.Marshal(project)
		return raw, recipe, nil, err
	}
	backdrop := *recipe.Backdrop
	var warnings []string

	channelIDs := make([]string, 0, len(backdrop.Channels))
	for _, link := range backdrop.Channels {
		channelIDs = append(channelIDs, link.ChannelID)
	}
	missingChannels, err := s.store.BackdropChannels(ctx, channelIDs)
	if err != nil {
		return nil, recipe, nil, err
	}
	missing := make(map[string]bool, len(missingChannels))
	for _, id := range missingChannels {
		missing[id] = true
		warnings = append(warnings, fmt.Sprintf("Kênh nền %s chưa có trong Thư viện › Video nền nên đã bỏ khỏi mẫu video — thêm kênh rồi chọn lại", id))
	}
	kept := make([]domain.BackdropChannelLink, 0, len(backdrop.Channels))
	for _, link := range backdrop.Channels {
		if missing[link.ChannelID] {
			continue
		}
		if len(link.Pinned) > 0 {
			gone, err := s.store.BackdropVideos(ctx, link.ChannelID, link.Pinned)
			if err != nil {
				return nil, recipe, nil, err
			}
			if len(gone) > 0 {
				warnings = append(warnings, fmt.Sprintf("Video ghim của kênh nền %s chưa có trong thư viện nên đã bỏ: %s", link.ChannelID, strings.Join(gone, ", ")))
				drop := make(map[string]bool, len(gone))
				for _, id := range gone {
					drop[id] = true
				}
				pinned := make([]string, 0, len(link.Pinned))
				for _, id := range link.Pinned {
					if !drop[id] {
						pinned = append(pinned, id)
					}
				}
				link.Pinned = pinned
			}
		}
		kept = append(kept, link)
	}
	backdrop.Channels = kept
	if len(backdrop.FixedVideoIDs) > 0 {
		fixed := make([]string, 0, len(backdrop.FixedVideoIDs))
		for _, id := range backdrop.FixedVideoIDs {
			ok, err := s.store.BackdropVideoExists(ctx, id)
			if err != nil {
				return nil, recipe, nil, err
			}
			if ok {
				fixed = append(fixed, id)
				continue
			}
			warnings = append(warnings, fmt.Sprintf("Video nền %s chưa có trong Thư viện › Video nền nên đã bỏ khỏi mẫu video", id))
		}
		backdrop.FixedVideoIDs = fixed
	}
	if len(backdrop.Channels) == 0 && len(backdrop.FixedVideoIDs) == 0 {
		recipe.Backdrop = nil
	} else {
		recipe.Backdrop = &backdrop
	}
	// Project giữ đúng phần còn dùng được để màn Cài đặt template không dựng
	// lại kênh thiếu từ track khi người dùng bấm Lưu.
	updated, err := syncProjectBackdrop(project, recipe.Backdrop)
	if err != nil {
		return nil, recipe, nil, err
	}
	return updated, recipe, warnings, nil
}

// syncProjectBackdrop ghi lại `templateBackdrop` trên track nền theo recipe đã
// lọc (nil = bỏ khoá để track không còn kênh thiếu).
func syncProjectBackdrop(project editor.Project, backdrop *domain.BackdropRecipe) (json.RawMessage, error) {
	tracks, ok := project.Timeline()["tracks"].([]any)
	if !ok {
		return editor.Marshal(project)
	}
	for _, value := range tracks {
		track, ok := value.(map[string]any)
		if !ok || track["templateSlot"] != "background-video" {
			continue
		}
		if backdrop == nil {
			delete(track, "templateBackdrop")
			continue
		}
		link := map[string]any{"channels": []any{}, "clipSec": backdrop.ClipSec}
		channels := make([]any, 0, len(backdrop.Channels))
		for _, channel := range backdrop.Channels {
			kinds := make([]any, 0, len(channel.Kinds))
			for _, kind := range channel.Kinds {
				kinds = append(kinds, kind)
			}
			pinned := make([]any, 0, len(channel.Pinned))
			for _, id := range channel.Pinned {
				pinned = append(pinned, id)
			}
			channels = append(channels, map[string]any{"channelId": channel.ChannelID, "kinds": kinds, "pinned": pinned})
		}
		link["channels"] = channels
		if backdrop.ClipCount != nil {
			link["clipCount"] = *backdrop.ClipCount
		}
		track["templateBackdrop"] = link
		break
	}
	return editor.Marshal(project)
}

// ── tiện ích ───────────────────────────────────────────────────────────────

// mustMarshalProject mã hoá project để Parse kiểm lại hình dạng (Project là map
// nên Parse chỉ cần JSON hợp lệ).
func mustMarshalProject(project editor.Project) []byte {
	data, err := json.Marshal(project)
	if err != nil {
		return []byte("{}")
	}
	return data
}

// baseMIME bỏ tham số codec khỏi loại tệp khai trong manifest
// (`video/mp4; codecs="…"` → `video/mp4`).
func baseMIME(mime string) string {
	if index := strings.IndexByte(mime, ';'); index >= 0 {
		return strings.TrimSpace(mime[:index])
	}
	return strings.TrimSpace(mime)
}

// mimePrefix là nhóm loại tệp ("video/") dùng để tìm lại media cùng nội dung.
func mimePrefix(mime string) string {
	base := baseMIME(mime)
	if index := strings.IndexByte(base, '/'); index > 0 {
		return base[:index+1]
	}
	return ""
}

// safeExtension giữ đuôi tệp của tên gốc cho tệp tạm (probe cần đuôi để chọn
// bộ đọc), không bao giờ dùng cả tên.
func safeExtension(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if len(ext) > 10 {
		return ""
	}
	for _, char := range ext {
		if char != '.' && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return ""
		}
	}
	return ext
}
