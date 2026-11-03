-- Clone pipeline: job/step/artifact rows + các thao tác fenced của runner.
-- Queue scheduling sống ở queries/queue.sql (P05QueueDev); file này chỉ chứa
-- truy vấn job/step/artifact và đọc tối thiểu cho read-model.

-- name: GetCloneJob :one
SELECT * FROM clone_jobs WHERE id = $1;

-- name: GetCloneJobForUpdate :one
SELECT * FROM clone_jobs WHERE id = $1 FOR UPDATE;

-- name: GetCloneJobBySourceKey :one
SELECT * FROM clone_jobs WHERE source_key = $1 AND superseded_at IS NULL;

-- name: InsertCloneJob :one
INSERT INTO clone_jobs (
    id, channel_id, source_key, source_snapshot, template_id,
    template_snapshot, model_refs, no_images, generation, priority, origin,
    status, stage_group, fence, version, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11,
    'pending', 'working', 0, 1, $12::timestamptz, $12::timestamptz
) RETURNING *;

-- name: SupersedeCloneJob :exec
UPDATE clone_jobs SET superseded_at = $2::timestamptz, status = 'cancelled', paused_at = NULL,
    version = version + 1, updated_at = $2::timestamptz
WHERE id = $1 AND superseded_at IS NULL;

-- name: CancelOpenSteps :exec
-- Step chưa xong của generation bị thay thế phải đóng lại để không nhìn như
-- việc còn chạy được.
UPDATE clone_steps SET status = 'cancelled', updated_at = $2::timestamptz
WHERE job_id = $1 AND status IN ('pending', 'running', 'waiting', 'stale');

-- name: InsertCloneStep :exec
INSERT INTO clone_steps (job_id, step, status, updated_at)
VALUES ($1, $2, 'pending', $3::timestamptz);

-- name: ListCloneSteps :many
SELECT * FROM clone_steps WHERE job_id = $1;

-- name: GetCloneStep :one
SELECT * FROM clone_steps WHERE job_id = $1 AND step = $2;

-- name: ListCloneStepsByJobs :many
SELECT * FROM clone_steps WHERE job_id = ANY($1::uuid[]);

-- name: BumpJobFence :one
UPDATE clone_jobs SET fence = fence + 1 WHERE id = $1 RETURNING fence;

-- name: ClaimCloneStep :one
-- CAS theo trạng thái quan sát được: pending/stale luôn claim được; waiting
-- chỉ claim lại khi retryAt đã qua (awaiting_review không retryAt nên bị loại
-- tự nhiên).
UPDATE clone_steps SET status = 'running', attempt = attempt + 1,
    fence = $3, started_at = $4::timestamptz, finished_at = NULL, error = NULL,
    progress_done = 0, progress_total = NULL, progress_phase = '',
    updated_at = $4::timestamptz
WHERE job_id = $1 AND step = $2
  AND (status IN ('pending', 'stale')
       OR (status = 'waiting' AND error->>'retryAt' IS NOT NULL
           AND (error->>'retryAt')::timestamptz <= $4::timestamptz))
RETURNING *;

-- name: WriteStepProgress :execrows
UPDATE clone_steps SET progress_done = $4, progress_total = $5,
    progress_phase = $6, updated_at = $7::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $8::int;

-- name: WriteStepCheckpoint :execrows
UPDATE clone_steps SET checkpoint = $4::jsonb, updated_at = $5::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $6::int;

-- name: WriteStepProcessGroup :execrows
UPDATE clone_steps SET process_group = $4, updated_at = $5::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $6::int;

-- name: WriteStepFingerprint :execrows
UPDATE clone_steps SET input_fingerprint = $4, updated_at = $5::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $6::int;

-- name: ApplyStepSuccess :execrows
UPDATE clone_steps SET status = 'succeeded', output_manifest = $4::jsonb,
    error = NULL, finished_at = $5::timestamptz,
    progress_done = 0, progress_total = NULL, progress_phase = '',
    checkpoint = NULL, updated_at = $5::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $6::int;

-- name: ApplyStepWaiting :execrows
-- Đỗ step (quota/tool_missing/awaiting_review/interrupted): output_manifest
-- vẫn được giữ khi step publish được phần việc.
UPDATE clone_steps SET status = 'waiting', output_manifest = $4::jsonb,
    error = $7::jsonb, finished_at = $5::timestamptz, updated_at = $5::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $6::int;

