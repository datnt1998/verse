package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

func migrateSchema(t *testing.T, dsn string) {
	t.Helper()
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	defer db.Close()
	if _, err := migrate.Up(context.Background(), db); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
}

func openPool(t *testing.T, dsn string) *postgres.Pool {
	t.Helper()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestPoolLifecycle proves Open pings the database, the pool serves queries
// and Close is idempotent.
func TestPoolLifecycle(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)

	pool := openPool(t, dsn)
	var one int
	if err := pool.Pool().QueryRow(context.Background(), "SELECT 1").Scan(&one); err != nil {
		t.Fatalf("query on pool: %v", err)
	}
	if one != 1 {
		t.Fatalf("SELECT 1 = %d", one)
	}
	pool.Close()
	// second Close must not panic or error
	pool.Close()

	// unreachable DSN must fail at Open, not at first use — build the string
	// directly (pgx.ConnString() silently drops Port and RuntimeParams).
	badDSN := "postgres://postgres:test@127.0.0.1:1/postgres?sslmode=disable"
	if _, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: badDSN, ConnectTimeout: 2 * time.Second}); err == nil {
		t.Fatal("pool opened against unreachable database")
	}
}

// TestPoolSchemaCompatibility proves Open checks the foundation schema marker
// and refuses databases that lack it or carry an incompatible format.
func TestPoolSchemaCompatibility(t *testing.T) {
	unmigrated := pgtest.Provision(t)
	if _, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: unmigrated}); err == nil {
		t.Fatal("pool opened on unmigrated schema; schema check missing")
	} else {
		t.Logf("unmigrated schema refused: %v", err)
	}

	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)

	// a schema.format outside the supported range must fail the readiness check
	if err := postgres.ExecCAS(context.Background(), pool.Pool(),
		"UPDATE noverse_meta SET value = '999' WHERE key = 'schema.format' AND value = '1'"); err != nil {
		t.Fatalf("CAS update schema.format: %v", err)
	}
	if err := pool.CheckSchema(context.Background()); err == nil {
		t.Fatal("CheckSchema accepted incompatible schema.format")
	} else {
		t.Logf("incompatible format refused: %v", err)
	}
}

// TestTransactionCommit makes a committed write visible on later connections.
func TestTransactionCommit(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)

	if err := postgres.RunInTx(context.Background(), pool.Pool(), func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			"INSERT INTO noverse_meta(key, value) VALUES ('tx.commit', 'yes')")
		return err
	}); err != nil {
		t.Fatalf("RunInTx commit: %v", err)
	}

	var v string
	if err := pool.Pool().QueryRow(context.Background(),
		"SELECT value FROM noverse_meta WHERE key = 'tx.commit'").Scan(&v); err != nil || v != "yes" {
		t.Fatalf("committed write not visible: v=%q err=%v", v, err)
	}
}

// TestTransactionRollbackOnError proves a failed fn rolls back every write in
// the transaction — no partial commit survives.
func TestTransactionRollbackOnError(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)

	sentinel := errors.New("boom")
	err := postgres.RunInTx(context.Background(), pool.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(),
			"INSERT INTO noverse_meta(key, value) VALUES ('tx.rollback', 'partial')"); err != nil {
			t.Fatalf("insert inside tx: %v", err)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("RunInTx returned %v, want the original sentinel error", err)
	}
	var n int
	if err := pool.Pool().QueryRow(context.Background(),
		"SELECT count(*) FROM noverse_meta WHERE key = 'tx.rollback'").Scan(&n); err != nil {
		t.Fatalf("count rollback row: %v", err)
	}
	if n != 0 {
		t.Fatalf("partial write committed: %d rows", n)
	}
}

// TestTransactionRollbackOnCancel cancels the context after the first write;
// the write must be rolled back and the context error surfaced.
func TestTransactionRollbackOnCancel(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)

	ctx, cancel := context.WithCancel(context.Background())
	err := postgres.RunInTx(ctx, pool.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			"INSERT INTO noverse_meta(key, value) VALUES ('tx.cancel', 'partial')"); err != nil {
			t.Fatalf("insert inside tx: %v", err)
		}
		cancel()
		return nil
	})
	if err == nil {
		t.Fatal("RunInTx swallowed cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunInTx returned %v, want context.Canceled in chain", err)
	}
	var n int
	if err := pool.Pool().QueryRow(context.Background(),
		"SELECT count(*) FROM noverse_meta WHERE key = 'tx.cancel'").Scan(&n); err != nil {
		t.Fatalf("count cancel row: %v", err)
	}
	if n != 0 {
		t.Fatalf("cancelled transaction committed %d rows", n)
	}
}

// TestCASSingleWinner runs two writers that each CAS the same row from the
// same expected value. Exactly one wins; the loser gets ErrStorageConflict
// and the row ends in the winner's state — no lost update.
func TestCASSingleWinner(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	ctx := context.Background()

	if err := postgres.ExecCAS(ctx, pool.Pool(),
		"INSERT INTO noverse_meta(key, value) VALUES ('cas.key', 'base')"); err != nil {
		t.Fatalf("insert base: %v", err)
	}
	if err := postgres.ExecCAS(ctx, pool.Pool(),
		"UPDATE noverse_meta SET value = 'first' WHERE key = 'cas.key' AND value = 'base'"); err != nil {
		t.Fatalf("first CAS update: %v", err)
	}
	// second writer saw the same expected 'base' — must conflict, not overwrite
	err := postgres.ExecCAS(ctx, pool.Pool(),
		"UPDATE noverse_meta SET value = 'second' WHERE key = 'cas.key' AND value = 'base'")
	if !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("stale CAS returned %v, want ErrStorageConflict", err)
	}
	var v string
	if err := pool.Pool().QueryRow(ctx,
		"SELECT value FROM noverse_meta WHERE key = 'cas.key'").Scan(&v); err != nil {
		t.Fatalf("read cas.key: %v", err)
	}
	if v != "first" {
		t.Fatalf("losing writer overwrote row: value=%q", v)
	}
}

// TestCASConcurrentWriters fires N goroutines at the same expected value and
// asserts exactly one commits.
func TestCASConcurrentWriters(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	ctx := context.Background()

	if err := postgres.ExecCAS(ctx, pool.Pool(),
		"INSERT INTO noverse_meta(key, value) VALUES ('cas.race', 'base')"); err != nil {
		t.Fatalf("insert base: %v", err)
	}

	const writers = 8
	results := make(chan error, writers)
	for i := range writers {
		go func(i int) {
			results <- postgres.ExecCAS(ctx, pool.Pool(),
				"UPDATE noverse_meta SET value = $1 WHERE key = 'cas.race' AND value = 'base'",
				fmt.Sprintf("writer-%02d", i))
		}(i)
	}
	won, conflicts := 0, 0
	for range writers {
		err := <-results
		switch {
		case err == nil:
			won++
		case errors.Is(err, errs.ErrStorageConflict):
			conflicts++
		default:
			t.Fatalf("unexpected CAS error: %v", err)
		}
	}
	if won != 1 || conflicts != writers-1 {
		t.Fatalf("CAS race: %d won, %d conflicts; want exactly 1/7", won, conflicts)
	}
}
