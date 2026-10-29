package context

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/agent"
)

type stubModel struct {
	generate func(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error)
}

func (m stubModel) Generate(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return m.generate(ctx, messages, tools, opts...)
}

func (m stubModel) GenerateStream(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	return nil, context.Canceled
}

func (m stubModel) SupportsTools() bool { return true }

func TestFindCutPoint_SkipsToolResultBoundary(t *testing.T) {
	msgs := []llmtypes.AgentMessage{
		llmtypes.UserMsg("old"),
		llmtypes.Message{
			Role:    llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "tc1", Name: "read"})},
		},
		llmtypes.ToolResultMsg("tc1", []byte(`"ok"`), false),
		llmtypes.UserMsg("recent"),
	}

	cut := FindSummaryCut(msgs, 2)
	if cut.FirstKeptIndex != 3 {
		t.Fatalf("expected cut to advance past tool result to index 3, got %d", cut.FirstKeptIndex)
	}
	if cut.IsSplitTurn {
		t.Fatal("expected cut at user boundary, got split turn")
	}
}

func TestFindCutPoint_ReportsSplitTurn(t *testing.T) {
	msgs := []llmtypes.AgentMessage{
		llmtypes.UserMsg("old"),
		llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("done")}},
		llmtypes.UserMsg("current task"),
		llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("working")}},
	}

	cut := FindSummaryCut(msgs, 1)
	if cut.FirstKeptIndex != 3 {
		t.Fatalf("expected assistant message to be first kept item, got %d", cut.FirstKeptIndex)
	}
	if !cut.IsSplitTurn {
		t.Fatal("expected split turn to be reported")
	}
	if cut.TurnStartIndex != 2 {
		t.Fatalf("expected split turn to start at index 2, got %d", cut.TurnStartIndex)
	}
}

func TestExtractFileOps_DeduplicatesAndSeparates(t *testing.T) {
	msgs := []llmtypes.AgentMessage{
		llmtypes.Message{
			Role: llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{
				llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "1", Name: "read", Args: []byte(`{"path":"a.go"}`)}),
				llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "2", Name: "read", Args: []byte(`{"path":"b.go"}`)}),
				llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "3", Name: "edit", Args: []byte(`{"path":"b.go"}`)}),
				llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "4", Name: "write", Args: []byte(`{"path":"c.go"}`)}),
				llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "5", Name: "read", Args: []byte(`{"path":"a.go"}`)}),
			},
		},
	}

	readFiles, modifiedFiles := extractFileOps(msgs)
	if got := strings.Join(readFiles, ","); got != "a.go" {
		t.Fatalf("expected read-only files to be a.go, got %q", got)
	}
	if got := strings.Join(modifiedFiles, ","); got != "b.go,c.go" {
		t.Fatalf("expected modified files to be b.go,c.go, got %q", got)
	}
}

func TestRunSummaryCompaction_CompactsAndPreservesRecentMessages(t *testing.T) {
	model := stubModel{
		generate: func(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
			return &llmtypes.LLMResponse{
				Message: llmtypes.Message{
					Role:    llmtypes.RoleAssistant,
					Content: []llmtypes.ContentBlock{llmtypes.TextBlock("checkpoint body")},
				},
			}, nil
		},
	}

	cfg := summaryRunConfig{
		Model:            model,
		ContextWindow:    16,
		ReserveTokens:    4,
		KeepRecentTokens: 1,
	}

	msgs := []llmtypes.AgentMessage{
		llmtypes.UserMsg(strings.Repeat("a", 80)),
		llmtypes.Message{
			Role: llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{
				llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "1", Name: "read", Args: []byte(`{"path":"old.go"}`)}),
				llmtypes.ToolCallBlock(llmtypes.ToolCall{ID: "2", Name: "edit", Args: []byte(`{"path":"new.go"}`)}),
			},
		},
		llmtypes.UserMsg("keep"),
	}

	out, info, err := runSummaryCompaction(context.Background(), cfg, msgs, true)
	if err != nil {
		t.Fatalf("unexpected compaction error: %v", err)
	}
	if info == nil {
		t.Fatal("expected compaction info")
	}
	if len(out) != 2 {
		t.Fatalf("expected compacted summary + recent message, got %d entries", len(out))
	}

	summary, ok := out[0].(ContextSummary)
	if !ok {
		t.Fatalf("expected first message to be ContextSummary, got %T", out[0])
	}
	if !strings.Contains(summary.Summary, "checkpoint body") {
		t.Fatalf("expected generated summary content, got %q", summary.Summary)
	}
	if !strings.Contains(summary.Summary, "<read-files>\nold.go\n</read-files>") {
		t.Fatalf("expected read file section, got %q", summary.Summary)
	}
	if !strings.Contains(summary.Summary, "<modified-files>\nnew.go\n</modified-files>") {
		t.Fatalf("expected modified file section, got %q", summary.Summary)
	}
	if out[1].TextContent() != "keep" {
		t.Fatalf("expected recent message to be preserved, got %q", out[1].TextContent())
	}
}

