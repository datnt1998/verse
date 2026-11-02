-- Đọc cho mảng đối thủ (Khám phá). Ghi scan/video thuộc queries/scan.sql
-- của luồng quét; file này chỉ chứa truy vấn đọc và đếm.

-- name: ListRivals :many
-- Một truy vấn cho cả sidebar: kênh + số video/điểm nổi trội/nổi trội mới +
-- trạng thái quét lịch sử hiển thị + cờ kiểm tra video mới đang chạy.
-- Việc lịch sử chọn giống liveJobTx của kho quét: việc còn sống (state NOT IN
-- ('complete','cancelled')) trước, không thì việc cập nhật gần nhất; running mà
-- worker đã chết (lease platform_state hết hạn) đọc là queued. "Còn sống" phải
-- khớp định nghĩa của scan.sql, nếu không một run đã huỷ sẽ thắng một run mới
-- đã xong và sidebar hiện "đã huỷ" cho kênh vừa quét xong.
SELECT c.id, c.handle, c.title, c.description, c.avatar_url, c.url,
       c.subscriber_count, c.median_views, c.median_at, c.added_at, c.scanned_at,
       COALESCE(s.indexed_videos, 0) AS indexed_videos,
       COALESCE(s.history_end, false) AS history_end,
       COALESCE(counts.video_count, 0)::bigint AS video_count,
       COALESCE(counts.outliers, 0)::bigint AS outliers,
       COALESCE(counts.new_outliers, 0)::bigint AS new_outliers,
       COALESCE(hj.state, '') AS history_job_state,
       CASE
           WHEN hj.state IS NULL THEN CASE WHEN s.history_end THEN 'complete' ELSE 'idle' END
           WHEN hj.state = 'running' AND NOT COALESCE(worker.live, false) THEN 'queued'
           ELSE hj.state
       END::text AS history_state,
       COALESCE(dj.state IN ('queued', 'running', 'backoff'), false)::bool AS discovery_running
FROM rival_channels c
LEFT JOIN scan_state s ON s.channel_id = c.id
-- Một lượt gộp cho toàn bộ video (không phải một subquery tương quan mỗi
-- kênh): số đếm sidebar giữ nguyên chi phí khi số kênh tăng.
LEFT JOIN (
    SELECT v.channel_id,
           count(*)::bigint AS video_count,
           count(*) FILTER (WHERE v.score >= sqlc.arg(outlier_min)::float8)::bigint AS outliers,
           count(*) FILTER (WHERE v.score >= sqlc.arg(outlier_min)::float8
                              AND v.origin = 'discovery'
                              AND v.first_seen_at >= sqlc.arg(new_since)::timestamptz)::bigint AS new_outliers
    FROM rival_videos v
    GROUP BY v.channel_id
) counts ON counts.channel_id = c.id
LEFT JOIN LATERAL (
    SELECT j.state FROM scan_jobs j
    WHERE j.channel_id = c.id AND j.kind = 'history'
    ORDER BY (j.state NOT IN ('complete', 'cancelled')) DESC, j.updated_at DESC
    LIMIT 1
) hj ON true
LEFT JOIN LATERAL (
    SELECT j.state FROM scan_jobs j
    WHERE j.channel_id = c.id AND j.kind = 'discovery'
    ORDER BY (j.state NOT IN ('complete', 'cancelled')) DESC, j.updated_at DESC
    LIMIT 1
) dj ON true
LEFT JOIN LATERAL (
    SELECT (p.scanner_lease_until > sqlc.arg(now)::timestamptz) AS live
    FROM platform_state p
    WHERE p.platform = sqlc.arg(platform)
) worker ON true
ORDER BY c.added_at ASC, c.id;

-- name: RivalExists :one
SELECT EXISTS(SELECT 1 FROM rival_channels WHERE id = $1)::bool AS exists;

