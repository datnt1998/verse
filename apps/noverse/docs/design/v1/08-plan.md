# 08 · Kế hoạch implement

Mỗi pha merge được độc lập. Pha 0–3 chỉ dùng API hiện có; pha 4–5 cần backend (GAP trong `06`).

## P0 · Nền móng
- Thay `web/src/style.css` bằng `@import "tailwindcss"; @import "./tokens.css";` (copy `tokens.css` vào `web/src/`).
- `web/src/ui/`: Icon (từ `docs/design/v1/icons/`), AgentOrb, Button, Kbd, Chip, Badge, Card, Tabs, Segmented, Switch, Checkbox, Field, Spinner, Shimmer, ProgressBar, Dialog, Toast, Scroll.
- `AppShell`, `AppHeader`, `ContextCrumb`, `MainNav`; router hash theo bảng route (`02`), redirect `#/book/:id`.
- `ConnectionBanner`, `LockedDialog` (409). 401 → `#/login` (làm ở PA, trong lúc chờ dùng màn đăng nhập tối thiểu).
- `web/src/i18n/vi.ts`, `web/src/i18n/hostStrings.ts` (bảng dịch `06`).
- **Nghiệm thu**: build/typecheck sạch; header + nav đúng artboard; focus ring; reduced-motion tắt animation.

## PA · Tài khoản & đăng nhập (API có sẵn trong `platform/identity`)
Làm ngay sau P0 vì mọi màn đều cần phiên. Chi tiết `09-auth-onboard.md`.
- `AuthShell`, `OrbHero`, `AccountPicker` (localStorage `ainovel.accounts`), `PasswordField`, `#/login` 4 trạng thái (chọn · mật khẩu · khác · hết hạn) + lỗi 401/429.
- `AccountMenu`, `PairingDialog` (QR từ `svg` server, tabs `links[]`, đếm ngược 5 phút, dò `sessions` 3s), `SessionList`, Thiết bị & bảo mật, đổi mật khẩu.
- Quản lý tài khoản (admin): bảng, editor vai trò/quyền truyện, tạo tài khoản, đặt lại mật khẩu, khoá/xoá, chặn quản trị viên cuối.
- Điện thoại: `#/pair/:code` (xác nhận → `POST /api/pair`), màn đăng nhập mobile, `MobileShell`.
- Onboard `#/welcome`: dựng đủ khung + bước Hồ sơ, Điện thoại & người cùng viết, Hoàn tất, Mật khẩu riêng; các bước cần GAP (3–6) hiển thị chỉ đọc cho tới khi có GAP-5/13/15/16. Trong lúc chưa có GAP-15: coi “lần đầu” = chưa có khoá `ainovel.onboarded.<username>` trong localStorage.
- **Nghiệm thu**: đăng nhập sai 401 giữ nội dung; 429 khoá nút + đếm ngược; QR quét trên iPhone thật trong LAN đăng nhập được, desktop tự hiện “Đã đăng nhập”; hết hạn mã → nút tạo lại; thu hồi phiên ở máy A làm máy B về `#/login` ở request kế tiếp; người đọc không thấy nút tạo truyện; quản trị viên cuối không khoá/xoá được.

## P1 · Workspace (giá trị cao nhất)
- `useBookEvents` giữ nguyên, thêm gom `delta` theo rAF.
- RunHeader + chips, OutlineRail, ActivityFeed/ActivityRow, LiveStream + parser `streamRounds.ts` (có unit test với chuỗi mẫu `✻ 打磨` / `✻ 章节提交`), ThinkingLine, Composer + RunRing, ShortcutHints, InspectorTabs (Agent/Chi phí/Truyện), GateCard.
- Hành động: input/abort/resume/next/review/reopen (ReopenDialog), CommandPalette (lệnh có API).
- Ẩn/hiện panel + phím `[` `]` + nhớ localStorage. Mobile tab đáy.
- **Nghiệm thu** (chạy `npm run dev:mock` và với `serve` thật):
  1. 1440×900 và 1280×800: trang không cuộn; ô chat luôn thấy; nhật ký & luồng viết tự bám đáy.
  2. Đang chạy: spinner, orb thở, viền chạy ô chat; Tạm dừng: tất cả dừng, placeholder đổi đúng `07`.
  3. Sự kiện cập nhật tại chỗ theo `id`; tool thụt `├`; thời lượng đúng định dạng.
  4. Round `✻` hiển thị nhãn tiếng Việt; `edit_chapter` ra diff; `commit_chapter` ra thẻ phục bút.
  5. Không còn chuỗi tiếng Trung lộ ra với các mẫu trong bảng dịch.
  6. 375px: không cuộn ngang, ô chat sticky.

