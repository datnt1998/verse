# Phase 05 · Nghiệm thu hai app, tài liệu và security gate

- **Trạng thái:** pending. **Ước lượng:** 16h.
- **Phụ thuộc:** 01–04 đã bàn giao; migration/cutover diễn tập hoàn tất trong môi trường tạm.
- **Owner:** main/integration engineer, sau khi mọi phase đã land. Không chạy nhiều lượt build/test nặng trên half-finished edits của phase khác. `security-reviewer` là reviewer độc lập cuối, không tự duyệt diff mình viết.

## Bối cảnh và tệp sở hữu

- Workspace chưa có runner E2E ở root; `package.json:1-9` chỉ khai báo workspaces. Vitest/typecheck/build đã có ở `apps/noverse/web/package.json:6-16` và `apps/viderse/web/package.json:6-15`. Thêm runner nhỏ tái lập được, không thay bộ test cũ.
- UI thực tế lấy bản nhúng, không tự đọc source TSX: Noverse `apps/noverse/internal/entry/web/gateway.go:42-43,737`; Viderse `apps/viderse/internal/web/gateway.go:23-24,39`. Phải build web trước Go binary dùng smoke. Dist Viderse được commit theo `apps/viderse/README.md:105-113`; không coi UI test source pass là binary đã mới.
- Postgres harness có chế độ bắt buộc DB (`platform/pgtest/pgtest.go:21-33,101-104`) và schema tạm (`:7-8`); dùng cùng ý đồ không skip, không test bằng role migrate thay runtime.

**Tệp mới đề xuất:** `tests/sso/{playwright.config.ts,fixture.ts,sso.spec.ts,README.md}`; helper fixture Go dưới `tests/sso/fixture/` nếu cần dùng pgtest/migration binary. Thêm `@playwright/test` devDependency và script `test:sso` tại root `package.json`, cập nhật `package-lock.json` bằng package manager. Không thêm framework thứ hai cho unit tests.

**Tệp docs/env cần cập nhật chính xác:**
- `README.md` (ranh giới shared identity hiện tại `:9-11`, diagram cây platform ở `:20`), `apps/noverse/README.md`, `apps/viderse/README.md`.
- `apps/noverse/deploy/{README.md,.env.example,docker-compose.dokploy.yml,dsn-entrypoint.sh}`; mới `sso-cutover.md` từ phase 04. Không nhét SSO vào tài liệu TTS worker như một identity provider khác.
- Mới `apps/viderse/deploy/{README.md,.env.example}`: data DSN + identity DSN, provision role, origin/cookie, setup/password/grants CLI, startup failure và rollback.
- `apps/noverse/docs/design/v1/09-auth-onboard.md`: sửa cookie/file-vs-PG, setup và password policy đã lỗi thời (`:10-19,27-35`); `11-api-contract.md`: thêm grants/view, error codes/neutral routes và 503 (`:8-10`), grant endpoint và password reauthenticate.
- `apps/noverse/cmd/ainovel-cli/main.go`, `serve.go`, `storage.go`; `apps/viderse/cmd/viderse/main.go`. Noverse main hiện chỉ có dispatch/init (`main.go:48-70`), usage serve ở `serve.go:34-47`; phải thêm help/env đúng luồng và dẫn tới usage sở hữu, không giả có env list cũ. Viderse env list thực tế tại `main.go:31-35`; thêm extra DSN/origins/domain và CLI grant, không lẫn provider login.
- Generated dist Noverse/Viderse theo build script hiện hữu; không chỉnh tay bundle/hash asset.

Tệp đã sửa bởi phase trước chỉ được phase 05 nhận sau handoff, không có owner song song. Không sửa các plan lịch sử để xoá dấu vết SSO từng deferred; link kế hoạch này từ docs đang dùng là đủ.

## Fixture và luồng test thật

