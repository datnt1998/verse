package storageimport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// Options configures one import (or dry-run) of a frozen snapshot.
type Options struct {
	Scan     *ScanResult
	Pool     *postgres.Pool
	Blobs    blobstore.BlobStore
	Driver   string // "local" | "s3"
	DryRun   bool
	Resume   bool
	OwnerMap map[string]string // project slug -> username
	// Keys builds object keys for one STORAGE_ENV. The zero value is dev.
	Keys blobkey.Keys
	// ReadOnlyRoot is opened for blob bytes; it must be the same snapshot the
	// manifest was built from.
	ReadOnlyRoot string
	Now          time.Time
}

// DomainStat is the per-domain accounting shown in the report.
type DomainStat struct {
	Records  int    `json:"records"`
	Inserted int    `json:"inserted"`
	Skipped  int    `json:"skipped"`
	Blobs    int    `json:"blobs"`
	Bytes    int64  `json:"bytes"`
	Digest   string `json:"digest"`
}

// Result is the machine-readable outcome of a run.
type Result struct {
	RunID          string                 `json:"runId"`
	SourceID       string                 `json:"sourceId"`
	ManifestSHA256 string                 `json:"manifestSha256"`
	DryRun         bool                   `json:"dryRun"`
	Status         string                 `json:"status"`
	RowsInserted   int                    `json:"rowsInserted"`
	RowsSkipped    int                    `json:"rowsSkipped"`
	BlobsUploaded  int                    `json:"blobsUploaded"`
	BlobsSkipped   int                    `json:"blobsSkipped"`
	BlobBytes      int64                  `json:"blobBytes"`
	Quarantined    []string               `json:"quarantined"`
	Tolerated      []string               `json:"tolerated"`
	ToleratedCount int                    `json:"toleratedCount"`
	Conflicts      []string               `json:"conflicts"`
	Derived        int                    `json:"derived"`
	Excluded       int                    `json:"excluded"`
	Domains        map[string]*DomainStat `json:"domains"`
	SessionsDrop   int                    `json:"sessionsInvalidated"`
	PairingsDrop   int                    `json:"pairingsInvalidated"`
	Users          int                    `json:"users"`
	Projects       int                    `json:"projects"`
	Stories        int                    `json:"stories"`
	StartedAt      time.Time              `json:"startedAt"`
	FinishedAt     time.Time              `json:"finishedAt"`
}

// Failed reports whether the run must exit nonzero.
func (r *Result) Failed() bool {
	return len(r.Conflicts) > 0 || len(r.Quarantined) > 0 || r.Status == "failed"
}

// RunIDFor derives the run id; the same source+manifest always maps to it.
func RunIDFor(sourceID, manifestSHA string) string {
	return DeterministicUUID(sourceID, "import_run", manifestSHA)
}

type importer struct {
	opts   Options
	scan   *ScanResult
	ids    *DecodeContext
	root   *os.Root
	result *Result
	tx     pgx.Tx
	// synopsis comes from meta/book.json, read in the pre-pass so stories can
	// be inserted with their real synopsis even when the state row lands later.
	synopsis     map[string]string
	owners       map[string]string
	pending      []ledgerRow
	existing     map[string]existingRow
	roundReviews map[string]bool
}

// existingRow is a ledger row already present for this source.
type existingRow struct {
	raw, semantic string
}

func ledgerKey(path, key string) string { return path + "\x00" + key }

// precheck decides whether an item can be skipped because the ledger already
// holds the identical record. A different raw digest means the source changed
// under a frozen manifest: that is a loud conflict, never a silent re-import.
func (im *importer) precheck(f ManifestFile, key string, raw string) (skip bool, conflict string) {
	row, ok := im.existing[ledgerKey(f.Path, key)]
	if !ok {
		return false, ""
	}
	if row.raw != raw {
		return true, fmt.Sprintf("%s: nguồn đã đổi so với lần nhập trước (raw hash khác) — cần manifest mới", f.Path)
	}
	return true, ""
}

