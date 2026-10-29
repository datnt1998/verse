package web

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/store"
)

// recordWithBuild 直接写一条章节记录，避免依赖全局 version.SetBuild。
func recordWithBuild(t *testing.T, s *store.Store, chapter int, build string, acceptedAt time.Time) {
	t.Helper()
	content := domain.NormalizeChapterContent("nội dung chương")
	rec := domain.ChapterRecord{
		Version:       domain.ChapterRecordVersion,
		Chapter:       chapter,
		Revision:      1,
		Build:         build,
		Origin:        domain.ChapterOriginGenerated,
		Content:       content,
		ContentSHA256: domain.ChapterContentSHA256(content),
		AcceptedAt:    acceptedAt,
	}
	if err := s.ChapterRecords.Save(rec); err != nil {
		t.Fatalf("save record %d: %v", chapter, err)
	}
}

// D1：story meta GET trả builds theo từng khoảng chương, gộp các chương liền nhau
// cùng một bản build; version tách khỏi commit; chương thiếu bản ghi → "".
func TestStoryMetaBuilds(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1, 2, 3, 4})
	recordWithBuild(t, s, 1, "v0.9.3+aaa1111", time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC))
	recordWithBuild(t, s, 2, "v0.9.3+aaa1111", time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC))
	recordWithBuild(t, s, 3, "v0.9.4+bbb2222", time.Date(2026, 10, 24, 14, 2, 0, 0, time.UTC))
	// chương 4 không có bản ghi → không rõ bản build.

	req := httptest.NewRequest("GET", "/x/meta", nil)
	rec := httptest.NewRecorder()
	g.serveStoryMeta(rec, req, library.FileRepository(&library.Library{}), adminUser(), st)
	if rec.Code != 200 {
		t.Fatalf("meta: %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Status string            `json:"status"`
		Builds []StoryBuildRange `json:"builds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status == "" {
		t.Fatalf("meta lost the story fields: %s", rec.Body)
	}
	want := []StoryBuildRange{
		// at lấy từ chương đầu của khoảng (chương 2 cùng bản build nên không đổi ngày).
		{From: 1, To: 2, Version: "0.9.3", Commit: "aaa1111", At: "2026-10-20T09:00:00Z"},
		{From: 3, To: 3, Version: "0.9.4", Commit: "bbb2222", At: "2026-10-24T14:02:00Z"},
		{From: 4, To: 4, Version: "", Commit: "", At: ""},
	}
	if len(body.Builds) != len(want) {
		t.Fatalf("builds = %+v, want %+v", body.Builds, want)
	}
	for i := range want {
		if body.Builds[i] != want[i] {
			t.Fatalf("builds[%d] = %+v, want %+v", i, body.Builds[i], want[i])
		}
	}

	// Truyện chưa viết chương nào → mảng rỗng (không null).
	empty, _ := storyFixture(t, nil)
	got := storyBuilds(host.NewStoryReader(empty.Dir))
	if got == nil || len(got) != 0 {
		t.Fatalf("empty story builds = %#v", got)
	}
}

// D1：build 戳拆分 — 无 "+commit"、commit 为 "unknown"（dev 构建）、超长哈希。
func TestSplitBuild(t *testing.T) {
	cases := map[string]struct{ version, commit string }{
		"v0.9.3+aaa1111":      {"0.9.3", "aaa1111"},
		"v0.9.3":              {"0.9.3", ""},
		"dev+unknown":         {"dev", ""},
		"v1.0.0+0123456789ab": {"1.0.0", "0123456"},
		"":                    {"", ""},
	}
	for raw, want := range cases {
		version, commit := splitBuild(raw)
		if version != want.version || commit != want.commit {
			t.Fatalf("splitBuild(%q) = %q, %q; want %q, %q", raw, version, commit, want.version, want.commit)
		}
	}
}
