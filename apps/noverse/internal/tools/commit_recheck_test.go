package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

func TestCommitQueuesOnlyFirstReviewDrivenRecheck(t *testing.T) {
	for _, tc := range []struct {
		name, cause, scope        string
		rechecked, required, want bool
	}{
		{"first review", store.RewriteCauseReview, "chapter", false, true, true},
		{"user rewrite", store.RewriteCauseUser, "chapter", false, true, false},
		{"second rewrite", store.RewriteCauseReview, "chapter", true, true, false},
		{"aggregate", store.RewriteCauseReview, "arc", false, true, false},
		{"no required finding", store.RewriteCauseReview, "chapter", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := recheckFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(s.Progress.DropRechecksFrom(1))
			severity := "warning"
			if tc.required {
				severity = "error"
			}
			source := domain.ReviewEntry{Chapter: 1, Scope: tc.scope, Recheck: tc.rechecked, Verdict: "rewrite", Summary: "Nguồn", Issues: []domain.ConsistencyIssue{{Severity: severity, Description: "Lỗi cửa", Quote: recheckOldQuote, RequiresChange: tc.required, Chapters: []int{1}}}}
			must(s.World.SaveReview(source))
			_, err := s.Progress.ApplyReviewOutcome(domain.FlowRewriting, []int{1}, "Cần sửa", tc.cause, 0)
			must(err)
			text, err := s.Drafts.LoadChapterText(1)
			must(err)
			saveTestChapterRecord(t, s, 1, text)
			must(s.Drafts.SaveDraft(1, recheckNewQuote+" Lan nhìn lại sân vắng."))
			args, err := json.Marshal(map[string]any{"chapter": 1, "title": "Cửa mở", "summary": "Lan ra sân", "key_events": []string{"Ra sân"}})
			must(err)
			_, err = newTestCommitChapterTool(s).Execute(context.Background(), args)
			must(err)
			p, err := s.Progress.Load()
			must(err)
			if got := len(p.PendingRechecks) > 0; got != tc.want {
				t.Fatalf("pending recheck=%v want %v: %+v", got, tc.want, p)
			}
			if tc.want && p.Flow == domain.FlowWriting {
				t.Fatal("released flow while recheck pending")
			}
		})
	}
}