// Run performs the import. A dry run classifies and decodes everything and
// reports conflicts without writing a single row or blob.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Scan == nil || opts.Scan.Manifest == nil {
		return nil, errors.New("thiếu manifest nguồn")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if opts.ReadOnlyRoot == "" {
		return nil, errors.New("thiếu ReadOnlyRoot")
	}
	root, err := os.OpenRoot(opts.ReadOnlyRoot)
	if err != nil {
		return nil, fmt.Errorf("mở snapshot nguồn: %w", err)
	}
	defer func() { _ = root.Close() }()

	im := &importer{
		opts: opts, scan: opts.Scan, root: root,
		synopsis: map[string]string{},
		result: &Result{
			RunID:          RunIDFor(opts.Scan.SourceID, opts.Scan.Manifest.SHA256),
			SourceID:       opts.Scan.SourceID,
			ManifestSHA256: opts.Scan.Manifest.SHA256,
			DryRun:         opts.DryRun,
			Domains:        map[string]*DomainStat{},
			StartedAt:      opts.Now,
		},
	}
	if err := im.prepare(ctx); err != nil {
		return im.result, err
	}
	if err := im.exec(ctx); err != nil {
		im.result.Status = "failed"
		if len(im.result.Conflicts) == 0 {
			im.result.Conflicts = append(im.result.Conflicts, err.Error())
		}
		return im.result, err
	}
	im.result.FinishedAt = time.Now().UTC()
	return im.result, nil
}

// prepare builds the deterministic id map. It decodes only the meta files
// (accounts, library.json, project.json, story.json, meta/book.json) — the
// rest is streamed during exec.
func (im *importer) prepare(ctx context.Context) error {
	scan := im.scan
	ids := &DecodeContext{
		SourceID: scan.SourceID,
		Library:  scan.Library,
		Projects: map[string]string{},
		Stories:  map[string]string{},
		Users:    map[string]string{},
	}
	for _, p := range scan.Projects {
		ids.Projects[p.Slug] = DeterministicUUID(scan.SourceID, "project", p.Slug)
	}
	for _, s := range scan.Stories {
		ids.Stories[s.ProjectSlug+"/"+s.Slug] = DeterministicUUID(scan.SourceID, "story", s.ProjectSlug+"/"+s.Slug)
	}
	accountsProj, err := decodeAccountsRaw(scan)
	if err != nil {
		return err
	}
	im.result.SessionsDrop = len(accountsProj.Sessions)
	im.result.PairingsDrop = len(accountsProj.Pairings)
	im.result.Users = len(accountsProj.Users)
	for _, u := range accountsProj.Users {
		ids.Users[u.Username] = DeterministicUUID(scan.SourceID, "user", strings.ToLower(u.Username))
	}
	im.ids = ids
	im.result.Projects = len(scan.Projects)
	im.result.Stories = len(scan.Stories)

	// owner map: exactly one explicit owner per project, never guessed.
	owners := map[string]string{}
	for slug, username := range im.opts.OwnerMap {
		if _, ok := ids.Projects[slug]; !ok {
			return fmt.Errorf("--owner-map trỏ tới dự án không tồn tại: %q", slug)
		}
		if _, ok := ids.Users[username]; !ok {
			return fmt.Errorf("--owner-map trỏ tới tài khoản không tồn tại: %q", username)
		}
		owners[slug] = username
	}
	for _, u := range accountsProj.Users {
		for slug, role := range u.Projects {
			if role != "owner" {
				continue
			}
			if existing, ok := owners[slug]; ok && existing != u.Username {
				im.result.Conflicts = append(im.result.Conflicts,
					fmt.Sprintf("dự án %q có hơn một owner (nguồn: %q, owner-map: %q) — cần bảng sửa tay", slug, u.Username, existing))
				continue
			}
			owners[slug] = u.Username
		}
	}
	for _, p := range scan.Projects {
		if _, ok := owners[p.Slug]; !ok {
			im.result.Conflicts = append(im.result.Conflicts,
				fmt.Sprintf("dự án %q không có owner xác định — thêm --owner-map %s=<username>", p.Slug, p.Slug))
		}
	}
	// A reviews/<n>.json file is the latest-round projection; when the
	// per-round file exists it is the authority and the projection is not
	// imported twice.
	im.roundReviews = map[string]bool{}
	for _, f := range scan.Manifest.Files {
		if f.Role != "review_round" {
			continue
		}
		base := strings.TrimSuffix(path.Base(f.Path), ".json")
		if n, _, ok := strings.Cut(base, "-r"); ok {
			im.roundReviews[im.storyDirOf(f.Path)+"/"+n] = true
		}
	}
	// synopsis pre-pass from meta/book.json.
	for _, f := range scan.Manifest.Files {
		if f.Role != "engine_book" {
			continue
		}
		raw, err := readFileFS(rootFS{root: im.root}, f.Path)
		if err != nil {
			return fmt.Errorf("đọc %s: %w", f.Path, err)
		}
		var book struct {
			Title    string `json:"title"`
			Synopsis string `json:"synopsis"`
		}
		if err := jsonUnmarshal(raw, &book); err != nil {
			return fmt.Errorf("%s hỏng: %w", f.Path, err)
		}
		for _, s := range scan.Stories {
			if strings.HasPrefix(f.Path, s.Dir+"/") {
				im.synopsis[s.ProjectSlug+"/"+s.Slug] = book.Synopsis
			}
		}
	}
	im.owners = owners
	return nil
}

