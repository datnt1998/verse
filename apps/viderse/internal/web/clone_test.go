package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
)

type fakeCloneService struct {
	view       *domain.CloneJobView
	list       domain.CloneJobList
	counts     domain.CloneCounts
	job        *domain.CloneJob
	step       *domain.CloneStepRecord
	artifact   *domain.CloneArtifact
	err        error
	calls      []string
	id         string
	artifactID string
	stepName   domain.CloneStep
	channelID  *string
	limit      int
	ctx        context.Context
}

func (f *fakeCloneService) record(ctx context.Context, action, id string) {
	f.ctx, f.id = ctx, id
	f.calls = append(f.calls, action)
}

func (f *fakeCloneService) Get(ctx context.Context, id string) (*domain.CloneJobView, error) {
	f.record(ctx, "get", id)
	return f.view, f.err
}

func (f *fakeCloneService) List(ctx context.Context, channel *string, limit int) (domain.CloneJobList, error) {
	f.record(ctx, "list", "")
	f.channelID, f.limit = channel, limit
	return f.list, f.err
}

func (f *fakeCloneService) Counts(ctx context.Context) (domain.CloneCounts, error) {
	f.record(ctx, "counts", "")
	return f.counts, f.err
}

func (f *fakeCloneService) Run(ctx context.Context, id string) (*domain.CloneJob, error) {
	f.record(ctx, "run", id)
	return f.job, f.err
}

func (f *fakeCloneService) Pause(ctx context.Context, id string) (*domain.CloneJob, error) {
	f.record(ctx, "pause", id)
	return f.job, f.err
}

func (f *fakeCloneService) Resume(ctx context.Context, id string) (*domain.CloneJob, error) {
	f.record(ctx, "resume", id)
	return f.job, f.err
}

func (f *fakeCloneService) Cancel(ctx context.Context, id string) (*domain.CloneJob, error) {
	f.record(ctx, "cancel", id)
	return f.job, f.err
}

func (f *fakeCloneService) Delete(ctx context.Context, id string) error {
	f.record(ctx, "delete", id)
	return f.err
}

func (f *fakeCloneService) Step(ctx context.Context, id string, step domain.CloneStep) (*domain.CloneStepRecord, error) {
	f.record(ctx, "step", id)
	f.stepName = step
	return f.step, f.err
}

func (f *fakeCloneService) Artifact(ctx context.Context, id, artifactID string) (*domain.CloneArtifact, error) {
	f.record(ctx, "artifact", id)
	f.artifactID = artifactID
	return f.artifact, f.err
}

type fakeCloneEnqueuer struct {
	request domain.CloneEnqueueRequest
	result  domain.CloneEnqueueResult
	err     error
	calls   int
}

func (f *fakeCloneEnqueuer) Enqueue(_ context.Context, request domain.CloneEnqueueRequest) (domain.CloneEnqueueResult, error) {
	f.request = request
	f.calls++
	return f.result, f.err
}

func cloneFixture(t *testing.T, svc *fakeCloneService, queue *fakeCloneEnqueuer, files cloneMedia) (*testGateway, string) {
	t.Helper()
	tg := newTestGateway(t)
	tg.g.Clone = svc
	tg.g.registerCloneRoutes(queue, files)
	return tg, tg.sessionCookie()
}

