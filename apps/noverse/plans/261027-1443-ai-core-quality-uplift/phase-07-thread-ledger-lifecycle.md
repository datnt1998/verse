---
title: "Phase 7: Thread ledger lifecycle"
status: todo
priority: P2
effort: "2 ngày"
depends_on: [6]
---

# Phase 7: Thread ledger lifecycle

## Overview

Sổ phục bút có 3 điểm mù:

- **Không biết tác giả định thu ở đâu.** Phục bút có thể bị treo hoặc bị thu quá sớm.
- **Độ "cũ" bị đo sai mốc.** Tuổi được tính từ `planted_at` chứ không phải từ lần đẩy gần nhất (`internal/tools/novel_context.go:893-908`). `diag` thì chỉ đếm phục bút ở trạng thái `planted` (`internal/diag/diag.go:124-128`, `rules_planning.go:21-29`).
- **Không có cách khai báo bỏ tuyến có chủ đích.**

Phase này thêm:

- `due_by`: chương dự kiến thu, tùy chọn khi plant;
- `last_touched`: chương gần nhất có plant hoặc advance;
- action `abandon`, có lý do;
- logic **overdue/dormant** dùng chung cho context và diag;
- **một** máy trạng thái duy nhất cho cả `UpdateForeshadow` và projector.

Hiện trạng đã kiểm chứng:

- Hai đường chạy cùng một logic: `internal/store/world.go:163-235` và `internal/revision/projector.go:113-140`.
- Hiện tại `advance` trên phục bút đã `resolved` sẽ lặng lẽ đổi trạng thái về `advanced`, còn `resolve` lặp lại thì ghi đè `ResolvedAt` (`world.go:214-227`). Không có lỗi nào được báo.
- Commit saga và projector phát lại toàn bộ record (`internal/store/revisions.go`, `projector.go`). Vì vậy **mọi** thao tác lên phục bút đã đóng phải không bao giờ trả lỗi, nếu không replay sẽ vỡ.
- Enum action nằm ở `internal/chapterfacts/facts.go:24` và `internal/host/imp/contracts.go:59`.
- `OutlineEntry` **không có** trường phục bút (`internal/domain/story.go:35-45`).
- TUI dùng `ForeshadowOpen` và `ForeshadowStale` (`internal/entry/tui/report.go:133-135`).

## File ownership

| Tạo mới | Sửa |
|---------|-----|
| `internal/domain/foreshadow.go` (`ApplyForeshadow`, `ThreadHealth`, `ForeshadowDormantThreshold`) + test | `internal/domain/review.go` (`ForeshadowEntry`: `DueBy`, `LastTouched`, `AbandonReason`, `AbandonedAt`; `ForeshadowUpdate`: `DueBy`, `Reason`) |
| | `internal/store/world.go`, `internal/revision/projector.go` (gọi `ApplyForeshadow`) + test so khớp hai đường |
| | `internal/chapterfacts/facts.go`, `internal/host/imp/contracts.go` (enum thêm `abandon`; property `due_by`, `reason` nullable) |
| | `internal/tools/novel_context.go` (`selectStoryThreads`, xóa `foreshadowAgingChapters`) + test |
| | `internal/diag/diag.go`, `rules_planning.go` (dùng `ThreadHealth`; thêm finding overdue trong `StaleForeshadow`) + test; `internal/entry/tui/report.go` nếu nhãn đổi |
| | `internal/eval/case.go`, `grade.go` (`expect.context_contains`) + test |
| | `assets/prompts/writer.md` (mục commit phục bút), `assets/prompts/architect-long.md` (`promise`), golden |
| | `web/src/**` (`foreshadowStatus` thêm `abandoned`; hiển thị `due_by`), `types.gen.ts`, `dist/**` |
| | `docs/design/v1/06-data-contract.md` |

## Requirements

1. **`ApplyForeshadow(entries, idx, chapter, u) ([]ForeshadowEntry, error)`** là nơi **duy nhất** chứa máy trạng thái.
   - **Giữ nguyên**: plant cùng chương là idempotent; plant trùng ID ở chương khác báo lỗi (giữ thông điệp `world.go:189-194`); advance/resolve một ID chưa biết báo lỗi.
   - **`plant`**: gán `LastTouched = chapter`. Nếu có `DueBy` thì phải > `chapter`, không thì `ErrToolArgs`.
   - **`advance`** trên phục bút đang mở: gán `Status = advanced` và `LastTouched = chapter`.
   - **`resolve`** trên phục bút đang mở: hành vi như hiện tại.
   - **`abandon`** trên phục bút đang mở: gán `Status = abandoned`, `AbandonedAt`, `AbandonReason`. Thiếu `reason` thì `ErrToolArgs`. Đây là kiểm tra đầu vào; dữ liệu replay đã có `reason` nên không vướng.
   - **Phục bút đã đóng** (`resolved` hoặc `abandoned`): mọi `advance/resolve/abandon` là **no-op, không báo lỗi**, giữ nguyên trạng thái đóng. Riêng `resolve` lặp lại trong cùng chương của `ResolvedAt` vẫn idempotent như cũ.
     - Đây là thay đổi hành vi có chủ đích: trước đây `advance` sau `resolve` âm thầm mở lại phục bút. Ghi chú thay đổi này trong docs.
     - Replay không bao giờ vỡ.
   - `Evidence` (từ phase 06) cũng được ghi ở đây.
   - `world.go` và `projector.go` cùng gọi hàm này. Test so khớp: cùng một chuỗi record thì cả hai đường cho ra ledger giống hệt nhau. Có thêm case "advance sau resolve" và "abandon sau resolve".
