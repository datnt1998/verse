package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/noverse/internal/errs"
)

// RunInTx executes fn inside a real Postgres transaction on the shared pool:
// Begin → fn → Commit, with a deferred Rollback that is a no-op after commit.
// The fn error is propagated unchanged; commit errors (including context
// cancellation mid-transaction) roll everything back — no partial writes,
// no auto-retry side effects. External calls (LLM, S3, ffmpeg) must never
// run inside fn: the connection is held for the transaction's lifetime.
func RunInTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("begin transaction: %w", cerr)
		}
		return fmt.Errorf("begin transaction: %w", sanitizeErr(err))
	}
	// Rollback after Commit is documented no-op; covers fn error and panic.
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		// pgx wraps a cancelled context in an opaque message that does not
		// chain to context.Canceled — surface the ctx error directly so the
		// cancellation contract holds.
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("commit transaction: %w", cerr)
		}
		return fmt.Errorf("commit transaction: %w", sanitizeErr(err))
	}
	return nil
}

// ExecCAS runs a compare-and-swap write: the statement must embed its own
// expected-state predicate (WHERE … AND expected = $n). Zero rows affected
// means a concurrent writer won or the expected state is gone — surfaced as
// errs.ErrStorageConflict, never silently ignored.
func ExecCAS(ctx context.Context, q Querier, sql string, args ...any) error {
	tag, err := q.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("CAS write: %w", sanitizeErr(err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("expected state changed under concurrent write: %w", errs.ErrStorageConflict)
	}
	return nil
}

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx so CAS works inside
// and outside transactions.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
