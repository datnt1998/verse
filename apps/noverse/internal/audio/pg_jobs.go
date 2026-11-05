package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// Gói AK on Postgres: the web queue keeps one durable jobs row per (chapter,
// attempt) with kind/redo/updateEtaSec/segments frozen in the input. Position
// is recomputed per volume on read exactly like Queue.Snapshot does for the
// file backend.

// pgTTSInput is the frozen job input the web layer writes at enqueue time; it
// is the same snapshot the remote worker receives verbatim.
type pgTTSInput struct {
	Kind           string         `json:"kind"`
	Chapter        int            `json:"chapter"`
	Volume         int            `json:"volume"`
	ChapterID      string         `json:"chapterId"`
	ScriptRevision int32          `json:"scriptRevision"`
	SourceRevision int32          `json:"sourceRevision"`
	Ownership      string         `json:"ownership,omitempty"`
	Redo           bool           `json:"redo"`
	Quality        string         `json:"quality"`
	UpdateEtaSec   float64        `json:"updateEtaSec,omitempty"`
	Segments       []pgTTSSegment `json:"segments"`
	// NeedsScript marks a preparation row: the chapter was queued before its
	// bản đọc exists (or the source changed since it was composed), so the web
	// process must compose + lock the script before the remote worker may claim
	// the job. Such a row has no segments yet and stays invisible to the
	// remote claim until CompletePreparationWith replaces this input.
	NeedsScript bool `json:"needsScript,omitempty"`
}

// pgTTSSegment is one speakable unit with exact synthesis parameters; the
// field names match the worker protocol's Segment shape.
type pgTTSSegment struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Speaker  string  `json:"speaker"`
	Voice    string  `json:"voice"`
	Tone     string  `json:"tone"`
	Quality  string  `json:"quality"`
	Speed    float64 `json:"speed"`
	Pitch    float64 `json:"pitch"`
	Engine   string  `json:"engine"`
	Model    string  `json:"model"`
	SynthKey string  `json:"synthKey"`
	Select   *bool   `json:"select,omitempty"`
}

func (s *PGService) engineIdent() (engine, model string) {
	engine = s.deps.Engine
	if engine == "" {
		engine = "remote"
	}
	model = s.deps.Model
	if model == "" {
		model = "remote"
	}
	return engine, model
}

// pgSegmentKey computes the synth key for one segment under the current cast,
// settings and frozen engine/model identity. The hosted synth text is the
// segment text itself (remote cue support is unknown), tone stays a parameter.
func pgSegmentKey(seg Segment, cast *Cast, voices map[string]ProjectVoice, settings Settings, engine, model string) SynthKey {
	ref := ResolveVoice(cast, seg, voices)
	tone := seg.Tone
	if cast != nil && cast.Mode == ModeSingle && !cast.Single.KeepTones {
		tone = ""
	}
	return SynthKey{
		Text: seg.Text, VoiceID: ref.VoiceID, Tone: tone,
		Speed: 1, Pitch: ref.Pitch, Quality: settings.Quality,
		Engine: engine, Model: model,
	}
}

// pgSegmentVoice resolves the voice/pitch pair for the worker payload.
func pgSegmentVoice(seg Segment, cast *Cast, voices map[string]ProjectVoice) (voiceID string, pitch float64) {
	ref := ResolveVoice(cast, seg, voices)
	return ref.VoiceID, ref.Pitch
}

// queueSnapshot reads durable TTS jobs of this story as audio.Job rows.
func (s *PGService) queueSnapshot(ctx context.Context) ([]Job, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	rows, err := s.deps.Jobs.List(ctx, scope)
	if err != nil {
		return nil, pgErr(err)
	}
	out := make([]Job, 0, len(rows))
	waiting := map[int]int{}
	for _, row := range rows {
		switch row.Kind {
		case postgres.JobTTS:
		case postgres.JobAudioExport:
			var playback pgPlaybackInput
			if err := json.Unmarshal(row.Input, &playback); err != nil {
				return nil, err
			}
			if playback.Playback {
				continue
			}
		default:
			continue
		}
		job := pgJobView(row)
		if job.State == JobQueued && row.Kind == postgres.JobTTS {
			waiting[job.Volume]++
			job.Position = waiting[job.Volume]
		}
		out = append(out, job)
	}
	return out, nil
}

