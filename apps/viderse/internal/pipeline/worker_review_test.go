package pipeline

import (
	"context"
	"errors"
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

// Huỷ vòng đời serve trong lúc một bước còn chạy là dừng sạch: lỗi mà bước
// trả về lúc đó (context.Canceled, hay lỗi kết nối pgx bọc nó) không được
// biến lệnh dừng thành lỗi của serve.
func TestWorkerStopsCleanlyWhenLifecycleCancelledMidPoll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := Worker{Store: reviewPollStore{}, Runner: &Runner{}, OnPoll: func(context.Context) error {
		cancel()
		return context.Canceled
	}}
	if err := worker.Run(ctx); err != nil {
		t.Fatalf("dừng sạch trả lỗi %v, muốn nil", err)
	}
}

// Lỗi thật (vòng đời còn sống) vẫn phải nổi lên cho serve báo.
func TestWorkerReportsGenuinePollFailure(t *testing.T) {
	boom := errors.New("kho hỏng")
	worker := Worker{Store: reviewPollStore{}, Runner: &Runner{}, OnPoll: func(context.Context) error { return boom }}
	if err := worker.Run(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Run = %v, muốn %v", err, boom)
	}
}