func TestCloneListFiltersAndPreservesViewModel(t *testing.T) {
	view := domain.CloneJobView{
		CloneJob:  domain.CloneJob{ID: "job-1", ChannelID: "channel-1", StageGroup: domain.StagePaused},
		Subline:   "Đã tạm dừng",
		AgentView: []domain.CloneAgentView{{ID: domain.AgentArtist, State: domain.AgentStopped}},
		Steps:     []domain.CloneStepRecord{},
	}
	svc := &fakeCloneService{list: domain.CloneJobList{Jobs: []domain.CloneJobView{view}, QueuedTotal: 3}}
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
	for _, tc := range []struct {
		query   string
		channel string
		limit   int
	}{
		{"", "", 10000},
		{"?channel=all", "", 10000},
		{"?channelId=channel-1&limit=10", "channel-1", 10},
		{"?channel=channel-2&limit=100", "channel-2", 100},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w := tg.do(http.MethodGet, "/api/clone"+tc.query, cookie, "")
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if svc.limit != tc.limit || (tc.channel == "" && svc.channelID != nil) || (tc.channel != "" && (svc.channelID == nil || *svc.channelID != tc.channel)) {
				t.Fatalf("filter = %v, limit = %d", svc.channelID, svc.limit)
			}
			// Compare the wire representation: a nil RawMessage is JSON null,
			// which unmarshals into a non-nil RawMessage containing "null".
			expected := svc.list
			expected.BulkEligibility = domain.BulkEligibility(expected.Jobs)
			want, err := json.Marshal(expected)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bytes.TrimSpace(w.Body.Bytes()), want) {
				t.Fatalf("view changed: got %s, want %s", w.Body.Bytes(), want)
			}
		})
	}
	before := len(svc.calls)
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=no", "?limit=", "?limit=1&limit=2", "?channel=../private", "?channel=a&channelId=b", "?unknown=x"} {
		w := tg.do(http.MethodGet, "/api/clone"+query, cookie, "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d: %s", query, w.Code, w.Body.String())
		}
	}
	if len(svc.calls) != before {
		t.Fatal("invalid query reached pipeline")
	}
}

func TestCloneEnqueueUsesQueueAndPreservesResult(t *testing.T) {
	queue := &fakeCloneEnqueuer{result: domain.CloneEnqueueResult{
		Items:      []domain.CloneQueueItem{{ID: "queued-1", ChannelID: "channel-1", State: domain.QueueQueued}},
		Duplicates: []domain.ClonePlanDuplicate{{URL: "https://youtu.be/duplicate01", State: "queued"}},
	}}
	svc := &fakeCloneService{}
	tg, cookie := cloneFixture(t, svc, queue, nil)
	w := tg.do(http.MethodPost, "/api/jobs", cookie, `{"channelId":"channel-1","urls":["https://youtu.be/source00001"],"noImages":true}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	want := domain.CloneEnqueueRequest{ChannelID: "channel-1", URLs: []string{"https://youtu.be/source00001"}, NoImages: true}
	if !reflect.DeepEqual(queue.request, want) || queue.calls != 1 || len(svc.calls) != 0 {
		t.Fatalf("queue request %#v; calls %d; pipeline calls %v", queue.request, queue.calls, svc.calls)
	}
	var got domain.CloneEnqueueResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, queue.result) {
		t.Fatalf("result changed: %#v", got)
	}
	for _, body := range []string{
		`{}`, `null`, `{"channelId":"channel-1","urls":[]}`,
		`{"channelId":"../private","urls":["https://youtu.be/source00001"]}`,
		`{"channelId":"channel-1","urls":["https://youtu.be/source00001"],"priority":"urgent"}`,
		`{"channelId":"channel-1","urls":["https://youtu.be/source00001"],"priority":"pick"}`,
		`{"channelId":"channel-1","urls":["https://youtu.be/source00001"],"priority":"batch"}`,
		`{"channelId":"channel-1","urls":["https://youtu.be/source00001"],"extra":true}`,
		`{"channelId":"channel-1","urls":["https://youtu.be/source00001"]}{}`,
	} {
		w := tg.do(http.MethodPost, "/api/jobs", cookie, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid enqueue status %d: %s", w.Code, w.Body.String())
		}
	}
	if queue.calls != 1 {
		t.Fatal("invalid enqueue reached queue")
	}
	queue.err = &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video đã có trong hàng chờ."}
	w = tg.do(http.MethodPost, "/api/jobs", cookie, `{"channelId":"channel-1","urls":["https://youtu.be/source00001"]}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("enqueue failure status %d: %s", w.Code, w.Body.String())
	}
}

