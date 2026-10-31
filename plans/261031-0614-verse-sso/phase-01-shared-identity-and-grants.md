# Phase 01 · Repository Postgres dùng chung và quyền từng app

- **Trạng thái:** pending. **Ước lượng:** 32h.
- **Phụ thuộc:** không. **Chặn:** 02–05.
- **Owner:** kỹ sư identity/database; sở hữu mọi tệp dưới đây đến khi bàn giao cho 02. Không sửa song song với phase khác.

## Bối cảnh đã xác minh

- Hợp đồng `identity.Repository` hiện chứa cả tài khoản lẫn project (`platform/identity/repository.go:24-54`); facade tạo tại `:73-75`. Account repository là **process-scoped**, actor đi qua tham số (`apps/noverse/internal/persistence/postgres/accounts.go:28-37`); limiter cũng giữ pool/clock (`auth_limiter.go:17-24`). Không thêm current app/user/grant mutable vào các instance này.
- Không thể chỉ chuyển tên package: constructor nhận `*postgres.Pool`, import `errs`, `library` và sqlc của Noverse (`accounts.go:17-20,28-37`); projection user luôn query membership (`:163-205`); update/delete giữ invariant owner (`:529-555,670-684`). `library.ValidProjectID` được gọi tại `:281,452`.
- SQL tài khoản ở `queries/accounts.sql:9-199`; query limiter và audit trộn trong `queries/projects.sql:85-151`. `InsertAudit` hiện `RETURNING sequence` (`:85-89`); `Audit` đọc join project (`audit.go:81-99`, `queries/projects.sql:91-103`). Cấp SELECT audit_logs cho Viderse sẽ làm lộ lịch sử Noverse.
- First-admin có lock DB thực sự: `CreateFirstAdmin` lấy singleton trước đếm user (`accounts.go:380-404`); schema lock tại `migrations/00002_accounts_projects_blobs.sql:169-178`. Giữ nguyên cơ chế này, không thay bằng mutex process.
- Schema users dùng UUID, `citext UNIQUE`, giữ cả tombstone (`00002_accounts_projects_blobs.sql:10-27`); session FK/temporary credential tại `:101-119`; audit có identity sequence tại `:214-229`.

Các đường dẫn rút gọn `accounts.go`, `sessions.go`, `auth_limiter.go`, `audit.go`, `passwords.go`, `memberships.go`, `queries/*` trong phần bối cảnh này đều thuộc `apps/noverse/internal/persistence/postgres/`.

## Kiến trúc đã chọn

**Chuyển implementation chung**, không dựng auth service mới, không nhân bản account schema và không giữ alias/re-export `postgres.NewAccounts` cũ. Đích mới: `platform/identity/postgres` (package `postgres`, caller alias `identitypg`). Constructor mới nhận `*pgxpool.Pool` và options bất biến, không nhận kiểu Noverse.

Chọn seam nhỏ đã duyệt: giữ `identity.Repository` hiện tại; thêm `ProjectAccess` bất biến vào options repository. Noverse truyền true và validator từ `identity.Options`; Viderse false. Không tách hàng loạt interface/caller trong lần này.

- `ProjectAccess=true`: giữ đầy đủ projection membership, last-owner checks, project APIs và audit reader; thay import validator bằng option. Không đổi UUID deterministic salt `ainovel|v1|…` (`accounts.go:69-78`).
- `ProjectAccess=false`: user projection trả `projects:{}` theo đúng mô hình Viderse, **không query** projects/project_members. Các project API, project-bearing CreateUser/UpdateUser, thay global role, khoá/mở khoá/xoá user, đọc/purge audit phải bị từ chối **trước SQL**, không trả thành công giả và không bỏ invariant. Login, Authenticate, setup, password, session/recovery, đọc/sửa grant vẫn có implementation thật. Không có route Viderse nào được nối vào thao tác bị cấm.
- Bằng chứng reachability: HTTP dùng chung chỉ gọi HasUsers/LoginLimiter khi dựng (`platform/webapp/server.go:71-80`), Authenticate (`:152`), Login (`:256`), RevokeSession (`:270`), FirstUserCreatedAt (`:316,343`), CreateFirstAdmin (`:419`), IssueRecovery (`:435`). Login/setup/me/logout là route nền (`:139-176,289-311`). Phase 03 thêm ChangePassword và quản lý phiên của chính mình, **không thêm** `/api/users`, project mutation, global user mutation hay audit reader của Viderse. Grant CLI không gọi các API đó.
- Audit tài khoản/grant vẫn ghi cùng transaction vào audit_logs, nhưng dùng query mới `:exec` **không RETURNING**, bỏ cột project_id khỏi account INSERT. Project audit reader vẫn chỉ Noverse bật; không tạo bảng audit khác hoặc cấp SELECT history cho Viderse.
- Helper dùng chung với repository nghiệp vụ Noverse không được mất khi dời file. Giữ helper còn dùng ở tệp nội bộ Noverse riêng (đề xuất `pgtypes.go`), hoặc chuyển thuật toán định danh thực sự dùng chung sang API chung rồi migrate caller. Không để wrapper constructor cũ. Timestamp wrappers ngắn không phải lý do kéo toàn bộ pool/facts/migrations lên platform.

