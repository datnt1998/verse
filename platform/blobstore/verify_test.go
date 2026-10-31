package blobstore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestVerifyReadsActualBytes(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	info, err := store.Put(context.Background(), "object", strings.NewReader("verified bytes"), PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), store, info.Key, info.SHA256, info.Size); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), store, info.Key, strings.Repeat("0", 64), info.Size); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("wrong digest: %v", err)
	}
	if _, err := Verify(context.Background(), store, info.Key, info.SHA256, info.Size+1); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("wrong size: %v", err)
	}
	if _, err := Verify(context.Background(), store, "missing", info.SHA256, info.Size); !IsNotFound(err) {
		t.Fatalf("missing object: %v", err)
	}
}

func TestPresignHeadPutUsesBrowserEndpoint(t *testing.T) {
	st, err := NewS3(S3Config{Endpoint: "https://internal.example.com", PresignEndpoint: "https://browser.example.com", Region: "garage", Bucket: "media", AccessKeyID: "key", SecretAccessKey: "secret", ForcePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"HEAD", "PUT"} {
		var signed string
		if method == "HEAD" {
			signed, err = st.PresignHead(context.Background(), "object", time.Hour)
		} else {
			signed, err = st.PresignPut(context.Background(), "object", time.Hour, PutOptions{ContentType: "text/plain"})
		}
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(signed)
		if err != nil {
			t.Fatal(err)
		}
		if u.Host != "browser.example.com" || u.Query().Get("X-Amz-Expires") != "900" || u.Query().Get("X-Amz-Signature") == "" {
			t.Fatalf("%s signature: %s", method, signed)
		}
	}
}

func TestConditionalProbeUsesExplicitNamespace(t *testing.T) {
	var paths []string
	st, err := NewS3(S3Config{Endpoint: "https://storage.example.com", Region: "garage", Bucket: "test", AccessKeyID: "key", SecretAccessKey: "secret", ForcePathStyle: true, ConditionalProbePrefix: "prod/staging/", HTTPClient: &http.Client{Transport: s3TestTransport(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		status, body := 200, ""
		if req.Header.Get("If-None-Match") == "*" {
			status = 412
			body = "<Error><Code>PreconditionFailed</Code></Error>"
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	state, err := st.ProbeConditional(context.Background())
	if err != nil || state != "supported" || len(paths) != 3 {
		t.Fatalf("probe %s %v paths=%v", state, err, paths)
	}
	for _, path := range paths {
		if !strings.HasPrefix(path, "/test/prod/staging/") {
			t.Fatalf("wrong probe namespace: %s", path)
		}
	}
}