// forkTestMessages forces a cut while retaining a tail.
func forkTestMessages() []llmtypes.AgentMessage {
	return []llmtypes.AgentMessage{
		llmtypes.UserMsg(strings.Repeat("a", 80)),
		llmtypes.Message{
			Role:    llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.TextBlock("worked on it")},
		},
		llmtypes.UserMsg("keep"),
	}
}

func summaryReplyModel(record func([]llmtypes.Message, []llmtypes.ToolSpec), texts ...string) *stubModel {
	call := 0
	return &stubModel{
		generate: func(_ context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
			if record != nil {
				record(messages, tools)
			}
			text := texts[min(call, len(texts)-1)]
			call++
			return &llmtypes.LLMResponse{Message: llmtypes.Message{
				Role:    llmtypes.RoleAssistant,
				Content: []llmtypes.ContentBlock{llmtypes.TextBlock(text)},
			}}, nil
		},
	}
}

func TestRunSummaryCompaction_ForkReusesParentPrefix(t *testing.T) {
	var sent [][]llmtypes.Message
	var sentTools [][]llmtypes.ToolSpec
	model := summaryReplyModel(func(m []llmtypes.Message, tools []llmtypes.ToolSpec) {
		sent = append(sent, m)
		sentTools = append(sentTools, tools)
	}, "<analysis>thinking</analysis>\n<summary>checkpoint body</summary>")

	prefix := agentcore.LLMPrefix{
		System:       []llmtypes.Message{llmtypes.SystemMsg("parent system")},
		Tools:        []llmtypes.ToolSpec{{Name: "read"}},
		Model:        model,
		CacheControl: "ephemeral",
	}
	cfg := summaryRunConfig{
		Model:            model,
		ContextWindow:    16,
		ReserveTokens:    4,
		KeepRecentTokens: 1,
		Fork:             &prefix,
	}

	msgs := forkTestMessages()
	out, info, err := runSummaryCompaction(context.Background(), cfg, msgs, true)
	if err != nil {
		t.Fatalf("unexpected compaction error: %v", err)
	}
	if info == nil {
		t.Fatal("expected compaction info")
	}
	if len(sent) != 1 {
		t.Fatalf("expected a single forked call, got %d", len(sent))
	}
	req := sent[0]

	if len(sentTools[0]) != 1 || sentTools[0][0].Name != "read" {
		t.Fatalf("fork must send the parent tool list, got %+v", sentTools[0])
	}
	if req[0].Role != llmtypes.RoleSystem || req[0].TextContent() != "parent system" {
		t.Fatalf("fork must lead with the parent system message, got %+v", req[0])
	}
	// system + 3 conversation messages + instruction
	if len(req) != 5 {
		t.Fatalf("expected the whole view sent as real messages, got %d", len(req))
	}
	if !strings.Contains(req[3].TextContent(), "keep") {
		t.Fatalf("expected the verbatim tail to ride along, got %q", req[3].TextContent())
	}
	if req[3].Metadata["cache_control"] != "ephemeral" {
		t.Fatalf("expected the breakpoint on the last shared message, got %+v", req[3].Metadata)
	}
	last := req[len(req)-1]
	if last.Role != llmtypes.RoleUser || !strings.HasPrefix(last.TextContent(), "Stop working on the task") {
		t.Fatalf("expected the guarded instruction last, got %+v", last)
	}

	summary, ok := out[0].(ContextSummary)
	if !ok || !strings.Contains(summary.Summary, "checkpoint body") {
		t.Fatalf("expected the forked summary to be used, got %+v", out[0])
	}
}