func TestCloneJobActionsReadsAndDelete(t *testing.T) {
	svc := &fakeCloneService{
		job:    &domain.CloneJob{ID: "job-1", ChannelID: "channel-1", Status: domain.CloneRunning},
		view:   &domain.CloneJobView{CloneJob: domain.CloneJob{ID: "job-1"}, Subline: "Đang vẽ ảnh"},
		step:   &domain.CloneStepRecord{JobID: "job-1", Step: domain.CloneImages, Status: domain.CloneWaiting},
		counts: domain.CloneCounts{Totals: domain.CloneBucketCounts{Paused: 2}, ByOwner: map[string]domain.CloneBucketCounts{"channel-1": {Working: 1}}},
	}
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
	for _, action := range []string{"run", "pause", "resume", "cancel"} {
		w := tg.do(http.MethodPost, "/api/clone/job-1", cookie, `{"action":"`+action+`"}`)
		if w.Code != http.StatusOK || svc.calls[len(svc.calls)-1] != action || svc.id != "job-1" || decode(t, w)["id"] != "job-1" {
			t.Fatalf("%s: status %d, calls %v, body %s", action, w.Code, svc.calls, w.Body.String())
		}
	}
	w := tg.do(http.MethodGet, "/api/clone/job-1", cookie, "")
	if w.Code != http.StatusOK || decode(t, w)["subline"] != "Đang vẽ ảnh" || decode(t, w)["id"] != "job-1" {
		t.Fatalf("get: status %d: %s", w.Code, w.Body.String())
	}
	w = tg.do(http.MethodGet, "/api/clone/job-1/steps/images", cookie, "")
	if w.Code != http.StatusOK || svc.stepName != domain.CloneImages || decode(t, w)["step"].(map[string]any)["jobId"] != "job-1" {
		t.Fatalf("step: status %d: %s", w.Code, w.Body.String())
	}
	w = tg.do(http.MethodGet, "/api/clone/counts", cookie, "")
	if w.Code != http.StatusOK || svc.calls[len(svc.calls)-1] != "counts" || decode(t, w)["totals"].(map[string]any)["paused"] != float64(2) {
		t.Fatalf("counts: status %d: %s", w.Code, w.Body.String())
	}
	w = tg.do(http.MethodDelete, "/api/clone/job-1", cookie, "")
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 || svc.calls[len(svc.calls)-1] != "delete" {
		t.Fatalf("delete: status %d: %s", w.Code, w.Body.String())
	}
	before := len(svc.calls)
	for _, body := range []string{`{}`, `{"action":"rewind"}`, `{"action":"pause","extra":true}`, `{"action":"pause"}{}`, `null`} {
		w = tg.do(http.MethodPost, "/api/clone/job-1", cookie, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid action: status %d: %s", w.Code, w.Body.String())
		}
	}
	w = tg.do(http.MethodGet, "/api/clone/job-1/steps/unknown", cookie, "")
	if w.Code != http.StatusBadRequest || len(svc.calls) != before {
		t.Fatalf("invalid action or step reached pipeline; status %d, calls %v", w.Code, svc.calls)
	}
}

