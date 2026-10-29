package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/rules"
	"github.com/datnt1998/noverse/internal/store"
)

func newTestContextTool(st *store.Store, refs References, style string) *ContextTool {
	return NewContextTool(st, refs, style, NewStyleStatsIndex(st))
}

// 规划阶段必须能用显式 null 读取基础状态，不能被引向尚不存在的第一章。
func TestContextNullChapterReturnsUnfinishedFoundation(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	tool := newTestContextTool(st, References{}, "default")
	for _, args := range []string{`{}`, `{"chapter":null}`} {
		raw, err := tool.Execute(context.Background(), json.RawMessage(args))
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Foundation struct {
				Status struct {
					Missing []string `json:"missing"`
				} `json:"foundation_status"`
			} `json:"foundation_memory"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		for _, missing := range []string{"book", "premise", "outline", "characters", "world_rules"} {
			if !slices.Contains(payload.Foundation.Status.Missing, missing) {
				t.Fatalf("args %s: missing foundation action %s: %s", args, missing, raw)
			}
		}
	}
}

func TestBuildProgressStatusHidesLayeredCapacityEstimate(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Save(&domain.Progress{TotalChapters: 66, Layered: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.Outline.SaveLayeredOutline([]domain.VolumeOutline{{
		Index: 1, Arcs: []domain.ArcOutline{
			{Index: 1, Chapters: []domain.OutlineEntry{{Title: "一"}, {Title: "二"}}},
			{Index: 2, EstimatedChapters: 64},
		},
	}}); err != nil {
		t.Fatal(err)
	}

	result := map[string]any{}
	newTestContextTool(st, References{}, "default").buildProgressStatus(result, &contextReads{})
	status, ok := result["progress_status"].(map[string]any)
	if !ok {
		t.Fatalf("progress_status = %#v", result["progress_status"])
	}
	if status["dynamic_planning"] != true || status["outlined_chapters"] != 2 {
		t.Fatalf("动态规划进度错误: %#v", status)
	}
	if _, exists := status["total_chapters"]; exists {
		t.Fatalf("分层容量估算不得作为 total_chapters 暴露: %#v", status)
	}
}

func TestContextToolInjectsStyleStats(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	progress := &domain.Progress{TotalChapters: 10}
	body := "# Chương 1\nAnh không phải do dự, mà là sợ hãi. Im lặng vài giây. Như thể có ánh sáng.\nĐêm buông xuống.\nAnh bước đi."
	for ch := 1; ch <= 6; ch++ {
		if err := st.Drafts.SaveFinalChapter(ch, body); err != nil {
			t.Fatalf("SaveFinalChapter: %v", err)
		}
		progress.CompletedChapters = append(progress.CompletedChapters, ch)
	}
	if err := st.Progress.Save(progress); err != nil {
		t.Fatalf("Save progress: %v", err)
	}

	tool := newTestContextTool(st, References{}, "default")
	args, _ := json.Marshal(map[string]any{"chapter": 7})
	raw, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload struct {
		Episodic map[string]json.RawMessage `json:"episodic_memory"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	statsRaw, ok := payload.Episodic["style_stats"]
	if !ok {
		t.Fatalf("expected episodic_memory.style_stats, got keys %v", keysOf(payload.Episodic))
	}
	var stats struct {
		Chapters int `json:"chapters"`
		Patterns []struct {
			Name  string `json:"name"`
			Total int    `json:"total"`
		} `json:"patterns"`
	}
	if err := json.Unmarshal(statsRaw, &stats); err != nil {
		t.Fatalf("Unmarshal stats: %v", err)
	}
	if stats.Chapters != 6 || len(stats.Patterns) == 0 {
		t.Errorf("stats content: %+v", stats)
	}
	if usage, ok := payload.Episodic["_usage"]; !ok || len(usage) == 0 {
		t.Error("expected episodic_memory._usage annotation")
	}
}

func TestContextToolWarnsWhenOptionalDataIsCorrupt(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta", "simulation_profile.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := newTestContextTool(st, References{}, "default").Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	warnings, _ := got["_warnings"].([]any)
	if len(warnings) == 0 || !strings.Contains(warnings[0].(string), "simulation_profile") {
		t.Fatalf("可选资料损坏必须显式告警: %+v", got["_warnings"])
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestContextToolRejectsCorruptCoreState(t *testing.T) {
	dir := t.TempDir()
	store := store.NewStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "meta", "progress.json"), []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("write progress.json: %v", err)
	}

	tool := newTestContextTool(store, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 2})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	_, err = tool.Execute(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "progress") {
		t.Fatalf("核心事实损坏必须终止上下文装配: %v", err)
	}
}

func TestContextToolChapterModeIncludesWorkingAndReferenceFields(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Outline.SavePremise(`## 题材和基调
少年成长，偏紧张压迫。

## 题材定位
少年升级流

## 核心冲突
主角必须在宗门竞争中活下来。

## 主角目标
进入内门。

## 终局方向
成为真正的执棋者。

## 写作禁区
不提前揭露师尊真相。

## 差异化卖点
弱者逆袭。

## 差异化钩子
每阶段都要用更高代价换成长。

## 核心兑现承诺
持续兑现危机与突破。

## 故事引擎
试炼、资源争夺与身份升级共同推进。

## 中段转折
主角被迫转向另一条修行路线。
`); err != nil {
		t.Fatalf("SavePremise: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "入门", CoreEvent: "主角进入宗门", Scenes: []string{"拜师", "立誓"}},
		{Chapter: 2, Title: "试炼", CoreEvent: "参加外门试炼", Scenes: []string{"集合", "出发"}},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Characters.Save([]domain.Character{
		{Name: "林砚", Role: "主角", Description: "少年修士", Arc: "成长", Traits: []string{"冷静"}},
	}); err != nil {
		t.Fatalf("SaveCharacters: %v", err)
	}
	if err := s.World.SaveWorldRules([]domain.WorldRule{
		{Category: "magic", Rule: "灵气可以炼化", Boundary: "凡人不可直接驾驭"},
	}); err != nil {
		t.Fatalf("SaveWorldRules: %v", err)
	}
	if err := s.Progress.Init(2); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.Summaries.SaveSummary(domain.ChapterSummary{
		Chapter:    1,
		Summary:    "主角拜入宗门，确立目标。",
		Characters: []string{"林砚"},
		KeyEvents:  []string{"拜师"},
	}); err != nil {
		t.Fatalf("SaveSummary: %v", err)
	}
	if err := s.Drafts.SaveFinalChapter(1, "第一章正文结尾，留下试炼悬念。"); err != nil {
		t.Fatalf("SaveFinalChapter: %v", err)
	}
	if err := s.Drafts.SaveChapterPlan(domain.ChapterPlan{
		Chapter: 2,
		Title:   "试炼",
		Goal:    "通过第一关",
		Contract: domain.ChapterContract{
			RequiredBeats:    []string{"必须让主角通过第一关", "必须埋下内门试炼邀请"},
			ForbiddenMoves:   []string{"不能提前揭露师尊真实身份"},
			ContinuityChecks: []string{"主角左臂旧伤仍未痊愈"},
			EvaluationFocus:  []string{"重点检查试炼节奏是否拖沓"},
		},
	}); err != nil {
		t.Fatalf("SaveChapterPlan: %v", err)
	}
	if err := s.World.SaveStyleRules(domain.WritingStyleRules{
		Volume: 1,
		Arc:    1,
		Prose:  []string{"叙述保持克制"},
	}); err != nil {
		t.Fatalf("SaveStyleRules: %v", err)
	}
	if err := s.RunMeta.SetPlanningTier(domain.PlanningTierLong); err != nil {
		t.Fatalf("SetPlanningTier: %v", err)
	}

	tool := newTestContextTool(s, References{
		Consistency:      "一致性检查",
		HookTechniques:   "钩子技巧",
		QualityChecklist: "质量清单",
	}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 2})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, key := range []string{
		"premise",
		"premise_sections",
		"premise_structure",
		"world_rules",
		"memory_policy",
		"working_memory",
		"episodic_memory",
		"reference_pack",
	} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected key %q in chapter context", key)
		}
	}
	if _, ok := payload["outline"]; ok {
		t.Fatal("chapter context must not include the whole outline")
	}
	working := payload["working_memory"].(map[string]any)
	for _, key := range []string{"current_chapter_outline", "recent_summaries", "chapter_plan", "chapter_contract", "chapter_goals", "previous_tail"} {
		if _, ok := working[key]; !ok {
			t.Fatalf("expected working_memory.%s", key)
		}
	}
	episodic := payload["episodic_memory"].(map[string]any)
	if _, ok := episodic["planning_tier"]; !ok {
		t.Fatal("expected episodic_memory.planning_tier")
	}
	referencePack := payload["reference_pack"].(map[string]any)
	for _, key := range []string{"style_rules", "references"} {
		if _, ok := referencePack[key]; !ok {
			t.Fatalf("expected reference_pack.%s", key)
		}
	}
	for _, key := range []string{"planning_tier", "current_chapter_outline", "recent_summaries", "chapter_plan", "chapter_contract", "chapter_goals", "previous_tail", "style_rules", "references"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("unexpected top-level memory field %q", key)
		}
	}
}

