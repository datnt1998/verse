package tools

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestCompletionWaitsForRecheckInBothModes(t *testing.T) {
	for _, layered := range []bool{false, true} {
		name := "flat"
		if layered {
			name = "layered"
		}
		t.Run(name, func(t *testing.T) {
			s, r := recheckFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			// Start with a completed one-chapter book, then use the real reopen route.
			must(s.Progress.DropRechecksFrom(1))
			if layered {
				must(s.Outline.SaveLayeredOutline([]domain.VolumeOutline{{Index: 1, Title: "Tập cuối", Arcs: []domain.ArcOutline{{Index: 1, Title: "Kết", Chapters: []domain.OutlineEntry{{Chapter: 1, Title: "Cửa"}}}}}}))
				must(s.Progress.SetLayered(true))
			} else {
				must(s.Progress.SetTotalChapters(1))
			}
			must(s.Progress.MarkComplete())
			must(s.Progress.Reopen([]int{1}, "sửa review", store.RewriteCauseReview))
			must(s.Progress.CompleteRewrite(1, true))
			p, err := s.Progress.Load()
			must(err)
			complete, err := evaluateCompletionAfterDrain(s, p)
			must(err)
			if complete {
				t.Fatal("completed before recheck")
			}
			if layered {
				complete, err = ReconcileLayeredCompletion(s)
				must(err)
				if complete {
					t.Fatal("layered reconciliation bypassed queue")
				}
			}
			r.RecheckResults[0].Status = "fixed"
			r.RecheckResults[0].Quote = recheckNewQuote
			r.RecheckResults[0].Note = "Đã đổi ngữ cảnh cửa"
			_, err = NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r))
			must(err)
			p, err = s.Progress.Load()
			must(err)
			if p.Phase != domain.PhaseComplete || !p.ReviewWorkDrained() {
				t.Fatalf("fixed recheck did not re-complete: %+v", p)
			}
		})
	}
}
