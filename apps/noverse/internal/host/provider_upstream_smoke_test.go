package host

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore/subagent"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
)

// Real OpenAI-compatible upstreams exercise the assembled worker path:
// expired/key or quota beyond two minutes must reach the host policy rather
// than retry/fail over invisibly inside a lower wrapper.
func TestTwoProviderFakeUpstreamFallbackAndNextTurnUsesSecond(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		kind   string
	}{
		{"expired", http.StatusPaymentRequired, `{"error":{"message":"insufficient_quota: plan expired","type":"insufficient_quota"}}`, "expired"},
		{"key", http.StatusUnauthorized, `{"error":{"message":"invalid_api_key","type":"invalid_api_key"}}`, "key"},
		{"quota", http.StatusTooManyRequests, `{"error":{"message":"rate limit","type":"rate_limit_exceeded"}}`, "quota"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var firstCalls, secondCalls atomic.Int32
			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				firstCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "240")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer first.Close()
			second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				secondCalls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"id\":\"second\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"b-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Xong.\"},\"finish_reason\":null}]}\n\n"))
				_, _ = w.Write([]byte("data: {\"id\":\"second\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"b-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1,\"total_tokens\":3}}\n\n"))
				_, _ = w.Write([]byte("data: [DONE]\n\n"))
			}))
			defer second.Close()

			cfg := bootstrap.Config{
				Provider: "a", ModelName: "a-model",
				Providers: map[string]llmconfig.ProviderConfig{
					"a": {Type: "openai", APIKey: "k", BaseURL: first.URL + "/v1", Models: []llmconfig.ModelConfig{{Name: "a-model"}}},
					"b": {Type: "openai", APIKey: "k", BaseURL: second.URL + "/v1", Models: []llmconfig.ModelConfig{{Name: "b-model"}}},
				},
				Roles: map[string]roles.RoleConfig{
					"writer": {
						Provider: "a", Model: "a-model",
						Fallbacks: []roles.ModelRef{{Provider: "b", Model: "b-model"}},
					},
				},
			}
			models, err := bootstrap.NewModelSet(cfg)
			if err != nil {
				t.Fatal(err)
			}
			h := &Host{cfg: cfg, models: models, events: make(chan Event, 8)}

			st := store.NewStore(t.TempDir())
			runner := subagent.NewRunner(subagent.Config{Name: "writer", Model: models.ForWorkerRole("writer", nil), SystemPrompt: "test"})
			e, events, _ := newTestEngine(t, st, runner, nil)
			e.providerOf = func(role string) (string, string) {
				provider, model, _ := h.CurrentModelSelection(role)
				return provider, model
			}
			e.providerFallback = h.activateProviderFallback
			e.recordProviderFailure = h.recordProviderFailure
			e.clearProviderFailure = h.clearProviderFailure

			inst := &flow.Instruction{Agent: "writer", Task: "Viết một câu."}
			runTurn := func() error {
				for range 3 {
					err := e.runWorker(context.Background(), inst)
					if err == nil {
						return nil
					}
					if e.handleWorkerError(context.Background(), inst, err) {
						return err
					}
				}
				return fmt.Errorf("engine không hội tụ")
			}
			if err := runTurn(); err != nil {
				t.Fatalf("lượt đầu phải tự chuyển dự phòng và chạy xong: %v", err)
			}
			if firstCalls.Load() != 1 {
				t.Fatalf("provider đầu phải được gọi đúng 1 lần, nhận %d", firstCalls.Load())
			}
			if secondCalls.Load() != 1 {
				t.Fatalf("provider dự phòng phải nhận lượt chạy, nhận %d", secondCalls.Load())
			}
			if provider, model, _ := h.CurrentModelSelection("writer"); provider != "b" || model != "b-model" {
				t.Fatalf("sau fallback phải dùng provider thứ hai: %s/%s", provider, model)
			}
			select {
			case ev := <-h.events:
				if ev.Kind != "fallback.active" || ev.FromProvider != "a" || ev.Provider != "b" || ev.Reason != tc.kind {
					t.Fatalf("fallback.active sai: %+v", ev)
				}
			default:
				t.Fatalf("thiếu fallback.active; engine events=%+v", *events)
			}
			if tc.kind == "quota" {
				failure := h.providerErrorSnapshot()
				if failure == nil || failure.RetryAt == nil {
					t.Fatalf("quota deadline lost after fallback: %+v", failure)
				}
			}

			// Next turn keeps using the second upstream (no repeated expiry loop).
			if err := e.runWorker(context.Background(), inst); err != nil {
				t.Fatalf("lượt kế tiếp phải dùng provider thứ hai: %v", err)
			}
			if firstCalls.Load() != 1 {
				t.Fatalf("không được gọi lại provider hết hạn, nhận %d", firstCalls.Load())
			}
			if secondCalls.Load() != 2 {
				t.Fatalf("provider thứ hai phải nhận lượt kế tiếp, nhận %d", secondCalls.Load())
			}
		})
	}
}
