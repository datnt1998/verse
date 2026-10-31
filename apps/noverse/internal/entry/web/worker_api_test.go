package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/platform/blobstore"
)

// stubQueue is a scriptable WorkerQueue: each call is a field so a test can
// assert the HTTP contract (status codes, fence handling, capability checks)
// without a database. The durable claim/fence semantics themselves are proven
// against real Postgres in internal/persistence/postgres/jobs_test.go.
type stubQueue struct {
	authenticate func(hash []byte) (WorkerIdentity, error)
	touch        func(workerID string) error
	claim        func(workerID, projectID string, lease time.Duration) (WorkerJob, error)
	heartbeat    func(workerID, jobID string, fence int64, lease time.Duration) (WorkerJob, error)
	input        func(workerID, jobID string) (WorkerJob, error)
	complete     func(workerID, jobID string, fence int64, progress int32, result any, att []WorkerAttachment) (WorkerJob, error)
	fail         func(workerID, jobID string, fence int64, cause any) (WorkerJob, error)
	release      func(workerID, jobID string, fence int64) (WorkerJob, error)

	lastTokenHash []byte
	lastTouch     string
	completed     []WorkerAttachment
}

func (s *stubQueue) Authenticate(_ context.Context, hash []byte) (WorkerIdentity, error) {
	s.lastTokenHash = hash
	if s.authenticate != nil {
		return s.authenticate(hash)
	}
	return WorkerIdentity{ID: "11111111-1111-4111-8111-111111111111", Name: "mac", Capabilities: []string{"tts"}}, nil
}

func (s *stubQueue) Touch(_ context.Context, workerID string) error {
	s.lastTouch = workerID
	if s.touch != nil {
		return s.touch(workerID)
	}
	return nil
}

func (s *stubQueue) Claim(ctx context.Context, workerID, projectID string, lease time.Duration) (WorkerJob, error) {
	if s.claim != nil {
		return s.claim(workerID, projectID, lease)
	}
	return WorkerJob{}, errNoWorkerJob
}

func (s *stubQueue) Heartbeat(ctx context.Context, workerID, jobID string, fence int64, lease time.Duration) (WorkerJob, error) {
	if s.heartbeat != nil {
		return s.heartbeat(workerID, jobID, fence, lease)
	}
	return WorkerJob{}, errWorkerFencedOut
}

func (s *stubQueue) JobInput(ctx context.Context, workerID, jobID string) (WorkerJob, error) {
	if s.input != nil {
		return s.input(workerID, jobID)
	}
	return WorkerJob{}, errWorkerNotFound
}

func (s *stubQueue) Complete(ctx context.Context, workerID, jobID string, fence int64, progress int32, result any, att []WorkerAttachment) (WorkerJob, error) {
	s.completed = att
	if s.complete != nil {
		return s.complete(workerID, jobID, fence, progress, result, att)
	}
	return WorkerJob{}, errWorkerFencedOut
}

func (s *stubQueue) Fail(ctx context.Context, workerID, jobID string, fence int64, cause any) (WorkerJob, error) {
	if s.fail != nil {
		return s.fail(workerID, jobID, fence, cause)
	}
	return WorkerJob{}, errWorkerFencedOut
}

func (s *stubQueue) Release(ctx context.Context, workerID, jobID string, fence int64) (WorkerJob, error) {
	if s.release != nil {
		return s.release(workerID, jobID, fence)
	}
	return WorkerJob{}, errWorkerFencedOut
}

// stubRegistry records upload registrations.
type stubRegistry struct {
	calls []string
	err   error
}

func (s *stubRegistry) RegisterBlob(_ context.Context, projectID, jobID, key, driver, contentType, sha string, size int64) error {
	s.calls = append(s.calls, projectID+"|"+key+"|"+driver+"|"+sha)
	return s.err
}

func newTestWorkerAPI(t *testing.T, queue WorkerQueue) (*WorkerAPI, *blobstore.Local) {
	t.Helper()
	blobs, err := blobstore.NewLocal(filepath.Join(t.TempDir(), "blobs"))
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}
	return &WorkerAPI{
		Queue:      queue,
		Blobs:      blobs,
		Registry:   &stubRegistry{},
		BlobDriver: "local",
		Lease:      120 * time.Second,
		MaxUpload:  1 << 20,
	}, blobs
}

