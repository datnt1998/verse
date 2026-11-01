-- +goose Up
-- Nhận xét đã áp dụng: nhận xét tính lại từ số liệu mỗi lần nên chỉ cần nhớ
-- id ổn định của nhận xét và kết quả (quy tắc đã học hoặc template chạy nhiều hơn).
CREATE TABLE performance_insights_applied (
    channel_id uuid NOT NULL REFERENCES owner_channels(id) ON DELETE CASCADE,
    insight_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('rule','rotation')),
    rule_id text,
    applied_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, insight_id)
);
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON performance_insights_applied TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE performance_insights_applied;
