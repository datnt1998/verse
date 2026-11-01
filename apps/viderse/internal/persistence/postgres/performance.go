package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/performance"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// PerformanceStore lưu facts, số liệu sau đăng và nhận xét đã áp dụng (§21).
type PerformanceStore struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

var _ performance.Store = (*PerformanceStore)(nil)

func NewPerformanceStore(pool *pgxpool.Pool) *PerformanceStore {
	return &PerformanceStore{pool: pool, q: gen.New(pool)}
}

func (s *PerformanceStore) SeedsWithoutFacts(ctx context.Context, limit int) ([]performance.Seed, error) {
	rows, err := s.q.ListPostedJobsWithoutFacts(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]performance.Seed, len(rows))
	for i, r := range rows {
		var snap struct {
			DurationSec *float64 `json:"durationSec"`
		}
		_ = json.Unmarshal(r.SourceSnapshot, &snap)
		d := 0
		if snap.DurationSec != nil && *snap.DurationSec > 0 {
			d = int(*snap.DurationSec + 0.5)
		}
		out[i] = performance.Seed{JobID: uuidString(r.ID), DurationSec: d}
	}
	return out, nil
}

func (s *PerformanceStore) EnsureFacts(ctx context.Context, jobID, title string, durationSec int) error {
	d := pgtype.Int4{}
	if durationSec > 0 {
		d = pgtype.Int4{Int32: int32(durationSec), Valid: true}
	}
	return s.q.EnsurePostFacts(ctx, gen.EnsurePostFactsParams{JobID: queueUUID(jobID), Title: title, DurationSec: d})
}

func (s *PerformanceStore) SetYoutube(ctx context.Context, jobID, youtubeID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := s.q.WithTx(tx)
	id := queueUUID(jobID)
	changed, err := q.SetPostYoutube(ctx, gen.SetPostYoutubeParams{JobID: id, YoutubeID: pgtype.Text{String: youtubeID, Valid: true}})
	if err != nil {
		return err
	}
	if changed > 0 {
		if err = q.DeletePostStats(ctx, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PerformanceStore) YoutubeID(ctx context.Context, jobID string) (string, error) {
	v, err := s.q.GetPostYoutubeID(ctx, queueUUID(jobID))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return v.String, err
}

func (s *PerformanceStore) ListPosted(ctx context.Context, channelID string) ([]performance.PostedRow, error) {
	rows, err := s.q.ListChannelPosted(ctx, queueUUID(channelID))
	if err != nil {
		return nil, err
	}
	out := make([]performance.PostedRow, len(rows))
	for i, r := range rows {
		row := performance.PostedRow{JobID: uuidString(r.JobID), PostedAt: r.PostedAt.Time, TemplateID: uuidString(r.TemplateID), TemplateName: r.TemplateName, YoutubeID: r.YoutubeID.String, Title: r.Title, DurationSec: int(r.DurationSec.Int32)}
		if r.PublishedAt.Valid {
			t := r.PublishedAt.Time
			row.PublishedAt = &t
		}
		out[i] = row
	}
	return out, nil
}

func (s *PerformanceStore) ListStats(ctx context.Context, channelID string) ([]performance.StatRow, error) {
	rows, err := s.q.ListChannelPostStats(ctx, queueUUID(channelID))
	if err != nil {
		return nil, err
	}
	out := make([]performance.StatRow, len(rows))
	for i, r := range rows {
		out[i] = performance.StatRow{
			JobID: uuidString(r.JobID), Milestone: performance.Milestone(r.Milestone), Source: performance.Source(r.Source),
			Stat: performance.Stat{Views: r.Views, Likes: int8Ptr(r.Likes), Comments: int8Ptr(r.Comments), ReadAt: r.ReadAt.Time},
		}
	}
	return out, nil
}

func (s *PerformanceStore) PostedChannelIDs(ctx context.Context) ([]string, error) {
	ids, err := s.q.ListPostedChannelIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = uuidString(id)
	}
	return out, nil
}

func (s *PerformanceStore) ReadCandidates(ctx context.Context, now time.Time) ([]performance.Candidate, error) {
	rows, err := s.q.ListReadCandidates(ctx, pgtype.Timestamptz{Time: now.UTC(), Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]performance.Candidate, len(rows))
	for i, r := range rows {
		c := performance.Candidate{JobID: uuidString(r.JobID), YoutubeID: r.YoutubeID.String, PostedAt: r.PostedAt.Time, Attempts: int(r.ReadAttempts), Have: map[performance.Milestone]bool{}}
		if r.PublishedAt.Valid {
			t := r.PublishedAt.Time
			c.PublishedAt = &t
		}
		for _, m := range r.Have {
			c.Have[performance.Milestone(m)] = true
		}
		out[i] = c
	}
	return out, nil
}

func (s *PerformanceStore) SetPublished(ctx context.Context, jobID string, at time.Time) error {
	return s.q.SetPostPublished(ctx, gen.SetPostPublishedParams{JobID: queueUUID(jobID), PublishedAt: pgtype.Timestamptz{Time: at.UTC(), Valid: true}})
}

func (s *PerformanceStore) SaveStat(ctx context.Context, jobID string, milestone performance.Milestone, r performance.Reading, at time.Time) error {
	return s.q.InsertPostStat(ctx, gen.InsertPostStatParams{
		JobID: queueUUID(jobID), Milestone: string(milestone), Views: r.Views, Likes: int8Of(r.Likes), Comments: int8Of(r.Comments),
		Source: string(r.Source), ReadAt: pgtype.Timestamptz{Time: at.UTC(), Valid: true},
	})
}

func (s *PerformanceStore) ReadFailed(ctx context.Context, jobID string, next time.Time) error {
	return s.q.RecordPostReadFailure(ctx, gen.RecordPostReadFailureParams{JobID: queueUUID(jobID), NextReadAt: pgtype.Timestamptz{Time: next.UTC(), Valid: true}})
}

func (s *PerformanceStore) ReadOK(ctx context.Context, jobID string) error {
	return s.q.ClearPostReadFailures(ctx, queueUUID(jobID))
}

func (s *PerformanceStore) ListApplied(ctx context.Context, channelID string) (map[string]performance.Applied, error) {
	rows, err := s.q.ListAppliedInsights(ctx, queueUUID(channelID))
	if err != nil {
		return nil, err
	}
	out := make(map[string]performance.Applied, len(rows))
	for _, r := range rows {
		out[r.InsightID] = performance.Applied{Kind: r.Kind, RuleID: r.RuleID.String}
	}
	return out, nil
}

func (s *PerformanceStore) MarkApplied(ctx context.Context, channelID, insightID string, applied performance.Applied) (bool, error) {
	rule := pgtype.Text{}
	if applied.RuleID != "" {
		rule = pgtype.Text{String: applied.RuleID, Valid: true}
	}
	n, err := s.q.InsertAppliedInsight(ctx, gen.InsertAppliedInsightParams{ChannelID: queueUUID(channelID), InsightID: insightID, Kind: applied.Kind, RuleID: rule})
	return n > 0, err
}

func (s *PerformanceStore) SetTemplateWeight(ctx context.Context, channelID, templateID string, weight int) error {
	n, err := s.q.SetTemplateWeight(ctx, gen.SetTemplateWeightParams{OwnerChannelID: queueUUID(channelID), TemplateID: queueUUID(templateID), Weight: int16(weight)})
	if err != nil {
		return err
	}
	if n == 0 {
		return performance.ErrInsightGone
	}
	return nil
}
