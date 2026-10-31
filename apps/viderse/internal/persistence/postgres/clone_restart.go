package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// PreviousCloneJobID resolves the immediately preceding retained generation.
func (s *CloneStore) PreviousCloneJobID(ctx context.Context, sourceKey string, generation int) (*string, error) {
	if generation <= 1 {
		return nil, nil
	}
	var id string
	err := mediaDB(ctx, s.pool).QueryRow(ctx, `SELECT id::text FROM clone_jobs WHERE source_key=$1 AND generation=$2 ORDER BY created_at DESC LIMIT 1`, sourceKey, generation-1).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}
