# Platforms (Settings › Tài khoản nền tảng + Bilibili QR)

## What changed

New packages, all dependency-injected (no globals, no `init`), composition only in `cmd/viderse`:

- `internal/domain/platforms.go` — shared value vocabulary: `PlatformID/Use/Kind/LoginType/Condition`, `PlatformState`, `PlatformLastError`, `PlatformLogin`, `PlatformUsage`, `PlatformStatus`, `BrowserProfile`, `QRSession`, `QRPoll`, `RefreshOutcome`.
- `internal/platforms/` (consumer-side interfaces only):
  - `registry.go` — youtube (360, browser_cookie, competitor/video), twitch (480, none, backdrop/clip+highlight+vod), bilibili (1080, qr, backdrop/video). `Registry()` returns copies; `Lookup()`.
  - `state.go` — `StateStore` seam, `PausedError`, `RateLimited`/`CodedError` classification, `PauseMinutes` (15·2^(n−1), cap 120).
  - `service.go` — `Status` (one state query + one usage call, no N+1), `conditionOf`, `Attention` (login_expired → `#/settings?section=accounts`), `RecordBlocked/RecordSuccess/ClearPause/RecordError/ClearError`.
  - `pacing.go` — port of `pacedRequest`: Bilibili 5 s + jitter slot reservation (one atomic UPSERT), refuse while paused, translated `PausedError`, backoff reset only on `resetBackoff`, rate limit → escalating pause, `WithScanner` cross-process lease.
  - `login.go` — registry dispatch for PUT/DELETE login, profiles, QR, refresh, check; rejected YouTube login records `cookies_failed`, successful Bilibili login/refresh/QR clears pause.
- `internal/accounts/`:
  - `accounts.go` — `Service` over injected `*http.Client`, settings, probe, clock; **one login lock** for login/refresh/check (in-process mutex + `flock` on `~/.viderse/bilibili-cookie.txt.lock`), `~/.viderse` files mode 0600, `HOME` respected.
  - `bilibili_cookie.go` — port of `bilibili-cookies.ts`: `NormalizeBilibiliCookie`, nav verification, atomic snapshot write + cookie mirror, snapshot-wins-over-mirror read, masked SESSDATA, Netscape jar for `*.bilibili.com` only, `Set-Cookie` gộp với URL.
  - `bilibili_qr.go` — port of `bilibili-qr-login.ts` + `bilibili-login.ts`: buvid3/4, 180 s session, poll codes 86101/86090/86038/0/other, RSA-OAEP-SHA256 correspond path, refresh chain `cookie/info → correspond → cookie/refresh → confirm/refresh → nav`, 24 h gate, `refreshError` on failure keeping old credentials.
  - `youtube.go` — port of `cookies.ts` + verification: read-only `Local State` profile listing (5 Chromium browsers, natural sort, 60 s cache + `?refresh=1`), `SetCookieSource` verifies via `YouTubeProbe` before saving, `Check`, `CookieArgs`.
  - `filelock.go` — shared lock, `ReasonBusy` when held.
- `internal/persistence/migrations/00004_platform_state.sql` — `platform_state` (next_request_at, paused_until, pause_reason, failure_count, scanner lease, `last_error_code/at/job_id`).
- `internal/persistence/postgres/queries/platforms.sql` + `postgres/platforms.go` — atomic UPSERTs; escalation computed in SQL.
- `internal/web/platforms.go` — thin handlers, wire DTOs separate from domain.
- `cmd/viderse/platforms_wiring.go` — builds tools/accounts/platforms, mounts routes, adapts `Attention` to `status.Source`, publishes `platforms` SSE **and** bumps status.
- `serve.go` — one wiring block.

## Wire API

`GET /api/platforms` → array:
`{id,label,uses[],kinds[],targetHeight,login{type,saved,masked?,account?,browserProfile?,method:("qr"|"manual"|null),renewable,lastRefreshAt?,refreshError?},state("ready"|"paused"|"needs_login"),pausedUntil?,pauseReason?,failureCount?,lastError?{code:"cookies_failed",at,jobId?},usage{channels,videos,rivals?}}`; timestamps RFC3339; usage counts real zeros (via `UsageCounter`, default zero source).

- `GET /api/platforms/youtube/login/profiles[?refresh=1]` → `[{browser,profile,name,email?}]`
- `PUT /api/platforms/:id/login` — `{cookie}` (bilibili, verified, bilibili.tv rejected) or `{browser,profile}` (youtube, yt-dlp verified)
- `DELETE /api/platforms/:id/login` → login wire
- `POST /api/platforms/:id/login/check` → login wire ("Thử lại", clears lastError/refreshError on success)
- `POST /api/platforms/bilibili/login/qr` → `{qrcodeKey,qrImage,loginUrl,expiresAt}`
- `GET /api/platforms/bilibili/login/qr/:key` → `{status:"waiting"|"scanned"|"expired"|"success"|"rejected",account?}`
- `POST /api/platforms/bilibili/login/refresh` → login wire + `status:"refreshed"|"current"|"skipped"|"failed"`

