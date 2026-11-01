package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// Audio metadata on Postgres (phase 07). Binaries never live here: samples,
// segment renders and export outputs are BlobStore objects referenced by
// blob_id, and every cache/selection key is content-addressed — never a path
// or an mtime.
//
// Scope rules from the schema report:
//   * settings and cast are per (project, story);
//   * voices are per project;
//   * scripts/segments/renders hang off a chapter and a script revision;
//   * exactly one render per segment is selected.

var (
	// ErrAudioSettingsMissing means the story has no saved audio settings yet;
	// callers fall back to defaults (the file backend does the same).
	ErrAudioSettingsMissing = errors.New("truyện chưa có cấu hình âm thanh")
	synthKeyRe              = regexp.MustCompile(`^[0-9a-f]{64}$`)
	sha256Re                = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Audio is the audio-metadata repository.
type Audio struct {
	pool *pgxpool.Pool
}

// NewAudio builds the audio repository on the shared pool.
func NewAudio(p *Pool) *Audio { return &Audio{pool: p.Pool()} }

func (a *Audio) tx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return RunInTx(ctx, a.pool, fn)
}

func marshalPayload(v any) ([]byte, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal audio payload: %w", errInvalidJobInput)
	}
	return body, nil
}

// ── settings ─────────────────────────────────────────────────────────────────

// Settings returns the stored settings JSON and the row's optimistic version.
// ErrAudioSettingsMissing when the story has never saved any.
func (a *Audio) Settings(ctx context.Context, scope ScopeRef) ([]byte, int64, error) {
	row, err := gen.New(a.pool).GetAudioSettings(ctx, gen.GetAudioSettingsParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
	})
	if err != nil {
		if noRows(err) {
			return nil, 0, ErrAudioSettingsMissing
		}
		return nil, 0, sanitizeErr(err)
	}
	return row.Payload, row.Version, nil
}

// SaveSettings stores the settings snapshot, returning the new row version.
func (a *Audio) SaveSettings(ctx context.Context, scope ScopeRef, settings any) (int64, error) {
	body, err := marshalPayload(settings)
	if err != nil {
		return 0, err
	}
	row, err := gen.New(a.pool).UpsertAudioSettings(ctx, gen.UpsertAudioSettingsParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
		Payload:   body,
	})
	if err != nil {
		return 0, sanitizeErr(err)
	}
	return row.Version, nil
}

// ── voices ───────────────────────────────────────────────────────────────────

// Voice is a project-scoped voice entry; LegacyID is the file-mode voice name
// (e.g. "vieneu-nu-bac") so imported and new data share one identity.
type Voice struct {
	ID           pgtype.UUID
	LegacyID     string
	Metadata     []byte
	SampleBlobID pgtype.UUID
	Version      int64
}

func voiceFromRow(r gen.AudioVoice) Voice {
	return Voice{ID: r.ID, LegacyID: r.LegacyID, Metadata: r.Metadata, SampleBlobID: r.SampleBlobID, Version: r.Version}
}

// UpsertVoice creates or updates a voice by its legacy id. The row id is
// stable across updates so cast references stay valid.
func (a *Audio) UpsertVoice(ctx context.Context, projectID pgtype.UUID, legacyID string, metadata any, sampleBlobID pgtype.UUID) (Voice, error) {
	body, err := marshalPayload(metadata)
	if err != nil {
		return Voice{}, err
	}
	id, err := randomUUID()
	if err != nil {
		return Voice{}, fmt.Errorf("voice id: %w", errInvalidJobInput)
	}
	row, err := gen.New(a.pool).UpsertAudioVoice(ctx, gen.UpsertAudioVoiceParams{
		ProjectID:    projectID,
		ID:           id,
		LegacyID:     legacyID,
		Metadata:     body,
		SampleBlobID: sampleBlobID,
	})
	if err != nil {
		return Voice{}, sanitizeErr(err)
	}
	return voiceFromRow(row), nil
}

// ListVoices returns the project's voices ordered by legacy id.
func (a *Audio) ListVoices(ctx context.Context, projectID pgtype.UUID) ([]Voice, error) {
	rows, err := gen.New(a.pool).ListAudioVoices(ctx, projectID)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]Voice, 0, len(rows))
	for _, r := range rows {
		out = append(out, voiceFromRow(r))
	}
	return out, nil
}

// VoiceByLegacy returns the project voice for a file-mode voice id
// (legacy_id); ErrJobNotFound when it was never imported or created.
func (a *Audio) VoiceByLegacy(ctx context.Context, projectID pgtype.UUID, legacyID string) (Voice, error) {
	row, err := gen.New(a.pool).GetAudioVoiceByLegacy(ctx, gen.GetAudioVoiceByLegacyParams{
		ProjectID: projectID,
		LegacyID:  legacyID,
	})
	if err != nil {
		if noRows(err) {
			return Voice{}, ErrJobNotFound
		}
		return Voice{}, sanitizeErr(err)
	}
	return voiceFromRow(row), nil
}

