---
title: "Dựng khung web Viderse theo kiến trúc Noverse"
description: "Thay giao diện tối giản của apps/viderse bằng khung web cùng kiến trúc Noverse, dùng chung UI kit (platform/ui) và lớp HTTP đăng nhập (platform/webapp)."
status: done
priority: P1
branch: "main của datnt1998/verse (Actions đang tắt)"
tags: [monorepo, web, ui-kit, platform, scaffold]
created: 2026-10-29
---

# Kết quả cần đạt

Anh yêu cầu "Viderse — chỉ khởi tạo, kiến trúc UI tương tự Noverse; cứ dựng core codebase, chưa cần làm gì cả".

`viderse serve` phục vụ một khung web có cùng kiến trúc với Noverse:

- React 19, Vite, StyleX + Tailwind 4;
- `@noverse/ui`, hash router có kiểu;
- `api/client` + `SessionProvider`;
- `shell/` desktop và mobile, `screens/`, `i18n/`.

Khung có thiết lập lần đầu (mã → quản trị viên → mã khôi phục), đăng nhập/đăng xuất, trang chủ và trang tài khoản. Chưa dựng màn nghiệp vụ.

# Đính chính trong lúc làm

Lần đầu em đọc "Vidverse" thành một app mới và dựng `apps/vidverse` (commit `53b9ae0`). Anh đã đính chính: đó là Viderse.

- Khung web được chuyển sang `apps/viderse/web`, thay giao diện tối giản cũ.
- `apps/vidverse` đã gỡ hẳn: Go, web, dist, README.
- Hai phần dùng chung vẫn giữ vì Viderse dùng chúng:
  - UI kit `platform/ui` và npm workspace gốc;
  - `platform/webapp` (`d6f8768`).

# Quyết định

| Quyết định | Nội dung |
|---|---|
| Vị trí UI kit | `platform/ui`, npm workspace ở gốc repo gồm `platform/ui`, `apps/noverse/web`, `apps/viderse/web`. Tên gói giữ `@noverse/ui` |
| Lớp HTTP | Thiết lập/đăng nhập/phiên/SPA nhúng chung ở `platform/webapp`. Viderse chỉ đăng ký route runs/scripts |
| Màn nghiệp vụ | Chưa dựng, theo lời anh. Giao diện tối giản cũ (gửi brief, danh sách run, xem kịch bản) đã bị thay thế; xem lại được ở commit `445f0f1`, file `apps/viderse/web/src/App.tsx`. API runs/scripts vẫn còn nguyên |
| Brand | Màu chính `#F43F5E` qua `defineBrand`, đổi ở `web/src/main.tsx` |

# Tiêu chí nghiệm thu

- `go build ./...` và `go test` cho `apps/viderse`, `platform/webapp`, `archtest` xanh.
- Ở gốc repo, `npm ci` xanh; typecheck và test xanh cho cả 3 workspace.
- `npm run build -w apps/noverse/web` không làm hỏng dist của Noverse.
- Docker build `apps/noverse/Dockerfile` qua.
- Smoke thật `viderse serve` (HOME tạm, database `viderse_go` thật):
  - thiết lập → shell;
  - `/api/me` trả 200;
  - đăng xuất → 401;
  - đăng nhập lại được;
  - POST khác origin → 403.
- Live Noverse 8790 không bị ảnh hưởng.

# Kết quả (2026-10-29)

**Go.** Lượt đầy đủ `go test ./...` (`53b9ae0`): 2779 test cấp cao nhất pass, 108 skip, 0 fail. Sau khi chuyển khung sang Viderse (chỉ đổi web/dist) chạy lại `apps/viderse/...`, `platform/webapp`, `archtest`: xanh. `-race` xanh cho `platform/webapp` và `apps/viderse/internal/web`.

**Web.** Typecheck cả 3 workspace xanh.

| Workspace | Test pass |
|---|---|
| `platform/ui` | 116 |
| `apps/noverse/web` | 1524 |
| `apps/viderse/web` | 16 |

- Kit + Noverse = 1640 test, so với 1639 trước khi dời. Test thêm là "có app dùng kit để kiểm" trong `boundary.test.ts`.
- 10 test cũ của `apps/viderse/web/src/lib.test.ts` đi cùng màn cũ.

**Dist Noverse sau khi dời kit**
- CSS giống từng byte. Cần `@source` trỏ tới `platform/ui/src` trong `base.css`; thiếu nó, Tailwind không quét kit.
- JS chỉ khác thứ tự module, vì `KIT_LABELS_VI` nay nằm trong kit.

**Smoke thật** (`viderse serve`, HOME tạm, DB `viderse_go`, Chrome headless):
- Thiết lập → vào shell; cookie `viderse_session`.
- `/api/me` 200; `/api/runs` trả 2 run có sẵn.
- Đăng xuất → `/api/me` 401; đăng nhập lại về đúng trang đang mở.
- POST từ origin `:8790` → 403; `/api/runs` không cookie → 401.
- Console không có lỗi. Bảng `agent_runs` không đổi (2 `succeeded`).

**Live Noverse 8790** không bị ảnh hưởng.
