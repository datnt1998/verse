package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
)

// WorkerAPI is the private pull API the Mac TTS worker talks to (phase 07).
//
// It is served on the private listener only (default :8081, bound to the
// Tailscale host IP); it must never be registered on the public :8080 mux or
// routed by Traefik. Authentication is a least-privilege service token with
// the 'tts' capability — never a user session or an admin token — and every
// mutating call presents the job's fencing token, so a worker that lost its
// lease cannot publish.
//
// Protocol (claim → snapshot → upload → ACK):
//
//	POST /internal/worker/claim
//	     → 200 {"job":{...},"lease":{...}} | 204 (nothing claimable)
//	POST /internal/worker/jobs/{id}/heartbeat   {"fence":n,"leaseSec":n}
//	POST /internal/worker/jobs/{id}/blob        (raw body, X-Fence, X-Sha256,
//	                                             Content-Type, X-Filename)
//	POST /internal/worker/jobs/{id}/complete    {"fence":n,"progressDone":n,
//	                                             "result":{...},"attachments":[...]}
//	POST /internal/worker/jobs/{id}/fail        {"fence":n,"error":{...}}
//	GET  /internal/worker/jobs/{id}/input       (the frozen job snapshot)
//
// The upload endpoint derives the object key itself from (project, job, sha256)
// and verifies the streamed hash, so a worker can neither pick an arbitrary
// key nor overwrite another tenant's object. Completion re-checks the blob's
// recorded hash/size and attaches render metadata and the job result in one
// transaction.
type WorkerAPI struct {
	Queue WorkerQueue
	Blobs blobstore.BlobStore
	// Registry records uploaded objects in the blobs table so the ACK can
	// verify them. Uploads are refused when it is unset: an object no ACK can
	// reference is leaked storage, not a success.
	Registry BlobRegistry
	// BlobDriver names the active storage driver ('local'|'s3') for the
	// registry row. It has no safe default — the listener must be configured
	// with the same driver the process serves blobs from.
	BlobDriver string
	// Keys builds object keys for the process STORAGE_ENV. The zero value is
	// the dev environment; production wiring passes blobkey.FromEnv().
	Keys blobkey.Keys
	// Lease is the default lease when a claim does not ask for one.
	Lease time.Duration
	// MaxUpload bounds one uploaded object (a segment or export file).
	MaxUpload int64
}

// BlobRegistry records an immutable uploaded object in the database registry.
// Implementations must be idempotent for the same (key, sha256, size): a
// retried upload of identical content is not an error.
type BlobRegistry interface {
	RegisterBlob(ctx context.Context, projectID, jobID, key, driver, contentType, sha256Hex string, size int64) error
}

// BlobLookup is the optional registry capability that lets the upload path
// skip storing bytes it already has. A hit must mean "this exact content is
// already committed under this exact key": the implementation checks the ready
// row's digest against the digest embedded in the content-addressed key.
type BlobLookup interface {
	ReadyBlobSize(ctx context.Context, projectID, key string) (size int64, found bool, err error)
}

// WorkerIdentity is the authenticated service credential.
type WorkerIdentity struct {
	ID           string
	Name         string
	Capabilities []string
}

