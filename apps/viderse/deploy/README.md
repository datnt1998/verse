# Viderse — triển khai Postgres

Viderse cần **hai** cơ sở dữ liệu Postgres:

| DB | Nội dung | Role của Viderse | Biến môi trường |
|----|----------|------------------|----------------|
| Data DB (VD `viderse_db`) | Bảng nghiệp vụ Viderse (migration riêng, `apps/viderse/internal/persistence/migrations`) | `viderse_app` (data), `viderse_migrate` (migration) | `VIDERSE_DATABASE_URL`, `VIDERSE_MIGRATION_DATABASE_URL` |
| Identity DB (VD `noverse_db`) | Kho tài khoản **dùng chung** do Noverse sở hữu (users, sessions, app_grants…) | `viderse_identity` — role đọc-ghi giới hạn bảng tài khoản, không đụng bảng project Noverse | `VERSE_IDENTITY_DATABASE_URL` |

## Tuyến live trên Mac mini

`https://vid.aarverse.studio` chạy bằng launchd `com.dat.viderse`, bind
`127.0.0.1:8789`. Tuyến triển khai hiện hành là
`~/server/bin/build-deploy-viderse.sh`: lấy `origin/main` tại
`~/server/src/verse`, dựng binary macOS arm64 bằng Docker, migrate rồi restart.
Binary đang chạy là tệp thường `~/server/apps/viderse/bin/viderse`; commit nằm
ở `viderse.commit` cùng thư mục. Giữ đường dẫn binary ổn định để không xin lại
quyền riêng tư macOS mỗi lần phát hành.

Script kiểm `/readyz` trước khi nhận bản mới và tự trả về bản trước nếu lỗi.
`/healthz` kiểm tiến trình; kiểm thêm origin công khai và màn vừa thay đổi sau
triển khai. Không chạy đồng thời script này và `build-deploy.sh` của Noverse:
hai tuyến dùng chung checkout nguồn. Secret chỉ đọc từ cấu hình runtime hiện
có; không truyền token hoặc DSN trên đối số hay chép vào tài liệu.

