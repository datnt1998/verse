-- Truy vấn của thư viện Video nền: kênh nền, danh mục video của kênh, video
-- lẻ, đọc recipe template (chỉ đọc) và yêu cầu hỗ trợ nền tảng.

-- name: ListBackdropChannels :many
SELECT * FROM backdrop_channels
ORDER BY added_at DESC, id ASC;

-- name: GetBackdropChannel :one
SELECT * FROM backdrop_channels
WHERE id = $1;

-- name: InsertBackdropChannel :execrows
INSERT INTO backdrop_channels (
    id, platform, handle, url, title, avatar_url, added_at,
    listed_at, list_error_code, list_error, scan_next_page, scan_total, scan_stop_known
) VALUES (
    sqlc.arg(id), sqlc.arg(platform), sqlc.arg(handle), sqlc.arg(url), sqlc.narg(title),
    sqlc.narg(avatar_url), sqlc.arg(added_at)::timestamptz, sqlc.narg(listed_at)::timestamptz,
    sqlc.arg(list_error_code), sqlc.arg(list_error), sqlc.narg(scan_next_page),
    sqlc.arg(scan_total), sqlc.arg(scan_stop_known)
)
ON CONFLICT (id) DO NOTHING;

-- name: MarkBackdropChannelListed :exec
-- Lần đọc danh sách thành công: đặt listed_at, xoá lỗi cũ.
UPDATE backdrop_channels
SET listed_at = sqlc.arg(listed_at)::timestamptz, list_error_code = '', list_error = ''
WHERE id = sqlc.arg(id);

-- name: SetBackdropChannelListError :exec
-- Lần đọc danh sách hỏng: giữ nguyên listed_at và danh mục đệm.
UPDATE backdrop_channels
SET list_error_code = sqlc.arg(code), list_error = sqlc.arg(message)
WHERE id = sqlc.arg(id);

-- name: UpdateBackdropChannelScan :exec
UPDATE backdrop_channels
SET scan_next_page = sqlc.narg(next_page),
    scan_stop_known = sqlc.arg(stop_known),
    scan_total = sqlc.arg(total),
    title = coalesce(sqlc.narg(title), title)
WHERE id = sqlc.arg(id);

-- name: UpdateBackdropChannelTitle :exec
UPDATE backdrop_channels
SET title = sqlc.arg(title)
WHERE id = sqlc.arg(id);

-- name: DeleteBackdropChannel :execrows
DELETE FROM backdrop_channels WHERE id = $1;

-- name: ListBackdropChannelVideos :many
SELECT * FROM backdrop_channel_videos
WHERE channel_id = $1
ORDER BY kind, position, video_id;

-- name: GetBackdropChannelVideo :one
SELECT * FROM backdrop_channel_videos
WHERE channel_id = $1 AND video_id = $2;

-- name: UpsertBackdropChannelVideos :exec
-- Ghi một lượt đọc danh sách: trường đã dò (duration, ảnh, ngày đăng) sống
-- sót khi danh sách không mang, lý do tự hết (expired/live) được xoá.
INSERT INTO backdrop_channel_videos (
    channel_id, video_id, kind, url, title, duration_sec, thumbnail_url, published_at, position, listed_at
)
SELECT
    sqlc.arg(channel_id)::text, v.video_id, v.kind, v.url, coalesce(v.title, ''),
    v.duration_sec, v.thumbnail_url, v.published_at, v.position, sqlc.arg(listed_at)::timestamptz
FROM jsonb_to_recordset(sqlc.arg(videos)::jsonb) AS v(
    video_id text, kind text, url text, title text, duration_sec real,
    thumbnail_url text, published_at timestamptz, position integer
)
ON CONFLICT (channel_id, video_id) DO UPDATE SET
    kind = excluded.kind,
    url = excluded.url,
    position = excluded.position,
    listed_at = excluded.listed_at,
    title = coalesce(nullif(excluded.title, ''), backdrop_channel_videos.title),
    duration_sec = coalesce(excluded.duration_sec, backdrop_channel_videos.duration_sec),
    thumbnail_url = coalesce(excluded.thumbnail_url, backdrop_channel_videos.thumbnail_url),
    published_at = coalesce(excluded.published_at, backdrop_channel_videos.published_at),
    unavailable_at = CASE
        WHEN backdrop_channel_videos.unavailable_reason IN ('expired', 'live') THEN NULL
        ELSE backdrop_channel_videos.unavailable_at END,
    unavailable_reason = CASE
        WHEN backdrop_channel_videos.unavailable_reason IN ('expired', 'live') THEN NULL
        ELSE backdrop_channel_videos.unavailable_reason END;

