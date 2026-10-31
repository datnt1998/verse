# Phase 03 · Viderse dùng account store chung

- **Trạng thái:** pending. **Ước lượng:** 28h.
- **Phụ thuộc:** 01 (repository/role/grants), 02 (cookie/origin/HTTP contract/shared CLI). **Chặn:** 04, 05.
- **Owner:** kỹ sư Viderse; nhận `platform/webapp` và các tệp Viderse, không sửa đồng thời với 01/02.

## Bối cảnh đã xác minh

- Serve mở file accounts tại `apps/viderse/cmd/viderse/serve.go:53,238-249`, pool nghiệp vụ riêng tại `:65-69`; nhận Config webguard tại `:60-62,79`. Phải đổi accounts thành một pool khác có closer riêng, không dùng lại pool data Viderse.
- Viderse tạo `webapp.New` với cookie `viderse_session` ở `apps/viderse/internal/web/gateway.go:44-50`. Shared server Authenticate từng request rồi dispatch (`platform/webapp/server.go:147-175`), nhưng **chưa có** grant gate hoặc forced password route/gate.
- HTTP hiện có login/logout/me/setup; flow setup dùng state process-local để kiểm mã nhưng DB CreateFirstAdmin mới là quyết định cuối (`platform/webapp/server.go:367-378,402-440`). Repo PG khoá singleton thật (`apps/noverse/internal/persistence/postgres/accounts.go:392-404`).
- Boot UI bắt mọi lỗi me rồi xoá user/hỏi setup (`apps/viderse/web/src/api/session.tsx:39-55`); Root gắn SSE chỉ dựa trên có user (`App.tsx:74-89`). Login chỉ set user khi 2xx (`screens/auth/LoginScreen.tsx:23-27`). Cần tách no-grant/forced-change/offline, không chữa bằng đổi 403 thành 401.
- SSE còn đọc cookie hardcode `viderse_session`, không qua Options (`apps/viderse/internal/web/gateway.go:194-200`); recheck 15 giây tại `:174-177`. Nếu chỉ đổi cookie ở constructor, luồng SSE sẽ tự đóng sai.
- `core/auth.Configure` là OAuth nhà cung cấp AI, không phải web identity (`apps/viderse/internal/bootstrap/bootstrap.go:18-29`); không chia sẻ hoặc nhập `auth.json`.

## Tệp chính xác

- `apps/viderse/cmd/viderse/{serve.go,main.go,serve_shutdown_test.go}`; mới `identity.go`, `identity_test.go` nối shared grant CLI và kiểm extra DSN.
- `apps/viderse/internal/bootstrap/bootstrap.go`; mới `identity_config_test.go` nếu cần test parser riêng.
- `apps/viderse/internal/web/{gateway.go,gateway_test.go}`; test SSE/grants thêm cạnh gateway, không import Noverse internal.
- `platform/webapp/{server.go,server_test.go}`; dùng helpers đã làm trong `platform/webguard/session-cookie.go`; thêm auth error mapping nếu file hiện tại chỉ map 401.
- `apps/viderse/web/src/{App.tsx,App.test.tsx,api/session.tsx,api/client.ts,api/client.test.ts,api/request.ts,api/types.ts,screens/auth/LoginScreen.tsx,screens/auth/SetupScreen.tsx,screens/account/AccountScreen.tsx,i18n/vi.ts}`; mới `screens/auth/AccessDeniedScreen.tsx`, `ChangePasswordScreen.tsx` và test tương ứng; session/request test theo convention Vitest hiện có.
- `apps/viderse/README.md`; mới `apps/viderse/deploy/README.md`, `.env.example` (repo hiện chưa có deploy directory, README hiện là owning setup guide `apps/viderse/README.md:20-40`). Phase 05 hoàn thiện liên kết/docs sau nghiệm thu.

## Cấu hình và vòng đời

