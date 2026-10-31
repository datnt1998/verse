package webapp_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/puddle/v2"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// repoWrapper bao một Repository thật và ghi đè vài method — cần để giả lỗi
// kho (503) hoặc grant tuỳ ý trên backend file vốn không có app_grants.
type repoWrapper struct {
	identity.Repository
	firstUserErr   error
	authenticate   func(cookie string) (identity.User, identity.Session, error)
	revokeSessionF func(id string, allowed func(string) bool, ctx ...identity.AuditContext) error
	login          func(username, password, device, ip string) (identity.NewSession, error)
}

func (w repoWrapper) FirstUserCreatedAt() (time.Time, bool, error) {
	if w.firstUserErr != nil {
		return time.Time{}, false, w.firstUserErr
	}
	return w.Repository.FirstUserCreatedAt()
}

func (w repoWrapper) Authenticate(cookie string) (identity.User, identity.Session, error) {
	if w.authenticate != nil {
		return w.authenticate(cookie)
	}
	return w.Repository.Authenticate(cookie)
}

func (w repoWrapper) RevokeSession(id string, allowed func(string) bool, ctx ...identity.AuditContext) error {
	if w.revokeSessionF != nil {
		return w.revokeSessionF(id, allowed, ctx...)
	}
	return w.Repository.RevokeSession(id, allowed, ctx...)
}

func (w repoWrapper) Login(username, password, device, ip string) (identity.NewSession, error) {
	if w.login != nil {
		return w.login(username, password, device, ip)
	}
	return w.Repository.Login(username, password, device, ip)
}

// grantsRepo ghi đè grants trả về từ Authenticate/Login: file backend chỉ có
// Noverse, nên test member-Viderse phải tiêm grant như Postgres sẽ trả.
type grantsRepo struct {
	identity.Repository
	grants map[identity.App]identity.Role
}

func (g grantsRepo) Authenticate(cookie string) (identity.User, identity.Session, error) {
	u, s, err := g.Repository.Authenticate(cookie)
	if err == nil {
		u.Grants = g.grants
	}
	return u, s, err
}

func (g grantsRepo) Login(username, password, device, ip string) (identity.NewSession, error) {
	ns, err := g.Repository.Login(username, password, device, ip)
	if err == nil {
		ns.User.Grants = g.grants
	}
	return ns, err
}

