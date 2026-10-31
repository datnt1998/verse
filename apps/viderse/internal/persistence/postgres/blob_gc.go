package postgres

import (
	"context"
	"time"

	"github.com/datnt1998/verse/platform/blobstore"
)

// Collect uses a nonblocking placement lock: long readers and writers are
// skipped, not interrupted. A preview never authorizes a subsequent deletion.
func (s *BlobRepository) Collect(ctx context.Context, store string, info blobstore.BlobInfo, cutoff time.Time, apply bool, remove func(context.Context) error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var locked bool
	if err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1,253025))", store+"/"+info.Key).Scan(&locked); err != nil || !locked {
		return false, err
	}
	var retained bool
	err = tx.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM storage_settings WHERE previous_store=$1)
 OR EXISTS(SELECT 1 FROM storage_migrations WHERE state IN ('running','paused','error') AND (from_store=$1 OR to_store=$1))
 OR EXISTS(SELECT 1 FROM storage_live_refs live JOIN blob_refs r USING(logical_path) WHERE r.object_key=$2)
 OR EXISTS(SELECT 1 FROM blob_locations WHERE store_id=$1 AND object_key=$2 AND protected_until>now())`, store, info.Key).Scan(&retained)
	if err != nil {
		return false, err
	}
	if retained {
		if _, err = tx.Exec(ctx, "UPDATE blob_locations SET unreferenced_at=NULL WHERE store_id=$1 AND object_key=$2", store, info.Key); err != nil {
			return false, err
		}
		return false, tx.Commit(ctx)
	}
	// A recently detached old object gets a fresh grace period. Object age alone
	// is insufficient; an upload made months ago may have lost its last ref now.
	var known bool
	var since *time.Time
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM blob_locations WHERE store_id=$1 AND object_key=$2),(SELECT unreferenced_at FROM blob_locations WHERE store_id=$1 AND object_key=$2)", store, info.Key).Scan(&known, &since)
	if err != nil {
		return false, err
	}
	if known && since == nil {
		if _, err = tx.Exec(ctx, "UPDATE blob_locations SET unreferenced_at=now() WHERE store_id=$1 AND object_key=$2", store, info.Key); err != nil {
			return false, err
		}
		return false, tx.Commit(ctx)
	}
	if known && since.After(cutoff) {
		return false, nil
	}
	if info.ModTime.IsZero() || info.ModTime.After(cutoff) {
		return false, nil
	}
	if apply {
		if err = remove(ctx); err != nil {
			return false, err
		}
		if _, err = tx.Exec(ctx, "DELETE FROM blob_locations WHERE store_id=$1 AND object_key=$2", store, info.Key); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}
