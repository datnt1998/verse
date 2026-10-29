package context

import (
	"github.com/datnt1998/noverse/internal/agent"
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func compactModel(text string) stubModel {
	return stubModel{generate: func(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
		return &llmtypes.LLMResponse{Message: llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock(text)}}}, nil
	}}
}
func TestCompactionRegressionThreshold(t *testing.T) {
	msgs := []llmtypes.AgentMessage{llmtypes.UserMsg("original"), llmtypes.UserMsg(strings.Repeat("a", 800)), llmtypes.UserMsg("latest")}
	tokens := EstimateContextTokens(msgs).Tokens
	for _, delta := range []int{0, -1} {
		t.Run(string(rune('a'-delta)), func(t *testing.T) {
			e := NewEngine(EngineConfig{ContextWindow: tokens + 20 + delta, ReserveTokens: 20, Strategies: []Strategy{NewLightTrim(LightTrimConfig{KeepRecent: 1, TextThreshold: 100, PreserveHead: 10, PreserveTail: 10})}})
			got, err := e.Project(context.Background(), msgs)
			if err != nil {
				t.Fatal(err)
			}
			changed := !reflect.DeepEqual(got.Messages, msgs)
			if changed != (delta < 0) {
				t.Fatalf("at threshold delta=%d changed=%v", delta, changed)
			}
		})
	}
}
func TestCompactionRegressionRepeatedSummaryAndRecent(t *testing.T) {
	e := NewEngine(EngineConfig{ContextWindow: 300, ReserveTokens: 50, CommitStrategies: []string{"full_summary"}, Strategies: []Strategy{NewFullSummary(FullSummaryConfig{Model: compactModel("checkpoint"), KeepRecentTokens: 1})}})
	msgs := []llmtypes.AgentMessage{llmtypes.UserMsg("original"), llmtypes.UserMsg(strings.Repeat("old ", 500)), llmtypes.UserMsg("latest")}
	for round := 0; round < 3; round++ {
		got, err := e.Project(context.Background(), msgs)
		if err != nil {
			t.Fatal(err)
		}
		if !got.ShouldCommit {
			t.Fatal("summary not committed")
		}
		if _, ok := got.Messages[1].(ContextSummary); !ok {
			t.Fatalf("summary not after original request: %T", got.Messages[0])
		}
		if got.Messages[len(got.Messages)-1].TextContent() != "latest" {
			t.Fatal("recent turn lost")
		}
		if EstimateTotal(got.Messages) >= 250 {
			t.Fatalf("compacted payload exceeds threshold: %d", EstimateTotal(got.Messages))
		}
		again, err := e.Project(context.Background(), got.Messages)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(again.Messages, got.Messages) {
			t.Fatal("no-pressure repeated projection changed context")
		}
		msgs = append(got.Messages, llmtypes.UserMsg(strings.Repeat("new ", 500)), llmtypes.UserMsg("latest"))
	}
}
func TestCompactionRegressionCancelDuringSummary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := stubModel{generate: func(ctx context.Context, _ []llmtypes.Message, _ []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	e := NewEngine(EngineConfig{ContextWindow: 300, ReserveTokens: 50, Strategies: []Strategy{NewFullSummary(FullSummaryConfig{Model: model, KeepRecentTokens: 1})}})
	msgs := []llmtypes.AgentMessage{llmtypes.UserMsg("original"), llmtypes.UserMsg(strings.Repeat("old", 1000)), llmtypes.UserMsg("latest")}
	_, err := e.Project(ctx, msgs)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if !reflect.DeepEqual(e.snapshotTranscript(), msgs) {
		t.Fatal("cancel mutated baseline")
	}
}
func TestCompactionRegressionParallelToolPairs(t *testing.T) {
	msgs := []llmtypes.AgentMessage{llmtypes.UserMsg("old"), llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "a", Name: "read"}), llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "b", Name: "read"})}}, llmtypes.ToolResultMsg("b", []byte(`"b"`), false), llmtypes.ToolResultMsg("a", []byte(`"a"`), false), llmtypes.UserMsg("latest")}
	for keep := 1; keep < EstimateTotal(msgs); keep++ {
		cut := FindSummaryCut(msgs, keep).FirstKeptIndex
		if cut > 1 && cut < 4 {
			t.Fatalf("cut=%d splits parallel call/result group", cut)
		}
	}
}

