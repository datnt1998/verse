-- +goose Up
ALTER TABLE clone_queue ADD COLUMN restart_of uuid REFERENCES clone_jobs(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE clone_queue DROP COLUMN restart_of;
