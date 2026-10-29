package host

import (
	"fmt"
	"slices"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

type ReopenCheck struct {
	ID       string `json:"id"`
	OK       bool   `json:"ok"`
	Label    string `json:"label"`
	Note     string `json:"note"`
	Chapters []int  `json:"chapters"`
}

type ReopenChecks struct {
	SchemaVersion int           `json:"schemaVersion"`
	Title         string        `json:"title"`
	Chapters      int           `json:"chapters"`
	Words         int           `json:"words"`
	LockedAt      *time.Time    `json:"lockedAt"`
	ClosedAt      *time.Time    `json:"closedAt"`
	Checks        []ReopenCheck `json:"checks"`
	Allowed       bool          `json:"allowed"`
}

// ReopenPreconditions reads the same persisted chapter state used by Reopen.
// It never accepts revisions or starts the engine.
func (h *Host) ReopenPreconditions() (ReopenChecks, error) {
	out := ReopenChecks{SchemaVersion: 1, Checks: []ReopenCheck{}, Allowed: true}
	p, err := h.store.Progress.Load()
	if err != nil {
		return out, err
	}
	book, err := h.store.Book.Load()
	if err != nil {
		return out, err
	}
	if book != nil {
		out.Title = book.Title
	}
	changed, err := h.CheckChapterRevisions()
	if err != nil {
		return out, err
	}
	h.mu.Lock()
	idle := h.lifecycle != lifecycleRunning && !h.cocreating && h.exclusive == ""
	h.mu.Unlock()
	complete := p != nil && p.Phase == domain.PhaseComplete
	blocked := append([]int{}, changed...)
	completed := 0
	if p != nil {
		out.Chapters, out.Words = len(p.CompletedChapters), p.TotalWordCount
		out.LockedAt, out.ClosedAt = p.LockedAt, p.LockedAt
		completed = len(p.CompletedChapters)
		for _, n := range p.PendingRewrites {
			if !slices.Contains(blocked, n) {
				blocked = append(blocked, n)
			}
		}
	}
	slices.Sort(blocked)
	clean := completed
	if p != nil {
		for _, n := range blocked {
			if slices.Contains(p.CompletedChapters, n) {
				clean--
			}
		}
	}
	note := "Các chương cũ được giữ nguyên"
	if len(blocked) > 0 {
		note = fmt.Sprintf("Các chương %v còn bản sửa chưa duyệt; hãy tiếp nhận bằng /sync hoặc hoàn tất viết lại", blocked)
	}
	out.Checks = []ReopenCheck{
		{ID: "completed", OK: complete, Label: "Truyện đã hoàn thành", Chapters: []int{}},
		{ID: "idle", OK: idle, Label: "Hệ thống đang rảnh, không có phiên đồng sáng tác hay tác vụ độc quyền", Chapters: []int{}},
		{ID: "clean", OK: len(blocked) == 0, Label: fmt.Sprintf("%d/%d chương đã duyệt sạch", clean, completed), Note: note, Chapters: blocked},
	}
	for _, check := range out.Checks {
		if !check.OK {
			out.Allowed = false
		}
	}
	return out, nil
}
