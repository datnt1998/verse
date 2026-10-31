-- +goose Up
ALTER TABLE sessions
    ADD COLUMN revoked_reason text NOT NULL DEFAULT 'unknown'
        CHECK (revoked_reason IN ('unknown', 'logout', 'password_changed', 'admin_changed', 'locked')),
    ADD COLUMN revoked_app text
        CHECK (revoked_app IN ('noverse', 'viderse'));

-- +goose Down
ALTER TABLE sessions DROP COLUMN revoked_app, DROP COLUMN revoked_reason;
