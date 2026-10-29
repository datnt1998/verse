package userrules

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/llmcontract"
)

func TestExtractJSON_StripsCodeFences(t *testing.T) {
	cases := []struct{ in, wantHas string }{
		{"```json\n{\"a\":1}\n```", `"a":1`},
		{"```\n{\"a\":1}\n```", `"a":1`},
		{"前缀解释\n{\"a\":1}\n后缀", `"a":1`},
		{"{\"a\":1}", `"a":1`},
	}
	for _, c := range cases {
		got := llmcontract.ExtractJSONObject(c.in)
		if got == "" {
			t.Fatalf("extractJSON(%q) 返回空", c.in)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(got), &m); err != nil {
			t.Fatalf("extractJSON(%q)=%q 不是合法 JSON: %v", c.in, got, err)
		}
	}
	if llmcontract.ExtractJSONObject("没有任何 JSON") != "" {
		t.Fatal("无 JSON 时应返回空串")
	}
}

func TestParseNormalizerJSON_FullOutput(t *testing.T) {
	raw := "```json\n" + `{
  "structured": {
    "genre": "đô thị",
    "forbidden_chars": [],
    "forbidden_phrases": ["theo một nghĩa nào đó"],
    "fatigue_words": [{"word": "bỗng nhiên", "per_1000": null, "max_per_chapter": 2}]
  },
  "preferences": "nhân chính lạnh lùng, kiềm chế",
  "uncertain": ["少用比喻：无阈值"],
  "is_uncertain": true,
  "uncertain_note": "chưa rõ “hạn chế” là bao nhiêu phép so sánh",
  "options": ["Tối đa 2 phép so sánh mỗi chương", "Hạn chế so sánh, ưu tiên miêu tả trực tiếp"]
}` + "\n```"
	body := llmcontract.ExtractJSONObject(raw)
	if err := llmcontract.ValidateJSON(normalizeContract.Schema, []byte(body)); err != nil {
		t.Fatalf("应解析成功: %v", err)
	}
	var out normalizerOutput
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("应解码成功: %v", err)
	}
	cand, err := out.toCandidate("startup_prompt")
	if err != nil {
		t.Fatalf("toCandidate: %v", err)
	}
	if cand.Structured.Genre != "đô thị" {
		t.Fatalf("genre 解析错误：%+v", cand.Structured)
	}
	if len(cand.Structured.ForbiddenPhrases) != 1 || cand.Structured.ForbiddenPhrases[0] != "theo một nghĩa nào đó" {
		t.Fatalf("forbidden_phrases 解析错误：%v", cand.Structured.ForbiddenPhrases)
	}
	if got := cand.Structured.FatigueWords["bỗng nhiên"].Per1000(); got != 1.33 {
		t.Fatalf("fatigue_words max_per_chapter=2 phải quy đổi thành per_1000=1.33: %v", cand.Structured.FatigueWords)
	}
	if cand.Preferences != "nhân chính lạnh lùng, kiềm chế" {
		t.Fatalf("preferences 解析错误：%q", cand.Preferences)
	}
	if len(cand.Uncertain) != 1 {
		t.Fatalf("uncertain 应有 1 条，得到 %v", cand.Uncertain)
	}
	if len(cand.Options) != 2 || cand.Options[0] != "Tối đa 2 phép so sánh mỗi chương" {
		t.Fatalf("options 应解析成 2 条，得到 %v", cand.Options)
	}
}

// fatigue 条目校验：空词与缺失/非正阈值都是可反馈修正的业务错误。
func TestToCandidateRejectsInvalidFatigueEntries(t *testing.T) {
	two := 2
	zero := 0
	bad := normalizerOutput{Structured: normalizerStructured{
		FatigueWords: []fatigueWordEntry{{Word: " ", MaxPerChapter: &two}},
	}}
	if _, err := bad.toCandidate("x"); err == nil {
		t.Fatal("空词条目应报错")
	}
	bad = normalizerOutput{Structured: normalizerStructured{
		FatigueWords: []fatigueWordEntry{{Word: "bỗng nhiên", MaxPerChapter: &zero}},
	}}
	if _, err := bad.toCandidate("x"); err == nil {
		t.Fatal("非正整数阈值应报错")
	}
	bad = normalizerOutput{Structured: normalizerStructured{
		FatigueWords: []fatigueWordEntry{{Word: "bỗng nhiên"}},
	}}
	if _, err := bad.toCandidate("x"); err == nil {
		t.Fatal("两个阈值都缺应报错")
	}
}

