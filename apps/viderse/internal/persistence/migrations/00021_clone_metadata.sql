-- +goose Up
CREATE TABLE clone_metadata_states (
    job_id uuid PRIMARY KEY REFERENCES clone_jobs(id) ON DELETE CASCADE,
    state jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE clone_metadata_states;
