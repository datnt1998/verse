package domain

import (
 "testing"
 "time"
)

func TestPickDefaultJobPriorityAndAge(t *testing.T) {
 now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
 jobs := []CloneJobView{
  {CloneJob: CloneJob{ID:"review", StageGroup:StageReview, CreatedAt:now.Add(-4*time.Hour)}},
  {CloneJob: CloneJob{ID:"new-attention", StageGroup:StageAttention, CreatedAt:now.Add(-time.Hour)}},
  {CloneJob: CloneJob{ID:"old-attention", StageGroup:StageAttention, CreatedAt:now.Add(-2*time.Hour)}},
  {CloneJob: CloneJob{ID:"working", StageGroup:StageWorking, CreatedAt:now}},
 }
 if got := PickDefaultJob(jobs); got != "old-attention" { t.Fatalf("got %q", got) }
 if got := PickDefaultJob(jobs[:1]); got != "review" { t.Fatalf("got %q", got) }
 if got := PickDefaultJob(nil); got != "" { t.Fatalf("got %q", got) }
}
