package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/core/agentcore"
)

// handleToolUpdate 处理 Worker 的进度中继(ProgressPayload):TOOL 行、流式正文、
// thinking、retry、context。Engine 经 observer.workerProgress 喂入。
func (o *observer) handleToolUpdate(ev agentcore.Event) {
	if ev.Progress == nil {
		return
	}
	switch ev.Progress.Kind {
	case agentcore.ProgressToolDelta:
		if ev.Progress.Delta != "" {
			o.handleSubagentDelta(ev.Progress)
		}
	case agentcore.ProgressToolStart:
		// Worker 内部的工具调用（如 writer → draft_chapter）。
		// 注意：TOOL 行可能已经在流式识别阶段被 handleSubagentDelta 提前发出。
		// 此处：若已发 → 只更新 summary（args 此时完整，能显示 "tool(第N章)"）；否则正常发。
		if ev.Progress.Agent == "" || ev.Progress.Tool == "" {
			break
		}
		o.recoverRetry(ev.Progress.Agent)
		// Some providers deliver complete arguments without tool deltas.
		// Render them through the same extractor instead of an empty header.
		if _, streamed := o.streamExtractors[ev.Progress.Agent]; !streamed {
			extractor := newToolExtractor(ev.Progress.Tool)
			if text := extractor.Feed(string(ev.Progress.Args)); text != "" {
				o.streamClear()
				o.emitStreamDelta(text, false)
			}
		}
		toolName := displayToolName(ev.Progress.Tool, ev.Progress.Args)
		if call := o.toolStarts[ev.Progress.Agent]; call != nil {
			call.setTool(ev.Progress.Tool, ev.Progress.Args)
		}
		if _, ok := o.toolStarts[ev.Progress.Agent]; ok {
			o.updateToolCallSummary(ev.Progress.Agent, ev.Progress.Tool, toolName)
			o.updateAgent(ev.Progress.Agent, func(a *agentState) {
				a.state = "working"
				a.tool = ev.Progress.Tool
				a.summary = fmt.Sprintf("%s → %s", ev.Progress.Agent, toolName)
			})
			break
		}
		// Non-streaming calls emit the same lifecycle once, after rendering args.
		id := nextEventID()
		o.toolStarts[ev.Progress.Agent] = &activeCall{id: id, start: time.Now(), summary: toolName, depth: 1}
		o.toolStarts[ev.Progress.Agent].setTool(ev.Progress.Tool, ev.Progress.Args)
		o.emitAndLog(Event{
			Tool: ev.Progress.Tool, Args: EventText(string(ev.Progress.Args), 400), Status: "running",
			ID:       id,
			Time:     time.Now(),
			Category: "TOOL",
			Agent:    ev.Progress.Agent,
			Summary:  toolName,
			Level:    "info",
			Depth:    1,
		})
		o.updateAgent(ev.Progress.Agent, func(a *agentState) {
			a.state = "working"
			a.tool = ev.Progress.Tool
			a.summary = fmt.Sprintf("%s → %s", ev.Progress.Agent, toolName)
		})
	case agentcore.ProgressToolEnd:
		delete(o.streamExtractors, ev.Progress.Agent)
		if ev.Progress.Agent == "" {
			return
		}
		call, ok := o.toolStarts[ev.Progress.Agent]
		if !ok {
			return
		}
		delete(o.toolStarts, ev.Progress.Agent)
		if call.chapter > 0 && (call.tool == "edit_chapter" || call.tool == "draft_chapter" || call.tool == "commit_chapter") {
			o.finishPolishAttempts(ev.Progress.Agent, call.chapter, nil)
		}
		// 同 ID 更新事件：TUI 按 ID 定位原 TOOL 行，回填 FinishedAt / Duration。
		// Summary / Depth 也带上，保证 runtime queue replay 时能还原完整行。
		finishEv := Event{
			ID:         call.id,
			Time:       call.start,
			FinishedAt: time.Now(),
			Category:   "TOOL",
			Agent:      ev.Progress.Agent,
			Summary:    call.summary,
			Level:      "info",
			Depth:      call.depth,
			Duration:   time.Since(call.start),
		}
		call.fillToolEvent(&finishEv, true)
		o.emitEv(finishEv)
		o.persistEvent(finishEv)
	case agentcore.ProgressThinking:
		o.handleThinkingProgress(ev)
	case agentcore.ProgressRetry:
		// 只展示上游明确报告的实际等待时间，避免本地估算与真实退避节奏不一致。
		// Summary 不嵌静态延时——UI 依 RetryAt 逐秒倒计时；Detail/日志保留发出时的延时快照。
		delay := retryProgressDelay(ev.Progress)
		if call := o.toolStarts[ev.Progress.Agent]; call != nil {
			call.retries = append(call.retries, EventRetry{AtSec: time.Since(call.start).Seconds(), Error: EventText(ev.Progress.Message, 800), WaitSec: delay.Seconds()})
		}
		retryEv := Event{
			ID:       o.retryEventID(ev.Progress.Agent, ev.Progress.Attempt),
			Time:     time.Now(),
			Category: "SYSTEM",
			Agent:    ev.Progress.Agent,
			Summary:  retryPrefix(ev.Progress.Attempt, ev.Progress.MaxRetries, 0) + truncate(ev.Progress.Message, 80),
			Detail:   retryPrefix(ev.Progress.Attempt, ev.Progress.MaxRetries, delay) + ev.Progress.Message,
			Kind:     errorKind(nil, ev.Progress.Message),
			Level:    "warn",
			Depth:    1,
		}
		if delay > 0 {
			retryEv.RetryAt = retryEv.Time.Add(delay)
		}
		retryEv.CanRetryNow = retryProgressCanRetryNow(ev.Progress)
		if o.retryRecords == nil {
			o.retryRecords = make(map[string]Event)
		}
		prior := o.retryRecords[ev.Progress.Agent]
		retryEv.Retries = append(prior.Retries, EventRetry{Error: EventText(ev.Progress.Message, 800), WaitSec: delay.Seconds()})
		o.retryRecords[ev.Progress.Agent] = retryEv
		o.emitEv(retryEv)
		o.persistEvent(retryEv)
	case agentcore.ProgressToolError:
		delete(o.streamExtractors, ev.Progress.Agent)
		msg := ev.Progress.Message
		if msg == "" {
			msg = "unknown error"
		}
		// 如果有进行中的 TOOL 行，原地标记为失败并把完整错误放进 Detail。
		// 同一次失败只生成一个 ERROR 级事件，避免 TOOL 失败态与附加 ERROR
		// 详情在 tui.log 中被误读成两次独立故障。
		if call, ok := o.toolStarts[ev.Progress.Agent]; ok {
			delete(o.toolStarts, ev.Progress.Agent)
			if ev.Progress.Tool == "edit_chapter" && call.chapter > 0 {
				call.result = msg
				call.retries = append(call.retries, EventRetry{AtSec: time.Since(call.start).Seconds(), Error: EventText(msg, 800)})
				if o.polishAttempts == nil {
					o.polishAttempts = make(map[string][]*activeCall)
				}
				o.polishAttempts[ev.Progress.Agent] = append(o.polishAttempts[ev.Progress.Agent], call)
				pending := Event{ID: call.id, Time: call.start, Category: "TOOL", Agent: ev.Progress.Agent, Summary: call.summary, Detail: fmt.Sprintf("%s lỗi: %s", ev.Progress.Tool, msg), Level: "info", Depth: call.depth}
				call.fillToolEvent(&pending, true)
				pending.Result = "" // diagnostics stay in Detail/Retries, not the user-facing result
				o.emitEv(pending)
				o.persistEvent(pending)
				return
			}
			detail := fmt.Sprintf("%s lỗi: %s", ev.Progress.Tool, msg)
			finishEv := Event{
				ID:         call.id,
				Time:       call.start,
				FinishedAt: time.Now(),
				Failed:     true,
				Category:   "TOOL",
				Agent:      ev.Progress.Agent,
				Summary:    fmt.Sprintf("%s lỗi: %s", call.summary, truncate(msg, 100)),
				Detail:     detail,
				Kind:       errorKind(nil, msg),
				Level:      "error",
				Depth:      call.depth,
				Duration:   time.Since(call.start),
			}
			call.result = msg
			call.fillToolEvent(&finishEv, false)
			o.emitEv(finishEv)
			o.persistEvent(finishEv)
			return
		}
		// 极少数缺失 start 的进度流无法原地更新，保留独立 ERROR 事件暴露故障。
		errEv := Event{
			Time:     time.Now(),
			Category: "ERROR",
			Agent:    ev.Progress.Agent,
			Summary:  fmt.Sprintf("%s lỗi: %s", ev.Progress.Tool, truncate(msg, 100)),
			Detail:   fmt.Sprintf("%s lỗi: %s", ev.Progress.Tool, msg),
			Kind:     errorKind(nil, msg),
			Level:    "error",
			Depth:    1,
		}
		o.emitEv(errEv)
		o.persistEvent(errEv)
	case agentcore.ProgressContext:
		o.handleContextProgress(ev)
	}
}

