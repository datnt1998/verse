package agentcore

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Errors produced by agentcore fall into two layers:
//
//   1. Agent layer — loop control, agent state machine, context management.
//      Surfaced as sentinel errors (Err*) or typed errors (*Error).
//      Match with errors.Is(err, ErrXxx) or errors.As(err, &SomeError{}).
//
//   2. Model layer — provider errors. The kernel does not import any LLM SDK;
//      model adapters classify their SDK's errors at the boundary by mapping
//      them onto this package's sentinels (ErrContextOverflow, ErrProvider*)
//      via errors.Is and the RetryableError / RetryHinter interfaces. Adapters
//      Unwrap to the original SDK error, so callers that DO know the SDK can
//      still match it with errors.As.
//
// User cancellation surfaces as context.Canceled, not a custom sentinel —
// use errors.Is(err, context.Canceled) for abort detection.

// Sentinel errors. Use with errors.Is.
var (
	ErrMaxTurns         = errors.New("max turns reached")
	ErrNoMessages       = errors.New("cannot continue: no messages in context")
	ErrAlreadyRunning   = errors.New("agent is already running")
	ErrBadContinuation  = errors.New("cannot continue from this message role without queued messages")
	ErrStopGuard        = errors.New("stop guard escalated: run terminated")
	ErrStreamPartial    = errors.New("stream closed without done event")
	ErrToolValidation   = errors.New("tool argument validation failed")
	ErrInjectNilMessage = errors.New("inject message is nil")
	ErrRunsHeld         = errors.New("agent runs are held")
	ErrToolBlocked      = errors.New("tool execution blocked")
)

// RetryableError when implemented by an error in the chain, tells the loop
// whether re-issuing the identical request may succeed. Model adapters
// implement it so the kernel decides same-provider retries without importing
// any LLM SDK. Errors that do not implement it are treated as non-retryable
// (the loop still has its own message-pattern classification as a fallback).
type RetryableError interface {
	Retryable() bool
}

// RetryHinter when implemented, supplies a provider-specified backoff hint
// (e.g. a Retry-After header). The loop honors it for the next retry delay,
// capped at its own maximum. A zero duration means "no hint, use backoff".
type RetryHinter interface {
	RetryAfter() time.Duration
}

// isRetryable reports whether the error chain advertises retryability via
// RetryableError.
func isRetryable(err error) bool {
	var r RetryableError
	return errors.As(err, &r) && r.Retryable()
}

// retryAfterHint extracts a provider backoff hint from the chain, or 0 if none
// is present.
func retryAfterHint(err error) time.Duration {
	var h RetryHinter
	if errors.As(err, &h) {
		return h.RetryAfter()
	}
	return 0
}

// MaxTurnsError carries the configured turn limit. errors.Is matches ErrMaxTurns.
type MaxTurnsError struct {
	Limit int
}

func (e *MaxTurnsError) Error() string        { return fmt.Sprintf("max turns (%d) reached", e.Limit) }
func (e *MaxTurnsError) Is(target error) bool { return target == ErrMaxTurns }

// PartialStreamError indicates a stream closed without a terminal done event.
// Partial carries any content received before truncation; callers can inspect
// it for diagnostics but MUST NOT persist it as a completed message — the
// stream did not finish cleanly (missing StopReason, possibly truncated
// tool_call args, unclosed thinking blocks).
type PartialStreamError struct {
	Partial llmtypes.Message
}

func (e *PartialStreamError) Error() string        { return "stream closed without done event" }
func (e *PartialStreamError) Is(target error) bool { return target == ErrStreamPartial }

// ToolValidationError is returned when tool call arguments fail schema
// validation. The agent loop surfaces it as a tool_result with IsError=true,
// not as a fatal loop error, so the model can self-correct on the next turn.
// errors.Is matches ErrToolValidation.
type ToolValidationError struct {
	ToolName string
	Issues   []ValidationIssue
}

func (e *ToolValidationError) Error() string        { return formatValidationIssues(e.ToolName, e.Issues) }
func (e *ToolValidationError) Is(target error) bool { return target == ErrToolValidation }

// Tool-blocked classification.
//
// ToolBlockedError marks an execution refusal: a gate or guard declined to run
// the tool and returned an error instead. It wraps the refusal text so the
// tool result the model reads stays identical; only classification changes.
// errors.Is matches ErrToolBlocked.
type ToolBlockedError struct {
	Reason string // short machine-friendly code for observers (e.g. "repeat_failure")
	Err    error  // the refusal text surfaced to the model
}

func (e *ToolBlockedError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Reason
}

func (e *ToolBlockedError) Unwrap() error        { return e.Err }
func (e *ToolBlockedError) Is(target error) bool { return target == ErrToolBlocked }

