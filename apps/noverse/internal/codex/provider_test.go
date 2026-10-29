package codex

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/llm/wire"
)

type fakeTokens struct {
	mu          sync.Mutex
	current     Token
	next        Token
	invalidated []string
}

func (f *fakeTokens) Token(context.Context) (Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current, nil
}

func (f *fakeTokens) Invalidate(_ context.Context, stale string) (Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invalidated = append(f.invalidated, stale)
	f.current = f.next
	return f.current, nil
}

type capturedRequest struct {
	header http.Header
	body   map[string]any
}

const sseOK = "event: response.created\n" +
	`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}` + "\n\n" +
	"event: response.output_text.delta\n" +
	`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"Xin chào"}` + "\n\n" +
	"event: response.completed\n" +
	`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Xin chào"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}` + "\n\n"

// newBackend giả lập backend Codex: 401 với token "expired", SSE với token khác.
func newBackend(t *testing.T) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	var mu sync.Mutex
	var captured []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		mu.Lock()
		captured = append(captured, capturedRequest{header: r.Header.Clone(), body: body})
		mu.Unlock()
		if r.URL.Path != "/backend-api/codex/responses" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") == "Bearer expired" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"message":"token expired"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sseOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func newTestProvider(t *testing.T, srv *httptest.Server, tokens TokenSource) *Provider {
	t.Helper()
	p, err := New(Config{BaseURL: srv.URL + "/backend-api/codex", Tokens: tokens,
		Headers:   map[string]string{"X-Custom": "1"},
		ExtraBody: map[string]any{"temperature": 0.8, "service_tier": "flex"}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func ptr[T any](v T) *T { return &v }

func TestChatAppliesCodexContract(t *testing.T) {
	srv, captured := newBackend(t)
	tokens := &fakeTokens{current: Token{Access: "tok", AccountID: "acct-1"}}
	p := newTestProvider(t, srv, tokens)

	resp, err := p.Chat(context.Background(), &litellm.Request{
		Model: "gpt-test",
		Messages: []litellm.Message{
			{Role: litellm.RoleSystem, Blocks: []litellm.Block{litellm.TextBlock{Text: "Bạn là nhà văn.", Cache: &litellm.CacheControl{}}}},
			{Role: litellm.RoleUser, Blocks: []litellm.Block{litellm.TextBlock{Text: "Viết một câu."}}},
		},
		MaxTokens:       ptr(512),
		Temperature:     ptr(0.7),
		TopP:            ptr(0.9),
		ProviderOptions: map[string]any{"session_id": "book-42"},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text() != "Xin chào" {
		t.Fatalf("Chat phải gom stream thành phản hồi đầy đủ, content=%q", resp.Text())
	}

	req := (*captured)[0]
	body := req.body
	if body["store"] != false || body["stream"] != true {
		t.Fatalf("store/stream = %v/%v, want false/true", body["store"], body["stream"])
	}
	for _, key := range []string{"max_output_tokens", "temperature", "top_p", "session_id", "prompt_cache_retention"} {
		if _, ok := body[key]; ok {
			t.Fatalf("body còn tham số Codex từ chối %q: %v", key, body)
		}
	}
	if body["service_tier"] != "flex" {
		t.Fatalf("extra_body hợp lệ phải được gửi kèm, got %v", body["service_tier"])
	}
	if body["instructions"] != "Bạn là nhà văn." {
		t.Fatalf("system prompt phải thành instructions (kể cả khi có cache breakpoint), got %v", body["instructions"])
	}
	for _, item := range body["input"].([]any) {
		if m, _ := item.(map[string]any); m["role"] == "developer" || m["role"] == "system" {
			t.Fatalf("system prompt bị lặp trong input: %v", body["input"])
		}
	}
	if include, _ := json.Marshal(body["include"]); !strings.Contains(string(include), "reasoning.encrypted_content") {
		t.Fatalf("include thiếu reasoning.encrypted_content: %s", include)
	}

	h := req.header
	for name, want := range map[string]string{
		"Authorization":      "Bearer tok",
		"Chatgpt-Account-Id": "acct-1",
		"Originator":         "ainovel",
		"Version":            clientVersion,
		"Session_id":         "book-42",
		"Accept":             "text/event-stream",
		"X-Custom":           "1",
	} {
		if got := h.Get(name); got != want {
			t.Fatalf("header %s = %q, want %q", name, got, want)
		}
	}
}

func TestStreamRefreshesTokenOnceOn401(t *testing.T) {
	srv, captured := newBackend(t)
	tokens := &fakeTokens{current: Token{Access: "expired", AccountID: "acct"}, next: Token{Access: "fresh", AccountID: "acct"}}
	p := newTestProvider(t, srv, tokens)

	resp, err := p.Chat(context.Background(), &litellm.Request{Model: "gpt-test",
		Messages: []litellm.Message{{Role: litellm.RoleUser, Blocks: []litellm.Block{litellm.TextBlock{Text: "hi"}}}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text() != "Xin chào" {
		t.Fatalf("content = %q", resp.Text())
	}
	if len(tokens.invalidated) != 1 || tokens.invalidated[0] != "expired" {
		t.Fatalf("invalidated = %v, want [expired]", tokens.invalidated)
	}
	if len(*captured) != 2 || (*captured)[1].header.Get("Authorization") != "Bearer fresh" {
		t.Fatalf("phải gửi lại đúng một lần với token mới, requests=%d", len(*captured))
	}
	// Body gửi lại phải là body đầy đủ, không rỗng sau lần đọc đầu.
	if (*captured)[1].body["model"] != "gpt-test" {
		t.Fatalf("body lần gửi lại bị mất: %v", (*captured)[1].body)
	}
}

func TestNormalizeInputDropsServerItemReferences(t *testing.T) {
	got := normalizeInput([]any{
		map[string]any{"type": "item_reference", "id": "rs_1"},
		map[string]any{"type": "reasoning", "id": "rs_2", "encrypted_content": "x"},
		map[string]any{"type": "function_call", "id": "fc_1", "call_id": "toolu_01|abc", "name": "f"},
		map[string]any{"type": "function_call_output", "call_id": "toolu_01|abc", "output": "ok"},
	}).([]any)
	if len(got) != 3 {
		t.Fatalf("item_reference phải bị bỏ (store=false), got %v", got)
	}
	for _, raw := range got {
		if _, ok := raw.(map[string]any)["id"]; ok {
			t.Fatalf("id item phát lại phải bị bỏ: %v", raw)
		}
	}
	call, output := got[1].(map[string]any)["call_id"], got[2].(map[string]any)["call_id"]
	if call != output {
		t.Fatalf("call_id của function_call (%v) và output (%v) phải khớp sau chuẩn hoá", call, output)
	}

	str := normalizeInput("hello").([]any)
	if len(str) != 1 || str[0].(map[string]any)["role"] != "user" {
		t.Fatalf("input dạng chuỗi phải thành message user, got %v", str)
	}
}

func TestSanitizeCallID(t *testing.T) {
	valid := regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	for _, id := range []string{"call_abc", "toolu_01|x/y", strings.Repeat("a", 100), "|||"} {
		got := sanitizeCallID(id)
		if !valid.MatchString(got) {
			t.Fatalf("sanitizeCallID(%q) = %q không hợp lệ", id, got)
		}
		if got != sanitizeCallID(id) {
			t.Fatalf("sanitizeCallID(%q) không tất định", id)
		}
	}
	if sanitizeCallID("call_abc") != "call_abc" {
		t.Fatal("call_id hợp lệ phải giữ nguyên")
	}
	if sanitizeCallID("a|b") == sanitizeCallID("a/b") {
		t.Fatal("hai id khác nhau không được trùng sau chuẩn hoá")
	}
}

func TestParseModelsHidesAndSortsByPriority(t *testing.T) {
	models, err := parseModels([]byte(`{"models":[
		{"slug":"b","priority":5,"visibility":"list","context_window":272000},
		{"slug":"hidden","priority":1,"visibility":"hide"},
		{"slug":"a","priority":2,"display_name":"A"},
		{"slug":"","priority":0}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].Slug != "a" || models[1].Slug != "b" || models[1].ContextWindow != 272000 {
		t.Fatalf("models = %+v", models)
	}
}

// sseParallelCalls ghi lại lỗi production (tui.log 2026-10-26): một lượt writer
// phát hai function_call song song; call_id có ngay ở output_item.added nhưng
// name chỉ đầy đủ ở arguments.done/output_item.done. litellm v1.8.10 bỏ qua
// output_item.done và không truyền name của arguments.done khi đã có delta —
// kết quả là ToolCall name rỗng ("tool \"\" not found", rồi history bị
// "tool use … missing name").
const sseParallelCalls = "event: response.created\n" +
	`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}` + "\n\n" +
	"event: response.output_item.added\n" +
	`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"","arguments":"","status":"in_progress"}}` + "\n\n" +
	"event: response.function_call_arguments.delta\n" +
	`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"chapter\":"}` + "\n\n" +
	"event: response.function_call_arguments.delta\n" +
	`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"2}"}` + "\n\n" +
	"event: response.function_call_arguments.done\n" +
	`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"name":"read_chapter","arguments":"{\"chapter\":2}"}` + "\n\n" +
	"event: response.output_item.done\n" +
	`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter","arguments":"{\"chapter\":2}","status":"completed"}}` + "\n\n" +
	"event: response.output_item.added\n" +
	`data: {"type":"response.output_item.added","output_index":1,"item":{"id":"fc_2","type":"function_call","call_id":"call_2","name":"","arguments":"","status":"in_progress"}}` + "\n\n" +
	"event: response.function_call_arguments.delta\n" +
	`data: {"type":"response.function_call_arguments.delta","item_id":"fc_2","output_index":1,"delta":"{\"chapter\":2,\"title\":\"x\"}"}` + "\n\n" +
	"event: response.function_call_arguments.done\n" +
	`data: {"type":"response.function_call_arguments.done","item_id":"fc_2","output_index":1,"name":"plan_chapter","arguments":"{\"chapter\":2,\"title\":\"x\"}"}` + "\n\n" +
	"event: response.output_item.done\n" +
	`data: {"type":"response.output_item.done","output_index":1,"item":{"id":"fc_2","type":"function_call","call_id":"call_2","name":"plan_chapter","arguments":"{\"chapter\":2,\"title\":\"x\"}","status":"completed"}}` + "\n\n" +
	"event: response.completed\n" +
	`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed","output":[{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter","arguments":"{\"chapter\":2}"},{"id":"fc_2","type":"function_call","call_id":"call_2","name":"plan_chapter","arguments":"{\"chapter\":2,\"title\":\"x\"}"}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}` + "\n\n"

// newSSEBackend phục vụ một fixture SSE cố định thay vì sseOK.
func newSSEBackend(t *testing.T, sse string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestChatParallelToolCallsKeepNames: hai function_call song song phải giữ đủ
// name + call_id + arguments sau khi gom stream. Trước vá: litellm mất name
// (arguments.done không forward name khi đã stream delta) nên cả hai block
// đều name rỗng.
func TestChatParallelToolCallsKeepNames(t *testing.T) {
	srv := newSSEBackend(t, sseParallelCalls)
	p := newTestProvider(t, srv, &fakeTokens{current: Token{Access: "tok"}})

	resp, err := p.Chat(context.Background(), &litellm.Request{Model: "gpt-test",
		Messages: []litellm.Message{{Role: litellm.RoleUser, Blocks: []litellm.Block{litellm.TextBlock{Text: "hi"}}}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	var calls []litellm.ToolUseBlock
	for _, b := range resp.Blocks {
		if tc, ok := b.(litellm.ToolUseBlock); ok {
			calls = append(calls, tc)
		}
	}
	if len(calls) != 2 {
		t.Fatalf("want 2 tool use blocks, got %d: %#v", len(calls), resp.Blocks)
	}
	want := []struct{ id, name, args string }{
		{"call_1", "read_chapter", `{"chapter":2}`},
		{"call_2", "plan_chapter", `{"chapter":2,"title":"x"}`},
	}
	for i, w := range want {
		if calls[i].ID != w.id || calls[i].Name != w.name || string(calls[i].Arguments) != w.args {
			t.Fatalf("call[%d] = {id:%q name:%q args:%s}, want {id:%q name:%q args:%s}",
				i, calls[i].ID, calls[i].Name, calls[i].Arguments, w.id, w.name, w.args)
		}
	}
}

// TestStreamParallelToolCallsTriple: phía streaming (đường agentcore dùng) phải
// nhận đúng một bộ ba start/delta/done trên mỗi call, key khớp nhau (id+itemID)
// và delta mang toàn bộ arguments — agentcore khớp key "id:"+ID.
func TestStreamParallelToolCallsTriple(t *testing.T) {
	srv := newSSEBackend(t, sseParallelCalls)
	p := newTestProvider(t, srv, &fakeTokens{current: Token{Access: "tok"}})

	stream, err := p.Stream(context.Background(), &litellm.Request{Model: "gpt-test",
		Messages: []litellm.Message{{Role: litellm.RoleUser, Blocks: []litellm.Block{litellm.TextBlock{Text: "hi"}}}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer stream.Close()
	var starts []litellm.ToolUseStart
	var deltas []litellm.ToolUseDelta
	var dones []litellm.ToolUseDone
	for {
		ev, err := stream.Next()
		if err != nil {
			break
		}
		switch e := ev.(type) {
		case litellm.ToolUseStart:
			starts = append(starts, e)
		case litellm.ToolUseDelta:
			deltas = append(deltas, e)
		case litellm.ToolUseDone:
			dones = append(dones, e)
		}
	}
	if len(starts) != 2 || len(dones) != 2 {
		t.Fatalf("want 2 starts + 2 dones, got %d/%d (deltas %d)", len(starts), len(dones), len(deltas))
	}
	for i, w := range []struct{ id, name, args string }{
		{"call_1", "read_chapter", `{"chapter":2}`},
		{"call_2", "plan_chapter", `{"chapter":2,"title":"x"}`},
	} {
		if starts[i].ID != w.id || starts[i].Name != w.name || starts[i].ItemID == "" {
			t.Fatalf("start[%d] = %#v, want id=%q name=%q itemID set", i, starts[i], w.id, w.name)
		}
		var args []byte
		for _, d := range deltas {
			if d.ID == w.id {
				args = append(args, d.ArgumentsDelta...)
			}
		}
		if string(args) != w.args {
			t.Fatalf("deltas for %q = %s, want %s", w.id, args, w.args)
		}
		if dones[i].ID != w.id {
			t.Fatalf("done[%d].ID = %q, want %q", i, dones[i].ID, w.id)
		}
	}
}

// TestStreamToolCallEventOrders covers the wire orderings the backend actually
// emits for parallel calls: arguments before the name/added item, missing
// output_item.done, and an item.done that carries everything on its own.
func TestStreamToolCallEventOrders(t *testing.T) {
	created := "event: response.created\n" +
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}` + "\n\n"
	completed := "event: response.completed\n" +
		`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}` + "\n\n"

	tests := []struct {
		name string
		sse  string
		want []struct{ id, name, args string }
	}{
		{
			name: "delta trước added, thiếu output_item.done",
			sse: created +
				"event: response.function_call_arguments.delta\n" +
				`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"chapter\":2}"}` + "\n\n" +
				"event: response.output_item.added\n" +
				`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter"}}` + "\n\n" +
				"event: response.function_call_arguments.done\n" +
				`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"name":"read_chapter","arguments":"{\"chapter\":2}"}` + "\n\n" +
				completed,
			want: []struct{ id, name, args string }{{"call_1", "read_chapter", `{"chapter":2}`}},
		},
		{
			// Chuỗi thật gây lỗi: call_id chỉ xuất hiện ở added — nằm GIỮA hai
			// delta, nên litellm đổi ID của delta sau từ fc_1 sang call_1 và
			// agentcore tách thành hai block (name không args + args không name).
			name: "call_id xuất hiện giữa các delta",
			sse: created +
				"event: response.function_call_arguments.delta\n" +
				`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"chapter\":"}` + "\n\n" +
				"event: response.output_item.added\n" +
				`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter"}}` + "\n\n" +
				"event: response.function_call_arguments.delta\n" +
				`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"2}"}` + "\n\n" +
				"event: response.function_call_arguments.done\n" +
				`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"name":"read_chapter","arguments":"{\"chapter\":2}"}` + "\n\n" +
				"event: response.output_item.done\n" +
				`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter","arguments":"{\"chapter\":2}","status":"completed"}}` + "\n\n" +
				completed,
			want: []struct{ id, name, args string }{{"call_1", "read_chapter", `{"chapter":2}`}},
		},
		{
			name: "name chỉ có ở added, arguments chỉ có ở done",
			sse: created +
				"event: response.output_item.added\n" +
				`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"plan_chapter"}}` + "\n\n" +
				"event: response.function_call_arguments.done\n" +
				`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"name":"plan_chapter","arguments":"{\"chapter\":2}"}` + "\n\n" +
				"event: response.output_item.done\n" +
				`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"plan_chapter","arguments":"{\"chapter\":2}","status":"completed"}}` + "\n\n" +
				completed,
			want: []struct{ id, name, args string }{{"call_1", "plan_chapter", `{"chapter":2}`}},
		},
		{
			name: "chỉ có output_item.done (không added/delta)",
			sse: created +
				"event: response.output_item.done\n" +
				`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"fc_9","type":"function_call","call_id":"call_9","name":"check_consistency","arguments":"{\"chapter\":2}","status":"completed"}}` + "\n\n" +
				completed,
			want: []struct{ id, name, args string }{{"call_9", "check_consistency", `{"chapter":2}`}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newSSEBackend(t, tc.sse)
			p := newTestProvider(t, srv, &fakeTokens{current: Token{Access: "tok"}})
			resp, err := p.Chat(context.Background(), &litellm.Request{Model: "gpt-test",
				Messages: []litellm.Message{{Role: litellm.RoleUser, Blocks: []litellm.Block{litellm.TextBlock{Text: "hi"}}}}})
			if err != nil {
				t.Fatalf("Chat: %v", err)
			}
			var calls []litellm.ToolUseBlock
			for _, b := range resp.Blocks {
				if tc, ok := b.(litellm.ToolUseBlock); ok {
					calls = append(calls, tc)
				}
			}
			if len(calls) != len(tc.want) {
				t.Fatalf("calls = %d, want %d: %#v", len(calls), len(tc.want), resp.Blocks)
			}
			for i, w := range tc.want {
				if calls[i].ID != w.id || calls[i].Name != w.name || string(calls[i].Arguments) != w.args {
					t.Fatalf("call[%d] = {id:%q name:%q args:%s}, want %+v", i, calls[i].ID, calls[i].Name, calls[i].Arguments, w)
				}
			}
		})
	}
}

