package ctxpack

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"time"

	corecontext "github.com/datnt1998/noverse/internal/agent/context"
	"github.com/datnt1998/noverse/internal/store"
)

const storeSummaryStrategyName = "store_summary"

type StoreSummaryCompactConfig struct {
	Store              *store.Store
	KeepRecentTokens   int
	SummaryTokenBudget int
}

type StoreSummaryCompactStrategy struct {
	store              *store.Store
	keepRecentTokens   int
	summaryTokenBudget int
}

func NewStoreSummaryCompact(cfg StoreSummaryCompactConfig) *StoreSummaryCompactStrategy {
	if cfg.KeepRecentTokens <= 0 {
		cfg.KeepRecentTokens = 20000
	}
	if cfg.SummaryTokenBudget <= 0 {
		cfg.SummaryTokenBudget = defaultStoreSummaryBudgetTokens
	}
	return &StoreSummaryCompactStrategy{
		store:              cfg.Store,
		keepRecentTokens:   cfg.KeepRecentTokens,
		summaryTokenBudget: cfg.SummaryTokenBudget,
	}
}

func (s *StoreSummaryCompactStrategy) Name() string { return storeSummaryStrategyName }

func (s *StoreSummaryCompactStrategy) Apply(ctx context.Context, _ []llmtypes.AgentMessage, view []llmtypes.AgentMessage, budget corecontext.Budget) ([]llmtypes.AgentMessage, corecontext.StrategyResult, error) {
	if budget.Window <= 0 || budget.Tokens <= budget.Threshold {
		return view, corecontext.StrategyResult{Name: s.Name()}, nil
	}
	return s.apply(ctx, view, budget)
}

func (s *StoreSummaryCompactStrategy) ForceApply(ctx context.Context, transcript []llmtypes.AgentMessage, view []llmtypes.AgentMessage, budget corecontext.Budget) ([]llmtypes.AgentMessage, corecontext.StrategyResult, error) {
	base := transcript
	if len(base) == 0 {
		base = view
	}
	return s.apply(ctx, base, budget)
}

func (s *StoreSummaryCompactStrategy) apply(_ context.Context, msgs []llmtypes.AgentMessage, budget corecontext.Budget) ([]llmtypes.AgentMessage, corecontext.StrategyResult, error) {
	if s.store == nil || len(msgs) == 0 {
		return msgs, corecontext.StrategyResult{Name: s.Name()}, nil
	}

	summary, ok, err := buildWriterStoreSummaryText(s.store, s.summaryTokenBudget)
	if err != nil {
		return nil, corecontext.StrategyResult{Name: s.Name()}, err
	}
	if !ok {
		return msgs, corecontext.StrategyResult{Name: s.Name()}, nil
	}

	pinned := corecontext.PinnedPrefixLength(msgs)
	cut := corecontext.FindSummaryCut(msgs[pinned:], s.keepRecentTokens)
	if cut.IsSplitTurn && cut.TurnStartIndex > 0 {
		cut.FirstKeptIndex = cut.TurnStartIndex
		cut.IsSplitTurn = false
	}
	if cut.FirstKeptIndex <= 0 || pinned+cut.FirstKeptIndex >= len(msgs) {
		return msgs, corecontext.StrategyResult{Name: s.Name()}, nil
	}

	toKeep := msgs[pinned+cut.FirstKeptIndex:]
	tokensBefore := corecontext.EstimateTotal(msgs)
	result := make([]llmtypes.AgentMessage, 0, pinned+1+len(toKeep))
	result = append(result, msgs[:pinned]...)
	result = append(result, corecontext.ContextSummary{
		Summary:      summary,
		TokensBefore: tokensBefore,
		Timestamp:    time.Now(),
	})
	result = append(result, toKeep...)

	tokensAfter := corecontext.EstimateTotal(result)
	if tokensAfter >= tokensBefore {
		return msgs, corecontext.StrategyResult{Name: s.Name()}, nil
	}

	info := &corecontext.SummaryInfo{
		TokensBefore:   tokensBefore,
		TokensAfter:    tokensAfter,
		MessagesBefore: len(msgs),
		MessagesAfter:  len(result),
		CompactedCount: cut.FirstKeptIndex,
		KeptCount:      len(toKeep),
		IsSplitTurn:    cut.IsSplitTurn,
		SummaryLen:     len([]rune(summary)),
		Duration:       time.Millisecond,
	}
	if budget.Tokens > budget.Threshold && tokensAfter > budget.Threshold {
		info.Duration = 2 * time.Millisecond
	}

	return result, corecontext.StrategyResult{
		Applied:     true,
		TokensSaved: max(0, tokensBefore-tokensAfter),
		Name:        s.Name(),
		Info:        info,
	}, nil
}
