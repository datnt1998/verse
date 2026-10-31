package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/jackc/pgx/v5"
)

var _ serverstorage.Repository = (*BlobRepository)(nil)

func (s *BlobRepository) Usage(ctx context.Context) ([]serverstorage.UsageGroup, error) {
	var files, bytes int64
	err := s.pool.QueryRow(ctx, "SELECT count(*),coalesce(sum(b.bytes),0) FROM storage_live_refs live JOIN blob_refs r USING(logical_path) JOIN blobs b USING(object_key)").Scan(&files, &bytes)
	if err != nil {
		return nil, err
	}
	return []serverstorage.UsageGroup{{Group: "images"}, {Group: "editor"}, {Group: "library", Files: &files, Bytes: &bytes}, {Group: "sources"}, {Group: "exports"}}, nil
}
func (s *BlobRepository) PreviousTotals(ctx context.Context, id string) (int64, int64, error) {
	var n, b int64
	err := s.pool.QueryRow(ctx, "SELECT count(*),coalesce(sum(b.bytes),0) FROM blob_locations l JOIN blobs b USING(object_key) WHERE store_id=$1", id).Scan(&n, &b)
	return n, b, err
}
func (s *BlobRepository) LatestMigration(ctx context.Context) (*serverstorage.MigrationRecord, error) {
	var r serverstorage.MigrationRecord
	var raw []byte
	err := s.pool.QueryRow(ctx, "SELECT task_id,from_store,to_store,state,revision,journal_seq,started_at,finished_at,progress FROM storage_migrations ORDER BY started_at DESC LIMIT 1").Scan(&r.Migration.TaskID, &r.FromID, &r.ToID, &r.Migration.State, &r.Revision, &r.JournalSeq, &r.Migration.StartedAt, &r.Migration.FinishedAt, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var saved serverstorage.Migration
	if err = json.Unmarshal(raw, &saved); err != nil {
		return nil, err
	}
	saved.TaskID = r.Migration.TaskID
	saved.State = r.Migration.State
	saved.StartedAt = r.Migration.StartedAt
	saved.FinishedAt = r.Migration.FinishedAt
	r.Migration = saved
	if r.Migration.Groups == nil {
		r.Migration.Groups = []serverstorage.MigrationGroup{}
	}
	if r.Migration.Errors == nil {
		r.Migration.Errors = []serverstorage.MigrationError{}
	}
	return &r, nil
}
func (s *BlobRepository) CreateMigration(ctx context.Context, id string) (serverstorage.MigrationRecord, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return serverstorage.MigrationRecord{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(253025,2)"); err != nil {
		return serverstorage.MigrationRecord{}, err
	}
	state, err := readStorageState(ctx, tx)
	if err != nil {
		return serverstorage.MigrationRecord{}, err
	}
	if state.Pending == "" {
		return serverstorage.MigrationRecord{}, media.ErrConflict
	}
	var existing string
	err = tx.QueryRow(ctx, "SELECT task_id FROM storage_migrations WHERE state IN ('running','paused','error') LIMIT 1").Scan(&existing)
	if err == nil {
		return serverstorage.MigrationRecord{}, media.ErrBusy
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return serverstorage.MigrationRecord{}, err
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, "INSERT INTO storage_migrations(task_id,from_store,to_store,state,revision,started_at) VALUES($1,$2,$3,'running',$4,$5)", id, state.Active, state.Pending, state.Revision, now)
	if err != nil {
		return serverstorage.MigrationRecord{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return serverstorage.MigrationRecord{}, err
	}
	return serverstorage.MigrationRecord{FromID: state.Active, ToID: state.Pending, Revision: state.Revision, Migration: serverstorage.Migration{TaskID: id, State: "running", StartedAt: now, Groups: []serverstorage.MigrationGroup{}, Errors: []serverstorage.MigrationError{}}}, nil
}
func (s *BlobRepository) SetMigrationState(ctx context.Context, id, from, to string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, "UPDATE storage_migrations SET state=$3,finished_at=CASE WHEN $3 IN ('cancelled','done') THEN now() ELSE NULL END WHERE task_id=$1 AND state=$2", id, from, to)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return media.ErrConflict
	}
	if to == "running" {
		// Recovery re-verifies existing target bytes without re-PUT.
		if _, err = tx.Exec(ctx, "UPDATE storage_migration_items SET state='waiting',error_code=NULL,detail=NULL WHERE task_id=$1", id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *BlobRepository) ReconcileMigration(ctx context.Context, id string) ([]serverstorage.MigrationItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var source string
	if err = tx.QueryRow(ctx, "SELECT from_store FROM storage_migrations WHERE task_id=$1 AND state='running' FOR UPDATE", id).Scan(&source); err != nil {
		return nil, err
	}
	var missing bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_live_refs live LEFT JOIN blob_refs r USING(logical_path) LEFT JOIN blob_locations l ON l.object_key=r.object_key AND l.store_id=$1 WHERE r.object_key IS NULL OR l.verified_at IS NULL)", source).Scan(&missing); err != nil {
		return nil, err
	}
	if missing {
		return nil, media.ErrConflict
	}
	if _, err = tx.Exec(ctx, "INSERT INTO storage_migration_items(task_id,logical_path,object_key,usage_group,state) SELECT $1,live.logical_path,r.object_key,live.usage_group,'waiting' FROM storage_live_refs live JOIN blob_refs r USING(logical_path) ON CONFLICT DO NOTHING", id); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "UPDATE storage_migrations SET journal_seq=(SELECT coalesce(max(seq),0) FROM storage_journal) WHERE task_id=$1", id); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT i.logical_path,i.object_key,b.sha256,b.bytes,r.content_type,r.filename,coalesce(l.source_path,''),i.usage_group,i.state,coalesce(i.error_code,'') FROM storage_migration_items i JOIN storage_live_refs live USING(logical_path) JOIN blob_refs r ON r.logical_path=i.logical_path AND r.object_key=i.object_key JOIN blobs b ON b.object_key=i.object_key JOIN blob_locations l ON l.object_key=i.object_key AND l.store_id=$2 WHERE i.task_id=$1 ORDER BY i.logical_path", id, source)
	if err != nil {
		return nil, err
	}
	out := []serverstorage.MigrationItem{}
	for rows.Next() {
		var i serverstorage.MigrationItem
		i.Ref.StoreID = source
		if err = rows.Scan(&i.Ref.Path, &i.Ref.Key, &i.Ref.SHA256, &i.Ref.Bytes, &i.Ref.ContentType, &i.Ref.Filename, &i.Ref.SourcePath, &i.Group, &i.State, &i.ErrorCode); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
func (s *BlobRepository) VerifyMigrationItem(ctx context.Context, id string, ref media.Ref, code, detail string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var target, state string
	if err = tx.QueryRow(ctx, "SELECT to_store,state FROM storage_migrations WHERE task_id=$1 FOR UPDATE", id).Scan(&target, &state); err != nil {
		return err
	}
	if state != "running" {
		return media.ErrConflict
	}
	status := "verified"
	if code != "" {
		status = "error"
	} else {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,253025))", target+"/"+ref.Key); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO blob_locations(store_id,object_key,verified_at) VALUES($1,$2,now()) ON CONFLICT(store_id,object_key) DO UPDATE SET verified_at=excluded.verified_at,unreferenced_at=NULL", target, ref.Key); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, "UPDATE storage_migration_items SET state=$4,error_code=nullif($5,''),detail=nullif($6,''),verified_at=CASE WHEN $4='verified' THEN now() ELSE NULL END WHERE task_id=$1 AND logical_path=$2 AND object_key=$3", id, ref.Path, ref.Key, status, code, detail)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *BlobRepository) SaveMigrationProgress(ctx context.Context, r serverstorage.MigrationRecord) error {
	raw, err := json.Marshal(r.Migration)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, "UPDATE storage_migrations SET progress=$2 WHERE task_id=$1 AND state IN ('running','error','paused')", r.Migration.TaskID, raw)
	return err
}
func (s *BlobRepository) TryCutover(ctx context.Context, id string) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var locked bool
	if err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(253025,1)").Scan(&locked); err != nil || !locked {
		return false, err
	}
	var from, to, state string
	var revision int64
	if err = tx.QueryRow(ctx, "SELECT from_store,to_store,state,revision FROM storage_migrations WHERE task_id=$1 FOR UPDATE", id).Scan(&from, &to, &state, &revision); err != nil {
		return false, err
	}
	if state != "running" {
		return false, nil
	}
	active, err := readStorageState(ctx, tx)
	if err != nil {
		return false, err
	}
	if active.Active != from || active.Pending != to || active.Revision != revision {
		return false, media.ErrConflict
	}
	var pending bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_live_refs live LEFT JOIN blob_refs r USING(logical_path) LEFT JOIN blob_locations l ON l.object_key=r.object_key AND l.store_id=$2 LEFT JOIN storage_migration_items i ON i.task_id=$1 AND i.logical_path=live.logical_path AND i.object_key=r.object_key WHERE r.object_key IS NULL OR l.verified_at IS NULL OR i.state IS DISTINCT FROM 'verified')", id, to).Scan(&pending)
	if err != nil || pending {
		return false, err
	}
	if _, err = tx.Exec(ctx, "UPDATE storage_settings SET previous_store=active_store,active_store=pending_store,pending_store=NULL,generation=generation+1,revision=revision+1 WHERE singleton"); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, "UPDATE storage_migrations SET state='done',finished_at=now(),journal_seq=(SELECT coalesce(max(seq),0) FROM storage_journal) WHERE task_id=$1", id); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
func (s *BlobRepository) SaveHealth(ctx context.Context, h serverstorage.Health) error {
	_, err := s.pool.Exec(ctx, "INSERT INTO storage_health(singleton,store_id,online,checked_at,since,error_code) VALUES(true,$1,$2,$3,$4,nullif($5,'')) ON CONFLICT(singleton) DO UPDATE SET store_id=excluded.store_id,online=excluded.online,checked_at=excluded.checked_at,since=excluded.since,error_code=excluded.error_code", h.StoreID, h.Online, h.CheckedAt, h.Since, h.ErrorCode)
	return err
}
func (s *BlobRepository) Health(ctx context.Context) (*serverstorage.Health, error) {
	var h serverstorage.Health
	err := s.pool.QueryRow(ctx, "SELECT store_id,online,checked_at,since,coalesce(error_code,'') FROM storage_health WHERE singleton").Scan(&h.StoreID, &h.Online, &h.CheckedAt, &h.Since, &h.ErrorCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &h, err
}
