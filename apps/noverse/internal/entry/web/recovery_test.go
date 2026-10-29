package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/webauth"
)

var recoveryCodeFormat = regexp.MustCompile(`^[A-Z0-9]{4}(-[A-Z0-9]{4}){3}$`)

const wrongRecoveryCode = "AAAA-BBBB-CCCC-DDDD"

func recoverBody(username, code, password string) string {
	return fmt.Sprintf(`{"username":%q,"code":%q,"newPassword":%q}`, username, code, password)
}

func issueRecovery(t *testing.T, tg *testGateway, cookie, path string) string {
	t.Helper()
	w := tg.do("POST", path, cookie, `{}`)
	if w.Code != 200 {
		t.Fatalf("POST %s -> %d %s", path, w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("recovery code response must not be cacheable: %v", w.Header())
	}
	code, _ := decodeJSONBody(t, w)["code"].(string)
	if !recoveryCodeFormat.MatchString(code) {
		t.Fatalf("code format: %q", code)
	}
	return code
}

func TestMeRecoveryAdminOnlyAndLifecycle(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	for _, role := range []string{"writer", "reader"} {
		cookie := tg.login(role, role+"-pass")
		for _, c := range []struct{ method, path string }{
			{"GET", "/api/me/recovery"}, {"POST", "/api/me/recovery"}, {"POST", "/api/me/recovery/rotate"},
		} {
			if code := tg.do(c.method, c.path, cookie, `{}`).Code; code != 403 {
				t.Fatalf("%s %s %s -> %d, want 403", role, c.method, c.path, code)
			}
		}
	}

	if m := decodeJSONBody(t, tg.do("GET", "/api/me/recovery", admin, "")); m["createdAt"] != nil || m["used"] != false {
		t.Fatalf("status before any code: %v", m)
	}
	first := issueRecovery(t, tg, admin, "/api/me/recovery")
	m := decodeJSONBody(t, tg.do("GET", "/api/me/recovery", admin, ""))
	if m["createdAt"] == nil || m["used"] != false {
		t.Fatalf("status after create: %v", m)
	}
	if _, leaked := m["code"]; leaked {
		t.Fatal("GET must never return the code")
	}

	// 已有未使用的码：POST 返回 409，提示改用 rotate；rotate 换新码并让旧码失效。
	w := tg.do("POST", "/api/me/recovery", admin, `{}`)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "tạo mã mới") {
		t.Fatalf("second create -> %d %s", w.Code, w.Body.String())
	}
	second := issueRecovery(t, tg, admin, "/api/me/recovery/rotate")
	if second == first {
		t.Fatal("rotate returned the same code")
	}
	if code := tg.do("POST", "/api/recover", "", recoverBody("admin", first, "new-password-1")).Code; code != 401 {
		t.Fatalf("rotated-away code -> %d, want 401", code)
	}
	if code := tg.do("POST", "/api/recover", "", recoverBody("admin", second, "new-password-1")).Code; code != 200 {
		t.Fatalf("rotated code -> %d, want 200", code)
	}
}

func TestRecoverEndpoint(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	code := issueRecovery(t, tg, admin, "/api/me/recovery")

	// 非管理员账号和不存在的账号都返回同样的 401，不泄露账号是否存在。
	for _, user := range []string{"writer", "reader", "ghost"} {
		w := tg.do("POST", "/api/recover", "", recoverBody(user, code, "new-password-1"))
		if w.Code != 401 || decodeJSONBody(t, w)["error"] != "Mã khôi phục không đúng hoặc đã dùng" {
			t.Fatalf("target %s -> %d %s", user, w.Code, w.Body.String())
		}
	}
	// 码错：401；密码太短：400，且不烧码。
	if w := tg.do("POST", "/api/recover", "", recoverBody("admin", wrongRecoveryCode, "new-password-1")); w.Code != 401 {
		t.Fatalf("wrong code -> %d %s", w.Code, w.Body.String())
	}
	w := tg.do("POST", "/api/recover", "", recoverBody("admin", code, "short"))
	if w.Code != 400 || decodeJSONBody(t, w)["error"] != "Mật khẩu cần ít nhất 8 ký tự" {
		t.Fatalf("short password -> %d %s", w.Code, w.Body.String())
	}

	// 右码：设密码、吊销旧会话、发新 cookie（与 /api/login 同样的标志）。
	w = tg.do("POST", "/api/recover", "", recoverBody("admin", strings.ToLower(code), "new-password-1"))
	if w.Code != 200 {
		t.Fatalf("recover -> %d %s", w.Code, w.Body.String())
	}
	fresh := sessionFrom(t, w)
	if fresh == admin {
		t.Fatal("recover reissued the old session")
	}
	if tg.do("GET", "/api/me", admin, "").Code != 401 {
		t.Fatal("old session survived recovery")
	}
	if tg.do("GET", "/api/me", fresh, "").Code != 200 {
		t.Fatal("new session is not valid")
	}
	if tg.do("POST", "/api/login", "", `{"username":"admin","password":"admin-pass"}`).Code != 401 {
		t.Fatal("old password still works")
	}
	tg.login("admin", "new-password-1")

	// 码一次性：再用必须 401；状态变为 used。
	if w := tg.do("POST", "/api/recover", "", recoverBody("admin", code, "other-password-1")); w.Code != 401 {
		t.Fatalf("used code -> %d %s", w.Code, w.Body.String())
	}
	if m := decodeJSONBody(t, tg.do("GET", "/api/me/recovery", fresh, "")); m["used"] != true {
		t.Fatalf("status after use: %v", m)
	}
}

