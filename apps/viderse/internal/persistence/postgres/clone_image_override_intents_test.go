package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

func TestLivePromptOverrideSurvivesRestartAndFencesNewFlights(t *testing.T) {
	ctx := context.Background()
	store := cloneStore(t)
	job := cloneTestJob(t, store, "live-override")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil { t.Fatal(err) }
	var claim *pipeline.ClaimedStep
	for {
		var err error
		claim, err = store.ClaimNextStep(ctx, job.ID, "worker")
		if err != nil || claim == nil { t.Fatalf("claim: %v %v", claim, err) }
		if claim.Step.Step == domain.CloneImages { break }
		if err := store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, domain.CloneStepOutput{}); err != nil { t.Fatal(err) }
	}
	m := images.Manifest{Version: 1, Revision: 3, TemplatePrompt: "old"}
	for i := range 6 {
		item := images.Item{ID: fmt.Sprintf("scene-%d", i), Kind: "scene", Status: "pending"}
		if i < 4 { item.Status = "review"; item.Versions = []images.Version{{Version: 1, Prompt: "old"}} }
		if i == 4 { item.Status = "running"; item.InFlight = &images.Flight{GenerationID:"already-running"} }
		m.Items = append(m.Items, item)
	}
	raw, _ := json.Marshal(m)
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, raw); err != nil { t.Fatal(err) }
	edits := &jobedit.Service{Store: store}
	before, err := edits.Read(ctx, job.ID, domain.CloneImages); if err != nil { t.Fatal(err) }
	desired := m
	desired.TemplatePrompt = "new visual style"
	desired.QueueOnly = []string{"scene-0", "scene-1", "scene-2", "scene-3", "scene-4"}
	desired.Revision++
	changed, _ := json.Marshal(desired)
	if err := store.Commit(ctx, before, jobedit.Change{Checkpoint:changed, InvalidateFrom:domain.CloneImages, Rerun:true, ImageOverride:true}); err != nil { t.Fatal(err) }
	visible, err := edits.Read(ctx, job.ID, domain.CloneImages); if err != nil { t.Fatal(err) }
	var projected images.Manifest
	if err := json.Unmarshal(visible.Manifest, &projected); err != nil { t.Fatal(err) }
	if projected.Revision != 4 || projected.TemplatePrompt != "new visual style" || projected.Items[4].InFlight == nil { t.Fatalf("bad projected intent: %+v", projected) }
	m.Items[5].Status = "running"
	m.Items[5].InFlight = &images.Flight{GenerationID:"must-not-start"}
	raw, _ = json.Marshal(m)
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, raw); !errors.Is(err, pipeline.ErrImageOverridePending) { t.Fatalf("new old-prompt flight must be fenced: %v", err) }
	m.Items[5].Status = "pending"
	m.Items[5].InFlight = nil
	m.Items[4].Status = "review"
	m.Items[4].InFlight = nil
	m.Items[4].Versions = []images.Version{{Version:1, Prompt:"old"}}
	m.Revision++
	raw, _ = json.Marshal(m)
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, raw); err != nil { t.Fatal(err) }
	if err := store.ApplyStepSuccess(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, domain.CloneStepOutput{}); err != nil { t.Fatal(err) }
	restarted, err := store.ClaimNextStep(ctx, job.ID, "worker"); if err != nil || restarted == nil || restarted.Step.Step != domain.CloneImages { t.Fatalf("restart: %+v %v", restarted, err) }
	merged, err := store.DrainImageIntents(ctx, job.ID, restarted.Fence, restarted.Step.Attempt, restarted.Step.Checkpoint); if err != nil { t.Fatal(err) }
	if err := json.Unmarshal(merged, &m); err != nil { t.Fatal(err) }
	if m.TemplatePrompt != "new visual style" || m.Items[4].Status != "pending" || len(m.Items[4].Versions) != 1 || m.Items[4].Versions[0].Prompt != "old" || m.Items[5].Status != "pending" { t.Fatalf("lost active result or pending work: %+v", m) }
	for i := range 4 {
		if m.Items[i].Status != "pending" || len(m.Items[i].Versions) != 1 || m.Items[i].Versions[0].Prompt != "old" { t.Fatalf("wrong redraw %d: %+v", i, m.Items[i]) }
	}
}
