package eval

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore"
	"github.com/datnt1998/verse/core/agentcore/subagent"
)

// writeSessionRaw 追加若干原始 JSONL 行（事件行 _event 或裁剪过的消息 JSON）。
// fixture 直接复刻真实 session 的行形态：{"_event":{...}} 与 llm.Message 交错。
func writeSessionRaw(t *testing.T, root, rel string, lines ...string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir session: %v", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer f.Close()
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			t.Fatalf("write session: %v", err)
		}
	}
}

// 采集器契约：tool result 按 metadata.error_kind 分桶（缺 kind 归 business）、
// _event 行进 retries/stream_idle/compactions_by_tool/subagent_exit_by_trigger，
// assistant usage 按文件 agent 归并，duration_s 由 run.json 与最后时间戳算出。
// fixture 裁剪自真实 run 的行形态（短内容、无秘密），四种错误分类齐出。
func TestCollectCoreSignals(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-90 * time.Second).UTC().Truncate(time.Second)
	if err := s.RunMeta.Save(domain.RunMeta{StartedAt: started.Format(time.RFC3339)}); err != nil {
		t.Fatalf("save run meta: %v", err)
	}
	if err := s.Usage.Save(domain.UsageState{
		UpdatedAt: started.Add(90 * time.Second),
		Overall:   domain.AgentUsageTotals{Input: 100, Output: 10, Cost: 0.01},
	}); err != nil {
		t.Fatalf("save usage: %v", err)
	}

	msg := func(role llmtypes.Role, md map[string]any) llmtypes.Message {
		return llmtypes.Message{
			Role:      role,
			Content:   []llmtypes.ContentBlock{llmtypes.TextBlock(`"x"`)},
			Metadata:  md,
			Timestamp: started.Add(60 * time.Second),
		}
	}
	toolMsg := func(kind string) llmtypes.Message {
		return msg(llmtypes.RoleTool, map[string]any{
			"tool_call_id": "c-" + kind,
			"tool_name":    "novel_context",
			"is_error":     true,
			"error_kind":   kind,
		})
	}
	toolMsgUntagged := msg(llmtypes.RoleTool, map[string]any{
		"tool_call_id": "c-old", "tool_name": "read_chapter", "is_error": true,
	})
	assistant := llmtypes.Message{
		Role:      llmtypes.RoleAssistant,
		Content:   []llmtypes.ContentBlock{llmtypes.TextBlock("ok")},
		Usage:     &llmtypes.Usage{Input: 100, Output: 10, CacheRead: 5, Cost: &llmtypes.Cost{Total: 0.002}},
		Timestamp: started.Add(80 * time.Second),
	}

	writeSessionLines(t, dir, "meta/sessions/agents/writer-ch01.jsonl",
		msg(llmtypes.RoleUser, nil),
		assistant,
		toolMsg("validation"), toolMsg("business"), toolMsg("blocked"), toolMsg("circuit"), toolMsgUntagged,
	)
	writeSessionRaw(t, dir, "meta/sessions/agents/writer-ch01.jsonl",
		`{"_event":{"kind":"retry","agent":"writer","at":"t","attempt":1,"max":7,"delay_ms":1000,"error_kind":"stream_idle"}}`,
		`{"_event":{"kind":"stream_idle","agent":"writer","at":"t","error_kind":"stream_idle"}}`,
		`{"_event":{"kind":"tool_blocked","agent":"writer","at":"t","tool":"novel_context","reason":"repeat_failure"}}`,
		`{"_event":{"kind":"circuit_open","agent":"writer","at":"t","tool":"read_chapter"}}`,
		`{"_event":{"kind":"compaction","agent":"writer","at":"t","tool":"read_chapter","reason":"tool_result_microcompact"}}`,
		`{"_event":{"kind":"compaction","agent":"writer","at":"t","reason":"overflow"}}`,
		`{"_event":{"kind":"subagent_exit","agent":"writer","at":"t","trigger":"tool"}}`,
		`{"_event":{"kind":"unknown_future","agent":"writer","at":"t"}}`,
	)
	writeSessionLines(t, dir, "meta/sessions/agents/architect_long-001.jsonl",
		msg(llmtypes.RoleUser, nil),
		llmtypes.Message{
			Role:      llmtypes.RoleAssistant,
			Content:   []llmtypes.ContentBlock{llmtypes.TextBlock("plan")},
			Usage:     &llmtypes.Usage{Input: 50, Output: 5, Cost: &llmtypes.Cost{Total: 0.001}},
			Timestamp: started.Add(40 * time.Second),
		},
	)
	writeSessionRaw(t, dir, "meta/sessions/agents/architect_long-001.jsonl",
		`{"_event":{"kind":"subagent_exit","agent":"architect_long","at":"t","trigger":"stop"}}`,
	)

	col := Collect(dir, nil)
	if len(col.LoadErrors) != 0 {
		t.Fatalf("不应有读取错误: %v", col.LoadErrors)
	}
	if col.ToolErrors != 5 {
		t.Fatalf("tool_errors = %d want 5", col.ToolErrors)
	}
	want := map[string]int{"validation": 1, "business": 2, "blocked": 1, "circuit": 1}
	for k, n := range want {
		if col.ToolErrorsByKind[k] != n {
			t.Fatalf("tool_errors_by_kind[%s] = %d want %d (%v)", k, col.ToolErrorsByKind[k], n, col.ToolErrorsByKind)
		}
	}
	sum := 0
	for _, n := range col.ToolErrorsByKind {
		sum += n
	}
	if sum != col.ToolErrors {
		t.Fatalf("不变式破坏: sum(tool_errors_by_kind)=%d != tool_errors=%d", sum, col.ToolErrors)
	}
	if col.Retries != 1 || col.StreamIdle != 1 {
		t.Fatalf("retries=%d stream_idle=%d want 1/1", col.Retries, col.StreamIdle)
	}
	if col.ToolBlocked != 1 || col.CircuitOpen != 1 {
		t.Fatalf("tool_blocked=%d circuit_open=%d want 1/1", col.ToolBlocked, col.CircuitOpen)
	}
	if col.CompactionsByTool["read_chapter"] != 1 || col.CompactionsByTool["context"] != 1 {
		t.Fatalf("compactions_by_tool = %v", col.CompactionsByTool)
	}
	if col.SubagentExitByTrigger["tool"] != 1 || col.SubagentExitByTrigger["stop"] != 1 {
		t.Fatalf("subagent_exit_by_trigger = %v", col.SubagentExitByTrigger)
	}
	w := col.UsageByAgent["writer"]
	if w.Input != 100 || w.Output != 10 || w.CacheRead != 5 || w.CostUSD != 0.002 {
		t.Fatalf("usage_by_agent[writer] = %+v", w)
	}
	if col.UsageByAgent["architect_long"].Input != 50 {
		t.Fatalf("usage_by_agent[architect_long] = %+v", col.UsageByAgent["architect_long"])
	}
	if col.DurationS <= 0 || col.DurationS > 120 {
		t.Fatalf("duration_s = %v", col.DurationS)
	}
	// 新字段在干净的 run 也不得为 null map。
	for name, m := range map[string]map[string]int{
		"tool_errors_by_kind": col.ToolErrorsByKind, "compactions_by_tool": col.CompactionsByTool,
		"subagent_exit_by_trigger": col.SubagentExitByTrigger,
	} {
		if m == nil {
			t.Fatalf("%s 为 null", name)
		}
	}
	if col.UsageByAgent == nil {
		t.Fatal("usage_by_agent 为 null")
	}
}

