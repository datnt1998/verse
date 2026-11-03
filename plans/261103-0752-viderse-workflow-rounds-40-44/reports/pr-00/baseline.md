# Bước 0 · Baseline Viderse trước refactor

Ngày: 2026-11-03. Revision nguồn lúc chụp: `c34cd512002af842ba48532adcc66f026038e51f`.

**Đây là kiểm kê và ảnh code cũ, không phải nghiệm thu V36–V40. Chưa sửa UI sản phẩm, chưa deploy.**

## Bằng chứng đã chạy

| Hạng mục | Kết quả quan sát | Giới hạn |
|---|---|---|
| Trích footer artboard | 480 giá trị enum/boolean, 12 probe số/text, 42 board được phân loại | Probe chưa được seed/chạy trên app |
| `node tests/viderse-workflow/artboard-state-manifest.mjs --check` | Exit 0; 480 rows, 12 probes | Chỉ kiểm inventory, không chứng minh UI |
| Reference thật qua `__dcSetProps` | 480/480 ảnh với defaults + context + patch, readback props; font loaded; 0 overflow ngang; 0 lỗi render | Có context cha và framing cho trạng thái dưới fold; chưa mọi tổ hợp hoặc visual oracle cho từng row |
| QA | 2 DB mới riêng, HOME/media/blob riêng, 3 kênh, 3 video, mỗi video 7 ảnh; FFmpeg + cwebp tạo media thật | Fixture có nhãn tổng hợp; không chứng minh AI sinh ảnh hoặc fallback |
| Baseline code | Playwright chụp 28 board ở viewport thiết kế 1440×900 / 390×844, DPR 1, font loaded, 0 overflow; 6 board ghi rõ thiếu target prerequisite | Những board thiếu chỉ có route nền, không phải ảnh target ready |
| Control inventory | 577 control hiện hữu, 553 enabled trong receipt Playwright cuối | Chưa chạy audit mọi control; không được gọi “0 nút chết” |
| Smoke tương tác | Ảnh sau → đúng ảnh kế và URL artifact khác; Ảnh trước → khôi phục ảnh cũ; Chỉ dẫn đã gửi → sheet vào màn, Esc tháo sheet và trả focus | 3 control trên Chromium mobile emulation; chưa Safari thiết bị thật, chưa full motion/reduced-motion |
| Preflight production | GET channels/templates/notification settings/queue đều 200; 1 kênh, 2 template, 0 rotation rỗng/dangling, 0 nhịp ngoài 1–20, 0 notification exception, 0 queued/running | Không ghi production. Snapshot trong DB và effective autonomy tại admission chưa được kiểm |
| Regression local trước merge | Typecheck cả 3 workspace; Go vet; unit UI 239, Noverse 1894, Viderse 3700 pass / 4 skip có sẵn; focused export dialog 3 pass | GitHub Actions đang tắt, không claim hosted CI xanh hoặc đã chạy lại mọi task trong CI recipe |

Nguồn máy đọc: [state matrix](../artboard-state-matrix.json), [runtime trace index](capture-index.json), [reference captures](reference-captures.json), [app baseline](app-baseline.json), [comparison paths](comparisons.json), [control smoke](control-smoke.json), [commands](commands.json), [local gates và test cleanup](local-gates.json), [isolation](isolation.json), [legacy preflight](legacy-preflight.json), [preview prerequisite](template-preview-prerequisite.json).

## Bảng màn → hiện trạng và owner sửa

| Màn/component | Hiện trạng trong baseline | Gói/PR chịu trách nhiệm |
|---|---|---|
| Segmented, popover/sheet, confirm, toast | Inventory source có primitive dùng lại; chưa đạt gate font/overlay/4 giây | Nền / PR1 |
| Review, Editor, Template, ảnh tại Main/Library | Media QA đọc được; chưa shared viewer với image address đầy đủ | V36 / PR2; phần Editor/Template V37 / PR3 |
| Main, Activity, Dialogs, System | Main còn Bản tin/composer/4 tab; Activity/Dialogs/System chưa mở đủ target overlay | V37 / PR3 và owner xác nhận tương ứng |
| Channel, CreateChannel | Còn Video/rotation/autonomy/priority; dữ liệu QA default template đúng brief | V38 / PR4 |
| Onboard | Account QA có 3 kênh nên route mở form CreateChannel, chưa phải wizard account chưa có kênh | V38 / PR4; prerequisite thiếu, không gọi target match |
| Library, MLibrary, MTemplate | Contract/bố cục cũ; preview API trả state=none, sources=[]; chưa có source hoàn tất đủ điều kiện | V38 / PR4, mobile V40 / PR6 |
| NewVideo, Explore, Queue | Còn trang tạo cũ; chưa canonical jobs API hoặc hàng đợi prepend mới | V39 / PR5 |
| MMain, MReview, MChannel, MNew, MExplore, MQueue, MSettings, MCreate, MAuth | Có baseline; chưa parity đợt44. MReview đã chụp lại sau khi ảnh thực tải xong | V40 / PR6 |
| MCompVideo | Route baseline chỉ mở Khám phá; thiếu rival/video QA để mở detail đúng artboard | V39 / PR5 và V40 / PR6; **không coi là ảnh đối chiếu đúng trạng thái** |
| MSystem, Dialogs, Activity | Route nền có ảnh, lớp phủ target chưa được mở/seed đầy đủ | Owner ở state matrix; **không coi là target match** |

