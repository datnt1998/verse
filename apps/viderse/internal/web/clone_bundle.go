package web

import (
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/bulk"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/bundle"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/webapp"
)

type CloneBundles = bundle.Service
type CloneBulk = bulk.Service

func (ops CloneOperations) download(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if len(query) != 1 || len(query["ids"]) != 1 {
		invalidClone(w, "Chọn từ 1 đến 50 video để tải.")
		return
	}
	ids := strings.Split(query.Get("ids"), ",")
	if len(ids) < 1 || len(ids) > bundle.MaxJobs {
		invalidClone(w, "Chọn từ 1 đến 50 video để tải.")
		return
	}
	for _, id := range ids {
		if !cloneSegment(id) || len(id) > 64 {
			invalidClone(w, "Mã video không hợp lệ.")
			return
		}
	}
	if !cloneAvailable(w, ops.Bundles != nil) {
		return
	}
	prepared, err := ops.Bundles.Prepare(r.Context(), ids)
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": prepared.FileName}))
	if err := prepared.Write(r.Context(), w); err != nil {
		slog.Error("clone bundle stream failed", "err", err)
	}
}
func (ops CloneOperations) thumbnail(w http.ResponseWriter, r *http.Request, id string) {
	if !cloneAvailable(w, ops.Bundles != nil) {
		return
	}
	name, data, err := ops.Bundles.Thumbnail(r.Context(), id)
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	_, _ = w.Write(data)
}
func (ops CloneOperations) bulk(w http.ResponseWriter, r *http.Request) {
	var input bulk.Input
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if !webapp.DecodeJSONCode(w, r, &input, domain.CloneErrInvalidRequest) {
		return
	}
	if !cloneAvailable(w, ops.Bulk != nil) {
		return
	}
	result, err := ops.Bulk.Run(r.Context(), input)
	cloneOperationJSON(w, result, err)
}