// 无 _event 与 error_kind 的旧日志：新字段全部零值但不为 null，sum 不变式仍成立。
func TestCollectSignalsEmptyRun(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	writeSessionLines(t, dir, "meta/sessions/agents/writer-ch01.jsonl",
		llmtypes.Message{Role: llmtypes.RoleUser, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("task")}},
		llmtypes.Message{
			Role:     llmtypes.RoleTool,
			Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(`"boom"`)},
			Metadata: map[string]any{"tool_call_id": "c1", "tool_name": "read_chapter", "is_error": true},
		},
	)
	col := Collect(dir, nil)
	if col.ToolErrors != 1 || col.ToolErrorsByKind["business"] != 1 {
		t.Fatalf("旧结果应归 business: %+v", col.ToolErrorsByKind)
	}
	if col.Retries != 0 || col.StreamIdle != 0 || col.ToolBlocked != 0 || col.CircuitOpen != 0 {
		t.Fatalf("事件计数应为 0: %+v", col)
	}
	if col.CompactionsByTool == nil || col.SubagentExitByTrigger == nil ||
		col.ToolErrorsByKind == nil || col.UsageByAgent == nil {
		t.Fatal("空 run 的新 map 字段不得为 null")
	}
}

// scriptedModel 按脚本返回流事件；仅本测试使用。第一次 GenerateStream 返回
// stream-idle（可重试）错误，第二次返回带 tool_use 的正常完成。
type scriptedModel struct {
	calls int
}

func (m *scriptedModel) SupportsTools() bool { return true }

