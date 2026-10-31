---
title: "SSO Noverse và Viderse trên một kho tài khoản Postgres"
description: "Dùng chung verse_session, kiểm quyền từng app ở mỗi request, di trú tài khoản an toàn và cutover do chủ máy thực hiện."
status: pending
priority: P1
effort: 144h
branch: main
tags: [identity, sso, security, postgres, noverse, viderse]
created: 2026-10-31
---

# Kết quả cần đạt

Đăng nhập bằng giao diện của một app, mở app còn lại trong cùng trình duyệt mà không đăng nhập lại. Hai app dùng **một kho tài khoản Postgres của Noverse và cookie `verse_session`**. Đăng xuất, thu hồi phiên, khoá tài khoản và đổi mật khẩu được hai app đọc lại từ DB ở request kế tiếp. Admin nền tảng quản trị cả hai app; thành viên cần grant riêng `noverse` và `viderse`.

Đây là **kế hoạch, chưa triển khai**. Các trích dẫn là mã nguồn đã đọc ngày 2026-10-31, không phải bằng chứng test đã chạy. Tên tệp/symbol mới ghi “mới/đề xuất” chưa tồn tại. Không chạy build, test, server, migration hay thao tác live trong lần lập kế hoạch này.

Kế hoạch này tiếp nối §4d từng hoãn ở `plans/261029-1138-verse-monorepo/phase-04-security-identity-viderse.md:137-153`. Thay lựa chọn kho/file grants còn mở ở ghi chú cũ bằng quyết định đã chốt: Postgres hiện có + app_grants trong DB; không tạo apps.json/grants.json.

# Ranh giới đã khoá

- Schema tài khoản hiện có vẫn ở DB Noverse; migration do Noverse sở hữu. Chuyển repository sang `platform/identity/postgres`, không để Viderse import `apps/noverse/**`. Ranh giới này hiện được mô tả ở `README.md:9-11` và scanner tính cả test tại `archtest/boundary_test.go:32-35`.
- Viderse giữ DB nghiệp vụ riêng; thêm `VERSE_IDENTITY_DATABASE_URL` tới DB Noverse bằng role **`viderse_identity`**. Không dùng DSN migrate hay `noverse_app` cho Viderse.
- `app_grants(user_id, app, role, granted_by, granted_at)` chỉ có grant `member`; `global_role=admin` cho quyền admin hiệu lực ở cả hai app. Không thêm app-admin độc lập hoặc partition project cho Viderse.
- Cả hai app giữ form login và gọi **`/api/login` của chính mình**. Không thêm IdP, JWT, redirect login, CORS wildcard hay chia sẻ OAuth nhà cung cấp AI.
- Local: hai app bind **127.0.0.1**, khác cổng, cookie host-only không Domain; cookie bỏ qua cổng nhưng localhost/IP không chia sẻ. Hosted: HTTPS qua Traefik, parent VERSE_COOKIE_DOMAIN + Secure, explicit VERSE_APP_ORIGINS; private container bind0.0.0.0:8080 được nhưng **không publish app HTTP port ra host/public**, chỉ Traefik public (`apps/noverse/deploy/docker-compose.dokploy.yml:187-215`).
- Chỉ SSO khi Noverse dùng `STORE_DRIVER=postgres`. Giữ chế độ file độc lập của Noverse cho người chưa cutover, cookie cũ `ainovel_session` chỉ trong chế độ đó; không đọc cookie cũ như fallback trong chế độ Postgres. Không quảng cáo file mode là SSO. Viderse sau cutover không tự rơi về file.
- Không chạm `/Users/dat/Projects/verse-live`, launchd `dev.verse.noverse-live`, `~/.ainovel`, `~/.viderse`. Mọi test dùng HOME/DB tạm. Runbook và lệnh cutover chỉ được viết trong repo; **chủ máy tự chạy**.

# Các phase và phụ thuộc

| Phase | Kết quả | Chặn bởi | Ước lượng | Trạng thái |
|---|---|---|---:|---|
| [01 · Repository chung và grant schema](phase-01-shared-identity-and-grants.md) | Repository, limiter, quyền app, SQL role | — | 32h | pending |
| [02 · Noverse, cookie, CSRF và quản trị grant](phase-02-noverse-cookie-and-authorization.md) | HTTP/UI Noverse, worker không nhận credential tài khoản | 01 | 28h | pending |
| [03 · Viderse dùng kho chung](phase-03-viderse-shared-store.md) | Hai pool riêng, setup nguyên tử, grant/đổi mật khẩu bắt buộc, UI | 01, 02 | 28h | pending |
| [04 · Lệnh nhập và runbook cutover](phase-04-migration-and-cutover.md) | v4 Noverse + file Viderse, báo trùng, backup, phục hồi | 01–03 | 40h | pending |
| [05 · Nghiệm thu hai app và tài liệu](phase-05-smoke-and-documentation.md) | Test thật hai server/một browser, docs/env/deploy | 01–04 | 16h | pending |

