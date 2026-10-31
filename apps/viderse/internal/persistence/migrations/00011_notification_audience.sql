-- +goose Up
ALTER TABLE notifications
    ADD COLUMN audience text NOT NULL DEFAULT 'all' CHECK (audience IN ('all', 'admin')),
    ADD COLUMN object_ref jsonb;
ALTER TABLE error_events
    ADD COLUMN audience text NOT NULL DEFAULT 'all' CHECK (audience IN ('all', 'admin'));
CREATE INDEX notifications_audience_unread ON notifications (audience, created_at DESC) WHERE read_at IS NULL;

-- +goose Down
DROP INDEX notifications_audience_unread;
ALTER TABLE error_events DROP COLUMN audience;
ALTER TABLE notifications DROP COLUMN object_ref, DROP COLUMN audience;
