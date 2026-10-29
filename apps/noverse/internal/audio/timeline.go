package audio

import "time"

// TimelineChapter is the studio's duration-weighted chapter strip. Sec is only
// present after all segments have been generated; partial audio belongs in MadeSec.
type TimelineChapter struct {
	Chapter int    `json:"chapter"`
	Title   string `json:"title"`
	State   string `json:"state"`
	// Kind/Position 只在排进队列后有值（§AK）：update/redo 让界面知道
	// 等的是"cập nhật"；position 是等待序号（1 = Tiếp theo）。
	Kind           string     `json:"kind,omitempty"`
	Position       int        `json:"position,omitempty"`
	Sec            *float64   `json:"sec"`
	EstSec         float64    `json:"estSec"`
	Pct            float64    `json:"pct"`
	Segment        int        `json:"segment"`
	Segments       int        `json:"segments"`
	ETASec         float64    `json:"etaSec"`
	Quality        string     `json:"quality"`
	Reason         string     `json:"reason,omitempty"`
	At             *time.Time `json:"at,omitempty"`
	ChapterChanged bool       `json:"chapterChanged"`
	ScriptChanged  bool       `json:"scriptChanged"`
	StaleReason    string     `json:"staleReason"`
}

type QueueView struct {
	Jobs          []Job              `json:"jobs"`
	Chapters      []TimelineChapter  `json:"chapters"`
	MadeSec       float64            `json:"madeSec"`
	EstTotalSec   float64            `json:"estTotalSec"`
	QualityETASec map[string]float64 `json:"qualityEtaSec,omitempty"`
}

// Timeline reuses the script speaking-rate estimator, including chapters whose
// script is not prepared yet. Unknown text uses the completed-chapter mean,
// then the known estimates' mean; an entirely unknown volume has equal shares.
func (s *Service) Timeline(source []SourceChapter, volume int) (QueueView, error) {
	out := QueueView{Jobs: []Job{}, Chapters: make([]TimelineChapter, 0, len(source))}
	for _, job := range s.Queue() {
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
	madeTotal, madeCount, estimatedTotal, estimatedCount := 0.0, 0, 0.0, 0
	for _, sc := range source {
		script, err := ReadScript(s.layout, volume, sc.Chapter)
		if err != nil {
			return QueueView{}, err
		}
		chapter := TimelineChapter{Chapter: sc.Chapter, Title: sc.Title, State: "unlocked", Quality: s.Settings().Quality}
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
	fallback := 1.0 // equal shares, not an invented speaking-rate estimate
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
