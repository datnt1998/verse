package llmcontract

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"errors"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/agent/schema"
)

type baseModel struct{}

func (baseModel) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return nil, nil
}

func (baseModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	return nil, nil
}

func (baseModel) SupportsTools() bool { return true }

type capsModel struct {
	baseModel
	caps llmtypes.Capabilities
}

func (m capsModel) Capabilities() llmtypes.Capabilities { return m.caps }

type overrideModel struct {
	capsModel
	override *bool
}

func (m overrideModel) JSONSchemaOverride() *bool { return m.override }

type infoModel struct {
	baseModel
	info llmtypes.ModelInfo
}

func (m infoModel) Info() llmtypes.ModelInfo { return m.info }

type factsModel struct {
	baseModel
	facts ModelFacts
}

func (m factsModel) StructuredOutputFacts() ModelFacts { return m.facts }

func structured(jsonSchema, strict llmtypes.Support) llmtypes.Capabilities {
	return llmtypes.Capabilities{
		Provider:   "prov",
		Model:      "mod",
		Structured: llmtypes.StructuredCapabilities{JSONSchema: jsonSchema, Strict: strict},
	}
}

func boolPtr(v bool) *bool { return &v }

// TestResolveMatrix 覆盖 config 三态 × adapter 能力的全部组合。
func TestResolveMatrix(t *testing.T) {
	cases := []struct {
		name       string
		model      llmtypes.ChatModel
		wantMode   Mode
		wantSource Source
		wantStrict bool
	}{
		{"无任何能力接口", baseModel{}, ModePromptContract, SourceUnknown, false},
		{"adapter yes + strict yes", capsModel{caps: structured(llmtypes.SupportYes, llmtypes.SupportYes)}, ModeNativeJSONSchema, SourceAdapter, true},
		{"adapter yes + strict no（Gemini 形态）", capsModel{caps: structured(llmtypes.SupportYes, llmtypes.SupportNo)}, ModeNativeJSONSchema, SourceAdapter, false},
		{"adapter yes + strict unknown", capsModel{caps: structured(llmtypes.SupportYes, llmtypes.SupportUnknown)}, ModeNativeJSONSchema, SourceAdapter, false},
		{"adapter no", capsModel{caps: structured(llmtypes.SupportNo, llmtypes.SupportNo)}, ModePromptContract, SourceAdapter, false},
		{"adapter unknown", capsModel{caps: structured(llmtypes.SupportUnknown, llmtypes.SupportUnknown)}, ModePromptContract, SourceUnknown, false},
		{"config true 无能力信息默认 strict", overrideModel{override: boolPtr(true)}, ModeNativeJSONSchema, SourceConfig, true},
		{"config true + adapter strict no 关 strict", overrideModel{capsModel{caps: structured(llmtypes.SupportUnknown, llmtypes.SupportNo)}, boolPtr(true)}, ModeNativeJSONSchema, SourceConfig, false},
		{"config false 覆盖 adapter yes", overrideModel{capsModel{caps: structured(llmtypes.SupportYes, llmtypes.SupportYes)}, boolPtr(false)}, ModePromptContract, SourceConfig, false},
		{"config 未配置回落 adapter", overrideModel{capsModel{caps: structured(llmtypes.SupportYes, llmtypes.SupportYes)}, nil}, ModeNativeJSONSchema, SourceAdapter, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Resolve(tc.model)
			if res.Mode != tc.wantMode || res.Source != tc.wantSource || res.Strict != tc.wantStrict {
				t.Fatalf("Resolve = %+v, want mode=%s source=%s strict=%v", res, tc.wantMode, tc.wantSource, tc.wantStrict)
			}
		})
	}
}

