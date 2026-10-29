package audio

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// QueueComposer resolves current story text, cast and model when a waiting job
// starts, including after a restart. It must not call the provider itself.
type QueueComposer func(volume, chapter int) (SourceChapter, Composer, CharacterRef, error)

type QueueInput struct {
	Chapters []int `json:"chapters"`
	// Redo 是要"đọc lại từ đầu"的章节（§AK）：必须有已 chốt 的 bản đọc，
	// 生成时强制重读全部段；与 Chapters 不能重复。
	Redo      []int  `json:"redo,omitempty"`
	Quality   string `json:"quality"`
	VoiceMode string `json:"voiceMode"`
	Lock      bool   `json:"lock"`
}
type QueueWarning struct {
	Chapter int    `json:"chapter"`
	Reason  string `json:"reason"`
}
type QueueResult struct {
	Chapters  []int          `json:"chapters"`
	ETASec    float64        `json:"etaSec"`
	ETAReason string         `json:"etaReason,omitempty"`
	Warnings  []QueueWarning `json:"warnings"`
}

// QueueChapters validates the entire selection before locking or enqueuing it.
func (s *Service) QueueChapters(source []SourceChapter, volume int, in QueueInput) (QueueResult, error) {
	out := QueueResult{Chapters: []int{}, Warnings: []QueueWarning{}, ETAReason: "Chưa đo tốc độ tạo ở chất lượng này."}
	selection := make([]int, 0, len(in.Chapters)+len(in.Redo))
	redo := map[int]bool{}
	for _, n := range in.Redo {
		redo[n] = true
	}
	seen := map[int]bool{}
	// Theo thứ tự gửi lên: chapters trước, rồi các chương "tạo lại từ đầu".
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
	scripts := make([]*Script, 0, len(selection))
	numbers := make([]int, 0, len(selection))
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
		script, err := ReadScript(s.layout, volume, n)
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
			script = nil
		}
		if (script == nil || script.Status != StatusLocked) && !in.Lock {
			return out, conflict("Chương %d chưa chốt bản đọc.", n)
		}
		if script != nil && script.Status != StatusLocked {
			now := time.Now()
			script.Status = StatusLocked
			script.LockedAt = &now
		}
		var segments []Segment
		if script != nil {
			segments = script.Segments
		}
		for _, seg := range segments {
			if len(seg.Flags) > 0 {
				out.Warnings = append(out.Warnings, QueueWarning{n, "Bản đọc còn chỗ cần xem; bạn có thể sửa trên máy tính rồi tạo lại."})
				break
			}
		}
		scripts = append(scripts, script)
		numbers = append(numbers, n)
	}
	if in.VoiceMode != "" {
		cast, err := s.castForRender()
		if err != nil {
			return out, err
		}
		cast.VoiceMode = in.VoiceMode
		cast.Mode = ModeCast
		if in.VoiceMode == ModeSingle {
			cast.Mode = ModeSingle
		}
		if in.VoiceMode == "pack" {
			if err = s.ApplyPackVoice(cast); err != nil {
				return out, err
			}
		}
		if _, err = s.SaveCast(cast); err != nil {
			return out, err
		}
	}
	for i, script := range scripts {
		if script != nil {
			if err := WriteScript(s.layout, script); err != nil {
				return out, err
			}
		}
		n := numbers[i]
		kind := JobKindCreate
		if redo[n] {
			kind = JobKindRedo
		} else {
			done, _ := script.AudioStats()
			_, scriptChanged, _ := script.freshness("")
			if done > 0 && (script.needsComposition(SourceHash(sourceChapterText(source, n))) || scriptChanged) {
				kind = JobKindUpdate
			}
		}
		id, _ := s.queue.Enqueue(volume, n)
		s.queue.Update(id, func(j *Job) {
			j.Kind = kind
			j.Quality = in.Quality
			j.NeedsScript = script.needsComposition(SourceHash(sourceChapterText(source, n)))
			if j.NeedsScript {
				j.Step = "compose"
				j.Segments, j.SegmentsDone, j.ETASec = 0, 0, 0
			} else {
				j.Step = "render"
				j.Segments = len(script.Segments)
				// create 任务从零开始报进度；update/redo 的进度在 runJob 按
				// "真正要读的段"重数，队列里先归零免得把已读段算进来。
				j.SegmentsDone = 0
				if kind == JobKindCreate {
					j.SegmentsDone, _ = script.AudioStats()
				}
				// Speaking duration is not generation time. Progress supplies the
				// latter once the engine has actually been measured.
				j.ETASec = 0
			}
		})
		out.Chapters = append(out.Chapters, numbers[i])
	}
	s.emitQueue()
	s.kick()
	return out, nil
}

