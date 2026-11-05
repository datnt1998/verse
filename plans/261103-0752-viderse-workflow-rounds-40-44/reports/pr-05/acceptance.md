# V39 — Nhập nhanh, Khám phá và Hàng chờ

Ngày kiểm: 05/11/2026. Chưa deploy. V40 và release gates vẫn mở.

## Màn → đã sửa / còn lệch

| Màn / hợp đồng | Đã sửa | Bằng chứng / giới hạn |
|---|---|---|
| Làm video PC / MNew | Một owner lazy ở shell; N không cướp input/overlay; link, nhiều link và selection; chọn kênh/template, mặc định kênh và override riêng; huỷ không tạo việc | [Native UI](native-ui.json), [selection từ Khám phá](composer-explore.json), [20 cặp tham chiếu](comparisons.json). Dữ liệu QA khác ví dụ thiết kế; không chứng nhận pixel equality. |
| Focus ô link | Dùng kit-frame để bỏ outline vuông do global CSS thắng StyleX layer; ring bo tròn inset không bị vùng cuộn cắt | [Kiểm bàn phím/computed style và ảnh trước–sau](focus-border-repair/results.json): PC/mobile2/2, Tab/Shift+Tab giữ focus. Các ảnh cũ có hai viền được giữ làm lịch sử, không phải bản sau sửa. |
| Khám phá / MExplore / MCompVideo | Chip desktop nổi/mobile Sheet; bulk vào cùng Nhập nhanh; MComp direct add và secondary handoff; bỏ Why | [32 vectors và control ledger](explore-acceptance/manifest.json). Response fixtures được ghi rõ, không coi là provider/persistence thật. Lỗi length mobile đã sửa và [kiểm sau sửa](post-fix-surfaces.json). |
| Hàng chờ / MQueue | Thứ tự toàn cục, ordinal không đánh lại khi lọc, workers1–6, cadence theo kênh; summary điện thoại không còn imageWorkers/rescan | [11 vectors/124 actions](queue-acceptance/manifest.json), [kiểm sau sửa](post-fix-surfaces.json). Hai lỗi ghi trong manifest là bản trước sửa; source cuối và postfix receipt thay thế. |
| API / PostgreSQL | POST /api/jobs, dedup(source,kênh), queueItemId và mapping sau admission, override không đổi default; prepend global serialize trước cap; migration41 giữ in-flight; restart duplicate ưu tiên pending | [Native API12ca](native-api.json), suite PostgreSQL và race. Metadata nguồn là cache fixture khai báo rõ; không truy cập YouTube/provider. |
| Native consumer | PC override + phone default thật qua POST jobs, GET queue/mapping và reload Bàn dựng; CLI run/batch thật trả duplicate đúng | [UI lưu/readback2ca](native-ui-save-readback.json), [CLI2ca](native-cli.json). Worker bị fence; không vẽ/download/cấp phát job mới. |

## Kiểm tra đã chạy

- Full frontend441file:3656qua/4skip; sau đó thay đúng markup kit-frame và ring inset, focused New14qua, typecheck/Vite/native build qua, kiểm native focus2/2. Không gán lần full trước cho source style cuối.
- Full Go `./apps/viderse/...` qua với DB QA regression riêng và SQLC model overlay; sau sửa pending-restart duplicate, nhóm real-PG/race dedup+concurrency qua. `go vet` toàn Viderse qua.
- Native API12ca, UI POST/readback2ca, CLI run/batch2ca. Ledger vẫn3job/27step/2paid receipt; enqueue QA riêng không có inference.
- Canonical63ảnh có props thực, SHA nguồn và font local. Không biến số ảnh canonical thành số state app pixel-pass.
- Hai review source có phạm vi: queue phát hiện duplicate restart và đã sửa bằng regression real-PG; flow không thêm blocker. Cùng họ model, không tuyên bố review khác họ độc lập.
- Cảnh báo React act(), Vite annotation/chunk size vẫn có; không tắt hoặc che failure.

## Ranh giới và phát hành

Không sửa user index hoặc user SQLC models.go; compile dùng model tái sinh trong overlay. Không sửa artboard/credential/source app DB. Bản production còn revision cũ, readyz OK không đồng nghĩa đã deploy.

V40 giữ gate mobile tổng, motion/keyboard và Safari thiết bị thật; chưa gọi toàn workflow1:1 hoặc phát hành xong. Round45 viewer mobile không làm. Trước rollout phải backup/quiesce writer và migrate41 theo route operator hiện có.
