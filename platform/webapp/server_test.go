package webapp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
	"github.com/datnt1998/verse/platform/webguard"
)

const testCookie = "verse_session"

func testUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html":           &fstest.MapFile{Data: []byte("<html>spa</html>")},
		"assets/app-abc123.js": &fstest.MapFile{Data: []byte("console.log(1)")},
		"docs/readme.txt":      &fstest.MapFile{Data: []byte("hello")},
	}
}

type testServer struct {
	t        *testing.T
	s        *webapp.Server
	accounts *identity.Store
}

// newTestServer dựng Server trên một kho file-backend trống; withUser tạo
// sẵn tài khoản admin để bỏ qua luồng thiết lập (admin file-mode có grant
// hiệu lực cả hai app, xem identity.fileGrants).
func newTestServer(t *testing.T, withUser bool) *testServer {
	t.Helper()
	accounts := identity.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"), identity.Options{})
	ts := &testServer{t: t, accounts: accounts}
	if withUser {
		ts.createUser("admin", "admin-pass")
	}
	ts.s = webapp.New(accounts, webapp.Options{
		AppName: "TestApp",
		App:     identity.AppViderse,
		UI:      testUI(),
		// Mirror allowlist upload của gateway thật: multipart chỉ đi qua ở
		// đúng hai route nhận tệp, các gate auth vẫn chạy sau đó.
		AllowNonJSON: func(r *http.Request) bool {
			return r.Method == http.MethodPost &&
				(r.URL.Path == "/api/overlays" || r.URL.Path == "/api/image-prompts/import")
		},
	})
	return ts
}

func (ts *testServer) createUser(username, password string) {
	ts.t.Helper()
	if _, err := ts.accounts.CreateUser(identity.NewUser{
		Username: username, Role: identity.RoleAdmin, Password: password,
	}); err != nil {
		ts.t.Fatal(err)
	}
}

func (ts *testServer) do(method, path, cookie, body string) *httptest.ResponseRecorder {
	ts.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8799"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	}
	if method == http.MethodPost || method == http.MethodPut || body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	ts.s.ServeHTTP(w, r)
	return w
}

func (ts *testServer) sessionCookie() string {
	ts.t.Helper()
	w := ts.do(http.MethodPost, "/api/login", "", `{"username":"admin","password":"admin-pass"}`)
	if w.Code != http.StatusOK {
		ts.t.Fatalf("login status %d: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == testCookie && c.Value != "" {
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
				ts.t.Fatalf("session cookie flags: %+v", c)
			}
			return c.Value
		}
	}
	ts.t.Fatalf("no session cookie (status %d: %s)", w.Code, w.Body.String())
	return ""
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body không phải JSON: %q", w.Body.String())
	}
	return m
}

// ── probes / ngoài api ──────────────────────────────────────────────────────

func TestHealthzReadyz(t *testing.T) {
	ts := newTestServer(t, true)
	for _, path := range []string{"/healthz", "/readyz"} {
		w := ts.do(http.MethodGet, path, "", "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s status %d", path, w.Code)
		}
		if m := decode(t, w); m["ok"] != true {
			t.Fatalf("%s body %v", path, m)
		}
	}
}