// DeleteVoice removes a voice; cast rows referencing it keep the delete from
// succeeding (FK RESTRICT) instead of silently dropping an assignment.
func (a *Audio) DeleteVoice(ctx context.Context, projectID pgtype.UUID, legacyID string) error {
	n, err := gen.New(a.pool).DeleteAudioVoice(ctx, gen.DeleteAudioVoiceParams{ProjectID: projectID, LegacyID: legacyID})
	if err != nil {
		return sanitizeErr(err)
	}
	if n == 0 {
		return ErrJobNotFound
	}
	return nil
}

// ── cast ─────────────────────────────────────────────────────────────────────

// CastRole assigns a voice to a story role (narration, character name, ...).
type CastRole struct {
	RoleKey string
	VoiceID pgtype.UUID
	Payload []byte
	Version int64
}

// ListCast returns the story's cast ordered by role key.
func (a *Audio) ListCast(ctx context.Context, scope ScopeRef) ([]CastRole, error) {
	rows, err := gen.New(a.pool).ListAudioCast(ctx, gen.ListAudioCastParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]CastRole, 0, len(rows))
	for _, r := range rows {
		out = append(out, CastRole{RoleKey: r.RoleKey, VoiceID: r.VoiceID, Payload: r.Payload, Version: r.Version})
	}
	return out, nil
}

// SaveCast upserts one role assignment.
func (a *Audio) SaveCast(ctx context.Context, scope ScopeRef, roleKey string, voiceID pgtype.UUID, payload any) (CastRole, error) {
	body, err := marshalPayload(payload)
	if err != nil {
		return CastRole{}, err
	}
	row, err := gen.New(a.pool).UpsertAudioCast(ctx, gen.UpsertAudioCastParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
		RoleKey:   roleKey,
		VoiceID:   voiceID,
		Payload:   body,
	})
	if err != nil {
		return CastRole{}, sanitizeErr(err)
	}
	return CastRole{RoleKey: row.RoleKey, VoiceID: row.VoiceID, Payload: row.Payload, Version: row.Version}, nil
}

// DeleteCast removes one role assignment.
func (a *Audio) DeleteCast(ctx context.Context, scope ScopeRef, roleKey string) error {
	n, err := gen.New(a.pool).DeleteAudioCastRole(ctx, gen.DeleteAudioCastRoleParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
		RoleKey:   roleKey,
	})
	if err != nil {
		return sanitizeErr(err)
	}
	if n == 0 {
		return ErrJobNotFound
	}
	return nil
}

// ── scripts ──────────────────────────────────────────────────────────────────

// Script is a TTS script bound to one accepted chapter revision. SourceSHA256
// is the chapter text hash at the time the script was generated, which is what
// staleness checks compare against the chapter's latest accepted revision.
type Script struct {
	ChapterID      pgtype.UUID
	RevisionNo     int32
	SourceRevision int32
	SourceSHA256   string
	Status         string
	Payload        []byte
	Version        int64
}

func scriptFromRow(r gen.AudioScript) Script {
	return Script{
		ChapterID:      r.ChapterID,
		RevisionNo:     r.RevisionNo,
		SourceRevision: r.SourceRevision,
		SourceSHA256:   r.SourceSha256,
		Status:         r.Status,
		Payload:        r.Payload,
		Version:        r.Version,
	}
}

// audioScriptStatuses mirrors the domain's finite script states
// (internal/audio/script.go). The DB CHECK added in migration 00009 is the
// integrity backstop; this guard returns a useful domain error first.
var audioScriptStatuses = map[string]bool{
	"todo": true, "draft": true, "review": true, "locked": true, "stale": true,
}

func validateScriptMeta(script Script) error {
	if !script.ChapterID.Valid || script.RevisionNo <= 0 || script.SourceRevision <= 0 {
		return fmt.Errorf("script thiếu chapter/revision: %w", errInvalidJobInput)
	}
	if !audioScriptStatuses[script.Status] {
		return fmt.Errorf("trạng thái bản đọc %q không hợp lệ: %w", script.Status, errInvalidJobInput)
	}
	if !sha256Re.MatchString(script.SourceSHA256) {
		return fmt.Errorf("source_sha256 %q không hợp lệ: %w", script.SourceSHA256, errInvalidJobInput)
	}
	return nil
}

// SaveScript upserts a script revision. The (project, chapter, revision) key
// makes a concurrent regeneration idempotent rather than duplicating rows.
func (a *Audio) SaveScript(ctx context.Context, scope ScopeRef, script Script) (Script, error) {
	if err := validateScriptMeta(script); err != nil {
		return Script{}, err
	}
	row, err := gen.New(a.pool).UpsertAudioScript(ctx, gen.UpsertAudioScriptParams{
		ProjectID:      scope.ProjectID,
		StoryID:        scope.StoryID,
		ChapterID:      script.ChapterID,
		RevisionNo:     script.RevisionNo,
		SourceRevision: script.SourceRevision,
		SourceSha256:   script.SourceSHA256,
		Status:         script.Status,
		Payload:        script.Payload,
	})
	if err != nil {
		return Script{}, sanitizeErr(err)
	}
	return scriptFromRow(row), nil
}

