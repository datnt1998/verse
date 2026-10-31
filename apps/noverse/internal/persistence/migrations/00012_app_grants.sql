-- +goose Up
-- Per-app grants of the shared account store. A row grants exactly membership
-- of one app; admin of an app is inherited from users.global_role = 'admin',
-- so no app-admin row or separate partition exists.
--
-- Backfill: every live account keeps its Noverse access, including locked
-- accounts (unlocking must not silently lose the permission). Viderse is NOT
-- backfilled: it is granted intentionally after cutover. Soft-deleted
-- tombstones keep their username reserved but get no grant.
CREATE TABLE app_grants (
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    app         text NOT NULL CHECK (app IN ('noverse','viderse')),
    role        text NOT NULL CHECK (role = 'member'),
    granted_by  uuid REFERENCES users(id) ON DELETE RESTRICT,
    granted_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, app)
);
CREATE INDEX app_grants_by_app ON app_grants (app);

INSERT INTO app_grants (user_id, app, role)
SELECT id, 'noverse', 'member' FROM users WHERE deleted_at IS NULL;

-- +goose Down
-- Throwaway databases only; the grant table is additive and rows must survive a
-- code rollback on a database with writers.
DROP TABLE IF EXISTS app_grants;
