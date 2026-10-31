-- Identity account/session primitives for the shared Postgres repository.
-- Every statement is parameterized; actor scoping happens in Go on top of
-- these primitives — never trust tenant data from process-global state.
-- Sessions/devices keep their inet and nullable-uuid parameters here with
-- explicit casts (sqlc types them as *netip.Addr / pgtype.UUID), so runtime
-- domain SQL never lives in the repository.

-- name: CountActiveUsers :one
SELECT count(*) FROM users WHERE deleted_at IS NULL;

-- name: FirstUserCreatedAt :one
SELECT min(created_at) FROM users WHERE deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL;

-- name: GetUserByUsernameAny :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserForUpdate :one
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ListActiveUsers :many
SELECT * FROM users WHERE deleted_at IS NULL ORDER BY username;

-- name: CountActiveAdmins :one
SELECT count(*) FROM users
WHERE global_role = 'admin' AND status = 'active' AND deleted_at IS NULL;

-- name: InsertUser :one
INSERT INTO users (id, username, display_name, global_role, status,
    must_change_password, onboarded_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, 'active', $5, $6, $7, $7)
RETURNING *;

-- name: UpdateUserAfterPatch :one
UPDATE users SET display_name = $2, global_role = $3, status = $4,
    must_change_password = $5, disabled_at = $6, disabled_by = $7,
    disabled_reason = $8, updated_at = $9, version = version + 1
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: ClearMustChangePassword :exec
-- Column-minimal variant for the least-privilege identity role: only the
-- columns granted by deploy/postgres-init/02-identity-role.sql.
UPDATE users SET must_change_password = false, updated_at = $2,
    version = version + 1
WHERE id = $1 AND deleted_at IS NULL;

-- name: MarkOnboarded :exec
UPDATE users SET onboarded_at = $2, updated_at = $2, version = version + 1
WHERE id = $1 AND onboarded_at IS NULL AND deleted_at IS NULL;

-- name: SoftDeleteUser :exec
UPDATE users SET deleted_at = $2, updated_at = $2, version = version + 1
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetCredential :one
SELECT * FROM user_credentials WHERE user_id = $1;

-- name: InsertCredential :exec
INSERT INTO user_credentials (user_id, password_hash, password_changed_at)
VALUES ($1, $2, $3);

-- name: SetCredentialHash :exec
UPDATE user_credentials SET password_hash = $2, password_changed_at = $3,
    updated_at = $3, version = version + 1
WHERE user_id = $1;

-- name: GetOpenTempPasswordForUpdate :one
SELECT * FROM temporary_passwords
WHERE user_id = $1 AND used_at IS NULL AND revoked_at IS NULL
ORDER BY created_at DESC LIMIT 1 FOR UPDATE;

-- name: GetLatestTempPassword :one
SELECT * FROM temporary_passwords
WHERE user_id = $1
ORDER BY created_at DESC LIMIT 1;

-- name: InsertTempPassword :exec
INSERT INTO temporary_passwords (id, user_id, password_hash, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5);

-- name: ConsumeTempPassword :exec
UPDATE temporary_passwords SET used_at = $2, updated_at = $2, version = version + 1
WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL;

-- name: RevokeAllTempPasswords :exec
UPDATE temporary_passwords SET revoked_at = $2, updated_at = $2, version = version + 1
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: InsertSession :exec
INSERT INTO sessions (id, user_id, device_id, token_hash, credential_kind,
    temporary_password_id, last_ip, user_agent, last_seen_at, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(device_id), sqlc.arg(token_hash),
    sqlc.arg(credential_kind), sqlc.narg(temporary_password_id),
    sqlc.narg(last_ip)::inet, sqlc.arg(user_agent), sqlc.arg(last_seen_at),
    sqlc.arg(expires_at));

-- name: UpsertDevice :one
INSERT INTO devices (id, user_id, name, last_ip, last_seen_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(name), sqlc.narg(last_ip)::inet,
    sqlc.arg(last_seen_at))
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name,
    last_ip = EXCLUDED.last_ip, last_seen_at = EXCLUDED.last_seen_at,
    updated_at = clock_timestamp(), version = devices.version + 1
RETURNING *;

