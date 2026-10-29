package opencodego

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/llm/provider/compat"
	"github.com/datnt1998/verse/apps/noverse/internal/llm/wire"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

type captured struct {
	session string
	body    map[string]any
}

func newCapturingProvider(t *testing.T, sink *[]captured) *compat.Provider {
	t.Helper()
	p, err := New(compat.Config{
		APIKey:  "key",
		BaseURL: "https://opencode-go.test",
		HTTPClient: roundTripFunc(func(httpReq *http.Request) (*http.Response, error) {
			var body map[string]any
			if err := json.NewDecoder(httpReq.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			*sink = append(*sink, captured{session: httpReq.Header.Get("x-opencode-session"), body: body})
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))}, nil
		}),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func TestEveryRequestCarriesRoutingSession(t *testing.T) {
	var calls []captured
	p := newCapturingProvider(t, &calls)
	chat := func(options litellm.ProviderOptions) {
		t.Helper()
		if _, err := p.Chat(context.Background(), &litellm.Request{
			Model:           "kimi-k2.7-code",
			Messages:        []litellm.Message{litellm.UserText("hi")},
			ProviderOptions: options,
		}); err != nil {
			t.Fatalf("Chat: %v", err)
		}
	}
	chat(litellm.ProviderOptions{ProviderOptionPromptCacheKey: "book-writer"})
	chat(nil)
	chat(nil)
	chat(litellm.ProviderOptions{ProviderOptionSessionID: "explicit", ProviderOptionPromptCacheKey: "book-writer"})

	if calls[0].session != "book-writer" {
		t.Fatalf("cache key session = %q", calls[0].session)
	}
	if calls[1].session == "" || calls[1].session != calls[2].session {
		t.Fatalf("one-shot calls must reuse a stable fallback session, got %q and %q", calls[1].session, calls[2].session)
	}
	if calls[3].session != "explicit" {
		t.Fatalf("explicit session id must win, got %q", calls[3].session)
	}
	for i, call := range calls {
		for _, key := range []string{ProviderOptionSessionID, ProviderOptionPromptCacheKey} {
			if _, leaked := call.body[key]; leaked {
				t.Fatalf("call %d leaked %q into body: %#v", i, key, call.body)
			}
		}
	}
}

func TestThinkingMapsToReasoningEffort(t *testing.T) {
	var calls []captured
	p := newCapturingProvider(t, &calls)
	for _, thinking := range []*litellm.Thinking{
		{Mode: litellm.ThinkingDisabled},
		{Mode: litellm.ThinkingEnabled, Effort: "High"},
		{Mode: litellm.ThinkingEnabled},
	} {
		if _, err := p.Chat(context.Background(), &litellm.Request{Model: "deepseek-v4.1-flash", Messages: []litellm.Message{litellm.UserText("hi")}, Thinking: thinking}); err != nil {
			t.Fatalf("Chat(%+v): %v", thinking, err)
		}
	}
	if calls[0].body["reasoning_effort"] != "none" || calls[1].body["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort = %#v / %#v", calls[0].body["reasoning_effort"], calls[1].body["reasoning_effort"])
	}
	if _, set := calls[2].body["reasoning_effort"]; set {
		t.Fatalf("thinking without effort must keep the model default: %#v", calls[2].body)
	}

	budget := 2048
	for _, thinking := range []*litellm.Thinking{
		{Mode: litellm.ThinkingEnabled, BudgetTokens: &budget},
		{Mode: litellm.ThinkingEnabled, Effort: "extreme"},
	} {
		if _, err := p.Chat(context.Background(), &litellm.Request{Model: "deepseek-v4.1-flash", Messages: []litellm.Message{litellm.UserText("hi")}, Thinking: thinking}); err == nil {
			t.Fatalf("thinking %+v must be rejected before sending", thinking)
		}
	}
	if len(calls) != 3 {
		t.Fatalf("rejected thinking must not reach the network, sent %d requests", len(calls))
	}
}

func TestMiniMaxReasoningIsSplitFromContent(t *testing.T) {
	var calls []captured
	p := newCapturingProvider(t, &calls)
	for _, model := range []string{"minimax-m3", "kimi-k2.7-code"} {
		if _, err := p.Chat(context.Background(), &litellm.Request{Model: model, Messages: []litellm.Message{litellm.UserText("hi")}}); err != nil {
			t.Fatalf("Chat(%s): %v", model, err)
		}
	}
	if calls[0].body["reasoning_split"] != true {
		t.Fatalf("minimax request must ask for split reasoning: %#v", calls[0].body)
	}
	if _, set := calls[1].body["reasoning_split"]; set {
		t.Fatalf("non-minimax request must not carry reasoning_split: %#v", calls[1].body)
	}
}
