// Package llm owns the canonical conversation, tool, usage and streaming model.
// Provider transports live below this seam; the agent loop consumes only these
// normalized contracts. No provider implementation depends on the agent loop.
// Construct a model with [NewModel].
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/datnt1998/verse/core/models"
	"maps"
	"strings"
	"time"

	"github.com/datnt1998/verse/core/llm/provider/anthropic"
	"github.com/datnt1998/verse/core/llm/provider/bedrock"
	"github.com/datnt1998/verse/core/llm/provider/compat"
	"github.com/datnt1998/verse/core/llm/provider/deepseek"
	"github.com/datnt1998/verse/core/llm/provider/gemini"
	"github.com/datnt1998/verse/core/llm/provider/glm"
	"github.com/datnt1998/verse/core/llm/provider/grok"
	"github.com/datnt1998/verse/core/llm/provider/mimo"
	"github.com/datnt1998/verse/core/llm/provider/minimax"
	"github.com/datnt1998/verse/core/llm/provider/ollama"
	"github.com/datnt1998/verse/core/llm/provider/openai"
	"github.com/datnt1998/verse/core/llm/provider/opencodego"
	"github.com/datnt1998/verse/core/llm/provider/openrouter"
	"github.com/datnt1998/verse/core/llm/provider/qwen"
	"github.com/datnt1998/verse/core/llm/wire"
)

// providerConfig is the common configuration consumed by provider factories.
type providerConfig struct {
	APIKey  string
	BaseURL string
	Extra   map[string]any
}

// ClientModel adapts litellm to the ChatModel interface.
type ClientModel struct {
	*BaseModel
	client *litellm.Client
	model  string
	extra  map[string]any // model-level Extra merged into every request
}

// NewClientModel wraps an existing litellm.Client as a ChatModel. Use this
// when you need to reuse a Client or inject a custom Provider instance; for
// the common case prefer NewModel.
func NewClientModel(model string, client *litellm.Client) *ClientModel {
	modelInfo := ModelInfo{
		Name:     model,
		Provider: client.ProviderName(),
		Capabilities: []string{
			string(CapabilityChat),
			string(CapabilityCompletion),
			string(CapabilityStreaming),
			string(CapabilityToolCalling),
		},
	}

	return &ClientModel{
		BaseModel: NewBaseModel(modelInfo, DefaultGenerationConfig),
		client:    client,
		model:     model,
	}
}

// modelConfig collects optional knobs for NewModel.
type modelConfig struct {
	apiKey        string
	baseURL       string
	streamIdle    time.Duration
	streamIdleSet bool // distinguishes "unset" from explicit 0 (disable watchdog)
	extra         map[string]any
	providerExtra map[string]any
}

// ModelOption configures NewModel.
type ModelOption func(*modelConfig)

func WithAPIKey(key string) ModelOption  { return func(c *modelConfig) { c.apiKey = key } }
func WithBaseURL(url string) ModelOption { return func(c *modelConfig) { c.baseURL = url } }

// WithStreamIdleTimeout aborts a streaming response if no chunk arrives within
// the window. Pass 0 to disable the watchdog explicitly. When the option is
// never set no watchdog is installed; the application supplies its own default.
func WithStreamIdleTimeout(d time.Duration) ModelOption {
	return func(c *modelConfig) {
		c.streamIdle = d
		c.streamIdleSet = true
	}
}

// WithExtra sets model-level, provider-specific request parameters merged into
// every request's Extra map (e.g. min_p, presence_penalty, or provider keys like
// chat_template_kwargs). OpenAI-compatible providers pass these through verbatim
// into the request body — the extra_body convention. Per-call Extra entries
// (e.g. session_id) are added alongside, not overwritten.
func WithExtra(extra map[string]any) ModelOption {
	return func(c *modelConfig) { c.extra = extra }
}

// WithProviderExtra sets provider-level configuration passed to
// provider transport configuration.Extra. Use it for HTTP headers or other provider
// client options, while WithExtra remains request-body Extra.
func WithProviderExtra(extra map[string]any) ModelOption {
	return func(c *modelConfig) { c.providerExtra = extra }
}

// cloneExtra copies the model-level extra map so per-call mutations (e.g.
// session_id) never leak back into the shared model config across requests.
func cloneExtra(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	c := make(map[string]any, len(m))
	maps.Copy(c, m)
	return c
}

// NewModel constructs a ChatModel by provider name. The provider must be
// registered in litellm (builtin or via litellm.RegisterProvider).
func NewModel(provider, model string, opts ...ModelOption) (*ClientModel, error) {
	cfg := modelConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	pcfg := providerConfig{
		APIKey:  cfg.apiKey,
		BaseURL: cfg.baseURL,
		Extra:   cloneExtra(cfg.providerExtra),
	}
	providerImpl, err := newProvider(provider, pcfg)
	if err != nil {
		return nil, fmt.Errorf("llm: %s: %w", provider, err)
	}
	var clientOpts []litellm.ClientOption
	if cfg.streamIdleSet {
		clientOpts = append(clientOpts, litellm.WithStreamIdleTimeout(cfg.streamIdle))
	}
	client, err := litellm.New(providerImpl, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("llm: %s client: %w", provider, err)
	}
	adapter := NewClientModel(model, client)
	adapter.extra = cfg.extra
	return adapter, nil
}

