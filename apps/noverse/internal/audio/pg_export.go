package audio

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// Hosted audio export (phase 07): the web layer enqueues a durable
// audio_export job; PGExportRunner — an in-process ffmpeg runner on the VPS —
// claims it, materializes the locked inputs (scripts, selected render WAVs,
// settings, cast, voices) into a private scratch layout, reuses the
// file-backend export pipeline, publishes every produced file as an immutable
// blob, and attaches the export_cache manifest inside the job-completion
// transaction. Identical inputs (content key) short-circuit ffmpeg entirely.

// pgExportInput is the frozen snapshot an audio_export job carries.
type pgExportInput struct {
	Volume      int             `json:"volume"`
	Chapters    []int           `json:"chapters"`
	Options     ExportOptions   `json:"options"`
	Source      []SourceChapter `json:"source"`
	CacheKey    string          `json:"cacheKey"`
	BookTitle   string          `json:"bookTitle"`
	ShortTitle  string          `json:"shortTitle"`
	Engine      string          `json:"engine"`
	Model       string          `json:"model"`
	ProjectSlug string          `json:"projectSlug"`
	StorySlug   string          `json:"storySlug"`
}

// pgExportManifest is the output manifest stored on the export_cache row.
type pgExportManifest struct {
	ContentKey   string        `json:"contentKey"`
	Volume       int           `json:"volume"`
	Files        []ExportFile  `json:"files"`
	ChapterMarks []ChapterMark `json:"chapterMarks"`
	DurationSec  float64       `json:"durationSec"`
	ExportedAt   time.Time     `json:"exportedAt"`
	Split        bool          `json:"split,omitempty"`
}

// exportCacheKind maps export options to the export_cache kind (the artifact
// format — the schema constrains the column to the produced file types).
func exportCacheKind(opts ExportOptions) string {
	if opts.Pack == "zip" {
		return "zip"
	}
	switch opts.Format {
	case "mp3", "m4b", "wav":
		return opts.Format
	default:
		return "mp3"
	}
}

// exportWorkerName is the durable worker identity the in-process runner
// claims jobs with. The row exists so the jobs.worker_id FK holds; its
// capabilities list satisfies the schema's ['tts'] constraint and is
// documented in the row's name rather than by a new capability.
const exportWorkerName = "vps-ffmpeg-export"

// jobKindExport is the public queue label for an export job (Kind on
// audio.Job is free-form; Gói AK kinds apply to chapter renders).
const jobKindExport = "export"

