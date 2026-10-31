# Phase 02 · Noverse: cookie chung, grant gate, CSRF và worker secrets

- **Trạng thái:** pending. **Ước lượng:** 28h.
- **Phụ thuộc:** 01 đã bàn giao schema/repository/grants. **Chặn:** 03–05.
- **Owner:** kỹ sư Noverse/web security. Nhận lại tệp CLI/repository cần chỉnh sau 01, không sửa đồng thời.

## Bối cảnh và luồng hiện có

- `Gateway.ServeHTTP` chuẩn hoá proxy → health/pair redirect → CSRF → static/JSON → login/pair/recovery/setup công khai → đọc cookie/Authenticate → feedback → authorize → route (`apps/noverse/internal/entry/web/gateway.go:607-725`). Grant gate phải nằm **sau Authenticate nhưng trước feedback/handler nghiệp vụ**; không chỉ đặt ở `/api/projects`.
- Cookie hiện host-only tên `ainovel_session`, set/clear là hàm tự do (`apps/noverse/internal/entry/web/auth.go:18,69-75`). Me đã trả user qua view (`:249-259`); logout revoke trước clear (`:242-248`). Forced-password gate hiện có ở `authorization.go:116-119`.
- `AllowedOrigin` hosted chỉ nhận PUBLIC_URL, local chỉ nhận own host:port/IP (`platform/webguard/webguard.go:225-242`); `CrossSiteRequest` luôn chặn `Sec-Fetch-Site:cross-site` trước Origin (`:249-262`). Không cần cross-app POST để cookie SSO hoạt động: mỗi app gọi API cùng origin. Allowlist là mở tin cậy **có chủ ý**, không phải lý do bật CORS toàn hệ thống.
- Hosted worker hiện nhận cả `os.Environ()` lẫn DSN app qua DATABASE_URL (`apps/noverse/internal/entry/web/gateway.go:885-898`); worker dùng DSN để mở **facts**, không mở identity (`run_worker.go:124-155`, Resolve actor nil ở `:140-144`). Vì DB giờ chứa auth chung, không chuyển DSN account/server-app sang child.

## Tệp chính xác

- `platform/webguard/{webguard.go,webguard_test.go}`; mới `platform/webguard/session-cookie.go`, `session-cookie_test.go` cho cấu hình cookie chung và helpers thuần.
- `apps/noverse/internal/entry/web/{auth.go,gateway.go,authorization.go,setup.go,recovery.go,notify.go,events.go,run_worker.go,httperr.go}`; `auth_test.go`, `proxy_test.go`, `gateway_test.go`, `members_ad_test.go`, `accounts_backend_test.go`, `security_review_test.go`, `storage_cutover_test.go` và test worker env mới.
- `apps/noverse/cmd/ainovel-cli/{serve.go,main.go}`; `serve_members_test.go`; `apps/noverse/internal/bootstrap/{storage.go,storage_test.go}` nếu file test hiện hữu cần bổ sung; mới shared CLI `platform/identity/identitycli/grants.go` và test (tên package đề xuất).
- `apps/noverse/web/src/{App.tsx,api/types.ts,api/client.ts,api/session.tsx,screens/settings/members.tsx,screens/settings/members.test.tsx,screens/settings/members.styles.ts}` và `i18n/vi.ts`, `i18n/members.ts`, `screens/auth/{index.tsx,parts.tsx}`, `api/storage.ts`, `router.ts`; thêm password consumer `screens/account/DevicesDialog.tsx`, `screens/more/mobile/Password.tsx`, `screens/onboard/index.tsx`. Test session/app/return target tương ứng, tạo access-denied component nhỏ trong screens/auth nếu cần, không nhân bản login.
- Mới `apps/noverse/deploy/postgres-init/03-engine-worker-role.sql`; `apps/noverse/deploy/{README.md,.env.example,docker-compose.dokploy.yml,dsn-entrypoint.sh}` để topology thử/deploy cung cấp DSN worker riêng. Phase 05 hoàn thiện docs sau tích hợp; phase này ghi hợp đồng bảo mật cạnh code/script.

## Hợp đồng cấu hình cookie và CSRF