// Can reports whether the credential holds a capability.
func (w WorkerIdentity) Can(capability string) bool {
	for _, c := range w.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// WorkerJob is the job snapshot handed to a worker.
type WorkerJob struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"projectId"`
	StoryID       string          `json:"storyId"`
	Kind          string          `json:"kind"`
	State         string          `json:"state"`
	Input         json.RawMessage `json:"input"`
	ProgressDone  int32           `json:"progressDone"`
	ProgressTotal *int32          `json:"progressTotal,omitempty"`
	Fence         int64           `json:"fence"`
	LeaseUntil    time.Time       `json:"leaseUntil"`
}

// WorkerAttachment is one uploaded object claimed as a segment/export output.
// Key and SHA256 are exactly what the upload endpoint returned; the adapter
// verifies them against the blob registry before attaching.
type WorkerAttachment struct {
	SegmentID      string          `json:"segmentId"`
	SynthKey       string          `json:"synthKey"`
	Key            string          `json:"key"`
	SHA256         string          `json:"sha256"`
	DurationSec    float64         `json:"durationSec"`
	SampleRate     int32           `json:"sampleRate"`
	Parameters     json.RawMessage `json:"parameters,omitempty"`
	ChapterID      string          `json:"chapterId"`
	ScriptRevision int32           `json:"scriptRevision"`
	Select         bool            `json:"select"`
}

// WorkerQueue is the durable-queue surface the API needs. The Postgres adapter
// implements it; tests use a fake, so the HTTP contract and the DB fence
// semantics are verified independently.
type WorkerQueue interface {
	Authenticate(ctx context.Context, tokenHash []byte) (WorkerIdentity, error)
	Touch(ctx context.Context, workerID string) error
	Claim(ctx context.Context, workerID, projectID string, lease time.Duration) (WorkerJob, error)
	Heartbeat(ctx context.Context, workerID, jobID string, fence int64, lease time.Duration) (WorkerJob, error)
	JobInput(ctx context.Context, workerID, jobID string) (WorkerJob, error)
	Complete(ctx context.Context, workerID, jobID string, fence int64, progressDone int32, result any, attachments []WorkerAttachment) (WorkerJob, error)
	Fail(ctx context.Context, workerID, jobID string, fence int64, cause any) (WorkerJob, error)
	Release(ctx context.Context, workerID, jobID string, fence int64) (WorkerJob, error)
}

var workerTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{32,}$`)

// Handler returns the private mux. It is intentionally NOT mounted on the
// public gateway mux.
func (a *WorkerAPI) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/worker/claim", a.authenticated(a.handleClaim))
	mux.HandleFunc("POST /internal/worker/jobs/{id}/heartbeat", a.authenticated(a.handleHeartbeat))
	mux.HandleFunc("POST /internal/worker/jobs/{id}/blob", a.authenticated(a.handleUpload))
	mux.HandleFunc("POST /internal/worker/jobs/{id}/complete", a.authenticated(a.handleComplete))
	mux.HandleFunc("POST /internal/worker/jobs/{id}/fail", a.authenticated(a.handleFail))
	mux.HandleFunc("POST /internal/worker/jobs/{id}/release", a.authenticated(a.handleRelease))
	mux.HandleFunc("GET /internal/worker/jobs/{id}/input", a.authenticated(a.handleInput))
	return mux
}

type workerAuthFunc = func(http.ResponseWriter, *http.Request, WorkerIdentity)

// authenticated resolves the bearer token by hash, enforces the tts
// capability and records liveness. Anything else is a 401/403 before any job
// state is touched.
func (a *WorkerAPI) authenticated(next workerAuthFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			jsonError(w, http.StatusUnauthorized, "Thiếu phiên worker")
			return
		}
		sum := sha256.Sum256([]byte(token))
		identity, err := a.Queue.Authenticate(r.Context(), sum[:])
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "Phiên worker không hợp lệ")
			return
		}
		if !identity.Can("tts") {
			jsonError(w, http.StatusForbidden, "Worker không có quyền TTS")
			return
		}
		if err := a.Queue.Touch(r.Context(), identity.ID); err != nil {
			// Liveness is informational; a touch failure must not turn a
			// successful claim into an error, but it is logged by the caller.
			_ = err
		}
		next(w, r, identity)
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	if !workerTokenRe.MatchString(token) {
		return "", false
	}
	return token, true
}

// Lease bounds are server-authoritative. A worker may ask for a lease, but the
// server clamps it: a shorter value would let a live worker lose its job mid
// synthesis, and an arbitrarily long one would pin the queue to a dead Mac.
const (
	minWorkerLease = 30 * time.Second
	maxWorkerLease = 10 * time.Minute
)

