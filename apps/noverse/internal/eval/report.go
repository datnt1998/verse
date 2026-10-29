package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ArmSingle   = "single"
	ArmBaseline = "baseline"
	ArmVariant  = "variant"
)

// unknownCorrectNA 表示该指标还没有实现（目标检查清单）。明确写 N/A，而不是 0，
// 避免把"没能力判"记成"判了、结果为零"。
const unknownCorrectNA = "N/A"

// costInconclusive 是 usage_missing>0 时成本/token 的结论：缺用量的样本不能相加，
// 加出来的数字只会诱导错误决策。
const costInconclusive = "không kết luận được"

// Suite 是一次评测运行的聚合结果。
type Suite struct {
	RunID   string       `json:"run_id"`
	Mode    string       `json:"mode"` // single / ab
	Variant string       `json:"variant,omitempty"`
	Repeat  int          `json:"repeat"`
	Gate    Outcome      `json:"gate"`
	Metrics SuiteMetrics `json:"metrics"`
	Cases   []CaseResult `json:"cases"`
}

// RateMetric 是"命中/总数"形式的指标。A/B 决策看 Rate，不看总和。
type RateMetric struct {
	Hit   int     `json:"hit"`
	Total int     `json:"total"`
	Rate  float64 `json:"rate"`
}

// TokenMetrics 是全部运行累加的 token 用量。
type TokenMetrics struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cache_read"`
	CacheWrite int `json:"cache_write"`
}

// SuiteMetrics 把不同性质的质量信号分开记：漏报（critical_hit）、误报
// （false_positive）、机械可用性（mechanical_valid）、写作违例与成本各占一行。
// 刻意不合成单一得分——一个总分既看不出漏报，也看不出误报。
type SuiteMetrics struct {
	MechanicalValid  RateMetric     `json:"mechanical_valid"`
	CriticalHit      RateMetric     `json:"critical_hit"`
	FalsePositive    int            `json:"false_positive"`
	UnknownCorrect   string         `json:"unknown_correct"`
	WriterViolations int            `json:"writer_violations"`
	ToolErrors       int            `json:"tool_errors"`
	ContextTrims     int            `json:"context_trims"`
	ContextMissing   int            `json:"context_missing,omitempty"`
	QuoteCoverage    RateMetric     `json:"quote_coverage"`
	WorkerRuns       map[string]int `json:"worker_runs"`
	Tokens           TokenMetrics   `json:"tokens"`
	CostUSD          float64        `json:"cost_usd"`
	UsageMissing     int            `json:"usage_missing"`
	CostConclusion   string         `json:"cost_conclusion,omitempty"`
}

type CaseResult struct {
	CaseID   string        `json:"case_id"`
	Category string        `json:"category"`
	Role     string        `json:"role,omitempty"`
	Outcome  Outcome       `json:"outcome"`
	Runs     []RunResult   `json:"runs"`
	Deltas   []Delta       `json:"deltas,omitempty"`
	Summary  RepeatSummary `json:"summary"`
}

type RunResult struct {
	Arm    string `json:"arm"`
	Repeat int    `json:"repeat"`
	Result Result `json:"result"`
}

type RepeatSummary struct {
	PassCount       int           `json:"pass_count"`
	PassRate        float64       `json:"pass_rate"`
	HardFailRuns    int           `json:"hard_fail_runs"`
	WarningRuns     int           `json:"warning_runs"`
	CostUSD         *RangeSummary `json:"cost_usd,omitempty"`
	ToolCalls       *RangeSummary `json:"tool_calls,omitempty"`
	VariantPassRate float64       `json:"variant_pass_rate,omitempty"`
	DeltaPassRate   float64       `json:"delta_pass_rate,omitempty"`
}

type RangeSummary struct {
	Min float64 `json:"min"`
	Avg float64 `json:"avg"`
	Max float64 `json:"max"`
}

