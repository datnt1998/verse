-- name: ListPlatformStates :many
SELECT platform, next_request_at, paused_until, pause_reason, failure_count,
       scanner_owner, scanner_lease_until, last_error_code, last_error_at, last_error_job_id
FROM platform_state
ORDER BY platform;

-- name: GetPlatformState :one
SELECT platform, next_request_at, paused_until, pause_reason, failure_count,
       scanner_owner, scanner_lease_until, last_error_code, last_error_at, last_error_job_id
FROM platform_state
WHERE platform = $1;

-- name: EnsurePlatformState :exec
INSERT INTO platform_state (platform) VALUES ($1) ON CONFLICT (platform) DO NOTHING;

-- name: ClearExpiredPlatformPause :exec
-- Dọn giờ chờ đã hết hạn (đếm chặn giữ nguyên). Chỉ chạy khi vừa đọc thấy hạn
-- đã qua, nên đọc giờ chờ lúc bình thường không ghi dòng nào; điều kiện
-- paused_until <= now giữ nguyên lần chặn mới ghi chen giữa đọc và dọn.
UPDATE platform_state
SET paused_until = NULL, pause_reason = NULL
WHERE platform = sqlc.arg(platform) AND paused_until <= sqlc.arg(now)::timestamptz;

-- name: RecordPlatformBlocked :one
-- Lần bị chặn thứ n: đếm tăng một, giờ chờ = now + max(15·2^(n-1) phút trần
-- 120, Retry-After của server). Nhánh insert là lần chặn đầu (đếm = 1).
INSERT INTO platform_state (platform, paused_until, pause_reason, failure_count)
VALUES (
    sqlc.arg(platform),
    sqlc.arg(now)::timestamptz
        + greatest(15 * interval '1 minute', sqlc.arg(retry_after_ms)::double precision * interval '1 millisecond'),
    sqlc.arg(reason),
    1
)
ON CONFLICT (platform) DO UPDATE SET
    paused_until = sqlc.arg(now)::timestamptz + greatest(
        least(120.0, 15 * power(2, least(platform_state.failure_count, 3))) * interval '1 minute',
        sqlc.arg(retry_after_ms)::double precision * interval '1 millisecond'),
    pause_reason = sqlc.arg(reason),
    failure_count = platform_state.failure_count + 1
RETURNING paused_until, pause_reason, failure_count;

-- name: ClearPlatformPause :exec
UPDATE platform_state
SET paused_until = NULL, pause_reason = NULL, failure_count = 0
WHERE platform = $1;

-- name: RecordPlatformError :exec
INSERT INTO platform_state (platform, last_error_code, last_error_at, last_error_job_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (platform) DO UPDATE SET
    last_error_code = $2, last_error_at = $3, last_error_job_id = $4;

-- name: ClearPlatformError :exec
UPDATE platform_state
SET last_error_code = '', last_error_at = NULL, last_error_job_id = NULL
WHERE platform = $1;

-- name: ReservePlatformSlot :one
-- Đặt chỗ gọi kế tiếp: start = max(now, next cũ); next mới = start + spacing.
-- Trả start để caller ngủ đúng khoảng. Bilibili gọi với spacing ~5 giây.
INSERT INTO platform_state (platform, next_request_at)
VALUES (
    sqlc.arg(platform),
    sqlc.arg(now)::timestamptz + sqlc.arg(spacing_ms)::double precision * interval '1 millisecond'
)
ON CONFLICT (platform) DO UPDATE SET
    next_request_at = greatest(platform_state.next_request_at, sqlc.arg(now)::timestamptz)
                    + sqlc.arg(spacing_ms)::double precision * interval '1 millisecond'
RETURNING (next_request_at - sqlc.arg(spacing_ms)::double precision * interval '1 millisecond')::timestamptz AS start_at;

-- name: AcquirePlatformScanner :one
-- Giữ lease quét: chỉ thành công khi chưa có lease hoặc lease đã hết hạn
-- (không có dòng trả về = một scanner khác còn hạn).
INSERT INTO platform_state (platform, scanner_owner, scanner_lease_until)
VALUES (sqlc.arg(platform), sqlc.arg(owner), sqlc.arg(until)::timestamptz)
ON CONFLICT (platform) DO UPDATE SET
    scanner_owner = sqlc.arg(owner), scanner_lease_until = sqlc.arg(until)::timestamptz
WHERE platform_state.scanner_lease_until IS NULL
   OR platform_state.scanner_lease_until <= sqlc.arg(now)::timestamptz
RETURNING true AS acquired;

-- name: ReleasePlatformScanner :exec
UPDATE platform_state
SET scanner_owner = NULL, scanner_lease_until = NULL
WHERE platform = $1 AND scanner_owner = $2;
