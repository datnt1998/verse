package web

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/performance"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

const (
	perfChannel  = "11111111-1111-4111-8111-111111111111"
	perfTemplate = "aaaaaaaa-1111-4111-8111-111111111111"
	perfTemplB   = "bbbbbbbb-1111-4111-8111-111111111111"
)

type perfFakeRules struct{ got []performance.RuleInput }

func (f *perfFakeRules) AddRule(_ context.Context, in performance.RuleInput) (string, error) {
	f.got = append(f.got, in)
	return "rule-1", nil
}

func TestPerformanceStorePostgres(t *testing.T) {
	_, pool := planFixture(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`INSERT INTO templates(id,name,recipe) VALUES ($1,'Mẫu B','{}')`, perfTemplB)
	exec(`INSERT INTO owner_channel_templates(owner_channel_id,template_id,position) VALUES ($1,$2,1)`, perfChannel, perfTemplB)
	type vid struct {
		title, tpl string
		dur        int
		hour       int
		views      int64
	}
	videos := []vid{
		{"Tại sao A?", perfTemplate, 150, 9, 3000}, {"Vì sao B?", perfTemplate, 160, 9, 2800}, {"Làm sao C?", perfTemplate, 170, 10, 3200}, {"Kể A", perfTemplate, 140, 9, 2900},
		{"Chuyện A", perfTemplB, 600, 20, 1500}, {"Chuyện B", perfTemplB, 620, 20, 1400}, {"Chuyện C", perfTemplB, 640, 21, 1600}, {"Kể B", perfTemplB, 650, 21, 1300}, {"Kể C", perfTemplB, 610, 20, 1450},
	}
	base := time.Now().UTC().Add(-30 * 24 * time.Hour)
	store := postgres.NewPerformanceStore(pool)
	for i, v := range videos {
		id := fmt.Sprintf("33333333-3333-4333-8333-%012d", i)
		postedAt := base.Add(time.Duration(i) * time.Hour)
		exec(`INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,template_id,status,stage_group,posted_at) VALUES ($1,$2,$3,$4::jsonb,$5,'succeeded','posted',$6)`,
			id, perfChannel, fmt.Sprintf("channel:%s:video:%d", perfChannel, i), fmt.Sprintf(`{"id":"v%d","durationSec":%d}`, i, v.dur), v.tpl, postedAt)
		if err := store.EnsureFacts(ctx, id, v.title, v.dur); err != nil {
			t.Fatal(err)
		}
		published := time.Date(2026, 10, 1, v.hour-7, 0, 0, 0, time.UTC) // v.hour theo giờ VN
		if err := store.SetPublished(ctx, id, published); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveStat(ctx, id, performance.M7d, performance.Reading{Views: v.views, Source: performance.SourcePublicPage}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	rules := &perfFakeRules{}
	svc := &performance.Service{Store: store, Rules: rules, Loc: time.FixedZone("ICT", 7*3600)}
	view, err := svc.Performance(ctx, perfChannel)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Posted) != 9 || view.Counted != 9 || view.NeedMore != 0 || len(view.Insights) == 0 || len(view.Insights) > performance.MaxInsights {
		t.Fatalf("view: posted=%d counted=%d needMore=%d insights=%d", len(view.Posted), view.Counted, view.NeedMore, len(view.Insights))
	}
	for _, in := range view.Insights {
		if !in.Preliminary {
			t.Errorf("%s not preliminary", in.ID)
		}
	}

	// áp dụng nhận xét tiêu đề: quy tắc có nguồn số, áp lần hai → 409
	titleID := "title_pattern:question"
	w := httptest.NewRecorder()
	applyInsight(w, httptest.NewRequest("POST", "/api/channels/"+perfChannel+"/insights/x/apply", nil), svc, perfChannel, titleID)
	if w.Code != 200 || len(rules.got) != 1 || rules.got[0].Source != "Từ số liệu 9 video" {
		t.Fatalf("apply status=%d body=%s rules=%+v", w.Code, w.Body.String(), rules.got)
	}
	w = httptest.NewRecorder()
	applyInsight(w, httptest.NewRequest("POST", "/", nil), svc, perfChannel, titleID)
	if w.Code != 409 || len(rules.got) != 1 {
		t.Fatalf("second apply status=%d rules=%d", w.Code, len(rules.got))
	}

	// Template luân phiên: Đều đặn không đổi; áp dụng nhận xét mẫu → mẫu A chạy gấp đôi
	queueStore := postgres.NewQueueStore(pool)
	owner, err := queueStore.Owner(ctx, perfChannel)
	if err != nil || owner == nil {
		t.Fatalf("owner %v %v", owner, err)
	}
	seq, err := queueStore.RotationTemplates(ctx, *owner)
	if err != nil || len(seq) != 2 {
		t.Fatalf("steady rotation = %d %v", len(seq), err)
	}
	res, err := svc.Apply(ctx, perfChannel, "template:"+perfTemplate)
	if err != nil || res.Kind != "rotation" {
		t.Fatalf("apply template: %+v %v", res, err)
	}
	seq, _ = queueStore.RotationTemplates(ctx, *owner)
	if len(seq) != 3 || seq[0].ID != perfTemplate || seq[2].ID != perfTemplate {
		t.Fatalf("weighted rotation = %+v", seq)
	}

	// Ưu tiên Lượt xem: mẫu B (trung vị thấp) không được thêm lượt, mẫu A vẫn gấp đôi, không bao giờ quá 2
	owner.Priority = domain.PriorityViews
	seq, _ = queueStore.RotationTemplates(ctx, *owner)
	counts := map[string]int{}
	for _, tpl := range seq {
		counts[tpl.ID]++
	}
	if counts[perfTemplate] != 2 || counts[perfTemplB] != 1 {
		t.Fatalf("views rotation = %v", counts)
	}

	// Tự lấy: video nổi trội nhất lên trước
	exec(`UPDATE rival_videos SET score = 8.5 WHERE id = 'bbbbbbbbbbb'`)
	exec(`UPDATE rival_videos SET score = 2.0 WHERE id = 'aaaaaaaaaaa'`)
	scores, err := queueStore.RivalVideoScores(ctx, []string{"aaaaaaaaaaa", "bbbbbbbbbbb"})
	if err != nil {
		t.Fatal(err)
	}
	if got := performance.OrderByScore([]string{"aaaaaaaaaaa", "bbbbbbbbbbb"}, scores); got[0] != "bbbbbbbbbbb" {
		t.Fatalf("views autopick order = %v scores=%v", got, scores)
	}

	// Đổi link YouTube xoá số của link cũ
	first := fmt.Sprintf("33333333-3333-4333-8333-%012d", 0)
	if err = store.SetYoutube(ctx, first, "abcdefghijk"); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveStat(ctx, first, performance.M24h, performance.Reading{Views: 5, Source: performance.SourcePublicPage}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = store.SetYoutube(ctx, first, "abcdefghijk"); err != nil {
		t.Fatal(err)
	}
	stats, _ := store.ListStats(ctx, perfChannel)
	has24 := false
	for _, s := range stats {
		has24 = has24 || (s.JobID == first && s.Milestone == performance.M24h)
	}
	if !has24 {
		t.Fatal("same link must keep stats")
	}
	if err = store.SetYoutube(ctx, first, "zzzzzzzzzzz"); err != nil {
		t.Fatal(err)
	}
	stats, _ = store.ListStats(ctx, perfChannel)
	for _, s := range stats {
		if s.JobID == first {
			t.Fatalf("new link kept old stats: %+v", s)
		}
	}
}
