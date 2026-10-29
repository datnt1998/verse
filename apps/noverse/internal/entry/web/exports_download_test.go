package web

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

func TestFriendlyExportName(t *testing.T) {
	one := 1
	cases := []struct {
		title  string
		volume *int
		ext    string
		want   string
	}{
		{"Trà Xanh Cướp Kịch Bản", &one, ".epub", "Trà Xanh Cướp Kịch Bản · Tập 1.epub"},
		{"Trà Xanh Cướp Kịch Bản", nil, ".txt", "Trà Xanh Cướp Kịch Bản.txt"},
		{`  A/B: "C"?  `, &one, ".txt", "A B C · Tập 1.txt"},
		{"///", nil, ".txt", "Truyện.txt"},
	}
	for _, c := range cases {
		if got := friendlyExportName(c.title, c.volume, c.ext); got != c.want {
			t.Errorf("friendlyExportName(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

func TestServeStoryRead_ExportsDownloadFriendlyName(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1, 2})
	if err := library.WriteStoryFile(st.Dir, library.StoryFile{Title: "Trà Xanh Cướp Kịch Bản", Status: "writing", Tags: []string{}}); err != nil {
		t.Fatal(err)
	}
	rec := storyReq(t, g, "POST", "/x/exports", adminUser(), st, "/exports", `{"format":"epub","from":1,"to":2}`)
	if rec.Code != 200 {
		t.Fatalf("POST exports: %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}

	rec = storyReq(t, g, "GET", "/x/exports/download?file="+url.QueryEscape(res.Path), adminUser(), st, "/exports/download", "")
	if rec.Code != 200 {
		t.Fatalf("download: %d %s", rec.Code, rec.Body)
	}
	disposition := rec.Header().Get("Content-Disposition")
	// Gói AF/RFC 6266: filename="ascii" trước, filename*=UTF-8''… sau.
	const prefix = "attachment; filename=\""
	if !strings.HasPrefix(disposition, prefix) || !strings.Contains(disposition, "filename*=UTF-8''") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}

	// Đường dẫn tính từ thư mục truyện (kết quả lệnh xuất) cũng tải được, cùng tên thân thiện.
	rec = storyReq(t, g, "GET", "/x/exports/download?file="+url.QueryEscape("xuat-ban/"+res.Path), adminUser(), st, "/exports/download", "")
	if rec.Code != 200 {
		t.Fatalf("story-relative download: %d %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Disposition"); got != disposition {
		t.Errorf("story-relative Content-Disposition = %q, want %q", got, disposition)
	}
	name, err := url.PathUnescape(disposition[strings.Index(disposition, "filename*=UTF-8''")+len("filename*=UTF-8''"):])
	if err != nil {
		t.Fatal(err)
	}
	if name != "Trà Xanh Cướp Kịch Bản · Tập 1.epub" {
		t.Errorf("friendly name = %q", name)
	}
	if strings.Contains(disposition, "test-story") || strings.Contains(disposition, "tap-1") {
		t.Errorf("disk name leaked into Content-Disposition: %q", disposition)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/epub+zip" {
		t.Errorf("Content-Type = %q", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("empty body")
	}

	// 路径穿越 / 绝对路径 / 缺参数 / 不存在：都不能 200。
	for _, evil := range []string{"../story.json", "..%2fstory.json", "/etc/passwd", "sub/../../x", "tap-1/../../story.json", ""} {
		rec = storyReq(t, g, "GET", "/x/exports/download?file="+evil, adminUser(), st, "/exports/download", "")
		if rec.Code != 400 {
			t.Errorf("file %q → %d, want 400", evil, rec.Code)
		}
	}
	rec = storyReq(t, g, "GET", "/x/exports/download?file=khong-co.txt", adminUser(), st, "/exports/download", "")
	if rec.Code != 404 {
		t.Errorf("missing file → %d, want 404", rec.Code)
	}
}

func TestServeStoryRead_ExportsDownloadReadPermission(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1})
	rec := storyReq(t, g, "POST", "/x/exports", adminUser(), st, "/exports", `{"format":"txt"}`)
	if rec.Code != 200 {
		t.Fatalf("POST exports: %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	target := "/x/exports/download?file=" + url.QueryEscape(res.Path)

	stranger := webauth.User{Username: "khach", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"khac": webauth.ProjectView}}
	if rec := storyReq(t, g, "GET", target, stranger, st, "/exports/download", ""); rec.Code != 404 {
		t.Fatalf("reader without access → %d, want 404", rec.Code)
	}
	disabled := webauth.User{Username: "tat", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{st.Project: webauth.ProjectView}, Disabled: true}
	if rec := storyReq(t, g, "GET", target, disabled, st, "/exports/download", ""); rec.Code != 404 {
		t.Fatalf("disabled user → %d, want 404", rec.Code)
	}
	reader := webauth.User{Username: "doc-gia", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{st.Project: webauth.ProjectView}}
	if rec := storyReq(t, g, "GET", target, reader, st, "/exports/download", ""); rec.Code != 200 {
		t.Fatalf("reader with access → %d %s", rec.Code, rec.Body)
	}
}
