# Báo cáo Phase 01 — Repository Postgres dùng chung và quyền từng app

Trạng thái: hoàn tất phần code + test. Không commit/push.

## Phạm vi đã giao

### Chuyển repository sang `platform/identity/postgres`

Tệp mới (package `postgres`, caller alias `identitypg`):

- `platform/identity/postgres/accounts.go` — account lifecycle, projection, first-admin, last-admin guard; constructor `NewAccounts(*pgxpool.Pool, Options)`.
- `platform/identity/postgres/sessions.go` — login/pair/authenticate/session management, token hash SHA-256, timing-safe compare, `RevokeAll`.
- `platform/identity/postgres/passwords.go` — change password, recovery, temp-password TTL; không còn `keepSession`.
- `platform/identity/postgres/memberships.go` — project/member APIs, owner invariants, transfer.
- `platform/identity/postgres/auth_limiter.go` — shared `AuthLimiter` + `WindowLimiter` trên `login_rate_limits`.
- `platform/identity/postgres/audit.go` — audit writer tách account/project, reader chỉ Noverse.
- `platform/identity/postgres/grants.go` — `SetAppGrant`/`RevokeAppGrant` + operator entry.
- `platform/identity/postgres/options.go` — `Options{ProjectAccess, ValidProjectID, Now}` bất biến.
- `platform/identity/postgres/pool.go` — `Pool`, `Open`, `RunInTx`, `CheckContract`, redacted error.
- `platform/identity/postgres/sqlc.yaml`, `queries/{accounts,grants,projects}.sql`, `gen/*` (sinh lại).

Đã xoá khỏi Noverse (không alias/re-export): `accounts.go`, `sessions.go`, `passwords.go`, `memberships.go`, `auth_limiter.go`, `audit.go`.

Helper còn dùng bởi domain Noverse được giữ tại `apps/noverse/internal/persistence/postgres/pgtypes.go` (`randomUUID`, `ts`, `tsPtr`, `noRows`, `constraintErr`). `queries/accounts.sql` của Noverse chỉ còn `GetUserByUsername`/`GetUserByID`; `queries/projects.sql` giữ anchor/membership mà repo nghiệp vụ còn dùng (`GetProjectBySlug`, `EnsureProject`, `ListProjects`, `InsertProject`, `UpdateProjectFile`, `ListMembershipsByUser`, `UpsertMember`). Identity/limiter/audit SQL đã chuyển khỏi Noverse.

### Facade / view / grant capability

- `platform/identity/app-grants.go`: `App` (`noverse`,`viderse`), `AppGranter`, `ErrUnknownApp`, `EffectiveGrants`, `fileGrants`, `cloneGrants`; facade `AppGrants`, `SetAppGrant`, `RevokeAppGrant`, `OperatorSetAppGrant`, `OperatorRevokeAppGrant`. File backend trả `ErrUnsupported` cho mutation grant (không giả thành công), vẫn có quyền Noverse hiệu lực.
- `platform/identity/store.go`: `User.Grants`, `UserView.Grants`; `View`/`UserFromView` copy map phòng thủ; `Authenticate` file copy grants/projects. Bỏ `keepSession` khỏi `ChangePassword`; `CreateUser` file từ chối grant Viderse.
- `platform/identity/members.go`: `RevokeAll(username, context)` — bỏ `keep`.
- `platform/identity/repository.go`: interface cập nhật `RevokeAll`/`ChangePassword`; bổ sung `NewUser.Apps` (chỉ server set, `json:"-"`).
- Sentinel mới `identity.ErrUnsupported`, `identity.ErrConflict`; `httperr.go` map `ErrConflict→409`, `ErrUnsupported→501`.

### Schema / role / migration

- `apps/noverse/internal/persistence/migrations/00012_app_grants.sql`: bảng `app_grants` + index `(app)`, backfill mọi account `deleted_at IS NULL` (gồm locked) sang `noverse:member`; không backfill Viderse; tombstone không grant. Down chỉ drop bảng (additive, dùng cho DB throwaway).
- `apps/noverse/deploy/postgres-init/02-identity-role.sql`: role `viderse_identity LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT`; nhận `identity_password`/`identity_schema` qua psql variable; tự bỏ qua khi thiếu password (an toàn cho `docker-entrypoint-initdb.d`); revoke toàn bộ quyền cũ trước khi grant để hội tụ; không `GRANT ALL`/`ALL TABLES`/`ALL SEQUENCES`/default privileges; grant đúng danh sách phase; `USAGE` sequence audit lấy qua `pg_get_serial_sequence`; comment từng GRANT.

### Callers / CI / test cũ