func sourceChapterText(source []SourceChapter, chapter int) string {
	sc, _ := findSource(source, chapter)
	return sc.Text
}

func (s *Service) composeQueuedScript(job Job) (*Script, error) {
	old, err := ReadScript(s.layout, job.Volume, job.Chapter)
	if err != nil {
		return nil, err
	}
	if s.queuedComposer == nil {
		if old != nil && !job.NeedsScript {
			return old, nil
		}
		return nil, ErrNoModel
	}
	sc, composer, ref, resolveErr := s.queuedComposer(job.Volume, job.Chapter)
	// Resolve the chapter again when work starts, not from the queue request.
	// A configured model is only required when the text actually needs composing.
	if sc.Chapter == job.Chapter && !old.needsComposition(SourceHash(sc.Text)) {
		return old, nil
	}
	if resolveErr != nil {
		return nil, resolveErr
	}
	if composer == nil {
		return nil, ErrNoModel
	}
	if !sc.Completed || strings.TrimSpace(sc.Text) == "" {
		return nil, conflict("Chương %d chưa nghiệm thu hoặc chưa có chính văn để soạn bản đọc.", job.Chapter)
	}
	cast, err := s.castForRender()
	if err != nil {
		return nil, err
	}
	enriched := EnsureCast(cast, ref.Characters, ref.Support)
	dict, err := s.Pronunciations()
	if err != nil {
		return nil, err
	}
	s.queue.Update(job.ID, func(j *Job) { j.State = JobRendering; j.Step = "compose" })
	s.setComposing(job.Volume, job.Chapter, true)
	defer s.setComposing(job.Volume, job.Chapter, false)
	s.emitQueue()
	result, err := composer.Compose(s.ctx, ComposeInput{
		Volume: job.Volume, Chapter: job.Chapter, Title: sc.Title, Text: sc.Text,
		Characters: composeSpeakers(&enriched, ref), Mode: enriched.Mode,
		Normalize: s.Settings().Normalize, Dict: dict,
	})
	if err != nil {
		return nil, fmt.Errorf("Không soạn được bản đọc chương %d: %w", job.Chapter, err)
	}
	if len(result.Segments) == 0 {
		return nil, conflict("AI chưa trả về đoạn đọc nào cho chương %d. Hãy thử lại.", job.Chapter)
	}
	if s.waitJob(job.ID) == JobCanceled {
		return nil, context.Canceled
	}
	now := time.Now()
	script := &Script{Volume: job.Volume, Chapter: job.Chapter, Status: StatusLocked,
		LockedAt: &now, ComposedAt: &now, SourceHash: SourceHash(sc.Text),
		Model: result.Model, Segments: result.Segments}
	if old != nil {
		script, _ = mergeScript(old, script)
	}
	applyVoiceFlags(script, &enriched)
	if err := WriteScript(s.layout, script); err != nil {
		return nil, err
	}
	s.queue.Update(job.ID, func(j *Job) {
		j.NeedsScript = false
		j.Step = "render"
		j.Segments = len(script.Segments)
	})
	s.emitQueue()
	return script, nil
}

func (s *Service) Prioritize(volume, chapter int) error {
	jobs := s.Queue()
	for i := len(jobs) - 1; i >= 0; i-- {
		if jobs[i].Volume == volume && jobs[i].Chapter == chapter {
			if err := s.queue.Prioritize(jobs[i].ID); err != nil {
				return err
			}
			s.emitQueue()
			return nil
		}
	}
	return ErrNotFound
}
func (s *Service) Retry(jobID, quality string) error {
	if quality != "" && quality != "fast" && quality != "standard" && quality != "high" {
		return conflict("Chất lượng giọng không hợp lệ.")
	}
	job, ok := s.queue.Find(jobID)
	if !ok {
		return ErrNotFound
	}
	if job.State != JobError && job.State != JobPaused {
		return conflict("Chỉ thử lại được việc đang dừng.")
	}
	if quality != "" {
		s.queue.Update(jobID, func(j *Job) { j.Quality = quality })
	}
	return s.Resume(jobID)
}
