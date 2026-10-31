package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
)

const (
	storageStreamPath   = "/api/system/storage/test"
	storageStreamAccept = "application/x-ndjson"
)

// Step order the probe emits. Proxy delivery skips presign work and appends
// app_range; redirect delivery appends cors_probe after delete.
var (
	proxyProbeSteps    = []string{"write", "read", "range", "clock", "presign", "app_range", "delete"}
	redirectProbeSteps = []string{"write", "read", "range", "clock", "presign", "delete", "cors_probe"}
)

// storageGate is a reverse proxy in front of Garage that records every S3 request
// the app makes. The first GET is the probe's read step (Open -> GetObject); Put
// only sends PUT and HEAD, and presign is local signing. The gate holds that GET
// until the test opens it, or until the app drops the request.
type storageGate struct {
	bucket      string
	proxy       *httputil.ReverseProxy
	entered     chan struct{} // closed when the held GET arrives
	release     chan struct{} // closed by the test to let the held GET through
	aborted     chan struct{} // closed when the app drops the held GET
	releaseOnce sync.Once

	mu       sync.Mutex
	requests []recordedS3
	held     int // index into requests of the held GET; -1 until it arrives
}

type recordedS3 struct {
	Method string
	Key    string
}

func newStorageGate(bucket string, garage *url.URL) *storageGate {
	return &storageGate{
		bucket:  bucket,
		proxy:   httputil.NewSingleHostReverseProxy(garage),
		entered: make(chan struct{}),
		release: make(chan struct{}),
		aborted: make(chan struct{}),
		held:    -1,
	}
}

func (g *storageGate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := recordedS3{Method: r.Method, Key: strings.TrimPrefix(r.URL.Path, "/"+g.bucket+"/")}
	g.mu.Lock()
	g.requests = append(g.requests, rec)
	hold := r.Method == http.MethodGet && g.held < 0
	if hold {
		g.held = len(g.requests) - 1
	}
	g.mu.Unlock()
	if hold {
		close(g.entered)
		select {
		case <-g.release:
		case <-r.Context().Done():
			close(g.aborted)
			return
		}
	}
	g.proxy.ServeHTTP(w, r)
}

func (g *storageGate) open() {
	g.releaseOnce.Do(func() { close(g.release) })
}

// heldKey returns the object key of the held GET. Call only after entered is closed.
func (g *storageGate) heldKey() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.requests[g.held].Key
}

// afterHeld returns every S3 request that reached the gate after the held GET.
func (g *storageGate) afterHeld() []recordedS3 {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.held < 0 {
		return nil
	}
	return append([]recordedS3(nil), g.requests[g.held+1:]...)
}

// allRequests returns every S3 request the gate received, held or not.
func (g *storageGate) allRequests() []recordedS3 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]recordedS3(nil), g.requests...)
}

// storageStream serves the real gateway over HTTP. Its candidates point at Garage
// through the gate (via) so the first read step can be held mid-flight.
type storageStream struct {
	f      *storageHTTPFixture
	client *s3.Client
	direct bootstrap.StorageCandidate
	via    bootstrap.StorageCandidate
	gate   *storageGate
	app    *httptest.Server
}

// newStorageStream wires the app behind wrap, so a test can substitute a buffering
// response writer. storageS3 must run before storageFixture: the fixture clears S3_* env vars.
func newStorageStream(t *testing.T, wrap func(http.Handler) http.Handler) *storageStream {
	t.Helper()
	direct, client := storageS3(t)
	garage, err := url.Parse(direct.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	f := storageFixture(t, true)
	gate := newStorageGate(direct.Bucket, garage)
	gateSrv := httptest.NewServer(gate)
	t.Cleanup(gateSrv.Close)
	via := direct
	via.Endpoint = gateSrv.URL
	app := httptest.NewServer(wrap(f.fx.g))
	t.Cleanup(app.Close)
	// Cleanups run last-in first-out: release any held S3 request before the servers close.
	t.Cleanup(gate.open)
	return &storageStream{f: f, client: client, direct: direct, via: via, gate: gate, app: app}
}

// passThrough serves the real handler. The buffered proof swaps it for bufferedWrap.
func passThrough(h http.Handler) http.Handler { return h }

// bufferedResponse keeps every byte until the handler returns. Flush succeeds but
// does nothing, so the handler behaves as if flushing while the bytes stay buffered.
type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header { return b.header }

func (b *bufferedResponse) WriteHeader(code int) {
	if b.status == 0 {
		b.status = code
	}
}

func (b *bufferedResponse) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

func (b *bufferedResponse) Flush() {}

func bufferedWrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bw := &bufferedResponse{header: http.Header{}}
		h.ServeHTTP(bw, r)
		for k, v := range bw.header {
			w.Header()[k] = v
		}
		if bw.status == 0 {
			bw.status = http.StatusOK
		}
		w.WriteHeader(bw.status)
		_, _ = w.Write(bw.body.Bytes())
	})
}

