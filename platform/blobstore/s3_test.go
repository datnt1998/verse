package blobstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/datnt1998/verse/platform/blobstore/s3test"
)

// minioConfig is the shared S3 configuration for the disposable Garage.
// Missing infrastructure is a skip unless REQUIRE_S3_TEST=1.
func minioConfig(t *testing.T) S3Config {
	t.Helper()
	endpoint := strings.TrimSpace(os.Getenv("S3_TEST_ENDPOINT"))
	if endpoint == "" {
		if os.Getenv("REQUIRE_S3_TEST") == "1" {
			t.Fatal("REQUIRE_S3_TEST=1 but S3_TEST_ENDPOINT is unset")
		}
		t.Skip("S3_TEST_ENDPOINT unset — skipping S3 suite")
	}
	return S3Config{
		Endpoint:        endpoint,
		Region:          envOr("S3_TEST_REGION", "garage"),
		Bucket:          envOr("S3_TEST_BUCKET", "blobstore-test"),
		AccessKeyID:     envOr("S3_TEST_ACCESS_KEY", ""),
		SecretAccessKey: envOr("S3_TEST_SECRET_KEY", ""),
		ForcePathStyle:  true,
		AllowPlainHTTP:  true, // Private disposable test endpoint.
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// newMinioStore provisions an isolated bucket via Garage's admin API.
func newMinioStore(t *testing.T) *S3 {
	t.Helper()
	cfg := minioConfig(t)
	seed, err := NewS3(cfg)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	cfg.Bucket = s3test.Bucket(t, seed.Client(), cfg.AccessKeyID, cfg.Bucket)
	st, err := NewS3(cfg)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	t.Cleanup(func() {
		// Empty objects before the provisioning helper deletes the bucket.
		ctx := context.Background()
		cursor := ""
		for {
			page, err := st.List(ctx, "", cursor, 1000)
			if err != nil {
				return
			}
			for _, bi := range page.Infos {
				_ = st.Delete(ctx, bi.Key)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
	})
	return st
}

// TestConformanceMinIO runs the identical suite against the real S3 service.
// Real SDK and network; REQUIRE_S3_TEST prevents a skipped acceptance run.
func TestConformanceMinIO(t *testing.T) {
	newMinioStore(t)
	runConformance(t, func(t *testing.T) BlobStore { return newMinioStore(t) })
}

// TestMultipartAbort: a part failure after a verified spool must trigger the
// uploader's abort — no committed object and no orphaned multipart upload.
func TestMultipartAbort(t *testing.T) {
	st := newMinioStore(t)
	ctx := context.Background()
	var aborts atomic.Int32
	cfg := st.cfg
	cfg.HTTPClient = &http.Client{Transport: s3TestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Has("partNumber") {
			return &http.Response{
				StatusCode: http.StatusBadRequest, Status: "400 Bad Request",
				Header:  http.Header{"Content-Type": []string{"application/xml"}},
				Body:    io.NopCloser(strings.NewReader("<Error><Code>InvalidRequest</Code><Message>injected part failure</Message></Error>")),
				Request: r,
			}, nil
		}
		if r.Method == http.MethodDelete && r.URL.Query().Has("uploadId") {
			aborts.Add(1)
		}
		return http.DefaultTransport.RoundTrip(r)
	})}
	failing, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = failing.Put(ctx, "abort/victim", io.LimitReader(zeroReader{}, 12<<20), PutOptions{})
	if err == nil {
		t.Fatal("Put of failing multipart upload must fail")
	}
	if aborts.Load() != 1 {
		t.Fatalf("multipart abort requests = %d, want 1", aborts.Load())
	}
	if _, err := st.Stat(ctx, "abort/victim"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("aborted object must not exist, stat err = %v", err)
	}
	out, err := st.Client().ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(st.Bucket()),
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads: %v", err)
	}
	if len(out.Uploads) != 0 {
		t.Fatalf("orphaned multipart uploads: %+v", out.Uploads)
	}
}

type s3TestTransport func(*http.Request) (*http.Response, error)

func (f s3TestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// corruptBody round-trips every response body with the first byte flipped —
// simulating a corrupted transport. Our own SHA-256 (never the ETag) must
// catch it on read-back.
type corruptBody struct{ base http.RoundTripper }

func (c corruptBody) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if req.Method == http.MethodGet && resp.StatusCode == 200 && resp.Body != nil {
		b, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr == nil && len(b) > 0 {
			b[0] ^= 0xff
			resp.Body = io.NopCloser(bytes.NewReader(b))
		}
	}
	return resp, err
}

// TestCorruptTransportDetection: bytes mangled in flight are caught by the
// canonical SHA-256 comparison, proving integrity does not rest on ETag or
// provider checksums.
func TestCorruptTransportDetection(t *testing.T) {
	clean := newMinioStore(t)
	cfg := clean.cfg
	ctx := context.Background()
	payload := []byte("integrity-payload-" + strings.Repeat("x", 512))
	put, err := clean.Put(ctx, "corrupt/victim", bytes.NewReader(payload), PutOptions{})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Now read through a corrupting transport and hash what we actually got.
	cfg.HTTPClient = &http.Client{Transport: corruptBody{base: http.DefaultTransport}}
	dirty, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rc, _, err := dirty.Open(ctx, "corrupt/victim", FullRange)
	if err != nil {
		t.Fatalf("Open via corrupt transport: %v", err)
	}
	gotSHA, _, err := DigestAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if gotSHA == put.SHA256 {
		t.Fatal("corrupted body produced the source digest — hash check blind")
	}
}

// TestS3ConfigValidation covers the strict option parsing that stands between
// an operator typo and a wrong endpoint.
func TestS3ConfigValidation(t *testing.T) {
	base := S3Config{Region: "us-east-1", Bucket: "b", AccessKeyID: "k", SecretAccessKey: "s"}

	missing := base
	missing.Bucket = ""
	if _, err := NewS3(missing); err == nil {
		t.Fatal("empty bucket must fail")
	}
	missing = base
	missing.AccessKeyID = ""
	if _, err := NewS3(missing); err == nil {
		t.Fatal("empty credentials must fail")
	}
	missing = base
	missing.Region = ""
	if _, err := NewS3(missing); err == nil {
		t.Fatal("empty region must fail")
	}

	http := base
	http.Endpoint = "http://minio.internal:9000"
	if _, err := NewS3(http); err == nil {
		t.Fatal("plain http endpoint must require explicit opt-in")
	}
	http.AllowPlainHTTP = true
	if _, err := NewS3(http); err != nil {
		t.Fatalf("opted-in http endpoint rejected: %v", err)
	}

	badPath := base
	badPath.Endpoint = "https://s3.example.com/bucket"
	if _, err := NewS3(badPath); err == nil {
		t.Fatal("endpoint with path must fail")
	}
	smallPart := base
	smallPart.PartSize = 1 << 20
	if _, err := NewS3(smallPart); err == nil {
		t.Fatal("part size below 5MiB must fail")
	}
	if _, err := NewS3(base); err != nil {
		t.Fatalf("valid AWS-style config rejected: %v", err)
	}
}
