---
title: "Khởi tạo Vidverse"
description: "Dựng app thứ ba apps/vidverse: backend Go tối thiểu và web theo kiến trúc UI của Noverse, dùng chung bộ UI kit và lớp HTTP đăng nhập qua platform/."
status: done
priority: P1
branch: "main của datnt1998/verse (Actions đang tắt)"
tags: [monorepo, web, ui-kit, platform, scaffold]
created: 2026-10-29
---

# Kết quả cần đạt

`vidverse serve` chạy được ở `127.0.0.1:8788`. Lần đầu nó in mã thiết lập. Trên web có thể:

- nhập mã, tạo quản trị viên, nhận mã khôi phục;
- vào app shell: điều hướng desktop và mobile, trang chủ trống, trang tài khoản;
- đăng xuất rồi đăng nhập lại.

Giao diện dùng cùng kiến trúc với Noverse và mang brand riêng: React 19, Vite, StyleX + Tailwind 4, `@noverse/ui`, hash router có kiểu, `api/client` + `SessionProvider`, `shell/`, `screens/`, `i18n/`.

# Quyết định (anh chốt 2026-10-29)

| Quyết định | Nội dung |
|---|---|
| Vị trí UI kit | Dời `apps/noverse/web/packages/ui` → `platform/ui`, dùng npm workspace ở gốc repo (`platform/ui`, `apps/noverse/web`, `apps/vidverse/web`). Tên gói giữ `@noverse/ui` |
| Quan hệ với Viderse | Vidverse là app mới riêng; Viderse giữ nguyên sản phẩm. Web Viderse vẫn đứng riêng, không vào workspace |

# Giả định em tự chốt

- **Lớp HTTP dùng chung.** Phần chung của gateway Viderse (SPA nhúng, `/healthz`, setup lần đầu, login/logout/me, cookie phiên) được dời ra `platform/webapp`. Viderse chuyển sang dùng gói này mà hành vi HTTP không đổi. Đây là luật "app thứ hai cần thì dời ra platform" trong README.
- **Nhãn tiếng Việt của kit.** `KIT_LABELS_VI` dời vào kit. Mỗi app chỉ ghi đè `brand`.
- **Dữ liệu riêng của Vidverse:** thư mục `~/.vidverse`, cookie `vidverse_session`, cổng mặc định 8788, brand màu chính `#F43F5E`.
- **Ngoài phạm vi (non-goals):** Postgres, agent/LLM, PWA, deep link, giao diện tablet, Dockerfile của Vidverse.

# Các bước

1. Dời UI kit và dựng workspace gốc. Noverse phải giữ nguyên:
   - Noverse typecheck và test (tổng số test của Noverse cộng kit không đổi), build;
   - dist so với bản đã commit (giống hệt, hoặc giải thích được mọi khác biệt);
   - Docker build web stage;
   - Viderse web vẫn `npm ci`/test riêng được.
2. `platform/webapp` và chuyển Viderse sang dùng. Test Viderse xanh, `archtest` xanh.
3. Web Vidverse (`apps/vidverse/web`), build ra `apps/vidverse/internal/web/dist`.
4. Backend Vidverse (`apps/vidverse/cmd/vidverse`, `internal/bootstrap`, `internal/web`).
5. Tích hợp: CI (job web Vidverse, test kit), README gốc và README app, `.gitignore`.
6. Kiểm:
   - `go test ./...`, `-race` cho `platform/webapp`;
   - web của cả 3 workspace;
   - smoke trình duyệt (setup → shell → đăng xuất/đăng nhập, desktop + mobile).

# Tiêu chí nghiệm thu

- `go build ./...`, `go test ./...` và `go test ./archtest` xanh.
- Ở gốc: `npm ci` xanh, sau đó:
  - `npm test` và `npm run typecheck` xanh cho cả `platform/ui`, `apps/noverse/web`, `apps/vidverse/web`;
  - `npm run build -w apps/noverse/web` không làm hỏng dist của Noverse.
- `apps/viderse/web`: `npm ci && npm test` vẫn xanh.
- Smoke thật trên `vidverse serve` (HOME tạm):
  - mã thiết lập → tạo admin → shell hiện tên người dùng;
  - `/api/me` 200;
  - đăng xuất → `/api/me` 401;
  - đăng nhập lại được;
  - POST khác origin → 403.
- Live Noverse 8790 không bị ảnh hưởng.

# Kết quả (2026-10-29)

**Go**
- `go test ./...`: 2779 test cấp cao nhất pass, 108 skip (cần Postgres/credential), 0 fail.
- `-race` xanh cho `platform/webapp` và `apps/viderse/internal/web`.
- `go vet` sạch; `gofmt` (Go 1.26 và 1.27) sạch.

**Web.** Tổng test của kit và Noverse là 116 + 1524 = 1640, so với 1639 trước khi dời; test thêm là "có app dùng kit để kiểm" trong `boundary.test.ts`. Typecheck cả 3 workspace xanh.

| Workspace | Test pass |
|---|---|
| `platform/ui` | 116 |
| `apps/noverse/web` | 1524 |
| `apps/vidverse/web` | 16 |
| `apps/viderse/web` (`npm ci` riêng) | 10 |

**Dist Noverse sau khi dời kit**
- CSS giống từng byte với bản đã commit. Muốn vậy phải thêm `@source` trỏ tới `platform/ui/src` trong `base.css`; trước đó Tailwind không quét kit và rơi mất `!hidden` và `shadow`.
- JS chỉ khác thứ tự module: `KIT_LABELS_VI` nay nằm trong kit nên được nạp sớm hơn, minifier đặt lại tên biến. Nội dung không đổi.
- Docker build `apps/noverse/Dockerfile` (workspace gốc) qua; container trả `/healthz` 200.

**Smoke thật** (`vidverse serve`, HOME tạm, Chrome headless):
- `/api/me` khi chưa đăng nhập trả 401; POST từ origin `:8790` trả 403; `/api/*` lạ trả 404 JSON.
- Thiết lập: nhập mã → tạo admin → thấy mã khôi phục → vào shell.
- Shell desktop (nav trên cùng, menu tài khoản) và mobile 390px (nav dưới) đều chạy; trang Tài khoản hiện tên, vai trò, ngày tạo.
- Sau khi đăng xuất, `/api/me` trả 401. Đăng nhập lại được.
- Khởi động lại server, phiên vẫn còn. Đăng xuất qua menu tài khoản chạy được.
- Console không có lỗi.

**Sửa trong lúc smoke.** Chip trạng thái máy chủ ở màn đăng nhập trước luôn ghi "đang hoạt động". Nay nó theo lần kiểm phiên gần nhất, kèm 2 test (mất mạng; máy chủ trả lỗi HTTP).

**Live Noverse 8790** không bị ảnh hưởng: `/healthz` trả 200 suốt quá trình.
