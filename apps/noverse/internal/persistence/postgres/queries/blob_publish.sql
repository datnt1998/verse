-- name: LockPublishBlob :one
SELECT * FROM blobs WHERE object_key = $1 FOR UPDATE;

-- name: GetDeliveryBlob :one
SELECT * FROM blobs WHERE object_key = $1 AND state = 'ready';

-- name: InsertPublishedBlob :execrows
INSERT INTO blobs (id, project_id, object_key, storage_driver, content_type, sha256, size_bytes, state, upload_job_id, usage_kind)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'ready', $8, $9)
ON CONFLICT (object_key) DO NOTHING;

-- name: ResumePublishedBlob :one
UPDATE blobs SET state = 'ready', sha256 = $2, size_bytes = $3,
    upload_job_id = coalesce(upload_job_id, $4),
    updated_at = clock_timestamp(), version = version + 1, usage_kind=$5
WHERE id = $1 AND state = 'staged'
RETURNING *;
