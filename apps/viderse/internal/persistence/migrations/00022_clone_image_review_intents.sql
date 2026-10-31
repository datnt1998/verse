-- +goose Up
CREATE TABLE clone_image_review_intents (
 job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
 image_id text NOT NULL,
 item jsonb NOT NULL,
 expected_revision integer NOT NULL CHECK (expected_revision > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (job_id, image_id)
);

-- +goose Down
DROP TABLE clone_image_review_intents;
