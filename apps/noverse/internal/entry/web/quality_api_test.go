package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

type qualityTestRewrite struct {
	At    string `json:"at"`
	Cause string `json:"cause"`
}

type qualityTestResponse struct {
	Pack     *reviewPackAPI `json:"pack"`
	Sections []struct {
		Ref     string `json:"ref"`
		RefName string `json:"refName"`
	} `json:"sections"`
	Chapters []struct {
		N        int                  `json:"n"`
		Title    string               `json:"title"`
		Status   string               `json:"status"`
		Rewrites []qualityTestRewrite `json:"rewrites"`
	} `json:"chapters"`
	Cells []struct {
		Ref     string `json:"ref"`
		Ch      int    `json:"ch"`
		Error   int    `json:"error"`
		Warning int    `json:"warning"`
		Fixed   int    `json:"fixed"`
		Kept    int    `json:"kept"`
	} `json:"cells"`
	Issues []struct {
		reviewItemAPI
		Ch         int    `json:"ch"`
		State      string `json:"state"`
		KeptReason string `json:"keptReason"`
	} `json:"issues"`
}

func readQualityTest(t *testing.T, tg *testGateway, cookie, story string) qualityTestResponse {
	t.Helper()
	w := tg.do(http.MethodGet, "/api/projects/_/stories/"+story+"/quality", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("quality = %d: %s", w.Code, w.Body)
	}
	var out qualityTestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func qualityTestTotals(out qualityTestResponse) [4]int {
	var counts [4]int
	for _, cell := range out.Cells {
		counts[0] += cell.Error
		counts[1] += cell.Warning
		counts[2] += cell.Fixed
		counts[3] += cell.Kept
	}
	return counts
}

func TestQualityRoundsCountsAndSummary(t *testing.T) {
	tg := newTestGateway(t)
	s, writer := gateStory(t, tg)
	first := errorReview(1)
	first.Issues = append(first.Issues,
		domain.ConsistencyIssue{Severity: "warning", RuleRef: "Master §15.3", Quote: "Cô ấy nói.", Description: "Xưng hô cần xem"},
		domain.ConsistencyIssue{Severity: "warning", RuleText: "Giữ câu ngắn", Quote: "Một câu dài.", Description: "Câu dài"},
	)
	if err := s.World.SaveReview(first); err != nil {
		t.Fatal(err)
	}
	before := readQualityTest(t, tg, writer, "oldbook")
	if got := qualityTestTotals(before); got != [4]int{1, 2, 0, 0} {
		t.Fatalf("round 1 counts = %v, want [1 2 0 0]", got)
	}
	var keptID string
	for _, item := range before.Issues {
		if item.State != "open" || item.Ch != 1 {
			t.Fatalf("initial issue = %+v", item)
		}
		if item.Ref == "Master §15.3" {
			keptID = item.ID
		}
	}
	if keptID == "" {
		t.Fatal("missing keepable item")
	}
	if w := tg.do(http.MethodPost, keepPath(keptID), writer, `{"reason":"Cố ý đổi cách gọi."}`); w.Code != http.StatusOK {
		t.Fatalf("keep = %d: %s", w.Code, w.Body)
	}
	kept := readQualityTest(t, tg, writer, "oldbook")
	if got := qualityTestTotals(kept); got != [4]int{1, 1, 0, 1} {
		t.Fatalf("kept excluded from open = %v", got)
	}
	second := errorReview(1)
	second.Issues = append(second.Issues, domain.ConsistencyIssue{
		Severity: "warning", RuleRef: "Bộ A §4.3", Quote: "Cô ấy cười.", Description: "Phản diện cần xem",
	})
	if err := s.World.SaveReview(second); err != nil {
		t.Fatal(err)
	}
	// Same stable issue in the global review must not double-count the chapter card.
	global := second
	global.Scope = "global"
	if err := s.World.SaveReview(global); err != nil {
		t.Fatal(err)
	}
	after := readQualityTest(t, tg, writer, "oldbook")
	if got := qualityTestTotals(after); got != [4]int{1, 1, 1, 1} {
		t.Fatalf("round 2 counts = %v, want [1 1 1 1]", got)
	}
	if after.Pack == nil || after.Pack.ID != "nu-tan-bao-thu" || len(after.Issues) != 4 {
		t.Fatalf("pack/issues = %+v / %+v", after.Pack, after.Issues)
	}
	for _, item := range after.Issues {
		if item.ID == keptID && (item.State != "kept" || item.KeptReason != "Cố ý đổi cách gọi." || item.Decision == nil) {
			t.Fatalf("historical kept issue = %+v", item)
		}
		if item.Ref == "Quy tắc của bạn" && item.State != "fixed" {
			t.Fatalf("disappeared issue = %+v", item)
		}
	}
	var refs []string
	for _, section := range after.Sections {
		refs = append(refs, section.Ref)
		if section.RefName == "" {
			t.Fatalf("section missing pack/fallback title: %+v", section)
		}
	}
	if !slices.Equal(refs, []string{"Master §7", "Master §15.3", "Bộ A §4.3", "Quy tắc của bạn"}) {
		t.Fatalf("sections in pack order = %v", refs)
	}
	w := tg.do(http.MethodGet, "/api/projects/_/stories/oldbook/reviews/1", writer, "")
	var review struct {
		Chapter reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	var open [2]int
	for _, item := range review.Chapter.Items {
		if item.Decision != nil && item.Decision.State == "kept" {
			continue
		}
		if item.Level == "error" {
			open[0]++
		} else {
			open[1]++
		}
	}
	counts := qualityTestTotals(after)
	if open != [2]int{counts[0], counts[1]} || len(review.Chapter.Previous) != 1 {
		t.Fatalf("review DTO counts/history = %v / %+v; matrix %v", open, review.Chapter.Previous, counts)
	}
	w = tg.do(http.MethodGet, "/api/projects/_/stories", writer, "")
	var listing struct {
		Stories []struct {
			ID         string `json:"id"`
			OpenErrors *int   `json:"openErrors"`
		} `json:"stories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, story := range listing.Stories {
		if story.ID == "_/oldbook" {
			found = true
			if story.OpenErrors == nil || *story.OpenErrors != counts[0] {
				t.Fatalf("summary openErrors = %v, matrix = %d", story.OpenErrors, counts[0])
			}
		}
	}
	if !found {
		t.Fatalf("story missing from summary: %s", w.Body)
	}
}

func TestQualityChapterStatusesAndEmpty(t *testing.T) {
	tg := newTestGateway(t)
	s, writer := gateStory(t, tg)
	empty := readQualityTest(t, tg, writer, "oldbook")
	if empty.Chapters == nil || empty.Sections == nil || empty.Cells == nil || empty.Issues == nil || len(empty.Issues) != 0 {
		t.Fatalf("empty quality arrays = %+v", empty)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}, {Chapter: 3, Title: "Ba"}, {Chapter: 4, Title: "Bốn"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(4); err != nil {
		t.Fatal(err)
	}
	for ch := 1; ch <= 3; ch++ {
		if err := s.Progress.MarkChapterComplete(ch, 1, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Progress.SetPendingRewrites([]int{2, 3}, "Sửa lại", store.RewriteCauseUser); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.StartChapter(2); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveReview(errorReview(5)); err != nil {
		t.Fatal(err)
	}
	out := readQualityTest(t, tg, writer, "oldbook")
	var statuses []string
	for _, ch := range out.Chapters {
		statuses = append(statuses, ch.Status)
	}
	if !slices.Equal(statuses, []string{"approved", "rewriting", "previous", "previous"}) {
		t.Fatalf("chapter statuses = %v", statuses)
	}
	if out.Chapters[0].Title != "Một" || out.Chapters[3].N != 5 || len(out.Cells) != 1 || out.Cells[0].Ch != 5 {
		t.Fatalf("outline/review-only chapters = %+v / %+v", out.Chapters, out.Cells)
	}
	if err := s.World.SaveReview(errorReview(1)); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "accept"}); err != nil {
		t.Fatal(err)
	}
	out = readQualityTest(t, tg, writer, "oldbook")
	// Câu 9 §H: bằng chứng thẩm duyệt cũ không còn đủ để gọi là "đã viết lại";
	// chỉ log viết lại thật mới quyết định.
	if out.Chapters[0].Status != "approved" || len(out.Chapters[0].Rewrites) != 0 {
		t.Fatalf("blocking round without rewrite log = %q %+v, want approved []", out.Chapters[0].Status, out.Chapters[0].Rewrites)
	}
	// Removing the active chapter leaves queued chapters as old versions, not active rewrites.
	p, err := s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	p.InProgressChapter = 0
	if err := s.Progress.Save(p); err != nil {
		t.Fatal(err)
	}
	out = readQualityTest(t, tg, writer, "oldbook")
	if out.Chapters[1].Status != "previous" {
		t.Fatalf("queued but inactive = %q", out.Chapters[1].Status)
	}
}

func TestQualityRewritesFromUserRewriteLog(t *testing.T) {
	tg := newTestGateway(t)
	s, writer := gateStory(t, tg)
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(2); err != nil {
		t.Fatal(err)
	}
	for _, ch := range []int{1, 2} {
		if err := s.Progress.MarkChapterComplete(ch, 1000, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	// Đường người dùng: áp dụng sửa xưng hô/quan hệ (apply=mark) đưa chương 1 vào
	// hàng đợi viết lại với cause=user; viết xong thì hàng đợi trống.
	if err := naoQueueRewrite(s, []int{1}, "Sửa cách gọi"); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.CompleteRewrite(1, false); err != nil {
		t.Fatal(err)
	}
	out := readQualityTest(t, tg, writer, "oldbook")
	type chapterView struct {
		Status   string
		Rewrites []qualityTestRewrite
	}
	byN := map[int]chapterView{}
	for _, ch := range out.Chapters {
		byN[ch.N] = chapterView{Status: ch.Status, Rewrites: ch.Rewrites}
	}
	first, second := byN[1], byN[2]
	if first.Status != "rewritten" {
		t.Fatalf("chương có log viết lại = %q, muốn rewritten", first.Status)
	}
	if len(first.Rewrites) != 1 || first.Rewrites[0].Cause != store.RewriteCauseUser || first.Rewrites[0].At == "" {
		t.Fatalf("rewrites chương 1 = %+v, muốn một mục cause=user kèm at", first.Rewrites)
	}
	if second.Status != "approved" || len(second.Rewrites) != 0 {
		t.Fatalf("chương không có nguồn = %q %+v, muốn approved []", second.Status, second.Rewrites)
	}
}

func TestQualityUnreadableStory(t *testing.T) {
	tg := newTestGateway(t)
	reader := tg.login("reader", "reader-pass")
	w := tg.do(http.MethodGet, "/api/projects/proj/stories/secret/quality", reader, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("unreadable quality = %d: %s", w.Code, w.Body)
	}
	visible := tg.do(http.MethodGet, "/api/projects/_/stories/oldbook/quality", reader, "")
	if visible.Code != http.StatusOK {
		t.Fatalf("read-only quality = %d: %s", visible.Code, visible.Body)
	}
}
