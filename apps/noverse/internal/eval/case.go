// Package eval 是 ainovel-cli 的离线评测 harness。
//
// 设计立足点：评测器（确定性诊断 diag、全书文体 stylestat、七维 rubric）项目里已经
// 存在，eval 只做薄薄一层——批量驱动 case、采集产出、把 diag Finding 与 case 契约映射
// 成门禁、聚合报告。一份事实定义，不在评测层重写一遍判断。详见 docs/evaluation-system.md。
//
// 当前已覆盖确定性主线：单路门禁、baseline/variant A/B delta、repeat 聚合与 stylestat 回归。
// LLM Judge 仍是可选后续层，不能污染确定性门禁。
package eval

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// caseIDPattern 限制 case id 为安全字符：id 会拼进输出目录并被 RunCase 的 RemoveAll 清理，
// 禁止 . / 等路径字符，杜绝 "../" 路径穿越删到工作区外。
var caseIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// fixtureNamePattern 限制 fixture 名：只允许安全字符，名字会拼进 evals/fixtures/<name>
// 的路径，禁止 . / 等路径字符与 "../" 穿越。
var fixtureNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

const (
	// defaultDeltaRatio 是 A/B 成本与工具调用增幅的默认阈值。
	defaultDeltaRatio = 0.3
	// maxCaseRegexes / maxRegexBytes 限制 case 里的正则规模：正则只用于断言个别
	// 词法标记，超过这个量说明该写进 fixture 或 diag 规则，而不是堆在 case 里。
	maxCaseRegexes = 20
	maxRegexBytes  = 200
	// defaultExpectSeverity 是 must_find / must_not_find 未写 severity_min 时的口径：
	// 只把 error 以上的问题算数，warning 是建议不是缺陷。
	defaultExpectSeverity = "error"
)

// Case 是一个评测样本：一段创作需求 + 一组事实层断言。
type Case struct {
	ID            string       `json:"id"`
	Category      string       `json:"category"`       // 评测层：smoke/workflow/quality/longform/recovery/steering
	Role          string       `json:"role,omitempty"` // 被测角色：writer/architect/editor（与 Category 正交）
	Description   string       `json:"description,omitempty"`
	Prompt        string       `json:"prompt"`                   // 用户创作需求
	Fixture       string       `json:"fixture,omitempty"`        // evals/fixtures/<name>，只填目录名
	Instruction   *Instruction `json:"instruction,omitempty"`    // fixture 模式下要跑的单个 instruction
	Control       bool         `json:"control,omitempty"`        // 对照 case：正确答案是不报问题
	Style         string       `json:"style,omitempty"`          // 覆盖配置风格
	MaxChapters   int          `json:"max_chapters"`             // 章数上限；0 表示只跑到规划完成（进入 writing）
	TargetPrompts []string     `json:"target_prompts,omitempty"` // 本 case 主要验证的 prompt 文件（信息性）
	Rubric        string       `json:"rubric,omitempty"`         // LLM Judge 评分表（Phase 3 启用）
	Expect        Expect       `json:"expect"`
	Gate          Gate         `json:"gate"`
}

// Instruction 是 fixture 模式下唯一要跑的 Worker 任务。task 要照抄 Router 给该
// Worker 的原话，评测的是同一段指令在固定书稿上的表现。
type Instruction struct {
	Agent string `json:"agent"` // editor / writer
	Task  string `json:"task"`
}

// FixtureMode 判断本 case 是否走"固定书稿 + 单个 instruction"路径。
func (c Case) FixtureMode() bool { return strings.TrimSpace(c.Fixture) != "" }

// Expect 是 case 级契约断言——只声明 diag 通用规则覆盖不到、与本 case 强相关的预期。
type Expect struct {
	Phase                string        `json:"phase,omitempty"`                  // 期望最终 phase
	MinCompletedChapters int           `json:"min_completed_chapters,omitempty"` // 至少完成的章数
	RequiredCheckpoints  []string      `json:"required_checkpoints,omitempty"`   // 形如 "chapter:1:commit" / "arc:1:1:arc_summary" / "global:layered_outline"
	NoPending            []string      `json:"no_pending,omitempty"`             // 结束时应清空的信号：pending_commit/pending_steer/last_commit/last_review
	Review               *ReviewExpect `json:"review,omitempty"`                 // 评审断言
	Prose                *ProseExpect  `json:"prose,omitempty"`                  // 正文本体断言
	Facts                *FactsExpect  `json:"facts,omitempty"`                  // 章节事实断言
	// ContextContains 断言本次运行的 novel_context 工具结果：每个串都必须出现在
	// 某一次调用的结果里。用来钉住"上下文到底把什么递给了模型"这种机械事实。
	ContextContains []string `json:"context_contains,omitempty"`
}

