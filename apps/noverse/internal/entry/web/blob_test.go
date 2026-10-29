package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/blobstore"
)

// fakeStore is an in-memory BlobStore for seam tests: records calls, can
// toggle presign support and serve stale/expired URLs.
type fakeStore struct {
	objects       map[string][]byte
	presignErr    error
	presignURL    string
	presignCalled bool
}

func newFakeStore() *fakeStore { return &fakeStore{objects: map[string][]byte{}} }

func (f *fakeStore) Put(ctx context.Context, key string, r io.Reader, opts blobstore.PutOptions) (blobstore.BlobInfo, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	f.objects[key] = b
	sha, n, _ := blobstore.DigestAll(bytes.NewReader(b))
	return blobstore.BlobInfo{Key: key, Size: n, ContentType: opts.ContentType, SHA256: sha}, nil
}

func (f *fakeStore) Open(ctx context.Context, key string, rng blobstore.ByteRange) (io.ReadCloser, blobstore.BlobInfo, error) {
	b, ok := f.objects[key]
	if !ok {
		return nil, blobstore.BlobInfo{}, blobstore.ErrNotFound
	}
	start := rng.Offset
	end := int64(len(b))
	if rng.Length >= 0 && start+rng.Length < end {
		end = start + rng.Length
	}
	if start < 0 || start > int64(len(b)) {
		return nil, blobstore.BlobInfo{}, blobstore.ErrInvalidRange
	}
	return io.NopCloser(bytes.NewReader(b[start:end])),
		blobstore.BlobInfo{Key: key, Size: int64(len(b))}, nil
}

func (f *fakeStore) Stat(ctx context.Context, key string) (blobstore.BlobInfo, error) {
	b, ok := f.objects[key]
	if !ok {
		return blobstore.BlobInfo{}, blobstore.ErrNotFound
	}
	sha, n, _ := blobstore.DigestAll(bytes.NewReader(b))
	return blobstore.BlobInfo{Key: key, Size: n, ContentType: "application/octet-stream", SHA256: sha}, nil
}