func (m *scriptedModel) Generate(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return nil, errors.New("generate not supported in test model")
}

// streamIdleErr 同时满足 RetryableError 与 ErrProviderStreamIdle 分类——
// 等价于 adapter 对 watchdog 中止的映射。
type streamIdleErr struct{ err error }

func (e *streamIdleErr) Error() string        { return e.err.Error() }
func (e *streamIdleErr) Unwrap() error        { return e.err }
func (e *streamIdleErr) Retryable() bool      { return true }
func (e *streamIdleErr) Is(target error) bool { return target == llmtypes.ErrProviderStreamIdle }

func (m *scriptedModel) GenerateStream(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	m.calls++
	if m.calls == 1 {
		return nil, &streamIdleErr{err: fmt.Errorf("%w: stream idle timeout", llmtypes.ErrProviderStreamIdle)}
	}
	ch := make(chan llmtypes.StreamEvent, 2)
	if m.calls == 2 {
		ch <- llmtypes.StreamEvent{
			Type: llmtypes.StreamEventDone,
			Message: llmtypes.Message{
				Role:       llmtypes.RoleAssistant,
				Content:    []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "call-1", Name: "read_chapter", Args: json.RawMessage(`{}`)})},
				StopReason: llmtypes.StopReasonToolUse,
				Usage:      &llmtypes.Usage{Input: 10, Output: 5},
			},
			StopReason: llmtypes.StopReasonToolUse,
		}
	} else {
		ch <- llmtypes.StreamEvent{
			Type: llmtypes.StreamEventDone,
			Message: llmtypes.Message{
				Role:       llmtypes.RoleAssistant,
				Content:    []llmtypes.ContentBlock{llmtypes.TextBlock("done")},
				Usage:      &llmtypes.Usage{Input: 10, Output: 5},
				StopReason: llmtypes.StopReasonStop,
			},
			StopReason: llmtypes.StopReasonStop,
		}
	}
	close(ch)
	return ch, nil
}

// 端到端桥接：fake model 第一次流式失败触发 retry，工具调用参数缺必填字段触发
// validation 错误；整条链路走 subagent.Runner → OnMessage/OnEvent →
// Sessions 的真实写入，再由 Collect 读回计数。
func TestSessionBridgeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	onMsg := s.Sessions.SubAgentLogger(nil)
	writeEvent := s.Sessions.SubAgentEventLogger()
	onEvent := func(agentName, task string, ev agentcore.Event) {
		kind, fields := store.SessionEventFields(ev)
		if kind == "" {
			return
		}
		writeEvent(agentName, task, kind, fields)
	}
	// 必填 chapter 的 schema：模型传 {} 必失败 schema validation。
	tool := agentcore.NewFuncTool("read_chapter", "đọc chương", map[string]any{
		"type":       "object",
		"properties": map[string]any{"chapter": map[string]any{"type": "integer"}},
		"required":   []any{"chapter"},
	}, func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"chapter":1}`), nil
	})

	runner := subagent.NewRunner(subagent.Config{
		Name:         "writer",
		Description:  "test writer",
		Model:        &scriptedModel{},
		SystemPrompt: "test",
		Tools:        []agentcore.Tool{tool},
		MaxTurns:     5,
		MaxRetries:   2,
		OnMessage:    onMsg,
		OnEvent:      onEvent,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := runner.Run(ctx, "writer", "Viết chương 1")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if res.Output == "" {
		t.Fatal("run produced no output")
	}

	col := Collect(dir, nil)
	if len(col.LoadErrors) != 0 {
		t.Fatalf("不应有读取错误: %v", col.LoadErrors)
	}
	if col.Retries < 1 {
		t.Fatalf("retries = %d want >= 1（首次流式失败应重试）", col.Retries)
	}
	if col.StreamIdle < 1 {
		t.Fatalf("stream_idle = %d want >= 1", col.StreamIdle)
	}
	if col.ToolErrorsByKind["validation"] < 1 {
		t.Fatalf("tool_errors_by_kind.validation = %d want >= 1 (%v)", col.ToolErrorsByKind["validation"], col.ToolErrorsByKind)
	}
	sum := 0
	for _, n := range col.ToolErrorsByKind {
		sum += n
	}
	if sum != col.ToolErrors {
		t.Fatalf("不变式破坏: sum=%d != tool_errors=%d", sum, col.ToolErrors)
	}
	if col.SubagentExitByTrigger["stop"] != 1 {
		t.Fatalf("subagent_exit_by_trigger = %v want stop=1", col.SubagentExitByTrigger)
	}
}
