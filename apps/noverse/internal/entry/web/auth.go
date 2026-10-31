package web

import (
	"errors"
	"math"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webguard"
)

const sessionCookie = "ainovel_session"

type accountUserView struct {
	identity.UserView
	LastSeenAt *time.Time `json:"lastSeenAt"`
}

func (g *Gateway) userWithLastSeen(user identity.UserView) (accountUserView, error) {
	view := accountUserView{UserView: user}
	sessions, err := g.accounts.Sessions(user.Username, "")
	if err != nil {
		return view, err
	}
	for _, session := range sessions {
		if view.LastSeenAt == nil || session.LastSeen.After(*view.LastSeenAt) {
			seen := session.LastSeen
			view.LastSeenAt = &seen
		}
	}
	return view, nil
}

// A nil envelope returns a bare user; otherwise fill its user field.
func (g *Gateway) writeAccountUser(w http.ResponseWriter, status int, user identity.UserView, envelope map[string]any) {
	view, err := g.userWithLastSeen(user)
	if err != nil {
		accountError(w, err)
		return
	}
	if envelope == nil {
		writeJSON(w, status, view)
		return
	}
	envelope["user"] = view
	writeJSON(w, status, envelope)
}

// clientIP 返回可信的客户端地址：经 ServeHTTP 归一化的请求带 platform/webguard
// 边界算出的地址；直接调用 handler（测试/内部）时退回对端地址。
func clientIP(r *http.Request) string {
	return webguard.ClientIP(r)
}

// secureRequest 判断浏览器看到的是否 HTTPS：直连 TLS，或 TRUSTED_PROXIES 内的对端
// 转发的 X-Forwarded-Proto（未配置 TRUSTED_PROXIES 时不信任任何对端，loopback 也不例外）。
// 远端地址的自报头一律忽略，否则伪造头可以把 Secure cookie 降级；规则实现在
// platform/webguard。
func secureRequest(r *http.Request) bool {
	return webguard.SecureRequest(r)
}

func setSession(w http.ResponseWriter, r *http.Request, s identity.NewSession) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: s.Cookie, Path: "/", Expires: s.ExpiresAt, MaxAge: int(time.Until(s.ExpiresAt).Seconds()), HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteStrictMode})
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteStrictMode})
}

// accountError maps account/session errors with the shared policy and never
// echoes a repository cause: repository wrappers (unique constraints, SQLSTATE,
// table names) stay server-side; only the curated sentinel text is returned.
func accountError(w http.ResponseWriter, err error) {
	jsonMappedError(w, err)
}

func tooMany(w http.ResponseWriter, wait time.Duration) {
	seconds := int(math.Ceil(wait.Seconds()))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "Thử sai quá nhiều lần, vui lòng đợi rồi thử lại", "retryAfter": seconds})
}

func (g *Gateway) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	// Usernames are lowercase and Postgres matches them case-insensitively, so
	// the account bucket and the audit target use the folded spelling: changing
	// letter case must not open a fresh bucket for the same account.
	username := strings.ToLower(strings.TrimSpace(req.Username))
	keys := []string{"ip:" + clientIP(r)}
	if identity.ValidUsername(username) {
		keys = append(keys, "user:"+username)
	}
	if wait := g.limiter.Wait(keys...); wait > 0 {
		tooMany(w, wait)
		return
	}
	s, err := g.accounts.Login(req.Username, req.Password, identity.DeviceLabel(r.UserAgent()), clientIP(r))
	if err != nil {
		if errors.Is(err, identity.ErrInvalidCredentials) {
			g.limiter.Fail(keys...)
			if len(keys) > 1 && g.limiter.Wait(keys[1]) > 0 {
				if auditErr := g.accounts.RecordSecurityFailure(username); auditErr != nil {
					accountError(w, auditErr)
					return
				}
			}
		}
		accountError(w, err)
		return
	}
	g.limiter.Succeed(keys...)
	setSession(w, r, s)
	g.accountChanged(s.User.Username)
	g.notifyLogin(r, s)
	g.writeAccountUser(w, http.StatusOK, s.User, map[string]any{})
}

