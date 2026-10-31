package platforms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Pacer nhịp hoá các lời gọi tới một nền tảng (chuyển từ pacedRequest của
// yt-factory): đặt chỗ theo khoảng cách tối thiểu, kiểm hạn mức cuộn, từ chối
// khi đang trong giờ chờ, xoá backoff sau thành công và đặt giờ chờ tăng dần
// khi bị chặn.
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

// BudgetOf tính hạn mức cuộn từ lịch sử đặt chỗ (chuyển từ windowBudget của
// pacing.ts): một cửa sổ còn nhận yêu cầu khi tổng trọng số của các lần trong
// cửa sổ cộng thêm cost không vượt Limit; nếu vượt, cửa sổ nhả chỗ dần khi
// các lần cũ nhất trượt khỏi cửa sổ. Hàm thuần — Postgres và test dùng chung.
func BudgetOf(history []domain.PlatformRequest, now time.Time, windows []BudgetWindow, cost int) BudgetReport {
	if cost < 1 {
		cost = 1
	}
	out := BudgetReport{Windows: make([]WindowBudget, 0, len(windows)), AvailableAt: now}
	for _, w := range windows {
		active := make([]domain.PlatformRequest, 0, len(history))
		used := 0
		for _, e := range history {
			if e.At.After(now.Add(-w.Window)) {
				active = append(active, e)
				used += e.Cost
			}
		}
		sort.SliceStable(active, func(i, j int) bool { return active[i].At.Before(active[j].At) })
		remaining := used
		availableAt := now
		for _, e := range active {
			if remaining+cost <= w.Limit {
				break
			}
			remaining -= e.Cost
			availableAt = e.At.Add(w.Window)
		}
		out.Windows = append(out.Windows, WindowBudget{Window: w.Window, Used: used, Limit: w.Limit, AvailableAt: availableAt})
		if availableAt.After(out.AvailableAt) {
			out.AvailableAt = availableAt
		}
	}
	return out
}

// PruneHistory bỏ các lần đặt chỗ cũ hơn trần giữ lịch sử (một ngày).
func PruneHistory(history []domain.PlatformRequest, now time.Time) []domain.PlatformRequest {
	out := history[:0]
	for _, e := range history {
		if e.At.After(now.Add(-requestHistoryWindow)) {
			out = append(out, e)
		}
	}
	return out
}

// Request chạy MỘT lời gọi có nhịp tới nền tảng (không phải cả một lượt trích
// nhiều trang). resetBackoff=false cho các lời gọi tiện ích (nav, thiết bị)
// không được xoá backoff của trang bị chặn — giữ ngữ nghĩa của bản TS.
func (p *Pacer) Request(ctx context.Context, id domain.PlatformID, resetBackoff bool, request func(ctx context.Context) error) error {
	return p.RequestCost(ctx, id, resetBackoff, 1, request)
}

// RequestCost như Request nhưng khai trọng số của lần gọi (một trang = 1, một
// lượt trích yt-dlp = 4, đọc lại info đã lưu = 1). Nền tảng có hạn mức cuộn
// đi qua đặt chỗ có trọng số; nền tảng khác giữ nguyên nhịp cũ.
func (p *Pacer) RequestCost(ctx context.Context, id domain.PlatformID, resetBackoff bool, cost int, request func(ctx context.Context) error) error {
	d, ok := Lookup(id)
	if !ok {
		return ErrPlatformUnknown
	}
	return p.paced(ctx, d, resetBackoff, cost, 0, request)
}

// paced là ruột chung của RequestCost/RunYtDlp. spacingOverride > 0 thay cho
// khoảng cách mặc định của nền tảng (yt-dlp hẹp hơn lời gọi API).
func (p *Pacer) paced(ctx context.Context, d Descriptor, resetBackoff bool, cost int, spacingOverride time.Duration, request func(ctx context.Context) error) error {
	spacing := spacingOverride
	if spacing == 0 && len(d.Budget) > 0 {
		spacing = bilibiliSpacing + time.Duration(p.random()*float64(bilibiliJitterMs))*time.Millisecond
	}
	startAt, err := p.reserve(ctx, d, cost, spacing)
	if err != nil {
		return err
	}
	if wait := startAt.Sub(p.svc.now()); wait > 0 {
		if err := p.sleep(ctx, wait); err != nil {
			return err
		}
	}
	return p.execute(ctx, d, resetBackoff, request)
}

// reserve đặt chỗ và trả thời điểm được bắt đầu; từ chối khi đang chờ hoặc hết
// hạn mức (không ghi gì trong cả hai trường hợp).
func (p *Pacer) reserve(ctx context.Context, d Descriptor, cost int, spacing time.Duration) (time.Time, error) {
	if len(d.Budget) == 0 {
		return p.svc.d.Store.ReserveSlot(ctx, d.ID, p.svc.now(), spacing)
	}
	decision, err := p.svc.d.Store.ReserveWeighted(ctx, d.ID, p.svc.now(), SlotRequest{Spacing: spacing, Cost: cost, Windows: d.Budget})
	if err != nil {
		return time.Time{}, err
	}
	if decision.PausedUntil != nil {
		return time.Time{}, &PausedError{Platform: d.ID, Label: d.Label, Until: *decision.PausedUntil, Reason: strValue(decision.PauseReason)}
	}
	if decision.BudgetAt != nil {
		return time.Time{}, &BudgetError{Platform: d.ID, Label: d.Label, AvailableAt: *decision.BudgetAt}
	}
	return decision.StartAt, nil
}

// execute chạy lời gọi tại thời điểm đã đặt chỗ: kiểm giờ chờ lần cuối, phân
// loại lỗi để đặt giờ chờ, xoá backoff sau thành công.
func (p *Pacer) execute(ctx context.Context, d Descriptor, resetBackoff bool, request func(ctx context.Context) error) error {
	until, reason, err := p.svc.d.Store.Pause(ctx, d.ID, p.svc.now())
	if err != nil {
		return err
	}
	if until != nil {
		return &PausedError{Platform: d.ID, Label: d.Label, Until: *until, Reason: strValue(reason)}
	}
	if err := request(ctx); err != nil {
		if len(d.Budget) > 0 && isRateLimited(err) {
			var budget *BudgetError
			if errors.As(err, &budget) {
				return err
			}
			return p.pause(ctx, d, err)
		}
		return err
	}
	if resetBackoff {
		if err := p.svc.d.Store.ClearPause(ctx, d.ID); err != nil {
			return err
		}
	}
	return nil
}

// RunYtDlp chạy một lệnh yt-dlp có nhịp. Nền tảng có hạn mức (Bilibili) đi
// qua lease chéo process để mọi process chỉ chạy một lệnh mỗi lúc. Lease phủ
// thời điểm bắt đầu THẬT sau khi đặt chỗ + timeout + đệm: giữ lease trước,
// đặt chỗ, rồi gia hạn lease tới StartAt+timeout+đệm; lệnh chạy dưới deadline
// của lease (cả bước cài binary lẫn tiến trình đều bị cắt khi hết lease).
func (p *Pacer) RunYtDlp(ctx context.Context, id domain.PlatformID, cost int, timeout time.Duration, run func(ctx context.Context) error) error {
	d, ok := Lookup(id)
	if !ok {
		return ErrPlatformUnknown
	}
	if len(d.Budget) == 0 {
		return p.paced(ctx, d, true, cost, 0, run)
	}
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	owner := newOwner()
	until := time.Time{}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		now := p.svc.now()
		st, _, err := p.svc.d.Store.One(ctx, id)
		if err != nil {
			return err
		}
		leaseStart := now
		if st.NextRequestAt.After(leaseStart) {
			leaseStart = st.NextRequestAt
		}
		until = leaseStart.Add(timeout + ytDlpLeaseMargin)
		acquired, err := p.svc.d.Store.AcquireYtDlp(ctx, id, owner, now, until)
		if err != nil {
			return err
		}
		if acquired {
			break
		}
		if pauseUntil, reason, err := p.svc.d.Store.Pause(ctx, id, p.svc.now()); err != nil {
			return err
		} else if pauseUntil != nil {
			return &PausedError{Platform: id, Label: d.Label, Until: *pauseUntil, Reason: strValue(reason)}
		}
		if err := p.sleep(ctx, time.Second); err != nil {
			return err
		}
	}
	defer func() { _ = p.svc.d.Store.ReleaseYtDlp(context.Background(), id, owner) }()
	spacing := bilibiliYtDlpSpacing + time.Duration(p.random()*float64(bilibiliYtDlpJitterMs))*time.Millisecond
	startAt, err := p.reserve(ctx, d, cost, spacing)
	if err != nil {
		return err
	}
	// Đặt chỗ có thể lùi thời điểm bắt đầu so với mốc đọc lúc giữ lease.
	runUntil := startAt.Add(timeout + ytDlpLeaseMargin)
	if runUntil.After(until) {
		extended, err := p.svc.d.Store.ExtendYtDlp(ctx, id, owner, runUntil)
		if err != nil {
			return err
		}
		if !extended {
			return fmt.Errorf("mất lease yt-dlp %s khi chờ tới lượt", id)
		}
		until = runUntil
	}
	if wait := startAt.Sub(p.svc.now()); wait > 0 {
		if err := p.sleep(ctx, wait); err != nil {
			return err
		}
	}
	runCtx, cancel := context.WithDeadline(ctx, until)
	defer cancel()
	return p.execute(runCtx, d, true, run)
}

