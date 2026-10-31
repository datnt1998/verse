package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/datnt1998/verse/platform/blobhttp"
	"github.com/datnt1998/verse/platform/blobstore"
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
	ttl := blobhttp.EffectiveDeliveryTTL(d.Store, d.PresignTTL)
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
		"expiresAt": blobhttp.SignedExpiry(url, signedAt.Add(ttl)).Format(time.RFC3339),
	})
}

func (d *BlobDelivery) metadata(r *http.Request, key string) (blobstore.BlobInfo, error) {
	if d.Metadata != nil {
		return d.Metadata(r, key)
	}
	return d.Store.Stat(r.Context(), key)
}

// serveStoredBlob translates transport errors into the app's JSON contract.
// Its caller has already resolved and authorized the object.
func serveStoredBlob(w http.ResponseWriter, r *http.Request, store blobstore.BlobStore, info blobstore.BlobInfo, cache, kind, name string, ttl time.Duration) {
	if err := blobhttp.Serve(w, r, store, info, blobhttp.Options{CacheControl: cache, Disposition: kind, Filename: name, PresignTTL: ttl}); err != nil {
		if errors.Is(err, blobstore.ErrInvalidRange) {
			jsonError(w, http.StatusRequestedRangeNotSatisfiable, "Khoảng byte không hợp lệ")
			return
		}
		writeBlobErr(w, err)
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

func fileNameOf(key string, r *http.Request) string {
	if fn := strings.TrimSpace(r.URL.Query().Get("filename")); fn != "" {
		return fn
	}
	if i := strings.LastIndexByte(key, '/'); i >= 0 {
		return key[i+1:]
	}
	return key
}
