package identity

import (
	"sync"
	"time"
)

// WindowLimiter 是滑动窗口限流：同一键在 window 内最多记 max 次失败，之后一律拒绝，
// 直到最早一次失败滑出窗口。仅存内存：重启 serve 即清零。
// 用于恢复码（每 IP 每小时 5 次）——码是一次性的高熵凭据，比登录更适合硬上限而非指数退避。
type WindowLimiter struct {
	mu     sync.Mutex
	now    func() time.Time
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func NewWindowLimiter(max int, window time.Duration) *WindowLimiter {
	return &WindowLimiter{now: time.Now, max: max, window: window, hits: make(map[string][]time.Time)}
}

// prune 丢弃窗口外的记录；调用方持锁。
func (l *WindowLimiter) prune(key string, now time.Time) []time.Time {
	hits := l.hits[key]
	i := 0
	for i < len(hits) && !hits[i].Add(l.window).After(now) {
		i++
	}
	hits = hits[i:]
	if len(hits) == 0 {
		delete(l.hits, key)
		return nil
	}
	l.hits[key] = hits
	return hits
}

// Wait 返回 key 还需等待多久才能再次尝试；0 表示可以尝试。
func (l *WindowLimiter) Wait(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	hits := l.prune(key, now)
	if len(hits) < l.max {
		return 0
	}
	return hits[0].Add(l.window).Sub(now)
}

// Fail 记一次失败。
func (l *WindowLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.hits) > limiterCap {
		for k := range l.hits {
			l.prune(k, now)
		}
	}
	l.hits[key] = append(l.prune(key, now), now)
}

// Succeed 清空 key 的失败记录。
func (l *WindowLimiter) Succeed(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}
