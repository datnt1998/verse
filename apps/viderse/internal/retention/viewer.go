package retention

import "context"

type cleanupViewerKey struct{}
type CleanupViewer struct {
	Actor string
	Admin bool
}

// WithCleanupViewer is set only by the authenticated HTTP boundary.
func WithCleanupViewer(ctx context.Context, actor string, admin bool) context.Context {
	return context.WithValue(ctx, cleanupViewerKey{}, CleanupViewer{actor, admin})
}
func Viewer(ctx context.Context) (CleanupViewer, bool) {
	viewer, ok := ctx.Value(cleanupViewerKey{}).(CleanupViewer)
	return viewer, ok
}
