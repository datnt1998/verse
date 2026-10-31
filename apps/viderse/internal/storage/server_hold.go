package storage

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// SetServerStorageOffline owns a separate persisted source for HoldOffline.
// Recovery cannot clear a disk hold, HoldMoving, or a user's manual pause.
func (s *Service) SetServerStorageOffline(ctx context.Context, offline bool) error {
	if err := s.deps.Settings.Set(ctx, domain.KeyStorageServerOffline, offline); err != nil {
		return err
	}
	s.mu.Lock()
	changed := s.serverOffline != offline
	s.serverOffline = offline
	s.mu.Unlock()
	if changed {
		s.emit("")
		s.bump()
	}
	return nil
}

// SetRemoteStorage fences legacy disk watchers before their first scan.
func (s *Service) SetRemoteStorage(remote bool) { s.mu.Lock(); s.remoteStorage = remote; s.mu.Unlock() }
func (s *Service) RemoteStorage() bool          { s.mu.Lock(); defer s.mu.Unlock(); return s.remoteStorage }
