// Package webapp là lớp HTTP dùng chung của các app trong repo: SPA nhúng,
// các route /api nền (setup lần đầu, login, me, logout) sau session cookie và
// guard origin của platform/webguard. App chỉ định danh riêng qua Options và
// đăng ký route nghiệp vụ bằng Handle.
package webapp

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/puddle/v2"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/secheaders"
	"github.com/datnt1998/verse/platform/webguard"
)

// MIME của tệp gốc SPA: Go không có sẵn hai đuôi này, và máy không có
// mime.types hệ thống (container tối giản) sẽ trả Content-Type rỗng. Trình
// duyệt từ chối manifest PWA khi kiểu không phải application/manifest+json,
// nên đăng ký một lần cho mọi app dùng lớp này.
func init() {
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	_ = mime.AddExtensionType(".ico", "image/x-icon")
}

// Options là định danh riêng của từng app mà Server cần.
type Options struct {
	// AppName đi vào các lỗi hiển thị cho người dùng trong luồng thiết lập
	// và lỗi thiếu quyền (vd "Viderse đã có quản trị viên").
	AppName string
	// App là định danh của app trong app_grants: session hợp lệ chỉ vào route
	// nghiệp vụ khi grants hiệu lực chứa app này (admin toàn cục luôn có).
	App identity.App
	// UI là gốc SPA đã build (index.html + assets/ ở root). Caller thường
	// truyền fs.Sub(embed, "dist").
	UI fs.FS
	// AllowNonJSON miễn kiểm Content-Type application/json cho những request
	// tải tệp (multipart) mà app tự khai. nil = mọi body phải JSON. Chỉ dùng
	// cho route nhận tệp: ranh giới CSRF vẫn là webguard.CrossSiteRequest +
	// cookie SameSite chạy trước đó.
	AllowNonJSON func(*http.Request) bool
}

// sessionCookieName là tên cookie chia sẻ của cả hai app trên kho tài khoản
// chung: đăng nhập ở app này mở được app kia. Không có option đổi tên — hai
// tên khác nhau sẽ làm vỡ contract "một phiên một cookie".
const sessionCookieName = "verse_session"

// legacySessionCookies là các tên cookie của các bản trước. Đăng nhập chung
// expire chúng ở mọi scope deployment có thể đã đặt; không bao giờ đọc chúng
// làm fallback.
var legacySessionCookies = []string{"ainovel_session", "viderse_session"}

// HandlerFunc là route nghiệp vụ của app: chỉ chạy sau khi session cookie đã
// xác thực, tài khoản có grant app và không bị bắt đổi mật khẩu; user là
// tài khoản hiện tại từ Store.Authenticate của chính request đó.
type HandlerFunc func(w http.ResponseWriter, r *http.Request, user identity.User)

// route là một route /api do app đăng ký: exact khớp đúng path, prefix khớp mọi path bên dưới.
type route struct {
	method string
	exact  string
	prefix string
	h      HandlerFunc
}

// Server phục vụ toàn bộ HTTP của app.
type Server struct {
	accounts *identity.Store
	opts     Options
	proxy    *webguard.Config
	sec      secheaders.Policy
	limiter  identity.LoginLimiter

	setup     *identity.SetupState
	setupCode string
	setupMu   sync.Mutex

	routes []route
}

// New dựng handler: accounts là kho identity; khi chưa có tài khoản nào,
// server tạo một setup state (chỉ giữ hash) — caller in mã plaintext một lần
// qua SetupCodeOnce.
func New(accounts *identity.Store, opts Options) *Server {
	if !opts.App.Valid() {
		// App là hợp đồng bảo mật: sai định danh nghĩa là gate kiểm sai app,
		// tốt hơn crash tại construction là chạy một gate vô nghĩa.
		panic("webapp.Options.App phải là app đã đăng ký (noverse|viderse)")
	}
	s := &Server{
		accounts: accounts,
		opts:     opts,
		limiter:  accounts.LoginLimiter(),
	}
	if has, err := accounts.HasUsers(); err == nil && !has {
		if state, code, err := identity.NewSetupState(time.Now); err == nil {
			s.setup, s.setupCode = state, code
		}
	}
	return s
}