-- name: ExpireMissingBackdropVods :exec
-- Twitch giữ VOD 7–60 ngày: VOD vắng mặt trong danh sách trọn là đã bị xoá.
-- Clip và highlight không bao giờ hết hạn.
UPDATE backdrop_channel_videos
SET unavailable_at = sqlc.arg(now)::timestamptz, unavailable_reason = 'expired'
WHERE channel_id = sqlc.arg(channel_id)
  AND kind = 'vod'
  AND unavailable_at IS NULL
  AND NOT (video_id = ANY(sqlc.arg(keep)::text[]));

-- name: MarkBackdropChannelVideoUnavailable :exec
UPDATE backdrop_channel_videos
SET unavailable_at = sqlc.arg(now)::timestamptz, unavailable_reason = sqlc.arg(reason)
WHERE channel_id = sqlc.arg(channel_id) AND video_id = sqlc.arg(video_id);

-- name: UpdateBackdropChannelVideoProbe :exec
UPDATE backdrop_channel_videos
SET title = coalesce(sqlc.narg(title), title),
    duration_sec = coalesce(sqlc.narg(duration_sec), duration_sec),
    thumbnail_url = coalesce(sqlc.narg(thumbnail_url), thumbnail_url),
    published_at = coalesce(sqlc.narg(published_at)::timestamptz, published_at),
    probed_at = sqlc.arg(probed_at)::timestamptz,
    probe_info = coalesce(sqlc.narg(probe_info)::jsonb, probe_info),
    unavailable_at = CASE WHEN sqlc.arg(clear_unavailable)::boolean THEN NULL ELSE unavailable_at END,
    unavailable_reason = CASE WHEN sqlc.arg(clear_unavailable)::boolean THEN NULL ELSE unavailable_reason END
WHERE channel_id = sqlc.arg(channel_id) AND video_id = sqlc.arg(video_id);

-- name: UsableBackdropCounts :many
SELECT channel_id, kind, count(*)::int AS total
FROM backdrop_channel_videos
WHERE channel_id = ANY(sqlc.arg(channel_ids)::text[]) AND unavailable_at IS NULL
GROUP BY channel_id, kind;

-- name: PageBackdropChannelVideos :many
-- Keyset trên (published_at DESC NULLS LAST, video_id ASC). `has_cursor` và
-- các tham số con trỏ đến từ ChannelVideoCursor đã kiểm.
SELECT * FROM backdrop_channel_videos
WHERE channel_id = sqlc.arg(channel_id)
  AND (cardinality(sqlc.arg(kinds)::text[]) = 0 OR kind = ANY(sqlc.arg(kinds)::text[]))
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (
          sqlc.narg(cursor_published)::timestamptz IS NULL
          AND published_at IS NULL
          AND video_id > sqlc.arg(cursor_video)::text
      )
      OR (
          sqlc.narg(cursor_published)::timestamptz IS NOT NULL
          AND (
              published_at < sqlc.narg(cursor_published)::timestamptz
              OR (published_at = sqlc.narg(cursor_published)::timestamptz AND video_id > sqlc.arg(cursor_video)::text)
              OR published_at IS NULL
          )
      )
  )
ORDER BY published_at DESC NULLS LAST, video_id ASC
LIMIT sqlc.arg(limit_count);

-- name: CountBackdropChannelVideos :one
SELECT count(*)::int
FROM backdrop_channel_videos
WHERE channel_id = sqlc.arg(channel_id)
  AND (cardinality(sqlc.arg(kinds)::text[]) = 0 OR kind = ANY(sqlc.arg(kinds)::text[]));

-- name: CountUnavailableBackdropChannelVideos :one
SELECT count(*)::int
FROM backdrop_channel_videos
WHERE channel_id = sqlc.arg(channel_id)
  AND unavailable_at IS NOT NULL
  AND (cardinality(sqlc.arg(kinds)::text[]) = 0 OR kind = ANY(sqlc.arg(kinds)::text[]));

-- name: ListBackdropVideos :many
SELECT * FROM backdrop_videos
ORDER BY added_at DESC, id ASC;

-- name: GetBackdropVideo :one
SELECT * FROM backdrop_videos WHERE id = $1;

-- name: InsertBackdropVideo :execrows
INSERT INTO backdrop_videos (
    id, platform, url, title, duration_sec, segment_from_sec, segment_to_sec,
    file_path, file_height, file_bytes, state, progress, error_code, error, added_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(platform), sqlc.arg(url), sqlc.arg(title), sqlc.narg(duration_sec),
    sqlc.arg(segment_from_sec), sqlc.arg(segment_to_sec), sqlc.narg(file_path),
    sqlc.narg(file_height), sqlc.narg(file_bytes), sqlc.arg(state), sqlc.narg(progress),
    sqlc.arg(error_code), sqlc.arg(error), sqlc.arg(added_at)::timestamptz, sqlc.arg(updated_at)::timestamptz
)
ON CONFLICT (id) DO NOTHING;

