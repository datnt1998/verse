package render

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestRenderRecoveredExportRequeuesInterruptedPipeline(t *testing.T) {
	s, _, _ := gateAdapterFixture(t)
	repo := s.Jobs.(renderJobRepo)
	repo.job.Status = domain.CloneFailed
	s.Jobs = repo
	request, err := s.Store.CreateExport("project", "job")
	if err != nil {
		t.Fatal(err)
	}
	s.Supervisor = &Supervisor{Store: s.Store, ProcessOps: &supervisorOps{groups: map[int]string{}}}
	prepared := 0
	s.Resume = func(context.Context, string) error { prepared++; return nil }
	if err = s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	replay, err := s.Store.GetExport(request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prepared != 1 || replay.Status != "queued" || replay.ID != request.ID || replay.Nonce == request.Nonce {
		t.Fatalf("request not replayed through score/export: prepared=%d request=%+v", prepared, replay)
	}
}

func TestRenderDuplicateInterruptedIntentRequeuesSameRequest(t *testing.T) {
	s, _, _ := gateAdapterFixture(t)
	repo := s.Jobs.(renderJobRepo)
	repo.job.Status = domain.CloneFailed
	s.Jobs = repo
	request, err := s.Store.CreateExport("project", "job")
	if err != nil {
		t.Fatal(err)
	}
	prepared := 0
	s.Resume = func(context.Context, string) error { prepared++; return nil }
	replay, err := s.BeginCloneExport(context.Background(), "job", true)
	if err != nil {
		t.Fatal(err)
	}
	if replay.ID != request.ID || prepared != 1 {
		t.Fatalf("interrupted intent stuck queued: id=%s prepared=%d", replay.ID, prepared)
	}
}