// TestAdapterAssemblesParallelToolCalls chạy đúng đường production:
// provider Codex → litellm.Client → agentcore ClientModel.GenerateStream.
// Trước vá, agentcore nhận hai ToolCall name rỗng (log: "tool \"\" not found"
// rồi "tool use … missing name"); sau vá phải đủ name/args cho cả hai.
func TestAdapterAssemblesParallelToolCalls(t *testing.T) {
	srv := newSSEBackend(t, sseParallelCalls)
	p := newTestProvider(t, srv, &fakeTokens{current: Token{Access: "tok"}})
	client, err := litellm.New(p)
	if err != nil {
		t.Fatal(err)
	}
	adapter := llmtypes.NewClientModel("gpt-test", client)

	events, err := adapter.GenerateStream(context.Background(), []llmtypes.Message{
		llmtypes.UserMsg("viết chương 2"),
	}, nil)
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	var final llmtypes.Message
	for ev := range events {
		if ev.Err != nil {
			t.Fatalf("stream error: %v", ev.Err)
		}
		if ev.Type == llmtypes.StreamEventDone {
			final = ev.Message
		}
	}
	calls := final.ToolCalls()
	if len(calls) != 2 {
		t.Fatalf("agentcore assembled %d tool calls, want 2: %#v", len(calls), final.Content)
	}
	want := []struct{ id, name, args string }{
		{"call_1", "read_chapter", `{"chapter":2}`},
		{"call_2", "plan_chapter", `{"chapter":2,"title":"x"}`},
	}
	for i, w := range want {
		if calls[i].ID != w.id || calls[i].Name != w.name || string(calls[i].Args) != w.args {
			t.Fatalf("tool call[%d] = %#v, want id=%q name=%q args=%s", i, calls[i], w.id, w.name, w.args)
		}
		if calls[i].ArgsInvalid {
			t.Fatalf("tool call[%d] args marked invalid: %#v", i, calls[i])
		}
	}
}