1. Thêm **bắt buộc** `VERSE_IDENTITY_DATABASE_URL`. `VIDERSE_DATABASE_URL` và `VIDERSE_MIGRATION_DATABASE_URL` vẫn chỉ DB nghiệp vụ (`apps/viderse/internal/bootstrap/bootstrap.go:41-49`). Không fallback từ identity DSN sang một trong hai biến đó, DATABASE_URL Noverse, file HOME hoặc migration DSN.
2. `openAccounts` trả `(*identity.Store, close func(), error)` và dùng identitypg pool bằng role `viderse_identity`, `ProjectAccess=false`, identity.Options không legacy project. Pool ping + kiểm account/grant contract từ phase 01, không kiểm Noverse meta/goose bằng account role.
3. `runServe` parse toàn bộ cấu hình trước side effect: giữ cổng và kiểm kết nối trước start background workers; defer close cho cả hai pool ở mọi đường lỗi và shutdown. Giữ nguyên ý đồ cổng bận không tác động server đầu (`serve.go:80-86`). Test DSN lỗi/thiếu phải fail trước tạo file account hoặc mã setup.
4. Hai DSN phải thực sự trỏ đúng kho: fixture identity DB và data DB khác nhau, account rows không xuất hiện data DB, nghiệp vụ Viderse không ghi vào Noverse. Không đòi hai app biết DSN của nhau; dùng same identity fixture trong smoke để chứng minh cùng rows.
5. Serve không tự chạy migration Noverse. Startup schema lỗi chỉ hướng dẫn owner chạy `ainovel-cli storage migrate up` bằng role migrate. Secrets chỉ env/protected config, lỗi parse/connect không in raw DSN.
6. Shared webapp Options có định danh app `viderse` và cookie policy, không comment yêu cầu cookie unique per app nữa (`platform/webapp/server.go:29-31`). Không lưu user/grant của request vào `Server` process-scoped (`:55-65`).

## HTTP, grant và đổi mật khẩu bắt buộc

Luồng mới: Normalize → health → CSRF → static/JSON → login/setup → Authenticate → auth-neutral self-service → grant Viderse → must-change gate → handler nghiệp vụ. Không route nghiệp vụ nào đi trước hai gate, kể cả multipart và SSE.

- Dùng `verse_session` helpers phase 02 cho set, read, clear; xoá option cookie-name per-app ở caller SSO nếu không còn consumer độc lập. Không dual-read `viderse_session`. Unknown/expired/revoked/locked/deleted → 401; repository unavailable → 503, không clear cookie. Hiện webapp gom mọi Authenticate error thành 401 (`server.go:152-158`); phải sửa nhánh này.
- `/api/login` của Viderse kiểm **cùng** Postgres credential/limiter. Key/prefix không thêm `viderse`; đúng credential no-grant không tăng failure counter. Kết quả thiếu quyền 403 `{code:"app_access_denied",app:"viderse",error:"Tài khoản của bạn chưa được cấp quyền dùng Viderse. Vui lòng liên hệ quản trị viên.",user}` và set cookie chung.
- `/api/me` 200 có user.grants khi có quyền; không quyền 403 có safe user/grants. Admin global có quyền admin cả hai, explicit grants không nâng global role. Projects trong view Viderse rỗng, **không đọc** Noverse membership để dựng JSON.
- Cho phiên hợp lệ, kể cả thiếu app grant, gọi auth-neutral: `POST /api/logout`, `POST /api/me/password`, `GET /api/me/sessions`, `POST /api/me/sessions/revoke-all`, `POST /api/sessions/{id}/revoke` (chỉ owner session). Me dùng contract 200/403 trên. Không thêm `/api/users`, admin audit, QR/project APIs hoặc global update/delete.
- Password endpoint nhận `{current,new}`, gọi shared ChangePassword **không tham số keep session** theo phase 01; key limiter `password:<username>` giống Noverse (`apps/noverse/internal/entry/web/auth.go:291-305`). Sai current → 403, DB lỗi → 503; success revoke tất cả session, clear cookie, `{ok:true,reauthenticate:true}`. Temporary session vẫn đổi được khi mã tạm đã dùng/hết TTL (`apps/noverse/internal/persistence/postgres/passwords.go:25-32`), nhưng không sống sót sau đổi mật khẩu.
- Khi `MustChangePassword=true`, mọi business request trả 403 `{code:"password_change_required",mustChangePassword:true,error:"Bạn cần đổi mật khẩu tạm trước khi tiếp tục."}`. Chỉ exact auth-neutral đi qua; cả admin cũng chịu gate. Đổi xong xoá local user và yêu cầu đăng nhập lại; flag false trên DB, mọi old cookie hai app 401. Revoke-all cũng không giữ current.ID.
- Logout revoke trước clear; nếu DB không thể revoke thì trả lỗi, không báo giả đã đăng xuất trên cả hai. Logout với phiên đã bị thu hồi trả 401 và clear cookie đúng scope, client vẫn chuyển về login.
- SSE `eventSessionValid` đọc cookie qua helper chung và kiểm grant/forced-password sau Authenticate, không chỉ err==nil. Request mới deny ngay; stream cũ đóng ở nhịp 15 giây hiện có, không hứa instant cancellation. Không cần thêm bus revoke xuyên app cho yêu cầu “request kế tiếp”.

