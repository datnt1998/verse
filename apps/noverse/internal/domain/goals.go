package domain

import (
	"fmt"
	"strings"
)

// ChapterGoal 是章节验收清单里的一项：由 ChapterContract 里可二值判定的
// 条目冻结而来，ID 按条目在 contract 中的顺序确定（rb/fm/cc + 序号）。
// evaluation_focus / emotion_target / payoff_points / hook_goal 不是二值
// 验收项，不进入清单，仍由 Editor 按维度评分覆盖。
type ChapterGoal struct {
	Chapter int    `json:"-"` // 目标所属章节；context 里按章注入，序列化无需重复
	ID      string `json:"id"`
	Kind    string `json:"kind"` // rb（必须落地）/ fm（禁止发生）/ cc（连续性核对）
	Text    string `json:"text"`
}

// GoalCheck 是 Editor 对单个 ChapterGoal 的判定。kind/text 由代码从 contract
// 快照回填（模型只需提交 id/status/quote/note），保证存档里的检查项与原
// 目标文本一一对应。
type GoalCheck struct {
	ID     string `json:"id"`
	Kind   string `json:"kind,omitempty"`
	Text   string `json:"text,omitempty"`
	Status string `json:"status"`          // pass / fail / unknown；unknown 一律不算达成
	Quote  string `json:"quote,omitempty"` // 支撑判定的原文引文（是否需要随 kind+status 而定）
	Note   string `json:"note,omitempty"`  // fail/unknown 时说明缺什么、为何无法判定
}

// GoalsFromContract 把 contract 的二值条目展开成带稳定 ID 的验收清单。
// 空白条目（TrimSpace 后为空）跳过，序号按保留的条目连续编号。
// chapter 仅用于回填 ChapterGoal.Chapter。
func GoalsFromContract(chapter int, c ChapterContract) []ChapterGoal {
	var goals []ChapterGoal
	appendGoals := func(kind string, items []string) {
		n := 0
		for _, item := range items {
			text := strings.TrimSpace(item)
			if text == "" {
				continue
			}
			n++
			goals = append(goals, ChapterGoal{
				Chapter: chapter,
				ID:      fmt.Sprintf("%s%d", kind, n),
				Kind:    kind,
				Text:    text,
			})
		}
	}
	appendGoals("rb", c.RequiredBeats)
	appendGoals("fm", c.ForbiddenMoves)
	appendGoals("cc", c.ContinuityChecks)
	return goals
}

// DeriveContractStatus 由逐项检查结论汇总出 contract_status 与
// contract_misses，规则固定、不给模型自由裁定空间：
//   - 全部 pass → met；
//   - rb/fm 中 fail 数 ≥ rb+fm 总数的一半 → missed；
//   - 其余有 fail 或 unknown → partial。
//
// unknown 永不计入达成；fail/unknown 的条目按 "[id|status] text" 记入 misses。
// checks 为空时没有可汇总的对象，返回空状态与 nil。
func DeriveContractStatus(goals []ChapterGoal, checks []GoalCheck) (string, []string) {
	if len(goals) == 0 || len(checks) == 0 {
		return "", nil
	}
	byID := make(map[string]GoalCheck, len(checks))
	for _, check := range checks {
		byID[check.ID] = check
	}
	var binaryTotal, binaryFailed int
	var misses []string
	nonPass := false
	for _, goal := range goals {
		check, ok := byID[goal.ID]
		status := "unknown"
		if ok && check.Status != "" {
			status = check.Status
		}
		if goal.Kind == "rb" || goal.Kind == "fm" {
			binaryTotal++
			if status == "fail" {
				binaryFailed++
			}
		}
		if status == "pass" {
			continue
		}
		nonPass = true
		misses = append(misses, fmt.Sprintf("[%s|%s] %s", goal.ID, status, goal.Text))
	}
	switch {
	case !nonPass:
		return "met", nil
	case binaryTotal > 0 && binaryFailed*2 >= binaryTotal:
		return "missed", misses
	default:
		return "partial", misses
	}
}
