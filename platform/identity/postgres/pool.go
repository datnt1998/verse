// Package postgres is the shared account repository of the verse apps. It owns
// the identity tables of the Noverse Postgres database: accounts, credentials,
// devices/sessions, recovery, the shared login limiter, audit entries and the
// per-app grant rows. The schema itself stays with the Noverse goose migrations;
// this package only queries it, so Viderse never imports an app package.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/platform/identity"
)

// PoolConfig bounds the identity pool. Zero values get explicit defaults; the
// caller owns Close.
type PoolConfig struct {
	DSN             string
	MaxConns        int32
	MaxConnIdleTime time.Duration
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	// SkipContractCheck is for tests that provision a deliberately incomplete
	// schema; production Open always checks the account/grant contract.
	SkipContractCheck bool
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

// Pool wraps the bounded pgx pool used by the shared identity repository.
type Pool struct {
	pool *pgxpool.Pool
}

// Open parses the DSN, bounds the pool, pings within ConnectTimeout and checks
// that the account/grant tables are readable by this role. It deliberately does
// NOT read the Noverse schema marker or goose history: Viderse's identity role
// has no access to those tables.
func Open(ctx context.Context, cfg PoolConfig) (*Pool, error) {
	cfg = cfg.withDefaults()
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		// pgx parse errors may echo the raw DSN (credentials included);
		// never forward them.
		return nil, errors.New("invalid VERSE_IDENTITY_DATABASE_URL: cannot parse connection string")
	}
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	pcfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	// UTC is the single instant boundary: the session timezone is UTC so
	// server-side casts agree, and timestamptz decodes in UTC so serialized
	// timestamps carry "Z" instead of the process/host offset.
	if pcfg.ConnConfig.RuntimeParams == nil {
		pcfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	pcfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pcfg.AfterConnect = scanTimestampsInUTC

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	native, err := pgxpool.NewWithConfig(pingCtx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("identity postgres pool init: %w", sanitizeErr(err))
	}
	if err := native.Ping(pingCtx); err != nil {
		native.Close()
		return nil, fmt.Errorf("identity postgres ping failed: %w", sanitizeErr(err))
	}
	p := &Pool{pool: native}
	if !cfg.SkipContractCheck {
		if err := p.CheckContract(pingCtx); err != nil {
			native.Close()
			return nil, err
		}
	}
	return p, nil
}

// scanTimestampsInUTC installs the UTC decode location on each fresh
// connection. It registers a per-connection type instead of mutating pgx's
// process-wide default map (which would race every other connection).
func scanTimestampsInUTC(_ context.Context, conn *pgx.Conn) error {
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name:  "timestamptz",
		OID:   pgtype.TimestamptzOID,
		Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
	})
	return nil
}

// Pool exposes the native pool for the repository and its transactions.
func (p *Pool) Pool() *pgxpool.Pool { return p.pool }

// Close releases the pool; idempotent.
func (p *Pool) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
}

// identityContract lists the columns this repository reads or writes. Login and
// grant flows are the acceptance surface: a role that cannot see them must fail
// at startup, not on the first request.
var identityContract = map[string][]string{
	"users":               {"id", "username", "display_name", "global_role", "status", "must_change_password", "onboarded_at", "disabled_at", "disabled_by", "disabled_reason", "deleted_at", "created_at", "updated_at", "version"},
	"user_credentials":    {"user_id", "password_hash", "password_changed_at"},
	"devices":             {"id", "user_id", "name", "last_ip", "last_seen_at"},
	"sessions":            {"id", "user_id", "device_id", "token_hash", "credential_kind", "temporary_password_id", "last_seen_at", "expires_at", "revoked_at"},
	"temporary_passwords": {"id", "user_id", "password_hash", "expires_at", "used_at", "revoked_at", "created_by"},
	"recovery_codes":      {"id", "user_id", "code_hash", "mask", "used_at", "revoked_at"},
	"device_pairings":     {"id", "user_id", "token_hash", "created_by", "expires_at", "used_at", "revoked_at"},
	"login_rate_limits":   {"bucket_hash", "failures", "window_started_at", "blocked_until", "expires_at"},
	"auth_invariant_lock": {"id"},
	"audit_logs":          {"id", "actor_user_id", "actor_kind", "action", "target_kind", "target_id", "request_id", "details", "occurred_at"},
	"app_grants":          {"user_id", "app", "role", "granted_by", "granted_at"},
}

// CheckContract verifies the account/grant tables and their required columns
// exist. information_schema is readable without grants on the tables
// themselves, so the check never requires SELECT on audit history.
func (p *Pool) CheckContract(ctx context.Context) error {
	names := make([]string, 0, len(identityContract))
	for table := range identityContract {
		names = append(names, table)
	}
	rows, err := p.pool.Query(ctx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = ANY (current_schemas(true))
		  AND table_name = ANY ($1)`, names)
	if err != nil {
		return fmt.Errorf("kiểm tra schema identity thất bại: %w: %w", identity.ErrUnsupported, sanitizeErr(err))
	}
	defer rows.Close()
	found := map[string]map[string]bool{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			return fmt.Errorf("đọc schema identity thất bại: %w: %w", identity.ErrUnsupported, sanitizeErr(err))
		}
		if found[table] == nil {
			found[table] = map[string]bool{}
		}
		found[table][column] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("đọc schema identity thất bại: %w: %w", identity.ErrUnsupported, sanitizeErr(err))
	}
	missing := make([]string, 0)
	for table, columns := range identityContract {
		for _, column := range columns {
			if !found[table][column] {
				missing = append(missing, table+"."+column)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("schema identity thiếu %s — chạy `storage migrate up` trước: %w",
			strings.Join(missing, ", "), identity.ErrUnsupported)
	}
	return nil
}

// RunInTx executes fn inside a real transaction on the pool: Begin → fn →
// Commit, with a deferred Rollback that is a no-op after commit. External calls
// (SQL, hash) must never run inside fn: the connection is held for the whole
// transaction.
func RunInTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("begin transaction: %w", cerr)
		}
		return fmt.Errorf("begin transaction: %w", sanitizeErr(err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("commit transaction: %w", cerr)
		}
		return fmt.Errorf("commit transaction: %w", sanitizeErr(err))
	}
	return nil
}

// sanitizedError keeps the sanitized text but still unwraps to the original
// error, so callers can classify dependency failures (pgconn/net/closed pool)
// with errors.Is/As while no credential ever reaches the message.
type sanitizedError struct {
	msg string
	err error
}

func (e sanitizedError) Error() string { return e.msg }
func (e sanitizedError) Unwrap() error { return e.err }

// sanitizeErr strips any credential material a driver error may embed. pgx
// wraps the normalized conn config (which contains the DSN) in some error
// paths; error text must never carry passwords or full DSNs.
func sanitizeErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, frag := range []string{"password=", "sslcert=", "sslkey="} {
		if idx := strings.Index(msg, frag); idx >= 0 {
			return sanitizedError{msg[:idx] + "[credentials redacted]", err}
		}
	}
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
