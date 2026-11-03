---
phase: 1
title: "Bước 0 · Kiểm kê và chốt baseline"
status: in-progress
priority: P1
dependencies: []
---

# Bước 0 · Kiểm kê — PR inventory/QA riêng, trước sửa UI sản phẩm

## Mục tiêu
Gửi bảng code ↔ artboard ↔ V36–V40 ↔ giữ/sửa/xoá, xác nhận hiện trạng và đường nghiệm thu có thể tái lập. Không sửa runtime/UI sản phẩm ở bước này.

## Nguồn và deliverable
- [Bảng kiểm kê](reports/inventory.md), [hợp đồng backend](backend-contracts.md), [giao thức nghiệm thu](verification.md).
- Đọc `apps/viderse/docs/design/v1/artboards/Workflow.dc.html` mục 01–06; `01-ia-flows.md` §0.3/§6.1/§6.2 khối Đợt44 và W3–W13; `08-dev-prompt.md` V36–V40, `artboards/README.md`, footer data-props, 05-kit/06-copy/07-data-contract/02-stack-verse/13-changelog.
- Không sửa bất kỳ file trong `docs/design/v1/artboards/`. Các tài liệu cũ còn nhắc feature đã bỏ chỉ là lịch sử, không được thắng board.

## File sở hữu và việc làm
1. Re-scout từng package trước PR của nó vì thời gian giữa các PR có thể làm source map đổi; đổi exported symbol phải references mọi consumer. Bảng inventory hiện có là baseline, không phải chứng minh UI đã đạt.
1a. Trước tạo/chạy fixture hay logout/draw: lập isolation receipt không-secret cho app DSN, identity DSN, HOME/config/auth.DefaultStore, media/blob và browser storage. QA login + app DB riêng không tách machine provider credentials; thiếu một ranh giới thì không mutation, không dùng credential production để lấp chỗ thiếu.
2. Tạo `reports/artboard-state-matrix.json` trong plan này bằng đọc footer thực, đối chiếu bảng tweak. Map mỗi giá trị tới app state/seed/route/control oracle/PR owner. Không tạo props thử nghiệm trong app chỉ để giống artboard.
3. Chụp baseline source hiện tại và reference ở 1440×900, 390×844 theo `verification.md`, trên dữ liệu QA riêng. Lưu `reports/pr-00/` cùng command/PID/port/cwd và revision. Không giả ảnh comparison khi chỉ mới mở reference.
4. Lập ledger route/caller/backend contracts: top-nav/More, trang new→dialog, channel videos route, create APIs, rotation/autonomy fields, removed data-key endpoint tên thực tế.
5. Preflight chỉ đọc cho legacy data: empty/dangling template, perDay>20, resolved autonomy + AutoPick, persisted job snapshots và queue explicit template IDs, notification prefs mixed/per-channel. Nếu có policy thiếu trong thiết kế, ghi câu hỏi mới theo số tiếp theo trong `11-hoi-dap.md`, gửi chủ thiết kế và chờ; không tự sửa dữ liệu.
6. Gửi đúng bảng kiểm kê và baseline report cho người dùng trước bất kỳ sửa source ở PR nền. PR0 được merge riêng; không gộp với implementation.

## Ma trận tình huống và rủi ro
| Mức | Cần kiểm | Điều kiện qua |
|---|---|---|
| Critical | Source authority lệch giữa Workflow và docs cũ | Mỗi row ghi nguồn đích, không copy tính năng cũ vào scope |
| Critical | Schema/template migration có dữ liệu ngoài thiết kế | Preflight đầy đủ hoặc stop có câu hỏi; không silent clamp/fallback |
| High | Tweak có trong HTML nhưng thiếu README/API | Ghi coverage gap và owner, không đánh pass hoặc tự fake API |
| High | Kit đã hỗ trợ requirement, callsite vẫn lỗi | Chọn reuse/consumer fix bằng source và runtime, không rebuild kit |
| Medium | Mobile đợt45/Editor nằm ngoài thiết kế | Mark explicit exclusion, không bỏ nhầm MReview hoặc V40 |

## Xác minh
- Kiểm manifest row từng giá trị với footer, mỗi screen có đúng PR owner; links/source paths tồn tại, không viện source dist thay source TS.
- Chạy CLI validate/reindex của plan, đọc mọi phase và kiểm dependencies không chu kỳ; không gọi kế hoạch đã triển khai.
- Đã chạy extractor và capture Playwright reference/app trong QA riêng; xem [baseline, ảnh, command và giới hạn](reports/pr-00/baseline.md). Pending target không được gọi full tweak/control acceptance.

## Checklist
- [x] Gửi inventory và chốt actual contracts/callers trước sửa code.
- [x] Hoàn tất state inventory và baseline theo viewport artboard; trạng thái target chưa seed/map được giữ pending với owner, không gọi UI acceptance.
- [x] Có preflight legacy API và giới hạn đã kiểm; DB snapshot/admission và release gate vẫn thuộc PR migration. Chưa có policy legacy thực tế cần hỏi.
- [ ] Merge PR0 riêng, chuyển PR1 nền theo đúng dependencies.

## Quay lui và bảo mật
PR0 không đổi runtime/DB. Không commit credentials/raw private data/trace chưa rà. Hoàn tác chỉ là revert tài liệu của PR, không xoá baseline người dùng hoặc chỉnh repo `verse-os`.
