package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// projectBrainFixture creates three legacy stories in project "_".
// Permission-isolation cases move the secret story to an unassigned project.
func projectBrainFixture(t *testing.T, tg *testGateway, chapters map[string][]string) {
	t.Helper()
	write := func(story, rel, body string) {
		t.Helper()
		path := filepath.Join(tg.root, story, "output", "novel", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("oldbook", "characters.json", `[
  {"id":"nv:ba-tu","name":"Bà Tư","aliases":["Bà Tư bán xôi"],"role":"Hàng xóm bán xôi","description":"Người bán xôi đầu ngõ."},
  {"id":"nv:lan","name":"Lan","role":"Nữ chính"}
]`)
	write("secret", "characters.json", `[
  {"id":"nv:ba-tu","name":"Bà Tư","role":"Vai trong truyện ẩn"},
  {"id":"nv:mat-vu","name":"Mật Vụ","role":"Phản diện ẩn"}
]`)
	write("third", "characters.json", `[
  {"id":"nv:ba-tu-ban-xoi","name":"bà tư  bán xôi","role":"Nhân chứng ở đám giỗ"},
  {"id":"nv:lan","name":"Lân","role":"Em trai"}
]`)
	naoWriteRecord(t, filepath.Join(tg.root, "oldbook", "output", "novel"), 1, domain.ChapterFacts{
		Places: []domain.FactEntity{{Name: "Chợ Vân Châu", Desc: "Chợ phiên đầu truyện."}},
	})
	naoWriteRecord(t, filepath.Join(tg.root, "secret", "output", "novel"), 1, domain.ChapterFacts{
		Places: []domain.FactEntity{{Name: "Chợ Vân Châu", Desc: "Sân khấu cao trào ẩn."}},
	})
	for _, story := range []string{"oldbook", "secret", "third"} {
		texts := chapters[story]
		if len(texts) == 0 {
			texts = []string{"Chính văn."}
		}
		done := make([]string, 0, len(texts))
		for i, text := range texts {
			done = append(done, strconv.Itoa(i+1))
			write(story, fmt.Sprintf("chapters/%02d.md", i+1), text)
		}
		write(story, "meta/progress.json", `{"phase":"writing","completed_chapters":[`+strings.Join(done, ",")+`]}`)
	}
	projects := map[string]webauth.ProjectRole{"_": webauth.ProjectView}
	if _, err := tg.g.accounts.UpdateUser("reader", webauth.UserPatch{Projects: &projects}); err != nil {
		t.Fatal(err)
	}
}

type universeBody struct {
	Name   string `json:"name"`
	Counts struct {
		Stories int `json:"stories"`
		Chars   int `json:"chars"`
		Places  int `json:"places"`
	} `json:"counts"`
	Entities []struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Stories []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"stories"`
		LastRole string `json:"lastRole"`
	} `json:"entities"`
	Checks    []universeCheck     `json:"checks"`
	KeptCount int                 `json:"keptCount"`
	Kept      []universeKeptCheck `json:"kept"`
}

func getUniverse(t *testing.T, tg *testGateway, cookie string) (universeBody, string) {
	t.Helper()
	return getUniverseAt(t, tg, cookie, "_")
}

func getUniverseAt(t *testing.T, tg *testGateway, cookie, project string) (universeBody, string) {
	t.Helper()
	w := tg.do("GET", "/api/projects/"+project+"/universe", cookie, "")
	if w.Code != 200 {
		t.Fatalf("universe %s: %d %s", project, w.Code, w.Body.String())
	}
	var body universeBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("universe body %q: %v", w.Body.String(), err)
	}
	return body, w.Body.String()
}

func universeStoryIDs(stories []struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}) string {
	ids := make([]string, 0, len(stories))
	for _, s := range stories {
		ids = append(ids, s.ID)
	}
	return strings.Join(ids, ",")
}

// TestProjectUniverseMergesAcrossStories: thực thể gộp giữa các truyện theo tên hoặc
// biệt danh (giữ dấu: "Lan" ≠ "Lân"), chỉ thực thể có ở ≥2 truyện; truyện mới nhất
// đứng đầu và cho lastRole.
func TestProjectUniverseMergesAcrossStories(t *testing.T) {
	tg := newTestGateway(t)
	projectBrainFixture(t, tg, nil)
	admin := tg.login("admin", "admin-pass")

	body, raw := getUniverse(t, tg, admin)
	if len(body.Entities) != 2 {
		t.Fatalf("entities = %s", raw)
	}
	ba := body.Entities[0]
	if ba.ID != "char:ba-tu" || ba.Type != "char" || ba.Name != "Bà Tư" ||
		universeStoryIDs(ba.Stories) != "third,secret,oldbook" || ba.LastRole != "Nhân chứng ở đám giỗ" {
		t.Fatalf("Bà Tư = %+v", ba)
	}
	cho := body.Entities[1]
	if cho.ID != "place:cho-van-chau" || cho.Type != "place" || universeStoryIDs(cho.Stories) != "secret,oldbook" ||
		cho.LastRole != "Sân khấu cao trào ẩn." {
		t.Fatalf("Chợ Vân Châu = %+v", cho)
	}
	if body.Counts.Stories != 3 || body.Counts.Chars != 1 || body.Counts.Places != 1 {
		t.Fatalf("counts = %+v", body.Counts)
	}
	if strings.Contains(raw, "Lân") {
		t.Fatalf("Lan/Lân phải là hai người: %s", raw)
	}
	// Master §19 chưa có nguồn chấm: null (chưa chấm), không phải [] (đã chấm, Đạt).
	if !strings.Contains(raw, `"checks":null`) {
		t.Fatalf("checks phải là null khi chưa chấm: %s", raw)
	}
	// Dự án ẩn danh "_": không có tên vũ trụ, FE ẩn dòng tên.
	if body.Name != "" {
		t.Fatalf("name dự án ẩn danh = %q, muốn \"\"", body.Name)
	}
}

// TestProjectUniverseHidesUnviewableStory: truyện người gọi không được xem không góp
// vào thực thể, truyện của thực thể hay con số nào — kể cả mục Master §19.
func TestProjectUniverseHidesUnviewableStory(t *testing.T) {
	tg := newTestGateway(t)
	projectBrainFixture(t, tg, nil)
	saveStoryReview(t, tg.root, "secret", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "rewrite", RuleAware: true,
		Issues: []domain.ConsistencyIssue{recapIssue("Bí mật của Mật Vụ nhắc chuyện cũ bốn câu.")},
	})
	saveStoryReview(t, tg.root, "oldbook", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "accept", RuleAware: true,
	})
	moveStoryToUnassignedProject(t, tg.root, filepath.Join(tg.root, "secret"))
	reader := tg.login("reader", "reader-pass")

	body, raw := getUniverse(t, tg, reader)
	for _, leak := range []string{"secret", "Mật Vụ", "Vai trong truyện ẩn", "Sân khấu cao trào ẩn", "Chợ Vân Châu", "Bí mật"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("lộ truyện ẩn (%q): %s", leak, raw)
		}
	}
	if len(body.Entities) != 1 || universeStoryIDs(body.Entities[0].Stories) != "third,oldbook" {
		t.Fatalf("entities = %s", raw)
	}
	if body.Counts.Stories != 2 || body.Counts.Chars != 1 || body.Counts.Places != 0 {
		t.Fatalf("counts = %+v", body.Counts)
	}
	// oldbook đã chấm bằng lược đồ có rule và sạch ⇒ [] (đã kiểm), không phải null.
	if !strings.Contains(raw, `"checks":[]`) {
		t.Fatalf("checks phải là [] khi đã chấm mà sạch: %s", raw)
	}
}

// recapIssue là mục Biên tập viên phát hiện nhắc chuyện cũ quá 2 câu (Master §19).
func recapIssue(text string) domain.ConsistencyIssue {
	return domain.ConsistencyIssue{
		Type: "recap", Severity: "warning", Description: text,
		Evidence: "Bốn câu nhắc lại chuyện truyện trước.", RuleRef: "Master §19", Rule: "s19-recap",
	}
}

// saveStoryReview lưu một bài thẩm duyệt vào reviews/ của truyện; lần lưu sau nội
// dung khác sẽ tự đẩy bài trước vào reviews/rounds/ (cùng đường engine dùng).
func saveStoryReview(t *testing.T, root, story string, entry domain.ReviewEntry) {
	t.Helper()
	s := store.NewStore(filepath.Join(root, story, "output", "novel"))
	if err := s.World.SaveReview(entry); err != nil {
		t.Fatalf("save review %s: %v", story, err)
	}
}

// TestProjectUniverseChecks: chỉ mục `ref:"Master §19"` + `rule:"s19-recap"` của
// truyện dùng chung thực thể, gồm cả vòng chấm đã lưu trữ; mục thiếu `rule` bị bỏ.
func TestProjectUniverseChecks(t *testing.T) {
	tg := newTestGateway(t)
	projectBrainFixture(t, tg, nil)
	admin := tg.login("admin", "admin-pass")

	// oldbook: vòng 1 phát hiện, vòng 2 sạch ⇒ mục chỉ còn ở reviews/rounds/.
	saveStoryReview(t, tg.root, "oldbook", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "rewrite", RuleAware: true,
		Issues: []domain.ConsistencyIssue{recapIssue("“Bà Tư bán xôi” nhắc lại bốn câu về truyện cũ.")},
	})
	saveStoryReview(t, tg.root, "oldbook", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "accept", RuleAware: true,
	})
	// third: đã chấm, có mục Master §19 nhưng engine cũ không phát rule ⇒ không tính.
	saveStoryReview(t, tg.root, "third", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "accept", RuleAware: true,
		Issues: []domain.ConsistencyIssue{{
			Type: "recap", Severity: "warning", Description: "Mục cũ thiếu rule.",
			RuleRef: "Master §19",
		}},
	})
	// lonely: có vi phạm recap nhưng không dùng chung thực thể nào ⇒ không tính.
	engine := mkdirAll(t, tg.root, "lonely", "output", "novel")
	writeFile := func(rel, body string) {
		t.Helper()
		path := filepath.Join(engine, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("characters.json", `[{"id":"nv:rieng","name":"Chỉ Riêng","role":"Không xuất hiện ở truyện khác"}]`)
	writeFile("meta/progress.json", `{"phase":"writing","completed_chapters":[1]}`)
	writeFile("chapters/01.md", "Chính văn.")
	saveStoryReview(t, tg.root, "lonely", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "rewrite", RuleAware: true,
		Issues: []domain.ConsistencyIssue{recapIssue("Truyện riêng nhắc chuyện cũ bốn câu.")},
	})

	body, raw := getUniverse(t, tg, admin)
	if len(body.Checks) != 1 {
		t.Fatalf("checks = %s", raw)
	}
	if strings.Contains(raw, "Truyện riêng") {
		t.Fatalf("truyện không dùng chung thực thể không được góp: %s", raw)
	}
	check := body.Checks[0]
	if check.Ref != "Master §19" || check.Story != "oldbook" || check.Ch != 1 ||
		check.Text != "“Bà Tư bán xôi” nhắc lại bốn câu về truyện cũ." ||
		!strings.HasPrefix(check.QuoteID, "ch1-") {
		t.Fatalf("check = %+v", check)
	}
}

// TestProjectUniverseKeptItems: mục Master §19 đã "Giữ nguyên" (GAP-47) không vào
// `checks`, được đếm ở `keptCount` và trả nguyên hình ở `kept`; mục đã giữ của
// truyện người gọi không xem được không bao giờ được đếm.
func TestProjectUniverseKeptItems(t *testing.T) {
	tg := newTestGateway(t)
	projectBrainFixture(t, tg, nil)
	admin := tg.login("admin", "admin-pass")

	keptText := "Mục người dùng quyết định giữ nguyên."
	otherText := "Mục vẫn cần sửa."
	issue := func(text, quote string) domain.ConsistencyIssue {
		return domain.ConsistencyIssue{
			Type: "recap", Severity: "warning", Description: text,
			Evidence: quote, RuleRef: "Master §19", Rule: "s19-recap",
		}
	}
	saveStoryReview(t, tg.root, "oldbook", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "rewrite", RuleAware: true,
		Issues: []domain.ConsistencyIssue{issue(keptText, "Trích A."), issue(otherText, "Trích B.")},
	})

	body, raw := getUniverse(t, tg, admin)
	if len(body.Checks) != 2 {
		t.Fatalf("checks before keep = %s", raw)
	}
	var keptID string
	for _, c := range body.Checks {
		if c.Text == keptText {
			keptID = c.QuoteID
		}
	}
	if keptID == "" {
		t.Fatalf("kept item missing from checks: %s", raw)
	}
	oldbook := store.NewStore(filepath.Join(tg.root, "oldbook", "output", "novel"))
	if err := oldbook.World.SaveReviewDecision(keptID, store.ReviewDecision{
		State: "kept", Reason: "Cố ý nhắc lại.", By: "admin", At: "2026-10-26T10:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	body, raw = getUniverse(t, tg, admin)
	if len(body.Checks) != 1 || body.Checks[0].Text != otherText {
		t.Fatalf("kept item still in checks: %s", raw)
	}
	if body.KeptCount != 1 || len(body.Kept) != 1 {
		t.Fatalf("keptCount=%d kept=%+v (%s)", body.KeptCount, body.Kept, raw)
	}
	got := body.Kept[0]
	if got.Ref != "Master §19" || got.Story != "oldbook" || got.Ch != 1 || got.Text != keptText || got.QuoteID != keptID {
		t.Fatalf("kept item = %+v", got)
	}
	// §J câu 1: lý do và người/giờ quyết định đi kèm mục đã giữ.
	if got.Reason != "Cố ý nhắc lại." || got.DecidedBy != "admin" || got.DecidedAt != "2026-10-26T10:00:00Z" {
		t.Fatalf("kept decision fields = %+v", got)
	}
	if !strings.Contains(raw, `"decidedBy":"admin"`) || !strings.Contains(raw, `"decidedAt":"2026-10-26T10:00:00Z"`) {
		t.Fatalf("kept JSON thiếu trường quyết định: %s", raw)
	}

	// Truyện ẩn với người đọc "reader": mục đã giữ ở đó không được đếm.
	hiddenID := domain.ReviewItemID(1, map[string]int{}, "Master §19", "Trích ẩn.")
	saveStoryReview(t, tg.root, "secret", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "rewrite", RuleAware: true,
		Issues: []domain.ConsistencyIssue{issue("Bí mật đã giữ nguyên.", "Trích ẩn.")},
	})
	secret := store.NewStore(filepath.Join(tg.root, "secret", "output", "novel"))
	if err := secret.World.SaveReviewDecision(hiddenID, store.ReviewDecision{
		State: "kept", Reason: "Cố ý.", By: "admin", At: "2026-10-26T10:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	moveStoryToUnassignedProject(t, tg.root, filepath.Join(tg.root, "secret"))

	reader := tg.login("reader", "reader-pass")
	body, raw = getUniverse(t, tg, reader)
	if body.KeptCount != 1 || len(body.Kept) != 1 || body.Kept[0].Text != keptText {
		t.Fatalf("hidden kept item counted: %s", raw)
	}
	for _, leak := range []string{"Bí mật", "Trích ẩn", "secret"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("lộ mục đã giữ của truyện ẩn (%q): %s", leak, raw)
		}
	}
}

// TestProjectUniverseKeptLegacyAndUnkeep: bản ghi GAP-47 cũ thiếu lý do/người/giờ
// trả "" (không bịa); giữ từ nay ghi đủ ba trường; bỏ giữ đưa mục về `checks` và
// giảm `keptCount` ngay.
func TestProjectUniverseKeptLegacyAndUnkeep(t *testing.T) {
	tg := newTestGateway(t)
	projectBrainFixture(t, tg, nil)
	admin := tg.login("admin", "admin-pass")

	text := "Nhắc lại chuyện cũ bốn câu."
	saveStoryReview(t, tg.root, "oldbook", domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "rewrite", RuleAware: true,
		Issues: []domain.ConsistencyIssue{recapIssue(text)},
	})
	body, raw := getUniverse(t, tg, admin)
	if len(body.Checks) != 1 {
		t.Fatalf("checks before keep = %s", raw)
	}
	itemID := body.Checks[0].QuoteID

	// Quyết định cũ: chỉ có `state`, không có reason/by/at.
	decisions := filepath.Join(tg.root, "oldbook", "output", "novel", "reviews", "decisions.json")
	if err := os.MkdirAll(filepath.Dir(decisions), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decisions, []byte(`{"items":{"`+itemID+`":{"state":"kept"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	body, raw = getUniverse(t, tg, admin)
	if body.KeptCount != 1 || len(body.Kept) != 1 {
		t.Fatalf("kept legacy = %s", raw)
	}
	legacy := body.Kept[0]
	if legacy.QuoteID != itemID || legacy.Reason != "" || legacy.DecidedBy != "" || legacy.DecidedAt != "" {
		t.Fatalf("bản ghi cũ phải trả chuỗi rỗng, không bịa: %+v", legacy)
	}
	if !strings.Contains(raw, `"reason":""`) || !strings.Contains(raw, `"decidedBy":""`) || !strings.Contains(raw, `"decidedAt":""`) {
		t.Fatalf("kept JSON phải có đủ ba khoá rỗng: %s", raw)
	}

	// Giữ lại từ nay: lý do lấy từ thân yêu cầu, người giữ là tên hiển thị.
	if w := tg.do("POST", keepPath(itemID), admin, `{"reason":"Cố ý mở chuyện cũ."}`); w.Code != 200 {
		t.Fatalf("keep = %d %s", w.Code, w.Body.String())
	}
	body, raw = getUniverse(t, tg, admin)
	if len(body.Kept) != 1 || body.Kept[0].Reason != "Cố ý mở chuyện cũ." || body.Kept[0].DecidedBy != "admin" || body.Kept[0].DecidedAt == "" {
		t.Fatalf("kept sau khi giữ = %s", raw)
	}
	if len(body.Checks) != 0 {
		t.Fatalf("mục đã giữ còn trong checks: %s", raw)
	}

	// Bỏ giữ: mục về `checks` ngay, keptCount = 0.
	if w := tg.do("DELETE", keepPath(itemID), admin, ""); w.Code != 204 {
		t.Fatalf("bỏ giữ = %d %s", w.Code, w.Body.String())
	}
	body, raw = getUniverse(t, tg, admin)
	if body.KeptCount != 0 || len(body.Kept) != 0 {
		t.Fatalf("keptCount sau bỏ giữ = %d (%s)", body.KeptCount, raw)
	}
	if len(body.Checks) != 1 || body.Checks[0].QuoteID != itemID {
		t.Fatalf("mục không về lại checks sau bỏ giữ: %s", raw)
	}
}

// TestProjectUniverseNameRoundTrip: universeName sửa qua PUT dự án, trả ở GET dự án
// và danh sách; name vũ trụ = universeName || tên dự án; xoá thì quay lại tên dự án.
func TestProjectUniverseNameRoundTrip(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	getDetail := func() projectDetail {
		t.Helper()
		w := tg.do("GET", "/api/projects/proj", admin, "")
		if w.Code != 200 {
			t.Fatalf("get project: %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Project projectDetail `json:"project"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode project: %v", err)
		}
		return body.Project
	}

	// Chưa đặt: "" và vũ trụ dùng tên dự án.
	if got := getDetail().UniverseName; got != "" {
		t.Fatalf("universeName mặc định = %q", got)
	}
	if body, raw := getUniverseAt(t, tg, admin, "proj"); body.Name != "Proj" {
		t.Fatalf("name vũ trụ = %q (%s), muốn tên dự án", body.Name, raw)
	}
	// Dự án có project.json nhưng chưa đặt tên ⇒ "" (không bịa tên thư mục).
	plain := mkdirAll(t, tg.root, "plain")
	if err := os.WriteFile(filepath.Join(plain, "project.json"), []byte(`{"name":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mkdirAll(t, tg.root, "plain", "kich_ban")
	if body, raw := getUniverseAt(t, tg, admin, "plain"); body.Name != "" {
		t.Fatalf("name dự án chưa đặt tên = %q (%s), muốn \"\"", body.Name, raw)
	}

	w := tg.do("PUT", "/api/projects/proj", admin, `{"name":"Proj","description":"","notes":"","universeName":"Vũ trụ Vân Châu"}`)
	if w.Code != 200 {
		t.Fatalf("put universeName: %d %s", w.Code, w.Body.String())
	}
	if got := getDetail().UniverseName; got != "Vũ trụ Vân Châu" {
		t.Fatalf("universeName sau PUT = %q", got)
	}
	if body, raw := getUniverseAt(t, tg, admin, "proj"); body.Name != "Vũ trụ Vân Châu" {
		t.Fatalf("name vũ trụ = %q (%s)", body.Name, raw)
	}
	list := tg.do("GET", "/api/library", admin, "")
	var library struct {
		Projects []struct {
			ID           string `json:"id"`
			UniverseName string `json:"universeName"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &library); err != nil {
		t.Fatalf("decode library: %v", err)
	}
	found := false
	for _, project := range library.Projects {
		if project.ID == "proj" {
			found = true
			if project.UniverseName != "Vũ trụ Vân Châu" {
				t.Fatalf("danh sách: universeName = %q", project.UniverseName)
			}
		}
	}
	if !found {
		t.Fatalf("danh sách thiếu dự án proj: %s", list.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(tg.root, "proj", "project.json"))
	if err != nil || !strings.Contains(string(raw), `"universeName": "Vũ trụ Vân Châu"`) {
		t.Fatalf("project.json phải lưu universeName: %s %v", raw, err)
	}

	// Chuỗi rỗng = xoá, quay về tên dự án; tệp không còn khoá.
	w = tg.do("PUT", "/api/projects/proj", admin, `{"name":"Proj","universeName":""}`)
	if w.Code != 200 {
		t.Fatalf("clear universeName: %d %s", w.Code, w.Body.String())
	}
	if got := getDetail().UniverseName; got != "" {
		t.Fatalf("universeName sau khi xoá = %q", got)
	}
	if body, raw := getUniverseAt(t, tg, admin, "proj"); body.Name != "Proj" {
		t.Fatalf("name vũ trụ sau khi xoá = %q (%s)", body.Name, raw)
	}
	if raw, err := os.ReadFile(filepath.Join(tg.root, "proj", "project.json")); err != nil || strings.Contains(string(raw), "universeName") {
		t.Fatalf("project.json phải bỏ universeName khi xoá: %s %v", raw, err)
	}
}
