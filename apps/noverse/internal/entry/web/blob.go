package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/blobstore"
)

// BlobAuthorizer decides whether the request's actor may read one blob. The
// domain-level implementation (project-scoped permission resolved through the
// blobs table) is wired in phase 04/05; this seam only trusts the injected
// function — the handler is never public-wired without an authorizer.
type BlobAuthorizer func(r *http.Request, key string) error

// ErrBlobDenied is the authorization denial the authorizer should return (or
// wrap); any non-nil error denies, this one maps to 403.
var ErrBlobDenied = errors.New("không có quyền đọc blob")

// BlobDelivery is the authenticated delivery seam for private blobs:
// streaming GET/HEAD with single-range support, and a presign-issuing route
// that degrades to the streaming URL when the driver cannot sign (local) or
// signing fails — the client always ends up with a working URL.
//
// Routes (registered by the owner phase, NOT wired to live paths at 03):
//
//	GET/HEAD /api/blobs/stream/{key...}   → bytes, 200/206/404/416/403
//	GET      /api/blobs/sign/{key...}     → {mode:"presigned",url,expiresAt}
//	                                        | {mode:"stream",url}
type BlobDelivery struct {
	Store      blobstore.BlobStore
	Authorize  BlobAuthorizer
	PresignTTL time.Duration // <=0 → driver default (60s), clamped by driver
}

func (d *BlobDelivery) blobKey(r *http.Request) (string, error) {
	key := r.PathValue("key")
	if key == "" {
		key = strings.TrimSpace(r.URL.Query().Get("key"))
	}
	if err := blobstore.ValidateKey(key); err != nil {
		return "", err
	}
	return key, nil
}

func (d *BlobDelivery) authorize(w http.ResponseWriter, r *http.Request, key string) bool {
	if d.Authorize == nil {
		// Misconfiguration is a denial, never an open door.
		jsonError(w, http.StatusInternalServerError, "Chưa cấu hình quyền truy cập blob")
		return false
	}
	if err := d.Authorize(r, key); err != nil {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return false
	}
	return true
}

// privateHeaders marks responses as private, non-sniffable content. The blob
// key is immutable so a short private cache window is safe; "private" keeps it
// out of shared caches entirely.
func privateHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Accept-Ranges", "bytes")
}

// ServeStream serves blob bytes with Range support: a single range (including
// suffix ranges) yields 206 with Content-Range; unsatisfiable ranges yield 416;
// multi-range requests are refused as unsatisfiable rather than guessing.
func (d *BlobDelivery) ServeStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		jsonError(w, http.StatusMethodNotAllowed, "Chỉ GET/HEAD")
		return
	}
	key, err := d.blobKey(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Khóa blob không hợp lệ")
		return
	}
	if !d.authorize(w, r, key) {
		return
	}
	privateHeaders(w)

	info, err := d.Store.Stat(r.Context(), key)
	if err != nil {
		writeBlobErr(w, err)
		return
	}

	start, end, status, rerr := resolveHTTPRange(r.Header.Get("Range"), info.Size)
	if rerr != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size))
		jsonError(w, http.StatusRequestedRangeNotSatisfiable, "Khoảng byte không hợp lệ")
		return
	}
	length := int64(0)
	if end >= start {
		length = end - start + 1
	}
	if status == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size))
	}
	ct := info.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	if info.SHA256 != "" {
		// Application-level digest (base64 would need conversion; hex is the
		// canonical form used by the blobs table).
		w.Header().Set("X-Content-SHA256", info.SHA256)
	}
	if info.ETag != "" {
		w.Header().Set("ETag", `"`+info.ETag+`"`)
	}
	if r.URL.Query().Get("download") != "" {
		w.Header().Set("Content-Disposition", blobDisposition("attachment", fileNameOf(key, r)))
	} else {
		w.Header().Set("Content-Disposition", blobDisposition("inline", fileNameOf(key, r)))
	}
	w.WriteHeader(status)
	if r.Method == http.MethodHead || length == 0 {
		return
	}
	rc, _, err := d.Store.Open(r.Context(), key, blobstore.ByteRange{Offset: start, Length: length})
	if err != nil {
		// Headers already sent: nothing left to report; the client sees a
		// truncated body which is honest.
		return
	}
	defer rc.Close()
	_, _ = io.Copy(w, rc)
}

