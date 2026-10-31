package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

type deliveryStore struct {
	*fakeStore
	driver       string
	opens, stats int
	openErr      error
	copyErr      bool
	ttl          time.Duration
	download     blobstore.DownloadOptions
}

func (s *deliveryStore) DriverName() string { return s.driver }
func (s *deliveryStore) EffectivePresignTTL(ttl time.Duration) time.Duration {
	if ttl > 10*time.Minute {
		return 10 * time.Minute
	}
	return ttl
}
func (s *deliveryStore) Stat(ctx context.Context, key string) (blobstore.BlobInfo, error) {
	s.stats++
	return s.fakeStore.Stat(ctx, key)
}
func (s *deliveryStore) Open(ctx context.Context, key string, rng blobstore.ByteRange) (io.ReadCloser, blobstore.BlobInfo, error) {
	s.opens++
	if s.openErr != nil {
		return nil, blobstore.BlobInfo{}, s.openErr
	}
	if s.copyErr {
		return io.NopCloser(errorBlobReader{}), blobstore.BlobInfo{}, nil
	}
	return s.fakeStore.Open(ctx, key, rng)
}
func (s *deliveryStore) PresignGet(ctx context.Context, key string, ttl time.Duration, opts blobstore.DownloadOptions) (string, error) {
	s.ttl = ttl
	s.download = opts
	return s.fakeStore.PresignGet(ctx, key, ttl, opts)
}

type errorBlobReader struct{}