func workerReq(method, path, token string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

const testWorkerToken = "abcdefghijklmnopqrstuvwxyz0123456789"

// TestWorkerAPIAuth: the private listener rejects missing/short/unknown tokens
// with 401 and a worker without the tts capability with 403 — before any job
// lookup happens.
func TestWorkerAPIAuth(t *testing.T) {
	q := &stubQueue{}
	api, _ := newTestWorkerAPI(t, q)
	h := api.Handler()

	cases := []struct {
		name  string
		token string
		auth  func([]byte) (WorkerIdentity, error)
		want  int
	}{
		{name: "no token", token: "", want: http.StatusUnauthorized},
		{name: "short token", token: "short", want: http.StatusUnauthorized},
		{name: "unknown token", token: testWorkerToken, auth: func([]byte) (WorkerIdentity, error) {
			return WorkerIdentity{}, errors.New("no such worker")
		}, want: http.StatusUnauthorized},
		{name: "wrong capability", token: testWorkerToken, auth: func([]byte) (WorkerIdentity, error) {
			return WorkerIdentity{ID: "w", Capabilities: []string{"export"}}, nil
		}, want: http.StatusForbidden},
		{name: "revoked", token: testWorkerToken, auth: func([]byte) (WorkerIdentity, error) {
			return WorkerIdentity{}, errWorkerFencedOut
		}, want: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q.authenticate = tc.auth
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/claim", tc.token, nil))
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", rr.Code, tc.want, rr.Body.String())
			}
		})
	}

	// The handler hashes the bearer token, never stores/compares plaintext.
	q.authenticate = nil
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/claim", testWorkerToken, nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("claim with no work = %d, want 204", rr.Code)
	}
	sum := sha256.Sum256([]byte(testWorkerToken))
	if !bytes.Equal(q.lastTokenHash, sum[:]) {
		t.Fatalf("token hash = %x, want %x", q.lastTokenHash, sum[:])
	}
	if q.lastTouch != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("touch not recorded: %q", q.lastTouch)
	}
}

// TestWorkerAPIClaimAndFence: claim returns the snapshot, a stale fence is a
// 409 on heartbeat/complete/fail, and the JSON shapes are stable.
func TestWorkerAPIClaimAndFence(t *testing.T) {
	job := WorkerJob{
		ID: "22222222-2222-4222-8222-222222222222", ProjectID: "33333333-3333-4333-8333-333333333333",
		StoryID: "44444444-4444-4444-8444-444444444444", Kind: "tts", State: "running",
		Input: json.RawMessage(`{"chapterId":"55555555-5555-4555-8555-555555555555","scriptRevision":1}`),
		Fence: 7, LeaseUntil: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
	}
	q := &stubQueue{
		claim: func(string, string, time.Duration) (WorkerJob, error) { return job, nil },
	}
	api, _ := newTestWorkerAPI(t, q)
	h := api.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/claim", testWorkerToken,
		strings.NewReader(`{"leaseSec":120}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("claim = %d (%s)", rr.Code, rr.Body.String())
	}
	var body struct {
		Job WorkerJob `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode claim: %v", err)
	}
	if body.Job.ID != job.ID || body.Job.Fence != 7 || string(body.Job.Input) != string(job.Input) {
		t.Fatalf("claim job = %+v", body.Job)
	}

	// Stale fence on every publish path.
	q.heartbeat = nil
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/jobs/"+job.ID+"/heartbeat", testWorkerToken,
		strings.NewReader(`{"fence":6}`)))
	if rr.Code != http.StatusConflict {
		t.Fatalf("stale heartbeat = %d, want 409", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/jobs/"+job.ID+"/complete", testWorkerToken,
		strings.NewReader(`{"fence":6,"progressDone":2}`)))
	if rr.Code != http.StatusConflict {
		t.Fatalf("stale complete = %d, want 409", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/jobs/"+job.ID+"/fail", testWorkerToken,
		strings.NewReader(`{"fence":6,"error":{"message":"boom"}}`)))
	if rr.Code != http.StatusConflict {
		t.Fatalf("stale fail = %d, want 409", rr.Code)
	}

	// Unknown job id is 404 (never reveals a foreign job).
	q.input = nil
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodGet, "/internal/worker/jobs/99999999-9999-4999-8999-999999999999/input", testWorkerToken, nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown job = %d, want 404", rr.Code)
	}

	// The private paths are not part of the public gateway routes: an unknown
	// worker route is a plain 404 from the private mux too.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodGet, "/internal/worker/secret", testWorkerToken, nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown route = %d, want 404", rr.Code)
	}
}