func TestCompactionKeepsPinnedPrefix(t *testing.T) {
	msgs := []llmtypes.AgentMessage{llmtypes.SystemMsg("system rules"), llmtypes.UserMsg("original request"), llmtypes.UserMsg(strings.Repeat("old ", 500)), llmtypes.UserMsg("latest")}
	e := NewEngine(EngineConfig{ContextWindow: 300, ReserveTokens: 50, Strategies: []Strategy{NewFullSummary(FullSummaryConfig{Model: compactModel("checkpoint"), KeepRecentTokens: 1})}})
	got, err := e.Project(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) < 4 || got.Messages[0].TextContent() != "system rules" || got.Messages[1].TextContent() != "original request" {
		t.Fatalf("pinned prefix lost: %+v", got.Messages)
	}
	if _, ok := got.Messages[2].(ContextSummary); !ok {
		t.Fatalf("summary inserted at wrong position: %T", got.Messages[2])
	}
}

func TestLightTrimKeepsSystemAndFirstMessage(t *testing.T) {
	msgs := []llmtypes.AgentMessage{llmtypes.SystemMsg(strings.Repeat("system", 200)), llmtypes.UserMsg(strings.Repeat("first", 200)), llmtypes.UserMsg(strings.Repeat("old", 1000)), llmtypes.UserMsg("latest")}
	s := NewLightTrim(LightTrimConfig{KeepRecent: 1, TextThreshold: 100, PreserveHead: 10, PreserveTail: 10})
	got, _, err := s.Apply(context.Background(), msgs, msgs, Budget{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if !reflect.DeepEqual(got[i], msgs[i]) {
			t.Fatalf("pinned message %d was trimmed", i)
		}
	}
	if reflect.DeepEqual(got[2], msgs[2]) {
		t.Fatal("old unpinned message was not trimmed")
	}
}

func TestCompactionNonadjacentToolResult(t *testing.T) {
	msgs := []llmtypes.AgentMessage{llmtypes.UserMsg("request"), llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "a", Name: "read"})}}, llmtypes.UserMsg(strings.Repeat("steer", 100)), llmtypes.ToolResultMsg("a", []byte(`"ok"`), false), llmtypes.UserMsg("latest")}
	cut := FindSummaryCut(msgs, EstimateTokens(msgs[4])+EstimateTokens(msgs[3])+1).FirstKeptIndex
	if cut > 1 && cut <= 3 {
		t.Fatalf("cut %d splits tool call/result a across steering", cut)
	}
}

func TestCompactionRejectsCanceledStrategyResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := stubModel{generate: func(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
		cancel()
		return &llmtypes.LLMResponse{Message: llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("checkpoint")}}}, nil
	}}
	e := NewEngine(EngineConfig{ContextWindow: 300, ReserveTokens: 50, Strategies: []Strategy{NewFullSummary(FullSummaryConfig{Model: model, KeepRecentTokens: 1})}})
	_, err := e.Project(ctx, []llmtypes.AgentMessage{llmtypes.UserMsg("first"), llmtypes.UserMsg(strings.Repeat("old ", 500)), llmtypes.UserMsg("latest")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled rewrite was accepted: %v", err)
	}
}

func TestCompactionRejectsOversizedPayload(t *testing.T) {
	e := NewEngine(EngineConfig{ContextWindow: 300, ReserveTokens: 50, Strategies: []Strategy{NewFullSummary(FullSummaryConfig{Model: compactModel(strings.Repeat("summary ", 1000)), KeepRecentTokens: 1})}})
	_, err := e.Project(context.Background(), []llmtypes.AgentMessage{llmtypes.UserMsg("first"), llmtypes.UserMsg(strings.Repeat("old ", 500)), llmtypes.UserMsg("latest")})
	if !llmtypes.IsContextOverflow(err) {
		t.Fatalf("oversized rewritten payload was accepted: %v", err)
	}
}

