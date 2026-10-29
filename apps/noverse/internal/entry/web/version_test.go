package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceVersionConfigConflictAndSuccess(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, configWithKey)
	cookie := tg.login("admin", "admin-pass")
	first := tg.do("GET", "/api/config", cookie, "")
	version, _ := decodeResp(t, first)["version"].(string)
	if version == "" {
		t.Fatal("GET missing resource version")
	}
	if first.Header().Get("ETag") != "\""+version+"\"" {
		t.Fatal("GET missing matching ETag")
	}
	tg.globalConfig(t, strings.ReplaceAll(configWithKey, "gpt-4o", "gpt-4.1"))
	stale := tg.doWith("PUT", "/api/config", cookie, `{"model":"stale"}`, func(r *http.Request) { r.Header.Set("If-Match", version) })
	if stale.Code != 409 {
		t.Fatalf("stale config PUT = %d, want 409: %s", stale.Code, stale.Body.String())
	}
	conflict := decodeResp(t, stale)
	if conflict["conflict"] != true || conflict["error"] != "Đã có thay đổi từ nơi khác" {
		t.Fatalf("conflict shape: %v", conflict)
	}
	if strings.Contains(stale.Body.String(), "sk-secret") {
		t.Fatal("conflict current leaks provider key")
	}
	current := conflict["current"].(map[string]any)
	if current["version"] != conflict["version"] {
		t.Fatal("current version mismatch")
	}
	fresh := conflict["version"].(string)
	ok := tg.doWith("PUT", "/api/config", cookie, `{"model":"updated"}`, func(r *http.Request) { r.Header.Set("If-Match", "\""+fresh+"\"") })
	if ok.Code != 200 {
		t.Fatalf("matching PUT = %d: %s", ok.Code, ok.Body.String())
	}
	if got := decodeResp(t, ok)["version"]; got == fresh || got == "" {
		t.Fatalf("write did not advance version: %v", got)
	}
	if got := tg.do("PUT", "/api/config", cookie, `{"model":"last-write-wins"}`).Code; got != 200 {
		t.Fatalf("unconditional PUT = %d", got)
	}
}

func TestResourceVersionRulesProjectStoryAndAccount(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("admin", "admin-pass")
	cases := []struct{ get, put, body string }{
		{"/api/projects/proj", "/api/projects/proj", `{"name":"New","notes":"N"}`},
		{"/api/projects/proj/stories/01-story/meta", "/api/projects/proj/stories/01-story/meta", `{"title":"New","status":"idea"}`},
		{"/api/me", "/api/me", `{"name":"New"}`},
	}
	for _, tc := range cases {
		t.Run(tc.get, func(t *testing.T) {
			get := tg.do("GET", tc.get, cookie, "")
			if get.Code != 200 {
				t.Fatalf("GET = %d %s", get.Code, get.Body.String())
			}
			v := decodeResp(t, get)["version"].(string)
			ok := tg.doWith("PUT", tc.put, cookie, tc.body, func(r *http.Request) { r.Header.Set("If-Match", v) })
			if ok.Code != 200 {
				t.Fatalf("PUT = %d %s", ok.Code, ok.Body.String())
			}
			stale := tg.doWith("PUT", tc.put, cookie, tc.body, func(r *http.Request) { r.Header.Set("If-Match", v) })
			if stale.Code != 409 {
				t.Fatalf("stale PUT = %d %s", stale.Code, stale.Body.String())
			}
		})
	}
	path := "/api/rules/file?scope=story&project=proj&story=01-story&name=test.md"
	body := `{"scope":"story","project":"proj","story":"01-story","name":"test.md","text":"first"}`
	if w := tg.do("PUT", "/api/rules/file", cookie, body); w.Code != 200 {
		t.Fatalf("rule PUT: %d %s", w.Code, w.Body.String())
	}
	version := decodeResp(t, tg.do("GET", path, cookie, ""))["version"].(string)
	if err := os.WriteFile(filepath.Join(tg.story, ".ainovel", "rules", "test.md"), []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	w := tg.do("PUT", "/api/rules/file", cookie, strings.TrimSuffix(body, "}")+fmt.Sprintf(`,"version":%q}`, version))
	if w.Code != 409 {
		t.Fatalf("stale body version = %d %s", w.Code, w.Body.String())
	}
	if current := decodeResp(t, w)["current"].(map[string]any); current["text"] != "external" {
		t.Fatalf("current rule: %v", current)
	}
}

func TestResourceVersionUsersItemMatchesUpdate(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("admin", "admin-pass")
	users := decodeResp(t, tg.do("GET", "/api/users", cookie, ""))["users"].([]any)
	var v string
	for _, item := range users {
		u := item.(map[string]any)
		if u["username"] == "writer" {
			v = u["version"].(string)
		}
	}
	w := tg.doWith("PUT", "/api/users/writer", cookie, `{"name":"Writer mới"}`, func(r *http.Request) { r.Header.Set("If-Match", v) })
	if w.Code != 200 {
		t.Fatalf("per-user version not accepted: %d %s", w.Code, w.Body.String())
	}
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result["version"] == v {
		t.Fatal("account version not advanced")
	}
}
