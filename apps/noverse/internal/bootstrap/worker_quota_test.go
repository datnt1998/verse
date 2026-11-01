package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	llmtypes "github.com/datnt1998/verse/core/llm"
	litellm "github.com/datnt1998/verse/core/llm/wire"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/llmretry"
	"github.com/datnt1998/verse/core/roles"
)

// A real 429 must reach the worker engine with Retry-After intact instead of
// sleeping in the request retry gate or switching invisibly before AU policy.
func TestWorkerQuotaDefersRetryAndRequestFailoverToEngine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var primaryCalls, fallbackCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		w.Header().Set("Retry-After", "240")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limit","type":"rate_limit_exceeded"}}`))
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		t.Error("request-level fallback bypassed engine policy")
		w.WriteHeader(500)
	}))
	defer fallback.Close()
	cfg := Config{Provider: "a", ModelName: "a", Providers: map[string]llmconfig.ProviderConfig{
		"a": {Type: "openai", APIKey: "a", BaseURL: primary.URL + "/v1"}, "b": {Type: "openai", APIKey: "b", BaseURL: fallback.URL + "/v1"},
	}, Roles: map[string]roles.RoleConfig{"writer": {Provider: "a", Model: "a", Fallbacks: []roles.ModelRef{{Provider: "b", Model: "b"}}}}}
	models, err := NewModelSet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	models.SetRetryGate(llmretry.NewGate())
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err = models.ForWorkerRole("writer", nil).Generate(ctx, []llmtypes.Message{llmtypes.UserMsg("test")}, nil)
	if !errors.Is(err, llmtypes.ErrProviderRateLimit) {
		t.Fatalf("quota must remain classified, got %v", err)
	}
	var retry interface{ Retryable() bool }
	if !errors.As(err, &retry) || retry.Retryable() {
		t.Fatalf("kernel would retry before engine: %v", err)
	}
	var upstream *litellm.LiteLLMError
	if !errors.As(err, &upstream) || upstream.RetryAfter != 240 {
		t.Fatalf("Retry-After lost: %v", err)
	}
	if primaryCalls.Load() != 1 || fallbackCalls.Load() != 0 {
		t.Fatalf("calls primary=%d fallback=%d", primaryCalls.Load(), fallbackCalls.Load())
	}
}
