package postgres

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ListJobSteps uses one query for the whole board, avoiding N+1 round trips.
func (s *CloneStore) ListJobSteps(ctx context.Context, ids []string) (map[string][]domain.CloneStepRecord, error) {
	uuids := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := cloneUUIDOf(id)
		if err != nil {
			return nil, err
		}
		uuids = append(uuids, u)
	}
	rows, err := s.q(ctx).ListCloneStepsByJobs(ctx, uuids)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]domain.CloneStepRecord, len(ids))
	for _, row := range rows {
		rec := stepFromGen(row)
		out[rec.JobID] = append(out[rec.JobID], rec)
	}
	return out, nil
}

// GuardSourceCleanup holds the row lock until Publication commits. Resume,
// edit and supersede cannot race deletion of source references.
func (s *CloneStore) GuardSourceCleanup(ctx context.Context, id string, version int64) error {
	tx, ok := ctx.Value(mediaTxKey{}).(pgx.Tx)
	if !ok {
		return ErrFenceLost
	}
	u, err := cloneUUIDOf(id)
	if err != nil {
		return ErrNotFound
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT status='succeeded' AND superseded_at IS NULL AND version=$2 AND (lease_expires_at IS NULL OR lease_expires_at<=now()) FROM clone_jobs WHERE id=$1 FOR UPDATE`, u, version).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrFenceLost
	}
	return nil
}
