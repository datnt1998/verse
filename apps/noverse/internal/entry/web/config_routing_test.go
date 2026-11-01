package web

import (
	"context"
	"encoding/json"
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
	// Ordinary lower-layer tuning keeps working (style is still per story).
	if code, body := request(http.MethodPut, storyURL, collab, `{"style":"xianxia"}`); code != 200 {
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

// 02/11 (chủ dự án chốt): provider/model/mức suy luận dùng chung cho cả app.
// Truyện từng ghim provider riêng nên khi gói thuê bao hết hạn, đổi provider ở
// cài đặt chung vẫn không viết tiếp được. Lớp dự án/truyện không được đổi các
// khoá này; ghim cũ bị bỏ khỏi cấu hình hiệu lực và khỏi nội dung /api/config.
func TestConfigAppWideModelSelectionIsGlobalOnly(t *testing.T) {
	f := provisionHostedGateway(t, "app-wide-model", "story")
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
	collab := tg.login("hosted-viewer", "mat-khau-12345")
	request := func(method, path, cookie, body string) (int, string) {
		w := doFrom(t, f.g, method, path, cookie, body, "127.0.0.1:1234", nil)
		return w.Code, w.Body.String()
	}

	global := `{"provider":"relay","model":"m","reasoning_effort":"high",` +
		`"providers":{"relay":{"type":"openai","base_url":"https://models.example/v1","api_key":"owner-secret","models":[{"name":"m"}]}},` +
		`"roles":{"writer":{"provider":"relay","model":"m","reasoning_effort":"high"}}}`
	if code, body := request(http.MethodPut, "/api/config?scope=global", admin, global); code != 200 {
		t.Fatalf("global write %d: %s", code, body)
	}

	storyURL := "/api/config?scope=story&project=" + f.project + "&story=" + f.story
	rejected := []struct{ name, body string }{
		{"story provider", `{"provider":"xai-oauth"}`},
		{"story model", `{"model":"grok-4.7"}`},
		{"story reasoning", `{"reasoning_effort":"low"}`},
		{"story role model", `{"roles":{"writer":{"provider":"xai-oauth","model":"grok-4.7"}}}`},
		{"story fallback", `{"roles":{"writer":{"fallbacks":[{"provider":"relay","model":"other"}]}}}`},
		{"story model list", `{"providers":{"relay":{"models":[{"name":"m"},{"name":"other"}]}}}`},
	}
	for _, c := range rejected {
		if code, body := request(http.MethodPut, storyURL, collab, c.body); code < 400 || code >= 500 {
			t.Errorf("%s: status %d, want 4xx: %s", c.name, code, body)
		}
	}

	// A layer document written outside the config API (older client, another
	// writer) must not pin the engine either.
	forged := `{"provider":"xai-oauth","model":"grok-4.7","reasoning_effort":"low",` +
		`"providers":{"xai-oauth":{"models":[{"name":"grok-4.7"}]}},` +
		`"roles":{"architect":{"provider":"xai-oauth","model":"grok-4.7"}},"style":"xianxia"}`
	docs := f.repo.ConfigDocuments()
	actor := &postgres.Actor{Username: f.owner.Username}
	if err := docs.Put(ctx, actor, f.project, f.story, []byte(forged), ""); err != nil {
		t.Fatal(err)
	}
	cfg, err := bootstrap.LoadHostedConfig(ctx, docs, f.project, f.story)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "relay" || cfg.ModelName != "m" || cfg.ReasoningEffort != "high" {
		t.Fatalf("lớp truyện ghim provider/model: %q/%q/%q", cfg.Provider, cfg.ModelName, cfg.ReasoningEffort)
	}
	if role, ok := cfg.Roles["architect"]; ok {
		t.Fatalf("vai chỉ khai ở lớp truyện lọt vào cấu hình: %+v", role)
	}
	if _, ok := cfg.Providers["xai-oauth"]; ok {
		t.Fatalf("provider chỉ khai ở lớp truyện lọt vào cấu hình")
	}
	if cfg.Style != "xianxia" {
		t.Fatalf("văn phong vẫn theo cấp truyện: %q", cfg.Style)
	}

	// /api/config của lớp truyện không hiện lại ghim cũ (để lần Lưu sau dọn sạch).
	code, body := request(http.MethodGet, storyURL, collab, "")
	if code != 200 {
		t.Fatalf("story config %d: %s", code, body)
	}
	var resp struct {
		Target struct {
			Config map[string]any `json:"config"`
		} `json:"target"`
		Effective map[string]any    `json:"effective"`
		Sources   map[string]string `json:"sources"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"provider", "model", "reasoning_effort", "roles", "providers"} {
		if _, ok := resp.Target.Config[key]; ok {
			t.Errorf("target lớp truyện còn khoá %q: %v", key, resp.Target.Config[key])
		}
		if scope, ok := resp.Sources[key]; ok && scope != "global" {
			t.Errorf("nguồn của %q phải là global, được %q", key, scope)
		}
	}
	if resp.Effective["model"] != "m" {
		t.Errorf("effective model phải là của app: %v", resp.Effective["model"])
	}
}
