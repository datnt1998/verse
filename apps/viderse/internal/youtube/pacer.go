package youtube

import (
	"context"
	mrand "math/rand/v2"
	"net/http"
	"sync"
	"time"
)

// Pacer là bộ điều tốc toàn cục của một Client: tối đa một request chạy tại một
// thời điểm và request sau bắt đầu không sớm hơn [MinGap,MaxGap] (có jitter) sau
// khi phản hồi trước hoàn tất. HTTP 429 là tín hiệu chung: thời điểm Retry-After
// (30 giây nếu thiếu) đẩy lịch chung nên mọi caller đều chờ nó.
//
// Pacer không tự thử lại. Khi ô lịch nằm xa hơn ngân sách chờ của caller —
// deadline của context, hoặc MaxWait nếu context không có deadline — nó trả
// *DeferredError mang RetryAt thay vì chờ.
//
// Pacer dùng được cả qua NewPacer() lẫn literal &Pacer{...} (ruột khởi tạo lười).
type Pacer struct {
	// Now là đồng hồ; mặc định time.Now. Test tiêm đồng hồ giả.
	Now func() time.Time
	// MinGap/MaxGap là khoảng tối thiểu giữa lúc xong request này và bắt đầu
	// request kế; MaxGap < MinGap thì dùng MinGap. Cả hai 0 → mặc định 1s–2s.
	MinGap, MaxGap time.Duration
	// MaxWait là ngân sách chờ khi context không có deadline; mặc định 5s.
	MaxWait time.Duration
	// RateLimitBase là trễ khi 429 không có Retry-After; mặc định 30s.
	RateLimitBase time.Duration
	// Rand trả [0,1) cho jitter; nil → math/rand/v2 (nhịp thật). Test tiêm hàm 0 cho nhịp cố định.
	Rand func() float64

	initOnce     sync.Once
	sem          chan struct{} // giữ ô lịch trong suốt một request
	freed        chan struct{} // báo thức dậy waiter khi ô được nhả
	stateMu      sync.Mutex
	nextStart    time.Time
	penaltyUntil time.Time
}

// NewPacer tạo bộ điều tốc với giá trị mặc định của nhịp chia sẻ.
func NewPacer() *Pacer {
	p := &Pacer{Rand: mrand.Float64}
	p.ensure()
	return p
}

// ensure khởi tạo lười phần ruột, để literal &Pacer{} cũng dùng được.
func (p *Pacer) ensure() {
	p.initOnce.Do(func() {
		p.sem = make(chan struct{}, 1)
		p.freed = make(chan struct{}, 1)
	})
}

func (p *Pacer) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func (p *Pacer) gaps() (min, max time.Duration) {
	min, max = p.MinGap, p.MaxGap
	if min == 0 && max == 0 {
		return time.Second, 2 * time.Second
	}
	if max < min {
		max = min
	}
	return min, max
}

func (p *Pacer) jitter() time.Duration {
	min, max := p.gaps()
	rand := p.Rand
	if rand == nil {
		rand = mrand.Float64 // jitter mặc định có thật, như upstream Math.random
	}
	return min + time.Duration(rand()*float64(max-min+1))
}

func (p *Pacer) maxWait() time.Duration {
	if p.MaxWait > 0 {
		return p.MaxWait
	}
	return 5 * time.Second
}

func (p *Pacer) rateLimitBase() time.Duration {
	if p.RateLimitBase > 0 {
		return p.RateLimitBase
	}
	return 30 * time.Second
}

