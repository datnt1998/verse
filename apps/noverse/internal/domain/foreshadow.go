package domain

import (
	"fmt"
	"strings"

	"github.com/datnt1998/noverse/internal/errs"
)

// ── 伏笔状态机 ──
//
// ApplyForeshadow 是伏笔账本的唯一状态机：实时提交（commit_chapter）与章节记录
// 全量重放（revision projector）都调用它。两条路只要喂同一串 update，账本必须
// 逐字节相同——恢复后的书稿不能因为走了重放分支就漂移。

// foreshadowDormantMin 是一条伏笔"睡太久"的最小章数：完成章数的三分之一，
// 但不少于这个下限。阈值只陈述时间事实，不替作者判定该不该收。
const foreshadowDormantMin = 8

// ForeshadowOpen 报告伏笔是否仍未收束。空状态按 planted 处理：那是旧账本
// 没有写 status 的遗留条目，语义上仍是已埋未收。
func ForeshadowOpen(status string) bool {
	return status == "" || status == "planted" || status == "advanced"
}

// ForeshadowDormantThreshold 按已完成章数给出"睡太久"的章数阈值。
func ForeshadowDormantThreshold(completed int) int {
	t := completed / 3
	if t < foreshadowDormantMin {
		return foreshadowDormantMin
	}
	return t
}

// ThreadHealth 判定一条伏笔相对 chapter 的两种拖延状态：
//   - overdue：已过 due_by，却仍是 planted / advanced；
//   - dormant：距最近一次 plant / advance（老账本没有 last_touched 时退回
//     planted_at）已超过 ForeshadowDormantThreshold。
//
// 已收束（resolved / abandoned）的伏笔两者皆为 false。
func ThreadHealth(e ForeshadowEntry, chapter, completed int) (overdue, dormant bool) {
	if !ForeshadowOpen(e.Status) {
		return false, false
	}
	touched := max(e.LastTouched, e.PlantedAt)
	return e.DueBy > 0 && chapter > e.DueBy, chapter-touched > ForeshadowDormantThreshold(completed)
}

// ApplyForeshadow 把一条增量操作应用到账本，返回更新后的账本并就地维护 idx。
// idx 是 id → entries 下标；调用方可以跨多次调用复用它（重放整串记录）。
//
// 规则：
//   - plant 同章重复是幂等（章节记录重放必须容忍），跨章重复 ID 报错；
//   - advance / resolve / abandon 未知 ID 报错；
//   - 已收束的伏笔上任何 advance / resolve / abandon 都是静默 no-op：重放
//     全部记录时它们是历史事实，报错会直接弄断恢复；
//   - abandon 必须带 reason（输入校验；重放数据自身一定有）。
func ApplyForeshadow(entries []ForeshadowEntry, idx map[string]int, chapter int, u ForeshadowUpdate) ([]ForeshadowEntry, error) {
	if strings.TrimSpace(u.ID) == "" {
		return entries, fmt.Errorf("id phục bút không được để trống")
	}
	entryIdx := -1 // 本条 update 落到的账本条目；-1 = 未应用
	switch u.Action {
	case "plant":
		if strings.TrimSpace(u.Description) == "" {
			return entries, fmt.Errorf("plant foreshadow %q requires description", u.ID)
		}
		if u.DueBy != 0 && u.DueBy <= chapter {
			return entries, fmt.Errorf(
				"due_by của phục bút %q phải sau chương gieo %d, nhận %d: %w", u.ID, chapter, u.DueBy, errs.ErrToolArgs)
		}
		if i, ok := idx[u.ID]; ok {
			// 同一章重复 plant 是章节记录全量重放，必须幂等；不同章撞上同一个 id
			// 则是新伏笔被旧条目挡住——此前这里一律 continue，新伏笔连同描述被静默丢弃。
			// 实测一本 22 章的书 plant 了 6 次却只入账 2 条，模型每次都用同一个占位 id
			//（"1" 与 "new"），4 条伏笔无声蒸发，且没有任何一处报错。
			if planted := entries[i].PlantedAt; planted != 0 && planted != chapter {
				return entries, fmt.Errorf(
					"id phục bút %q đã bị chương %d chiếm (%s); mỗi phục bút cần một id riêng, duy nhất và ổn định, "+
						"đừng dùng lại các id giữ chỗ như \"new\"/\"1\" — các chương sau dựa vào id để tìm lại nó mà advance/resolve: %w",
					u.ID, planted, truncateForeshadow(entries[i].Description), errs.ErrToolArgs)
			}
			// 同章重放：只补空白字段，不覆盖已有描述、状态与到期章，账本保持幂等。
			if entries[i].Description == "" {
				entries[i].Description = u.Description
			}
			if entries[i].PlantedAt == 0 {
				entries[i].PlantedAt = chapter
			}
			if entries[i].Status == "" {
				entries[i].Status = "planted"
			}
			if entries[i].DueBy == 0 {
				entries[i].DueBy = u.DueBy
			}
			if entries[i].LastTouched == 0 {
				entries[i].LastTouched = chapter
			}
			entryIdx = i
			break
		}
		entryIdx = len(entries)
		idx[u.ID] = entryIdx
		entries = append(entries, ForeshadowEntry{
			ID:          u.ID,
			Description: u.Description,
			PlantedAt:   chapter,
			Status:      "planted",
			DueBy:       u.DueBy,
			LastTouched: chapter,
		})
	case "advance":
		i, ok := idx[u.ID]
		if !ok {
			return entries, fmt.Errorf("advance unknown foreshadow %q", u.ID)
		}
		if ForeshadowOpen(entries[i].Status) {
			entries[i].Status = "advanced"
			entries[i].LastTouched = chapter
		}
		entryIdx = i
	case "resolve":
		i, ok := idx[u.ID]
		if !ok {
			return entries, fmt.Errorf("resolve unknown foreshadow %q", u.ID)
		}
		if ForeshadowOpen(entries[i].Status) {
			entries[i].Status = "resolved"
			entries[i].ResolvedAt = chapter
		}
		entryIdx = i
	case "abandon":
		i, ok := idx[u.ID]
		if !ok {
			return entries, fmt.Errorf("abandon unknown foreshadow %q", u.ID)
		}
		if ForeshadowOpen(entries[i].Status) {
			if strings.TrimSpace(u.Reason) == "" {
				return entries, fmt.Errorf("abandon foreshadow %q requires reason: %w", u.ID, errs.ErrToolArgs)
			}
			entries[i].Status = "abandoned"
			entries[i].AbandonedAt = chapter
			entries[i].AbandonReason = u.Reason
		}
		entryIdx = i
	default:
		return entries, fmt.Errorf("invalid foreshadow action %q", u.Action)
	}
	// 带 quote 的 update 追加一条证据；严格相等去重让重放幂等。
	if entryIdx >= 0 && strings.TrimSpace(u.Quote) != "" {
		AppendForeshadowEvidence(&entries[entryIdx], EvidenceRef{Chapter: chapter, Action: u.Action, Quote: u.Quote})
	}
	return entries, nil
}

// truncateForeshadow 截短描述，避免把整段伏笔正文塞进错误消息。
func truncateForeshadow(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= 30 {
		return string(r)
	}
	return string(r[:30]) + "…"
}
