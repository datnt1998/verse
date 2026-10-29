package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func reviewDims() []domain.DimensionScore {
	return []domain.DimensionScore{{Dimension: "hook", Score: 72, Comment: "Đã đọc chương"}}
}

// TestSaveReviewSecondRoundArchivesFirst pin D4 + lưu trữ lần chấm: chấm lại cùng
// chương ghi được (trước đây là ErrToolConflict), bài cũ nằm nguyên trong
// reviews/rounds/03-r1.json để dải xanh "lần chấm 1" và ma trận chất lượng còn dữ liệu.
func TestSaveReviewSecondRoundArchivesFirst(t *testing.T) {
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

	first := domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "polish", Summary: "Còn mục bắt sửa",
		Dimensions: reviewDims(),
		Issues: []domain.ConsistencyIssue{{
			Type: "hook", Severity: "error", Description: "Kết chương an toàn",
			Evidence: "Tôi đi ngủ.", Suggestion: "Mở bằng câu thoại", Chapters: []int{3},
			RequiresChange: true,
		}},
	}
	if _, err := tool.Execute(context.Background(), reviewArgs(t, first)); err != nil {
		t.Fatalf("round 1: %v", err)
	}
	if err := s.Progress.CompleteRewrite(3, false); err != nil {
		t.Fatalf("CompleteRewrite: %v", err)
	}

	second := domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "accept", Summary: "Đạt",
		Dimensions: reviewDims(),
		Issues: []domain.ConsistencyIssue{{
			Type: "pacing", Severity: "warning", Description: "Nhịp chậm", Evidence: "đoạn 2",
			Chapters: []int{3},
		}},
	}
	if _, err := tool.Execute(context.Background(), reviewArgs(t, second)); err != nil {
		t.Fatalf("round 2 phải ghi được: %v", err)
	}

	saved, err := s.World.LoadReview(3)
	if err != nil || saved == nil || saved.Verdict != "accept" {
		t.Fatalf("review hiện tại = %+v, %v", saved, err)
	}
	rounds, err := s.World.LoadReviewRounds(3, store.ReviewScopeChapter)
	if err != nil || len(rounds) != 1 {
		t.Fatalf("rounds = %+v, %v", rounds, err)
	}
	if rounds[0].Verdict != "polish" || len(rounds[0].Issues) != 1 || rounds[0].Issues[0].Severity != "error" {
		t.Fatalf("round 1 không được giữ nguyên: %+v", rounds[0])
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), "reviews", "rounds", "03-r1.json")); err != nil {
		t.Fatalf("thiếu tệp lưu trữ: %v", err)
	}

	// Cùng nội dung ghi lại = vẫn cùng một lần chấm, không sinh thêm bản lưu trữ.
	if _, err := tool.Execute(context.Background(), reviewArgs(t, second)); err != nil {
		t.Fatalf("ghi lại round 2: %v", err)
	}
	if rounds, err := s.World.LoadReviewRounds(3, store.ReviewScopeChapter); err != nil || len(rounds) != 1 {
		t.Fatalf("ghi lặp không được tạo round mới: %+v, %v", rounds, err)
	}
}
