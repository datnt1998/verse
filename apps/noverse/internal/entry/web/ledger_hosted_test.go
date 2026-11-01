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
	"github.com/datnt1998/verse/core/roles"
)

func TestHostedLedgerUsesGlobalArchitectAcrossScopes(t *testing.T) {
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
		if request.Model != "global-architect" {
			http.Error(w, "ledger must select the app-wide architect", http.StatusBadRequest)
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
	// AU: credentials and every model/role selection are app-wide. Legacy
	// project/story pins cannot redirect the ledger to a stale provider/model.
	global := bootstrap.Config{
		Provider: "ledger", ModelName: "global-default",
		Roles: map[string]roles.RoleConfig{"architect": {Provider: "ledger", Model: "global-architect"}},
		Providers: map[string]llmconfig.ProviderConfig{
			"ledger": {Type: "openai", APIKey: "test-only", BaseURL: server.URL + "/v1"},
		},
	}
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
				t.Fatalf("hosted ledger %s must use app-wide config: %d %s", action, w.Code, w.Body.String())
			}
			var response struct {
				Values ledger.Values `json:"values"`
				Brief  string        `json:"brief"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Values != values || action == "suggest" && response.Brief != "Yêu cầu đã đổi" {
				t.Fatalf("wrong app-wide architect result: %s", w.Body.String())
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
	// A forged stale project pin is ignored and physically migrated, while
	// unrelated project settings survive.
	put("", `{"roles":{"architect":{"provider":"missing","model":"invalid"}},"style":"romance"}`)
	w := tg.do(http.MethodPost, base+"/check", owner, `{"brief":"Yêu cầu truyện"}`)
	if w.Code != http.StatusOK || calls.Load() != before+1 {
		t.Fatalf("legacy scoped pin redirected ledger: %d %s", w.Code, w.Body.String())
	}
	stored, _, err := docs.Get(t.Context(), actor, f.project, "")
	if err != nil {
		t.Fatal(err)
	}
	var migrated map[string]any
	if err := json.Unmarshal(stored, &migrated); err != nil {
		t.Fatal(err)
	}
	if _, pinned := migrated["roles"]; pinned || migrated["style"] != "romance" {
		t.Fatalf("migration lost non-model data or retained the role pin: %s", stored)
	}
	// Invalid global selection still fails closed without invoking an upstream.
	before = calls.Load()
	global.Roles["architect"] = roles.RoleConfig{Provider: "missing", Model: "invalid"}
	if err := bootstrap.SaveConfig(bootstrap.DefaultConfigPath(), global); err != nil {
		t.Fatal(err)
	}
	w = tg.do(http.MethodPost, base+"/check", owner, `{"brief":"Yêu cầu truyện"}`)
	if w.Code != http.StatusBadGateway || calls.Load() != before {
		t.Fatalf("invalid app-wide config did not fail closed: %d %s", w.Code, w.Body.String())
	}
	global.Provider, global.ModelName, global.Roles = "", "", nil
	if err := bootstrap.SaveConfig(bootstrap.DefaultConfigPath(), global); err != nil {
		t.Fatal(err)
	}
	put("", `{}`)
	w = tg.do(http.MethodPost, base+"/check", owner, `{"brief":"Yêu cầu truyện"}`)
	if w.Code != http.StatusConflict || decodeResp(t, w)["needsConfig"] != true {
		t.Fatalf("missing model must remain actionable: %d %s", w.Code, w.Body.String())
	}
}
