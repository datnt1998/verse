package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// ReadEditorProject exposes the same Project JSON shape as the old
// GET /api/editor/projects/:id (yt-factory projects.ts:338-389). Registry
// templates are never copied into editor workspace blobs or revision pointers.
func (s *Service) ReadEditorProject(ctx context.Context, id string) (editor.Project, string, error) {
	template, err := s.store.GetTemplate(ctx, id)
	if errors.Is(err, ErrNotFound) || err == nil && template == nil {
		return nil, "", blobstore.ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	template, err = s.ensureTemplateMedia(ctx, template)
	if err != nil {
		return nil, "", editorProjectError(err)
	}
	project, err := editor.Parse(template.Project)
	return project, strconv.FormatInt(template.Revision, 10), editorProjectError(err)
}

func (s *Service) ensureTemplateMedia(ctx context.Context, template *domain.Template) (*domain.Template, error) {
	for {
		var err error
		template, err = s.ensureTemplateProject(ctx, template)
		if err != nil || s.media == nil || s.importer == nil {
			return template, err
		}
		project, err := editor.Parse(template.Project)
		if err != nil {
			return nil, err
		}
		ready, _, err := s.prepareEditorTemplateMedia(ctx, template, project)
		var failure *Error
		if !errors.As(err, &failure) || failure.Code != CodeConflict {
			return ready, err
		}
		template, err = s.store.GetTemplate(ctx, template.ID)
		if err != nil {
			return nil, err
		}
		if template == nil {
			return nil, ErrNotFound
		}
	}
}

// Media đi kèm mẫu phải có metadata thật trong kho Biên tập, không chỉ có
// blob của thư viện. Nhập theo hash rồi CAS project; media đã nhập dùng lại
// được nếu một lần lưu đồng thời thắng, không ghi đè bản người dùng vừa sửa.
func (s *Service) prepareEditorTemplateMedia(ctx context.Context, template *domain.Template, project editor.Project) (*domain.Template, editor.Project, error) {
	if s.media == nil {
		return template, project, nil
	}
	assets, err := standardAssets()
	if err != nil {
		return nil, nil, err
	}
	known := make(map[string]standardAsset, len(assets))
	for _, asset := range assets {
		known[standardAssetID(asset.Name)] = asset
	}
	var mapping map[string]string
	for _, id := range projectMediaIDs(project) {
		asset, ok := known[id]
		if !ok {
			continue
		}
		if s.importer == nil {
			return nil, nil, invalidInput("Kho media Biên tập chưa sẵn sàng", "Khởi động lại Viderse rồi mở lại mẫu")
		}
		handle, err := s.media.Acquire(ctx, standardAssetPath(asset.Name))
		if errors.Is(err, blobstore.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			if err := s.publishStandardAssetFiles(ctx, []standardAsset{asset}, nil); err != nil {
				return nil, nil, err
			}
			handle, err = s.media.Acquire(ctx, standardAssetPath(asset.Name))
		}
		if err != nil {
			return nil, nil, err
		}
		mime := "image/webp"
		switch filepath.Ext(asset.Name) {
		case ".wav":
			mime = "audio/wav"
		case ".mp4":
			mime = "video/mp4"
		}
		// Copy the bytes while the read lease is held, then release the lease
		// before the editor imports them. PostgreSQL's lease takes a shared
		// advisory lock on the content key; media.Publish takes the exclusive
		// lock for that same key, so importing straight from the leased file
		// would make the publication wait on this request forever.
		staged, stageErr := stageTemplateAsset(ctx, handle, asset.Size)
		handle.Release()
		if stageErr != nil {
			return nil, nil, stageErr
		}
		record, _, importErr := s.importer.ImportFileOnce(ctx, editor.ImportFileOnceOptions{
			File: staged, FileName: asset.Name, DeclaredMIME: mime,
			ContentHash: "sha256:" + asset.SHA256, MIMEPrefix: mimePrefix(mime),
		})
		os.Remove(staged)
		if importErr != nil {
			return nil, nil, importErr
		}
		if record.ID == "" || record.ID == id {
			return nil, nil, fmt.Errorf("media đã nhập chưa có mã Biên tập riêng: %s", id)
		}
		if mapping == nil {
			mapping = make(map[string]string, len(known))
		}
		mapping[id] = record.ID
	}
	if len(mapping) == 0 {
		return template, project, nil
	}
	remapEditorTemplateMedia(project, mapping)
	raw, err := editor.Marshal(project)
	if err != nil {
		return nil, nil, err
	}
	saved, err := s.Patch(ctx, template.ID, Patch{Revision: template.Revision, Project: raw})
	return saved, project, err
}

// stageTemplateAsset copies a leased library asset into an unleased temporary
// file. The editor's media publication locks the content key exclusively, so it
// must not run while the library still holds that key's shared read lease.
func stageTemplateAsset(ctx context.Context, handle *media.Handle, size int64) (string, error) {
	source, err := os.Open(handle.Path)
	if err != nil {
		return "", err
	}
	target, err := os.CreateTemp("", "viderse-template-import-*"+filepath.Ext(handle.Path))
	if err != nil {
		return "", errors.Join(err, source.Close())
	}
	name := target.Name()
	written, copyErr := copyBounded(ctx, target, source, size+1)
	sourceErr := source.Close()
	closeErr := target.Close()
	if copyErr == nil && sourceErr == nil && closeErr == nil && written != size {
		copyErr = fmt.Errorf("tệp mẫu video sai kích thước: %s (%d/%d)", filepath.Base(handle.Path), written, size)
	}
	if err := errors.Join(copyErr, sourceErr, closeErr); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func remapEditorTemplateMedia(project editor.Project, mapping map[string]string) {
	refs, _ := project["mediaItems"].([]any)
	for _, value := range refs {
		if item, ok := value.(map[string]any); ok {
			id, _ := item["id"].(string)
			if replacement := mapping[id]; replacement != "" {
				item["id"] = replacement
			}
		}
	}
	var visit func(any)
	visit = func(value any) {
		switch value := value.(type) {
		case []any:
			for _, child := range value {
				visit(child)
			}
		case map[string]any:
			for key, child := range value {
				text, isText := child.(string)
				if isText && (key == "mediaId" || key == "coverMediaId") {
					if replacement := mapping[text]; replacement != "" {
						value[key] = replacement
					}
				} else if isText && key == "src" && strings.HasPrefix(text, "/api/editor/media/") {
					tail := strings.TrimPrefix(text, "/api/editor/media/")
					end := strings.IndexAny(tail, "/?")
					if end < 0 {
						end = len(tail)
					}
					if replacement := mapping[tail[:end]]; replacement != "" {
						value[key] = "/api/editor/media/" + replacement + tail[end:]
					}
				} else {
					visit(child)
				}
			}
		}
	}
	visit(map[string]any(project))
}

// WriteEditorProject uses the existing schema-15 validator and the same
// transaction/CAS as the library form. Every successful edit invalidates the
// template preview through UpdateTemplate and preserves authored timeline data.
func (s *Service) WriteEditorProject(ctx context.Context, id string, p editor.Project, expected string) (string, error) {
	template, err := s.store.GetTemplate(ctx, id)
	if errors.Is(err, ErrNotFound) || err == nil && template == nil {
		return "", blobstore.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	revision, err := strconv.ParseInt(expected, 10, 64)
	if err != nil || revision <= 0 {
		return "", &editor.Error{Code: "invalid_request", Message: "Thiếu revision template khi lưu"}
	}
	if revision != template.Revision {
		return "", editor.ErrConflict
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", &editor.Error{Code: "invalid_request", Message: "Project template không hợp lệ"}
	}
	name, _ := p["name"].(string)
	name = strings.TrimSpace(name)
	updated, err := s.Patch(ctx, id, Patch{Revision: revision, Name: &name, Project: raw})
	if err != nil {
		return "", editorProjectError(err)
	}
	return strconv.FormatInt(updated.Revision, 10), nil
}

func (s *Service) ListEditorProjects(ctx context.Context) ([]editor.ProjectSummary, error) {
	templates, err := s.store.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]editor.ProjectSummary, 0, len(templates))
	for _, template := range templates {
		result = append(result, editor.ProjectSummary{ID: template.ID, Name: template.Name, UpdatedAt: float64(template.UpdatedAt.UnixMilli()), CreatedAt: float64(template.CreatedAt.UnixMilli())})
	}
	return result, nil
}

func (s *Service) DeleteEditorProject(ctx context.Context, id string) error {
	return editorProjectError(s.DeleteTemplate(ctx, id))
}

func editorProjectError(err error) error {
	if err == nil {
		return nil
	}
	var failure *Error
	if errors.As(err, &failure) {
		code := "invalid_request"
		if failure.Code == CodeConflict {
			code = "revision_conflict"
		}
		if failure.Code == CodeInUse {
			code = "conflict"
		}
		if failure.Code == CodeNotFound {
			return blobstore.ErrNotFound
		}
		extra := editor.Object{}
		if len(failure.Channels) > 0 {
			extra["channels"] = failure.Channels
		}
		if len(failure.Fields) > 0 {
			extra["fieldErrors"] = failure.Fields
		}
		return &editor.Error{Code: code, Message: failure.Message, Extra: extra}
	}
	return err
}
