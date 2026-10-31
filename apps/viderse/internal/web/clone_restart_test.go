package web

import (
	"context"
	"encoding/json"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"net/http"
	"testing"
)

func (f *fakeCloneService) Restart(ctx context.Context, id string) (*domain.CloneQueueItem, error) {
	f.record(ctx, "restart", id)
	if f.err != nil {
		return nil, f.err
	}
	if f.job == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrNotFound, Message: "missing"}
	}
	return &domain.CloneQueueItem{ID: f.job.ID, ChannelID: f.job.ChannelID, State: domain.QueueQueued}, nil
}

func TestCloneRestartUnknownJob(t *testing.T) {
	svc := &fakeCloneService{}
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
	w := tg.do(http.MethodPost, "/api/clone/missing", cookie, `{"action":"restart"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("restart unknown: %d %s", w.Code, w.Body.String())
	}
}

func TestCloneRestartResponseAndConflict(t *testing.T) {
	svc := &fakeCloneService{job: &domain.CloneJob{ID: "new-job", SourceKey: "source", ChannelID: "owner", Generation: 2, Status: domain.ClonePending}}
	tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
	w := tg.do(http.MethodPost, "/api/clone/old-job", cookie, `{"action":"restart"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("restart: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		RestartedFrom string `json:"restartedFrom"`
		QueueItem     struct {
			ID    string `json:"id"`
			Owner string `json:"channelId"`
		} `json:"queueItem"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.RestartedFrom != "old-job" || body.QueueItem.ID != "new-job" || body.QueueItem.Owner != "owner" {
		t.Fatalf("response %+v: %v", body, err)
	}
	svc.err = &domain.CloneError{Code: domain.CloneErrConflict, Message: "running"}
	w = tg.do(http.MethodPost, "/api/clone/old-job", cookie, `{"action":"restart"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("conflict: %d %s", w.Code, w.Body.String())
	}
}
