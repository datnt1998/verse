-- +goose Up
-- Kênh của bạn, Thư viện (template, prompt ảnh, lớp phủ) và Video nền.
-- Đường dẫn tệp (avatar, ảnh bìa kênh, lớp phủ, video lẻ) lưu tương đối so với
-- thư mục dự án (storage.Service.Resolve), để đổi thư mục không phải sửa DB.

-- Template: recipe JSON do internal/library kiểm theo một schema dùng chung cho
-- API và CLI. revision tăng mỗi lần lưu; ghi với revision cũ bị từ chối (409).
-- preview_revision là revision lúc dựng bản xem thử gần nhất: null = chưa có
-- bản xem thử, khác revision = bản xem thử đã cũ.
CREATE TABLE IF NOT EXISTS templates (
    id               uuid PRIMARY KEY,
    name             text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    recipe           jsonb NOT NULL,
    revision         bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    preview_revision bigint,
    preview_built_at timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS templates_updated_idx ON templates (updated_at DESC, id);

-- Prompt ảnh của bạn (07 §13). Prompt có sẵn (builtin:scene, builtin:cover-*)
-- là hằng số trong code, không có dòng ở đây. Tên duy nhất không phân biệt
-- hoa thường; trùng với tên prompt có sẵn do code kiểm.
CREATE TABLE IF NOT EXISTS image_prompts (
    id          uuid PRIMARY KEY,
    name        text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    kind        text NOT NULL CHECK (kind IN ('scene', 'cover')),
    composition text CHECK (composition IN ('clean', 'hook', 'circle-arrow', 'hook-arrow', 'split', 'closeup')),
    prompt      text NOT NULL CHECK (char_length(prompt) <= 2000),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'cover') = (composition IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS image_prompts_name_idx ON image_prompts (lower(name));

-- Lớp phủ: video/ảnh phủ lên toàn video, dùng chung cho mọi template. Một
-- dòng mỗi nội dung (sha256). builtin = id hiệu ứng có sẵn (rain, fog…), không
-- bỏ được. removed_at: lớp phủ tự thêm đã bỏ khỏi thư viện nhưng giữ tệp cho
-- template đang dùng.
CREATE TABLE IF NOT EXISTS overlays (
    id           uuid PRIMARY KEY,
    sha256       text NOT NULL UNIQUE CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    name         text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    file_path    text NOT NULL,
    mime         text NOT NULL,
    bytes        bigint NOT NULL CHECK (bytes > 0),
    duration_sec real,
    width        integer,
    height       integer,
    background   text NOT NULL DEFAULT 'none' CHECK (background IN ('black', 'green', 'none')),
    builtin      text UNIQUE,
    removed_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS overlays_listed_idx ON overlays (created_at DESC, id) WHERE removed_at IS NULL;

-- Kênh của bạn (07 §8, §16; mức tự chủ và ưu tiên kênh ở §20 chưa có).
-- name rỗng = chưa đặt tên (bước "đặt tên" của nhận diện sẽ điền).
-- auto_pick + pace_per_day: Tự lấy và Nhịp mỗi ngày; luồng tự lấy và hàng
-- chờ đọc chúng. template_cursor: con trỏ luân phiên template.
-- branding: {brief, summary, look, avatar, banner} của nhận diện; identity_*
-- là lần tạo nhận diện đang chạy hoặc vừa hỏng, với lease để chỉ một lần
-- chạy mỗi kênh và để phục hồi sau sập.
CREATE TABLE IF NOT EXISTS owner_channels (
    id                   uuid PRIMARY KEY,
    name                 text NOT NULL DEFAULT '' CHECK (char_length(name) <= 200),
    youtube_url          text CHECK (char_length(youtube_url) <= 2048),
    language             text NOT NULL DEFAULT 'auto' CHECK (language IN ('en', 'vi', 'auto')),
    style_note           text CHECK (char_length(style_note) <= 8000),
    description_footer   text CHECK (char_length(description_footer) <= 1000),
    always_tags          text[] NOT NULL DEFAULT '{}' CHECK (cardinality(always_tags) <= 10),
    auto_pick            boolean NOT NULL DEFAULT true,
    pace_per_day         integer NOT NULL DEFAULT 5 CHECK (pace_per_day BETWEEN 1 AND 50),
    template_cursor      integer NOT NULL DEFAULT 0,
    branding             jsonb NOT NULL DEFAULT '{}'::jsonb,
    avatar_path          text,
    banner_path          text,
    identity_state       text NOT NULL DEFAULT 'idle' CHECK (identity_state IN ('idle', 'running', 'failed')),
    identity_part        text CHECK (identity_part IN ('all', 'name', 'avatar', 'banner')),
    identity_task_id     text,
    identity_error_code  text,
    identity_error       text,
    -- Bước của run hỏng: 'text' (Đạo diễn đặt tên/mô tả) hoặc 'images' (Hoạ
    -- sĩ vẽ); NULL khi chưa tới bước nào hoặc run chết giữa chừng.
    identity_failed_step text CHECK (identity_failed_step IN ('text', 'images')),
    identity_retry_at    timestamptz,
    identity_started_at  timestamptz,
    identity_lease_owner text,
    identity_lease_token uuid,
    identity_lease_until timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

-- Đối thủ liên kết với kênh. Đối thủ tham chiếu nhận diện = dòng linked_at
-- sớm nhất. auto_baseline_at: Tự lấy chỉ tính video thấy lần đầu sau mốc này.
CREATE TABLE IF NOT EXISTS owner_channel_sources (
    owner_channel_id uuid NOT NULL REFERENCES owner_channels (id) ON DELETE CASCADE,
    rival_id         text NOT NULL REFERENCES rival_channels (id) ON DELETE CASCADE,
    linked_at        timestamptz NOT NULL DEFAULT clock_timestamp(),
    auto_baseline_at timestamptz,
    PRIMARY KEY (owner_channel_id, rival_id)
);
CREATE INDEX IF NOT EXISTS owner_channel_sources_order_idx ON owner_channel_sources (owner_channel_id, linked_at, rival_id);
CREATE INDEX IF NOT EXISTS owner_channel_sources_rival_idx ON owner_channel_sources (rival_id);

-- Template luân phiên của kênh, position liền mạch từ 0. Template đang được
-- kênh dùng thì không xoá được (RESTRICT, và library báo tên kênh).
CREATE TABLE IF NOT EXISTS owner_channel_templates (
    owner_channel_id uuid NOT NULL REFERENCES owner_channels (id) ON DELETE CASCADE,
    template_id      uuid NOT NULL REFERENCES templates (id) ON DELETE RESTRICT,
    position         integer NOT NULL CHECK (position >= 0),
    PRIMARY KEY (owner_channel_id, template_id)
);
CREATE INDEX IF NOT EXISTS owner_channel_templates_order_idx ON owner_channel_templates (owner_channel_id, position);
CREATE INDEX IF NOT EXISTS owner_channel_templates_template_idx ON owner_channel_templates (template_id);

-- Kênh video nền (Thư viện › Video nền › Kênh). scan_*: con trỏ đọc danh
-- sách Bilibili từng trang; list_error_*: lần đọc danh sách gần nhất hỏng.
CREATE TABLE IF NOT EXISTS backdrop_channels (
    id              text PRIMARY KEY CHECK (id ~ '^(twitch:[a-z0-9_]{1,25}|bilibili:[0-9]{1,20})$'),
    platform        text NOT NULL CHECK (platform IN ('twitch', 'bilibili')),
    handle          text NOT NULL,
    url             text NOT NULL,
    title           text,
    avatar_url      text,
    added_at        timestamptz NOT NULL DEFAULT now(),
    listed_at       timestamptz,
    list_error_code text,
    list_error      text,
    scan_next_page  integer,
    scan_total      integer NOT NULL DEFAULT 0,
    scan_stop_known boolean NOT NULL DEFAULT false
);

-- Danh mục video của một kênh nền (đọc từ nền tảng, không tải tệp).
-- position: thứ tự trong danh sách nền tảng theo từng loại.
CREATE TABLE IF NOT EXISTS backdrop_channel_videos (
    channel_id         text NOT NULL REFERENCES backdrop_channels (id) ON DELETE CASCADE,
    video_id           text NOT NULL,
    kind               text NOT NULL CHECK (kind IN ('clip', 'highlight', 'vod', 'video')),
    url                text NOT NULL,
    title              text NOT NULL DEFAULT '',
    duration_sec       real,
    thumbnail_url      text,
    published_at       timestamptz,
    position           integer NOT NULL DEFAULT 0,
    listed_at          timestamptz NOT NULL,
    probed_at          timestamptz,
    probe_info         jsonb,
    unavailable_at     timestamptz,
    unavailable_reason text CHECK (unavailable_reason IN ('expired', 'live', 'unavailable', 'login_required')),
    PRIMARY KEY (channel_id, video_id)
);
CREATE INDEX IF NOT EXISTS backdrop_channel_videos_newest_idx
    ON backdrop_channel_videos (channel_id, published_at DESC NULLS LAST, video_id);
CREATE INDEX IF NOT EXISTS backdrop_channel_videos_kind_idx
    ON backdrop_channel_videos (channel_id, kind, published_at DESC NULLS LAST, video_id);

-- Video lẻ (Thư viện › Video nền › Video lẻ, 07 §24): id "<platform>:<videoId>",
-- một đoạn dùng ≤ 600 giây, tải về bằng yt-dlp (việc nền backdrop_download).
CREATE TABLE IF NOT EXISTS backdrop_videos (
    id               text PRIMARY KEY CHECK (id ~ '^[a-z]+:.+$'),
    platform         text NOT NULL,
    url              text NOT NULL,
    title            text NOT NULL DEFAULT '',
    duration_sec     real,
    segment_from_sec real NOT NULL DEFAULT 0 CHECK (segment_from_sec >= 0),
    segment_to_sec   real NOT NULL,
    file_path        text,
    file_height      integer,
    file_bytes       bigint,
    state            text NOT NULL DEFAULT 'downloading' CHECK (state IN ('downloading', 'ready', 'failed')),
    progress         real CHECK (progress BETWEEN 0 AND 1),
    error_code       text,
    error            text,
    -- Chủ sở hữu lượt tải: mỗi lượt tải tăng generation; mọi ghi tiến độ/kết
    -- thúc phải khớp owner+generation (lease còn hạn) để lượt cũ không ghi đè
    -- lượt mới và process chết được dọn khi lease hết.
    download_generation  bigint NOT NULL DEFAULT 0,
    download_owner       text,
    download_lease_until timestamptz,
    added_at         timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (segment_to_sec > segment_from_sec AND segment_to_sec - segment_from_sec <= 600)
);
CREATE INDEX IF NOT EXISTS backdrop_videos_added_idx ON backdrop_videos (added_at DESC, id);

-- Yêu cầu hỗ trợ nền tảng chưa có (D8): lưu trên máy, gửi kèm khi Báo lỗi.
CREATE TABLE IF NOT EXISTS platform_requests (
    host     text PRIMARY KEY,
    label    text NOT NULL,
    count    integer NOT NULL DEFAULT 1 CHECK (count >= 1),
    first_at timestamptz NOT NULL DEFAULT now(),
    last_at  timestamptz NOT NULL DEFAULT now()
);

-- Điều tốc theo nền tảng cho Video nền: lịch sử yêu cầu trong một ngày
-- ([{at, cost}]) cho hạn mức cuộn của Bilibili, mốc đọc dần trang kế tiếp,
-- và lease yt-dlp để mọi process chỉ chạy một lệnh mỗi lúc trên nền tảng đó.
ALTER TABLE platform_state
    ADD COLUMN IF NOT EXISTS request_history   jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS next_trickle_at   timestamptz NOT NULL DEFAULT 'epoch',
    ADD COLUMN IF NOT EXISTS ytdlp_owner       text,
    ADD COLUMN IF NOT EXISTS ytdlp_lease_until timestamptz;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
            ON templates, image_prompts, overlays, owner_channels, owner_channel_sources,
               owner_channel_templates, backdrop_channels, backdrop_channel_videos,
               backdrop_videos, platform_requests TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE platform_state
    DROP COLUMN IF EXISTS ytdlp_lease_until,
    DROP COLUMN IF EXISTS ytdlp_owner,
    DROP COLUMN IF EXISTS next_trickle_at,
    DROP COLUMN IF EXISTS request_history;
DROP TABLE IF EXISTS platform_requests;
DROP TABLE IF EXISTS backdrop_videos;
DROP TABLE IF EXISTS backdrop_channel_videos;
DROP TABLE IF EXISTS backdrop_channels;
DROP TABLE IF EXISTS owner_channel_templates;
DROP TABLE IF EXISTS owner_channel_sources;
DROP TABLE IF EXISTS owner_channels;
DROP TABLE IF EXISTS overlays;
DROP TABLE IF EXISTS image_prompts;
DROP TABLE IF EXISTS templates;
