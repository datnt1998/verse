package host

import (
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
	"testing"
)

func TestRoundTitleStructuredState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		inst  flow.Instruction
		state flow.State
		want  string
	}{
		{"missing", flow.Instruction{Agent: "writer", Chapter: 2}, flow.State{}, ""},
		{"write", flow.Instruction{Agent: "writer", Chapter: 2}, flow.State{Progress: &domain.Progress{Phase: domain.PhaseWriting}}, "Viết chương 2"},
		{"rewrite", flow.Instruction{Agent: "writer", Chapter: 2}, flow.State{Progress: &domain.Progress{Phase: domain.PhaseWriting, PendingRewrites: []int{2}}}, "Viết lại chương 2"},
		{"planning", flow.Instruction{Agent: "architect_long"}, flow.State{Progress: &domain.Progress{Phase: domain.PhaseInit}}, "Lập kế hoạch truyện"},
		{"audit", flow.Instruction{Agent: "architect_long"}, flow.State{Progress: &domain.Progress{Phase: domain.PhaseOutline}, FoundationMissing: []string{"foundation_audit"}}, "Soát nền truyện"},
		{"feedback", flow.Instruction{Agent: "architect_long"}, flow.State{Progress: &domain.Progress{Phase: domain.PhaseWriting}, ImmediateFeedbackCount: 1}, "Xem góp ý dàn ý"},
		{"review", flow.Instruction{Agent: "editor", Chapter: 4}, flow.State{Progress: &domain.Progress{Phase: domain.PhaseWriting}}, "Thẩm chương 4"},
		{"arc", flow.Instruction{Agent: "editor"}, flow.State{Progress: &domain.Progress{Phase: domain.PhaseWriting}, AggregateRefresh: &flow.AggregateRefresh{Kind: flow.AggregateArcSummary, Arc: 3}}, "Tóm tắt cung 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := roundTitle(&tc.inst, tc.state); got != tc.want {
				t.Fatalf("title = %q, want %q", got, tc.want)
			}
		})
	}
}
