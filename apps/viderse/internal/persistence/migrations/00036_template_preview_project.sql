-- +goose Up
-- Bản xem thử là một project editor (bản sao project template đã đổ nội dung
-- của một video xong), không phải video dựng sẵn. Giữ mã project + job nguồn
-- + số phút; preview_revision đã có từ 00008 quyết định còn mới hay đã cũ.
ALTER TABLE templates
    ADD COLUMN preview_project_id text,
    ADD COLUMN preview_job_id uuid,
    ADD COLUMN preview_minutes integer CHECK (preview_minutes IS NULL OR preview_minutes BETWEEN 1 AND 10),
    -- Token của lượt dựng đang giữ quyền công bố. Lượt cũ (kể cả của process
    -- trước) không thể ghi đè project/revision của lượt mới.
    ADD COLUMN preview_run_id uuid,
    -- Lỗi có hành động của lần dựng gần nhất. Bản xem thử cũ vẫn giữ nguyên.
    ADD COLUMN preview_error jsonb;

-- Danh mục project xem thử: độc lập với con trỏ preview_project_id để project
-- vừa tạo luôn bị ẩn khỏi Editor › Projects, kể cả khi crash trước khi công
-- bố, khi xoá project best effort thất bại, hoặc khi template đã bị xoá.
CREATE TABLE template_preview_projects (
    project_id text PRIMARY KEY,
    -- Không cascade: xoá template vẫn phải giữ dấu để ẩn/dọn project mồ côi.
    template_id uuid REFERENCES templates(id) ON DELETE SET NULL,
    job_id uuid,
    minutes integer CHECK (minutes IS NULL OR minutes BETWEEN 1 AND 10),
    built_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX template_preview_projects_template_idx ON template_preview_projects(template_id);

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON template_preview_projects TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

INSERT INTO template_preview_projects(project_id, template_id, job_id, minutes, built_at)
SELECT preview_project_id, id, preview_job_id, preview_minutes, preview_built_at
FROM templates
WHERE preview_project_id IS NOT NULL;

-- +goose Down
DROP TABLE template_preview_projects;
ALTER TABLE templates
    DROP COLUMN preview_error,
    DROP COLUMN preview_run_id,
    DROP COLUMN preview_minutes,
    DROP COLUMN preview_job_id,
    DROP COLUMN preview_project_id;
