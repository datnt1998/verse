# Triển khai Dokploy, sao lưu và cutover

Tài liệu vận hành cho phase 09. Áp dụng cho VPS chạy **Dokploy** (Traefik tích
hợp, Postgres tạo trong Dokploy) + **worker TTS tại nhà** qua Tailscale. Không
dùng Caddy, SQLite, Redis hay GPU trên VPS.

Mã nguồn nằm trong monorepo `datnt1998/verse`, ở thư mục `apps/noverse/`. Mọi
đường dẫn trong tài liệu này (`deploy/…`, `Dockerfile`) đều tính từ
`apps/noverse/`. Riêng build context của image là **gốc repo** (nơi có `go.mod`).

## 1. Kiến trúc và ranh giới

```
Internet ── HTTPS 443 ──► Traefik (Dokploy) ──► app:8080 (private)
                                                   │
                                                   ├─► Postgres trong Dokploy (internal hostname)
                                                   └─► BlobStore: bucket S3 private (R2/AWS/GCS)
                                                       hoặc volume /var/lib/noverse/blobs

Mac ở nhà ── Tailscale ──► <tailscale-ip>:8081 (worker listener riêng, token DB)
```

- `:8080` **không publish**; Traefik là cửa duy nhất. `:5432` cũng không publish.
- Worker listener là mux riêng (`WORKER_LISTEN_ADDR`), không nằm trong router
  công khai; chỉ publish trên IP Tailscale của host.
- Một replica web (setup code/event hub trong process). Job/lease đã bền nên
  rolling restart không mất việc; nhưng scale >1 chưa được thiết kế.
- `/healthz` = liveness (không phụ thuộc gì). `/readyz` = DB + schema version +
  BlobStore. **Mac TTS không nằm trong readiness** — Mac rớt là hàng đợi chậm,
  không phải app bệnh.

Image: multi-stage `Dockerfile` — web (Node 22 + Vite) → Go 1.26 (static,
CGO off, dist nhúng đúng `internal/entry/web/dist`) → Alpine 3.22 có
ffmpeg/ffprobe/CA/tzdata, user `10001:10001`, chỉ
`/var/lib/noverse/blobs`, `/var/cache/noverse`, `/home/noverse/.ainovel` ghi được.

## 2. Chạy thử cục bộ (bản mirror của Dokploy)

`deploy/docker-compose.dokploy.yml` dựng đúng topology: Traefik TLS tự ký →
app → Postgres 17 + MinIO, kèm job migrate một lần và bucket init.

```sh
cd deploy
docker compose -f docker-compose.dokploy.yml up -d --build

# Lấy chứng chỉ tự ký để curl xác thực hostname thay vì -k
docker compose -f docker-compose.dokploy.yml cp traefik:/certs/noverse.crt /tmp/noverse.crt

# Lần đầu: chưa có tài khoản → đọc mã thiết lập một lần trong log
docker compose -f docker-compose.dokploy.yml logs app | grep 'Mã thiết lập'
curl -s --cacert /tmp/noverse.crt --resolve noverse.localhost:8443:127.0.0.1 \
  https://noverse.localhost:8443/api/setup
# POST /api/setup/verify {code} → setupToken → POST /api/setup/admin

# Smoke đầy đủ (healthz, readyz, login, tạo project/truyện, đọc lại, worker 404)
PUBLIC_URL=https://noverse.localhost:8443 \
SMOKE_USERNAME=dat SMOKE_PASSWORD='…' \
SMOKE_CA_CERT=/tmp/noverse.crt SMOKE_RESOLVE=noverse.localhost:8443:127.0.0.1 \
  ./smoke.sh

docker compose -f docker-compose.dokploy.yml down -v   # xoá volume test
```

Giá trị trong compose (`local-*-password`, MinIO keys) **chỉ dùng cho máy
local**; Dokploy lấy secret thật từ Environment/secret store.

Mật khẩu Postgres trong mirror **không** bị nội suy vào DSN: `postgres-init`
tạo role từ giá trị thô (qua psql variable, SQL-literal an toàn), còn
`deploy/dsn-entrypoint.sh` dựng `DATABASE_URL`/`MIGRATION_DATABASE_URL` và
percent-encode mọi ký tự dành riêng. Mật khẩu sinh tự động chứa `' " ; @ : / %
# ?` vẫn chạy — không được giới hạn bảng chữ cái. Chạy chứng minh:

```sh
cd deploy
POSTGRES_APP_PASSWORD='p@ss:wo/rd%'"'"'";x' \
POSTGRES_MIGRATE_PASSWORD='m!g@r:ate/%'"'"'";y' \
  docker compose -f docker-compose.dokploy.yml up -d --build
docker compose -f docker-compose.dokploy.yml logs migrate | tail -5   # áp dụng migration
docker compose -f docker-compose.dokploy.yml down -v
```

Ở Dokploy: dán **nguyên văn** DSN mà platform in ra (đã percent-encode).

## 3. Thiết lập trên Dokploy

### 3.1. Postgres

