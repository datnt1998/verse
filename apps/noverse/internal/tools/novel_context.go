package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore/schema"
)

// References 嵌入的参考资料。
type References struct {
	// V0
	ChapterGuide      string
	HookTechniques    string
	QualityChecklist  string
	OutlineTemplate   string
	CharacterTemplate string
	ChapterTemplate   string
	// V1
	Consistency      string
	ContentExpansion string
	DialogueWriting  string
	// V2
	StyleReference   string // 风格补充参考（可为空）
	LongformPlanning string // 通用长篇规划参考
	Differentiation  string // 通用差异化设计参考
	ArcTemplates     string // 题材弧型模板（按 style 加载，可为空）
	AntiAITone       string // 去 AI 味判据库（writer/editor 共用，全程注入）
}

// ContextTool 组装当前章节所需上下文。
type ContextTool struct {
	store      *store.Store
	refs       References
	style      string
	styleStats *StyleStatsIndex
}

type contextReads struct {
	warnings []string
	seen     map[string]struct{}
	err      error
}

func (r *contextReads) warn(scope string, err error) {
	if err == nil || os.IsNotExist(err) {
		return
	}
	msg := fmt.Sprintf("%s đọc thất bại: %v", scope, err)
	if r.seen == nil {
		r.seen = make(map[string]struct{})
	}
	if _, ok := r.seen[msg]; ok {
		return
	}
	r.seen[msg] = struct{}{}
	r.warnings = append(r.warnings, msg)
}

func (r *contextReads) require(scope string, err error) {
	if r.err != nil || err == nil || os.IsNotExist(err) || errors.Is(err, store.ErrOutlineChapterNotFound) {
		return
	}
	r.err = fmt.Errorf("%s đọc thất bại: %w", scope, err)
}

// NewContextTool 创建上下文工具。styleStats 必须与 commit_chapter 共享，
// 否则重写章节后上下文会继续读取旧统计。
// user_rules 由 buildUserRules 直接读本书快照（meta/user_rules.json）注入，不再依赖加载选项。
func NewContextTool(
	store *store.Store,
	refs References,
	style string,
	styleStats *StyleStatsIndex,
) *ContextTool {
	if styleStats == nil {
		panic("tools: NewContextTool requires StyleStatsIndex")
	}
	return &ContextTool{store: store, refs: refs, style: style, styleStats: styleStats}
}

func (t *ContextTool) Name() string { return "novel_context" }
func (t *ContextTool) Description() string {
	return "Lấy trạng thái hiện tại của truyện và ngữ cảnh sáng tác." +
		"chapter=null (hoặc bỏ tham số): trả về progress_status + thiết lập nền tảng, dùng để lập kế hoạch và quyết định bước tiếp theo." +
		"Truyền chapter=N: trả thêm tóm tắt tình tiết trước đó, phục bút, trạng thái nhân vật, quy tắc văn phong... của chương đó"
}
func (t *ContextTool) Label() string { return "Nạp ngữ cảnh" }

// 纯读工具，可被并发调度。
func (t *ContextTool) ReadOnly(_ json.RawMessage) bool        { return true }
func (t *ContextTool) ConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *ContextTool) Schema() map[string]any {
	return schema.Object(
		schema.Property("chapter", map[string]any{
			"type":        []string{"integer", "null"},
			"description": "Kiến trúc sư lập nền tảng: truyền null để lấy progress_status và foundation_memory. Writer/Editor: truyền số chương từ 1 tới tổng số chương của đại cương để lấy ngữ cảnh chương đó. Không dùng chapter=1 thay cho null khi truyện chưa có nền tảng.",
		}).Required(),
	)
}