func TestResolveIdentity(t *testing.T) {
	res := Resolve(capsModel{caps: structured(llmtypes.SupportYes, llmtypes.SupportYes)})
	if res.Provider != "prov" || res.Model != "mod" {
		t.Fatalf("caps 身份未取到: %+v", res)
	}
	res = Resolve(infoModel{info: llmtypes.ModelInfo{Provider: "p2", Name: "m2"}})
	if res.Provider != "p2" || res.Model != "m2" {
		t.Fatalf("Info 兜底身份未取到: %+v", res)
	}
}

func TestResolveUsesAtomicFactsSnapshot(t *testing.T) {
	res := Resolve(factsModel{facts: ModelFacts{
		Capabilities:       structured(llmtypes.SupportYes, llmtypes.SupportYes),
		Info:               llmtypes.ModelInfo{Provider: "snapshot-provider", Name: "snapshot-model"},
		JSONSchemaOverride: boolPtr(false),
	}})
	if res.Mode != ModePromptContract || res.Source != SourceConfig {
		t.Fatalf("config false 快照应关闭原生 schema: %+v", res)
	}
	if res.Provider != "prov" || res.Model != "mod" {
		t.Fatalf("能力身份应来自同一快照: %+v", res)
	}
}

func testContract() Contract {
	return Contract{
		Name:        "test_decision",
		Description: "测试契约",
		Schema: schema.Object(
			schema.Property("action", schema.Enum("动作", "a", "b")).Required(),
			schema.Property("reason", schema.String("理由")).Required(),
		),
	}
}

func TestPlanNativeOptions(t *testing.T) {
	opts, res := Plan(capsModel{caps: structured(llmtypes.SupportYes, llmtypes.SupportYes)}, testContract())
	if res.Mode != ModeNativeJSONSchema || len(opts) == 0 {
		t.Fatalf("native 模式应产出 opts: res=%+v opts=%d", res, len(opts))
	}
	cfg := llmtypes.ResolveCallConfig(opts)
	rf := cfg.ResponseFormat
	if rf == nil || rf.Type != llmtypes.ResponseFormatJSONSchema || rf.JSONSchema == nil {
		t.Fatalf("ResponseFormat = %+v", rf)
	}
	if rf.JSONSchema.Name != "test_decision" || rf.JSONSchema.Strict == nil || !*rf.JSONSchema.Strict {
		t.Fatalf("JSONSchema = %+v", rf.JSONSchema)
	}
}

func TestPlanPromptContractNoOptions(t *testing.T) {
	opts, res := Plan(baseModel{}, testContract())
	if res.Mode != ModePromptContract || opts != nil {
		t.Fatalf("prompt 模式不应产出 opts: res=%+v opts=%v", res, opts)
	}
}

func TestPreparePromptUsesSchemaOnlyForPromptContract(t *testing.T) {
	contract := testContract()
	base := "只负责判断动作与理由。"

	prompt, err := PreparePrompt(base, contract, Resolution{Mode: ModePromptContract})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{base, "<output-json-schema>", `"action"`, `"required"`} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt contract 缺少 %q:\n%s", want, prompt)
		}
	}

	native, err := PreparePrompt(base, contract, Resolution{Mode: ModeNativeJSONSchema})
	if err != nil {
		t.Fatal(err)
	}
	if native != base {
		t.Fatalf("native 模式不应改写语义提示词: %q", native)
	}
}

func TestPreparePromptRejectsUnmarshalableSchema(t *testing.T) {
	_, err := PreparePrompt("semantic", Contract{
		Name:   "bad",
		Schema: map[string]any{"bad": make(chan int)},
	}, Resolution{Mode: ModePromptContract})
	if err == nil || !strings.Contains(err.Error(), "marshal bad prompt schema") {
		t.Fatalf("应暴露 schema 序列化错误, got %v", err)
	}
}

func TestNullableCopies(t *testing.T) {
	orig := schema.String("可空字段")
	out := Nullable(orig)
	got, ok := out["type"].([]string)
	if !ok || len(got) != 2 || got[0] != "string" || got[1] != "null" {
		t.Fatalf("Nullable type = %v", out["type"])
	}
	if orig["type"] != "string" {
		t.Fatalf("Nullable 修改了传入 map: %v", orig["type"])
	}
}

