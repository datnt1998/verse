package eval

import (
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestRecheckFixtureSeedsSourceAndPendingQueue(t *testing.T) {
	out := filepath.Join(t.TempDir(), "novel")
	if err := SeedFixture(filepath.Join("..", "..", "evals", "fixtures"), "ed-recheck-unfixed", out, "warning"); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(out)
	p, err := s.Progress.Load()
	if err != nil || len(p.PendingRechecks) != 1 || p.PendingRechecks[0] != 2 {
		t.Fatalf("queue=%+v err=%v", p, err)
	}
	source, err := s.World.LoadReview(2)
	if err != nil || source == nil {
		t.Fatalf("source=%+v err=%v", source, err)
	}
	id := domain.ReviewEntryItemIDs(*source)[0]
	next := domain.ReviewEntry{Chapter: 2, Scope: "chapter", Recheck: true, Verdict: "rewrite", Summary: "Vẫn lỗi", RecheckResults: []domain.RecheckResult{{FindingID: id, Status: "unfixed", Quote: source.Issues[0].Quote}}}
	if err := s.World.SaveReview(next); err != nil {
		t.Fatal(err)
	}
	col := Collect(out, nil)
	if len(col.Reviews) != 1 || len(col.ReviewRounds) != 1 || len(col.Reviews[0].RecheckResults) != 1 {
		t.Fatalf("collector dropped recheck results: %+v", col)
	}
	c := reviewCase()
	c.Control = false
	c.Expect.Review.MustFind = nil
	c.Expect.Review.MustNotFind = nil
	c.Expect.Review.VerdictIn = []string{"rewrite"}
	c.Expect.Review.RecheckStatus = map[string]string{source.Issues[0].Quote: "unfixed"}
	res := Result{}
	gradeReviewExpect(c, col, &res)
	if len(res.HardFails) != 0 || len(res.Skips) != 0 {
		t.Fatalf("source quote grading failed: %+v", res)
	}
}

func TestRecheckStatusMissingAndWrongFail(t *testing.T) {
	for _, status := range []string{"", "fixed", "unknown"} {
		t.Run(status, func(t *testing.T) {
			c := reviewCase()
			c.Control = false
			c.Expect.Review.MustFind = nil
			c.Expect.Review.MustNotFind = nil
			c.Expect.Review.RecheckStatus = map[string]string{"finding": "unfixed"}
			col := Collected{Reviews: []domain.ReviewEntry{{Chapter: 2, Scope: "chapter", Verdict: "accept", Recheck: true}}}
			if status != "" {
				col.Reviews[0].RecheckResults = []domain.RecheckResult{{FindingID: "finding", Status: status}}
			}
			res := Result{}
			gradeReviewExpect(c, col, &res)
			if !hasIssue(res.HardFails, "contract:review:recheck_status", "") {
				t.Fatalf("wrong status passed: %+v", res)
			}
			if len(res.Skips) != 0 {
				t.Fatalf("recheck still skipped: %+v", res.Skips)
			}
		})
	}
}
