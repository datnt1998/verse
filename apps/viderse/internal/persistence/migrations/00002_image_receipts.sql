-- +goose Up
-- Biên nhận của mỗi lần vẽ ảnh trả phí: một dòng = một attempt. Ghi TRƯỚC khi
-- provider được phép tiêu tiền và hoàn tất SAU khi có kết quả, để app chết giữa
-- chừng vẫn biết attempt đó đã tới chỗ trả tiền hay chưa — không bao giờ vẽ lại
-- mù quáng. `charged` mặc định TRUE (thận trọng: đã qua điểm tiêu tiền) và chỉ
-- hạ xuống FALSE khi lỗi thuộc loại chắc chắn chưa tốn ảnh nào (provider bận,
-- từ chối nội dung, hết hạn mức của gói, thiếu đăng nhập).
CREATE TABLE IF NOT EXISTS image_receipts (
    draw_key     text NOT NULL,             -- khoá idempotent do pipeline đặt (vd. generationId)
    attempt      integer NOT NULL,
    provider     text NOT NULL,
    model        text NOT NULL DEFAULT '',
    output_path  text NOT NULL DEFAULT '',
    state        text NOT NULL DEFAULT 'running'
        CHECK (state IN ('running', 'ok', 'failed', 'failed_unknown', 'abandoned')),
    charged      boolean NOT NULL DEFAULT TRUE,
    error_code   text NOT NULL DEFAULT '',
    error_text   text NOT NULL DEFAULT '',
    receipt      jsonb,
    scratch      jsonb,
    started_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz,
    PRIMARY KEY (draw_key, attempt)
);

CREATE INDEX IF NOT EXISTS image_receipts_provider_day_idx
    ON image_receipts (provider, started_at);

-- Tối đa MỘT attempt 'running' cho mỗi key, kể cả giữa hai process: chủ thứ
-- hai đụng unique khi Begin và phải đối chiếu dòng đang chạy thay vì vẽ mù.
CREATE UNIQUE INDEX IF NOT EXISTS image_receipts_one_running_idx
    ON image_receipts (draw_key)
    WHERE state = 'running';

-- Grants mirror deploy/postgres-init; guard cho DB dev chưa có role app.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON image_receipts TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS image_receipts;