// Run chiếm ô lịch, chờ tới giờ được phép rồi gọi send. send nhận ctx gốc (bộ
// điều tốc trong-process không cần tín hiệu riêng). Trả (resp, err) của send;
// khi send cho HTTP 429, lịch chung bị đẩy theo Retry-After.
//
// Ngân sách chờ là tích luỹ: một mốc cutoff duy nhất (min(deadline ctx,
// now+MaxWait)) tính khi Run bắt đầu — tổng thời gian xếp hàng cộng chờ ô lịch
// không vượt nó, mọi ô lịch xa hơn trả *DeferredError thay vì chờ.
func (p *Pacer) Run(ctx context.Context, send func(context.Context) (*http.Response, error)) (*http.Response, error) {
	p.ensure()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cutoff, hasCutoff := p.cutoff(ctx)
	startAt, err := p.acquire(ctx, cutoff, hasCutoff)
	if err != nil {
		return nil, err
	}
	started := false
	defer func() {
		p.finish(started)
	}()
	if wait := startAt.Sub(p.now()); wait > 0 {
		if err := p.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	started = true
	resp, err := send(ctx)
	if err == nil && resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		at := p.now()
		delay := ParseRetryAfter(resp.Header.Get("Retry-After"), at)
		until := at.Add(p.rateLimitBase())
		if delay != nil {
			until = at.Add(*delay)
		}
		p.penalize(until)
	}
	return resp, err
}

// cutoff trả mốc tới hạn chờ duy nhất của lời gọi: sớm hơn trong hai mốc
// deadline của context và now+MaxWait. false khi không có giới hạn nào.
func (p *Pacer) cutoff(ctx context.Context) (time.Time, bool) {
	limit := p.now().Add(p.maxWait())
	if dl, ok := ctx.Deadline(); ok && dl.Before(limit) {
		limit = dl
	}
	return limit, true
}

// acquire chờ ô lịch rảnh rồi chốt thời điểm bắt đầu theo lịch chung. Vòng lặp:
// nếu bận thì ngủ ngắn rồi thử lại; lượt chờ nào đẩy tổng chờ qua cutoff thì
// trả DeferredError (retryAt = thời điểm caller nên thử lại).
func (p *Pacer) acquire(ctx context.Context, cutoff time.Time, hasCutoff bool) (time.Time, error) {
	for {
		if err := ctx.Err(); err != nil {
			return time.Time{}, err
		}
		select {
		case p.sem <- struct{}{}:
			// Ô trống: giờ bắt đầu là max(now, nextStart, penaltyUntil).
			p.stateMu.Lock()
			startAt := p.now()
			if p.nextStart.After(startAt) {
				startAt = p.nextStart
			}
			if p.penaltyUntil.After(startAt) {
				startAt = p.penaltyUntil
			}
			p.stateMu.Unlock()
			if hasCutoff && startAt.After(cutoff) {
				<-p.sem // nhả ô ngay; request chưa chạy nên không dời lịch
				return time.Time{}, &DeferredError{RetryAt: startAt}
			}
			return startAt, nil
		default:
		}
		// Đang có request khác chạy: đợi nó xong hoặc hết ngân sách.
		wait := 250 * time.Millisecond
		if hasCutoff {
			if remaining := cutoff.Sub(p.now()); remaining <= 0 {
				return time.Time{}, &DeferredError{RetryAt: cutoff}
			} else if remaining < wait {
				wait = remaining
			}
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return time.Time{}, ctx.Err()
		case <-p.freed:
			t.Stop()
		case <-t.C:
		}
	}
}

// finish giải phóng ô lịch; request đã chạy thì đẩy lịch chung thêm một nhịp jitter.
func (p *Pacer) finish(started bool) {
	if started {
		p.stateMu.Lock()
		next := p.now().Add(p.jitter())
		if next.After(p.nextStart) {
			p.nextStart = next
		}
		p.stateMu.Unlock()
	}
	<-p.sem
	select {
	case p.freed <- struct{}{}:
	default:
	}
}

// penalize đẩy lịch chung tới until (tín hiệu 429 toàn cục).
func (p *Pacer) penalize(until time.Time) {
	p.ensure()
	p.stateMu.Lock()
	if until.After(p.penaltyUntil) {
		p.penaltyUntil = until
	}
	p.stateMu.Unlock()
}

func (p *Pacer) sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
