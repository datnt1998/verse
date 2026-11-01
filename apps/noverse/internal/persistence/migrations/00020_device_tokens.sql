-- +goose Up
-- Known devices of an account: a long-lived token minted after a successful
-- credential check. It never authenticates anything by itself — it only tells
-- the login limiter that this browser already proved the account password, so
-- failures aimed at the account from elsewhere cannot lock the owner out.
--
-- Only SHA-256(token) is stored (32 bytes, same shape as session secrets);
-- revocation is revoked_at so the row survives for audit. Expired/revoked rows
-- are pruned by the account flow, exactly like expired sessions.
CREATE TABLE device_tokens (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash          bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at          timestamptz NOT NULL,
    revoked_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (expires_at > created_at)
);
CREATE INDEX device_tokens_user_live ON device_tokens (user_id, expires_at)
    WHERE revoked_at IS NULL;
CREATE INDEX device_tokens_expiry ON device_tokens (expires_at);

-- Same mutable-row guard as every other versioned identity table (00008).
CREATE TRIGGER trg_device_tokens_version_guard BEFORE UPDATE ON device_tokens
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();

-- +goose Down
DROP TABLE IF EXISTS device_tokens;
