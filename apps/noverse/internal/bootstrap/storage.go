package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// StoreDriver selects the fact-storage backend for the process. Selection is
// explicit per run: CLI/offline defaults to file; hosted serve must declare
// postgres — there is no silent fallback either direction.
type StoreDriver string

const (
	StoreDriverFile     StoreDriver = "file"
	StoreDriverPostgres StoreDriver = "postgres"
)

// StorageConfig is the validated result of the storage env contract. Fields
// are parsed once at startup; bad input is a startup error, never a guess.
type StorageConfig struct {
	Driver       StoreDriver
	DatabaseURL  string // app-role DSN (never printed)
	MigrationURL string // migration-role DSN (never printed)
	// WorkerURL is the dedicated DSN for hosted engine-worker child processes
	// (NOVERSE_WORKER_DATABASE_URL): a role scoped to facts/lease tables only.
	// The parent hands it to the child as DATABASE_URL so the child contract
	// stays single-keyed; the child's env never carries the app DSN.
	WorkerURL      string        // engine-worker DSN (never printed)
	MaxConns       int32         // DB_MAX_CONNS, bounded for one VPS
	ConnectTimeout time.Duration // DB_CONNECT_TIMEOUT
}

// MigrationDSN returns the migration-role DSN, falling back to the app DSN
// when no dedicated role is configured (dev/test convenience only).
func (c StorageConfig) MigrationDSN() string {
	if c.MigrationURL != "" {
		return c.MigrationURL
	}
	return c.DatabaseURL
}

// SafeSummary renders a credential-free one-line description for logs.
func (c StorageConfig) SafeSummary() string {
	if c.Driver == StoreDriverPostgres {
		return fmt.Sprintf("driver=postgres max_conns=%d connect_timeout=%s dsn=[configured]",
			c.MaxConns, c.ConnectTimeout)
	}
	return "driver=file"
}

// Pool bounds are part of the env contract: a value outside them is a startup
// error, never silently clamped. One small VPS cannot use more than
// MaxPoolConns connections, and a connect timeout above MaxConnectTimeout is a
// typo, not a policy.
const (
	MinPoolConns      int32 = 1
	MaxPoolConns      int32 = 100
	MinConnectTimeout       = time.Second
	MaxConnectTimeout       = 2 * time.Minute
)

