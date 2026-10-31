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
//   - Reference decisions lock and recheck the registry row. A persistent
//     deleting claim excludes publication and attachments until storage deletion
//     succeeds; only then is the row removed. Every crash boundary is resumable.
//   - Grace covers bytes written before publication and asynchronous attachment.
//   - Registry candidates and storage reconciliation use bounded keyset pages.
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
	defaultPageSize   = 1000
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
	MultipartSeen    int      `json:"multipartSeen"`
	MultipartAborted int      `json:"multipartAborted"`
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
	fmt.Fprintf(&b, "  multipart quá hạn: %d thấy / %d huỷ\n", r.MultipartSeen, r.MultipartAborted)
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
	if pageSize <= 0 || pageSize > blobstore.DeleteBatchSize {
		pageSize = defaultPageSize
	}
	if !opts.DryRun {
		unlock, err := repo.LockRun(ctx)
		if err != nil {
			return rep, err
		}
		defer unlock()
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
		report: &rep,
	}

	// Staged rows retain their original age and active-job gate.
	stagedCutoff := started.Add(-stagedAge)
	r.forEachBlob("staged", stagedCutoff, func(blob postgres.GCBlob) {
		rep.StagedSeen++
		if opts.DryRun {
			rep.StagedTombstoned++
			return
		}
		moved, err := repo.TombstoneStaged(ctx, blob.ID, stagedCutoff)
		if err != nil {
			r.fail("staged", blob.ObjectKey, err)
			return
		}
		if moved {
			rep.StagedTombstoned++
		}
	})

	// Previously interrupted deletes and this run's abandoned stages.
	r.forEachBlob("deleting", cutoff, func(blob postgres.GCBlob) {
		rep.DeletingFound++
		if opts.DryRun {
			rep.DeletingReaped++
			return
		}
		res, err := repo.ReapBlob(ctx, blob.ID, cutoff, true)
		if err != nil {
			r.fail("deleting", blob.ObjectKey, err)
			return
		}
		if !res.Deleted {
			return
		}
		rep.DeletingReaped++
		r.enqueue(deleteTask{blob: blob, reason: "tombstone"})
	})

	// Expired references are removed only within the active environment.
	var after postgres.GCExportCacheEntry
	for {
		page, err := repo.ExpiredExportCachePage(ctx, opts.Keys.Prefix(), after, pageSize)
		if err != nil {
			r.fail("export_cache", "", err)
			break
		}
		if len(page) == 0 {
			break
		}
		for _, entry := range page {
			rep.CacheExpired++
			if opts.DryRun {
				rep.CacheDeleted++
				continue
			}
			res, removed, err := repo.ReapExpiredCacheEntry(ctx, entry)
			if err != nil {
				r.fail("export_cache", "", err)
				continue
			}
			if removed {
				rep.CacheDeleted++
			}
			if res.Deleted {
				rep.CacheBlobsReaped++
				if !r.inScope(res.ObjectKey) {
					rep.OutOfScope++
					continue
				}
				r.enqueue(deleteTask{blob: postgres.GCBlob{ID: entry.BlobID, ObjectKey: res.ObjectKey}, reason: "export_cache hết hạn"})
			}
		}
		r.flush()
		after = page[len(page)-1]
	}

	r.forEachBlob("ready", cutoff, func(blob postgres.GCBlob) {
		rep.UnreferencedSeen++
		if opts.DryRun {
			rep.BlobsReaped++
			return
		}
		res, err := repo.ReapBlob(ctx, blob.ID, cutoff, false)
		if err != nil {
			r.fail("blob", blob.ObjectKey, err)
			return
		}
		switch res.Reason {
		case postgres.GCReasonDeleted:
			rep.BlobsReaped++
			r.enqueue(deleteTask{blob: blob, reason: "không tham chiếu"})
		case postgres.GCReasonReferenced:
			rep.BlobsReferenced++
		default:
			rep.BlobsSkipped++
		}
	})

	if err := r.forEachObjectPage(opts.Keys.StagingPrefix(), func(page []blobstore.BlobInfo) {
		for _, info := range page {
			if !r.inScope(info.Key) {
				rep.OutOfScope++
				continue
			}
			rep.StagingSeen++
			if !info.ModTime.Before(stagingCutoff) {
				continue
			}
			if opts.DryRun {
				rep.StagingDeleted++
				continue
			}
			r.enqueue(deleteTask{blob: postgres.GCBlob{ObjectKey: info.Key}, reason: "staging quá hạn", done: func() { rep.StagingDeleted++ }})
		}
		r.flush()
	}); err != nil {
		r.fail("staging", "", err)
	}

	// Reconcile just the current object page. Deleting rows remain registered
	// after failures, so the orphan pass never bypasses their durable claim.
	if err := r.forEachObjectPage(opts.Keys.Prefix(), func(page []blobstore.BlobInfo) {
		keys := make([]string, 0, len(page))
		for _, info := range page {
			keys = append(keys, info.Key)
		}
		known, err := repo.RegisteredKeys(ctx, keys)
		if err != nil {
			r.fail("orphan", "", err)
			return
		}
		registered := make(map[string]bool, len(known))
		for _, key := range known {
			registered[key] = true
		}
		for _, info := range page {
			key := info.Key
			if strings.HasPrefix(key, opts.Keys.StagingPrefix()) || strings.HasPrefix(key, opts.Keys.BackupPrefix()) || registered[key] || !info.ModTime.Before(cutoff) {
				continue
			}
			obj, err := blobkey.Parse(key)
			if err != nil || obj.Env != opts.Keys.Env() {
				rep.UnknownObjects++
				continue
			}
			if obj.Kind == blobkey.KindStaging || obj.Kind == blobkey.KindBackupPG {
				continue
			}
			rep.OrphansSeen++
			if opts.DryRun {
				rep.OrphansDeleted++
				continue
			}
			r.enqueue(deleteTask{blob: postgres.GCBlob{ObjectKey: key}, reason: "object mồ côi", done: func() { rep.OrphansDeleted++ }})
		}
		r.flush()
	}); err != nil {
		r.fail("orphan", "", err)
	}

	// Multipart state is invisible to ListObjects. Scope to the complete env
	// prefix, including final media keys, and never abort on a dry-run.
	if multipart, ok := opts.Blobs.(blobstore.MultipartReaper); ok {
		cursor := ""
		for {
			page, err := multipart.ListMultipartUploads(ctx, opts.Keys.Prefix(), cursor, pageSize)
			if err != nil {
				r.fail("multipart", "", err)
				break
			}
			for _, upload := range page.Uploads {
				if !strings.HasPrefix(upload.Key, opts.Keys.Prefix()) {
					rep.OutOfScope++
					continue
				}
				rep.MultipartSeen++
				if upload.Initiated.IsZero() || !upload.Initiated.Before(cutoff) {
					continue
				}
				if opts.DryRun {
					rep.MultipartAborted++
					continue
				}
				if err := multipart.AbortMultipartUpload(ctx, upload); err != nil {
					r.fail("multipart", upload.Key, err)
					continue
				}
				rep.MultipartAborted++
			}
			if page.NextCursor == "" {
				break
			}
			if page.NextCursor == cursor {
				r.fail("multipart", "", errors.New("multipart cursor did not advance"))
				break
			}
			cursor = page.NextCursor
		}
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
	report        *Report
	pending       []deleteTask
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

type deleteTask struct {
	blob   postgres.GCBlob
	reason string
	done   func()
}

func (r *runner) enqueue(task deleteTask) {
	if !task.blob.ID.Valid {
		claim, claimed, err := r.repo.ClaimOrphan(r.ctx, task.blob.ObjectKey, blobstore.DriverName(r.opts.Blobs))
		if err != nil {
			r.fail("claim orphan", task.blob.ObjectKey, err)
			return
		}
		if !claimed {
			return
		}
		task.blob = claim
	}
	r.pending = append(r.pending, task)
	if len(r.pending) >= blobstore.DeleteBatchSize {
		r.flush()
	}
}

func (r *runner) flush() {
	if len(r.pending) == 0 {
		return
	}
	tasks := r.pending
	r.pending = nil
	keys := make([]string, len(tasks))
	for i, task := range tasks {
		keys[i] = task.blob.ObjectKey
	}
	results, err := blobstore.DeleteMany(r.ctx, r.opts.Blobs, keys)
	if err != nil {
		r.fail("batch delete", "", err)
	}
	for i, result := range results {
		if i >= len(tasks) {
			break
		}
		task := tasks[i]
		if result.Err != nil {
			r.fail(task.reason, result.Key, result.Err)
			continue
		}
		if task.blob.ID.Valid {
			if err := r.repo.FinishBlob(r.ctx, task.blob.ID); err != nil {
				r.fail("finish delete", result.Key, err)
				continue
			}
		}
		r.report.ObjectsDeleted++
		if task.done != nil {
			task.done()
		}
		r.log.Info("xoá object", "key", result.Key, "reason", task.reason)
	}
}

func (r *runner) forEachBlob(state string, cutoff time.Time, fn func(postgres.GCBlob)) {
	cursor := ""
	for {
		page, err := r.repo.ScanBlobs(r.ctx, r.opts.Keys.Prefix(), cursor, state, cutoff, r.pageSize)
		if err != nil {
			r.fail(state, "", err)
			return
		}
		if len(page) == 0 {
			return
		}
		for _, blob := range page {
			if !r.inScope(blob.ObjectKey) {
				r.report.OutOfScope++
				continue
			}
			fn(blob)
		}
		r.flush()
		next := page[len(page)-1].ObjectKey
		if next <= cursor {
			r.fail(state, "", errors.New("registry cursor did not advance"))
			return
		}
		cursor = next
	}
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

// forEachObjectPage keeps provider and registry reconciliation memory bounded.
func (r *runner) forEachObjectPage(prefix string, fn func([]blobstore.BlobInfo)) error {
	cursor := ""
	for {
		page, err := r.opts.Blobs.List(r.ctx, prefix, cursor, r.pageSize)
		if err != nil {
			return fmt.Errorf("liệt kê %q: %w", prefix, err)
		}
		fn(page.Infos)
		if page.NextCursor == "" {
			return nil
		}
		if page.NextCursor == cursor {
			return fmt.Errorf("provider trả cursor không tiến cho %q: %q", prefix, cursor)
		}
		cursor = page.NextCursor
	}
}