// isRateLimited nhận diện lỗi "bị giới hạn yêu cầu". Lỗi có mã thì mã quyết
// định (Retry-After chỉ là metadata kéo dài giờ chờ, không biến lỗi 404/500
// thành bị chặn); lỗi không mã mà có RetryAfter mới coi là bị chặn.
func isRateLimited(err error) bool {
	var coded CodedError
	if errors.As(err, &coded) {
		return coded.ErrorCode() == ErrCodeRateLimited
	}
	var rl RateLimited
	return errors.As(err, &rl)
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
	return p.WithScannerLease(ctx, id, scannerLease, scan)
}

// WithScannerLease như WithScanner với thời hạn lease chọn được (Bilibili
// dùng 5 phút: một trang danh sách xong trong vài giây).
func (p *Pacer) WithScannerLease(ctx context.Context, id domain.PlatformID, lease time.Duration, scan func(ctx context.Context) error) error {
	d, ok := Lookup(id)
	if !ok {
		return ErrPlatformUnknown
	}
	owner := newOwner()
	now := p.svc.now()
	acquired, err := p.svc.d.Store.AcquireScanner(ctx, id, owner, now, now.Add(lease))
	if err != nil {
		return err
	}
	if !acquired {
		return &ScannerBusyError{Platform: id, Label: d.Label}
	}
	defer func() { _ = p.svc.d.Store.ReleaseScanner(context.Background(), id, owner) }()
	// Lượt quét bị cắt khi lease hết hạn: không giữ tài nguyên quá hạn.
	scanCtx, cancel := context.WithDeadline(ctx, now.Add(lease))
	defer cancel()
	return scan(scanCtx)
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
