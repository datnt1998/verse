package host

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"testing"
)

type plainTrackedTestModel struct{}

func (*plainTrackedTestModel) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return &llmtypes.LLMResponse{}, nil
}

func (*plainTrackedTestModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	ch := make(chan llmtypes.StreamEvent)
	close(ch)
	return ch, nil
}

func (*plainTrackedTestModel) SupportsTools() bool { return true }

type capableTrackedTestModel struct {
	*plainTrackedTestModel
	caps llmtypes.Capabilities
}

func (m *capableTrackedTestModel) Capabilities() llmtypes.Capabilities { return m.caps }

func TestUsageTrackedModelPreservesOptionalCapabilities(t *testing.T) {
	want := llmtypes.Capabilities{
		Provider: "openai",
		Model:    "gpt-chat",
		Thinking: llmtypes.ThinkingCapabilities{Supported: llmtypes.SupportNo},
	}
	inner := &capableTrackedTestModel{plainTrackedTestModel: &plainTrackedTestModel{}, caps: want}
	wrapped := newUsageTrackedModel(inner, "arbiter", func(string, string, llmtypes.AgentMessage) {})
	cp, ok := wrapped.(llmtypes.CapabilityProvider)
	if !ok {
		t.Fatal("usage wrapper dropped CapabilityProvider")
	}
	if got := cp.Capabilities(); got.Provider != want.Provider || got.Model != want.Model || got.Thinking.Supported != llmtypes.SupportNo {
		t.Fatalf("capabilities changed through wrapper: %+v", got)
	}

	plain := newUsageTrackedModel(&plainTrackedTestModel{}, "arbiter", func(string, string, llmtypes.AgentMessage) {})
	if _, ok := plain.(llmtypes.CapabilityProvider); ok {
		t.Fatal("wrapper must not invent capabilities for an unknown model")
	}
}

type overrideCapableTestModel struct {
	*capableTrackedTestModel
	override *bool
}

func (m *overrideCapableTestModel) JSONSchemaOverride() *bool { return m.override }

// usage 包装器必须透传 config json_schema 覆盖值；inner 未携带时返回 nil
// （"未配置"），不伪造能力。
func TestUsageTrackedModelForwardsJSONSchemaOverride(t *testing.T) {
	tr := true
	inner := &overrideCapableTestModel{
		capableTrackedTestModel: &capableTrackedTestModel{plainTrackedTestModel: &plainTrackedTestModel{}},
		override:                &tr,
	}
	wrapped := newUsageTrackedModel(inner, "arbiter", func(string, string, llmtypes.AgentMessage) {})
	o, ok := wrapped.(interface{ JSONSchemaOverride() *bool })
	if !ok {
		t.Fatal("usage wrapper dropped JSONSchemaOverride")
	}
	if v := o.JSONSchemaOverride(); v == nil || !*v {
		t.Fatalf("override không được truyền qua: %v", v)
	}

	capsOnly := newUsageTrackedModel(&capableTrackedTestModel{plainTrackedTestModel: &plainTrackedTestModel{}}, "arbiter", func(string, string, llmtypes.AgentMessage) {})
	o, ok = capsOnly.(interface{ JSONSchemaOverride() *bool })
	if !ok {
		t.Fatal("capability wrapper should expose JSONSchemaOverride")
	}
	if v := o.JSONSchemaOverride(); v != nil {
		t.Fatalf("inner khi không có override phải là nil: %v", v)
	}
}