Nếu fetch HTTPS thất bại vì credential, dùng fallback SSH agent tạm theo
[tuyến Mac mini dùng chung](../../noverse/deploy/README.md#tuyến-vận-hành-mac-mini-hiện-dùng),
vẫn gọi `build-deploy-viderse.sh`: forward đúng key có TTL, ép URL HTTPS→SSH
chỉ bằng env của lệnh, xác minh GitHub host key, không chép private key hoặc
đổi global Git config. Dừng agent đã tạo và kiểm `viderse.commit`/health/ready
sau rollout.

### Viderse không lên dù PostgreSQL trong Colima vẫn khỏe

Dành cho người vận hành Mac mini đã có quyền SSH, trong phạm vi phục hồi dịch vụ
hiện hành; không đồng nghĩa được deploy, đổi secret hoặc restart toàn bộ VM.
Log ứng dụng: `~/server/logs/viderse.log`; trạng thái:
`launchctl print gui/$(id -u)/com.dat.viderse`.

Nếu log báo `127.0.0.1:5433 connection refused`, đối chiếu
`docker exec noverse-postgres-1 pg_isready -h 127.0.0.1 -p 5432` với
`nc -z -w 3 127.0.0.1 5433` trên macOS. Docker CLI cần PATH có
`/opt/homebrew/bin`. Container khỏe nhưng host không mở cổng có thể là forwarding,
không phải mất dữ liệu. Nguồn transport là cấu hình sinh bởi Lima:
`~/.colima/_lima/colima/ssh.config`; log là `ha.stderr.log` cùng thư mục.

Chỉ khi đã xác nhận đúng VM, đúng cấu hình và không có listener cạnh tranh,
kiểm SSH master bằng
`ssh -F "$HOME/.colima/_lima/colima/ssh.config" -O check lima-colima`,
rồi nối lại cổng qua **master hiện có**, không tạo tunnel nền riêng:

```sh
ssh -F "$HOME/.colima/_lima/colima/ssh.config" -O forward \
  -L 127.0.0.1:5433:127.0.0.1:5433 lima-colima
ssh -F "$HOME/.colima/_lima/colima/ssh.config" -O forward \
  -L 127.0.0.1:3900:127.0.0.1:3900 lima-colima
```

Cổng3900 là Garage; kiểm cả hai bằng `nc` trước khi
`launchctl kickstart gui/$(id -u)/com.dat.viderse`. Sau đó kiểm `/readyz`
ở loopback và origin công khai, cùng màn đăng nhập/assets thực. HTTP403 từ Garage
không kèm chữ ký chỉ chứng minh endpoint tới được, không chứng minh đọc object
có xác thực. Nếu master/cấu hình không còn hợp lệ, dừng và chẩn đoán transport;
không tự sửa file sinh, đổi DSN hoặc restart database/VM để thử.
Nối lại cổng phục hồi đường truy cập, không phải bằng chứng đã sửa nguyên nhân
nội bộ khiến Lima trước đó báo SSH forwarding exit255.

### Trang đo/xuất không nhận việc trên hosted

Chrome headless và preflight workspace giữ HTTP loopback theo listener thực.
PCM/video là dữ liệu lớn; vòng qua HTTPS/Cloudflare có thể làm request đã claim
hết hạn dù heartbeat và byte count vẫn tăng. Không đổi launcher sang public origin
chỉ để qua claim hoặc tăng timeout khi chưa đo nguyên nhân nghẽn.

Ngoại lệ origin nội bộ chỉ nằm ở nhánh request-scoped `/api/render/` và
`/api/measure/`; nguồn chính sách là [`render.go`](../internal/web/render.go),
nguồn launcher là [`render_wiring.go`](../cmd/viderse/render_wiring.go).
Chỉ nhận authority loopback đã bind, peer trực tiếp và không có dấu hiệu proxy.
Token/nonce/fence/binding/project scope vẫn bắt buộc; không mở allowlist PUBLIC_URL,
SSO hoặc API editor/tài khoản. Nếu request còn queued, xem lỗi origin trước retry;
nếu đã upload nhưng chậm, đo đường truyền và storage thay vì chạy lại bước AI.

Khi hotfix backend trong lúc UI mới chưa nghiệm thu, không chạy script lấy toàn bộ
`origin/main`: dựng từ đúng revision production với patch tối thiểu, giữ nguyên
assets đang phục vụ và kiểm SHA. Giữ binary/marker trước đó để rollback, kiểm
writer trước thay tệp tại đường dẫn ổn định. Nếu không đổi schema thì không chạy
migration. Sau rollout phải chứng minh request thật nhận claim, upload và hoàn tất
đo qua checkpoint, không chỉ `/readyz`. Retry dùng thao tác repository/API hiện
có và giữ nguyên checkpoint các bước AI đã thành công.

Không xem SHA của file đang cài là bằng chứng process đang dùng file đó. Với
launchd tự khởi động lại, SIGTERM trước khi thay file có thể mở lại inode cũ.
Thứ tự hotfix không đổi schema: **cài → `launchctl kickstart -k` → kiểm inode
process → health → chạy tiếp việc đủ điều kiện**. Lấy PID từ
`launchctl list com.dat.viderse`, đối chiếu dòng executable `txt` của
`lsof -nP -a -p <PID> -d txt` với
`stat -f 'device=%d inode=%i' ~/server/apps/viderse/bin/viderse`.
Nếu cần dừng writer hoàn toàn, phải ngăn auto-restart trước khi thay tệp;
không coi SIGTERM hoặc khoảng trống listener ngắn là đã quiesce.




## Data DB

1. Tạo hai role và DB (tham chiếu `apps/noverse/deploy/postgres-init/01-roles.sh` cho mô hình role; Viderse dùng cặp `viderse_migrate`/`viderse_app` như `pgtest.RoleSpec`):

   Trong psql (không dán mật khẩu vào câu SQL):

   ```text
   CREATE ROLE viderse_migrate LOGIN;
   \password viderse_migrate
   CREATE ROLE viderse_app NOLOGIN;
   CREATE DATABASE viderse_db OWNER viderse_migrate;
   GRANT viderse_app TO viderse_migrate;
   -- Optional operational login:
   -- CREATE ROLE viderse_app_login LOGIN IN ROLE viderse_app;
   -- \password viderse_app_login
   ```

   `\password` hỏi mật khẩu kín và chỉ gửi verifier SCRAM tới server; secret
   không nằm trong shell history hoặc SQL plaintext.

2. Migration: `viderse migrate up` với `VIDERSE_MIGRATION_DATABASE_URL` (mặc định dùng `VIDERSE_DATABASE_URL` nếu không đặt — nên đặt role migrate riêng). Migration tự `GRANT ... TO viderse_app`, `SET ROLE` phải nằm trong DSN (vd `?role=viderse_migrate`) hoặc session của role migrate.

3. `VIDERSE_DATABASE_URL` trỏ `viderse_app` vào schema nghiệp vụ (vd `postgres://viderse_app@host:5432/viderse_db?search_path=public&role=viderse_app`).

## Identity DB (kho tài khoản chung)

`VERSE_IDENTITY_DATABASE_URL` là **bắt buộc** cho `viderse serve` và
`viderse user grant|revoke` — không có fallback: file `~/.viderse/web-accounts.json`
không còn tồn tại nữa.

1. DB identity do Noverse sở hữu: đã migrate bằng `ainovel-cli storage migrate up` (schema trong `apps/noverse/internal/persistence/migrations`).

2. Cấp role `viderse_identity` bằng script có sẵn trong repo Noverse:

   ```sh
   # File 0600 chứa IDENTITY_ROLE_PASSWORD=…; DSN quản trị không kèm mật khẩu
   # (PGPASSFILE), để không secret nào nằm trên dòng lệnh hay shell history.
   ( set -a; . /secure/identity-role.env
     exec psql -X "$NOVERSE_DB_ADMIN_DSN" -v ON_ERROR_STOP=1 \
       -v "identity_schema=<schema-identity>" \
       -f apps/noverse/deploy/postgres-init/02-identity-role.sql )
   ```

   Cần psql 15+ (`\getenv`) và **superuser khi tạo login chưa có**. Script
   tắt `log_transaction_sample_rate` ở mức session **trước BEGIN**, rồi tắt
   statement/duration/sample/debug logging, đặt `pg_stat_statements.track=none`
   và `pg_stat_statements.track_utility=off`, tắt `auto_explain`, `pgaudit` và
   `track_activities` transaction-local trong transaction CREATE ROLE; mật khẩu
   không xuất hiện trong `pg_stat_activity.query`. Automation cũng truyền mật
   khẩu qua env `IDENTITY_ROLE_PASSWORD`, không qua đối số `-v`.
   Không bật/nạp hai extension ghi log đó trong phiên này. Script kiểm tra
   lại GUC trên backend của transaction: transaction pooler chọn backend còn
   sampling thì bị rollback. Nếu không thể tắt log, script dừng **trước**
   khi gửi bất kỳ câu lệnh chứa mật khẩu nào, bất kể cờ `ON_ERROR_STOP` ban đầu.
   Mật khẩu rỗng bị từ chối khi tạo login.

   Superuser có thể tạo role không mật khẩu, đặt bằng
   `\password viderse_identity`, rồi giao ADMIN OPTION trên role cho principal
   quản trị schema. Khi login đã có, chủ database/schema/bảng có CREATEROLE
   và ADMIN OPTION có thể chạy lại script không cần env mật khẩu: script chỉ
   đồng bộ thuộc tính/grants, không đọc/gửi mật khẩu hoặc đổi GUC ghi log.
   Đổi mật khẩu login đã có bằng `\password viderse_identity`.

3. `VERSE_IDENTITY_DATABASE_URL=postgres://viderse_identity:<mật-khẩu>@host:5432/<db-identity>?search_path=<schema-identity>`

   - Role chỉ chạm bảng tài khoản, phiên, grant và ghi thêm audit; không đọc
     `project_members`, dự án hay lịch sử audit, không đổi vai trò, khoá hay
     xoá tài khoản (Postgres trả `42501`). Role chỉ tạo được tài khoản khi kho
     còn trống (thiết lập lần đầu); sau đó trigger của Noverse chặn mọi lệnh
     tạo tài khoản từ role này. Không dùng DSN này cho lệnh quản trị Noverse.
   - Rủi ro còn lại có chủ ý: Viderse phải ghi mật khẩu, phiên, mã khôi phục
     và `app_grants` (đổi mật khẩu, thiết lập, `viderse user grant`), nên một
     tiến trình Viderse bị chiếm có thể chiếm tài khoản sẵn có. Giữ Viderse
     cùng mức tin cậy với Noverse: cùng máy hoặc mạng riêng, DSN chỉ trong env
     của `viderse serve`.
   - Cấp/thu quyền app chạy bằng `ainovel-cli serve user grant|revoke` hoặc
     `viderse user grant|revoke`: cùng hàng trong `app_grants`, đổi thật thì
     người đó bị đăng xuất khỏi mọi thiết bị ở cả hai app.
   - Cắt Viderse khỏi kho tài khoản khi khẩn cấp: `ALTER ROLE viderse_identity
     NOLOGIN` rồi `pg_terminate_backend` các kết nối của role đó. Noverse không
     dùng role này nên vẫn chạy.

## Cookie và origin

Cả hai app đọc **một** cookie `verse_session`:

- Local dev (không `PUBLIC_URL`): cookie host-only (`HttpOnly; SameSite=Strict`); login trên
  một cổng loopback mở được cổng kia của cùng user. Khi đó `VERSE_APP_ORIGINS`
  (local SSO) bắt buộc bind loopback — Viderse từ chối `0.0.0.0`/`::`/LAN.
- Hosted (`PUBLIC_URL=https://...` + `SECURE_COOKIES=1`): đặt
  `VERSE_COOKIE_DOMAIN=.verse.example` để cookie chia sẻ giữa hai subdomain, và
  `VERSE_APP_ORIGINS` = danh sách origin allowlist chính xác của cả hai app.
- `TRUSTED_PROXIES` quyết định tin `X-Forwarded-For`/`Proto`: chỉ đặt CIDR của
  proxy thật; để trống khi chạy trực tiếp.
- Chuyển app: đặt cùng
  `VERSE_APP_LINKS=noverse=https://nov.example.com,viderse=https://vid.example.com`
  ở cả hai app. Chỉ hai id này, không trùng; origin phải là `PUBLIC_URL` hoặc
  thuộc `VERSE_APP_ORIGINS` (cùng quy tắc HTTPS/cookie domain). Cấu hình sai làm
  khởi động thất bại; không đặt thì chỉ có app hiện tại, không hiện bộ chuyển app.
  `GET /api/me` và `403 app_access_denied` trả `apps: [{id,name,origin}]` đã lọc
  theo grant; quản trị viên nhận mọi app đã cấu hình.
- Áp migration identity `00019` trước binary mới: phiên bị thu hồi giữ
  `revoked_reason`/`revoked_app`. Cả hai app trả `401 {code:"session_ended",error,reason,app?}`;
  chỉ cookie có bí mật đúng mới đọc được lý do. Cookie thiếu/sai nhận `unknown`,
  không có `app` hoặc thông tin người dùng; lỗi DB vẫn 503, giữ cookie.
  Đăng xuất chung và 401 nhận diện được phiên cũ không xoá/gia hạn cookie đã
  thu hồi: chỉ giữ đến hạn cũ để app kia đọc lý do. Lần đăng nhập sau ghi đè.
- Đăng xuất trả 204; đổi mật khẩu thu hồi mọi phiên. Thiết bị › đăng xuất các
  thiết bị khác giữ phiên đang dùng (`POST /api/me/sessions/revoke-all`).

## Lệnh liên quan

```sh
viderse migrate up|status|version            # migration DB nghiệp vụ
viderse user grant  <tên> --app viderse      # cấp quyền dùng Viderse (kho chung)
viderse user revoke <tên> --app viderse      # thu hồi (thu hồi luôn phiên)
viderse serve [-addr HOST:PORT]              # cần cả ba DSN
```

## Kho lưu trữ máy chủ (V25, mốc A)

Viderse dùng `platform/blobstore` và `platform/blobhttp`, cùng hợp đồng env với
Noverse. **Mỗi app một bucket và một cặp khoá runtime riêng.** Không dùng khoá
admin Garage trong tiến trình app. Khoá runtime cần đọc, ghi, liệt kê và xoá
object trong đúng bucket của app; thử đọc bucket Noverse bằng khoá Viderse phải
bị từ chối. Sao lưu Postgres, bucket và `secret.key` cùng nhau.

| Biến | Giá trị |
| --- | --- |
| `STORAGE_DRIVER` | `local` (mặc định) hoặc `s3`; DTO giao diện dùng `kind:"disk"`, không phải alias env |
| `S3_PROVIDER` | `garage`, `r2`, `aws`, `gcs`, `wasabi`, `b2`, `spaces`, `other`; giao diện gọi Spaces là `do` |
| `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET` | Endpoint gốc không có path/query/credential; Garage dùng vùng `garage` |
| `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | Cặp khoá runtime, không truyền qua đối số CLI hoặc ghi vào log |
| `S3_FORCE_PATH_STYLE` | Boolean nghiêm ngặt; Garage dùng `true` |
| `S3_ALLOW_INSECURE_ENDPOINT` | Chỉ đặt `true` khi chủ động dùng HTTP nội bộ tin cậy |
| `BLOB_DELIVERY` | `proxy` (mặc định Viderse) hoặc `redirect`; cả hai xác thực tài nguyên trước I/O |
| `S3_PRESIGN_ENDPOINT` | Endpoint trình duyệt truy cập được, chỉ phục vụ URL ký; không thay endpoint nội bộ |
| `BLOB_CACHE_DIR`, `BLOB_CACHE_MAX_BYTES` | Thư mục tuyệt đối và ngân sách bộ đệm; mặc định `<config>/cache/blobstore`, 2 GiB |

Đặt `STORAGE_DRIVER` hoặc bất kỳ tên biến `S3_*` khoá cấu hình trên giao diện,
kể cả giá trị rỗng. Không đặt các biến này khi muốn lưu cấu hình từ Cài đặt.
`BLOB_DELIVERY` và cache riêng lẻ không khoá form. Không có parser `disk|s3`
hay biến có tiền tố Viderse cho transport.

### Garage và kiểm tra trình duyệt

Tạo bucket bằng Garage admin API trên cổng **3903**, không dùng S3 CreateBucket.
Quy trình admin là `POST /v2/CreateKey`, `POST /v2/CreateBucket`, rồi
`POST /v2/AllowBucketKey` cho đúng key/bucket. Lưu phản hồi chứa khoá vào kho bí
mật, không vào shell history hoặc log. Cổng S3 là 3900. Garage có thể bỏ qua
`If-None-Match:*`; transport tự probe rồi dùng staging và full readback SHA-256,
không coi ETag hay multipart ETag là checksum.

Ưu tiên proxy nếu chưa có endpoint công khai. Redirect cần CORS của bucket với
origin chính xác của Viderse, methods `GET`, `HEAD`, allowed headers viết thường
`range`, `content-type`, expose headers `content-range`, `content-length`,
`accept-ranges`, `etag`. Không dùng wildcard origin kèm credentials. Địa chỉ
`http://garage:3900` có thể đọc được từ server nhưng không từ trình duyệt.
Lượt kiểm tra redirect chỉ thành công sau khi trình duyệt đọc URL ký bằng
`Range: bytes=0-1023`, nhận 206 và đủ 1024 byte, rồi server xoá object probe.
Trình duyệt không phân biệt chắc chắn lỗi mạng với CORS qua `fetch`; lỗi
`browser_unreachable` cần kiểm tra cả endpoint công khai và CORS.

### Chuyển trực tuyến, import và khôi phục

Apply migration cộng thêm trước khi chạy binary mới. Lần đầu chạy local đăng ký
các đường dẫn thật của lớp phủ, video nền và ảnh kênh vào manifest. Import kiểm
tra toàn bộ cây trước khi ghi manifest: tệp pipeline (ảnh đã vẽ/editor/nguồn/bản
xuất), layout lạ hoặc symlink bị từ chối kèm đường dẫn, không bị bỏ qua. Không
quét thư mục cấu hình để đẩy secret, cookie, model, process registry hay scratch
của lần vẽ ảnh đã trả tiền lên kho.

Lưu cấu hình S3 mới không đổi kho đọc hiện tại. Bấm chuyển để chép và xác minh
từng object trong khi video vẫn chạy. Pause/resume/cancel/retry lưu trong
Postgres. Cancel giữ cả object đã chép và nguồn; lượt sau xác minh rồi dùng lại,
không PUT lại. Cutover dùng fence xuyên tiến trình và kiểm tra lại toàn bộ
tham chiếu hiện hành, không chờ network trong đoạn đổi con trỏ. Không có nút xoá
kho cũ. CLI và web dùng cùng manifest, generation và fence.

```sh
# Mặc định mọi lệnh bảo trì chỉ xem trước.
viderse storage import --dry-run
viderse storage import --apply
viderse storage gc --dry-run
viderse storage gc --apply --grace 24h
viderse storage materialize --to /srv/viderse-rollback --dry-run
viderse storage materialize --to /srv/viderse-rollback --apply
```

GC chỉ xét codec `v1/sha256`, giữ thời gian chờ sau khi mất ref, kho cũ, lượt
chuyển chưa kết thúc, URL ký chưa hết hạn và các read/publish lease. Apply khoá
rồi kiểm tra ref lại; preview không phải giấy phép xoá về sau. Dọn trên web giữ
nguyên các route §23, tính `orphan_objects` theo manifest và `work_cache` chỉ cho
admin; run dùng đúng preview đã xác thực, giữ cache đang pin. Chế độ local giữ
hành vi V22; S3 không gọi scanner/reclaimer pipeline local.

Trước rollback sang binary chỉ biết local, **dừng writer**, materialize manifest
hiện tại (kể cả tệp mới sau cutover), kiểm tra SHA-256, cấu hình đường dẫn local
đó rồi mới chạy binary cũ. Không chỉ đổi `STORAGE_DRIVER=local` và giả định thư
mục nguồn cũ còn đầy đủ. Materialize không tự đổi backend, không ghi đè tệp khác
nội dung, không sửa schema. Migration down từ chối xoá manifest/lịch sử đã có dữ
liệu. Giữ DB, bucket và key để có thể roll forward an toàn.

### Khoá, cache và giới hạn mốc A

`secret.key` phải là tệp thường 32 byte, thuộc user chạy app, quyền 0600. API trả
`encryptionKey={ok,path,issue?}` với `readonly_dir`, `bad_permissions`, `corrupt`.
Không chmod, thay thế hay tái tạo khoá đã tồn tại sai. Khi mất khoá mà còn
ciphertext, không tạo khoá mới để che lỗi; khôi phục đúng bản backup. Được thử
cặp khoá mới khi khoá máy chưa dùng được, nhưng không được lưu cho tới khi cả
descriptor hiện tại và các descriptor giữ lại đều giải mã được.

Cache không phải nguồn dữ liệu. Có thể xoá cache khi đã dừng server; lần đọc sau
sẽ hydrate, kiểm tra SHA-256 và publish nguyên tử. Pin sống tới hết HTTP/CLI/job;
cache lỗi/hỏng sau restart không được tin cậy. Input vượt ngân sách dùng workspace
có owner lock thay vì ép vào cache LRU. Kho mất kết nối bật hold độc lập với pause
của người dùng, báo `storageDown`, thông báo admin và tự thử lại mỗi phút.

Mốc A chỉ có lớp phủ, video nền và ảnh kênh. Các nhóm ảnh đã vẽ, project editor,
video nguồn và MP4 trả số lượng/dung lượng `null`, không phải 0. **Chưa có bằng
chứng render/editor end-to-end trên S3**; phần này bị chặn tới mốc B.

