package images

import (
	"context"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

func unskipService(t *testing.T) (*Service, *editStore) {
	t.Helper()
	s, store := testService()
	other := store.m.Items[0]
	other.ID = "scene-2"
	store.m.Items = append(store.m.Items, other)
	s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	s.QuotaLeft = func(context.Context) (int, error) { t.Fatal("undo queried paid image quota"); return 0, nil }
	s.Quotas = func(context.Context) ([]ProviderQuota, error) {
		t.Fatal("undo queried provider quotas")
		return nil, nil
	}
	return s, store
}

func reviewAction(ctx context.Context, s *Service, revision int, actions ...Action) (ReviewResult, error) {
	return s.Review(ctx, "job", ReviewInput{ExpectedRevision: &revision, Actions: actions})
}

func TestReviewUnskipReloadRestoresOlderSelectionWithoutGeneration(t *testing.T) {
	for _, status := range []string{"review", "approved", "failed", "blocked", "legacy-ready"} {
		t.Run(status, func(t *testing.T) {
			s, store := unskipService(t)
			one, two := 1, 2
			i := &store.m.Items[0]
			i.Status, i.CurrentVersion, i.ReviewVersion = status, &one, &two
			i.Versions = append(i.Versions, model.Version{Version: 2, SourceFingerprint: model.SourceFingerprint(*i)})
			before := *i
			skipped, err := reviewAction(context.Background(), s, 3, Action{ID: "scene-1", Op: "skip"})
			if err != nil {
				t.Fatal(err)
			}
			if store.m.Items[0].BeforeSkip == nil {
				t.Fatal("committed manifest lost undo snapshot")
			}
			// editStore serializes every commit and read, exercising persisted JSON
			// rather than retaining the model transition's in-memory pointers.
			restored, err := reviewAction(context.Background(), s, skipped.Manifest.Revision, Action{ID: "scene-1", Op: "unskip"})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(store.m.Items[0], before) {
				t.Fatalf("undo changed selected older version or review state: before=%+v after=%+v", before, store.m.Items[0])
			}
			if restored.Generation != nil || store.change.Rerun || len(store.m.QueueOnly) != 0 || len(store.drafts) != 0 {
				t.Fatalf("undo requested generation or owner approval: %+v %+v", restored, store.change)
			}
			if !restored.ChosenChanged || !restored.Invalidated || store.change.InvalidateFrom != domain.CloneBuild {
				t.Fatalf("restored chosen media did not invalidate downstream: %+v %+v", restored, store.change)
			}
			_, err = reviewAction(context.Background(), s, skipped.Manifest.Revision, Action{ID: "scene-1", Op: "unskip"})
			requireCode(t, err, domain.CloneErrConflict)
			_, err = reviewAction(context.Background(), s, restored.Manifest.Revision, Action{ID: "scene-1", Op: "unskip"})
			requireCode(t, err, domain.CloneErrConflict)
			if store.commits != 2 {
				t.Fatalf("repeated undo committed %d changes", store.commits)
			}
		})
	}
}

func TestReviewUnskipInvalidatesSceneInclusionWithoutSelectingCandidate(t *testing.T) {
	s, store := unskipService(t)
	skipped, err := reviewAction(context.Background(), s, 3, Action{ID: "scene-1", Op: "skip"})
	if err != nil || !skipped.Invalidated || store.change.InvalidateFrom != domain.CloneBuild {
		t.Fatalf("skip missed scene inclusion change: %+v %+v %v", skipped, store.change, err)
	}
	restored, err := reviewAction(context.Background(), s, skipped.Manifest.Revision, Action{ID: "scene-1", Op: "unskip"})
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Invalidated || restored.ChosenChanged || restored.Approved || restored.Generation != nil || store.change.Rerun || store.change.InvalidateFrom != domain.CloneBuild {
		t.Fatalf("candidate undo invented selection/generation or missed stage gap: %+v %+v", restored, store.change)
	}
	if store.m.Items[0].Status != "review" || store.m.Items[0].CurrentVersion != nil || store.m.Items[0].ReviewVersion == nil {
		t.Fatalf("candidate review state lost: %+v", store.m.Items[0])
	}
}

func TestReviewUnskipRejectsStaleLaterActionAndInvalidPayload(t *testing.T) {
	s, store := unskipService(t)
	skipped, err := reviewAction(context.Background(), s, 3, Action{ID: "scene-1", Op: "skip"})
	if err != nil {
		t.Fatal(err)
	}
	one := 1
	if _, err := reviewAction(context.Background(), s, skipped.Manifest.Revision, Action{ID: "thumbnail", Op: "approve", Version: &one}); err != nil {
		t.Fatal(err)
	}
	_, err = reviewAction(context.Background(), s, skipped.Manifest.Revision, Action{ID: "scene-1", Op: "unskip"})
	requireCode(t, err, domain.CloneErrConflict)
	empty := ""
	for _, action := range []Action{
		{ID: "scene-1", Op: "unskip", Version: &one},
		{ID: "scene-1", Op: "unskip", Prompt: &empty},
	} {
		_, err := reviewAction(context.Background(), s, store.m.Revision, action)
		requireCode(t, err, domain.CloneErrInvalidRequest)
	}
	if store.commits != 2 || store.m.Items[0].Status != "skipped" {
		t.Fatal("invalid or stale undo changed manifest")
	}
}

func TestReviewBatchUnskipUsesSameSnapshotSemanticsAtomically(t *testing.T) {
	s, store := unskipService(t)
	skipped, err := reviewAction(context.Background(), s, 3, Action{ID: "scene-1", Op: "skip"})
	if err != nil {
		t.Fatal(err)
	}
	store.m.Items[2].Status, store.m.Items[2].SkipReason = "skipped", "refused"
	_, err = reviewAction(context.Background(), s, skipped.Manifest.Revision,
		Action{ID: "scene-1", Op: "unskip"}, Action{ID: "scene-2", Op: "unskip"})
	requireCode(t, err, domain.CloneErrConflict)
	if store.commits != 1 || store.m.Items[0].Status != "skipped" {
		t.Fatal("failed batch partially consumed undo")
	}
}

type unskipBoundaryStore struct {
	*editStore
	imageStatus    domain.CloneStatus
	commitConflict bool
}

func (s unskipBoundaryStore) Read(ctx context.Context, id string, step domain.CloneStep) (jobedit.Snapshot, error) {
	snap, err := s.editStore.Read(ctx, id, step)
	if step == domain.CloneImages {
		snap.Step.Status = s.imageStatus
	}
	return snap, err
}

func TestReviewBatchUnskipRestoresEachPersistedItem(t *testing.T) {
	s, store := unskipService(t)
	third := store.m.Items[0]
	third.ID = "scene-3"
	store.m.Items = append(store.m.Items, third)
	before, err := model.CloneManifest(store.m)
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := reviewAction(context.Background(), s, 3,
		Action{ID: "scene-1", Op: "skip"}, Action{ID: "scene-2", Op: "skip"})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reviewAction(context.Background(), s, skipped.Manifest.Revision,
		Action{ID: "scene-1", Op: "unskip"}, Action{ID: "scene-2", Op: "unskip"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Items, store.m.Items) || restored.Generation != nil || store.change.Rerun || store.commits != 2 {
		t.Fatalf("batch undo did not restore each original item: %+v %+v", store.m, restored)
	}
}

func TestReviewUnskipRejectsLegacyAndRefusedWithoutCommitting(t *testing.T) {
	for _, reason := range []string{"owner", "refused"} {
		t.Run(reason, func(t *testing.T) {
			s, store := unskipService(t)
			store.m.Items[0].Status, store.m.Items[0].SkipReason = "skipped", reason
			before, err := model.CloneManifest(store.m)
			if err != nil {
				t.Fatal(err)
			}
			_, err = reviewAction(context.Background(), s, 3, Action{ID: "scene-1", Op: "unskip"})
			requireCode(t, err, domain.CloneErrConflict)
			if store.commits != 0 || !reflect.DeepEqual(before, store.m) {
				t.Fatal("legacy/refused undo fabricated or changed state")
			}
		})
	}
}
func (s unskipBoundaryStore) Commit(ctx context.Context, before jobedit.Snapshot, change jobedit.Change) error {

	if s.commitConflict {
		return jobedit.Conflict("worker advanced after review read")
	}
	return s.editStore.Commit(ctx, before, change)
}

func TestReviewUnskipRejectsActiveConsumersAndPropagatesCommitCAS(t *testing.T) {
	for _, imageStatus := range []domain.CloneStatus{domain.CloneSucceeded, domain.CloneRunning} {
		for _, consumer := range []domain.CloneStep{domain.CloneBuild, domain.CloneExport} {
			t.Run(string(imageStatus)+"/"+string(consumer), func(t *testing.T) {
				s, store := unskipService(t)
				skipped, err := reviewAction(context.Background(), s, 3, Action{ID: "scene-1", Op: "skip"})
				if err != nil {
					t.Fatal(err)
				}
				before, err := model.CloneManifest(store.m)
				if err != nil {
					t.Fatal(err)
				}
				store.steps = []domain.CloneStepRecord{{Step: consumer, Status: domain.CloneRunning}}
				s.Edits.Store = unskipBoundaryStore{editStore: store, imageStatus: imageStatus}
				_, err = reviewAction(context.Background(), s, skipped.Manifest.Revision, Action{ID: "scene-1", Op: "unskip"})
				requireCode(t, err, domain.CloneErrConflict)
				if store.commits != 1 || !reflect.DeepEqual(before, store.m) || len(store.drafts) != 0 {
					t.Fatal("busy consumer changed manifest/undo receipt or owner approval")
				}
			})
		}
	}
	s, store := unskipService(t)
	skipped, err := reviewAction(context.Background(), s, 3, Action{ID: "scene-1", Op: "skip"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := model.CloneManifest(store.m)
	if err != nil {
		t.Fatal(err)
	}
	s.Edits.Store = unskipBoundaryStore{editStore: store, commitConflict: true}
	out, err := reviewAction(context.Background(), s, skipped.Manifest.Revision, Action{ID: "scene-1", Op: "unskip"})
	requireCode(t, err, domain.CloneErrConflict)
	if store.commits != 1 || !reflect.DeepEqual(before, store.m) || out.Generation != nil || out.Manifest.Revision != 0 {
		t.Fatalf("commit race acknowledged or consumed undo: %+v", out)
	}
}
