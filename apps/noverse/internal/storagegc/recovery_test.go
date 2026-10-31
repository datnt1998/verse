package storagegc_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/storagegc"
	"github.com/datnt1998/verse/platform/blobstore"
)

type lostDeleteAck struct{ blobstore.BlobStore }

func (s lostDeleteAck) Delete(ctx context.Context, key string) error {
	if err := s.BlobStore.Delete(ctx, key); err != nil {
		return err
	}
	return errors.New("lost delete acknowledgement")
}
func (s lostDeleteAck) DriverName() string { return blobstore.DriverName(s.BlobStore) }

func TestGCRecoversAfterObjectDeletedBeforeRowFinish(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now().Add(48 * time.Hour)
	key, _ := f.keys.AudioSegment(testProject, testStory, hexOf("lost-ack"), "wav")
	id := "00000000-0000-4000-8000-000000000082"
	f.put(t, key, "lost-ack")
	f.insertBlob(t, id, key, "ready", time.Now().Add(-48*time.Hour))
	rep, err := storagegc.Run(context.Background(), storagegc.Options{Pool: f.pool, Blobs: lostDeleteAck{f.store}, Keys: f.keys, Now: func() time.Time { return f.now }})
	if err == nil || len(rep.Errors) == 0 {
		t.Fatal("lost ack should leave an error")
	}
	var state string
	if err := f.pool.Pool().QueryRow(context.Background(), `SELECT state FROM blobs WHERE id=$1`, id).Scan(&state); err != nil || state != "deleting" {
		t.Fatalf("lost claim after deleting object: %q %v", state, err)
	}
	if f.exists(key) {
		t.Fatal("test did not delete object")
	}
	f.run(t, false)
	if len(f.blobRows(t)) != 0 {
		t.Fatal("next run did not finish claim")
	}
}

type orphanListBarrier struct {
	blobstore.BlobStore
	prefix          string
	reached, resume chan struct{}
	once            sync.Once
}

func (s *orphanListBarrier) DriverName() string { return blobstore.DriverName(s.BlobStore) }
func (s *orphanListBarrier) List(ctx context.Context, prefix, cursor string, limit int) (blobstore.ListPage, error) {
	page, err := s.BlobStore.List(ctx, prefix, cursor, limit)
	if err == nil && prefix == s.prefix {
		s.once.Do(func() {
			close(s.reached)
			select {
			case <-s.resume:
			case <-ctx.Done():
			}
		})
	}
	return page, err
}

