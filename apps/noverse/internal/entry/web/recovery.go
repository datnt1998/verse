package web

import (
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/noverse/internal/webauth"
)

// GAP-33：mã khôi phục quản trị (một lần, lưu dạng băm) và POST /api/recover.

const (
	// recoverMaxFails / recoverWindow: 5 lần sai mỗi giờ mỗi IP; lần thứ 6 → 429.
	recoverMaxFails = 5
	recoverWindow   = time.Hour

	recoverPasswordShort = "Mật khẩu cần ít nhất 8 ký tự"
)

// recoverPassword xử lý POST /api/recover (công khai, không cần phiên). Cùng điều kiện Content-Type
// JSON và cookie SameSite=Strict như /api/login (đã kiểm ở đầu ServeHTTP).
func (g *Gateway) recoverPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Code        string `json:"code"`
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	key := "ip:" + clientIP(r)
	if wait := g.recoverLimiter.Wait(key); wait > 0 {
		tooMany(w, wait)
		return
	}
	// Mật khẩu quá ngắn không liên quan tới việc đoán mã: không tính vào giới hạn, không chạm tới mã.
	if utf8.RuneCountInString(req.NewPassword) < webauth.MinPasswordLen {
		jsonError(w, http.StatusBadRequest, recoverPasswordShort)
		return
	}
	s, revoked, err := g.accounts.Recover(req.Username, req.Code, req.NewPassword, webauth.DeviceLabel(r.UserAgent()), clientIP(r))
	if err != nil {
		if errors.Is(err, webauth.ErrInvalidRecovery) {
			g.recoverLimiter.Fail(key)
			jsonError(w, http.StatusUnauthorized, webauth.ErrInvalidRecovery.Error())
			return
		}
		accountError(w, err)
		return
	}
	g.recoverLimiter.Succeed(key)
	setSession(w, r, s)
	for _, id := range revoked {
		g.revokeEvent(s.User.Username, id)
	}
	g.accountChanged(s.User.Username)
	g.notifyLogin(r, s)
	g.writeAccountUser(w, http.StatusOK, s.User, map[string]any{})
}

// serveRecovery xử lý /api/me/recovery[/rotate]; false nghĩa là đường dẫn không thuộc nó.
func (g *Gateway) serveRecovery(w http.ResponseWriter, r *http.Request, user webauth.User) bool {
	path := r.URL.Path
	if path != "/api/me/recovery" && path != "/api/me/recovery/rotate" {
		return false
	}
	if !isAdmin(user) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return true
	}
	switch {
	case path == "/api/me/recovery" && r.Method == http.MethodGet:
		status, err := g.accounts.RecoveryStatus(user.Username)
		if err != nil {
			accountError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, status)
	case r.Method == http.MethodPost:
		if !g.emptyBody(w, r) {
			return true
		}
		code, created, err := g.accounts.IssueRecovery(user.Username, path == "/api/me/recovery/rotate")
		if err != nil {
			if errors.Is(err, webauth.ErrRecoveryExists) {
				jsonError(w, http.StatusConflict, err.Error())
				return true
			}
			accountError(w, err)
			return true
		}
		// Mã chỉ hiện một lần: cấm mọi lớp cache giữ lại.
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"code": code, "createdAt": created})
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
	return true
}