// Queue returns the durable queue snapshot.
func (s *PGService) Queue() []Job {
	jobs, err := s.queueSnapshot(context.Background())
	if err != nil {
		return []Job{}
	}
	return jobs
}

// pgJobView maps a durable jobs row to the public audio.Job shape.
func pgJobView(row postgres.Job) Job {
	var in pgTTSInput
	_ = json.Unmarshal(row.Input, &in)
	job := Job{
		ID:        postgres.UUIDString(row.ID),
		Volume:    in.Volume,
		Chapter:   in.Chapter,
		Kind:      in.Kind,
		Quality:   in.Quality,
		ETASec:    in.UpdateEtaSec,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if job.Kind == "" {
		if row.Kind == postgres.JobAudioExport {
			job.Kind = jobKindExport
		} else {
			job.Kind = JobKindCreate
		}
	}
	if row.ProgressTotal != nil {
		job.Segments = int(*row.ProgressTotal)
	}
	if job.Segments == 0 {
		job.Segments = len(in.Segments)
	}
	job.SegmentsDone = int(row.ProgressDone)
	switch row.State {
	case postgres.JobQueued, postgres.JobWaiting:
		job.State = JobQueued
	case postgres.JobRunning:
		job.State = JobRendering
	case postgres.JobPaused:
		job.State = JobPaused
	case postgres.JobCompleted:
		job.State = JobDone
		job.Percent = 100
	case postgres.JobCancelled:
		job.State = JobCanceled
	case postgres.JobFailed:
		job.State = JobError
		job.Error = pgJobErrorText(row.Error)
		at := row.FinishedAt
		if at == nil {
			at = &row.UpdatedAt
		}
		job.ErrorAt = at
	}
	if job.Segments > 0 && job.State != JobDone {
		job.Percent = float64(job.SegmentsDone) / float64(job.Segments) * 100
	}
	// GenPickEmpty contract: a preparation row reports the compose step and no
	// segment/ETA numbers (they do not exist yet); a render row reports render.
	// Export rows keep their existing shape.
	if row.Kind == postgres.JobTTS {
		job.NeedsScript = in.NeedsScript
		switch {
		case in.NeedsScript:
			job.Step = "compose"
			job.Segments, job.SegmentsDone, job.Percent = 0, 0, 0
		case job.State != JobDone && job.State != JobCanceled:
			job.Step = "render"
		}
	}
	return job
}

// pgJobErrorText unwraps the stored error JSON to a display string.
func pgJobErrorText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var v struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &v); err == nil {
		if v.Message != "" {
			return v.Message
		}
		if v.Error != "" {
			return v.Error
		}
	}
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain
	}
	return string(raw)
}

// errChapterDone means every segment already has a matching selected render;
// a non-redo job would have nothing to do.
var errChapterDone = errors.New("chương này đã có đủ giọng, không có đoạn nào cần đọc lại")

// isNoopEnqueue reports the "already fully voiced" soft stop used when a
// role-change sweep queues updates chapter by chapter.
func isNoopEnqueue(err error) bool { return errors.Is(err, errChapterDone) }

// pgQueueIntent is the frozen work a caller wants queued for one chapter. A
// durable row is only reusable when every field that changes what the worker
// would synthesize still matches: an active row queued before a chapter,
// script or cast edit renders the old inputs, and its ACK is refused, so the
// chapter would silently finish with no audio.
type pgQueueIntent struct {
	ChapterID      string
	Volume         int
	Chapter        int
	Kind           string
	Quality        string
	SourceRevision int32
	ScriptRevision int32
	Redo           bool
	NeedsScript    bool
	Ownership      string
	Segments       []pgTTSSegment
}

// pgQueueSlot is the enqueue decision for one chapter: reuse an identical
// active row, rewrite a stale one in place, or insert a new attempt.
type pgQueueSlot struct {
	reuse    string
	replace  pgtype.UUID
	attempts int
}

