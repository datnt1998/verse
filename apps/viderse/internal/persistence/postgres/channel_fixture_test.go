package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const fixtureTemplate = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"

// All fixtures use provision's private schema; never seed a production DB.
func seedFixtureTemplate(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO templates(id,name,recipe) VALUES($1,'Fixture','{}') ON CONFLICT(id) DO NOTHING`, fixtureTemplate); err != nil {
		t.Fatal(err)
	}
}

func seedCloneChannel(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	seedFixtureTemplate(t, pool)
	if _, err := pool.Exec(context.Background(), `INSERT INTO owner_channels(id,name,language,auto_fetch,default_template_id) VALUES('11111111-1111-4111-8111-111111111111','Clone fixture','en',false,$1)`, fixtureTemplate); err != nil {
		t.Fatal(err)
	}
}
