-- +goose Up
-- Quy tắc đã học: chỉ có hiệu lực khi người dùng bấm Lưu (không học ngầm).
-- Quy tắc của Đạo diễn thuộc một kênh; ghi chú của Hoạ sĩ thuộc một prompt ảnh
-- (id có sẵn như "builtin:scene" hoặc id prompt của bạn), nên không có khoá ngoại.
CREATE TABLE learned_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id uuid REFERENCES owner_channels(id) ON DELETE CASCADE,
    image_prompt_id text,
    agent text NOT NULL CHECK (agent IN ('director','artist')),
    text text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 200),
    pattern_key text,
    source text NOT NULL CHECK (source IN ('edits','redraw','performance','assistant','manual')),
    source_label text NOT NULL DEFAULT '',
    learned_from jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((channel_id IS NOT NULL) <> (image_prompt_id IS NOT NULL))
);
CREATE UNIQUE INDEX learned_rules_channel_text_idx ON learned_rules(channel_id, lower(text)) WHERE channel_id IS NOT NULL;
CREATE UNIQUE INDEX learned_rules_prompt_text_idx ON learned_rules(image_prompt_id, lower(text)) WHERE image_prompt_id IS NOT NULL;
CREATE INDEX learned_rules_channel_idx ON learned_rules(channel_id, created_at);
CREATE INDEX learned_rules_prompt_idx ON learned_rules(image_prompt_id, created_at);

-- Một dòng mỗi (video, phần, kiểu sửa): so bản Đạo diễn viết với bản chủ kênh
-- sửa. Sửa lại cùng video thì thay các dòng của video đó.
CREATE TABLE learn_edits (
    job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES owner_channels(id) ON DELETE CASCADE,
    part text NOT NULL CHECK (part IN ('title','description','tags')),
    key text NOT NULL,
    before_text text NOT NULL,
    after_text text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, part, key)
);
CREATE INDEX learn_edits_channel_key_idx ON learn_edits(channel_id, key, created_at);

-- Góp ý vẽ lại của chủ kênh, gom theo prompt ảnh. norm là câu đã chuẩn hoá
-- (model nếu có, nếu không thì bằng code) để gom các góp ý cùng nghĩa.
CREATE TABLE learn_feedback (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES owner_channels(id) ON DELETE CASCADE,
    image_prompt_id text NOT NULL,
    text text NOT NULL,
    norm text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX learn_feedback_prompt_idx ON learn_feedback(image_prompt_id, norm, created_at);

-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
  GRANT SELECT, INSERT, UPDATE, DELETE ON learned_rules, learn_edits, learn_feedback TO viderse_app;
 END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE learn_feedback;
DROP TABLE learn_edits;
DROP TABLE learned_rules;
