// Package storagegc deletes unreachable objects and rows from the blob store
// and the blob registry. It is the storage half of `ainovel-cli storage gc`
// and the only component that may remove an object without a request asking
// for it.
//
// What one run collects, in this order:
//
//  1. stale staged uploads without references or an active job become tombstones;
//     tombstones (`blobs.state = 'deleting'`) follow the physical-delete path;
//  2. expired `export_cache` entries and the blobs only they referenced;
//  3. blobs no owner table references and older than the grace window;
//  4. objects under `{env}/staging/` older than the staging age (reserved for
//     resumable/multipart uploads that never finished);
//  5. orphan objects under `{env}/` — present in the store, absent from the
//     registry, older than the grace window.
//
// Safety properties:
//
//   - Environment isolation: every listing, every key parse and every delete
//     is confined to `Keys.Prefix()`. Backups (`{env}/backups/`) and staging
//     are excluded from the orphan sweep, and a key from another environment
//     is reported, never touched.
//   - Attaching/referenced blobs are excluded transactionally: the row is
//     locked before the decision and the registry delete happens in the same
//     transaction (see postgres.BlobGC), so a concurrent attachment either
//     wins the race and keeps the blob or fails its foreign key. Staged uploads
//     remain protected while their job is active/leased or inside staged grace.
//   - Grace: a blob younger than the grace window is never unreferenced-blob
//     or orphan material, which covers an upload whose object landed before
//     its registry row committed.
//   - Crash safety: rows are deleted before objects; the worst outcome is an
//     orphan object that the next run collects, never a row pointing at
//     missing bytes.
//   - Idempotence: a second run over the same store deletes nothing and
//     reports zeroes.
package storagegc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// Defaults. The grace window covers "object written, registry row not yet
// committed" and any legal asynchronous attachment; the staging age is the
// documented expiry of an abandoned resumable upload.
const (
	DefaultGrace      = 24 * time.Hour
	DefaultStagingAge = 24 * time.Hour
	DefaultStagedAge  = 24 * time.Hour
	defaultPageSize   = 500
)

// Options configures one GC run.
type Options struct {
	Pool  *postgres.Pool
	Blobs blobstore.BlobStore
	// Repo overrides the registry repository (tests); nil builds it from Pool.
	Repo *postgres.BlobGC
	// Keys is the active object-key scheme (STORAGE_ENV). GC never leaves its
	// prefix.
	Keys blobkey.Keys
	// Now is the clock; nil means time.Now. Tests age objects by moving the
	// clock forward (provider modification times cannot be rewritten).
	Now func() time.Time
	// Grace is the minimum age of an unreferenced blob/orphan object.
	Grace time.Duration
	// StagingAge is the minimum age of an abandoned staging object.
	StagingAge time.Duration
	// StagedAge is the minimum age of a staged registry row (not a staging object).
	StagedAge time.Duration
	DryRun    bool
	// Logger receives one line per action with module=storagegc; nil uses the
	// default logger.
	Logger   *slog.Logger
	PageSize int
}

// Report is the machine-readable outcome of one run.
type Report struct {
	DryRun           bool     `json:"dryRun"`
	Env              string   `json:"env"`
	GraceSeconds     int64    `json:"graceSeconds"`
	StagingAgeSecs   int64    `json:"stagingAgeSeconds"`
	StagedAgeSecs    int64    `json:"stagedAgeSeconds"`
	StagedSeen       int      `json:"stagedSeen"`
	StagedTombstoned int      `json:"stagedTombstoned"`
	DeletingFound    int      `json:"deletingFound"`
	DeletingReaped   int      `json:"deletingReaped"`
	CacheExpired     int      `json:"cacheExpired"`
	CacheDeleted     int      `json:"cacheDeleted"`
	CacheBlobsReaped int      `json:"cacheBlobsReaped"`
	UnreferencedSeen int      `json:"unreferencedSeen"`
	BlobsReaped      int      `json:"blobsReaped"`
	BlobsReferenced  int      `json:"blobsSkippedReferenced"`
	BlobsSkipped     int      `json:"blobsSkippedOther"`
	StagingSeen      int      `json:"stagingSeen"`
	StagingDeleted   int      `json:"stagingDeleted"`
	OrphansSeen      int      `json:"orphansSeen"`
	OrphansDeleted   int      `json:"orphansDeleted"`
	ObjectsDeleted   int      `json:"objectsDeleted"`
	OutOfScope       int      `json:"outOfScope"`
	UnknownObjects   int      `json:"unknownObjects"`
	Errors           []string `json:"errors,omitempty"`
}

