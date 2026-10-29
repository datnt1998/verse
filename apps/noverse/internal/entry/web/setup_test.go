package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/version"
	"github.com/datnt1998/noverse/internal/webauth"
)

// 8 ký tự bỏ 0 O 1 I L, hiển thị XXXX-XXXX.
var setupCodeFormat = regexp.MustCompile(`^[A-HJKMNP-Z2-9]{4}-[A-HJKMNP-Z2-9]{4}$`)

// setupClock 是可控时钟：过期与锁定测试要推进时间。
type setupClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *setupClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *setupClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type setupHarness struct {
	t        *testing.T
	g        *Gateway
	accounts *webauth.Store
	state    *webauth.SetupState
	code     string
	clock    *setupClock
}

// newSetupHarness 建立"还没有任何账号"的网关，并换成可控时钟的设置状态。
func newSetupHarness(t *testing.T) *setupHarness {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	accounts := webauth.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	g := NewGateway(t.TempDir(), accounts, version.Info{})
	t.Cleanup(g.Close)
	clock := &setupClock{t: time.Now()}
	state, code, err := webauth.NewSetupState(clock.now)
	if err != nil {
		t.Fatal(err)
	}
	g.setup, g.setupCode = state, code
	return &setupHarness{t: t, g: g, accounts: accounts, state: state, code: code, clock: clock}
}

func (h *setupHarness) do(method, path, cookie, body string) *httptest.ResponseRecorder {
	h.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8686"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	}
	if method == "POST" || method == "PUT" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.g.ServeHTTP(w, r)
	return w
}

func (h *setupHarness) verify(code string) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do("POST", "/api/setup/verify", "", fmt.Sprintf(`{"code":%q}`, code))
}

func (h *setupHarness) admin(token, name, username, password string) *httptest.ResponseRecorder {
	h.t.Helper()
	body, err := json.Marshal(map[string]string{"setupToken": token, "name": name, "username": username, "password": password})
	if err != nil {
		h.t.Fatal(err)
	}
	return h.do("POST", "/api/setup/admin", "", string(body))
}

func (h *setupHarness) token() string {
	h.t.Helper()
	w := h.verify(h.code)
	if w.Code != http.StatusOK {
		h.t.Fatalf("verify mã đúng: %d %s", w.Code, w.Body.String())
	}
	token, _ := decodeJSONBody(h.t, w)["setupToken"].(string)
	if token == "" {
		h.t.Fatal("verify không trả setupToken")
	}
	return token
}

// assertNoCode 断言响应体不含设置码（明文只允许出现在启动输出里）。
func (h *setupHarness) assertNoCode(w *httptest.ResponseRecorder) {
	h.t.Helper()
	body := strings.ToUpper(w.Body.String())
	if strings.Contains(body, strings.ReplaceAll(h.code, "-", "")) || strings.Contains(body, h.code) {
		h.t.Fatalf("响应体泄露设置码: %s", w.Body.String())
	}
}