func doStorageTest(ctx context.Context, base, cookie string, body any, accept string) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+storageStreamPath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", base)
	req.AddCookie(&http.Cookie{Name: fileSessionCookie, Value: cookie})
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return http.DefaultClient.Do(req)
}

func postStorageTest(t *testing.T, ctx context.Context, base, cookie string, body any, accept string) *http.Response {
	t.Helper()
	resp, err := doStorageTest(ctx, base, cookie, body, accept)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// startStorageTest posts in the background so a test can watch the gate before the
// response headers exist. The response is sent on the channel; the channel closes
// without a value if the request fails.
func startStorageTest(ctx context.Context, base, cookie string, body any, accept string) <-chan *http.Response {
	ch := make(chan *http.Response, 1)
	go func() {
		defer close(ch)
		if resp, err := doStorageTest(ctx, base, cookie, body, accept); err == nil {
			ch <- resp
		}
	}()
	return ch
}

func requireNDJSONResponse(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d, want 200: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, storageStreamAccept) {
		t.Fatalf("Content-Type %q, want %s", ct, storageStreamAccept)
	}
}

// ndjsonLines reads newline-delimited JSON on its own goroutine, so tests can wait
// for one line at a time with a deadline. The channel closes at end of body.
func ndjsonLines(body io.Reader) <-chan []byte {
	out := make(chan []byte, 32)
	go func() {
		defer close(out)
		br := bufio.NewReader(body)
		for {
			line, err := br.ReadBytes('\n')
			if line = bytes.TrimSpace(line); len(line) > 0 {
				out <- line
			}
			if err != nil {
				return
			}
		}
	}()
	return out
}

// tryLine returns the next line, io.EOF once the stream has ended, or a timeout error.
func tryLine(lines <-chan []byte, within time.Duration) ([]byte, error) {
	timer := time.NewTimer(within)
	defer timer.Stop()
	select {
	case line, ok := <-lines:
		if !ok {
			return nil, io.EOF
		}
		return line, nil
	case <-timer.C:
		return nil, errors.New("no NDJSON line within deadline")
	}
}

func nextLine(t *testing.T, lines <-chan []byte) []byte {
	t.Helper()
	line, err := tryLine(lines, 15*time.Second)
	if err != nil {
		t.Fatalf("expected next NDJSON line: %v", err)
	}
	return line
}

func requireStreamEnded(t *testing.T, lines <-chan []byte) {
	t.Helper()
	if line, err := tryLine(lines, 10*time.Second); err != io.EOF {
		t.Fatalf("stream continued after done line: err=%v line=%s", err, line)
	}
}

func decodeLine(t *testing.T, line []byte) map[string]json.RawMessage {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(line, &obj); err != nil {
		t.Fatalf("line is not one JSON object: %v: %s", err, line)
	}
	return obj
}

var stepLineKeys = map[string]bool{"step": true, "ok": true, "ms": true, "code": true, "reason": true}

