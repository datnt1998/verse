// Package render owns durable, fenced headless export and measurement requests.
// State transitions are ported from yt-factory 2c201e6 clone/export-requests.ts.
package render

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

const (
	ExportRequestLease     = time.Minute
	ExportHeartbeatStale   = 45 * time.Second
	ExportRequestRetention = 3 * 24 * time.Hour
	RequestTokenTTL        = 4 * time.Hour
)

// Store instances sharing Root also share their in-process mutation lock.
// The per-request flock additionally fences writers in other server processes.
type Store struct {
	Root string
	Now  func() time.Time
}

type RequestError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e RequestError) Error() string { return e.Code + ": " + e.Message }

func requestError(code, message string) error { return RequestError{Code: code, Message: message} }

type ChromeProcess struct {
	PID        int    `json:"pid"`
	PGID       int    `json:"pgid"`
	ProfileDir string `json:"profileDir"`
	StartedAt  string `json:"startedAt"`
}

type ExportProgress struct {
	Phase       string `json:"phase"`
	FramesDone  *int64 `json:"framesDone"`
	FramesTotal *int64 `json:"framesTotal"`
	BytesSent   *int64 `json:"bytesSent"`
	BytesTotal  *int64 `json:"bytesTotal"`
}

func (p ExportProgress) validate() error {
	for _, n := range []*int64{p.FramesDone, p.FramesTotal, p.BytesSent, p.BytesTotal} {
		if n != nil && *n < 0 {
			return requestError("invalid_request", "Tiến độ phải là số nguyên không âm")
		}
	}
	return nil
}

type ExportRequest struct {
	Version        int             `json:"version"`
	ID             string          `json:"id"`
	Nonce          string          `json:"nonce"`
	Token          string          `json:"token"`
	TokenIssuedAt  string          `json:"tokenIssuedAt"`
	Status         string          `json:"status"`
	CreatedAt      string          `json:"createdAt"`
	ProjectID      string          `json:"projectId"`
	JobID          string          `json:"jobId"`
	ExportID       string          `json:"exportId"`
	Fence          string          `json:"fence"`
	ClaimedAt      string          `json:"claimedAt"`
	LeaseExpiresAt string          `json:"leaseExpiresAt"`
	HeartbeatAt    string          `json:"heartbeatAt"`
	Progress       *ExportProgress `json:"progress"`
	Chrome         *ChromeProcess  `json:"chrome"`
	Error          *RequestError   `json:"error"`
	FinishedAt     string          `json:"finishedAt"`
}

func (r ExportRequest) Terminal() bool {
	switch r.Status {
	case "done", "failed", "cancelled", "stale":
		return true
	}
	return false
}

func (r ExportRequest) LeaseExpired(now time.Time) bool {
	return expiredTimestamp(r.LeaseExpiresAt, now)
}

func (r ExportRequest) HeartbeatStale(now time.Time) bool {
	return (r.Status == "running" || r.Status == "uploading") && staleHeartbeat(r.HeartbeatAt, now)
}

func expiredTimestamp(value string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339Nano, value)
	return err != nil || !t.After(now)
}

func staleHeartbeat(value string, now time.Time) bool {
	if value == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	return err != nil || now.Sub(t) > ExportHeartbeatStale
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func timestamp(now time.Time) string { return now.UTC().Format(time.RFC3339Nano) }

// Identifiers are path components, never caller-supplied paths.
func validRequestComponent(id string) bool {
	if id == "" || id == "." || id == ".." || len(id) > 255 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func (s *Store) ProfileDir(id string) string {
	if !validRequestComponent(id) {
		return ""
	}
	return filepath.Join(s.Root, "chrome-profiles", id)
}

func (s *Store) exportPath(id string) string {
	return filepath.Join(s.Root, "export-requests", id+".json")
}

var requestRootLocks sync.Map

func (s *Store) withRequestLock(key string, fn func() error) (err error) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	value, loaded := requestRootLocks.Load(root)
	if !loaded {
		value, _ = requestRootLocks.LoadOrStore(root, &sync.Mutex{})
	}
	mu := value.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	lockDir := filepath.Join(root, ".locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(lockDir, key+".lock"))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lock.Unlock()) }()
	return fn()
}

// Atomic JSON replacement keeps failed writes from exposing partial records.
// any is limited to the JSON encoder boundary; request APIs remain typed.
func writeRequestJSON(file string, value any) (err error) {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".request-*")
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.Remove(tmp.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	_, writeErr := tmp.Write(data)
	if writeErr == nil {
		writeErr = tmp.Sync()
	}
	if err := errors.Join(writeErr, tmp.Close()); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), file); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(file))
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

