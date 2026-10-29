package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/host"
)

func (s *workerServer) getReopenChecks(w http.ResponseWriter) {
	provider, ok := s.h.(interface {
		ReopenPreconditions() (host.ReopenChecks, error)
	})
	if !ok {
		jsonError(w, 500, "Không đọc được điều kiện mở lại truyện")
		return
	}
	checks, err := provider.ReopenPreconditions()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, checks)
}

var syncChapterMention = regexp.MustCompile(`(?i)(?:chương|chapter|ch\.)\s*(\d+)`)

func syncAnalysis(chapter int, title string, a domain.RevisionAnalysis, titles map[int]string) map[string]any {
	issues := []map[string]any{}
	for i, text := range a.DownstreamIssues {
		var target any
		if match := syncChapterMention.FindStringSubmatch(text); len(match) > 1 {
			n, _ := strconv.Atoi(match[1])
			if _, ok := titles[n]; ok {
				target = n
			}
		}
		issues = append(issues, map[string]any{"text": text, "chapter": target, "issueIndex": i, "sent": false})
	}
	classification := "wording"
	if a.StoryChanged {
		classification = "facts"
	}
	if len(issues) > 0 {
		classification = "conflicts"
	}
	return map[string]any{"schemaVersion": 1, "chapter": chapter, "title": title, "changeSummary": a.ChangeSummary, "storyChanged": a.StoryChanged, "classification": classification, "conflictCount": len(issues), "downstreamIssues": issues}
}

