package web

import (
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/webapp"
	"net/http"
)

func planClone(w http.ResponseWriter, r *http.Request, svc QueueService, diag func(error)) {
	var body domain.CloneEnqueueRequest
	if !webapp.DecodeJSONCode(w, r, &body, domain.CloneErrInvalidRequest) {
		return
	}
	plan, err := svc.Plan(r.Context(), body)
	if err != nil {
		writeQueueError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, plan)
}
