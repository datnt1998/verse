package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mediaTxKey struct{}

func mediaQueries(ctx context.Context, q *gen.Queries) *gen.Queries {
	if tx, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); ok {
		return q.WithTx(tx)
	}
	return q
}
func mediaDB(ctx context.Context, db gen.DBTX) gen.DBTX {
	if tx, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); ok {
		return tx
	}
	return db
}

type BlobRepository struct {
	pool      *pgxpool.Pool
	readSlots chan struct{}
}

func NewBlobRepository(pool *pgxpool.Pool) *BlobRepository {
	// Bound dedicated reader sessions independently of the query pool; each
	// reader can now leave all query connections available to its body.
	limit := int(pool.Config().MaxConns / 2)
	if limit < 1 {
		limit = 1
	}
	return &BlobRepository{pool: pool, readSlots: make(chan struct{}, limit)}
}

var _ media.Repository = (*BlobRepository)(nil)

func (s *BlobRepository) Initialize(ctx context.Context, d media.Descriptor) (media.State, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return media.State{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(253025,1)"); err != nil {
		return media.State{}, err
	}
	if err = s.insertDescriptor(ctx, tx, d); err != nil {
		return media.State{}, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO storage_settings(singleton,active_store) VALUES(true,$1) ON CONFLICT DO NOTHING", d.ID); err != nil {
		return media.State{}, err
	}
	state, err := readStorageState(ctx, tx)
	if err != nil {
		return state, err
	}
	return state, tx.Commit(ctx)
}
func readStorageState(ctx context.Context, db gen.DBTX) (media.State, error) {
	var state media.State
	err := db.QueryRow(ctx, "SELECT active_store,coalesce(pending_store,''),coalesce(previous_store,''),revision,generation FROM storage_settings WHERE singleton").Scan(&state.Active, &state.Pending, &state.Previous, &state.Revision, &state.Generation)
	return state, err
}
func (s *BlobRepository) State(ctx context.Context) (media.State, error) {
	return readStorageState(ctx, mediaDB(ctx, s.pool))
}
func (s *BlobRepository) Descriptor(ctx context.Context, id string) (media.Descriptor, error) {
	var d media.Descriptor
	var raw []byte
	err := mediaDB(ctx, s.pool).QueryRow(ctx, "SELECT id,config,credentials FROM storage_stores WHERE id=$1", id).Scan(&d.ID, &raw, &d.Credentials)
	if err != nil {
		return d, err
	}
	err = json.Unmarshal(raw, &d.Config)
	return d, err
}
func (s *BlobRepository) insertDescriptor(ctx context.Context, db gen.DBTX, d media.Descriptor) error {
	cfg := d.Config
	cfg.S3.AccessKeyID = ""
	cfg.S3.SecretAccessKey = ""
	cfg.S3.HTTPClient = nil
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, "INSERT INTO storage_stores(id,driver,config,credentials) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO NOTHING", d.ID, cfg.Driver, raw, d.Credentials)
	return err
}
func (s *BlobRepository) Configure(ctx context.Context, d media.Descriptor, revision int64, immediate bool) (media.State, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return media.State{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(253025,1)"); err != nil {
		return media.State{}, err
	}
	state, err := readStorageState(ctx, tx)
	if err != nil {
		return state, err
	}
	if state.Revision != revision {
		return state, media.ErrConflict
	}
	var busy bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_migrations WHERE state IN ('running','paused','error'))").Scan(&busy); err != nil {
		return state, err
	}
	if busy {
		return state, media.ErrBusy
	}
	if err = s.insertDescriptor(ctx, tx, d); err != nil {
		return state, err
	}
	if immediate {
		if _, err = tx.Exec(ctx, "INSERT INTO blob_locations(store_id,object_key,source_path,verified_at,protected_until) SELECT $1,object_key,source_path,verified_at,protected_until FROM blob_locations WHERE store_id=$2 ON CONFLICT DO NOTHING", d.ID, state.Active); err != nil {
			return state, err
		}
		_, err = tx.Exec(ctx, "UPDATE storage_settings SET active_store=$1,pending_store=NULL,revision=revision+1,generation=generation+1 WHERE singleton", d.ID)
	} else {
		_, err = tx.Exec(ctx, "UPDATE storage_settings SET pending_store=$1,revision=revision+1 WHERE singleton", d.ID)
	}
	if err != nil {
		return state, err
	}
	state, err = readStorageState(ctx, tx)
	if err != nil {
		return state, err
	}
	return state, tx.Commit(ctx)
}
func (s *BlobRepository) Publication(ctx context.Context, fn func(context.Context, media.State) error) error {
	if _, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); ok {
		return errors.New("nested media publication")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(253025,1)"); err != nil {
		return err
	}
	txctx := context.WithValue(ctx, mediaTxKey{}, tx)
	state, err := readStorageState(txctx, tx)
	if err != nil {
		return err
	}
	if err = fn(txctx, state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Lock protects the placement throughout PUT/verify/manifest commit, using
// the same per-store lock as readers and the destructive GC recheck.
func (s *BlobRepository) Lock(ctx context.Context, storeID, key string) error {
	tx, ok := ctx.Value(mediaTxKey{}).(pgx.Tx)
	if !ok {
		return errors.New("placement lock requires publication transaction")
	}
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,253025))", storeID+"/"+key)
	return err
}

