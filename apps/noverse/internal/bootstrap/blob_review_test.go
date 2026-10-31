package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestStorageRegionIdentityAndPurgeAliases(t *testing.T) {
	a := BlobConfig{Driver: BlobDriverS3, S3: blobstore.S3Config{Endpoint: "https://ACCOUNT.r2.cloudflarestorage.com:443/", Bucket: "novels", Region: "us-east-1"}}
	b := a
	b.S3.Endpoint = "https://account.r2.cloudflarestorage.com"
	b.S3.Region = "auto"
	if !SamePhysicalBlobStore(a, b) {
		t.Error("signing region changed physical identity")
	}
	for _, endpoint := range []string{b.S3.Endpoint, "https://alias.example.test"} {
		b.S3.Endpoint = endpoint
		if !samePurgeStore(a, b) {
			t.Errorf("purge permitted region/endpoint alias %s", endpoint)
		}
	}
}

func TestStorageRegionSaveInPlace(t *testing.T) {
	f := newPurgeFixture(t, 1)
	ctx := context.Background()
	cfg := f.m.configs[f.target]
	// Garage only accepts its configured signing region. A local HTTP provider
	// exercises both R2 signing aliases without requiring live R2 credentials.
	var mu sync.Mutex
	type object struct {
		data []byte
		sha  string
	}
	objects := map[string]object{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		key := r.URL.Path
		value, exists := objects[key]
		switch r.Method {
		case http.MethodPut:
			if exists && r.Header.Get("If-None-Match") == "*" {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusPreconditionFailed)
				_, _ = io.WriteString(w, "<Error><Code>PreconditionFailed</Code></Error>")
				return
			}
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read failed", 400)
				return
			}
			objects[key] = object{data, r.Header.Get("X-Amz-Meta-Sha256")}
		case http.MethodGet, http.MethodHead:
			if !exists {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("X-Amz-Meta-Sha256", value.sha)
			data := value.data
			if r.Header.Get("Range") != "" {
				var start, end int
				if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil || start < 0 || end >= len(data) || end < start {
					w.WriteHeader(416)
					return
				}
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
				data = data[start : end+1]
				w.Header().Set("Content-Length", strconv.Itoa(len(data)))
				w.WriteHeader(206)
			} else {
				w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			}
			if r.Method == http.MethodGet {
				_, _ = w.Write(data)
			}
		case http.MethodDelete:
			delete(objects, key)
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	}))
	defer server.Close()
	cfg.S3.Endpoint = server.URL
	cfg.S3.Region = "us-east-1"
	tx, err := f.pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	old, err := f.m.saveConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_settings SET active_store=$1 WHERE singleton`, old); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE blobs SET store_id=$1,version=version+1,updated_at=clock_timestamp()`, old); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var version int64
	if err = f.pool.Pool().QueryRow(ctx, `SELECT version FROM storage_settings`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	result, err := f.m.Save(ctx, StorageCandidate{Version: version, Driver: "s3", Provider: "r2", Endpoint: cfg.S3.Endpoint, Region: "auto", Bucket: cfg.S3.Bucket, PathStyle: true, AllowHTTP: true, Delivery: "proxy"})
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsMigration {
		t.Fatal("region alias requires migration instead of in-place update")
	}
	var moved bool
	if err = f.pool.Pool().QueryRow(ctx, `SELECT NOT EXISTS(SELECT FROM blobs WHERE store_id<>(SELECT active_store FROM storage_settings)) AND (SELECT pending_store IS NULL FROM storage_settings)`).Scan(&moved); err != nil || !moved {
		t.Fatalf("in-place routing not updated: %v %v", moved, err)
	}
}

