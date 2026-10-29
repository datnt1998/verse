package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/datnt1998/verse/platform/identity"
)

// GAP-41：首次运行在网页上创建管理员（A0 · Setup.dc.html）。
// 三个路由都在鉴权之前（与 /api/login 同层）：还没有账号时也要能访问。
// 设置码只出现在 serve 的启动输出里，任何响应、日志都不含明文。

const (
	setupTakenMessage = "Noverse đã có quản trị viên"
	setupNoCode       = "Chưa tạo được mã thiết lập — khởi động lại Noverse"
)

// setupStatusResponse 是 GET /api/setup 的响应；已不需要时只有 needed=false。
// version 只在 needed=true 时出现（A0 页脚 "· Noverse 0.9.3"）：已有管理员时不再向
// 未登录请求泄露版本号。Câu 1: mã hết hạn vẫn giữ expiresAt cũ và thêm expired=true.
type setupStatusResponse struct {
	Needed       bool       `json:"needed"`
	Version      string     `json:"version,omitempty"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	Expired      bool       `json:"expired,omitempty"`
	AttemptsLeft int        `json:"attemptsLeft"`
	LockedUntil  *time.Time `json:"lockedUntil,omitempty"`
}

// SetupCodeOnce 返回首次设置码的明文，只给 serve 在启动时打印一次；随后从内存清除。
func (g *Gateway) SetupCodeOnce() string {
	g.setupMu.Lock()
	defer g.setupMu.Unlock()
	code := g.setupCode
	g.setupCode = ""
	return code
}

// serveSetup 处理 /api/setup、/api/setup/verify、/api/setup/admin；返回 false 表示路径不归它管。
func (g *Gateway) serveSetup(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/api/setup":
		if r.Method != http.MethodGet {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return true
		}
		g.setupStatus(w)
	case "/api/setup/verify":
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return true
		}
		g.setupVerify(w, r)
	case "/api/setup/admin":
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return true
		}
		g.setupAdmin(w, r)
	default:
		return false
	}
	return true
}

func (g *Gateway) setupStatus(w http.ResponseWriter) {
	_, has, err := g.accounts.FirstUserCreatedAt()
	if err != nil {
		accountError(w, err)
		return
	}
	if has {
		writeJSON(w, http.StatusOK, setupStatusResponse{Needed: false})
		return
	}
	state := g.setup
	if state == nil {
		jsonError(w, http.StatusInternalServerError, setupNoCode)
		return
	}
	st := state.Status()
	resp := setupStatusResponse{
		Needed:       true,
		Version:      g.build.Version,
		AttemptsLeft: st.AttemptsLeft,
		Expired:      st.Expired,
	}
	if !st.ExpiresAt.IsZero() {
		resp.ExpiresAt = &st.ExpiresAt
	}
	if !st.LockedUntil.IsZero() {
		resp.LockedUntil = &st.LockedUntil
	}
	writeJSON(w, http.StatusOK, resp)
}

// setupTaken 在已经存在账号时写 409 {at}（at = 最早的账号创建时间）；返回 false 表示还没有账号。
func (g *Gateway) setupTaken(w http.ResponseWriter) bool {
	at, has, err := g.accounts.FirstUserCreatedAt()
	if err != nil {
		accountError(w, err)
		return true
	}
	if !has {
		return false
	}
	writeJSON(w, http.StatusConflict, map[string]any{"error": setupTakenMessage, "at": at})
	return true
}

func (g *Gateway) setupVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if g.setupTaken(w) {
		return
	}
	state := g.setup
	if state == nil {
		jsonError(w, http.StatusInternalServerError, setupNoCode)
		return
	}
	res, err := state.Verify(req.Code)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"setupToken": res.Token, "expiresAt": res.ExpiresAt})
	case errors.Is(err, identity.ErrSetupExpired):
		jsonError(w, http.StatusGone, err.Error())
	case errors.Is(err, identity.ErrSetupLocked):
		writeJSON(w, http.StatusLocked, map[string]any{"error": err.Error(), "lockedUntil": res.LockedUntil})
	case errors.Is(err, identity.ErrSetupFormat):
		// Câu 1: mã sai định dạng trả 400 {format:true} và không tính vào 5 lần thử.
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "format": true})
	case errors.Is(err, identity.ErrSetupWrong):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "attemptsLeft": res.AttemptsLeft})
	default:
		jsonError(w, http.StatusInternalServerError, "Không kiểm được mã thiết lập")
	}
}

func (g *Gateway) setupAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SetupToken string `json:"setupToken"`
		Name       string `json:"name"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if g.setupTaken(w) {
		return
	}
	state := g.setup
	if state == nil {
		jsonError(w, http.StatusInternalServerError, setupNoCode)
		return
	}
	// token 只校验不消费：创建失败时表单还能重试（token 与码同寿）。
	if err := state.CheckToken(req.SetupToken); err != nil {
		jsonError(w, http.StatusGone, err.Error())
		return
	}
	if field, msg := identity.ValidateSetupAdmin(req.Name, req.Username, req.Password); field != "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": msg, "field": field})
		return
	}
	s, err := g.accounts.CreateFirstAdmin(
		identity.NewUser{Username: req.Username, Name: req.Name, Password: req.Password},
		identity.DeviceLabel(r.UserAgent()), clientIP(r),
	)
	if err != nil {
		if errors.Is(err, identity.ErrExists) {
			// 并发创建：另一个人先成功 → 这里 409 {at}。
			if !g.setupTaken(w) {
				jsonError(w, http.StatusConflict, setupTakenMessage)
			}
			return
		}
		accountError(w, err)
		return
	}
	// 创建成功才作废码与 token（两者都是一次性）。
	state.Invalidate()
	code, _, err := g.accounts.IssueRecovery(s.User.Username, false)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	setSession(w, r, s)
	g.accountChanged(s.User.Username)
	g.notifyLogin(r, s)
	g.writeAccountUser(w, http.StatusOK, s.User, map[string]any{"recoveryCode": code})
}
