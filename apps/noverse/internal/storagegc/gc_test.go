package storagegc_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/datnt1998/noverse/internal/blobkey"
	"github.com/datnt1998/noverse/internal/blobstore"
	"github.com/datnt1998/noverse/internal/persistence/migrate"
	"github.com/datnt1998/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/noverse/internal/persistence/postgres"
	"github.com/datnt1998/noverse/internal/storagegc"
)

// The suite needs the disposable MinIO and Postgres from deploy/compose.test.yml
// (see deploy/README.md). Absence is a skip; REQUIRE_MINIO_TEST=1 /
// REQUIRE_POSTGRES_TEST=1 turn it into a hard failure so acceptance runs cannot
// pass on a skipped suite.

const (
	testProject = "11111111-1111-4111-8111-111111111111"
	testStory   = "22222222-2222-4222-8222-222222222222"
	testUpload  = "44444444-4444-4444-8444-444444444444"
)

type fixture struct {
	pool  *postgres.Pool
	store *blobstore.S3
	keys  blobkey.Keys
	// now is the injected clock: the future clock ages every object uploaded
	// by the test past the grace window (provider modification times cannot be
	// rewritten, so "old object" in a test means "GC clock moved forward").
	now time.Time
}

func newFixture(t *testing.T, env blobkey.Env) *fixture {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration db: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	keys, err := blobkey.New(env)
	if err != nil {
		t.Fatalf("keys: %v", err)
	}
	ctx := context.Background()
	if _, err := pool.Pool().Exec(ctx,
		`INSERT INTO projects (id, slug, name) VALUES ($1, 'gc-project', 'GC')`, testProject); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Pool().Exec(ctx,
		`INSERT INTO stories (project_id, id, slug, title, status) VALUES ($1,$2,'gc-story','GC','writing')`,
		testProject, testStory); err != nil {
		t.Fatalf("insert story: %v", err)
	}
	return &fixture{pool: pool, store: newMinioStore(t), keys: keys}
}

// newMinioStore builds the S3 driver against the disposable MinIO and creates a
// per-run bucket (removed with its objects on cleanup).
func newMinioStore(t *testing.T) *blobstore.S3 {
	t.Helper()
	endpoint := strings.TrimSpace(os.Getenv("MINIO_TEST_ENDPOINT"))
	if endpoint == "" {
		if os.Getenv("REQUIRE_MINIO_TEST") == "1" {
			t.Fatal("REQUIRE_MINIO_TEST=1 but MINIO_TEST_ENDPOINT is unset")
		}
		t.Skip("MINIO_TEST_ENDPOINT unset — skipping MinIO suite")
	}
	cfg := blobstore.S3Config{
		Endpoint:        endpoint,
		Region:          minioEnv("MINIO_TEST_REGION", "us-east-1"),
		Bucket:          fmt.Sprintf("%s-%d", minioEnv("MINIO_TEST_BUCKET", "blobstore-test"), time.Now().UnixNano()%1_000_000_000),
		AccessKeyID:     minioEnv("MINIO_TEST_ACCESS_KEY", "minioadmin"),
		SecretAccessKey: minioEnv("MINIO_TEST_SECRET_KEY", "minioadmin123"),
		ForcePathStyle:  true,
		AllowPlainHTTP:  true,
	}
	store, err := blobstore.NewS3(cfg)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deadline := time.Now().Add(20 * time.Second)
	for {
		_, err := store.Client().CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(cfg.Bucket)})
		if err == nil {
			break
		}
		var owned *s3types.BucketAlreadyOwnedByYou
		var exists *s3types.BucketAlreadyExists
		if errors.As(err, &owned) || errors.As(err, &exists) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("CreateBucket (minio ready?): %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		cursor := ""
		for {
			page, err := store.List(ctx, "", cursor, 1000)
			if err != nil {
				return
			}
			for _, info := range page.Infos {
				_ = store.Delete(ctx, info.Key)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		_, _ = store.Client().DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(cfg.Bucket)})
	})
	return store
}

func minioEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func hexOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (f *fixture) put(t *testing.T, key, body string) {
	t.Helper()
	if _, err := f.store.Put(context.Background(), key, strings.NewReader(body), blobstore.PutOptions{}); err != nil {
		t.Fatalf("put %s: %v", key, err)
	}
}

func (f *fixture) exists(key string) bool {
	_, err := f.store.Stat(context.Background(), key)
	return err == nil
}

