---
phase: 4
title: "Accounts, sessions, devices, members và audit"
status: completed (2026-10-29; PG_ACCOUNTS opt-in, not production-ready; live v3→v4 migration proven byte-equal + .v3.bak on copy of real web-accounts.json; full suite green on disposable PG)
priority: P1
effort: 40h
dependencies: [2, 3]
---

# Phase 04 — Miền nghiệp vụ đầu tiên: identity và quyền

## Context và scope bắt buộc

Foundation 01–03 không phải cắt dữ liệu domain trước accounts: **accounts là domain đầu tiên chuyển**. Sau gate 02/03, chưa production until 08/09. Theo [schema §2/4](./reports/schema-and-contracts.md), tạo đầy đủ user/auth/project membership/audit model, không chỉ thay `web-accounts.json` bằng một JSON column.

Facts cập nhật: global `admin|member`, Projects map `owner|collab|view` (`internal/webauth/store.go:39-41,63-119`); Argon2id/hash-slot cap (`internal/webauth/password.go:14-23,35-74`); create/update/delete/first-admin (`store.go:477-551,584-700`); sessions (`:738-880`); password change keep-current (`:897-931`); migration old books grants (`internal/webauth/members.go:97-195`); admin-only HTTP user management (`internal/entry/web/authorization.go:21-22`). Không dùng tên global collaborator hay handler `serveMembers` không tồn tại.

Bảo toàn user management: admin tạo member, sửa display name/global role/Projects, khóa/mở/xóa, cấp/reset mật khẩu tạm trả một lần, buộc đổi mật khẩu, liệt kê/revoke từng phiên/device/revoke-all, pairing, recovery/onboarding, audit filter/pagination. Bổ sung TTL/one-time temp và race-safe DB invariants theo yêu cầu; không làm OAuth hoặc email workflow mới.

## Architecture / inputs → transforms → outputs

Request JSON + authenticated admin/session → authorization giữ nguyên → validate/Argon2 ngoài transaction (slot bounded) → transaction recheck hash/version/status + user/membership/session mutation + audit event → response envelope/SSE invalidate. Login generic credential error cho user không có/disabled/wrong; unknown user vẫn dummy Argon2. Bound time/memory parameters khi parse hash trước verify để import độc hại không đốt RAM.

User ID mới UUID deterministic khi import vì format hiện tại chỉ username; username/email citext unique toàn bộ soft-deleted rows. Database `status=active|locked` map JSON `disabled`; giữ disabled metadata/onboardedAt/passwordChangedAt. `user_credentials` permanent hash; `temporary_passwords` hash/expiry/used/revoked, restricted session có link token temp. Temp consume+session create atomic. `devices` metadata và revoke, không khóa theo fingerprint. Session chỉ token hash, expiry/revoked/lastSeen/IP/UA; tokens plaintext không log.

Project rows được tạo ở phase này làm anchor cho memberships, blobs FK, exact-one-owner. Story content đến 05; không tạo empty story service giả để vượt gate. Project ownership transfer dưới row lock + deferred constraint; last-admin mutation dưới singleton lock. User khóa/xóa khi đang owner phải transfer hoặc delete project trong cùng transaction, có lỗi/action tiếng Việt rõ. Existing nhiều owner cần explicit import mapping, không tự demote.

## Files / owners

`R=/Users/dat/Projects/ainovel-cli`.

- Modify `R/internal/webauth/store.go:63-203,477-551,584-700,738-931`, `members.go:53-78,97-195,239-310`, `password.go:14-74`, `recovery.go:91-216`, `setup.go:51-81`; giữ security helpers, không viết algorithm thứ hai.
- Modify `R/internal/entry/web/auth.go:54-133,245-371,390-549`, `authorization.go:14-155`, `gateway.go:73-152,478-581`; `R/cmd/ainovel-cli/serve.go:130-149`; `R/README.md` account and migration notes.
- Create `R/internal/webauth/repository.go`, `file_repository.go`, `temporary_password.go`; `R/internal/persistence/postgres/accounts.go`, `memberships.go`, `audit.go`, `auth_limiter.go`; `R/internal/persistence/migrations/00002_accounts_projects_blobs.sql`; `R/internal/persistence/postgres/accounts_test.go`, `identity_invariants_test.go`; `R/internal/entry/web/accounts_backend_test.go`.
- Update tests `R/internal/webauth/{store,members,recovery,security_review}_test.go`; `R/internal/entry/web/{auth,members_ad,path_permissions,setup}_test.go`; `R/cmd/ainovel-cli/serve_members_test.go`. These exact test files were globbed/read; reused fixtures not duplicate convention.
- No Delete public APIs. Existing `webauth.NewStore` constructor remains file mode; account facade backed by selected repository passed to unchanged NewGateway signature, so constructor callers in source inventory §4 do not need gratuitous rewrite. New backend factory injected only at serve composition.