## Tệp chính xác và ownership

**Chuyển/tách:**
- `apps/noverse/internal/persistence/postgres/{accounts.go,sessions.go,passwords.go,memberships.go,auth_limiter.go,audit.go}` → các tệp tương ứng mới ở `platform/identity/postgres/`.
- `apps/noverse/internal/persistence/postgres/queries/accounts.sql`, phần identity/limiter/audit trong `queries/projects.sql`; `apps/noverse/sqlc.yaml`; các output sqlc `gen/accounts.sql.go`, `gen/projects.sql.go`, `gen/models.go`, `gen/db.go` khi có thay đổi sinh ra.
- Mới `platform/identity/postgres/{sqlc.yaml,queries/accounts.sql,queries/grants.sql,queries/projects.sql,gen/*,grants.go,options.go,pool.go}`. Schema source của sqlc trỏ tới migration Noverse đã có, chỉ lúc sinh mã; Go platform **không embed/import** migrations/app. Không chép DDL gốc sang schema thứ hai.
- `platform/identity/{repository.go,store.go,options.go,members.go}`; mới `platform/identity/app-grants.go`, test tương ứng. Thêm backend grant capability riêng, không ép file store v4 đổi version chỉ vì SSO. File Noverse độc lập có quyền Noverse hiệu lực; thao tác cấp grant SSO yêu cầu backend Postgres.
- Mới migration **`apps/noverse/internal/persistence/migrations/00012_app_grants.sql`** (số tiếp theo đang trống; kiểm lại trước triển khai nếu nhánh khác thêm migration).
- Mới `apps/noverse/deploy/postgres-init/02-identity-role.sql`; vai trò cấp bởi owner/admin PostgreSQL, không để app runtime tự tạo role. Script dùng schema/database input được quote an toàn; không chứa password thật.

**19 callsite constructor cũ phải đổi**, toàn bộ đã grep:
1. `apps/noverse/cmd/ainovel-cli/serve.go:270`.
2. `apps/noverse/cmd/ainovel-cli/storage.go:250` (purge audit bằng maintenance role).
3. `apps/noverse/internal/entry/web/accounts_backend_test.go:51`.
4. `apps/noverse/internal/entry/web/library_backend_test.go:66`.
5–11. `apps/noverse/internal/persistence/postgres/accounts_test.go:25,101,136,137,149,150,166`.
12. `apps/noverse/internal/persistence/postgres/guard_test.go:116`.
13–17. `apps/noverse/internal/persistence/postgres/identity_invariants_test.go:25,60,61,108,109`.
18. `apps/noverse/internal/persistence/postgres/library_test.go:40`.
19. `apps/noverse/internal/persistence/postgres/narrative_security_test.go:35`.

Hai constructor limiter nội bộ cần chuyển cùng repository: `accounts.go:48,53`. `platform/identity.NewWindowLimiter` là limiter file khác, không thay nhầm (`platform/identity/limiter.go:128`). Giữ contract suite `platform/identity/identitytest/suite.go`; test đầy đủ Noverse chạy với ProjectAccess=true; test role Viderse chạy account/grant contract không cấp quyền project để “cho qua”.

## Schema và hợp đồng grant

