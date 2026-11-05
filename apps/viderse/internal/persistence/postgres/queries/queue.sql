-- Hàng chờ clone (§3): mục hàng chờ, lane theo kênh, nhịp đếm từ mục batch
-- đã chạy trong ngày, dữ liệu ETA (bước đã đo), đối thủ liên kết cho Tự lấy.

-- name: InsertQueueItem :exec
INSERT INTO clone_queue (id, channel_id, source_video_id, source_key, source_snapshot,
                         priority, origin, position, no_images, template_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetQueueItem :one
SELECT * FROM clone_queue WHERE id = $1;

-- name: ListQueuedItems :many
-- Chọn manual toàn cục trước workset cap; auto giữ thứ tự trong lane.
SELECT * FROM clone_queue WHERE state = 'queued' AND removed_at IS NULL
ORDER BY CASE WHEN priority = 'pick' THEN 0 ELSE 1 END,
         CASE WHEN priority = 'pick' THEN position END,
         channel_id, position, id
LIMIT 2000;

-- name: QueuePositionBounds :one
-- Aggregate trên toàn eligible set, không lấy bounds từ workset LIMIT.
SELECT coalesce(min(position) FILTER (WHERE priority = 'pick'), 0)::double precision AS manual_min,
       coalesce(max(position) FILTER (WHERE priority = 'batch'), 0)::double precision AS auto_max
FROM clone_queue WHERE state = 'queued' AND removed_at IS NULL;

-- name: ListLaneItems :many
SELECT * FROM clone_queue
WHERE channel_id = $1 AND state = 'queued' AND removed_at IS NULL
ORDER BY position, id;

-- name: UpdateQueuePosition :exec
UPDATE clone_queue SET position = $2 WHERE id = $1 AND state = 'queued' AND removed_at IS NULL;

-- name: RemoveQueueItem :execrows
UPDATE clone_queue SET removed_at = $2
WHERE id = $1 AND state = 'queued' AND removed_at IS NULL;

-- name: RestoreQueueItem :execrows
UPDATE clone_queue q SET removed_at = NULL
WHERE q.id = $1 AND q.state = 'queued' AND q.job_id IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM clone_jobs j
    WHERE j.source_key = q.source_key AND j.superseded_at IS NULL AND j.id IS DISTINCT FROM q.restart_of)
  AND NOT EXISTS (
    SELECT 1 FROM clone_queue active
    WHERE active.source_key = q.source_key AND active.id <> q.id
      AND active.state IN ('queued', 'started') AND active.removed_at IS NULL AND active.job_id IS DISTINCT FROM q.restart_of);

-- name: PruneQueueHistory :exec
-- Lịch sử giữ 30 ngày; mục đã bỏ giữ 24 giờ kể từ removed_at.
DELETE FROM clone_queue
WHERE (removed_at IS NULL AND state IN ('started', 'skipped') AND created_at < sqlc.arg(history_cutoff))
   OR removed_at < sqlc.arg(removed_cutoff);

-- name: MarkQueueItemStarted :execrows
-- Kiểm trạng thái trong câu lệnh: mục đã đổi tay chỗ khác thì execrows=0.
UPDATE clone_queue SET state = 'started', job_id = $2, started_at = $3, reason = NULL
WHERE id = $1 AND state = 'queued' AND removed_at IS NULL;

-- name: MarkQueueItemSkipped :exec
UPDATE clone_queue SET state = 'skipped', reason = $2
WHERE id = $1 AND state = 'queued' AND removed_at IS NULL;

-- name: ActiveSourceKeys :many
-- Khoá job đang hoạt động (chưa bị thay thế) để khử trùng khi xếp hàng.
SELECT source_key FROM clone_jobs WHERE superseded_at IS NULL;

-- name: QueuedSourceKeys :many
SELECT source_key FROM clone_queue WHERE state = 'queued' AND removed_at IS NULL;

-- name: CountQueuedByChannel :many
SELECT channel_id, count(*)::bigint AS queued
FROM clone_queue WHERE state = 'queued' AND removed_at IS NULL GROUP BY channel_id;

