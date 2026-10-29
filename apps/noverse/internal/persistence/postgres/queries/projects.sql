-- Project anchors, memberships, audit log and the shared auth rate-limit
-- windows (phase 04).

-- name: GetProjectBySlug :one
SELECT * FROM projects WHERE slug = $1 AND deleted_at IS NULL;

-- name: EnsureProject :one
-- Insert-or-fetch anchor by slug. DO NOTHING (instead of a self-assignment
-- UPDATE) keeps the row version guard meaningful: an existing anchor is
-- returned untouched.
WITH ins AS (
    INSERT INTO projects (id, slug, name)
    VALUES ($1, $2, $3)
    ON CONFLICT (slug) DO NOTHING
    RETURNING *
)
SELECT * FROM ins
UNION ALL
SELECT * FROM projects WHERE slug = $2
LIMIT 1;

-- name: ListProjects :many
SELECT * FROM projects WHERE deleted_at IS NULL ORDER BY slug;

-- name: InsertProject :one
INSERT INTO projects (id, slug, name, description, notes, universe_name)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateProjectFile :execrows
UPDATE projects
SET name = $2, description = $3, notes = $4, universe_name = $5,
    updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListProjectMembers :many
SELECT pm.project_id, pm.user_id, pm.role, pm.removed_at, pm.created_at,
    pm.updated_at, pm.version, u.username::text AS username, u.display_name,
    u.status AS user_status
FROM project_members pm
JOIN users u ON u.id = pm.user_id AND u.deleted_at IS NULL
JOIN projects p ON p.id = pm.project_id AND p.deleted_at IS NULL
WHERE pm.project_id = $1 AND pm.removed_at IS NULL
ORDER BY u.username;

-- name: ListMembershipsByUser :many
SELECT pm.role, p.slug
FROM project_members pm
JOIN projects p ON p.id = pm.project_id AND p.deleted_at IS NULL
WHERE pm.user_id = $1 AND pm.removed_at IS NULL;

-- name: LockProjectMembers :many
SELECT pm.* FROM project_members pm
WHERE pm.project_id = $1 AND pm.removed_at IS NULL
ORDER BY pm.user_id
FOR UPDATE;

-- name: CountActiveOwners :one
SELECT count(*) FROM project_members pm
JOIN users u ON u.id = pm.user_id AND u.deleted_at IS NULL AND u.status = 'active'
WHERE pm.project_id = $1 AND pm.role = 'owner' AND pm.removed_at IS NULL;

-- name: UpsertMember :exec
INSERT INTO project_members (project_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role,
    removed_at = NULL, updated_at = clock_timestamp(),
    version = project_members.version + 1;

-- name: RemoveMember :exec
UPDATE project_members SET removed_at = $3, updated_at = $3, version = version + 1
WHERE project_id = $1 AND user_id = $2 AND removed_at IS NULL;

-- name: DemoteOwnersToCollab :exec
UPDATE project_members SET role = 'collab', updated_at = $2, version = version + 1
WHERE project_id = $1 AND role = 'owner' AND removed_at IS NULL;

-- name: OwnerCountIncludingRemoved :one
SELECT count(*) FROM project_members
WHERE project_id = $1 AND role = 'owner' AND removed_at IS NULL;

-- name: GetLimitBucketForUpdate :one
SELECT * FROM login_rate_limits WHERE bucket_hash = $1 FOR UPDATE;

-- name: InsertAudit :one
INSERT INTO audit_logs (id, project_id, actor_user_id, actor_kind, action,
    target_kind, target_id, request_id, details, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING sequence;

-- name: ListAudit :many
SELECT al.sequence, al.actor_kind, al.action, al.target_kind, al.target_id,
    al.details, al.occurred_at, p.slug AS project_slug,
    coalesce(u.username, '')::text AS actor_username,
    coalesce(u.display_name, '')::text AS actor_name
FROM audit_logs al
LEFT JOIN projects p ON p.id = al.project_id
LEFT JOIN users u ON u.id = al.actor_user_id
WHERE ($1::text = '' OR al.target_kind = $1)
  AND ($2::text = '' OR u.username::text = $2 OR (u.id IS NULL AND $2 = 'system'))
  AND ($3::bigint = 0 OR al.sequence < $3)
ORDER BY al.sequence DESC
LIMIT $4;

-- name: PurgeAuditBefore :execrows
DELETE FROM audit_logs WHERE occurred_at < $1;

-- name: GetLimitBucket :one
SELECT * FROM login_rate_limits WHERE bucket_hash = $1;

-- name: InsertLimitBucket :exec
INSERT INTO login_rate_limits (bucket_hash, failures, window_started_at, blocked_until, expires_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (bucket_hash) DO UPDATE SET failures = EXCLUDED.failures,
    window_started_at = EXCLUDED.window_started_at,
    blocked_until = EXCLUDED.blocked_until,
    expires_at = EXCLUDED.expires_at,
    updated_at = clock_timestamp(), version = login_rate_limits.version + 1;

-- name: DeleteLimitBucket :exec
DELETE FROM login_rate_limits WHERE bucket_hash = $1;

-- name: PurgeExpiredLimitBuckets :exec
DELETE FROM login_rate_limits WHERE expires_at < $1;

-- name: GetReadyBlobProject :one
SELECT p.slug FROM blobs b
JOIN projects p ON p.id = b.project_id AND p.deleted_at IS NULL
WHERE b.object_key = $1 AND b.state = 'ready';
