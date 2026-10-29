package comments

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chapterWith ghép chương từ các đoạn văn xuôi (mỗi phần tử một đoạn).
func chapterWith(paras ...string) string {
	return strings.Join(paras, "\n\n")
}

func mustAdd(t *testing.T, s *Store, chapter int, in Input) Comment {
	t.Helper()
	c, err := s.Add(chapter, in, "dat")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	return c
}

func mustList(t *testing.T, s *Store, chapter int) []Comment {
	t.Helper()
	list, err := s.List(chapter)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return list
}

// TestParagraphsSplitSharedWithFE: khối không phải văn xuôi (tiêu đề, trích,
// danh sách, dấu ngắt cảnh) không được tính vào chỉ số đoạn — khớp bộ đếm
// kind=="paragraph" của web/src/lib/markdown.tsx.
func TestParagraphsSplitSharedWithFE(t *testing.T) {
	text := "# Chương 8\n\nĐoạn một.\n\n---\n\n> Trích người khác\n\n- mục một\n- mục hai\n\nĐoạn hai **đậm** vẫn một đoạn.\n\n### Tiểu mục\n\nĐoạn ba."
	got := Paragraphs(text)
	if len(got) != 3 {
		t.Fatalf("paragraphs = %v", got)
	}
	norm := normalizedParagraphs(text)
	if norm[0] != "Đoạn một." || norm[1] != "Đoạn hai đậm vẫn một đoạn." || norm[2] != "Đoạn ba." {
		t.Fatalf("normalized = %v", norm)
	}
}

func TestReanchorInsertAboveKeepsQuote(t *testing.T) {
	dir := t.TempDir()
	s := NewForEngine(filepath.Join(dir, "output", "novel"))
	textV1 := chapterWith("Alpha mở đầu.", "Mục tiêu giữa chương.", "Omega kết thúc.")
	c := mustAdd(t, s, 1, Input{Paragraph: ptr(1), Quote: "Mục tiêu giữa chương.", Text: "Giữ ý chính"})
	// Chen đoạn phía trên: trích vẫn khớp nguyên văn → dời theo, không cờ moved.
	if _, err := s.OnChapterWritten(1, chapterWith("Đoạn chen mới.", "Alpha mở đầu.", "Mục tiêu giữa chương.", "Omega kết thúc.")); err != nil {
		t.Fatal(err)
	}
	got := mustList(t, s, 1)[0]
	if got.ID != c.ID || got.Status != StatusPending || got.Moved {
		t.Fatalf("status = %+v", got)
	}
	if got.Anchor == nil || got.Anchor.Paragraph != 2 {
		t.Fatalf("anchor = %+v", got.Anchor)
	}
	if *got.Paragraph != 1 || got.Quote != "Mục tiêu giữa chương." {
		t.Fatalf("vị trí gốc bị ghi đè: %+v", got)
	}
	// Xoá đoạn phía trên lần nữa: quay về đoạn 1.
	if _, err := s.OnChapterWritten(1, textV1); err != nil {
		t.Fatal(err)
	}
	if got := mustList(t, s, 1)[0]; got.Anchor == nil || got.Anchor.Paragraph != 1 {
		t.Fatalf("anchor sau xoá = %+v", got.Anchor)
	}
}

func TestReanchorSplitAndMerge(t *testing.T) {
	dir := t.TempDir()
	s := NewForEngine(filepath.Join(dir, "output", "novel"))
	c := mustAdd(t, s, 1, Input{Paragraph: ptr(0), Quote: "vẫn còn nguyên trong mảnh hai", Text: "Tách đôi"})
	// Tách đoạn: trích còn nguyên trong mảnh thứ hai → anchor theo mảnh đó.
	if _, err := s.OnChapterWritten(1, chapterWith("Phần đầu không liên quan.", "Mảnh hai vẫn còn nguyên trong mảnh hai xong hẳn.", "Đoạn khác.")); err != nil {
		t.Fatal(err)
	}
	got := mustList(t, s, 1)[0]
	if got.ID != c.ID || got.Anchor == nil || got.Anchor.Paragraph != 1 || got.Status != StatusPending {
		t.Fatalf("split = %+v", got)
	}
	// Gộp lại hai mảnh: trích quay về đoạn đã gộp.
	if _, err := s.OnChapterWritten(1, chapterWith("Phần đầu không liên quan. Mảnh hai vẫn còn nguyên trong mảnh hai xong hẳn.", "Đoạn khác.")); err != nil {
		t.Fatal(err)
	}
	if got := mustList(t, s, 1)[0]; got.Anchor == nil || got.Anchor.Paragraph != 0 {
		t.Fatalf("merge = %+v", got.Anchor)
	}
}