func pgJobActiveState(state postgres.JobState) bool {
	switch state {
	case postgres.JobQueued, postgres.JobRunning, postgres.JobPaused:
		return true
	}
	return false
}

func pgSameSegments(a, b []pgTTSSegment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.ID != y.ID || x.Text != y.Text || x.Speaker != y.Speaker ||
			x.Voice != y.Voice || x.Tone != y.Tone || x.Quality != y.Quality ||
			x.Speed != y.Speed || x.Pitch != y.Pitch || x.Engine != y.Engine ||
			x.Model != y.Model || x.SynthKey != y.SynthKey {
			return false
		}
		if (x.Select == nil) != (y.Select == nil) {
			return false
		}
		if x.Select != nil && *x.Select != *y.Select {
			return false
		}
	}
	return true
}

// pgQueueIntentMatches reports whether the frozen row input is exactly the work
// the caller is asking for now. Preparation rows are compared on the chapter
// source revision (the script is about to be recomposed and merged); render
// rows additionally pin the script revision, ownership fingerprint and the
// segment set so a stale voice/cast/quality can never be reused.
func pgQueueIntentMatches(in pgTTSInput, want pgQueueIntent) bool {
	if in.ChapterID != want.ChapterID || in.Chapter != want.Chapter ||
		(want.Volume > 0 && in.Volume != want.Volume) {
		return false
	}
	kind := in.Kind
	if kind == "" {
		kind = JobKindCreate
	}
	if kind != want.Kind || in.Quality != want.Quality || in.Redo != want.Redo ||
		in.NeedsScript != want.NeedsScript {
		return false
	}
	if want.NeedsScript {
		// A preparation for a later accepted revision already covers this
		// chapter: the runner composes the current source, so never downgrade
		// it to an older request that raced AcceptRecord.
		return in.SourceRevision >= want.SourceRevision
	}
	if in.SourceRevision != want.SourceRevision {
		return false
	}
	if in.ScriptRevision != want.ScriptRevision || !pgSameSegments(in.Segments, want.Segments) {
		return false
	}
	return want.Ownership == "" || in.Ownership == want.Ownership
}

// chapterQueueSlot decides how to enqueue one chapter. An identical active row
// is reused (resuming a paused one); any other active row is returned for an
// atomic in-place supersede, so a chapter never keeps two active jobs and the
// old row's worker/preparation fence dies with it.
func (s *PGService) chapterQueueSlot(ctx context.Context, scope postgres.ScopeRef, want pgQueueIntent) (pgQueueSlot, error) {
	var slot pgQueueSlot
	rows, err := s.deps.Jobs.List(ctx, scope)
	if err != nil {
		return slot, pgErr(err)
	}
	active := make([]postgres.Job, 0, 2)
	for _, row := range rows {
		if row.Kind != postgres.JobTTS {
			continue
		}
		var in pgTTSInput
		if err := json.Unmarshal(row.Input, &in); err != nil {
			continue
		}
		if in.Chapter != want.Chapter || (want.Volume > 0 && in.Volume != want.Volume) {
			continue
		}
		slot.attempts++
		if pgJobActiveState(row.State) {
			active = append(active, row)
		}
	}
	if len(active) == 0 {
		return slot, nil
	}
	// List is queue-ordered now (priority first), so pick the newest active row
	// explicitly: supersede must always replace the most recent attempt.
	newest := active[0]
	for _, row := range active[1:] {
		if row.CreatedAt.After(newest.CreatedAt) {
			newest = row
		}
	}
	var in pgTTSInput
	if err := json.Unmarshal(newest.Input, &in); err == nil && pgQueueIntentMatches(in, want) {
		// Duplicate active rows can only come from older bugs or a racing
		// request; keep the newest matching row and fence the rest out.
		for _, extra := range active {
			if extra.ID == newest.ID {
				continue
			}
			if _, cerr := s.deps.Jobs.Cancel(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: extra.ID}); cerr != nil &&
				!errors.Is(cerr, postgres.ErrJobState) && !errors.Is(cerr, postgres.ErrJobNotFound) {
				return slot, pgErr(cerr)
			}
		}
		if newest.State == postgres.JobPaused {
			if _, rerr := s.resumeJob(ctx, postgres.UUIDString(newest.ID)); rerr != nil {
				return slot, rerr
			}
		}
		slot.reuse = postgres.UUIDString(newest.ID)
		return slot, nil
	}
	slot.replace = newest.ID
	return slot, nil
}

