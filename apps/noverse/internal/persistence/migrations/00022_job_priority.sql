-- +goose Up
ALTER TABLE jobs ADD COLUMN queue_priority integer NOT NULL DEFAULT 0 CHECK (queue_priority >= 0);
CREATE INDEX jobs_claim_priority ON jobs (kind, queue_priority DESC, created_at, id)
    WHERE state IN ('queued', 'running');

-- +goose Down
DROP INDEX jobs_claim_priority;
ALTER TABLE jobs DROP COLUMN queue_priority;
