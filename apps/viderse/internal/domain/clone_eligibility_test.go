package domain

import (
	"reflect"
	"testing"
)

func TestBulkEligibilityGroups(t *testing.T) {
	jobs := []CloneJobView{
		{CloneJob: CloneJob{ID: "failed", StageGroup: StageAttention}},
		{CloneJob: CloneJob{ID: "review", StageGroup: StageReview}},
		{CloneJob: CloneJob{ID: "running", StageGroup: StageWorking}},
		{CloneJob: CloneJob{ID: "ready", StageGroup: StageReady}},
		{CloneJob: CloneJob{ID: "posted", StageGroup: StagePosted}},
	}
	got := BulkEligibility(jobs)
	for name, pair := range map[string][2][]string{
		"retry": {got.Retry, []string{"failed"}}, "approve": {got.Approve, []string{"review"}},
		"markPosted": {got.MarkPosted, []string{"ready"}}, "cancel": {got.Cancel, []string{"running"}},
		"delete": {got.Delete, []string{"failed", "review", "ready", "posted"}},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s = %v, want %v", name, pair[0], pair[1])
		}
	}
}
