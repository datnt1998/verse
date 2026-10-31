package render

import (
	"context"
	"fmt"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

// BeginCloneExport serializes duplicate HTTP approvals and leaves execution to
// the fenced worker. An existing live intent is returned without resetting any
// score/export step that the worker has already started.
func (s *Service) BeginCloneExport(ctx context.Context, jobID string, accept bool) (ExportRequest, error) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	_, _, pid, _, err := s.Inputs(ctx, jobID)
	if err != nil {
		return ExportRequest{}, err
	}
	if err = s.CheckTemplate(ctx, jobID); err != nil {
		return ExportRequest{}, err
	}
	gate, err := s.CheckGate(ctx, pid, accept)
	if err != nil {
		return ExportRequest{}, err
	}
	if !gate.Allowed {
		return ExportRequest{}, renderError(domain.CloneErrQualityGate, fmt.Sprintf("Video chưa đạt Độ hoàn thiện để xuất: %v", gate.Reasons))
	}
	requests, err := s.Store.ListExports()
	if err != nil {
		return ExportRequest{}, err
	}
	for _, r := range requests {
		if r.JobID == jobID && r.ProjectID == pid && !r.Terminal() {
			job, err := s.Jobs.GetJob(ctx, jobID)
			if err != nil {
				return ExportRequest{}, err
			}
			if job == nil {
				return ExportRequest{}, pipeline.ErrNotFound
			}
			if job.Status == domain.CloneFailed && s.Resume != nil {
				if err = s.Resume(ctx, jobID); err != nil {
					return ExportRequest{}, err
				}
			}
			return r, nil
		}
	}
	if s.Resume == nil {
		return ExportRequest{}, pipeline.ErrConflict
	}
	request, err := s.Store.CreateExport(pid, jobID)
	if err != nil {
		return request, err
	}
	if err = s.Resume(ctx, jobID); err != nil {
		_, _ = s.Store.CancelExport(request.ID, request.Nonce)
		return request, err
	}
	return request, nil
}
func (s *Service) PendingMeasurement(jobID, pid string, binding quality.Binding) (MeasurementRequest, error) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	requests, err := s.Store.ListMeasurements(jobID)
	if err != nil {
		return MeasurementRequest{}, err
	}
	for _, r := range requests {
		if r.ProjectID == pid && r.Status == "queued" && r.Binding == binding {
			return r, nil
		}
	}
	return s.Store.CreateMeasurement(jobID, pid, binding)
}

// Recover runs only under the worker advisory lock. Pipeline-owned requests are
// left queued for that pipeline; independent editor measurements are restarted.
func (s *Service) Recover(ctx context.Context) error {
	if err := s.Supervisor.Sweep(ctx, true); err != nil {
		return err
	}
	exports, err := s.Store.ListExports()
	if err != nil {
		return err
	}
	for _, request := range exports {
		if request.Status != "queued" || request.JobID == "" {
			continue
		}
		job, err := s.Jobs.GetJob(ctx, request.JobID)
		if err != nil {
			return err
		}
		if job == nil || job.SupersededAt != nil {
			if _, err = s.Store.CancelExport(request.ID, request.Nonce); err != nil {
				return err
			}
			continue
		}
		if s.Resume == nil {
			return pipeline.ErrConflict
		}
		// The existing durable intent is the owner's prior approval. After
		// kill -9 the worker reconciles its interrupted step to failed; renew
		// that pipeline, preserving the request ID and its rotated credential.
		if err = s.Resume(ctx, request.JobID); err != nil {
			return err
		}
	}
	requests, err := s.Store.AllMeasurements()
	if err != nil {
		return err
	}
	for _, r := range requests {
		if r.Status != "queued" {
			continue
		}
		job, err := s.Jobs.GetJob(ctx, r.JobID)
		if err != nil {
			return err
		}
		if job == nil {
			_, _ = s.Store.CancelMeasurement(r.ID, r.Nonce)
			continue
		}
		if job.Status == domain.ClonePending || job.Status == domain.CloneRunning {
			continue
		}
		go func(r MeasurementRequest) { _, _ = s.Supervisor.LaunchMeasurement(ctx, r) }(r)
	}
	return nil
}
