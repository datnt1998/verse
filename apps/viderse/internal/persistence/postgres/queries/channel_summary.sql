-- name: ChannelSummaryStats :many
SELECT c.id, (SELECT count(*) FROM clone_queue q WHERE q.channel_id=c.id AND q.state='started' AND q.priority='batch' AND q.removed_at IS NULL AND q.started_at >= $1::timestamptz)::bigint AS pace_used,
       (SELECT count(*) FROM clone_jobs j WHERE j.channel_id=c.id AND j.superseded_at IS NULL AND j.posted_at >= $2::timestamptz)::bigint AS posted_week
FROM owner_channels c;

-- name: ChannelSummaryTemplateIssues :many
SELECT oct.owner_channel_id, t.id, t.name
FROM owner_channel_templates oct JOIN templates t ON t.id=oct.template_id
WHERE t.recipe->'backdrop' IS NOT NULL AND t.recipe->'backdrop' <> 'null'::jsonb
AND NOT EXISTS (
 SELECT 1 FROM jsonb_array_elements_text(COALESCE(t.recipe->'backdrop'->'fixedVideoIds','[]'::jsonb)) AS f(id)
 JOIN backdrop_videos v ON v.id=f.id AND v.state='ready'
)
AND NOT EXISTS (
 SELECT 1 FROM jsonb_array_elements(COALESCE(t.recipe->'backdrop'->'channels','[]'::jsonb)) AS l(link)
 JOIN backdrop_channel_videos v ON v.channel_id=l.link->>'channelId' AND v.unavailable_at IS NULL
 WHERE (jsonb_array_length(COALESCE(l.link->'kinds','[]'::jsonb))=0 OR l.link->'kinds' ? v.kind)
 AND (jsonb_array_length(COALESCE(l.link->'pinned','[]'::jsonb))=0 OR l.link->'pinned' ? v.video_id)
)
ORDER BY oct.owner_channel_id, oct.position;