func decodeAccountsRaw(scan *ScanResult) (AccountFile, error) {
	var file AccountFile
	if len(scan.AccountsRaw) == 0 {
		return file, nil
	}
	if err := jsonUnmarshal(scan.AccountsRaw, &file); err != nil {
		return file, fmt.Errorf("web-accounts.json hỏng: %w", err)
	}
	return file, nil
}

func (im *importer) exec(ctx context.Context) error {
	// Dry run never touches the database.
	if im.opts.DryRun {
		return im.walk(ctx, nil)
	}
	pool := im.opts.Pool
	if pool == nil {
		return errors.New("thiếu kết nối Postgres")
	}
	conn, err := pool.Pool().Acquire(ctx)
	if err != nil {
		return fmt.Errorf("lấy kết nối: %w", err)
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey(im.scan.SourceID)).Scan(&locked); err != nil {
		return fmt.Errorf("khoá import: %w", err)
	}
	if !locked {
		return errors.New("đã có importer khác chạy cho nguồn này")
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey(im.scan.SourceID))
	}()

	runID := im.result.RunID
	_, err = conn.Exec(ctx, `
INSERT INTO import_runs (id, source_id, manifest_sha256, status, importer_version, source_label, started_at)
VALUES ($1, $2, $3, 'running', $4, $5, $6)
ON CONFLICT (id) DO NOTHING`,
		uuidArg(runID), uuidArg(im.scan.SourceID), im.scan.Manifest.SHA256,
		ImporterVersion, im.scan.Manifest.SourceLabel, im.result.StartedAt)
	if err != nil {
		return fmt.Errorf("tạo import_run: %w", err)
	}

	rows, err := loadLedgerRows(ctx, conn, im.scan.SourceID)
	if err != nil {
		return err
	}
	im.existing = rows
	if err := im.walk(ctx, pool); err != nil {
		_, _ = conn.Exec(context.Background(), `
UPDATE import_runs SET status='failed', error=$2, updated_at=now(), version=version+1 WHERE id=$1`,
			uuidArg(runID), truncate(err.Error(), 2000))
		return err
	}
	if im.result.Conflicts != nil || len(im.result.Quarantined) > 0 {
		_, _ = conn.Exec(context.Background(), `
UPDATE import_runs SET status='failed', summary=$2, updated_at=now(), version=version+1 WHERE id=$1`,
			uuidArg(runID), mustJSON(im.result))
		return fmt.Errorf("import dừng: %d xung đột, %d đường dẫn bị cách ly",
			len(im.result.Conflicts), len(im.result.Quarantined))
	}
	_, err = conn.Exec(ctx, `
UPDATE import_runs SET status='verified', summary=$2, verified_at=now(), finished_at=now(), updated_at=now(), version=version+1 WHERE id=$1`,
		uuidArg(runID), mustJSON(im.result))
	if err != nil {
		return fmt.Errorf("chốt import_run: %w", err)
	}
	return nil
}

