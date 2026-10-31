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
app → Postgres 17 + Garage v2.5.0, kèm job migrate một lần và bucket/key init.

```sh
cd deploy
# Đặt trước các secret Garage/S3 theo deploy/.env.example (không dùng secret thật).
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

Mật khẩu Postgres mặc định trong compose **chỉ dùng cho máy local**. Garage yêu
cầu RPC/admin token và cặp S3 key mới qua env; Dokploy dùng secret store riêng.

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
2. Chạy `deploy/postgres-init/01-roles.sh` bằng **superuser** của Dokploy
   (init hook của image Postgres đã dùng superuser). Nạp secret từ file 0600
   trong subshell, không gõ mật khẩu vào console hoặc giữ trong shell vận hành:
   `( set -a; . /secure/postgres-roles.env; exec sh deploy/postgres-init/01-roles.sh )`.
   File chứa `POSTGRES_USER`, `POSTGRES_DB`, `POSTGRES_MIGRATE_PASSWORD`,
   `POSTGRES_APP_PASSWORD` và thông tin kết nối PG cần thiết. Script tạo
   `noverse_migrate` (chủ schema, chạy DDL), `noverse_app` (chỉ DML),
   `citext`/`pg_trgm` và default privileges cho bảng mới của migration role.
   Khi cần tạo login, script tắt `log_transaction_sample_rate` ở mức session
   **trước BEGIN**, rồi tắt log câu lệnh, log duration/sample/debug trong
   transaction tạo role. Các script tự bật `ON_ERROR_STOP` và dừng trước khi
   gửi mật khẩu nếu không thể tắt log; không phụ thuộc cờ của caller.
   Migration 00010 sau đó **thu hồi** UPDATE/DELETE trên bảng lịch sử
   (`audit_logs`, `chapter_versions`, `import_items`) khỏi app role: lịch sử
   chỉ được purge bởi `noverse_migrate` (hoặc superuser) qua lệnh bảo trì
   `storage purge-audit`, không bao giờ bởi thao tác đọc của app.
3. Ghi lại DSN nội bộ cho hai role:
   - `DATABASE_URL` → app role
   - `MIGRATION_DATABASE_URL` → migrate role (chỉ job migrate giữ)
4. Sau `storage migrate up`, tạo role engine-worker bằng script
   `deploy/postgres-init/03-engine-worker-role.sql` (**superuser**):
   `( set -a; . /secure/worker-role.env; exec psql -X "$ADMIN_DSN" -f deploy/postgres-init/03-engine-worker-role.sql )`.
   File 0600 chứa `WORKER_ROLE_PASSWORD`; cần psql 15+ (`\getenv`).
   Automation cũng chỉ truyền mật khẩu qua env, không qua đối số `-v`.
   `$ADMIN_DSN` không kèm mật khẩu (`PGPASSFILE`). Với login chưa có, script
   từ chối mật khẩu rỗng, tắt transaction sampling **trước BEGIN**, rồi tắt
   statement/duration/sample/debug logging và `track_activities` trong transaction
   CREATE ROLE để mật khẩu không xuất hiện trong `pg_stat_activity.query`.
   Script đặt `pg_stat_statements.track=none` và `track_utility=off` transaction-local,
   nên cả SELECT chứa mật khẩu lẫn CREATE ROLE đều không được extension thu thập.
   Không bật/nạp `auto_explain` hay `pgaudit` trong phiên này; script cũng tắt
   chúng transaction-local. Backend của transaction được kiểm tra lại: nếu
   transaction pooler chọn backend còn sampling, script rollback trước khi
   gửi mật khẩu.
   Role `noverse_engine_worker` chỉ đọc/ghi bảng facts + story lease; không
   account/grant/audit/jobs/blobs, không DDL, không default privileges — bảng
   mới worker cần phải được grant chủ động trong file đó. DSN của nó đặt ở
   `NOVERSE_WORKER_DATABASE_URL`; worker hosted thiếu DSN này là fail closed,
   không bao giờ dùng `DATABASE_URL` của app.
5. Khi chạy Viderse cùng kho tài khoản: tạo role `viderse_identity` bằng
   `deploy/postgres-init/02-identity-role.sql` (**superuser**, sau
   `storage migrate up`):
   `( set -a; . /secure/identity-role.env; exec psql -X "$ADMIN_DSN" -v identity_schema=public -f deploy/postgres-init/02-identity-role.sql )`.
   File 0600 chứa `IDENTITY_ROLE_PASSWORD`, theo cùng quy tắc bảo vệ log,
   `pg_stat_activity` và từ chối mật khẩu rỗng khi tạo login.
   Có thể tạo login không mật khẩu bằng
   superuser rồi đặt bằng `\password viderse_identity` (tương tự engine-worker).
   Khi login đã có, cả hai SQL script không đọc/gửi mật khẩu và không đổi GUC
   ghi log. Chủ database/schema/bảng có CREATEROLE và ADMIN OPTION trên login
   có thể chạy lại không cần env mật khẩu để đồng bộ thuộc tính/grants.
   Role này chỉ chạm bảng tài khoản/phiên/grant, ghi thêm audit, và chỉ tạo
   được tài khoản khi kho còn trống. DSN của nó là `VERSE_IDENTITY_DATABASE_URL`
   của Viderse, không bao giờ cấp cho Noverse.

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
| | `S3_FORCE_PATH_STYLE` | `true` cho Garage/GCS, `false` AWS/R2 |
| | `S3_ALLOW_INSECURE_ENDPOINT` | chỉ `http://` mạng nội bộ; không dùng cho kho công khai |
| | `S3_PROVIDER` | preset tuỳ chọn; `garage` mặc định region `garage` và bắt buộc path style |
| | `BLOB_DELIVERY` | `proxy` truyền có Range qua app; `redirect` trả link ký. Mặc định proxy cho HTTP/IP riêng/hostname nội bộ, redirect cho đám mây |
| | `S3_PRESIGN_ENDPOINT` | địa chỉ trình duyệt truy cập được; client ký riêng, không sửa host của URL đã ký |
| | `CONFIG_ENCRYPTION_KEY` | base64 chuẩn của đúng 32 byte, dùng AES-256-GCM cho credentials kho lưu trong DB; giá trị sai làm khởi động thất bại |
| Server | `HTTP_ADDR` | mặc định container `0.0.0.0:8080` |
| | `PUBLIC_URL` | origin chuẩn; bắt buộc `https://` khi secure cookie |
| | `TRUSTED_PROXIES` | CIDR của proxy được phép gắn X-Forwarded-*; một tên duy nhất (không alias), từ chối `0.0.0.0/0` |
| | `SECURE_COOKIES` | `true` bắt buộc hosted |
| | `VERSE_COOKIE_DOMAIN` | domain cha cho `verse_session` dùng chung (SSO Noverse↔Viderse); chỉ `STORE_DRIVER=postgres`, bắt buộc `SECURE_COOKIES` và DNS-label chứa host của `PUBLIC_URL`; từ chối public suffix/IP/wildcard/single label |
| | `VERSE_APP_ORIGINS` | origin sibling được phép cho request ghi (match canonical chính xác): hosted chỉ `https` nằm trong cookie domain; local chỉ loopback và bắt buộc bind loopback |
| | `VERSE_APP_LINKS` | danh sách `noverse=https://nov.example.com,viderse=https://vid.example.com`; chỉ hai id đã biết, mỗi id một lần, origin phải là `PUBLIC_URL` hoặc nằm trong `VERSE_APP_ORIGINS`; sai thì từ chối khởi động. Không đặt: chỉ app hiện tại, ẩn bộ chuyển app |
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
- `GET /api/me` và `403 app_access_denied` trả `apps: [{id,name,origin}]`,
  lọc theo grant hiệu lực (quản trị viên: mọi app đã cấu hình). Hai app nên đặt
  cùng `VERSE_APP_LINKS`; cấu hình này không tự cấp thêm quyền hoặc mở rộng CSRF.
  Migration identity `00019` lưu lý do và app thu hồi; `401 session_ended` trả
  `reason` (`logout`, `password_changed`, `admin_changed`, `locked`, `expired`,
  `unknown`) và `app` nếu xác định được. Cookie thiếu/sai bí mật không lộ metadata.
  Khi đăng xuất chung và khi 401 nhận diện được phiên cũ, cookie đã thu hồi được
  giữ nguyên hạn (không gia hạn) để app kia đọc lý do; token không còn quyền vào
  API, lần đăng nhập sau ghi đè. Cookie thiếu/sai vẫn được dọn; file mode giữ cách xoá cũ.
  Đăng xuất trả 204; đổi mật khẩu thu hồi mọi phiên và yêu cầu đăng nhập lại.
  `POST /api/me/sessions/revoke-all` chỉ đăng xuất các thiết bị khác, giữ phiên
  đang dùng; đường quản trị `/api/users/:name/sessions/revoke-all` vẫn thu hồi tất cả.
  `GET /api/setup` có `sharedAccounts` để màn đăng nhập phân biệt Postgres/file.
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
   object từ bản sao media độc lập (hoặc phiên bản S3 nếu provider hỗ trợ)
   trước khi mở traffic. R2 không có S3 bucket versioning để dựa vào.
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
    audit: chạy `ainovel-cli storage purge-audit --days 365 --dry-run` để xem
    count/cutoff UTC, rồi bỏ `--dry-run` khi đúng chính sách. Lệnh thật commit
    từng batch tối đa 1000 row; lỗi có báo số đã commit, có thể chạy tiếp.
    Cần `MIGRATION_DATABASE_URL`; app role không được xoá audit.

