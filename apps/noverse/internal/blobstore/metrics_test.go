package blobstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type metricsFake struct {
	BlobStore
	err  error
	body io.ReadCloser
}

func (f *metricsFake) DriverName() string { return "tenant-secret-driver" }
func (f *metricsFake) Put(_ context.Context, _ string, r io.Reader, _ PutOptions) (BlobInfo, error) {
	n, err := io.Copy(io.Discard, r)
	return BlobInfo{Size: n}, err
}
func (f *metricsFake) Open(context.Context, string, ByteRange) (io.ReadCloser, BlobInfo, error) {
	return f.body, BlobInfo{Size: 999}, f.err
}
func (f *metricsFake) Stat(context.Context, string) (BlobInfo, error) {
	// Inject an actually slow operation; no-op calls may legitimately measure
	// zero at the platform clock's resolution.
	time.Sleep(time.Millisecond)
	return BlobInfo{}, f.err
}
func (f *metricsFake) Delete(context.Context, string) error { return f.err }
func (f *metricsFake) List(context.Context, string, string, int) (ListPage, error) {
	return ListPage{}, f.err
}
func (f *metricsFake) PresignGet(context.Context, string, time.Duration, DownloadOptions) (string, error) {
	return "https://bucket.example/key?X-Amz-Signature=presign-secret", f.err
}
func (f *metricsFake) EffectivePresignTTL(time.Duration) time.Duration { return 42 * time.Second }

type metricsRequestError struct{}

func (metricsRequestError) Error() string {
	return "503 https://access-secret:password-secret@bucket.example/private-key?X-Amz-Signature=presign-secret Authorization: bearer-secret"
}
func (metricsRequestError) ServiceRequestID() string { return "sdk-request-123" }

type metricsCapabilities struct {
	*metricsFake
	calls int
}

func (f *metricsCapabilities) DeleteMany(_ context.Context, keys []string) ([]DeleteResult, error) {
	f.calls++
	return []DeleteResult{{Key: keys[0], Err: f.err}}, nil
}
func (f *metricsCapabilities) ListMultipartUploads(context.Context, string, string, int) (MultipartPage, error) {
	f.calls++
	return MultipartPage{NextCursor: "opaque-secret"}, nil
}
func (f *metricsCapabilities) AbortMultipartUpload(context.Context, MultipartUpload) error {
	f.calls++
	return nil
}

func TestMetricsStoreBoundedAndPrivate(t *testing.T) {
	var logs bytes.Buffer
	fake := &metricsFake{body: io.NopCloser(strings.NewReader("range"))}
	wrapped := NewMetrics(fake, slog.New(slog.NewJSONHandler(&logs, nil)), time.Nanosecond)
	snapshot := wrapped.(interface{ Snapshot() []OperationMetric })
	if _, ok := wrapped.(BatchDeleter); ok {
		t.Fatal("invented batch capability")
	}
	if _, ok := wrapped.(MultipartReaper); ok {
		t.Fatal("invented multipart capability")
	}
	if DriverName(wrapped) != fake.DriverName() {
		t.Fatal("persistence driver changed")
	}
	if wrapped.(interface {
		EffectivePresignTTL(time.Duration) time.Duration
	}).EffectivePresignTTL(time.Minute) != 42*time.Second {
		t.Fatal("lost TTL capability")
	}
	ctx := context.Background()
	_, _ = wrapped.Put(ctx, "private-key", strings.NewReader("payload"), PutOptions{})
	body, _, err := wrapped.Open(ctx, "private-key", FullRange)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(io.Discard, body); err != nil {
		t.Fatal(err)
	}
	_ = body.Close()
	_ = body.Close()
	fake.err = fmt.Errorf("wrapped: %w", metricsRequestError{})
	_, _ = wrapped.Stat(ctx, "private-key")
	fake.err = context.Canceled
	_ = wrapped.Delete(ctx, "private-key")
	fake.err = context.DeadlineExceeded
	_, _ = wrapped.List(ctx, "private-key", "cursor-secret", 1000)
	fake.err = nil
	url, _ := wrapped.PresignGet(ctx, "private-key", time.Minute, DownloadOptions{Filename: "filename-secret"})
	if !strings.Contains(url, "presign-secret") {
		t.Fatal("presign return altered")
	}
	fake.err = ErrNotFound
	_, _, _ = wrapped.Open(ctx, "private-key", FullRange)
	fake.err = ErrPresignUnsupported
	_, _ = wrapped.PresignGet(ctx, "private-key", 0, DownloadOptions{})
	got := snapshot.Snapshot()
	if len(got) != 8 {
		t.Fatalf("metric groups = %d, want 8: %+v", len(got), got)
	}
	allowed := map[string]string{"put/ok": "7", "open/ok": "5", "open/not_found": "0", "stat/error": "0", "delete/canceled": "0", "list/timeout": "0", "presign_get/ok": "0", "presign_get/unsupported": "0"}
	for _, m := range got {
		want, ok := allowed[m.Op+"/"+m.Outcome]
		if !ok || m.Driver != "unknown" || m.Count != 1 || fmt.Sprint(m.Bytes) != want || m.DurationSeconds < 0 || m.MaxDurationSeconds < 0 || m.MaxDurationSeconds > m.DurationSeconds {
			t.Errorf("unbounded or incorrect metric: %+v", m)
		}
		if m.Op == "stat" && (m.DurationSeconds <= 0 || m.MaxDurationSeconds <= 0) {
			t.Errorf("slow operation duration not recorded: %+v", m)
		}
	}
	encoded, _ := json.Marshal(got)
	for _, secret := range []string{"private-key", "tenant-secret-driver", "access-secret", "password-secret", "presign-secret", "bearer-secret", "cursor-secret", "filename-secret", "X-Amz", "Authorization"} {
		if strings.Contains(logs.String(), secret) || bytes.Contains(encoded, []byte(secret)) {
			t.Errorf("secret leaked: %s", secret)
		}
	}
	if !strings.Contains(logs.String(), `"request_id":"sdk-request-123"`) {
		t.Fatal("missing SDK request ID")
	}
	got[0].Count = 999
	if snapshot.Snapshot()[0].Count != 1 {
		t.Fatal("snapshot aliases internal counters")
	}

	caps := &metricsCapabilities{metricsFake: &metricsFake{err: errors.New("per-key secret")}}
	measured := NewMetrics(caps, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	results, err := measured.(BatchDeleter).DeleteMany(ctx, []string{"private-key"})
	if err != nil || len(results) != 1 || results[0].Err != caps.err {
		t.Fatal("lost per-key error")
	}
	page, err := measured.(MultipartReaper).ListMultipartUploads(ctx, "prod/", "", 10)
	if err != nil || page.NextCursor != "opaque-secret" {
		t.Fatal("lost multipart cursor")
	}
	_ = measured.(MultipartReaper).AbortMultipartUpload(ctx, MultipartUpload{})
	if caps.calls != 3 {
		t.Fatalf("capability calls=%d", caps.calls)
	}
	metrics := measured.(interface{ Snapshot() []OperationMetric }).Snapshot()
	if len(metrics) != 3 {
		t.Fatalf("capability metrics=%+v", metrics)
	}
	for _, m := range metrics {
		if m.Op == "delete_many" && m.Outcome != "error" {
			t.Fatalf("per-key failure not counted: %+v", m)
		}
	}
}
