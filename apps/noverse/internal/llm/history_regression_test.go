package llm

import (
	"context"
	"encoding/json"
	"testing"
)

func TestModelRepairsPoisonedHistory(t *testing.T) {
	p := &captureProvider{}
	m := NewClientModel("m", mustClient(t, p))
	msgs := []Message{UserMsg("request"), {Role: RoleAssistant, Content: []ContentBlock{
		ToolCallBlock(ToolCall{ID: "a", Name: "work"}), ToolCallBlock(ToolCall{ID: "a", Args: json.RawMessage(`{"n":1}`)}),
	}}, ToolResultMsg("a", json.RawMessage(`"first"`), false), ToolResultMsg("a", json.RawMessage(`"duplicate"`), true)}
	if _, err := m.Generate(context.Background(), msgs, nil); err != nil {
		t.Fatal(err)
	}
	if len(msgs[1].Content) != 2 || len(msgs) != 4 {
		t.Fatal("history repair mutated caller transcript")
	}
}
