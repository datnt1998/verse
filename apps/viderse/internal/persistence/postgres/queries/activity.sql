-- Việc nền (§11): bảng background_tasks chỉ giữ việc chưa kết thúc; kết thúc
-- là DELETE (Finish). notifications giữ lịch sử thông báo đã gộp.

-- name: FindTaskByObject :one
-- Việc còn tồn tại cho một đối tượng (cùng kind + object_ref JSON): mỗi đối
-- tượng có tối đa một việc nền — retry sau failed kế thừa dòng cũ.
SELECT * FROM background_tasks
WHERE kind = sqlc.arg(kind) AND object_ref = sqlc.arg(object_ref)::jsonb
LIMIT 1;

-- name: InsertTask :one
INSERT INTO background_tasks (id, kind, agent, title, object_ref, phase, progress, eta_sec, state, action, owner_pid)
VALUES (gen_random_uuid(), sqlc.arg(kind), sqlc.arg(agent), sqlc.arg(title), sqlc.arg(object_ref)::jsonb,
        sqlc.arg(phase), sqlc.arg(progress)::jsonb, sqlc.arg(eta_sec), 'running', sqlc.arg(action)::jsonb,
        sqlc.arg(owner_pid))
RETURNING *;

-- name: AdoptTask :exec
-- Nhận lại một dòng việc đã có mà giữ nguyên tiến độ đang hiện (ResetTask chỉ
-- dành cho retry sau failed, nơi tiến độ cũ phải bị xoá sạch).
UPDATE background_tasks
SET agent      = sqlc.arg(agent),
    title      = sqlc.arg(title),
    object_ref = sqlc.arg(object_ref)::jsonb,
    phase      = sqlc.arg(phase),
    progress   = COALESCE(sqlc.narg(progress)::jsonb, progress),
    eta_sec    = sqlc.arg(eta_sec),
    state      = sqlc.arg(state),
    action     = sqlc.arg(action)::jsonb,
    owner_pid  = sqlc.arg(owner_pid),
    updated_at = now()
WHERE id = sqlc.arg(id)::uuid;

-- name: UpdateTask :execrows
-- Trường nil giữ giá trị cũ; state rỗng giữ trạng thái cũ. Chỉ chạm dòng còn
-- tồn tại: việc đã Finish không hồi sinh.
UPDATE background_tasks
SET phase      = COALESCE(sqlc.narg(phase), phase),
    progress   = COALESCE(sqlc.narg(progress)::jsonb, progress),
    eta_sec    = COALESCE(sqlc.narg(eta_sec), eta_sec),
    state      = COALESCE(NULLIF(sqlc.narg(state), ''), state),
    action     = COALESCE(sqlc.narg(action)::jsonb, action),
    updated_at = now()
WHERE id = sqlc.arg(id)::uuid;

-- name: DeleteTask :exec
DELETE FROM background_tasks WHERE id = sqlc.arg(id)::uuid;

-- name: ListTasks :many
SELECT * FROM background_tasks ORDER BY created_at;
-- name: ResetTask :exec
-- Viết lại toàn bộ nội dung một việc (retry sau failed): trạng thái về
-- running, các trường tiến độ cũ bị ghi đè sạch.
UPDATE background_tasks
SET agent      = sqlc.arg(agent),
    title      = sqlc.arg(title),
    object_ref = sqlc.arg(object_ref)::jsonb,
    phase      = sqlc.arg(phase),
    progress   = sqlc.arg(progress)::jsonb,
    eta_sec    = sqlc.arg(eta_sec),
    state      = 'running',
    action     = sqlc.arg(action)::jsonb,
    owner_pid  = sqlc.arg(owner_pid),
    updated_at = now()
WHERE id = sqlc.arg(id)::uuid;

-- name: CountTasks :one
-- Đếm theo (kind, state); kind rỗng thì khớp mọi loại.
SELECT COUNT(*)::bigint FROM background_tasks
WHERE (sqlc.arg(kind) = '' OR kind = sqlc.arg(kind)) AND state = sqlc.arg(state);

-- name: FailTasksByIDs :many
-- Phục hồi sau sập: chỉ dòng của process đã chết (owner_pid do Go lọc theo
-- kill(pid,0)) chuyển thành failed với Thử lại — không đụng bản serve đang sống.
UPDATE background_tasks
SET state      = 'failed',
    phase      = 'Bị gián đoạn khi app khởi động lại',
    action     = jsonb_build_object('label', 'Thử lại', 'op', 'retry'),
    updated_at = now()
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND state IN ('running', 'paused', 'waiting')
RETURNING *;

-- name: GetTask :one
SELECT * FROM background_tasks WHERE id = sqlc.arg(id)::uuid;

-- name: InsertErrorEvent :exec
INSERT INTO error_events (kind, what, why, href, action_label, op, object_ref, audience)
VALUES (sqlc.arg(kind), sqlc.arg(what), sqlc.arg(why), sqlc.arg(href), sqlc.arg(action_label), sqlc.arg(op), sqlc.narg(object_ref)::jsonb, sqlc.arg(audience));

-- name: ListErrorEvents :many
SELECT * FROM error_events
WHERE at >= sqlc.arg(since)
  AND (sqlc.arg(admin)::boolean OR (audience = 'all' AND COALESCE(object_ref->>'type', '') <> 'storage'))
ORDER BY at DESC LIMIT sqlc.arg(row_limit);

-- name: CountErrorEvents :one
SELECT COUNT(*)::bigint FROM error_events
WHERE at >= sqlc.arg(since)
  AND (sqlc.arg(admin)::boolean OR (audience = 'all' AND COALESCE(object_ref->>'type', '') <> 'storage'));

-- name: LatestNotification :one
-- Thông báo mới nhất của một kind để áp luật gộp 10 giây của §11.
SELECT * FROM notifications
WHERE kind = sqlc.arg(kind) AND audience = sqlc.arg(audience)
  AND object_ref IS NOT DISTINCT FROM sqlc.narg(object_ref)::jsonb
ORDER BY created_at DESC
LIMIT 1;

-- name: BumpNotification :exec
-- Gộp: tăng count và cập nhật nội dung mới nhất; giữ created_at của lần đầu.
UPDATE notifications
SET count = count + 1, text = sqlc.arg(text), href = sqlc.arg(href),
    action_label = sqlc.arg(action_label), read_at = NULL
WHERE id = sqlc.arg(id)::uuid;

-- name: InsertNotification :one
INSERT INTO notifications (id, kind, count, text, href, action_label, audience, object_ref)
VALUES (gen_random_uuid(), sqlc.arg(kind), 1, sqlc.arg(text), sqlc.arg(href), sqlc.arg(action_label), sqlc.arg(audience), sqlc.narg(object_ref)::jsonb)
RETURNING *;

-- name: ListUnreadNotifications :many
SELECT * FROM notifications
WHERE read_at IS NULL
  AND (sqlc.arg(admin)::boolean OR (audience = 'all' AND COALESCE(object_ref->>'type', '') <> 'storage'))
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: MarkNotificationsRead :exec
-- Đánh dấu đã đọc; danh sách rỗng (jsonb '[]') nghĩa là "tất cả".
UPDATE notifications
SET read_at = now()
WHERE read_at IS NULL
  AND (sqlc.arg(admin)::boolean OR (audience = 'all' AND COALESCE(object_ref->>'type', '') <> 'storage'))
  AND (sqlc.arg(ids)::jsonb = '[]'::jsonb
       OR id::text IN (SELECT jsonb_array_elements_text(sqlc.arg(ids)::jsonb)));
