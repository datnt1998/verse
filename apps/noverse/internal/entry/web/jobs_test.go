package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/diag"
	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/webauth"
)

type testJobsProvider struct{ jobs []Job }

func (p *testJobsProvider) Kind() string                                    { return "audio" }
func (p *testJobsProvider) Jobs() []Job                                     { return p.jobs }
func (p *testJobsProvider) Watch(ctx context.Context, emit func(JobChange)) { <-ctx.Done() }
func (p *testJobsProvider) Control(context.Context, string, string) error   { return nil }

func TestJobsAggregateSourcesAndFilterPermissions(t *testing.T) {
	hub := newJobsHub(newEventHub())
	defer hub.close()
	hub.addProvider(&testJobsProvider{jobs: []Job{{ID: "p/visible/audio/1", Kind: "audio", Project: "p", Story: "visible", Status: JobRunning}}})
	hub.resetStory("p", "visible", []Job{{ID: "p/visible/write/1", Kind: "write", Project: "p", Story: "visible", Status: JobRunning}})
	hub.resetStory("hidden", "secret", []Job{{ID: "hidden/secret/import/1", Kind: "import", Project: "hidden", Story: "secret", Status: JobWaiting}})
	reader := webauth.User{Username: "r", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"p": webauth.ProjectView}}
	jobs := hub.list(reader)
	if len(jobs) != 2 {
		t.Fatalf("expected readable worker + provider jobs, got %+v", jobs)
	}
	for _, job := range jobs {
		if job.Story != "visible" {
			t.Fatalf("permission leak: %+v", job)
		}
	}
	if got := len(hub.list(webauth.User{Role: webauth.RoleAdmin})); got != 3 {
		t.Fatalf("admin aggregation: %d", got)
	}
}

func TestJobsDoneRetentionAndGatewayOwnership(t *testing.T) {
	hub := newJobsHub(newEventHub())
	defer hub.close()
	now := time.Now()
	hub.now = func() time.Time { return now }
	finished := now.Add(-9 * time.Minute)
	job := Job{ID: "p/s/export/900", Kind: "export", Project: "p", Story: "s", Status: JobDoneS, FinishedAt: &finished}
	hub.controls = map[string]func(context.Context, string) error{job.ID: func(context.Context, string) error { return nil }}
	hub.apply(JobChange{Job: job})
	hub.resetStory("p", "s", nil)
	admin := webauth.User{Role: webauth.RoleAdmin}
	if got := len(hub.list(admin)); got != 1 {
		t.Fatalf("worker reset removed gateway result: %d", got)
	}
	now = now.Add(2 * time.Minute)
	if got := len(hub.list(admin)); got != 0 {
		t.Fatalf("expired result retained: %d", got)
	}
	if len(hub.controls) != 0 {
		t.Fatal("expired job control retained")
	}
}

func TestGlobalEventsReplayFiltersPermissions(t *testing.T) {
	hub := newEventHub()
	user := webauth.User{Username: "r", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"p": webauth.ProjectView}}
	first := hub.Publish(Event{Type: "job.created", Project: "p", Story: "s", Data: map[string]string{"id": "first"}})
	hub.Publish(Event{Type: "job.created", Project: "hidden", Story: "secret", Data: map[string]string{"id": "secret"}})
	hub.Publish(Event{Type: "job.done", Project: "p", Story: "s", Data: map[string]string{"id": "visible"}})
	sub, replay, stale := hub.subscribe(user, first, []string{"hidden/secret", "p/s"})
	defer hub.unsubscribe(sub)
	if stale || len(replay) != 1 || replay[0].Type != "job.done" {
		t.Fatalf("replay = %+v, stale=%v", replay, stale)
	}
	if hub.Watched()["hidden/secret"] {
		t.Fatal("unauthorized watch key retained")
	}
	for range eventRing + 1 {
		hub.Publish(Event{Type: "story.changed", Project: "p", Story: "s", Data: map[string]any{}})
	}
	newer, _, stale := hub.subscribe(user, first, nil)
	defer hub.unsubscribe(newer)
	if !stale {
		t.Fatal("replay cursor older than ring must reset")
	}
	admin := webauth.User{Username: "admin", Role: webauth.RoleAdmin}
	if !(Event{User: "r", AdminsToo: true}).visibleTo(admin) {
		t.Fatal("account event hidden from administrator")
	}
	if (Event{User: "other"}).visibleTo(user) {
		t.Fatal("private account event leaked")
	}
}

