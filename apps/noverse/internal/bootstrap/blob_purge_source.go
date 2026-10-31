package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5"
)

var ErrSourcePurgeRefused = errors.New("chưa thể dọn hết kho nguồn; xem các lý do từ chối")

// SourcePurgeRefusal identifies each object or migration-wide safety refusal.
type SourcePurgeRefusal struct {
	Key    string `json:"key,omitempty"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type SourcePurgeReport struct {
	MigrationID  string               `json:"migrationId"`
	DryRun       bool                 `json:"dryRun"`
	Count        int64                `json:"count"`
	Bytes        int64                `json:"bytes"`
	Deleted      int64                `json:"deleted"`
	DeletedBytes int64                `json:"deletedBytes"`
	Refusals     []SourcePurgeRefusal `json:"refusals"`
}

func (r SourcePurgeReport) Format() string {
	var out strings.Builder
	fmt.Fprintf(&out, "Dọn kho nguồn của lần chuyển %s (dry-run=%t): %d tệp / %d byte có thể xoá; đã xoá %d tệp / %d byte.\n", r.MigrationID, r.DryRun, r.Count, r.Bytes, r.Deleted, r.DeletedBytes)
	for _, refusal := range r.Refusals {
		fmt.Fprintf(&out, "Từ chối [%s] %s: %s\n", refusal.Code, refusal.Key, refusal.Reason)
	}
	return out.String()
}

// PurgeMigrationSource opens only the saved source and target, never an active
// local store whose normal constructor might create directories during preview.
func PurgeMigrationSource(ctx context.Context, pool *postgres.Pool, keys blobkey.Keys, id string, dryRun bool) (SourcePurgeReport, error) {
	crypt, err := encryptionFromEnv()
	if err != nil {
		return SourcePurgeReport{MigrationID: id, DryRun: dryRun}, err
	}
	m := &ManagedBlobs{pool: pool, cipher: crypt, keys: keys, locked: BlobEnvLocked(), stores: map[string]blobstore.BlobStore{}, configs: map[string]BlobConfig{}}
	return m.PurgeMigrationSource(ctx, id, dryRun)
}

// PurgeMigrationSource deletes only this migration's verified manifest. The
// activation fence is held across each small delete batch and its progress
// commit, not across the whole run. A process crash after physical deletion but
// before commit safely repeats the idempotent delete on the next invocation.
// Dry runs use read-only transactions and never initialise settings or progress.
func (m *ManagedBlobs) PurgeMigrationSource(ctx context.Context, id string, dryRun bool) (SourcePurgeReport, error) {
	report := SourcePurgeReport{MigrationID: id, DryRun: dryRun, Refusals: []SourcePurgeRefusal{}}
	if err := postgres.NewBlobGC(m.pool).RequireMaintenance(ctx); err != nil {
		return report, err
	}
	var source, target string
	err := m.pool.Pool().QueryRow(ctx, `SELECT source_store,target_store FROM storage_migrations WHERE id=$1`, id).Scan(&source, &target)
	if errors.Is(err, pgx.ErrNoRows) {
		report.Refusals = append(report.Refusals, SourcePurgeRefusal{Code: "migration_missing", Reason: "Không tìm thấy lần chuyển kho"})
		return report, ErrSourcePurgeRefused
	}
	if err != nil {
		return report, err
	}
	sourceConfig, err := m.loadConfig(ctx, source)
	if err != nil {
		return report, err
	}
	targetConfig, err := m.loadConfig(ctx, target)
	if err != nil {
		return report, err
	}
	src, closeSource, err := m.openPurgeStore(ctx, source, sourceConfig)
	if err != nil {
		return report, err
	}
	defer closeSource()
	dst, closeTarget, err := m.openPurgeStore(ctx, target, targetConfig)
	if err != nil {
		return report, err
	}
	defer closeTarget()
	var runtimeConfig *BlobConfig
	if m.locked {
		cfg, err := ResolveBlobConfig(ctx, m.pool)
		if err != nil {
			return report, err
		}
		runtimeConfig = &cfg
	}
	cursor := ""
	for {
		last, finished, err := m.purgeSourceBatch(ctx, id, sourceConfig, targetConfig, runtimeConfig, target, src, dst, cursor, &report)
		if err != nil {
			return report, err
		}
		if finished {
			break
		}
		cursor = last
	}
	if len(report.Refusals) != 0 {
		return report, ErrSourcePurgeRefused
	}
	return report, nil
}

type sourcePurgeItem struct {
	key, sha string
	size     int64
	verified bool
}

func (m *ManagedBlobs) purgeSourceBatch(ctx context.Context, id string, sourceConfig, targetConfig BlobConfig, runtimeConfig *BlobConfig, target string, src, dst blobstore.BlobStore, cursor string, report *SourcePurgeReport) (string, bool, error) {
	txOptions := pgx.TxOptions{}
	if report.DryRun {
		txOptions.AccessMode = pgx.ReadOnly
	}
	tx, err := m.pool.Pool().BeginTx(ctx, txOptions)
	if err != nil {
		return cursor, false, err
	}
	defer tx.Rollback(ctx)
	// The same exclusive advisory fence protects Save, migration controls and
	// activation. Holding it through DELETE prevents A→B→A between check/delete.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71427014)`); err != nil {
		return cursor, false, err
	}
	global := []SourcePurgeRefusal{}
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM storage_migrations WHERE id=$1`, id).Scan(&state); err != nil {
		return cursor, false, err
	}
	if state != "completed" {
		global = append(global, SourcePurgeRefusal{Code: "migration_incomplete", Reason: "Chỉ được dọn nguồn của lần chuyển đã hoàn thành"})
	}
	if samePurgeStore(sourceConfig, targetConfig) {
		global = append(global, SourcePurgeRefusal{Code: "source_is_target", Reason: "Kho nguồn và kho đích là cùng một kho vật lý"})
	}
	var active, pending *string
	if err = tx.QueryRow(ctx, `SELECT active_store,pending_store FROM storage_settings WHERE singleton`).Scan(&active, &pending); err != nil {
		return cursor, false, err
	}
	for _, check := range []struct {
		id           *string
		code, reason string
	}{
		{active, "source_active", "Kho nguồn đang được dùng; không được xoá"},
		{pending, "source_pending", "Kho nguồn đang là kho đích chờ chuyển; không được xoá"},
	} {
		if check.id == nil {
			continue
		}
		cfg, err := m.loadConfigFrom(ctx, tx, *check.id)
		if err != nil {
			return cursor, false, err
		}
		if samePurgeStore(sourceConfig, cfg) {
			global = append(global, SourcePurgeRefusal{Code: check.code, Reason: check.reason})
		}
	}
	if runtimeConfig != nil && samePurgeStore(sourceConfig, *runtimeConfig) {
		global = append(global, SourcePurgeRefusal{Code: "source_active_env", Reason: "Kho nguồn đang được máy chủ dùng qua cấu hình môi trường"})
	}
	if len(global) > 0 {
		report.Refusals = append(report.Refusals, global...)
		if !report.DryRun {
			return cursor, false, ErrSourcePurgeRefused
		}
	}
	rows, err := tx.Query(ctx, `SELECT object_key,sha256,size_bytes,verified FROM storage_migration_items WHERE migration_id=$1 AND source_purged_at IS NULL AND object_key>$2 ORDER BY object_key LIMIT 16`, id, cursor)
	if err != nil {
		return cursor, false, err
	}
	items := make([]sourcePurgeItem, 0, 16)
	for rows.Next() {
		var item sourcePurgeItem
		if err = rows.Scan(&item.key, &item.sha, &item.size, &item.verified); err != nil {
			rows.Close()
			return cursor, false, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return cursor, false, err
	}
	if len(items) == 0 {
		if !report.DryRun && len(report.Refusals) == 0 {
			_, err = tx.Exec(ctx, `UPDATE storage_migrations SET purged_at=COALESCE(purged_at,clock_timestamp()) WHERE id=$1 AND state='completed' AND NOT EXISTS(SELECT FROM storage_migration_items WHERE migration_id=$1 AND source_purged_at IS NULL)`, id)
			if err != nil {
				return cursor, false, err
			}
		}
		return cursor, true, tx.Commit(ctx)
	}
	keys := make([]string, 0, len(items))
	sizes := make(map[string]int64, len(items))
	for _, item := range items {
		cursor = item.key
		refuse := func(code, reason string) {
			report.Refusals = append(report.Refusals, SourcePurgeRefusal{Key: item.key, Code: code, Reason: reason})
		}
		if !item.verified {
			refuse("manifest_unverified", "Manifest chưa xác minh tệp trên kho đích")
			continue
		}
		if !strings.HasPrefix(item.key, m.keys.Prefix()) {
			refuse("wrong_environment", "Tệp không thuộc môi trường đang bảo trì")
			continue
		}
		// Prevent registry reassignment/tombstoning until this batch commits.
		query := `SELECT store_id,sha256,size_bytes,state FROM blobs WHERE object_key=$1`
		if !report.DryRun {
			query += ` FOR SHARE`
		}
		var registryStore *string
		var sha, registryState string
		var size int64
		err = tx.QueryRow(ctx, query, item.key).Scan(&registryStore, &sha, &size, &registryState)
		if errors.Is(err, pgx.ErrNoRows) {
			refuse("registry_missing", "Tệp không còn trong sổ đăng ký")
			continue
		}
		if err != nil {
			return cursor, false, err
		}
		if registryStore == nil || *registryStore != target || sha != item.sha || size != item.size || registryState != "ready" {
			refuse("registry_not_target", "Sổ đăng ký không còn trỏ đúng tệp đã xác minh trên kho đích")
			continue
		}
		info, statErr := dst.Stat(ctx, item.key)
		if statErr != nil {
			code, reason := "target_unavailable", "Không kiểm tra được tệp trên kho đích"
			if errors.Is(statErr, blobstore.ErrNotFound) {
				code, reason = "target_missing", "Kho đích không còn tệp; giữ nguyên tệp nguồn"
			}
			refuse(code, reason)
			continue
		}
		if info.Size != item.size || (info.SHA256 != "" && info.SHA256 != item.sha) {
			refuse("target_mismatch", "Kích thước hoặc SHA-256 trên kho đích không khớp manifest")
			continue
		}
		if len(global) != 0 {
			continue
		}
		report.Count++
		report.Bytes += item.size
		keys = append(keys, item.key)
		sizes[item.key] = item.size
	}
	if !report.DryRun && len(keys) > 0 {
		// Do not use the helper's retry loop: a failed batch is reported and the
		// next invocation rechecks every safety condition before retrying.
		var results []blobstore.DeleteResult
		var deleteErr error
		if batch, ok := src.(blobstore.BatchDeleter); ok {
			results, deleteErr = batch.DeleteMany(ctx, keys)
		} else {
			for _, key := range keys {
				results = append(results, blobstore.DeleteResult{Key: key, Err: src.Delete(ctx, key)})
			}
		}
		for index, result := range results {
			if index >= len(keys) || result.Key != keys[index] {
				return cursor, false, errors.New("kho nguồn trả kết quả xoá không khớp lô")
			}
			if result.Err != nil {
				report.Refusals = append(report.Refusals, SourcePurgeRefusal{Key: result.Key, Code: "delete_failed", Reason: "Chưa xoá được tệp nguồn; có thể chạy lại"})
				continue
			}
			if _, err = tx.Exec(ctx, `UPDATE storage_migration_items SET source_purged_at=clock_timestamp() WHERE migration_id=$1 AND object_key=$2 AND verified AND source_purged_at IS NULL`, id, result.Key); err != nil {
				return cursor, false, err
			}
			report.Deleted++
			report.DeletedBytes += sizes[result.Key]
		}
		if len(results) != len(keys) || deleteErr != nil {
			// Unknown outcomes are deliberately not stamped; DELETE is idempotent.
			if err = tx.Commit(ctx); err != nil {
				return cursor, false, err
			}
			return cursor, false, fmt.Errorf("lô xoá nguồn bị gián đoạn; chạy lại để tiếp tục: %w", errors.Join(deleteErr, errors.New("kết quả lô chưa đầy đủ")))
		}
	}
	return cursor, false, tx.Commit(ctx)
}

// Endpoints and credentials may both change while the backend stays the same.
// Without a provider instance identity, matching bucket names must refuse
// deletion regardless of signing region, even across possible endpoint aliases.
func samePurgeStore(a, b BlobConfig) bool {
	if SamePhysicalBlobStore(a, b) {
		return true
	}
	return a.Driver == BlobDriverS3 && b.Driver == BlobDriverS3 &&
		a.S3.Bucket == b.S3.Bucket
}

func (m *ManagedBlobs) openPurgeStore(ctx context.Context, id string, cfg BlobConfig) (blobstore.BlobStore, func(), error) {
	m.mu.Lock()
	cached := m.stores[id]
	m.mu.Unlock()
	if cached != nil {
		return cached, func() {}, nil
	}
	if cfg.Driver == BlobDriverLocal {
		local, err := blobstore.OpenLocalExisting(cfg.LocalRoot)
		if err != nil {
			return nil, nil, err
		}
		return local, func() { _ = local.Close() }, nil
	}
	store, _, err := m.store(ctx, id)
	return store, func() {}, err
}