// pgContentKey hashes everything an export depends on: the selected chapters
// with their script revisions and selected render keys, cast, settings,
// voices, options and the frozen engine/model identity. Any input change
// produces a different key; identical inputs hit the cache.
func (s *PGService) exportContentKey(ctx context.Context, scope postgres.ScopeRef, source []SourceChapter, opts ExportOptions, engine, model string) (string, error) {
	h := sha256.New()
	w := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	w("audio_export", "v1", engine, model)
	optsJSON, _ := json.Marshal(opts)
	w(string(optsJSON))
	cast, _, err := s.loadCastFor(ctx, scope)
	if err != nil {
		return "", err
	}
	castJSON, _ := json.Marshal(cast)
	w(string(castJSON))
	doc, ver, err := s.docFor(ctx, scope)
	if err != nil {
		return "", err
	}
	settingsJSON, _ := json.Marshal(doc.Settings)
	w(string(settingsJSON), fmt.Sprint(ver))
	voices, err := s.deps.Audio.ListVoices(ctx, scope.ProjectID)
	if err != nil {
		return "", pgErr(err)
	}
	for _, v := range voices {
		w(v.LegacyID, postgres.UUIDString(v.SampleBlobID), fmt.Sprint(v.Version))
	}
	for _, sc := range source {
		if len(opts.Chapters) > 0 {
			sel := false
			for _, c := range opts.Chapters {
				if c == sc.Chapter {
					sel = true
				}
			}
			if !sel {
				continue
			}
		}
		ref, err := s.chapterRef(ctx, scope, sc.Chapter)
		if err != nil {
			return "", err
		}
		row, err := s.deps.Audio.LatestScript(ctx, scope.ProjectID, ref.ID)
		if err != nil {
			return "", pgErr(err)
		}
		renders, err := s.deps.Audio.Renders(ctx, scope.ProjectID, ref.ID, row.RevisionNo)
		if err != nil {
			return "", pgErr(err)
		}
		w(fmt.Sprint(sc.Chapter), fmt.Sprint(row.RevisionNo), row.SourceSHA256)
		ids := make([]string, 0, len(renders))
		for _, r := range renders {
			if r.Selected {
				ids = append(ids, r.SegmentID+":"+r.SynthKey+":"+postgres.UUIDString(r.BlobID))
			}
		}
		sort.Strings(ids)
		w(ids...)
	}
	// Tư liệu (bìa/nhạc) feed the same engine paths on scratch — a changed
	// cover or music bed must produce a different content key.
	w(hex.EncodeToString(s.assetDocsFingerprint(ctx, scope.ProjectID)))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// StartExport validates the request like the file backend, freezes the input
// snapshot and enqueues a durable audio_export job. Duplicate requests while
// one is queued/running surface as the existing busy conflict.
func (s *PGService) StartExport(ctx context.Context, volume int, bookTitle string, source []SourceChapter, opts ExportOptions) error {
	selected, chapters, opts, err := normalizeExportOptions(source, volume, opts)
	if err != nil {
		return err
	}
	if opts.WhenReady {
		return pgUnsupported("xuất khi đủ giọng")
	}
	scope, err := s.resolve(ctx)
	if err != nil {
		return pgErr(err)
	}
	// Busy check first: an active export job for this volume conflicts.
	jobs, err := s.queueSnapshot(ctx)
	if err != nil {
		return err
	}
	attempts := 0
	for _, j := range jobs {
		if j.Kind != jobKindExport || j.Volume != volume {
			continue
		}
		attempts++
		if j.State == JobQueued || j.State == JobRendering || j.State == JobPaused {
			return &ExportBusyError{Volume: volume}
		}
	}
	engine, model := s.engineIdent()
	title, short := bookTitle, ""
	if s.storyTitle != "" {
		title = s.storyTitle
	}
	short = s.storyShort
	if short == "" {
		short = title
	}
	key, err := s.exportContentKey(ctx, scope, selected, opts, engine, model)
	if err != nil {
		return err
	}
	in := pgExportInput{
		Volume: volume, Chapters: chapters, Options: opts, Source: selected,
		CacheKey: key, BookTitle: title, ShortTitle: short,
		Engine: engine, Model: model,
		ProjectSlug: s.deps.Project, StorySlug: s.deps.Story,
	}
	_, err = s.deps.Jobs.Enqueue(ctx, scope, postgres.JobAudioExport,
		fmt.Sprintf("audio-export-v%d-%s-%d", volume, key[:16], attempts), in, nil, pgtype.UUID{})
	if err != nil {
		return pgErr(err)
	}
	s.hub.Publish(NewEvent(EventExport, map[string]any{"volume": volume, "state": JobQueued}))
	return nil
}

// ExportInfo lists produced exports for one volume from the export_cache
// manifests (newest manifest wins for the summary fields; every file of every
// manifest is listed, mirroring the file backend's directory listing).
func (s *PGService) ExportInfo(ctx context.Context, volume int) (ExportInfo, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return ExportInfo{}, pgErr(err)
	}
	rows, err := s.deps.Cache.List(ctx, scope)
	if err != nil {
		return ExportInfo{}, pgErr(err)
	}
	out := ExportInfo{Volume: volume, Files: []ExportFile{}, ChapterFiles: []ExportFile{}}
	var newest *pgExportManifest
	for _, row := range rows {
		switch row.Kind {
		case "mp3", "m4b", "wav", "zip":
		default:
			continue
		}
		var m pgExportManifest
		if err := json.Unmarshal(row.InputManifest, &m); err != nil || m.Volume != volume {
			continue
		}
		for i := range m.Files {
			f := m.Files[i]
			if f.URL == "" && f.Path != "" {
				f.URL = s.exportFileURL(f.Path)
			}
			out.Files = append(out.Files, f)
			out.TotalBytes += f.SizeBytes
		}
		for i := range m.ChapterMarks {
			out.ChapterMarks = append(out.ChapterMarks, m.ChapterMarks[i])
		}
		if newest == nil || m.ExportedAt.After(newest.ExportedAt) {
			cp := m
			newest = &cp
		}
	}
	if newest != nil {
		at := newest.ExportedAt
		out.ExportedAt = &at
		out.DurationSec = newest.DurationSec
		for _, f := range newest.Files {
			if f.Format == "audio" {
				out.Format = filepath.Ext(f.Name)
				out.Format = strings.TrimPrefix(out.Format, ".")
			}
		}
	}
	return out, nil
}

