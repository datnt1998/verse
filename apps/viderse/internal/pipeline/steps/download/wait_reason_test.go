package download

import (
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func TestMappedSourceWaitKeepsWorkerContractAndSafeCause(t *testing.T) {
	now := time.Date(2026, 11, 8, 17, 16, 36, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		code    tools.YtDlpErrorCode
		message string
		reason  string
		backoff time.Duration
	}{
		{"bot challenge", tools.YtDlpLoginRequired, "Sign in to confirm you're not a bot. private-video-id", domain.CloneWaitSourceAuth, 30 * time.Minute},
		{"transient source failure", tools.YtDlpFailed, "HTTP Error 429 private-video-id", domain.CloneWaitSource, 10 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := MapDownloadError(&tools.YtDlpError{Code: tc.code, Message: tc.message, StderrTail: "private-stderr"}, now.UnixMilli())
			if err.Code != domain.CloneErrQuota || !domain.CloneErrRetryable(err.Code) || err.WaitReason != tc.reason {
				t.Fatalf("wait contract %+v", err)
			}
			if err.RetryAt == nil || !err.RetryAt.Equal(now.Add(tc.backoff)) || err.RetryAtSource != domain.RetryAtWorker {
				t.Fatalf("backoff contract %+v", err)
			}
			for _, raw := range []string{tc.message, "private-video-id", "private-stderr", "Sign in", "HTTP Error"} {
				if strings.Contains(err.Message, raw) || (err.Recovery != nil && strings.Contains(*err.Recovery, raw)) {
					t.Fatalf("unsafe human error: %+v", err)
				}
			}
			step := domain.CloneStepRecord{Step: domain.CloneDownload, Status: domain.CloneWaiting, Attempt: 1, Error: err}
			if domain.AutoRetryAt(step, now) == nil {
				t.Fatal("mapped source wait lost worker retry")
			}
			line := domain.Subline(domain.CloneJob{Status: domain.CloneRunning}, []domain.CloneStepRecord{step}, now)
			if strings.Contains(line, "hạn mức") || strings.Contains(line, "private") {
				t.Fatalf("misleading visible wait %q", line)
			}
			step.Attempt = domain.AutoRetryAttempts
			if domain.AutoRetryAt(step, now) != nil {
				t.Fatal("mapped wait bypassed attempt budget")
			}
		})
	}
}

func TestRetryErrorPreservesStorageAndTransientCauses(t *testing.T) {
	now := time.Unix(1700000000, 0)
	for _, reason := range []string{domain.CloneWaitStorage, domain.CloneWaitTransient} {
		err := retryError(reason, "Thông báo an toàn", "Kiểm tra rồi thử lại", now.UnixMilli(), 5*time.Minute)
		step := domain.CloneStepRecord{Step: domain.CloneDownload, Status: domain.CloneWaiting, Attempt: 1, Error: err}
		if domain.CloneWaitReason(step.Step, err) != reason || domain.AutoRetryAt(step, now) == nil {
			t.Fatalf("cause/backoff lost: %+v", err)
		}
	}
}
