-- +goose Up
-- Audio script finite-state + digest CHECKs. The columns stay text (the repo
-- does not use native Postgres enums); the constraint names are explicit so
-- operator errors are readable. Repository boundary validation returns the
-- domain error first; these are the integrity backstop.
--
-- Status values mirror internal/audio/script.go:
--   todo, draft, review, locked, stale
-- source_sha256 is the canonical lowercase SHA-256 hex of the accepted
-- chapter revision (same rule as blobs.sha256 / audio_renders.synth_key).
ALTER TABLE audio_scripts
    ADD CONSTRAINT audio_scripts_status_check
        CHECK (status IN ('todo', 'draft', 'review', 'locked', 'stale')),
    ADD CONSTRAINT audio_scripts_source_sha256_check
        CHECK (source_sha256 ~ '^[0-9a-f]{64}$');

-- +goose Down
-- Down is for throwaway databases only.
ALTER TABLE audio_scripts
    DROP CONSTRAINT IF EXISTS audio_scripts_source_sha256_check,
    DROP CONSTRAINT IF EXISTS audio_scripts_status_check;
