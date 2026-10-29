package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/platform/identity"
)

// newAuthGateway 建立只带账号路由的网关；helper 与 gateway_test.go 相互独立，避免共享命名。
func newAuthGateway(t *testing.T) (*Gateway, *identity.Store) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	accounts := identity.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"), library.IdentityOptions())
	for _, u := range []identity.NewUser{
		{Username: "admin", Role: identity.RoleAdmin, Password: "admin-pass"},
		{Username: "writer", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"p": identity.ProjectCollab}, Password: "writer-pass"},
	} {
		if _, err := accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	g := NewGateway(t.TempDir(), accounts, version.Info{})
	t.Cleanup(g.Close)
	return g, accounts
}

func authDo(t *testing.T, g *Gateway, method, path, cookie, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8686"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	}
	if method == "POST" || body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	return w
}

func authSession(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c.Value
		}
	}
	t.Fatalf("no session cookie (%d: %s)", w.Code, w.Body.String())
	return ""
}

func authLogin(t *testing.T, g *Gateway, username, password string) string {
	t.Helper()
	w := authDo(t, g, "POST", "/api/login", "", fmt.Sprintf(`{"username":%q,"password":%q}`, username, password))
	if w.Code != 200 {
		t.Fatalf("login %s: %d %s", username, w.Code, w.Body.String())
	}
	return authSession(t, w)
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return m
}

func TestMeEndpoints(t *testing.T) {
	g, _ := newAuthGateway(t)
	cookie := authLogin(t, g, "writer", "writer-pass")

	// POST /api/me 改显示名。
	w := authDo(t, g, "POST", "/api/me", cookie, `{"name":"  Tác Giả  "}`)
	if w.Code != 200 {
		t.Fatalf("rename: %d %s", w.Code, w.Body.String())
	}
	user := decodeBody(t, w)["user"].(map[string]any)
	if user["name"] != "Tác Giả" {
		t.Fatalf("name = %v", user["name"])
	}
	if got := authDo(t, g, "GET", "/api/me", cookie, ""); !strings.Contains(got.Body.String(), `"name":"Tác Giả"`) {
		t.Fatalf("name not persisted: %s", got.Body.String())
	}
	// 超长名字 → 400。
	if code := authDo(t, g, "POST", "/api/me", cookie, `{"name":"`+strings.Repeat("a", 61)+`"}`).Code; code != 400 {
		t.Fatalf("long name: %d", code)
	}

	// POST /api/me/onboarded 记录时间且幂等。
	w = authDo(t, g, "POST", "/api/me/onboarded", cookie, `{}`)
	if w.Code != 200 {
		t.Fatalf("onboarded: %d %s", w.Code, w.Body.String())
	}
	user = decodeBody(t, w)["user"].(map[string]any)
	at, ok := user["onboardedAt"].(string)
	if !ok || at == "" {
		t.Fatalf("onboardedAt = %v", user["onboardedAt"])
	}
	w = authDo(t, g, "POST", "/api/me/onboarded", cookie, `{}`)
	if got := decodeBody(t, w)["user"].(map[string]any)["onboardedAt"]; got != at {
		t.Fatalf("second onboarded call changed the timestamp: %v → %v", at, got)
	}
}

func TestAccountUserLastSeenShape(t *testing.T) {
	g, _ := newAuthGateway(t)
	login := authDo(t, g, http.MethodPost, "/api/login", "", `{"username":"admin","password":"admin-pass"}`)
	cookie := authSession(t, login)
	for _, tc := range []struct {
		name       string
		res        *httptest.ResponseRecorder
		wrapped    bool
		hasSession bool
	}{
		{"login", login, true, true},
		{"me", authDo(t, g, http.MethodGet, "/api/me", cookie, ""), true, true},
		{"update", authDo(t, g, http.MethodPost, "/api/me", cookie, `{"name":"Admin"}`), true, true},
		{"onboarded", authDo(t, g, http.MethodPost, "/api/me/onboarded", cookie, `{}`), true, true},
		{"create", authDo(t, g, http.MethodPost, "/api/users", cookie, `{"username":"reader","password":"reader-pass","role":"member"}`), false, false},
		{"edit", authDo(t, g, http.MethodPost, "/api/users/writer", cookie, `{"name":"Writer"}`), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.res.Code < 200 || tc.res.Code >= 300 {
				t.Fatalf("response: %d %s", tc.res.Code, tc.res.Body.String())
			}
			body := decodeBody(t, tc.res)
			if tc.wrapped {
				body = body["user"].(map[string]any)
			}
			seen, present := body["lastSeenAt"]
			if !present {
				t.Fatal("lastSeenAt must be present, including null for users without sessions")
			}
			if tc.hasSession {
				if value, ok := seen.(string); !ok || value == "" {
					t.Fatalf("expected latest session timestamp, got %v", seen)
				}
			} else if seen != nil {
				t.Fatalf("expected null without sessions, got %v", seen)
			}
		})
	}
}

func TestMustChangePasswordViaAPI(t *testing.T) {
	g, accounts := newAuthGateway(t)
	admin := authLogin(t, g, "admin", "admin-pass")

	// 管理员在网页端建号 → 新账号带 mustChangePassword。
	w := authDo(t, g, "POST", "/api/users", admin, `{"username":"newbie","password":"newbie-pass","role":"member"}`)
	if w.Code != 201 {
		t.Fatalf("create user: %d %s", w.Code, w.Body.String())
	}
	if got := decodeBody(t, w)["mustChangePassword"]; got != true {
		t.Fatalf("mustChangePassword = %v", got)
	}

	// 管理员为他人重置密码 → 重新置标记。
	w = authDo(t, g, "POST", "/api/users/writer", admin, `{"password":"reset-pass-1"}`)
	if w.Code != 200 || decodeBody(t, w)["mustChangePassword"] != true {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}

	// GET /api/users 带 lastSeenAt/sessionCount。
	cookie := authLogin(t, g, "newbie", "newbie-pass")
	w = authDo(t, g, "GET", "/api/users", admin, "")
	if w.Code != 200 {
		t.Fatalf("users: %d", w.Code)
	}
	users := decodeBody(t, w)["users"].([]any)
	var newbie map[string]any
	for _, u := range users {
		if u.(map[string]any)["username"] == "newbie" {
			newbie = u.(map[string]any)
		}
	}
	if newbie == nil {
		t.Fatal("newbie missing from /api/users")
	}
	if newbie["sessionCount"] != float64(1) || newbie["lastSeenAt"] == nil {
		t.Fatalf("session stats: %v", newbie)
	}
	if newbie["mustChangePassword"] != true {
		t.Fatalf("flag in list: %v", newbie["mustChangePassword"])
	}

	// 自己改密成功 → 标记清除；旧密码同时失效。
	w = authDo(t, g, "POST", "/api/me/password", cookie, `{"current":"newbie-pass","new":"final-pass-1"}`)
	if w.Code != 200 {
		t.Fatalf("password change: %d %s", w.Code, w.Body.String())
	}
	if code := authDo(t, g, "POST", "/api/login", "", `{"username":"newbie","password":"newbie-pass"}`).Code; code != 401 {
		t.Fatalf("old password still works: %d", code)
	}
	u, err := accounts.Login("newbie", "final-pass-1", "", "")
	if err != nil || u.User.MustChangePassword {
		t.Fatalf("flag must be cleared: %v %+v", err, u.User)
	}
	// 改密不吊销当前会话。
	if code := authDo(t, g, "GET", "/api/me", cookie, "").Code; code != 200 {
		t.Fatalf("current session revoked: %d", code)
	}
}
