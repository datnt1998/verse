package postgres

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// ErrLeaseLost is returned when a write transaction runs without a live,
// matching story lease (another process owns the story, or the lease expired).
// It is a hard failure: the caller must stop the run, never retry the write.
var ErrLeaseLost = errors.New("quyền ghi truyện đã mất")

// FenceCheck runs inside a write transaction before the first mutation. The
// backend wires it to the story lease so a stale writer cannot commit even when
// the model returns after the lease moved elsewhere.
type FenceCheck func(ctx context.Context, tx pgx.Tx) error

// StoryLease is the durable exclusive write right for one story: a row in
// story_leases with a fencing token and a bounded lifetime. Renewal extends the
// lifetime and bumps the token; every write transaction re-reads the row, so a
// writer whose lease was taken over fails closed instead of overwriting the new
// owner's work.
type StoryLease struct {
	nar    *Narrative
	scope  ScopeRef
	holder pgtype.UUID
	ttl    time.Duration

	mu    sync.Mutex
	token int64
	lost  error

	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// NewStoryLease claims the story lease for a fresh holder. It fails when
// another live holder owns the story — there is no "wait and steal" path.
func NewStoryLease(ctx context.Context, nar *Narrative, scope ScopeRef, ttl time.Duration) (*StoryLease, error) {
	if ttl <= 0 {
		ttl = 120 * time.Second
	}
	holder := pgtype.UUID{}
	if _, err := rand.Read(holder.Bytes[:]); err != nil {
		return nil, fmt.Errorf("tạo holder id: %w", err)
	}
	holder.Valid = true
	l := &StoryLease{nar: nar, scope: scope, holder: holder, ttl: ttl, done: make(chan struct{})}
	if err := l.claim(ctx); err != nil {
		return nil, err
	}
	renewCtx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	go l.renewLoop(renewCtx)
	return l, nil
}

func (l *StoryLease) claim(ctx context.Context) error {
	until := pgtype.Timestamptz{Time: time.Now().Add(l.ttl), Valid: true}
	var token int64
	err := l.nar.tx(ctx, func(tx pgx.Tx) error {
		n, err := gen.New(tx).ClaimStoryLease(ctx, gen.ClaimStoryLeaseParams{
			ProjectID: l.scope.ProjectID, ID: l.scope.StoryID,
			HolderID: l.holder, LeaseUntil: until, Draft: l.scope.Draft,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			return ErrLeaseLost
		}
		// Promotion/removal must lock the lease we now hold. Recheck the row
		// after claiming in case it changed while the INSERT was waiting.
		var draft bool
		err = tx.QueryRow(ctx, `SELECT status = 'draft' FROM stories
			WHERE project_id=$1 AND id=$2 AND deleted_at IS NULL`,
			l.scope.ProjectID, l.scope.StoryID).Scan(&draft)
		if noRows(err) || (err == nil && draft != l.scope.Draft) {
			return ErrLeaseLost
		}
		if err != nil {
			return sanitizeErr(err)
		}
		row, err := gen.New(tx).GetStoryLease(ctx, gen.GetStoryLeaseParams{
			ProjectID: l.scope.ProjectID, StoryID: l.scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		if row.HolderID != l.holder {
			return ErrLeaseLost
		}
		token = row.FencingToken
		return nil
	})
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.token = token
	l.mu.Unlock()
	return nil
}

// Err reports why the lease was lost; nil while still held.
func (l *StoryLease) Err() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lost
}

// Token is the current fencing token (0 before the first claim).
func (l *StoryLease) Token() int64 {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.token
}

// Checker returns the fence every write transaction must pass. It share-locks
// the lease row, so a takeover waits until the checked write commits instead
// of landing between the check and the commit.
func (l *StoryLease) Checker() FenceCheck {
	return func(ctx context.Context, tx pgx.Tx) error {
		if err := l.Err(); err != nil {
			return err
		}
		l.mu.Lock()
		wantHolder, wantToken := l.holder, l.token
		l.mu.Unlock()
		row, err := gen.New(tx).GetStoryLeaseForWrite(ctx, gen.GetStoryLeaseForWriteParams{
			ProjectID: l.scope.ProjectID, StoryID: l.scope.StoryID})
		if err != nil {
			if noRows(err) {
				return ErrLeaseLost
			}
			return sanitizeErr(err)
		}
		if !row.HolderID.Valid || row.HolderID != wantHolder || row.FencingToken != wantToken {
			return fmt.Errorf("%w: fencing token %d (cần %d)", ErrLeaseLost, row.FencingToken, wantToken)
		}
		if !row.LeaseUntil.Valid || !row.LeaseUntil.Time.After(time.Now()) {
			return fmt.Errorf("%w: hết hạn", ErrLeaseLost)
		}
		return nil
	}
}

// renew extends the lease this holder still owns at its fencing token. It
// never re-claims: once the lease expired or moved, renewal reports it lost
// even if the row has since become free.
func (l *StoryLease) renew(ctx context.Context) error {
	l.mu.Lock()
	token := l.token
	l.mu.Unlock()
	return l.nar.tx(ctx, func(tx pgx.Tx) error {
		n, err := gen.New(tx).RenewStoryLease(ctx, gen.RenewStoryLeaseParams{
			ProjectID: l.scope.ProjectID, StoryID: l.scope.StoryID,
			HolderID: l.holder, FencingToken: token,
			LeaseUntil: pgtype.Timestamptz{Time: time.Now().Add(l.ttl), Valid: true},
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			return ErrLeaseLost
		}
		return nil
	})
}

// renewLoop extends the lease at a third of its lifetime. A failed renewal is
// sticky: writes fail from then on and the Host cancels the run.
func (l *StoryLease) renewLoop(ctx context.Context) {
	defer close(l.done)
	interval := l.ttl / 3
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := l.renew(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				l.mu.Lock()
				if l.lost == nil {
					l.lost = fmt.Errorf("gia hạn quyền ghi: %w", err)
				}
				l.mu.Unlock()
				return
			}
		}
	}
}

// Close stops renewal and releases the row when this holder still owns it.
func (l *StoryLease) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		if l.cancel != nil {
			l.cancel()
			<-l.done
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return l.nar.ReleaseLease(ctx, l.scope, l.holder)
}
