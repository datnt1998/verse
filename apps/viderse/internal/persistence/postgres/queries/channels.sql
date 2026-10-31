-- Kênh của bạn: kênh + đối thủ liên kết + danh sách template luân phiên +
-- bộ nhận diện (lease một run mỗi kênh). Ghi nhiều dòng nguyên tử nằm trong
-- transaction của store; mỗi câu ở đây là một bước đơn vị.

-- name: ListOwnerChannels :many
-- Tên rỗng (kênh chưa được đặt tên) đứng cuối danh sách.
SELECT * FROM owner_channels
ORDER BY (name = ''), lower(name), id;

-- name: GetOwnerChannel :one
SELECT * FROM owner_channels WHERE id = $1;

-- name: ChannelExists :one
SELECT EXISTS(SELECT 1 FROM owner_channels WHERE id = $1)::bool AS exists;

-- name: RivalExistsForChannels :one
SELECT EXISTS(SELECT 1 FROM rival_channels WHERE id = $1)::bool AS exists;

-- name: TemplateExistsForChannels :one
SELECT EXISTS(SELECT 1 FROM templates WHERE id = $1)::bool AS exists;

-- name: InsertOwnerChannel :exec
INSERT INTO owner_channels (id, name, youtube_url, language, auto_pick, pace_per_day)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UnlinkSource :execrows
DELETE FROM owner_channel_sources
WHERE owner_channel_id = $1 AND rival_id = $2;

-- name: CountSources :one
SELECT count(*)::bigint FROM owner_channel_sources WHERE owner_channel_id = $1;

-- name: LinkSource :execrows
-- Idempotent: liên kết lại là no-op (ON CONFLICT DO NOTHING); số dòng trả về
-- cho caller biết có phải link mới.
INSERT INTO owner_channel_sources (owner_channel_id, rival_id)
VALUES ($1, $2)
ON CONFLICT (owner_channel_id, rival_id) DO NOTHING;

-- name: ListChannelSources :many
-- Đối thủ của nhiều kênh trong một truy vấn, thứ tự liên kết (linked_at);
-- dòng đầu mỗi kênh là đối thủ tham chiếu nhận diện.
SELECT s.owner_channel_id, s.rival_id, s.linked_at, r.title, r.handle
FROM owner_channel_sources s
JOIN rival_channels r ON r.id = s.rival_id
WHERE s.owner_channel_id = ANY($1::uuid[])
ORDER BY s.owner_channel_id, s.linked_at, s.rival_id;

-- name: ListChannelTemplates :many
-- Template luân phiên của nhiều kênh trong một truy vấn, theo position.
SELECT t.owner_channel_id, t.template_id, t.position, tp.name
FROM owner_channel_templates t
JOIN templates tp ON tp.id = t.template_id
WHERE t.owner_channel_id = ANY($1::uuid[])
ORDER BY t.owner_channel_id, t.position;

-- name: ListChannelTemplateIDs :many
-- Danh sách position của một kênh, để chèn cuối/xếp lại trong tx.
SELECT template_id, position FROM owner_channel_templates
WHERE owner_channel_id = $1
ORDER BY position;

-- name: InsertChannelTemplate :execrows
INSERT INTO owner_channel_templates (owner_channel_id, template_id, position)
VALUES ($1, $2, $3)
ON CONFLICT (owner_channel_id, template_id) DO NOTHING;

-- name: DeleteChannelTemplate :execrows
DELETE FROM owner_channel_templates
WHERE owner_channel_id = $1 AND template_id = $2;

-- name: SetChannelTemplatePosition :exec
UPDATE owner_channel_templates
SET position = $3
WHERE owner_channel_id = $1 AND template_id = $2;

