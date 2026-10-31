package score

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
)

type Deps struct {
	Measure  func(context.Context, string, bool) (*quality.Score, error)
	Accepted func(context.Context, string) (bool, error)
}
type Handler struct{ Deps Deps }

func (h Handler) Run(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
	accepted := false
	var err error
	if h.Deps.Accepted != nil {
		accepted, err = h.Deps.Accepted(ctx, s.Job.ID)
		if err != nil {
			return pipeline.StepResult{}, err
		}
	}
	var score *quality.Score
	// One re-measure/re-score after a gate miss; repeatedly rendering a failed
	// checklist cannot fix it and must leave the decision with the owner.
	for round := range 2 {
		if err = s.Progress(ctx, domain.CloneProgress{Phase: "measuring"}); err != nil {
			return pipeline.StepResult{}, err
		}
		score, err = h.Deps.Measure(ctx, s.Job.ID, round > 0)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		if score == nil {
			return pipeline.StepResult{}, steps.CloneError(domain.CloneErrInternal, "Không có kết quả Độ hoàn thiện", "Đo lại")
		}
		if accepted || quality.AllowsExport(score) {
			break
		}
	}
	file := filepath.Join(s.StagingDir, "score.json")
	if err = os.MkdirAll(s.StagingDir, 0700); err != nil {
		return pipeline.StepResult{}, err
	}
	raw, err := json.MarshalIndent(score, "", "  ")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if err = os.WriteFile(file, raw, 0600); err != nil {
		return pipeline.StepResult{}, err
	}
	artifact := media.Artifact{Path: "score/score.json", File: file, ContentType: "application/json"}
	if !accepted && !quality.AllowsExport(score) {
		if err = s.Publish(ctx, []media.Artifact{artifact}, nil); err != nil {
			return pipeline.StepResult{}, err
		}
		return pipeline.StepResult{}, steps.CloneError(domain.CloneErrQualityGate, "Video chưa đạt Độ hoàn thiện để xuất", "Sửa các mục chưa đạt trong editor rồi Thử lại, hoặc bấm Vẫn xuất để xuất như hiện tại")
	}
	return pipeline.StepResult{Artifacts: []media.Artifact{artifact}, Data: map[string]any{"total": score.Total, "passed": score.Passed, "accepted": accepted, "score": score, "measurementId": score.MeasurementID}}, nil
}
