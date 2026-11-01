package web

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// Project and story layers are writable by non-admin members, while the API
// key and OAuth token live only in the global layer. A lower layer therefore
// must never choose where those credentials are sent.
func TestConfigLowerLayersCannotRedirectGlobalCredentials(t *testing.T) {
	f := provisionHostedGateway(t, "route-guard", "story")
	tg := &testGateway{t: t, g: f.g}
	ctx := context.Background()

	q := gen.New(f.pool.Pool())
	project, err := q.GetProjectBySlug(ctx, f.project)
	if err != nil {
		t.Fatal(err)
	}
	collabUser, err := q.GetUserByUsername(ctx, "hosted-viewer")
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertMember(ctx, gen.UpsertMemberParams{ProjectID: project.ID, UserID: collabUser.ID, Role: "collab"}); err != nil {
		t.Fatal(err)
	}
	admin := tg.login("hosted-admin", "mat-khau-12345")
	owner := tg.login("hosted-owner", "mat-khau-12345")
	collab := tg.login("hosted-viewer", "mat-khau-12345")
	request := func(method, path, cookie, body string) (int, string) {
		w := doFrom(t, f.g, method, path, cookie, body, "127.0.0.1:1234", nil)
		return w.Code, w.Body.String()
	}

	const globalURL = "https://models.example/v1"
	const globalTTS = "https://tts.example/v1"
	global := `{"provider":"relay","model":"m","providers":{"relay":{"type":"openai","base_url":"` + globalURL + `","api_key":"owner-secret","models":[{"name":"m"}]}},` +
		`"tts":{"engine":"openai-speech","endpoint":"` + globalTTS + `","api_key":"tts-secret"}}`
	if code, body := request(http.MethodPut, "/api/config?scope=global", admin, global); code != 200 {
		t.Fatalf("global write %d: %s", code, body)
	}

	storyURL := "/api/config?scope=story&project=" + f.project + "&story=" + f.story
	projectURL := "/api/config?scope=project&project=" + f.project
	rejected := []struct{ name, url, cookie, body string }{
		{"story base_url", storyURL, collab, `{"providers":{"relay":{"base_url":"https://attacker.example/v1"}}}`},
		{"story type", storyURL, collab, `{"providers":{"relay":{"type":"ollama"}}}`},
		{"story extra headers", storyURL, collab, `{"providers":{"relay":{"extra":{"headers":{"X-Route":"x"}}}}}`},
		{"story new provider base_url", storyURL, collab, `{"providers":{"evil":{"type":"openai","base_url":"http://garage:3903"}}}`},
		{"story tts endpoint", storyURL, collab, `{"tts":{"endpoint":"https://attacker.example/tts"}}`},
		{"project base_url", projectURL, owner, `{"providers":{"relay":{"base_url":"https://attacker.example/v1"}}}`},
		{"project tts endpoint", projectURL, owner, `{"tts":{"endpoint":"https://attacker.example/tts"}}`},
	}
	for _, c := range rejected {
		code, body := request(http.MethodPut, c.url, c.cookie, c.body)
		if code < 400 || code >= 500 {
			t.Errorf("%s: status %d, want 4xx: %s", c.name, code, body)
		}
	}
	// Ordinary lower-layer tuning keeps working.
	if code, body := request(http.MethodPut, storyURL, collab, `{"style":"xianxia","providers":{"relay":{"models":[{"name":"m","json_schema":true}]}}}`); code != 200 {
		t.Fatalf("plain story write %d: %s", code, body)
	}

	docs := f.repo.ConfigDocuments()
	actor := &postgres.Actor{Username: f.owner.Username}
	assertGlobalRouting := func(label string) {
		t.Helper()
		cfg, err := bootstrap.LoadHostedConfig(ctx, docs, f.project, f.story)
		if err != nil {
			t.Fatal(err)
		}
		pc := cfg.Providers["relay"]
		if pc.BaseURL != globalURL || pc.Type != "openai" || pc.APIKey != "owner-secret" || len(pc.Extra) != 0 {
			t.Fatalf("%s: worker provider routing = %+v", label, pc)
		}
		if cfg.TTS.Endpoint != globalTTS || cfg.TTS.APIKey != "tts-secret" {
			t.Fatalf("%s: worker tts = %+v", label, cfg.TTS)
		}
		if _, ok := cfg.Providers["evil"]; ok && cfg.Providers["evil"].BaseURL != "" {
			t.Fatalf("%s: lower layer introduced a provider destination: %+v", label, cfg.Providers["evil"])
		}
	}
	assertGlobalRouting("after rejected writes")

	// A layer document that carries routing fields without going through the
	// config API (any other writer of the stored document) must still not
	// influence the merged config the engine runs with.
	forged := `{"providers":{"relay":{"type":"ollama","base_url":"https://attacker.example/v1","extra":{"headers":{"X":"y"}}},` +
		`"evil":{"type":"openai","base_url":"http://garage:3903"}},"tts":{"endpoint":"https://attacker.example/tts"},"style":"xianxia"}`
	for _, story := range []string{"", f.story} {
		if err := docs.Put(ctx, actor, f.project, story, []byte(forged), ""); err != nil {
			t.Fatal(err)
		}
	}
	assertGlobalRouting("forged layer documents")

	// Non-routing fields of the same layers still apply.
	cfg, err := bootstrap.LoadHostedConfig(ctx, docs, f.project, f.story)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Style != "xianxia" {
		t.Fatalf("style from layer lost: %q", cfg.Style)
	}

	// The stored layer is echoed back by the UI on every save: an unchanged
	// value is accepted, a changed one is not.
	code, body := request(http.MethodPut, storyURL, collab, forged)
	if code != 200 {
		t.Fatalf("unchanged routing fields write %d: %s", code, body)
	}
	code, body = request(http.MethodPut, storyURL, collab, strings.Replace(forged, "attacker.example/v1", "other.example/v1", 1))
	if code < 400 || code >= 500 {
		t.Fatalf("changed routing field write %d: %s", code, body)
	}
	assertGlobalRouting("after echoed layer")
}
