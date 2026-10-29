package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

// WorldStore 管理时间线、伏笔、人物关系、状态变化、世界规则、风格规则、审阅和交接。
type fileWorldStore struct {
	io *IO

	timeline                *appendLog[domain.TimelineEvent]
	stateChanges            *appendLog[domain.StateChange]
	timelineProjectionReady bool
}

func newFileWorldStore(io *IO) *fileWorldStore {
	return &fileWorldStore{io: io,
		timeline: newAppendLog(
			"timeline.jsonl",
			"timeline.json",
			timelineEventKey,
			cloneTimelineEvent,
		),
		stateChanges: newAppendLog(
			"meta/state_changes.jsonl",
			"meta/state_changes.json",
			stateChangeKey,
			func(change domain.StateChange) domain.StateChange { return change },
		)}
}

// ── 时间线 ──

// SaveTimeline 全量替换时间线事实与人类可读投影。
func (s *fileWorldStore) SaveTimeline(events []domain.TimelineEvent) error {
	return s.io.WithWriteLock(func() error {
		if err := s.timeline.replaceUnlocked(s.io, events); err != nil {
			s.timelineProjectionReady = false
			return err
		}
		if err := s.io.WriteMarkdownUnlocked("timeline.md", renderTimeline(events, s.io.labels())); err != nil {
			s.timelineProjectionReady = false
			return err
		}
		s.timelineProjectionReady = true
		return nil
	})
}

// LoadTimeline 读取时间线。
func (s *fileWorldStore) LoadTimeline() (_ []domain.TimelineEvent, err error) {
	defer s.io.lockWrite()(&err)
	return s.timeline.allUnlocked(s.io)
}

// AppendTimelineEvents 追加时间线事件。同一事件重复提交时按稳定 key 去重，保证
// commit_chapter 崩溃后重跑不会污染时间线。
func (s *fileWorldStore) AppendTimelineEvents(newEvents []domain.TimelineEvent) error {
	return s.io.WithWriteLock(func() error {
		if !s.timelineProjectionReady {
			existing, err := s.timeline.allUnlocked(s.io)
			if err != nil {
				return err
			}
			if err := s.ensureTimelineProjectionUnlocked(existing); err != nil {
				return err
			}
		}

		added, err := s.timeline.appendUnlocked(s.io, newEvents)
		if err != nil {
			// 追加错误时磁盘上可能已有部分完整记录，重放前必须
			// 从事实日志重建投影，不能仅依赖 added 的返回值。
			s.timelineProjectionReady = false
			return err
		}
		if len(added) == 0 {
			return nil
		}
		if err := s.io.AppendLineUnlocked("timeline.md", []byte(renderTimelineEntries(added, s.io.labels()))); err != nil {
			s.timelineProjectionReady = false
			return err
		}
		return nil
	})
}

// ensureTimelineProjectionUnlocked 在进程首次追加或上次投影失败后核对 timeline.md。
// 正常路径只执行一次全量核对，之后每章与 JSONL 同步追加；投影不参与事实读取。
func (s *fileWorldStore) ensureTimelineProjectionUnlocked(events []domain.TimelineEvent) error {
	if s.timelineProjectionReady {
		return nil
	}
	expected := renderTimeline(events, s.io.labels())
	actual, err := s.io.ReadFileUnlocked("timeline.md")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if string(actual) != expected {
		if err := s.io.WriteMarkdownUnlocked("timeline.md", expected); err != nil {
			return err
		}
	}
	s.timelineProjectionReady = true
	return nil
}

