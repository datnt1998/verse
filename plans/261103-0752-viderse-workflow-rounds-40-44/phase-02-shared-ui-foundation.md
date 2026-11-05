---
phase: 2
title: "Bước 1 · Nền UI dùng chung"
status: completed
priority: P1
dependencies: [1]
---

# Bước 1 · Nền UI dùng chung — PR riêng

## Mục tiêu và nguồn
Mọi màn dùng cùng quy tắc chọn, lớp phủ, xác nhận và phản hồi trước khi thay workflow.
Nguồn: `Workflow.dc.html` mục 04; `UkControls`, `UkInputs`, `UkFeedback`, `UkOverlays`, `UkMotion`; yêu cầu người dùng quy định toast kết quả 4 giây, thay thời gian cũ trong `04-components.md`.

## Bằng chứng hiện tại
- `/Users/dat/Projects/verse/platform/ui/src/Tabs.tsx:173-214` đã đo bằng hộp thật, quan sát nhóm và từng nút, chờ `document.fonts.ready`. Không xây lại thuật toán chỉ vì checklist nhắc nó; kiểm các consumer còn tự vẽ nhóm chọn, toạ độ, scroll và font tải muộn.
- `Sheet.tsx:40-115` đã có portal, scrim, Esc, focus trap, scroll lock và reduced motion; `Overlays.tsx:18-49` đã sở hữu stack Esc.
- `Selects.tsx:185-190` dùng absolute popover; `Menus.tsx:90-130` dùng portal menu. Tái dùng, không dựng danh sách trong document flow.
- `Toast.tsx:147-156` đang có thời gian khác nhau và `undo` ép 5 giây. `src/App.tsx:81` mount `ToastHost activity`, không có ToastProvider hay components/Toast.tsx riêng. Viderse cần policy 4 giây; không đổi mặc định của Noverse.
- `mobile/sheets.tsx:156-237` đã có ConfirmSheet, lỗi tại chỗ và pending lock. `components/confirm.tsx` đã có các thân xác nhận; bỏ các biến thể autonomy khi tới V38.