func (t *ContextTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter int `json:"chapter"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	// Chặn sớm số chương vô lý: trước đây chapter=2147483647 làm LoadLastReview
	// quét lùi từng chương (hàng tỷ lần đọc tệp) và treo agent (tui.log
	// 2026-10-26 22:31, không có kết quả trả về).
	if err := checkChapterParam(t.store, a.Chapter); err != nil {
		return nil, err
	}

	result := make(map[string]any)
	reads := &contextReads{}

	if a.Chapter > 0 {
		// Writer 路径：加载全量基础数据 + 章节上下文
		t.buildBaseContext(result, reads)
		seed := newChapterContextEnvelope()
		state := t.prepareChapterContext(a.Chapter, &seed, reads)
		seed.apply(result)
		t.buildChapterContext(result, state, reads)
		t.applyGraphContext(result, state, reads)
		// 该章的机械违规事实(commit 时按 user_rules 检查并落盘):
		// editor 评审据此映射进七维(editor.md §机械检查映射);writer 返工时自查。
		if violations := t.store.World.LoadRuleViolations(a.Chapter); len(violations) > 0 {
			result["rule_violations"] = violations
		}
		// GAP-47: mục người dùng đã "Giữ nguyên" kèm lý do — Editor không báo lại ở
		// lần chấm này trừ khi có bằng chứng mới (câu 14).
		if kept := t.keptReviewItems(a.Chapter); len(kept) > 0 {
			result["kept_review_items"] = map[string]any{
				"_usage": "người dùng đã giữ nguyên các mục thẩm duyệt này kèm lý do; lần chấm này không báo lại chúng trừ khi có bằng chứng mới",
				"items":  kept,
			}
		}
		// episodic 是已写入正文的备忘，不是待写素材。
		if epi, ok := result["episodic_memory"].(map[string]any); ok && len(epi) > 0 {
			epi["_usage"] = "vùng chứa này là bản ghi sự thật của phần đã viết (dùng để đối chiếu nhất quán và nối tiếp); nhắc lại nguyên văn những nội dung này trong chương mới là lỗi lặp"
		}
	} else {
		// Architect 路径：只返回状态 + 结构化数据，不加载全量原文
		t.buildProgressStatus(result, reads)
		t.buildArchitectContext(result, reads)
	}

	// 注入 working_memory.user_rules（canonical 路径）。架构师路径原本没有 working_memory，
	// 由 buildUserRules 按需新建只装 user_rules 的容器。快照缺失时退到内置默认，
	// 始终输出稳定结构，避免 LLM 看到 user_rules=null 走异常分支。
	if a.Chapter > 0 {
		t.buildSimulationProfile(result, "working_memory", reads)
	} else {
		t.buildSimulationProfile(result, "planning_memory", reads)
	}

	t.buildUserRules(result, reads)

	if reads.err != nil {
		return nil, reads.err
	}
	if len(reads.warnings) > 0 {
		result["_warnings"] = reads.warnings
	}

	// 优先级预算：总大小超过阈值时裁剪低优先级数据；摘要在裁剪完成后重建，
	// 确保展示的字段数量和 _trimmed 与最终 payload 一致。
	budget := 60 * 1024
	if a.Chapter > 0 {
		budget = 100 * 1024
	}
	return finalizeContextPayload(result, a.Chapter, budget)
}

// keptReviewItems trả các mục thẩm duyệt của chương mà người dùng đã "Giữ
// nguyên" kèm lý do (GAP-47). itemId mang số chương ("ch<N>-…") nên lọc được
// quyết định của đúng chương mà không cần đọc lại bài thẩm duyệt.
func (t *ContextTool) keptReviewItems(chapter int) []map[string]any {
	if t.store == nil || chapter < 1 {
		return nil
	}
	decisions, err := t.store.World.LoadReviewDecisions()
	if err != nil || len(decisions) == 0 {
		return nil
	}
	prefix := fmt.Sprintf("ch%d-", chapter)
	ids := make([]string, 0, len(decisions))
	for id := range decisions {
		if strings.HasPrefix(id, prefix) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		d := decisions[id]
		out = append(out, map[string]any{
			"ref":     d.Ref,
			"refName": d.RefName,
			"text":    d.Text,
			"quote":   d.Quote,
			"reason":  d.Reason,
			"by":      d.By,
			"at":      d.At,
		})
	}
	return out
}

// finalizeContextPayload ghép payload cuối cùng với ngân sách mềm.
//
// Ngân sách KHÔNG còn là điều kiện lỗi: khi không cắt được xuống dưới hạn,
// payload đã cắt tối đa vẫn được trả kèm cảnh báo trong _warnings. Trả lỗi
// nghĩa là agent không có ngữ cảnh nào để làm việc — production 2026-10-26:
// kiến trúc sư vượt 116 byte (61556/61440), gọi lại novel_context 10 lần với
// chapter 0/-1/1/9/25/100/2147483647 rồi treo ở chương 2147483647.
func finalizeContextPayload(result map[string]any, chapter, budget int) (json.RawMessage, error) {
	if err := trimByBudget(result, budget); err != nil {
		noteBudgetOverflow(result, err)
	}
	result["_loading_summary"] = buildLoadingSummary(result, chapter)
	if err := trimByBudget(result, budget); err != nil {
		noteBudgetOverflow(result, err)
	}
	result["_loading_summary"] = buildLoadingSummary(result, chapter)

	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal context payload: %w", err)
	}
	if len(data) > budget {
		noteBudgetOverflow(result, fmt.Errorf("context payload exceeds budget after summary rebuild: size=%d budget=%d", len(data), budget))
		result["_loading_summary"] = buildLoadingSummary(result, chapter)
		if data, err = json.Marshal(result); err != nil {
			return nil, fmt.Errorf("marshal context payload: %w", err)
		}
	}
	return data, nil
}

// noteBudgetOverflow ghi cảnh báo vượt ngân sách vào _warnings, nêu rõ là KHÔNG
// nên gọi lại cùng tham số (đó chính là vòng lặp đã đốt 10 lượt trong production).
func noteBudgetOverflow(result map[string]any, cause error) {
	msg := fmt.Sprintf("ngữ cảnh vượt ngân sách và đã cắt hết mục ưu tiên thấp (%v); dùng ngữ cảnh này, ĐỪNG gọi lại y nguyên tham số — nếu thiếu dữ liệu cụ thể hãy đọc riêng bằng công cụ khác", cause)
	warnings, _ := result["_warnings"].([]string)
	for _, w := range warnings {
		if w == msg {
			return
		}
	}
	result["_warnings"] = append(warnings, msg)
}

// buildLoadingSummary 从已组装的 result 中统计各项数据量，生成一行可读摘要。
func buildLoadingSummary(result map[string]any, chapter int) string {
	var parts []string
	working, _ := result["working_memory"].(map[string]any)
	episodic, _ := result["episodic_memory"].(map[string]any)
	planning, _ := result["planning_memory"].(map[string]any)
	foundation, _ := result["foundation_memory"].(map[string]any)
	referencePack, _ := result["reference_pack"].(map[string]any)

	if chapter > 0 {
		parts = append(parts, fmt.Sprintf("ch=%d", chapter))
		if tier, ok := episodic["planning_tier"].(domain.PlanningTier); ok && tier != "" {
			parts = append(parts, fmt.Sprintf("tier=%s", tier))
		}
	} else {
		parts = append(parts, "architect")
		if tier, ok := planning["planning_tier"].(domain.PlanningTier); ok && tier != "" {
			parts = append(parts, fmt.Sprintf("tier=%s", tier))
		}
	}

	if pos, ok := episodic["position"].(map[string]any); ok {
		parts = append(parts, fmt.Sprintf("V%dA%d", pos["volume"], pos["arc"]))
	}

	var items []string

	if n := firstSliceLen(episodic["character_snapshots"], foundation["character_snapshots"]); n > 0 {
		items = append(items, fmt.Sprintf("nhân vật:%d(ảnh chụp)", n))
	} else if n := firstSliceLen(episodic["characters"], foundation["characters"]); n > 0 {
		items = append(items, fmt.Sprintf("nhân vật:%d", n))
	}

	if len(working) > 0 {
		items = append(items, fmt.Sprintf("bộ nhớ làm việc:%d", len(working)))
	}
	if len(episodic) > 0 {
		items = append(items, fmt.Sprintf("bộ nhớ tình tiết:%d", len(episodic)))
	}
	if len(planning) > 0 {
		items = append(items, fmt.Sprintf("bộ nhớ quy hoạch:%d", len(planning)))
	}
	if len(foundation) > 0 {
		items = append(items, fmt.Sprintf("bộ nhớ nền tảng:%d", len(foundation)))
	}

	if n := firstSliceLen(working["volume_summaries"], planning["volume_summaries"]); n > 0 {
		items = append(items, fmt.Sprintf("tóm tắt tập:%d", n))
	}
	if n := firstSliceLen(working["arc_summaries"], planning["arc_summaries"]); n > 0 {
		items = append(items, fmt.Sprintf("tóm tắt cung:%d", n))
	}
	if n := sliceLen(working["recent_summaries"]); n > 0 {
		items = append(items, fmt.Sprintf("tóm tắt chương:%d", n))
	}

	if n := sliceLen(planning["layered_outline"]); n > 0 {
		items = append(items, fmt.Sprintf("đại cương phân tầng:%d tập", n))
	}

	if n := sliceLen(working["timeline"]); n > 0 {
		items = append(items, fmt.Sprintf("dòng thời gian:%d", n))
	}
	if n := firstSliceLen(episodic["foreshadow_ledger"], foundation["foreshadow_ledger"]); n > 0 {
		items = append(items, fmt.Sprintf("phục bút:%d", n))
	}
	if n := sliceLen(episodic["relationship_state"]); n > 0 {
		items = append(items, fmt.Sprintf("quan hệ:%d", n))
	}
	if n := sliceLen(episodic["recent_state_changes"]); n > 0 {
		items = append(items, fmt.Sprintf("thay đổi trạng thái:%d", n))
	}
	if _, ok := working["previous_tail"]; ok {
		items = append(items, "đuôi chương trước:ok")
	}
	if _, ok := referencePack["style_rules"]; ok {
		items = append(items, "quy tắc văn phong:ok")
	}
	if n := sliceLen(episodic["related_chapters"]); n > 0 {
		items = append(items, fmt.Sprintf("chương liên quan:%d", n))
	}
	if selected, ok := result["selected_memory"].(map[string]any); ok && len(selected) > 0 {
		if n := sliceLen(selected["story_threads"]); n > 0 {
			items = append(items, fmt.Sprintf("gợi nhớ tuyến truyện:%d", n))
		}
		if n := sliceLen(selected["review_lessons"]); n > 0 {
			items = append(items, fmt.Sprintf("gợi nhớ thẩm duyệt:%d", n))
		}
	}

	if refs, ok := referencePack["references"].(map[string]string); ok && len(refs) > 0 {
		items = append(items, fmt.Sprintf("tham chiếu:%d mục", len(refs)))
	}
	if len(referencePack) > 0 {
		items = append(items, fmt.Sprintf("gói tham chiếu:%d", len(referencePack)))
	}
	if _, ok := result["memory_policy"]; ok {
		items = append(items, "chiến lược bộ nhớ:ok")
	}
	if _, ok := working["simulation_profile"]; ok {
		items = append(items, "chân dung mô phỏng:ok")
	} else if _, ok := planning["simulation_profile"]; ok {
		items = append(items, "chân dung mô phỏng:ok")
	}
	if warnings, ok := result["_warnings"].([]string); ok && len(warnings) > 0 {
		items = append(items, fmt.Sprintf("cảnh báo:%d", len(warnings)))
	}
	if trimmed, ok := result["_trimmed"].([]string); ok && len(trimmed) > 0 {
		items = append(items, fmt.Sprintf("cắt bớt:%s", strings.Join(trimmed, ",")))
	}

	if len(items) > 0 {
		parts = append(parts, strings.Join(items, " "))
	}
	return strings.Join(parts, " | ")
}

// sliceLen 对 any 类型尝试取 slice 长度。
func sliceLen(v any) int {
	switch s := v.(type) {
	case []domain.ChapterSummary:
		return len(s)
	case []domain.ArcSummary:
		return len(s)
	case []domain.VolumeSummary:
		return len(s)
	case []domain.CharacterSnapshot:
		return len(s)
	case []domain.TimelineEvent:
		return len(s)
	case []domain.ForeshadowEntry:
		return len(s)
	case []domain.RelationshipEntry:
		return len(s)
	case []domain.StateChange:
		return len(s)
	case []domain.VolumeOutline:
		return len(s)
	case []domain.Character:
		return len(s)
	case []domain.RelatedChapter:
		return len(s)
	case []domain.RecallItem:
		return len(s)
	case []planningVolumeOutline:
		return len(s)
	default:
		return 0
	}
}

func firstSliceLen(values ...any) int {
	for _, value := range values {
		if n := sliceLen(value); n > 0 {
			return n
		}
	}
	return 0
}

// loadFilteredCharacters 按 Tier 和场景出场过滤角色。
// core/important 始终返回；secondary/decorative 只在当前章节大纲提及时返回。
func (t *ContextTool) loadFilteredCharacters(result map[string]any, chapter int, reads *contextReads) {
	chars, err := t.store.Characters.Load()
	if err != nil {
		reads.require("characters", err)
		return
	}
	if len(chars) == 0 {
		return
	}

	// 获取当前章节大纲的场景描述，用于匹配次要角色
	entry, err := t.store.Outline.GetChapterOutline(chapter)
	if err != nil {
		reads.require("current_chapter_outline", err)
		result["characters"] = chars
		return
	}
	if entry == nil {
		result["characters"] = chars
		return
	}
	sceneText := strings.Join(entry.Scenes, " ") + " " + entry.CoreEvent + " " + entry.Title

	var filtered []domain.Character
	for _, c := range chars {
		switch c.Tier {
		case "secondary", "decorative":
			if matchCharacter(sceneText, c) {
				filtered = append(filtered, c)
			}
		default: // core, important, 或未设置
			filtered = append(filtered, c)
		}
	}
	result["characters"] = filtered
}

// matchCharacter 检查场景文本中是否包含角色的正式名或任一别名。
func matchCharacter(text string, c domain.Character) bool {
	if strings.Contains(text, c.Name) {
		return true
	}
	for _, alias := range c.Aliases {
		if strings.Contains(text, alias) {
			return true
		}
	}
	return false
}

// loadLayeredSummaries 分层摘要加载：卷摘要 + 当前卷弧摘要 + 弧内章摘要。
func (t *ContextTool) loadLayeredSummaries(result map[string]any, chapter, summaryWindow int, reads *contextReads) {
	vol, arc, err := t.store.Outline.LocateChapter(chapter)
	if err != nil {
		reads.require("layered_outline_position", err)
		return
	}

	// 1. 已完成卷的卷摘要
	if volSummaries, err := t.store.Summaries.LoadAllVolumeSummaries(); err == nil && len(volSummaries) > 0 {
		result["volume_summaries"] = volSummaries
	} else {
		reads.require("volume_summaries", err)
	}

	// 2. 当前卷内已完成弧的弧摘要（不含当前弧）
	if arcSummaries, err := t.store.Summaries.LoadArcSummaries(vol); err == nil && len(arcSummaries) > 0 {
		var prior []domain.ArcSummary
		for _, s := range arcSummaries {
			if s.Arc < arc {
				prior = append(prior, s)
			}
		}
		if len(prior) > 0 {
			result["arc_summaries"] = prior
		}
	} else {
		reads.require("arc_summaries", err)
	}

	// 3. 当前弧内最近 N 章的章摘要
	if summaries, err := t.store.Summaries.LoadRecentSummaries(chapter, summaryWindow); err == nil && len(summaries) > 0 {
		result["recent_summaries"] = summaries
	} else {
		reads.require("recent_summaries", err)
	}
}

// loadLayeredCharacters Layered 模式下的角色加载：优先用最近快照，回退到原始设定 + Tier 过滤。
func (t *ContextTool) loadLayeredCharacters(result map[string]any, chapter int, reads *contextReads) {
	snapshots, err := t.store.Characters.LoadLatestSnapshots()
	if err == nil && len(snapshots) > 0 {
		result["character_snapshots"] = snapshots
		// 同时保留原始设定中的 core/important 角色（快照可能不含新登场角色）
		t.loadFilteredCharacters(result, chapter, reads)
		return
	}
	reads.require("character_snapshots", err)
	// 无快照时回退到原始设定
	t.loadFilteredCharacters(result, chapter, reads)
}

// writerReferences 返回写作参考资料。章节 1 返回全量，后续章节裁剪掉不再需要的模板。
func (t *ContextTool) writerReferences(chapter int) map[string]string {
	refs := map[string]string{}
	add := func(k, v string) {
		if v != "" {
			refs[k] = v
		}
	}
	// 渐进式加载：始终保留核心参考，前 3 章额外加载完整写作指南
	add("consistency", t.refs.Consistency)
	add("hook_techniques", t.refs.HookTechniques)
	add("quality_checklist", t.refs.QualityChecklist)
	add("anti_ai_tone", t.refs.AntiAITone) // 去 AI 味判据全程注入，不随章节裁剪
	if chapter <= 3 {
		add("chapter_guide", t.refs.ChapterGuide)
		add("dialogue_writing", t.refs.DialogueWriting)
		add("style_reference", t.refs.StyleReference)
	}

	// 仅首章加载的补充参考
	if chapter <= 1 {
		add("chapter_template", t.refs.ChapterTemplate)
		add("content_expansion", t.refs.ContentExpansion)
	}
	return refs
}

func (t *ContextTool) architectReferences() map[string]string {
	refs := map[string]string{}
	add := func(k, v string) {
		if v != "" {
			refs[k] = v
		}
	}
	add("outline_template", t.refs.OutlineTemplate)
	add("character_template", t.refs.CharacterTemplate)
	add("longform_planning", t.refs.LongformPlanning)
	add("differentiation", t.refs.Differentiation)
	add("style_reference", t.refs.StyleReference)
	add("arc_templates", t.refs.ArcTemplates)
	add("anti_ai_tone", t.refs.AntiAITone) // architect 大纲去 AI 腔；亦兜 editor 走 Chapter=0 路径
	return refs
}

// foundationStatus 检查基础设定的完备性，返回缺失项列表。
// 与 save_foundation 工具共用 store.FoundationMissing 判定逻辑，保证 LLM 从
// novel_context 看到的 ready/missing 与 save_foundation 返回的 foundation_ready
// 永远一致（长篇 compass 必需项等细节不会漂移）。
func (t *ContextTool) foundationStatus() (map[string]any, error) {
	missing, err := t.store.FoundationMissing()
	if err != nil {
		return nil, err
	}
	status := map[string]any{"ready": len(missing) == 0}
	if len(missing) > 0 {
		status["missing"] = missing
	}
	if len(missing) == 1 && missing[0] == "foundation_audit" {
		fingerprint, err := t.store.FoundationFingerprint()
		if err != nil {
			return nil, err
		}
		status["fingerprint"] = fingerprint
	}
	if audit, err := t.store.Outline.LoadFoundationAudit(); err != nil {
		return nil, err
	} else if audit != nil && !audit.Ready {
		status["last_audit"] = audit
	}
	return status, nil
}

// trimByBudget 按优先级裁剪 result，使 JSON 总大小不超过 budget 字节。
// 优先级（从低到高）：premise_sections/skeleton_arcs（dữ liệu dẫn xuất）
// < references < voice_samples < style_anchors < style_rules
//
//	< evidence_passages < previous_tail < timeline
//
//	< recent_state_changes < foreshadow_ledger < relationship_state < 其余（不裁剪）
//
// style_stats 是体积有界的全书级核心信号，不参与裁剪。
//
// 裁剪的 key 会记录到 result["_trimmed"] 供日志排查。
func trimByBudget(result map[string]any, budget int) error {
	// 先测量当前大小
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("measure context payload: %w", err)
	}
	if len(data) <= budget {
		return nil
	}

	// 按优先级从低到高列出可裁剪的 key
	trimOrder := []string{
		// Dữ liệu dẫn xuất, cắt được mà không mất thông tin: premise_sections là
		// bản tách của premise (nguyên văn vẫn còn), skeleton_arcs là bản chiếu
		// của layered_outline. Production 2026-10-26: payload kiến trúc sư vượt
		// ngân sách 116 byte và bị trả lỗi nên agent không có ngữ cảnh nào.
		"premise_sections",
		"skeleton_arcs",
		"references",
		"voice_samples",
		"style_anchors",
		"style_rules",
		// evidence_passages 可由 quote 在原章节里重新回取，丢失可再生成；
		// 但它比 previous_tail 更接近事实，故排在其前先被裁剪。
		"evidence_passages",
		"previous_tail",
		"timeline",
		"recent_state_changes",
		"foreshadow_ledger",
		"relationship_state",
	}

	trimmed, _ := result["_trimmed"].([]string)
	trimmed = append([]string(nil), trimmed...)
	if graph, _ := result["_graph_context"].(bool); graph {
		for {
			label, ok := trimGraphItem(result, trimOrder)
			if !ok {
				break
			}
			trimmed = append(trimmed, label)
			result["_trimmed"] = trimmed
			data, err = json.Marshal(result)
			if err != nil {
				return fmt.Errorf("measure trimmed context payload: %w", err)
			}
			if len(data) <= budget {
				return nil
			}
		}
		return fmt.Errorf("context payload exceeds budget after item trimming: size=%d budget=%d", len(data), budget)
	}
	for _, key := range trimOrder {
		if !deleteContextKey(result, key) {
			continue
		}
		trimmed = append(trimmed, key)
		result["_trimmed"] = append([]string(nil), trimmed...)
		data, err = json.Marshal(result)
		if err != nil {
			return fmt.Errorf("measure trimmed context payload: %w", err)
		}
		if len(data) <= budget {
			return nil
		}
	}
	return fmt.Errorf("context payload exceeds budget after trimming: size=%d budget=%d", len(data), budget)
}

func deleteContextKey(result map[string]any, key string) bool {
	deleted := false
	for _, containerKey := range []string{
		"working_memory",
		"episodic_memory",
		"planning_memory",
		"foundation_memory",
		"reference_pack",
		"selected_memory",
	} {
		section, ok := result[containerKey].(map[string]any)
		if !ok {
			continue
		}
		if _, ok := section[key]; ok {
			delete(section, key)
			deleted = true
		}
	}
	return deleted
}

// buildRelatedChapters 根据结构化数据反查与当前章相关的历史章节。
// 从伏笔、角色出场、状态变化、关系四个维度推荐，去重后最多返回 5 条。
// 所有数据通过参数传入，不做额外 IO。
func (t *ContextTool) buildRelatedChapters(
	chapter int,
	entry *domain.OutlineEntry,
	foreshadow []domain.ForeshadowEntry,
	relationships []domain.RelationshipEntry,
	stateChanges []domain.StateChange,
	reads *contextReads,
) []domain.RelatedChapter {
	const recentWindow = 10
	const maxResults = 5

	seen := make(map[int]struct{})
	var results []domain.RelatedChapter
	add := func(ch int, reason string) {
		if ch <= 0 || ch >= chapter {
			return
		}
		// 最近几章太近，不推荐
		if ch > chapter-recentWindow {
			return
		}
		if _, ok := seen[ch]; ok {
			return
		}
		seen[ch] = struct{}{}
		results = append(results, domain.RelatedChapter{Chapter: ch, Reason: reason})
	}

	// 拼接大纲文本用于关键词匹配
	outlineText := entry.Title + " " + entry.CoreEvent
	for _, s := range entry.Scenes {
		outlineText += " " + s
	}

	// 1. 伏笔反查：活跃伏笔的描述是否与当前章大纲相关
	for _, f := range foreshadow {
		if strings.Contains(outlineText, f.ID) || containsAny(outlineText, strings.Fields(f.Description)) {
			add(f.PlantedAt, fmt.Sprintf("chương gieo phục bút %s (%s)", f.ID, truncateRunes(f.Description, 15)))
		}
		if len(results) >= maxResults {
			break
		}
	}

	// 2. 角色出场反查：批量单次遍历，IO 从 O(角色数×章节数) 降为 O(章节数)
	chars, err := t.store.Characters.Load()
	if err != nil {
		reads.warn("related_chapters.characters", err)
	}
	outlineChars := matchOutlineCharacters(outlineText, chars)
	if len(outlineChars) > 0 {
		appearances, err := t.store.Summaries.FindCharacterAppearances(outlineChars, chapter, recentWindow)
		if err != nil {
			reads.warn("related_chapters.summaries", err)
		}
		for _, name := range outlineChars {
			if len(results) >= maxResults {
				break
			}
			if ch, ok := appearances[name]; ok {
				add(ch, fmt.Sprintf("chương xuất hiện cuối của nhân vật '%s'", name))
			}
		}
	}

	// 3. 状态变化反查：在已加载的 slice 上操作，零 IO
	for _, name := range outlineChars {
		if len(results) >= maxResults {
			break
		}
		ch := findLastStateChange(stateChanges, name, chapter)
		if ch > 0 && ch <= chapter-recentWindow {
			add(ch, fmt.Sprintf("chương thay đổi trạng thái của '%s'", name))
		}
	}

	// 4. 关系反查：当前章涉及的角色对之间关系最后变化
	if len(relationships) > 0 && len(outlineChars) >= 2 {
		charSet := make(map[string]struct{}, len(outlineChars))
		for _, c := range outlineChars {
			charSet[c] = struct{}{}
		}
		for _, r := range relationships {
			if len(results) >= maxResults {
				break
			}
			_, aIn := charSet[r.CharacterA]
			_, bIn := charSet[r.CharacterB]
			if aIn && bIn {
				add(r.Chapter, fmt.Sprintf("thay đổi quan hệ %s-%s", r.CharacterA, r.CharacterB))
			}
		}
	}

	return results
}

// findLastStateChange 在已加载的状态变化列表中查找实体最近一次变化的章节号。
func findLastStateChange(changes []domain.StateChange, entity string, currentChapter int) int {
	for i := len(changes) - 1; i >= 0; i-- {
		if changes[i].Entity == entity && changes[i].Chapter < currentChapter {
			return changes[i].Chapter
		}
	}
	return 0
}

// matchOutlineCharacters 从大纲文本中匹配出场角色名。
func matchOutlineCharacters(text string, chars []domain.Character) []string {
	var matched []string
	for _, c := range chars {
		if strings.Contains(text, c.Name) {
			matched = append(matched, c.Name)
			continue
		}
		for _, alias := range c.Aliases {
			if strings.Contains(text, alias) {
				matched = append(matched, c.Name)
				break
			}
		}
	}
	return matched
}

// containsAny 检查 text 是否包含 words 中的任一词（至少 2 字才匹配，避免噪音）。
func containsAny(text string, words []string) bool {
	for _, w := range words {
		if len([]rune(w)) >= 2 && strings.Contains(text, w) {
			return true
		}
	}
	return false
}

func (t *ContextTool) selectStoryThreads(state contextBuildState) []domain.RecallItem {
	if state.currentEntry == nil {
		return nil
	}

	const maxThreads = 5
	var items []domain.RecallItem
	seen := make(map[string]struct{})
	picked := make(map[string]struct{}) // 已选中的伏笔 ID，供到期/睡过头回填去重
	add := func(item domain.RecallItem) {
		key := item.Kind + "|" + item.Key + "|" + item.Summary
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		picked[item.Key] = struct{}{}
		items = append(items, item)
	}

	// 1. 相关性召回：与当前章 focus 词重叠的伏笔。
	focusTerms := recallFocusTerms(state.currentEntry, state.chapterPlan)
	focusText := strings.Join(focusTerms, " ")
	// 少量伏笔仍按旧策略不重复做关键词摘要；到期/休眠提醒不能被数量门槛吞掉。
	for _, entry := range state.foreshadow {
		if len(state.foreshadow) < storyThreadRecallThreshold || !domain.ForeshadowOpen(entry.Status) {
			continue
		}
		if !matchesRecallTerms(entry.ID+" "+entry.Description, focusTerms) && !strings.Contains(focusText, entry.ID) {
			continue
		}
		add(domain.RecallItem{
			Kind:    "story_thread",
			Key:     entry.ID,
			Chapter: entry.PlantedAt,
			Reason:  "chương hiện tại có thể cần nối tiếp phục bút đã có",
			Summary: fmt.Sprintf("phục bút “%s” gieo ở chương %d: %s", entry.ID, entry.PlantedAt, truncateRunes(entry.Description, 30)),
		})
		if len(items) >= maxThreads {
			return items
		}
	}

	// 2. 到期/睡过头回填：关键词召回天然只看得见与本章相关的线，看不见那些
	//    与本章无关、却已经拖得太久的。按紧迫度补足剩余名额：已过 due_by 的
	//    （到期章早的优先）→ due_by 落在 [N, N+2] 的 → 睡太久的（最近一次被
	//    推得早的优先）。补进去的每条都写明是哪一种，让模型知道为什么此刻提它。
	completed := 0
	if state.progress != nil {
		completed = len(state.progress.CompletedChapters)
	}
	priorityRecall := false
	for _, item := range storyThreadBackfill(state.foreshadow, state.chapter, completed, picked) {
		add(item)
		priorityRecall = true
		if len(items) >= maxThreads {
			break
		}
	}
	if len(items) < storyThreadRecallMinSelected && !priorityRecall {
		return nil
	}

	return items
}

// storyThreadBackfill 排出关键词召回之外的伏笔，按紧迫度分三档：
// overdue（已过 due_by，到期章早的优先）、dueSoon（due_by 在 [chapter, chapter+2]）、
// dormant（距最近一次 plant/advance 太久的，最近一次被推得早的优先）。
// 跳过 picked 中已被关键词召回选中的；已收束的伏笔永不回填。
func storyThreadBackfill(all []domain.ForeshadowEntry, chapter, completed int, picked map[string]struct{}) []domain.RecallItem {
	var overdue, dueSoon, dormant []domain.ForeshadowEntry
	for _, entry := range all {
		if !domain.ForeshadowOpen(entry.Status) {
			continue
		}
		if _, ok := picked[entry.ID]; ok {
			continue
		}
		isOverdue, isDormant := domain.ThreadHealth(entry, chapter, completed)
		switch {
		case isOverdue:
			overdue = append(overdue, entry)
		case entry.DueBy >= chapter && entry.DueBy <= chapter+storyThreadDueSoonWindow:
			dueSoon = append(dueSoon, entry)
		case isDormant:
			dormant = append(dormant, entry)
		}
	}
	sort.SliceStable(overdue, func(i, j int) bool { return overdue[i].DueBy < overdue[j].DueBy })
	sort.SliceStable(dueSoon, func(i, j int) bool { return dueSoon[i].DueBy < dueSoon[j].DueBy })
	sort.SliceStable(dormant, func(i, j int) bool { return threadTouchedAt(dormant[i]) < threadTouchedAt(dormant[j]) })

	items := make([]domain.RecallItem, 0, len(overdue)+len(dueSoon)+len(dormant))
	for _, entry := range overdue {
		items = append(items, domain.RecallItem{
			Kind:    "story_thread",
			Key:     entry.ID,
			Chapter: entry.PlantedAt,
			Reason:  "phục bút đã quá hạn thu, cần đẩy hoặc thu ngay từ chương này",
			Summary: fmt.Sprintf("phục bút “%s” quá hạn thu từ chương %d (hiện chương %d): %s",
				entry.ID, entry.DueBy, chapter, truncateRunes(entry.Description, 30)),
		})
	}
	for _, entry := range dueSoon {
		items = append(items, domain.RecallItem{
			Kind:    "story_thread",
			Key:     entry.ID,
			Chapter: entry.PlantedAt,
			Reason:  "phục bút sắp đến hạn thu, nên chuẩn bị đẩy hoặc thu đúng lúc",
			Summary: fmt.Sprintf("phục bút “%s” dự kiến thu ở chương %d (hiện chương %d): %s",
				entry.ID, entry.DueBy, chapter, truncateRunes(entry.Description, 30)),
		})
	}
	for _, entry := range dormant {
		touched := threadTouchedAt(entry)
		items = append(items, domain.RecallItem{
			Kind:    "story_thread",
			Key:     entry.ID,
			Chapter: entry.PlantedAt,
			Reason:  "phục bút ngủ quên lâu chưa được đẩy, nên đẩy hoặc thu đúng lúc",
			Summary: fmt.Sprintf("phục bút “%s” gieo ở chương %d, đã %d chương chưa được đẩy: %s",
				entry.ID, entry.PlantedAt, chapter-touched, truncateRunes(entry.Description, 30)),
		})
	}
	return items
}

// threadTouchedAt 是伏笔"最近一次被动过"的章号；老账本没有 last_touched 时退回埋设章。
func threadTouchedAt(e domain.ForeshadowEntry) int {
	return max(e.LastTouched, e.PlantedAt)
}

func (t *ContextTool) selectReviewLessons(chapter int, reads *contextReads) []domain.RecallItem {
	if chapter <= 1 {
		return nil
	}

	var items []domain.RecallItem
	seen := make(map[string]struct{})
	add := func(item domain.RecallItem) {
		key := item.Summary
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		items = append(items, item)
	}

	appendReview := func(review *domain.ReviewEntry) bool {
		if review == nil {
			return false
		}
		for i, miss := range review.ContractMisses {
			add(domain.RecallItem{
				Kind:    "review_lesson",
				Key:     fmt.Sprintf("review-%d-contract-%d", review.Chapter, i),
				Chapter: review.Chapter,
				Reason:  "thẩm duyệt gần nhất chỉ ra mục contract bị bỏ sót",
				Summary: fmt.Sprintf("chương %d sót mục contract: %s", review.Chapter, miss),
			})
			if len(items) >= 3 {
				return true
			}
		}
		for i, issue := range review.Issues {
			switch issue.Severity {
			case "", "warning", "error", "critical":
				add(domain.RecallItem{
					Kind:    "review_lesson",
					Key:     fmt.Sprintf("review-%d-issue-%d", review.Chapter, i),
					Chapter: review.Chapter,
					Reason:  "thẩm duyệt gần nhất chỉ ra vấn đề cần tránh lặp lại",
					Summary: fmt.Sprintf("nhắc nhở thẩm duyệt chương %d: %s", review.Chapter, truncateRunes(issue.Description, 36)),
				})
			}
			if len(items) >= 3 {
				return true
			}
		}
		return false
	}

	for ch := chapter - 1; ch >= max(chapter-3, 1); ch-- {
		review, err := t.store.World.LoadReview(ch)
		if err != nil {
			reads.warn("review", err)
			continue
		}
		if appendReview(review) {
			return items
		}
	}

	globalReview, err := t.store.World.LoadLastReview(chapter - 1)
	if err != nil {
		reads.warn("global_review", err)
	} else if appendReview(globalReview) {
		return items
	}
	return items
}

func recallFocusTerms(entry *domain.OutlineEntry, plan *domain.ChapterPlan) []string {
	if entry == nil {
		return nil
	}
	var terms []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			terms = append(terms, v)
		}
	}

	add(entry.Title)
	add(entry.CoreEvent)
	add(entry.Hook)
	for _, scene := range entry.Scenes {
		add(scene)
	}
	if plan != nil {
		add(plan.Goal)
		add(plan.Hook)
		for _, point := range plan.Contract.PayoffPoints {
			add(point)
		}
		add(plan.Contract.HookGoal)
	}
	return terms
}

func matchesRecallTerms(text string, terms []string) bool {
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if len([]rune(term)) < 2 {
			continue
		}
		if strings.Contains(text, term) || strings.Contains(term, text) {
			return true
		}
		if hasMeaningfulOverlap(term, text) {
			return true
		}
	}
	return false
}

func hasMeaningfulOverlap(a, b string) bool {
	ar := []rune(strings.TrimSpace(a))
	br := []rune(strings.TrimSpace(b))
	if len(ar) < 5 || len(br) < 5 {
		return false
	}
	shorter := len(ar)
	if len(br) < shorter {
		shorter = len(br)
	}
	threshold := 5
	switch {
	case shorter >= 12:
		threshold = 7
	case shorter >= 9:
		threshold = 6
	}
	return longestCommonSubstringRunes(ar, br) >= threshold
}

const storyThreadRecallThreshold = 6
const storyThreadRecallMinSelected = 2

// storyThreadDueSoonWindow：due_by 落在 [N, N+storyThreadDueSoonWindow] 的伏笔，
// 即使与本章关键词无关也提前提醒——等到过期的下一章才提，往往已经来不及铺。
const storyThreadDueSoonWindow = 2

func longestCommonSubstringRunes(a, b []rune) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	prev := make([]int, len(b)+1)
	best := 0
	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		for j := 1; j <= len(b); j++ {
			if a[i-1] != b[j-1] {
				continue
			}
			curr[j] = prev[j-1] + 1
			if curr[j] > best {
				best = curr[j]
			}
		}
		prev = curr
	}
	return best
}

// truncateRunes 截断字符串到指定 rune 数。
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
