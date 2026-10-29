package imp

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
)

func TestStructuredContractsAreStrictReady(t *testing.T) {
	for _, contract := range []llmcontract.Contract{segmentContract, analysisContract, rangeContract, synthesisContract} {
		if err := llmcontract.ValidateStrictReady(contract.Schema); err != nil {
			t.Fatalf("%s: %v", contract.Name, err)
		}
	}
}

// 导入契约必须能表达"放弃这条伏笔线"：abandon 进枚举，due_by / reason 可空但在
// strict 模式下必须出现，否则模型没有字段可填，重放会永远缺这两个事实。
func TestChapterFactsSchemaCarriesForeshadowAbandon(t *testing.T) {
	props, _ := chapterFactsSchema()["properties"].(map[string]any)
	arr, _ := props["foreshadow_updates"].(map[string]any)
	items, _ := arr["items"].(map[string]any)
	itemProps, _ := items["properties"].(map[string]any)

	action, _ := itemProps["action"].(map[string]any)
	values, _ := action["enum"].([]string)
	if !slices.Contains(values, "abandon") {
		t.Fatalf("action enum = %v, want abandon", values)
	}
	if got := fmt.Sprint(itemProps["due_by"].(map[string]any)["type"]); got != "[integer null]" {
		t.Fatalf("due_by type = %s, want nullable integer", got)
	}
	if got := fmt.Sprint(itemProps["reason"].(map[string]any)["type"]); got != "[string null]" {
		t.Fatalf("reason type = %s, want nullable string", got)
	}
}

type nativeImportModel struct {
	*mockModel
	messages []llmtypes.Message
	config   llmtypes.CallConfig
}

func (m *nativeImportModel) Capabilities() llmtypes.Capabilities {
	return llmtypes.Capabilities{
		Provider:   "openai",
		Model:      "gpt-test",
		Structured: llmtypes.StructuredCapabilities{JSONSchema: llmtypes.SupportYes, Strict: llmtypes.SupportYes},
	}
}

func (m *nativeImportModel) Generate(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.messages = messages
	m.config = llmtypes.ResolveCallConfig(opts)
	return m.mockModel.Generate(ctx, messages, tools, opts...)
}

func TestCallStructuredUsesNativeSchemaWithoutPromptDuplication(t *testing.T) {
	model := &nativeImportModel{mockModel: &mockModel{responses: []string{`{"boundaries":[]}`}}}
	const prompt = "判断真实边界。"
	_, err := callStructured[boundaryBatch](t.Context(), model, segmentContract, prompt, `{}`, 100, callProfile{}, nil)
	if err != nil {
		t.Fatalf("callStructured: %v", err)
	}
	format := model.config.ResponseFormat
	if format == nil || format.JSONSchema == nil || format.JSONSchema.Name != segmentContract.Name {
		t.Fatalf("response format = %#v", format)
	}
	if got := model.messages[0].TextContent(); got != prompt {
		t.Fatalf("native prompt 被重复注入 schema: %s", got)
	}
}

func TestCallStructuredPromptModeInjectsContract(t *testing.T) {
	model := &nativeImportModel{mockModel: &mockModel{responses: []string{`{"boundaries":[]}`}}}
	modelCaps := &promptImportModel{nativeImportModel: model}
	_, err := callStructured[boundaryBatch](t.Context(), modelCaps, segmentContract, "判断真实边界。", `{}`, 100, callProfile{}, nil)
	if err != nil {
		t.Fatalf("callStructured: %v", err)
	}
	if model.config.ResponseFormat != nil {
		t.Fatalf("prompt mode 不应发送 response_format: %#v", model.config.ResponseFormat)
	}
	if !strings.Contains(model.messages[0].TextContent(), "<output-json-schema>") {
		t.Fatalf("prompt mode 未注入契约: %s", model.messages[0].TextContent())
	}
}

type promptImportModel struct{ *nativeImportModel }

func (m *promptImportModel) Capabilities() llmtypes.Capabilities { return llmtypes.Capabilities{} }
