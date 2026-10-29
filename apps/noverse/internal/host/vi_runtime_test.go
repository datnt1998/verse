package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/agent"
	"github.com/datnt1998/noverse/internal/domain"
	storepkg "github.com/datnt1998/noverse/internal/store"
)

func TestFormatCountGroupsThousandsLikeViVN(t *testing.T) {
	cases := map[int]string{0: "0", 999: "999", 1000: "1.000", 8766: "8.766", 1234567: "1.234.567", -4321: "-4.321"}
	for in, want := range cases {
		if got := formatCount(in); got != want {
			t.Errorf("formatCount(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestCompletionSummaryVietnamese(t *testing.T) {
	progress := domain.Progress{CompletedChapters: []int{1, 2, 3}, TotalWordCount: 12345}
	got := completionSummary(progress, domain.BookMetadata{Title: "Thọ Nguyên Trướng"})
	if want := "«Thọ Nguyên Trướng» hoàn thành: 3 chương, 12.345 chữ"; got != want {
		t.Fatalf("completionSummary = %q, want %q", got, want)
	}
}

// Thẻ “Lần nộp gần nhất / thẩm duyệt gần nhất” trên TUI hiển thị nguyên văn LastCommitSummary /
// LastReviewSummary, nên host phải phát sẵn tiếng Việt (web vẫn dịch dạng Trung cũ để tương thích).
func TestFillDetailsSummariesVietnamese(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	review := domain.ReviewEntry{
		Chapter: 8, Scope: "global", Verdict: "polish", Summary: "nhịp chậm",
		Issues:           []domain.ConsistencyIssue{{Type: "pacing", Severity: "warning", Description: "chậm"}, {Type: "hook", Severity: "error", Description: "thiếu móc"}},
		AffectedChapters: []int{8, 9},
	}
	if err := st.World.SaveReview(review); err != nil {
		t.Fatalf("SaveReview: %v", err)
	}
	h := &Host{store: st}
	snap := &UISnapshot{}
	h.fillDetails(snap, &domain.Progress{CompletedChapters: []int{8}, ChapterWordCounts: map[int]int{8: 8766}})

	if want := "Chương 8 · 8.766 chữ"; snap.LastCommitSummary != want {
		t.Errorf("LastCommitSummary = %q, want %q", snap.LastCommitSummary, want)
	}
	if want := "Cần sửa · 2 vấn đề · ảnh hưởng chương 8, 9"; snap.LastReviewSummary != want {
		t.Errorf("LastReviewSummary = %q, want %q", snap.LastReviewSummary, want)
	}
}

func TestDescribeResumeRewriteLabelVietnamese(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	progress := &domain.Progress{Phase: domain.PhaseWriting, PendingRewrites: []int{3}}
	got, err := describeResume(st, progress)
	if err != nil {
		t.Fatalf("describeResume: %v", err)
	}
	if want := "Khôi phục viết lại: 1 chương chờ xử lý"; got != want {
		t.Fatalf("describeResume = %q, want %q", got, want)
	}
	progress.Flow = domain.FlowPolishing
	if got, _ := describeResume(st, progress); got != "Khôi phục gọt giũa: 1 chương chờ xử lý" {
		t.Fatalf("describeResume(polishing) = %q", got)
	}
}

// The resume label in the planning phase names the next step that will run —
// “Khôi phục · tiếp tục từ Lưu dàn ý” (21-dev-prompt.md Gói O mục 1) — thay vì
// lặp lại tên giai đoạn. Bước lấy từ cùng dữ kiện Flow Router dùng: mục đầu của
// FoundationMissing, dịch qua bảng nhãn bước (labels.ts › TOOLS/FOUNDATION).
func TestDescribeResumePlanningPhaseVietnamese(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Kho mới khởi tạo: mọi thiết lập đều thiếu, bước kế tiếp là lưu thông tin truyện.
	if got, err := describeResume(st, &domain.Progress{Phase: domain.PhaseInit}); err != nil || got != "Khôi phục · tiếp tục từ Lưu thông tin truyện" {
		t.Fatalf("describeResume(init, fresh store) = %q, err=%v", got, err)
	}
	if err := st.Book.Save(domain.BookMetadata{Title: "Tuổi Sáng Phản Bộn", Synopsis: "Một nữ thư ký vạch trần vụ cướp kịch bản."}); err != nil {
		t.Fatalf("Book.Save: %v", err)
	}
	if got, _ := describeResume(st, &domain.Progress{Phase: domain.PhasePremise}); got != "Khôi phục · tiếp tục từ Lưu tiền đề" {
		t.Errorf("describeResume(premise) = %q", got)
	}
	if err := st.Outline.SavePremise("# Tuổi Sáng Phản Bộn\n\n## Xung đột cốt lõi\nDanh tính và tác quyền."); err != nil {
		t.Fatalf("SavePremise: %v", err)
	}
	if got, _ := describeResume(st, &domain.Progress{Phase: domain.PhaseOutline}); got != "Khôi phục · tiếp tục từ Lưu dàn ý" {
		t.Errorf("describeResume(outline) = %q", got)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Mở đầu", CoreEvent: "Gặp người vắng mặt"}}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := st.Characters.Save([]domain.Character{{Name: "Hạ Vy", Role: "Nữ chính"}}); err != nil {
		t.Fatalf("Characters.Save: %v", err)
	}
	if err := st.World.SaveWorldRules([]domain.WorldRule{{Category: "society", Rule: "Ký giúp việc bằng giấy"}}); err != nil {
		t.Fatalf("SaveWorldRules: %v", err)
	}
	// Mọi sản phẩm đã lưu: chỉ còn bước soát nền truyện trước khi sang giai đoạn sáng tác.
	if got, _ := describeResume(st, &domain.Progress{Phase: domain.PhaseOutline}); got != "Khôi phục · tiếp tục từ Soát nền truyện" {
		t.Errorf("describeResume(audit) = %q", got)
	}
}

// Premise cũ của dự án tiếng Việt dùng tiêu đề "## Xung đột cốt lõi"; migrate phải nhận cả dạng này
// bên cạnh "## 核心冲突" (TestUpgradeProjectMigratesLegacyBook giữ ca tiếng Trung).
func TestUpgradeProjectMigratesLegacyBookVietnameseHeading(t *testing.T) {
	dir := t.TempDir()
	st := storepkg.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	premise := "# «Thọ Nguyên Trướng»\n\n## Xung đột cốt lõi\n\nPhàm nhân đổi thọ nguyên lấy linh tính.\n\n## Mục tiêu nhân vật chính\n\nSống sót."
	if err := st.Outline.SavePremise(premise); err != nil {
		t.Fatalf("SavePremise: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta", "progress.json"), []byte(`{"novel_name":"Thọ Nguyên Trướng"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := upgradeProject(st); err != nil {
		t.Fatalf("upgradeProject: %v", err)
	}
	book, err := st.Book.Load()
	if err != nil {
		t.Fatalf("Load book: %v", err)
	}
	if book == nil || book.Title != "Thọ Nguyên Trướng" || book.Synopsis != "Phàm nhân đổi thọ nguyên lấy linh tính." {
		t.Fatalf("unexpected migrated book: %+v", book)
	}
}

// Sự kiện nén ngữ cảnh hiển thị nguyên văn trên TUI/web: khoá chiến lược của agentcore phải đổi thành nhãn Việt.
func TestContextProgressSummaryUsesVietnameseStrategyLabel(t *testing.T) {
	var events []Event
	o := testObserver(&events)
	o.handleContextProgress(agentcore.Event{Progress: &agentcore.ProgressPayload{
		Agent: "writer",
		Meta:  json.RawMessage(`{"tokens":90000,"context_window":100000,"percent":90,"scope":"chapter","strategy":"full_summary"}`),
	}})
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	if want := "writer ngữ cảnh 90% (90000/100000) · chiến lược: tóm tắt toàn diện"; events[0].Summary != want {
		t.Fatalf("summary = %q, want %q", events[0].Summary, want)
	}
}

// web/src/i18n/hostStrings.ts STREAM_ROUNDS khớp header ✻ theo đúng chuỗi host phát: mọi header
// trong toolDisplays phải có mặt ở đó cùng tool, nếu không web rơi về kind raw (mất thẻ diff/commit/plan).
func TestStreamHeadersMatchWebStreamRounds(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "i18n", "hostStrings.ts"))
	if err != nil {
		t.Fatalf("read hostStrings.ts: %v", err)
	}
	web := string(src)
	for tool, display := range toolDisplays {
		if display.header == "" {
			continue
		}
		header := strings.TrimPrefix(display.header, "✻ ")
		entry := fmt.Sprintf("header: '%s'", header)
		idx := strings.Index(web, entry)
		if idx < 0 {
			t.Errorf("%s header %q missing from STREAM_ROUNDS", tool, header)
			continue
		}
		line := web[idx:]
		if end := strings.IndexByte(line, '\n'); end >= 0 {
			line = line[:end]
		}
		if !strings.Contains(line, fmt.Sprintf("tool: '%s'", tool)) {
			t.Errorf("STREAM_ROUNDS entry for %q does not map to tool %s: %s", header, tool, line)
		}
	}
}
