-- Truy vấn phụ của mảng Lưu trữ. Vị trí/bản cũ/dọn dẹp nằm trong settings;
-- bảng duy nhất cần đọc là biên nhận ảnh để biết thư mục tạm nào là phiên
-- phục hồi (không được dọn).

-- name: UnfinishedImageReceiptScratch :many
-- Scratch (work dir, owner…) của mọi lần vẽ ảnh còn 'running': thư mục tạm
-- chúng trỏ tới là dữ liệu phục hồi, bộ dọn phải bỏ qua.
SELECT scratch
FROM image_receipts
WHERE state = 'running' AND scratch IS NOT NULL;
