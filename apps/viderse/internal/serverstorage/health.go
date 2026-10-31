package serverstorage

import (
	"context"
	"os"
	"time"
)

func (s *Service) monitor() {
	for {
		started := time.Now()
		s.checkHealth()
		s.cleanupTests()
		s.mu.Lock()
		online := s.health.Online
		s.mu.Unlock()
		interval := 50 * time.Second
		if !online {
			interval = time.Minute
		}
		timer := time.NewTimer(time.Until(started.Add(interval)))
		select {
		case <-s.ctx.Done():
			timer.Stop()
			s.cleanupAllTests()
			return
		case <-timer.C:
		}
	}
}
func (s *Service) checkHealth() {
	ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
	defer cancel()
	store, d, state, err := s.opts.Media.Active(ctx)
	if err == nil {
		if s.opts.ModeChanged != nil {
			s.opts.ModeChanged(d.Config.Driver == "s3")
		}
		if d.Config.Driver == "s3" {
			_, err = store.List(ctx, "v1/sha256/", "", 1)
		} else {
			var file *os.File
			file, err = os.CreateTemp(d.Config.LocalRoot, ".storage-health-*")
			if err == nil {
				path := file.Name()
				err = file.Close()
				removeErr := os.Remove(path)
				if err == nil {
					err = removeErr
				}
			}
		}
	}
	if s.ctx.Err() != nil {
		return
	}
	s.mu.Lock()
	old := s.health
	next := Health{StoreID: state.Active, Online: err == nil, CheckedAt: time.Now().UTC()}
	if err != nil {
		next.ErrorCode = errorCode(err)
		next.Since = old.Since
		if next.Since == nil || old.StoreID != state.Active {
			since := next.CheckedAt
			next.Since = &since
		}
	}
	s.health = next
	s.mu.Unlock()
	persistence, persistCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer persistCancel()
	s.logFailure("persist_health", s.opts.Repository.SaveHealth(persistence, next))
	if s.opts.Hold != nil {
		s.logFailure("storage_hold", s.opts.Hold(persistence, !next.Online))
	}
	if !next.Online && (old.Online || old.Since == nil || old.StoreID != next.StoreID) && s.opts.NotifyOffline != nil {
		s.logFailure("storage_notification", s.opts.NotifyOffline(persistence, "Không kết nối được kho lưu trữ. Viderse sẽ tự thử lại mỗi phút."))
	}
	if old.Online != next.Online || old.StoreID != next.StoreID {
		s.changed()
	}
}
func (s *Service) cleanupAllTests() {
	s.mu.Lock()
	sessions := s.tests
	s.tests = make(map[string]*testSession)
	s.mu.Unlock()
	for _, test := range sessions {
		test.mu.Lock()
		if test.result.Steps[5].Status == "skip" {
			s.finishTest(test)
		}
		test.mu.Unlock()
	}
}
