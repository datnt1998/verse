-- +goose Up
-- Bộ lọc Tự lấy theo từng đối thủ của một kênh: chỉ lấy video dài từ
-- min_duration_sec giây. NULL = không lọc.
ALTER TABLE owner_channel_sources
    ADD COLUMN min_duration_sec integer CHECK (min_duration_sec IS NULL OR min_duration_sec > 0);

-- +goose Down
ALTER TABLE owner_channel_sources DROP COLUMN min_duration_sec;
