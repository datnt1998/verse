package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReviewSublineSeparatesBuildDraftFromImageApproval(t *testing.T) {
	job := CloneJob{Status: CloneWaiting}
	build := []CloneStepRecord{
		{Step: CloneImages, Status: CloneSucceeded},
		{Step: CloneBuild, Status: CloneWaiting, Error: &CloneError{Code: CloneErrAwaitingReview, Message: "Bản dựng đang chờ duyệt"}},
	}
	if got := StageGroupOf(job, build); got != StageReview {
		t.Fatalf("build gate group = %s", got)
	}
	if got := Subline(job, build, time.Now()); got != "Bản dựng đang chờ duyệt" {
		t.Fatalf("build gate subline = %q", got)
	}
	images := []CloneStepRecord{
		{Step: CloneImages, Status: CloneWaiting, Error: &CloneError{Code: CloneErrAwaitingReview, Message: "Chờ duyệt ảnh"}},
		{Step: CloneBuild, Status: ClonePending},
	}
	if got := Subline(job, images, time.Now()); got != "Chờ bạn duyệt ảnh" {
		t.Fatalf("image gate subline = %q", got)
	}
	both := append(append([]CloneStepRecord{}, images...), build[1])
	if got := Subline(job, both, time.Now()); got != "Chờ bạn duyệt ảnh" {
		t.Fatalf("image gate must win over a stale build gate: %q", got)
	}
}

func TestReviewOriginRedrawKeepsReviewGroup(t *testing.T) {
	job := CloneJob{Status: CloneRunning}
	steps := []CloneStepRecord{{Step: CloneImages, Status: CloneRunning, Checkpoint: json.RawMessage(`{"reviewOrigin":true,"items":[{"id":"a","status":"running"},{"id":"b","status":"pending"},{"id":"c","status":"approved"}]}`)}}
	if got := StageGroupOf(job, steps); got != StageReview {
		t.Fatalf("group = %s", got)
	}
	if got := RedrawingCount(steps); got != 2 {
		t.Fatalf("redrawing = %d", got)
	}
	if got := Subline(job, steps, time.Now()); !strings.Contains(got, "Đang vẽ lại") {
		t.Fatalf("subline = %q", got)
	}
	steps[0].Checkpoint = json.RawMessage(`{"items":[{"id":"a","status":"running"}]}`)
	if got := StageGroupOf(job, steps); got != StageWorking {
		t.Fatalf("initial generation = %s", got)
	}
}