// walk runs the manifest in path order. Blobs and facts share one pass so a
// partial run resumes at the first missing ledger row.
func (im *importer) walk(ctx context.Context, pool *postgres.Pool) error {
	var deferred []deferredItem
	// Phase 1: identity and container rows (accounts, library, projects,
	// stories). Everything else references them by foreign key, so the order is
	// not cosmetic — it is the FK order.
	firstPhase := map[string]bool{
		"library_config": true, "accounts": true, "project_meta": true, "story_meta": true,
		"chapter_record": true,
	}
	// Ordered sub-phases: library + accounts, then projects, then stories. The
	// path order alone is not the FK order.
	for _, roles := range [][]string{
		{"library_config", "accounts"},
		{"project_meta"},
		{"story_meta"},
		{"chapter_record"},
	} {
		want := map[string]bool{}
		for _, role := range roles {
			want[role] = true
		}
		for _, f := range im.scan.Manifest.Files {
			if !want[f.Role] {
				continue
			}
			if err := im.processFile(ctx, pool, f, &deferred); err != nil {
				return err
			}
		}
	}
	// Phase 2: domains, in path order, plus blobs.
	for _, f := range im.scan.Manifest.Files {
		if firstPhase[f.Role] {
			continue
		}
		if err := im.processFile(ctx, pool, f, &deferred); err != nil {
			return err
		}
	}
	// Deferred: chapter prose (needs chapters), memberships (needs projects).
	if err := im.applyMemberships(ctx, pool); err != nil {
		return err
	}
	for _, d := range deferred {
		if err := im.apply(ctx, pool, d.file, d.item, d.scope); err != nil {
			return err
		}
	}
	if !im.opts.DryRun && len(im.pending) > 0 {
		return postgres.RunInTx(ctx, pool.Pool(), func(tx pgx.Tx) error { return im.flush(ctx, tx) })
	}
	return nil
}

// loadLedgerRows reads the ledger already stored for this source.
func loadLedgerRows(ctx context.Context, q querier, sourceID string) (map[string]existingRow, error) {
	rows, err := q.Query(ctx, `SELECT relative_path, record_key, raw_sha256, semantic_sha256 FROM import_items WHERE source_id=$1`,
		uuidArg(sourceID))
	if err != nil {
		return nil, fmt.Errorf("đọc import_items: %w", err)
	}
	defer rows.Close()
	out := map[string]existingRow{}
	for rows.Next() {
		var path, key, raw, semantic string
		if err := rows.Scan(&path, &key, &raw, &semantic); err != nil {
			return nil, err
		}
		out[ledgerKey(path, key)] = existingRow{raw: raw, semantic: semantic}
	}
	return out, rows.Err()
}

