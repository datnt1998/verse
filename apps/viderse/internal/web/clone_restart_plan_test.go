package web

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"net/http"
	"strings"
	"testing"
)

func (f *fakeCloneService) RestartPlan(ctx context.Context, id string) (*domain.CloneRestartPlan, error) {
	f.record(ctx, "restartPlan", id)
	if f.job == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrNotFound, Message: "missing"}
	}
	return &domain.CloneRestartPlan{Template: domain.ClonePlanTemplate{ID: "next", Name: "Next"}, Position: "front"}, nil
}

func TestCloneRestartPlanUnknown(t *testing.T) {
	svc := &fakeCloneService{}
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
	w := tg.do(http.MethodPost, "/api/clone/missing", cookie, `{"action":"restart","dryRun":true}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("preview unknown: %d %s", w.Code, w.Body.String())
	}
}

func TestCloneRestartPlanReturnsPreview(t *testing.T) {
	svc := &fakeCloneService{job: &domain.CloneJob{ID: "old"}}
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
	w := tg.do(http.MethodPost, "/api/clone/old", cookie, `{"action":"restart","dryRun":true}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"position":"front"`) || !strings.Contains(w.Body.String(), `"imageEstimate":null`) || !strings.Contains(w.Body.String(), `"etaStart":null`) {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
}