-- name: PatchChannel :execrows
-- Cập nhật đúng các trường được đưa (mỗi trường một cờ *_set); branding chỉ
-- đổi khi cần sửa brief của nhận diện.
UPDATE owner_channels SET
    name               = CASE WHEN sqlc.arg(name_set)::bool        THEN sqlc.narg(name)::text        ELSE name END,
    youtube_url        = CASE WHEN sqlc.arg(youtube_url_set)::bool THEN sqlc.narg(youtube_url)::text ELSE youtube_url END,
    language           = CASE WHEN sqlc.arg(language_set)::bool    THEN sqlc.arg(language)::text     ELSE language END,
    style_note         = CASE WHEN sqlc.arg(style_note_set)::bool  THEN sqlc.narg(style_note)::text  ELSE style_note END,
    description_footer = CASE WHEN sqlc.arg(footer_set)::bool      THEN sqlc.narg(footer)::text      ELSE description_footer END,
    always_tags        = CASE WHEN sqlc.arg(tags_set)::bool        THEN sqlc.arg(tags)::text[]       ELSE always_tags END,
    auto_pick          = CASE WHEN sqlc.arg(auto_pick_set)::bool   THEN sqlc.arg(auto_pick)::bool    ELSE auto_pick END,
    pace_per_day       = CASE WHEN sqlc.arg(pace_set)::bool        THEN sqlc.arg(pace)::int          ELSE pace_per_day END,
    branding           = CASE WHEN sqlc.arg(branding_set)::bool
                              THEN jsonb_set(branding, '{brief}', to_jsonb(sqlc.arg(brief)::text), true)
                              ELSE branding END,
    updated_at         = clock_timestamp()
WHERE id = sqlc.arg(id)::uuid;

-- name: DeleteChannel :execrows
DELETE FROM owner_channels WHERE id = $1;

-- name: LockOwnerChannel :one
-- Khoá hàng kênh trong transaction trước khi đọc-đổi-ghi link/template —
-- serialize hai thao tác đồng thời trên cùng kênh.
SELECT id FROM owner_channels WHERE id = $1 FOR UPDATE;

-- name: SetAutoPick :execrows
-- Bật/tắt Tự lấy; tắt khi bỏ liên kết đối thủ cuối cùng.
UPDATE owner_channels
SET auto_pick = $2, updated_at = clock_timestamp()
WHERE id = $1;

-- name: SetAutoPickTouch :execrows
-- Bump updated_at sau một thay đổi link/template (mọi trang đọc lại theo mốc).
UPDATE owner_channels
SET updated_at = clock_timestamp()
WHERE id = $1;

-- name: ReferenceRivalText :many
-- Tiêu đề + mô tả của đối thủ tham chiếu (dòng đầu trong thứ tự liên kết) —
-- đầu vào đoán ngôn ngữ.
SELECT r.id, r.title, r.description
FROM owner_channel_sources s
JOIN rival_channels r ON r.id = s.rival_id
WHERE s.owner_channel_id = $1
ORDER BY s.linked_at, s.rival_id
LIMIT 1;

-- name: RecentRivalTitles :many
-- Vài tiêu đề video gần nhất của đối thủ tham chiếu (position tăng từ mới).
SELECT v.title
FROM rival_videos v
WHERE v.channel_id = $1
ORDER BY v.position, v.id
LIMIT 20;

-- name: ListCompetitors :many
-- Đối thủ liên kết của kênh, thứ tự liên kết — bối cảnh cho prompt nhận diện.
SELECT r.id, r.title, r.handle, r.description, r.avatar_url
FROM owner_channel_sources s
JOIN rival_channels r ON r.id = s.rival_id
WHERE s.owner_channel_id = $1
ORDER BY s.linked_at, s.rival_id;

-- name: AcquireIdentityLease :one
-- Một run mỗi kênh: giành lease chỉ khi không còn lease sống. Token do app
-- cấp để fence mọi ghi của run sau đó.
UPDATE owner_channels SET
    identity_state        = 'running',
    identity_part         = $2,
    identity_task_id      = $3,
    identity_error_code   = NULL,
    identity_error        = NULL,
    identity_failed_step  = NULL,
    identity_retry_at     = NULL,
    identity_started_at   = clock_timestamp(),
    identity_lease_owner  = $4,
    identity_lease_token  = $5,
    identity_lease_until  = $6,
    updated_at            = clock_timestamp()
WHERE id = $1
  AND (identity_lease_token IS NULL OR identity_lease_until <= clock_timestamp())
RETURNING id;

-- name: HeartbeatIdentityLease :execrows
-- Kéo dài lease; false = đã mất quyền run (lease hết hạn/chủ khác).
UPDATE owner_channels
SET identity_lease_until = $3
WHERE id = $1 AND identity_lease_token = $2
  AND identity_lease_until > clock_timestamp();

-- name: LeaseIdentityLive :one
-- Lease còn sống không (mọi chủ) — dùng cho trạng thái hiển thị.
SELECT (identity_lease_token IS NOT NULL AND identity_lease_until > clock_timestamp())::bool AS live
FROM owner_channels WHERE id = $1;

