-- Đọc/ghi cho mảng Thư viện: template, prompt ảnh, lớp phủ. Truy vấn chéo
-- mảng chỉ ĐỌC bảng của mảng khác (owner_channels.default_template_id để đếm
-- kênh dùng template; backdrop_* để kiểm kênh/video nền còn tồn tại).

-- ── template ───────────────────────────────────────────────────────────────

-- name: ListTemplates :many
-- Mọi template kèm kênh đang dùng (một lượt gộp, không N+1).
SELECT t.id, t.name, t.recipe, t.revision, t.preview_revision, t.preview_built_at,
       t.created_at, t.updated_at,
       COALESCE(ch.channels, '[]'::jsonb) AS channels
FROM templates t
LEFT JOIN LATERAL (
    SELECT jsonb_agg(jsonb_build_object('id', c.id, 'name', c.name)
                     ORDER BY c.name, c.id) AS channels
    FROM owner_channels c
    WHERE c.default_template_id = t.id
) ch ON true
ORDER BY t.updated_at DESC, t.id;

-- name: GetTemplate :one
SELECT t.id, t.name, t.recipe, t.revision, t.preview_revision, t.preview_built_at,
       t.created_at, t.updated_at,
       COALESCE(ch.channels, '[]'::jsonb) AS channels
FROM templates t
LEFT JOIN LATERAL (
    SELECT jsonb_agg(jsonb_build_object('id', c.id, 'name', c.name)
                     ORDER BY c.name, c.id) AS channels
    FROM owner_channels c
    WHERE c.default_template_id = t.id
) ch ON true
WHERE t.id = sqlc.arg(id);

-- name: InsertTemplate :exec
INSERT INTO templates (id, name, recipe, revision)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(recipe)::jsonb, sqlc.arg(revision));

-- name: UpdateTemplate :one
-- Ghi recipe (và tên nếu có) chỉ khi revision khớp — hai người lưu cùng lúc
-- thì người sau nhận 0 dòng và API trả 409.
UPDATE templates
SET name = COALESCE(sqlc.narg(name)::text, name),
    recipe = sqlc.arg(recipe)::jsonb,
    revision = revision + 1,
    updated_at = now()
WHERE id = sqlc.arg(id) AND revision = sqlc.arg(expect_revision)
RETURNING revision;

-- name: RenameTemplate :one
UPDATE templates
SET name = sqlc.arg(name), updated_at = now()
WHERE id = sqlc.arg(id)
  AND (sqlc.narg(expect_revision)::bigint IS NULL OR revision = sqlc.narg(expect_revision))
RETURNING revision;

-- name: NameChannelStandardTemplates :many
-- Mẫu chuẩn tạo lúc kênh chưa có tên mang tên trần; khi kênh có tên thì đổi
-- sang tên kèm kênh. Chỉ template còn đúng tên trần và chỉ kênh này dùng —
-- đã đổi tên tay hoặc kênh khác cũng dùng thì giữ nguyên. Không đổi revision.
UPDATE templates t
SET name = sqlc.arg(name), updated_at = now()
WHERE t.name = sqlc.arg(plain_name)
  AND EXISTS (SELECT 1 FROM owner_channels c
              WHERE c.default_template_id = t.id AND c.id = sqlc.arg(channel_id)::uuid)
  AND NOT EXISTS (SELECT 1 FROM owner_channels c
                  WHERE c.default_template_id = t.id AND c.id <> sqlc.arg(channel_id)::uuid)
RETURNING t.id;

-- name: DeleteTemplate :execrows
DELETE FROM templates WHERE id = sqlc.arg(id);

-- name: TemplateChannels :many
-- Kênh đang dùng template làm mặc định ("Mặc định của …" và guard xoá).
SELECT c.id, c.name
FROM owner_channels c
WHERE c.default_template_id = sqlc.arg(template_id)::uuid
ORDER BY c.name, c.id;

-- name: TemplatesUsingPrompt :many
SELECT id, name
FROM templates
WHERE recipe->'scenes'->>'scenePromptId' = sqlc.arg(prompt_id)::text
   OR recipe->'scenes'->>'coverPromptId' = sqlc.arg(prompt_id)::text
ORDER BY name, id;

-- name: OverlayUsage :many
SELECT t.id, t.name, o.value::text AS overlay_id
FROM templates t
CROSS JOIN LATERAL jsonb_array_elements_text(t.recipe->'overlays') AS o(value)
WHERE jsonb_typeof(t.recipe->'overlays') = 'array'
ORDER BY o.value, t.name;

-- ── prompt ảnh ─────────────────────────────────────────────────────────────

-- name: ListPrompts :many
SELECT id, name, kind, COALESCE(composition, '') AS composition, prompt, updated_at
FROM image_prompts
WHERE (sqlc.arg(kind)::text = '' OR kind = sqlc.arg(kind)::text)
  AND (sqlc.arg(q)::text = ''
       OR strpos(fold_title(name), sqlc.arg(q)::text) > 0
       OR strpos(fold_title(prompt), sqlc.arg(q)::text) > 0)