// exportFileURL is the blob-stream URL for a produced object key.
func (s *PGService) exportFileURL(key string) string {
	return "/api/blobs/stream/" + key
}

// ── runner ──────────────────────────────────────────────────────────────────

// PGExportRunner claims audio_export jobs of one project and runs them on the
// VPS. It is the hosted ffmpeg worker: pull-only through the jobs table, no
// listener, bounded by the job lease.
type PGExportRunner struct {
	deps     PGDeps
	workerID pgtype.UUID
}

// NewPGExportRunner registers (or reuses) the durable export worker identity
// and returns the runner. The worker row carries a random token hash — it is
// never used to authenticate, the job claim is internal.
func NewPGExportRunner(ctx context.Context, deps PGDeps) (*PGExportRunner, error) {
	if _, err := configuredBlobCache(); err != nil {
		return nil, err
	}
	workers, err := deps.Jobs.ListWorkers(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	for _, w := range workers {
		if w.Name == exportWorkerName && w.RevokedAt == nil {
			deps.WorkerID = w.ID
			return &PGExportRunner{deps: deps, workerID: w.ID}, nil
		}
	}
	var hash [32]byte
	if _, err := rand.Read(hash[:]); err != nil {
		return nil, err
	}
	w, err := deps.Jobs.RegisterWorker(ctx, exportWorkerName, hash[:], []string{"tts"})
	if err != nil {
		return nil, pgErr(err)
	}
	deps.WorkerID = w.ID
	return &PGExportRunner{deps: deps, workerID: w.ID}, nil
}

// RunOnce claims at most one audio_export job and processes it to
// completion (jobs carry their own story scope, so the runner serves the
// whole installation). Returns true when a job ran.
func (r *PGExportRunner) RunOnce(ctx context.Context) (bool, error) {
	job, err := r.deps.Jobs.Claim(ctx, postgres.JobAudioExport, r.workerID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		if errors.Is(err, postgres.ErrNoClaimableJob) {
			return false, nil
		}
		return false, pgErr(err)
	}
	lease := postgres.WorkerLease{
		JobKey:   postgres.JobKey{ProjectID: job.ProjectID, ID: job.ID},
		WorkerID: r.workerID,
		Fence:    job.FencingToken,
	}
	svc := NewPGService(r.deps)
	defer svc.Close()
	if err := svc.processExportJob(ctx, job); err != nil {
		if errors.Is(err, postgres.ErrJobLeaseLost) {
			// Fence moved between the last heartbeat and completion; the job
			// is gone from this worker either way — abandoned, not failed.
			return true, nil
		}
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		if _, ferr := r.deps.Jobs.Fail(ctx, lease, map[string]any{"error": err.Error()}, nil); ferr != nil {
			return true, fmt.Errorf("job failed (%v) and fail-report failed: %w", err, ferr)
		}
		return true, nil
	}
	return true, nil
}

// Run polls RunOnce until ctx is done; a quiet period idles for idle between
// claims.
func (r *PGExportRunner) Run(ctx context.Context, idle time.Duration) error {
	if idle <= 0 {
		idle = 5 * time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ran, err := r.RunOnce(ctx)
		if err != nil {
			// Real runner failures (claim/DB/reporting) are logged and
			// retried after the idle backoff; only parent-context
			// cancellation stops the loop permanently.
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Error("export runner lỗi — thử lại sau backoff", "module", "audio-export", "err", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(idle):
				continue
			}
		}
		if !ran {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(idle):
			}
		}
	}
}

// exportHeartbeatInterval is the runner's lease-renewal cadence — a var only
// so runner-level tests can shorten it without waiting the real 30 seconds.
var exportHeartbeatInterval = postgres.DefaultHeartbeat

// exportHeartbeat renews the claimed lease — a var so tests can gate each
// beat deterministically instead of racing a real 30-second ticker.
var exportHeartbeat = func(ctx context.Context, deps PGDeps, lease postgres.WorkerLease) error {
	_, err := deps.Jobs.Heartbeat(ctx, lease, postgres.DefaultLease)
	return err
}

