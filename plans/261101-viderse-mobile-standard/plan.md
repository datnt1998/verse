---
title: "Viderse mobile: chuẩn mobile chung với Noverse, khớp artboard từng màn"
description: "Đưa hợp đồng safe-area, thanh tab, giữ màn, bàn phím và component mobile lên platform/ui; sửa mọi màn điện thoại Viderse theo artboard M*; hết cảnh thanh tab đè nội dung; đạt ngân sách hiệu năng."
status: in_progress
priority: P0
branch: main
tags: [frontend, mobile, design-system]
created: 2026-11-01
---

# Outcome

Mọi màn điện thoại Viderse (< 768 px) khớp artboard `apps/viderse/docs/design/v1/artboards/M*.dc.html` từng màn, từng trạng thái, từng component; không phần tử nào bị thanh tab dưới, notch hay home indicator đè; đổi tab không mất màn/vị trí cuộn và không tải lại dữ liệu; hiệu năng bằng hoặc tốt hơn Noverse mobile. Thứ dùng chung giữa hai app nằm ở `platform/ui`, không còn hai bản.

# Nguồn

- Chuẩn Noverse + so sánh 28 điểm + catalogue component: `plans/reports/viderse-mobile/01-noverse-mobile-standard.md`
- Hiệu năng: `plans/reports/viderse-mobile/02-performance.md` (đã đo; tối ưu tiếp theo ngân sách)
- Audit màn lõi (MAuth, MMain, MNew, MQueue, MReview, MKit, shell): `plans/reports/viderse-mobile/03-audit-core.md`
- Audit màn nội dung (MChannel, MCompVideo, MCreate, MExplore, MSettings, Thư viện, Template): `plans/reports/viderse-mobile/04-audit-content.md`

# Quyết định

1. **Mốc bề ngang**: giữ hai mốc có chủ đích — Noverse điện thoại ≤ 640 (+ máy tính bảng 641–1099), Viderse điện thoại ≤ 767 (không có artboard máy tính bảng). Cả hai là hằng số đặt tên trong `platform/ui` (TS + CSS); kit không còn mốc thứ ba (759 trong `Toast.tsx`). Đổi được sau nếu chủ dự án muốn một mốc.
2. **Safe area một nguồn** (luật R-SA1…R-SA9 của báo cáo 01 §4.3): biến `--m-safe-top/-bottom`, `--m-edge`, `--m-top`, `--m-tabbar-h`, `--m-tabbar-bottom`, `--kb` khai báo **một lần** trong `platform/ui`; Noverse chuyển sang dùng bản đó (xoá khai báo riêng); Viderse dùng cùng bản với `--m-tabbar-gap` 30 px theo artboard (Noverse 20 px). Cấm đọc `env(safe-area-inset-*)` ngoài khai báo — test hợp đồng ở kit.
3. **Thanh tab không bao giờ đè nội dung**: vùng cuộn đệm đáy `= tabbar-bottom + tabbar-h + khe` (Viderse 118 px ở inset 0, đúng `04-components.md:229`); mọi thanh dính đáy (CTA, thanh chọn nhiều, footer trang con, composer) hoặc nằm **trên** thanh tab theo cùng gốc `calc(var(--m-tabbar-bottom) + var(--m-tabbar-h) + …)`, hoặc màn yêu cầu thanh tab **ẩn** (`useTabMode('hide')`) khi artboard thay thanh tab bằng thanh riêng (chọn nhiều, trang con, bàn phím mở). Không viết cứng px.
4. **Cấu trúc**: màn có artboard M* nhưng đang render layout desktop (Bàn dựng, Duyệt, Khám phá, Chi tiết video) có thư mục `mobile/` riêng như Noverse; không nhồi nhánh `mobile ?` vào màn desktop.
5. **Component hệ thống** (báo cáo 01 §13, 04 §9) lên `platform/ui`: `KeepAlive`, `MobileTabBar` (slot `centerAction` cho FAB Viderse), `MobileTopBar`, `MobilePage`, `ActionSheet`/`ConfirmSheet`, `ListRow`, `StatusPill`, `MeterBar`, `SelectionBar`, `VideoRow`, `MobileChip`/`Chip density`, `Stepper`, `EmptyState`, `OfflineBanner`, `InitialsAvatar`, hooks `useTabMode`, `useMobileScroll`, `useVisualViewportKeyboard`, `useToastBottom`. Noverse được cắt sang bản kit khi component đó lên kit (không để hai bản).
6. **GAP thiết kế / dữ liệu**: Gói V34 đã bổ sung và triển khai MLibrary, MTemplate, MChannel kênh mới, trạng thái danh sách MExplore, nhóm YouTube Data API và Xong khi Đạo diễn lỗi; xem `apps/viderse/docs/design/v1/10-gap-closure.md` › Gói V34. Xem thử nền và xuất template đã có API/service thật; còn GAP danh sách chương B2 trong hợp đồng thiết kế. Không dựng dữ liệu giả trong sản phẩm.

