-- Số liệu sau đăng và nhận xét (§21).

-- name: EnsurePostFacts :exec
-- Tiêu đề chỉ được điền khi lần chụp trước chưa đọc được.
INSERT INTO clone_post_facts (job_id, title, duration_sec)
VALUES ($1, $2, $3)
ON CONFLICT (job_id) DO UPDATE SET title = EXCLUDED.title, updated_at = now()
WHERE clone_post_facts.title = '' AND EXCLUDED.title <> '';

-- name: SetPostYoutube :execrows
-- Link khác: xoá thời điểm đăng đã đọc và lịch thử lại; caller xoá số cũ khi có dòng đổi.
UPDATE clone_post_facts
SET youtube_id = $2, published_at = NULL, read_attempts = 0, next_read_at = NULL, updated_at = now()
WHERE job_id = $1 AND youtube_id IS DISTINCT FROM $2;

-- name: GetPostYoutubeID :one
SELECT youtube_id FROM clone_post_facts WHERE job_id = $1;

-- name: DeletePostStats :exec
DELETE FROM clone_post_stats WHERE job_id = $1;

-- name: SetPostPublished :exec
UPDATE clone_post_facts SET published_at = $2, updated_at = now() WHERE job_id = $1;

-- name: RecordPostReadFailure :exec
UPDATE clone_post_facts
SET read_attempts = read_attempts + 1, next_read_at = $2, updated_at = now()
WHERE job_id = $1;

-- name: ClearPostReadFailures :exec
UPDATE clone_post_facts SET read_attempts = 0, next_read_at = NULL, updated_at = now()
WHERE job_id = $1;

-- name: InsertPostStat :exec
INSERT INTO clone_post_stats (job_id, milestone, views, likes, comments, source, read_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (job_id, milestone) DO NOTHING;

-- name: ListPostedJobsWithoutFacts :many
SELECT j.id, j.source_snapshot
FROM clone_jobs j
LEFT JOIN clone_post_facts f ON f.job_id = j.id
WHERE j.posted_at IS NOT NULL AND j.superseded_at IS NULL AND f.job_id IS NULL
ORDER BY j.posted_at, j.id
LIMIT $1;

-- name: ListChannelPosted :many
SELECT j.id AS job_id, j.posted_at, j.template_id, COALESCE(t.name, '')::text AS template_name,
       f.youtube_id, COALESCE(f.title, '')::text AS title, f.duration_sec, f.published_at
FROM clone_jobs j
LEFT JOIN clone_post_facts f ON f.job_id = j.id
LEFT JOIN templates t ON t.id = j.template_id
WHERE j.channel_id = $1 AND j.posted_at IS NOT NULL AND j.superseded_at IS NULL
ORDER BY j.posted_at DESC, j.id;

-- name: ListChannelPostStats :many
SELECT s.job_id, s.milestone, s.views, s.likes, s.comments, s.source, s.read_at
FROM clone_post_stats s
JOIN clone_jobs j ON j.id = s.job_id
WHERE j.channel_id = $1 AND j.posted_at IS NOT NULL AND j.superseded_at IS NULL;

-- name: ListPostedChannelIDs :many
SELECT DISTINCT channel_id FROM clone_jobs
WHERE posted_at IS NOT NULL AND superseded_at IS NULL;

-- name: ListReadCandidates :many
-- Video đã có link YouTube, chưa quá 60 ngày sau khi đăng, tới lượt thử đọc.
SELECT f.job_id, f.youtube_id, f.published_at, f.read_attempts, j.posted_at,
       COALESCE(array_agg(s.milestone) FILTER (WHERE s.milestone IS NOT NULL), '{}')::text[] AS have
FROM clone_post_facts f
JOIN clone_jobs j ON j.id = f.job_id
LEFT JOIN clone_post_stats s ON s.job_id = f.job_id
WHERE f.youtube_id IS NOT NULL AND j.posted_at IS NOT NULL AND j.superseded_at IS NULL
  AND (f.next_read_at IS NULL OR f.next_read_at <= $1::timestamptz)
  AND j.posted_at > $1::timestamptz - interval '60 days'
GROUP BY f.job_id, f.youtube_id, f.published_at, f.read_attempts, j.posted_at
ORDER BY j.posted_at, f.job_id
LIMIT 50;

-- name: ListAppliedInsights :many
SELECT insight_id, kind, rule_id FROM performance_insights_applied WHERE channel_id = $1;

-- name: InsertAppliedInsight :execrows
INSERT INTO performance_insights_applied (channel_id, insight_id, kind, rule_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (channel_id, insight_id) DO NOTHING;

-- name: SetTemplateWeight :execrows
UPDATE owner_channel_templates SET weight = $3
WHERE owner_channel_id = $1 AND template_id = $2;

-- name: ListTemplateWeights :many
SELECT template_id, weight FROM owner_channel_templates WHERE owner_channel_id = $1;

-- name: ListChannelTemplateViews7d :many
-- Lượt xem 7 ngày của video đã đăng, theo template (cho Template luân phiên khi ưu tiên Lượt xem).
SELECT j.template_id, s.views
FROM clone_post_stats s
JOIN clone_jobs j ON j.id = s.job_id
WHERE j.channel_id = $1 AND j.posted_at IS NOT NULL AND j.superseded_at IS NULL
  AND s.milestone = '7d' AND j.template_id IS NOT NULL;

-- name: ListRivalVideoScores :many
-- Điểm nổi trội của video đối thủ (cho Tự lấy khi ưu tiên Lượt xem).
SELECT id, score FROM rival_videos WHERE id = ANY($1::text[]);
