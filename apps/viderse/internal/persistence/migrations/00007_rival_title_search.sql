-- +goose Up
-- Tìm tiêu đề video đối thủ không phân biệt hoa thường và dấu: "ngoi nha"
-- khớp "Ngôi Nhà", "duong" khớp "Đường". title_fold là tiêu đề đã gấp (chữ
-- thường, tách dấu NFD rồi bỏ dấu kết hợp, đ → d), tính một lần khi ghi; câu
-- tìm gấp chuỗi tìm bằng cùng hàm nên hai phía luôn khớp quy tắc.
--
-- NFD (không phải NFKD) để gấp không bao giờ sinh ra %, _ hay \ từ ký tự
-- khác: mẫu LIKE đã escape ở Go vẫn đúng sau khi gấp.
CREATE FUNCTION fold_title(t text) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN translate(regexp_replace(normalize(lower(t), NFD), '[\u0300-\u036f]', '', 'g'), 'đð', 'dd');

ALTER TABLE rival_videos
    ADD COLUMN title_fold text GENERATED ALWAYS AS (fold_title(title)) STORED;

DROP INDEX IF EXISTS rival_videos_title_trgm_idx;
CREATE INDEX rival_videos_title_fold_trgm_idx
    ON rival_videos USING gin (title_fold gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS rival_videos_title_fold_trgm_idx;
ALTER TABLE rival_videos DROP COLUMN IF EXISTS title_fold;
DROP FUNCTION IF EXISTS fold_title(text);
CREATE INDEX IF NOT EXISTS rival_videos_title_trgm_idx
    ON rival_videos USING gin (title gin_trgm_ops);