// Tool error kinds. Every tool_result with is_error=true is classified into
// exactly one of these labels so counters can sum back to the error total:
//   - ToolErrorValidation: argument schema validation (ToolValidationError /
//     ErrToolValidation), retryable by the model on its next turn;
//   - ToolErrorBlocked: refused before execution by a gate or guard
//     (ToolBlockedError / ErrToolBlocked);
//   - ToolErrorCircuit: skipped by the consecutive-error circuit breaker;
//   - ToolErrorBusiness: everything else — tool bugs, denials inside the tool,
//     skips and cancellations.
//
// The labels travel on tool_result metadata ("error_kind") and are part of
// the observability contract: classifiers match error types, never message
// text.
const (
	ToolErrorValidation = "validation"
	ToolErrorBlocked    = "blocked"
	ToolErrorCircuit    = "circuit"
	ToolErrorBusiness   = "business"
)

// ToolErrorKindOf classifies an execution error into a tool error kind.
// Classification is by error type: ToolValidationError/ErrToolValidation →
// validation, ToolBlockedError/ErrToolBlocked → blocked, everything else →
// business. Circuit skips carry no error value (the call never ran) and are
// tagged ToolErrorCircuit directly at the skip site.
func ToolErrorKindOf(err error) string {
	if err == nil {
		return ""
	}
	var tbe *ToolBlockedError
	if errors.As(err, &tbe) || errors.Is(err, ErrToolBlocked) {
		return ToolErrorBlocked
	}
	if errors.Is(err, ErrToolValidation) {
		return ToolErrorValidation
	}
	return ToolErrorBusiness
}

// isStreamIdleError reports whether err marks a stream idle-timeout abort,
// matched by sentinel or by the rendered message (sub-agent JSON results
// flatten the error chain).
func isStreamIdleError(err error) bool {
	return errors.Is(err, llmtypes.ErrProviderStreamIdle) || IsStreamIdleMessage(err.Error())
}

// ValidationIssue describes a single schema mismatch from tool arg validation.
type ValidationIssue struct {
	Kind     string
	Path     string
	Expected string
	Received string
	Hint     string // optional fix hint, appended to the rendered message
}

const (
	IssueMissing = "missing"
	IssueType    = "type"
	IssueValue   = "value"
	IssueUnknown = "unknown"
)

// ErrorKind returns a stable, log-friendly label for err: "canceled",
// "stop_guard", "max_turns", "context_overflow", "stream_partial",
// "tool_validation", "stream_idle", "quota", "rate_limit", "timeout",
// "auth", "network", "overloaded", "content_filter". Returns "" for nil and
// "unknown" when
// nothing matches.
//
// Labels are part of the public API contract — they will not change between
// minor versions, so harnesses can key alert routing and log filters on them
// instead of matching error strings.
func ErrorKind(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, ErrStopGuard):
		return "stop_guard"
	case errors.Is(err, ErrMaxTurns):
		return "max_turns"
	case llmtypes.IsContextOverflow(err):
		return "context_overflow"
	case errors.Is(err, ErrStreamPartial):
		return "stream_partial"
	case errors.Is(err, ErrToolValidation):
		return "tool_validation"
	}
	switch classifyProviderSentinel(err) {
	case llmtypes.ErrProviderStreamIdle:
		return "stream_idle"
	case llmtypes.ErrProviderQuota:
		return "quota"
	case llmtypes.ErrProviderRateLimit:
		return "rate_limit"
	case llmtypes.ErrProviderTimeout:
		return "timeout"
	case llmtypes.ErrProviderAuth:
		return "auth"
	case llmtypes.ErrProviderNetwork:
		return "network"
	case llmtypes.ErrProviderOverloaded:
		return "overloaded"
	case llmtypes.ErrProviderContentFilter:
		return "content_filter"
	}
	return "unknown"
}

// streamIdleMsgPattern matches the rendered message of a stream-idle abort.
const streamIdleMsgPattern = "stream idle timeout"

// IsStreamIdleMessage reports whether s contains the rendered marker of a
// stream idle-timeout abort. Useful when only the error string survives
// (sub-agent JSON results, structured event payloads that flatten the chain).
func IsStreamIdleMessage(s string) bool {
	return strings.Contains(strings.ToLower(s), streamIdleMsgPattern)
}

// ClassifyProvider inspects an LLM/provider error and returns the most specific
// matching sentinel from this package's Err* variables. Returns nil when err is
// nil; returns err unchanged when no classification applies, so callers can wrap
// with their own context.
//
// Stream-idle is checked before generic timeout: it is a stuck connection that
// failover can typically rescue, whereas a generic timeout may just be a slow
// model. Both error-chain matching (adapters map stream-idle onto
// ErrProviderStreamIdle) and message pattern matching are supported because
// sub-agent JSON results flatten the original error to a plain string.
//
// Context overflow is intentionally not returned here — use IsContextOverflow,
// which covers both the agentcore wrapper and adapter-classified errors.
func ClassifyProvider(err error) error {
	if err == nil {
		return nil
	}
	if sentinel := classifyProviderSentinel(err); sentinel != nil {
		return sentinel
	}
	return err
}