func TestSetupStatusWrongAttemptsAndLock(t *testing.T) {
	h := newSetupHarness(t)
	w := h.do("GET", "/api/setup", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/setup: %d %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["needed"] != true {
		t.Fatalf("chưa có tài khoản mà needed=%v", body["needed"])
	}
	if body["attemptsLeft"] != float64(webauth.SetupMaxAttempts) {
		t.Fatalf("attemptsLeft=%v, muốn %d", body["attemptsLeft"], webauth.SetupMaxAttempts)
	}
	if _, ok := body["lockedUntil"]; ok {
		t.Fatalf("chưa sai lần nào mà đã khoá: %v", body["lockedUntil"])
	}
	if _, ok := body["expiresAt"]; !ok {
		t.Fatal("thiếu expiresAt")
	}
	h.assertNoCode(w)

	// Mã sai: 400, mỗi lần giảm một lượt.
	if w := h.verify("AAAAAAAA"); w.Code != http.StatusBadRequest {
		t.Fatalf("mã sai: %d %s", w.Code, w.Body.String())
	} else if got := decodeJSONBody(t, w)["attemptsLeft"]; got != float64(4) {
		t.Fatalf("sai 1 lần: attemptsLeft=%v, muốn 4", got)
	}
	// Đúng sau một lần sai: thành công và đếm về 5.
	h.token()
	if body := decodeJSONBody(t, h.do("GET", "/api/setup", "", "")); body["attemptsLeft"] != float64(webauth.SetupMaxAttempts) {
		t.Fatalf("thành công rồi mà attemptsLeft=%v, muốn %d", body["attemptsLeft"], webauth.SetupMaxAttempts)
	}

	// Sai tới lần thứ 5 → 423 kèm lockedUntil (+10 phút).
	for i := range webauth.SetupMaxAttempts {
		w := h.verify("BBBBBBBB")
		if i < webauth.SetupMaxAttempts-1 {
			if w.Code != http.StatusBadRequest {
				t.Fatalf("lần sai %d: %d %s", i+1, w.Code, w.Body.String())
			}
			continue
		}
		if w.Code != http.StatusLocked {
			t.Fatalf("lần sai thứ %d: %d %s, muốn 423", i+1, w.Code, w.Body.String())
		}
		locked, _ := decodeJSONBody(t, w)["lockedUntil"].(string)
		until, err := time.Parse(time.RFC3339, locked)
		if err != nil {
			t.Fatalf("lockedUntil không phải RFC3339: %q", locked)
		}
		if d := until.Sub(h.clock.now()); d < webauth.SetupLockDuration-time.Minute || d > webauth.SetupLockDuration {
			t.Fatalf("lockedUntil cách hiện tại %v, muốn %v", d, webauth.SetupLockDuration)
		}
	}
	// Đang khoá: mã đúng cũng bị từ chối.
	if w := h.verify(h.code); w.Code != http.StatusLocked {
		t.Fatalf("đang khoá mà mã đúng trả %d %s", w.Code, w.Body.String())
	}
	body = decodeJSONBody(t, h.do("GET", "/api/setup", "", ""))
	if _, ok := body["lockedUntil"]; !ok {
		t.Fatal("đang khoá mà GET /api/setup không có lockedUntil")
	}
	if body["attemptsLeft"] != float64(0) {
		t.Fatalf("đang khoá mà attemptsLeft=%v", body["attemptsLeft"])
	}

	// Khoá hết hạn → đếm về 0 và mã đúng lại dùng được.
	h.clock.advance(webauth.SetupLockDuration + time.Second)
	if w := h.verify(h.code); w.Code != http.StatusOK {
		t.Fatalf("hết khoá mà mã đúng trả %d %s", w.Code, w.Body.String())
	}
}

func TestSetupCodeExpired(t *testing.T) {
	h := newSetupHarness(t)
	token := h.token()
	h.clock.advance(webauth.SetupCodeTTL + time.Minute)

	if w := h.verify(h.code); w.Code != http.StatusGone {
		t.Fatalf("mã hết hạn: %d %s, muốn 410", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, h.do("GET", "/api/setup", "", ""))
	if body["needed"] != true {
		t.Fatalf("hết hạn nhưng needed=%v", body["needed"])
	}
	expires, _ := body["expiresAt"].(string)
	at, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		t.Fatalf("expiresAt không phải RFC3339: %q", expires)
	}
	if !at.Before(h.clock.now()) {
		t.Fatalf("expiresAt %v chưa ở quá khứ so với %v", at, h.clock.now())
	}
	// setupToken cấp trước đó cũng hết hạn theo mã.
	if w := h.admin(token, "Chủ máy", "chu.may", "mat-khau-123"); w.Code != http.StatusGone {
		t.Fatalf("token hết hạn: %d %s, muốn 410", w.Code, w.Body.String())
	}
}

func TestSetupAdminCreatesFirstAdmin(t *testing.T) {
	h := newSetupHarness(t)
	token := h.token()
	w := h.admin(token, "Chủ máy", "chu.may", "mat-khau-123")
	if w.Code != http.StatusOK {
		t.Fatalf("tạo quản trị: %d %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	user, _ := body["user"].(map[string]any)
	if user["role"] != "admin" || user["username"] != "chu.may" {
		t.Fatalf("user trả về sai: %v", user)
	}
	code, _ := body["recoveryCode"].(string)
	if !recoveryCodeFormat.MatchString(code) {
		t.Fatalf("recoveryCode sai dạng: %q", code)
	}
	h.assertNoCode(w)

	// Cookie giống /api/login: dùng được ngay.
	cookie := sessionFrom(t, w)
	if me := h.do("GET", "/api/me", cookie, ""); me.Code != http.StatusOK {
		t.Fatalf("/api/me bằng cookie vừa cấp: %d %s", me.Code, me.Body.String())
	}

	// Đã có quản trị: GET needed=false, mọi POST 409 {at}.
	status := decodeJSONBody(t, h.do("GET", "/api/setup", "", ""))
	if status["needed"] != false {
		t.Fatalf("đã có quản trị mà needed=%v", status["needed"])
	}
	if status["attemptsLeft"] != float64(0) {
		t.Fatalf("đã có quản trị mà attemptsLeft=%v", status["attemptsLeft"])
	}
	if _, ok := status["expiresAt"]; ok {
		t.Fatalf("đã có quản trị mà vẫn trả expiresAt: %v", status["expiresAt"])
	}
	created, _, err := h.accounts.FirstUserCreatedAt()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []*httptest.ResponseRecorder{
		h.verify(h.code),
		h.admin(token, "Người khác", "nguoi.khac", "mat-khau-123"),
	} {
		if w.Code != http.StatusConflict {
			t.Fatalf("đã có quản trị mà POST trả %d %s", w.Code, w.Body.String())
		}
		at, _ := decodeJSONBody(t, w)["at"].(string)
		got, err := time.Parse(time.RFC3339, at)
		if err != nil {
			t.Fatalf("at không phải RFC3339: %q", at)
		}
		if !got.Equal(created) {
			t.Fatalf("at=%v, muốn %v (thời điểm tạo quản trị đầu)", got, created)
		}
	}

	// Token đã bị tiêu (chỉ tiêu khi tạo thành công).
	if err := h.state.CheckToken(token); !errors.Is(err, webauth.ErrSetupToken) {
		t.Fatalf("token sau khi tạo thành công: %v, muốn ErrSetupToken", err)
	}
}

func TestSetupAdminValidation(t *testing.T) {
	h := newSetupHarness(t)
	token := h.token()
	cases := []struct {
		name, username, password, field string
	}{
		{"Chủ máy", "chu.may", "ngan", "password"},
		{"Chủ máy", "Chủ Máy", "mat-khau-123", "username"},
		{strings.Repeat("a", webauth.MaxNameLen+1), "chu.may", "mat-khau-123", "name"},
	}
	for _, c := range cases {
		w := h.admin(token, c.name, c.username, c.password)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s, muốn 400", c.field, w.Code, w.Body.String())
		}
		if got := decodeJSONBody(t, w)["field"]; got != c.field {
			t.Fatalf("field=%v, muốn %q", got, c.field)
		}
	}
	// Dữ liệu sai không tiêu token.
	if w := h.admin(token, "Chủ máy", "chu.may", "mat-khau-123"); w.Code != http.StatusOK {
		t.Fatalf("sau các lần 400 vẫn phải tạo được: %d %s", w.Code, w.Body.String())
	}
}

func TestSetupConcurrentCreateOnlyOneWins(t *testing.T) {
	h := newSetupHarness(t)
	token := h.token()
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = h.admin(token, "Quản trị", fmt.Sprintf("admin%d", i), "mat-khau-123").Code
		}(i)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("kết quả đồng thời %v, muốn đúng một 200 và một 409", codes)
	}
	users, err := h.accounts.Users()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("có %d tài khoản sau khi hai người cùng tạo, muốn 1", len(users))
	}
}

