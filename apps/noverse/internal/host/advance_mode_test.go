package host

import (
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// 26/10: đổi sang chế độ tự động khi sáng tác đang dừng phải chỉ người dùng bấm nút
// “Tiếp tục” trên giao diện, không bảo họ gõ lệnh (TUI không có nút, web có nút).
func TestSetAdvanceModeStoppedSummaryPointsAtResumeButton(t *testing.T) {
	for _, state := range []lifecycle{lifecycleIdle, lifecyclePaused} {
		t.Run(string(state), func(t *testing.T) {
			st := storepkg.NewStore(t.TempDir())
			if err := st.Init(); err != nil {
				t.Fatalf("store init: %v", err)
			}
			if err := st.RunMeta.Init("default", "test", "test"); err != nil {
				t.Fatalf("run meta init: %v", err)
			}
			h := &Host{store: st, events: make(chan Event, 8), lifecycle: state}

			if err := h.SetAdvanceMode(domain.ChapterAdvanceAuto); err != nil {
				t.Fatalf("SetAdvanceMode: %v", err)
			}
			summary := (<-h.events).Summary
			if !strings.Contains(summary, "Đã chuyển cách đẩy chương: Tự động") {
				t.Fatalf("summary thiếu phần đổi chế độ: %q", summary)
			}
			if strings.Contains(summary, "nhập lệnh") {
				t.Fatalf("summary vẫn bảo người dùng gõ lệnh: %q", summary)
			}
			if !strings.Contains(summary, "bấm Tiếp tục") {
				t.Fatalf("summary thiếu chỉ dẫn bấm Tiếp tục: %q", summary)
			}
		})
	}
}

func TestSetAdvanceModeRunningSummaryHasNoResumeHint(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("store init: %v", err)
	}
	if err := st.RunMeta.Init("default", "test", "test"); err != nil {
		t.Fatalf("run meta init: %v", err)
	}
	h := &Host{store: st, events: make(chan Event, 8), lifecycle: lifecycleRunning}
	if err := h.SetAdvanceMode(domain.ChapterAdvanceAuto); err != nil {
		t.Fatalf("SetAdvanceMode: %v", err)
	}
	summary := (<-h.events).Summary
	if summary != "Đã chuyển cách đẩy chương: Tự động" {
		t.Fatalf("đang chạy không được thêm lời nhắc tạm dừng: %q", summary)
	}
}