var knownProviders = map[string]struct{}{
	"anthropic":   {},
	"bedrock":     {},
	"deepseek":    {},
	"gemini":      {},
	"glm":         {},
	"grok":        {},
	"minimax":     {},
	"mimo":        {},
	"ollama":      {},
	"opencode-go": {},
	"openai":      {},
	"openrouter":  {},
	"qwen":        {},
}

// IsProviderRegistered reports whether the provider name is known to this adapter.
func IsProviderRegistered(name string) bool {
	_, ok := knownProviders[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// RegisteredProviders returns all provider names known to this adapter.
func RegisteredProviders() []string {
	return []string{"anthropic", "bedrock", "deepseek", "gemini", "glm", "grok", "minimax", "mimo", "ollama", "openai", "opencode-go", "openrouter", "qwen"}
}

// ProviderName returns the provider name (e.g. "openai", "anthropic").
// Implements ProviderNamer for per-provider API key resolution.
func (l *ClientModel) ProviderName() string {
	return l.Info().Provider
}

// Capabilities returns the provider/model capability view exposed by litellm.
func (l *ClientModel) Capabilities() Capabilities {
	if l == nil {
		return Capabilities{}
	}
	if l.client == nil {
		caps := Capabilities{Model: l.model}
		if l.BaseModel != nil {
			info := l.Info()
			caps.Provider = info.Provider
			if caps.Model == "" {
				caps.Model = info.Name
			}
		}
		return caps
	}
	return fromLiteLLMCapabilities(l.client.Capabilities(l.model))
}

// Generate produces a synchronous response.
func (l *ClientModel) Generate(ctx context.Context, messages []Message, tools []ToolSpec, opts ...CallOption) (*LLMResponse, error) {
	cfg := l.GetConfig()
	llmMessages := convertMessages(RepairToolHistory(messages))

	ltReq := &litellm.Request{
		Model:           l.model,
		Messages:        llmMessages,
		MaxTokens:       &cfg.MaxTokens,
		ProviderOptions: litellm.ProviderOptions(cloneExtra(l.extra)),
	}
	applySamplingConfig(ltReq, cfg)

	var err error
	ctx, err = applyCallConfig(ctx, ltReq, opts, l.client.Capabilities(l.model))
	if err != nil {
		return nil, err
	}
	if err := applyToolConfig(ltReq, tools); err != nil {
		return nil, err
	}

	ltResp, err := l.client.Chat(ctx, *ltReq)
	if err != nil {
		return nil, wrapProviderError(err)
	}

	msg := convertResponse(ltResp)
	if msg.Usage != nil {
		msg.Usage.Cost = CalculateCost(pricingForUsage(l.Info().Pricing, msg.Usage), msg.Usage)
	}
	return &LLMResponse{Message: msg}, nil
}

// GenerateStream produces a streaming response with fine-grained events.
func (l *ClientModel) GenerateStream(ctx context.Context, messages []Message, tools []ToolSpec, opts ...CallOption) (<-chan StreamEvent, error) {
	cfg := l.GetConfig()
	llmMessages := convertMessages(RepairToolHistory(messages))

	request := &litellm.Request{
		Model:           l.model,
		Messages:        llmMessages,
		MaxTokens:       &cfg.MaxTokens,
		ProviderOptions: litellm.ProviderOptions(cloneExtra(l.extra)),
	}
	applySamplingConfig(request, cfg)

	var err error
	ctx, err = applyCallConfig(ctx, request, opts, l.client.Capabilities(l.model))
	if err != nil {
		return nil, err
	}
	if err := applyToolConfig(request, tools); err != nil {
		return nil, err
	}

	stream, err := l.client.Stream(ctx, *request)
	if err != nil {
		return nil, wrapProviderError(err)
	}

	eventChan := make(chan StreamEvent, 100)

	go func() {
		defer close(eventChan)
		defer stream.Close()
		emit := func(ev StreamEvent) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			// Observers retain events asynchronously. Never publish the mutable
			// accumulator's slices, maps or tool-call pointers as snapshots.
			if len(ev.Message.Content) > 0 {
				ev.Message.Content = append([]ContentBlock(nil), ev.Message.Content...)
				for i := range ev.Message.Content {
					if call := ev.Message.Content[i].ToolCall; call != nil {
						copy := *call
						ev.Message.Content[i].ToolCall = &copy
					}
				}
			}
			ev.Message.Metadata = maps.Clone(ev.Message.Metadata)
			select {
			case eventChan <- ev:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		var (
			partial          = Message{Role: RoleAssistant, Timestamp: time.Now()}
			textIdx          = -1
			thinkIdx         = -1
			toolBlockIndices = make(map[string]int)
			toolArgs         = make(map[string][]byte)
		)

		resp, err := litellm.Handle(stream, func(ev litellm.Event) error {
			switch e := ev.(type) {
			case litellm.ReasoningDelta:
				if e.Text == "" {
					return nil
				}
				if thinkIdx < 0 {
					partial.Content = append(partial.Content, ThinkingBlock(""))
					thinkIdx = len(partial.Content) - 1
					if err := emit(StreamEvent{Type: StreamEventThinkingStart, ContentIndex: thinkIdx, Message: partial}); err != nil {
						return err
					}
				}
				partial.Content[thinkIdx].Thinking += e.Text
				if err := emit(StreamEvent{Type: StreamEventThinkingDelta, ContentIndex: thinkIdx, Delta: e.Text, Message: partial}); err != nil {
					return err
				}
			case litellm.ContentDelta:
				if textIdx < 0 {
					partial.Content = append(partial.Content, TextBlock(""))
					textIdx = len(partial.Content) - 1
					if err := emit(StreamEvent{Type: StreamEventTextStart, ContentIndex: textIdx, Message: partial}); err != nil {
						return err
					}
				}
				partial.Content[textIdx].Text += e.Text
				if err := emit(StreamEvent{Type: StreamEventTextDelta, ContentIndex: textIdx, Delta: e.Text, Message: partial}); err != nil {
					return err
				}
			case litellm.RefusalDelta:
				if e.Text == "" {
					return nil
				}
				if textIdx < 0 {
					partial.Content = append(partial.Content, TextBlock(""))
					textIdx = len(partial.Content) - 1
					if err := emit(StreamEvent{Type: StreamEventTextStart, ContentIndex: textIdx, Message: partial}); err != nil {
						return err
					}
				}
				partial.Content[textIdx].Text += e.Text
				if partial.Metadata == nil {
					partial.Metadata = make(map[string]any)
				}
				refusal, _ := partial.Metadata["refusal"].(string)
				partial.Metadata["refusal"] = refusal + e.Text
				if err := emit(StreamEvent{Type: StreamEventTextDelta, ContentIndex: textIdx, Delta: e.Text, Message: partial}); err != nil {
					return err
				}
			case litellm.ToolUseStart:
				key := toolUseEventKey(e.ID, e.ItemID, e.Index, e.OutputIndex)
				partial.Content = append(partial.Content, ToolCallBlock(ToolCall{ID: e.ID, Name: e.Name, ThoughtSignature: e.Signature}))
				idx := len(partial.Content) - 1
				if key != "" {
					toolBlockIndices[key] = idx
				}
				if err := emit(StreamEvent{Type: StreamEventToolCallStart, ContentIndex: idx, ToolCallID: e.ID, Message: partial}); err != nil {
					return err
				}
			case litellm.ToolUseDelta:
				key := toolUseEventKey(e.ID, e.ItemID, e.Index, e.OutputIndex)
				idx := findPendingToolCallBlock(partial.Content, toolBlockIndices, key)
				if idx >= 0 {
					block := partial.Content[idx]
					if block.ToolCall != nil {
						if e.ID != "" && block.ToolCall.ID == "" {
							block.ToolCall.ID = e.ID
						}
						if e.Signature != "" && block.ToolCall.ThoughtSignature == "" {
							block.ToolCall.ThoughtSignature = e.Signature
						}
						partial.Content[idx] = block
					}
					if key != "" {
						toolBlockIndices[key] = idx
					}
				}
				if len(e.ArgumentsDelta) > 0 {
					toolArgs[key] = append(toolArgs[key], e.ArgumentsDelta...)
					if err := emit(StreamEvent{Type: StreamEventToolCallDelta, ContentIndex: idx, ToolCallID: e.ID, Delta: string(e.ArgumentsDelta), Message: partial}); err != nil {
						return err
					}
				}
			case litellm.ToolUseDone:
				key := toolUseEventKey(e.ID, e.ItemID, e.Index, e.OutputIndex)
				idx := findPendingToolCallBlock(partial.Content, toolBlockIndices, key)
				var current ToolCall
				if idx >= 0 && partial.Content[idx].ToolCall != nil {
					current = *partial.Content[idx].ToolCall
				}
				if current.ID == "" {
					current.ID = e.ID
				}
				args := string(toolArgs[key])
				completed := buildToolCall(current.ID, current.Name, args, current.ThoughtSignature)
				if idx >= 0 {
					partial.Content[idx] = ToolCallBlock(completed)
				} else {
					partial.Content = append(partial.Content, ToolCallBlock(completed))
					idx = len(partial.Content) - 1
				}
				if err := emit(StreamEvent{Type: StreamEventToolCallEnd, ContentIndex: idx, ToolCallID: completed.ID, Message: partial, CompletedToolCall: &completed}); err != nil {
					return err
				}
			case litellm.UsageEvent:
				u := e.Usage
				partial.Usage = &Usage{Provider: l.ProviderName(), Model: l.model, Input: u.InputTokens, Output: u.OutputTokens, CacheRead: u.CacheReadTokens, CacheWrite: u.CacheWriteTokens, TotalTokens: u.TotalTokens}
				partial.Usage.Cost = CalculateCost(pricingForUsage(l.Info().Pricing, partial.Usage), partial.Usage)
				if err := emit(StreamEvent{Type: StreamEventUsage, Usage: partial.Usage}); err != nil {
					return err
				}
			}
			return nil
		})

		if textIdx >= 0 {
			if emit(StreamEvent{Type: StreamEventTextEnd, ContentIndex: textIdx, Message: partial}) != nil {
				return
			}
		}
		if thinkIdx >= 0 {
			if emit(StreamEvent{Type: StreamEventThinkingEnd, ContentIndex: thinkIdx, Message: partial}) != nil {
				return
			}
		}

		if err != nil {
			if emit(StreamEvent{Type: StreamEventError, Err: wrapProviderError(err)}) != nil {
				return
			}
			return
		}

		partial.Content = finalizePendingStreamToolCalls(partial.Content, toolBlockIndices, toolArgs)
		if resp != nil && resp.Usage.HasTokens() {
			u := resp.Usage
			provider, model := responseUsageModel(resp)
			partial.Usage = &Usage{
				Provider:    provider,
				Model:       model,
				Input:       u.InputTokens,
				Output:      u.OutputTokens,
				CacheRead:   u.CacheReadTokens,
				CacheWrite:  u.CacheWriteTokens,
				TotalTokens: u.TotalTokens,
			}
			partial.Usage.Cost = CalculateCost(pricingForUsage(l.Info().Pricing, partial.Usage), partial.Usage)
		}
		if resp != nil {
			partial.StopReason = mapStopReason(resp.FinishReason)
			mergeResponseMetadata(&partial, resp)
		}
		if emit(StreamEvent{Type: StreamEventDone, Message: partial, StopReason: partial.StopReason}) != nil {
			return
		}
	}()

	return eventChan, nil
}

func finalizePendingStreamToolCalls(content []ContentBlock, indices map[string]int, argsByKey map[string][]byte) []ContentBlock {
	if len(indices) == 0 {
		return content
	}
	for key, idx := range indices {
		if idx < 0 || idx >= len(content) {
			continue
		}
		block := content[idx]
		if block.ToolCall == nil || len(block.ToolCall.Args) > 0 {
			continue
		}
		tc := *block.ToolCall
		completed := buildToolCall(tc.ID, tc.Name, string(argsByKey[key]), tc.ThoughtSignature)
		content[idx] = ToolCallBlock(completed)
	}
	return content
}

// convertMessages converts Message to litellm.Message.
// Handles multipart content with ordered litellm Blocks.
//
// Drops two classes of assistant turns that strict OpenAI-compatible providers
// reject with "assistant must provide content, reasoning_content or
// tool_calls":
//   - Fully empty turns (no content/reasoning/tool_calls/images at all).
//   - Reasoning-only turns that stopped naturally (stop_reason=stop, no
//     externally-visible action). These are valid on reasoning-aware
//     providers (DeepSeek/GLM/Qwen) which accept reasoning_content on
//     replay, but trip providers that ignore reasoning_content on the
//     request side. Skipping is semantically equivalent to "this turn
//     produced nothing": no text was uttered, no tool was invoked, so the
//     next request lets the model decide afresh from prior history without
//     leaking the discarded internal reasoning back into the transcript.
func convertMessages(messages []Message) []litellm.Message {
	llmMessages := make([]litellm.Message, 0, len(messages))
	for _, msg := range messages {
		converted := convertSingleMessage(msg)
		if msg.Role == RoleAssistant {
			if isEmptyAssistantMessage(converted) {
				continue
			}
			if isReasoningOnlyStopAssistant(msg, converted) {
				continue
			}
		}
		llmMessages = append(llmMessages, converted)
	}
	return llmMessages
}

// isEmptyAssistantMessage reports whether a converted assistant message has
// no payload that any provider could consume.
func isEmptyAssistantMessage(m litellm.Message) bool {
	return len(m.Blocks) == 0
}

// isReasoningOnlyStopAssistant reports whether an assistant turn carried only
// internal reasoning and stopped without any externally-visible action.
func isReasoningOnlyStopAssistant(orig Message, converted litellm.Message) bool {
	if orig.StopReason != StopReasonStop {
		return false
	}
	if len(converted.Blocks) != 1 {
		return false
	}
	_, ok := converted.Blocks[0].(litellm.ReasoningBlock)
	return ok
}

// hasImageContent reports whether any content block is an image.
func hasImageContent(blocks []ContentBlock) bool {
	for _, b := range blocks {
		if b.Type == ContentImage && b.Image != nil {
			return true
		}
	}
	return false
}

// convertSingleMessage converts one Message to litellm.Message.
func convertSingleMessage(msg Message) litellm.Message {
	llmMsg := litellm.Message{Role: litellm.Role(msg.Role)}
	var blocks []litellm.Block
	cache := cacheControlFromMetadata(msg.Metadata)
	if msg.Role == RoleTool {
		toolCallID, _ := msg.Metadata["tool_call_id"].(string)
		isError, _ := msg.Metadata["is_error"].(bool)
		// Cache lands on the outer tool_result block only; nested content
		// cannot carry provider cache breakpoints.
		content := convertAgentBlocks(msg.Content, nil)
		blocks = append(blocks, litellm.ToolResultBlock{ToolUseID: toolCallID, Content: content, IsError: isError, Cache: cache})
	} else {
		blocks = convertAgentBlocks(msg.Content, cache)
	}
	llmMsg.Blocks = blocks
	return llmMsg
}

// cacheControlFromMetadata parses the "cache_control" metadata value into a
// litellm CacheControl. The value is "type" or "type:ttl" — e.g. "ephemeral"
// (provider-default TTL) or "ephemeral:1h" (extended TTL where supported).
// TTL validity is enforced by the provider layer.
func cacheControlFromMetadata(metadata map[string]any) *litellm.CacheControl {
	value, _ := metadata["cache_control"].(string)
	if value == "" {
		return nil
	}
	if typ, ttl, ok := strings.Cut(value, ":"); ok {
		return &litellm.CacheControl{Type: typ, TTL: ttl}
	}
	return &litellm.CacheControl{Type: value}
}

func cloneCacheControl(cache *litellm.CacheControl) *litellm.CacheControl {
	if cache == nil {
		return nil
	}
	return &litellm.CacheControl{Type: cache.Type, TTL: cache.TTL}
}

// convertAgentBlocks converts content blocks to litellm blocks. A non-nil
// cache is attached to the LAST converted block only: message-level
// cache_control means "write one breakpoint after this message", and marking
// every block would burn one provider breakpoint per block (Anthropic allows
// at most 4 per request).
func convertAgentBlocks(content []ContentBlock, cache *litellm.CacheControl) []litellm.Block {
	blocks := make([]litellm.Block, 0, len(content))
	for _, b := range content {
		switch b.Type {
		case ContentText:
			if b.Text != "" {
				blocks = append(blocks, litellm.TextBlock{Text: b.Text})
			}
		case ContentThinking:
			if b.Thinking != "" {
				blocks = append(blocks, litellm.ReasoningBlock{Text: b.Thinking})
			}
		case ContentImage:
			if b.Image != nil {
				blocks = append(blocks, convertImageBlock(*b.Image))
			}
		case ContentToolCall:
			if b.ToolCall != nil {
				tc := sanitizeOutgoingToolCall(*b.ToolCall)
				blocks = append(blocks, litellm.ToolUseBlock{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Args,
					Signature: tc.ThoughtSignature,
				})
			}
		case ContentToolRef:
			if b.ToolName != "" {
				blocks = append(blocks, litellm.ToolReferenceBlock{ToolName: b.ToolName})
			}
		}
	}
	if cache != nil {
		// Anthropic rejects cache_control on thinking blocks — land the
		// breakpoint on the last cacheable block instead.
		for i := len(blocks) - 1; i >= 0; i-- {
			if _, isReasoning := blocks[i].(litellm.ReasoningBlock); isReasoning {
				continue
			}
			blocks[i] = withBlockCache(blocks[i], cache)
			break
		}
	}
	return blocks
}

// withBlockCache returns a copy of the block with cache_control attached.
// Blocks are value types, so mutating the copy never touches the caller's.
func withBlockCache(b litellm.Block, cache *litellm.CacheControl) litellm.Block {
	cc := cloneCacheControl(cache)
	switch v := b.(type) {
	case litellm.TextBlock:
		v.Cache = cc
		return v
	case litellm.ImageBlock:
		v.Cache = cc
		return v
	case litellm.ToolUseBlock:
		v.Cache = cc
		return v
	case litellm.ToolReferenceBlock:
		v.Cache = cc
		return v
	}
	return b
}

func sanitizeOutgoingToolCall(tc ToolCall) ToolCall {
	if len(tc.Args) > 0 && json.Valid(tc.Args) {
		return tc
	}
	raw := string(tc.Args)
	if raw == "" {
		raw = tc.ArgsRawText
	}
	n := normalizeArgs(raw)
	tc.Args = n.Args
	if n.Invalid {
		tc.ArgsInvalid = true
		if tc.ArgsRawText == "" {
			tc.ArgsRawText = n.RawText
		}
		if tc.ArgsParseError == "" {
			tc.ArgsParseError = n.ParseErr
		}
	}
	return tc
}

func convertImageBlock(img ImageData) litellm.ImageBlock {
	if img.URL != "" {
		return litellm.ImageBlock{URL: img.URL, MIME: img.MimeType}
	}
	return litellm.ImageBlock{Data: []byte(img.Data), MIME: img.MimeType}
}

// convertResponse converts litellm.Response to Message with content blocks.
func convertResponse(response *litellm.Response) Message {
	content := convertResponseContent(response)

	var usage *Usage
	if response != nil && response.Usage.HasTokens() {
		provider, model := responseUsageModel(response)
		usage = &Usage{
			Provider:    provider,
			Model:       model,
			Input:       response.Usage.InputTokens,
			Output:      response.Usage.OutputTokens,
			CacheRead:   response.Usage.CacheReadTokens,
			CacheWrite:  response.Usage.CacheWriteTokens,
			TotalTokens: response.Usage.TotalTokens,
		}
	}

	finish := StopReasonStop
	if response != nil {
		finish = mapStopReason(response.FinishReason)
	}
	return Message{
		Role:       RoleAssistant,
		Content:    content,
		StopReason: finish,
		Usage:      usage,
		Metadata:   responseMetadata(response),
	}
}

func responseMetadata(response *litellm.Response) map[string]any {
	if response == nil {
		return nil
	}
	var metadata map[string]any
	if response.Refusal != "" {
		metadata = map[string]any{"refusal": response.Refusal}
	}
	if response.FinishReasonRaw != "" {
		if metadata == nil {
			metadata = make(map[string]any)
		}
		metadata["finish_reason_raw"] = response.FinishReasonRaw
	}
	return metadata
}

func mergeResponseMetadata(message *Message, response *litellm.Response) {
	if message == nil {
		return
	}
	for key, value := range responseMetadata(response) {
		if message.Metadata == nil {
			message.Metadata = make(map[string]any)
		}
		message.Metadata[key] = value
	}
}

func convertResponseContent(response *litellm.Response) []ContentBlock {
	if response == nil {
		return nil
	}
	var content []ContentBlock
	for _, block := range response.Blocks {
		switch b := block.(type) {
		case litellm.TextBlock:
			content = append(content, TextBlock(b.Text))
		case litellm.ReasoningBlock:
			if b.Text != "" {
				content = append(content, ThinkingBlock(b.Text))
			}
		case litellm.ToolUseBlock:
			content = append(content, ToolCallBlock(buildToolCall(b.ID, b.Name, string(b.Arguments), b.Signature)))
		}
	}
	return content
}

func responseUsageModel(response *litellm.Response) (provider, model string) {
	if response == nil {
		return "", ""
	}
	provider = response.Usage.Provider
	model = response.Usage.Model
	if provider == "" {
		provider = response.Provider
	}
	if model == "" {
		model = response.Model
	}
	return provider, model
}

func pricingForUsage(fallback *ModelPricing, usage *Usage) *ModelPricing {
	if usage == nil || usage.Model == "" {
		return fallback
	}
	entry, ok := models.DefaultRegistry().Resolve(usage.Model)
	name := usage.Model
	if slash := strings.LastIndexByte(name, '/'); slash >= 0 {
		name = name[slash+1:]
	}
	// Resolve also supports search for interactive model selection. Billing
	// must use identity matching, never a fuzzy match for a custom model.
	if !ok || !models.SameModelID(entry.ID, name) {
		return fallback
	}
	return &ModelPricing{
		InputPerToken:      entry.InputCostPer1M / 1_000_000,
		OutputPerToken:     entry.OutputCostPer1M / 1_000_000,
		CacheReadPerToken:  entry.CacheReadCostPer1M / 1_000_000,
		CacheWritePerToken: entry.CacheWriteCostPer1M / 1_000_000,
	}
}

// mapStopReason maps litellm canonical FinishReason to agentcore StopReason.
func mapStopReason(reason litellm.FinishReason) StopReason {
	switch reason {
	case litellm.FinishReasonStop, litellm.FinishReason(""):
		return StopReasonStop
	case litellm.FinishReasonLength:
		return StopReasonLength
	case litellm.FinishReasonToolCall:
		return StopReasonToolUse
	case litellm.FinishReasonError:
		return StopReasonError
	case litellm.FinishReasonSafety:
		return StopReasonSafety
	default:
		return StopReason(string(reason))
	}
}

type apiKeyOverrideKey struct{}

func contextWithAPIKey(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}
	return context.WithValue(ctx, apiKeyOverrideKey{}, key)
}

func apiKeyFunc(defaultKey string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if key, ok := ctx.Value(apiKeyOverrideKey{}).(string); ok && key != "" {
			return key, nil
		}
		return defaultKey, nil
	}
}

func applySamplingConfig(req *litellm.Request, cfg *GenerationConfig) {
	if req == nil || cfg == nil {
		return
	}
	if cfg.Temperature != DefaultGenerationConfig.Temperature {
		req.Temperature = &cfg.Temperature
	}
}

// applyCallConfig resolves CallOptions once and applies API key, thinking,
// session config, cache routing, and response format to the litellm request.
// caps carries the provider's declared capabilities so capability-gated
// options are dropped instead of tripping strict provider-option validation.
func applyCallConfig(ctx context.Context, req *litellm.Request, opts []CallOption, caps litellm.Capabilities) (context.Context, error) {
	callCfg := ResolveCallConfig(opts)
	ctx = contextWithAPIKey(ctx, callCfg.APIKey)

	// Thinking level + budget.
	// Anthropic budget-token thinking requires temperature=1.
	switch callCfg.ThinkingLevel {
	case "":
		// Leave unspecified to allow model/provider defaults.
	case ThinkingOff:
		req.Thinking = &litellm.Thinking{Mode: litellm.ThinkingDisabled}
	default:
		req.Thinking = &litellm.Thinking{Mode: litellm.ThinkingEnabled, Effort: string(callCfg.ThinkingLevel)}
		if callCfg.ThinkingBudget > 0 {
			req.Thinking.BudgetTokens = &callCfg.ThinkingBudget
		}
		// Each provider owns its thinking-time sampling constraint: Anthropic
		// wants temperature=1 (omitting it defaults to 1), while mimo forbids a
		// custom temperature/top_p entirely. Clear the injected default and let
		// the provider decide instead of forcing an Anthropic-specific value.
		req.Temperature = nil
	}

	if callCfg.SessionID != "" {
		if req.ProviderOptions == nil {
			req.ProviderOptions = make(litellm.ProviderOptions)
		}
		req.ProviderOptions["session_id"] = callCfg.SessionID
	}

	// Prompt-cache routing identity. Capability-gated: litellm providers
	// validate provider options strictly, so an unsupported key must be
	// dropped here rather than rejected there.
	if callCfg.PromptCacheKey != "" && caps.Cache.PromptKey == litellm.SupportYes {
		if req.ProviderOptions == nil {
			req.ProviderOptions = make(litellm.ProviderOptions)
		}
		req.ProviderOptions["prompt_cache_key"] = callCfg.PromptCacheKey
	}

	if callCfg.MaxTokens > 0 {
		req.MaxTokens = &callCfg.MaxTokens
	}

	if callCfg.ToolChoice != nil {
		req.ToolChoice = callCfg.ToolChoice
	}

	if callCfg.ResponseFormat != nil {
		format, err := convertResponseFormat(callCfg.ResponseFormat)
		if err != nil {
			return ctx, err
		}
		req.ResponseFormat = format
	}
	return ctx, nil
}

func convertResponseFormat(format *ResponseFormat) (*litellm.ResponseFormat, error) {
	if format == nil {
		return nil, nil
	}
	out := &litellm.ResponseFormat{
		Type: litellm.ResponseFormatType(format.Type),
	}
	if format.JSONSchema != nil {
		schema, err := litellm.SchemaFrom(format.JSONSchema.Schema)
		if err != nil {
			return nil, fmt.Errorf("llm: response format schema: %w", err)
		}
		out.JSONSchema = &litellm.JSONSchema{
			Name:        format.JSONSchema.Name,
			Description: format.JSONSchema.Description,
			Schema:      schema,
			Strict:      strictMode(format.JSONSchema.Strict),
		}
	}
	return out, nil
}

func applyToolConfig(request *litellm.Request, tools []ToolSpec) error {
	if len(tools) == 0 {
		return nil
	}
	ltTools := make([]litellm.Tool, 0, len(tools))
	for _, t := range tools {
		if t.Name == "" {
			continue
		}
		schema, err := litellm.SchemaFrom(t.Parameters)
		if err != nil {
			return fmt.Errorf("llm: tool %q schema: %w", t.Name, err)
		}
		ltTools = append(ltTools, litellm.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schema,
			Strict:      strictMode(t.Strict),
		})
	}
	request.Tools = ltTools
	return nil
}