// processExportJob executes one claimed audio_export job.
func (s *PGService) processExportJob(ctx context.Context, job postgres.Job) (resultErr error) {
	var in pgExportInput
	if err := json.Unmarshal(job.Input, &in); err != nil {
		return fmt.Errorf("audio_export input hỏng: %w", err)
	}
	scope := postgres.ScopeRef{ProjectID: job.ProjectID, StoryID: job.StoryID, StorySlug: in.StorySlug}
	lease := postgres.WorkerLease{
		JobKey:   postgres.JobKey{ProjectID: job.ProjectID, ID: job.ID},
		WorkerID: s.deps.WorkerID,
		Fence:    job.FencingToken,
	}
	completionCtx := ctx
	ctx, stopHeartbeat := maintainExportLease(ctx, exportHeartbeatInterval, func(heartbeatCtx context.Context) error {
		return exportHeartbeat(heartbeatCtx, s.deps, lease)
	}, nil)
	defer func() {
		// A heartbeat error (lease loss included) takes precedence over the
		// error the aborted work returned — completing or failing under a
		// lost lease is never correct.
		if err := stopHeartbeat(); err != nil {
			resultErr = err
		}
		// Losing the lease is a per-job outcome, not a runner error: the
		// heartbeat already stopped and cancelled every in-flight
		// operation, so the stale claim is abandoned without Fail/Complete
		// and the runner continues with the next job.
		if errors.Is(resultErr, postgres.ErrJobLeaseLost) {
			slog.Info("export mất lease — bỏ job đã claim, tiếp tục claim job khác", "module", "audio-export", "job", postgres.UUIDString(job.ID))
			resultErr = nil
		}
	}()

	// A heartbeat that already reported lease loss cancelled this context;
	// never start materialization or ffmpeg for a job we no longer own.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Cache hit: identical inputs were exported before — attach nothing new,
	// report the manifest, ffmpeg never runs.
	if hit, files, err := s.exportCacheHit(ctx, scope, in.CacheKey, exportCacheKind(in.Options)); err == nil && hit {
		result := map[string]any{
			"cacheHit": true, "cacheKey": in.CacheKey, "files": files,
		}
		if err := stopHeartbeat(); err != nil {
			return err
		}
		_, err = s.deps.Jobs.Complete(completionCtx, lease, result, int32(len(files)), nil)
		return pgErr(err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Materialize the locked inputs into a private scratch layout so the
	// file-backend pipeline runs unchanged.
	scratch, err := s.materializeExport(ctx, scope, in)
	if err != nil {
		return err
	}
	defer scratch.cleanup()

	fileSvc, err := NewService(scratch.layout, NullSynth{Reason: "hosted"})
	if err != nil {
		return err
	}
	defer fileSvc.Close()
	fileSvc.SetStoryNames(in.BookTitle, in.ShortTitle)

	result, err := fileSvc.Export(ctx, in.Volume, in.BookTitle, in.Source, in.Options)
	if err != nil {
		return err
	}

	// Upload every produced file as an immutable blob and record the manifest
	// inside the completion transaction. Chapter companions go to the
	// audio/ch namespace (per-chapter playback) and are cached under their own
	// chapter-audio key; everything else is an export artifact.
	type upload struct {
		file    ExportFile
		blob    postgres.Blob
		chapter int // 0 = volume-level artifact
	}
	chapterOf := map[string]int{}
	for _, cf := range result.ChapterFiles {
		chapterOf[cf.Path] = cf.Chapter
	}
	var uploads []pgExportUpload
	exportDir := scratch.layout.ExportsDir
	for _, f := range result.Files {
		u, err := s.uploadExportFile(ctx, scope, job.ID, in.Volume, chapterOf[f.Path], exportDir, f)
		if err != nil {
			return err
		}
		uploads = append(uploads, u)
	}
	if len(uploads) == 0 {
		return errors.New("export produced no files")
	}
	manifest := pgExportManifest{
		ContentKey: in.CacheKey, Volume: in.Volume,
		Files:        make([]ExportFile, 0, len(uploads)),
		ChapterMarks: result.ChapterMarks, DurationSec: result.DurationSec,
		ExportedAt: time.Now(), Split: in.Options.Split,
	}
	var chapterAudio []pgExportUpload
	var primary postgres.Blob
	havePrimary := false
	for _, u := range uploads {
		if u.chapter > 0 {
			chapterAudio = append(chapterAudio, u)
			continue
		}
		if !havePrimary {
			primary, havePrimary = u.blob, true
		}
		f := u.file
		f.URL = s.exportFileURL(u.blob.ObjectKey)
		f.Path = u.blob.ObjectKey
		manifest.Files = append(manifest.Files, f)
	}
	if !havePrimary {
		primary = uploads[0].blob
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	result2 := map[string]any{
		"cacheHit": false, "cacheKey": in.CacheKey,
		"files": manifest.Files, "durationSec": result.DurationSec,
	}
	// Stop and join renewals before fenced completion. A failed heartbeat
	// cannot race successful attachment, and a completed job is not mistaken
	// for a lost lease by an in-flight renewal.
	if err := stopHeartbeat(); err != nil {
		return err
	}
	_, err = s.deps.Jobs.CompleteWith(completionCtx, lease, result2, int32(len(uploads)),
		func(ctx context.Context, tx pgx.Tx, _ postgres.Job) error {
			entry := postgres.ExportCacheEntry{
				CacheKey: in.CacheKey, Kind: exportCacheKind(in.Options),
				BlobID: primary.ID, InputManifest: manifestJSON,
			}
			if err := s.deps.Audio.PutExportCacheTx(ctx, tx, scope, entry, nil); err != nil {
				return err
			}
			// Chapter audio is addressed by (volume, chapter), not by the
			// volume's content key: the play endpoint looks it up directly.
			for _, u := range chapterAudio {
				ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(u.file.Name)), ".")
				if err := s.deps.Audio.PutExportCacheTx(ctx, tx, scope, postgres.ExportCacheEntry{
					CacheKey:      ChapterAudioCacheKey(in.Volume, u.chapter),
					Kind:          ext,
					BlobID:        u.blob.ID,
					InputManifest: exportFileManifestJSON(u, in.Volume),
				}, nil); err != nil {
					return err
				}
			}
			// Every produced file also registers under the logical export
			// path (export-file/<rel>) so the hosted /exports list and
			// /exports/file download resolve them like the file backend's
			// directory walk.
			for _, u := range uploads {
				ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(u.file.Name)), ".")
				rel := strings.TrimPrefix(filepath.ToSlash(u.file.Path), "/")
				if err := s.deps.Audio.PutExportCacheTx(ctx, tx, scope, postgres.ExportCacheEntry{
					CacheKey:      "export-file/" + rel,
					Kind:          ext,
					BlobID:        u.blob.ID,
					InputManifest: exportFileManifestJSON(u, in.Volume),
				}, nil); err != nil {
					return err
				}
			}
			return nil
		})
	if err != nil {
		return pgErr(err)
	}
	s.hub.Publish(NewEvent(EventExport, map[string]any{
		"volume": in.Volume, "state": JobDone, "percent": float64(100),
	}))
	return nil
}

