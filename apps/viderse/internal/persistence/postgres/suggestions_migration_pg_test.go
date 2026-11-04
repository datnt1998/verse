package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

func suggestionsCutoverSQL(t *testing.T) string {
	t.Helper()
	contents, err := migrations.FS.ReadFile("00038_global_suggestions.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, found := strings.Cut(string(contents), "-- +goose Down")
	if !found {
		t.Fatal("migration must separate the production cutover from scratch rollback")
	}
	return up
}

func provisionSuggestionsBeforeCutover(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_, maintenanceDSN := pgtest.ProvisionRoles(t, roleSpec)
	ctx := context.Background()
	db, err := pgmigrate.OpenDB(maintenanceDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 37); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(ctx, maintenanceDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestSuggestionsCutoverPreservesPreferencesAndClosesOnlyRetiredWork(t *testing.T) {
	ctx := context.Background()
	pool := provisionSuggestionsBeforeCutover(t)
	prefs := `{"badge":false,"merge":true,"recapOnReturn":false,"suggestions":{"off":[{"kind":"hot_video"},{"kind":"template_failing","channelId":null},{"kind":"cleanup"},{"kind":"learn_rule"}]},"retained":{"channel":"email"}}`
	if _, err := pool.Exec(ctx, `INSERT INTO settings(key,value) VALUES('notifications.preferences',$1::jsonb),('recap.lastSeenAt','"old"'),('learn.model.candidates','[]')`, prefs); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO suggestions(kind,agent,object,title,reason,primary_action,state,expires_at) VALUES
		('learn_rule','director','retired-open','old','{}','{}','open',now()+interval '1 day'),
		('learn_rule','director','retired-history','old','{}','{}','accepted',now()+interval '1 day'),
		('hot_video','scout','survivor','live','{}','{}','open',now()+interval '1 day');
		INSERT INTO background_tasks(id,kind,object_ref) VALUES(gen_random_uuid(),'assistant_plan','{}'),(gen_random_uuid(),'image_trial','{}');`); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, suggestionsCutoverSQL(t)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err = pool.QueryRow(ctx, `SELECT value FROM settings WHERE key='notifications.preferences'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	var retained struct{ Channel string }
	if err = json.Unmarshal(got["retained"], &retained); err != nil {
		t.Fatal(err)
	}
	if string(got["suggestions"]) != "false" || string(got["badge"]) != "false" || string(got["merge"]) != "true" || retained.Channel != "email" || got["recapOnReturn"] != nil {
		t.Fatalf("cutover changed unrelated preferences or failed to preserve global off: %s", raw)
	}
	rows, err := pool.Query(ctx, `SELECT object,state,resolved_at IS NOT NULL FROM suggestions ORDER BY object`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	states := map[string]string{}
	resolved := map[string]bool{}
	for rows.Next() {
		var object, state string
		var done bool
		if err = rows.Scan(&object, &state, &done); err != nil {
			t.Fatal(err)
		}
		states[object], resolved[object] = state, done
	}
	if rows.Err() != nil || states["retired-open"] != "expired" || !resolved["retired-open"] || states["retired-history"] != "accepted" || states["survivor"] != "open" {
		t.Fatalf("retired work/history boundaries: states=%v resolved=%v err=%v", states, resolved, rows.Err())
	}
	var removedKeys, tasks, liveTasks int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM settings WHERE key IN ('recap.lastSeenAt','learn.model.candidates')`).Scan(&removedKeys); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE kind='image_trial') FROM background_tasks`).Scan(&tasks, &liveTasks); err != nil {
		t.Fatal(err)
	}
	if removedKeys != 0 || tasks != 1 || liveTasks != 1 {
		t.Fatalf("retired pointers/tasks were not scoped: keys=%d tasks=%d live=%d", removedKeys, tasks, liveTasks)
	}
	for _, query := range []string{
		`INSERT INTO suggestions(kind,agent,object,title,reason,primary_action,state,expires_at) VALUES ('learn_rule','director','retired-new','old','{}','{}','open',now()+interval '1 day')`,
		`UPDATE suggestions SET state='open' WHERE object='retired-history'`,
	} {
		_, err = pool.Exec(ctx, query)
		var constraintError *pgconn.PgError
		if !errors.As(err, &constraintError) || constraintError.Code != "23514" || constraintError.ConstraintName != "suggestions_open_kind_check" {
			t.Fatalf("retired suggestions must remain historical, never open: %v", err)
		}
	}
}

func TestSuggestionsCutoverStopsBeforeLosingScopedOrPartialDisables(t *testing.T) {
	for name, prefs := range map[string]string{
		"scoped":  `{"recapOnReturn":true,"suggestions":{"off":[{"kind":"hot_video"},{"kind":"template_failing","channelId":"11111111-1111-4111-8111-111111111111"},{"kind":"cleanup","channelId":"11111111-1111-4111-8111-111111111111"}]}}`,
		"partial": `{"recapOnReturn":true,"suggestions":{"off":[{"kind":"hot_video"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool := provisionSuggestionsBeforeCutover(t)
			if _, err := pool.Exec(ctx, `INSERT INTO settings(key,value) VALUES('notifications.preferences',$1::jsonb)`, prefs); err != nil {
				t.Fatal(err)
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, suggestionsCutoverSQL(t))
			var migrationError *pgconn.PgError
			if !errors.As(err, &migrationError) || migrationError.Code != "P0001" {
				t.Fatalf("unrepresentable preferences must stop the cutover: %v", err)
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var unchanged bool
			if err := pool.QueryRow(ctx, `SELECT value = $1::jsonb FROM settings WHERE key='notifications.preferences'`, prefs).Scan(&unchanged); err != nil || !unchanged {
				t.Fatalf("stopped cutover changed the owner's opt-outs: unchanged=%v err=%v", unchanged, err)
			}
		})
	}
}

func TestSuggestionsCutoverRejectsMalformedScopeWithoutLosingLegacyData(t *testing.T) {
	ctx := context.Background()
	pool := provisionSuggestionsBeforeCutover(t)
	prefs := `{"recapOnReturn":false,"suggestions":{"off":[{"kind":"hot_video"},{"kind":"template_failing"},{"kind":"cleanup","channelId":47}]}}`
	if _, err := pool.Exec(ctx, `INSERT INTO settings(key,value) VALUES('notifications.preferences',$1::jsonb)`, prefs); err != nil {
		t.Fatal(err)
	}
	var before []byte
	if err := pool.QueryRow(ctx, `SELECT value FROM settings WHERE key='notifications.preferences'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, suggestionsCutoverSQL(t))
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || pgError.Code != "P0001" {
		_ = tx.Rollback(ctx)
		t.Fatalf("malformed scope must stop rather than turn off all suggestions: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var after []byte
	if err = pool.QueryRow(ctx, `SELECT value FROM settings WHERE key='notifications.preferences'`).Scan(&after); err != nil || string(after) != string(before) {
		t.Fatalf("failed cutover did not preserve legacy preference data: before=%s after=%s err=%v", before, after, err)
	}
}