1. Một DB identity thử áp migration Noverse, một DB data Viderse thử áp migration Viderse. Fixture dùng migration/DBA role **chỉ chuẩn bị**, sau đó server runtime dùng noverse_app, viderse_identity và viderse_app; worker DSN riêng. Có thể dùng cluster thử chung với schema/role suffix random, nhưng không được cấp quyền migrate cho role runtime để test qua.
2. HOME/root/library/blob/report/backup đều ở temporary directory; guard cấm `verse-live`, user HOME thật và DSN production. Dùng fixture v4 synthetic + command phase 04, không đọc account file thật. Test dùng cluster disposable hoặc schema random do harness tạo và chỉ cleanup object mình tạo; không DROP/reset DB tùy ý từ env.
3. Build web rồi build hai Go binary. Chọn hai cổng loopback còn trống; mỗi app own PUBLIC_URL, cùng hai AppOrigins, CookieDomain rỗng. Listener readiness + health + me phải pass, không chỉ process còn sống. Fail startup phải dump log đã redact và teardown process/pools.
4. Playwright dùng **một BrowserContext, hai Page** cho hai app, APIRequest context cùng cookie jar để kiểm status không cần mock. Không dùng hai context rồi copy cookie thủ công. Context khác chỉ để kiểm nhiều thiết bị/revoke-all hoặc actor admin riêng.
5. User fixture: admin active global; member cả hai grant; member Noverse-only; member Viderse-only; member temp password; locked/deleted. Quyền/project Noverse có ít nhất một owner và story thật, Viderse không có project partition. Mọi test tự seed qua migration/command/API thật, không test-only auth bypass.
6. Hosted fixture: HTTPS hai subdomain cùng test parent domain bằng resolver/proxy thử, cert tin cậy trong browser; CookieDomain parent. Không dùng `localhost` như public-suffix parent giả. Kiểm Strict navigation thực tế; local host-only pass không thay thế hosted test.

## Bảng smoke bắt buộc (tên test đề xuất)

| Test | Thao tác | Kết quả quan sát |
|---|---|---|
| `SSOLocalCookiePortScope` | Login form Noverse rồi mở Viderse page trong cùng context | Noverse/Viderse me + một GET nghiệp vụ đều 200; jar một verse_session host-only Path=/, không cookie theo port; không form login lần hai |
| `SSOReverseLogin` | Context sạch, login Viderse rồi mở Noverse | Hai app 200, cùng user/grants; project Noverse vẫn theo membership, Viderse không query project |
| `SSOLocalHostBoundary` | Login 127.0.0.1, mở localhost/IPv6 host khác | Không được coi là cùng cookie; 401/normal login, docs giải thích host khác; không đổi sang Domain=127.0.0.1 |
| `SSOHostedCookieNavigation` | Login app A HTTPS, navigate app B cùng site | Domain parent đúng, Strict/HttpOnly/Secure, không login lại; khác parent không nhận cookie; proxy không spoof Secure |
| `SSONoverseOnlyMember` | Login Noverse-only rồi mở Viderse | Viderse me và business 403 app_access_denied tiếng Việt có user.grants; cookie còn; Noverse 200; không SSE/login loop |
| `SSOViderseOnlyMember` | Chiều ngược | Noverse 403 đúng giải thích; Viderse 200; project list không bị dùng thay grant |
| `SSOGlobalAdminAndGrantMutation` | Admin không grant row; cấp/revoke quyền target từ Noverse UI và từ hai CLI | Admin 200 cả hai; chỉ global admin HTTP sửa được; target mọi session bị revoke khi quyền đổi; login lại thấy 200/403 mới; audit đầy đủ, project membership không đổi |
| `SSORevocationAcrossBothApps` | Revoke shared current session từ Noverse, rồi làm lại từ Viderse | Old token me/business 401 ở **cả hai** request kế tiếp; không per-app cache; cookie clear đúng scope |
| `SSOLogoutEverywhere` | Logout một session; tạo hai device contexts và logout-all/revoke-all từ mỗi app | Logout current làm cùng context cả hai 401; logout-all/revoke-all mọi contexts cả hai 401, không giữ current |
| `SSOPasswordChangeRevokesAll` | Đổi mật khẩu từ Noverse và Viderse ở từng lượt | 200 reauthenticate + cookie clear, mọi old token/session cả hai 401, password mới login tạo session ID/secret mới; password cũ generic fail |
| `SSOAdminResetAndDisable` | Admin reset/disable target tại Noverse | Mọi session target cả hai 401 request kế tiếp; re-enable không hồi sinh phiên đã revoke |
| `SSOForcedPasswordBothApps` | Login bằng temp còn hạn từ mỗi app, thử upload/write/SSE/route trực tiếp | Me có flag, business 403 password_change_required, không skip; đổi được dù temp đã consumed; sau đổi phải login mới, flag false hai app, no stale session |
| `SSOConcurrentSetupOneAdmin` | DB/schema trống riêng, hai setup token, race hai POST | Đúng một admin/user/session; loser409, refresh needed=false; không double recovery; DB down503 không giả empty |
| `SSOCSRFNonAllowlistedOrigin` | Origin là same-host port thứ ba, foreign/null/lookalike; có cookie hợp lệ | POST/PUT/PATCH/DELETE 403 trước handler, DB/audit không đổi; allowlisted sibling qua CSRF nhưng vẫn chịu grant/admin gate |
| `SSOCRSFRealBrowserForm` | Trang cổng thứ ba gửi form POST qua browser | Handler đích không chạy/DB không đổi; không bật wildcard CORS. APIRequest có Origin explicit kiểm status 403 bổ sung, không thay thế browser attempt |
| `SSOReturnTargetBoundary` | Tamper local next route và link/app URL | Không điều hướng tới external/scheme-relative/javascript; không gửi session token trong URL; điều hướng nội bộ vẫn dùng hash router |
| `SSOAlternatingLoginRateLimit` | Xen kẽ sai credential giữa hai app, cùng username/IP, rồi thử đúng | Chung budget/backoff/Retry-After, không được nhân đôi số lượt; restart app không reset DB budget; unknown-user cùng generic behavior |
| `SSORoleAndWorkerBoundary` | Query bằng role Viderse/worker, capture child env, chạy một facts operation | Viderse account flows pass nhưng SELECT projects/stories/audit/memberships và UPDATE global_role deny; worker facts/lease pass nhưng account reads deny, không identity/app/migrate DSN |
| `SSOImportedAccountsAndRollback` | Import hai file v4, collision preflight, resume, restore rehearsal | Backup byte-equal, fail conflict không partial identity, TTL/used/tombstone/grants đúng, old sessions invalid; Noverse file facts/blob đủ; restore trước ghi và freeze-after-write đúng runbook |