// StorageConfigFromEnv parses the storage environment strictly:
//
//	STORE_DRIVER                file|postgres (empty = file; anything else is an error)
//	DATABASE_URL                required when driver=postgres
//	MIGRATION_DATABASE_URL      optional migration-role DSN; falls back to DATABASE_URL
//	NOVERSE_WORKER_DATABASE_URL engine-worker-role DSN for hosted worker
//	                            children; postgres-only, validated when set.
//	                            Worker launch fails closed when it is absent —
//	                            never falls back to the app DSN.
//	DB_MAX_CONNS                optional pool bound, integer 1..100 (default 8)
//	DB_CONNECT_TIMEOUT          optional duration 1s..2m (e.g. 10s, default 10s)
//
// Precedence (one documented contract, see README):
//   - Runtime infrastructure comes from this validated env only.
//   - An explicit CLI flag beats its own env (serve --addr > HTTP_ADDR;
//     worker tts --api > TTS_WORKER_API_URL).
//   - Maintenance commands (storage migrate/import/reconcile/export-files)
//     prefer MIGRATION_DATABASE_URL over DATABASE_URL — explicitly, not as a
//     silent fallback for the serving process.
//   - `storage copy` reads self-contained 0600 config files and ignores this
//     ambient env.
//
// A database URL without STORE_DRIVER=postgres stays inert — file mode never
// silently activates postgres, and postgres never falls back to file.
func StorageConfigFromEnv() (StorageConfig, error) {
	if err := ValidateConfigEncryptionKey(); err != nil {
		return StorageConfig{}, err
	}
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }

	cfg := StorageConfig{Driver: StoreDriverFile}
	switch d := get("STORE_DRIVER"); d {
	case "", "file":
		cfg.Driver = StoreDriverFile
	case "postgres":
		cfg.Driver = StoreDriverPostgres
	default:
		return cfg, fmt.Errorf("STORE_DRIVER=%q không hợp lệ (file|postgres): %w", d, errs.ErrConfig)
	}

	cfg.DatabaseURL = get("DATABASE_URL")
	cfg.MigrationURL = get("MIGRATION_DATABASE_URL")
	cfg.WorkerURL = get("NOVERSE_WORKER_DATABASE_URL")

	if cfg.Driver == StoreDriverPostgres {
		if cfg.DatabaseURL == "" {
			return cfg, fmt.Errorf("STORE_DRIVER=postgres yêu cầu DATABASE_URL (không tự quay về file): %w", errs.ErrConfig)
		}
		if err := validateDSN(cfg.DatabaseURL); err != nil {
			return cfg, err
		}
		if cfg.MigrationURL != "" {
			if err := validateDSN(cfg.MigrationURL); err != nil {
				return cfg, err
			}
		}
		if cfg.WorkerURL != "" {
			if err := validateDSN(cfg.WorkerURL); err != nil {
				return cfg, err
			}
		}
	} else if cfg.WorkerURL != "" {
		return cfg, fmt.Errorf("NOVERSE_WORKER_DATABASE_URL chỉ có nghĩa khi STORE_DRIVER=postgres: %w", errs.ErrConfig)
	}

	if v := get("DB_MAX_CONNS"); v != "" {
		// Atoi rejects the whole string; "8 conns"/"8x" never parse.
		n, err := strconv.Atoi(v)
		if err != nil || n < int(MinPoolConns) || n > int(MaxPoolConns) {
			return cfg, fmt.Errorf("DB_MAX_CONNS=%q không hợp lệ (số nguyên %d..%d): %w", v, MinPoolConns, MaxPoolConns, errs.ErrConfig)
		}
		cfg.MaxConns = int32(n)
	}
	if v := get("DB_CONNECT_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < MinConnectTimeout || d > MaxConnectTimeout {
			return cfg, fmt.Errorf("DB_CONNECT_TIMEOUT=%q không hợp lệ (%s..%s, ví dụ 10s): %w", v, MinConnectTimeout, MaxConnectTimeout, errs.ErrConfig)
		}
		cfg.ConnectTimeout = d
	}
	return cfg, nil
}

// validateDSN checks shape only — the string is never echoed back in errors,
// so a malformed secret cannot leak into logs.
func validateDSN(dsn string) error {
	if !strings.Contains(dsn, "://") && !strings.Contains(dsn, "host=") {
		return fmt.Errorf("DSN Postgres không đúng dạng kết nối: %w", errs.ErrConfig)
	}
	return nil
}

// OpenPool opens the application-role pgx pool for a postgres config: bounded,
// pinged, schema-compatibility checked. The caller owns Close; the pool is
// process-scoped and must never carry request actor state.
func OpenPool(ctx context.Context, cfg StorageConfig) (*postgres.Pool, error) {
	if cfg.Driver != StoreDriverPostgres {
		return nil, fmt.Errorf("OpenPool chỉ dành cho STORE_DRIVER=postgres (đang là %q): %w", cfg.Driver, errs.ErrConfig)
	}
	return postgres.Open(ctx, postgres.PoolConfig{
		DSN:            cfg.DatabaseURL,
		MaxConns:       cfg.MaxConns,
		ConnectTimeout: cfg.ConnectTimeout,
	})
}

// BlobDriver selects the binary-object backend. It is independent of
// StoreDriver: file facts can still use S3 blobs and vice versa.
type BlobDriver string

const (
	BlobDriverLocal BlobDriver = "local"
	BlobDriverS3    BlobDriver = "s3"
)

