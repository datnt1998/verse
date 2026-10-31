package web

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
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
	PresignTTL time.Duration // <=0 defaults to five minutes
	Metadata   func(*http.Request, string) (blobstore.BlobInfo, error)
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
	info, err := d.metadata(r, key)
	if err != nil {
		writeBlobErr(w, err)
		return
	}
	kind := "inline"
	if r.URL.Query().Get("download") != "" {
		kind = "attachment"
	}
	serveStoredBlob(w, r, d.Store, info, "private, max-age=60", kind, fileNameOf(key, r), d.PresignTTL)
}

// ServeSign answers how to fetch the blob: a short-lived presigned URL when
// the driver supports it, otherwise (or on any signing error) the
// authenticated streaming URL. Presign failure is never a dead end.
func (d *BlobDelivery) ServeSign(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
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
	if _, err := d.metadata(r, key); err != nil {
		writeBlobErr(w, err)
		return
	}
	streamURL := "/api/blobs/stream/" + key
	if filename := r.URL.Query().Get("filename"); filename != "" {
		streamURL += "?" + url.Values{"filename": {filename}, "download": {"1"}}.Encode()
	}
	ttl := effectiveDeliveryTTL(d.Store, d.PresignTTL)
	signedAt := time.Now().UTC().Truncate(time.Second)
	url, err := d.Store.PresignGet(r.Context(), key, ttl, blobstore.DownloadOptions{
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
		"expiresAt": signedExpiry(url, signedAt.Add(ttl)).Format(time.RFC3339),
	})
}

func (d *BlobDelivery) metadata(r *http.Request, key string) (blobstore.BlobInfo, error) {
	if d.Metadata != nil {
		return d.Metadata(r, key)
	}
	return d.Store.Stat(r.Context(), key)
}

func effectiveDeliveryTTL(store blobstore.BlobStore, ttl time.Duration) time.Duration {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if capper, ok := store.(interface {
		EffectivePresignTTL(time.Duration) time.Duration
	}); ok {
		return capper.EffectivePresignTTL(ttl)
	}
	if ttl > 15*time.Minute {
		ttl = 15 * time.Minute
	}
	return ttl
}

func signedExpiry(raw string, fallback time.Time) time.Time {
	u, err := url.Parse(raw)
	if err != nil {
		return fallback
	}
	at, err := time.Parse("20060102T150405Z", u.Query().Get("X-Amz-Date"))
	if err != nil {
		return fallback
	}
	seconds, err := strconv.ParseInt(u.Query().Get("X-Amz-Expires"), 10, 64)
	if err != nil {
		return fallback
	}
	return at.Add(time.Duration(seconds) * time.Second)
}

func shaETag(sha string) string {
	if sha == "" {
		return ""
	}
	return `"` + strings.ToLower(sha) + `"`
}

func matchesETag(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || (etag != "" && strings.TrimPrefix(candidate, "W/") == etag) {
			return true
		}
	}
	return false
}

// serveStoredBlob uses registry metadata, authorizes at its caller, and never
// commits a successful streaming response before storage has opened.
func serveStoredBlob(w http.ResponseWriter, r *http.Request, store blobstore.BlobStore, info blobstore.BlobInfo, cache, kind, name string, ttl time.Duration) {
	privateHeaders(w)
	w.Header().Set("Cache-Control", cache)
	etag := shaETag(info.SHA256)
	if etag != "" {
		w.Header().Set("ETag", etag)
		w.Header().Set("X-Content-SHA256", info.SHA256)
	}
	if matchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	rangeHeader := r.Header.Get("Range")
	if v := r.Header.Get("If-Range"); v != "" && v != etag {
		rangeHeader = ""
	}
	start, end, status, err := resolveHTTPRange(rangeHeader, info.Size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size))
		jsonError(w, http.StatusRequestedRangeNotSatisfiable, "Khoảng byte không hợp lệ")
		return
	}
	if r.Method == http.MethodGet && blobstore.DriverName(store) == "s3" {
		signed, err := store.PresignGet(r.Context(), info.Key, effectiveDeliveryTTL(store, ttl), blobstore.DownloadOptions{Filename: name, Disposition: kind})
		if err == nil {
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Location", signed)
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		if !errors.Is(err, blobstore.ErrPresignUnsupported) {
			writeBlobErr(w, err)
			return
		}
	}
	length := int64(0)
	if end >= start {
		length = end - start + 1
	}
	var rc io.ReadCloser
	if r.Method != http.MethodHead && length > 0 {
		rc, _, err = store.Open(r.Context(), info.Key, blobstore.ByteRange{Offset: start, Length: length})
		if err != nil {
			writeBlobErr(w, err)
			return
		}
		defer rc.Close()
	}
	if status == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size))
	}
	w.Header().Set("Content-Type", orDefault(info.ContentType, "application/octet-stream"))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Disposition", blobDisposition(kind, name))
	w.WriteHeader(status)
	if rc != nil {
		if _, err := io.Copy(w, rc); err != nil {
			slog.Error("blob response interrupted", "module", "blob_delivery", "err", err)
		}
	}
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
