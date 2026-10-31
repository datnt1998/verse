package bootstrap

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/jackc/pgx/v5"
)

type StorageUsage struct {
	Count int64 `json:"count"`
	Bytes int64 `json:"bytes"`
}
type StorageSaveResult struct {
	Version        int64             `json:"version"`
	NeedsMigration bool              `json:"needsMigration"`
	Count          int64             `json:"count"`
	Bytes          int64             `json:"bytes"`
	Test           StorageTestResult `json:"test"`
}

type StorageActive struct {
	Driver          string            `json:"driver"`
	Provider        string            `json:"provider"`
	EndpointHost    string            `json:"endpointHost"`
	EndpointScheme  string            `json:"endpointScheme"`
	PresignEndpoint string            `json:"presignEndpoint"`
	Region          string            `json:"region"`
	Bucket          string            `json:"bucket"`
	PathStyle       bool              `json:"pathStyle"`
	AllowHTTP       bool              `json:"allowHTTP"`
	Delivery        string            `json:"delivery"`
	Env             string            `json:"env"`
	Source          map[string]string `json:"source"`
	AccessKeyLast4  string            `json:"accessKeyLast4"`
}

func (m *ManagedBlobs) Usage(ctx context.Context) (map[string]StorageUsage, StorageUsage, error) {
	out := map[string]StorageUsage{}
	for _, kind := range []string{"audio", "export", "cover", "music", "voiceSample", "import"} {
		out[kind] = StorageUsage{}
	}
	rows, err := m.pool.Pool().Query(ctx, `SELECT CASE WHEN usage_kind='music' AND EXISTS(SELECT FROM audio_voices v WHERE v.sample_blob_id=b.id) THEN 'voiceSample' ELSE usage_kind END, count(*),COALESCE(sum(size_bytes),0)::bigint FROM blobs b WHERE state='ready' AND usage_kind<>'' AND object_key LIKE $1 GROUP BY 1`, m.keys.Prefix()+"%")
	if err != nil {
		return nil, StorageUsage{}, err
	}
	defer rows.Close()
	var total StorageUsage
	for rows.Next() {
		var kind string
		var u StorageUsage
		if err := rows.Scan(&kind, &u.Count, &u.Bytes); err != nil {
			return nil, total, err
		}
		if _, known := out[kind]; known {
			out[kind] = u
		}
		total.Count += u.Count
		total.Bytes += u.Bytes
	}
	return out, total, rows.Err()
}

