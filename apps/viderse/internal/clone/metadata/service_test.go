package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

type metadataFixture struct{ snapshot jobedit.Snapshot }

func (f *metadataFixture) Read(context.Context, string, domain.CloneStep) (jobedit.Snapshot, error) {
	return f.snapshot, nil
}
func (f *metadataFixture) ApproveDraft(context.Context, string, int) error { return nil }
func (f *metadataFixture) Commit(_ context.Context, before jobedit.Snapshot, change jobedit.Change) error {
	if string(before.Step.Checkpoint) != string(f.snapshot.Step.Checkpoint) {
		return conflict()
	}
	f.snapshot.Step.Checkpoint = append(json.RawMessage(nil), change.Checkpoint...)
	f.snapshot.Step.Output = &domain.CloneStepOutput{Artifacts: change.Artifacts, Data: map[string]any{}}
	f.snapshot.Artifacts = append(f.snapshot.Artifacts, change.Artifacts...)
	return nil
}
func TestMetadataServicePatchHistoryAndSelectedRewrite(t *testing.T) {
	ctx := context.Background()
	files := &stepstest.FakeMedia{Root: t.TempDir()}
	path := "clone/job/original/metadata/metadata.json"
	files.Put(path, []byte(`{"version":1,"title":"original","description":"exact\ntext","tags":["first","two"]}`))
	artifact := domain.CloneArtifact{Path: path, Step: domain.CloneMetadata}
	store := &metadataFixture{snapshot: jobedit.Snapshot{Job: domain.CloneJob{ID: "job"}, Step: domain.CloneStepRecord{Step: domain.CloneMetadata, Status: domain.CloneSucceeded, Output: &domain.CloneStepOutput{Artifacts: []domain.CloneArtifact{artifact}}}, Artifacts: []domain.CloneArtifact{artifact}}}
	svc := &Service{Edits: &jobedit.Service{Store: store, Media: files}, Generate: func(_ context.Context, _ jobedit.Snapshot, parts []string, feedback string) (Content, error) {
		if !reflect.DeepEqual(parts, []string{"title"}) || feedback != "shorter" {
			t.Fatalf("bad rewrite request: %v %q", parts, feedback)
		}
		return Content{Title: "rewritten", Description: "must not replace", Tags: []string{"different"}}, nil
	}}
	rev := 0
	title := "manual"
	view, err := svc.Patch(ctx, "job", Patch{ExpectedRevision: &rev, Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	if view.Revision != 1 || view.Metadata.Title != "manual" || !view.Metadata.Edited["title"] {
		t.Fatalf("patch: %+v", view)
	}
	hist, err := svc.History(ctx, "job")
	if err != nil || len(hist) != 1 || hist[0].Title != "original" {
		t.Fatalf("history: %+v %v", hist, err)
	}
	_, err = svc.Patch(ctx, "job", Patch{ExpectedRevision: &rev, Title: &title})
	var ce *domain.CloneError
	if !errors.As(err, &ce) || ce.Code != domain.CloneErrConflict {
		t.Fatalf("stale: %v", err)
	}
	rev = 1
	view, err = svc.Rewrite(ctx, "job", Rewrite{ExpectedRevision: &rev, Parts: []string{"title"}, Direction: "shorter"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Metadata.Title != "rewritten" || view.Metadata.Description != "exact\ntext" || !reflect.DeepEqual(view.Metadata.Tags, []string{"first", "two"}) {
		t.Fatalf("rewrite changed unselected parts: %+v", view.Metadata)
	}
}
