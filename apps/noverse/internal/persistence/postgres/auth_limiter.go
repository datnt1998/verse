package postgres

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// AuthLimiter là backoff limiter login/pairing trên login_rate_limits:
// cùng mô hình retryAfter/base/cap như webauth.Limiter nhưng state sống trong
// DB nên hạn chế chia sẻ giữa các process/serve và sống sót restart.
// Bucket key = sha256(prefix + "|" + key) — không lưu IP/username thô.
type AuthLimiter struct {
	pool *Pool
	now  func() time.Time
}

// NewAuthLimiter builds the shared login/pairing limiter on the app pool.
func NewAuthLimiter(pool *Pool) *AuthLimiter {
	return &AuthLimiter{pool: pool, now: time.Now}
}

const (
	limitMaxFails = 4
	limitBase     = 2 * time.Second
	limitCap      = 10 * time.Minute
)

func bucketKey(kind, key string) []byte {
	sum := sha256.Sum256([]byte(kind + "|" + key))
	return sum[:]
}

// Wait returns the longest remaining block across keys (0 = allowed) — the
// same contract as webauth.Limiter.Wait.
func (l *AuthLimiter) Wait(keys ...string) time.Duration {
	maxRetry := time.Duration(0)
	now := l.now()
	for _, k := range keys {
		bucket := bucketKey("auth", k)
		row, err := gen.New(l.pool.Pool()).GetLimitBucket(context.Background(), bucket)
		if err != nil {
			continue // missing bucket or read failure: fail open, Fail records
		}
		if row.BlockedUntil.Valid && row.BlockedUntil.Time.After(now) {
			if d := row.BlockedUntil.Time.Sub(now); d > maxRetry {
				maxRetry = d
			}
		}
	}
	return maxRetry
}

// Fail records one failure per key; at maxFails it blocks for the geometric
// backoff (base·2^extra, capped at cap) — same shape as the memory limiter.
func (l *AuthLimiter) Fail(keys ...string) {
	now := l.now()
	for _, k := range keys {
		l.bump(bucketKey("auth", k), now)
	}
}

// Succeed clears the buckets after a successful credential check.
func (l *AuthLimiter) Succeed(keys ...string) {
	q := gen.New(l.pool.Pool())
	for _, k := range keys {
		_ = q.DeleteLimitBucket(context.Background(), bucketKey("auth", k))
	}
}

// bump records one failure. The gateway skips Fail while a key is blocked, so
// the count keeps growing across expired blocks and each block doubles, like
// the memory limiter; a bucket idle past expires_at starts over.
func (l *AuthLimiter) bump(bucket []byte, now time.Time) {
	_ = RunInTx(context.Background(), l.pool.Pool(), func(tx pgx.Tx) error {
		q := gen.New(tx)
		row, err := q.GetLimitBucketForUpdate(context.Background(), bucket)
		fails := int32(0)
		if err == nil {
			if row.ExpiresAt.Valid && row.ExpiresAt.Time.After(now) {
				fails = row.Failures
			}
		} else if !noRows(err) {
			return nil
		}
		fails++
		var until time.Time
		if fails >= limitMaxFails {
			until = now.Add(l.backoff(fails))
		}
		return q.InsertLimitBucket(context.Background(), gen.InsertLimitBucketParams{
			BucketHash:      bucket,
			Failures:        fails,
			WindowStartedAt: ts(now),
			BlockedUntil:    pgTimestamptz(until),
			ExpiresAt:       ts(now.Add(2 * limitCap)),
		})
	})
}

func (l *AuthLimiter) backoff(fails int32) time.Duration {
	d := limitBase
	for i := int32(limitMaxFails); i < fails && d < limitCap; i++ {
		d *= 2
	}
	if d > limitCap {
		d = limitCap
	}
	return d
}

// --- recovery window limiter -------------------------------------------------

// WindowLimiter là limiter cửa sổ trượt N lần/window trên cùng bảng — dùng
// cho recovery (5 lần/giờ). Khác backoff: Wait chỉ nhìn failures trong
// window hiện tại; bucket reset khi window_started_at cũ hết hạn.
type WindowLimiter struct {
	pool   *Pool
	now    func() time.Time
	kind   string
	limit  int
	window time.Duration
}

// NewWindowLimiter builds a DB sliding-window limiter (recovery codes).
func NewWindowLimiter(pool *Pool, kind string, limit int, window time.Duration) *WindowLimiter {
	return &WindowLimiter{pool: pool, now: time.Now, kind: kind, limit: limit, window: window}
}

// Wait reports the remaining block time (0 = allowed). Read-only: a try is
// charged by Fail, matching webauth.WindowLimiter semantics exactly.
func (l *WindowLimiter) Wait(key string) time.Duration {
	bucket := bucketKey(l.kind, key)
	row, err := gen.New(l.pool.Pool()).GetLimitBucket(context.Background(), bucket)
	if err != nil {
		return 0 // read failure fails open; Fail still records the attempt
	}
	now := l.now()
	if !row.WindowStartedAt.Valid || now.Sub(row.WindowStartedAt.Time) >= l.window {
		return 0
	}
	if row.Failures < int32(l.limit) {
		return 0
	}
	retry := row.WindowStartedAt.Time.Add(l.window).Sub(now)
	if retry < 0 {
		return 0
	}
	return retry
}

// Fail records one attempt; the window slides on the first failure of the
// current window — N tries per window per bucket, exactly like the RAM
// sliding-window limiter.
func (l *WindowLimiter) Fail(key string) {
	bucket := bucketKey(l.kind, key)
	now := l.now()
	_ = RunInTx(context.Background(), l.pool.Pool(), func(tx pgx.Tx) error {
		q := gen.New(tx)
		row, err := q.GetLimitBucketForUpdate(context.Background(), bucket)
		fails := int32(0)
		windowStart := now
		if err == nil {
			if row.WindowStartedAt.Valid && now.Sub(row.WindowStartedAt.Time) < l.window {
				fails = row.Failures
				windowStart = row.WindowStartedAt.Time
			}
		} else if !noRows(err) {
			return err
		}
		fails++
		return q.InsertLimitBucket(context.Background(), gen.InsertLimitBucketParams{
			BucketHash:      bucket,
			Failures:        fails,
			WindowStartedAt: ts(windowStart),
			BlockedUntil:    pgTimestamptz(time.Time{}),
			ExpiresAt:       ts(windowStart.Add(2 * l.window)),
		})
	})
}

// Succeed clears the bucket after a valid code.
func (l *WindowLimiter) Succeed(key string) {
	q := gen.New(l.pool.Pool())
	_ = q.DeleteLimitBucket(context.Background(), bucketKey(l.kind, key))
}