## Steps

1. Add fail-before parity/security tests: exact role names/response fields, admin-only create/update, restricted session route allowlist, current session retained on password change, onboarding, recovery code mask/rotation/one-time, disabled metadata, revoke streams, existing username 2 chars allowed. Test file and postgres via same factory.
2. Implement goose SQL for users/credentials/identities/temp/reset/devices/sessions/pairings/recovery/limiter/auth lock, projects/memberships/audit/blobs as schema report. Include all grants/triggers, not merely prose. Migration 00002 precedes all story FKs in 05. Empty DB supports setup HTTP only; application may be live for setup, but story readiness false until first admin/root; cannot bypass setup by arbitrary first account API.
3. Repository extraction preserves file version3 reading and one-time backups; for new temp/device metadata, increment file schema version explicitly and write upgrader preserving old bytes. File backend new requirements use existing cross-process flock; do not replace it with process-only mutex. PG facade does not read accounts file or keep stale permission snapshot.
4. Implement admin CreateUser/UpdateUser/Projects atomically with audit. Argon2 calculation outside tx, then compare expected credential/user version under lock; if changed return conflict/retry user action, no overwrite. Do not grant system admin through project role. Enforce last active admin and one active owner constraints at transaction commit, serialize competing mutations. CRUD no-op must not duplicate audit/version changes.
5. Implement temporary password state machine: admin create/reset must-change → pending temp expires24h → first login consumes under lock → restricted session allowed me/logout/password/sessions → ChangePassword checks current temp hash, writes permanent, clears must-change, keeps current session, revokes other sessions/pairings. Retain consumed temp hash only while needed for restricted session verification, no login reuse. If restricted session lost, admin issues new temp. New TTL/one-time behavior tested in both backends.
6. Implement tokens/session/device revoke/pair/recovery/reset transactions, expiry and audit. Preserve id.secret cookie shape externally; DB may store ID+secret SHA-256 instead of full cookie digest but one documented verifier, no plain secrets. Recovery codes currently Argon2id (`recovery.go:173-179`): **keep hash and mask**, not SHA-256 conversion without plaintext. Reset random tokens may use SHA-256 separately. Setup remains bounded in-memory one-replica code but `CreateFirstAdmin` DB lock wins multi-process races; no first-admin env password printed to logs.
7. Replace in-memory login limiter on postgres mode with atomic DB windows IP+username; keep existing file limiter interface. Trust client IP only from proxy normalization implemented 09; until then RemoteAddr safe. Update recovery/setup limiter same source identity and shared DB windows for hosted restarts; Argon2 slot count bounded to VPS RAM, request body size limited.
8. Wire authenticated blob permission authorizer from 03; only `ready` scoped project blob returned. Global metadata/system settings admin only; SSE/list/Jobs later receive current permission actor, no global cache field. Preserve stream auth rechecks `authorization.go:131-155`.
9. Add account import adapter schema mapping/version3 and legacy permission decoder used by full importer08. Do not run mutate-on-read migration against live source; decode from snapshot read-only. Add docs explaining logout once at cutover/temp TTL/owner repair.
10. **Owner-scoped membership API (đã chốt 2026-10-28, behavior change):** add `POST /api/projects/:p/members` (invite existing user), `PATCH/DELETE /api/projects/:p/members/:userID` (change collab/view role / remove), `POST /api/projects/:p/owner` (atomic transfer under row lock + deferred one-owner check). Guard = project owner or admin; owner must not touch global role/password/disabled, must not remove the last active owner, and `/api/users` stays admin-only for owners (assert 403). Fail-before tests: run the new cases against current code — owner PATCH `/api/projects/:p/members/:userID` → 404 route not found; collab invite → 403 — before adding handlers.
11. **Collab unkeep Cảnh báo (đã chốt 2026-10-28, behavior change):** `DELETE …/keep` becomes symmetric with `POST …/keep` for collab. Carve the keep route out of the DELETE→owner mapping in `routePermission` (`internal/entry/web/authorization.go:63-66`) so collab DELETE keep reaches the handler (`internal/entry/web/review_keep.go:78-101`); other DELETEs (story/chapter/comment) stay owner-only. Fail-before test: collab DELETE keep → 403 with current guard, then → 2xx after the carve-out.

