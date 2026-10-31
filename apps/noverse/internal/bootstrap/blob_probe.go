package bootstrap

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/aws/smithy-go"
	"github.com/datnt1998/verse/platform/blobstore"
)

type StorageTestStep struct {
	Step   string `json:"step"`
	OK     bool   `json:"ok"`
	MS     int64  `json:"ms"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}
type StorageTestResult struct {
	OK             bool              `json:"ok"`
	Steps          []StorageTestStep `json:"steps"`
	ConditionalPut string            `json:"conditionalPut"`
	CORS           any               `json:"cors,omitempty"`
	ProbeURL       string            `json:"corsProbeUrl,omitempty"`
	ProbeExpiresAt *time.Time        `json:"corsProbeExpiresAt,omitempty"`
	cleanupProbe   func(context.Context) error
}

// DiscardProbe removes an unpublished browser probe after request cancellation.
func (r StorageTestResult) DiscardProbe(ctx context.Context) error {
	if r.cleanupProbe == nil {
		return nil
	}
	return r.cleanupProbe(ctx)
}

var errStorageClockSkew = errors.New("storage clock skew")
var errStorageClockUnknown = errors.New("storage clock unavailable")
var errStorageBrowserEndpoint = errors.New("browser endpoint required")

func storageError(err error, step string) (string, string) {
	if err == nil {
		return "ok", "Đạt"
	}
	if errors.Is(err, blobstore.ErrUnsafeEndpoint) {
		return "unsafe_endpoint", "Không được dùng địa chỉ liên kết cục bộ hoặc địa chỉ siêu dữ liệu máy chủ làm kho tệp"
	}
	var cleanup *blobstore.ProbeCleanupError
	if errors.As(err, &cleanup) {
		// The outer operation may be PUT/read, but this failure came from DELETE.
		return storageError(cleanup.Err, "delete")
	}
	if errors.Is(err, errStorageBrowserEndpoint) {
		return "browser_endpoint_required", "Kho ở mạng nội bộ; cần địa chỉ trình duyệt truy cập được hoặc chọn truyền qua app"
	}
	if errors.Is(err, errStorageClockSkew) {
		return "clock_skew", "Đồng hồ máy chủ lệch quá 5 phút so với kho tệp; hãy đồng bộ giờ máy chủ"
	}
	if errors.Is(err, errStorageClockUnknown) {
		return "clock_unavailable", "Kho tệp không cung cấp giờ phản hồi; chưa kiểm tra được đồng hồ máy chủ"
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		message := strings.ToLower(api.ErrorMessage())
		if strings.Contains(message, "invalid signature") || strings.Contains(message, "no such key:") {
			return "wrong_key", "Mã khoá truy cập hoặc khoá bí mật không đúng"
		}
		if strings.Contains(message, "date is too old") || strings.Contains(message, "date is in the future") {
			return "clock_skew", "Đồng hồ máy chủ lệch; hãy đồng bộ giờ của máy chủ"
		}
		switch api.ErrorCode() {
		case "InvalidAccessKeyId", "InvalidAccessKeyID", "SignatureDoesNotMatch", "InvalidToken", "AuthFailure":
			return "wrong_key", "Mã khoá truy cập hoặc khoá bí mật không đúng"
		case "NoSuchBucket":
			return "no_bucket", "Không có tên kho này; hãy tạo kho hoặc kiểm tra lại tên"
		case "RequestTimeTooSkewed", "RequestExpired", "RequestInTheFuture":
			return "clock_skew", "Đồng hồ máy chủ lệch; hãy đồng bộ giờ của máy chủ"
		case "AccessDenied", "Forbidden":
			if step == "delete" {
				return "no_delete_permission", "Khoá chưa có quyền xoá tệp"
			}
			if step == "write" {
				return "no_write_permission", "Khoá chưa có quyền ghi tệp"
			}
			return "no_read_permission", "Khoá chưa có quyền đọc tệp"
		}
	}
	var network net.Error
	if errors.As(err, &network) || errors.Is(err, context.DeadlineExceeded) {
		return "unreachable_endpoint", "Không kết nối được địa chỉ máy chủ lưu trữ"
	}
	if errors.Is(err, blobstore.ErrNotFound) {
		return "no_bucket", "Không tìm thấy kho hoặc tệp kiểm tra"
	}
	if errors.Is(err, blobstore.ErrDigestMismatch) {
		return "content_mismatch", "Nội dung đọc lại không khớp với tệp đã ghi"
	}
	return "storage_error", "Kho tệp chưa hoàn thành được bước kiểm tra; kiểm tra quyền và kết nối"
}

func (m *ManagedBlobs) Candidate(ctx context.Context, c StorageCandidate) (BlobConfig, error) {
	id, err := m.active(ctx)
	if err != nil {
		return BlobConfig{}, err
	}
	_, old, err := m.store(ctx, id)
	if err != nil {
		return BlobConfig{}, err
	}
	if c.Current {
		return old, nil
	}
	if c.Driver == "" {
		if c.AccessKeyID != nil {
			old.S3.AccessKeyID = *c.AccessKeyID
		}
		if c.Secret != nil {
			old.S3.SecretAccessKey = *c.Secret
		}
		return old, nil
	}
	return candidateConfig(c, old)
}

func (m *ManagedBlobs) CheckCandidate(ctx context.Context, c StorageCandidate) (StorageTestResult, error) {
	return m.CheckCandidateProgress(ctx, c, nil)
}

func (m *ManagedBlobs) CheckCandidateProgress(ctx context.Context, c StorageCandidate, progress func(StorageTestStep) error) (StorageTestResult, error) {
	cfg, err := m.Candidate(ctx, c)
	if err != nil {
		return StorageTestResult{}, err
	}
	result, err := m.checkProgress(ctx, cfg, true, progress)
	if (c.Current || (c.Driver == "" && c.Secret == nil && c.AccessKeyID == nil)) && !errors.Is(err, context.Canceled) {
		m.recordStorageCheck(result)
	}
	return result, err
}

func (m *ManagedBlobs) check(ctx context.Context, cfg BlobConfig, probe bool) (StorageTestResult, error) {
	return m.checkProgress(ctx, cfg, probe, nil)
}

func (m *ManagedBlobs) checkProgress(ctx context.Context, cfg BlobConfig, probe bool, progress func(StorageTestStep) error) (StorageTestResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	st, err := OpenBlobStore(cfg)
	if err != nil {
		return StorageTestResult{}, errors.New("Thông tin kho tệp không hợp lệ")
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return StorageTestResult{}, err
	}
	// Probe keys are owned by blobkey and remain inside the environment's staging prefix.
	id := hex.EncodeToString(token[:])
	uuid := id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
	key, err := m.keys.Staging(uuid)
	if err != nil {
		return StorageTestResult{}, err
	}
	deleted := false
	defer func() {
		if deleted {
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = st.Delete(cleanup, key)
	}()
	payload := []byte("noverse-storage-range-probe")
	result := StorageTestResult{OK: true, Steps: []StorageTestStep{}}
	defer func() {
		if ctx.Err() != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = result.DiscardProbe(cleanup)
		}
	}()
	run := func(name string, fn func() error) {
		if ctx.Err() != nil {
			return
		}
		start := time.Now()
		err := fn()
		code, reason := storageError(err, name)
		result.Steps = append(result.Steps, StorageTestStep{Step: name, OK: err == nil, MS: time.Since(start).Milliseconds(), Code: code, Reason: reason})
		if err != nil {
			result.OK = false
		}
		if progress != nil {
			if err := progress(result.Steps[len(result.Steps)-1]); err != nil {
				cancel()
			}
		}
	}
	var writeErr error
	run("write", func() error {
		_, writeErr = st.Put(ctx, key, bytes.NewReader(payload), blobstore.PutOptions{ContentType: "application/octet-stream"})
		return writeErr
	})
	read := func(rng blobstore.ByteRange, want []byte) error {
		if writeErr != nil {
			return writeErr
		}
		rc, _, err := st.Open(ctx, key, rng)
		if err != nil {
			return err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, int64(len(payload)+1)))
		if err != nil {
			return err
		}
		if !bytes.Equal(data, want) {
			return blobstore.ErrDigestMismatch
		}
		return nil
	}
	run("read", func() error { return read(blobstore.FullRange, payload) })
	run("range", func() error { return read(blobstore.ByteRange{Offset: 2, Length: 7}, payload[2:9]) })
	if cfg.Driver == BlobDriverS3 {
		run("clock", func() error {
			if writeErr != nil {
				return writeErr
			}
			source, ok := st.(interface{ ServerClockOffset() (time.Duration, bool) })
			if !ok {
				return errStorageClockUnknown
			}
			offset, known := source.ServerClockOffset()
			if !known {
				return errStorageClockUnknown
			}
			if offset > 5*time.Minute || offset < -5*time.Minute {
				return errStorageClockSkew
			}
			return nil
		})
	}
	run("presign", func() error {
		if cfg.Driver == BlobDriverLocal || cfg.Delivery == "proxy" {
			return nil
		}
		if privateEndpoint(cfg.S3.Endpoint) && cfg.S3.PresignEndpoint == "" {
			return errStorageBrowserEndpoint
		}
		_, err := st.PresignGet(ctx, key, time.Minute, blobstore.DownloadOptions{})
		return err
	})
	if cfg.Delivery == "proxy" {
		run("app_range", func() error { return read(blobstore.ByteRange{Offset: 2, Length: 7}, payload[2:9]) })
	}
	run("delete", func() error { err := st.Delete(ctx, key); deleted = err == nil; return err })
	if cfg.Driver == BlobDriverS3 && cfg.Delivery == "redirect" && probe && result.OK {
		// A distinct key survives the delete-permission test for the browser.
		token[0] ^= 0x80
		probeID := hex.EncodeToString(token[:])
		probeUUID := probeID[:8] + "-" + probeID[8:12] + "-" + probeID[12:16] + "-" + probeID[16:20] + "-" + probeID[20:]
		probeKey, err := m.keys.Staging(probeUUID)
		if err != nil {
			return result, err
		}
		run("cors_probe", func() error {
			durable := false
			result.cleanupProbe = func(cleanup context.Context) error {
				if err := st.Delete(cleanup, probeKey); err != nil {
					return err
				}
				if durable {
					if _, err := m.pool.Pool().Exec(cleanup, `DELETE FROM storage_probes WHERE object_key=$1`, probeKey); err != nil {
						return err
					}
					return m.pruneUnusedStoreConfigs(cleanup)
				}
				return nil
			}
			expires := time.Now().UTC().Add(time.Minute)
			if m.cipher.aead != nil {
				tx, err := m.pool.Pool().Begin(ctx)
				if err != nil {
					return err
				}
				defer tx.Rollback(ctx)
				storeID, err := m.saveConfig(ctx, tx, cfg)
				if err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `INSERT INTO storage_probes(store_id,object_key,expires_at) VALUES($1,$2,$3)`, storeID, probeKey, expires); err != nil {
					return err
				}
				if err = tx.Commit(ctx); err != nil {
					return err
				}
				durable = true
			}
			// The timer covers env-only deployments without an encryption key; durable
			// cleanup above also covers a process restart when credentials can be saved.
			cleanupProbe := result.cleanupProbe
			time.AfterFunc(time.Until(expires), func() {
				cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				// Durable rows remain retryable by the reaper on cleanup failure.
				_ = cleanupProbe(cleanup)
			})
			if _, err = st.Put(ctx, probeKey, bytes.NewReader(payload), blobstore.PutOptions{}); err != nil {
				return err
			}
			result.ProbeURL, err = st.PresignGet(ctx, probeKey, time.Minute, blobstore.DownloadOptions{})
			if err == nil {
				result.ProbeExpiresAt = &expires
			}
			return err
		})
	}
	result.ConditionalPut = "not_applicable"
	if source, ok := st.(interface{ ConditionalPutSupport() string }); ok {
		result.ConditionalPut = source.ConditionalPutSupport()
	}
	if err := ctx.Err(); err != nil {
		result.OK = false
		return result, err
	}
	return result, nil
}

func (m *ManagedBlobs) recordStorageCheck(result StorageTestResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthAt = time.Now().UTC()
	m.healthError = ""
	if !result.OK {
		m.healthError = "Chưa hoàn tất kiểm tra kho tệp"
		for _, step := range result.Steps {
			if !step.OK {
				m.healthError = step.Reason
				break
			}
		}
	}
}
