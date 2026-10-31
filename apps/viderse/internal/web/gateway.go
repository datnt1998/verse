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
	"mime"
	"net/http"
	"strings"
	"time"

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
	Clone             CloneService
	accounts          *identity.Store
	hub               *events.Hub
	status            *status.Service
	serverStorageMode func() bool
	scopedRender      http.Handler
	renderCloneRoute  func(http.ResponseWriter, *http.Request) bool
	cloneReviewRoute  func(http.ResponseWriter, *http.Request) bool
	clonePostedRoute  func(http.ResponseWriter, *http.Request) bool
	channelRoutes     map[string][]channelRoute
}

// NewGateway dựng handler: accounts là kho identity chung (Postgres); khi
// chưa có tài khoản nào, server tạo một setup state (chỉ giữ hash) — serve
// in mã plaintext một lần qua SetupCodeOnce. svc phải được dựng với
// publishStatus của cùng hub (xem serve.go).
func NewGateway(accounts *identity.Store, hub *events.Hub, svc *status.Service) *Gateway {
	dist, err := fs.Sub(ui, "dist")
	if err != nil {
		// embed đảm bảo dist tồn tại khi build; lỗi ở đây là binary hỏng.
		panic(err)
	}
	g := &Gateway{
		Server: webapp.New(accounts, webapp.Options{
			AppName:      "Viderse",
			App:          identity.AppViderse,
			UI:           dist,
			AllowNonJSON: allowFileUpload,
		}),
		accounts: accounts,
		hub:      hub,
		status:   svc,
	}
	g.Handle(http.MethodGet, "/api/events", g.serveEvents)
	g.Handle(http.MethodGet, "/api/status", g.getStatus)
	return g
}

// SetServerStorageMode supplies the live runtime capability; desktop callers
// leave it unset. The callback must not perform filesystem or network I/O.
func (g *Gateway) SetServerStorageMode(mode func() bool) {
	g.serverStorageMode = mode
}

// allowFileUpload exempts only authenticated binary-upload endpoints from the
// JSON content-type guard. Origin/auth checks still run at the shared gateway.
func allowFileUpload(r *http.Request) bool {
	if r.Method == http.MethodPost && r.URL.Path == "/api/editor/media" {
		return true
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/clone/") {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/clone/"), "/")
		if len(parts) == 2 && cloneSegment(parts[0]) && parts[1] == "cover" {
			return true
		}
	}
	if r.Method == http.MethodPut {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/editor/"), "/")
		if strings.HasPrefix(r.URL.Path, "/api/editor/") {
			if len(parts) == 2 && parts[0] == "media" && parts[1] != "" {
				return true
			}
			if len(parts) == 4 && parts[0] == "projects" && parts[1] != "" && parts[2] == "exports" && parts[3] != "" {
				return true
			}
		}
	}
	if r.Method != http.MethodPost {
		return false
	}
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || typ != "multipart/form-data" {
		return false
	}
	switch r.URL.Path {
	case "/api/overlays", "/api/image-prompts/import":
		return true
	}
	return false
}

// statusWire và attentionWire là wire DTO của GET /api/status theo hợp đồng —
// giữ ở tầng web để status.Service tiến hoá không ngầm đổi hợp đồng bên ngoài.
type statusWire struct {
	Running     int              `json:"running"`
	Queued      int              `json:"queued"`
	Paused      bool             `json:"paused"`
	ReviewCount int              `json:"reviewCount"`
	Attention   []attentionWire  `json:"attention"`
	StorageDown *storageDownWire `json:"storageDown,omitempty"`
}

type attentionWire struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Href string `json:"href"`
}

type storageDownWire struct {
	Label string `json:"label"`
	Since string `json:"since"`
}