func TestRunSummaryCompaction_ForkFallsBackWhenModelAnswersInstead(t *testing.T) {
	var sent [][]llmtypes.Message
	// Untagged task prose must not become the checkpoint.
	model := summaryReplyModel(func(m []llmtypes.Message, _ []llmtypes.ToolSpec) {
		sent = append(sent, m)
	}, "Sure — the bug is in the retry loop, I'll fix it next.", "checkpoint body")

	prefix := agentcore.LLMPrefix{System: []llmtypes.Message{llmtypes.SystemMsg("parent system")}, Model: model}
	cfg := summaryRunConfig{
		Model:            model,
		ContextWindow:    16,
		ReserveTokens:    4,
		KeepRecentTokens: 1,
		Fork:             &prefix,
	}

	out, _, err := runSummaryCompaction(context.Background(), cfg, forkTestMessages(), true)
	if err != nil {
		t.Fatalf("unexpected compaction error: %v", err)
	}
	if len(sent) != 2 {
		t.Fatalf("expected the fork to fall back to one standalone call, got %d calls", len(sent))
	}
	if !strings.Contains(sent[1][1].TextContent(), "<conversation>") {
		t.Fatalf("expected the fallback to use the standalone prompt, got %q", sent[1][1].TextContent())
	}
	summary, ok := out[0].(ContextSummary)
	if !ok || !strings.Contains(summary.Summary, "checkpoint body") {
		t.Fatalf("expected the fallback summary to be used, got %+v", out[0])
	}
}

func TestRunSummaryCompaction_SkipsForkOnModelMismatch(t *testing.T) {
	var sent [][]llmtypes.Message
	model := summaryReplyModel(func(m []llmtypes.Message, _ []llmtypes.ToolSpec) {
		sent = append(sent, m)
	}, "checkpoint body")

	// A different model cannot reuse the prefix cache.
	prefix := agentcore.LLMPrefix{
		System: []llmtypes.Message{llmtypes.SystemMsg("parent system")},
		Model:  summaryReplyModel(nil, "unused"),
	}
	cfg := summaryRunConfig{
		Model:            model,
		ContextWindow:    16,
		ReserveTokens:    4,
		KeepRecentTokens: 1,
		Fork:             &prefix,
	}

	if _, _, err := runSummaryCompaction(context.Background(), cfg, forkTestMessages(), true); err != nil {
		t.Fatalf("unexpected compaction error: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("expected a single standalone call, got %d", len(sent))
	}
	if !strings.Contains(sent[0][1].TextContent(), "<conversation>") {
		t.Fatalf("expected the standalone prompt, got %q", sent[0][1].TextContent())
	}
}

func TestEstimateContextTokens_UsesLastAssistantUsage(t *testing.T) {
	msgs := []llmtypes.AgentMessage{
		llmtypes.UserMsg("before"),
		llmtypes.Message{
			Role: llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{
				llmtypes.TextBlock("done"),
			},
			Usage: &llmtypes.Usage{TotalTokens: 100},
		},
		llmtypes.UserMsg(strings.Repeat("x", 20)),
	}

	estimate := EstimateContextTokens(msgs)
	if estimate.UsageTokens != 100 {
		t.Fatalf("expected usage tokens=100, got %d", estimate.UsageTokens)
	}
	if estimate.TrailingTokens == 0 {
		t.Fatal("expected trailing tokens to be estimated")
	}
	if estimate.Tokens != estimate.UsageTokens+estimate.TrailingTokens {
		t.Fatalf("unexpected total tokens: %+v", estimate)
	}
}

// The prior summary must appear only in its dedicated block.
func TestPreviousSummaryIsNotAlsoInTheConversation(t *testing.T) {
	t.Parallel()

	var prompt string
	model := stubModel{
		generate: func(_ context.Context, messages []llmtypes.Message, _ []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
			prompt = messages[len(messages)-1].TextContent()
			return &llmtypes.LLMResponse{Message: llmtypes.Message{
				Role:    llmtypes.RoleAssistant,
				Content: []llmtypes.ContentBlock{llmtypes.TextBlock("<summary>fresh</summary>")},
			}}, nil
		},
	}

	const old = "UNIQUE-PRIOR-SUMMARY-TEXT"
	msgs := []llmtypes.AgentMessage{
		ContextSummary{Summary: old},
		llmtypes.Message{Role: llmtypes.RoleUser, Content: []llmtypes.ContentBlock{llmtypes.TextBlock(strings.Repeat("a ", 4000))}},
		llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("ok")}},
		// Absorb KeepRecentTokens so the previous turn is summarized.
		llmtypes.Message{Role: llmtypes.RoleUser, Content: []llmtypes.ContentBlock{llmtypes.TextBlock(strings.Repeat("z ", 400))}},
	}

	cfg := summaryRunConfig{Model: model, ContextWindow: 200000, ReserveTokens: 16000, KeepRecentTokens: 100}
	if _, _, err := runSummaryCompaction(context.Background(), cfg, msgs, true); err != nil {
		t.Fatalf("compaction: %v", err)
	}
	if strings.Count(prompt, old) != 1 {
		t.Fatalf("prior summary appears %d times in the prompt, want 1", strings.Count(prompt, old))
	}
	if !strings.Contains(prompt, "<previous-summary>") {
		t.Fatal("prior summary must ride in its own block")
	}
}

