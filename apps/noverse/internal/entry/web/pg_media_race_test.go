package web

// Concurrency regressions for hosted name allocation (review finding 3):
// create-only claims must make concurrent uploads/exports distinct-or-409,
// never a silent replacement of the winner's row.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
)

// registry section while storage Put is slow, so a read-then-upsert
// implementation is guaranteed to overlap.
type sleepyPutStore struct {
	blobstore.BlobStore
	delay func()
}

func (s *sleepyPutStore) Put(ctx context.Context, key string, r io.Reader, opts blobstore.PutOptions) (blobstore.BlobInfo, error) {
	if s.delay != nil {
		s.delay()
	}
	return s.BlobStore.Put(ctx, key, r, opts)
}

func (s *sleepyPutStore) DriverName() string { return blobstore.DriverName(s.BlobStore) }

func TestPGConcurrentAssetUploadsGetDistinctNames(t *testing.T) {
	f := provisionHostedMedia(t)
	slowClaims(f)

	recs := [2]*httptest.ResponseRecorder{}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			recs[i] = f.uploadAssetCall(t, f.fx.owner, "cover", "same.png", tinyPNG(t))
		}(i)
	}
	close(start)
	wg.Wait()

	paths := map[string]bool{}
	for i, rec := range recs {
		if rec.Code != http.StatusCreated {
			t.Fatalf("upload %d: %d %s", i, rec.Code, rec.Body.String())
		}
		paths[decodeAny(t, rec)["path"].(string)] = true
	}
	if len(paths) != 2 {
		t.Fatalf("concurrent uploads must claim distinct paths, got %v", paths)
	}
	docs, err := f.fx.g.pgAudio.Audio.ListAssetDocs(context.Background(), f.scope.ProjectID)
	if err != nil || len(docs) != 2 {
		t.Fatalf("two asset docs expected, got %d %v", len(docs), err)
	}
}

func slowClaims(f *pgMediaFixture) {
	f.fx.g.pgAudio.Blobs = &sleepyPutStore{BlobStore: f.fx.g.pgAudio.Blobs, delay: func() { time.Sleep(80 * time.Millisecond) }}
}

func TestPGConcurrentFailExportsRaceOneWinner(t *testing.T) {
	f := provisionHostedMedia(t)
	slowClaims(f)
	recs := [2]*httptest.ResponseRecorder{}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			recs[i] = f.storyCall(t, f.fx.owner, "POST", "/exports", "", `{"format":"txt","volume":1,"onExists":"fail"}`)
		}(i)
	}
	close(start)
	wg.Wait()

	wins, conflicts := 0, 0
	for _, rec := range recs {
		switch rec.Code {
		case http.StatusOK:
			wins++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected status %d %s", rec.Code, rec.Body.String())
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("fail exports must resolve 1 winner + 1 conflict, got %d wins %d conflicts", wins, conflicts)
	}
}

func TestPGConcurrentKeepExportsKeepBoth(t *testing.T) {
	f := provisionHostedMedia(t)
	slowClaims(f)
	recs := [2]*httptest.ResponseRecorder{}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			recs[i] = f.storyCall(t, f.fx.owner, "POST", "/exports", "", `{"format":"txt","volume":1,"onExists":"keep"}`)
		}(i)
	}
	close(start)
	wg.Wait()

	keys := map[string]bool{}
	for i, rec := range recs {
		if rec.Code != http.StatusOK {
			t.Fatalf("keep export %d: %d %s", i, rec.Code, rec.Body.String())
		}
		keys[decodeAny(t, rec)["path"].(string)] = true
	}
	if len(keys) != 2 {
		t.Fatalf("keep exports must produce two distinct artifacts, got %v", keys)
	}
	rec := f.storyCall(t, f.fx.owner, "GET", "/exports", "", "")
	if l := decodeAny(t, rec)["files"].([]any); len(l) != 2 {
		t.Fatalf("two concurrent keeps must leave two registry rows, got %d", len(l))
	}
}
