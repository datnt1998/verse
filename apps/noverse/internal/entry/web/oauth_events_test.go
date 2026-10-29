package web

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/webauth"
)

func nextOAuthEvent(t *testing.T, sub *EventSub, flow, status string) Event {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-sub.ch:
			data, ok := event.Data.(map[string]any)
			if !ok || event.Type != "config.changed" || data["scope"] != "oauth" {
				continue
			}
			if data["flow"] != flow || data["status"] != status {
				t.Fatalf("OAuth event = %v, want flow=%s status=%s", data, flow, status)
			}
			return event
		case <-timer.C:
			t.Fatalf("missing OAuth transition event: flow=%s status=%s", flow, status)
			return Event{}
		}
	}
}

func TestOAuthTransitionEventsPrivateSuccessFailureExpiry(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("admin", "admin-pass")
	provider := registerFakeOAuth(t)
	owner, _, _ := tg.g.events().subscribe(webauth.User{Username: "admin", Role: webauth.RoleAdmin}, 0, nil)
	defer tg.g.events().unsubscribe(owner)
	other, _, _ := tg.g.events().subscribe(webauth.User{Username: "another-admin", Role: webauth.RoleAdmin}, 0, nil)
	defer tg.g.events().unsubscribe(other)
	start := func(device bool) string {
		w := tg.doWith("POST", "/api/config/oauth/start", cookie, fmt.Sprintf(`{"provider":%q,"device":%t}`, provider, device), func(r *http.Request) { r.RemoteAddr = "127.0.0.1:8888" })
		if w.Code != 200 {
			t.Fatalf("start OAuth = %d %s", w.Code, w.Body.String())
		}
		return decodeResp(t, w)["id"].(string)
	}
	device := start(true)
	nextOAuthEvent(t, owner, device, "pending")
	// A code/URL becoming available is a meaningful pending-state update.
	nextOAuthEvent(t, owner, device, "pending")
	nextOAuthEvent(t, owner, device, "done")
	browser := start(false)
	nextOAuthEvent(t, owner, browser, "pending")
	nextOAuthEvent(t, owner, browser, "pending")
	w := tg.do("POST", "/api/config/oauth/cancel", cookie, fmt.Sprintf(`{"id":%q}`, browser))
	if w.Code != 200 {
		t.Fatalf("cancel = %d %s", w.Code, w.Body.String())
	}
	nextOAuthEvent(t, owner, browser, "failed")
	expired := start(false)
	nextOAuthEvent(t, owner, expired, "pending")
	nextOAuthEvent(t, owner, expired, "pending")
	sess := getOAuthSession(expired)
	sess.mu.Lock()
	sess.expiresAt = time.Now().Add(-time.Second)
	sess.mu.Unlock()
	w = tg.do("GET", "/api/config/oauth/status?id="+expired, cookie, "")
	if w.Code != 200 || decodeResp(t, w)["state"] != "failed" {
		t.Fatalf("expired state = %d %s", w.Code, w.Body.String())
	}
	nextOAuthEvent(t, owner, expired, "failed")
	sess.cancel()
	for {
		select {
		case event := <-other.ch:
			if data, ok := event.Data.(map[string]any); ok && data["scope"] == "oauth" {
				t.Fatalf("OAuth flow leaked to another admin: %+v", event)
			}
		default:
			return
		}
	}
}
