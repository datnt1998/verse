package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/notify"
)

func TestNotificationSecurityOnNewLoginAndReadAll(t *testing.T) {
	g, _ := newAuthGateway(t)
	defer g.Close()
	cookie := authLogin(t, g, "writer", "writer-pass")
	w := authDo(t, g, "GET", "/api/notifications", cookie, "")
	if w.Code != 200 {
		t.Fatalf("inbox: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Notifications []notify.PushNotification `json:"notifications"`
		Unread        int                       `json:"unread"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		t.Fatal(e)
	}
	if len(body.Notifications) != 1 || body.Notifications[0].Type != "security" || body.Notifications[0].Title != "Đăng nhập trên thiết bị mới" || body.Unread != 1 {
		t.Fatalf("new login missing security event: %s", w.Body.String())
	}
	id := body.Notifications[0].ID
	w = authDo(t, g, "POST", "/api/notifications/read", cookie, `{"all":true}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"unread":0`) {
		t.Fatalf("read all: %d %s", w.Code, w.Body.String())
	}
	w = authDo(t, g, "POST", "/api/notifications/"+id+"/action", cookie, `{"action":"revoke"}`)
	if w.Code != 200 {
		t.Fatalf("security revoke: %d %s", w.Code, w.Body.String())
	}
	w = authDo(t, g, "GET", "/api/me", cookie, "")
	if w.Code != 401 {
		t.Fatalf("security action did not revoke target session: %d", w.Code)
	}
}
func TestNotificationPrefsConflictAndSessionCleanup(t *testing.T) {
	g, _ := newAuthGateway(t)
	defer g.Close()
	cookie := authLogin(t, g, "writer", "writer-pass")
	w := authDo(t, g, "GET", "/api/notify/prefs", cookie, "")
	var p notify.Prefs
	if e := json.Unmarshal(w.Body.Bytes(), &p); e != nil {
		t.Fatal(e)
	}
	p.Enabled = false
	b, _ := json.Marshal(p)
	put := func(version string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", "/api/notify/prefs", strings.NewReader(string(b)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("If-Match", version)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		return w
	}
	if w = put(p.Version); w.Code != 200 {
		t.Fatalf("prefs put: %d %s", w.Code, w.Body.String())
	}
	if w = put(p.Version); w.Code != 409 {
		t.Fatalf("stale prefs accepted: %d %s", w.Code, w.Body.String())
	}
	w = authDo(t, g, "POST", "/api/push/subscriptions", cookie, `{"endpoint":"https://push.example/test","keys":{"p256dh":"test","auth":"test"},"device":"Điện thoại"}`)
	if w.Code != 200 {
		t.Fatalf("subscribe: %d %s", w.Code, w.Body.String())
	}
	w = authDo(t, g, "POST", "/api/logout", cookie, `{}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if len(g.notifications.store.Subscriptions("writer")) != 0 {
		t.Fatal("logout left push subscription registered")
	}
}