## 8. Runbook cutover

Chuyển tài khoản file cũ của Noverse/Viderse sang kho chung (đăng nhập chung
hai app): runbook riêng [sso-cutover.md](sso-cutover.md).

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
| `{env}/staging/{uploadUUID}` | object staging tường minh; không phải nơi chứa mọi multipart đang dở |
| `{env}/backups/pg/YYYY/MM/DD/<ts>.dump` | dump DB (+ `<ts>.blobs.tsv`, `<ts>.schema.txt`, `<ts>.SHA256SUMS`) |

Định danh là UUID, nội dung là SHA-256 đầy đủ (64 hex); đổi tên dự án/truyện
không di chuyển byte nào. Khoá do `internal/blobkey` sở hữu — không gói nào
khác được ghép khoá.

Content addressing vẫn nằm trong môi trường/dự án/truyện: hai job cùng truyện
có thể dùng lại segment, nhưng không có dedupe toàn cục hay xuyên tenant.
Xoá truyện là soft-delete, không phải purge prefix; tham chiếu render/cache/artifact
còn tồn tại vẫn giữ blob. Không xoá prefix thủ công để thay cho GC kiểm tham chiếu.

### 9.2. Quy tắc vòng đời theo provider

**Không đặt Expiration/Delete theo tuổi trên `{env}/`, `audio/`, `export/`
hoặc `assets/`: đó có thể là media đang được tham chiếu.** Phân biệt hai việc:

