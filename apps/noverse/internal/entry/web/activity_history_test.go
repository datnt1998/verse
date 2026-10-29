package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/gofrs/flock"
)

func activityFixture(t *testing.T) (*testGateway, string, string) {
	t.Helper()
	tg := newTestGateway(t)
	t.Cleanup(tg.g.Close)
	path := filepath.Join(mkdirAll(t, tg.root, "oldbook", "output", "novel", "logs"), "tui.log")
	events := []host.Event{
		{ID: "a", Time: time.Unix(100, 0), Category: "TOOL", Agent: "writer", Chapter: 1, Summary: "Viết chương đầu", Tool: "draft_chapter"},
		{ID: "b", Time: time.Unix(101, 0), Category: "TOOL", Agent: "editor", Chapter: 2, Summary: "Đã đọc bản nháp", Level: "error", Failed: true},
		{ID: "c", Time: time.Unix(102, 0), Category: "USER", Agent: "user", Chapter: 2, Username: "reader", Summary: "Đổi hướng đi"},
		{ID: "d", Time: time.Unix(103, 0), Category: "USER", Agent: "user", Chapter: 2, Username: "admin", Summary: "Giữ nguyên"},
	}
	for _, ev := range events {
		appendActivityFixture(t, path, ev)
	}
	return tg, tg.login("reader", "reader-pass"), path
}
func appendActivityFixture(t *testing.T, path string, ev host.Event) {
	t.Helper()
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	slog.New(slog.NewTextHandler(f, nil)).Info(ev.Summary, "module", "event", "session", "test-process", "event_data", string(raw))
}
func activityGet(t *testing.T, tg *testGateway, cookie, query string) activityHistory {
	t.Helper()
	rec := tg.do("GET", "/api/projects/_/stories/oldbook/events"+query, cookie, "")
	if rec.Code != 200 {
		t.Fatalf("history %d %s", rec.Code, rec.Body.String())
	}
	var data activityHistory
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	return data
}
func TestActivityHistoryFilters(t *testing.T) {
	tg, cookie, _ := activityFixture(t)
	for _, tc := range []struct{ q, id string }{{"?chapter=1", "a"}, {"?agent=editor", "b"}, {"?level=error", "b"}, {"?mine=1", "c"}, {"?chapter=2&agent=editor&level=error", "b"}} {
		got := activityGet(t, tg, cookie, tc.q)
		if len(got.Events) != 1 || got.Events[0].ID != tc.id {
			t.Fatalf("filter %s: %+v", tc.q, got.Events)
		}
	}
	counts := activityGet(t, tg, cookie, "?level=error").Counts
	if counts.Total != 4 || counts.Errors != 1 || counts.Mine != 1 || counts.ByAgent["writer"] != 1 {
		t.Fatalf("chip counts: %+v", counts)
	}
	if got := tg.do("GET", "/api/projects/proj/stories/secret/events", cookie, ""); got.Code != 403 {
		t.Fatalf("permission: %d", got.Code)
	}
}
func TestActivityHistoryPagingStableAppend(t *testing.T) {
	tg, cookie, path := activityFixture(t)
	lock := flock.New(filepath.Join(tg.root, "oldbook", "output", "novel", ".ainovel.lock"))
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer lock.Unlock()
	first := activityGet(t, tg, cookie, "?limit=2")
	if len(first.Events) != 2 || first.NextCursor == nil {
		t.Fatalf("first page: %+v", first)
	}
	appendActivityFixture(t, path, host.Event{ID: "new", Time: time.Unix(104, 0), Summary: "Sự kiện mới"})
	// A completion for an existing call must not change the frozen page either.
	appendActivityFixture(t, path, host.Event{ID: "a", Time: time.Unix(100, 0), Summary: "Hoàn thành", FinishedAt: time.Unix(105, 0)})
	second := activityGet(t, tg, cookie, "?limit=2&before="+url.QueryEscape(*first.NextCursor))
	if len(second.Events) != 2 || second.Events[0].ID != "b" || second.Events[1].ID != "a" || second.Events[1].Summary != "Viết chương đầu" || second.NextCursor != nil {
		t.Fatalf("unstable second page: %+v", second)
	}
	fresh := activityGet(t, tg, cookie, "")
	if len(fresh.Events) != 5 || fresh.Events[0].ID != "new" {
		t.Fatalf("fresh page: %+v", fresh)
	}
	if rec := tg.do("GET", "/api/projects/_/stories/oldbook/events?agent=writer&before="+url.QueryEscape(*first.NextCursor), cookie, ""); rec.Code != 400 {
		t.Fatalf("foreign filter cursor: %d", rec.Code)
	}
}
func TestActivityHistoryDiacriticsAndLegacy(t *testing.T) {
	tg, cookie, path := activityFixture(t)
	got := activityGet(t, tg, cookie, "?q="+url.QueryEscape("DA DOC BAN NHAP"))
	if len(got.Events) != 1 || got.Events[0].ID != "b" {
		t.Fatalf("diacritics search: %+v", got.Events)
	}
	got = activityGet(t, tg, cookie, "?q=doi+huong+di")
	if len(got.Events) != 1 || got.Events[0].ID != "c" {
		t.Fatalf("đ search: %+v", got.Events)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, `time=2026-10-20T12:00:00Z level=INFO msg="Đọc chương 3" session=old module=event category=TOOL agent=writer event_id=e1 state=completed duration=2s`)
	fmt.Fprint(f, `time=2026-10-20T12:00:01Z level=INFO msg="unfinished`)
	f.Close()
	got = activityGet(t, tg, cookie, "?q=doc+chuong+3")
	if len(got.Events) != 1 || got.Events[0].Chapter != 3 || got.Events[0].DurationMs != 2000 || got.Events[0].Status != "ok" {
		t.Fatalf("legacy: %+v", got.Events)
	}
}

