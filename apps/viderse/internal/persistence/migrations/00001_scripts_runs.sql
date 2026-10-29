-- +goose Up
-- Foundation: the two tables the Writer pipeline owns.
--   agent_runs  one row per submitted brief → run lifecycle + usage accounting
--   scripts     one row per saved script, pointing at the run that produced it

CREATE TABLE IF NOT EXISTS agent_runs (
    id              uuid PRIMARY KEY,
    status          text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    provider        text NOT NULL,
    model           text NOT NULL,
    brief           jsonb NOT NULL,
    input_tokens    integer NOT NULL DEFAULT 0,
    output_tokens   integer NOT NULL DEFAULT 0,
    cost_usd        double precision,
    error           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    started_at      timestamptz,
    finished_at     timestamptz
);

CREATE TABLE IF NOT EXISTS scripts (
    id              uuid PRIMARY KEY,
    run_id          uuid NOT NULL REFERENCES agent_runs (id),
    title           text NOT NULL DEFAULT '',
    sections        jsonb NOT NULL,
    full_text       text NOT NULL,
    word_count      integer NOT NULL,
    est_duration_sec integer NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS agent_runs_created_idx ON agent_runs (created_at DESC);
CREATE INDEX IF NOT EXISTS scripts_run_idx ON scripts (run_id);

-- Grants mirror deploy/postgres-init: the app role gets DML on every table the
-- migration role creates, never DDL. Guarded so the same migration runs on a
-- development database where roles may not exist yet.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON agent_runs, scripts TO viderse_app;
        ALTER DEFAULT PRIVILEGES FOR ROLE CURRENT_USER
            GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS scripts;
DROP TABLE IF EXISTS agent_runs;
