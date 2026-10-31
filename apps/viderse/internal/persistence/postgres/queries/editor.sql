-- Immutable editor artifacts are bound by media.Publish before these pointer
-- mutations, in that publication's transaction. The repository compares wire
-- digests, generates revision counters, and releases unowned prior bindings.

-- name: GetEditorProject :one
SELECT project_id, revision, logical_path, content_revision, updated_at
FROM editor_projects WHERE project_id = $1;

-- name: GetEditorResource :one
SELECT resource_key, revision, logical_path, content_revision, updated_at
FROM editor_resources WHERE resource_key = $1;

-- name: ListEditorPointers :many
SELECT resource_key, content_revision, logical_path, revision, updated_at FROM (
    SELECT 'projects/' || project_id AS resource_key, content_revision, logical_path, revision, updated_at
    FROM editor_projects
    UNION ALL
    SELECT resource_key, content_revision, logical_path, revision, updated_at
    FROM editor_resources
) heads
WHERE left(resource_key, length(sqlc.arg(prefix)::text)) = sqlc.arg(prefix)::text
ORDER BY resource_key;

-- name: ListEditorBlobPaths :many
SELECT logical_path FROM blob_refs
WHERE left(logical_path, length(sqlc.arg(prefix)::text)) = sqlc.arg(prefix)::text
ORDER BY logical_path;

-- name: CreateEditorProjectPointer :execrows
INSERT INTO editor_projects (project_id, revision, logical_path, content_revision, updated_at)
VALUES ($1, 1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: CASEditorProjectPointer :execrows
UPDATE editor_projects
SET revision = revision + 1,
    logical_path = sqlc.arg(next_path),
    content_revision = sqlc.arg(next_content_revision),
    updated_at = sqlc.arg(updated_at)
WHERE project_id = sqlc.arg(project_id)
  AND content_revision = sqlc.arg(expected_content_revision);

-- name: CreateEditorResourcePointer :execrows
INSERT INTO editor_resources (resource_key, revision, logical_path, content_revision, updated_at)
VALUES ($1, 1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: CASEditorResourcePointer :execrows
UPDATE editor_resources
SET revision = revision + 1,
    logical_path = sqlc.arg(next_path),
    content_revision = sqlc.arg(next_content_revision),
    updated_at = sqlc.arg(updated_at)
WHERE resource_key = sqlc.arg(resource_key)
  AND content_revision = sqlc.arg(expected_content_revision);

-- name: DeleteEditorProjectPointer :execrows
DELETE FROM editor_projects
WHERE project_id = $1 AND content_revision = $2;

-- name: DeleteEditorResourcePointer :execrows
DELETE FROM editor_resources
WHERE resource_key = $1 AND content_revision = $2;

-- name: IsEditorResourceDeleted :one
SELECT EXISTS (
    SELECT 1 FROM editor_deleted_resources WHERE resource_key = $1
);

-- name: MarkEditorResourceDeleted :exec
INSERT INTO editor_deleted_resources(resource_key)
VALUES ($1)
ON CONFLICT(resource_key) DO UPDATE SET updated_at = now();

-- name: ClearEditorResourceDeleted :exec
DELETE FROM editor_deleted_resources WHERE resource_key = $1;
