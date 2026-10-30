-- +goose Up
-- Kết nối AI (hợp đồng dữ liệu §15): nhà cung cấp model và danh mục đã lấy.
-- Khoá API KHÔNG bao giờ ở dạng plaintext — key_enc là ciphertext AES-GCM của
-- internal/secretbox (khoá máy ở ~/.viderse/secret.key); DB chỉ giữ key_hint.
-- chatgpt/grok là hai thẻ do app quản (đăng nhập gói qua core/auth): luôn có
-- một dòng mỗi loại, không tạo/xoá qua API thường.
CREATE TABLE IF NOT EXISTS ai_providers (
    id           text PRIMARY KEY,
    kind         text NOT NULL
        CHECK (kind IN ('chatgpt','grok','openai','gemini','openrouter',
                        'deepseek','xai','anthropic','ollama','openai_compat')),
    auth         text NOT NULL
        CHECK (auth IN ('subscription','api_key','local')),
    label        text NOT NULL DEFAULT '',
    key_enc      bytea,
    key_hint     text NOT NULL DEFAULT '',
    base_url     text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'ready'
        CHECK (status IN ('ready','logged_out','login_pending','auth_failed','unreachable')),
    catalog_at   timestamptz,
    selected     jsonb NOT NULL DEFAULT '[]',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Mỗi loại đăng nhập gói chỉ một thẻ (credential sống trong auth.json của app).
CREATE UNIQUE INDEX IF NOT EXISTS ai_providers_subscription_kind_idx
    ON ai_providers (kind)
    WHERE auth = 'subscription';

CREATE TABLE IF NOT EXISTS ai_models (
    provider_id  text NOT NULL REFERENCES ai_providers (id) ON DELETE CASCADE,
    id           text NOT NULL,
    caps         jsonb NOT NULL DEFAULT '[]',
    context_k    integer,
    in_per_m     double precision,
    out_per_m    double precision,
    per_image    double precision,
    recommended  boolean NOT NULL DEFAULT FALSE,
    manual       boolean NOT NULL DEFAULT FALSE,
    PRIMARY KEY (provider_id, id)
);

CREATE INDEX IF NOT EXISTS ai_models_provider_idx ON ai_models (provider_id);

-- Hai thẻ quản lý + các model mặc định để Trợ lý AI và ModelPicker có dữ liệu
-- trước lần lấy danh mục đầu tiên (catalog_at để trống = chưa lấy thật).
INSERT INTO ai_providers (id, kind, auth, label, status, selected)
VALUES
    ('chatgpt', 'chatgpt', 'subscription', 'ChatGPT', 'logged_out',
     '["gpt-6.1-sol","gpt-image-2"]'),
    ('grok', 'grok', 'subscription', 'Grok', 'logged_out',
     '["grok-imagine-image"]')
ON CONFLICT (id) DO NOTHING;

INSERT INTO ai_models (provider_id, id, caps, context_k, recommended)
VALUES
    ('chatgpt', 'gpt-6.1-sol', '["text","vision"]', 272, TRUE),
    ('chatgpt', 'gpt-image-2', '["image"]', NULL, TRUE),
    ('grok', 'grok-imagine-image', '["image"]', NULL, TRUE)
ON CONFLICT (provider_id, id) DO NOTHING;

-- Grants mirror deploy/postgres-init; guard cho DB dev chưa có role app.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON ai_providers, ai_models TO viderse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS ai_models;
DROP TABLE IF EXISTS ai_providers;
