# Viderse — triển khai Postgres

Viderse cần **hai** cơ sở dữ liệu Postgres:

| DB | Nội dung | Role của Viderse | Biến môi trường |
|----|----------|------------------|----------------|
| Data DB (VD `viderse_db`) | Bảng nghiệp vụ Viderse (migration riêng, `apps/viderse/internal/persistence/migrations`) | `viderse_app` (data), `viderse_migrate` (migration) | `VIDERSE_DATABASE_URL`, `VIDERSE_MIGRATION_DATABASE_URL` |
| Identity DB (VD `noverse_db`) | Kho tài khoản **dùng chung** do Noverse sở hữu (users, sessions, app_grants…) | `viderse_identity` — role đọc-ghi giới hạn bảng tài khoản, không đụng bảng project Noverse | `VERSE_IDENTITY_DATABASE_URL` |

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