// Do not reword a prior summary when no new history was cut.
func TestSummaryIsNoOpWhenOnlyThePriorSummaryIsCut(t *testing.T) {
	t.Parallel()

	calls := 0
	model := stubModel{
		generate: func(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
			calls++
			return &llmtypes.LLMResponse{Message: llmtypes.Message{
				Role:    llmtypes.RoleAssistant,
				Content: []llmtypes.ContentBlock{llmtypes.TextBlock("<summary>x</summary>")},
			}}, nil
		},
	}

	msgs := []llmtypes.AgentMessage{
		ContextSummary{Summary: "prior"},
		llmtypes.Message{Role: llmtypes.RoleUser, Content: []llmtypes.ContentBlock{llmtypes.TextBlock(strings.Repeat("b ", 4000))}},
	}

	cfg := summaryRunConfig{Model: model, ContextWindow: 200000, ReserveTokens: 16000, KeepRecentTokens: 100}
	out, info, err := runSummaryCompaction(context.Background(), cfg, msgs, true)
	if err != nil {
		t.Fatalf("compaction: %v", err)
	}
	if calls != 0 {
		t.Fatalf("model called %d time(s) with no new history to summarize", calls)
	}
	if info != nil || len(out) != len(msgs) {
		t.Fatal("expected a no-op")
	}
}

func TestRunSummaryCompaction_SummarizesSplitTurnAfterPriorCheckpoint(t *testing.T) {
	var calls int
	model := summaryReplyModel(func([]llmtypes.Message, []llmtypes.ToolSpec) { calls++ }, "checkpoint body")

	// The oversized turn prefix still needs summarizing after a checkpoint.
	msgs := []llmtypes.AgentMessage{
		ContextSummary{Summary: "earlier work"},
		llmtypes.UserMsg(strings.Repeat("a", 200)),
		llmtypes.Message{
			Role:    llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.TextBlock(strings.Repeat("b", 200))},
		},
	}

	cfg := summaryRunConfig{Model: model, ContextWindow: 16, ReserveTokens: 4, KeepRecentTokens: 1}
	out, info, err := runSummaryCompaction(context.Background(), cfg, msgs, true)
	if err != nil {
		t.Fatalf("unexpected compaction error: %v", err)
	}
	if info == nil {
		t.Fatal("expected the oversized turn to be compacted, got a no-op")
	}
	if calls == 0 {
		t.Fatal("expected the split-turn prefix to be summarized")
	}
	if !info.IsSplitTurn {
		t.Fatal("expected the compaction to report a split turn")
	}
	if _, ok := out[0].(ContextSummary); !ok {
		t.Fatalf("expected a checkpoint at the head, got %T", out[0])
	}
}

func TestFullSummaryStrategy_SavingExcludesStaticOverhead(t *testing.T) {
	model := summaryReplyModel(nil, "checkpoint body")
	strategy := NewFullSummary(FullSummaryConfig{Model: model, KeepRecentTokens: 1})

	view := []llmtypes.AgentMessage{
		llmtypes.UserMsg("original request"), llmtypes.UserMsg(strings.Repeat("a", 4000)),
		llmtypes.Message{
			Role:    llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.TextBlock("done")},
		},
		llmtypes.UserMsg("keep"),
	}

	// Provider tokens include static prompt overhead hidden from EstimateTotal.
	const staticOverhead = 50_000
	budget := Budget{
		Tokens:    EstimateTotal(view) + staticOverhead,
		Window:    200_000,
		Threshold: 40_000, // below Tokens, so the strategy actually runs
	}

	next, res, err := strategy.Apply(context.Background(), nil, view, budget)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !res.Applied {
		t.Fatal("expected the summary to apply")
	}
	if want := EstimateTotal(view) - EstimateTotal(next); res.TokensSaved != want {
		t.Fatalf("saving must be measured between views: got %d, want %d", res.TokensSaved, want)
	}
	if res.TokensSaved >= staticOverhead {
		t.Fatalf("saving %d swallowed the static overhead, budget would read empty", res.TokensSaved)
	}
}