-- name: PublishIdentity :execrows
-- Công bố tên + tài sản + branding trong MỘT ghi, fence kép: còn giữ lease
-- (token + chưa hết hạn) và tên kênh vẫn là tên đọc lúc bắt đầu run — kênh đổi
-- tên hay bị xoá giữa chừng thì 0 dòng, không hồi sinh/không ghi đè.
UPDATE owner_channels SET
    name                  = sqlc.arg(publish_name)::text,
    branding              = sqlc.arg(branding)::jsonb,
    avatar_path           = CASE WHEN sqlc.arg(avatar_path)::text = '' THEN avatar_path ELSE sqlc.arg(avatar_path)::text END,
    banner_path           = CASE WHEN sqlc.arg(banner_path)::text = '' THEN banner_path ELSE sqlc.arg(banner_path)::text END,
    identity_state        = 'idle',
    identity_part         = NULL,
    identity_task_id      = NULL,
    identity_error_code   = NULL,
    identity_error        = NULL,
    identity_failed_step  = NULL,
    identity_retry_at     = NULL,
    identity_started_at   = NULL,
    identity_lease_owner  = NULL,
    identity_lease_token  = NULL,
    identity_lease_until  = NULL,
    updated_at            = clock_timestamp()
WHERE id = sqlc.arg(id)::uuid AND identity_lease_token = sqlc.arg(identity_lease_token)::uuid
  AND identity_lease_until > clock_timestamp()
  AND name = sqlc.arg(expected_name)::text;

-- name: FailIdentity :execrows
-- Ghi lỗi của run (chỉ khi vẫn giữ lease) và NHẢ lease: run kết thúc bằng
-- lỗi thì lần sau được giành lại quyền ngay.
UPDATE owner_channels SET
    identity_state        = 'failed',
    identity_error_code   = $3,
    identity_error        = $4,
    identity_retry_at     = $5,
    identity_failed_step  = $6,
    identity_lease_owner  = NULL,
    identity_lease_token  = NULL,
    identity_lease_until  = NULL,
    updated_at            = clock_timestamp()
WHERE id = $1 AND identity_lease_token = $2;

-- name: AttachIdentityTask :execrows
-- Gắn id việc nền sau khi đã giữ lease (task tạo SAU acquire): chỉ chủ lease
-- được gắn — run đã mất quyền thì 0 dòng.
UPDATE owner_channels
SET identity_task_id = $3
WHERE id = $1 AND identity_lease_token = $2;

-- name: NextIdentityLeaseExpiry :one
-- Hạn lease sớm nhất còn sống — để hẹn reconcile sau startup/ sau acquire.
SELECT coalesce(min(identity_lease_until), 'epoch'::timestamptz)::timestamptz AS until
FROM owner_channels
WHERE identity_lease_token IS NOT NULL
  AND identity_lease_until > clock_timestamp();

-- name: ReleaseIdentityLease :exec
-- Nhả lease của run thành công mà không đụng trạng thái (publish đã xoá cột).
UPDATE owner_channels
SET identity_lease_owner = NULL, identity_lease_token = NULL, identity_lease_until = NULL
WHERE id = $1 AND identity_lease_token = $2;

-- name: MarkInterruptedIdentities :many
-- Phục hồi sau sập/khởi động lại: run còn ghi 'running' nhưng lease đã hết
-- hạn thì đánh dấu hỏng 'interrupted' — không tự vẽ lại ảnh trả phí.
UPDATE owner_channels SET
    identity_state        = 'failed',
    identity_error_code   = 'interrupted',
    identity_error        = 'Lần tạo nhận diện trước dừng giữa chừng.',
    identity_failed_step  = NULL,
    identity_lease_owner  = NULL,
    identity_lease_token  = NULL,
    identity_lease_until  = NULL,
    updated_at            = clock_timestamp()
WHERE identity_state = 'running'
  AND (identity_lease_token IS NULL OR identity_lease_until <= clock_timestamp())
RETURNING id;

-- name: ChannelAssetPaths :one
-- Đường tương đối của hai tài sản đang dùng — GET /branding/{file} kiểm tên
-- tệp được phép phục vụ trước khi mở.
SELECT avatar_path, banner_path FROM owner_channels WHERE id = $1;

-- name: LockChannelForCreate :exec
-- Khoá danh sách đối thủ sẽ link: FOR UPDATE để một rival bị gỡ giữa chừng
-- làm tx tạo kênh hỏng thay vì ghi link chết.
SELECT id FROM rival_channels WHERE id = ANY($1::text[]) ORDER BY id FOR UPDATE;
