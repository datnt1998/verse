// Smoke thật (tắt mặc định): chạy `VIDERSE_YT_SMOKE=1 go test -run TestLiveSmoke`.
// Chỉ đọc: trang đầu + một continuation của một kênh công khai.
package youtube

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveSmoke(t *testing.T) {
	if os.Getenv("VIDERSE_YT_SMOKE") != "1" {
		t.Skip("đặt VIDERSE_YT_SMOKE=1 để chạy smoke thật")
	}
	channel := os.Getenv("VIDERSE_YT_SMOKE_CHANNEL")
	if channel == "" {
		channel = "@YouTube"
	}
	c := NewClient(Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	first, err := c.FirstChannelPage(ctx, channel)
	if err != nil {
		t.Fatalf("trang đầu %s: %v", channel, err)
	}
	if len(first.Videos) == 0 {
		t.Fatal("trang đầu không có video")
	}
	t.Logf("trang đầu: %d video, profile=%q handle=%s", len(first.Videos), first.Profile.Title, textOrDash(first.Profile.Handle))
	for _, v := range first.Videos[:3] {
		t.Logf("  %s %q dur=%ds views=%d pub=%s", v.ID, v.Title, numOrZero(v.DurationSec), numOrZero(v.ViewCount), textOrDash(v.PublishedText))
	}
	if first.NextCursor == nil {
		t.Skip("kênh không có trang kế; bỏ qua phần continuation")
	}
	second, err := c.NextChannelPage(ctx, *first.NextCursor)
	if err != nil {
		t.Fatalf("trang kế: %v", err)
	}
	if len(second.Videos) == 0 {
		t.Fatal("trang kế không có video")
	}
	t.Logf("trang kế: %d video, đầu=%s", len(second.Videos), second.Videos[0].ID)
}

func numOrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func textOrDash(v *string) string {
	if v == nil {
		return "-"
	}
	return *v
}
