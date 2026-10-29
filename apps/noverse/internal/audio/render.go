package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 后台生成：锁定章节 → 队列 → 逐段合成（命中缓存就跳过）→ 生成整章播放文件。
// 每段都落盘，所以"暂停/取消/崩溃"都不会丢掉已经生成的部分。

// runJobs 顺序处理队列，直到没有排队任务（或服务关闭）。
func (s *Service) runJobs() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		job, ok := s.queue.NextQueued()
		if !ok {
			s.runPendingExports()
			return
		}
		s.runJob(job)
	}
}

// runJob 生成一章的全部段落。
func (s *Service) runJob(job Job) {
	recastRoles := job.RerenderRoles
	script, err := ReadScript(s.layout, job.Volume, job.Chapter)
	if err != nil {
		s.failJob(job.ID, job.Volume, job.Chapter, 0, 0, err)
		return
	}
	if job.NeedsScript || s.queuedComposer != nil {
		previousHash := ""
		if script != nil {
			previousHash = script.SourceHash
		}
		script, err = s.composeQueuedScript(job)
		if err != nil {
			current, exists := s.queue.Find(job.ID)
			if s.ctx.Err() == nil && exists && current.State != JobCanceled {
				reason := fmt.Sprintf("Không soạn được bản đọc chương %d. Kiểm tra kết nối và mô hình của nhà cung cấp AI rồi thử lại.", job.Chapter)
				var state *StateError
				if errors.As(err, &state) {
					reason = state.Msg
				} else if errors.Is(err, ErrNoModel) {
					reason = ErrNoModel.Error()
				} else if errors.Is(err, context.DeadlineExceeded) {
					reason = "Nhà cung cấp AI phản hồi quá lâu khi soạn bản đọc. Hãy thử lại."
				}
				s.failJob(job.ID, job.Volume, job.Chapter, 0, 0, conflict("%s", reason))
			}
			return
		}
		// A changed chapter must refresh every changed line, not only the recast
		// role. Content-addressed segment keys still reuse every unchanged line.
		if script.SourceHash != previousHash {
			job.RerenderRoles = nil
		}
	}
	if _, changed, _ := script.freshness(""); changed {
		job.RerenderRoles = nil
	}
	if script == nil || len(script.Segments) == 0 {
		s.failJob(job.ID, job.Volume, job.Chapter, 0, 0, fmt.Errorf("Chương %d chưa có bản đọc.", job.Chapter))
		return
	}
	if script.Status != StatusLocked {
		s.failJob(job.ID, job.Volume, job.Chapter, 0, 0, fmt.Errorf("Chương %d chưa chốt bản đọc.", job.Chapter))
		return
	}
	settings := s.Settings()
	if job.Quality != "" {
		settings.Quality = job.Quality
	}
	cast, err := s.castForRender()
	if err != nil {
		s.failJob(job.ID, job.Volume, job.Chapter, 0, 0, err)
		return
	}
	// Bản đọc soạn trước khi có quy tắc gộp dấu câu có thể còn đoạn chỉ có "," —
	// gộp lại rồi ghi xuống cùng lần ghi cờ no_voice bên dưới.
	if err := foldScriptUnspeakable(script); err != nil {
		s.failJob(job.ID, job.Volume, job.Chapter, 0, 0, err)
		return
	}
	// Ghi lại cờ no_voice theo phân vai lúc tạo giọng để file bản đọc không giữ cờ cũ.
	applyVoiceFlags(script, cast)
	if err := WriteScript(s.layout, script); err != nil {
		s.failJob(job.ID, job.Volume, job.Chapter, 0, 0, err)
		return
	}
	voices, err := s.Voices(s.ctx)
	if err != nil {
		s.failJob(job.ID, job.Volume, job.Chapter, 0, 0, err)
		return
	}
	voiceMap := VoiceMap(voices)
	engine, model := s.engineIdent(s.ctx)
	st := s.synth.Status(s.ctx)
	if !synthUsable(st) {
		msg := st.Error
		if msg == "" {
			msg = ErrEngineMissing.Error()
		}
		s.failJob(job.ID, job.Volume, job.Chapter, 0, len(script.Segments), errors.New(msg))
		return
	}
	if !FFmpegAvailable() {
		s.failJob(job.ID, job.Volume, job.Chapter, 0, len(script.Segments), ErrFFmpegMissing)
		return
	}

	// §AK：先给每段算"按当前参数该有的缓存键"和"要不要真正读"。
	// update/create 只重读键对不上、还没有音频或缓存文件缺失/坏掉的段；
	// redo（tạo lại từ đầu）无视缓存逐段重读；role re-read 只动指定说话人。
	roleMode := len(job.RerenderRoles) > 0
	redo := job.Kind == JobKindRedo
	keys := make([]SynthKey, len(script.Segments))
	needs := make([]bool, len(script.Segments))
	total := 0
	for i := range script.Segments {
		seg := script.Segments[i]
		// Đổi giọng một vai phải ghi đè "giọng chọn riêng" đã nhận trước đó:
		// bỏ cờ Rerendered TRƯỚC khi tính khoá, nếu không khoá vẫn mang giọng cũ.
		if roleMode && seg.Rendered != nil && seg.Rendered.Rerendered {
			for _, role := range recastRoles {
				if role == seg.Speaker || role == "single" {
					seg.Rendered.Rerendered = false
					script.Segments[i] = seg
					break
				}
			}
		}
		keys[i] = s.segmentKey(seg, cast, voiceMap, settings, engine, model)
		keyHash := keys[i].Hash()
		switch {
		case roleMode:
			for _, role := range job.RerenderRoles {
				if seg.Rendered != nil && (role == seg.Speaker || role == "single") {
					needs[i] = true
					break
				}
			}
		case redo:
			needs[i] = true
		default:
			needs[i] = seg.Rendered == nil || seg.Rendered.Key != keyHash || seg.Rendered.DurationSec <= 0
			if !needs[i] {
				// 键对上了但缓存文件丢了或是 0 秒坏档：这一段仍算"要读"。
				wav, rerr := ReadSegmentAudio(s.layout, job.Volume, job.Chapter, keyHash)
				if rerr != nil {
					s.failJob(job.ID, job.Volume, job.Chapter, i, len(script.Segments), rerr)
					return
				}
				if wav == nil || WAVDurationSec(wav) <= 0 {
					needs[i] = true
				}
			}
		}
		if needs[i] {
			total++
		}
	}
	s.queue.Update(job.ID, func(j *Job) {
		j.State = JobRendering
		j.Step = "render"
		j.NeedsScript = false
		j.Segments = total
		j.Error = ""
		j.ErrorAt = nil
	})
	s.emitQueue()

	started := time.Now()
	audioSec := 0.0
	processed := 0
	allAudio := true
	for i := range script.Segments {
		if s.waitJob(job.ID) == JobCanceled {
			return
		}
		seg := script.Segments[i]
		// A role re-read preserves other roles, including voices changed earlier
		// with rerenderOld disabled.
		if roleMode && !needs[i] {
			if seg.Rendered == nil {
				allAudio = false
			} else if wav, err := ReadSegmentAudio(s.layout, job.Volume, job.Chapter, seg.Rendered.Key); err != nil || wav == nil {
				allAudio = false
			}
			continue
		}
		if !needs[i] {
			// 未变的段（§AK "giữ đoạn không đổi"）：缓存键已核实有效，
			// 只把时长记进整章统计，不算进度、不重读。
			audioSec += seg.Rendered.DurationSec
			continue
		}
		processed++
		key := keys[i]
		keyHash := key.Hash()
		now := time.Now()
		// 缓存命中：补齐元信息即可（"nghe thử" 与后台生成共用同一份缓存）。
		// 0 秒缓存是坏结果：留着它，"tạo lại"会永远命中同一份空音频，必须删掉重读。
		// redo 明确"đọc lại từ đầu"，不看缓存。
		var wav []byte
		var rerr error
		if !redo {
			wav, rerr = ReadSegmentAudio(s.layout, job.Volume, job.Chapter, keyHash)
			if rerr == nil && wav != nil && WAVDurationSec(wav) <= 0 {
				if err := removeSegmentAudio(s.layout, job.Volume, job.Chapter, keyHash); err != nil {
					s.failJob(job.ID, job.Volume, job.Chapter, i, total, err)
					return
				}
				wav = nil
			}
		}
		if rerr == nil && wav != nil {
			if seg.Rendered == nil || seg.Rendered.Key != keyHash || seg.Rendered.DurationSec <= 0 || seg.Rendered.Quality != key.Quality {
				seg.Rendered = &SegmentAudio{
					Key: keyHash, VoiceID: key.VoiceID, Tone: key.Tone, Speed: key.Speed, Pitch: key.Pitch,
					DurationSec: WAVDurationSec(wav), Engine: engine, Model: model, Quality: key.Quality, RenderedAt: &now,
					Rerendered: seg.Rendered != nil && seg.Rendered.Rerendered,
				}
				script.Segments[i] = seg
				if err := WriteScript(s.layout, script); err != nil {
					s.failJob(job.ID, job.Volume, job.Chapter, i, total, err)
					return
				}
			}
			audioSec += seg.Rendered.DurationSec
			s.progress(job.ID, job.Kind, script, processed, total, seg, started, audioSec, "ready")
			continue
		}
		ref := ResolveVoice(cast, seg, voiceMap)
		if !ref.OK {
			s.failJob(job.ID, job.Volume, job.Chapter, i, total,
				conflict("Chưa chọn giọng cho %q. Mở Phân vai để chọn giọng.", speakerLabel(cast, seg.Speaker)))
			return
		}
		s.progress(job.ID, job.Kind, script, processed-1, total, seg, started, audioSec, "rendering")
		audio, serr := s.synth.Synthesize(s.ctx, SynthRequest{
			Text: key.Text, VoiceID: key.VoiceID, RefAudio: ref.RefAudio,
			Tone: key.Tone, Quality: settings.Quality, Speed: 1, Pitch: key.Pitch,
		})
		if serr != nil {
			s.failJob(job.ID, job.Volume, job.Chapter, i, total, serr)
			return
		}
		wav = audio.WAV
		if key.Speed != 1 || key.Pitch != 1 {
			adjusted, aerr := adjustAudio(s.ctx, wav, key.Speed, key.Pitch)
			if aerr != nil {
				s.failJob(job.ID, job.Volume, job.Chapter, i, total, aerr)
				return
			}
			wav = adjusted
		}
		dur := WAVDurationSec(wav)
		if dur == 0 {
			dur = audio.DurationSec
		}
		if dur <= 0 {
			s.failJob(job.ID, job.Volume, job.Chapter, i, total,
				conflict("Bộ đọc trả về âm thanh rỗng cho đoạn %s (%q). Sửa chữ của đoạn này trong bản đọc rồi tạo lại.", seg.ID, seg.Text))
			return
		}
		if redo {
			// WriteSegmentAudio 对已有文件视为命中不覆盖；tạo lại 要先删掉旧文件。
			if err := removeSegmentAudio(s.layout, job.Volume, job.Chapter, keyHash); err != nil {
				s.failJob(job.ID, job.Volume, job.Chapter, i, total, err)
				return
			}
		}
		if err := WriteSegmentAudio(s.layout, job.Volume, job.Chapter, keyHash, wav); err != nil {
			s.failJob(job.ID, job.Volume, job.Chapter, i, total, err)
			return
		}
		seg.Rendered = &SegmentAudio{
			Key: keyHash, VoiceID: key.VoiceID, Tone: key.Tone, Speed: key.Speed, Pitch: key.Pitch,
			DurationSec: dur, Engine: engine, Model: model, Quality: key.Quality, RenderedAt: &now,
		}
		script.Segments[i] = seg
		if err := WriteScript(s.layout, script); err != nil {
			s.failJob(job.ID, job.Volume, job.Chapter, i, total, err)
			return
		}
		audioSec += dur
		s.progress(job.ID, job.Kind, script, processed, total, seg, started, audioSec, "rendering")
	}

	// 全部段都有音频：顺手生成整章播放文件（监听页直接用）。
	if allAudio {
		script.RenderedFrom = &RenderSource{ScriptHash: script.ScriptHash(), CastHash: castHash(cast), Quality: settings.Quality}
		if err := WriteScript(s.layout, script); err != nil {
			s.failJob(job.ID, job.Volume, job.Chapter, total, total, err)
			return
		}
		if _, err := s.ensureChapterAudio(s.ctx, job.Volume, job.Chapter, script); err != nil {
			s.failJob(job.ID, job.Volume, job.Chapter, total, total, err)
			return
		}
	}
	done := time.Now()
	s.queue.Update(job.ID, func(j *Job) {
		j.State = JobDone
		j.Percent = 100
		j.ETASec = 0
		j.SegmentsDone = total
		j.Segments = total
		j.CurrentText = ""
		j.SegmentID = ""
		j.UpdatedAt = done
	})
	s.hub.Publish(NewEvent(EventRender, map[string]any{
		"jobId": job.ID, "chapter": job.Chapter, "segment": total, "segments": total, "kind": job.Kind,
		"percent": float64(100), "etaSec": float64(0), "state": "done",
	}))
	s.emitQueue()
	s.runPendingExports()
}

