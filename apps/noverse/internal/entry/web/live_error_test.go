package web

import (
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

func TestLiveHistoryErrorMessagesAreSafeAndSpecific(t *testing.T) {
	tests := []struct {
		name, kind, tool string
		chapter          int
		want             string
	}{
		{"validation", "tool_validation", "commit_chapter", 4, "Thông tin thao tác ở chương 4 chưa hợp lệ. Trợ lý sẽ kiểm tra và thử lại."},
		{"timeout", "timeout", "save_review", 5, "Thao tác ở chương 5 đã quá thời gian chờ. Trợ lý sẽ kiểm tra và thử lại."},
		{"provider", "network", "draft_chapter", 0, "Không kết nối được nhà cung cấp AI. Trợ lý sẽ kiểm tra và thử lại."},
		{"tool", "blocked", "edit_chapter", 7, "Thao tác ở chương 7 chưa thể thực hiện. Trợ lý sẽ kiểm tra và thử lại."},
		{"unknown", "unknown", "edit_chapter", 4, "Thao tác không thành công. Trợ lý tự xem lỗi và chạy lại ở thẻ kế tiếp."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			at := time.Unix(100, 0)
			ev := host.Event{ID: "op", Time: at.Add(time.Second), FinishedAt: at.Add(2 * time.Second), Category: "TOOL", Agent: "writer", Tool: tc.tool, Kind: tc.kind, Chapter: tc.chapter, Status: "error", Failed: true, Result: "token=sk-secret /private/path", Summary: "secret /private/path"}
			rounds, err := liveHistoryRounds([]domain.RuntimeQueueItem{{Payload: host.Event{ID: "round", Category: "DISPATCH", Agent: "writer", Time: at}}, {Payload: ev}})
			if err != nil || len(rounds) != 1 || len(rounds[0].Operations) != 1 {
				t.Fatalf("rounds=%+v err=%v", rounds, err)
			}
			got := rounds[0].Operations[0].Error
			if got != tc.want {
				t.Fatalf("error = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "secret") || strings.Contains(got, "/private") {
				t.Fatalf("leaked raw data: %q", got)
			}
		})
	}
}
