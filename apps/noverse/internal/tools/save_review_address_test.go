package tools

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/rules"
	"github.com/datnt1998/noverse/internal/store"
)

// addressViolationFixture: chương 3 đã xong, đã có một vi phạm xưng hô mức error
// do commit_chapter ghi (meta/rule_violations.jsonl).
func addressViolationFixture(t *testing.T, severity rules.Severity) (*store.Store, *SaveReviewTool) {
	t.Helper()
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	// quote trong rule_violations phải định vị được trong chính văn đã nộp.
	if err := s.Drafts.SaveFinalChapter(3, "Tri Hạ quay sang Nhược Vy. Em về trước đi. Rồi nàng bước đi."); err != nil {
		t.Fatalf("SaveFinalChapter: %v", err)
	}
	if err := s.World.SaveRuleViolations(3, []rules.Violation{{
		Rule: "address_terms", Target: "Tri Hạ → Nhược Vy", Actual: "em", Limit: "cô",
		Severity: severity,
		Message:  "[LỖI] chương 3, dòng 5: Tri Hạ tự xưng \"em\" thay vì \"cô\" khi nói với Nhược Vy. Trích: Em về trước đi.",
	}}); err != nil {
		t.Fatalf("SaveRuleViolations: %v", err)
	}
	return s, NewSaveReviewTool(s, nil)
}

func executeReview(t *testing.T, tool *SaveReviewTool, entry domain.ReviewEntry) error {
	t.Helper()
	_, err := tool.Execute(context.Background(), reviewArgs(t, entry))
	return err
}

// TestSaveReviewAddressLintErrorQueuesRewrite pin câu 13 / D9: vi phạm xưng hô
// mức error một mình vẫn là mục Bắt sửa — được gộp vào bài thẩm duyệt, chặn
// "Đạt" và đưa chương vào hàng đợi viết lại như mọi Bắt sửa.
func TestSaveReviewAddressLintErrorQueuesRewrite(t *testing.T) {
	s, tool := addressViolationFixture(t, rules.SeverityError)
	err := executeReview(t, tool, domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "accept", Summary: "Chương đọc ổn",
		Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 80, Comment: "Đã đọc chương"}},
		Issues:     []domain.ConsistencyIssue{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	progress, err := s.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("Progress.Load: %v", err)
	}
	if !slices.Equal(progress.PendingRewrites, []int{3}) {
		t.Fatalf("PendingRewrites = %v, muốn [3]", progress.PendingRewrites)
	}
	if progress.Flow != domain.FlowRewriting {
		t.Fatalf("Flow = %s, muốn %s", progress.Flow, domain.FlowRewriting)
	}
	if !strings.Contains(progress.RewriteReason, "Xưng hô") {
		t.Fatalf("lý do viết lại thiếu ghi chú xưng hô: %q", progress.RewriteReason)
	}

	review, err := s.World.LoadReview(3)
	if err != nil || review == nil {
		t.Fatalf("review = %+v, %v", review, err)
	}
	if review.Verdict != "rewrite" {
		t.Fatalf("verdict = %q, muốn rewrite (Đạt không duyệt được chương còn Bắt sửa)", review.Verdict)
	}
	if len(review.Issues) != 1 {
		t.Fatalf("issues = %+v, muốn đúng một mục xưng hô", review.Issues)
	}
	issue := review.Issues[0]
	if issue.RuleRef != "Xưng hô" || issue.Severity != "error" || !issue.RequiresChange {
		t.Fatalf("mục xưng hô = %+v", issue)
	}
	if !slices.Equal(issue.Chapters, []int{3}) {
		t.Fatalf("chapters = %v, muốn [3]", issue.Chapters)
	}
	if issue.Quote != "Em về trước đi." {
		t.Fatalf("quote = %q", issue.Quote)
	}
	if !strings.Contains(issue.Suggestion, "Chỉ sửa những câu dùng “em”") {
		t.Fatalf("ghi chú viết lại phải giới hạn ở câu có cách gọi đó: %q", issue.Suggestion)
	}
}

// TestSaveReviewAddressLintWarningDoesNotBlock pin câu 13: cùng vi phạm ở mức
// warning chỉ là Cảnh báo — "Đạt" giữ nguyên, hàng đợi rỗng.
func TestSaveReviewAddressLintWarningDoesNotBlock(t *testing.T) {
	s, tool := addressViolationFixture(t, rules.SeverityWarning)
	if err := executeReview(t, tool, domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "accept", Summary: "Chương đọc ổn",
		Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 80, Comment: "Đã đọc chương"}},
		Issues:     []domain.ConsistencyIssue{},
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	progress, err := s.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("Progress.Load: %v", err)
	}
	if len(progress.PendingRewrites) != 0 {
		t.Fatalf("PendingRewrites = %v, muốn rỗng", progress.PendingRewrites)
	}
	review, err := s.World.LoadReview(3)
	if err != nil || review == nil {
		t.Fatalf("review = %+v, %v", review, err)
	}
	if review.Verdict != "accept" {
		t.Fatalf("verdict = %q, muốn accept", review.Verdict)
	}
	if len(review.Issues) != 1 || review.Issues[0].Severity != "warning" || review.Issues[0].RequiresChange {
		t.Fatalf("mục xưng hô mức warning = %+v", review.Issues)
	}
}
