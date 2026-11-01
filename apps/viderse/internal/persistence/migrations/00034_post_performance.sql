-- +goose Up
-- Số liệu sau đăng (§21): mỗi video đã đăng có một dòng "facts" chụp lúc bắt
-- đầu theo dõi (tiêu đề, độ dài, link YouTube) và tối đa ba dòng số liệu theo
-- mốc 24 giờ / 7 ngày / 28 ngày. Lỗi đọc chỉ để trống, không chặn gì.
CREATE TABLE clone_post_facts (
    job_id uuid PRIMARY KEY REFERENCES clone_jobs(id) ON DELETE CASCADE,
    youtube_id text,
    title text NOT NULL DEFAULT '',
    duration_sec integer,
    published_at timestamptz,
    read_attempts integer NOT NULL DEFAULT 0,
    next_read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX clone_post_facts_read_idx ON clone_post_facts(next_read_at) WHERE youtube_id IS NOT NULL;
CREATE TABLE clone_post_stats (
    job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
    milestone text NOT NULL CHECK (milestone IN ('24h','7d','28d')),
    views bigint NOT NULL CHECK (views >= 0),
    likes bigint CHECK (likes >= 0),
    comments bigint CHECK (comments >= 0),
    source text NOT NULL CHECK (source IN ('data_api','public_page')),
    read_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, milestone)
);
-- "Cho chạy nhiều hơn": template được chạy gấp đôi lượt trong luân phiên.
ALTER TABLE owner_channel_templates ADD COLUMN weight smallint NOT NULL DEFAULT 1 CHECK (weight BETWEEN 1 AND 2);
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON clone_post_facts, clone_post_stats TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE owner_channel_templates DROP COLUMN weight;
DROP TABLE clone_post_stats;
DROP TABLE clone_post_facts;