-- name: CountRivalVideos :one
-- Tổng video khớp bộ lọc của một kênh (channel_id NOT NULL) hoặc mọi kênh
-- (channel_id NULL). q là mẫu LIKE đã escape sẵn ('' = không lọc); so với
-- title_fold sau khi gấp q bằng fold_title (không phân biệt hoa thường, dấu).
-- made/notMade lọc theo lượt làm thật (job chưa bị thay thế/chưa huỷ hoặc mục
-- hàng chờ còn sống), tính trước LIMIT và trước khi đếm.
SELECT count(*)::bigint
FROM rival_videos v
CROSS JOIN LATERAL (
    SELECT array_agg(DISTINCT u.channel_id) AS channels
    FROM (
        SELECT j.channel_id::text AS channel_id
        FROM clone_jobs j
        WHERE j.source_key IN ('channel:' || j.channel_id::text || ':video:' || v.id, 'video:' || v.id)
          AND j.superseded_at IS NULL
          AND j.status <> 'cancelled'
        UNION
        SELECT q.channel_id::text
        FROM clone_queue q
        WHERE q.source_video_id = v.id
          AND q.state IN ('queued', 'started')
          AND q.removed_at IS NULL
    ) u
) made
WHERE (sqlc.narg(channel_id)::text IS NULL OR v.channel_id = sqlc.narg(channel_id)::text)
  AND (NOT sqlc.arg(long_only)::bool OR v.duration_sec >= sqlc.arg(long_min_sec)::int)
  AND (sqlc.arg(within_days)::int <= 0
       OR COALESCE(v.uploaded_at, v.published_at) >= sqlc.arg(now)::timestamptz - sqlc.arg(within_days)::int * interval '1 day')
  AND (sqlc.arg(q)::text = '' OR v.title_fold LIKE fold_title(sqlc.arg(q)::text))
  AND (sqlc.narg(made_for)::text IS NULL
       OR sqlc.narg(made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[])))
  AND (sqlc.narg(not_made_for)::text IS NULL
       OR NOT (sqlc.narg(not_made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[]))))
  AND (NOT sqlc.arg(made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) > 0)
  AND (NOT sqlc.arg(not_made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) = 0);

-- name: PageRivalVideosByOutlier :many
-- Trang keyset theo điểm nổi trội (score DESC NULLS LAST, id ASC). Cursor =
-- (cursor_score, cursor_id) của dòng cuối trang trước; cursor_score NULL bao
-- hết khoá sau khi mọi score khác null đã qua.
SELECT v.id, v.channel_id, v.title, v.duration_sec, v.view_count,
       v.published_text, v.published_at, v.uploaded_at, v.upload_checked_at,
       v.has_captions, v.score, v.position, v.origin, v.first_seen_at, v.updated_at
FROM rival_videos v
CROSS JOIN LATERAL (
    SELECT array_agg(DISTINCT u.channel_id) AS channels
    FROM (
        SELECT j.channel_id::text AS channel_id
        FROM clone_jobs j
        WHERE j.source_key IN ('channel:' || j.channel_id::text || ':video:' || v.id, 'video:' || v.id)
          AND j.superseded_at IS NULL
          AND j.status <> 'cancelled'
        UNION
        SELECT q.channel_id::text
        FROM clone_queue q
        WHERE q.source_video_id = v.id
          AND q.state IN ('queued', 'started')
          AND q.removed_at IS NULL
    ) u
) made
WHERE (sqlc.narg(channel_id)::text IS NULL OR v.channel_id = sqlc.narg(channel_id)::text)
  AND (NOT sqlc.arg(long_only)::bool OR v.duration_sec >= sqlc.arg(long_min_sec)::int)
  AND (sqlc.arg(within_days)::int <= 0
       OR COALESCE(v.uploaded_at, v.published_at) >= sqlc.arg(now)::timestamptz - sqlc.arg(within_days)::int * interval '1 day')
  AND (sqlc.arg(q)::text = '' OR v.title_fold LIKE fold_title(sqlc.arg(q)::text))
  AND (sqlc.narg(made_for)::text IS NULL
       OR sqlc.narg(made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[])))
  AND (sqlc.narg(not_made_for)::text IS NULL
       OR NOT (sqlc.narg(not_made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[]))))
  AND (NOT sqlc.arg(made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) > 0)
  AND (NOT sqlc.arg(not_made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) = 0)
  AND (sqlc.narg(cursor_id)::text IS NULL
       OR (sqlc.narg(cursor_score)::float8 IS NOT NULL AND
           (v.score < sqlc.narg(cursor_score)::float8
            OR (v.score = sqlc.narg(cursor_score)::float8 AND v.id > sqlc.narg(cursor_id)::text)
            OR v.score IS NULL))
       OR (sqlc.narg(cursor_score)::float8 IS NULL AND v.score IS NULL AND v.id > sqlc.narg(cursor_id)::text))
