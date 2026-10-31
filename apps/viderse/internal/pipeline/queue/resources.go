package queue

import (
	"context"
	"errors"
	"sync"
)

// ResourceSlots bounds expensive steps inside the process holding the worker
// advisory lock. Unlike the multi-process TS implementation, no expiring DB
// lease is needed: process exit releases both ownership and every goroutine.
// Limits apply at the next acquisition; reducing one never interrupts work.
type ResourceSlots struct {
	mu      sync.Mutex
	used    map[string]int
	changed chan struct{}
}

func NewResourceSlots() *ResourceSlots {
	return &ResourceSlots{used: map[string]int{}, changed: make(chan struct{})}
}
func (s *ResourceSlots) Acquire(ctx context.Context, resource string, limit int) (func(), error) {
	if limit < 1 {
		return nil, errors.New("giới hạn tài nguyên phải lớn hơn 0")
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s.mu.Lock()
		if s.used[resource] < limit {
			s.used[resource]++
			s.mu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					s.mu.Lock()
					s.used[resource]--
					close(s.changed)
					s.changed = make(chan struct{})
					s.mu.Unlock()
				})
			}, nil
		}
		changed := s.changed
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}
func (s *ResourceSlots) With(ctx context.Context, resource string, limit int, run func(context.Context) error) error {
	release, err := s.Acquire(ctx, resource, limit)
	if err != nil {
		return err
	}
	defer release()
	return run(ctx)
}
