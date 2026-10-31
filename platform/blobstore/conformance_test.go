package blobstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// storeFactory builds a fresh, empty store per call. The conformance suite is
// driver-agnostic: the same assertions must hold on local and on MinIO.
type storeFactory func(t *testing.T) BlobStore

func TestConformanceLocal(t *testing.T) {
	runConformance(t, func(t *testing.T) BlobStore {
		st, err := NewLocal(t.TempDir() + "/blobs")
		if err != nil {
			t.Fatalf("NewLocal: %v", err)
		}
		return st
	})
}

// runConformance is the single suite every driver must pass.
func runConformance(t *testing.T, newStore storeFactory) {
	t.Run("put_stat_get", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		payload := []byte("xin chào blob — nội dung utf8 ✓")
		info, err := st.Put(ctx, "tenant-a/uuid-1", bytes.NewReader(payload), PutOptions{ContentType: "text/plain"})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		if info.Size != int64(len(payload)) {
			t.Fatalf("size = %d, want %d", info.Size, len(payload))
		}
		wantSHA, _, _ := DigestAll(bytes.NewReader(payload))
		if info.SHA256 != wantSHA {
			t.Fatalf("Put SHA256 = %q, want %q", info.SHA256, wantSHA)
		}
		got, err := st.Stat(ctx, "tenant-a/uuid-1")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if got.Size != info.Size {
			t.Fatalf("Stat size = %d, want %d", got.Size, info.Size)
		}
		rc, oi, err := st.Open(ctx, "tenant-a/uuid-1", FullRange)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer rc.Close()
		body, _ := io.ReadAll(rc)
		if !bytes.Equal(body, payload) {
			t.Fatalf("bytes mismatch")
		}
		if oi.Size != int64(len(payload)) {
			t.Fatalf("Open info size = %d", oi.Size)
		}
	})

	t.Run("range_read", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		payload := []byte("0123456789abcdef")
		if _, err := st.Put(ctx, "r/range", bytes.NewReader(payload), PutOptions{}); err != nil {
			t.Fatalf("Put: %v", err)
		}
		rc, _, err := st.Open(ctx, "r/range", ByteRange{Offset: 4, Length: 5})
		if err != nil {
			t.Fatalf("Open range: %v", err)
		}
		body, _ := io.ReadAll(rc)
		rc.Close()
		if string(body) != "45678" {
			t.Fatalf("range body = %q, want %q", body, "45678")
		}
		if _, _, err := st.Open(ctx, "r/range", ByteRange{Offset: 999}); !errors.Is(err, ErrInvalidRange) {
			t.Fatalf("out-of-range err = %v, want ErrInvalidRange", err)
		}
	})

	t.Run("unicode_keys", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		key := "p/项目/章节-01/âm thanh.wav"
		if _, err := st.Put(ctx, key, strings.NewReader("data"), PutOptions{}); err != nil {
			t.Fatalf("Put unicode: %v", err)
		}
		if _, err := st.Stat(ctx, key); err != nil {
			t.Fatalf("Stat unicode: %v", err)
		}
		rc, _, err := st.Open(ctx, key, FullRange)
		if err != nil {
			t.Fatalf("Open unicode: %v", err)
		}
		rc.Close()
	})

	t.Run("empty_object", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		info, err := st.Put(ctx, "e/empty", bytes.NewReader(nil), PutOptions{})
		if err != nil {
			t.Fatalf("Put empty: %v", err)
		}
		if info.Size != 0 {
			t.Fatalf("empty size = %d", info.Size)
		}
		rc, _, err := st.Open(ctx, "e/empty", FullRange)
		if err != nil {
			t.Fatalf("Open empty: %v", err)
		}
		body, _ := io.ReadAll(rc)
		rc.Close()
		if len(body) != 0 {
			t.Fatalf("empty body len = %d", len(body))
		}
	})

	t.Run("large_object_multipart", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		// >5 MiB forces multipart on S3 and exercises chunked write locally.
		payload := make([]byte, 9<<20)
		if _, err := rand.Read(payload); err != nil {
			t.Fatal(err)
		}
		info, err := st.Put(ctx, "big/obj", bytes.NewReader(payload), PutOptions{ContentType: "application/octet-stream"})
		if err != nil {
			t.Fatalf("Put 9MiB: %v", err)
		}
		wantSHA, _, _ := DigestAll(bytes.NewReader(payload))
		if info.SHA256 != wantSHA {
			t.Fatalf("big SHA256 = %q, want %q", info.SHA256, wantSHA)
		}
		rc, _, err := st.Open(ctx, "big/obj", FullRange)
		if err != nil {
			t.Fatalf("Open big: %v", err)
		}
		gotSHA, gotSize, err := DigestAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read big: %v", err)
		}
		if gotSHA != wantSHA || gotSize != int64(len(payload)) {
			t.Fatalf("big roundtrip sha=%q size=%d", gotSHA, gotSize)
		}
	})

	t.Run("list_pagination", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		var want []string
		for i := range 12 {
			key := fmt.Sprintf("pg/k%02d", i)
			if _, err := st.Put(ctx, key, strings.NewReader(fmt.Sprintf("v%d", i)), PutOptions{}); err != nil {
				t.Fatalf("Put %s: %v", key, err)
			}
			want = append(want, key)
		}
		var got []string
		cursor, pages := "", 0
		for {
			page, err := st.List(ctx, "pg/", cursor, 5)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			pages++
			for _, bi := range page.Infos {
				got = append(got, bi.Key)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
			if pages > 10 {
				t.Fatal("pagination did not terminate")
			}
		}
		if pages < 3 {
			t.Fatalf("pages = %d, want >=3 for 12 items @5", pages)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("listed keys = %v, want %v", got, want)
		}
	})

	t.Run("delete_idempotent", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		if _, err := st.Put(ctx, "d/gone", strings.NewReader("x"), PutOptions{}); err != nil {
			t.Fatalf("Put: %v", err)
		}
		if err := st.Delete(ctx, "d/gone"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if err := st.Delete(ctx, "d/gone"); err != nil {
			t.Fatalf("Delete absent must be idempotent: %v", err)
		}
		if _, err := st.Stat(ctx, "d/gone"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Stat deleted = %v, want ErrNotFound", err)
		}
	})

	t.Run("expected_sha256_mismatch", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		bad := strings.Repeat("0", 64)
		_, err := st.Put(ctx, "m/bad", strings.NewReader("payload"), PutOptions{ExpectedSHA256: bad})
		if !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("Put wrong hash err = %v, want ErrDigestMismatch", err)
		}
		if _, err := st.Stat(ctx, "m/bad"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("rejected object must not be visible, stat err = %v", err)
		}
	})

	t.Run("invalid_keys", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		for _, key := range []string{"", "..", "../escape", "/abs", "a/../b", "a//b", "a/./b", "back\\slash", "trail/"} {
			if _, err := st.Put(ctx, key, strings.NewReader("x"), PutOptions{}); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Put(%q) err = %v, want ErrInvalidKey", key, err)
			}
			if _, err := st.Stat(ctx, key); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Stat(%q) err = %v, want ErrInvalidKey", key, err)
			}
		}
	})

	t.Run("presign_capability", func(t *testing.T) {
		st := newStore(t)
		ctx := context.Background()
		if _, err := st.Put(ctx, "s/signed", strings.NewReader("x"), PutOptions{}); err != nil {
			t.Fatalf("Put: %v", err)
		}
		url, err := st.PresignGet(ctx, "s/signed", time.Minute, DownloadOptions{})
		if errors.Is(err, ErrPresignUnsupported) {
			return // acceptable capability answer
		}
		if err != nil {
			t.Fatalf("PresignGet: %v", err)
		}
		if !strings.HasPrefix(url, "http") {
			t.Fatalf("presigned URL = %q", url)
		}
	})
}
