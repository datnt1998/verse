package store

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

func TestRecheckAtomicTransitionsAndInvalidation(t *testing.T) {
	s := NewStore(t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Init())
	must(s.Progress.Init(4))
	must(s.Progress.UpdatePhase(domain.PhaseWriting))
	for _, ch := range []int{1, 2} {
		must(s.Progress.MarkChapterComplete(ch, 100, "", ""))
	}
	_, err := s.Progress.ApplyReviewOutcome(domain.FlowRewriting, []int{1, 2}, "lỗi", RewriteCauseReview, 0)
	must(err)
	must(s.Progress.CompleteRewrite(1, true))
	must(s.Progress.CompleteRewrite(1, true))
	p, err := s.Progress.Load()
	must(err)
	if len(p.PendingRechecks) != 1 || len(p.PendingRewrites) != 1 || p.Flow != domain.FlowRewriting {
		t.Fatalf("transition not atomic/idempotent: %+v", p)
	}
	must(s.Progress.CompleteRewrite(2, false))
	p, err = s.Progress.Load()
	must(err)
	if p.ReviewWorkDrained() || p.Flow == domain.FlowWriting {
		t.Fatalf("pending recheck released writing: %+v", p)
	}
	if err := s.Progress.MarkComplete(); err == nil {
		t.Fatal("completed with pending recheck")
	}
	p, err = s.Progress.ApplyReviewOutcome(domain.FlowRewriting, []int{1}, "unfixed", RewriteCauseReview, 1)
	must(err)
	if len(p.PendingRechecks) != 0 || len(p.PendingRewrites) != 1 {
		t.Fatalf("recheck-to-rewrite not atomic: %+v", p)
	}
	must(s.Progress.CompleteRewrite(1, true))
	must(s.Progress.DropRechecksFrom(1))
	p, err = s.Progress.Load()
	must(err)
	if !p.ReviewWorkDrained() || p.Flow != domain.FlowWriting {
		t.Fatalf("orphan recheck survived: %+v", p)
	}
}

func TestLegacyProgressWithoutRechecks(t *testing.T) {
	var p domain.Progress
	if err := json.Unmarshal([]byte(`{"phase":"writing","flow":"writing","completed_chapters":[1]}`), &p); err != nil {
		t.Fatal(err)
	}
	if !p.ReviewWorkDrained() {
		t.Fatalf("legacy queue not drained: %+v", p)
	}
}

// recheckModeStore 搭好"返工完一轮、等待复查"的状态：from 是 CompleteRewrite
// 入队复查时保留的返工模式，chapter 已重写完毕并挂进 PendingRechecks。
func recheckModeStore(t *testing.T, from domain.FlowState) *Store {
	t.Helper()
	s := NewStore(t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Init())
	must(s.Progress.Init(4))
	for _, ch := range []int{1, 2, 3} {
		must(s.Progress.MarkChapterComplete(ch, 100, "", ""))
	}
	must(s.Progress.SetPendingRewrites([]int{1, 2}, "返工", RewriteCauseReview))
	must(s.Progress.SetFlow(from))
	must(s.Progress.CompleteRewrite(1, true))
	return s
}

// TestRecheckVerdictMaySwitchReworkMode 防回归：CompleteRewrite(ch,true) 把章节
// 挂进 PendingRechecks 时保留 rewriting/polishing，而复查裁定可以合法地改用另一种
// 返工模式。未修复时 ApplyReviewOutcome 拒绝 rewriting↔polishing，review 已落盘但
// 进度写不进，同样的 review 重试也永远失败——形成死路。
func TestRecheckVerdictMaySwitchReworkMode(t *testing.T) {
	cases := []struct {
		name     string
		from, to domain.FlowState
	}{
		{"rewriting→polishing", domain.FlowRewriting, domain.FlowPolishing},
		{"polishing→rewriting", domain.FlowPolishing, domain.FlowRewriting},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := recheckModeStore(t, tc.from)
			p, err := s.Progress.ApplyReviewOutcome(tc.to, []int{1}, "复查需换模式", RewriteCauseReview, 1)
			if err != nil {
				t.Fatalf("queued recheck mode switch rejected: %v", err)
			}
			if p.Flow != tc.to {
				t.Fatalf("flow = %s, want %s", p.Flow, tc.to)
			}
			if len(p.PendingRechecks) != 0 {
				t.Fatalf("recheck not dequeued: %+v", p.PendingRechecks)
			}
			// 队列是合并而非覆盖：未完成的 ch2 必须保留，ch1 重新入队。
			if len(p.PendingRewrites) != 2 || p.PendingRewrites[0] != 2 || p.PendingRewrites[1] != 1 {
				t.Fatalf("pending rewrites = %v, want [2 1]", p.PendingRewrites)
			}
		})
	}
}