func (errorBlobReader) Read([]byte) (int, error) { return 0, errors.New("late-copy-fixture") }
func deliveryInfo(t *testing.T, s *deliveryStore) blobstore.BlobInfo {
	t.Helper()
	i, err := s.Put(context.Background(), "test/media.mp3", strings.NewReader("0123456789"), blobstore.PutOptions{ContentType: "audio/mpeg"})
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func deliveryPaths(s *deliveryStore, info blobstore.BlobInfo) map[string]http.HandlerFunc {
	d := &BlobDelivery{Store: s, Authorize: allowAll, Metadata: func(*http.Request, string) (blobstore.BlobInfo, error) { return info, nil }}
	g := &Gateway{pgAudio: &PGAudioDeps{Blobs: s}}
	return map[string]http.HandlerFunc{
		"generic": d.ServeStream,
		"audio": func(w http.ResponseWriter, r *http.Request) {
			serveAudioObject(w, r, audio.Object{Store: s, Key: info.Key, Size: info.Size, SHA256: info.SHA256, ContentType: info.ContentType, Name: "chapter.mp3"})
		},
		"hosted": func(w http.ResponseWriter, r *http.Request) {
			g.serveHostedBlob(w, r, postgres.Blob{ObjectKey: info.Key, SizeBytes: info.Size, SHA256: info.SHA256, ContentType: info.ContentType}, "private, max-age=60", "")
		},
	}
}
func TestBlobDeliveryConditionals(t *testing.T) {
	s := &deliveryStore{fakeStore: newFakeStore(), driver: "local"}
	info := deliveryInfo(t, s)
	for name, handler := range deliveryPaths(s, info) {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/blobs/stream/"+info.Key, nil)
			r.SetPathValue("key", info.Key)
			r.Header.Set("If-None-Match", "W/"+shaETag(info.SHA256))
			w := httptest.NewRecorder()
			before := s.opens
			handler(w, r)
			if w.Code != 304 || s.opens != before || s.stats != 0 || w.Header().Get("ETag") != shaETag(info.SHA256) {
				t.Fatalf("conditional code=%d opens=%d stats=%d etag=%q", w.Code, s.opens, s.stats, w.Header().Get("ETag"))
			}
			for _, match := range []bool{true, false} {
				r = httptest.NewRequest("GET", "/api/blobs/stream/"+info.Key, nil)
				r.SetPathValue("key", info.Key)
				r.Header.Set("Range", "bytes=2-4")
				v := shaETag(info.SHA256)
				if !match {
					v = `"other"`
				}
				r.Header.Set("If-Range", v)
				w = httptest.NewRecorder()
				handler(w, r)
				want := 200
				body := "0123456789"
				if match {
					want = 206
					body = "234"
				}
				if w.Code != want || w.Body.String() != body {
					t.Fatalf("if-range match=%v code=%d body=%q", match, w.Code, w.Body.String())
				}
			}
			r = httptest.NewRequest("HEAD", "/api/blobs/stream/"+info.Key, nil)
			r.SetPathValue("key", info.Key)
			w = httptest.NewRecorder()
			before = s.opens
			handler(w, r)
			if w.Code != 200 || s.opens != before || w.Body.Len() != 0 {
				t.Fatalf("HEAD opens storage: code=%d opens=%d", w.Code, s.opens)
			}
		})
	}
}
func TestBlobDeliveryOpenBeforeHeadersAndLateError(t *testing.T) {
	s := &deliveryStore{fakeStore: newFakeStore(), driver: "local", openErr: blobstore.ErrNotFound}
	info := deliveryInfo(t, s)
	for name, handler := range deliveryPaths(s, info) {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/blobs/stream/"+info.Key, nil)
			r.SetPathValue("key", info.Key)
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code != 404 {
				t.Fatalf("failed Open reported success: %d", w.Code)
			}
		})
	}
	s.openErr = nil
	s.copyErr = true
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(old)
	for name, handler := range deliveryPaths(s, info) {
		t.Run(name, func(t *testing.T) {
			logs.Reset()
			r := httptest.NewRequest("GET", "/api/blobs/stream/"+info.Key, nil)
			r.SetPathValue("key", info.Key)
			w := httptest.NewRecorder()
			handler(w, r)
			if !strings.Contains(logs.String(), "late-copy-fixture") {
				t.Fatal("late copy failure was silent")
			}
		})
	}
}
func TestBlobDeliveryDirectS3(t *testing.T) {
	s := &deliveryStore{fakeStore: newFakeStore(), driver: "s3"}
	s.presignURL = "https://storage.invalid/private?signature=secret"
	info := deliveryInfo(t, s)
	for name, handler := range deliveryPaths(s, info) {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/blobs/stream/"+info.Key, nil)
			r.SetPathValue("key", info.Key)
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code != 307 || w.Header().Get("Location") != s.presignURL || w.Body.Len() != 0 || s.opens != 0 || s.stats != 0 {
				t.Fatalf("not direct: code=%d bytes=%d opens=%d stats=%d", w.Code, w.Body.Len(), s.opens, s.stats)
			}
			if s.download.Disposition != "inline" {
				t.Fatalf("playback disposition=%q", s.download.Disposition)
			}
		})
	}
	g := &Gateway{pgAudio: &PGAudioDeps{Blobs: s}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/exports/file", nil)
	g.serveHostedBlob(w, r, postgres.Blob{ObjectKey: info.Key, SizeBytes: info.Size, SHA256: info.SHA256}, "private, no-store", "Tên sách.mp3")
	if s.download.Disposition != "attachment" || s.download.Filename != "Tên sách.mp3" {
		t.Fatalf("download options=%+v", s.download)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/exports/file?inline=1", nil)
	g.serveHostedBlob(w, r, postgres.Blob{ObjectKey: info.Key, SizeBytes: info.Size, SHA256: info.SHA256}, "private, no-store", "Tên sách.mp3")
	if s.download.Disposition != "inline" || s.download.Filename != "Tên sách.mp3" {
		t.Fatalf("export playback options=%+v", s.download)
	}
	denied := &BlobDelivery{Store: s, Authorize: func(*http.Request, string) error { return ErrBlobDenied }}
	s.presignCalled = false
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/blobs/stream/"+info.Key, nil)
	r.SetPathValue("key", info.Key)
	denied.ServeStream(w, r)
	if w.Code != 403 || s.presignCalled {
		t.Fatal("unauthorized request signed")
	}
}
func TestBlobSignEffectiveExpiry(t *testing.T) {
	for _, ttl := range []time.Duration{0, time.Minute, time.Hour} {
		t.Run(ttl.String(), func(t *testing.T) {
			s := &deliveryStore{fakeStore: newFakeStore(), driver: "s3"}
			info := deliveryInfo(t, s)
			s.presignURL = "https://storage.invalid/x?X-Amz-Date=20261031T120000Z&X-Amz-Expires=123"
			d := &BlobDelivery{Store: s, Authorize: allowAll, PresignTTL: ttl, Metadata: func(*http.Request, string) (blobstore.BlobInfo, error) { return info, nil }}
			r := httptest.NewRequest("GET", "/api/blobs/sign/"+info.Key, nil)
			r.SetPathValue("key", info.Key)
			w := httptest.NewRecorder()
			d.ServeSign(w, r)
			var response struct {
				ExpiresAt string `json:"expiresAt"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			want := ttl
			if ttl == 0 {
				want = 5 * time.Minute
			}
			if want > 10*time.Minute {
				want = 10 * time.Minute
			}
			if s.ttl != want || response.ExpiresAt != "2026-10-31T12:02:03Z" || w.Header().Get("Cache-Control") != "private, no-store" || s.stats != 0 {
				t.Fatalf("ttl=%v expiry=%s cache=%q stats=%d", s.ttl, response.ExpiresAt, w.Header().Get("Cache-Control"), s.stats)
			}
		})
	}
}