-- name: ListSessions :many
SELECT s.id, s.user_id, s.device_id, s.token_hash, s.credential_kind,
    s.temporary_password_id, s.last_ip, s.user_agent, s.last_seen_at,
    s.expires_at, s.revoked_at, s.created_at, s.updated_at, s.version,
    u.username::text AS username, d.name AS device_name
FROM sessions s
JOIN users u ON u.id = s.user_id AND u.deleted_at IS NULL
JOIN devices d ON d.id = s.device_id AND d.user_id = s.user_id
WHERE s.revoked_at IS NULL AND s.expires_at > now()
  AND (sqlc.narg(user_id)::uuid IS NULL OR s.user_id = sqlc.narg(user_id)::uuid)
ORDER BY s.last_seen_at DESC;

-- name: RevokeAllUserSessions :many
-- Revokes every live session of one account, current one included. There is
-- deliberately no keep/except parameter: credential, privilege and status
-- changes must force a fresh login on both apps.
UPDATE sessions SET revoked_at = $2, updated_at = clock_timestamp(),
    version = version + 1
WHERE user_id = $1 AND revoked_at IS NULL
RETURNING id;

-- name: SessionStatsByUser :many
SELECT user_id, count(*)::int AS session_count, max(last_seen_at) AS last_seen_at
FROM sessions
WHERE revoked_at IS NULL AND expires_at > now()
GROUP BY user_id;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = $2, updated_at = $2, version = version + 1
WHERE id = $1 AND revoked_at IS NULL;

-- name: TouchSessionLastSeen :exec
UPDATE sessions SET last_seen_at = $2, updated_at = clock_timestamp(),
    version = version + 1
WHERE id = $1 AND revoked_at IS NULL;

-- name: InsertPairing :exec
INSERT INTO device_pairings (id, user_id, token_hash, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetPairingByTokenHash :one
SELECT * FROM device_pairings WHERE token_hash = $1;

-- name: ConsumePairing :exec
UPDATE device_pairings SET used_at = $2, updated_at = $2, version = version + 1
WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at > $2;

-- name: RevokeUserPairings :exec
UPDATE device_pairings SET revoked_at = $2, updated_at = $2, version = version + 1
WHERE user_id = $1 AND used_at IS NULL AND revoked_at IS NULL;

-- name: GetLatestRecoveryCode :one
SELECT * FROM recovery_codes
WHERE user_id = $1
ORDER BY created_at DESC LIMIT 1;

-- name: GetLatestRecoveryCodeForUpdate :one
SELECT * FROM recovery_codes
WHERE user_id = $1
ORDER BY created_at DESC LIMIT 1 FOR UPDATE;

-- name: InsertRecoveryCode :exec
INSERT INTO recovery_codes (id, user_id, code_hash, mask)
VALUES ($1, $2, $3, $4);

-- name: ConsumeRecoveryCode :exec
UPDATE recovery_codes SET used_at = $2, updated_at = $2, version = version + 1
WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL;

-- name: RevokeRecoveryCode :exec
UPDATE recovery_codes SET revoked_at = $2, updated_at = $2, version = version + 1
WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL;

-- name: LockAuthInvariant :one
SELECT id FROM auth_invariant_lock WHERE id = 1 FOR UPDATE;

-- name: InsertAccountAudit :exec
-- Account/grant audit has no project scope and returns nothing: the
-- least-privilege identity role holds INSERT on this column list only, and
-- RETURNING would require SELECT on the identity sequence column.
INSERT INTO audit_logs (id, actor_user_id, actor_kind, action, target_kind,
    target_id, request_id, details, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetLimitBucket :one
SELECT * FROM login_rate_limits WHERE bucket_hash = $1;

-- name: GetLimitBucketForUpdate :one
SELECT * FROM login_rate_limits WHERE bucket_hash = $1 FOR UPDATE;

-- name: InsertLimitBucket :exec
INSERT INTO login_rate_limits (bucket_hash, failures, window_started_at, blocked_until, expires_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (bucket_hash) DO UPDATE SET failures = EXCLUDED.failures,
    window_started_at = EXCLUDED.window_started_at,
    blocked_until = EXCLUDED.blocked_until,
    expires_at = EXCLUDED.expires_at,
    updated_at = clock_timestamp(), version = login_rate_limits.version + 1;

-- name: DeleteLimitBucket :exec
DELETE FROM login_rate_limits WHERE bucket_hash = $1;
