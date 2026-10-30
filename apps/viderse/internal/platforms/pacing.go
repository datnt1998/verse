package platforms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Pacer nhịp hoá các lời gọi tới một nền tảng (chuyển từ pacedRequest của
// yt-factory): đặt chỗ theo khoảng cách tối thiểu, từ chối khi đang trong giờ
// chờ, xoá backoff sau thành công và đặt giờ chờ tăng dần khi bị chặn.
type Pacer struct {
	svc    *Service
	sleep  func(ctx context.Context, d time.Duration) error
	random func() float64
}

// NewPacer dựng bộ nhịp trên cùng store của service; sleep/random tiêm được
// để test không chờ thật (nil → mặc định hệ thống).
func (s *Service) NewPacer(sleep func(ctx context.Context, d time.Duration) error, random func() float64) *Pacer {
	if sleep == nil {
		sleep = sleepCtx
	}
	if random == nil {
		random = randFloat
	}
	return &Pacer{svc: s, sleep: sleep, random: random}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func randFloat() float64 {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<30))
	if err != nil {
		return 0
	}
	return float64(n.Int64()) / (1 << 30)
}

// Request chạy MỘT lời gọi có nhịp tới nền tảng (không phải cả một lượt trích
// nhiều trang). resetBackoff=false cho các lời gọi tiện ích (nav, thiết bị)
// không được xoá backoff của trang bị chặn — giữ ngữ nghĩa của bản TS.
func (p *Pacer) Request(ctx context.Context, id domain.PlatformID, resetBackoff bool, request func(ctx context.Context) error) error {
	d, ok := Lookup(id)
	if !ok {
		return ErrPlatformUnknown
	}
	spacing := time.Duration(0)
	if id == domain.PlatformBilibili {
		spacing = bilibiliSpacing + time.Duration(p.random()*bilibiliJitterMs)*time.Millisecond
	}
	startAt, err := p.svc.d.Store.ReserveSlot(ctx, id, p.svc.now(), spacing)
	if err != nil {
		return err
	}
	if wait := startAt.Sub(p.svc.now()); wait > 0 {
		if err := p.sleep(ctx, wait); err != nil {
			return err
		}
	}
	until, reason, err := p.svc.d.Store.Pause(ctx, id, p.svc.now())
	if err != nil {
		return err
	}
	if until != nil {
		return &PausedError{Platform: id, Label: d.Label, Until: *until, Reason: strValue(reason)}
	}
	if err := request(ctx); err != nil {
		if id == domain.PlatformBilibili && isRateLimited(err) {
			return p.pause(ctx, d, err)
		}
		return err
	}
	if resetBackoff {
		if err := p.svc.d.Store.ClearPause(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// isRateLimited nhận diện lỗi "bị giới hạn yêu cầu": client trả Retry-After
// qua RateLimited, hoặc mang mã "rate_limited" qua CodedError (yt-dlp).
func isRateLimited(err error) bool {
	var rl RateLimited
	if errors.As(err, &rl) {
		return true
	}
	var coded CodedError
	return errors.As(err, &coded) && coded.ErrorCode() == ErrCodeRateLimited
}

func retryAfterOf(err error) time.Duration {
	var rl RateLimited
	if errors.As(err, &rl) {
		return rl.RetryAfter()
	}
	return 0
}

// pause ghi lần bị chặn và trả lỗi "tạm nghỉ" đã dịch thay cho chi tiết thô.
func (p *Pacer) pause(ctx context.Context, d Descriptor, cause error) error {
	st, err := p.svc.d.Store.RecordBlocked(ctx, d.ID, p.svc.now(), pauseReason(d), retryAfterOf(cause))
	if err != nil {
		return err
	}
	p.svc.notify(d.ID)
	until := p.svc.now()
	if st.PausedUntil != nil {
		until = *st.PausedUntil
	}
	return &PausedError{Platform: d.ID, Label: d.Label, Until: until, Reason: strValue(st.PauseReason)}
}

// WithScanner chạy một lượt quét kênh của nền tảng dưới lease chéo process
// (chuyển từ withBilibiliScanner): chỉ một scanner hoạt động mỗi lúc, lease
// 30 phút tự hết nếu process chết, nhả khi xong kể cả lỗi.
func (p *Pacer) WithScanner(ctx context.Context, id domain.PlatformID, scan func(ctx context.Context) error) error {
	d, ok := Lookup(id)
	if !ok {
		return ErrPlatformUnknown
	}
	owner := newOwner()
	now := p.svc.now()
	ok2, err := p.svc.d.Store.AcquireScanner(ctx, id, owner, now, now.Add(scannerLease))
	if err != nil {
		return err
	}
	if !ok2 {
		return fmt.Errorf("một kênh %s đang được quét. Hãy đợi lượt quét đó xong rồi thử lại", d.Label)
	}
	defer func() { _ = p.svc.d.Store.ReleaseScanner(context.Background(), id, owner) }()
	return scan(ctx)
}

func newOwner() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b[:])
}

func strValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