func TestSetupVerifyAcceptsDashesAndLowercase(t *testing.T) {
	h := newSetupHarness(t)
	plain := strings.ReplaceAll(h.code, "-", "")
	for _, variant := range []string{
		strings.ToLower(plain),
		strings.ToLower(h.code),
		" " + h.code[:4] + " " + h.code[5:] + " ",
	} {
		w := h.verify(variant)
		if w.Code != http.StatusOK {
			t.Fatalf("verify %q: %d %s", variant, w.Code, w.Body.String())
		}
		h.assertNoCode(w)
	}
}

func TestNewGatewayIssuesSetupCodeOnlyWhenNoAccounts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	empty := webauth.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	g := NewGateway(t.TempDir(), empty, version.Info{})
	t.Cleanup(g.Close)
	code := g.SetupCodeOnce()
	if !setupCodeFormat.MatchString(code) {
		t.Fatalf("mã thiết lập %q không đúng dạng XXXX-XXXX (bỏ 0 O 1 I L)", code)
	}
	if again := g.SetupCodeOnce(); again != "" {
		t.Fatalf("mã chỉ được trả một lần, lần hai trả %q", again)
	}
	r := httptest.NewRequest("GET", "/api/setup", nil)
	r.Host = "127.0.0.1:8686"
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/setup: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(strings.ToUpper(w.Body.String()), strings.ReplaceAll(code, "-", "")) {
		t.Fatalf("GET /api/setup trả cả mã: %s", w.Body.String())
	}

	withUser := webauth.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	if _, err := withUser.CreateUser(webauth.NewUser{Username: "admin", Role: webauth.RoleAdmin, Password: "admin-pass"}); err != nil {
		t.Fatal(err)
	}
	g2 := NewGateway(t.TempDir(), withUser, version.Info{})
	t.Cleanup(g2.Close)
	if code := g2.SetupCodeOnce(); code != "" {
		t.Fatalf("đã có tài khoản mà vẫn sinh mã: %q", code)
	}
}

