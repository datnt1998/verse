package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// pgRevision is the audio_scripts.revision_no a loaded Script came from. It is
// not part of the payload: the row key is authoritative, matching how the
// file backend knows its script by path.
type pgScriptMeta struct {
	Revision int32
	Version  int64
}

func (s *PGService) chapterRef(ctx context.Context, scope postgres.ScopeRef, chapter int) (postgres.ChapterRef, error) {
	ref, err := s.deps.Nar.ChapterRefByNumber(ctx, scope, chapter)
	if err != nil {
		return postgres.ChapterRef{}, pgErr(err)
	}
	return ref, nil
}

// chapterList returns the committed chapter numbers (for per-role stale scans).
func (s *PGService) chapterList(ctx context.Context, scope postgres.ScopeRef) ([]int, error) {
	infos, err := s.deps.Nar.ListChapters(ctx, s.deps.Project, s.deps.Story, nil)
	if err != nil {
		return nil, pgErr(err)
	}
	out := make([]int, 0, len(infos))
	for _, info := range infos {
		if info.CurrentRevision != nil {
			out = append(out, info.Number)
		}
	}
	return out, nil
}

// loadScript assembles the domain script for a chapter from the tables:
// payload for provenance, audio_segments for the segments, audio_renders for
// the selected audio of each segment. nil when the chapter/script does not
// exist — same contract as ReadScript.
func (s *PGService) loadScript(ctx context.Context, scope postgres.ScopeRef, chapter int) (*Script, *pgScriptMeta, error) {
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return nil, nil, err
	}
	return s.loadScriptRef(ctx, scope, ref)
}

func (s *PGService) loadScriptRef(ctx context.Context, scope postgres.ScopeRef, ref postgres.ChapterRef) (*Script, *pgScriptMeta, error) {
	if !ref.Committed && !ref.ID.Valid {
		return nil, nil, nil
	}
	row, err := s.deps.Audio.LatestScript(ctx, scope.ProjectID, ref.ID)
	if err != nil {
		if errors.Is(err, postgres.ErrJobNotFound) {
			return nil, nil, nil
		}
		return nil, nil, pgErr(err)
	}
	var script Script
	if err := json.Unmarshal(row.Payload, &script); err != nil {
		return nil, nil, fmt.Errorf("bản đọc chương %d hỏng: %w", ref.Number, err)
	}
	segRows, err := s.deps.Audio.Segments(ctx, scope.ProjectID, ref.ID, row.RevisionNo)
	if err != nil {
		return nil, nil, pgErr(err)
	}
	renderRows, err := s.deps.Audio.Renders(ctx, scope.ProjectID, ref.ID, row.RevisionNo)
	if err != nil {
		return nil, nil, pgErr(err)
	}
	selected := map[string]postgres.Render{}
	for _, r := range renderRows {
		if r.Selected {
			selected[r.SegmentID] = r
		}
	}
	script.pgRevision = row.RevisionNo
	script.Segments = make([]Segment, 0, len(segRows))
	for _, sr := range segRows {
		var seg Segment
		if err := json.Unmarshal(sr.Payload, &seg); err != nil {
			seg = Segment{}
		}
		seg.ID = sr.SegmentID
		seg.Speaker = sr.Speaker
		seg.Text = sr.Text
		if r, ok := selected[sr.SegmentID]; ok {
			seg.Rendered = pgSegmentAudio(r)
		}
		script.Segments = append(script.Segments, seg)
	}
	return &script, &pgScriptMeta{Revision: row.RevisionNo, Version: row.Version}, nil
}