// Aggregate 把单 case 结果汇总成 suite，并计算整体门禁：任一 FAIL→FAIL，否则任一 WARN→WARN。
func Aggregate(runID, mode, variant string, repeat int, cases []CaseResult) Suite {
	gate := Pass
	for _, c := range cases {
		gate = worstOutcome(gate, c.Outcome)
	}
	if repeat <= 0 {
		repeat = 1
	}
	return Suite{
		RunID:   runID,
		Mode:    mode,
		Variant: variant,
		Repeat:  repeat,
		Gate:    gate,
		Metrics: computeSuiteMetrics(cases),
		Cases:   cases,
	}
}

func NewSingleCaseResult(c Case, r Result) CaseResult {
	r.Arm = ArmSingle
	r.Repeat = 1
	return NewSingleRunsCaseResult(c, []RunResult{{Arm: ArmSingle, Repeat: 1, Result: r}})
}

func NewSingleRunsCaseResult(c Case, runs []RunResult) CaseResult {
	cr := CaseResult{
		CaseID:   c.ID,
		Category: c.Category,
		Role:     c.Role,
		Runs:     runs,
	}
	cr.finalize()
	return cr
}

func NewABCaseResult(c Case, runs []RunResult, deltas []Delta) CaseResult {
	cr := CaseResult{
		CaseID:   c.ID,
		Category: c.Category,
		Role:     c.Role,
		Runs:     runs,
		Deltas:   deltas,
	}
	cr.finalize()
	return cr
}

func (c *CaseResult) finalize() {
	outcome := Pass
	for _, r := range c.Runs {
		outcome = worstOutcome(outcome, r.Result.Outcome)
	}
	for _, d := range c.Deltas {
		outcome = worstOutcome(outcome, d.Outcome)
	}
	c.Outcome = outcome
	c.Summary = summarizeRuns(c.Runs, c.Deltas)
}

// summarizeRuns 汇总重复运行。pass 指"断言全部成立"（无 hard fail）：fixture 模式下
// editor 正确判 rewrite 后，书稿诊断必然报"待重写"等 warning，那是正确行为的后果，
// 不能让抓到错误的运行被计为未通过。warning 仍单独计数，CI 退出码仍取最差 Outcome。
func summarizeRuns(runs []RunResult, deltas []Delta) RepeatSummary {
	var s RepeatSummary
	total := len(runs)
	if total > 0 {
		pass := 0
		var costs, toolCalls []float64
		for _, run := range runs {
			switch run.Result.Outcome {
			case Fail:
				s.HardFailRuns++
			case Warn:
				s.WarningRuns++
				pass++
			case Pass:
				pass++
			}
			if run.Result.Metrics.Usage.UsageRecorded {
				costs = append(costs, run.Result.Metrics.Usage.CostUSD)
			}
			if run.Result.Metrics.ToolCalls > 0 {
				toolCalls = append(toolCalls, float64(run.Result.Metrics.ToolCalls))
			}
		}
		s.PassCount = pass
		s.PassRate = round2(float64(pass) / float64(total))
		s.CostUSD = summarizeRange(costs)
		s.ToolCalls = summarizeRange(toolCalls)
	}
	variantTotal, variantPass := 0, 0
	for _, run := range runs {
		if run.Arm != ArmVariant {
			continue
		}
		variantTotal++
		if run.Result.Outcome != Fail {
			variantPass++
		}
	}
	if variantTotal > 0 {
		s.VariantPassRate = round2(float64(variantPass) / float64(variantTotal))
	}
	if len(deltas) > 0 {
		pass := 0
		for _, d := range deltas {
			if d.Outcome == Pass {
				pass++
			}
		}
		s.DeltaPassRate = round2(float64(pass) / float64(len(deltas)))
	}
	return s
}

func summarizeRange(values []float64) *RangeSummary {
	if len(values) == 0 {
		return nil
	}
	min, max, sum := values[0], values[0], 0.0
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	return &RangeSummary{Min: round2(min), Avg: round2(sum / float64(len(values))), Max: round2(max)}
}

