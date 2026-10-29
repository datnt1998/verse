package web

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host/imp"
	"github.com/datnt1998/verse/apps/noverse/internal/host/sim"
	"github.com/datnt1998/verse/apps/noverse/internal/revision"
)

// jobEvent 对应契约 §5 JobEvent：长任务（import/simulate/importsim/sync/export）经 SSE `job` 下发的进度。
type jobEvent struct {
	Job      string        `json:"job"`
	Kind     string        `json:"kind"`
	Stage    string        `json:"stage"`
	Current  int           `json:"current"`
	Total    int           `json:"total"`
	Message  string        `json:"message"`
	Level    string        `json:"level"`
	RetryAt  *time.Time    `json:"retryAt"`
	Done     bool          `json:"done"`
	Error    string        `json:"error"`
	Awaiting string        `json:"awaiting"`
	Export   any           `json:"export,omitempty"`
	Sync     any           `json:"sync,omitempty"`
	Simulate any           `json:"simulate,omitempty"`
	Import   *imp.Progress `json:"import,omitempty"`
}

// jobTracker 登记每类长任务的运行状态与最新事件：同 kind 并发 → 409；
// 最新事件供 GET /import 展示与新 SSE 订阅者重放恢复进度面板。
type jobTracker struct {
	mu       sync.Mutex
	seq      int
	running  map[string]bool
	last     map[string]jobEvent
	history  map[string]jobEvent
	entries  map[string]Job
	cancels  map[string]context.CancelFunc
	contexts map[string]context.Context
	retries  map[string]func() (string, error)
	notify   func(Job, Job, bool)
}

func newJobTracker() *jobTracker {
	return &jobTracker{seq: int(time.Now().UnixNano()), running: map[string]bool{}, last: map[string]jobEvent{}, history: map[string]jobEvent{}, entries: map[string]Job{}, cancels: map[string]context.CancelFunc{}, contexts: map[string]context.Context{}, retries: map[string]func() (string, error){}}
}

// begin 原子占用一个任务种类；已占用返回 ok=false（对应 409）。
func (t *jobTracker) begin(kind string) (id string, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running[kind] {
		return "", false
	}
	t.running[kind] = true
	t.seq++
	idKind := kind
	if idKind == "importsim" {
		idKind = "simulate"
	}
	id = fmt.Sprintf("%s/%d", idKind, t.seq)
	if last, exists := t.last[kind]; exists && last.Awaiting != "" {
		id = last.Job
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.contexts[id], t.cancels[id] = ctx, cancel
	return id, true
}

func (t *jobTracker) end(kind string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.running, kind)
}

func (t *jobTracker) record(ev jobEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.last[ev.Kind] = ev
	t.history[ev.Job] = ev
	t.recordJobLocked(ev)
}

func (t *jobTracker) latest(kind string) (jobEvent, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ev, ok := t.last[kind]
	return ev, ok
}

// replays preserves every retained job result across workspace remounts.
func (t *jobTracker) replays() []message {
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := make([]string, 0, len(t.history))
	for id := range t.history {
		j, exists := t.entries[id]
		if exists && j.Status == JobDoneS && j.FinishedAt != nil && time.Since(*j.FinishedAt) > jobRetention {
			delete(t.history, id)
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]message, 0, len(ids))
	for _, id := range ids {
		out = append(out, message{kind: "job", data: t.history[id]})
	}
	return out
}

// emitJob 记录并广播一条 job 事件。
func (s *workerServer) emitJob(ev jobEvent) {
	s.jobs.record(ev)
	s.b.emit(message{kind: "job", data: ev})
}

// finishJob 释放任务槽并补发终态事件（done:true）；上游未发任何事件时给一条兜底。
func (s *workerServer) finishJob(id, kind string, last jobEvent) {
	s.jobs.end(kind)
	last.Job, last.Kind, last.Done = id, kind, true
	if s.jobs.context(id).Err() != nil {
		last.Stage, last.Level, last.Message, last.Error, last.Awaiting = "cancelled", "info", "Đã huỷ", "", ""
	}
	if last.Stage == "" {
		last.Stage = "done"
	}
	if last.Level == "" {
		last.Level = "info"
	}
	if last.Message == "" && last.Error == "" {
		last.Message = "Hoàn tất"
	}
	s.emitJob(last)
}

