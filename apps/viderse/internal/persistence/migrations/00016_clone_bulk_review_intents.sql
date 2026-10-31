-- +goose Up
CREATE TABLE clone_bulk_review_intents (
 job_id uuid PRIMARY KEY REFERENCES clone_jobs(id) ON DELETE CASCADE,
 image_revision integer NOT NULL CHECK(image_revision > 0),
 created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE clone_bulk_review_intents;