Migration thêm:

```sql
CREATE TABLE app_grants (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    app text NOT NULL CHECK (app IN ('noverse','viderse')),
    role text NOT NULL CHECK (role = 'member'),
    granted_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, app)
);
INSERT INTO app_grants (user_id, app, role)
SELECT id, 'noverse', 'member' FROM users WHERE deleted_at IS NULL;
```

- Bao gồm tài khoản locked trong backfill để mở khoá không làm mất quyền Noverse; không backfill Viderse hoặc cấp quyền project. Tombstone tiếp tục chiếm username nhưng không được grant mới. Admin ngầm có cả hai quyền mà không cần row; UI phân biệt quyền kế thừa và row rõ ràng.
- API view mới `user.grants: {noverse?: 'member'|'admin', viderse?: 'member'|'admin'}`. Thêm copy ở `User.View` (`platform/identity/store.go:120-125`) và `UserFromView` (`:134-150`), cả NewSession/Users/Authenticate; không lộ hash. Map phải là bản sao, không cho request sửa map cache của Store.
- Grant mutation mới là SetAppGrant/RevokeAppGrant (tên **đề xuất**), validate app/role, user chưa xoá, lấy invariant lock rồi user lock theo thứ tự thống nhất, idempotent nếu không đổi, audit và revoke mọi session target trong cùng tx khi quyền thực sự đổi. HTTP kiểm actor global admin trong tx; CLI dùng trusted-operator entry point riêng, granted_by NULL, không giả actor user.
- Không lưu grant vào session. Authenticate đọc user/grants từ DB mỗi request; không process/session claim cache. Đổi global role hoặc grant bắt buộc login lại để có session ID mới; sau demote chỉ explicit grant còn hiệu lực. Chỉ đọc thấy thiếu grant không revoke cookie: trả 403 như contract.

## Role Viderse: quyền tối thiểu phải giao

Script tạo role `viderse_identity LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT`, không membership `noverse_app`/`noverse_migrate`/`viderse_migrate`; `CONNECT` chỉ DB identity và `USAGE` schema identity. Không `GRANT ALL`, `ALL TABLES`, `ALL SEQUENCES` hay default privileges sang role này.

SQL mẫu theo schema public (script thật nhận schema kiểm tra/quote, test dùng schema riêng):

```sql
GRANT USAGE ON SCHEMA public TO viderse_identity;
GRANT SELECT, INSERT ON public.users TO viderse_identity;
GRANT UPDATE (must_change_password, updated_at, version) ON public.users
    TO viderse_identity;
GRANT SELECT, INSERT, UPDATE ON public.user_credentials, public.devices,
    public.sessions, public.recovery_codes TO viderse_identity;
GRANT SELECT, UPDATE ON public.temporary_passwords, public.device_pairings
    TO viderse_identity;
GRANT SELECT, UPDATE ON public.auth_invariant_lock TO viderse_identity;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.login_rate_limits,
    public.app_grants TO viderse_identity;
GRANT INSERT (id, actor_user_id, actor_kind, action, target_kind, target_id,
    request_id, details, occurred_at) ON public.audit_logs TO viderse_identity;
```

Cấp `USAGE` **chỉ sequence của `audit_logs.sequence`**, lấy tên thật qua `pg_get_serial_sequence('public.audit_logs','sequence')` và quote identifier; không cần SELECT sequence hay SELECT audit. UUID các bảng account tự sinh trong Go (`accounts.go:83-91`), app_grants không có sequence. Không grant user_identities/password_reset_tokens vì route Viderse hiện không dùng; giữ nguyên bảng và dữ liệu, không xoá chúng. Không grant INSERT temporary_passwords/device_pairings vì Viderse không cấp mật khẩu tạm/QR trong scope này; password change/revoke cần SELECT/UPDATE.

Không có UPDATE toàn bảng users hoặc UPDATE global_role/status/deleted_at/display_name cho role Viderse. Query password hiện `UpdateUserAfterPatch` ghi cả global_role/status dù không đổi (`queries/accounts.sql:40-45`, `passwords.go:73-77`) phải thay bằng query mới chỉ clear must_change_password + timestamps/version. Không nới quyền cột để giữ query cũ. Test cả `UPDATE users SET global_role='admin'` và query bulk/current-column assignment: PostgreSQL phải deny; kiểm inherited/PUBLIC ACL không cấp lại quyền.

