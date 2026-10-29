package web

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/webauth"
)

func inboxOf(t *testing.T, g *Gateway, cookie string) []map[string]any {
	t.Helper()
	w := authDo(t, g, "GET", "/api/notifications", cookie, "")
	if w.Code != 200 {
		t.Fatalf("inbox: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Notifications []map[string]any `json:"notifications"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		t.Fatal(e)
	}
	return body.Notifications
}

func actionIDs(item map[string]any) []string {
	ids := []string{}
	for _, raw := range item["actions"].([]any) {
		ids = append(ids, raw.(map[string]any)["action"].(string))
	}
	return ids
}

func find(items []map[string]any, typ, job string) map[string]any {
	for _, item := range items {
		if item["type"] != typ {
			continue
		}
		if j, _ := item["job"].(string); job == "" || j == job {
			return item
		}
	}
	return nil
}

// routeResolves mô phỏng web/src/router.ts: tách theo “/” rồi mới giải mã từng đoạn.
func routeResolves(t *testing.T, link, kind, story string, chapter string) {
	t.Helper()
	path, _, _ := strings.Cut(link, "?")
	seg := strings.Split(strings.Trim(path, "/"), "/")
	want := 2
	if chapter != "" {
		want = 3
	}
	if len(seg) != want || seg[0] != kind {
		t.Fatalf("%s không phải liên kết sâu /%s/:story%s", link, kind, map[bool]string{true: "/:chương", false: ""}[chapter != ""])
	}
	if id, e := url.PathUnescape(seg[1]); e != nil || id != story {
		t.Fatalf("%s: mã truyện giải ra %q, muốn %q", link, id, story)
	}
	if chapter != "" && seg[2] != chapter {
		t.Fatalf("%s: chương %q, muốn %q", link, seg[2], chapter)
	}
}

func TestNotificationPayloadContractPerType(t *testing.T) {
	g, accounts := newAuthGateway(t)
	if _, e := accounts.CreateUser(webauth.NewUser{Username: "reader", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"p": webauth.ProjectView}, Password: "reader-pass"}); e != nil {
		t.Fatal(e)
	}
	cookie := authLogin(t, g, "writer", "writer-pass")
	readerCookie := authLogin(t, g, "reader", "reader-pass")
	prefs := g.notifications.store.Prefs("writer")
	prefs.Digest = "each"
	if _, e := g.notifications.store.PutPrefs("writer", prefs, ""); e != nil {
		t.Fatal(e)
	}
	publish := func(typ string, data map[string]any) {
		g.events().Publish(Event{Type: typ, Project: "p", Story: "s", Data: data})
	}
	publish("gate.waiting", map[string]any{"chapter": 25, "body": ""})
	publish("engine.stopped.error", map[string]any{"body": "POST https://api.example/v1/chat: 429 Too Many Requests"})
	publish("chapter.committed", map[string]any{"chapter": 20, "body": "commit_chapter(chương 20)"})
	publish("job.done", map[string]any{"kind": "audio", "id": "p/s/audio/1/export", "title": "Đã xuất audio Tập 1", "result": map[string]any{"sub": "118 giây · M4B"}})
	publish("job.done", map[string]any{"kind": "export", "id": "p/s/export/1", "title": "x", "result": map[string]any{"title": "Đã xuất truyện", "sub": "24 chương · 2,1 MB", "actions": []any{map[string]any{"kind": "download", "href": "/api/projects/p/stories/s/exports/file?path=a.epub"}}}})
	publish("job.done", map[string]any{"kind": "import", "id": "p/s/import/1", "title": "Nhập", "unit": map[string]any{"done": 24, "total": 24}})
	publish("job.done", map[string]any{"kind": "sync", "id": "p/s/sync/1", "title": "Đồng bộ"})
	publish("job.done", map[string]any{"kind": "cocreate", "id": "p/s/cocreate/1", "title": "Bàn"})
	items := inboxOf(t, g, cookie)

	cases := []struct {
		typ       string
		title     string
		bodyHas   string
		actions   []string
		urlCheck  func(link string)
		wantChap  float64
		wantJob   string
		urlPrefix string
	}{
		{typ: "gate", title: "Chương 25 chờ bạn duyệt", actions: []string{"approve", "rewrite", "mute"}, wantChap: 25,
			urlCheck: func(l string) { routeResolves(t, l, "read", "p/s", "25"); expectQuery(t, l, "gate=1") }},
		{typ: "error", title: "Truyện dừng do lỗi model", bodyHas: "hết hạn mức", actions: []string{},
			urlCheck: func(l string) { routeResolves(t, l, "write", "p/s", "") }},
		{typ: "done", title: "Xong chương 20", actions: []string{}, wantChap: 20,
			urlCheck: func(l string) { routeResolves(t, l, "read", "p/s", "20") }},
		{typ: "job", title: "Đã xuất sách nói Tập 1", bodyHas: "M4B", actions: []string{"share"}, wantJob: "p/s/audio/1/export",
			urlCheck: func(l string) { routeResolves(t, l, "listen", "p/s", "") }},
		{typ: "job", title: "Đã xuất truyện", bodyHas: "24 chương", actions: []string{"share"}, wantJob: "p/s/export/1",
			urlPrefix: "/api/projects/p/stories/s/exports/file?path=a.epub"},
		{typ: "import", title: "Đã nhập 24 chương", actions: []string{}, wantJob: "p/s/import/1", urlPrefix: "/cmd/p%2Fs%2Fimport%2F1"},
		{typ: "sync", title: "Đã tiếp nhận chương sửa", actions: []string{}, wantJob: "p/s/sync/1", urlPrefix: "/cmd/p%2Fs%2Fsync%2F1"},
		{typ: "cocreate", title: "Đã bàn xong hướng đi", actions: []string{}, wantJob: "p/s/cocreate/1", urlPrefix: "/cmd/p%2Fs%2Fcocreate%2F1"},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			item := find(items, c.typ, c.wantJob)
			if item == nil {
				t.Fatalf("thiếu thông báo %s", c.typ)
			}
			for _, key := range []string{"id", "type", "story", "title", "body", "url", "actions", "createdAt", "read"} {
				if _, ok := item[key]; !ok {
					t.Fatalf("payload thiếu khoá %q: %v", key, item)
				}
			}
			if item["story"] != "p/s" || item["title"] != c.title {
				t.Fatalf("story/title = %v / %v", item["story"], item["title"])
			}
			if c.bodyHas != "" && !strings.Contains(item["body"].(string), c.bodyHas) {
				t.Fatalf("body %q thiếu %q", item["body"], c.bodyHas)
			}
			if strings.Contains(item["body"].(string), "429") || strings.Contains(item["body"].(string), "commit_chapter") {
				t.Fatalf("body lộ nội dung kỹ thuật: %q", item["body"])
			}
			if got := actionIDs(item); strings.Join(got, ",") != strings.Join(c.actions, ",") {
				t.Fatalf("actions = %v, muốn %v", got, c.actions)
			}
			if chapter, _ := item["chapter"].(float64); chapter != c.wantChap {
				t.Fatalf("chapter = %v, muốn %v", chapter, c.wantChap)
			}
			if job, _ := item["job"].(string); job != c.wantJob {
				t.Fatalf("job = %q, muốn %q", job, c.wantJob)
			}
			link := item["url"].(string)
			if c.urlCheck != nil {
				c.urlCheck(link)
			}
			if c.urlPrefix != "" && link != c.urlPrefix {
				t.Fatalf("url = %q, muốn %q", link, c.urlPrefix)
			}
		})
	}
	// Rewrite trỏ về cùng liên kết sâu mã hoá; người chỉ đọc không nhận thao tác ghi.
	gate := find(items, "gate", "")
	rewrite := gate["actions"].([]any)[1].(map[string]any)
	routeResolves(t, rewrite["url"].(string), "read", "p/s", "25")
	expectQuery(t, rewrite["url"].(string), "rewrite=1")
	readerGate := find(inboxOf(t, g, readerCookie), "gate", "")
	if readerGate == nil || strings.Join(actionIDs(readerGate), ",") != "mute" {
		t.Fatalf("người chỉ đọc phải chỉ có thao tác tắt loại thông báo, nhận %v", readerGate)
	}
}

