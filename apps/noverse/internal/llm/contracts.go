package llm

import (
	"context"
	"encoding/json"
	"sync"
)

// LLMRequest carries the inputs to a single LLM call.
type LLMRequest struct {
	Messages []Message
	Tools    []ToolSpec
}

// LLMResponse carries the result of a single LLM call.
type LLMResponse struct {
	Message Message
}

// ToolSpec describes a tool for the LLM (name + description + JSON schema).
type ToolSpec struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Parameters   any    `json:"parameters"`
	DeferLoading bool   `json:"defer_loading,omitempty"`
	// Strict enables provider-side strict schema enforcement (OpenAI strict
	// tool calling / Structured Outputs for arguments). nil leaves the
	// provider default. Set via the optional StrictSchemaTool interface.
	Strict *bool `json:"strict,omitempty"`
}

// ToolCall represents a tool invocation request from the LLM.
//
// When the LLM emits args that don't parse as JSON (common cause: stream
// truncation, provider format bug), Args is replaced with "{}" so the
// surrounding Message stays JSON-serializable for persistence; the original
// payload and parser diagnostic are preserved in ArgsRawText / ArgsParseError.
// Downstream schema validation short-circuits on ArgsInvalid and surfaces the
// captured raw text — pointing at the real root cause instead of running
// "missing field" checks against the {} placeholder.
type ToolCall struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Args           json.RawMessage `json:"args"`
	ArgsInvalid    bool            `json:"args_invalid,omitempty"`
	ArgsRawText    string          `json:"args_raw_text,omitempty"`
	ArgsParseError string          `json:"args_parse_error,omitempty"`
	// ThoughtSignature is an opaque provider reasoning signature (Gemini 3) that
	// must be persisted and replayed verbatim across turns. Empty when absent.
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

// ---------------------------------------------------------------------------
// Call Options
// ---------------------------------------------------------------------------

// CallOption configures per-call LLM parameters.
type CallOption func(*CallConfig)

// CallConfig holds per-call configuration resolved from CallOptions.
type CallConfig struct {
	ThinkingLevel  ThinkingLevel
	ThinkingBudget int    // max thinking tokens, 0 = use provider default
	APIKey         string // per-call API key override, empty = use model default
	SessionID      string // provider session caching identifier
	PromptCacheKey string // prompt-cache routing identity, empty = no hint
	MaxTokens      int    // per-call max tokens override, 0 = use model default
	ToolChoice     any    // portable: "auto", "required", "none"; objects are provider-specific
	ResponseFormat *ResponseFormat
}

const (
	ResponseFormatText       = "text"
	ResponseFormatJSONObject = "json_object"
	ResponseFormatJSONSchema = "json_schema"
)

// ResponseFormat controls provider-native structured output.
//
// JSON object mode asks the model to return valid JSON. JSON schema mode asks
// compatible providers to constrain the final response to the supplied schema.
// Callers should still unmarshal and validate model output like any external
// input.
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema describes the provider-native JSON schema response format.
type JSONSchema struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Schema      any    `json:"schema"`
	Strict      *bool  `json:"strict,omitempty"`
}

