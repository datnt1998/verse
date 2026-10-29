package eval

import (
	"strings"
	"testing"
)

func TestAggregateRepeatSummary(t *testing.T) {
	c := writerSmokeCase()
	pass := cleanResult()
	pass.Metrics.Usage = UsageMetrics{UsageRecorded: true, CostUSD: 0.1}
	pass.Metrics.ToolCalls = 10
	warn := pass
	warn.Outcome = Warn
	warn.Warnings = []Issue{{Kind: "warning", Source: "x", Detail: "warn"}}
	fail := pass
	fail.Outcome = Fail
	fail.HardFails = []Issue{{Kind: "hard_fail", Source: "x", Detail: "fail"}}

	cr := NewSingleRunsCaseResult(c, []RunResult{
		{Arm: ArmSingle, Repeat: 1, Result: pass},
		{Arm: ArmSingle, Repeat: 2, Result: warn},
		{Arm: ArmSingle, Repeat: 3, Result: fail},
	})
	if cr.Outcome != Fail {
		t.Fatalf("case outcome = %s want FAIL", cr.Outcome)
	}
	// WARN 运行的断言全部成立，计入通过率；只有 hard fail 不算。
	if cr.Summary.PassCount != 2 || cr.Summary.PassRate != 0.67 {
		t.Fatalf("pass = %d (%.2f) want 2 (0.67)", cr.Summary.PassCount, cr.Summary.PassRate)
	}
	if cr.Summary.HardFailRuns != 1 || cr.Summary.WarningRuns != 1 {
		t.Fatalf("summary runs 不正确: %+v", cr.Summary)
	}
	if cr.Summary.CostUSD == nil || cr.Summary.CostUSD.Avg != 0.1 ||
		cr.Summary.ToolCalls == nil || cr.Summary.ToolCalls.Avg != 10 {
		t.Fatalf("range summary 不正确: %+v", cr.Summary)
	}

	s := Aggregate("run", "single", "", 3, []CaseResult{cr})
	if s.Gate != Fail || s.Repeat != 3 || len(s.Cases) != 1 {
		t.Fatalf("suite 聚合不正确: %+v", s)
	}
}

func TestAggregateABDeltaControlsGate(t *testing.T) {
	c := writerSmokeCase()
	base := cleanResult()
	variant := cleanResult()
	variant.Metrics.WarningFindings = base.Metrics.WarningFindings + 1
	delta := GradeDelta(c, base, variant)
	cr := NewABCaseResult(c, []RunResult{
		{Arm: ArmBaseline, Repeat: 1, Result: base},
		{Arm: ArmVariant, Repeat: 1, Result: variant},
	}, []Delta{delta})

	if cr.Outcome != Warn {
		t.Fatalf("delta warning 应让 case WARN，得到 %s", cr.Outcome)
	}
	if cr.Summary.VariantPassRate != 1 || cr.Summary.DeltaPassRate != 0 {
		t.Fatalf("AB summary 不正确: %+v", cr.Summary)
	}
	s := Aggregate("run", "ab", "writer-x", 1, []CaseResult{cr})
	if s.Gate != Warn {
		t.Fatalf("suite gate = %s want WARN", s.Gate)
	}
}