// E2EM-10 / E2EM-4: dòng tui.log của bản cũ (tiếng Trung, đường dẫn tuyệt đối) phải ra Nhật ký
// bằng tiếng Việt và đường dẫn tương đối thư mục truyện / thư viện.
func TestActivityHistoryLegacyChineseAndAbsolutePaths(t *testing.T) {
	tg, cookie, path := activityFixture(t)
	story := filepath.Join(tg.root, "oldbook")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	for i, msg := range []string{
		"Xuất file thất bại: 写入失败：rename " + story + ".tmp-3539959194 " + story + ": file exists",
		"Xuất file thất bại: 尚无已完成章节，无内容可导出",
		"Xuất file thất bại: 文件已存在：~" + tg.root + "（添加 --overwrite 覆盖）",
		"✓ Đã xuất 8 chương / 81.6 KB đến " + story + "/Trà Xanh.txt",
		// Thư viện đã chuyển chỗ sau khi ghi log: tiền tố cũ không còn là root hiện tại.
		"✓ Đã xuất 2 chương / 1.0 KB đến /Users/khac/thu-vien-cu/oldbook/xuat-ban/a.epub",
	} {
		fmt.Fprintf(f, "time=2026-10-24T20:5%d:00Z level=ERROR msg=%q session=old module=event category=ERROR\n", i, msg)
	}
	fmt.Fprintln(f, `time=2026-10-24T21:00:00Z level=INFO msg="commit_chapter(第8章)" session=old module=event category=TOOL agent=writer event_id=t8 state=completed duration=14.7s`)
	f.Close()

	rec := tg.do("GET", "/api/projects/_/stories/oldbook/events?limit=20", cookie, "")
	if rec.Code != 200 {
		t.Fatalf("history %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if loc := regexp.MustCompile(`[\x{4e00}-\x{9fff}]`).FindStringIndex(body); loc != nil {
		t.Fatalf("Nhật ký còn chữ Trung Quốc: …%s…", body[max(0, loc[0]-80):min(len(body), loc[1]+80)])
	}
	if strings.Contains(body, tg.root) || strings.Contains(body, "/Users/khac") {
		t.Fatalf("Nhật ký lộ đường dẫn tuyệt đối: %s", body)
	}
	var got activityHistory
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	summaries := map[string]activityEvent{}
	for _, ev := range got.Events {
		summaries[ev.Summary] = ev
	}
	for _, want := range []string{
		"Xuất file thất bại: ghi tệp thất bại: đã có thư mục hoặc tệp trùng tên oldbook",
		"Xuất file thất bại: chưa có chương hoàn thành nào để xuất",
		"Xuất file thất bại: tệp đã tồn tại: " + filepath.Base(tg.root) + " (dùng --overwrite để ghi đè)",
		"✓ Đã xuất 8 chương / 81.6 KB đến Trà Xanh.txt",
		"✓ Đã xuất 2 chương / 1.0 KB đến xuat-ban/a.epub",
	} {
		if _, ok := summaries[want]; !ok {
			t.Errorf("thiếu dòng %q trong %+v", want, got.Events)
		}
	}
	if ev, ok := summaries["commit_chapter(chương 8)"]; !ok || ev.Tool != "commit_chapter(chương 8)" || ev.Chapter != 8 {
		t.Errorf("nhãn công cụ cũ chưa đổi sang tiếng Việt: %+v", got.Events)
	}
}

// Opt-in real-disk curl smoke; BLOG_SMOKE_LOG must point to a copied real story.
func TestActivityHistoryRealLogCurl(t *testing.T) {
	source := os.Getenv("BLOG_SMOKE_LOG")
	if source == "" {
		t.Skip("set BLOG_SMOKE_LOG to copied real tui.log")
	}
	tg := newTestGateway(t)
	t.Cleanup(tg.g.Close)
	log, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(mkdirAll(t, tg.root, "oldbook", "output", "novel", "logs"), "tui.log")
	if err = os.WriteFile(path, log, 0600); err != nil {
		t.Fatal(err)
	}
	cookie := tg.login("reader", "reader-pass")
	server := httptest.NewServer(tg.g)
	defer server.Close()
	command := exec.Command("curl", "--fail-with-body", "--silent", "--show-error", "--cookie", sessionCookie+"="+cookie, server.URL+"/api/projects/_/stories/oldbook/events?limit=3")
	body, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("curl: %v %s", err, body)
	}
	var got activityHistory
	if err = json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Events) == 0 {
		t.Fatalf("real legacy log returned no events: %s", body)
	}
	if bytes.Contains(body, []byte("event_data")) {
		t.Fatal("leaked storage envelope")
	}
	t.Logf("curl real legacy log: events=%d total=%d nextCursor=%t first=%s", len(got.Events), got.Counts.Total, got.NextCursor != nil, strings.TrimSpace(got.Events[0].Summary))
}

