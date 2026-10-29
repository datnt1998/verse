package diag

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ChronicLowDimension 检测某评审维度跨多章持续低分。
func ChronicLowDimension(snap *Snapshot) []Finding {
	if len(snap.Reviews) < 2 {
		return nil
	}

	dimSums := make(map[string]float64)
	dimCounts := make(map[string]int)
	for _, r := range snap.Reviews {
		for _, d := range r.Dimensions {
			dimSums[d.Dimension] += float64(d.Score)
			dimCounts[d.Dimension]++
		}
	}

	var findings []Finding
	for name, sum := range dimSums {
		count := dimCounts[name]
		if count < 2 {
			continue
		}
		avg := sum / float64(count)
		if avg >= ThresholdDimScoreLow {
			continue
		}
		findings = append(findings, Finding{
			Rule:       "ChronicLowDimension",
			Category:   CatQuality,
			Severity:   SevWarning,
			Confidence: ConfMedium,
			AutoLevel:  AutoNone,
			Target:     "prompt.writer",
			Title:      fmt.Sprintf("Tiêu chí %s có điểm thấp kéo dài", dimensionLabel(name)),
			Evidence:   fmt.Sprintf("Qua %d lần thẩm duyệt, tiêu chí này đạt trung bình %.1f/100 điểm.", count, avg),
			Suggestion: "Mở các bài thẩm duyệt, xem lý do điểm thấp và yêu cầu Người viết sửa những điểm yếu lặp lại theo góp ý của Biên tập viên.",
			Technical:  fmt.Sprintf("dimension=%s", name),
		})
	}
	return findings
}

// ContractMissPattern 检测合同履约率过低。
func ContractMissPattern(snap *Snapshot) []Finding {
	if len(snap.Reviews) == 0 {
		return nil
	}

	var total, missed int
	var missedChapters []int
	for ch, r := range snap.Reviews {
		total++
		if r.ContractStatus == "partial" || r.ContractStatus == "missed" {
			missed++
			missedChapters = append(missedChapters, ch)
		}
	}
	if total == 0 {
		return nil
	}
	rate := float64(missed) / float64(total)
	if rate <= ThresholdContractMissRate {
		return nil
	}
	return []Finding{{
		Rule:       "ContractMissPattern",
		Category:   CatQuality,
		Severity:   SevWarning,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "prompt.writer",
		Title:      fmt.Sprintf("Hợp đồng chương chưa được đáp ứng đầy đủ (%.0f%% chưa đạt)", rate*100),
		Evidence:   fmt.Sprintf("Chương %s chưa đạt hợp đồng chương (%d/%d).", sortedChaptersText(missedChapters), missed, total),
		Suggestion: "Yêu cầu Người viết bám hợp đồng chương và sửa các yêu cầu còn thiếu. Nếu một chương ôm quá nhiều việc, hãy cùng Kiến trúc sư điều chỉnh kế hoạch chương.",
	}}
}

// HookWeakChain 检测章节 hook 评分连续偏弱。
func HookWeakChain(snap *Snapshot) []Finding {
	if len(snap.Reviews) < ThresholdHookWeakChain {
		return nil
	}

	chapters := sortedChapterReviews(snap)
	var weakChain []int
	for _, ch := range chapters {
		review := snap.Reviews[ch]
		if review == nil || review.Scope != "chapter" {
			continue
		}
		hook := review.Dimension("hook")
		if hook == nil || hook.Score >= ThresholdHookWeakScore {
			if len(weakChain) >= ThresholdHookWeakChain {
				break
			}
			weakChain = weakChain[:0]
			continue
		}
		weakChain = append(weakChain, ch)
	}
	if len(weakChain) < ThresholdHookWeakChain {
		return nil
	}

	var parts []string
	for _, ch := range weakChain {
		if hook := snap.Reviews[ch].Dimension("hook"); hook != nil {
			parts = append(parts, fmt.Sprintf("Chương %d đạt %d/100 điểm móc cuối chương.", ch, hook.Score))
		}
	}
	return []Finding{{
		Rule:       "HookWeakChain",
		Category:   CatQuality,
		Severity:   SevWarning,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "prompt.writer",
		Title:      fmt.Sprintf("Điểm móc cuối chương yếu liên tiếp (%d chương liên tiếp)", len(weakChain)),
		Evidence:   strings.Join(parts, " "),
		Suggestion: "Yêu cầu Người viết sửa đoạn kết các chương này để tạo lý do đọc tiếp rõ ràng, rồi nhờ Biên tập viên thẩm duyệt lại.",
	}}
}