// 带 quote 的事实要回取原文段落注入 episodic_memory.evidence_passages，
// 让 writer/editor 直接对照正字而不是只靠结构化摘要。
func TestContextToolEmitsEvidencePassages(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Outline.SavePremise("## 题材和基调\n测试。"); err != nil {
		t.Fatalf("SavePremise: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "入门", CoreEvent: "Lâm Uyển nhập môn"},
		{Chapter: 2, Title: "Thí luyện", CoreEvent: "Lâm Uyển tham gia thử thách"},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Characters.Save([]domain.Character{{Name: "Lâm Uyển", Role: "main"}}); err != nil {
		t.Fatalf("SaveCharacters: %v", err)
	}
	if err := s.World.SaveWorldRules([]domain.WorldRule{{Category: "magic", Rule: "灵气可以炼化"}}); err != nil {
		t.Fatalf("SaveWorldRules: %v", err)
	}
	if err := s.Progress.Init(2); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.Drafts.SaveFinalChapter(1, "Mở đầu chương.\n\nLâm Uyển nhìn thấy bức thư cũ trên bàn.\n\nKết chương."); err != nil {
		t.Fatalf("SaveFinalChapter: %v", err)
	}
	if err := s.World.AppendStateChanges([]domain.StateChange{
		{Chapter: 1, Entity: "Lâm Uyển", Field: "location", NewValue: "sảnh",
			Quote: "nhìn thấy bức thư cũ trên bàn"},
	}); err != nil {
		t.Fatalf("AppendStateChanges: %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	raw, err := tool.Execute(context.Background(), json.RawMessage(`{"chapter":2}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	episodic, _ := payload["episodic_memory"].(map[string]any)
	ep, ok := episodic["evidence_passages"].(map[string]any)
	if !ok {
		t.Fatalf("evidence_passages missing: keys=%v", keysOfMap(episodic))
	}
	usage, _ := ep["_usage"].(string)
	if !strings.Contains(usage, "dữ liệu truyện") {
		t.Fatalf("evidence_passages must label passages as story data, not instructions: %q", usage)
	}
	items, _ := ep["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 evidence item, got %v", items)
	}
	item := items[0].(map[string]any)
	if item["chapter"] != float64(1) || item["kind"] != "state_change" {
		t.Fatalf("wrong item: %+v", item)
	}
	passage, _ := item["passage"].(string)
	if !strings.Contains(passage, "bức thư cũ trên bàn") || !strings.Contains(passage, "Mở đầu chương") {
		t.Fatalf("passage should carry the quote paragraph and neighbours: %q", passage)
	}
}

// 超预算时 evidence_passages 先于 previous_tail 被裁：尾部可由 read_chapter
// 补读，证据段落次之。
func TestContextToolTrimsEvidenceBeforePreviousTail(t *testing.T) {
	result := map[string]any{
		"episodic_memory": map[string]any{
			"evidence_passages": map[string]any{"items": []any{strings.Repeat("x", 300)}},
		},
		"working_memory": map[string]any{
			"previous_tail": strings.Repeat("y", 300),
		},
		"style_stats": map[string]any{"fixed": true},
	}
	budget := len(mustMarshal(t, result)) - 200 // 裁一项即可达标
	if err := trimByBudget(result, budget); err != nil {
		t.Fatalf("trimByBudget: %v", err)
	}
	trimmed, _ := result["_trimmed"].([]string)
	if !slices.Equal(trimmed, []string{"evidence_passages"}) {
		t.Fatalf("evidence_passages should trim before previous_tail: %v", trimmed)
	}
	if _, ok := result["working_memory"].(map[string]any)["previous_tail"]; !ok {
		t.Fatal("previous_tail must survive while evidence_passages covers the budget")
	}
}

func keysOfMap(m map[string]any) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestContextToolArchitectModeIncludesPlanningAndFoundation(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(6); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.Progress.SetLayered(true); err != nil {
		t.Fatalf("SetLayered: %v", err)
	}
	if err := s.Progress.UpdateVolumeArc(1, 1); err != nil {
		t.Fatalf("UpdateVolumeArc: %v", err)
	}
	if err := s.Outline.SavePremise(`## 题材和基调
群像冒险，偏冷峻史诗。

## 题材定位
群像长篇冒险

## 核心冲突
众人必须在不断失控的旧秩序中寻找新秩序。

## 主角目标
抵达真相核心。

## 终局方向
揭开古老真相并重建秩序。

## 写作禁区
不靠天降设定收尾。

## 差异化卖点
群像关系推进。

## 差异化钩子
每卷都改变队伍关系结构。

## 核心兑现承诺
持续提供发现、牺牲与选择。

## 故事引擎
旅途推进、真相调查与队伍关系共同驱动。

## 关系/成长主线
队伍从互不信任走向分裂再重组。

## 升级路径
从地方事件走向世界级危机。

## 中期转向
真相并非敌人，而是秩序本身有问题。

## 终局命题
秩序应由谁定义。
`); err != nil {
		t.Fatalf("SavePremise: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "起点", CoreEvent: "旅途开始"},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Characters.Save([]domain.Character{
		{Name: "沈曜", Role: "主角", Description: "流浪剑客", Arc: "寻找真相", Traits: []string{"敏锐"}},
	}); err != nil {
		t.Fatalf("SaveCharacters: %v", err)
	}
	if err := s.World.SaveWorldRules([]domain.WorldRule{
		{Category: "society", Rule: "城邦林立", Boundary: "皇权不可直辖边地"},
	}); err != nil {
		t.Fatalf("SaveWorldRules: %v", err)
	}
	if err := s.Outline.SaveLayeredOutline([]domain.VolumeOutline{
		{
			Index: 1, Title: "第一卷", Theme: "踏上旅途",
			Arcs: []domain.ArcOutline{
				{Index: 1, Title: "启程", Goal: "建立队伍", Chapters: []domain.OutlineEntry{{Chapter: 1, Title: "起点"}}},
				{Index: 2, Title: "迷雾", Goal: "逼近秘密", EstimatedChapters: 5},
			},
		},
	}); err != nil {
		t.Fatalf("SaveLayeredOutline: %v", err)
	}
	if err := s.Summaries.SaveArcSummary(domain.ArcSummary{
		Volume: 1, Arc: 1, Title: "启程", Summary: "队伍建立，但因真相分歧出现裂痕。", KeyEvents: []string{"队伍建立", "分歧浮现"},
	}); err != nil {
		t.Fatalf("SaveArcSummary: %v", err)
	}
	if err := s.Outline.SaveCompass(domain.StoryCompass{
		EndingDirection: "揭开古老真相",
		EstimatedScale:  "预计 3 卷",
	}); err != nil {
		t.Fatalf("SaveCompass: %v", err)
	}
	if err := s.World.SaveStyleRules(domain.WritingStyleRules{
		Volume: 1,
		Arc:    1,
		Prose:  []string{"保持冷峻节制"},
	}); err != nil {
		t.Fatalf("SaveStyleRules: %v", err)
	}
	if err := s.RunMeta.SetPlanningTier(domain.PlanningTierLong); err != nil {
		t.Fatalf("SetPlanningTier: %v", err)
	}

	tool := newTestContextTool(s, References{
		OutlineTemplate:   "大纲模板",
		CharacterTemplate: "角色模板",
		LongformPlanning:  "长篇规划",
	}, "default")
	args, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, key := range []string{"memory_policy", "planning_memory", "foundation_memory", "reference_pack"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected key %q in architect context", key)
		}
	}
	planning := payload["planning_memory"].(map[string]any)
	for _, key := range []string{"planning_tier", "layered_outline", "skeleton_arcs", "arc_summaries", "compass"} {
		if _, ok := planning[key]; !ok {
			t.Fatalf("expected planning_memory.%s", key)
		}
	}
	foundation := payload["foundation_memory"].(map[string]any)
	for _, key := range []string{"premise", "premise_sections", "premise_structure", "characters", "foundation_status"} {
		if _, ok := foundation[key]; !ok {
			t.Fatalf("expected foundation_memory.%s", key)
		}
	}
	referencePack := payload["reference_pack"].(map[string]any)
	for _, key := range []string{"style_rules", "references"} {
		if _, ok := referencePack[key]; !ok {
			t.Fatalf("expected reference_pack.%s", key)
		}
	}
	for _, key := range []string{"planning_tier", "layered_outline", "skeleton_arcs", "arc_summaries", "compass", "premise", "premise_sections", "premise_structure", "characters", "foundation_status", "style_rules", "references"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("unexpected top-level memory field %q", key)
		}
	}
}

func TestContextToolArchitectModeIncludesFlatOutline(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "开端"}}); err != nil {
		t.Fatal(err)
	}

	raw, err := newTestContextTool(s, References{}, "default").Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	planning := payload["planning_memory"].(map[string]any)
	if _, ok := planning["outline"]; !ok {
		t.Fatal("expected planning_memory.outline")
	}
	if _, ok := payload["outline"]; ok {
		t.Fatal("unexpected top-level outline")
	}
}