1. Thêm vào `webguard.Config` các giá trị parse một lần: CookieDomain và danh sách AppOrigins; không dùng biến global. `ConfigFromEnv` đọc:
   - `VERSE_COOKIE_DOMAIN`: local để rỗng; hosted subdomain dùng domain cha, chấp nhận dấu chấm đầu rồi canonicalise về một dạng. Không scheme, path, port, IP, wildcard, userinfo; không public suffix (`com`, `co.uk`), domain đơn nhãn hoặc domain không chứa PUBLIC_URL/app origins theo ranh giới nhãn DNS. Dùng public-suffix parser thay vì kiểm chuỗi đuôi ngây thơ.
   - `VERSE_APP_ORIGINS`: comma-separated absolute origin, canonicalise scheme/host/effective port; cấm `*`, `null`, userinfo, query, fragment và path khác rỗng hoặc `/`. Hosted chỉ HTTPS, cùng cookie domain nếu có. Local allowlist chỉ localhost/IP cụ thể; own-origin vẫn theo luật DNS-rebinding hiện có.
   - `PUBLIC_URL` vẫn là URL của **app đang chạy**, không thay bằng URL chung. `SECURE_COOKIES=true` và HTTPS bắt buộc khi CookieDomain không rỗng. Startup fail khi sai; không silently ignore mục lỗi.
2. Origin được nhận = own origin theo luật hiện có **hợp** explicit AppOrigins. Không tự cho phép mọi subdomain của CookieDomain. Giữ chặn cross-site metadata dù origin có trong allowlist; hai app phải cùng schemeful site, không hỗ trợ SSO hai eTLD+1 khác nhau.
3. Helper set/clear dùng cùng cấu hình: Name `verse_session`, Path `/`, HttpOnly, SameSite Strict, Secure theo normalized request/SECURE_COOKIES; Domain chỉ khi hosted. Strict vẫn cho hai app cùng schemeful site điều hướng qua lại; không cần đổi Lax chỉ để SSO. `__Host-` không tương thích Domain và HTTP local, nên không đổi tên đã chốt. Clear khớp Domain/Path, MaxAge=-1 và expiry quá khứ.
4. **Chỉ local mode:** hai app bind 127.0.0.1, khác cổng; ví dụ DB thử Noverse http://127.0.0.1:18790 và Viderse http://127.0.0.1:18789, CookieDomain rỗng, AppOrigins đúng hai URL. Local SSO không nhận -lan/0.0.0.0. Cookie gửi tới mọi cổng host này, kể cả server local không đáng tin: docs phải cảnh báo; không trộn localhost/IP/IPv6.
5. **Hosted mode:** HTTPS qua Traefik, cookie parent Domain/Secure, own PUBLIC_URL và exact allowlist; app có thể bind 0.0.0.0:8080 trong private container network, **không publish HTTP app port ra host/public**. Compose hiện làm đúng: app expose8080 (`apps/noverse/deploy/docker-compose.dokploy.yml:187-193`), chỉ publish private TTS worker8081 (`:194-198`), public HTTP/TLS ở Traefik (`:201-215`). Giữ invariant này cho cả hai app, không ép hosted bind loopback. Allowlist CSRF không tự thêm CORS.
6. Noverse `STORE_DRIVER=file` độc lập giữ `ainovel_session` và quyền hiện có; set CookieDomain SSO trong file mode là cấu hình lỗi. Postgres chỉ đọc/ghi `verse_session`, không chấp nhận legacy cookie. Bộ chọn cookie bất biến trên Gateway theo backend, không bị request/app khác thay đổi. Khi dùng trong HTTP production, constructor phải được truyền backend mode rõ ràng, không đoán bằng chuỗi Path().

## Gate quyền và HTTP/UI