// Script returns one revision; ErrJobNotFound when it was never generated.
func (a *Audio) Script(ctx context.Context, projectID, chapterID pgtype.UUID, revision int32) (Script, error) {
	row, err := gen.New(a.pool).GetAudioScript(ctx, gen.GetAudioScriptParams{
		ProjectID:  projectID,
		ChapterID:  chapterID,
		RevisionNo: revision,
	})
	if err != nil {
		if noRows(err) {
			return Script{}, ErrJobNotFound
		}
		return Script{}, sanitizeErr(err)
	}
	return scriptFromRow(row), nil
}

// LatestScript returns the newest script revision for a chapter.
func (a *Audio) LatestScript(ctx context.Context, projectID, chapterID pgtype.UUID) (Script, error) {
	row, err := gen.New(a.pool).LatestAudioScript(ctx, gen.LatestAudioScriptParams{
		ProjectID: projectID,
		ChapterID: chapterID,
	})
	if err != nil {
		if noRows(err) {
			return Script{}, ErrJobNotFound
		}
		return Script{}, sanitizeErr(err)
	}
	return scriptFromRow(row), nil
}

// ── segments ─────────────────────────────────────────────────────────────────

// Segment is one speakable unit of a script revision. SegmentID is stable for
// the same text position so re-running the segmenter does not orphan renders.
type Segment struct {
	SegmentID string
	Ordinal   int32
	Speaker   string
	Text      string
	Payload   []byte
	Version   int64
}

// SaveSegment upserts a segment of a script revision.
func (a *Audio) SaveSegment(ctx context.Context, projectID, chapterID pgtype.UUID, scriptRevision int32, seg Segment) (Segment, error) {
	row, err := gen.New(a.pool).UpsertAudioSegment(ctx, gen.UpsertAudioSegmentParams{
		ProjectID:      projectID,
		ChapterID:      chapterID,
		ScriptRevision: scriptRevision,
		SegmentID:      seg.SegmentID,
		Ordinal:        seg.Ordinal,
		Speaker:        seg.Speaker,
		TextContent:    seg.Text,
		Payload:        seg.Payload,
	})
	if err != nil {
		return Segment{}, sanitizeErr(err)
	}
	return Segment{SegmentID: row.SegmentID, Ordinal: row.Ordinal, Speaker: row.Speaker, Text: row.TextContent, Payload: row.Payload, Version: row.Version}, nil
}

// Segments lists a script revision's segments in reading order.
func (a *Audio) Segments(ctx context.Context, projectID, chapterID pgtype.UUID, scriptRevision int32) ([]Segment, error) {
	rows, err := gen.New(a.pool).ListAudioSegments(ctx, gen.ListAudioSegmentsParams{
		ProjectID:      projectID,
		ChapterID:      chapterID,
		ScriptRevision: scriptRevision,
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]Segment, 0, len(rows))
	for _, r := range rows {
		out = append(out, Segment{SegmentID: r.SegmentID, Ordinal: r.Ordinal, Speaker: r.Speaker, Text: r.TextContent, Payload: r.Payload, Version: r.Version})
	}
	return out, nil
}

// ── renders ──────────────────────────────────────────────────────────────────

// Render is one synthesized candidate for a segment. The synth key is the
// parameter hash, so two settings can coexist as candidates; Selected marks
// the one the current script uses.
type Render struct {
	SegmentID   string
	SynthKey    string
	BlobID      pgtype.UUID
	DurationSec float64
	SampleRate  int32
	Selected    bool
	Parameters  []byte
	CreatedAt   time.Time
	Version     int64
}

func renderFromRow(r gen.AudioRender) Render {
	return Render{
		SegmentID:   r.SegmentID,
		SynthKey:    r.SynthKey,
		BlobID:      r.BlobID,
		DurationSec: r.DurationSec,
		SampleRate:  r.SampleRate,
		Selected:    r.Selected,
		Parameters:  r.Parameters,
		CreatedAt:   r.CreatedAt.Time.UTC(),
		Version:     r.Version,
	}
}

// AddRender records a synthesized candidate. Re-publishing the same
// (segment, synth key) is an upsert: the blob reference and parameters update,
// the row and its selection flag survive.
func (a *Audio) AddRender(ctx context.Context, projectID, chapterID pgtype.UUID, scriptRevision int32, r Render) (Render, error) {
	return addRender(ctx, gen.New(a.pool), projectID, chapterID, scriptRevision, r)
}

