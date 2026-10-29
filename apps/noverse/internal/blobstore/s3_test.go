package blobstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// minioConfig builds the S3Config for the disposable MinIO from
// deploy/compose.test.yml (ports 59000/59001). Like pgtest, absence is a skip
// unless REQUIRE_MINIO_TEST=1 makes it a hard failure, so acceptance runs
// cannot pass on a skipped suite.
func minioConfig(t *testing.T) S3Config {
	t.Helper()
	endpoint := strings.TrimSpace(os.Getenv("MINIO_TEST_ENDPOINT"))
	if endpoint == "" {
		if os.Getenv("REQUIRE_MINIO_TEST") == "1" {
			t.Fatal("REQUIRE_MINIO_TEST=1 but MINIO_TEST_ENDPOINT is unset")
		}
		t.Skip("MINIO_TEST_ENDPOINT unset — skipping MinIO suite")
	}
	return S3Config{
		Endpoint:        endpoint,
		Region:          envOr("MINIO_TEST_REGION", "us-east-1"),
		Bucket:          envOr("MINIO_TEST_BUCKET", "blobstore-test"),
		AccessKeyID:     envOr("MINIO_TEST_ACCESS_KEY", "minioadmin"),
		SecretAccessKey: envOr("MINIO_TEST_SECRET_KEY", "minioadmin123"),
		ForcePathStyle:  true,
		AllowPlainHTTP:  true, // MinIO test container on the private docker net
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// newMinioStore builds the store and ensures a fresh bucket exists. Buckets
// are per-test-run unique so reruns never collide.
func newMinioStore(t *testing.T) *S3 {
	t.Helper()
	cfg := minioConfig(t)
	cfg.Bucket = fmt.Sprintf("%s-%d", cfg.Bucket, time.Now().UnixNano()%1_000_000_000)
	st, err := NewS3(cfg)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Wait for MinIO readiness (compose has no guaranteed ordering).
	deadline := time.Now().Add(20 * time.Second)
	for {
		_, err := st.Client().CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(cfg.Bucket)})
		if err == nil {
			break
		}
		var bae *s3types.BucketAlreadyOwnedByYou
		var ba *s3types.BucketAlreadyExists
		if errors.As(err, &bae) || errors.As(err, &ba) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("CreateBucket (minio ready?): %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Cleanup(func() {
		// Best-effort teardown: empty then drop the bucket.
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
		_, _ = st.Client().DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(cfg.Bucket)})
	})
	return st
}

// TestConformanceMinIO runs the identical suite as local against real MinIO —
// real SDK, real network, no skip under REQUIRE_MINIO_TEST.
func TestConformanceMinIO(t *testing.T) {
	newMinioStore(t) // fails/skips before subtests if MinIO absent
	runConformance(t, func(t *testing.T) BlobStore { return newMinioStore(t) })
}

// TestMultipartAbort: a source that errors mid-upload must trigger the
// uploader's abort — no committed object and no orphaned multipart upload.
func TestMultipartAbort(t *testing.T) {
	st := newMinioStore(t)
	ctx := context.Background()
	// 12 MiB across three 5 MiB parts, then the reader dies.
	_, err := st.Put(ctx, "abort/victim", io.MultiReader(
		io.LimitReader(rand.Reader, 12<<20),
		failReader{},
	), PutOptions{})
	if err == nil {
		t.Fatal("Put of failing reader must fail")
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
	cfg := minioConfig(t)
	cfg.Bucket = fmt.Sprintf("%s-corrupt-%d", cfg.Bucket, time.Now().UnixNano()%1_000_000_000)
	clean, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := clean.Client().CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(cfg.Bucket)}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	defer func() {
		_ = clean.Delete(ctx, "corrupt/victim")
		_, _ = clean.Client().DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(cfg.Bucket)})
	}()
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