func TestStaticSPA(t *testing.T) {
	ts := newTestServer(t, true)

	w := ts.do(http.MethodGet, "/", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<html>spa</html>") {
		t.Fatalf("GET / status %d body %q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("index cache header %q", got)
	}
	if typ := w.Header().Get("Content-Type"); !strings.Contains(typ, "text/html") {
		t.Fatalf("index content type %q", typ)
	}

	// Fallback SPA: đường client-side chưa biết vẫn về index.html.
	w = ts.do(http.MethodGet, "/app/duong/sau", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<html>spa</html>") {
		t.Fatalf("fallback status %d body %q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("fallback cache header %q", got)
	}

	// Asset băm phải ra đúng tệp với cache immutable.
	w = ts.do(http.MethodGet, "/assets/app-abc123.js", "", "")
	if w.Code != http.StatusOK || w.Body.String() != "console.log(1)" {
		t.Fatalf("asset status %d body %q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("asset cache header %q", got)
	}

	// Asset không tồn tại là 404 thật, không fallback.
	w = ts.do(http.MethodGet, "/assets/khong-co.js", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing asset status %d", w.Code)
	}

	// Tệp tĩnh khác phục vụ bình thường, cache no-cache.
	w = ts.do(http.MethodGet, "/docs/readme.txt", "", "")
	if w.Code != http.StatusOK || w.Body.String() != "hello" {
		t.Fatalf("docs status %d body %q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("docs cache header %q", got)
	}

	// HEAD chỉ trả header.
	w = ts.do(http.MethodHead, "/", "", "")
	if w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatalf("HEAD status %d body %q", w.Code, w.Body.String())
	}

	// POST ngoài /api là 404.
	w = ts.do(http.MethodPost, "/", "", `{}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("POST / status %d", w.Code)
	}
}

func TestAPIPathMatrix(t *testing.T) {
	ts := newTestServer(t, true)

	w := ts.do(http.MethodGet, "/api", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /api status %d", w.Code)
	}
	if m := decode(t, w); m["error"] != "không tìm thấy" {
		t.Fatalf("GET /api body %v", m)
	}

	w = ts.do(http.MethodGet, "/api/khong-co", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown api unauthenticated status %d", w.Code)
	}
	cookie := ts.sessionCookie()
	w = ts.do(http.MethodGet, "/api/khong-co", cookie, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown api status %d", w.Code)
	}
	if m := decode(t, w); m["error"] != "không tìm thấy" {
		t.Fatalf("unknown api body %v", m)
	}
}

func TestJSONContentTypeRequired(t *testing.T) {
	ts := newTestServer(t, true)
	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{}`))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8799"
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	ts.s.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
	if m := decode(t, w); m["error"] != "Content-Type phải là application/json" {
		t.Fatalf("body %v", m)
	}
}

func TestCrossSiteRequestRejected(t *testing.T) {
	ts := newTestServer(t, true)
	ts.s.SetProxyConfig(&webguard.Config{})

	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"admin-pass"}`))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8799"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	ts.s.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status %d", w.Code)
	}
	if m := decode(t, w); m["error"] != "yêu cầu khác origin bị từ chối" {
		t.Fatalf("body %v", m)
	}

	// Origin hợp lệ (cùng host:port, IP literal) đi qua.
	r2 := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"admin-pass"}`))
	r2.RemoteAddr = "192.0.2.1:1234"
	r2.Host = "127.0.0.1:8799"
	r2.Header.Set("Content-Type", "application/json")
	r2.Header.Set("Origin", "http://127.0.0.1:8799")
	w2 := httptest.NewRecorder()
	ts.s.ServeHTTP(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("same-origin status %d body %s", w2.Code, w2.Body.String())
	}
}

// ── login / logout / me ─────────────────────────────────────────────────────