// per_1000 是主阈值：与 max_per_chapter 同现时 per_1000 胜出；只给旧的
// max_per_chapter 时按章长基准 1.500 tiếng 换算（2 → 1,33）。
func TestToCandidateFatiguePer1000Precedence(t *testing.T) {
	p1000, nine := 2.5, 9
	out := normalizerOutput{Structured: normalizerStructured{
		FatigueWords: []fatigueWordEntry{
			{Word: "một chút", Per1000: &p1000, MaxPerChapter: &nine},
			{Word: "bỗng nhiên", MaxPerChapter: &nine},
		},
	}}
	cand, err := out.toCandidate("x")
	if err != nil {
		t.Fatalf("toCandidate: %v", err)
	}
	if got := cand.Structured.FatigueWords["một chút"].Per1000(); got != 2.5 {
		t.Errorf("per_1000 phải thắng: got %v, want 2.5", got)
	}
	if got := cand.Structured.FatigueWords["bỗng nhiên"].Per1000(); got != 6.0 {
		t.Errorf("max_per_chapter=9 → per_1000 6.0, got %v", got)
	}
}

func TestParseNormalizerJSON_GarbageFails(t *testing.T) {
	if body := llmcontract.ExtractJSONObject("模型只回了一句话，没有 JSON"); body != "" {
		t.Fatal("无 JSON 应解析失败（触发降级）")
	}
	if body := llmcontract.ExtractJSONObject("{ 不完整"); body != "" {
		t.Fatal("残缺 JSON 应解析失败")
	}
}

// 契约测试(RFC §11.1):根为 object、全属性(含嵌套 structured/fatigue_words 条目)required。
func TestNormalizeContractIsStrictReady(t *testing.T) {
	if normalizeContract.Schema["type"] != "object" {
		t.Fatal("根必须是 object")
	}
	if err := llmcontract.ValidateStrictReady(normalizeContract.Schema); err != nil {
		t.Fatal(err)
	}
}

func TestNormalize_NilModelErrors(t *testing.T) {
	// 无模型可用：返回明确错误，由 Service 层降级为 raw preferences。
	var n *Normalizer = NewNormalizer(nil)
	if _, err := n.Normalize(t.Context(), "startup_prompt", "mỗi chương 1200 chữ, nhân chính lạnh lùng"); err == nil {
		t.Fatal("无模型应返回错误")
	}
}

// scriptedModel 是最小 fake ChatModel：按调用次序吐预设回复，并记录最后一轮收到的
// messages，供断言反馈式重试是否把纠正提示并入了下一轮对话。回复用尽后重复最后一条。
type scriptedModel struct {
	replies  []string
	calls    int
	lastMsgs []llmtypes.Message
	lastCfg  llmtypes.CallConfig
	err      error // 非 nil 时 Generate 恒返回该错误
}

func (m *scriptedModel) Generate(_ context.Context, messages []llmtypes.Message, _ []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	var cfg llmtypes.CallConfig
	for _, o := range opts {
		o(&cfg)
	}
	m.lastCfg = cfg
	m.lastMsgs = messages
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	i := m.calls - 1
	if i >= len(m.replies) {
		i = len(m.replies) - 1
	}
	return &llmtypes.LLMResponse{Message: llmtypes.Message{
		Role:    llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{llmtypes.TextBlock(m.replies[i])},
	}}, nil
}

func (m *scriptedModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	return nil, nil
}

func (m *scriptedModel) SupportsTools() bool { return false }