// LoadRecentTimeline 返回最近 window 章内的时间线事件。
func (s *fileWorldStore) LoadRecentTimeline(current, window int) ([]domain.TimelineEvent, error) {
	all, err := s.LoadTimeline()
	if err != nil {
		return nil, err
	}
	minCh := max(current-window, 1)
	var filtered []domain.TimelineEvent
	for _, e := range all {
		if e.Chapter >= minCh {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// ── 伏笔 ──

// SaveForeshadowLedger 全量写入 foreshadow_ledger.json + foreshadow_ledger.md（原子写入）。
func (s *fileWorldStore) SaveForeshadowLedger(entries []domain.ForeshadowEntry) error {
	return s.io.WithWriteLock(func() error {
		if err := s.io.WriteJSONUnlocked("foreshadow_ledger.json", entries); err != nil {
			return err
		}
		return s.io.WriteMarkdownUnlocked("foreshadow_ledger.md", renderForeshadow(entries, s.io.labels()))
	})
}

// LoadForeshadowLedger 读取伏笔账本。
func (s *fileWorldStore) LoadForeshadowLedger() ([]domain.ForeshadowEntry, error) {
	var entries []domain.ForeshadowEntry
	if err := s.io.ReadJSON("foreshadow_ledger.json", &entries); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return entries, nil
}

// UpdateForeshadow 批量应用伏笔增量操作。状态机在 domain.ApplyForeshadow：
// 章节记录全量重放走同一函数，两条路必须产出同一份账本。
func (s *fileWorldStore) UpdateForeshadow(chapter int, updates []domain.ForeshadowUpdate) error {
	return s.io.WithWriteLock(func() error {
		var entries []domain.ForeshadowEntry
		if err := s.io.ReadJSONUnlocked("foreshadow_ledger.json", &entries); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		}
		idx := make(map[string]int, len(entries))
		for i, e := range entries {
			idx[e.ID] = i
		}
		for _, u := range updates {
			var err error
			if entries, err = domain.ApplyForeshadow(entries, idx, chapter, u); err != nil {
				return err
			}
		}
		if err := s.io.WriteJSONUnlocked("foreshadow_ledger.json", entries); err != nil {
			return err
		}
		return s.io.WriteMarkdownUnlocked("foreshadow_ledger.md", renderForeshadow(entries, s.io.labels()))
	})
}

// LoadActiveForeshadow 返回未收束的伏笔条目（resolved 与 abandoned 都算收束）。
func (s *fileWorldStore) LoadActiveForeshadow() ([]domain.ForeshadowEntry, error) {
	all, err := s.LoadForeshadowLedger()
	if err != nil {
		return nil, err
	}
	var active []domain.ForeshadowEntry
	for _, e := range all {
		if domain.ForeshadowOpen(e.Status) {
			active = append(active, e)
		}
	}
	return active, nil
}

// ── 人物关系 ──

// SaveRelationships 追加重建后的差异；历史只追加，不因修订删除旧事实。
func (s *fileWorldStore) SaveRelationships(entries []domain.RelationshipEntry) error {
	return s.updateRelationships(entries, true)
}

// LoadRelationships 读取人物关系状态。
func (s *fileWorldStore) LoadRelationships() ([]domain.RelationshipEntry, error) {
	var entries []domain.RelationshipEntry
	if err := s.io.ReadJSON("relationship_state.json", &entries); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return entries, nil
}

// UpdateRelationships 追加关系变化，再从历史重建当前无向关系。
func (s *fileWorldStore) UpdateRelationships(changes []domain.RelationshipEntry) error {
	return s.updateRelationships(changes, false)
}

// ── 状态变化 ──

// AppendStateChanges 追加角色状态变化。同一状态变化重复提交时按稳定 key 去重。
func (s *fileWorldStore) AppendStateChanges(changes []domain.StateChange) error {
	return s.io.WithWriteLock(func() error {
		_, err := s.stateChanges.appendUnlocked(s.io, changes)
		return err
	})
}

// LoadStateChanges 读取全部状态变化记录。
func (s *fileWorldStore) LoadStateChanges() (_ []domain.StateChange, err error) {
	defer s.io.lockWrite()(&err)
	return s.stateChanges.allUnlocked(s.io)
}

// SaveStateChanges 全量替换状态变化事实，供章节修订后重建投影。
func (s *fileWorldStore) SaveStateChanges(changes []domain.StateChange) error {
	return s.io.WithWriteLock(func() error {
		return s.stateChanges.replaceUnlocked(s.io, changes)
	})
}

// ── 世界规则 ──

// SaveWorldRules 全量写入 world_rules.json + world_rules.md（原子写入）。
func (s *fileWorldStore) SaveWorldRules(rules []domain.WorldRule) error {
	return s.io.WithWriteLock(func() error {
		if err := s.io.WriteJSONUnlocked("world_rules.json", rules); err != nil {
			return err
		}
		return s.io.WriteMarkdownUnlocked("world_rules.md", renderWorldRules(rules, s.io.labels()))
	})
}

// LoadWorldRules 读取世界规则。
func (s *fileWorldStore) LoadWorldRules() ([]domain.WorldRule, error) {
	var rules []domain.WorldRule
	if err := s.io.ReadJSON("world_rules.json", &rules); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return rules, nil
}

// ── 风格规则 ──

// SaveStyleRules 保存写作风格规则。
func (s *fileWorldStore) SaveStyleRules(rules domain.WritingStyleRules) error {
	return s.io.WriteJSON("meta/style_rules.json", rules)
}

// LoadStyleRules 读取写作风格规则。
func (s *fileWorldStore) LoadStyleRules() (*domain.WritingStyleRules, error) {
	var rules domain.WritingStyleRules
	if err := s.io.ReadJSON("meta/style_rules.json", &rules); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &rules, nil
}

func (s *fileWorldStore) SaveAuthorRevisionStyle(style domain.AuthorRevisionStyle) error {
	return s.io.WriteJSON("meta/author_revision_style.json", style)
}

func (s *fileWorldStore) LoadAuthorRevisionStyle() (*domain.AuthorRevisionStyle, error) {
	var style domain.AuthorRevisionStyle
	if err := s.io.ReadJSON("meta/author_revision_style.json", &style); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &style, nil
}

// ── 审阅 ──

// reviewRoundsDir 存放被新一轮覆盖之前的旧评审：reviews/rounds/NN[-global]-r<K>.json。
// 文件内容与 reviews/ 下的原件逐字节相同，因此不改变任何既有文件格式。
const reviewRoundsDir = "reviews/rounds"

// ReviewScope 是评审所在的那一份文件。引擎把 scope=chapter 与 scope=arc 写进
// reviews/NN.json，把 scope=global 写进 reviews/NN-global.json，所以一章最多两份
// 评审。读写都用这个类型说明「哪一份」，不再用 bool 或 nil 表达同一件事。
type ReviewScope string

const (
	ReviewScopeChapter ReviewScope = "chapter"
	ReviewScopeArc     ReviewScope = "arc"
	ReviewScopeGlobal  ReviewScope = "global"
)

// ReviewScopeOf 把已存评审的 scope 字段归一成文件位置：只有 global 是总评审，
// arc 与 chapter 共用章节文件（与 save_review 的 artifact 一致）。
func ReviewScopeOf(scope string) ReviewScope {
	if strings.TrimSpace(scope) == string(ReviewScopeGlobal) {
		return ReviewScopeGlobal
	}
	return ReviewScopeChapter
}

// FileName 是这一份评审的文件名："03.json" / "03-global.json"。
func (s ReviewScope) FileName(chapter int) string {
	if s == ReviewScopeGlobal {
		return fmt.Sprintf("%02d-global.json", chapter)
	}
	return fmt.Sprintf("%02d.json", chapter)
}

// Artifact 是这一份评审在 store 内的相对路径："reviews/03.json"。
func (s ReviewScope) Artifact(chapter int) string {
	return "reviews/" + s.FileName(chapter)
}

// archivePrefix 是存档名前缀："03-r" / "03-global-r"。
func (s ReviewScope) archivePrefix(chapter int) string {
	return strings.TrimSuffix(s.FileName(chapter), ".json") + "-r"
}

// ArchiveArtifact 是第 round 轮（1-based）旧评审的存档路径。
func (s ReviewScope) ArchiveArtifact(chapter, round int) string {
	return fmt.Sprintf("%s/%s%d.json", reviewRoundsDir, s.archivePrefix(chapter), round)
}

// SaveReview 保存审阅结果。若同一文件已有内容不同的旧评审（Biên tập viên chấm
// lại cùng một chương），先把旧内容原样存档到 reviews/rounds/ 再覆盖，让 round 1
// 的 mục còn cho dải xanh ở cửa duyệt / ma trận chất lượng.
func (s *fileWorldStore) SaveReview(r domain.ReviewEntry) error {
	scope := ReviewScopeOf(r.Scope)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return s.io.WithWriteLock(func() error {
		if err := s.archiveReviewRoundUnlocked(scope, r.Chapter, data); err != nil {
			return err
		}
		return s.io.WriteFileUnlocked(scope.Artifact(r.Chapter), data)
	})
}

// archiveReviewRoundUnlocked 在覆盖前把旧文件原样复制到下一档；内容相同说明
// 还是同一次 chấm，不存档。调用方须持有 io.mu 写锁。
func (s *fileWorldStore) archiveReviewRoundUnlocked(scope ReviewScope, chapter int, next []byte) error {
	prev, err := s.io.ReadFileUnlocked(scope.Artifact(chapter))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if bytes.Equal(prev, next) {
		return nil
	}
	archives, err := s.reviewArchivesUnlocked(scope, chapter)
	if err != nil {
		return err
	}
	return s.io.WriteFileUnlocked(scope.ArchiveArtifact(chapter, len(archives)+1), prev)
}

// reviewArchivesUnlocked 列出某一份评审已存的轮次：reviews/rounds/<base>-r<K>.json。
func (s *fileWorldStore) reviewArchivesUnlocked(scope ReviewScope, chapter int) (map[int]string, error) {
	entries, err := s.io.ListDirUnlocked(reviewRoundsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	prefix := scope.archivePrefix(chapter)
	byRound := map[int]string{}
	for _, e := range entries {
		name := e.Name
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		if k, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".json")); err == nil {
			byRound[k] = name
		}
	}
	return byRound, nil
}

// LoadReviewRounds 读取某一份评审被覆盖过的各轮旧内容，cũ nhất trước (r1, r2, …).
// 现行文件不在其中——它由 LoadReview/LoadGlobalReview 读取。
func (s *fileWorldStore) LoadReviewRounds(chapter int, scope ReviewScope) ([]domain.ReviewEntry, error) {
	byRound, err := s.reviewArchivesUnlocked(scope, chapter)
	if err != nil {
		return nil, err
	}
	if len(byRound) == 0 {
		return nil, nil
	}
	keys := make([]int, 0, len(byRound))
	for k := range byRound {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := make([]domain.ReviewEntry, 0, len(keys))
	for _, k := range keys {
		var rv domain.ReviewEntry
		if err := s.io.ReadJSON(reviewRoundsDir+"/"+byRound[k], &rv); err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, nil
}

// HasArcReview 检查指定章节（弧末章）是否已保存 scope=arc 的评审。
func (s *fileWorldStore) HasArcReview(chapter int) (bool, error) {
	rv, err := s.LoadReview(chapter)
	if err != nil {
		return false, err
	}
	return rv != nil && rv.Scope == "arc", nil
}

// HasGlobalReview 检查指定章节是否已保存 scope=global 的全局审阅
// (save_review 落盘为 reviews/%02d-global.json;非分层书按 ReviewInterval 触发)。
func (s *fileWorldStore) HasGlobalReview(chapter int) (bool, error) {
	r, err := s.LoadGlobalReview(chapter)
	if err != nil {
		return false, err
	}
	return r != nil && r.Scope == "global", nil
}

// loadReview 读取某一份评审的现行文件；文件不存在返回 nil。
func (s *fileWorldStore) loadReview(scope ReviewScope, chapter int) (*domain.ReviewEntry, error) {
	var r domain.ReviewEntry
	if err := s.io.ReadJSON(scope.Artifact(chapter), &r); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// LoadGlobalReview 读取指定截止章节的全局审阅。
func (s *fileWorldStore) LoadGlobalReview(chapter int) (*domain.ReviewEntry, error) {
	return s.loadReview(ReviewScopeGlobal, chapter)
}

// LoadReview 读取章节审阅结果。
func (s *fileWorldStore) LoadReview(chapter int) (*domain.ReviewEntry, error) {
	return s.loadReview(ReviewScopeChapter, chapter)
}

// LoadLastReview 读取最近一次全局审阅。
//
// Quét lùi từng chương tốn O(fromChapter) lần đọc tệp, nên trước tiên hạ trần
// xuống chương lớn nhất thực có tệp global review. Không có trần này, một tham
// số chương rác (2147483647) biến lời gọi thành vòng lặp hàng tỷ lần đọc và
// treo agent — production 2026-10-26 22:31, phiên kiến trúc sư mở rộng cung.
func (s *fileWorldStore) LoadLastReview(fromChapter int) (*domain.ReviewEntry, error) {
	if top, err := s.maxGlobalReviewChapter(); err == nil && top > 0 && top < fromChapter {
		fromChapter = top
	}
	for ch := fromChapter; ch >= 1; ch-- {
		r, err := s.loadReview(ReviewScopeGlobal, ch)
		if err != nil {
			return nil, err
		}
		if r != nil {
			return r, nil
		}
	}
	return nil, nil
}

// maxGlobalReviewChapter trả số chương lớn nhất đã có reviews/NN-global.json.
func (s *fileWorldStore) maxGlobalReviewChapter() (int, error) {
	entries, err := s.io.ListDirUnlocked("reviews")
	if err != nil {
		return 0, err
	}
	top := 0
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		name := entry.Name
		chapter, ok := strings.CutSuffix(name, "-global.json")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(chapter)
		if err != nil || n <= top {
			continue
		}
		top = n
	}
	return top, nil
}

// LoadReviewsAffectingChapter 返回所有明确把 chapter 纳入返工队列的评审，
// 新到旧排列。弧/全局评审存放在评审终点，不能再按目标章节文件名查找。
func (s *fileWorldStore) LoadReviewsAffectingChapter(chapter int) ([]domain.ReviewEntry, error) {
	entries, err := s.io.ListDirUnlocked("reviews")
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var reviews []domain.ReviewEntry
	for _, entry := range entries {
		if entry.IsDir || !strings.HasSuffix(entry.Name, ".json") {
			continue
		}
		data, err := s.io.ReadFileUnlocked("reviews/" + entry.Name)
		if err != nil {
			return nil, err
		}
		var review domain.ReviewEntry
		if err := json.Unmarshal(data, &review); err != nil {
			return nil, fmt.Errorf("parse reviews/%s: %w", entry.Name, err)
		}
		if slices.Contains(review.AffectedChapters, chapter) ||
			(review.Scope == "chapter" && review.Chapter == chapter && review.Verdict != "accept" && len(review.AffectedChapters) == 0) {
			reviews = append(reviews, review)
		}
	}
	slices.SortFunc(reviews, func(a, b domain.ReviewEntry) int {
		return b.Chapter - a.Chapter
	})
	return reviews, nil
}

// ── render helpers ──

func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

func timelineEventKey(e domain.TimelineEvent) string {
	chars := append([]string(nil), e.Characters...)
	slices.Sort(chars)
	parts := make([]string, 0, len(chars)+2)
	parts = append(parts, e.Time, e.Event)
	parts = append(parts, chars...)
	return stableRecordKey(e.Chapter, parts...)
}

func cloneTimelineEvent(event domain.TimelineEvent) domain.TimelineEvent {
	event.Characters = append([]string(nil), event.Characters...)
	return event
}

func stateChangeKey(c domain.StateChange) string {
	return stableRecordKey(c.Chapter, c.Entity, c.Field, c.OldValue, c.NewValue)
}

// stableRecordKey 使用长度前缀编码可变文本，避免内容中的分隔符导致去重碰撞。
func stableRecordKey(chapter int, parts ...string) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(chapter))
	for _, part := range parts {
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(len(part)))
		b.WriteByte(':')
		b.WriteString(part)
	}
	return b.String()
}