Grant mutation làm target cần login lại là chủ ý rotate-on-privilege-change, không mâu thuẫn với no-grant 403: **phiên còn hợp lệ** nhưng không có grant →403; phiên đã bị revoke bởi mutation →401; login mới thiếu grant →403.

Không đưa raw Cookie/Set-Cookie/storageState/DSN/hash/password vào evidence. Auth network trace có thể chứa secrets: không giữ trace nguyên bản; tắt capture auth body/header hoặc redact trước khi lưu. Screenshot chỉ trạng thái UI không credential. Cleanup browser context/profile tạm sau test.

## Gate kiểm thử hợp nhất

Main chỉ chạy sau mọi phase đã land; khi fail phải sửa đúng implementation, không nới SQL role, tắt origin guard hay đổi expected thành 200 cho qua.

```sh
npm ci
npm run typecheck -w apps/noverse/web
npm run typecheck -w apps/viderse/web
npm test -w apps/noverse/web
npm test -w apps/viderse/web
npm run build -w apps/noverse/web
npm run build -w apps/viderse/web
REQUIRE_POSTGRES_TEST=1 go test ./...
go build ./apps/noverse/cmd/ainovel-cli
go build ./apps/viderse/cmd/viderse
npm run test:sso
```

TEST_DATABASE_URL/fixture runtime DSNs được runner cấu hình vào cluster thử, không lộ trên command line. `test:sso` là script mới phải tạo, không phải command sẵn có. Test fixture hai app không chạy engine/provider trả phí. Chạy SQL generator cho config Noverse và platform rồi xác nhận output dùng đúng source; migration test phải thật sự chạy, không SKIP do thiếu Postgres.

## Checklist docs/env và backwards compatibility

