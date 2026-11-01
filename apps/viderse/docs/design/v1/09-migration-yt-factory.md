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
- Truy cập từ điện thoại: D1 đã chốt cách (a) — LAN + ghép thiết bị bằng QR ([data-contract §10](07-data-contract.md)). <!-- r32 -->
- Không nhập dữ liệu yt-factory: Viderse bản verse bắt đầu trắng (#9).

<!-- r3 -->

## 6. Việc cần tắt ở yt-factory khi chuyển hẳn (phase 10)

Đọc từ mã nguồn `/Users/dat/Personal/yt-factory` tại commit `2c201e6` (`git show`). **Chưa tắt gì** — danh sách này để chủ máy tắt theo thứ tự. Lý do: hai hệ cùng quét đối thủ thì tốn gấp đôi lượt đọc, cùng vẽ thì tốn gấp đôi hạn mức ảnh trả phí.

| # | Thứ cần tắt | Ở đâu trong mã | Cách nhận biết / kiểm |
|---|---|---|---|
| 1 | **App desktop “YT Factory”** (`local.ytfactory.app`, Electron 44.7.2) | `desktop/main.mjs` — app tự chạy `next start -H 127.0.0.1 -p 3220` bằng Node đóng gói, `cwd` trong `~/Library/Application Support/YT Factory/app`, `YTF_DATA_DIR=…/YT Factory/data`, pid ở `server.pid`, log `server.log`; thoát app thì gửi SIGTERM rồi SIGKILL cả nhóm process | Thoát app; `lsof -iTCP:3220 -sTCP:LISTEN` phải rỗng; không còn process nào chứa `Application Support/YT Factory/app` |
| 2 | **Máy chủ web yt-factory** nếu chạy ngoài app | `package.json` › `start`: `next start -H 127.0.0.1 -p 3210` | `lsof -iTCP:3210 -sTCP:LISTEN` rỗng |
| 3 | **Worker dựng video** (mỗi job một process, giữ lease trong SQLite nên vẫn vẽ tiếp dù máy chủ đã chết) | `src/server/clone/worker.ts --job <id> --worker <id> --handlers <spec>`, spawn bởi `ensureWorker` (`src/server/clone/runner.ts:607`, gọi ở `launch.ts:71`) | `pgrep -fl "clone/worker.ts"` rỗng |
| 4 | **Worker quét đối thủ** (process riêng, giữ lease `worker`, tự đặt nhịp 1–2 giây/request) | `src/server/channels/scan-worker-entry.ts --scan-worker --worker <id>`, spawn bởi `requestScanWorker` (`src/server/channels/scan-worker.ts:656`) từ nhịp 60 giây ở `src/instrumentation.ts:53` và từ thao tác trong `channels/service.ts` | `pgrep -fl "scan-worker-entry.ts"` rỗng |
| 5 | **Bộ lập lịch trong process máy chủ**: nhịp 60 giây `drainCloneQueue` (bắt đầu job), nhịp **15 phút `runAutoPick`** (tự thêm video đối thủ — chính là “quét đối thủ”), dọn dẹp mỗi 6 giờ | `src/instrumentation.ts` (`register`) | Tắt máy chủ là hết; muốn giữ giao diện mà không quét/không vẽ thì đặt `YTF_QUEUE_AUTODRAIN=0` khi chạy máy chủ yt-factory (`register` thoát sớm) |
| 6 | **Lệnh CLI `ytf` có vẽ/quét** nếu có cron hay alias gọi: `ytf clone run|batch`, `ytf channel scan <…> --history`, `ytf channel scan-control`, `ytf backdrop refresh`, `ytf tools image-provider`, `ytf template …` | `cli/ytf.ts` (inventory đầy đủ ở `USAGE`) | `crontab -l`, `launchctl list \| grep -i ytf` không còn dòng nào |
| 7 | **Process con mồ côi**: Chrome for Testing và ffmpeg do yt-factory bật | `src/server/tools/*`, `src/server/clone/export-launcher.ts` | `pgrep -fl "Chrome for Testing"`, `pgrep -fl ffmpeg` — chỉ còn process của Viderse (nếu đang chạy) |

Thứ tự đề nghị: (1) → (3) → (4) → (2) → kiểm (7) → chỉ sau đó bật Viderse live và chuyển. Khi đã tắt xong, `pgrep -fl "yt-factory\|clone/worker\|scan-worker-entry"` phải rỗng.

**Giữ nguyên, không xoá:** repo yt-factory, `data/app.db` (SQLite, `YTF_DATA_DIR` mặc định là `<cwd>/data`), thư mục `~/Library/Application Support/YT Factory/`, project editor và bản xuất cũ — phase 10 chỉ ngừng chạy, không di trú và không xoá dữ liệu.

