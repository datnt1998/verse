# Viderse (Go)

Khung agent bằng Go trong monorepo `verse` — một Writer agent viết kịch bản
video từ brief. Giao diện web nhúng vào binary, dựng theo kiến trúc của Noverse
(React + Vite, StyleX + Tailwind, kit `@noverse/ui` ở `platform/ui`, hash router,
`SessionProvider`, shell desktop/mobile). Tách biệt hoàn toàn khỏi Noverse: dữ
liệu ở `~/.viderse`, database `viderse_go`, không chia sẻ credential hay lịch sử.

## Yêu cầu

- Postgres cục bộ (ví dụ Homebrew tại `127.0.0.1:5432`) với database `viderse_go`
  và hai role (mirroring `apps/noverse/deploy/postgres-init`):

  ```sql
  CREATE ROLE viderse_migrate WITH LOGIN;
  CREATE ROLE viderse_app WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
  CREATE DATABASE viderse_go OWNER viderse_migrate;
  ```

- Go 1.26+; Node cho phần web (`apps/viderse/web`).

## Biến môi trường

| Biến | Ý nghĩa |
| --- | --- |
| `VIDERSE_DATABASE_URL` | DSN của app (bắt buộc cho `serve`/`migrate`), vd `postgres://viderse_app@127.0.0.1:5432/viderse_go` |
| `VIDERSE_MIGRATION_DATABASE_URL` | DSN role migrate; mặc định dùng chung DSN app |
| `VIDERSE_ADDR` | Địa chỉ lắng nghe, mặc định `127.0.0.1:8789` (`-addr` ghi đè) |
| `TRUSTED_PROXIES` / `PUBLIC_URL` / `SECURE_COOKIES` | Ranh giới trusted-proxy (xem `platform/webguard`) |
| `VIDERSE_CLAUDE_CODE_VERSION` | Ghi đè wire version Claude Code cho anthropic-oauth |

## Lệnh

```bash
go run ./apps/viderse/cmd/viderse serve            # web UI + writer agent
go run ./apps/viderse/cmd/viderse migrate up       # áp schema (goose nhúng)
go run ./apps/viderse/cmd/viderse migrate status
go run ./apps/viderse/cmd/viderse login            # đăng nhập gói thuê bao (mặc định openai-codex)
go run ./apps/viderse/cmd/viderse login --status
go run ./apps/viderse/cmd/viderse logout
```

- Lần `serve` đầu (chưa có tài khoản): terminal in một **mã thiết lập** dùng một
  lần; mở `http://127.0.0.1:8789`, nhập mã, tạo quản trị viên, nhận mã khôi phục.
- Giao diện web hiện mới là **khung**: thiết lập, đăng nhập/đăng xuất, shell
  (trang chủ, tài khoản). Chưa có màn gửi brief hay xem kịch bản; API vẫn đủ:
  `GET/POST /api/runs`, `GET /api/runs/{id}`, `GET /api/scripts`,
  `GET /api/scripts/{id}`. Run chạy nền, kịch bản lưu vào bảng `scripts`, số đo
  run ở `agent_runs` (status/provider/model/token/cost/thời điểm/lỗi).
- Hàng đợi run nằm trong `agent_runs` (database là hàng đợi): `serve` claim
  run queued cũ nhất; deployment giả định **một tiến trình serve mỗi
  database** — lúc khởi động nó đánh `failed` mọi run còn `running` của tiến
  trình đã dừng.
- Model mặc định: `gpt-6-luna` qua provider `openai-codex` (đăng nhập bằng
  `viderse login`). Đổi qua `~/.viderse/config.json` (`provider`/`model`/
  `providers`/`roles`).

## Dữ liệu

| Đường dẫn | Nội dung |
| --- | --- |
| `~/.viderse/auth.json` | credential OAuth (do `viderse login` ghi) |
| `~/.viderse/web-accounts.json` | tài khoản web + phiên (cookie `viderse_session`) |
| `~/.viderse/config.json` | cấu hình provider/model/roles |
| `~/.viderse/models-cache.json` | bảng giá + cửa sổ ngữ cảnh model (OpenRouter, cache 24h) để tính `agent_runs.cost_usd` |
| database `viderse_go` | bảng `agent_runs`, `scripts` |

## Phát triển web

Web là một workspace npm ở gốc repo (cùng `platform/ui` và `apps/noverse/web`):

```bash
# ở gốc repo
npm ci                                   # một lần
npm run dev -w apps/viderse/web          # vite dev server, proxy /api → 127.0.0.1:8789
npm run typecheck -w apps/viderse/web
npm test -w apps/viderse/web
npm run build -w apps/viderse/web        # build ra internal/web/dist (được //go:embed vào binary)
```

Brand Viderse (màu chính `#F43F5E`) và nhãn tiếng Việt nạp ở `web/src/main.tsx`.
Bản build `internal/web/dist` được commit vì `go build` nhúng nó; sửa web xong
phải build lại.
