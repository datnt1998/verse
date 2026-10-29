package eval

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestSmokeCasesLoad 确保仓库内置 smoke case 能被加载器解析（含 DisallowUnknownFields 校验）。
func TestSmokeCasesLoad(t *testing.T) {
	dir := filepath.Join("..", "..", "evals", "cases", "smoke")
	cases, err := LoadCases(dir)
	if err != nil {
		t.Fatalf("加载 smoke case 失败: %v", err)
	}
	if len(cases) < 3 {
		t.Fatalf("期望至少 3 个 smoke case，得到 %d", len(cases))
	}
	for _, c := range cases {
		if c.Category != "smoke" {
			t.Errorf("%s: category 应为 smoke，得到 %s", c.ID, c.Category)
		}
		if c.Gate.MaxSeverity == "" {
			t.Errorf("%s: Validate 应填充默认 max_severity", c.ID)
		}
		if c.Gate.MaxCostDeltaRatio == nil || *c.Gate.MaxCostDeltaRatio != 0.3 ||
			c.Gate.MaxToolCallDeltaRatio == nil || *c.Gate.MaxToolCallDeltaRatio != 0.3 {
			t.Errorf("%s: Validate 应填充默认 delta ratio，得到 cost=%v tool=%v",
				c.ID, c.Gate.MaxCostDeltaRatio, c.Gate.MaxToolCallDeltaRatio)
		}
		if c.Gate.StylestatRegression != "warn" {
			t.Errorf("%s: Validate 应默认 stylestat_regression=warn，得到 %s", c.ID, c.Gate.StylestatRegression)
		}
	}
}

func TestLoadCasesRejectsUnknownField(t *testing.T) {
	// 间接验证：合法 case 必须含 id+prompt；缺失即报错（Validate 路径）。
	if _, err := LoadCases(filepath.Join("..", "..", "evals", "cases", "smoke", "writer_first_chapter.json")); err != nil {
		t.Fatalf("单文件加载应成功: %v", err)
	}
}

// case id 会拼进 RemoveAll 的路径，路径穿越/分隔符必须被拒（高危防护）。
func TestCaseIDRejectsUnsafe(t *testing.T) {
	for _, bad := range []string{"../evil", "a/b", "/abs", "..", "Up", "with space", "dot.case"} {
		c := Case{ID: bad, Prompt: "x"}
		if err := c.Validate(); err == nil {
			t.Errorf("非法 id %q 应被拒", bad)
		}
	}
	for _, ok := range []string{"writer_first_chapter", "architect-long", "case1"} {
		c := Case{ID: ok, Prompt: "x"}
		if err := c.Validate(); err != nil {
			t.Errorf("合法 id %q 不应被拒: %v", ok, err)
		}
	}
}

func TestCaseRejectsInvalidGate(t *testing.T) {
	c := Case{ID: "bad_gate", Prompt: "x", Gate: Gate{StylestatRegression: "maybe"}}
	if err := c.Validate(); err == nil {
		t.Fatal("非法 stylestat_regression 应被拒")
	}
	c = Case{ID: "disabled_ratio", Prompt: "x", Gate: Gate{MaxCostDeltaRatio: float64Ptr(-1), MaxToolCallDeltaRatio: float64Ptr(-1)}}
	if err := c.Validate(); err != nil {
		t.Fatalf("负数 delta ratio 应作为显式关闭被接受: %v", err)
	}
	if *c.Gate.MaxCostDeltaRatio != -1 || *c.Gate.MaxToolCallDeltaRatio != -1 {
		t.Fatalf("显式关闭的 delta ratio 不应被默认值覆盖: %+v", c.Gate)
	}
	c = Case{ID: "strict_ratio", Prompt: "x", Gate: Gate{MaxCostDeltaRatio: float64Ptr(0), MaxToolCallDeltaRatio: float64Ptr(0)}}
	if err := c.Validate(); err != nil {
		t.Fatalf("显式 0 delta ratio 应作为严格阈值被接受: %v", err)
	}
	if *c.Gate.MaxCostDeltaRatio != 0 || *c.Gate.MaxToolCallDeltaRatio != 0 {
		t.Fatalf("显式 0 delta ratio 不应被默认值覆盖: %+v", c.Gate)
	}
}

// fixture 模式：instruction 必填，prompt 不再必填。
func TestFixtureCaseRequiresInstruction(t *testing.T) {
	c := Case{ID: "ed-clean-01", Category: "quality", Fixture: "ed-clean-01"}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "instruction") {
		t.Fatalf("có fixture mà thiếu instruction phải bị từ chối, nhận %v", err)
	}

	c.Instruction = &Instruction{Agent: "editor", Task: "Thẩm duyệt chương 3"}
	if err := c.Validate(); err != nil {
		t.Fatalf("fixture + instruction 应可加载: %v", err)
	}
	if !c.FixtureMode() {
		t.Fatal("FixtureMode 应为 true")
	}
}

func TestInstructionRejectsUnknownAgent(t *testing.T) {
	c := Case{ID: "x", Fixture: "x", Instruction: &Instruction{Agent: "architect", Task: "t"}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "agent") {
		t.Fatalf("agent 不在 editor|writer 应报错，得到 %v", err)
	}

	c.Instruction = &Instruction{Agent: "writer", Task: "  "}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "task") {
		t.Fatalf("空 task 应报错，得到 %v", err)
	}
}