// processFile classifies one manifest entry and applies its items.
func (im *importer) processFile(ctx context.Context, pool *postgres.Pool, f ManifestFile, deferred *[]deferredItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	scope := im.scopeOf(f.Path)
	switch f.Kind {
	case KindBlob:
		return im.importBlob(ctx, pool, f, scope)
	case KindDerived:
		im.recordDisposition(f, DispDerived, f.Reason)
		im.result.Derived++
		return nil
	case KindScratch, KindLocal, KindExclude:
		im.recordDisposition(f, DispExcluded, firstNonEmpty(f.Reason, "ngoài phạm vi dữ liệu nghiệp vụ"))
		im.result.Excluded++
		return nil
	}
	data := im.scan.AccountsRaw
	var err error
	synthetic := false
	if syn, ok := im.scan.Synthetic[f.Path]; ok {
		data, synthetic = syn, true
	} else if f.Path != PathAccounts {
		data, err = readFileFS(rootFS{root: im.root}, f.Path)
	}
	if err != nil || (f.Path == PathAccounts && len(data) == 0) {
		if f.Path == PathAccounts {
			return fmt.Errorf("đọc %s: %w", f.Path, err)
		}
		im.ledger(f, "", 0, DispQuarantined, "", "", f.SHA256, f.SHA256, "không đọc được: "+scrubError(err))
		im.result.Quarantined = append(im.result.Quarantined, f.Path+" (không đọc được)")
		return nil
	}
	// The snapshot must be frozen: a file that changed after the scan (or a
	// synthetic file whose bytes disagree with the manifest) is a loud
	// quarantine, never a silent import under the old hash.
	if !synthetic && f.Path != PathAccounts && (int64(len(data)) != f.Size || RawDigest(data) != f.SHA256) {
		reason := "nội dung đổi giữa lúc quét và lúc nhập (snapshot không đóng băng)"
		im.ledger(f, "", int64(len(data)), DispQuarantined, "", "", f.SHA256, f.SHA256, reason)
		im.result.Quarantined = append(im.result.Quarantined, f.Path+" ("+reason+")")
		return nil
	}
	if f.Role == "review_latest" {
		base := strings.TrimSuffix(path.Base(f.Path), ".json")
		if im.roundReviews[im.storyDirOf(f.Path)+"/"+base] {
			im.recordDisposition(f, DispDerived, "bản chiếu review mới nhất từ reviews/rounds/")
			im.result.Derived++
			return nil
		}
	}
	items, err := DecodeFile(im.ids, im.storyRefFor(f.Path), f.Role, f.Path, data, im.modTime(f.Path))
	if err != nil {
		im.ledger(f, "", 0, DispQuarantined, "", "", f.SHA256, f.SHA256, err.Error())
		im.result.Quarantined = append(im.result.Quarantined, f.Path+" ("+err.Error()+")")
		return nil
	}
	if len(items) == 0 {
		// A classified file that yields no record (empty character list, empty
		// comment file…) still gets its ledger row: every path is accounted for.
		semantic, _ := SemanticDigest(map[string]string{"path": f.Path, "role": f.Role})
		im.ledger(f, "", f.Size, DispImported, "", "", f.SHA256, semantic, "file không sinh bản ghi miền")
		return nil
	}
	for _, item := range items {
		if item.Kind == "story" {
			if row, ok := item.Payload.(StoryRow); ok {
				row.Synopsis = im.synopsis[row.ProjectID+"/"+row.Slug]
				item.Payload = row
			}
		}
		if item.Disposition == DispTolerated {
			im.ledger(f, item.Key, f.Size, DispTolerated, "", "", f.SHA256, f.SHA256, item.Reason)
			im.result.Tolerated = append(im.result.Tolerated, f.Path+": "+item.Reason)
			im.result.ToleratedCount++
			continue
		}
		if item.Disposition == DispQuarantined {
			im.ledger(f, item.Key, 0, DispQuarantined, "", "", f.SHA256, f.SHA256, item.Reason)
			im.result.Quarantined = append(im.result.Quarantined, f.Path+": "+item.Reason)
			continue
		}
		if item.Kind == "chapter_prose" || item.Kind == "chapter_draft" {
			*deferred = append(*deferred, deferredItem{file: f, item: item, scope: scope})
			continue
		}
		if err := im.apply(ctx, pool, f, item, scope); err != nil {
			return fmt.Errorf("%s: %w", f.Path, err)
		}
	}
	return nil
}

