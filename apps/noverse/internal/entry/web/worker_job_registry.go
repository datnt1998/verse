package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/revision"
)

var jobTitles = map[string]string{"write": "Sáng tác", "import": "Nhập truyện", "importsim": "Nhập hồ sơ văn phong", "simulate": "Học văn phong", "sync": "Đồng bộ tu chỉnh", "export": "Xuất truyện", "diag": "Chẩn đoán"}

func (t *jobTracker) context(id string) context.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ctx := t.contexts[id]; ctx != nil {
		return ctx
	}
	return context.Background()
}

func (t *jobTracker) remember(id string, retry func() (string, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.retries[id] = retry
}

// recordJobLocked projects legacy story progress into the global dock model.
func (t *jobTracker) recordJobLocked(ev jobEvent) {
	prev, existed := t.entries[ev.Job]
	j := prev
	if !existed {
		kind := ev.Kind
		if kind == "importsim" {
			kind = "simulate"
		}
		j = Job{ID: ev.Job, Kind: kind, Title: jobTitles[ev.Kind], Status: JobRunning, Href: "work", StartedAt: time.Now()}
	}
	if prev.Status == JobPausedS {
		return
	}
	j.Step, j.Error = ev.Message, ev.Error
	normalizeJobError(&j)
	j.Status = JobRunning
	if ev.Total > 0 {
		p := float64(ev.Current) / float64(ev.Total)
		p = max(0, min(1, p))
		j.Progress = &p
		j.Unit = &JobUnit{Done: ev.Current, Total: ev.Total, Label: "Chương"}
	}
	switch {
	case ev.Error != "":
		j.Status = JobFailed
	case ev.Awaiting != "":
		j.Status = JobWaiting
		j.Href = "work?dlg=import"
		j.Result = &JobResult{Title: "Chờ xác nhận nhập truyện", Sub: ev.Message, Actions: []JobAction{{Label: "Xem các chương", Kind: "open", Href: j.Href, Primary: true}}}
		if ev.Import != nil {
			j.Result.Data = ev.Import
		}
	case ev.Done:
		j.Status = JobDoneS
	}
	if ev.Done && ev.Awaiting == "" {
		now := time.Now()
		j.FinishedAt = &now
		j.Result = &JobResult{Title: j.Title, Sub: ev.Message, Actions: []JobAction{}}
		switch {
		case ev.Export != nil:
			j.Result.Data = ev.Export
			if result, ok := ev.Export.(map[string]any); ok {
				path, _ := result["path"].(string)
				j.Result.Actions = []JobAction{{Label: "Tải xuống", Kind: "download", Href: exportDownloadHref(path), Primary: true}}
			}
		case ev.Sync != nil:
			j.Result.Data = ev.Sync
			j.Result.Actions = []JobAction{{Label: "Xem kết quả", Kind: "open", Href: "work", Primary: true}}
		case ev.Simulate != nil:
			j.Result.Data = ev.Simulate
			j.Result.Actions = []JobAction{{Label: "Xem hồ sơ", Kind: "open", Href: "/settings/writing", Primary: true}}
		case ev.Import != nil:
			j.Result.Data = ev.Import
			j.Href = "work?dlg=import"
			j.Result.Actions = []JobAction{{Label: "Mở truyện", Kind: "open", Href: "work", Primary: true}}
		}
		if ev.Kind == "diag" {
			j.Result.Actions = []JobAction{{Label: "Xem báo cáo", Kind: "open", Href: "work?dlg=diag:last", Primary: true}}
		}
		if cancel := t.cancels[ev.Job]; cancel != nil {
			cancel()
			delete(t.cancels, ev.Job)
			delete(t.contexts, ev.Job)
		}
	}
	t.entries[ev.Job] = j
	if t.notify != nil && (!existed || !jobsEqual(prev, j)) {
		t.notify(prev, j, existed)
	}
}

func (t *jobTracker) listJobs() []Job {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := []Job{}
	for id, j := range t.entries {
		if j.Status == JobDoneS && j.FinishedAt != nil && time.Since(*j.FinishedAt) > jobRetention {
			delete(t.entries, id)
			delete(t.retries, id)
			delete(t.history, id)
			continue
		}
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

func (s *workerServer) updateWritingJob(snap host.UISnapshot) {
	t := s.jobs
	t.mu.Lock()
	defer t.mu.Unlock()
	var prev Job
	existed := false
	for _, j := range t.entries {
		if j.Kind == "write" && j.Status != JobDoneS {
			prev = j
			existed = true
			break
		}
	}
	if !existed && !snap.IsRunning && snap.WaitingFor != "gate" {
		return
	}
	j := prev
	if !existed {
		t.seq++
		j = Job{ID: fmt.Sprintf("write/%d", t.seq), Kind: "write", Title: "Sáng tác", StartedAt: time.Now(), Href: ""}
	}
	j.Status = JobRunning
	j.Href = "work"
	j.Agent = ""
	switch {
	case snap.WaitingFor == "gate":
		j.Status = JobWaiting
		j.Step = "Chờ duyệt chương"
		j.Href = "work"
	case !snap.IsRunning:
		j.Status = JobPausedS
		j.Step = host.StatusLabelText(snap.StatusLabel)
	default:
		j.Step = host.StatusLabelText(snap.StatusLabel)
	}
	if snap.Phase == "complete" {
		j.Status = JobDoneS
		now := time.Now()
		j.FinishedAt = &now
	}
	j.Unit = &JobUnit{Done: snap.CompletedCount, Total: snap.TotalChapters, Label: "Chương"}
	if snap.TotalChapters > 0 {
		p := min(1, float64(snap.CompletedCount)/float64(snap.TotalChapters))
		j.Progress = &p
	}
	for _, a := range snap.Agents {
		if a.State == "running" {
			j.Agent = a.Name
			break
		}
	}
	t.entries[j.ID] = j
	if t.notify != nil && (!existed || !jobsEqual(prev, j)) {
		t.notify(prev, j, existed)
	}
}

func (s *workerServer) workerJobs(w http.ResponseWriter, r *http.Request, p string) bool {
	if r.Method == http.MethodGet && p == "jobs" {
		s.snapshot()
		writeJSON(w, 200, map[string]any{"jobs": s.jobs.listJobs()})
		return true
	}
	if r.Method == http.MethodGet && strings.HasPrefix(p, "jobs/") {
		s.getWorkerJob(w, strings.TrimPrefix(p, "jobs/"))
		return true
	}
	if r.Method != http.MethodPost || !strings.HasPrefix(p, "jobs/") {
		return false
	}
	rest := strings.TrimPrefix(p, "jobs/")
	id, op, ok := strings.Cut(rest, "/")
	n, op, ok2 := strings.Cut(op, "/")
	id += "/" + n
	if !ok || !ok2 || !jobOps[op] {
		jsonError(w, 404, errJobNotFound.Error())
		return true
	}
	t := s.jobs
	t.mu.Lock()
	j, known := t.entries[id]
	cancel := t.cancels[id]
	retry := t.retries[id]
	t.mu.Unlock()
	if !known {
		jsonError(w, 404, errJobNotFound.Error())
		return true
	}
	var err error
	switch {
	case op == "cancel" && j.Status == JobDoneS:
		t.mu.Lock()
		delete(t.entries, id)
		delete(t.retries, id)
		t.mu.Unlock()
		s.b.emit(message{kind: "job.removed", data: map[string]string{"id": id}})
	case j.Kind == "write":
		switch op {
		case "pause":
			if !s.h.PauseAfterStep() {
				err = errJobUnsupported
			}
		case "resume", "retry":
			_, err = s.h.Resume()
		case "cancel":
			s.h.Abort()
		}
	case op == "cancel":
		if cancel != nil {
			cancel()
		}
		if j.Status == JobWaiting {
			s.finishJob(id, j.Kind, jobEvent{Message: "Đã huỷ"})
		}
	case op == "retry" && retry != nil && (j.Status == JobFailed || j.Status == JobDoneS):
		_, err = retry()
	default:
		err = errJobUnsupported
	}
	if err != nil {
		jsonError(w, 409, err.Error())
	} else {
		writeJSON(w, 200, map[string]bool{"ok": true})
	}
	return true
}

func syncResult(result *revision.Result, h workerHost) map[string]any {
	applied := []int{}
	analyses := []map[string]any{}
	titles := map[int]string{}
	if chapters, err := h.Chapters(); err == nil {
		for _, c := range chapters {
			titles[c.Chapter] = c.Title
		}
	}
	if result != nil {
		applied = append(applied, result.Applied...)
		for i, a := range result.Analyses {
			chapter := 0
			if i < len(result.Applied) {
				chapter = result.Applied[i]
			}
			analyses = append(analyses, syncAnalysis(chapter, titles[chapter], a, titles))
		}
	}
	return map[string]any{"schemaVersion": 1, "applied": applied, "analyses": analyses}
}

func (s *workerServer) getRevisionDiff(w http.ResponseWriter, r *http.Request, p string) {
	chapter, err := strconv.Atoi(strings.TrimPrefix(p, "sync/diff/"))
	if err != nil || chapter < 1 {
		jsonError(w, 404, "Không tìm thấy chương")
		return
	}
	diff, err := s.h.RevisionDiffFor(chapter)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, diff)
}
