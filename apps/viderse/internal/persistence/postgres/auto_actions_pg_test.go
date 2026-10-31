package postgres_test

import (
	"context"
	"encoding/json"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"testing"
	"time"
)

func TestRecapIncludesDurableAutoActions(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	var job string
	err := pool.QueryRow(ctx, `WITH c AS (INSERT INTO owner_channels(id,name) VALUES(gen_random_uuid(),'test') RETURNING id) INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot) SELECT gen_random_uuid(),id,'source','{}'::jsonb FROM c RETURNING id::text`).Scan(&job)
	if err != nil {
		t.Fatal(err)
	}
	reason, _ := json.Marshal(insight.Reason{Agent: "artist", Text: "Lý do", Facts: []insight.Fact{{Label: "Ảnh", Value: "scene-1", Source: "Bước Ảnh"}}})
	_, err = pool.Exec(ctx, `INSERT INTO clone_auto_actions(job_id,kind,text,reason) VALUES($1,'safety_redraw','Tự vẽ lại',$2::jsonb)`, job, reason)
	if err != nil {
		t.Fatal(err)
	}
	recap, err := postgres.NewActivityStore(pool).ReadRecap(ctx, time.Now().Add(-time.Hour), time.Now().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(recap.Auto) != 1 || recap.Auto[0].Kind != "safety_redraw" || recap.Auto[0].Reason == nil || recap.Auto[0].Reason.Agent != "artist" || !recap.HasNew {
		t.Fatalf("recap: %+v", recap)
	}
}

func TestSafetyRedrawCheckpointRecordsOnce(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	var job string
	err := pool.QueryRow(ctx, `WITH c AS (INSERT INTO owner_channels(id,name) VALUES(gen_random_uuid(),'test') RETURNING id) INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot) SELECT gen_random_uuid(),id,'source','{}'::jsonb FROM c RETURNING id::text`).Scan(&job)
	if err != nil {
		t.Fatal(err)
	}
	old := json.RawMessage(`{"items":[{"id":"scene-1","status":"running","inFlight":{"generationId":"g1"}}]}`)
	next := json.RawMessage(`{"items":[{"id":"scene-1","status":"running","modest":true,"inFlight":{"generationId":"g1"}}]}`)
	_, err = pool.Exec(ctx, `INSERT INTO clone_steps(job_id,step,status,attempt,fence,checkpoint) VALUES($1,'images','running',1,1,$2::jsonb)`, job, old)
	if err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	for range 2 {
		if err = store.WriteStepCheckpoint(ctx, job, domain.CloneImages, 1, 1, next); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clone_auto_actions WHERE job_id=$1 AND kind='safety_redraw'`, job).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("redraw records=%d", count)
	}
}
