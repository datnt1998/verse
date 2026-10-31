package pipeline

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func activityTransition(status domain.CloneStatus, step *domain.CloneStepRecord) string {
	switch status {
	case domain.CloneSucceeded:
		return "ready_to_post"
	case domain.CloneFailed:
		return "needs_attention"
	case domain.CloneWaiting:
		if step != nil && step.Error != nil && step.Error.Code == domain.CloneErrAwaitingReview && (step.Step == domain.CloneImages || step.Step == domain.CloneBuild) {
			return "review_ready"
		}
		if step != nil && step.Error != nil && step.Error.Code == domain.CloneErrQuota {
			return ""
		}
		return "needs_attention"
	}
	return ""
}
func (r *Runner) humanState(ctx context.Context, job *domain.CloneJob, status domain.CloneStatus, step *domain.CloneStepRecord) {
	if r.OnHumanState == nil || job == nil {
		return
	}
	if kind := activityTransition(status, step); kind != "" {
		r.OnHumanState(ctx, job, kind)
	}
}