func (m *ManagedBlobs) View(ctx context.Context) (map[string]any, error) {
	var version int64
	var savedID, pendingID *string
	if err := m.pool.Pool().QueryRow(ctx, `SELECT active_store,pending_store,version FROM storage_settings WHERE singleton`).Scan(&savedID, &pendingID, &version); err != nil {
		return nil, err
	}
	id := ""
	if !m.locked && savedID != nil {
		id = *savedID
	}
	st, cfg, err := m.store(ctx, id)
	if err != nil {
		return nil, err
	}
	source := map[string]string{}
	value := "app"
	if m.locked {
		value = "env"
	}
	for _, field := range []string{"driver", "provider", "endpoint", "presignEndpoint", "region", "bucket", "pathStyle", "allowHTTP", "delivery", "env", "accessKeyId", "secretAccessKey", "localRoot"} {
		source[field] = value
	}
	host, scheme := "", ""
	if u, e := url.Parse(cfg.S3.Endpoint); e == nil {
		host, scheme = u.Host, u.Scheme
	}
	last := cfg.S3.AccessKeyID
	if len(last) > 4 {
		last = last[len(last)-4:]
	}
	active := StorageActive{Driver: string(cfg.Driver), Provider: cfg.Provider, EndpointHost: host, EndpointScheme: scheme, PresignEndpoint: cfg.S3.PresignEndpoint, Region: cfg.S3.Region, Bucket: cfg.S3.Bucket, PathStyle: cfg.S3.ForcePathStyle, AllowHTTP: cfg.S3.AllowPlainHTTP, Delivery: cfg.Delivery, Env: string(m.keys.Env()), Source: source, AccessKeyLast4: last}
	usage, total, err := m.Usage(ctx)
	if err != nil {
		return nil, err
	}
	migration, err := m.MigrationStatus(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	at, healthErr := m.healthAt, m.healthError
	m.mu.Unlock()
	var lastCheck *time.Time
	if !at.IsZero() {
		lastCheck = &at
	}
	conditional := "not_applicable"
	if source, ok := st.(interface{ ConditionalPutSupport() string }); ok {
		conditional = source.ConditionalPutSupport()
	}
	var pending any
	if pendingID != nil {
		cfg, err := m.loadConfig(ctx, *pendingID)
		if err != nil {
			return nil, err
		}
		host := ""
		if u, err := url.Parse(cfg.S3.Endpoint); err == nil {
			host = u.Host
		}
		pending = map[string]string{"provider": cfg.Provider, "bucket": cfg.S3.Bucket, "endpointHost": host}
	}
	return map[string]any{"mode": "postgres", "version": version, "locked": m.locked, "active": active, "pending": pending, "encryption": map[string]bool{"available": m.cipher.aead != nil}, "usage": usage, "total": total, "operations": m.Snapshot(), "migration": migration, "health": map[string]any{"lastCheck": lastCheck, "error": healthErr}, "capabilities": map[string]string{"conditionalPut": conditional}}, nil
}

// CheckSaveVersion rejects stale requests before spending any network probe
// quota. Save still compares again inside the exclusive activation fence.
func (m *ManagedBlobs) CheckSaveVersion(ctx context.Context, expected int64) error {
	if m.locked {
		return ErrStorageLocked
	}
	var current int64
	if err := m.pool.Pool().QueryRow(ctx, `SELECT version FROM storage_settings WHERE singleton`).Scan(&current); err != nil {
		return err
	}
	if current != expected {
		return ErrStorageConflict
	}
	return nil
}

func (m *ManagedBlobs) Save(ctx context.Context, c StorageCandidate) (StorageSaveResult, error) {
	result := StorageSaveResult{}
	if err := m.CheckSaveVersion(ctx, c.Version); err != nil {
		return result, err
	}
	if (c.Secret != nil || c.AccessKeyID != nil) && m.cipher.aead == nil {
		return result, ErrStorageEncryption
	}
	cfg, err := m.Candidate(ctx, c)
	if err != nil {
		return result, err
	}
	result.Test, err = m.check(ctx, cfg, false)
	if err != nil {
		return result, err
	}
	if !result.Test.OK {
		return result, errors.New("Kho tệp chưa vượt qua kiểm tra kết nối")
	}
	tx, err := m.pool.Pool().Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71427014)`); err != nil {
		return result, err
	}
	var version int64
	var active string
	if err = tx.QueryRow(ctx, `SELECT version,active_store FROM storage_settings WHERE singleton FOR UPDATE`).Scan(&version, &active); err != nil {
		return result, err
	}
	if version != c.Version {
		return result, ErrStorageConflict
	}
	var busy bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM storage_migrations WHERE state IN ('running','paused','error'))`).Scan(&busy); err != nil {
		return result, err
	}
	if busy {
		return result, errors.New("Đang chuyển kho; hãy hoàn thành hoặc huỷ trước khi đổi cấu hình")
	}
	if err = tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(size_bytes),0)::bigint FROM blobs WHERE state='ready' AND store_id=$1`, active).Scan(&result.Count, &result.Bytes); err != nil {
		return result, err
	}
	id, err := m.saveConfig(ctx, tx, cfg)
	if err != nil {
		return result, err
	}
	oldCfg, err := m.loadConfigFrom(ctx, tx, active)
	if err != nil {
		return result, err
	}
	same := SamePhysicalBlobStore(oldCfg, cfg)
	result.NeedsMigration = result.Count > 0 && !same
	if same {
		if _, err = tx.Exec(ctx, `UPDATE blobs SET store_id=$1,version=version+1,updated_at=clock_timestamp() WHERE store_id=$2`, id, active); err != nil {
			return result, err
		}
	}
	result.Version = version + 1
	if result.NeedsMigration {
		_, err = tx.Exec(ctx, `UPDATE storage_settings SET pending_store=$1,version=version+1 WHERE singleton`, id)
	} else {
		_, err = tx.Exec(ctx, `UPDATE storage_settings SET active_store=$1,pending_store=NULL,version=version+1 WHERE singleton`, id)
	}
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	if !result.NeedsMigration {
		m.recordStorageCheck(result.Test)
	}
	return result, nil
}

// MigrationError is deliberately bounded to kind/story and a translated code.
type MigrationError struct {
	Kind       string `json:"kind"`
	Story      string `json:"story"`
	StoryTitle string `json:"storyTitle"`
	Code       string `json:"code"`
	Reason     string `json:"reason"`
}
type StorageMigrationStatus struct {
	ID          string           `json:"id"`
	State       string           `json:"state"`
	ErrorCode   string           `json:"errorCode,omitempty"`
	CopiedCount int64            `json:"copiedCount"`
	CopiedBytes int64            `json:"copiedBytes"`
	TotalCount  int64            `json:"totalCount"`
	TotalBytes  int64            `json:"totalBytes"`
	Rate        float64          `json:"bytesPerSecond"`
	ErrorCount  int64            `json:"errorCount"`
	Errors      []MigrationError `json:"errors"`
	StartedAt   time.Time        `json:"startedAt"`
	CompletedAt *time.Time       `json:"completedAt"`
	PurgedAt    *time.Time       `json:"purgedAt"`
}

func (m *ManagedBlobs) MigrationStatus(ctx context.Context) (*StorageMigrationStatus, error) {
	status := &StorageMigrationStatus{Errors: []MigrationError{}}
	err := m.pool.Pool().QueryRow(ctx, `SELECT id,state,error_code,copied_count,copied_bytes,total_count,total_bytes,started_at,completed_at,purged_at FROM storage_migrations ORDER BY started_at DESC LIMIT 1`).Scan(&status.ID, &status.State, &status.ErrorCode, &status.CopiedCount, &status.CopiedBytes, &status.TotalCount, &status.TotalBytes, &status.StartedAt, &status.CompletedAt, &status.PurgedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(status.StartedAt).Seconds()
	if elapsed > 0 {
		status.Rate = float64(status.CopiedBytes) / elapsed
	}
	if err = m.pool.Pool().QueryRow(ctx, `SELECT count(*) FROM storage_migration_items i JOIN blobs b USING(object_key) WHERE migration_id=$1 AND error_code<>'' AND b.state='ready'`, status.ID).Scan(&status.ErrorCount); err != nil {
		return nil, err
	}
	rows, err := m.pool.Pool().Query(ctx, `SELECT i.object_key,i.error_code,CASE WHEN b.usage_kind='music' AND EXISTS(SELECT FROM audio_voices v WHERE v.sample_blob_id=b.id) THEN 'voiceSample' ELSE b.usage_kind END,b.content_type FROM storage_migration_items i JOIN blobs b USING(object_key) WHERE migration_id=$1 AND error_code<>'' AND b.state='ready' ORDER BY object_key LIMIT 100`, status.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []string{}
	for rows.Next() {
		var key, code, kind, contentType string
		if err = rows.Scan(&key, &code, &kind, &contentType); err != nil {
			return nil, err
		}
		object, _ := blobkey.Parse(key)
		if kind == "unclassified" || kind == "other" || kind == "" {
			kind = blobkey.UsageKind(key, contentType)
			if kind == "" || kind == "other" {
				kind = string(object.Kind)
			}
		}
		reason := "Chưa chép và xác minh được tệp; hãy kiểm tra kết nối rồi thử lại"
		if code == "source_missing" {
			reason = "Không tìm thấy tệp ở kho nguồn; hãy khôi phục tệp hoặc đối soát trước khi thử lại"
		}
		status.Errors = append(status.Errors, MigrationError{Kind: kind, Story: object.StoryID, Code: code, Reason: reason})
		projects = append(projects, object.ProjectID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i, item := range status.Errors {
		if item.Story == "" {
			continue
		}
		var title string
		if err := m.pool.Pool().QueryRow(ctx, `SELECT title FROM stories WHERE project_id=$1::uuid AND id=$2::uuid`, projects[i], item.Story).Scan(&title); err == nil {
			status.Errors[i].StoryTitle = title
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	return status, nil
}