func expectQuery(t *testing.T, link, want string) {
	t.Helper()
	if _, q, _ := strings.Cut(link, "?"); !strings.Contains(q, want) {
		t.Fatalf("%s thiếu %s", link, want)
	}
}

func TestNotificationEventReplayIsDeduped(t *testing.T) {
	g, _ := newAuthGateway(t)
	cookie := authLogin(t, g, "writer", "writer-pass")
	for range 3 {
		g.events().Publish(Event{Type: "gate.waiting", Project: "p", Story: "s", Data: map[string]any{"chapter": 25}})
		g.events().Publish(Event{Type: "job.done", Project: "p", Story: "s", Data: map[string]any{"kind": "import", "id": "p/s/import/1", "unit": map[string]any{"done": 3}}})
	}
	items := inboxOf(t, g, cookie)
	gates, imports := 0, 0
	for _, item := range items {
		switch item["type"] {
		case "gate":
			gates++
		case "import":
			imports++
		}
	}
	if gates != 1 || imports != 1 {
		t.Fatalf("phát lại sự kiện tạo %d cổng và %d lượt nhập, muốn 1 và 1", gates, imports)
	}
}

func TestFriendlyModelError(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"POST https://openrouter.ai/api/v1: 429 Too Many Requests", "Nhà cung cấp model đã hết hạn mức hoặc đang giới hạn tốc độ"},
		{"insufficient_quota: You exceeded your current quota", "Nhà cung cấp model đã hết hạn mức hoặc đang giới hạn tốc độ"},
		{"401 Unauthorized: invalid api key", "Khoá API không hợp lệ hoặc đã hết hạn"},
		{"context deadline exceeded", "Model phản hồi quá lâu"},
		{"503 Service Unavailable", "Dịch vụ model đang quá tải hoặc tạm ngưng"},
		{"dial tcp: lookup api.example: no such host", "Mất kết nối tới nhà cung cấp model"},
		{"panic: runtime error: index out of range", "Model gặp lỗi khi viết. Mở truyện để xem chi tiết và chạy lại"},
		{"", "Model gặp lỗi khi viết. Mở truyện để xem chi tiết và chạy lại"},
	}
	for _, c := range cases {
		if got := friendlyModelError(c.raw); got != c.want {
			t.Errorf("friendlyModelError(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestChapterFactsText(t *testing.T) {
	cases := []struct {
		body  string
		score float64
		want  string
	}{
		{"Bản gốc trở về", 84, "Bản gốc trở về · Biên tập viên chấm 8,4"},
		{"", 84, "Biên tập viên chấm 8,4"},
	}
	for _, c := range cases {
		if got := chapterFactsText(c.body, c.score); got != c.want {
			t.Errorf("chapterFactsText(%q, %v) = %q, want %q", c.body, c.score, got, c.want)
		}
	}
}

func TestSubscribeDefaultsDeviceLabel(t *testing.T) {
	g, _ := newAuthGateway(t)
	cookie := authLogin(t, g, "writer", "writer-pass")
	w := authDo(t, g, "POST", "/api/push/subscriptions", cookie, `{"endpoint":"https://push.example/a","keys":{"p256dh":"k","auth":"a"},"device":"   "}`)
	if w.Code != 200 {
		t.Fatalf("subscribe: %d %s", w.Code, w.Body.String())
	}
	subs := g.notifications.store.Subscriptions("writer")
	if len(subs) != 1 || strings.TrimSpace(subs[0].Device) == "" {
		t.Fatalf("thiết bị không tên phải có nhãn mặc định: %+v", subs)
	}
	w = authDo(t, g, "POST", "/api/push/subscriptions", cookie, `{"endpoint":"not a url","keys":{"p256dh":"k","auth":"a"}}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"error":"Thiết lập thông báo không hợp lệ"`) {
		t.Fatalf("đăng ký sai phải 400 tiếng Việt: %d %s", w.Code, w.Body.String())
	}
}