// pgSegmentAudio maps the selected render row to the per-segment audio view.
func pgSegmentAudio(r postgres.Render) *SegmentAudio {
	out := &SegmentAudio{Key: r.SynthKey, DurationSec: r.DurationSec, RenderedAt: &r.CreatedAt}
	var params struct {
		VoiceID string  `json:"voiceId"`
		Voice   string  `json:"voice"`
		Tone    string  `json:"tone"`
		Speed   float64 `json:"speed"`
		Pitch   float64 `json:"pitch"`
		Engine  string  `json:"engine"`
		Model   string  `json:"model"`
		Quality string  `json:"quality"`
	}
	if err := json.Unmarshal(r.Parameters, &params); err == nil {
		out.VoiceID = params.VoiceID
		if out.VoiceID == "" {
			out.VoiceID = params.Voice
		}
		out.Tone = params.Tone
		out.Speed = params.Speed
		if out.Speed == 0 {
			out.Speed = 1
		}
		out.Pitch = params.Pitch
		if out.Pitch == 0 {
			out.Pitch = 1
		}
		out.Engine = params.Engine
		out.Model = params.Model
		out.Quality = params.Quality
	}
	return out
}

// Script returns the raw script (nil when absent).
func (s *PGService) Script(ctx context.Context, volume, chapter int) (*Script, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	script, _, err := s.loadScript(ctx, scope, chapter)
	return script, err
}

// scriptSegmentPayload is the segment row's extra fields (everything the
// script revision needs that the column set does not cover).
func scriptSegmentPayload(seg Segment) []byte {
	p := struct {
		Source       string        `json:"source,omitempty"`
		PauseMs      int           `json:"pauseMs"`
		Tone         string        `json:"tone"`
		Normalized   []Change      `json:"normalized,omitempty"`
		SpeakerGuess []SpeakerPick `json:"speakerGuess,omitempty"`
		Flags        []string      `json:"flags,omitempty"`
	}{
		Source: seg.Source, PauseMs: seg.PauseMs, Tone: seg.Tone,
		Normalized: seg.Normalized, SpeakerGuess: seg.SpeakerGuess, Flags: seg.Flags,
	}
	body, err := json.Marshal(p)
	if err != nil {
		return []byte("{}")
	}
	return body
}

// saveScriptPayload updates the script row only (queue-time status flips,
// segments stay untouched). Callers that accept an external ETag use
// persistScript instead, which carries the expected version into the write.
func (s *PGService) saveScriptPayload(ctx context.Context, scope postgres.ScopeRef, script *Script) error {
	ref, err := s.chapterRef(ctx, scope, script.Chapter)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(script)
	if err != nil {
		return err
	}
	_, err = s.deps.Audio.SaveScript(ctx, scope, postgres.Script{
		ChapterID:      ref.ID,
		RevisionNo:     script.pgRevision,
		SourceRevision: int32(ref.CurrentRevision),
		SourceSHA256:   script.SourceHash,
		Status:         script.Status,
		Payload:        payload,
	})
	return pgErr(err)
}

// persistScript mirrors WriteScript on the file backend: script payload plus
// segment rows in one transaction; deleted segments lose their renders,
// edited segments lose only the selected marker (takes stay as candidates).
// expected is the row version the write must find; 0 creates a new row.
func (s *PGService) persistScript(ctx context.Context, scope postgres.ScopeRef, ref postgres.ChapterRef, script *Script, edited []string, expected int64) error {
	payload, err := json.Marshal(script)
	if err != nil {
		return err
	}
	segments := make([]postgres.Segment, 0, len(script.Segments))
	for i, seg := range script.Segments {
		segments = append(segments, postgres.Segment{
			SegmentID: seg.ID, Ordinal: int32(i), Speaker: seg.Speaker,
			Text: seg.Text, Payload: scriptSegmentPayload(seg),
		})
	}
	_, err = s.deps.Audio.SaveScriptBundle(ctx, scope, postgres.Script{
		ChapterID:      ref.ID,
		RevisionNo:     script.pgRevision,
		SourceRevision: int32(ref.CurrentRevision),
		SourceSHA256:   script.SourceHash,
		Status:         script.Status,
		Payload:        payload,
	}, segments, edited, expected)
	return pgErr(err)
}

