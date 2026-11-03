---
title: "Viderse · Refactor workflow đợt 40–44"
description: "Kiểm kê trước code, tái dùng kit và chuyển UI/backend theo V36–V40 qua bảy PR tuần tự."
status: in-progress
priority: P1
branch: main
tags: [refactor, frontend, backend, database, api]
blockedBy: []
blocks: []
created: 2026-11-03
---

# Viderse · Workflow gọn theo thiết kế đợt40–44

## Kết quả và ranh giới
HOLD SCOPE · kế hoạch đã chốt theo mode **deep**, người dùng đã gọi `ak-cook` để triển khai. Inventory/QA đã merge qua [PR riêng #1](https://github.com/datnt1998/verse/pull/1) ngày 2026-11-03, merge commit `f120a62c8ce664b8e4c42f8edda6a74a24e55d91`; tiếp tục PR1 nền UI. Local gates đã qua; GitHub Actions đang tắt, không có kết quả CI hosted. **Chưa deploy UI mới**. Desktop/mobile cùng chức năng/default theo thiết kế, mọi thao tác có phản hồi; không làm mobile image viewer đợt45 hoặc tự thiết kế mobile Editor. Không đụng Noverse, `verse-os`, provider production hay hàng đợi người dùng.
Nguồn ưu tiên: `apps/viderse/docs/design/v1/artboards/Workflow.dc.html` → 01-ia-flows §0.3/§6.1/§6.2 Đợt44/W3–W13 → 08-dev-prompt V36–V40 → artboard/footer/kit/copy/data/stack. Không sửa artboards; muốn lệch phải ghi `11-hoi-dap.md` và chờ trả lời.

## Bảng kiểm kê gửi trước sửa code
[Bảng code ↔ artboard ↔ gói ↔ giữ/sửa/xoá](reports/inventory.md). Kit đã có RO/fonts/overlay lifecycle; tái dùng thay vì dựng lại. Các cutover lớn: channel rotation/autonomy, create-page→dialog, canonical jobs API và ordering queue.

## Phases — một bước một PR, không chạy song song
| Phase | Bước / PR | Owner sản phẩm | Phụ thuộc | Trạng thái |
|---|---|---|---|---|
| 1 | [0 · Kiểm kê và baseline](phase-01-inventory.md) | Tài liệu/state manifest/evidence | — | Completed · PR #1 merged |
| 2 | [1 · Nền UI dùng chung](phase-02-shared-ui-foundation.md) | Kit tối thiểu + Viderse policy | PR0 merge | In progress |
| 3 | [2 · V36 Viewer và nối nút](phase-03-v36-viewer-and-actions.md) | Desktop shared viewer, data-key removal | PR1 merge | Pending |
| 4 | [3 · V37 Bàn dựng gọn](phase-04-v37-compact-studio.md) | Main/progress/proposals/obsolete backend | PR2 merge | Pending |
| 5 | [4 · V38 Kênh, Thư viện, Template](phase-05-v38-channels-and-library.md) | FE+BE+schema/default-template | PR3 merge | Pending |
| 6 | [5 · V39 Nhập nhanh và hàng chờ](phase-06-v39-quick-create-and-queue.md) | Canonical creation/dedup/front order | PR4 merge | Pending |
| 7 | [6 · V40 Mobile và full-flow gate](phase-07-v40-mobile-parity.md) | §6.2 Đợt44 + desktop regression | PR5 merge | Pending |

## Kiến trúc và hợp đồng
Shell/session → adapter responsive trên `@noverse/ui` → cùng dữ liệu/action desktop-mobile. Một desktop image viewer reuse review/provenance/mutations; không viewer/service riêng cho từng màn. Existing HTTP→service→transactional postgres queue được giữ.
[Backend scope và chuyển dữ liệu](backend-contracts.md): POST `/api/jobs` theo brief, migrate caller rồi bỏ creation routes cũ; default FK/autoApprove85/autoFetch/perDay; manual mới prepend, auto append; only workers editable. Không đổi job snapshots đang chạy hoặc hạ kiểm provenance để tránh lỗi.

## Nghiệm thu mỗi PR và các điểm dừng
[Giao thức đầy đủ](verification.md): mọi giá trị tweak hợp lệ; reference/code/comparison1440×900 và390×844; control ledger Playwright0dead; overlays trong viewport; slider≤2px sau font; motion normal/reduced; touch≥44px; report/gap với commit thật. Fixture defaults đúng brief, chỉ QA, không ghi đè dữ liệu production.
Không merge PR có blocker. Preflight có perDay>20/rotation rỗng/dangling, thiếu API state hoặc timing fallback khác thiết kế: ghi câu hỏi và chờ, không clamp/fake/fallback tuỳ ý. Mỗi phase re-scout trước sửa; future-phase paths là source map, không quyền bỏ qua references.

## Execution checklist / task hydration
- [x] PR0: gửi inventory, hoàn tất state manifest/baseline/preflight rồi merge riêng.
- [ ] PR1: nền UI reuse đúng và qua shared-kit regression trước V36.
- [ ] PR2: viewer/action/data-key theo V36, đủ evidence trước V37.
- [ ] PR3: Main/progress/3proposal + backend obsolete theo V37.
- [ ] PR4: default template/automation/library/create cùng schema, cả mobile contract.
- [ ] PR5: Nhập nhanh/jobs API/dedup/front ordering/compact queue rules.
- [ ] PR6: full mobile parity/motion/keyboard/prompt4parts và toàn gate cuối.

## Red Team Review
4 lens/verification roles; 10finding đã nhận và đưa vào phase tương ứng (1Critical/8High/1Medium). [Bằng chứng và adjudication](reports/plan-review.md). Deltas gồm routed v13, img route, MComp direct-add, notification prefs, collector retirement, admission snapshot, queue-cap/dedup, safe rollback và QA credential isolation.

## Validation Log
CLI schema/link/dependency sweep và giới hạn chạy thật nằm trong review record. Không có quyết định sản phẩm mới bị tự áp dụng; dữ liệu legacy thiếu policy phải preflight/hỏi trước, không giả đã có câu hỏi được trả lời.

## Review, validation và handoff
[Review/verification record](reports/plan-review.md) giữ bằng chứng lúc lập kế hoạch. [PR0 baseline](reports/pr-00/baseline.md) giữ runtime hiện tại: reference/app comparisons trong QA riêng và smoke có oracle, **chưa full-control/motion acceptance**. Không đọc baseline thành kết quả UI đã đạt.
Tiếp tục qua từng PR sau khi evidence/review và merge gate của PR trước qua; không cần xin lại quyền cook thường lệ. Không tự thêm `--auto`, tạo worktree sau yêu cầu dọn, nhảy dependency hoặc deploy ngoài release gate đã chốt.