// waitJob 在任务被暂停时原地等待；返回最新状态（取消时返回 JobCanceled）。
func (s *Service) waitJob(jobID string) string {
	for {
		job, ok := s.queue.Find(jobID)
		if !ok {
			return JobCanceled
		}
		switch job.State {
		case JobPaused:
			select {
			case <-s.ctx.Done():
				return JobCanceled
			case <-time.After(250 * time.Millisecond):
			}
		case JobCanceled:
			return JobCanceled
		default:
			return job.State
		}
	}
}

// progress 更新任务并广播 audio.render（percent 以段数为准，eta 以实测 RTF 为准；
// kind 透传给界面区分 "Đang tạo" / "Đang cập nhật"）。
func (s *Service) progress(jobID, kind string, script *Script, done, total int, seg Segment, started time.Time, audioSec float64, state string) {
	percent := 0.0
	if total > 0 {
		percent = float64(done) / float64(total) * 100
	}
	// RTF = 已生成音频秒数 / 已用墙钟秒数；剩余段数按平均段长估时。
	eta := 0.0
	if elapsed := time.Since(started).Seconds(); elapsed > 0.5 && done > 0 && audioSec > 0 {
		rtf := audioSec / elapsed
		remainAudio := audioSec / float64(done) * float64(total-done)
		if rtf > 0 {
			eta = remainAudio / rtf
		}
	}
	speaker := seg.Speaker
	s.queue.Update(jobID, func(j *Job) {
		j.SegmentsDone = done
		j.Segments = total
		j.Percent = percent
		j.ETASec = eta
		j.Speaker = speaker
		j.SegmentID = seg.ID
		j.CurrentText = seg.Text
	})
	s.hub.Publish(NewEvent(EventRender, map[string]any{
		"jobId": jobID, "chapter": script.Chapter, "segment": done, "segments": total, "kind": kind,
		"segmentId": seg.ID, "speaker": speaker, "percent": percent, "etaSec": eta, "state": state,
	}))
}

