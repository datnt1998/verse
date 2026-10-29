// Package web là HTTP gateway của viderse serve: SPA nhúng, các route /api
// (setup lần đầu, login, me, runs, scripts) sau session cookie viderse_session
// và guard origin của platform/webguard.
package web

import (
	"context"
	"embed"
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

	"github.com/datnt1998/verse/apps/viderse/internal/agent"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/script"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webguard"
	"github.com/jackc/pgx/v5/pgtype"
)

//go:embed all:dist
var ui embed.FS

const sessionCookie = "viderse_session"

// StoreView là phần API phía runs/scripts mà gateway cần (gen.Queries thoả).
type StoreView interface {
	GetRun(ctx context.Context, id pgtype.UUID) (gen.AgentRun, error)
	ListRuns(ctx context.Context, limit int32) ([]gen.AgentRun, error)
	GetScript(ctx context.Context, id pgtype.UUID) (gen.Script, error)
	ListScripts(ctx context.Context, limit int32) ([]gen.Script, error)
}

// Submitter là cổng đưa brief vào hàng chạy (agent.Runner thoả).
type Submitter interface {
	Submit(ctx context.Context, brief script.Brief) (pgtype.UUID, error)
}

// Gateway phục vụ toàn bộ HTTP của app.
type Gateway struct {
	accounts *identity.Store
	queries  StoreView
	runner   Submitter
	proxy    *webguard.Config
	limiter  identity.LoginLimiter

	setup     *identity.SetupState
	setupCode string
	setupMu   sync.Mutex
}

// NewGateway dựng handler: accounts là kho identity file-backend; khi chưa có
// tài khoản nào, gateway tạo một setup state (chỉ giữ hash) — serve in mã
// plaintext một lần qua SetupCodeOnce.
func NewGateway(accounts *identity.Store, queries StoreView, runner Submitter) *Gateway {
	g := &Gateway{
		accounts: accounts,
		queries:  queries,
		runner:   runner,
		limiter:  accounts.LoginLimiter(),
	}
	if has, err := accounts.HasUsers(); err == nil && !has {
		if state, code, err := identity.NewSetupState(time.Now); err == nil {
			g.setup, g.setupCode = state, code
		}
	}
	return g
}

// SetProxyConfig cài ranh giới trusted-proxy đọc từ env lúc khởi động.
func (g *Gateway) SetProxyConfig(cfg *webguard.Config) { g.proxy = cfg }

// SetupCodeOnce trả mã thiết lập plaintext cho serve in đúng một lần; sau đó
// xoá khỏi bộ nhớ (chỉ còn hash).
func (g *Gateway) SetupCodeOnce() string {
	g.setupMu.Lock()
	defer g.setupMu.Unlock()
	code := g.setupCode
	g.setupCode = ""
	return code
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Chuẩn hoá trusted-proxy phải đi trước mọi phán đoán bảo mật (khoá giới hạn, Secure cookie, origin CSRF).
	r = g.proxy.Normalize(r)
	switch r.URL.Path {
	case "/healthz":
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	case "/readyz":
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if g.proxy.CrossSiteRequest(r) {
		jsonError(w, http.StatusForbidden, "yêu cầu khác origin bị từ chối")
		return
	}
	if r.URL.Path == "/api" || !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.URL.Path == "/api" {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		g.serveStatic(w, r)
		return
	}
	if webguard.JSONBodyRequired(r) && !webguard.JSONContent(r) {
		jsonError(w, http.StatusBadRequest, "Content-Type phải là application/json")
		return
	}
	switch {
	case r.URL.Path == "/api/login" && r.Method == http.MethodPost:
		g.login(w, r)
		return
	case g.serveSetup(w, r):
		// /api/setup* ngang hàng /api/login: chưa có tài khoản cũng phải vào được.
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, identity.ErrUnauthenticated.Error())
		return
	}
	user, current, err := g.accounts.Authenticate(cookie.Value)
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			clearSession(w, r)
		}
		jsonError(w, http.StatusUnauthorized, identity.ErrUnauthenticated.Error())
		return
	}
	_ = user // mọi route hiện tại chỉ cần đăng nhập; phân quyền để phase sau.
	switch {
	case r.URL.Path == "/api/logout" && r.Method == http.MethodPost:
		g.logout(w, r, current)
	case r.URL.Path == "/api/me" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"user": user.View()})
	case r.URL.Path == "/api/runs" && r.Method == http.MethodGet:
		g.listRuns(w, r)
	case r.URL.Path == "/api/runs" && r.Method == http.MethodPost:
		g.createRun(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/runs/") && r.Method == http.MethodGet:
		g.getRun(w, r)
	case r.URL.Path == "/api/scripts" && r.Method == http.MethodGet:
		g.listScripts(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/scripts/") && r.Method == http.MethodGet:
		g.getScript(w, r)
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// ── tĩnh (SPA nhúng) ────────────────────────────────────────────────────────

func (g *Gateway) serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	path := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+r.URL.Path)), "/")
	if path == "" {
		path = "index.html"
	}
	f, err := fs.Sub(ui, "dist")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	data, err := fs.ReadFile(f, path)
	if err != nil {
		// Tệp băm trong assets/ không có thì là 404 thật.
		if strings.HasPrefix(path, "assets/") {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		path = "index.html"
		data, err = fs.ReadFile(f, path)
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
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

func setSession(w http.ResponseWriter, r *http.Request, s identity.NewSession) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: s.Cookie, Path: "/",
		Expires: s.ExpiresAt, MaxAge: int(time.Until(s.ExpiresAt).Seconds()),
		HttpOnly: true, Secure: webguard.SecureRequest(r), SameSite: http.SameSiteStrictMode,
	})
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: webguard.SecureRequest(r), SameSite: http.SameSiteStrictMode,
	})
}

