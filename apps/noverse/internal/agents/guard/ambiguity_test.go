package guard

import (
	"context"
	"testing"

	agentcore "github.com/datnt1998/noverse/internal/agent"
	llmtypes "github.com/datnt1998/noverse/internal/llm"
)

func TestWorkerAmbiguityHandsBackToArbiter(t *testing.T) {
	for _, text := range []string{
		"Cần Trọng tài: Giữ cả hai cảnh hoặc cắt cho khớp đại cương. Khuyến nghị cắt.",
		"Đã đọc chương. Bạn muốn tôi giữ cả hai cảnh hay cắt cho khớp?",
	} {
		t.Run(text, func(t *testing.T) {
			g := NewWriterStopGuard(newTestStore(t), nil)
			d := g(context.Background(), agentcore.StopInfo{Message: llmtypes.Message{
				Role: llmtypes.RoleAssistant, StopReason: llmtypes.StopReasonStop,
				Content: []llmtypes.ContentBlock{llmtypes.TextBlock(text)},
			}})
			if !d.Allow {
				t.Fatalf("ambiguity must return to arbiter rather than force an unauthorized commit: %+v", d)
			}
		})
	}
}