// TestWorkerAPIUpload: the upload endpoint derives the object key itself,
// verifies the streamed hash and rejects a mismatched digest without leaving
// an object behind.
func TestWorkerAPIUpload(t *testing.T) {
	job := WorkerJob{
		ID: "22222222-2222-4222-8222-222222222222", ProjectID: "33333333-3333-4333-8333-333333333333",
		StoryID: "44444444-4444-4444-8444-444444444444",
		Fence:   4, State: "running",
	}
	q := &stubQueue{
		heartbeat: func(_ string, jobID string, fence int64, _ time.Duration) (WorkerJob, error) {
			if jobID != job.ID || fence != 4 {
				return WorkerJob{}, errWorkerFencedOut
			}
			return job, nil
		},
	}
	api, blobs := newTestWorkerAPI(t, q)
	h := api.Handler()

	payload := []byte("ID3 fake mp3 bytes")
	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])
	path := "/internal/worker/jobs/" + job.ID + "/blob"

	upload := func(fence, digest, contentType string, body []byte) *httptest.ResponseRecorder {
		req := workerReq(http.MethodPost, path, testWorkerToken, bytes.NewReader(body))
		req.Header.Set(uploadFenceHeader, fence)
		req.Header.Set(uploadSHAHeader, digest)
		req.Header.Set("Content-Type", contentType)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	if rr := upload("", sha, "audio/mpeg", payload); rr.Code != http.StatusBadRequest {
		t.Fatalf("missing fence = %d, want 400", rr.Code)
	}
	if rr := upload("4", "nope", "audio/mpeg", payload); rr.Code != http.StatusBadRequest {
		t.Fatalf("bad sha header = %d, want 400", rr.Code)
	}
	if rr := upload("4", sha, "video/mp4", payload); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported type = %d, want 415", rr.Code)
	}
	if rr := upload("3", sha, "audio/mpeg", payload); rr.Code != http.StatusConflict {
		t.Fatalf("stale fence upload = %d, want 409", rr.Code)
	}
	// Declared hash does not match the bytes: rejected, no object committed.
	otherSha := sha256.Sum256([]byte("different"))
	otherKey, err := blobkey.Keys{}.AudioSegment(job.ProjectID, job.StoryID, hex.EncodeToString(otherSha[:]), "mp3")
	if err != nil {
		t.Fatalf("build expected key: %v", err)
	}
	if rr := upload("4", hex.EncodeToString(otherSha[:]), "audio/mpeg", payload); rr.Code != http.StatusBadRequest {
		t.Fatalf("digest mismatch = %d, want 400 (%s)", rr.Code, rr.Body.String())
	}
	if _, _, err := blobs.Open(context.Background(), otherKey, blobstore.FullRange); !blobstore.IsNotFound(err) {
		t.Fatalf("mismatched upload left an object: %v", err)
	}

	rr := upload("4", sha, "audio/mpeg", payload)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload = %d (%s)", rr.Code, rr.Body.String())
	}
	var stored struct {
		Key    string `json:"key"`
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode upload: %v", err)
	}
	// The key is content-addressed and story-scoped: the tenant prefix (dev
	// env) + the job's project/story, no job id, and the full digest.
	tenantPrefix := "dev/p/" + job.ProjectID + "/s/" + job.StoryID + "/"
	if !strings.HasPrefix(stored.Key, tenantPrefix) {
		t.Fatalf("key = %q, want prefix %q (worker must not choose the key)", stored.Key, tenantPrefix)
	}
	if strings.Contains(stored.Key, job.ID) {
		t.Fatalf("key = %q must not embed the job id (content-addressed for dedup)", stored.Key)
	}
	if !strings.HasSuffix(stored.Key, sha+".mp3") {
		t.Fatalf("key = %q, want the full sha256 + extension suffix", stored.Key)
	}
	if stored.SHA256 != sha || stored.Size != int64(len(payload)) {
		t.Fatalf("stored = %+v", stored)
	}
	rc, info, err := blobs.Open(context.Background(), stored.Key, blobstore.FullRange)
	if err != nil {
		t.Fatalf("open uploaded object: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, payload) || info.Size != int64(len(payload)) {
		t.Fatalf("object bytes = %q size=%d", got, info.Size)
	}
}