## Fail-before matrix

| Layer | Cases | Assertions |
|---|---|---|
| Unit/contract | create/temp/reset/change, correct/incorrect current secret, username/email casing, disabled/reactivate/delete/onboard | Existing envelopes/errors; no plaintext hash exposure; duplicate identifiers blocked |
| Unit/contract | expiry/one-time temp, reset token, pairing, recovery mask and role eligibility | Second consume fails; ChangePassword can finish after temp expires but valid restricted session |
| DB integration | two first-admin requests, two demote/disable/delete-last-admin, two transfer-owner, remove-last-owner | One serial outcome; never 0 or 2 owner, never 0 admin |
| DB integration | member grant writes+audit failure rollback, CAS stale patch, concurrent password reset/login | No partial grants/audit; old credential/session rejected |
| HTTP integration | admin/member roles × project none/view/collab/owner, IDOR URL/body/query scope, must-change streams/blob/jobs; **owner-scoped members API** (invite/patch/remove/transfer, owner cannot hit `/api/users`, cannot demote last owner); **collab DELETE keep → 2xx**, collab DELETE story/comment still 403 | 401/403 existing contract, no cross-project rows/events/bytes; new matrix rows match schema report §9 quyết định 2026-10-28 |
| HTTP integration | revoke current/other/all devices, long stream revoke permission | Auth fails immediately on next request; stream canceled ≤existing 1s polling+network delay |
| E2E | admin create user→temp login→forced change→project edit as granted→revoke device | Full path with real DB, not mock web API |

## Verification commands (future implementation)

```sh
go test ./internal/webauth -run 'TestAccountContract|TestMustChangePassword|TestTemporaryPassword|TestRecovery' -count=1
go test ./internal/persistence/postgres -run 'TestAccounts|TestIdentityInvariants|TestConcurrentOwner|TestConcurrentLastAdmin|TestAuthLimiter' -count=1
go test ./internal/entry/web -run 'TestAD|TestPath|TestMustChangePasswordViaAPI|TestAccountsBackend|TestSetup' -count=1
go test ./cmd/ainovel-cli -run TestServe -count=1
```

Set isolated `TEST_DATABASE_URL` and `REQUIRE_POSTGRES_TEST=1` for DB gates. Tests new names explicitly created above; selection must execute assertions, not zero tests. No test/formatter run while writing this plan.

## Risks / rollback / done

- **Critical (L cao × I cao):** lose current account management or grant wrong role on import. Mitigation: v3/legacy fixtures, current handler tests first, no broad grant wildcard; explicit owner map.
- **High (L vừa × I cao):** last-admin/owner races, stale permissions. DB locks+constraints+CAS, session/stream reauth; file flock parity.
- **High (L vừa × I cao):** temp/recovery plaintext or reusability. Argon2/hash tokens, atomic consume/expiry, redacted audit/log; no public reset UI beyond existing flows.
- **High (L vừa × I cao):** Argon2 concurrent login RAM DoS. Preserve slot cap/parameter bounds and limiter before hashing; no DB tx during slow hash.

Rollback pre-production: file mode remains intact; remove staging DB, source unchanged. Once accounts written in PG, rollback app only to DB-compatible release; otherwise freeze and use08 reverse-export including new grants/credentials/temp/revocations before selecting file mode. Never revive old sessions. Done: same user lifecycle tests file+postgres, invariant races pass, no source account access PG path, role/permission/401/403 contract preserved, audit consistency proved. Phase05 begins only after this identity gate.
