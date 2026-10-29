package agents

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/core/agentcore"
	corecontext "github.com/datnt1998/verse/core/agentcore/context"
	"github.com/datnt1998/verse/core/agentcore/subagent"
	litellm "github.com/datnt1998/verse/core/llm/wire"
)

// Constructed provider transcripts cross the real provider collector, model
// adapter, agent loop, tool executor and subagent runner; no external API is used.
type replayProvider struct {
	streams [][]litellm.Event
	calls   atomic.Int32
	inspect func(*litellm.Request)
	wait    bool
}

func (p *replayProvider) Name() string                             { return "replay" }
func (p *replayProvider) Capabilities(string) litellm.Capabilities { return litellm.Capabilities{} }
func (p *replayProvider) Chat(context.Context, *litellm.Request) (*litellm.Response, error) {
	return nil, errors.New("unexpected non-stream call")
}
func (p *replayProvider) Stream(ctx context.Context, req *litellm.Request) (litellm.Stream, error) {
	if p.inspect != nil {
		p.inspect(req)
	}
	i := int(p.calls.Add(1)) - 1
	if i >= len(p.streams) {
		return nil, errors.New("unexpected extra provider turn")
	}
	return &replayStream{ctx: ctx, events: p.streams[i], wait: p.wait}, nil
}

type replayStream struct {
	ctx    context.Context
	events []litellm.Event
	wait   bool
}

func (s *replayStream) Close() error { return nil }
func (s *replayStream) Next() (litellm.Event, error) {
	if len(s.events) > 0 {
		e := s.events[0]
		s.events = s.events[1:]
		return e, nil
	}
	if s.wait {
		<-s.ctx.Done()
		return nil, s.ctx.Err()
	}
	return nil, io.EOF
}
func replayModel(t *testing.T, p *replayProvider) *llmtypes.ClientModel {
	t.Helper()
	c, err := litellm.New(p)
	if err != nil {
		t.Fatal(err)
	}
	return llmtypes.NewClientModel("replay-model", c)
}
func replayDone() litellm.Event {
	return litellm.DoneEvent{FinishReason: litellm.FinishReasonStop, Provider: "replay", Model: "replay-model"}
}

func TestProviderReplayStreamingTextAndSubagent(t *testing.T) {
	p := &replayProvider{streams: [][]litellm.Event{{litellm.ReasoningDelta{Text: "think"}, litellm.ContentDelta{Text: "hello "}, litellm.ContentDelta{Text: "world"}, litellm.UsageEvent{Usage: litellm.Usage{InputTokens: 12, OutputTokens: 3, TotalTokens: 15}}, replayDone()}}}
	r := subagent.NewRunner(subagent.Config{Name: "child", Model: replayModel(t, p), SystemPrompt: "system"})
	got, err := r.Run(context.Background(), "child", "task")
	if err != nil {
		t.Fatal(err)
	}
	if got.Output != "hello world" || got.Usage.Input != 12 || got.Usage.Output != 3 || got.Usage.Turns != 1 {
		t.Fatalf("result = %+v", got)
	}
}
func TestProviderReplayParallelToolCalls(t *testing.T) {
	var seen atomic.Int32
	tool := agentcore.NewFuncTool("work", "record", map[string]any{"type": "object"}, func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		if !json.Valid(args) {
			t.Errorf("invalid args: %s", args)
		}
		seen.Add(1)
		return json.RawMessage(`"ok"`), nil
	})
	p := &replayProvider{streams: [][]litellm.Event{{
		litellm.ToolUseStart{ID: "a", Name: "work"}, litellm.ToolUseStart{ID: "b", Name: "work"},
		litellm.ToolUseDelta{ID: "b", ArgumentsDelta: []byte(`{"n":2}`)}, litellm.ToolUseDelta{ID: "a", ArgumentsDelta: []byte(`{"n":1}`)},
		litellm.ToolUseDone{ID: "b"}, litellm.ToolUseDone{ID: "a"}, litellm.DoneEvent{FinishReason: litellm.FinishReasonToolCall, Provider: "replay", Model: "replay-model"},
	}, {litellm.ContentDelta{Text: "finished"}, replayDone()}}}
	msgs, err := agentcore.Collect(agentcore.AgentLoop(context.Background(), []llmtypes.AgentMessage{llmtypes.UserMsg("run")}, agentcore.AgentContext{Tools: []agentcore.Tool{tool}}, agentcore.LoopConfig{Model: replayModel(t, p), MaxTurns: 3, MaxToolConcurrency: 2}))
	if err != nil {
		t.Fatal(err)
	}
	if seen.Load() != 2 || p.calls.Load() != 2 {
		t.Fatalf("tools=%d calls=%d messages=%+v", seen.Load(), p.calls.Load(), msgs)
	}
}
func TestProviderReplayMidStreamCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &replayProvider{streams: [][]litellm.Event{{litellm.ContentDelta{Text: "partial"}}}, wait: true}
	for ev := range agentcore.AgentLoop(ctx, []llmtypes.AgentMessage{llmtypes.UserMsg("run")}, agentcore.AgentContext{}, agentcore.LoopConfig{Model: replayModel(t, p), MaxTurns: 1}) {
		if ev.Type == agentcore.EventMessageUpdate {
			cancel()
		}
	}
	if ctx.Err() == nil || p.calls.Load() != 1 {
		t.Fatalf("cancel=%v calls=%d", ctx.Err(), p.calls.Load())
	}
}
func TestProviderReplayContextTrim(t *testing.T) {
	var trimmed bool
	p := &replayProvider{streams: [][]litellm.Event{{litellm.ContentDelta{Text: "ok"}, replayDone()}}}
	p.inspect = func(req *litellm.Request) {
		for _, m := range req.Messages {
			for _, b := range m.Blocks {
				if text, ok := b.(litellm.TextBlock); ok && strings.Contains(text.Text, "characters trimmed") {
					trimmed = true
				}
			}
		}
	}
	manager := corecontext.NewEngine(corecontext.EngineConfig{ContextWindow: 200, ReserveTokens: 20, Strategies: []corecontext.Strategy{corecontext.NewLightTrim(corecontext.LightTrimConfig{KeepRecent: 1, TextThreshold: 100, PreserveHead: 10, PreserveTail: 10})}})
	_, err := agentcore.Collect(agentcore.AgentLoop(context.Background(), []llmtypes.AgentMessage{llmtypes.UserMsg("latest")}, agentcore.AgentContext{Messages: []llmtypes.AgentMessage{llmtypes.UserMsg("original"), llmtypes.UserMsg(strings.Repeat("old ", 1000))}}, agentcore.LoopConfig{Model: replayModel(t, p), ContextManager: manager, MaxTurns: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if !trimmed {
		t.Fatal("provider did not receive trimmed context")
	}
}