-- name: CountTotalQueued :one
SELECT count(*)::bigint FROM clone_queue WHERE state = 'queued' AND removed_at IS NULL;

-- name: LaneLastStarts :many
-- Lượt cuối của mỗi kênh trong vòng luân phiên: giờ bắt đầu mục gần nhất và
-- job gần nhất nó tạo ra (chống hoà bằng channel_id ở tầng nghiệp vụ).
SELECT q.channel_id, max(q.started_at)::timestamptz AS last_start,
       max(j.created_at)::timestamptz AS last_sequence
FROM clone_queue q
LEFT JOIN clone_jobs j ON j.id = q.job_id
WHERE q.state = 'started' AND q.started_at IS NOT NULL AND q.removed_at IS NULL
GROUP BY q.channel_id;

-- name: PaceUsedToday :many
-- Nhịp hôm nay: chỉ mục `batch` đã bắt đầu từ 00:00 giờ máy ($1 = local midnight).
SELECT channel_id, count(*)::bigint AS used
FROM clone_queue
WHERE state = 'started' AND priority = 'batch' AND started_at >= $1 AND removed_at IS NULL
GROUP BY channel_id;

-- name: CountActiveJobs :one
-- Job đang giữ luồng: pending/running; waiting (kể cả quota) không giữ luồng.
SELECT count(*)::int
FROM clone_jobs
WHERE status IN ('pending', 'running')
  AND (paused_at IS NULL OR status = 'running')
  AND NOT EXISTS (
    SELECT 1 FROM clone_steps s
    WHERE s.job_id = clone_jobs.id AND s.status = 'waiting'
      AND s.error->>'code' = 'awaiting_review');

-- name: ListActiveJobs :many
SELECT * FROM clone_jobs
WHERE status IN ('pending', 'running')
  AND (paused_at IS NULL OR status = 'running')
  AND NOT EXISTS (
    SELECT 1 FROM clone_steps s
    WHERE s.job_id = clone_jobs.id AND s.status = 'waiting'
      AND s.error->>'code' = 'awaiting_review')
ORDER BY created_at;

-- name: ListJobSteps :many
SELECT * FROM clone_steps WHERE job_id = $1
ORDER BY step;

-- name: ListRecentStepDurations :many
-- 400 lần bước thành công gần nhất; tầng nghiệp vụ giữ 20 lần đầu mỗi bước.
SELECT s.step, extract(epoch FROM (s.finished_at - s.started_at))::double precision AS elapsed_seconds,
       j.source_snapshot, s.output_manifest, s.finished_at
FROM clone_steps s
JOIN clone_jobs j ON j.id = s.job_id
WHERE s.status = 'succeeded' AND s.started_at IS NOT NULL AND s.finished_at IS NOT NULL
  AND s.finished_at > s.started_at
ORDER BY s.finished_at DESC
LIMIT 400;

-- name: FindJobsBySourceKeys :many
-- Job đang hoạt động theo khoá nguồn (kế hoạch: video trùng). Thứ tự created_at
-- để bản mới nhất đứng cuối.
SELECT * FROM clone_jobs
WHERE source_key = ANY($1::text[]) AND superseded_at IS NULL
ORDER BY created_at;

-- name: FindQueueRowsBySourceKeys :many
SELECT * FROM clone_queue
WHERE source_key = ANY($1::text[]) AND state IN ('queued', 'started') AND removed_at IS NULL;

-- name: ListVideosByIDs :many
SELECT * FROM rival_videos WHERE id = ANY($1::text[]);

-- name: ListChannelsByIDs :many
SELECT * FROM rival_channels WHERE id = ANY($1::text[]);

-- name: ListSourcesWithChannels :many
-- Đối thủ của một kênh kèm bản ghi kênh đối thủ (quét Tự lấy).
SELECT s.owner_channel_id, s.rival_id, s.linked_at, s.auto_baseline_at, s.min_duration_sec,
       r.id, r.handle, r.title, r.scanned_at
FROM owner_channel_sources s
JOIN rival_channels r ON r.id = s.rival_id
WHERE s.owner_channel_id = $1
ORDER BY s.linked_at, s.rival_id;

