-- Trinh sát (Gói V5): đọc/ghi của luồng quét đối thủ — fence kênh trên
-- scan_state.lease_*, lease worker trên platform_state('youtube'), việc quét,
-- upsert trang video và thăm dò giờ đăng.

-- ── thứ tự khoá của một giao dịch ghi ───────────────────────────────────────
-- Mọi giao dịch ghi của luồng quét khoá theo đúng một thứ tự:
-- rival_channels → scan_state → scan_jobs → rival_videos. Nhờ vậy trang đang
-- ghi và lệnh gỡ kênh không thể khoá chéo nhau, và fence chỉ được kiểm sau khi
-- đã giữ khoá trạng thái (không còn khe đọc-rồi-ghi).

-- name: ScanLockChannel :one
SELECT id FROM rival_channels WHERE id = $1 FOR UPDATE;

-- ── kênh đối thủ ────────────────────────────────────────────────────────────

-- name: ScanGetChannel :one
SELECT * FROM rival_channels WHERE id = $1;

-- name: ScanDeleteChannel :execrows
DELETE FROM rival_channels WHERE id = $1;

-- name: ScanInsertChannel :execrows
-- Chỉ lần ghi đầu của một đối thủ mới mới tạo dòng kênh: insert thuần, 0 dòng
-- nghĩa là kênh đã có chủ (một lượt thêm khác thắng cuộc đua) — không bao giờ
-- ghi đè hồ sơ đã có, cũng không làm sống lại kênh vừa bị gỡ.
INSERT INTO rival_channels (id, handle, title, description, avatar_url, url, subscriber_count)
VALUES (
    sqlc.arg(id), sqlc.narg(handle), sqlc.arg(title),
    coalesce(sqlc.narg(description), ''), sqlc.narg(avatar_url),
    sqlc.arg(url), sqlc.narg(subscriber_count)
)
ON CONFLICT (id) DO NOTHING;

-- name: ScanUpdateChannelHead :exec
-- Mức thường + mốc "đã kiểm tra video mới" sau head check.
UPDATE rival_channels
SET median_views = sqlc.narg(median_views),
    median_at = sqlc.arg(at)::timestamptz,
    scanned_at = sqlc.arg(at)::timestamptz
WHERE id = sqlc.arg(id);

-- ── scan_state + fence kênh ─────────────────────────────────────────────────

-- name: ScanEnsureState :one
-- Tạo lười dòng trạng thái từ dữ liệu đã lưu: đếm video, anchors là 150 id
-- đầu theo thứ tự kênh. Trả dòng (mới hay cũ); nhánh ON CONFLICT lấy khoá của
-- hàng đã có nên caller khoá parent trước là đủ thứ tự.
INSERT INTO scan_state (channel_id, indexed_videos, head_anchors, head_newest_id)
SELECT sqlc.arg(channel_id),
       (SELECT count(*) FROM rival_videos WHERE channel_id = sqlc.arg(channel_id)),
       coalesce((SELECT array_agg(id) FROM (
           SELECT id FROM rival_videos WHERE channel_id = sqlc.arg(channel_id)
           ORDER BY position, id LIMIT 150
       ) a), '{}'::text[]),
       (SELECT id FROM rival_videos WHERE channel_id = sqlc.arg(channel_id)
        ORDER BY position, id LIMIT 1)
WHERE EXISTS (SELECT 1 FROM rival_channels WHERE id = sqlc.arg(channel_id))
ON CONFLICT (channel_id) DO UPDATE SET channel_id = EXCLUDED.channel_id
RETURNING *;

-- name: ScanAcquireChannelLease :one
-- Giữ fence của kênh: thành công khi chưa ai giữ hoặc lease đã hết hạn
-- (không có dòng trả về = một holder khác còn hạn).
UPDATE scan_state
SET lease_owner = sqlc.arg(owner),
    lease_token = sqlc.arg(token)::uuid,
    lease_until = sqlc.arg(until)::timestamptz
WHERE channel_id = sqlc.arg(channel_id)
  AND (lease_until IS NULL OR lease_until <= sqlc.arg(now)::timestamptz)
RETURNING channel_id;

