package diag

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

// InvalidPendingRewrites 检测返工队列里混入未完成章节。
func InvalidPendingRewrites(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Progress.PendingRewrites) == 0 {
		return nil
	}
	p := snap.Progress
	completed := append([]int(nil), p.CompletedChapters...)
	slices.Sort(completed)

	var invalid []int
	for _, ch := range p.PendingRewrites {
		if ch <= 0 || !slices.Contains(completed, ch) {
			invalid = append(invalid, ch)
		}
	}
	if len(invalid) == 0 {
		return nil
	}
	slices.Sort(invalid)
	return []Finding{{
		Rule:       "InvalidPendingRewrites",
		Category:   CatFlow,
		Severity:   SevCritical,
		Confidence: ConfHigh,
		AutoLevel:  AutoSuggest,
		Target:     "meta/progress.json",
		Title:      "Hàng đợi viết lại có chương chưa viết xong",
		Evidence:   fmt.Sprintf("Chương %s đang chờ viết lại nhưng chưa được ghi nhận là đã hoàn thành.", intsToStr(invalid)),
		Suggestion: "Tạm dừng sáng tác và yêu cầu trợ lý rà lại hàng đợi viết lại. Chỉ tiếp tục sau khi đã bỏ các chương chưa viết xong khỏi hàng đợi.",
		Technical:  fmt.Sprintf("pending_rewrites=[%s], completed_chapters=[%s], flow=%s", intsToStr(p.PendingRewrites), intsToStr(completed), p.Flow),
	}}
}

// RewritePendingPressure 检测存在待改写章节（当前仅检测状态存在，不判定停滞）。
func RewritePendingPressure(snap *Snapshot) []Finding {
	if snap.Progress == nil {
		return nil
	}
	p := snap.Progress
	if len(p.PendingRewrites) == 0 {
		return nil
	}
	if p.Flow != domain.FlowRewriting && p.Flow != domain.FlowPolishing {
		return nil
	}
	chapters := sortedChaptersText(p.PendingRewrites)
	return []Finding{{
		Rule:       "RewritePendingPressure",
		Category:   CatFlow,
		Severity:   SevWarning,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "runtime.flow",
		Title:      fmt.Sprintf("Có %d chương đang chờ viết lại", len(p.PendingRewrites)),
		Evidence:   fmt.Sprintf("Chương %s đang nằm trong hàng đợi viết lại.", chapters),
		Suggestion: "Mở bài thẩm duyệt của các chương này, xem các mục cần sửa rồi yêu cầu Người viết xử lý trước khi viết tiếp.",
	}}
}

// OrphanedSteer 检测未消费的用户转向指令。
func OrphanedSteer(snap *Snapshot) []Finding {
	if snap.RunMeta == nil || snap.RunMeta.PendingSteer == "" {
		return nil
	}
	if snap.Progress != nil && snap.Progress.Flow == domain.FlowSteering {
		return nil // 正在处理中，不算孤立
	}
	return []Finding{{
		Rule:       "OrphanedSteer",
		Category:   CatFlow,
		Severity:   SevWarning,
		Confidence: ConfHigh,
		AutoLevel:  AutoSafe,
		Target:     "runtime.recovery",
		Title:      "Có chỉ thị chuyển hướng chưa được xử lý",
		Evidence:   "Chỉ thị chuyển hướng của bạn đã được lưu nhưng chưa được áp dụng vào truyện.",
		Suggestion: "Yêu cầu trợ lý xử lý chỉ thị chuyển hướng đang chờ trước khi tiếp tục sáng tác.",
		Technical:  fmt.Sprintf("pending_steer=%q, flow=%s", snap.RunMeta.PendingSteer, flowStr(snap.Progress)),
	}}
}

// PhaseFlowMismatch 检测阶段与流程状态不匹配。
func PhaseFlowMismatch(snap *Snapshot) []Finding {
	if snap.Progress == nil {
		return nil
	}
	p := snap.Progress
	if p.Phase == domain.PhaseWriting || p.Phase == "" {
		return nil
	}
	if p.Flow == "" || p.Flow == domain.FlowWriting {
		return nil
	}
	return []Finding{{
		Rule:       "PhaseFlowMismatch",
		Category:   CatFlow,
		Severity:   SevCritical,
		Confidence: ConfHigh,
		AutoLevel:  AutoSafe,
		Target:     "runtime.flow",
		Title:      "Giai đoạn sáng tác và công việc hiện tại không khớp",
		Evidence:   "Truyện không ở giai đoạn viết nhưng vẫn có công việc viết lại, thẩm duyệt hoặc can thiệp đang được ghi nhận.",
		Suggestion: "Tạm dừng và yêu cầu trợ lý rà lại tiến độ truyện, xử lý công việc còn dở trước khi tiếp tục.",
		Technical:  fmt.Sprintf("phase=%s, flow=%s", p.Phase, p.Flow),
	}}
}

// ChapterGaps 检测已完成章节列表中的跳号。
func ChapterGaps(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Progress.CompletedChapters) < 2 {
		return nil
	}
	sorted := append([]int(nil), snap.Progress.CompletedChapters...)
	sort.Ints(sorted)

	var gaps []int
	for i := 1; i < len(sorted); i++ {
		for ch := sorted[i-1] + 1; ch < sorted[i]; ch++ {
			gaps = append(gaps, ch)
		}
	}
	if len(gaps) == 0 {
		return nil
	}
	return []Finding{{
		Rule:       "ChapterGaps",
		Category:   CatFlow,
		Severity:   SevWarning,
		Confidence: ConfHigh,
		AutoLevel:  AutoNone,
		Target:     "runtime.flow",
		Title:      "Danh sách chương đã viết bị thiếu số",
		Evidence:   fmt.Sprintf("Chương %s chưa được ghi nhận hoàn thành, dù các chương sau đã viết xong.", intsToStr(gaps)),
		Suggestion: "Mở danh sách chương để kiểm tra các chương còn thiếu, rồi yêu cầu trợ lý khôi phục bản nộp hoặc viết bổ sung trước khi đi tiếp.",
	}}
}

func flowStr(p *domain.Progress) string {
	if p == nil {
		return "<nil>"
	}
	return string(p.Flow)
}

func truncStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-3]) + "..."
}

func intsToStr(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, ", ")
}

func sortedChaptersText(nums []int) string {
	sorted := slices.Clone(nums)
	slices.Sort(sorted)
	return intsToStr(sorted)
}