// exportCacheHit reports whether the content key already has a manifest and
// returns its file list (URLs refreshed).
func (s *PGService) exportCacheHit(ctx context.Context, scope postgres.ScopeRef, key, kind string) (bool, []ExportFile, error) {
	entry, err := s.deps.Cache.Get(ctx, scope, key, kind)
	if err != nil {
		if errors.Is(err, postgres.ErrJobNotFound) {
			return false, nil, nil
		}
		return false, nil, pgErr(err)
	}
	var m pgExportManifest
	if err := json.Unmarshal(entry.InputManifest, &m); err != nil {
		return false, nil, fmt.Errorf("export cache manifest hỏng: %w", err)
	}
	return true, m.Files, nil
}

// pgExportScratch is the temporary file layout one export runs in.
type pgExportScratch struct {
	root    string
	layout  Layout
	cleanup func()
}

// materializeExport builds the scratch layout: scripts JSON with selected
// renders filled in, render WAVs at their content-key paths, settings/cast/
// voices at the file backend's locations.
func (s *PGService) materializeExport(ctx context.Context, scope postgres.ScopeRef, in pgExportInput) (*pgExportScratch, error) {
	cache, err := configuredBlobCache()
	if err != nil {
		return nil, err
	}
	root, cleanup, err := cache.workDir()
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*pgExportScratch, error) {
		cleanup()
		return nil, err
	}
	storyDir := filepath.Join(root, "story")
	layout := Layout{
		StoryDir:   storyDir,
		AudioDir:   filepath.Join(storyDir, defaultAudioDir),
		AssetsDir:  filepath.Join(root, "assets"),
		ExportsDir: filepath.Join(storyDir, "xuat-ban"),
	}
	if err := os.MkdirAll(layout.AudioDir, 0o755); err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(layout.ExportsDir, 0o755); err != nil {
		return fail(err)
	}

	chapters := map[int]bool{}
	for _, c := range in.Chapters {
		chapters[c] = true
	}
	var downloads []blobMaterialization
	for _, sc := range in.Source {
		if len(chapters) > 0 && !chapters[sc.Chapter] {
			continue
		}
		ref, err := s.chapterRef(ctx, scope, sc.Chapter)
		if err != nil {
			return fail(err)
		}
		row, err := s.deps.Audio.LatestScript(ctx, scope.ProjectID, ref.ID)
		if err != nil {
			return fail(pgErr(err))
		}
		var script Script
		if err := json.Unmarshal(row.Payload, &script); err != nil {
			return fail(fmt.Errorf("bản đọc chương %d hỏng: %w", sc.Chapter, err))
		}
		renders, err := s.deps.Audio.Renders(ctx, scope.ProjectID, ref.ID, row.RevisionNo)
		if err != nil {
			return fail(pgErr(err))
		}
		selected := map[string]postgres.Render{}
		for _, r := range renders {
			if r.Selected {
				selected[r.SegmentID] = r
			}
		}
		for i := range script.Segments {
			seg := &script.Segments[i]
			r, ok := selected[seg.ID]
			if !ok {
				continue
			}
			blob, err := s.deps.Audio.BlobByID(ctx, scope.ProjectID, r.BlobID)
			if err != nil {
				return fail(pgErr(err))
			}
			if blob.State != "ready" {
				return fail(fmt.Errorf("blob của đoạn %s chưa sẵn sàng", seg.ID))
			}
			dest := layout.CachePath(in.Volume, sc.Chapter, r.SynthKey)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return fail(err)
			}
			downloads = append(downloads, blobMaterialization{blob: blob, dest: dest})
			now := time.Now()
			seg.Rendered = &SegmentAudio{
				Key: r.SynthKey, DurationSec: r.DurationSec, RenderedAt: &now,
				Engine: in.Engine, Model: in.Model, Quality: in.Options.Format,
			}
		}
		script.Status = StatusLocked
		if err := WriteScript(layout, &script); err != nil {
			return fail(err)
		}
	}

	// Settings + cast + voices at the file backend's locations.
	doc, _, err := s.docFor(ctx, scope)
	if err != nil {
		return fail(err)
	}
	if err := WriteSettings(layout, doc.Settings); err != nil {
		return fail(err)
	}
	cast, _, err := s.loadCastFor(ctx, scope)
	if err != nil {
		return fail(err)
	}
	if err := WriteCast(layout, cast); err != nil {
		return fail(err)
	}
	voices, err := s.deps.Audio.ListVoices(ctx, scope.ProjectID)
	if err != nil {
		return fail(pgErr(err))
	}
	list := make([]ProjectVoice, 0, len(voices))
	for _, v := range voices {
		var pv ProjectVoice
		if err := json.Unmarshal(v.Metadata, &pv); err == nil && pv.ID != "" {
			list = append(list, pv)
		}
	}
	if err := WriteProjectVoices(layout, list); err != nil {
		return fail(err)
	}

	// Tư liệu (bìa, nhạc nền): file mode reads them off the assets dir
	// (Layout.CoverPNG for m4b embed, pickMusic for withMusic). Hosted keeps
	// them as project_documents blobs; download into the scratch layout so
	// the untouched engine paths consume them.
	if err := s.materializeBlobs(ctx, downloads); err != nil {
		return fail(err)
	}
	if err := s.materializeAssets(ctx, scope, layout, in.Options); err != nil {
		return fail(err)
	}
	return &pgExportScratch{root: root, layout: layout, cleanup: cleanup}, nil
}

