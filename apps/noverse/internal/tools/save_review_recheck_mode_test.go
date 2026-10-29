package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// recheckModeFixture 搭好"打磨返工完毕、等待复查"的链路：source 裁定 polish →
// ApplyReviewOutcome(FlowPolishing) → CompleteRewrite(1, true) 入队复查并保留
// polishing。之后的 recheck 若裁定 rewrite，就命中 rewriting↔polishing 的合法换模式。
func recheckModeFixture(t *testing.T) (*store.Store, domain.ReviewEntry) {
	t.Helper()
	s := store.NewStore(t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Init())
	must(s.Progress.Init(3))
	must(s.Progress.MarkChapterComplete(1, 100, "", ""))
	must(s.Drafts.SaveFinalChapter(1, recheckOldQuote+"\n"+recheckNewQuote))
	source := domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "polish", Summary: "Câu kết gấp", RuleAware: true,
		Issues: []domain.ConsistencyIssue{{
			Type: "pacing", Severity: "warning", Description: "Nhịp cuối gấp",
			Quote: recheckOldQuote, Evidence: recheckOldQuote, Chapters: []int{1}, RequiresChange: true,
		}},
	}
	must(s.World.SaveReview(source))
	_, err := s.Progress.ApplyReviewOutcome(domain.FlowPolishing, []int{1}, source.Summary, store.RewriteCauseReview, 0)
	must(err)
	must(s.Progress.CompleteRewrite(1, true))
	entry := domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Recheck: true, Verdict: "rewrite",
		Summary: "Đoạn lỗi vẫn còn nguyên", Dimensions: reviewDims(),
		Issues: []domain.ConsistencyIssue{{
			Type: "pacing", Severity: "error", Description: "Nhịp cuối vẫn gấp",
			Quote: recheckOldQuote, Evidence: recheckOldQuote, Chapters: []int{1}, RequiresChange: true,
		}},
		RecheckResults: []domain.RecheckResult{{
			FindingID: domain.ReviewEntryItemIDs(source)[0], Status: "unfixed", Quote: recheckOldQuote,
		}},
	}
	return s, entry
}

// TestSaveReviewRecheckSwitchesPolishToRewrite 防回归：save_review 全链路。
// CompleteRewrite 保留 polishing，复查裁定 rewrite 必须一次写成功——否则 review
// 落盘后进度被拒，同样的 review 重试永远失败，改成别的裁定又撞上 sameStoredReview
// 冲突，复查队列永远排不空。
func TestSaveReviewRecheckSwitchesPolishToRewrite(t *testing.T) {
	s, r := recheckModeFixture(t)
	p, err := s.Progress.Load()
	if err != nil || p.Flow != domain.FlowPolishing || len(p.PendingRechecks) != 1 {
		t.Fatalf("fixture broken: %+v %v", p, err)
	}
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r)); err != nil {
		t.Fatalf("recheck polish→rewrite rejected: %v", err)
	}
	p, err = s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Flow != domain.FlowRewriting || len(p.PendingRewrites) != 1 || p.PendingRewrites[0] != 1 || len(p.PendingRechecks) != 0 {
		t.Fatalf("mode switch not applied atomically: %+v", p)
	}
	// 幂等重试：复查已排空但 artifact 一致，不得复活工作也不得冲突。
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r)); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
}

// TestSaveReviewRecheckModeSwitchRetryAfterProgressFailure 防回归：换模式的复查在
// artifact 落盘、progress 写失败之后，同一 review 重试必须收敛（队列排空 + flow
// 已切换），而不是卡在非法迁移上。
func TestSaveReviewRecheckModeSwitchRetryAfterProgressFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directory requires unprivileged process")
	}
	s, r := recheckModeFixture(t)
	meta := filepath.Join(s.Dir(), "meta")
	if err := os.Chmod(meta, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(meta, 0o755); err != nil {
			t.Error(err)
		}
	})
	_, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r))
	if err == nil || !strings.Contains(err.Error(), "apply review outcome") {
		t.Fatalf("expected progress failure after review save: %v", err)
	}
	current, err := s.World.LoadReview(1)
	if err != nil || current == nil || !current.Recheck {
		t.Fatalf("artifact not persisted: %v %v", current, err)
	}
	p, err := s.Progress.Load()
	if err != nil || len(p.PendingRechecks) != 1 || p.Flow != domain.FlowPolishing {
		t.Fatalf("queue lost: %+v %v", p, err)
	}
	if err := os.Chmod(meta, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r)); err != nil {
		t.Fatalf("retry did not converge: %v", err)
	}
	p, err = s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.PendingRechecks) != 0 || len(p.PendingRewrites) != 1 || p.Flow != domain.FlowRewriting {
		t.Fatalf("retry did not apply mode switch: %+v", p)
	}
}
