-- +goose Up
-- Trinh sát (Gói V5): kênh đối thủ, video đã đọc từ tab Videos, trạng thái
-- quét từng kênh và lịch chung của luồng quét (chuyển từ channels, videos,
-- channel_scan_state, channel_scan_jobs, channel_scan_schedule của yt-factory).
--
-- Lease của luồng quét toàn cục nằm ở platform_state (dòng 'youtube');
-- fence của từng kênh là các cột lease_* của scan_state. Không có bảng lease
-- riêng.

-- Extension là toàn cục của database nên ghim vào public (xem pgtest).
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

-- Một dòng mỗi kênh đối thủ đang theo dõi. Xoá kênh xoá theo video, trạng thái
-- quét và việc quét (ON DELETE CASCADE).
CREATE TABLE IF NOT EXISTS rival_channels (
    id               text PRIMARY KEY CHECK (id ~ '^UC[A-Za-z0-9_-]{22}$'),
    handle           text,
    title            text NOT NULL,
    description      text NOT NULL DEFAULT '',
    avatar_url       text,
    url              text NOT NULL,
    subscriber_count bigint,
    -- Mức thường: trung vị lượt xem của 150 video mới nhất có lượt xem; null
    -- khi chưa có số nào. median_at là lúc tính.
    median_views     bigint,
    median_at        timestamptz,
    added_at         timestamptz NOT NULL DEFAULT now(),
    -- Lần kiểm tra video mới gần nhất chạy xong.
    scanned_at       timestamptz
);

