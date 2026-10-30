# Viderse (Go)

Viderse vận hành nhiều kênh YouTube: theo dõi kênh đối thủ, chọn video, rồi
làm lại video qua chuỗi 9 bước máy do 4 trợ lý đảm nhận (Trinh sát, Đạo diễn,
Hoạ sĩ, Kiểm định). App đang được dựng lại từ yt-factory theo
[plan dựng lại](../../plans/261029-2242-viderse-rebuild-from-yt-factory/plan.md);
bảng phase trong plan cho biết phần nào đã chạy.

Một binary `viderse` gồm máy chủ web (Go + Postgres) và giao diện React nhúng
sẵn, dựng trên kit `@noverse/ui` ở `platform/ui`. Tách biệt hoàn toàn khỏi
Noverse: dữ liệu ở `~/.viderse`, database `viderse_go`, cookie
`viderse_session`, không chia sẻ credential hay lịch sử.

## Thiết kế

Toàn bộ thiết kế UI/UX (logic sản phẩm, tokens, kit, copy, hợp đồng dữ liệu, gói việc, brand, artboard) nằm ở
[`docs/design/v1/`](docs/design/v1/README.md). Đó là nguồn sự thật cho hành vi và giao diện; mọi cập nhật thiết kế
và hỏi đáp với design ghi ở [`docs/design/v1/11-hoi-dap.md`](docs/design/v1/11-hoi-dap.md).

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
go run ./apps/viderse/cmd/viderse serve            # máy chủ web + giao diện nhúng
go run ./apps/viderse/cmd/viderse migrate up       # áp schema (goose nhúng)
go run ./apps/viderse/cmd/viderse migrate status
go run ./apps/viderse/cmd/viderse login            # đăng nhập gói thuê bao (mặc định openai-codex)
go run ./apps/viderse/cmd/viderse login --status
go run ./apps/viderse/cmd/viderse logout
```

- Lần `serve` đầu (chưa có tài khoản): terminal in một **mã thiết lập** dùng một
  lần; mở `http://127.0.0.1:8789`, nhập mã, tạo quản trị viên, nhận mã khôi phục.
- Mọi `/api/*` cần phiên đăng nhập. Hai đường dùng chung cho mọi màn:
  - `GET /api/status`: số video đang chạy, chờ lượt, chờ duyệt, cờ tạm dừng
    hàng chờ và các mục Cần xử lý (hợp đồng dữ liệu §5). Có `ETag`; gửi lại
    `If-None-Match` khớp thì nhận 304.
  - `GET /api/events`: luồng sự kiện SSE (`hello`, `reset`, rồi các sự kiện có
    `id`), nối lại bằng `Last-Event-ID`. Giao diện làm mới dữ liệu theo sự kiện
    và chỉ poll `/api/status` mỗi 3 giây khi mất luồng này.

## Dữ liệu

| Đường dẫn | Nội dung |
| --- | --- |
| `~/.viderse/auth.json` | credential OAuth (do `viderse login` ghi) |
| `~/.viderse/web-accounts.json` | tài khoản web + phiên (cookie `viderse_session`) |
| database `viderse_go` | `settings`, `background_tasks`, `notifications`; các bảng nghiệp vụ thêm theo từng phần |

## Cấu trúc mã

Mục tiêu: đổi một nhà cung cấp, một bước pipeline hay một màn mà không phải sửa
lan sang chỗ khác.

- `cmd/viderse`: lệnh CLI và nơi ráp mọi phần (constructor nhận phụ thuộc,
  không biến toàn cục).
- `internal/<mảng>`: mỗi gói là một mảng nghiệp vụ (`status`, `tools`,
  `youtube`, …). `internal/domain` chỉ chứa kiểu giá trị và mã lỗi dùng chung.
  Chiều phụ thuộc: `cmd` → `web`, `persistence` → gói nghiệp vụ → `domain`;
  gói nghiệp vụ không import `web`, `persistence`, `bootstrap` hay `cmd`
  (`archtest` › `TestLayeredAppImports` chặn ở mã chạy thật).
- Interface đặt ở phía dùng và nhỏ nhất có thể; hệ ngoài (binary, HTTP, model,
  YouTube) luôn nằm sau interface để thay bản khác hoặc test bằng bản giả.
- `internal/persistence`: migration goose (`migrations/`) và store Postgres
  (`postgres/`, truy vấn mỗi mảng một tệp trong `queries/`, sinh bằng sqlc).
  Gói nghiệp vụ dùng store qua interface.
- `internal/web`: route `/api/*` mỏng (đọc yêu cầu → gọi service → ghi JSON),
  kiểu wire tách khỏi kiểu domain.
- `web/src`: `api/` (client có kiểu), `data/` (hook dữ liệu, bảng sự kiện →
  khoá cache), `screens/<route>/` (mỗi route một chunk lazy), `components/`,
  `shell/`, `i18n/vi.ts` (mọi câu chữ), `styles/`. Thành phần dùng chung nằm
  trong kit `@noverse/ui`; app không viết lại thứ kit đã có.

## Phát triển web

Web là một workspace npm ở gốc repo (cùng `platform/ui` và `apps/noverse/web`):

```bash
# ở gốc repo
npm ci                                   # một lần
npm run dev -w apps/viderse/web          # vite dev server, proxy /api → 127.0.0.1:8789
npm run typecheck -w apps/viderse/web
npm test -w apps/viderse/web
npm run build -w apps/viderse/web        # build ra internal/web/dist (được //go:embed vào binary)
npm run size -w apps/viderse/web         # kích thước gzip JS tải lần đầu, lỗi khi vượt 250 kB
```

Brand Viderse (màu chính `#FF4D63`, bộ màu ở `docs/design/v1/02-stack-verse.md`
§2) nạp ở `web/src/brand.ts`. Bản dev có trang `#/dev/kit` dựng lại board `Kit`
và `MKit` bằng component thật để so với artboard. Bản build `internal/web/dist`
được commit vì `go build` nhúng nó; sửa web xong phải build lại.
