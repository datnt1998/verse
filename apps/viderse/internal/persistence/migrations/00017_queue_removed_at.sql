-- +goose Up
ALTER TABLE clone_queue ADD COLUMN removed_at timestamptz;
DROP INDEX clone_queue_active_key_idx;
CREATE UNIQUE INDEX clone_queue_active_key_idx ON clone_queue(source_key)
    WHERE state = 'queued' AND removed_at IS NULL;
CREATE INDEX clone_queue_removed_at_idx ON clone_queue(removed_at)
    WHERE removed_at IS NOT NULL;

-- +goose Down
-- Throwaway-only: discard tombstones before restoring the old unique index.
DELETE FROM clone_queue WHERE removed_at IS NOT NULL;
DROP INDEX clone_queue_removed_at_idx;
DROP INDEX clone_queue_active_key_idx;
CREATE UNIQUE INDEX clone_queue_active_key_idx ON clone_queue(source_key) WHERE state = 'queued';
ALTER TABLE clone_queue DROP COLUMN removed_at;