func TestStorageRegionPurgeRefusesActiveAndPending(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, alias := range []bool{false, true} {
			t.Run(fmt.Sprintf("pending=%v/alias=%v", pending, alias), func(t *testing.T) {
				f := newPurgeFixture(t, 1)
				cfg := f.m.configs[f.target]
				cfg.S3.Region = "us-east-1"
				tx, err := f.pool.Pool().Begin(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				source, err := f.m.saveConfig(context.Background(), tx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				cfg.S3.Region = "auto"
				if alias {
					cfg.S3.Endpoint = "https://rotated-alias.example.test"
				}
				current, err := f.m.saveConfig(context.Background(), tx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(context.Background(), `UPDATE storage_migrations SET source_store=$1 WHERE id=$2`, source, f.migration); err != nil {
					t.Fatal(err)
				}
				query := `UPDATE storage_settings SET active_store=$1 WHERE singleton`
				code := "source_active"
				if pending {
					query = `UPDATE storage_settings SET active_store=$2,pending_store=$1 WHERE singleton`
					code = "source_pending"
				}
				args := []any{current}
				if pending {
					args = append(args, f.source)
				}
				if _, err = tx.Exec(context.Background(), query, args...); err != nil {
					t.Fatal(err)
				}
				if err = tx.Commit(context.Background()); err != nil {
					t.Fatal(err)
				}
				// These clients are never reached on a safe refusal. Cache them so a broken
				// identity predicate cannot send traffic to the deliberately fictitious alias.
				f.m.stores[source] = f.dst
				f.m.configs[source] = f.m.configs[f.target]
				f.m.stores[current] = f.dst
				f.m.configs[current] = cfg
				report, err := f.m.PurgeMigrationSource(context.Background(), f.migration, false)
				requirePurgeRefusal(t, report, err, code)
				if _, err = f.dst.Stat(context.Background(), f.keys[0]); err != nil {
					t.Fatalf("active object deleted: %v", err)
				}
			})
		}
	}
}

func publicationFixture(t *testing.T, max int32) (*ManagedBlobs, string) {
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
	pool, err := postgres.Open(ctx, postgres.PoolConfig{DSN: dsn, MaxConns: max})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	t.Setenv("CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{31}, 32)))
	crypt, err := encryptionFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := blobkey.New(blobkey.EnvDev)
	m := &ManagedBlobs{pool: pool, cipher: crypt, keys: keys, stores: map[string]blobstore.BlobStore{}, configs: map[string]BlobConfig{}}
	tx, err := pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	cfg := normalizeBlobConfig(BlobConfig{Driver: BlobDriverLocal, LocalRoot: t.TempDir()})
	id, err := m.saveConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_settings SET active_store=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO projects(id,slug,name) VALUES($1,'publication','Publication')`, purgeProject); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return m, dsn
}

type publicationBarrier struct {
	blobstore.BlobStore
	entered chan<- struct{}
	resume  <-chan struct{}
}

func (b *publicationBarrier) DriverName() string { return blobstore.DriverName(b.BlobStore) }

func (b *publicationBarrier) Put(ctx context.Context, key string, r io.Reader, o blobstore.PutOptions) (blobstore.BlobInfo, error) {
	select {
	case b.entered <- struct{}{}:
	case <-ctx.Done():
		return blobstore.BlobInfo{}, ctx.Err()
	}
	select {
	case <-b.resume:
	case <-ctx.Done():
		return blobstore.BlobInfo{}, ctx.Err()
	}
	return b.BlobStore.Put(ctx, key, r, o)
}
func reviewPublish(ctx context.Context, m *ManagedBlobs, index int) error {
	body := []byte(fmt.Sprintf("publication-%d", index))
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	key, err := m.keys.Import(purgeProject, "", digest, "publication.dat")
	if err != nil {
		return err
	}
	var project pgtype.UUID
	if err = project.Scan(purgeProject); err != nil {
		return err
	}
	_, err = postgres.UploadAndPublishBlob(ctx, postgres.NewAudio(m.pool), m, postgres.PublishInput{ProjectID: project, Key: key, Driver: "local", ContentType: "application/octet-stream", SHA256: digest, Size: int64(len(body))}, bytes.NewReader(body))
	return err
}
func TestPublicationSingleConnectionAndSaturatedPool(t *testing.T) {
	for _, max := range []int32{1, 3} {
		t.Run(fmt.Sprint(max), func(t *testing.T) {
			m, _ := publicationFixture(t, max)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			id, err := m.active(ctx)
			if err != nil {
				t.Fatal(err)
			}
			st, _, err := m.store(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{}, max)
			resume := make(chan struct{})
			done := make(chan error, max)
			m.stores[id] = &publicationBarrier{BlobStore: st, entered: entered, resume: resume}
			// Also exercise a cold config cache with the only pool connection held.
			if max == 1 {
				delete(m.stores, id)
				delete(m.configs, id)
			}
			for i := range int(max) {
				go func() { done <- reviewPublish(ctx, m, i) }()
			}
			if max > 1 {
				for range max {
					select {
					case <-entered:
					case <-ctx.Done():
						t.Fatal("publishers exhausted pool before reaching upload")
					}
				}
			}
			close(resume)
			for range max {
				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("publication with pool=%d failed: %v", max, err)
					}
				case <-ctx.Done():
					t.Fatal("publication deadlocked")
				}
			}
		})
	}
}
func TestPublicationQueuedExclusiveFlip(t *testing.T) {
	m, dsn := publicationFixture(t, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	observer, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close(context.Background())
	source, err := m.active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := m.store(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := m.pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	target, err := m.saveConfig(ctx, tx, normalizeBlobConfig(BlobConfig{Driver: BlobDriverLocal, LocalRoot: t.TempDir()}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_settings SET pending_store=$1`, target); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO storage_migrations(id,source_store,target_store,state) VALUES('queued-flip',$1,$2,'running')`, source, target); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{}, 2)
	resume := make(chan struct{})
	done := make(chan error, 2)
	m.stores[source] = &publicationBarrier{BlobStore: st, entered: entered, resume: resume}
	go func() { done <- reviewPublish(ctx, m, 1) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("old publisher never reached upload")
	}
	flip := make(chan error, 1)
	go func() { flip <- m.migrationPass(ctx) }()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var queued bool
		if err = observer.QueryRow(ctx, `SELECT EXISTS(SELECT FROM pg_locks WHERE locktype='advisory' AND objid=71427014 AND NOT granted AND mode='ExclusiveLock')`).Scan(&queued); err != nil {
			t.Fatal(err)
		}
		if queued {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("exclusive flip never queued")
		}
	}
	go func() { done <- reviewPublish(ctx, m, 2) }()
	for m.pool.Pool().Stat().AcquiredConns() != 3 {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("late publisher never queued")
		}
	}
	close(resume)
	for range 2 {
		select {
		case err = <-done:
			if err != nil {
				t.Fatalf("publisher stalled behind queued flip: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("old publisher/flip/pool deadlocked")
		}
	}
	if err = <-flip; err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err = m.migrationPass(ctx); err != nil {
			t.Fatal(err)
		}
	}
	active, err := m.active(ctx)
	if err != nil || active != target {
		t.Fatalf("flip did not complete: %s %v", active, err)
	}
}

type reviewTransport func(*http.Request) (*http.Response, error)

func (f reviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStorageConditionalCleanupDeleteClassification(t *testing.T) {
	m, _ := publicationFixture(t, 2)
	cfg := normalizeBlobConfig(BlobConfig{Driver: BlobDriverS3, Delivery: "proxy", S3: blobstore.S3Config{Endpoint: "http://storage.invalid", Region: "garage", Bucket: "test", AccessKeyID: "test", SecretAccessKey: "test", AllowPlainHTTP: true, ForcePathStyle: true, HTTPClient: &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
		status, body := 200, ""
		if r.Method == http.MethodDelete {
			status = 403
			body = "<Error><Code>AccessDenied</Code><Message>Delete denied</Message></Error>"
		}
		if r.Header.Get("If-None-Match") == "*" {
			status = 412
			body = "<Error><Code>PreconditionFailed</Code></Error>"
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}}})
	result, err := m.check(context.Background(), cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatal("delete denial swallowed")
	}
	for _, step := range result.Steps {
		if !step.OK && step.Code != "no_delete_permission" {
			t.Errorf("cleanup lost source operation: %+v", step)
		}
	}
}
func TestStorageProbeExpiryBoundsRowsAndCaches(t *testing.T) {
	f := newPurgeFixture(t, 0)
	ctx := context.Background()
	cfg := f.m.configs[f.target]
	cfg.Delivery = "redirect"
	cfg.S3.PresignEndpoint = cfg.S3.Endpoint
	before := len(f.m.stores)
	for range 12 {
		result, err := f.m.check(ctx, cfg, true)
		if err != nil || !result.OK || result.ProbeURL == "" {
			t.Fatalf("probe failed: %+v %v", result, err)
		}
	}
	// Also evict clients warmed by an earlier process/version; probe-only
	// entries must not outlive their last durable reference.
	rows, err := f.pool.Pool().Query(ctx, `SELECT store_id FROM storage_probes`)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if _, _, err := f.m.store(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	f.exec(t, `UPDATE storage_probes SET expires_at=now()-interval '1 second'`)
	if err := f.m.reapProbes(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.pool.Pool().QueryRow(ctx, `SELECT count(*) FROM blob_stores`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 || len(f.m.stores) > before || len(f.m.configs) > before {
		t.Fatalf("expired probes retained credentials: rows=%d clients=%d configs=%d bound=%d", count, len(f.m.stores), len(f.m.configs), before)
	}
}
func TestStorageMetadataEndpointClassified(t *testing.T) {
	m, _ := publicationFixture(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cfg := normalizeBlobConfig(BlobConfig{Driver: BlobDriverS3, Delivery: "proxy", S3: blobstore.S3Config{Endpoint: "http://169.254.169.254", Region: "garage", Bucket: "test", AccessKeyID: "test", SecretAccessKey: "test", AllowPlainHTTP: true, ForcePathStyle: true}})
	result, err := m.check(ctx, cfg, false)
	if len(result.Steps) == 0 {
		t.Fatalf("no classified metadata refusal: %v", err)
	}
	for _, step := range result.Steps {
		if !step.OK && (step.Code != "unsafe_endpoint" || !strings.Contains(step.Reason, "địa chỉ")) {
			t.Errorf("metadata endpoint not safely classified: %+v", step)
		}
	}
	if result.OK || (!errors.Is(err, context.DeadlineExceeded) && err != nil) {
		t.Fatalf("unexpected result: %+v %v", result, err)
	}
}