// TestAdapterKeepsCallTogetherWhenIDResolvesLate: đúng chuỗi production —
// call_id chỉ đến ở added nằm giữa hai delta. agentcore khoá tool call theo
// một key duy nhất (id:…), nên nếu ID đổi giữa chừng mà không chuẩn hoá thì
// name và arguments rơi vào hai block khác nhau ("tool \"\" not found").
func TestAdapterKeepsCallTogetherWhenIDResolvesLate(t *testing.T) {
	created := "event: response.created\n" +
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}` + "\n\n"
	sse := created +
		"event: response.function_call_arguments.delta\n" +
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"chapter\":"}` + "\n\n" +
		"event: response.output_item.added\n" +
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter"}}` + "\n\n" +
		"event: response.function_call_arguments.delta\n" +
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"2}"}` + "\n\n" +
		"event: response.function_call_arguments.done\n" +
		`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"name":"read_chapter","arguments":"{\"chapter\":2}"}` + "\n\n" +
		"event: response.output_item.done\n" +
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter","arguments":"{\"chapter\":2}","status":"completed"}}` + "\n\n" +
		"event: response.completed\n" +
		`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}` + "\n\n"

	srv := newSSEBackend(t, sse)
	p := newTestProvider(t, srv, &fakeTokens{current: Token{Access: "tok"}})
	client, err := litellm.New(p)
	if err != nil {
		t.Fatal(err)
	}
	adapter := llmtypes.NewClientModel("gpt-test", client)

	events, err := adapter.GenerateStream(context.Background(), []llmtypes.Message{llmtypes.UserMsg("hi")}, nil)
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	var final llmtypes.Message
	for ev := range events {
		if ev.Err != nil {
			t.Fatalf("stream error: %v", ev.Err)
		}
		if ev.Type == llmtypes.StreamEventDone {
			final = ev.Message
		}
	}
	calls := final.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("agentcore chia một lời gọi thành %d block: %#v", len(calls), final.Content)
	}
	if calls[0].ID != "call_1" || calls[0].Name != "read_chapter" || string(calls[0].Args) != `{"chapter":2}` {
		t.Fatalf("tool call = %#v, want id/name/args đầy đủ", calls[0])
	}
}

