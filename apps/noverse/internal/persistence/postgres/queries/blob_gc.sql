-- name: GCScanBlobs :many
-- Keyset paging bounds memory even while earlier pages are tombstoned/deleted.
SELECT b.* FROM blobs b
WHERE b.object_key LIKE sqlc.arg(prefix)::text || '%'
  AND b.object_key > sqlc.arg(after_key)::text
  AND b.state = sqlc.arg(state)::text
  AND (b.state = 'deleting' OR (
    b.created_at < sqlc.arg(cutoff)::timestamptz
    AND (b.state = 'staged' OR b.updated_at < sqlc.arg(cutoff)::timestamptz)
    AND NOT EXISTS (SELECT 1 FROM export_cache c WHERE c.project_id=b.project_id AND c.blob_id=b.id)
    AND NOT EXISTS (SELECT 1 FROM audio_renders r WHERE r.project_id=b.project_id AND r.blob_id=b.id)
    AND NOT EXISTS (SELECT 1 FROM audio_voices v WHERE v.project_id=b.project_id AND v.sample_blob_id=b.id)
    AND NOT EXISTS (SELECT 1 FROM project_documents d WHERE d.project_id=b.project_id AND d.blob_id=b.id)
    AND NOT EXISTS (SELECT 1 FROM story_artifacts a WHERE a.project_id=b.project_id AND a.blob_id=b.id)
    AND (b.state <> 'staged' OR NOT EXISTS (
      SELECT 1 FROM jobs j WHERE j.project_id=b.project_id
        AND (b.upload_job_id IS NULL OR j.id=b.upload_job_id)
        AND (j.state IN ('queued','paused','waiting')
          OR (j.state='running' AND j.lease_until IS NULL) OR j.lease_until>now())))
  ))
ORDER BY b.object_key LIMIT sqlc.arg(page_size);

-- name: GCRegisteredKeys :many
-- Reconcile just one provider page, never load the complete registry.
SELECT object_key FROM blobs WHERE object_key = ANY(sqlc.arg(keys)::text[]);

-- name: GCClaimBlob :execrows
UPDATE blobs SET state='deleting', updated_at=clock_timestamp(), version=version+1
WHERE id=$1 AND state IN ('ready','deleting');

-- name: GCFinishBlob :execrows
DELETE FROM blobs WHERE id=$1 AND state='deleting';

-- name: GCExpiredCachePage :many
SELECT c.project_id,c.story_id,c.cache_key,c.kind,c.blob_id
FROM export_cache c JOIN blobs b ON b.project_id=c.project_id AND b.id=c.blob_id
WHERE c.expires_at IS NOT NULL AND c.expires_at < now()
  AND b.object_key LIKE sqlc.arg(prefix)::text || '%'
  AND (c.project_id,c.story_id,c.cache_key,c.kind) >
    (sqlc.arg(after_project)::uuid,sqlc.arg(after_story)::uuid,sqlc.arg(after_cache_key)::text,sqlc.arg(after_kind)::text)
ORDER BY c.project_id,c.story_id,c.cache_key,c.kind LIMIT sqlc.arg(page_size);

-- name: GCClaimOrphan :one
-- NULL project_id is an unattached maintenance tombstone, never tenant content.
-- The deleting state excludes publication/reads; owner FKs cannot attach to it.
INSERT INTO blobs(id,project_id,object_key,storage_driver,sha256,size_bytes,content_type,state)
VALUES ($1,NULL,$2,$3,repeat('0',64),0,'application/octet-stream','deleting')
ON CONFLICT(object_key) DO NOTHING RETURNING id;