// A0 页脚 "· Noverse 0.9.3": version chỉ có khi needed=true; đã có quản trị thì không trả
// (không lộ phiên bản cho request chưa đăng nhập).
func TestSetupStatusVersionOnlyWhileNeeded(t *testing.T) {
	old := checkLatestRelease
	checkLatestRelease = func(context.Context, version.CheckOptions) (*version.CheckResult, error) {
		return nil, nil
	}
	defer func() { checkLatestRelease = old }()

	t.Setenv("HOME", t.TempDir())
	accounts := webauth.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	g := NewGateway(t.TempDir(), accounts, version.Info{Version: "v9.9.9", Commit: "abc1234", Date: "2026-10-25"})
	t.Cleanup(g.Close)
	_ = g.SetupCodeOnce()
	get := func() map[string]any {
		r := httptest.NewRequest("GET", "/api/setup", nil)
		r.Host = "127.0.0.1:8686"
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /api/setup: %d %s", w.Code, w.Body.String())
		}
		return decodeJSONBody(t, w)
	}
	if body := get(); body["version"] != "v9.9.9" {
		t.Fatalf("chưa có quản trị mà version=%v, muốn v9.9.9 (khớp /api/system)", body["version"])
	}
	if _, err := accounts.CreateUser(webauth.NewUser{Username: "admin", Role: webauth.RoleAdmin, Password: "admin-pass"}); err != nil {
		t.Fatal(err)
	}
	body := get()
	if body["needed"] != false {
		t.Fatalf("đã có quản trị mà needed=%v", body["needed"])
	}
	if v, ok := body["version"]; ok {
		t.Fatalf("đã có quản trị mà vẫn trả version=%v", v)
	}
}

