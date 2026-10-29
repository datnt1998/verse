package host

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"encoding/json"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/noverse/internal/agent"
	"github.com/datnt1998/verse/apps/noverse/internal/agent/subagent"
)

// EventText caps presentation text by Unicode characters, never splitting UTF-8.
func EventText(text string, limit int) string {
	n := 0
	for i := range text {
		if n == limit {
			return text[:i] + "…"
		}
		n++
	}
	if !utf8.ValidString(text) {
		return string([]rune(text))
	}
	return text
}

func (c *activeCall) setTool(tool string, args json.RawMessage) {
	c.tool, c.args = tool, EventText(string(args), 400)
	var p struct {
		Chapter int    `json:"chapter"`
		Scope   string `json:"scope"`
		Volume  int    `json:"volume"`
		Arc     int    `json:"arc"`
	}
	if json.Unmarshal(args, &p) == nil {
		c.chapter = p.Chapter
		c.scope, c.volume, c.arc = p.Scope, p.Volume, p.Arc
	}
}

func (c *activeCall) fillToolEvent(ev *Event, ok bool) {
	if c.tool == "" && ev.Category != "TOOL" {
		return
	}
	ev.Tool, ev.Args, ev.Result, ev.Chapter = c.tool, c.args, EventText(c.result, 800), c.chapter
	ev.Scope, ev.Volume, ev.Arc = c.scope, c.volume, c.arc
	ev.ResultHint = c.hint
	ev.DurationMs = ev.Duration.Milliseconds()
	ev.Status = "running"
	if !ev.FinishedAt.IsZero() {
		ev.Status = "ok"
		if !ok {
			ev.Status = "error"
		}
	}
	ev.Retries = append([]EventRetry(nil), c.retries...)
	if !ev.FinishedAt.IsZero() {
		for i := range ev.Retries {
			ev.Retries[i].OK = ok
		}
	}
}

// The raw callback runs before agentcore's progress relay, which omits results.
func (o *observer) captureToolResult(meta subagent.RunMeta, ev agentcore.Event) {
	switch ev.Type {
	case agentcore.EventToolExecEnd:
		if c := o.toolStarts[meta.Agent]; c != nil {
			c.hint = resultHint(ev.Result)
			c.result = EventText(string(ev.Result), 800)
		}
	case agentcore.EventModelResponse:
		o.refreshContextUsage(meta.Agent, ev.Message)
	}
}

// refreshContextUsage làm mới thanh ngữ cảnh ngay sau mỗi phản hồi model.
//
// agentcore chỉ phát ProgressContext ở cuối tool và cuối lượt assistant
// (subagent.reportContext), nên suốt một lượt dài — đúng lúc người dùng đang
// nhìn — thanh ctx đứng yên. Usage.Input của phản hồi chính là số token prompt
// vừa gửi, tức giá trị mới nhất mà backend báo; dùng nó cho Tokens và suy lại
// Percent theo ContextWindow đã biết. Chưa biết ContextWindow thì chỉ cập nhật
// Tokens, Percent giữ nguyên.
func (o *observer) refreshContextUsage(agent string, msg llmtypes.AgentMessage) {
	if agent == "" || msg == nil {
		return
	}
	m, ok := msg.(llmtypes.Message)
	if !ok || m.Usage == nil || m.Usage.Input <= 0 {
		return
	}
	tokens := m.Usage.Input
	// Resolve ngoài agentMu: tránh đảo thứ tự khoá với Snapshot (đọc h.mu).
	window := o.configuredContextWindow(agent)
	o.updateAgent(agent, func(a *agentState) {
		if a.context.ContextWindow <= 0 && window > 0 {
			a.context.ContextWindow = window
		}
		a.context.Tokens = tokens
		if a.context.ContextWindow > 0 {
			a.context.Percent = float64(tokens) / float64(a.context.ContextWindow) * 100
		}
	})
}

func (o *observer) configuredContextWindow(agent string) int {
	if o.contextWindowFor == nil {
		return 0
	}
	return o.contextWindowFor(agent)
}

// contextWindowForAgent trả cửa sổ ngữ cảnh đang cấu hình cho agent, theo role
// (architect_short/architect_long → architect) và cấu hình model hiện hành.
func (h *Host) contextWindowForAgent(agent string) int {
	h.mu.Lock()
	provider, model := h.cfg.Provider, h.cfg.ModelName
	cfg := h.cfg
	models := h.models
	h.mu.Unlock()

	role := agentRoleName(agent)
	if models != nil {
		if p, m, ok := models.CurrentSelection(role); ok {
			provider, model = p, m
		}
	} else if rc, ok := cfg.Roles[role]; ok {
		if rc.Provider != "" {
			provider = rc.Provider
		}
		if rc.Model != "" {
			model = rc.Model
		}
	}
	window, _ := cfg.ResolveContextWindow(provider, model)
	return window
}

// RecordUserEvent emits and persists user-originated input, not engine feedback.
func (h *Host) RecordUserEvent(username, command, text string) {
	h.emitEvent(Event{Time: time.Now(), Category: "USER", Agent: "user", Username: username, Summary: EventText(text, 1200), Command: command, Input: text, Kind: "user", Level: "info"})
}

// RecordActionError persists a rejected web action alongside engine failures.
func (h *Host) RecordActionError(username, message string) {
	now := time.Now()
	h.emitEvent(Event{Time: now, FinishedAt: now, Category: "ERROR", Agent: "host", Username: username, Summary: EventText(message, 1200), Kind: "command", Level: "error", Failed: true})
}

func (o *observer) recoverRetry(agent string) {
	ev, ok := o.retryRecords[agent]
	if !ok {
		return
	}
	delete(o.retryRecords, agent)
	ev.RetryAt = time.Time{}
	ev.CanRetryNow = false
	ev.Level = "success"
	ev.FinishedAt = time.Now()
	for i := range ev.Retries {
		ev.Retries[i].OK = true
	}
	o.emitEv(ev)
	o.persistEvent(ev)
}