- Thêm helper quyền app chung ở identity từ phase 01; Gateway áp cho `noverse`. Admin active luôn qua, member phải có row. Disabled/deleted đã không Authenticate được; thiếu grant là **403**, lỗi DB **503**, không được clear shared cookie trên hai nhánh đó.
- Phân loại auth-neutral chính xác: `GET /api/me` trả 200 nếu có grant, 403 body có safe user/grants nếu thiếu; `POST /api/logout`, `POST /api/me/password`, `GET /api/me/sessions`, `POST /api/me/sessions/revoke-all`, `POST /api/sessions/{id}/revoke` chỉ được đụng phiên của chính mình. Nhánh no-grant phải chặn admin session revoke người khác, profile/onboard/project/config/SSE/pairing tạo mới.
- Thứ tự sau Authenticate: xử lý auth-neutral an toàn → grant app → forced password → project/admin authorization → feedback/dispatch. Giữ forced-password exception tương đương hiện có, nhưng method/path exact, không cho một prefix tuỳ ý đi qua. Thiếu grant và must-change đồng thời: `/api/me` vẫn 403 app_access_denied; cho đổi mật khẩu trước nhưng đổi xong vẫn không có grant.
- Login, pair, recovery và setup thành công dùng cookie chung ở PG, view có grants. Login đúng credential nhưng no-grant set cookie rồi trả 403; không trả 401, không cộng lượt limiter sai. Pair/recovery không bypass grant khi truy cập nghiệp vụ.
- Sau password change thành công, revoke **tất cả** sessions theo phase 01, clear cookie chung và trả `{ok:true,reauthenticate:true}`; không refresh me rồi báo lỗi 401 như thể đổi mật khẩu thất bại. Các UI caller bắt buộc sửa: `apps/noverse/web/src/screens/account/DevicesDialog.tsx:48-50`, `screens/more/mobile/Password.tsx:33-35`, `screens/onboard/index.tsx:445-447`; hiển thị “Đã đổi mật khẩu. Vui lòng đăng nhập lại.” và xoá session UI. Revoke-all endpoints cũng không giữ current.ID. Login SSO clear cookie cũ ainovel_session/viderse_session ở host-only và Domain scope đã cấu hình, không đọc fallback.
- Thông báo thiếu quyền: **“Tài khoản của bạn chưa được cấp quyền dùng Noverse. Vui lòng liên hệ quản trị viên.”** Body `{code:"app_access_denied", app:"noverse", error, user}`. Giao diện hiển thị người đang đăng nhập, nút thử lại và Đăng xuất; giữ user/grants thay vì redirect login. Không mount library/data/SSE khi access denied. Chỉ 401 kích hoạt luồng session-expired.
- Me/login/UserListView có `user.grants`; UI Thành viên thêm hai checkbox quyền Noverse/Viderse, admin hiển thị kế thừa readonly. Member tạo mới mặc định chọn Noverse, Viderse chỉ khi admin chọn. Row grants độc lập với Projects; mất app grant không xoá membership dự án.
- Endpoint mới **đề xuất**: `PUT /api/users/{username}/grants/{app}` với `{role:"member"}`; `DELETE` cùng path revoke. Actor lấy từ session, global admin kiểm lại trong transaction. GET `/api/users` đã trả grants; không thêm endpoint list dư. Grant mutation không nhận `global_role`, `projects`, arbitrary app hay actor từ body.
- UI lưu grant riêng qua endpoints mới và refresh user/grants sau save; lỗi một request không hiển thị toàn bộ batch là thành công. Cho revoke Noverse grant của member nhưng không làm admin tự mất quyền ngầm. Checkbox admin không cấp row giả.

### Caller cookie phải migrate rõ ràng

Chuyển set/clear thành method Gateway hoặc helper nhận config bất biến; không global config:
- Set: `apps/noverse/internal/entry/web/auth.go:125,152`; `recovery.go:54`; `setup.go:195`.
- Clear: `auth.go:247,334`; `gateway.go:660`; `notify.go:484`.
- Read: `gateway.go:652`; `authorization.go:144`; `events.go:524`.
- Hằng/flags assertion trong `gateway_test.go:132-134`, helper `auth_test.go:41,54`; giữ các test khác dùng constant được chọn cho fixture mode thay vì thay chuỗi tên cookie mù quáng.

Recheck của stream phải kiểm cả grant Noverse: `authorization.go:157-158`, và `events.go:523` trở đi. Request mới bị chặn ngay; stream đã mở đóng ở nhịp recheck hiện có, không hứa tức thời trước nhịp đó.

## CLI grant tối thiểu trong cả hai app

Shared parser/service mới ở `platform/identity/identitycli`:

```text
ainovel-cli serve user grant <username> --app noverse|viderse
ainovel-cli serve user revoke <username> --app noverse|viderse
viderse user grant <username> --app noverse|viderse
viderse user revoke <username> --app noverse|viderse
```

Noverse dùng DATABASE_URL với STORE_DRIVER=postgres; Viderse dùng VERSE_IDENTITY_DATABASE_URL. Theo convention `runServeUser` hiện có (`apps/noverse/cmd/ainovel-cli/serve.go:316-337`), đây là **operator cục bộ giữ credential DB**, không phải browser user API. Audit actor system, granted_by NULL; không nhận `--actor admin` để giả danh. Không password/DSN trên argv hoặc stdout. Thiếu --app/user/backend PG → lỗi nonzero, không đoán app; lệnh không tạo user, nâng global role, khoá hay xoá tài khoản. Phase 03 chỉ nối Viderse dispatcher, không sao chép parser. Người giữ DSN identity là trusted identity operator; hạn chế file/env secret là ranh giới vận hành, không quảng cáo đây là DB role chỉ đọc session.