1. **Create Database → PostgreSQL 17**, tên `noverse`. Không bật public port.
2. Tạo hai role riêng trong database console (thay mật khẩu thật):
   nội dung `deploy/postgres-init/01-roles.sh` (phần SQL) chạy bằng user quản
   trị của Dokploy. Nó tạo `noverse_migrate` (chủ schema, chạy DDL) và
   `noverse_app` (chỉ DML), tạo sẵn `citext`/`pg_trgm`, đặt default privileges
   để bảng mới do migration role tạo vẫn đọc/ghi được bởi app role.
   Migration 00010 sau đó **thu hồi** UPDATE/DELETE trên bảng lịch sử
   (`audit_logs`, `chapter_versions`, `import_items`) khỏi app role: lịch sử
   chỉ được purge bởi `noverse_migrate` (hoặc superuser) qua lệnh bảo trì
   `storage purge-audit`, không bao giờ bởi thao tác đọc của app.
3. Ghi lại DSN nội bộ cho hai role:
   - `DATABASE_URL` → app role
   - `MIGRATION_DATABASE_URL` → migrate role (chỉ job migrate giữ)
4. Sau `storage migrate up`, tạo role engine-worker bằng script
   `deploy/postgres-init/03-engine-worker-role.sql` (chạy bằng user quản trị):
   `psql "$ADMIN_DSN" -v worker_password="$POSTGRES_WORKER_PASSWORD" -f .../03-engine-worker-role.sql`.
   Role `noverse_engine_worker` chỉ đọc/ghi bảng facts + story lease; không
   account/grant/audit/jobs/blobs, không DDL, không default privileges — bảng
   mới worker cần phải được grant chủ động trong file đó. DSN của nó đặt ở
   `NOVERSE_WORKER_DATABASE_URL`; worker hosted thiếu DSN này là fail closed,
   không bao giờ dùng `DATABASE_URL` của app.

Nếu cần TLS trong cụm, dùng `sslmode=require` + CA của Dokploy; `sslmode=disable`
chỉ chấp nhận trong mạng nội bộ đã kiểm soát (bản compose local).

### 3.2. Application

1. **Create Application** → repo `datnt1998/verse`, build bằng Dockerfile:
   Dockerfile path `apps/noverse/Dockerfile`, build context `.` (gốc repo).
