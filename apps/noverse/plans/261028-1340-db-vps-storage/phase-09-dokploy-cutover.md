---
phase: 9
title: "Docker đa tầng, Dokploy/Traefik, health, proxy và cutover"
status: in-progress
priority: P1
effort: 16h
dependencies: [8]
---

# Phase 09 — VPS Dokploy có bảo mật và phục hồi kiểm chứng được

## Context / yêu cầu

Sau08 restore/reconcile gate và tất cả domain gates. [Dokploy research](./research/platform-storage.md), [plan](./plan.md), backup/runbook08. Dockerfile hiện chỉ Go builder+Alpine/CA/tzdata (`Dockerfile:1-30`), **không build web hoặc cài ffmpeg**; compose chỉ interactive CLI/home/workspace mounts (`docker-compose.yml:1-10`). Vite build output vào Go embed dist (`web/vite.config.ts:24-25`); `web/package.json:5-13` có build/test nhưng **chưa có Playwright/E2E script**. Phải bổ sung thật, không viện dẫn test runner chưa tồn tại.

Bắt buộc: multi-stage web/Go/runtime ffmpeg+ffprobe; env config; `/healthz`; trusted X-Forwarded-*; Secure cookie; login limiter04; Postgres nằm trong Dokploy; BlobStore local durable volume hoặc private S3; Traefik sẵn, **không Caddy**; Mac TTS private pull; backup/restore/rollback runbook. Chỉ một web replica mặc định để giữ in-process setup/event hub compatibility; bền vững jobs/fences vẫn cần cho restart/overlap rolling deploy.

## Architecture / deployment data flow

Internet HTTPS :443 → Dokploy Traefik → app :8080 (container private only) → internal Dokploy Postgres hostname, private S3 or durable local volume. App serves embedded SPA/API; readiness checks DB ping/schema and local volume writability where local selected, not LLM/GPU readiness. Public `/healthz` no auth/no secret, 200 when ready, 503 DB/schema/storage-local unavailable; remote S3 bucket validation at startup with timeout + bounded cached check, do not upload a probe per request. TTS Mac outage is queue degradation, **not** app unhealthy/restart loop.

VPS host Tailscale IP publishes private worker listener :8081 only; public mux never registers worker handlers. Dokploy/host firewall+ACL allow only Mac worker, no Postgres public port. Migration one-shot runs goose with migration credentials before app readiness; app role has no DDL. Local blob volume `/var/lib/noverse/blobs`, ephemeral scratch `/var/cache/noverse` with quota/tmpfs choice; nonroot UID owns only these. No mount original home library required.

## Files / ownership

`R=/Users/dat/Projects/ainovel-cli`.

- Modify `R/Dockerfile:1-30`, `R/docker-compose.yml:1-10` (offline example vs hosted recipe), `R/.dockerignore:1-16`, `R/cmd/ainovel-cli/serve.go:24-65,122-149`, `R/internal/bootstrap/storage.go` (02), `R/internal/entry/web/{gateway,auth,authorization}.go` (`gateway.go:478-506`, `auth.go:54-72`), `R/web/package.json:5-13`, `R/web/package-lock.json`, `R/README.md`, `R/docs/architecture.md`.
- Create `R/internal/entry/web/proxy.go`, `health.go`, `proxy_test.go`, `health_test.go`; `R/deploy/dokploy.compose.yml`, `R/deploy/.env.example`, `R/docs/dokploy.md`; `R/web/playwright.config.ts`, `R/web/e2e/storage.spec.ts`, `accounts.spec.ts`, `audio-worker.spec.ts`; extend `R/deploy/compose.test.yml` from03 for E2E real app/service tests.
- Go embed owner `R/internal/entry/web/gateway.go:34` is `//go:embed all:dist`; Vite target `R/web/vite.config.ts:24-25` matches it. Docker web output must be copied there before Go build. No source code/dist changes in this planning assignment, no Caddyfile/second reverse proxy.

## Env contract