// addRender validates and upserts one candidate through whatever query source
// the caller holds (pool or transaction), so the pool and ACK paths cannot
// drift apart.
func addRender(ctx context.Context, q *gen.Queries, projectID, chapterID pgtype.UUID, scriptRevision int32, r Render) (Render, error) {
	if err := validateRender(r); err != nil {
		return Render{}, err
	}
	params := []byte(r.Parameters)
	if len(params) == 0 {
		params = []byte("{}")
	}
	row, err := q.InsertAudioRender(ctx, gen.InsertAudioRenderParams{
		ProjectID:      projectID,
		ChapterID:      chapterID,
		ScriptRevision: scriptRevision,
		SegmentID:      r.SegmentID,
		SynthKey:       r.SynthKey,
		BlobID:         r.BlobID,
		DurationSec:    r.DurationSec,
		SampleRate:     r.SampleRate,
		Parameters:     params,
	})
	if err != nil {
		return Render{}, sanitizeErr(err)
	}
	return renderFromRow(row), nil
}

func validateRender(r Render) error {
	if !synthKeyRe.MatchString(r.SynthKey) {
		return fmt.Errorf("synth key %q không hợp lệ: %w", r.SynthKey, errInvalidJobInput)
	}
	if !r.BlobID.Valid || r.SampleRate <= 0 || r.DurationSec < 0 {
		return fmt.Errorf("render thiếu blob/sample rate: %w", errInvalidJobInput)
	}
	return nil
}

// SelectRender makes one candidate current for a segment. Clearing and setting
// happen in one transaction so the partial unique index never sees two
// selected rows, and a stale worker's candidate stays historical.
func (a *Audio) SelectRender(ctx context.Context, projectID, chapterID pgtype.UUID, scriptRevision int32, segmentID, synthKey string) (Render, error) {
	var row gen.AudioRender
	err := a.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		if _, err := q.ClearSelectedAudioRenders(ctx, gen.ClearSelectedAudioRendersParams{
			ProjectID:      projectID,
			ChapterID:      chapterID,
			ScriptRevision: scriptRevision,
			SegmentID:      segmentID,
		}); err != nil {
			return sanitizeErr(err)
		}
		var err error
		row, err = q.SelectAudioRender(ctx, gen.SelectAudioRenderParams{
			ProjectID:      projectID,
			ChapterID:      chapterID,
			ScriptRevision: scriptRevision,
			SegmentID:      segmentID,
			SynthKey:       synthKey,
		})
		if noRows(err) {
			return ErrJobNotFound
		}
		return sanitizeErr(err)
	})
	if err != nil {
		return Render{}, err
	}
	return renderFromRow(row), nil
}

// Renders lists every candidate of a script revision.
func (a *Audio) Renders(ctx context.Context, projectID, chapterID pgtype.UUID, scriptRevision int32) ([]Render, error) {
	rows, err := gen.New(a.pool).ListAudioRenders(ctx, gen.ListAudioRendersParams{
		ProjectID:      projectID,
		ChapterID:      chapterID,
		ScriptRevision: scriptRevision,
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]Render, 0, len(rows))
	for _, r := range rows {
		out = append(out, renderFromRow(r))
	}
	return out, nil
}

// SelectedRender returns the current render of one segment.
func (a *Audio) SelectedRender(ctx context.Context, projectID, chapterID pgtype.UUID, scriptRevision int32, segmentID string) (Render, error) {
	row, err := gen.New(a.pool).SelectedAudioRender(ctx, gen.SelectedAudioRenderParams{
		ProjectID:      projectID,
		ChapterID:      chapterID,
		ScriptRevision: scriptRevision,
		SegmentID:      segmentID,
	})
	if err != nil {
		if noRows(err) {
			return Render{}, ErrJobNotFound
		}
		return Render{}, sanitizeErr(err)
	}
	return renderFromRow(row), nil
}

// DeleteRender drops one candidate. Deleting the selected row is allowed:
// the segment then reads as "no audio" until another render is selected.
func (a *Audio) DeleteRender(ctx context.Context, projectID, chapterID pgtype.UUID, scriptRevision int32, segmentID, synthKey string) error {
	n, err := gen.New(a.pool).DeleteAudioRender(ctx, gen.DeleteAudioRenderParams{
		ProjectID:      projectID,
		ChapterID:      chapterID,
		ScriptRevision: scriptRevision,
		SegmentID:      segmentID,
		SynthKey:       synthKey,
	})
	if err != nil {
		return sanitizeErr(err)
	}
	if n == 0 {
		return ErrJobNotFound
	}
	return nil
}

// CarryRenders copies a previous script revision's renders onto a new one for
// segments whose audible text is unchanged. Edited segments get no row, so
// they read as "no audio" until re-synthesized — the same rule the file
// backend applies when seg.Rendered is cleared.
func (a *Audio) CarryRenders(ctx context.Context, projectID, chapterID pgtype.UUID, newRevision, oldRevision int32) (int64, error) {
	n, err := gen.New(a.pool).CarryAudioRenders(ctx, gen.CarryAudioRendersParams{
		ProjectID:        projectID,
		ChapterID:        chapterID,
		ScriptRevision:   newRevision,
		ScriptRevision_2: oldRevision,
	})
	if err != nil {
		return 0, sanitizeErr(err)
	}
	return n, nil
}

