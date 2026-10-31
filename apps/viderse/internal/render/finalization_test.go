package render

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestExportStateWaitsForMatchingPipelineFinalization(t *testing.T) {
	request := ExportRequest{ID: "request", JobID: "job", ProjectID: "project", Status: "done", ExportID: "new-export", FinishedAt: "browser-done"}
	repo := renderJobRepo{job: domain.CloneJob{Status: domain.CloneRunning}, records: []domain.CloneStepRecord{{Step: domain.CloneExport, Status: domain.CloneRunning}}}
	service := &Service{Jobs: repo}
	state, err := service.ExportState(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "running" || state.Progress == nil || state.Progress.Phase != "compressing" || state.FinishedAt != "" {
		t.Fatalf("browser upload was exposed as final output: %+v", state)
	}
	for _, id := range []string{"old-export", "new-export"} {
		repo.records[0].Status = domain.CloneSucceeded
		repo.records[0].Output = &domain.CloneStepOutput{Data: map[string]any{"exportId": id}}
		service.Jobs = repo
		state, err = service.ExportState(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if (state.Status == "done") != (id == request.ExportID) {
			t.Fatalf("output binding %s yielded%s", id, state.Status)
		}
	}
}

func TestExportStateFinalizationFailureIsTerminal(t *testing.T) {
	service := &Service{Jobs: renderJobRepo{job: domain.CloneJob{Status: domain.CloneFailed}, records: []domain.CloneStepRecord{{Step: domain.CloneExport, Status: domain.CloneFailed}}}}
	request := ExportRequest{JobID: "job", Status: "done", ExportID: "export"}
	state, err := service.ExportState(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "failed" || state.Error == nil {
		t.Fatalf("failed pipeline exposed ready MP4: %+v", state)
	}
}