// pumpImport 转发导入事件为 JobEvent；上游通道关闭（完成/停点/错误）后补 done:true。
func (s *workerServer) pumpImport(id string, ch <-chan imp.Event) {
	var last jobEvent
	var progress *imp.Progress
	if previous, ok := s.jobs.latest("import"); ok && previous.Job == id {
		progress = previous.Import
	}
	started := time.Now()
	for ev := range ch {
		progress = imp.AdvanceProgress(progress, ev, started)
		last = jobEvent{
			Job: id, Kind: "import", Stage: string(ev.Stage),
			Current: ev.Current, Total: ev.Total, Message: ev.Message,
			Import: progress,
		}
		switch {
		case ev.Stage == imp.StageError:
			last.Level = "error"
			if ev.Err != nil {
				last.Error = ev.Err.Error()
			}
		case ev.Stage == imp.StageDone:
			last.Level = "success"
		case ev.Level == "warn":
			last.Level = "warn"
		default:
			last.Level = "info"
		}
		switch ev.Stage {
		case imp.StageAwaitingConfirmation:
			last.Awaiting = "confirm"
		case imp.StageAwaitingStoryStatus:
			last.Awaiting = "storyStatus"
		}
		if !ev.RetryAt.IsZero() {
			t := ev.RetryAt
			last.RetryAt = &t
		}
		s.emitJob(last)
	}
	s.finishJob(id, "import", last)
}

// pumpSim 转发仿写画像事件（simulate / importsim 共用 sim.Event）。
func (s *workerServer) pumpSim(id, kind string, ch <-chan sim.Event) {
	var last jobEvent
	for ev := range ch {
		last = jobEvent{
			Job: id, Kind: kind, Stage: string(ev.Stage),
			Current: ev.Current, Total: ev.Total, Message: ev.Message,
		}
		switch ev.Stage {
		case sim.StageError:
			last.Level = "error"
			if ev.Err != nil {
				last.Error = ev.Err.Error()
			}
		case sim.StageDone:
			last.Level = "success"
		default:
			last.Level = "info"
		}
		s.emitJob(last)
	}
	if view, err := s.h.SimulationProfileView(); err == nil {
		last.Simulate = view
	}
	s.finishJob(id, kind, last)
}

// runSyncJob 把同步章节修订（一次模型分析批次）包成 job：起始进度 + 终态汇总。
func (s *workerServer) runSyncJob(id string) {
	s.emitJob(jobEvent{Job: id, Kind: "sync", Stage: "sync", Level: "info",
		Message: "Đang phân tích và tiếp nhận các chương tu chỉnh..."})
	res, err := s.h.SyncChapterRevisions(s.jobs.context(id))
	last := jobEvent{Job: id, Kind: "sync", Stage: "done", Level: "success", Message: viSyncSummary(res)}
	last.Sync = syncResult(res, s.h)
	if err != nil {
		last.Stage, last.Level = "error", "error"
		last.Error, last.Message = err.Error(), err.Error()
	}
	s.finishJob(id, "sync", last)
}

// viSyncSummary 移植 TUI formatRevisionResult 的越南语汇总。
func viSyncSummary(result *revision.Result) string {
	if result == nil || len(result.Applied) == 0 {
		return "Không phát hiện chương nào bị chỉnh sửa từ bên ngoài"
	}
	parts := make([]string, 0, len(result.Analyses))
	for i, analysis := range result.Analyses {
		if i >= len(result.Applied) {
			break
		}
		part := fmt.Sprintf("Chương %d: %s", result.Applied[i], analysis.ChangeSummary)
		if analysis.StoryChanged {
			part += " (sự thật cốt truyện đã cập nhật)"
		}
		if len(analysis.DownstreamIssues) > 0 {
			part += fmt.Sprintf(" (phát hiện %d xung đột phía sau)", len(analysis.DownstreamIssues))
		}
		parts = append(parts, part)
	}
	summary := fmt.Sprintf("Đã tiếp nhận tu chỉnh các chương: %v", result.Applied)
	if len(parts) > 0 {
		summary += "; " + strings.Join(parts, "; ")
	}
	return summary
}
