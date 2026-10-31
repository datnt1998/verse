package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

func recordImageApproval(ctx context.Context, tx pgx.Tx, id any, before, after json.RawMessage) error {
	var old, next images.Manifest
	hasBefore := json.Unmarshal(before, &old) == nil && old.Version != 0
	if err := json.Unmarshal(after, &next); err != nil {
		return err
	}
	if images.ImagesApproved(old) || !images.ImagesApproved(next) {
		return nil
	}
	query := `UPDATE clone_jobs SET image_approved_at=now() WHERE id=$1`
	if !hasBefore {
		query = `UPDATE clone_jobs SET image_approved_at=COALESCE(image_approved_at,now()) WHERE id=$1`
	}
	_, err := tx.Exec(ctx, query, id)
	return err
}

// ApprovedToday counts videos whose image approval completed during the
// server's current local calendar day. Superseded generations are excluded.
func (s *CloneStore) ApprovedToday(ctx context.Context, channelID string) (int, error) {
	now := time.Now().In(time.Local)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	var count int
	if channelID == "" {
		err := s.pool.QueryRow(ctx, `SELECT count(*) FROM clone_jobs WHERE superseded_at IS NULL AND image_approved_at >= $1 AND image_approved_at < $2`, start, end).Scan(&count)
		return count, err
	}
	channel, err := cloneUUIDOf(channelID)
	if err != nil || !channel.Valid {
		return 0, ErrNotFound
	}
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM clone_jobs WHERE channel_id=$1 AND superseded_at IS NULL AND image_approved_at >= $2 AND image_approved_at < $3`, channel, start, end).Scan(&count)
	return count, err
}
