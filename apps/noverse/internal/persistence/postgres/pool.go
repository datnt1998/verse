// Package postgres holds the Postgres storage plumbing: the shared pgx
// pool, the transaction helper and the CAS primitive domain repositories
// build on. Repository code lives alongside in later phases; nothing here
// pretends a database is a filesystem.
//
// The pool is process-scoped and never carries actor/session state — actors
// are request-scoped parameters to repositories. Blob publication retains one
// connection across upload to hold its activation fence through registry commit;
// it must reuse that connection, never acquire another while holding the fence.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrations"
)

// SupportedSchemaFormat is the schema.format value this binary was built for.
// CheckSchema refuses a database whose marker differs — the readiness hook
// phase 09 wires to deployment health.
const SupportedSchemaFormat = "1"

// PoolConfig bounds the native pgx pool. Zero values get sane defaults sized
// for a single-VPS deployment; all bounds are explicit in StorageConfig.
type PoolConfig struct {
	DSN             string
	MaxConns        int32
	MaxConnIdleTime time.Duration
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	// SkipSchemaCheck is for tests that provision an unmigrated schema on
	// purpose; production Open always validates.
	SkipSchemaCheck bool
}

func (c PoolConfig) withDefaults() PoolConfig {
	if c.MaxConns <= 0 {
		c.MaxConns = 8
	}
	if c.MaxConnIdleTime <= 0 {
		c.MaxConnIdleTime = 5 * time.Minute
	}
	if c.MaxConnLifetime <= 0 {
		c.MaxConnLifetime = 30 * time.Minute
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	return c
}

// Pool wraps *pgxpool.Pool with schema compatibility validation.
type Pool struct {
	pool *pgxpool.Pool
}

// Open creates the pool, pings the database within ConnectTimeout and checks
// schema compatibility. A database that fails ping or carries an incompatible
// schema.format is an error at startup — no lazy fallback, no file mode.
func Open(ctx context.Context, cfg PoolConfig) (*Pool, error) {
	cfg = cfg.withDefaults()
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		// pgx parse errors may echo the raw DSN (credentials included);
		// never forward them.
		return nil, errors.New("invalid DATABASE_URL: cannot parse connection string")
	}
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	pcfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	// UTC is the one boundary for instants: the session timezone is UTC (so
	// server-side casts/text agree) and timestamptz decodes in UTC, which makes
	// every serialized timestamp an RFC3339 "Z" value instead of whatever
	// offset the process/host happens to use. Instant identity is unchanged;
	// this is presentation, not storage.
	if pcfg.ConnConfig.RuntimeParams == nil {
		pcfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	pcfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pcfg.AfterConnect = scanTimestampsInUTC

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(pingCtx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("postgres pool init: %w", sanitizeErr(err))
	}
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping failed: %w", sanitizeErr(err))
	}
	p := &Pool{pool: pool}
	if !cfg.SkipSchemaCheck {
		if err := p.CheckSchema(ctx); err != nil {
			pool.Close()
			return nil, err
		}
		if err := p.CheckMigrations(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return p, nil
}

// scanTimestampsInUTC installs the UTC decode location for timestamptz on a
// fresh connection. pgx otherwise returns the host's local location, which
// leaks into JSON/RFC3339 output and operator listings. It registers a new
// type on this connection's map: the *Type that TypeForOID returns may come
// from pgx's process-wide default map, and mutating it races every other
// connection that is scanning.
func scanTimestampsInUTC(_ context.Context, conn *pgx.Conn) error {
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name:  "timestamptz",
		OID:   pgtype.TimestamptzOID,
		Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
	})
	return nil
}

// Pool exposes the native pgxpool for repositories and transactions.
func (p *Pool) Pool() *pgxpool.Pool { return p.pool }

// Close releases the pool; idempotent.
func (p *Pool) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
}

