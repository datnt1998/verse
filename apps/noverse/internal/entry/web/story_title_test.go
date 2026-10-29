package web

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// TestProjectStoryUsesEngineBookTitle: thanh “Áp dụng cho” của Tuỳ chỉnh đọc tên truyện từ
// GET /api/projects/{p} — truyện tạo từ brief có story.json.title = dòng đầu brief, còn tên thật
// engine đặt nằm ở meta/book.json. Thẻ tóm tắt phải trả tên sách thật.
// Fail-before: storySummary chỉ thay bằng tên sách khi story.json.title trùng slug thư mục.
func TestProjectStoryUsesEngineBookTitle(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	out := saveLedgerTestStory(t, tg, "01-truyen", ledgerValues("book title"), false)
	working := "Viết kịch bản (dàn ý chi tiết) cho một truyện audio tiếng Việt, thể loại trọng sinh"
	if err := library.WriteStoryFile(filepath.Join(tg.project, "kich_ban", "01-truyen"), library.StoryFile{Title: working}); err != nil {
		t.Fatal(err)
	}
	real := "Thiên Kim Thật Đòi Lại Giọng Nói"
	if err := store.NewStore(out).Book.Save(domain.BookMetadata{Title: real, Synopsis: "Cô con gái thật trở về đòi lại giọng nói."}); err != nil {
		t.Fatal(err)
	}
	w := tg.do("GET", "/api/projects/proj", tg.login("admin", "admin-pass"), "")
	if w.Code != 200 {
		t.Fatalf("GET project: %d %s", w.Code, w.Body)
	}
	var response struct {
		Stories []storySummary `json:"stories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	var title string
	for _, story := range response.Stories {
		if story.ID == "proj/01-truyen" {
			title = story.Title
		}
	}
	if title != real {
		t.Fatalf("story title = %q, want %q: %s", title, real, w.Body)
	}
}

// metaTitle returns the title that GET /meta shows for the story.
func metaTitle(t *testing.T, tg *configTestGateway, story, cookie string) string {
	t.Helper()
	w := tg.do("GET", "/api/projects/proj/stories/"+story+"/meta", cookie, "")
	if w.Code != 200 {
		t.Fatalf("GET meta: %d %s", w.Code, w.Body)
	}
	title, _ := decodeResp(t, w)["title"].(string)
	return title
}

// TestStoryTitleSyncsFromBookJSON: truyện có story.json.title khác tên sách
// (bản ghi cũ lấy từ yêu cầu lúc tạo) và chưa từng đổi tên tay → các mặt hiển thị
// và chính story.json phải theo tên sách engine đặt. Fail-before: /meta trả title
// lưu thô và story.json không bao giờ được đồng bộ.
func TestStoryTitleSyncsFromBookJSON(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	out := saveLedgerTestStory(t, tg, "01-truyen", ledgerValues("book title"), false)
	stale := "Rules kịch bản: “Khi sếp học Vibecode” — viết kịch bản audio 8 chương"
	dir := filepath.Join(tg.project, "kich_ban", "01-truyen")
	if err := library.WriteStoryFile(dir, library.StoryFile{Title: stale}); err != nil {
		t.Fatal(err)
	}
	real := "KHI SẾP HỌC VIBECODE"
	if err := store.NewStore(out).Book.Save(domain.BookMetadata{Title: real, Synopsis: "Sếp học vibecode ba ngày."}); err != nil {
		t.Fatal(err)
	}
	cookie := tg.login("admin", "admin-pass")
	if got := metaTitle(t, tg, "01-truyen", cookie); got != real {
		t.Fatalf("GET meta title = %q, want %q", got, real)
	}
	f, err := library.LoadStoryFile(dir)
	if err != nil || f == nil || f.Title != real {
		t.Fatalf("story.json not synced: %+v %v", f, err)
	}
}

// TestStoryTitleManualRenameRespected: người dùng đổi "Tên truyện" trong Thông tin
// truyện → titleRenamed ghi nhận và đồng bộ theo meta/book.json không được ghi đè.
// Fail-before: storySummary/GET meta luôn ưu tiên tên sách, tên đổi tay bị đè.
func TestStoryTitleManualRenameRespected(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	out := saveLedgerTestStory(t, tg, "01-truyen", ledgerValues("book title"), false)
	dir := filepath.Join(tg.project, "kich_ban", "01-truyen")
	if err := store.NewStore(out).Book.Save(domain.BookMetadata{Title: "Tên Sách Engine", Synopsis: "x"}); err != nil {
		t.Fatal(err)
	}
	cookie := tg.login("admin", "admin-pass")
	w := tg.do("PUT", "/api/projects/proj/stories/01-truyen/meta", cookie,
		`{"title":"Tên Riêng Của Tôi","status":"writing","tags":["a"],"idea":"","shortTitle":""}`)
	if w.Code != 200 {
		t.Fatalf("PUT meta: %d %s", w.Code, w.Body)
	}
	if got := metaTitle(t, tg, "01-truyen", cookie); got != "Tên Riêng Của Tôi" {
		t.Fatalf("GET meta title = %q, want manual name", got)
	}
	w = tg.do("GET", "/api/projects/proj", cookie, "")
	var response struct {
		Stories []storySummary `json:"stories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	var title string
	for _, story := range response.Stories {
		if story.ID == "proj/01-truyen" {
			title = story.Title
		}
	}
	if title != "Tên Riêng Của Tôi" {
		t.Fatalf("summary title = %q, want manual name", title)
	}
	f, err := library.LoadStoryFile(dir)
	if err != nil || f == nil || !f.TitleRenamed || f.Title != "Tên Riêng Của Tôi" {
		t.Fatalf("story.json: %+v %v", f, err)
	}
	// Đổi về đúng tên sách → không còn là đổi tay, đồng bộ bật lại.
	w = tg.do("PUT", "/api/projects/proj/stories/01-truyen/meta", cookie,
		`{"title":"Tên Sách Engine","status":"writing","tags":["a"],"idea":"","shortTitle":""}`)
	if w.Code != 200 {
		t.Fatalf("PUT meta: %d %s", w.Code, w.Body)
	}
	if f, _ := library.LoadStoryFile(dir); f == nil || f.TitleRenamed {
		t.Fatalf("titleRenamed should clear when title matches book: %+v", f)
	}
}

// TestStoryTitleTagSaveKeepsSync: lưu Thông tin mà không đổi tên (tiêu đề nhận từ
// GET = tên sách) không được lật titleRenamed — truyện vẫn đồng bộ về sau.
// Fail-before: PUT mới ghi luôn titleRenamed=true khi title khác bản lưu cũ lấy từ yêu cầu.
func TestStoryTitleTagSaveKeepsSync(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	out := saveLedgerTestStory(t, tg, "01-truyen", ledgerValues("book title"), false)
	dir := filepath.Join(tg.project, "kich_ban", "01-truyen")
	real := "Tên Sách Engine"
	if err := library.WriteStoryFile(dir, library.StoryFile{Title: "Yêu cầu cũ cắt dòng đầu"}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewStore(out).Book.Save(domain.BookMetadata{Title: real, Synopsis: "x"}); err != nil {
		t.Fatal(err)
	}
	cookie := tg.login("admin", "admin-pass")
	// GET trước như UI thật: form nhận tên sách đã vá.
	if got := metaTitle(t, tg, "01-truyen", cookie); got != real {
		t.Fatalf("GET meta title = %q, want %q", got, real)
	}
	// Lưu chỉ thêm thẻ, tiêu đề gửi lại = tên sách → không phải đổi tay.
	w := tg.do("PUT", "/api/projects/proj/stories/01-truyen/meta", cookie,
		`{"title":"`+real+`","status":"writing","tags":["a"],"idea":"","shortTitle":""}`)
	if w.Code != 200 {
		t.Fatalf("PUT meta: %d %s", w.Code, w.Body)
	}
	if f, _ := library.LoadStoryFile(dir); f == nil || f.TitleRenamed {
		t.Fatalf("titleRenamed should stay false: %+v", f)
	}
}
