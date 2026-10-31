package web

import (
	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
	"net/http"
	"time"
)

func MountRecap(g *Gateway, svc *insight.Service) {
	g.Handle(http.MethodGet, "/api/recap", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		var since *time.Time
		if raw := r.URL.Query().Get("since"); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				webapp.Error(w, http.StatusBadRequest, "mốc thời gian không hợp lệ")
				return
			}
			since = &t
		}
		data, err := svc.Recap(r.Context(), since)
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		webapp.WriteJSON(w, http.StatusOK, data)
	})
	g.Handle(http.MethodPost, "/api/recap/seen", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		if err := svc.Seen(r.Context()); err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
}