func TestGlobalEventsLastEventIDHeader(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	hub := tg.g.events()
	first := hub.Publish(Event{Type: "job.created", Project: "p", Story: "s", Data: map[string]string{"id": "first"}})
	hub.Publish(Event{Type: "job.done", Project: "p", Story: "s", Data: map[string]string{"id": "replayed"}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	request.Header.Set("Last-Event-ID", "1")
	if first != 1 {
		t.Fatalf("unexpected first id %d", first)
	}
	writer := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	done := make(chan struct{})
	go func() { tg.g.serveEvents(writer, request, webauth.User{Role: webauth.RoleAdmin}); close(done) }()
	body := writer.until(t, `"id":"replayed"`)
	if strings.Contains(body, `"id":"first"`) {
		t.Fatalf("replayed acknowledged event: %s", body)
	}
	if !strings.Contains(body, "id: 2\nevent: job.done") {
		t.Fatalf("missing replay frame id: %s", body)
	}
	cancel()
	<-done
}

func TestWorkerSyncCheckExposesDeltasAndJob(t *testing.T) {
	h := newLiveFakeHost()
	h.changes = []host.RevisionChange{{Chapter: 2, Title: "Hai", Added: 3, Removed: 1, Kind: "edit"}}
	s := newWorkerServer(h, "secret", nil, newBroadcaster())
	response := doReq(t, s, "POST", "/api/command", `{"line":"/sync --check"}`)
	if response.Code != 200 {
		t.Fatalf("sync check: %d %s", response.Code, response.Body.String())
	}
	result := decodeLiveBody(t, response)["result"].(map[string]any)
	if result["kind"] != "syncCheck" {
		t.Fatalf("wrong command outcome: %+v", result)
	}
	changes := result["changed"].([]any)
	change := changes[0].(map[string]any)
	if change["added"] != float64(3) || change["removed"] != float64(1) {
		t.Fatalf("missing dry-run deltas: %+v", change)
	}
	jobs := s.jobs.listJobs()
	if len(jobs) != 1 || jobs[0].Kind != "sync" || jobs[0].Status != JobDoneS {
		t.Fatalf("dry-run missing dock job: %+v", jobs)
	}
	if len(s.jobs.replays()) != 1 {
		t.Fatal("sync check result not replayable")
	}
}

func TestWorkerWritingPauseUsesStepBoundary(t *testing.T) {
	h := newLiveFakeHost()
	h.snapshot = host.UISnapshot{IsRunning: true, TotalChapters: 5}
	s := newWorkerServer(h, "secret", nil, newBroadcaster())
	s.snapshot()
	jobs := s.jobs.listJobs()
	if len(jobs) != 1 {
		t.Fatalf("missing write job %+v", jobs)
	}
	response := doReq(t, s, "POST", "/api/jobs/"+jobs[0].ID+"/pause", `{}`)
	if response.Code != 200 || !strings.Contains(callsJoined(h), "pauseAfterStep") {
		t.Fatalf("write pause was not graceful: %d %s", response.Code, callsJoined(h))
	}
	if s.jobs.listJobs()[0].Status != JobRunning {
		t.Fatal("pause request reported paused before boundary")
	}
	response = doReq(t, s, "POST", "/api/abort", `{"afterStep":true}`)
	if response.Code != 200 {
		t.Fatalf("graceful abort: %d", response.Code)
	}
}

func TestDismissedProviderResultDoesNotReappear(t *testing.T) {
	hub := newJobsHub(newEventHub())
	defer hub.close()
	job := Job{ID: "p/s/audio/1", Kind: "audio", Status: JobDoneS, Project: "p", Story: "s"}
	hub.apply(JobChange{Job: job})
	hub.dismiss(job)
	hub.apply(JobChange{Job: job})
	if _, ok := hub.get(job.ID); ok {
		t.Fatal("provider refresh resurrected a dismissed result")
	}
	job.Status = JobRunning
	hub.apply(JobChange{Job: job})
	if _, ok := hub.get(job.ID); !ok {
		t.Fatal("dismissal hid a new run from the same source")
	}
}

func TestDiagnosisActionIsIdempotentAndCachedReportTracksSent(t *testing.T) {
	h := newLiveFakeHost()
	h.report = diag.Report{Actions: []diag.Action{{Fingerprint: "finding", Message: "Kiểm tra đại cương", Summary: "Kiểm tra", Kind: diag.ActionEnqueueFollowUp}}}
	s := newWorkerServer(h, "secret", nil, newBroadcaster())
	if response := doReq(t, s, "GET", "/api/diag/last", ""); response.Code != 404 {
		t.Fatal("missing report should return 404")
	}
	if response := doReq(t, s, "GET", "/api/diag", ""); response.Code != 200 {
		t.Fatalf("diagnose: %s", response.Body.String())
	}
	for range 2 {
		response := doReq(t, s, "POST", "/api/diag/actions/finding", `{}`)
		if response.Code != 200 || decodeLiveBody(t, response)["action"] != "continue" {
			t.Fatalf("send suggested action: %s", response.Body.String())
		}
	}
	if got := strings.Count(callsJoined(h), "continue"); got != 1 {
		t.Fatalf("suggestion submitted %d times", got)
	}
	response := doReq(t, s, "GET", "/api/diag/last", "")
	actions := decodeLiveBody(t, response)["actions"].([]any)
	if actions[0].(map[string]any)["sent"] != true {
		t.Fatal("cached report lost sent state")
	}
}
