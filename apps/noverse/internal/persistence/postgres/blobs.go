package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// BlobGC is the storage-garbage-collection repository: the only place that
// reads GC candidates and the only place that may delete a blob registry row.
//
// A decision locks the blob and rechecks references, then commits a deleting
// claim. Physical deletion happens while that unique-key row still exists;
// FinishBlob removes it only after storage confirms success. Attachment
// triggers reject deleting rows, and publishers serialize on the same row.
// Crashes before or after storage deletion leave a recoverable claim.
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
	// Deleted means this call claimed the object for physical deletion.
	// The registry row remains until FinishBlob succeeds.
	Deleted bool
	Reason  string
}

// GC reasons.
const (
	GCReasonDeleted    = "deleted"    // durable deletion claim acquired
	GCReasonAbsent     = "absent"     // row already gone (another run won)
	GCReasonAttaching  = "attaching"  // state = 'staged': an upload owns it
	GCReasonDeleting   = "deleting"   // tombstoned, not eligible in this pass
	GCReasonTooNew     = "too-new"    // inside the grace window
	GCReasonReferenced = "referenced" // an owner table still points at it
	GCReasonRenewed    = "renewed"    // export-cache entry was renewed meanwhile
)

// ScanBlobs returns one keyset page confined to the active environment.
func (g *BlobGC) ScanBlobs(ctx context.Context, prefix, after, state string, cutoff time.Time, limit int) ([]GCBlob, error) {
	rows, err := gen.New(g.pool).GCScanBlobs(ctx, gen.GCScanBlobsParams{
		Prefix: prefix, AfterKey: after, State: state, Cutoff: ts(cutoff), PageSize: int32(min(max(limit, 1), 1000)),
	})
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

// RegisteredKeys reconciles a bounded provider page, not the entire registry.
func (g *BlobGC) RegisteredKeys(ctx context.Context, keys []string) ([]string, error) {
	out, err := gen.New(g.pool).GCRegisteredKeys(ctx, keys)
	return out, sanitizeErr(err)
}

// ExpiredExportCachePage uses the last entry's composite primary key as cursor.
func (g *BlobGC) ExpiredExportCachePage(ctx context.Context, prefix string, after GCExportCacheEntry, limit int) ([]GCExportCacheEntry, error) {
	if !after.ProjectID.Valid {
		after.ProjectID = pgtype.UUID{Valid: true}
	}
	if !after.StoryID.Valid {
		after.StoryID = pgtype.UUID{Valid: true}
	}
	rows, err := gen.New(g.pool).GCExpiredCachePage(ctx, gen.GCExpiredCachePageParams{
		Prefix: prefix, AfterProject: after.ProjectID, AfterStory: after.StoryID,
		AfterCacheKey: after.CacheKey, AfterKind: after.Kind, PageSize: int32(min(max(limit, 1), 1000)),
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]GCExportCacheEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, GCExportCacheEntry{ProjectID: row.ProjectID, StoryID: row.StoryID, CacheKey: row.CacheKey, Kind: row.Kind, BlobID: row.BlobID})
	}
	return out, nil
}

// FinishBlob releases a deletion claim after an idempotent object delete.
func (g *BlobGC) FinishBlob(ctx context.Context, id pgtype.UUID) error {
	_, err := gen.New(g.pool).GCFinishBlob(ctx, id)
	return sanitizeErr(err)
}

// ClaimOrphan reserves an absent registry key before its bytes are deleted.
// If publication won the insertion race, the caller must leave bytes alone.
func (g *BlobGC) ClaimOrphan(ctx context.Context, key, driver string) (GCBlob, bool, error) {
	id, err := randomUUID()
	if err != nil {
		return GCBlob{}, false, err
	}
	claimed, err := gen.New(g.pool).GCClaimOrphan(ctx, gen.GCClaimOrphanParams{ID: id, ObjectKey: key, StorageDriver: driver})
	if noRows(err) {
		return GCBlob{}, false, nil
	}
	if err != nil {
		return GCBlob{}, false, sanitizeErr(err)
	}
	return GCBlob{ID: claimed, ObjectKey: key, State: "deleting"}, true, nil
}

// LockRun prevents overlapping collectors from acting on stale claims after
// another collector has finished and a publisher has reused the key. The lock
// lives on its own connection; process death automatically releases it.
func (g *BlobGC) LockRun(ctx context.Context) (func(), error) {
	conn, err := g.pool.Acquire(ctx)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	var acquired bool
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(726976442711::bigint)`).Scan(&acquired)
	if err != nil || !acquired {
		conn.Release()
		if err != nil {
			return nil, sanitizeErr(err)
		}
		return nil, fmt.Errorf("storage gc: another collection is running")
	}
	return func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := conn.Exec(cleanup, `SELECT pg_advisory_unlock(726976442711::bigint)`)
		if err != nil {
			_ = conn.Conn().Close(cleanup)
		}
		conn.Release()
	}, nil
}

// ReapBlob rechecks reachability under a row lock and commits a deleting claim.
// The caller must delete bytes before calling FinishBlob.
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
		n, err := q.GCClaimBlob(ctx, id)
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

// ReapExpiredCacheEntry rechecks expiry and removes the expired reference,
// then claims its blob if no other owner remains. Bytes and the claim are
// removed separately by the caller.
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
		removed, err := q.GCClaimBlob(ctx, e.BlobID)
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