// failJob 把任务标成错误（保留已生成的段），并广播带原因的 audio.render。
func (s *Service) failJob(jobID string, volume, chapter, segment, segments int, err error) {
	if s.ctx.Err() != nil {
		return
	} // keep interrupted rendering resumable on restart
	now := time.Now()
	reason := renderFailureReason(err)
	kind := JobKindCreate
	s.queue.Update(jobID, func(j *Job) {
		j.State = JobError
		j.Error = reason
		j.ErrorAt = &now
		j.CurrentText = ""
		j.SegmentID = ""
		kind = j.Kind
	})
	s.hub.Publish(NewEvent(EventRender, map[string]any{
		"jobId": jobID, "chapter": chapter, "segment": segment, "segments": segments, "kind": kind,
		"percent": float64(0), "etaSec": float64(0), "state": "error", "error": reason,
	}))
	s.emitQueue()
}

func (s *Service) emitQueue() {
	jobs := s.queue.Snapshot()
	sortJobs(jobs)
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": jobs}))
}

// speakerLabel 把说话人 id 换成界面上的名字。
func speakerLabel(cast *Cast, id string) string {
	if cast != nil {
		if sp, ok := cast.SpeakerByID(id); ok {
			return sp.Name
		}
	}
	if id == "" || id == NarratorID {
		return "Dẫn truyện"
	}
	return id
}

