package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ManagedBlobs keeps old clients alive for in-flight operations and routes each
// registered key by its durable store identity, not merely local versus S3.
type ManagedBlobs struct {
	pool        *postgres.Pool
	cipher      configCipher
	locked      bool
	keys        blobkey.Keys
	mu          sync.Mutex
	stores      map[string]blobstore.BlobStore
	configs     map[string]BlobConfig
	healthAt    time.Time
	healthError string
}

func NewManagedBlobs(ctx context.Context, pool *postgres.Pool, keys blobkey.Keys) (*ManagedBlobs, error) {
	crypt, err := encryptionFromEnv()
	if err != nil {
		return nil, err
	}
	m := &ManagedBlobs{pool: pool, cipher: crypt, locked: BlobEnvLocked(), keys: keys, stores: map[string]blobstore.BlobStore{}, configs: map[string]BlobConfig{}}
	cfg, err := ResolveBlobConfig(ctx, pool)
	if err != nil {
		return nil, err
	}
	// A global environment configuration is deliberately not persisted with its
	// credentials. It cannot be migrated or changed through the app.
	if m.locked {
		st, err := OpenBlobStore(cfg)
		if err != nil {
			return nil, err
		}
		m.stores[""] = blobstore.NewMetrics(st, slog.Default(), time.Second)
		m.configs[""] = cfg
		return m, nil
	}
	tx, err := pool.Pool().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71427014)`); err != nil {
		return nil, err
	}
	var id *string
	if err = tx.QueryRow(ctx, `SELECT active_store FROM storage_settings WHERE singleton FOR UPDATE`).Scan(&id); err != nil {
		return nil, err
	}
	if id == nil {
		var unsafe bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM blobs WHERE state='ready' AND store_id IS NULL AND storage_driver<>'local')`).Scan(&unsafe); err != nil {
			return nil, err
		}
		if unsafe {
			return nil, errors.New("Có tệp đã đăng ký nhưng chưa có cấu hình kho tương ứng. Giữ lại biến kho cũ, đặt CONFIG_ENCRYPTION_KEY rồi chạy storage settings adopt-env trước khi bỏ biến môi trường")
		}
		value, err := m.saveConfig(ctx, tx, cfg)
		if err != nil {
			return nil, err
		}
		id = &value
		if _, err = tx.Exec(ctx, `UPDATE storage_settings SET active_store=$1 WHERE singleton`, value); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE blobs SET store_id=$1,version=version+1,updated_at=clock_timestamp() WHERE store_id IS NULL`, value); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if _, _, err = m.store(ctx, *id); err != nil {
		return nil, err
	}
	return m, nil
}

// OpenMaintenanceBlobs resolves the identical routing contract without
// initializing settings: a dry-run maintenance command must not write rows.
func OpenMaintenanceBlobs(ctx context.Context, pool *postgres.Pool, keys blobkey.Keys) (*ManagedBlobs, error) {
	crypt, err := encryptionFromEnv()
	if err != nil {
		return nil, err
	}
	cfg, err := ResolveBlobConfig(ctx, pool)
	if err != nil {
		return nil, err
	}
	m := &ManagedBlobs{pool: pool, cipher: crypt, locked: BlobEnvLocked(), keys: keys, stores: map[string]blobstore.BlobStore{}, configs: map[string]BlobConfig{}}
	id, err := m.active(ctx)
	if err != nil {
		return nil, err
	}
	st, err := OpenBlobStore(cfg)
	if err != nil {
		return nil, err
	}
	m.stores[id], m.configs[id] = blobstore.NewMetrics(st, slog.Default(), time.Second), cfg
	return m, nil
}

func (m *ManagedBlobs) store(ctx context.Context, id string) (blobstore.BlobStore, BlobConfig, error) {
	return m.storeFrom(ctx, m.pool.Pool(), id)
}

func (m *ManagedBlobs) storeFrom(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (blobstore.BlobStore, BlobConfig, error) {
	m.mu.Lock()
	st, cfg := m.stores[id], m.configs[id]
	m.mu.Unlock()
	if st != nil {
		return st, cfg, nil
	}
	cfg, err := m.loadConfigFrom(ctx, db, id)
	if err != nil {
		return nil, cfg, err
	}
	raw, err := OpenBlobStore(cfg)
	if err != nil {
		return nil, cfg, err
	}
	st = blobstore.NewMetrics(raw, slog.Default(), time.Second)
	m.mu.Lock()
	if existing := m.stores[id]; existing != nil {
		st = existing
	} else {
		m.stores[id], m.configs[id] = st, cfg
	}
	m.mu.Unlock()
	return st, cfg, nil
}

func (m *ManagedBlobs) active(ctx context.Context) (string, error) {
	if m.locked {
		return "", nil
	}
	var id *string
	err := m.pool.Pool().QueryRow(ctx, `SELECT active_store FROM storage_settings WHERE singleton`).Scan(&id)
	if err != nil || id == nil {
		return "", err
	}
	return *id, nil
}

func (m *ManagedBlobs) route(ctx context.Context, key string) (blobstore.BlobStore, BlobConfig, error) {
	id, err := m.active(ctx)
	if err != nil {
		return nil, BlobConfig{}, err
	}
	if !m.locked {
		var recorded *string
		err = m.pool.Pool().QueryRow(ctx, `SELECT store_id FROM blobs WHERE object_key=$1`, key).Scan(&recorded)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, BlobConfig{}, err
		}
		if recorded != nil {
			id = *recorded
		}
	}
	return m.store(ctx, id)
}

// PublicationStore holds the cross-process shared fence from before PUT until
// registry publication commits. Callers release it on every return path.
func (m *ManagedBlobs) PublicationStore(ctx context.Context) (blobstore.BlobStore, *pgxpool.Conn, func(), error) {
	conn, err := m.pool.Pool().Acquire(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock_shared(71427014)`); err != nil {
		conn.Release()
		return nil, nil, nil, err
	}
	release := func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanup, `SELECT pg_advisory_unlock_shared(71427014)`); err != nil {
			_ = conn.Conn().Close(cleanup)
		}
		conn.Release()
	}
	id := ""
	if !m.locked {
		// Resolve only after the shared lock is granted: a queued flip may
		// change active_store while this publisher waits. Reuse this connection.
		err = conn.QueryRow(ctx, `SELECT active_store FROM storage_settings WHERE singleton`).Scan(&id)
	}
	if err != nil {
		release()
		return nil, nil, nil, err
	}
	st, _, err := m.storeFrom(ctx, conn, id)
	if err != nil {
		release()
		return nil, nil, nil, err
	}
	return st, conn, release, nil
}