// materializeAssets downloads project asset documents (kind='asset') into
// the scratch layout's assets dir: nhac/<name> → MusicDir, bia/<name> →
// CoverDir. Logical names already carry the file-mode relative path.
func (s *PGService) materializeAssets(ctx context.Context, scope postgres.ScopeRef, layout Layout, opts ExportOptions) error {
	docs, err := s.deps.Audio.ListAssetDocs(ctx, scope.ProjectID)
	if err != nil {
		return pgErr(err)
	}
	var cover *postgres.AssetDoc
	music := map[string]postgres.AssetDoc{}
	for i := range docs {
		d := &docs[i]
		name := filepath.Base(d.LogicalName)
		switch {
		case strings.HasPrefix(d.LogicalName, "nhac/") && opts.Intro && !opts.Split:
			if !isAudioFile(name) || strings.HasPrefix(name, ".") {
				continue
			}
			// Let the file pipeline's existing selector choose intro/outro
			// from names, without downloading unused project music.
			if err := os.MkdirAll(layout.MusicDir(), 0700); err != nil {
				return err
			}
			path := filepath.Join(layout.MusicDir(), name)
			if err := os.WriteFile(path, nil, 0600); err != nil {
				return err
			}
			music[path] = *d
		case strings.HasPrefix(d.LogicalName, "bia/"):
			switch strings.ToLower(filepath.Ext(name)) {
			case ".jpg", ".jpeg", ".png", ".webp":
				if cover == nil || d.UpdatedAt.After(cover.UpdatedAt) {
					cover = d
				}
			}
		}
	}
	intro, outro, err := pickMusic(layout.MusicDir())
	if err != nil {
		return err
	}
	var selected []postgres.AssetDoc
	for path, d := range music {
		if err := os.Remove(path); err != nil {
			return err
		}
		if path == intro || path == outro {
			selected = append(selected, d)
		}
	}
	if cover != nil {
		selected = append(selected, *cover)
	}
	var downloads []blobMaterialization
	for _, d := range selected {
		blob, err := s.deps.Audio.BlobByID(ctx, scope.ProjectID, d.BlobID)
		if err != nil {
			return pgErr(err)
		}
		if blob.State != "ready" {
			return errors.New("project asset is not ready")
		}
		dir := layout.CoverDir()
		if strings.HasPrefix(d.LogicalName, "nhac/") {
			dir = layout.MusicDir()
		}
		downloads = append(downloads, blobMaterialization{blob: blob,
			dest: filepath.Join(dir, filepath.Base(d.LogicalName))})
	}
	return s.materializeBlobs(ctx, downloads)
}

