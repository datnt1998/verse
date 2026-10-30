# P3 — ExploreWeb (Khám phá / Viderse phase 03)

Status: DONE_WITH_CONCERNS

## Đã làm

- `api/rivals.ts`: client đủ wire của `/api/rivals*` (list/add/remove, videos phân trang cursor, scan + `start|pause|resume|cancel|check`, `PATCH historyCap`, posting-slots `zone=vn|ny`), `apiErrorInfo` đọc envelope `{error,code,recovery,retryAt,rival}`.
- `data/keys.ts` + `data/eventKeys.ts`: khoá `rivals/rivalVideos/rivalScan/rivalPosting`; SSE `rivals` map về `[['rivals']]`; `SERVER_EVENT_TYPES` thêm `rivals`.
- `i18n/vi.ts`: khối `screens.explore.*` (~90 chuỗi) — trạng thái quét, lỗi thêm đối thủ theo `code`, lý do khoá, lịch đăng, chọn nhiều.
- Màn `screens/explore/**`: `ExploreScreen` (3 cột 252 / giữa / 316), `RivalSidebar`, `AddRivalForm`, `FilterBar`, `VideoGrid` + `VideoCard`, `useVirtualGrid` (tự viết, không thêm phụ thuộc), `useRivalVideos`, `RivalCard`, `ScanCard`, `scanView`, `PostingCard`, `posting`, `PreviewPanel`, `BulkBar`, `fmt`, `explore.css`.
- Token mới trong `styles/viderse.css`: `--vv-s-11/76/132/160/240/250`, `--vv-thumb-veil`, `--vv-heat-empty`, `--vv-check-ring`, `--vv-line-28/35/70`, `--vv-wash-6/8`, `--vv-tile-hover`, `--vv-shadow-bulk`, `--vv-r-3`, `--vv-rav-1..5`.
- Không sửa kit `platform/ui`, không sửa Noverse.

## Số đo (Chrome headless thật, DB `viderse_smoke_explore`, 20.000 video)

| Hạng mục | Kết quả |
|---|---|
| 1440×900, tải lần đầu | 18 thẻ DOM, 6 hàng, 3 cột, `clientHeight` 845 / `scrollHeight` 18.295 |
| Cuộn 0 → video ~9.710 (bước 4.000 px) | tối đa **27 thẻ DOM** (min 15), 0 tràn ngang |
| `longtask` (PerformanceObserver, buffered) | **0 task > 50 ms**, task dài nhất 0 (không có longtask) |
| 390×844 | **4 thẻ DOM**, 1 cột, tràn ngang 0, `clientHeight` 608 / `scrollHeight` 64.929 |
| `npm run size` | 128,4 kB gzip (ngưỡng 250) — chunk Khám phá 12,44 kB gzip |

Trước khi sửa bản ≤767 px: 240 thẻ/240 hàng, cols=1 và `scrollHeight` 48.520 (MobileShell không tự cuộn). Đã sửa: media query 767 px, `.vx-center { overflow-y:auto; max-height:72dvh }`; test mới cho đường 390×844.

## Cặp ảnh (đều 1440×900, `/tmp/viderse-explore/shots/`)

