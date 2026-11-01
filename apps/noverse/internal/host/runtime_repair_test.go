package host

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore/subagent"
	llm "github.com/datnt1998/verse/core/llm"
	wire "github.com/datnt1998/verse/core/llm/wire"
	"github.com/datnt1998/verse/core/llmconfig"
)

func TestSelectionReloadFailurePausesBeforeStaleArbiterAndRetainsDispatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var workerCalls, arbiterCalls int
	worker := &scriptedChatModel{fn: func([]llm.Message) llm.Message { workerCalls++; return testTextMsg("done") }}
	arbiter := &scriptedChatModel{fn: func([]llm.Message) llm.Message {
		arbiterCalls++
		return testTextMsg(`{"action":"abort","reason":"stale"}`)
	}}
	runner := subagent.NewRunner(subagent.Config{Name: "editor", Model: worker})
	e, _, _ := newTestEngine(t, store.NewStore(t.TempDir()), runner, arbiter)
	inst := &flow.Instruction{Agent: "editor", Task: "preserve this exact instruction", Reason: "user dispatch", Chapter: 3}
	loads := 0
	e.prepareWorkerTurn = func() error { loads++; return errors.New("reload unavailable") }
	pauses := 0
	e.onPause = func(string) { pauses++ }
	// Repeated infrastructure failure must never enter the ordinary retry/Arbiter path.
	for range 2 {
		e.failedKey = instructionKey(inst)
		err := e.runWorker(context.Background(), inst)
		if err == nil {
			t.Fatal("expected failed boundary reload")
		}
		if !e.handleWorkerError(context.Background(), inst, err) {
			t.Fatal("reload failure must pause immediately")
		}
	}
	if workerCalls != 0 || arbiterCalls != 0 {
		t.Fatalf("stale requests: worker=%d arbiter=%d, want 0/0", workerCalls, arbiterCalls)
	}
	if pauses != 2 || loads != 2 {
		t.Fatalf("pauses=%d loads=%d", pauses, loads)
	}
	if got := e.takeNext(); got != inst {
		t.Fatalf("dispatch lost: %+v", got)
	}
	e.prepareWorkerTurn = func() error { loads++; return nil }
	if err := e.runWorker(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	if loads != 3 || workerCalls != 1 || arbiterCalls != 0 {
		t.Fatalf("resume did not retry reload: loads=%d worker=%d arbiter=%d", loads, workerCalls, arbiterCalls)
	}
}

func TestStructured429BillingAndCredentialClassification(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, tc := range []struct {
		name, body, kind string
		wait             time.Duration
	}{
		{"code exhaustion", `{"error":{"code":"insufficient_quota","message":"Account unavailable"}}`, "expired", 0},
		{"type exhaustion", `{"error":{"type":"insufficient_quota","message":"Account unavailable"}}`, "expired", 0},
		{"billing", `{"error":{"code":"billing_hard_limit_reached","message":"Account unavailable"}}`, "expired", 0},
		{"true rate limit", `{"error":{"type":"rate_limit_exceeded","message":"Too many requests"}}`, "quota", 37 * time.Second},
		{"expired key", `{"error":{"code":"api_key_expired","type":"insufficient_quota","message":"Please update billing"}}`, "key", 0},
		{"expired token", `{"error":{"code":"token_expired","message":"Your subscription expired"}}`, "key", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := wire.NewHTTPError("a", 429, tc.body, http.Header{"Retry-After": []string{"37"}})
			got := ClassifyProviderFailure(err)
			if got.Kind != tc.kind || got.RetryAfter != tc.wait {
				t.Fatalf("got %+v, want kind=%s wait=%s; parsed=%+v", got, tc.kind, tc.wait, err)
			}
		})
	}
}

func TestLateProviderFailureAfterLiveCutoverKeepsReplacementHealthy(t *testing.T) {
	for _, agent := range []string{"writer", "architect_short", "editor"} {
		for _, status := range []int{401, 402, 429} {
			t.Run(fmt.Sprintf("%s/%d", agent, status), func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				entered, release := make(chan struct{}), make(chan struct{})
				var aCalls, bCalls atomic.Int32
				a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					aCalls.Add(1)
					close(entered)
					<-release
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Retry-After", "240")
					w.WriteHeader(status)
					fmt.Fprint(w, `{"error":{"message":"upstream rejected request"}}`)
				}))
				defer a.Close()
				b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					bCalls.Add(1)
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"id\":\"b\",\"object\":\"chat.completion.chunk\",\"model\":\"b-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Done\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"b\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				}))
				defer b.Close()
				cfg := bootstrap.Config{Provider: "a", ModelName: "a-model", Providers: map[string]llmconfig.ProviderConfig{
					"a": {Type: "openai", APIKey: "test", BaseURL: a.URL + "/v1", Models: []llmconfig.ModelConfig{{Name: "a-model"}}},
					"b": {Type: "openai", APIKey: "test", BaseURL: b.URL + "/v1", Models: []llmconfig.ModelConfig{{Name: "b-model"}}},
				}}
				models, err := bootstrap.NewModelSet(cfg)
				if err != nil {
					t.Fatal(err)
				}
				h := &Host{cfg: cfg, models: models, events: make(chan Event, 16)}
				role := providerRoleOfAgent(agent)
				runner := subagent.NewRunner(subagent.Config{Name: agent, Model: models.ForWorkerRole(role, nil), SystemPrompt: "test"})
				e, _, _ := newTestEngine(t, store.NewStore(t.TempDir()), runner, nil)
				e.providerOf = func(role string) (string, string) { p, m, _ := h.CurrentModelSelection(role); return p, m }
				e.recordProviderFailure = h.recordProviderFailure
				e.providerFallback = h.activateProviderFallback
				e.onPause = func(s string) { t.Errorf("late A failure paused healthy B: %s", s) }
				inst := &flow.Instruction{Agent: agent, Task: "keep exact instruction"}
				result := make(chan error, 1)
				go func() { result <- e.runWorker(context.Background(), inst) }()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("A never received request")
				}
				cfg.Provider, cfg.ModelName = "b", "b-model"
				h.configLoader = func() (bootstrap.Config, error) { return cfg, nil }
				if err := h.ReloadModelSelection(); err != nil {
					close(release)
					t.Fatal(err)
				}
				close(release)
				select {
				case err = <-result:
				case <-time.After(5 * time.Second):
					t.Fatal("A did not finish")
				}
				if err == nil {
					t.Fatal("A must fail")
				}
				policyCtx, cancel := context.WithCancel(context.Background())
				cancel()
				if e.handleWorkerError(policyCtx, inst, err) {
					t.Fatal("late A error stopped engine")
				}
				if failure := h.providerErrorSnapshot(); failure != nil {
					t.Fatalf("new selection diagnosed: %+v", failure)
				}
				if p, m, _ := h.CurrentModelSelection(role); p != "b" || m != "b-model" {
					t.Fatalf("replacement changed: %s/%s", p, m)
				}
				if got := e.takeNext(); got != inst {
					t.Fatalf("instruction lost after stale failure: %+v", got)
				}
				if err := e.runWorker(context.Background(), inst); err != nil {
					t.Fatalf("next request failed: %v", err)
				}
				if aCalls.Load() != 1 || bCalls.Load() != 1 {
					t.Fatalf("requests A=%d B=%d, want 1/1", aCalls.Load(), bCalls.Load())
				}
			})
		}
	}
}
