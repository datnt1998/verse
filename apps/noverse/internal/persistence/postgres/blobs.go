package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
)

// BlobGC is the storage-garbage-collection repository: the only place that
// reads GC candidates and the only place that may delete a blob registry row.
//
// Two invariants make deletes safe without stopping writers:
//
//   - Every delete decision is re-taken inside a transaction that first locks
//     the blob row (LockBlobForDelete). A concurrent attachment takes a KEY
//     SHARE lock through its foreign key, so it either commits before the lock
//     (and BlobReferenced sees it) or waits and fails its FK check once the row
//     is gone — a reference can never point at a deleted blob.
//   - A blob row is deleted inside the transaction and its object afterwards,
//     so a crash between the two leaves an orphan object (harmless, collected
//     by the next run) and never a row pointing at missing bytes.
//
// Stale staged rows first become tombstones under the same row lock. The
// associated job is locked first, matching job completion's lock order.
type BlobGC struct {
	pool *pgxpool.Pool
}

// NewBlobGC builds the GC repository on the shared pool.
func NewBlobGC(p *Pool) *BlobGC { return &BlobGC{pool: p.Pool()} }

// RequireMaintenance rejects the app principal before even a dry-run. Merely
// naming an app DSN MIGRATION_DATABASE_URL never grants maintenance authority.
func (g *BlobGC) RequireMaintenance(ctx context.Context) error {
	var allowed bool
	if err := g.pool.QueryRow(ctx, `SELECT noverse_is_maintenance()`).Scan(&allowed); err != nil {
		return sanitizeErr(err)
	}
	if !allowed {
		return fmt.Errorf("storage gc yêu cầu vai trò bảo trì noverse_migrate qua MIGRATION_DATABASE_URL")
	}
	return nil
}

func (g *BlobGC) tx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return RunInTx(ctx, g.pool, fn)
}

// GCBlob is one registry row as GC sees it (identity + key + age).
type GCBlob struct {
	ID        pgtype.UUID
	ProjectID pgtype.UUID
	ObjectKey string
	State     string
	CreatedAt time.Time
}

func gcBlob(r gen.Blob) GCBlob {
	return GCBlob{
		ID: r.ID, ProjectID: r.ProjectID, ObjectKey: r.ObjectKey,
		State: r.State, CreatedAt: r.CreatedAt.Time,
	}
}

// GCExportCacheEntry is one expired export-cache row.
type GCExportCacheEntry struct {
	ProjectID pgtype.UUID
	StoryID   pgtype.UUID
	CacheKey  string
	Kind      string
	BlobID    pgtype.UUID
}

// GCReapResult is the outcome of one reaped candidate. Reason is stable text
// (not a message shown to users) so reports and tests can assert on it.
type GCReapResult struct {
	ObjectKey string
	// Deleted reports that the registry row was removed by this call.
	Deleted bool
	Reason  string
}

// GC reasons.
const (
	GCReasonDeleted    = "deleted"    // row removed by this decision
	GCReasonAbsent     = "absent"     // row already gone (another run won)
	GCReasonAttaching  = "attaching"  // state = 'staged': an upload owns it
	GCReasonDeleting   = "deleting"   // tombstoned, not eligible in this pass
	GCReasonTooNew     = "too-new"    // inside the grace window
	GCReasonReferenced = "referenced" // an owner table still points at it
	GCReasonRenewed    = "renewed"    // export-cache entry was renewed meanwhile
)

// Deleting lists every tombstoned blob, oldest first (bounded by the caller's
// page through the whole table: the set is expected to be tiny).
func (g *BlobGC) Deleting(ctx context.Context) ([]GCBlob, error) {
	rows, err := gen.New(g.pool).ListDeletingBlobs(ctx)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]GCBlob, 0, len(rows))
	for _, r := range rows {
		out = append(out, gcBlob(r))
	}
	return out, nil
}

// Unreferenced lists blobs no owner table points at and that were created
// before cutoff. The listing is advisory: Reap decides again under a lock.
func (g *BlobGC) Unreferenced(ctx context.Context, cutoff time.Time) ([]GCBlob, error) {
	rows, err := gen.New(g.pool).ListUnreferencedBlobs(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]GCBlob, 0, len(rows))
	for _, r := range rows {
		out = append(out, gcBlob(r))
	}
	return out, nil
}

// StaleStaged lists abandoned staged uploads by their original stage time.
// Live leases and queued/paused/waiting jobs are excluded even after grace.
func (g *BlobGC) StaleStaged(ctx context.Context, cutoff time.Time) ([]GCBlob, error) {
	rows, err := gen.New(g.pool).ListStaleStagedBlobs(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]GCBlob, 0, len(rows))
	for _, row := range rows {
		out = append(out, gcBlob(row))
	}
	return out, nil
}