// assetDocsFingerprint hashes the project's asset documents into the export
// content key: a changed cover or music bed must not hit a stale cache row.
func (s *PGService) assetDocsFingerprint(ctx context.Context, projectID pgtype.UUID) []byte {
	docs, err := s.deps.Audio.ListAssetDocs(ctx, projectID)
	if err != nil {
		return nil
	}
	h := sha256.New()
	for _, d := range docs {
		fmt.Fprintf(h, "%s=%s@%d\n", d.LogicalName, postgres.UUIDString(d.BlobID), d.Version)
	}
	return h.Sum(nil)
}

// downloadBlob streams one blob object to a scratch file, verifying the
// recorded hash.
func (s *PGService) downloadBlob(ctx context.Context, blob postgres.Blob, dest string) error {
	cache, err := configuredBlobCache()
	if err != nil {
		return err
	}
	return cache.materialize(ctx, s.deps.Blobs, blob.StorageDriver, blob.ObjectKey, blob.SHA256, blob.SizeBytes, dest)
}

type blobMaterialization struct {
	blob postgres.Blob
	dest string
}

func (s *PGService) materializeBlobs(ctx context.Context, items []blobMaterialization) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	work := make(chan blobMaterialization)
	var workers sync.WaitGroup
	var firstErr error
	var once sync.Once
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range work {
				if err := s.downloadBlob(ctx, item.blob, item.dest); err != nil {
					once.Do(func() { firstErr = err; cancel() })
				}
			}
		}()
	}
send:
	for _, item := range items {
		select {
		case work <- item:
		case <-ctx.Done():
			break send
		}
	}
	close(work)
	workers.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

