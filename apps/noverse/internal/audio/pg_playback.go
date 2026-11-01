package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/jackc/pgx/v5"
)

type pgPlaybackInput struct {
	Playback  bool   `json:"playback"`
	Volume    int    `json:"volume"`
	Chapter   int    `json:"chapter"`
	ChapterID string `json:"chapterId"`
	Revision  string `json:"revision"`
}

type pgPlaybackManifest struct {
	Revision string `json:"playbackRevision"`
	Volume   int    `json:"volume"`
	Chapter  int    `json:"chapter"`
}

// processPlaybackJob uses the export runner's lease, immutable blob publisher,
// scratch materialization and the same chapter assembler used by exports. It
// never synthesizes, runs in a GET, or requires a volume export first.
func (s *PGService) processPlaybackJob(ctx, completionCtx context.Context, lease postgres.WorkerLease, job postgres.Job, in pgPlaybackInput, stopHeartbeat func() error) error {
	chapterID, err := postgres.ParseUUID(in.ChapterID)
	if err != nil || in.Volume <= 0 || in.Chapter <= 0 || in.Revision == "" {
		return fmt.Errorf("invalid chapter playback job")
	}
	scope := postgres.ScopeRef{ProjectID: job.ProjectID, StoryID: job.StoryID}
	revision, err := s.deps.Audio.PlaybackRevision(ctx, scope, chapterID, PlaybackProof{})
	if err != nil {
		return err
	}
	finish := func(attach func(context.Context, pgx.Tx, postgres.Job) error) error {
		if err := stopHeartbeat(); err != nil {
			return err
		}
		_, err := s.deps.Jobs.CompleteWith(completionCtx, lease, map[string]any{"playback": true}, 0, attach)
		return err
	}
	if revision.Content != in.Revision || revision.SourceRevision != revision.CurrentRevision {
		return finish(nil)
	}
	// A repeated ACK/restart already published this exact immutable chapter.
	if cached, err := s.deps.Cache.Get(ctx, scope, ChapterAudioCacheKey(in.Volume, in.Chapter), "mp3"); err == nil {
		var manifest pgPlaybackManifest
		if json.Unmarshal(cached.InputManifest, &manifest) == nil && manifest.Revision == in.Revision {
			return finish(nil)
		}
	}
	export := pgExportInput{Volume: in.Volume, Chapters: []int{in.Chapter},
		Source: []SourceChapter{{Chapter: in.Chapter}}, Options: ExportOptions{Format: "mp3"}, Engine: "remote", Model: "remote"}
	scratch, err := s.materializeExport(ctx, scope, export)
	if err != nil {
		return err
	}
	defer scratch.cleanup()
	fileSvc, err := NewService(scratch.layout, NullSynth{Reason: "hosted"})
	if err != nil {
		return err
	}
	defer fileSvc.Close()
	path, err := fileSvc.ensureChapterAudio(ctx, in.Volume, in.Chapter, nil)
	if err != nil {
		return err
	}
	upload, err := s.uploadExportFile(ctx, scope, job.ID, in.Volume, in.Chapter, filepath.Dir(path), ExportFile{Name: filepath.Base(path), Path: filepath.Base(path), Chapter: in.Chapter})
	if err != nil {
		return err
	}
	manifest, err := json.Marshal(pgPlaybackManifest{Revision: in.Revision, Volume: in.Volume, Chapter: in.Chapter})
	if err != nil {
		return err
	}
	return finish(func(ctx context.Context, tx pgx.Tx, _ postgres.Job) error {
		current, err := s.deps.Audio.PlaybackRevisionTx(ctx, tx, scope, chapterID, PlaybackProof{})
		if err != nil {
			return err
		}
		// CAS against every input and the newest render attempt, not just the
		// export worker fence. Superseded work completes without publication.
		if current.Content != in.Revision {
			return nil
		}
		return s.deps.Audio.PutExportCacheTx(ctx, tx, scope, postgres.ExportCacheEntry{
			CacheKey: ChapterAudioCacheKey(in.Volume, in.Chapter), Kind: "mp3", BlobID: upload.blob.ID, InputManifest: manifest,
		}, nil)
	})
}

