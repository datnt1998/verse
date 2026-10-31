package editor

import "context"

// TemplateProjects keeps registry-backed templates in their authoritative row.
// A decimal template revision is an opaque editor wire revision just like a
// sha256 workspace revision. ErrNotFound means the id belongs to the workspace.
type TemplateProjects interface {
	ReadEditorProject(context.Context, string) (Project, string, error)
	WriteEditorProject(context.Context, string, Project, string) (string, error)
}