func TestReanchorDuplicatesBecomeUnsure(t *testing.T) {
	dir := t.TempDir()
	s := NewForEngine(filepath.Join(dir, "output", "novel"))
	mustAdd(t, s, 1, Input{Paragraph: ptr(0), Quote: "Câu lặp y hệt.", Text: "Sửa chỗ này"})
	// Viết lại tạo hai đoạn chứa cùng trích → không chắc vị trí + ứng viên.
	if _, err := s.OnChapterWritten(1, chapterWith("Câu lặp y hệt.", "Đoạn xen giữa.", "Câu lặp y hệt.")); err != nil {
		t.Fatal(err)
	}
	got := mustList(t, s, 1)[0]
	if got.Status != StatusUnsure || len(got.Candidates) != 2 {
		t.Fatalf("unsure = %+v", got)
	}
	if got.Anchor != nil {
		t.Fatal("unsure phải bỏ anchor")
	}
	// Gắn vào đây: PATCH relink về một ứng viên → Chờ gửi + relinked.
	// (Không gọi Patch ở đây vì không có chính văn trên đĩa; kiểm ở test PATCH.)
	events, err := s.DrainEvents()
	if err != nil || len(events) != 1 || events[0].Chapter != 1 || events[0].Need != 1 {
		t.Fatalf("events = %+v %v", events, err)
	}
}

func TestReanchorDeletedQuoteBecomesLost(t *testing.T) {
	dir := t.TempDir()
	s := NewForEngine(filepath.Join(dir, "output", "novel"))
	mustAdd(t, s, 1, Input{Paragraph: ptr(1), Quote: "Đoạn bị xoá hoàn toàn không còn giống gì.", Text: "Bỏ phần này"})
	// Viết lại xoá hẳn đoạn và thay bằng nội dung không liên quan → Mất vị trí.
	if _, err := s.OnChapterWritten(1, chapterWith("Alpha mở đầu.", "Nội dung hoàn toàn khác viết lại.", "Omega kết thúc.")); err != nil {
		t.Fatal(err)
	}
	got := mustList(t, s, 1)[0]
	if got.Status != StatusLost || got.Anchor != nil {
		t.Fatalf("lost = %+v", got)
	}
	// Mất vị trí không tự ẩn: vẫn nằm trong danh sách.
	if len(mustList(t, s, 1)) != 1 {
		t.Fatal("lost comment auto-hidden")
	}
}

func TestReanchorRewriteFuzzyMoves(t *testing.T) {
	dir := t.TempDir()
	s := NewForEngine(filepath.Join(dir, "output", "novel"))
	old := "Anh mở cửa bước vào phòng, đặt tập tài liệu xuống bàn rồi kéo ghế ngồi xuống."
	mustAdd(t, s, 1, Input{Paragraph: ptr(0), Quote: old, Text: "Viết lại mượt hơn"})
	// Viết lại câu gần giống → khớp gần đúng duy nhất: vẫn Chờ gửi nhưng moved.
	revised := "Anh mở cửa bước vào phòng, đặt tập tài liệu lên bàn rồi ngồi xuống ghế."
	if _, err := s.OnChapterWritten(1, chapterWith(revised, "Đoạn khác hoàn toàn.")); err != nil {
		t.Fatal(err)
	}
	got := mustList(t, s, 1)[0]
	if got.Status != StatusPending || !got.Moved || got.Anchor == nil || got.Anchor.Paragraph != 0 {
		t.Fatalf("moved = %+v", got)
	}
	if got.Anchor.Quote == old {
		t.Fatal("anchor phải trỏ bản mới, không phải trích gốc")
	}
}

