-- Project anchors and the membership lookup/mutation primitives the Noverse
-- business repositories still use. Account, session, limiter and audit SQL
-- lives in platform/identity/postgres.

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

-- name: ListMembershipsByUser :many
SELECT pm.role, p.slug
FROM project_members pm
JOIN projects p ON p.id = pm.project_id AND p.deleted_at IS NULL
WHERE pm.user_id = $1 AND pm.removed_at IS NULL;

-- name: UpsertMember :exec
INSERT INTO project_members (project_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role,
    removed_at = NULL, updated_at = clock_timestamp(),
    version = project_members.version + 1;


-- name: GetReadyBlobProject :one
SELECT p.slug FROM blobs b
JOIN projects p ON p.id = b.project_id AND p.deleted_at IS NULL
WHERE b.object_key = $1 AND b.state = 'ready';