// SetProxyConfig cài ranh giới trusted-proxy đọc từ env lúc khởi động. Header
// bảo mật đi cùng: HSTS chỉ đúng khi PUBLIC_URL là https.
func (s *Server) SetProxyConfig(cfg *webguard.Config) {
	s.proxy = cfg
	s.sec = secheaders.FromProxy(cfg)
}

// SecurityHeaders ghi bộ header bảo mật của deployment. ServeHTTP gọi trước
// dispatch; app nào có đường dẫn bỏ qua ServeHTTP (render riêng của Viderse)
// phải gọi lại, nếu không response đó mất nosniff/frame-ancestors.
func (s *Server) SecurityHeaders(w http.ResponseWriter) { s.sec.Apply(w) }

// SetupCodeOnce trả mã thiết lập plaintext cho caller in đúng một lần; sau đó
// xoá khỏi bộ nhớ (chỉ còn hash).
func (s *Server) SetupCodeOnce() string {
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	code := s.setupCode
	s.setupCode = ""
	return code
}

// Handle đăng ký một route /api đã xác thực của app. pattern là path chính
// xác ("/api/runs") hoặc prefix kết thúc bằng "/" khớp mọi path bên dưới
// ("/api/runs/"). Thứ tự đăng ký là thứ tự khớp.
func (s *Server) Handle(method, pattern string, h HandlerFunc) {
	r := route{method: method, h: h}
	if strings.HasSuffix(pattern, "/") {
		r.prefix = pattern
	} else {
		r.exact = pattern
	}
	s.routes = append(s.routes, r)
}

// RequireSameOrigin applies the configured browser-origin policy, including its
// app-origin allowlist, to sensitive reads as well as writes. Headerless clients
// retain the normal non-browser policy. Callers must return on rejection.
func (s *Server) RequireSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	check := *r
	check.Method = http.MethodPost
	if s.proxy.CrossSiteRequest(&check) {
		Error(w, http.StatusForbidden, "yêu cầu khác origin bị từ chối")
		return false
	}
	return true
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Chuẩn hoá trusted-proxy phải đi trước mọi phán đoán bảo mật (khoá giới hạn, Secure cookie, origin CSRF).
	r = s.proxy.Normalize(r)
	// Header bảo mật đứng trước dispatch: health, 401, tệp tĩnh và lỗi API đều mang header.
	s.sec.Apply(w)
	if s.sec.Robots(w, r) {
		return
	}
	switch r.URL.Path {
	case "/healthz":
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	case "/readyz":
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if s.proxy.CrossSiteRequest(r) {
		Error(w, http.StatusForbidden, "yêu cầu khác origin bị từ chối")
		return
	}
	if r.URL.Path == "/api" || !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.URL.Path == "/api" {
			Error(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		s.serveStatic(w, r)
		return
	}
	if webguard.JSONBodyRequired(r) && !webguard.JSONContent(r) &&
		(s.opts.AllowNonJSON == nil || !s.opts.AllowNonJSON(r)) {
		Error(w, http.StatusBadRequest, "Content-Type phải là application/json")
		return
	}
	switch {
	case r.URL.Path == "/api/login" && r.Method == http.MethodPost:
		s.login(w, r)
		return
	case s.serveSetup(w, r):
		// /api/setup* ngang hàng /api/login: chưa có tài khoản cũng phải vào được.
		return
	}
	cookie, err := r.Cookie(s.cookiePolicy().Name)
	if err != nil {
		WriteJSON(w, http.StatusUnauthorized, identity.EndedBody(identity.ErrUnauthenticated))
		return
	}
	user, current, err := s.accounts.Authenticate(cookie.Value)
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			var ended *identity.SessionEnded
			if !errors.As(err, &ended) {
				s.clearSession(w, r)
			}
			WriteJSON(w, http.StatusUnauthorized, identity.EndedBody(err))
			return
		}
		s.writeAccountError(w, err)
		return
	}
	// Auth-neutral trước mọi gate: session hợp lệ — kể cả thiếu grant app hay
	// đang bắt đổi mật khẩu — vẫn được logout, đổi mật khẩu và quản phiên của
	// CHÍNH MÌNH. Không đường nào ở nhánh này đụng tài khoản người khác.
	if s.serveAuthNeutral(w, r, user, current) {
		return
	}
	// Grant gate: thiếu quyền app trả 403 app_access_denied kèm view an toàn —
	// phiên vẫn sống (có thể mở được app kia), không phải hết hạn.
	if !s.appAccessGranted(user.Grants) {
		s.writeAppAccessDenied(w, user.View())
		return
	}
	// GET /api/me là đường duy nhất còn đọc được khi bắt đổi mật khẩu: màn
	// đổi mật khẩu cần biết đang đăng nhập ai.
	if r.URL.Path == "/api/me" && r.Method == http.MethodGet {
		WriteJSON(w, http.StatusOK, map[string]any{
			"user": user.View(), "session": map[string]any{"expiresAt": current.ExpiresAt},
			"apps": s.appLinks(user.View()),
		})
		return
	}
	// Forced-password gate chạy trước mọi dispatch: upload multipart, SSE và
	// cả path chưa biết đều 403 password_change_required, không vào handler.
	if user.MustChangePassword {
		s.writePasswordChangeRequired(w)
		return
	}
	for _, rt := range s.routes {
		if rt.method != r.Method {
			continue
		}
		if (rt.exact != "" && r.URL.Path == rt.exact) || (rt.prefix != "" && strings.HasPrefix(r.URL.Path, rt.prefix)) {
			rt.h(w, r, user)
			return
		}
	}
	Error(w, http.StatusNotFound, "không tìm thấy")
}

