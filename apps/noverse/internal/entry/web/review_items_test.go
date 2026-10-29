package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/naolint"
	"github.com/datnt1998/noverse/internal/notify"
	"github.com/datnt1998/noverse/internal/rules"
	"github.com/datnt1998/noverse/internal/store"
)

// packStoryFixture = storyFixture + cấu hình truyện chọn gói Nữ tần báo thù, để
// refName lấy được tiêu đề mục từ file luật của gói.
func packStoryFixture(t *testing.T, completed []int, style string) (library.Story, *store.Store) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	st, s := storyFixture(t, completed)
	dir := filepath.Join(st.Dir, ".ainovel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	cfg := `{"style":"` + style + `"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("config: %v", err)
	}
	return st, s
}

func reviewEntryBody(t *testing.T, body []byte) reviewEntryAPI {
	t.Helper()
	var entry reviewEntryAPI
	if err := json.Unmarshal(body, &entry); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return entry
}

// TestReviewItemsMapping pin DTO GAP-36: level/ref/refName/text/quote/fix/id, round,
// pack, blocking và các mục "Xưng hô" của naolint (D9).
func TestReviewItemsMapping(t *testing.T) {
	g := &Gateway{}
	st, s := packStoryFixture(t, []int{1, 2, 3}, "nu-tan-bao-thu")
	review := domain.ReviewEntry{
		Chapter: 3,
		Scope:   "chapter",
		Verdict: "polish",
		Summary: "Cần gọt giũa",
		Issues: []domain.ConsistencyIssue{
			{
				Type: "hook", Severity: "error", Description: "Kết chương ở trạng thái an toàn",
				Evidence: "Tôi đi ngủ.", RuleRef: "Master §7", Suggestion: "Mở bằng một câu thoại",
				RequiresChange: true,
			},
			{
				Type: "style", Severity: "warning", Description: "Dùng từ cấm",
				Evidence: "khá là", RuleText: "Không dùng từ cấm", RuleRef: "",
			},
		},
	}
	if err := s.World.SaveReview(review); err != nil {
		t.Fatalf("save review: %v", err)
	}
	if err := s.World.SaveRuleViolations(3, []rules.Violation{
		{
			Rule: "address_terms", Target: "Tri Hạ → Nhược Vy", Actual: "em", Limit: "cô",
			Severity: rules.SeverityWarning,
			Message:  "Master §15.3 [CẢNH BÁO] chương 3, dòng 5: Tri Hạ tự xưng \"em\". Trích: Em đừng đi.",
		},
		{Rule: "address_terms", Actual: "chị", Limit: "cô", Severity: rules.SeverityWarning, Message: "không rõ người nói"},
		{Rule: "fatigue_words", Target: "nhìn", Severity: rules.SeverityWarning, Message: "bỏ qua"},
	}); err != nil {
		t.Fatalf("save violations: %v", err)
	}
	// Hai lần chấm khác nội dung: checkpoint là nguồn duy nhất của round.
	for _, digest := range []string{"sha256:lan1", "sha256:lan2"} {
		if _, err := s.Checkpoints.Append(domain.ChapterScope(3), "review", "reviews/03.json", digest); err != nil {
			t.Fatalf("checkpoint: %v", err)
		}
	}

	rec := storyReq(t, g, "GET", "/x/reviews/3", adminUser(), st, "/reviews/3", "")
	if rec.Code != 200 {
		t.Fatalf("reviews/3 = %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Chapter *reviewEntryAPI `json:"chapter"`
		Global  *reviewEntryAPI `json:"global"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Chapter == nil || body.Global != nil {
		t.Fatalf("chapter=%v global=%v", body.Chapter, body.Global)
	}
	entry := *body.Chapter
	if entry.Verdict != "polish" || len(entry.Issues) != 2 {
		t.Fatalf("trường cũ phải giữ nguyên: %+v", entry)
	}
	if entry.Round != 2 {
		t.Errorf("round = %d, muốn 2", entry.Round)
	}
	if entry.Pack == nil || entry.Pack.ID != "nu-tan-bao-thu" || !strings.Contains(entry.Pack.Ten, "Nữ tần") {
		t.Errorf("pack = %+v", entry.Pack)
	}
	if !entry.Blocking {
		t.Error("có mục Bắt sửa ⇒ blocking = true")
	}
	if len(entry.Items) != 4 {
		t.Fatalf("items = %d, muốn 4 (2 issue + 2 xưng hô): %+v", len(entry.Items), entry.Items)
	}

	packIssue := entry.Items[0]
	if packIssue.Level != "error" || packIssue.Ref != "Master §7" || packIssue.RefName != "Đầu chương và cuối chương" {
		t.Errorf("item gói = %+v", packIssue)
	}
	if packIssue.Text != "Kết chương ở trạng thái an toàn" || packIssue.Quote != "Tôi đi ngủ." || packIssue.Fix != "Mở bằng một câu thoại" {
		t.Errorf("text/quote/fix = %+v", packIssue)
	}
	userIssue := entry.Items[1]
	if userIssue.Level != "warning" || userIssue.Ref != "Quy tắc của bạn" || userIssue.RefName != "Không dùng từ cấm" || userIssue.Fix != "" {
		t.Errorf("item quy tắc của bạn = %+v", userIssue)
	}
	address := entry.Items[2]
	if address.Level != "warning" || address.Ref != "Xưng hô" || address.RefName != "Tri Hạ → Nhược Vy" {
		t.Errorf("item xưng hô = %+v", address)
	}
	if address.Quote != "Em đừng đi." || !strings.Contains(address.Text, "Tri Hạ") || !strings.Contains(address.Text, "cô") {
		t.Errorf("xưng hô text/quote = %+v", address)
	}
	unknown := entry.Items[3]
	if unknown.RefName != "Bảng xưng hô của truyện" || unknown.Ref != "Xưng hô" {
		t.Errorf("item không rõ chiều xưng hô = %+v", unknown)
	}
	ids := map[string]bool{}
	for _, item := range entry.Items {
		if item.ID == "" || ids[item.ID] {
			t.Fatalf("id phải khác rỗng và không trùng: %+v", entry.Items)
		}
		ids[item.ID] = true
	}
	// id ổn định giữa hai lần đọc cùng một bài đã lưu.
	again := storyReq(t, g, "GET", "/x/reviews/3", adminUser(), st, "/reviews/3", "")
	var againBody struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(again.Body.Bytes(), &againBody); err != nil {
		t.Fatal(err)
	}
	if againBody.Chapter == nil || againBody.Chapter.Items[0].ID != packIssue.ID {
		t.Fatalf("id không ổn định: %+v", againBody.Chapter)
	}

	// /reviews/latest trả cùng DTO (bài mới nhất = chương 3).
	latest := storyReq(t, g, "GET", "/x/reviews/latest", adminUser(), st, "/reviews/latest", "")
	latestEntry := reviewEntryBody(t, latest.Body.Bytes())
	if latest.Code != 200 || latestEntry.Chapter != 3 || len(latestEntry.Items) != 4 || latestEntry.Round != 2 {
		t.Fatalf("latest = %d %+v", latest.Code, latestEntry)
	}
}

// TestReviewItemsCarryRuleCode pin câu 4 §H: mã phụ `rule` và cờ rule-aware đi
// qua DTO của vòng hiện tại lẫn các vòng đã lưu trữ (reviews/rounds/).
func TestReviewItemsCarryRuleCode(t *testing.T) {
	g := &Gateway{}
	st, s := packStoryFixture(t, []int{1}, "nu-tan-bao-thu")
	first := domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "polish", Summary: "Nhắc lại truyện trước",
		RuleAware: true,
		Issues: []domain.ConsistencyIssue{{
			Type: "continuity", Severity: "warning", Description: "Nhắc lại truyện trước quá dài",
			Evidence: "Ba câu tóm tắt.", RuleRef: "Master §19", Rule: "s19-recap",
			Quote: "Ba câu tóm tắt.",
		}},
	}
	if err := s.World.SaveReview(first); err != nil {
		t.Fatalf("save review: %v", err)
	}
	rec := storyReq(t, g, "GET", "/x/reviews/1", adminUser(), st, "/reviews/1", "")
	var body struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	entry := *body.Chapter
	if !entry.RuleAware {
		t.Fatal("vòng rule-aware phải mang rule_aware trong DTO")
	}
	if len(entry.Items) != 1 || entry.Items[0].Rule != "s19-recap" || entry.Items[0].Ref != "Master §19" {
		t.Fatalf("item rule/ref = %+v", entry.Items)
	}
	// Vòng cũ bị ghi đè ⇒ vẫn giữ rule và cờ rule-aware trong lịch sử.
	second := domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "Đã sửa", RuleAware: true,
	}
	if err := s.World.SaveReview(second); err != nil {
		t.Fatalf("overwrite review: %v", err)
	}
	rec = storyReq(t, g, "GET", "/x/reviews/1", adminUser(), st, "/reviews/1", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	entry = *body.Chapter
	if len(entry.Previous) != 1 {
		t.Fatalf("previous = %+v, muốn 1 vòng lưu trữ", entry.Previous)
	}
	prev := entry.Previous[0]
	if !prev.RuleAware || len(prev.Items) != 1 || prev.Items[0].Rule != "s19-recap" {
		t.Fatalf("vòng lưu trữ = %+v", prev)
	}
}

// TestReviewItemsWithoutPack: truyện không chọn gói ⇒ pack null, refName rơi về
// rule_text (rồi ""), items vẫn là [] chứ không null.
func TestReviewItemsWithoutPack(t *testing.T) {
	g := &Gateway{}
	t.Setenv("HOME", t.TempDir())
	st, s := storyFixture(t, []int{1})
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "ok"}); err != nil {
		t.Fatalf("save review: %v", err)
	}
	rec := storyReq(t, g, "GET", "/x/reviews/1", adminUser(), st, "/reviews/1", "")
	var body struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	if body.Chapter == nil {
		t.Fatalf("thiếu bài thẩm duyệt: %s", rec.Body)
	}
	entry := *body.Chapter
	if entry.Pack != nil {
		t.Errorf("pack phải null khi không có gói: %+v", entry.Pack)
	}
	if entry.Blocking || len(entry.Items) != 0 || entry.Round != 1 {
		t.Errorf("bài sạch: %+v", entry)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("items phải là []: %s", rec.Body)
	}
}

// TestActivityReviewPayload pin D4 cho Nhật ký: dòng Biên tập viên mang
// review{round, items} để danh sách mục hiện ngay dưới dòng đó; dòng khác không có.
func TestActivityReviewPayload(t *testing.T) {
	tg := newTestGateway(t)
	t.Cleanup(tg.g.Close)
	path := filepath.Join(mkdirAll(t, tg.root, "oldbook", "output", "novel", "logs"), "tui.log")
	appendActivityFixture(t, path, host.Event{ID: "e1", Time: time.Unix(100, 0), Category: "RUN", Agent: "editor", Chapter: 1, Summary: "Thẩm duyệt chương 1"})
	appendActivityFixture(t, path, host.Event{ID: "w1", Time: time.Unix(101, 0), Category: "RUN", Agent: "writer", Chapter: 1, Summary: "Viết chương 1"})
	st := store.NewStore(filepath.Join(tg.root, "oldbook", "output", "novel"))
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.World.SaveReview(errorReview(1)); err != nil {
		t.Fatalf("review: %v", err)
	}
	got := activityGet(t, tg, tg.login("writer", "writer-pass"), "")
	var editor, writer *activityEvent
	for i := range got.Events {
		switch got.Events[i].Agent {
		case "editor":
			editor = &got.Events[i]
		case "writer":
			writer = &got.Events[i]
		}
	}
	if editor == nil || editor.Review == nil {
		t.Fatalf("dòng Biên tập viên thiếu review: %+v", got.Events)
	}
	if editor.Review.Round != 1 || len(editor.Review.Items) != 1 || editor.Review.Items[0].Ref != "Master §7" {
		t.Fatalf("review = %+v", editor.Review)
	}
	if writer == nil || writer.Review != nil {
		t.Fatalf("dòng không phải Biên tập viên không được gắn review: %+v", writer)
	}
}

// TestReviewPreviousRoundsAndGateUsesLatest pin lưu trữ lần chấm: sau khi chấm lại,
// DTO có previous[{round,verdict,items}] (lỗi của lần 1 còn nguyên), round = 2,
// revision đổi (UI tự làm mới) và cửa duyệt chỉ xét lần chấm mới nhất.
func TestReviewPreviousRoundsAndGateUsesLatest(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	stubNextWorker(t, tg.g)
	s, writer := gateStory(t, tg)
	base := "/api/projects/_/stories/oldbook"
	revision := func() string {
		w := tg.do("GET", base+"/chapters", writer, "")
		if w.Code != 200 {
			t.Fatalf("chapters = %d: %s", w.Code, w.Body)
		}
		var v struct {
			Revision string `json:"revision"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v.Revision
	}
	before := revision()

	if err := s.World.SaveReview(errorReview(1)); err != nil {
		t.Fatalf("round 1: %v", err)
	}
	clean := domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "Đạt",
		Issues: []domain.ConsistencyIssue{{
			Type: "pacing", Severity: "warning", Description: "Nhịp chậm", Evidence: "đoạn 2",
			Chapters: []int{1},
		}},
	}
	if err := s.World.SaveReview(clean); err != nil {
		t.Fatalf("round 2: %v", err)
	}
	if after := revision(); after == before {
		t.Fatalf("revision không đổi sau khi có lần chấm mới: %s", after)
	}

	rec := tg.do("GET", base+"/reviews/1", writer, "")
	if rec.Code != 200 {
		t.Fatalf("reviews/1 = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	if body.Chapter == nil {
		t.Fatalf("thiếu bài thẩm duyệt: %s", rec.Body)
	}
	entry := *body.Chapter
	if entry.Round != 2 || entry.Blocking {
		t.Fatalf("round/blocking = %d/%v", entry.Round, entry.Blocking)
	}
	if len(entry.Previous) != 1 {
		t.Fatalf("previous = %+v", entry.Previous)
	}
	prev := entry.Previous[0]
	if prev.Round != 1 || prev.Verdict != "polish" || len(prev.Items) != 1 {
		t.Fatalf("previous[0] = %+v", prev)
	}
	if prev.Items[0].Level != "error" || prev.Items[0].Ref != "Master §7" || prev.Items[0].RefName != "Đầu chương và cuối chương" {
		t.Fatalf("mục lần chấm 1 = %+v", prev.Items[0])
	}
	// Cửa duyệt chỉ xét lần chấm mới nhất ⇒ mở dù lần 1 còn lỗi.
	if w := tg.do("POST", base+"/next", writer, `{"applyComments":true,"chapter":1}`); w.Code != 200 {
		t.Fatalf("duyệt sau lần chấm sạch = %d: %s", w.Code, w.Body)
	}
}

// stubNextWorker trả 200 cho /api/next để chứng minh cửa duyệt đã mở.
func stubNextWorker(t *testing.T, g *Gateway) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	}))
	t.Cleanup(server.Close)
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	t.Cleanup(stop)
	g.launch = func(id, dir, root, token string) (*launchedWorker, error) {
		return &launchedWorker{
			addr:  strings.TrimPrefix(server.URL, "http://"),
			stdin: io.NopCloser(strings.NewReader("")),
			done:  done,
			kill:  func() error { stop(); return nil },
		}, nil
	}
}

