package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// GAP-35：văn mẫu sống trong cùng thư mục /simulate đọc, tối đa 20 mẫu,
// chỉ admin/người có quyền viết, và không lộ tên thư mục.
func TestStyleSamplesLifecycle(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1})
	dir := filepath.Join(st.Dir, styleSamplesDir)

	empty := storyReq(t, g, "GET", "/x/style-samples", adminUser(), st, "/style-samples", "")
	if empty.Code != 200 {
		t.Fatalf("list empty: %d %s", empty.Code, empty.Body)
	}
	var listed struct {
		Samples []StyleSample     `json:"samples"`
		Total   StyleSampleTotals `json:"total"`
	}
	if err := json.Unmarshal(empty.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Samples) != 0 || listed.Total.Count != 0 || listed.Total.Chars != 0 {
		t.Fatalf("empty list = %+v", listed)
	}
	if strings.Contains(empty.Body.String(), styleSamplesDir) {
		t.Fatalf("list leaks the corpus folder name: %s", empty.Body)
	}

	created := storyReq(t, g, "POST", "/x/style-samples", adminUser(), st, "/style-samples", `{"title":"Giọng của tôi","text":"Trời mưa rất to, con hẻm vắng tanh."}`)
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var sample StyleSample
	if err := json.Unmarshal(created.Body.Bytes(), &sample); err != nil {
		t.Fatal(err)
	}
	if sample.Title != "Giọng của tôi" || sample.Chars != 8 || sample.ID == "" || sample.AddedAt == "" {
		t.Fatalf("created sample = %+v", sample)
	}
	// 与 /simulate 共用语料目录：文件必须真的落在那里，且 /simulate 的扫描器认这个后缀。
	raw, err := os.ReadFile(filepath.Join(dir, sample.ID))
	if err != nil {
		t.Fatalf("sample not written to the /simulate corpus: %v", err)
	}
	if !strings.Contains(string(raw), "con hẻm vắng tanh") {
		t.Fatalf("sample content = %q", raw)
	}
	if strings.Contains(created.Body.String(), styleSamplesDir) {
		t.Fatalf("create leaks the corpus folder name: %s", created.Body)
	}

	again := storyReq(t, g, "GET", "/x/style-samples", adminUser(), st, "/style-samples", "")
	if err := json.Unmarshal(again.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Samples) != 1 || listed.Samples[0].Title != "Giọng của tôi" || listed.Total.Chars != 8 {
		t.Fatalf("list after create = %+v", listed)
	}

	// Thiếu nội dung → 400.
	for _, body := range []string{`{"title":"Rỗng","text":"   "}`, `{"title":"Rỗng"}`} {
		if rec := storyReq(t, g, "POST", "/x/style-samples", adminUser(), st, "/style-samples", body); rec.Code != 400 {
			t.Fatalf("%s: want 400, got %d %s", body, rec.Code, rec.Body)
		}
	}

	// Tiêu đề có dấu / không được tạo thư mục con trong ngữ liệu.
	slash := storyReq(t, g, "POST", "/x/style-samples", adminUser(), st, "/style-samples", `{"title":"nhóm/một","text":"Một đoạn văn."}`)
	if slash.Code != 201 {
		t.Fatalf("slash title: %d %s", slash.Code, slash.Body)
	}
	var slashSample StyleSample
	if err := json.Unmarshal(slash.Body.Bytes(), &slashSample); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(slashSample.ID, `/\`) || !strings.HasPrefix(slashSample.ID, "nhóm-một") {
		t.Fatalf("unsafe sample id = %q", slashSample.ID)
	}
	if _, err := os.Stat(filepath.Join(dir, "nhóm")); !os.IsNotExist(err) {
		t.Fatalf("title created a subdirectory: %v", err)
	}
	if rec := storyReq(t, g, "DELETE", "/x/style-samples/"+url.PathEscape(slashSample.ID), adminUser(), st, "/style-samples/"+slashSample.ID, ""); rec.Code != 204 {
		t.Fatalf("cleanup delete: %d %s", rec.Code, rec.Body)
	}

	// Tải .txt lên cũng được (multipart), tiêu đề lấy từ tên tệp.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "Mẫu hai.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("Gió lạnh thổi qua sân."))
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/x/style-samples", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	if !g.serveStoryRead(rec, req, adminUser(), st, "/style-samples") {
		t.Fatal("serveStoryRead returned false")
	}
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	var uploaded StyleSample
	if err := json.Unmarshal(rec.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.Title != "Mẫu hai" {
		t.Fatalf("upload title = %q", uploaded.Title)
	}

	// Xoá: 204, tệp biến mất, danh sách còn 1.
	del := func(id string) *httptest.ResponseRecorder {
		t.Helper()
		return storyReq(t, g, "DELETE", "/x/style-samples/"+url.PathEscape(id), adminUser(), st, "/style-samples/"+id, "")
	}
	if rec := del(uploaded.ID); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(dir, uploaded.ID)); !os.IsNotExist(err) {
		t.Fatalf("delete kept the file: %v", err)
	}
	// Xoá lần hai → 404.
	if rec := del(uploaded.ID); rec.Code != 404 {
		t.Fatalf("second delete: %d %s", rec.Code, rec.Body)
	}
	// Đường dẫn thoát khỏi thư mục hoặc đuôi lạ → 404, không xoá gì.
	outside := filepath.Join(st.Dir, "ngoài.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../ngoài.txt", "..%2Fngoài.txt", "ngoài.txt", "a.json"} {
		if rec := del(id); rec.Code != 404 {
			t.Fatalf("%s: want 404, got %d %s", id, rec.Code, rec.Body)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("traversal deleted a file outside the corpus: %v", err)
	}
}

// GAP-35：mẫu thứ 21 bị từ chối 409; người chỉ đọc bị 404 ở cả ba phương thức (ẩn).
func TestStyleSamplesLimitsAndPermissions(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1})
	dir := filepath.Join(st.Dir, styleSamplesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= maxStyleSamples; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("mau-%02d.txt", i)), []byte("một hai ba"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	full := storyReq(t, g, "GET", "/x/style-samples", adminUser(), st, "/style-samples", "")
	var listed struct {
		Samples []StyleSample     `json:"samples"`
		Total   StyleSampleTotals `json:"total"`
	}
	if err := json.Unmarshal(full.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Total.Count != maxStyleSamples || len(listed.Samples) != maxStyleSamples {
		t.Fatalf("count at limit = %+v", listed.Total)
	}
	if rec := storyReq(t, g, "POST", "/x/style-samples", adminUser(), st, "/style-samples", `{"text":"một mẫu nữa"}`); rec.Code != 409 {
		t.Fatalf("21st sample: want 409, got %d %s", rec.Code, rec.Body)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != maxStyleSamples {
		t.Fatalf("rejected sample was written: %d files", len(entries))
	}

	reader := webauth.User{Username: "reader", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{st.Project: webauth.ProjectView}}
	if !reader.CanRead(st.ID) {
		t.Fatal("fixture reader cannot read the story")
	}
	// Người có quyền viết (không phải admin) vẫn dùng được: đây là “admin hoặc người viết”.
	writer := webauth.User{Username: "writer", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{st.Project: webauth.ProjectCollab}}
	if rec := storyReq(t, g, "GET", "/x/style-samples", writer, st, "/style-samples", ""); rec.Code != 200 {
		t.Fatalf("writer list: want 200, got %d %s", rec.Code, rec.Body)
	}
	if rec := storyReq(t, g, "GET", "/x/style-samples", reader, st, "/style-samples", ""); rec.Code != 200 {
		t.Fatalf("view sample list: %d %s", rec.Code, rec.Body)
	}
	for _, call := range []struct{ method, rest, body string }{
		{"POST", "/style-samples", `{"text":"lén thêm"}`},
		{"DELETE", "/style-samples/mau-01.txt", ""},
	} {
		rec := storyReq(t, g, call.method, "/x"+call.rest, reader, st, call.rest, call.body)
		if rec.Code != 404 {
			t.Fatalf("reader %s %s: want 404, got %d %s", call.method, call.rest, rec.Code, rec.Body)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "mau-01.txt")); err != nil {
		t.Fatalf("reader deleted a sample: %v", err)
	}
}
