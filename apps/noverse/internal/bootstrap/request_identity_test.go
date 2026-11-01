package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	llm "github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
)

type identityTestModel struct {
	generate func() (*llm.LLMResponse, error)
	stream   func() (<-chan llm.StreamEvent, error)
}

func (m identityTestModel) SupportsTools() bool { return true }
func (m identityTestModel) Generate(context.Context, []llm.Message, []llm.ToolSpec, ...llm.CallOption) (*llm.LLMResponse, error) {
	return m.generate()
}
func (m identityTestModel) GenerateStream(context.Context, []llm.Message, []llm.ToolSpec, ...llm.CallOption) (<-chan llm.StreamEvent, error) {
	return m.stream()
}

func TestModelRequestIdentitySurvivesSwapBackAndErrorChain(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := errors.New("actual A failure")
	model := identityTestModel{generate: func() (*llm.LLMResponse, error) { return nil, root }}
	slot := NewSwappableModel("a", "a-model", model, nil)
	_, err := slot.Generate(context.Background(), nil, nil)
	var request *ModelRequestError
	if !errors.As(err, &request) || !errors.Is(err, root) || !request.IsCurrent() {
		t.Fatalf("lost current request chain: %v", err)
	}
	slot.Swap("b", "b-model", model, nil)
	slot.Swap("a", "a-model", model, nil)
	if request.IsCurrent() || request.Provider != "a" || request.Model != "a-model" {
		t.Fatalf("old generation mistaken for current: %+v", request)
	}
}

func TestModelRequestIdentityStreamPreservesOrderAndCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := make(chan llm.StreamEvent, 3)
	root := errors.New("late stream error")
	source <- llm.StreamEvent{Type: llm.StreamEventTextDelta, Delta: "first"}
	source <- llm.StreamEvent{Type: llm.StreamEventTextDelta, Delta: "second"}
	source <- llm.StreamEvent{Type: llm.StreamEventError, Err: root}
	close(source)
	model := identityTestModel{stream: func() (<-chan llm.StreamEvent, error) { return source, nil }}
	slot := NewSwappableModel("a", "a-model", model, nil)
	stream, err := slot.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	slot.Swap("b", "b-model", model, nil)
	var events []llm.StreamEvent
	for event := range stream {
		events = append(events, event)
	}
	if len(events) != 3 || events[0].Delta != "first" || events[1].Delta != "second" {
		t.Fatalf("stream changed: %+v", events)
	}
	var request *ModelRequestError
	if !errors.As(events[2].Err, &request) || !errors.Is(events[2].Err, root) || request.IsCurrent() {
		t.Fatalf("async identity lost: %+v", events[2])
	}
	// Cancel while the producer is silent; forwarding must not block on receive.
	idle := make(chan llm.StreamEvent)
	model.stream = func() (<-chan llm.StreamEvent, error) { return idle, nil }
	slot.Swap("b", "b-model", model, nil)
	ctx, cancel := context.WithCancel(context.Background())
	stream, err = slot.GenerateStream(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case _, ok := <-stream:
		if ok {
			t.Fatal("cancelled stream emitted event")
		}
	case <-time.After(time.Second):
		t.Fatal("stream forwarder blocked after cancellation")
	}
	// Cancel while a chunk is waiting for its consumer, too.
	pending := make(chan llm.StreamEvent, 1)
	pending <- llm.StreamEvent{Type: llm.StreamEventTextDelta, Delta: "unread"}
	close(pending)
	model.stream = func() (<-chan llm.StreamEvent, error) { return pending, nil }
	slot.Swap("b", "b-model", model, nil)
	ctx, cancel = context.WithCancel(context.Background())
	stream, err = slot.GenerateStream(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-stream:
	case <-time.After(time.Second):
		t.Fatal("stream forwarder blocked on send")
	}
}

func TestRequestLevelFallbackErrorCarriesActualIdentityAndGeneration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":{"code":"rate_limit_exceeded","message":"too many requests"}}`)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		fmt.Fprint(w, `{"error":{"code":"insufficient_quota","message":"payment required"}}`)
	}))
	defer fallback.Close()
	cfg := Config{Provider: "a", ModelName: "a-model", Providers: map[string]llmconfig.ProviderConfig{
		"a": {Type: "openai", APIKey: "test", BaseURL: primary.URL + "/v1", Models: []llmconfig.ModelConfig{{Name: "a-model"}}},
		"b": {Type: "openai", APIKey: "test", BaseURL: fallback.URL + "/v1", Models: []llmconfig.ModelConfig{{Name: "b-model"}}},
	}, Roles: map[string]roles.RoleConfig{"writer": {Provider: "a", Model: "a-model", Fallbacks: []roles.ModelRef{{Provider: "b", Model: "b-model"}}}}}
	models, err := NewModelSet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	model := models.ForRoleWithFailover("writer", nil)
	_, err = model.Generate(context.Background(), []llm.Message{llm.UserMsg("test")}, nil)
	var request *ModelRequestError
	if !errors.As(err, &request) || request.Provider != "b" || request.Model != "b-model" || !request.IsCurrent() {
		t.Fatalf("fallback identity incorrect: %v (%+v)", err, request)
	}
	next, err := NewModelSet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	models.ApplyPrepared(next)
	if request.IsCurrent() {
		t.Fatal("fallback error survived selection generation cutover")
	}
}
