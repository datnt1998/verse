package imp

import "time"

// Progress is the versioned mobile projection carried by every import job event.
type Progress struct {
	SchemaVersion int            `json:"schemaVersion"`
	Steps         []ProgressStep `json:"steps"`
	EtaSec        *int           `json:"etaSec,omitempty"`
	Summary       *Summary       `json:"summary,omitempty"`
	Preview       *Preview       `json:"preview,omitempty"`
	Continued     bool           `json:"continued"`
}

type ProgressStep struct {
	SchemaVersion int    `json:"schemaVersion"`
	ID            string `json:"id"`
	Title         string `json:"title"`
	Current       int    `json:"current"`
	Total         int    `json:"total"`
	Status        string `json:"status"`
}

type Summary struct {
	SchemaVersion int `json:"schemaVersion"`
	Chapters      int `json:"chapters"`
	Characters    int `json:"characters"`
	Foreshadows   int `json:"foreshadows"`
}

// AdvanceProgress preserves measured counts through retry and terminal events.
// ETA is available only after measured chapter work, never a fabricated duration.
func AdvanceProgress(previous *Progress, ev Event, started time.Time) *Progress {
	out := Progress{SchemaVersion: 1}
	if previous != nil {
		out = *previous
		out.Steps = append([]ProgressStep(nil), previous.Steps...)
	}
	if len(out.Steps) == 0 {
		for i, id := range []string{"ingesting", "segmenting", "analyzing", "synthesizing", "publishing"} {
			title := []string{"Đọc tệp", "Chia chương", "Tóm tắt từng chương", "Dựng nhân vật & bối cảnh", "Lập dàn ý & phục bút"}[i]
			out.Steps = append(out.Steps, ProgressStep{SchemaVersion: 1, ID: id, Title: title, Status: "pending"})
		}
	}
	idx := -1
	switch ev.Stage {
	case StageIngesting:
		idx = 0
	case StageSegmenting, StageAwaitingConfirmation:
		idx = 1
	case StageAnalyzing:
		idx = 2
	case StageSynthesizing, StageAwaitingStoryStatus:
		idx = 3
	case StageValidating, StagePublishing:
		idx = 4
	case StageDone:
		idx = 5
	}
	if idx >= 0 {
		for i := range out.Steps {
			step := &out.Steps[i]
			if i < idx {
				step.Status = "done"
				if step.Total == 0 {
					step.Total = 1
					if i > 0 && ev.Total > 0 {
						step.Total = ev.Total
					}
				}
				step.Current = step.Total
			}
			if i == idx {
				step.Status = "running"
				if ev.Total > 0 {
					step.Current = ev.Current
					step.Total = ev.Total
				}
				if step.Total > 0 && step.Current >= step.Total {
					step.Status = "done"
				}
			}
		}
	}
	out.EtaSec = nil
	if ev.Stage == StageAnalyzing && ev.Current > 0 && ev.Total > ev.Current && !started.IsZero() {
		remaining := int(time.Since(started).Seconds() * float64(ev.Total-ev.Current) / float64(ev.Current))
		out.EtaSec = &remaining
	}
	if ev.Preview != nil {
		out.Preview = ev.Preview
	}
	if ev.Summary != nil {
		out.Summary = ev.Summary
	}
	if ev.Stage == StageDone {
		out.Continued = ev.Continued
	}
	return &out
}

func (r *runner) completionSummary() (*Summary, error) {
	progress, err := r.deps.Store.Progress.Load()
	if err != nil {
		return nil, err
	}
	chars, err := r.deps.Store.Characters.Load()
	if err != nil {
		return nil, err
	}
	foreshadows, err := r.deps.Store.World.LoadForeshadowLedger()
	if err != nil {
		return nil, err
	}
	n := 0
	if progress != nil {
		n = len(progress.CompletedChapters)
	}
	return &Summary{SchemaVersion: 1, Chapters: n, Characters: len(chars), Foreshadows: len(foreshadows)}, nil
}
