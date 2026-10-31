package storagegc_test

import (
	"context"
	"errors"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/storagegc"
)

type deleteBarrier struct {
	blobstore.BlobStore
	reached chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func (b *deleteBarrier) Delete(ctx context.Context, key string) error {
	b.once.Do(func() { close(b.reached) })
	select {
	case <-b.resume:
	case <-ctx.Done():
		return ctx.Err()
	}
	return b.BlobStore.Delete(ctx, key)
}
func (b *deleteBarrier) DriverName() string { return blobstore.DriverName(b.BlobStore) }

func TestGCClaimSurvivesBeforeObjectDelete(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now().Add(48 * time.Hour)
	key, _ := f.keys.AudioSegment(testProject, testStory, hexOf("claim"), "wav")
	id := "00000000-0000-4000-8000-000000000081"
	f.put(t, key, "claim")
	f.insertBlob(t, id, key, "ready", time.Now().Add(-48*time.Hour))
	b := &deleteBarrier{BlobStore: f.store, reached: make(chan struct{}), resume: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := storagegc.Run(ctx, storagegc.Options{Pool: f.pool, Blobs: b, Keys: f.keys, Now: func() time.Time { return f.now }})
		done <- err
	}()
	select {
	case <-b.reached:
	case err := <-done:
		t.Fatalf("GC did not reach delete: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	defer close(b.resume)
	var state string
	if err := f.pool.Pool().QueryRow(ctx, `SELECT state FROM blobs WHERE id=$1`, id).Scan(&state); err != nil || state != "deleting" {
		t.Fatalf("deletion claim missing before object delete: state=%q err=%v", state, err)
	}
	project, _ := postgres.ParseUUID(testProject)
	if _, err := postgres.PublishBlob(ctx, f.pool.Pool(), postgres.PublishInput{ProjectID: project, Key: key, Driver: "s3", ContentType: "application/octet-stream", SHA256: hexOf(id), Size: int64(len(key))}); !errors.Is(err, postgres.ErrBlobDeleting) {
		t.Fatalf("publication during delete claim: %v", err)
	}
	_, err := f.pool.Pool().Exec(ctx, `INSERT INTO export_cache(project_id,story_id,cache_key,kind,blob_id,input_manifest) VALUES($1,$2,'claim','txt',$3,'{}')`, testProject, testStory, id)
	if err == nil {
		t.Fatal("attachment to deleting blob succeeded")
	}
	if !f.exists(key) {
		t.Fatal("object gone before delete barrier")
	}
	cancel()
	if err := <-done; err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("interrupted GC: %v", err)
	}
	// The next run resumes the durable claim after a crash before object delete.
	f.run(t, false)
	if len(f.blobRows(t)) != 0 || f.exists(key) {
		t.Fatal("claim was not recovered")
	}
}
