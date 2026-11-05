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
HOLD SCOPE · kế hoạch mode **deep**, người dùng đã gọi `ak-cook`. Inventory, nền UI và V36–V39 merge qua PR#1–#6 (workflow merge mới nhất `9ad1effc`). V40 vẫn Draft PR#7: UI kiểm tới `24bf453d`, backend `776195e6` có full affected Go checks; helper duration `bbfab83c` có36focusedtests qua. #91/#92 đã chốt/triển khai, giữ Safari iPhone thật#93. GitHub Actions tắt, không nhận CI hosted. **Chưa deploy toàn bộ workflow/UI mới.** Hotfix production riêng: scoped-loopback/catalog ảnh `401d1b45`, rồi playback backport tối thiểu trên UI cũ `9d440603`. [Recovery](reports/pr-06/production-video-recovery.json): score100/100; export attempt1 nay thành công, MP4432.805.526B/SHA/ffprobe3380,7s đã kiểm. [Playback](reports/pr-06/playback-hotfix/receipt.json): built UI PC/mobile phát/seek đúng file thật qua read-response fixture; public chunk khớp build, không có authenticated prod/Safari physical proof. Không thay PUBLIC_URL/SSO/editor policy, không bỏ hash/token/workspace hoặc chạy lại AI; không mở gate UI. Không làm viewer45/mobile Editor, không đụng Noverse/`verse-os` hoặc provider production.
Nguồn ưu tiên: `apps/viderse/docs/design/v1/artboards/Workflow.dc.html` → 01-ia-flows §0.3/§6.1/§6.2 Đợt44/W3–W13 → 08-dev-prompt V36–V40 → artboard/footer/kit/copy/data/stack. Không sửa artboards; muốn lệch phải ghi `11-hoi-dap.md` và chờ trả lời.

## Bảng kiểm kê gửi trước sửa code
[Bảng code ↔ artboard ↔ gói ↔ giữ/sửa/xoá](reports/inventory.md). Kit đã có RO/fonts/overlay lifecycle; tái dùng thay vì dựng lại. Các cutover lớn: channel rotation/autonomy, create-page→dialog, canonical jobs API và ordering queue.