// ── blob registry ────────────────────────────────────────────────────────────

// Blob is the database registry row of an immutable object in BlobStore.
type Blob struct {
	ID            pgtype.UUID
	ProjectID     pgtype.UUID
	ObjectKey     string
	StorageDriver string
	SHA256        string
	SizeBytes     int64
	ContentType   string
	State         string
	Version       int64
}

func blobFromRow(r gen.Blob) Blob {
	return Blob{
		ID:            r.ID,
		ProjectID:     r.ProjectID,
		ObjectKey:     r.ObjectKey,
		StorageDriver: r.StorageDriver,
		SHA256:        r.Sha256,
		SizeBytes:     r.SizeBytes,
		ContentType:   r.ContentType,
		State:         r.State,
		Version:       r.Version,
	}
}

// BlobByKey returns the registry row by tenant-scoped object key.
func (a *Audio) BlobByKey(ctx context.Context, projectID pgtype.UUID, objectKey string) (Blob, error) {
	row, err := gen.New(a.pool).GetBlobByKey(ctx, gen.GetBlobByKeyParams{ProjectID: projectID, ObjectKey: objectKey})
	if err != nil {
		if noRows(err) {
			return Blob{}, ErrJobNotFound
		}
		return Blob{}, sanitizeErr(err)
	}
	return blobFromRow(row), nil
}

// BlobByID returns a blob registry row by id (renders point at it).
func (a *Audio) BlobByID(ctx context.Context, projectID, blobID pgtype.UUID) (Blob, error) {
	row, err := gen.New(a.pool).GetBlob(ctx, gen.GetBlobParams{ProjectID: projectID, ID: blobID})
	if err != nil {
		if noRows(err) {
			return Blob{}, ErrJobNotFound
		}
		return Blob{}, sanitizeErr(err)
	}
	return blobFromRow(row), nil
}

// ── export cache ─────────────────────────────────────────────────────────────

// ExportCacheEntry is one exported artifact manifest. CacheKey hashes the
// exact source/script/cast/settings/options/voice/cover inputs; tag-only
// metadata changes produce a new manifest/hash rather than reusing by mtime.
type ExportCacheEntry struct {
	CacheKey      string
	Kind          string
	BlobID        pgtype.UUID
	InputManifest []byte
	ExpiresAt     *time.Time
}

// ExportCache looks up and stores export artifacts.
type ExportCache struct {
	pool *pgxpool.Pool
}

// NewExportCache builds the export-cache repository on the shared pool.
func NewExportCache(p *Pool) *ExportCache { return &ExportCache{pool: p.Pool()} }

// Get returns a live cache entry; ErrJobNotFound when absent or expired.
func (c *ExportCache) Get(ctx context.Context, scope ScopeRef, cacheKey, kind string) (ExportCacheEntry, error) {
	row, err := gen.New(c.pool).GetExportCache(ctx, gen.GetExportCacheParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
		CacheKey:  cacheKey,
		Kind:      kind,
	})
	if err != nil {
		if noRows(err) {
			return ExportCacheEntry{}, ErrJobNotFound
		}
		return ExportCacheEntry{}, sanitizeErr(err)
	}
	return exportCacheFromRow(row), nil
}

func exportCacheFromRow(r gen.ExportCache) ExportCacheEntry {
	return ExportCacheEntry{
		CacheKey:      r.CacheKey,
		Kind:          r.Kind,
		BlobID:        r.BlobID,
		InputManifest: r.InputManifest,
		ExpiresAt:     timePtr(r.ExpiresAt),
	}
}

// Put records the output blob and its input manifest. Re-exporting identical
// inputs overwrites the pointer and manifest, never the object itself.
func (c *ExportCache) Put(ctx context.Context, scope ScopeRef, entry ExportCacheEntry, expiresAt *time.Time) (ExportCacheEntry, error) {
	var exp pgtype.Timestamptz
	if expiresAt != nil {
		exp = pgtype.Timestamptz{Time: *expiresAt, Valid: true}
	}
	manifest := entry.InputManifest
	if len(manifest) == 0 {
		manifest = []byte("{}")
	}
	row, err := gen.New(c.pool).UpsertExportCache(ctx, gen.UpsertExportCacheParams{
		ProjectID:     scope.ProjectID,
		StoryID:       scope.StoryID,
		CacheKey:      entry.CacheKey,
		BlobID:        entry.BlobID,
		Kind:          entry.Kind,
		InputManifest: manifest,
		ExpiresAt:     exp,
	})
	if err != nil {
		return ExportCacheEntry{}, sanitizeErr(err)
	}
	return exportCacheFromRow(row), nil
}

// ErrExportCacheExists means the logical export name was claimed between the
// caller's existence check and its create-only insert.
var ErrExportCacheExists = errors.New("tên tệp xuất vừa được sử dụng")

