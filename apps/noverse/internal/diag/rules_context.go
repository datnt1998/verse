package diag

import (
	"fmt"
	"strings"
)

// GhostCharacter 检测 core/important 角色长期未出现。
func GhostCharacter(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Characters) == 0 || len(snap.Summaries) == 0 {
		return nil
	}
	completed := snap.CompletedCount()
	if completed < 5 {
		return nil
	}

	// 计算每个角色最后出现的章节号
	lastSeen := make(map[string]int)
	for ch, s := range snap.Summaries {
		for _, name := range s.Characters {
			if ch > lastSeen[name] {
				lastSeen[name] = ch
			}
		}
	}

	threshold := completed / 3
	if threshold < 5 {
		threshold = 5
	}
	latest := snap.LatestCompleted()

	var ghosts []string
	for _, c := range snap.Characters {
		if c.Tier != "core" && c.Tier != "important" {
			continue
		}
		seen, ok := lastSeen[c.Name]
		if !ok {
			// 也检查别名
			for _, alias := range c.Aliases {
				if s, exists := lastSeen[alias]; exists && s > seen {
					seen = s
					ok = true
				}
			}
		}
		gap := latest - seen
		if !ok {
			ghosts = append(ghosts, fmt.Sprintf("%s chưa xuất hiện trong các bản tóm tắt.", c.Name))
		} else if gap > threshold {
			ghosts = append(ghosts, fmt.Sprintf("%s xuất hiện lần cuối ở chương %d và đã vắng %d chương.", c.Name, seen, gap))
		}
	}
	if len(ghosts) == 0 {
		return nil
	}
	return []Finding{{
		Rule:       "GhostCharacter",
		Category:   CatContext,
		Severity:   SevInfo,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "context.characters",
		Title:      fmt.Sprintf("Nhân vật biến mất: %d nhân vật cốt lõi vắng lâu", len(ghosts)),
		Evidence:   strings.Join(ghosts, " "),
		Suggestion: "Gửi chỉ thị cho Người viết đưa các nhân vật còn vai trò trở lại. Nếu nhân vật đã rời tuyến chính, hãy cập nhật mức độ quan trọng trong hồ sơ nhân vật.",
	}}
}

// TimelineGaps 检测已完成章节缺少时间线事件。
func TimelineGaps(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Progress.CompletedChapters) == 0 {
		return nil
	}
	if len(snap.Timeline) == 0 && snap.CompletedCount() > 0 {
		return []Finding{{
			Rule:       "TimelineGaps",
			Category:   CatContext,
			Severity:   SevInfo,
			Confidence: ConfMedium,
			AutoLevel:  AutoNone,
			Target:     "context.timeline",
			Title:      "Dòng thời gian trống",
			Evidence:   fmt.Sprintf("Đã viết %d chương nhưng chưa có sự kiện nào trong dòng thời gian.", snap.CompletedCount()),
			Suggestion: "Yêu cầu trợ lý bổ sung các sự kiện chính của những chương đã viết vào dòng thời gian.",
		}}
	}

	// 建立章节→事件映射
	chaptersWithEvents := make(map[int]bool)
	for _, e := range snap.Timeline {
		chaptersWithEvents[e.Chapter] = true
	}

	var missing []int
	for _, ch := range snap.Progress.CompletedChapters {
		if !chaptersWithEvents[ch] {
			missing = append(missing, ch)
		}
	}
	// 允许少量缺失（某些过渡章可能确实无重大事件）
	if len(missing) == 0 || float64(len(missing))/float64(snap.CompletedCount()) < ThresholdTimelineGapRate {
		return nil
	}
	return []Finding{{
		Rule:       "TimelineGaps",
		Category:   CatContext,
		Severity:   SevInfo,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "context.timeline",
		Title:      fmt.Sprintf("Thiếu hụt dòng thời gian: %d chương không có bản ghi sự kiện", len(missing)),
		Evidence:   fmt.Sprintf("Chương %s chưa có sự kiện được ghi vào dòng thời gian.", sortedChaptersText(missing)),
		Suggestion: "Mở dòng thời gian, đối chiếu các chương còn thiếu và yêu cầu trợ lý bổ sung những sự kiện quan trọng.",
	}}
}

// RelationshipStagnation 检测关系数据停止更新。
func RelationshipStagnation(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Relationships) == 0 {
		return nil
	}
	completed := snap.CompletedCount()
	if completed < 6 {
		return nil
	}

	// 找到关系数据的最新章节
	latestRelCh := 0
	for _, r := range snap.Relationships {
		if r.Chapter > latestRelCh {
			latestRelCh = r.Chapter
		}
	}

	// 如果最新关系数据在前 1/3，判定为停滞
	cutoff := snap.LatestCompleted() - completed/3
	if latestRelCh >= cutoff {
		return nil
	}
	return []Finding{{
		Rule:       "RelationshipStagnation",
		Category:   CatContext,
		Severity:   SevInfo,
		Confidence: ConfLow,
		AutoLevel:  AutoNone,
		Target:     "context.relationships",
		Title:      "Quan hệ nhân vật lâu chưa được cập nhật",
		Evidence:   fmt.Sprintf("Có %d bản ghi quan hệ; lần cập nhật gần nhất ở chương %d, trong khi truyện đã viết đến chương %d.", len(snap.Relationships), latestRelCh, snap.LatestCompleted()),
		Suggestion: "Mở quan hệ nhân vật để đối chiếu diễn biến mới. Yêu cầu trợ lý ghi lại các thay đổi còn thiếu; nếu quan hệ không đổi thì có thể bỏ qua phát hiện này.",
	}}
}