// Format renders the human summary printed by the CLI.
func (r Report) Format() string {
	mode := "thật"
	if r.DryRun {
		mode = "dry-run (không xoá gì)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "storage gc env=%s chế_độ=%s grace=%s staging_age=%s\n",
		r.Env, mode, time.Duration(r.GraceSeconds)*time.Second, time.Duration(r.StagingAgeSecs)*time.Second)
	fmt.Fprintf(&b, "  tombstone: %d thấy / %d xoá\n", r.DeletingFound, r.DeletingReaped)
	fmt.Fprintf(&b, "  staged quá hạn (%s): %d thấy / %d đánh dấu xoá\n",
		time.Duration(r.StagedAgeSecs)*time.Second, r.StagedSeen, r.StagedTombstoned)
	fmt.Fprintf(&b, "  export_cache hết hạn: %d thấy / %d xoá (blob chỉ cache dùng: %d)\n",
		r.CacheExpired, r.CacheDeleted, r.CacheBlobsReaped)
	fmt.Fprintf(&b, "  blob không tham chiếu: %d thấy / %d xoá (còn tham chiếu: %d, bỏ qua khác: %d)\n",
		r.UnreferencedSeen, r.BlobsReaped, r.BlobsReferenced, r.BlobsSkipped)
	fmt.Fprintf(&b, "  staging: %d thấy / %d xoá\n", r.StagingSeen, r.StagingDeleted)
	fmt.Fprintf(&b, "  object mồ côi: %d thấy / %d xoá\n", r.OrphansSeen, r.OrphansDeleted)
	fmt.Fprintf(&b, "  object đã xoá: %d | ngoài phạm vi: %d | không nhận dạng: %d | lỗi: %d\n",
		r.ObjectsDeleted, r.OutOfScope, r.UnknownObjects, len(r.Errors))
	return b.String()
}