// Create insert-claims a fresh export logical name. Callers that must not
// overwrite (onExists fail/keep) use this instead of Put; the unique row is
// the arbiter so two concurrent writers can never replace one another.
func (c *ExportCache) Create(ctx context.Context, scope ScopeRef, entry ExportCacheEntry) (ExportCacheEntry, error) {
	manifest := entry.InputManifest
	if len(manifest) == 0 {
		manifest = []byte("{}")
	}
	n, err := gen.New(c.pool).InsertExportCacheIfAbsent(ctx, gen.InsertExportCacheIfAbsentParams{
		ProjectID:     scope.ProjectID,
		StoryID:       scope.StoryID,
		CacheKey:      entry.CacheKey,
		BlobID:        entry.BlobID,
		Kind:          entry.Kind,
		InputManifest: manifest,
	})
	if err != nil {
		return ExportCacheEntry{}, sanitizeErr(err)
	}
	if n == 0 {
		return ExportCacheEntry{}, ErrExportCacheExists
	}
	return c.Get(ctx, scope, entry.CacheKey, entry.Kind)
}

// Expire invalidates live manifests without deleting rows or bytes. Only the
// maintenance GC removes expired entries and their exclusively owned blobs.
func (c *ExportCache) Expire(ctx context.Context, scope ScopeRef) (int64, error) {
	n, err := gen.New(c.pool).ExpireExportCache(ctx, gen.ExpireExportCacheParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
	})
	if err != nil {
		return 0, sanitizeErr(err)
	}
	return n, nil
}

// List returns every live cache row for the story (kinds included — the
// caller filters).
func (c *ExportCache) List(ctx context.Context, scope ScopeRef) ([]ExportCacheEntry, error) {
	rows, err := gen.New(c.pool).ListExportCache(ctx, gen.ListExportCacheParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]ExportCacheEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, exportCacheFromRow(r))
	}
	return out, nil
}

// ── transactional attach (worker ACK) ────────────────────────────────────────
//
// The remote worker's ACK must land the render rows and the job result in one
// transaction; these helpers take the caller's tx (opened by
// Jobs.CompleteWith) instead of the pool.

// PutExportCacheTx records an export artifact inside tx (worker ACK path).
func (a *Audio) PutExportCacheTx(ctx context.Context, tx pgx.Tx, scope ScopeRef, entry ExportCacheEntry, expiresAt *time.Time) error {
	var exp pgtype.Timestamptz
	if expiresAt != nil {
		exp = pgtype.Timestamptz{Time: *expiresAt, Valid: true}
	}
	manifest := entry.InputManifest
	if len(manifest) == 0 {
		manifest = []byte("{}")
	}
	_, err := gen.New(tx).UpsertExportCache(ctx, gen.UpsertExportCacheParams{
		ProjectID:     scope.ProjectID,
		StoryID:       scope.StoryID,
		CacheKey:      entry.CacheKey,
		BlobID:        entry.BlobID,
		Kind:          entry.Kind,
		InputManifest: manifest,
		ExpiresAt:     exp,
	})
	return sanitizeErr(err)
}

// BlobByKeyTx resolves a ready blob by object key inside tx.
func (a *Audio) BlobByKeyTx(ctx context.Context, tx pgx.Tx, projectID pgtype.UUID, objectKey string) (Blob, error) {
	row, err := gen.New(tx).GetBlobByKey(ctx, gen.GetBlobByKeyParams{ProjectID: projectID, ObjectKey: objectKey})
	if err != nil {
		if noRows(err) {
			return Blob{}, ErrJobNotFound
		}
		return Blob{}, sanitizeErr(err)
	}
	return blobFromRow(row), nil
}

// ── project asset documents (Tư liệu: bìa / nhạc nền) ────────────────────────

// AssetDoc is one project_documents row of kind 'asset' — the hosted twin of
// a file under <project>/<dirs.assets>. LogicalName keeps the file-mode
// relative path ("bia/<name>", "nhac/<name>") so every consumer stays
// prefix-compatible; BlobID points at the content-addressed object.
type AssetDoc struct {
	ID          pgtype.UUID
	LogicalName string
	Payload     []byte
	BlobID      pgtype.UUID
	UpdatedAt   time.Time
	Version     int64
}

func assetDocFromRow(r gen.ProjectDocument) AssetDoc {
	return AssetDoc{
		ID: r.ID, LogicalName: r.LogicalName, Payload: r.Payload,
		BlobID: r.BlobID, UpdatedAt: r.UpdatedAt.Time, Version: r.Version,
	}
}

// ListAssetDocs returns the project's asset documents ordered by logical name.
func (a *Audio) ListAssetDocs(ctx context.Context, projectID pgtype.UUID) ([]AssetDoc, error) {
	rows, err := gen.New(a.pool).ListProjectDocumentsByKind(ctx, gen.ListProjectDocumentsByKindParams{
		ProjectID: projectID,
		Kind:      "asset",
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]AssetDoc, 0, len(rows))
	for _, r := range rows {
		out = append(out, assetDocFromRow(r))
	}
	return out, nil
}

