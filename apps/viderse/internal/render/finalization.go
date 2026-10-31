package render

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// ExportState is the owner-facing lifecycle, including pipeline finalization
// after the browser has finished uploading its staged MP4. Runtime credentials
// still expire at browser completion; this does not reopen the token capability.
func (s *Service) ExportState(ctx context.Context, request ExportRequest) (ExportRequest, error) {
	if request.Status != "done" || request.JobID == "" {
		return request, nil
	}
	records, err := s.Jobs.ListSteps(ctx, request.JobID)
	if err != nil {
		return request, err
	}
	failure := ""
	for _, step := range records {
		if step.Step != domain.CloneExport {
			continue
		}
		if step.Status == domain.CloneSucceeded && step.Output != nil && step.Output.Data["exportId"] == request.ExportID {
			return request, nil
		}
		if step.Status == domain.CloneFailed || step.Status == domain.CloneCancelled || step.Status == domain.CloneWaiting && step.Error != nil {
			failure = "Không hoàn tất được bản xuất"
			if step.Error != nil && step.Error.Message != "" {
				failure = step.Error.Message
			}
		}
	}
	job, err := s.Jobs.GetJob(ctx, request.JobID)
	if err != nil {
		return request, err
	}
	if failure == "" && (job.Status == domain.CloneFailed || job.Status == domain.CloneCancelled || job.Status == domain.CloneSucceeded) {
		failure = "Không hoàn tất được bản xuất"
	}
	if failure != "" {
		request.Status = "failed"
		request.Error = &RequestError{Code: "finalization_failed", Message: failure}
		return request, nil
	}
	request.Status = "running"
	request.FinishedAt = ""
	request.Progress = &ExportProgress{Phase: "compressing"}
	return request, nil
}