// TestRecheckModeSwitchKeepsOtherQueues 防回归：模式切换豁免不得吞掉同一章之外的
// 未完成工作。另一章挂起复查时，本次复查仍可换模式，且另一条 PendingRechecks 保留。
func TestRecheckModeSwitchKeepsOtherQueues(t *testing.T) {
	s := recheckModeStore(t, domain.FlowRewriting)
	if err := s.Progress.CompleteRewrite(2, true); err != nil {
		t.Fatal(err)
	}
	p, err := s.Progress.ApplyReviewOutcome(domain.FlowPolishing, []int{1}, "复查需打磨", RewriteCauseReview, 1)
	if err != nil {
		t.Fatalf("mode switch rejected with sibling recheck pending: %v", err)
	}
	if p.Flow != domain.FlowPolishing {
		t.Fatalf("flow = %s, want polishing", p.Flow)
	}
	if len(p.PendingRechecks) != 1 || p.PendingRechecks[0] != 2 {
		t.Fatalf("sibling recheck lost: %+v", p.PendingRechecks)
	}
	if p.ReviewWorkDrained() {
		t.Fatalf("drained while ch2 still pending recheck: %+v", p)
	}
}

// TestSpuriousRecheckCannotSwitchReworkMode 防回归：recheckedChapter>0 本身不构成
// 放行依据。章节不在 PendingRechecks 中时，rewriting↔polishing 仍是非法迁移，且任何
// 校验失败都必须保持队列与 flow 原状（同一次写锁内先校验后写）。
func TestSpuriousRecheckCannotSwitchReworkMode(t *testing.T) {
	s := recheckModeStore(t, domain.FlowRewriting)
	p, err := s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	// ch3 已完成但从未进入 PendingRechecks：伪造的 recheckedChapter。
	if _, err := s.Progress.ApplyReviewOutcome(domain.FlowPolishing, []int{3}, "伪复查", RewriteCauseReview, 3); !errors.Is(err, errs.ErrFlowTransition) {
		t.Fatalf("spurious recheck bypassed flow check: %v", err)
	}
	// 真正的复查章是 1，但调用方谎报 3：同样不得放行。
	if _, err := s.Progress.ApplyReviewOutcome(domain.FlowPolishing, []int{1}, "错挂复查", RewriteCauseReview, 3); !errors.Is(err, errs.ErrFlowTransition) {
		t.Fatalf("mismatched recheck bypassed flow check: %v", err)
	}
	// 普通复审（recheckedChapter=0）的换模式仍然非法。
	if _, err := s.Progress.ApplyReviewOutcome(domain.FlowPolishing, []int{1}, "普通复审", RewriteCauseReview, 0); !errors.Is(err, errs.ErrFlowTransition) {
		t.Fatalf("normal review bypassed flow check: %v", err)
	}
	after, err := s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if after.Flow != p.Flow || len(after.PendingRewrites) != len(p.PendingRewrites) ||
		len(after.PendingRechecks) != len(p.PendingRechecks) {
		t.Fatalf("rejected outcome dirtied progress: before=%+v after=%+v", p, after)
	}
	if len(after.PendingRechecks) != 1 || after.PendingRechecks[0] != 1 {
		t.Fatalf("recheck queue lost: %+v", after.PendingRechecks)
	}
}