// retryProgressCanRetryNow 读取上游 meta 的 can_retry_now 标记（G4）。
func retryProgressCanRetryNow(p *agentcore.ProgressPayload) bool {
	if p == nil || len(p.Meta) == 0 {
		return false
	}
	var meta struct {
		CanRetryNow bool `json:"can_retry_now"`
	}
	return json.Unmarshal(p.Meta, &meta) == nil && meta.CanRetryNow
}

func retryProgressDelay(p *agentcore.ProgressPayload) time.Duration {
	if p == nil {
		return 0
	}
	if len(p.Meta) > 0 {
		var meta struct {
			DelayMS int64 `json:"retry_delay_ms"`
		}
		if json.Unmarshal(p.Meta, &meta) == nil && meta.DelayMS > 0 {
			return time.Duration(meta.DelayMS) * time.Millisecond
		}
	}
	return 0
}

func dispatchSummary(agent, task string) string {
	if agent == "" {
		agent = "subagent"
	}
	if task == "" {
		return agent
	}
	// 不在代码里按字符数截断任务首行（21-dev-prompt.md Gói O mục 4）：
	// UI 按实际宽度 text-overflow，完整文案进事件日志与悬停 tooltip。
	firstLine := strings.TrimSpace(strings.SplitN(task, "\n", 2)[0])
	if firstLine == "" {
		return agent
	}
	return agent + " (" + firstLine + ")"
}