func gateStory(t *testing.T, tg *testGateway) (*store.Store, string) {
	t.Helper()
	s := store.NewStore(filepath.Join(tg.root, "oldbook", "output", "novel"))
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	dir := filepath.Join(tg.root, "oldbook", ".ainovel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"style":"nu-tan-bao-thu"}`), 0o644); err != nil {
		t.Fatalf("config: %v", err)
	}
	return s, tg.login("writer", "writer-pass")
}

func errorReview(chapter int) domain.ReviewEntry {
	return domain.ReviewEntry{
		Chapter: chapter, Scope: "chapter", Verdict: "polish", Summary: "Còn lỗi",
		Issues: []domain.ConsistencyIssue{{
			Type: "hook", Severity: "error", Description: "Kết chương an toàn",
			Evidence: "Tôi đi ngủ.", RuleRef: "Master §7", Suggestion: "Mở bằng câu thoại",
			Chapters: []int{chapter}, RequiresChange: true,
		}},
	}
}

// TestReviewApproveGateBlocksErrorItems pin D4: /next trả 409 kèm items Bắt sửa
// khi bài thẩm duyệt mới nhất của chương còn lỗi, và mở lại sau một lần chấm sạch.
func TestReviewApproveGateBlocksErrorItems(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	stubNextWorker(t, tg.g)
	s, writer := gateStory(t, tg)
	if err := s.World.SaveReview(errorReview(1)); err != nil {
		t.Fatalf("review: %v", err)
	}
	path := "/api/projects/_/stories/oldbook/next"
	w := tg.do("POST", path, writer, `{"applyComments":true,"chapter":1}`)
	if w.Code != 409 {
		t.Fatalf("duyệt khi còn Bắt sửa = %d: %s", w.Code, w.Body)
	}
	body := decodeJSONBody(t, w)
	if body["error"] != "Chương 1 còn mục Bắt sửa, chưa duyệt được." {
		t.Fatalf("error = %v", body["error"])
	}
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %v", body["items"])
	}
	first, _ := items[0].(map[string]any)
	if first["level"] != "error" || first["ref"] != "Master §7" || first["refName"] != "Đầu chương và cuối chương" {
		t.Fatalf("item = %v", first)
	}

	// Chấm lại sau khi sửa: file cũ bị thay bằng bài sạch (chỉ cảnh báo).
	if err := os.Remove(filepath.Join(s.Dir(), "reviews", "01.json")); err != nil {
		t.Fatalf("remove review: %v", err)
	}
	clean := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "Đạt"}
	clean.Issues = []domain.ConsistencyIssue{{
		Type: "pacing", Severity: "warning", Description: "Nhịp chậm", Evidence: "đoạn 2",
		Chapters: []int{1},
	}}
	if err := s.World.SaveReview(clean); err != nil {
		t.Fatalf("clean review: %v", err)
	}
	w = tg.do("POST", path, writer, `{"applyComments":true,"chapter":1}`)
	if w.Code != 200 {
		t.Fatalf("duyệt sau lần chấm sạch = %d: %s", w.Code, w.Body)
	}
}

