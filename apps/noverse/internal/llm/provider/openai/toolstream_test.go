package openai

import (
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/llm/wire"
)

func TestResponsesParallelLateIdentity(t *testing.T) {
	lines := []string{
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"item-a","type":"function_call","call_id":"call-a","name":""}}`, ``,
		`event: response.function_call_arguments.delta`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"item-a","output_index":0,"delta":"{\"n\":1}"}`, ``,
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","output_index":1,"item":{"id":"item-b","type":"function_call","call_id":"call-b","name":""}}`, ``,
		`event: response.function_call_arguments.delta`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"item-b","output_index":1,"delta":"{\"n\":2}"}`, ``,
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"item-a","type":"function_call","call_id":"call-a","name":"first","arguments":"{\"n\":1}"}}`, ``,
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","output_index":1,"item":{"id":"item-b","type":"function_call","call_id":"call-b","name":"second","arguments":"{\"n\":2}"}}`, ``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"model":"test","status":"completed","usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`, ``,
	}
	s := newResponsesStream(streamResponse(strings.Join(lines, "\n")), "test")
	defer s.Close()
	r, err := litellm.Collect(s)
	if err != nil {
		t.Fatal(err)
	}
	calls := r.ToolCalls()
	if len(calls) != 2 {
		t.Fatalf("calls=%+v", calls)
	}
	for i, want := range []struct{ id, name, args string }{{"call-a", "first", `{"n":1}`}, {"call-b", "second", `{"n":2}`}} {
		if calls[i].ID != want.id || calls[i].Name != want.name || string(calls[i].Arguments) != want.args {
			t.Fatalf("call[%d]=%+v", i, calls[i])
		}
	}
}