func (m *ManagedBlobs) DriverName() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, err := m.active(ctx)
	if err != nil {
		return ""
	}
	_, cfg, err := m.store(ctx, id)
	if err != nil {
		return ""
	}
	return string(cfg.Driver)
}
func (m *ManagedBlobs) Put(ctx context.Context, key string, r io.Reader, opts blobstore.PutOptions) (blobstore.BlobInfo, error) {
	id, err := m.active(ctx)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	st, _, err := m.store(ctx, id)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	return st.Put(ctx, key, r, opts)
}
func (m *ManagedBlobs) Open(ctx context.Context, key string, r blobstore.ByteRange) (io.ReadCloser, blobstore.BlobInfo, error) {
	st, _, err := m.route(ctx, key)
	if err != nil {
		return nil, blobstore.BlobInfo{}, err
	}
	return st.Open(ctx, key, r)
}
func (m *ManagedBlobs) Stat(ctx context.Context, key string) (blobstore.BlobInfo, error) {
	st, _, err := m.route(ctx, key)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	return st.Stat(ctx, key)
}
func (m *ManagedBlobs) Delete(ctx context.Context, key string) error {
	st, _, err := m.route(ctx, key)
	if err != nil {
		return err
	}
	return st.Delete(ctx, key)
}
func (m *ManagedBlobs) List(ctx context.Context, prefix, cursor string, limit int) (blobstore.ListPage, error) {
	id, err := m.active(ctx)
	if err != nil {
		return blobstore.ListPage{}, err
	}
	st, _, err := m.store(ctx, id)
	if err != nil {
		return blobstore.ListPage{}, err
	}
	return st.List(ctx, prefix, cursor, limit)
}
func (m *ManagedBlobs) PresignGet(ctx context.Context, key string, ttl time.Duration, opts blobstore.DownloadOptions) (string, error) {
	st, cfg, err := m.route(ctx, key)
	if err != nil {
		return "", err
	}
	if cfg.Delivery == "proxy" {
		return "", blobstore.ErrPresignUnsupported
	}
	return st.PresignGet(ctx, key, ttl, opts)
}
func (m *ManagedBlobs) EffectivePresignTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return time.Minute
	}
	if ttl > 15*time.Minute {
		return 15 * time.Minute
	}
	return ttl
}
func (m *ManagedBlobs) Snapshot() []blobstore.OperationMetric {
	m.mu.Lock()
	stores := make([]blobstore.BlobStore, 0, len(m.stores))
	for _, st := range m.stores {
		stores = append(stores, st)
	}
	m.mu.Unlock()
	result := []blobstore.OperationMetric{}
	for _, st := range stores {
		if source, ok := st.(interface {
			Snapshot() []blobstore.OperationMetric
		}); ok {
			result = append(result, source.Snapshot()...)
		}
	}
	return result
}

func (m *ManagedBlobs) CheckHealth(ctx context.Context) error {
	id, err := m.active(ctx)
	if err != nil {
		return err
	}
	st, cfg, err := m.store(ctx, id)
	if err == nil {
		err = CheckBlobStore(ctx, st, cfg)
	}
	m.mu.Lock()
	m.healthAt = time.Now().UTC()
	m.healthError = ""
	if err != nil {
		_, m.healthError = storageError(err, "read")
	}
	m.mu.Unlock()
	return err
}

func (m *ManagedBlobs) DeleteMany(ctx context.Context, keys []string) ([]blobstore.DeleteResult, error) {
	// Preserve per-row routing, including GC tombstones from a previous store.
	results := make([]blobstore.DeleteResult, 0, len(keys))
	for _, key := range keys {
		results = append(results, blobstore.DeleteResult{Key: key, Err: m.Delete(ctx, key)})
	}
	return results, nil
}

func (m *ManagedBlobs) ListMultipartUploads(ctx context.Context, prefix, cursor string, limit int) (blobstore.MultipartPage, error) {
	id, err := m.active(ctx)
	if err != nil {
		return blobstore.MultipartPage{}, err
	}
	st, _, err := m.store(ctx, id)
	if err != nil {
		return blobstore.MultipartPage{}, err
	}
	if reaper, ok := st.(blobstore.MultipartReaper); ok {
		return reaper.ListMultipartUploads(ctx, prefix, cursor, limit)
	}
	return blobstore.MultipartPage{}, nil
}

func (m *ManagedBlobs) AbortMultipartUpload(ctx context.Context, upload blobstore.MultipartUpload) error {
	id, err := m.active(ctx)
	if err != nil {
		return err
	}
	st, _, err := m.store(ctx, id)
	if err != nil {
		return err
	}
	if reaper, ok := st.(blobstore.MultipartReaper); ok {
		return reaper.AbortMultipartUpload(ctx, upload)
	}
	return nil
}