- Facts: `STORE_DRIVER=postgres`, `DATABASE_URL`, `MIGRATION_DATABASE_URL`, `DB_MAX_CONNS`, `DB_CONNECT_TIMEOUT`; CLI defaults file absent explicit hosted config.
- Blob: all **8** requested env keys from03 plus `BLOB_LOCAL_ROOT`; credentials secrets in Dokploy Environment, not image or `.env.example` values.
- Server: `HTTP_ADDR=0.0.0.0:8080`, `PUBLIC_URL=https://<domain>`, `TRUSTED_PROXY_CIDRS=<actual Traefik source subnet/IP>`, `SECURE_COOKIES=true` required hosted, header/body/server timeout config. Reject wildcard0.0.0.0/0 proxy trust by default.
- Auth: TTL/limiter/hash concurrency env bounded; reuse Argon2 defaults, do not add insecure plaintext bootstrap admin default. Setup code printed once only if no users and terminal/Dokploy log access is secured; first-admin transaction04 remains authoritative.
- Worker: `WORKER_HTTP_ADDR=0.0.0.0:8081` private mux, `TAILSCALE_IP` host publish binding, Mac `TTS_WORKER_API_URL` and `TTS_WORKER_TOKEN`; one-time admin credential provisioning, no bucket/DB secrets Mac.
- App config secrets stored encrypted require `CONFIG_ENCRYPTION_KEY` from Dokploy secret; backup key separately encrypted, restoring DB without key must fail clearly, not overwrite settings. Env runtime policy overrides DB defaults only documented ops keys; creative config inheritance preserved.

## Steps

1. Write fail-before proxy/health/packaging tests. Current `secureRequest` trusts any X-Forwarded-Proto (`auth.go:63-64`), `clientIP` ignores proxy (`:54-59`), static routing eats non-/api paths (`gateway.go:482-488`). Tests show spoof and SPA `/healthz` false positive now before implementation.
2. Implement trusted proxy normalization before request routing: only trust forwarded headers if immediate RemoteAddr in configured CIDRs; derive real client IP by walk XFF right-to-left through known proxies, discard malformed/multiple proto/host, validate scheme/host against PUBLIC_URL allowed origin; untrusted headers ignored. Prefer configured PUBLIC_URL for absolute pairing/callback links, not arbitrary Host/XFH. Use normalized values for cookie/origin/rate limit/audit; do not mutate process-global state.
3. Hosted cookie Secure always true by config and fail startup when PUBLIC_URL not HTTPS; retain HttpOnly/SameSiteStrict/path and correct expiry/logout cookie. Direct local CLI HTTP remains available only file/local mode. Anti-CSRF checks POST/PUT/PATCH/DELETE and narrow multipart exemptions against actual allowed origin/JSON contract; preserve existing account behavior. Traefik-only public networking prevents bypass through published8080. Test forged forwarding cannot evade limiter or claim HTTPS from direct HTTP.
4. Add `/healthz` handling before static guard, GET/HEAD only, JSON minimal status and no DSN/version secrets. Validate schema compatibility/DB connection/local blob root; use deadlines to return503 quickly, not hang. Graceful shutdown stops claims, cancels workers/streams, releases leases, drains HTTP, closes pool; in-flight blobs stay recoverable or orphan for GC, no false success.
5. Dockerfile web stage pinned compatible Node (`npm ci && npm run build`), Go stage1.25 builds `./cmd/ainovel-cli` after web output copied to exact embed dist, runtime nonroot Alpine/Debian with CA/tzdata/ffmpeg/ffprobe. Decide base using actual media/font dependencies, include embedded fonts/assets/build metadata; do not copy home/.env/keys/test caches. Entrypoint `ainovel-cli serve` binds configured0.0.0.0; CLI args can override documented env but not bypass hosted Secure policy. Image healthcheck uses binary HTTP check or runtime wget installed intentionally, no absent curl.
6. Dokploy: create Postgres17 service inside platform, app role/migration role and internal DSNs; don't expose5432. Configure app Dockerfile/domain/HTTPS Traefik, service port8080, health/restart limits, one replica, migration one-shot. Mount local blob durable volume only when selected; S3 private bucket config optional instead, ffmpeg scratch quota. Deploy example compose must not create a second proxy or accidentally attach standalone Postgres outside Dokploy. Set Tailscale host publishing8081 firewall/ACL and verify private/public reachability from actual network, not only container localhost.
7. Configure nightly off-VPS Dokploy Postgres backup + ready-blob manifest and immutable object retention/verified copy; local blob volume backup separately, not assume database backup covers it. Do restore drill using08 procedures and include encryption key recovery, record measured RTO/RPO and checksum totals. Keep prior image/DB expand-only schema/old source snapshot through rollback window7d proposed configurable.
8. Add Playwright dev dependency/script/config (not present now); tests target built real Go app+Postgres+local/MinIO, never `dev:mock`. Browser account flow: setup/admin/create member/temp/must-change/project grants/view+collab+owner enforcement/revoke device; story flow create/edit/concurrent conflict/comment/review/search; audio worker flow queue→real Mac→play/range/export→restart download. Automate deterministic job/lease cases with controlled actual HTTP worker; mark real VieNeu cross-machine scenario separate required manual/real endpoint gate, not fake passed E2E.
9. Final cutover: announce maintenance/logout; stop file CLI/TUI/web writers and Mac claims → immutable final snapshot → dry-run/import/reconcile/restore proof08 → deploy schema+app under maintenance → smoke HTTPS/auth/read/write/audio/export → enable traffic+claims once mismatch0. Observe first restart+backup/restore. No write to old file system after authority switch.
10. Rollback rehearsal both before and after new writes; retain updated DB on app rollback. When forced return file, maintenance freeze+latest reverse-export+reconcile then start offline server; revoke sessions and leases, keep history. Dokploy image rollback alone does not undo DB migration or copy blob data.