Không cấp SELECT/ghi `projects`, `project_members`, mọi bảng story/blob/import, audit history, `noverse_meta`, `goose_db_version`, CREATE schema hay role. Pool identity phải ping và kiểm contract bằng query các cột account/grant cần dùng, **không gọi Noverse Pool.CheckSchema/CheckMigrations** vốn đọc metadata (`apps/noverse/internal/persistence/postgres/pool.go:147-150,168-171,208-214`). Pool bounded, UTC decode, timeout và redacted errors tương đương `pool.go:71-115,118-129`; close do app caller sở hữu.

## Trình tự thực hiện

1. Viết test contract grant/projection và test DB role với least-privilege grants trước. Ghi rõ kết quả mong đợi, không dựng user fake ở đường HTTP thay cho login.
2. Thêm migration backfill; giữ expand-only, không đổi migration đã chạy. Sinh sqlc platform từ migration source hiện có; tách query account khỏi query domain Noverse, tái sử dụng query project mà backend nghiệp vụ vẫn cần thay vì xoá nhầm.
3. Chuyển repository và limiter, thay phụ thuộc Pool/errs/library/gen. Map storage conflict sang sentinel chung trong identity và map HTTP Noverse về 409 tương đương; không để import `internal/errs` từ platform hoặc trả SQLSTATE ra client.
4. Gắn ProjectAccess bất biến, guard trước SQL; giữ owner transaction/locking cho Noverse. Chuyển 19 constructor, giữ helper domain còn sống đúng owner. Giữ thuật toán hash/UUID/TTL; thay policy session revocation theo yêu cầu bảo mật bên dưới, không giữ keepSession.
5. Thêm grants facade, view và audit writer :exec; nối tất cả read/mutation/first-admin vào contract. Noverse tạo member mới phải có explicit grant Noverse trong cùng tx khi caller chọn app; không auto cấp Viderse.
6. Tạo role script, tests negative privileges; document cạnh script mục đích từng GRANT. Không chạy script trên DB của chủ máy.

## Validation giao cho người triển khai/main agent

- Unit: hiệu lực admin/member/locked/deleted; app lạ/role lạ; idempotency; copy map không alias; deny branch không query project; no-grant không đồng nghĩa invalid session.
- Postgres: backfill giữ user cũ, singleton concurrent setup, last-admin/last-owner/transfer không đổi, rollback mutation cũng rollback audit/grant; Authenticate từ hai instance thấy grant mới; ProjectAccess=false thực sự login bằng role Viderse.
- Negative SQL: role Viderse đọc/ghi account theo bảng trên được; SELECT audit/projects/stories/membership, INSERT temporary password, DDL, đổi role đều bị từ chối. Thử cả query trực tiếp và method có ProjectAccess=false; không dùng migration role để che thiếu GRANT.
- Limiter: hai instance chia sẻ `login_rate_limits`, cùng prefix `auth` và `ip:`/`user:` (`auth_limiter.go:33-45`, HTTP Noverse `auth.go:101-105`, webapp `server.go:247-250`); không thêm prefix app. Giữ semantics lỗi DB hiện có, không mở scope sửa thuật toán backoff.
- Lệnh dự kiến từ gốc repo: `go test ./platform/identity/... ./archtest`; `REQUIRE_POSTGRES_TEST=1 go test ./apps/noverse/internal/persistence/postgres/... ./apps/noverse/internal/entry/web/...`. TEST_DATABASE_URL phải trỏ DB thử; không skip integration trong gate. Chạy sqlc diff theo config Noverse và config platform mới.

## Kiểm bảo mật bắt buộc và test có tên

Các test dưới là tên đề xuất, là gate bắt buộc của phase:

