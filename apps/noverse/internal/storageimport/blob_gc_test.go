package storageimport

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/pgtest"
)

func gcImportPool(t *testing.T) *postgres.Pool {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type importPutBarrier struct {
	blobstore.BlobStore
	once            sync.Once
	reached, resume chan struct{}
}

func (s *importPutBarrier) Put(ctx context.Context, key string, r io.Reader, o blobstore.PutOptions) (blobstore.BlobInfo, error) {
	info, err := s.BlobStore.Put(ctx, key, r, o)
	if err != nil {
		return info, err
	}
	s.once.Do(func() {
		close(s.reached)
		select {
		case <-s.resume:
		case <-ctx.Done():
		}
	})
	return info, ctx.Err()
}

func TestImporterRepublishesAfterOrphanClaimCompletes(t *testing.T) {
	pool := gcImportPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	project := "11111111-1111-4111-8111-111111111111"
	if _, err := pool.Pool().Exec(ctx, `INSERT INTO projects(id,slug,name)VALUES($1,'import-gc','Import GC')`, project); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	body := "verified source bytes"
	if err := os.WriteFile(filepath.Join(dir, "source.bin"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	barrier := &importPutBarrier{BlobStore: store, reached: make(chan struct{}), resume: make(chan struct{})}
	keys, _ := blobkey.New(blobkey.EnvDev)
	key, err := BlobObjectKey(keys, project, "", RawDigest([]byte(body)), "source.bin")
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewBlobGC(pool)
	claim, ok, err := repo.ClaimOrphan(ctx, key, "local")
	if err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	im := &importer{root: root, opts: Options{Driver: "local", Blobs: barrier}}
	spec := &BlobSpec{Path: "source.bin", SHA256: RawDigest([]byte(body)), Size: int64(len(body)), ContentType: "application/octet-stream", ProjectID: project, ObjectKey: key}
	id := "22222222-2222-4222-8222-222222222222"
	type outcome struct {
		inserted bool
		conflict string
		err      error
	}
	done := make(chan outcome, 1)
	go func() {
		_, inserted, conflict, err := im.writeBlob(ctx, pool, spec, id)
		done <- outcome{inserted, conflict, err}
	}()
	select {
	case <-barrier.reached:
	case got := <-done:
		t.Fatalf("deleting tombstone treated as mismatch/skip: %+v", got)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Complete GC between the initial PUT and publication. Publication must
	// restore bytes while holding the new key before committing the ready row.
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishBlob(ctx, claim.ID); err != nil {
		t.Fatal(err)
	}
	close(barrier.resume)
	got := <-done
	if got.err != nil || got.conflict != "" || !got.inserted {
		t.Fatalf("import publication: %+v", got)
	}
	var state, actualID string
	if err := pool.Pool().QueryRow(ctx, `SELECT state,id::text FROM blobs WHERE object_key=$1`, key).Scan(&state, &actualID); err != nil || state != "ready" || actualID != id {
		t.Fatalf("published identity/state=%s %s err=%v", actualID, state, err)
	}
	rc, _, err := store.Open(ctx, key, blobstore.FullRange)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || string(data) != body {
		t.Fatalf("ready row has missing/corrupt bytes: %q %v", data, err)
	}
}

func TestReconcileIgnoresDeletingTombstones(t *testing.T) {
	pool := gcImportPool(t)
	keys, _ := blobkey.New(blobkey.EnvDev)
	key, _ := keys.AudioSegment("11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", strings.Repeat("a", 64), "wav")
	if _, ok, err := postgres.NewBlobGC(pool).ClaimOrphan(context.Background(), key, "local"); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	report := &Report{}
	if err := verifyBlobKeys(context.Background(), pool.Pool(), keys, report); err != nil || report.BlobKeysChecked != 0 || len(report.Mismatches) > 0 {
		t.Fatalf("transient tombstone considered tenant content: %+v err=%v", report, err)
	}
}