// ticks is injectable so tests can cross the 120-second lease without sleeping.
// The real ticker runs throughout all I/O and ffmpeg. A failed or stuck renewal
// cancels the context shared by copies, CommandContext, and fenced completion.
func maintainExportLease(parent context.Context, interval time.Duration, heartbeat func(context.Context) error, ticks <-chan time.Time) (context.Context, func() error) {
	ctx, cancel := context.WithCancelCause(parent)
	var ticker *time.Ticker
	if ticks == nil {
		ticker = time.NewTicker(interval)
		ticks = ticker.C
	}
	// stopCh signals the heartbeater to exit instead of cancelling the work
	// context: an in-flight renewal must report the real lease verdict (lease
	// lost vs renewed), not a derived context error from stop itself.
	stopCh := make(chan struct{})
	done := make(chan struct{})
	var hbErr error
	go func() {
		defer close(done)
		if ticker != nil {
			defer ticker.Stop()
		}
		for {
			renewCtx, renewCancel := context.WithTimeout(ctx, interval)
			err := heartbeat(renewCtx)
			renewCancel()
			if err != nil {
				// A failure on an already-cancelled work context is derived
				// (stop or the caller cancelled) — the real failure is the
				// cancellation itself, so don't record a bogus heartbeat loss.
				if ctx.Err() == nil {
					cancel(fmt.Errorf("export heartbeat: %w", err))
					hbErr = err
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticks:
			}
		}
	}()
	var stopOnce sync.Once
	var stopErr error
	// hbErr is written only while the work context is live and read after
	// done closes — the channel close orders them.
	return ctx, func() error {
		stopOnce.Do(func() {
			close(stopCh)
			<-done
			cancel(context.Canceled)
			stopErr = hbErr
		})
		return stopErr
	}
}

// pgExportUpload is one published export artifact.
type pgExportUpload struct {
	file    ExportFile
	blob    postgres.Blob
	chapter int // 0 = volume-level artifact
}

// exportFileManifestJSON builds the per-file registry manifest consumed by
// the web layer's hosted /exports list and /exports/file resolver. The
// shape mirrors the web package's pgExportFileManifest (rel/name/size/
// sha256/key/volume/format/exportedAt) — keep both in sync.
func exportFileManifestJSON(u pgExportUpload, volume int) []byte {
	rel := strings.TrimPrefix(filepath.ToSlash(u.file.Path), "/")
	format := "other"
	switch strings.ToLower(filepath.Ext(u.file.Name)) {
	case ".mp3", ".m4b", ".wav", ".ogg", ".flac", ".zip":
		format = "audio"
	case ".txt":
		format = "txt"
	case ".epub":
		format = "epub"
	case ".srt":
		format = "other"
	}
	vol := volume
	body, _ := json.Marshal(map[string]any{
		"rel": rel, "name": u.file.Name, "size": u.blob.SizeBytes,
		"sha256": u.blob.SHA256, "key": u.blob.ObjectKey,
		"volume": vol, "format": format,
		"exportedAt": time.Now().UTC(),
	})
	return body
}

// uploadExportFile publishes one produced file: blob object first (key from
// internal/blobkey — story-scoped UUID paths, chapter audio under audio/ch),
// then the registry row. The registry records the driver the bytes actually
// live on (blobstore.DriverName), never a hardcoded label.
func (s *PGService) uploadExportFile(ctx context.Context, scope postgres.ScopeRef, jobID pgtype.UUID, volume, chapter int, exportDir string, f ExportFile) (pgExportUpload, error) {
	var zero pgExportUpload
	src := filepath.Join(exportDir, filepath.FromSlash(f.Path))
	data, err := os.Open(src)
	if err != nil {
		return zero, err
	}
	defer data.Close()
	h := sha256.New()
	size, err := io.Copy(h, data)
	if err != nil {
		return zero, err
	}
	sha := hex.EncodeToString(h.Sum(nil))
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(f.Name)), ".")
	var key string
	if chapter > 0 {
		key, err = s.deps.Keys.AudioChapter(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), volume, chapter, sha, ext)
	} else {
		key, err = s.deps.Keys.Export(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), sha, f.Name)
	}
	if err != nil {
		return zero, fmt.Errorf("khoá xuất %s: %w", f.Name, err)
	}
	driver := blobstore.DriverName(s.deps.Blobs)
	if driver == "" {
		return zero, errors.New("BlobStore không khai báo driver — không ghi được blobs.storage_driver")
	}
	if _, err := data.Seek(0, io.SeekStart); err != nil {
		return zero, err
	}
	blob, err := postgres.UploadAndPublishBlob(ctx, s.deps.Audio, s.deps.Blobs, postgres.PublishInput{
		ProjectID: scope.ProjectID, Key: key, Driver: driver,
		ContentType: exportContentType(f.Name), SHA256: sha, Size: size,
		JobID: jobID,
	}, data)
	if err != nil {
		return zero, pgErr(err)
	}
	return pgExportUpload{file: f, blob: blob, chapter: chapter}, nil
}

// exportContentType maps a produced file name to a content type.
func exportContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp3":
		return "audio/mpeg"
	case ".m4b":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".srt":
		return "text/plain"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}