ORDER BY v.score DESC NULLS LAST, v.id ASC
LIMIT sqlc.arg(page_size)::bigint;

-- name: PageRivalVideosByViews :many
-- Trang keyset theo lượt xem (view_count DESC NULLS LAST, id ASC).
SELECT v.id, v.channel_id, v.title, v.duration_sec, v.view_count,
       v.published_text, v.published_at, v.uploaded_at, v.upload_checked_at,
       v.has_captions, v.score, v.position, v.origin, v.first_seen_at, v.updated_at
FROM rival_videos v
CROSS JOIN LATERAL (
    SELECT array_agg(DISTINCT u.channel_id) AS channels
    FROM (
        SELECT j.channel_id::text AS channel_id
        FROM clone_jobs j
        WHERE j.source_key IN ('channel:' || j.channel_id::text || ':video:' || v.id, 'video:' || v.id)
          AND j.superseded_at IS NULL
          AND j.status <> 'cancelled'
        UNION
        SELECT q.channel_id::text
        FROM clone_queue q
        WHERE q.source_video_id = v.id
          AND q.state IN ('queued', 'started')
          AND q.removed_at IS NULL
    ) u
) made
WHERE (sqlc.narg(channel_id)::text IS NULL OR v.channel_id = sqlc.narg(channel_id)::text)
  AND (NOT sqlc.arg(long_only)::bool OR v.duration_sec >= sqlc.arg(long_min_sec)::int)
  AND (sqlc.arg(within_days)::int <= 0
       OR COALESCE(v.uploaded_at, v.published_at) >= sqlc.arg(now)::timestamptz - sqlc.arg(within_days)::int * interval '1 day')
  AND (sqlc.arg(q)::text = '' OR v.title_fold LIKE fold_title(sqlc.arg(q)::text))
  AND (sqlc.narg(made_for)::text IS NULL
       OR sqlc.narg(made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[])))
  AND (sqlc.narg(not_made_for)::text IS NULL
       OR NOT (sqlc.narg(not_made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[]))))
  AND (NOT sqlc.arg(made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) > 0)
  AND (NOT sqlc.arg(not_made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) = 0)
  AND (sqlc.narg(cursor_id)::text IS NULL
       OR (sqlc.narg(cursor_views)::bigint IS NOT NULL AND
           (v.view_count < sqlc.narg(cursor_views)::bigint
            OR (v.view_count = sqlc.narg(cursor_views)::bigint AND v.id > sqlc.narg(cursor_id)::text)
            OR v.view_count IS NULL))
       OR (sqlc.narg(cursor_views)::bigint IS NULL AND v.view_count IS NULL AND v.id > sqlc.narg(cursor_id)::text))
ORDER BY v.view_count DESC NULLS LAST, v.id ASC
LIMIT sqlc.arg(page_size)::bigint;

-- name: PageRivalVideosByPosition :many
-- "Mới nhất" của một kênh: position ASC (nhỏ = mới), id ASC.
SELECT v.id, v.channel_id, v.title, v.duration_sec, v.view_count,
       v.published_text, v.published_at, v.uploaded_at, v.upload_checked_at,
       v.has_captions, v.score, v.position, v.origin, v.first_seen_at, v.updated_at
FROM rival_videos v
CROSS JOIN LATERAL (
    SELECT array_agg(DISTINCT u.channel_id) AS channels
    FROM (
        SELECT j.channel_id::text AS channel_id
        FROM clone_jobs j
        WHERE j.source_key IN ('channel:' || j.channel_id::text || ':video:' || v.id, 'video:' || v.id)
          AND j.superseded_at IS NULL
          AND j.status <> 'cancelled'
        UNION
        SELECT q.channel_id::text
        FROM clone_queue q
        WHERE q.source_video_id = v.id
          AND q.state IN ('queued', 'started')
          AND q.removed_at IS NULL
    ) u
) made
WHERE v.channel_id = sqlc.arg(channel_id)
  AND (NOT sqlc.arg(long_only)::bool OR v.duration_sec >= sqlc.arg(long_min_sec)::int)
  AND (sqlc.arg(within_days)::int <= 0
       OR COALESCE(v.uploaded_at, v.published_at) >= sqlc.arg(now)::timestamptz - sqlc.arg(within_days)::int * interval '1 day')
  AND (sqlc.arg(q)::text = '' OR v.title_fold LIKE fold_title(sqlc.arg(q)::text))
  AND (sqlc.narg(made_for)::text IS NULL
       OR sqlc.narg(made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[])))
  AND (sqlc.narg(not_made_for)::text IS NULL
       OR NOT (sqlc.narg(not_made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[]))))
  AND (NOT sqlc.arg(made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) > 0)
  AND (NOT sqlc.arg(not_made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) = 0)
  AND (sqlc.narg(cursor_id)::text IS NULL
       OR v.position > sqlc.narg(cursor_pos)::bigint
       OR (v.position = sqlc.narg(cursor_pos)::bigint AND v.id > sqlc.narg(cursor_id)::text))
