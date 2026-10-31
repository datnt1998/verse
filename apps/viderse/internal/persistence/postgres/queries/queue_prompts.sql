-- name: UpdateQueueOverrides :execrows
UPDATE clone_queue
SET scene_prompt_id = $3, cover_prompt_id = $4, no_images = $5,
    override_revision = override_revision + 1
WHERE id = $1 AND override_revision = $2 AND state = 'queued' AND removed_at IS NULL;

-- name: LockQueueItemForAdmission :one
SELECT * FROM clone_queue WHERE id = $1 FOR UPDATE;