func classifyProviderSentinel(err error) error {
	if errors.Is(err, llmtypes.ErrProviderContentFilter) {
		return llmtypes.ErrProviderContentFilter
	}
	if errors.Is(err, llmtypes.ErrProviderStreamIdle) {
		return llmtypes.ErrProviderStreamIdle
	}
	if errors.Is(err, llmtypes.ErrProviderQuota) {
		return llmtypes.ErrProviderQuota
	}
	if errors.Is(err, llmtypes.ErrProviderOverloaded) {
		return llmtypes.ErrProviderOverloaded
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return llmtypes.ErrProviderTimeout
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, streamIdleMsgPattern):
		return llmtypes.ErrProviderStreamIdle
	case containsAny(msg, "insufficient_quota", "quota exceeded", "quota exhausted", "usage_limit_reached", "usage limit"):
		return llmtypes.ErrProviderQuota
	case containsAny(msg, "rate limit", "too many requests", "429"):
		return llmtypes.ErrProviderRateLimit
	case containsAny(msg, "deadline exceeded", "timeout", "timed out"):
		return llmtypes.ErrProviderTimeout
	case containsAny(msg, "invalid api key", "incorrect api key", "unauthorized", "authentication failed", "forbidden", "401", "403"):
		return llmtypes.ErrProviderAuth
	case containsAny(msg, "connection refused", "connection reset", "no such host", "dial tcp", "tls handshake timeout", "server misbehaving", "broken pipe", "eof"):
		return llmtypes.ErrProviderNetwork
	case containsAny(msg, "overloaded", "over capacity", "529"):
		return llmtypes.ErrProviderOverloaded
	}
	return nil
}

// IsFailoverEligible reports whether err matches a transient provider error
// suitable for cross-provider failover: rate_limit, timeout, network, or
// stream_idle. Returns false for auth errors, content filters,
// context_overflow, user cancellation, or unclassified errors.
func IsFailoverEligible(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	classified := ClassifyProvider(err)
	return errors.Is(classified, llmtypes.ErrProviderRateLimit) ||
		errors.Is(classified, llmtypes.ErrProviderTimeout) ||
		errors.Is(classified, llmtypes.ErrProviderNetwork) ||
		errors.Is(classified, llmtypes.ErrProviderStreamIdle) ||
		errors.Is(classified, llmtypes.ErrProviderOverloaded)
}

// FailoverReason returns a stable short label ("rate_limit" / "timeout" /
// "stream_idle" / "network" / "overloaded") suitable for structured logging.
// Returns "" when err is not failover-eligible.
func FailoverReason(err error) string {
	if err == nil {
		return ""
	}
	classified := ClassifyProvider(err)
	switch {
	case errors.Is(classified, llmtypes.ErrProviderStreamIdle):
		return "stream_idle"
	case errors.Is(classified, llmtypes.ErrProviderRateLimit):
		return "rate_limit"
	case errors.Is(classified, llmtypes.ErrProviderTimeout):
		return "timeout"
	case errors.Is(classified, llmtypes.ErrProviderNetwork):
		return "network"
	case errors.Is(classified, llmtypes.ErrProviderOverloaded):
		return "overloaded"
	}
	return ""
}

func containsAny(msg string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// formatValidationIssues renders issues as a single multi-line block.
// Missing params come first; within each group, paths sort alphabetically for
// stable output.
func formatValidationIssues(toolName string, issues []ValidationIssue) string {
	// Sort a copy: Error() must not mutate the caller's Issues slice.
	issues = append([]ValidationIssue(nil), issues...)
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Kind != issues[j].Kind {
			return validationIssueRank(issues[i].Kind) < validationIssueRank(issues[j].Kind)
		}
		return issues[i].Path < issues[j].Path
	})

	lines := make([]string, 0, len(issues))
	for _, it := range issues {
		var line string
		switch it.Kind {
		case IssueMissing:
			line = fmt.Sprintf("The required parameter `%s` is missing", it.Path)
		case IssueType:
			line = fmt.Sprintf(
				"The parameter `%s` type is expected as `%s` but provided as `%s`",
				it.Path, it.Expected, it.Received,
			)
		case IssueValue:
			line = fmt.Sprintf(
				"The parameter `%s` must be one of %s but provided as %s",
				it.Path, it.Expected, it.Received,
			)
		case IssueUnknown:
			line = fmt.Sprintf("The parameter `%s` is not allowed", it.Path)
		default:
			continue
		}
		if it.Hint != "" {
			line += ". " + it.Hint
		}
		lines = append(lines, line)
	}

	noun := "issue"
	if len(lines) > 1 {
		noun = "issues"
	}
	header := fmt.Sprintf("InputValidationError: %s failed due to the following %s:", toolName, noun)
	return header + "\n" + strings.Join(lines, "\n")
}

func validationIssueRank(kind string) int {
	switch kind {
	case IssueMissing:
		return 0
	case IssueType:
		return 1
	case IssueValue:
		return 2
	case IssueUnknown:
		return 3
	default:
		return 4
	}
}