type deferredItem struct {
	file  ManifestFile
	item  Item
	scope recordScope
}

type recordScope struct {
	ProjectID string
	StoryID   string
}

// modTime returns the source file modification time, used as the accepted_at
// of an imported chapter version so the value is reproducible from the frozen
// snapshot instead of the import clock.
func (im *importer) modTime(rel string) time.Time {
	info, err := im.root.Stat(rel)
	if err != nil {
		return time.Unix(0, 0).UTC()
	}
	return info.ModTime().UTC()
}

func (im *importer) scopeOf(rel string) recordScope {
	for _, s := range im.scan.Stories {
		if strings.HasPrefix(rel, s.Dir+"/") {
			return recordScope{
				ProjectID: im.ids.Projects[s.ProjectSlug],
				StoryID:   im.ids.Stories[s.ProjectSlug+"/"+s.Slug],
			}
		}
	}
	for _, p := range im.scan.Projects {
		if p.Dir != "" && strings.HasPrefix(rel, p.Dir+"/") {
			return recordScope{ProjectID: im.ids.Projects[p.Slug]}
		}
	}
	return recordScope{}
}

// storyDirOf returns the story directory a path belongs to ("" when none).
func (im *importer) storyDirOf(rel string) string {
	for _, s := range im.scan.Stories {
		if strings.HasPrefix(rel, s.Dir+"/") {
			return s.Dir
		}
	}
	return ""
}

func (im *importer) storyRefFor(rel string) StoryRef {
	for _, s := range im.scan.Stories {
		if strings.HasPrefix(rel, s.Dir+"/") {
			return s
		}
	}
	for _, p := range im.scan.Projects {
		if p.Dir != "" && strings.HasPrefix(rel, p.Dir+"/") {
			return StoryRef{ProjectSlug: p.Slug}
		}
	}
	return StoryRef{}
}

// apply writes one item in its own bounded transaction and records the ledger
// row in the same transaction, so a killed importer resumes consistently.
func (im *importer) apply(ctx context.Context, pool *postgres.Pool, f ManifestFile, item Item, scope recordScope) error {
	semantic, err := ItemDigest(item.Kind, item.Payload)
	if err != nil {
		return fmt.Errorf("%s: %w", f.Path, err)
	}
	if skip, conflict := im.precheck(f, item.Key, f.SHA256); skip {
		if conflict != "" {
			im.result.Conflicts = append(im.result.Conflicts, conflict)
			return nil
		}
		stat := im.stat(item.Domain)
		stat.Records++
		stat.Skipped++
		im.result.RowsSkipped++
		return nil
	}
	inserted, conflict, err := im.writeItem(ctx, pool, item, scope, semantic)
	if err != nil {
		return err
	}
	if conflict != "" {
		im.result.Conflicts = append(im.result.Conflicts, conflict)
		return nil
	}
	stat := im.stat(item.Domain)
	stat.Records++
	stat.Digest = DigestOfStrings([]string{stat.Digest, semantic})
	if inserted {
		stat.Inserted++
		im.result.RowsInserted++
	} else {
		stat.Skipped++
		im.result.RowsSkipped++
	}
	im.ledger(f, item.Key, f.Size, DispImported, item.Kind, item.TargetID, f.SHA256, semantic, "")
	return nil
}

func (im *importer) stat(domain string) *DomainStat {
	if im.result.Domains[domain] == nil {
		im.result.Domains[domain] = &DomainStat{}
	}
	return im.result.Domains[domain]
}

func (im *importer) recordDisposition(f ManifestFile, disp, reason string) {
	im.ledger(f, "", f.Size, disp, "", "", f.SHA256, f.SHA256, reason)
}