// checkStepLine verifies a flat step line: the expected step name, ok=true, the
// documented keys only, and no done marker.
func checkStepLine(t *testing.T, line []byte, want string) {
	t.Helper()
	obj := decodeLine(t, line)
	if _, done := obj["done"]; done {
		t.Fatalf("step %q arrived as final line: %s", want, line)
	}
	for k := range obj {
		if !stepLineKeys[k] {
			t.Fatalf("step line has unexpected key %q: %s", k, line)
		}
	}
	for _, k := range []string{"step", "ok", "ms"} {
		if _, ok := obj[k]; !ok {
			t.Fatalf("step line missing %q: %s", k, line)
		}
	}
	var step string
	var ok bool
	if err := json.Unmarshal(obj["step"], &step); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(obj["ok"], &ok); err != nil {
		t.Fatal(err)
	}
	if step != want {
		t.Fatalf("step line %q, want %q: %s", step, want, line)
	}
	if !ok {
		t.Fatalf("step %q failed: %s", step, line)
	}
}

// checkDoneLine verifies the final line: done=true, the result fields, and the
// step list in the same order the step lines arrived. It returns corsProbeUrl.
func checkDoneLine(t *testing.T, line []byte, wantSteps []string, redirect bool) string {
	t.Helper()
	obj := decodeLine(t, line)
	var done struct {
		Done           bool   `json:"done"`
		OK             bool   `json:"ok"`
		ConditionalPut string `json:"conditionalPut"`
		Steps          []struct {
			Step string `json:"step"`
			OK   bool   `json:"ok"`
		} `json:"steps"`
		CORSProbeURL string `json:"corsProbeUrl"`
	}
	if err := json.Unmarshal(line, &done); err != nil {
		t.Fatalf("final line: %v: %s", err, line)
	}
	if _, ok := obj["step"]; ok {
		t.Fatalf("final line looks like a step line: %s", line)
	}
	if !done.Done {
		t.Fatalf("final line lacks done:true: %s", line)
	}
	if !done.OK {
		t.Fatalf("final line reports ok=false: %s", line)
	}
	if done.ConditionalPut == "" {
		t.Fatalf("final line lacks conditionalPut: %s", line)
	}
	got := make([]string, 0, len(done.Steps))
	for _, s := range done.Steps {
		if !s.OK {
			t.Fatalf("final steps include failed %q: %s", s.Step, line)
		}
		got = append(got, s.Step)
	}
	if !reflect.DeepEqual(got, wantSteps) {
		t.Fatalf("final steps %v, want %v", got, wantSteps)
	}
	if redirect {
		if done.CORSProbeURL == "" {
			t.Fatalf("redirect final line lacks corsProbeUrl: %s", line)
		}
		if _, ok := obj["cors"]; !ok {
			t.Fatalf("redirect final line lacks cors rules: %s", line)
		}
	} else if _, ok := obj["corsProbeUrl"]; ok {
		t.Fatalf("proxy final line must not carry corsProbeUrl: %s", line)
	}
	return done.CORSProbeURL
}

// requireSingleJSON checks a plain or pre-step error response: one JSON value with
// a JSON content type and no NDJSON done marker. It returns the body.
func requireSingleJSON(t *testing.T, resp *http.Response, status int) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != status {
		t.Fatalf("status %d, want %d: %s", resp.StatusCode, status, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type %q, want application/json: %s", ct, body)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		t.Fatalf("body is not JSON: %v: %s", err, body)
	}
	if _, err := dec.Token(); err != io.EOF {
		t.Fatalf("body has more than one JSON value (NDJSON?): %s", body)
	}
	if _, ok := obj["done"]; ok {
		t.Fatalf("response carries NDJSON done marker: %s", body)
	}
	return body
}

