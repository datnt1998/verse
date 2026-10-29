package domain

import (
	"reflect"
	"testing"
)

func TestReviewEntryItemIDs(t *testing.T) {
	entry := ReviewEntry{Chapter: 3, Issues: []ConsistencyIssue{
		{RuleRef: "Master §7", Evidence: "Tôi đi ngủ."},
		{RuleRef: "", Evidence: "khá là"},
		{RuleRef: "Xưng hô", Quote: "Em đừng đi."},
		{RuleRef: " Bộ A  § 4.3 ", Quote: "Đèn vẫn sáng."},
		{RuleRef: "not-a-pack", Evidence: "Quy tắc riêng."},
		{RuleRef: "Master §7", Quote: "Tôi đi ngủ."},
	}}
	ids := ReviewEntryItemIDs(entry)
	used := map[string]int{}
	refs := []string{"Master §7", "Quy tắc của bạn", "Xưng hô", "Bộ A §4.3", "Quy tắc của bạn", "Master §7"}
	quotes := []string{"Tôi đi ngủ.", "khá là", "Em đừng đi.", "Đèn vẫn sáng.", "Quy tắc riêng.", "Tôi đi ngủ."}
	for i := range ids {
		if ref := ReviewIssueRef(entry.Issues[i]); ref != refs[i] {
			t.Fatalf("reference %d=%s", i, ref)
		}
		if want := ReviewItemID(3, used, refs[i], quotes[i]); ids[i] != want {
			t.Fatalf("ID %d=%s want %s", i, ids[i], want)
		}
	}
	if ids[5] != ids[0]+"-2" {
		t.Fatalf("duplicate suffix lost: %v", ids)
	}
	if !reflect.DeepEqual(ids, ReviewEntryItemIDs(entry)) {
		t.Fatal("counter escaped entry")
	}
}