func TestCompactionBreakerNeverSendsOversizedPayload(t *testing.T) {
	e := NewEngine(EngineConfig{ContextWindow: 100, ReserveTokens: 1, MaxConsecutiveFailures: 1, Strategies: []Strategy{&failingStrategy{}}})
	msgs := []llmtypes.AgentMessage{llmtypes.UserMsg(strings.Repeat("old ", 500))}
	if _, err := e.Project(context.Background(), msgs); err == nil {
		t.Fatal("expected strategy failure")
	}
	if _, err := e.Project(context.Background(), msgs); !llmtypes.IsContextOverflow(err) {
		t.Fatalf("breaker bypass sent oversized context: %v", err)
	}
}

func TestCompactionCancellationDoesNotTripBreaker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := NewEngine(EngineConfig{ContextWindow: 100, Strategies: []Strategy{&failingStrategy{}}})
	_, err := e.Project(ctx, []llmtypes.AgentMessage{llmtypes.UserMsg("task")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if e.ConsecutiveFailures() != 0 {
		t.Fatalf("cancellation counted as %d compaction failures", e.ConsecutiveFailures())
	}
}

func TestCompactionWithoutStrategiesRejectsOversizedPayload(t *testing.T) {
	e := NewEngine(EngineConfig{ContextWindow: 100, ReserveTokens: 1})
	_, err := e.Project(context.Background(), []llmtypes.AgentMessage{llmtypes.UserMsg(strings.Repeat("old ", 500))})
	if !llmtypes.IsContextOverflow(err) {
		t.Fatalf("oversized context without strategies was accepted: %v", err)
	}
}

func TestContextEstimateIncludesLatestAssistantOutput(t *testing.T) {
	msg := llmtypes.Message{Role: llmtypes.RoleAssistant, Usage: &llmtypes.Usage{Input: 100, Output: 50, CacheWrite: 20, TotalTokens: 150}}
	if got := EstimateContextTokens([]llmtypes.AgentMessage{msg}).Tokens; got != 170 {
		t.Fatalf("latest assistant output missing from next prompt: got %d, want 170", got)
	}
}

func TestCompactionCheckpointInvalidatesOldUsage(t *testing.T) {
	e := NewEngine(EngineConfig{ContextWindow: 300, ReserveTokens: 50, Strategies: []Strategy{NewFullSummary(FullSummaryConfig{Model: compactModel("checkpoint"), KeepRecentTokens: 3})}})
	msgs := []llmtypes.AgentMessage{llmtypes.UserMsg("first"), llmtypes.UserMsg(strings.Repeat("a", 4000)), llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("done")}, Usage: &llmtypes.Usage{Input: 1010, Output: 1, TotalTokens: 1011}}, llmtypes.UserMsg("latest")}
	got, err := e.Project(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.Project(context.Background(), got.Messages)
	if err != nil {
		t.Fatalf("checkpoint reused pre-compaction usage: %v", err)
	}
	if !reflect.DeepEqual(got.Messages, again.Messages) {
		t.Fatal("stable compacted context changed on repeat")
	}
}

func TestCompactionBudgetIncludesStaticPrefix(t *testing.T) {
	for _, kind := range []string{"system", "tools"} {
		t.Run(kind, func(t *testing.T) {
			e := NewEngine(EngineConfig{ContextWindow: 100, ReserveTokens: 1})
			prefix := agentcore.LLMPrefix{}
			if kind == "system" {
				prefix.System = []llmtypes.Message{llmtypes.SystemMsg(strings.Repeat("s", 2000))}
			} else {
				prefix.Tools = []llmtypes.ToolSpec{{Name: "work", Description: strings.Repeat("d", 2000), Parameters: map[string]any{"type": "object"}}}
			}
			e.SetForkPrefix(prefix)
			_, err := e.Project(context.Background(), []llmtypes.AgentMessage{llmtypes.UserMsg("task")})
			if !llmtypes.IsContextOverflow(err) {
				t.Fatalf("oversized %s prefix ignored: %v", kind, err)
			}
		})
	}
}