Errors: 404 unknown, 400 `invalid`/`unsupported`, 422 `rejected`, 409 `busy`, 502 `unavailable`. SSE `platforms` with `{rev,ref}`; event keys already present in `data/keys.ts`/`eventKeys.ts` (added by SettingsWebB).

## Kiến trúc & hiệu năng

Ranh giới gói: `accounts` không biết `platforms`; `platforms` biết `accounts` qua interface tự khai (LoginInfo/LoginActions). Chỉ `cmd/viderse` biết cả hai. `platforms` không import `internal/status`: `Attention` trả `platforms.AttentionItem`, adapter `platformAttention` ở `cmd` đổi sang `status.Report` (và `Bump` để chấm Cài đặt đổi cùng nhịp SSE). Mọi In-ra đều tiêm: `*http.Client`, đồng hồ, sleep, random, store, probe, notify.

- `GET /api/platforms`: đúng 1 truy vấn `platform_state` (`ListPlatformStates`) + 1 lượt usage + 3 lượt đọc đăng nhập (YouTube: settings; Bilibili: stat tệp + cache theo mtime). Không HTTP, không yt-dlp, không quét thư mục. Giữ bởi test `TestStatusDoesOneStateQueryAndNoNetworkOrProfileScan` (20 lượt: All=20, profiles/check/refresh/qr=0).
- Hồ sơ trình duyệt: cache 60 giây, quét lại chỉ khi `?refresh=1` (test `TestProfilesAreCachedForAMinute`).
- QR/refresh: chỉ gọi Bilibili khi client hỏi; phiên hết hạn/khác key trả `expired` cục bộ (đo 0,7–1,1 ms/lượt). Mọi lời gọi Bilibili đi qua `request(ctx, …)` nên chết theo ctx của request; khoá đăng nhập là mutex + flock (tự nhả khi process chết). Không có vòng lặp nền.
- Đo thật (server 18793, Postgres thật, 200 request tuần tự sau 5 lượt khởi động): **min 0,40 ms · p50 0,55 ms · p95 1,09 ms · p99 2,25 ms · max 4,36 ms** (mean 0,66 ms).

## Verification

- `go build ./apps/viderse/...` — pass (toàn app).
- `go test -p 2 -count=1` on platforms, accounts, web, persistence/postgres (with `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`) — pass.
- `go test -race -p 1 -count=1` same four packages — pass.
- `sqlc generate` (under `/tmp/viderse-sqlc.lock`) run after query rewrite; postgres package builds against generated names.
- gofmt with go1.26.0 **and go1.27.0** on every changed file (`gofmt -l` empty).

### Smoke thật (port 18793, DB `viderse_smoke_platforms`, HOME tạm)

- `createdb -O viderse_migrate` + `migrate up` → 00001…00004 áp dụng sạch; `serve -addr 127.0.0.1:18793`, thiết lập admin qua mã thiết lập.
- `GET /api/platforms` → đúng 3 nền tảng; youtube ready/browser_cookie/method null, twitch ready/none, bilibili ready/qr; usage 0 thật (`rivals` vắng).
- QR thật: start → key + `loginUrl` account.bilibili.com + `expiresAt` +180 s + ảnh SVG data URI; 3 lượt poll đều `{"status":"waiting"}` (86101). Tổng 5 request Bilibili (spi+generate+3 poll).
- Huỷ: client dừng poll; 5 lượt poll key lạ trả `{"status":"expired"}` trong 0,65–1,06 ms (không ra mạng). Phiên QR `bilibili-qr-session.json` 0600.
- `GET /api/platforms/youtube/login/profiles` với HOME tạm → `[]`; quét thật chỉ-đọc trên home người dùng (chương trình tạm trong module, đã xoá) → `chrome Default/Profile 1/Profile 2`, 3 hồ sơ, không ghi gì.
- Dọn: kill server (cổng về 0 listener), `dropdb viderse_smoke_platforms`, xoá `/tmp/viderse-platforms`.

## Provenance nguồn port

Các tệp tham chiếu Bilibili QR/login/cookie không tồn tại trong commit `2c201e6` của yt-factory — chúng nằm ở working tree chưa được commit (`git status`: `??`). Port dựa trên nội dung đọc được; SHA-256 ghi lại để đối chiếu sau này:

