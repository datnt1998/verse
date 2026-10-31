package web

import (
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/webapp"
	"net/http"
)

func (ops CloneOperations) metadata(w http.ResponseWriter, r *http.Request, id string, parts []string) {
	if !cloneAvailable(w, ops.Metadata != nil) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var result any
	var err error
	switch {
	case len(parts) == 0 && r.Method == http.MethodGet:
		result, err = ops.Metadata.Read(r.Context(), id)
	case len(parts) == 0 && r.Method == http.MethodPatch:
		var p clonemetadata.Patch
		if !webapp.DecodeJSONCode(w, r, &p, domain.CloneErrInvalidRequest) {
			return
		}
		result, err = ops.Metadata.Patch(r.Context(), id, p)
	case len(parts) == 1 && parts[0] == "history" && r.Method == http.MethodGet:
		var history []clonemetadata.Version
		history, err = ops.Metadata.History(r.Context(), id)
		result = map[string]any{"history": history}
	case len(parts) == 1 && parts[0] == "rewrite" && r.Method == http.MethodPost:
		var p clonemetadata.Rewrite
		if !webapp.DecodeJSONCode(w, r, &p, domain.CloneErrInvalidRequest) {
			return
		}
		result, err = ops.Metadata.Rewrite(r.Context(), id, p)
	case len(parts) == 1 && parts[0] == "restore" && r.Method == http.MethodPost:
		var p clonemetadata.Restore
		if !webapp.DecodeJSONCode(w, r, &p, domain.CloneErrInvalidRequest) {
			return
		}
		result, err = ops.Metadata.Restore(r.Context(), id, p)
	default:
		cloneNotFound(w)
		return
	}
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, result)
}
