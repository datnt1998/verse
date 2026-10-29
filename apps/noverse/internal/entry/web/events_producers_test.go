package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/webauth"
)

func TestRulesMutationCarriesScopeAndSection(t *testing.T) {
	tg := newConfigTestGateway(t)
	sub, _, _ := tg.g.events().subscribe(webauth.User{Username: "admin", Role: webauth.RoleAdmin}, 0, nil)
	defer tg.g.events().unsubscribe(sub)
	for _, scope := range []string{"global", "project", "story"} {
		t.Run(scope, func(t *testing.T) {
			p, s := "", ""
			if scope != "global" {
				p = "proj"
			}
			if scope == "story" {
				s = "01-story"
			}
			for _, path := range []string{"file", "file/delete", "mkdir"} {
				r := httptest.NewRequest(http.MethodPost, "/api/rules/"+path, nil)
				r = r.WithContext(context.WithValue(r.Context(), mutationScopeKey{}, mutationScope{scope: scope, project: p, story: s}))
				tg.g.publishMutation(r, webauth.User{}, webauth.Session{}, nil)
				ev := <-sub.ch
				data := ev.Data.(map[string]any)
				if ev.Type != "config.changed" || data["section"] != "rules" || data["scope"] != scope || ev.Project != p || ev.Story != s {
					t.Fatalf("rules mutation missing discriminator/scope: %+v", ev)
				}
				for len(sub.ch) > 0 {
					<-sub.ch
				}
			}
		})
	}
}

func TestGatewayMutationPublishesImmediately(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("admin", "admin-pass")
	sub, _, _ := tg.g.events().subscribe(webauth.User{Username: "admin", Role: webauth.RoleAdmin}, 0, nil)
	defer tg.g.events().unsubscribe(sub)
	w := tg.do("PUT", "/api/config?scope=story&project=proj&story=01-story", cookie, `{"model":"test"}`)
	if w.Code != 200 {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	select {
	case ev := <-sub.ch:
		if ev.Type != "config.changed" || ev.Project != "proj" || ev.Story != "01-story" {
			t.Fatalf("event: %+v", ev)
		}
	default:
		t.Fatal("successful gateway write emitted no immediate event")
	}
	w = tg.doWith("PUT", "/api/config?scope=story&project=proj&story=01-story", cookie, `{"model":"test"}`, func(r *http.Request) { r.Header.Set("If-Match", "stale") })
	if w.Code != 409 {
		t.Fatalf("stale write = %d", w.Code)
	}
	select {
	case ev := <-sub.ch:
		t.Fatalf("failed mutation emitted event: %+v", ev)
	default:
	}
}

func TestWorkerChangeMapping(t *testing.T) {
	g := &Gateway{}
	sub, _, _ := g.events().subscribe(webauth.User{Role: webauth.RoleAdmin}, 0, nil)
	defer g.events().unsubscribe(sub)
	for _, tc := range []struct{ summary, part string }{
		{"commit_chapter(chương 1)", "chapters"},
		{"save_review(chương 1·accept)", "reviews"},
		{"save_foundation[outline]", "outline"},
		{"revise_outline", "outline"},
	} {
		ev := host.Event{Category: "TOOL", Summary: tc.summary, FinishedAt: time.Now()}
		data, _ := json.Marshal(ev)
		g.publishWorkerChange("proj/story", "event", data)
		select {
		case event := <-sub.ch:
			parts := event.Data.(map[string]any)["parts"].([]string)
			if !strings.Contains(strings.Join(parts, ","), tc.part) {
				t.Fatalf("%s: %v", tc.summary, parts)
			}
		default:
			t.Fatalf("%s emitted no invalidation", tc.summary)
		}
	}
	g.publishWorkerChange("proj/story", "job.done", []byte(`{"kind":"export","status":"done"}`))
	select {
	case ev := <-sub.ch:
		parts := ev.Data.(map[string]any)["parts"].([]string)
		if !strings.Contains(strings.Join(parts, ","), "exports") {
			t.Fatalf("export parts: %v", parts)
		}
	default:
		t.Fatal("completed export emitted no invalidation")
	}
	g.publishWorkerChange("proj/story", "event", []byte(`{"category":"TOOL","summary":"commit_chapter(chương 1)","failed":true,"finishedAt":"2026-01-01T00:00:00Z"}`))
	select {
	case ev := <-sub.ch:
		t.Fatalf("failed tool invalidated: %+v", ev)
	default:
	}
}

func TestRevokingSessionPublishesOnlyRevokedSession(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("writer", "writer-pass")
	second := tg.login("writer", "writer-pass")
	user := webauth.User{Username: "writer", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"proj": webauth.ProjectCollab}}
	sub, _, _ := tg.g.events().subscribe(user, 0, nil)
	defer tg.g.events().unsubscribe(sub)
	_, session, err := tg.g.accounts.Authenticate(second)
	if err != nil {
		t.Fatal(err)
	}
	w := tg.do("POST", "/api/sessions/"+session.ID+"/revoke", cookie, `{}`)
	if w.Code != 200 {
		t.Fatalf("revoke = %d %s", w.Code, w.Body.String())
	}
	select {
	case ev := <-sub.ch:
		if ev.Type != "session.revoked" || ev.Data.(map[string]any)["session"] != session.ID {
			t.Fatalf("revoke event: %+v", ev)
		}
	default:
		t.Fatal("no session revoked event")
	}
	if _, _, err := tg.g.accounts.Authenticate(cookie); err != nil {
		t.Fatalf("current session revoked: %v", err)
	}
}