func (f *fakeStore) Delete(ctx context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

func (f *fakeStore) List(ctx context.Context, prefix, cursor string, limit int) (blobstore.ListPage, error) {
	return blobstore.ListPage{}, nil
}

func (f *fakeStore) PresignGet(ctx context.Context, key string, ttl time.Duration, opts blobstore.DownloadOptions) (string, error) {
	f.presignCalled = true
	if f.presignErr != nil {
		return "", f.presignErr
	}
	return f.presignURL, nil
}

// deliveryFixture wires the seam exactly as the router will: wildcard key.
func deliveryFixture(t *testing.T, auth BlobAuthorizer, st blobstore.BlobStore) *httptest.Server {
	t.Helper()
	d := &BlobDelivery{Store: st, Authorize: auth, PresignTTL: 60 * time.Second}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/blobs/stream/{key...}", d.ServeStream)
	mux.HandleFunc("/api/blobs/sign/{key...}", d.ServeSign)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func allowAll(r *http.Request, key string) error { return nil }

// TestBlobDelivery is the HTTP-integration gate: auth denial, private
// headers, Range/suffix/HEAD/416, presign-unsupported fallback.
func TestBlobDelivery(t *testing.T) {
	payload := []byte("0123456789abcdef") // 16 bytes

	t.Run("auth_denial", func(t *testing.T) {
		st := newFakeStore()
		_, _ = st.Put(context.Background(), "t/a", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, func(r *http.Request, key string) error {
			return fmt.Errorf("deny %s: %w", key, ErrBlobDenied)
		}, st)
		resp, err := http.Get(srv.URL + "/api/blobs/stream/t/a")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("denied status = %d, want 403", resp.StatusCode)
		}
	})

	t.Run("nil_authorizer_denies", func(t *testing.T) {
		st := newFakeStore()
		_, _ = st.Put(context.Background(), "t/a", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, nil, st)
		resp, _ := http.Get(srv.URL + "/api/blobs/stream/t/a")
		resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("nil authorizer status = %d, want 500", resp.StatusCode)
		}
	})

	t.Run("full_get_and_headers", func(t *testing.T) {
		st := newFakeStore()
		put, _ := st.Put(context.Background(), "t/a.bin", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, allowAll, st)
		resp, err := http.Get(srv.URL + "/api/blobs/stream/t/a.bin")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !bytes.Equal(body, payload) {
			t.Fatalf("status=%d body=%q", resp.StatusCode, body)
		}
		if !strings.Contains(resp.Header.Get("Cache-Control"), "private") {
			t.Fatalf("Cache-Control = %q", resp.Header.Get("Cache-Control"))
		}
		if resp.Header.Get("Accept-Ranges") != "bytes" {
			t.Fatalf("Accept-Ranges = %q", resp.Header.Get("Accept-Ranges"))
		}
		if resp.Header.Get("Content-Length") != "16" {
			t.Fatalf("Content-Length = %q", resp.Header.Get("Content-Length"))
		}
		if resp.Header.Get("X-Content-SHA256") != put.SHA256 {
			t.Fatalf("digest header = %q", resp.Header.Get("X-Content-SHA256"))
		}
		if resp.Header.Get("Content-Security-Policy") == "" {
			t.Fatal("missing CSP on private content")
		}
	})

	t.Run("range_206", func(t *testing.T) {
		st := newFakeStore()
		_, _ = st.Put(context.Background(), "t/r", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, allowAll, st)
		req, _ := http.NewRequest("GET", srv.URL+"/api/blobs/stream/t/r", nil)
		req.Header.Set("Range", "bytes=4-9")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusPartialContent {
			t.Fatalf("status = %d, want 206", resp.StatusCode)
		}
		if string(body) != "456789" {
			t.Fatalf("range body = %q", body)
		}
		if resp.Header.Get("Content-Range") != "bytes 4-9/16" {
			t.Fatalf("Content-Range = %q", resp.Header.Get("Content-Range"))
		}
	})

	t.Run("suffix_range", func(t *testing.T) {
		st := newFakeStore()
		_, _ = st.Put(context.Background(), "t/r", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, allowAll, st)
		req, _ := http.NewRequest("GET", srv.URL+"/api/blobs/stream/t/r", nil)
		req.Header.Set("Range", "bytes=-4")
		resp, _ := http.DefaultClient.Do(req)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 206 || string(body) != "cdef" {
			t.Fatalf("suffix range: status=%d body=%q", resp.StatusCode, body)
		}
	})

	t.Run("range_416", func(t *testing.T) {
		st := newFakeStore()
		_, _ = st.Put(context.Background(), "t/r", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, allowAll, st)
		for _, h := range []string{"bytes=99-", "bytes=16-20", "bytes=0-2,5-9"} {
			req, _ := http.NewRequest("GET", srv.URL+"/api/blobs/stream/t/r", nil)
			req.Header.Set("Range", h)
			resp, _ := http.DefaultClient.Do(req)
			resp.Body.Close()
			if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
				t.Fatalf("Range %q → %d, want 416", h, resp.StatusCode)
			}
			if resp.Header.Get("Content-Range") != "bytes */16" {
				t.Fatalf("416 Content-Range = %q", resp.Header.Get("Content-Range"))
			}
		}
	})

	t.Run("head", func(t *testing.T) {
		st := newFakeStore()
		_, _ = st.Put(context.Background(), "t/h", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, allowAll, st)
		resp, err := http.Head(srv.URL + "/api/blobs/stream/t/h")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || resp.Header.Get("Content-Length") != "16" {
			t.Fatalf("HEAD status=%d len=%q", resp.StatusCode, resp.Header.Get("Content-Length"))
		}
		if n, _ := strconv.Atoi(resp.Header.Get("Content-Length")); n != 16 {
			t.Fatal("HEAD must report object length")
		}
	})

	t.Run("not_found", func(t *testing.T) {
		st := newFakeStore()
		srv := deliveryFixture(t, allowAll, st)
		resp, _ := http.Get(srv.URL + "/api/blobs/stream/t/missing")
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("missing blob status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("sign_presigned_mode", func(t *testing.T) {
		st := newFakeStore()
		st.presignURL = "https://signed.example.com/x?sig=1"
		_, _ = st.Put(context.Background(), "t/s", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, allowAll, st)
		resp, _ := http.Get(srv.URL + "/api/blobs/sign/t/s")
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var out struct {
			Mode string `json:"mode"`
			URL  string `json:"url"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("sign response: %v", err)
		}
		if out.Mode != "presigned" || out.URL != st.presignURL {
			t.Fatalf("sign response = %+v", out)
		}
	})

	t.Run("sign_unsupported_falls_back", func(t *testing.T) {
		st := newFakeStore()
		st.presignErr = blobstore.ErrPresignUnsupported
		_, _ = st.Put(context.Background(), "t/s", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, allowAll, st)
		resp, _ := http.Get(srv.URL + "/api/blobs/sign/t/s")
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var out struct {
			Mode string `json:"mode"`
			URL  string `json:"url"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		if out.Mode != "stream" || !strings.HasSuffix(out.URL, "/api/blobs/stream/t/s") {
			t.Fatalf("fallback response = %+v, want mode=stream url=.../stream", out)
		}
	})

	t.Run("sign_error_falls_back", func(t *testing.T) {
		st := newFakeStore()
		st.presignErr = errors.New("signing backend down")
		_, _ = st.Put(context.Background(), "t/s", bytes.NewReader(payload), blobstore.PutOptions{})
		srv := deliveryFixture(t, allowAll, st)
		resp, _ := http.Get(srv.URL + "/api/blobs/sign/t/s")
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var out struct{ Mode string }
		if json.Unmarshal(body, &out); out.Mode != "stream" {
			t.Fatalf("signing outage must degrade to stream, got %q", body)
		}
	})
}