// enqueueTTSRow resolves the chapter slot and either reuses it or hands it to
// the repository's one-transaction enqueue, then wakes the preparation runner.
func (s *PGService) enqueueTTSRow(ctx context.Context, scope postgres.ScopeRef, want pgQueueIntent, input pgTTSInput, total int32) (string, error) {
	slot, err := s.chapterQueueSlot(ctx, scope, want)
	if err != nil {
		return "", err
	}
	if slot.reuse != "" {
		return slot.reuse, nil
	}
	chapterID, err := postgres.ParseUUID(want.ChapterID)
	if err != nil {
		return "", pgErr(err)
	}
	job, err := s.deps.Jobs.EnqueueChapter(ctx, scope,
		fmt.Sprintf("tts-v%d-c%d-a%d", want.Volume, want.Chapter, slot.attempts+1),
		input, &total, chapterID, slot.replace)
	if err != nil {
		return "", pgErr(err)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	s.wakePreparation()
	return postgres.UUIDString(job.ID), nil
}

func (s *PGService) enqueueTTS(ctx context.Context, scope postgres.ScopeRef, chapter postgres.ChapterRef, script *Script, kind string, segs []pgTTSSegment, quality string, updateEta float64) (string, error) {
	revision, err := s.deps.Audio.PlaybackRevision(ctx, scope, chapter.ID, PlaybackProof{})
	if err != nil {
		return "", pgErr(err)
	}
	input := pgTTSInput{
		Kind: kind, Chapter: script.Chapter, Volume: script.Volume,
		ChapterID:      postgres.UUIDString(chapter.ID),
		ScriptRevision: script.pgRevision,
		SourceRevision: int32(chapter.CurrentRevision),
		Ownership:      revision.Ownership,
		Redo:           kind == JobKindRedo,
		Quality:        quality,
		UpdateEtaSec:   updateEta,
		Segments:       segs,
	}
	want := pgQueueIntent{
		ChapterID: input.ChapterID, Volume: input.Volume, Chapter: input.Chapter,
		Kind: kind, Quality: quality, SourceRevision: input.SourceRevision,
		ScriptRevision: input.ScriptRevision, Redo: input.Redo,
		Ownership: revision.Ownership, Segments: segs,
	}
	return s.enqueueTTSRow(ctx, scope, want, input, int32(len(segs)))
}

// enqueueTTSPrep queues the same tts kind as a preparation row: no segments,
// no ownership snapshot yet, needsScript=true. The web process composes and
// locks the bản đọc first, then CompletePreparationWith turns this row into the
// render job. The remote worker never sees a needsScript row.
func (s *PGService) enqueueTTSPrep(ctx context.Context, scope postgres.ScopeRef, volume int, chapter postgres.ChapterRef, script *Script, kind, quality string) (string, error) {
	scriptRevision := int32(0)
	if script != nil {
		scriptRevision = script.pgRevision
	}
	input := pgTTSInput{
		Kind: kind, Chapter: chapter.Number, Volume: volume,
		ChapterID:      postgres.UUIDString(chapter.ID),
		ScriptRevision: scriptRevision,
		SourceRevision: int32(chapter.CurrentRevision),
		Redo:           kind == JobKindRedo,
		Quality:        quality,
		Segments:       []pgTTSSegment{},
		NeedsScript:    true,
	}
	want := pgQueueIntent{
		ChapterID: input.ChapterID, Volume: volume, Chapter: chapter.Number,
		Kind: kind, Quality: quality, SourceRevision: input.SourceRevision,
		ScriptRevision: scriptRevision, Redo: input.Redo,
		NeedsScript: true, Segments: input.Segments,
	}
	return s.enqueueTTSRow(ctx, scope, want, input, 0)
}

// wantSegments computes which segments a job must synthesize under the
// current parameters: redo → all; create/update → every segment without a
// selected render keyed to the wanted parameters.
func (s *PGService) wantSegments(script *Script, cast *Cast, voiceMap map[string]ProjectVoice, settings Settings, redo bool) []pgTTSSegment {
	engine, model := s.engineIdent()
	selectAll := true
	out := make([]pgTTSSegment, 0, len(script.Segments))
	for _, seg := range script.Segments {
		key := pgSegmentKey(seg, cast, voiceMap, settings, engine, model)
		if !redo && seg.Rendered != nil && seg.Rendered.Key == key.Hash() && seg.Rendered.DurationSec > 0 {
			continue
		}
		voiceID, pitch := pgSegmentVoice(seg, cast, voiceMap)
		sel := &selectAll
		out = append(out, pgTTSSegment{
			ID: seg.ID, Text: seg.Text, Speaker: seg.Speaker,
			Voice: voiceID, Tone: seg.Tone, Quality: settings.Quality,
			Speed: 1, Pitch: pitch, Engine: engine, Model: model,
			SynthKey: key.Hash(), Select: sel,
		})
	}
	return out
}

// Enqueue queues one locked chapter (chapter render action).
func (s *PGService) Enqueue(ctx context.Context, volume, chapter int, currentHash string) (string, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return "", pgErr(err)
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return "", err
	}
	script, _, err := s.loadScriptRef(ctx, scope, ref)
	if err != nil {
		return "", err
	}
	if script == nil || len(script.Segments) == 0 {
		return "", conflict("Chương %d chưa có bản đọc để tạo giọng.", chapter)
	}
	if script.Status != StatusLocked {
		return "", conflict("Chương %d chưa chốt — hãy chốt bản đọc trước khi tạo giọng.", chapter)
	}
	kind := JobKindCreate
	if cc, sc, _ := script.freshness(currentHash); cc || sc {
		kind = JobKindUpdate
	}
	settings := s.Settings()
	if script.needsComposition(currentHash) {
		// The chapter source changed after this bản đọc was composed: the
		// worker would synthesize the old text and its ACK would be refused.
		// The web runner composes + locks first on the same row.
		return s.enqueueTTSPrep(ctx, scope, volume, ref, script, kind, settings.Quality)
	}
	cast, err := s.castForRender(ctx)
	if err != nil {
		return "", err
	}
	voices, err := s.Voices(ctx)
	if err != nil {
		return "", err
	}
	segs := s.wantSegments(script, cast, VoiceMap(voices), settings, false)
	if len(segs) == 0 {
		return "", fmt.Errorf("chương %d: %w", chapter, errChapterDone)
	}
	return s.enqueueTTS(ctx, scope, ref, script, kind, segs, settings.Quality, 0)
}