// PayoffMissPattern 检测带 payoff_points 的章节长期未兑现。
func PayoffMissPattern(snap *Snapshot) []Finding {
	var total, missed int
	var missedChapters []int
	for ch, plan := range snap.Plans {
		if plan == nil || len(plan.Contract.PayoffPoints) == 0 {
			continue
		}
		review := snap.Reviews[ch]
		if review == nil {
			continue
		}
		total++
		if review.ContractStatus == "partial" || review.ContractStatus == "missed" {
			missed++
			missedChapters = append(missedChapters, ch)
		}
	}
	if total < 2 {
		return nil
	}
	rate := float64(missed) / float64(total)
	if rate <= ThresholdPayoffMissRate {
		return nil
	}
	sort.Ints(missedChapters)
	details := make([]string, 0, len(missedChapters))
	for i, ch := range missedChapters {
		prefix := "chương"
		if i == 0 {
			prefix = "Chương"
		}
		details = append(details, fmt.Sprintf("%s %d có %d điểm hồi đáp", prefix, ch, len(snap.Plans[ch].Contract.PayoffPoints)))
	}
	return []Finding{{
		Rule:       "PayoffMissPattern",
		Category:   CatQuality,
		Severity:   SevWarning,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "prompt.writer",
		Title:      fmt.Sprintf("Các điểm hồi đáp chưa được thể hiện đầy đủ (%.0f%% chương chưa đạt)", rate*100),
		Evidence:   fmt.Sprintf("%s. Có %d/%d chương có điểm hồi đáp chưa đạt hợp đồng chương.", strings.Join(details, "; "), missed, total),
		Suggestion: "Rà các điểm hồi đáp trong kế hoạch chương, giữ những điểm thực sự cần thiết và yêu cầu Người viết thể hiện rõ trong chính văn, không chỉ gợi trước.",
	}}
}

// ExcessiveRewrites 检测改写率过高。
func ExcessiveRewrites(snap *Snapshot) []Finding {
	if len(snap.Reviews) < 2 {
		return nil
	}

	var total, rewrites int
	for _, r := range snap.Reviews {
		total++
		if r.Verdict == "rewrite" {
			rewrites++
		}
	}
	if total == 0 {
		return nil
	}
	rate := float64(rewrites) / float64(total)
	if rate <= ThresholdRewriteRate {
		return nil
	}
	return []Finding{{
		Rule:       "ExcessiveRewrites",
		Category:   CatQuality,
		Severity:   SevWarning,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "prompt.editor",
		Title:      fmt.Sprintf("Tỷ lệ viết lại cao (%.0f%%)", rate*100),
		Evidence:   fmt.Sprintf("Có %d/%d lần thẩm duyệt yêu cầu viết lại.", rewrites, total),
		Suggestion: "Xem các lỗi lặp lại trong bài thẩm duyệt, thống nhất yêu cầu với Biên tập viên rồi yêu cầu Người viết sửa theo từng nhóm vấn đề.",
	}}
}

// WordCountAnomaly 检测章节字数异常。
func WordCountAnomaly(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Progress.ChapterWordCounts) < 3 {
		return nil
	}
	wc := snap.Progress.ChapterWordCounts

	var sum float64
	for _, w := range wc {
		sum += float64(w)
	}
	avg := sum / float64(len(wc))
	if avg == 0 {
		return nil
	}

	var anomalies []string
	chapters := make([]int, 0, len(wc))
	for ch := range wc {
		chapters = append(chapters, ch)
	}
	sort.Ints(chapters)
	for _, ch := range chapters {
		w := wc[ch]
		ratio := float64(w) / avg
		if ratio < ThresholdWordShortRatio {
			anomalies = append(anomalies, fmt.Sprintf("Chương %d có %d chữ, bằng %.0f%% độ dài trung bình.", ch, w, ratio*100))
		} else if ratio > ThresholdWordLongRatio {
			anomalies = append(anomalies, fmt.Sprintf("Chương %d có %d chữ, bằng %.0f%% độ dài trung bình.", ch, w, ratio*100))
		}
	}
	if len(anomalies) == 0 {
		return nil
	}
	return []Finding{{
		Rule:       "WordCountAnomaly",
		Category:   CatQuality,
		Severity:   SevInfo,
		Confidence: ConfLow,
		AutoLevel:  AutoNone,
		Target:     "context.window",
		Title:      fmt.Sprintf("Số chữ chương bất thường (trung bình %d chữ)", int(math.Round(avg))),
		Evidence:   strings.Join(anomalies, " "),
		Suggestion: "Mở các chương có độ dài khác thường để kiểm tra bản thảo có bị thiếu đoạn hay lan man. Yêu cầu Người viết bổ sung hoặc gọt gọn theo độ dài bạn mong muốn.",
	}}
}

func sortedChapterReviews(snap *Snapshot) []int {
	chapters := make([]int, 0, len(snap.Reviews))
	for ch := range snap.Reviews {
		chapters = append(chapters, ch)
	}
	sort.Ints(chapters)
	return chapters
}

func dimensionLabel(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "hook":
		return "điểm móc cuối chương"
	case "pacing":
		return "nhịp truyện"
	case "consistency", "continuity":
		return "tính nhất quán"
	case "character", "characters", "characterization":
		return "khắc hoạ nhân vật"
	case "style", "prose":
		return "văn phong"
	case "plot":
		return "cốt truyện"
	case "dialogue":
		return "hội thoại"
	case "emotion":
		return "cảm xúc"
	default:
		return "chất lượng bản thảo"
	}
}