func (g *Gateway) pair(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	key := "pair:" + clientIP(r)
	if wait := g.limiter.Wait(key); wait > 0 {
		tooMany(w, wait)
		return
	}
	s, err := g.accounts.Pair(req.Code, identity.DeviceLabel(r.UserAgent()), clientIP(r))
	if err != nil {
		if errors.Is(err, identity.ErrInvalidPairing) {
			g.limiter.Fail(key)
		}
		accountError(w, err)
		return
	}
	g.limiter.Succeed(key)
	setSession(w, r, s)
	g.accountChanged(s.User.Username)
	g.notifyLogin(r, s)
	g.writeAccountUser(w, http.StatusOK, s.User, map[string]any{})
}

type pairingLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	SVG   string `json:"svg"`
}

// pairingBases 返回手机可达的地址：serve 公布的局域网/Tailscale 地址，外加当前请求的非回环地址。
func (g *Gateway) pairingBases(r *http.Request) []string {
	bases := slices.Clone(g.PublicURLs)
	if g.network != nil {
		for _, u := range g.network.URLs() {
			if !slices.Contains(bases, u) {
				bases = append(bases, u)
			}
		}
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if host != "" && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		scheme := "http"
		if secureRequest(r) {
			scheme = "https"
		}
		current := scheme + "://" + r.Host
		bases = append([]string{current}, slices.DeleteFunc(bases, func(b string) bool { return b == current })...)
	}
	return bases
}

// pairingLinks 生成一次性配对码，并为每个手机可达的地址构造二维码链接（createPairing 与 PUT /api/system/network 共用）。
func (g *Gateway) pairingLinks(r *http.Request, username, createdBy string) (string, time.Time, []pairingLink, error) {
	code, expires, err := g.accounts.NewPairing(username, createdBy)
	if err != nil {
		return "", time.Time{}, nil, err
	}
	links := []pairingLink{}
	for _, base := range g.pairingBases(r) {
		link := strings.TrimRight(base, "/") + pairShortPath + code
		svg, err := identity.QRSVG(link)
		if err != nil {
			return "", time.Time{}, nil, err
		}
		label := base
		if u, err := url.Parse(base); err == nil {
			label = u.Host
		}
		links = append(links, pairingLink{Label: label, URL: link, SVG: svg})
	}
	return code, expires, links, nil
}

// pairShortPath 是二维码里的短地址前缀：链接越短，二维码版本越低、模块越大，手机越容易扫。
const pairShortPath = "/p/"

// pairRedirect 把 GET /p/<code> 重定向到前端的配对页 /#/pair/<code>；旧链接 /#/pair/<code> 仍有效。
func pairRedirect(w http.ResponseWriter, r *http.Request) bool {
	code, ok := strings.CutPrefix(r.URL.Path, pairShortPath)
	if !ok || code == "" || strings.Trim(code, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_") != "" {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/#/pair/"+code, http.StatusFound)
	return true
}

func (g *Gateway) createPairing(w http.ResponseWriter, r *http.Request, username, createdBy string) {
	code, expires, links, err := g.pairingLinks(r, username, createdBy)
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": code, "username": username, "expiresAt": expires, "links": links})
}

