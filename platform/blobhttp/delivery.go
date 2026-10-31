// Package blobhttp delivers already-authorized objects. Route resolution,
// registry lookup and application error responses remain with the caller.
package blobhttp

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

	"github.com/datnt1998/verse/platform/blobstore"
)

// Options controls transport only; no authorization or resource lookup occurs.
type Options struct {
	CacheControl string
	Disposition  string
	Filename     string
	PresignTTL   time.Duration
	// Delivery is proxy or redirect. Empty permits redirects, preserving the
	// existing S3 delivery policy. HEAD never uses a signed GET URL.
	Delivery string
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

// EffectiveDeliveryTTL applies the delivery default and store-specific signing cap.
func EffectiveDeliveryTTL(store blobstore.BlobStore, ttl time.Duration) time.Duration {
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

// SignedExpiry reads the expiry from the original signed URL without rewriting it.
func SignedExpiry(raw string, fallback time.Time) time.Time {
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

// Serve delivers metadata already resolved and authorized by its caller. Errors
// returned before the response starts are translated by the app. Interrupted
// streams are logged here because a second error response cannot be written.
func Serve(w http.ResponseWriter, r *http.Request, store blobstore.BlobStore, info blobstore.BlobInfo, opts Options) error {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return errors.New("blob delivery requires GET or HEAD")
	}
	if opts.Delivery != "" && opts.Delivery != "proxy" && opts.Delivery != "redirect" {
		return errors.New("blob delivery requires proxy or redirect")
	}
	privateHeaders(w)
	if opts.CacheControl != "" {
		w.Header().Set("Cache-Control", opts.CacheControl)
	}
	etag := shaETag(info.SHA256)
	if etag != "" {
		w.Header().Set("ETag", etag)
		w.Header().Set("X-Content-SHA256", info.SHA256)
	}
	if matchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	rangeHeader := r.Header.Get("Range")
	if v := r.Header.Get("If-Range"); v != "" && v != etag {
		rangeHeader = ""
	}
	start, end, status, err := resolveHTTPRange(rangeHeader, info.Size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size))
		return blobstore.ErrInvalidRange
	}
	if r.Method == http.MethodGet && opts.Delivery != "proxy" && blobstore.DriverName(store) == "s3" {
		signed, err := store.PresignGet(r.Context(), info.Key, EffectiveDeliveryTTL(store, opts.PresignTTL), blobstore.DownloadOptions{Filename: opts.Filename, Disposition: opts.Disposition})
		if err == nil {
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Location", signed)
			w.WriteHeader(http.StatusTemporaryRedirect)
			return nil
		}
		if !errors.Is(err, blobstore.ErrPresignUnsupported) {
			return err
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
			return err
		}
		defer rc.Close()
	}
	if status == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size))
	}
	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Disposition", blobstore.ContentDisposition(opts.Disposition, opts.Filename))
	w.WriteHeader(status)
	if rc != nil {
		if _, err := io.Copy(w, rc); err != nil {
			slog.Error("blob response interrupted", "module", "blob_delivery", "err", err)
		}
	}
	return nil
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
