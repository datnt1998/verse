package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

type QueueService interface {
	List(context.Context) (domain.CloneQueueView, error)
	Patch(context.Context, domain.QueuePatch) (domain.CloneQueueView, error)
	Plan(context.Context, domain.CloneEnqueueRequest) (domain.ClonePlan, error)
	ItemTarget(context.Context, string) (*domain.QueueItemTarget, error)
}

func MountQueue(g *Gateway, svc QueueService) { MountQueueDiag(g, svc, nil) }
func MountQueueDiag(g *Gateway, svc QueueService, diag func(error)) {
	g.Handle(http.MethodGet, "/api/queue", func(w http.ResponseWriter, r *http.Request, _ identity.User) { getQueue(w, r, svc, diag) })
	g.Handle(http.MethodPatch, "/api/queue", func(w http.ResponseWriter, r *http.Request, user identity.User) { patchQueue(w, r, user, svc, diag) })
	g.Handle(http.MethodPost, "/api/clone/plan", func(w http.ResponseWriter, r *http.Request, _ identity.User) { planClone(w, r, svc, diag) })
	g.Handle(http.MethodGet, "/api/queue/items/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		w.Header().Set("Cache-Control", "no-store")
		id := strings.TrimPrefix(r.URL.Path, "/api/queue/items/")
		item, err := svc.ItemTarget(r.Context(), id)
		if err != nil {
			writeQueueError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, item)
	})
}
func getQueue(w http.ResponseWriter, r *http.Request, svc QueueService, diag func(error)) {
	view, err := svc.List(r.Context())
	if err != nil {
		writeQueueError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, view)
}
func patchQueue(w http.ResponseWriter, r *http.Request, user identity.User, svc QueueService, diag func(error)) {
	var body domain.QueuePatch
	if !webapp.DecodeJSONCode(w, r, &body, domain.CloneErrInvalidRequest) {
		return
	}
	// Số luồng là cấu hình toàn máy; tạm dừng và thao tác mục là vận hành.
	if body.Workers != nil && !requireAdmin(w, user) {
		return
	}
	view, err := svc.Patch(r.Context(), body)
	if err != nil {
		writeQueueError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, view)
}
func writeQueueError(w http.ResponseWriter, err error, diag func(error)) {
	var ce *domain.CloneError
	if diag != nil && (!errors.As(err, &ce) || ce.Code == domain.CloneErrInternal) {
		diag(err)
	}
	WriteCloneError(w, err)
}
