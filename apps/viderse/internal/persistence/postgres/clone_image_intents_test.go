package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

func TestImageIntentSurvivesCompletionAndMergesWorkerResult(t *testing.T) {
	ctx := context.Background()
	store := cloneStore(t)
	job := cloneTestJob(t, store, "image-intents")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil { t.Fatal(err) }
	var claim *pipeline.ClaimedStep
	for {
		var err error
		claim, err = store.ClaimNextStep(ctx, job.ID, "worker")
		if err != nil || claim == nil { t.Fatalf("claim: %v %v", claim, err) }
		if claim.Step.Step == domain.CloneImages { break }
		if err := store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, domain.CloneStepOutput{}); err != nil { t.Fatal(err) }
	}
	m := images.Manifest{Version: 1, Revision: 1, Items: []images.Item{
		{ID: "a", Kind: "scene", Status: "running", InFlight: &images.Flight{GenerationID: "a"}},
		{ID: "b", Kind: "scene", Status: "review"},
	}}
	raw, _ := json.Marshal(m)
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, raw); err != nil { t.Fatal(err) }
	edits := &jobedit.Service{Store: store}
	before, err := edits.Read(ctx, job.ID, domain.CloneImages); if err != nil { t.Fatal(err) }
	next, err := images.Review(m, "b", "regenerate", "warmer", nil); if err != nil { t.Fatal(err) }
	next.QueueOnly = []string{"b"}
	change, _ := json.Marshal(next)
	if err = store.Commit(ctx, before, jobedit.Change{Checkpoint: change, InvalidateFrom: domain.CloneImages, Rerun: true, ImageReview: true, ImageIDs: []string{"b"}}); err != nil { t.Fatal(err) }
	visible, err := edits.Read(ctx, job.ID, domain.CloneImages); if err != nil { t.Fatal(err) }
	var overlay images.Manifest
	if err = json.Unmarshal(visible.Manifest, &overlay); err != nil { t.Fatal(err) }
	if overlay.Revision != 2 || overlay.Items[1].Direction != "warmer" || overlay.Items[1].Status != "pending" { t.Fatalf("intent not visible: %+v", overlay) }
	_, err = images.Review(overlay, "b", "regenerate", "again", nil)
	var ce *domain.CloneError
	if !errors.As(err, &ce) || ce.Code != domain.CloneErrConflict { t.Fatalf("duplicate intent: %v", err) }
	// Complete A after B's intent was accepted. Completion must retain both
	// the checkpoint and the intent for a restarted worker.
	m.Items[0].Status = "review"
	m.Items[0].InFlight = nil
	m.Items[0].Versions = []images.Version{{Version: 1, GenerationID: "a"}}
	m.Revision++
	raw, _ = json.Marshal(m)
	if err = store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, raw); err != nil { t.Fatal(err) }
	if err = store.ApplyStepSuccess(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, domain.CloneStepOutput{}); err != nil { t.Fatal(err) }
	restarted, err := store.ClaimNextStep(ctx, job.ID, "worker"); if err != nil || restarted == nil || restarted.Step.Step != domain.CloneImages { t.Fatalf("restart: %+v %v", restarted, err) }
	merged, err := store.DrainImageIntents(ctx, job.ID, restarted.Fence, restarted.Step.Attempt, restarted.Step.Checkpoint); if err != nil { t.Fatal(err) }
	if err = json.Unmarshal(merged, &m); err != nil { t.Fatal(err) }
	if len(m.Items[0].Versions) != 1 || m.Items[1].Status != "pending" || m.Items[1].Direction != "warmer" { t.Fatalf("lost worker result or intent: %+v", m) }
}