// QueueChapters validates the entire selection before locking or enqueuing it.
// Chapters without a script cannot be queued on hosted: the remote worker
// only synthesizes, composition needs the story model (unsupported here).
func (s *PGService) QueueChapters(ctx context.Context, source []SourceChapter, volume int, in QueueInput) (QueueResult, error) {
	out := QueueResult{Chapters: []int{}, Warnings: []QueueWarning{}, ETAReason: "Chưa đo tốc độ tạo ở chất lượng này."}
	scope, err := s.resolve(ctx)
	if err != nil {
		return out, pgErr(err)
	}
	selection := make([]int, 0, len(in.Chapters)+len(in.Redo))
	redo := map[int]bool{}
	for _, n := range in.Redo {
		redo[n] = true
	}
	seen := map[int]bool{}
	for _, n := range in.Chapters {
		if redo[n] {
			return out, conflict("Chương %d không vừa cập nhật vừa tạo lại từ đầu được.", n)
		}
		if !seen[n] {
			seen[n] = true
			selection = append(selection, n)
		}
	}
	for _, n := range in.Redo {
		if !seen[n] {
			seen[n] = true
			selection = append(selection, n)
		}
	}
	if len(selection) == 0 {
		return out, conflict("Chọn ít nhất một chương để tạo giọng.")
	}
	type picked struct {
		n      int
		script *Script
		ref    postgres.ChapterRef
	}
	picks := make([]picked, 0, len(selection))
	for _, n := range selection {
		var sc *SourceChapter
		for i := range source {
			if source[i].Chapter == n {
				sc = &source[i]
				break
			}
		}
		if sc == nil {
			return out, conflict("Chương %d không thuộc tập này.", n)
		}
		ref, err := s.chapterRef(ctx, scope, n)
		if err != nil {
			return out, err
		}
		script, _, err := s.loadScriptRef(ctx, scope, ref)
		if err != nil {
			return out, err
		}
		if script == nil || len(script.Segments) == 0 {
			if redo[n] {
				return out, conflict("Chương %d chưa có bản đọc để tạo lại từ đầu.", n)
			}
			if !sc.Completed || strings.TrimSpace(sc.Text) == "" {
				return out, conflict("Chương %d chưa nghiệm thu hoặc chưa có chính văn để soạn bản đọc.", n)
			}
			// GenPickEmpty: an accepted chapter without a bản đọc still enters
			// the queue. Locking is the caller's explicit "AI sẽ soạn bản đọc
			// rồi chốt" consent, exactly like the file backend.
			if !in.Lock {
				return out, conflict("Chương %d chưa có bản đọc — hãy soạn bản đọc trước khi tạo giọng.", n)
			}
			picks = append(picks, picked{n: n, script: nil, ref: ref})
			continue
		}
		if script.Status != StatusLocked && !in.Lock {
			return out, conflict("Chương %d chưa chốt bản đọc.", n)
		}
		for _, seg := range script.Segments {
			if len(seg.Flags) > 0 {
				out.Warnings = append(out.Warnings, QueueWarning{n, "Bản đọc còn chỗ cần xem; bạn có thể sửa trên máy tính rồi tạo lại."})
				break
			}
		}
		picks = append(picks, picked{n: n, script: script, ref: ref})
	}
	if in.VoiceMode != "" {
		cast, err := s.castForRender(ctx)
		if err != nil {
			return out, err
		}
		cast.VoiceMode = in.VoiceMode
		cast.Mode = ModeCast
		if in.VoiceMode == ModeSingle {
			cast.Mode = ModeSingle
		}
		if _, err = s.saveCast(ctx, cast); err != nil {
			return out, err
		}
	}
	cast, err := s.castForRender(ctx)
	if err != nil {
		return out, err
	}
	voices, err := s.Voices(ctx)
	if err != nil {
		return out, err
	}
	voiceMap := VoiceMap(voices)
	settings := s.Settings()
	if in.Quality != "" && !ValidateQuality(in.Quality) {
		return out, conflict("quality phải là fast, standard hoặc high")
	}
	if in.Quality != "" {
		settings.Quality = in.Quality
	}
	for _, p := range picks {
		script := p.script
		if script != nil && script.Status != StatusLocked {
			now := time.Now()
			script.Status = StatusLocked
			script.LockedAt = &now
			if err := s.saveScriptPayload(ctx, scope, script); err != nil {
				return out, err
			}
		}
		sourceHash := SourceHash(sourceChapterText(source, p.n))
		stale := script.needsComposition(sourceHash)
		kind := JobKindCreate
		if redo[p.n] {
			kind = JobKindRedo
		} else {
			done, _ := script.AudioStats()
			_, scriptChanged, _ := script.freshness("")
			if done > 0 && (stale || scriptChanged) {
				kind = JobKindUpdate
			}
		}
		if stale {
			// The web runner composes this chapter's bản đọc from the accepted
			// revision, locks it, and swaps the same row into a render job.
			if _, err := s.enqueueTTSPrep(ctx, scope, volume, p.ref, script, kind, settings.Quality); err != nil {
				return out, err
			}
			out.Chapters = append(out.Chapters, p.n)
			continue
		}
		segs := s.wantSegments(script, cast, voiceMap, settings, kind == JobKindRedo)
		if len(segs) == 0 {
			continue
		}
		if _, err := s.enqueueTTS(ctx, scope, p.ref, script, kind, segs, settings.Quality, 0); err != nil {
			return out, err
		}
		out.Chapters = append(out.Chapters, p.n)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	return out, nil
}

// jobByID resolves a durable job row of this story.
func (s *PGService) jobByID(ctx context.Context, jobID string) (postgres.ScopeRef, postgres.Job, error) {
	scope, err := s.resolve(ctx)
	var zero postgres.Job
	if err != nil {
		return scope, zero, pgErr(err)
	}
	id, err := postgres.ParseUUID(jobID)
	if err != nil {
		return scope, zero, ErrNotFound
	}
	row, err := s.deps.Jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: id})
	if err != nil {
		return scope, zero, pgErr(err)
	}
	if row.StoryID != scope.StoryID {
		return scope, zero, ErrNotFound
	}
	return scope, row, nil
}

