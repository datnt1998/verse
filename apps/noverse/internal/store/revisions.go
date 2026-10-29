package store

import (
	"os"

	"github.com/datnt1998/noverse/internal/domain"
)

const pendingRevisionPath = "meta/pending_revision.json"

type fileRevisionStore struct{ io *IO }

func newFileRevisionStore(io *IO) *fileRevisionStore { return &fileRevisionStore{io: io} }

func (s *fileRevisionStore) LoadPending() (*domain.PendingRevision, error) {
	var pending domain.PendingRevision
	if err := s.io.ReadJSON(pendingRevisionPath, &pending); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &pending, nil
}

func (s *fileRevisionStore) SavePending(pending domain.PendingRevision) error {
	return s.io.WriteJSON(pendingRevisionPath, pending)
}

func (s *fileRevisionStore) ClearPending() error {
	return s.io.RemoveFile(pendingRevisionPath)
}
