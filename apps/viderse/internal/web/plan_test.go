package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func planFixture(t *testing.T) (*queue.Service, *pgxpool.Pool) {
	t.Helper()
	app, maintenance := pgtest.ProvisionRoles(t, pgtest.RoleSpec{MigrateRole: "viderse_migrate", AppRole: "viderse_app", LockKey1: 2790, LockKey2: 1, InitScriptRef: "deploy/postgres-init/01-roles.sh"})
	db, err := pgmigrate.OpenDB(maintenance)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = pgmigrate.Up(context.Background(), db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO templates(id,name,recipe) VALUES ('aaaaaaaa-1111-4111-8111-111111111111','Mẫu', '{"scenes":{"images":{"mode":"fixed","count":6},"coverSource":"ai"}}')`,
		`INSERT INTO owner_channels(id,name,per_day,default_template_id) VALUES ('11111111-1111-4111-8111-111111111111','Kênh',1,'aaaaaaaa-1111-4111-8111-111111111111')`,
		`INSERT INTO rival_channels(id,title,url,median_views) VALUES ('UCaaaaaaaaaaaaaaaaaaaaaa','Nguồn','https://youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa',100)`,
		`INSERT INTO owner_channel_sources(owner_channel_id,rival_id) VALUES ('11111111-1111-4111-8111-111111111111','UCaaaaaaaaaaaaaaaaaaaaaa')`,
		`INSERT INTO rival_videos(id,channel_id,title,position,origin,duration_sec,view_count,has_captions) VALUES ('aaaaaaaaaaa','UCaaaaaaaaaaaaaaaaaaaaaa','Video A',1,'discovery',120,200,true),('bbbbbbbbbbb','UCaaaaaaaaaaaaaaaaaaaaaa','Video B',2,'discovery',180,300,false)`,
	} {
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.Local)
	return queue.New(queue.Options{Pool: pool, Store: postgres.NewQueueStore(pool), Settings: postgres.NewSettingsStore(pool), Now: func() time.Time { return now }}), pool
}
func postPlan(t *testing.T, svc *queue.Service, urls []string) domain.ClonePlan {
	t.Helper()
	body, _ := json.Marshal(domain.CloneEnqueueRequest{ChannelID: "11111111-1111-4111-8111-111111111111", URLs: urls})
	r := httptest.NewRequest("POST", "/api/clone/plan", bytes.NewReader(body))
	w := httptest.NewRecorder()
	planClone(w, r, svc, func(e error) { t.Log(e) })
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out domain.ClonePlan
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestPlanEndpointMultipleLinksUsesBatchPace(t *testing.T) {
	svc, _ := planFixture(t)
	p := postPlan(t, svc, []string{"https://youtu.be/aaaaaaaaaaa", "https://www.youtube.com/watch?v=bbbbbbbbbbb"})
	if len(p.Accepted) != 2 || p.Position != "back" || p.PaceLeftToday != 1 || p.OverflowDays != 1 {
		t.Fatalf("plan=%+v", p)
	}
	if p.ImageCalls != 16 || p.PriceUSD != nil {
		t.Fatalf("image calls/price=%d/%v", p.ImageCalls, p.PriceUSD)
	}
	if p.ETAStart == nil || p.Template.Name != "Mẫu" {
		t.Fatalf("missing ETA/template: %+v", p)
	}
	if p.Accepted[0].ViewCount == nil || *p.Accepted[0].ViewCount != 200 || !p.Accepted[0].HasCaptions {
		t.Fatalf("source data=%+v", p.Accepted[0])
	}
}
func TestPlanEndpointReportsDuplicateJobsAndQueuedLinks(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,stage_group,status) VALUES ('22222222-2222-4222-8222-222222222222','11111111-1111-4111-8111-111111111111','channel:11111111-1111-4111-8111-111111111111:video:aaaaaaaaaaa','{"id":"aaaaaaaaaaa","title":"Đã làm A"}','ready','succeeded')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,source_snapshot,position) VALUES ('33333333-3333-4333-8333-333333333333','11111111-1111-4111-8111-111111111111','bbbbbbbbbbb','channel:11111111-1111-4111-8111-111111111111:video:bbbbbbbbbbb','{"id":"bbbbbbbbbbb","title":"Đang chờ B"}',1)`); err != nil {
		t.Fatal(err)
	}
	p := postPlan(t, svc, []string{"https://youtu.be/aaaaaaaaaaa", "https://youtu.be/bbbbbbbbbbb", "https://youtube.com/watch?v=aaaaaaaaaaa"})
	if len(p.Accepted) != 0 || len(p.Duplicates) != 3 || p.Duplicates[0].JobID == nil || p.Duplicates[1].JobID != nil || p.Duplicates[1].State != "queued" || p.Duplicates[0].Title != "Đã làm A" {
		t.Fatalf("duplicates=%+v", p.Duplicates)
	}
}
func TestPlanEndpointDedupesSameVideoWithinNewRequest(t *testing.T) {
	svc, _ := planFixture(t)
	p := postPlan(t, svc, []string{"https://youtu.be/aaaaaaaaaaa", "https://youtube.com/watch?v=aaaaaaaaaaa"})
	if len(p.Accepted) != 1 {
		t.Fatalf("accepted repeated source twice: %+v", p)
	}
}

