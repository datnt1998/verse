-- +goose Up
CREATE TABLE clone_auto_actions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK (kind IN ('safety_redraw','step_retry')),
 text text NOT NULL,
 reason jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX clone_auto_actions_created_idx ON clone_auto_actions(created_at DESC,id DESC);
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
  GRANT SELECT, INSERT ON clone_auto_actions TO viderse_app;
 END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE clone_auto_actions;
