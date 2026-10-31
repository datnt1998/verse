-- name: CountAuditBefore :one
SELECT count(*) FROM audit_logs WHERE occurred_at < $1;

-- name: DeleteAuditRetentionBatch :execrows
DELETE FROM audit_logs WHERE id IN (
    SELECT id FROM audit_logs WHERE occurred_at < sqlc.arg(cutoff)::timestamptz
    ORDER BY occurred_at, id LIMIT sqlc.arg(batch_size)
);
