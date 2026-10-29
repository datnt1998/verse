package revision

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
)

func TestUserRevisionDropsOrphanRechecks(t *testing.T) {
	s := newRevisionTestStore(t, 3)
	facts := domain.ChapterFacts{Title: "Cửa", Summary: "Lan ra sân", KeyEvents: []string{"Ra sân"}}
	acceptTestChapter(t, s, 1, "Lan đi ra sân.", facts)
	source := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Issues: []domain.ConsistencyIssue{{Severity: "error", Description: "Cửa khóa", RequiresChange: true}}}
	if err := s.World.SaveReview(source); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.CompleteRewrite(1, true); err != nil {
		t.Fatal(err)
	}
	record := testRecord(1, "Lan mở khóa rồi bước ra sân.", facts, domain.StyleDelta{}, time.Now())
	record.Revision = 2
	if err := s.Drafts.SaveFinalChapter(1, record.Content); err != nil {
		t.Fatal(err)
	}
	if err := s.ChapterRecords.Save(record); err != nil {
		t.Fatal(err)
	}
	pending := domain.PendingRevision{Stage: domain.RevisionStageRecordsApplied, Items: []domain.PendingRevisionItem{{Chapter: 1, Record: record, Analysis: domain.RevisionAnalysis{Facts: facts}}}, StartedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.Revisions.SavePending(pending); err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(s, nil, "", nil).Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, err := s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.PendingRechecks) != 0 {
		t.Fatalf("orphan queue survived edit: %+v", p)
	}
	review, err := s.World.LoadReview(1)
	if err != nil || review != nil {
		t.Fatalf("source review not invalidated: %+v %v", review, err)
	}
	inst := flow.Route(flow.State{Progress: p})
	if inst != nil && inst.Agent == "editor" && inst.Chapter == 1 {
		t.Fatalf("orphan recheck dispatched: %+v", inst)
	}
}
