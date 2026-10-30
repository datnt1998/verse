-- +goose Up
-- Chủ sở hữu việc nền và lịch sử lỗi bền (7 ngày) cho mục Báo lỗi.
ALTER TABLE background_tasks ADD COLUMN IF NOT EXISTS owner_pid integer NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS error_events (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    at           timestamptz NOT NULL DEFAULT now(),
    kind         text NOT NULL,
    what         text NOT NULL,
    why          text NOT NULL DEFAULT '',
    href         text NOT NULL DEFAULT '',
    action_label text NOT NULL DEFAULT '',
    op           text NOT NULL DEFAULT '',
    object_ref   jsonb
);

CREATE INDEX IF NOT EXISTS error_events_at_idx ON error_events (at DESC);

-- +goose Down
DROP INDEX IF EXISTS error_events_at_idx;
DROP TABLE IF EXISTS error_events;
ALTER TABLE background_tasks DROP COLUMN IF EXISTS owner_pid;