// TombstoneStaged rechecks an advisory candidate under FOR UPDATE. A worker
// that has already advanced it to ready wins; GC never changes a ready row in
// this pass. Holding the job lock also excludes concurrent claims/heartbeats.
func (g *BlobGC) TombstoneStaged(ctx context.Context, id pgtype.UUID, cutoff time.Time) (bool, error) {
	var moved bool
	err := g.tx(ctx, func(tx pgx.Tx) error {
		moved = false
		q := gen.New(tx)
		jobs, err := q.LockBlobUploadJobs(ctx, id)
		if err != nil {
			return sanitizeErr(err)
		}
		for _, active := range jobs {
			if active {
				return nil
			}
		}
		row, err := q.LockBlobForDelete(ctx, id)
		if noRows(err) {
			return nil
		}
		if err != nil {
			return sanitizeErr(err)
		}
		if row.State != "staged" || !row.CreatedAt.Time.Before(cutoff) {
			return nil
		}
		referenced, err := q.BlobReferenced(ctx, gen.BlobReferencedParams{ProjectID: row.ProjectID, BlobID: row.ID})
		if err != nil {
			return sanitizeErr(err)
		}
		if referenced.Bool {
			return nil
		}
		n, err := q.TombstoneStagedBlob(ctx, id)
		if err != nil {
			return sanitizeErr(err)
		}
		moved = n > 0
		return nil
	})
	return moved, err
}

// ObjectKeys returns every registered object key (the orphan-object ledger).
func (g *BlobGC) ObjectKeys(ctx context.Context) ([]string, error) {
	keys, err := gen.New(g.pool).ListBlobObjectKeys(ctx)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	return keys, nil
}

// ExpiredExportCache lists cache entries past their expiry.
func (g *BlobGC) ExpiredExportCache(ctx context.Context) ([]GCExportCacheEntry, error) {
	rows, err := gen.New(g.pool).ListExpiredExportCache(ctx)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]GCExportCacheEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, GCExportCacheEntry{
			ProjectID: r.ProjectID, StoryID: r.StoryID,
			CacheKey: r.CacheKey, Kind: r.Kind, BlobID: r.BlobID,
		})
	}
	return out, nil
}

// ReapBlob decides one blob row under a row lock and deletes it when it is
// still unreachable. allowDeleting admits tombstoned rows (the 'deleting'
// pass); every other pass only reaps live rows older than cutoff.
//
// The caller deletes the returned object key after this returns Deleted.
func (g *BlobGC) ReapBlob(ctx context.Context, id pgtype.UUID, cutoff time.Time, allowDeleting bool) (GCReapResult, error) {
	var out GCReapResult
	err := g.tx(ctx, func(tx pgx.Tx) error {
		out = GCReapResult{}
		q := gen.New(tx)
		row, err := q.LockBlobForDelete(ctx, id)
		if err != nil {
			if noRows(err) {
				out.Reason = GCReasonAbsent
				return nil
			}
			return sanitizeErr(err)
		}
		out.ObjectKey = row.ObjectKey
		switch row.State {
		case "staged":
			out.Reason = GCReasonAttaching
			return nil
		case "deleting":
			if !allowDeleting {
				out.Reason = GCReasonDeleting
				return nil
			}
		default:
			if !row.CreatedAt.Time.Before(cutoff) || !row.UpdatedAt.Time.Before(cutoff) {
				out.Reason = GCReasonTooNew
				return nil
			}
		}
		referenced, err := q.BlobReferenced(ctx, gen.BlobReferencedParams{ProjectID: row.ProjectID, BlobID: row.ID})
		if err != nil {
			return sanitizeErr(err)
		}
		if referenced.Bool {
			out.Reason = GCReasonReferenced
			return nil
		}
		n, err := q.DeleteBlobRow(ctx, id)
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			out.Reason = GCReasonAbsent
			return nil
		}
		out.Deleted = true
		out.Reason = GCReasonDeleted
		return nil
	})
	return out, err
}

// ReapExpiredCacheEntry deletes one expired export-cache row and, in the same
// transaction, its blob row when nothing else references it. Expiry is
// re-checked at delete time, so an entry renewed between the listing and this
// call survives untouched.
//
// The caller deletes ObjectKey after this returns BlobDeleted.
func (g *BlobGC) ReapExpiredCacheEntry(ctx context.Context, e GCExportCacheEntry) (GCReapResult, bool, error) {
	var (
		out          GCReapResult
		entryDeleted bool
	)
	err := g.tx(ctx, func(tx pgx.Tx) error {
		out = GCReapResult{}
		entryDeleted = false
		q := gen.New(tx)
		n, err := q.DeleteExportCacheEntryIfExpired(ctx, gen.DeleteExportCacheEntryIfExpiredParams{
			ProjectID: e.ProjectID, StoryID: e.StoryID, CacheKey: e.CacheKey, Kind: e.Kind,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			out.Reason = GCReasonRenewed
			return nil
		}
		entryDeleted = true
		row, err := q.LockBlobForDelete(ctx, e.BlobID)
		if err != nil {
			if noRows(err) {
				out.Reason = GCReasonAbsent
				return nil
			}
			return sanitizeErr(err)
		}
		out.ObjectKey = row.ObjectKey
		if row.State == "staged" {
			// An expired cache row pointing at a staged blob is inconsistent;
			// never delete an object an upload may still be writing.
			out.Reason = GCReasonAttaching
			return nil
		}
		referenced, err := q.BlobReferenced(ctx, gen.BlobReferencedParams{ProjectID: row.ProjectID, BlobID: row.ID})
		if err != nil {
			return sanitizeErr(err)
		}
		if referenced.Bool {
			out.Reason = GCReasonReferenced
			return nil
		}
		removed, err := q.DeleteBlobRow(ctx, e.BlobID)
		if err != nil {
			return sanitizeErr(err)
		}
		if removed == 0 {
			out.Reason = GCReasonAbsent
			return nil
		}
		out.Deleted = true
		out.Reason = GCReasonDeleted
		return nil
	})
	return out, entryDeleted, err
}
