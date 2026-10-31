package activity

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Audience is persisted with notifications and their durable error history.
type Audience string

const (
	AudienceAll   Audience = "all"
	AudienceAdmin Audience = "admin"
)

type viewerKey struct{}

// WithViewer scopes reads and acknowledgements to the authenticated viewer.
// An absent viewer is deliberately treated as a member, never as an admin.
func WithViewer(ctx context.Context, globalAdmin bool) context.Context {
	return context.WithValue(ctx, viewerKey{}, globalAdmin)
}

func IsAdminViewer(ctx context.Context) bool {
	admin, _ := ctx.Value(viewerKey{}).(bool)
	return admin
}

type NotificationSpec struct {
	Kind        string
	Text        string
	Href        string
	ActionLabel string
	Audience    Audience
	Object      *domain.ObjectRef
}

// EffectiveAudience fails closed for storage objects, even if a caller forgot
// to mark a storage failure as administrative.
func EffectiveAudience(a Audience, object *domain.ObjectRef) Audience {
	if object != nil && object.Type == "storage" {
		return AudienceAdmin
	}
	if a == "" {
		return AudienceAll
	}
	return a
}

func adminTask(t Task) bool {
	return t.Object.Type == "storage" || t.Kind == "storage_migrate" || t.Kind == "storage_move"
}