// ledger records one import_items row. Path+record key is unique, so a rerun
// with identical bytes inserts nothing; different bytes are a drift conflict.
func (im *importer) ledger(f ManifestFile, key string, size int64, disp, kind, targetID, raw, semantic, reason string) {
	if im.opts.DryRun {
		return
	}
	if _, ok := im.existing[ledgerKey(f.Path, key)]; ok {
		return
	}
	im.pending = append(im.pending, ledgerRow{
		runID: im.result.RunID, sourceID: im.scan.SourceID, domain: f.Domain,
		path: f.Path, key: key, disp: disp, kind: kind, targetID: targetID,
		raw: raw, semantic: semantic, size: size, reason: truncate(reason, 1000),
	})
}

func (im *importer) flush(ctx context.Context, tx pgx.Tx) error {
	if len(im.pending) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, row := range im.pending {
		batch.Queue(`
INSERT INTO import_items (run_id, source_id, domain, relative_path, record_key, disposition,
    target_kind, target_id, raw_sha256, semantic_sha256, size_bytes, reason)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT DO NOTHING`,
			uuidArg(row.runID), uuidArg(row.sourceID), row.domain, row.path, row.key, row.disp,
			row.kind, row.targetID, row.raw, row.semantic, row.size, row.reason)
	}
	results := tx.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for range im.pending {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("ghi import_items: %w", err)
		}
	}
	im.pending = im.pending[:0]
	return nil
}