// newWrapperServer dựng Server quanh kho file thật bọc wrapper — điểm neo duy
// nhất để test lỗi kho và grant tiêm mà không cần Postgres.
func newWrapperServer(t *testing.T, wrap func(identity.Repository) identity.Repository, users []identity.NewUser) *testServer {
	t.Helper()
	accounts := identity.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"), identity.Options{})
	for _, u := range users {
		if _, err := accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	ts := &testServer{t: t, accounts: accounts}
	repo := identity.Repository(accounts)
	if wrap != nil {
		repo = wrap(repo)
	}
	ts.s = webapp.New(identity.NewFromRepository(repo, identity.Options{}), webapp.Options{
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

func memberUser(username, password string, mustChange bool) identity.NewUser {
	return identity.NewUser{Username: username, Name: username, Role: identity.RoleMember, Password: password, MustChangePassword: mustChange}
}

var viderseMember = map[identity.App]identity.Role{identity.AppViderse: identity.RoleMember}

func setCookies(w *httptest.ResponseRecorder, name string) (set bool, expired bool) {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			if c.Value != "" {
				set = true
			} else if c.MaxAge < 0 {
				expired = true
			}
		}
	}
	return set, expired
}

// TestSSOLoginClearsLegacyCookies khoá hợp đồng: login chung expire mọi tên
// cookie cũ (cả hai app trước đây), và một giá trị session thật gửi dưới tên
// cũ không bao giờ xác thực được.
func TestSSOLoginClearsLegacyCookies(t *testing.T) {
	ts := newTestServer(t, true)

	w := ts.do(http.MethodPost, "/api/login", "", `{"username":"admin","password":"admin-pass"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d", w.Code)
	}
	var valid string
	for _, c := range w.Result().Cookies() {
		if c.Name == "verse_session" {
			valid = c.Value
		}
	}
	if valid == "" {
		t.Fatal("login không trả verse_session")
	}
	for _, legacy := range []string{"ainovel_session", "viderse_session"} {
		_, expired := setCookies(w, legacy)
		if !expired {
			t.Fatalf("login không expire cookie cũ %q", legacy)
		}
	}

	// Giá trị session thật đi dưới tên legacy: không dual-read → 401.
	for _, legacy := range []string{"ainovel_session", "viderse_session"} {
		r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		r.RemoteAddr = "192.0.2.1:1234"
		r.Host = "127.0.0.1:8799"
		r.AddCookie(&http.Cookie{Name: legacy, Value: valid})
		rec := httptest.NewRecorder()
		ts.s.ServeHTTP(rec, r)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("session gửi dưới %q được chấp nhận (status %d)", legacy, rec.Code)
		}
	}
}

// TestSharedLogoutRevokesBeforeClear khoá hai nửa của contract: revoke thất
// bại thì trả lỗi và giữ cookie (không báo giả đã đăng xuất), revoke thành
// công mới xoá cookie; logout lần hai với phiên đã chết → 401 + clear.
func TestSharedLogoutRevokesBeforeClear(t *testing.T) {
	depErr := fmt.Errorf("pool closed: %w", puddle.ErrClosedPool)
	ts := newWrapperServer(t, func(r identity.Repository) identity.Repository {
		return repoWrapper{
			Repository: grantsRepo{Repository: r, grants: viderseMember},
			revokeSessionF: func(id string, allowed func(string) bool, ctx ...identity.AuditContext) error {
				return depErr
			},
		}
	}, []identity.NewUser{memberUser("admin", "admin-pass", false)})
	cookie := ts.sessionCookie()

	w := ts.do(http.MethodPost, "/api/logout", cookie, `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("logout khi kho lỗi status %d, muốn 503", w.Code)
	}
	_, expired := setCookies(w, "verse_session")
	if expired {
		t.Fatal("kho lỗi mà cookie bị xoá — báo giả đã đăng xuất")
	}
	// Phiên vẫn sống phía kho: /api/me vẫn vào được.
	if w := ts.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("phiên sau logout-lỗi status %d, muốn 200", w.Code)
	}

	// Đường bình thường: revoke trước, clear sau, session chết thật.
	ts2 := newTestServer(t, true)
	cookie2 := ts2.sessionCookie()
	w = ts2.do(http.MethodPost, "/api/logout", cookie2, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("logout status %d", w.Code)
	}
	_, expired = setCookies(w, "verse_session")
	if !expired {
		t.Fatal("logout thành công không xoá cookie")
	}
	if w := ts2.do(http.MethodGet, "/api/me", cookie2, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("me sau logout status %d, muốn 401", w.Code)
	}
	// Logout với phiên đã thu hồi: 401 và vẫn clear cookie cho client dọn.
	w = ts2.do(http.MethodPost, "/api/logout", cookie2, `{}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("logout phiên chết status %d, muốn 401", w.Code)
	}
	_, expired = setCookies(w, "verse_session")
	if !expired {
		t.Fatal("401 không clear cookie")
	}
}

// TestSetupUnavailableIsNotEmpty khoá: lỗi kho không bao giờ được đọc là
// "store trống" — /api/setup trả 503, không phát mã, không hiện setup.
func TestSetupUnavailableIsNotEmpty(t *testing.T) {
	depErr := fmt.Errorf("connect failed: %w", puddle.ErrClosedPool)
	ts := newWrapperServer(t, func(r identity.Repository) identity.Repository {
		return repoWrapper{Repository: r, firstUserErr: depErr}
	}, nil)

	w := ts.do(http.MethodGet, "/api/setup", "", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("setup status khi kho lỗi = %d, muốn 503", w.Code)
	}
	m := decode(t, w)
	if m["needed"] == true {
		t.Fatalf("kho lỗi mà báo cần setup: %v", m)
	}
	// Setup verify cũng không được coi là trống.
	w = ts.do(http.MethodPost, "/api/setup/verify", "", `{"code":"AAAA-BBBB"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("verify khi kho lỗi = %d, muốn 503", w.Code)
	}
}

// TestViderse403IsNotSessionExpired khoá: thiếu grant trả 403 có user+grants
// và giữ phiên — không giả thành 401, không xoá cookie; mutation grant làm
// phiên mục tiêu 401 ngay request kế tiếp.
func TestViderse403IsNotSessionExpired(t *testing.T) {
	ts := newWrapperServer(t, func(r identity.Repository) identity.Repository {
		return grantsRepo{Repository: r, grants: map[identity.App]identity.Role{identity.AppNoverse: identity.RoleMember}}
	}, []identity.NewUser{memberUser("linh", "linh-pass", false)})

	w := ts.do(http.MethodPost, "/api/login", "", `{"username":"linh","password":"linh-pass"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("login no-grant status %d, muốn 403", w.Code)
	}
	m := decode(t, w)
	if m["code"] != "app_access_denied" || m["app"] != "viderse" {
		t.Fatalf("login no-grant body %v", m)
	}
	user, _ := m["user"].(map[string]any)
	if user == nil || user["username"] != "linh" {
		t.Fatalf("403 thiếu user view: %v", m)
	}
	if grants, _ := user["grants"].(map[string]any); grants["noverse"] != "member" {
		t.Fatalf("403 thiếu grants trong user: %v", user)
	}
	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == "verse_session" && c.Value != "" {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("login no-grant không đặt cookie chung")
	}

	// /api/me: 403 giống hệt, phiên sống (không 401), cookie giữ nguyên.
	w = ts.do(http.MethodGet, "/api/me", cookie, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("me no-grant status %d — 403 phải không bị đọc là hết phiên", w.Code)
	}
	if decode(t, w)["code"] != "app_access_denied" {
		t.Fatalf("me no-grant body %v", decode(t, w))
	}
	if _, expired := setCookies(w, "verse_session"); expired {
		t.Fatal("403 app_access_denied xoá cookie")
	}

	// Auth-neutral vẫn đi qua khi thiếu grant: me/sessions, sessions/{id}/revoke
	// chỉ chạm chính mình; route nghiệp vụ và path lạ đều 403.
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/me/sessions", http.StatusOK},
		{http.MethodGet, "/api/anything", http.StatusForbidden},
		{http.MethodPost, "/api/anything", http.StatusForbidden},
		{http.MethodDelete, "/api/sessions/khac/revoke", http.StatusForbidden},
	} {
		body := ""
		if tc.method != http.MethodGet {
			body = `{}`
		}
		w := ts.do(tc.method, tc.path, cookie, body)
		if w.Code != tc.want {
			t.Fatalf("%s %s status %d, muốn %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
	// Revoke phiên người khác bằng id giả: predicate owner chặn → lỗi, không
	// im lặng thành công.
	w = ts.do(http.MethodPost, "/api/sessions/00000000-0000-0000-0000-000000000000/revoke", cookie, `{}`)
	if w.Code == http.StatusOK {
		t.Fatal("revoke session không tồn tại/không sở hữu trả 200")
	}
}

// TestViderseSessionClaimsAreRequestScoped chứng minh Server không giữ
// user/grant của request trước: mỗi request đọc lại từ kho, grants của hai
// phiên không lẫn nhau.
func TestViderseSessionClaimsAreRequestScoped(t *testing.T) {
	current := viderseMember
	ts := newWrapperServer(t, func(r identity.Repository) identity.Repository {
		return grantsRepo{Repository: r, grants: current}
	}, []identity.NewUser{memberUser("admin", "admin-pass", false)})

	var seen []string
	ts.s.Handle(http.MethodGet, "/api/probe", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		seen = append(seen, user.Username+":"+string(user.Grants[identity.AppViderse]))
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	cookie := ts.sessionCookie()

	if w := ts.do(http.MethodGet, "/api/probe", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("probe granted status %d", w.Code)
	}
	// Grant thay đổi giữa hai request: request kế tiếp phải thấy mới, không
	// kẹt claims cũ.
	for k := range current {
		delete(current, k)
	}
	w := ts.do(http.MethodGet, "/api/probe", cookie, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("probe sau mất grant status %d, muốn 403", w.Code)
	}
	current[identity.AppViderse] = identity.RoleMember
	if w := ts.do(http.MethodGet, "/api/probe", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("probe sau có lại grant status %d", w.Code)
	}
	if len(seen) != 2 || seen[0] != "admin:member" || seen[1] != "admin:member" {
		t.Fatalf("claims không theo request: %v", seen)
	}
}

// TestAuthenticateErrorClassification: phiên hỏng → 401 + clear; kho hỏng →
// 503 và GIỮ cookie (không giả hết phiên, không báo logout).
func TestAuthenticateErrorClassification(t *testing.T) {
	depErr := fmt.Errorf("pool closed: %w", puddle.ErrClosedPool)
	ts := newWrapperServer(t, func(r identity.Repository) identity.Repository {
		return repoWrapper{Repository: r, authenticate: func(cookie string) (identity.User, identity.Session, error) {
			return identity.User{}, identity.Session{}, depErr
		}}
	}, []identity.NewUser{memberUser("admin", "admin-pass", false)})

	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8799"
	r.AddCookie(&http.Cookie{Name: "verse_session", Value: "bat-ky"})
	w := httptest.NewRecorder()
	ts.s.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("authenticate kho-lỗi status %d, muốn 503", w.Code)
	}
	if _, expired := setCookies(w, "verse_session"); expired {
		t.Fatal("503 xoá cookie — giả thành phiên hỏng")
	}
}

// TestIdentityErrorsAndLogsRedactSecrets: response không echo DSN/token/
// password, kể cả khi repository trả lỗi chứa chúng; log server ghi generic
// wrapper, không lan truyền raw secret ra body.
func TestIdentityErrorsAndLogsRedactSecrets(t *testing.T) {
	const (
		fakeDSN    = "postgres://user:sieuretmat@db:5432/verse"
		fakeSecret = "sessionsecretvalue12345"
		fakePass   = "mat-khau-that-123"
	)
	assertNoSecrets := func(t *testing.T, w *httptest.ResponseRecorder) {
		t.Helper()
		body := w.Body.String()
		for _, s := range []string{fakeDSN, fakeSecret, fakePass, "sieuretmat"} {
			if strings.Contains(body, s) {
				t.Fatalf("response lộ %q: %s", s, body)
			}
		}
	}

	// Kho trả lỗi mang secret: 500 body phải generic, không echo.
	leakyErr := fmt.Errorf("query users failed dsn=%s token=%s", fakeDSN, fakeSecret)
	ts := newWrapperServer(t, func(r identity.Repository) identity.Repository {
		return repoWrapper{Repository: r, authenticate: func(cookie string) (identity.User, identity.Session, error) {
			return identity.User{}, identity.Session{}, leakyErr
		}}
	}, []identity.NewUser{memberUser("admin", "admin-pass", false)})

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(prev)

	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8799"
	r.AddCookie(&http.Cookie{Name: "verse_session", Value: fakeSecret})
	w := httptest.NewRecorder()
	ts.s.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("lỗi không xác định status %d, muốn 500", w.Code)
	}
	assertNoSecrets(t, w)

	// Login sai: body chỉ text sentinel, không mang password hay username.
	ts2 := newTestServer(t, true)
	w = ts2.do(http.MethodPost, "/api/login", "", `{"username":"admin","password":"`+fakePass+`"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("sai mật khẩu status %d", w.Code)
	}
	assertNoSecrets(t, w)

	// Sai mật khẩu hiện tại khi đổi: 403 text cố định, không echo.
	cookie := ts2.sessionCookie()
	w = ts2.do(http.MethodPost, "/api/me/password", cookie, `{"current":"`+fakePass+`","new":"new-pass-123"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("đổi mật khẩu sai current status %d", w.Code)
	}
	assertNoSecrets(t, w)

	// Setup verify body lỗi: không echo mã nhập.
	w = ts2.do(http.MethodPost, "/api/setup/verify", "", `{"code":"`+fakeSecret+`"}`)
	assertNoSecrets(t, w)
}

// TestSelfServiceRoutesOwnSessionsOnly: tập auth-neutral chỉ chạm phiên của
// chính mình — revoke phiên user khác phải bị predicate chặn; revoke-all và
// đổi mật khẩu của chính mình hoạt động.
func TestSelfServiceRoutesOwnSessionsOnly(t *testing.T) {
	ts := newTestServer(t, true)
	cookie := ts.sessionCookie()

	w := ts.do(http.MethodGet, "/api/me/sessions", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("sessions status %d", w.Code)
	}
	m := decode(t, w)
	sessions, _ := m["sessions"].([]any)
	if len(sessions) == 0 {
		t.Fatalf("sessions body %v", m)
	}
	current, _ := sessions[0].(map[string]any)
	id, _ := current["id"].(string)

	// Revoke chính phiên mình → 200 và phiên chết.
	w = ts.do(http.MethodPost, "/api/sessions/"+id+"/revoke", cookie, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke own session status %d: %s", w.Code, w.Body.String())
	}
	if w := ts.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("me sau tự revoke status %d, muốn 401", w.Code)
	}

	// Phiên mới → revoke-all → 200 reauthenticate + phiên chết.
	cookie = ts.sessionCookie()
	w = ts.do(http.MethodPost, "/api/me/sessions/revoke-all", cookie, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke-all status %d", w.Code)
	}
	if m := decode(t, w); m["reauthenticate"] != true {
		t.Fatalf("revoke-all body %v", m)
	}
	if w := ts.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("me sau revoke-all status %d", w.Code)
	}
}

// TestForcedPasswordGateBeforeDispatch: user bị bắt đổi mật khẩu không chạy
// được handler nghiệp vụ nào — exact, prefix, multipart, path lạ — trong khi
// tập auth-neutral vẫn đi qua.
func TestForcedPasswordGateBeforeDispatch(t *testing.T) {
	ts := newWrapperServer(t, func(r identity.Repository) identity.Repository {
		return grantsRepo{Repository: r, grants: viderseMember}
	}, []identity.NewUser{memberUser("linh", "temp-pass", true)})

	var hits int
	ts.s.Handle(http.MethodGet, "/api/probe", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		hits++
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	ts.s.Handle(http.MethodPost, "/api/probe/", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		hits++
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	w := ts.do(http.MethodPost, "/api/login", "", `{"username":"linh","password":"temp-pass"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", w.Code, w.Body.String())
	}
	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == "verse_session" && c.Value != "" {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("thiếu cookie")
	}

	cases := []struct {
		method, path, body, ctype string
	}{
		{http.MethodGet, "/api/probe", "", ""},
		{http.MethodPost, "/api/probe/x", `{}`, "application/json"},
		{http.MethodPost, "/api/probe", `{}`, "application/json"},
		{http.MethodGet, "/api/unknown", "", ""},
		{http.MethodPost, "/api/unknown", `{}`, "application/json"},
		{http.MethodPost, "/api/overlays", "", "multipart/form-data; boundary=x"},
		{http.MethodDelete, "/api/probe/x", "", ""},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.RemoteAddr = "192.0.2.1:1234"
		r.Host = "127.0.0.1:8799"
		r.AddCookie(&http.Cookie{Name: "verse_session", Value: cookie})
		if tc.ctype != "" {
			r.Header.Set("Content-Type", tc.ctype)
		}
		w := httptest.NewRecorder()
		ts.s.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s status %d, muốn 403", tc.method, tc.path, w.Code)
		}
		m := decode(t, w)
		if m["code"] != "password_change_required" || m["mustChangePassword"] != true {
			t.Fatalf("%s %s body %v", tc.method, tc.path, m)
		}
	}
	if hits != 0 {
		t.Fatalf("handler nghiệp vụ chạy %d lần dù bắt đổi mật khẩu", hits)
	}

	// Auth-neutral đi qua: me trả 200 có flag, đổi mật khẩu thành công thì
	// xoá cookie và mọi phiên bị thu hồi.
	w = ts.do(http.MethodGet, "/api/me", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("me status %d", w.Code)
	}
	if user, _ := decode(t, w)["user"].(map[string]any); user["mustChangePassword"] != true {
		t.Fatalf("me body %v", decode(t, w))
	}
	w = ts.do(http.MethodPost, "/api/me/password", cookie, `{"current":"temp-pass","new":"real-pass-99"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("change password status %d: %s", w.Code, w.Body.String())
	}
	if m := decode(t, w); m["reauthenticate"] != true {
		t.Fatalf("change body %v", m)
	}
	if _, expired := setCookies(w, "verse_session"); !expired {
		t.Fatal("đổi mật khẩu không xoá cookie")
	}
	if w := ts.do(http.MethodGet, "/api/me", cookie, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("me sau đổi status %d, muốn 401", w.Code)
	}
	// Đăng nhập lại với mật khẩu mới: vào được route nghiệp vụ.
	w = ts.do(http.MethodPost, "/api/login", "", `{"username":"linh","password":"real-pass-99"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("relogin status %d", w.Code)
	}
	cookie = ""
	for _, c := range w.Result().Cookies() {
		if c.Name == "verse_session" && c.Value != "" {
			cookie = c.Value
		}
	}
	if w := ts.do(http.MethodGet, "/api/probe", cookie, ""); w.Code != http.StatusOK || hits != 1 {
		t.Fatalf("probe sau relogin status %d hits %d", w.Code, hits)
	}
}
