package audio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
)

func TestBlobCachePeaksWarmAndVersionedInputs(t *testing.T) {
	data := silenceWAV(1500, 48000)
	svc, store, blob := cacheFixture(t, data)
	cache, err := configuredBlobCache()
	if err != nil {
		t.Fatal(err)
	}
	obj := svc.objectFor(blob, "chapter.wav")
	var decodes atomic.Int32
	decode := func(ctx context.Context, src, dest string) error {
		decodes.Add(1)
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		closeErr := out.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	for range 2 {
		peaks, duration, err := cache.peaks(context.Background(), obj, 500*time.Millisecond, decode)
		if err != nil {
			t.Fatal(err)
		}
		if len(peaks) != 3 || duration != 1.5 {
			t.Fatalf("peaks=%v duration=%v", peaks, duration)
		}
	}
	if gets, n := store.gets.Load(), decodes.Load(); gets != 1 || n != 1 {
		t.Fatalf("warm peaks: GETs=%d decodes=%d, want 1 total each", gets, n)
	}
	if _, _, err := cache.peaks(context.Background(), obj, 250*time.Millisecond, decode); err != nil {
		t.Fatal(err)
	}
	if n := decodes.Load(); n != 2 {
		t.Fatalf("bucket width change reused stale peaks: decodes=%d", n)
	}
	if n := store.gets.Load(); n != 1 {
		t.Fatalf("new bucket refetched verified source: GETs=%d", n)
	}
	next := silenceWAV(2000, 48000)
	sum := sha256.Sum256(next)
	obj.SHA256 = hex.EncodeToString(sum[:])
	obj.Key = testSegmentKey(t, obj.SHA256)
	obj.Size = int64(len(next))
	if _, err := store.Put(context.Background(), obj.Key, bytes.NewReader(next), blobstore.PutOptions{ExpectedSHA256: obj.SHA256}); err != nil {
		t.Fatal(err)
	}
	_, duration, err := cache.peaks(context.Background(), obj, 500*time.Millisecond, decode)
	if err != nil || duration != 2 || decodes.Load() != 3 || store.gets.Load() != 2 {
		t.Fatalf("source change: duration=%v decodes=%d GETs=%d err=%v", duration, decodes.Load(), store.gets.Load(), err)
	}
}

func TestBlobCacheStreamingPeaks(t *testing.T) {
	data := silenceWAV(1250, 48000)
	for i := 44; i+1 < len(data); i += 1024 {
		data[i] = 0
		data[i+1] = 128
	}
	path := filepath.Join(t.TempDir(), "pcm.wav")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	expected, err := WAVPeaks(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	got, duration, err := streamWAVPeaks(context.Background(), path, 500*time.Millisecond)
	if err != nil || !reflect.DeepEqual(got, expected) || duration != WAVDurationSec(data) {
		t.Fatalf("stream peaks=%v duration=%v err=%v; want %v", got, duration, err, expected)
	}
	for _, body := range [][]byte{[]byte("not WAV"), data[:len(data)-1]} {
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := streamWAVPeaks(context.Background(), path, 500*time.Millisecond); err == nil {
			t.Fatal("malformed/truncated PCM accepted")
		}
	}
}

func TestBlobCacheSingleFlight(t *testing.T) {
	svc, store, blob := cacheFixture(t, bytes.Repeat([]byte("audio"), 1024))
	entered := make(chan struct{}, 20)
	release := make(chan struct{})
	store.onOpen = func(ctx context.Context) error {
		entered <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	var workers sync.WaitGroup
	errs := make(chan error, 20)
	for range 20 {
		dest := filepath.Join(t.TempDir(), "source.wav")
		workers.Add(1)
		go func() { defer workers.Done(); errs <- svc.downloadBlob(context.Background(), blob, dest) }()
	}
	select {
	case <-entered:
	case <-time.After(30 * time.Second):
		t.Fatal("fetch did not start")
	}
	close(release)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := store.gets.Load(); n != 1 {
		t.Fatalf("single-flight GETs=%d, want 1", n)
	}
}

func TestBlobCacheFetchConcurrency(t *testing.T) {
	svc, store, _ := cacheFixture(t, []byte("unused"))
	entered := make(chan struct{}, 20)
	release := make(chan struct{})
	var active, maxActive atomic.Int32
	store.onOpen = func(ctx context.Context) error {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maxActive.Load(); n > old; old = maxActive.Load() {
			if maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		entered <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	var items []blobMaterialization
	for i := range 12 {
		data := []byte(fmt.Sprintf("source %d", i))
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		key := testSegmentKey(t, digest)
		if _, err := store.Put(context.Background(), key, bytes.NewReader(data), blobstore.PutOptions{ExpectedSHA256: digest}); err != nil {
			t.Fatal(err)
		}
		items = append(items, blobMaterialization{postgres.Blob{ObjectKey: key, SHA256: digest, SizeBytes: int64(len(data)), StorageDriver: "local", State: "ready"}, filepath.Join(t.TempDir(), "source.wav")})
	}
	done := make(chan error, 1)
	go func() { done <- svc.materializeBlobs(context.Background(), items) }()
	for range 4 {
		select {
		case <-entered:
		case <-time.After(30 * time.Second):
			close(release)
			t.Fatal("four concurrent fetches did not start")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := maxActive.Load(); n != 4 {
		t.Fatalf("fetch concurrency=%d, want exactly 4", n)
	}
}

func TestBlobCacheBudgetAndSpace(t *testing.T) {
	svc, store, blob := cacheFixture(t, []byte("12345678"))
	cache, err := newVerifiedBlobCache(t.TempDir(), 16)
	if err != nil {
		t.Fatal(err)
	}
	fetch := func(b postgres.Blob) error {
		return cache.materialize(context.Background(), svc.deps.Blobs, b.StorageDriver, b.ObjectKey, b.SHA256, b.SizeBytes, filepath.Join(t.TempDir(), "source.wav"))
	}
	if err := fetch(blob); err != nil {
		t.Fatal(err)
	}
	first, _ := cache.entry(blob.StorageDriver, blob.ObjectKey, blob.SHA256)
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(first, old, old); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{"abcdefgh", "ABCDEFGH"} {
		sum := sha256.Sum256([]byte(data))
		digest := hex.EncodeToString(sum[:])
		next := blob
		next.ObjectKey = testSegmentKey(t, digest)
		next.SHA256 = digest
		if _, err := store.Put(context.Background(), next.ObjectKey, bytes.NewBufferString(data), blobstore.PutOptions{ExpectedSHA256: digest}); err != nil {
			t.Fatal(err)
		}
		if err := fetch(next); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("least recently used cache entry not evicted: %v", err)
	}
	cache.freeBytes = func(string) (uint64, error) { return blobCacheHeadroom, nil }
	before := store.gets.Load()
	if err := fetch(blob); err == nil {
		t.Fatal("fetch accepted insufficient disk space")
	}
	if n := store.gets.Load(); n != before {
		t.Fatal("free-space failure fetched remote bytes")
	}
	oversized := blob
	oversized.SizeBytes = 17
	if err := fetch(oversized); err == nil {
		t.Fatal("oversized object exceeded budget")
	}
}

func TestBlobCacheStartupPreservesLiveWork(t *testing.T) {
	root := t.TempDir()
	cache, err := newVerifiedBlobCache(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	live, cleanup, err := cache.workDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	stale := filepath.Join(root, "work", "job-crashed")
	if err := os.MkdirAll(stale, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := newVerifiedBlobCache(root, 1<<20); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("startup deleted a live job: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("startup retained stale job: %v", err)
	}
}

func TestExportHeartbeatCrossesLeaseAndCancelsOnFenceLoss(t *testing.T) {
	var mu sync.Mutex
	now := time.Unix(0, 0)
	until := now.Add(postgres.DefaultLease)
	revoked := false
	ticks := make(chan time.Time)
	renewed := make(chan time.Time, 1)
	ctx, stop := maintainExportLease(context.Background(), postgres.DefaultHeartbeat, func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		if revoked || !now.Before(until) {
			return postgres.ErrJobLeaseLost
		}
		until = now.Add(postgres.DefaultLease)
		renewed <- now
		return nil
	}, ticks)
	defer stop()
	select {
	case <-renewed:
	case <-ctx.Done():
		t.Fatalf("initial heartbeat failed: %v", context.Cause(ctx))
	case <-time.After(30 * time.Second):
		t.Fatal("initial heartbeat did not run")
	}
	for _, seconds := range []int{30, 60, 90, 120, 150} {
		mu.Lock()
		now = time.Unix(int64(seconds), 0)
		mu.Unlock()
		ticks <- now
		select {
		case <-renewed:
		case <-ctx.Done():
			t.Fatalf("lease lost before renewal at %ds: %v", seconds, context.Cause(ctx))
		}
	}
	if ctx.Err() != nil {
		t.Fatalf("export failed beyond original 120s lease: %v", context.Cause(ctx))
	}
	mu.Lock()
	revoked = true
	mu.Unlock()
	ticks <- time.Unix(180, 0)
	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
		t.Fatal("fence loss did not cancel work")
	}
	if err := stop(); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("stop=%v, want lease loss", err)
	}
}

func TestBlobCacheRejectsWrongRemoteContent(t *testing.T) {
	svc, store, blob := cacheFixture(t, []byte("good"))
	if _, err := store.Put(context.Background(), blob.ObjectKey, bytes.NewBufferString("evil"), blobstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "source.wav")
	if err := svc.downloadBlob(context.Background(), blob, dest); err == nil {
		t.Fatal("unverified remote bytes materialized")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("failed fetch exposed destination: %v", err)
	}
}