func TestLoginLogoutMe(t *testing.T) {
	ts := newTestServer(t, true)

	// Sai mật khẩu: 401 với text sentinel.
	w := ts.do(http.MethodPost, "/api/login", "", `{"username":"admin","password":"sai"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status %d", w.Code)
	}

	w = ts.do(http.MethodPost, "/api/login", "", `{"username":"  ADMIN  ","password":"admin-pass"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	user, ok := m["user"].(map[string]any)
	if !ok || user["username"] != "admin" {
		t.Fatalf("login user %v", m["user"])
	}
	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == testCookie {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("login không trả session cookie")
	}

	// GET /api/me trả view của user hiện tại.
	w = ts.do(http.MethodGet, "/api/me", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("me status %d", w.Code)
	}
	if m := decode(t, w); m["user"].(map[string]any)["username"] != "admin" {
		t.Fatalf("me body %v", m)
	}

	// Cookie vớ vẩn → 401 và cookie bị xoá.
	w = ts.do(http.MethodGet, "/api/me", "rac-rac", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("me bad cookie status %d", w.Code)
	}
	cleared := false
	for _, c := range w.Result().Cookies() {
		if c.Name == testCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("cookie rác không bị xoá")
	}

	// Logout: session chết thật, /api/me sau đó 401.
	w = ts.do(http.MethodPost, "/api/logout", cookie, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("logout status %d", w.Code)
	}
	w = ts.do(http.MethodGet, "/api/me", cookie, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status %d", w.Code)
	}
}

func TestLoginBadJSON(t *testing.T) {
	ts := newTestServer(t, true)
	w := ts.do(http.MethodPost, "/api/login", "", `{không phải json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
	if m := decode(t, w); m["error"] != "body không phải JSON hợp lệ" {
		t.Fatalf("body %v", m)
	}
}

// TestDecodeRejectsTrailingData khoá hành vi chung của DecodeJSON: chỉ một
// giá trị JSON, không rác sau nó, và body quá cỡ bị chặn trước khi service
// chạy (login là route dùng chung đầu tiên tiện thử).
func TestDecodeRejectsTrailingData(t *testing.T) {
	ts := newTestServer(t, true)
	cases := []struct {
		name string
		body string
	}{
		{"hai giá trị", `{"username":"admin","password":"admin-pass"}{"username":"x"}`},
		{"rác sau giá trị", `{"username":"admin","password":"admin-pass"} xyz`},
		{"mảng thừa", `{"username":"admin","password":"admin-pass"}[1]`},
	}
	for _, c := range cases {
		w := ts.do(http.MethodPost, "/api/login", "", c.body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", c.name, w.Code)
		}
		var m map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatalf("%s: body %q", c.name, w.Body.String())
		}
		if m["error"] != "body không phải JSON hợp lệ" {
			t.Fatalf("%s: body %v", c.name, m)
		}
	}
}

// TestDecodeCodeVariant kiểm DecodeJSONCode gắn code khi client sai body.
func TestDecodeCodeVariant(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{rác`))
	var into map[string]any
	if webapp.DecodeJSONCode(rec, r, &into, "invalid_input") {
		t.Fatal("body rác phải trả false")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["code"] != "invalid_input" {
		t.Fatalf("code %v", m["code"])
	}
	// Không code: giữ vỏ cũ, không thêm khoá.
	rec = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{rác`))
	if webapp.DecodeJSON(rec, r, &into) {
		t.Fatal("body rác phải trả false")
	}
	var plain map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &plain); err != nil {
		t.Fatal(err)
	}
	if _, ok := plain["code"]; ok {
		t.Fatalf("DecodeJSON không được thêm code: %v", plain)
	}
}

func TestLoginLimiter(t *testing.T) {
	ts := newTestServer(t, true)
	// Limiter khoá sau 5 lần sai liên tiếp trên cùng key.
	for i := range 5 {
		w := ts.do(http.MethodPost, "/api/login", "", `{"username":"admin","password":"sai"}`)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status %d", i+1, w.Code)
		}
	}
	w := ts.do(http.MethodPost, "/api/login", "", `{"username":"admin","password":"admin-pass"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("locked status %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 thiếu Retry-After")
	}
	m := decode(t, w)
	if m["error"] != "Thử sai quá nhiều lần, vui lòng đợi rồi thử lại" {
		t.Fatalf("429 body %v", m)
	}
	if _, ok := m["retryAfter"].(float64); !ok {
		t.Fatalf("429 retryAfter %v", m)
	}
}

// ── setup lần đầu ───────────────────────────────────────────────────────────

func TestSetupStatusFresh(t *testing.T) {
	ts := newTestServer(t, false)
	w := ts.do(http.MethodGet, "/api/setup", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	m := decode(t, w)
	if m["needed"] != true || m["expiresAt"] == nil || m["attemptsLeft"] == nil {
		t.Fatalf("setup status %v", m)
	}
}

func TestSetupCodeOnce(t *testing.T) {
	ts := newTestServer(t, false)
	code := ts.s.SetupCodeOnce()
	if code == "" {
		t.Fatal("store trống phải có mã thiết lập")
	}
	if again := ts.s.SetupCodeOnce(); again != "" {
		t.Fatalf("mã chỉ được in một lần, lần hai %q", again)
	}
}

func TestSetupCodeOnceWithUsers(t *testing.T) {
	ts := newTestServer(t, true)
	if code := ts.s.SetupCodeOnce(); code != "" {
		t.Fatalf("đã có tài khoản không nên có mã thiết lập, %q", code)
	}
	w := ts.do(http.MethodGet, "/api/setup", "", "")
	m := decode(t, w)
	if m["needed"] != false {
		t.Fatalf("setup status with users %v", m)
	}
}

func TestSetupVerifyWrongAndFormat(t *testing.T) {
	ts := newTestServer(t, false)

	// Sai định dạng → 400 {format:true}.
	w := ts.do(http.MethodPost, "/api/setup/verify", "", `{"code":"x!"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("format status %d", w.Code)
	}
	if m := decode(t, w); m["format"] != true {
		t.Fatalf("format body %v", m)
	}

	// Sai mã đúng định dạng → 400 kèm attemptsLeft.
	w = ts.do(http.MethodPost, "/api/setup/verify", "", `{"code":"ZZZZ-ZZZZ"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("wrong status %d", w.Code)
	}
	m := decode(t, w)
	if m["attemptsLeft"] == nil || m["format"] != nil {
		t.Fatalf("wrong body %v", m)
	}
}

func TestSetupFlowEndToEnd(t *testing.T) {
	ts := newTestServer(t, false)
	code := ts.s.SetupCodeOnce()
	if code == "" {
		t.Fatal("thiếu mã thiết lập")
	}

	// Sai token → 410 trước khi verify.
	w := ts.do(http.MethodPost, "/api/setup/admin", "", `{"setupToken":"vov","name":"Admin","username":"admin","password":"admin-pass"}`)
	if w.Code != http.StatusGone {
		t.Fatalf("admin no token status %d", w.Code)
	}

	w = ts.do(http.MethodPost, "/api/setup/verify", "", `{"code":"`+code+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("verify status %d: %s", w.Code, w.Body.String())
	}
	token, _ := decode(t, w)["setupToken"].(string)
	if token == "" {
		t.Fatal("verify không trả setupToken")
	}

	// Form hỏng → 400 kèm field.
	w = ts.do(http.MethodPost, "/api/setup/admin", "", `{"setupToken":"`+token+`","name":"","username":"a!","password":"x"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad form status %d", w.Code)
	}
	if m := decode(t, w); m["field"] == nil {
		t.Fatalf("bad form body %v", m)
	}

	w = ts.do(http.MethodPost, "/api/setup/admin", "", `{"setupToken":"`+token+`","name":"Quản trị","username":"admin","password":"admin-pass"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("admin status %d: %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if m["recoveryCode"] == nil || m["user"].(map[string]any)["username"] != "admin" {
		t.Fatalf("admin body %v", m)
	}
	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == testCookie && c.Value != "" {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("setup admin không trả session cookie")
	}

	// Sau khi có tài khoản: status lật needed=false, setup/admin 409.
	w = ts.do(http.MethodGet, "/api/setup", "", "")
	if m := decode(t, w); m["needed"] != false {
		t.Fatalf("status after admin %v", m)
	}
	w = ts.do(http.MethodPost, "/api/setup/admin", "", `{"setupToken":"`+token+`","name":"X","username":"x2","password":"x2-pass"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("admin after taken status %d", w.Code)
	}
	if m := decode(t, w); m["error"] != "TestApp đã có quản trị viên" {
		t.Fatalf("taken body %v", m)
	}

	// Session cấp lúc setup dùng được ngay.
	w = ts.do(http.MethodGet, "/api/me", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("me after setup status %d", w.Code)
	}
}

// ── Handle: route của app ───────────────────────────────────────────────────

func TestHandleExactAndPrefix(t *testing.T) {
	ts := newTestServer(t, true)
	cookie := ts.sessionCookie()

	var hits []string
	ts.s.Handle(http.MethodGet, "/api/thing", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		hits = append(hits, "exact:"+user.Username)
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"hit": "exact"})
	})
	ts.s.Handle(http.MethodGet, "/api/thing/", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		hits = append(hits, "prefix:"+r.URL.Path)
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"hit": "prefix"})
	})

	// Chưa đăng nhập: route của app vẫn 401.
	w := ts.do(http.MethodGet, "/api/thing", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated app route status %d", w.Code)
	}
	if len(hits) != 0 {
		t.Fatalf("handler chạy khi chưa đăng nhập: %v", hits)
	}

	// Path chính xác khớp exact, không rơi vào prefix.
	w = ts.do(http.MethodGet, "/api/thing", cookie, "")
	if w.Code != http.StatusOK || decode(t, w)["hit"] != "exact" {
		t.Fatalf("exact status %d body %s", w.Code, w.Body.String())
	}
	if hits[len(hits)-1] != "exact:admin" {
		t.Fatalf("user không vào handler: %v", hits)
	}

	// Path bên dưới khớp prefix.
	w = ts.do(http.MethodGet, "/api/thing/sau/sau", cookie, "")
	if w.Code != http.StatusOK || decode(t, w)["hit"] != "prefix" {
		t.Fatalf("prefix status %d body %s", w.Code, w.Body.String())
	}

	// Sai method → 404.
	w = ts.do(http.MethodPost, "/api/thing", cookie, `{}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("wrong method status %d", w.Code)
	}
}

func TestHandlePrefixDoesNotEatExactPath(t *testing.T) {
	ts := newTestServer(t, true)
	cookie := ts.sessionCookie()

	ts.s.Handle(http.MethodGet, "/api/runs/", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"hit": "prefix"})
	})
	// "/api/runs" không kết thúc "/" nên không khớp prefix "/api/runs/".
	w := ts.do(http.MethodGet, "/api/runs", cookie, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d", w.Code)
	}
	w = ts.do(http.MethodGet, "/api/runs/123", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("prefix status %d", w.Code)
	}
}
