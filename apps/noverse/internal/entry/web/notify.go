package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/notify"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

type gatewayNotifications struct {
	store     *notify.Store
	err       error
	mu        sync.Mutex
	snapshots map[string]host.UISnapshot
	errors    map[string]string
}

func (g *Gateway) initNotifications() {
	n := &gatewayNotifications{snapshots: map[string]host.UISnapshot{}, errors: map[string]string{}}
	g.notifications = n
	n.store, n.err = notify.Open(notify.Options{Allowed: g.notifyAllowed, SessionActive: func(user, id string) bool {
		sessions, e := g.accounts.Sessions(user, "")
		if e != nil {
			return false
		}
		for _, s := range sessions {
			if s.ID == id {
				return true
			}
		}
		return false
	}, Changed: func(user string, unread int) {
		g.events().Publish(Event{Type: "notify.changed", User: user, Data: map[string]any{"unread": unread}})
	}})
	if n.err != nil {
		slog.Error("Không mở được kho thông báo", "module", "web", "err", n.err)
		return
	}
	hub := g.events()
	hub.mu.Lock()
	hub.onEvent = append(hub.onEvent, g.consumeNotifyEvent)
	hub.mu.Unlock()
}
func (g *Gateway) closeNotifications() {
	if g.notifications != nil && g.notifications.store != nil {
		g.notifications.store.Close()
	}
}
func (g *Gateway) notifyAllowed(username, story string) bool {
	if g.accounts == nil {
		return false
	}
	users, e := g.accounts.Users()
	if e != nil {
		return false
	}
	for _, v := range users {
		if v.Username == username {
			u := webauth.User{Username: v.Username, Role: v.Role, Projects: v.Projects, Disabled: v.Disabled}
			return !u.Disabled && (story == "" || u.CanRead(story))
		}
	}
	return false
}
func (g *Gateway) notifyLogin(r *http.Request, s webauth.NewSession) {
	id, _, _ := strings.Cut(s.Cookie, ".")
	g.events().Publish(Event{Type: "security.login", User: s.User.Username, Data: map[string]any{"session": id, "device": webauth.DeviceLabel(r.UserAgent()), "ip": clientIP(r)}})
}
func (g *Gateway) consumeNotifyEvent(ev Event) {
	n := g.notifications
	if n == nil || n.store == nil {
		return
	}
	if ev.Type == "notify.changed" {
		return
	}
	var data map[string]any
	b, _ := json.Marshal(ev.Data)
	_ = json.Unmarshal(b, &data)
	if ev.Type == "session.revoked" {
		session, _ := data["session"].(string)
		if e := n.store.Revoke(ev.User, session); e != nil {
			slog.Error("Không gỡ được đăng ký thông báo", "module", "web", "err", e)
		}
		return
	}
	item := notify.PushNotification{Story: ev.Project + "/" + ev.Story, Actions: []notify.Action{}}
	text := func(k string) string { v, _ := data[k].(string); return v }
	num := func(k string) int { v, _ := data[k].(float64); return int(v) }
	switch ev.Type {
	case "security.login":
		item.Type = "security"
		item.Story = ""
		item.Title = "Đăng nhập trên thiết bị mới"
		item.Body = text("device") + " · " + text("ip")
		item.URL = "/more/devices"
		item.Session = text("session")
		item.Actions = []notify.Action{{Action: "revoke", Title: "Không phải tôi"}}
	case "gate.waiting":
		item.Type = "gate"
		item.Chapter = num("chapter")
		item.Title = fmt.Sprintf("Chương %d chờ bạn duyệt", item.Chapter)
		item.Body = text("body")
		item.Body, item.Score = g.notifyChapterFacts(item.Story, item.Chapter, item.Body)
		item.URL = fmt.Sprintf("%s/%d?gate=1", notifyStoryPath("read", item.Story), item.Chapter)
		item.Actions = []notify.Action{{Action: "approve", Title: "Duyệt & viết tiếp"}, {Action: "rewrite", Title: "Nhờ viết lại…", URL: notifyRewriteURL(item.Story, item.Chapter)}, {Action: "mute", Title: "Tắt loại thông báo này"}}
	case "engine.stopped.error":
		item.Type = "error"
		item.Title = "Truyện dừng do lỗi model"
		item.Body = friendlyModelError(text("body"))
		item.URL = notifyStoryPath("write", item.Story)
	case "chapter.committed":
		item.Type = "done"
		item.Chapter = num("chapter")
		item.Title = fmt.Sprintf("Xong chương %d", item.Chapter)
		// Không dùng tóm tắt của công cụ (commit_chapter…) làm nội dung: chỉ tên chương + điểm.
		item.Body, item.Score = g.notifyChapterFacts(item.Story, item.Chapter, "")
		item.URL = fmt.Sprintf("%s/%d", notifyStoryPath("read", item.Story), item.Chapter)
	case "job.done":
		kind := text("kind")
		if kind != "export" && kind != "audio" && kind != "import" && kind != "sync" && kind != "cocreate" {
			return
		}
		item.Type = "job"
		item.Job = text("id")
		item.Title = text("title")
		item.Body = text("step")
		item.URL = "/cmd/" + url.PathEscape(item.Job)
		switch kind {
		case "audio":
			item.Title = strings.Replace(text("title"), "Đã xuất audio", "Đã xuất sách nói", 1)
			if result, ok := data["result"].(map[string]any); ok {
				if sub, ok := result["sub"].(string); ok {
					item.Body = sub
				}
			}
			item.URL = notifyStoryPath("listen", item.Story)
			item.Actions = []notify.Action{{Action: "share", Title: "Chia sẻ", URL: item.URL}}
		case "export":
			item.Title = "Đã xuất truyện"
			if result, ok := data["result"].(map[string]any); ok {
				if title, ok := result["title"].(string); ok && title != "" {
					item.Title = title
				}
				if sub, ok := result["sub"].(string); ok {
					item.Body = sub
				}
				if acts, ok := result["actions"].([]any); ok {
					for _, raw := range acts {
						if a, ok := raw.(map[string]any); ok && a["kind"] == "download" {
							if href, ok := a["href"].(string); ok && strings.HasPrefix(href, "/api/") {
								item.URL = href
							}
						}
					}
				}
			}
			item.Actions = []notify.Action{{Action: "share", Title: "Chia sẻ", URL: item.URL}}
		case "import":
			item.Type = kind
			count := 0
			if unit, ok := data["unit"].(map[string]any); ok {
				if v, ok := unit["done"].(float64); ok {
					count = int(v)
				}
			}
			item.Title = fmt.Sprintf("Đã nhập %d chương", count)
		case "sync":
			item.Type = kind
			item.Title = "Đã tiếp nhận chương sửa"
		case "cocreate":
			item.Type = kind
			item.Title = "Đã bàn xong hướng đi"
		}
	default:
		return
	}
	users, e := g.accounts.Users()
	if e != nil {
		return
	}
	for _, v := range users {
		u := webauth.User{Username: v.Username, Role: v.Role, Projects: v.Projects, Disabled: v.Disabled}
		if ev.visibleTo(u) {
			if e := n.store.Record(v.Username, notifyForRecipient(item, u)); e != nil {
				slog.Error("Không lưu được thông báo", "module", "web", "err", e)
			}
		}
	}
}