// TestStreamDropsTruncatedToolCall: stream gãy giữa arguments (không có
// output_item.done/response.completed) không được phát thành một ToolUse hoàn
// chỉnh — nếu không worker sẽ chạy công cụ bằng arguments cụt.
func TestStreamDropsTruncatedToolCall(t *testing.T) {
	sse := "event: response.created\n" +
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}` + "\n\n" +
		"event: response.output_item.added\n" +
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"draft_chapter"}}` + "\n\n" +
		"event: response.function_call_arguments.delta\n" +
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"content\":\"nửa chươ"}` + "\n\n"

	srv := newSSEBackend(t, sse)
	p := newTestProvider(t, srv, &fakeTokens{current: Token{Access: "tok"}})
	stream, err := p.Stream(context.Background(), &litellm.Request{Model: "gpt-test",
		Messages: []litellm.Message{{Role: litellm.RoleUser, Blocks: []litellm.Block{litellm.TextBlock{Text: "hi"}}}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer stream.Close()
	var errSeen bool
	for {
		ev, err := stream.Next()
		if err != nil {
			errSeen = true
			break
		}
		switch ev.(type) {
		case litellm.ToolUseStart, litellm.ToolUseDelta, litellm.ToolUseDone:
			t.Fatalf("stream gãy vẫn phát %T — arguments cụt sẽ được dispatch", ev)
		}
	}
	if !errSeen {
		t.Fatal("stream gãy phải trả lỗi")
	}
}

// sseToolEvents dựng fixture SSE từ các sự kiện wire của backend Codex.
func sseToolEvents(events ...string) string {
	const created = "event: response.created\n" +
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}` + "\n\n"
	const completed = "event: response.completed\n" +
		`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}` + "\n\n"
	out := created
	for _, ev := range events {
		out += ev
	}
	return out + completed
}