- `src/server/backdrop/bilibili-qr-login.ts` — `20d84f9da31ac6948cea8554805eac4f557507ae30e0a0185f917ea6ba04b52f`
- `src/server/backdrop/bilibili-qr-login-storage.ts` — `4c9650db083d4a8c7a15b4bc764e16f679ad8b1d31fc9b18ac594528c7ac6032`
- `src/server/backdrop/bilibili-login.ts` — `938ec9b4bec19db825a122cd3341a60baf835526f2eb6d0d064dc8ff19e23784`
- `src/server/tools/bilibili-cookies.ts` — `c5e56acecb78d25f45efcca87eca18ea014f27ee50476807cf15dd71538403e3`
- `src/server/tools/cookies.ts` — `8e8585d8903f387922ea92facf5aa901ca1faa93f1b846c19e1bb5f768495273`
- `src/server/backdrop/pacing.ts` — `a3a901f88d9010432ca2a605cf2834b1a31bc755472bca34dd9526fb19bc8e0b` (đã sửa trong working tree so với commit; `git status` `M`)

Các tệp test đối ứng (`bilibili-cookies.test.ts`, `bilibili-qr-login.test.ts`, `bilibili-login.test.ts`, `pacing.test.ts`) cũng untracked cùng trạng thái.

## Review fixes (Review-p2-secrets.md)

- **H4** — snapshot đăng nhập Bilibili giờ công bố bất biến qua `cacheMu` (RWMutex): `readLoginState` trả `clone()` sâu, `writeLoginState`/`clearBilibiliLocked` publish clone; `bilibiliLogin`/`CookieHeader`/`Login` đọc dưới cacheMu mà không giữ `mu`. GET không chặn bởi chuỗi refresh mạng. Test: `TestConcurrentLoginReadsRaceFree` (4 reader × save/check/clear, -race) + `TestBusyGateDoesNotQueue` phần GET.
- **M6** — `withLock` đổi sang `s.mu.TryLock()` → `errBusy` (409) ngay, không xếp hàng sau network; `SetCookieSource`/`Check`/`ClearLogin` (YouTube) cùng cơ chế. `EnsureFresh`/`CookieHeader` không giữ `mu` nên trả cookie hiện có khi thao tác khác giữ khoá. Test: `TestBusyGateDoesNotQueue` (network blocked → refresh thứ hai busy <50 ms, EnsureFresh vẫn trả cookie), `TestEnsureFreshReturnsCookieWhileLocked`.
- **M7** — `Status` chuẩn hoá giờ chờ hết hạn trong read model theo đồng hồ tiêm (`st.PausedUntil.After(now)`), không ghi ngược; `failureCount` giữ nguyên. Test: `TestStatusNormalizesExpiredPause` (trước/tại/sau hạn + refreshError → needs_login ngay).
- **M8** — `web/platforms.go`: mọi lỗi không thuộc tập 404/400/409/422/502 trả vỏ trung lập `"Có lỗi nội bộ xảy ra…"`; nguyên nhân đi qua `logs.Store.Redact` (che Bearer/cookie/token/JWT/path home) vào `logs/platforms.log` qua `MountPlatformsDiag`. Test: `TestInternalErrorsReturnNeutralEnvelope` (sentinel path/DSN/token không lộ trong JSON, diag nhận đủ), `TestRedactedDiagHidesSecrets` (redactor che sentinel + home → `~`).
- **L2a** — `mountPlatforms` nhận `toolKit.YtDlp` dùng chung của `serve.go` (cùng runner/registry/quyền cài đặt), bỏ `tools.New` thứ hai; probe qua interface `ytdlpRunner` tiêm ở cmd. Test: `TestProbeUsesSuppliedRunner` + `TestProbeMapsOutcomes` (cmd package).
- **L2b** — `Profiles` cache cả danh sách rỗng (`profilesOK`), refresh qua `?refresh=1`. Test: `TestEmptyProfileListIsCachedToo`.
- **Provenance** — các tệp QR/login/cookie của yt-factory là untracked (không có trong `2c201e6`); SHA-256 của nội dung đã port ghi trong mục "Provenance nguồn port" ở trên.

Verify: `go test -p 2` + `-race -p 1` xanh trên accounts/platforms/web/persistence-postgres; `go tool sqlc diff` sạch; `gofmt -l` rỗng dưới go1.26.0 và go1.27.0; `go build ./apps/viderse/...` xanh.

## Deviations / open issues

- Registry uses `login.type = "qr"` for Bilibili (§24.1 wins over the task's `cookie_string`; manual paste still accepted).
- `platform_state` also carries the scanner lease and slot columns ported from `backdrop_platform_state` (needed by pacing/scan).
- Pacing applies escalating pauses only to Bilibili (faithful to `pacing.ts`); the store/API is generic.
- No server-side QR cancel endpoint: the server only calls Bilibili when the client polls; a locally expired session returns `expired` with zero network requests (verified by test).
- YouTube probe treats `unavailable` (video/region issue) as pass — profile cookie DB was still readable; `cookies_failed`/`login_required` reject.
- Không có endpoint huỷ phiên QR phía server: huỷ = client ngừng poll; phiên tự hết hạn cục bộ (đã đo, 0 request).
- `go build ./apps/viderse/...` xanh tại thời điểm chạy smoke; các gói khác vẫn đang được đồng nghiệp hoàn thiện nên build toàn app có thể đổi sau đó.
