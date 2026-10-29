package web

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

// The "Sáng tác" job row in the jobs dock shows Step directly: it must be Vietnamese, never an engine code like RUNNING.
func TestWritingJobStepIsVietnamese(t *testing.T) {
	cases := []struct {
		name string
		snap host.UISnapshot
		want string
	}{
		{"running", host.UISnapshot{IsRunning: true, StatusLabel: "RUNNING"}, "Đang chạy"},
		{"reviewing", host.UISnapshot{IsRunning: true, StatusLabel: "REVIEW"}, "Thẩm duyệt"},
		{"rewriting", host.UISnapshot{IsRunning: true, StatusLabel: "REWRITE"}, "Viết lại"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &workerServer{jobs: newJobTracker()}
			s.updateWritingJob(tc.snap)
			jobs := s.jobs.listJobs()
			if len(jobs) != 1 {
				t.Fatalf("jobs = %d, want 1", len(jobs))
			}
			if jobs[0].Step != tc.want {
				t.Fatalf("Step = %q, want %q", jobs[0].Step, tc.want)
			}
		})
	}
}

func TestStatusLabelTextUnknownCodeNotUppercase(t *testing.T) {
	if got := host.StatusLabelText(""); got != "Sẵn sàng" {
		t.Fatalf("empty = %q", got)
	}
	if got := host.StatusLabelText("WEIRD"); got != "weird" {
		t.Fatalf("unknown = %q", got)
	}
}