// computeSuiteMetrics 汇总所有 run 的质量信号。A/B 模式下两种 arm 都计入，
// 因为报告回答的是"这批跑动实际发生了什么"，arm 之间的比较由 delta 负责。
func computeSuiteMetrics(cases []CaseResult) SuiteMetrics {
	m := SuiteMetrics{WorkerRuns: map[string]int{}, UnknownCorrect: unknownCorrectNA}
	var unknownHit, unknownTotal int
	for _, c := range cases {
		for _, run := range c.Runs {
			r := run.Result
			if r.ReviewChecked {
				m.MechanicalValid.Total++
				if r.MechanicalValid {
					m.MechanicalValid.Hit++
				}
			}
			m.CriticalHit.Hit += r.MustFindHit
			m.CriticalHit.Total += r.MustFindTotal
			m.FalsePositive += r.FalsePositives
			m.WriterViolations += r.WriterViolations
			m.ToolErrors += r.Metrics.ToolErrors
			for agent, count := range r.Metrics.WorkerRuns {
				m.WorkerRuns[agent] += count
			}
			m.ContextTrims += r.Metrics.ContextTrims
			m.ContextMissing += r.Metrics.ContextMissing
			m.QuoteCoverage.Hit += r.Metrics.QuoteCoverage.Hit
			m.QuoteCoverage.Total += r.Metrics.QuoteCoverage.Total
			unknownHit += r.UnknownHit
			unknownTotal += r.UnknownExpected
			usage := r.Metrics.Usage
			m.Tokens.Input += usage.Input
			m.Tokens.Output += usage.Output
			m.Tokens.CacheRead += usage.CacheRead
			m.Tokens.CacheWrite += usage.CacheWrite
			m.CostUSD += usage.CostUSD
			m.UsageMissing += usage.MissingUsage
		}
	}
	m.MechanicalValid.Rate = rateOf(m.MechanicalValid.Hit, m.MechanicalValid.Total)
	m.QuoteCoverage.Rate = rateOf(m.QuoteCoverage.Hit, m.QuoteCoverage.Total)
	m.CriticalHit.Rate = rateOf(m.CriticalHit.Hit, m.CriticalHit.Total)
	if unknownTotal > 0 {
		m.UnknownCorrect = fmt.Sprintf("%d/%d (%.2f)", unknownHit, unknownTotal, round2(float64(unknownHit)/float64(unknownTotal)))
	}
	m.CostUSD = round4(m.CostUSD)
	if m.UsageMissing > 0 {
		m.CostConclusion = costInconclusive
	}
	return m
}

func rateOf(hit, total int) float64 {
	if total <= 0 {
		return 0
	}
	return round2(float64(hit) / float64(total))
}

func round4(f float64) float64 {
	if f < 0 {
		return -round4(-f)
	}
	return float64(int64(f*10000+0.5)) / 10000
}

func worstOutcome(a, b Outcome) Outcome {
	if a == Fail || b == Fail {
		return Fail
	}
	if a == Warn || b == Warn {
		return Warn
	}
	return Pass
}

// WriteReport 在 outDir 下写 report.json（机读）与 report.md（人读）。
func WriteReport(s Suite, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "report.json"), data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "report.md"), []byte(renderMarkdown(s)), 0o644)
}

// Summary 是给 stdout 的精简结论。
func Summary(s Suite) string {
	var hardFails, warnings int
	for _, c := range s.Cases {
		for _, run := range c.Runs {
			hardFails += len(run.Result.HardFails)
			warnings += len(run.Result.Warnings)
		}
		for _, d := range c.Deltas {
			hardFails += len(d.HardFails)
			warnings += len(d.Warnings)
		}
	}
	return fmt.Sprintf("Gate: %s  (%d cases, %d hard fails, %d warnings)",
		s.Gate, len(s.Cases), hardFails, warnings)
}