func TestTrimByBudgetRemovesCanonicalMemoryKeys(t *testing.T) {
	result := map[string]any{
		"reference_pack": map[string]any{
			"references": map[string]string{
				"a": strings.Repeat("x", 200),
				"b": strings.Repeat("y", 200),
			},
			"style_rules": []string{"克制"},
		},
	}

	if err := trimByBudget(result, 80); err != nil {
		t.Fatal(err)
	}

	pack, ok := result["reference_pack"].(map[string]any)
	if !ok {
		t.Fatal("expected reference_pack to remain available")
	}
	if _, ok := pack["references"]; ok {
		t.Fatal("expected references to be trimmed from reference_pack")
	}
}

func TestTrimByBudgetKeepsStyleStats(t *testing.T) {
	styleStats := map[string]any{
		"chapters": 200,
		"patterns": []map[string]any{
			{"name": "Câu đối lập“không phải… mà là”", "total": 80, "per_chapter": 0.4},
		},
	}
	result := map[string]any{
		"reference_pack": map[string]any{
			"references": strings.Repeat("x", 500),
		},
		"episodic_memory": map[string]any{
			"style_stats": styleStats,
		},
	}

	if err := trimByBudget(result, 200); err != nil {
		t.Fatal(err)
	}

	episodic := result["episodic_memory"].(map[string]any)
	if _, ok := episodic["style_stats"]; !ok {
		t.Fatal("style_stats must remain in episodic_memory")
	}
	if trimmed, ok := result["_trimmed"].([]string); ok && slices.Contains(trimmed, "style_stats") {
		t.Fatal("style_stats must not be reported as trimmed")
	}
}

func TestTrimByBudgetRejectsUntrimmablePayload(t *testing.T) {
	result := map[string]any{"required": strings.Repeat("x", 500)}
	if err := trimByBudget(result, 100); err == nil || !strings.Contains(err.Error(), "exceeds budget") {
		t.Fatalf("expected explicit budget error, got %v", err)
	}
}

