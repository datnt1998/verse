package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

const recheckOldQuote = "Cánh cửa đã bị khóa từ phía ngoài."
const recheckNewQuote = "Lan mở chốt bên trong rồi đẩy cửa bước ra."

func recheckFixture(t *testing.T) (*store.Store, domain.ReviewEntry) {
	t.Helper()
	s := store.NewStore(t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Init())
	must(s.Progress.Init(3))
	must(s.Progress.MarkChapterComplete(1, 100, "", ""))
	must(s.Drafts.SaveFinalChapter(1, recheckOldQuote+"\n"+recheckNewQuote))
	source := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "rewrite", Summary: "Mâu thuẫn cửa", RuleAware: true, Issues: []domain.ConsistencyIssue{{Type: "continuity", Severity: "error", Description: "Cửa khóa ngoài nhưng Lan đi ra", Quote: recheckOldQuote, Evidence: recheckOldQuote, Chapters: []int{1}, RequiresChange: true}}}
	must(s.World.SaveReview(source))
	_, err := s.Progress.ApplyReviewOutcome(domain.FlowRewriting, []int{1}, source.Summary, store.RewriteCauseReview, 0)
	must(err)
	must(s.Progress.CompleteRewrite(1, true))
	entry := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Recheck: true, Verdict: "accept", Summary: "Đã kiểm lại", Issues: []domain.ConsistencyIssue{}, Dimensions: reviewDims(), RecheckResults: []domain.RecheckResult{{FindingID: domain.ReviewEntryItemIDs(source)[0], Status: "unknown", Note: "Chưa xác định được cách mở khóa"}}}
	return s, entry
}

func TestSaveReviewRecheckValidation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*domain.ReviewEntry)
		want   string
	}{
		{"unknown", func(*domain.ReviewEntry) {}, ""},
		{"unknown needs note", func(r *domain.ReviewEntry) { r.RecheckResults[0].Note = "" }, "unknown cần note"},
		{"fixed needs note", func(r *domain.ReviewEntry) { r.RecheckResults[0].Status = "fixed"; r.RecheckResults[0].Note = "" }, "fixed cần note"},
		{"fixed unchanged", func(r *domain.ReviewEntry) { r.RecheckResults[0].Status = "fixed" }, "đoạn lỗi còn nguyên"},
		{"fixed same quote", func(r *domain.ReviewEntry) {
			r.RecheckResults[0].Status = "fixed"
			r.RecheckResults[0].Quote = recheckOldQuote
		}, "đoạn lỗi còn nguyên"},
		{"fixed new context", func(r *domain.ReviewEntry) {
			r.RecheckResults[0].Status = "fixed"
			r.RecheckResults[0].Quote = recheckNewQuote
		}, ""},
		{"missing", func(r *domain.ReviewEntry) { r.RecheckResults = nil }, "thiếu finding_id"},
		{"duplicate", func(r *domain.ReviewEntry) { r.RecheckResults = append(r.RecheckResults, r.RecheckResults[0]) }, "lạ/trùng"},
		{"foreign", func(r *domain.ReviewEntry) { r.RecheckResults[0].FindingID = "foreign" }, "lạ/trùng"},
		{"bad status", func(r *domain.ReviewEntry) { r.RecheckResults[0].Status = "pass" }, "status không hợp lệ"},
		{"ungrounded", func(r *domain.ReviewEntry) {
			r.RecheckResults[0].Quote = "Một câu chưa từng có trong chính văn."
		}, "quote không tìm thấy"},
		{"unfixed missing issue", func(r *domain.ReviewEntry) {
			r.RecheckResults[0].Status = "unfixed"
			r.RecheckResults[0].Quote = recheckOldQuote
		}, "issue requires_change"},
		{"unfixed without quote", func(r *domain.ReviewEntry) { r.RecheckResults[0].Status = "unfixed" }, "issue requires_change"},
		{"unfixed mismatched quote", func(r *domain.ReviewEntry) {
			r.Verdict = "rewrite"
			r.RecheckResults[0].Status = "unfixed"
			r.RecheckResults[0].Quote = recheckOldQuote
			r.Issues = []domain.ConsistencyIssue{{Type: "continuity", Severity: "error", Description: "Mục khác", Evidence: recheckNewQuote, Quote: recheckNewQuote, RequiresChange: true}}
		}, "issue requires_change"},
		{"unfixed", func(r *domain.ReviewEntry) {
			r.Verdict = "rewrite"
			r.RecheckResults[0].Status = "unfixed"
			r.RecheckResults[0].Quote = recheckOldQuote
			r.Issues = []domain.ConsistencyIssue{{Type: "continuity", Severity: "error", Description: "Cửa vẫn khóa", Evidence: recheckOldQuote, Quote: recheckOldQuote, RequiresChange: true}}
		}, ""},
		{"normal with results", func(r *domain.ReviewEntry) { r.Recheck = false }, "review thường"},
		{"wrong scope", func(r *domain.ReviewEntry) { r.Scope = "global" }, "scope=chapter"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, r := recheckFixture(t)
			tc.change(&r)
			_, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r))
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("want %q got %v", tc.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			p, err := s.Progress.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(p.PendingRechecks) != 0 {
				t.Fatalf("recheck not drained: %+v", p)
			}
			if r.RecheckResults[0].Status == "unfixed" && len(p.PendingRewrites) != 1 {
				t.Fatalf("unfixed not queued: %+v", p)
			}
			if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r)); err != nil {
				t.Fatalf("retry: %v", err)
			}
			rounds, err := s.World.LoadReviewRounds(1, store.ReviewScopeChapter)
			if err != nil || len(rounds) != 1 {
				t.Fatalf("retry archived twice: %v %v", rounds, err)
			}
		})
	}
}