func (s *BlobRepository) Bind(ctx context.Context, r media.Ref) error {
	db := mediaDB(ctx, s.pool)
	if _, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); !ok {
		return errors.New("manifest bind requires publication transaction")
	}
	if err := media.ValidatePath(r.Path); err != nil {
		return err
	}
	key, err := media.ContentKey(r.SHA256)
	if err != nil || key != r.Key {
		return blobstore.ErrInvalidKey
	}
	if _, err = db.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,253025))", r.StoreID+"/"+r.Key); err != nil {
		return err
	}
	result, err := db.Exec(ctx, "INSERT INTO blobs(object_key,sha256,bytes) VALUES($1,$2,$3) ON CONFLICT(object_key) DO UPDATE SET bytes=blobs.bytes WHERE blobs.sha256=excluded.sha256 AND blobs.bytes=excluded.bytes", r.Key, r.SHA256, r.Bytes)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return blobstore.ErrDigestMismatch
	}
	if _, err = db.Exec(ctx, "INSERT INTO blob_locations(store_id,object_key,source_path,verified_at) VALUES($1,$2,nullif($3,''),$4) ON CONFLICT(store_id,object_key) DO UPDATE SET verified_at=excluded.verified_at,unreferenced_at=NULL", r.StoreID, r.Key, r.SourcePath, r.VerifiedAt); err != nil {
		return err
	}
	result, err = db.Exec(ctx, "INSERT INTO blob_refs(logical_path,object_key,content_type,filename) VALUES($1,$2,$3,$4) ON CONFLICT(logical_path) DO UPDATE SET content_type=excluded.content_type WHERE blob_refs.object_key=excluded.object_key", r.Path, r.Key, r.ContentType, r.Filename)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return media.ErrConflict
	}
	return nil
}