func (g *Gateway) notifyChapterFacts(story string, chapter int, fallback string) (string, *float64) {
	st, err := g.resolveStoryID(story)
	if err != nil {
		return fallback, nil
	}
	// Notifications run without a request actor. Hosted facts need a scoped
	// actor, so the notification keeps its deterministic fallback instead of
	// reading a directory that does not exist.
	if g.factStore != nil {
		return fallback, nil
	}
	reader := host.NewStoryReader(st.Dir)
	body := fallback
	if chapters, err := reader.Chapters(); err == nil {
		for _, item := range chapters {
			if item.Chapter == chapter && item.Title != "" {
				body = item.Title
				break
			}
		}
	}
	reviews, err := reader.Reviews(chapter)
	if err != nil || reviews.Chapter == nil || len(reviews.Chapter.Dimensions) == 0 {
		return body, nil
	}
	var total float64
	for _, dimension := range reviews.Chapter.Dimensions {
		total += float64(dimension.Score)
	}
	score := total / float64(len(reviews.Chapter.Dimensions))
	return chapterFactsText(body, score), &score
}

// chapterFactsText: “<tên chương> · Biên tập viên chấm 8,4” (điểm 0–100 hiển thị thang 10).
func chapterFactsText(body string, score float64) string {
	text := "Biên tập viên chấm " + notify.ScoreText(score)
	if body != "" {
		text = body + " · " + text
	}
	return text
}

