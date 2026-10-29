package context

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"sync/atomic"
	"time"

	"github.com/datnt1998/verse/core/agentcore"
)

// FullSummaryConfig controls ContextSummary checkpoint generation.
// KeepRecentTokens reserves a recent suffix of messages to keep verbatim.
// PostSummaryHooks may inject lightweight reminder messages after the summary.
type FullSummaryConfig struct {
	// Model performs the actual summary generation.
	Model llmtypes.ChatModel
	// StripImages controls whether images are removed before summarization.
	// Nil defaults to true.
	StripImages *bool
	// KeepRecentTokens reserves a recent suffix to keep verbatim. Zero scales it
	// from the budget instead.
	KeepRecentTokens int
	// PostSummaryHooks inject lightweight reminder messages after the summary.
	PostSummaryHooks []PostSummaryHook

	// Custom summary prompts. Empty strings fall back to the built-in defaults
	// (code-assistant oriented). Set these to override with domain-specific
	// prompts — e.g., novel-writing prompts that preserve narrative continuity.
	SystemPrompt        string
	SummaryPrompt       string
	UpdateSummaryPrompt string
	TurnPrefixPrompt    string
}

// FullSummaryStrategy rewrites older context into a ContextSummary checkpoint
// while keeping a recent suffix of messages verbatim.
type FullSummaryStrategy struct {
	cfg FullSummaryConfig
	// fork is updated by the loop and may be read by /compact concurrently.
	fork atomic.Pointer[agentcore.LLMPrefix]
}

// NewFullSummary constructs the terminal summary strategy used when lighter
// rewrites are insufficient. Model is required for actual summarization.
func NewFullSummary(cfg FullSummaryConfig) *FullSummaryStrategy {
	return &FullSummaryStrategy{cfg: cfg}
}

// keepRecentTokens scales the verbatim tail with the current threshold.
func (s *FullSummaryStrategy) keepRecentTokens(budget Budget) int {
	if s.cfg.KeepRecentTokens > 0 {
		return s.cfg.KeepRecentTokens
	}
	return min(maxKeepRecentTokens, max(minKeepRecentTokens, budget.Threshold/4))
}

func (s *FullSummaryStrategy) Name() string { return "full_summary" }

func (s *FullSummaryStrategy) Apply(ctx context.Context, _ []llmtypes.AgentMessage, view []llmtypes.AgentMessage, budget Budget) ([]llmtypes.AgentMessage, StrategyResult, error) {
	if budget.Window <= 0 || budget.Tokens <= budget.Threshold {
		return view, StrategyResult{Name: s.Name()}, nil
	}
	return s.apply(ctx, view, budget, false)
}

func (s *FullSummaryStrategy) ForceApply(ctx context.Context, _ []llmtypes.AgentMessage, view []llmtypes.AgentMessage, budget Budget) ([]llmtypes.AgentMessage, StrategyResult, error) {
	return s.apply(ctx, view, budget, true)
}

// SetPostSummaryHooks replaces the hook list used to inject lightweight
// reminder messages after a summary checkpoint is produced.
func (s *FullSummaryStrategy) SetPostSummaryHooks(hooks ...PostSummaryHook) {
	s.cfg.PostSummaryHooks = hooks
}

// SetForkPrefix installs the parent prefix used by cache-aware summaries.
func (s *FullSummaryStrategy) SetForkPrefix(p agentcore.LLMPrefix) {
	s.fork.Store(&p)
}

func (s *FullSummaryStrategy) apply(ctx context.Context, view []llmtypes.AgentMessage, budget Budget, force bool) ([]llmtypes.AgentMessage, StrategyResult, error) {
	if len(view) == 0 || s.cfg.Model == nil {
		return view, StrategyResult{Name: s.Name()}, nil
	}

	ctxWindow := budget.Window
	reserve := budget.Window - budget.Threshold
	if reserve <= 0 {
		reserve = 1
	}
	if force {
		ctxWindow = max(budget.Tokens, 2)
		reserve = 1
	}

	cfg := summaryRunConfig{
		Model:               s.cfg.Model,
		ContextWindow:       ctxWindow,
		ReserveTokens:       reserve,
		KeepRecentTokens:    s.keepRecentTokens(budget),
		SystemPrompt:        s.cfg.SystemPrompt,
		SummaryPrompt:       s.cfg.SummaryPrompt,
		UpdateSummaryPrompt: s.cfg.UpdateSummaryPrompt,
		TurnPrefixPrompt:    s.cfg.TurnPrefixPrompt,
		Fork:                s.fork.Load(),
	}
	stripImages := true
	if s.cfg.StripImages != nil {
		stripImages = *s.cfg.StripImages
	}

	pinned := PinnedPrefixLength(view)
	next, info, err := runSummaryCompaction(ctx, cfg, view[pinned:], stripImages)
	if err != nil {
		return nil, StrategyResult{Name: s.Name()}, err
	}
	if info == nil || !containsContextSummary(next) {
		return view, StrategyResult{Name: s.Name()}, nil
	}

	// View deltas exclude static system and tool-schema overhead.
	saved := max(0, EstimateTotal(view[pinned:])-EstimateTotal(next))
	room := max(0, budget.Threshold-applySaving(budget.Tokens, saved))

	next, err = s.applyHooks(ctx, next, *info, room)
	if err != nil {
		return nil, StrategyResult{Name: s.Name()}, err
	}

	if pinned > 0 {
		out := make([]llmtypes.AgentMessage, 0, pinned+len(next))
		out = append(out, view[:pinned]...)
		next = append(out, next...)
	}
	info.TokensBefore = EstimateTotal(view)
	info.MessagesBefore = len(view)
	info.MessagesAfter = len(next)
	info.TokensAfter = EstimateTotal(next)
	if info.Duration == 0 {
		info.Duration = time.Millisecond
	}

	return next, StrategyResult{
		Applied:     true,
		TokensSaved: max(0, EstimateTotal(view)-EstimateTotal(next)),
		Name:        s.Name(),
		Info:        info,
	}, nil
}

// applyHooks draws each injection from the remaining room.
func (s *FullSummaryStrategy) applyHooks(ctx context.Context, msgs []llmtypes.AgentMessage, info SummaryInfo, room int) ([]llmtypes.AgentMessage, error) {
	if len(s.cfg.PostSummaryHooks) == 0 || len(msgs) == 0 {
		return msgs, nil
	}
	kept := append([]llmtypes.AgentMessage(nil), msgs[1:]...)
	var injected []llmtypes.AgentMessage
	var err error
	for _, hook := range s.cfg.PostSummaryHooks {
		var extra []llmtypes.AgentMessage
		extra, err = hook(ctx, info, kept, room)
		if err != nil {
			return nil, err
		}
		injected = append(injected, extra...)
		room = max(0, room-EstimateTotal(extra))
	}
	if len(injected) == 0 {
		return msgs, nil
	}
	out := make([]llmtypes.AgentMessage, 0, len(msgs)+len(injected))
	out = append(out, msgs[0])
	out = append(out, injected...)
	out = append(out, msgs[1:]...)
	return out, nil
}

func containsContextSummary(msgs []llmtypes.AgentMessage) bool {
	for _, msg := range msgs {
		if _, ok := msg.(ContextSummary); ok {
			return true
		}
	}
	return false
}
