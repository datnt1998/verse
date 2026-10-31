package web

import (
	"strings"
	"testing"
)

func TestGlobalRulesScopeRejectedOnBothBackends(t *testing.T) {
	for _, backend := range []string{"file", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := newDocsFixture(t, backend)
			for _, endpoint := range []struct{ method, path, body string }{
				{"GET", "/api/rules?scope=global", ""},
				{"GET", "/api/rules/file?scope=global&name=tone.md", ""},
				{"GET", "/api/rules/effective?scope=global", ""},
				{"PUT", "/api/rules/file", `{"scope":"global","name":"tone.md","text":"x"}`},
				{"POST", "/api/rules/file/delete", `{"scope":"global","name":"tone.md"}`},
				{"POST", "/api/rules/understand", `{"scope":"global","text":"x"}`},
			} {
				w := f.request(endpoint.method, endpoint.path, f.admin, endpoint.body, "")
				if w.Code != 400 || !strings.Contains(w.Body.String(), retiredGlobalRulesMessage) {
					t.Fatalf("%s %s: %d %s", endpoint.method, endpoint.path, w.Code, w.Body)
				}
			}
		})
	}
}

func TestGlobalConfigKeepsModelsButNotStyle(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, `{"provider":"shared","model":"shared-model","style":"fantasy"}`)
	admin := tg.login("admin", "admin-pass")
	w := tg.do("GET", "/api/config?scope=global", admin, "")
	if w.Code != 200 {
		t.Fatalf("global config: %d %s", w.Code, w.Body)
	}
	body := decodeResp(t, w)
	for _, key := range []string{"target", "effective"} {
		m := body[key].(map[string]any)
		if key == "target" {
			m = m["config"].(map[string]any)
		}
		if _, ok := m["style"]; ok {
			t.Fatalf("global style exposed in %s: %v", key, m)
		}
		if m["model"] != "shared-model" {
			t.Fatalf("global model removed: %v", m)
		}
	}
	w = tg.do("GET", "/api/config?scope=project&project=proj", admin, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), `"style":"fantasy"`) {
		t.Fatalf("project inherited global style: %d %s", w.Code, w.Body)
	}
	w = tg.do("PUT", "/api/config?scope=global", admin, `{"style":"fantasy"}`)
	if w.Code != 400 {
		t.Fatalf("global style write: %d %s", w.Code, w.Body)
	}
}