// notifyStoryPath dựng liên kết sâu /read|write|listen/:story. Mã truyện là "dự-án/truyện" nên
// dấu “/” phải được mã hoá (%2F): router tách đoạn theo “/” rồi mới giải mã.
func notifyStoryPath(kind, story string) string {
	return "/" + kind + "/" + url.PathEscape(story)
}

func notifyRewriteURL(story string, chapter int) string {
	return fmt.Sprintf("%s/%d?rewrite=1", notifyStoryPath("read", story), chapter)
}

// notifyForRecipient bỏ thao tác ghi (duyệt/viết lại) khỏi bản gửi cho người chỉ được đọc truyện.
func notifyForRecipient(item notify.PushNotification, u webauth.User) notify.PushNotification {
	if item.Type != "gate" || u.CanWrite(item.Story) {
		return item
	}
	item.Actions = slices.DeleteFunc(slices.Clone(item.Actions), func(a notify.Action) bool { return a.Action == "approve" || a.Action == "rewrite" })
	return item
}

var (
	errQuotaText = regexp.MustCompile(`(?i)\b429\b|quota|rate.?limit|insufficient|credit|billing|hạn mức`)
	errAuthText  = regexp.MustCompile(`(?i)\b40[13]\b|unauthori[sz]ed|forbidden|invalid.{0,12}(api.?)?key|api.?key|authenticat|khoá api|khóa api`)
	errSlowText  = regexp.MustCompile(`(?i)timeout|timed out|deadline exceeded|quá thời gian`)
	errBusyText  = regexp.MustCompile(`(?i)\b50[023]\b|overloaded|unavailable|bad gateway|quá tải`)
	errNetText   = regexp.MustCompile(`(?i)connection|dial |no such host|\beof\b|network|reset by peer|mất kết nối`)
)

// friendlyModelError đổi lỗi thô của engine thành nguyên nhân dễ hiểu (Bất biến #7: không đẩy
// mã trạng thái, tên công cụ hay nội dung lỗi kỹ thuật ra thông báo).
func friendlyModelError(summary string) string {
	switch {
	case errQuotaText.MatchString(summary):
		return "Nhà cung cấp model đã hết hạn mức hoặc đang giới hạn tốc độ"
	case errAuthText.MatchString(summary):
		return "Khoá API không hợp lệ hoặc đã hết hạn"
	case errSlowText.MatchString(summary):
		return "Model phản hồi quá lâu"
	case errBusyText.MatchString(summary):
		return "Dịch vụ model đang quá tải hoặc tạm ngưng"
	case errNetText.MatchString(summary):
		return "Mất kết nối tới nhà cung cấp model"
	}
	return "Model gặp lỗi khi viết. Mở truyện để xem chi tiết và chạy lại"
}

var committedChapter = regexp.MustCompile(`commit_chapter\((?:chương )?([0-9]+)\)`)

