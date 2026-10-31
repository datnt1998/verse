-- +goose Up
CREATE TABLE clone_image_override_intents (
 job_id uuid PRIMARY KEY REFERENCES clone_jobs(id) ON DELETE CASCADE,
 expected_revision integer NOT NULL CHECK (expected_revision > 0),
 manifest jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE clone_image_override_intents;