func randomRequestID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func requestCredentials() (nonce, token string, err error) {
	nonce, err = randomRequestID()
	if err != nil {
		return "", "", err
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	return nonce, hex.EncodeToString(b[:]), nil
}

func (s *Store) CreateExport(projectID, jobID string) (ExportRequest, error) {
	if !validRequestComponent(projectID) || jobID != "" && !validRequestComponent(jobID) {
		return ExportRequest{}, requestError("invalid_request", "Mã dự án hoặc công việc không hợp lệ")
	}
	id, err := randomRequestID()
	if err != nil {
		return ExportRequest{}, err
	}
	nonce, token, err := requestCredentials()
	if err != nil {
		return ExportRequest{}, err
	}
	now := timestamp(s.now())
	r := ExportRequest{Version: 1, ID: id, Nonce: nonce, Token: token, TokenIssuedAt: now, Status: "queued", CreatedAt: now, ProjectID: projectID, JobID: jobID}
	err = s.withRequestLock("export-"+id, func() error { return writeRequestJSON(s.exportPath(id), r) })
	return r, err
}

func (s *Store) GetExport(id string) (ExportRequest, error) {
	var r ExportRequest
	if !validRequestComponent(id) {
		return r, requestError("invalid_request", "Mã yêu cầu xuất không hợp lệ")
	}
	data, err := os.ReadFile(s.exportPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return r, requestError("not_found", "Yêu cầu xuất không tồn tại")
	}
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return ExportRequest{}, requestError("not_found", "Yêu cầu xuất không hợp lệ")
	}
	if r.ID != id || r.Version != 1 || !validRequestComponent(r.ProjectID) || r.Nonce == "" || r.CreatedAt == "" ||
		(r.Status != "queued" && r.Status != "running" && r.Status != "uploading" && !r.Terminal()) {
		return ExportRequest{}, requestError("not_found", "Yêu cầu xuất không hợp lệ")
	}
	if r.Progress != nil {
		if err := r.Progress.validate(); err != nil {
			return ExportRequest{}, err
		}
	}
	return r, nil
}

