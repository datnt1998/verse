package bootstrap

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/blobstore/s3test"
	"github.com/datnt1998/verse/platform/pgtest"
)

const purgeProject = "11111111-1111-4111-8111-111111111111"

type purgeFixture struct {
	m                         *ManagedBlobs
	pool                      *postgres.Pool
	source, target, migration string
	src, dst                  blobstore.BlobStore
	keys                      []string
	bytes                     int64
	sourceConfig              BlobConfig
}

func newPurgeFixture(t *testing.T, count int) *purgeFixture {
	t.Helper()
	ctx := context.Background()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, err = migrate.Up(ctx, db)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.Open(ctx, postgres.PoolConfig{DSN: dsn, MaxConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	t.Setenv("CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("p", 32))))
	crypt, err := encryptionFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := blobkey.New(blobkey.EnvDev)
	m := &ManagedBlobs{pool: pool, cipher: crypt, keys: keys, stores: map[string]blobstore.BlobStore{}, configs: map[string]BlobConfig{}}
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		if os.Getenv("REQUIRE_S3_TEST") == "1" {
			t.Fatal("S3_TEST_ENDPOINT is required")
		}
		t.Skip("S3_TEST_ENDPOINT unset")
	}
	s3cfg := blobstore.S3Config{Endpoint: endpoint, Region: os.Getenv("S3_TEST_REGION"), Bucket: os.Getenv("S3_TEST_BUCKET"), AccessKeyID: os.Getenv("S3_TEST_ACCESS_KEY"), SecretAccessKey: os.Getenv("S3_TEST_SECRET_KEY"), ForcePathStyle: true, AllowPlainHTTP: true}
	if s3cfg.Region == "" {
		s3cfg.Region = "garage"
	}
	base, err := blobstore.NewS3(s3cfg)
	if err != nil {
		t.Fatal(err)
	}
	s3cfg.Bucket = s3test.Bucket(t, base.Client(), s3cfg.AccessKeyID, s3cfg.Bucket)
	dst, err := blobstore.NewS3(s3cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for {
			page, err := dst.List(cleanup, "", "", 1000)
			if err != nil {
				t.Error(err)
				return
			}
			for _, info := range page.Infos {
				if err := dst.Delete(cleanup, info.Key); err != nil {
					t.Error(err)
					return
				}
			}
			if page.NextCursor == "" {
				return
			}
		}
	})
	cfg := normalizeBlobConfig(BlobConfig{Driver: BlobDriverLocal, LocalRoot: t.TempDir()})
	src, err := OpenBlobStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.(*blobstore.Local).Close() })
	tx, err := pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	source, err := m.saveConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	target, err := m.saveConfig(ctx, tx, normalizeBlobConfig(BlobConfig{Driver: BlobDriverS3, Provider: "garage", S3: s3cfg}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_settings SET active_store=$1,pending_store=$2 WHERE singleton`, source, target); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	m.stores[source], m.configs[source] = src, cfg
	m.stores[target], m.configs[target] = dst, normalizeBlobConfig(BlobConfig{Driver: BlobDriverS3, Provider: "garage", S3: s3cfg})
	f := &purgeFixture{m: m, pool: pool, source: source, target: target, src: src, dst: dst, sourceConfig: cfg}
	f.exec(t, `INSERT INTO projects(id,slug,name) VALUES($1,'purge-project','Purge')`, purgeProject)
	for index := range count {
		key := fmt.Sprintf("dev/p/%s/import/%03d.dat", purgeProject, index)
		body := fmt.Sprintf("verified payload %03d", index)
		info, err := src.Put(ctx, key, strings.NewReader(body), blobstore.PutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		f.exec(t, `INSERT INTO blobs(id,project_id,object_key,storage_driver,sha256,size_bytes,content_type,state) VALUES($1,$2,$3,'local',$4,$5,'application/octet-stream','ready')`, fmt.Sprintf("22222222-2222-4222-8222-%012d", index+1), purgeProject, key, info.SHA256, info.Size)
		f.keys = append(f.keys, key)
		f.bytes += info.Size
	}
	if err := m.MigrationAction(ctx, "start", 1); err != nil {
		t.Fatal(err)
	}
	if err := pool.Pool().QueryRow(ctx, `SELECT id FROM storage_migrations`).Scan(&f.migration); err != nil {
		t.Fatal(err)
	}
	for range count/32 + 3 {
		if err := m.migrationPass(ctx); err != nil {
			t.Fatal(err)
		}
		var state string
		if err := pool.Pool().QueryRow(ctx, `SELECT state FROM storage_migrations WHERE id=$1`, f.migration).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state == "completed" {
			return f
		}
	}
	t.Fatal("migration did not complete")
	return nil
}

func (f *purgeFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.pool.Pool().Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f *purgeFixture) progress(t *testing.T) (int64, bool) {
	t.Helper()
	var count int64
	var purged bool
	if err := f.pool.Pool().QueryRow(context.Background(), `SELECT (SELECT count(*) FROM storage_migration_items WHERE migration_id=$1 AND source_purged_at IS NOT NULL),purged_at IS NOT NULL FROM storage_migrations WHERE id=$1`, f.migration).Scan(&count, &purged); err != nil {
		t.Fatal(err)
	}
	return count, purged
}

func requirePurgeRefusal(t *testing.T, report SourcePurgeReport, err error, code string) {
	t.Helper()
	if !errors.Is(err, ErrSourcePurgeRefused) {
		t.Fatalf("expected safety refusal %s; got report=%+v err=%v", code, report, err)
	}
	for _, refusal := range report.Refusals {
		if refusal.Code == code {
			return
		}
	}
	t.Fatalf("missing refusal %s: %+v", code, report)
}

func TestSourcePurgeLocalToGarage(t *testing.T) {
	f := newPurgeFixture(t, 3)
	targetConfig := f.m.configs[f.target].S3
	for name, value := range map[string]string{
		"STORAGE_DRIVER": "s3", "S3_PROVIDER": "garage",
		"S3_ENDPOINT": targetConfig.Endpoint, "S3_REGION": targetConfig.Region,
		"S3_BUCKET": targetConfig.Bucket, "S3_ACCESS_KEY_ID": targetConfig.AccessKeyID,
		"S3_SECRET_ACCESS_KEY":       targetConfig.SecretAccessKey,
		"S3_ALLOW_INSECURE_ENDPOINT": "true", "S3_FORCE_PATH_STYLE": "true",
	} {
		t.Setenv(name, value)
	}
	ctx := context.Background()
	const unrelated = "dev/backups/pg/unrelated.sql"
	if _, err := f.src.Put(ctx, unrelated, strings.NewReader("keep backup"), blobstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	report, err := PurgeMigrationSource(ctx, f.pool, f.m.keys, f.migration, true)
	if err != nil || report.Count != 3 || report.Bytes != f.bytes || report.Deleted != 0 || len(report.Refusals) != 0 {
		t.Fatalf("dry-run totals incorrect: %+v %v", report, err)
	}
	if n, purged := f.progress(t); n != 0 || purged {
		t.Fatalf("dry-run wrote progress: %d %v", n, purged)
	}
	for _, key := range f.keys {
		if _, err := f.src.Stat(ctx, key); err != nil {
			t.Fatalf("dry-run deleted %s: %v", key, err)
		}
	}
	report, err = PurgeMigrationSource(ctx, f.pool, f.m.keys, f.migration, false)
	if err != nil || report.Deleted != 3 || report.DeletedBytes != f.bytes {
		t.Fatalf("real purge totals incorrect: %+v %v", report, err)
	}
	for _, key := range f.keys {
		if _, err := f.src.Stat(ctx, key); !errors.Is(err, blobstore.ErrNotFound) {
			t.Fatalf("source still contains %s: %v", key, err)
		}
		reader, _, err := f.m.Open(ctx, key, blobstore.FullRange)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || !strings.HasPrefix(string(body), "verified payload ") {
			t.Fatalf("Garage read failed: %s %v", body, err)
		}
	}
	if _, err := f.src.Stat(ctx, unrelated); err != nil {
		t.Fatal("unrelated backup was deleted", err)
	}
	if n, purged := f.progress(t); n != 3 || !purged {
		t.Fatalf("missing durable purge state: %d %v", n, purged)
	}
	report, err = PurgeMigrationSource(ctx, f.pool, f.m.keys, f.migration, false)
	if err != nil || report.Deleted != 0 || report.Count != 0 {
		t.Fatalf("purge not idempotent: %+v %v", report, err)
	}
}

func TestSourcePurgeRefusesPhysicalReactivation(t *testing.T) {
	f := newPurgeFixture(t, 1)
	tx, err := f.pool.Pool().Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	alias, err := f.m.saveConfig(context.Background(), tx, f.sourceConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if alias == f.source {
		t.Fatal("fixture must use a different store ID")
	}
	for _, mode := range []string{"active", "pending"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "active" {
				f.exec(t, `UPDATE storage_settings SET active_store=$1,pending_store=NULL WHERE singleton`, alias)
			} else {
				f.exec(t, `UPDATE storage_settings SET active_store=$1,pending_store=$2 WHERE singleton`, f.target, alias)
			}
			report, err := f.m.PurgeMigrationSource(context.Background(), f.migration, false)
			requirePurgeRefusal(t, report, err, "source_"+mode)
			if report.Deleted != 0 {
				t.Fatal("reactivated source was deleted")
			}
			if _, err := f.src.Stat(context.Background(), f.keys[0]); err != nil {
				t.Fatal("reactivated source key missing", err)
			}
		})
	}
}

func TestSourcePurgeMissingTargetAndAllRefusals(t *testing.T) {
	f := newPurgeFixture(t, 4)
	ctx := context.Background()
	if err := f.dst.Delete(ctx, f.keys[0]); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `UPDATE blobs SET store_id=$1,version=version+1,updated_at=clock_timestamp() WHERE object_key=$2`, f.source, f.keys[1])
	f.exec(t, `UPDATE storage_migration_items SET verified=false WHERE migration_id=$1 AND object_key=$2`, f.migration, f.keys[2])
	for _, dry := range []bool{true, false} {
		report, err := f.m.PurgeMigrationSource(ctx, f.migration, dry)
		requirePurgeRefusal(t, report, err, "target_missing")
		for _, code := range []string{"registry_not_target", "manifest_unverified"} {
			requirePurgeRefusal(t, report, err, code)
		}
		expectedDeleted := int64(1)
		if dry {
			expectedDeleted = 0
		}
		if len(report.Refusals) != 3 || report.Deleted != expectedDeleted || report.Count != 1 {
			t.Fatalf("not all unsafe keys reported: %+v", report)
		}
		for _, key := range f.keys[:3] {
			if _, err := f.src.Stat(ctx, key); err != nil {
				t.Fatal("unsafe source key deleted", err)
			}
		}
		if n, purged := f.progress(t); n != expectedDeleted || purged {
			t.Fatalf("unsafe progress advanced: %d %v", n, purged)
		}
	}
}

type purgeBatchBarrier struct {
	blobstore.BlobStore
	deleteMany func(context.Context, []string) ([]blobstore.DeleteResult, error)
}

func (b *purgeBatchBarrier) DeleteMany(ctx context.Context, keys []string) ([]blobstore.DeleteResult, error) {
	return b.deleteMany(ctx, keys)
}

func TestSourcePurgeReactivationBetweenBatches(t *testing.T) {
	f := newPurgeFixture(t, 17)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	activationDone := make(chan error, 1)
	first := true
	f.m.stores[f.source] = &purgeBatchBarrier{BlobStore: f.src, deleteMany: func(ctx context.Context, keys []string) ([]blobstore.DeleteResult, error) {
		results, err := f.src.(blobstore.BatchDeleter).DeleteMany(ctx, keys)
		if !first || err != nil {
			return results, err
		}
		first = false
		pidReady := make(chan uint32, 1)
		go func() {
			conn, err := f.pool.Pool().Acquire(ctx)
			if err != nil {
				activationDone <- err
				return
			}
			defer conn.Release()
			tx, err := conn.Begin(ctx)
			if err != nil {
				activationDone <- err
				return
			}
			defer tx.Rollback(ctx)
			pidReady <- conn.Conn().PgConn().PID()
			if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71427014)`); err == nil {
				_, err = tx.Exec(ctx, `UPDATE storage_settings SET active_store=$1,version=version+1 WHERE singleton`, f.source)
			}
			if err == nil {
				err = tx.Commit(ctx)
			}
			activationDone <- err
		}()
		var pid uint32
		select {
		case pid = <-pidReady:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		// The activation is queued on the exact fence before this delete returns.
		// PostgreSQL grants it before the purger can lock the next batch.
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			var waiting bool
			if err := f.pool.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted)`, pid).Scan(&waiting); err != nil {
				return nil, err
			}
			if waiting {
				break
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return results, nil
	}}
	report, err := f.m.PurgeMigrationSource(ctx, f.migration, false)
	requirePurgeRefusal(t, report, err, "source_active")
	if err := <-activationDone; err != nil {
		t.Fatal(err)
	}
	if report.Deleted != 16 {
		t.Fatalf("deleted after reactivation: %+v", report)
	}
	if _, err := f.src.Stat(ctx, f.keys[16]); err != nil {
		t.Fatal("next batch deleted after reactivation", err)
	}
	if n, purged := f.progress(t); n != 16 || purged {
		t.Fatalf("unexpected reactivation progress: %d %v", n, purged)
	}
}

func TestSourcePurgeCrashResume(t *testing.T) {
	f := newPurgeFixture(t, 17)
	ctx := context.Background()
	calls := 0
	crash := errors.New("simulated process crash after source deletion")
	f.m.stores[f.source] = &purgeBatchBarrier{BlobStore: f.src, deleteMany: func(ctx context.Context, keys []string) ([]blobstore.DeleteResult, error) {
		calls++
		results, err := f.src.(blobstore.BatchDeleter).DeleteMany(ctx, keys)
		if calls == 2 {
			return nil, crash
		}
		return results, err
	}}
	report, err := f.m.PurgeMigrationSource(ctx, f.migration, false)
	if !errors.Is(err, crash) || report.Deleted != 16 {
		t.Fatalf("crash not exposed with committed progress: %+v %v", report, err)
	}
	if n, purged := f.progress(t); n != 16 || purged {
		t.Fatalf("crash lost committed progress: %d %v", n, purged)
	}
	if _, err := f.src.Stat(ctx, f.keys[16]); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("crash boundary not after physical deletion: %v", err)
	}
	f.m.stores[f.source] = f.src
	report, err = f.m.PurgeMigrationSource(ctx, f.migration, false)
	if err != nil || report.Count != 1 || report.Deleted != 1 {
		t.Fatalf("resume did not reconcile exactly pending key: %+v %v", report, err)
	}
	if n, purged := f.progress(t); n != 17 || !purged {
		t.Fatalf("resume incomplete: %d %v", n, purged)
	}
}

func TestSourcePurgeRequiresCompletedMigration(t *testing.T) {
	f := newPurgeFixture(t, 1)
	f.exec(t, `UPDATE storage_migrations SET state='cancelled',completed_at=NULL WHERE id=$1`, f.migration)
	report, err := f.m.PurgeMigrationSource(context.Background(), f.migration, false)
	requirePurgeRefusal(t, report, err, "migration_incomplete")
	if _, err := f.src.Stat(context.Background(), f.keys[0]); err != nil {
		t.Fatal("incomplete migration source deleted", err)
	}
}

func TestSourcePurgeDryRunDoesNotRecreateMissingRoot(t *testing.T) {
	f := newPurgeFixture(t, 1)
	if err := f.src.(*blobstore.Local).Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(f.sourceConfig.LocalRoot); err != nil {
		t.Fatal(err)
	}
	delete(f.m.stores, f.source)
	report, err := f.m.PurgeMigrationSource(context.Background(), f.migration, true)
	if err == nil || report.Deleted != 0 {
		t.Fatalf("missing source root accepted: %+v %v", report, err)
	}
	if _, err := os.Stat(f.sourceConfig.LocalRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run recreated source root: %v", err)
	}
}

func TestSourcePurgeTargetDigestMismatch(t *testing.T) {
	f := newPurgeFixture(t, 1)
	ctx := context.Background()
	if err := f.dst.Delete(ctx, f.keys[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.dst.Put(ctx, f.keys[0], strings.NewReader(strings.Repeat("x", int(f.bytes))), blobstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	report, err := f.m.PurgeMigrationSource(ctx, f.migration, false)
	requirePurgeRefusal(t, report, err, "target_mismatch")
	if _, err := f.src.Stat(ctx, f.keys[0]); err != nil {
		t.Fatal("mismatched target caused source deletion", err)
	}
}

func TestSourcePurgeRotatedCredentialAlias(t *testing.T) {
	source := BlobConfig{Driver: BlobDriverS3, S3: blobstore.S3Config{
		Endpoint: "https://old-storage.example.test", Bucket: "novels", Region: "garage",
		AccessKeyID: "old-key", SecretAccessKey: "old-secret",
	}}
	active := BlobConfig{Driver: BlobDriverS3, S3: blobstore.S3Config{
		Endpoint: "https://storage-alias.example.test", Bucket: "novels", Region: "garage",
		AccessKeyID: "rotated-key", SecretAccessKey: "rotated-secret",
	}}
	if SamePhysicalBlobStore(source, active) {
		t.Fatal("fixture must have distinct endpoint identities")
	}
	if !samePurgeStore(source, active) || !samePurgeStore(active, source) {
		t.Fatal("endpoint aliases with rotated credentials must refuse source purge")
	}
}