- Abort multipart chưa hoàn tất: rule **chỉ abort**, filter `{env}/` cho từng
  môi trường đang dùng (`dev/`, `staging/`, `prod/`). Upload multipart dùng cả
  key media cuối cùng, không chỉ `{env}/staging/`.
- Expire object staging đã hoàn tất: rule riêng `{env}/staging/`, ví dụ 1 ngày.
  Chỉ prefix staging dành riêng này được dùng expiry ngắn. Rule retention backup
  là chính sách độc lập, không mở rộng sang media.

**AWS S3:** tạo rule enabled có `Filter.Prefix="{env}/"` và
`AbortIncompleteMultipartUpload.DaysAfterInitiation=1`, **không có `Expiration`**.
Tạo `expire-staging` riêng với `Expiration.Days=1` cho `{env}/staging/`.
Versioning là thiết lập **cấp bucket**, không bật theo prefix; nếu cần Object Lock
cho backup, dùng bucket backup riêng và cấu hình retention/phiên bản theo chính
sách phục hồi. Không dùng expiry media để giảm dung lượng.

**Cloudflare R2:** Settings → Object Lifecycle Rules → Add rule, prefix `{env}/`,
action abort incomplete multipart sau 1 ngày; lặp lại cho từng môi trường.
Giữ rule xoá object staging riêng nếu dùng staging. R2 có rule multipart mặc định
7 ngày, nhưng phải đọc lại cấu hình thực tế sau khi đổi rule, không giả định app
tự cài lifecycle. Xem [R2 lifecycle](https://developers.cloudflare.com/r2/buckets/object-lifecycles/).

Không dựa vào “bật versioning trên R2”: [ma trận S3 API của R2](https://developers.cloudflare.com/r2/api/s3/api/)
liệt kê bucket versioning và Object Lock APIs chưa triển khai. Giữ bản sao media
độc lập ngoài bucket/credential app, có retention theo snapshot/phiên bản và
restore drill. `deploy/backup.sh` xuất dump + manifest key/hash/size; **không sao
chép byte từ bucket S3 nguồn**. Vì vậy phải có lịch backup media riêng, không
sync xoá từ nguồn sang bản sao, và giữ các byte còn được manifest backup tham chiếu.
Nếu chỉ backup DB, một lần GC/xoá bucket vẫn có thể làm bản backup không phục hồi được.

**GCS XML interop:** dùng lifecycle Delete tuổi 1 ngày chỉ với
`matchesPrefix: ["{env}/staging/"]`; không áp cả bucket. Kiểm tra chính sách xử lý
multipart dở của provider riêng, không nhầm completed staging object với multipart.

Lifecycle là lớp bảo vệ khi tiến trình chết; chỉ GC biết tham chiếu trong DB.
Trên bucket thử riêng, ngắt một multipart dưới key media cuối rồi kiểm tra danh
sách multipart/rule abort; giữ một ready object qua cùng cửa sổ để xác nhận nó
không bị expiry. Không thử phá huỷ trên bucket production.

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
`{env}/` cũ hơn grace, rồi multipart chưa hoàn tất dưới `{env}/` quá
`--orphan-age`. `--staged-age` là tuổi hàng DB; `--staging-age` là tuổi object
đã hoàn tất trong prefix staging, không phải tuổi mọi multipart.

- GC kiểm tham chiếu dưới khoá hàng rồi commit claim `state='deleting'`;
  **giữ hàng này trong khi xoá object**, chỉ xoá hàng sau khi storage xác nhận
  thành công. Crash/lỗi storage giữ tombstone để lần sau tiếp tục. Publication
  không hồi sinh hàng `deleting`; helper upload/publish kiểm object lại dưới
  key lock, upload lại nếu GC vừa xoá xong trước khi publish. Các lượt GC thật
  được serialize bằng khoá bảo trì. Xem `internal/storagegc/gc.go` và
  `internal/persistence/postgres/blob_publish.go` cho giao thức chung.
- Scan DB theo keyset/page, xoá S3 tối đa 1000 key/request, giữ lỗi từng key;
  store không hỗ trợ batch dùng Delete tuần tự. Principal S3 của GC cần list/read/
  delete object và list/abort multipart (`s3:ListBucketMultipartUploads`,
  `s3:AbortMultipartUpload` trên AWS); tài khoản cấu hình lifecycle tách khỏi app.
  **Multipart listing cần quyền ở root bucket, không giới hạn Prefix trong
  request**: provider duyệt theo key/upload-id markers rồi lọc `{env}/` tại client
  để tương thích MinIO. Vì vậy lượt scan có đọc metadata upload dở của môi trường
  khác trong cùng bucket, nhưng không trả chúng về GC để abort. Completed-object
  listing vẫn theo prefix môi trường. Nếu cần tách quyền list tuyệt đối, dùng
  bucket riêng cho từng môi trường. Nguồn: `platform/blobstore/s3_batch.go`.
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
- `--dry-run` chỉ báo would-delete/would-abort, không claim hàng hay xoá byte;
  muốn thực sự dọn phải chạy lại **không** có cờ này (lệnh mặc định có mutation).
- Không bao giờ xoá/abort tiền tố môi trường khác; object backup **đã hoàn tất**
  dưới `{env}/backups/` không bị GC xoá. Multipart dở vẫn theo grace toàn
  `{env}/`; key orphan không nhận dạng được chỉ bị báo, không bị xoá.

### 9.4. Thiết lập Cloudflare R2 cho media private

1. R2 object storage → Create bucket, chọn vị trí/jurisdiction phù hợp. Giữ
   bucket private: không bật r2.dev hoặc custom domain public. App dùng HTTPS
   S3 API endpoint, không cần CDN public.
2. R2 → Manage API Tokens → Create Account API token (hoặc User API token).
   Chọn **Object Read & Write**, scope **chỉ bucket media vừa tạo**; lưu Access
   Key ID + Secret Access Key vào Dokploy secret store. Không dùng token
   Admin Read & Write cho app; thay đổi bucket/CORS/lifecycle bằng dashboard
   với tài khoản operator. Xem [R2 authentication](https://developers.cloudflare.com/r2/api/tokens/).
3. Dùng các giá trị khuyến nghị dưới đây (mẫu đầy đủ: `deploy/.env.example`).
   Endpoint phải trùng bucket jurisdiction mà dashboard cung cấp; không thêm
   tên bucket, userinfo, query hoặc fragment. Browser người dùng phải truy cập
   được endpoint này vì byte sẽ đi trực tiếp từ R2, không qua app.

   ```dotenv
   STORE_DRIVER=postgres
   STORAGE_DRIVER=s3
   STORAGE_ENV=prod
   S3_ENDPOINT=https://ACCOUNT_ID.r2.cloudflarestorage.com
   S3_REGION=auto
   S3_BUCKET=noverse
   S3_FORCE_PATH_STYLE=false
   S3_ALLOW_INSECURE_ENDPOINT=false
   BLOB_CACHE_DIR=/var/cache/noverse/ainovel-blob-cache
   BLOB_CACHE_MAX_BYTES=2147483648
   ```

   `S3_ACCESS_KEY_ID`/`S3_SECRET_ACCESS_KEY` lấy từ bước 2, không ghi vào repo.
   AWS S3 dùng endpoint trống + region thật; Garage dùng endpoint nội bộ, region
   `garage`, path style `true` và `BLOB_DELIVERY=proxy` khi trình duyệt không tới được kho.
4. Bucket → Settings → CORS Policy → JSON, lưu policy dưới đây với **origin
   ứng dụng chính xác**, cùng scheme/host/port với `PUBLIC_URL`, không có path
   hay dấu `/` cuối. Không dùng `*`; không cần gửi cookie app sang R2.

   ```json
   [
     {
       "AllowedOrigins": ["https://noverse.example"],
       "AllowedMethods": ["GET", "HEAD"],
       "AllowedHeaders": ["range", "if-range", "if-none-match", "if-modified-since"],
       "ExposeHeaders": ["content-range", "accept-ranges", "etag", "content-length", "content-disposition"],
       "MaxAgeSeconds": 3600
     }
   ]
   ```

   Đây là JSON **dashboard**, không phải schema Wrangler. CORS không cấp
   quyền đọc: presigned URL mới là bearer credential. Xem [R2 CORS](https://developers.cloudflare.com/r2/buckets/cors/).
   Fetch-based download/waveform và mobile Share cần policy này cả khi native
   `<audio>` phát được; kiểm fetch/Share trong browser thật, không chỉ nút Play.
5. Cài lifecycle abort multipart ở mục 9.2 cho **mọi môi trường dùng trong
   bucket**. Lưu/read-back rules; không có expiry trên prefix media sống.
6. Deploy rồi kiểm tra `/readyz`, login bằng tài khoản được phép, phát audio,
   seek và tải export. DevTools phải thấy authorize ở app rồi signed GET tới
   R2; Range trả `206` với Content-Range đúng. Kiểm tra lại với tài khoản không
   có quyền, URL hết hạn, và browser thật (curl thành công không chứng minh CORS).
   URL ký là bí mật tạm thời: không dán vào log, screenshot hay báo cáo.
   Ranh giới validator: SHA ETag của app áp dụng cho local streaming và
   `If-None-Match` → `304` trước khi ký. Response redirect có SHA ETag, nhưng
   representation cuối từ S3 và các lần `If-Range`/`If-None-Match` gửi trực tiếp
   sau đó dùng **ETag của provider S3**. `307` không viết lại request header được
   browser chuyển tiếp; không dùng SHA ETag của app để thay ETag multipart S3.
7. Lên lịch backup DB **và media độc lập**, kiểm restore theo manifest trước
   khi coi R2 là bản lưu chính. Không có cam kết provider smoke từ MinIO:
   cần credential/bucket thử R2 thật, origin HTTPS thật và kiểm anonymous GET
   bị từ chối. Dừng app giữa multipart chỉ trong bucket thử disposable.

### 9.5. Cache, scratch và quan sát

Nguồn cấu hình: `internal/audio/blobcache.go`; `BLOB_CACHE_DIR` mặc định
`$TMPDIR/ainovel-blob-cache` (Docker: `/var/cache/noverse/ainovel-blob-cache`),
`BLOB_CACHE_MAX_BYTES` mặc định 2147483648 (2 GiB, số byte nguyên dương).
Giữ root riêng, writable bởi UID app và không public. Đổi bucket/nguồn storage
trên cùng tenant nên đổi root cache, không chia sẻ root giữa deployment độc lập.
Compose mount `scratch:/var/cache/noverse`; trên Dokploy cần mount tương đương
nếu muốn cache sống qua restart.

Cache xác minh SHA/size, reuse export/peaks trong tenant+driver; tối đa 4 GET
đồng thời, eviction LRU và kiểm free space. Thư mục work có flock để sweep không
xoá job sống. Đây **không phải quota toàn volume**: hardlink còn được job giữ,
PCM decode, merge, cover, output và spool upload vẫn cần dung lượng ngoài budget.
Khởi đầu 2 GiB cache; cấp thêm ít nhất working-set đầu vào + PCM/merge/output
của số job đồng thời, rồi đo free disk ở tải thật. Headroom 64 MiB trong code
không phải bảo đảm đủ disk cho cả truyện. Không xoá work dir đang chạy.

`GET /api/system/storage` dùng session/admin guard sẵn có, trả counters từ khi
process khởi động: op/driver/outcome, count, bytes, tổng/max duration. Label
không chứa key/user; metadata/sign/delete có payload bytes=0, Open tính byte
thực đọc và duration đến Close. Operation quá 1 giây ghi log chậm, kèm SDK request
ID khi error cung cấp; không log URL/key/error string. Đây là app-operation
metrics, **không** đếm SDK retry/part, không đo bandwidth redirect ở R2; xem
provider metrics riêng cho direct GET. Không proxy metrics qua endpoint public.

### 9.6. Cấu hình trong app và chuyển kho không ngừng phục vụ

Đầu mối thực thi: `internal/bootstrap/blob_{settings,router,probe,admin,migration}.go`,
`internal/entry/web/storage_metrics.go` và migration `00016_storage_settings.sql`.
Các endpoint yêu cầu **admin toàn cục Verse** (`users.global_role=admin`), không
phải chủ dự án hoặc app grant; dùng phiên `verse_session` và guard cùng origin.
Port giữ nguyên migration SSO `00012`/`00013`; các migration nguồn
`00012`→`00014`, `00013`→`00015`, `00014`→`00016`, `00015`→`00017`,
`00016`→`00018`. Manifest nhúng và readiness lấy đủ 18 migration.
Các bảng lưu trữ mới không cấp quyền cho `viderse_identity` hoặc `PUBLIC`.

- **Chọn một nguồn cấu hình.** Chỉ cần một biến `STORAGE_DRIVER`, `BLOB_LOCAL_ROOT`,
  `BLOB_DELIVERY` hoặc `S3_*` được khai báo (kể cả rỗng), toàn bộ cấu hình do env
  quản lý và app từ chối lưu với 409. Muốn quản lý trong app, bỏ hẳn các biến đó
  khỏi environment của app và lệnh bảo trì; đừng chỉ đặt chuỗi rỗng. `STORAGE_ENV`
  vẫn là tiền tố triển khai, không được đổi khi chuyển kho.
- Không có env thì dùng cấu hình DB; chưa lưu gì thì dùng local mặc định:
  `/var/lib/noverse/blobs` trong container, thư mục cấu hình người dùng
  `noverse/blobs` trên máy native. Mount thư mục local trước khi ghi dữ liệu.
- Tạo khoá mã hoá bằng `openssl rand -base64 32`, đưa vào secret store dưới tên
  `CONFIG_ENCRYPTION_KEY`; giữ cùng giá trị cho app, import/reconcile/export,
  `storage gc` và `storage purge-audit`. Sao lưu riêng khoá này cùng kế hoạch
  phục hồi DB. Không có khoá thì xem/kiểm tra được nhưng không lưu được credentials.
  Không đổi hoặc bỏ khoá khi DB còn cấu hình đã mã hoá; key id chỉ chuẩn bị cho
  quy trình rotation sau này, chưa phải công cụ tự động đổi khoá.
- Với Garage nội bộ, ưu tiên **proxy**: app tự stream, xử lý Range/ETag/If-Range,
  trình duyệt không cần truy cập kho hoặc CORS. Với **redirect**, đặt
  `presignEndpoint`/`S3_PRESIGN_ENDPOINT` khi địa chỉ public khác endpoint nội bộ;
  hai địa chỉ phải trỏ cùng kho và credentials. Không rewrite host sau khi ký.
- Bước kiểm tra trả kết quả ghi/đọc/đọc đoạn/đồng hồ/ký/xoá và mã lỗi đã dịch.
  Proxy kiểm tra thêm đọc đoạn qua app; redirect trả `corsProbeUrl` sống một phút
  cùng CORS config cho đúng origin app. Đây là URL SigV4 có **đầy đủ access-key ID**
  trong query và chữ ký tạm thời, không phải secret-access-key. Không log hoặc
  lưu cache URL này; cả JSON và NDJSON `/test` đều gửi `Cache-Control: no-store`.
  Browser phải thật sự fetch URL với `Range`; server không thể tự chứng minh CORS
  thay trình duyệt. Object probe bị dọn theo TTL, không phụ thuộc browser fetch;
  khi có khoá mã hoá, lịch dọn nằm trong DB để phục hồi sau restart. Config và
  client chỉ phục vụ probe được thu hồi khi không còn tham chiếu.
- **CORS của Garage phân biệt chữ hoa/thường ở AllowedHeaders.** Luôn dùng các
  tên viết thường như mẫu ở trên; chính sách đó vẫn hợp lệ trên R2 và AWS.
  Test bằng preflight có `Access-Control-Request-Headers: range,if-none-match`.
  Garage không từ chối timestamp tương lai: test so `Date` phản hồi với giờ app
  và từ chối lệch quá năm phút. `capabilities.conditionalPut` cho biết kết quả
  probe thật, không suy ra từ một PUT thành công.

Chuyển một deployment đang dùng env sang quản lý trong app:

1. Giữ nguyên toàn bộ biến kho cũ, đặt `CONFIG_ENCRYPTION_KEY` hợp lệ và
   `MIGRATION_DATABASE_URL` của vai trò bảo trì.
2. Chạy `ainovel-cli storage settings adopt-env`. Lệnh kiểm tra kết nối trước
   khi nhận cấu hình, mã hoá credentials, gắn identity đúng cho blob cũ và
   chỉ in tóm tắt không có khoá. Chạy lại cùng cấu hình không thay DB.
3. Chỉ sau khi lệnh thành công mới bỏ hẳn các biến blob env khỏi app và lệnh
   bảo trì, giữ khoá mã hoá và khởi động lại app. Blob cũ vẫn đọc từ đúng kho.
   Nếu bỏ env trước khi tiếp nhận, startup dừng khi thấy ready blob không phải
   local mà thiếu store config; không tự nhận nhầm dữ liệu đó là local.
4. Sau nâng cấp, chạy `ainovel-cli storage settings backfill-usage --dry-run`,
   rồi bỏ `--dry-run` để phân loại registry cũ bằng parser của `blobkey`.
   Bước này yêu cầu maintenance DSN, phân trang và chạy lại an toàn; app không
   backfill lớn khi startup. Trước backfill, row `unclassified` vẫn nằm trong
   tổng nhưng chưa thuộc sáu nhóm. Avatar/backup đã phân loại không nằm trong
   tổng hay sáu nhóm. Blob mới được phân loại ngay khi publication.

Đổi khoá, signing region, `delivery` hoặc địa chỉ ký trên cùng canonical
endpoint + bucket không phải chuyển dữ liệu: server vẫn test trước, đổi
config/routing tại chỗ và PUT trả `needsMigration=false`. Region dùng để ký,
không định danh kho vật lý (R2 chấp nhận cả `us-east-1` và `auto`).

Runbook đổi kho:

1. Tạo kho private mới, cấp quyền đọc/ghi/xoá và cấu hình CORS nếu dùng redirect.
   Trong mục Lưu trữ tệp, kiểm tra cấu hình đích rồi lưu với `version` từ GET.
   409 do phiên bản cũ nghĩa là tải lại trước khi sửa tiếp, không ghi đè.
2. Kho rỗng đổi ngay, không restart. Có tệp thì PUT trả
   `needsMigration`, `count`, `bytes`: kho cũ vẫn active. Xác nhận số tệp/dung
   lượng rồi POST `/api/system/storage/migration` với `version` mới.
3. Theo dõi trạng thái trong GET. Tệp mới vẫn ghi kho cũ; mọi tệp được chép và
   đọc lại để xác minh SHA-256 + size. Pause/resume/cancel qua các hậu tố
   `/pause`, `/resume`, `/cancel`, mỗi lệnh dùng version hiện tại. Sau lỗi,
   sửa quyền/kết nối rồi resume để thử lại các tệp chưa xác minh.
4. Có thể restart app giữa chừng: job/progress ở DB và tự tiếp tục. Khi hết
   delta, write fence ngắn đợi publication đang chạy kết thúc rồi chuyển config
   và store của các hàng ready trong cùng transaction; không cần maintenance.
5. Huỷ giữ kho cũ, không làm mất tệp. **Không tự xoá kho cũ** sau khi thành công
   hay huỷ; kiểm chứng backup/phát/tải và thời gian lưu giữ trước khi operator
   quyết định dọn độc lập. Không dùng expiry toàn prefix để thay việc đối soát.

Dọn kho nguồn sau khi đã kiểm chứng migration hoàn thành:

```bash
ainovel-cli storage migration purge-source <migration-id> --dry-run=true
# Chỉ sau khi đã xem kỹ số tệp/byte và các từ chối:
ainovel-cli storage migration purge-source <migration-id> --dry-run=false
```

Phải ghi tường minh `--dry-run=true` hoặc `--dry-run=false`, có
`MIGRATION_DATABASE_URL` thật của vai trò bảo trì, khoá mã hoá tương ứng và
`STORAGE_ENV` khớp key. Lệnh chỉ dọn key đã được chép/xác minh trong manifest,
kiểm tra lại bản đích và registry, không list rồi xoá prefix. Nó từ chối kho
đang active/pending, kể cả quay A→B→A hoặc alias có thể trỏ cùng kho. Tiến độ
xoá từng key phục hồi sau crash; key thiếu đích hoặc không an toàn được giữ
lại và lệnh trả lỗi. `migration.completedAt`/`purgedAt` ghi mốc hoàn thành
chuyển/dọn; không chạy lệnh này như GC định kỳ.

Kiểm tra kết nối (`POST /api/system/storage/test`) trả JSON thường, hoặc stream
từng bước một dòng khi gửi `Accept: application/x-ndjson`: proxy phía trước
(Traefik/Cloudflare) không được buffer response này (`X-Accel-Buffering: no` đã
đặt sẵn). JSON, NDJSON và bước test của PUT dùng chung hạn mức **6 lần/phút mỗi
quản trị**; PUT có version cũ trả 409 trước khi gọi kho. Sức khoẻ kho được kiểm
tra nền ngay khi khởi động rồi mỗi 30 giây (timeout 3 giây); giao diện chỉ đọc
kết quả đó. Purge từ chối cả hai kho khác endpoint nhưng trùng bucket bất kể
region hay credentials (có thể là alias của cùng một kho): fail-closed khi không
chứng minh được là hai cụm khác nhau.

S3 transport vẫn cho phép mạng riêng (Garage nội bộ), nhưng chặn link-local
`169.254.0.0/16`, `fe80::/10` và metadata IPv6 `fd00:ec2::254` tại thời điểm
dial theo IP thật sau DNS. Client không đi theo redirect và không dùng HTTP proxy
từ env, tránh chuyển tiếp request đã ký hoặc vòng qua kiểm tra đích.

## 10. Đã kiểm chứng cục bộ / còn lại

Đã chạy (2026-10-29, Docker 29.2.1, compose v5.0.2, arm64):

- `docker build` thành công; runtime `uid=10001`, `ffmpeg 6.1.2`,
  `ffprobe 6.1.2`; không có node/npm/curl trong ảnh.
- Stack compose (Traefik TLS tự ký → app → Postgres/MinIO) healthy; migrate job
  áp dụng manifest lịch sử khi chạy thử 00001–00010
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
- Ghi chú lịch sử MinIO ở lần thử trên không còn áp dụng: compose hiện dùng
  Garage v2.5.0, init nhận secret qua environment, không nội suy shell.

Còn lại (cần môi trường thật, không thể giả lập ở local):

- CIDR Traefik thật của Dokploy + firewall/Tailscale ACL thật; kiểm tra
  443/8081 từ mạng ngoài.
- ACME/Let's Encrypt và domain thật; S3 provider thật (R2/AWS/GCS) + presign.
- Audio end-to-end với Mac/VieNeu thật (queue → play/range → export → restart).
- Backup Dokploy → S3 thật + đo RTO/RPO trên dữ liệu thật.
- Mã hoá at-rest cho file cấu hình model vẫn là phần riêng; `CONFIG_ENCRYPTION_KEY`
  hiện bảo vệ credentials kho tệp trong DB, không tự mã hoá các file model cũ.
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

## 12. File mode + bind mount / Colima trên macOS

Postgres (`STORE_DRIVER=postgres`) lưu thư viện trong database + BlobStore.
Thiết lập web không chọn thư mục; bước **Nơi lưu truyện** chỉ giải thích nơi
lưu rồi cho **Tiếp tục**. Không cần mount thư mục truyện của host vào bản này.

Để dùng thư mục truyện có sẵn trên Mac, chọn **file mode**, dùng
`deploy/docker-compose.file.yml`. Tạo thư mục thử riêng, không trỏ vào dữ liệu
thật khi kiểm tra lần đầu:

```sh
mkdir -p /tmp/noverse-library
export LIBRARY_HOST_ROOT=/tmp/noverse-library
export PUID=\"$(id -u)\" PGID=\"$(id -g)\"
docker compose -p noverse-file -f deploy/docker-compose.file.yml up -d --build
```

Mở `http://<IP-LAN-của-Mac>:8861` từ máy khác. Lấy mã thiết lập ở log app.
Compose chạy app bằng `PUID:PGID` của chủ thư mục; job `config-init` chỉ cấp
quyền volume cấu hình riêng `/home/noverse/.ainovel`, không chown dữ liệu truyện.
App ghi bằng UID/GID này; lớp chia sẻ file của Docker Desktop/Colima có thể
ánh xạ GID khác trên Mac, nên kiểm tra cả chủ sở hữu trên host. Thư mục host
phải tồn tại và cho UID/GID chạy app ghi; không dùng `chmod 777` hoặc chạy
app bằng root để né lỗi.

`LIBRARY_BROWSE_ROOTS=/library` giới hạn bộ duyệt web vào mount này. Chưa có
lựa chọn đã lưu thì root đầu tiên là thư viện mặc định. Muốn thêm root thứ hai,
thêm bind mount `/Users/dat/Truyen:/mnt/truyen` và đổi env thành
`LIBRARY_BROWSE_ROOTS=/library,/mnt/truyen` (đường dẫn **trong container**,
phân cách bằng dấu phẩy). Không mount `/` của host. Bộ duyệt, kiểm tra thư
mục và đổi thư viện từ chối thoát gốc bằng `..` hoặc symlink; cấu hình root
thiếu/sai/không đọc được khiến server dừng thay vì mở rộng phạm vi duyệt.
Root chỉ đọc vẫn có thể hiện nhưng không được chọn làm thư viện ghi.

Image file không khai báo `LIBRARY_BROWSE_ROOTS` dùng volume mặc định
`/var/lib/noverse/library` (`NOVERSE_LIBRARY_ROOT`). Mount volume bền vào đó;
web chỉ hiện thông báo dùng vùng dữ liệu, không bắt duyệt filesystem container.
Bản chạy native không đặt các biến này vẫn giữ bộ chọn thư mục cũ.

### Colima: chia sẻ thư mục Mac vào VM trước

Bind mount của Docker đi qua VM Colima. Colima thường chỉ chia sẻ `$HOME`,
và cấu hình chia sẻ có thể chỉ đọc. Nếu thư mục có trên Mac nhưng không có
trong container, hoặc báo không ghi được, kiểm tra ở VM:

```sh
colima ssh -- ls /Users/dat/NoverseLive
```

Ví dụ bật chia sẻ ghi (thay `/Users/dat` bằng home của bạn):

```sh
colima start --vm-type vz --mount-type virtiofs --mount /Users/dat:w
```

Hoặc sửa `~/.colima/default/colima.yaml`, giữ các cấu hình VM khác:

```yaml
mounts:
  - location: /Users/dat
    writable: true
  - location: /Volumes/Ten-o-ngoai
    writable: true
```

Sau đó `colima restart`. Thư mục ngoài `$HOME`, ví dụ `/Volumes/<ổ ngoài>`,
phải thêm mount riêng. Docker Desktop có mục File Sharing tương ứng; không
cần lệnh Colima khi Docker context đang là `desktop-linux`.

### Origin, cookie và proxy khi setup từ xa

- LAN HTTP trực tiếp: có thể bỏ `PUBLIC_URL`, `SECURE_COOKIES=false`,
  không đặt `TRUSTED_PROXIES`. Mở giao diện và API trên cùng origin LAN.
- HTTPS sau proxy: đặt `PUBLIC_URL=https://<domain>`,
  `SECURE_COOKIES=true`, `TRUSTED_PROXIES` chỉ chứa CIDR thật của proxy;
  proxy chuyển `Host` và `X-Forwarded-Proto: https` đúng public origin.
- Không bật Secure cookie khi trình duyệt vào HTTP: trình duyệt không gửi
  cookie đó, nên setup/login không giữ được phiên. Không tắt origin guard
  hoặc tin mọi proxy để né cấu hình sai; origin khác vẫn bị chặn chống CSRF.

### Không lưu truyện trong lớp ghi tạm của container

Image đặt `NOVERSE_CONTAINER=1`. File mode chỉ cho chọn root thuộc
`LIBRARY_BROWSE_ROOTS`, hoặc `/var/lib/noverse/library` khi không có allowlist;
không cho chọn `/home/noverse/Truyện Noverse`. Cả ba compose mẫu app/TUI đều
khai báo volume/bind thư viện; `docker-compose.yml` vẫn là sandbox CLI,
không phải bản hosted.

Server kiểm tra `/proc/self/mountinfo` khi khởi động và khi kiểm tra/chọn
thư mục. Nếu đường dẫn không có mount bền (overlay và tmpfs không tính),
log và web hiện **“Thư mục này sẽ mất khi cập nhật container — hãy gắn volume.”**
Đừng bỏ qua cảnh báo: dừng app, sao lưu dữ liệu cũ ra host trước, rồi gắn
bind/named volume vào đúng container path. Không xoá/tạo lại container cũ
trước khi sao lưu. `NOVERSE_LIBRARY_ROOT` chỉ định đường dẫn, không tự tạo
volume Docker; luôn kiểm tra mục `Mounts` của container.
