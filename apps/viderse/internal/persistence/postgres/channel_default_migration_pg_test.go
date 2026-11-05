package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

func provisionLegacyChannelSchema(t *testing.T) (*pgxpool.Pool, *goose.Provider) {
	t.Helper()
	_, dsn := pgtest.ProvisionRoles(t, roleSpec)
	db, err := pgmigrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(context.Background(), 38); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, provider
}

func execChannelSQL(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func legacyChannel(t *testing.T, pool *pgxpool.Pool, id, raw string, auto bool, pace int) {
	t.Helper()
	execChannelSQL(t, pool, `INSERT INTO owner_channels(id,name,auto_pick,pace_per_day,autonomy,template_cursor,priority) VALUES($1,'Legacy',$2,$3,$4::jsonb,7,'views')`, id, auto, pace, raw)
	execChannelSQL(t, pool, `INSERT INTO owner_channel_templates(owner_channel_id,template_id,position,weight) VALUES($1,$2,0,1)`, id, tplA)
}

func TestChannelUpgradeEffectiveAutomationTruthTable(t *testing.T) {
	pool, provider := provisionLegacyChannelSchema(t)
	seedChannelTemplate(t, pool, tplA, "First")
	type expected struct {
		id             string
		fetch, approve bool
	}
	var rows []expected
	add := func(raw string, auto, fetch, approve bool) {
		id := fmt.Sprintf("%08x-0000-4000-8000-000000000001", len(rows)+1)
		legacyChannel(t, pool, id, raw, auto, 5)
		rows = append(rows, expected{id, fetch, approve})
	}
	// Every valid approve/pick/old-toggle combination; unknown keys/preset
	// are permitted by the old decoder and must not silently disable it.
	for _, approve := range []string{"ask", "score", "all"} {
		for _, pick := range []string{"suggest", "score2", "all"} {
			for _, auto := range []bool{false, true} {
				raw := fmt.Sprintf(`{"preset":"untrusted","pick":%q,"approve":{"mode":%q,"min":70},"onError":"stop","belowQuality":"fix_then_ask","extra":true}`, pick, approve)
				add(raw, auto, auto && pick != "suggest", approve != "ask")
			}
		}
	}
	valid := `{"pick":"suggest","approve":{"mode":"all","min":85},"onError":"retry_once","belowQuality":"ask"}`
	var decoded map[string]any
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil {
		t.Fatal(err)
	}
	invalid := []string{`null`, `{}`, `[]`, `"malformed legacy object"`, `{"pick":"suggest"}`,
		`{"pick":"suggest","approve":[],"onError":"stop","belowQuality":"ask"}`,
		`{"pick":1,"approve":{"mode":"score","min":85},"onError":"stop","belowQuality":"ask"}`,
		`{"pick":"all","approve":{"mode":"score","min":85},"onError":"unknown","belowQuality":"ask"}`,
		`{"pick":"all","approve":{"mode":"score","min":85},"onError":"stop","belowQuality":false}`,
		`{"pick":"all","approve":{"mode":"unknown","min":85},"onError":"stop","belowQuality":"ask"}`}
	for _, min := range []string{`85.0`, `"85"`, `85.5`, `null`, `69`, `100`} {
		invalid = append(invalid, fmt.Sprintf(`{"pick":"suggest","approve":{"mode":"all","min":%s},"onError":"retry_once","belowQuality":"ask"}`, min))
	}
	for _, missing := range []string{"pick", "approve", "onError", "belowQuality"} {
		m := map[string]any{}
		for k, v := range decoded {
			if k != missing {
				m[k] = v
			}
		}
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		invalid = append(invalid, string(raw))
	}
	for _, raw := range invalid {
		for _, auto := range []bool{false, true} {
			add(raw, auto, auto, false)
		}
	}
	if _, err := provider.UpTo(context.Background(), 40); err != nil {
		t.Fatal(err)
	}
	for _, want := range rows {
		var fetch, approve bool
		var template string
		var perDay int
		if err := pool.QueryRow(context.Background(), `SELECT auto_fetch,auto_approve,per_day,default_template_id::text FROM owner_channels WHERE id=$1`, want.id).Scan(&fetch, &approve, &perDay, &template); err != nil {
			t.Fatal(err)
		}
		if fetch != want.fetch || approve != want.approve || perDay != 5 || template != tplA {
			t.Fatalf("channel %s: fetch=%v approve=%v perDay=%d template=%s; want %+v", want.id, fetch, approve, perDay, template, want)
		}
	}
}

func TestChannelUpgradePreservesSnapshotsAndUsesPositionNotCursorOrWeight(t *testing.T) {
	pool, provider := provisionLegacyChannelSchema(t)
	seedChannelTemplate(t, pool, tplA, "A")
	seedChannelTemplate(t, pool, tplB, "B")
	for _, id := range []string{chOne, chTwo} {
		legacyChannel(t, pool, id, `{}`, false, 20)
		// Duplicate first positions break by template UUID, not insertion order,
		// cursor 7, or the larger weight on B.
		execChannelSQL(t, pool, `INSERT INTO owner_channel_templates(owner_channel_id,template_id,position,weight) VALUES($1,$2,0,2)`, id, tplB)
	}
	const oldJob = "33333333-3333-4333-8333-333333333333"
	const queued = "44444444-4444-4444-8444-444444444444"
	execChannelSQL(t, pool, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,template_id,template_snapshot,model_refs,status,auto_fix) VALUES($1,$2,'old','{}',$3,'{"revision":9,"unchanged":[1,2]}','{"artist":"pinned"}','running','{"at":"2026-01-01T00:00:00Z","result":"retrying","generation":1}')`, oldJob, chOne, tplB)
	execChannelSQL(t, pool, `INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,position,template_id) VALUES($1,$2,'queued','queued',3,$3)`, queued, chOne, tplB)
	var beforeJob, beforeQueue string
	if err := pool.QueryRow(context.Background(), `SELECT (to_jsonb(j)-'instructions')::text FROM clone_jobs j WHERE id=$1`, oldJob).Scan(&beforeJob); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT to_jsonb(q)::text FROM clone_queue q WHERE id=$1`, queued).Scan(&beforeQueue); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(context.Background(), 40); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{chOne, chTwo} {
		var def string
		if err := pool.QueryRow(context.Background(), `SELECT default_template_id::text FROM owner_channels WHERE id=$1`, id).Scan(&def); err != nil || def != tplA {
			t.Fatalf("default=%s err=%v", def, err)
		}
	}
	var afterJob, afterQueue string
	if err := pool.QueryRow(context.Background(), `SELECT (to_jsonb(j)-'instructions')::text FROM clone_jobs j WHERE id=$1`, oldJob).Scan(&afterJob); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT to_jsonb(q)::text FROM clone_queue q WHERE id=$1`, queued).Scan(&afterQueue); err != nil {
		t.Fatal(err)
	}
	if beforeJob != afterJob || beforeQueue != afterQueue {
		t.Fatal("cutover mutated existing job or explicit queued override")
	}
	store := postgres.NewLibraryStore(pool)
	refs, err := store.TemplateChannels(context.Background(), tplA)
	if err != nil || len(refs) != 2 {
		t.Fatalf("shared refs=%+v err=%v", refs, err)
	}
	if err := store.DeleteTemplate(context.Background(), tplA); err == nil {
		t.Fatal("default template deletion bypassed RESTRICT")
	}
	// Non-default templates remain in the library, including queued/job refs.
	var exists bool
	if err := pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM templates WHERE id=$1)`, tplB).Scan(&exists); err != nil || !exists {
		t.Fatalf("lost non-default template: %v", err)
	}
}

