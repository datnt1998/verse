-- Kết nối AI: nhà cung cấp và danh mục model (§15).

-- name: ListAiProviders :many
-- Thẻ mới thêm gần nhất đứng đầu danh sách (giống "thẻ mới lên đầu" của W13).
-- Kèm số model trong CÙNG truy vấn: GET không được N+1 theo provider.
SELECT p.*, (SELECT count(*) FROM ai_models m WHERE m.provider_id = p.id)::int AS model_count
FROM ai_providers p
ORDER BY p.created_at DESC, p.id;

-- name: ListAiSubscribed :many
-- Chỉ thẻ đăng nhập gói (chatgpt/grok) + số model — đường đọc attention.
SELECT p.*, (SELECT count(*) FROM ai_models m WHERE m.provider_id = p.id)::int AS model_count
FROM ai_providers p
WHERE p.auth = 'subscription'
ORDER BY p.id;

-- name: GetAiProvider :one
SELECT * FROM ai_providers WHERE id = $1;

-- name: InsertAiProvider :one
INSERT INTO ai_providers (id, kind, auth, label, key_enc, key_hint, base_url, status, selected)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateAiProviderKey :exec
-- Chỉ khoá/địa chỉ/trạng thái: lựa chọn model có câu riêng nên PATCH khoá
-- không ghi đè lựa chọn mới của tab khác.
UPDATE ai_providers
SET key_enc = $2, key_hint = $3, base_url = $4, status = $5, updated_at = now()
WHERE id = $1;

-- name: SetAiProviderSelected :exec
UPDATE ai_providers
SET selected = $2, updated_at = now()
WHERE id = $1;

-- name: SetAiProviderStatus :exec
UPDATE ai_providers
SET status = $2, updated_at = now()
WHERE id = $1;

-- name: SetAiProviderCatalogAt :exec
UPDATE ai_providers
SET catalog_at = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteAiProvider :exec
DELETE FROM ai_providers WHERE id = $1;

-- name: ListAiModels :many
SELECT * FROM ai_models
WHERE provider_id = $1
ORDER BY id;

-- name: SearchAiModels :many
-- Một trang danh mục: lọc q/filter + keyset theo id (cursor = id cuối trang
-- trước) đều trong SQL; caller xin limit+1 để biết còn trang.
SELECT * FROM ai_models
WHERE provider_id = $1
  AND ($2::text = '' OR position($2::text in lower(id)) > 0)
  AND (
    $3::text = '' OR $3::text = 'all'
    OR ($3::text = 'selected' AND id = ANY ($4::text[]))
    OR ($3::text = 'recommended' AND recommended)
  )
  AND ($5::text = '' OR id > $5::text)
ORDER BY id
LIMIT $6;

-- name: CountAiModels :one
-- Tổng số model khớp bộ lọc (không tính cursor/limit) cho `total` của trang.
SELECT count(*)::int FROM ai_models
WHERE provider_id = $1
  AND ($2::text = '' OR position($2::text in lower(id)) > 0)
  AND (
    $3::text = '' OR $3::text = 'all'
    OR ($3::text = 'selected' AND id = ANY ($4::text[]))
    OR ($3::text = 'recommended' AND recommended)
  );

-- name: GetAiModel :one
SELECT * FROM ai_models
WHERE provider_id = $1 AND id = $2;

-- name: UpsertAiModel :exec
-- Ghi đè trường catalog nhưng GIỮ cờ manual một khi đã bật: model đã thêm tay
-- không bị lần lấy danh mục tự động hạ cấp rồi xoá.
INSERT INTO ai_models (provider_id, id, caps, context_k, in_per_m, out_per_m, per_image, recommended, manual)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (provider_id, id) DO UPDATE
SET caps = $3, context_k = $4, in_per_m = $5, out_per_m = $6,
    per_image = $7, recommended = $8,
    manual = ai_models.manual OR EXCLUDED.manual;

-- name: DeleteAiCatalogAuto :exec
-- Chỉ xoá model lấy từ danh mục: model thêm tay (manual) sống sót qua
-- mỗi lần lấy danh mục lại.
DELETE FROM ai_models
WHERE provider_id = $1 AND NOT manual;

-- name: DeleteAiModel :exec
DELETE FROM ai_models
WHERE provider_id = $1 AND id = $2;

-- name: AiProviderInUse :many
-- Model nào của provider đang được một trợ lý gán — cho DELETE 409 in_use.
SELECT id FROM ai_models
WHERE provider_id = $1 AND id = ANY ($2::text[]);
