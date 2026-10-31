package images

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

type runningEditStore struct{ *editStore }

func (s runningEditStore) Read(ctx context.Context, id string, step domain.CloneStep) (jobedit.Snapshot, error) {
	snapshot, err := s.editStore.Read(ctx, id, step)
	if step == domain.CloneImages {
		snapshot.Step.Status = domain.CloneRunning
		snapshot.Steps = []domain.CloneStepRecord{snapshot.Step}
	}
	return snapshot, err
}

func TestReviewOtherImageWhileRunningLocksDuplicateAndVideoApproval(t *testing.T) {
	s, store := testService()
	store.m.Items[0].Status = "running"
	store.m.Items[0].InFlight = &model.Flight{GenerationID: "first"}
	s.Edits.Store = runningEditStore{store}
	s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	rev := 3
	prompt := "warmer light"
	result, err := s.Redraw(context.Background(), "job", "thumbnail", RedrawInput{ExpectedRevision: &rev, Prompt: &prompt})
	if err != nil || !result.Accepted { t.Fatalf("other image redraw: %+v %v", result, err) }
	if store.m.Items[1].Direction != prompt || store.m.Revision != 4 { t.Fatalf("missing redraw: %+v", store.m) }
	rev = 4
	_, err = s.Redraw(context.Background(), "job", "thumbnail", RedrawInput{ExpectedRevision: &rev})
	requireCode(t, err, domain.CloneErrConflict)
	_, err = s.ApproveAll(context.Background(), "job", RevisionInput{ExpectedRevision: &rev})
	requireCode(t, err, domain.CloneErrConflict)
}
