package render

import (
	"context"
	"errors"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
)

func (s *Service) ScoreForPipeline(ctx context.Context, jobID string, force bool) (*quality.Score, error) {
	if !force {
		binding, err := s.CurrentBinding(ctx, jobID)
		if err != nil {
			return nil, err
		}
		measurements, err := s.Store.ListMeasurements(jobID)
		if err != nil {
			return nil, err
		}
		for _, r := range measurements {
			if r.Status == "measured" && r.Result != nil && r.Binding == binding {
				return s.Score(ctx, jobID, &r.Result.LUFS, &r.ID, nil)
			}
		}
	}
	_, score, err := s.Measure(ctx, jobID)
	if err == nil {
		return score, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	state, stateErr := s.State(jobID)
	if stateErr != nil {
		return nil, stateErr
	}
	if state.AcceptedAt != nil {
		return s.Score(ctx, jobID, nil, nil, nil)
	}
	return nil, err
}
func (s *Service) FailQueuedExports(jobID string, cause error) error {
	requests, err := s.Store.ListExports()
	if err != nil {
		return err
	}
	code := domain.CloneErrInternal
	var clone *domain.CloneError
	if errors.As(cause, &clone) {
		code = clone.Code
	}
	for _, r := range requests {
		if r.JobID != jobID || r.Status != "queued" {
			continue
		}
		_, err = s.Store.mutateExport(r.ID, func(request *ExportRequest) error {
			if request.Status != "queued" {
				return nil
			}
			finishExport(request, "failed", s.Store.now())
			request.Error = &RequestError{Code: code, Message: cause.Error()}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