func renderMarkdown(s Suite) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Eval Report\n\n")
	fmt.Fprintf(&b, "Gate: **%s**\n\n", s.Gate)

	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "- run_id: %s\n", s.RunID)
	fmt.Fprintf(&b, "- mode: %s\n", s.Mode)
	if s.Variant != "" {
		fmt.Fprintf(&b, "- variant: %s\n", s.Variant)
	}
	fmt.Fprintf(&b, "- repeat: %d\n", s.Repeat)
	fmt.Fprintf(&b, "- cases: %d\n\n", len(s.Cases))

	writeSuiteMetrics(&b, s.Metrics)

	fmt.Fprintf(&b, "## Cases\n\n")
	for _, c := range s.Cases {
		fmt.Fprintf(&b, "### %s  [%s]\n\n", c.CaseID, c.Outcome)
		if c.Role != "" {
			fmt.Fprintf(&b, "- category=%s role=%s\n", c.Category, c.Role)
		} else {
			fmt.Fprintf(&b, "- category=%s\n", c.Category)
		}
		total := len(c.Runs)
		if total == 0 {
			total = s.Repeat
		}
		fmt.Fprintf(&b, "- pass_rate=%d/%d (%.2f) hard_fail_runs=%d warning_runs=%d",
			c.Summary.PassCount, total, c.Summary.PassRate, c.Summary.HardFailRuns, c.Summary.WarningRuns)
		if c.Summary.VariantPassRate > 0 || len(c.Deltas) > 0 {
			fmt.Fprintf(&b, " variant_pass_rate=%.2f delta_pass_rate=%.2f",
				c.Summary.VariantPassRate, c.Summary.DeltaPassRate)
		}
		fmt.Fprintf(&b, "\n")
		if c.Summary.CostUSD != nil {
			r := c.Summary.CostUSD
			fmt.Fprintf(&b, "- cost_usd: min=%.2f avg=%.2f max=%.2f\n", r.Min, r.Avg, r.Max)
		}
		if c.Summary.ToolCalls != nil {
			r := c.Summary.ToolCalls
			fmt.Fprintf(&b, "- tool_calls: min=%.0f avg=%.0f max=%.0f\n", r.Min, r.Avg, r.Max)
		}

		for _, run := range c.Runs {
			writeRun(&b, run)
		}
		for i, d := range c.Deltas {
			writeDelta(&b, i+1, d)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func writeRun(b *strings.Builder, run RunResult) {
	r := run.Result
	m := r.Metrics
	label := run.Arm
	if run.Repeat > 1 {
		label = fmt.Sprintf("%s#%d", run.Arm, run.Repeat)
	}
	fmt.Fprintf(b, "- %s: %s phase=%s flow=%s completed=%d/%d words=%d findings(crit=%d warn=%d) tool_calls=%d",
		label, r.Outcome, m.Phase, m.Flow, m.CompletedChapters, m.TotalChapters, m.TotalWords,
		m.CriticalFindings, m.WarningFindings, m.ToolCalls)
	if m.Usage.UsageRecorded {
		fmt.Fprintf(b, " cost=$%.4f tokens(in=%d out=%d)", m.Usage.CostUSD, m.Usage.Input, m.Usage.Output)
	}
	if m.StylestatStatus != "" {
		fmt.Fprintf(b, " stylestat=%s", m.StylestatStatus)
	}
	if len(m.WorkerRuns) > 0 {
		fmt.Fprintf(b, " worker_runs=%s", renderWorkerRuns(m.WorkerRuns))
	}
	if m.ToolErrors > 0 {
		fmt.Fprintf(b, " tool_errors=%d", m.ToolErrors)
	}
	if m.ContextTrims > 0 {
		fmt.Fprintf(b, " context_trims=%d", m.ContextTrims)
	}
	if m.ContextMissing > 0 {
		fmt.Fprintf(b, " context_missing=%d", m.ContextMissing)
	}
	if m.QuoteCoverage.Total > 0 {
		fmt.Fprintf(b, " quote_coverage=%d/%d", m.QuoteCoverage.Hit, m.QuoteCoverage.Total)
	}
	fmt.Fprintf(b, "\n")
	writeIssues(b, "Hard Fail", r.HardFails)
	writeIssues(b, "Warnings", r.Warnings)
	writeIssues(b, "Notes", r.Notes)
	writeIssues(b, "Skipped", r.Skips)
	if r.Dir != "" {
		fmt.Fprintf(b, "  - artifacts: %s\n", r.Dir)
	}
}

// writeSuiteMetrics 把各类质量信号分行写出。成本缺用量时明确标"không kết luận
// được"，不让读者把残缺的累加值当成真实花费。
func writeSuiteMetrics(b *strings.Builder, m SuiteMetrics) {
	fmt.Fprintf(b, "## Metrics\n\n")
	fmt.Fprintf(b, "- mechanical_valid: %s\n", renderRate(m.MechanicalValid))
	fmt.Fprintf(b, "- critical_hit: %s\n", renderRate(m.CriticalHit))
	fmt.Fprintf(b, "- false_positive: %d\n", m.FalsePositive)
	fmt.Fprintf(b, "- unknown_correct: %s\n", m.UnknownCorrect)
	fmt.Fprintf(b, "- writer_violations: %d\n", m.WriterViolations)
	fmt.Fprintf(b, "- tool_errors: %d\n", m.ToolErrors)
	if m.ContextMissing > 0 {
		fmt.Fprintf(b, "- context_trims: N/A (%d observed; %d missing results)\n", m.ContextTrims, m.ContextMissing)
	} else {
		fmt.Fprintf(b, "- context_trims: %d\n", m.ContextTrims)
	}
	fmt.Fprintf(b, "- context_missing: %d\n", m.ContextMissing)
	fmt.Fprintf(b, "- quote_coverage: %s\n", renderRate(m.QuoteCoverage))
	fmt.Fprintf(b, "- worker_runs: %s\n", renderWorkerRuns(m.WorkerRuns))
	suffix := ""
	if m.CostConclusion != "" {
		suffix = " (" + m.CostConclusion + ")"
	}
	fmt.Fprintf(b, "- tokens: input=%d output=%d cache_read=%d cache_write=%d%s\n",
		m.Tokens.Input, m.Tokens.Output, m.Tokens.CacheRead, m.Tokens.CacheWrite, suffix)
	fmt.Fprintf(b, "- cost_usd: %.4f%s\n", m.CostUSD, suffix)
	fmt.Fprintf(b, "- usage_missing: %d\n\n", m.UsageMissing)
}

func renderRate(metric RateMetric) string {
	if metric.Total <= 0 {
		return unknownCorrectNA
	}
	return fmt.Sprintf("%d/%d (%.2f)", metric.Hit, metric.Total, metric.Rate)
}

func renderWorkerRuns(runs map[string]int) string {
	if len(runs) == 0 {
		return unknownCorrectNA
	}
	agents := make([]string, 0, len(runs))
	for agent := range runs {
		agents = append(agents, agent)
	}
	sort.Strings(agents)
	parts := make([]string, 0, len(agents))
	for _, agent := range agents {
		parts = append(parts, fmt.Sprintf("%s:%d", agent, runs[agent]))
	}
	return strings.Join(parts, " ")
}

func writeDelta(b *strings.Builder, idx int, d Delta) {
	m := d.Metrics
	fmt.Fprintf(b, "- delta#%d: %s completed_delta=%+d crit_delta=%+d warn_delta=%+d words_ratio=%.2f tool_calls_delta=%.2f cost_delta=%.2f\n",
		idx, d.Outcome, m.CompletedChapters, m.CriticalFindings, m.WarningFindings,
		m.TotalWordsRatio, m.ToolCallDeltaRatio, m.CostDeltaRatio)
	if m.Stylestat != nil {
		sd := m.Stylestat
		fmt.Fprintf(b, "  - stylestat: %s pattern_top=%+0.1f ending_short=%+0.2f repeated=%+d title_mixed=%+d\n",
			sd.Status, sd.PatternTopPerChapter, sd.EndingShortRatio, sd.RepeatedSentences, sd.TitleMixedDelta)
	}
	writeIssues(b, "Delta Hard Fail", d.HardFails)
	writeIssues(b, "Delta Warnings", d.Warnings)
	writeIssues(b, "Delta Notes", d.Notes)
}

func writeIssues(b *strings.Builder, title string, issues []Issue) {
	if len(issues) == 0 {
		return
	}
	fmt.Fprintf(b, "  - %s:\n", title)
	for _, it := range issues {
		if it.Severity != "" {
			fmt.Fprintf(b, "    - [%s] %s - %s\n", it.Severity, it.Source, it.Detail)
		} else {
			fmt.Fprintf(b, "    - %s - %s\n", it.Source, it.Detail)
		}
	}
}