-- name: ScanRenewChannelLease :execrows
-- Gia hạn fence theo token; 0 dòng = fence đã đổi chủ.
UPDATE scan_state
SET lease_until = sqlc.arg(until)::timestamptz
WHERE channel_id = sqlc.arg(channel_id) AND lease_token = sqlc.arg(token)::uuid;

-- name: ScanReleaseChannelLease :exec
UPDATE scan_state
SET lease_owner = NULL, lease_token = NULL, lease_until = NULL
WHERE channel_id = sqlc.arg(channel_id) AND lease_token = sqlc.arg(token)::uuid;

-- name: ScanUpdateHistoryCap :exec
UPDATE scan_state
SET history_cap = sqlc.arg(cap), updated_at = sqlc.arg(now)::timestamptz
WHERE channel_id = sqlc.arg(channel_id);

-- name: ScanClearHistoryBookmark :exec
-- Bookmark tab Videos chuyển sang việc quét lịch sử khi việc được tạo.
UPDATE scan_state
SET history_cursor = NULL, history_remainder = NULL, updated_at = sqlc.arg(now)::timestamptz
WHERE channel_id = sqlc.arg(channel_id);

-- name: ScanCommitState :exec
-- Cập nhật trạng thái sau một trang đã commit: đếm + revision luôn tăng;
-- history_end chỉ lật lên; bookmark chỉ ghi ở trang baseline (phần dư của lần
-- đọc đầu chuyển cho việc lịch sử); head chỉ ghi ở đúng đường quét.
UPDATE scan_state
SET indexed_videos = indexed_videos + sqlc.arg(added),
    revision = revision + 1,
    history_end = history_end OR sqlc.arg(ended)::boolean,
    history_cursor = CASE WHEN sqlc.arg(save_bookmark)::boolean
                          THEN sqlc.narg(cursor) ELSE history_cursor END,
    history_remainder = CASE WHEN sqlc.arg(save_bookmark)::boolean
                             THEN sqlc.narg(remainder) ELSE history_remainder END,
    head_newest_id = CASE WHEN sqlc.arg(mark_head)::boolean
                          THEN sqlc.narg(newest_id) ELSE head_newest_id END,
    head_anchors = CASE WHEN sqlc.arg(set_anchors)::boolean
                        THEN sqlc.arg(anchors)::text[] ELSE head_anchors END,
    head_checked_at = CASE WHEN sqlc.arg(mark_head)::boolean
                           THEN sqlc.arg(checked_at)::timestamptz ELSE head_checked_at END,
    discovery_caught_up = CASE WHEN sqlc.arg(mark_discovery)::boolean
                               THEN false ELSE discovery_caught_up END,
    updated_at = sqlc.arg(now)::timestamptz
WHERE channel_id = sqlc.arg(channel_id);

-- name: ScanSetHeadState :exec
-- Mốc đầu kênh sau head check: anchors mới, video mới nhất đã thấy, đã đuổi kịp.
UPDATE scan_state
SET head_anchors = sqlc.arg(anchors)::text[],
    head_newest_id = coalesce(sqlc.narg(newest_id), head_newest_id),
    head_checked_at = sqlc.arg(checked_at)::timestamptz,
    discovery_caught_up = true,
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE channel_id = sqlc.arg(channel_id);

-- ── video ───────────────────────────────────────────────────────────────────

-- name: ScanVideoRefs :many
SELECT id, channel_id, position FROM rival_videos WHERE id = ANY($1::text[]);

-- name: ScanMaxPosition :one
SELECT coalesce(max(position) + 1, 0)::bigint FROM rival_videos WHERE channel_id = $1;

-- name: ScanMinPosition :one
SELECT coalesce(min(position), 0)::bigint FROM rival_videos WHERE channel_id = $1;

-- name: ScanRecentAnchorIDs :many
-- Video mới nhất không đến từ quét lịch sử: chỉ những id này được làm anchor
-- (quét lịch sử dở không được đóng khoảng trống video mới).
SELECT id FROM rival_videos
WHERE channel_id = $1 AND origin IN ('baseline', 'discovery')
ORDER BY position, id
LIMIT 150;

