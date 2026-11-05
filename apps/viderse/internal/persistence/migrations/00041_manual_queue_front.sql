-- +goose Up
-- Các mục người dùng đã thêm (kể cả mục có thể Hoàn tác) không còn
-- chịu nhịp Tự lấy. Giữ thứ tự cũ, không sửa created_at hoặc job đã admission.
-- Các mục auto từng được Lên đầu cũng giữ vị trí trong cùng hàng manual.
SELECT pg_advisory_xact_lock(27900503);

WITH manual AS (
  SELECT id, row_number() OVER (
    ORDER BY CASE WHEN priority = 'pick' THEN 0 ELSE 1 END,
             position, created_at, id
  )::double precision AS new_position
  FROM clone_queue
  WHERE state = 'queued' AND (priority = 'pick' OR origin <> 'auto')
)
UPDATE clone_queue q SET priority = 'pick', position = manual.new_position
FROM manual WHERE q.id = manual.id;

ALTER TABLE clone_queue ALTER COLUMN priority SET DEFAULT 'pick';
ALTER TABLE clone_queue ALTER COLUMN origin SET DEFAULT 'manual';

-- +goose Down
-- Chỉ hoàn lại default schema. Không thể suy ngược ý định manual cũ sau khi
-- hệ thống đã nhận request V39; không coi Down là rollback dữ liệu lossless.
ALTER TABLE clone_queue ALTER COLUMN priority SET DEFAULT 'batch';
ALTER TABLE clone_queue ALTER COLUMN origin SET DEFAULT 'bulk';
