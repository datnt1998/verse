package web

import (
 "context"
 "encoding/json"
 "net/http"
 "testing"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

func (f *fakeCloneService) Restart(ctx context.Context, id string) (*pipeline.StartJobResult, error) {
 f.record(ctx, "restart", id)
 if f.err != nil { return nil, f.err }
 if f.job == nil { return nil, &domain.CloneError{Code: domain.CloneErrNotFound, Message:"missing"} }
 return &pipeline.StartJobResult{Job:f.job, Created:true}, nil
}

func TestCloneRestartUnknownJob(t *testing.T) {
 svc := &fakeCloneService{}
 tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
 w := tg.do(http.MethodPost, "/api/clone/missing", cookie, `{"action":"restart"}`)
 if w.Code != http.StatusNotFound { t.Fatalf("restart unknown: %d %s", w.Code, w.Body.String()) }
}

func TestCloneRestartResponseAndConflict(t *testing.T) {
 svc := &fakeCloneService{job: &domain.CloneJob{ID:"new-job", SourceKey:"source", ChannelID:"owner", Generation:2, Status:domain.ClonePending}}
 tg, cookie := cloneFixture(t, svc, &fakeCloneEnqueuer{}, nil)
 w := tg.do(http.MethodPost, "/api/clone/old-job", cookie, `{"action":"restart"}`)
 if w.Code != http.StatusCreated { t.Fatalf("restart: %d %s", w.Code,w.Body.String()) }
 var body struct { RestartedFrom string `json:"restartedFrom"`; Created bool `json:"created"`; Job struct { ID string `json:"id"`; Generation int `json:"generation"`; Owner string `json:"ownerChannelId"` } `json:"job"` }
 if err:=json.Unmarshal(w.Body.Bytes(),&body); err!=nil || body.RestartedFrom!="old-job" || !body.Created || body.Job.ID!="new-job" || body.Job.Generation!=2 || body.Job.Owner!="owner" { t.Fatalf("response %+v: %v",body,err) }
 svc.err = &domain.CloneError{Code:domain.CloneErrConflict, Message:"running"}
 w = tg.do(http.MethodPost, "/api/clone/old-job", cookie, `{"action":"restart"}`)
 if w.Code != http.StatusConflict { t.Fatalf("conflict: %d %s",w.Code,w.Body.String()) }
}
