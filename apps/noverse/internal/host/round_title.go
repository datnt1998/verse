package host

import (
	"fmt"
	"strings"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
)

func roundTitle(inst *flow.Instruction, state flow.State) string {
	if inst == nil || state.Progress == nil { return "" }
	p := state.Progress
	if strings.HasPrefix(inst.Agent, "architect") {
		if p.Phase != domain.PhaseWriting {
			if len(state.FoundationMissing) == 1 && state.FoundationMissing[0] == "foundation_audit" { return "Soát nền truyện" }
			return "Lập kế hoạch truyện"
		}
		if state.ImmediateFeedbackCount > 0 { return "Xem góp ý dàn ý" }
	}
	if inst.Agent == "writer" && inst.Chapter > 0 && p.Phase == domain.PhaseWriting {
		if len(p.PendingRewrites)>0 && p.PendingRewrites[0] == inst.Chapter { return fmt.Sprintf("Viết lại chương %d", inst.Chapter) }
		return fmt.Sprintf("Viết chương %d", inst.Chapter)
	}
	if inst.Agent == "editor" && p.Phase == domain.PhaseWriting {
		if state.AggregateRefresh != nil && state.AggregateRefresh.Kind == flow.AggregateArcSummary && state.AggregateRefresh.Arc > 0 { return fmt.Sprintf("Tóm tắt cung %d",state.AggregateRefresh.Arc) }
		if state.ArcBoundary != nil && state.ArcBoundary.IsArcEnd && state.HasArcReview && !state.HasArcSummary && state.ArcBoundary.Arc > 0 { return fmt.Sprintf("Tóm tắt cung %d",state.ArcBoundary.Arc) }
		if inst.Chapter > 0 { return fmt.Sprintf("Thẩm chương %d",inst.Chapter) }
	}
	return ""
}

// NormalizeRole collapses internal architect variants to the board role.
func NormalizeRole(role string) string {
	if role == "architect_long" || role == "architect_short" { return "architect" }
	return role
}
