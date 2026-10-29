package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/noverse/internal/persistence/postgres"
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
	Redo           bool           `json:"redo"`
	Quality        string         `json:"quality"`
	UpdateEtaSec   float64        `json:"updateEtaSec,omitempty"`
	Segments       []pgTTSSegment `json:"segments"`
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

// findActiveJob returns the chapter's claimable/in-flight job if one exists.
func (s *PGService) findActiveJob(ctx context.Context, volume, chapter int) (*Job, error) {
	jobs, err := s.queueSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		j := &jobs[i]
		if j.Chapter != chapter || (volume > 0 && j.Volume != volume) {
			continue
		}
		if j.State == JobQueued || j.State == JobRendering || j.State == JobPaused {
			return j, nil
		}
	}
	return nil, nil
}

// chapterAttempts counts prior durable jobs for one chapter (any state) so the
// idempotency key names a unique attempt: reuse the active row, never collide
// with a finished one.
func (s *PGService) chapterAttempts(ctx context.Context, volume, chapter int) (active *Job, attempts int, err error) {
	jobs, err := s.queueSnapshot(ctx)
	if err != nil {
		return nil, 0, err
	}
	for i := range jobs {
		j := &jobs[i]
		if j.Chapter != chapter || (volume > 0 && j.Volume != volume) {
			continue
		}
		attempts++
		switch j.State {
		case JobQueued, JobRendering, JobPaused:
			active = j
		}
	}
	return active, attempts, nil
}

// enqueueTTS writes the durable job. If an active row exists for the chapter
// it is reused (a paused row resumes); a finished chapter starts a new
// attempt under a distinct idempotency key.
func (s *PGService) enqueueTTS(ctx context.Context, scope postgres.ScopeRef, chapter postgres.ChapterRef, script *Script, kind string, segs []pgTTSSegment, quality string, updateEta float64) (string, error) {
	active, attempts, err := s.chapterAttempts(ctx, script.Volume, script.Chapter)
	if err != nil {
		return "", err
	}
	if active != nil {
		if active.State == JobPaused {
			if _, err := s.resumeJob(ctx, active.ID); err != nil {
				return "", err
			}
		}
		return active.ID, nil
	}
	input := pgTTSInput{
		Kind: kind, Chapter: script.Chapter, Volume: script.Volume,
		ChapterID:      postgres.UUIDString(chapter.ID),
		ScriptRevision: script.pgRevision,
		SourceRevision: int32(chapter.CurrentRevision),
		Redo:           kind == JobKindRedo,
		Quality:        quality,
		UpdateEtaSec:   updateEta,
		Segments:       segs,
	}
	total := int32(len(segs))
	job, err := s.deps.Jobs.Enqueue(ctx, scope, postgres.JobTTS,
		fmt.Sprintf("tts-v%d-c%d-a%d", script.Volume, script.Chapter, attempts+1),
		input, &total, pgtype.UUID{})
	if err != nil {
		return "", pgErr(err)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	return postgres.UUIDString(job.ID), nil
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
	cast, err := s.castForRender(ctx)
	if err != nil {
		return "", err
	}
	voices, err := s.Voices(ctx)
	if err != nil {
		return "", err
	}
	settings := s.Settings()
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
			if !sc.Completed || sc.Text == "" {
				return out, conflict("Chương %d chưa nghiệm thu hoặc chưa có chính văn để soạn bản đọc.", n)
			}
			return out, conflict("Chương %d chưa có bản đọc — hãy soạn bản đọc trước khi tạo giọng.", n)
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
		if script.Status != StatusLocked {
			now := time.Now()
			script.Status = StatusLocked
			script.LockedAt = &now
			if err := s.saveScriptPayload(ctx, scope, script); err != nil {
				return out, err
			}
		}
		kind := JobKindCreate
		if redo[p.n] {
			kind = JobKindRedo
		} else {
			done, _ := script.AudioStats()
			_, scriptChanged, _ := script.freshness("")
			if done > 0 && (script.needsComposition(SourceHash(sourceChapterText(source, p.n))) || scriptChanged) {
				kind = JobKindUpdate
			}
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
	_, row, err := s.jobByID(ctx, jobID)
	if err != nil {
		return err
	}
	if row.State != postgres.JobPaused && row.State != postgres.JobFailed {
		return conflict("Chỉ tiếp tục được việc đang tạm dừng hoặc bị lỗi.")
	}
	if _, err := s.resumeJob(ctx, jobID); err != nil {
		return pgErr(err)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
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
	return nil
}

// Prioritize needs a queue order column the durable jobs table does not
// expose yet; it answers 501 instead of pretending to reorder.
func (s *PGService) Prioritize(ctx context.Context, volume, chapter int) error {
	return pgUnsupported("Sắp xếp lại hàng đợi")
}