| Đối tượng | Nội dung bắt buộc |
|---|---|
| Noverse | DATABASE_URL runtime noverse_app, MIGRATION_DATABASE_URL chỉ migrate, STORE_DRIVER=postgres cho SSO; file standalone cookie riêng không SSO; NOVERSE_WORKER_DATABASE_URL mới, worker không nhận auth secret |
| Viderse | VIDERSE_DATABASE_URL=data, VIDERSE_MIGRATION_DATABASE_URL=data migration; VERSE_IDENTITY_DATABASE_URL=viderse_identity riêng; không file fallback; provider auth không phải web SSO |
| Cả hai | PUBLIC_URL riêng, VERSE_COOKIE_DOMAIN hosted parent/rỗng local, VERSE_APP_ORIGINS exact list, TRUSTED_PROXIES, SECURE_COOKIES; SameSite Strict và lý do, không __Host-; bind và cảnh báo cookie bỏ qua cổng |
| API/UI | me user.grants; 401 vs app_access_denied403 vs password_change_required403 vs backend503; logout/password/revoke-all ảnh hưởng cả hai; grant mutation yêu cầu login lại; tiếng Việt, no login loop |
| Quản trị | Noverse màn Thành viên + endpoints global admin; CLI grant/revoke trong cả binary theo trusted local operator, --app bắt buộc; không Viderse HTTP user mutations |
| Di trú | Collision fail/report, rename explicit, Viderse admin approval, `.v4.bak`/dump/checksum, v4 hashes/TTL/tombstone, full Noverse file facts/blob, invalidate old cookie/session |
| Rollback | Expand-only schema, không goose down DB đang dùng, pre-write restore rehearsal, hậu-cutover freeze/dump/diff trước quay file; không hứa reverse export chưa kiểm |

Hosted invariant đã khoá: `apps/noverse/deploy/docker-compose.dokploy.yml:65` bind app0.0.0.0:8080 trong container; app chỉ expose8080 nội bộ (`:187-193`), không host publish HTTP app. Ports app tại `:194-198` chỉ là private TTS worker8081, không nhầm với HTTP UI. Traefik publish HTTP/TLS (`:201-215`). Giữ private app network + public ingress duy nhất qua Traefik, cookie parent Domain+Secure và trusted-proxy config; không áp loopback-only local lên hosted. Thêm `TestHostedAppHTTPPortIsNotPublished` kiểm resolved compose cho cả hai app.

## Review bảo mật độc lập bắt buộc trước commit

Sau test hợp nhất, giao **toàn diff 01–05** và threat table ở plan.md cho reviewer độc lập vai trò `security-reviewer` qua capability runtime có sẵn khi triển khai. Reviewer không là người implement. Nếu runtime không có agent tên đó, dùng reviewer người thật/vai trò tương đương, không tự chứng nhận.

Reviewer phải trace cookie/token generation, SQL privileges từng column/sequence, grant actor transaction, setup concurrency, CSRF canonicalization, forced-password neutral bypass, worker env, migration collision và raw-secret logs. Đối chiếu mỗi threat với test có tên và kết quả thực chạy. Sửa mọi blocker/major, chạy lại checks ảnh hưởng và **review lại đến 0 blocker/major trước commit**; minor còn lại chỉ chấp nhận nếu không làm sai tiêu chí đã chốt. Lưu findings/resolution và evidence đã redact trong reports của implementation plan. Không commit secret fixture hoặc dump, không coi test pass thay review.

## Thành công, rủi ro và rollback

**Done:** mọi ô smoke pass trên runtime roles thật; unit/PG/archtest/UI pass không SKIP integration; docs/usage khớp code; generated UI build mới; migration restore rehearsal pass; security reviewer 0 blocker/major. Report chỉ ghi lệnh/kết quả đã chạy, phân biệt local/hosted. Không ghi pass hosted nếu chỉ kiểm local.

| Rủi ro | Xác suất × tác động | Giảm thiểu |
|---|---|---|
| Test mock hoặc migrate role che lỗi runtime | Cao × Cao | Hai binary/browser thật, runtime roles thật, negative SQL bắt buộc |
| Build nhúng UI cũ | Vừa × Cao | Build web trước Go, kiểm hash asset và text denied/password trong binary smoke |
| Test chạm HOME/DB live | Thấp × Rất cao | Fixture temp/schema ownership, deny forbidden paths, không default DSN, không root DB reset |
| Trace/evidence lộ session | Vừa × Cao | Redact/tắt auth network traces, không storageState artifact, secret-scanning evidence trước bàn giao |
| Hosted proxy/domain chưa khả dụng | Vừa × Cao | Diễn tập HTTPS parent-domain riêng; kiểm private app HTTP/no host publish và trusted proxy, live domain là owner gate; không giả test pass |

Rollback phase: có thể revert runner/docs độc lập nếu code runtime chưa triển khai; không dùng revert tests để bỏ gate. Nếu nghiệm thu phát hiện bug, dừng cutover, sửa phase owner và review lại. Nếu đã mở ghi thực tế, áp dụng quy trình đóng băng/backup/diff ở phase 04, không test revert trên dữ liệu live.