func TestFinalizeContextPayloadReportsAppliedTrimming(t *testing.T) {
	result := map[string]any{
		"reference_pack": map[string]any{
			"references": map[string]string{"guide": strings.Repeat("x", 1000)},
		},
		"episodic_memory": map[string]any{
			"style_stats": map[string]any{"chapters": 20},
		},
	}
	raw, err := finalizeContextPayload(result, 3, 400)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 400 {
		t.Fatalf("payload = %d bytes, budget = 400", len(raw))
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	summary, _ := payload["_loading_summary"].(string)
	if !strings.Contains(summary, "cắt bớt:references") {
		t.Fatalf("loading summary must reflect final trimming, got %q", summary)
	}
}

func TestProjectLayeredOutlineCompactsOnlyCompletedArcs(t *testing.T) {
	volumes := []domain.VolumeOutline{{
		Index: 1,
		Arcs: []domain.ArcOutline{
			{Index: 1, Chapters: []domain.OutlineEntry{{Title: "一"}, {Title: "二"}}},
			{Index: 2, Chapters: []domain.OutlineEntry{{Title: "三"}, {Title: "四"}}},
		},
	}}

	projected := projectLayeredOutlineForPlanning(volumes, 2)
	if got := projected[0].Arcs[0]; got.Status != "completed" || len(got.Chapters) != 0 || got.StartChapter != 1 || got.EndChapter != 2 {
		t.Fatalf("completed arc projection = %+v", got)
	}
	if got := projected[0].Arcs[1]; got.Status != "expanded" || len(got.Chapters) != 2 || got.StartChapter != 3 || got.EndChapter != 4 {
		t.Fatalf("future arc projection = %+v", got)
	}
}

func TestContextToolLongLayeredPlanningStaysWithinBudget(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	volumes := make([]domain.VolumeOutline, 10)
	completed := make([]int, 0, 500)
	chapter := 0
	for vi := range volumes {
		volumes[vi] = domain.VolumeOutline{Index: vi + 1, Title: fmt.Sprintf("卷%d", vi+1), Theme: strings.Repeat("主题", 10)}
		for ai := 0; ai < 10; ai++ {
			arc := domain.ArcOutline{Index: ai + 1, Title: fmt.Sprintf("弧%d", ai+1), Goal: strings.Repeat("目标", 20)}
			for ci := 0; ci < 5; ci++ {
				chapter++
				completed = append(completed, chapter)
				arc.Chapters = append(arc.Chapters, domain.OutlineEntry{
					Title: fmt.Sprintf("第%d章", chapter), CoreEvent: strings.Repeat("关键事件", 30),
					Hook: strings.Repeat("悬念", 20), Scenes: []string{strings.Repeat("场景", 20)},
				})
			}
			volumes[vi].Arcs = append(volumes[vi].Arcs, arc)
		}
	}
	if err := s.Outline.SaveLayeredOutline(volumes); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Save(&domain.Progress{
		Phase: domain.PhaseWriting, Layered: true, CompletedChapters: completed,
		CurrentVolume: 10, CurrentArc: 10,
	}); err != nil {
		t.Fatal(err)
	}

	raw, err := newTestContextTool(s, References{}, "default").Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 60*1024 {
		t.Fatalf("architect payload = %d bytes, budget = %d", len(raw), 60*1024)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	planning := payload["planning_memory"].(map[string]any)
	encoded, err := json.Marshal(planning["layered_outline"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "关键事件") {
		t.Fatal("completed chapter details must not remain in architect planning projection")
	}
}

func TestContextToolWriterDoesNotIncludeWholeOutline(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	outline := make([]domain.OutlineEntry, 200)
	for i := range outline {
		outline[i] = domain.OutlineEntry{
			Chapter: i + 1, Title: fmt.Sprintf("第%d章", i+1),
			CoreEvent: strings.Repeat("事件", 20), Hook: strings.Repeat("悬念", 10),
		}
	}
	if err := s.Outline.SaveOutline(outline); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, TotalChapters: 200}); err != nil {
		t.Fatal(err)
	}

	raw, err := newTestContextTool(s, References{}, "default").Execute(context.Background(), json.RawMessage(`{"chapter":100}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["outline"]; ok {
		t.Fatal("writer payload must not include the whole outline")
	}
	working := payload["working_memory"].(map[string]any)
	if _, ok := working["current_chapter_outline"]; !ok {
		t.Fatal("writer payload must retain the current chapter outline")
	}
	window, ok := working["outline_window"].([]any)
	if !ok || len(window) != domain.ReviewInterval {
		t.Fatalf("writer outline window = %T/%d, want %d entries", working["outline_window"], len(window), domain.ReviewInterval)
	}
}

func TestContextToolSelectedMemoryRecallsStoryThreadsAndReviewLessons(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "邀约", CoreEvent: "长老暗中给出内门试炼邀请", Scenes: []string{"密谈", "留下试炼令"}},
		{Chapter: 2, Title: "试炼前夜", CoreEvent: "林砚准备回应内门试炼邀请", Hook: "谁在背后推动这场试炼", Scenes: []string{"整理线索", "决定赴约"}},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Progress.Init(8); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "trial_invite", Description: "内门试炼邀请的真实目的", PlantedAt: 1, Status: "planted"},
		{ID: "trial_mastermind", Description: "谁在背后推动这场试炼", PlantedAt: 1, Status: "planted"},
		{ID: "trial_rules", Description: "试炼规则碑文残卷", PlantedAt: 1, Status: "planted"},
		{ID: "outer_disciple", Description: "外门弟子的旧债纠纷", PlantedAt: 1, Status: "planted"},
		{ID: "elder_token", Description: "长老手中令牌的来历", PlantedAt: 1, Status: "planted"},
		{ID: "hidden_gate", Description: "山门背后的隐藏通道", PlantedAt: 1, Status: "planted"},
		{ID: "trial_bet", Description: "试炼盘口的幕后操盘人", PlantedAt: 1, Status: "planted"},
	}); err != nil {
		t.Fatalf("SaveForeshadowLedger: %v", err)
	}
	if err := s.Drafts.SaveChapterPlan(domain.ChapterPlan{
		Chapter: 2,
		Title:   "试炼前夜",
		Goal:    "决定是否回应邀请",
		Contract: domain.ChapterContract{
			PayoffPoints: []string{"回应内门试炼邀请"},
			HookGoal:     "抛出谁在背后推动试炼",
		},
	}); err != nil {
		t.Fatalf("SaveChapterPlan: %v", err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter:        1,
		Scope:          "chapter",
		Verdict:        "polish",
		Summary:        "主线启动完成，但伏笔不够明确。",
		ContractStatus: "partial",
		ContractMisses: []string{"未明确埋下内门试炼邀请"},
		Issues: []domain.ConsistencyIssue{
			{Type: "hook", Severity: "warning", Description: "章末钩子不够具体"},
		},
	}); err != nil {
		t.Fatalf("SaveReview: %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 2})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload struct {
		Selected struct {
			StoryThreads  []domain.RecallItem `json:"story_threads"`
			ReviewLessons []domain.RecallItem `json:"review_lessons"`
		} `json:"selected_memory"`
		Summary string `json:"_loading_summary"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(payload.Selected.StoryThreads) == 0 {
		t.Fatal("expected story thread recall items")
	}
	if len(payload.Selected.ReviewLessons) == 0 {
		t.Fatal("expected review lesson recall items")
	}
	if !containsRecallSummary(payload.Selected.StoryThreads, "内门试炼邀请") {
		t.Fatalf("expected story thread recall to mention invite, got %+v", payload.Selected.StoryThreads)
	}
	if !containsRecallSummary(payload.Selected.StoryThreads, "推动这场试炼") {
		t.Fatalf("expected story thread recall to mention trial mastermind, got %+v", payload.Selected.StoryThreads)
	}
	if containsRecallSummary(payload.Selected.StoryThreads, "试炼规则碑文残卷") {
		t.Fatalf("expected weak-overlap foreshadow to stay out, got %+v", payload.Selected.StoryThreads)
	}
	if containsRecallSummary(payload.Selected.StoryThreads, "建议回看第") {
		t.Fatalf("expected related_chapters not to be duplicated into story_threads, got %+v", payload.Selected.StoryThreads)
	}
	if !containsRecallSummary(payload.Selected.ReviewLessons, "sót mục contract") {
		t.Fatalf("expected review lesson recall to mention contract miss, got %+v", payload.Selected.ReviewLessons)
	}
	if !strings.Contains(payload.Summary, "gợi nhớ tuyến truyện:") || !strings.Contains(payload.Summary, "gợi nhớ thẩm duyệt:") {
		t.Fatalf("expected loading summary to report selected memory, got %q", payload.Summary)
	}
}

// 睡太久的伏笔即使与当前章关键词无关，也应被回填进 story_threads——
// 这正是相关性召回的盲区（独自悬挂太久、却没在本章撞上关键词的那根线）。
// 近期被动过的伏笔（账龄 < 阈值）不应被误标为"ngủ quên"。
func TestContextToolSelectedMemorySurfacesAgingForeshadow(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// 当前章主题与所有伏笔都不沾边，确保相关性召回为空，只剩账龄回填生效。
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 50, Title: "瘟疫", CoreEvent: "林砚在城南医馆救治瘟疫病患", Scenes: []string{"熬药", "封锁街巷"}},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Progress.Init(60); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	// 6 条满足召回阈值；前两条账龄 ≥30（久挂），后四条账龄 <30（近期）。
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "ancient_seal", Description: "上古封印的裂隙", PlantedAt: 3, Status: "planted"},
		{ID: "lost_bloodline", Description: "主角失落的血脉来历", PlantedAt: 5, Status: "advanced"},
		{ID: "market_feud", Description: "昨夜集市的口角", PlantedAt: 47, Status: "planted"},
		{ID: "rumor_a", Description: "近日传闻甲", PlantedAt: 48, Status: "planted"},
		{ID: "rumor_b", Description: "近日传闻乙", PlantedAt: 48, Status: "planted"},
		{ID: "rumor_c", Description: "近日传闻丙", PlantedAt: 49, Status: "planted"},
	}); err != nil {
		t.Fatalf("SaveForeshadowLedger: %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 50})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload struct {
		Selected struct {
			StoryThreads []domain.RecallItem `json:"story_threads"`
		} `json:"selected_memory"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// 两条睡太久的伏笔应被回填，且带"chưa được đẩy"的账龄标注。
	if !containsRecallSummary(payload.Selected.StoryThreads, "上古封印的裂隙") {
		t.Fatalf("expected aging foreshadow to surface despite no relevance, got %+v", payload.Selected.StoryThreads)
	}
	if !containsRecallSummary(payload.Selected.StoryThreads, "失落的血脉") {
		t.Fatalf("expected second aging foreshadow to surface, got %+v", payload.Selected.StoryThreads)
	}
	if !containsRecallSummary(payload.Selected.StoryThreads, "chưa được đẩy") {
		t.Fatalf("expected aging item to carry dormant annotation, got %+v", payload.Selected.StoryThreads)
	}
	if !containsRecallReason(payload.Selected.StoryThreads, "ngủ quên") {
		t.Fatalf("expected aging item to state it was dormant, got %+v", payload.Selected.StoryThreads)
	}
	// 近期伏笔（账龄 < 阈值 且不相关）不应被回填。
	if containsRecallSummary(payload.Selected.StoryThreads, "昨夜集市的口角") {
		t.Fatalf("recent foreshadow must not be labeled dormant, got %+v", payload.Selected.StoryThreads)
	}
}

func TestContextToolSelectedMemoryIncludesGlobalReviewLessons(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "开端", CoreEvent: "故事开始"},
		{Chapter: 2, Title: "推进", CoreEvent: "主线继续推进"},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Progress.Init(6); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 1,
		Scope:   "global",
		Verdict: "polish",
		Summary: "全局推进合格，但角色目标表达还不够稳定。",
		Issues: []domain.ConsistencyIssue{
			{Type: "character", Severity: "warning", Description: "主角目标表达不够稳定"},
		},
	}); err != nil {
		t.Fatalf("SaveReview(global): %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 2})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload struct {
		Selected struct {
			ReviewLessons []domain.RecallItem `json:"review_lessons"`
		} `json:"selected_memory"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !containsRecallSummary(payload.Selected.ReviewLessons, "主角目标表达不够稳定") {
		t.Fatalf("expected global review lesson to be recalled, got %+v", payload.Selected.ReviewLessons)
	}
}

func TestContextToolKeepsFullForeshadowWhenRecallNotTriggered(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "起势", CoreEvent: "故事起势"},
		{Chapter: 2, Title: "推进", CoreEvent: "继续推进"},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Progress.Init(4); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "small_1", Description: "第一条小伏笔", PlantedAt: 1, Status: "planted"},
		{ID: "small_2", Description: "第二条小伏笔", PlantedAt: 1, Status: "planted"},
	}); err != nil {
		t.Fatalf("SaveForeshadowLedger: %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 2})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	episodic := payload["episodic_memory"].(map[string]any)
	if _, ok := episodic["foreshadow_ledger"]; !ok {
		t.Fatal("expected full foreshadow ledger to remain when selected recall is not triggered")
	}
	if _, ok := payload["selected_memory"]; ok {
		t.Fatalf("expected no selected_memory for small foreshadow sets, got %+v", payload["selected_memory"])
	}
}

func TestContextToolFallsBackToFullForeshadowWhenSelectionIsTooSparse(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "邀约", CoreEvent: "长老暗中给出内门试炼邀请"},
		{Chapter: 2, Title: "试炼前夜", CoreEvent: "林砚准备回应内门试炼邀请", Scenes: []string{"整理线索", "决定赴约"}},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Progress.Init(8); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "trial_invite", Description: "内门试炼邀请的真实目的", PlantedAt: 1, Status: "planted"},
		{ID: "trial_rules", Description: "试炼规则碑文残卷", PlantedAt: 1, Status: "planted"},
		{ID: "outer_disciple", Description: "外门弟子的旧债纠纷", PlantedAt: 1, Status: "planted"},
		{ID: "elder_token", Description: "长老手中令牌的来历", PlantedAt: 1, Status: "planted"},
		{ID: "hidden_gate", Description: "山门背后的隐藏通道", PlantedAt: 1, Status: "planted"},
		{ID: "trial_bet", Description: "试炼盘口的幕后操盘人", PlantedAt: 1, Status: "planted"},
	}); err != nil {
		t.Fatalf("SaveForeshadowLedger: %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 2})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	episodic := payload["episodic_memory"].(map[string]any)
	if _, ok := episodic["foreshadow_ledger"]; !ok {
		t.Fatal("expected full foreshadow ledger when selection is too sparse")
	}
	if selected, ok := payload["selected_memory"].(map[string]any); ok {
		if _, exists := selected["story_threads"]; exists {
			t.Fatalf("expected sparse story_threads to fall back to full ledger, got %+v", selected["story_threads"])
		}
	}
}

// 回填按紧迫度分档：quá hạn（due_by cũ trước）→ sắp đến hạn → ngủ quên；
// 已 bỏ tuyến 的伏笔永不召回。
func TestContextToolStoryThreadBackfillTiers(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// 本章与所有伏笔都不沾边，确保只剩回填逻辑生效。
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 50, Title: "Thủy triều", CoreEvent: "Sóng dữ dội đánh vào bờ cát", Scenes: []string{"kéo lưới", "vá thuyền"}},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Progress.Init(60); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "qua_han_cu", Description: "Chiếc hộp gỗ khắc hình rắn", PlantedAt: 1, Status: "planted", DueBy: 5},
		{ID: "qua_han_moi", Description: "Con dấu sáp đỏ trên tờ hôn thư", PlantedAt: 20, Status: "advanced", DueBy: 40, LastTouched: 20},
		{ID: "sap_den_han", Description: "Lời thề năm xưa giữa hai dòng họ", PlantedAt: 45, Status: "planted", DueBy: 51},
		{ID: "ngu_quen", Description: "Chiếc nhẫn đồng chôn sau đình", PlantedAt: 1, Status: "planted"},
		{ID: "da_bo", Description: "Tuyến truyện đã cắt", PlantedAt: 1, Status: "abandoned", DueBy: 2, AbandonedAt: 3, AbandonReason: "cắt vì trùng tuyến"},
		{ID: "moi_tao", Description: "Vết mực mới trên bàn", PlantedAt: 49, Status: "planted"},
		{ID: "moi_hai", Description: "Mảnh gương vỡ", PlantedAt: 49, Status: "planted"},
	}); err != nil {
		t.Fatalf("SaveForeshadowLedger: %v", err)
	}

	items := executeStoryThreads(t, s, 50)
	wantKeys := []string{"qua_han_cu", "qua_han_moi", "sap_den_han", "ngu_quen"}
	if len(items) != len(wantKeys) {
		t.Fatalf("muốn %d tuyến, nhận %+v", len(wantKeys), items)
	}
	for i, key := range wantKeys {
		if items[i].Key != key {
			t.Fatalf("thứ tự tuyến sai: nhận %+v, muốn %v", items, wantKeys)
		}
	}
	if !strings.Contains(items[0].Reason, "quá hạn") || !strings.Contains(items[1].Reason, "quá hạn") {
		t.Fatalf("hai tuyến quá hạn phải nói rõ quá hạn: %+v", items)
	}
	if !strings.Contains(items[2].Reason, "sắp đến hạn") {
		t.Fatalf("tuyến due_by trong [N, N+2] phải nói sắp đến hạn: %+v", items[2])
	}
	if !strings.Contains(items[3].Reason, "ngủ quên") {
		t.Fatalf("tuyến bỏ quên phải nói ngủ quên: %+v", items[3])
	}
	for _, item := range items {
		if item.Key == "da_bo" || item.Key == "moi_tao" || item.Key == "moi_hai" {
			t.Fatalf("không được召回 tuyến đã bỏ hoặc mới toanh: %+v", item)
		}
	}
}

