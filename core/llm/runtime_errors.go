package llm

import (
	"errors"
)

var (
	ErrNoModel         = errors.New("no model configured")
	ErrContextOverflow = errors.New("context window overflow")
)

// Provider runtime sentinels. These categorize errors returned by the model
// adapter at call time (provider API errors, network failures, server
// responses). Use ClassifyProvider to derive the most specific sentinel from
// an error chain, or match directly with errors.Is.
var (
	ErrProviderRateLimit     = errors.New("provider rate limit")
	ErrProviderQuota         = errors.New("provider quota exhausted")
	ErrProviderTimeout       = errors.New("provider timeout")
	ErrProviderStreamIdle    = errors.New("provider stream idle")
	ErrProviderNetwork       = errors.New("provider network")
	ErrProviderAuth          = errors.New("provider auth")
	ErrProviderOverloaded    = errors.New("provider overloaded")
	ErrProviderContentFilter = errors.New("provider content filter")
)

// ContextOverflowError wraps an underlying context-overflow cause (typically
// a provider error). errors.Is matches ErrContextOverflow; Unwrap reaches the
// raw cause so callers can extract provider-specific details if needed.
type ContextOverflowError struct {
	Cause error
}

func (e *ContextOverflowError) Error() string {
	if e.Cause == nil {
		return "context window overflow"
	}
	return "context window overflow: " + e.Cause.Error()
}
func (e *ContextOverflowError) Unwrap() error        { return e.Cause }
func (e *ContextOverflowError) Is(target error) bool { return target == ErrContextOverflow }

// IsContextOverflow reports whether err indicates a context-overflow condition.
// Both the agentcore wrapper (*ContextOverflowError) and adapter-classified
// provider errors map onto ErrContextOverflow, so a single errors.Is covers
// both layers. Convenience for callers that want to detect "request too big"
// without caring where it surfaced.
func IsContextOverflow(err error) bool {
	return errors.Is(err, ErrContextOverflow)
}
