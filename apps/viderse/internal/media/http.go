package media

import (
	"errors"
	"mime"
	"net/http"
	"time"

	"github.com/datnt1998/verse/platform/blobhttp"
	"github.com/datnt1998/verse/platform/blobstore"
)

// ServeHTTP serves an already-authorized logical resource. It never accepts a
// bucket or object key from the request. Callers resolve domain ownership first.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request, path string) error {
	store, d, ref, release, err := s.Resolve(r.Context(), path)
	if err != nil {
		return err
	}
	defer release()
	const ttl = 2 * time.Minute
	if d.Config.Driver == "s3" && d.Config.Delivery == "redirect" && r.Method == http.MethodGet {
		if err := s.Protect(r.Context(), ref, time.Now().Add(ttl+2*time.Minute)); err != nil {
			return err
		}
	}
	cache := w.Header().Get("Cache-Control")
	if cache == "" {
		cache = "private, max-age=60"
	}
	kind, name := "inline", ref.Filename
	if disposition := w.Header().Get("Content-Disposition"); disposition != "" {
		if parsed, params, err := mime.ParseMediaType(disposition); err == nil {
			kind = parsed
			if params["filename"] != "" {
				name = params["filename"]
			}
		}
	}
	if r.URL.Query().Get("download") != "" {
		kind = "attachment"
	}
	err = blobhttp.Serve(w, r, store, blobstore.BlobInfo{Key: ref.Key, Size: ref.Bytes, SHA256: ref.SHA256, ContentType: ref.ContentType, ModTime: ref.VerifiedAt}, blobhttp.Options{Delivery: d.Config.Delivery, PresignTTL: ttl, CacheControl: cache, Disposition: kind, Filename: name})
	if errors.Is(err, blobstore.ErrInvalidRange) {
		http.Error(w, "Khoảng byte không hợp lệ", http.StatusRequestedRangeNotSatisfiable)
		return nil
	}
	return err
}