func TestActivityHistoryWatcherFreshness(t *testing.T) {
	tg, _, path := activityFixture(t)
	sub, _, _ := tg.g.events().subscribe(adminUser(), 0, []string{"_/oldbook"})
	defer tg.g.events().unsubscribe(sub)
	watcher := &fileWatcher{g: tg.g, previous: map[string]string{}}
	watcher.scan(false)
	appendActivityFixture(t, path, host.Event{ID: "fresh", Time: time.Now(), Summary: "Mới ghi từ TUI"})
	watcher.scan(true)
	select {
	case ev := <-sub.ch:
		parts := ev.Data.(map[string]any)["parts"].([]string)
		if ev.Type != "story.changed" || !strings.Contains(strings.Join(parts, ","), "activity") {
			t.Fatalf("activity invalidation: %+v", ev)
		}
	default:
		t.Fatal("TUI log append did not invalidate activity history")
	}
}

// M6 Nhật ký: khối công cụ mở ra cần đủ {tool,args,result,durationMs,status,retries[]}; lọc Lỗi
// gồm cả lỗi đã tự hồi phục; bộ lọc + từ khoá + before dùng chung một con trỏ ổn định.
func TestActivityHistoryToolDetailAndMobileFilters(t *testing.T) {
	tg, cookie, path := activityFixture(t)
	appendActivityFixture(t, path, host.Event{
		ID: "t1", Time: time.Unix(110, 0), FinishedAt: time.Unix(112, 300_000_000), Category: "TOOL", Agent: "writer", Chapter: 3,
		Summary: "Model trả lỗi 429 khi soát nhất quán", Tool: "check_consistency", Args: `{"chapter":3}`, Result: "429 Too Many Requests",
		Duration: 2300 * time.Millisecond, Status: "ok", Retries: []host.EventRetry{{AtSec: 0.5, Error: "429", WaitSec: 8, OK: true}},
	})
	appendActivityFixture(t, path, host.Event{ID: "t2", Time: time.Unix(111, 0), Category: "TOOL", Agent: "writer", Chapter: 3, Summary: "Lưu chương 3", Tool: "save_chapter", Args: `{"chapter":3}`, Status: "ok", Duration: 200 * time.Millisecond})
	appendActivityFixture(t, path, host.Event{ID: "t3", Time: time.Unix(112, 0), Category: "TOOL", Agent: "writer", Chapter: 3, Summary: "Lưu chương 3 lỗi", Tool: "save_chapter", Args: strings.Repeat("ạ", 500), Result: "ENOSPC", Status: "error", Failed: true, Level: "error"})

	got := activityGet(t, tg, cookie, "?chapter=3&agent=writer&level=error")
	if len(got.Events) != 2 || got.Events[0].ID != "t3" || got.Events[1].ID != "t1" {
		t.Fatalf("error filter must keep failed and recovered-after-retry calls: %+v", got.Events)
	}
	if got.Counts.Total != 3 || got.Counts.Errors != 2 || got.Chapters[0].Chapter != 3 || got.Chapters[0].EventCount != 2 {
		t.Fatalf("counts/chapters: %+v %+v", got.Counts, got.Chapters)
	}
	ev := got.Events[1]
	if ev.Tool != "check_consistency" || ev.Args != `{"chapter":3}` || ev.Result != "429 Too Many Requests" || ev.DurationMs != 2300 || ev.Status != "ok" ||
		len(ev.Retries) != 1 || ev.Retries[0].WaitSec != 8 || !ev.Retries[0].OK || ev.Retries[0].Error != "429" {
		t.Fatalf("tool detail lost in history: %+v", ev)
	}
	if running := activityGet(t, tg, cookie, "?q=luu+chuong+3&limit=1"); len(running.Events) != 1 || running.Events[0].ID != "t3" || running.NextCursor == nil {
		t.Fatalf("q+limit: %+v", running)
	} else if next := activityGet(t, tg, cookie, "?q=luu+chuong+3&limit=1&before="+url.QueryEscape(*running.NextCursor)); len(next.Events) != 1 || next.Events[0].ID != "t2" || next.NextCursor != nil {
		t.Fatalf("q+before: %+v", next)
	}
	if none := activityGet(t, tg, cookie, "?chapter=3&mine=1"); len(none.Events) != 0 || none.Events == nil {
		t.Fatalf("mine without own events must be [] not null: %+v", none.Events)
	}
}
