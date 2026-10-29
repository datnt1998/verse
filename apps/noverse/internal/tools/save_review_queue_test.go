package tools

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// TestSaveReviewErrorIssueQueuesRewrite pin D4: mục Bắt sửa (severity error) tự
// đưa chương vào hàng chờ sửa kể cả khi Editor để requires_change=false, vì
// domain.RequiresChangeFor chuẩn hoá rồi Progress.ApplyReviewOutcome ghi hàng chờ.
func TestSaveReviewErrorIssueQueuesRewrite(t *testing.T) {
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

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "polish", Summary: "Còn mục bắt sửa",
		Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 70, Comment: "Đã đọc chương"}},
		Issues: []domain.ConsistencyIssue{{
			Type: "hook", Severity: domain.SeverityError, Description: "Kết chương an toàn",
			Evidence: "Tôi đi ngủ.", Suggestion: "Mở bằng câu thoại", Chapters: []int{3},
			RequiresChange: false,
		}},
	})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	progress, err := s.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("Progress.Load: %v", err)
	}
	if !slices.Equal(progress.PendingRewrites, []int{3}) {
		t.Fatalf("PendingRewrites = %v, muốn [3]", progress.PendingRewrites)
	}
	if progress.Flow != domain.FlowPolishing {
		t.Fatalf("Flow = %s, muốn %s", progress.Flow, domain.FlowPolishing)
	}
	review, err := s.World.LoadReview(3)
	if err != nil || review == nil {
		t.Fatalf("review = %+v, %v", review, err)
	}
	if !review.Issues[0].RequiresChange {
		t.Fatalf("requires_change phải được chuẩn hoá thành true: %+v", review.Issues[0])
	}
	// Câu 9 §H: hàng đợi do thẩm duyệt lấp đầy ⇒ đúng một dòng log cause=review.
	rewrites, err := s.Progress.Rewrites()
	if err != nil {
		t.Fatalf("Rewrites: %v", err)
	}
	if len(rewrites) != 1 || rewrites[0].Chapter != 3 || rewrites[0].Cause != store.RewriteCauseReview {
		t.Fatalf("rewrites = %+v, muốn đúng một mục review cho chương 3", rewrites)
	}
	if _, err := time.Parse(time.RFC3339, rewrites[0].At); err != nil {
		t.Fatalf("at = %q, muốn RFC3339: %v", rewrites[0].At, err)
	}
}

// Can thiệp của người dùng sửa nhiều chương được lưu bằng nhiều lần
// save_review(scope=chapter): lần sau phải cộng thêm vào hàng chờ, không được
// đẩy chương đã xếp hàng trước đó ra ngoài.
func TestSaveReviewPerChapterSavesAccumulateQueue(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	for _, ch := range []int{7, 8} {
		if err := s.Progress.MarkChapterComplete(ch, 3000, "", ""); err != nil {
			t.Fatalf("MarkChapterComplete(%d): %v", ch, err)
		}
	}

	tool := NewSaveReviewTool(s, nil)
	for _, ch := range []int{7, 8} {
		args := reviewArgs(t, domain.ReviewEntry{
			Chapter: ch, Scope: "chapter", Verdict: "rewrite", Summary: "Xóa nhãn cảnh",
			Dimensions: []domain.DimensionScore{{Dimension: "aesthetic", Score: 80, Comment: "Đã đọc nguyên văn"}},
			Issues: []domain.ConsistencyIssue{{
				Type: "format", Severity: domain.SeverityError, Description: "Nhãn phân cảnh lẫn vào chính văn",
				Evidence: "Dòng nhãn ngay sau tiêu đề", Suggestion: "Xóa riêng dòng nhãn", Chapters: []int{ch},
				RequiresChange: true,
			}},
		})
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("Execute chương %d: %v", ch, err)
		}
	}

	progress, err := s.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("Progress.Load: %v", err)
	}
	if !slices.Equal(progress.PendingRewrites, []int{7, 8}) {
		t.Fatalf("PendingRewrites = %v, muốn [7 8]", progress.PendingRewrites)
	}
	if progress.Flow != domain.FlowRewriting {
		t.Fatalf("Flow = %s, muốn %s", progress.Flow, domain.FlowRewriting)
	}
}