func dispatchDetail(task, reason string) string {
	var parts []string
	if strings.TrimSpace(reason) != "" {
		parts = append(parts, "Lý do điều phối: "+reason)
	}
	if strings.TrimSpace(task) != "" {
		parts = append(parts, "Nhiệm vụ đầy đủ:\n"+task)
	}
	return strings.Join(parts, "\n")
}

func (o *observer) updateToolCallSummary(agent, tool, summary string) {
	if agent == "" || summary == "" {
		return
	}
	call, ok := o.toolStarts[agent]
	if !ok {
		return
	}
	call.summary = summary
	o.emitEv(Event{
		ID:   call.id,
		Tool: call.tool, Args: call.args, Chapter: call.chapter, Status: "running",
		Time:     call.start,
		Category: "TOOL",
		Agent:    agent,
		Summary:  summary,
		Level:    "info",
		Depth:    call.depth,
	})
	o.updateAgent(agent, func(a *agentState) {
		a.state = "working"
		a.tool = tool
		a.summary = fmt.Sprintf("%s → %s", agent, summary)
	})
}

func (o *observer) updateToolCallSummaryFromDelta(agent, tool, delta string) {
	key := streamArgKey(agent, tool)
	prefix := o.streamArgPrefixes[key] + delta
	if len(prefix) > 512 {
		prefix = prefix[:512]
	}
	o.streamArgPrefixes[key] = prefix

	summary := streamedToolLabel(tool, prefix)
	if summary == "" {
		return
	}
	if o.streamArgLabels[key] == summary {
		return
	}
	o.streamArgLabels[key] = summary
	o.updateToolCallSummary(agent, tool, summary)
}

func streamArgKey(agent, tool string) string {
	return agent + "\x00" + tool
}