## Phạm vi file
| Hành động | File / ranh giới | Nội dung và ảnh hưởng kiểm thử |
|---|---|---|
| Giữ, chỉ sửa khi chứng minh sai | `/Users/dat/Projects/verse/platform/ui/src/Tabs.tsx`, `Sheet.tsx`, `Overlays.tsx`, `Selects.tsx`, `Menus.tsx`, `mobile/sheets.tsx` | Reuse primitive; regression cho hành vi thật, không test cấu trúc source |
| Sửa có giới hạn | `/Users/dat/Projects/verse/platform/ui/src/Toast.tsx` | Cho explicit duration thắng mặc định undo, giữ nguyên fallback của consumer khác |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/App.tsx`; `src/components/Segmented.tsx`, `confirm.tsx`, `index.ts`; `src/shell/AppShell.tsx`, `toastAnchor.ts` | Policy ứng dụng ở host thật, ranh giới session, xác nhận responsive, neo toast |
| Tạo khi chưa có owner tương đương | `/Users/dat/Projects/verse/apps/viderse/web/src/components/responsive-overlays.tsx`, `action-feedback.ts` | Adapter mỏng cho cùng options/actions và toast 4 giây; không copy primitive hay tạo service |
| Sửa/tạo kiểm thử | Các test kit liền kề đã có; `/Users/dat/Projects/verse/tests/pc-uikit/` | Bổ sung browser audit responsive theo [giao thức nghiệm thu](verification.md) |

## Kiến trúc và thực hiện
1. Trước khi sửa exported API của kit, dùng LSP references nếu có; nếu không có language server, lập danh sách import/caller bằng tìm kiếm và xác nhận chữ ký thực tế. Chỉ mở rộng props khi adapter Viderse không đủ.
2. Một dữ liệu options/actions dùng chung desktop/mobile: ≥768px chọn/menu là popover không đẩy nội dung; <768px là Sheet/ActionSheet. Không đổi nhãn, mặc định, quyền hay action giữa hai bố cục.
3. Tái dùng stack Esc; state lớp tương tác thuộc AppShell/session, không singleton chia sẻ giữa tài khoản. Một lớp có quyền tương tác trên cùng; khi thay một sheet bằng xác nhận, đóng lớp cũ rồi mở lớp mới. Không mở Dialog bên trong Dialog. Popover chọn trong Nhập nhanh không phải hộp thoại thứ ba.
4. Confirmation chỉ Huỷ + một primary, thân lấy số ảnh/GB/job bị ảnh hưởng từ dữ liệu thật. Giữ lỗi và dữ liệu nhập khi mutation thất bại; chặn double submit; Huỷ/scrim/Esc trước gửi không ghi dữ liệu. Không huỷ giả một mutation đã gửi.
5. Dùng `toast(..., { duration: 4000 })` qua policy Viderse cho kết quả thành công/lỗi có thể xử lý; undo chỉ hiện nếu thao tác ngược thực sự có sẵn. Đổi precedence duration trong kit để undo 4 giây có hiệu lực. Thẻ sự kiện nền không biến thành toast giả tiến độ; giữ phân biệt kết quả hành động và thông báo cần xử lý.
6. Đo viên trượt sau font, thay nhãn/options, resize từng nút, scroll và remount. Nếu kit đúng nhưng consumer sai, sửa consumer; không thêm offset bù 4px.
7. Chuyển callsite thuộc màn chung trong PR này; các màn được viết lại ở PR sau nhận cùng adapter, không giữ hai convention. Không sửa artboard.

## Ma trận tình huống
| Mức | Tình huống | Điều kiện qua |
|---|---|---|
| Critical | Sheet → xác nhận; một lần Esc/scrim | Chỉ lớp trên đóng, nền không nhận click, focus về đúng trigger, scroll khôi phục |
| Critical | Xác nhận xoá/logout/tốn hạn mức; lỗi mạng/double click | Có hệ quả thật, mutation tối đa một lần, lỗi có đường sửa, không toast thành công giả |
| High | Font chậm, nhãn dài, resize và cuộn nhóm chọn | Viên trượt cách mọi cạnh nút chọn ≤2px sau settle |
| High | Toast có/không undo, hover/focus, thao tác undo lỗi | Budget hiển thị 4000ms; pause/resume không mất thời gian còn lại; undo thay đổi trạng thái thật |
| High | 767/768px, keyboard mobile, safe area | Đúng loại lớp phủ, không tràn/nhảy dock, vùng chạm ≥44px |
| Medium | Reduced motion, mở/đóng nhanh, thay account/route | Không animation mắc kẹt, không lớp hay dữ liệu phiên cũ còn tương tác |

## Xác minh và điều kiện hoàn thành
- Chạy focused kit tests qua `npm test -w platform/ui -- Tabs Sheet escapeLayers Selects Toast mobile`; chỉ thêm regression cho các ranh giới đã đổi. Chạy `npm test -w apps/viderse/web -- Segmented confirm shell`.
- Chạy `npm run typecheck -w platform/ui`, `npm run typecheck -w apps/viderse/web`; nếu script kit khác, đọc package manifest tại thời điểm thực thi, không tự đoán lệnh.
- Browser thật theo `verification.md`: 1440×900, 390×844, thêm 767/768px; normal + reduced motion; screenshots sau font/finite animation và video/trace khi đang chuyển động.
- Shared-kit change phải smoke Noverse selection, sheet và undo để chứng minh không đổi mặc định ứng dụng kia.
- Chỉ merge khi 0 điều khiển thiếu oracle/phản hồi, overlay trong viewport, slider ≤2px; báo cáo PR và cập nhật gap có commit thật.

## Rủi ro, bảo mật và quay lui
Kit dùng chung: không chỉnh theme hoặc thời gian mặc định Noverse. Mọi API giữ auth/CSRF; không đặt credentials vào screenshots/trace. Mutation chỉ trên dữ liệu QA riêng.
Nếu stack primitive không đủ, chứng minh bằng scenario trên rồi sửa tối thiểu; không tạo overlay framework mới.
Rollback bằng revert PR trước PR kế tiếp; không reset dữ liệu hay thay branch người dùng.

## Checklist
- [x] Chốt lại source map và adapter cần dùng, không tái tạo primitive đã đạt.
- [x] Hoàn thành responsive overlay/confirmation và policy toast 4 giây.
- [x] Qua focused checks, browser smoke hai app và toàn bộ gate liên quan; [bằng chứng và giới hạn](reports/pr-01/foundation.md).
- [x] Gửi [báo cáo ảnh/control/motion](reports/pr-01/foundation.md), gap với commit thật; [PR #2](https://github.com/datnt1998/verse/pull/2) merge `a7341d37db67e7cfafdb8035818f700820a11594` ngày 03/11/2026. Actions tắt, không có hosted check; chưa deploy.