- `cmd/ainovel-cli/serve.go`, `storage.go` dùng `identitypg.NewAccounts(pool.Pool(), Options{ProjectAccess:true, ValidProjectID: library.ValidProjectID})`.
- `entry/web/auth.go`: bỏ tham số keep ở `ChangePassword`/`RevokeAll` (self và admin).
- 19 callsite test cũ đổi sang `identityRepo`/`identitypg`; `guard_test` dùng SQL trực tiếp cho `last_seen`; assertion giữ phiên cũ đổi thành “revoked + login lại” (`contract_test.go`, `store_test.go`, `identitytest/suite.go`, `auth_test.go`, `members_ad_test.go`).
- `.github/workflows/ci.yml`: thêm `go tool sqlc diff` cho `platform/identity/postgres`.

## Quyết định thiết kế

1. **ProjectAccess=false** từ chối *trước SQL*: project/member APIs, `UpdateUser` có `Role`/`Projects`/`Disabled`, `DeleteUser`, `Audit`, `PurgeAuditBefore` trả `identity.ErrUnsupported`. Projection trả `projects:{}` và không query `projects`/`project_members` (chứng minh bằng test đổi tên hai bảng rồi login/authenticate/list).
2. **Grant hiệu lực**: admin toàn cục ⇒ `admin` cả hai app, không cần row; member cần row rõ ràng. Row chỉ lưu `role='member'`; yêu cầu cấp `admin` trả `ErrInvalidInput` (admin chỉ đến từ global role). Grant không đọc/ghi vào session; `Authenticate` đọc DB mỗi request.
3. **Idempotency của grant** quyết định theo row: row không đổi ⇒ no-op, không audit, không revoke. Row đổi ⇒ audit + revoke *mọi* session target trong cùng tx. Không dựa vào quyền hiệu lực (admin có row Víverse vẫn bị revoke khi row đổi — an toàn hơn).
4. **Lock order**: `auth_invariant_lock` → target user `FOR UPDATE`. Actor (HTTP path) được đọc và kiểm `global_role='admin'`, `status='active'` *sau* khi lấy invariant lock, trong cùng tx. Actor bị demote trong lúc chờ lock sẽ bị từ chối (test `TestAppGrantActorCheckedAfterLockWait`).
5. **Audit**: `InsertAccountAudit :exec` không `RETURNING`, không `project_id`; `InsertProjectAudit :exec` giữ `project_id`; reader `ListAudit` không đổi, chỉ chạy khi ProjectAccess=true.
6. **Password change**: dùng `ClearMustChangePassword` chỉ ghi `must_change_password/updated_at/version`; revoke mọi session qua `RevokeAllUserSessions` (không còn tham số keep/except).
7. **Pool identity**: bounded (`MaxConns` mặc định 8), timeout, UTC decode bằng đăng ký type theo connection (không sửa map toàn cục), ping + contract check qua `information_schema.columns` cho đúng bảng account/grant; không gọi `CheckSchema/CheckMigrations` của Noverse; lỗi được sanitize (URI userinfo, `password=`).
8. **New user Noverse**: mặc định chèn row `noverse:member` trong cùng tx khi `ProjectAccess=true`; `NewUser.Apps` (server-only) cho phép caller chọn app khác; không bao giờ auto cấp Viderse. `CreateFirstAdmin` cũng chèn row Noverse.
9. **Query chết đã bỏ** khi tách: `GetOpenTempPassword`, `RevokeOpenTempPasswords`, `RevokePairingByTokenHash`, `ActiveSessionIDsForUser`, `OwnerCountIncludingRemoved`, `PurgeExpiredLimitBuckets`, `GetReadyBlobProject`. Giữ `EnsureProject` (anchor dùng bởi test/domain).

## Test bảo mật bắt buộc — kết quả thực tế

Chạy với Postgres thật (`TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test REQUIRE_POSTGRES_TEST=1`):