-- name: SetSourceBaseline :exec
UPDATE owner_channel_sources SET auto_baseline_at = $3
WHERE owner_channel_id = $1 AND rival_id = $2;

-- name: LastAutoQueuedChannel :one
-- Neo luân phiên đối thủ: kênh đối thủ của mục `auto` mới nhất.
SELECT v.channel_id FROM clone_queue q
JOIN rival_videos v ON v.id = q.source_video_id
WHERE q.channel_id = $1 AND q.origin = 'auto' AND q.removed_at IS NULL
ORDER BY q.created_at DESC, q.position DESC, q.id DESC
LIMIT 1;

-- name: ListEligibleVideos :many
-- Video ứng viên Tự lấy của một đối thủ: thấy sau baseline, không phải
-- video nền lịch sử/baseline, không có job sống, không còn trong hàng.
-- $4 = số giây tối thiểu của bộ lọc theo đối thủ (0 = không lọc); video chưa
-- đọc được thời lượng không qua bộ lọc đang bật.
SELECT v.id FROM rival_videos v
WHERE v.channel_id = $1
  AND v.origin = 'discovery'
  AND v.first_seen_at > $2
  AND (v.published_at IS NULL OR v.published_at > $2)
  AND ($4::int <= 0 OR v.duration_sec >= $4::int)
  AND NOT EXISTS (
    SELECT 1 FROM clone_jobs j
    WHERE j.source_key = ('channel:' || $3::text || ':video:' || v.id)
      AND j.superseded_at IS NULL)
  AND NOT EXISTS (
    SELECT 1 FROM clone_queue q
    WHERE q.source_key = ('channel:' || $3::text || ':video:' || v.id)
      AND q.state IN ('queued', 'started') AND q.removed_at IS NULL)
ORDER BY v.position, v.id
LIMIT 1000;

-- name: ListDoneTodayJobs :many
-- Xong hôm nay: job ở nhóm ready/posted/review cập nhật sau 00:00 ($1).
SELECT id, channel_id, source_snapshot, stage_group, updated_at
FROM clone_jobs
WHERE superseded_at IS NULL
  AND stage_group IN ('ready', 'posted', 'review')
  AND updated_at >= $1
ORDER BY updated_at DESC;

-- name: ListJobStageCounts :many
-- Bộ đếm badge: job chưa bị thay thế, chưa huỷ, gom theo (kênh, stage_group).
SELECT channel_id, stage_group, count(*)::bigint AS n
FROM clone_jobs
WHERE superseded_at IS NULL AND status <> 'cancelled'
GROUP BY channel_id, stage_group;

-- name: ListQueueCountsByChannel :many
SELECT channel_id, count(*)::bigint AS n
FROM clone_queue WHERE state = 'queued' AND removed_at IS NULL
GROUP BY channel_id;

-- name: ListDoneTodayCounts :many
-- doneToday theo kênh: job ready/posted cập nhật sau 00:00 ($1).
SELECT channel_id, count(*)::bigint AS n
FROM clone_jobs
WHERE superseded_at IS NULL AND stage_group IN ('ready', 'posted') AND updated_at >= $1
GROUP BY channel_id;

-- name: CountChargedImagesToday :many
-- Ảnh trả phí đã tính hôm nay theo provider ($1 = local midnight).
SELECT provider, count(*)::bigint AS used
FROM image_receipts
WHERE charged AND started_at >= $1
GROUP BY provider;

-- name: TryAdvisoryLock :one
-- Khoá drain/Tự lấy mức session (giữ trên một *pgxpool.Conn).
SELECT pg_try_advisory_lock($1) AS acquired;

-- name: UnlockAdvisory :exec
SELECT pg_advisory_unlock($1);

-- name: LockQueueOwner :one
SELECT id FROM owner_channels WHERE id = $1 FOR UPDATE;

-- name: PromoteQueuePick :exec
UPDATE clone_queue SET priority = 'pick', position = $2
WHERE id = $1 AND state = 'queued' AND removed_at IS NULL;
