package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/notify"
)

// Mọi truyện trong dự án dùng chung quyền: view cho đọc, collab cho ghi,
// owner cho xoá/chia sẻ. Dự án không được gán và thao tác vượt quyền trả 403.
func TestStoryRouteAccessPolicy(t *testing.T) {
	tg := newTestGateway(t)
	stubNextWorker(t, tg.g) // 交给 worker 的路由统一返回 {"ok":true}
	reader := tg.login("reader", "reader-pass")
	writer := tg.login("writer", "writer-pass")

	gate403 := func(t *testing.T, method, path, cookie, body string) {
		t.Helper()
		if w := tg.do(method, path, cookie, body); w.Code != http.StatusForbidden {
			t.Fatalf("%s %s phải bị từ chối = %d %s", method, path, w.Code, w.Body.String())
		}
	}
	passes := func(t *testing.T, method, path, cookie, body string) {
		t.Helper()
		w := tg.do(method, path, cookie, body)
		if w.Code == http.StatusForbidden || (w.Code == http.StatusNotFound && strings.Contains(w.Body.String(), "không tìm thấy truyện")) {
			t.Fatalf("%s %s bị từ chối ngoài ý muốn = %d %s", method, path, w.Code, w.Body.String())
		}
	}

	type route struct {
		name               string
		method, path, body string
		writeOnly          bool
		denyOnly           bool // Không chạy nhánh được phép cho SSE hoặc thao tác đổi fixture.
	}
	story := "/api/projects/_/stories/oldbook"
	routes := []route{
		{"meta GET", "GET", story + "/meta", "", false, false},
		{"meta PUT", "PUT", story + "/meta", `{"status":"idea","tags":[]}`, false, false},
		{"rename", "POST", story + "/rename", `{"slug":"x"}`, false, true},
		{"close", "POST", story + "/close", `{}`, false, false},
		{"delete", "DELETE", story + "/", `{"confirm":"oldbook"}`, false, true},
		{"chapters", "GET", story + "/chapters", "", false, false},
		{"outline", "GET", story + "/outline", "", false, false},
		{"quality", "GET", story + "/quality", "", false, false},
		{"reviews", "GET", story + "/reviews/latest", "", false, false},
		{"review keep", "POST", story + "/review/items/ch1-x/keep", `{"reason":"x"}`, false, false},
		{"activity history", "GET", story + "/events", "", false, false},
		{"export", "GET", story + "/export", "", false, false},
		{"exports POST", "POST", story + "/exports", `{"format":"txt"}`, false, false},
		{"exports download", "GET", story + "/exports/download?file=x.txt", "", false, false},
		{"rules effective", "GET", story + "/rules/effective", "", false, false},
		{"rules apply", "POST", story + "/rules/apply", `{}`, false, false},
		{"rules runtime keep", "POST", story + "/rules/runtime/rt-1/keep", `{}`, false, false},
		{"frame PUT", "PUT", story + "/frame", `{"difference":"x"}`, false, false},
		{"retention", "GET", story + "/retention", "", false, false},
		{"metrics", "GET", story + "/metrics", "", false, false},
		{"address-terms PUT", "PUT", story + "/address-terms", `{"term":{}}`, false, false},
		{"relations POST", "POST", story + "/relations", `{}`, false, false},
		{"brain", "GET", story + "/brain", "", false, false},
		{"style-samples GET", "GET", story + "/style-samples", "", false, false},
		{"audio ready", "GET", story + "/audio/ready", "", false, false},
		{"audio cast PUT", "PUT", story + "/audio/cast", `{"entries":[]}`, false, false},
		{"audio events SSE", "GET", story + "/audio/events", "", false, true},
		{"comments GET", "GET", story + "/chapters/1/comments", "", false, false},
		{"comments POST", "POST", story + "/chapters/1/comments", `{"paragraph":null,"text":"x"}`, false, false},
		{"next (worker)", "POST", story + "/next", `{}`, false, false},
		{"input (worker)", "POST", story + "/input", `{"text":"x"}`, false, false},
		{"resume (worker)", "POST", story + "/resume", `{}`, false, false},
		{"live SSE (worker)", "GET", story + "/events/live", "", false, false},
		{"job control", "POST", "/api/jobs/_/oldbook/audio/1/cancel", `{}`, false, false},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			// Giữ nguyên tên truyện nhưng đổi sang dự án người dùng không được gán.
			hidden := strings.Replace(rt.path, "/_/", "/proj/", 1)
			if hidden != rt.path {
				gate403(t, rt.method, hidden, reader, rt.body)
			}
			// SSE chỉ kiểm tra nhánh từ chối để không giữ kết nối trong test.
			if rt.denyOnly {
				if rt.method != http.MethodGet || rt.writeOnly {
					gate403(t, rt.method, rt.path, reader, rt.body)
				}
				return
			}
			if rt.method == http.MethodGet && !rt.writeOnly {
				passes(t, rt.method, rt.path, reader, rt.body)
			} else {
				gate403(t, rt.method, rt.path, reader, rt.body)
			}
			// Chủ dự án được phép đi tới handler cho mọi thao tác.
			passes(t, rt.method, rt.path, writer, rt.body)
		})
	}
}

// Không tiết lộ sự tồn tại của từng truyện qua các phản hồi ghi bị từ chối
// trong cùng một dự án có quyền view.
func TestStoryGateRejectsViewOnlyWrites(t *testing.T) {
	tg := newTestGateway(t)
	stubNextWorker(t, tg.g)
	reader := tg.login("reader", "reader-pass")

	visible := tg.do("POST", "/api/projects/_/stories/oldbook/reopen", reader, `{}`)
	invisible := tg.do("POST", "/api/projects/_/stories/secret/reopen", reader, `{}`)
	missing := tg.do("POST", "/api/projects/_/stories/khong-co/reopen", reader, `{}`)

	if visible.Code != http.StatusForbidden {
		t.Fatalf("Ghi khi chỉ có quyền xem = %d: %s", visible.Code, visible.Body.String())
	}
	if invisible.Code != visible.Code || invisible.Body.String() != visible.Body.String() {
		t.Fatalf("不可见与不可写响应必须一致: %d %q vs %d %q",
			invisible.Code, invisible.Body.String(), visible.Code, visible.Body.String())
	}
	if missing.Code != visible.Code || missing.Body.String() != visible.Body.String() {
		t.Fatalf("不存在与不可写响应必须一致: %d %q vs %d %q",
			missing.Code, missing.Body.String(), visible.Code, visible.Body.String())
	}
}

// TestNotifyApproveGateHides 覆盖通知 approve 动作（notify 转 /next）的同一规则：
// gate 通知存在但用户对该故事没有写权限 → 404，与通知/故事不存在不可分辨。
func TestNotifyApproveGateHides(t *testing.T) {
	tg := newTestGateway(t)
	stubNextWorker(t, tg.g)
	reader := tg.login("reader", "reader-pass")

	if err := tg.g.notifications.store.Record("reader", notify.PushNotification{
		Type: "gate", Story: "_/oldbook", Chapter: 1,
		Title: "Chương 1 chờ duyệt", URL: "/x",
		Actions: []notify.Action{{Action: "approve", Title: "Duyệt"}},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	items, _, _ := tg.g.notifications.store.Inbox("reader", "")
	if len(items) == 0 {
		t.Fatal("inbox rỗng")
	}
	if w := tg.do("POST", "/api/notifications/"+items[0].ID+"/action", reader, `{"action":"approve"}`); w.Code != http.StatusNotFound {
		t.Fatalf("reader approve = %d（改动前是 403）: %s", w.Code, w.Body.String())
	}
}