// ErrAssetDocNotFound is a missing project asset document inside the
// caller's scope — distinct from job errors so callers don't conflate
// registry misses with queue state.
var ErrAssetDocNotFound = errors.New("không tìm thấy tư liệu này")

// AssetDocByName resolves one asset document by logical name.
func (a *Audio) AssetDocByName(ctx context.Context, projectID pgtype.UUID, name string) (AssetDoc, error) {
	row, err := gen.New(a.pool).GetProjectDocumentByName(ctx, gen.GetProjectDocumentByNameParams{
		ProjectID:   projectID,
		Kind:        "asset",
		LogicalName: name,
	})
	if err != nil {
		if noRows(err) {
			return AssetDoc{}, ErrAssetDocNotFound
		}
		return AssetDoc{}, sanitizeErr(err)
	}
	return assetDocFromRow(row), nil
}

// PutAssetDoc upserts one asset document and returns the stored row.
func (a *Audio) PutAssetDoc(ctx context.Context, projectID pgtype.UUID, name string, payload any, blobID pgtype.UUID) (AssetDoc, error) {
	docID, err := randomUUID()
	if err != nil {
		return AssetDoc{}, err
	}
	body, err := marshalPayload(payload)
	if err != nil {
		return AssetDoc{}, err
	}
	row, err := gen.New(a.pool).UpsertProjectBlobDocument(ctx, gen.UpsertProjectBlobDocumentParams{
		ProjectID:   projectID,
		ID:          docID,
		Kind:        "asset",
		LogicalName: name,
		Payload:     body,
		BlobID:      blobID,
	})
	if err != nil {
		return AssetDoc{}, sanitizeErr(err)
	}
	return assetDocFromRow(row), nil
}

// ErrAssetDocExists means the logical asset name was claimed between the
// caller's existence check and its create-only insert.
var ErrAssetDocExists = errors.New("tên tư liệu vừa được sử dụng")

// CreateAssetDoc insert-claims one freshly allocated asset logical name and
// returns the stored row. A unique-conflict loses to the concurrent winner —
// unlike PutAssetDoc it never replaces an existing document.
func (a *Audio) CreateAssetDoc(ctx context.Context, projectID pgtype.UUID, name string, payload any, blobID pgtype.UUID) (AssetDoc, error) {
	docID, err := randomUUID()
	if err != nil {
		return AssetDoc{}, err
	}
	body, err := marshalPayload(payload)
	if err != nil {
		return AssetDoc{}, err
	}
	n, err := gen.New(a.pool).InsertAssetDocIfAbsent(ctx, gen.InsertAssetDocIfAbsentParams{
		ProjectID:   projectID,
		ID:          docID,
		LogicalName: name,
		Payload:     body,
		BlobID:      blobID,
	})
	if err != nil {
		return AssetDoc{}, sanitizeErr(err)
	}
	if n == 0 {
		return AssetDoc{}, ErrAssetDocExists
	}
	return a.AssetDocByName(ctx, projectID, name)
}

// DeleteAssetDoc removes one asset document. The blob row/object stay behind
// for the maintenance GC — the app never deletes bytes inline.
func (a *Audio) DeleteAssetDoc(ctx context.Context, projectID pgtype.UUID, name string) error {
	n, err := gen.New(a.pool).DeleteProjectDocumentByName(ctx, gen.DeleteProjectDocumentByNameParams{
		ProjectID:   projectID,
		Kind:        "asset",
		LogicalName: name,
	})
	if err != nil {
		return sanitizeErr(err)
	}
	if n == 0 {
		return ErrAssetDocNotFound
	}
	return nil
}

// AddRenderTx records a candidate inside tx.
func (a *Audio) AddRenderTx(ctx context.Context, tx pgx.Tx, projectID, chapterID pgtype.UUID, scriptRevision int32, r Render) (Render, error) {
	return addRender(ctx, gen.New(tx), projectID, chapterID, scriptRevision, r)
}

// SelectRenderTx makes the candidate current inside tx (clear + select).
func (a *Audio) SelectRenderTx(ctx context.Context, tx pgx.Tx, projectID, chapterID pgtype.UUID, scriptRevision int32, segmentID, synthKey string) (Render, error) {
	q := gen.New(tx)
	if _, err := q.ClearSelectedAudioRenders(ctx, gen.ClearSelectedAudioRendersParams{
		ProjectID:      projectID,
		ChapterID:      chapterID,
		ScriptRevision: scriptRevision,
		SegmentID:      segmentID,
	}); err != nil {
		return Render{}, sanitizeErr(err)
	}
	row, err := q.SelectAudioRender(ctx, gen.SelectAudioRenderParams{
		ProjectID:      projectID,
		ChapterID:      chapterID,
		ScriptRevision: scriptRevision,
		SegmentID:      segmentID,
		SynthKey:       synthKey,
	})
	if noRows(err) {
		return Render{}, ErrJobNotFound
	}
	if err != nil {
		return Render{}, sanitizeErr(err)
	}
	return renderFromRow(row), nil
}