ORDER BY v.position ASC, v.id ASC
LIMIT sqlc.arg(page_size)::bigint;

-- name: PageAllRivalVideosByNewest :many
-- "Mới nhất" gộp mọi kênh: giờ đăng (chính xác rồi ước lượng) DESC NULLS
-- LAST, id ASC — cùng khoá với rival_videos_published_idx.
SELECT v.id, v.channel_id, v.title, v.duration_sec, v.view_count,
       v.published_text, v.published_at, v.uploaded_at, v.upload_checked_at,
       v.has_captions, v.score, v.position, v.origin, v.first_seen_at, v.updated_at
FROM rival_videos v
CROSS JOIN LATERAL (
    SELECT array_agg(DISTINCT u.channel_id) AS channels
    FROM (
        SELECT j.channel_id::text AS channel_id
        FROM clone_jobs j
        WHERE j.source_key IN ('channel:' || j.channel_id::text || ':video:' || v.id, 'video:' || v.id)
          AND j.superseded_at IS NULL
          AND j.status <> 'cancelled'
        UNION
        SELECT q.channel_id::text
        FROM clone_queue q
        WHERE q.source_video_id = v.id
          AND q.state IN ('queued', 'started')
          AND q.removed_at IS NULL
    ) u
) made
WHERE (NOT sqlc.arg(long_only)::bool OR v.duration_sec >= sqlc.arg(long_min_sec)::int)
  AND (sqlc.arg(within_days)::int <= 0
       OR COALESCE(v.uploaded_at, v.published_at) >= sqlc.arg(now)::timestamptz - sqlc.arg(within_days)::int * interval '1 day')
  AND (sqlc.arg(q)::text = '' OR v.title_fold LIKE fold_title(sqlc.arg(q)::text))
  AND (sqlc.narg(made_for)::text IS NULL
       OR sqlc.narg(made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[])))
  AND (sqlc.narg(not_made_for)::text IS NULL
       OR NOT (sqlc.narg(not_made_for)::text = ANY(COALESCE(made.channels, ARRAY[]::text[]))))
  AND (NOT sqlc.arg(made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) > 0)
  AND (NOT sqlc.arg(not_made_any)::bool OR COALESCE(array_length(made.channels, 1), 0) = 0)
  AND (sqlc.narg(cursor_id)::text IS NULL
       OR (sqlc.narg(cursor_ts)::timestamptz IS NOT NULL AND
           (COALESCE(v.uploaded_at, v.published_at) < sqlc.narg(cursor_ts)::timestamptz
            OR (COALESCE(v.uploaded_at, v.published_at) = sqlc.narg(cursor_ts)::timestamptz AND v.id > sqlc.narg(cursor_id)::text)
            OR COALESCE(v.uploaded_at, v.published_at) IS NULL))
       OR (sqlc.narg(cursor_ts)::timestamptz IS NULL AND COALESCE(v.uploaded_at, v.published_at) IS NULL AND v.id > sqlc.narg(cursor_id)::text))
ORDER BY COALESCE(v.uploaded_at, v.published_at) DESC NULLS LAST, v.id ASC
LIMIT sqlc.arg(page_size)::bigint;

-- name: ListPostingSample :many
-- Mẫu giờ đăng: 30 video mới nhất (position) của kênh, kèm cờ đã đọc giờ
-- đăng chính xác để suy ra total/pending.
SELECT v.id, v.uploaded_at, v.upload_checked_at
FROM rival_videos v
WHERE v.channel_id = sqlc.arg(channel_id)
ORDER BY v.position ASC, v.id ASC
LIMIT sqlc.arg(sample)::bigint;