-- name: ReportStepFailure :execrows
UPDATE clone_steps SET status = $4, error = $7::jsonb,
    finished_at = $5::timestamptz, updated_at = $5::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $6::int;

-- name: ReportStepCancelled :execrows
UPDATE clone_steps SET status = 'cancelled', finished_at = $4::timestamptz,
    updated_at = $4::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $5::int;

-- name: ParkStepPaused :execrows
-- Tạm dừng: step về pending giữ checkpoint (không đụng attempt/fence) để bản
-- chạy tiếp claim lại từ đúng điểm.
UPDATE clone_steps SET status = 'pending', finished_at = NULL,
    progress_phase = '', updated_at = $4::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'running'
  AND fence = $3::bigint AND attempt = $5::int;

-- name: RequeueParkedStep :execrows
-- "Thử lại ngay": step đỗ waiting (không phải cổng duyệt) về pending.
UPDATE clone_steps SET status = 'pending', error = NULL, updated_at = $3::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'waiting'
  AND (error IS NULL OR error->>'code' IS DISTINCT FROM 'awaiting_review');

-- name: InvalidateStepRange :execrows
-- Sửa job: step + dependents về stale (giữ output) hoặc pending (chưa có
-- output); running giữ nguyên — fence của attempt đang chạy tự quyết.
UPDATE clone_steps SET status = CASE WHEN status IN ('succeeded','stale') THEN 'stale' ELSE 'pending' END,
    input_fingerprint = CASE WHEN status IN ('succeeded','stale') THEN input_fingerprint ELSE '' END,
    error = NULL, finished_at = NULL, updated_at = $3::timestamptz
WHERE job_id = $1 AND step = ANY($2::text[]) AND status NOT IN ('running', 'cancelled');

-- name: MarkStepStale :execrows
UPDATE clone_steps SET status = 'stale', updated_at = $4::timestamptz
WHERE job_id = $1 AND step = $2 AND status = 'succeeded'
  AND input_fingerprint IS DISTINCT FROM $3;

-- name: ResumeSteps :execrows
-- Chạy tiếp: step failed và waiting (trừ cổng duyệt) về pending; step
-- cancelled của một job bị huỷ cũng mở lại khi chủ nhân chạy tiếp job đó.
UPDATE clone_steps SET status = 'pending', error = NULL, finished_at = NULL,
    updated_at = $2::timestamptz
WHERE job_id = $1
  AND (status = 'failed'
       OR (status = 'waiting' AND (error IS NULL OR error->>'code' IS DISTINCT FROM 'awaiting_review')));

-- name: ResumeCancelledSteps :execrows
UPDATE clone_steps SET status = 'pending', error = NULL, finished_at = NULL,
    updated_at = $2::timestamptz
WHERE job_id = $1 AND status = 'cancelled';

-- name: InsertCloneArtifact :one
INSERT INTO clone_artifacts (id, job_id, step, path, sha256, bytes, content_type, filename)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (job_id, id) DO UPDATE SET
    path = excluded.path, sha256 = excluded.sha256, bytes = excluded.bytes,
    content_type = excluded.content_type, filename = excluded.filename
RETURNING *;

-- name: DeleteCloneArtifacts :exec
DELETE FROM clone_artifacts WHERE job_id = $1 AND id = ANY($2::text[]);

-- name: DeleteCloneArtifactsByPath :exec
DELETE FROM clone_artifacts WHERE job_id = $1 AND path = ANY($2::text[]);

-- name: ListCloneArtifacts :many
SELECT * FROM clone_artifacts WHERE job_id = $1 ORDER BY created_at, id;

-- name: GetCloneArtifact :one
SELECT * FROM clone_artifacts WHERE job_id = $1 AND id = $2;

-- name: SetJobDerived :exec
-- Mọi write của repo gọi cuối tx: version bump + updated_at + stage_group +
-- active_step (read model của Bàn dựng, cùng transaction — không trigger).
UPDATE clone_jobs SET stage_group = $3, active_step = $4,
    version = version + 1, updated_at = $2::timestamptz
WHERE id = $1;

-- name: SetJobStatus :exec
UPDATE clone_jobs SET status = $2, version = version + 1,
    updated_at = $3::timestamptz
