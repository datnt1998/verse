package eval

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/stylestat"
)

// Collected 是一次运行产出的只读采集结果。全部来自已有评测器与事实层，eval 不自己重算。
type Collected struct {
	Dir         string
	Report      diag.Report // diag.Diagnose：工件 + 运行时 Findings + Stats
	Progress    *domain.Progress
	Checkpoints []domain.Checkpoint
	Pending     map[string]bool // 残留信号：pending_commit/pending_steer/last_commit/last_review
	LoadErrors  []string        // 契约依赖工件的真实读取失败（非"不存在"）；Grade 据此 hard fail
	RuntimeErr  string          // runner 捕获的运行时错误（hard fail），空=无
	Style       StyleCollection
	Usage       UsageMetrics
	ToolCalls   int

	// Reviews 是现行评审（reviews/*.json）；ReviewRounds 是被覆盖前的旧轮次
	// （reviews/rounds/*.json）。复审闭环要用后者比对同一 finding 是否修好。
	Reviews      []domain.ReviewEntry
	ReviewRounds []domain.ReviewEntry
	// ChapterTexts / ChapterFacts 是正文本体断言与事实断言的输入。
	ChapterTexts map[int]string
	ChapterFacts map[int]domain.ChapterFacts
	// WorkerRuns 是每个 agent 的 session 文件数（一次 worker 运行一个文件）；
	// ToolErrors 是 session 日志里标了 is_error 的 tool result 条数。
	WorkerRuns map[string]int
	ToolErrors int
	// ContextTrims 是 novel_context 结果里 _trimmed 命中 previous_tail 的次数：
	// 上下文被预算裁剪到丢掉上文尾部的次数，A/B previous_tail 需要它。
	// 只统计读到原文的结果；被压缩/出错的次数另记在 ContextMissing——
	// 找不到可测结果时不把 0 当成"没有裁剪"谎报。
	ContextTrims int
	// ContextMissing 是 novel_context 工具结果里读不到原文的次数
	// （旧工件被 [session_compact: ...] 压缩、或调用出错），这些结果对
	// context_contains 与 context_trims 都不可测。
	ContextMissing int
	// QuoteCoverage 是 writer 提交里 foreshadow update 与 state change 携带
	// quote 的比例；Total=0 表示本次运行没有可统计的提交项。
	// 只统计本次 run 中 commit_chapter 成功提交的章节——fixture 预置的历史
	// 章节事实与 editor-only run 不算 writer 的提交行为。
	QuoteCoverage RateMetric
	// ContextResults 是本次会话里所有 novel_context 工具结果的原文（JSON 文本）。
	// 被压缩成 [session_compact: ...] 占位符与出错的结果不入列：
	// context_contains 断言只读这些字节，不重新构造上下文。
	ContextResults []string
}

// StyleCollection 是从章节终稿中采集的全书文体事实。
type StyleCollection struct {
	Status string           `json:"status"` // ok / insufficient_sample
	Stats  *stylestat.Stats `json:"stats,omitempty"`
}

// UsageMetrics 是 meta/usage.json 中已有的可靠成本/token 事实。
type UsageMetrics struct {
	Input         int     `json:"input,omitempty"`
	Output        int     `json:"output,omitempty"`
	CacheRead     int     `json:"cache_read,omitempty"`
	CacheWrite    int     `json:"cache_write,omitempty"`
	CostUSD       float64 `json:"cost_usd,omitempty"`
	MissingUsage  int     `json:"missing_usage,omitempty"`
	UsageRecorded bool    `json:"usage_recorded"`
}