Bảng code ↔ artboard ↔ giữ/sửa/xoá đầy đủ: [inventory](../inventory.md). Không đổi file artboard. Mobile shared viewer đợt45 và mobile Editor vẫn nằm ngoài scope đã chốt.

## Cách đọc ảnh

`reference/` chứa ảnh render thật theo từng row có context cha; một số row cần cuộn tới nội dung và receipt có target/offset. `app/` chứa code cũ đã đợi chunk/API/ảnh; từng board có context Playwright mới. `comparison/` đặt reference bên trái, code bên phải, cùng tỷ lệ. `reference-selection.json` chọn row tương ứng route/data baseline; `comparisons.json` ghi target prerequisite thiếu. Không gọi row `isDefault=true` là toàn bộ defaults nếu context đã đổi view cha.

Các màn chỉ có artboard desktop dùng 1440×900; các màn M* dùng 390×844. Không kéo giãn artboard desktop thành mẫu điện thoại. Những trạng thái chưa thể biểu diễn trong app vẫn pending trong matrix, không tự đổi DOM/state để làm ảnh đẹp.

## Lỗi/lượt thử được giữ nguyên

- Lượt seed đầu: FFmpeg trên máy thiếu encoder libwebp. Fixture chuyển sang tạo PNG rồi dùng binary cwebp có sẵn để tạo WebP hợp lệ.
- Lượt seed thứ hai: build từ chối version PNG vì contract yêu cầu WebP. Đã sửa fixture đúng contract, tạo lại **chỉ hai DB QA do phiên này sở hữu**; lượt seed sau exit 0, không đổi validator sản phẩm.
- Lượt chụp MChannel đầu timeout khi đợi animation; báo cáo này giữ lịch sử lỗi, receipt cuối là lượt Playwright mới đã hoàn tất, không trộn hai lượt.
- Review độc lập phát hiện 10 desktop loading skeleton và MReview/MSettings chưa ready. Đã bỏ các ảnh đó, chụp lại 28 board bằng Playwright headless; gate đợi route, bỏ Suspense aria-busy, API kết thúc, font/ảnh decode và animation hữu hạn. Không dùng độ dài text để kết luận ready.
- Review phát hiện defaults+patch giấu các tweak sau tab/screen/role/step khác. Đã thêm context từ render conditions và framing để thấy recovery, usage server, inbox/jobs, metadata và lỗi S3 dưới fold. Readback props không thay cho visual oracle; receipt vẫn `visibleStateVerified:false` và `productAcceptance:false`.
- Managed-browser đầu dùng profile tồn tại và viewport helper khác raw page; đã thay bằng profile Playwright tạm mới, context riêng từng board. MAuth có context rỗng thật, không mang banner hết phiên từ localStorage cũ. Các cửa sổ Chrome QA native đã đóng; không đụng Chrome người dùng.
- Gate API-idle nghiêm hơn tái hiện request `/api/setup/state` tồn đọng sau reload/chuyển route trong page dùng chung. Không bỏ qua endpoint hoặc nới timeout: mỗi baseline board chuyển sang context mới; sequence Review/Main/Template qua 3/3, sau đó full app qua 28/28. Lượt lỗi và nguyên nhân nằm trong [failure history](failure-history.json).
- Receipt cuối ghi 4 HTTP lỗi có board và expected flag: 2 GET `/api/storage/server` 404 ở kho local (route không có theo storage-mode contract), 2 GET `/api/me` 401 ở MAuth anonymous. Không có HTTP lỗi ngoài tập đã giải thích.

## Gate còn mở

- Tổ hợp phụ thuộc, probe số/text, preview ready/building, discovery detail, overlay và mọi control/oracle chưa đủ. PR0 chưa có full-control Playwright pass.
- Chưa có ảnh/video chứng minh normal + reduced-motion cho toàn bộ workflow, slider ≤2px sau font, hit area ≥44px trên mọi control, hay Safari keyboard/dock thật.
- Chưa có kiểm chứng provider QA có hạn mức được duyệt; không gọi provider production để lấp thiếu này.
- Commit inventory/QA thật: `d245db8c38c95d0d9d3c98cf84f24294b7c190e9`; [PR riêng #1](https://github.com/datnt1998/verse/pull/1), chưa merge. Actions của repo đang tắt, giữ nguyên cấu hình và dùng gate local đã ghi rõ; PR1 chỉ bắt đầu sau PR0 merge.
- Chưa phát sinh câu hỏi policy legacy có dữ liệu thực buộc phải quyết định; không tự clamp nhịp, đổi template hay đổi notification preference.
