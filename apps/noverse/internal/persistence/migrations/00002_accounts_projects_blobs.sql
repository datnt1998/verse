-- +goose Up
-- Phase 04: identity domain — users/credentials/identities, temporary and
-- reset credentials, devices/sessions (token hashes only), pairings,
-- recovery codes, shared auth rate-limit windows, the singleton
-- auth_invariant_lock, projects/project_members (exactly one owner row
-- per project via partial unique index), audit_logs (append-only) and
-- blobs metadata. DDL follows reports/schema-and-contracts.md §2/§7.
-- All story-domain tables arrive in 00003.

CREATE TABLE users (
    id                  uuid PRIMARY KEY,
    username            citext NOT NULL UNIQUE,
    email               citext UNIQUE,
    display_name        text NOT NULL,
    global_role         text NOT NULL CHECK (global_role IN ('admin','member')),
    status              text NOT NULL CHECK (status IN ('active','locked')),
    must_change_password boolean NOT NULL DEFAULT false,
    onboarded_at        timestamptz,
    disabled_at         timestamptz,
    disabled_by         uuid REFERENCES users(id) ON DELETE RESTRICT,
    disabled_reason     text NOT NULL DEFAULT '',
    deleted_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (length(username::text) BETWEEN 2 AND 32 AND username::text ~ '^[a-z0-9._-]+$'),
    CHECK (email IS NULL OR (email::text = trim(email::text) AND email::text <> ''))
);

CREATE TABLE user_credentials (
    user_id             uuid PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    algorithm           text NOT NULL DEFAULT 'argon2id' CHECK (algorithm = 'argon2id'),
    password_hash       text,
    password_changed_at timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (password_hash IS NULL OR password_hash LIKE '$argon2id$%')
);

CREATE TABLE user_identities (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    provider            text NOT NULL,
    provider_subject    text NOT NULL,
    provider_email      citext,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (provider, provider_subject),
    UNIQUE (user_id, provider, provider_subject)
);

CREATE TABLE temporary_passwords (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    password_hash       text NOT NULL,
    expires_at          timestamptz NOT NULL,
    used_at             timestamptz,
    revoked_at          timestamptz,
    created_by          uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (user_id, id),
    CHECK (password_hash LIKE '$argon2id$%'),
    CHECK (expires_at > created_at)
);
-- At most one open (unused + unrevoked) temporary password per account.
CREATE UNIQUE INDEX temporary_passwords_one_open
    ON temporary_passwords (user_id)
    WHERE used_at IS NULL AND revoked_at IS NULL;

CREATE TABLE password_reset_tokens (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash          bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at          timestamptz NOT NULL,
    used_at             timestamptz,
    revoked_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (expires_at > created_at)
);

CREATE TABLE devices (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name                text NOT NULL,
    user_agent          text NOT NULL DEFAULT '',
    last_ip             inet,
    last_seen_at        timestamptz,
    revoked_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (user_id, id)
);

