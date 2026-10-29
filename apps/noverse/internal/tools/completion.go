package tools

import (
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// evaluateCompletionAfterDrain never completes a book while review work remains.
func evaluateCompletionAfterDrain(s *store.Store, progress *domain.Progress) (bool, error) {
	if progress == nil || !progress.ReviewWorkDrained() {
		return false, nil
	}
	complete := false
	var err error
	switch {
	case progress.Layered && progress.ReopenedFromComplete:
		complete, err = layeredStructurallyComplete(s, progress)
	case progress.Layered:
		complete, err = layeredComplete(s, progress)
	default:
		complete = progress.TotalChapters > 0 && len(progress.CompletedChapters) >= progress.TotalChapters
	}
	if err != nil {
		return false, fmt.Errorf("evaluate completion: %w", err)
	}
	if complete {
		if err := s.Progress.MarkComplete(); err != nil {
			return false, fmt.Errorf("mark complete: %w", err)
		}
	}
	return complete, nil
}