// awaitObjectGone polls HEAD until Garage returns 404 for key, or fails after 30s.
// The probe's deferred cleanup runs after the handler returns, so the object can
// outlive the cancellation briefly.
func awaitObjectGone(t *testing.T, client *s3.Client, bucket, key string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		cancel()
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("probe %q still present after cancellation (last HEAD result: %v)", key, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// streamWhileHeld waits until step 2's S3 GET is held, then requires the response
// to have started and the named step line to be readable while that GET is still
// pending. A buffering writer sends no headers and no lines until the handler
// returns, so it fails here within 5s with the GET still held.
func streamWhileHeld(t *testing.T, s *storageStream, respCh <-chan *http.Response, step string) <-chan []byte {
	t.Helper()
	select {
	case <-s.gate.entered:
	case <-time.After(15 * time.Second):
		t.Fatal("step 2 never issued its S3 GET")
	}
	var resp *http.Response
	select {
	case r, ok := <-respCh:
		if !ok {
			t.Fatal("storage test request failed before the stream started")
		}
		resp = r
	case <-time.After(5 * time.Second):
		t.Fatalf("no response headers while step 2 was held; step %q line not delivered", step)
	}
	t.Cleanup(func() { resp.Body.Close() })
	requireNDJSONResponse(t, resp)
	lines := ndjsonLines(resp.Body)
	line, err := tryLine(lines, 5*time.Second)
	if err != nil {
		t.Fatalf("step %q line not delivered while step 2 was held: %v", step, err)
	}
	checkStepLine(t, line, step)
	return lines
}

// TestStorageStreamFlushesEachStepBeforeTheNextRuns proves per-step flushing over a
// real connection. Step 2's S3 GET is held at the gate; step 1's line must reach
// the client while that GET is still pending.
func TestStorageStreamFlushesEachStepBeforeTheNextRuns(t *testing.T) {
	s := newStorageStream(t, passThrough)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	respCh := startStorageTest(ctx, s.app.URL, s.f.cookie, s.via, storageStreamAccept)

	lines := streamWhileHeld(t, s, respCh, "write")
	// Step 2 is held in flight, so no further line may appear yet.
	if line, err := tryLine(lines, 300*time.Millisecond); err == nil {
		t.Fatalf("step line arrived while step 2 was held: %s", line)
	}
	s.gate.open()
	for _, want := range proxyProbeSteps[1:] {
		checkStepLine(t, nextLine(t, lines), want)
	}
	checkDoneLine(t, nextLine(t, lines), proxyProbeSteps, false)
	requireStreamEnded(t, lines)
}

// TestStorageStreamLineOrderAndShape checks order, shape, and final-line content for
// proxy delivery and for redirect delivery, which carries corsProbeUrl.
func TestStorageStreamLineOrderAndShape(t *testing.T) {
	t.Run("proxy delivery", func(t *testing.T) {
		s := newStorageStream(t, passThrough)
		s.gate.open()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		resp := postStorageTest(t, ctx, s.app.URL, s.f.cookie, s.via, storageStreamAccept)
		requireNDJSONResponse(t, resp)
		lines := ndjsonLines(resp.Body)
		for _, want := range proxyProbeSteps {
			checkStepLine(t, nextLine(t, lines), want)
		}
		if url := checkDoneLine(t, nextLine(t, lines), proxyProbeSteps, false); url != "" {
			t.Fatalf("proxy delivery returned corsProbeUrl %q", url)
		}
		requireStreamEnded(t, lines)
	})
	t.Run("redirect delivery", func(t *testing.T) {
		s := newStorageStream(t, passThrough)
		s.gate.open()
		// Presign signs against the browser-facing endpoint. Here that is Garage
		// itself, since the probe URL is only generated, never fetched.
		candidate := s.via
		candidate.Delivery = "redirect"
		candidate.PresignEndpoint = s.direct.Endpoint
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		resp := postStorageTest(t, ctx, s.app.URL, s.f.cookie, candidate, storageStreamAccept)
		requireNDJSONResponse(t, resp)
		lines := ndjsonLines(resp.Body)
		for _, want := range redirectProbeSteps {
			checkStepLine(t, nextLine(t, lines), want)
		}
		probe := checkDoneLine(t, nextLine(t, lines), redirectProbeSteps, true)
		if !strings.HasPrefix(probe, s.direct.Endpoint) {
			t.Fatalf("corsProbeUrl %q does not use the presign endpoint %q", probe, s.direct.Endpoint)
		}
		requireStreamEnded(t, lines)
	})
}

// TestStorageStreamClientCancelStopsRemainingStepsAndCleansProbe cancels the client
// while the read GET is held. The app must drop that GET, no later step may reach
// Garage, and the staging probe must be gone afterward.
func TestStorageStreamClientCancelStopsRemainingStepsAndCleansProbe(t *testing.T) {
	s := newStorageStream(t, passThrough)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	respCh := startStorageTest(ctx, s.app.URL, s.f.cookie, s.via, storageStreamAccept)

	lines := streamWhileHeld(t, s, respCh, "write")
	cancel()
	select {
	case <-s.gate.aborted:
	case <-time.After(15 * time.Second):
		t.Fatal("app did not drop the held S3 GET after client cancellation")
	}
	if line, err := tryLine(lines, 10*time.Second); err == nil {
		t.Fatalf("line arrived after cancellation: %s", line)
	}

	key := s.gate.heldKey()
	awaitObjectGone(t, s.client, s.direct.Bucket, key)
	// Only the probe's deferred cleanup DELETE may follow the held GET. The
	// remaining steps (range, clock, presign, app_range, delete) must not run.
	want := []recordedS3{{Method: http.MethodDelete, Key: key}}
	if got := s.gate.afterHeld(); !reflect.DeepEqual(got, want) {
		t.Fatalf("requests after cancellation = %v, want only cleanup %v", got, want)
	}
}

// TestStorageStreamPreStepErrorsStayPlainJSON covers failures before the first step.
// They must keep their normal HTTP status and a plain JSON body, not NDJSON.
func TestStorageStreamPreStepErrorsStayPlainJSON(t *testing.T) {
	s := newStorageStream(t, passThrough)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	t.Run("non-admin is forbidden", func(t *testing.T) {
		viewer, err := s.f.fx.accounts.Login("hosted-viewer", "mat-khau-12345", "storage-test", "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		resp := postStorageTest(t, ctx, s.app.URL, viewer.Cookie, s.via, storageStreamAccept)
		requireSingleJSON(t, resp, http.StatusForbidden)
	})
	t.Run("unknown field is rejected before any step", func(t *testing.T) {
		body := map[string]any{"driver": "s3", "notAField": true}
		resp := postStorageTest(t, ctx, s.app.URL, s.f.cookie, body, storageStreamAccept)
		requireSingleJSON(t, resp, http.StatusBadRequest)
	})
	t.Run("invalid candidate is rejected before any step", func(t *testing.T) {
		bad := s.via
		bad.Driver = "no-such-driver"
		resp := postStorageTest(t, ctx, s.app.URL, s.f.cookie, bad, storageStreamAccept)
		requireSingleJSON(t, resp, http.StatusBadRequest)
	})
	if n := len(s.gate.allRequests()); n != 0 {
		t.Fatalf("pre-step errors reached Garage: %d S3 request(s)", n)
	}
}

// TestStorageStreamPlainJSONWithoutAcceptIsUnchanged keeps the plain response when the
// client does not ask for NDJSON: one JSON object with the full result.
func TestStorageStreamPlainJSONWithoutAcceptIsUnchanged(t *testing.T) {
	s := newStorageStream(t, passThrough)
	s.gate.open()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	resp := postStorageTest(t, ctx, s.app.URL, s.f.cookie, s.via, "")
	body := requireSingleJSON(t, resp, http.StatusOK)
	var result struct {
		OK             bool   `json:"ok"`
		ConditionalPut string `json:"conditionalPut"`
		Steps          []struct {
			Step string `json:"step"`
			OK   bool   `json:"ok"`
		} `json:"steps"`
		ProbeURL string `json:"corsProbeUrl"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.ConditionalPut == "" {
		t.Fatalf("plain result not OK or missing conditionalPut: %s", body)
	}
	got := make([]string, 0, len(result.Steps))
	for _, st := range result.Steps {
		if !st.OK {
			t.Fatalf("plain result includes failed step %q: %s", st.Step, body)
		}
		got = append(got, st.Step)
	}
	if !reflect.DeepEqual(got, proxyProbeSteps) {
		t.Fatalf("plain steps %v, want %v", got, proxyProbeSteps)
	}
	if result.ProbeURL != "" {
		t.Fatalf("proxy plain result must not carry corsProbeUrl: %s", body)
	}
}
