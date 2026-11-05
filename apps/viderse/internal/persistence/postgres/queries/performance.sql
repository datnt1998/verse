-- Metadata mark-posted: facts chụp lúc bắt đầu theo dõi và link YouTube của
-- video đã đăng. V38 đã bỏ collector/số liệu sau đăng; bảng clone_post_stats
-- và performance_insights_applied đã drop, các cột read_* còn lại là legacy
-- read-only (chờ migration retention riêng).

-- name: EnsurePostFacts :exec
-- Tiêu đề chỉ được điền khi lần chụp trước chưa đọc được.
INSERT INTO clone_post_facts (job_id, title, duration_sec)
VALUES ($1, $2, $3)
ON CONFLICT (job_id) DO UPDATE SET title = EXCLUDED.title, updated_at = now()
WHERE clone_post_facts.title = '' AND EXCLUDED.title <> '';

-- name: SetPostYoutube :execrows
-- Link khác: xoá thời điểm đăng đã đọc và lịch thử lại (không còn số liệu đọc).
UPDATE clone_post_facts
SET youtube_id = $2, published_at = NULL, read_attempts = 0, next_read_at = NULL, updated_at = now()
WHERE job_id = $1 AND youtube_id IS DISTINCT FROM $2;

-- name: GetPostYoutubeID :one
SELECT youtube_id FROM clone_post_facts WHERE job_id = $1;