// ── 单段：nghe thử / đọc lại ──

// PreviewResult 对应 POST S/audio/segments/:id/preview。
type PreviewResult struct {
	URL         string  `json:"url"`
	DurationSec float64 `json:"durationSec"`
	Cached      bool    `json:"cached"`
}

// findSegment 在章节里找一段。
func findSegment(script *Script, segID string) (int, Segment, error) {
	for i, seg := range script.Segments {
		if seg.ID == segID {
			return i, seg, nil
		}
	}
	return 0, Segment{}, ErrNotFound
}

// Preview 生成（或命中）一段音频并保留。第一次点击就是"nghe thử"，之后一直复用。
func (s *Service) Preview(ctx context.Context, volume, chapter int, segID string) (PreviewResult, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return PreviewResult{}, err
	}
	if script == nil {
		return PreviewResult{}, ErrNotFound
	}
	idx, seg, err := findSegment(script, segID)
	if err != nil {
		return PreviewResult{}, err
	}
	settings := s.Settings()
	cast, err := s.castForRender()
	if err != nil {
		return PreviewResult{}, err
	}
	voices, err := s.Voices(ctx)
	if err != nil {
		return PreviewResult{}, err
	}
	voiceMap := VoiceMap(voices)
	engine, model := s.engineIdent(ctx)
	key := s.segmentKey(seg, cast, voiceMap, settings, engine, model)
	keyHash := key.Hash()
	url := s.segmentURL(volume, chapter, seg.ID)
	if wav, rerr := ReadSegmentAudio(s.layout, volume, chapter, keyHash); rerr == nil && wav != nil {
		if dur := WAVDurationSec(wav); dur > 0 {
			return PreviewResult{URL: url, DurationSec: dur, Cached: true}, nil
		}
		if err := removeSegmentAudio(s.layout, volume, chapter, keyHash); err != nil {
			return PreviewResult{}, err
		}
	}
	if st := s.synth.Status(ctx); !synthUsable(st) {
		return PreviewResult{}, ErrEngineMissing
	}
	if !FFmpegAvailable() {
		return PreviewResult{}, ErrFFmpegMissing
	}
	ref := ResolveVoice(cast, seg, voiceMap)
	if !ref.OK {
		return PreviewResult{}, conflict("Chưa chọn giọng cho %q. Mở Phân vai để chọn giọng.", speakerLabel(cast, seg.Speaker))
	}
	audio, err := s.synth.Synthesize(ctx, SynthRequest{
		Text: key.Text, VoiceID: key.VoiceID, RefAudio: ref.RefAudio,
		Tone: key.Tone, Quality: settings.Quality, Speed: 1, Pitch: key.Pitch,
	})
	if err != nil {
		return PreviewResult{}, err
	}
	wav := audio.WAV
	if key.Speed != 1 || key.Pitch != 1 {
		adjusted, aerr := adjustAudio(ctx, wav, key.Speed, key.Pitch)
		if aerr != nil {
			return PreviewResult{}, aerr
		}
		wav = adjusted
	}
	dur := WAVDurationSec(wav)
	if dur == 0 {
		dur = audio.DurationSec
	}
	if dur <= 0 {
		return PreviewResult{}, conflict("Bộ đọc trả về âm thanh rỗng cho đoạn %s (%q). Sửa chữ của đoạn này rồi nghe thử lại.", seg.ID, seg.Text)
	}
	if err := WriteSegmentAudio(s.layout, volume, chapter, keyHash, wav); err != nil {
		return PreviewResult{}, err
	}
	now := time.Now()
	script.Segments[idx].Rendered = &SegmentAudio{
		Key: keyHash, VoiceID: key.VoiceID, Tone: key.Tone, Speed: key.Speed, Pitch: key.Pitch,
		DurationSec: dur, Engine: engine, Model: model, Quality: key.Quality, RenderedAt: &now,
	}
	if err := WriteScript(s.layout, script); err != nil {
		return PreviewResult{}, err
	}
	return PreviewResult{URL: url, DurationSec: dur}, nil
}