func TestGCOrphanListingDoesNotDeleteConcurrentPublication(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now().Add(48 * time.Hour)
	key, _ := f.keys.AudioSegment(testProject, testStory, hexOf("published-orphan"), "wav")
	f.put(t, key, "published-orphan")
	barrier := &orphanListBarrier{BlobStore: f.store, prefix: f.keys.Prefix(), reached: make(chan struct{}), resume: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := storagegc.Run(ctx, storagegc.Options{Pool: f.pool, Blobs: barrier, Keys: f.keys, Now: func() time.Time { return f.now }})
		done <- err
	}()
	select {
	case <-barrier.reached:
	case err := <-done:
		t.Fatalf("GC did not list orphan: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	project, _ := postgres.ParseUUID(testProject)
	published, err := postgres.PublishBlob(ctx, f.pool.Pool(), postgres.PublishInput{ProjectID: project, Key: key, Driver: "s3", ContentType: "application/octet-stream", SHA256: hexOf("published-orphan"), Size: 16})
	close(barrier.resume)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Stat(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	claim, claimed, err := postgres.NewBlobGC(f.pool).ClaimOrphan(ctx, key, "s3")
	if err != nil || claimed {
		t.Fatalf("orphan claim overwrote publication: %+v %v %v", claim, claimed, err)
	}
	if !f.exists(key) || len(f.blobRows(t)) != 1 || !published.ID.Valid {
		t.Fatal("GC removed published bytes")
	}
}

func TestGCMultipartGraceDryRunAndEnvironmentIsolation(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	ctx := context.Background()
	key, _ := f.keys.AudioSegment(testProject, testStory, hexOf("multipart"), "wav")
	prod, _ := blobkey.New(blobkey.EnvProd)
	foreign, _ := prod.AudioSegment(testProject, testStory, hexOf("multipart"), "wav")
	for _, k := range []string{key, key, foreign} {
		out, err := f.store.Client().CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: aws.String(f.store.Bucket()), Key: aws.String(k)})
		if err != nil {
			t.Fatal(err)
		}
		u := blobstore.MultipartUpload{Key: k, UploadID: aws.ToString(out.UploadId)}
		if _, err := f.store.Client().UploadPart(ctx, &s3.UploadPartInput{
			Bucket: aws.String(f.store.Bucket()), Key: aws.String(k), UploadId: out.UploadId, PartNumber: aws.Int32(1), Body: strings.NewReader("abandoned-part"),
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.store.AbortMultipartUpload(context.Background(), u) })
	}
	f.now = time.Now()
	if rep := f.run(t, false); rep.MultipartAborted != 0 {
		t.Fatal("young multipart aborted")
	}
	f.now = time.Now().Add(48 * time.Hour)
	rep := f.run(t, true)
	if rep.MultipartSeen != 2 || rep.MultipartAborted != 2 || len(f.blobRows(t)) != 0 {
		t.Fatalf("multipart dry preview: %+v", rep)
	}
	page, err := f.store.ListMultipartUploads(ctx, "", "", 1000)
	if err != nil || len(page.Uploads) != 3 {
		t.Fatalf("dry-run mutated multipart uploads: %+v %v", page, err)
	}
	rep = f.run(t, false)
	if rep.MultipartAborted != 2 {
		t.Fatalf("old multipart not aborted: %+v", rep)
	}
	page, err = f.store.ListMultipartUploads(ctx, "", "", 1000)
	if err != nil || len(page.Uploads) != 1 || page.Uploads[0].Key != foreign {
		t.Fatalf("foreign multipart deleted: %+v %v", page, err)
	}
}

// The wrapper records actual GC batches and injects a real per-key failure.
type countingBatchStore struct {
	blobstore.BlobStore
	sizes  []int
	failed bool
}

func (s *countingBatchStore) DriverName() string { return blobstore.DriverName(s.BlobStore) }
func (s *countingBatchStore) DeleteMany(ctx context.Context, keys []string) ([]blobstore.DeleteResult, error) {
	s.sizes = append(s.sizes, len(keys))
	out := make([]blobstore.DeleteResult, len(keys))
	for i, key := range keys {
		if !s.failed {
			s.failed = true
			out[i] = blobstore.DeleteResult{Key: key, Err: errors.New("transient")}
			continue
		}
		out[i] = blobstore.DeleteResult{Key: key, Err: s.BlobStore.Delete(ctx, key)}
	}
	return out, nil
}

func TestGCBatches1001ClaimsAndDryRunIsReadOnly(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now().Add(48 * time.Hour)
	// Missing objects are valid idempotent deletes and avoid 1001 fixture PUTs.
	for i := range 1001 {
		key, _ := f.keys.AudioSegment(testProject, testStory, hexOf(fmt.Sprint(i)), "wav")
		f.insertBlob(t, fmt.Sprintf("00000000-0000-4000-8000-%012d", i+100), key, "ready", time.Now().Add(-48*time.Hour))
	}
	store := &countingBatchStore{BlobStore: f.store}
	options := storagegc.Options{Pool: f.pool, Blobs: store, Keys: f.keys, Now: func() time.Time { return f.now }, DryRun: true}
	before := int64(0)
	if err := f.pool.Pool().QueryRow(context.Background(), `SELECT sum(version) FROM blobs`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	rep, err := storagegc.Run(context.Background(), options)
	if err != nil || rep.BlobsReaped != 1001 || len(store.sizes) != 0 {
		t.Fatalf("dry run: %+v %v sizes=%v", rep, err, store.sizes)
	}
	var after int64
	if err := f.pool.Pool().QueryRow(context.Background(), `SELECT sum(version) FROM blobs`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after || len(f.blobRows(t)) != 1001 {
		t.Fatal("dry-run mutated registry")
	}
	options.DryRun = false
	rep, err = storagegc.Run(context.Background(), options)
	if err != nil || rep.ObjectsDeleted != 1001 || len(f.blobRows(t)) != 0 {
		t.Fatalf("batch GC: %+v %v", rep, err)
	}
	if fmt.Sprint(store.sizes) != "[1000 1 1]" {
		t.Fatalf("expected two batches plus one failed-key retry, got %v", store.sizes)
	}
}
