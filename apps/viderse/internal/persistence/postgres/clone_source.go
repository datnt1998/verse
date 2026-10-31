package postgres

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
)

// SourceRival resolves only an actual tracked YouTube URL, never an untrusted source ID.
func (s *CloneStore) SourceRival(ctx context.Context, url string) (*domain.CloneSourceSnapshot, error) {
	id, reason := queue.ParseVideoURL(url)
	if reason != "" {
		return nil, nil
	}
	rows, err := s.q(ctx).ListVideosByIDs(ctx, []string{id})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	return &domain.CloneSourceSnapshot{Outlier: float8Ptr(row.Score), ViewCount: int8Ptr(row.ViewCount), PublishedAt: timePtr(row.PublishedAt), HasCaptions: boolPtr(row.HasCaptions)}, nil
}
