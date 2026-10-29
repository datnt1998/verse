// Ca test bộ điều tốc: xếp hàng, nhịp tối thiểu, phạt 429, DeferredError khi ô
// lịch quá xa ngân sách, huỷ trong lúc chờ. Tương đương phần governor của
// yt-factory src/server/channels/scan-worker.test.ts.
package youtube

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

func okResp() *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: http.NoBody}
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestPacerSerialisesRequests(t *testing.T) {
	p := &Pacer{MinGap: time.Nanosecond, MaxGap: time.Nanosecond}
	var mu sync.Mutex
	running := 0
	maxSeen := 0
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Run(context.Background(), func(ctx context.Context) (*http.Response, error) {
				mu.Lock()
				running++
				if running > maxSeen {
					maxSeen = running
				}
				mu.Unlock()
				time.Sleep(time.Millisecond)
				mu.Lock()
				running--
				mu.Unlock()
				return okResp(), nil
			})
			if err != nil {
				t.Errorf("Run: %v", err)
			}
		}()
	}
	wg.Wait()
	if maxSeen != 1 {
		t.Fatalf("maxSeen = %d, muốn 1", maxSeen)
	}
}

func TestPacerSpacesStartsByJitteredGap(t *testing.T) {
	// Đồng hồ thật: chờ ngắn để kiểm nhịp mà không đánh cược deadline.
	p := &Pacer{MinGap: 30 * time.Millisecond, MaxGap: 30 * time.Millisecond, Rand: func() float64 { return 0 }}
	var sent []time.Time
	send := func(ctx context.Context) (*http.Response, error) {
		sent = append(sent, time.Now())
		return okResp(), nil
	}
	if _, err := p.Run(context.Background(), send); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Run(context.Background(), send); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 {
		t.Fatalf("sent = %v", sent)
	}
	if gap := sent[1].Sub(sent[0]); gap < 30*time.Millisecond {
		t.Fatalf("request 2 chạy sớm: gap=%v", gap)
	}
}

func TestPacerPenalty429PushesSchedule(t *testing.T) {
	now := time.Now()
	clock := &fakeClock{t: now}
	p := &Pacer{Now: clock.Now, MinGap: time.Nanosecond, MaxGap: time.Nanosecond, Rand: func() float64 { return 0 }}
	rateLimited := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": {"120"}},
		Body:       http.NoBody,
	}
	send := func(ctx context.Context) (*http.Response, error) { return rateLimited, nil }
	if _, err := p.Run(context.Background(), send); err != nil {
		t.Fatal(err)
	}
	// Ô lịch bị đẩy tới +120s: caller với ngân sách nhỏ nhận DeferredError mang retryAt.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := p.Run(ctx, send)
	var de *DeferredError
	if !errors.As(err, &de) {
		t.Fatalf("mong đợi *DeferredError, nhận %v", err)
	}
	if de.RetryAt.Before(now.Add(119 * time.Second)) {
		t.Fatalf("retryAt = %v", de.RetryAt)
	}
}

func TestPacer429WithoutRetryAfterUsesBase(t *testing.T) {
	now := time.Now()
	clock := &fakeClock{t: now}
	p := &Pacer{Now: clock.Now, MinGap: time.Nanosecond, MaxGap: time.Nanosecond, RateLimitBase: 30 * time.Second}
	rateLimited := &http.Response{StatusCode: 429, Header: http.Header{}, Body: http.NoBody}
	send := func(ctx context.Context) (*http.Response, error) { return rateLimited, nil }
	if _, err := p.Run(context.Background(), send); err != nil {
		t.Fatal(err)
	}
	clock.Advance(31 * time.Second)
	// Hết cửa sổ phạt: request chạy ngay (nhịp nanosecond).
	if _, err := p.Run(context.Background(), send); err != nil {
		t.Fatalf("Run sau penalty: %v", err)
	}
}

func TestPacerCancelDuringWait(t *testing.T) {
	p := &Pacer{MinGap: 500 * time.Millisecond, MaxGap: 500 * time.Millisecond}
	send := func(ctx context.Context) (*http.Response, error) { return okResp(), nil }
	if _, err := p.Run(context.Background(), send); err != nil {
		t.Fatal(err)
	}
	// Request kế phải chờ ~500ms (dưới MaxWait); huỷ ctx trong lúc chờ trả ctx.Err.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := p.Run(ctx, send)
		done <- err
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	// Request bị huỷ trước khi chạy không được dời lịch thêm.
	if _, err := p.Run(context.Background(), send); err != nil {
		t.Fatalf("Run sau cancel: %v", err)
	}
}

func TestPacerDeferredBeyondCallerDeadline(t *testing.T) {
	now := time.Now()
	clock := &fakeClock{t: now}
	p := &Pacer{Now: clock.Now, MinGap: time.Nanosecond, MaxGap: time.Nanosecond}
	p.penalize(now.Add(10 * time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.Run(ctx, func(ctx context.Context) (*http.Response, error) { return okResp(), nil })
	var de *DeferredError
	if !errors.As(err, &de) {
		t.Fatalf("mong đợi *DeferredError, nhận %v", err)
	}
	if !de.RetryAt.Equal(now.Add(10 * time.Second)) {
		t.Fatalf("retryAt = %v", de.RetryAt)
	}
	// Cùng lịch đó nhưng không deadline: chờ MaxWait rồi cũng Deferred.
	p2 := &Pacer{Now: clock.Now, MinGap: time.Nanosecond, MaxGap: time.Nanosecond, MaxWait: time.Millisecond}
	p2.penalize(now.Add(10 * time.Second))
	_, err = p2.Run(context.Background(), func(ctx context.Context) (*http.Response, error) { return okResp(), nil })
	if !errors.As(err, &de) {
		t.Fatalf("không deadline vẫn phải Deferred: %v", err)
	}
}

func TestPacerCancelledBeforeSendSendsNothing(t *testing.T) {
	p := &Pacer{MinGap: time.Nanosecond, MaxGap: time.Nanosecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, err := p.Run(ctx, func(ctx context.Context) (*http.Response, error) {
		called = true
		return okResp(), nil
	})
	if called {
		t.Fatal("send được gọi dù ctx đã huỷ")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