## Fail-before test matrix

| Level | Cases | Measurable gate |
|---|---|---|
| Unit | trusted/untrusted proxy, XFF chain, proto/host spoof, IPv6, cookie flags, public URL links | Normalized identity only through trusted boundary |
| HTTP integration | health before SPA, DB down/schema mismatch/local volume read-only, Mac down | 503 actual storage failure; Mac down still health200 |
| HTTP/security | restart limiter, brute-force buckets, CSRF/origin/multipart, revoked user signed vs stream TTL | Existing auth API retained, no forwarding bypass |
| Container integration | build clean without dist, nonroot, ffmpeg/ffprobe, shutdown/restart, local volume/S3 persistence | UI/API loaded from image; persisted facts/blobs survive |
| Network/Dokploy | Internet443 vs private8081/5432/8080, actual Traefik source CIDR, tailscale ACL | Only443 public; worker only authorized Mac |
| E2E | accounts/project/story/comments/audio/export plus restart and backup restore | Real application flows; artifacts/media/hash and stored state |

## Verification commands — main verifier executes once, not planning agent

```sh
go test ./internal/entry/web -run 'TestTrustedProxy|TestHealth|TestSecureCookies|TestHostedCSRF' -count=1
go test ./... -count=1
npm --prefix web ci
npm --prefix web run test
npm --prefix web run build
docker build -t noverse:db-vps .
docker compose -f deploy/compose.test.yml up -d
docker compose -f deploy/compose.test.yml exec app ffmpeg -version
docker compose -f deploy/compose.test.yml exec app ffprobe -version
npm --prefix web run test:e2e
curl --fail --silent --show-error "$PUBLIC_URL/healthz"
curl --fail --silent --show-error --cookie "$SMOKE_COOKIE_JAR" "$PUBLIC_URL/api/me"
```

`SMOKE_COOKIE_JAR` is a protected mode0600 jar from the real HTTPS login smoke; anonymous `/api/me` must401. Negative public worker-path/port checks assert expected status/connectivity rather than assume curl--fail means pass. Commands use isolated test env except explicit final PUBLIC_URL. Reconcile/copy/restore commands08 also required; record actual outputs, never claim pass from this list.

## Risks / rollback / success

- **High (L cao × I cao):** proxy spoof causes IP bypass/insecure cookies/wrong links. Trusted CIDRs+canonical URL+public-port isolation; tests before deployment.
- **High (L vừa × I cao):** image missing UI/media/dependencies or read-only paths. Clean build/container media tests, nonroot explicit writable mounts.
- **High (L vừa × I cao):** DB/volume loss or false rollback confidence. Off-VPS DB+blob+key backup, fresh restore, latest reverse-export post-write before fallback.
- **High (L vừa × I cao):** mispublished private ports/worker credential. Separate mux/private listener, tailnet binding/ACL and external-network checks.
- **Medium:** health depends on home Mac and restarts healthy app. Health excludes TTS readiness, queue status reports degradation.