func TestPlanSourceLinkFlagTracksTargetChannel(t *testing.T) {
	svc, pool := planFixture(t)
	p := postPlan(t, svc, []string{"https://youtu.be/aaaaaaaaaaa"})
	if len(p.Accepted) != 1 || p.Accepted[0].SourceChannelID == nil || *p.Accepted[0].SourceChannelID != "UCaaaaaaaaaaaaaaaaaaaaaa" || !p.Accepted[0].RivalLinked {
		t.Fatalf("linked source missing: %+v", p.Accepted)
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM owner_channel_sources`); err != nil {
		t.Fatal(err)
	}
	p = postPlan(t, svc, []string{"https://youtu.be/aaaaaaaaaaa"})
	if len(p.Accepted) != 1 || p.Accepted[0].RivalLinked || p.Accepted[0].SourceChannelID == nil {
		t.Fatalf("unlinked source misreported: %+v", p.Accepted)
	}
}

func TestPlanDuplicatePreviewUsesSnapshotThenRivalFallback(t *testing.T) {
	svc, pool := planFixture(t)
	_, err := pool.Exec(context.Background(), `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,stage_group,status) VALUES
 ('22222222-2222-4222-8222-222222222222','11111111-1111-4111-8111-111111111111','channel:11111111-1111-4111-8111-111111111111:video:aaaaaaaaaaa','{"id":"aaaaaaaaaaa","title":"Frozen source","thumbnailUrl":"https://snapshot/image.jpg","durationSec":17,"viewCount":42,"hasCaptions":false}','ready','succeeded'),
 ('33333333-3333-4333-8333-333333333333','11111111-1111-4111-8111-111111111111','channel:11111111-1111-4111-8111-111111111111:video:bbbbbbbbbbb','{"id":"bbbbbbbbbbb","title":"Historical source"}','ready','succeeded')`)
	if err != nil {
		t.Fatal(err)
	}
	p := postPlan(t, svc, []string{"https://youtu.be/aaaaaaaaaaa", "https://youtu.be/bbbbbbbbbbb"})
	if len(p.Duplicates) != 2 {
		t.Fatal(p)
	}
	a, b := p.Duplicates[0], p.Duplicates[1]
	if a.ThumbnailURL == nil || *a.ThumbnailURL != "https://snapshot/image.jpg" || a.DurationSec == nil || *a.DurationSec != 17 || a.ViewCount == nil || *a.ViewCount != 42 || a.HasCaptions {
		t.Fatalf("snapshot preview changed: %+v", a)
	}
	if b.ThumbnailURL == nil || b.DurationSec == nil || *b.DurationSec != 180 || b.ViewCount == nil || *b.ViewCount != 300 || b.HasCaptions {
		t.Fatalf("historical preview not hydrated: %+v", b)
	}
}
