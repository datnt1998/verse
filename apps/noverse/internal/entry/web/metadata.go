package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

// metadataRepository is the row-backed equivalent of the file metadata
// resource. CAS lives in the repository transaction, not a gateway-local lock.
// The base Repository writes remain available for authorized internal callers.
type metadataRepository interface {
	MetadataVersion(context.Context, *library.Actor, string, string) (string, error)
	UpdateProjectCAS(context.Context, *library.Actor, string, library.ProjectFile, string) error
	WriteStoryFileCAS(context.Context, *library.Actor, library.Story, library.StoryFile, string) error
}

func hostedMetadataResource(ctx context.Context, lib library.Repository, user identity.User, project, story string, writable bool) *versionResource {
	repo, ok := lib.(metadataRepository)
	if !ok {
		return nil
	}
	return &versionResource{
		lockKey: "metadata/" + project + "/" + story, writable: writable,
		versionFn: func() string {
			version, err := repo.MetadataVersion(ctx, actorOf(user), project, story)
			if err != nil {
				return ""
			}
			return version
		},
	}
}

// metadataWriteError handles a CAS race after the feedback middleware's read.
// Another gateway may have committed in between; return the same current /
// version / conflict envelope as the file backend's stale-save response.
func (g *Gateway) metadataWriteError(w http.ResponseWriter, r *http.Request, user identity.User, err error) {
	if !errors.Is(err, errs.ErrStorageConflict) {
		libError(w, err)
		return
	}
	res := g.resourceFor(r, user, nil)
	if res == nil {
		libError(w, err)
		return
	}
	current := g.currentResource(r, user, identity.Session{}, res)
	version := g.resourceVersion(res)
	if current == nil || version == "" {
		jsonError(w, http.StatusInternalServerError, "Không đọc được dữ liệu mới nhất")
		return
	}
	if object, ok := current.(map[string]any); ok {
		object["version"] = version
		if project, ok := object["project"].(map[string]any); ok {
			project["version"] = version
		}
	}
	writeJSON(w, http.StatusConflict, map[string]any{
		"error": "Đã có thay đổi từ nơi khác", "conflict": true, "version": version, "current": current,
	})
}
