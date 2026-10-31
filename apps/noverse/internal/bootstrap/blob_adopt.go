package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

type StorageAdoption struct {
	Changed bool   `json:"changed"`
	Version int64  `json:"version"`
	Count   int64  `json:"count"`
	Driver  string `json:"driver"`
	Bucket  string `json:"bucket"`
}

func requireStorageMaintenance(ctx context.Context, pool *postgres.Pool) error {
	var permitted bool
	if err := pool.Pool().QueryRow(ctx, `SELECT noverse_is_maintenance()`).Scan(&permitted); err != nil {
		return errors.New("Không xác minh được vai trò bảo trì")
	}
	if !permitted {
		return errors.New("Lệnh này yêu cầu vai trò bảo trì noverse_migrate qua MIGRATION_DATABASE_URL")
	}
	return nil
}

// AdoptBlobEnv is an explicit, tested handover from locked environment config
// to durable application config. It does not copy or silently relocate data.
func AdoptBlobEnv(ctx context.Context, pool *postgres.Pool, keys blobkey.Keys) (StorageAdoption, error) {
	result := StorageAdoption{}
	crypt, err := encryptionFromEnv()
	if err != nil {
		return result, err
	}
	if crypt.aead == nil {
		return result, ErrStorageEncryption
	}
	if err = requireStorageMaintenance(ctx, pool); err != nil {
		return result, err
	}
	if !BlobEnvLocked() {
		return result, errors.New("Chưa có cấu hình kho trong môi trường để tiếp nhận")
	}
	cfg, err := BlobConfigFromEnv()
	if err != nil {
		return result, err
	}
	cfg = normalizeBlobConfig(cfg)
	manager := &ManagedBlobs{pool: pool, cipher: crypt, keys: keys}
	check, err := manager.check(ctx, cfg, false)
	if err != nil {
		return result, err
	}
	if !check.OK {
		return result, errors.New("Kho môi trường chưa vượt qua đủ các bước kiểm tra kết nối; chưa thay đổi dữ liệu")
	}
	tx, err := pool.Pool().Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71427014)`); err != nil {
		return result, err
	}
	var active *string
	if err = tx.QueryRow(ctx, `SELECT active_store,version FROM storage_settings WHERE singleton FOR UPDATE`).Scan(&active, &result.Version); err != nil {
		return result, err
	}
	var busy bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM storage_migrations WHERE state IN ('running','paused','error'))`).Scan(&busy); err != nil {
		return result, err
	}
	if busy {
		return result, errors.New("Hãy hoàn thành hoặc huỷ việc chuyển kho trước khi tiếp nhận cấu hình môi trường")
	}
	if active != nil {
		old, err := manager.loadConfigFrom(ctx, tx, *active)
		if err != nil {
			return result, err
		}
		a, _ := json.Marshal(old)
		b, _ := json.Marshal(cfg)
		if bytes.Equal(a, b) {
			result.Driver, result.Bucket = string(cfg.Driver), cfg.S3.Bucket
			return result, nil
		}
		if !SamePhysicalBlobStore(old, cfg) {
			return result, errors.New("Cấu hình môi trường trỏ sang kho khác với DB; hãy giữ kho hiện tại và dùng chức năng chuyển kho, không tiếp nhận đè")
		}
	}
	var mismatch bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM blobs WHERE state='ready' AND store_id IS NULL AND storage_driver<>$1)`, string(cfg.Driver)).Scan(&mismatch); err != nil {
		return result, err
	}
	if mismatch {
		return result, errors.New("Loại kho trong môi trường không khớp tệp đã đăng ký; giữ nguyên cấu hình cũ và đối soát trước khi tiếp nhận")
	}
	id, err := manager.saveConfig(ctx, tx, cfg)
	if err != nil {
		return result, err
	}
	tag, err := tx.Exec(ctx, `UPDATE blobs SET store_id=$1,version=version+1,updated_at=clock_timestamp() WHERE store_id IS NULL OR store_id=$2`, id, active)
	if err != nil {
		return result, err
	}
	result.Count = tag.RowsAffected()
	if _, err = tx.Exec(ctx, `UPDATE storage_settings SET active_store=$1,pending_store=NULL,version=version+1 WHERE singleton`, id); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Changed = true
	result.Version++
	result.Driver, result.Bucket = string(cfg.Driver), cfg.S3.Bucket
	return result, nil
}

// BackfillStorageUsage is an explicit, idempotent, paginated maintenance job.
// The serving role never performs a bulk backfill while starting up.
func BackfillStorageUsage(ctx context.Context, pool *postgres.Pool, dryRun bool) (int64, error) {
	if err := requireStorageMaintenance(ctx, pool); err != nil {
		return 0, err
	}
	type entry struct{ key, contentType string }
	var count int64
	cursor := ""
	for {
		rows, err := pool.Pool().Query(ctx, `SELECT object_key,content_type FROM blobs WHERE state='ready' AND usage_kind='unclassified' AND object_key>$1 ORDER BY object_key LIMIT 256`, cursor)
		if err != nil {
			return count, err
		}
		batch := []entry{}
		for rows.Next() {
			var item entry
			if err = rows.Scan(&item.key, &item.contentType); err != nil {
				rows.Close()
				return count, err
			}
			batch = append(batch, item)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return count, err
		}
		if len(batch) == 0 {
			return count, nil
		}
		if dryRun {
			count += int64(len(batch))
		} else {
			tx, err := pool.Pool().Begin(ctx)
			if err != nil {
				return count, err
			}
			for _, item := range batch {
				tag, err := tx.Exec(ctx, `UPDATE blobs SET usage_kind=$2,version=version+1,updated_at=clock_timestamp() WHERE object_key=$1 AND state='ready' AND usage_kind='unclassified'`, item.key, blobkey.UsageKind(item.key, item.contentType))
				if err != nil {
					tx.Rollback(ctx)
					return count, err
				}
				count += tag.RowsAffected()
			}
			if err = tx.Commit(ctx); err != nil {
				return count, err
			}
		}
		cursor = batch[len(batch)-1].key
	}
}