## Phases — một bước một PR, không chạy song song
| Phase | Bước / PR | Owner sản phẩm | Phụ thuộc | Trạng thái |
|---|---|---|---|---|
| 1 | [0 · Kiểm kê và baseline](phase-01-inventory.md) | Tài liệu/state manifest/evidence | — | Completed · PR #1 merged |
| 2 | [1 · Nền UI dùng chung](phase-02-shared-ui-foundation.md) | Kit tối thiểu + Viderse policy | PR0 merge | Completed · PR #2 merged |
| 3 | [2 · V36 Viewer và nối nút](phase-03-v36-viewer-and-actions.md) | Desktop shared viewer, data-key removal | PR1 merge | Completed · PR #3 merged `3e238b08` |
| 4 | [3 · V37 Bàn dựng gọn](phase-04-v37-compact-studio.md) | Main/progress/proposals/obsolete backend | PR2 merge | Completed · [PR #4 merged](https://github.com/datnt1998/verse/pull/4) `277b902b`, [nghiệm thu](reports/pr-03/acceptance.md) |
| 5 | [4 · V38 Kênh, Thư viện, Template](phase-05-v38-channels-and-library.md) | FE+BE+schema/default-template | PR3 merge | Completed · [PR #5 merged](https://github.com/datnt1998/verse/pull/5) `1591b66b`; mobile final gate ở phase07 |
| 6 | [5 · V39 Nhập nhanh và hàng chờ](phase-06-v39-quick-create-and-queue.md) | Canonical creation/dedup/front order | PR4 merge | Completed · [PR #6 merged](https://github.com/datnt1998/verse/pull/6) `9ad1effc`, [nghiệm thu có giới hạn](reports/pr-05/acceptance.md) |
| 7 | [6 · V40 Mobile và full-flow gate](phase-07-v40-mobile-parity.md) | §6.2 Đợt44 + desktop regression | PR5 merge | [Draft PR #7](https://github.com/datnt1998/verse/pull/7), source/native checks qua, #91/#92 đã đóng; chờ [Safari iPhone thật](reports/pr-06/physical-safari-handoff.md), chưa merge/deploy |

## Kiến trúc và hợp đồng
Shell/session → adapter responsive trên `@noverse/ui` → cùng dữ liệu/action desktop-mobile. Một desktop image viewer reuse review/provenance/mutations; không viewer/service riêng cho từng màn. Existing HTTP→service→transactional postgres queue được giữ.
[Backend scope và chuyển dữ liệu](backend-contracts.md): POST `/api/jobs` theo brief, migrate caller rồi bỏ creation routes cũ; default FK/autoApprove85/autoFetch/perDay; manual mới prepend, auto append; only workers editable. Không đổi job snapshots đang chạy hoặc hạ kiểm provenance để tránh lỗi.
- Ngoại lệ backend đã được người dùng duyệt tại design #82: sửa nguồn ảnh tách nền sang ảnh đã chuẩn hoá và xử lý derivative cũ từ ảnh hiện có; không thêm lượt AI/không giảm validation.
- Ngoại lệ backend #84A đã được người dùng duyệt và triển khai tại `52d3871225e4a6c8dd0799c0e5d151692463d600`: Sửa dùng đúng phiên bản đang xem qua `baseVersion` tùy chọn, giữ prompt/tham chiếu/provenance chưa đổi; không tự đổi bản đang dùng. [Bằng chứng tích hợp](reports/pr-02/selected-source-integration.json) và [trạng thái nghiệm thu V36](reports/pr-02/viewer-and-actions.md). PR #3 đã merge tại `3e238b08`; không suy ra V37–V40 hoặc gate deploy đã qua.

## Nghiệm thu mỗi PR và các điểm dừng
[Giao thức đầy đủ](verification.md): mọi giá trị tweak hợp lệ; reference/code/comparison1440×900 và390×844; control ledger Playwright0dead; overlays trong viewport; slider≤2px sau font; motion normal/reduced; touch≥44px; report/gap với commit thật. Fixture defaults đúng brief, chỉ QA, không ghi đè dữ liệu production.
Không merge PR có blocker chức năng/UI. Ngoại lệ thực thi được người dùng duyệt tại design #83: hai gate toàn cục cold phone≤200KiB và độ trễ Khám phá được giữ mở tới gate deploy cuối, phải báo cáo ở mỗi PR và đạt trước deploy; không đổi ngưỡng. Preflight có perDay>20/rotation rỗng/dangling, thiếu API state hoặc timing fallback khác thiết kế: ghi câu hỏi và chờ, không clamp/fake/fallback tuỳ ý. Mỗi phase re-scout trước sửa; future-phase paths là source map, không quyền bỏ qua references.

## Execution checklist / task hydration
- [x] PR0: gửi inventory, hoàn tất state manifest/baseline/preflight rồi merge riêng.
- [x] PR1: nền UI reuse đúng, shared-kit regression và [bằng chứng](reports/pr-01/foundation.md); PR #2 merged.
- [x] PR2: viewer/action/data-key theo V36, PR #3 merged; giới hạn thiết bị và gate#83 vẫn giữ ở nghiệm thu cuối.
- [x] PR3: Main/progress/3proposal + backend obsolete theo V37; PR #4 merged.
- [x] PR4: default template/automation/library/create cùng schema, cả mobile contract; PR #5 merged `1591b66b`.
- [x] PR5: Nhập nhanh/jobs API/dedup/front ordering/compact queue rules; PR #6 merged `9ad1effc`.
- [ ] PR6: full mobile parity/motion/keyboard/prompt4parts và toàn gate cuối.

## Red Team Review
4 lens/verification roles; 10finding đã nhận và đưa vào phase tương ứng (1Critical/8High/1Medium). [Bằng chứng và adjudication](reports/plan-review.md). Deltas gồm routed v13, img route, MComp direct-add, notification prefs, collector retirement, admission snapshot, queue-cap/dedup, safe rollback và QA credential isolation.

## Validation Log
CLI schema/link/dependency sweep và giới hạn chạy thật nằm trong review record. Không có quyết định sản phẩm mới bị tự áp dụng; dữ liệu legacy thiếu policy phải preflight/hỏi trước, không giả đã có câu hỏi được trả lời.

## Review, validation và handoff
[Review/verification record](reports/plan-review.md) giữ bằng chứng lúc lập kế hoạch. [PR0 baseline](reports/pr-00/baseline.md) giữ runtime hiện tại: reference/app comparisons trong QA riêng và smoke có oracle, **chưa full-control/motion acceptance**. Không đọc baseline thành kết quả UI đã đạt.
Tiếp tục qua từng PR sau khi evidence/review và merge gate của PR trước qua; không cần xin lại quyền cook thường lệ. Không tự thêm `--auto`, tạo worktree sau yêu cầu dọn, nhảy dependency hoặc deploy ngoài release gate đã chốt.