// Pause suspends a queued or running job (the fence bumps, so an in-flight
// worker's publish is refused).
func (s *PGService) Pause(jobID string) error {
	scope, row, err := s.jobByID(context.Background(), jobID)
	if err != nil {
		return err
	}
	if row.State != postgres.JobQueued && row.State != postgres.JobRunning && row.State != postgres.JobWaiting {
		return conflict("Chỉ tạm dừng được việc đang chờ hoặc đang tạo.")
	}
	if _, err := s.deps.Jobs.Pause(context.Background(), postgres.JobKey{ProjectID: scope.ProjectID, ID: row.ID}); err != nil {
		return pgErr(err)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	return nil
}

func (s *PGService) resumeJob(ctx context.Context, jobID string) (postgres.Job, error) {
	scope, row, err := s.jobByID(ctx, jobID)
	if err != nil {
		return row, err
	}
	return s.deps.Jobs.Resume(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: row.ID})
}

// Resume continues a paused (or failed) job.
func (s *PGService) Resume(jobID string) error {
	ctx := context.Background()
	scope, row, err := s.jobByID(ctx, jobID)
	if err != nil {
		return err
	}
	if row.State != postgres.JobPaused && row.State != postgres.JobFailed {
		return conflict("Chỉ tiếp tục được việc đang tạm dừng hoặc bị lỗi.")
	}
	key := postgres.JobKey{ProjectID: scope.ProjectID, ID: row.ID}
	if row.State == postgres.JobFailed {
		_, err = s.deps.Jobs.Retry(ctx, key, nil)
	} else {
		_, err = s.deps.Jobs.Resume(ctx, key)
	}
	if err != nil {
		return pgErr(err)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	s.wakePreparation()
	return nil
}

// Cancel stops a job (rendered segments stay).
func (s *PGService) Cancel(jobID string) error {
	ctx := context.Background()
	scope, row, err := s.jobByID(ctx, jobID)
	if err != nil {
		return err
	}
	if row.State == postgres.JobCompleted {
		return conflict("Việc này đã xong.")
	}
	if _, err := s.deps.Jobs.Cancel(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: row.ID}); err != nil {
		return pgErr(err)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	return nil
}

// Retry requeues a failed job; the fence bump invalidates any stale worker.
func (s *PGService) Retry(ctx context.Context, jobID, quality string) error {
	scope, row, err := s.jobByID(ctx, jobID)
	if err != nil {
		return err
	}
	if row.State != postgres.JobFailed && row.State != postgres.JobCancelled {
		return conflict("Chỉ thử lại được việc bị lỗi.")
	}
	if _, err := s.deps.Jobs.Retry(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: row.ID}, nil); err != nil {
		return pgErr(err)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	s.wakePreparation()
	return nil
}

// Prioritize moves the chapter's queued tts job ahead of every other job in
// the story. The durable row keeps its lease/fence/created_at; only its queue
// priority changes, so the list reload and the native claim agree.
func (s *PGService) Prioritize(ctx context.Context, volume, chapter int) error {
	scope, err := s.resolve(ctx)
	if err != nil {
		return pgErr(err)
	}
	if _, err := s.deps.Jobs.PrioritizeChapter(ctx, scope, volume, chapter); err != nil {
		if errors.Is(err, postgres.ErrJobState) {
			return conflict("Chỉ làm trước được chương đang chờ.")
		}
		return pgErr(err)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	s.wakePreparation()
	return nil
}
