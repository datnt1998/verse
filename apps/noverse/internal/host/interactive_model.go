package host

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"errors"
	"net"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/llm/wire"

	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
)

// ActionCallTimeout bounds host-side model calls (rule normalization, Arbiter
// decisions) made for one user action, so a slow provider cannot keep the
// action pending forever. Engine writing runs keep their own retry policy.
const ActionCallTimeout = 2 * time.Minute

// providerUnreachableMessage leads every provider transport failure shown to users.
const providerUnreachableMessage = "Không kết nối được nhà cung cấp AI — kiểm tra kết nối và cấu hình nhà cung cấp"

// interactiveModel is used for calls a person is waiting on. A provider that
// cannot be reached fails immediately instead of being retried until the
// context ends; singleAttempt additionally disables retries for every error.
type interactiveModel struct {
	llmtypes.ChatModel
	singleAttempt bool
}

func (m interactiveModel) Generate(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	resp, err := m.ChatModel.Generate(ctx, messages, tools, opts...)
	switch {
	case err == nil:
		return resp, nil
	case providerUnreachable(err):
		return resp, &providerUnreachableError{err}
	case m.singleAttempt:
		return resp, &noRetryError{err}
	}
	return resp, err
}

func (m interactiveModel) GenerateStream(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	ch, err := m.ChatModel.GenerateStream(ctx, messages, tools, opts...)
	switch {
	case err == nil:
		return ch, nil
	case providerUnreachable(err):
		return ch, &providerUnreachableError{err}
	case m.singleAttempt:
		return ch, &noRetryError{err}
	}
	return ch, err
}

func (m interactiveModel) StructuredOutputFacts() llmcontract.ModelFacts {
	if provider, ok := m.ChatModel.(interface{ StructuredOutputFacts() llmcontract.ModelFacts }); ok {
		return provider.StructuredOutputFacts()
	}
	var facts llmcontract.ModelFacts
	if provider, ok := m.ChatModel.(llmtypes.CapabilityProvider); ok {
		facts.Capabilities = provider.Capabilities()
	}
	if provider, ok := m.ChatModel.(interface{ Info() llmtypes.ModelInfo }); ok {
		facts.Info = provider.Info()
	}
	if override, ok := m.ChatModel.(interface{ JSONSchemaOverride() *bool }); ok {
		facts.JSONSchemaOverride = override.JSONSchemaOverride()
	}
	return facts
}

// providerUnreachable reports transport-level failures (refused, DNS, network
// timeout), not provider answers such as 429 or invalid output.
func providerUnreachable(err error) bool {
	var network net.Error
	var provider *litellm.LiteLLMError
	return errors.Is(err, llmtypes.ErrProviderNetwork) || errors.As(err, &network) ||
		(errors.As(err, &provider) && (provider.Type == litellm.ErrorTypeNetwork || provider.Type == litellm.ErrorTypeTimeout))
}

type noRetryError struct{ error }

func (e *noRetryError) Unwrap() error   { return e.error }
func (e *noRetryError) Retryable() bool { return false }

type providerUnreachableError struct{ error }

func (e *providerUnreachableError) Unwrap() error   { return e.error }
func (e *providerUnreachableError) Retryable() bool { return false }
func (e *providerUnreachableError) Error() string {
	return providerUnreachableMessage + " (" + e.error.Error() + ")"
}

// userFacingProviderError returns the unreachable-provider error itself so the
// message leads with the Vietnamese reason instead of internal call-site prefixes.
func userFacingProviderError(err error) error {
	var unreachable *providerUnreachableError
	if errors.As(err, &unreachable) {
		return unreachable
	}
	return err
}

// actionContext derives a bounded context for model calls of one user action.
// Abort cancels it while the engine loop is not running yet.
func (h *Host) actionContext() (context.Context, context.CancelFunc) {
	parent := h.runCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, ActionCallTimeout)
	h.mu.Lock()
	if h.actionCancels == nil {
		h.actionCancels = map[*context.Context]context.CancelFunc{}
	}
	h.actionCancels[&ctx] = cancel
	h.mu.Unlock()
	return ctx, func() {
		h.mu.Lock()
		delete(h.actionCancels, &ctx)
		h.mu.Unlock()
		cancel()
	}
}

// cancelActions stops pending host-side action calls; it reports whether any ran.
func (h *Host) cancelActions() bool {
	h.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(h.actionCancels))
	for _, cancel := range h.actionCancels {
		cancels = append(cancels, cancel)
	}
	h.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return len(cancels) > 0
}
