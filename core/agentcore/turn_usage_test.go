package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	llmtypes "github.com/datnt1998/verse/core/llm"
)

func TestAgentLoopTracksUsageAndCostPerTurn(t *testing.T) {
	first := assistantMsg("", llmtypes.StopReasonToolUse)
	first.Content = []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "a", Name: "work", Args: json.RawMessage(`{}`)})}
	first.Usage = &llmtypes.Usage{Input: 100, Output: 5, CacheRead: 20, TotalTokens: 105, Cost: &llmtypes.Cost{Input: 0.01, Output: 0.02, Total: 0.03}}
	last := assistantMsg("done", llmtypes.StopReasonStop)
	last.Usage = &llmtypes.Usage{Input: 120, Output: 7, TotalTokens: 127, Cost: &llmtypes.Cost{Input: 0.02, Output: 0.04, Total: 0.06}}
	tool := NewFuncTool("work", "work", nil, func(context.Context, json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`"ok"`), nil })
	events := runTestLoop(t, []llmtypes.AgentMessage{llmtypes.UserMsg("run")}, AgentContext{Tools: []Tool{tool}}, LoopConfig{Model: mockModel(first, last), MaxTurns: 3})
	end, ok := findEvent(events, EventAgentEnd)
	if !ok || end.Summary == nil {
		t.Fatal("missing summary")
	}
	s := end.Summary
	if len(s.Turns) != 2 || s.Turns[0].Index != 1 || s.Turns[1].Index != 2 || s.Turns[0].Usage.Input != 100 || s.Turns[1].Usage.Output != 7 {
		t.Fatalf("turns=%+v", s.Turns)
	}
	if s.Usage.Input != 220 || s.Usage.Output != 12 || s.Usage.TotalTokens != 232 || s.Usage.Cost == nil || s.Usage.Cost.Total != 0.09 {
		t.Fatalf("total=%+v cost=%+v", s.Usage, s.Usage.Cost)
	}
	if first.Usage.Cost.Total != 0.03 || last.Usage.Cost.Total != 0.06 {
		t.Fatal("aggregation mutated per-turn costs")
	}
}