func renderTimeline(events []domain.TimelineEvent, l mdLabels) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", l.timeline)
	b.WriteString(renderTimelineEntries(events, l))
	return b.String()
}

func renderTimelineEntries(events []domain.TimelineEvent, l mdLabels) string {
	var b strings.Builder
	for _, e := range events {
		chars := ""
		if len(e.Characters) > 0 {
			chars = l.openParen + strings.Join(e.Characters, l.listSep) + l.closeParen
		}
		fmt.Fprintf(&b, "- **"+l.chapterFmt+" [%s]**%s%s%s\n", e.Chapter, e.Time, l.colon, e.Event, chars)
	}
	return b.String()
}

func renderForeshadow(entries []domain.ForeshadowEntry, l mdLabels) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", l.foreshadow)
	for _, e := range entries {
		status := e.Status
		if e.ResolvedAt > 0 {
			status = fmt.Sprintf(l.resolvedAtFmt, e.ResolvedAt)
		}
		fmt.Fprintf(&b, "- **[%s]** %s — "+l.plantedAtFmt+"\n",
			e.ID, e.Description, e.PlantedAt, status)
	}
	return b.String()
}

func renderRelationships(entries []domain.RelationshipEntry, l mdLabels) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", l.relationships)
	for _, e := range entries {
		fmt.Fprintf(&b, "- **%s ↔ %s**%s%s"+l.atChapterFmt+"\n",
			e.CharacterA, e.CharacterB, l.colon, e.Relation, e.Chapter)
	}
	return b.String()
}

