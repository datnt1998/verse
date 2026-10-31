package web

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestAllChannelBoardTrimsQueueAndSelectsOldestAttention(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	jobs := []domain.CloneJobView{
		{CloneJob: domain.CloneJob{ID: "review", StageGroup: domain.StageReview, CreatedAt: now.Add(-3 * time.Hour)}},
		{CloneJob: domain.CloneJob{ID: "attention-new", StageGroup: domain.StageAttention, CreatedAt: now}},
		{CloneJob: domain.CloneJob{ID: "attention-old", StageGroup: domain.StageAttention, CreatedAt: now.Add(-time.Hour)}},
		{CloneJob: domain.CloneJob{ID: "posted", StageGroup: domain.StagePosted}},
	}
	for _, id := range []string{"q1", "q2", "q3", "q4"} {
		jobs = append(jobs, domain.CloneJobView{CloneJob: domain.CloneJob{ID: id, StageGroup: domain.StageQueued}})
	}
	svc := &fakeCloneService{list: domain.CloneJobList{Jobs: jobs, QueuedTotal: 4}}
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
	w := tg.do(http.MethodGet, "/api/clone?channel=all", cookie, "")
	var out domain.CloneJobList
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != http.StatusOK {
		t.Fatalf("response %d: %s, %v", w.Code, w.Body.String(), err)
	}
	if out.DefaultJobID == nil || *out.DefaultJobID != "attention-old" || out.QueuedTotal != 4 || len(out.Jobs) != 6 {
		t.Fatalf("wrong board: %+v", out)
	}
	for _, job := range out.Jobs {
		if job.ID == "posted" || job.ID == "q4" {
			t.Fatalf("unexpected board item %s", job.ID)
		}
	}
}
