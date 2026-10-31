package library

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
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
	template, err = s.ensureTemplateProject(ctx, template)
	if err != nil {
		return nil, "", editorProjectError(err)
	}
	project, err := editor.Parse(template.Project)
	if err != nil {
		return nil, "", editorProjectError(err)
	}
	return project, strconv.FormatInt(template.Revision, 10), nil
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
