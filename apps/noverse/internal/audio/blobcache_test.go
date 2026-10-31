package audio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
)

type countingAudioStore struct {
	blobstore.BlobStore
	gets   atomic.Int32
	onOpen func(context.Context) error
}

func (s *countingAudioStore) DriverName() string { return blobstore.DriverName(s.BlobStore) }
func (s *countingAudioStore) Open(ctx context.Context, key string, rng blobstore.ByteRange) (io.ReadCloser, blobstore.BlobInfo, error) {
	s.gets.Add(1)
	if s.onOpen != nil {
		if err := s.onOpen(ctx); err != nil {
			return nil, blobstore.BlobInfo{}, err
		}
	}
	return s.BlobStore.Open(ctx, key, rng)
}

const (
	testProjectUUID = "15a9d8f8-0c22-4a51-af3c-4618a3c2ecf1"
	testStoryUUID   = "4ef8a04b-b526-4c1d-9e58-78a90656a65c"
)

func testSegmentKey(t *testing.T, digest string) string {
	t.Helper()
	key, err := (blobkey.Keys{}).AudioSegment(testProjectUUID, testStoryUUID, digest, "wav")
	if err != nil {
		t.Fatal(err)
	}
	return key
}
func cacheFixture(t *testing.T, data []byte) (*PGService, *countingAudioStore, postgres.Blob) {
	t.Helper()
	t.Setenv("BLOB_CACHE_DIR", t.TempDir())
	st, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := &countingAudioStore{BlobStore: st}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	key := testSegmentKey(t, digest)
	if _, err := store.Put(context.Background(), key, bytes.NewReader(data), blobstore.PutOptions{ExpectedSHA256: digest}); err != nil {
		t.Fatal(err)
	}
	blob := postgres.Blob{ObjectKey: key, SHA256: digest, SizeBytes: int64(len(data)), StorageDriver: "local", State: "ready"}
	return NewPGService(PGDeps{Blobs: store}), store, blob
}
func TestBlobCacheExportFormatsShareSource(t *testing.T) {
	svc, store, blob := cacheFixture(t, bytes.Repeat([]byte("source audio"), 4096))
	for _, format := range []string{"mp3", "m4b"} {
		dir := t.TempDir()
		if err := svc.downloadBlob(context.Background(), blob, filepath.Join(dir, format, "source.wav")); err != nil {
			t.Fatal(err)
		}
	}
	if got := store.gets.Load(); got != 1 {
		t.Fatalf("two formats downloaded source %d times; warm export must issue 0 GETs", got)
	}
}
func TestBlobCacheCorruptionRefetched(t *testing.T) {
	data := bytes.Repeat([]byte("source audio"), 4096)
	svc, store, blob := cacheFixture(t, data)
	first := filepath.Join(t.TempDir(), "source.wav")
	if err := svc.downloadBlob(context.Background(), blob, first); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(first, 0600); err != nil {
		t.Fatal(err)
	}
	// Materialization is a hardlink when possible: corruption of either inode
	// must invalidate the cached content, not just its filename/metadata.
	if err := os.WriteFile(first, bytes.Repeat([]byte("x"), len(data)), 0600); err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(t.TempDir(), "source.wav")
	if err := svc.downloadBlob(context.Background(), blob, next); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(next)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("corrupt cache bytes escaped verification")
	}
	if n := store.gets.Load(); n != 2 {
		t.Fatalf("GETs=%d, want initial fetch + repair", n)
	}
}