**Tổng 144h**, gồm triển khai, test và review từng phase; không gồm thời gian chủ máy chọn cửa sổ bảo trì hay DNS thực tế. Thực hiện tuần tự 01 → 02 → 03 → 04 → 05. Không có hai phase sửa tệp đồng thời; mỗi phase bàn giao ownership trước khi phase kế tiếp nhận các tệp lặp lại. Không triển khai riêng phase 02 hoặc 03 ra live.

# Luồng dữ liệu và hợp đồng chung

1. Form login gửi username/password tới API cùng origin. Gateway chuẩn hoá proxy, kiểm CSRF và JSON trước login. Hai app dùng cùng key limiter DB, xác minh Argon2, tạo session opaque và cookie chung; không đưa password/hash vào response.
2. Request có cookie → repository đọc session, user, grant hiện tại → phân loại 401/403 → kiểm bắt buộc đổi mật khẩu → quyền nghiệp vụ. Noverse đọc thêm membership project; Viderse không phát query project. Không cache grant trong session hay biến dùng chung giữa request.
3. `GET /api/me` có quyền app → 200, `user.grants` là map quyền **hiệu lực** (`admin` hoặc `member`). Không có grant → 403 `{code:"app_access_denied", app, error, user}`; `user` là view an toàn, vẫn có grants. Không xoá cookie và không biến 403 thành hết phiên. Không có/phiên bị thu hồi → 401. Lỗi DB → 503, không giả thành 401 hoặc kích hoạt setup.
4. Login đúng credential nhưng thiếu grant vẫn tạo cookie để có thể mở app được cấp; trả 403 cùng payload trên. Sai credential → lỗi generic hiện có. `/api/logout`, `/api/me/password` và API quản lý **phiên của chính mình** được phép với phiên hợp lệ dù thiếu grant; không được lách sang nghiệp vụ.
5. Logout revoke session trong DB rồi xoá cookie đúng Name/Path/Domain; “Đăng xuất mọi thiết bị”, revoke-all, đổi/reset mật khẩu và disable thu hồi mọi session của tài khoản, kể cả session đang dùng, nên cả hai app nhận 401 ở request kế tiếp. Đây là thay đổi bảo mật có chủ ý so với hợp đồng giữ current session tại `apps/noverse/internal/persistence/postgres/passwords.go:80-89`; đổi cả repository, HTTP, UI và test cũ, không giữ đường lách keepSession.
6. CLI và màn Thành viên Noverse sửa grant → mutation + audit + revoke toàn bộ phiên target trong cùng transaction. Thay quyền thực sự bắt buộc đăng nhập lại để có session ID mới; grant đọc lại không cache. UI quản trị grant chỉ ở Noverse; CLI có trong **cả hai binary**, theo mô hình operator máy chủ đáng tin cậy đang dùng tại `apps/noverse/cmd/ainovel-cli/serve.go:316-337`.
7. Công cụ di trú đọc snapshot file chỉ định rõ, kiểm trùng trên toàn bộ nguồn và DB, ghi backup `.v4.bak`, nhập hash/trạng thái/quyền, không nhập bearer session/pairing; report 0600 không chứa hash/DSN. Nếu Noverse đang file mode, cutover cả facts/blob theo importer hiện có vì `STORE_DRIVER` chọn trọn bộ (`apps/noverse/cmd/ainovel-cli/serve.go:258-276`).

# Tiêu chí nghiệm thu

