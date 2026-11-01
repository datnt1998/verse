package web

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

// Bộ lọc thời lượng Tự lấy theo đối thủ đi qua đủ đường lưu → đọc liên kết →
// truy vấn ứng viên: video ngắn hơn ngưỡng không thành ứng viên.
func TestAutoPickMinDurationFiltersEligibleVideos(t *testing.T) {
	_, pool := planFixture(t)
	ctx := context.Background()
	const owner, rival = "11111111-1111-4111-8111-111111111111", "UCaaaaaaaaaaaaaaaaaaaaaa"
	channels := postgres.NewChannelStore(pool)
	queueStore := postgres.NewQueueStore(pool)
	baseline := time.Now().Add(-24 * time.Hour)

	ids, err := queueStore.ListEligibleVideos(ctx, rival, baseline, owner, 0)
	if err != nil || len(ids) != 2 {
		t.Fatalf("không lọc: ids=%v err=%v, muốn 2 video", ids, err)
	}
	threshold := 150
	found, err := channels.SetSourceMinDuration(ctx, owner, rival, &threshold)
	if err != nil || !found {
		t.Fatalf("SetSourceMinDuration found=%v err=%v", found, err)
	}
	links, err := queueStore.ListSourcesWithChannels(ctx, owner)
	if err != nil || len(links) != 1 || links[0].MinDurationSec == nil || *links[0].MinDurationSec != threshold {
		t.Fatalf("liên kết không mang bộ lọc: %+v err=%v", links, err)
	}
	ids, err = queueStore.ListEligibleVideos(ctx, rival, baseline, owner, *links[0].MinDurationSec)
	if err != nil || len(ids) != 1 || ids[0] != "bbbbbbbbbbb" {
		t.Fatalf("lọc ≥150s: ids=%v err=%v, muốn chỉ bbbbbbbbbbb (180s)", ids, err)
	}
	if found, err = channels.SetSourceMinDuration(ctx, owner, "UCbbbbbbbbbbbbbbbbbbbbbb", &threshold); err != nil || found {
		t.Fatalf("đối thủ chưa liên kết: found=%v err=%v", found, err)
	}
	if found, err = channels.SetSourceMinDuration(ctx, owner, rival, nil); err != nil || !found {
		t.Fatalf("bỏ lọc found=%v err=%v", found, err)
	}
	links, err = queueStore.ListSourcesWithChannels(ctx, owner)
	if err != nil || links[0].MinDurationSec != nil {
		t.Fatalf("bỏ lọc nhưng còn %v err=%v", links[0].MinDurationSec, err)
	}
}
