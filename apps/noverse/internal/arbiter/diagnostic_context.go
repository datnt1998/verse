package arbiter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// Use the same story rules and fingerprints as the diagnostic panel. §N in a
// finding's title names a rule section, never its position in the sorted report.
func collectDiagnosticContext(st *storepkg.Store, tier string, recent []RecentDecision) (string, error) {
	decisions, err := st.World.LoadReviewDecisions()
	if err != nil {
		return "", fmt.Errorf("đọc quyết định thẩm duyệt cho chẩn đoán: %w", err)
	}
	report, _ := diag.Diagnose(st)
	var b strings.Builder
	b.WriteString("Chẩn đoán hiện tại (tham chiếu §N là mục luật, không phải số thứ tự; mã mục phân biệt các phát hiện cùng §):")
	if len(report.Findings) == 0 {
		b.WriteString("\nKhông có phát hiện còn mở.")
	}
	architect := "architect_long"
	if tier == "short" {
		architect = "architect_short"
	}
	for _, finding := range report.Findings {
		fmt.Fprintf(&b, "\n- [%s] %s · %s: %s Đề xuất: %s", diag.FindingFingerprint(finding), finding.Rule, finding.Title, finding.Evidence, finding.Suggestion)
		if finding.ChapterFrom > 0 && finding.ChapterTo >= finding.ChapterFrom {
			fmt.Fprintf(&b, " Phạm vi đại cương: chương %d–%d.", finding.ChapterFrom, finding.ChapterTo)
		}
		if finding.Target == "prompt.architect" {
			fmt.Fprintf(&b, " Giao %s: đọc novel_context, đối chiếu chính văn chương liên quan bằng read_chapter (source=final), rồi revise_outline; với chương đã hoàn thành chỉ đồng bộ nhãn theo chính văn bằng reconcile_completed=true, không tạo tình tiết mới hay sửa chính văn.", architect)
		}
	}
	ids := make([]string, 0, len(decisions))
	for id, decision := range decisions {
		if decision.State == "kept" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		b.WriteString("\nQuyết định người dùng đã lưu (không tự sửa các mục Giữ nguyên):")
		for _, id := range ids {
			d := decisions[id]
			fmt.Fprintf(&b, "\n- [%s] %s · chương %d · %s: Giữ nguyên — %s", id, d.Ref, d.Chapter, d.Text, d.Reason)
		}
	}
	if len(recent) > 0 {
		b.WriteString("\nGóp ý sửa / lựa chọn đã gửi (tóm tắt, phải đọc nguyên văn facts.recent_decisions để giữ đủ phạm vi và điều cấm; không chứng minh đã sửa xong):")
		for _, turn := range recent {
			fmt.Fprintf(&b, "\n- %s: %s", turn.At, truncateRunes(turn.Input, 300))
		}
	}
	return b.String(), nil
}
