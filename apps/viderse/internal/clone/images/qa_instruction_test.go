package images

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

type qaAutoApproveEnabled struct{}

func (qaAutoApproveEnabled) JobAutoApprove(context.Context, string) (bool, bool, error) {
	return true, true, nil
}

// Only the persistence boundary is replaced. The manifest load, score masking,
// dispatch decision and owner review are the real production implementations.
type qaReviewLedger struct {
	revision    int
	outcomes    []autonomy.ReviewOutcome
	toggleReads int
}

func (s *qaReviewLedger) PendingReviews(context.Context, int) ([]autonomy.PendingReview, error) {
	return []autonomy.PendingReview{{JobID: "job", ChannelID: "channel", ImageRevision: s.revision, AutoApprove: true}}, nil
}
func (s *qaReviewLedger) RecordReview(_ context.Context, outcome autonomy.ReviewOutcome) error {
	s.outcomes = append(s.outcomes, outcome)
	return nil
}
func (s *qaReviewLedger) ChannelAutoApprove(context.Context, string) (bool, error) {
	s.toggleReads++
	return true, nil
}
func (*qaReviewLedger) BeginAutoFix(context.Context, string, domain.CloneStep, string, time.Time) (bool, error) {
	return false, errors.New("unexpected repair during image review")
}
func (*qaReviewLedger) SettleAutoFix(context.Context, string, domain.CloneStep) error {
	return errors.New("unexpected repair settlement during image review")
}

func TestQAContractPersistedIssueBlocksAutomaticApprovalButKeepsOwnerReview(t *testing.T) {
	for _, tc := range []struct {
		name         string
		blockedIndex int
		approved     bool
		newCandidate bool
	}{
		{name: "scene candidate", blockedIndex: 0},
		{name: "cover candidate", blockedIndex: 1},
		{name: "current scene", blockedIndex: 0, approved: true},
		{name: "current cover", blockedIndex: 1, approved: true},
		{name: "unapplied candidate beats old high score", blockedIndex: 0, newCandidate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, store := testService()
			s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
			s.AutoApprove = qaAutoApproveEnabled{}
			if tc.approved {
				approvedFixture(store)
			}
			for index := range store.m.Items {
				v := &store.m.Items[index].Versions[0]
				v.Score = &domain.ImageScore{Value: 100}
				v.Path = "images/" + store.m.Items[index].ID + ".webp"
				v.SHA256 = "paid-image-hash-" + store.m.Items[index].ID
				v.CostImages = 1
				store.artifacts = append(store.artifacts, domain.CloneArtifact{ID: "artifact-" + store.m.Items[index].ID, Path: v.Path, SHA256: v.SHA256})
			}
			blocked := &store.m.Items[tc.blockedIndex]
			if tc.newCandidate {
				one, two := 1, 2
				blocked.CurrentVersion = &one
				blocked.ReviewVersion = &two
				candidate := blocked.Versions[0]
				candidate.Version = 2
				blocked.Versions = append(blocked.Versions, candidate)
			}
			blocked.Versions[len(blocked.Versions)-1].ScoreIssue = "Chưa áp dụng kiểm tra nhạc/giọng đọc; cần chủ kênh duyệt."
			before, err := model.CloneManifest(store.m)
			if err != nil {
				t.Fatal(err)
			}
			reviewer := AutoReviewer{Service: s}
			state, err := reviewer.ReviewState(ctx, "job")
			if err != nil || !state.Ready || state.Revision != 3 || len(state.Items) != 2 {
				t.Fatalf("images must remain owner-reviewable: state=%+v err=%v", state, err)
			}
			if state.Items[tc.blockedIndex].Score != nil || state.Items[1-tc.blockedIndex].Score == nil {
				t.Fatalf("persisted issue did not mask only its own score: %+v", state.Items)
			}
			ledger := &qaReviewLedger{revision: state.Revision}
			dispatcher := autonomy.Service{Store: ledger, Reviewer: reviewer}
			if err := dispatcher.DispatchReviews(ctx); err != nil {
				t.Fatal(err)
			}
			if len(ledger.outcomes) != 1 || ledger.outcomes[0].Result != autonomy.ResultBelow || ledger.outcomes[0].Verdict.AllPass || ledger.outcomes[0].Verdict.Passed != 1 || len(ledger.outcomes[0].Verdict.Below) != 1 || ledger.outcomes[0].Verdict.Below[0].ID != blocked.ID || ledger.outcomes[0].Verdict.Below[0].Value != -1 || ledger.toggleReads != 0 || store.commits != 0 || len(store.drafts) != 0 {
				t.Fatalf("unapplied instruction entered approval: outcomes=%+v toggleReads=%d commits=%d drafts=%v", ledger.outcomes, ledger.toggleReads, store.commits, store.drafts)
			}
			view, err := s.Read(ctx, "job")
			if err != nil {
				t.Fatal(err)
			}
			item := view.Items[tc.blockedIndex]
			version := item.Versions[len(item.Versions)-1]
			if item.Score != nil || item.ScoreIssue == "" || version.Score != nil || version.ScoreIssue == "" || version.OriginalURL == "" || version.Path == "" {
				t.Fatalf("manual-review API hid issue or paid image: item=%+v version=%+v", item, version)
			}
			if len(view.Images) != len(view.Items) {
				t.Fatalf("canonical image view incomplete: %+v", view.Images)
			}
			image := view.Images[tc.blockedIndex]
			imageVersion := image.Versions[len(image.Versions)-1]
			if image.ScoreIssue == "" || imageVersion.ScoreIssue == "" || imageVersion.URL == nil || *imageVersion.URL == "" {
				t.Fatalf("canonical review API hid issue or paid bytes: image=%+v version=%+v", image, imageVersion)
			}
			if view.Autonomy == nil || view.Autonomy.Min != 85 || view.Autonomy.Passed != 1 || len(view.Autonomy.Below) != 1 || view.Autonomy.Below[0].ID != blocked.ID || view.Autonomy.Below[0].Value != -1 {
				t.Fatalf("summary counted unapplied score: %+v", view.Autonomy)
			}
			if !reflect.DeepEqual(store.m, before) {
				t.Fatal("read/dispatch destroyed the persisted paid image or original score")
			}
			if !tc.approved {
				revision := store.m.Revision
				if _, err := s.ApproveAll(ctx, "job", RevisionInput{ExpectedRevision: &revision}); err != nil {
					t.Fatalf("owner must still be able to approve paid images: %v", err)
				}
				if !model.ImagesApproved(store.m) || store.commits != 1 {
					t.Fatalf("owner approval did not persist: %+v", store.m)
				}
				for index := range before.Items {
					if !reflect.DeepEqual(store.m.Items[index].Versions, before.Items[index].Versions) {
						t.Fatal("owner approval rewrote paid image history, score or issue")
					}
				}
			}
		})
	}
}