// BlobConfig is the validated blob environment. Secrets stay in memory and
// are never rendered into logs or errors.
type BlobConfig struct {
	Driver   BlobDriver
	Delivery string
	Provider string
	// local driver
	LocalRoot string
	// s3 driver — verbatim recipe fields (see research provider matrix)
	S3           blobstore.S3Config
	insecureHTTP bool // S3_ALLOW_INSECURE_ENDPOINT parsed flag
}

// SafeSummary renders a credential-free one-line description.
func (c BlobConfig) SafeSummary() string {
	if c.Driver == BlobDriverS3 {
		ep := "aws-default"
		if c.S3.Endpoint != "" {
			ep = "invalid-endpoint"
			if u, err := url.Parse(c.S3.Endpoint); err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") {
				ep = u.Scheme + "://" + u.Host
			}
		}
		return fmt.Sprintf("driver=s3 bucket=%s region=%s endpoint=%s path_style=%t",
			c.S3.Bucket, c.S3.Region, ep, c.S3.ForcePathStyle)
	}
	return fmt.Sprintf("driver=local root=%s", c.LocalRoot)
}

// BlobConfigFromEnv parses the blob environment strictly:
//
//	STORAGE_DRIVER             local|s3 (empty = local; requires BLOB_LOCAL_ROOT)
//	BLOB_LOCAL_ROOT            absolute root for the local driver
//	S3_ENDPOINT                https base endpoint; empty = AWS resolver;
//	                           http only with S3_ALLOW_INSECURE_ENDPOINT=true
//	S3_REGION                  required (auto for R2, us-east-1 for GCS/MinIO)
//	S3_BUCKET                  required
//	S3_ACCESS_KEY_ID           required (GCS: HMAC access id)
//	S3_SECRET_ACCESS_KEY       required (GCS: HMAC secret)
//	S3_FORCE_PATH_STYLE        true|false strict (required by MinIO/GCS)
//	S3_ALLOW_INSECURE_ENDPOINT true|false strict; only enables http:// endpoints
//	                           for private/test networks (MinIO). Never for AWS.
//
// Every recipe is the same driver; no provider-specific config shape exists.
func BlobConfigFromEnv() (BlobConfig, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	cfg := BlobConfig{Driver: BlobDriverLocal}
	switch d := get("STORAGE_DRIVER"); d {
	case "", "local":
		cfg.Driver = BlobDriverLocal
	case "s3":
		cfg.Driver = BlobDriverS3
	default:
		return cfg, fmt.Errorf("STORAGE_DRIVER=%q không hợp lệ (local|s3): %w", d, errs.ErrConfig)
	}

	if insecure := get("S3_ALLOW_INSECURE_ENDPOINT"); insecure != "" {
		b, err := strictBool("S3_ALLOW_INSECURE_ENDPOINT", insecure)
		if err != nil {
			return cfg, err
		}
		cfg.insecureHTTP = b
	}

	cfg.Delivery = get("BLOB_DELIVERY")
	if cfg.Delivery != "" && cfg.Delivery != "redirect" && cfg.Delivery != "proxy" {
		return cfg, fmt.Errorf("BLOB_DELIVERY phải là redirect hoặc proxy: %w", errs.ErrConfig)
	}
	cfg.Provider = get("S3_PROVIDER")
	if cfg.Driver == BlobDriverLocal {
		cfg.LocalRoot = get("BLOB_LOCAL_ROOT")
		if cfg.LocalRoot == "" {
			return cfg, fmt.Errorf("STORAGE_DRIVER=local yêu cầu BLOB_LOCAL_ROOT: %w", errs.ErrConfig)
		}
		return cfg, nil
	}

	s3cfg := blobstore.S3Config{
		Endpoint:        get("S3_ENDPOINT"),
		PresignEndpoint: get("S3_PRESIGN_ENDPOINT"),
		Region:          get("S3_REGION"),
		Bucket:          get("S3_BUCKET"),
		AccessKeyID:     get("S3_ACCESS_KEY_ID"),
		SecretAccessKey: get("S3_SECRET_ACCESS_KEY"),
		AllowPlainHTTP:  cfg.insecureHTTP,
	}
	if v := get("S3_FORCE_PATH_STYLE"); v != "" {
		b, err := strictBool("S3_FORCE_PATH_STYLE", v)
		if err != nil {
			return cfg, err
		}
		s3cfg.ForcePathStyle = b
	}
	if cfg.Provider == "garage" {
		if s3cfg.Region == "" {
			s3cfg.Region = "garage"
		}
		s3cfg.ForcePathStyle = true
	}
	if s3cfg.Region == "" || s3cfg.Bucket == "" || s3cfg.AccessKeyID == "" || s3cfg.SecretAccessKey == "" {
		return cfg, fmt.Errorf("STORAGE_DRIVER=s3 yêu cầu S3_REGION, S3_BUCKET, S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY: %w", errs.ErrConfig)
	}
	cfg.S3 = s3cfg
	return cfg, nil
}