func (a *WorkerAPI) leaseDuration(sec int) time.Duration {
	want := time.Duration(sec) * time.Second
	if sec <= 0 {
		want = a.Lease
		if want <= 0 {
			want = 120 * time.Second
		}
	}
	return min(max(want, minWorkerLease), maxWorkerLease)
}

type claimRequest struct {
	ProjectID string `json:"projectId"`
	LeaseSec  int    `json:"leaseSec"`
}

func (a *WorkerAPI) handleClaim(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	var req claimRequest
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	job, err := a.Queue.Claim(r.Context(), worker.ID, req.ProjectID, a.leaseDuration(req.LeaseSec))
	if errors.Is(err, errNoWorkerJob) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeWorkerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

type fenceRequest struct {
	Fence    int64 `json:"fence"`
	LeaseSec int   `json:"leaseSec"`
}

func (a *WorkerAPI) handleHeartbeat(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	var req fenceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	job, err := a.Queue.Heartbeat(r.Context(), worker.ID, r.PathValue("id"), req.Fence, a.leaseDuration(req.LeaseSec))
	if err != nil {
		writeWorkerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"leaseUntil": job.LeaseUntil, "fence": job.Fence})
}

type completeRequest struct {
	Fence        int64              `json:"fence"`
	ProgressDone int32              `json:"progressDone"`
	Result       json.RawMessage    `json:"result"`
	Attachments  []WorkerAttachment `json:"attachments"`
}