// TestLiveChapter8Fixture: dữ liệu truyện thật (kịch bản) — ba góp ý theo đoạn
// cũ mất trích sau khi Người viết viết lại chương 8, góp ý cả chương giữ nguyên.
// Đúng kết quả: góp ý 27/28/46 → lost, góp ý cả chương → pending không đổi.
func TestLiveChapter8Fixture(t *testing.T) {
	dir := t.TempDir()
	story := filepath.Join(dir, "story")
	if err := os.MkdirAll(filepath.Join(story, ".ainovel", "gop-y"), 0o700); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("testdata/live-8.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(story, ".ainovel", "gop-y", "8.json"), fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	draft, err := os.ReadFile("testdata/live-8.draft.md")
	if err != nil {
		t.Fatal(err)
	}
	// Chính văn hiện tại (bản nháp sau viết lại) phải nằm trong engine dir để
	// đường đọc giải được mốc neo — y hệt khi gateway GET làm mới.
	if err := os.MkdirAll(filepath.Join(story, "output", "novel", "drafts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(story, "output", "novel", "drafts", "08.draft.md"), draft, 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(story)
	list := mustList(t, s, 8)
	if len(list) != 4 {
		t.Fatalf("fixture list = %d", len(list))
	}
	byQuote := map[string]Comment{}
	for _, c := range list {
		byQuote[c.Quote] = c
	}
	if got := byQuote[""]; got.Status != StatusPending || !got.Whole {
		t.Fatalf("whole = %+v", got)
	}
	// Ba trích kịch bản đã bị xoá khỏi bản viết lại: phải vào nhóm Mất vị trí,
	// không được đoán bừa đoạn hay giữ badge ở đoạn không liên quan.
	for _, q := range []string{"CẮT SANG:", "CẢNH 9", "CẮT ĐEN."} {
		got, ok := byQuote[q]
		if !ok {
			t.Fatalf("thiếu góp ý %q", q)
		}
		if got.Status != StatusLost || got.Anchor != nil {
			t.Fatalf("góp ý %q phải là Mất vị trí, được: %+v", q, got)
		}
	}
	for _, c := range list {
		if c.Anchor != nil && c.Anchor.Quote != "" && !strings.Contains(normalizeText(string(draft)), normalizeText(c.Anchor.Quote)) {
			t.Fatalf("anchor trỏ câu không còn trong chương: %+v", c.Anchor)
		}
	}
}

func TestApplyThenWriterFixesViaFixedComments(t *testing.T) {
	dir := t.TempDir()
	engine := filepath.Join(dir, "output", "novel")
	s := NewForEngine(engine)
	old := "Câu cần viết lại cho hay hơn."
	newText := "Câu đã được viết lại cho hay hơn rồi."
	other := "Chỗ khác vẫn giữ nguyên lâu dài."
	mustAdd(t, s, 1, Input{Paragraph: ptr(0), Quote: old, Text: "Sửa câu này"})
	mustAdd(t, s, 1, Input{Paragraph: ptr(1), Quote: other, Text: "Sửa chỗ này nữa"})
	count, err := s.Apply(1, func(instruction string) error {
		if !strings.Contains(instruction, "fixed_comments") {
			t.Fatalf("instruction không dạy writer báo fixed_comments: %q", instruction)
		}
		return nil
	})
	if err != nil || count != 2 {
		t.Fatalf("apply = %d,%v", count, err)
	}
	list := mustList(t, s, 1)
	var fixedID string
	for _, c := range list {
		if c.Status != StatusSent {
			t.Fatalf("sau Apply = %+v", c)
		}
		if c.Quote == old {
			fixedID = c.ID
		}
	}
	// Writer sửa xong một góp ý, góp ý kia bỏ quên: nộp chương với fixed_comments.
	ev, err := AfterWrite(engine, 1, chapterWith(newText, other), []string{fixedID})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Fixed != 1 || ev.Unfixed != 1 || ev.Need != 0 {
		t.Fatalf("event = %+v", ev)
	}
	list = mustList(t, s, 1)
	for _, c := range list {
		switch c.ID {
		case fixedID:
			if c.Status != StatusFixed || c.FixedBy != FixedByAgent || c.FixedAt == nil {
				t.Fatalf("fixed = %+v", c)
			}
		default:
			if c.Status != StatusPending || !c.Unfixed {
				t.Fatalf("unfixed = %+v", c)
			}
		}
	}
	// Lũy đẳng: báo lại cùng id trên bản nộp sau không đổi gì thêm.
	if _, err := AfterWrite(engine, 1, chapterWith(newText, other), []string{fixedID}); err != nil {
		t.Fatal(err)
	}
	if got := mustList(t, s, 1)[0]; got.Status != StatusFixed {
		t.Fatalf("re-report = %+v", got)
	}
	// Báo id không tồn tại: bỏ qua, không lỗi.
	if _, err := AfterWrite(engine, 1, chapterWith(newText, other), []string{"KHONG-TON-TAI"}); err != nil {
		t.Fatal(err)
	}
	// Duyệt lần kế tiếp: Đã sửa → Đã xử lý.
	if err := s.ResolveAllFixed(); err != nil {
		t.Fatal(err)
	}
	for _, c := range mustList(t, s, 1) {
		if c.ID == fixedID && (c.Status != StatusDone || c.ResolvedAt == nil) {
			t.Fatalf("resolved = %+v", c)
		}
	}
}

// TestSentQuoteGoneMarksFixed: góp ý đã gửi mà trích gốc mất nhưng thấy đoạn
// thay duy nhất → fixed(quote_gone) kèm newQuote cho thẻ Trước/Sau.
func TestSentQuoteGoneMarksFixed(t *testing.T) {
	dir := t.TempDir()
	engine := filepath.Join(dir, "output", "novel")
	s := NewForEngine(engine)
	old := "Anh mở cửa bước vào phòng, đặt tập tài liệu xuống bàn rồi kéo ghế ngồi xuống."
	c := mustAdd(t, s, 1, Input{Paragraph: ptr(0), Quote: old, Text: "Viết lại mượt hơn"})
	if _, err := s.Apply(1, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	revised := "Anh mở cửa bước vào phòng, đặt tập tài liệu lên bàn rồi ngồi xuống ghế."
	ev, err := s.OnChapterWritten(1, chapterWith(revised, "Đoạn hoàn toàn khác ở cuối chương."))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Fixed != 1 {
		t.Fatalf("event = %+v", ev)
	}
	got := mustList(t, s, 1)[0]
	if got.ID != c.ID || got.Status != StatusFixed || got.FixedBy != FixedByQuoteGone {
		t.Fatalf("fixed = %+v", got)
	}
	if got.NewQuote == "" || got.Quote != old {
		t.Fatalf("newQuote = %+v", got)
	}
}

// TestSentQuoteStillThereBacksToPending: Người viết chưa sửa chỗ này (trích còn
// nguyên sau khi ghi chương) → về Chờ gửi + unfixed, sẵn sàng gửi lại.
func TestSentQuoteStillThereBacksToPending(t *testing.T) {
	dir := t.TempDir()
	engine := filepath.Join(dir, "output", "novel")
	s := NewForEngine(engine)
	quote := "Câu này Người viết quên chưa sửa."
	mustAdd(t, s, 1, Input{Paragraph: ptr(0), Quote: quote, Text: "Sửa câu này"})
	if _, err := s.Apply(1, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	ev, err := s.OnChapterWritten(1, chapterWith(quote, "Đoạn mới viết thêm."))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Unfixed != 1 || ev.Fixed != 0 {
		t.Fatalf("event = %+v", ev)
	}
	got := mustList(t, s, 1)[0]
	if got.Status != StatusPending || !got.Unfixed {
		t.Fatalf("unfixed = %+v", got)
	}
}

func TestPatchActions(t *testing.T) {
	dir := t.TempDir()
	story := filepath.Join(dir, "story")
	engine := filepath.Join(story, "output", "novel")
	if err := os.MkdirAll(filepath.Join(engine, "chapters"), 0o700); err != nil {
		t.Fatal(err)
	}
	text := chapterWith("Câu lặp y hệt.", "Đoạn xen giữa giữ nguyên.", "Câu lặp y hệt.")
	if err := os.WriteFile(filepath.Join(engine, "chapters", "01.md"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(story)
	c := mustAdd(t, s, 1, Input{Paragraph: ptr(0), Quote: "Câu lặp y hệt.", Text: "Sửa chỗ này"})
	// Tạo trùng: không chắc vị trí ở lần đọc kế.
	if _, err := s.OnChapterWritten(1, text); err != nil {
		t.Fatal(err)
	}
	if got := mustList(t, s, 1)[0]; got.Status != StatusUnsure {
		t.Fatalf("setup unsure = %+v", got)
	}
	// relink về ứng viên thứ hai.
	got, err := s.Patch(1, c.ID, PatchInput{Action: ActionRelink, Paragraph: ptr(2), Quote: "Câu lặp y hệt."})
	if err != nil {
		t.Fatalf("relink: %v", err)
	}
	if got.Status != StatusPending || !got.Relinked || got.Anchor == nil || got.Anchor.Paragraph != 2 {
		t.Fatalf("relinked = %+v", got)
	}
	// reopen trên góp ý đang chờ: no-op, vẫn pending.
	got, err = s.Patch(1, c.ID, PatchInput{Action: ActionReopen})
	if err != nil || got.Status != StatusPending {
		t.Fatalf("reopen pending = %+v %v", got, err)
	}
	// Gửi lại cả chương.
	got, err = s.Patch(1, c.ID, PatchInput{Action: ActionResendWhole})
	if err != nil || !got.Whole || got.Status != StatusPending {
		t.Fatalf("resend_whole = %+v %v", got, err)
	}
	// Compose gom nó vào nhóm cả chương kèm trích gốc.
	inst := Compose(1, mustList(t, s, 1))
	if !strings.Contains(inst, "Phạm vi: cả chương") || !strings.Contains(inst, "Câu lặp y hệt.") {
		t.Fatalf("compose whole = %q", inst)
	}
	// resolve → done; resolve lại lần nữa vẫn ok (lũy đẳng).
	got, err = s.Patch(1, c.ID, PatchInput{Action: ActionResolve})
	if err != nil || got.Status != StatusDone || got.ResolvedAt == nil {
		t.Fatalf("resolve = %+v %v", got, err)
	}
	if _, err := s.Patch(1, c.ID, PatchInput{Action: ActionResolve}); err != nil {
		t.Fatal(err)
	}
	// Lỗi là câu tiếng Việt.
	if _, err := s.Patch(1, c.ID, PatchInput{Action: "xoa"}); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "thao tác") {
		t.Fatalf("unknown action = %v", err)
	}
	if _, err := s.Patch(1, "KHONG-CO", PatchInput{Action: ActionResolve}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	reopened, err := s.Patch(1, c.ID, PatchInput{Action: ActionRelink, Paragraph: ptr(5), Quote: "x"})
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "không còn") {
		t.Fatalf("relink out of range = %+v %v", reopened, err)
	}
}

// TestLegacyMigration: tệp cũ (paragraph+quote, appliedAt cho đã gửi) migrate
// không mất dữ liệu: status suy ra đúng, appliedAt thành sentAt, anchor dựng lại.
func TestLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	story := filepath.Join(dir, "story")
	engine := filepath.Join(story, "output", "novel")
	if err := os.MkdirAll(filepath.Join(engine, "chapters"), 0o700); err != nil {
		t.Fatal(err)
	}
	text := chapterWith("Alpha mở đầu.", "Mục tiêu giữa chương.", "Omega kết thúc.")
	if err := os.WriteFile(filepath.Join(engine, "chapters", "02.md"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(story, ".ainovel", "gop-y"), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `[
	  {"id":"A","paragraph":null,"quote":"","chips":[],"text":"Cả chương","author":"dat","createdAt":"2026-10-28T08:00:00Z"},
	  {"id":"B","paragraph":1,"quote":"Mục tiêu giữa chương.","chips":[],"text":"Đã gửi","author":"dat","createdAt":"2026-10-28T08:00:01Z","appliedAt":"2026-10-28T09:00:00Z"},
	  {"id":"C","paragraph":2,"quote":"Omega kết thúc.","chips":[],"text":"Chưa gửi","author":"dat","createdAt":"2026-10-28T08:00:02Z"}
	]`
	if err := os.WriteFile(filepath.Join(story, ".ainovel", "gop-y", "2.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(story)
	list := mustList(t, s, 2)
	byID := map[string]Comment{}
	for _, c := range list {
		byID[c.ID] = c
	}
	if got := byID["A"]; got.Status != StatusPending || !got.Whole {
		t.Fatalf("A = %+v", got)
	}
	if got := byID["B"]; got.Status != StatusSent || got.SentAt == nil || got.SentAt.Format("15:04") != "09:00" {
		t.Fatalf("B = %+v", got)
	}
	if got := byID["B"]; got.Anchor == nil || got.Anchor.Paragraph != 1 {
		t.Fatalf("B anchor = %+v", got.Anchor)
	}
	if got := byID["C"]; got.Status != StatusPending || got.Anchor == nil || got.Anchor.Paragraph != 2 {
		t.Fatalf("C = %+v", got)
	}
	// Tệp trên đĩa đã migrate: không còn appliedAt.
	raw, err := os.ReadFile(filepath.Join(story, ".ainovel", "gop-y", "2.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "appliedAt") {
		t.Fatal("appliedAt còn sót trong tệp sau migrate")
	}
	var check []Comment
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Fatal(err)
	}
}