# Phase

| # | Phase | Phụ thuộc | Trạng thái |
|---|---|---|---|
| 1a | **Nền safe-area + thanh tab không đè** (kit tokens, Noverse cutover, Viderse shell/tab bar 30 px, đệm cuộn, mọi thanh dính đáy, `.vv-mset` dưới notch, sheet padding, toast neo, `--kb`, tab mode `hide`/`mini`) | — | đã tích hợp; sửa vùng cuộn có 220 phép đo cuối màn không che khuất, kiểm lại sau đổi bố cục |
| 1b | **Giữ màn khi đổi tab** (`KeepAlive` lên kit, Viderse pane keep-alive, giữ cuộn, 0 request khi cache còn, prefetch idle) | — | đã tích hợp |
| 1c | **Component mobile hệ thống** theo MKit (kích thước 44/46/48, sheet title 18, chip 36/44, field 16 px…) + Noverse cutover | — | đã tích hợp V33; kiểm kit233 test, native và focus/menu/segmented đạt |
| 2 | **Màn theo artboard**: MMain (Bàn dựng 1 cột + chi tiết + kế hoạch + bản tin + việc nền), MReview (8 trạng thái), MExplore + MCompVideo (B2 màn riêng), MCreate + MChannel + MSettings, MNew + MQueue + MAuth polish | 1a, 1c | đã tích hợp; có báo cáo/ảnh native từng slice và lượt kiểm bản production tích hợp 390/360 trong `plans/reports/261102-final-built-mobile/` |
| 3 | **Hiệu năng** theo báo cáo 02 (ngân sách JS màn đầu, TBT, render storm, ảnh) | 1b | đang tối ưu tiếp theo lựa chọn rõ ràng của chủ dự án, **không chấp nhận ngoại lệ**: CSS theo route giữ cascade, gộp dependency màn đầu/inline sáu icon, khử request trùng nhưng giữ prefetch; sau đó đo cold production và deploy lại. Baseline ở báo cáo02 §10 |
| 4 | **GAP design** → làm màn khi artboard về | design | V34 đã tích hợp; còn các GAP API nêu trên |

# Nghiệm thu mỗi màn

- Ảnh chụp cặp artboard ↔ app ở 390×844 (thêm 360×780, 430×932 khi bố cục co giãn) với inset giả lập 59/34 (`Emulation.setSafeAreaInsetsOverride`, đặt lại sau mỗi điều hướng) **và** inset 0; artboard mở qua HTTP (không `file://`).
- Không phần tử tương tác nào bị thanh tab/notch/home indicator che (kiểm bằng `elementFromPoint` ở tâm từng nút/hàng cuối); vùng chạm ≥ 44 px.
- `src/checks/design-rules.test.ts` + test hợp đồng safe-area xanh; typecheck; vitest Viderse + Noverse + platform/ui đầy đủ.
- Dist nhúng build lại, deploy, kiểm trên production.
