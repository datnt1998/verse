package store_test

import (
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/store/narrativetest"
)

// fileReviewPort adapts WorldStore (review files keyed NN.json / NN-global.json)
// to the suite's explicit-scope port.
type fileReviewPort struct{ w store.WorldStore }

func (p fileReviewPort) SaveReview(r domain.ReviewEntry) error { return p.w.SaveReview(r) }
func (p fileReviewPort) LoadReview(chapter int, scope store.ReviewScope) (*domain.ReviewEntry, error) {
	if scope == store.ReviewScopeGlobal {
		return p.w.LoadGlobalReview(chapter)
	}
	return p.w.LoadReview(chapter)
}
func (p fileReviewPort) LoadReviewRounds(chapter int, scope store.ReviewScope) ([]domain.ReviewEntry, error) {
	return p.w.LoadReviewRounds(chapter, scope)
}
func (p fileReviewPort) SaveReviewDecision(itemID string, d store.ReviewDecision) error {
	return p.w.SaveReviewDecision(itemID, d)
}
func (p fileReviewPort) LoadReviewDecisions() (map[string]store.ReviewDecision, error) {
	return p.w.LoadReviewDecisions()
}
func (p fileReviewPort) DeleteReviewDecision(itemID string) (bool, error) {
	return p.w.DeleteReviewDecision(itemID)
}

// fileHarness builds the suite harness on a fresh file-backend story.
// storyDir mirrors <root>/<project>/<story>/output/novel — the same layout
// comments.New resolves its engine dir from.
func fileHarness(t *testing.T) narrativetest.Harness {
	t.Helper()
	storyDir := filepath.Join(t.TempDir(), "project", "truyen")
	engineDir := filepath.Join(storyDir, "output", "novel")
	s := store.NewStore(engineDir)
	return narrativetest.Harness{
		Book:        s.Book,
		Progress:    s.Progress,
		Outline:     s.Outline,
		Drafts:      s.Drafts,
		Records:     s.ChapterRecords,
		Characters:  s.Characters,
		Checkpoints: s.Checkpoints,
		Decisions:   s.Decisions,
		Revisions:   s.Revisions,
		Reviews:     fileReviewPort{w: s.World},
		Comments:    comments.New(storyDir),
	}
}

// TestNarrativeContractFile chạy contract suite chia sẻ trên file backend —
// parity baseline mà backend postgres phải khớp (postgres/narrative_test.go
// chạy cùng suite).
func TestNarrativeContractFile(t *testing.T) {
	narrativetest.Run(t, fileHarness(t))
}
