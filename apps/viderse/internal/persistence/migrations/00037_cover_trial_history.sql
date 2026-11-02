-- Lịch sử vẽ thử ảnh (Template.dc.html › lib^cover "Vẽ thử"; Library.dc.html
-- › view=editor ảnh mẫu). Mỗi hàng là MỘT lần vẽ thật; media_path trỏ tới blob
-- đã bind qua media.Publish nên blob không bị GC quét nhầm. prompt_id là text
-- vì prompt có sẵn ("builtin:…") không có hàng trong image_prompts.

-- +goose Up
CREATE TABLE IF NOT EXISTS cover_trials (
    id              uuid PRIMARY KEY,
    template_id     uuid NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    job_id          uuid NOT NULL,
    job_title       text NOT NULL DEFAULT '',
    prompt_id       text NOT NULL,
    prompt_name     text NOT NULL DEFAULT '',
    composition     text NOT NULL,
    hook_text       text NOT NULL DEFAULT '',
    state           text NOT NULL CHECK (state IN ('running', 'ready', 'failed', 'cancelled')),
    media_path      text,
    provider        text NOT NULL DEFAULT '',
    model           text NOT NULL DEFAULT '',
    width           integer NOT NULL DEFAULT 0,
    height          integer NOT NULL DEFAULT 0,
    cost_images     integer NOT NULL DEFAULT 0 CHECK (cost_images IN (0, 1)),
    error_code      text,
    error_message   text,
    error_recovery  text,
    retry_at        timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    finished_at     timestamptz,
    CONSTRAINT cover_trials_composition_check
        CHECK (composition IN ('clean', 'hook', 'circle-arrow', 'hook-arrow', 'split', 'closeup')),
    CONSTRAINT cover_trials_ready_has_media CHECK (state <> 'ready' OR media_path IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS cover_trials_template_idx
    ON cover_trials (template_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS image_prompt_samples (
    id              uuid PRIMARY KEY,
    prompt_id       text NOT NULL,
    prompt_name     text NOT NULL DEFAULT '',
    prompt_kind     text NOT NULL CHECK (prompt_kind IN ('scene', 'cover')),
    composition     text,
    video_id        uuid NOT NULL,
    video_title     text NOT NULL DEFAULT '',
    scene_id        text NOT NULL DEFAULT '',
    hook_text       text NOT NULL DEFAULT '',
    state           text NOT NULL CHECK (state IN ('running', 'ready', 'failed', 'cancelled')),
    media_path      text,
    provider        text NOT NULL DEFAULT '',
    model           text NOT NULL DEFAULT '',
    width           integer NOT NULL DEFAULT 0,
    height          integer NOT NULL DEFAULT 0,
    cost_images     integer NOT NULL DEFAULT 0 CHECK (cost_images IN (0, 1)),
    error_code      text,
    error_message   text,
    error_recovery  text,
    retry_at        timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    finished_at     timestamptz,
    CONSTRAINT image_prompt_samples_composition_check
        CHECK (composition IS NULL OR composition IN ('clean', 'hook', 'circle-arrow', 'hook-arrow', 'split', 'closeup')),
    CONSTRAINT image_prompt_samples_cover_has_composition
        CHECK ((prompt_kind = 'cover') = (composition IS NOT NULL)),
    CONSTRAINT image_prompt_samples_ready_has_media CHECK (state <> 'ready' OR media_path IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS image_prompt_samples_prompt_idx
    ON image_prompt_samples (prompt_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS image_prompt_samples;
DROP TABLE IF EXISTS cover_trials;