- Một browser context, hai server `127.0.0.1` khác cổng: login Noverse → Viderse `/api/me` và API nghiệp vụ 200; kiểm chiều ngược lại; jar có một `verse_session` host-only.
- Logout/revoke current session từ app bất kỳ → cả hai app 401 cho session đó; đổi/reset mật khẩu, disable và revoke-all → **mọi session** cả hai app 401, kể cả current. Login tiếp tạo ID/secret mới.
- Member chưa có grant Viderse → 403 có giải thích tiếng Việt, không hiện form login lặp; mutation grant thu hồi session target, login lại thấy quyền mới. Grant Noverse không thay membership project.
- Hai app cùng setup trên DB trống chỉ tạo được **một** admin; bên thua 409, không tạo user/session/recovery mồ côi.
- Cùng limiter Postgres: chia lượt sai giữa hai app vẫn đạt cùng backoff; không có limiter RAM trên đường SSO.
- CSRF vẫn chặn mọi origin không allowlist (kể cả cùng host khác cổng), `Origin:null`, wildcard và `Sec-Fetch-Site:cross-site`. Origin allowlisted không tự tạo quyền nghiệp vụ hoặc CORS.
- Role Viderse login/setup/password/grants hoạt động nhưng SELECT/ghi project, story, audit history, DDL đều bị DB từ chối; hosted worker không nhận DSN identity/migrate/server-app.
- Nhập Noverse v4 và Viderse file giữ hash, user/tombstone, metadata, membership Noverse và grant theo nguồn; mọi collision khác provenance làm lệnh thất bại có report; backup đúng byte, rerun không đè dữ liệu.
- Runbook có dump/restore diễn tập, quy trình Noverse file → Postgres toàn bộ, cấm agent thao tác live, cổng dừng và xử lý dữ liệu phát sinh sau cutover; hai deploy README và usage/env được cập nhật.
- Review bảo mật độc lập toàn diff bởi vai trò `security-reviewer`, sửa và review lại đến **0 blocker/major trước commit**; evidence không có token/DSN/hash thật.

# Threat model bắt buộc

Hai app và mọi subdomain trong cookie domain là cùng một trust boundary identity. Cookie chung không cô lập được app bị chiếm; least-privilege DB giảm blast radius dữ liệu Noverse, không biến Viderse thành IdP không đáng tin cậy.

| Đe doạ | Kiểm soát đã chốt | Test cụ thể / nơi chịu trách nhiệm |
|---|---|---|
| Trộm/fixation session | Mỗi login sinh UUID/secret mới; password/privilege change revoke toàn bộ, login lại rotate; token chỉ hash SHA-256 at rest, so timing-safe. Có nguồn `apps/noverse/internal/persistence/postgres/sessions.go:110-128,320-325` | `TestLoginRotatesSessionAndHashesToken`, `TestPrivilegeChangeRevokesAllSessions` · 01 |
| Cookie bị script/network đọc | HttpOnly, Secure bắt buộc hosted, Path=/, SameSite=Strict. Chọn Strict vì hai app cùng schemeful site nên cross-app navigation vẫn gửi cookie; không cần Lax để đáp ứng scope. Lax chỉ có lợi cho liên kết từ website ngoài, không phải yêu cầu. Prefix `__Host-` không dùng được với Domain và HTTP local; tên đã chốt là verse_session | `TestSharedCookiePolicy`, `SSOHostedCookieNavigation` · 02,05 |
| App local khác nhận cookie | Local hai app bind127.0.0.1; mọi server trên host này có thể nhận bearer Cookie, nên không dùng profile SSO với server local không tin cậy; Origin không ngăn server đó đọc Cookie. Hosted khác: private container HTTP không host publish, chỉ Traefik public | `TestSSOLocalBindRequiresLoopback`, `SSOLocalCookiePortScope`, `TestHostedAppHTTPPortIsNotPublished` · 02,03,05 |
| CSRF giữa app hoặc origin độc hại | Exact canonical scheme+host+port allowlist, không wildcard/suffix matching; reject null; cross-site metadata luôn reject; own-origin vẫn DNS-rebinding guard | `TestSSOOriginAllowlistMatrix` · 02; `SSOCSRFNonAllowlistedOrigin` · 05 |
| Open redirect/return URL | Không thêm redirect liên app; nếu có link/returnTo phải exact allowlist origin và path local hợp lệ, cấm scheme-relative/userinfo/javascript. Không dùng input URL tuỳ ý | `TestSSOReturnTargetRejectsExternal` · 02; `SSOReturnTargetBoundary` · 05 |
| Tự nâng quyền/grant giả | Chỉ global admin HTTP, check trong tx; CLI là local operator giữ DB secret; audit mọi grant/revoke thành công thật; no-op không đổi quyền không tạo audit giả; quyền đổi revoke sessions | `TestAppGrantRequiresGlobalAdminAndAudits`, `TestGrantCLIOperatorBoundary` · 01,02 |
| Role Viderse đọc facts hay nâng global_role | Exact table/column/sequence grants; không UPDATE users.global_role, không project/story/audit SELECT/joins, không DDL/membership migrate | `TestViderseIdentityRoleCannotReadNoverseOrPromoteUser` · 01 |
| Session stale sau password/revoke/disable | Không per-app claims cache dài hơn một request; DB check mỗi request, invalidate current lẫn mọi phiên khác khi password/disable/revoke-all | `TestPasswordChangeRevokesEverySession`, `SSORevocationAcrossBothApps` · 01,05 |
| Logout không có tác dụng app kia | Revoke shared row trước clear đúng scope; logout-all thu hồi mọi row; DB lỗi không báo thành công giả | `TestSharedLogoutRevokesBeforeClear`, `SSOLogoutEverywhere` · 02,03,05 |
| Brute force luân phiên hai app | Cùng login_rate_limits, cùng username/IP keys/backoff, không namespace theo app; unknown user vẫn Argon2 dummy | `TestSharedLimiterAcrossInstances`, `TestUnknownUserRunsPasswordVerifier`, `SSOAlternatingLoginRateLimit` · 01,05 |
| Setup race | auth_invariant_lock, count + insert cùng tx; code process-local không phải authority | `TestConcurrentFirstAdminAcrossRepositories`, `SSOConcurrentSetupOneAdmin` · 01,03,05 |
| Cookie cũ trùng/sống lại | Clear hai tên cookie cũ khi login SSO, không dual-read; invalidate sessions/pairings cutover; Domain/Path clear khớp | `TestSSOLoginClearsLegacyCookies`, `TestSSOCutoverInvalidatesOldSessions` · 02,03,04 |
| Bypass forced password | Server gate exact auth-neutral methods trước mọi route nghiệp vụ, SSE/upload; UI chỉ hỗ trợ, không là ranh giới | `TestForcedPasswordGateEveryBusinessRoute`, `SSOForcedPasswordBothApps` · 02,03,05 |
| Worker/log làm lộ secrets | Worker DSN facts riêng, env allowlist không identity/app/migrate secrets; redacted log/error/report, không ghi raw token/hash/password | `TestHostedWorkerEnvExcludesIdentitySecrets`, `TestIdentityErrorsAndLogsRedactSecrets`, `TestMigrationReportContainsNoSecrets` · 01,02,04 |