Rollback per steps10 and08; stop claims/writers before any authority change, never goose down live. Done: HTTPS public URL works with Secure cookie/trusted client IP/rate limiter; health accurately reports storage; full real DB UI/story/media paths survive restart; only permitted public ports; restore proof + final reconciliation zero mismatch; no original library mount or Caddy. Main verifier signs release gate with actual evidence; this plan remains pending until executed.

## Trạng thái thực thi — 2026-10-29

**Gate cục bộ: PASS.** Gate production (Dokploy/Traefik/S3/provider thật) chưa
thể chạy vì không có VPS/credentials; danh sách còn lại ở cuối mục này.

### Đã giao

- `Dockerfile`: 3 stage `web` (node:22-alpine, `npm ci && npm run build` vào
  `internal/entry/web/dist`) → `go` (golang:1.26-alpine, CGO off, ldflags
  version/commit/date) → `alpine:3.22` có ca-certificates/tzdata/ffmpeg
  (kèm ffprobe), user `10001:10001`, chỉ 3 thư mục ghi được, healthcheck
  BusyBox `wget /healthz`. Không node/npm/curl/testdata trong runtime.
- `.dockerignore`: loại `.git`, `.env*`, log, `ainovel-cli`, `dist`,
  `**/node_modules`, `**/testdata`, workspace/refer.
- `internal/entry/web/proxy.go` + `proxy_test.go`: `TRUSTED_PROXIES`
  (alias `TRUSTED_PROXY_CIDRS`), từ chối `0.0.0.0/0`/`::/0`; XFF đi ngược
  phải→trái qua hop tin cậy; proto nhiều/không hợp lệ bị bỏ; `PUBLIC_URL` +
  `SECURE_COOKIES` (fail startup nếu không https); origin/`Sec-Fetch-Site`
  cho POST/PUT/PATCH/DELETE khi có `PUBLIC_URL`.
- `internal/entry/web/health.go` + `health_test.go`: `/healthz` (liveness,
  trước static SPA, GET/HEAD, JSON, không phụ thuộc dependency) và `/readyz`
  (schema version + DB + BlobStore, deadline riêng, cache 30s cho blob, 503
  khi lỗi, không lộ DSN).
- `auth.go`: `clientIP`/`secureRequest` đọc kết quả đã chuẩn hoá từ context, giữ
  nguyên fallback loopback cho file mode.
- `pool.go`: `SchemaVersion` (đọc `goose_db_version`, read-only);
  `bootstrap.CheckBlobStore`: local = ghi thử file trong root, s3 = `List`
  bounded.
- `serve.go`: `HTTP_ADDR`, parse proxy config sớm (lỗi = fail startup),
  gắn readiness probe; `projects.go`: bỏ chặn 409 khi hosted có
  `libRepo` (root file rỗng là hợp lệ).
- `deploy/docker-compose.dokploy.yml`: Traefik v3.3 TLS tự ký → app; Postgres
  17 + roles/DML-only; MinIO + bucket init; job `migrate` một lần
  (`storage migrate up`); worker publish `127.0.0.1:8081`;
  `deploy/postgres-init/01-roles.sh`, `deploy/traefik/dynamic.yml`.
- `docker-compose.yml`: chỉ rõ đây là sandbox CLI offline; hosted ở
  `deploy/`.
- `deploy/.env.example` (env reference), `deploy/README.md` (Dokploy guide,
  backup schedule, restore drill, cutover + rollback runbook),
  `deploy/backup.sh` (pg_dump -Fc + blob manifest + SHA256SUMS → S3, có chế
  độ drill local), `deploy/smoke.sh` (HTTPS smoke end-to-end).

### Bằng chứng đo được

- `docker build -t noverse:phase09 .` → thành công (web build 57s, Go 16s).
- Trong app container: `uid=10001(noverse)`, `ffmpeg version 6.1.2`,
  `ffprobe version 6.1.2`, không có node/npm/curl.
- Compose up: postgres/minio/app healthy; `migrate` áp dụng
  `00001…00007`; app role `CREATE TABLE` → permission denied; migrate role
  sở hữu schema.
