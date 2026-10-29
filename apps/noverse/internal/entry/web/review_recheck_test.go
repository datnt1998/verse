package web

import (
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
	"testing"
)

func TestReviewItemsMatchChapterLocalIDs(t *testing.T) {
	reader := &reviewReader{}
	entry := domain.ReviewEntry{Chapter: 3, Scope: "chapter", Issues: []domain.ConsistencyIssue{
		{RuleRef: "Master §7", Evidence: "Tôi đi ngủ."},
		{Evidence: "khá là", RuleText: "Không dùng từ cấm"},
		{RuleRef: "Xưng hô", Quote: "Em đừng đi."},
		{RuleRef: " Bộ A  § 4.3 ", Quote: "Đèn vẫn sáng."},
		{RuleRef: "not-a-pack", Evidence: "Quy tắc riêng."},
		{RuleRef: "Master §7", Quote: "Tôi đi ngủ."},
	}}
	items := reader.issueItems(&entry, map[string]int{})
	ids := domain.ReviewEntryItemIDs(entry)
	for i, item := range items {
		if item.ID != ids[i] {
			t.Fatalf("item %d=%s want %s", i, item.ID, ids[i])
		}
	}
}

func TestReviewRecheckUsesArchivedFindingIDs(t *testing.T) {
	source := domain.ReviewEntry{Chapter: 2, Scope: "chapter", Issues: []domain.ConsistencyIssue{{RuleRef: "Master §7", Quote: "Tôi đi ngủ.", Description: "Kết chương an toàn"}}}
	id := domain.ReviewEntryItemIDs(source)[0]
	reader := &reviewReader{loadRounds: func(int, store.ReviewScope) ([]domain.ReviewEntry, error) { return []domain.ReviewEntry{source}, nil }}
	current := domain.ReviewEntry{Chapter: 2, Scope: "chapter", Recheck: true, RecheckResults: []domain.RecheckResult{{FindingID: id, Status: "fixed", Note: "Đã đổi kết"}}}
	view := reader.view(&current)
	if view.Previous[0].Items[0].ID != view.RecheckResults[0].FindingID {
		t.Fatalf("lost previous round identity: %+v", view)
	}
}
