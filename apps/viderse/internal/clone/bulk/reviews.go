package bulk

import "context"

type ReviewIntent struct {
	JobID         string
	ImageRevision int
}
type ApprovalStore interface {
	ListBulkReviewIntents(context.Context, int) ([]ReviewIntent, error)
	ConsumeBulkReviewIntent(context.Context, ReviewIntent, func(context.Context, string) error) error
}

// DispatchReviews is called by the existing worker poll after attempt leases
// have been released. The same Render approval transition is used as HTTP.
func (s *Service) DispatchReviews(ctx context.Context) error {
	if s.Approvals == nil || s.ApproveProject == nil {
		return nil
	}
	intents, err := s.Approvals.ListBulkReviewIntents(ctx, 100)
	if err != nil {
		return err
	}
	for _, intent := range intents {
		if err := s.Approvals.ConsumeBulkReviewIntent(ctx, intent, s.ApproveProject); err != nil {
			return err
		}
		if s.Publish != nil {
			s.Publish("clone.job.updated", map[string]any{"jobId": intent.JobID})
		}
	}
	return nil
}