type ledgerRow struct {
	runID, sourceID, domain, path, key, disp, kind, targetID, raw, semantic, reason string
	size                                                                            int64
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ── blob import ──────────────────────────────────────────────────────────────

func (im *importer) importBlob(ctx context.Context, pool *postgres.Pool, f ManifestFile, scope recordScope) error {
	if scope.ProjectID == "" {
		return fmt.Errorf("%s: blob ngoài dự án", f.Path)
	}
	if skip, conflict := im.precheck(f, "", f.SHA256); skip {
		if conflict != "" {
			im.result.Conflicts = append(im.result.Conflicts, conflict)
			return nil
		}
		stat := im.stat(DomainAsset)
		if f.Domain == DomainAudio {
			stat = im.stat(DomainAudio)
		}
		stat.Records++
		stat.Skipped++
		im.result.BlobsSkipped++
		return nil
	}
	objectKey, err := BlobObjectKey(im.opts.Keys, scope.ProjectID, scope.StoryID, f.SHA256, f.Path)
	if err != nil {
		return fmt.Errorf("%s: dựng khoá đối tượng: %w", f.Path, err)
	}
	spec := &BlobSpec{
		Path: f.Path, SHA256: f.SHA256, Size: f.Size, ContentType: ContentType(f.Path),
		ProjectID: scope.ProjectID, StoryID: scope.StoryID, ObjectKey: objectKey, Role: f.Role,
	}
	blobID := DeterministicUUID(im.scan.SourceID, "blob", objectKey)
	blobID, inserted, conflict, err := im.writeBlob(ctx, pool, spec, blobID)
	if err != nil {
		return err
	}
	if conflict != "" {
		im.result.Conflicts = append(im.result.Conflicts, conflict)
		return nil
	}
	stat := im.stat(DomainAsset)
	if f.Domain == DomainAudio {
		stat = im.stat(DomainAudio)
	}
	stat.Records++
	stat.Blobs++
	if inserted {
		stat.Inserted++
		im.result.BlobsUploaded++
		im.result.BlobBytes += f.Size
	} else {
		stat.Skipped++
		im.result.BlobsSkipped++
	}
	stat.Digest = DigestOfStrings([]string{stat.Digest, f.SHA256})
	im.ledger(f, "", f.Size, DispBlob, "blob", blobID, f.SHA256, f.SHA256, "")
	return nil
}

// writeBlob uses the shared verified publication/deletion protocol. New
// imports retain their deterministic UUID; exact duplicates retain the stored
// identity, which the caller records in the import ledger.
func (im *importer) writeBlob(ctx context.Context, pool *postgres.Pool, spec *BlobSpec, blobID string) (string, bool, string, error) {
	if im.opts.DryRun {
		return blobID, true, "", nil
	}
	var alreadyReady bool
	if err := pool.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blobs WHERE object_key=$1 AND state='ready')`, spec.ObjectKey).Scan(&alreadyReady); err != nil {
		return "", false, "", fmt.Errorf("kiểm tra blob %s: %w", spec.Path, err)
	}
	data, err := readFileFS(rootFS{root: im.root}, spec.Path)
	if err != nil {
		return "", false, "", fmt.Errorf("đọc blob %s: %w", spec.Path, err)
	}
	if RawDigest(data) != spec.SHA256 {
		return "", false, "", fmt.Errorf("blob %s: nội dung đổi giữa lúc quét và lúc nhập (snapshot không đóng băng)", spec.Path)
	}
	if im.opts.Blobs == nil {
		return "", false, "", errors.New("thiếu BlobStore")
	}
	id, err := postgres.ParseUUID(blobID)
	if err != nil {
		return "", false, "", err
	}
	project, err := postgres.ParseUUID(spec.ProjectID)
	if err != nil {
		return "", false, "", err
	}
	published, err := postgres.UploadAndPublishBlob(ctx, postgres.NewAudio(pool), im.opts.Blobs, postgres.PublishInput{
		ID: id, ProjectID: project, Key: spec.ObjectKey, Driver: im.opts.Driver, ContentType: spec.ContentType, SHA256: spec.SHA256, Size: spec.Size,
	}, bytes.NewReader(data))
	if errors.Is(err, postgres.ErrBlobContentMismatch) {
		return "", false, fmt.Sprintf("blob %s đã tồn tại với nội dung khác (nguồn đã đổi) — không ghi đè", spec.Path), nil
	}
	if err != nil {
		return "", false, "", fmt.Errorf("tải blob %s: %w", spec.Path, err)
	}
	if err := postgres.RunInTx(ctx, pool.Pool(), func(tx pgx.Tx) error { return im.flush(ctx, tx) }); err != nil {
		return "", false, "", err
	}
	return postgres.UUIDString(published.ID), !alreadyReady, "", nil
}

// applyMemberships creates exactly one owner row per project from the explicit
// owner map plus every explicit non-owner grant in the accounts file.
func (im *importer) applyMemberships(ctx context.Context, pool *postgres.Pool) error {
	if im.opts.DryRun {
		return nil
	}
	accounts, err := decodeAccountsRaw(im.scan)
	if err != nil {
		return err
	}
	type grant struct{ project, username, role string }
	var grants []grant
	for _, u := range accounts.Users {
		for slug, role := range u.Projects {
			if role != "owner" {
				grants = append(grants, grant{slug, u.Username, role})
			}
		}
	}
	for slug, username := range im.owners {
		grants = append(grants, grant{slug, username, "owner"})
	}
	sort.Slice(grants, func(i, j int) bool {
		if grants[i].project != grants[j].project {
			return grants[i].project < grants[j].project
		}
		return grants[i].username < grants[j].username
	})
	for _, g := range grants {
		projectID, ok := im.ids.Projects[g.project]
		if !ok {
			continue
		}
		userID, ok := im.ids.Users[g.username]
		if !ok {
			return fmt.Errorf("quyền dự án %q trỏ tới tài khoản lạ %q", g.project, g.username)
		}
		err := postgres.RunInTx(ctx, pool.Pool(), func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
INSERT INTO project_members (project_id, user_id, role) VALUES ($1,$2,$3)
ON CONFLICT (project_id, user_id) DO NOTHING`,
				uuidArg(projectID), uuidArg(userID), g.role)
			return err
		})
		if err != nil {
			return fmt.Errorf("quyền %s/%s: %w", g.project, g.username, err)
		}
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func uuidArg(s string) pgtype.UUID {
	var u pgtype.UUID
	if s == "" {
		return u
	}
	_ = u.Scan(s)
	return u
}

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func strconvInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

var _ = fs.ErrNotExist
