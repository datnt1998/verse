package web

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/identity"
)

func TestRulesChangesCarryScopeAndSection(t *testing.T) {
	tg := newConfigTestGateway(t)
	g := tg.g
	sub, _, _ := g.events().subscribe(identity.User{Username: "admin", Role: identity.RoleAdmin}, 0, []string{"proj/01-story"})
	defer g.events().unsubscribe(sub)
	watcher := &fileWatcher{g: g, previous: map[string]string{}, stop: make(chan struct{})}
	watcher.scan(false)
	for _, tc := range []struct{ scope, project, story, dir string }{
		{"global", "", "", rulesHomeDir()},
		{"project", "proj", "", rulesDirIn(tg.project)},
		{"story", "proj", "01-story", rulesDirIn(tg.story)},
	} {
		t.Run(tc.scope, func(t *testing.T) {
			if err := os.MkdirAll(tc.dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(tc.dir, "live.md"), []byte("Quy tắc mới"), 0o644); err != nil {
				t.Fatal(err)
			}
			watcher.scan(true)
			seen := false
			for len(sub.ch) > 0 {
				ev := <-sub.ch
				if ev.Type != "config.changed" {
					continue
				}
				seen = true
				data := ev.Data.(map[string]any)
				if data["section"] != "rules" || data["scope"] != tc.scope || ev.Project != tc.project || ev.Story != tc.story {
					t.Fatalf("rules event missing discriminator/scope: %+v", ev)
				}
			}
			if !seen {
				t.Fatal("rules disk edit emitted no config.changed")
			}
		})
	}
}

