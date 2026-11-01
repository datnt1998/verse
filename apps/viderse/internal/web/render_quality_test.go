package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
)

// qualityJobs là repo job tối thiểu để route kiểm tra video có thật.
type qualityJobs struct{ job *domain.CloneJob }

func (j qualityJobs) GetJob(context.Context, string) (*domain.CloneJob, error) { return j.job, nil }
func (j qualityJobs) ListSteps(context.Context, string) ([]domain.CloneStepRecord, error) {
	return nil, nil
}

// Nút "Vẫn xuất" của đồng hồ Độ hoàn thiện đặt lại/đặt ý định chủ duyệt:
// POST /api/clone/:id/quality {accept}. Trước đây client gọi action không tồn tại.
func TestCloneQualityRouteAcceptsAndPutsTheChecklistBack(t *testing.T) {
	const jobID = "47c2bbb3-af91-4de4-aebe-2b211c6ed2f2"
	store := &render.Store{Root: t.TempDir()}
	svc := &render.Service{Store: store, Jobs: qualityJobs{job: &domain.CloneJob{ID: jobID}}}
	tg := newTestGateway(t)
	MountClone(tg.g, nil, nil, nil)
	MountRender(tg.g, svc, context.Background())
	cookie := tg.sessionCookie()

	w := tg.do(http.MethodPost, "/api/clone/"+jobID+"/quality", cookie, `{"accept":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Accepted bool `json:"accepted"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !body.Accepted {
		t.Fatalf("body = %s (%v)", w.Body.String(), err)
	}
	state, err := svc.State(jobID)
	if err != nil || state.AcceptedAt == nil {
		t.Fatalf("acceptedAt = %+v (%v)", state.AcceptedAt, err)
	}

	w = tg.do(http.MethodPost, "/api/clone/"+jobID+"/quality", cookie, `{"accept":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if state, err = svc.State(jobID); err != nil || state.AcceptedAt != nil {
		t.Fatalf("acceptedAt = %+v (%v)", state.AcceptedAt, err)
	}

	// Body sai kiểu và video lạ đều bị từ chối, không ghi trạng thái.
	w = tg.do(http.MethodPost, "/api/clone/"+jobID+"/quality", cookie, `{"accept":"yes"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	w = tg.do(http.MethodPost, "/api/clone/"+jobID+"/quality", cookie, `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

func TestCloneQualityRouteRejectsAnUnknownVideo(t *testing.T) {
	const jobID = "47c2bbb3-af91-4de4-aebe-2b211c6ed2f2"
	tg := newTestGateway(t)
	MountClone(tg.g, nil, nil, nil)
	MountRender(tg.g, &render.Service{Store: &render.Store{Root: t.TempDir()}, Jobs: qualityJobs{}}, context.Background())
	w := tg.do(http.MethodPost, "/api/clone/"+jobID+"/quality", tg.sessionCookie(), `{"accept":true}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}
