-- Kênh của bạn: kênh + đối thủ liên kết + template mặc định trong Thư viện +
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
INSERT INTO owner_channels (id, name, youtube_url, language, auto_fetch, per_day, auto_approve, default_template_id, agent_instructions)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb);

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
SELECT s.owner_channel_id, s.rival_id, s.linked_at, s.min_duration_sec, r.title, r.handle
FROM owner_channel_sources s
JOIN rival_channels r ON r.id = s.rival_id
WHERE s.owner_channel_id = ANY($1::uuid[])
ORDER BY s.owner_channel_id, s.linked_at, s.rival_id;

-- name: SetSourceMinDuration :execrows
-- Bộ lọc Tự lấy theo đối thủ (giây; NULL = không lọc).
UPDATE owner_channel_sources SET min_duration_sec = sqlc.narg(min_duration_sec)::int
WHERE owner_channel_id = sqlc.arg(owner_channel_id)::uuid AND rival_id = sqlc.arg(rival_id)::text;

-- name: ListChannelDefaults :many
-- Template mặc định của nhiều kênh trong một truy vấn (bỏ kênh không có).
SELECT c.id AS owner_channel_id, t.id AS template_id, t.name
FROM owner_channels c
JOIN templates t ON t.id = c.default_template_id
WHERE c.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY c.id;

-- name: DefaultTemplate :one
-- Template mặc định hiện tại của một kênh.
SELECT t.id, t.name
FROM owner_channels c
JOIN templates t ON t.id = c.default_template_id
WHERE c.id = sqlc.arg(channel_id)::uuid;

-- name: TemplateNames :many
-- Tên template theo id (cho override của hàng chờ và job đang chạy).
SELECT id, name FROM templates WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id;

-- name: LockChannelForAdmission :one
-- Khoá hàng kênh trong transaction admission và trả template mặc định + chỉ
-- dẫn "Dặn trợ lý về kênh" đang có hiệu lực tại thời điểm commit. Cùng một
-- khoá với PATCH kênh nên không thể chốt nửa mặc định/nửa chỉ dẫn.
SELECT default_template_id, agent_instructions FROM owner_channels WHERE id = sqlc.arg(id)::uuid FOR UPDATE;

-- name: TemplateRevisionForShare :one
-- Fence revision của template: job chỉ được persist đúng revision đã build
-- snapshot. SHARE chặn sửa revision và xoá cho tới khi admission commit.
SELECT revision FROM templates WHERE id = sqlc.arg(id)::uuid FOR SHARE;

-- name: ChannelAutoApprove :one
-- Cờ Tự duyệt tại thời điểm xét (đọc lại trước khi duyệt để toggle tắt giữa
-- lượt poll không bị bỏ qua).
SELECT auto_approve FROM owner_channels WHERE id = sqlc.arg(id)::uuid;

-- name: PatchChannel :execrows
-- Cập nhật đúng các trường được đưa (mỗi trường một cờ *_set); branding chỉ
-- đổi khi cần sửa brief của nhận diện. default_template_id là NOT NULL nên
-- null/"" bị chặn ở tầng service trước khi tới đây.
WITH reset_auto_baselines AS (
    UPDATE owner_channel_sources SET auto_baseline_at = NULL
    WHERE owner_channel_id = sqlc.arg(id)::uuid
      AND sqlc.arg(auto_fetch_set)::bool AND NOT sqlc.arg(auto_fetch)::bool
)
UPDATE owner_channels SET
    name                = CASE WHEN sqlc.arg(name_set)::bool           THEN sqlc.narg(name)::text          ELSE name END,
    youtube_url         = CASE WHEN sqlc.arg(youtube_url_set)::bool    THEN sqlc.narg(youtube_url)::text   ELSE youtube_url END,
    language            = CASE WHEN sqlc.arg(language_set)::bool       THEN sqlc.arg(language)::text       ELSE language END,
    style_note          = CASE WHEN sqlc.arg(style_note_set)::bool     THEN sqlc.narg(style_note)::text    ELSE style_note END,
    description_footer  = CASE WHEN sqlc.arg(footer_set)::bool         THEN sqlc.narg(footer)::text        ELSE description_footer END,
    always_tags         = CASE WHEN sqlc.arg(tags_set)::bool           THEN sqlc.arg(tags)::text[]         ELSE always_tags END,
    auto_fetch          = CASE WHEN sqlc.arg(auto_fetch_set)::bool     THEN sqlc.arg(auto_fetch)::bool     ELSE auto_fetch END,
    auto_approve        = CASE WHEN sqlc.arg(auto_approve_set)::bool   THEN sqlc.arg(auto_approve)::bool   ELSE auto_approve END,
    per_day             = CASE WHEN sqlc.arg(per_day_set)::bool        THEN sqlc.arg(per_day)::int          ELSE per_day END,
    default_template_id = CASE WHEN sqlc.arg(default_template_set)::bool THEN sqlc.arg(default_template_id)::uuid ELSE default_template_id END,
    -- Chỉ dẫn "Dặn trợ lý về kênh": gộp đúng một trợ lý (giữ các trợ lý khác);
    -- giá trị rỗng xoá khoá. Cả câu lệnh là một UPDATE nên nguyên tử.
    agent_instructions  = CASE WHEN sqlc.arg(agent_set)::bool
                               THEN CASE WHEN COALESCE(sqlc.narg(agent_value)::text, '') = ''
                                         THEN agent_instructions - sqlc.arg(agent_key)::text
                                         ELSE agent_instructions || jsonb_build_object(sqlc.arg(agent_key)::text, sqlc.narg(agent_value)::text) END
                               ELSE agent_instructions END,
    branding            = CASE WHEN sqlc.arg(branding_set)::bool
                               THEN jsonb_set(branding, '{brief}', to_jsonb(sqlc.arg(brief)::text), true)
                               ELSE branding END,
    updated_at          = clock_timestamp()
WHERE id = sqlc.arg(id)::uuid;

-- name: DeleteChannel :execrows
DELETE FROM owner_channels WHERE id = $1;

-- name: LockOwnerChannel :one
-- Khoá hàng kênh trong transaction trước khi đọc-đổi-ghi link/template —
-- serialize hai thao tác đồng thời trên cùng kênh.
SELECT id FROM owner_channels WHERE id = $1 FOR UPDATE;

-- name: SetAutoFetch :execrows
-- Bật/tắt Tự lấy; tắt khi bỏ liên kết đối thủ cuối cùng.
WITH reset_auto_baselines AS (
    UPDATE owner_channel_sources SET auto_baseline_at = NULL
    WHERE owner_channel_id = sqlc.arg(id)::uuid AND NOT sqlc.arg(auto_fetch)::bool
)
UPDATE owner_channels
SET auto_fetch = sqlc.arg(auto_fetch)::bool, updated_at = clock_timestamp()
WHERE id = sqlc.arg(id)::uuid;

-- name: SetAutoFetchTouch :execrows
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
