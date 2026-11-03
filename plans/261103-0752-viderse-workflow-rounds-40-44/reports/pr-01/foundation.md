# Nền UI dùng chung — bằng chứng PR1

Trạng thái: **local verification đã qua; đang chốt PR, chưa deploy**. PR kiểm kê [#1](https://github.com/datnt1998/verse/pull/1) đã merge `f120a62c8ce664b8e4c42f8edda6a74a24e55d91` ngày 03/11/2026.

## Màn/component → thay đổi / còn lệch

| Owner thật | Đã thay đổi | Phần vẫn thuộc gói sau |
|---|---|---|
| `platform/ui/src/Tabs.tsx`, `components/Segmented.tsx` | Giữ đo nhóm/từng nút/font của kit, không offset bù. Browser sau font/resize đạt ≤2 px; native smoke các cạnh 0 px. | Mọi consumer/tweak theo owner V36–V40, không suy từ smoke ra toàn app đạt. |
| `components/responsive-overlays.tsx` | Select/ChipSelect/ActionMenu nổi trên desktop, Sheet dưới 768 px; cùng lựa chọn; vùng chạm ≥44 px. Callback đợi sheet gỡ focus/scroll lock. | Màn Nhập nhanh, dữ liệu/avatar của lựa chọn và bố cục target đổi ở PR5/PR6. Không ép RichSelect/Combobox thành adapter chữ. |
| `Sheet.tsx`, `ConfirmDialog.tsx` | Handoff sau commit; xác nhận responsive riêng Viderse; giữ form/pending/lỗi/phrase; chặn nhấn đôi/Esc/scrim trong lúc gửi; trả focus/scroll; dọn khi đổi phiên. | Cắt form/workflow obsolete theo V37/V39. Không tuyên bố mọi business action đã nối đúng. |
| `Toast.tsx`, `main.tsx`, `App.tsx` | Viderse 4000 ms; explicit duration ưu tiên; Noverse giữ mặc định; thẻ việc nền không tự mất. Sửa hold hover/focus, keyed collapse, in-flight undo/cap và kết quả cũ sau đổi phiên. | Native timing là UI Kit, không phải bằng chứng hoàn tác business mutation. |
| `ChangePasswordScreen.tsx` | Bỏ toast ngay trước thu hồi phiên; dùng banner lý do kết thúc phiên đã có ở đăng nhập. | Không đổi API/mật khẩu/quyền/provider. |

## Kiểm tra thực chạy

| Gate | Kết quả |
|---|---|
| Kit focused cuối | 5 file / 42 ca qua |
| Viderse adapter/App/password focused cuối | 3 file / 18 ca qua |
| Full kit | 33 file / 259 ca qua |
| Full Noverse | 227 file / 1894 ca qua |
| Full Viderse | 453 file / 3709 ca qua; 4 skip có sẵn |
| Typecheck | Cả ba workspace qua |
| Build Viderse | Qua; còn cảnh báo chunk lớn có sẵn |
| Playwright desktop/mobile × normal/reduced | 12/12 qua, 0 flaky/skip/unexpected; 96.6 giây |
| Chương trình Go chứa UI build thật | `/` và `/api/me` 200; mobile font loaded, slider 0 px, width 390/390, không page error/request failure |

Ngoài workspace đang dùng, đã export chính xác proposed Git tree `8dc94957c1cc89519a22a6c4123add8beefcf004` sang thư mục tạm (không tạo worktree), chạy Viderse typecheck qua trong 51.27 giây. Bước này tránh lấy nhầm file từ ordinary index cũ làm điều kiện compile.

Lượt root test chạy đồng thời build ban đầu có timeout Noverse; không tính là xanh. Hai file ảnh hưởng chạy lại qua 9 ca, rồi toàn bộ Noverse qua riêng. Build đầu timeout khi tranh tài nguyên; build riêng sau đó qua. Lịch sử browser lỗi môi trường/readiness/locator được giữ trong `runs/`; kết quả chuẩn cuối là [run 07](runs/07-settled-foundation/interaction-results.json). Không nới tolerance hoặc sửa Tabs để che thời điểm đang animation.

[Receipt và hash source](receipt.json) · [báo cáo Playwright](runs/07-settled-foundation/html/index.html) · [native timing và lỗi action thật](native-feedback.json) · [compiled smoke](compiled-smoke.json) · [ảnh compiled mobile](compiled-mobile.png) · [quan sát resize theo frame](segment-resize-observation.json).

Native UI Kit: Viderse toast khoảng 4001 ms; Noverse toast 5999 ms, undo 5003 ms, đúng fallback riêng. Regression mounted kiểm tra pause/resume, thao tác bất đồng bộ lỗi/thử lại, keyed replacement, pending lock và dọn phiên. Không gọi các demo này là business undo thành công.

## Ảnh đối chiếu

| Viewport | Chọn kênh | Chọn template |
|---|---|---|
| 1440×900 | [Artboard cạnh UI thật](comparisons/desktop-channel.png) | [Artboard cạnh UI thật](comparisons/desktop-template.png) |
| 390×844 | [Artboard cạnh UI thật](comparisons/mobile-channel.png) | [Artboard cạnh UI thật](comparisons/mobile-template.png) |

Ảnh cho thấy rõ **chưa đạt toàn màn 1:1**: New vẫn là trang cũ, target là Nhập nhanh; cutover thuộc PR5/PR6. PR1 chứng minh loại overlay, chọn đổi ngay, focus/scroll, viewport và motion, không nghiệm thu thay gói tương lai. UK.0–UK.8 là kit hiện hành; Kit/MKit đã archive, không lấy làm target mới.

## Lỗi action đã xác định và owner

Menu video → xác nhận xoá → gửi thật trả `Thao tác làm video không hợp lệ.`; job đọc lại vẫn 200. Frontend đang POST action `delete`, trong khi server đã có HTTP DELETE. Đã đưa sửa và bằng chứng xoá/404 vào phase V36 (PR2). PR1 đã chứng minh pending không đóng/nhấn đôi, lỗi giữ form; **chưa xoá thành công**, không toast thành công giả.

## Giới hạn và an toàn

- Không sửa artboards; không gọi provider, không đổi queue/provider production. QA dùng DB/HOME/session riêng.
- Không lưu auth trace; screenshot/video và semantic receipt không chứa cookie/password.
- Lượt browser môi trường cũ hỏng sau tool timeout mất Python globals và TemporaryDirectory; đã dừng service cũ, dựng QA riêng và chạy lại. Runtime mới có cleanup tường minh, không phụ thuộc finalizer.
- GitHub Actions đang tắt; chỉ có local gates đã thực chạy, không có hosted CI xanh.
- Chưa kiểm Safari iPhone bàn phím/dock thật. Chromium emulation không thay bằng chứng đó; gate cuối giữ rõ giới hạn.
- Nội dung cũ Bản tin/luân phiên/ưu tiên còn theo inventory và owner PR tiếp theo. Không ghi V36–V40 hoàn tất trước cutover.
