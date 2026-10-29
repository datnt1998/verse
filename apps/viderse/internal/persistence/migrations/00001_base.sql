-- +goose Up
-- Schema nền của Viderse: ba bảng dùng chung mọi mảng nghiệp vụ phụ thuộc vào.
--   settings          khoá/giá trị JSONB cho các cờ và tham số (vd cờ tạm dừng
--                     hàng chờ clone.queuePaused) — hình dáng theo settings-store
--                     của yt-factory nhưng value là jsonb nên DB tự chặn chuỗi
--                     không phải JSON.
--   background_tasks  việc nền đang chạy cho Việc nền & thông báo
--                     (hợp đồng dữ liệu §11).
--   notifications     lịch sử thông báo đã gộp cho cùng mục đích.

CREATE TABLE IF NOT EXISTS settings (
    key         text PRIMARY KEY,
    value       jsonb NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS background_tasks (
    id          uuid PRIMARY KEY,
    kind        text NOT NULL,
    agent       text,
    title       text NOT NULL DEFAULT '',
    object_ref  jsonb NOT NULL,
    phase       text NOT NULL DEFAULT '',
    progress    jsonb,
    eta_sec     integer,
    state       text NOT NULL DEFAULT 'running'
        CHECK (state IN ('running', 'paused', 'waiting', 'failed')),
    action      jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS background_tasks_state_idx ON background_tasks (state);

CREATE TABLE IF NOT EXISTS notifications (
    id           uuid PRIMARY KEY,
    kind         text NOT NULL,
    count        integer NOT NULL DEFAULT 1,
    text         text NOT NULL DEFAULT '',
    href         text NOT NULL DEFAULT '',
    action_label text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    read_at      timestamptz
);

CREATE INDEX IF NOT EXISTS notifications_created_idx ON notifications (created_at DESC);

-- Grants mirror deploy/postgres-init: the app role gets DML on every table the
-- migration role creates, never DDL. Guarded so the same migration runs on a
-- development database where roles may not exist yet.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON settings, background_tasks, notifications TO viderse_app;
        ALTER DEFAULT PRIVILEGES FOR ROLE CURRENT_USER
            GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS background_tasks;
DROP TABLE IF EXISTS settings;
