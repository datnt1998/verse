package domain

import (
	"strings"
	"testing"
	"time"
)

func TestWaitPresentationPreservesRetryBoundaries(t *testing.T) {
	now := time.Date(2026, 11, 8, 17, 16, 36, 0, time.UTC)
	at := now.Add(30 * time.Minute)
	for _, tc := range []struct {
		name, message, reason, want, visible string
		step                                 CloneStep
	}{
		{"stored bot challenge", "yt-dlp: Sign in to confirm you’re not a bot job-private-id", "", CloneWaitSourceAuth, "YouTube", CloneDownload},
		{"stored disk wait", "Không đủ chỗ: cần 4 GB", "", CloneWaitStorage, "dung lượng", CloneDownload},
		{"stored timeout", "Transcode nguồn bị timeout", "", CloneWaitTransient, "thử lại", CloneDownload},
		{"stored source wait", "yt-dlp: HTTP Error 429 private-id", "", CloneWaitSource, "nguồn", CloneDownload},
		{"explicit auth", "safe message", CloneWaitSourceAuth, CloneWaitSourceAuth, "YouTube", CloneDownload},
		{"explicit storage", "safe message", CloneWaitStorage, CloneWaitStorage, "dung lượng", CloneDownload},
		{"explicit transient", "safe message", CloneWaitTransient, CloneWaitTransient, "thử lại", CloneDownload},
		{"background source", "unavailable", "", CloneWaitSource, "nguồn", CloneBackdrop},
		{"real image quota", "image limit", "", CloneWaitImageQuota, "hạn mức ảnh", CloneImages},
		{"unrelated English is not auth", "Sign in to confirm you’re not a bot", "", CloneWaitTransient, "thử lại", CloneBuild},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := &CloneError{Code: CloneErrQuota, Message: tc.message, WaitReason: tc.reason, RetryAt: &at, RetryAtSource: RetryAtWorker}
			step := CloneStepRecord{Step: tc.step, Status: CloneWaiting, Attempt: 1, Error: err}
			if got := CloneWaitReason(step.Step, err); got != tc.want {
				t.Fatalf("reason %q want %q", got, tc.want)
			}
			if !CloneErrRetryable(err.Code) {
				t.Fatal("wait would become a terminal failure")
			}
			for _, status := range []CloneStatus{CloneRunning, CloneWaiting} {
				job := CloneJob{Status: status}
				line := Subline(job, []CloneStepRecord{step}, now)
				if !strings.Contains(line, tc.visible) || strings.Contains(line, tc.message) || strings.Contains(line, "private-id") {
					t.Fatalf("wrong visible reason: %q", line)
				}
				if tc.step != CloneImages && strings.Contains(line, "hạn mức") {
					t.Fatalf("non-image wait claims quota: %q", line)
				}
				if StageGroupOf(job, []CloneStepRecord{step}) != StageWorking {
					t.Fatal("presentation changed the parked stage")
				}
			}
			if got := AutoRetryAt(step, now); (got != nil) != (tc.step != CloneImages) {
				t.Fatalf("reason changed auto retry eligibility: %v", got)
			}
			step.Attempt = AutoRetryAttempts
			if AutoRetryAt(step, now) != nil {
				t.Fatal("retry attempt budget bypassed")
			}
			step.Attempt = 1
			err.RetryAtSource = RetryAtProvider
			if AutoRetryAt(step, now) != nil {
				t.Fatal("provider reset became worker backoff")
			}
			err.RetryAtSource = RetryAtWorker
			later := at.Add(time.Millisecond)
			err.RetryAt = &later
			if AutoRetryAt(step, now) != nil {
				t.Fatal("long backoff budget bypassed")
			}
		})
	}
}

func TestWaitReasonDoesNotReclassifyTerminalSourceErrors(t *testing.T) {
	for _, code := range []string{CloneErrInvalidRequest, CloneErrSourceUnavailable} {
		err := &CloneError{Code: code, Message: "Sign in to confirm you're not a bot", WaitReason: CloneWaitSourceAuth}
		if CloneWaitReason(CloneDownload, err) != "" || CloneErrRetryable(code) {
			t.Fatalf("terminal error became a retryable wait: %+v", err)
		}
	}
}