## P2 · Trình đọc & xuất
- ChapterPicker, ReaderToolbar (cỡ chữ nhớ localStorage), ReaderBody, ExportPanel (format → `downloadExport`), QuickActions điền sẵn ô chat.
- ReviewPanel dùng `lastReviewSummary` (verdict + số vấn đề) cho tới khi có GAP-7; ẩn các phần chưa có dữ liệu thay vì hiển thị mẫu.

## P3 · Tạo truyện (Bắt đầu nhanh)
- PromptBox (≤5000), ví dụ, FlowPreview, AltEntries (disabled + nhãn “Chưa có API”).
- `createBook` (tên tạm `NN-truyen-moi`) → `sendInput` → điều hướng Workspace.

## P4 · Dự án & thư viện (cần GAP-1/2/3/9/11/12)
- Backend: gateway 3 cấp, project.json/story.json, worker cwd = thư mục truyện, đổi tên thư mục, ý tưởng, validate.
- Frontend: ProjectSwitcher, ProjectTree, ProjectOverview, StoryView (5 tab), FolderTree, NewProjectDialog.

## P5 · Cài đặt, xuất tự động, nhật ký đầy đủ (cần GAP-4/5/10)
- SettingsNav + 9 mục, SaveBar, kiểm tra tên thư mục, xem trước slug & mẫu tên tệp, chuỗi kế thừa.
- Activity page: bộ lọc 7 loại, ẩn/hiện tool, nhóm theo phiên.
- Xuất tự động + ExportMatrix.

## P6 · Audio (v1.1, cần GAP-17…21) — chi tiết `10-audio.md`
- Bản đọc: danh sách chương theo trạng thái, đoạn (người nói, sắc thái, ngừng, chuẩn hoá), nghe thử đoạn, gợi ý người nói/tách câu, chốt/mở khoá/stale.
- Phân vai, mẫu giọng, cách đọc; Cài đặt › Giọng đọc.
- Xác nhận tạo → tiến độ (nghe chương đã xong) → trình nghe (sóng âm, bản đọc chạy theo, cờ, đọc lại đoạn A/B) → ghép & xuất (mốc chương, .srt).
- **Nghiệm thu**: không tạo được khi chưa chốt đủ chương; sửa 1 đoạn chỉ tạo lại đoạn đó; tạm dừng/tiếp tục giữ tiến độ; `Range` tua được trên Safari; mốc chương khớp thời gian thật ±1 s; phụ đề `.srt` mở được trên YouTube.

## Sau v1
Đồng sáng tác (GAP-6), `/import` (Dialogs › import), `/sync`, `/diag`, `/simulate`, đổi model lúc chạy, giao diện sáng.

## Quy ước cho Claude Code
- Không sửa `output/novel/` hay engine khi làm frontend; mọi thay đổi backend ghi rõ GAP-ID trong commit.
- Mọi giá trị màu/bo/khoảng cách lấy từ token; không hard-code hex trong component (trừ gradient đã có biến).
- Mỗi component có trạng thái rỗng / đang tải / lỗi.
- Đối chiếu pixel với `artboards/*.dc.html` (mở bằng trình soạn Design hoặc đọc giá trị inline style).
