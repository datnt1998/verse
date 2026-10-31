package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/testidentity"
	"github.com/datnt1998/verse/platform/events"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// pgGateway là test gateway chạy trên kho tài khoản chung thật (Postgres, role
// viderse_identity) — mọi grant/mật khẩu đi qua đúng mặt store sản xuất.
type pgGateway struct {
	t  *testing.T
	fx *testidentity.Fixture
	g  *Gateway
}

func newPGGateway(t *testing.T) *pgGateway {
	t.Helper()
	fx := testidentity.Provision(t)
	hub := events.New(func(any, events.Event) bool { return true }, events.PingEvery(time.Hour))
	t.Cleanup(hub.Close)
	set := &fakeSettings{}
	svc := status.New(set, PublishStatus(hub))
	return &pgGateway{t: t, fx: fx, g: NewGateway(fx.Accounts, hub, svc)}
}

func (p *pgGateway) do(method, path, cookie, body string) *httptest.ResponseRecorder {
	p.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8789"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	p.g.ServeHTTP(w, r)
	return w
}

// doLegacy gửi request với một tên cookie tuỳ ý (kiểm tên cũ).
func (p *pgGateway) doLegacy(method, path, name, value string) *httptest.ResponseRecorder {
	p.t.Helper()
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8789"
	r.AddCookie(&http.Cookie{Name: name, Value: value})
	w := httptest.NewRecorder()
	p.g.ServeHTTP(w, r)
	return w
}

func (p *pgGateway) login(username, password string) *httptest.ResponseRecorder {
	p.t.Helper()
	return p.do(http.MethodPost, "/api/login", "",
		`{"username":`+jsonString(username)+`,"password":`+jsonString(password)+`}`)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func sessionCookieOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == testCookie && c.Value != "" {
			return c.Value
		}
	}
	t.Fatalf("không có %s (status %d: %s)", testCookie, w.Code, w.Body.String())
	return ""
}

