package web

import (
	"fmt"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/revision"
)

type failingMobileRepairHost struct {
	*mobileCommandHost
	fail bool
}

func (h *failingMobileRepairHost) Continue(text, username string) error {
	if h.fail {
		return fmt.Errorf("Chưa gửi được cho Người viết")
	}
	return h.mobileCommandHost.Continue(text, username)
}

func TestMobileSyncRepairFailureRemainsRetryable(t *testing.T) {
	h := &failingMobileRepairHost{mobileCommandHost: &mobileCommandHost{newLiveFakeHost()}, fail: true}
	s := newLiveServer(h)
	result := syncResult(&revision.Result{Applied: []int{3}, Analyses: []domain.RevisionAnalysis{{DownstreamIssues: []string{"Chương 14 còn mâu thuẫn"}}}}, h)
	s.emitJob(jobEvent{Job: "sync/2", Kind: "sync", Done: true, Stage: "done", Sync: result})
	w := doReq(t, s, "POST", "/api/sync/fix", `{"chapter":3}`)
	if w.Code != 409 {
		t.Fatalf("failed delivery must be visible: %d %s", w.Code, w.Body.String())
	}
	w = doReq(t, s, "GET", "/api/jobs/sync/2", "")
	if strings.Contains(w.Body.String(), `"sent":true`) {
		t.Fatalf("failed delivery cannot claim sent: %s", w.Body.String())
	}
	h.fail = false
	w = doReq(t, s, "POST", "/api/sync/fix", `{"chapter":3}`)
	if w.Code != 200 || callsJoined(h.liveFakeHost) != "continue" {
		t.Fatalf("retry did not deliver: %d %s", w.Code, w.Body.String())
	}
}
