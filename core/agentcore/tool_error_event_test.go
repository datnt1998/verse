package agentcore

import (
	"encoding/json"
	"reflect"
	"testing"

	llmtypes "github.com/datnt1998/verse/core/llm"
)

func TestToolErrorKindReachesEndEvent(t *testing.T) {
	vt := &validatorTool{result: ValidationResult{OK: false, Message: "denied"}}
	events := runTestLoop(t, []llmtypes.AgentMessage{llmtypes.UserMsg("test")}, AgentContext{Tools: []Tool{vt}}, LoopConfig{Model: mockModel(toolCallMsg(llmtypes.ToolCall{ID: "tc", Name: "vtool", Args: json.RawMessage(`{"x":"y"}`)}), assistantMsg("done", llmtypes.StopReasonStop))})
	ev, ok := findEvent(events, EventToolExecEnd)
	if !ok || !ev.IsError {
		t.Fatalf("missing error event: %+v", ev)
	}
	kind := reflect.ValueOf(ev).FieldByName("ErrorKind")
	if !kind.IsValid() || kind.String() != ToolErrorBusiness {
		t.Fatalf("missing structured error kind: %+v", ev)
	}
}
