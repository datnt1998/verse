# Viderse — Chuyển từ code hiện tại

Bản đồ từ giao diện **yt-factory** cũ sang thiết kế Viderse: route, thành phần, thiết lập và hành vi đã đổi. Dùng khi port logic từ yt-factory sang verse; đây không phải chuẩn thiết kế. Route trong verse (hash router) ở [02-stack-verse.md](02-stack-verse.md) §4.

## 1. Route

URL cũ được giữ khi có thể, để link đã lưu vẫn chạy.

| Route hiện tại | Trang mới | Ghi chú |
|---|---|---|
| `/` | **Bàn dựng** | `/?channel=<id>&video=<jobId>`; thiếu `video` → `pickDefaultJob` |
| `/clone` | Redirect 308 → `/` | Giữ `channel`; `stage` → lọc nhóm |
| `/clone/[jobId]` | Redirect 308 → `/?video=<jobId>` | Giữ `returnTo` |
| `/review` | **Duyệt** | Giữ `?channel=` |
| `/queue` | **Hàng chờ** | — |
| `/my-channels` | Redirect → kênh dùng gần nhất | Không có kênh → `/my-channels/new` |
| `/my-channels/[id]` | **Kênh** | `?tab=videos|sources|templates|settings`; `design` và `settings` cũ → `settings` (**Hồ sơ kênh**) |
| `/my-channels/new` | **Tạo kênh** (mới) | `?step=create|done`; nhận diện và Mẫu chuẩn chạy nền |
| `/channels`, `/channels/[id]` | **Khám phá** | — |
| `/videos/[id]` | Redirect → Khám phá đã chọn video (`/channels/<id>?video=<id>`) | Trang chi tiết bị cắt; transcript chỉ còn ở CLI |
| `/templates` | **Thư viện** › Template | `?tab=templates` |
| `/templates/backdrop-channels[/id]` | Thư viện › Video nền › Kênh | `?tab=backdrops` · đợt 22 <!-- r22-sources --> |
| Clip cố định trong recipe template | Thư viện › Video nền › Video lẻ, template giữ `fixedVideoIds` | migration Gói V23 |
| Cài đặt › Tài khoản phụ | Cài đặt › Tài khoản nền tảng | `?section=accounts` không đổi |
| `/templates/overlays` | Thư viện › Lớp phủ (mới) | — |
| `/templates/[id]` | **Cài đặt template** | `?section=images|cover|backdrop|output` |
| `/editor` | Redirect → Thư viện | Danh sách project bị cắt; mở editor từ video hoặc template |
| `/editor/[projectId]…` | Editor | Không đổi |
| `/settings` | **Cài đặt** | `?section=tools|connect|agents|accounts|storage|logs`; `images` cũ → `agents` | <!-- r8-ai -->
| `/setup` | **Lần đầu mở** | — |
| `/?guide=1`, tour | Bỏ | Xoá mã tour sau khi V9 xong |

## 2. Thành phần

| Hiện tại | Thay bằng |
|---|---|
| `NavRail` (56px) | `AppHeader` + `MainNav` + `JobsPill` |
| `JobCard`, `/clone/[jobId]` layout | Bàn dựng: `ProgressLog`, `LiveStudio`, `AgentCard`, `Banner` |
| `terminal-panel.tsx` | `ProgressLog` (giữ `cloneTerminal` làm nguồn câu) |
| `job-table.tsx` | Kênh › Video, bảng 4 cột + `BulkBar` |
| `queue-board.tsx` | Trang Hàng chờ mới |
| Tổng quan (hướng dẫn 5 bước, bảng kênh) | Bỏ; Lần đầu mở + EmptyState |
| Tour | Bỏ |
| Màu primary `#f52f39` + chữ tối | Nút chính sáng `--vv-primary` + chữ tối (như Noverse); đỏ `--vv-accent` chỉ là màu nhấn (chọn, bật, focus) <!-- r19-kit --> |
| Geist | Be Vietnam Pro (subset vietnamese) |
| Nhãn ẩn, chỉ icon | Nhãn chữ cho điều hướng và nút chính; icon-only chỉ cho nút phụ (⋯, đóng, tiến/lùi) |

## 3. Thiết lập đổi nhà

| Khoá | Trước | Sau |
|---|---|---|
| `clone.jobConcurrency` | Cài đặt → Clone video | Hàng chờ › Quy tắc chạy |
| `clone.imageConcurrency` | Cài đặt → Clone video | Hàng chờ › Quy tắc chạy |
| khoảng quét lại đối thủ | Cài đặt → Clone video | Hàng chờ › Quy tắc chạy |
| tạm dừng hàng chờ | Cài đặt + `/queue` | Hàng chờ (một nơi) |
| giới hạn mỗi ngày | tab Nguồn | Kênh › Nguồn & tự động, đổi nghĩa thành **nhịp** (chỉ tính video hàng loạt và tự lấy) |
| Thiết kế (tab kênh) + Cài đặt (tab kênh) | template + nhận diện; tên, link, giọng văn | Template luân phiên · **Hồ sơ kênh** (nhận diện + thông tin kênh) |
| `recipe.style` | Cài đặt template › Kiểu ảnh | **Cài đặt template › Cách ghép ảnh vào video** (giữ nguyên trường) | <!-- r9-assets -->
| `recipe.imagePrompt` + `recipe.imagePromptMode` | Cài đặt template › Mô tả phong cách ảnh | **Thư viện › Prompt ảnh** (prompt ảnh cảnh); template chỉ chọn. `imagePromptMode` suy ra: prompt có sẵn = `blend`, prompt của bạn = `custom` |
| `recipe.thumbnail` | Cài đặt template › Kiểu ảnh bìa | **Thư viện › Prompt ảnh** (prompt ảnh bìa); template chọn `coverPromptId` |
| Nhóm **Bản cũ** (`legacy`) | bộ lọc riêng | Job cũ có MP4 → Sẵn sàng / Đã đăng (chỉ đọc); job cũ dở → liệt kê trong Cài đặt › Lưu trữ để xoá |

## 4. Hành vi đổi

1. **Bỏ “bắt đầu thẳng không qua hàng chờ”.** Mọi video vào hàng chờ; video chọn tay là `priority: "pick"` và lên đầu. Còn luồng thì bắt đầu ngay.
2. **Giới hạn mỗi ngày → nhịp.** Video chọn tay không còn tính vào giới hạn.
3. **Hết hạn mức ảnh không còn là Cần xử lý.** Video ở Đang làm với dòng phụ chờ giờ đặt lại; nút “Thử lại ngay” cho hạn mức bị bỏ.
4. **Thêm Tạm dừng / Chạy tiếp** cho từng video (khác Huỷ).
5. **Tạo kênh bắt buộc có ít nhất một đối thủ** (chỉ trên UI).
6. **“Duyệt & xuất” → “Duyệt cả video & dựng”**: hành vi không đổi, chỉ đổi tên.
7. **Thao tác hàng loạt chuyển về trang Kênh**; “Mọi video của 3 kênh” thay `/clone` không lọc.

## 5. Phần giữ nguyên
- Step chain, manifest, job edit, quality gate (≥ 90).
- CLI `ytf`: thêm `clone pause|resume` và `--priority` cho `clone batch`, không đổi lệnh cũ.
- Editor và mật độ của editor.
- Chính sách chỉ chạy local (`127.0.0.1`) cho tới khi chốt D1 ([data-contract §10](07-data-contract.md#10--truy-cập-từ-điện-thoại-chờ-quyết-định-d1)).

<!-- r3 -->