func TestCloneHandlersPropagateErrorsAndMissingRecords(t *testing.T) {
	svc := &fakeCloneService{err: fmt.Errorf("wrapped: %w", &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Không tìm thấy video."})}
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/clone", ""},
		{http.MethodGet, "/api/clone/counts", ""},
		{http.MethodGet, "/api/clone/job-1", ""},
		{http.MethodGet, "/api/clone/job-1/steps/images", ""},
		{http.MethodGet, "/api/clone/job-1/artifacts/image-1", ""},
		{http.MethodPost, "/api/clone/job-1", `{"action":"resume"}`},
		{http.MethodDelete, "/api/clone/job-1", ""},
	} {
		w := tg.do(tc.method, tc.path, cookie, tc.body)
		if w.Code != http.StatusNotFound || decode(t, w)["code"] != domain.CloneErrNotFound {
			t.Fatalf("%s %s: status %d: %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	svc.err = nil
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/clone/job-1", ""},
		{http.MethodGet, "/api/clone/job-1/steps/images", ""},
		{http.MethodGet, "/api/clone/job-1/artifacts/image-1", ""},
		{http.MethodPost, "/api/clone/job-1", `{"action":"pause"}`},
	} {
		w := tg.do(tc.method, tc.path, cookie, tc.body)
		if w.Code != http.StatusNotFound {
			t.Fatalf("missing record %s: status %d: %s", tc.path, w.Code, w.Body.String())
		}
	}
	svc.view = &domain.CloneJobView{CloneJob: domain.CloneJob{ID: "other-job"}}
	svc.job = &domain.CloneJob{ID: "other-job"}
	svc.step = &domain.CloneStepRecord{JobID: "other-job", Step: domain.CloneImages}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/clone/job-1", ""},
		{http.MethodGet, "/api/clone/job-1/steps/images", ""},
		{http.MethodPost, "/api/clone/job-1", `{"action":"resume"}`},
	} {
		if w := tg.do(tc.method, tc.path, cookie, tc.body); w.Code != http.StatusNotFound {
			t.Fatalf("foreign record returned: status %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestCloneErrorsPreserveRecoveryAndRedactInternals(t *testing.T) {
	retry := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	recovery := "Thử lại sau khi hết hạn mức."
	for _, tc := range []struct {
		code   string
		status int
	}{
		{domain.CloneErrInvalidRequest, 400}, {domain.CloneErrNotFound, 404},
		{domain.CloneErrConflict, 409}, {domain.CloneErrQuota, 429},
		{domain.CloneErrToolMissing, 503}, {domain.CloneErrSourceUnavailable, 410},
		{domain.CloneErrCancelled, 409}, {domain.CloneErrInterrupted, 503},
		{domain.CloneErrStale, 409}, {domain.CloneErrTemplateChanged, 409},
		{domain.CloneErrQualityGate, 409}, {domain.CloneErrAwaitingReview, 409},
	} {
		t.Run(tc.code, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteCloneError(w, fmt.Errorf("wrapped: %w", &domain.CloneError{
				Code: tc.code, Message: "Chưa thực hiện được.", Recovery: &recovery, RetryAt: &retry, RetryAtSource: domain.RetryAtProvider,
			}))
			body := decode(t, w)
			if w.Code != tc.status || body["code"] != tc.code || body["recovery"] != recovery || body["retryAt"] != "2026-10-31T12:00:00Z" || body["retryAtSource"] != "provider" {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
	for _, err := range []error{
		errors.New("database password=secret /private/jobs"),
		&domain.CloneError{Code: domain.CloneErrInternal, Message: "provider api-key=secret"},
	} {
		w := httptest.NewRecorder()
		WriteCloneError(w, err)
		if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "secret") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("internal error leaked: status %d: %s", w.Code, w.Body.String())
		}
	}
}

type fakeCloneMedia struct {
	store      blobstore.BlobStore
	descriptor media.Descriptor
	ref        media.Ref
	err        error
	protectErr error
	calls      int
	released   int
	protected  int
	path       string
}

func (f *fakeCloneMedia) Resolve(_ context.Context, path string) (blobstore.BlobStore, media.Descriptor, media.Ref, func(), error) {
	f.calls++
	f.path = path
	if f.err != nil {
		return nil, media.Descriptor{}, media.Ref{}, nil, f.err
	}
	return f.store, f.descriptor, f.ref, func() { f.released++ }, nil
}

func (f *fakeCloneMedia) Protect(_ context.Context, ref media.Ref, until time.Time) error {
	if ref != f.ref || !until.After(time.Now()) {
		return errors.New("incorrect media protection")
	}
	f.protected++
	return f.protectErr
}

func cloneArtifactFixture(t *testing.T) (*fakeCloneService, *fakeCloneMedia, *blobstore.Local) {
	t.Helper()
	store, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("0123456789")))
	key, err := media.ContentKey(digest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), key, strings.NewReader("0123456789"), blobstore.PutOptions{ExpectedSHA256: digest}); err != nil {
		t.Fatal(err)
	}
	path := "clone/job-1/" + digest + "/download/source.mp4"
	artifactID := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(path)))
	svc := &fakeCloneService{artifact: &domain.CloneArtifact{
		ID: artifactID, Path: path, SHA256: digest, Bytes: 10, Step: domain.CloneDownload,
		ContentType: "video/mp4", Filename: "source.mp4",
	}}
	files := &fakeCloneMedia{store: store, ref: media.Ref{
		Path: path, Key: key, SHA256: digest, Bytes: 10, ContentType: "video/mp4", Filename: "source.mp4",
	}, descriptor: media.Descriptor{Config: blobstore.Config{Driver: "local", Delivery: "proxy"}}}
	return svc, files, store
}

func cloneArtifactRequest(tg *testGateway, cookie, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr, r.Host = "192.0.2.1:1234", "127.0.0.1:8789"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	}
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	return w
}