// 刚被 đẩy 的伏笔即使埋得很早，也不该被当成 "ngủ quên" 召回。
func TestContextToolStoryThreadNotDormantAfterAdvance(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 50, Title: "Thủy triều", CoreEvent: "Sóng dữ dội đánh vào bờ cát", Scenes: []string{"kéo lưới", "vá thuyền"}},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Progress.Init(60); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	// 只有第一条埋得早，但它上一章刚被 đẩy；其余都是近期埋下的。
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "vua_day", Description: "Chiếc hộp gỗ khắc hình rắn", PlantedAt: 1, Status: "advanced", LastTouched: 49},
		{ID: "moi_a", Description: "Con dấu sáp đỏ", PlantedAt: 48, Status: "planted", LastTouched: 48},
		{ID: "moi_b", Description: "Lời thề năm xưa", PlantedAt: 48, Status: "planted", LastTouched: 48},
		{ID: "moi_c", Description: "Chiếc nhẫn đồng", PlantedAt: 49, Status: "planted", LastTouched: 49},
		{ID: "moi_d", Description: "Vết mực mới", PlantedAt: 49, Status: "planted", LastTouched: 49},
		{ID: "moi_e", Description: "Mảnh gương vỡ", PlantedAt: 49, Status: "planted", LastTouched: 49},
	}); err != nil {
		t.Fatalf("SaveForeshadowLedger: %v", err)
	}

	items := executeStoryThreads(t, s, 50)
	for _, item := range items {
		if item.Key == "vua_day" {
			t.Fatalf("phục bút vừa được đẩy không được召回 với lý do ngủ quên: %+v", item)
		}
	}
}

