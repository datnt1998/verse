package host

import (
	"fmt"
	"slices"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/flow"
	"github.com/datnt1998/noverse/internal/store"
)

// AdvanceOneChapterWithComments does not call Continue/Steer: those start the
// engine before a separate /next can grant its permit. The explicit Writer task
// and permit are committed together, then the existing launch path runs once.
func (h *Host) AdvanceOneChapterWithComments(chapter int, instruction string) error {
	if strings.TrimSpace(instruction) == "" {
		return h.AdvanceOneChapter()
	}
	return h.advanceOneChapter(&domain.CommentReview{Chapter: chapter, Instruction: instruction})
}

// commentReviewInstruction replays a durable approval before normal routing,
// including after a process restart between approval and Writer dispatch.
func (e *engine) commentReviewInstruction() (*flow.Instruction, error) {
	meta, err := e.store.RunMeta.Load()
	if err != nil {
		return nil, err
	}
	if meta == nil || meta.PendingCommentReview == nil {
		return nil, nil
	}
	review := meta.PendingCommentReview
	progress, err := e.store.Progress.Load()
	if err != nil {
		return nil, err
	}
	if progress == nil || !slices.Contains(progress.CompletedChapters, review.Chapter) {
		return nil, fmt.Errorf("Chương góp ý %d chưa hoàn thành", review.Chapter)
	}
	if !slices.Contains(progress.PendingRewrites, review.Chapter) {
		queue := append([]int{review.Chapter}, progress.PendingRewrites...)
		if err := e.store.Progress.SetPendingRewrites(queue, review.Instruction, store.RewriteCauseUser); err != nil {
			return nil, err
		}
	}
	if progress.Flow != domain.FlowRewriting {
		if err := e.store.Progress.SetFlow(domain.FlowRewriting); err != nil {
			return nil, err
		}
	}
	return &flow.Instruction{Agent: "writer", Chapter: review.Chapter, Task: review.Instruction, Reason: "Áp dụng góp ý đã duyệt theo đoạn"}, nil
}

func (e *engine) finishCommentReview(inst *flow.Instruction) error {
	meta, err := e.store.RunMeta.Load()
	if err != nil {
		return err
	}
	if meta == nil || meta.PendingCommentReview == nil {
		return nil
	}
	review := *meta.PendingCommentReview
	if inst.Agent != "writer" || inst.Chapter != review.Chapter || inst.Task != review.Instruction {
		return nil
	}
	return e.store.RunMeta.ClearCommentReview(review)
}