// TestReviewApproveGateNotificationInherits: thao tác "approve" của thông báo đi
// qua cùng cửa duyệt nên nhận đúng 409 thay vì duyệt lén.
func TestReviewApproveGateNotificationInherits(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	stubNextWorker(t, tg.g)
	s, writer := gateStory(t, tg)
	if err := s.World.SaveReview(errorReview(1)); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := tg.g.notifications.store.Record("writer", notify.PushNotification{
		Type: "gate", Story: "_/oldbook", Chapter: 1,
		Title: "Chương 1 chờ duyệt", Body: "Còn mục Bắt sửa", URL: "/x",
		Actions: []notify.Action{{Action: "approve", Title: "Duyệt"}},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	items, _, _ := tg.g.notifications.store.Inbox("writer", "")
	if len(items) == 0 {
		t.Fatal("inbox rỗng")
	}
	w := tg.do("POST", "/api/notifications/"+items[0].ID+"/action", writer, `{"action":"approve"}`)
	if w.Code != 409 {
		t.Fatalf("approve qua thông báo = %d: %s", w.Code, w.Body)
	}
	body := decodeJSONBody(t, w)
	if body["error"] != "Chương 1 còn mục Bắt sửa, chưa duyệt được." {
		t.Fatalf("error = %v", body["error"])
	}

	// Chương 2 chưa có bài thẩm duyệt ⇒ cửa mở; thân request do notify.go dựng
	// lại phải đi qua proxy với Content-Length đúng (không 502).
	if err := tg.g.notifications.store.Record("writer", notify.PushNotification{
		Type: "gate", Story: "_/oldbook", Chapter: 2,
		Title: "Chương 2 chờ duyệt", Body: "Đạt", URL: "/x",
		Actions: []notify.Action{{Action: "approve", Title: "Duyệt"}},
	}); err != nil {
		t.Fatalf("record ch2: %v", err)
	}
	all, _, _ := tg.g.notifications.store.Inbox("writer", "")
	var second string
	for _, item := range all {
		if item.Chapter == 2 {
			second = item.ID
		}
	}
	if second == "" {
		t.Fatalf("thiếu thông báo chương 2: %+v", all)
	}
	if w := tg.do("POST", "/api/notifications/"+second+"/action", writer, `{"action":"approve"}`); w.Code != 200 {
		t.Fatalf("approve chương sạch = %d: %s", w.Code, w.Body)
	}
}

// TestReviewAddressLintAloneBlocksApprove pin câu 13 / D9 ở tầng API: một vi phạm
// xưng hô mức error là mục Bắt sửa — bài thẩm duyệt blocking=true và cửa duyệt
// trả 409 kèm đúng thẻ "Xưng hô" (cả bài cũ chưa gộp mục này vào issues[]).
func TestReviewAddressLintAloneBlocksApprove(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	stubNextWorker(t, tg.g)
	s, writer := gateStory(t, tg)
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "Đạt",
		Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 90, Comment: "Ổn"}},
	}); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := s.World.SaveRuleViolations(1, []rules.Violation{{
		Rule: "address_terms", Target: "Tri Hạ → Nhược Vy", Actual: "em", Limit: "cô",
		Severity: rules.SeverityError,
		Message:  "[LỖI] chương 1, dòng 5: Tri Hạ tự xưng \"em\". Trích: Em về trước đi.",
	}}); err != nil {
		t.Fatalf("violations: %v", err)
	}

	rec := tg.do("GET", "/api/projects/_/stories/oldbook/reviews/1", writer, "")
	if rec.Code != 200 {
		t.Fatalf("reviews/1 = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Chapter == nil || !body.Chapter.Blocking {
		t.Fatalf("lỗi xưng hô mức error một mình phải blocking: %+v", body.Chapter)
	}
	if len(body.Chapter.Items) != 1 || body.Chapter.Items[0].Ref != "Xưng hô" || body.Chapter.Items[0].Level != "error" {
		t.Fatalf("items = %+v", body.Chapter.Items)
	}

	w := tg.do("POST", "/api/projects/_/stories/oldbook/next", writer, `{"applyComments":true,"chapter":1}`)
	if w.Code != 409 {
		t.Fatalf("duyệt khi chỉ còn lỗi xưng hô = %d: %s", w.Code, w.Body)
	}
	gate := decodeJSONBody(t, w)
	items, _ := gate["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items 409 = %v", gate["items"])
	}
	first, _ := items[0].(map[string]any)
	if first["ref"] != "Xưng hô" || first["level"] != "error" {
		t.Fatalf("mục 409 = %v", first)
	}
}

