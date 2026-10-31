package queue

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

var stepSlotLimits = map[domain.CloneStep]int{domain.CloneDownload: 2, domain.CloneBackdrop: 1, domain.CloneTranscript: 1, domain.CloneScenes: 1, domain.CloneExport: 1}

func (s *ResourceSlots) Wrap(step domain.CloneStep, handler pipeline.StepHandler) pipeline.StepHandler {
	limit, ok := stepSlotLimits[step]
	if !ok {
		return handler
	}
	return pipeline.HandlerFunc(func(ctx context.Context, sc *pipeline.StepContext) (pipeline.StepResult, error) {
		release, err := s.Acquire(ctx, "clone-step:"+string(step), limit)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		defer release()
		return handler.Run(ctx, sc)
	})
}

// WithImageSlot is directly assignable to StepContext.ImageSlot. Rules are
// reread at use time so PATCH /api/queue takes effect without restart.
func (s *Service) WithImageSlot(slots *ResourceSlots) func(context.Context, func(context.Context) error) error {
	return func(ctx context.Context, run func(context.Context) error) error {
		rules, err := s.Rules(ctx)
		if err != nil {
			return err
		}
		return slots.With(ctx, "images", rules.ImageWorkers, run)
	}
}