-- Video của kênh đối thủ. position tăng dần từ mới tới cũ, bất biến sau lần
-- ghi đầu (cùng origin và first_seen_at); các trường số liệu được làm mới mỗi
-- lần đọc lại, giá trị null không xoá giá trị đã biết.
CREATE TABLE IF NOT EXISTS rival_videos (
    id                text PRIMARY KEY CHECK (id ~ '^[A-Za-z0-9_-]{11}$'),
    channel_id        text NOT NULL REFERENCES rival_channels (id) ON DELETE CASCADE,
    title             text NOT NULL,
    duration_sec      integer CHECK (duration_sec >= 0),
    view_count        bigint CHECK (view_count >= 0),
    published_text    text,
    -- Ước lượng từ chữ tương đối ("3 ngày trước") lúc đọc trang.
    published_at      timestamptz,
    -- Giờ đăng chính xác đọc từ trang video; upload_checked_at khác null nghĩa
    -- là đã đọc xong (kể cả khi không có giờ).
    uploaded_at       timestamptz,
    upload_checked_at timestamptz,
    -- Có phụ đề do kênh đăng (không tính phụ đề tự động); null = chưa đọc.
    has_captions      boolean,
    -- Điểm nổi trội = view_count / median_views của kênh; tính lại cả kênh
    -- trong cùng giao dịch mỗi khi mức thường đổi.
    score             double precision,
    position          bigint NOT NULL,
    origin            text NOT NULL CHECK (origin IN ('baseline', 'history', 'discovery')),
    first_seen_at      timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    -- Hoãn kiểm tra duy nhất tới lúc commit: lúc dồn lại thứ tự (chạm miền
    -- khoá an toàn) một câu UPDATE đổi toàn bộ position, kiểm tra từng hàng sẽ
    -- đụng chính các hàng chưa kịp đổi. Vẫn bắt buộc duy nhất khi commit.
    CONSTRAINT rival_videos_channel_position_key UNIQUE (channel_id, position)
        DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX IF NOT EXISTS rival_videos_channel_published_idx
    ON rival_videos (channel_id, published_at DESC);
CREATE INDEX IF NOT EXISTS rival_videos_channel_score_idx
    ON rival_videos (channel_id, score DESC NULLS LAST, position);
CREATE INDEX IF NOT EXISTS rival_videos_channel_views_idx
    ON rival_videos (channel_id, view_count DESC NULLS LAST, position);
CREATE INDEX IF NOT EXISTS rival_videos_score_idx
    ON rival_videos (score DESC NULLS LAST, id);
-- Khoá "mới nhất" gộp mọi kênh: giờ đăng chính xác rồi ước lượng, null cuối.
CREATE INDEX IF NOT EXISTS rival_videos_published_idx
    ON rival_videos ((coalesce(uploaded_at, published_at)) DESC NULLS LAST, id);
CREATE INDEX IF NOT EXISTS rival_videos_title_trgm_idx
    ON rival_videos USING gin (title gin_trgm_ops);

-- Trạng thái quét của từng kênh. history_* giữ chỗ dừng của lần đọc đầu để
-- việc quét lịch sử đọc tiếp; head_* là mốc của lần kiểm tra video mới gần
-- nhất (anchors = id các video mới nhất đã biết, dùng để biết đã gặp lại video
-- cũ). lease_* là fence của kênh: mọi ghi của luồng quét kiểm tra token còn
-- hạn trong cùng giao dịch.
CREATE TABLE IF NOT EXISTS scan_state (
    channel_id          text PRIMARY KEY REFERENCES rival_channels (id) ON DELETE CASCADE,
    history_cap         integer NOT NULL DEFAULT 2000 CHECK (history_cap BETWEEN 150 AND 10000),
    indexed_videos      integer NOT NULL DEFAULT 0,
    history_end         boolean NOT NULL DEFAULT false,
    history_cursor      text,
    history_remainder   text,
    head_anchors        text[] NOT NULL DEFAULT '{}',
    head_newest_id      text,
    head_checked_at     timestamptz,
    discovery_caught_up boolean NOT NULL DEFAULT true,
    revision            bigint NOT NULL DEFAULT 0,
    lease_owner         text,
    lease_token         uuid,
    lease_until         timestamptz,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- Việc quét: history (quét lịch sử tới history_cap) hoặc discovery (kiểm tra
-- video mới tới khi gặp video đã biết). Mỗi kênh có tối đa một việc còn sống
-- mỗi loại; việc đã xong giữ lại để hiện kết quả.
CREATE TABLE IF NOT EXISTS scan_jobs (
    id              uuid PRIMARY KEY,
    channel_id      text NOT NULL REFERENCES rival_channels (id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('history', 'discovery')),
    state           text NOT NULL CHECK (state IN
        ('queued', 'running', 'backoff', 'paused', 'capped', 'complete', 'cancelled', 'error')),
    requested_cap   integer,
    cursor          text,
    cursor_at       timestamptz,
    remainder       text,
    replay_target   text,
    last_video_id   text,
    order_base      bigint,
    next_ordinal    integer NOT NULL DEFAULT 0,
    anchors         text[] NOT NULL DEFAULT '{}',
    newest_id       text,
    pages           integer NOT NULL DEFAULT 0,
    scanned         integer NOT NULL DEFAULT 0,
    added           integer NOT NULL DEFAULT 0,
    attempts        integer NOT NULL DEFAULT 0,
    restarts        integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz,
    error_code      text NOT NULL DEFAULT '',
    last_error      text NOT NULL DEFAULT '',
    recovery        text NOT NULL DEFAULT '',
    reached_end     boolean NOT NULL DEFAULT false,
    task_id         uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    started_at      timestamptz,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    finished_at     timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS scan_jobs_live_idx ON scan_jobs (channel_id, kind)
    WHERE state IN ('queued', 'running', 'backoff', 'paused', 'capped', 'error');
CREATE INDEX IF NOT EXISTS scan_jobs_runnable_idx ON scan_jobs (next_attempt_at)
    WHERE state IN ('queued', 'running', 'backoff');
-- Việc gần nhất mỗi loại của một kênh (danh sách đối thủ đọc một hàng mỗi kênh):
-- cột đúng thứ tự sắp xếp "còn sống trước, rồi mới nhất". "Còn sống" khớp
-- liveJobTx của ScanStore: complete và cancelled là đã dừng.
CREATE INDEX IF NOT EXISTS scan_jobs_latest_idx
    ON scan_jobs (channel_id, kind,
                  (state NOT IN ('complete', 'cancelled')) DESC, updated_at DESC);

-- Lịch chung của luồng quét (một dòng): việc đọc giờ đăng chính xác tạm nghỉ
-- tới upload_times_next_at sau khi bị chặn hoặc lỗi liên tiếp.
CREATE TABLE IF NOT EXISTS scan_schedule (
    id                   boolean PRIMARY KEY DEFAULT true CHECK (id),
    upload_times_next_at timestamptz NOT NULL DEFAULT 'epoch',
    upload_time_failures integer NOT NULL DEFAULT 0
);

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
            ON rival_channels, rival_videos, scan_state, scan_jobs, scan_schedule TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS scan_schedule;
DROP TABLE IF EXISTS scan_jobs;
DROP TABLE IF EXISTS scan_state;
DROP TABLE IF EXISTS rival_videos;
DROP TABLE IF EXISTS rival_channels;