// Collect 对一个已完成的输出目录做离线采集。runtimeErr 是 runner 驱动期间的错误（如有）。
// 工件读取错误不静默吞：文件不存在视为"无数据"，其余（损坏/无权限）记入 LoadErrors，
// 避免"读不到 pending 文件"被误判成"没有 pending"而 false pass（fail-loud）。
func Collect(dir string, runtimeErr error) Collected {
	s := store.NewStore(dir)
	rep, _ := diag.Diagnose(s)

	var loadErrors []string
	check := func(name string, err error) {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", name, err))
		}
	}

	prog, err := s.Progress.Load()
	check("progress", err)
	cks := s.Checkpoints.All()

	pending := map[string]bool{}
	pc, err := s.Signals.LoadPendingCommit()
	check("pending_commit", err)
	if pc != nil {
		pending["pending_commit"] = true
	}
	lc, err := s.Signals.LoadLastCommit()
	check("last_commit", err)
	if lc != nil {
		pending["last_commit"] = true
	}
	lr, err := s.Signals.LoadLastReviewSignal()
	check("last_review", err)
	if lr != nil {
		pending["last_review"] = true
	}
	rm, err := s.RunMeta.Load()
	check("run_meta", err)
	if rm != nil && rm.PendingSteer != "" {
		pending["pending_steer"] = true
	}
	style := collectStyle(s, prog, check)
	usage := collectUsage(s, check)
	texts, facts := collectChapters(s, check)
	reviews, rounds := collectReviews(s, check)
	scan := scanSessionLogs(dir, check)
	// quote_coverage 只统计本次 run 内 writer 真正提交的章节——按 session 里
	// commit_chapter 成功结果识别，seed 的历史章节与 editor-only run 不计入。
	committedFacts := map[int]domain.ChapterFacts{}
	for chapter := range scan.committedChapters {
		if f, ok := facts[chapter]; ok {
			committedFacts[chapter] = f
		}
	}

	errStr := ""
	if runtimeErr != nil {
		errStr = runtimeErr.Error()
	}
	return Collected{
		Dir:            dir,
		Report:         rep,
		Progress:       prog,
		Checkpoints:    cks,
		Pending:        pending,
		LoadErrors:     loadErrors,
		RuntimeErr:     errStr,
		Style:          style,
		Usage:          usage,
		ToolCalls:      scan.toolCalls,
		ToolErrors:     scan.toolErrors,
		WorkerRuns:     scan.workerRuns,
		Reviews:        reviews,
		ReviewRounds:   rounds,
		ChapterTexts:   texts,
		ChapterFacts:   facts,
		ContextTrims:   scan.contextTrims,
		ContextMissing: scan.contextMissing,
		ContextResults: scan.contextResults,
		QuoteCoverage:  collectQuoteCoverage(committedFacts),
	}
}

func collectStyle(s *store.Store, prog *domain.Progress, check func(string, error)) StyleCollection {
	input := stylestat.Input{}
	if prog != nil {
		chapters := append([]int(nil), prog.CompletedChapters...)
		sort.Ints(chapters)
		for _, ch := range chapters {
			text, err := s.Drafts.LoadChapterText(ch)
			check(fmt.Sprintf("chapter:%d", ch), err)
			if strings.TrimSpace(text) == "" {
				check(fmt.Sprintf("chapter:%d", ch), fmt.Errorf("progress đánh dấu đã hoàn thành nhưng bản cuối rỗng"))
				continue
			}
			input.Chapters = append(input.Chapters, text)
			input.Titles = append(input.Titles, chapterTitle(s, ch, text, check))
		}
	}
	chars, err := s.Characters.Load()
	check("characters", err)
	for _, c := range chars {
		if c.Name != "" {
			input.Stopwords = append(input.Stopwords, c.Name)
		}
		input.Stopwords = append(input.Stopwords, c.Aliases...)
	}

	stats := stylestat.Compute(input)
	if stats == nil {
		return StyleCollection{Status: "insufficient_sample"}
	}
	return StyleCollection{Status: "ok", Stats: stats}
}

func chapterTitle(s *store.Store, chapter int, text string, check func(string, error)) string {
	summary, err := s.Summaries.LoadSummary(chapter)
	check(fmt.Sprintf("summary:%d", chapter), err)
	if summary != nil && strings.TrimSpace(summary.Title) != "" {
		return summary.Title
	}
	entries, err := s.Outline.LoadOutline()
	check("outline", err)
	for _, entry := range entries {
		if entry.Chapter == chapter && strings.TrimSpace(entry.Title) != "" {
			return entry.Title
		}
	}
	volumes, err := s.Outline.LoadLayeredOutline()
	check("layered_outline", err)
	for _, v := range volumes {
		for _, a := range v.Arcs {
			for _, entry := range a.Chapters {
				if entry.Chapter == chapter && strings.TrimSpace(entry.Title) != "" {
					return entry.Title
				}
			}
		}
	}
	return firstMarkdownTitle(text)
}