2. **`ThreadHealth(e, chapter, completed int) (overdue, dormant bool)`**:
   - `overdue = DueBy > 0 && chapter > DueBy && status ∈ {planted, advanced}`
   - `dormant = status ∈ {planted, advanced} && chapter - max(LastTouched, PlantedAt) > ForeshadowDormantThreshold(completed)`
   - `ForeshadowDormantThreshold` chuyển từ `diag.staleForeshadowThreshold` sang domain.
   - **Xóa** hằng `foreshadowAgingChapters`.
3. **Context.** `selectStoryThreads` giữ bước 1 (khớp từ khóa). Bước 2 điền chỗ trống theo thứ tự:
   1. `overdue`, `DueBy` cũ hơn trước;
   2. `DueBy ∈ [N, N+2]`;
   3. `dormant`, `LastTouched` cũ hơn trước.

   Giữ `maxThreads = 5`. `Reason` ghi rõ loại. `abandoned` không bao giờ được truy hồi.
4. **Diag.**
   - `ForeshadowOpen` không đếm `abandoned`.
   - `ForeshadowStale` dùng `dormant`.
   - `StaleForeshadow` (`rules_planning.go`) thêm dòng cho các phục bút overdue, cùng mức severity, **không** thêm rule mới.
   - Cập nhật test diag và nhãn TUI nếu cần.
5. **Prompt.**
   - `writer.md`: khi plant thì cân nhắc `due_by`; chỉ `abandon` khi có quyết định rõ trong truyện hoặc theo chỉ đạo của user, kèm `reason`.
   - `architect-long.md`: ghi chương dự kiến thu của phục bút chính trong `promise` của outline (`OutlineEntry.Promise`, `internal/domain/story.go:42`). **Không** thêm trường mới vào outline.
6. **Web.** Thêm nhãn `abandoned`, hiển thị `due_by`. Chạy `go generate`, sau đó `npm run typecheck && npm test && npm run build`, commit `dist`.

## Implementation Steps

1. Viết `domain/foreshadow.go` và test bảng cho mọi chuyển trạng thái, gồm các no-op trên phục bút đã đóng.
2. Thay logic ở `world.go` và `projector.go`. Viết test so khớp bằng fixture 6 chương: plant, advance, abandon, resolve, advance-sau-resolve.
3. Cập nhật enum và property trong `chapterfacts` và `host/imp/contracts.go`, cùng golden schema nếu có.
4. Viết `ThreadHealth`, chuyển ngưỡng, sửa `selectStoryThreads` và diag. Cập nhật test aging (`rg -n 'agingForeshadow\|foreshadowAgingChapters' internal`).
5. Sửa prompt, golden. Thêm case `wr-thread-overdue` (dev): fixture có phục bút `due_by = N-1`. Assertion cơ học: kết quả `novel_context` trong session log có phục bút đó kèm lý do quá hạn. Cần thêm `expect.context_contains` vào harness: chuỗi phải xuất hiện trong kết quả tool `novel_context`.
6. Web và build, chụp ảnh trên sách fixture tổng hợp.
7. Docs.

## Test Matrix

| Lớp | Lệnh |
|-----|------|
| Domain | `go test ./internal/domain/...` |
| Store, revision | `go test ./internal/store/... ./internal/revision/...` |
| Tool, diag, TUI | `go test ./internal/tools -run 'ContextTool\|StoryThread\|Commit' ./internal/diag/... ./internal/entry/tui/...` |
| Import | `go test ./internal/host/imp/...` |
| Web | `cd web && npm run typecheck && npm test && npm run build` |

## Success Criteria

- [x] Máy trạng thái phục bút chỉ còn ở `domain/foreshadow.go`. Kiểm bằng `rg -n 'case "advance"' internal --glob '!*_test.go'`: chỉ còn 1 kết quả ngoài lớp schema/enum.
- [x] Phục bút vừa được đẩy không bị truy hồi với lý do "ngủ quên" (có test).
- [x] Phục bút `abandoned` không vào context và không bị diag tính là treo.
- [x] Ledger cũ (chưa có trường mới) load và replay cho kết quả giống hệt trước.

## Risk Assessment

- **Model lạm dụng `abandon`.** `reason` bắt buộc và hiển thị trên UI. Nếu A/B cho thấy hiện tượng này thì ghi vào báo cáo; rule diag riêng nằm ngoài phạm vi.

## Rollback

Revert commit. Các trường mới đều `omitempty`. Ledger đã có `abandoned` sẽ bị code cũ coi là còn mở, không mất dữ liệu; ghi trong ghi chú phát hành.
