package agentcore

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"encoding/json"
)

// ---------------------------------------------------------------------------
// Agent Context & Loop Config
// ---------------------------------------------------------------------------

// SystemBlock is one segment of a multi-part system prompt.
// Use with AgentContext.SystemBlocks for per-block cache control.
type SystemBlock struct {
	Text         string `json:"text"`
	CacheControl string `json:"cache_control,omitempty"` // e.g. "ephemeral"
}

// AgentContext holds the immutable context for a single agent loop invocation.
type AgentContext struct {
	SystemPrompt string        // single-string system prompt (legacy)
	SystemBlocks []SystemBlock // multi-block system prompt with cache control (takes precedence)
	Messages     []llmtypes.AgentMessage
	Tools        []Tool
}

// LoopConfig configures the agent loop.
type LoopConfig struct {
	Model         llmtypes.ChatModel
	MaxTurns      int                    // safety limit, default 100
	MaxRetries    int                    // LLM call retry limit for retryable errors, default 3
	MaxToolErrors int                    // consecutive tool failure threshold per tool, 0 = unlimited
	ThinkingLevel llmtypes.ThinkingLevel // reasoning depth

	// Context lifecycle. ContextManager drives prompt projection, overflow
	// recovery, and usage reporting; ConvertToLLM is auto-wired from it when
	// the manager implements ContextLLMConverter.
	ContextManager ContextManager
	ConvertToLLM   func(msgs []llmtypes.AgentMessage) []llmtypes.Message

	// CommitContext replaces the runtime message baseline after an explicit
	// committed compaction, a committed projection rewrite, or committed
	// overflow recovery.
	CommitContext func(msgs []llmtypes.AgentMessage, usage *ContextUsage) error

	// CommitMessage durably records a message before it enters runtime context.
	// Returning an error stops the run; tools requested by that message are not
	// executed. OnMessage remains the post-commit observer hook.
	CommitMessage func(msg llmtypes.AgentMessage) error

	// ToolGate, when non-nil, is called once per tool call after argument
	// validation and the optional Previewer pass. Allowed=false rejects the
	// call (Reason becomes the tool result). The agent core does no permission
	// reasoning of its own.
	ToolGate ToolGate

	// Steering: called after each tool execution to check for user interruptions.
	GetSteeringMessages func() []llmtypes.AgentMessage

	// FollowUp: called when the agent would otherwise stop.
	GetFollowUpMessages func() []llmtypes.AgentMessage

	// Middlewares are applied around each tool execution (outermost first).
	// Use for logging, timing, argument/result modification, etc.
	Middlewares []ToolMiddleware

	// MaxToolConcurrency limits parallel tool execution.
	// 0 or 1 = sequential (default, backward compatible).
	// >1 = up to N tools execute concurrently within a single turn.
	MaxToolConcurrency int

	// ShouldEmitAbortMarker reports whether an abort marker message should be
	// emitted when the context is cancelled. When nil or returns false, the
	// cancellation is silent (legacy behavior). Set by Agent.Abort().
	ShouldEmitAbortMarker func() bool

	// StopAfterTool, if non-nil, is called after each successful (non-error)
	// tool execution. If it returns true, the loop exits with EndReasonStop.
	// Use this to let a terminal tool (e.g. commit_chapter) end the loop
	// without wasting turns. The exit passes through StopGuard (with
	// Trigger=StopTriggerAfterTool) like any other normal stop, so a guard
	// can veto a premature terminal-tool exit.
	StopAfterTool func(toolName string) bool

	// StopAfterToolResult is the result-aware variant of StopAfterTool. It is
	// useful when the same tool can be an intermediate step or a terminal step
	// depending on its structured result.
	StopAfterToolResult func(toolName string, result json.RawMessage) bool

	// OnMessage, if non-nil, is called after each committed message is appended
	// to context (assistant, tool result, steering). It is observational; use
	// CommitMessage when persistence failure must stop execution.
	OnMessage func(msg llmtypes.AgentMessage)

	// StopGuard is consulted on every normal stop (end_turn and
	// StopAfterTool exits). Nil (default) means every stop is allowed.
	StopGuard StopGuard

	// LengthRecoveryPrompt overrides the user message injected when the
	// model's output is truncated (max_tokens) with no completed tool calls.
	// Empty uses a built-in default. Each recovery is an extra LLM call on
	// top of normal turn accounting, bounded by an internal cap.
	LengthRecoveryPrompt string

	// AbortMarkerText overrides the marker message recorded when a run is
	// cancelled mid-inference (only when ShouldEmitAbortMarker returns true).
	// Empty uses a built-in default. Lets non-English harnesses localize it.
	AbortMarkerText string
	// AbortMarkerToolText overrides the marker message recorded when
	// cancellation lands during tool execution. Empty uses a built-in default.
	AbortMarkerToolText string

	// CacheLastMessage, when non-empty, instructs the loop to tag the last
	// non-system message in every LLM request with this cache_control value
	// (e.g. "ephemeral"). Providers that support prompt caching place a write
	// breakpoint at that position covering the entire preceding prefix. Empty
	// string (default) leaves messages untouched — keep cache placement under
	// application control.
	//
	// The breakpoint follows the freshest turn (user input, tool_result, or
	// assistant) and skips trailing per-turn system reminders.
	CacheLastMessage string

	// PromptCacheKey, when non-empty, is attached to every LLM request as the
	// provider's prompt-cache routing identity (e.g. OpenAI prompt_cache_key).
	// Use one key per conversation so all requests of a session land on the
	// same cache shard. The adapter drops the hint for providers without
	// key-routed caching. Empty (default) sends nothing.
	PromptCacheKey string
}

// ---------------------------------------------------------------------------
// Context Usage Estimation
// ---------------------------------------------------------------------------

// ContextEstimateFn estimates the current context token consumption from messages.
// Returns total tokens, tokens from LLM Usage, and estimated trailing tokens.
type ContextEstimateFn func(msgs []llmtypes.AgentMessage) (tokens, usageTokens, trailingTokens int)

// ContextUsage represents the current context window occupancy estimate.
type ContextUsage struct {
	Tokens         int     `json:"tokens"`          // estimated total tokens in context
	ContextWindow  int     `json:"context_window"`  // model's context window size
	Percent        float64 `json:"percent"`         // tokens / contextWindow * 100
	UsageTokens    int     `json:"usage_tokens"`    // from last LLM-reported Usage
	TrailingTokens int     `json:"trailing_tokens"` // chars/4 estimate for trailing messages
}