-- name: UpdateBackdropVideo :exec
UPDATE backdrop_videos
SET state = coalesce(sqlc.narg(state), state),
    progress = CASE WHEN sqlc.arg(clear_progress)::boolean THEN NULL ELSE coalesce(sqlc.narg(progress), progress) END,
    file_path = coalesce(sqlc.narg(file_path), file_path),
    file_height = coalesce(sqlc.narg(file_height), file_height),
    file_bytes = coalesce(sqlc.narg(file_bytes), file_bytes),
    error_code = coalesce(sqlc.narg(error_code), error_code),
    error = coalesce(sqlc.narg(error), error),
    segment_from_sec = coalesce(sqlc.narg(segment_from_sec), segment_from_sec),
    segment_to_sec = coalesce(sqlc.narg(segment_to_sec), segment_to_sec),
    title = coalesce(sqlc.narg(title), title),
    duration_sec = coalesce(sqlc.narg(duration_sec), duration_sec),
    updated_at = sqlc.arg(updated_at)::timestamptz
WHERE id = sqlc.arg(id);

-- name: DeleteBackdropVideo :execrows
DELETE FROM backdrop_videos WHERE id = $1;

-- name: ClaimBackdropVideoDownload :one
UPDATE backdrop_videos
SET download_generation = download_generation + 1,
    download_owner = sqlc.arg(owner),
    download_lease_until = sqlc.arg(lease_until)::timestamptz,
    state = 'downloading',
    progress = 0,
    error_code = NULL,
    error = NULL,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)
  AND (sqlc.arg(force)::boolean
       OR state <> 'downloading'
       OR download_lease_until IS NULL
       OR download_lease_until < sqlc.arg(now)::timestamptz)
RETURNING download_generation;

-- name: TouchBackdropVideoDownload :execrows
UPDATE backdrop_videos
SET progress = sqlc.narg(progress),
    download_lease_until = sqlc.arg(lease_until)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)
  AND download_owner = sqlc.arg(owner)
  AND download_generation = sqlc.arg(generation);

-- name: FinishBackdropVideoDownload :execrows
UPDATE backdrop_videos
SET state = 'ready',
    progress = NULL,
    file_path = sqlc.arg(file_path),
    file_height = sqlc.arg(file_height),
    file_bytes = sqlc.arg(file_bytes),
    error_code = NULL,
    error = NULL,
    download_owner = NULL,
    download_lease_until = NULL,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)
  AND download_owner = sqlc.arg(owner)
  AND download_generation = sqlc.arg(generation);

-- name: FailBackdropVideoDownload :execrows
UPDATE backdrop_videos
SET state = 'failed',
    progress = NULL,
    error_code = sqlc.arg(error_code),
    error = sqlc.arg(error),
    download_owner = NULL,
    download_lease_until = NULL,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)
  AND download_owner = sqlc.arg(owner)
  AND download_generation = sqlc.arg(generation);

-- name: ClearStaleBackdropVideoDownloads :execrows
UPDATE backdrop_videos
SET state = 'failed',
    progress = NULL,
    error_code = 'interrupted',
    error = 'Lượt tải bị ngắt trước khi xong',
    download_owner = NULL,
    download_lease_until = NULL,
    updated_at = sqlc.arg(now)::timestamptz
WHERE state = 'downloading'
  AND (download_lease_until IS NULL OR download_lease_until < sqlc.arg(now)::timestamptz);

-- name: ListTemplatesForBackdrop :many
SELECT id::text AS id, name, recipe::text AS recipe
FROM templates
ORDER BY name, id;

-- name: RecordPlatformRequest :one
INSERT INTO platform_requests (host, label, count, first_at, last_at)
VALUES (sqlc.arg(host), sqlc.arg(label), 1, sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz)
ON CONFLICT (host) DO UPDATE SET
    label = sqlc.arg(label),
    count = platform_requests.count + 1,
    last_at = sqlc.arg(now)::timestamptz
RETURNING *;

-- name: ListPlatformRequests :many
SELECT * FROM platform_requests
ORDER BY last_at DESC, host ASC;

-- name: LockBackdropResource :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(key)::text, 0));

-- name: NotifyBackdropChange :exec
SELECT pg_notify('backdrop_changed', sqlc.arg(key)::text);