// BlobProject resolves a ready blob's object key to its project slug — the
// web BlobDelivery authorizer needs the slug for permission checks.
func (a *Audio) BlobProject(ctx context.Context, key string) (string, error) {
	slug, err := gen.New(a.pool).GetReadyBlobProject(ctx, key)
	if err != nil {
		if noRows(err) {
			return "", ErrJobNotFound
		}
		return "", sanitizeErr(err)
	}
	return slug, nil
}

// SaveScriptBundle writes one script revision and its segment rows in a
// single transaction, mirroring the file backend's atomic WriteScript.
// expectedVersion is the row version the caller read (0 = create): the
// script row is written with UPDATE ... WHERE version = expected, so a
// concurrent edit is a conflict instead of a silent overwrite.
//   - segments that disappeared are deleted together with their renders (the
//     FK on audio_renders forbids orphaning them);
//   - segments whose audible text changed keep their row but lose the
//     *selected* marker on renders keyed to the old text — stale takes stay
//     as historical candidates, never current audio;
//   - payload must already carry the caller's assembled script.
func (a *Audio) SaveScriptBundle(ctx context.Context, scope ScopeRef, script Script, segments []Segment, editedSegmentIDs []string, expectedVersion int64) (Script, error) {
	if len(script.Payload) == 0 {
		return Script{}, fmt.Errorf("script thiếu payload: %w", errInvalidJobInput)
	}
	if err := validateScriptMeta(script); err != nil {
		return Script{}, err
	}
	var saved Script
	err := a.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		var row gen.AudioScript
		var err error
		if expectedVersion == 0 {
			row, err = q.InsertAudioScript(ctx, gen.InsertAudioScriptParams{
				ProjectID:      scope.ProjectID,
				StoryID:        scope.StoryID,
				ChapterID:      script.ChapterID,
				RevisionNo:     script.RevisionNo,
				SourceRevision: script.SourceRevision,
				SourceSha256:   script.SourceSHA256,
				Status:         script.Status,
				Payload:        script.Payload,
			})
			if err != nil {
				if errors.Is(constraintErr(err), errs.ErrStorageConflict) {
					return fmt.Errorf("tạo bản đọc chương: %w", errs.ErrStorageConflict)
				}
				return sanitizeErr(err)
			}
		} else {
			row, err = q.UpdateAudioScriptCAS(ctx, gen.UpdateAudioScriptCASParams{
				ProjectID:      scope.ProjectID,
				ChapterID:      script.ChapterID,
				RevisionNo:     script.RevisionNo,
				Version:        expectedVersion,
				SourceRevision: script.SourceRevision,
				SourceSha256:   script.SourceSHA256,
				Status:         script.Status,
				Payload:        script.Payload,
			})
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("ghi bản đọc chương (version %d): %w", expectedVersion, errs.ErrStorageConflict)
				}
				return sanitizeErr(err)
			}
		}
		saved = scriptFromRow(row)
		oldIDs, err := q.ListAudioSegmentIDs(ctx, gen.ListAudioSegmentIDsParams{
			ProjectID:      scope.ProjectID,
			ChapterID:      script.ChapterID,
			ScriptRevision: script.RevisionNo,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		keep := map[string]bool{}
		for _, seg := range segments {
			keep[seg.SegmentID] = true
		}
		for _, id := range oldIDs {
			if keep[id] {
				continue
			}
			if _, err := q.DeleteAudioSegmentRenders(ctx, gen.DeleteAudioSegmentRendersParams{
				ProjectID:      scope.ProjectID,
				ChapterID:      script.ChapterID,
				ScriptRevision: script.RevisionNo,
				SegmentID:      id,
			}); err != nil {
				return sanitizeErr(err)
			}
			if _, err := q.DeleteAudioSegment(ctx, gen.DeleteAudioSegmentParams{
				ProjectID:      scope.ProjectID,
				ChapterID:      script.ChapterID,
				ScriptRevision: script.RevisionNo,
				SegmentID:      id,
			}); err != nil {
				return sanitizeErr(err)
			}
		}
		for _, seg := range segments {
			if _, err := q.UpsertAudioSegment(ctx, gen.UpsertAudioSegmentParams{
				ProjectID:      scope.ProjectID,
				ChapterID:      script.ChapterID,
				ScriptRevision: script.RevisionNo,
				SegmentID:      seg.SegmentID,
				Ordinal:        seg.Ordinal,
				Speaker:        seg.Speaker,
				TextContent:    seg.Text,
				Payload:        seg.Payload,
			}); err != nil {
				return sanitizeErr(err)
			}
		}
		for _, id := range editedSegmentIDs {
			if _, err := q.ClearSelectedAudioRenders(ctx, gen.ClearSelectedAudioRendersParams{
				ProjectID:      scope.ProjectID,
				ChapterID:      script.ChapterID,
				ScriptRevision: script.RevisionNo,
				SegmentID:      id,
			}); err != nil {
				return sanitizeErr(err)
			}
		}
		return nil
	})
	return saved, err
}