// BlobConfigFromFile loads the same contract from a JSON config file for
// `storage copy`. The file must be mode 0600 and lives outside the repo —
// credentials never travel through argv.
func BlobConfigFromFile(path string) (BlobConfig, error) {
	var cfg BlobConfig
	info, err := os.Stat(path)
	if err != nil {
		return cfg, fmt.Errorf("đọc config blob: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return cfg, fmt.Errorf("config blob %s phải có quyền 0600 (chứa credentials): %w", path, errs.ErrConfig)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("đọc config blob: %w", err)
	}
	var raw struct {
		Driver                string `json:"driver"`
		LocalRoot             string `json:"localRoot"`
		Endpoint              string `json:"s3Endpoint"`
		Region                string `json:"s3Region"`
		Bucket                string `json:"s3Bucket"`
		AccessKeyID           string `json:"s3AccessKeyId"`
		SecretAccessKey       string `json:"s3SecretAccessKey"`
		ForcePathStyle        bool   `json:"s3ForcePathStyle"`
		AllowInsecureEndpoint bool   `json:"s3AllowInsecureEndpoint"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, fmt.Errorf("config blob không phải JSON hợp lệ: %w", errs.ErrConfig)
	}
	switch raw.Driver {
	case "local":
		cfg.Driver = BlobDriverLocal
		cfg.LocalRoot = raw.LocalRoot
		if cfg.LocalRoot == "" {
			return cfg, fmt.Errorf("driver local yêu cầu localRoot: %w", errs.ErrConfig)
		}
	case "s3":
		cfg.Driver = BlobDriverS3
		cfg.S3 = blobstore.S3Config{
			Endpoint:        raw.Endpoint,
			Region:          raw.Region,
			Bucket:          raw.Bucket,
			AccessKeyID:     raw.AccessKeyID,
			SecretAccessKey: raw.SecretAccessKey,
			ForcePathStyle:  raw.ForcePathStyle,
			AllowPlainHTTP:  raw.AllowInsecureEndpoint,
		}
	default:
		return cfg, fmt.Errorf("driver=%q không hợp lệ (local|s3): %w", raw.Driver, errs.ErrConfig)
	}
	return cfg, nil
}

// OpenBlobStore is the composition root for blobs. Bad config is a startup
// error; there is no fallback driver.
func OpenBlobStore(cfg BlobConfig) (blobstore.BlobStore, error) {
	var st blobstore.BlobStore
	var err error
	switch cfg.Driver {
	case BlobDriverLocal:
		st, err = blobstore.NewLocal(cfg.LocalRoot)
	case BlobDriverS3:
		st, err = blobstore.NewS3(cfg.S3)
	default:
		return nil, fmt.Errorf("STORAGE_DRIVER=%q không được hỗ trợ: %w", cfg.Driver, errs.ErrConfig)
	}
	if err != nil {
		return nil, err // never leak a typed-nil driver as a non-nil interface
	}
	return st, nil
}

// CheckBlobStore is the readiness probe for the configured blob backend. It
// proves reachability without writing user-visible objects:
//
//	local → the root must exist and accept a temp file (read-only volume or a
//	        missing mount is exactly the failure readiness must catch)
//	s3    → one bounded List call with the caller's deadline
//
// It is called through a cached, timeout-bounded readiness check — never per
// request, and never as an upload probe against a remote bucket.
func CheckBlobStore(ctx context.Context, st blobstore.BlobStore, cfg BlobConfig) error {
	if st == nil {
		return fmt.Errorf("blob store chưa mở: %w", errs.ErrConfig)
	}
	switch cfg.Driver {
	case BlobDriverS3:
		if _, err := st.List(ctx, "", "", 1); err != nil {
			return fmt.Errorf("blob s3 không phản hồi: %w", err)
		}
		return nil
	case BlobDriverLocal:
		dir := cfg.LocalRoot
		if dir == "" {
			return fmt.Errorf("BLOB_LOCAL_ROOT trống: %w", errs.ErrConfig)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("blob local không tạo được thư mục: %w", err)
		}
		f, err := os.CreateTemp(dir, ".readyz-*")
		if err != nil {
			return fmt.Errorf("blob local không ghi được: %w", err)
		}
		name := f.Name()
		closeErr := f.Close()
		removeErr := os.Remove(name)
		if closeErr != nil {
			return fmt.Errorf("blob local không ghi được: %w", closeErr)
		}
		if removeErr != nil {
			return fmt.Errorf("blob local không xoá được tệp thử: %w", removeErr)
		}
		return nil
	}
	return fmt.Errorf("STORAGE_DRIVER=%q không được hỗ trợ: %w", cfg.Driver, errs.ErrConfig)
}

// strictBool parses the locked boolean contract: only "true"/"false"/"1"/"0"
// — never "yes"/"on"/"", so a typo fails closed instead of silently changing
// endpoint style.
func strictBool(name, v string) (bool, error) {
	switch v {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%s=%q không hợp lệ (true|false): %w", name, v, errs.ErrConfig)
	}
}

// OpenStoryStore is the composition root for a scoped postgres story store.
// It resolves (project, story) beside the actor's membership inside the
// repository, then returns the complete facade — or the capability error if a
// domain is still missing. There is no directory fallback on this path.
func OpenStoryStore(ctx context.Context, pool *postgres.Pool, projectSlug, storySlug string, actor *postgres.Actor) (*store.Store, error) {
	if projectSlug == "" || storySlug == "" {
		return nil, fmt.Errorf("postgres store cần project và story: %w", errs.ErrConfig)
	}
	// Đường hosted bắt buộc actor thật: nil actor tắt kiểm membership trong
	// resolve, nên request không có danh tính phải bị từ chối thay vì đi qua.
	if actor == nil {
		return nil, fmt.Errorf("postgres store cần actor của request (membership check): %w", errs.ErrStorageForbidden)
	}
	nar := postgres.NewNarrative(pool)
	scope, err := nar.Resolve(ctx, projectSlug, storySlug, actor, false)
	if err != nil {
		return nil, err
	}
	return store.NewFromBackend(postgres.NewBackend(ctx, pool, nar, scope))
}

// OpenStoryStoreInternal opens a scoped story store for process-internal
// paths (worker drain, engine child) that have no request account: the
// repository resolves scope without membership, and exclusivity is the story
// lease, not an account. Request-surface calls must keep using
// OpenStoryStore with the caller's actor.
func OpenStoryStoreInternal(ctx context.Context, pool *postgres.Pool, projectSlug, storySlug string) (*store.Store, error) {
	if projectSlug == "" || storySlug == "" {
		return nil, fmt.Errorf("postgres store cần project và story: %w", errs.ErrConfig)
	}
	nar := postgres.NewNarrative(pool)
	scope, err := nar.Resolve(ctx, projectSlug, storySlug, nil, false)
	if err != nil {
		return nil, err
	}
	return store.NewFromBackend(postgres.NewBackend(ctx, pool, nar, scope))
}