// serveAccount 处理账号相关接口；返回 false 表示路径不归它管。
func (g *Gateway) serveAccount(w http.ResponseWriter, r *http.Request, user identity.User, current identity.Session) bool {
	path, get, post := r.URL.Path, r.Method == http.MethodGet, r.Method == http.MethodPost
	switch {
	case path == "/api/logout" && post:
		if err := g.accounts.RevokeSession(current.ID, func(string) bool { return true }, identity.AuditContext{Actor: accountActor(user)}); err != nil && !errors.Is(err, identity.ErrNotFound) {
			accountError(w, err)
			return true
		}
		clearSession(w, r)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case path == "/api/me" && get:
		extra := map[string]any{"session": current}
		if isAdmin(user) {
			count, err := g.pendingLessons(user)
			if err != nil {
				jsonError(w, http.StatusInternalServerError, "Không đọc được bài học: "+err.Error())
				return true
			}
			extra["pendingLessons"] = count
		}
		g.writeAccountUser(w, http.StatusOK, user.View(), extra)
	case path == "/api/me" && (post || r.Method == http.MethodPut):
		var req struct {
			Name string `json:"name"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		updated, err := g.accounts.UpdateUser(user.Username, identity.UserPatch{Name: &req.Name, Actor: accountActor(user)})
		if err != nil {
			accountError(w, err)
			return true
		}
		g.writeAccountUser(w, http.StatusOK, updated, map[string]any{})
	case path == "/api/me/onboarded" && post:
		if !g.emptyBody(w, r) {
			return true
		}
		updated, err := g.accounts.MarkOnboarded(user.Username)
		if err != nil {
			accountError(w, err)
			return true
		}
		g.writeAccountUser(w, http.StatusOK, updated, map[string]any{})
	case path == "/api/me/password" && post:
		var req struct {
			Current string `json:"current"`
			New     string `json:"new"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		key := "password:" + user.Username
		if wait := g.limiter.Wait(key); wait > 0 {
			tooMany(w, wait)
			return true
		}
		if err := g.accounts.ChangePassword(user.Username, req.Current, req.New); err != nil {
			if errors.Is(err, identity.ErrForbidden) {
				g.limiter.Fail(key)
				jsonError(w, http.StatusForbidden, "Mật khẩu hiện tại không đúng")
				return true
			}
			accountError(w, err)
			return true
		}
		g.limiter.Succeed(key)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case path == "/api/me/sessions" && get:
		g.listSessions(w, user.Username, current.ID)
	case path == "/api/me/sessions/revoke-all" && post:
		if !g.emptyBody(w, r) {
			return true
		}
		if err := g.accounts.RevokeAll(user.Username, identity.AuditContext{Actor: accountActor(user)}); err != nil {
			accountError(w, err)
			return true
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	case path == "/api/me/pairings" && post:
		if !g.emptyBody(w, r) {
			return true
		}
		g.createPairing(w, r, user.Username, user.Username)
	case strings.HasPrefix(path, "/api/sessions/") && strings.HasSuffix(path, "/revoke") && post:
		if !g.emptyBody(w, r) {
			return true
		}
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/sessions/"), "/revoke")
		err := g.accounts.RevokeSession(id, func(owner string) bool { return owner == user.Username || user.Role == identity.RoleAdmin }, identity.AuditContext{Actor: accountActor(user)})
		if err != nil {
			accountError(w, err)
			return true
		}
		if id == current.ID {
			clearSession(w, r)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case path == "/api/admin/audit":
		if !isAdmin(user) {
			jsonError(w, 403, "Chỉ quản trị viên được xem nhật ký quản trị.")
			return true
		}
		if !get {
			jsonError(w, 405, "Nhật ký quản trị chỉ được đọc.")
			return true
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if r.URL.Query().Has("limit") && (err != nil || limit < 1) {
			jsonError(w, 400, "Số dòng nhật ký không hợp lệ.")
			return true
		}
		page, err := g.accounts.Audit(identity.AuditQuery{Type: r.URL.Query().Get("type"), Actor: r.URL.Query().Get("actor"), Before: r.URL.Query().Get("before"), Limit: limit})
		if err != nil {
			accountError(w, err)
			return true
		}
		writeJSON(w, 200, page)
	case memberRoute(path) != "":
		if !g.serveProjectMembers(w, r, user) {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
		}
	case path == "/api/users" || strings.HasPrefix(path, "/api/users/"):
		if user.Role != identity.RoleAdmin {
			jsonError(w, http.StatusForbidden, "Không có quyền")
			return true
		}
		g.serveUsers(w, r, user, current)
	default:
		return false
	}
	return true
}

// emptyBody 读取并丢弃 {} 请求体，保持"所有 POST 都是 JSON"的约定。
func (g *Gateway) emptyBody(w http.ResponseWriter, r *http.Request) bool {
	var v struct{}
	return decodeJSON(w, r, &v)
}

func (g *Gateway) listSessions(w http.ResponseWriter, username, currentID string) {
	sessions, err := g.accounts.Sessions(username, currentID)
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (g *Gateway) serveUsers(w http.ResponseWriter, r *http.Request, admin identity.User, current identity.Session) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/users"), "/")
	name, action, _ := strings.Cut(rest, "/")
	get, post := r.Method == http.MethodGet, r.Method == http.MethodPost
	switch {
	case name == "" && get:
		users, err := g.accounts.Users()
		if err != nil {
			accountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	case name == "" && post:
		var req identity.NewUser
		if !decodeJSON(w, r, &req) {
			return
		}
		// 管理员在网页端建号 → 首次登录必须改密；CLI `serve user add` 不设此标记。
		req.MustChangePassword = true
		req.Actor = accountActor(admin)
		created, err := g.accounts.CreateUser(req)
		if err != nil {
			if errors.Is(err, identity.ErrExists) {
				g.duplicateUsername(w, req.Username)
				return
			}
			accountError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, struct {
			identity.UserListView
			Password string `json:"password"`
		}{identity.UserListView{UserView: created}, req.Password})
	case name != "" && action == "" && (post || r.Method == http.MethodPut):
		var patch identity.UserPatch
		if !decodeJSON(w, r, &patch) {
			return
		}
		patch.Actor = accountActor(admin)
		if name == admin.Username && ((patch.Disabled != nil && *patch.Disabled) || patch.Password != nil) {
			users, err := g.accounts.Users()
			if err != nil {
				accountError(w, err)
				return
			}
			admins := 0
			for _, u := range users {
				if u.Role == identity.RoleAdmin && !u.Disabled {
					admins++
				}
			}
			if patch.Disabled != nil && *patch.Disabled && admins == 1 {
				accountError(w, identity.ErrLastAdmin)
			} else {
				jsonError(w, 409, "Bạn không thể khoá hoặc đặt lại mật khẩu của chính mình tại đây.")
			}
			return
		}
		// 管理员为他人重置密码 → 标记必须改密；自己改自己仍走 /api/me/password 的清除逻辑。
		if patch.Password != nil && name != admin.Username {
			force := true
			patch.MustChangePassword = &force
		}
		updated, err := g.accounts.UpdateUser(name, patch)
		if err != nil {
			accountError(w, err)
			return
		}
		if patch.Password != nil {
			writeJSON(w, 200, struct {
				identity.UserListView
				Password string `json:"password"`
			}{identity.UserListView{UserView: updated}, *patch.Password})
		} else {
			g.writeAccountUser(w, http.StatusOK, updated, nil)
		}
	case name != "" && action == "delete" && post:
		var req struct {
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if name == admin.Username {
			users, err := g.accounts.Users()
			if err != nil {
				accountError(w, err)
				return
			}
			admins := 0
			for _, u := range users {
				if u.Role == identity.RoleAdmin && !u.Disabled {
					admins++
				}
			}
			if admins == 1 {
				accountError(w, identity.ErrLastAdmin)
			} else {
				jsonError(w, 409, "Bạn không thể xoá tài khoản của chính mình.")
			}
			return
		}
		if err := g.accounts.DeleteUser(name, identity.AuditContext{Actor: accountActor(admin), Reason: req.Reason}); err != nil {
			accountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case name != "" && action == "sessions" && get:
		g.listSessions(w, name, current.ID)
	case name != "" && action == "sessions/revoke-all" && post:
		if !g.emptyBody(w, r) {
			return
		}
		if err := g.accounts.RevokeAll(name, identity.AuditContext{Actor: accountActor(admin)}); err != nil {
			accountError(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	case name != "" && action == "pairings" && post:
		if !g.emptyBody(w, r) {
			return
		}
		g.createPairing(w, r, name, admin.Username)
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

func accountActor(user identity.User) *identity.AuditActor {
	return &identity.AuditActor{Username: user.Username, Name: user.Name}
}
func (g *Gateway) duplicateUsername(w http.ResponseWriter, username string) {
	users, err := g.accounts.Users()
	if err != nil {
		accountError(w, err)
		return
	}
	taken := map[string]bool{}
	for _, u := range users {
		taken[u.Username] = true
	}
	base := strings.TrimSpace(username)
	suggest := ""
	for n := 2; ; n++ {
		suffix := "." + strconv.Itoa(n)
		stem := base
		if len(stem) > 32-len(suffix) {
			stem = stem[:32-len(suffix)]
		}
		suggest = stem + suffix
		if !taken[suggest] {
			break
		}
	}
	writeJSON(w, 409, map[string]any{"error": "Tên đăng nhập này đã có. Hãy chọn tên khác.", "field": "username", "suggest": suggest})
}

// memberRoute trả slug project khi path thuộc owner-scoped member API:
//
//	GET    /api/projects/{p}/members          → danh sách thành viên
//	POST   /api/projects/{p}/members          → mời user hiện có (collab|view)
//	PATCH  /api/projects/{p}/members/{user}   → đổi vai collab↔view
//	DELETE /api/projects/{p}/members/{user}   → gỡ khỏi dự án
//	POST   /api/projects/{p}/owner            → chuyển quyền chủ dự án
//
// Guard quyền nằm ở routePermission (owner hoặc admin); handler chỉ parse và
// gọi repository, repository giữ invariant đúng một owner.
func memberRoute(path string) string {
	seg := strings.Split(strings.Trim(path, "/"), "/")
	if len(seg) < 4 || seg[0] != "api" || seg[1] != "projects" || seg[2] == "adopt" {
		return ""
	}
	switch {
	case seg[3] == "members" && (len(seg) == 4 || len(seg) == 5):
		return seg[2]
	case seg[3] == "owner" && len(seg) == 4:
		return seg[2]
	}
	return ""
}

// serveProjectMembers xử lý các route memberRoute; trả false khi method không
// hợp lệ (caller trả 404 theo convention).
func (g *Gateway) serveProjectMembers(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	seg := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	project, kind := seg[2], seg[3]
	lib, err := g.lib(r.Context(), actorOf(user))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, project) {
		return true
	}
	actor := accountActor(user)
	switch {
	case kind == "members" && len(seg) == 4 && r.Method == http.MethodGet:
		members, err := g.accounts.ProjectMembers(project)
		if err != nil {
			accountError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members})
	case kind == "members" && len(seg) == 4 && r.Method == http.MethodPost:
		var req struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		updated, err := g.accounts.SetProjectMember(project, req.Username, identity.ProjectRole(req.Role), actor)
		if err != nil {
			accountError(w, err)
			return true
		}
		g.accountChanged(updated.Username)
		writeJSON(w, http.StatusCreated, identity.ProjectMember{
			Username: updated.Username, Name: updated.Name,
			Role: updated.Projects[project], Disabled: updated.Disabled,
		})
	case kind == "members" && len(seg) == 5 && (r.Method == http.MethodPatch || r.Method == http.MethodPut):
		var req struct {
			Role string `json:"role"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		updated, err := g.accounts.SetProjectMember(project, seg[4], identity.ProjectRole(req.Role), actor)
		if err != nil {
			accountError(w, err)
			return true
		}
		g.accountChanged(updated.Username)
		writeJSON(w, http.StatusOK, identity.ProjectMember{
			Username: updated.Username, Name: updated.Name,
			Role: updated.Projects[project], Disabled: updated.Disabled,
		})
	case kind == "members" && len(seg) == 5 && r.Method == http.MethodDelete:
		if err := g.accounts.RemoveProjectMember(project, seg[4], actor); err != nil {
			accountError(w, err)
			return true
		}
		g.accountChanged(seg[4])
		w.WriteHeader(http.StatusNoContent)
	case kind == "owner" && len(seg) == 4 && r.Method == http.MethodPost:
		var req struct {
			Username string `json:"username"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		if err := g.accounts.TransferProjectOwner(project, req.Username, actor); err != nil {
			accountError(w, err)
			return true
		}
		members, err := g.accounts.ProjectMembers(project)
		if err != nil {
			accountError(w, err)
			return true
		}
		for _, m := range members {
			g.accountChanged(m.Username)
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members})
	default:
		return false
	}
	return true
}
