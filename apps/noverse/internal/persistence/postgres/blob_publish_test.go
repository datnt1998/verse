package postgres_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func publicationInput(f pgFixture, key string) postgres.PublishInput {
	sum := sha256.Sum256([]byte("verified bytes"))
	return postgres.PublishInput{ProjectID: f.scope.ProjectID, Key: key, Driver: "local", ContentType: "text/plain", SHA256: hex.EncodeToString(sum[:]), Size: 14}
}

func TestPublishBlobStateMachine(t *testing.T) {
	f := provisionNarrative(t, "publish-project", "publish-story")
	ctx := context.Background()
	seed := func(t *testing.T, state string) postgres.PublishInput {
		t.Helper()
		in := publicationInput(f, "test/publish/"+t.Name())
		id := deterministicUUID("blob", in.Key)
		_, err := f.pool.Pool().Exec(ctx, `INSERT INTO blobs(id,project_id,object_key,storage_driver,content_type,sha256,size_bytes,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, in.ProjectID, in.Key, in.Driver, in.ContentType, in.SHA256, in.Size, state)
		if err != nil {
			t.Fatal(err)
		}
		return in
	}
	t.Run("duplicate", func(t *testing.T) {
		in := seed(t, "ready")
		first, err := postgres.PublishBlob(ctx, f.pool.Pool(), in)
		if err != nil {
			t.Fatal(err)
		}
		second, err := postgres.PublishBlob(ctx, f.pool.Pool(), in)
		if err != nil || first.ID != second.ID || first.Version != second.Version {
			t.Fatalf("exact duplicate not idempotent: %+v %v", second, err)
		}
	})
	t.Run("staged", func(t *testing.T) {
		in := seed(t, "staged")
		resumed, err := postgres.PublishBlob(ctx, f.pool.Pool(), in)
		if err != nil || resumed.State != "ready" || resumed.Version != 2 {
			t.Fatalf("staged retry not resumed: %+v %v", resumed, err)
		}
	})
	for _, field := range []string{"sha", "size", "driver", "tenant", "type"} {
		t.Run(field, func(t *testing.T) {
			in := seed(t, "ready")
			switch field {
			case "sha":
				in.SHA256 = hex.EncodeToString(make([]byte, 32))
			case "size":
				in.Size++
			case "driver":
				in.Driver = "s3"
			case "tenant":
				in.ProjectID = deterministicUUID("project", "foreign")
			case "type":
				in.ContentType = "audio/mpeg"
			}
			if _, err := postgres.PublishBlob(ctx, f.pool.Pool(), in); !errors.Is(err, postgres.ErrBlobContentMismatch) {
				t.Fatalf("mismatch accepted: %v", err)
			}
		})
	}
	t.Run("deleting", func(t *testing.T) {
		in := seed(t, "deleting")
		if _, err := postgres.PublishBlob(ctx, f.pool.Pool(), in); !errors.Is(err, postgres.ErrBlobDeleting) {
			t.Fatalf("deleting publication = %v", err)
		}
	})
}

func TestPublishBlobConcurrentPublishers(t *testing.T) {
	f := provisionNarrative(t, "publish-concurrent", "publish-story")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	in := publicationInput(f, "test/concurrent")
	if _, err := postgres.PublishBlob(ctx, f.pool.Pool(), in); err != nil {
		t.Fatal(err)
	}
	tx, err := f.pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	first, err := postgres.PublishBlob(ctx, tx, in)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		second, err := postgres.PublishBlob(ctx, f.pool.Pool(), in)
		if err == nil && second.ID != first.ID {
			err = errors.New("concurrent publisher changed identity")
		}
		done <- err
	}()
	<-started
	select {
	case err := <-done:
		t.Fatalf("publisher did not serialize: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := postgres.PublishBlob(ctx, f.pool.Pool(), in)
			if err != nil || b.ID != first.ID {
				t.Errorf("duplicate race: %+v %v", b, err)
			}
		}()
	}
	wg.Wait()
}

type publicationBarrierStore struct {
	blobstore.BlobStore
	afterPut func()
	puts     int
}

func (s *publicationBarrierStore) Put(ctx context.Context, key string, r io.Reader, opts blobstore.PutOptions) (blobstore.BlobInfo, error) {
	info, err := s.BlobStore.Put(ctx, key, r, opts)
	s.puts++
	if err == nil && s.afterPut != nil {
		callback := s.afterPut
		s.afterPut = nil
		callback()
	}
	return info, err
}

func TestPublishBlobRepairsCompletedGCGap(t *testing.T) {
	f := provisionNarrative(t, "publish-gc-gap", "publish-story")
	ctx := context.Background()
	in := publicationInput(f, "test/gc-gap")
	local, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st := &publicationBarrierStore{BlobStore: local}
	st.afterPut = func() {
		// Deterministic cutpoint: upload has finished, publication has not begun.
		if _, err := postgres.PublishBlob(ctx, f.pool.Pool(), in); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Pool().Exec(ctx, `UPDATE blobs SET state='deleting', version=version+1, updated_at=clock_timestamp() WHERE object_key=$1`, in.Key); err != nil {
			t.Fatal(err)
		}
		if err := st.Delete(ctx, in.Key); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Pool().Exec(ctx, `DELETE FROM blobs WHERE object_key=$1 AND state='deleting'`, in.Key); err != nil {
			t.Fatal(err)
		}
	}
	ready, err := postgres.UploadAndPublishBlob(ctx, postgres.NewAudio(f.pool), st, in, bytes.NewReader([]byte("verified bytes")))
	if err != nil || ready.State != "ready" {
		t.Fatalf("publish after completed GC: %+v %v", ready, err)
	}
	body, _, err := st.Open(ctx, in.Key, blobstore.FullRange)
	if err != nil {
		t.Fatalf("ready row has missing bytes: %v", err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil || string(data) != "verified bytes" || st.puts != 2 {
		t.Fatalf("repair bytes=%q puts=%d err=%v", data, st.puts, err)
	}
}