// ScriptView mirrors Service.ScriptView: each segment's state comes from the
// selected render compared against the wanted synth key.
func (s *PGService) ScriptView(ctx context.Context, volume, chapter int, currentHash string) (*ScriptView, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	script, _, err := s.loadScript(ctx, scope, chapter)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrNotFound
	}
	settings := s.Settings()
	cast, err := s.castForRender(ctx)
	if err != nil {
		return nil, err
	}
	applyVoiceFlags(script, cast)
	voices, err := s.Voices(ctx)
	if err != nil {
		return nil, err
	}
	voiceMap := VoiceMap(voices)
	engine, model := s.engineIdent()
	view := &ScriptView{
		Chapter:    script.Chapter,
		Volume:     script.Volume,
		Status:     script.Status,
		LockedAt:   script.LockedAt,
		SourceHash: script.SourceHash,
		Stale:      script.Stale(currentHash),
		Segments:   make([]SegmentView, 0, len(script.Segments)),
	}
	view.ChapterChanged, view.ScriptChanged, view.StaleReason = script.freshness(currentHash)
	for _, seg := range script.Segments {
		key := pgSegmentKey(seg, cast, voiceMap, settings, engine, model)
		sv := SegmentView{Segment: seg}
		sv.Normalized = seg.Normalized
		if sv.Normalized == nil {
			sv.Normalized = []Change{}
		}
		sv.Flags = seg.Flags
		if sv.Flags == nil {
			sv.Flags = []string{}
		}
		switch {
		case seg.Rendered == nil:
			sv.Audio = &SegmentAudioView{State: "none", URL: s.segmentURL(script.Volume, script.Chapter, seg.ID)}
		case seg.Rendered.Key != key.Hash():
			sv.Changed = true
			sv.Audio = &SegmentAudioView{State: "stale", URL: s.segmentURL(script.Volume, script.Chapter, seg.ID), DurationSec: seg.Rendered.DurationSec}
		default:
			sv.Audio = &SegmentAudioView{State: "ready", URL: s.segmentURL(script.Volume, script.Chapter, seg.ID), DurationSec: seg.Rendered.DurationSec}
		}
		view.Segments = append(view.Segments, sv)
	}
	return view, nil
}

// ScriptVersion is the ETag token for one chapter script.
func (s *PGService) ScriptVersion(volume, chapter int) string {
	scope, err := s.resolve(context.Background())
	if err != nil {
		return "pg-script-0"
	}
	_, meta, err := s.loadScript(context.Background(), scope, chapter)
	if err != nil || meta == nil {
		return "pg-script-0"
	}
	return "pg-script-" + strconv.FormatInt(meta.Version, 10)
}

// scriptExpectedVersion parses the ETag token into the row version a write
// must find. 0 means "create"; an unknown token can never match a row.
func scriptExpectedVersion(token string) (int64, bool) {
	token = strings.TrimSpace(token)
	if unq, err := strconv.Unquote(token); err == nil {
		token = unq
	}
	const prefix = "pg-script-"
	if !strings.HasPrefix(token, prefix) {
		return 0, false
	}
	version, err := strconv.ParseInt(strings.TrimPrefix(token, prefix), 10, 64)
	if err != nil || version < 0 {
		return 0, false
	}
	return version, true
}