## Setup: app nào sở hữu admin đầu tiên?

**Store chung sở hữu quyết định**, không một app cố định sở hữu setup. Cả hai được hiện setup và in mã process-local khi store thật sự trống; code của app A chỉ đổi được setup token tại A, không dùng tại B. Bất kỳ app nào hoàn thành trước tạo **admin nền tảng**.

- Giữ DB lock auth_invariant_lock và kiểm trống trong cùng transaction CreateFirstAdmin; không dựa riêng vào `HasUsers` lúc startup.
- Bên thắng tạo một user/session + audit; bên thua 409 (có thời điểm nếu sẵn), invalidate local setup token và chuyển UI sang login/refresh. Không tạo recovery code hoặc session thứ hai ở loser.
- SetupStatus luôn hỏi DB (đã có ở `platform/webapp/server.go:315-323`), nên app khởi động trước winner cũng thấy needed=false. DB lỗi trả 503, **không** coi là trống để phát mã hoặc hiển thị setup.
- Recovery issue diễn ra sau create hiện tại (`server.go:433-441`): nếu issue lỗi, không rollback giả user đã commit. Hoàn tất set cookie/response rõ “đã tạo tài khoản nhưng chưa cấp được mã khôi phục”; không bắt owner setup lại hoặc in code giả. Cho admin đăng nhập lại và dùng luồng recovery Noverse có sẵn để phát mã. Test nhánh lỗi này, không mở thêm recovery UI Viderse ngoài scope.
- Nếu dữ liệu import đã có users, không có app nào được tạo admin mới qua setup, kể cả store chỉ có locked users. Preflight migration yêu cầu còn admin active.

## UI và CLI

- SessionState tách `user`, `accessDenied`, `mustChangePassword`, `serverReachable`, `setupNeeded`; grants đi cùng user. Chỉ 401 của me mới hỏi setup. 403 access-denied giữ user; 503/network hiển thị lỗi/thử lại, không login/setup giả.
- Root gating trước shell và trước startDataEvents: unchecked → loading; thật sự trống → Setup; unauthenticated → Login; denied → AccessDenied; must-change → ChangePassword; sau đó shell. Không mount data/SSE khi denied/restricted. Không cho hash route vượt password screen; nút bỏ qua không tồn tại.
- Login vẫn gọi own `/api/login` (`apps/viderse/web/src/api/client.ts:16`), nhưng catch 403 đọc safe user/grants để chuyển đúng denied screen. Giữ message 401/429 hiện có. Không redirect sang login Noverse.
- ChangePassword dùng component kit, nhãn tiếng Việt và xác nhận; success xoá session UI, hiển thị “Đã đổi mật khẩu. Vui lòng đăng nhập lại.”, không refresh me rồi báo thất bại giả. AccountScreen hiện chỉ profile/logout (`screens/account/AccountScreen.tsx:14-34,52-56`); thêm đổi mật khẩu thường bằng cùng form.
- AccessDenied có “Thử lại” (refresh me) và “Đăng xuất”; không tự huỷ cookie khiến app kia mất phiên. `request.ts:51-63` chỉ phát session-expired cho 401, thêm xử lý mã grant/forced-password ở session layer mà không coi mọi 403 business là mất quyền app.
- Nối `viderse user grant|revoke` vào shared CLI phase 02; operator dùng identity DSN, không pool nghiệp vụ. Không nhầm `viderse login/logout` (OAuth nhà cung cấp) ở `main.go:65-68` với web session/logout HTTP.

## Trình tự và validation

