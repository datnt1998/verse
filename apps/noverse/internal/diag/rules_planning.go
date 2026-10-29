package diag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

// StaleForeshadow 检测拖着不动的伏笔：已过 due_by（quá hạn）或太久没被
// plant/advance（ngủ quên）。两者同一级别——都是"这条线该处理了"，不新增规则。
func StaleForeshadow(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Foreshadow) == 0 {
		return nil
	}
	latest := snap.LatestCompleted()
	completed := snap.CompletedCount()

	var stale []string
	var technical []string
	for _, f := range snap.Foreshadow {
		overdue, dormant := domain.ThreadHealth(f, latest, completed)
		switch {
		case overdue:
			stale = append(stale, fmt.Sprintf("Phục bút gieo ở chương %d đã quá hạn thu (dự kiến chương %d).", f.PlantedAt, f.DueBy))
			technical = append(technical, fmt.Sprintf("%s: planted_at=%d due_by=%d", f.ID, f.PlantedAt, f.DueBy))
		case dormant:
			touched := max(f.LastTouched, f.PlantedAt)
			stale = append(stale, fmt.Sprintf("Phục bút gieo ở chương %d đã qua %d chương chưa được đẩy tiếp.", f.PlantedAt, latest-touched))
			technical = append(technical, fmt.Sprintf("%s: planted_at=%d last_touched=%d", f.ID, f.PlantedAt, touched))
		}
	}
	if len(stale) == 0 {
		return nil
	}
	return []Finding{{
		Rule:       "StaleForeshadow",
		Category:   CatPlanning,
		Severity:   SevWarning,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "context.foreshadow",
		Title:      fmt.Sprintf("Có %d phục bút quá hạn hoặc lâu chưa được đẩy tiếp", len(stale)),
		Evidence:   strings.Join(stale, " "),
		Suggestion: "Mở sổ phục bút, chọn những mục còn phù hợp và yêu cầu Người viết đẩy tiếp hoặc thu phục bút trong các chương sắp tới.",
		Technical:  strings.Join(technical, "; "),
	}}
}

// CompassDrift 检测指南针长期未更新。
func CompassDrift(snap *Snapshot) []Finding {
	if snap.Progress == nil || !snap.Progress.Layered {
		return nil
	}
	if snap.Compass == nil {
		if snap.CompletedCount() > 5 {
			return []Finding{{
				Rule:       "CompassDrift",
				Category:   CatPlanning,
				Severity:   SevWarning,
				Confidence: ConfMedium,
				AutoLevel:  AutoNone,
				Target:     "prompt.architect",
				Title:      "Truyện dài chưa có la bàn cốt truyện",
				Evidence:   fmt.Sprintf("Đã viết %d chương nhưng chưa có la bàn cốt truyện để theo dõi các tuyến dài.", snap.CompletedCount()),
				Suggestion: "Yêu cầu Kiến trúc sư lập la bàn cốt truyện, nêu rõ đích đến và các tuyến còn dang dở.",
			}}
		}
		return nil
	}

	gap := snap.LatestCompleted() - snap.Compass.LastUpdated
	if gap <= ThresholdCompassDrift {
		return nil
	}
	return []Finding{{
		Rule:       "CompassDrift",
		Category:   CatPlanning,
		Severity:   SevInfo,
		Confidence: ConfLow,
		AutoLevel:  AutoNone,
		Target:     "prompt.architect",
		Title:      fmt.Sprintf("La bàn cốt truyện đã %d chương chưa cập nhật", gap),
		Evidence:   fmt.Sprintf("La bàn được cập nhật lần cuối ở chương %d; truyện đã viết đến chương %d và còn %d tuyến chưa khép lại.", snap.Compass.LastUpdated, snap.LatestCompleted(), len(snap.Compass.OpenThreads)),
		Suggestion: "Yêu cầu Kiến trúc sư rà lại la bàn cốt truyện theo diễn biến mới nhất, nhất là khi chuyển cung hoặc sang tập mới.",
	}}
}

// OutlineExhausted 检测大纲耗尽但小说未完结。
func OutlineExhausted(snap *Snapshot) []Finding {
	if snap.Progress == nil {
		return nil
	}
	p := snap.Progress
	if p.Phase == domain.PhaseComplete || p.Phase == domain.PhaseInit {
		return nil
	}

	completed := snap.CompletedCount()
	if completed == 0 {
		return nil
	}

	outlinedCount := p.TotalChapters
	if outlinedCount <= 0 {
		outlinedCount = len(snap.Outline)
	}
	if outlinedCount <= 0 {
		return nil
	}

	if completed < outlinedCount {
		return nil
	}

	return []Finding{{
		Rule:       "OutlineExhausted",
		Category:   CatPlanning,
		Severity:   SevCritical,
		Confidence: ConfHigh,
		AutoLevel:  AutoSafe,
		Target:     "runtime.recovery",
		Title:      "Đã viết hết phần đại cương hiện có",
		Evidence:   fmt.Sprintf("Đã viết %d/%d chương trong đại cương; truyện chưa được đánh dấu hoàn tất.", completed, outlinedCount),
		Suggestion: "Yêu cầu Kiến trúc sư mở rộng đại cương hoặc thêm tập trước khi viết tiếp. Nếu truyện đã kết thúc, hãy yêu cầu hoàn tất truyện.",
	}}
}

// MissingSummaries 检测已完成章节缺少摘要。
func MissingSummaries(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Progress.CompletedChapters) == 0 {
		return nil
	}

	var missing []int
	for _, ch := range snap.Progress.CompletedChapters {
		if _, ok := snap.Summaries[ch]; !ok {
			missing = append(missing, ch)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Ints(missing)
	return []Finding{{
		Rule:       "MissingSummaries",
		Category:   CatPlanning,
		Severity:   SevWarning,
		Confidence: ConfHigh,
		AutoLevel:  AutoNone,
		Target:     "runtime.flow",
		Title:      fmt.Sprintf("Có %d chương thiếu tóm tắt", len(missing)),
		Evidence:   fmt.Sprintf("Chương %s đã viết xong nhưng chưa có tóm tắt.", intsToStr(missing)),
		Suggestion: "Yêu cầu trợ lý bổ sung tóm tắt các chương còn thiếu để Người viết theo sát diễn biến khi viết tiếp.",
	}}
}
