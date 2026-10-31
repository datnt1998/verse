package insight

import "testing"

func TestRecapSummaryUsesWaitingJobCounts(t *testing.T) {
	r := Recap{NeedsYou: []Need{{State: "review"}, {State: "review"}, {State: "attention"}}}
	r.Summarize()
	if r.Summary != "Trong lúc bạn vắng: 2 video chờ duyệt, 1 cần xử lý." {
		t.Fatal(r.Summary)
	}
}