// TestReviewAddressItemNotDuplicated pin câu 13: bài do engine lưu đã gộp mục
// xưng hô vào issues[] thì DTO không gộp lại lần nữa (một thẻ duy nhất).
func TestReviewAddressItemNotDuplicated(t *testing.T) {
	g := &Gateway{}
	st, s := packStoryFixture(t, []int{1, 2, 3}, "nu-tan-bao-thu")
	violation := rules.Violation{
		Rule: "address_terms", Target: "Tri Hạ → Nhược Vy", Actual: "em", Limit: "cô",
		Severity: rules.SeverityError,
		Message:  "[LỖI] chương 3, dòng 5: Tri Hạ tự xưng \"em\". Trích: Em về trước đi.",
	}
	if err := s.World.SaveRuleViolations(3, []rules.Violation{violation}); err != nil {
		t.Fatalf("violations: %v", err)
	}
	merged := naolint.Issue(violation, 3)
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "rewrite", Summary: "Còn Bắt sửa",
		Issues: []domain.ConsistencyIssue{merged},
	}); err != nil {
		t.Fatalf("review: %v", err)
	}
	rec := storyReq(t, g, "GET", "/x/reviews/3", adminUser(), st, "/reviews/3", "")
	var body struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	if body.Chapter == nil {
		t.Fatalf("thiếu bài: %s", rec.Body)
	}
	if len(body.Chapter.Items) != 1 {
		t.Fatalf("items = %+v, muốn đúng một thẻ xưng hô", body.Chapter.Items)
	}
	item := body.Chapter.Items[0]
	if item.Ref != "Xưng hô" || item.RefName != "Tri Hạ → Nhược Vy" || item.Quote != "Em về trước đi." || item.Level != "error" {
		t.Fatalf("thẻ xưng hô = %+v", item)
	}
	if !body.Chapter.Blocking {
		t.Fatal("còn mục Bắt sửa ⇒ blocking")
	}
}

// TestReviewEntryRoundAt pin câu 14: round.at là thời điểm lưu vòng chấm, lấy từ
// checkpoint step=review của đúng tệp thẩm duyệt; không có checkpoint thì bỏ trống.
func TestReviewEntryRoundAt(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1})
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "ok"}); err != nil {
		t.Fatalf("review: %v", err)
	}
	read := func() *reviewEntryAPI {
		rec := storyReq(t, g, "GET", "/x/reviews/1", adminUser(), st, "/reviews/1", "")
		var body struct {
			Chapter *reviewEntryAPI `json:"chapter"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", rec.Body, err)
		}
		return body.Chapter
	}
	if entry := read(); entry == nil || entry.At != "" {
		t.Fatalf("chưa có checkpoint thì at phải rỗng: %+v", entry)
	}
	if _, err := s.Checkpoints.AppendArtifact(domain.ChapterScope(1), "review", "reviews/01.json"); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	entry := read()
	if entry == nil || entry.At == "" {
		t.Fatalf("có checkpoint thì at phải có: %+v", entry)
	}
	if _, err := time.Parse(time.RFC3339, entry.At); err != nil {
		t.Fatalf("at không phải RFC3339: %q", entry.At)
	}
}