// postSyncFix accepts only an issue from an actual retained sync analysis.
// Repeated requests queue it once; failed delivery remains retryable.
func (s *workerServer) postSyncFix(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Chapter    int    `json:"chapter"`
		IssueIndex int    `json:"issueIndex"`
		Job        string `json:"job"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Chapter < 1 || req.IssueIndex < 0 {
		jsonError(w, 400, "Chương hoặc mâu thuẫn không hợp lệ")
		return
	}
	// syncFixMu covers only reading/marking the retained issue, never Arbiter work.
	s.syncFixMu.Lock()
	ev, issue, status, msg := s.syncIssue(req.Job, req.Chapter, req.IssueIndex)
	if status != 0 {
		s.syncFixMu.Unlock()
		jsonError(w, status, msg)
		return
	}
	action := "continue"
	if s.h.Snapshot().IsRunning {
		action = "steer"
	}
	if issue["sent"] == true {
		s.syncFixMu.Unlock()
		writeJSON(w, 200, map[string]any{"schemaVersion": 1, "ok": true, "action": action, "sent": true})
		return
	}
	s.markSyncIssue(ev.Job, req.Chapter, req.IssueIndex, true)
	s.syncFixMu.Unlock()
	unmark := func() {
		s.syncFixMu.Lock()
		s.markSyncIssue(ev.Job, req.Chapter, req.IssueIndex, false)
		s.syncFixMu.Unlock()
	}
	text := fmt.Sprintf("Nhờ sửa mâu thuẫn phát hiện sau khi tiếp nhận chương %d: %s", req.Chapter, issue["text"])
	pending, err := s.dispatchAction("Đang gửi yêu cầu sửa mâu thuẫn", func() error {
		var err error
		if action == "steer" {
			err = s.h.Steer(text, r.Header.Get("X-Ainovel-Username"))
		} else {
			err = s.h.Continue(text, r.Header.Get("X-Ainovel-Username"))
		}
		if err != nil {
			unmark()
		}
		return err
	})
	if err != nil {
		unmark()
	}
	writeDispatched(w, pending, err, map[string]any{"schemaVersion": 1, "ok": true, "action": action, "sent": true})
}

// syncIssue resolves one downstream issue from a retained sync result.
func (s *workerServer) syncIssue(job string, chapter, issueIndex int) (jobEvent, map[string]any, int, string) {
	s.jobs.mu.Lock()
	ev, ok := s.jobs.last["sync"]
	if job != "" {
		ev, ok = s.jobs.history[job]
	}
	s.jobs.mu.Unlock()
	result, valid := ev.Sync.(map[string]any)
	if !ok || ev.Kind != "sync" || !valid {
		return ev, nil, 404, "Không tìm thấy kết quả đồng bộ"
	}
	analyses, _ := result["analyses"].([]map[string]any)
	for _, a := range analyses {
		if a["chapter"] == chapter {
			issues, _ := a["downstreamIssues"].([]map[string]any)
			if issueIndex < len(issues) {
				return ev, issues[issueIndex], 0, ""
			}
			break
		}
	}
	return ev, nil, 404, "Không tìm thấy mâu thuẫn cần sửa"
}

// markSyncIssue publishes a copied result so in-flight readers keep immutable data.
func (s *workerServer) markSyncIssue(job string, chapter, issueIndex int, sentFlag bool) {
	ev, issue, status, _ := s.syncIssue(job, chapter, issueIndex)
	if status != 0 {
		return
	}
	result := ev.Sync.(map[string]any)
	analyses, _ := result["analyses"].([]map[string]any)
	index := -1
	var issues []map[string]any
	for i, a := range analyses {
		if a["chapter"] == chapter {
			index = i
			issues, _ = a["downstreamIssues"].([]map[string]any)
			break
		}
	}
	next := make(map[string]any, len(result))
	for k, v := range result {
		next[k] = v
	}
	nextAnalyses := append([]map[string]any{}, analyses...)
	analysis := make(map[string]any, len(analyses[index]))
	for k, v := range analyses[index] {
		analysis[k] = v
	}
	nextIssues := append([]map[string]any{}, issues...)
	sent := make(map[string]any, len(issue))
	for k, v := range issue {
		sent[k] = v
	}
	if sentFlag {
		sent["sent"] = true
	} else {
		delete(sent, "sent")
	}
	nextIssues[issueIndex] = sent
	analysis["downstreamIssues"] = nextIssues
	nextAnalyses[index] = analysis
	next["analyses"] = nextAnalyses
	ev.Sync = next
	s.jobs.mu.Lock()
	s.jobs.history[ev.Job] = ev
	if s.jobs.last["sync"].Job == ev.Job {
		s.jobs.last["sync"] = ev
	}
	if previous, exists := s.jobs.entries[ev.Job]; exists && previous.Result != nil {
		job := previous
		updated := *previous.Result
		updated.Data = next
		job.Result = &updated
		s.jobs.entries[ev.Job] = job
		if s.jobs.notify != nil {
			s.jobs.notify(previous, job, true)
		}
	}
	s.jobs.mu.Unlock()
	s.b.emit(message{kind: "job", data: ev})
}

func (s *workerServer) getWorkerJob(w http.ResponseWriter, id string) {
	s.jobs.mu.Lock()
	job, ok := s.jobs.entries[id]
	ev, hasEvent := s.jobs.history[id]
	s.jobs.mu.Unlock()
	if !ok {
		jsonError(w, 404, "Không tìm thấy việc nền")
		return
	}
	var event any
	if hasEvent {
		event = ev
	}
	writeJSON(w, 200, map[string]any{"schemaVersion": 1, "job": job, "event": event})
}

func (g *Gateway) getJobDetail(w http.ResponseWriter, r *http.Request, userCanRead func(string) bool, id string) {
	// IDs are four segments (project/story/kind/sequence); they may be URL encoded.
	segs := strings.Split(id, "/")
	if len(segs) != 4 || !userCanRead(segs[0]+"/"+segs[1]) {
		jsonError(w, 404, "Không tìm thấy việc nền")
		return
	}
	job, ok := g.jobsHub().get(id)
	if !ok {
		jsonError(w, 404, "Không tìm thấy việc nền")
		return
	}
	var event any
	g.mu.Lock()
	slot := g.workers[segs[0]+"/"+segs[1]]
	g.mu.Unlock()
	if slot != nil {
		slot.mu.Lock()
		worker := slot.worker
		slot.mu.Unlock()
		if worker != nil && !workerDead(worker) {
			status, body, err := g.workerCall(r.Context(), worker, http.MethodGet, "jobs/"+strings.Join(segs[2:], "/"), nil)
			if err == nil && status == http.StatusOK {
				var payload struct {
					Event *jobEvent `json:"event"`
				}
				if json.Unmarshal(body, &payload) == nil {
					event = payload.Event
				}
			}
		}
	}
	writeJSON(w, 200, map[string]any{"schemaVersion": 1, "project": job.Project, "story": job.Story, "job": job, "event": event})
}