func executeStoryThreads(t *testing.T, s *store.Store, chapter int) []domain.RecallItem {
	t.Helper()
	tool := newTestContextTool(s, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": chapter})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var payload struct {
		Selected struct {
			StoryThreads []domain.RecallItem `json:"story_threads"`
		} `json:"selected_memory"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return payload.Selected.StoryThreads
}

func containsRecallReason(items []domain.RecallItem, want string) bool {
	for _, item := range items {
		if strings.Contains(item.Reason, want) {
			return true
		}
	}
	return false
}

func containsRecallSummary(items []domain.RecallItem, want string) bool {
	for _, item := range items {
		if strings.Contains(item.Summary, want) {
			return true
		}
	}
	return false
}

func TestContextToolInjectsRewriteBriefForPendingRewriteChapter(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(3); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(2, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := s.Progress.SetPendingRewrites([]int{2}, "节奏拖沓，需要压缩前半段", store.RewriteCauseUser); err != nil {
		t.Fatalf("SetPendingRewrites: %v", err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 2,
		Scope:   "chapter",
		Verdict: "rewrite",
		Summary: "前半段铺垫过长，冲突迟迟不出现。",
		Issues: []domain.ConsistencyIssue{
			{Type: "pacing", Severity: "error", Description: "前 2000 字无推进"},
		},
		ContractMisses: []string{"未兑现试炼开场"},
	}); err != nil {
		t.Fatalf("SaveReview: %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 2})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	working := payload["working_memory"].(map[string]any)
	brief, ok := working["rewrite_brief"].(map[string]any)
	if !ok {
		t.Fatalf("expected working_memory.rewrite_brief, got %T", working["rewrite_brief"])
	}
	if got := brief["reason"]; got != "节奏拖沓，需要压缩前半段" {
		t.Fatalf("expected rewrite reason, got %v", got)
	}
	if got, _ := brief["review_summary"].(string); !strings.Contains(got, "铺垫过长") {
		t.Fatalf("expected review summary from chapter review, got %v", brief["review_summary"])
	}
	if issues, _ := brief["issues"].([]any); len(issues) == 0 {
		t.Fatalf("expected review issues in rewrite_brief, got %v", brief["issues"])
	}
	if misses, _ := brief["contract_misses"].([]any); len(misses) == 0 {
		t.Fatalf("expected contract misses in rewrite_brief, got %v", brief["contract_misses"])
	}
}

func TestContextToolOmitsRewriteBriefForNormalChapter(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(3); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	args, err := json.Marshal(map[string]any{"chapter": 2})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	working := payload["working_memory"].(map[string]any)
	if _, ok := working["rewrite_brief"]; ok {
		t.Fatal("expected no rewrite_brief for chapter outside PendingRewrites")
	}
}

func TestContextToolLoadsArcReviewAffectingEarlierChapter(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(4); err != nil {
		t.Fatal(err)
	}
	for chapter := 1; chapter <= 4; chapter++ {
		if err := s.Progress.MarkChapterComplete(chapter, 100, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Progress.SetPendingRewrites([]int{3}, "弧评审返工", store.RewriteCauseUser); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 4, Scope: "arc", Verdict: "polish", Summary: "第二弧节奏需压缩", AffectedChapters: []int{3},
		Issues: []domain.ConsistencyIssue{{
			Type: "pacing", Severity: "error", Description: "第3章铺垫过长", Evidence: "冲突迟到",
			Chapters: []int{3}, RequiresChange: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := newTestContextTool(s, References{}, "default").Execute(context.Background(), json.RawMessage(`{"chapter":3}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatal(err)
	}
	working := payload["working_memory"].(map[string]any)
	brief, _ := working["rewrite_brief"].(map[string]any)
	if brief == nil || !strings.Contains(fmt.Sprint(brief["review_summary"]), "第二弧") {
		t.Fatalf("expected arc review handoff for chapter 3, got %#v", brief)
	}
}

