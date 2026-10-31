package web

import (
	"encoding/json"
	"github.com/datnt1998/verse/core/llmconfig"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/ledger"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func TestHostedLedgerProjectArchitectConfig(t *testing.T) {
	f := provisionHostedGateway(t, "ledger-config", "story")
	tg := &testGateway{t: t, g: f.g}
	owner := tg.login("hosted-owner", "mat-khau-12345")
	viewer := tg.login("hosted-viewer", "mat-khau-12345")
	outsider := tg.login("hosted-outsider", "mat-khau-12345")
	var calls atomic.Int32
	values := ledgerValues("hosted-architect")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		calls.Add(1)
		if request.Model != "project-architect" {
			http.Error(w, "ledger must select the project architect", http.StatusBadRequest)
			return
		}
		proposal, err := json.Marshal(ledger.Proposal{Values: values, Brief: "Yêu cầu đã đổi", Changed: []int{4, 6}})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": "hosted-ledger", "object": "chat.completion", "model": request.Model,
			"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(proposal)}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	t.Cleanup(server.Close)
	// Credentials are global; the default model and architect selection live only
	// in the project document. A sibling story override must not affect the ledger.
	global := bootstrap.Config{Providers: map[string]llmconfig.ProviderConfig{
		"ledger": {Type: "openai", APIKey: "test-only", BaseURL: server.URL + "/v1"},
	}}
	if err := bootstrap.SaveConfig(bootstrap.DefaultConfigPath(), global); err != nil {
		t.Fatal(err)
	}
	actor := &postgres.Actor{Username: f.owner.Username}
	docs := f.repo.ConfigDocuments()
	put := func(story, body string) {
		t.Helper()
		if err := docs.Put(t.Context(), actor, f.project, story, []byte(body), ""); err != nil {
			t.Fatal(err)
		}
	}
	put("", `{"provider":"ledger","model":"project-default","roles":{"architect":{"provider":"ledger","model":"project-architect"}}}`)
	put(f.story, `{"roles":{"architect":{"provider":"ledger","model":"story-architect"}}}`)
	base := "/api/projects/" + f.project + "/ledger"
	for _, action := range []string{"check", "suggest"} {
		t.Run(action, func(t *testing.T) {
			w := tg.do(http.MethodPost, base+"/"+action, owner, `{"brief":"Yêu cầu truyện"}`)
			if w.Code != http.StatusOK {
				t.Fatalf("hosted ledger %s must use project config: %d %s", action, w.Code, w.Body.String())
			}
			var response struct {
				Values ledger.Values `json:"values"`
				Brief  string        `json:"brief"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Values != values || action == "suggest" && response.Brief != "Yêu cầu đã đổi" {
				t.Fatalf("wrong project architect result: %s", w.Body.String())
			}
		})
	}
	before := calls.Load()
	for _, denied := range []struct {
		cookie string
		status int
	}{{viewer, http.StatusForbidden}, {outsider, http.StatusForbidden}} {
		w := tg.do(http.MethodPost, base+"/check", denied.cookie, `{"brief":"Yêu cầu truyện"}`)
		if w.Code != denied.status {
			t.Fatalf("ledger permission: %d want %d: %s", w.Code, denied.status, w.Body.String())
		}
	}
	if calls.Load() != before {
		t.Fatal("denied request invoked the model")
	}
	// Broken project configuration must fail closed, not read gateway cwd or
	// silently use a working global model instead.
	global.Provider, global.ModelName = "ledger", "project-architect"
	if err := bootstrap.SaveConfig(bootstrap.DefaultConfigPath(), global); err != nil {
		t.Fatal(err)
	}
	put("", `{"roles":{"architect":{"provider":"missing","model":"invalid"}}}`)
	w := tg.do(http.MethodPost, base+"/check", owner, `{"brief":"Yêu cầu truyện"}`)
	if w.Code != http.StatusBadGateway || calls.Load() != before {
		t.Fatalf("invalid project config fell back to global: %d %s (calls %d -> %d)", w.Code, w.Body.String(), before, calls.Load())
	}
	global.Provider, global.ModelName = "", ""
	if err := bootstrap.SaveConfig(bootstrap.DefaultConfigPath(), global); err != nil {
		t.Fatal(err)
	}
	put("", `{}`)
	w = tg.do(http.MethodPost, base+"/check", owner, `{"brief":"Yêu cầu truyện"}`)
	if w.Code != http.StatusConflict || decodeResp(t, w)["needsConfig"] != true {
		t.Fatalf("missing model must remain actionable: %d %s", w.Code, w.Body.String())
	}
}