// 反馈式重试：首轮吐坏 JSON、次轮才合法。Normalize 应成功，且次轮对话里带上了上一轮的
// 坏输出与纠正提示（反馈式，而非原样盲重试）。
func TestNormalize_FeedbackRetryRecovers(t *testing.T) {
	model := &scriptedModel{replies: []string{
		"这不是 JSON",
		`{"structured":{"genre":"","forbidden_chars":[],"forbidden_phrases":["某种程度上"],"fatigue_words":[]},"preferences":"","uncertain":[],"is_uncertain":false,"uncertain_note":null,"options":[]}`,
	}}
	n := NewNormalizer(model)

	cand, err := n.Normalize(t.Context(), "startup_prompt", "đừng để xuất hiện theo một nghĩa nào đó")
	if err != nil {
		t.Fatalf("次轮已返回合法 JSON，不应失败: %v", err)
	}
	if len(cand.Structured.ForbiddenPhrases) != 1 {
		t.Fatalf("应解析出 forbidden_phrases，got %+v", cand.Structured)
	}
	if model.calls != 2 {
		t.Fatalf("应在第 2 次成功，实际调用 %d 次", model.calls)
	}

	var sawBad, sawHint bool
	for _, msg := range model.lastMsgs {
		text := msg.TextContent()
		if text == "这不是 JSON" {
			sawBad = true
		}
		if strings.Contains(text, "JSON Schema") && strings.Contains(text, "Lỗi: ") {
			sawHint = true
		}
	}
	if !sawBad || !sawHint {
		t.Errorf("次轮应并入上一轮坏输出与纠正提示，sawBad=%v sawHint=%v", sawBad, sawHint)
	}
	system := model.lastMsgs[0].TextContent()
	if !strings.Contains(system, "<output-json-schema>") || !strings.Contains(system, `"fatigue_words"`) {
		t.Fatalf("prompt contract 应从 Contract 自动附加 schema:\n%s", system)
	}
}

// 归一化不覆盖模型的 thinking 默认；普通 chat 模型会拒绝显式 off。
func TestNormalize_LeavesThinkingUnspecifiedAndReservesTokens(t *testing.T) {
	model := &scriptedModel{replies: []string{`{"structured":{"genre":"","forbidden_chars":[],"forbidden_phrases":[],"fatigue_words":[]},"preferences":"x","uncertain":[],"is_uncertain":false,"uncertain_note":null,"options":[]}`}}
	n := NewNormalizer(model)

	if _, err := n.Normalize(t.Context(), "startup_prompt", "随便一条规则"); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if model.lastCfg.ThinkingLevel != llmtypes.ThinkingAuto {
		t.Errorf("不应发送 thinking 参数，got %q", model.lastCfg.ThinkingLevel)
	}
	if model.lastCfg.MaxTokens != normalizeMaxTokens {
		t.Errorf("max_tokens 应为 %d，got %d", normalizeMaxTokens, model.lastCfg.MaxTokens)
	}
}

// 全程坏 JSON：schema/业务重问有上限（默认 3），第 4 次调用仍失败即以契约失败返回。
func TestNormalize_FeedbackRetryStopsAfterCorrectionCap(t *testing.T) {
	model := &scriptedModel{replies: []string{"坏"}}
	n := NewNormalizer(model)

	_, err := n.Normalize(context.Background(), "startup_prompt", "mỗi chương 1200 chữ")
	var failure *llmcontract.Failure
	if !errors.As(err, &failure) || failure.Kind != llmcontract.FailureContract {
		t.Fatalf("自愈上限耗尽应返回契约失败，得 %T %v", err, err)
	}
	if model.calls != 4 {
		t.Fatalf("首次调用 + 3 次重问共 4 次，不应有第 5 次，实际 %d", model.calls)
	}
}

type terminalTestError struct{}

func (terminalTestError) Error() string   { return "401 authentication failed" }
func (terminalTestError) Retryable() bool { return false }

type retryableTestError struct{}

func (retryableTestError) Error() string             { return "provider unavailable" }
func (retryableTestError) Retryable() bool           { return true }
func (retryableTestError) RetryAfter() time.Duration { return time.Millisecond }

