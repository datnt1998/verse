package host

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/domain"
)

// ─── “Kế hoạch truyện”面板（Gói P · 21-dev-prompt.md P1）────────────────────
//
// 五步固定序：Thông tin truyện · Tiền đề · Nhân vật · Quy tắc thế giới · Dàn ý。
// 状态不是新增事实源：每一步的 done/pending 由 FoundationMissing 同一事实推导，
// 时长由 checkpoint 的 OccurredAt 间隔推导（与 Nhật ký 里 TOOL 行的时长同源，
// 都是那次落盘相对上一条规划事实的墙钟差）。

// planStepOrder 是面板的固定行序（spec 文案序，与 FoundationMissing 的内部序无关）。
var planStepOrder = []string{"book", "premise", "characters", "worldRules", "outline"}

// planStepArtifacts 把面板步映回 FoundationMissing 的缺项键。
var planStepArtifacts = map[string][]string{
	"book":       {"book"},
	"premise":    {"premise"},
	"characters": {"characters"},
	"worldRules": {"world_rules"},
	"outline":    {"outline", "layered_outline", "compass"},
}

// planStepCheckpoints 是每步对应的 checkpoint step 名（同一工件可能多种写法）。
var planStepCheckpoints = map[string][]string{
	"book":       {"book"},
	"premise":    {"premise"},
	"characters": {"characters"},
	"worldRules": {"world_rules"},
	"outline":    {"outline", "layered_outline", "expand_arc", "append_volume", "compass", "update_compass"},
}

// planToolSteps 把正在跑的保存工具映回面板步（含 save_foundation[type] 变体）。
var planToolSteps = map[string]string{
	"save_book":                       "book",
	"save_foundation:premise":         "premise",
	"save_foundation:characters":      "characters",
	"save_foundation:world_rules":     "worldRules",
	"save_foundation:outline":         "outline",
	"save_foundation:layered_outline": "outline",
	"save_foundation:expand_arc":      "outline",
	"save_foundation:append_volume":   "outline",
	"save_foundation:compass":         "outline",
	"save_foundation:update_compass":  "outline",
	"revise_outline":                  "outline",
}

// saveFoundationVariant 从 agent.summary（"agent → save_foundation[type]"）里抠 type。
var saveFoundationVariant = regexp.MustCompile(`save_foundation\[([a-z_]+)\]`)

// fillPlan 组装规划期面板数据；非规划期返回 nil（progress 已进 writing/complete，
// 或没有任何基础设定缺项）。读取失败只降级为空面板并告警，不打断快照。
func (h *Host) fillPlan(snap *UISnapshot, progress *domain.Progress) {
	if progress == nil || progress.Phase == domain.PhaseWriting || progress.Phase == domain.PhaseComplete {
		return
	}
	missing, err := h.store.FoundationMissing()
	if err != nil {
		slog.Warn("Đọc trạng thái thiết lập nền thất bại, bỏ khung kế hoạch", "module", "host.snapshot", "err", err)
		return
	}
	if len(missing) == 0 {
		return
	}
	// 共创/导入待确认占用同一框架，规划面板不出现。
	if snap.WaitingFor == "cocreate" || snap.WaitingFor == "confirm" {
		return
	}

	pending := map[string]bool{}
	for _, artifact := range missing {
		for _, key := range planStepOrder {
			for _, a := range planStepArtifacts[key] {
				if a == artifact {
					pending[key] = true
				}
			}
		}
	}

	running := runningPlanStep(snap, pending)
	planStart := h.planStartTime()
	checkpoints := h.store.Checkpoints.All()

	plan := &PlanSnapshot{Total: len(planStepOrder)}
	var prevEnd time.Time // 上一条规划 checkpoint 的落盘时刻
	latestPlanCp := latestPlanCheckpoint(checkpoints)
	for _, key := range planStepOrder {
		step := PlanStep{Key: key}
		switch {
		case key == running && snap.IsRunning:
			step.Status = "running"
			step.StartedAt = latestPlanCp.OccurredAt
			if step.StartedAt.IsZero() {
				step.StartedAt = planStart
			}
		case key == running:
			step.Status = "paused"
		case pending[key]:
			step.Status = "pending"
		default:
			step.Status = "done"
			plan.Done++
		}
		if end, ok := latestStepCheckpoint(checkpoints, planStepCheckpoints[key]); ok {
			anchor := prevEnd
			if anchor.IsZero() {
				anchor = planStart
			}
			if !anchor.IsZero() {
				step.DurationMs = end.Sub(anchor).Milliseconds()
				if step.DurationMs < 0 {
					step.DurationMs = 0
				}
			}
			if end.After(prevEnd) {
				prevEnd = end
			}
		}
		step.Summary, step.Detail = h.planStepContent(key)
		plan.Steps = append(plan.Steps, step)
	}
	plan.RunningStep = running
	snap.Plan = plan
}

// runningPlanStep 推定面板里“正在执行”的步骤：
// agent 正跑保存工具且该步还缺 → 那一步；否则第一个仍缺的步。
func runningPlanStep(snap *UISnapshot, pending map[string]bool) string {
	if snap.IsRunning {
		for _, agent := range snap.Agents {
			if !strings.HasPrefix(strings.ToLower(agent.Name), "architect") {
				continue
			}
			st := strings.ToLower(agent.State)
			if st != "running" && st != "working" {
				continue
			}
			key, known := "", false
			if agent.Tool == "save_foundation" {
				if m := saveFoundationVariant.FindStringSubmatch(agent.Summary); len(m) == 2 {
					key, known = planToolSteps["save_foundation:"+m[1]], true
				}
				// 变体未知：不臆造，退回第一个缺项。
			} else {
				key, known = planToolSteps[agent.Tool]
			}
			if known && key != "" && pending[key] {
				return key
			}
		}
	}
	for _, key := range planStepOrder {
		if pending[key] {
			return key
		}
	}
	return ""
}

