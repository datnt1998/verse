package sim

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"strings"
	"testing"

	"github.com/datnt1998/verse/core/llmcontract"
)

func TestSimulationContractsAreStrictReady(t *testing.T) {
	for _, contract := range []llmcontract.Contract{sourceReportContract, synthesisContract} {
		if err := llmcontract.ValidateStrictReady(contract.Schema); err != nil {
			t.Fatalf("%s: %v", contract.Name, err)
		}
	}
}

type nativeSimulationModel struct {
	response string
	messages []llmtypes.Message
	config   llmtypes.CallConfig
}

func (m *nativeSimulationModel) Capabilities() llmtypes.Capabilities {
	return llmtypes.Capabilities{
		Provider:   "openai",
		Model:      "gpt-test",
		Structured: llmtypes.StructuredCapabilities{JSONSchema: llmtypes.SupportYes, Strict: llmtypes.SupportYes},
	}
}

func (m *nativeSimulationModel) Generate(_ context.Context, messages []llmtypes.Message, _ []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.messages = messages
	m.config = llmtypes.ResolveCallConfig(opts)
	return &llmtypes.LLMResponse{Message: llmtypes.Message{
		Role:       llmtypes.RoleAssistant,
		Content:    []llmtypes.ContentBlock{llmtypes.TextBlock(m.response)},
		StopReason: llmtypes.StopReasonStop,
	}}, nil
}

func TestAnalyzeSourceUsesNativeSchema(t *testing.T) {
	model := &nativeSimulationModel{response: validSourceReportJSON("Tóm tắt rõ ràng")}
	report, err := AnalyzeSource(t.Context(), model, "Chỉ phân tích phương pháp viết.", scannedSource{})
	if err != nil {
		t.Fatalf("AnalyzeSource: %v", err)
	}
	if report.Summary == "" {
		t.Fatal("summary rỗng")
	}
	format := model.config.ResponseFormat
	if format == nil || format.JSONSchema == nil || format.JSONSchema.Name != sourceReportContract.Name {
		t.Fatalf("response format = %#v", format)
	}
	if strings.Contains(model.messages[0].TextContent(), "<output-json-schema>") {
		t.Fatalf("native prompt không nên chèn schema: %s", model.messages[0].TextContent())
	}
}

func TestAnalyzeSourcePromptModeRepairsMissingRequiredFields(t *testing.T) {
	model := &scriptedLLM{responses: []string{
		`{}`,
		validSourceReportJSON("Tóm tắt đã sửa"),
	}}
	report, err := AnalyzeSource(t.Context(), model, "Chỉ phân tích phương pháp viết.", scannedSource{})
	if err != nil {
		t.Fatalf("AnalyzeSource: %v", err)
	}
	if report.Summary != "Tóm tắt đã sửa" || model.calls.Load() != 2 {
		t.Fatalf("Thiếu trường thì phải phản hồi tự phục hồi: report=%+v calls=%d", report, model.calls.Load())
	}
}

var _ LLMChat = (*nativeSimulationModel)(nil)