// Ported from the source artifact route: ranges address only published media,
// unpublished paths cannot read neighboring files, and deleted media is a 404.
func TestCloneArtifactRangesAndImmutableContentIdentity(t *testing.T) {
	svc, files, store := cloneArtifactFixture(t)
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, files)
	path := "/api/clone/job-1/artifacts/" + url.PathEscape(svc.artifact.ID)
	etag := `"` + svc.artifact.SHA256 + `"`
	for _, tc := range []struct {
		name, method, requestedRange, ifRange, body, contentRange string
		status                                                    int
	}{
		{"full", http.MethodGet, "", "", "0123456789", "", 200},
		{"partial", http.MethodGet, "bytes=2-5", "", "2345", "bytes 2-5/10", 206},
		{"suffix", http.MethodGet, "bytes=-3", "", "789", "bytes 7-9/10", 206},
		{"open", http.MethodGet, "bytes=8-", "", "89", "bytes 8-9/10", 206},
		{"head", http.MethodHead, "bytes=2-5", "", "", "bytes 2-5/10", 206},
		{"matching if-range", http.MethodGet, "bytes=2-5", etag, "2345", "bytes 2-5/10", 206},
		{"changed if-range", http.MethodGet, "bytes=2-5", `"old"`, "0123456789", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := files.released
			w := cloneArtifactRequest(tg, cookie, tc.method, path, map[string]string{"Range": tc.requestedRange, "If-Range": tc.ifRange})
			if w.Code != tc.status || w.Body.String() != tc.body || w.Header().Get("Content-Range") != tc.contentRange {
				t.Fatalf("status %d, range %q, body %q", w.Code, w.Header().Get("Content-Range"), w.Body.String())
			}
			if w.Header().Get("ETag") != etag || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") || w.Header().Get("Accept-Ranges") != "bytes" || w.Header().Get("Content-Type") != "video/mp4" {
				t.Fatalf("headers = %v", w.Header())
			}
			if svc.id != "job-1" || svc.artifactID != svc.artifact.ID || files.path != svc.artifact.Path || files.released != before+1 {
				t.Fatal("artifact ownership or read lease was not preserved")
			}
		})
	}
	w := cloneArtifactRequest(tg, cookie, http.MethodGet, path, map[string]string{"If-None-Match": "W/" + etag})
	if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("conditional status %d: %s", w.Code, w.Body.String())
	}
	for _, requestedRange := range []string{"bytes=10-", "bytes=5-2", "bytes=0-1,3-4", "invalid"} {
		w := cloneArtifactRequest(tg, cookie, http.MethodGet, path, map[string]string{"Range": requestedRange})
		if w.Code != http.StatusRequestedRangeNotSatisfiable || w.Header().Get("Content-Range") != "bytes */10" || strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("invalid range %q: status %d, headers %v", requestedRange, w.Code, w.Header())
		}
	}
	if err := store.Delete(context.Background(), files.ref.Key); err != nil {
		t.Fatal(err)
	}
	w = cloneArtifactRequest(tg, cookie, http.MethodGet, path, nil)
	if w.Code != http.StatusNotFound || decode(t, w)["code"] != domain.CloneErrNotFound || strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("deleted media status %d: %s", w.Code, w.Body.String())
	}
}