// RerenderOptions 是"đọc lại đoạn này"的临时参数（A5 对比 Bản cũ / Bản mới）。
type RerenderOptions struct {
	VoiceID string  `json:"voiceId"`
	Speed   float64 `json:"speed"`
	Tone    string  `json:"tone"`
}

// Candidate 是一次重读的候选结果（未采纳前不影响现有音频）。
type Candidate struct {
	ID          string  `json:"candidateId"`
	URL         string  `json:"url"`
	DurationSec float64 `json:"durationSec"`
}

// candidateMeta 是候选音频的落盘元信息。
type candidateMeta struct {
	ID          string    `json:"id"`
	Volume      int       `json:"volume"`
	Chapter     int       `json:"chapter"`
	SegmentID   string    `json:"segmentId"`
	Key         string    `json:"key"`
	VoiceID     string    `json:"voiceId"`
	Tone        string    `json:"tone"`
	Speed       float64   `json:"speed"`
	Pitch       float64   `json:"pitch"`
	Engine      string    `json:"engine"`
	Model       string    `json:"model"`
	Quality     string    `json:"quality,omitempty"`
	DurationSec float64   `json:"durationSec"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Rerender 用新参数重读一段，产出候选（A5 里并排试听，选一个）。
func (s *Service) Rerender(ctx context.Context, volume, chapter int, segID string, opts RerenderOptions) (Candidate, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return Candidate{}, err
	}
	if script == nil {
		return Candidate{}, ErrNotFound
	}
	_, seg, err := findSegment(script, segID)
	if err != nil {
		return Candidate{}, err
	}
	settings := s.Settings()
	cast, err := s.castForRender()
	if err != nil {
		return Candidate{}, err
	}
	voices, err := s.Voices(ctx)
	if err != nil {
		return Candidate{}, err
	}
	voiceMap := VoiceMap(voices)
	engine, model := s.engineIdent(ctx)
	ref := ResolveVoice(cast, seg, voiceMap)
	if opts.VoiceID != "" {
		ref.VoiceID = opts.VoiceID
		if v, ok := voiceMap[opts.VoiceID]; ok {
			ref.RefAudio = v.File
		}
		ref.OK = true
	}
	if !ref.OK {
		return Candidate{}, conflict("Chưa chọn giọng cho %q.", speakerLabel(cast, seg.Speaker))
	}
	if st := s.synth.Status(ctx); !synthUsable(st) {
		return Candidate{}, ErrEngineMissing
	}
	if !FFmpegAvailable() {
		return Candidate{}, ErrFFmpegMissing
	}
	tone := seg.Tone
	if opts.Tone != "" {
		tone = opts.Tone
	}
	speed := 1.0
	if opts.Speed > 0 {
		speed = opts.Speed
	}
	pitch := ref.Pitch
	key := SynthKey{Text: s.synthesisText(seg.Text, tone), VoiceID: ref.VoiceID, Tone: tone, Speed: speed, Pitch: pitch, Quality: settings.Quality, Engine: engine, Model: model}
	audio, err := s.synth.Synthesize(ctx, SynthRequest{
		Text: key.Text, VoiceID: key.VoiceID, RefAudio: ref.RefAudio,
		Tone: key.Tone, Quality: settings.Quality, Speed: 1, Pitch: key.Pitch,
	})
	if err != nil {
		return Candidate{}, err
	}
	wav := audio.WAV
	if key.Speed != 1 || key.Pitch != 1 {
		adjusted, aerr := adjustAudio(ctx, wav, key.Speed, key.Pitch)
		if aerr != nil {
			return Candidate{}, aerr
		}
		wav = adjusted
	}
	now := time.Now()
	id := fmt.Sprintf("%d-%s-%d", chapter, seg.ID, now.UnixNano())
	dir := s.layout.candidatesPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Candidate{}, err
	}
	if err := writeFileAtomic(filepath.Join(dir, id+".wav"), wav); err != nil {
		return Candidate{}, err
	}
	meta := candidateMeta{
		ID: id, Volume: volume, Chapter: chapter, SegmentID: seg.ID, Key: key.Hash(),
		VoiceID: key.VoiceID, Tone: key.Tone, Speed: key.Speed, Pitch: key.Pitch,
		Engine: engine, Model: model, Quality: key.Quality, DurationSec: WAVDurationSec(wav), CreatedAt: now,
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return Candidate{}, err
	}
	if err := writeFileAtomic(filepath.Join(dir, id+".json"), data); err != nil {
		return Candidate{}, err
	}
	return Candidate{ID: id, URL: s.base() + "/audio/candidates/" + id, DurationSec: meta.DurationSec}, nil
}

// CandidatePath 返回候选音频文件路径（handler 提供试听）。
func (s *Service) CandidatePath(id string) (string, error) {
	if strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return "", ErrNotFound
	}
	path := filepath.Join(s.layout.candidatesPath(), id+".wav")
	if _, err := os.Stat(path); err != nil {
		return "", ErrNotFound
	}
	return path, nil
}

// readCandidate 读候选元信息。
func (s *Service) readCandidate(id string) (candidateMeta, error) {
	if strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return candidateMeta{}, ErrNotFound
	}
	data, err := os.ReadFile(filepath.Join(s.layout.candidatesPath(), id+".json"))
	if err != nil {
		return candidateMeta{}, ErrNotFound
	}
	var meta candidateMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return candidateMeta{}, err
	}
	return meta, nil
}

// AcceptCandidate 采纳候选：写入缓存并把段的参数改成候选参数（下次生成沿用）。
func (s *Service) AcceptCandidate(volume, chapter int, segID, candidateID string) error {
	meta, err := s.readCandidate(candidateID)
	if err != nil {
		return err
	}
	if meta.Chapter != chapter || meta.SegmentID != segID {
		return conflict("Bản đọc lại này không thuộc đoạn đang sửa.")
	}
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return err
	}
	if script == nil {
		return ErrNotFound
	}
	idx, seg, err := findSegment(script, segID)
	if err != nil {
		return err
	}
	wav, err := os.ReadFile(filepath.Join(s.layout.candidatesPath(), candidateID+".wav"))
	if err != nil {
		return ErrNotFound
	}
	if err := WriteSegmentAudio(s.layout, volume, chapter, meta.Key, wav); err != nil {
		return err
	}
	now := time.Now()
	seg.Rendered = &SegmentAudio{
		Key: meta.Key, VoiceID: meta.VoiceID, Tone: meta.Tone, Speed: meta.Speed, Pitch: meta.Pitch,
		DurationSec: meta.DurationSec, Engine: meta.Engine, Model: meta.Model, Quality: meta.Quality,
		RenderedAt: &now, Rerendered: true,
	}
	script.Segments[idx] = seg
	if err := WriteScript(s.layout, script); err != nil {
		return err
	}
	s.dropCandidate(candidateID)
	return s.invalidateChapterAudio(volume, chapter)
}

// RejectCandidate 丢弃候选（保留现有音频）。
func (s *Service) RejectCandidate(candidateID string) error {
	if _, err := s.readCandidate(candidateID); err != nil {
		return err
	}
	s.dropCandidate(candidateID)
	return nil
}

func (s *Service) dropCandidate(id string) {
	dir := s.layout.candidatesPath()
	_ = os.Remove(filepath.Join(dir, id+".wav"))
	_ = os.Remove(filepath.Join(dir, id+".json"))
}

// SegmentAudioPath 返回某段当前缓存文件路径（handler 试听用）。
func (s *Service) SegmentAudioPath(ctx context.Context, volume, chapter int, segID string) (string, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return "", err
	}
	if script == nil {
		return "", ErrNotFound
	}
	_, seg, err := findSegment(script, segID)
	if err != nil {
		return "", err
	}
	if seg.Rendered == nil {
		return "", ErrNotFound
	}
	path := s.layout.CachePath(volume, chapter, seg.Rendered.Key)
	if _, err := os.Stat(path); err != nil {
		return "", ErrNotFound
	}
	return path, nil
}
