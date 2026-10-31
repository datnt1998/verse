package blobstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type capturedS3Request struct {
	method string
	query  string
	header http.Header
}

type s3RequestCapture struct {
	sync.Mutex
	requests []capturedS3Request
}

func (c *s3RequestCapture) add(r *http.Request) {
	c.Lock()
	defer c.Unlock()
	header := r.Header.Clone()
	header.Del("Authorization")
	c.requests = append(c.requests, capturedS3Request{r.Method, r.URL.RawQuery, header})
}

func (c *s3RequestCapture) take() []capturedS3Request {
	c.Lock()
	defer c.Unlock()
	out := c.requests
	c.requests = nil
	return out
}

// Capture the real signed HTTPS requests at a TLS proxy, not SDK inputs.
func capturedMinioStore(t *testing.T, delay time.Duration) (*S3, *s3RequestCapture) {
	t.Helper()
	clean := newMinioStore(t)
	upstream, err := url.Parse(clean.cfg.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	capture := &s3RequestCapture{}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ModifyResponse = func(*http.Response) error { time.Sleep(delay / 2); return nil }
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.add(r)
		time.Sleep(delay / 2)
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	cfg := clean.cfg
	cfg.Endpoint = server.URL
	cfg.HTTPClient = server.Client()
	store, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Measure steady-state operations separately from the one-time capability
	// probe. Capability tests below exercise the lazy path and its concurrency.
	state, err := store.probeConditional(context.Background(), "dev/test-fixture")
	if err != nil {
		t.Fatal(err)
	}
	store.conditionalState.Store(state)
	capture.take()
	return store, capture
}

func TestS3VerifyBeforePublishing(t *testing.T) {
	store, capture := capturedMinioStore(t, 0)
	ctx := context.Background()
	// Exact destructive-overwrite scenario from /tmp/s3audit/repro.
	good := []byte("legitimate referenced immutable bytes")
	sum := sha256.Sum256(good)
	sha := hex.EncodeToString(sum[:])
	key := "dev/audit/existing-good"
	if _, err := store.Put(ctx, key, bytes.NewReader(good), PutOptions{ExpectedSHA256: sha}); err != nil {
		t.Fatal(err)
	}
	capture.take()
	_, err := store.Put(ctx, key, strings.NewReader("corrupt replacement"), PutOptions{ExpectedSHA256: sha})
	if !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("mismatch = %v", err)
	}
	if requests := capture.take(); len(requests) != 0 {
		t.Errorf("digest mismatch reached final storage: %+v", requests)
	}
	body, _, err := store.Open(ctx, key, FullRange)
	if err != nil {
		t.Fatalf("previously good object lost: %v", err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(got, good) {
		t.Fatalf("good object changed: %q, %v", got, err)
	}

	capture.take()
	if _, err := store.Put(ctx, "dev/audit/new-bad", strings.NewReader("bad"), PutOptions{ExpectedSHA256: sha}); !errors.Is(err, ErrDigestMismatch) {
		t.Fatal(err)
	}
	if requests := capture.take(); len(requests) != 0 {
		t.Fatalf("new corrupt object reached final storage: %+v", requests)
	}

	// A corrupt writer paused before EOF must not interfere with a concurrent
	// verified retry of the same tenant key.
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	failed := make(chan error, 1)
	go func() {
		_, err := store.Put(ctx, key, reader, PutOptions{ExpectedSHA256: sha})
		failed <- err
	}()
	if _, err := writer.Write([]byte("concurrent corrupt bytes")); err != nil {
		t.Fatal(err)
	}
	capture.take()
	if _, err := store.Put(ctx, key, bytes.NewReader(good), PutOptions{ExpectedSHA256: sha}); err != nil {
		t.Fatal(err)
	}
	requests := capture.take()
	want := 2
	if store.ConditionalPutSupport() == "unsupported" {
		want = 1
	}
	if len(requests) != want || requests[len(requests)-1].method != "HEAD" {
		t.Errorf("concurrent identical publication did not deduplicate: %+v", requests)
	}
	writer.Close()
	select {
	case err := <-failed:
		if !errors.Is(err, ErrDigestMismatch) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("corrupt writer did not complete")
	}
	body, _, err = store.Open(ctx, key, FullRange)
	if err != nil {
		t.Fatal(err)
	}
	got, err = io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(got, good) {
		t.Fatalf("concurrent writer damaged final bytes: %q %v", got, err)
	}
}

func TestS3HashMetadataAndDeduplicate(t *testing.T) {
	store, capture := capturedMinioStore(t, 0)
	ctx := context.Background()
	data := []byte("hash even without an expected digest")
	put, err := store.Put(ctx, "metadata/computed", bytes.NewReader(data), PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	requests := capture.take()
	want := 1
	if store.ConditionalPutSupport() == "unsupported" {
		want = 2
	}
	if len(requests) != want || requests[len(requests)-1].method != "PUT" ||
		(store.ConditionalPutSupport() == "supported" && requests[0].header.Get("If-None-Match") != "*") {
		t.Errorf("PUT requests: %+v; want verified conditional PUT or HEAD + PUT fallback", requests)
	}
	info, err := store.Stat(ctx, put.Key)
	if err != nil {
		t.Fatal(err)
	}
	if info.SHA256 != put.SHA256 || info.Size != int64(len(data)) || info.ETag != put.ETag {
		t.Errorf("Put/Stat metadata disagree: put=%+v stat=%+v", put, info)
	}
	capture.take()
	if _, err := store.Put(ctx, put.Key, bytes.NewReader(data), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	requests = capture.take()
	want = 2
	if store.ConditionalPutSupport() == "unsupported" {
		want = 1
	}
	if len(requests) != want || requests[len(requests)-1].method != "HEAD" {
		t.Fatalf("identical retry should resolve existing content through HEAD: %+v", requests)
	}
}

func TestS3ConditionalPutConflicts(t *testing.T) {
	clean := newMinioStore(t)
	var requests []string
	cfg := clean.cfg
	cfg.HTTPClient = &http.Client{Transport: s3TestTransport(func(req *http.Request) (*http.Response, error) {
		response, err := http.DefaultTransport.RoundTrip(req)
		if err == nil {
			requests = append(requests, fmt.Sprintf("%s:%d", req.Method, response.StatusCode))
		}
		return response, err
	})}
	store, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	good := "good immutable bytes"
	first, err := store.Put(ctx, "conditional/data", strings.NewReader(good), PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantFirst := "[PUT:200 PUT:412 DELETE:204 PUT:200]"
	wantRetry := "[PUT:412 HEAD:200]"
	if store.ConditionalPutSupport() == "unsupported" {
		wantFirst = "[PUT:200 PUT:200 DELETE:204 HEAD:404 PUT:200]"
		wantRetry = "[HEAD:200]"
	}
	if fmt.Sprint(requests) != wantFirst {
		t.Errorf("new-key requests = %v, want %s", requests, wantFirst)
	}
	requests = nil
	retry, err := store.Put(ctx, first.Key, strings.NewReader(good), PutOptions{})
	if err != nil || retry.SHA256 != first.SHA256 {
		t.Errorf("identical retry = %+v, %v", retry, err)
	}
	if fmt.Sprint(requests) != wantRetry {
		t.Errorf("same-key requests = %v, want %s", requests, wantRetry)
	}
	requests = nil
	if _, err := store.Put(ctx, first.Key, strings.NewReader("different verified bytes"), PutOptions{}); !errors.Is(err, ErrDigestMismatch) {
		t.Errorf("different-content upload = %v, want ErrDigestMismatch", err)
	}
	if fmt.Sprint(requests) != wantRetry {
		t.Errorf("conflict requests = %v, want %s", requests, wantRetry)
	}
	body, _, err := clean.Open(ctx, first.Key, FullRange)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || string(got) != good {
		t.Fatalf("conflict replaced existing object: %q, %v", got, err)
	}
}

func TestS3ConditionalPutUnsupportedFallback(t *testing.T) {
	clean := newMinioStore(t)
	var requests []string
	var probes int
	cfg := clean.cfg
	cfg.HTTPClient = &http.Client{Transport: s3TestTransport(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("If-None-Match") == "*" {
			probes++
			requests = append(requests, "PUT:501")
			return &http.Response{
				StatusCode: http.StatusNotImplemented, Status: "501 Not Implemented",
				Header:  http.Header{"Content-Type": []string{"application/xml"}},
				Body:    io.NopCloser(strings.NewReader("<Error><Code>NotImplemented</Code><Message>conditional header unsupported</Message></Error>")),
				Request: req,
			}, nil
		}
		response, err := http.DefaultTransport.RoundTrip(req)
		if err == nil {
			requests = append(requests, fmt.Sprintf("%s:%d", req.Method, response.StatusCode))
		}
		return response, err
	})}
	store, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, key := range []string{"fallback/first", "fallback/second"} {
		requests = nil
		if _, err := store.Put(ctx, key, strings.NewReader("good"), PutOptions{}); err != nil {
			t.Fatal(err)
		}
		want := "[HEAD:404 PUT:200]"
		if key == "fallback/first" {
			want = "[PUT:200 PUT:501 DELETE:204 HEAD:404 PUT:200]"
		}
		if fmt.Sprint(requests) != want {
			t.Errorf("fallback requests = %v, want %s", requests, want)
		}
	}
	if probes != 1 {
		t.Errorf("unsupported conditional probes = %d, want once per instance", probes)
	}
	requests = nil
	if _, err := store.Put(ctx, "fallback/first", strings.NewReader("good"), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(requests) != "[HEAD:200]" {
		t.Errorf("fallback identical retry requests = %v", requests)
	}
	if _, err := store.Put(ctx, "fallback/first", strings.NewReader("different"), PutOptions{}); !errors.Is(err, ErrDigestMismatch) {
		t.Errorf("fallback conflict = %v", err)
	}
	body, _, err := clean.Open(ctx, "fallback/first", FullRange)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || string(got) != "good" {
		t.Fatalf("fallback conflict replaced existing bytes: %q, %v", got, err)
	}
}

func TestS3ChecksumRequests(t *testing.T) {
	for _, size := range []int{100 << 10, 10 << 20} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			store, capture := capturedMinioStore(t, 0)
			put, err := store.Put(context.Background(), "checksum/data", bytes.NewReader(make([]byte, size)), PutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			requests := capture.take()
			parts := 0
			for _, request := range requests {
				if strings.Contains(request.query, "partNumber=") {
					parts++
				}
				for name := range request.header {
					lower := strings.ToLower(name)
					if strings.Contains(lower, "checksum") || lower == "x-amz-trailer" || lower == "trailer" {
						t.Errorf("automatic checksum header on %s ?%s: %s=%s", request.method, request.query, name, request.header.Get(name))
					}
				}
			}
			want := 1
			if store.ConditionalPutSupport() == "unsupported" {
				want = 2
			} // HEAD + PUT.
			if size > int(defaultS3PartSize) {
				want = 5
				if parts != 2 {
					t.Errorf("multipart parts = %d, want 2", parts)
				}
			}
			if len(requests) != want {
				t.Errorf("request count = %d, want %d: %+v", len(requests), want, requests)
			}
			info, err := store.Stat(context.Background(), put.Key)
			if err != nil || info.SHA256 != put.SHA256 || info.ETag != put.ETag || info.Size != int64(size) {
				t.Errorf("upload metadata: put=%+v stat=%+v err=%v", put, info, err)
			}
			if size > int(defaultS3PartSize) && !strings.Contains(put.ETag, "-") {
				t.Errorf("multipart ETag lost: %q", put.ETag)
			}
		})
	}
}

func TestS3OpenSingleGET(t *testing.T) {
	store, capture := capturedMinioStore(t, 0)
	ctx := context.Background()
	put, err := store.Put(ctx, "open/data", strings.NewReader("0123456789"), PutOptions{ContentType: "audio/wav"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		rng  ByteRange
		want string
	}{
		{"full", FullRange, "0123456789"},
		{"bounded", ByteRange{Offset: 2, Length: 3}, "234"},
		{"open-ended", ByteRange{Offset: 7, Length: -1}, "789"},
		{"clamped", ByteRange{Offset: 8, Length: 10}, "89"},
		{"overflow", ByteRange{Offset: 8, Length: 1<<63 - 1}, "89"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture.take()
			body, info, err := store.Open(ctx, put.Key, tc.rng)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(body)
			body.Close()
			if err != nil || string(got) != tc.want {
				t.Errorf("body = %q, %v", got, err)
			}
			if info.Size != 10 || info.ContentType != "audio/wav" || info.SHA256 != put.SHA256 || info.ETag != put.ETag || info.ModTime.IsZero() {
				t.Errorf("whole-object metadata = %+v", info)
			}
			if requests := capture.take(); len(requests) != 1 || requests[0].method != "GET" {
				t.Errorf("Open requests: %+v; want one GET", requests)
			}
		})
	}
	for _, rng := range []ByteRange{{Offset: -1, Length: 2}, {Offset: 10, Length: -1}, {Offset: 11, Length: 0}} {
		if body, _, err := store.Open(ctx, put.Key, rng); !errors.Is(err, ErrInvalidRange) {
			if body != nil {
				body.Close()
			}
			t.Errorf("range %+v: %v", rng, err)
		}
	}
	body, info, err := store.Open(ctx, put.Key, ByteRange{Offset: 2, Length: 0})
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || len(got) != 0 || info.Size != 10 {
		t.Errorf("empty range = %q, %+v, %v", got, info, err)
	}
	if _, err := store.Put(ctx, "open/empty", strings.NewReader(""), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, rng := range []ByteRange{FullRange, {Offset: 0, Length: 0}, {Offset: 0, Length: 3}} {
		body, info, err := store.Open(ctx, "open/empty", rng)
		if err != nil {
			t.Fatalf("empty object range %+v: %v", rng, err)
		}
		got, err := io.ReadAll(body)
		body.Close()
		if err != nil || len(got) != 0 || info.Size != 0 {
			t.Errorf("empty object range %+v: %q %+v %v", rng, got, info, err)
		}
	}
}

func TestS3UploadPolicy(t *testing.T) {
	store := newMinioStore(t)
	for _, tc := range []struct {
		key                string
		opts               PutOptions
		cache, disposition string
	}{
		{"policy/default", PutOptions{}, "private, no-store", "inline"},
		{"policy/download", PutOptions{CacheControl: "private, max-age=300", ContentDisposition: ContentDisposition("attachment", "Chương một.wav")}, "private, max-age=300", ContentDisposition("attachment", "Chương một.wav")},
	} {
		if _, err := store.Put(context.Background(), tc.key, strings.NewReader(tc.key), tc.opts); err != nil {
			t.Fatal(err)
		}
		head, err := store.Client().HeadObject(context.Background(), &s3.HeadObjectInput{Bucket: aws.String(store.Bucket()), Key: aws.String(tc.key)})
		if err != nil {
			t.Fatal(err)
		}
		if aws.ToString(head.CacheControl) != tc.cache || aws.ToString(head.ContentDisposition) != tc.disposition {
			t.Errorf("policy for %s = %q / %q", tc.key, aws.ToString(head.CacheControl), aws.ToString(head.ContentDisposition))
		}
	}
}

func TestS3SpoolFailureCleanup(t *testing.T) {
	store, capture := capturedMinioStore(t, 0)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	_, err := store.Put(context.Background(), "spool/bad-source", io.MultiReader(io.LimitReader(zeroReader{}, 12<<20), failReader{}), PutOptions{})
	if err == nil {
		t.Fatal("failing source accepted")
	}
	if requests := capture.take(); len(requests) != 0 {
		t.Errorf("unverified source caused requests: %+v", requests)
	}
	store.cfg.MaxParts = 1
	store.cfg.PartSize = 5 << 20
	if _, err := store.Put(context.Background(), "spool/too-large", io.LimitReader(zeroReader{}, (5<<20)+1), PutOptions{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("bounded spool = %v", err)
	}
	if requests := capture.take(); len(requests) != 0 {
		t.Errorf("over-limit bytes reached storage: %+v", requests)
	}
	// Cancellation while consuming a caller's source also cleans the spool.
	ctx, cancel := context.WithCancel(context.Background())
	source := cancelSource{cancel: cancel}
	if _, err := store.Put(ctx, "spool/cancelled", source, PutOptions{}); !errors.Is(err, context.Canceled) {
		t.Errorf("spool cancellation = %v", err)
	}
	if requests := capture.take(); len(requests) != 0 {
		t.Errorf("cancelled source reached storage: %+v", requests)
	}
	if _, err := store.Put(context.Background(), "spool/success", strings.NewReader("verified"), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil || len(entries) != 0 {
		t.Errorf("spool files leaked: %v, %v", entries, err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type cancelSource struct{ cancel context.CancelFunc }

func (r cancelSource) Read(p []byte) (int, error) {
	r.cancel()
	p[0] = 'x'
	return 1, nil
}

func TestS3RejectEndpointSecrets(t *testing.T) {
	for _, endpoint := range []string{"https://user:secret@s3.example.com", "https://s3.example.com?token=secret", "https://s3.example.com#secret", "https://s3.example.com?"} {
		_, err := NewS3(S3Config{Endpoint: endpoint, Region: "auto", Bucket: "b", AccessKeyID: "k", SecretAccessKey: "s"})
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Errorf("endpoint accepted or leaked: %v", err)
		}
	}
}

func TestS3PresignDispositionAndTTL(t *testing.T) {
	store, err := NewS3(S3Config{Endpoint: "https://s3.example.com", Region: "auto", Bucket: "bucket", AccessKeyID: "k", SecretAccessKey: "s", MaxPresignTTL: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		opts DownloadOptions
		want string
	}{
		{DownloadOptions{Disposition: "inline"}, "inline"},
		{DownloadOptions{Filename: "Chương một.wav", Disposition: "inline"}, ContentDisposition("inline", "Chương một.wav")},
		{DownloadOptions{Filename: "Chương một.wav"}, ContentDisposition("attachment", "Chương một.wav")},
	} {
		signed, err := store.PresignGet(context.Background(), "presign/data", time.Hour, tc.opts)
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(signed)
		if err != nil {
			t.Fatal(err)
		}
		if got := u.Query().Get("response-content-disposition"); got != tc.want {
			t.Errorf("signed disposition = %q, want %q", got, tc.want)
		}
		if got := u.Query().Get("X-Amz-Expires"); got != "30" {
			t.Errorf("signed expiry = %q, want configured cap 30", got)
		}
	}
	if _, err := store.PresignGet(context.Background(), "presign/data", 0, DownloadOptions{Disposition: "invalid"}); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("invalid disposition = %v", err)
	}
}

func TestS3TransportReuseAndCancellation(t *testing.T) {
	const concurrency = 16
	arrived := make(chan struct{}, concurrency)
	release := make(chan struct{}, concurrency)
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bucket/stall" {
			<-r.Context().Done()
			return
		}
		arrived <- struct{}{}
		select {
		case <-release:
			w.Header().Set("Content-Length", "0")
		case <-r.Context().Done():
		}
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	store, err := NewS3(S3Config{Endpoint: server.URL, Region: "auto", Bucket: "bucket", AccessKeyID: "k", SecretAccessKey: "s", AllowPlainHTTP: true, ForcePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for range 2 {
		errs := make(chan error, concurrency)
		for range concurrency {
			go func() { _, err := store.Stat(ctx, "reuse"); errs <- err }()
		}
		for range concurrency {
			select {
			case <-arrived:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		for range concurrency {
			release <- struct{}{}
		}
		for range concurrency {
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := connections.Load(); got != concurrency {
		t.Errorf("two 16-request waves opened %d connections; want reuse of all 16", got)
	}
	stalled, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	start := time.Now()
	if _, err := store.Stat(stalled, "stall"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("stalled headers cancellation = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancel did not interrupt stalled headers")
	}
}

// Opt-in measurements use the same 30ms + 30ms exchange delay as the audit.
// The test's payload has no bandwidth cap; it does not model production R2.
func TestS3ProxyMeasurements(t *testing.T) {
	if os.Getenv("S3_MEASURE") != "1" {
		t.Skip("opt-in 60ms proxy measurements")
	}
	store, capture := capturedMinioStore(t, 60*time.Millisecond)
	median := func(values []float64) float64 { sort.Float64s(values); return values[len(values)/2] }
	for _, size := range []int{100 << 10, 5 << 20, 60 << 20} {
		data := make([]byte, size)
		var puts, gets, firsts, ranges []float64
		var putCount, getCount, rangeCount int
		for rep := range 3 {
			key := fmt.Sprintf("measure/%d/%d", size, rep)
			capture.take()
			start := time.Now()
			_, err := store.Put(context.Background(), key, bytes.NewReader(data), PutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			puts = append(puts, float64(time.Since(start).Microseconds())/1000)
			putCount = len(capture.take())
			start = time.Now()
			body, _, err := store.Open(context.Background(), key, FullRange)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(body, make([]byte, 1)); err != nil {
				t.Fatal(err)
			}
			firsts = append(firsts, float64(time.Since(start).Microseconds())/1000)
			if _, err := io.Copy(io.Discard, body); err != nil {
				t.Fatal(err)
			}
			body.Close()
			gets = append(gets, float64(time.Since(start).Microseconds())/1000)
			getCount = len(capture.take())
			start = time.Now()
			body, _, err = store.Open(context.Background(), key, ByteRange{Offset: 1024, Length: 65536})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(body, make([]byte, 1)); err != nil {
				t.Fatal(err)
			}
			ranges = append(ranges, float64(time.Since(start).Microseconds())/1000)
			if _, err := io.Copy(io.Discard, body); err != nil {
				t.Fatal(err)
			}
			body.Close()
			rangeCount = len(capture.take())
		}
		t.Logf("bytes=%d PUT_ms=%.3f GET_ms=%.3f GET_TTFB_ms=%.3f range_TTFB_ms=%.3f requests_put/get/range=%d/%d/%d", size, median(puts), median(gets), median(firsts), median(ranges), putCount, getCount, rangeCount)
	}
}