func strictMode(v *bool) litellm.StrictMode {
	if v == nil {
		return litellm.StrictDefault
	}
	if *v {
		return litellm.StrictEnabled
	}
	return litellm.StrictDisabled
}

func newProvider(name string, cfg providerConfig) (litellm.Provider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	headers, err := headersFromExtra(cfg.Extra)
	if err != nil {
		return nil, err
	}
	userAgent := stringFromExtra(cfg.Extra, "user_agent")
	switch name {
	case "openai":
		return openai.New(openai.Config{APIKeyFunc: apiKeyFunc(cfg.APIKey), BaseURL: cfg.BaseURL, API: firstStringFromExtra(cfg.Extra, "api", "api_mode"), Headers: headers, UserAgent: userAgent, PromptCacheParams: boolFromExtra(cfg.Extra, "prompt_cache_params")})
	case "anthropic":
		return anthropic.New(anthropic.Config{APIKeyFunc: apiKeyFunc(cfg.APIKey), BaseURL: cfg.BaseURL, Beta: stringFromExtra(cfg.Extra, "anthropic_beta"), Headers: headers, UserAgent: userAgent})
	case "bedrock":
		bedrockCfg, err := bedrockConfig(cfg)
		if err != nil {
			return nil, err
		}
		return bedrock.New(bedrockCfg)
	case "gemini":
		return gemini.New(gemini.Config{APIKeyFunc: apiKeyFunc(cfg.APIKey), BaseURL: cfg.BaseURL})
	case "deepseek":
		return deepseek.New(compatProviderConfig(cfg, headers, userAgent))
	case "glm":
		return glm.New(compatProviderConfig(cfg, headers, userAgent))
	case "grok":
		return grok.New(compatProviderConfig(cfg, headers, userAgent))
	case "minimax":
		return minimax.New(compatProviderConfig(cfg, headers, userAgent))
	case "mimo":
		return mimo.New(compatProviderConfig(cfg, headers, userAgent))
	case "ollama":
		return ollama.New(compatProviderConfig(cfg, headers, userAgent))
	case opencodego.ProviderName:
		return opencodego.New(compatProviderConfig(cfg, headers, userAgent))
	case "openrouter":
		return openrouter.New(compatProviderConfig(cfg, headers, userAgent))
	case "qwen":
		return qwen.New(compatProviderConfig(cfg, headers, userAgent))
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}

func compatProviderConfig(cfg providerConfig, headers map[string]string, userAgent string) compat.Config {
	return compat.Config{
		APIKeyFunc: apiKeyFunc(cfg.APIKey),
		BaseURL:    cfg.BaseURL,

		Headers:                     headers,
		UserAgent:                   userAgent,
		AllowUnknownProviderOptions: true,
	}
}

func bedrockConfig(cfg providerConfig) (bedrock.Config, error) {
	accessKeyID := firstStringFromExtra(cfg.Extra, "access_key_id", "aws_access_key_id")
	secretAccessKey := firstStringFromExtra(cfg.Extra, "secret_access_key", "aws_secret_access_key")
	if accessKeyID == "" || secretAccessKey == "" {
		return bedrock.Config{}, fmt.Errorf("bedrock: access_key_id and secret_access_key are required in provider extra")
	}
	return bedrock.Config{
		Region:              firstStringFromExtra(cfg.Extra, "region", "aws_region"),
		BaseURL:             cfg.BaseURL,
		ControlPlaneBaseURL: stringFromExtra(cfg.Extra, "control_plane_base_url"),
		Credentials: bedrock.StaticCredentials(
			accessKeyID,
			secretAccessKey,
			firstStringFromExtra(cfg.Extra, "session_token", "aws_session_token"),
		),
	}, nil
}

func headersFromExtra(extra map[string]any) (map[string]string, error) {
	raw, ok := extra["headers"]
	if !ok || raw == nil {
		return nil, nil
	}
	switch h := raw.(type) {
	case map[string]string:
		return maps.Clone(h), nil
	case map[string]any:
		out := make(map[string]string, len(h))
		for k, v := range h {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("llm: provider header %q must be string", k)
			}
			out[k] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("llm: provider headers must be map[string]string")
	}
}

func stringFromExtra(extra map[string]any, key string) string {
	v, _ := extra[key].(string)
	return v
}

// boolFromExtra accepts both JSON booleans and "true" strings so that
// stringly-typed config files work without surprises.
func boolFromExtra(extra map[string]any, key string) bool {
	switch v := extra[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}

func firstStringFromExtra(extra map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromExtra(extra, key); value != "" {
			return value
		}
	}
	return ""
}

// normalizedArgs is the parsed shape of raw LLM tool-call args.
//   - Args is always valid JSON so the parent ToolCall stays JSON-serializable
//     (json.RawMessage marshalling validates its bytes; invalid args here would
//     break agent.ExportMessages → json.Marshal for persistence).
//   - When the raw payload was malformed, Args is the "{}" placeholder, and
//     RawText + ParseErr carry the original bytes and parser diagnostic so
//     downstream validation can point at the true root cause (stream
//     truncation, provider bug) instead of "missing field" against {}.
type normalizedArgs struct {
	Args     json.RawMessage
	Invalid  bool
	RawText  string
	ParseErr string
}

func normalizeArgs(raw string) normalizedArgs {
	if raw == "" {
		return normalizedArgs{Args: json.RawMessage("{}")}
	}
	if json.Valid([]byte(raw)) {
		return normalizedArgs{Args: json.RawMessage(raw)}
	}
	var probe any
	parseErr := json.Unmarshal([]byte(raw), &probe)
	return normalizedArgs{
		Args:     json.RawMessage("{}"),
		Invalid:  true,
		RawText:  raw,
		ParseErr: parseErr.Error(),
	}
}

// buildToolCall constructs an agentcore.ToolCall from raw litellm fields,
// routing malformed args into dedicated diagnostic fields (see ToolCall doc).
func buildToolCall(id, name, rawArgs, thoughtSignature string) ToolCall {
	n := normalizeArgs(rawArgs)
	return ToolCall{
		ID:               id,
		Name:             name,
		Args:             n.Args,
		ArgsInvalid:      n.Invalid,
		ArgsRawText:      n.RawText,
		ArgsParseError:   n.ParseErr,
		ThoughtSignature: thoughtSignature,
	}
}

func toolUseEventKey(id, itemID string, index, outputIndex *int) string {
	switch {
	case id != "":
		return "id:" + id
	case itemID != "":
		return "item:" + itemID
	case index != nil:
		return fmt.Sprintf("index:%d", *index)
	case outputIndex != nil:
		return fmt.Sprintf("output:%d", *outputIndex)
	default:
		return ""
	}
}

func findPendingToolCallBlock(content []ContentBlock, byID map[string]int, toolCallID string) int {
	if toolCallID != "" {
		if idx, ok := byID[toolCallID]; ok && idx >= 0 && idx < len(content) {
			if block := content[idx]; block.ToolCall != nil {
				return idx
			}
		}
	}
	for i := len(content) - 1; i >= 0; i-- {
		block := content[i]
		if block.ToolCall == nil {
			continue
		}
		if toolCallID != "" && block.ToolCall.ID == toolCallID {
			return i
		}
		if len(block.ToolCall.Args) == 0 {
			return i
		}
	}
	return -1
}
