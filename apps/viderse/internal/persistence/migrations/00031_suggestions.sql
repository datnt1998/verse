-- +goose Up
-- Đề xuất của trợ lý. Một dòng cho mỗi khoá trùng (kind + kênh + đối tượng):
-- bỏ qua hoặc nhận thì cooldown_until = +7 ngày, hết hạn đó dòng mới được mở lại.
CREATE TABLE suggestions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL CHECK (kind IN ('hot_video','template_failing','learn_rule','raise_autonomy','pace','perf_insight','cleanup')),
    agent text NOT NULL CHECK (agent IN ('scout','director','artist','qa')),
    channel_id uuid REFERENCES owner_channels(id) ON DELETE CASCADE,
    object text NOT NULL,
    title text NOT NULL,
    reason jsonb NOT NULL,
    primary_action jsonb NOT NULL,
    value integer NOT NULL DEFAULT 0,
    state text NOT NULL DEFAULT 'open' CHECK (state IN ('open','accepted','dismissed','expired')),
    cooldown_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    resolved_at timestamptz
);
CREATE UNIQUE INDEX suggestions_dedupe_idx ON suggestions(kind, COALESCE(channel_id, '00000000-0000-0000-0000-000000000000'::uuid), object);
CREATE INDEX suggestions_open_idx ON suggestions(value DESC, created_at DESC) WHERE state = 'open';
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
  GRANT SELECT, INSERT, UPDATE, DELETE ON suggestions TO viderse_app;
 END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE suggestions;