func TestSaveReviewRecheckCrashBeforeProgress(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directory requires unprivileged process")
	}
	s, r := recheckFixture(t)
	meta := filepath.Join(s.Dir(), "meta")
	if err := os.Chmod(meta, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(meta, 0o755); err != nil {
			t.Error(err)
		}
	})
	_, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r))
	if err == nil || !strings.Contains(err.Error(), "apply review outcome") {
		t.Fatalf("expected progress failure after review save: %v", err)
	}
	current, err := s.World.LoadReview(1)
	if err != nil || current == nil || !current.Recheck {
		t.Fatalf("artifact not persisted: %v %v", current, err)
	}
	p, err := s.Progress.Load()
	if err != nil || len(p.PendingRechecks) != 1 {
		t.Fatalf("queue lost: %+v %v", p, err)
	}
	if err := os.Chmod(meta, 0o755); err != nil {
		t.Fatal(err)
	}
	tool := NewContextTool(s, References{}, "", NewStyleStatsIndex(s))
	envelope := newChapterContextEnvelope()
	reads := &contextReads{}
	tool.prepareChapterContext(1, &envelope, reads)
	findings := envelope.Working["recheck_findings"].([]map[string]any)
	if len(findings) != 1 || findings[0]["finding_id"] != r.RecheckResults[0].FindingID || findings[0]["quote_still_present"] != true {
		t.Fatalf("retry context lost source: %+v", findings)
	}
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r)); err != nil {
		t.Fatal(err)
	}
	p, err = s.Progress.Load()
	if err != nil || len(p.PendingRechecks) != 0 {
		t.Fatalf("retry did not drain: %+v %v", p, err)
	}
}

func TestSaveReviewRecheckRetryDoesNotResurrectRewrite(t *testing.T) {
	s, r := recheckFixture(t)
	r.Verdict = "rewrite"
	r.RecheckResults[0].Status = "unfixed"
	r.RecheckResults[0].Quote = recheckOldQuote
	r.Issues = []domain.ConsistencyIssue{{Type: "continuity", Severity: "error", Description: "Vẫn khóa", Evidence: recheckOldQuote, Quote: recheckOldQuote, RequiresChange: true}}
	tool := NewSaveReviewTool(s, nil)
	if _, err := tool.Execute(context.Background(), reviewArgs(t, r)); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.CompleteRewrite(1, false); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Execute(context.Background(), reviewArgs(t, r)); err != nil {
		t.Fatal(err)
	}
	p, err := s.Progress.Load()
	if err != nil || !p.ReviewWorkDrained() {
		t.Fatalf("retry resurrected work: %+v %v", p, err)
	}
}

func TestSaveReviewRecheckRequiresQueuedChapter(t *testing.T) {
	s, r := recheckFixture(t)
	if err := s.Progress.DropRechecksFrom(1); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r)); err == nil || !strings.Contains(err.Error(), "PendingRechecks") {
		t.Fatalf("unqueued recheck accepted: %v", err)
	}
}

func TestSaveReviewFixedRemovedQuoteNeedsNoReplacement(t *testing.T) {
	s, r := recheckFixture(t)
	if err := s.Drafts.SaveFinalChapter(1, recheckNewQuote); err != nil {
		t.Fatal(err)
	}
	r.RecheckResults[0].Status = "fixed"
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, r)); err != nil {
		t.Fatal(err)
	}
}
