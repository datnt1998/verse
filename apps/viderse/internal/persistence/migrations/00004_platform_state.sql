-- +goose Up
-- Trạng thái bền của từng nền tảng nguồn (chuyển từ backdrop_platform_state
-- của yt-factory, mở rộng cho mọi nền tảng kể cả YouTube): nhịp gọi, giờ chờ
-- bị chặn tăng dần, lease của luồng quét và lỗi gần nhất cần người xử lý
-- (YouTube "xác nhận bạn không phải bot"). Một dòng mỗi platform.
CREATE TABLE IF NOT EXISTS platform_state (
    platform            text PRIMARY KEY,
    next_request_at     timestamptz NOT NULL DEFAULT 'epoch',
    paused_until        timestamptz,
    pause_reason        text,
    failure_count       integer NOT NULL DEFAULT 0,
    scanner_owner       text,
    scanner_lease_until timestamptz,
    last_error_code     text NOT NULL DEFAULT '',
    last_error_at       timestamptz,
    last_error_job_id   text
);

-- Grants mirror deploy/postgres-init; guard cho DB dev chưa có role app.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON platform_state TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS platform_state;
