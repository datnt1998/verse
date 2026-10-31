package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/jackc/pgx/v5"
)

func (m *ManagedBlobs) MigrationAction(ctx context.Context, action string, version int64) error {
	if m.locked {
		return ErrStorageLocked
	}
	tx, err := m.pool.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize controls with the final switch; cancel never races a completed flip.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71427014)`); err != nil {
		return err
	}
	var current int64
	var source string
	var target *string
	if err = tx.QueryRow(ctx, `SELECT version,active_store,pending_store FROM storage_settings WHERE singleton FOR UPDATE`).Scan(&current, &source, &target); err != nil {
		return err
	}
	if current != version {
		return ErrStorageConflict
	}
	switch action {
	case "start":
		if target == nil {
			return errors.New("Chưa có cấu hình kho mới để chuyển")
		}
		var token [16]byte
		if _, err = rand.Read(token[:]); err != nil {
			return err
		}
		id := hex.EncodeToString(token[:])
		_, err = tx.Exec(ctx, `INSERT INTO storage_migrations(id,source_store,target_store,state,total_count,total_bytes) SELECT $1,$2,$3,'running',count(*),COALESCE(sum(size_bytes),0)::bigint FROM blobs WHERE state='ready' AND store_id=$2`, id, source, *target)
	case "pause":
		tag, e := tx.Exec(ctx, `UPDATE storage_migrations SET state='paused',updated_at=now() WHERE state='running'`)
		err = e
		if err == nil && tag.RowsAffected() == 0 {
			err = ErrStorageConflict
		}
	case "resume":
		tag, e := tx.Exec(ctx, `UPDATE storage_migrations SET state='running',error_code='',updated_at=now() WHERE state IN ('paused','error')`)
		err = e
		if err == nil && tag.RowsAffected() == 0 {
			err = ErrStorageConflict
		}
	case "cancel":
		tag, e := tx.Exec(ctx, `UPDATE storage_migrations SET state='cancelled',updated_at=now() WHERE state IN ('running','paused','error')`)
		err = e
		if err == nil && tag.RowsAffected() == 0 {
			err = ErrStorageConflict
		}
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE storage_settings SET pending_store=NULL WHERE singleton`)
		}
	default:
		return errors.New("Thao tác chuyển kho không hợp lệ")
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_settings SET version=version+1 WHERE singleton`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Run resumes durable jobs and expires browser probes. One advisory leader lock
// prevents two serving processes copying/finalizing the same migration.
func (m *ManagedBlobs) Run(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	nextHealth := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		if !time.Now().Before(nextHealth) {
			healthContext, cancel := context.WithTimeout(ctx, 3*time.Second)
			_ = m.CheckHealth(healthContext)
			cancel()
			nextHealth = time.Now().Add(30 * time.Second)
		}
		if err := m.reapProbes(ctx); err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if m.locked {
			continue
		}
		if err := m.migrationPass(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, _ = m.pool.Pool().Exec(ctx, `UPDATE storage_migrations SET state='error',error_code='storage_unavailable',updated_at=now() WHERE state='running'`)
		}
	}
}

func (m *ManagedBlobs) reapProbes(ctx context.Context) error {
	rows, err := m.pool.Pool().Query(ctx, `SELECT store_id,object_key FROM storage_probes WHERE expires_at<=now() LIMIT 32`)
	if err != nil {
		return err
	}
	type probe struct{ id, key string }
	items := []probe{}
	for rows.Next() {
		var p probe
		if err = rows.Scan(&p.id, &p.key); err != nil {
			rows.Close()
			return err
		}
		items = append(items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range items {
		// Probe configs are temporary. Never put their decrypted credentials or
		// clients in the durable routing caches just to expire an object.
		cfg, err := m.loadConfig(ctx, p.id)
		if err != nil {
			return err
		}
		st, err := OpenBlobStore(cfg)
		if err != nil {
			return err
		}
		deleteErr := st.Delete(ctx, p.key)
		if closer, ok := st.(interface{ Close() error }); ok {
			deleteErr = errors.Join(deleteErr, closer.Close())
		}
		if deleteErr != nil {
			return deleteErr
		}
		_, err = m.pool.Pool().Exec(ctx, `DELETE FROM storage_probes WHERE store_id=$1 AND object_key=$2`, p.id, p.key)
		if err != nil {
			return err
		}
	}
	return m.pruneUnusedStoreConfigs(ctx)
}

// References, rather than age or credentials, determine whether a saved config
// is still needed. This also catches a crash between probe and config removal.
func (m *ManagedBlobs) pruneUnusedStoreConfigs(ctx context.Context) error {
	rows, err := m.pool.Pool().Query(ctx, `DELETE FROM blob_stores s
		WHERE NOT EXISTS(SELECT FROM storage_settings WHERE active_store=s.id OR pending_store=s.id)
		AND NOT EXISTS(SELECT FROM storage_migrations WHERE source_store=s.id OR target_store=s.id)
		AND NOT EXISTS(SELECT FROM storage_probes WHERE store_id=s.id)
		AND NOT EXISTS(SELECT FROM blobs WHERE store_id=s.id)
		RETURNING id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		m.mu.Lock()
		delete(m.stores, id)
		delete(m.configs, id)
		m.mu.Unlock()
	}
	return rows.Err()
}

type migrationBlob struct {
	key, sha, store string
	size            int64
}