// 各类质量信号分开汇总：漏报看 critical_hit，误报看 false_positive，
// 机械可用性只看声明了 review 断言的 run。
func TestSuiteMetricsSeparateSignals(t *testing.T) {
	c := writerSmokeCase()
	good := cleanResult()
	good.ReviewChecked = true
	good.MechanicalValid = true
	good.MustFindTotal = 1
	good.MustFindHit = 1
	good.Metrics.ToolErrors = 2
	good.Metrics.WorkerRuns = map[string]int{"editor": 1}
	good.Metrics.Usage = UsageMetrics{UsageRecorded: true, Input: 100, Output: 50, CostUSD: 0.25}

	bad := cleanResult()
	bad.Outcome = Fail
	bad.HardFails = []Issue{{Kind: "hard_fail", Source: "x", Detail: "fail"}}
	bad.ReviewChecked = true
	bad.MustFindTotal = 1
	bad.FalsePositives = 2
	bad.WriterViolations = 1
	bad.Metrics.ToolErrors = 1
	bad.Metrics.WorkerRuns = map[string]int{"editor": 1}
	bad.Metrics.Usage = UsageMetrics{UsageRecorded: true, Input: 100, Output: 50, CostUSD: 0.25, MissingUsage: 1}

	s := Aggregate("run", "single", "", 2, []CaseResult{NewSingleRunsCaseResult(c, []RunResult{
		{Arm: ArmSingle, Repeat: 1, Result: good},
		{Arm: ArmSingle, Repeat: 2, Result: bad},
	})})

	m := s.Metrics
	if m.MechanicalValid.Hit != 1 || m.MechanicalValid.Total != 2 || m.MechanicalValid.Rate != 0.5 {
		t.Fatalf("mechanical_valid = %+v want 1/2", m.MechanicalValid)
	}
	if m.CriticalHit.Hit != 1 || m.CriticalHit.Total != 2 {
		t.Fatalf("critical_hit = %+v want 1/2", m.CriticalHit)
	}
	if m.FalsePositive != 2 || m.WriterViolations != 1 {
		t.Fatalf("false_positive=%d writer_violations=%d", m.FalsePositive, m.WriterViolations)
	}
	if m.ToolErrors != 3 {
		t.Fatalf("tool_errors = %d want 3", m.ToolErrors)
	}
	if m.WorkerRuns["editor"] != 2 {
		t.Fatalf("worker_runs = %+v", m.WorkerRuns)
	}
	if m.Tokens.Input != 200 || m.Tokens.Output != 100 {
		t.Fatalf("tokens = %+v", m.Tokens)
	}
	if m.CostUSD != 0.5 || m.UsageMissing != 1 {
		t.Fatalf("cost=%v usage_missing=%d", m.CostUSD, m.UsageMissing)
	}
	if m.CostConclusion != costInconclusive {
		t.Fatalf("usage_missing>0 时成本必须标为不可结论，得到 %q", m.CostConclusion)
	}
	if m.UnknownCorrect != unknownCorrectNA {
		t.Fatalf("unknown_correct = %q want N/A", m.UnknownCorrect)
	}

	md := renderMarkdown(s)
	for _, want := range []string{"mechanical_valid: 1/2", "critical_hit: 1/2", "unknown_correct: N/A", "không kết luận được", "pass_rate=1/2"} {
		if !strings.Contains(md, want) {
			t.Fatalf("report.md 缺少 %q:\n%s", want, md)
		}
	}
}

// 没有 review 断言的 case 不参与 mechanical_valid 分母；成本齐全时不做不可结论标记。
func TestSuiteMetricsSkipsMechanicalWithoutReview(t *testing.T) {
	c := writerSmokeCase()
	r := cleanResult()
	r.Metrics.Usage = UsageMetrics{UsageRecorded: true, CostUSD: 0.1}
	s := Aggregate("run", "single", "", 1, []CaseResult{NewSingleCaseResult(c, r)})
	if s.Metrics.MechanicalValid.Total != 0 {
		t.Fatalf("未要求评审的 case 不应进 mechanical_valid 分母: %+v", s.Metrics.MechanicalValid)
	}
	if s.Metrics.CostConclusion != "" {
		t.Fatalf("usage 齐全时不应标不可结论，得到 %q", s.Metrics.CostConclusion)
	}
	if s.Metrics.CriticalHit.Total != 0 {
		t.Fatalf("没有 must_find 的 case 不应进 critical_hit 分母")
	}
}

// unknown_correct 只统计金标为 unknown 的 id：命中数/期望数聚合成比率，
// 没有 unknown 期望时保持 N/A。
func TestSuiteMetricsUnknownCorrectRate(t *testing.T) {
	c := writerSmokeCase()
	hit := cleanResult()
	hit.UnknownExpected = 2
	hit.UnknownHit = 1
	miss := cleanResult()
	miss.UnknownExpected = 1
	miss.UnknownHit = 0
	s := Aggregate("run", "single", "", 2, []CaseResult{NewSingleRunsCaseResult(c, []RunResult{
		{Arm: ArmSingle, Repeat: 1, Result: hit},
		{Arm: ArmSingle, Repeat: 2, Result: miss},
	})})
	if s.Metrics.UnknownCorrect != "1/3 (0.33)" {
		t.Fatalf("unknown_correct = %q, want 1/3 (0.33)", s.Metrics.UnknownCorrect)
	}
}
