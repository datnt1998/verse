# P2 — Code slice: `opening_state` + `upcoming_outline` (Implementation Step 1 & 6)

Đã xong phần code của phase 02 (không đụng `assets/prompts/*.md` — phần prompt
đi qua variant A/B như phase định).

## Thay đổi

- `domain.ChapterPlan.OpeningState` (`json:"opening_state,omitempty"`); cùng
  property optional trong schema `plan_chapter` (mô tả đúng phase) và decoder.
- `working_memory.upcoming_outline` trong `buildOutlineWindow`: `{note, entries}`
  với `note = "kế hoạch tương lai — chưa xảy ra, không thực hiện sớm"`, tối đa 2
  entry ngay sau chương hiện tại; mode phân tầng giới hạn trong arc hiện tại
  (`min(boundary.EndChapter, len(outline))`). `outline_window` giữ nguyên.
- Web: `ChapterPlan.opening_state?`, `PlanFields` hiển thị, mock plan ch.5 có
  `opening_state`, dist rebuild.
- **Consumer field-list phát hiện thêm, đã sửa cùng scope web:**
  `web/src/screens/reader/copy.ts` (nhãn `openingState`), `web/src/lib/labels.ts`
  (`FIELDS.opening_state`), `web/src/lib/streamRounds.ts` (`MULTILINE_KEYS`).
- **Không sửa, ghi nhận:** `web/src/screens/project/tabs/OutlineTab.tsx`
  (`planLines`) chỉ xem trước goal/conflict/hook/emotion_arc — đã không hiển thị
  cả `notes`, nên giữ nguyên là nhất quán, không bắt buộc thêm `opening_state`.

## trimOrder (`internal/tools/novel_context.go`)

`upcoming_outline` **không** được đưa vào `trimOrder` (file thuộc slice phase 06).
Key mới tối đa 2 entry nhỏ, nằm trong `working_memory`; khi vượt ngân sách nó
không bị cắt — đúng vai trò "fact trỏ nội". Nếu phase 06 muốn cắt được, thêm vào
`trimOrder` sau `foreshadow_ledger`, trước `relationship_state`. Không cần sửa để
compile/pass test hiện tại.

## Schema golden

`internal/tools/testdata/` chỉ có `save_review_schema.json` (phase 04). Không có
golden cho `plan_chapter` → không cần `-update`.

## `go generate ./internal/entry/web`

`gentypes` chỉ sinh DTO trong `api_dto.go`; `ChapterPlan` là type viết tay →
không cần chạy lại.

## Test

- `go test ./internal/tools -run 'PlanChapter|Retention|ContextTool' ./internal/domain/...` — green
- `go test -count=1 ./internal/tools/...` — green
- `cd web && npm run typecheck && npm test && npm run build` — green (1484 test)
