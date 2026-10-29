package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevealNonAdminPaths(t *testing.T) {
	tg := newConfigTestGateway(t)
	for _, role := range []string{"reader", "writer"} {
		cookie := tg.login(role, role+"-pass")
		for _, target := range []struct{ query, want string }{
			{"target=project&project=proj", "proj"},
			{"target=story&project=proj&story=01-story", "proj/kich_ban/01-story"},
			{"target=exports&project=proj&story=01-story", "proj/kich_ban/01-story/xuat-ban"},
			{"target=audio&project=proj&story=01-story", "proj/kich_ban/01-story/audio"},
			{"target=activity&project=proj&story=01-story", "proj/kich_ban/01-story/output/novel/meta"},
			{"target=rules&project=proj", "proj/.ainovel/rules"},
			{"target=rules&scope=story&project=proj&story=01-story", "proj/kich_ban/01-story/.ainovel/rules"},
		} {
			t.Run(role+"/"+target.query, func(t *testing.T) {
				w := doFrom(t, tg.g, http.MethodGet, "/api/reveal?"+target.query, cookie, "", "127.0.0.1:5555", nil)
				if w.Code != http.StatusOK {
					t.Fatalf("reveal status: %d %s", w.Code, w.Body.String())
				}
				body := decodeResp(t, w)
				if body["path"] != target.want || body["canReveal"] != false {
					t.Errorf("non-admin reveal must be root-relative without open permission: %v", body)
				}
			})
		}
	}
}

func TestRevealPostRequiresAdmin(t *testing.T) {
	tg := newConfigTestGateway(t)
	oldOpen := openFolder
	opened := false
	openFolder = func(string) error { opened = true; return nil }
	t.Cleanup(func() { openFolder = oldOpen })
	for _, role := range []string{"reader", "writer"} {
		cookie := tg.login(role, role+"-pass")
		w := doFrom(t, tg.g, http.MethodPost, "/api/reveal", cookie, `{"target":"exports","project":"proj","story":"01-story"}`, "127.0.0.1:5555", nil)
		if w.Code != http.StatusForbidden || opened {
			t.Errorf("%s opened server folder: status=%d opened=%v body=%s", role, w.Code, opened, w.Body.String())
		}
		if _, err := os.Stat(filepath.Join(tg.story, "xuat-ban")); !os.IsNotExist(err) {
			t.Errorf("denied reveal must not create server folders: %v", err)
		}
	}
}

func TestSystemNonAdminPathsAndReveal(t *testing.T) {
	tg := newConfigTestGateway(t)
	for _, role := range []string{"reader", "writer"} {
		cookie := tg.login(role, role+"-pass")
		w := doFrom(t, tg.g, http.MethodGet, "/api/system", cookie, "", "127.0.0.1:5555", nil)
		body := decodeResp(t, w)
		if w.Code != http.StatusOK || body["root"] != "." || body["canReveal"] != false {
			t.Errorf("%s system leaks host path or reveal permission: %d %v", role, w.Code, body)
		}
	}
}

func TestNonAdminPathEndpointAudit(t *testing.T) {
	tg := newConfigTestGateway(t)
	for _, role := range []string{"reader", "writer"} {
		cookie := tg.login(role, role+"-pass")
		for _, endpoint := range []string{
			"/api/library",
			"/api/projects/proj",
			"/api/config?scope=global",
			"/api/config?scope=project&project=proj",
			"/api/config?scope=story&project=proj&story=01-story",
			"/api/rules?scope=global",
			"/api/rules?scope=project&project=proj",
			"/api/rules?scope=story&project=proj&story=01-story",
			"/api/projects/proj/stories/01-story/exports",
		} {
			t.Run(role+endpoint, func(t *testing.T) {
				w := tg.do(http.MethodGet, endpoint, cookie, "")
				if w.Code != http.StatusOK {
					t.Fatalf("read status: %d %s", w.Code, w.Body.String())
				}
				body := decodeResp(t, w)
				text := bodyText(body)
				if strings.Contains(text, tg.root) || strings.Contains(text, tg.home) {
					t.Errorf("non-admin response leaks absolute server paths: %s", w.Body.String())
				}
			})
		}
	}
}

func TestNonAdminConfigPathsInsideHomeStayRootRelative(t *testing.T) {
	tg := newConfigTestGateway(t)
	t.Setenv("HOME", filepath.Dir(tg.root))
	reader := tg.login("reader", "reader-pass")
	config := decodeResp(t, tg.do(http.MethodGet, "/api/config?scope=story&project=proj&story=01-story", reader, ""))
	if target := config["target"].(map[string]any); target["path"] != "proj/kich_ban/01-story/.ainovel/config.json" {
		t.Errorf("config target leaks home-relative server path: %v", target["path"])
	}
	rules := decodeResp(t, tg.do(http.MethodGet, "/api/rules?scope=story&project=proj&story=01-story", reader, ""))
	want := []string{"~/.ainovel/rules", "proj/.ainovel/rules", "proj/kich_ban/01-story/.ainovel/rules"}
	for i, level := range rules["chain"].([]any) {
		if dir := level.(map[string]any)["dir"]; dir != want[i] {
			t.Errorf("rules level %d leaks server path: %v", i, dir)
		}
	}
}

func TestPickFolderRequiresAdmin(t *testing.T) {
	tg := newConfigTestGateway(t)
	oldPick := pickFolder
	picked := false
	pickFolder = func(string) (string, bool, error) { picked = true; return "/may-chu/bi-mat", false, nil }
	t.Cleanup(func() { pickFolder = oldPick })
	for _, role := range []string{"reader", "writer"} {
		w := doFrom(t, tg.g, http.MethodPost, "/api/system/pick-folder", tg.login(role, role+"-pass"), `{}`, "127.0.0.1:5555", nil)
		if w.Code != http.StatusForbidden || picked {
			t.Errorf("%s opened server folder picker: status=%d picked=%v body=%s", role, w.Code, picked, w.Body.String())
		}
	}
}