// CheckSchema validates the foundation marker written by migration 00001:
// the schema must be migrated to a supported format before the app serves.
// Format compatibility is deliberately distinct from migration completeness;
// CheckMigrations covers the latter.
func (p *Pool) CheckSchema(ctx context.Context) error {
	var format string
	err := p.pool.QueryRow(ctx,
		"SELECT value FROM noverse_meta WHERE key = 'schema.format'").Scan(&format)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("schema not initialized — run `storage migrate up` first: %w", errs.ErrStorageUnsupported)
		}
		return fmt.Errorf("schema check failed (is `storage migrate up` applied?): %w", sanitizeErr(err))
	}
	if format != SupportedSchemaFormat {
		return fmt.Errorf("schema.format %q is not supported by this build (supports %q): %w",
			format, SupportedSchemaFormat, errs.ErrStorageUnsupported)
	}
	return nil
}

// SchemaVersion returns the highest applied goose migration version. It is a
// read-only query on the application pool: readiness reports the version the
// database is actually at, and never runs DDL. A database without the goose
// table is unmigrated, not "version 0".
func (p *Pool) SchemaVersion(ctx context.Context) (int64, error) {
	var version int64
	err := p.pool.QueryRow(ctx,
		"SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("schema version query failed: %w", sanitizeErr(err))
	}
	return version, nil
}

var requiredMigrationRe = regexp.MustCompile(`^(\d{5})_.*\.sql$`)

// RequiredMigrations returns the goose versions embedded in this binary, in
// ascending order. It is derived from the embed FS, never hand-maintained.
func RequiredMigrations() []int64 {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return nil
	}
	versions := make([]int64, 0, len(entries))
	for _, entry := range entries {
		m := requiredMigrationRe.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		v, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	return versions
}

// CheckMigrations rejects a database that is missing any migration this
// binary embeds. Older additive schemas are unsafe: handlers query relations
// introduced by later migrations. A database NEWER than this binary is left
// alone (expand-only rollout: the previous release must keep serving while a
// new schema is prepared), so this is a subset check, not equality.
func (p *Pool) CheckMigrations(ctx context.Context) error {
	required := RequiredMigrations()
	if len(required) == 0 {
		return fmt.Errorf("danh sách migration nhúng rỗng: %w", errs.ErrStorageUnsupported)
	}
	rows, err := p.pool.Query(ctx,
		"SELECT version_id FROM goose_db_version WHERE is_applied")
	if err != nil {
		return fmt.Errorf("kiểm tra migration đã áp dụng thất bại: %w", errs.ErrStorageUnsupported)
	}
	defer rows.Close()
	applied := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("đọc migration đã áp dụng thất bại: %w", errs.ErrStorageUnsupported)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("đọc migration đã áp dụng thất bại: %w", errs.ErrStorageUnsupported)
	}
	missing := make([]string, 0, len(required))
	for _, v := range required {
		if !applied[v] {
			missing = append(missing, fmt.Sprintf("%05d", v))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("schema thiếu migration nhúng %s — chạy `storage migrate up` trước: %w",
			strings.Join(missing, ", "), errs.ErrStorageUnsupported)
	}
	return nil
}

// sanitizeErr strips any credential material a driver error may embed.
// pgx wraps the normalized conn config (which contains the DSN) in some
// error paths; error text must never carry passwords or full DSNs.
// sanitizedError keeps the sanitized text but still unwraps to the original
// error, so callers can classify dependency failures (pgconn/net/closed pool)
// with errors.Is/As while no credential ever reaches the message.
type sanitizedError struct {
	msg string
	err error
}

func (e sanitizedError) Error() string { return e.msg }
func (e sanitizedError) Unwrap() error { return e.err }

func sanitizeErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, frag := range []string{"password=", "sslcert=", "sslkey="} {
		if idx := strings.Index(msg, frag); idx >= 0 {
			// truncate at the first credential fragment
			return sanitizedError{msg[:idx] + "[credentials redacted]", err}
		}
	}
	// driver messages may embed "postgres://user:secret@host"; strip the
	// secret inside a URI if present
	if idx := strings.Index(msg, "://"); idx >= 0 {
		if at := strings.Index(msg[idx+3:], "@"); at > 0 {
			cred := msg[idx+3 : idx+3+at]
			if ci := strings.Index(cred, ":"); ci >= 0 {
				msg = msg[:idx+3] + cred[:ci] + ":[redacted]" + msg[idx+3+at:]
			}
		}
	}
	return sanitizedError{msg, err}
}
