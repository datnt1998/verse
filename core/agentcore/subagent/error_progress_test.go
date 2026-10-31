package subagent

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/core/agentcore"
	llmtypes "github.com/datnt1998/verse/core/llm"
)

func TestRunnerRelaysStructuredToolErrorKind(t *testing.T) {
	model := newSequential(func(i int, _ *llmtypes.LLMRequest) (*llmtypes.LLMResponse, error) {
		if i == 0 {
			return &llmtypes.LLMResponse{Message: llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "tc", Name: "echo", Args: json.RawMessage(`{}`)})}, StopReason: llmtypes.StopReasonToolUse}}, nil
		}
		return &llmtypes.LLMResponse{Message: llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("done")}, StopReason: llmtypes.StopReasonStop}}, nil
	})
	echo := agentcore.NewFuncTool("echo", "echo", map[string]any{"type": "object"}, func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, agentcore.ErrToolValidation
	})
	var got string
	ctx := agentcore.WithToolProgress(context.Background(), func(p agentcore.ProgressPayload) {
		if p.Kind == agentcore.ProgressToolError {
			v := reflect.ValueOf(p).FieldByName("ErrorKind")
			if v.IsValid() {
				got = v.String()
			}
		}
	})
	_, err := NewRunner(Config{Name: "writer", Model: model, MaxTurns: 3, Tools: []agentcore.Tool{echo}}).Run(ctx, "writer", "go")
	if err != nil {
		t.Fatal(err)
	}
	if got != agentcore.ToolErrorValidation {
		t.Fatalf("error kind=%q", got)
	}
}