func (g *Gateway) notifyWorkerEvent(key, kind string, data []byte) {
	n := g.notifications
	if n == nil || n.store == nil {
		return
	}
	p, s, ok := strings.Cut(key, "/")
	if !ok || strings.HasPrefix(key, "draft:") {
		return
	}
	publish := func(kind string, data any) { g.events().Publish(Event{Type: kind, Project: p, Story: s, Data: data}) }
	switch kind {
	case "snapshot":
		var snap host.UISnapshot
		if json.Unmarshal(data, &snap) != nil {
			return
		}
		n.mu.Lock()
		prev := n.snapshots[key]
		n.snapshots[key] = snap
		errBody := n.errors[key]
		n.mu.Unlock()
		if snap.WaitingFor == "gate" && (prev.WaitingFor != "gate" || prev.CurrentChapter != snap.CurrentChapter) {
			chapter := snap.AdvancePermitChapter
			if chapter == 0 {
				chapter = snap.CompletedCount
			}
			if chapter == 0 {
				chapter = snap.CurrentChapter
			}
			publish("gate.waiting", map[string]any{"chapter": chapter, "body": snap.LastReviewSummary})
		}
		if prev.IsRunning && !snap.IsRunning && snap.WaitingFor != "gate" && snap.Phase != "complete" && errBody != "" {
			publish("engine.stopped.error", map[string]any{"body": errBody})
			n.mu.Lock()
			delete(n.errors, key)
			n.mu.Unlock()
		}
		if !prev.IsRunning && snap.IsRunning {
			n.mu.Lock()
			delete(n.errors, key)
			n.mu.Unlock()
		}
	case "event":
		var ev host.Event
		if json.Unmarshal(data, &ev) != nil {
			return
		}
		if ev.Category == "ERROR" || ev.Level == "error" {
			n.mu.Lock()
			n.errors[key] = ev.Summary
			n.mu.Unlock()
		}
		if ev.Category == "TOOL" && !ev.Failed && !ev.FinishedAt.IsZero() {
			n.mu.Lock()
			delete(n.errors, key)
			n.mu.Unlock()
			match := committedChapter.FindStringSubmatch(ev.Summary)
			if len(match) > 1 {
				chapter, _ := strconv.Atoi(match[1])
				publish("chapter.committed", map[string]any{"chapter": chapter, "body": ev.Summary})
			}
		}
	}
}
func (g *Gateway) serveNotify(w http.ResponseWriter, r *http.Request, user webauth.User, current webauth.Session) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/notify/") && !strings.HasPrefix(path, "/api/push/") && path != "/api/notifications" && !strings.HasPrefix(path, "/api/notifications/") {
		return false
	}
	n := g.notifications
	if n == nil || n.store == nil {
		jsonError(w, 503, "Thông báo chưa sẵn sàng")
		return true
	}
	store := n.store
	fail := func(e error) {
		switch {
		case errors.Is(e, notify.ErrInvalid):
			jsonError(w, 400, e.Error())
		default:
			jsonError(w, 500, "Không lưu được thông báo")
		}
	}
	switch {
	case path == "/api/push/vapid" && r.Method == "GET":
		writeJSON(w, 200, map[string]any{"publicKey": store.PublicKey()})
	case path == "/api/push/subscriptions" && r.Method == "GET":
		writeJSON(w, 200, map[string]any{"subscriptions": store.Subscriptions(user.Username)})
	case path == "/api/push/subscriptions" && (r.Method == "POST" || r.Method == "DELETE"):
		var sub notify.Subscription
		if !decodeJSON(w, r, &sub) {
			return true
		}
		var e error
		if r.Method == "POST" {
			sub.Device = strings.TrimSpace(sub.Device)
			if sub.Device == "" {
				sub.Device = webauth.DeviceLabel(r.UserAgent())
			}
			e = store.Subscribe(user.Username, current.ID, sub)
		} else {
			e = store.Unsubscribe(user.Username, sub.Endpoint)
		}
		if e != nil {
			fail(e)
		} else {
			writeJSON(w, 200, map[string]bool{"ok": true})
		}
	case path == "/api/notify/prefs" && r.Method == "GET":
		p := store.Prefs(user.Username)
		w.Header().Set("ETag", "\""+p.Version+"\"")
		writeJSON(w, 200, p)
	case path == "/api/notify/prefs" && r.Method == "PUT":
		var p notify.Prefs
		if !decodeJSON(w, r, &p) {
			return true
		}
		p, e := store.PutPrefs(user.Username, p, r.Header.Get("If-Match"))
		if errors.Is(e, notify.ErrConflict) {
			writeJSON(w, 409, map[string]any{"error": e.Error(), "conflict": true, "version": p.Version, "current": p})
		} else if e != nil {
			fail(e)
		} else {
			w.Header().Set("ETag", "\""+p.Version+"\"")
			writeJSON(w, 200, p)
			_, count, _ := store.Inbox(user.Username, "")
			g.events().Publish(Event{Type: "notify.changed", User: user.Username, Data: map[string]any{"unread": count}})
		}
	case path == "/api/notifications" && r.Method == "GET":
		items, count, next := store.Inbox(user.Username, r.URL.Query().Get("before"))
		writeJSON(w, 200, map[string]any{"notifications": items, "unread": count, "nextBefore": next})
	case path == "/api/notifications/read" && r.Method == "POST":
		var req struct {
			IDs []string `json:"ids"`
			All bool     `json:"all"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		count, e := store.Read(user.Username, req.IDs, req.All)
		if e != nil {
			fail(e)
		} else {
			writeJSON(w, 200, map[string]any{"ok": true, "unread": count})
		}
	case path == "/api/notify/test" && r.Method == "POST":
		if !g.emptyBody(w, r) {
			return true
		}
		e := store.Record(user.Username, notify.PushNotification{Type: "security", Title: "Thông báo thử từ Noverse", Body: "Điện thoại đã sẵn sàng nhận thông báo", URL: "/notifications"})
		if e != nil {
			fail(e)
		} else {
			writeJSON(w, 200, map[string]bool{"ok": true})
		}
	case strings.HasPrefix(path, "/api/notifications/") && strings.HasSuffix(path, "/action") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/notifications/"), "/action")
		item, ok := store.Find(user.Username, id)
		if !ok {
			jsonError(w, 404, "Không tìm thấy thông báo")
			return true
		}
		var req struct {
			Action string `json:"action"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		offered := false
		for _, action := range item.Actions {
			if action.Action == req.Action {
				offered = true
				break
			}
		}
		if !offered {
			jsonError(w, 400, "Thông báo không có thao tác này")
			return true
		}
		switch req.Action {
		case "revoke":
			if item.Type != "security" || item.Session == "" {
				jsonError(w, 400, "Thông báo không có thiết bị để đăng xuất")
				return true
			}
			e := g.accounts.RevokeSession(item.Session, func(owner string) bool { return owner == user.Username })
			if e != nil && !errors.Is(e, webauth.ErrNotFound) {
				accountError(w, e)
				return true
			}
			_ = store.Revoke(user.Username, item.Session)
			g.revokeEvent(user.Username, item.Session)
			if item.Session == current.ID {
				clearSession(w, r)
			}
		case "mute":
			p := store.Prefs(user.Username)
			switch item.Type {
			case "gate":
				p.Types.Gate = false
			case "done":
				p.Types.Done = false
			case "job", "import", "sync", "cocreate":
				p.Types.Job = false
			default:
				jsonError(w, 400, "Không thể tắt thông báo an toàn hoặc lỗi")
				return true
			}
			if _, e := store.PutPrefs(user.Username, p, p.Version); e != nil {
				fail(e)
				return true
			}
		case "approve":
			// 与故事路由同一隐藏规则：非 gate 通知或无写权限都回 404。
			if item.Type != "gate" || !user.CanWrite(item.Story) {
				jsonError(w, 404, "Không tìm thấy truyện")
				return true
			}
			p, s, _ := strings.Cut(item.Story, "/")
			copy := r.Clone(r.Context())
			copy.URL.Path = "/api/projects/" + p + "/stories/" + s + "/next"
			copy.Body = io.NopCloser(bytes.NewReader([]byte(fmt.Sprintf(`{"applyComments":true,"chapter":%d}`, item.Chapter))))
			rec := &commentStatusWriter{ResponseWriter: w, status: 200}
			g.serveComments(rec, copy, user)
			if rec.status < 300 {
				_, _ = store.Read(user.Username, []string{id}, false)
			}
			return true
		case "rewrite":
			writeJSON(w, 200, map[string]any{"ok": true, "url": notifyRewriteURL(item.Story, item.Chapter)})
			return true
		case "share":
			writeJSON(w, 200, map[string]any{"ok": true, "url": item.URL})
			return true
		default:
			jsonError(w, 400, "Thao tác thông báo không hợp lệ")
			return true
		}
		_, _ = store.Read(user.Username, []string{id}, false)
		writeJSON(w, 200, map[string]bool{"ok": true})
	default:
		jsonError(w, 404, "Không tìm thấy")
	}
	return true
}