## Worker: tách DSN và không kế thừa bí mật thừa

- Thêm `NOVERSE_WORKER_DATABASE_URL` vào StorageConfig, chỉ server parent giữ. Hosted launch cần DSN riêng; thiếu thì startup/hosted capability fail rõ, không fallback DATABASE_URL app. Parent truyền child `DATABASE_URL=<worker DSN>` vì `openWorkerStorage` đọc contract hiện hữu (`run_worker.go:129-136`). Không đổi protocol facts/lease.
- Tạo role mới `noverse_engine_worker` không kế thừa noverse_app/migrate. Script explicit grants: SELECT `noverse_meta`, `goose_db_version`, `projects`; SELECT/UPDATE `stories`; SELECT/INSERT/UPDATE/DELETE trên `project_documents`, `volumes`, `chapters`, `outlines`, `characters`, `comments`, `comment_replies`, `reviews`, `review_decisions`, `story_leases`, `story_state`, `story_artifacts`, `story_events`, `checkpoints`, `agent_sessions`; SELECT/INSERT trên `chapter_versions` (history không UPDATE/DELETE). Đối chiếu từng query worker khi triển khai để **thu hẹp**, không thêm ALL TABLES/default privilege. Không grant bảng account/grants/audit/blobs/jobs nếu chưa có worker path thực sự cần; không DDL. Các bảng narrative có nguồn `00003_narrative.sql:58-366`; reset stream giữ marker hiện có, không cấp maintenance membership.
- Thay `append(os.Environ(), …)` bằng builder env lọc có chủ ý cho **cả local và hosted** (`gateway.go:869-871,886-890`). Loại mọi occurrence của DATABASE_URL trước đặt worker DSN; loại MIGRATION_DATABASE_URL, VERSE_IDENTITY_DATABASE_URL, VIDERSE_DATABASE_URL, VIDERSE_MIGRATION_DATABASE_URL, NOVERSE_WORKER_DATABASE_URL, credential bootstrap/provision DB và secret deploy server. Không giữ duplicate env key.
- Chỉ giữ env hệ thống cần chạy engine/provider như HOME/PATH/TMPDIR/locale, cấu hình provider thực sự engine dùng và config root; không truyền secret SSO/migrate/S3/admin chỉ vì nó ở parent. Khi triển khai lập danh sách exact provider keys theo bootstrap/provider đang dùng và test allowlist đó; không dùng một wildcard “mọi *_KEY”. Worker vẫn được dùng OAuth/config AI theo cơ chế riêng, không đổi credential nhà cung cấp trong scope này.
- Test capture env trước spawn; kiểm worker role không đọc users/user_credentials/sessions/app_grants, nhưng mở story, tự lấy lease, đọc/ghi facts thực sự được. Đây là engine child, **không phải** HTTP TTS worker ở `apps/noverse/README.md:451-455`.

## Trình tự và validation

1. Viết bảng test parser/origin/cookie local-hosted và grant middleware trước; sửa Config/helper rồi gắn Noverse PG/file cookie selection.
2. Gắn grant gate đúng control flow, me/login/neutral exceptions/error mapping; giữ authorisation project nguyên trạng sau gate app.
3. Thêm endpoint/CLI grants và UI Thành viên + denied state; kiểm no-grant không gọi pendingLessons/feedback/project query.
4. Tách worker role/env/DSN và compose/entrypoint test topology. Không ghi credential vào fixtures.
5. Kiểm unit matrix: {POST,PUT,PATCH,DELETE} × {local,hosted} × {own,allowlisted sibling,non-allowlisted sibling,foreign,null,missing Origin} × {same-origin,same-site,cross-site,missing metadata}. Own/allowlisted + không cross-site được qua; không Origin giữ convention CLI hiện có. Cấm wildcard/malformed config; spoof proxy không đổi Secure.
6. Handler: member grant/no-grant, admin không row, demote admin, forced-password, revoke và DB down; no-grant me 403 vẫn user.grants, logout không grant vẫn revoke, grant UI không đụng Projects.
7. Browser component: 403 không login loop/SSE, 401 vẫn logout gate, checkbox và lỗi save, file mode không hiển thị SSO controls. Test cookie jar host/port và deletion Domain/Path trùng.
8. Gate do main chạy: `go test ./platform/webguard/... ./platform/identity/identitycli/... ./apps/noverse/cmd/ainovel-cli/... ./apps/noverse/internal/entry/web/...`; PostgreSQL bật REQUIRE_POSTGRES_TEST=1; `npm run typecheck -w apps/noverse/web`; `npm test -w apps/noverse/web`. Không chạy live.