1. Test startup extra DSN/closer/role isolation; thay file account open và constructor gateway.
2. Test webapp cookie/grants/error classification, thêm neutral password/sessions routes và forced gate. Nối setup race contract và sửa error sau tạo admin.
3. Sửa SSE hardcode/recheck; kiểm worker/tool upload vẫn chịu app grant.
4. Cập nhật types/client/session/Root/login/password/denied UI, nối CLI rồi docs cấu hình.
5. Unit webapp: missing/revoked cookie, no-grant, global admin, restricted session, method bypass, multipart bypass, DB fail và state không lẫn hai user. UI Vitest: 403 không login loop; no-grant không SSE; forced password không skip; 503 không setup; logout message có tác động cả hai app.
6. Postgres với role Viderse thật: setup/IssueRecovery/login/Authenticate/ChangePassword/revoke/grant CLI chạy được; không SELECT project/audit; setup hai server đồng thời chỉ một admin. Temporary-password single-use và metadata preserved.
7. Test teardown đóng đủ hai pool khi một pool fail, listener bận hoặc shutdown; không tạo `~/.viderse/web-accounts.json` ở HOME tạm.
8. Main chạy `go test ./platform/webapp/... ./apps/viderse/cmd/viderse/... ./apps/viderse/internal/web/... ./archtest`; `npm run typecheck -w apps/viderse/web`; `npm test -w apps/viderse/web`. Integration bật REQUIRE_POSTGRES_TEST=1 + TEST_DATABASE_URL DB thử.

## Kiểm bảo mật bắt buộc và test có tên

- `TestViderseIdentityDSNNoFallback`: thiếu/sai extra DSN fail startup, không mở file/data/migrate identity. `TestSSOLocalBindRequiresLoopback`: local SSO chỉ 127.0.0.1, cấm wildcard/LAN; tài liệu cảnh báo server cùng host nhận cookie theo cổng. Hosted chấp nhận private container bind0.0.0.0 nhưng không publish app HTTP port, chỉ Traefik public.
- `TestForcedPasswordGateEveryBusinessRoute`: enum registered route/method gồm upload/SSE/unknown write; restricted user không chạy handler; auth-neutral self-service không cho revoke session người khác.
- `TestVidersePasswordChangeRevokesBothAppSessions`: role Viderse thật đổi mật khẩu bằng query clear flag column-limited, mọi old cookie revoked, cookie được xoá, UI relogin; không cấp UPDATE global_role chỉ để query thành công.
- `TestSharedLogoutRevokesBeforeClear`, `TestSSOLoginClearsLegacyCookies`: chạy với shared server; lỗi DB không báo logout thành công, legacy cookie không Authenticate được và được expire khi login.
- `TestConcurrentFirstAdminAcrossHTTPApps`: hai setup token process riêng, hai POST cùng lúc → một user/admin/session, loser 409, không recovery mồ côi. `TestSetupUnavailableIsNotEmpty`: DB fail → 503, không setup.
- `TestViderse403IsNotSessionExpired`: no-grant giữ cookie/user/grants và không SSE; grant mutation làm session target 401, login lại phản ánh 200/403. `TestViderseSessionClaimsAreRequestScoped`: user/grant request trước không lẫn sang request sau.
- `TestIdentityErrorsAndLogsRedactSecrets`: HTTP response/server logs không in token/password/hash/DSN kể cả setup/login/password/query fail.

## Rủi ro và rollback

| Rủi ro | Xác suất × tác động | Giảm thiểu |
|---|---|---|
| Dùng data/migrate DSN làm identity | Vừa × Cao | Extra DSN bắt buộc, role negative tests, hai DB fixture; không fallback |
| UI coi 403/503 là 401, làm hỏng phiên app kia | Cao × Cao | Phân loại code + giữ cookie/user, gate trước SSE, test login/refresh/expired |
| Setup race tạo hai admin | Vừa × Cao | Singleton tx lock; race cả hai HTTP server; loser 409 và không issue recovery |
| SSE vẫn đọc cookie cũ | Cao × Vừa | Đổi hardcode `gateway.go:195`, browser giữ luồng sau login app kia và test recheck |

Rollback: trước mở ghi, dừng hai app, quay binary/config Viderse cũ + file backup dưới quyền chủ máy; bỏ identity DSN khỏi runtime cũ. Không tái dùng old file nếu đã có password/grant thay đổi sau cutover mà chưa đối soát. Revert webapp và Viderse caller cùng commit; giữ schema/grants additive. Không import ngược DB về file bằng cách bỏ qua tombstone/credential đã đổi.