func TestQAContractNoIssuePreservesPersistedScoreAnd85ApprovalBoundary(t *testing.T) {
	ctx := context.Background()
	s, store := testService()
	s.AutoApprove = qaAutoApproveEnabled{}
	for index := range store.m.Items {
		store.m.Items[index].Versions[0].Score = &domain.ImageScore{Value: 85}
	}
	before, err := model.CloneManifest(store.m)
	if err != nil {
		t.Fatal(err)
	}
	state, err := (AutoReviewer{Service: s}).ReviewState(ctx, "job")
	if err != nil || !state.Ready || !autonomy.Evaluate(domain.AutoApproveMin, state.Items).AllPass {
		t.Fatalf("legacy 85-point review changed: %+v %v", state, err)
	}
	view, err := s.Read(ctx, "job")
	if err != nil || view.Autonomy == nil || view.Autonomy.Passed != 2 || len(view.Autonomy.Below) != 0 {
		t.Fatalf("legacy summary changed: %+v %v", view.Autonomy, err)
	}
	for _, item := range view.Items {
		if item.Score == nil || item.Score.Value != 85 || item.ScoreIssue != "" || len(item.Versions) != 1 || item.Versions[0].Score == nil || item.Versions[0].Score.Value != 85 || item.Versions[0].ScoreIssue != "" {
			t.Fatalf("legacy score lost: %+v", item)
		}
	}
	if !reflect.DeepEqual(store.m, before) || store.commits != 0 {
		t.Fatal("no-note read altered legacy persisted scores")
	}
}
