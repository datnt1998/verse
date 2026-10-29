package flow

import (
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"testing"
)

func TestRecheckRoutingPriorityAndHold(t *testing.T) {
	p := &domain.Progress{Phase: domain.PhaseWriting, Flow: domain.FlowRewriting, PendingRewrites: []int{1}, PendingRechecks: []int{2}, CompletedChapters: []int{1, 2}, TotalChapters: 5}
	state := State{Progress: p}
	if inst := Route(state); inst == nil || inst.Agent != "writer" || inst.Chapter != 1 {
		t.Fatalf("rewrite priority: %+v", inst)
	}
	p.PendingRewrites = nil
	if inst := Route(state); inst == nil || inst.Agent != "editor" || inst.Chapter != 2 {
		t.Fatalf("recheck priority: %+v", inst)
	}
	hold := &domain.AdvanceHold{After: domain.AdvanceHoldAfterRewritesDrained, Reason: "kiểm xong mới dừng"}
	if got, err := ResolveAdvanceHold(hold, p); err != nil || got != AdvanceHoldKeep {
		t.Fatalf("hold released recheck: %v %v", got, err)
	}
	if StartsForwardChapter(&Instruction{Agent: "writer", Chapter: 3}, p, nil) {
		t.Fatal("pending recheck counted as forward writing")
	}
	p.PendingRechecks = nil
	p.Flow = domain.FlowWriting
	if got, err := ResolveAdvanceHold(hold, p); err != nil || got != AdvanceHoldConsumeAndStop {
		t.Fatalf("drained hold: %v %v", got, err)
	}
}