func TestContextToolDoesNotInjectUserDirectives(t *testing.T) {
	// save_directive 已移除：novel_context 不再注入 working_memory.user_directives，
	// 长期写作要求统一走 user_rules。锁死这条，防止回归。
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(3); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}

	tool := newTestContextTool(s, References{}, "default")
	for name, chapter := range map[string]int{"writer": 1, "architect": 0} {
		args, _ := json.Marshal(map[string]any{"chapter": chapter})
		result, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("[%s] Execute: %v", name, err)
		}
		var payload map[string]any
		if err := json.Unmarshal(result, &payload); err != nil {
			t.Fatalf("[%s] Unmarshal: %v", name, err)
		}
		working, ok := payload["working_memory"].(map[string]any)
		if !ok {
			t.Fatalf("[%s] missing working_memory", name)
		}
		if _, exists := working["user_directives"]; exists {
			t.Errorf("[%s] working_memory 不应再有 user_directives（已统一到 user_rules）", name)
		}
		// user_rules 仍应稳定注入
		if _, ok := working["user_rules"].(map[string]any); !ok {
			t.Errorf("[%s] working_memory.user_rules 应稳定注入", name)
		}
	}
}

// TestContextToolInjectsRuleViolations 违规事实管道契约(第五轮评审):
// commit 落盘的机械违规必须经 novel_context(chapter=N) 真实注入——
// editor.md §机械检查映射消费的就是这个字段,管道断了 prompt 就成空头支票。
func TestContextToolInjectsRuleViolations(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Save(&domain.Progress{TotalChapters: 3, Phase: domain.PhaseWriting}); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.World.SaveRuleViolations(2, []rules.Violation{
		{Rule: "fatigue_words", Target: "不禁", Actual: 9, Severity: rules.SeverityWarning},
	}); err != nil {
		t.Fatalf("save violations: %v", err)
	}

	tool := newTestContextTool(st, References{}, "default")
	args, _ := json.Marshal(map[string]any{"chapter": 2})
	raw, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	vs, ok := result["rule_violations"].([]any)
	if !ok || len(vs) != 1 {
		t.Fatalf("rule_violations 必须注入章节上下文, got %v", result["rule_violations"])
	}

	// 无违规章节:字段缺省(editor.md 约定)
	args3, _ := json.Marshal(map[string]any{"chapter": 3})
	raw3, err := tool.Execute(context.Background(), args3)
	if err != nil {
		t.Fatalf("Execute ch3: %v", err)
	}
	var result3 map[string]any
	_ = json.Unmarshal(raw3, &result3)
	if _, has := result3["rule_violations"]; has {
		t.Fatal("无违规章节不应带 rule_violations 字段")
	}
}

// TestContextToolInjectsKeptReviewItems pin GAP-47 / câu 14: mục đã "Giữ nguyên"
// kèm lý do phải tới được Biên tập viên ở lần chấm sau để không báo lại.
func TestContextToolInjectsKeptReviewItems(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Save(&domain.Progress{TotalChapters: 3, Phase: domain.PhaseWriting}); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.World.SaveReviewDecision("ch2-abcd1234", store.ReviewDecision{
		State: "kept", Reason: "Cố ý: nhịp chậm để lấy không khí",
		By: "Dat", At: "2026-10-25T21:58:00+07:00",
		Ref: "Master §5", RefName: "Nhịp sóng và mật độ", Text: "Nhịp chậm ở đoạn giữa",
		Quote: "đoạn 2", Chapter: 2,
	}); err != nil {
		t.Fatalf("save decision: %v", err)
	}
	// Quyết định của chương khác không được lẫn vào.
	if err := st.World.SaveReviewDecision("ch3-ffff0000", store.ReviewDecision{
		State: "kept", Reason: "chương khác", By: "Dat", At: "2026-10-25T21:58:00+07:00",
	}); err != nil {
		t.Fatalf("save decision ch3: %v", err)
	}

	tool := newTestContextTool(st, References{}, "default")
	args, _ := json.Marshal(map[string]any{"chapter": 2})
	raw, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	block, ok := result["kept_review_items"].(map[string]any)
	if !ok {
		t.Fatalf("kept_review_items không được注入: %v", result["kept_review_items"])
	}
	items, _ := block["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v, muốn đúng mục của chương 2", items)
	}
	first, _ := items[0].(map[string]any)
	if first["reason"] != "Cố ý: nhịp chậm để lấy không khí" || first["refName"] != "Nhịp sóng và mật độ" {
		t.Fatalf("mục đã giữ = %v", first)
	}
	if note, _ := block["_usage"].(string); note == "" {
		t.Fatal("thiếu _usage giải thích cho Biên tập viên")
	}

	args1, _ := json.Marshal(map[string]any{"chapter": 1})
	raw1, err := tool.Execute(context.Background(), args1)
	if err != nil {
		t.Fatalf("Execute ch1: %v", err)
	}
	var result1 map[string]any
	_ = json.Unmarshal(raw1, &result1)
	if _, has := result1["kept_review_items"]; has {
		t.Fatal("chương không có mục giữ nguyên thì không được có kept_review_items")
	}
}

