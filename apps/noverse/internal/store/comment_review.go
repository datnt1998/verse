package store

import (
	"fmt"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// GrantCommentReview atomically persists the Writer request and next permit.
func (s *fileRunMetaStore) GrantCommentReview(next int, review domain.CommentReview) error {
	if next < 1 || review.Chapter < 1 || review.Chapter >= next || review.Instruction == "" {
		return fmt.Errorf("Chương góp ý hoặc giấy phép không hợp lệ")
	}
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil || meta.AdvanceMode != domain.ChapterAdvanceReview {
			return fmt.Errorf("Góp ý chỉ được duyệt trong chế độ nghiệm thu từng chương")
		}
		if meta.AdvanceHold != nil {
			return fmt.Errorf("Vẫn còn ý định tạm dừng đang chờ xử lý")
		}
		if meta.AdvancePermitChapter != 0 && meta.AdvancePermitChapter != next {
			return fmt.Errorf("Đã có giấy phép chương %d", meta.AdvancePermitChapter)
		}
		if old := meta.PendingCommentReview; old != nil && *old != review {
			return fmt.Errorf("Góp ý đã duyệt trước đó vẫn đang chờ Người viết")
		}
		meta.AdvancePermitChapter = next
		meta.PendingCommentReview = &review
		return s.saveUnlocked(*meta)
	})
}

func (s *fileRunMetaStore) ClearCommentReview(review domain.CommentReview) error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil || meta.PendingCommentReview == nil {
			return nil
		}
		if *meta.PendingCommentReview != review {
			return fmt.Errorf("Góp ý đang chờ đã thay đổi")
		}
		meta.PendingCommentReview = nil
		return s.saveUnlocked(*meta)
	})
}
