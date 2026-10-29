package host

import (
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
)

func TestMobileReopenPreconditionsAndCompletionTimestamp(t *testing.T) {
	st := commentReviewStore(t)
	h := &Host{store: st, events: make(chan Event, 8)}
	if err := st.Progress.MarkComplete(); err != nil {
		t.Fatal(err)
	}
	p, err := st.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.LockedAt == nil || p.CompletionVersion != 1 {
		t.Fatalf("completion timestamp/version missing: %+v", p)
	}
	original := *p.LockedAt
	if err := st.Progress.MarkComplete(); err != nil {
		t.Fatal(err)
	}
	checks, err := h.ReopenPreconditions()
	if err != nil {
		t.Fatal(err)
	}
	if !checks.Allowed || checks.SchemaVersion != 1 || checks.LockedAt == nil || !checks.LockedAt.Equal(original) || checks.Checks[2].Label != "1/1 chương đã duyệt sạch" {
		t.Fatalf("clean completion: %+v", checks)
	}
	if err := st.Drafts.SaveFinalChapter(1, "Nội dung đã sửa tay"); err != nil {
		t.Fatal(err)
	}
	checks, err = h.ReopenPreconditions()
	if err != nil {
		t.Fatal(err)
	}
	if checks.Allowed || checks.Checks[2].OK || checks.Checks[2].Label != "0/1 chương đã duyệt sạch" || len(checks.Checks[2].Chapters) != 1 || checks.Checks[2].Chapters[0] != 1 {
		t.Fatalf("unapproved chapter links/count: %+v", checks)
	}
	if err := h.Reopen(""); err == nil {
		t.Fatal("unapproved revisions must block actual reopen")
	}
	if err := st.Drafts.SaveFinalChapter(1, "Nội dung chương một"); err != nil {
		t.Fatal(err)
	}
	p, _ = st.Progress.Load()
	p.PendingRewrites = []int{1}
	if err := st.Progress.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := h.Reopen(""); err == nil {
		t.Fatal("pending rewrite must block actual reopen")
	}
	p.PendingRewrites = nil
	if err := st.Progress.Save(p); err != nil {
		t.Fatal(err)
	}
	h.lifecycle = lifecycleRunning
	checks, err = h.ReopenPreconditions()
	if err != nil {
		t.Fatal(err)
	}
	if checks.Allowed || checks.Checks[1].OK {
		t.Fatal("running engine must block reopen")
	}
	h.lifecycle = lifecycleIdle
	if err := h.Reopen(""); err != nil {
		t.Fatal(err)
	}
	p, _ = st.Progress.Load()
	if p.Phase != domain.PhaseWriting || p.LockedAt != nil {
		t.Fatalf("reopen must clear current lock: %+v", p)
	}
}