// ReviewExpect 是对 Editor 保存的 ReviewEntry 的断言。
type ReviewExpect struct {
	VerdictIn   []string            `json:"verdict_in,omitempty"`    // 最近一次评审的 verdict 必须落在这个集合里
	MustFind    []MustFindExpect    `json:"must_find,omitempty"`     // 必须被抓到的缺陷
	MustNotFind []MustNotFindExpect `json:"must_not_find,omitempty"` // 不该出现的误报
	// GoalStatus uses checklist IDs; RecheckStatus uses stable finding IDs or source quotes.
	GoalStatus    map[string]string `json:"goal_status,omitempty"`
	RecheckStatus map[string]string `json:"recheck_status,omitempty"`
}

// MustFindExpect 声明一条"必须抓到"的缺陷。空字段不参与匹配。
type MustFindExpect struct {
	SeverityMin   string `json:"severity_min,omitempty"`   // 抓到的 issue 至少要到这个严重度（默认 error）
	RuleRef       string `json:"rule_ref,omitempty"`       // 精确匹配 issue.rule_ref
	Rule          string `json:"rule,omitempty"`           // 精确匹配 issue.rule
	Chapter       int    `json:"chapter,omitempty"`        // chapter>0 时要求 issue 落在这章
	QuoteContains string `json:"quote_contains,omitempty"` // 证据/引文/描述里出现这段文字
}

// MustNotFindExpect 声明一种"不该出现"的误报。
type MustNotFindExpect struct {
	SeverityMin   string `json:"severity_min,omitempty"`   // 达到该严重度才算误报（默认 error）
	QuoteContains string `json:"quote_contains,omitempty"` // 引文/证据/描述命中即算误报
	TypeContains  string `json:"type_contains,omitempty"`  // issue.type 命中即算误报
}

// ProseExpect 断言某一章正文。Regex 只用于 unique 词法标记（地名、称呼），
// 不用于判断文笔。
type ProseExpect struct {
	Chapter                int      `json:"chapter"`
	ForbidRegex            []string `json:"forbid_regex,omitempty"`
	RequireRegex           []string `json:"require_regex,omitempty"`
	MaxOpeningOverlapRunes int      `json:"max_opening_overlap_runes,omitempty"` // 与上一章结尾的最大重复 rune 数
}

// FactsExpect 断言某一章落盘的结构化事实（ChapterFacts）。
type FactsExpect struct {
	Chapter        int      `json:"chapter"`
	ForbidContains []string `json:"forbid_contains,omitempty"`
}

// Gate 是本 case 的门禁阈值。本期只用 MaxSeverity；其余字段为 A/B（regression）阶段预留，
// 解析但不参与门禁——保留是为了 case 文件能按 docs/evaluation-system.md 的完整 schema 书写。
type Gate struct {
	MaxSeverity string `json:"max_severity,omitempty"` // diag Finding 允许的最高严重度（默认 warning）：超过即 hard fail

	MaxCostDeltaRatio     *float64 `json:"max_cost_delta_ratio,omitempty"`
	MaxToolCallDeltaRatio *float64 `json:"max_tool_call_delta_ratio,omitempty"`
	StylestatRegression   string   `json:"stylestat_regression,omitempty"`
}

// Validate 校验 case 必填字段。
func (c *Case) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("case thiếu id")
	}
	if !caseIDPattern.MatchString(c.ID) {
		return fmt.Errorf("case id không hợp lệ %q: chỉ cho phép chữ thường/số/gạch dưới/gạch nối, không chứa ký tự đường dẫn", c.ID)
	}
	if c.FixtureMode() {
		if !fixtureNamePattern.MatchString(c.Fixture) {
			return fmt.Errorf("fixture của case %q không hợp lệ %q: chỉ cho phép chữ thường/số/gạch nối, không chứa đường dẫn", c.ID, c.Fixture)
		}
		if c.Instruction == nil {
			return fmt.Errorf("case %q có fixture nên cần instruction", c.ID)
		}
		if err := c.Instruction.validate(); err != nil {
			return fmt.Errorf("case %q: %w", c.ID, err)
		}
	} else {
		if strings.TrimSpace(c.Prompt) == "" {
			return fmt.Errorf("case %q thiếu prompt", c.ID)
		}
		if c.Instruction != nil {
			return fmt.Errorf("case %q có instruction nhưng không có fixture: instruction chỉ dùng ở fixture mode", c.ID)
		}
	}
	if c.Gate.MaxSeverity == "" {
		c.Gate.MaxSeverity = "warning"
	}
	if !validSeverity(c.Gate.MaxSeverity) {
		return fmt.Errorf("gate.max_severity của case %q không hợp lệ: %s", c.ID, c.Gate.MaxSeverity)
	}
	if c.Gate.MaxCostDeltaRatio == nil {
		c.Gate.MaxCostDeltaRatio = float64Ptr(defaultDeltaRatio)
	}
	if c.Gate.MaxToolCallDeltaRatio == nil {
		c.Gate.MaxToolCallDeltaRatio = float64Ptr(defaultDeltaRatio)
	}
	if c.Gate.StylestatRegression == "" {
		c.Gate.StylestatRegression = "warn"
	}
	if !validStylestatGate(c.Gate.StylestatRegression) {
		return fmt.Errorf("gate.stylestat_regression của case %q không hợp lệ: %s", c.ID, c.Gate.StylestatRegression)
	}
	if err := c.Expect.validate(c.ID); err != nil {
		return err
	}
	return nil
}