- HTTPS (cert tự ký, xác thực bằng `--cacert`, không `-k`):
  - `GET /healthz` → `200 {"status":"ok"}`
  - `GET /readyz` → `200 {"status":"ok","schemaVersion":7,"checks":[…]ok}`
  - `GET /` → `200 text/html` (SPA)
  - `GET /api/me` anonymous → 401
  - `POST /api/login` sai mật khẩu với `Origin` cùng origin → 401;
    `Origin: https://evil.example` → 403
  - Setup: `POST /api/setup/verify` → token, `POST /api/setup/admin` → 200
    `role=admin`, `Set-Cookie: … HttpOnly; Secure; SameSite=Strict`
  - `POST /api/projects` → 201; `POST …/stories` → 201;
    `GET …/stories` + `…/meta` → đọc đúng tiêu đề.
- Restart app: giữ cookie cũ vẫn đọc được truyện (phiên + dữ liệu trong
  Postgres) → không mất dữ liệu khi rebuild/restart.
- `docker compose stop postgres`: `/healthz` 200, `/readyz` 503
  `database:false, blob:true`; bật lại → 200 `schemaVersion=7`.
- Worker: `127.0.0.1:8081/internal/worker/claim` (publish đúng địa chỉ cấu
  hình) → 401 không token; `192.168.1.10:8081` → connection refused;
  `/internal/worker/claim` qua HTTPS public → 404; `app` chỉ publish 8081
  (8080 chỉ sau Traefik).
- Backup drill (`deploy/backup.sh`, chế độ local): `noverse.dump` 148 KB,
  `blobs.tsv`, `schema.txt` (`schema.format=1`, `migration.version=7`),
  `SHA256SUMS` verify OK; `pg_restore` vào `noverse_restore` cho counts
  `users=1 projects=1 stories=1 sessions=1 schema=7` trùng production và đọc
  lại đúng `01-chuong-dau | Chương đầu | idea`.
- `deploy/smoke.sh` PASS: healthz, readyz, anonymous 401, login Secure
  cookie, tạo project/truyện, đọc lại, worker path 404.
- Unit: `go test ./internal/entry/web ./internal/bootstrap
  ./internal/persistence/postgres ./cmd/ainovel-cli -count=1 -timeout 1800s`
  → `ok` cả 4 package (web 151.3s); test mới chạy riêng cũng `ok`;
  `gofmt -l` sạch; `go vet ./...` sạch.
- `storage migrate up` chạy lần hai → "Không có migration nào chờ — schema đã
  mới nhất" (idempotent). Image cuối 227 MB. Toàn bộ container/volume/network
  và image `noverse:phase09` đã xoá sau kiểm chứng.
- Fail-before đã chứng minh: bỏ route health → `TestHealthBeforeSPA` fail
  với `content-type="text/html"`; bỏ kiểm tra trusted peer →
  `TestTrustedProxyNormalization` fail `secureRequest=true, want false`;
  trả lại guard root cũ → `TestHostedRepositorySkipsLibraryGuard` fail 409.

### Còn lại trước khi ký release gate production

1. CIDR nguồn thật của Traefik trong Dokploy cho `TRUSTED_PROXIES`; kiểm
   tra 443 công khai, 8081 chỉ từ tailnet, 5432/8080 đóng — từ mạng ngoài.
2. Domain + ACME thật; xác nhận cookie Secure/redirect HTTP→HTTPS trên
   domain thật (local chỉ chứng minh bằng cert tự ký).
3. S3 provider thật (R2/AWS/GCS): multipart, presign/streaming fallback,
   checksum; local chỉ chạy MinIO.
4. E2E audio với Mac/VieNeu thật qua Tailscale (queue → play/range → export
   → restart vẫn tải được); chưa chạy trong lượt này.
5. Backup Dokploy → S3 thật + đo RTO/RPO trên dữ liệu thật; drill local chỉ
   chứng minh script và pg_restore.
6. `CONFIG_ENCRYPTION_KEY` (plan §Env) **chưa có trong code**; cần quyết định
   triển khai hoặc bỏ khỏi contract trước production.
7. Playwright E2E (step 8 của phase) chưa thêm; acceptance của lượt này dùng
   `deploy/smoke.sh` + curl.