func channelLegacyState(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var state string
	// Full column/constraint/index definitions and all affected data, not a
	// count-only assertion: a stopped migration must leave the exact old state.
	err := pool.QueryRow(context.Background(), `SELECT jsonb_build_object(
	'columns',(SELECT jsonb_agg(to_jsonb(c) ORDER BY table_name,ordinal_position) FROM information_schema.columns c WHERE table_schema=current_schema()),
	'constraints',(SELECT jsonb_agg(jsonb_build_array(c.relname,n.conname,pg_get_constraintdef(n.oid)) ORDER BY c.relname,n.conname) FROM pg_constraint n JOIN pg_class c ON c.oid=n.conrelid JOIN pg_namespace s ON s.oid=c.relnamespace WHERE s.nspname=current_schema()),
	'indexes',(SELECT jsonb_agg(jsonb_build_array(tablename,indexname,indexdef) ORDER BY indexname) FROM pg_indexes WHERE schemaname=current_schema()),
	'channels',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM owner_channels c),
	'rotation',(SELECT jsonb_agg(to_jsonb(r) ORDER BY owner_channel_id,template_id) FROM owner_channel_templates r),
	'templates',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM templates t),
	'versions',(SELECT jsonb_agg(to_jsonb(v) ORDER BY id) FROM goose_db_version v))::text`).Scan(&state)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestChannelUpgradeStopsAndRollsBackExactLegacyState(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pace     int
		rotation bool
	}{
		{"missing-rotation", 5, false}, {"per-day-21", 21, true}, {"per-day-50", 50, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, provider := provisionLegacyChannelSchema(t)
			seedChannelTemplate(t, pool, tplA, "A")
			legacyChannel(t, pool, chOne, `{}`, true, tc.pace)
			if !tc.rotation {
				execChannelSQL(t, pool, `DELETE FROM owner_channel_templates WHERE owner_channel_id=$1`, chOne)
			}
			before := channelLegacyState(t, pool)
			if _, err := provider.UpTo(context.Background(), 40); err == nil {
				t.Fatal("unsafe legacy data did not stop cutover")
			}
			if after := channelLegacyState(t, pool); before != after {
				t.Fatal("failed cutover left partial schema/data/version changes")
			}
			// Retry after an explicit fixture-only correction proves no hidden
			// migration remnants. Never clamp production data to make it pass.
			if !tc.rotation {
				execChannelSQL(t, pool, `INSERT INTO owner_channel_templates(owner_channel_id,template_id,position) VALUES($1,$2,0)`, chOne, tplA)
			}
			execChannelSQL(t, pool, `UPDATE owner_channels SET pace_per_day=20 WHERE id=$1`, chOne)
			if _, err := provider.UpTo(context.Background(), 40); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChannelDanglingDefaultStopsAfterDDLAndRollsBack(t *testing.T) {
	pool, provider := provisionLegacyChannelSchema(t)
	seedChannelTemplate(t, pool, tplA, "A")
	legacyChannel(t, pool, chOne, `{}`, true, 5)
	// Simulate a corrupt legacy restore with the old FK absent. This reaches
	// the post-backfill validity assertion, after columns have been added.
	execChannelSQL(t, pool, `DO $$ DECLARE fk text; BEGIN SELECT conname INTO fk FROM pg_constraint WHERE conrelid='owner_channel_templates'::regclass AND confrelid='templates'::regclass AND contype='f'; EXECUTE format('ALTER TABLE owner_channel_templates DROP CONSTRAINT %I',fk); END $$`)
	execChannelSQL(t, pool, `UPDATE owner_channel_templates SET template_id=$1 WHERE owner_channel_id=$2`, tplB, chOne)
	before := channelLegacyState(t, pool)
	if _, err := provider.UpTo(context.Background(), 40); err == nil {
		t.Fatal("dangling default was accepted")
	}
	if after := channelLegacyState(t, pool); after != before {
		t.Fatal("post-DDL stop did not roll back exact legacy state")
	}
}