func TestRecoverRateLimitPerIP(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	code := issueRecovery(t, tg, admin, "/api/me/recovery")

	for i := 1; i <= 5; i++ {
		if w := tg.do("POST", "/api/recover", "", recoverBody("admin", wrongRecoveryCode, "new-password-1")); w.Code != 401 {
			t.Fatalf("attempt %d -> %d", i, w.Code)
		}
	}
	// 第 6 次：即使码是对的也 429（给出 Retry-After 与中文提示），且没有消耗这个码。
	w := tg.do("POST", "/api/recover", "", recoverBody("admin", code, "new-password-1"))
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "vui lòng đợi") {
		t.Fatalf("6th attempt -> %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	// 限制是按 IP：另一个 IP 不受影响，并且码仍可用。
	other := doFrom(t, tg.g, "POST", "/api/recover", "", recoverBody("admin", code, "new-password-1"), "198.51.100.7:4000", nil)
	if other.Code != 200 {
		t.Fatalf("other IP -> %d %s", other.Code, other.Body.String())
	}
}

func TestRecoverShortPasswordDoesNotCountTowardLimit(t *testing.T) {
	tg := newTestGateway(t)
	for range 8 {
		if w := tg.do("POST", "/api/recover", "", recoverBody("admin", wrongRecoveryCode, "short")); w.Code != 400 {
			t.Fatalf("short password -> %d", w.Code)
		}
	}
	if w := tg.do("POST", "/api/recover", "", recoverBody("admin", wrongRecoveryCode, "new-password-1")); w.Code != 401 {
		t.Fatalf("limiter charged for validation errors: %d", w.Code)
	}
}

func TestRecoverUsesLoginRequestRules(t *testing.T) {
	tg := newTestGateway(t)
	// Giống /api/login: POST phải là application/json (chặn form CSRF), nếu không 400.
	r := httptest.NewRequest("POST", "/api/recover", strings.NewReader(recoverBody("admin", wrongRecoveryCode, "new-password-1")))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("non-JSON recover -> %d", w.Code)
	}
	// Không nhận GET.
	if w := tg.do("GET", "/api/recover", "", ""); w.Code != 401 {
		t.Fatalf("GET /api/recover -> %d, want the normal unauthenticated 401", w.Code)
	}
}

func TestRecoverPublishesSessionRevokedForEverySession(t *testing.T) {
	tg := newTestGateway(t)
	first := tg.login("admin", "admin-pass")
	second := tg.login("admin", "admin-pass")
	code := issueRecovery(t, tg, first, "/api/me/recovery")
	want := map[string]bool{}
	for _, cookie := range []string{first, second} {
		_, s, err := tg.g.accounts.Authenticate(cookie)
		if err != nil {
			t.Fatal(err)
		}
		want[s.ID] = true
	}
	sub, _, _ := tg.g.events().subscribe(webauth.User{Username: "admin", Role: webauth.RoleAdmin}, 0, nil)
	defer tg.g.events().unsubscribe(sub)

	if w := tg.do("POST", "/api/recover", "", recoverBody("admin", code, "new-password-1")); w.Code != 200 {
		t.Fatalf("recover -> %d %s", w.Code, w.Body.String())
	}
	for len(want) > 0 {
		select {
		case ev := <-sub.ch:
			if ev.Type == "session.revoked" {
				delete(want, ev.Data.(map[string]any)["session"].(string))
			}
		default:
			t.Fatalf("no session.revoked event for sessions %v", want)
		}
	}
}

func TestMeRecoveryRejectsUnknownBodyFields(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	if w := tg.do("POST", "/api/me/recovery", admin, `{"code":"AAAA-AAAA-AAAA-AAAA"}`); w.Code != 400 {
		t.Fatalf("client-supplied code accepted: %d %s", w.Code, w.Body.String())
	}
	if m := decodeJSONBody(t, tg.do("GET", "/api/me/recovery", admin, "")); m["createdAt"] != nil {
		t.Fatalf("a rejected request created a code: %v", m)
	}
}