// SaveScript mirrors Service.SaveScript: edited segments lose their selected
// marker (their old takes stay as candidates), removed segments are dropped
// with their renders; unchanged segments keep their audio. expectedVersion is
// the ETag the caller read and becomes the script row's CAS predicate, so two
// editors cannot both publish.
func (s *PGService) SaveScript(ctx context.Context, volume, chapter int, expectedVersion string, edit ScriptEdit) (*Script, error) {
	expected, ok := scriptExpectedVersion(expectedVersion)
	if !ok {
		return nil, errs.ErrStorageConflict
	}
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return nil, err
	}
	script, _, err := s.loadScriptRef(ctx, scope, ref)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrNotFound
	}
	if len(edit.Segments) == 0 {
		return nil, conflict("Bản đọc phải còn ít nhất một đoạn.")
	}
	cast, err := s.castForRender(ctx)
	if err != nil {
		return nil, err
	}
	prev := map[string]Segment{}
	script.captureLegacyRenderSource()
	for _, seg := range script.Segments {
		prev[seg.ID] = seg
	}
	for i := range edit.Segments {
		edit.Segments[i].Text = strings.TrimSpace(edit.Segments[i].Text)
		if edit.Segments[i].Text == "" {
			return nil, conflict("Đoạn %d không có chữ để đọc.", i+1)
		}
	}
	edit.Segments = foldUnspeakable(edit.Segments)
	if len(edit.Segments) == 0 {
		return nil, conflict("Bản đọc chỉ có dấu câu, không có chữ để đọc.")
	}
	out := make([]Segment, 0, len(edit.Segments))
	edited := []string{}
	for i, seg := range edit.Segments {
		seg.Source = strings.TrimSpace(seg.Source)
		seg.Speaker = strings.TrimSpace(seg.Speaker)
		if seg.Speaker == "" {
			seg.Speaker = NarratorID
		}
		seg.PauseMs = clampPause(seg.PauseMs)
		seg.Tone = strings.TrimSpace(seg.Tone)
		seg.ID = strings.TrimSpace(seg.ID)
		if seg.ID == "" {
			seg.ID = fmt.Sprintf("s%04d", i+1)
		}
		old, hadOld := prev[seg.ID]
		if hadOld && old.Text == seg.Text {
			seg.Rendered = old.Rendered
			seg.Normalized = old.Normalized
			seg.Source = old.Source
		} else {
			seg.Rendered = nil
			seg.Normalized = nil
			if hadOld {
				edited = append(edited, seg.ID)
			}
		}
		flags := make([]string, 0, 3)
		for _, f := range seg.Flags {
			switch f {
			case FlagUnknownSpeaker, FlagNoVoice:
				flags = append(flags, f)
			}
		}
		if countWords(seg.Source) > LongSentenceWords || (seg.Source == "" && countWords(seg.Text) > LongSentenceWords) {
			flags = append(flags, FlagLongSentence)
		}
		if len(englishWords(seg.Text)) > 0 {
			flags = append(flags, FlagEnglish)
		}
		seg.Flags = sortFlags(flags)
		out = append(out, seg)
	}
	script.Segments = out
	applyVoiceFlags(script, cast)
	if err := s.persistScript(ctx, scope, ref, script, edited, expected); err != nil {
		return nil, err
	}
	fresh, _, err := s.loadScriptRef(ctx, scope, ref)
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

// Lock mirrors Service.Lock: approve the script and, when AutoOnLock is on,
// queue the chapter for synthesis.
func (s *PGService) Lock(ctx context.Context, volume, chapter int, currentHash string) (*Script, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return nil, err
	}
	script, meta, err := s.loadScriptRef(ctx, scope, ref)
	if err != nil {
		return nil, err
	}
	if script == nil || len(script.Segments) == 0 {
		return nil, conflict("Chương %d chưa có bản đọc để chốt.", chapter)
	}
	if err := foldScriptUnspeakable(script); err != nil {
		return nil, err
	}
	now := time.Now()
	script.Status = StatusLocked
	script.LockedAt = &now
	if err := s.persistScript(ctx, scope, ref, script, nil, meta.Version); err != nil {
		return nil, err
	}
	if s.Settings().AutoOnLock {
		if _, err := s.Enqueue(ctx, volume, chapter, currentHash); err != nil && !isNoopEnqueue(err) {
			return nil, err
		}
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	return script, nil
}

