-- Biên nhận ảnh trả phí: ghi trước khi tiêu tiền, hoàn tất sau khi vẽ.

-- name: BeginImageReceipt :one
-- Mở attempt mới cho draw_key: attempt = số attempt đã ghi + 1, nên retry chủ
-- động sau một attempt failed vẫn kế thừa đúng chi phí ngày (attempt-1 attempt
-- trước nó). Hai process cùng bắt đầu một (key, attempt) đụng PK — caller coi
-- đó là xung đột.
INSERT INTO image_receipts (draw_key, attempt, provider, model, output_path, state, charged, started_at)
VALUES (
    $1,
    (SELECT COALESCE(MAX(attempt), 0) + 1 FROM image_receipts WHERE draw_key = $1),
    $2, $3, $4, 'running', TRUE, $5
)
RETURNING *;

-- name: ImageReceiptScratch :exec
-- Cập nhật scratch của attempt đang chạy (con trỏ phục hồi: work dir, owner…).
-- Chỉ chạm dòng còn 'running' — attempt đã hoàn tất là bất biến.
UPDATE image_receipts
SET scratch = $3
WHERE draw_key = $1 AND attempt = $2 AND state = 'running';

-- name: CompleteImageReceipt :execrows
-- Chỉ chủ đang giữ attempt 'running' mới kết luận được: attempt đã bị process
-- khác hoàn tất thì câu này không chạm dòng nào (caller coi là xung đột, không
-- ghi đè quyết định của chủ trước).
UPDATE image_receipts
SET state = $3, charged = $4, error_code = $5, error_text = $6,
    receipt = $7, finished_at = $8
WHERE draw_key = $1 AND attempt = $2 AND state = 'running';

-- name: LatestImageReceipt :one
SELECT * FROM image_receipts
WHERE draw_key = $1 AND state <> 'abandoned'
ORDER BY attempt DESC
LIMIT 1;

-- name: AbandonImageReceipts :exec
-- Chủ hữu quyết định vẽ lại một key đã failed_unknown/failed: bản ghi giữ làm
-- bằng chứng nhưng không còn chặn latest.
UPDATE image_receipts
SET state = 'abandoned'
WHERE draw_key = $1 AND state IN ('failed', 'failed_unknown');

-- name: CountChargedImageReceipts :one
SELECT COUNT(*)::bigint FROM image_receipts
WHERE provider = $1 AND charged AND started_at >= $2;