// planStartTime 取启动裁定时刻作为第一步时长的锚点；没有裁定记录返回零值。
func (h *Host) planStartTime() time.Time {
	records, err := h.store.Decisions.Recent(0)
	if err != nil {
		slog.Warn("Đọc nhật ký phán định thất bại", "module", "host.snapshot", "err", err)
		return time.Time{}
	}
	for _, rec := range records {
		if rec.Kind != "plan_start" || rec.At == "" {
			continue
		}
		if at, err := time.Parse(time.RFC3339, rec.At); err == nil {
			return at // 最早一条启动裁定（重试不改变规划起点）
		}
	}
	return time.Time{}
}

// latestStepCheckpoint 返回某步最近一次落盘的时刻。
func latestStepCheckpoint(all []domain.Checkpoint, steps []string) (time.Time, bool) {
	want := map[string]bool{}
	for _, s := range steps {
		want[s] = true
	}
	var latest time.Time
	found := false
	for i := range all {
		if !want[all[i].Step] {
			continue
		}
		if all[i].OccurredAt.After(latest) {
			latest = all[i].OccurredAt
			found = true
		}
	}
	return latest, found
}

// latestPlanCheckpoint 返回最近一条规划期 checkpoint（给运行中步骤的 startedAt）。
func latestPlanCheckpoint(all []domain.Checkpoint) domain.Checkpoint {
	var latest domain.Checkpoint
	for i := range all {
		if _, ok := planStepCheckpointsByName(all[i].Step); !ok {
			continue
		}
		if all[i].OccurredAt.After(latest.OccurredAt) {
			latest = all[i]
		}
	}
	return latest
}

func planStepCheckpointsByName(step string) (string, bool) {
	for key, steps := range planStepCheckpoints {
		for _, s := range steps {
			if s == step {
				return key, true
			}
		}
	}
	return "", false
}

// planStepContent 产出某步的单行摘要与展开 Detail；工件缺失返回空。
func (h *Host) planStepContent(key string) (string, json.RawMessage) {
	switch key {
	case "book":
		book, err := h.store.Book.Load()
		if err != nil || book == nil {
			return "", nil
		}
		summary := book.Title
		if book.Synopsis != "" {
			summary = truncate(book.Synopsis, 90)
		}
		return summary, mustJSON(PlanBookDetail{Kind: "book", Title: book.Title, Synopsis: book.Synopsis})
	case "premise":
		premise, err := h.store.Outline.LoadPremise()
		if err != nil || premise == "" {
			return "", nil
		}
		return truncate(premise, 90), mustJSON(PlanPremiseDetail{Kind: "premise", Text: premise})
	case "characters":
		chars, err := h.store.Characters.Load()
		if err != nil || len(chars) == 0 {
			return "", nil
		}
		list := make([]PlanCharacter, 0, len(chars))
		for _, c := range chars {
			list = append(list, PlanCharacter{Name: c.Name, Role: c.Role, Kind: string(c.Kind)})
		}
		return fmt.Sprintf("%d nhân vật", len(list)), mustJSON(PlanCharactersDetail{Kind: "characters", Characters: list})
	case "worldRules":
		rules, err := h.store.World.LoadWorldRules()
		if err != nil || len(rules) == 0 {
			return "", nil
		}
		list := make([]PlanRule, 0, len(rules))
		for _, r := range rules {
			list = append(list, PlanRule{Rule: r.Rule, Boundary: r.Boundary})
		}
		return fmt.Sprintf("%d quy tắc", len(list)), mustJSON(PlanRulesDetail{Kind: "worldRules", Rules: list})
	case "outline":
		return h.planOutlineContent()
	}
	return "", nil
}

// planOutlineContent 组装 “Dàn ý” 步：Saved 为已细化章数，Total 为
// 含骨架估算的总规模（扁平大纲条目数 + 未展开弧的 EstimatedChapters）。
func (h *Host) planOutlineContent() (string, json.RawMessage) {
	flat, err := h.store.Outline.LoadOutline()
	if err != nil {
		return "", nil
	}
	volumes, err := h.store.Outline.LoadLayeredOutline()
	if err != nil {
		return "", nil
	}
	if len(flat) == 0 && len(volumes) == 0 {
		return "", nil
	}
	total := len(flat)
	if len(volumes) > 0 {
		total = domain.EstimatedChapterCapacity(volumes)
	}
	chapters := make([]PlanChapter, 0, len(flat))
	for _, e := range flat {
		chapters = append(chapters, PlanChapter{Chapter: e.Chapter, Title: e.Title, Hook: e.Hook})
	}
	summary := fmt.Sprintf("đã lưu %d/%d chương", len(chapters), total)
	return summary, mustJSON(PlanOutlineDetail{Kind: "outline", Saved: len(chapters), Total: total, Chapters: chapters})
}

// mustJSON 序列化 Detail；出错返回 nil（面板缺展开区优于整快照失败）。
func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
