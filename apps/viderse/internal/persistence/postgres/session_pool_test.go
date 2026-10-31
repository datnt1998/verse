package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSessionLocksDoNotExhaustQueryPool(t *testing.T) {
	base := provision(t)
	config := base.Config()
	config.MaxConns = 3
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := postgres.NewImageReceiptStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var releases []func()
	defer func() {
		for _, release := range releases {
			release()
		}
	}()
	for i := range 3 {
		release, err := store.LockKey(ctx, fmt.Sprintf("pool-exhaustion-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	for range 3 {
		var value int
		if err := pool.QueryRow(ctx, "SELECT 1").Scan(&value); err != nil || value != 1 {
			t.Fatalf("query while three draw locks are held: value=%d err=%v", value, err)
		}
	}
}
