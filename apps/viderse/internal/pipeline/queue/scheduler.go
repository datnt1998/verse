package queue

import (
	"context"
	"errors"
	"time"
)

const DrainTick = time.Minute
const DiscoveryTick = 15 * time.Minute

// RequestDrain coalesces wakeups, but does not delay a newly enqueued pick:
// L1 requires a free worker to start it within the same second.
func (s *Service) RequestDrain() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return errors.New("bộ xếp hàng đã chạy")
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(DrainTick)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-s.wake:
			case <-ticker.C:
			}
			_, err := s.Drain(s.ctx)
			s.diag(err)
		}
	}()
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(DiscoveryTick)
		defer ticker.Stop()
		s.diag(s.Discover(s.ctx))
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.diag(s.Discover(s.ctx))
			}
		}
	}()
	s.RequestDrain()
	return nil
}
func (s *Service) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}
	return nil
}