// ServeSign answers how to fetch the blob: a short-lived presigned URL when
// the driver supports it, otherwise (or on any signing error) the
// authenticated streaming URL. Presign failure is never a dead end.
func (d *BlobDelivery) ServeSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "Chỉ GET")
		return
	}
	key, err := d.blobKey(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Khóa blob không hợp lệ")
		return
	}
	if !d.authorize(w, r, key) {
		return
	}
	if _, err := d.Store.Stat(r.Context(), key); err != nil {
		writeBlobErr(w, err)
		return
	}
	streamURL := "/api/blobs/stream/" + key
	url, err := d.Store.PresignGet(r.Context(), key, d.PresignTTL, blobstore.DownloadOptions{
		Filename: r.URL.Query().Get("filename"),
	})
	if err != nil {
		// ErrPresignUnsupported (local) or a signing outage both degrade to
		// the authenticated stream — one retry path, no infinite fallback.
		writeJSON(w, http.StatusOK, map[string]any{
			"mode": "stream",
			"url":  streamURL,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":      "presigned",
		"url":       url,
		"expiresAt": time.Now().Add(d.PresignTTL).UTC().Format(time.RFC3339),
	})
}

func writeBlobErr(w http.ResponseWriter, err error) {
	switch {
	case blobstore.IsNotFound(err):
		jsonError(w, http.StatusNotFound, "Blob không tồn tại")
	case errors.Is(err, blobstore.ErrInvalidKey), errors.Is(err, blobstore.ErrInvalidRange):
		jsonError(w, http.StatusBadRequest, "Yêu cầu blob không hợp lệ")
	default:
		jsonError(w, http.StatusInternalServerError, "Lỗi đọc blob")
	}
}

// resolveHTTPRange parses a single-range header against a known size.
// Returns start/end inclusive, the status (200 or 206), or an error meaning
// 416. "bytes=5-" on a 10-byte object → start 5 end 9; "bytes=-4" → last 4.
func resolveHTTPRange(h string, size int64) (start, end int64, status int, err error) {
	if h == "" {
		return 0, size - 1, http.StatusOK, nil
	}
	h = strings.TrimSpace(h)
	if !strings.HasPrefix(h, "bytes=") {
		return 0, 0, 0, errInvalidRangeHeader
	}
	specs := strings.Split(strings.TrimPrefix(h, "bytes="), ",")
	if len(specs) != 1 {
		return 0, 0, 0, errInvalidRangeHeader // single ranges only
	}
	spec := strings.TrimSpace(specs[0])
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, 0, errInvalidRangeHeader
	}
	first, last := spec[:dash], spec[dash+1:]
	if first == "" {
		// suffix range: last N bytes
		n, e := strconv.ParseInt(last, 10, 64)
		if e != nil || n <= 0 {
			return 0, 0, 0, errInvalidRangeHeader
		}
		if n > size {
			n = size
		}
		if size == 0 {
			return 0, 0, 0, errInvalidRangeHeader
		}
		return size - n, size - 1, http.StatusPartialContent, nil
	}
	s, e := strconv.ParseInt(first, 10, 64)
	if e != nil || s < 0 || s >= size {
		return 0, 0, 0, errInvalidRangeHeader
	}
	e2 := size - 1
	if last != "" {
		v, perr := strconv.ParseInt(last, 10, 64)
		if perr != nil || v < s {
			return 0, 0, 0, errInvalidRangeHeader
		}
		if v < e2 {
			e2 = v
		}
	}
	return s, e2, http.StatusPartialContent, nil
}

var errInvalidRangeHeader = errors.New("range không hợp lệ")

func fileNameOf(key string, r *http.Request) string {
	if fn := strings.TrimSpace(r.URL.Query().Get("filename")); fn != "" {
		return fn
	}
	if i := strings.LastIndexByte(key, '/'); i >= 0 {
		return key[i+1:]
	}
	return key
}

// blobDisposition is re-exported from blobstore's sanitizer so the header is
// safe for non-ASCII names.
func blobDisposition(kind, name string) string {
	return blobstore.ContentDisposition(kind, name)
}