-- name: UploadProbePausedUntil :one
-- Giờ tạm nghỉ đọc giờ đăng của luồng quét (một dòng scan_schedule; chưa có
-- dòng = không nghỉ). epoch nghĩa là không nghỉ.
SELECT COALESCE(
    (SELECT ss.upload_times_next_at FROM scan_schedule ss WHERE ss.id),
    'epoch'::timestamptz
)::timestamptz AS paused_until;
-- name: CountRivalChannels :one
SELECT count(*)::bigint FROM rival_channels;

-- name: OwnerChannelExists :one
-- madeFor/notMadeFor chỉ nhận kênh của bạn; so bằng text để id sai hình dạng
-- trả false thay vì lỗi ép uuid.
SELECT EXISTS(SELECT 1 FROM owner_channels WHERE id::text = sqlc.arg(owner_channel_id)::text)::bool AS exists;

-- name: RivalVideoChannel :one
-- Kênh đối thủ của một video — dùng để kiểm phạm vi trước khi xem trước.
SELECT channel_id FROM rival_videos WHERE id = $1;

-- name: ListVideoUsage :many
-- Lượt làm thật của từng video trong trang: job chưa bị thay thế/chưa huỷ,
-- cộng mục hàng chờ còn sống. Một truy vấn cho cả trang (không N+1); kênh
-- của bạn lấy tên hiển thị, mục hàng chờ lấy id để mở đúng chỗ.
SELECT v.id AS video_id,
       oc.id AS channel_id,
       oc.name AS channel_name,
       j.stage_group AS stage_group,
       COALESCE(j.active_step, '')::text AS active_step,
       j.id AS job_id,
       q.id AS queue_item_id,
       j.updated_at AS updated_at
FROM rival_videos v
JOIN clone_jobs j
  ON j.source_key IN ('channel:' || j.channel_id::text || ':video:' || v.id, 'video:' || v.id)
JOIN owner_channels oc ON oc.id = j.channel_id
LEFT JOIN LATERAL (
    SELECT q.id FROM clone_queue q
    WHERE q.job_id = j.id
    ORDER BY q.created_at DESC, q.id
    LIMIT 1
) q ON true
WHERE v.id = ANY(sqlc.arg(video_ids)::text[])
  AND j.superseded_at IS NULL
  AND j.status <> 'cancelled'
UNION ALL
SELECT v.id,
       oc.id,
       oc.name,
       'queued'::text,
       ''::text,
       NULL::uuid,
       q.id,
       q.created_at
FROM rival_videos v
JOIN clone_queue q
  ON q.source_video_id = v.id
 AND q.state = 'queued'
 AND q.removed_at IS NULL
JOIN owner_channels oc ON oc.id = q.channel_id
WHERE v.id = ANY(sqlc.arg(video_ids)::text[])
  AND NOT EXISTS (
    SELECT 1 FROM clone_jobs j2
    WHERE j2.channel_id = q.channel_id
      AND j2.source_key IN ('channel:' || j2.channel_id::text || ':video:' || v.id, 'video:' || v.id)
      AND j2.superseded_at IS NULL
      AND j2.status <> 'cancelled')
ORDER BY video_id, channel_name;

-- name: ListAutoPickBaselines :many
-- Mốc bật Tự lấy của từng cặp (kênh của bạn, đối thủ) — dùng cho dòng giải
-- thích "Tự lấy chưa lấy video này" khi video đăng trước mốc.
SELECT s.owner_channel_id, c.name, c.auto_pick, s.rival_id, s.auto_baseline_at
FROM owner_channel_sources s
JOIN owner_channels c ON c.id = s.owner_channel_id
WHERE s.auto_baseline_at IS NOT NULL
ORDER BY s.owner_channel_id, s.rival_id;

-- name: SourceInfoArtifactPath :one
-- Đường dẫn artifact download/source.info.json đã công bố của video nguồn (bất
-- kỳ job nào, ưu tiên job đang sống) — dùng để đọc chương thật khi xem trước.
SELECT a.path
FROM clone_artifacts a
JOIN clone_jobs j ON j.id = a.job_id
WHERE a.path LIKE '%/download/source.info.json'
  AND j.source_key IN ('channel:' || j.channel_id::text || ':video:' || sqlc.arg(video_id)::text,
                       'video:' || sqlc.arg(video_id)::text)
ORDER BY (j.superseded_at IS NULL) DESC, j.updated_at DESC
LIMIT 1;