func sseEvent(name, data string) string { return "event: " + name + "\ndata: " + data + "\n\n" }

// adapterToolCalls chạy đúng đường production (provider Codex → litellm.Client →
// agentcore ClientModel) và trả các ToolCall agentcore sẽ thực thi.
func adapterToolCalls(t *testing.T, sse string) []llmtypes.ToolCall {
	t.Helper()
	srv := newSSEBackend(t, sse)
	p := newTestProvider(t, srv, &fakeTokens{current: Token{Access: "tok"}})
	client, err := litellm.New(p)
	if err != nil {
		t.Fatal(err)
	}
	adapter := llmtypes.NewClientModel("gpt-test", client)
	events, err := adapter.GenerateStream(context.Background(), []llmtypes.Message{llmtypes.UserMsg("viết chương 2")}, nil)
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	var final llmtypes.Message
	for ev := range events {
		if ev.Err != nil {
			t.Fatalf("stream lỗi: %v", ev.Err)
		}
		if ev.Type == llmtypes.StreamEventDone {
			final = ev.Message
		}
	}
	return final.ToolCalls()
}

// TestStreamNeverEmitsNamelessToolCall: lời gọi không có name ở mọi sự kiện
// phải bị bỏ hẳn. Trước vá: agentcore dựng ToolCall{Name: ""} → chạy
// `tool "" not found` rồi ghi vào lịch sử, lượt sau bị litellm chặn
// ("tool use … missing name") — đúng vòng lặp đã dừng phiên writer.
func TestStreamNeverEmitsNamelessToolCall(t *testing.T) {
	sse := sseToolEvents(
		sseEvent("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"","arguments":"","status":"in_progress"}}`),
		sseEvent("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"name":"","arguments":"{\"chapter\":1}"}`),
		sseEvent("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"","arguments":"{\"chapter\":1}","status":"completed"}}`),
	)
	if calls := adapterToolCalls(t, sse); len(calls) != 0 {
		t.Fatalf("calls = %#v, want none (lời gọi thiếu name không được thực thi)", calls)
	}
}

// TestStreamMergesCallSplitAcrossItemIDs: output_item.added mang name với một
// item id, còn delta/arguments.done mang arguments dưới item_id KHÁC. Trước
// vá: hai ToolCall — một {name, args rỗng} và một {name rỗng, args đầy đủ}
// (chính là hình dạng đã lưu trong writer-ch02.jsonl: mỗi lời gọi hai block,
// block sau name rỗng → "tool \"\" not found" và history bị
// "duplicate tool use id"). Sau vá: một lời gọi đủ name + args.
func TestStreamMergesCallSplitAcrossItemIDs(t *testing.T) {
	sse := sseToolEvents(
		sseEvent("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter","arguments":"","status":"in_progress"}}`),
		sseEvent("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_9","output_index":0,"delta":"{\"chapter\":1,\"source\":\"final\"}"}`),
		sseEvent("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","item_id":"fc_9","output_index":0,"name":"","arguments":"{\"chapter\":1,\"source\":\"final\"}"}`),
		sseEvent("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter","arguments":"{\"chapter\":1,\"source\":\"final\"}","status":"completed"}}`),
	)
	calls := adapterToolCalls(t, sse)
	if len(calls) != 1 {
		t.Fatalf("calls = %d (%#v), want 1 — name và arguments phải gộp về một lời gọi", len(calls), calls)
	}
	if calls[0].ID != "call_1" || calls[0].Name != "read_chapter" || string(calls[0].Args) != `{"chapter":1,"source":"final"}` {
		t.Fatalf("call = %#v, want id=call_1 name=read_chapter args đầy đủ", calls[0])
	}
}