// Unlock mirrors Service.Unlock: back to review and cancel pending jobs.
func (s *PGService) Unlock(ctx context.Context, volume, chapter int) (*Script, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return nil, err
	}
	script, meta, err := s.loadScriptRef(ctx, scope, ref)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrNotFound
	}
	script.Status = StatusReview
	script.LockedAt = nil
	if err := s.persistScript(ctx, scope, ref, script, nil, meta.Version); err != nil {
		return nil, err
	}
	jobs, err := s.queueSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	for _, j := range jobs {
		if j.Chapter != chapter {
			continue
		}
		if j.State == JobQueued || j.State == JobRendering || j.State == JobPaused {
			if err := s.Cancel(j.ID); err != nil {
				return nil, err
			}
		}
	}
	return script, nil
}

// Compose needs the story model (LLM segmentation); hosted keeps it a 501
// capability instead of pretending a script was composed.
func (s *PGService) Compose(source []SourceChapter, volume int, chapters []int, composer Composer, ref CharacterRef) error {
	return pgUnsupported("soạn bản đọc bằng mô hình")
}

// Refresh re-composes one chapter script: same hosted limitation as Compose.
func (s *PGService) Refresh(source []SourceChapter, volume, chapter int, composer Composer, ref CharacterRef) error {
	return pgUnsupported("soạn lại bản đọc bằng mô hình")
}

// ── views ────────────────────────────────────────────────────────────────────

// Volumes mirrors Service.Volumes with data from the tables; RTF is unknown
// (the engine lives on the worker), so updateEtaSec stays absent.
func (s *PGService) Volumes(ctx context.Context, source []SourceChapter, volume int) (VolumeView, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return VolumeView{}, pgErr(err)
	}
	settings := s.Settings()
	cast, err := s.castForRender(ctx)
	if err != nil {
		return VolumeView{}, err
	}
	voices, err := s.Voices(ctx)
	if err != nil {
		return VolumeView{}, err
	}
	voiceMap := VoiceMap(voices)
	engine, model := s.engineIdent()
	jobs, err := s.queueSnapshot(ctx)
	if err != nil {
		return VolumeView{}, err
	}
	out := VolumeView{Chapters: make([]ChapterView, 0, len(source))}
	for _, sc := range source {
		script, _, err := s.loadScript(ctx, scope, sc.Chapter)
		if err != nil {
			return VolumeView{}, err
		}
		hash := ""
		if strings.TrimSpace(sc.Text) != "" {
			hash = SourceHash(sc.Text)
		}
		view := ChapterView{
			Chapter: sc.Chapter,
			Title:   sc.Title,
			Status:  DeriveStatus(script, false, hash),
		}
		if script != nil {
			view.ChapterChanged, view.ScriptChanged, view.StaleReason = script.freshness(hash)
			applyVoiceFlags(script, cast)
			view.LockedAt = script.LockedAt
			view.Issues = script.IssueSegments()
			view.Segments = len(script.Segments)
			view.Speakers = script.SpeakerCount()
			done, duration := script.AudioStats()
			view.ChangedSegments = script.ChangedSegments(func(seg Segment) string {
				return pgSegmentKey(seg, cast, voiceMap, settings, engine, model).Hash()
			})
			audioView := &ChapterAudioView{SegmentsDone: done, Segments: len(script.Segments)}
			if len(script.Segments) > 0 {
				audioView.Percent = float64(done) / float64(len(script.Segments)) * 100
			}
			if done == len(script.Segments) && len(script.Segments) > 0 {
				d := duration
				view.DurationSec = &d
				audioView.DurationSec = &d
				audioView.Version = chapterHash(script, cast)[:16]
			}
			view.Audio = audioView
		}
		for _, j := range jobs {
			if j.Volume != volume || j.Chapter != sc.Chapter {
				continue
			}
			if view.Audio == nil {
				view.Audio = &ChapterAudioView{}
			}
			if j.State == JobRendering || j.State == JobPaused || j.State == JobQueued {
				view.Audio.Rendering = j.State == JobRendering
				if j.Segments > 0 {
					view.Audio.Percent = float64(j.SegmentsDone) / float64(j.Segments) * 100
				}
			}
		}
		out.Chapters = append(out.Chapters, view)
	}
	return out, nil
}