func firstMarkdownTitle(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.TrimSpace(strings.TrimLeft(line, "#"))
	}
	return ""
}

func collectUsage(s *store.Store, check func(string, error)) UsageMetrics {
	state, err := s.Usage.Load()
	check("usage", err)
	if state == nil {
		return UsageMetrics{}
	}
	return UsageMetrics{
		Input:         state.Overall.Input,
		Output:        state.Overall.Output,
		CacheRead:     state.Overall.CacheRead,
		CacheWrite:    state.Overall.CacheWrite,
		CostUSD:       state.Overall.Cost,
		MissingUsage:  state.MissingUsage,
		UsageRecorded: true,
	}
}

type sessionLine struct {
	llmtypes.Message
}

type sessionScan struct {
	toolCalls         int
	toolErrors        int
	contextTrims      int
	contextMissing    int
	contextResults    []string
	workerRuns        map[string]int
	commitPending     map[string]int // tool_call_id → 调参里的 chapter，等结果对账
	committedChapters map[int]bool   // 本次 run commit_chapter 成功提交的章节
}

// scanSessionLogs 一次遍历 meta/sessions：工具调用数、工具错误数、novel_context
// 结果原文、每个 agent 的运行文件数。session 是运行时自己写的追加日志，这里只读不动。
func scanSessionLogs(dir string, check func(string, error)) sessionScan {
	scan := sessionScan{workerRuns: map[string]int{}, commitPending: map[string]int{}, committedChapters: map[int]bool{}}
	sessionDir := filepath.Join(dir, "meta", "sessions")
	err := filepath.WalkDir(sessionDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		if agent, ok := agentFromSessionPath(sessionDir, path); ok {
			scan.workerRuns[agent]++
		}
		return scanSessionFile(path, &scan)
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		check("sessions", err)
	}
	return scan
}

// agentFromSessionPath 从 meta/sessions/agents/<agent>-<suffix>.jsonl 取出 agent 名。
// cocreate.jsonl 等非 agent 日志不参与 worker 计数。
func agentFromSessionPath(sessionDir, path string) (string, bool) {
	rel, err := filepath.Rel(filepath.Join(sessionDir, "agents"), path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	name := strings.TrimSuffix(rel, ".jsonl")
	if i := strings.IndexByte(name, '-'); i > 0 {
		name = name[:i]
	}
	if name == "" || strings.ContainsRune(name, filepath.Separator) {
		return "", false
	}
	return name, true
}

func scanSessionFile(path string, scan *sessionScan) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for sc.Scan() {
		var sl sessionLine
		if err := json.Unmarshal(sc.Bytes(), &sl); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		scan.toolCalls += len(sl.Message.ToolCalls())
		// commit_chapter 调用先记 tool_call_id → chapter，等工具结果对账：
		// 只有非错误返回才算本次 run 里 writer 真正提交的章节。
		for _, call := range sl.Message.ToolCalls() {
			if call.Name == "commit_chapter" {
				scan.commitPending[call.ID] = commitChapterFromArgs(call.Args)
			}
		}
		if sl.Message.Role != llmtypes.RoleTool {
			continue
		}
		if isErrorResult(sl.Message) {
			scan.toolErrors++
		}
		if name, _ := sl.Message.Metadata["tool_name"].(string); name == "commit_chapter" {
			callID, _ := sl.Message.Metadata["tool_call_id"].(string)
			scan.noteCommitResult(callID, sl.Message)
		}
		if previousTailTrimmed(sl.Message) {
			scan.contextTrims++
		}
		scan.contextResults = append(scan.contextResults, novelContextResults(sl.Message)...)
		if contextResultMissing(sl.Message) {
			scan.contextMissing++
		}
	}
	return sc.Err()
}

