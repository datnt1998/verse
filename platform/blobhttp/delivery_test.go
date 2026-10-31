package blobhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/blobstore"
)

type signedStore struct {
	blobstore.BlobStore
	signs   int
	signErr error
}

func (s *signedStore) DriverName() string { return "s3" }
func (s *signedStore) PresignGet(context.Context, string, time.Duration, blobstore.DownloadOptions) (string, error) {
	s.signs++
	return "https://browser.example.com/object?signature=original", s.signErr
}

func TestDeliveryProxyAndHead(t *testing.T) {
	store, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	info, err := store.Put(context.Background(), "object", strings.NewReader("0123456789"), blobstore.PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, rng, body string
		status            int
		contentRange      string
	}{
		{"GET", "", "0123456789", 200, ""}, {"GET", "bytes=2-4", "234", 206, "bytes 2-4/10"}, {"GET", "bytes=-3", "789", 206, "bytes 7-9/10"}, {"GET", "bytes=8-", "89", 206, "bytes 8-9/10"}, {"HEAD", "bytes=2-4", "", 206, "bytes 2-4/10"},
	} {
		t.Run(tc.method+tc.rng, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set("Range", tc.rng)
			w := httptest.NewRecorder()
			if err := Serve(w, req, store, info, Options{Filename: "tệp.txt", Disposition: "inline"}); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || w.Body.String() != tc.body || w.Header().Get("Content-Range") != tc.contentRange || w.Header().Get("Cache-Control") != "private, max-age=60" || w.Header().Get("ETag") != `"`+info.SHA256+`"` {
				t.Fatalf("response: %d %q %v", w.Code, w.Body.String(), w.Header())
			}
		})
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Range", "bytes=99-")
	w := httptest.NewRecorder()
	if err := Serve(w, req, store, info, Options{}); !errors.Is(err, blobstore.ErrInvalidRange) || w.Header().Get("Content-Range") != "bytes */10" || w.Body.Len() != 0 {
		t.Fatalf("unsatisfiable range: %v %v", err, w.Header())
	}
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-None-Match", `W/"`+info.SHA256+`"`)
	w = httptest.NewRecorder()
	if err := Serve(w, req, store, info, Options{}); err != nil || w.Code != 304 || w.Body.Len() != 0 {
		t.Fatalf("conditional response: %v %d", err, w.Code)
	}
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Range", "bytes=2-4")
	req.Header.Set("If-Range", `"different"`)
	w = httptest.NewRecorder()
	if err := Serve(w, req, store, info, Options{}); err != nil || w.Code != 200 || w.Body.String() != "0123456789" {
		t.Fatalf("If-Range response: %v %d", err, w.Code)
	}
}

func TestDeliveryRedirectAndFailures(t *testing.T) {
	local, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	info, err := local.Put(context.Background(), "object", strings.NewReader("payload"), blobstore.PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	store := &signedStore{BlobStore: local}
	for _, tc := range []struct {
		method, delivery string
		status, signs    int
	}{{"GET", "redirect", 307, 1}, {"HEAD", "redirect", 200, 0}, {"GET", "proxy", 200, 0}} {
		store.signs = 0
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, "/", nil)
		if err := Serve(w, req, store, info, Options{Delivery: tc.delivery}); err != nil {
			t.Fatal(err)
		}
		if w.Code != tc.status || store.signs != tc.signs {
			t.Fatalf("%s %s: status %d signs %d", tc.method, tc.delivery, w.Code, store.signs)
		}
		if tc.status == 307 && (w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("Location") != "https://browser.example.com/object?signature=original") {
			t.Fatalf("redirect headers: %v", w.Header())
		}
	}
	store.signErr = errors.New("signing unavailable")
	w := httptest.NewRecorder()
	if err := Serve(w, httptest.NewRequest("GET", "/", nil), store, info, Options{}); !errors.Is(err, store.signErr) || w.Body.Len() != 0 {
		t.Fatalf("sign error not returned before response: %v", err)
	}
	store.signErr = blobstore.ErrPresignUnsupported
	w = httptest.NewRecorder()
	if err := Serve(w, httptest.NewRequest("GET", "/", nil), store, info, Options{}); err != nil || w.Body.String() != "payload" {
		t.Fatalf("proxy fallback: %v %q", err, w.Body.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w = httptest.NewRecorder()
	if err := Serve(w, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx), local, info, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled proxy: %v", err)
	}
}