-- name: ScanRebaseOrder :exec
-- Gom về dãy 0..n-1 theo thứ tự hiện tại — chỉ chạy lúc mở một run discovery
-- khi khối khoá sắp chạm miền an toàn. Khoá duy nhất của (channel_id, position)
-- hoãn tới lúc commit nên một câu UPDATE đổi toàn bộ vị trí là hợp lệ.
WITH ranked AS (
    SELECT id, (row_number() OVER (ORDER BY position, id) - 1) AS rn
    FROM rival_videos WHERE channel_id = $1
)
UPDATE rival_videos v SET position = r.rn
FROM ranked r WHERE v.id = r.id AND v.channel_id = $1;

-- name: ScanUpsertVideo :exec
-- Một dòng của trang: id là khoá; position/origin/first_seen_at bất biến sau
-- lần ghi đầu; null của trang mới không xoá giá trị đã biết; hàng của kênh
-- khác không được đụng (setWhere channel_id khớp). score tính theo mức thường
-- hiện tại của kênh (median truyền vào, 0 = chưa có).
INSERT INTO rival_videos (id, channel_id, title, duration_sec, view_count,
    published_text, published_at, score, position, origin, first_seen_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(channel_id), sqlc.arg(title),
    sqlc.narg(duration_sec), sqlc.narg(view_count),
    sqlc.narg(published_text), sqlc.narg(published_at),
    CASE WHEN sqlc.arg(median)::bigint > 0 AND sqlc.narg(view_count)::bigint IS NOT NULL
         THEN sqlc.narg(view_count)::bigint::double precision / sqlc.arg(median_f)::double precision
         ELSE NULL END,
    sqlc.arg(position), sqlc.arg(origin),
    sqlc.arg(first_seen)::timestamptz, sqlc.arg(updated)::timestamptz
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    duration_sec = coalesce(EXCLUDED.duration_sec, rival_videos.duration_sec),
    view_count = coalesce(EXCLUDED.view_count, rival_videos.view_count),
    published_text = coalesce(EXCLUDED.published_text, rival_videos.published_text),
    published_at = coalesce(EXCLUDED.published_at, rival_videos.published_at),
    score = CASE WHEN sqlc.arg(median)::bigint > 0
                      AND coalesce(EXCLUDED.view_count, rival_videos.view_count) IS NOT NULL
                 THEN coalesce(EXCLUDED.view_count, rival_videos.view_count)::double precision
                      / sqlc.arg(median_f)::double precision
                 ELSE NULL END,
    updated_at = EXCLUDED.updated_at
WHERE rival_videos.channel_id = EXCLUDED.channel_id;

-- name: ScanMedianSample :many
-- Mẫu trung vị: 150 hàng mới nhất theo thứ tự kênh, lấy view_count (null rơi ra
-- khi tính median, không phải khi chọn mẫu).
SELECT view_count FROM rival_videos
WHERE channel_id = $1
ORDER BY position, id
LIMIT 150;

-- name: ScanRecomputeScores :execrows
-- Tính lại điểm nổi trội cả kênh khi mức thường đổi (trong tx head check);
-- chỉ ghi hàng có điểm thật sự khác (so sánh null-safe) nên lần đổi mức nhỏ
-- không chạm cả bảng.
UPDATE rival_videos
SET score = CASE WHEN sqlc.arg(median)::bigint > 0 AND view_count IS NOT NULL
                 THEN view_count::double precision / sqlc.arg(median_f)::double precision
                 ELSE NULL END
WHERE channel_id = sqlc.arg(channel_id)
  AND score IS DISTINCT FROM CASE WHEN sqlc.arg(median)::bigint > 0 AND view_count IS NOT NULL
                 THEN view_count::double precision / sqlc.arg(median_f)::double precision
                 ELSE NULL END;

-- ── việc quét ───────────────────────────────────────────────────────────────

-- name: ScanLiveJob :one
-- Việc còn sống của một loại — đúng tập trạng thái mà chỉ mục duy nhất giữ
-- (complete/cancelled đã dừng hẳn nên không chặn lượt mới).
SELECT * FROM scan_jobs
WHERE channel_id = $1 AND kind = $2
  AND state IN ('queued', 'running', 'backoff', 'paused', 'capped', 'error')
ORDER BY updated_at DESC
LIMIT 1;

-- name: ScanLatestJob :one
-- Việc gần nhất của một loại kể cả đã xong — cho trạng thái hiển thị.
SELECT * FROM scan_jobs
WHERE channel_id = $1 AND kind = $2
ORDER BY updated_at DESC
LIMIT 1;

-- name: ScanGetJob :one
SELECT * FROM scan_jobs WHERE id = $1;

-- name: ScanInsertHistoryJob :one
INSERT INTO scan_jobs (id, channel_id, kind, state, requested_cap,
    cursor, cursor_at, remainder, started_at)
VALUES (sqlc.arg(id)::uuid, sqlc.arg(channel_id), 'history', sqlc.arg(state),
        sqlc.arg(cap), sqlc.narg(cursor),
        CASE WHEN sqlc.narg(cursor)::text IS NULL THEN NULL ELSE sqlc.arg(now)::timestamptz END,
        sqlc.narg(remainder), sqlc.arg(now)::timestamptz)
RETURNING *;

-- name: ScanInsertDiscoveryJob :one
INSERT INTO scan_jobs (id, channel_id, kind, state, order_base, anchors, started_at)
VALUES (sqlc.arg(id)::uuid, sqlc.arg(channel_id), 'discovery', 'queued',
        sqlc.arg(order_base), sqlc.arg(anchors)::text[], sqlc.arg(now)::timestamptz)
RETURNING *;

-- name: ScanPruneFinishedJobs :exec
-- Dọn việc đã dừng của một kênh/loại, chừa đúng dòng đã dừng mới nhất: mỗi
-- giờ một lượt kiểm tra video mới là một dòng complete, thay lượt hỏng là một
-- dòng cancelled — không dọn thì bảng lớn mãi. Luồng quét không đọc lại việc
-- cũ; danh sách đối thủ chỉ đọc trạng thái của dòng mới nhất, còn mốc kiểm tra
-- gần nhất đã nằm ở scan_state.head_checked_at.
DELETE FROM scan_jobs j
WHERE j.channel_id = sqlc.arg(channel_id)
  AND j.kind = sqlc.arg(kind)
  AND j.state IN ('complete', 'cancelled')
  AND j.id <> (
      SELECT k.id FROM scan_jobs k
      WHERE k.channel_id = sqlc.arg(channel_id)
        AND k.kind = sqlc.arg(kind)
        AND k.state IN ('complete', 'cancelled')
      ORDER BY k.updated_at DESC, k.id DESC
      LIMIT 1
  );

-- name: ScanRequeueJob :exec
-- Đưa việc về hàng chờ: xoá đếm lỗi, lịch thử lại và chẩn đoán.
UPDATE scan_jobs
SET state = 'queued', attempts = 0, restarts = 0, next_attempt_at = NULL,
    error_code = '', last_error = '', recovery = '',
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid;

-- name: ScanPauseJob :exec
UPDATE scan_jobs
SET state = 'paused', updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state IN ('queued', 'running', 'backoff');

-- name: ScanCancelJob :exec
UPDATE scan_jobs
SET state = 'cancelled', next_attempt_at = NULL, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state <> 'cancelled';

-- name: ScanSetRequestedCap :exec
UPDATE scan_jobs
SET requested_cap = sqlc.arg(cap), updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid;

-- name: ScanMarkJobCapped :exec
-- Hạ việc về capped khi chỉ số đã chạm cap mới (còn hàng chờ/backoff).
UPDATE scan_jobs
SET state = 'capped', next_attempt_at = NULL, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state IN ('queued', 'backoff');

-- name: ScanRunnableJobs :many
-- Việc chạy được ngay: queued, running (worker chết để lại), backoff đã tới
-- hạn — trừ các kênh mà nơi gọi đang bận. Tập bận lọc ngay trong câu truy vấn
-- trước LIMIT: nếu lọc sau, 50 dòng bận đầu tiên sẽ che mất việc của kênh thứ
-- 51 đang rảnh và worker quay lại đúng tiền tố đó. Discovery trước, rồi việc ít
-- được đụng nhất.
SELECT * FROM scan_jobs
WHERE (state IN ('queued', 'running')
       OR (state = 'backoff' AND next_attempt_at <= sqlc.arg(now)::timestamptz))
  AND channel_id <> ALL(sqlc.arg(skip)::text[])
ORDER BY CASE kind WHEN 'discovery' THEN 0 ELSE 1 END, updated_at, id
LIMIT 50;

-- name: ScanRunnableJobForChannel :one
SELECT * FROM scan_jobs
WHERE channel_id = sqlc.arg(channel_id)
  AND (state IN ('queued', 'running')
       OR (state = 'backoff' AND next_attempt_at <= sqlc.arg(now)::timestamptz))
ORDER BY CASE kind WHEN 'discovery' THEN 0 ELSE 1 END, updated_at, id
LIMIT 1;

-- name: ScanClaimJob :one
-- Nhận việc dưới fence (fence đã kiểm trong cùng tx): chỉ việc còn runnable.
UPDATE scan_jobs
SET state = 'running', next_attempt_at = NULL,
    started_at = coalesce(started_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid
  AND (state IN ('queued', 'running')
       OR (state = 'backoff' AND next_attempt_at <= sqlc.arg(now)::timestamptz))
RETURNING *;

-- name: ScanSetJobBackoff :execrows
-- Ghi fenced: chỉ khi việc còn running (0 dòng = job_stopped).
UPDATE scan_jobs
SET state = 'backoff', attempts = sqlc.arg(attempts),
    next_attempt_at = sqlc.arg(next_at)::timestamptz,
    error_code = sqlc.arg(error_code), last_error = sqlc.arg(last_error),
    recovery = sqlc.arg(recovery), updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state = 'running';

-- name: ScanSetJobError :execrows
UPDATE scan_jobs
SET state = 'error', next_attempt_at = NULL,
    attempts = CASE WHEN sqlc.narg(attempts)::int IS NULL
                    THEN attempts ELSE sqlc.narg(attempts)::int END,
    error_code = sqlc.arg(error_code), last_error = sqlc.arg(last_error),
    recovery = sqlc.arg(recovery), updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state = 'running';

-- name: ScanSetJobCapped :execrows
UPDATE scan_jobs
SET state = 'capped', next_attempt_at = NULL, error_code = '', last_error = '',
    recovery = '', updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state = 'running';

-- name: ScanSetJobPaused :execrows
UPDATE scan_jobs
SET state = 'paused', next_attempt_at = NULL, error_code = sqlc.arg(error_code),
    last_error = sqlc.arg(last_error), recovery = sqlc.arg(recovery),
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state = 'running';

-- name: ScanCompleteJob :execrows
UPDATE scan_jobs
SET state = 'complete', cursor = NULL, remainder = NULL, next_attempt_at = NULL,
    error_code = '', last_error = '', recovery = '', reached_end = true,
    finished_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state = 'running';

-- name: ScanFinishDiscoveryJob :execrows
-- Kết thúc việc discovery trong tx head check (phải còn running).
UPDATE scan_jobs
SET state = 'complete', cursor = NULL, remainder = NULL, next_attempt_at = NULL,
    error_code = '', last_error = '', recovery = '',
    finished_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND channel_id = sqlc.arg(channel_id)
  AND kind = 'discovery' AND state = 'running';

-- name: ScanRestartJobCursor :execrows
-- Continuation hỏng: quay về đầu và phát lại tới id đã commit cuối; số lần
-- restart tăng, ordinal của discovery về 0 (đọc lại cùng thứ tự → cùng khoá).
UPDATE scan_jobs
SET cursor = NULL, remainder = NULL,
    replay_target = coalesce(replay_target, last_video_id),
    restarts = restarts + 1,
    next_ordinal = CASE WHEN kind = 'discovery' THEN 0 ELSE next_ordinal END,
    error_code = 'cursor_restart', last_error = sqlc.arg(last_error),
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND state = 'running';

-- name: ScanCommitJobPage :execrows
-- Ghi của một trang đã commit vào việc đang chạy (0 dòng = job_stopped):
-- cursor/remainder mới, replay_target tắt khi id đích đã thấy lại, đếm và
-- trạng thái lỗi được xoá.
UPDATE scan_jobs
SET cursor = sqlc.narg(cursor),
    cursor_at = CASE WHEN sqlc.narg(cursor)::text IS NULL THEN NULL
                     ELSE sqlc.arg(now)::timestamptz END,
    remainder = sqlc.narg(remainder),
    last_video_id = coalesce(sqlc.narg(last_id), last_video_id),
    replay_target = CASE WHEN sqlc.arg(seen_target)::boolean THEN NULL ELSE replay_target END,
    pages = pages + 1,
    scanned = scanned + sqlc.arg(scanned),
    added = added + sqlc.arg(added),
    reached_end = sqlc.arg(ended)::boolean,
    next_ordinal = coalesce(sqlc.narg(next_ordinal), next_ordinal),
    newest_id = coalesce(newest_id, sqlc.narg(newest_id)),
    attempts = 0, next_attempt_at = NULL,
    error_code = '', last_error = '', recovery = '',
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::uuid AND channel_id = sqlc.arg(channel_id) AND state = 'running';

-- name: ScanLinkTask :execrows
-- Nối việc vào dòng Việc nền: chỉ khi việc còn sống và kênh của nó còn tồn tại
-- (0 dòng = việc/kênh đã bị gỡ → nơi gọi phải dọn dòng vừa tạo).
UPDATE scan_jobs j SET task_id = sqlc.narg(task)::uuid
WHERE j.id = sqlc.arg(id)::uuid
  AND j.state NOT IN ('complete', 'cancelled')
  AND EXISTS (SELECT 1 FROM rival_channels c WHERE c.id = j.channel_id);

-- name: ScanClearTaskLink :exec
-- Dòng Việc nền đã kết thúc: bỏ liên kết để lần chạy sau tạo dòng mới thay vì
-- cập nhật một dòng đã bị xoá.
UPDATE scan_jobs SET task_id = NULL
WHERE id = sqlc.arg(id)::uuid AND task_id IS NOT NULL;

-- name: ScanDeleteChannelTasks :exec
-- Xoá dòng Việc nền channel_scan của một kênh khi kênh bị gỡ (cùng tx delete).
-- object_ref do activity ghi từ domain.ObjectRef nên khoá là "ID" (JSON của
-- struct Go không gắn tag), không phải "id".
DELETE FROM background_tasks
WHERE kind = 'channel_scan' AND object_ref->>'ID' = $1::text;

-- name: ScanInsertTask :one
-- Tạo dòng Việc nền ngay trong giao dịch của luồng quét (cùng khoá với kênh
-- và việc), để Remove không thể chen giữa tạo và nối.
INSERT INTO background_tasks (id, kind, agent, title, object_ref, phase, progress, eta_sec, state, action, owner_pid)
VALUES (gen_random_uuid(), sqlc.arg(kind), sqlc.arg(agent), sqlc.arg(title), sqlc.arg(object_ref)::jsonb,
        sqlc.arg(phase), sqlc.narg(progress)::jsonb, sqlc.narg(eta_sec), sqlc.arg(state),
        sqlc.arg(action)::jsonb, sqlc.arg(owner_pid))
RETURNING *;

-- ── lịch quét + thăm dò giờ đăng ────────────────────────────────────────────

-- name: ScanDueChannels :many
-- Kênh đến hạn kiểm tra video mới: head_checked_at cũ hơn rescan giờ và chưa
-- có việc discovery còn sống (cùng tập trạng thái với chỉ mục duy nhất).
SELECT c.id
FROM rival_channels c
JOIN scan_state s ON s.channel_id = c.id
WHERE s.head_checked_at < sqlc.arg(now)::timestamptz - make_interval(hours => sqlc.arg(hours)::int)
  AND NOT EXISTS (
      SELECT 1 FROM scan_jobs j
      WHERE j.channel_id = c.id AND j.kind = 'discovery'
        AND j.state IN ('queued', 'running', 'backoff', 'paused', 'capped', 'error')
  )
ORDER BY s.head_checked_at;

-- name: ScanWakeup :one
-- Một lần đọc lịch duy nhất của vòng rảnh.
-- Actionable = có việc claim được NGAY: một việc đang chạy (của chính worker
-- này), hoặc một kênh có việc đến hạn (việc chờ/backoff tới hạn, đến hạn kiểm
-- tra video mới mà chưa có run discovery sống, video chưa đọc trong mẫu 30 khi
-- hết nghỉ đọc giờ đăng) mà kênh ấy không bị fence của người khác giữ.
-- Next = hạn tương lai gần nhất: backoff, rescan, hết nghỉ đọc giờ đăng, fence
-- của kênh đang có việc đến hạn, và fence của kênh có video chờ thăm dò — nhờ
-- vậy worker ngủ đúng lúc chứ không quay vòng hỏi lại việc còn bị chặn.
-- Không có dòng scan_schedule nghĩa là chưa từng tạm nghỉ: việc đọc giờ đăng
-- vẫn tính là sẵn sàng. Nghỉ đã hết một mình không làm việc gì thành claim được.
WITH live_discovery AS (
    SELECT DISTINCT channel_id FROM scan_jobs
    WHERE kind = 'discovery'
      AND state IN ('queued', 'running', 'backoff', 'paused', 'capped', 'error')
),
upload_sample AS (
    SELECT c.id AS channel_id, r.checked
    FROM rival_channels c
    JOIN LATERAL (
        SELECT v.upload_checked_at AS checked FROM rival_videos v
        WHERE v.channel_id = c.id
        ORDER BY v.position, v.id
        LIMIT 30
    ) r ON true
),
upload_pause AS (
    SELECT coalesce(
        (SELECT upload_times_next_at FROM scan_schedule WHERE id),
        'epoch'::timestamptz
    ) AS until
),
-- Việc đến hạn ngay của từng kênh (chưa xét fence). Việc đang chạy cũng vào
-- đây: một dòng running còn lại sau khi chủ cũ hết hạn chỉ claim được khi fence
-- của kênh đã trống hoặc hết hạn — fence còn sống (của bất kỳ chủ nào) thì chỉ
-- cho hạn ngủ.
channel_work AS (
    SELECT j.channel_id
    FROM scan_jobs j
    WHERE j.state IN ('queued', 'running')
       OR (j.state = 'backoff' AND j.next_attempt_at <= sqlc.arg(now)::timestamptz)
    UNION ALL
    SELECT s.channel_id
    FROM scan_state s
    WHERE s.head_checked_at IS NOT NULL
      AND s.head_checked_at + make_interval(hours => sqlc.arg(hours)::int)
          <= sqlc.arg(now)::timestamptz
      AND NOT EXISTS (SELECT 1 FROM live_discovery ld WHERE ld.channel_id = s.channel_id)
    UNION ALL
    SELECT us.channel_id
    FROM upload_sample us, upload_pause up
    WHERE us.checked IS NULL
      AND up.until <= sqlc.arg(now)::timestamptz
),
ready AS (
    SELECT TRUE
    FROM channel_work cw
    WHERE NOT EXISTS (
        SELECT 1 FROM scan_state s
        WHERE s.channel_id = cw.channel_id
          AND s.lease_until IS NOT NULL
          AND s.lease_until > sqlc.arg(now)::timestamptz
    )
),
wait AS (
    SELECT j.next_attempt_at AS at
    FROM scan_jobs j
    WHERE j.state = 'backoff'
      AND j.next_attempt_at > sqlc.arg(now)::timestamptz
    UNION ALL
    SELECT s.head_checked_at + make_interval(hours => sqlc.arg(hours)::int)
    FROM scan_state s
    WHERE s.head_checked_at IS NOT NULL
      AND s.head_checked_at + make_interval(hours => sqlc.arg(hours)::int)
          > sqlc.arg(now)::timestamptz
      AND NOT EXISTS (SELECT 1 FROM live_discovery ld WHERE ld.channel_id = s.channel_id)
    UNION ALL
    SELECT up.until
    FROM upload_pause up
    WHERE up.until > sqlc.arg(now)::timestamptz
      AND EXISTS (SELECT 1 FROM upload_sample us WHERE us.checked IS NULL)
    UNION ALL
    SELECT s.lease_until
    FROM scan_state s
    WHERE s.lease_until IS NOT NULL
      AND s.lease_until > sqlc.arg(now)::timestamptz
      AND (EXISTS (SELECT 1 FROM channel_work cw WHERE cw.channel_id = s.channel_id)
           OR EXISTS (
               SELECT 1 FROM scan_jobs j
               WHERE j.channel_id = s.channel_id
                 AND j.state IN ('queued', 'running', 'backoff', 'paused', 'capped', 'error')
           ))
)
SELECT
    EXISTS (SELECT 1 FROM ready) AS actionable,
    (SELECT min(w.at) FROM wait w WHERE w.at > sqlc.arg(now)::timestamptz)::timestamptz AS next_at;

-- name: ScanNextUploadCandidate :one
-- Video kế tiếp cần đọc giờ đăng chính xác: mẫu 30 video mới nhất mỗi kênh
-- (chọn mẫu trước), rồi mới lọc hàng chưa đọc — quét hết mẫu là hết việc,
-- không tràn xuống lịch sử. Kênh quét gần nhất trước. Trả hàng rỗng khi đang
-- tạm nghỉ.
SELECT r.id, r.channel_id
FROM rival_channels c
JOIN LATERAL (
    SELECT v.id, v.channel_id, v.position, v.upload_checked_at
    FROM rival_videos v
    WHERE v.channel_id = c.id
    ORDER BY v.position, v.id
    LIMIT 30
) r ON r.upload_checked_at IS NULL
WHERE coalesce((SELECT upload_times_next_at FROM scan_schedule WHERE id = true), 'epoch'::timestamptz)
      <= sqlc.arg(now)::timestamptz
  AND NOT EXISTS (
      SELECT 1 FROM scan_state s
      WHERE s.channel_id = c.id
        AND s.lease_until IS NOT NULL
        AND s.lease_until > sqlc.arg(now)::timestamptz
  )
ORDER BY c.scanned_at DESC NULLS LAST, r.position, r.id
LIMIT 1;

-- name: ScanRecordUploadTime :execrows
-- Chốt một lần đọc giờ đăng của video thuộc đúng kênh đang giữ fence
-- (uploaded_at null cũng là đã đọc xong).
UPDATE rival_videos
SET uploaded_at = sqlc.narg(uploaded), upload_checked_at = sqlc.arg(now)::timestamptz,
    has_captions = sqlc.arg(has_captions)::boolean
WHERE id = sqlc.arg(id) AND channel_id = sqlc.arg(channel_id);

-- name: ScanPauseUploads :exec
-- Tạm nghỉ thăm dò giờ đăng tới mốc cho trước (max-upsert: chỉ kéo dài).
INSERT INTO scan_schedule (id, upload_times_next_at)
VALUES (true, sqlc.arg(until)::timestamptz)
ON CONFLICT (id) DO UPDATE SET
    upload_times_next_at = greatest(scan_schedule.upload_times_next_at, EXCLUDED.upload_times_next_at);

-- name: ScanBumpUploadFailures :one
-- Đếm lỗi thăm dò liên tiếp (bền qua restart); reset về 0 sau khi đạt ngưỡng.
INSERT INTO scan_schedule (id, upload_time_failures) VALUES (true, 1)
ON CONFLICT (id) DO UPDATE SET upload_time_failures = scan_schedule.upload_time_failures + 1
RETURNING upload_time_failures;

-- name: ScanResetUploadFailures :exec
UPDATE scan_schedule SET upload_time_failures = 0 WHERE id = true;

-- ── lease worker trên platform_state('youtube') ────────────────────────────

-- name: ScanAcquireWorkerLease :one
-- Lease của luồng quét toàn cục: scanner_owner làm token (CAS một câu). Trả
-- chính owner vừa ghi — gia hạn/nhả so với cột này.
INSERT INTO platform_state (platform, scanner_owner, scanner_lease_until)
VALUES ('youtube', sqlc.arg(owner), sqlc.arg(until)::timestamptz)
ON CONFLICT (platform) DO UPDATE SET
    scanner_owner = EXCLUDED.scanner_owner,
    scanner_lease_until = EXCLUDED.scanner_lease_until
WHERE platform_state.scanner_lease_until IS NULL
   OR platform_state.scanner_lease_until <= sqlc.arg(now)::timestamptz
RETURNING scanner_owner;

-- name: ScanRenewWorkerLease :execrows
UPDATE platform_state
SET scanner_lease_until = sqlc.arg(until)::timestamptz
WHERE platform = 'youtube' AND scanner_owner = sqlc.arg(owner);

-- name: ScanReleaseWorkerLease :exec
UPDATE platform_state
SET scanner_owner = NULL, scanner_lease_until = NULL
WHERE platform = 'youtube' AND scanner_owner = sqlc.arg(owner);

-- name: ScanWorkerLeaseLive :one
SELECT EXISTS (
    SELECT 1 FROM platform_state
    WHERE platform = 'youtube' AND scanner_lease_until > sqlc.arg(now)::timestamptz
) AS live;