| Test | Kết quả |
|---|---|
| `TestLoginRotatesSessionAndHashesToken` | PASS — cookie mới mỗi login, DB chỉ có SHA-256(secret), cookie ghép id+secret khác bị 401 |
| `TestUnknownUserRunsPasswordVerifier` | PASS — cùng lỗi generic, dummy hash argon2id, cả hai ≥15ms (floor, không benchmark so sánh) |
| `TestPasswordChangeRevokesEverySession` | PASS — self/temporary/admin-reset/disable/revoke-all/recovery đều revoke mọi session; login mới đúng flag |
| `TestPrivilegeChangeRevokesAllSessions` | PASS — promote/demote revoke + audit cùng tx; no-op không audit/revoke; last-admin fail rollback |
| `TestAppGrantRequiresGlobalAdminAndAudits` | PASS — non-admin bị 403 không row/audit; grant/revoke audit + revoke session; no-op; operator `granted_by NULL` + actor system; target deleted bị 404 |
| `TestViderseIdentityRoleCannotReadNoverseOrPromoteUser` | PASS — role thật từ script: account/login/password/grant chạy; SELECT projects/stories/project_members/audit_logs/noverse_meta/goose_db_version, UPDATE global_role/status/deleted_at/display_name, bulk assignment, INSERT temporary_passwords, DDL, SET ROLE đều bị Postgres từ chối; PUBLIC không cấp quyền; password clear flag hoạt động |
| `TestSharedLimiterAcrossInstances` | PASS — hai instance chung budget user/IP; casing sau khi fold chung một bucket |
| `TestConcurrentFirstAdminAcrossRepositories` | PASS — 4 instance, đúng 1 winner + 1 session + 1 grant |
| `TestIdentityErrorsAndLogsRedactSecrets` | PASS — parse/connect/query/closed-pool errors không lộ password/hash/token; lỗi DB không thành `ErrUnauthenticated`; log rỗng |

Test bổ sung:

- `TestAppGrantRollsBackWhenAuditFails` — PASS: audit INSERT fail ⇒ grant row và revoke rollback.
- `TestAppGrantActorCheckedAfterLockWait` — PASS: actor bị demote trong lúc chờ invariant lock ⇒ `ErrForbidden`, không row/audit.
- `TestAppGrantSeenBySecondInstance` — PASS: grant/revoke thấy ngay từ instance thứ hai; session cũ bị revoke.
- `TestProjectAccessFalseSkipsProjectQueries` — PASS: đổi tên `projects`/`project_members` vẫn login/authenticate/users/password được; mọi API bị cấm trả `ErrUnsupported`.
- `TestAppGrantBackfillKeepsExistingAccounts` — PASS: migrate tới 11, seed active/locked/deleted, migrate 12; active+locked có `noverse:member`, tombstone không, không có Viderse; FK `ON DELETE RESTRICT`.
- `TestEffectiveGrants`, `TestViewGrantsAreDefensiveCopies`, `TestFileStoreGrantCapability` — PASS.
- Owner/last-admin/transfer/contract suite cũ vẫn PASS sau khi đổi constructor và semantics session.

## Lệnh đã chạy

Từ gốc repo, trừ khi ghi khác:

- `go build ./...` — PASS.
- `go vet ./...` — PASS.
- `go test -p 2 ./archtest/...` — PASS.
- `go test ./platform/identity ./platform/identity/identitytest ./platform/identity/postgres` (Postgres env) — PASS.
- `go test ./apps/noverse/internal/persistence/postgres` (Postgres env) — PASS.
- `go test -p 2 ./...` (Postgres env, `-count=1`, lọc dòng FAIL) — PASS toàn bộ, **chỉ còn** đúng lỗi có sẵn `apps/noverse/internal/persistence/migrate TestMigrateApplicationRoleCannotDDL` (`app DSN substitution failed; check TEST_DATABASE_URL userinfo`, không sửa theo yêu cầu). Lần chạy đầu còn một assertion cũ trong `TestADMembersContract` (“existing session must see current permissions”) do đổi contract password-change; đã sửa thành “phiên cũ 401 + đăng nhập lại”, giữ nguyên coverage và package `entry/web` PASS.
- `cd apps/noverse && go tool sqlc diff` — exit 0.
- `cd platform/identity/postgres && go tool sqlc diff` — exit 0.
- `gofmt -l` trên toàn bộ file Go thay đổi, với cả `go1.26.0` và `go1.27.0` — sạch.

## Khác biệt so với plan

- Test DB của repository dùng chung nằm trong `apps/noverse/internal/persistence/postgres` (package test) thay vì `platform/identity/postgres`, vì archtest cấm `platform/**` import `apps/**` (kể cả `internal/persistence/migrations`). `platform/identity/postgres` vẫn có test unit + redaction.
- Thêm `identity.ErrUnsupported`/`ErrConflict` và map HTTP tương ứng (409/501); plan chỉ yêu cầu “map storage conflict sang sentinel chung”.
- `TestUnknownUserRunsPasswordVerifier` dùng floor thời gian ≥15ms + kiểm shape dummy hash (không có seam để đếm invocation verifier); không dùng benchmark so sánh.
- Thêm hai test ngoài danh sách tên: `TestAppGrantRollsBackWhenAuditFails`, `TestAppGrantActorCheckedAfterLockWait`.
- CI thêm bước `sqlc diff` cho config platform (giữ invariant sinh mã).