// TestContextToolChapterParamOutsideRange: chapter vô lý phải bị từ chối ngay
// bằng thông điệp nêu rõ khoảng, để model sửa tham số ở lượt sau.
//
// Trước vá: chapter=100 trả payload bình thường (coi như chương tương lai), còn
// chapter=2147483647 đi vào LoadLastReview quét lùi từng chương — hàng tỷ lần
// đọc tệp, agent treo và không có kết quả trả về (tui.log 2026-10-26 22:31).
func TestContextToolChapterParamOutsideRange(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(64); err != nil {
		t.Fatal(err)
	}
	tool := newTestContextTool(st, References{}, "default")

	for _, args := range []string{`{"chapter":65}`, `{"chapter":100}`, `{"chapter":2147483647}`} {
		if _, err := tool.Execute(context.Background(), json.RawMessage(args)); err == nil {
			t.Fatalf("%s: muốn lỗi ngoài khoảng 1..64, nhưng tool vẫn trả ngữ cảnh", args)
		} else if !strings.Contains(err.Error(), "1..64") {
			t.Fatalf("%s: lỗi phải nêu khoảng hợp lệ 1..64, got %v", args, err)
		}
	}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"chapter":-1}`)); err == nil {
		t.Fatal("chapter âm phải bị từ chối")
	} else if !strings.Contains(err.Error(), "tính từ 1") {
		t.Fatalf("lỗi chapter âm phải giải thích cách gọi đúng, got %v", err)
	}
	// Trong khoảng hợp lệ vẫn chạy; bỏ chapter vẫn là nhánh kiến trúc sư.
	for _, args := range []string{`{"chapter":64}`, `{}`} {
		if _, err := tool.Execute(context.Background(), json.RawMessage(args)); err != nil {
			t.Fatalf("%s: %v", args, err)
		}
	}
}

// TestContextToolArchitectPayloadOverBudgetStillReturnsContext: vượt ngân sách
// chỉ là cảnh báo, không phải lỗi.
//
// Trước vá: foundation_memory.premise không nằm trong danh sách cắt nên payload
// kiến trúc sư vượt 60KB là lỗi cứng — agent không có ngữ cảnh nào (production
// 61556/61440, rồi 10 lượt gọi lại novel_context với chapter rác).
func TestContextToolArchitectPayloadOverBudgetStillReturnsContext(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(64); err != nil {
		t.Fatal(err)
	}
	// premise vượt ngân sách 60KB và KHÔNG có trong danh sách cắt.
	if err := st.Outline.SavePremise(strings.Repeat("Nguyên văn thiết lập nền tảng rất dài. ", 3000)); err != nil {
		t.Fatal(err)
	}
	tool := newTestContextTool(st, References{}, "default")

	raw, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("kiến trúc sư phải nhận được ngữ cảnh kèm cảnh báo, got err = %v", err)
	}
	if len(raw) <= 60*1024 {
		t.Fatalf("payload = %d bytes, test cần vượt ngân sách 60KB để tái hiện lỗi", len(raw))
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	warnings, _ := payload["_warnings"].([]any)
	found := false
	for _, w := range warnings {
		if text, ok := w.(string); ok && strings.Contains(text, "vượt ngân sách") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("thiếu cảnh báo vượt ngân sách trong _warnings: %v", payload["_warnings"])
	}
}

// upcoming_outline 给 Writer/Editor 看当前章之后最多 2 条大纲，并带
// "kế hoạch tương lai" 标签——正文事件不得提前实现（phase 报告里的早期执行问题）。
func TestContextToolUpcomingOutlineNonLayered(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "一"}, {Chapter: 2, Title: "二"}, {Chapter: 3, Title: "三"},
		{Chapter: 4, Title: "四"}, {Chapter: 5, Title: "五"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, TotalChapters: 5}); err != nil {
		t.Fatal(err)
	}

	raw, err := newTestContextTool(s, References{}, "default").Execute(context.Background(), json.RawMessage(`{"chapter":3}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	working := payload["working_memory"].(map[string]any)
	upcoming, ok := working["upcoming_outline"].(map[string]any)
	if !ok {
		t.Fatalf("expected working_memory.upcoming_outline, got %v", working["upcoming_outline"])
	}
	if note, _ := upcoming["note"].(string); !strings.Contains(note, "chưa xảy ra") {
		t.Fatalf("upcoming_outline phải gắn nhãn kế hoạch tương lai, got %v", upcoming["note"])
	}
	entries, _ := upcoming["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("upcoming_outline entries = %d, want 2 (chương 4 và 5)", len(entries))
	}
	if got := entries[0].(map[string]any)["chapter"]; got != float64(4) {
		t.Fatalf("upcoming entry[0] chapter = %v, want 4", got)
	}

	raw, err = newTestContextTool(s, References{}, "default").Execute(context.Background(), json.RawMessage(`{"chapter":5}`))
	if err != nil {
		t.Fatal(err)
	}
	payload = map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	working = payload["working_memory"].(map[string]any)
	if _, ok := working["upcoming_outline"]; ok {
		t.Fatal("chương cuối không được có upcoming_outline")
	}
}

// Mode phân tầng: upcoming_outline chỉ lấy trong cùng arc, không lấn sang arc sau.
func TestContextToolUpcomingOutlineLayeredStaysInArc(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	volumes := []domain.VolumeOutline{{
		Index: 1, Title: "第一卷",
		Arcs: []domain.ArcOutline{
			{Index: 1, Title: "第一弧", Chapters: []domain.OutlineEntry{
				{Chapter: 1, Title: "一"}, {Chapter: 2, Title: "二"}, {Chapter: 3, Title: "三"},
			}},
			{Index: 2, Title: "第二弧", Chapters: []domain.OutlineEntry{
				{Chapter: 4, Title: "四"},
			}},
			{Index: 3, Title: "第三弧", EstimatedChapters: 47},
		},
	}}
	if err := s.Outline.SaveLayeredOutline(volumes); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline(domain.FlattenOutline(volumes)); err != nil {
		t.Fatal(err)
	}
	// 分层上下文档位只在 >50 章的长篇生效（NewContextProfile），测试必须给出
	// 同等规模，否则 profile.Layered=false 会走回非分层窗口。
	if err := s.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, Layered: true, TotalChapters: 51}); err != nil {
		t.Fatal(err)
	}

	tool := newTestContextTool(s, References{}, "default")

	// 弧中章（ch.2，弧跨 1-3）：只看到 ch.3，不越弧看到 ch.4。
	raw, err := tool.Execute(context.Background(), json.RawMessage(`{"chapter":2}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	upcoming, ok := payload["working_memory"].(map[string]any)["upcoming_outline"].(map[string]any)
	if !ok {
		t.Fatal("expected working_memory.upcoming_outline trong arc hiện tại")
	}
	entries, _ := upcoming["entries"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["chapter"] != float64(3) {
		t.Fatalf("upcoming_outline phân tầng = %v, chỉ được chứa chương 3 của cùng arc", entries)
	}

	// 弧末章（ch.3）：upcoming_outline 缺席，不让 writer 提前看到下一弧。
	raw, err = tool.Execute(context.Background(), json.RawMessage(`{"chapter":3}`))
	if err != nil {
		t.Fatal(err)
	}
	payload = map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["working_memory"].(map[string]any)["upcoming_outline"]; ok {
		t.Fatal("chương cuối arc không được có upcoming_outline")
	}
}