func expiredCookie(t *testing.T, w *httptest.ResponseRecorder, name string) bool {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

// TestConcurrentFirstAdminAcrossHTTPApps: hai tiến trình app (hai kho +
// hai webapp.Server riêng, setup token riêng) POST setup/admin đồng thời —
// store quyết một winner; loser 409, token cục bộ bị vô hiệu, không user/
// session/recovery mồ côi.
func TestConcurrentFirstAdminAcrossHTTPApps(t *testing.T) {
	fx := testidentity.Provision(t)
	other, closeOther := fx.OpenViderseStore(t)
	defer closeOther()

	mk := func(accounts *identity.Store) (*Gateway, func()) {
		hub := events.New(func(any, events.Event) bool { return true }, events.PingEvery(time.Hour))
		svc := status.New(&fakeSettings{}, PublishStatus(hub))
		return NewGateway(accounts, hub, svc), hub.Close
	}
	gA, closeA := mk(fx.Accounts)
	defer closeA()
	gB, closeB := mk(other)
	defer closeB()

	post := func(g *Gateway, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		r.RemoteAddr = "192.0.2.1:1234"
		r.Host = "127.0.0.1:8789"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		return w
	}
	token := func(g *Gateway, code string) string {
		w := post(g, "/api/setup/verify", `{"code":`+jsonString(code)+`}`)
		if w.Code != http.StatusOK {
			t.Fatalf("verify status %d: %s", w.Code, w.Body.String())
		}
		tok, _ := decode(t, w)["setupToken"].(string)
		if tok == "" {
			t.Fatal("thiếu setupToken")
		}
		return tok
	}
	codeA, codeB := gA.SetupCodeOnce(), gB.SetupCodeOnce()
	if codeA == "" || codeB == "" || codeA == codeB {
		t.Fatalf("mã thiết lập hai tiến trình: %q / %q", codeA, codeB)
	}
	tokenA, tokenB := token(gA, codeA), token(gB, codeB)

	body := func(tok string) string {
		return `{"setupToken":` + jsonString(tok) +
			`,"name":"Quản trị","username":"boss","password":"boss-pass-99"}`
	}
	type result struct {
		status int
		body   string
	}
	res := make(chan result, 2)
	go func() {
		w := post(gA, "/api/setup/admin", body(tokenA))
		res <- result{w.Code, w.Body.String()}
	}()
	go func() {
		w := post(gB, "/api/setup/admin", body(tokenB))
		res <- result{w.Code, w.Body.String()}
	}()
	first, second := <-res, <-res
	statuses := map[int]int{first.status: 1, second.status: 1}
	if statuses[http.StatusOK] != 1 || statuses[http.StatusConflict] != 1 {
		t.Fatalf("hai setup đồng thời: %d và %d, muốn một 200 một 409",
			first.status, second.status)
	}

	users, err := fx.Admin.Users()
	if err != nil || len(users) != 1 || users[0].Username != "boss" || users[0].Role != identity.RoleAdmin {
		t.Fatalf("users sau race: %+v err=%v", users, err)
	}
	if n := fx.SessionCount(t, "boss"); n != 1 {
		t.Fatalf("mồ côi session: %d", n)
	}
}

// TestSetupUnavailableIsNotEmpty: kho chết thì /api/setup trả 503 — không coi
// là trống, không phát mã, không cho verify.
func TestSetupUnavailableIsNotEmpty(t *testing.T) {
	p := newPGGateway(t)
	p.fx.Close() // giết kho ngay sau khi dựng server
	w := p.do(http.MethodGet, "/api/setup", "", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("setup status kho chết = %d, muốn 503", w.Code)
	}
	if decode(t, w)["needed"] == true {
		t.Fatalf("kho chết mà báo cần setup: %s", w.Body.String())
	}
	w = p.do(http.MethodPost, "/api/setup/verify", "", `{"code":"AAAA-BBBB"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("verify kho chết = %d, muốn 503", w.Code)
	}
}

// TestSSOLoginClearsLegacyCookies: login expire mọi tên cookie cũ và giá trị
// session thật gửi dưới tên cũ không bao giờ xác thực được.
func TestSSOLoginClearsLegacyCookies(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "linh", "linh-pass-9", false, identity.AppViderse)

	w := p.login("linh", "linh-pass-9")
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", w.Code, w.Body.String())
	}
	cookie := sessionCookieOf(t, w)
	for _, legacy := range []string{"ainovel_session", "viderse_session"} {
		if !expiredCookie(t, w, legacy) {
			t.Fatalf("login không expire %q", legacy)
		}
		// Giá trị session thật dưới tên cũ: không dual-read → 401.
		if w := p.doLegacy(http.MethodGet, "/api/me", legacy, cookie); w.Code != http.StatusUnauthorized {
			t.Fatalf("session dưới %q được chấp nhận (%d)", legacy, w.Code)
		}
	}
}

// TestSharedLogoutRevokesBeforeClear: revoke trước — lỗi kho thì trả 503 và
// giữ cookie, không báo giả đăng xuất; phiên đã chết thì 401 + clear.
func TestSharedLogoutRevokesBeforeClear(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "linh", "linh-pass-9", false, identity.AppViderse)
	cookie := sessionCookieOf(t, p.login("linh", "linh-pass-9"))

	// Kho chết: logout phải lỗi, cookie giữ nguyên, phiên vẫn còn khi kho sống lại.
	p.fx.Close()
	w := p.do(http.MethodPost, "/api/logout", cookie, `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("logout kho chết = %d, muốn 503", w.Code)
	}
	if expiredCookie(t, w, testCookie) {
		t.Fatal("logout lỗi mà xoá cookie")
	}
	if n := p.fx.SessionCount(t, "linh"); n != 1 {
		t.Fatalf("logout lỗi mà phiên bị thu hồi: còn %d", n)
	}

	// Kho sống lại (fixture tự dựng qua OpenViderseStore): phiên chưa bị
	// revoke vẫn vào được — bằng chứng không báo giả.
	store, closeStore := p.fx.OpenViderseStore(t)
	defer closeStore()
	hub := events.New(func(any, events.Event) bool { return true }, events.PingEvery(time.Hour))
	defer hub.Close()
	p.g = NewGateway(store, hub, status.New(&fakeSettings{}, PublishStatus(hub)))
	if w := p.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("me sau logout-lỗi = %d, muốn 200 (phiên chưa chết)", w.Code)
	}

	// Logout thật: revoke trước, cookie xoá sau, phiên chết khỏi DB.
	w = p.do(http.MethodPost, "/api/logout", cookie, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("logout status %d", w.Code)
	}
	if !expiredCookie(t, w, testCookie) {
		t.Fatal("logout không xoá cookie")
	}
	if n := p.fx.SessionCount(t, "linh"); n != 0 {
		t.Fatalf("phiên còn sống sau logout: %d", n)
	}
	if w := p.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("me sau logout = %d, muốn 401", w.Code)
	}
}