## Kiểm bảo mật bắt buộc và test có tên

- `TestSharedCookiePolicy`: host-only local, exact parent Domain hosted, HttpOnly/Strict/Secure, clear scope khớp; cookie Strict qua hai app cùng site. `TestSSOLocalBindRequiresLoopback`: chỉ local SSO reject wildcard/LAN; `TestHostedAppHTTPPortIsNotPublished`: hosted private binding được nhưng không host port publish8080. File-mode độc lập không bị đổi ngầm.
- `TestSSOOriginAllowlistMatrix`: exact scheme+host+effective port; cấm null, suffix/lookalike, userinfo, scheme-relative, wildcard, malformed origin; cross-site metadata bị chặn kể cả allowlisted; non-allowlisted same-host khác port vẫn 403 và DB không đổi.
- `TestSSOReturnTargetRejectsExternal`: không thêm redirect SSO liên app. Noverse lưu next-route raw tại `apps/noverse/web/src/api/storage.ts:25-28`, `authDestination` đọc raw tại `screens/auth/parts.tsx:65-68` nhưng router chỉ sửa hash URL hiện tại (`router.ts:118-123`); giữ flow nội bộ này và validate next là local path hợp lệ, không biến thành location.href external. Link liên app mới nếu có dùng exact origin allowlist, cấm //evil/javascript/encoded bypass. Không sửa OAuth callback/QR khác mục tiêu.
- `TestAppGrantHTTPRequiresGlobalAdmin`: member/forged body/actor vừa bị demote không grant được; mọi thay quyền thật có audit + revoke target sessions. `TestGrantCLIOperatorBoundary`: CLI chỉ grant/revoke, không nhận actor/role nâng quyền và không đọc DSN từ argv.
- `TestSharedLogoutRevokesBeforeClear`: logout và revoke-all từ Noverse làm old cookie 401, DB lỗi không báo thành công giả; password change success clear + reauthenticate, không giữ current.
- `TestSSOLoginClearsLegacyCookies`: cookie cũ được expire, cookie mới không reuse ID/secret người gọi cung cấp. `TestForcedPasswordGateEveryBusinessRoute`: enum route/method, upload/SSE/unknown write không bypass; chỉ exact neutral exceptions được qua.
- `TestHostedWorkerEnvExcludesIdentitySecrets`: capture env cả local/hosted, inject duplicate/unknown secret DSN; child chỉ có worker DATABASE_URL và engine-required keys. `TestWorkerRoleCannotReadIdentity`: role worker facts/lease thật chạy được nhưng account/grant SELECT và migration không được.
- `TestIdentityErrorsAndLogsRedactSecrets`: capture HTTP/access/spawn logs có request Cookie/body/DSN errors, tuyệt đối không chứa raw session token/password/hash/DSN credential. Không log body login/setup/password ở middleware feedback.

## Rủi ro và rollback

| Rủi ro | Xác suất × tác động | Giảm thiểu |
|---|---|---|
| Domain rộng làm app không tin cậy nhận session | Vừa × Cao | Chỉ domain cha sở hữu hoàn toàn; public-suffix validation; explicit origins; không host tenant không tin cậy dưới domain này |
| Đặt gate sau handler gây ghi trái phép | Vừa × Cao | Trace gateway và test spy: no-grant không gọi feedback/domain; negative mutation kiểm DB không đổi |
| Env append giữ DSN cũ/worker mất quyền facts | Cao × Cao | Builder loại key trùng; worker DSN fail closed; integration facts/lease + negative account reads |
| Standalone file Noverse và PG cookie trộn | Vừa × Cao | Backend cookie selection bất biến, không dual-read; chỉ bật SSO sau cutover cả hai |

Rollback: revert HTTP/UI/config commit cùng nhau khi hai app đã dừng; giữ schema additive. Khi đã phát cookie domain, clear đúng domain/path và buộc đăng nhập lại binary cũ, không chỉ đổi tên. Worker role có thể giữ; khi revert worker hardening phải ghi rõ việc quay lại rủi ro DSN cũ, không tự redeploy ra live. Không rollback grants/password rows bằng cách nạp snapshot cũ trong lúc app còn ghi.
