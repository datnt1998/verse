package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/gofrs/flock"
)

func TestCommentsGatewayCRUDPermissionAndTUILock(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	tg.g.launch = func(string, string, string, string) (*launchedWorker, error) {
		t.Error("comments must not launch worker")
		return nil, errors.New("worker forbidden")
	}
	lock := flock.New(filepath.Join(tg.root, "oldbook", "output", "novel", ".ainovel.lock"))
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	reader := tg.login("reader", "reader-pass")
	writer := tg.login("writer", "writer-pass")
	sub, _, _ := tg.g.events().subscribe(adminUser(), 0, nil)
	defer tg.g.events().unsubscribe(sub)
	path := "/api/projects/_/stories/oldbook/chapters/1/comments"
	if w := tg.do("GET", path, "", ""); w.Code != 401 {
		t.Fatalf("unauthenticated = %d", w.Code)
	}
	if w := tg.do("GET", "/api/projects/proj/stories/secret/chapters/1/comments", reader, ""); w.Code != 403 {
		t.Fatalf("unreadable = %d", w.Code)
	}
	if w := tg.do("POST", path, reader, `{"paragraph":null,"text":"Thêm thoại"}`); w.Code != 403 {
		t.Fatalf("reader write = %d: %s", w.Code, w.Body)
	}
	w := tg.do("POST", path, writer, `{"paragraph":0,"quote":"Đoạn mở đầu","chips":["Ngắn lại"],"text":"Giữ ý chính"}`)
	if w.Code != 201 {
		t.Fatalf("create = %d: %s", w.Code, w.Body)
	}
	select {
	case ev := <-sub.C:
		if ev.Type != "story.changed" || ev.Meta.(Event).Project != "_" || ev.Meta.(Event).Story != "oldbook" {
			t.Fatalf("comment event = %+v", ev)
		}
		parts := ev.Data.(map[string]any)["parts"].([]string)
		if len(parts) != 1 || parts[0] != "comments" {
			t.Fatalf("comment parts = %v", parts)
		}
	default:
		t.Fatal("comment write did not publish story.changed")
	}
	var c comments.Comment
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if c.ID == "" || c.Author != "writer" || c.Paragraph == nil || *c.Paragraph != 0 {
		t.Fatalf("comment = %+v", c)
	}
	w = tg.do("GET", path, reader, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), c.ID) {
		t.Fatalf("read = %d: %s", w.Code, w.Body)
	}
	if w := tg.do("DELETE", path+"?id="+c.ID, reader, ""); w.Code != 403 {
		t.Fatalf("reader delete = %d", w.Code)
	}
	if w := tg.do("DELETE", path+"?id="+c.ID, writer, ""); w.Code != 200 {
		t.Fatalf("delete = %d: %s", w.Code, w.Body)
	}
	w = tg.do("GET", path, reader, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"comments":[]`) {
		t.Fatalf("after delete = %d: %s", w.Code, w.Body)
	}
	if w := tg.do("POST", path, writer, `{"text":"x","author":"admin"}`); w.Code != 400 {
		t.Fatalf("forged author = %d", w.Code)
	}
}

type commentApprovalHost struct {
	*liveFakeHost
	chapter     int
	instruction string
	err         error
}

func (h *commentApprovalHost) AdvanceOneChapterWithComments(chapter int, instruction string) error {
	h.chapter, h.instruction = chapter, instruction
	return h.err
}

func TestCommentsNextDispatchAndAppliedMarking(t *testing.T) {
	h := &commentApprovalHost{liveFakeHost: newLiveFakeHost(), err: errors.New("cổng chưa sẵn sàng")}
	s := newLiveServer(h.liveFakeHost)
	s.h = h
	s.storyDir = t.TempDir()
	store := comments.New(s.storyDir)
	if _, err := store.Add(2, comments.Input{Text: "Giữ giọng văn"}, "writer"); err != nil {
		t.Fatal(err)
	}
	w := doReq(t, s, http.MethodPost, "/api/next", `{"applyComments":true,"chapter":2}`)
	if w.Code != 409 {
		t.Fatalf("rejected next = %d: %s", w.Code, w.Body)
	}
	list, _ := store.List(2)
	if list[0].Status != comments.StatusPending || list[0].SentAt != nil {
		t.Fatal("rejected next applied feedback")
	}
	h.err = nil
	w = doReq(t, s, http.MethodPost, "/api/next", `{"applyComments":true,"chapter":2}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"appliedComments":1`) {
		t.Fatalf("next = %d: %s", w.Code, w.Body)
	}
	if h.chapter != 2 || !strings.Contains(h.instruction, "Giữ giọng văn") {
		t.Fatalf("Writer dispatch = %d %q", h.chapter, h.instruction)
	}
	list, _ = store.List(2)
	if list[0].Status != comments.StatusSent || list[0].SentAt == nil {
		t.Fatal("successful next did not mark feedback sent")
	}
}

func TestCommentsGatewayPatchActions(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	w := tg.do("POST", "/api/projects/proj/stories", admin, `{"name":"Truyện Góp Ý"}`)
	if w.Code != 201 {
		t.Fatalf("create story: %d %s", w.Code, w.Body.String())
	}
	storyDir := mkdirAll(t, tg.root, "proj", "kich_ban", "01-truyen-gop-y")
	engine := mkdirAll(t, storyDir, "output", "novel", "chapters")
	if err := os.WriteFile(filepath.Join(engine, "01.md"), []byte("Đoạn một nguyên văn.\n\nCâu lặp y hệt.\n\nĐoạn giữa.\n\nCâu lặp y hệt."), 0o644); err != nil {
		t.Fatal(err)
	}
	s := comments.New(storyDir)
	comment, err := s.Add(1, comments.Input{Paragraph: ptr(1), Quote: "Câu lặp y hệt.", Text: "Sửa chỗ này"}, "writer")
	if err != nil {
		t.Fatal(err)
	}
	// Đọc lại để trạng thái thành không chắc (hai đoạn cùng trích).
	list, err := s.List(1)
	if err != nil || len(list) != 1 || list[0].Status != comments.StatusUnsure {
		t.Fatalf("setup unsure = %+v %v", list, err)
	}
	base := "/api/projects/proj/stories/01-truyen-gop-y/chapters/1/comments/" + comment.ID
	w = tg.do("PATCH", base, admin, `{"action":"relink","paragraph":3,"quote":"Câu lặp y hệt."}`)
	if w.Code != 200 {
		t.Fatalf("relink = %d %s", w.Code, w.Body.String())
	}
	got := decodeJSONBody(t, w)
	anchor, _ := got["anchor"].(map[string]any)
	if got["status"] != comments.StatusPending || anchor == nil || anchor["paragraph"] != float64(3) {
		t.Fatalf("relinked = %v", got)
	}
	if w = tg.do("PATCH", base, admin, `{"action":"resolve"}`); w.Code != 200 || decodeJSONBody(t, w)["status"] != comments.StatusDone {
		t.Fatalf("resolve = %d %s", w.Code, w.Body.String())
	}
	if w = tg.do("PATCH", base, admin, `{"action":"reopen"}`); w.Code != 200 || decodeJSONBody(t, w)["status"] != comments.StatusPending {
		t.Fatalf("reopen = %d %s", w.Code, w.Body.String())
	}
	if w = tg.do("PATCH", base, admin, `{"action":"resend_whole"}`); w.Code != 200 || decodeJSONBody(t, w)["whole"] != true {
		t.Fatalf("resend_whole = %d %s", w.Code, w.Body.String())
	}
	if w = tg.do("PATCH", base, admin, `{"action":"xoa"}`); w.Code != 400 || !strings.Contains(w.Body.String(), "thao tác") {
		t.Fatalf("bad action = %d %s", w.Code, w.Body.String())
	}
	if w = tg.do("PATCH", base+"-khong-co", admin, `{"action":"resolve"}`); w.Code != 404 {
		t.Fatalf("missing = %d %s", w.Code, w.Body.String())
	}
}

func ptr(v int) *int { return &v }