Những tên test trên là **test đề xuất phải viết**, chưa chạy. Chi tiết input/expected nằm ở phase tương ứng; phase 05 kiểm toàn bộ mapping threat → evidence trước cho phép commit.

# Câu hỏi còn mở (không chặn viết code)

1. **Tên domain, URL/cổng và DSN live chưa xác minh** vì không đọc vùng live. Mặc định đề xuất: local dùng 127.0.0.1 ở cả hai app; hosted dùng hai subdomain HTTPS cùng domain cha, allowlist hai origin chính xác. Chủ máy điền và kiểm trước cutover; không đoán giá trị live.
2. **Username trùng giữa hai nguồn có phải cùng người?** Không suy luận qua tên/email/hash. Mặc định: fail toàn bộ preflight; chủ máy cung cấp rename map cho tài khoản khác người hoặc chọn một account đích và cấp grant thủ công sau khi xác minh cùng người. Công cụ không tự merge.
3. **Admin cũ của Viderse sẽ thành admin cả nền tảng.** Mặc định giữ role đã có nhưng chỉ apply khi chủ máy chấp nhận danh sách admin trong report; không âm thầm mở quyền Noverse. Có lựa chọn hạ member rõ ràng trong mapping đã duyệt trước apply.
4. **Kích hoạt plan theo hook runtime:** repo đích không có script activation. Plan path được bàn giao trực tiếp, không ghi session state/tệp ngoài thư mục plan.

# Rollback tổng thể

Trước mở ghi: dừng hai app ứng viên, khôi phục cấu hình/binary cũ, dùng file `.v4.bak` hoặc dump DB trước cutover; không chạy goose down trên DB có dữ liệu. Sau mở ghi: đóng băng cả hai app, sao lưu hậu-cutover trước mọi restore; ưu tiên revert binary tương thích schema mở rộng. Nếu buộc quay về file, phải đối soát thay đổi tài khoản/quyền/mật khẩu và facts phát sinh; không tuyên bố restore backup cũ là không mất dữ liệu. Chi tiết và cổng quyết định ở phase 04.
