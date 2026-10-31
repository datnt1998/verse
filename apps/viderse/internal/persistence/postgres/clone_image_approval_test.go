package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

func TestVideoImageApprovalCountsOnceToday(t *testing.T) {
	ctx := context.Background()
	store := cloneStore(t)
	job := cloneTestJob(t, store, "approval-today")
	before, err := store.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	n := 1
	m := images.Manifest{Version: 1, Revision: 1, Items: []images.Item{
		{ID: "scene", Kind: "scene", Status: "review", ReviewVersion: &n},
		{ID: "cover", Kind: "thumbnail", Status: "review", ReviewVersion: &n},
	}}
	for i := range m.Items {
		m.Items[i].Versions = []images.Version{{Version: 1, SourceFingerprint: images.SourceFingerprint(m.Items[i])}}
	}
	raw, _ := json.Marshal(m)
	if err := store.Commit(ctx, before, jobedit.Change{Checkpoint: raw}); err != nil {
		t.Fatal(err)
	}
	edits := &jobedit.Service{Store: store}
	before, err = edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	m, err = images.Review(m, "scene", "approve", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err = images.Review(m, "cover", "approve", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(m)
	if err := store.Commit(ctx, before, jobedit.Change{Checkpoint: raw}); err != nil {
		t.Fatal(err)
	}
	count, err := store.ApprovedToday(ctx, job.ChannelID)
	if err != nil || count != 1 {
		t.Fatalf("approved today = %d, %v; want 1", count, err)
	}
	count, err = store.ApprovedToday(ctx, "")
	if err != nil || count != 1 {
		t.Fatalf("all channels = %d, %v; want 1", count, err)
	}
}

func TestAutomaticImageApprovalIsCounted(t *testing.T) {
	ctx := context.Background()
	store := cloneStore(t)
	job := cloneTestJob(t, store, "auto-approval")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	var claim *pipeline.ClaimedStep
	for {
		var err error
		claim, err = store.ClaimNextStep(ctx, job.ID, "worker")
		if err != nil || claim == nil {
			t.Fatalf("claim: %v %v", claim, err)
		}
		if claim.Step.Step == domain.CloneImages {
			break
		}
		if err := store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, domain.CloneStepOutput{}); err != nil {
			t.Fatal(err)
		}
	}
	n := 1
	m := images.Manifest{Version: 1, Revision: 1, Items: []images.Item{
		{ID: "scene", Kind: "scene", Status: "approved", CurrentVersion: &n},
		{ID: "cover", Kind: "thumbnail", Status: "approved", CurrentVersion: &n},
	}}
	for i := range m.Items {
		m.Items[i].Versions = []images.Version{{Version: 1, SourceFingerprint: images.SourceFingerprint(m.Items[i])}}
	}
	raw, _ := json.Marshal(m)
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, raw); err != nil {
		t.Fatal(err)
	}
	count, err := store.ApprovedToday(ctx, "")
	if err != nil || count != 1 {
		t.Fatalf("automatic approval count = %d, %v; want 1", count, err)
	}
}