// TestSetupMalformedCodeDoesNotCount pin câu 1: mã sai định dạng trả 400
// {format:true}, không trừ lượt và không bao giờ làm khoá trang thiết lập.
func TestSetupMalformedCodeDoesNotCount(t *testing.T) {
	h := newSetupHarness(t)
	for _, bad := range []string{"", "abc", "0000-0000", "AAAA-AAAAA", "IIII-IIII"} {
		w := h.verify(bad)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("mã sai định dạng %q = %d %s, muốn 400", bad, w.Code, w.Body.String())
		}
		body := decodeJSONBody(t, w)
		if body["format"] != true {
			t.Fatalf("mã sai định dạng %q thiếu format:true: %v", bad, body)
		}
		if _, ok := body["attemptsLeft"]; ok {
			t.Fatalf("mã sai định dạng %q không được trả attemptsLeft: %v", bad, body)
		}
	}
	// Vẫn còn đủ 5 lượt sau 5 lần sai định dạng.
	body := decodeJSONBody(t, h.do("GET", "/api/setup", "", ""))
	if body["attemptsLeft"] != float64(webauth.SetupMaxAttempts) {
		t.Fatalf("sai định dạng làm trừ lượt: attemptsLeft=%v", body["attemptsLeft"])
	}
	// Bốn lần sai đúng định dạng vẫn chưa khoá (lượt chỉ bị trừ bởi mã sai).
	for i := range webauth.SetupMaxAttempts - 1 {
		if w := h.verify("BBBBBBBB"); w.Code != http.StatusBadRequest {
			t.Fatalf("lần sai %d = %d %s", i+1, w.Code, w.Body.String())
		}
	}
	if w := h.verify(h.code); w.Code != http.StatusOK {
		t.Fatalf("mã đúng sau 4 lần sai = %d %s", w.Code, w.Body.String())
	}
}

// TestSetupExpiredBeatsLocked pin câu 1: hết hạn thắng khoá — GET giữ expiresAt và
// thêm expired:true; verify trả 410 trước 423 kể cả khi trang vẫn đang bị khoá.
func TestSetupExpiredBeatsLocked(t *testing.T) {
	h := newSetupHarness(t)
	// Khoá muộn trong đời mã (còn 5 phút) để lúc mã hết hạn khoá vẫn còn hiệu lực.
	h.clock.advance(webauth.SetupCodeTTL - 5*time.Minute)
	for range webauth.SetupMaxAttempts {
		h.verify("BBBBBBBB")
	}
	body := decodeJSONBody(t, h.do("GET", "/api/setup", "", ""))
	if _, ok := body["lockedUntil"]; !ok {
		t.Fatalf("chưa khoá sau %d lần sai: %v", webauth.SetupMaxAttempts, body)
	}
	if body["expired"] == true {
		t.Fatalf("mã chưa hết hạn mà expired:true: %v", body)
	}
	h.clock.advance(6 * time.Minute) // mã hết hạn, khoá (10 phút) vẫn còn

	body = decodeJSONBody(t, h.do("GET", "/api/setup", "", ""))
	if body["expired"] != true {
		t.Fatalf("hết hạn mà thiếu expired:true: %v", body)
	}
	if _, ok := body["expiresAt"]; !ok {
		t.Fatalf("hết hạn mà mất expiresAt: %v", body)
	}
	if _, ok := body["lockedUntil"]; !ok {
		t.Fatalf("khoá phải còn hiệu lực để phép so ưu tiên có nghĩa: %v", body)
	}
	if w := h.verify(h.code); w.Code != http.StatusGone {
		t.Fatalf("hết hạn + đang khoá = %d %s, muốn 410", w.Code, w.Body.String())
	}
	if w := h.verify("ABC"); w.Code != http.StatusGone {
		t.Fatalf("hết hạn + sai định dạng = %d %s, muốn 410", w.Code, w.Body.String())
	}
}
