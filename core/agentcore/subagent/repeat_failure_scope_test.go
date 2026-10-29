package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/core/agentcore"
	llmtypes "github.com/datnt1998/verse/core/llm"
)

// TestRepeatFailureGuardPersistsAcrossRuns pins the guard's lifetime: the
// middleware instance lives on Runner.Config, so failure counts survive
// sequential Runner.Run calls — a call blocked in run N stays blocked in
// run N+1, and the refusal still surfaces as a tool_blocked event with
// reason "repeat_failure" plus an is_error tool result.
func TestRepeatFailureGuardPersistsAcrossRuns(t *testing.T) {
	var toolRuns atomic.Int32
	tool := agentcore.NewFuncTool("always_fails", "f", map[string]any{"type": "object"},
		func(context.Context, json.RawMessage) (json.RawMessage, error) {
			toolRuns.Add(1)
			return nil, errors.New("boom")
		})

	var blockedEvents atomic.Int32
	var blockedReasons []string
	cfg := Config{
		Name:     "worker",
		Model:    newSequential(modelDecidesOnToolResult),
		Tools:    []agentcore.Tool{tool},
		MaxTurns: 3,
		Middlewares: []agentcore.ToolMiddleware{
			agentcore.NewRepeatFailureMiddleware(nil),
		},
		OnEvent: func(agentName, task string, ev agentcore.Event) {
			if ev.Type == agentcore.EventToolBlocked {
				blockedEvents.Add(1)
				blockedReasons = append(blockedReasons, ev.Reason)
			}
		},
	}
	runner := NewRunner(cfg)

	// Two runs, one failing call each → the limit is reached.
	for i := range agentcore.RepeatFailureLimit {
		if _, err := runner.Run(context.Background(), "worker", "go"); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	if got := toolRuns.Load(); int(got) != agentcore.RepeatFailureLimit {
		t.Fatalf("tool ran %d times across two runs, want %d", got, agentcore.RepeatFailureLimit)
	}

	// Third run: the identical call is refused before execution — proof the
	// counter outlives a single run.
	if _, err := runner.Run(context.Background(), "worker", "go"); err != nil {
		t.Fatalf("run 3: %v", err)
	}
	if got := toolRuns.Load(); int(got) != agentcore.RepeatFailureLimit {
		t.Fatalf("blocked call executed the tool in a later run: runs=%d", got)
	}
	if blockedEvents.Load() != 1 {
		t.Fatalf("want exactly one tool_blocked event, got %d", blockedEvents.Load())
	}
	if len(blockedReasons) != 1 || blockedReasons[0] != "repeat_failure" {
		t.Fatalf("tool_blocked reason = %v, want [repeat_failure]", blockedReasons)
	}
}

// modelDecidesOnToolResult always issues the same failing call; once it sees
// the tool result (success or refusal) it stops. Keeps each run to one
// attempted call so the runner-level scope test stays deterministic.
func modelDecidesOnToolResult(_ int, req *llmtypes.LLMRequest) (*llmtypes.LLMResponse, error) {
	for _, m := range req.Messages {
		if m.Role == llmtypes.RoleTool {
			blocked := m.Metadata["is_error"] == true
			if !blocked {
				return nil, errors.New("expected an error tool result")
			}
			return &llmtypes.LLMResponse{Message: llmtypes.Message{
				Role:       llmtypes.RoleAssistant,
				Content:    []llmtypes.ContentBlock{llmtypes.TextBlock("done")},
				StopReason: llmtypes.StopReasonStop,
			}}, nil
		}
	}
	return &llmtypes.LLMResponse{Message: llmtypes.Message{
		Role: llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{
			ID:   "call_1",
			Name: "always_fails",
			Args: json.RawMessage(`{"x":1}`),
		})},
		StopReason: llmtypes.StopReasonToolUse,
	}}, nil
}