// PlaybackProof interprets persisted inputs with the same voice/key/pause
// functions as synthesis and chapter assembly, never database row metadata.
type PlaybackProof struct{}

type playbackSegmentInput struct {
	ID        string   `json:"id"`
	Speaker   string   `json:"-"`
	Tone      string   `json:"tone"`
	Key       SynthKey `json:"key"`
	PauseMs   int      `json:"pauseMs"`
	Sample    string   `json:"sample,omitempty"`
	Reference string   `json:"reference,omitempty"`
}

func (p PlaybackProof) inputs(ctx context.Context, db gen.DBTX, scope postgres.ScopeRef, row postgres.Script) ([]playbackSegmentInput, error) {
	q := gen.New(db)
	voiceRows, err := q.ListAudioVoices(ctx, scope.ProjectID)
	if err != nil {
		return nil, err
	}
	voices := make([]postgres.Voice, 0, len(voiceRows))
	for _, v := range voiceRows {
		voices = append(voices, postgres.Voice{ID: v.ID, LegacyID: v.LegacyID, Metadata: v.Metadata, SampleBlobID: v.SampleBlobID, Version: v.Version})
	}
	castRows, err := q.ListAudioCast(ctx, gen.ListAudioCastParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID})
	if err != nil {
		return nil, err
	}
	roles := make([]postgres.CastRole, 0, len(castRows))
	for _, role := range castRows {
		roles = append(roles, postgres.CastRole{RoleKey: role.RoleKey, VoiceID: role.VoiceID, Payload: role.Payload, Version: role.Version})
	}
	cast, _, err := castFromRows(roles, voices)
	if err != nil {
		return nil, err
	}
	doc := pgDoc{Settings: DefaultSettings()}
	settings, err := q.GetAudioSettings(ctx, gen.GetAudioSettingsParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(settings.Payload, &doc); err != nil {
			return nil, err
		}
		if !ValidateQuality(doc.Settings.Quality) {
			doc.Settings.Quality = DefaultSettings().Quality
		}
	}
	voiceMap := make(map[string]ProjectVoice, len(voices))
	samples := make(map[string]string, len(voices))
	for _, voice := range voices {
		var v ProjectVoice
		if err := json.Unmarshal(voice.Metadata, &v); err != nil {
			return nil, err
		}
		v.ID = voice.LegacyID
		voiceMap[v.ID] = v
		if voice.SampleBlobID.Valid {
			samples[v.ID] = postgres.UUIDString(voice.SampleBlobID)
		}
	}
	var script Script
	if err := json.Unmarshal(row.Payload, &script); err != nil {
		return nil, err
	}
	base := make(map[string]Segment, len(script.Segments))
	for _, seg := range script.Segments {
		base[seg.ID] = seg
	}
	segments, err := q.ListAudioSegments(ctx, gen.ListAudioSegmentsParams{ProjectID: scope.ProjectID, ChapterID: row.ChapterID, ScriptRevision: row.RevisionNo})
	if err != nil {
		return nil, err
	}
	out := make([]playbackSegmentInput, 0, len(segments))
	for _, sr := range segments {
		seg := base[sr.SegmentID]
		if err := json.Unmarshal(sr.Payload, &seg); err != nil {
			return nil, err
		}
		seg.ID, seg.Speaker, seg.Text = sr.SegmentID, sr.Speaker, sr.TextContent
		key := pgSegmentKey(seg, cast, voiceMap, doc.Settings, "", "")
		ref := ResolveVoice(cast, seg, voiceMap)
		out = append(out, playbackSegmentInput{ID: seg.ID, Speaker: seg.Speaker, Tone: seg.Tone, Key: key,
			PauseMs: effectivePauseMs(cast, seg), Sample: samples[key.VoiceID], Reference: ref.RefAudio})
	}
	return out, nil
}

