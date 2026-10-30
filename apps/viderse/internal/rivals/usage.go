package rivals

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// UsageCounter đếm kênh đối thủ làm nguồn của YouTube cho /api/platforms —
// thoả platforms.UsageCounter mà không import platforms (interface nhỏ nhất:
// chỉ cần hàm Usage trả map theo nền tảng).
type UsageCounter struct {
	store Store
}

// NewUsageCounter dựng bộ đếm usage trên store đọc đối thủ.
func NewUsageCounter(store Store) *UsageCounter {
	return &UsageCounter{store: store}
}

// Usage trả số đếm một lượt cho mọi nền tảng: hiện chỉ YouTube có khái niệm
// đối thủ (PlatformUsage.Rivals là con trỏ vì chỉ một số nền tảng có).
func (c *UsageCounter) Usage(ctx context.Context) (map[domain.PlatformID]domain.PlatformUsage, error) {
	n, err := c.store.CountRivalChannels(ctx)
	if err != nil {
		return nil, err
	}
	count := int(n)
	return map[domain.PlatformID]domain.PlatformUsage{
		domain.PlatformYouTube: {Rivals: &count},
	}, nil
}