// ── login / logout ──────────────────────────────────────────────────────────

func (g *Gateway) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	// Username chữ thường: bucket limiter và tra cứu dùng chung một dạng.
	username := strings.ToLower(strings.TrimSpace(req.Username))
	keys := []string{"ip:" + webguard.ClientIP(r)}
	if identity.ValidUsername(username) {
		keys = append(keys, "user:"+username)
	}
	if wait := g.limiter.Wait(keys...); wait > 0 {
		tooMany(w, wait)
		return
	}
	s, err := g.accounts.Login(username, req.Password, identity.DeviceLabel(r.UserAgent()), webguard.ClientIP(r))
	if err != nil {
		if errors.Is(err, identity.ErrInvalidCredentials) {
			g.limiter.Fail(keys...)
		}
		accountError(w, err)
		return
	}
	g.limiter.Succeed(keys...)
	setSession(w, r, s)
	writeJSON(w, http.StatusOK, map[string]any{"user": s.User})
}

func (g *Gateway) logout(w http.ResponseWriter, r *http.Request, current identity.Session) {
	if err := g.accounts.RevokeSession(current.ID, func(string) bool { return true }); err != nil && !errors.Is(err, identity.ErrNotFound) {
		accountError(w, err)
		return
	}
	clearSession(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ── setup lần đầu ───────────────────────────────────────────────────────────

const (
	setupTakenMessage = "Viderse đã có quản trị viên"
	setupNoCode       = "Chưa tạo được mã thiết lập — khởi động lại Viderse"
)

type setupStatusResponse struct {
	Needed       bool       `json:"needed"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	Expired      bool       `json:"expired,omitempty"`
	AttemptsLeft int        `json:"attemptsLeft"`
	LockedUntil  *time.Time `json:"lockedUntil,omitempty"`
}

// serveSetup xử /api/setup, /api/setup/verify, /api/setup/admin; false = không thuộc nó quản.
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
	resp := setupStatusResponse{Needed: true, AttemptsLeft: st.AttemptsLeft, Expired: st.Expired}
	if !st.ExpiresAt.IsZero() {
		resp.ExpiresAt = &st.ExpiresAt
	}
	if !st.LockedUntil.IsZero() {
		resp.LockedUntil = &st.LockedUntil
	}
	writeJSON(w, http.StatusOK, resp)
}

// setupTaken ghi 409 {at} khi đã có tài khoản; false = chưa có tài khoản.
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
	// token chỉ kiểm không tiêu: tạo thất bại thì form còn thử lại được.
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
		identity.DeviceLabel(r.UserAgent()), webguard.ClientIP(r),
	)
	if err != nil {
		if errors.Is(err, identity.ErrExists) {
			if !g.setupTaken(w) {
				jsonError(w, http.StatusConflict, setupTakenMessage)
			}
			return
		}
		accountError(w, err)
		return
	}
	// Tạo thành công mới vô hiệu mã và token (cả hai dùng một lần).
	state.Invalidate()
	code, _, err := g.accounts.IssueRecovery(s.User.Username, false)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	setSession(w, r, s)
	writeJSON(w, http.StatusOK, map[string]any{"user": s.User, "recoveryCode": code})
}

// ── runs / scripts ──────────────────────────────────────────────────────────

func (g *Gateway) createRun(w http.ResponseWriter, r *http.Request) {
	var brief script.Brief
	if !decodeJSON(w, r, &brief) {
		return
	}
	id, err := g.runner.Submit(r.Context(), brief)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	run, err := g.queries.GetRun(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run": runWire(run)})
}

func (g *Gateway) listRuns(w http.ResponseWriter, r *http.Request) {
	rows, err := g.queries.ListRuns(r.Context(), 100)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, runWire(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}

func (g *Gateway) getRun(w http.ResponseWriter, r *http.Request) {
	id, err := agent.ParseUUID(strings.TrimPrefix(r.URL.Path, "/api/runs/"))
	if err != nil {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	row, err := g.queries.GetRun(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": runWire(row)})
}

func (g *Gateway) listScripts(w http.ResponseWriter, r *http.Request) {
	rows, err := g.queries.ListScripts(r.Context(), 100)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, scriptSummaryWire(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"scripts": out})
}

func (g *Gateway) getScript(w http.ResponseWriter, r *http.Request) {
	id, err := agent.ParseUUID(strings.TrimPrefix(r.URL.Path, "/api/scripts/"))
	if err != nil {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	row, err := g.queries.GetScript(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	var sections []script.Section
	if err := json.Unmarshal(row.Sections, &sections); err != nil {
		jsonError(w, http.StatusInternalServerError, "sections hỏng")
		return
	}
	wire := scriptSummaryWire(row)
	wire["sections"] = sections
	writeJSON(w, http.StatusOK, map[string]any{"script": wire})
}

func runWire(row gen.AgentRun) map[string]any {
	out := map[string]any{
		"id":            agent.UUIDString(row.ID),
		"status":        row.Status,
		"provider":      row.Provider,
		"model":         row.Model,
		"input_tokens":  row.InputTokens,
		"output_tokens": row.OutputTokens,
		"created_at":    row.CreatedAt.Time,
	}
	var brief script.Brief
	if err := json.Unmarshal(row.Brief, &brief); err == nil {
		out["brief"] = brief
	}
	if row.CostUsd.Valid {
		out["cost_usd"] = row.CostUsd.Float64
	}
	if row.Error.Valid {
		out["error"] = row.Error.String
	}
	if row.StartedAt.Valid {
		out["started_at"] = row.StartedAt.Time
	}
	if row.FinishedAt.Valid {
		out["finished_at"] = row.FinishedAt.Time
	}
	return out
}

func scriptSummaryWire(row gen.Script) map[string]any {
	return map[string]any{
		"id":               agent.UUIDString(row.ID),
		"run_id":           agent.UUIDString(row.RunID),
		"title":            row.Title,
		"full_text":        row.FullText,
		"word_count":       row.WordCount,
		"est_duration_sec": row.EstDurationSec,
		"created_at":       row.CreatedAt.Time,
	}
}

// ── tiện ích HTTP ───────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		jsonError(w, http.StatusBadRequest, "body không phải JSON hợp lệ")
		return false
	}
	return true
}

func tooMany(w http.ResponseWriter, wait time.Duration) {
	seconds := int(wait.Seconds() + 0.5)
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"error": "Thử sai quá nhiều lần, vui lòng đợi rồi thử lại", "retryAfter": seconds})
}

// accountError ánh lỗi identity sang status ổn định; không echo nguyên nhân
// repository — chỉ trả text sentinel đã duyệt.
func accountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrUnauthenticated):
		jsonError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, identity.ErrInvalidCredentials):
		jsonError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, identity.ErrExists):
		jsonError(w, http.StatusConflict, err.Error())
	default:
		jsonError(w, http.StatusBadRequest, err.Error())
	}
}
