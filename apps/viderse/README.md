# Viderse (Go)

Viderse vận hành nhiều kênh YouTube: theo dõi kênh đối thủ, chọn video, rồi
làm lại video qua chuỗi 9 bước máy do 4 trợ lý đảm nhận (Trinh sát, Đạo diễn,
Hoạ sĩ, Kiểm định). App đang được dựng lại từ yt-factory theo
[plan dựng lại](../../plans/261029-2242-viderse-rebuild-from-yt-factory/plan.md);
bảng phase trong plan cho biết phần nào đã chạy.

Một binary `viderse` gồm máy chủ web (Go + Postgres) và giao diện React nhúng
sẵn, dựng trên kit `@noverse/ui` ở `platform/ui`. Dữ liệu riêng với Noverse:
thư mục `~/.viderse`, database `viderse_go`, credential nhà cung cấp AI và lịch
sử. Chỉ tài khoản web là dùng chung: đăng nhập một lần bằng cookie
`verse_session` là vào được cả hai app (xem [Tài khoản và phiên](#tài-khoản-và-phiên)).

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

## Chạy

```sh
# 1. Schema nghiệp vụ (goose nhúng trong binary)
VIDERSE_MIGRATION_DATABASE_URL=… go run ./apps/viderse/cmd/viderse migrate up
# 2. Máy chủ web + giao diện nhúng
VIDERSE_DATABASE_URL=… VERSE_IDENTITY_DATABASE_URL=… go run ./apps/viderse/cmd/viderse serve
# 3. Kiểm sống
curl -fsS http://127.0.0.1:8789/healthz
```

Mở `http://127.0.0.1:8789`. Lần đầu (kho tài khoản trống) terminal in **mã thiết lập** dùng một lần; nhập mã rồi tạo quản trị viên. Sau khi đăng nhập, máy chưa cài đủ công cụ thì app mở `#/setup` (Chuẩn bị máy, 6 việc: công cụ, đăng nhập ChatGPT, kiểm tra dung lượng…), xong thì sang Tạo kênh.

## Công cụ trên máy

Mọi binary nằm dưới `~/.viderse/bin` (thư mục dữ liệu `~/.viderse`), tải trong màn Chuẩn bị máy hoặc Cài đặt › Công cụ; không đóng gói kèm repo.

| Công cụ | Bản | Nguồn / giấy phép | Dùng cho |
| --- | --- | --- | --- |
| `ffmpeg` + `ffprobe` | 6.0 (ffmpeg-static `b6.1.1`) | GitHub `eugeneware/ffmpeg-static`, GPL-2.0-or-later (bản build `--enable-gpl --enable-nonfree`) | dựng video, tách tiếng, lớp phủ, xuất MP4 |
| Chrome for Testing | 155.0.8059.79 | Google Chrome for Testing, theo Google Chrome Terms; không tự cập nhật | đăng nhập YouTube/Bilibili bằng hồ sơ trình duyệt |
| `uv` | 0.13.0 | Astral, Apache-2.0 OR MIT | dựng môi trường Python cho bước chép lời (`scripts/clone-transcribe.py`, `internal/tools/transcript-requirements.txt`) |
| `codex` (codex-cli) | 0.162.1 | OpenAI, Apache-2.0 | gọi model chữ/ảnh qua gói ChatGPT |
| `yt-dlp` | bản mới nhất | release `yt-dlp/yt-dlp`, ngoài manifest ghim | tải video/phụ đề |

`ffmpeg`, Chrome, `uv`, `codex` ghim phiên bản + SHA-256 trong `internal/tools/manifest.go`; `yt-dlp` tự cập nhật nên Cài đặt › Công cụ hiện “Có bản mới” và nút Cập nhật. ffmpeg chọn theo thứ tự: đường dẫn người dùng đã chọn → binary hệ thống → bản app tự quản; mọi trạng thái đều kiểm bằng cách chạy binary (phiên bản, arch, filter/encoder cần). Chrome giữ thứ tự đó ở bản desktop; bản chạy trên máy chủ (`PUBLIC_URL`) bỏ hẳn bước binary hệ thống — chỉ đường dẫn người dùng chọn → bản app quản — vì tiến trình phục vụ người khác không được mở Chrome cá nhân của chủ máy (Chrome tự cập nhật đụng bundle của nó, macOS hỏi chủ máy cấp quyền quản lý ứng dụng); thiếu bản app quản thì hàng Chrome báo “Chưa cài” và app tự cài Chrome for Testing.

## Thư mục dự án

Tệp nặng (video nguồn, ảnh đã vẽ, project editor, bản xuất) nằm ngoài DB, mặc định `~/.viderse/projects`; đổi ở Cài đặt › Lưu trữ.

- Máy chủ đọc đường dẫn từ DB (`storage.location`) mỗi lần dùng, nên đổi thư mục **không cần khởi động lại** app (hỏi đáp #10: bản web không dùng `storage.json`).
- Trước khi chuyển: chỗ trống ≥ dữ liệu × 1,1, ghi thử được, không nằm trong thư mục đồng bộ đám mây (`~/Library/CloudStorage`, iCloud Drive, Dropbox). Cùng ổ thì `rename`, khác ổ thì chép theo nhóm có kiểm kích thước + hash, xong mới đổi con trỏ.
- Hàng chờ tạm dừng khi chuyển hoặc khi mất ổ; ổ xuất hiện lại thì tự chạy tiếp. Bản cũ giữ 7 ngày.
- Chạy trên máy chủ (V25) thì tệp có thể nằm trong kho S3; xem `deploy/README.md`.

## LAN và ghép thiết bị — tạm hoãn

`VIDERSE_ADDR` mặc định `127.0.0.1:8789`: máy chủ **không** nghe ra LAN. Quyết định D1 chọn cách LAN + ghép thiết bị một lần bằng mã QR, nhưng Gói V10 (bản điện thoại, ghép thiết bị) đang **tạm hoãn** — chưa có mã ghép, chưa có bản điện thoại riêng, nên đừng mở cổng ra ngoài. Cần xem từ máy khác thì dùng đường hầm SSH:

```sh
ssh -N -L 8789:127.0.0.1:8789 <máy-chạy-viderse>
```

Local SSO chỉ cho phép origin loopback; đặt `VIDERSE_ADDR` ra LAN/`0.0.0.0` sẽ bị từ chối khi chưa cấu hình `PUBLIC_URL` + `VERSE_APP_ORIGINS` (xem `deploy/README.md`).

## Live trên máy chủ

Viderse chạy trên Mac mini cùng Noverse, địa chỉ công khai `vid.aarverse.studio`, dùng chung kho tài khoản và cookie `verse_session` (đặt `VERSE_APP_LINKS=noverse=…,viderse=…` ở cả hai app). Cổng nội bộ `127.0.0.1:8789`, kiểm bằng `/healthz`.

Tuyến live dùng `~/server/bin/build-deploy-viderse.sh` và launchd `com.dat.viderse`; script dựng từ `origin/main`, migrate, kiểm `/readyz` và tự rollback nếu lỗi. Xem [tuyến Mac mini và cấu hình triển khai](deploy/README.md#tuyến-live-trên-mac-mini). Dockerfile vận hành nằm trên máy chủ, không phải Dockerfile của Noverse; hai tuyến dùng chung checkout nên triển khai tuần tự.

## Sao lưu và khôi phục

Sao lưu **cùng lúc**: DB nghiệp vụ, DB identity (kho tài khoản dùng chung), thư mục dự án (hoặc bucket S3) và `~/.viderse/secret.key`.

```sh
# DB nghiệp vụ (role migrate để đọc đủ bảng)
pg_dump --no-owner --no-privileges "$VIDERSE_MIGRATION_DATABASE_URL" | gzip > viderse-data-$(date +%F).sql.gz
# DB identity — sao lưu cùng Noverse, không tách rời
pg_dump --no-owner --no-privileges "$VERSE_IDENTITY_DATABASE_URL" | gzip > verse-identity-$(date +%F).sql.gz
# Tệp dự án + khoá máy (secret.key 0600 mã hoá khoá API/S3 đã lưu)
tar -czf viderse-projects-$(date +%F).tgz -C ~/.viderse projects secret.key
```

Khôi phục: tạo lại hai DB → `psql < dump` → `viderse migrate up` (áp migration còn thiếu) → trả `secret.key` đúng quyền `0600` và thư mục dự án → chạy `serve`. Mất `secret.key` mà còn ciphertext thì **không** tạo khoá mới: khôi phục đúng bản sao lưu (xem `deploy/README.md`). Nếu đang dùng S3, sao lưu bucket bằng công cụ của kho và giữ `secret.key` cùng bản dump; `viderse storage materialize --to <thư mục> --apply` tạo bản local để rollback.

## Biến môi trường

| Biến | Ý nghĩa |
| --- | --- |
| `VIDERSE_DATABASE_URL` | Chuỗi kết nối Postgres (bắt buộc `serve`, `migrate`, `rival …`) |
| `VIDERSE_MIGRATION_DATABASE_URL` | Chuỗi kết nối role migrate; mặc định dùng chung `VIDERSE_DATABASE_URL` |
| `VERSE_IDENTITY_DATABASE_URL` | Chuỗi kết nối kho tài khoản **dùng chung** (role `viderse_identity` trên DB identity của Noverse) — bắt buộc `serve` và `user grant\|revoke`; không có fallback |
| `VIDERSE_ADDR` | Địa chỉ lắng nghe, mặc định `127.0.0.1:8789` (`-addr` ghi đè). Local SSO chỉ cho phép loopback |
| `TRUSTED_PROXIES` / `PUBLIC_URL` / `SECURE_COOKIES` | Ranh giới trusted-proxy (xem `platform/webguard`) |
| `VERSE_COOKIE_DOMAIN` / `VERSE_APP_ORIGINS` | Cookie chung `verse_session` + allowlist origin của SSO (xem `apps/viderse/deploy/README.md`) |
| `VERSE_APP_LINKS` | Liên kết chuyển app `noverse=<origin>,viderse=<origin>`; origin phải đã được phép. Không đặt: chỉ app hiện tại; xem deploy README |
| `VIDERSE_CLAUDE_CODE_VERSION` | Ghi đè wire version Claude Code cho anthropic-oauth |
| `STORAGE_DRIVER` | `local` (mặc định) hoặc `s3`; cùng parser với Noverse, không có alias `disk` trong env |
| `S3_PROVIDER` / `S3_ENDPOINT` / `S3_REGION` / `S3_BUCKET` | Nhà cung cấp và bucket **riêng của Viderse**; Garage dùng vùng `garage` |
| `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | Cặp khoá runtime chỉ có quyền bucket Viderse; không dùng khoá admin hoặc khoá Noverse |
| `S3_FORCE_PATH_STYLE` / `S3_ALLOW_INSECURE_ENDPOINT` | Boolean nghiêm ngặt; HTTP chỉ khi chủ động cho phép mạng nội bộ |
| `BLOB_DELIVERY` / `S3_PRESIGN_ENDPOINT` | `proxy` (mặc định) hoặc `redirect`; endpoint ký phải truy cập được từ trình duyệt |
| `BLOB_CACHE_DIR` / `BLOB_CACHE_MAX_BYTES` | Bộ đệm làm việc cục bộ, có pin và giới hạn; không phải nguồn dữ liệu duy nhất |

Xem `apps/viderse/deploy/README.md` + `apps/viderse/deploy/.env.example` cho
hai DB, role, cookie SSO, origin, Garage/CORS, chuyển kho và khôi phục. Đặt
`STORAGE_DRIVER` hoặc bất kỳ `S3_*` sẽ khoá cấu hình kho trên giao diện.
V25 mốc A hỗ trợ lớp phủ, video nền, ảnh kênh; editor/render/MP4 chưa được
tích hợp và các nhóm usage tương ứng trả `null`.

## Tài khoản và phiên

Tài khoản web sống trong **kho dùng chung** (`platform/identity/postgres`,
DSN `VERSE_IDENTITY_DATABASE_URL`) cùng Noverse — không còn
`~/.viderse/web-accounts.json`:

- Một tài khoản = một username + mật khẩu; đăng nhập ở app này mở được app kia
  bằng đúng cookie `verse_session` (host-only ở local, `VERSE_COOKIE_DOMAIN`
  khi hosted). `viderse_session`/`ainovel_session` là tên cũ: chỉ bị expire.
- Quyền dùng từng app nằm trong `app_grants` (admin toàn cục luôn vào được):
  phiên hợp lệ mà thiếu grant Viderse nhận `403 {code:"app_access_denied"}`
  kèm `user`/`grants` — phiên vẫn sống (có thể mở được Noverse), cookie không
  bị xoá. Cấp/thu hồi: `viderse user grant|revoke <tên> --app noverse|viderse`
  (hoặc `ainovel-cli serve user grant|revoke`); grant mutation thu hồi mọi
  phiên của tài khoản mục tiêu.
- Tài khoản bị đặt mật khẩu tạm (`must_change_password`): mọi route nghiệp
  vụ trả `403 {code:"password_change_required"}` cho tới khi đổi mật khẩu;
  chỉ `/api/me`, `/api/me/password`, `/api/me/sessions*` và `/api/logout` còn
  vào được. Đổi mật khẩu thu hồi **mọi** phiên của tài khoản (cả app kia) và
  trả `{ok:true,reauthenticate:true}` + xoá cookie.
- `GET /api/me` có `session.expiresAt` và `apps: [{id,name,origin}]`; màn thiếu
  quyền cũng nhận `apps` đã lọc theo grant. `401 session_ended` có lý do và app
  thu hồi nếu cookie có bí mật đúng; cookie sai không lộ thông tin phiên.
  Đăng xuất trả 204; đăng xuất các thiết bị khác giữ phiên hiện tại.
- `GET /api/setup` chỉ báo `needed` khi kho đọc được và trống — lỗi kho trả
  503, không phát mã ảo. Sau thiết lập lần đầu, Viderse không tạo được tài
  khoản mới: thêm thành viên ở Noverse (Tuỳ chỉnh › Thành viên).
- Cấu hình toàn máy chỉ quản trị viên được ghi; thành viên nhận `403` ở một
  cổng chung của gateway (`internal/web/admin_gate.go`): provider và agent AI
  (`/api/ai/providers*` ghi, luồng đăng nhập, `PUT /api/ai/agents`), mọi
  `/api/platforms/*` trừ `POST /api/platforms/requests` (cookie, hồ sơ Chrome
  kèm email, QR), khoá YouTube Data, `/api/tools/*` ghi, `/api/setup/*` ghi
  (Chuẩn bị máy; tạo quản trị viên đầu tiên vẫn là `/api/setup`,
  `/api/setup/verify`, `/api/setup/admin` do platform/webapp phục vụ trước
  phiên), `/api/storage/cleanup/*`, thao tác thư mục lưu trữ (kể cả duyệt thư mục máy chủ), cài đặt
  thông báo và quy tắc chạy của hàng chờ (`workers`, `imageWorkers`,
  `rescanHours`). Thành viên vẫn đọc được danh sách provider/nền tảng nhưng
  không thấy `keyHint`, `baseUrl`, tài khoản hay hồ sơ trình duyệt.
- Địa chỉ tuỳ biến của provider (`ollama`, `openai_compat`): khi `PUBLIC_URL`
  được đặt (hosted), baseUrl phải phân giải (DNS trước khi kiểm) ra địa chỉ
  công khai — loopback, link-local (gồm metadata cloud), mạng riêng, CGNAT và
  multicast bị từ chối, nên Ollama trên chính máy chủ không dùng được khi
  hosted. Quy tắc đó được áp lại **ở mỗi lần nối** (transport của client
  provider, và transport của client text), không chỉ lúc lưu: tên được phân
  giải một lần rồi nối bằng IP literal, nên DNS đổi về loopback sau khi kiểm
  (DNS rebinding), redirect hay keep-alive nối lại đều bị chặn. Không đặt
  `PUBLIC_URL` (app một chủ trên máy cá nhân) thì giữ nguyên
  Ollama ở localhost/LAN. Đổi baseUrl sang **máy chủ khác** của thẻ đã lưu khoá
  cần nhập lại khoá API trong cùng request; khoá đã lưu không bao giờ được gửi
  tới địa chỉ mới.
- Chuyển `~/.viderse/web-accounts.json` cũ vào kho chung:
  [apps/noverse/deploy/sso-cutover.md](../noverse/deploy/sso-cutover.md).

## Lệnh

```bash
go run ./apps/viderse/cmd/viderse serve            # máy chủ web + giao diện nhúng
go run ./apps/viderse/cmd/viderse migrate up       # áp schema (goose nhúng)
go run ./apps/viderse/cmd/viderse migrate status
go run ./apps/viderse/cmd/viderse login            # đăng nhập gói thuê bao (mặc định openai-codex)
go run ./apps/viderse/cmd/viderse login --status
go run ./apps/viderse/cmd/viderse logout
go run ./apps/viderse/cmd/viderse user grant  linh --app viderse   # cấp quyền dùng Viderse (kho chung)
go run ./apps/viderse/cmd/viderse user revoke linh --app viderse   # thu hồi + thu hồi phiên
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
| tài khoản web + phiên | kho chung Postgres (`VERSE_IDENTITY_DATABASE_URL`), cookie `verse_session` |
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
npm run size -w apps/viderse/web         # kích thước gzip JS tải lần đầu, lỗi khi vượt 150 kB
```

Các màn Cài đặt dùng StyleX (`web/src/screens/settings/settings.stylex.ts`),
không nạp stylesheet CSS riêng. Trị số artboard chưa có token chung nằm trong
`settingsVars.stylex.ts`; trạng thái và breakpoint được ghép bằng StyleX.
Kit nhận `xstyle` cho phần tử gốc và `<part>Style` cho phần tử con (ví dụ
`inputStyle`, `buttonStyle`), để app ghi đè đúng thứ tự mà không dùng selector
xuyên vào DOM nội bộ của kit.

Brand Viderse (màu chính `#FF4D63`, bộ màu ở `docs/design/v1/02-stack-verse.md`
§2) nạp ở `web/src/brand.ts`. Bản dev có trang `#/dev/kit` dựng lại board `Kit`
và `MKit` bằng component thật để so với artboard. Bản build `internal/web/dist`
được commit vì `go build` nhúng nó; sửa web xong phải build lại.