func TestNullableExtendsEnumWithNull(t *testing.T) {
	orig := schema.Enum("可空枚举", "a", "b")
	out := Nullable(orig)
	enum, ok := out["enum"].([]any)
	if !ok || len(enum) != 3 || enum[2] != nil {
		t.Fatalf("Nullable enum = %#v", out["enum"])
	}
	if _, ok := orig["enum"].([]string); !ok {
		t.Fatalf("Nullable 修改了传入 enum: %#v", orig["enum"])
	}
	if err := ValidateJSON(out, []byte("null")); err != nil {
		t.Fatalf("可空枚举应接受 null: %v", err)
	}
	if err := ValidateJSON(out, []byte(`"other"`)); err == nil {
		t.Fatal("可空枚举不应接受枚举外字符串")
	}
}

func TestFingerprintStable(t *testing.T) {
	a, b := testContract(), testContract()
	if a.Fingerprint() != b.Fingerprint() || len(a.Fingerprint()) != 12 {
		t.Fatalf("同一契约 fingerprint 应稳定: %s vs %s", a.Fingerprint(), b.Fingerprint())
	}
	c := testContract()
	c.Schema = schema.Object(schema.Property("other", schema.String("x")).Required())
	if a.Fingerprint() == c.Fingerprint() {
		t.Fatalf("不同 schema fingerprint 不应相同")
	}
}

func TestValidateJSONEnforcesStrictContract(t *testing.T) {
	contract := testContract()
	if err := ValidateJSON(contract.Schema, []byte(`{"action":"a","reason":"ok"}`)); err != nil {
		t.Fatalf("合法 JSON 被拒绝: %v", err)
	}
	for name, raw := range map[string]string{
		"缺 required": `{"action":"a"}`,
		"非法 enum":    `{"action":"other","reason":"x"}`,
		"字段类型错误":     `{"action":1,"reason":"x"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateJSON(contract.Schema, []byte(raw)); err == nil {
				t.Fatalf("应拒绝 %s", raw)
			}
		})
	}
}

func TestValidateJSONRejectsInvalidEnumContract(t *testing.T) {
	contract := testContract()
	contract.Schema["enum"] = []any{1}
	if err := ValidateJSON(contract.Schema, []byte(`{"action":"a","reason":"ok"}`)); err == nil || !strings.Contains(err.Error(), "enum: hợp đồng không hợp lệ") {
		t.Fatalf("应暴露非法 enum 契约，err=%v", err)
	}
}

func TestExtractJSONObjectBalanced(t *testing.T) {
	raw := "前缀 ```json\n{\"a\":\"}\",\"nested\":{\"b\":1}}\n``` 尾部"
	want := `{"a":"}","nested":{"b":1}}`
	if got := ExtractJSONObject(raw); got != want {
		t.Fatalf("ExtractJSONObject = %q, want %q", got, want)
	}
}

type executionModel struct {
	responses []string
	stops     []llmtypes.StopReason
	calls     int
	messages  []llmtypes.Message
	config    llmtypes.CallConfig
}

func (m *executionModel) Generate(_ context.Context, messages []llmtypes.Message, _ []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.messages = messages
	m.config = llmtypes.ResolveCallConfig(opts)
	response := m.responses[m.calls]
	var stop llmtypes.StopReason
	if m.calls < len(m.stops) {
		stop = m.stops[m.calls]
	}
	m.calls++
	return &llmtypes.LLMResponse{Message: llmtypes.Message{
		Role:       llmtypes.RoleAssistant,
		Content:    []llmtypes.ContentBlock{llmtypes.TextBlock(response)},
		StopReason: stop,
	}}, nil
}

type nativeExecutionModel struct{ *executionModel }

func (m *nativeExecutionModel) Capabilities() llmtypes.Capabilities {
	return structured(llmtypes.SupportYes, llmtypes.SupportYes)
}

func TestExecutePromptModeSelfHealsSchemaViolation(t *testing.T) {
	model := &executionModel{responses: []string{
		`{}`,
		`{"action":"a","reason":"fixed"}`,
	}}
	type output struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	out, err := Execute(t.Context(), model, Request[output]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入", Agent: "test",
	})
	if err != nil || out.Reason != "fixed" {
		t.Fatalf("自愈失败: %+v %v", out, err)
	}
	if model.calls != 2 {
		t.Fatalf("应在第二次成功，calls=%d", model.calls)
	}
	if len(model.messages) != 4 || !strings.Contains(model.messages[3].TextContent(), "$.action") {
		t.Fatalf("重问未携带精确 Schema 错误: %+v", model.messages)
	}
}

func TestExecuteNativeSchemaViolationFailsImmediately(t *testing.T) {
	model := &nativeExecutionModel{executionModel: &executionModel{responses: []string{`{}`}}}
	_, err := Execute(t.Context(), model, Request[map[string]any]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入",
	})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureContract {
		t.Fatalf("应返回原生契约错误，得 %T %v", err, err)
	}
	if model.calls != 1 {
		t.Fatalf("原生契约违约不应重问，calls=%d", model.calls)
	}
}