| Tweak state | Artboard | App | Khác biệt |
|---|---|---|---|
| empty | `art-S-empty.png` | — | Chưa chụp được: cần DB thứ hai không có đối thủ; copy/CTA đã có trong `screens.explore` và test render trạng thái trống |
| mặc định | `art-default.png` | `app-1440-default.png` | Ảnh thu nhỏ trống vì `allowed_domains` chặn `i.ytimg.com` (giới hạn harness); lưới 3 cột, cột trái 252, rail 316 khớp |
| preview | `art-S-preview.png` | `app-1440-preview.png` | Khớp khối; nút chính khoá (đúng hợp đồng) |
| preview + why | `art-S-preview-why.png` | `app-1440-preview-why.png` | Nội dung facts khớp (1,2 Tr · 286 N · ×3 · giờ đăng); mẫu số dùng 150 video thay vì 30 |
| select | `art-S-select.png` | `app-1440-select.png` | Thanh chọn "Đã chọn 3 · Top 10 · Top 20 · Bỏ chọn · Sao chép link · Làm 3 video" khớp |
| rivalMenu | `art-S-rivalmenu.png` | `app-1440-rivalmenu.png` | 4 mục khớp; hint "không đợi chu kỳ 1 giờ" lấy từ `rescanHours` |
| scan=running | `art-default.png` (mặc định) | — | Chưa chụp riêng; bản ánh xạ đã test bảng |
| scan=paused | `art-S-scan-paused.png` | `app-1440-scan-paused.png` | Khớp: "Đã tạm dừng…" + nút Tiếp tục |
| scan=limited | `art-S-scan-limited.png` | `app-1440-scan-limited.png` | Khớp: "YouTube đang giới hạn tốc độ… tự thử lại lúc HH:MM" |
| scan=done | `art-S-scan-done.png` | `app-1440-scan-done.png` | Khớp: "Đã quét đủ giới hạn…" + nút Quét lại |
| cuộn giữa danh sách | — | `app-1440-scrolled.png` | 27 thẻ, không giật |
| điện thoại | — | `app-390-default.png` | 1 cột, không tràn ngang |

## Khác biệt đã sửa trong lượt này

1. Cửa sổ cuộn ≤767 px không bị chặn → mount cả danh sách (240 thẻ). Nay `max-height:72dvh` + `overflow-y:auto`, đo lại còn 4 thẻ ở 390×844.
2. Số "đã quét" vượt giới hạn hiện `20.000 / 2.000` → kẹp theo cap, hiện `2.000 / 2.000`.
3. Hai chỗ viết tay khoảng cách hàng 14 px → dùng `GRID_GAP_Y`.
4. Segmented "Sắp xếp" / "VN · New York" bị co: thêm `flex:'0 0 auto'`; phần còn lại là hành vi chia đều nhãn của kit.

## Tests

- `npx tsc --noEmit -p .` sạch.
- `npx vitest run --maxWorkers=2` → **213/213 pass, 19 file**. Mới: windowing 20.000 (desktop) + 390×844, phân trang cursor 200→240, multi-select Top 10/20/Bỏ chọn sống qua SSE, mọi mã lỗi thêm đối thủ, `slotCells` (qua nửa đêm, phút lẻ), `scanView` 14 trường hợp, lý do khoá qua tooltip, preview + why.
- `npm run build` OK; `npm run size` 128,4 kB gzip.
- `src/checks/design-rules.test.ts` xanh (không hex/không bán kính cứng/không kẻ một bên/không khoảng cách số cứng).

## Hỏi-đáp / còn mở

1. Segmented của kit chia đều nhãn nên "Mới nhất"/"New York" bị ba chấm ở bề rộng hiện tại; cần kit cho auto-width hoặc nhãn ngắn hơn.
2. Ảnh thu nhỏ không tải trong harness (`allowed_domains` chỉ 127.0.0.1) — không phải lỗi app.
3. Chưa chụp `empty` và `running` riêng; `preview`/`why`/`select`/`menu`/`paused`/`limited`/`done` đã có cặp.
4. Dữ liệu seed giờ đăng đều nhau nên Lịch đăng hiện "0 khung lặp lại", "Mỗi tuần 30 video" (thuật toán server, không phải UI).
5. Nút "Liên kết với kênh của tôi" và lọc Trạng thái bị khoá kèm lý do (chưa có kênh sở hữu); "chapters" ẩn vì chưa có dữ liệu.
6. Bộ lọc sort/long/within là state cục bộ (router chỉ chuẩn hoá `?rival=`/`?video=`); `?rival=<handle>` dùng cho phạm vi.