func (a *WorkerAPI) handleComplete(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	var req completeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	var result any
	if len(req.Result) > 0 {
		result = req.Result
	}
	job, err := a.Queue.Complete(r.Context(), worker.ID, r.PathValue("id"), req.Fence, req.ProgressDone, result, req.Attachments)
	if err != nil {
		writeWorkerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

type failRequest struct {
	Fence int64           `json:"fence"`
	Error json.RawMessage `json:"error"`
}

func (a *WorkerAPI) handleFail(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	var req failRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	var cause any
	if len(req.Error) > 0 {
		cause = req.Error
	}
	job, err := a.Queue.Fail(r.Context(), worker.ID, r.PathValue("id"), req.Fence, cause)
	if err != nil {
		writeWorkerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

// handleRelease is the graceful shutdown path: the worker hands the job back
// so it is claimed again immediately instead of waiting for the lease to lapse.
func (a *WorkerAPI) handleRelease(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	var req fenceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	job, err := a.Queue.Release(r.Context(), worker.ID, r.PathValue("id"), req.Fence)
	if err != nil {
		writeWorkerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (a *WorkerAPI) handleInput(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	job, err := a.Queue.JobInput(r.Context(), worker.ID, r.PathValue("id"))
	if err != nil {
		writeWorkerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

const (
	uploadFenceHeader = "X-Worker-Fence"
	uploadSHAHeader   = "X-Sha256"
	uploadNameHeader  = "X-Filename"
	maxUploadSlack    = 1 << 20 // header/metadata slack above MaxUpload
)

var uploadExtByType = map[string]string{
	"audio/mpeg":           "mp3",
	"audio/mp4":            "m4b",
	"audio/wav":            "wav",
	"audio/x-wav":          "wav",
	"audio/flac":           "flac",
	"application/zip":      "zip",
	"text/plain":           "txt",
	"application/x-subrip": "srt",
}

// handleUpload streams one object straight into BlobStore. The key is derived
// from the job's project and the declared sha256, so the worker cannot choose
// a key, target another job/project, or overwrite unrelated content.
func (a *WorkerAPI) handleUpload(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	jobID := r.PathValue("id")
	fence, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get(uploadFenceHeader)), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Thiếu fencing token")
		return
	}
	sha := strings.ToLower(strings.TrimSpace(r.Header.Get(uploadSHAHeader)))
	if !sha256HexRe.MatchString(sha) {
		jsonError(w, http.StatusBadRequest, "Thiếu sha256 của tệp")
		return
	}
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	ext, ok := uploadExtByType[contentType]
	if !ok {
		jsonError(w, http.StatusUnsupportedMediaType, "Định dạng tệp không hỗ trợ")
		return
	}

	// The claim/heartbeat path is the authority on (job, worker, fence): a
	// stale fence is rejected before a single byte is stored.
	job, err := a.Queue.Heartbeat(r.Context(), worker.ID, jobID, fence, a.Lease)
	if err != nil {
		writeWorkerError(w, err)
		return
	}

	max := a.MaxUpload
	if max <= 0 {
		max = 64 << 20
	}
	body := http.MaxBytesReader(w, r.Body, max)
	if a.Registry == nil || a.BlobDriver == "" {
		jsonError(w, http.StatusInternalServerError, "Worker chưa được cấu hình kho tệp")
		return
	}
	// The key is derived server-side from the job's tenant scope and the
	// declared digest: the worker can never choose a key, address another
	// project/story or overwrite unrelated content.
	key, err := a.Keys.AudioSegment(job.ProjectID, job.StoryID, sha, ext)
	if err != nil {
		slog.Error("không dựng được khoá blob cho việc tải lên",
			"module", "worker_api", "job_id", job.ID, "err", err)
		jsonError(w, http.StatusInternalServerError, "Việc này chưa gắn với truyện")
		return
	}
	// Deduplication across jobs: the key is content-addressed, so identical
	// bytes already stored and registered need no second upload. The registry
	// row is the authority (a ready row whose digest equals the key's digest).
	if lookup, ok := a.Registry.(BlobLookup); ok {
		size, found, err := lookup.ReadyBlobSize(r.Context(), job.ProjectID, key)
		if err != nil {
			writeWorkerError(w, err)
			return
		}
		if found {
			writeJSON(w, http.StatusCreated, map[string]any{
				"key": key, "sha256": sha, "size": size, "contentType": contentType, "deduplicated": true,
			})
			return
		}
	}
	var info blobstore.BlobInfo
	publisher, publishes := a.Registry.(interface {
		UploadBlob(context.Context, blobstore.BlobStore, string, string, string, string, string, string, io.Reader) (blobstore.BlobInfo, error)
	})
	if publishes {
		info, err = publisher.UploadBlob(r.Context(), a.Blobs, job.ProjectID, job.ID, key, a.BlobDriver, contentType, sha, body)
	} else {
		info, err = a.Blobs.Put(r.Context(), key, body, blobstore.PutOptions{ContentType: contentType, ExpectedSHA256: sha})
	}
	if err != nil {
		if errors.Is(err, blobstore.ErrDigestMismatch) {
			jsonError(w, http.StatusBadRequest, "sha256 không khớp nội dung")
			return
		}
		writeWorkerError(w, err)
		return
	}
	if !publishes {
		if err := a.Registry.RegisterBlob(r.Context(), job.ProjectID, job.ID, info.Key, a.BlobDriver, info.ContentType, info.SHA256, info.Size); err != nil {
			writeWorkerError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"key": info.Key, "sha256": info.SHA256, "size": info.Size, "contentType": info.ContentType,
	})
}

var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ── shared response helpers ──────────────────────────────────────────────────

var (
	errNoWorkerJob     = errors.New("không có việc nào đang chờ")
	errWorkerFencedOut = errors.New("quyền xử lý việc này đã mất")
)

func writeWorkerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNoWorkerJob):
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, errWorkerFencedOut):
		jsonError(w, http.StatusConflict, "Việc này đã bị thu hồi hoặc đã xong")
	case errors.Is(err, errWorkerNotFound):
		jsonError(w, http.StatusNotFound, "Không tìm thấy việc này")
	case errors.Is(err, errWorkerBadRequest):
		jsonError(w, http.StatusBadRequest, "Yêu cầu không hợp lệ")
	default:
		// A 500 is an operator problem (DB/storage). The shared policy maps
		// classified storage errors and, for anything unexpected, logs the
		// sanitized cause and returns the generic message to the worker.
		jsonMappedError(w, err)
	}
}

var (
	errWorkerNotFound   = errors.New("không tìm thấy việc này")
	errWorkerBadRequest = errors.New("yêu cầu không hợp lệ")
)