// Timeline mirrors Service.Timeline reading scripts and durable jobs.
func (s *PGService) Timeline(ctx context.Context, source []SourceChapter, volume int) (QueueView, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return QueueView{}, pgErr(err)
	}
	out := QueueView{Jobs: []Job{}, Chapters: make([]TimelineChapter, 0, len(source))}
	jobs, err := s.queueSnapshot(ctx)
	if err != nil {
		return QueueView{}, err
	}
	for _, job := range jobs {
		if job.Volume == volume {
			out.Jobs = append(out.Jobs, job)
		}
	}
	out.QualityETASec = map[string]float64{}
	for _, job := range out.Jobs {
		if job.Volume == volume && job.State == JobRendering && job.Step != "compose" && job.Quality != "" && job.ETASec > 0 && job.Percent > 0 && job.Percent < 100 {
			out.QualityETASec[job.Quality] = job.ETASec / (1 - job.Percent/100)
		}
	}
	settings := s.Settings()
	madeTotal, madeCount, estimatedTotal, estimatedCount := 0.0, 0, 0.0, 0
	for _, sc := range source {
		script, _, err := s.loadScript(ctx, scope, sc.Chapter)
		if err != nil {
			return QueueView{}, err
		}
		chapter := TimelineChapter{Chapter: sc.Chapter, Title: sc.Title, State: "unlocked", Quality: settings.Quality}
		if script != nil {
			chapter.ChapterChanged, chapter.ScriptChanged, chapter.StaleReason = script.freshness(SourceHash(sc.Text))
			chapter.EstSec = script.EstimatedDuration()
			chapter.Segments = len(script.Segments)
			done, duration := script.AudioStats()
			chapter.Segment = done
			out.MadeSec += duration
			if script.Status == StatusLocked {
				chapter.State = "queued"
			}
			if len(script.Segments) > 0 {
				chapter.Pct = float64(done) / float64(len(script.Segments)) * 100
				if done == len(script.Segments) && script.Status == StatusLocked {
					chapter.State, chapter.Sec = "done", &duration
					madeTotal += duration
					madeCount++
				}
			}
		}
		if chapter.EstSec <= 0 {
			chapter.EstSec = (&Script{Segments: []Segment{{Text: sc.Text}}}).EstimatedDuration()
		}
		if chapter.EstSec > 0 {
			estimatedTotal += chapter.EstSec
			estimatedCount++
		}
		for i := len(out.Jobs) - 1; i >= 0; i-- {
			job := out.Jobs[i]
			if job.Volume != volume || job.Chapter != sc.Chapter {
				continue
			}
			chapter.Kind = job.Kind
			chapter.Position = job.Position
			if chapter.State != "done" && (chapter.State != "unlocked" || job.NeedsScript) {
				if job.State == JobQueued {
					chapter.State = "queued"
				} else if job.State == JobError {
					chapter.State = "error"
				} else if job.State == JobRendering || (job.State == JobPaused && (job.Percent > 0 || job.CurrentText != "")) {
					chapter.State = "rendering"
				}
				chapter.Pct = job.Percent
				chapter.Segment, chapter.Segments, chapter.ETASec = job.SegmentsDone, job.Segments, job.ETASec
				chapter.Reason, chapter.At = job.Error, job.ErrorAt
				if job.Quality != "" {
					chapter.Quality = job.Quality
				}
			}
			break
		}
		out.Chapters = append(out.Chapters, chapter)
	}
	fallback := 1.0
	if madeCount > 0 {
		fallback = madeTotal / float64(madeCount)
	} else if estimatedCount > 0 {
		fallback = estimatedTotal / float64(estimatedCount)
	}
	for i := range out.Chapters {
		chapter := &out.Chapters[i]
		if chapter.EstSec <= 0 {
			chapter.EstSec = fallback
		}
		if chapter.Sec != nil && *chapter.Sec > 0 {
			out.EstTotalSec += *chapter.Sec
		} else {
			out.EstTotalSec += chapter.EstSec
		}
	}
	return out, nil
}
