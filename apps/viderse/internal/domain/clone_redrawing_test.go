package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReviewOriginRedrawKeepsReviewGroup(t *testing.T) {
	job := CloneJob{Status: CloneRunning}
	steps := []CloneStepRecord{{Step: CloneImages, Status: CloneRunning, Checkpoint: json.RawMessage(`{"reviewOrigin":true,"items":[{"id":"a","status":"running"},{"id":"b","status":"pending"},{"id":"c","status":"approved"}]}`)}}
	if got := StageGroupOf(job, steps); got != StageReview { t.Fatalf("group = %s", got) }
	if got := RedrawingCount(steps); got != 2 { t.Fatalf("redrawing = %d", got) }
	if got := Subline(job, steps, time.Now()); !strings.Contains(got, "Đang vẽ lại") { t.Fatalf("subline = %q", got) }
	steps[0].Checkpoint = json.RawMessage(`{"items":[{"id":"a","status":"running"}]}`)
	if got := StageGroupOf(job, steps); got != StageWorking { t.Fatalf("initial generation = %s", got) }
}