func TestCloneArtifactFencesManifestAndMediaBinding(t *testing.T) {
	svc, files, _ := cloneArtifactFixture(t)
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, files)
	original := *svc.artifact
	ref := files.ref
	path := "/api/clone/job-1/artifacts/" + url.PathEscape(original.ID)
	for _, tc := range []struct {
		name    string
		mutate  func()
		status  int
		resolve bool
	}{
		{"unpublished", func() { svc.artifact = nil }, 404, false},
		{"wrong artifact", func() { svc.artifact.ID = "other-id" }, 404, false},
		{"other job", func() { svc.artifact.Path = strings.Replace(original.Path, "job-1", "job-2", 1) }, 404, false},
		{"traversal", func() { svc.artifact.Path = "clone/job-1/../private.txt" }, 400, false},
		{"backslash", func() { svc.artifact.Path = `clone/job-1/..\private.txt` }, 400, false},
		{"absolute", func() { svc.artifact.Path = "/private/file.txt" }, 400, false},
		{"missing digest", func() { svc.artifact.SHA256 = "" }, 400, false},
		{"mutable path", func() { svc.artifact.Path = "clone/job-1/download/source.mp4" }, 404, false},
		{"wrong binding path", func() { files.ref.Path = "clone/job-2/source.mp4" }, 409, true},
		{"wrong binding hash", func() { files.ref.SHA256 = strings.Repeat("a", 64) }, 409, true},
		{"wrong binding size", func() { files.ref.Bytes++ }, 409, true},
		{"wrong binding key", func() { files.ref.Key = "v1/private" }, 409, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := original
			svc.artifact, files.ref = &copy, ref
			beforeCalls, beforeRelease := files.calls, files.released
			tc.mutate()
			w := cloneArtifactRequest(tg, cookie, http.MethodGet, path, nil)
			if w.Code != tc.status || strings.Contains(w.Body.String(), "0123456789") || strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
				t.Fatalf("status %d: %s; headers %v", w.Code, w.Body.String(), w.Header())
			}
			want := 0
			if tc.resolve {
				want = 1
			}
			if files.calls-beforeCalls != want || files.released-beforeRelease != want {
				t.Fatalf("resolve/release delta %d/%d, want %d", files.calls-beforeCalls, files.released-beforeRelease, want)
			}
		})
	}
}

func TestCloneArtifactMediaErrorsAndProtection(t *testing.T) {
	svc, files, _ := cloneArtifactFixture(t)
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, files)
	path := "/api/clone/job-1/artifacts/" + url.PathEscape(svc.artifact.ID)
	files.err = blobstore.ErrNotFound
	w := cloneArtifactRequest(tg, cookie, http.MethodGet, path, nil)
	if w.Code != http.StatusNotFound || files.released != 0 {
		t.Fatalf("resolve failure status %d: %s", w.Code, w.Body.String())
	}
	files.err = nil
	files.descriptor.Config.Driver, files.descriptor.Config.Delivery = "s3", "redirect"
	files.protectErr = errors.New("lease database secret")
	w = cloneArtifactRequest(tg, cookie, http.MethodGet, path, nil)
	if w.Code != http.StatusInternalServerError || files.protected != 1 || files.released != 1 || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("protection failure status %d: %s", w.Code, w.Body.String())
	}
	files.protectErr = nil
	w = cloneArtifactRequest(tg, cookie, http.MethodGet, path+"?download=1", nil)
	if w.Code != http.StatusOK || files.protected != 2 || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("protected download status %d: %s; headers %v", w.Code, w.Body.String(), w.Header())
	}
}