func (m *ManagedBlobs) migrationPass(ctx context.Context) error {
	conn, err := m.pool.Pool().Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var leader bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(71427015)`).Scan(&leader); err != nil || !leader {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanup, `SELECT pg_advisory_unlock(71427015)`); err != nil {
			_ = conn.Conn().Close(cleanup)
		}
	}()
	var id, source, target string
	err = conn.QueryRow(ctx, `SELECT id,source_store,target_store FROM storage_migrations WHERE state='running' LIMIT 1`).Scan(&id, &source, &target)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	dst, targetConfig, err := m.storeFrom(ctx, conn, target)
	if err != nil {
		return err
	}
	rows, err := conn.Query(ctx, `SELECT b.object_key,b.sha256,b.size_bytes,b.store_id FROM blobs b LEFT JOIN storage_migration_items i ON i.migration_id=$1 AND i.object_key=b.object_key AND i.sha256=b.sha256 AND i.size_bytes=b.size_bytes AND i.verified WHERE b.state='ready' AND b.store_id=$2 AND i.object_key IS NULL ORDER BY b.object_key LIMIT 32`, id, source)
	if err != nil {
		return err
	}
	batch := []migrationBlob{}
	for rows.Next() {
		var b migrationBlob
		if err = rows.Scan(&b.key, &b.sha, &b.size, &b.store); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, b := range batch {
		var state string
		if err = conn.QueryRow(ctx, `SELECT state FROM storage_migrations WHERE id=$1`, id).Scan(&state); err != nil {
			return err
		}
		if state != "running" {
			return nil
		}
		src, _, copyErr := m.storeFrom(ctx, conn, b.store)
		var entry blobstore.LedgerEntry
		if copyErr == nil {
			entry, copyErr = blobstore.NewCopier(src, dst).CopyKey(ctx, b.key)
		}
		if copyErr == nil && (entry.SHA256 != b.sha || entry.Size != b.size || (entry.Status != "copied" && entry.Status != "matched")) {
			copyErr = blobstore.ErrDigestMismatch
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		code := ""
		if copyErr != nil {
			code, _ = storageError(copyErr, "copy")
			var origin *blobstore.CopySourceError
			if errors.As(copyErr, &origin) && errors.Is(copyErr, blobstore.ErrNotFound) {
				code = "source_missing"
			}
		}
		// Deleted/tombstoned rows are no longer migration input. Never resurrect one.
		_, err = conn.Exec(ctx, `INSERT INTO storage_migration_items(migration_id,object_key,sha256,size_bytes,verified,error_code) SELECT $1,$2,$3,$4,$5,$6 WHERE EXISTS(SELECT FROM blobs WHERE object_key=$2 AND state='ready' AND store_id=$7 AND sha256=$3 AND size_bytes=$4) ON CONFLICT(migration_id,object_key) DO UPDATE SET sha256=excluded.sha256,size_bytes=excluded.size_bytes,verified=excluded.verified,error_code=excluded.error_code`, id, b.key, b.sha, b.size, copyErr == nil, code, source)
		if err != nil {
			return err
		}
	}
	_, err = conn.Exec(ctx, `UPDATE storage_migrations SET copied_count=(SELECT count(*) FROM storage_migration_items WHERE migration_id=$1 AND verified),copied_bytes=(SELECT COALESCE(sum(size_bytes),0) FROM storage_migration_items WHERE migration_id=$1 AND verified),total_count=(SELECT count(*) FROM blobs WHERE state='ready' AND store_id=$2),total_bytes=(SELECT COALESCE(sum(size_bytes),0) FROM blobs WHERE state='ready' AND store_id=$2),updated_at=now() WHERE id=$1`, id, source)
	if err != nil {
		return err
	}
	var failures bool
	if err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT FROM storage_migration_items i JOIN blobs b ON b.object_key=i.object_key AND b.sha256=i.sha256 AND b.size_bytes=i.size_bytes WHERE i.migration_id=$1 AND i.error_code<>'' AND b.state='ready' AND b.store_id=$2)`, id, source).Scan(&failures); err != nil {
		return err
	}
	if failures {
		_, err = conn.Exec(ctx, `UPDATE storage_migrations SET state='error',updated_at=now() WHERE id=$1 AND state='running'`, id)
		return err
	}
	if len(batch) > 0 {
		return nil
	}
	// Fence is deliberately short: all network copies happen before taking it.
	// Writers already inside publication finish on the old client; later ones
	// wait and resolve the new active client after this transaction commits.
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71427014)`); err != nil {
		return err
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM storage_migrations WHERE id=$1 FOR UPDATE`, id).Scan(&state); err != nil {
		return err
	}
	if state != "running" {
		return nil
	}
	var missing bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM blobs b WHERE state='ready' AND store_id=$2 AND NOT EXISTS(SELECT FROM storage_migration_items i WHERE migration_id=$1 AND i.object_key=b.object_key AND i.sha256=b.sha256 AND i.size_bytes=b.size_bytes AND i.verified))`, id, source).Scan(&missing); err != nil {
		return err
	}
	if missing {
		return nil
	}
	if _, err = tx.Exec(ctx, `UPDATE blobs SET store_id=$1,storage_driver=$2,version=version+1,updated_at=clock_timestamp() WHERE state='ready' AND store_id=$3`, target, string(targetConfig.Driver), source); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_settings SET active_store=$1,pending_store=NULL,version=version+1 WHERE singleton AND active_store=$2`, target, source); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE storage_migrations SET state='completed',completed_at=clock_timestamp(),updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