// commitChapterFromArgs 从 commit_chapter 调参里取章节号；取不到返回 0。
func commitChapterFromArgs(args json.RawMessage) int {
	var payload struct {
		Chapter int `json:"chapter"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return 0
	}
	return payload.Chapter
}

// noteCommitResult 把一条 commit_chapter 工具结果对账到已提交章节集合。
// 错误结果撤掉对应 pending；成功结果以返回 JSON 的 chapter 为准——恢复重放的
// commit 会按冻结载荷提交，调参里的章节号不可信；结果读不出（如被压缩）才退回调参。
func (s *sessionScan) noteCommitResult(callID string, msg llmtypes.Message) {
	argChapter, pending := s.commitPending[callID]
	delete(s.commitPending, callID)
	if isErrorResult(msg) {
		return
	}
	var result struct {
		Chapter   int  `json:"chapter"`
		Committed bool `json:"committed"`
	}
	parsed := false
	for _, b := range msg.Content {
		if b.Type != llmtypes.ContentText || store.IsCompacted(b.Text) {
			continue
		}
		if err := json.Unmarshal([]byte(b.Text), &result); err == nil {
			parsed = true
			break
		}
	}
	switch {
	case parsed && result.Committed && result.Chapter > 0:
		s.committedChapters[result.Chapter] = true
	case parsed:
		// 返回明确说没提交（committed=false），不计入。
	case pending && argChapter > 0:
		// 非错误结果但原文不可读（被压缩）：以调参章节为准。
		s.committedChapters[argChapter] = true
	}
}

// contextResultMissing 报告这条 novel_context 工具结果是否不可测——出错、
// 被压缩成占位符、或没有任何非空文本，context_contains 与 context_trims
// 都不能拿它当事实。
func contextResultMissing(msg llmtypes.Message) bool {
	if name, _ := msg.Metadata["tool_name"].(string); name != "novel_context" {
		return false
	}
	if isErrorResult(msg) {
		return true
	}
	for _, b := range msg.Content {
		if b.Type == llmtypes.ContentText && strings.TrimSpace(b.Text) != "" && !store.IsCompacted(b.Text) {
			return false
		}
	}
	return true
}

// novelContextResults 取出这条 tool 消息里 novel_context 的结果原文：context_contains
// 断言的就该是模型真正拿到的那几段字节。出错结果与 [session_compact: ...] 占位符
// 不是模型读到的内容，一律不入列（由 contextResultMissing 单独计数）。
func novelContextResults(msg llmtypes.Message) []string {
	if name, _ := msg.Metadata["tool_name"].(string); name != "novel_context" {
		return nil
	}
	if isErrorResult(msg) {
		return nil
	}
	var out []string
	for _, b := range msg.Content {
		if b.Type == llmtypes.ContentText && strings.TrimSpace(b.Text) != "" && !store.IsCompacted(b.Text) {
			out = append(out, b.Text)
		}
	}
	return out
}

// previousTailTrimmed 报告这条 tool 消息是不是把 previous_tail 裁掉的
// novel_context 结果。只解析 JSON，不重新评估裁剪语义。
func previousTailTrimmed(msg llmtypes.Message) bool {
	name, _ := msg.Metadata["tool_name"].(string)
	if name != "novel_context" {
		return false
	}
	var payload struct {
		Trimmed []string `json:"_trimmed"`
	}
	for _, b := range msg.Content {
		if b.Type != llmtypes.ContentText {
			continue
		}
		if err := json.Unmarshal([]byte(b.Text), &payload); err == nil && slices.Contains(payload.Trimmed, "previous_tail") {
			return true
		}
	}
	return false
}

// collectQuoteCoverage 统计已接纳章节事实里 foreshadow update 与 state change
// 携带 quote 的比例——writer 是否照约定附证据引文，quote 非必填时靠它度量。
// 没有任何提交项时 Total=0，报表渲染为 N/A。
func collectQuoteCoverage(facts map[int]domain.ChapterFacts) RateMetric {
	var m RateMetric
	for _, f := range facts {
		for _, u := range f.ForeshadowUpdates {
			m.Total++
			if strings.TrimSpace(u.Quote) != "" {
				m.Hit++
			}
		}
		for _, c := range f.StateChanges {
			m.Total++
			if strings.TrimSpace(c.Quote) != "" {
				m.Hit++
			}
		}
	}
	m.Rate = rateOf(m.Hit, m.Total)
	return m
}

func isErrorResult(msg llmtypes.Message) bool {
	if msg.Metadata == nil {
		return false
	}
	failed, _ := msg.Metadata["is_error"].(bool)
	return failed
}

// collectReviews 读现行评审与被覆盖的旧轮次。批量评测要判"这一轮到底存了什么"，
// 因此直接列 reviews 目录，而不是按某一章查找。
func collectReviews(s *store.Store, check func(string, error)) ([]domain.ReviewEntry, []domain.ReviewEntry) {
	reviews := loadReviewDir(filepath.Join(s.Dir(), "reviews"), check)
	rounds := loadReviewDir(filepath.Join(s.Dir(), "reviews", "rounds"), check)
	sortReviews(reviews)
	sortReviews(rounds)
	return reviews, rounds
}

func loadReviewDir(dir string, check func(string, error)) []domain.ReviewEntry {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		check(filepath.Base(dir), err)
		return nil
	}
	var out []domain.ReviewEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			check(filepath.Join(filepath.Base(dir), entry.Name()), err)
			continue
		}
		var review domain.ReviewEntry
		if err := json.Unmarshal(data, &review); err != nil {
			check(filepath.Join(filepath.Base(dir), entry.Name()), err)
			continue
		}
		out = append(out, review)
	}
	return out
}

// collectChapters 采集 chapters/NN.md 与对应的章节事实记录。按目录实际存在的
// 终稿采集，而不是只看 progress：Writer 写到一半崩溃时，正文断言仍应读到正文。
func collectChapters(s *store.Store, check func(string, error)) (map[int]string, map[int]domain.ChapterFacts) {
	texts := map[int]string{}
	facts := map[int]domain.ChapterFacts{}
	entries, err := os.ReadDir(filepath.Join(s.Dir(), "chapters"))
	if errors.Is(err, os.ErrNotExist) {
		return texts, facts
	}
	if err != nil {
		check("chapters", err)
		return texts, facts
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		chapter, ok := chapterFromFileName(entry.Name())
		if !ok {
			continue
		}
		text, err := s.Drafts.LoadChapterText(chapter)
		if err != nil {
			check(fmt.Sprintf("chapter:%d", chapter), err)
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		texts[chapter] = text
		record, err := s.ChapterRecords.Load(chapter)
		if err != nil {
			check(fmt.Sprintf("chapter_record:%d", chapter), err)
			continue
		}
		if record != nil {
			facts[chapter] = record.Facts
		}
	}
	return texts, facts
}

func chapterFromFileName(name string) (int, bool) {
	base := strings.TrimSuffix(name, ".md")
	n, err := strconv.Atoi(base)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// HasCheckpoint 判断采集到的 checkpoint 中是否存在匹配 spec 的记录。
// spec 形如 "chapter:1:commit" / "arc:1:1:arc_summary" / "volume:1:volume_summary" / "global:layered_outline"。
func (c Collected) HasCheckpoint(spec string) (bool, error) {
	scope, step, err := parseCheckpointSpec(spec)
	if err != nil {
		return false, err
	}
	for _, ck := range c.Checkpoints {
		if ck.Step == step && ck.Scope == scope {
			return true, nil
		}
	}
	return false, nil
}

// parseCheckpointSpec 把契约串解析成 (Scope, step)。
func parseCheckpointSpec(spec string) (domain.Scope, string, error) {
	parts := strings.Split(spec, ":")
	bad := func() (domain.Scope, string, error) {
		return domain.Scope{}, "", fmt.Errorf("hợp đồng checkpoint không hợp lệ: %q", spec)
	}
	if len(parts) < 2 {
		return bad()
	}
	kind := parts[0]
	switch domain.ScopeKind(kind) {
	case domain.ScopeChapter: // chapter:N:step
		if len(parts) != 3 {
			return bad()
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			return bad()
		}
		return domain.Scope{Kind: domain.ScopeChapter, Chapter: n}, parts[2], nil
	case domain.ScopeArc: // arc:V:A:step
		if len(parts) != 4 {
			return bad()
		}
		v, err1 := strconv.Atoi(parts[1])
		a, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil {
			return bad()
		}
		return domain.Scope{Kind: domain.ScopeArc, Volume: v, Arc: a}, parts[3], nil
	case domain.ScopeVolume: // volume:V:step
		if len(parts) != 3 {
			return bad()
		}
		v, err := strconv.Atoi(parts[1])
		if err != nil {
			return bad()
		}
		return domain.Scope{Kind: domain.ScopeVolume, Volume: v}, parts[2], nil
	case domain.ScopeGlobal: // global:step
		if len(parts) != 2 {
			return bad()
		}
		return domain.Scope{Kind: domain.ScopeGlobal}, parts[1], nil
	default:
		return bad()
	}
}