// ── tĩnh (SPA nhúng) ────────────────────────────────────────────────────────

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	path := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+r.URL.Path)), "/")
	if path == "" {
		path = "index.html"
	}
	data, err := fs.ReadFile(s.opts.UI, path)
	if err != nil {
		// Tệp băm trong assets/ không có thì là 404 thật.
		if strings.HasPrefix(path, "assets/") {
			Error(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		path = "index.html"
		data, err = fs.ReadFile(s.opts.UI, path)
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if typ := mime.TypeByExtension(filepath.Ext(path)); typ != "" {
		w.Header().Set("Content-Type", typ)
	}
	if strings.HasPrefix(path, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

// ── session ─────────────────────────────────────────────────────────────────

// cookiePolicy trả hợp đồng cookie bất biến của server này: tên cố định của
// kho chung, domain lấy từ cấu hình proxy đã parse lúc khởi động.
func (s *Server) cookiePolicy() webguard.SessionCookie {
	return webguard.NewSessionCookie(sessionCookieName, s.proxy)
}

// SessionCookieName là tên cookie đọc-ghi của phiên chung — cho các luồng
// bên ngoài dispatch (vd. recheck phiên của SSE) đọc đúng tên thay vì
// hardcode một hằng riêng.
func (s *Server) SessionCookieName() string { return s.cookiePolicy().Name }

// appAccessGranted báo tài khoản có quyền hiệu lực dùng app này ngay bây giờ:
// admin toàn cục luôn vào được; member cần grant row đọc lại từ kho chung
// (Authenticate đọc mỗi request — không cache grant trên session).
func (s *Server) appAccessGranted(grants map[identity.App]identity.Role) bool {
	return grants[s.opts.App] != ""
}

// BusinessAllowed báo một user đã xác thực được phép vào route nghiệp vụ:
// đủ grant app và không bị bắt đổi mật khẩu. ServeHTTP dùng cùng điều kiện;
// caller ngoài (SSE recheck) gọi lại trên kết quả Authenticate mới.
func (s *Server) BusinessAllowed(user identity.User) bool {
	return s.appAccessGranted(user.Grants) && !user.MustChangePassword
}

func (s *Server) setSession(w http.ResponseWriter, r *http.Request, ns identity.NewSession) {
	policy := s.cookiePolicy()
	for _, name := range legacySessionCookies {
		policy.Expire(w, r, name)
	}
	policy.Set(w, r, ns.Cookie, ns.ExpiresAt)
}

func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	s.cookiePolicy().Clear(w, r)
}

// clearDeviceCookie forgets the browser's known-device token. The server
// already revoked it in the same request (password change, revoke-all); the
// cookie only stops carrying a dead value for the next 180 days.
func (s *Server) clearDeviceCookie(w http.ResponseWriter, r *http.Request) {
	webguard.NewDeviceCookie().Clear(w, r)
}

// writeAppAccessDenied ghi hợp đồng 403 của phiên hợp lệ thiếu grant app:
// khác 401 (phiên thật và còn sống), trường user là view an toàn — có cả
// grants, không bao giờ chứa credential.
func (s *Server) writeAppAccessDenied(w http.ResponseWriter, user identity.UserView) {
	WriteJSON(w, http.StatusForbidden, map[string]any{
		"code":  "app_access_denied",
		"app":   string(s.opts.App),
		"error": "Tài khoản của bạn chưa được cấp quyền dùng " + s.opts.AppName + ". Vui lòng liên hệ quản trị viên.",
		"user":  user,
		"apps":  s.appLinks(user),
	})
}

func (s *Server) appLinks(user identity.UserView) []webguard.AppLink {
	return s.proxy.LinksFor(string(s.opts.App), func(id string) bool {
		return !user.Disabled && (user.Role == identity.RoleAdmin || user.Grants[identity.App(id)] != "")
	})
}

// writePasswordChangeRequired ghi 403 của tài khoản phải đổi mật khẩu tạm
// trước khi tiếp tục — áp cho mọi route nghiệp vụ, kể cả admin.
func (s *Server) writePasswordChangeRequired(w http.ResponseWriter) {
	WriteJSON(w, http.StatusForbidden, map[string]any{
		"code":               "password_change_required",
		"mustChangePassword": true,
		"error":              "Bạn cần đổi mật khẩu tạm trước khi tiếp tục.",
	})
}

// ── login / logout ──────────────────────────────────────────────────────────

// rememberDevice records this browser as a known device of the account. Only a
// successful credential check reaches it, and the token it mints grants
// nothing on its own — it only moves the next login attempt of this browser to
// a per-device limiter bucket. An already-known device keeps its cookie (the
// server-side lifetime does not move); a mint failure never fails the login.
func (s *Server) rememberDevice(w http.ResponseWriter, r *http.Request, username string, known bool) {
	if known {
		return
	}
	token, err := s.accounts.IssueDeviceToken(username)
	if err != nil {
		return
	}
	webguard.NewDeviceCookie().Set(w, r, token.Token, token.ExpiresAt)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !DecodeJSON(w, r, &req) {
		return
	}
	// Username chữ thường: bucket limiter và tra cứu dùng chung một dạng.
	username := strings.ToLower(strings.TrimSpace(req.Username))
	device := webguard.NewDeviceCookie()
	// Thiết bị đã đăng nhập thành công trước đó được tính bucket riêng
	// (user, thiết bị): kẻ tấn công làm cạn bucket user:<tên> từ nơi khác
	// không khoá được nó. Token giả/hết hạn/lạ rơi về bucket chung.
	keys, known := s.accounts.LoginBuckets(username, webguard.ClientIP(r), device.Read(r))
	if wait := s.limiter.Wait(keys...); wait > 0 {
		tooMany(w, wait)
		return
	}
	ns, err := s.accounts.Login(username, req.Password, identity.DeviceLabel(r.UserAgent()), webguard.ClientIP(r))
	if err != nil {
		if errors.Is(err, identity.ErrInvalidCredentials) {
			s.limiter.Fail(keys...)
			if len(keys) > 1 && s.limiter.Wait(keys[1]) > 0 {
				if auditErr := s.accounts.RecordSecurityFailure(username); auditErr != nil {
					s.writeAccountError(w, auditErr)
					return
				}
			}
		}
		s.writeAccountError(w, err)
		return
	}
	s.limiter.Succeed(keys...)
	s.rememberDevice(w, r, username, known)
	s.setSession(w, r, ns)
	// Đúng credential mà thiếu grant app vẫn đặt cookie (phiên có thể mở app
	// kia) nhưng trả 403, không giả thành công cũng không tính fail limiter.
	if !s.appAccessGranted(ns.User.Grants) {
		s.writeAppAccessDenied(w, ns.User)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"user": ns.User})
}

// ── tự phục vụ phiên (auth-neutral) ─────────────────────────────────────────

// serveAuthNeutral xử các endpoint mọi session hợp lệ đều được phép — trước
// grant gate và forced-password gate: chỉ chạm tài khoản/phiên của chính
// caller. Method+path phải khớp chính xác; sai method rơi xuống các gate
// chung (thiếu grant → 403, không lộ diện route).
func (s *Server) serveAuthNeutral(w http.ResponseWriter, r *http.Request, user identity.User, current identity.Session) bool {
	path := r.URL.Path
	switch {
	case path == "/api/logout" && r.Method == http.MethodPost:
		s.doLogout(w, r, user, current)
	case path == "/api/me/password" && r.Method == http.MethodPost:
		s.doPasswordChange(w, r, user)
	case path == "/api/me/sessions" && r.Method == http.MethodGet:
		s.listSessions(w, user.Username, current.ID)
	case path == "/api/me/sessions/revoke-all" && r.Method == http.MethodPost:
		s.doRevokeAll(w, r, user, current)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/api/sessions/") && strings.HasSuffix(path, "/revoke"):
		s.doRevokeSession(w, r, user, current,
			strings.TrimSuffix(strings.TrimPrefix(path, "/api/sessions/"), "/revoke"))
	default:
		return false
	}
	return true
}

// doLogout invalidates the token but retains its original cookie lifetime.
// Sibling apps can then read the end reason; the token grants no API access.
func (s *Server) doLogout(w http.ResponseWriter, r *http.Request, user identity.User, current identity.Session) {
	if err := s.accounts.RevokeSession(current.ID, func(string) bool { return true }, identity.AuditContext{Actor: s.accountActor(user)}); err != nil && !errors.Is(err, identity.ErrNotFound) {
		s.writeAccountError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// doRevokeAll signs out other devices and preserves this session.
func (s *Server) doRevokeAll(w http.ResponseWriter, r *http.Request, user identity.User, current identity.Session) {
	if !emptyBody(w, r) {
		return
	}
	if err := s.accounts.RevokeOtherSessions(user.Username, current.ID, identity.AuditContext{Actor: s.accountActor(user)}); err != nil {
		s.writeAccountError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// doRevokeSession thu hồi một phiên — chỉ của chính caller; revoke phiên
// đang dùng thì xoá luôn cookie.
func (s *Server) doRevokeSession(w http.ResponseWriter, r *http.Request, user identity.User, current identity.Session, id string) {
	if !emptyBody(w, r) {
		return
	}
	err := s.accounts.RevokeSession(id, func(owner string) bool { return owner == user.Username }, identity.AuditContext{Actor: s.accountActor(user)})
	if err != nil {
		s.writeAccountError(w, err)
		return
	}
	if id == current.ID {
		s.clearSession(w, r)
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// doPasswordChange đổi mật khẩu tự phục vụ. Thành công thu hồi mọi phiên
// của tài khoản (hợp đồng kho chung, kể cả phiên này), nên response xoá
// cookie và yêu cầu đăng nhập lại — gọi lại /api/me ở đây sẽ biến một đổi
// mật khẩu thành công thành 401 giả.
func (s *Server) doPasswordChange(w http.ResponseWriter, r *http.Request, user identity.User) {
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !DecodeJSON(w, r, &req) {
		return
	}
	key := "password:" + user.Username
	if wait := s.limiter.Wait(key); wait > 0 {
		tooMany(w, wait)
		return
	}
	if err := s.accounts.ChangePassword(user.Username, req.Current, req.New, identity.AuditContext{Actor: s.accountActor(user)}); err != nil {
		if errors.Is(err, identity.ErrForbidden) {
			s.limiter.Fail(key)
			Error(w, http.StatusForbidden, "Mật khẩu hiện tại không đúng")
			return
		}
		s.writeAccountError(w, err)
		return
	}
	s.limiter.Succeed(key)
	s.clearSession(w, r)
	s.clearDeviceCookie(w, r)
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true, "reauthenticate": true})
}

func (s *Server) listSessions(w http.ResponseWriter, username, currentID string) {
	sessions, err := s.accounts.Sessions(username, currentID)
	if err != nil {
		s.writeAccountError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"sessions": identity.SessionList(sessions)})
}

// emptyBody đọc và bỏ body {} của một POST không tham số, giữ convention
// "mọi POST đều là JSON".
func emptyBody(w http.ResponseWriter, r *http.Request) bool {
	var v struct{}
	return DecodeJSON(w, r, &v)
}

func (s *Server) accountActor(user identity.User) *identity.AuditActor {
	return &identity.AuditActor{Username: user.Username, Name: user.Name, App: s.opts.App}
}

// ── setup lần đầu ───────────────────────────────────────────────────────────

type setupStatusResponse struct {
	Needed       bool       `json:"needed"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	Expired      bool       `json:"expired,omitempty"`
	AttemptsLeft int        `json:"attemptsLeft"`
	LockedUntil  *time.Time `json:"lockedUntil,omitempty"`
}

// serveSetup xử /api/setup, /api/setup/verify, /api/setup/admin; false = không thuộc nó quản.
func (s *Server) serveSetup(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/api/setup":
		if r.Method != http.MethodGet {
			Error(w, http.StatusNotFound, "không tìm thấy")
			return true
		}
		s.setupStatus(w)
	case "/api/setup/verify":
		if r.Method != http.MethodPost {
			Error(w, http.StatusNotFound, "không tìm thấy")
			return true
		}
		s.setupVerify(w, r)
	case "/api/setup/admin":
		if r.Method != http.MethodPost {
			Error(w, http.StatusNotFound, "không tìm thấy")
			return true
		}
		s.setupAdmin(w, r)
	default:
		return false
	}
	return true
}

func (s *Server) setupStatus(w http.ResponseWriter) {
	_, has, err := s.accounts.FirstUserCreatedAt()
	if err != nil {
		// Kho lỗi không được giả thành "trống": trả lỗi thật (503 khi mất
		// kết nối), không phát mã hay hiện setup trên dữ liệu không đọc được.
		s.writeAccountError(w, err)
		return
	}
	if has {
		WriteJSON(w, http.StatusOK, setupStatusResponse{Needed: false})
		return
	}
	state := s.setup
	if state == nil {
		Error(w, http.StatusInternalServerError, s.setupNoCode())
		return
	}
	st := state.Status()
	resp := setupStatusResponse{Needed: true, AttemptsLeft: st.AttemptsLeft, Expired: st.Expired}
	if !st.ExpiresAt.IsZero() {
		resp.ExpiresAt = &st.ExpiresAt
	}
	if !st.LockedUntil.IsZero() {
		resp.LockedUntil = &st.LockedUntil
	}
	WriteJSON(w, http.StatusOK, resp)
}

// setupTaken ghi 409 {at} khi đã có tài khoản; false = chưa có tài khoản.
func (s *Server) setupTaken(w http.ResponseWriter) bool {
	at, has, err := s.accounts.FirstUserCreatedAt()
	if err != nil {
		s.writeAccountError(w, err)
		return true
	}
	if !has {
		return false
	}
	WriteJSON(w, http.StatusConflict, map[string]any{"error": s.opts.AppName + " đã có quản trị viên", "at": at})
	return true
}

// setupNoCode là lỗi khi store trống mà không có setup state (khởi tạo mã lỗi).
func (s *Server) setupNoCode() string {
	return "Chưa tạo được mã thiết lập — khởi động lại " + s.opts.AppName
}

func (s *Server) setupVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if !DecodeJSON(w, r, &req) {
		return
	}
	if s.setupTaken(w) {
		return
	}
	state := s.setup
	if state == nil {
		Error(w, http.StatusInternalServerError, s.setupNoCode())
		return
	}
	res, err := state.Verify(req.Code)
	switch {
	case err == nil:
		WriteJSON(w, http.StatusOK, map[string]any{"setupToken": res.Token, "expiresAt": res.ExpiresAt})
	case errors.Is(err, identity.ErrSetupExpired):
		Error(w, http.StatusGone, err.Error())
	case errors.Is(err, identity.ErrSetupLocked):
		WriteJSON(w, http.StatusLocked, map[string]any{"error": err.Error(), "lockedUntil": res.LockedUntil})
	case errors.Is(err, identity.ErrSetupFormat):
		WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "format": true})
	case errors.Is(err, identity.ErrSetupWrong):
		WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "attemptsLeft": res.AttemptsLeft})
	default:
		Error(w, http.StatusInternalServerError, "Không kiểm được mã thiết lập")
	}
}

func (s *Server) setupAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SetupToken string `json:"setupToken"`
		Name       string `json:"name"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	}
	if !DecodeJSON(w, r, &req) {
		return
	}
	if s.setupTaken(w) {
		return
	}
	state := s.setup
	if state == nil {
		Error(w, http.StatusInternalServerError, s.setupNoCode())
		return
	}
	// token chỉ kiểm không tiêu: tạo thất bại thì form còn thử lại được.
	if err := state.CheckToken(req.SetupToken); err != nil {
		Error(w, http.StatusGone, err.Error())
		return
	}
	if field, msg := identity.ValidateSetupAdmin(req.Name, req.Username, req.Password); field != "" {
		WriteJSON(w, http.StatusBadRequest, map[string]any{"error": msg, "field": field})
		return
	}
	ns, err := s.accounts.CreateFirstAdmin(
		identity.NewUser{Username: req.Username, Name: req.Name, Password: req.Password},
		identity.DeviceLabel(r.UserAgent()), webguard.ClientIP(r),
	)
	if err != nil {
		if errors.Is(err, identity.ErrExists) {
			// Kho chung quyết thắng-thua: app này thua race setup → token
			// cục bộ vô hiệu ngay (UI rời màn setup), không tạo thêm user,
			// session hay recovery mồ côi ở phía thua.
			state.Invalidate()
			if !s.setupTaken(w) {
				Error(w, http.StatusConflict, s.opts.AppName+" đã có quản trị viên")
			}
			return
		}
		s.writeAccountError(w, err)
		return
	}
	// Tạo thành công mới vô hiệu mã và token (cả hai dùng một lần).
	state.Invalidate()
	var recoveryCode *string
	if code, _, err := s.accounts.IssueRecovery(ns.User.Username, false); err != nil {
		// User đã commit: không rollback giả và không bắt setup lại. Trả
		// phiên + recoveryCode=null để UI báo "đã tạo tài khoản nhưng chưa
		// có mã khôi phục"; admin đăng nhập lại và dùng luồng recovery có
		// sẵn của app quản trị để phát mã.
		slog.Error("phát mã khôi phục sau setup thất bại", "module", "webapp", "err", err)
	} else {
		recoveryCode = &code
	}
	s.setSession(w, r, ns)
	WriteJSON(w, http.StatusOK, map[string]any{"user": ns.User, "recoveryCode": recoveryCode})
}

// ── tiện ích HTTP ───────────────────────────────────────────────────────────

// WriteJSON ghi body JSON với status.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Error ghi lỗi {"error": msg} với status.
func Error(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]any{"error": msg})
}

// decodeBodyLimit là trần kích thước body JSON của mọi route (/ 1 MiB).
const decodeBodyLimit = 1 << 20

// DecodeJSON đọc body JSON (tối đa 1 MiB, cấm field lạ, không cho dữ liệu
// thừa sau giá trị đầu); false = đã ghi 400, không gọi tiếp service.
func DecodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	return DecodeJSONCode(w, r, into, "")
}

// DecodeJSONCode như DecodeJSON nhưng gắn code vào vỏ lỗi khi client sai body
// — route nào có mã lỗi ổn định (vd invalid_input) thì dùng bản này.
func DecodeJSONCode(w http.ResponseWriter, r *http.Request, into any, code string) bool {
	body := http.MaxBytesReader(w, r.Body, decodeBodyLimit)
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeDecodeError(w, code)
		return false
	}
	// Giá trị thứ hai phải là EOF: chặn {"a":1}{"a":2} và rác sau JSON, đồng
	// thời buộc reader giới hạn đọc hết phần đuôi trước khi vào service.
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		writeDecodeError(w, code)
		return false
	}
	return true
}