func TestExecuteExposesModelErrorStopReason(t *testing.T) {
	model := &executionModel{
		responses: []string{`{"action":"a","reason":"unused"}`},
		stops:     []llmtypes.StopReason{llmtypes.StopReasonError},
	}
	_, err := Execute(t.Context(), model, Request[map[string]any]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入",
	})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureProtocol || !strings.Contains(err.Error(), "stop_reason=error") {
		t.Fatalf("应暴露模型错误终止原因，得 %T %v", err, err)
	}
	if model.calls != 1 {
		t.Fatalf("错误终止不应作为 JSON 错误重问，calls=%d", model.calls)
	}
}

func TestExecutePromptModeStopsAfterCorrectionCap(t *testing.T) {
	model := &executionModel{responses: []string{
		`{}`, `{}`, `{}`, `{}`, `{"action":"a","reason":"never reached"}`,
	}}
	_, err := Execute(t.Context(), model, Request[map[string]any]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入", Agent: "test",
	})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureContract {
		t.Fatalf("超上限应返回契约失败，得 %T %v", err, err)
	}
	if failure.Raw != "{}" {
		t.Fatalf("失败应携带最后一次原始输出，得 %q", failure.Raw)
	}
	if model.calls != 4 {
		t.Fatalf("首次调用 + 3 次重问共 4 次，不应有第 5 次，calls=%d", model.calls)
	}
}

func TestExecuteRespectsMaxCorrectionsOverride(t *testing.T) {
	model := &executionModel{responses: []string{`{}`, `{}`, `{"action":"a","reason":"unused"}`}}
	_, err := Execute(t.Context(), model, Request[map[string]any]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入", Agent: "test",
		MaxCorrections: 1,
	})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureContract {
		t.Fatalf("超上限应返回契约失败，得 %T %v", err, err)
	}
	if model.calls != 2 {
		t.Fatalf("MaxCorrections=1 只允许 1 次重问，calls=%d", model.calls)
	}
}

func TestExecuteStripsLeadingThinkBlock(t *testing.T) {
	model := &executionModel{responses: []string{
		`  <think>先想清楚</think>
<THINKING>再核对一遍</THINKING> {"action":"a","reason":"fixed"}`,
	}}
	type output struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	out, err := Execute(t.Context(), model, Request[output]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入", Agent: "test",
	})
	if err != nil || out.Reason != "fixed" {
		t.Fatalf("剥掉开头 think 块后应直接成功: %+v %v", out, err)
	}
	if model.calls != 1 {
		t.Fatalf("think 块被剥掉不应触发重问，calls=%d", model.calls)
	}
}