- `TestLoginRotatesSessionAndHashesToken`: mỗi login sinh ID/32-byte secret mới, không reuse cookie attacker gửi; DB chỉ có SHA-256(secret), token plaintext không trong row/log. Nguồn hiện có `apps/noverse/internal/persistence/postgres/sessions.go:110-128`; giữ compare timing-safe tại `:324-325,355-363`, không đổi thành string equality.
- `TestUnknownUserRunsPasswordVerifier`: unknown user và wrong-password đều qua Argon2 verifier/dummy hash, cùng generic status/message; kiểm invocation đường hash thay vì timing benchmark flaky. Trace hiện có `sessions.go:154-178`, `platform/identity/password.go:25-32,71-74`.
- `TestPasswordChangeRevokesEverySession`: đổi thường/đổi temporary/reset/recovery/disable/revoke-all làm mọi session cũ fail Authenticate, kể cả current; login mới flag must-change đúng. Loại tham số keepSession khỏi ChangePassword ở Repository/Store/backend, không giữ shim. Các caller cần đổi: `apps/noverse/internal/entry/web/auth.go:296`; `apps/noverse/internal/persistence/postgres/accounts_test.go:122`; `platform/identity/store.go:1135`; `platform/identity/contract_test.go:97`; `platform/identity/store_test.go:134,138,291`; `platform/identity/identitytest/suite.go:189,190,230`. Cập nhật assertion giữ phiên cũ tại `contract_test.go:100`, `store_test.go:141`, `identitytest/suite.go:191-192,231-233` thành revoked + login lại, không xoá coverage.
- `TestPrivilegeChangeRevokesAllSessions`: thay global role/grant thực sự và audit/revoke commit cùng nhau; no-op không revoke; nếu tx fail mọi thứ không đổi. Grant HTTP chỉ global admin, actor bị demote cùng lúc không vượt kiểm trong tx: `TestAppGrantRequiresGlobalAdminAndAudits`.
- `TestViderseIdentityRoleCannotReadNoverseOrPromoteUser`: role thật không SELECT projects/stories/project_members/audit_logs hoặc join/view bắc cầu, không UPDATE users.global_role/status, không DDL/SET ROLE migrate; password clear flag vẫn được. Không thay test thành check ProjectAccess trong RAM.
- `TestSharedLimiterAcrossInstances`: luân phiên hai instance và casing username vẫn một ngân sách username/IP; `TestConcurrentFirstAdminAcrossRepositories`: race chỉ một admin/session, loser ErrExists.
- `TestIdentityErrorsAndLogsRedactSecrets`: inject DSN parse/connect/query errors và capture logs; không token, password, hash hay DSN có credential. Query lỗi không biến thành unauthenticated.

Password change/revoke-all là **breaking security contract đã được owner yêu cầu**: đồng bộ file/PG/backend/facade và suite. Không có production caller cần giữ session cho “revoke all”, nên bỏ luôn tham số keep của RevokeAll (không shim) ở `platform/identity/repository.go:39`, `platform/identity/members.go:357-359` và PG method tương ứng; đổi HTTP caller `apps/noverse/internal/entry/web/auth.go:313,501-505` và contract caller/assertions `platform/identity/identitytest/suite.go:175-178`. Tác vụ “đăng xuất thiết bị khác” UI có thể tiếp tục revoke từng session khác bằng RevokeSession, không dùng một API “all” giữ current ngầm.

## Rủi ro và rollback

| Rủi ro | Xác suất × tác động | Giảm thiểu / tín hiệu dừng |
|---|---|---|
| Move phá owner invariant hoặc helper dùng chung | Cao × Cao | Giữ transaction và contract race hiện có; grep symbol trước xoá; không duyệt nếu test owner/last-admin thay đổi kết quả |
| Viderse vô tình đọc project/audit | Cao × Cao | ProjectAccess false + test query path + DB role chặn; thiếu quyền phải sửa query, không nới GRANT |
| User/grant bị cache theo process | Vừa × Cao | Options bất biến; nạp claims từ DB từng request; test hai instance và hai user xen kẽ |
| Migration chạy trước provision role | Vừa × Vừa | Migration schema không bắt buộc role tồn tại; role script chạy sau migrate, trước serve; startup contract fail rõ |

Rollback phase: revert code move theo một commit có toàn bộ caller; **giữ** bảng app_grants additive và rows backfill khi chưa bật grant gate. Thu hồi role mới nếu chưa dùng. Không goose down trên DB có writers và không phục hồi user/password bằng dữ liệu cũ chỉ để revert code.
