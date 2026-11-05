package postgres

import (
	"context"
	"errors"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/posted"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostedStore giữ metadata mark-posted (§21): facts chụp lúc bắt đầu theo dõi và
// link YouTube của video đã đăng. V38 đã bỏ bộ đọc số liệu sau đăng nên store
// chỉ còn ba thao tác này.
type PostedStore struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

var _ posted.Store = (*PostedStore)(nil)

func NewPostedStore(pool *pgxpool.Pool) *PostedStore {
	return &PostedStore{pool: pool, q: gen.New(pool)}
}

func (s *PostedStore) EnsureFacts(ctx context.Context, jobID, title string, durationSec int) error {
	d := pgtype.Int4{}
	if durationSec > 0 {
		d = pgtype.Int4{Int32: int32(durationSec), Valid: true}
	}
	return s.q.EnsurePostFacts(ctx, gen.EnsurePostFactsParams{JobID: queueUUID(jobID), Title: title, DurationSec: d})
}

func (s *PostedStore) SetYoutube(ctx context.Context, jobID, youtubeID string) error {
	_, err := s.q.SetPostYoutube(ctx, gen.SetPostYoutubeParams{JobID: queueUUID(jobID), YoutubeID: pgtype.Text{String: youtubeID, Valid: true}})
	return err
}

func (s *PostedStore) YoutubeID(ctx context.Context, jobID string) (string, error) {
	v, err := s.q.GetPostYoutubeID(ctx, queueUUID(jobID))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return v.String, err
}
