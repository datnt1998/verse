package backdrop

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// RunScheduler đọc dần danh mục Bilibili: ngủ tới đúng mốc kế tiếp (trang kế
// hoặc lúc hết giờ chờ / hạn mức), không hỏi vòng khi rảnh. Thức dậy sớm khi
// Wake được gọi (thêm kênh, làm mới, đăng nhập mới, gỡ chặn).
func (s *Service) RunScheduler(ctx context.Context) {
	// Dọn lượt tải mồ côi của process chết trước (lease hết hạn) để người dùng
	// thêm lại được thay vì thấy "đang tải" mãi.
	if cleared, err := s.d.Store.ClearStaleDownloads(ctx, s.now()); err == nil && cleared > 0 {
		s.notify("")
	}
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		due, ok, err := s.nextDue(ctx)
		if err != nil {
			if !s.waitWake(ctx, time.Minute) {
				return
			}
			continue
		}
		if !ok {
			if !s.waitWake(ctx, 0) {
				return
			}
			continue
		}
		wait := due.Sub(s.now())
		if wait <= 0 {
			if _, err := s.Trickle(ctx); err != nil {
				if !s.waitWake(ctx, time.Minute) {
					return
				}
			}
			continue
		}
		if !s.waitWake(ctx, wait) {
			return
		}
	}
}

// waitWake ngủ `d` (0 = tới khi được đánh thức) hoặc tới khi Wake/ctx xong.
// Trả false khi ctx đã huỷ.
func (s *Service) waitWake(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return false
		case <-s.wake:
			return true
		}
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-s.wake:
		return true
	case <-timer.C:
		return true
	}
}

// nextDue trả mốc sớm nhất được đọc trang kế tiếp của Bilibili; ok=false khi
// không kênh nào còn trang chưa đọc.
func (s *Service) nextDue(ctx context.Context) (time.Time, bool, error) {
	status, err := s.bilibiliStatus(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	if len(status.Pending) == 0 || status.NextPageAt == nil {
		return time.Time{}, false, nil
	}
	return *status.NextPageAt, true, nil
}

// Trickle đọc một trang của kênh chờ lâu nhất nếu đã tới mốc. Trả served=true
// khi vừa đọc xong một trang. Lỗi nền tảng không nổi lên: trang hỏng chỉ được
// thử lại ở lượt sau, danh mục đệm và con trỏ còn nguyên.
func (s *Service) Trickle(ctx context.Context) (bool, error) {
	status, err := s.bilibiliStatus(ctx)
	if err != nil {
		return false, err
	}
	if len(status.Pending) == 0 || status.NextPageAt == nil {
		return false, nil
	}
	now := s.now()
	if status.NextPageAt.After(now) {
		return false, nil
	}
	if status.PausedUntil != nil && status.PausedUntil.After(now) {
		return false, nil
	}
	channel, found, err := s.d.Store.Channel(ctx, status.Pending[0].ChannelID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	link := ChannelLink{Platform: channel.Platform, Handle: channel.Handle, URL: channel.URL}
	if _, err := s.scanBilibili(ctx, link, false, true); err != nil {
		// Trang hỏng: chỉ thử lại ở lượt sau, không nổi lên scheduler.
		s.scheduleNextTrickle(ctx)
		return false, nil
	}
	return true, nil
}

// scheduleNextTrickle đặt mốc đọc tiếp sau một lượt hỏng.
func (s *Service) scheduleNextTrickle(ctx context.Context) {
	if s.d.Platforms == nil {
		return
	}
	at := s.now().Add(bilibiliTrickleMin + time.Duration(s.random()*float64(bilibiliTrickleJitter)))
	_ = s.d.Platforms.SetNextTrickle(ctx, domain.PlatformBilibili, at)
}