func writeDecodeError(w http.ResponseWriter, code string) {
	if code == "" {
		Error(w, http.StatusBadRequest, "body không phải JSON hợp lệ")
		return
	}
	WriteJSON(w, http.StatusBadRequest, map[string]any{
		"error": "body không phải JSON hợp lệ", "code": code})
}

func tooMany(w http.ResponseWriter, wait time.Duration) {
	seconds := int(wait.Seconds() + 0.5)
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	WriteJSON(w, http.StatusTooManyRequests, map[string]any{
		"error": "Thử sai quá nhiều lần, vui lòng đợi rồi thử lại", "retryAfter": seconds})
}

// errUnavailableMessage là body duy nhất của 503 phụ thuộc — kho tạm không
// sẵn sàng không phải lỗi client, không bao giờ xoá cookie hay giả thành 401.
const errUnavailableMessage = "Máy chủ tạm thời không sẵn sàng, vui lòng thử lại sau."

// errInternalMessage là body duy nhất của một 500 từ boundary này.
const errInternalMessage = "Máy chủ gặp lỗi, vui lòng thử lại sau."

// accountStatus ánh lỗi identity sang status ổn định; không echo nguyên nhân
// repository — chỉ trả text sentinel đã duyệt.
func accountStatus(err error) int {
	switch {
	case errors.Is(err, identity.ErrUnauthenticated):
		return http.StatusUnauthorized
	case errors.Is(err, identity.ErrInvalidCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, identity.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, identity.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, identity.ErrExists):
		return http.StatusConflict
	case errors.Is(err, identity.ErrLastAdmin):
		return http.StatusConflict
	case errors.Is(err, identity.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, identity.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, identity.ErrUnsupported):
		return http.StatusNotImplemented
	case dependencyUnavailable(err):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// dependencyUnavailable báo err là hỏng kho/mạng chứ không phải quyết định
// nghiệp vụ. Lỗi repository đã được gỡ credential trước khi tới boundary này
// (pool identity), nên khớp theo kiểu an toàn; text thô không rời log server.
func dependencyUnavailable(err error) bool {
	var pgErr *pgconn.PgError
	var connErr *pgconn.ConnectError
	var netErr net.Error
	return errors.As(err, &pgErr) || errors.As(err, &connErr) || errors.As(err, &netErr) ||
		errors.Is(err, puddle.ErrClosedPool)
}

// writeAccountError ghi lỗi tài khoản/phiên theo map chung: sentinel giữ
// status/envelope, lỗi bất ngờ thành 500 generic với nguyên nhân chỉ log
// phía server. Không bao giờ echo DSN, token hay credential vào response.
func (s *Server) writeAccountError(w http.ResponseWriter, err error) {
	switch status := accountStatus(err); {
	case status == http.StatusServiceUnavailable:
		Error(w, status, errUnavailableMessage)
	case status == http.StatusInternalServerError:
		slog.Error("yêu cầu tài khoản thất bại", "module", "webapp", "err", err)
		Error(w, status, errInternalMessage)
	default:
		Error(w, status, err.Error())
	}
}