func (in *Instruction) validate() error {
	switch in.Agent {
	case "editor", "writer":
	default:
		return fmt.Errorf("instruction.agent không hợp lệ %q: chỉ nhận editor hoặc writer", in.Agent)
	}
	if strings.TrimSpace(in.Task) == "" {
		return fmt.Errorf("instruction thiếu task")
	}
	return nil
}

func (e Expect) validate(caseID string) error {
	regexes := e.regexes()
	if len(regexes) > maxCaseRegexes {
		return fmt.Errorf("case %q có %d regex, vượt giới hạn %d", caseID, len(regexes), maxCaseRegexes)
	}
	for _, pattern := range regexes {
		if len(pattern) > maxRegexBytes {
			return fmt.Errorf("case %q có regex dài %d byte, vượt giới hạn %d: %q", caseID, len(pattern), maxRegexBytes, pattern)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("case %q có regex không hợp lệ %q: %w", caseID, pattern, err)
		}
	}
	if e.Review != nil {
		if err := e.Review.validate(caseID); err != nil {
			return err
		}
	}
	if e.Prose != nil {
		if e.Prose.Chapter <= 0 {
			return fmt.Errorf("case %q: expect.prose.chapter phải lớn hơn 0", caseID)
		}
		if e.Prose.MaxOpeningOverlapRunes < 0 {
			return fmt.Errorf("case %q: expect.prose.max_opening_overlap_runes không được âm", caseID)
		}
	}
	if e.Facts != nil && e.Facts.Chapter <= 0 {
		return fmt.Errorf("case %q: expect.facts.chapter phải lớn hơn 0", caseID)
	}
	for i, needle := range e.ContextContains {
		if strings.TrimSpace(needle) == "" {
			return fmt.Errorf("case %q: expect.context_contains[%d] không được để trống", caseID, i)
		}
	}
	return nil
}

// regexes 返回 case 里声明过的全部正则，供规模与可编译性校验统一计数。
func (e Expect) regexes() []string {
	if e.Prose == nil {
		return nil
	}
	out := make([]string, 0, len(e.Prose.ForbidRegex)+len(e.Prose.RequireRegex))
	out = append(out, e.Prose.ForbidRegex...)
	out = append(out, e.Prose.RequireRegex...)
	return out
}

func (r ReviewExpect) validate(caseID string) error {
	for _, verdict := range r.VerdictIn {
		switch verdict {
		case "accept", "polish", "rewrite":
		default:
			return fmt.Errorf("case %q: verdict_in chứa giá trị không hợp lệ %q", caseID, verdict)
		}
	}
	for _, item := range r.MustFind {
		if err := validateExpectSeverity(caseID, "must_find", item.SeverityMin); err != nil {
			return err
		}
		if item.Chapter < 0 {
			return fmt.Errorf("case %q: must_find.chapter không được âm", caseID)
		}
	}
	for _, item := range r.MustNotFind {
		if err := validateExpectSeverity(caseID, "must_not_find", item.SeverityMin); err != nil {
			return err
		}
	}
	for id, status := range r.GoalStatus {
		switch status {
		case "pass", "fail", "unknown":
		default:
			return fmt.Errorf("case %q: goal_status[%s] không hợp lệ %q", caseID, id, status)
		}
	}
	for quote, status := range r.RecheckStatus {
		if strings.TrimSpace(quote) == "" {
			return fmt.Errorf("case %q: recheck_status có khóa rỗng", caseID)
		}
		switch status {
		case "fixed", "unfixed", "unknown":
		default:
			return fmt.Errorf("case %q: recheck_status[%s] không hợp lệ %q", caseID, quote, status)
		}
	}
	return nil
}

func validateExpectSeverity(caseID, field, severity string) error {
	if severity == "" || validExpectSeverity(severity) {
		return nil
	}
	return fmt.Errorf("case %q: %s.severity_min không hợp lệ %q (chỉ nhận warning/error/critical)", caseID, field, severity)
}

func float64Ptr(v float64) *float64 { return &v }

func validStylestatGate(s string) bool {
	switch s {
	case "warn", "block", "off":
		return true
	default:
		return false
	}
}

// LoadCases 从单个 .json 文件或目录加载 case。目录下所有 *.json 递归加载，按 id 排序。
func LoadCases(path string) ([]Case, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var files []string
	if info.IsDir() {
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(p, ".json") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = []string{path}
	}

	var cases []Case
	seen := map[string]string{}
	for _, f := range files {
		c, err := loadCaseFile(f)
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[c.ID]; dup {
			return nil, fmt.Errorf("case id trùng: %q (%s và %s)", c.ID, prev, f)
		}
		seen[c.ID] = f
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("không tìm thấy case nào: %s", path)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	return cases, nil
}

func loadCaseFile(path string) (Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Case{}, err
	}
	var c Case
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields() // 拼错字段直接报错，避免静默忽略
	if err := dec.Decode(&c); err != nil {
		return Case{}, fmt.Errorf("phân tích case %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return Case{}, err
	}
	return c, nil
}