func (s *Store) ListExports() ([]ExportRequest, error) {
	out := []ExportRequest{}
	entries, err := os.ReadDir(filepath.Join(s.Root, "export-requests"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		r, err := s.GetExport(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			if requestErrorCode(err) == "not_found" || requestErrorCode(err) == "invalid_request" {
				continue
			}
			return nil, err
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return newerRequest(out[i].CreatedAt, out[j].CreatedAt) })
	return out, nil
}

func requestErrorCode(err error) string {
	var e RequestError
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func newerRequest(a, b string) bool {
	ta, ea := time.Parse(time.RFC3339Nano, a)
	tb, eb := time.Parse(time.RFC3339Nano, b)
	if ea == nil && eb == nil {
		return ta.After(tb)
	}
	return a > b
}

func (s *Store) mutateExport(id string, change func(*ExportRequest) error) (ExportRequest, error) {
	var r ExportRequest
	if !validRequestComponent(id) {
		return r, requestError("invalid_request", "Mã yêu cầu xuất không hợp lệ")
	}
	err := s.withRequestLock("export-"+id, func() error {
		var err error
		r, err = s.GetExport(id)
		if err != nil {
			return err
		}
		if err := change(&r); err != nil {
			return err
		}
		return writeRequestJSON(s.exportPath(id), r)
	})
	return r, err
}

func exportFence(r *ExportRequest, fence string) error {
	if r.Fence == "" || r.Fence != fence {
		return requestError("conflict", "Fence xuất không khớp — claim đã mất")
	}
	return nil
}

func activeExport(r *ExportRequest, fence string) error {
	if err := exportFence(r, fence); err != nil {
		return err
	}
	if r.Status != "running" && r.Status != "uploading" {
		return requestError("conflict", "Yêu cầu xuất không còn chạy")
	}
	return nil
}

func renewExport(r *ExportRequest, now time.Time) {
	r.LeaseExpiresAt = timestamp(now.Add(ExportRequestLease))
	r.HeartbeatAt = timestamp(now)
}

func (s *Store) ClaimExport(id, nonce string) (ExportRequest, error) {
	return s.mutateExport(id, func(r *ExportRequest) error {
		if r.Nonce != nonce {
			return requestError("conflict", "Nonce xuất không khớp — tạo yêu cầu xuất mới")
		}
		now := s.now()
		if r.Status != "queued" && !((r.Status == "running" || r.Status == "uploading") && r.LeaseExpired(now)) {
			return requestError("conflict", "Yêu cầu xuất đang được runtime khác giữ")
		}
		fence, err := randomRequestID()
		if err != nil {
			return err
		}
		r.Status = "running"
		r.Fence = fence
		if r.ClaimedAt == "" {
			r.ClaimedAt = timestamp(now)
		}
		renewExport(r, now)
		return nil
	})
}

func (s *Store) HeartbeatExport(id, fence string) (ExportRequest, error) {
	return s.mutateExport(id, func(r *ExportRequest) error {
		if err := activeExport(r, fence); err != nil {
			return err
		}
		renewExport(r, s.now())
		return nil
	})
}

func (s *Store) ProgressExport(id, fence string, progress ExportProgress) (ExportRequest, error) {
	if err := progress.validate(); err != nil {
		return ExportRequest{}, err
	}
	return s.mutateExport(id, func(r *ExportRequest) error {
		if err := activeExport(r, fence); err != nil {
			return err
		}
		if progress.Phase == "uploading" {
			r.Status = "uploading"
		}
		r.Progress = &progress
		renewExport(r, s.now())
		return nil
	})
}

func (s *Store) BindExport(id, fence, exportID string) (ExportRequest, error) {
	if !validRequestComponent(exportID) {
		return ExportRequest{}, requestError("invalid_request", "Mã bản xuất không hợp lệ")
	}
	return s.mutateExport(id, func(r *ExportRequest) error {
		if err := activeExport(r, fence); err != nil {
			return err
		}
		if r.ExportID != "" && r.ExportID != exportID {
			return requestError("conflict", "Yêu cầu xuất đã gắn export khác — tạo yêu cầu mới")
		}
		r.ExportID = exportID
		renewExport(r, s.now())
		return nil
	})
}

func (s *Store) AttachChrome(id, nonce string, chrome ChromeProcess) (ExportRequest, error) {
	if chrome.PID <= 0 || chrome.PGID <= 0 || chrome.ProfileDir == "" {
		return ExportRequest{}, requestError("invalid_request", "Tiến trình Chrome không hợp lệ")
	}
	return s.mutateExport(id, func(r *ExportRequest) error {
		if r.Nonce != nonce {
			return requestError("conflict", "Nonce xuất không khớp")
		}
		chrome.StartedAt = timestamp(s.now())
		r.Chrome = &chrome
		return nil
	})
}

func finishExport(r *ExportRequest, status string, now time.Time) {
	r.Status = status
	r.LeaseExpiresAt = ""
	r.HeartbeatAt = ""
	r.Fence = ""
	r.FinishedAt = timestamp(now)
}

func (s *Store) CompleteExport(id, fence, exportID string) (ExportRequest, error) {
	if !validRequestComponent(exportID) {
		return ExportRequest{}, requestError("invalid_request", "Mã bản xuất không hợp lệ")
	}
	return s.mutateExport(id, func(r *ExportRequest) error {
		if err := activeExport(r, fence); err != nil {
			return err
		}
		if r.ExportID == "" {
			r.ExportID = exportID
		}
		finishExport(r, "done", s.now())
		r.Error = nil
		return nil
	})
}

func (s *Store) FailExport(id, fence string, failure RequestError) (ExportRequest, error) {
	return s.mutateExport(id, func(r *ExportRequest) error {
		if err := exportFence(r, fence); err != nil {
			return err
		}
		if r.Terminal() {
			return nil
		}
		finishExport(r, "failed", s.now())
		r.Error = &failure
		return nil
	})
}

func (s *Store) CancelExport(id, nonce string) (ExportRequest, error) {
	return s.mutateExport(id, func(r *ExportRequest) error {
		if nonce != "" && r.Nonce != nonce {
			return requestError("conflict", "Nonce xuất không khớp")
		}
		if !r.Terminal() {
			finishExport(r, "cancelled", s.now())
		}
		return nil
	})
}

func (s *Store) StaleExport(id, reason string) (ExportRequest, error) {
	return s.mutateExport(id, func(r *ExportRequest) error {
		if !r.Terminal() {
			finishExport(r, "stale", s.now())
			r.Error = &RequestError{Code: "stale", Message: reason}
		}
		return nil
	})
}

// RecoverExport is creator-only: call after stopping the recorded Chrome group
// and removing any staged export. A stale page loses nonce, token and fence.
func (s *Store) RecoverExport(id string) (ExportRequest, error) {
	return s.mutateExport(id, func(r *ExportRequest) error {
		if r.Terminal() {
			return requestError("conflict", "Yêu cầu xuất đã kết thúc")
		}
		nonce, token, err := requestCredentials()
		if err != nil {
			return err
		}
		r.Nonce = nonce
		r.Token = token
		r.TokenIssuedAt = timestamp(s.now())
		r.Status = "queued"
		r.Fence = ""
		r.ClaimedAt = ""
		r.LeaseExpiresAt = ""
		r.HeartbeatAt = ""
		r.ExportID = ""
		r.Progress = nil
		r.Chrome = nil
		r.Error = nil
		r.FinishedAt = ""
		return nil
	})
}

// CheckToken authenticates only the named request, never a user session. Queued
// requests are active so the page may authenticate its initial nonce claim.
// Token lifetime follows issuance, including a fresh token after crash recovery;
// records predating tokenIssuedAt retain their original creation-based expiry.
func (s *Store) CheckToken(id, token, kind string) (string, error) {
	var projectID, stored, created, issued string
	var terminal bool
	switch kind {
	case "export", "render":
		r, err := s.GetExport(id)
		if err != nil {
			return "", requestError("unauthorized", "Token yêu cầu không hợp lệ")
		}
		projectID, stored, created, terminal = r.ProjectID, r.Token, r.CreatedAt, r.Terminal()
		issued = r.TokenIssuedAt
	case "measurement", "measure":
		r, err := s.GetMeasurement(id)
		if err != nil {
			return "", requestError("unauthorized", "Token yêu cầu không hợp lệ")
		}
		projectID, stored, created, terminal = r.ProjectID, r.Token, r.CreatedAt, r.Terminal()
		issued = r.TokenIssuedAt
	default:
		return "", requestError("unauthorized", "Loại yêu cầu không hợp lệ")
	}
	if issued == "" {
		issued = created
	}
	// Decode into fixed-size buffers before comparison so malformed token lengths
	// cannot bypass the constant-time comparison of the actual 32-byte secrets.
	var expected, supplied [32]byte
	validLengths := len(stored) == hex.EncodedLen(len(expected)) && len(token) == hex.EncodedLen(len(supplied))
	var storedErr, suppliedErr error
	if validLengths {
		_, storedErr = hex.Decode(expected[:], []byte(stored))
		_, suppliedErr = hex.Decode(supplied[:], []byte(token))
	}
	matched := subtle.ConstantTimeCompare(expected[:], supplied[:])
	born, err := time.Parse(time.RFC3339Nano, issued)
	now := s.now()
	if !validLengths || storedErr != nil || suppliedErr != nil || matched != 1 || terminal || err != nil || now.Before(born) || !now.Before(born.Add(RequestTokenTTL)) {
		return "", requestError("unauthorized", "Token yêu cầu đã hết hạn hoặc không hợp lệ")
	}
	return projectID, nil
}

func (s *Store) ListPrunableExports(now time.Time, maxAge time.Duration) ([]ExportRequest, error) {
	if maxAge <= 0 {
		maxAge = ExportRequestRetention
	}
	requests, err := s.ListExports()
	if err != nil {
		return nil, err
	}
	out := []ExportRequest{}
	for _, r := range requests {
		at := r.FinishedAt
		if at == "" {
			at = r.CreatedAt
		}
		finished, err := time.Parse(time.RFC3339Nano, at)
		if r.Terminal() && err == nil && now.Sub(finished) > maxAge {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) ExportRequestFiles(id string) []string {
	if !validRequestComponent(id) {
		return nil
	}
	return []string{s.exportPath(id), s.ProfileDir(id)}
}