// TestViderse403IsNotSessionExpired: phiên hợp lệ thiếu grant → 403 giữ
// cookie + user.grants; grant mutation thu hồi phiên mục tiêu (401 ngay);
// login lại phản ánh trạng thái mới.
func TestViderse403IsNotSessionExpired(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "linh", "linh-pass-9", false) // không grant app nào

	w := p.login("linh", "linh-pass-9")
	if w.Code != http.StatusForbidden {
		t.Fatalf("login no-grant = %d, muốn 403", w.Code)
	}
	m := decode(t, w)
	if m["code"] != "app_access_denied" || m["app"] != "viderse" {
		t.Fatalf("body %v", m)
	}
	user, _ := m["user"].(map[string]any)
	if user == nil || user["username"] != "linh" || user["grants"] == nil {
		t.Fatalf("403 thiếu user view an toàn: %v", m)
	}
	cookie := sessionCookieOf(t, w)

	w = p.do(http.MethodGet, "/api/me", cookie, "")
	if w.Code != http.StatusForbidden || decode(t, w)["code"] != "app_access_denied" {
		t.Fatalf("me no-grant = %d %s", w.Code, w.Body.String())
	}
	if expiredCookie(t, w, testCookie) {
		t.Fatal("403 xoá cookie — giả hết phiên")
	}
	// SSE cũng bị gate, không mở luồng.
	if w := p.do(http.MethodGet, "/api/events", cookie, ""); w.Code != http.StatusForbidden {
		t.Fatalf("events no-grant = %d, muốn 403", w.Code)
	}
	// Auth-neutral: logout vẫn đi được (revoke phiên thật).
	if w := p.do(http.MethodPost, "/api/logout", cookie, `{}`); w.Code != http.StatusOK {
		t.Fatalf("logout no-grant = %d", w.Code)
	}

	// Cấp grant: phiên cũ bị revoke ngay (401 request kế tiếp — cookie cũ
	// không "sống lại"), login lại → 200.
	cookie = sessionCookieOf(t, p.login("linh", "linh-pass-9"))
	if err := p.fx.Admin.OperatorSetAppGrant("linh", identity.AppViderse, identity.RoleMember); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if w := p.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("me cookie cũ sau grant = %d, muốn 401", w.Code)
	}
	w = p.login("linh", "linh-pass-9")
	if w.Code != http.StatusOK {
		t.Fatalf("login sau grant = %d: %s", w.Code, w.Body.String())
	}
	cookie = sessionCookieOf(t, w)
	if w := p.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("me granted = %d", w.Code)
	}
	m = decode(t, w)
	grants, _ := m["user"].(map[string]any)["grants"].(map[string]any)
	if grants["viderse"] != "member" {
		t.Fatalf("me grants %v", grants)
	}

	// Thu hồi grant: phiên hiện tại chết ngay, login lại về 403 có giải thích.
	if err := p.fx.Admin.OperatorRevokeAppGrant("linh", identity.AppViderse); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if w := p.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("me sau revoke grant = %d, muốn 401", w.Code)
	}
	w = p.login("linh", "linh-pass-9")
	if w.Code != http.StatusForbidden || decode(t, w)["code"] != "app_access_denied" {
		t.Fatalf("login sau revoke = %d %s", w.Code, w.Body.String())
	}
}

