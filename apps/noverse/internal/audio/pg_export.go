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
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	if err := svc.processExportJob(ctx, job); err != nil {
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
			return err
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

// processExportJob executes one claimed audio_export job.
func (s *PGService) processExportJob(ctx context.Context, job postgres.Job) error {
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

	// Cache hit: identical inputs were exported before — attach nothing new,
	// report the manifest, ffmpeg never runs.
	if hit, files, err := s.exportCacheHit(ctx, scope, in.CacheKey, exportCacheKind(in.Options)); err == nil && hit {
		result := map[string]any{
			"cacheHit": true, "cacheKey": in.CacheKey, "files": files,
		}
		_, err = s.deps.Jobs.Complete(ctx, lease, result, int32(len(files)), nil)
		return pgErr(err)
	}

	// Materialize the locked inputs into a private scratch layout so the
	// file-backend pipeline runs unchanged.
	scratch, err := s.materializeExport(ctx, scope, in)
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch.root)

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
	_, err = s.deps.Jobs.CompleteWith(ctx, lease, result2, int32(len(uploads)),
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
					CacheKey: ChapterAudioCacheKey(in.Volume, u.chapter),
					Kind:     ext,
					BlobID:   u.blob.ID,
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
	root   string
	layout Layout
}

// materializeExport builds the scratch layout: scripts JSON with selected
// renders filled in, render WAVs at their content-key paths, settings/cast/
// voices at the file backend's locations.
func (s *PGService) materializeExport(ctx context.Context, scope postgres.ScopeRef, in pgExportInput) (*pgExportScratch, error) {
	root, err := os.MkdirTemp("", "ainovel-pg-export-*")
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*pgExportScratch, error) {
		os.RemoveAll(root)
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
			if err := s.downloadBlob(ctx, blob, dest); err != nil {
				return fail(err)
			}
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
	return &pgExportScratch{root: root, layout: layout}, nil
}

// downloadBlob streams one blob object to a scratch file, verifying the
// recorded hash.
func (s *PGService) downloadBlob(ctx context.Context, blob postgres.Blob, dest string) error {
	rc, info, err := s.deps.Blobs.Open(ctx, blob.ObjectKey, blobstore.FullRange)
	if err != nil {
		return err
	}
	defer rc.Close()
	if info.SHA256 != "" && !strings.EqualFold(info.SHA256, blob.SHA256) {
		return fmt.Errorf("blob %s hash không khớp đăng ký", blob.ObjectKey)
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(f, io.TeeReader(rc, h)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, blob.SHA256) {
		return fmt.Errorf("blob %s sai nội dung", blob.ObjectKey)
	}
	return nil
}

// pgExportUpload is one published export artifact.
type pgExportUpload struct {
	file    ExportFile
	blob    postgres.Blob
	chapter int // 0 = volume-level artifact
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
	// Content-addressed: an identical re-export (cache expired/pruned) finds
	// the object already registered — reuse it instead of inserting a second
	// registry row (object_key is unique per project) or rewriting the bytes.
	if existing, err := s.deps.Audio.BlobByKey(ctx, scope.ProjectID, key); err == nil &&
		existing.State == "ready" && strings.EqualFold(existing.SHA256, sha) {
		return pgExportUpload{file: f, blob: existing, chapter: chapter}, nil
	}
	if _, err := data.Seek(0, io.SeekStart); err != nil {
		return zero, err
	}
	if _, err := s.deps.Blobs.Put(ctx, key, data, blobstore.PutOptions{
		ContentType:    exportContentType(f.Name),
		ExpectedSHA256: sha,
	}); err != nil {
		return zero, err
	}
	blob, err := s.deps.Audio.StageBlob(ctx, scope.ProjectID, key, driver, exportContentType(f.Name), pgtype.UUID{}, jobID)
	if err != nil {
		return zero, pgErr(err)
	}
	blob, err = s.deps.Audio.MarkBlobReady(ctx, scope.ProjectID, blob.ID, sha, size)
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