func TestCloneRoutesRequireSessionAndSameOrigin(t *testing.T) {
	svc, files, _ := cloneArtifactFixture(t)
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, files)
	path := "/api/clone/job-1/artifacts/" + url.PathEscape(svc.artifact.ID)
	for _, route := range []string{"/api/clone", "/api/clone/counts", "/api/clone/job-1", "/api/clone/job-1/steps/images", path} {
		w := tg.do(http.MethodGet, route, "", "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s: status %d", route, w.Code)
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, headers := range []map[string]string{
			{"Origin": "https://example.com", "Sec-Fetch-Site": "cross-site"},
			{"Sec-Fetch-Site": "cross-site"},
			{"Origin": "http://127.0.0.1:8790", "Sec-Fetch-Site": "same-site"},
			{"Origin": "null"},
		} {
			w := cloneArtifactRequest(tg, cookie, method, path, headers)
			if w.Code != http.StatusForbidden || len(svc.calls) != 0 || files.calls != 0 {
				t.Fatalf("%s with %v reached artifact: status %d", method, headers, w.Code)
			}
		}
	}
	for _, route := range []string{"/api/clone/job-1/artifacts/..%2Fprivate", "/api/clone/job-1/artifacts/%252e%252e", "/api/clone/%2e%2e"} {
		w := cloneArtifactRequest(tg, cookie, http.MethodGet, route, nil)
		if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
			t.Fatalf("traversal %s: status %d", route, w.Code)
		}
	}
	if len(svc.calls) != 0 || files.calls != 0 {
		t.Fatal("traversal reached pipeline or media")
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := cloneArtifactRequest(tg, cookie, method, path, map[string]string{
			"Origin": "http://127.0.0.1:8789", "Sec-Fetch-Site": "same-origin",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("same-origin %s: status %d: %s", method, w.Code, w.Body.String())
		}
	}
}

func TestCloneMountLeavesSpecificQueueRoutesReachable(t *testing.T) {
	tg := newTestGateway(t)
	for _, path := range []string{"/api/clone/plan"} {
		tg.g.Handle(http.MethodPost, path, func(w http.ResponseWriter, _ *http.Request, _ identity.User) {
			w.WriteHeader(http.StatusAccepted)
		})
	}
	svc := &fakeCloneService{}
	MountClone(tg.g, svc, &fakeCloneEnqueuer{}, nil)
	cookie := tg.sessionCookie()
	for _, path := range []string{"/api/clone/plan"} {
		if w := tg.do(http.MethodPost, path, cookie, `{}`); w.Code != http.StatusAccepted {
			t.Fatalf("specific queue route %s was shadowed: %d", path, w.Code)
		}
	}
	if len(svc.calls) != 0 {
		t.Fatal("queue route reached pipeline")
	}
}

func TestCloneUnmountedDependenciesReturnServiceUnavailable(t *testing.T) {
	tg := newTestGateway(t)
	MountClone(tg.g, nil, nil, nil)
	cookie := tg.sessionCookie()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/clone", ""},
		{http.MethodGet, "/api/clone/counts", ""},
		{http.MethodGet, "/api/clone/job-1", ""},
		{http.MethodPost, "/api/jobs", `{"channelId":"channel-1","urls":["https://youtu.be/source00001"]}`},
	} {
		w := tg.do(tc.method, tc.path, cookie, tc.body)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("missing dependency %s: status %d: %s", tc.path, w.Code, w.Body.String())
		}
	}
}