func renderWorldRules(rules []domain.WorldRule, l mdLabels) string {
	grouped := make(map[string][]domain.WorldRule)
	var order []string
	for _, r := range rules {
		cat := r.Category
		if cat == "" {
			cat = "other"
		}
		if _, exists := grouped[cat]; !exists {
			order = append(order, cat)
		}
		grouped[cat] = append(grouped[cat], r)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", l.worldRules)
	for _, cat := range order {
		fmt.Fprintf(&b, "## %s\n\n", cat)
		for _, r := range grouped[cat] {
			fmt.Fprintf(&b, "- **%s**%s%s\n", l.rule, l.colon, r.Rule)
			if r.Boundary != "" {
				fmt.Fprintf(&b, "  - %s%s%s\n", l.boundary, l.colon, r.Boundary)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ── 章节机械违规事实 ──
//
// commit_chapter 的 rule_violations(user_rules 机械检查的 warning 级结果)持久化,
// editor 评审该章时经 novel_context(chapter=N) 读取并映射进七维评审
// (editor.md §机械检查映射)。writer 返工该章时同样可见。追加式,同章最新一条为准。

// ChapterViolations 一章的机械违规记录。
type ChapterViolations struct {
	Chapter    int               `json:"chapter"`
	Violations []rules.Violation `json:"violations"`
	At         string            `json:"at"`
}

const ruleViolationsFile = "meta/rule_violations.jsonl"

// SaveRuleViolations 追加一章的机械违规(空列表也追加——覆盖旧记录,表示重写后已清)。
func (s *fileWorldStore) SaveRuleViolations(chapter int, violations []rules.Violation) error {
	rec := ChapterViolations{Chapter: chapter, Violations: violations, At: time.Now().Format(time.RFC3339)}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.io.AppendLine(ruleViolationsFile, append(data, '\n'))
}

// LoadRuleViolations 读取某章最新一条机械违规记录;无记录返回 nil。
func (s *fileWorldStore) LoadRuleViolations(chapter int) []rules.Violation {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	rec, ok := s.ruleViolationsByChapterUnlocked()[chapter]
	if !ok {
		return nil
	}
	return rec.Violations
}

// RuleViolationsByChapter 一次读到全量机械违规记录（每章最新一条）。给一个请求
// 内要多章查询的调用方用：review view 读 xưng hô cards 时不再每章重读整份 jsonl。
func (s *fileWorldStore) RuleViolationsByChapter() map[int][]rules.Violation {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	records := s.ruleViolationsByChapterUnlocked()
	if len(records) == 0 {
		return nil
	}
	out := make(map[int][]rules.Violation, len(records))
	for chapter, rec := range records {
		out[chapter] = rec.Violations
	}
	return out
}

// ruleViolationsByChapterUnlocked 解析 meta/rule_violations.jsonl，每章留最新一条。
// 调用方须持有 io.mu 读锁。
func (s *fileWorldStore) ruleViolationsByChapterUnlocked() map[int]ChapterViolations {
	data, err := s.io.ReadFileUnlocked(ruleViolationsFile)
	if err != nil {
		return nil
	}
	latest := map[int]ChapterViolations{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec ChapterViolations
		if json.Unmarshal([]byte(line), &rec) == nil && rec.Chapter > 0 {
			latest[rec.Chapter] = rec
		}
	}
	return latest
}

// RecentRuleViolations 返回最近 last 个有机械检查记录的章节（章号从大到小），每章取最新一条。
// 空违规的记录也算（表示该章检查过且干净）。last<=0 时返回 nil。
func (s *fileWorldStore) RecentRuleViolations(last int) []ChapterViolations {
	if last <= 0 {
		return nil
	}
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	latest := s.ruleViolationsByChapterUnlocked()
	out := make([]ChapterViolations, 0, len(latest))
	for _, rec := range latest {
		out = append(out, rec)
	}
	slices.SortFunc(out, func(a, b ChapterViolations) int { return b.Chapter - a.Chapter })
	if len(out) > last {
		out = out[:last]
	}
	return out
}