// ResolveCallConfig applies options and returns the resolved config.
func ResolveCallConfig(opts []CallOption) CallConfig {
	var cfg CallConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// WithThinking sets the thinking level for a single LLM call.
func WithThinking(level ThinkingLevel) CallOption {
	return func(c *CallConfig) { c.ThinkingLevel = NormalizeThinkingLevel(level) }
}

// WithThinkingBudget sets the max thinking tokens for a single LLM call.
func WithThinkingBudget(tokens int) CallOption {
	return func(c *CallConfig) { c.ThinkingBudget = tokens }
}

// WithCallAPIKey overrides the API key for a single LLM call.
// Enables key rotation, OAuth short-lived tokens, and multi-tenant scenarios.
func WithCallAPIKey(key string) CallOption {
	return func(c *CallConfig) { c.APIKey = key }
}

// WithCallPromptCacheKey sets the prompt-cache routing identity for a single
// LLM call. Providers with key-routed prefix caching (OpenAI prompt_cache_key)
// route requests sharing a key to the same cache shard; the adapter drops the
// hint for providers without support.
func WithCallPromptCacheKey(key string) CallOption {
	return func(c *CallConfig) { c.PromptCacheKey = key }
}

// WithCallSessionID sets a session identifier for a single LLM call.
func WithCallSessionID(id string) CallOption {
	return func(c *CallConfig) { c.SessionID = id }
}

// WithMaxTokens overrides the max output tokens for a single LLM call.
func WithMaxTokens(tokens int) CallOption {
	return func(c *CallConfig) { c.MaxTokens = tokens }
}

// WithToolChoice sets "auto", "required", "none", or a provider-specific object.
func WithToolChoice(choice any) CallOption {
	return func(c *CallConfig) { c.ToolChoice = choice }
}

// WithJSONMode requests valid JSON output without enforcing a specific schema.
func WithJSONMode() CallOption {
	return func(c *CallConfig) {
		c.ResponseFormat = &ResponseFormat{Type: ResponseFormatJSONObject}
	}
}

// WithJSONSchema requests structured JSON output constrained by a JSON Schema.
func WithJSONSchema(name, description string, schema any, strict bool) CallOption {
	return func(c *CallConfig) {
		c.ResponseFormat = &ResponseFormat{
			Type: ResponseFormatJSONSchema,
			JSONSchema: &JSONSchema{
				Name:        name,
				Description: description,
				Schema:      schema,
				Strict:      &strict,
			},
		}
	}
}

// WithResponseFormat sets a provider-native response format explicitly.
func WithResponseFormat(format *ResponseFormat) CallOption {
	return func(c *CallConfig) { c.ResponseFormat = format }
}

// ---------------------------------------------------------------------------
// ChatModel Interface
// ---------------------------------------------------------------------------

// ChatModel is the LLM provider interface.
type ChatModel interface {
	Generate(ctx context.Context, messages []Message, tools []ToolSpec, opts ...CallOption) (*LLMResponse, error)
	GenerateStream(ctx context.Context, messages []Message, tools []ToolSpec, opts ...CallOption) (<-chan StreamEvent, error)
	SupportsTools() bool
}

// ProviderNamer is an optional interface for ChatModel implementations
// to expose their provider name (e.g. "openai", "anthropic", "gemini").
// Used by the agent loop to pass provider context to GetApiKey callbacks.
type ProviderNamer interface {
	ProviderName() string
}

// ModelNamer is an optional interface for ChatModel implementations to
// expose their model identifier (e.g. "claude-sonnet-4-6"). Harnesses need
// this for display and telemetry; without it they fall back to concrete-type
// assertions.
type ModelNamer interface {
	ModelName() string
}

// SwappableModel wraps a ChatModel and allows replacing the underlying model
// at runtime. Swaps take effect on the next call.
type SwappableModel struct {
	mu    sync.RWMutex
	model ChatModel
}

func NewSwappableModel(initial ChatModel) *SwappableModel {
	return &SwappableModel{model: initial}
}

func (m *SwappableModel) Swap(next ChatModel) {
	m.mu.Lock()
	m.model = next
	m.mu.Unlock()
}

func (m *SwappableModel) Current() ChatModel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.model
}

func (m *SwappableModel) Generate(ctx context.Context, messages []Message, tools []ToolSpec, opts ...CallOption) (*LLMResponse, error) {
	model := m.Current()
	if model == nil {
		return nil, ErrNoModel
	}
	return model.Generate(ctx, messages, tools, opts...)
}

func (m *SwappableModel) GenerateStream(ctx context.Context, messages []Message, tools []ToolSpec, opts ...CallOption) (<-chan StreamEvent, error) {
	model := m.Current()
	if model == nil {
		return nil, ErrNoModel
	}
	return model.GenerateStream(ctx, messages, tools, opts...)
}

func (m *SwappableModel) SupportsTools() bool {
	model := m.Current()
	return model != nil && model.SupportsTools()
}

func (m *SwappableModel) ProviderName() string {
	model := m.Current()
	pn, ok := model.(ProviderNamer)
	if !ok {
		return ""
	}
	return pn.ProviderName()
}

func (m *SwappableModel) ModelName() string {
	model := m.Current()
	mn, ok := model.(ModelNamer)
	if !ok {
		return ""
	}
	return mn.ModelName()
}

// ---------------------------------------------------------------------------
// Stream Events (fine-grained)
// ---------------------------------------------------------------------------

// StreamEventType identifies LLM streaming event types.
type StreamEventType string

const (
	// Text content streaming
	StreamEventTextStart StreamEventType = "text_start"
	StreamEventTextDelta StreamEventType = "text_delta"
	StreamEventTextEnd   StreamEventType = "text_end"

	// Thinking/reasoning streaming
	StreamEventThinkingStart StreamEventType = "thinking_start"
	StreamEventThinkingDelta StreamEventType = "thinking_delta"
	StreamEventThinkingEnd   StreamEventType = "thinking_end"

	// Tool call streaming
	StreamEventToolCallStart StreamEventType = "toolcall_start"
	StreamEventToolCallDelta StreamEventType = "toolcall_delta"
	StreamEventToolCallEnd   StreamEventType = "toolcall_end"

	// Usage is a cumulative snapshot for this provider turn, not a delta.
	StreamEventUsage StreamEventType = "usage"

	// Terminal events
	StreamEventDone  StreamEventType = "done"
	StreamEventError StreamEventType = "error"
)

// StreamEvent is a streaming event from the LLM.
type StreamEvent struct {
	Type         StreamEventType
	ContentIndex int     // which content block is being updated
	Delta        string  // text/thinking/toolcall argument delta
	Message      Message // partial (during streaming) or final (done)
	ToolCallID   string
	Usage        *Usage // cumulative turn usage, including cost when known
	// CompletedToolCall is populated on StreamEventToolCallEnd with the fully
	// reconstructed tool call. It lets the loop start execution immediately
	// without re-parsing the partial assistant message.
	CompletedToolCall *ToolCall
	StopReason        StopReason // finish reason (for done events)
	Err               error      // for error events
}