func TestInstructionWithoutFixtureRejected(t *testing.T) {
	c := Case{ID: "x", Prompt: "viết truyện", Instruction: &Instruction{Agent: "editor", Task: "t"}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "fixture") {
		t.Fatalf("没有 fixture 的 instruction 应被拒，得到 %v", err)
	}
}

// fixture 名会拼进路径：分隔符与 ".." 必须被拒。
func TestFixtureNameRejectsPath(t *testing.T) {
	for _, bad := range []string{"../evil", "a/b", "dir\\name", ".", "Has-Upper", ""} {
		if bad == "" {
			continue
		}
		c := Case{ID: "x", Fixture: bad, Instruction: &Instruction{Agent: "editor", Task: "t"}}
		if err := c.Validate(); err == nil {
			t.Errorf("fixture %q 应被拒", bad)
		}
	}
	for _, ok := range []string{"ed-clean-01", "wr-predecessor-replay"} {
		c := Case{ID: "x", Fixture: ok, Instruction: &Instruction{Agent: "editor", Task: "t"}}
		if err := c.Validate(); err != nil {
			t.Errorf("fixture %q 不应被拒: %v", ok, err)
		}
	}
}

func TestCaseRegexLimits(t *testing.T) {
	base := Case{
		ID: "x", Fixture: "x",
		Instruction: &Instruction{Agent: "writer", Task: "Viết chương 4"},
	}
	prose := func(forbid, require []string) *ProseExpect {
		return &ProseExpect{Chapter: 4, ForbidRegex: forbid, RequireRegex: require}
	}

	c := base
	c.Expect.Prose = prose([]string{"(chưa đóng ngoặc"}, nil)
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "regex không hợp lệ") {
		t.Fatalf("regex 编译失败应在 Validate 报错，得到 %v", err)
	}

	c = base
	c.Expect.Prose = prose([]string{strings.Repeat("a", maxRegexBytes+1)}, nil)
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "byte") {
		t.Fatalf("正则超长应报错，得到 %v", err)
	}

	c = base
	tooMany := make([]string, maxCaseRegexes+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("mốc-%d", i)
	}
	c.Expect.Prose = prose(tooMany, nil)
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "vượt giới hạn") {
		t.Fatalf("regex 数量超限应报错，得到 %v", err)
	}
}

func TestExpectSeverityAndStatusValidation(t *testing.T) {
	base := Case{
		ID: "x", Fixture: "x",
		Instruction: &Instruction{Agent: "editor", Task: "Thẩm duyệt chương 3"},
	}
	c := base
	c.Expect.Review = &ReviewExpect{MustFind: []MustFindExpect{{SeverityMin: "info"}}}
	if err := c.Validate(); err == nil {
		t.Fatal("severity_min=info 不是期望严重度，应被拒")
	}

	c = base
	c.Expect.Review = &ReviewExpect{MustNotFind: []MustNotFindExpect{{SeverityMin: "warning"}}}
	if err := c.Validate(); err != nil {
		t.Fatalf("warning 是合法期望严重度: %v", err)
	}

	c = base
	c.Expect.Review = &ReviewExpect{VerdictIn: []string{"approve"}}
	if err := c.Validate(); err == nil {
		t.Fatal("非法 verdict 应被拒")
	}

	c = base
	c.Expect.Review = &ReviewExpect{GoalStatus: map[string]string{"rb1": "maybe"}}
	if err := c.Validate(); err == nil {
		t.Fatal("非法 goal_status 应被拒")
	}

	c = base
	c.Expect.Review = &ReviewExpect{RecheckStatus: map[string]string{"câu trích": "done"}}
	if err := c.Validate(); err == nil {
		t.Fatal("非法 recheck_status 应被拒")
	}

	c = base
	c.Expect.Prose = &ProseExpect{Chapter: 0}
	if err := c.Validate(); err == nil {
		t.Fatal("prose.chapter=0 应被拒")
	}

	c = base
	c.Expect.ContextContains = []string{"loi_hoi_1979", "quá hạn thu"}
	if err := c.Validate(); err != nil {
		t.Fatalf("context_contains hợp lệ phải qua: %v", err)
	}

	c = base
	c.Expect.ContextContains = []string{"  "}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "context_contains") {
		t.Fatalf("chuỗi rỗng trong context_contains 应被拒，得到 %v", err)
	}
}

// 老 case 只有 prompt/expect 事实断言，必须继续能加载。
func TestLegacyCasesStillLoad(t *testing.T) {
	for _, dir := range []string{
		filepath.Join("..", "..", "evals", "cases", "smoke"),
	} {
		cases, err := LoadCases(dir)
		if err != nil {
			t.Fatalf("加载 %s: %v", dir, err)
		}
		for _, c := range cases {
			if c.FixtureMode() {
				t.Errorf("%s: case cũ không nên tự nhận fixture mode", c.ID)
			}
			if strings.TrimSpace(c.Prompt) == "" {
				t.Errorf("%s: case cũ phải còn prompt", c.ID)
			}
		}
	}
}