CREATE TABLE sessions (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    device_id           uuid NOT NULL,
    token_hash          bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    credential_kind     text NOT NULL CHECK (credential_kind IN ('password','temporary','pairing','recovery')),
    temporary_password_id uuid,
    CHECK ((credential_kind = 'temporary') = (temporary_password_id IS NOT NULL)),
    last_ip             inet,
    user_agent          text NOT NULL DEFAULT '',
    last_seen_at        timestamptz NOT NULL DEFAULT now(),
    expires_at          timestamptz NOT NULL,
    revoked_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (expires_at > created_at),
    FOREIGN KEY (user_id, device_id) REFERENCES devices(user_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (user_id, temporary_password_id) REFERENCES temporary_passwords(user_id, id) ON DELETE RESTRICT
);
CREATE INDEX sessions_user_active ON sessions (user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expiry ON sessions (expires_at);

CREATE TABLE device_pairings (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash          bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_by          uuid REFERENCES users(id) ON DELETE RESTRICT,
    expires_at          timestamptz NOT NULL,
    used_at             timestamptz,
    revoked_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (expires_at > created_at)
);

CREATE TABLE recovery_codes (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    code_hash           text NOT NULL CHECK (code_hash LIKE '$argon2id$%'),
    mask                text,
    used_at             timestamptz,
    revoked_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);
CREATE UNIQUE INDEX recovery_one_open
    ON recovery_codes (user_id)
    WHERE used_at IS NULL AND revoked_at IS NULL;

-- Shared login/recovery rate-limit windows. bucket_hash = SHA-256 of the
-- caller-composed key ("ip:…", "user:…", "recover:ip:…"); deliberately no FK
-- to users so unknown usernames and IPs rate-limit identically and the table
-- cannot be abused for account enumeration.
CREATE TABLE login_rate_limits (
    bucket_hash         bytea PRIMARY KEY CHECK (octet_length(bucket_hash) = 32),
    failures            integer NOT NULL DEFAULT 0 CHECK (failures >= 0),
    window_started_at   timestamptz NOT NULL,
    blocked_until       timestamptz,
    expires_at          timestamptz NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);
CREATE INDEX login_rate_limits_expiry ON login_rate_limits (expires_at);

-- Singleton coordination row for serialized auth invariants (last active
-- admin, first-admin creation). Writers take SELECT … FOR UPDATE on id=1
-- before checking, so competing mutations see each other's outcome.
CREATE TABLE auth_invariant_lock (
    id                  smallint PRIMARY KEY CHECK (id = 1),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);
INSERT INTO auth_invariant_lock (id) VALUES (1);

CREATE TABLE projects (
    id                  uuid PRIMARY KEY,
    slug                text NOT NULL UNIQUE,
    name                text NOT NULL,
    description         text NOT NULL DEFAULT '',
    notes               text NOT NULL DEFAULT '',
    universe_name       text NOT NULL DEFAULT '',
    settings            jsonb NOT NULL DEFAULT '{}',
    deleted_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (length(slug) > 0)
);

CREATE TABLE project_members (
    project_id          uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    role                text NOT NULL CHECK (role IN ('owner','collab','view')),
    removed_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, user_id)
);
-- Exactly one non-removed owner row per project; the application additionally
-- refuses to leave zero owners, and transfers demote+promote in one tx.
CREATE UNIQUE INDEX project_one_owner
    ON project_members (project_id)
    WHERE role = 'owner' AND removed_at IS NULL;
CREATE INDEX project_members_by_user
    ON project_members (user_id, project_id)
    WHERE removed_at IS NULL;

CREATE TABLE audit_logs (
    id                  uuid PRIMARY KEY,
    project_id          uuid REFERENCES projects(id) ON DELETE RESTRICT,
    sequence            bigint GENERATED BY DEFAULT AS IDENTITY UNIQUE NOT NULL,
    actor_user_id       uuid REFERENCES users(id) ON DELETE RESTRICT,
    actor_kind          text NOT NULL CHECK (actor_kind IN ('user','worker','system','import')),
    action              text NOT NULL,
    target_kind         text NOT NULL,
    target_id           text NOT NULL,
    request_id          text,
    details             jsonb NOT NULL DEFAULT '{}',
    occurred_at         timestamptz NOT NULL DEFAULT now(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version = 1),
    CHECK (updated_at = created_at)
);
CREATE INDEX audit_project_time ON audit_logs (project_id, occurred_at DESC, id);
CREATE INDEX audit_actor_time ON audit_logs (actor_user_id, occurred_at DESC);

CREATE TABLE blobs (
    id                  uuid PRIMARY KEY,
    project_id          uuid REFERENCES projects(id) ON DELETE RESTRICT,
    object_key          text NOT NULL UNIQUE,
    storage_driver      text NOT NULL CHECK (storage_driver IN ('local','s3')),
    sha256              text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes          bigint NOT NULL CHECK (size_bytes >= 0),
    content_type        text NOT NULL,
    visibility          text NOT NULL DEFAULT 'private' CHECK (visibility IN ('private','public')),
    state               text NOT NULL CHECK (state IN ('staged','ready','deleting')),
    created_by          uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (project_id, id)
);
CREATE INDEX blobs_gc ON blobs (state, created_at);
CREATE INDEX blobs_tenant_hash ON blobs (project_id, sha256);

-- +goose Down
-- Down is for throwaway databases only; the playbook forbids it on any
-- database with writers.
DROP TABLE IF EXISTS blobs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS auth_invariant_lock;
DROP TABLE IF EXISTS login_rate_limits;
DROP TABLE IF EXISTS recovery_codes;
DROP TABLE IF EXISTS device_pairings;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS temporary_passwords;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS user_credentials;
DROP TABLE IF EXISTS users;
