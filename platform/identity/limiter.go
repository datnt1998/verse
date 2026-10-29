package identity

import (
	"sync"
	"time"
)

// 连续失败 freeFailures 次后开始锁定，锁定时长从 baseLock 起每次翻倍，封顶 maxLock。
const (
	freeFailures = 5
	baseLock     = 30 * time.Second
	maxLock      = 15 * time.Minute
	limiterCap   = 10000
)

type failure struct {
	count int
	until time.Time
	last  time.Time
}

// Limiter 按任意键（IP、用户名）限制登录与配对的暴力尝试，仅存内存：重启 serve 即清零。
type Limiter struct {
	mu   sync.Mutex
	now  func() time.Time
	keys map[string]*failure
}

func NewLimiter() *Limiter { return &Limiter{now: time.Now, keys: make(map[string]*failure)} }

// Wait 返回这些键中最长的剩余锁定时间；0 表示可以尝试。
func (l *Limiter) Wait(keys ...string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	var wait time.Duration
	for _, k := range keys {
		if f := l.keys[k]; f != nil && f.until.After(now) {
			wait = max(wait, f.until.Sub(now))
		}
	}
	return wait
}

func (l *Limiter) Fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.keys) > limiterCap {
		for k, f := range l.keys {
			if now.Sub(f.last) > maxLock && !f.until.After(now) {
				delete(l.keys, k)
			}
		}
	}
	for _, k := range keys {
		f := l.keys[k]
		if f == nil {
			f = &failure{}
			l.keys[k] = f
		}
		f.count++
		f.last = now
		if f.count >= freeFailures {
			lock := baseLock << min(f.count-freeFailures, 10)
			f.until = now.Add(min(lock, maxLock))
		}
	}
}

func (l *Limiter) Succeed(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.keys, k)
	}
}

// LoginLimiter là contract chống brute-force cho login/pairing/password
// changes; file mode dùng Limiter trong RAM, backend ngoài (postgres) cung
// cấp implementation dùng chung qua LoginLimiterProvider để giới hạn sống sót
// restart và chia sẻ giữa các replica.
type LoginLimiter interface {
	Wait(keys ...string) time.Duration
	Fail(keys ...string)
	Succeed(keys ...string)
}

// RecoveryLimiter là contract limiter cửa sổ trượt (recovery codes, setup…).
type RecoveryLimiter interface {
	Wait(key string) time.Duration
	Fail(key string)
	Succeed(key string)
}

// LoginLimiterProvider được backend ngoài implement để cấp limiter theo
// backend (bảng login_rate_limits trên postgres). File backend không có
// provider → Store rơi về limiter trong RAM như hiện tại.
type LoginLimiterProvider interface {
	LoginLimiter() LoginLimiter
	RecoveryLimiter(maxFails int, window time.Duration) RecoveryLimiter
}

// LoginLimiter trả limiter login của backend đang chọn. Postgres: buckets DB
// chia sẻ; file: limiter trong RAM giữ nguyên (CLI offline không cần hơn).
func (s *Store) LoginLimiter() LoginLimiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loginLimiter == nil {
		if p, ok := s.repo.(LoginLimiterProvider); ok {
			s.loginLimiter = p.LoginLimiter()
		} else {
			s.loginLimiter = NewLimiter()
		}
	}
	return s.loginLimiter
}

// RecoveryLimiter trả limiter cửa sổ cho recovery codes với chính policy của
// caller (maxFails/window sống ở web package, backend chỉ cấp storage).
func (s *Store) RecoveryLimiter(maxFails int, window time.Duration) RecoveryLimiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recoveryLimiter == nil {
		if p, ok := s.repo.(LoginLimiterProvider); ok {
			s.recoveryLimiter = p.RecoveryLimiter(maxFails, window)
		} else {
			s.recoveryLimiter = NewWindowLimiter(maxFails, window)
		}
	}
	return s.recoveryLimiter
}