2. Port/service `8080`, domain HTTPS (Let's Encrypt của Traefik), **1 replica**.
3. Health check: path `/healthz` (liveness). Nếu muốn chặn traffic khi DB chưa
   sẵn sàng, đặt health check của Traefik/Dokploy vào `/readyz`.
4. Environment: dán các khóa ở §4 (chỉ các khóa binary đọc). `PUBLIC_URL` =
   `https://<domain>`, `SECURE_COOKIES=true`, `TRUSTED_PROXIES` = subnet thật
   của Traefik:
   ```sh
   docker network inspect <dokploy-network> -f '{{range .IPAM.Config}}{{.Subnet}} {{end}}'
   ```
5. Volume (chỉ khi `STORAGE_DRIVER=local`): mount volume bền vào
   `/var/lib/noverse/blobs`. Với S3 private thì không cần volume blob.
6. Volume cấu hình model: `/home/noverse/.ainovel` (file cấu hình nhà cung cấp
   model; hiện chưa mã hoá at-rest — xem §10).
7. **Mã thiết lập** chỉ được in một lần ra log khi database chưa có tài khoản
   nào. Log của Dokploy phải giới hạn cho admin; sau khi tạo admin đầu tiên,
   mã hết hiệu lực và không xuất hiện lại.

### 3.3. Job migrate một lần

Dokploy không có job primitive riêng; dùng application thứ hai cùng image:

1. **Create Application** `noverse-migrate`, cùng repo/Dockerfile, **không domain**.
2. Override command: `storage migrate up` (entrypoint đã là `ainovel-cli`).
3. Env: `STORE_DRIVER=postgres`, `DATABASE_URL`, `MIGRATION_DATABASE_URL`.
4. Chạy (Deploy) một lần trước khi bật domain cho app; xem log kết thúc
   `Áp dụng …` hoặc `Không có migration nào chờ`.
5. App **không** có `MIGRATION_DATABASE_URL`, nên app không thể DDL. App role
   cũng không có quyền `CREATE`.

Kiểm tra: `storage migrate version` phải bằng `schemaVersion` trong `/readyz`.

### 3.4. Worker Mac (Tailscale)

1. VPS cài Tailscale; app publish port `8081` bind **chỉ** IP Tailscale:
   `ports: ["100.x.y.z:8081:8081"]`, `WORKER_LISTEN_ADDR=0.0.0.0:8081`.
2. Firewall host/Dokploy: chỉ mở 80/443 công khai; 8081 chỉ trên `tailscale0`;
   không mở 5432/8080.
3. Cấp credential worker bằng CLI **trên máy chủ** (cùng `STORE_DRIVER=postgres`
   và `DATABASE_URL`; không có màn UI nào cấp token):
   ```sh
   ainovel-cli worker token create --cap tts mac-mini   # in token đúng một lần
   ainovel-cli worker token list                        # không bao giờ in token
   ainovel-cli worker token revoke <uuid|tên-máy>
   ```
   Cờ phải đứng **trước** tên máy (Go flag dừng ở tham số vị trí đầu tiên; CLI
   từ chối cờ nằm sau tên thay vì gộp vào tên). Schema hiện chỉ cho phép
   capability `tts` — `--cap` khác bị từ chối ngay.
4. Mac: cài Tailscale + `ainovel-cli worker tts` với
   `TTS_WORKER_TOKEN=…` và `TTS_WORKER_API_URL=http://100.x.y.z:8081`
   (hoặc `--api`; cờ thắng env). Mac không có DSN Postgres/bucket secret.
   `--lease` là **yêu cầu** phía client; máy chủ kẹp trong 30s–10m nên một Mac
   chết không giữ việc quá lâu.
5. Kiểm chứng từ mạng thật: `curl -X POST http://<tailscale-ip>:8081/internal/worker/claim`
   trả `401` khi không token; từ Internet/LAN thường phải **connection refused**.

## 4. Tham chiếu env đầy đủ

Bản mẫu có chú thích: `deploy/.env.example`. Tóm tắt khóa binary thực đọc:

| Nhóm | Khóa | Ý nghĩa |
|---|---|---|
| Facts | `STORE_DRIVER` | `postgres` (hosted) hoặc `file` (CLI/offline) — chọn trọn bộ fact + account backend |
| | `DATABASE_URL` | DSN app role (không log); là URI nên mật khẩu phải percent-encode |
| | `MIGRATION_DATABASE_URL` | DSN migrate role, chỉ job migrate |
| | `DB_MAX_CONNS`, `DB_CONNECT_TIMEOUT` | bound pool (số nguyên 1..100) + timeout kết nối (1s..2m); sai là lỗi khởi động |
| Blob | `STORAGE_DRIVER` | `local` hoặc `s3` (độc lập `STORE_DRIVER`) |
| | `STORAGE_ENV` | `dev`\|`staging`\|`prod` — đoạn đầu của **mọi** khoá object, khoá backup và phạm vi `storage gc`. App và `storage gc`: trống = `dev` (giống `blobkey`). `deploy/backup.sh`: **bắt buộc**, không có mặc định — thiếu biến là dừng, vì backup vào sai tiền tố tệ hơn một lần chạy lỗi. Giá trị lạ là lỗi ở cả hai phía |
| | `BLOB_LOCAL_ROOT` | gốc tuyệt đối của local driver |
| | `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET` | recipe provider (xem `.env.example`) |
| | `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | credentials/HMAC |
| | `S3_FORCE_PATH_STYLE` | `true` cho MinIO/GCS, `false` AWS/R2 |
| | `S3_PUBLIC_BASE_URL` | chỉ cho asset public; blob private vẫn presign/stream |
| | `S3_ALLOW_INSECURE_ENDPOINT` | chỉ `http://` nội bộ test (MinIO) |
| Server | `HTTP_ADDR` | mặc định container `0.0.0.0:8080` |
| | `PUBLIC_URL` | origin chuẩn; bắt buộc `https://` khi secure cookie |
| | `TRUSTED_PROXIES` | CIDR của proxy được phép gắn X-Forwarded-*; một tên duy nhất (không alias), từ chối `0.0.0.0/0` |
| | `SECURE_COOKIES` | `true` bắt buộc hosted |
| | `VERSE_COOKIE_DOMAIN` | domain cha cho `verse_session` dùng chung (SSO Noverse↔Viderse); chỉ `STORE_DRIVER=postgres`, bắt buộc `SECURE_COOKIES` và DNS-label chứa host của `PUBLIC_URL`; từ chối public suffix/IP/wildcard/single label |
| | `VERSE_APP_ORIGINS` | origin sibling được phép cho request ghi (match canonical chính xác): hosted chỉ `https` nằm trong cookie domain; local chỉ loopback và bắt buộc bind loopback |
| | `NOVERSE_WORKER_DATABASE_URL` | DSN role `noverse_engine_worker` (facts/lease thôi) cho worker hosted; thiếu là fail closed, không bao giờ fallback `DATABASE_URL` |
| Worker | `WORKER_LISTEN_ADDR` | bind của mux worker (private) |
| | `TTS_WORKER_API_URL`, `TTS_WORKER_TOKEN` | **phía Mac**, không đặt trên VPS |

Bí mật (DSN, S3 keys, worker token) đặt trong Dokploy Environment/secret store.
TTL setup code, cửa sổ login limiter, tham số Argon2id, TTL session, presign TTL
và **biên lease job (30s–10m, máy chủ kẹp)** là hằng số trong code, không phải
env — deployment không thể nới lỏng chúng. Worker được gửi `--lease` như một
yêu cầu, nhưng giá trị cuối cùng do máy chủ quyết định.

## 5. Proxy, cookie và CSRF (đã cài trong code)

- Chỉ peer nằm trong `TRUSTED_PROXIES` mới được tin `X-Forwarded-For` /
  `X-Forwarded-Proto`; client IP đi ngược XFF qua các hop tin cậy, dừng ở hop
  đầu tiên không tin cậy. Giá trị proto không hợp lệ/nhiều giá trị bị bỏ qua.
  Để trống `TRUSTED_PROXIES` thì không tin peer nào, kể cả loopback.
- `SECURE_COOKIES=true` → cookie phiên luôn `Secure` (HttpOnly, SameSite=Strict,
  path `/`), không phụ thuộc header do client gửi.
- Request ghi dữ liệu (POST/PUT/PATCH/DELETE) bị `403` ở **mọi chế độ** khi có
  `Sec-Fetch-Site: cross-site`, hoặc khi có `Origin` không hợp lệ. Có
  `PUBLIC_URL`: chỉ đúng origin đó (cả scheme). Không có `PUBLIC_URL`: origin
  phải trùng host:port của chính request, và host phải là IP (127.0.0.1, ::1,
  địa chỉ LAN/Tailscale mà điện thoại mở) hoặc `localhost`; tên miền khác bị
  chặn để tránh DNS rebinding, nên ở local hãy mở bằng IP. So theo host:port,
  nên trang của app khác trên cùng máy (ví dụ `127.0.0.1:8789` gửi sang
  `127.0.0.1:8790`) bị chặn dù cookie `SameSite=Strict` không phân biệt cổng.
  Request không có `Origin` (CLI, worker, curl) vẫn qua.
- Body của POST/PUT phải là `application/json` (trừ multipart upload hiện có);
  PATCH/DELETE có body cũng vậy, sai thì `400`. DELETE không body vẫn qua.
- Muốn cookie `Secure` khi đứng sau một proxy loopback (ví dụ `tailscale serve`
  terminate HTTPS rồi chuyển về `127.0.0.1`): đặt `SECURE_COOKIES=true` cùng
  `PUBLIC_URL=https://<tên máy>…`, vì thiếu `PUBLIC_URL` https thì binary từ
  chối khởi động. Thêm `TRUSTED_PROXIES=127.0.0.1/32` nếu cần tin
  `X-Forwarded-*` (IP client cho limiter) từ proxy đó.
- Với `STORE_DRIVER=postgres`, cookie phiên là `verse_session` dùng chung giữa
  Noverse và Viderse: `VERSE_COOKIE_DOMAIN` (hosted, parent domain + Secure)
  hoặc host-only (local, hai app loopback khác cổng). `VERSE_APP_ORIGINS` mở
  thêm origin sibling **chính xác** cho request ghi ngoài own-origin — match
  canonical byte-for-byte sau chuẩn hoá scheme/host/port, không suffix hay
  wildcard, `Sec-Fetch-Site:cross-site` vẫn luôn bị chặn kể cả origin trong
  allowlist, và origin được allowlist **không** tự cấp quyền nghiệp vụ: quyền
  app đến từ hàng `app_grants` đọc lại mỗi request (admin hệ thống luôn qua,
  member thiếu grant nhận `403 app_access_denied`). File backend giữ
  `ainovel_session` độc lập và đặt các biến SSO ở file mode là lỗi khởi động.
- `/healthz` và `/readyz` được xử lý **trước** static SPA (nếu không sẽ trả
  index.html 200 giả).

## 6. Sao lưu định kỳ

### 6.1. Lịch

- **Dokploy Database → Backups**: destination S3 ngoài VPS, cron
  `15 3 * * *`, giữ 30 ngày (`retention`). Bật versioning/object-lock ở bucket.
- **`deploy/backup.sh`** (khuyến nghị chạy song song, cho blob manifest +
  checksum độc lập):
  ```sh
  # /opt/noverse/backup.env (0600) — DSN read-only/migrate, S3 backup creds
  CRON_TZ=UTC
  15 3 * * * cd /opt/noverse && set -a && . ./backup.env && set +a \
    && ./deploy/backup.sh >> /var/log/noverse-backup.log 2>&1
  ```
  Mỗi lần chạy tạo một bộ file phẳng trong thư mục ngày UTC
  `<STORAGE_ENV>/backups/pg/YYYY/MM/DD/`: `<ts>.dump` (pg_dump -Fc),
  `<ts>.blobs.tsv` (object_key, sha256, size của blob `ready`),
  `<ts>.schema.txt` (`schema.format`, migration version),
  `<ts>.SHA256SUMS` (chỉ liệt kê các file payload). Drill mode ghi cùng bố cục
  dưới `S3_BACKUP_DEST_DIR` và không cần `S3_REGION`/credential.
  `backup.env` phải đặt `STORAGE_ENV` (`dev`|`staging`|`prod`): script không
  đoán môi trường và dừng nếu biến trống.
  Script chỉ dùng **một** công cụ upload: AWS CLI v2 (`aws s3 cp`). Hợp đồng
  biến vẫn là `S3_BACKUP_BUCKET`, `S3_ENDPOINT` (trống = AWS), `S3_REGION`,
  `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` (và `S3_SESSION_TOKEN` nếu dùng
  credential tạm); script ánh xạ chúng thành `AWS_ACCESS_KEY_ID`,
  `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `--region`, `--endpoint-url`
  và bỏ `AWS_PROFILE` khỏi môi trường, nên một profile sẵn có trên host không
  bao giờ chọn nhầm principal. Credential backup là secret riêng, quyền khác
  credential blob của ứng dụng dù tên biến trùng nhau ở hai file env khác nhau.
  `mc` không được hỗ trợ (không cài song song để tránh chọn nhầm công cụ).
- **Múi giờ của lịch**: tên thư mục và mọi mốc trong backup là UTC, nhưng cron
  của host chạy theo giờ địa phương của host. Đặt `CRON_TZ=UTC` (hoặc cấu hình
  timezone trong Dokploy/`systemd`) nếu muốn 03:15 là 03:15 UTC; script
  `backup.sh` không thể tự đổi lịch của scheduler.
- Blob: bucket S3 do provider bền hoá; **backup DB không bao gồm object**.
  Với `STORAGE_DRIVER=local`, đặt `BLOB_LOCAL_ROOT` để script tar volume, và
  chép volume đó ra ngoài VPS riêng.
- Volume `/home/noverse/.ainovel` (cấu hình model) chép riêng — không có
  trong dump DB.

RPO = chu kỳ cron (24h mặc định), RTO = thời gian restore + verify (đo ở drill).

### 6.2. Kiểm chứng backup (drill cục bộ đã chạy)

```sh
# drill: ghi ra thư mục local thay vì S3, xác minh checksum + manifest
DATABASE_URL=… S3_BACKUP_BUCKET=noverse S3_BACKUP_DEST_DIR=/tmp/drill \
  deploy/backup.sh

# restore vào database nháp rồi so số dòng với production
createdb -h … -U … noverse_restore
pg_restore -h … -U … -d noverse_restore --no-owner --no-privileges /tmp/drill/*/noverse.dump
psql … -d noverse_restore -c \
  "SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM projects),
          (SELECT count(*) FROM stories), (SELECT count(*) FROM sessions),
          (SELECT max(version_id) FROM goose_db_version WHERE is_applied)"
```

Bằng chứng local (2026-10-29): dump 148 KB, `schema.format=1`,
`migration.version=7`; restore vào `noverse_restore` cho counts
`1 1 1 1 7` **trùng** production, đọc lại đúng truyện `01-chuong-dau`.

## 7. Runbook phục hồi

1. Chọn thư mục backup (`noverse-<UTC>/`); tải `SHA256SUMS`, chạy
   `sha256sum -c SHA256SUMS`.
2. Đọc `schema.txt`. Nếu `migration.version` lớn hơn version binary đang chạy →
   phải deploy image tương ứng trước (schema expand-only, không `goose down`).
3. Tạo database mới (không restore đè production đang chạy):
   ```sh
   createdb -O noverse_migrate noverse_restore
   pg_restore -d noverse_restore --no-owner --no-privileges noverse.dump
   ```
4. Kiểm tra: counts theo bảng chính, `storage migrate version` (dùng
   `MIGRATION_DATABASE_URL` trỏ DB nháp) = `schema.txt`.
5. Blob: so `blobs.tsv` với bucket (`mc ls --recursive`, hoặc `storage reconcile
   --manifest <manifest của lần import> --verify-blobs`). Thiếu object → restore
   object từ versioning/bản sao bucket trước khi mở traffic.
6. Chuyển app sang DB đã restore: cập nhật `DATABASE_URL` trong Dokploy →
   deploy lại (migration job chạy `up`, phải no-op).
7. **Thu hồi phiên cũ** trước khi mở traffic (cookie trong backup vẫn hợp lệ).
   Phải tăng `version` và đổi `updated_at` trong cùng câu lệnh — trigger chặn
   update không tuân hợp đồng:
   ```sql
   UPDATE sessions
      SET revoked_at = clock_timestamp(), updated_at = clock_timestamp(),
          version = version + 1
    WHERE revoked_at IS NULL;
   ```
8. `/readyz` phải 200, chạy `deploy/smoke.sh`, rồi mới mở traffic.
9. Ghi lại RTO thực tế và số blob/checksum đã đối soát vào log sự cố.
10. Audit retention chạy tay dưới vai trò bảo trì, không xoá khi đọc trang
    audit: `ainovel-cli storage purge-audit --days 365` với
    `MIGRATION_DATABASE_URL` (app role sẽ bị trigger từ chối).

## 8. Runbook cutover

Điều kiện vào: phase 08 đã rehearsal import/reconcile/restore trên bản sao;
image đã build; backup mới nhất đã restore thử thành công.

1. **Thông báo + maintenance**: chặn traffic (Dokploy scale app = 0 hoặc domain
   trỏ trang bảo trì). Ghi mốc thời gian bắt đầu freeze.
2. **Freeze writes**: dừng CLI/TUI/web file mode trên máy nhà, dừng worker Mac
   (không còn claim/lease). Chờ lease hiện tại hết hạn hoặc release qua UI.
   Không được có writer nào sau bước này.
3. **Snapshot bất biến**: copy `~/NoverseLive` + `web-accounts.json` sang thư
   mục snapshot, `chmod -R a-w`, ghi `shasum -a 256` toàn bộ. Đây là nguồn
   rollback trước khi chuyển quyền.
4. **Dry-run import**:
   ```sh
   ainovel-cli storage import --root SNAPSHOT --accounts SNAPSHOT/web-accounts.json \
     --source-id <uuid cố định> --owner-map 'du-an=username,…' \
     --manifest /srv/cutover/manifest.json --dry-run
   ```
   Xem conflicts/quarantine; chỉ đi tiếp khi 0 conflict ngoài dự kiến.
5. **Import thật** (cùng lệnh, bỏ `--dry-run`). Chạy lại lần hai (hoặc
   `--resume`) phải **không thêm row/blob**.
6. **Đối soát**: `storage reconcile --manifest … --verify-blobs --json` →
   mismatch = 0; lưu JSON làm biên bản. Blob store phải là đích thật (bucket
   private hoặc volume đã mount).
7. **Chứng minh phục hồi**: chạy `deploy/backup.sh` ngay sau import, restore
   vào DB nháp, so counts + schema version (mục 6.2). Không mở traffic khi bước
   này chưa đạt.
8. **Schema + app dưới maintenance**: chạy job `storage migrate up`; deploy app
   với env hosted; `/readyz` phải 200, `schemaVersion` khớp migration version.
9. **Smoke trước khi mở**: `deploy/smoke.sh` qua HTTPS thật (login, tạo
   project/truyện, đọc lại, worker path 404). Thử thêm blob: nghe 1 audio/export
   nếu đã có dữ liệu.
10. **Mở traffic + claims**: bật domain, bỏ maintenance, bật worker Mac. Theo dõi
    30 phút: login, ghi chương, comment/review, job audio, export.
11. **Quan sát**: restart app một lần (dữ liệu + phiên phải còn), chạy backup
    đầu tiên, xác nhận backup lên S3. Giữ image cũ, snapshot file và backup
    pre-cutover **7 ngày** (cấu hình được) trong cửa sổ rollback.
12. Sau khi chuyển quyền: **không ghi** vào thư viện file cũ nữa.

### Rollback

- **Trước khi app nhận traffic** (bước 1–7): dừng app, trả domain về release
  file mode, mở lại snapshot (bỏ `a-w`), khởi động CLI/web file. Không cần
  reverse export vì chưa có ghi mới.
- **Sau khi đã có ghi mới**: **không** trỏ lại snapshot cũ.
  - Ưu tiên: rollback image về release DB-compatible trước đó, giữ nguyên DB
    (schema expand-only). Dokploy rollback image **không** undo migration hay
    blob đã copy.
  - Nếu buộc quay về file mode: maintenance → freeze → `storage export-files
    --destination NEW_DIR --revoke-sessions` → đối soát/verify → chạy release
    file trên `NEW_DIR`. Không chạy `goose down` khi còn writer.
  - Thu hồi session/lease sau mọi lần rollback; giữ audit/history.

## 9. Vòng đời object và khoá blob (S3/R2/GCS)

### 9.1. Khoá

`STORAGE_ENV` là segment đầu tiên của **mọi** khoá; một bucket có thể chứa
nhiều môi trường và không quy tắc nào được phép băng qua tiền tố.

| Khoá | Nội dung |
|---|---|
| `{env}/p/{projectUUID}/s/{storyUUID}/audio/seg/{sha[:2]}/{sha}.{ext}` | segment TTS, content-addressed |
| `{env}/p/{projectUUID}/s/{storyUUID}/audio/ch/v{vol}/c{ch}/{sha}.{ext}` | audio chương đã ghép |
| `{env}/p/{projectUUID}/s/{storyUUID}/export/{sha}/{name}` | bản xuất (tên chỉ để người đọc) |
| `{env}/p/{projectUUID}/s/{storyUUID}/cover/{sha}.{ext}`, `…/music/{sha}.{ext}` | bìa / nhạc nền |
| `{env}/p/{projectUUID}/s/{storyUUID}/import/{sha[:2]}/{sha}-{pathhash}-{name}` | tệp nhập theo truyện |
| `{env}/p/{projectUUID}/import/…`, `{env}/p/{projectUUID}/assets/…` | blob cấp dự án (không thuộc truyện nào) |
| `{env}/u/{userUUID}/avatar/{sha}.{ext}` | avatar |
| `{env}/staging/{uploadUUID}` | upload dở (multipart/resumable) |
| `{env}/backups/pg/YYYY/MM/DD/<ts>.dump` | dump DB (+ `<ts>.blobs.tsv`, `<ts>.schema.txt`, `<ts>.SHA256SUMS`) |

Định danh là UUID, nội dung là SHA-256 đầy đủ (64 hex); đổi tên dự án/truyện
không di chuyển byte nào. Khoá do `internal/blobkey` sở hữu — không gói nào
khác được ghép khoá.

### 9.2. Quy tắc vòng đời theo provider

Chỉ đặt quy tắc trên **tiền tố**, không bao giờ trên cả bucket: quy tắc toàn
bucket sẽ xoá blob đang sống.

**S3 (lifecycle rule JSON / console)**
- Rule `expire-staging`: filter prefix `{env}/staging/`,
  `Expiration.Days=1`, `AbortIncompleteMultipartUpload.DaysAfterInitiation=1`.
- Rule `retain-backups`: filter prefix `{env}/backups/pg/`; giữ bản cũ theo
  `NoncurrentVersionExpiration`/Object Lock tuỳ chính sách (ví dụ 90 ngày).
  **Không** đặt `Expiration.Days` cho prefix này trừ khi chấp nhận mất bản
  backup cũ.
- Bật versioning + (tuỳ chọn) Object Lock cho prefix backup.

**Cloudflare R2** — Rules → Lifecycle: `Prefix = {env}/staging/`,
`Delete after 1 day`; thêm rule riêng cho `{env}/backups/pg/` nếu muốn xoá bản
cũ. R2 không có Object Lock kiểu S3; versioning (nếu bật) là lớp bảo vệ duy
nhất.

**GCS (XML interop)** — Object Lifecycle Management:
`{action: Delete, condition: {age: 1, matchesPrefix: ["{env}/staging/"]}}`.
Với backup dùng retention policy hoặc rule `age` dài hơn; đừng để rule
`age: 1` áp cả bucket.

Lifecycle của provider và `storage gc` bổ sung nhau: lifecycle dọn
`{env}/staging/` kể cả khi GC không chạy; GC là thứ duy nhất biết blob nào còn
được bảng nào tham chiếu.

### 9.3. `storage gc`

```sh
# dry-run trước, rồi cron hằng đêm (UTC)
# Nạp secret của vai trò bảo trì, không dùng riêng DSN app.
set -a; . /opt/noverse/gc.env; set +a
STORAGE_ENV=prod ainovel-cli storage gc --dry-run

# /opt/noverse/gc.env (0600) — MIGRATION_DATABASE_URL=noverse_migrate + S3_*
# DATABASE_URL vẫn là DSN app nếu STORE_DRIVER=postgres yêu cầu.
CRON_TZ=UTC
30 3 * * * cd /opt/noverse && set -a && . ./gc.env && set +a \
  && STORAGE_ENV=prod ./ainovel-cli storage gc >> /var/log/noverse-gc.log 2>&1
```

Một lượt dọn, theo thứ tự: hàng `staged` cũ hơn `--staged-age` (mặc định 24h,
tính từ `created_at`) không có tham chiếu/job hoạt động được đổi sang
`deleting` dưới `FOR UPDATE` → tombstone `blobs.state = 'deleting'` →
`export_cache` hết hạn (và blob chỉ cache đó tham chiếu) → blob ready không bảng
nào tham chiếu, cũ hơn `--orphan-age` (24h, cả `created_at` và `updated_at`) →
object trong `{env}/staging/` cũ hơn `--staging-age` (24h) → object mồ côi dưới
`{env}/` cũ hơn grace. `--staged-age` là tuổi hàng DB, khác `--staging-age`
là tuổi object multipart/resumable.

- Mọi quyết định xoá **hàng** được lấy lại trong transaction có khoá hàng, nên
  blob đang được gắn (FK `KEY SHARE`) không thể bị xoá; object chỉ bị xoá sau
  khi commit (crash giữa hai bước để lại object mồ côi cho lượt sau, không bao
  giờ để lại hàng trỏ vào byte đã mất).
- Job upload được liên kết bằng `blobs.upload_job_id`. GC khoá job trước blob:
  lease chưa hết hạn hoặc job queued/paused/waiting (hay running không có lease)
  luôn giữ hàng; running đã hết lease được xem là bỏ dở. Worker thắng race
  `staged → ready` thì hàng được giữ, và `updated_at` mở lại grace của ready.
  Với hàng cũ/đồng bộ chưa có `upload_job_id`, GC giữ bảo thủ khi **bất kỳ**
  job nào trong cùng project còn hoạt động; không đoán job sở hữu. Khi mọi job
  đó đã hết hoạt động/lease, hàng bỏ dở được dọn bình thường.
- `storage gc` bắt buộc `MIGRATION_DATABASE_URL` **và** principal bảo trì
  (`noverse_migrate` hoặc superuser); không bao giờ fallback sang `DATABASE_URL`.
  Migration 00011 thu hồi DELETE của app trên `blobs`/`export_cache`; lịch sử
  `audit_logs`/`chapter_versions`/`import_items` vẫn không cho app UPDATE/DELETE.
  App chỉ đánh dấu hết hạn cache (`expires_at`) hoặc tombstone (`state`);
  xoá hàng/object thật chỉ qua GC. Không cấp secret bảo trì cho service app.
- `--dry-run` chỉ báo cáo, không giữ khoá, không xoá; chạy lại một lượt thật
  là no-op.
- Không bao giờ chạm tiền tố môi trường khác và không đụng `{env}/backups/`;
  khoá không nhận dạng được chỉ bị báo, không bị xoá.

## 10. Đã kiểm chứng cục bộ / còn lại

Đã chạy (2026-10-29, Docker 29.2.1, compose v5.0.2, arm64):

- `docker build` thành công; runtime `uid=10001`, `ffmpeg 6.1.2`,
  `ffprobe 6.1.2`; không có node/npm/curl trong ảnh.
- Stack compose (Traefik TLS tự ký → app → Postgres/MinIO) healthy; migrate job
  áp dụng toàn bộ manifest hiện tại 00001–00010
  (`internal/persistence/migrations/migrations.go`); app role **không** tạo được
  bảng, migrate role chạy DDL; `/readyz` từ chối schema thiếu migration nhúng.
- HTTPS: `/healthz` 200, `/readyz` 200 `schemaVersion=10` (manifest 00001–00010),
  SPA 200, anonymous `/api/me` 401, login 200 với
  `Secure; HttpOnly; SameSite=Strict`, tạo project/truyện 201 và đọc lại đúng.
- Dừng Postgres: `/healthz` vẫn 200, `/readyz` 503 `database:false`; bật lại →
  200. Restart app: truyện và phiên vẫn còn (dữ liệu trong Postgres).
- Worker: publish `127.0.0.1:8081` trả 401 không token; LAN IP:8081 refused;
  `/internal/worker/claim` qua router công khai = 404. `app:8080` không publish.
- Backup drill + restore vào DB nháp: counts trùng, schema version 10.
- `deploy/smoke.sh` PASS toàn bộ trên stack local.

Đã chạy lại (2026-10-29, sau khi bỏ `PG_ACCOUNTS`, đổi init SQL + DSN encoding):

- `docker compose -p stdops up` với mật khẩu chứa `' " ; @ : / %`
  (`POSTGRES_APP_PASSWORD='p@ss:wo/rd%'";x'`,
  `POSTGRES_MIGRATE_PASSWORD='m!g@r:ate/%'";y'`), port lệch (HTTP 18080,
  HTTPS 18443, worker 18081):
  - `migrate` exit 0, áp dụng 00002–00010 qua `MIGRATION_DATABASE_URL` do
    `deploy/dsn-entrypoint.sh` percent-encode (không nội suy vào DSN).
  - `app` healthy, log `Tài khoản: postgres (DATABASE_URL)`; `psql -U noverse_app`
    với mật khẩu **thô** đăng nhập được → `noverse_app|UTC` (quote_literal an
    toàn + session timezone UTC).
  - `GET /readyz` (Host: noverse.localhost) →
    `{"status":"ok","schemaVersion":10,"checks":[{"name":"database","ok":true},{"name":"blob","ok":true}]}`.
  - `GET :18081/internal/worker/claim` → 405 (cổng worker reachable, sai method).
  - `down -v` sạch, không còn container.
- Chứng minh hỏng-trước-khi-sửa: `CREATE ROLE … PASSWORD '<pw có nháy>'` kiểu
  nội suy shell → `ERROR: unterminated quoted identifier`; DSN thô có mật khẩu
  chứa `%` → `psql: error: invalid percent-encoded token`. Cùng mật khẩu qua
  psql variable + DSN đã encode → OK.
- `production` note: một secret MinIO chứa nháy đơn vẫn có thể phá
  `minio-init` trong `docker-compose.dokploy.yml` (interpolation trong single
  quotes) — chưa sửa vì thuộc phần khoá blob; dùng secret không chứa `'` cho
  MinIO cho tới khi phần đó đổi.

Còn lại (cần môi trường thật, không thể giả lập ở local):

- CIDR Traefik thật của Dokploy + firewall/Tailscale ACL thật; kiểm tra
  443/8081 từ mạng ngoài.
- ACME/Let's Encrypt và domain thật; S3 provider thật (R2/AWS/GCS) + presign.
- Audio end-to-end với Mac/VieNeu thật (queue → play/range → export → restart).
- Backup Dokploy → S3 thật + đo RTO/RPO trên dữ liệu thật.
- Mã hoá at-rest cho file cấu hình model (`CONFIG_ENCRYPTION_KEY` mà plan nhắc
  hiện **chưa** được code đọc; đừng đặt env này và tưởng nó có tác dụng).
- Multi-replica (chưa thiết kế; giữ 1 replica).

## 11. Live trên Mac (launchd)

Bản live cá nhân chạy file mode trên Mac, tách khỏi repo đang phát triển:

| Mục | Giá trị |
|---|---|
| Worktree | `/Users/dat/Projects/verse-live`, nhánh `live` của `datnt1998/verse` |
| Binary | `verse-live/bin/noverse`, build bằng `apps/noverse/scripts/run-live.sh update` |
| Dữ liệu | Sách ở `~/NoverseLive`; tài khoản, cấu hình model, OAuth ở `~/.ainovel` |
| Địa chỉ | `127.0.0.1:8790` (`NOVERSE_LIVE_ADDR`, `NOVERSE_LIVE_ROOT` để đổi) |
| launchd | `~/Library/LaunchAgents/dev.verse.noverse-live.plist`: chạy `run-live.sh`, `RunAtLoad`, `KeepAlive`, PATH gồm `~/.local/bin` (uv) và `/opt/homebrew/bin` (ffmpeg, rg) |
| Log | `~/Library/Logs/noverse-live.log` |

```sh
# Cập nhật lên main mới nhất rồi khởi động lại
/Users/dat/Projects/verse-live/apps/noverse/scripts/run-live.sh update
launchctl kickstart -k gui/$(id -u)/dev.verse.noverse-live

# Dừng hẳn (chỉ kill process thì KeepAlive bật lại ngay)
launchctl bootout gui/$(id -u)/dev.verse.noverse-live

# Bật lại
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/dev.verse.noverse-live.plist
```

Không chạy song song một bản khác trên cùng `~/.ainovel` (ví dụ service omp
`noverse-live*` cũ): hai process cùng ghi file tài khoản. Trước khi cập nhật lớn,
sao lưu `~/NoverseLive` và `~/.ainovel` bằng `umask 077` rồi `tar` vào
`~/Backups/`; file chứa secret, không đưa vào repo.
