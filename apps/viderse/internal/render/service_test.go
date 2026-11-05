package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

type renderJobRepo struct {
	job       domain.CloneJob
	records   []domain.CloneStepRecord
	artifacts []domain.CloneArtifact
}

func (f renderJobRepo) GetJob(context.Context, string) (*domain.CloneJob, error) {
	copy := f.job
	return &copy, nil
}
func (f renderJobRepo) ListSteps(context.Context, string) ([]domain.CloneStepRecord, error) {
	return f.records, nil
}
func (f renderJobRepo) ListArtifacts(context.Context, string) ([]domain.CloneArtifact, error) {
	return f.artifacts, nil
}
func gateAdapterFixture(t *testing.T) (*Service, *stepstest.FakeMedia, string) {
	t.Helper()
	ctx := context.Background()
	p, err := editor.New("project", "Fixture", 1920, 1080, 30, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := editor.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	files := &stepstest.FakeMedia{Root: t.TempDir()}
	files.Put("editor/projects/project/project.json", raw)
	files.Put("template/source.webp", []byte("pinned template bytes"))
	sum := sha256.Sum256([]byte("pinned template bytes"))
	snapshot := library.Snapshot{Project: raw, Binding: library.TemplateBinding{ProjectID: "template", Media: []library.TemplateMediaBinding{{MediaID: "template-media", ContentHash: "sha256:" + hex.EncodeToString(sum[:])}}}, MediaFiles: map[string]string{"template-media": "template/source.webp"}}
	snap, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	job := stepstest.Job("job")
	job.TemplateSnapshot = snap
	repo := renderJobRepo{job: job, records: []domain.CloneStepRecord{{Step: domain.CloneBuild, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "project"}}}}}
	edit := &editor.Service{Store: editor.Store{Media: files}, ExportOwner: func(context.Context, string) (string, error) { return "job", nil }}
	s := &Service{Store: &Store{Root: t.TempDir()}, Editor: edit, Jobs: repo, Media: files}
	_, revision, err := edit.LoadProject(ctx, "project")
	if err != nil {
		t.Fatal(err)
	}
	return s, files, revision
}
func TestRenderGateAcceptedMissingScoreStillRejectsTemplateDrift(t *testing.T) {
	s, files, revision := gateAdapterFixture(t)
	if err := s.Accept("job", true); err != nil {
		t.Fatal(err)
	}
	files.Put("template/source.webp", []byte("replaced"))
	_, err := s.Begin(context.Background(), "project", revision)
	var failure *editor.Error
	if !errors.As(err, &failure) || failure.Extra["cloneGate"] != "template_media_drift" {
		t.Fatalf("accepted waived template integrity: %v", err)
	}
}
func TestRenderGateAcceptedMissingScoreAllowed(t *testing.T) {
	s, _, revision := gateAdapterFixture(t)
	if err := s.Accept("job", true); err != nil {
		t.Fatal(err)
	}
	policy, err := s.Begin(context.Background(), "project", revision)
	if err != nil {
		t.Fatal(err)
	}
	if policy["accepted"] != true || policy["projectRevision"] != revision || policy["binding"] == nil {
		t.Fatalf("wrong accepted policy: %+v", policy)
	}
}
func TestRenderGateMissingScoreNeedsMeasurementWithoutAcceptance(t *testing.T) {
	s, _, revision := gateAdapterFixture(t)
	_, err := s.Begin(context.Background(), "project", revision)
	var failure *editor.Error
	if !errors.As(err, &failure) || failure.Extra["cloneGate"] != "missing_score" {
		t.Fatalf("missing score did not block: %v", err)
	}
}
func TestRenderApprovalDuplicateReturnsSameIntent(t *testing.T) {
	s, _, _ := gateAdapterFixture(t)
	count := 0
	s.Resume = func(context.Context, string) error { count++; return nil }
	first, err := s.BeginCloneExport(context.Background(), "job", true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.BeginCloneExport(context.Background(), "job", true)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || count != 1 {
		t.Fatalf("duplicate approval reset work: first=%s second=%s transitions=%d", first.ID, second.ID, count)
	}
}
func TestRenderApprovalLeaseRejectionDoesNotLeaveQueuedIntent(t *testing.T) {
	s, _, _ := gateAdapterFixture(t)
	live := errors.New("active lease")
	s.Resume = func(context.Context, string) error { return live }
	_, err := s.BeginCloneExport(context.Background(), "job", true)
	if !errors.Is(err, live) {
		t.Fatal(err)
	}
	requests, err := s.Store.ListExports()
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || !requests[0].Terminal() {
		t.Fatal("lease rejection left a runnable intent")
	}
}
func TestRenderGateRevisionDriftNeverAccepted(t *testing.T) {
	s, files, revision := gateAdapterFixture(t)
	if err := s.Accept("job", true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(files.Root + "/editor/projects/project/project.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := editor.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	p.Metadata()["name"] = "edited"
	raw, err = editor.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	files.Put("editor/projects/project/project.json", raw)
	_, err = s.Begin(context.Background(), "project", revision)
	var failure *editor.Error
	if !errors.As(err, &failure) || failure.Extra["cloneGate"] != "revision_mismatch" {
		t.Fatalf("accepted waived render revision: %v", err)
	}
}