func statusToWire(s status.Snapshot, admin bool) statusWire {
	att := make([]attentionWire, 0, len(s.Attention))
	for _, a := range s.Attention {
		if !admin && (a.AdminOnly || strings.Contains(a.Href, "section=storage")) {
			continue
		}
		att = append(att, attentionWire{Kind: a.Kind, Text: a.Text, Href: a.Href})
	}
	out := statusWire{
		Running:     s.Running,
		Queued:      s.Queued,
		Paused:      s.Paused,
		ReviewCount: s.ReviewCount,
		Attention:   att,
	}
	if s.StorageDown != nil {
		out.StorageDown = &storageDownWire{
			Label: s.StorageDown.Label, Since: s.StorageDown.Since.UTC().Format(time.RFC3339),
		}
	}
	return out
}

// getStatus trả trạng thái đầu trang. ETag suy từ chính bytes của body đã mã
// hoá, không từ revision trong bộ nhớ: revision đếm lại từ 0 sau mỗi lần khởi
// động, nên hai tiến trình khác nhau có thể cùng "0" nhưng nội dung khác nhau —
// dùng revision làm ETag sẽ trả 304 với body cũ trong cache trình duyệt. Băm
// nội dung cũng tự đúng khi một trường (vd cờ tạm dừng) đổi mà không Bump.
// Revision vẫn cần cho sự kiện SSE.
func (g *Gateway) getStatus(w http.ResponseWriter, r *http.Request, user identity.User) {
	snap, err := g.status.Snapshot(r.Context())
	if err != nil {
		webapp.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	body, err := json.Marshal(statusToWire(snap, user.Role == identity.RoleAdmin))
	if err != nil {
		webapp.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	etag := statusETag(body)
	w.Header().Set("Cache-Control", "private, no-cache")
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
// lực thì đóng kết nối ở lần kiểm tra phiên kế tiếp (≤ sessionRecheck).
func (g *Gateway) serveEvents(w http.ResponseWriter, r *http.Request, user identity.User) {
	sub, replay, stale := g.hub.Subscribe(user, events.ResumeID(r))
	defer g.hub.Unsubscribe(sub)
	hooks := events.Hooks{
		OnTick: throttledCheck(sessionRecheck, time.Now, func() bool { return g.eventSessionValid(r) }),
	}
	if err := g.hub.Serve(w, r, sub, replay, stale, hooks); err != nil {
		webapp.Error(w, http.StatusInternalServerError, "không hỗ trợ luồng sự kiện")
	}
}

// sessionRecheck là nhịp kiểm tra lại phiên của luồng SSE. Ping 2 s chỉ để phát
// hiện mất kết nối; kiểm phiên mỗi ping là một truy vấn DB mỗi 2 s cho mỗi tab
// đang mở, nên phiên chỉ được kiểm lại sau mỗi 15 s (bằng nhịp ping của Noverse).
const sessionRecheck = 15 * time.Second

// throttledCheck gọi check khi đã quá every kể từ lần gọi trước (lần đầu thì
// chờ đủ every, vì phiên vừa được xác thực khi mở luồng); giữa hai lần gọi trả
// true. Một lần check false làm hook trả false để đóng luồng.
func throttledCheck(every time.Duration, now func() time.Time, check func() bool) func() bool {
	last := now()
	return func() bool {
		t := now()
		if t.Sub(last) < every {
			return true
		}
		last = t
		return check()
	}
}

// eventSessionValid đọc cookie qua đúng tên của policy chung và kiểm lại cả
// grant app lẫn cờ bắt đổi mật khẩu trên kết quả Authenticate mới — không
// chỉ err==nil: phiên bị revoke grant hoặc bị khoá vào màn đổi mật khẩu
// phải đóng luồng ở nhịp kiểm kế tiếp.
func (g *Gateway) eventSessionValid(r *http.Request) bool {
	cookie, err := r.Cookie(g.SessionCookieName())
	if err != nil {
		return false
	}
	user, _, err := g.accounts.Authenticate(cookie.Value)
	return err == nil && g.BusinessAllowed(user)
}