func (p PlaybackProof) Fingerprint(ctx context.Context, db gen.DBTX, scope postgres.ScopeRef, row postgres.Script) ([]byte, error) {
	inputs, err := p.inputs(ctx, db, scope, row)
	if err != nil {
		return nil, err
	}
	return json.Marshal(inputs)
}

// LegacyCurrent proves pre-ownership jobs from their actual frozen synthesis
// parameters. Missing/incomplete provenance is superseded, not a wildcard.
func (p PlaybackProof) LegacyCurrent(ctx context.Context, db gen.DBTX, job postgres.Job) (bool, error) {
	var frozen pgTTSInput
	if err := json.Unmarshal(job.Input, &frozen); err != nil {
		return false, nil
	}
	if frozen.SourceRevision <= 0 || len(frozen.Segments) == 0 {
		return false, nil
	}
	chapter, err := postgres.ParseUUID(frozen.ChapterID)
	if err != nil {
		return false, nil
	}
	scriptRow, err := gen.New(db).LatestAudioScript(ctx, gen.LatestAudioScriptParams{ProjectID: job.ProjectID, ChapterID: chapter})
	if err != nil {
		return false, err
	}
	row := postgres.Script{ChapterID: chapter, RevisionNo: scriptRow.RevisionNo, SourceRevision: scriptRow.SourceRevision, Payload: scriptRow.Payload}
	if row.RevisionNo != frozen.ScriptRevision || row.SourceRevision != frozen.SourceRevision {
		return false, nil
	}
	scope := postgres.ScopeRef{ProjectID: job.ProjectID, StoryID: job.StoryID}
	current, err := p.inputs(ctx, db, scope, row)
	if err != nil {
		return false, err
	}
	if len(current) != len(frozen.Segments) {
		return false, nil
	}
	var provenance struct {
		Segments []map[string]json.RawMessage `json:"segments"`
	}
	if err := json.Unmarshal(job.Input, &provenance); err != nil {
		return false, nil
	}
	byID := make(map[string]playbackSegmentInput, len(current))
	for _, seg := range current {
		byID[seg.ID] = seg
	}
	seen := make(map[string]bool, len(frozen.Segments))
	for i, seg := range frozen.Segments {
		for _, field := range []string{"id", "speaker", "text", "voice", "tone", "speed", "pitch", "quality", "engine", "model", "synthKey"} {
			value, present := provenance.Segments[i][field]
			if !present || string(value) == "null" {
				return false, nil
			}
		}
		want, ok := byID[seg.ID]
		if !ok || seen[seg.ID] || seg.Engine == "" || seg.Model == "" {
			return false, nil
		}
		seen[seg.ID] = true
		// Legacy snapshots did not freeze sample identity: a custom sample
		// cannot be proven historical merely from its current voice ID.
		if want.Sample != "" {
			return false, nil
		}
		key := want.Key
		key.Engine, key.Model = seg.Engine, seg.Model
		if seg.Speaker != want.Speaker || seg.Tone != want.Tone || seg.Text != key.Text || seg.Voice != key.VoiceID || seg.Speed != key.Speed ||
			seg.Pitch != key.Pitch || seg.Quality != key.Quality || seg.SynthKey != key.Hash() {
			return false, nil
		}
	}
	return true, nil
}

func (p PlaybackProof) Current(ctx context.Context, db gen.DBTX, job postgres.Job, revision postgres.PlaybackRevision) (bool, error) {
	var frozen struct {
		ScriptRevision int32  `json:"scriptRevision"`
		Ownership      string `json:"ownership"`
	}
	if err := json.Unmarshal(job.Input, &frozen); err != nil {
		return false, err
	}
	if revision.Revision != frozen.ScriptRevision || revision.SourceRevision != revision.CurrentRevision ||
		(revision.LatestJob.Valid && revision.LatestJob != job.ID) {
		return false, nil
	}
	if frozen.Ownership != "" {
		return frozen.Ownership == revision.Ownership, nil
	}
	return p.LegacyCurrent(ctx, db, job)
}