func TestFileWatcherExternalEditAndPermissionFilter(t *testing.T) {
	tg := newConfigTestGateway(t)
	g := tg.g
	writer := identity.User{Username: "writer", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"proj": identity.ProjectCollab}}
	denied := identity.User{Username: "other", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"other": identity.ProjectView}}
	g.startWatcher()
	defer g.Close()
	allowed, _, _ := g.events().subscribe(writer, 0, []string{"proj/01-story"})
	defer g.events().unsubscribe(allowed)
	other, _, _ := g.events().subscribe(denied, 0, []string{"proj/01-story"})
	defer g.events().unsubscribe(other)
	chapters := mkdirAll(t, tg.story, "output", "novel", "chapters")
	if err := os.WriteFile(filepath.Join(chapters, "01.md"), []byte("Nội dung mới"), 0644); err != nil {
		t.Fatal(err)
	}
	g.watcher.scan(true)
	select {
	case event := <-allowed.ch:
		if event.Type != "story.changed" || event.Project != "proj" || event.Story != "01-story" {
			t.Fatalf("external event: %+v", event)
		}
		parts := event.Data.(map[string]any)["parts"].([]string)
		if !strings.Contains(strings.Join(parts, ","), "chapters") {
			t.Fatalf("parts: %v", parts)
		}
	case <-time.After(time.Second):
		t.Fatal("external chapter edit emitted no event")
	}
	select {
	case event := <-other.ch:
		t.Fatalf("unauthorized event: %+v", event)
	default:
	}
	g.watcher.scan(true)
	select {
	case event := <-allowed.ch:
		t.Fatalf("unchanged scan emitted duplicate: %+v", event)
	default:
	}
	configDir := mkdirAll(t, tg.story, ".ainovel")
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{"model":"external"}`), 0644); err != nil {
		t.Fatal(err)
	}
	g.watcher.scan(true)
	select {
	case event := <-allowed.ch:
		if event.Type != "config.changed" {
			t.Fatalf("config event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("external config edit emitted no event")
	}
	mkdirAll(t, configDir, "rules")
	if err := os.WriteFile(filepath.Join(configDir, "rules", "new.md"), []byte("Quy tắc"), 0644); err != nil {
		t.Fatal(err)
	}
	g.watcher.scan(true)
	select {
	case event := <-allowed.ch:
		if event.Type != "config.changed" || event.Data.(map[string]any)["section"] != "rules" {
			t.Fatalf("rules event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("rules edit emitted no invalidation")
	}
}

// TestFileWatcherNaoParts: sửa tay address_terms.json / relationship_log.jsonl /
// foreshadow_ledger.json từ ngoài app phải phát story.changed với part address/brain,
// nếu không màn Xưng hô và Não truyện sẽ hiển thị dữ liệu cũ.
func TestFileWatcherNaoParts(t *testing.T) {
	tg := newConfigTestGateway(t)
	g := tg.g
	g.startWatcher()
	defer g.Close()
	sub, _, _ := g.events().subscribe(identity.User{Username: "admin", Role: identity.RoleAdmin}, 0, []string{"proj/01-story"})
	defer g.events().unsubscribe(sub)
	engine := mkdirAll(t, tg.story, "output", "novel")

	cases := []struct {
		file  string
		body  string
		parts []string
	}{
		{"address_terms.json", `[]`, []string{"address"}},
		{"relationship_log.jsonl", "", []string{"address"}},
		{"relationship_state.json", `[]`, []string{"address"}},
		{"foreshadow_ledger.json", `[]`, []string{"brain"}},
		{"timeline.json", `[]`, []string{"brain"}},
		{"world_rules.json", `[]`, []string{"brain"}},
	}
	for _, tc := range cases {
		if err := os.WriteFile(filepath.Join(engine, tc.file), []byte(tc.body), 0o644); err != nil {
			t.Fatal(err)
		}
		g.watcher.scan(true)
		parts := naoWatchParts(sub)
		if len(parts) == 0 {
			t.Fatalf("%s edit emitted no story.changed", tc.file)
		}
		for _, want := range tc.parts {
			found := false
			for _, part := range parts {
				if part == want {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s edit parts = %v, want %q", tc.file, parts, want)
			}
		}
	}
}

// naoWatchParts gom part của các story.changed đã phát trong một nhịp quét.
func naoWatchParts(sub *EventSub) []string {
	var parts []string
	for {
		select {
		case ev := <-sub.ch:
			if ev.Type != "story.changed" {
				continue
			}
			if got, ok := ev.Data.(map[string]any)["parts"].([]string); ok {
				parts = append(parts, got...)
			}
		case <-time.After(200 * time.Millisecond):
			return parts
		}
	}
}

func TestEventsWatchUpdatesOnlyOwnSubscriptions(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("writer", "writer-pass")
	user := identity.User{Username: "writer", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"proj": identity.ProjectCollab}}
	sub, _, _ := tg.g.events().subscribe(user, 0, nil)
	defer tg.g.events().unsubscribe(sub)
	foreign, _, _ := tg.g.events().subscribe(identity.User{Username: "other", Role: identity.RoleAdmin}, 0, nil)
	defer tg.g.events().unsubscribe(foreign)
	w := tg.do("POST", "/api/events/watch", cookie, `{"keys":["proj/01-story","secret/nope"]}`)
	if w.Code != 200 {
		t.Fatalf("watch POST = %d %s", w.Code, w.Body.String())
	}
	if watched := tg.g.events().Watched(); !watched["proj/01-story"] || watched["secret/nope"] {
		t.Fatalf("watched keys: %v", watched)
	}
	w = tg.do("POST", "/api/events/watch", cookie, fmt.Sprintf(`{"keys":["proj"],"subscriber":%q}`, foreign.ID))
	if w.Code != 404 {
		t.Fatalf("foreign subscriber update = %d", w.Code)
	}
}

func TestFileWatcherSSEDetectsEditWithinThreeSeconds(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("writer", "writer-pass")
	server := httptest.NewServer(tg.g)
	defer server.Close()
	defer tg.g.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/events?watch=proj/01-story", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		if scanner.Text() == "" {
			break
		}
	}
	chapters := mkdirAll(t, tg.story, "output", "novel", "chapters")
	start := time.Now()
	if err := os.WriteFile(filepath.Join(chapters, "01.md"), []byte("Thay đổi từ trình soạn thảo"), 0644); err != nil {
		t.Fatal(err)
	}
	for scanner.Scan() {
		if scanner.Text() == "event: story.changed" {
			if elapsed := time.Since(start); elapsed > 3*time.Second {
				t.Fatalf("external edit took %s", elapsed)
			}
			return
		}
	}
	t.Fatalf("no story.changed SSE: %v", scanner.Err())
}

func TestFileWatcherIgnoresMetadataOnlyStoryEdits(t *testing.T) {
	tg := newConfigTestGateway(t)
	g := tg.g
	sub, _, _ := g.events().subscribe(identity.User{Username: "admin", Role: identity.RoleAdmin}, 0, []string{"proj/01-story"})
	defer g.events().unsubscribe(sub)
	chapter := filepath.Join(mkdirAll(t, tg.story, "output", "novel", "chapters"), "01.md")
	if err := os.WriteFile(chapter, []byte("Nội dung không đổi"), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher := &fileWatcher{g: g, previous: map[string]string{}, stop: make(chan struct{})}
	watcher.scan(false)
	later := time.Now().Add(time.Second)
	if err := os.Chtimes(chapter, later, later); err != nil {
		t.Fatal(err)
	}
	watcher.scan(true)
	for len(sub.ch) > 0 {
		event := <-sub.ch
		if event.Type == "story.changed" {
			t.Fatalf("unchanged chapter invalidated parts: %v", event.Data)
		}
	}
	if err := os.WriteFile(chapter, []byte("Nội dung thật sự đã đổi"), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher.scan(true)
	select {
	case event := <-sub.ch:
		if got := strings.Join(event.Data.(map[string]any)["parts"].([]string), ","); got != "chapters,snapshot" {
			t.Fatalf("changed chapter invalidated unrelated parts: %s", got)
		}
	default:
		t.Fatal("changed chapter did not invalidate its own data")
	}
}
