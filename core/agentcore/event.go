package agentcore

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"time"
)

// ---------------------------------------------------------------------------
// Agent Events
// ---------------------------------------------------------------------------

// EventType identifies agent lifecycle event types.
type EventType string

const (
	EventAgentStart EventType = "agent_start"
	EventAgentEnd   EventType = "agent_end"
	EventTurnStart  EventType = "turn_start"
	// EventModelResponse fires after every model call completes, including any
	// tool executions it triggered. One model invocation produces one event —
	// not one logical user exchange — so steering injections and length
	// recoveries each produce additional ones within the same run.
	EventModelResponse  EventType = "model_response"
	EventMessageStart   EventType = "message_start"
	EventMessageUpdate  EventType = "message_update"
	EventMessageEnd     EventType = "message_end"
	EventToolExecStart  EventType = "tool_exec_start"
	EventToolExecUpdate EventType = "tool_exec_update"
	EventToolExecEnd    EventType = "tool_exec_end"
	EventRetry          EventType = "retry"
	EventError          EventType = "error"

	// Signal events below exist purely for observation: emitting them never
	// changes loop behavior. Names are stable so harnesses can key counters
	// and session-log bridges on them.
	// EventStreamIdle marks one model call aborted by the stream idle watchdog.
	EventStreamIdle EventType = "stream_idle"
	// EventToolBlocked marks a tool call refused before execution by a gate
	// or guard. Reason carries a short refusal code (e.g. the blocker's own
	// reason string), never argument or prompt text.
	EventToolBlocked EventType = "tool_blocked"
	// EventCircuitOpen marks a tool call skipped by the consecutive-error
	// circuit breaker (MaxToolErrors).
	EventCircuitOpen EventType = "circuit_open"
	// EventCompaction marks one context compaction. Tool carries the compacted
	// tool-result name (empty for whole-context compactions such as summary);
	// Reason carries the trigger ("overflow", "microcompact", "summary").
	EventCompaction EventType = "compaction"
	// EventSubagentExit marks one sub-agent run end. Reason carries the
	// trigger: "stop", "tool" (StopAfterTool* exit), "guard" (stop-guard
	// escalation), "error", "max_turns" or "aborted".
	EventSubagentExit EventType = "subagent_exit"
)

// ToolExecUpdateKind distinguishes update payload semantics for tool_exec_update events.
type ToolExecUpdateKind string

const (
	ToolExecUpdatePreview  ToolExecUpdateKind = "preview"
	ToolExecUpdateProgress ToolExecUpdateKind = "progress"
)

// EndReason describes why a single agent run stopped.
type EndReason string

const (
	EndReasonStop     EndReason = "stop"
	EndReasonMaxTurns EndReason = "max_turns"
	EndReasonAborted  EndReason = "aborted"
	EndReasonError    EndReason = "error"
)

// RunSummary captures loop facts that are known at the end of a run.
// It intentionally excludes higher-level policy judgments.
type RunSummary struct {
	TurnCount  int
	ToolCalls  int
	ToolErrors int
	EndReason  EndReason
	Usage      llmtypes.Usage // aggregate provider usage and known cost for this run
	Turns      []TurnUsage
}

// TurnUsage records one model turn independently of tool execution. Usage is
// nil when the provider did not report it, for example after an early cancel.
type TurnUsage struct {
	Index      int
	Usage      *llmtypes.Usage
	StopReason llmtypes.StopReason
}

// DeltaKind identifies what kind of content a message_update delta carries.
type DeltaKind string

const (
	DeltaText     DeltaKind = ""         // default: regular text
	DeltaThinking DeltaKind = "thinking" // model reasoning/thinking
	DeltaToolCall DeltaKind = "toolcall" // tool call argument JSON
)

