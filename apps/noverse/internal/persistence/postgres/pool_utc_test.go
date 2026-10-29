package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

// TestPoolPinsTimestampsToUTC pins the one serialization boundary for
// instants: the session timezone is UTC and timestamptz decodes in UTC, so
// JSON/RFC3339 output and operator listings never depend on the host locale.
// Timestamptz still stores the same instant; only the presented offset changes.
func TestPoolPinsTimestampsToUTC(t *testing.T) {
	dsn := pgtest.Provision(t)
	ctx := context.Background()
	pool, err := postgres.Open(ctx, postgres.PoolConfig{DSN: dsn, SkipSchemaCheck: true})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	var sessionTZ string
	if err := pool.Pool().QueryRow(ctx, "SHOW timezone").Scan(&sessionTZ); err != nil {
		t.Fatalf("show timezone: %v", err)
	}
	if sessionTZ != "UTC" {
		t.Fatalf("session timezone = %q, want UTC", sessionTZ)
	}

	var got time.Time
	if err := pool.Pool().QueryRow(ctx, "SELECT timestamptz '2026-10-29 12:00:00+07'").Scan(&got); err != nil {
		t.Fatalf("scan timestamptz: %v", err)
	}
	if got.Location() != time.UTC {
		t.Fatalf("decoded location = %v, want UTC", got.Location())
	}
	if formatted := got.Format(time.RFC3339); formatted != "2026-10-29T05:00:00Z" {
		t.Fatalf("serialized = %s, want 2026-10-29T05:00:00Z (same instant, UTC offset)", formatted)
	}
}
