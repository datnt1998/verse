package export

import (
	"context"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
)

type Deps struct {
	Gate   func(context.Context, string) (quality.GateDecision, error)
	Export func(context.Context, string, func(render.ExportProgress, bool)) (editor.ExportRecord, render.ExportLaunchResult, error)
}
type Handler struct{ Deps Deps }

func Units(phase string, done, total *int64) int64 {
	from, to := int64(0), int64(750)
	if phase == "uploading" {
		from, to = 750, 800
	}
	if phase == "compressing" {
		from, to = 800, 1000
	}
	if done == nil || total == nil || *total <= 0 {
		return from
	}
	d := max(int64(0), min(*done, *total))
	return from + (to-from)*d / *total
}
func (h Handler) Run(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
	gate, err := h.Deps.Gate(ctx, s.Job.ID)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if !gate.Allowed {
		return pipeline.StepResult{}, steps.CloneError(domain.CloneErrQualityGate, strings.Join(gate.Reasons, "; "), "Sửa các mục chưa đạt trong editor rồi Thử lại, hoặc bấm Vẫn xuất để xuất như hiện tại")
	}
	var progressErr error
	record, result, err := h.Deps.Export(ctx, s.Job.ID, func(p render.ExportProgress, rerun bool) {
		done, total := p.FramesDone, p.FramesTotal
		if p.Phase == "uploading" {
			done, total = p.BytesSent, p.BytesTotal
		}
		units := Units(p.Phase, done, total)
		phase := p.Phase
		if rerun {
			phase += "-again"
		}
		if progressErr == nil {
			n := int64(1000)
			progressErr = s.Progress(ctx, domain.CloneProgress{UnitsDone: units, UnitsTotal: &n, Phase: phase})
		}
	})
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if progressErr != nil {
		return pipeline.StepResult{}, progressErr
	}
	return pipeline.StepResult{Data: map[string]any{"projectId": result.ProjectID, "exportId": record.ID, "fileName": record.FileName, "size": record.Size, "durationMs": result.DurationMS, "accepted": gate.Accepted}}, nil
}