// Event is a lifecycle event emitted by the agent loop.
// This is the single output channel for all lifecycle information.
type Event struct {
	Type        EventType
	Message     llmtypes.AgentMessage // for message_start/update/end, turn_end
	Delta       string                // text delta for message_update
	DeltaKind   DeltaKind             // for message_update: what kind of delta
	ToolID      string                // for tool_exec_*
	Tool        string                // tool name for tool_exec_*
	ToolLabel   string                // human-readable tool label (from ToolLabeler)
	Args        json.RawMessage       // tool args for tool_exec_start/tool_exec_update
	Result      json.RawMessage       // tool result for tool_exec_end and preview updates
	Progress    *ProgressPayload
	UpdateKind  ToolExecUpdateKind
	IsError     bool // tool error flag for tool_exec_end
	ErrorKind   string // structured tool error classification, when IsError
	Preview     json.RawMessage
	ToolResults []ToolResult            // for turn_end: all tool results from this turn
	Err         error                   // for error events
	Reason      string                  // short code: tool_blocked reason, compaction trigger, subagent_exit trigger, retry source
	NewMessages []llmtypes.AgentMessage // for agent_end: messages added during this loop
	RetryInfo   *RetryInfo              // for retry events
	Summary     *RunSummary             // for agent_end: factual run summary
}

// RetryInfo carries retry context for EventRetry events.
type RetryInfo struct {
	Attempt    int
	MaxRetries int
	Delay      time.Duration
	Err        error
}

// RetrySourceModel is the Reason of an EventRetry reported through
// ReportEvent by a model wrapper that retries below the loop (before the
// stream's first event). The wrapper renders its own progress line, so
// runners count such events but must not render them again.
const RetrySourceModel = "model"

// ---------------------------------------------------------------------------
// In-context event reporting
// ---------------------------------------------------------------------------

// eventReporterKey is the context key holding the run's event reporter.
type eventReporterKey struct{}

// WithEventReporter binds the run's event sink to ctx so components that only
// receive a context (context strategies, tools, middleware) can emit
// observation events. AgentLoop installs it internally; callers never need to.
func WithEventReporter(ctx context.Context, emit func(Event)) context.Context {
	return context.WithValue(ctx, eventReporterKey{}, emit)
}

// ReportEvent emits an observation event onto the enclosing run's event
// channel when ctx carries a reporter installed by the loop; it is a no-op
// otherwise (unit tests, detached calls). It must only be used for events
// that observe behavior, never to drive it.
func ReportEvent(ctx context.Context, ev Event) {
	if fn, ok := ctx.Value(eventReporterKey{}).(func(Event)); ok {
		fn(ev)
	}
}

// ---------------------------------------------------------------------------
// Event Helpers
// ---------------------------------------------------------------------------

// eventSink delivers loop events bound to the run context. It is created
// once per run at the AgentLoop entry points, so every emission site shares
// the same lifetime regardless of narrower contexts (per-tool cancellation
// must not affect event delivery).
type eventSink struct {
	ctx context.Context
	ch  chan<- Event
}

// emit sends an event to the channel, blocking when it is full — backpressure,
// never event loss, while the run is live. Once the run context is canceled
// delivery degrades to best-effort: a buffered/ready send still succeeds (a
// draining reader receives the terminal events), but when the channel stays
// full the event is dropped so an abandoned channel cannot leak the loop
// goroutine.
func (s eventSink) emit(ev Event) {
	select {
	case s.ch <- ev:
	default:
		select {
		case s.ch <- ev:
		case <-s.ctx.Done():
		}
	}
}

// emitError sends an error event followed by agent_end.
func (s eventSink) emitError(err error, summary *RunSummary) {
	s.emit(Event{Type: EventError, Err: err})
	s.emit(Event{Type: EventAgentEnd, Err: err, Summary: summary})
}

// ---------------------------------------------------------------------------
// Message Sequence Repair
// ---------------------------------------------------------------------------

// DefaultConvertToLLM filters AgentMessages to LLM-compatible Messages.
// Custom message types are dropped; only user/assistant/system/tool messages pass through.
func DefaultConvertToLLM(msgs []llmtypes.AgentMessage) []llmtypes.Message {
	out := make([]llmtypes.Message, 0, len(msgs))
	for _, m := range msgs {
		if msg, ok := m.(llmtypes.Message); ok {
			if msg.StopReason == llmtypes.StopReasonError || msg.StopReason == llmtypes.StopReasonAborted {
				continue
			}
			out = append(out, msg)
		}
	}
	return out
}

// dequeue drains all messages from the queue.
func dequeue(queue *[]llmtypes.AgentMessage) []llmtypes.AgentMessage {
	if len(*queue) == 0 {
		return nil
	}
	result := *queue
	*queue = nil
	return result
}
