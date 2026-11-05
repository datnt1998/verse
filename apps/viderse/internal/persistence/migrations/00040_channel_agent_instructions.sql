-- +goose Up
-- "Dặn trợ lý về kênh này" — chỉ dẫn riêng theo kênh và theo trợ lý
-- (Trinh sát · Đạo diễn · Hoạ sĩ · Kiểm định). Chỉ lưu và đọc lại; KHÔNG gọi
-- model khi ghi, KHÔNG đụng snapshot video đang chạy. Chỉ dẫn áp cho video
-- được nhận vào hàng chờ từ lúc lưu trở đi (chốt vào clone_jobs.instructions
-- tại admission).
--
-- Khác Giọng văn (owner_channels.style_note) và Mô tả kênh
-- (branding.brief): đây là chỉ dẫn theo từng trợ lý, khoá là id trợ lý.

ALTER TABLE owner_channels
  ADD COLUMN agent_instructions jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose StatementBegin
DO $$
BEGIN
  ALTER TABLE owner_channels
    ADD CONSTRAINT owner_channels_agent_instructions_object
    CHECK (jsonb_typeof(agent_instructions) = 'object');
END $$;
-- +goose StatementEnd

-- Bản chốt lúc bắt đầu của một job: chỉ dẫn đọc từ kênh tại admission, để sửa
-- kênh sau đó không đổi video đã bắt đầu (cùng luật với template_snapshot).
ALTER TABLE clone_jobs
  ADD COLUMN instructions jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE clone_jobs DROP COLUMN instructions;
ALTER TABLE owner_channels DROP CONSTRAINT owner_channels_agent_instructions_object;
ALTER TABLE owner_channels DROP COLUMN agent_instructions;
