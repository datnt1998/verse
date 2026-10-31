# Viderse — triển khai Postgres

Viderse cần **hai** cơ sở dữ liệu Postgres:

| DB | Nội dung | Role của Viderse | Biến môi trường |
|----|----------|------------------|----------------|
| Data DB (VD `viderse_db`) | Bảng nghiệp vụ Viderse (migration riêng, `apps/viderse/internal/persistence/migrations`) | `viderse_app` (data), `viderse_migrate` (migration) | `VIDERSE_DATABASE_URL`, `VIDERSE_MIGRATION_DATABASE_URL` |
| Identity DB (VD `noverse_db`) | Kho tài khoản **dùng chung** do Noverse sở hữu (users, sessions, app_grants…) | `viderse_identity` — role đọc-ghi giới hạn bảng tài khoản, không đụng bảng project Noverse | `VERSE_IDENTITY_DATABASE_URL` |

## Data DB

1. Tạo hai role và DB (tham chiếu `apps/noverse/deploy/postgres-init/01-roles.sh` cho mô hình role; Viderse dùng cặp `viderse_migrate`/`viderse_app` như `pgtest.RoleSpec`):

   ```sql
   CREATE ROLE viderse_migrate LOGIN PASSWORD '<mật-khẩu>';
   CREATE ROLE viderse_app NOLOGIN;
   CREATE DATABASE viderse_db OWNER viderse_migrate;
   GRANT viderse_app TO viderse_migrate;
   -- viderse_app có thể được GRANT cho một login vận hành nếu muốn:
   -- CREATE ROLE viderse_app_login LOGIN PASSWORD '...' IN ROLE viderse_app;
   ```

2. Migration: `viderse migrate up` với `VIDERSE_MIGRATION_DATABASE_URL` (mặc định dùng `VIDERSE_DATABASE_URL` nếu không đặt — nên đặt role migrate riêng). Migration tự `GRANT ... TO viderse_app`, `SET ROLE` phải nằm trong DSN (vd `?role=viderse_migrate`) hoặc session của role migrate.

3. `VIDERSE_DATABASE_URL` trỏ `viderse_app` vào schema nghiệp vụ (vd `postgres://viderse_app@host:5432/viderse_db?search_path=public&role=viderse_app`).

## Identity DB (kho tài khoản chung)

`VERSE_IDENTITY_DATABASE_URL` là **bắt buộc** cho `viderse serve` và
`viderse user grant|revoke` — không có fallback: file `~/.viderse/web-accounts.json`
không còn tồn tại nữa.

1. DB identity do Noverse sở hữu: đã migrate bằng `ainovel-cli storage migrate up` (schema trong `apps/noverse/internal/persistence/migrations`).

2. Cấp role `viderse_identity` bằng script có sẵn trong repo Noverse:

   ```sh
   psql "$NOVERSE_DB_ADMIN_DSN" -v ON_ERROR_STOP=1 \
     -v "identity_password=<mật-khẩu-role>" \
     -v "identity_schema=<schema-identity>" \
     -f apps/noverse/deploy/postgres-init/02-identity-role.sql
   ```

   Script idempotent: chạy lại chỉ cập nhật mật khẩu và grants.

3. `VERSE_IDENTITY_DATABASE_URL=postgres://viderse_identity:<mật-khẩu>@host:5432/<db-identity>?search_path=<schema-identity>`

   - Role chỉ có account/grant; mọi đọc-ghi `project_members` hay quyền admin
     tài khoản trả `501 unsupported`/`42501` — không dùng được cho luồng operator
     Noverse. Mặt bằng đúng chuẩn.
   - Đổi/grant app cho người dùng chạy bằng `ainovel-cli serve user grant` hoặc
     `viderse user grant` — cùng hàng grant trong `app_grants`, ai cũng đọc được.
   - Cắt quyền khẩn cấp: `REVOKE viderse_identity FROM <login>` hoặc đổi mật
     khẩu role — revoke phiên người dùng cụ thể dùng `user revoke --app` /
     trang quản trị, vì grant mutation tự thu hồi phiên mục tiêu.

## Cookie và origin

Cả hai app đọc **một** cookie `verse_session`:

- Local dev (không `PUBLIC_URL`): cookie `Host`-only (`SameSite=Lax`); login trên
  một cổng loopback mở được cổng kia của cùng user. Khi đó `VERSE_APP_ORIGINS`
  (local SSO) bắt buộc bind loopback — Viderse từ chối `0.0.0.0`/`::`/LAN.
- Hosted (`PUBLIC_URL=https://...` + `SECURE_COOKIES=1`): đặt
  `VERSE_COOKIE_DOMAIN=.verse.example` để cookie chia sẻ giữa hai subdomain, và
  `VERSE_APP_ORIGINS` = danh sách origin allowlist chính xác của cả hai app.
- `TRUSTED_PROXIES` quyết định tin `X-Forwarded-For`/`Proto`: chỉ đặt CIDR của
  proxy thật; để trống khi chạy trực tiếp.

## Lệnh liên quan

```sh
viderse migrate up|status|version            # migration DB nghiệp vụ
viderse user grant  <tên> --app viderse      # cấp quyền dùng Viderse (kho chung)
viderse user revoke <tên> --app viderse      # thu hồi (thu hồi luôn phiên)
viderse serve [-addr HOST:PORT]              # cần cả ba DSN
```