// 终止错误（401 等）不得盲重试：恰好 1 次调用即返回错误。
func TestNormalize_TerminalErrorStopsImmediately(t *testing.T) {
	model := &scriptedModel{err: terminalTestError{}}
	n := NewNormalizer(model)

	_, err := n.Normalize(t.Context(), "startup_prompt", "luật")
	if err == nil || !errors.As(err, &terminalTestError{}) {
		t.Fatalf("应透出终止错误: %v", err)
	}
	if model.calls != 1 {
		t.Fatalf("终止错误不应重试，实际调用 %d 次", model.calls)
	}
}

// retryable 请求错误由 llmretry 退避重试。
type flakyModel struct {
	scriptedModel
	failures int
}

func (m *flakyModel) Generate(ctx context.Context, msgs []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	if m.scriptedModel.calls < m.failures {
		m.scriptedModel.calls++
		return nil, retryableTestError{}
	}
	return m.scriptedModel.Generate(ctx, msgs, tools, opts...)
}

func TestNormalize_RetryableErrorRecovers(t *testing.T) {
	model := &flakyModel{
		scriptedModel: scriptedModel{replies: []string{`{"structured":{"genre":"","forbidden_chars":[],"forbidden_phrases":[],"fatigue_words":[]},"preferences":"x","uncertain":[],"is_uncertain":false,"uncertain_note":null,"options":[]}`}},
		failures:      2,
	}
	n := NewNormalizer(model)
	cand, err := n.Normalize(t.Context(), "startup_prompt", "luật")
	if err != nil || cand.Preferences != "x" {
		t.Fatalf("退避后应成功: %+v %v", cand, err)
	}
}

// nativeRulesModel 声明支持原生 JSON Schema。
type nativeRulesModel struct {
	*scriptedModel
}

func (m *nativeRulesModel) Capabilities() llmtypes.Capabilities {
	return llmtypes.Capabilities{
		Provider:   "openai",
		Model:      "gpt-test",
		Structured: llmtypes.StructuredCapabilities{JSONSchema: llmtypes.SupportYes, Strict: llmtypes.SupportYes},
	}
}

func TestNormalize_NativeSendsSchemaAndRejectsFences(t *testing.T) {
	// 原生模式：schema 进请求；裸 JSON 成功。
	model := &nativeRulesModel{&scriptedModel{replies: []string{
		`{"structured":{"genre":"","forbidden_chars":[],"forbidden_phrases":[],"fatigue_words":[]},"preferences":"x","uncertain":[],"is_uncertain":false,"uncertain_note":null,"options":[]}`,
	}}}
	n := NewNormalizer(model)
	cand, err := n.Normalize(t.Context(), "startup_prompt", "luật")
	if err != nil || cand.Preferences != "x" {
		t.Fatalf("native 归一化失败: %+v %v", cand, err)
	}
	rf := model.lastCfg.ResponseFormat
	if rf == nil || rf.JSONSchema == nil || rf.JSONSchema.Name != "userrules_normalize" {
		t.Fatalf("native 模式应发送 schema: %+v", rf)
	}
	if got := model.lastMsgs[0].TextContent(); got != normalizerSystemPrompt {
		t.Fatalf("native 模式不应向提示词重复注入 schema:\n%s", got)
	}

	// 围栏输出=契约违约：立即报错，不走 extractJSON、不重问。
	fenced := &nativeRulesModel{&scriptedModel{replies: []string{
		"```json\n{\"structured\":{},\"preferences\":\"x\",\"uncertain\":[]}\n```",
	}}}
	n = NewNormalizer(fenced)
	_, err = n.Normalize(t.Context(), "startup_prompt", "luật")
	if err == nil || !strings.Contains(err.Error(), "vi phạm hợp đồng") {
		t.Fatalf("mong đợi lỗi vi phạm hợp đồng, got %v", err)
	}
	if fenced.calls != 1 {
		t.Fatalf("契约违约不应重问，实际 %d 次", fenced.calls)
	}
}
