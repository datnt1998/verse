package postgres

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/jackc/pgx/v5"
)

// RecordArtifacts is called only after checking the running attempt fence,
// inside media.Publication. It keeps checkpointed outputs live across restart.
func (s *CloneStore) RecordArtifacts(ctx context.Context, id string, step domain.CloneStep, artifacts []domain.CloneArtifact) error {
	if _, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); !ok {
		return ErrFenceLost
	}
	jobID, err := cloneUUIDOf(id)
	if err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := s.insertArtifact(ctx, jobID, step, artifact); err != nil {
			return err
		}
	}
	return nil
}
