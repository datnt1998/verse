package postgres

import (
 "context"
 "time"

 "math"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
 "github.com/jackc/pgx/v5/pgtype"
 "github.com/jackc/pgx/v5/pgxpool"
)

// ChannelSummaryStore reads the cadence and template health inputs for the board.
type ChannelSummaryStore struct { q *gen.Queries; clones *CloneStore }
func NewChannelSummaryStore(pool *pgxpool.Pool) *ChannelSummaryStore { return &ChannelSummaryStore{q: gen.New(pool), clones: NewCloneStore(pool)} }
func (s *ChannelSummaryStore) SummaryStats(ctx context.Context, day, week time.Time) (map[string][2]int, map[string][2]string, error) {
 rows, err := s.q.ChannelSummaryStats(ctx, gen.ChannelSummaryStatsParams{Column1: pgtype.Timestamptz{Time:day, Valid:true}, Column2: pgtype.Timestamptz{Time:week, Valid:true}})
 if err != nil { return nil, nil, err }
 stats := make(map[string][2]int, len(rows))
 for _, r := range rows { stats[rowUUID(r.ID)] = [2]int{int(r.PaceUsed), int(r.PostedWeek)} }
 issues, err := s.q.ChannelSummaryTemplateIssues(ctx)
 if err != nil { return nil, nil, err }
 byChannel := make(map[string][2]string)
 for _, r := range issues { id := rowUUID(r.OwnerChannelID); if _, exists := byChannel[id]; !exists { byChannel[id] = [2]string{rowUUID(r.ID), r.Name} } }
 return stats, byChannel, nil
}

// SummaryGroups uses the same classification as the clone board, not the
// denormalized stage_group column, which may lag a transition.
func (s *ChannelSummaryStore) SummaryGroups(ctx context.Context) (map[string][2]int, error) {
 jobs, err := s.clones.ListBoardJobs(ctx, nil, math.MaxInt32)
 if err != nil { return nil, err }
 ids := make([]string, len(jobs))
 for i := range jobs { ids[i] = jobs[i].ID }
 steps, err := s.clones.ListJobSteps(ctx, ids)
 if err != nil { return nil, err }
 counts := make(map[string][2]int)
 for _, job := range jobs {
  c := counts[job.ChannelID]
  switch domain.StageGroupOf(job, steps[job.ID]) {
  case domain.StageReview, domain.StageAttention, domain.StagePaused: c[0]++
  case domain.StageReady: c[1]++
  }
  counts[job.ChannelID] = c
 }
 return counts, nil
}
