package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/platform/identity"
)

// Session authorization and same-origin enforcement run before this handler.
func (g *Gateway) serveStorageMetrics(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	const base = "/api/system/storage"
	if r.URL.Path != base && !strings.HasPrefix(r.URL.Path, base+"/") {
		return false
	}
	if !isAdmin(user) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return true
	}
	w.Header().Set("Cache-Control", "private, no-store")
	if r.URL.Path == base+"/test" {
		// The JSON response, like NDJSON, may contain a signed browser probe URL.
		w.Header().Set("Cache-Control", "no-store")
	}
	var settings *bootstrap.ManagedBlobs
	if g.pgAudio != nil {
		settings = g.pgAudio.Storage
	}
	if r.URL.Path == base && r.Method == http.MethodGet {
		metrics := []blobstore.OperationMetric{}
		if g.pgAudio != nil && g.pgAudio.Blobs != nil {
			if source, ok := g.pgAudio.Blobs.(interface {
				Snapshot() []blobstore.OperationMetric
			}); ok {
				metrics = source.Snapshot()
			}
		}
		if settings == nil {
			mode := "file"
			if g.pgAudio != nil {
				mode = "postgres"
			}
			writeJSON(w, 200, map[string]any{"mode": mode, "operations": metrics, "encryption": map[string]bool{"available": false}, "migration": nil})
			return true
		}
		body, err := settings.View(r.Context())
		if err != nil {
			storageSettingsError(w, err)
			return true
		}
		cache, err := audio.CacheUsage()
		if err != nil {
			jsonError(w, 500, "Không đọc được dung lượng bộ nhớ đệm")
			return true
		}
		body["cache"] = cache
		writeJSON(w, 200, body)
		return true
	}
	if settings == nil {
		jsonError(w, 409, "Chỉ thay đổi kho tệp khi máy chủ dùng cơ sở dữ liệu")
		return true
	}
	var candidate bootstrap.StorageCandidate
	decode := func() bool {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&candidate); err != nil {
			jsonError(w, 400, "Thông tin kho tệp không hợp lệ")
			return false
		}
		return true
	}
	allowProbe := func() bool {
		if g.allowStorageTest(user.Username) {
			return true
		}
		w.Header().Set("Retry-After", "60")
		jsonError(w, 429, "Bạn đã kiểm tra nhiều lần; hãy chờ một phút rồi thử lại")
		return false
	}
	switch {
	case r.URL.Path == base && r.Method == http.MethodPut:
		if !decode() {
			return true
		}
		if err := settings.CheckSaveVersion(r.Context(), candidate.Version); err != nil {
			storageSettingsError(w, err)
			return true
		}
		if !allowProbe() {
			return true
		}
		result, err := settings.Save(r.Context(), candidate)
		if err != nil {
			storageSettingsError(w, err)
			return true
		}
		writeJSON(w, 200, result)
	case r.URL.Path == base+"/test" && r.Method == http.MethodPost:
		if !allowProbe() {
			return true
		}
		if !decode() {
			return true
		}
		streaming := strings.Contains(r.Header.Get("Accept"), "application/x-ndjson")
		started, delivered := false, false
		var encoder *json.Encoder
		var progress func(bootstrap.StorageTestStep) error
		if streaming {
			encoder = json.NewEncoder(w)
			progress = func(step bootstrap.StorageTestStep) error {
				if !started {
					w.Header().Set("Content-Type", "application/x-ndjson")
					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("X-Accel-Buffering", "no")
					started = true
				}
				if err := encoder.Encode(step); err != nil {
					return err
				}
				return http.NewResponseController(w).Flush()
			}
		}
		result, err := settings.CheckCandidateProgress(r.Context(), candidate, progress)
		defer func() {
			if r.Context().Err() != nil || (streaming && !delivered) {
				cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = result.DiscardProbe(cleanup)
			}
		}()
		if err != nil {
			if !started {
				storageSettingsError(w, err)
				return true
			}
			result.OK = false
			if r.Context().Err() != nil {
				return true
			}
		}
		if result.ProbeURL != "" || candidate.Delivery == "redirect" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			origin := scheme + "://" + r.Host
			if g.proxy != nil && g.proxy.PublicURL != nil {
				origin = g.proxy.PublicURL.Scheme + "://" + g.proxy.PublicURL.Host
			}
			result.CORS = map[string]any{"CORSRules": []any{map[string]any{"AllowedOrigins": []string{origin}, "AllowedMethods": []string{"GET", "HEAD"}, "AllowedHeaders": []string{"range", "if-none-match", "if-range"}, "ExposeHeaders": []string{"accept-ranges", "content-range", "content-length", "etag"}, "MaxAgeSeconds": 3600}}}
		}
		if streaming {
			if !started {
				w.Header().Set("Content-Type", "application/x-ndjson")
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("X-Accel-Buffering", "no")
			}
			if err := encoder.Encode(struct {
				Done bool `json:"done"`
				bootstrap.StorageTestResult
			}{true, result}); err == nil {
				delivered = http.NewResponseController(w).Flush() == nil
			}
		} else {
			writeJSON(w, 200, result)
		}
	case r.Method == http.MethodPost && (r.URL.Path == base+"/migration" || r.URL.Path == base+"/migration/pause" || r.URL.Path == base+"/migration/resume" || r.URL.Path == base+"/migration/cancel"):
		if !decode() {
			return true
		}
		action := strings.TrimPrefix(r.URL.Path, base+"/migration/")
		if r.URL.Path == base+"/migration" {
			action = "start"
		}
		if err := settings.MigrationAction(r.Context(), action, candidate.Version); err != nil {
			storageSettingsError(w, err)
			return true
		}
		body, err := settings.View(r.Context())
		if err != nil {
			storageSettingsError(w, err)
			return true
		}
		writeJSON(w, 200, body)
	default:
		w.Header().Set("Allow", "GET, PUT, POST")
		jsonError(w, 405, "Phương thức không được hỗ trợ")
	}
	return true
}

func storageSettingsError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, bootstrap.ErrStorageLocked) || errors.Is(err, bootstrap.ErrStorageEncryption) || errors.Is(err, bootstrap.ErrStorageConflict) {
		status = http.StatusConflict
	}
	// Errors from providers and SQL may contain credentials or signed URLs.
	// Only explicitly safe application errors cross the HTTP boundary.
	reason := "Không thực hiện được thay đổi kho tệp; kiểm tra cấu hình và thử lại"
	if status == http.StatusConflict {
		reason = err.Error()
	}
	jsonError(w, status, reason)
}

func (g *Gateway) allowStorageTest(admin string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.storageTests == nil {
		g.storageTests = map[string][]time.Time{}
	}
	now := time.Now()
	recent := g.storageTests[admin][:0]
	for _, at := range g.storageTests[admin] {
		if now.Sub(at) < time.Minute {
			recent = append(recent, at)
		}
	}
	if len(recent) >= 6 {
		g.storageTests[admin] = recent
		return false
	}
	g.storageTests[admin] = append(recent, now)
	return true
}
