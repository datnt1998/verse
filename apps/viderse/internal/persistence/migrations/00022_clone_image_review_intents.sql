-- +goose Up
ALTER TABLE clone_jobs ADD COLUMN image_approved_at timestamptz;
CREATE TABLE clone_image_review_intents (
 job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
 image_id text NOT NULL,
 item jsonb NOT NULL,
 expected_revision integer NOT NULL CHECK (expected_revision > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (job_id, image_id)
);

-- +goose Down
ALTER TABLE clone_jobs DROP COLUMN image_approved_at;
DROP TABLE clone_image_review_intents;
