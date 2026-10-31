package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type reviewPollStore struct{ WorkerStore }

func (reviewPollStore) AcquireWorkerLock(context.Context) (WorkerLock, error) {
	return reviewPollLock{}, nil
}
func (reviewPollStore) ListInterruptedJobs(context.Context, int) ([]domain.CloneJob, error) {
	return nil, nil
}
func (reviewPollStore) ListClaimableJobs(context.Context, int) ([]domain.CloneJob, error) {
	return nil, nil
}

type reviewPollLock struct{}

func (reviewPollLock) Check(context.Context) error { return nil }
func (reviewPollLock) Close(context.Context) error { return nil }
func TestWorkerDispatchesDurableReviewsWhileNoJobIsClaimable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	calls := 0
	worker := Worker{Store: reviewPollStore{}, Runner: &Runner{}, OnPoll: func(context.Context) error { calls++; cancel(); return nil }}
	if err := worker.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("idle worker did not dispatch persisted owner review: %d calls", calls)
	}
}
