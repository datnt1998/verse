package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/revision"
)

type mobileCommandHost struct{ *liveFakeHost }

func (h *mobileCommandHost) Chapters() ([]host.ChapterInfo, error) {
	return []host.ChapterInfo{{Chapter: 3, Title: "Ba"}, {Chapter: 14, Title: "Mười bốn"}}, nil
}
func (h *mobileCommandHost) ReopenPreconditions() (host.ReopenChecks, error) {
	return host.ReopenChecks{SchemaVersion: 1, Title: "Truyện", Chapters: 30, Allowed: false, Checks: []host.ReopenCheck{{ID: "clean", OK: false, Label: "28/30 chương đã duyệt sạch", Chapters: []int{28, 29}}}}, nil
}

func TestMobileSyncClassificationLinksAndRepair(t *testing.T) {
	h := &mobileCommandHost{newLiveFakeHost()}
	s := newLiveServer(h)
	res := &revision.Result{Applied: []int{3, 14, 20}, Analyses: []domain.RevisionAnalysis{
		{StoryChanged: true, ChangeSummary: "Đổi sự thật", DownstreamIssues: []string{"Chương 14 chưa biết sự thật", "Chương 999 không tồn tại"}},
		{StoryChanged: true, ChangeSummary: "Sự thật mới"},
		{ChangeSummary: "Chỉ đổi lời văn"},
	}}
	result := syncResult(res, h)
	analyses := result["analyses"].([]map[string]any)
	if analyses[0]["classification"] != "conflicts" || analyses[1]["classification"] != "facts" || analyses[2]["classification"] != "wording" || analyses[0]["conflictCount"] != 2 {
		t.Fatalf("incorrect classification: %v", analyses)
	}
	issues := analyses[0]["downstreamIssues"].([]map[string]any)
	if issues[0]["chapter"] != 14 || issues[1]["chapter"] != nil {
		t.Fatalf("issue links must refer to real named chapters: %v", issues)
	}
	s.emitJob(jobEvent{Job: "sync/1", Kind: "sync", Done: true, Stage: "done", Sync: result})
	for range 2 {
		w := doReq(t, s, "POST", "/api/sync/fix", `{"chapter":3,"issueIndex":0,"job":"sync/1"}`)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"sent":true`) {
			t.Fatalf("repair: %d %s", w.Code, w.Body.String())
		}
	}
	if strings.Count(callsJoined(h.liveFakeHost), "continue") != 1 {
		t.Fatalf("repair must queue once: %s", callsJoined(h.liveFakeHost))
	}
	w := doReq(t, s, "GET", "/api/jobs/sync/1", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"event"`) || !strings.Contains(w.Body.String(), `"sent":true`) {
		t.Fatalf("repair must survive result refresh: %s", w.Body.String())
	}
	if w = doReq(t, s, "POST", "/api/sync/fix", `{"chapter":3,"issueIndex":4}`); w.Code != 404 {
		t.Fatalf("fabricated conflict accepted: %s", w.Body.String())
	}
	h.mu.Lock()
	h.snapshot.IsRunning = true
	h.mu.Unlock()
	if w = doReq(t, s, "POST", "/api/sync/fix", `{"chapter":3,"issueIndex":1}`); w.Code != 200 || !strings.Contains(callsJoined(h.liveFakeHost), "steer") {
		t.Fatalf("running repair must steer: %s", w.Body.String())
	}
}

func TestMobileReopenEndpointAndDiagnosisReruns(t *testing.T) {
	h := &mobileCommandHost{newLiveFakeHost()}
	s := newLiveServer(h)
	w := doReq(t, s, "GET", "/api/reopen", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"chapters":[28,29]`) || !strings.Contains(w.Body.String(), `"allowed":false`) {
		t.Fatalf("reopen checks: %s", w.Body.String())
	}
	h.report = diag.Report{Stats: diag.Stats{CompletedChapters: 2}}
	w = doReq(t, s, "GET", "/api/diag", "")
	if !strings.Contains(w.Body.String(), `"completedChapters":2`) {
		t.Fatalf("initial report: %s", w.Body.String())
	}
	h.report = diag.Report{Stats: diag.Stats{CompletedChapters: 3}}
	w = doReq(t, s, "GET", "/api/diag", "")
	if !strings.Contains(w.Body.String(), `"completedChapters":3`) {
		t.Fatalf("Phân tích lại must scan fresh state: %s", w.Body.String())
	}
	if callsJoined(h.liveFakeHost) != "" {
		t.Fatalf("diagnosis/preconditions must not mutate story: %s", callsJoined(h.liveFakeHost))
	}
}

func TestMobileGlobalJobDeepLinkPermissions(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	now := time.Now()
	tg.g.jobsHub().apply(JobChange{Job: Job{ID: "p/s/import/1", Project: "p", Story: "s", Kind: "import", Status: JobDoneS, FinishedAt: &now}})
	w := httptest.NewRecorder()
	tg.g.getJobDetail(w, httptest.NewRequest("GET", "/api/jobs/p/s/import/1", nil), func(id string) bool { return id == "p/s" }, "p/s/import/1")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"project":"p"`) || !strings.Contains(w.Body.String(), `"event":null`) {
		t.Fatalf("deep link lookup: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	tg.g.getJobDetail(w, httptest.NewRequest("GET", "/api/jobs/p/s/import/1", nil), func(string) bool { return false }, "p/s/import/1")
	if w.Code != 404 {
		t.Fatalf("unreadable job leaked: %s", w.Body.String())
	}
}