// TestVidersePasswordChangeRevokesBothAppSessions: đổi mật khẩu bằng role
// viderse_identity thật — clear flag bằng UPDATE giới hạn cột, mọi phiên của
// tài khoản (cả "phiên app kia" cùng kho) bị thu hồi, cookie xoá, reauth.
func TestVidersePasswordChangeRevokesBothAppSessions(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "linh", "linh-pass-9", false, identity.AppViderse, identity.AppNoverse)

	// Hai phiên của cùng tài khoản = phiên "hai app" trên kho chung.
	viderseCookie := sessionCookieOf(t, p.login("linh", "linh-pass-9"))
	noverseCookie := sessionCookieOf(t, p.login("linh", "linh-pass-9"))
	if viderseCookie == noverseCookie {
		t.Fatal("hai login trả cùng session")
	}
	// Admin đặt mật khẩu tạm: must_change lên và MỌI phiên cũ chết ngay —
	// phiên sống sót kế tiếp chỉ có sau khi đăng nhập bằng mã tạm.
	p.fx.IssueTempPassword(t, "linh", "temp-pass-99")
	for name, cookie := range map[string]string{"viderse": viderseCookie, "noverse": noverseCookie} {
		if w := p.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("phiên %s sau admin reset = %d, muốn 401", name, w.Code)
		}
	}
	if !p.fx.MustChangeFlag(t, "linh") {
		t.Fatal("fixture không đặt must_change_password")
	}

	// Đăng nhập bằng mã tạm (một lần): route nghiệp vụ bị gate, me còn đọc được.
	restricted := sessionCookieOf(t, p.login("linh", "temp-pass-99"))
	if w := p.do(http.MethodGet, "/api/status", restricted, ""); w.Code != http.StatusForbidden ||
		decode(t, w)["code"] != "password_change_required" {
		t.Fatalf("business route trước đổi = %d %s", w.Code, w.Body.String())
	}

	w := p.do(http.MethodPost, "/api/me/password", restricted,
		`{"current":"temp-pass-99","new":"real-pass-77"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("change = %d: %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if m["ok"] != true || m["reauthenticate"] != true {
		t.Fatalf("change body %v", m)
	}
	if !expiredCookie(t, w, testCookie) {
		t.Fatal("đổi mật khẩu không xoá cookie")
	}
	// Cờ đã clear bằng UPDATE giới hạn cột của role viderse_identity (role
	// không có UPDATE global_role/status — không cấp thêm quyền để pass).
	if p.fx.MustChangeFlag(t, "linh") {
		t.Fatal("must_change_password vẫn còn sau đổi")
	}
	// Phiên restricted cũng chết — không "sống sót" qua đổi mật khẩu; mọi
	// phiên còn lại của tài khoản đã bị thu hồi.
	if w := p.do(http.MethodGet, "/api/me", restricted, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("phiên restricted sau đổi = %d, muốn 401", w.Code)
	}
	if n := p.fx.SessionCount(t, "linh"); n != 0 {
		t.Fatalf("còn %d phiên sống sau đổi mật khẩu", n)
	}
	// Mã tạm một lần: dùng lại → 401. Mật khẩu mới vào được, hết gate.
	if w := p.login("linh", "temp-pass-99"); w.Code != http.StatusUnauthorized {
		t.Fatalf("login mã tạm lần hai = %d", w.Code)
	}
	w = p.login("linh", "real-pass-77")
	if w.Code != http.StatusOK {
		t.Fatalf("relogin = %d: %s", w.Code, w.Body.String())
	}
	cookie := sessionCookieOf(t, w)
	if w := p.do(http.MethodGet, "/api/status", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("status sau đổi = %d", w.Code)
	}
	// Sai current → 403, không đụng phiên.
	if w := p.do(http.MethodPost, "/api/me/password", cookie,
		`{"current":"sai-sai-sai","new":"x-12345678"}`); w.Code != http.StatusForbidden {
		t.Fatalf("sai current = %d", w.Code)
	}
}

// TestViderseSessionClaimsAreRequestScoped: hai phiên hai user xen kẽ —
// handler thấy đúng user của từng request; thay đổi grant phía kho ảnh hưởng
// ngay request sau, không lẫn giữa hai phiên.
func TestViderseSessionClaimsAreRequestScoped(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "linh", "linh-pass-9", false, identity.AppViderse)
	p.fx.CreateMember(t, "hoa", "hoa-pass-99", false, identity.AppViderse)

	var seen []string
	p.g.Handle(http.MethodGet, "/api/probe", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		seen = append(seen, user.Username)
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	cookieL := sessionCookieOf(t, p.login("linh", "linh-pass-9"))
	cookieH := sessionCookieOf(t, p.login("hoa", "hoa-pass-99"))

	for i, tc := range []struct{ cookie, want string }{
		{cookieL, "linh"}, {cookieH, "hoa"}, {cookieL, "linh"}, {cookieH, "hoa"},
	} {
		w := p.do(http.MethodGet, "/api/probe", tc.cookie, "")
		if w.Code != http.StatusOK {
			t.Fatalf("probe %d = %d", i, w.Code)
		}
		if seen[len(seen)-1] != tc.want {
			t.Fatalf("probe %d thấy %q, muốn %q (claims lẫn phiên)", i, seen[len(seen)-1], tc.want)
		}
	}
}

// TestForcedPasswordGateEveryBusinessRoute: user bị bắt đổi mật khẩu (kể cả
// có grant) không chạy được handler nghiệp vụ nào — route đã đăng ký (status,
// SSE), probe exact/prefix, upload multipart, write path lạ; chỉ tập
// auth-neutral đi qua.
func TestForcedPasswordGateEveryBusinessRoute(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "linh", "temp-pass-99", true, identity.AppViderse)

	var hits int
	p.g.Handle(http.MethodGet, "/api/probe", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		hits++
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	p.g.Handle(http.MethodPost, "/api/probe/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		hits++
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	cookie := sessionCookieOf(t, p.login("linh", "temp-pass-99"))

	cases := []struct {
		method, path, body, ctype string
	}{
		{http.MethodGet, "/api/status", "", ""},
		{http.MethodGet, "/api/events", "", ""},                                   // SSE
		{http.MethodGet, "/api/probe", "", ""},                                    // exact
		{http.MethodPost, "/api/probe/x", `{}`, "application/json"},               // prefix
		{http.MethodPost, "/api/probe", `{}`, "application/json"},                 // method khác exact
		{http.MethodPost, "/api/overlays", "", "multipart/form-data; boundary=x"}, // upload
		{http.MethodPost, "/api/never-registered", `{}`, "application/json"},      // unknown write
		{http.MethodDelete, "/api/probe/x", "", ""},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.RemoteAddr = "192.0.2.1:1234"
		r.Host = "127.0.0.1:8789"
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
		if tc.ctype != "" {
			r.Header.Set("Content-Type", tc.ctype)
		}
		w := httptest.NewRecorder()
		p.g.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s = %d, muốn 403", tc.method, tc.path, w.Code)
		}
		m := decode(t, w)
		if m["code"] != "password_change_required" || m["mustChangePassword"] != true {
			t.Fatalf("%s %s body %v", tc.method, tc.path, m)
		}
	}
	if hits != 0 {
		t.Fatalf("handler chạy %d lần dù bị bắt đổi mật khẩu", hits)
	}

	// Auth-neutral vẫn vào: me 200 kèm flag; me/sessions OK; revoke phiên
	// người khác bị chặn (id lạ → lỗi, không 200).
	if w := p.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("me = %d", w.Code)
	}
	if w := p.do(http.MethodGet, "/api/me/sessions", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("sessions = %d", w.Code)
	}
	if w := p.do(http.MethodPost, "/api/sessions/00000000-0000-0000-0000-000000000000/revoke", cookie, `{}`); w.Code == http.StatusOK {
		t.Fatal("revoke phiên lạ thành công")
	}
}

// TestIdentityErrorsAndLogsRedactSecrets: response không chứa password, mã,
// token, DSN hay hash — kể cả trên đường lỗi (setup query fail → 503).
func TestIdentityErrorsAndLogsRedactSecrets(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "linh", "linh-pass-9", false, identity.AppViderse)
	secrets := []string{"linh-pass-9", "postgres://", "password=", "viderse_identity", "argon2"}
	assertClean := func(w *httptest.ResponseRecorder, label string) {
		t.Helper()
		body := w.Body.String()
		for _, s := range secrets {
			if strings.Contains(body, s) {
				t.Fatalf("%s lộ %q: %s", label, s, body)
			}
		}
	}

	// Sai mật khẩu → 401 text sentinel, không echo input.
	assertClean(p.login("linh", "sai-mat-khau"), "login sai")
	// Login đúng → 200, body không chứa password/hash.
	assertClean(p.login("linh", "linh-pass-9"), "login đúng")
	cookie := sessionCookieOf(t, p.login("linh", "linh-pass-9"))
	// Sai current → 403 text cố định.
	assertClean(p.do(http.MethodPost, "/api/me/password", cookie,
		`{"current":"sai-mat-khau","new":"new-pass-99"}`), "đổi mật khẩu sai current")
	// Verify mã linh tinh → body không echo mã.
	assertClean(p.do(http.MethodPost, "/api/setup/verify", "", `{"code":"ABCD-EFGH"}`), "setup verify")
	// Kho chết ngay trước query: 503 generic, không DSN.
	p.fx.Close()
	w := p.do(http.MethodGet, "/api/me", cookie, "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("me kho chết = %d, muốn 503", w.Code)
	}
	assertClean(w, "kho chết")
	if expiredCookie(t, w, testCookie) {
		t.Fatal("503 xoá cookie")
	}
}

// TestSSESessionRecheckHonoursGrant: recheck phiên của SSE đọc đúng cookie
// chung và kiểm grant + forced-password, không chỉ err==nil của Authenticate.
func TestSSESessionRecheckHonoursGrant(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "linh", "linh-pass-9", false, identity.AppViderse)
	cookie := sessionCookieOf(t, p.login("linh", "linh-pass-9"))

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	if !p.g.eventSessionValid(req) {
		t.Fatal("phiên granted phải hợp lệ")
	}
	// Cùng giá trị session nhưng tên cookie cũ → không đọc.
	legacy := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	legacy.AddCookie(&http.Cookie{Name: "viderse_session", Value: cookie})
	if p.g.eventSessionValid(legacy) {
		t.Fatal("SSE đọc cookie legacy")
	}
	// Mất grant → recheck false (dù Authenticate vẫn err==nil).
	if err := p.fx.Admin.OperatorRevokeAppGrant("linh", identity.AppViderse); err != nil {
		t.Fatal(err)
	}
	if p.g.eventSessionValid(req) {
		t.Fatal("SSE vẫn hợp lệ sau revoke grant")
	}
}