WHERE id = $1;

-- name: ClaimJobLease :one
UPDATE clone_jobs SET lease_owner = $2, lease_expires_at = $3::timestamptz,
    version = version + 1, updated_at = $4::timestamptz
WHERE id = $1 AND superseded_at IS NULL
  AND (lease_expires_at IS NULL OR lease_expires_at <= $4::timestamptz)
RETURNING *;

-- name: HeartbeatJobLease :execrows
UPDATE clone_jobs SET lease_expires_at = $3::timestamptz,
    updated_at = $4::timestamptz
WHERE id = $1 AND lease_owner = $2 AND lease_expires_at > $4::timestamptz;

-- name: ReleaseJobLease :exec
UPDATE clone_jobs SET lease_owner = NULL, lease_expires_at = NULL,
    updated_at = $3::timestamptz
WHERE id = $1 AND lease_owner = $2;

-- name: RequestJobCancel :exec
UPDATE clone_jobs SET cancel_requested_at = $2::timestamptz,
    version = version + 1, updated_at = $2::timestamptz
WHERE id = $1 AND status NOT IN ('succeeded', 'failed', 'cancelled');

-- name: SetJobPaused :exec
UPDATE clone_jobs SET paused_at = $2::timestamptz, version = version + 1,
    updated_at = now()
WHERE id = $1;

-- name: MarkJobPosted :exec
UPDATE clone_jobs SET posted_at = $2::timestamptz, version = version + 1,
    updated_at = now()
WHERE id = $1;

-- name: DeleteCloneJob :exec
DELETE FROM clone_jobs WHERE id = $1;

-- name: ListClaimableCloneJobs :many
-- Job worker có thể nhận: đang chờ, không tạm dừng, chưa thay thế, và lease
-- (nếu có) đã chết.
SELECT * FROM clone_jobs
WHERE status = 'pending' AND paused_at IS NULL AND superseded_at IS NULL
  AND (lease_expires_at IS NULL OR lease_expires_at <= now())
ORDER BY created_at, id
LIMIT $1;

-- name: ListInterruptedCloneJobs :many
-- Job có step đang 'running' mà lease đã chết: worker chết giữa chừng.
SELECT j.* FROM clone_jobs j
WHERE j.superseded_at IS NULL
  AND EXISTS (SELECT 1 FROM clone_steps s WHERE s.job_id = j.id AND s.status = 'running')
  AND (j.lease_expires_at IS NULL OR j.lease_expires_at <= now())
ORDER BY j.created_at
LIMIT $1;

-- name: ListBoardJobs :many
-- Bàn dựng: job chưa thay thế của một kênh (NULL = tất cả), chỉ số
-- (channel_id, stage_group, updated_at).
SELECT * FROM clone_jobs
WHERE superseded_at IS NULL
  AND ($1::uuid IS NULL OR channel_id = $1)
ORDER BY stage_group, updated_at DESC, id
LIMIT $2::int;

-- name: CountBoardGroups :many
SELECT channel_id, stage_group, count(*)::bigint AS n
FROM clone_jobs
WHERE superseded_at IS NULL
GROUP BY channel_id, stage_group;

-- name: CountBoardGroupsToday :many
-- "Xong hôm nay": job ready/posted cập nhật sau nửa đêm địa phương.
SELECT channel_id, count(*)::bigint AS n
FROM clone_jobs
WHERE superseded_at IS NULL
  AND stage_group IN ('ready', 'posted')
  AND updated_at >= $1::timestamptz
GROUP BY channel_id;

-- name: CountChannelJobsToday :one
-- Nhịp kênh: chỉ đếm job xuất phát từ hàng chờ (bulk/auto) từ nửa đêm địa phương.
SELECT count(*)::bigint AS n FROM clone_jobs
WHERE channel_id = $1 AND origin IN ('bulk', 'auto') AND created_at >= $2::timestamptz;

-- name: ListFinishedCloneJobs :many
-- Job đã xong (cho dọn nguồn và báo cáo): succeeded, không thay thế.
SELECT * FROM clone_jobs
WHERE superseded_at IS NULL AND status = 'succeeded'
  AND (lease_expires_at IS NULL OR lease_expires_at <= now())
ORDER BY updated_at DESC
LIMIT NULLIF($1::int, 0);
