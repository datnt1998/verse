-- +goose Up
ALTER TABLE notifications ADD COLUMN detail jsonb;

-- +goose Down
ALTER TABLE notifications DROP COLUMN detail;