// TestWorkerAPIComplete: completion passes the fence and attachments through,
// and the ACK result is what the queue returned.
func TestWorkerAPIComplete(t *testing.T) {
	var gotFence int64
	var gotProgress int32
	q := &stubQueue{
		complete: func(_ string, jobID string, fence int64, progress int32, result any, att []WorkerAttachment) (WorkerJob, error) {
			gotFence, gotProgress = fence, progress
			if len(att) != 1 || att[0].SegmentID != "seg-a" {
				t.Errorf("attachments = %+v", att)
			}
			return WorkerJob{ID: jobID, State: "completed", Fence: fence}, nil
		},
	}
	api, _ := newTestWorkerAPI(t, q)
	h := api.Handler()

	body := `{"fence":11,"progressDone":3,"result":{"file":"chapter-1.mp3"},
		"attachments":[{"segmentId":"seg-a","synthKey":"` + strings.Repeat("a", 64) + `",
		"key":"opaque/key","sha256":"` + strings.Repeat("b", 64) + `",
		"durationSec":1.5,"sampleRate":24000,"chapterId":"55555555-5555-4555-8555-555555555555",
		"scriptRevision":1,"select":true}]}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/jobs/j1/complete", testWorkerToken, strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("complete = %d (%s)", rr.Code, rr.Body.String())
	}
	if gotFence != 11 || gotProgress != 3 {
		t.Fatalf("fence/progress = %d/%d", gotFence, gotProgress)
	}
	if len(q.completed) != 1 || !q.completed[0].Select {
		t.Fatalf("completed attachments = %+v", q.completed)
	}

	// Unknown JSON fields are rejected, so the wire format stays explicit.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/jobs/j1/complete", testWorkerToken,
		strings.NewReader(`{"fence":11,"bogus":1}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d, want 400", rr.Code)
	}
}

// countingStore counts Put calls so the dedup path can be observed from the
// outside: an identical upload must not stream the same bytes twice.
type countingStore struct {
	blobstore.BlobStore
	puts int
}

func (c *countingStore) Put(ctx context.Context, key string, r io.Reader, opts blobstore.PutOptions) (blobstore.BlobInfo, error) {
	c.puts++
	return c.BlobStore.Put(ctx, key, r, opts)
}

// lookupRegistry is a registry that also answers the dedup question, like the
// Postgres adapter does (`ReadyBlobSize` over the ready blobs row).
type lookupRegistry struct {
	stubRegistry
	keys map[string]int64
}

func (l *lookupRegistry) ReadyBlobSize(_ context.Context, projectID, key string) (int64, bool, error) {
	size, ok := l.keys[projectID+"|"+key]
	return size, ok, nil
}

// TestWorkerAPIUploadDeduplicates: a second job that renders identical bytes
// uploads nothing. The key is content-addressed, so the ready registry row is
// the authority and the worker gets 201 with the existing object.
func TestWorkerAPIUploadDeduplicates(t *testing.T) {
	job := WorkerJob{
		ID: "22222222-2222-4222-8222-222222222222", ProjectID: "33333333-3333-4333-8333-333333333333",
		StoryID: "44444444-4444-4444-8444-444444444444", Fence: 4, State: "running",
	}
	q := &stubQueue{
		heartbeat: func(_ string, jobID string, fence int64, _ time.Duration) (WorkerJob, error) {
			if jobID != job.ID || fence != 4 {
				return WorkerJob{}, errWorkerFencedOut
			}
			return job, nil
		},
	}
	api, local := newTestWorkerAPI(t, q)
	store := &countingStore{BlobStore: local}
	api.Blobs = store
	payload := []byte("ID3 identical segment bytes")
	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])
	key, err := blobkey.Keys{}.AudioSegment(job.ProjectID, job.StoryID, sha, "mp3")
	if err != nil {
		t.Fatalf("build key: %v", err)
	}
	reg := &lookupRegistry{keys: map[string]int64{}}
	api.Registry = reg
	h := api.Handler()

	upload := func() *httptest.ResponseRecorder {
		req := workerReq(http.MethodPost, "/internal/worker/jobs/"+job.ID+"/blob", testWorkerToken, bytes.NewReader(payload))
		req.Header.Set(uploadFenceHeader, "4")
		req.Header.Set(uploadSHAHeader, sha)
		req.Header.Set("Content-Type", "audio/mpeg")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	if rr := upload(); rr.Code != http.StatusCreated {
		t.Fatalf("first upload = %d (%s)", rr.Code, rr.Body.String())
	}
	if store.puts != 1 {
		t.Fatalf("puts after first upload = %d, want 1", store.puts)
	}
	// The registry now reports the object ready under its content key.
	reg.keys[job.ProjectID+"|"+key] = int64(len(payload))

	rr := upload()
	if rr.Code != http.StatusCreated {
		t.Fatalf("second upload = %d (%s)", rr.Code, rr.Body.String())
	}
	if store.puts != 1 {
		t.Fatalf("puts after duplicate upload = %d, want 1 (bytes must not be streamed twice)", store.puts)
	}
	var stored struct {
		Key          string `json:"key"`
		Size         int64  `json:"size"`
		Deduplicated bool   `json:"deduplicated"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode upload: %v", err)
	}
	if stored.Key != key || stored.Size != int64(len(payload)) || !stored.Deduplicated {
		t.Fatalf("deduplicated response = %+v", stored)
	}
}