func TestExecuteUnclosedThinkTagAsksModelToFix(t *testing.T) {
	model := &executionModel{responses: []string{
		`<think>越想越偏`,
		`{"action":"a","reason":"fixed"}`,
	}}
	type output struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	out, err := Execute(t.Context(), model, Request[output]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入", Agent: "test",
	})
	if err != nil || out.Reason != "fixed" {
		t.Fatalf("协议错误反馈后应自愈: %+v %v", out, err)
	}
	if model.calls != 2 {
		t.Fatalf("缺闭标签应重问一次，calls=%d", model.calls)
	}
	if len(model.messages) != 4 || !strings.Contains(model.messages[3].TextContent(), "<think>") {
		t.Fatalf("重问应携带协议错误原因: %+v", model.messages)
	}
}
func TestExecuteStripsMultilineThinkBlock(t *testing.T) {
	model := &executionModel{responses: []string{
		"<think>\nreasoning line one\nreasoning line two\n</think>{\"action\":\"a\",\"reason\":\"fixed\"}",
	}}
	type output struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	out, err := Execute(t.Context(), model, Request[output]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入", Agent: "test",
	})
	if err != nil || out.Reason != "fixed" {
		t.Fatalf("多行 think 块剥掉后应首轮成功: %+v %v", out, err)
	}
	if model.calls != 1 {
		t.Fatalf("多行 think 块不应触发重问，calls=%d", model.calls)
	}
}

func TestExecuteNativeUnclosedThinkTagAsksModelToFix(t *testing.T) {
	model := &nativeExecutionModel{executionModel: &executionModel{responses: []string{
		`<THINKING>没闭合`,
		`{"action":"a","reason":"fixed"}`,
	}}}
	type output struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	out, err := Execute(t.Context(), model, Request[output]{
		Contract: testContract(), SystemPrompt: "判断。", Payload: "输入", Agent: "test",
	})
	if err != nil || out.Reason != "fixed" {
		t.Fatalf("原生模式缺闭标签也应重问修复: %+v %v", out, err)
	}
	if model.calls != 2 {
		t.Fatalf("calls=%d", model.calls)
	}
}

func TestStripLeadingThink(t *testing.T) {
	for _, c := range []struct {
		name    string
		raw     string
		rest    string
		wantErr bool
	}{
		{name: "no tag", raw: `{"a":1}`, rest: `{"a":1}`},
		{name: "leading whitespace", raw: "  \n<think>x</think> {\"a\":1}", rest: " {\"a\":1}"},
		{name: "thinking variant", raw: "<thinking>x</thinking>y", rest: "y"},
		{name: "case insensitive", raw: "<ThInK>x</ThInK>y", rest: "y"},
		{name: "multiple blocks", raw: "<think>a</think>  <think>b</think> {\"a\":1}", rest: " {\"a\":1}"},
		{name: "attrs on open tag", raw: `<think lang="vi">x</think>y`, rest: "y"},
		{name: "tag mid text not stripped", raw: `前言 <think>x</think> {"a":1}`, rest: `前言 <think>x</think> {"a":1}`},
		{name: "unclosed tag", raw: "<think> blah", wantErr: true},
		{name: "unclosed with attrs", raw: "<think lang=vi> blah", wantErr: true},
		{name: "closed then unclosed", raw: "<think>a</think> <think>b", wantErr: true},
		{name: "json lookalike untouched", raw: `{"t":"<think>x</think>"}`, rest: `{"t":"<think>x</think>"}`},
		{name: "thinkword not a tag", raw: "thinker {\"a\":1}", rest: "thinker {\"a\":1}"},
	} {
		rest, err := stripLeadingThink(c.raw)
		if c.wantErr {
			if err == nil {
				t.Fatalf("%s: 应报协议错误，得 rest=%q", c.name, rest)
			}
			continue
		}
		if err != nil || rest != c.rest {
			t.Fatalf("%s: 得 rest=%q err=%v", c.name, rest, err)
		}
	}
}