func streamedToolLabel(tool, delta string) string {
	if tool != "save_foundation" || delta == "" {
		return ""
	}
	typ := firstJSONStringField(delta, "type")
	if typ == "" {
		return ""
	}
	return fmt.Sprintf("%s[%s]", tool, typ)
}

func firstJSONStringField(raw, field string) string {
	needle := `"` + field + `"`
	idx := strings.Index(raw, needle)
	if idx < 0 {
		return ""
	}
	rest := raw[idx+len(needle):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return ""
	}
	rest = strings.TrimLeft(rest[colon+1:], " \t\r\n")
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	var value strings.Builder
	escape := false
	for i := 1; i < len(rest); i++ {
		c := rest[i]
		if escape {
			value.WriteByte(c)
			escape = false
			continue
		}
		switch c {
		case '\\':
			escape = true
		case '"':
			return value.String()
		default:
			value.WriteByte(c)
		}
	}
	return ""
}

func (o *observer) emitCallFinish(call *activeCall, category, agentName string, callErr error) {
	if call == nil {
		return
	}
	failed := callErr != nil
	level := "success"
	if failed {
		level = "error"
	}
	summary := call.summary
	detail := ""
	kind := ""
	var question *workerQuestion
	if errors.As(callErr, &question) {
		failed, level, kind = false, "warn", "attention"
		detail = question.text
		summary = call.summary + " · cần bạn xem"
	} else if failed {
		detail = callErr.Error()
		kind = errorKind(callErr, detail)
		summary = fmt.Sprintf("%s lỗi: %s", call.summary, truncate(detail, 100))
	}
	finishEv := Event{
		ID:         call.id,
		Time:       call.start,
		FinishedAt: time.Now(),
		Failed:     failed,
		Category:   category,
		Agent:      agentName,
		Summary:    summary,
		Detail:     detail,
		Kind:       kind,
		Level:      level,
		Depth:      call.depth,
		Duration:   time.Since(call.start),
	}
	call.fillToolEvent(&finishEv, !failed)
	if question != nil {
		finishEv.Status = "attention"
	}
	o.emitEv(finishEv)
	o.persistEvent(finishEv)
}

func displayToolName(tool string, args json.RawMessage) string {
	if len(args) == 0 {
		return tool
	}
	switch tool {
	case "save_foundation":
		var p struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(args, &p) == nil && p.Type != "" {
			return fmt.Sprintf("%s[%s]", tool, p.Type)
		}
	case "commit_chapter", "plan_chapter", "draft_chapter", "check_consistency":
		var p struct {
			Chapter int `json:"chapter"`
		}
		if json.Unmarshal(args, &p) == nil && p.Chapter > 0 {
			return fmt.Sprintf("%s(chương %d)", tool, p.Chapter)
		}
	case "save_review":
		var p domain.ReviewEntry
		if json.Unmarshal(args, &p) == nil {
			label := ""
			switch p.Scope {
			case "arc":
				label = "cung này"
			case "global":
				label = "toàn cục"
			default:
				if p.Chapter > 0 {
					label = fmt.Sprintf("chương %d", p.Chapter)
				}
			}
			if label == "" {
				return tool
			}
			if p.Verdict != "" {
				return fmt.Sprintf("%s(%s·%s)", tool, label, p.Verdict)
			}
			return fmt.Sprintf("%s(%s)", tool, label)
		}
	case "novel_context":
		var p struct {
			Chapter int `json:"chapter"`
		}
		if json.Unmarshal(args, &p) == nil && p.Chapter > 0 {
			return fmt.Sprintf("%s(chương %d)", tool, p.Chapter)
		}
	case "read_chapter":
		var p struct {
			Chapter   int    `json:"chapter"`
			Source    string `json:"source"`
			Character string `json:"character"`
		}
		if json.Unmarshal(args, &p) == nil && p.Chapter > 0 {
			suffix := ""
			if p.Character != "" {
				suffix = " · thoại " + p.Character
			} else if p.Source == "draft" {
				suffix = " · bản nháp"
			}
			return fmt.Sprintf("%s(chương %d%s)", tool, p.Chapter, suffix)
		}
	}
	return tool
}
