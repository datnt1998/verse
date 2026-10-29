// Package webapp là lớp HTTP dùng chung của các app trong repo: SPA nhúng,
// các route /api nền (setup lần đầu, login, me, logout) sau session cookie và
// guard origin của platform/webguard. App chỉ định danh riêng qua Options và
// đăng ký route nghiệp vụ bằng Handle.
package webapp

import (
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webguard"
)

// Options là định danh riêng của từng app mà Server cần.
type Options struct {
	// AppName đi vào các lỗi hiển thị cho người dùng trong luồng thiết lập
	// (vd "Viderse đã có quản trị viên").
	AppName string
	// CookieName là tên session cookie — phải duy nhất giữa các app vì cookie
	// scope theo host:port chung trên máy dev.
	CookieName string
	// UI là gốc SPA đã build (index.html + assets/ ở root). Caller thường
	// truyền fs.Sub(embed, "dist").
	UI fs.FS
}

// HandlerFunc là route nghiệp vụ của app: chỉ chạy sau khi session cookie đã
// xác thực; user là tài khoản hiện tại từ Store.Authenticate.
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

// SetProxyConfig cài ranh giới trusted-proxy đọc từ env lúc khởi động.
func (s *Server) SetProxyConfig(cfg *webguard.Config) { s.proxy = cfg }

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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Chuẩn hoá trusted-proxy phải đi trước mọi phán đoán bảo mật (khoá giới hạn, Secure cookie, origin CSRF).
	r = s.proxy.Normalize(r)
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
	if webguard.JSONBodyRequired(r) && !webguard.JSONContent(r) {
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
	cookie, err := r.Cookie(s.opts.CookieName)
	if err != nil {
		Error(w, http.StatusUnauthorized, identity.ErrUnauthenticated.Error())
		return
	}
	user, current, err := s.accounts.Authenticate(cookie.Value)
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			s.clearSession(w, r)
		}
		Error(w, http.StatusUnauthorized, identity.ErrUnauthenticated.Error())
		return
	}
	switch {
	case r.URL.Path == "/api/logout" && r.Method == http.MethodPost:
		s.logout(w, r, current)
	case r.URL.Path == "/api/me" && r.Method == http.MethodGet:
		WriteJSON(w, http.StatusOK, map[string]any{"user": user.View()})
	default:
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

func (s *Server) setSession(w http.ResponseWriter, r *http.Request, ns identity.NewSession) {
	http.SetCookie(w, &http.Cookie{
		Name: s.opts.CookieName, Value: ns.Cookie, Path: "/",
		Expires: ns.ExpiresAt, MaxAge: int(time.Until(ns.ExpiresAt).Seconds()),
		HttpOnly: true, Secure: webguard.SecureRequest(r), SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: s.opts.CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: webguard.SecureRequest(r), SameSite: http.SameSiteStrictMode,
	})
}

// ── login / logout ──────────────────────────────────────────────────────────

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
	keys := []string{"ip:" + webguard.ClientIP(r)}
	if identity.ValidUsername(username) {
		keys = append(keys, "user:"+username)
	}
	if wait := s.limiter.Wait(keys...); wait > 0 {
		tooMany(w, wait)
		return
	}
	ns, err := s.accounts.Login(username, req.Password, identity.DeviceLabel(r.UserAgent()), webguard.ClientIP(r))
	if err != nil {
		if errors.Is(err, identity.ErrInvalidCredentials) {
			s.limiter.Fail(keys...)
		}
		Error(w, accountStatus(err), err.Error())
		return
	}
	s.limiter.Succeed(keys...)
	s.setSession(w, r, ns)
	WriteJSON(w, http.StatusOK, map[string]any{"user": ns.User})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, current identity.Session) {
	if err := s.accounts.RevokeSession(current.ID, func(string) bool { return true }); err != nil && !errors.Is(err, identity.ErrNotFound) {
		Error(w, accountStatus(err), err.Error())
		return
	}
	s.clearSession(w, r)
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
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
		Error(w, accountStatus(err), err.Error())
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
		Error(w, accountStatus(err), err.Error())
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
			if !s.setupTaken(w) {
				Error(w, http.StatusConflict, s.opts.AppName+" đã có quản trị viên")
			}
			return
		}
		Error(w, accountStatus(err), err.Error())
		return
	}
	// Tạo thành công mới vô hiệu mã và token (cả hai dùng một lần).
	state.Invalidate()
	code, _, err := s.accounts.IssueRecovery(ns.User.Username, false)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setSession(w, r, ns)
	WriteJSON(w, http.StatusOK, map[string]any{"user": ns.User, "recoveryCode": code})
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

// DecodeJSON đọc body JSON (tối đa 1 MiB, cấm field lạ); false = đã ghi 400.
func DecodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		Error(w, http.StatusBadRequest, "body không phải JSON hợp lệ")
		return false
	}
	return true
}

func tooMany(w http.ResponseWriter, wait time.Duration) {
	seconds := int(wait.Seconds() + 0.5)
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	WriteJSON(w, http.StatusTooManyRequests, map[string]any{
		"error": "Thử sai quá nhiều lần, vui lòng đợi rồi thử lại", "retryAfter": seconds})
}

// accountStatus ánh lỗi identity sang status ổn định; không echo nguyên nhân
// repository — chỉ trả text sentinel đã duyệt.
func accountStatus(err error) int {
	switch {
	case errors.Is(err, identity.ErrUnauthenticated):
		return http.StatusUnauthorized
	case errors.Is(err, identity.ErrInvalidCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, identity.ErrExists):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