// Report.Run runs one collection pass. It returns a report even when some
// actions failed; the error aggregates the failures so the command exits
// nonzero while the successful part of the run stays visible.
func Run(ctx context.Context, opts Options) (Report, error) {
	rep := Report{DryRun: opts.DryRun}
	if opts.Pool == nil {
		return rep, errors.New("storage gc: thiếu kết nối Postgres")
	}
	if opts.Blobs == nil {
		return rep, errors.New("storage gc: thiếu BlobStore")
	}
	repo := opts.Repo
	if repo == nil {
		repo = postgres.NewBlobGC(opts.Pool)
	}
	if err := repo.RequireMaintenance(ctx); err != nil {
		return rep, err
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	grace := opts.Grace
	if grace <= 0 {
		grace = DefaultGrace
	}
	stagingAge := opts.StagingAge
	if stagingAge <= 0 {
		stagingAge = DefaultStagingAge
	}
	stagedAge := opts.StagedAge
	if stagedAge <= 0 {
		stagedAge = DefaultStagedAge
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("module", "storagegc", "env", string(opts.Keys.Env()), "dry_run", opts.DryRun)
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	started := now().UTC()
	cutoff := started.Add(-grace)
	stagingCutoff := started.Add(-stagingAge)
	rep.Env = string(opts.Keys.Env())
	rep.GraceSeconds = int64(grace / time.Second)
	rep.StagingAgeSecs = int64(stagingAge / time.Second)
	rep.StagedAgeSecs = int64(stagedAge / time.Second)

	r := &runner{
		ctx: ctx, opts: opts, repo: repo, log: logger, pageSize: pageSize,
		cutoff: cutoff, stagingCutoff: stagingCutoff,
		deleted: map[string]bool{},
	}

	// The registry snapshot is taken before any delete so the orphan sweep can
	// tell "row exists" from "row was deleted by this run" without re-querying.
	registeredKeys, err := repo.ObjectKeys(ctx)
	if err != nil {
		return rep, fmt.Errorf("đọc danh sách object đã đăng ký: %w", err)
	}
	registered := make(map[string]bool, len(registeredKeys))
	for _, key := range registeredKeys {
		registered[key] = true
	}

	// Abandoned staged rows enter the existing tombstone path. This is a
	// separate pass so a concurrent ready transition is never treated as a
	// stale staged candidate. Dry-run lists only; it takes no locks or writes.
	stagedCutoff := started.Add(-stagedAge)
	staged, err := repo.StaleStaged(ctx, stagedCutoff)
	if err != nil {
		r.fail("staged", "", err)
	} else {
		for _, blob := range staged {
			if !r.inScope(blob.ObjectKey) {
				rep.OutOfScope++
				continue
			}
			rep.StagedSeen++
			if opts.DryRun {
				rep.StagedTombstoned++
				continue
			}
			moved, err := repo.TombstoneStaged(ctx, blob.ID, stagedCutoff)
			if err != nil {
				r.fail("staged", blob.ObjectKey, err)
				continue
			}
			if moved {
				rep.StagedTombstoned++
			}
		}
	}

	// 1. Tombstones: the application already declared these dead.
	deleting, err := repo.Deleting(ctx)
	if err != nil {
		r.fail("deleting", "", err)
	} else {
		rep.DeletingFound = len(deleting)
		for _, blob := range deleting {
			if !r.inScope(blob.ObjectKey) {
				rep.OutOfScope++
				continue
			}
			if opts.DryRun {
				rep.DeletingReaped++
				continue
			}
			res, err := repo.ReapBlob(ctx, blob.ID, cutoff, true)
			if err != nil {
				r.fail("deleting", blob.ObjectKey, err)
				continue
			}
			if !res.Deleted {
				// Another run got there first (absent) or a reference appeared
				// (referenced): the object's ownership is no longer proven by
				// this listing, so never delete bytes from here.
				r.log.Debug("bỏ qua tombstone", "key", blob.ObjectKey, "reason", res.Reason)
				continue
			}
			rep.DeletingReaped++
			delete(registered, blob.ObjectKey)
			if r.deleteObject(blob.ObjectKey, "tombstone") {
				rep.ObjectsDeleted++
			}
		}
	}

	// 2. Expired export-cache entries and the blobs only they referenced.
	expired, err := repo.ExpiredExportCache(ctx)
	if err != nil {
		r.fail("export_cache", "", err)
	} else {
		rep.CacheExpired = len(expired)
		for _, entry := range expired {
			if opts.DryRun {
				rep.CacheDeleted++
				continue
			}
			res, entryDeleted, err := repo.ReapExpiredCacheEntry(ctx, entry)
			if err != nil {
				r.fail("export_cache", "", err)
				continue
			}
			if entryDeleted {
				rep.CacheDeleted++
			}
			if !res.Deleted {
				r.log.Debug("giữ blob của export_cache", "key", res.ObjectKey, "reason", res.Reason)
				continue
			}
			rep.CacheBlobsReaped++
			delete(registered, res.ObjectKey)
			if !r.inScope(res.ObjectKey) {
				// A foreign-environment key in this registry is a defect; the
				// row is gone but the object is left alone.
				r.log.Error("blob đã xoá hàng nhưng object ngoài phạm vi môi trường", "key", res.ObjectKey)
				rep.OutOfScope++
				continue
			}
			if r.deleteObject(res.ObjectKey, "export_cache hết hạn") {
				rep.ObjectsDeleted++
			}
		}
	}

	// 3. Unreferenced blobs older than the grace window.
	unreferenced, err := repo.Unreferenced(ctx, cutoff)
	if err != nil {
		r.fail("blob", "", err)
	} else {
		rep.UnreferencedSeen = len(unreferenced)
		for _, blob := range unreferenced {
			if !r.inScope(blob.ObjectKey) {
				rep.OutOfScope++
				continue
			}
			if opts.DryRun {
				// This listing contains only ready rows. Staged rows have
				// their own age/job-lease gate above.
				rep.BlobsReaped++
				continue
			}
			res, err := repo.ReapBlob(ctx, blob.ID, cutoff, false)
			if err != nil {
				r.fail("blob", blob.ObjectKey, err)
				continue
			}
			switch res.Reason {
			case postgres.GCReasonDeleted:
				rep.BlobsReaped++
				delete(registered, blob.ObjectKey)
				if r.deleteObject(blob.ObjectKey, "không tham chiếu") {
					rep.ObjectsDeleted++
				}
			case postgres.GCReasonReferenced:
				rep.BlobsReferenced++
			default:
				rep.BlobsSkipped++
			}
		}
	}

	// 4. Abandoned staging objects.
	if err := r.forEachObject(opts.Keys.StagingPrefix(), func(info blobstore.BlobInfo) {
		rep.StagingSeen++
		if !info.ModTime.Before(stagingCutoff) {
			return
		}
		if opts.DryRun {
			rep.StagingDeleted++
			return
		}
		if r.deleteObject(info.Key, "staging quá hạn") {
			rep.StagingDeleted++
			rep.ObjectsDeleted++
		}
	}); err != nil {
		r.fail("staging", "", err)
	}

	// 5. Orphan objects: in the store, absent from the registry, old enough.
	if err := r.forEachObject(opts.Keys.Prefix(), func(info blobstore.BlobInfo) {
		key := info.Key
		if strings.HasPrefix(key, opts.Keys.StagingPrefix()) || strings.HasPrefix(key, opts.Keys.BackupPrefix()) {
			return // owned by rule 4 or by the backup retention policy
		}
		if registered[key] || r.deleted[key] {
			return
		}
		if !info.ModTime.Before(cutoff) {
			return
		}
		obj, err := blobkey.Parse(key)
		if err != nil || obj.Env != opts.Keys.Env() {
			// Never guess: an unparseable key may belong to a future kind or
			// another environment, so it is reported instead of deleted.
			rep.UnknownObjects++
			r.log.Warn("bỏ qua object không nhận dạng", "key", key, "err", err)
			return
		}
		if obj.Kind == blobkey.KindStaging || obj.Kind == blobkey.KindBackupPG {
			return
		}
		rep.OrphansSeen++
		if opts.DryRun {
			rep.OrphansDeleted++
			return
		}
		if r.deleteObject(key, "object mồ côi") {
			rep.OrphansDeleted++
			rep.ObjectsDeleted++
		}
	}); err != nil {
		r.fail("orphan", "", err)
	}

	rep.Errors = r.errorMessages
	logger.Info("storage gc xong",
		"objects_deleted", rep.ObjectsDeleted,
		"blobs_reaped", rep.BlobsReaped,
		"cache_deleted", rep.CacheDeleted,
		"staging_deleted", rep.StagingDeleted,
		"orphans_deleted", rep.OrphansDeleted,
		"out_of_scope", rep.OutOfScope,
		"errors", len(rep.Errors),
	)
	if n := len(rep.Errors); n > 0 {
		return rep, fmt.Errorf("storage gc: %d hành động thất bại (xem log)", n)
	}
	return rep, nil
}

type runner struct {
	ctx           context.Context
	opts          Options
	repo          *postgres.BlobGC
	log           *slog.Logger
	pageSize      int
	cutoff        time.Time
	stagingCutoff time.Time
	deleted       map[string]bool
	errorMessages []string
}

// inScope answers whether this run may delete the object at key: it must parse
// as a key of the active environment and sit under that environment's prefix.
func (r *runner) inScope(key string) bool {
	if !strings.HasPrefix(key, r.opts.Keys.Prefix()) {
		r.log.Warn("bỏ qua khoá ngoài tiền tố môi trường", "key", key)
		return false
	}
	obj, err := blobkey.Parse(key)
	if err != nil || obj.Env != r.opts.Keys.Env() {
		r.log.Warn("bỏ qua khoá không nhận dạng", "key", key, "err", err)
		return false
	}
	return true
}

// deleteObject removes one object. Delete is idempotent, so a missing object is
// a success. Every action is logged.
func (r *runner) deleteObject(key, reason string) bool {
	if err := r.opts.Blobs.Delete(r.ctx, key); err != nil {
		r.fail(reason, key, err)
		return false
	}
	r.deleted[key] = true
	r.log.Info("xoá object", "key", key, "reason", reason)
	return true
}

// fail records one failure for the report and the returned error.
func (r *runner) fail(stage, key string, err error) {
	msg := fmt.Sprintf("%s: %v", stage, err)
	if key != "" {
		msg = fmt.Sprintf("%s %s: %v", stage, key, err)
	}
	r.errorMessages = append(r.errorMessages, msg)
	r.log.Error("gc thất bại", "stage", stage, "key", key, "err", err)
}

// forEachObject pages through one provider listing. The cursor is the provider's
// opaque continuation; a cursor that does not advance aborts instead of looping
// forever on a misbehaving provider.
func (r *runner) forEachObject(prefix string, fn func(blobstore.BlobInfo)) error {
	cursor := ""
	for {
		page, err := r.opts.Blobs.List(r.ctx, prefix, cursor, r.pageSize)
		if err != nil {
			return fmt.Errorf("liệt kê %q: %w", prefix, err)
		}
		for _, info := range page.Infos {
			fn(info)
		}
		if page.NextCursor == "" {
			return nil
		}
		if page.NextCursor == cursor {
			return fmt.Errorf("provider trả cursor không tiến cho %q: %q", prefix, cursor)
		}
		cursor = page.NextCursor
	}
}