func (f *fixture) insertBlob(t *testing.T, id, key, state string, createdAt time.Time) {
	t.Helper()
	_, err := f.pool.Pool().Exec(context.Background(),
		`INSERT INTO blobs (id, project_id, object_key, storage_driver, sha256, size_bytes,
		                    content_type, state, created_at)
		 VALUES ($1,$2,$3,'s3',$4,$5,'application/octet-stream',$6,$7)`,
		id, testProject, key, hexOf(id), int64(len(key)), state, createdAt)
	if err != nil {
		t.Fatalf("insert blob %s: %v", id, err)
	}
}

func (f *fixture) insertCache(t *testing.T, cacheKey, kind, blobID string, expiresAt time.Time) {
	t.Helper()
	_, err := f.pool.Pool().Exec(context.Background(),
		`INSERT INTO export_cache (project_id, story_id, cache_key, blob_id, kind, input_manifest, expires_at)
		 VALUES ($1,$2,$3,$4,$5,'{}'::jsonb,$6)`,
		testProject, testStory, cacheKey, blobID, kind, expiresAt)
	if err != nil {
		t.Fatalf("insert export_cache %s: %v", cacheKey, err)
	}
}

func (f *fixture) blobRows(t *testing.T) map[string]bool {
	t.Helper()
	rows, err := f.pool.Pool().Query(context.Background(), `SELECT id::text || ' ' || object_key FROM blobs`)
	if err != nil {
		t.Fatalf("select blobs: %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[s] = true
	}
	return out
}

func (f *fixture) run(t *testing.T, dry bool, opts ...func(*storagegc.Options)) storagegc.Report {
	t.Helper()
	o := storagegc.Options{
		Pool: f.pool, Blobs: f.store, Keys: f.keys,
		Now: func() time.Time { return f.now },
		// PageSize=1 exercises provider pagination on every listing.
		PageSize: 1,
		DryRun:   dry,
	}
	for _, fn := range opts {
		fn(&o)
	}
	rep, err := storagegc.Run(context.Background(), o)
	if err != nil {
		t.Fatalf("gc: %v\n%s", err, rep.Format())
	}
	return rep
}

func withGrace(d time.Duration) func(*storagegc.Options) {
	return func(o *storagegc.Options) { o.Grace = d }
}

func withStagingAge(d time.Duration) func(*storagegc.Options) {
	return func(o *storagegc.Options) { o.StagingAge = d }
}

// TestGCCollectsEveryRuleAndIsIdempotent drives all five collection rules on a
// real MinIO bucket and a real Postgres schema, then proves a second run is a
// no-op. The fixture's clock sits 48h ahead, so every object uploaded by the
// test is older than the 24h grace window without backdating provider metadata.
func TestGCCollectsEveryRuleAndIsIdempotent(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now().Add(48 * time.Hour)
	old := f.now.Add(-48 * time.Hour)
	fresh := f.now.Add(-time.Hour) // inside the 24h grace window
	expired := time.Now().Add(-time.Hour)
	live := time.Now().Add(24 * time.Hour)

	// A tombstone the application already declared dead.
	tombKey, err := f.keys.AudioSegment(testProject, testStory, hexOf("tomb"), "wav")
	if err != nil {
		t.Fatal(err)
	}
	tombID := "00000000-0000-4000-8000-000000000001"
	f.put(t, tombKey, "tombstone")
	f.insertBlob(t, tombID, tombKey, "deleting", fresh)

	// B unreferenced and old → collected. C unreferenced but fresh → kept.
	oldKey, _ := f.keys.AudioChapter(testProject, testStory, 1, 1, hexOf("old"), "wav")
	f.put(t, oldKey, "old-unreferenced")
	f.insertBlob(t, "00000000-0000-4000-8000-000000000002", oldKey, "ready", old)
	freshKey, _ := f.keys.Music(testProject, testStory, hexOf("fresh"), "mp3")
	f.put(t, freshKey, "fresh-unreferenced")
	f.insertBlob(t, "00000000-0000-4000-8000-000000000003", freshKey, "ready", fresh)

	// D referenced by a live export-cache entry → kept.
	liveKey, _ := f.keys.Cover(testProject, testStory, hexOf("live"), "png")
	f.put(t, liveKey, "live-referenced")
	liveID := "00000000-0000-4000-8000-000000000004"
	f.insertBlob(t, liveID, liveKey, "ready", old)
	f.insertCache(t, "cache-live", "cover", liveID, live)

	// E only referenced by an expired entry → entry and blob go.
	expKey, _ := f.keys.Export(testProject, testStory, hexOf("expired"), "het-han.txt")
	f.put(t, expKey, "expired-only")
	expID := "00000000-0000-4000-8000-000000000005"
	f.insertBlob(t, expID, expKey, "ready", old)
	f.insertCache(t, "cache-expired", "txt", expID, expired)

	// F shared by one expired and one live entry → only the expired entry goes.
	sharedKey, _ := f.keys.Export(testProject, testStory, hexOf("shared"), "chia-se.txt")
	f.put(t, sharedKey, "shared-cache")
	sharedID := "00000000-0000-4000-8000-000000000006"
	f.insertBlob(t, sharedID, sharedKey, "ready", old)
	f.insertCache(t, "cache-shared-expired", "epub", sharedID, expired)
	f.insertCache(t, "cache-shared-live", "txt", sharedID, live)

	// G fresh staged upload → kept inside the staged grace period.
	stagedKey, _ := f.keys.AudioSegment(testProject, testStory, hexOf("staged"), "mp3")
	f.put(t, stagedKey, "staged")
	f.insertBlob(t, "00000000-0000-4000-8000-000000000007", stagedKey, "staged", fresh)

	// Staging object, orphan object, foreign-env object and a backup object.
	stageKey, _ := f.keys.Staging(testUpload)
	f.put(t, stageKey, "abandoned-upload")
	orphanKey, _ := f.keys.Export(testProject, testStory, hexOf("orphan"), "mo-coi.txt")
	f.put(t, orphanKey, "orphan")
	prodKeys, _ := blobkey.New(blobkey.EnvProd)
	prodOrphan, _ := prodKeys.Export(testProject, testStory, hexOf("prod"), "prod.txt")
	f.put(t, prodOrphan, "cross-env")
	backupKey := f.keys.BackupPG(f.now)
	f.put(t, backupKey, "backup")

	rowsBefore := f.blobRows(t)
	if len(rowsBefore) != 7 {
		t.Fatalf("fixture rows = %d, want 7", len(rowsBefore))
	}

	// ── dry run: same candidates, zero writes ────────────────────────────────
	dry := f.run(t, true)
	if !dry.DryRun {
		t.Fatal("dry-run report must set DryRun")
	}
	if dry.DeletingReaped != 1 || dry.CacheDeleted != 2 || dry.CacheBlobsReaped != 0 ||
		dry.UnreferencedSeen != 1 || dry.BlobsReaped != 1 || dry.BlobsSkipped != 0 ||
		dry.StagingDeleted != 1 || dry.OrphansDeleted != 1 {
		t.Fatalf("dry-run candidates = %+v", dry)
	}
	if dry.ObjectsDeleted != 0 {
		t.Fatalf("dry-run deleted %d objects", dry.ObjectsDeleted)
	}
	if got := f.blobRows(t); len(got) != 7 {
		t.Fatalf("dry-run changed rows: %d", len(got))
	}
	for _, key := range []string{tombKey, oldKey, expKey, stageKey, orphanKey} {
		if !f.exists(key) {
			t.Fatalf("dry-run deleted object %s", key)
		}
	}

	// ── the real run ────────────────────────────────────────────────────────
	rep := f.run(t, false)
	if rep.DeletingFound != 1 || rep.DeletingReaped != 1 {
		t.Fatalf("tombstones = %+v", rep)
	}
	if rep.CacheExpired != 2 || rep.CacheDeleted != 2 || rep.CacheBlobsReaped != 1 {
		t.Fatalf("export cache = %+v", rep)
	}
	if rep.UnreferencedSeen != 1 || rep.BlobsReaped != 1 || rep.BlobsSkipped != 0 {
		t.Fatalf("unreferenced = %+v", rep)
	}
	if rep.StagingSeen != 1 || rep.StagingDeleted != 1 {
		t.Fatalf("staging = %+v", rep)
	}
	if rep.OrphansSeen != 1 || rep.OrphansDeleted != 1 {
		t.Fatalf("orphans = %+v", rep)
	}
	if rep.ObjectsDeleted != 5 {
		t.Fatalf("objects deleted = %d, want 5\n%s", rep.ObjectsDeleted, rep.Format())
	}
	if rep.OutOfScope != 0 || rep.UnknownObjects != 0 || len(rep.Errors) != 0 {
		t.Fatalf("unexpected noise: %+v", rep)
	}

	for _, key := range []string{tombKey, oldKey, expKey, stageKey, orphanKey} {
		if f.exists(key) {
			t.Fatalf("object %s should be gone", key)
		}
	}
	for _, key := range []string{freshKey, liveKey, sharedKey, stagedKey, prodOrphan, backupKey} {
		if !f.exists(key) {
			t.Fatalf("object %s must survive", key)
		}
	}
	if got := f.blobRows(t); len(got) != 4 {
		t.Fatalf("rows after run = %d, want 4: %v", len(got), got)
	}
	var cacheRows int
	if err := f.pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM export_cache`).Scan(&cacheRows); err != nil {
		t.Fatalf("count export_cache: %v", err)
	}
	if cacheRows != 2 {
		t.Fatalf("export_cache rows = %d, want 2 (one live + one shared live)", cacheRows)
	}

	// ── second run: nothing left to collect ─────────────────────────────────
	again := f.run(t, false)
	if again.ObjectsDeleted != 0 || again.DeletingReaped != 0 || again.CacheDeleted != 0 ||
		again.BlobsReaped != 0 || again.StagingDeleted != 0 || again.OrphansDeleted != 0 {
		t.Fatalf("second run was not a no-op: %+v", again)
	}
	if again.DeletingFound != 0 || again.CacheExpired != 0 || again.StagingSeen != 0 || again.OrphansSeen != 0 {
		t.Fatalf("second run still lists candidates: %+v", again)
	}
	if got := f.blobRows(t); len(got) != 4 {
		t.Fatalf("second run changed rows: %d", len(got))
	}
}

// TestGCGraceKeepsJustUploadedObjects runs with the real clock, so nothing the
// test just wrote is old enough to collect: an upload whose object landed
// before its registry row committed must never be reaped.
func TestGCGraceKeepsJustUploadedObjects(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now()

	orphanKey, _ := f.keys.Export(testProject, testStory, hexOf("young-orphan"), "tre.txt")
	f.put(t, orphanKey, "young-orphan")
	freshKey, _ := f.keys.Music(testProject, testStory, hexOf("young-blob"), "wav")
	f.put(t, freshKey, "young-blob")
	f.insertBlob(t, "00000000-0000-4000-8000-000000000011", freshKey, "ready", time.Now())
	stageKey, _ := f.keys.Staging(testUpload)
	f.put(t, stageKey, "young-staging")

	rep := f.run(t, false)
	if rep.ObjectsDeleted != 0 || rep.BlobsReaped != 0 || rep.StagingDeleted != 0 || rep.OrphansDeleted != 0 {
		t.Fatalf("grace not honored: %+v", rep)
	}
	if !f.exists(orphanKey) || !f.exists(freshKey) || !f.exists(stageKey) {
		t.Fatal("grace passed but an object was deleted")
	}
	if got := f.blobRows(t); len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
}

// TestGCAttachmentRaceKeepsBlob pins the transactional exclusion: an
// uncommitted attachment holds a KEY SHARE lock on the blob row, so GC must
// wait for it, then see the reference and keep the blob. Without the row lock
// GC would delete the row and the attachment's commit would fail its FK.
func TestGCAttachmentRaceKeepsBlob(t *testing.T) {
	f := newFixture(t, blobkey.EnvDev)
	f.now = time.Now().Add(48 * time.Hour)

	key, _ := f.keys.AudioChapter(testProject, testStory, 2, 3, hexOf("race"), "wav")
	f.put(t, key, "race")
	blobID := "00000000-0000-4000-8000-000000000021"
	f.insertBlob(t, blobID, key, "ready", f.now.Add(-48*time.Hour))

	ctx := context.Background()
	tx, err := f.pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO export_cache (project_id, story_id, cache_key, blob_id, kind, input_manifest, expires_at)
		 VALUES ($1,$2,'race-live',$3,'txt','{}'::jsonb, now() + interval '1 day')`,
		testProject, testStory, blobID); err != nil {
		tx.Rollback(ctx)
		t.Fatalf("attach: %v", err)
	}

	type outcome struct {
		rep storagegc.Report
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		rep, err := storagegc.Run(ctx, storagegc.Options{
			Pool: f.pool, Blobs: f.store, Keys: f.keys,
			Now: func() time.Time { return f.now },
			// Generous deadline: the run is expected to block on the row lock.
			PageSize: 1,
		})
		done <- outcome{rep: rep, err: err}
	}()

	// Give GC ample time to reach the decision on the locked row before
	// committing. With the row lock it blocks here; without it the run would
	// already have deleted the row (or died on the foreign key).
	time.Sleep(3 * time.Second)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit attachment: %v", err)
	}

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("gc: %v\n%s", got.err, got.rep.Format())
		}
		if got.rep.ObjectsDeleted != 0 || got.rep.BlobsReaped != 0 {
			t.Fatalf("gc deleted a concurrently attached blob: %+v", got.rep)
		}
		if len(got.rep.Errors) != 0 {
			t.Fatalf("gc hit errors instead of a clean skip: %v", got.rep.Errors)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("gc did not finish after the attachment committed (row lock leaked?)")
	}
	if got := f.blobRows(t); len(got) != 1 {
		t.Fatalf("rows = %d, want 1: %v", len(got), got)
	}
	if !f.exists(key) {
		t.Fatal("object deleted while an attachment was committing")
	}
}