ORDER BY updated_at DESC, id;

-- name: GetPrompt :one
SELECT id, name, kind, COALESCE(composition, '') AS composition, prompt, updated_at
FROM image_prompts
WHERE id = sqlc.arg(id);

-- name: PromptNameTaken :one
SELECT EXISTS(
    SELECT 1 FROM image_prompts
    WHERE lower(name) = lower(sqlc.arg(name)::text)
      AND (sqlc.narg(except_id)::uuid IS NULL OR id <> sqlc.narg(except_id)::uuid)
) AS taken;

-- name: PromptExists :one
SELECT EXISTS(
    SELECT 1 FROM image_prompts WHERE id = sqlc.arg(id) AND kind = sqlc.arg(kind)
) AS exists;

-- name: InsertPrompt :exec
INSERT INTO image_prompts (id, name, kind, composition, prompt, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(kind),
        sqlc.narg(composition), sqlc.arg(prompt), sqlc.arg(updated_at));

-- name: UpdatePrompt :exec
UPDATE image_prompts
SET name = sqlc.arg(name),
    kind = sqlc.arg(kind),
    composition = sqlc.narg(composition),
    prompt = sqlc.arg(prompt),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: LockPrompt :one
-- Khoá hàng prompt khi đổi loại/xoá: writer template khoá cùng hàng qua
-- LockPromptKinds nên hai đường không đua nhau tạo tham chiếu hỏng.
SELECT id, kind FROM image_prompts WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: LockPromptKinds :many
-- Khoá các prompt recipe trỏ tới (bỏ id builtin:* — đó là hằng số, không phải
-- hàng) và trả kind để store kiểm đúng loại trước khi ghi template.
SELECT id, kind FROM image_prompts WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id FOR UPDATE;

-- name: DeletePrompt :execrows
DELETE FROM image_prompts WHERE id = sqlc.arg(id);

-- ── lớp phủ ────────────────────────────────────────────────────────────────

-- name: ListOverlays :many
SELECT id, sha256, name, file_path, mime, bytes, duration_sec, width, height,
       background, COALESCE(builtin, '') AS builtin, created_at
FROM overlays
WHERE removed_at IS NULL
ORDER BY created_at DESC, id;

-- name: OverlayByID :one
SELECT id, sha256, name, file_path, mime, bytes, duration_sec, width, height,
       background, COALESCE(builtin, '') AS builtin, created_at
FROM overlays
WHERE id = sqlc.arg(id) AND removed_at IS NULL;

-- name: OverlayBySHA :one
SELECT id, sha256, name, file_path, mime, bytes, duration_sec, width, height,
       background, COALESCE(builtin, '') AS builtin, created_at,
       (removed_at IS NOT NULL)::bool AS removed
FROM overlays
WHERE sha256 = sqlc.arg(sha256);

-- name: OverlayExists :one
SELECT EXISTS(
    SELECT 1 FROM overlays WHERE id = sqlc.arg(id) AND removed_at IS NULL
) AS exists;

-- name: InsertOverlay :exec
INSERT INTO overlays (id, sha256, name, file_path, mime, bytes, duration_sec, width, height, background, builtin, created_at)
VALUES (sqlc.arg(id), sqlc.arg(sha256), sqlc.arg(name), sqlc.arg(file_path), sqlc.arg(mime),
        sqlc.arg(bytes), sqlc.narg(duration_sec), sqlc.narg(width), sqlc.narg(height),
        sqlc.arg(background), sqlc.narg(builtin), sqlc.arg(created_at));

-- name: RestoreOverlay :exec
UPDATE overlays SET removed_at = NULL, name = sqlc.arg(name) WHERE id = sqlc.arg(id);

-- name: RemoveOverlay :execrows
UPDATE overlays SET removed_at = now() WHERE id = sqlc.arg(id) AND removed_at IS NULL;

-- name: ExistingBuiltins :many
SELECT builtin FROM overlays WHERE builtin IS NOT NULL;

-- name: LockSharedRef :exec
-- Advisory transaction lock dùng chung với mảng Video nền — cùng hình khoá của
-- LockBackdropResource (backdrop.ChannelLockKey/VideoLockKey). Template ghi
-- recipe.backdrop phải giữ khoá này trong cùng transaction để "kiểm tồn tại
-- rồi ghi" không đua với xoá kênh/video nền.
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(key)::text, 0));

-- ── kiểm tham chiếu sang mảng Video nền (chỉ đọc) ──────────────────────────

-- name: BackdropChannelsExisting :many
SELECT id FROM backdrop_channels WHERE id = ANY(sqlc.arg(ids)::text[]);

-- name: BackdropVideosExisting :many
SELECT video_id
FROM backdrop_channel_videos
WHERE channel_id = sqlc.arg(channel_id) AND video_id = ANY(sqlc.arg(ids)::text[]);

-- name: BackdropVideoExists :one
SELECT EXISTS(SELECT 1 FROM backdrop_videos WHERE id = sqlc.arg(id)) AS exists;
