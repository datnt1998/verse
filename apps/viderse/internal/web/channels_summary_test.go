package web

import (
 "context"
 "encoding/json"
 "net/http"
 "testing"
 "time"

 "github.com/datnt1998/verse/apps/viderse/internal/channels"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

type fakeSummaryStats struct{}
func (fakeSummaryStats) SummaryStats(context.Context, time.Time, time.Time) (map[string][2]int, map[string][2]string, error) {
 return map[string][2]int{"ch-1": {2,3}}, map[string][2]string{"ch-1": {"t-1", "Cận cảnh"}}, nil
}
func (fakeSummaryStats) SummaryGroups(context.Context) (map[string][2]int, error) { return map[string][2]int{"ch-1": {3,1}}, nil }

func TestChannelSummaryRoute(t *testing.T) {
 svc := &fakeChannelService{list: []channels.ChannelView{sampleChannelView()}}
 tg, cookie := channelFixture(t, svc)
 jobs := []domain.CloneJobView{}
 for _, group := range []string{domain.StageReview, domain.StageAttention, domain.StagePaused, domain.StageReady, domain.StageWorking} {
  jobs = append(jobs, domain.CloneJobView{CloneJob: domain.CloneJob{ChannelID:"ch-1", StageGroup:group}})
 }
 tg.g.Clone = &fakeCloneService{list: domain.CloneJobList{Jobs: jobs}}
 MountChannelSummary(tg.g, svc, fakeSummaryStats{})
 w := tg.do(http.MethodGet, "/api/channels/summary", cookie, "")
 if w.Code != http.StatusOK { t.Fatalf("summary status %d: %s", w.Code, w.Body.String()) }
 var rows []channelSummary
 if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil { t.Fatal(err) }
 if len(rows) != 1 { t.Fatalf("summary: %+v", rows) }
 row := rows[0]
 if row.ChannelID != "ch-1" || row.Today.Cap != 5 || row.Today.Done != 2 || row.WaitingYou != 3 || row.Ready != 1 || row.PostedThisWeek != 3 || !row.AutoFetch || row.TemplateIssue == nil || row.TemplateIssue.TemplateID != "t-1" || row.AvatarURL == nil { t.Fatalf("summary: %+v", row) }
}

func TestChannelSummaryMatchesBoardPG(t *testing.T) {
 _, pool := planFixture(t)
 ctx := context.Background()
 _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name,pace_per_day,auto_pick) VALUES ('22222222-2222-4222-8222-222222222222','Hai',3,false);
 INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,status,stage_group,paused_at) VALUES
 ('33333333-3333-4333-8333-333333333333','11111111-1111-4111-8111-111111111111','one','{}','failed','working',NULL),
 ('44444444-4444-4444-8444-444444444444','22222222-2222-4222-8222-222222222222','two','{}','pending','working',now()),
 ('55555555-5555-4555-8555-555555555555','22222222-2222-4222-8222-222222222222','three','{}','waiting','working',NULL);
 INSERT INTO clone_steps(job_id,step,status,error) VALUES ('55555555-5555-4555-8555-555555555555','images','waiting','{"code":"awaiting_review"}')`)
 if err != nil { t.Fatal(err) }
 views, err := postgres.NewChannelStore(pool).ListChannels(ctx); if err != nil { t.Fatal(err) }
 svc := &fakeChannelService{list: views}
 tg, cookie := channelFixture(t, svc)
 tg.g.Clone = &pipeline.Service{Store:postgres.NewCloneStore(pool)}
 MountClone(tg.g, tg.g.Clone, nil, nil)
 MountChannelSummary(tg.g, svc, postgres.NewChannelSummaryStore(pool))
 summary := tg.do(http.MethodGet, "/api/channels/summary", cookie, "")
 if summary.Code != http.StatusOK { t.Fatalf("summary: %d %s", summary.Code, summary.Body.String()) }
 var rows []channelSummary
 if err := json.Unmarshal(summary.Body.Bytes(), &rows); err != nil { t.Fatal(err) }
 board := tg.do(http.MethodGet, "/api/clone?channel=all", cookie, "")
 if board.Code != http.StatusOK { t.Fatalf("board: %d %s", board.Code, board.Body.String()) }
 var list domain.CloneJobList
 if err := json.Unmarshal(board.Body.Bytes(), &list); err != nil { t.Fatal(err) }
 total, waiting := 0, 0
 for _, row := range rows { total += row.WaitingYou }
 for _, job := range list.Jobs { switch job.StageGroup { case domain.StageReview, domain.StageAttention, domain.StagePaused: waiting++ } }
 if total != 3 || total != waiting { t.Fatalf("summary waiting=%d board waiting=%d", total, waiting) }
}