// TestStreamKeepsCallWhenEventsCarryOnlyOutputIndex: backend chỉ gửi
// output_index (không item_id/call_id) cho delta và arguments.done. Trước vá:
// wrapper bỏ mọi sự kiện vì không có khoá → cả lời gọi biến mất khỏi lượt.
func TestStreamKeepsCallWhenEventsCarryOnlyOutputIndex(t *testing.T) {
	sse := sseToolEvents(
		sseEvent("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"chapter\":1}"}`),
		sseEvent("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","output_index":0,"name":"read_chapter","arguments":"{\"chapter\":1}"}`),
	)
	calls := adapterToolCalls(t, sse)
	if len(calls) != 1 {
		t.Fatalf("calls = %d (%#v), want 1 — chỉ có output_index vẫn phải giữ lời gọi", len(calls), calls)
	}
	if calls[0].Name != "read_chapter" || string(calls[0].Args) != `{"chapter":1}` {
		t.Fatalf("call = %#v", calls[0])
	}
}

// TestStreamKeepsSecondCallWhenItemIDReused: backend dùng lại item id cho lời
// gọi kế tiếp trong cùng phản hồi. Trước vá: buffer đã phát vẫn hút sự kiện
// của lời gọi mới nên lời gọi thứ hai biến mất (worker mất lượt đọc chương).
func TestStreamKeepsSecondCallWhenItemIDReused(t *testing.T) {
	sse := sseToolEvents(
		sseEvent("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter"}}`),
		sseEvent("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"chapter\":1}"}`),
		sseEvent("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_chapter","arguments":"{\"chapter\":1}","status":"completed"}}`),
		sseEvent("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"fc_1","type":"function_call","call_id":"call_2","name":"plan_chapter"}}`),
		sseEvent("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":1,"delta":"{\"chapter\":2}"}`),
		sseEvent("response.output_item.done", `{"type":"response.output_item.done","output_index":1,"item":{"id":"fc_1","type":"function_call","call_id":"call_2","name":"plan_chapter","arguments":"{\"chapter\":2}","status":"completed"}}`),
	)
	calls := adapterToolCalls(t, sse)
	if len(calls) != 2 {
		t.Fatalf("calls = %d (%#v), want 2", len(calls), calls)
	}
	want := []struct{ id, name, args string }{
		{"call_1", "read_chapter", `{"chapter":1}`},
		{"call_2", "plan_chapter", `{"chapter":2}`},
	}
	for i, w := range want {
		if calls[i].ID != w.id || calls[i].Name != w.name || string(calls[i].Args) != w.args {
			t.Fatalf("call[%d] = %#v, want %+v", i, calls[i], w)
		}
	}
}