// Unbind gỡ hàng blob_refs của (store, path) trong publication transaction.
// Khoá advisory per-key khớp Collect/Bind nên unbind không đua với GC đang
// kiểm tra cùng object; path vắng mặt là no-op. Một path còn chủ sống thì giữ
// nguyên binding: storage_live_refs là cùng nguồn quyền mà editor
// releaseBinding dùng, nên head editor của project khác, artifact/cutout của
// job khác hay thư viện đang trỏ vào path không bị xoá bytes dưới chân. Binding
// đó được nhả bởi lần xoá canonical của chủ cuối, không phải bởi Unbind.
func (s *BlobRepository) Unbind(ctx context.Context, storeID, path string) error {
	db := mediaDB(ctx, s.pool)
	if _, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); !ok {
		return errors.New("manifest unbind requires publication transaction")
	}
	if err := media.ValidatePath(path); err != nil {
		return err
	}
	// Khoá theo object key hiện tại của path (nếu có) để nghiêm chỉnh đua với
	// Collect; khoá theo path để hai unbind/bind cùng path cũng xếp hàng.
	var key string
	err := db.QueryRow(ctx, "SELECT object_key FROM blob_refs WHERE logical_path=$1", path).Scan(&key)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if key != "" {
		if _, err = db.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,253025))", storeID+"/"+key); err != nil {
			return err
		}
	}
	if _, err = db.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,253025))", storeID+"/path/"+path); err != nil {
		return err
	}
	var live bool
	if err = db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_live_refs WHERE logical_path=$1)", path).Scan(&live); err != nil {
		return err
	}
	if live {
		return nil
	}
	if _, err = db.Exec(ctx, "DELETE FROM blob_refs WHERE logical_path=$1", path); err != nil {
		return err
	}
	return nil
}
func (s *BlobRepository) Resolve(ctx context.Context, store, path string) (media.Ref, error) {
	var r media.Ref
	r.StoreID = store
	r.Path = path
	err := mediaDB(ctx, s.pool).QueryRow(ctx, "SELECT r.object_key,b.sha256,b.bytes,r.content_type,r.filename,coalesce(l.source_path,''),l.verified_at FROM blob_refs r JOIN blobs b USING(object_key) JOIN blob_locations l USING(object_key) WHERE l.store_id=$1 AND r.logical_path=$2", store, path).Scan(&r.Key, &r.SHA256, &r.Bytes, &r.ContentType, &r.Filename, &r.SourcePath, &r.VerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = blobstore.ErrNotFound
	}
	return r, err
}
func (s *BlobRepository) Roots(ctx context.Context) ([]media.Root, error) {
	rows, err := mediaDB(ctx, s.pool).Query(ctx, "SELECT logical_path,content_type,usage_group FROM storage_live_refs ORDER BY logical_path")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []media.Root{}
	for rows.Next() {
		var r media.Root
		if err := rows.Scan(&r.Path, &r.ContentType, &r.Group); err != nil {
			return nil, err
		}
		if err := media.ValidatePath(r.Path); err != nil {
			return nil, fmt.Errorf("invalid durable reference: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *BlobRepository) Protect(ctx context.Context, store, key string, until time.Time) error {
	result, err := mediaDB(ctx, s.pool).Exec(ctx, "UPDATE blob_locations SET protected_until=greatest(protected_until,$3) WHERE store_id=$1 AND object_key=$2", store, key, until)
	if err == nil && result.RowsAffected() != 1 {
		return blobstore.ErrNotFound
	}
	return err
}
func (s *BlobRepository) ReadLease(ctx context.Context, store, key string) (func(), error) {
	select {
	case s.readSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	leased := false
	defer func() {
		if !leased {
			<-s.readSlots
		}
	}()
	conn, err := pgx.ConnectConfig(ctx, s.pool.Config().ConnConfig.Copy())
	if err != nil {
		return nil, err
	}
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock_shared(hashtextextended($1,253025))", store+"/"+key); err != nil {
		_ = conn.Close(context.Background())
		return nil, err
	}
	var exists bool
	if err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM blob_locations WHERE store_id=$1 AND object_key=$2)", store, key).Scan(&exists); err != nil || !exists {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = conn.Exec(cleanup, "SELECT pg_advisory_unlock_shared(hashtextextended($1,253025))", store+"/"+key)
		_ = conn.Close(cleanup)
		cancel()
		if err == nil {
			err = blobstore.ErrNotFound
		}
		return nil, err
	}
	var once sync.Once
	leased = true
	return func() {
		once.Do(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = conn.Exec(cleanup, "SELECT pg_advisory_unlock_shared(hashtextextended($1,253025))", store+"/"+key)
			_ = conn.Close(cleanup)
			<-s.readSlots
		})
	}, nil
}
