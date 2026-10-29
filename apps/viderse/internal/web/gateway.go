// Package web là HTTP gateway của viderse serve: SPA nhúng, luồng sự kiện SSE
// /api/events và trạng thái đầu trang /api/status gắn trên platform/webapp.
// Handler chỉ đọc request → gọi service → ghi JSON.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/platform/events"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

//go:embed all:dist
var ui embed.FS

// Gateway phục vụ toàn bộ HTTP của app.
type Gateway struct {
	*webapp.Server
	accounts *identity.Store
	hub      *events.Hub
	status   *status.Service
}

// NewGateway dựng handler: accounts là kho identity file-backend; khi chưa có
// tài khoản nào, server tạo một setup state (chỉ giữ hash) — serve in mã
// plaintext một lần qua SetupCodeOnce. svc phải được dựng với publishStatus
// của cùng hub (xem serve.go).
func NewGateway(accounts *identity.Store, hub *events.Hub, svc *status.Service) *Gateway {
	dist, err := fs.Sub(ui, "dist")
	if err != nil {
		// embed đảm bảo dist tồn tại khi build; lỗi ở đây là binary hỏng.
		panic(err)
	}
	g := &Gateway{
		Server: webapp.New(accounts, webapp.Options{
			AppName:    "Viderse",
			CookieName: "viderse_session",
			UI:         dist,
		}),
		accounts: accounts,
		hub:      hub,
		status:   svc,
	}
	g.Handle(http.MethodGet, "/api/events", g.serveEvents)
	g.Handle(http.MethodGet, "/api/status", g.getStatus)
	return g
}

// statusWire và attentionWire là wire DTO của GET /api/status theo hợp đồng —
// giữ ở tầng web để status.Service tiến hoá không ngầm đổi hợp đồng bên ngoài.
type statusWire struct {
	Running     int             `json:"running"`
	Queued      int             `json:"queued"`
	Paused      bool            `json:"paused"`
	ReviewCount int             `json:"reviewCount"`
	Attention   []attentionWire `json:"attention"`
}

type attentionWire struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Href string `json:"href"`
}

func statusToWire(s status.Snapshot) statusWire {
	att := make([]attentionWire, 0, len(s.Attention))
	for _, a := range s.Attention {
		att = append(att, attentionWire{Kind: a.Kind, Text: a.Text, Href: a.Href})
	}
	return statusWire{
		Running:     s.Running,
		Queued:      s.Queued,
		Paused:      s.Paused,
		ReviewCount: s.ReviewCount,
		Attention:   att,
	}
}

// getStatus trả trạng thái đầu trang. ETag suy từ chính bytes của body đã mã
// hoá, không từ revision trong bộ nhớ: revision đếm lại từ 0 sau mỗi lần khởi
// động, nên hai tiến trình khác nhau có thể cùng "0" nhưng nội dung khác nhau —
// dùng revision làm ETag sẽ trả 304 với body cũ trong cache trình duyệt. Băm
// nội dung cũng tự đúng khi một trường (vd cờ tạm dừng) đổi mà không Bump.
// Revision vẫn cần cho sự kiện SSE.
func (g *Gateway) getStatus(w http.ResponseWriter, r *http.Request, _ identity.User) {
	snap, err := g.status.Snapshot(r.Context())
	if err != nil {
		webapp.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	body, err := json.Marshal(statusToWire(snap))
	if err != nil {
		webapp.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	etag := statusETag(body)
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// statusETag băm nội dung body thành ETag mạnh (16 hex đầu của SHA-256).
func statusETag(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf(`"%x"`, sum[:8])
}

// etagMatches so khớp If-None-Match theo RFC 9110: "*" khớp mọi thứ, còn lại là
// danh sách ETag phân tách bằng dấu phẩy, so khớp yếu (bỏ tiền tố W/).
func etagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		if strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}

// serveEvents phục vụ luồng SSE /api/events: Subscribe của hub quyết phần
// replay/reset, platform/events.Serve giữ khung frame chung; phiên hết hiệu
// lực thì đóng kết nối ở nhịp tiếp theo.
func (g *Gateway) serveEvents(w http.ResponseWriter, r *http.Request, user identity.User) {
	sub, replay, stale := g.hub.Subscribe(user, events.ResumeID(r))
	defer g.hub.Unsubscribe(sub)
	hooks := events.Hooks{
		OnTick: func() bool { return g.eventSessionValid(r) },
	}
	if err := g.hub.Serve(w, r, sub, replay, stale, hooks); err != nil {
		webapp.Error(w, http.StatusInternalServerError, "không hỗ trợ luồng sự kiện")
	}
}

func (g *Gateway) eventSessionValid(r *http.Request) bool {
	cookie, err := r.Cookie("viderse_session")
	if err != nil {
		return false
	}
	_, _, err = g.accounts.Authenticate(cookie.Value)
	return err == nil
}
