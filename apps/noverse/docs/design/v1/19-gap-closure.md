# 14 · Đóng các khoảng trống còn lại & Việt hoá lõi

Bổ sung cho `11-api-contract.md` (vẫn là hợp đồng gốc) và `13-commands.md`. Mọi mục dưới đây là **phải làm trong v1**; Audio (GAP-17…21) vẫn để v1.1.

## A. API bổ sung

Quy ước giữ nguyên `11-api-contract.md`: JSON, lỗi `{"error":"<tiếng Việt>"}`, quyền như §2; đường dẫn truyện `/api/projects/:p/stories/:s/*`.

| # | Màn | Endpoint / trường | Ghi chú |
|---|---|---|---|
| G1 | Nhật ký, Trình đọc, Cài đặt › Quy tắc, Dự án | `POST /api/reveal {project?, story?, target}` → `{ok:true, path}`; `target`: `library` · `project` · `story` · `exports` · `activity` (thư mục `output/novel/meta`) · `rules` (+ `scope: global\|project\|story`) · `config` (+ `scope`, cùng quy tắc: chọn đúng tệp `config.json` của cấp đó bằng `open -R` / `explorer /select,`; Linux mở thư mục chứa; tệp chưa có → mở thư mục chứa) | Mở thư mục bằng trình quản lý tệp **trên máy chủ** (`open` / `xdg-open` / `explorer`). Chỉ khi yêu cầu đến từ loopback; khác → 403 `Chỉ mở được thư mục khi dùng ngay trên máy chủ`. Quyền: đọc được đối tượng; `rules`/`config` toàn cục và `library` cần admin. `GET /api/system` thêm `canReveal: bool` (theo yêu cầu hiện tại) để UI chọn nút **Mở thư mục** hay **Chép đường dẫn**. |
| G2 | Cài đặt › Thư viện | `PUT /api/library {root}` (admin) đổi thư mục thư viện lúc chạy; `POST /api/system/pick-folder {start?}` → `{path}` hoặc `{cancelled:true}` | Gốc mới phải tồn tại, là thư mục, đọc/ghi được; từ chối 409 khi có truyện đang chạy. Lưu vào `~/.ainovel/serve.json {"root"}`; `serve` không có `--root` dùng giá trị này (cờ `--root` thắng). Pick-folder chỉ loopback (macOS `osascript choose folder`, Linux `zenity`/`kdialog`), không hỗ trợ → 501. |
| G3 | Nhật ký (nhóm theo phiên) | `HostEvent.sessionId` (string) | Tăng mỗi lần engine khởi chạy một phiên (start/resume/continue/reopen/import); có trong SSE `event` và `GET …/events`. |
| G4 | Nhật ký › “Thử lại ngay” | `POST …/retry` → `{ok:true}` | Đánh thức vòng chờ backoff (sự kiện có `retryAt`) ngay lập tức; không có gì đang chờ → 409 `Không có lượt nào đang chờ thử lại`. Quyền viết. |
| G5 | Workspace › StatusChip | `Snapshot.waitingFor`: `""` · `gate` (cổng duyệt/advance hold) · `cocreate` · `confirm` (import chờ xác nhận) · `input` (đang rảnh, chờ lệnh) | Chip: đang chạy → “Đang chạy”; `waitingFor` khác rỗng → “Đang chờ bạn”. |
| G6 | Cài đặt/Onboard › Thử kết nối | `POST /api/config/test` khi thiếu `model` dùng `defaultModel` của preset theo `type`/tên provider | Trả `model` đã dùng trong kết quả. |
| G7 | Cài đặt › OAuth | Sau `status: done`, `GET /api/config` trả tài khoản đã kết nối (persist) | Lỗi hiện tại. |
| G8 | Truyện mới › Đồng sáng tác | `GET/POST /api/projects/:p/cocreate[/message\|/commit\|/cancel]` với `?draft=<id>` | Phiên đồng sáng tác **trước khi có truyện**: worker chạy ở `<dự án>/.ainovel/drafts/<id>/`; `message` đầu tiên không có `draft` → tạo id, trả `{draft, …CoCreateState}`; `commit {name?}` → chuyển thư mục nháp thành `<kich_ban>/<NN-slug>` (slug theo tên truyện trong bản nháp) và bắt đầu viết → `{story}`; `cancel` → xoá thư mục nháp. Nháp quá 7 ngày bị dọn khi khởi động. Quyền: tạo truyện trong dự án. |
| G9 | Nhập bản thảo › “Xem các chương” | `POST …/import/preview {path}` → `{chapters:[{n,title,words,firstLine}], warnings[], encoding}` | Không ghi dữ liệu. Dùng đúng bộ tách chương của `/import`. |
| G10 | Trình đọc tự làm mới | `GET …/chapters` thêm `revision` (chuỗi đổi khi bất kỳ chương/bản nháp/thẩm duyệt đổi; dùng mtime + kích thước) | UI hỏi lại mỗi 5 s khi tab hiển thị; đổi `revision` → tải lại danh sách + chương đang mở (giữ vị trí cuộn). |
| G11 | Popover `/` & Hướng dẫn lệnh | `GET /api/commands` → `[{name, aliases[], usage, group, autoExecute, needsIdle, only?: "welcome"\|"running", params[]}]`, `params[] = {key, token, kind: path\|enum\|flag\|num\|text, required, prefix?, flag?, options?[{value,note}]}` | Sinh từ registry dùng chung (chuyển `slashCommandSpec` ra `internal/commands`, TUI dùng lại). GAP-23. |
| G12 | Tham số có khoảng trắng | Bộ tách tham số hỗ trợ `"…"` và `'…'` cho mọi lệnh | GAP-22; TUI và web dùng chung. |
| G13 | Lệnh từ ô chat web | `POST …/command {line}` → `{ok, message?}` | Ô chat web gửi dòng bắt đầu bằng `/` tới đây; server phân tích bằng registry G11/G12 và gọi đúng hành động (export, sync, diag, review, next, reopen, import, simulate, importsim, model, cocreate, help → UI tự xử lý). Lỗi cú pháp trả 400 với đúng thông báo `Cách dùng: …`; `needsIdle` khi đang chạy → 409 `Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /x`. GAP-25: nút ở Workspace/Trình đọc vẫn gọi API riêng. |

## B. Việt hoá lõi (GAP-24 mở rộng)

Quyết định của chủ dự án: **bỏ chế độ viết tiếng Trung**, toàn bộ chuỗi chạy (model đọc hoặc người dùng thấy) là tiếng Việt; **comment trong mã giữ nguyên**.

1. **Thuật ngữ**: dùng đúng bộ đã có — `internal/store/labels.go` (`labelsVI`: tập / cung / chương / đề cương / điểm móc / phục bút…), prompt `assets/prompts/*.md`, `web/src/i18n/vi.ts`, `07-copy.md`. Thuật ngữ mới ghi vào `local://vi-glossary.md` trước khi dùng.
2. **Chuỗi model đọc**: mô tả tool + JSON schema, kết quả/lỗi trả về cho model, nhãn ngữ cảnh (`novel_context`, ctxpack), thông điệp arbiter/flow, mẫu prompt dựng trong mã. Giữ nguyên tên tool, tên trường JSON, giá trị enum máy đọc.
3. **Chuỗi người dùng thấy**: sự kiện host, nhãn trạng thái, lỗi, nhật ký, xuất bản, thông báo, chẩn đoán, TUI.
4. **Bỏ `zh`**: xoá `assets/prompts/zh`, `assets/references/zh`, `assets/styles/zh*`, `assets/zh`, `voice_zh.md`, `labelsZH`, nhánh `zh` trong `assets/load.go`, exporter, setup TUI; `language` trong cấu hình chỉ còn `vi` (giá trị cũ `zh` → đọc như `vi`, ghi cảnh báo một lần). UI bỏ lựa chọn ngôn ngữ sáng tác (bước Onboard “Ngôn ngữ sáng tác” và ô Ngôn ngữ ở Dự án/Cài đặt).
5. **Tương thích dữ liệu cũ**: mọi bộ phân tích đọc dữ liệu đã lưu (tiêu đề `第N章`, nhãn cũ trong `meta/*.json`, `reviews/*.json`, phiên) **vẫn chấp nhận dạng Trung** bên cạnh dạng Việt; chỉ đầu ra đổi.
6. **Đếm chữ**: “chữ” = số từ (cụm ký tự chữ/số ngăn bởi khoảng trắng), không phải số ký tự. Mọi ngưỡng độ dài (mục tiêu chương, kiểm tra tối thiểu/tối đa, prompt) quy đổi theo; số liệu đã lưu được tính lại khi đọc từ tệp chương.
7. **Lint `cjk_leak`** giữ nguyên: chữ Hán trong văn bản truyện tiếng Việt vẫn là lỗi.
8. **Kiểm thử**: test kiểm chuỗi Trung → kiểm chuỗi Việt; test riêng chế độ `zh` → xoá; dữ liệu mẫu tiếng Trung chỉ giữ khi đang kiểm tính tương thích.

## C. Giao diện

- Popover `/` trong ô chat Workspace theo `Slash.dc.html` + `13-commands.md` §1 (dữ liệu từ G11, kiểm tra phía client, gửi qua G13).
- Trang `#/help/commands/:name?` theo `Commands.dc.html` + §2; nút **Hướng dẫn** ở header, phím `?`, lệnh `/help`, palette.
- Nối G1–G10 vào các màn tương ứng; bỏ mọi nhãn “Chưa có API”.
- Giao diện sáng: artboard có bộ chọn nhưng `tokens.css` chưa có bảng màu sáng → giữ “Tối”, hai lựa chọn còn lại ghi “Chưa có bảng màu” cho tới khi thiết kế bổ sung.

## D. Trạng thái GAP-22…29 (kiểm 25/10 · gói G)

Đối chiếu mã Go + kiểm thử; phần giao diện React chỉ đọc, ghi lệch ở cột cuối. Tất cả gói Go liên quan (`internal/commands`, `internal/entry/web`, `internal/host`, `internal/diag`, `internal/revision`, `internal/audio`) qua `go test`.

| GAP | Trạng thái | Bằng chứng (mã → kiểm thử) | Lệch còn lại |
|---|---|---|---|
| 22 · tham số có khoảng trắng | **Đóng** | `commands.SplitArgs/Split/Parse` (`internal/commands/parse.go`), TUI dùng qua `commands.Split` (`internal/entry/tui/commands.go`), web `postCommand` dùng `commands.Parse` → `TestSplitArgs`, `TestSplitArgsUnterminatedQuote`, `TestSplit`, `TestParseInvocations` | `tokenize()` ở `web/src/commands/model.ts` chưa hiểu `\"`/`\\` trong ngoặc và coi ngoặc không đóng là hợp lệ (Go trả `Cách dùng: …`); `quoteIfNeeded` không thoát `"` trong giá trị. Chỉ ảnh hưởng kiểm tra phía client. |
| 23 · `GET /api/commands` | **Đóng** | `serveCommands` → `commands.Specs()` (`Params`, `Aliases`, `Only`) → `TestCommandsEndpoint`, `TestCommandsEndpointRequiresAuth` | — |
| 24 · Việt hoá thông báo host | **Đóng** | `SetAdvanceMode`, `Reopen`, `requireCleanChapters` (`internal/host/host.go`) và tiêu đề luật `internal/diag/rules_*.go` đều tiếng Việt; `TestNoCJKUserFacingLiterals` (`internal/host/vi_literals_test.go`) cấm chuỗi Hán ngoài bảng tương thích | Còn `〈…〉` trong bước nhập `internal/host/imp/analyze.go` và `（…）` ở `internal/entry/tui/panels_activity.go` (dấu toàn chiều, không phải chữ Hán — bài kiểm tra không bắt). |
| 25 · nút `/next`, `/review` | **Đóng** | `POST S/next` → `postNext` và `POST S/review` → `Host.SetAdvanceMode`; lệnh `/next`, `/review` → `dispatchCommand` gọi cùng `Host.AdvanceOneChapter` / `SetAdvanceMode` → `TestCommandSyntaxAndIdleErrors`, `TestCommandDispatchesHostActions`, `comments_test.go` | — |
| 26 · việc nền | **Đóng** | `jobs.go` (`serveJobs`, `controlJob`, `jobEventName`), client `getJobs/getJob/jobAction`, `shell/jobs/store.ts` nghe `job.*` → `TestJobsAggregateSourcesAndFilterPermissions`, `TestJobsDoneRetentionAndGatewayOwnership`, `TestGlobalEventsReplayFiltersPermissions`, `TestWorkerWritingPauseUsesStepBoundary`, **mới** `TestJobControlHTTPPermissionsAndErrors` (401/403/404/409, bỏ việc đã xong) | — |
| 27 · `onExists` | **Đóng** | `serveExportCreate` (`storyread.go`), `runGatewayExport`, `exp.Run`; client `createExport({onExists})`, thẻ kết quả `exportExists` → `--overwrite`/`--keep`; **mới** `TestPostExportsOnExistsStrategies` (mặc định/`fail` 409 không tạo việc, `keep`, `replace`, `overwrite:true`, giá trị lạ 400), `TestKeepVariantPathAppendsCounterOnSameSecond` (trước đây trùng cùng giây báo lỗi dù hợp đồng nói thêm số) | Bảng xuất ở Trình đọc chỉ có ô “Ghi đè” (đúng artboard `Reader`), lựa chọn giữ cả hai nằm ở thẻ kết quả lệnh. |
| 28 · chẩn đoán JSON | **Đóng** | `diagnosis()`/`sendDiagnosisAction`/`getLastDiagnosis` (`worker_results.go`); client `getDiag/getLastDiag/sendDiagAction/downloadDiagExport` → `TestDiagSections`, `TestDiagnosisActionIsIdempotentAndCachedReportTracksSent`, **mới** `TestDiagJSONShape` (stats/findings/actions/sections) và `TestFindingFingerprintStableAndCarriedByActions` | — |
| 29 · `/sync --check` chi tiết | **Đóng** | `Host.CheckRevisionChanges` (`web_revision.go`), `checkSyncJob`, client `checkSync`/`getSyncDiff`, kiểu `SyncChanged` → `TestWorkerSyncCheckExposesDeltasAndJob`, `TestSyncCheckAndRun`, `TestMobileSyncClassificationLinksAndRepair`, **mới** `TestCheckRevisionChangesReportsWordDeltaAndKind` (bản thật: kind `edit`/`resume`, added/removed, modifiedAt = mtime, không ghi chương) | — |

## E. Nghiệm thu gói A–K (`21-dev-prompt.md`, kiểm 25/10)

> **Trả lời của design (25/10):** các câu giao diện còn treo của mục này đã có trả lời hoặc artboard ở `22-brief-2510.md` › *Câu hỏi treo* (Dialogs `locked`/`recovery`, `/simulate`, Esc, `/reopen`, `--keep`, Gói G, điện thoại và máy tính bảng, lỗi 500 của Quy tắc viết). “Tạo quản trị đầu tiên trên web” (Gói B) đã có artboard `Setup` (A0, PO duyệt 25/10), việc dev ở Gói M (GAP-41). Việc dev nằm ở Gói L của `21-dev-prompt.md`.

Chạy cuối: `tsc --noEmit` sạch · vitest 93 tệp / 837 test qua · `npm run build` qua · `go build`/`go vet`/`go test ./...` qua · `gofmt` chỉ còn `internal/entry/tui/panels_outline.go` (không thuộc phạm vi). ✓ = đạt có bằng chứng; ✗ = chưa kiểm hoặc còn lệch (ghi lý do).

| Gói | Mục nghiệm thu | | Bằng chứng / ghi chú |
|---|---|---|---|
| A | Test không lộ chữ kỹ thuật | ✓ | `web/src/lib/noTech.test.tsx`: render `<App/>` trên mock thật ở 1440/820/390, đủ màn chính + hộp thoại + onboard; đúng regex của gói; trừ Nâng cao, khối “Chi tiết kỹ thuật”. Fail-before: thêm `kich_ban/` vào tiêu đề Hướng dẫn → đỏ. Không còn cờ `pending`. |
| A | Nhãn công cụ qua `labels.ts` | ✓ | Workspace, Nhật ký (máy tính + điện thoại) dùng `web/src/lib/labels.ts`; tham số chỉ trong khối mở. |
| B | Không còn `ainovel-cli` trong `web/src` | ✓ | Chỉ còn trong regex/fixture test. Chuỗi host cuối cùng (`host.ErrBookInUse`) đổi thành “…một phiên Noverse khác sử dụng”. |
| B | Test Go `/api/recover` | ✓ | `recovery_test.go`: `TestRecoverEndpoint`, `TestRecoverRateLimitPerIP`, `TestRecoverShortPasswordDoesNotCountTowardLimit`, `TestRecoverUsesLoginRequestRules`, `TestRecoverPublishesSessionRevokedForEverySession`. |
| B | Bật/tắt LAN không mất phiên, không dừng worker | ✓ | `TestNetworkToggleKeepsSessionsAndWorkers` (+ quyền, rollback khi lưu lỗi). |
| C | Mọi tweak `Settings.dc.html` | ✓/✗ | 1440, quản trị: 8 mục × 3 cấp đo vị trí/cỡ/màu/icon/chữ khớp (lệch chỉ là dữ liệu mock). Nâng cao: thêm “Mở” tệp cấu hình (`target=config`, `TestRevealConfigFile`), “· N tệp”, đường dẫn rút gọn như artboard. ✗ Chưa chụp góc nhìn người viết (chỉ có vitest). |
| C | “Riêng truyện này” ↔ *Theo dự án* | ✓ | `sections/roles.test.tsx`, `labels.test`; fail-before đổi `ovrOf` → 3 test đỏ. Go `TestPutConfig_StoryScopeDeleteKeyRestoresInheritance`. |
| C | Người viết không thấy nhóm Hệ thống | ✓ | `nav.test.tsx` (5 mục + vương miện vắng). |
| D | “thở dài” → chip “Tối đa ‘thở dài’ · 2 lần/chương”; Áp dụng từ chương kế | ✓ | Chạy `serve` thật + stub model: chip đúng, `effectiveFrom`, job `rules.recheck`; chương đang viết giữ snapshot cũ (test backend GAP-32). |
| D | Luật học từ chat: nhóm riêng, *Giữ*/*Bỏ* | ✓ | `useRulesEditor.test.tsx` + test Go runtime keep/delete. |
| D | Từ cấm cấp truyện không làm mất 4 cụm mặc định | ✓ | `serve` thật: snapshot = 4 mặc định + từ mới (trộn hợp). |
| D | Chuẩn hoá lỗi → “Lời dặn” (`degraded`), không chặn | ✓/✗ | ✓ với lỗi 401 (trả 200 `degraded`). ✗ Lỗi 500: thư viện model thử lại tới hạn 2 phút trước khi hạ cấp — chờ chủ dự án quyết. |
| D | Điện thoại `rulesAdd`, `rulesThinking`, ẩn thanh tab khi gõ | ✗ | Chưa chụp được (máy quá tải); `rules` + action sheet đã khớp. |
| E | Không còn `y-tuong/`, `kich_ban/`, `*.json`, `output/novel` trên màn Dự án | ✓ | `noTech.test.tsx` (màn dự án/truyện, 3 cỡ); `FolderPanel` đã bỏ. |
| E | Câu tiếng Việt cho mỗi mã `validate`; `fixable=false` không vào nút sửa | ✓ | `screens/project/validate.test.ts` (12 test, đỏ trước khi có `validate.ts`). |
| F | Thanh tab nổi (thường/mini/ẩn, chỉ báo nảy) | ✓ | Đo computed style so `MWork`/`MMore`. |
| F | M1 Viết | ✓/✗ | running/gate/idle/paused/error/empty/offline/expired ✓ (expired: `expired.test.tsx`, không đóng được, giữ nháp). ✗ done, typing, chip thu lại khi cuộn chưa chụp. |
| F | M2 Đọc · M3 Nghe | ✓/✗ | Menu ⋯ + danh sách chương ✓ (mục “Chép đường dẫn tệp” chỉ quản trị: vitest). Nghe: list/player/export/running ✓. ✗ Đọc tập trung, thẻ chương tiếp, sheet góp ý trên bàn phím, sheet chữ; Nghe export-done. |
| F | M4 Thêm | ✓/✗ | home, accounts, Quy tắc viết, Tuỳ chỉnh truyện ✓. ✗ storyset/password/guide/devices/model/projects/firstPwd chưa đặt cạnh artboard. |
| F | M5 Kết quả lệnh | ✓ | cocreate, import (preview/guide/run), diag, sync check/done, reopen/bị chặn; id việc nền khớp backend (`qualifyJobId` test). |
| F | M6 Nhật ký + GAP-31 | ✓ | Bộ lọc, tìm tô sáng, tiêu đề dính, nhãn công cụ, pill “N sự kiện mới” (vị trí = artboard), toast `bottom:30px`; `TestActivityHistoryToolDetailAndMobileFilters`. |
| F | M7 Thông báo + GAP-30 | ✓ | 7 màn so 390×844; `needsHomeScreenInstall` có fail-before; 11 lệch push sửa kèm test Go. |
| F | T1 Máy tính bảng | ✓/✗ | 4/5 trạng thái đo ở 834×1194. ✗ vị trí toast thành công, Nghe/Thêm 560 px chưa chụp. |
| G | GAP-22…29 | ✓ | Mục D ở trên (đều Đóng). |
| G | Đối chiếu Slash/Commands/Jobs/CmdResults/Kit/Transcript/AudioExport | ✓/✗ | Jobs 7 kịch bản, CmdResults 10 view, Kit confirm/select/menu đo khớp. ✗ hover/focus/bàn phím ở Commands & Kit; Transcript stale/drafting/allDone/một giọng; AudioExport confirm/paused/error; Slash `busy=false`. |
| H | Phần mã ≥ 200 px | ✓ | `ui/QrCode.test.tsx` (mọi phiên bản, ≥ 7 px/ô, lề 4 ô). |
| H | Quét được bằng iPhone/Android ở 40 cm | ✗ | Cần thiết bị thật — chủ dự án kiểm. |
| H | Không phần tử nào trong vùng mã | ✓ | `QrCode.test.tsx`: lề nằm trong SVG, không padding/opacity/filter trên ô trắng; lớp hết hạn chỉ khi `expired`. |
| I | Cài mới gợi ý “Tài liệu › Truyện Noverse”, tạo đúng thư mục | ✓ | `serve` thật với HOME tạm: tạo `Documents/Truyện Noverse`, `serve.json` ghi đúng. |
| I | Chọn iCloud/ổ ngoài từ điện thoại | ✓ | Hộp chọn trong app ở 390: tạo thư mục trong iCloud Drive, `kind: empty`. |
| I | Không hiện “lib”, “~/Truyen”, `/Users/...` | ✓ | grep `web/src` + `rootDisplay` ở mọi màn; `GET /api/library` bỏ `root`. |
| I | Thư mục chỉ đọc bị chặn; `serve.json` chỉ ghi khi PUT thành công | ✓ | Test Go (5 đột biến fail-before). |
| J | Phiên thật: không `snake_case`, mã trợ lý, `retry/reroute/abort`, `Ch.` | ✓ | Bản sao truyện thật `01-tra-xanh` (8 chương, 368 sự kiện) chạy `serve`: Bàn viết, Nhật ký (cuộn hết lịch sử), Dự án ở 1440 và Nhật ký ở 390 đều 0 lần. Chữ “editor” duy nhất là cụm “Bản editor” trích từ quy tắc người dùng. Hai lỗi tìm ra khi kiểm đã sửa: Nhật ký máy tính không nạp lịch sử (`api/useActivityHistory.ts`, dùng chung cho điện thoại); lỗi thô của engine và mã người lập kế hoạch lọt vào câu (`lib/labels.ts`, `labels.test.ts` với fixture `real-activity.fixture.json`). |
| J | Không chữ Việt nào dùng font mono | ✓ | Duyệt `getComputedStyle(el).fontFamily` mọi nút chữ có dấu trên 8 route máy tính, điện thoại, máy tính bảng: 0 lần. |
| J | Công cụ mới chưa có trong bảng vẫn hiện được | ✓ | `labels.test`, `activity/mobile/logic.test`. |
| J | Đồng hồ, ∞, chip bước không xê dịch khi số đổi | ✓ | Đồng hồ `role="timer"`, `min-width:5ch`; đo rect sau 3 và 6 giây: lệch 0 px. |
| J | ThinkingOrb chỉ có chấm, đứng yên khi Giảm chuyển động | ✓ | `ui/ThinkingOrb.tsx` (chuyển từ `startOrbit()`), vẽ khung đầu khi mount, đọc reduced-motion mỗi khung. |
| J | “Người viết” tài khoản và trợ lý không đứng cạnh nhau | ✓ | Menu tài khoản luôn kèm chữ “tài khoản”. |
| K | Công tắc vượt đích, chấm radio nở, dấu ✓ được vẽ, nền nhóm chọn trượt | ✓ | `ui/controls.tsx`, `ui/Tabs.tsx` theo khối `motion:controls v1`; lấy mẫu `getAnimations()` (công tắc 14,6 → 14 px trong 300 ms; chấm 0 → 1,09 → 1; ✓ 28 → 0 trong 260 ms). Bộ lọc MLog giữ chip của artboard, không có nền trượt. |
| K | Đồng hồ hoặc SSE chạy nền không làm mục đã chọn nảy lại | ✓/✗ | ✓ Đếm ngược QR và Bàn viết điện thoại: 0 chuyển động mới. ✗ Chưa thử SSE của một phiên đang viết thật. |
| K | Giảm chuyển động trên macOS/iOS | ✓/✗ | ✓ Giả lập `reduce` trên Chrome: animation `none`, chỉ đổi màu 200 ms. ✗ Chưa thử trên Mac/iPhone thật. |
| K | Space/Enter, phím mũi tên ra cùng chuyển động | ✓ | Gõ phím thật trên Chrome: công tắc 300 ms, `mo-dot` 320 ms. |

**Chờ chủ dự án quyết:** tạo quản trị đầu tiên trên web (gói B, chưa làm); hộp thoại `locked` có chữ “TUI”; `/simulate` nhắc thư mục; Esc một lần hay hai lần; `/reopen` ẩn với người không có quyền viết; thẻ khôi phục khi đã có mã chưa dùng (409); `--keep` có ở registry nhưng artboard Commands chỉ vẽ `--overwrite`; gói G đã bỏ “Bỏ đoạn”, xoá cờ ở Nghe và các ô chương/phụ đề/m4b/khi đã có ở hộp xác nhận xuất audio vì artboard không vẽ; khung “chưa chắc” trên điện thoại; trạng thái cocreate trống; mốc giờ dòng hệ thống cocreate; nền tảng thiết bị trong danh sách push; breakpoint 760 px máy tính bảng; lỗi 500 ở “hiểu quy tắc”.

## F. Nghiệm thu gói L–M · đợt 1 (25/10)

Chạy cuối 25/10 21:25: `go build ./...` · `go vet ./...` · `go test -p 4 -count=1 ./...` exit 0 · `gofmt -l` chỉ `internal/entry/tui/panels_outline.go` (ngoài phạm vi) · `tsc --noEmit` sạch · vitest 104 tệp / 943 test · `npm run build` qua. BE đợt 2 đã xong trước: GAP-37 (`/frame`, `/retention`), `GET …/metrics`, chi tiết gói trong `/api/config/presets`, `releases[]` ở `/api/system`, `builds[]` có `commit`/`at`, `type` quan hệ, `previous[]` vòng chấm. ✓ = đạt có bằng chứng; **đợt 2/đợt 3** = chưa giao đợt này, không tính ✗.

| Gói | Mục nghiệm thu | | Bằng chứng / ghi chú |
|---|---|---|---|
| L | 1 · D1 Phiên bản | ✓ | Đo 1440: banner `820×64` nền `rgba(157,176,247,.07)` viền `.22`; chân thanh bên `ainovel 0.9.3` h34 + badge “Có bản mới” `75×20`; chấm avatar; menu `Có bản mới · 0.9.4 / Xem`; “Có gì mới” 560/r20 có mục markdown; Nâng cao nhóm “Phiên bản Noverse” mở mặc định; Thông tin `ainovel 0.9.2 · Chương 1–5 · a41c9e2 · 20/10/2026`. Nguồn: `/api/system` + `releases[]` (`internal/version/changelog.go`), `builds[]` (`internal/entry/web/story_builds.go`, `projects.go`). |
| L | 2 · D2 Gói thể loại | ✓ | Thẻ gói 254px, `Gói … gồm` (23 dòng § kèm số bắt sửa/cảnh báo, 8 chip số đo, hàng giọng đọc), chọn gói thì bỏ chọn kiểu viết, một khoá `style`. Nguồn `packs[]` chi tiết (`internal/entry/web/packdetail.go`). Lệch dữ liệu: artboard để B/C “Sắp có”, BE thấy cả 3 gói đủ luật → cả 3 “Có chấm” (câu hỏi 7). |
| L | 3 · D3 Bản đọc TTS | ✓ | `Reader fmt=narr` đo khớp sau 6 sửa (nhóm định dạng 30px, ô “Ghi đè tệp đã có” 13/`#C9C9D1`, tên `… · Tập 1 · bản đọc.txt`); ma trận Dự án có cột “Bản đọc TTS”; menu chương điện thoại bước 2; Hướng dẫn `/export` có `doc-so`, `--keep`; `overwrite` → `onExists`. ✗ chưa đo `CmdResults` exportRun/exportDone/exportExists (tab treo) — chỉ test + chuỗi. |
| L | 4 · D4 Thẻ thẩm duyệt | ✓ | `web/src/ui/ReviewIssue.tsx` đo khớp `Reader tab=review` + `Main phase=gate` (chip 20px, § nền `.06`, Literata 13, nút 28px, dải “lần chấm trước” nền `.06`); Activity lọc “Thẩm duyệt”; MLog `?loc=review` chọn sẵn “Biên tập”; BE `items/round/pack/blocking/previous`, cửa duyệt 409. |
| L | 5 · D5 Khung truyện + bản đồ giữ chân | **đợt 2** | BE xong: `GET/PUT …/frame`, `GET …/retention` (`internal/entry/web/retention_api.go`); màn `Outline omap` chưa làm. |
| L | 6 · D6 Số đo bằng máy | **đợt 2** | BE xong: `GET …/metrics?chapters=last3\|all` (`metrics_api.go`); rail Cài đặt + Nhật ký chưa nối. |
| L | 7 · D7–D9 Xưng hô | ✓ | Trang nhân vật đo khớp (`Outline tab=cast`: bảng `165 92 82 92 225 64`, dòng hết hiệu lực gạch xám, ô chương đang viết viền tím, banner khoá nút mờ .45, hộp xác nhận 2 radio); BE 423 `{writing:{chapter,role}}`, 409 `{affected}` (FE mở lại cùng hộp), `forward` = max(fromChapter, latestWritten+1); lint → mục `ref:"Xưng hô"`. |
| L | 8 · D10 Não truyện | **đợt 2** | BE xong: `GET …/brain` (`internal/entry/web/nao_api.go`); `Brain`/`StoryGraph` chưa làm. |
| L | 9 · D11 Audio theo gói | **đợt 2** | Chưa giao đợt 1; `packs[].voice` đã có trong `/api/config/presets`. |
| L | 10 · Quyết định 6 | ✓ | 7 bước quản trị, “Bước 1 / 7”; xoá plumbing ngôn ngữ (bước, `LANGUAGES`, `saveLanguage`, dòng tổng kết); rail “Lần chạy đầu ✓✓” (`aria-label`, nền `.04`, viền `.16`, r14, p10-12); chip Văn bản/Ebook/Bản đọc TTS, `narration` vào `library.autoExport.formats`. |
| L | 11 · Câu hỏi treo | ✓/✗ | `locked` bỏ “TUI”; hộp `recovery` đo khớp `Dialogs state=recovery` (480/p22/r18, mã `TRAX-••••-••••-41ZN`, nút 36px); `/simulate` không còn tên thư mục (BE `internal/host/sim/*`, `internal/commands/specs.go`, FE); Esc chỉ đóng lớp trên cùng (`ui/Overlays.tsx`, live: giữ nháp `/reopen`); `/reopen` ẩn + báo “Bạn chỉ có quyền đọc truyện này”; rules 500 thử lại 3 lần (0/10/20 s) rồi lưu nguyên văn (đo khớp `.rule`); `--keep` có trong Hướng dẫn lệnh. ✗ `locked` chưa dựng lại được trong mock (chỉ unit test). |
| L | Nghiệm thu 1 · mở artboard với tweak, so 1:1 | ✓/✗ | Đo `getComputedStyle`/`getBoundingClientRect` cho `Setup`, `Reader` (review, export), `Main gate`, `Activity`, `MLog`, `Settings` (D1, D2, adv, info), `Outline` (cast, cột phải), `MReader exportMenu`, `MMore storyset`. Artboard không render được trong trình duyệt (`support.js` thiếu) → so với giá trị trong mã artboard. ✗ chưa đo: `CmdResults` export*, `Commands` `/export` + `/reopen`, khối “Văn phong của bạn”. |
| L | Nghiệm thu 2 · không đường dẫn/tên tệp/cú pháp ngoài Nâng cao; không chữ Việt trong mono | ✓ | `noTech.test.tsx` + rà mono: 0 chữ Việt trong font mono (trừ tên tham số); `[giờ]` → `--:--`; ví dụ `/importsim` `van-phong/co-trang.json` bọc `data-tech` (`PathDialog.tsx`) vì là cú pháp lệnh; badge “Cần quyền viết” dùng `edit-2` (artboard `.i-edit`). |
| L | Nghiệm thu 3 · chương còn Bắt sửa không duyệt được; chấm lại sạch thì mở | ✓ | `TestReviewApproveGateBlocksErrorItems`, `TestReviewApproveGateNotificationInherits` (409 rồi 200 sau vòng sạch), `TestReviewPreviousRoundsAndGateUsesLatest`; FE tắt nút theo `blocking`. |
| L | Nghiệm thu 4 · sửa xưng hô chạm chương đã viết hiện hộp xác nhận; engine viết thì khoá | ✓ | Hộp hiện với `fromChapter ≤ latestWritten` (đo), 409 máy chủ mở lại cùng hộp; `TestNaoAddressTermNeedsApplyWhenTouchingWritten`, `TestNaoWriteLockedWhileEngineWrites`; nút sửa mờ .45 khi engine viết. |
| L | Nghiệm thu 5 · Não truyện đổi chương, chữ không tràn, Tab/Enter, giảm chuyển động | **đợt 2** | Màn `Brain` chưa làm. |
| M | 1 · A0 Lần chạy đầu (GAP-41) | ✓ | BE: 8 test + smoke binary thật (`Mã thiết lập: GHFR-4ZUR`, không prompt, 409 `{at}` sau khi có admin, không body nào chứa mã); FE: đủ 8 trạng thái, đo khớp `Setup` (chênh cao thẻ 340/263 = placeholder dòng message ẩn của design mode). |
| M | Nghiệm thu 1 · không gõ gì trong terminal; mã sai/khoá/hết hạn/đã có quản trị đúng trạng thái | ✓ | Smoke non-tty: chỉ in địa chỉ + mã; sai → `codeWrong` còn 4 lần; khoá → `codeLocked` + giờ mở lại bằng chữ; hết hạn → `codeExpired`; đã có admin → `taken`. |
| M | Nghiệm thu 2 · mã không vào phản hồi/log/URL, so sánh hằng thời gian, `needed=false` + 409 | ✓ | `subtle.ConstantTimeCompare` (mã và token); test “body không chứa mã”; `needed:false` + mọi POST 409; `version` chỉ trả khi `needed`. |
| M | Nghiệm thu 8 · chữ không tràn thẻ; không chữ Việt trong mono | ✓ | Thẻ cắt “…” + `title`, đo sau `document.fonts.ready`; mono 0 lần; ngoại lệ chỉ tên tham số trong cú pháp lệnh. |

| GAP | Trạng thái | Nơi làm / lệch |
|---|---|---|
| 35 · Văn mẫu | Đóng | `internal/entry/web/style_samples.go` — `GET/POST …/style-samples`, `DELETE …/style-samples/{id}` (JSON hoặc multipart `.txt`), 20 mẫu → 409, người chỉ đọc 403 (cả GET); FE `settings/sections/StyleSamples.tsx`. Truyện-scoped; `id` = đường dẫn tương đối trong `<truyện>/simulate/`, `title` = tên tệp, `chars` = số từ. |
| 36 · Thẩm duyệt có § | Đóng | `entry/web/review_items.go` (`items/round/pack/blocking/previous[]`), cửa duyệt 409 ở `comments.go` (cả action `approve`), engine `tools/save_review.go` (Bắt sửa ⇒ `RequiresChange` ⇒ `PendingRewrites`), vòng cũ `store/world.go` → `reviews/rounds/`; FE `ui/ReviewIssue.tsx`. |
| 37 · Khung truyện + bản đồ giữ chân | Đóng (BE) · FE đợt 2 | `entry/web/retention_api.go` — `GET/PUT …/frame`, `GET …/retention`; `audience_groups` thêm vào 3 `assets/packs/*/goi.json`; marker `meta/frame_source.json`. |
| 38 · Xưng hô & quan hệ | Đóng | `entry/web/nao_api.go` — `GET/PUT …/address-terms`, `POST …/relations`; 423/409/403/400; `forward` đóng dòng cũ ở chương đã viết cuối; `type` quan hệ thêm vào `domain` + `chapterfacts` + import (cũ/thiếu → `rel`). |
| 39 · Não truyện | Một phần | `entry/web/nao_api.go` `GET …/brain`: `char/fs/ev/rule` có; `place/obj` không có nguồn dữ liệu nào trong kho (câu hỏi 22). |
| 40 · Xuất bản đọc TTS | Đóng | `entry/web/storyread.go` (`format: narration`, `readNumbers`, `onExists`; bỏ `overwrite` → 400), tệp `-narration.txt`, `matrix[].narration`; FE `reader/ExportPanel.tsx`, `cmdresults/cards.tsx`. |
| 41 · Lần chạy đầu trên web | Đóng | `platform/identity/setup.go`, `entry/web/setup.go`, `cmd/ainovel-cli/serve.go` (in mã, bỏ `ensureFirstAdmin`); `GET /api/setup` (+`version` khi `needed`), `/verify`, `/admin`; FE `screens/auth/setup/**`. |
| 42 · Ma trận chất lượng (D13) | Chưa · đợt 3 | — |
| 43 · Sổ cái dự án (D14) | Chưa · đợt 3 | — |
| 44 · Vũ trụ chung (D14) | Chưa · đợt 3 | — |
| 45 · Tìm trong dự án (D14) | Chưa · đợt 3 | — |
| 46 · Não hệ thống (D15) | Chưa · đợt 3 | — |

**Câu hỏi cho design** (gộp từ báo cáo 11 agent, bỏ mục đã tự giải quyết; artboard · tweak trong ngoặc):

1. Mã thiết lập vừa hết hạn vừa bị khoá: artboard không vẽ, BE trả 410 trước 423; mã sai định dạng (dán thiếu ký tự) đang tính vào 5 lần; `GET /api/setup` trả `expiresAt` đã qua khi hết hạn (để FE vẽ `codeExpired`). (`Setup` · `codeExpired`/`codeLocked`/`codeWrong`)
2. `codeLocked`: artboard một câu “khoá 10 phút”, gói M đòi giờ mở lại bằng chữ → đang hiện cả hai; `taken` có “Noverse 0.9.3” nhưng 409 không mang `version`. (`Setup` · `codeLocked`, `taken`)
3. Preview cấu hình ở Onboard vẽ `"formats": ["narr"]`, hợp đồng dùng `narration` → preview hiện `narration`. (`Onboard` · `library`)
4. Nhánh “· main” cạnh commit: `/api/system` không có tên nhánh → bỏ. (`Settings` · `section=adv`)
5. Master 44/51 trên artboard = 43/50 thật + 1 dòng legend (số theo § khớp tuyệt đối) — chốt số nào? (`Settings` · `writing=packOpen`)
6. `overrides` “· 2 mục” không có dữ liệu (mọi `voice` null) → hiện 0; artboard còn vẽ “now · thay cho old” mà BE chỉ có `keys`. (`Settings` · `writing=packOpen`)
7. B/C vẽ “Sắp có” nhưng luật đã đủ → cả 3 gói “Có chấm”; thẻ gói thiếu `desc` và `meta` “8 chương · 1.400–1.600 tiếng/chương” (khoảng tiếng không có trong `goi.json`); mô tả 3 lớp luật không có trong dữ liệu gói. (`Settings` · `writing`; `NewStory` chip)
8. Tên tệp trên đĩa `<truyện>-narration.txt` vs artboard “· bản đọc.txt”: đổi hậu tố (đụng `Tập N.ban-doc.txt` của Audio) hay giữ map ở UI? (`Reader` · `tab=export`, `fmt=narr`)
9. `exportDone` chỉ vẽ cho EPUB: bản đọc TTS dùng badge `TXT` + “Đang xuất bản đọc TTS”; tên không có “· Tập N” (Reader thì có) — cố ý? “Xuất thêm định dạng” để trơ trong artboard → đã nối chạy lại `/export <format>` giữ `from/to`. (`CmdResults` · `exportDone`)
10. `/export` có thêm `format=` mà bảng artboard không vẽ → Hướng dẫn lệnh thêm 1 dòng; `/reopen` chip “Cần quyền viết” không có artboard (tông warn, icon `edit-2`). (`Commands` · `/export`, `/reopen`)
11. “Chưa rõ ai nói” không có nguồn phát (`naolint` bỏ trích không xác định được người nói) — có cần sinh cảnh báo cho mọi trích không rõ không? (`Reader` · `tab=review`, chip “Xưng hô”)
12. `pack.ten` trả nguyên tên gói dài; artboard ghi “theo gói Nữ tần báo thù” — cần nhãn ngắn trong API? (`Main` · `phase=gate`)
13. Cửa duyệt: review global/arc thắng review chương; lỗi `Xưng hô` một mình **không** chặn duyệt (không có đường đưa chương vào hàng chờ sửa) — xác nhận. (`Main` · `phase=gate`)
14. “Giữ nguyên” chưa có chỗ lưu lý do (đang lưu thành bình luận đoạn văn, mất khi tải lại); phụ đề thẩm duyệt có giờ “· 21:52” nhưng API không trả mốc thời gian. (`Reader` · `tab=review`)
15. Kiểu móc cũ (`crisis/mystery/desire/emotion/choice`) vs 4 kiểu Master §7 — cần bảng map chính thức. (`Main`/`Outline` · chương 4)
16. Số đo lệch artboard: “từ thủ tục” (`0,8/1.000 · ≤ 2` vs `n từ · ≤ 10`), “cụm 5 tiếng lặp” (`1 · ≤ 2` vs `n cụm · 0`), “trùng mở đầu” (`≤ 30%` vs `≤ 20%` của gói, chỉ chương 1 so được), dòng “mệt mỏi” (artboard một từ, BE chọn từ mật độ cao nhất). (`Settings` · `metrics`; `Activity` “Đo bằng máy”)
17. `frame.source` không suy được từ dữ liệu (architect và web cùng ghi `story_retention.json`) → thêm marker `meta/frame_source.json`; bỏ được không? (`Outline` · `omap`)
18. Gói C khai người nghe ngoài 5 nhóm → `audience:null`; cần `audienceText`? (`Outline` · `omap` thẻ “Khung truyện”)
19. Chương cuối: artboard chip “—” + “gieo hạt giống phần hai”, `hook.type` là enum đóng → mất câu chờ; dàn ý cũ không có trường giữ chân thì 4 phép kiểm báo “Chưa đánh dấu…” hay UI hiện “Chưa lập bản đồ giữ chân”? Vị trí tính % nguyên (5/8 = 63% → ngoài vùng); dàn ý nhiều lớp chưa mở thì mẫu số lấy `len(outline)`. (`Outline` · `omap`)
20. Chip “người kể · ngôi thứ nhất” đầu trang nhân vật không có trường dữ liệu → bỏ; màu chip quan hệ theo loại (log giờ có `type`, dữ liệu cũ/thiếu → `rel`) trong khi artboard tô 3 màu; lý do quan hệ do engine ghi luôn rỗng. (`Outline` · `tab=cast`, `xedit=detail`)
21. Form “Thêm dòng”/“Ghi thay đổi” và hộp xác nhận cho quan hệ không được vẽ → dùng đúng ngôn ngữ dòng, quan hệ gửi `apply:"mark"`; dòng hết hiệu lực không sửa được (artboard `canEdit: !old`, BE 400) — xác nhận. (`Outline` · `tab=cast`, `xedit`)
22. `place`/`obj` không có nguồn dữ liệu nào trong kho → Não truyện không bao giờ có địa điểm/đồ vật; cần nguồn phía engine. (`Brain` · D10)
23. Chương đang viết lần đầu: hợp đồng 3 trạng thái không có “đang viết” → xếp `unwritten`. (`Brain` · thanh chương)
24. Esc ở popover `/`: artboard vẽ “xoá /…”, `17-kit` nói giữ nháp → đang giữ nháp. (`Slash` · Esc)
25. Mã khôi phục: kho cũ không có `mask` → hiện `••••-••••-••••-••••` thay vì bịa ký tự. (`Dialogs` · `recovery`)
26. “Lưu nguyên văn”: artboard chỉ có chip, khung “chưa chắc” cần `uncertain[]` từ BE → đang để `status:'ready'`; `AudioExport` vẫn vẽ mốc giờ mono trong khi “Font mono (rà 25/10)” mới hơn → làm theo brief, cần vẽ lại artboard; thứ tự “Văn phong của bạn” đang id tăng dần, artboard không nói. (`Settings` · `rules=retry`, `writing`; `AudioExport`)

> **Trả lời của design (25/10):** đủ 26 câu ở `22-brief-2510.md` › *Trả lời 26 câu của dev*. Artboard đã sửa theo câu trả lời. Xem được bằng `artboards/preview.html` (chạy qua HTTP, cách chạy ở đầu `artboards/README.md`). API mới: GAP-47…49 và mục “Bổ sung” trong `06-data-contract.md`. <!-- tra-loi-f -->

## G. Nghiệm thu gói L–M · đợt 2 (25/10)

Gate tích hợp 25/10 ~23:10 (sau đợt 2b, thay kết quả 22:30): `tsc --noEmit` sạch; vitest **113 tệp / 1028 test đạt**; `npm run build`, `go build ./...`, `go vet ./...` đạt; `gofmt -l` chỉ `internal/entry/tui/panels_outline.go`; full `go test -p 4 -count=1 ./...` **exit 0 lúc ~23:40** (integrator báo). Bằng chứng dưới đây tổng hợp báo cáo đợt 2 + 2b (2b thắng khi trùng), không phải một lượt chạy lại của người viết docs. ✓ = phần nghiệm thu nêu trong dòng có bằng chứng; ✓/✗ = còn lệch được ghi rõ. Đường dẫn BE tính từ `internal/`, FE từ `web/src/`; `S` = `/api/projects/{p}/stories/{s}`.

**Rủi ro gate (integrator ghi nhận):** lần chạy trước có hai lỗi chỉ lộ khi tải nặng, không do đợt này: `TestEngine_ExitRaceRestoresPendingDispatch` (chạy riêng đạt 10/10) và `internal/voice` treo 10 phút (tệp sửa lần cuối 02:33, trước các đợt; chạy riêng với `-race` đạt 30/30). Full suite sau đó exit 0 không xoá ghi nhận rủi ro này. Fixture `internal/revision/revision_test.go:205` đã được integrator thêm `places/objects` nullable-required.

| Gói | Mục | ✓/✗ | Bằng chứng |
|---|---|---|---|
| L | 5 · D5 Khung truyện + bản đồ giữ chân | ✓ | `GET/PUT S/frame`, `GET S/retention`; `project/retention/**`, `tabs/OutlineTab.tsx`. Đo thẻ grid `150px 1fr auto`, chip h28/r8, `.seg` h28/r7, dải 8 đoạn, 4 phép kiểm §3/§8/§7/§2, bảng §21. 2b đã lưu `[2,1]` → “Nam trung niên · Nữ 18–35”, tên gói ngắn, chip `seed`, trạng thái rỗng → điền `Lập bản đồ giữ chân: ` và focus dock (không tự gửi). Vị trí dùng phân số gốc, không còn câu hỏi làm tròn. Lệch kỹ thuật còn: PUT frame chỉ phát part `snapshot`, chưa `frame`; FE tự cập nhật tab đang mở. |
| L | 6 · D6 Số đo bằng máy | ✓ | `GET S/metrics?chapters=last3\|all`, `metrics_api.go`, `metric_refs.go`; `settings/sections/rules/MetricsCard.tsx`, `activity/MetricsLogRow.tsx`. 2b: 9 số đo + dòng từ mệt (Settings 10 dòng, 266px/12,5px), badge theo `severity`, phụ theo `ref`, `detail` thay chuẩn khi có; ngoài ch.1 mở đầu = “—”, Activity ẩn mục không áp dụng. Badge Bắt sửa nền `rgba(255,154,162,.12)`/chữ `#FFB3BA`; chip Activity `3px 8px`, r6, không tràn. Lọc/giờ Activity còn câu hỏi #9, không còn thiếu severity/ref. |
| L | 8 · D10 Não truyện máy tính | ✓ | `screens/brain/**`, `GET S/brain`. Đo grid `244/836/360`, thẻ `172×54`…`109×32`, 6 gradient, cạnh `1.2/1.8`, hạt `0.1 22`, pill h21/r10.5/11px; 8 nhãn không đè thân thẻ (gap min 0,84px). GAP-49 đã có nguồn `place/obj`; loại không có dữ liệu ẩn. `writing` chấm tím/tooltip riêng. `mock/brain.ts` là nguồn chung Não, nhân vật, xưng hô, quan hệ, prose và tiêu đề (đã sửa lệch một chương). |
| L | 9 · D11 Audio theo gói | ✓ | `screens/audio/packVoice.ts` chỉ đọc `packs[].audio`, đã bỏ parser câu `voice`. Transcript thẻ r14/p12/g6, badge h20/r6; AudioExport “Một giọng kể · theo gói”; MAudio 390 có badge h19; mẫu giọng không lộ tên tệp. Chế độ phân vai khoá theo gói; giọng vẫn đổi được. Chưa có bằng chứng ép `single` ở BE với cast cũ (câu hỏi #10). |
| L | D12 · Nâng cao › Não truyện | ✓/✗ | `settings/advanced/NaoRows.tsx`, `entry/web/reveal.go`; nhóm thứ 2, 3 hàng, 2 nút h28. `POST /api/reveal {target:"nao-story"\|"nao-project"}` → `{ok:true}`, GET `{canReveal}`, không path; admin/loopback, 404/403 nguyên văn, không tự tạo kho. ✗ số “46/3 ghi chú” chưa có nguồn/API, câu hỏi #7. |
| L | Checklist 1 · artboard/tweak, chữ và trạng thái 1:1 | ✓/✗ | Đợt 2 đo Brain/MBrain/MReader/Transcript/AudioExport/MAudio, D5/D6/D12 theo nguồn artboard; 2b dùng preview thật đo Settings, Reader `keep=asking\|kept`, Outline `xedit=add\|rel` và xác nhận, omap, Activity; keep → tải lại còn lý do → bỏ giữ → tải lại hết. ExportDone TTS đã chạy thật trong mock. ✗ chưa đủ 1:1 cho các câu hỏi dưới; Câu 26b chưa có khối mốc Audio trong app. Không gọi mock là nghiệm thu trích xuất engine thật. |
| L | Checklist 2 · không lộ kỹ thuật; không chữ Việt trong mono | ✓ | Sentinel `noTech` đổi thành “Vai của tôi là đồ canh”; reveal GET/POST không path (`reveal_nao_test.go`); tên xuất dùng nhãn thân thiện, mẫu giọng bỏ tên tệp; D11/D12 dùng font thường. Mốc giờ số mono được phép theo Câu 26; tên chương phải font thường. Cú pháp lệnh chỉ ở nơi được checklist cho phép. |
| L | Checklist 5 · đổi chương, chữ, bàn phím, giảm chuyển động | ✓ | Ch.1→8: `5/8/11/12/13/13/13/13` thực thể, `5/7/10/14/16/16/16/16` cạnh (mock). Tên 44 ký tự cắt 126px ≤132; nhãn 50 ký tự cắt 121,2px ≤128; `<title>` trên `<g>`. Font trễ: nameW `76,12→80,08`, pillW `160,41→154,17` sau `loadingdone`; Tab/Enter chọn thẻ; giảm chuyển động 0 animation/0 hạt/không zoom. |
| M | 2 · M8 Não truyện điện thoại | ✓/✗ | 390×844: không SVG, topbar52/back44, chấm10/chương xem×1,4, chip34, sheet r26/max86%/CTA48, tabbar ẩn; MMore có hàng Não truyện. Integrator đo MReader `state=person`: gạch chấm 1,5px/offset4/c1@70%, gọi `/brain?upTo=2`; sheet có vai, “có mặt từ chương 1”, quan hệ, xưng hô, câu “Chỉ hiện những gì đã xảy ra…”, “Mở Não truyện”/“Đọc tiếp”. ✗ `desc` tĩnh vẫn có thể tiết lộ sự kiện ch.4 khi đọc ch.2; 26 câu chưa xử lý (câu hỏi #2). |
| M | Checklist 3 · ch.1→8 đúng nhãn, 390 không đồ thị | ✓ | Mới ở ch.2: Kỳ Dương/Buổi đọc thoại live/Luật thẻ ngẫu nhiên; ch.3: Đỗ Mẫn/Số lạ nhắn tin/Trường quay số ba; ch.4: Bản live đọc thoại; ch.5: Vỏ ổ cứng. Ch.5 “3 thay đổi”; `svgCount=0`. 2b phân biệt `writing`/`rewriting`, tooltip “đang viết”. |
| M | Checklist 8 · không tràn/font chậm, không Việt mono | ✓ | Đo 390: `overflowX=false`, `rowTextOverflow=[]` sau `document.fonts.ready`; dùng chung đo lại chữ/font của Brain; không thêm Việt mono (trừ tham số cú pháp). |

### Trả lời 26 câu → đã làm

| Câu | ✓/✗ | Nơi làm (tệp / endpoint) |
|---|---|---|
| 1 | ✓ | `webauth/setup.go`, `entry/web/setup.go`, `auth/setup/**`: 410 trước 423; GET `/api/setup.expired`; POST `/api/setup/verify` 400 `{format:true}` không mất lượt; FE không gửi mã thiếu 8 ký tự. |
| 2 | ✓ | `auth/setup/model.ts`, `i18n/vi.ts`: “Thử lại lúc HH:MM (sau 10 phút)”; `taken` bỏ footer phiên bản. |
| 3 | ✓ | `screens/onboard/**`: `formats:["txt","epub","narration"]`, nhãn Bản đọc TTS. |
| 4 | ✓ | `settings/advanced/VersionRows.tsx`: bỏ nhánh “· main”, giữ phiên bản/build. |
| 5 | ✓ | `packdetail.go`, `settings/sections/writing.tsx`: đếm Master từ dữ liệu, 22 mục / 43 Bắt sửa / 50 Cảnh báo. |
| 6 | ✓ | `packs[].overrides{count,keys,rows[{key,value}]}`; `writing.tsx` ẩn khi count=0, chỉ giá trị mới khi có. |
| 7 | ✓ | 3 `assets/packs/*/goi.json`, `packdetail.go`, `settings/lib/packs.ts`: `short/desc/meta`, `params[6]`, `layers[].desc`, cả 3 gói có chấm/chọn được. |
| 8 | ✓ | `reader/ExportPanel.tsx`, `cmdresults/cards.tsx` qua `friendlyExportName`: “… · Tập N · bản đọc.txt”; đĩa giữ `-narration.txt`. |
| 9 | ✓ | `cmdresults/cards.tsx`: badge TTS, “Đã xuất bản đọc TTS”, tên có Tập N, đã xuất narration trong mock. |
| 10 | ✓ | `commands/specs.go:228–240`, hướng dẫn/mock spec và `commands/model.test.ts`: có `format=`/`doc-so`; `/reopen` chip warn + `edit-2`. |
| 11 | ✓ | `naolint/address.go`, DTO/mock/types: bỏ loại “Chưa rõ ai nói”; ví dụ lint thật; cũ thiếu chiều → “Bảng xưng hô của truyện”. |
| 12 | ✓ | `settings/lib/packs.ts` `packLabel`: short, không có thì bỏ “Gói ”/cắt sau dấu phẩy; dùng ở Settings/NewStory/StorySet/Outline/Audio/ReviewPanel/GateCard; test applied đã re-pin. |
| 13 | ✓ | **Engine thật:** `tools/commit_chapter.go` lint → lưu vi phạm → `tools/save_review.go` gộp `ref:"Xưng hô"` error vào review, verdict accept→rewrite, `RequiresChange` → `ApplyReviewOutcome` → `store/progress.go` `PendingRewrites`; DTO `blocking:true`, `POST S/next` 409. Review global/arc vẫn thắng review chương; warning không chặn. Tests: `tools/save_review_address_test.go` `TestSaveReviewAddressLintErrorQueuesRewrite`/`TestSaveReviewAddressLintWarningDoesNotBlock`; `entry/web/review_items_test.go` `TestReviewAddressLintAloneBlocksApprove`/`TestReviewAddressItemNotDuplicated`/`TestReviewApproveGateBlocksErrorItems`/`TestReviewPreviousRoundsAndGateUsesLatest`; FE `workspace/gate.test.ts`. Fail-before đã ghi nhận: hàng chờ rỗng thay `[3]`, duyệt sai 200 thay 409; bỏ special-case FE làm test đỏ. |
| 14 | ✓ | `POST/DELETE S/review/items/{id}/keep`, `review_keep.go`, `store/review_decisions.go`, `tools/novel_context.go`, `ui/ReviewIssue.tsx`: lý do lưu/tải lại/bỏ giữ, truyền lần chấm sau; timestamp **flat `entry.at?`**, giữ `round:number` (khác đề xuất `round.at`). |
| 15 | ✓ | `domain/story_retention.go`, `tools/retention.go`: crisis→threat, mystery→info, choice/desire→decision, emotion→line; dữ liệu outline mới chỉ bốn kiểu + seed. Taxonomy commit facts riêng không đổi. |
| 16 | ✓ | `rules/metrics.go`, `metrics_api.go`, `metric_refs.go`, FE metrics: 9 số đo đúng nguồn, thêm `ngay_thu`; đếm số câu/đoạn/quãng quá chuẩn; mở đầu ngoài ch.1 null; `tiredWord{word,per1000,limit,ok}`, `detail?/severity?/ref?`. |
| 17 | ✓ | `retention_api.go`, `meta/frame_source.json`, `FrameCard.tsx`: marker tồn tại sau đọc lại, chip “Bạn đã sửa”. |
| 18 | ✓ | `GET/PUT S/frame.audience:number[]`; cả trường lưu `nguoi_nghe` và đường Kiến trúc sư là mảng; C `[2,1]`; dữ liệu nhãn cũ chuyển lúc đọc, ghi mảng lần lưu sau; editor nhiều nhóm. |
| 19 | ✓ | `GET S/retention`: `seed` ở chương cuối; sai vị trí **bỏ khi đọc + ghi lý do kiểm §7**; `empty:true,checks:[]` khi chưa khai; FE CTA điền dock; so vùng phân số gốc, mẫu số `len(outline)`. |
| 20 | ✓ | `nao_api.go`, `project/cast/**`, `bits.tsx`: web bắt buộc reason, engine thiếu reason ẩn dòng; bỏ chip ngôi kể, màu theo type (enemy đỏ/ally xanh lá/còn lại xanh lam). |
| 21 | ✓ | `PUT S/address-terms`, `POST S/relations`; form add/rel + xác nhận chung; `apply:"next"\|"mark"` (mặc định mark), forward→400; dòng hết hiệu lực PUT→409/ẩn nút sửa. `writing.state` đi cùng khoá 423; chỉ 409 có affected mới mở xác nhận. |
| 22 | ✓ | GAP-49: `chapterfacts/facts.go`, `domain/revision.go`, `nao_api.go`: places/objects → `/brain` place/obj; desktop/mobile ẩn loại rỗng, không bịa. |
| 23 | ✓ | `GET S/brain.chapters[].status`: written/rewriting/writing/unwritten; `GET S/address-terms.writing.state`; FE desktop/mobile chấm tím, tooltip/banner phân biệt đang viết/viết lại. |
| 24 | ✓ | `commands/SlashPopover.tsx`, `commands/reopen.test.ts`: Esc đóng gợi ý, giữ nháp; thêm ký tự mở lại. |
| 25 | ✓ | `dialogs/RecoveryConflictDialog.tsx`: không mask dùng `••••-••••-••••-••••`. |
| 26 | ✓/✗ | GAP-32 `items[].uncertain:boolean` + `uncertainNote`, model/cache đi xuyên; understand/runtime vẫn mảng lý do + uncertainNote; FE khung cam, chưa cờ giữ ready. `StyleSamples.tsx` mới nhất trước. ✗ AudioExport chưa render khối mốc chương (không có nguồn timestamp audio trong contract); quy tắc giờ mono/tên chương thường đã rõ, không hỏi lại font (câu hỏi #11). |

| GAP | Trạng thái | Nơi làm / lệch |
|---|---|---|
| 32 · uncertain bổ sung | Đóng | `userrules/understand.go`, normalize/service/apply, `store/user_rules.go`, `settings/sections/rules/**`: boolean + ghi chú ở item; không nhầm với mảng lý do runtime/understand. |
| 37 · khung/giữ chân | Đóng | `retention_api.go`, `domain/story_retention.go`, `tools/retention*.go`, FE retention: audience[], source, chuyển móc cũ, seed/empty/phân số/4 phép kiểm. Part SSE frame chưa có như ghi ở L5. |
| 38 · xưng hô + lint chặn | Đóng | `nao_api.go` + cast forms (`next\|mark`, reason, expired 409, khoá writing); `save_review.go`/`store/progress.go` + tests Câu 13, `/next` 409. |
| 39 · Não truyện | Một phần | BE/FE desktop/mobile, aliases/timeline, upTo, writing, place/obj đã có. Chưa bảo đảm “không lộ chương sau” cho desc tĩnh/tên chương; không suy `?upTo` đã xử lý desc (câu hỏi #2). |
| 41 · setup bổ sung | Đóng | `webauth/setup.go`, `entry/web/setup.go`, `auth/setup/**`: expiry thắng lock, format không mất lượt, expired flag, HH:MM, taken bỏ phiên bản. |
| 47 · quyết định thẩm duyệt | Đóng | `review_keep.go`, `store/review_decisions.go`, `novel_context.go`, `ReviewIssue.tsx`; POST reason/DELETE204, decision, entry.at thay round.at; tests keep/reject/context, reload đã đo. |
| 48 · gói + audience[] | Đóng | `assets/pack.go`, 3 goi.json, `config.go`/`packdetail.go`, `packLabel`, frame/domain/tools: short/desc/meta/params/layers/overrides; trường lưu cũng mảng. |
| 49 · địa điểm/vật | Đóng | `domain/revision.go` FactEntity; `chapterfacts/facts.go` strict nullable-required places/objects; `nao_api.go` ids `dd:<slug>`/`vat:<slug>`, with[] → rel “Liên quan”, upTo; facts cũ → []; fixture host `engine_test.go:235` và revision `revision_test.go:205` đã thêm null. Có tests schema/validate/store/API, chưa gọi mock là chứng cứ engine extraction chạy thật. |

**Câu hỏi mới cho design:**

1. Người kể/thẻ lớn và màu nhận diện chưa có nguồn: FE chọn char đầu tiên + bảng 6 gradient. Chốt trường narrator/màu hay chấp nhận quy ước FE? (`Brain` · D10, đồ thị/cột phải)
2. **Chống spoiler:** `?upTo=N` cắt thực thể/phase/appears/timeline, nhưng giữ `chapters[]` (mock cuối đã giống BE); `desc` tĩnh nhắc việc ch.4 khi MReader đang đọc ch.2. 26 câu chưa trả lời. Ẩn desc/tên tương lai trong xem nhanh hay cần desc theo chương ở API? (`MReader` · `state=person`; `Brain`/`MBrain` · thanh chương)
3. “Thêm”/sửa quan hệ trên Brain đi `?tab=cast&person=` và tắt với non-char; “Mở trang nhân vật” trên điện thoại cùng đích lại rơi về Bàn viết. Chốt đích desktop và ẩn CTA mobile hay vẽ trang nhân vật mobile? (`Brain` · cột phải; `MBrain` · sheet)
4. Brief ghi “Phục bút · Luật”, artboard ghi “Chi tiết cài cắm · Quy tắc thế giới” (app theo artboard). Chốt bộ chữ dùng chung? (`Brain`/`MBrain` · chip loại)
5. Sheet vẽ `“cô” · trong lời kể`, API không có nguồn “trong lời kể”; app hiện tự xưng từ bảng xưng hô. Đổi artboard sang dữ liệu đang có hay thêm trường? (`MReader` · `state=person`)
6. Avatar sheet vẽ 3 điểm màu + chữ `#3A1424`, CHR chỉ cặp màu (app `c1→c1 45%→c2`, chữ `--color-bg`); `.on` đang tô mọi lần cùng nhân vật thay vì riêng span chạm. Chốt avatar và phạm vi tô? (`MBrain` · sheet; `MReader` · `state=person`)
7. Số “46/3 ghi chú” chưa có nguồn; GET reveal chỉ `{canReveal}`, POST `{ok:true}`. Bỏ số khỏi artboard hay thêm `notes`/shape đếm? (`Settings` · `section=adv`, Não truyện)
8. Nút Sửa từng dòng đang điền `Sửa dòng chương N: ` vào dock (không API ghi dòng); tổng kết khi lệch chỉ “N chương · M cảnh báo”; chip móc trong danh sách chương chưa nối nguồn `/retention`. Chốt đích sửa/câu lệch và dùng nguồn móc hay bỏ chip danh sách? (`Outline` · `omap`, danh sách ch.4)
9. “Đo bằng máy” còn bám `commit_chapter` (giờ nộp, không thời lượng đo riêng), lọc `tool` trong app vs `review` trên artboard. Chốt nhóm lọc và giờ/thời lượng? Severity/ref đã làm, không hỏi lại. (`Activity` · dòng Đo bằng máy)
10. Gói chưa chỉ id giọng đề xuất; giong rỗng → “một giọng kể”, có giới tính → built-in đầu tiên khớp. Cast cũ có thể còn mode cast ở BE dù UI khoá single. Chốt dữ liệu giọng và ép single ở BE hay chỉ UI? (`Transcript`/`AudioExport`/`MAudio` · `voice=pack`)
11. App chưa có danh sách mốc chương Audio, contract chưa có timestamp audio; Câu 26 chỉ chốt font. Cần nguồn để dựng khối hay bỏ khối khỏi artboard? (`AudioExport` · mốc chương)

Không hỏi lại các mục đã được 26 câu chốt: số Master, tên gói, severity/ref, audience[], seed/empty/làm tròn, apply next/mark, lint blocking, writing, reason, keep, format setup và uncertain. Nhãn né thân thẻ giữ đúng yêu cầu D10; `with[]` dùng nhãn nhóm có sẵn “Liên quan”, quan hệ web thiếu type mặc định rel — là ghi chú triển khai, không tự nâng thành câu hỏi mới.

## H. Nghiệm thu gói L–M · đợt 3 (25/10)

Tổng hợp báo cáo 7 agent đợt 3 (follow-up thắng khi trùng), không phải lượt chạy lại của người viết docs. Đường dẫn BE tính từ `internal/`, FE từ `web/src/`; `S` = `/api/projects/{p}/stories/{s}`, `P` = `/api/projects/{p}`. **Gate cuối 25/10:** `go build ./...` đạt; `go vet ./...` đạt; `gofmt -l internal cmd assets` chỉ `internal/entry/tui/panels_outline.go` (ngoài phạm vi); `go test -p 4 -count=1 ./...` **exit 0**, chạy hai lần (150 s và 152 s) — lỗi một lần của `internal/entry/web` không tái hiện; web `npx tsc --noEmit -p .` sạch; `npx vitest run --pool=threads --maxWorkers=3` **119 tệp / 1053 test đạt**; `npm run build` đạt (chỉ cảnh báo chunk > 500 kB có sẵn). Lỗi TS2305 ở `screens/brain/mobile/view.test.ts` đã hết (FeRail6 làm xong). Số dưới đây là báo cáo từng agent; người viết docs không chạy lại. ✓ = có bằng chứng; ✓/✗ = còn lệch ghi rõ.

| Gói | Mục | ✓/✗ | Bằng chứng |
|---|---|---|---|
| M | 3 · D13 Chất lượng truyện (GAP-42) | ✓/✗ | BE `entry/web/quality_api.go` (+`storyread.go`, `projects.go` `openErrors`, `review_items.go`, `assets/packsections.go`); `TestQualityRoundsCountsAndSummary`, `TestQualityChapterStatusesAndEmpty`, `TestQualityUnreadableStory`. FE `screens/quality/**`, route `#/p/:p/s/:s/quality`, `view/level/scope` trong URL, lọc ở client. Todo 3 nhóm (gấp “Sẽ tự sửa”); “Nhờ sửa ở chương này” = bình luận chương (`addParagraphComment`) + hoàn tác; “Giữ nguyên…” = `ReviewKeepForm` chung với Reader (GAP-47); panel phải `ReviewIssue variant="quality"`. Lối vào: nút header (36px, số `#FFB3BA`, ẩn khi 0), link gate trong `workspace/LiveStream.tsx` `GateBody` (chỉ máy tính), link Reader “· N việc cần xem”. `len=40`: 42 cột `220 + 40×38 + 56`, cột đầu sticky (mép trái 25px trước/sau `scrollLeft=600`), `scrollWidth 1440 = innerWidth`. Preview vs app 17 tổ hợp: lệch còn lại do dữ liệu mock. ✗ không có bản điện thoại (không có artboard, câu 13). |
| M | 4 · D14 Não dự án (GAP-43…45) + `NewStory.check` | ✓/✗ | FE `screens/projbrain/**`, route `#/p/:p/brain`, mục thanh bên dự án + dòng phụ, 3 tab `?tab=ledger\|universe\|search`. Đo: tiêu đề x24 y95.75 h33, nội dung y166.25, thẻ sổ cái 1392/r16; ô tìm `720×42`, cột `320/1058` gap 14; `ghi am` tô `ghi âm`. BE `ledger/**`, `entry/web/ledger_api.go`, `tools/ledger_gate.go`, móc `save_foundation.go`/`cocreate_project.go`/`agents/build.go`: kiểm khi lưu brief và trước dàn ý (brief bị chặn giữ làm `candidate`, dàn ý bị chặn bằng mục kiểu review). `NewStory.check`: `newstory/{StoryCheck.tsx,useStoryCheck.ts}`, checking khớp (Nhanh `760×189.06` p16/18 g14; Đồng sáng tác `326×119.48` p12 g8); conflict/fixed cao hơn artboard 24–27px vì 4 phép kiểm (câu 1). ✗ `checks` vũ trụ = `null` (Master §19, câu 4). |
| M | 5 · D15 Não hệ thống (GAP-46) | ✓/✗ | BE `lessons/**`, `entry/web/lessons_api.go`, `userrules/lessons.go`, `auth.go` (`/api/me.pendingLessons`); `TestLessonsCandidateAndAlertBoundaries`, `TestSystemLessons*`. FE `screens/sysbrain/**`, route `#/system/brain`, hàng đầu nhóm HỆ THỐNG · QUẢN TRỊ + badge 18×18, chấm tím 7×7 trên nút Tuỳ chỉnh. 9 tổ hợp tab×gói: 8/9 khớp chữ; đã sửa màu back link và chuông `14×21.7 → 14×14`. Giữ l2 trên máy chủ thật: đếm 4/2/1 → 3/3/1, hoàn tác về 4/2/1. Giữ ghi `~/.ainovel/rules/lessons.json`; Kiến trúc sư nhận bài học + lời nhắc, truyện đã bắt đầu giữ bản đóng băng `meta/lesson_rules.json`. ✗ `origin:user` tắt (câu 7); dòng lý do bỏ của d1 thiếu (câu 8). |
| M | Checklist 3 · `MBrain` ch.1→8, 390 không đồ thị | ✓ | Đã nghiệm thu §G, đợt 3 không đổi. |
| M | Checklist 4 · số ma trận khớp thẻ thẩm duyệt; ô trống = đạt | ✓ | Truyện mock `01-tra-xanh`: ch3 Reader 1/2 = ma trận 1/2 (tổng 3), ch5 1/1 (2), còn lại 0, tổng 5; ô trống “·”. BE `TestQualityRoundsCountsAndSummary` (fail-before `round 1 counts = [0 0 0 0], want [1 2 0 0]`); FE `quality/model.test.ts` ghim mẫu 8 chương của artboard (cột 3 = 4, cột 8 = 1, 12 mở, nhóm 1/5/6; fail-before `rowTotal`); mock trả mục theo thứ tự backend (Master §, rồi `OrderedSectionRefs` của gói, rồi phần còn lại). |
| M | Checklist 5 · sổ cái ẩn truyện không được xem; `ProjBrain.check=conflict` đủ 4 phép kiểm | ✓ | `TestLedgerAPIVisibilityAndCandidate`, `TestLedgerRuleBoundaries` (fail-before `four checks: 0`), `TestLedgerAxisNamesMatchCanonicalSource`; vũ trụ/tìm `TestProjectUniverseHidesUnviewableStory`, `TestProjectSearchEntitiesAndVisibility` (fail-before `lộ truyện ẩn ("secret")`); FE `projbrain.test.tsx` 3 đạt. Thẻ conflict app h215 vs artboard h185 (4 hàng thay 3). |
| M | Checklist 6 · Não hệ thống ẩn với non-admin; bài học không có tên/tình tiết | ✓ | `TestSystemLessonsPermissions` (fail-before `reader GET /api/system/lessons = 200; want 403`), `TestSystemLessonsAggregationPrivacyAndLifecycle`, `TestLessonsEditedTextRejectsNamesAndPlots` (fail-before nhận “Đừng để Lâm Trần. nghỉ ngơi…”). Trên máy chủ thật với `lan`: không có `pendingLessons`, hai API 403, Settings không hàng/nhóm, `#/system/brain` → `#/settings`. |
| M | Checklist 7 · `NewStory` trùng 4 trục chính thì khoá; đổi trục thì được; chỉ Cảnh báo không chặn | ✓ | `TestLedgerAPICheckSuggestAndStartGates` (409 ở bắt đầu nhanh và commit đồng sáng tác, warning-only tới worker); FE `useStoryCheck.test.tsx`, vitest `src/screens/newstory` 4 tệp/14 test; smoke warning-only `startEnabled:true`, `rewritten:true` → `#/p/dit-dang-audio/s/07-truyen-trong-sinh-nu-tan/work`. |
| M | Checklist 8 · không tràn khi font chậm; không Việt mono | ✓ | FeAudit7 rà Quality/ProjBrain/NewStory/SysBrain mọi tweak × 1440/1280, lúc font đang tải (làm trễ 400 ms tệp font) và sau `fonts.ready`: 0 chỗ chữ Việt trong mono, 0 thuật ngữ kỹ thuật (regex FORBIDDEN), 54/54 ca bề rộng trang = khung nhìn. Sửa 3 chỗ tràn bằng `title` đủ câu cho span cắt “…” có sẵn: `quality/index.tsx:166` (tên truyện), `newstory/StoryCheck.tsx:82` (giá trị trục); fail-before “Truyện Dài Bốn Mươi Chương” 174 > 166 với `title=null`, “Khoảnh khắc tỉnh lại” 125 > 122 với `title=null`. Dữ liệu thô: `local://FeAudit7-measurements.json`. Đồng sáng tác conflict vẫn đo khối `326×220.73`, 4 hàng 18.75px, chữ 234px cắt “…” + `title` đủ câu. |

| GAP | Trạng thái | Nơi làm / lệch |
|---|---|---|
| 42 · Ma trận chất lượng (D13) | Đóng | `entry/web/quality_api.go` `GET S/quality`, `StorySummary.openErrors`; FE `screens/quality/**` (`#/p/:p/s/:s/quality`). Trạng thái chương `approved\|rewritten\|rewriting\|previous`; truyện không đọc được → 404. |
| 43 · Sổ cái + luật khoảng cách (D14) | Đóng | `ledger/**`, `entry/web/ledger_api.go`, `tools/ledger_gate.go`; FE `screens/projbrain/**` (`#/p/:p/brain?tab=ledger`), `screens/newstory/{StoryCheck,useStoryCheck}`. Lệch: `day7`/`channelMedianDay7` luôn `null` (câu 5); luật “khác 5 truyện gần nhất ≥ 2 trục” không thành phép kiểm thứ 5 (câu 1). |
| 44 · Vũ trụ chung (D14) | Một phần | `entry/web/universe_api.go` `GET P/universe`; FE `?tab=universe`. Gộp thực thể có ở ≥ 2 truyện; **`checks:null`** tới khi Master §19 có nguồn chấm (câu 4); `name` = tên dự án (câu 3). |
| 45 · Tìm trong dự án (D14) | Đóng | `entry/web/search_api.go` `GET P/search?q=`; FE `?tab=search`. Bỏ dấu/hoa thường, offset UTF-16, 50 đoạn + `total`, 50 thực thể. |
| 46 · Não hệ thống (D15) | Một phần | `lessons/**`, `entry/web/lessons_api.go`, `userrules/lessons.go`, `agents/build.go`; FE `screens/sysbrain/**` (`#/system/brain`), `settings/nav.tsx`, `shell/AppHeader.tsx`. **`origin:user` tắt** (câu 7); **không có dòng lý do bỏ** (câu 8). |

### Sửa theo design 25/10

Ba bản vẽ mới (`rail-v2`, `rail-m8-v3`, `ring-v2`) đã vào app; số đo dưới là báo cáo FeAudit7.

- **Thanh chương `rail-v2` (Brain máy tính).** Vốn đã khớp, đo lại: chấm 10×10 ở mọi trạng thái, chương đang xem phóng 1,4, phần tô dừng đúng chương đang viết (42,86% khi xem ch1/ch7 với chương đang viết = 4), tâm chấm trên tâm đường, số 15px. `brain/ChapterRail.test.tsx`; fail-before `expected '85.714%' to be '42.857…%'`.
- **`rail-m8-v3` (MBrain).** Thay dải chấm bằng ô số `38×40`, khe 4px, sọc 3px (xanh `#A6EDC1` / tím `#C9A6F5` / xám); ô đang xem nền `#EDEDEF`, đang viết thì `#DCC8F8`; chip “Đã viết / Đang viết lại / Đang viết / Chưa viết”; chưa viết có dòng phụ “Theo dàn ý · chưa có trong bản viết”; bỏ nút ‹ › và đường nối; radiogroup đi được bằng mũi tên/Home/End. `brain/mobile/{MobileBrain.tsx,view.ts,mbrain.css}`. **Tạm chờ design:** quá 8 chương thì ô giữ `min-width` 38px và hàng cuộn ngang tới ô đang xem (`m2-reader`, 30 chương: không đè, không tràn trang).
- **`ring-v2`.** `web/src/ui/ProgressRing.tsx` dùng chung (SVG, đầu nét tròn, `pathLength` 100, −90°, 0,5 s, 0% ẩn cung, không xác định = cung 25% đứng yên khi giảm chuyển động) thay mọi vòng conic+mask ở AudioExport, Transcript, Jobs, MAudio, MCmd, MWork, MTablet; cỡ/nét đo khớp preview: 96/7,5 · 22/2,5 · 34/2,5 · 38/2,5 · 72/5 · 84/5 · 148/7 · 28/2,5 · 42/2,5 · 40/3. Cột sóng bán kính 999px, cao tối thiểu 4px, `#C9A6F5` chỉ phần đã phát; sóng AudioExport là phần đã tạo nên giữ màu artboard `#C9C9D1`/`#C9A6F5`/`#2A2A2E`. Thời gian còn: “còn khoảng N phút” trong câu chữ, “còn N phút” trong tâm vòng. MAudio giữ “~” ở tổng thời lượng như D11. Ratchet màu −52. Giữ quầng sáng conic (artboard AudioExport dòng 280), `--grad-run-ring`, đếm ngược toast (đã là SVG đầu tròn).

**Câu hỏi cho design** (gộp từ báo cáo đợt 3, chỉ xung đột thật hoặc thiếu dữ liệu; artboard · tweak trong ngoặc):

1. Artboard vẽ 3 phép kiểm, checklist/D14 đòi 4 → app hiện đủ 4 server trả (ProjBrain thẻ h215 vs h185; NewStory cao hơn 24–27px). Luật gốc dòng 32 còn vế “khác mọi truyện trong 5 truyện gần nhất ≥ 2 trục” → không thành phép kiểm vì D14 chốt đúng 4. Vẽ lại 4 hàng và bỏ hẳn vế này? (`ProjBrain` · `check=conflict\|fixed`; `NewStory` · `check`)
2. Bản gọn Đồng sáng tác cần câu ngắn: server chỉ có `checks[].text` đầy đủ → app cắt một dòng “…” + `title`. Thêm trường (vd. `checks[].short`) hay chấp nhận cắt? (`NewStory` · `check`, Đồng sáng tác)
3. Tên vũ trụ “Thành phố Vân Châu” không có nguồn → trả tên dự án (`""` với dự án ẩn danh). Giữ hay thêm trường riêng? (`ProjBrain` · `tab=universe`)
4. Master §19 cần nguồn chấm: mục Biên tập viên `ref:"Master §19"` lẫn cả bullet “hạt giống phần hai”/“khép trọn mâu thuẫn”; không có dữ liệu đánh dấu truyện là phần sau. Cột “Kiểm” đang “—” + “Chưa kiểm được”, không thẻ vi phạm. (`ProjBrain` · `tab=universe`)
5. Ngày 7 và trung vị kênh: không có nguồn số người nghe → `null`, hiện “—”. Nguồn nào? (`ProjBrain` · `tab=ledger`)
6. Ngưỡng bài học 2 truyện (đề xuất) / 3 truyện có Bắt sửa (chuông) cần chốt; cùng mục khác mức (1 Bắt sửa + 1 Cảnh báo) đang tách nhóm theo gói + mục + mức nên chưa thành bài học — gộp không? (`SysBrain`)
7. `origin:user` cần mẫu sửa của người dùng có cấu trúc (mục luật/mẫu dùng lại được qua truyện); `RuntimeRule` chỉ có chữ thô → không sinh bài học “bạn sửa”. (`SysBrain` · chip nguồn)
8. Lý do bỏ bài học (d1 “· trùng với số đo của gói (tối đa 90 tiếng)”) không có trường → app bỏ vế này. Thêm trường hay bỏ khỏi artboard? (`SysBrain` · `tab=dropped`)
9. Chương viết lại do bình luận người dùng không được ghi nhận → hiện “Đã duyệt”; “Đã viết lại” chỉ khi vòng trước có Bắt sửa. Cần nguồn hay chấp nhận? (`Quality` · `view=matrix`)
10. “Master v3” không có trong API nào → phụ đề “Master + Bộ A”. (`Quality` · phụ đề)
11. “Thêm ghi chú cho Người viết” không làm gì trên artboard → app mở Reader ở chương đó, dùng ô bình luận chương có sẵn. Xác nhận. (`Quality` · panel phải)
12. Reader vẽ cứng “7 việc cần xem”, dữ liệu Quality của chính artboard cho 6 (1 + 5) → app tính từ dữ liệu. (`Reader` · `tab=review`)
13. Không có artboard điện thoại cho Quality → ẩn link gate trên điện thoại, không có màn mobile. Vẽ hay giữ? (`Quality`; `MMain` gate)
14. Tab Chờ duyệt/Đã giữ/Đã bỏ dùng control chọn chung với ProjBrain (đọc là radiogroup) thay vì tablist thật. Chốt vai trò ARIA? (`SysBrain` · tab)
15. Câu nhiều gói (“· 3 gói”, l4/k1) không bao giờ do backend sinh vì bài học nhóm theo từng gói → chỉ có trong mock. Bỏ khỏi artboard hay định nghĩa gộp liên gói? (`SysBrain` · l4, k1)
16. Ô đang viết trên MBrain có “thở” không? Luật `.live` toàn cục của artboard chạm vào ô này, app không áp; nếu có thì chỉ ô đang viết hay cả ô đang xem? (`MBrain` · thanh chương)
17. Quá 8 chương trên MBrain cần luật chính thức: đang giữ `min-width` 38px và cuộn ngang tới ô đang xem. Thu nhỏ ô, xuống dòng, hay giữ cuộn? (`MBrain` · thanh chương)
18. Màu cột sóng: `17-kit` ghi phần đã phát màu tím, artboard AudioExport/Listen lại dùng `#C9C9D1`/`#EDEDEF` → app theo artboard. Chốt bộ màu. (`AudioExport`/`Listen` · sóng)
19. Vòng mini nền của AudioExport là 40px/nét 3,5px, ngoài bảng cỡ của `17-kit`; app dùng `ring-v2` nét 3. Thêm cỡ vào kit hay đổi artboard? (`AudioExport` · vòng nền)
20. Brain máy tính: chỉ đổi `?ch=` trong hash không chọn lại chương (lỗi có trước đợt 3). Sửa hay để? (`Brain` · thanh chương)

> **Trả lời của design (26/10):** đủ 20 câu ở `22-brief-2510.md` › *Trả lời 20 câu của dev (§H)*. Artboard đã sửa theo các câu có ghi “đã sửa”; hợp đồng bổ sung ở `06-data-contract.md` › “Bổ sung §H”. <!-- tra-loi-h -->

## I. Nghiệm thu trả lời §H (26/10)

Gate 26/10 trên trạng thái cuối: `go build ./...`, `go vet ./...` đạt; `gofmt -l` chỉ `internal/entry/tui/panels_outline.go`; `go test -p 4 -count=1 ./...` **exit 0** (160 s); web `tsc --noEmit` sạch, vitest **125 tệp / 1087 test đạt**, `npm run build` đạt. Bảng dưới tổng hợp báo cáo 9 agent đợt H (follow-up thắng khi trùng); người viết docs không chạy lại. Đường dẫn BE tính từ `internal/`, FE từ `web/src/`; `S` = `/api/projects/{p}/stories/{s}`, `P` = `/api/projects/{p}`.

| Câu | ✓/✗ | Nơi làm / bằng chứng |
|---|---|---|
| 1 · Luôn 4 phép kiểm, đạt ghi “Đạt”; bỏ vế “5 truyện gần nhất ≥ 2 trục” | ✓ | BE `ledger/ledger.go` giữ đúng 4 (`combination`, `distance`, `overuse`, `opening`), `TestLedgerAlwaysFourChecksInOrder`; FE `projbrain/index.tsx` `checkRows` vẽ cả 4 theo thứ tự, chip `[Bắt sửa, Đạt, Cảnh báo, Đạt]`; `grep -rni "5 truyện\|≥ 2 trục"` trên ledger/web/tools không còn. Fail-before: bỏ hàng opening → `four checks: 3`. |
| 2 · `checks[].short` do server sinh | ✓ | `ledger.Check.Short` (`json:"short"`), mẫu từng phép kiểm (kể cả `level:"ok"`), đường gộp ` · ` cho phép dùng quá; `TestLedgerCheckShortTemplates`, `ledgerShortInJSON` (GET `candidate`, `POST /ledger/check`, 409 bắt đầu nhanh, 409 đồng sáng tác, cửa tool). FE `LedgerCheck.short` bắt buộc, bản gọn dùng nguyên câu, chỉ cắt “…” + `title` khi `short` rỗng. Fail-before: `combination short="Giống “Hai” ở 4 trục chính." want="… (bộ, nghề, phản diện, vũ khí)."`; `checks without short: {…}`. |
| 3 · Tên vũ trụ = tên dự án, thêm `project.universeName` | ✓/✗ | BE `library.ProjectFile.UniverseName` (`omitempty`), `PUT P` nhận/trim (rỗng = xoá), trả `universeName` ở chi tiết + tóm tắt danh sách; `/universe.name` = `universeName` ‖ tên dự án, dự án ẩn danh `""`; `TestProjectUniverseNameRoundTrip`, `TestProjectUniverseMergesAcrossStories`. FE Settings › Thông tin dự án có hàng “Tên vũ trụ”; lưu “Thành phố Vân Châu” → ProjBrain › Vũ trụ hiện đúng; aside ẩn hàng khi `""`. ✗ `Settings.dc.html` chưa vẽ hàng này (câu hỏi 2). |
| 4 · Nguồn chấm Master §19 = `rule:"s19-recap"`; không cần cờ “phần sau” | ✓ | `domain.ConsistencyIssue.Rule` + `ReviewEntry.RuleAware` (`rule_aware`), schema `save_review` `rule` nullable-required, prompt Biên tập §3d; DTO `items[].rule` (hiện tại + `previous[]`) và `rule_aware` trên entry + từng vòng; `TestSaveReviewPersistsRuleCode`, `TestReviewItemsCarryRuleCode`. `/universe.checks` lấy `ref:"Master §19"` + `rule:"s19-recap"` của mọi truyện dùng chung, gộp vòng hiện tại + `reviews/rounds/`, khử trùng `(truyện, id)`; chưa có vòng `rule_aware` → `null`, đã chấm sạch → `[]`; `TestProjectUniverseChecks`. Fail-before: bỏ lọc `rule` → 2 mục thay 1; bỏ vòng lưu trữ → `checks:[]`; `checks phải là [] khi đã chấm mà sạch: …null`. |
| 5 · Ngày 7: ẩn cả cột khi mọi dòng `null` | ✓ | FE `projbrain/index.tsx` ẩn cột + chú thích trung vị, lưới còn 11 track (`ledgerRowNoDay7`); BE vẫn trả `day7:null`/`channelMedianDay7:null`. Fail-before: `expected 'Kenh · 10 truyệnNão dự án…' not to contain 'Ngày 7'`. |
| 6 · Đề xuất ≥ 2 truyện, chuông ≥ 3 truyện Bắt sửa, gộp mức | ✓ | `lessons.CandidateStories=2`, `AlertStories=3`; `groups()` không tách mức, `evidence.count` đếm cả hai mức, `kind`/level = mức cao nhất; `TestLessonsLevelsMergeIntoOneCandidate`, `TestLessonsCandidateAndAlertBoundaries`. Fail-before: `one Bắt sửa + one Cảnh báo in two stories = 0 lessons; need one`, `two error stories must not alert`. |
| 7 · `origin:user` | ✗ | Design chấp nhận tắt tới khi có góp ý có cấu trúc; `lessons.go` chỉ sinh `origin:"review"`, chip “Từ góp ý của bạn” giữ trong artboard làm đích. |
| 8 · Bỏ lý do khỏi dòng “Đã bỏ” | ✓ | Không có `dropReason` (grep Go/TS = 0); `vi.ts` `droppedNow`/`droppedOn` chỉ ngày; artboard khôi phục đúng chuỗi “Bỏ ngày 19/10 · Noverse không đề xuất lại”. Fail-before (gắn lại lý do): `… not to contain 'trùng với số đo'`. |
| 9 · “Đã viết lại” có nguồn | ✓ | `store/rewrites.go` ghi thêm `meta/rewrites.jsonl` `{chapter, at, cause:"review"\|"user"}` khi chương **mới** vào hàng chờ, trước khi đổi tiến độ; `quality_api.go` trả `chapters[].rewrites` (luôn mảng, cũ→mới) và `rewritten` chỉ khi log khác rỗng; `cause=user` từ góp ý đã duyệt, xưng hô/quan hệ `mark`, `reopen_book`. `TestQualityRewritesFromUserRewriteLog`. Fail-before: `rewrites = [], muốn đúng một mục review cho chương 3`; `chương có log viết lại = "approved", muốn rewritten`. |
| 10 · Bỏ số phiên bản khỏi UI | ✓/✗ | Không còn `Master v*` trong Quality/Settings/MMore; phụ đề “Master + Bộ A”. MMore đổi sang khối gói: `Trợ lý đọc đủ Master + Bộ A · 65 mục bắt sửa, 61 mục cảnh báo` tính từ `pack.layers` (43+22 / 50+11), fail-before `43 … 50` khi chỉ cộng Master. ✗ hàng “1.400–1.600 tiếng” không có trường (câu hỏi 1). |
| 11 · “Thêm ghi chú cho Người viết” mở ô bình luận Reader | ✓ | `paths.readerNote(p,s,ch)` → `…/read/N?note=1`; Reader focus ô bình luận chương có sẵn rồi bỏ tham số khỏi hash; test router. Fail-before: `expected '/p/p/s/s/read/3' to be '…?note=1'`. |
| 12 · “N việc cần xem” tính từ dữ liệu | ✓ | `todoCount` = Đang chặn + Cần xem; test ghim `… · 6 việc cần xem` với dữ liệu artboard; artboard Reader sửa thành “6”. Fail-before: `expected '… 7 việc cần…' to be '… 6 việc cần…'`. |
| 13 · Không có Chất lượng trên điện thoại | ✓ | `LiveStream` truyền `quality={!mobile}`, link gate chỉ máy tính; `GateBody.test.tsx` render cả hai bề rộng. Fail-before: `… not to contain 'Chất lượng cả truyện'`. |
| 14 · Tab = `tablist`, lọc = `radiogroup` | ✓ | `ui/Tabs.tsx` `Segmented` thêm prop `tabs={{id,panel}}` (mặc định vẫn radiogroup). SysBrain 3 tab, ProjBrain 3 tab, Quality Cách xem dùng tablist/tab/tabpanel; Mức/Phạm vi, gói, “Áp dụng cho” giữ radiogroup. `Controls.test.tsx`, `sysbrain.test.tsx`, `projbrain.test.tsx`. Fail-before: `expected null to be truthy` tại `[role=tablist]`; `expected [ 'Cách xem', 'Mức', 'Phạm vi' ] to deeply equal [ 'Mức', 'Phạm vi' ]`. |
| 15 · Gộp liên gói cho mục dùng chung | ✓ | `lessons.IsPackRef` = `^Bộ\s+\S+\s*§`; khoá `(packID, ref)` cho mục `Bộ`, `(ref)` cho `Master §…`/`Định dạng §…`/`Xưng hô`/`Quy tắc của bạn`; `evidence.packsCount` = số gói, `evidence.pack` = tên gói khi `packsCount==1`, `""` khi ≥ 2; scope mặc định `all`. `TestLessonsSharedRefGroupsAcrossPacks`, `TestLessonsPackRefsStayPerPack`, `TestSystemLessonsCrossPackGrouping` (l4 = 9 Cảnh báo/6 truyện/3 gói, k1 = 6 Bắt sửa/4 truyện/2 gói sinh từ dữ liệu). Fail-before: `cross-pack grouping produced 3 lessons`; `+ "l1": "Rút từ 5 lần Bắt sửa ở 3 truyện"` thiếu “· gói Nữ tần báo thù”. |
| 16 · Ô đang viết trên MBrain không “thở” | ✓ | `mbrain.css` `.mb-cell.live{animation:none}` chặn luật `.live` toàn cục; đo 8 chương, live = ch4: mọi ô `animationName:none`, `getAnimations()` = 0 cả cây; chỉ còn transition nền/màu/transform. |
| 17 · Luật thanh chương MBrain | ✓/✗ | ≤ 10 chương: lưới chia đều (`grid-auto-columns: minmax(0,1fr)`), không cuộn; > 10: `flex` + `overflow-x:auto`, `scroll-snap-type:x mandatory`, ô `flex:0 0 38px` snap center, mask 18px hai mép, cuộn tới giữa khi mở và khi đổi chương (`RAIL_SCROLL_OVER=10`, `scrollLeftToCenter`). Đo 390: 8 ch → ô `38×40` x29 gap 4; 30/40 ch → `flex/auto`, mask byte-identical, ô đang xem 195 = tâm hàng 195 (delta 0), giảm chuyển động cuộn tức thì. Fail-before: `expected 966 to be 823`. ✗ artboard `center()` lệch một ô ở `len=30` (câu hỏi 6); hàng cuộn giữ 4px đệm focus ring (clientWidth 340 vs 332, câu hỏi 7). |
| 18 · Màu sóng | ✓ | Token `wave` (`--wave-played:#c9a6f5`, `--wave-unplayed:#2a2a2e`, `--wave-unplayed-phone:rgba(255,255,255,.16)`, `--wave-generated:#c9c9d1`, `--wave-newest:#c9a6f5`); đo khớp artboard ở Listen 120×64 (52/68 thanh), mini “Đã xuất”, Studio “nghe trong lúc chờ”, sóng tạo giọng (3 cột cuối `#C9A6F5`), MAudio 48 thanh; `wave.test.tsx`. Fail-before: `expected 2 to be 3`; `expected … to contain 'background:var(--wave-played)'`. ✗ thanh chờ tạo giọng 3px trong artboard vs 4px tối thiểu của kit (app 4px, câu hỏi 8). |
| 19 · Vòng 40px dùng nét 3px | ✓ | Artboard `AudioExport` đã sửa; app **không có** mặt phẳng mini-player 40px nền nên không dựng gì; vòng trong app: 22/2,5 · 38/2,5 · 72/5 · 84/5 · 96/7,5 · 148/7; MTablet 40×40 nét 3, r18,5 khớp bảng kit. |
| 20 · Đổi `?ch=` phải chọn lại chương | ✓ | `screens/brain/useChapterQuery.ts` (theo mẫu `audio/useAudio.ts`): đọc `?ch=` qua `useRoute`, chọn lại khi query đổi, giữ lựa chọn tay qua lần tải lại, bỏ qua số ngoài phạm vi; một chiều, không ghi ngược hash. `useChapterQuery.test.tsx` 5 test. Fail-before: `expected '2' to be '6'`. |

| GAP | Trạng thái | Nơi làm / lệch |
|---|---|---|
| 44 · Vũ trụ chung | Đóng | `universe_api.go` `universeChecks`: `ref:"Master §19"` + `rule:"s19-recap"`, vòng hiện tại + lưu trữ, bỏ truyện không đọc được, `null` khi chưa vòng `rule_aware` nào, `[]` khi đã chấm sạch; `checks[{ref,story,ch,text,quoteId}]` (quoteId = mã mục GAP-47 → Reader `?quote=`). Thêm `project.universeName`. Còn: mục đã “Giữ nguyên” vẫn nằm trong `checks` (câu hỏi 9; đổi 26/10: đã lọc sang `kept` — xem §J). |
| 46 · Não hệ thống | Đóng (trừ `origin:user`) | Nhóm `(gói, mục)` / `(mục)` liên gói, không tách mức, `packsCount`/`pack`, scope `all` mặc định; ID `lesson-` + `sha256(groupKey)`, **không migration** (đổi 26/10: nay có, `schema:2` — xem §J); không `dropReason`; ref `Xưng hô` dùng tên `naolint.IssueName` thay vì kênh chứa tên nhân vật. Bảng “Mục luật hay vấp” vẫn tính theo gói (số/level/chuông của gói) trong khi bài học mang số liên gói (câu hỏi 10). |

**Câu hỏi cho design (mới)** (chỉ xung đột thật hoặc thiếu dữ liệu; artboard · tweak trong ngoặc):

1. `MMore` hàng thứ hai cần “1.400–1.600 tiếng” nhưng `goi.json` không có trường; khoảng này chỉ nằm trong bảng markdown của luật gói (dòng “Độ dài”), `meta` là “8 chương · thoại 40–55%”. Thêm trường `goi.json` hay bỏ vế khỏi artboard? (`MMore` · `storyset`)
2. `Settings.dc.html` chưa vẽ hàng “Tên vũ trụ” (app dựng theo câu 3, cùng mẫu nhãn/ô của artboard). Vẽ lại, và đặt trên hay dưới “Mô tả”? (`Settings` · Thông tin dự án)
3. Thẻ sổ cái ProjBrain có cột gợi ý sửa căn phải (“Đổi ít nhất 1 trong 4 trục”, “Cân nhắc đổi”) mà API không có trường → không render. Bỏ khỏi artboard hay thêm `checks[].fix`? (`ProjBrain` · `tab=ledger`, `check=conflict`)
4. SysBrain l1/l2 chọn sẵn phạm vi “Gói …” nhưng d1 (cũng một gói) là `all`; theo câu 15 mục dùng chung mặc định “Mọi gói” nên l1/l2 là mẫu cũ. Sửa artboard hay giữ? (`SysBrain` · l1, l2)
5. Artboard l1 ghi “3 truyện” nhưng chip nêu 4 tên truyện — dữ liệu mẫu tự mâu thuẫn; number đếm từ nhóm, chip giữ nguyên chuỗi artboard. Sửa mẫu. (`SysBrain` · l1)
6. `center()` của artboard MBrain lệch một ô ở `len=30` (`offsetLeft` tính từ `.mroot`, không phải hàng) → app cuộn đúng giữa (delta 0). Sửa artboard hay làm theo đúng offset đang vẽ? (`MBrain` · `len=30`)
7. Hàng cuộn MBrain giữ 4px đệm focus ring (`clientWidth` 340 vs artboard 332, mask lệch 4px ra ngoài); bỏ thì vòng focus bị `overflow` cắt. Giữ hay đổi cách vẽ focus? (`MBrain` · `len=30`)
8. Sóng tạo giọng: artboard vẽ thanh chờ 3px, kit đặt tối thiểu 4px → app 4px. Sửa artboard hay hạ kit? (`AudioExport`/`Studio` · sóng tạo)
9. `/universe.checks` vẫn liệt kê mục Master §19 người dùng đã “Giữ nguyên” (GAP-47); design không yêu cầu lọc. Lọc theo quyết định giữ hay giữ nguyên? (`ProjBrain` · `tab=universe`)
10. Bảng “Mục luật hay vấp” của SysBrain tính số/level/chuông **theo gói** trong khi bài học nhóm liên gói → một mục dùng chung có thể hiện 1/4 không chuông cạnh bài học “6 Bắt sửa ở 4 truyện”. Chuông có nên theo nhóm liên gói? (`SysBrain` · aside, mục dùng chung)
11. `lessons.json` còn mục mang id cũ (tách mức) nằm im: GET vẫn trả, `Decide` vẫn chạy. Dọn một lần bằng tay hay để tự nhiên? (`SysBrain` · dữ liệu thật)

> **Trả lời của design (26/10):** đủ 11 câu ở `22-brief-2510.md` › *Trả lời 11 câu của dev (§I)*. Artboard đã sửa: `Settings` (Tên vũ trụ, độ dài chương), `SysBrain` (l1, l2), `MBrain` (canh giữa, vòng focus), `AudioExport` (thanh chờ 4 px). <!-- tra-loi-i -->

## J. Nghiệm thu trả lời §I (26/10)

Gate 26/10 (trạng thái cuối): web `tsc --noEmit` sạch, vitest **126 tệp / 1099 test đạt**, `npm run build` đạt; Go: `go build ./...` đạt, `go vet ./...` đạt, `gofmt -l internal cmd assets` chỉ `internal/entry/tui/panels_outline.go` (ngoài phạm vi), `go test -p 4 -count=1 ./...` **exit 0** (135 s). Bảng dưới tổng hợp báo cáo 7 agent đợt I (follow-up thắng khi trùng); người viết docs không chạy lại. `S`/`P` như §H; BE dưới `internal/`, FE dưới `web/src/`.

| Câu | ✓/✗ | Nơi làm / bằng chứng |
|---|---|---|
| 1 · Độ dài chương từ `goi.json` hiện ở MMore + chi tiết gói | ✓ | `assets/packs/*/goi.json` thêm `tieng_moi_chuong:{min:1400,max:1600}` (đối chiếu dòng “Độ dài” của từng luật gói: A 8 chương, B 7, C 10, đều 1.400–1.600); `assets/pack.go` `PackTiengMoiChuong` + `Pack.TiengMoiChuong`; `GET /api/config/presets` → `packs[].chapterWords:{min,max}\|null` (key luôn có, `null` với gói người dùng thiếu). Drift test `TestPackChapterWordsMatchRules` (đọc lại bảng luật, lỗi khi thiếu/trùng dòng/lệch số), `TestPresetsPacksChapterWordsJSON`. FE `settings/lib/packs.ts` (`chapterWordsText` → “1.400–1.600 tiếng mỗi chương”, chip sau `chapters`, icon `document-text`), `packInfoMeta` → “8 chương · 1.400–1.600 tiếng · thoại 40–55%” cho `MMore`; `meta` thẻ gói không đổi. Fail-before: `chapterWords phải là object, nhận được <nil>`; `goi.json phải khai tieng_moi_chuong`; `expected '1.400-1.600 …' to be '1.400–1.600 …'`; `expected '1.400–1.600 tiếng mỗi chương' to be ''` khi null. |
| 2 · Hàng “Tên vũ trụ” dưới “Mô tả” | ✓ | `screens/settings/sections/info.tsx` `PROJECT_FIELDS`: Tên dự án → Mô tả → **Tên vũ trụ · hiện ở Não dự án** → Ghi chú cho trợ lý; placeholder cố định “Để trống thì dùng tên dự án” (bỏ fallback `form.name`). Đo 1440: nhãn 12,5px `#A0A0AB`, ô 38/14, gap hàng 14, lưới `381 381`; khớp artboard. Fail-before: `expected [ 'Tên dự án', 'Tên vũ trụ', … ] to deeply equal [ 'Tên dự án', 'Mô tả', … ]`; `expected 'Kênh truyện audio' to be 'Để trống thì dùng tên dự án'`. |
| 3 · `checks[].fix` | ✓ | `ledger.Check.Fix *string` (`json:"fix"`, luôn có, `null` khi đạt/không có gợi ý): `combination` error → “Đổi ít nhất 1 trong 4 trục”, `overuse` warning → “Cân nhắc đổi”, còn lại `null` (kể cả `distance`, `opening`, mọi mức). Đi theo mọi payload như `short` (candidate, `POST /ledger/check`, hai cửa 409, kết quả `save_foundation`). FE `LedgerCheck.fix: string\|null`; `projbrain` thêm `[data-check-fix]` căn phải (12px `#A0A0AB`, `flex-shrink:0`, gap 10) **chỉ khi khác null**; `NewStory` không vẽ (artboard không có). Fail-before: `combination fix="" want="Đổi ít nhất 1 trong 4 trục"`; `expected [ null, null, null, null ] to deeply equal [ 'Đổi ít nhất 1 trong 4 trục', … ]`. |
| 4 · Dòng nguồn: mục dùng chung **luôn** “· N gói” | ✓ | `lessons.Aggregate`: `packLabel` chỉ đặt cho `Bộ …`, mục dùng chung `pack:""` + `packsCount` thật (kể cả 1); scope mặc định `all`. Mock `sysbrain`/`i18n` theo đúng: l1 “Rút từ 5 lần Bắt sửa ở 4 truyện · 1 gói”, l2 “Rút từ 7 Cảnh báo ở 3 truyện · 1 gói”, l4 “· 3 gói”, k1 “· 2 gói”; l1/l2 chọn sẵn “Mọi gói”. Fail-before: `shared ref must report packs, never a pack name: scope="all" pack="Gói A" packs=1`; `expected "… ở 4 truyện · 1 gói" / received "… ở 4 truyện"`. |
| 5 · Mẫu l1/l2 khớp chip | ✓ | Mock dựng từ bảng quan sát: l1 “ở 4 truyện” (thêm Mẹ Kế ch.6 vào §7), l2 “ở 3 truyện”, chữ l2 bỏ “của gói này” → “Chương đầu hay thiếu thoại (dưới mức của gói)…”. Fail-before: `- "… 4 truyện · 1 gói" / + "… 3 truyện · 1 gói"`; `Expected "Chương đầu hay thiếu thoại (dưới mức của gói)…" / Received "Chương đầu của gói này hay thiếu thoại (dưới 40%)…"`. |
| 6 · `center()` tính theo khung hàng | ✓ | Artboard sửa; app vốn đúng nên không đổi toán. Đo 390: `len=8` hàng `332×40` x29, ô `38×40` x29/71; `len=30` mask byte-identical, `scrollLeft` 861 khi xem chương 25 (=24·42−147); ô đang xem lệch tâm **0**; chương 1 → `scrollLeft 0`, lệch −147 như artboard (clamp). |
| 7 · Bỏ đệm 4px, focus ring bên trong ô | ✓ | `mbrain.css`: `.mb-cells` bỏ `padding:4px; margin:-4px` → hộp hàng đúng `332×40` (mask tính từ mép thật); `.mb-cell:focus-visible{outline:none; box-shadow:inset 0 0 0 2px var(--color-accent)}` (`#C9A6F5`, không thêm mã màu). Tab thật: `box-shadow: rgb(201,166,245) 0 0 0 2px inset`, `outline-style:none`. Fail-before (chèn lại luật cũ trên trang thật): hàng `x25 y148.75 340×48`, mask cách hàng 4px mọi phía; focus cũ `box-shadow:none, outline: solid 2px`. |
| 8 · Thanh chờ tạo giọng 4px | ✓ | Artboard sửa; app vốn 4px → không sửa. Đo `Studio` (kịch bản `rendering`): cao 4, `min-height` 4, bán kính 999, màu `#2A2A2E` = `var(--wave-unplayed)`; khớp artboard. |
| 9 · Lọc mục đã “Giữ nguyên” khỏi `checks` | ✓ | `universeChecks` → `(checks, kept)`; quyết định đọc qua `reviewContext.entry` (`item.Decision.State == "kept"`), không đọc lại tệp; đáp án thêm `keptCount` + `kept[{ref,story,ch,text,quoteId}]` (**luôn `[]`**), `null`/`[]` của `checks` giữ nguyên; truyện không đọc được không góp mục nào. FE: khi `keptCount > 0`, cuối bảng một dòng “N mục đã giữ nguyên” (`s.note` 12px `#8E8E99`, `aria-expanded`/`aria-controls`) mở danh sách dùng lại `RecapCard`; đo app “2 mục đã giữ nguyên”, mở 2 thẻ 2 link, mép phải thẳng bảng. Fail-before: `kept item still in checks`; `hidden kept item counted: … "keptCount":2`; FE `expected <button …> to be null`. |
| 10 · Chuông mục dùng chung tính trên mọi gói | ✓ | `lessons.Violations`: mục không phải `Bộ` → `alert = len(g.errors) >= 3` (Bắt sửa trên mọi gói), kèm `alertScope:"all"` và `alertStories` (`*int`, `omitempty` — **luôn phát, kể cả 0**); `Bộ …` không có hai khoá này và giữ chuông theo gói; `stories/total/level` vẫn theo gói; `Preamble` đọc `row.Alert` nên mỗi gói dùng chung bộ chuông nhưng số của gói mình. FE nhãn “· mọi gói” 11,5px dưới số + tooltip “Tính trên N truyện ở mọi gói” khi `alertScope === 'all'`. Đo gói A: chuông §7/§11/Xưng hô (như artboard), 5 hàng có nhãn, `Bộ A §4.3` không; gói B: chuông §10 **và §7**; gói C: không chuông. Fail-before: `pack-a · shared row must alert on every pack: … Alert:false AlertScope:`; `warning-only shared row must still report alertStories 0`. |
| 11 · Migration `schema:2` cho `lessons.json` | ✓ | `lessons/migrate.go`: `schemaVersion = 2`, `legacyID` (chỉ trong tệp này), gộp id tách mức về `ID(GroupKey(packID, ref))`, trùng thì `kept > dropped > pending` (giữ `createdAt` sớm nhất, `decidedAt` của bên thắng), bỏ id không khớp cả hai, mục dùng chung còn `pending` về `scope:"all"` (đã quyết giữ phạm vi người dùng), `Bộ …` không chạm; chạy dưới khoá store trước lần đọc đầu, `transaction()` cũng migrate; tệp đã `schema:2` không bị chạm (lần hai byte-identical). Tests `migrate_test.go`. Fail-before: `migrated 3 lessons; need the merged group alone: […3 mục…]`; `dropped must win over pending: {… State:pending …}`; `pending shared ref must default to Mọi gói: {… Scope:pack …}`. |

**Câu hỏi cho design (mới)** (chỉ chỗ chưa có artboard hoặc artboard còn mẫu cũ):

1. Dòng “N mục đã giữ nguyên” + danh sách mở ra ở cuối bảng vũ trụ **không có trong artboard**; app dùng dòng nhỏ sẵn có (`s.note`, 12px `#8E8E99`, đường kẻ trên như hàng bảng) + `RecapCard` nguyên bản. Thẻ đã giữ có nên mang nhãn “đã giữ” không? (`ProjBrain` · `tab=universe`)
2. Nhãn “· mọi gói” và chuông liên gói của SysBrain **chưa được vẽ**; app đặt nhãn 11,5px `--color-fg-4` căn phải dưới số đếm trong cột 54px sẵn có. Vẽ lại cột đếm? (`SysBrain` · aside “Mục luật hay vấp”)
3. Mẫu artboard l3/k2/d1 vẫn ghi nguồn theo kiểu cũ (không hậu tố / “gói Nữ tần báo thù”) trong khi câu 4 chốt “· 1 gói”. Cập nhật mẫu. (`SysBrain` · l3, k2, d1)
4. Gói B: chuông hiện ở §10 **và §7** (câu 10 tính trên mọi gói) nhưng artboard chỉ vẽ §10. Sửa artboard hay giữ chuông liên gói? (`SysBrain` · violations, pack B)

> **Trả lời của design (26/10):** đủ 4 câu ở `22-brief-2510.md` › *Trả lời 4 câu của dev (§J)*. Artboard đã sửa: `ProjBrain` (tweak `kept`), `SysBrain` (cột đếm 64 px, “· mọi gói”, nguồn l3/k2/d1, chuông §7 ở gói B). Đổi thương hiệu: `23-brand.md`, Gói N. <!-- tra-loi-j -->

## K. Nghiệm thu Gói N (Noverse) và trả lời §J (26/10)

Gate 26/10: web `npx tsc --noEmit -p .` sạch, vitest **129 tệp / 1120 test đạt**, `npm run build` đạt (dist mang icon/sw mới); Go `go build ./...` đạt, `go vet ./...` đạt, `gofmt -l internal cmd assets` chỉ `internal/entry/tui/panels_outline.go` (ngoài phạm vi), `go test -p 4 -count=1 ./...` **exit 0** (135 s). Tổng hợp báo cáo 6 agent đợt N (follow-up thắng khi trùng); người viết docs không chạy lại. Đường dẫn FE từ `web/`, BE từ `internal/`.

| Bước Gói N | ✓/✗ | Bằng chứng |
|---|---|---|
| 1 · Biểu tượng | ✓ | `web/public/icons/` 11 tệp mới: `favicon-16/32/48/64.png`, `favicon.ico`, `apple-touch-icon.png` (180), `icon-192.png`, `icon-512.png`, `icon-maskable-512.png`, `noverse-og-image.png` (1200×630), `favicon.svg`; xoá `icon-192.svg`/`icon-512.svg`. |
| 2 · `index.html` + manifest | ✓ | `<title>Noverse · Không gian viết</title>`, icon svg + `favicon.ico sizes=any`, apple-touch 180, `og:title/description/image`; MIME phục vụ: `.ico → image/x-icon`, `.png → image/png`, manifest `application/manifest+json`. `manifest.webmanifest`: `name` “Noverse · Xưởng viết truyện”, `short_name` “Noverse”, PNG 192+512 `any` + `icon-maskable-512.png` `maskable`, các trường khác giữ. |
| 3 · `sw.js` | ✓ | `APP_ASSETS` = manifest + 4 PNG + `favicon.svg`; matcher MIME theo `/icons/*.png`, `favicon.ico`, `favicon.svg`; cache `ainovel-mobile-static-v2 → v3` (**giữ tiền tố**, tên kỹ thuật); icon push → `.png` cả trong `sw.js` lẫn `src/mobile/pwa.ts` (trước trỏ tệp svg đã xoá). `sw.test.ts` canh `APP_ASSETS`; fail-before khi trả `/icons/icon-192.svg`: `expected [ '/manifest.webmanifest', …(5) ] to deeply equal […5…]`. |
| 4 · `BrandMark`/`BrandLockup` | ✓ | `ui/BrandMark.tsx`: SVG nội tuyến từ `noverse-mark(.flat\|-mono-*)`, `viewBox` cắt đúng bao hiển thị (`25 24 560 410`; mono `1 0 560 410`) nên `size` = chiều cao thật, id gradient/mask từ `useId`, props `size`/`variant`/`label`, tự `flat` dưới 32, `aria-hidden` cạnh chữ / `role=img aria-label="Noverse"` khi đứng một mình; `BrandLockup` (mark + Noverse + Studio tuỳ chọn) xuất ở `ui/index.ts`. Test `BrandMark.test.tsx`: fail-before khi bỏ auto-flat (`… to have a length of 16 but got 20`) và khi thay `useId` bằng hằng (`… to have a length of 11 but got 15`). Palette vào nhóm `brand` của `tokens.stylex.ts`; `BrandMark.tsx` 0 mã hex, ratchet màu không đổi baseline. |
| 5 · Logo thanh trên cùng | ✓ | Login/Setup/Onboard/Phone gate/mobile workspace dùng `BrandLockup`; `AppHeader.tsx` thay hàng 4 orb bằng mark 22 px (desktop) và 20 px (`MobileHeader`), `OrbCluster` chỉ còn cho `KitGallery`, xoá `.mw-top-orbs`. Đo (preview → app): Login `28,8×21` → **28,67×21**, gap 10, Noverse↔Studio 6, PJS 16/700 ls **−0,192 px**, Studio BVP 13/500 `rgb(126,126,138)`, baseline **35/35**, `fonts.check('700 16px "Plus Jakarta Sans"')` **true**; Setup y hệt; Onboard gap **14**, không Studio; Phone **27,31×20**, gap 8; app header **30,05×22** (x=60 sau toggle / x=16), `role=img aria-label=Noverse`. |
| 3b · Font hiệu | ✓ | `styles/fonts/plus-jakarta-sans-noverse.woff2` = **2.124 B**, 7 glyph (N o v e r s), trục `wght 200–800`, `pyftsubset … --text=Noverse --flavor=woff2` + `OFL.txt`; `@font-face` “Plus Jakarta Sans” 700–800, `swap`, `unicode-range` 6 ký tự; token `--font-brand`. Tự phục vụ (LAN/offline), không gọi Google Fonts — cùng ý `&text=Noverse` nhưng một tệp 2 KB thay vì CSS+font. |
| 4a · Đổi chữ trên web | ✓ | `grep -rniw ainovel web/src web/mock`: **333 → 143 dòng**, 100% là tên kỹ thuật, 0 dòng chưa phân loại — 61 khoá lưu trữ `ainovel.*`, 60 đường dẫn `.ainovel`, 10 hạ tầng event/cache/mock, 4 `ainovel-cli`, 2 tên tệp `ma-khoi-phuc-ainovel.txt`, 5 fixture “tên cũ” cố ý, 1 dòng Nâng cao. Copy người dùng thấy đều “Noverse” (không bao giờ `NOVERSE`); thư mục gợi ý mới **“Truyện Noverse”** (`FolderBrowserDialog.SUGGESTED_FOLDER`, copy, mock, test). Thư viện cũ “Truyện ainovel” vẫn nhận: Go `library/open.go` → `.ainovel/library.json`, không theo tên (giữ nguyên); test `mock/library.test.ts` + `libraryInspect.test.ts` có fail-before (`expected 'other' to be 'found'`; `expected 'Thư viện cũ không dùng được' to be 'Đã có 1 dự án Noverse…'`). |
| 4b · Đổi chữ phía Go | ✓ | Trang OAuth `auth/oauth.go` (chữ + wordmark thay hàng orb, xoá CSS `.orb/.o-*`; giữ CSP `default-src 'none'`), tiêu đề push (`host/host.go`, `host/engine.go`, `entry/web/notify.go` “Thông báo thử từ Noverse”), chuỗi lỗi/gợi ý hiện trên web (`host/book_lock.go`, `domain/runtime.go`, `entry/web/config.go`, `entry/web/setup.go`, `webauth/setup.go`, `store/nao_vault.go`, `userrules/normalize.go`), gợi ý **“Truyện Noverse”**. Không có mẫu email trên server (`smtp`/`net/mail` = 0) → không có gì để đổi. Giữ: `ainovel-cli login[*]` ở “Chi tiết kỹ thuật” của trang OAuth (`from=cli`). `grep -rnw -i ainovel internal cmd` = 1336 dòng, 978 là module path, còn lại `.ainovel` (246), `ainovel-cli` (67), header `X-Ainovel-*` (18), 27 lặt vặt (temp dir, cookie, env, seed). Test mới `TestInspectDetectsLibraryByNameIndependently` (cùng nội dung dưới “Truyện ainovel” và “Truyện Noverse”, hai bố cục) + guard trang OAuth (phải có wordmark, không còn “ainovel” ngoài lệnh). |
| 5 · Nghiệm thu | ✓/✗ | `grep -rniw ainovel web/src` chỉ còn tên kỹ thuật; dòng không-khớp-mẫu cuối là tiêu đề test `mobile/sw.test.ts:54` (tiền tố cache). CDP: `Page.getAppManifest` → `errors: []`, icon parse có `icon-maskable-512.png purpose=maskable`; `Page.getInstallabilityErrors` → **[]**. Favicon 16 px phóng ×8 (`pixelated`): đọc được 4 quả cầu (lam trái, tím trên, cam phải, mint dưới), 32 px ×4 và `.ico` như vậy. Ảnh: `/tmp/nv/{login,onboard,setup,work,project,audio}-{1440,390}.png`, `setup-header-390.png`, `favicon-8x.png`. ✗ Onboard 390 không đo được: bố cục điện thoại chuyển `#/welcome` → `#/` khi không `mustChangePassword` (câu 6). `internal/entry/web/dist/` đã build lại (icon mới + `sw.js` v3) và `go build` nhúng bản mới. |

| Câu §J | ✓/✗ | Nơi làm / bằng chứng |
|---|---|---|
| 1 · `kept[]` mang quyết định GAP-47 | ✓ | `entry/web/universe_api.go` `universeKeptCheck` (nhúng `universeCheck`, JSON phẳng) thêm `reason`/`decidedBy`/`decidedAt`, **luôn phát**, `""` với bản ghi cũ — không `omitempty` nên FE không có nhánh `undefined`; nguồn là `reviews/decisions.json` (`ReviewDecision{state,reason,by,at,…}`, một nơi ghi: `review_keep.go:95`). Không cần đổi store/handler. Bỏ giữ: `DELETE …/review/items/{itemId}/keep` → **204**, xoá quyết định, mục về `checks` và `keptCount` giảm ngay ở GET sau. FE `projbrain`: dòng “N mục đã giữ nguyên” (✓ 13 px, mũi tên 12 px xoay `.15s`, 12 px `--kit-fg-icon`, không viền, `aria-expanded`/`aria-controls`) nằm cột phải ngay dưới thẻ nhắc cuối; mở ra danh sách `RecapCard` biến thể *đã giữ* (viền `--line-2`, nền `--kit-surface-18`, ✓ xám, chip “Đã giữ nguyên”, dòng `Lý do: “…” · <decidedBy>, <dd/mm>` bỏ từng đoạn rỗng, chân thẻ “Mở đoạn đó ›” + “Bỏ giữ”); hết mục → “Không còn mục nào đã giữ nguyên”, `keptCount === 0` từ đầu thì ẩn dòng. Fail-before: `kept decision fields = {…} Reason: DecidedBy: DecidedAt:`; `keptCount sau bỏ giữ = 1 (… "checks":[],"keptCount":1 …)`; FE `expected '2 mục đã giữ nguyên' to be '1 mục đã giữ nguyên'`; `expected '<section data-kept=""…' not to contain 'data-kept-reason'`; `TypeError: Cannot read properties of null (reading 'textContent')` khi tái hiện lỗi ẩn/hiện. |
| 2 · Nhãn “· mọi gói” + tooltip + chú giải | ✓ | Cột số **64 px**; nhãn chỉ hiện khi `alertScope === "all"` **và** chuông bật; tooltip chung: “Bắt sửa ở N truyện, tính trên mọi gói · được nhắc trước khi viết truyện mới” (N = `alertStories`), tooltip gói: “Bắt sửa ở n/tổng truyện của gói · …”; chú giải thêm “· mọi gói: mục dùng chung, chuông tính trên cả 3 gói” (số gói lấy từ `packs.length`, ẩn tới khi có dữ liệu). Đo 9 trạng thái (3 tab × 3 gói): lưới `90px 212px 64px`, số 12,5 px `rgb(237,237,239)` `tabular-nums`, chuông 14×14 `rgb(255,179,186)`, nhãn 11,5 px `rgb(142,142,153)` mép phải thẳng ô — **giống hệt** artboard; gói B: §10 `3/4` + nhãn, §7 `1/4` + nhãn (“…ở 6 truyện, tính trên mọi gói…”); gói A: §7/§11/Xưng hô có chuông, §13/`Bộ A §4.3`/§5 không; gói C: không. Fail-before: `expected [false,true] / received [false,false]` (nhãn hiện khi chuông tắt); tooltip bị ép về dạng gói (`- "Bắt sửa ở 6 truyện, tính trên mọi gói…" / + "…5/6 truyện của gói…"`); mất chú giải (`Expected: "· mọi gói: mục dùng chung, chuông tính trên cả 3 gói"`). |
| 3 · Mẫu mock l3/k2/d1 | ✓ | Mock `lessons.ts`: l3 `packsCount: 2` (“· 2 gói”), k2 và d1 “· 1 gói”, dòng “Giữ ngày 18/10 · quy tắc của gói Nữ tần báo thù” của k2 giữ nguyên. Fail-before (l3 2→1): `expected [ 'pending', 'nu-tan-bao-thu', …(1) ] to deeply equal …` (nguồn `· 1 gói` thay `· 2 gói`). |
| 4 · `Preamble` dùng số liên gói | ✓ | `lessons.Preamble`: hàng `alertScope=="all"` và `*alertStories != Stories` → “N truyện ở mọi gói”, còn lại “n/tổng truyện”. Chuỗi đo được, khớp artboard từng byte — A: `Master §7 (…, 6 truyện ở mọi gói), Master §11 (…, 3/6 truyện), Xưng hô (…, 3/6 truyện)`; B: `Master §10 (…, 3/4 truyện), Master §7 (…, 6 truyện ở mọi gói)`. Fail-before: `thiếu "Master §7 (đầu chương và cuối chương, 6 truyện ở mọi gói)" trong lời nhắc: … 1/4 truyện…`. Mock SysBrain phản chiếu cùng luật. |

**Câu hỏi cho design (mới)** (chỉ chỗ luật và hình vẽ còn lệch):

1. `23-brand` §1 nói bản phẳng dưới 32 px, nhưng artboard vẽ bản “sáng” (viền + highlight + đổ bóng) ở các thanh 20–22 px; app theo luật nên thanh trên cùng dùng bản phẳng. Đổi luật (cho phép bản sáng ở ≥ 20) hay vẽ lại artboard? (`Login`/`Setup`/`Onboard`/`Phone`/`_appheader`)
2. Đề bài Gói N ghi logo Login 22 px, còn §4 và artboard ghi **21**; app dùng 21 (số acceptance đo). Chốt 21. (`Login`; `23-brand` §3)
3. Wordmark chào của TUI vẫn là “A I N O V E L” (`internal/entry/tui/panels.go:142`) — Gói N khoanh phạm vi web. Có đổi terminal sang Noverse không (kèm `ainovel-cli <ver>`, “chạy ainovel-cli update”)? (`TUI`)
4. Tên tệp mã khôi phục tải về vẫn `ma-khoi-phuc-ainovel.txt` (được xếp vào “tên tệp” phải giữ), nhưng người dùng thấy nó trong Downloads. Đổi thành `ma-khoi-phuc-noverse.txt`? (`Onboard` · mã khôi phục; `23-brand` §5)
5. Server không có mẫu email nào → không có gì để đổi chữ; nếu sau này thêm email thì áp cùng luật. (`23-brand` §5, mục email)
6. Onboard 390 không vào được trong app (điện thoại chuyển `#/welcome` → `#/`), nên chỉ đo Onboard 1440. Muốn đo 390 thì phải mở đường vào hay bỏ khung 390 khỏi artboard? (`Onboard`)

> **Trả lời của design (26/10):** đủ 4 câu ở `22-brief-2510.md` › *Trả lời 4 câu của dev (§K)*. Artboard: logo thanh trên cùng đổi sang bản phẳng, 22 px trên máy tính, 20 px trên điện thoại. `23-brand.md` §1 · §4 · §5 và Gói N đã cập nhật. <!-- tra-loi-k -->

## L. Nghiệm thu trả lời §K (26/10)

Áp dụng 4 câu trả lời §K: bản phẳng dưới 32 px (kể cả thanh trên cùng), một cỡ 22 px máy tính / 20 px điện thoại, đổi TUI/CLI, đổi tên tệp mã khôi phục. **Gate 26/10:** web `tsc` sạch, vitest **129 tệp / 1121 test đạt**, `npm run build` đạt; Go `go build ./...`/`go vet ./...` đạt, `gofmt -l` chỉ `panels_outline.go`, `go test -p 4 -count=1 ./...` **exit 0** (135 s). Tổng hợp báo cáo agent đợt L; người viết docs không chạy lại.

| Câu | ✓/✗ | Nơi làm / bằng chứng |
|---|---|---|
| 1 · Bản phẳng dưới 32 px, giữ nguyên luật | ✓ | `ui/BrandMark.tsx` không đổi nhánh auto-flat; bản phẳng bắt đầu từ **màu thứ hai** của dải, không bóng/không `filter`. `BrandMark.test.tsx` ghim ở 22 px: `radialGradient stop` = **12**, `g[filter]` = **0** (ở 96 px: 20 stop / 3 filter). Trang OAuth cũng dùng bản phẳng (`auth/oauth.go`, thay hàng 4 orb). |
| 2 · Bỏ cỡ 21, chốt 22 px / 20 px | ✓ | `BrandLockup` mặc định `size = 22`; `auth/parts.tsx:126`, `auth/setup/index.tsx:229`, `onboard/index.tsx:629` truyền 22; `AppHeader` 22, điện thoại 20 — không còn `21` trong cây auth/onboard. Đo Login 1440: mark **22,00×30,05**, gap **10/6**, baseline **y=35/35**; thanh app **22**; điện thoại **20,00×27,31**, gap **8**; OAuth **30,2×22** (`TestCallbackPageMarkSize22`, canh `width:30.2px;height:22px`, chặn `height:21px`). Fail-before: `expected '21px' to be '22px'`. Chưa đo trực tiếp Setup/Onboard (cần trạng thái chạy lần đầu) — dùng chung component với Login. |
| 3 · Đổi TUI/CLI | ✓ | `entry/tui/panels.go:142` màn chào **“N O V E R S E”** (giữ kiểu chữ giãn, vừa 60 cột); thanh trên “Noverse vX”; `--version` in “Noverse dev”; câu tự cập nhật “Noverse có bản mới … · chạy ainovel-cli update” (`model_update.go:496`); EPUB `<dc:creator>Noverse</dc:creator>` (`host/exp/epub.go:251`). Giữ mọi lệnh/cờ `ainovel-cli` trong help, `resume.go:23` (nhắc tên công cụ để nâng cấp), đường dẫn, biến môi trường, header. Fail-before: `welcome missing Noverse wordmark: "... A I N O V E L ..."`; `OPF missing "<dc:creator>Noverse</dc:creator>"`. |
| 4 · Tên tệp mã khôi phục | ✓ | `i18n/vi.ts` `adv.recovery.fileName = 'ma-khoi-phuc-noverse.txt'`, `OnboardSteps.tsx` dùng khoá đó (không còn literal trùng); nội dung tệp ghi “Noverse” lấy từ `t.brand.name`; tệp dựng hoàn toàn ở FE. Tệp đã tải trước đây không cần xử lý. Fail-before: `expected 'ma-khoi-phuc-ainovel.txt' to be 'ma-khoi-phuc-noverse.txt'`. |

## M. Gọn kiến trúc đường thẩm duyệt (26/10)

Việc kỹ thuật, không đổi giao diện. Ghi lại vì có đổi hành vi nhìn thấy được:
- **Truyện không xem được giờ trả 404 ở mọi route của truyện.** Trước đây export, chỉ số, Não truyện và thao tác ghi bị từ chối trả 403. Cấp dự án vẫn trả 403. Chi tiết ở `11-api-contract.md`.
- **Bài học nhận lỗi xưng hô từ cả hai nguồn:** lỗi engine đã gộp vào bản thẩm duyệt, và lỗi chỉ có trong store. Mỗi lỗi chỉ đếm một lần. Đúng với `22-brief-2510.md:645,855`. Vũ trụ vẫn chỉ lấy `s19-recap` (`06-data-contract.md:267,270`).
- **Danh sách truyện không còn đọc các vòng thẩm duyệt cũ.** Ở 50 truyện × 100 chương × 5 vòng, thời gian giảm từ 3,69 s xuống 134 ms.
- Mock web giờ phát lại golden do test Go sinh ra (`web/mock/golden/`). Kiểu TS cho các DTO thẩm duyệt, chất lượng, sổ cái, vũ trụ và bài học được sinh từ Go (`web/src/api/types.gen.ts`, chạy `go generate ./internal/entry/web`).
- Hai verdict `revise`/`reject` chỉ còn để hiển thị bản thẩm duyệt cũ (cùng nhãn "Cần sửa" với `polish`, và "Không đạt"). Chúng không đổi được 1-1 sang verdict hiện tại, nên không chuẩn hoá.

**Design cần sửa:** `22-brief-2510.md:240` (bảng D9) vẫn còn mục "Chưa rõ ai nói". Mục này đã bỏ theo dòng 549 và 827.

> **Trả lời của design (26/10):** §M ở `22-brief-2510.md` › *Trả lời dev (§M)*. Bảng D9 đã gạch dòng “Chưa rõ ai nói” và trỏ tới Câu 11. Câu nguồn bài học: chip theo người tạo, câu theo bằng chứng (`06` › Bổ sung §M). <!-- tra-loi-m -->

**Đã áp dụng §M:**
- Bằng chứng của bài học gồm `errors`, `warnings`, `userEdits` và `object`; trường `kind` bỏ khỏi JSON. Dữ liệu cũ dạng `{count, kind}` được quy đổi lúc đọc, tệp không bị ghi lại.
- Chip theo `origin`, đếm lại không đổi chip (test `TestSyncKeepsOriginOnRecount`).
- Câu nguồn ghép các phần khác 0 bằng " và ", theo thứ tự: bạn sửa → Bắt sửa → Cảnh báo. Ví dụ: "Rút từ 3 lần Bắt sửa và 1 Cảnh báo ở 4 truyện · 2 gói". Bài học của một gói (Bộ …) ghi "· gói {tên gói}". Mục dùng chung ghi "· K gói", kể cả khi K = 1, đã được design xác nhận. `sysbrain.test.tsx` đã ghim cả ba trường hợp, không cần sửa code.

> **Design báo lỗi UI (26/10):** 12 mục ở `21-dev-prompt.md` › *Gói O*. Gồm: Bàn viết (nhật ký tiến độ, luồng viết lúc lập kế hoạch), avatar nhân vật, logo. <!-- goi-o-bao -->

**Triển khai Gói O (26/10):** ảnh 1440 lưu ở `/tmp/nv/goi-o/`.

| Mục | Kết quả | Bằng chứng |
|---|---|---|
| O1 · Chip “Khôi phục · tiếp tục từ <Bước>” | ✓ | Bước lấy từ `FoundationMissing`, cùng dữ kiện Flow Router dùng. Test `TestDescribeResumePlanningPhaseVietnamese`: “…từ Lưu thông tin truyện / Lưu tiền đề / Lưu dàn ý / Soát nền truyện”. |
| O2 · Dòng đầu nhật ký bị cắt nửa | ✓ | Mép trên mờ 16 px (StyleX, `activityFeed.stylex.ts`). |
| O3 · “Bắt đầu sáng tác (… — lý do)” | ✓ | Nhãn “Kiến trúc sư lập kế hoạch truyện dài/ngắn”. Lý do kẹp 80 ký tự, toàn văn ở `title`. |
| O4 · Nhiệm vụ giao việc bị cắt cứng | ✓ | Không cắt trong code nữa. Chỗ cắt do CSS (“…”), toàn văn ở `title`. |
| O5 · Dòng con đang chạy lệch, mất “├” | ✓ | Thứ tự `├` → vòng xoay → biểu tượng. Chữ x=400, dòng đã xong x=401. |
| O6 · Tên và màu trợ lý đang chạy | ✓ | `streamAgent.ts`. Lúc viết là “Người viết · Chương 9” (tím), lúc lập dàn ý là Kiến trúc sư (xanh). |
| O7 · Thẻ bước “<Bước> · <Trợ lý>” | ✓ | “Đọc chương · Người viết”, nền và viền theo màu trợ lý. |
| O8 · Vạch tiến độ mảnh | ✓ | Theo `flow`: viết 40 %, viết lại 60 %, chờ duyệt 100 % xanh lá. |
| O9 · Khung gọn lúc lập kế hoạch | — | Gói P thay bằng khung “Kế hoạch truyện”, xem mục Gói P. |
| O10 · Chữ tắt avatar | ✓ | Một hàm dùng chung `lib/cast.ts › charInitials`, test bảng đủ các ví dụ ở `04-components.md:104-109`. Đã xoá 3 bản luật trùng. Lỗi “TD trên dòng Hạ Vy” là do lấy chữ tắt từ chuỗi “Tên (vai)”. |
| O11 · Dòng tên · vai | ✓ | Tên 13/500 và vai 12 px cùng một dòng, “;” đổi thành “ · ”. Áp dụng ở Bàn viết, Dự án & truyện, Não truyện, Não dự án, Audio. |
| O12 · Logo phẳng | ✓ | 22 px trên máy tính, 20 px trên điện thoại và máy tính bảng. Đã xoá `OrbCluster` và `.tablet-orbs`. “Noverse”/“Studio” cùng baseline, cách 6 px. |

**Câu hỏi còn mở cho design:**
- Snapshot vẫn phát chuỗi “Tên (vai)”, phần giao diện tự tách tên và vai. Lý do: `panels_outline.go` của TUI đang khoá sửa.
- Danh xưng “anh/chị/em” chưa được coi là danh xưng, vì có thể trùng tên riêng.
- Lý do kẹp 80 ký tự ở mọi dòng nhật ký, kể cả trang Nhật ký đầy đủ.

> **Design gửi Gói P (26/10):** Bàn viết giai đoạn Lập kế hoạch và lối vào nhanh Não truyện (ngăn kéo, thẻ rê, “Đổi ở chương”). Thay mục O9. Xem `21-dev-prompt.md` › *Gói P*. <!-- goi-p-bao -->

**Triển khai Gói P (27/10):** ảnh lưu ở `/tmp/nv/goi-p/`, báo cáo `/tmp/lm-reports-p.md`.

| Mục | Kết quả | Ghi chú |
|---|---|---|
| P1 · Khung “Kế hoạch truyện” lúc lập kế hoạch | ✓ | `PlanPanel` lấy dữ kiện kế hoạch thật từ máy chủ (`internal/host/plan.go`). |
| P2 · Ngăn kéo Não truyện (phím G, Esc) | ✓ | Dựng lại 1:1 theo `Main.dc.html` › drawer=brain: chip đầu, chú giải 5 mục, nút tròn, mở ở chương đang viết, thanh kéo theo ô, cột chi tiết “ở chương”, ghi chú khoá. |
| P3 · Thẻ rê tên nhân vật (peek) | ✓ | Dùng chung dữ liệu với ngăn kéo. |
| P4 · Thẻ khi không trợ lý nào chạy (chờ / sẵn sàng / xong) | ✓ | Nút nối hành động thật. “Xem kế hoạch” mở Truyện › Dàn ý (không có trang kế hoạch riêng). |
| P4 · Trọng tài được hỏi bạn | ✓ | Cờ `ask_user`, mặc định tắt, 3 cấp (27/10); công tắc ở Tuỳ chỉnh › Cách viết dưới “Duyệt từng chương”. Khi tắt, khối luật bị bỏ khỏi prompt; model vẫn trả `ask` thì máy chủ từ chối và tự chọn phương án bám dàn ý, ghi lý do vào nhật ký. |

**Câu hỏi còn mở (Gói P):** Ngăn kéo vẫn hiện mọi loại cạnh có dữ liệu thật, chú giải giữ danh mục cố định. *(Đã đóng 27/10: `Settings.dc.html` vẽ hàng “Trọng tài hỏi bạn khi phân vân”; schema giữ trường `ask` khi cờ tắt, máy chủ tự chọn phương án mặc định.)*

**Rà lại 34 artboard (27/10):** báo cáo chi tiết `/tmp/lm-reports-audit.md`, ảnh `/tmp/nv/audit/<nhóm>/`.

| Nhóm | Đã sửa chính | Còn ✗ / GAP |
|---|---|---|
| Bàn viết (`Main`) | 22 chỗ: chip đầu trung tính, nút 32 px, “Thẩm duyệt” ở cổng duyệt, “Kế hoạch xong”, thẻ vòng, nhãn rail “Mới nhất”, % ngữ cảnh làm tròn. | Nền chip 0.08 (artboard) hay 0.10 (kit)? Thẻ trợ lý đang chạy: artboard trơn, `04-components` có quầng. |
| Nhật ký, việc nền, lệnh, hộp thoại | Sự kiện `review` có cờ + màu Biên tập viên; trống khi lọc là đoạn chữ; “Bàn viết” thay “Workspace”; bỏ mã `ARBITER`. | Khối trích dẫn dưới dòng nhật ký và đường dẫn từng sự kiện: chưa có dữ liệu. |
| Đọc, dàn ý, chất lượng | Chữ thân `#DADAE0`; màu dòng sửa; nút nguồn luôn hiện; “Não dự án” một dòng ở cả hai màn. | Số đỏ “1” cạnh Não dự án và chip “Móc” ở chương: chưa có dữ liệu. Thanh làm mới 2 px không có trên artboard. |
| Não dự án, não hệ thống | Dòng ngày thêm ở sổ cái (`createdAt`); kết quả tìm kiếm; màu địa điểm; nút chọn gói giãn đủ ngang. | Hậu tố “· đang viết lại” ở sổ cái: chưa có trạng thái. |
| Giọng đọc, xuất audio | Thông tin bên phải thanh các bước; bước 3 tích khi đã xuất; đảo lại “chưa chốt”/“chờ”; chữ số đơn cách. | Nghe thử nhạc mở đầu, dung lượng trống, số đoạn có giọng từ lúc nghe thử, lịch sử nhật ký tạo. |
| Đăng nhập, cài đặt đầu, tạo truyện | Lời phụ đề, bỏ đường dẫn `/start`, “Vào Bàn viết”, chữ Việt thay Architect/Writer. | Tên thư viện trước khi đăng nhập; tên quản trị cho người viết. Nhãn vai giữ tiền tố “tài khoản …” theo quyết định Gói J. |
| Tuỳ chỉnh, Kit | `EmptyState` thêm kiểu khung theo Kit (mặc định kiểu cũ); khoảng cách mục Viết; viền nút chọn. | “Theo máy” vẫn tắt (chưa có bảng màu sáng). |
| Điện thoại, máy tính bảng | Thanh trên Bàn viết điện thoại dùng 3 quả cầu; thẻ “chương sau” ở trình đọc không còn bị chia ba; các khoảng đệm. | Lời chào ở màn chính: artboard đặt ở trên, app dùng thông báo chung ở dưới. |

> **Trả lời của design (27/10):** chip trạng thái 10%; thẻ trợ lý trơn; công tắc “Trọng tài hỏi bạn khi phân vân” ở Tuỳ chỉnh › Cách viết (3 cấp, mặc định tắt); chú giải ngăn kéo chỉ hiện loại đang có; bỏ “Theo máy” ở v1. Xem `22-brief-2510.md` › *Trả lời dev (rà 34 artboard và Gói P, 27/10)*. <!-- tra-loi-2710 -->

> **Design gửi Gói Q (27/10):** chuyển trang mượt: một khung app dùng chung, thanh điều hướng lưới 3 cột đứng yên, chỉ báo trượt, giữ trang cũ tới khi trang mới sẵn sàng, nội dung hiện dần. Xem `21-dev-prompt.md` › *Gói Q* và artboard `Nav.dc.html`. <!-- goi-q-bao -->

### Gói Q — chuyển trang mượt: thanh trên đứng yên, chỉ báo trượt, nội dung hiện dần (27/10)

| Mục | Đã làm | Còn lại |
|---|---|---|
| Khung app | `AppHeader` dựng một lần ở `AppShell` (route gốc); không màn nào tự vẽ thanh trên. Route cập nhật trong `startTransition`. | — |
| Lưới thanh trên | `grid-template-columns: minmax(0,1fr) auto minmax(0,1fr)`, chiều cao tự nhiên theo nội dung và lề; xem nghiệm thu khung dùng chung bên dưới. Cột trái `min-width:0` + cắt “…” nên đường dẫn không đẩy thanh giữa. | — |
| Chỉ báo trượt | Mọi mục đậm 500; một `span` nền duy nhất (`transform` + `width`, 280 ms, `cubic-bezier(.3,.9,.3,1)`), đo `offsetLeft`/`offsetWidth`, đo lại khi resize/font xong. Trượt ngay khi bấm, không chờ dữ liệu. | Artboard vẽ mục chọn đậm 600 trong `renderVals` — chờ design xác nhận 500 (theo luật 3). |
| Nội dung hiện dần | WAAPI: opacity 0→1 + nhích 6 px, 200 ms; không hiệu ứng khi rời trang; giảm chuyển động → mờ 120 ms. | Chưa dùng View Transitions API (phương án dự phòng của spec). |
| Chờ dữ liệu | `shell/RouteStage.tsx`: trang cũ **không bị gỡ** (giữ DOM/state), trang mới dựng trong lớp phủ trong suốt rồi đổi lớp khi có ≥40 ký tự nội dung (hoặc sau 1,2 s); đổi ngay trong `useLayoutEffect` nếu dữ liệu đã có trong bộ đệm. Quá 300 ms → vạch tiến độ 2 px `--grad-progress-nav`. Đo: **0 khung hình trống / 10 lần chuyển** giữa 5 tab đã từng mở. | Đổi lớp sau tối đa 1,2 s kể cả khi màn mới vẫn đang tải (chấp nhận để không treo trang cũ). |
| Nạp trước | `cache.prefetch` + `data/prefetch.ts`, có **ý định rê chuột 80 ms** (bấm ngang qua không bắn request): snapshot, chapters, outline, stories, config dự án; Audio chỉ `cast/settings/ready` — **không** `audio/volume` vì trên máy chủ thật đường đọc này chờ máy đọc TTS nạp mô hình và giữ 1 trong 6 kết nối HTTP/1.1. | Chưa nạp trước chương đang đọc của Trình đọc (số chương do màn tự chọn). |
| Chống xô lệch | `scrollbar-gutter: stable` cho `.scroll`; nhớ vị trí cuộn từng route; trạng thái panel vốn đã ở `shell/stores`. | Font đã tự host qua `@fontsource` nên không thêm `<link rel=preload>`. |
| Vòng focus | Chỉ hiện với bàn phím: `data-input="pointer"` khi bấm chuột → CSS bỏ vòng kể cả khi trình duyệt tự coi là `:focus-visible`; Tab vẫn có vòng 2 px accent. | — |
| Hiệu năng | `formatClock` dựng một `Intl.DateTimeFormat` + nhớ kết quả (116–138 ms → 0); `translateHost` nhớ kết quả (43–50 ms → ~0); `POST /api/events/watch` gộp 120 ms (4 → 1 mỗi lần chuyển trang). | Đo lại trên máy rảnh: số “click → có nội dung” của Bàn viết chưa chốt. |

**Triển khai Gói Q (27/10):** ảnh và số đo ở `/tmp/nv/goi-q/` (`nav-*.png`, `nav-geometry-after.json`, `before-timings.json`, `after-timings-fast.json`), báo cáo `/tmp/lm-reports-bug2710.md` › *GoiQ*. Đo trên mock 1440: nav = x 468,36 · rộng 503,28 ở **cả 5 mục** (0 px xê dịch), chỉ báo lệch tâm mục ≤ 0,49 px; bấm chuột không còn vòng focus, Tab vẫn có. `formatClock`/`translateHost` có test chặn hồi quy (`screens/workspace/format.test.ts`, fail-before đã chứng minh).

> **Design gửi Gói R (27/10):** chọn dự án/truyện ngay ở thanh “Áp dụng cho” của Tuỳ chỉnh (nút ▾, danh sách có tìm, hộp “chưa lưu”, dải “Đang xem…”). Xem `21-dev-prompt.md` › *Gói R*. <!-- goi-r-bao -->

### Gói R — chọn dự án/truyện ngay ở thanh “Áp dụng cho” (28/10)

| Mục | Đã làm | Còn lại |
|---|---|---|
| Thanh phạm vi | Chỉ hiện ở 5 mục có 3 cấp (Quy tắc viết, Cách viết, Trợ lý AI, Ngân sách, Thông tin); ô Truyện/Dự án hai nút có ▾; “Mọi truyện” không ▾; “Truyện này” chỉ khi đúng truyện đang mở. | Artboard chưa có tweak `pick`/`target` để vẽ trạng thái mở danh sách. |
| Danh sách | 360 px, có tìm, `N truyện · M quy tắc riêng` / `theo “Mọi truyện”` / `theo dự án`, chip “Đang mở”, ✓, “Dự án này chưa có truyện.”, cuối danh sách “Quay về truyện đang mở · …”; bàn phím như Kit Select. | — |
| Chưa lưu | Hộp cam “Có N thay đổi chưa lưu cho … Lưu trước khi chuyển?” với Lưu rồi chuyển / Bỏ thay đổi / Ở lại, hiện trong danh sách khi đổi chỗ xem. | — |
| “Đang xem…” | Dải xanh dưới thanh khi xem dự án khác hoặc truyện khác truyện đang mở, kèm nút quay về; truyện đang mở ở Bàn viết không đổi. | Câu “quy tắc” dùng chung cho cả 5 mục — cần copy riêng nếu muốn. |
| Đường dẫn | `#/settings/<mục>?project=&story=` (+ `scope=global`) giữ qua tải lại và khi chuyển mục. | — |
| Tên truyện | Thanh đọc tên sách máy chủ trả (`storyLabel`); `storySummary` ưu tiên `meta/book.json` trước `story.json.title` nên hết hiện dòng brief. | — |
| Điện thoại | Dòng “Áp dụng cho · Truyện này/Dự án/Mọi truyện” + bottom sheet 86 % dùng lại `ScopeList`/`UnsavedBox`: chip “Đang mở”, ✓, tìm, hộp cam, “Quay về truyện đang mở” giống máy tính. | — |

**Triển khai Gói R (28/10):** ảnh `/tmp/nv/goi-r/`, báo cáo `/tmp/lm-reports-bug2710.md` › *GoiR*. Chọn dự án ở ▾ rồi thêm quy tắc: mock ghi đúng `quy-tac.md` của dự án đang xem; tải lại giữ `#/settings/<mục>?project=&story=`; truyện đang mở ở Bàn viết không đổi.

> **Design gửi Gói S (28/10):** avatar nhân vật phân biệt theo loại vai (Chính · Phản diện · Đồng minh · Phụ, viền nét đứt khi chưa lộ), dữ liệu `kind`/`revealAt`. Xem `21-dev-prompt.md` › *Gói S*. <!-- goi-s-bao -->

## Gói S — Avatar nhân vật phân biệt theo loại vai (28/10 · P1) <!-- goi-s -->

**Trạng thái: xong. Huy hiệu góc khớp số đo artboard ở mọi cỡ (đo `getBoundingClientRect` trên mock + iframe preview); ảnh nghiệm thu 1440 hoãn vì trình chụp lỗi — nghiệm thu bằng số đo.**

- BE: `domain.Character` mang `kind` (`lead|antagonist|ally|support`) + `revealAt` (`*int`). `store.CharacterKindOf` suy kind từ `role` khi thiếu; `CharacterStore.Load` điền trong bộ nhớ, `Save` ghi xuống (đúng luật không ghi khi đọc). `save_foundation` type `characters` từ chối kind ngoài 4 giá trị; prompt Kiến trúc sư (long/short) đã yêu cầu điền `kind`/`revealAt`. `PlanCharacter.Kind` và `naoBrainEntity.kind/revealAt/ini` đã emit.
- `PUT S/characters`: sửa `kind`/`revealAt` theo `id`/`name`, `revealAt: null` xoá bí mật, kiểm cả lô rồi mới ghi; quyền ghi dùng chung `storyAccessAllowed` (người chỉ đọc nhận 404). Mock `web/mock/server.ts` có cùng hợp đồng.
- `universeEntity.Kind` (enum TS `CharacterKind` sinh bằng `go generate`); `audio.Speaker.Kind` điền từ hồ sơ nhân vật mỗi lần đọc dàn giọng.
- Golden `web/mock/golden/universe.json` đã cập nhật (thêm kind/ini/revealAt vào entity). Test xanh: `internal/entry/web/...`, `internal/store`, `internal/tools`, `internal/host`.
- Test: `internal/store/characters_kind_test.go`, `internal/tools/save_foundation_kind_test.go`, `internal/entry/web/characters_put_test.go` (lưu, lô nguyên tử, 400/404, reader 404), `internal/audio/cast_test.go › TestEnsureCastRefreshesSpeakerKind`, `web/src/ui/CharAvatar.test.tsx` (aria “{tên}, {loại}[, chưa lộ]”, nét đứt, huy hiệu >20 px, `sortByKind`).
- FE: token `kind` trong `tokens.stylex.ts`; `ui/CharAvatar.tsx` (`CharAvatar` sm20/md26/lg40/số px, `hidden`, `dim` bằng filter trên nền đặc, `KindChip`, `KindLegend` có “Chưa lộ”); `lib/charKind.ts` (thứ tự Chính→Phản diện→Đồng minh→Phụ, nhãn, aria, `latestWritten` cho “chương đang xem” ở Bàn viết). Thay toàn bộ call site (StoryTab đọc `getCharacters`, PlanPanel, LiveStream, CastTab, PersonPage 52 px + `KindEditor` “Loại vai”/“Lộ vai ở chương” lưu qua PUT, StoryGraph, DetailColumn 40 px + chip, MBrain/MobileReader, audio Studio/panels/ConfirmDialog/SegmentRow, projbrain); xoá `AVATARS`/`AVATAR_BY_NAME`/`avatarFor` và bảng `SPEAKER_RING`.
- Mock: `brain.ts` 6 nhân vật mẫu có `kind`/`revealAt`/`ini` (PC `revealAt: 5` → nét đứt ở chương đã nộp thứ 3); `sampleCharacters` cùng dữ liệu.
- Huy hiệu góc: `badgeGeometry(px)` trong `ui/CharAvatar.tsx` suy cỡ/vị trí từ cỡ avatar — <40 px → badge 13 px lệch 3 px, ≥40 px → 16 px lệch 2 px (tính từ hộp đệm, viền 1,5 px), `dx` tâm huy hiệu→tâm avatar dùng chung DOM và SVG nút đồ thị; icon căn giữa tâm badge, `overflow: visible` nên không bị cắt. Đo trên mock khớp artboard `Main.dc.html`: 26→13/+2, dx 8,5; 40→16/+1, dx 13; 46 (nút chọn) → 16/+1, dx 16.
- Test: `web/src/ui/CharAvatar.test.tsx` thêm ca “huy hiệu theo cỡ avatar” + “mép ngoài không bị cắt”; `web/src/screens/project/cast/KindEditor.test.tsx` (4 ca: nút Lưu, payload `kind`/`revealAt`, xoá bí mật → null, chương 0 báo lỗi không lưu).
- Còn: ảnh 1440 (Bàn viết › Truyện + drawer có nút chọn) ở `/tmp/nv/goi-s/` — hoãn vì trình chụp màn hình headless lỗi; migration ghi sẵn `kind` lúc nâng cấp dự án (hiện ghi ở lần Save kế tiếp).

> **Design gửi Gói T (28/10):** nút lên đầu cho chỗ đọc chương và danh sách dài (vòng tiến độ ở chỗ đọc, điện thoại ẩn cùng thanh). Artboard `ScrollTop` (10c). Xem `21-dev-prompt.md` › *Gói T*. <!-- goi-t-bao -->

> **Design gửi Gói U (28/10):** ngăn kéo Não truyện ở Bàn viết dùng đúng component đồ thị, dòng thời gian và bảng chi tiết của màn Não truyện; danh sách Nhân vật ở tab Truyện đọc cùng nguồn `…/brain`. Xem `21-dev-prompt.md` › *Gói U*. <!-- goi-u-bao -->

## Gói T — Nút lên đầu cho chương và danh sách dài (28/10 · P1) <!-- goi-t -->

- `web/src/ui/ScrollTopButton.tsx`: component dùng chung và `useScrollTop(ref)`; chỉ gắn nút/listener khi nội dung ≥3 màn hình, ngưỡng hiện >1 màn / ẩn <½ màn, listener passive gộp rAF; đo lại bằng ResizeObserver và khi nội dung đổi. Vòng tiến độ 2 px, icon `arrow-up-2`, 40/44 px, kính mờ/bóng/focus và nhịp hiện 180 ms / ẩn 120 ms lấy từ artboard; điện thoại đổi đáy 380 ms. Màu mới chỉ nằm trong token.
- Cuộn về đầu dùng smooth (auto khi giảm chuyển động), nhảy trước về cách đầu một màn nếu xa hơn sáu màn, giới hạn đoạn cuộn bằng timer 400 ms; focus tiêu đề với `preventScroll`, không có tiêu đề thì focus vùng cuộn. Nút ẩn có `tabIndex=-1` ngay và `visibility:hidden`; sheet, menu, bàn phím và chọn văn bản đều làm ẩn nút.
- Đã gắn: `ReaderBody`, `MobileReader`, `StoryView`, `ProjectOverview`, danh sách đoạn `Transcript`, trang `ActivityScreen`. Điện thoại ẩn cùng thanh, đáy 100 px khi không có nút duyệt / 166 px khi có nút duyệt; không gắn ở luồng Bàn viết hoặc nhật ký điện thoại. Đổi chương reset cuộn trước khi vẽ và nút dựng lại ở trạng thái ẩn.
- Đo bằng DOM/CSS trên mock 1440×900 và 390×844, đối chiếu iframe artboard ScrollTop/Reader/Outline/Transcript/MReader: Reader 40 px, phải 20 / đáy 16; Outline 40 px, phải 48 / đáy 92; Audio phải 20 / đáy 16 trên hàng Chốt; Nhật ký phải 24 / đáy 24; mobile 44 px, phải 16, vòng tiến độ khớp thanh trên (15%). Focus tím 2 px, cách 3 px; giảm chuyển động `transition:none`, về đầu ngay.
- Kiểm tra: 9 ca `ui/ScrollTopButton.test.tsx` xanh (đã chạy mutation fail-before), TypeScript xanh. Ratchet màu không tăng; lần chạy chung còn báo baseline của tệp ngoài Gói T `screens/activity/mobile/index.tsx` giảm 10→9, integrator cần chốt lại.
- GAP dữ liệu kiểm thử: chương có sẵn ở mock chỉ khoảng 233 chữ, chưa có chương 3.000 chữ. Đã đo nội dung dài bằng fixture DOM tạm 60 đoạn ×50 từ, lặp chính văn có nguồn ở mock, không ghi dữ liệu truyện hay thay mock server. Bằng chứng chi tiết và độ trễ timer khi máy bận ở `/tmp/lm-reports-bug2710.md` › `ScrollTopT`.


## Gói U — Ngăn kéo dùng chung Não truyện (28/10 · P1) <!-- goi-u -->

- `StoryGraph` là bộ vẽ duy nhất cho màn Não truyện và ngăn kéo (`compact`); xoá hoàn toàn bộ vẽ, bố cục ego, nhãn và ô chương riêng của ngăn kéo. Hai nơi dùng chung `ChapterRail`, `DetailColumn`, `edgesAt`/`relsAt` và `addressRowsAt`. SVG có id riêng theo instance để đồ thị giữ trong Activity không chiếm gradient của ngăn kéo.
- Ngăn kéo rộng `min(1180px,84vw)`, chi tiết 320 px, avatar 40 px; có mô tả, quan hệ có vạch màu, xưng hô có chiều và lịch sử đổi, tối đa 5 diễn biến, chương sau mờ. Compact không có lọc/tìm/danh sách/phóng to; chú giải vai ở dưới trái, gợi ý thao tác ở dưới phải. Mở toàn màn đóng ngăn kéo.
- Tab Truyện chuyển danh sách nhân vật từ endpoint hồ sơ sang thực thể `…/brain`; thẻ rê chọn đúng dòng xưng hô **đi từ** nhân vật đang rê, không lấy dòng gọi nhân vật đó. Thẻ “Đổi ở chương N” dùng cùng bộ lọc cạnh: quan hệ chạm nhân vật, Đổi trước Mới, tối đa 3 dòng.
- Chương 4 có 12 thực thể/14 quan hệ: Phùng Chấn chưa lộ (nét đứt), chỉ có “Ép xoá bản live” tới Tri Hạ; cạnh tới Nhược Vy bắt đầu ở chương 5 (“Chống lưng” theo nguồn Não truyện), không dùng dữ liệu tĩnh cũ “Đồng phạm · chưa lộ” của Main.
- Đo mock 1440×900: cả hai lựa chọn Nhược Vy/Phùng Chấn có cùng chính xác hình học thẻ/cạnh/nhãn và màu giữa full/compact. Thẻ Nhược Vy 164×46, Phùng Chấn 172×46, Tri Hạ 172×54 trong viewBox 860×690; nhãn khớp Brain trong 0,15 đơn vị. Đồ thị đông dùng cùng bố cục tránh chồng chữ; mock 36 thực thể/60 cạnh có 0 chồng thẻ, 0 chồng nhãn, 0 chữ tràn; chi tiết clientWidth=scrollWidth=320.
- Đo font bằng canvas thay cho ghi/đo SVG từng ký tự; tìm nhị phân khi cắt chữ, giữ tên đầy đủ trong title. Không thêm mã màu. 34 ca/5 suite và `tsc --noEmit -p .` xanh; fail-before được ghi trong `/tmp/lm-reports-bug2710.md` › `GraphU`, kèm bảng đo và 3 ảnh nghiệm thu 1440.
- GAP API: Brain chưa mang xưng hô; giữ **một** nguồn chung `…/address-terms` + `addressRowsAt` cho cả hai nơi, không mở rộng backend trong gói này theo quyết định integrator. Artboard Main đã được làm mới để dùng cùng hình học compact của Gói U.

> **Design gửi Gói V (28/10):** dòng thời gian chương ở Phòng thu cho tập dài: chương chưa tạo dùng thời lượng ước tính, ba chế độ hiển thị theo bề rộng (đầy đủ · chỉ số · dải màu có thước), thẻ khi rê, phím ← →. Xem `21-dev-prompt.md` › *Gói V*. <!-- goi-v-bao -->

> **Design gửi Gói W (28/10):** Tuỳ chỉnh › Trợ lý AI: ô chọn model cắt “…” đúng cách, thêm ô Ngữ cảnh theo model (tự nhận / tự đặt / cảnh báo mức dự phòng). Xem `21-dev-prompt.md` › *Gói W*. <!-- goi-w-bao -->

## Gói V — Dòng thời gian chương ở Phòng thu cho tập dài

- `internal/audio/timeline.go` và `GET …/audio/queue?volume=N`: giữ danh sách việc, thêm `chapters[{chapter,title,state,sec,estSec,pct}]`, `madeSec`, `estTotalSec`. Ước tính tái dùng `Script.EstimatedDuration()`; chưa có bản đọc dùng chính văn, thiếu chữ dùng trung bình chương đã tạo rồi trung bình ước tính đã biết; không có dữ liệu thì chia đều. Giây đã tạo cộng các đoạn thật và khoảng lặng, không lấy thời lượng ước tính làm số thật.
- `ChapterTimeline.tsx` dùng StyleX/token, trọng số thời lượng thật hoặc ước tính; `ResizeObserver` chọn đầy đủ/chỉ số/dải màu tại 72/28 px. Dải màu có thước 5/10/25, mốc đầu/cuối, nhãn chương đang tạo và chú giải; thẻ rê/focus, một điểm Tab, ← → Home End và Enter phát ở mini player. Tổng ghi “đã tạo … / khoảng … · a/b chương”, xong hết bỏ phần ước tính. Hàng đợi được làm mới riêng khi có thêm đoạn đã tạo, không tải lại toàn màn.
- Mock nhận `POST …/audio/demo {scenario,len:8|16|40|120,doneChapters?:N}`. Đã đo cả bốn độ dài ở 1440: thanh rộng 1018 px, chiều cao/cách lần lượt 40/4, 40/4, 28/2, 28/1; bốn ô đầu khớp artboard tới 1/64 px. Trường hợp 16 chương chỉ xong chương 1: rộng 53,30–71,33 px, không số bị cắt. Ảnh: `/tmp/AudioTimelineV-{8,16,40,120}.png`; fail-before và kiểm tra trong `/tmp/lm-reports-bug2710.md` › `AudioTimelineV`. Ratchet Studio giảm 29→24, không thêm màu cứng.
- **GAP thiết kế / nghiệm thu 1024 px:** giữ định tuyến máy tính bảng theo quyết định Main. `18-mobile.md:310,318–319` quy định 641–1099 px dùng bố cục Nghe điện thoại, cột giữa 560 px; `MTablet.dc.html:312` nói cùng điều đó. Vì vậy nghiệm thu chuyển chế độ thanh Phòng thu tại viewport 1024 là **N/A theo thiết kế**, không tự đổi `tablet/index.tsx:76`. Đã đo 1024: cột 560 px, không tràn ngang; đo riêng co container trên desktop từ 1018→586 px: đầy đủ→chỉ số tự động, không tràn.
- **GAP dữ liệu mock:** ngoài số tiêu đề có sẵn, chương dài chỉ mang nhãn “Chương N”, không đặt tên truyện/chương giả; thời lượng các chương mở rộng lấy đúng công thức fixture của artboard.

## Gói W — Ô chọn model và ngữ cảnh của Trợ lý AI

- `KitSelect` giữ nhãn đầy đủ trong `title`, cắt “…” trong vùng `min-width:0`; mũi tên không co. Đã rà mọi caller; chế độ model thêm mô tả phụ, chip ngữ cảnh và lối sang Kết nối AI. Các danh sách tự mở lên khi thiếu chỗ phía dưới.
- Tuỳ chỉnh › Trợ lý AI đổi thành khối hai dòng: orb/tên/việc/phạm vi, rồi Model · Mức suy nghĩ · Ngữ cảnh (176 px). Popover phân biệt tự nhận/bạn đặt/dự phòng, viền cảnh báo dùng token, có Tự nhận/Tự đặt, kiểm tra 8–2.000 nghìn, preset và ngưỡng tự tóm tắt; người không phải quản trị chỉ xem, không có Lưu. Lưu theo model ở cấp toàn cục, mọi hàng dùng model đó cập nhật cùng nhau; Tự nhận xoá trường riêng.
- Danh mục `/api/config/models` và `/model` trả `contextWindow`, `contextSource`, `registryWindow` từ cùng `ResolveContextWindow`; metadata ngưỡng và giá trị tự nhận lấy từ cùng quy tắc dự trữ. PUT cấu hình dùng lại store, chặn ngoài 8.000–2.000.000 và chặn người không phải quản trị sửa/xoá ngữ cảnh model. Ô model trong Nâng cao dùng cùng giá trị đã lưu.
- Sau khi lưu, gateway gửi cập nhật ngữ cảnh tới worker đang tồn tại; host và ModelSet chỉ cập nhật metadata, giữ lựa chọn/credential và không khởi động lại engine. Cấu hình được tách map/slice trước khi công bố dưới các khoá riêng; nhật ký dùng cửa sổ mới và lượt tạo bộ quản lý ngữ cảnh kế tiếp dùng resolver đã cập nhật.
- **Bổ sung W2:** cả Tuỳ chỉnh, bảng `/model` và Thêm › Model trợ lý dùng cùng cache theo `(provider, model)` và cùng PUT `context_window` toàn cục. `/model` giữ bản nháp theo model, cập nhật chip ngay và báo “Ngữ cảnh N model có thay đổi chưa lưu”; chỉ ghi khi Lưu thay đổi. Điện thoại ghi ngay lựa chọn/giá trị hợp lệ; Tự nhận xoá override. Cả ba cùng làm mới sau mỗi lần lưu, chỉ quản trị được sửa.
- Dùng chung `formatContextK`: `400K`, `256K`, `1M`, giữ phần lẻ theo đơn vị nghìn. Đã chuyển các nhãn ở danh sách model, khởi tạo, Nâng cao và phòng trưng bày; giá trị danh mục thật vẫn lấy API, số liệu mẫu chỉ thêm vào mock catalog.
- **Đã đóng GAP Settings:** `Settings.dc.html` hiện có đủ `ai=none/model/ctx/custom/fallback`. Đã đối chiếu lại bố cục hai dòng: card 860 px, hàng đầu 118,25 px/hàng còn lại 131,25 px, cột Ngữ cảnh 176 px; model menu tối thiểu 380 px; popover 360 px, custom cao 435,84 px, input 96×32 px, preset cao 26 px. Giữ nội dung/mô tả từ API, không chép lời đánh giá mẫu vào danh mục thật.
- Đo `/model` ở 1440: hộp 900×600 px, nhãn 10,5 px, nhóm ngữ cảnh nằm giữa Model và Mức suy luận. Đo điện thoại 390×844: nhóm 358 px, hàng chọn tối thiểu 54 px, ô số 96×46 px dùng kích thước trường nhập sẵn có, chip `overflow-x:auto; flex-wrap:nowrap`, không tràn ngang. Đặt 256K trong `/model` thấy “256K · bạn đặt” ở Tuỳ chỉnh và điện thoại; Tự nhận trên điện thoại xoá trường riêng qua API. Đã phục hồi override mock sau kiểm tra.
- **GAP nguồn mô tả model:** catalog/registry hiện không mang mô tả thật; `displayName` không phải mô tả. Theo quyết định Main, model thiếu mô tả chỉ hiện tên; renderer hỗ trợ cắt “…” khi catalog có trường mô tả, không tự gán lời đánh giá từ dữ liệu mẫu.
- Kiểm tra W2: **113 ca/8 suite FE**, `tsc --noEmit -p .`, `go build ./...`, `go vet ./internal/entry/web ./internal/host ./internal/bootstrap` xanh. Năm regression formatter/pending/mobile/cache/admin đều đỏ trước bản sửa; ratchet ModelPanel giảm 41→39 màu cứng. Báo cáo đầy đủ: `/tmp/lm-reports-bug2710.md` › `ModelCtxW2`; bằng chứng backend W ban đầu giữ ở `SettingsModelsW`.

## MobileJank — Giới hạn tái kết xuất và làm mới nền

- Cache giữ tham chiếu của nhánh dữ liệu không đổi; `useResource` không làm panel kết xuất lại chỉ vì `refreshing`/timestamp. `useResourceValue` và job selectors chỉ báo thay đổi lát dữ liệu đang dùng. Pane ẩn tiếp tục dùng `Activity`; sheet Việc nền đóng ngừng subscribe nhưng giữ nội dung khi trượt ra.
- Draft/prefill/đo textarea nằm trong `WorkspaceComposer`, không nằm trên stream/list. Reader cập nhật progress bằng transform trong scroll handler, tìm paragraph bằng binary search và lưu vị trí có debounce/flush. Runtime action context/callbacks ổn định, cờ scroll y hệt không đặt state mới.
- Dùng lại vạch 2 px của §16: `PanelRefreshBar` tự subscribe `refreshing` theo các key thuộc panel, overlay absolute không chiếm chỗ, trễ 300 ms/giữ tối thiểu 400 ms, chỉ animate transform. Mobile/tablet/desktop dùng cùng component; không thêm indicator thiết kế mới hay giảm blur của tabs.
- Watcher production chỉ hash bytes của chapters/outline/reviews khi metadata đổi để bỏ invalidation chỉ do mtime; mock snapshot phát parts theo revision, không phát lại mọi part ở mỗi tick.
- Kiểm chứng: TypeScript và full Vitest **169 suite / 1421 ca** xanh; Go watcher/worker regression xanh. Có fail-before cho composer, cache/Activity, runtime no-op, job selectors/closed sheet và metadata-only invalidation.
- **Chưa nghiệm thu hiệu năng native:** checkpoint idle có SSE giảm commit Đọc khoảng 13,40→0,03/s, pane ẩn 0; tuy nhiên lượt CDP ×4 vẫn đo longtask tối đa 1005 ms. Chưa có đủ sample đổi tab 10 lần, long chapter scroll và sheets trước/sau do browser-tool timeout; không gắn số checkpoint trước job-selector cutover cho source cuối. Toàn bộ profiler/observer/rAF probe đã xoá. Chi tiết và inventory 191 hooks: `/tmp/lm-reports-bug2710.md` › `MobileJank`.

> **Design gửi Gói X (28/10):** 10 lỗi ở Nhật ký hoạt động trên điện thoại (hàng chương lệch, câu sự kiện lộ tham số công cụ, xếp nhóm sai chương, tiêu đề mờ…). Xem `21-dev-prompt.md` › *Gói X*. <!-- goi-x-bao -->

## Gói X — Nhật ký hoạt động trên điện thoại

- **X1/X2:** hàng chương cao 52 px, biểu tượng sách 15 px tại lề trái 16 px; tên `flex:1; min-width:0`, cắt “…”, mũi tên 16 px sát lề phải 16 px. Thu gọn chỉ xuống, mở chỉ lên và `aria-expanded` đi cùng trạng thái.
- **X3/X4:** câu sự kiện bỏ phần chỉ dẫn gọi công cụ/`key=value`; mỗi công cụ có thẻ mở riêng, tham số chỉ xuất hiện khi mở với nhãn tiếng Việt. Khoảng chương dùng “–”. Bộ định dạng dùng chung cũng bỏ số chương vắng, ≤0 hoặc ≥2147483647; `novel_context` tổng quan không hiện “Chương 0”.
- **X5/X6:** API trả chapter riêng cùng scope/volume/arc/chapterFrom/chapterTo. Sự kiện cũ khôi phục đích từ args/result và công việc đã ghi; cung dùng khoảng đã ghi hoặc outline lưu trên đĩa, không đoán ở client. Nhóm theo đích thật, nhóm cung riêng, thứ tự theo sự kiện mới nhất; lượt viết lại chương40 không bị đưa vào cursor chương41. Tên thiếu hiện “chưa đặt tên”.
- **X7–X10:** tiêu đề không filter/transform; blur14 px ở lớp nền riêng. Dòng điểm ghi rõ “Biên tập 8,0 · 24 sự kiện” theo điểm/số sự kiện thực. Header sticky nền đặc, border1 px, fade12 px; dải chip có fade24 px, chip được chọn tự cuộn giữa.
- **Nguồn kiểm chứng:** fixture rút gọn8 sự kiện thật 27/10,13:10–13:26 từ `~/NoverseLive/tron-audio/kich_ban/01-viet-kich-ban-dan-y/output/novel/logs/tui.log` và tên chương38–40 trong `meta/chapter_records/`; không sửa nguồn. Mock `#/p/dit-dang-audio/s/nhat-ky-cung/activity`, cùng truyện dài40 chương. Bản ghi lịch sử mang cung2/tập1 là chỉ dẫn cũ bị đảo tọa độ; không sửa ngược lịch sử thành outline hiện tại.
- **Kiểm chứng:** đo cả10 mục với MLog artboard ở390×844; DOM fixture thật8 sự kiện/5 thẻ công cụ và mock dài32 sự kiện/20 thẻ đã mở không có `=` hay “Chương0”; chip đo tâm195,39 px so với giữa màn195 px. TypeScript,141 ca/9 suite FE, Go activity/StoryReader/log regression cùng build/vet hai package host/web xanh. Có fail-before cho nhóm, mũi tên/ARIA, khoảng chương, chương chưa tên, dòng điểm, số chương mặc định và API đích/cung.
- **GAP dữ liệu, không bịa:** nguồn tại lúc chụp chỉ có nội dung/tên tới chương40;41 mới là cursor, không phải công việc đang diễn ra. Fixture rút gọn không có điểm80 và đủ24 sự kiện của một chương; chuỗi ví dụ X8 được kiểm tra bằng fixture kiểm thử có điều khiển, không gán điểm/số lượng đó cho dữ liệu thật. Báo cáo phép đo và lệnh kiểm tra: `/tmp/lm-reports-bug2710.md` › `MobileLogX`.

> **Design gửi bổ sung Gói W và Gói Y (28/10):** ô Ngữ cảnh có thêm ở bảng `/model` và trên điện thoại (cùng số liệu); thẻ trợ lý trên màn Viết điện thoại có hiệu ứng đang chạy / chờ bạn / rảnh. Xem `21-dev-prompt.md` › *Gói W* (bổ sung) và *Gói Y*. <!-- goi-y-bao -->

> **Design gửi Gói Z (28/10):** điện thoại › Nghe có sheet quản lý tạo giọng (chạy / tạm dừng / lỗi, hàng đợi, chất lượng, tự tạo khi chốt) và *Xuất thử chương N* trong Ghép & xuất. Xem `21-dev-prompt.md` › *Gói Z*. <!-- goi-z-bao -->

## Gói Y — Thẻ trợ lý đang chạy trên màn Viết điện thoại

- Giữ thẻ cao 52 px, viền màu trợ lý 35 % và nền ánh từ trái; thẻ chờ/rảnh giữ viền xám. Chỉ trợ lý đang chạy có mặt cầu thở/nổi và quầng 3,2 giây, điểm sáng trắng lắc 6 giây; không sửa thẻ Bàn viết máy tính.
- Chữ hành động dùng màu trợ lý pha 28 % trắng và vệt sáng 2,2 giây, nền rộng 250 %. Keyframe riêng đi từ 100 % tới `calc(-200% / 3)`, đúng một chu kỳ ảnh; đổi bước giữ phần tử và Animation, không khởi động lại.
- Giảm chuyển động: cả hàng có 0 Animation, quầng giữ 50 %, chữ bỏ nền sáng và giữ màu pha. Nhãn đọc màn hình có tên, hành động và “đang chạy”.
- Kiểm chứng: 16 ca trong hai suite mobile xanh; TypeScript xanh. Đo Chromium 390×844 cho running/gate/idle, hai trợ lý chạy song song, giảm chuyển động và Animation liên tục khi đổi bước. Ratchet của stylesheet giảm 105→104; kiểm tra toàn cục hiện chỉ vướng phần giảm màu chưa chốt của ModelPanel ngoài phạm vi này. Bằng chứng và ảnh: `/tmp/lm-reports-bug2710.md` › `AgentCardY`.

> **Design gửi Gói AA (28/10):** xuất lẻ audio từng chương trên máy tính (Xuất gì: Cả tập / Chọn chương, danh sách tệp) và điện thoại (⋯ ở dòng chương, Cả tập / Từng chương, .zip). Xem `21-dev-prompt.md` › *Gói AA*. <!-- goi-aa-bao -->

## Gói U — Đối chiếu artboard làm mới

- Compact dùng lề đồ thị 12 px, chú giải có nền/viền và gợi ý riêng của ngăn kéo; tên chương không làm hàng tiêu đề timeline xuống dòng. Chi tiết dùng avatar 40 px, gap 12 px, tên 16 px; bỏ badge loại nhân vật và nhãn chiều dư so với Main, giữ badge cho thực thể không phải nhân vật. Chú thích khoá được cuộn cùng chi tiết, có viền/nền token, không chiếm một footer cố định ngoài bản thiết kế.
- Đo bản Main không dùng HTTP cache tại 1440×900: khung1180×900 ở(260,0), header70,844 px, chi tiết320 px; đồ thị834×733,156 ở(273,82,844), rail858×84 ở(261,816), khớp app. Rail compact dùng padding8×22×4 thay16px dọc của bản toàn màn. Bảng đo đủ bảy trạng thái và điều tra browser: `/tmp/lm-reports-bug2710.md` › `ArtboardRecheck`.

## Gói W — Hai trạng thái ngữ cảnh đã có artboard

- Đã bỏ GAP “CmdResults thiếu modelCtx / MMore thiếu pickCtx”. Bảng `/model` dùng segment 30 px, input 76×30 px và preset 26 px; điện thoại dùng nhóm lựa chọn không đệm lồng, input 120×46 px, chip 34 px và dải cuộn ngang. Câu ngưỡng giữ số thật từ API và nói rõ phạm vi áp dụng; không chép 224K của mẫu vào giá trị thật. Settings vẫn giữ input popover 96×32 px và cột Ngữ cảnh 176 px.
- Thêm hồi quy catalog không bị tải lại theo mỗi render của bảng model khi cache đã cũ. Mutation thêm fetcher không ổn định vào dependency effect làm ca mới đỏ (2 request thay vì 1); bỏ mutation trả về xanh. Không coi mutation là nguyên nhân đã chứng minh của hiện tượng treo browser trước đây.
- Đo mobile pickCtx358×110,797 px so với358×110,906 của mẫu; input120×46, chip34, document390 không tràn. Bảng `/model` khớp hộp900×600, segment588×38 và input76×30. 33 ca/5 suite xanh; kiểm TypeScript phần được giao sạch, lượt tích hợp còn lỗi kiểu trong GenerationSheets của gói audio đã chuyển Main. Picker không tái hiện vòng render trong các lượt đo rAF/mutation; cùng lỗi worker treo cũng xảy ra ở artboard tĩnh.

## Gói Y — Đối chiếu lại ba trạng thái Viết

- Giữ nguyên bản 3: thẻ 52 px, orb 28 px, khoảng cách 8 px, chữ 11,5 px; orb chuyển động, shimmer 2,2 giây/nền 250 % liền mạch và reduced-motion đứng yên không bị thay bằng hiệu ứng mới. Đã đo running/gate/idle tại 390×844. Công cụ `arbitrate` nay hiện nhãn tiếng Việt “đang phán định”, không lộ tên kỹ thuật trên thẻ Trọng tài.

## Gói Z — Tạo giọng, chọn chương và bản xuất thử trên điện thoại

- Thẻ tổng kết mở bảng Tạo giọng; hai lối vào Tạo giọng / Hàng đợi, chip từng chương, Thêm chương và Chốt & tạo dùng cùng hàng đợi thật. Có trạng thái chạy / tạm dừng / lỗi, thử lại với chất lượng nhanh, ưu tiên chương chờ, chất lượng cho việc mới và tự tạo khi chốt. Bảng Giọng đọc mở từ cả hai bảng và quay về đúng nơi mở, giữ lựa chọn chương.
- Bảng chọn chương loại chương đang tạo; chọn chương chưa có giọng, bật tạo lại chương đã có, hiển thị số đoạn / người nói / cảnh báo từ bản đọc. Lệnh hàng loạt kiểm tra toàn bộ lựa chọn trước khi chốt; chương 11–12 được lưu trạng thái locked và trả về danh sách chương, warnings, etaSec.
- GET hàng đợi trả tiến độ đoạn, chất lượng, lý do lỗi, thời điểm lỗi; hỗ trợ pause / resume / retry / rerender / prioritize. Chất lượng được chụp theo việc, đổi tuỳ chọn không xoá phần giọng đã tạo. Resume tiếp tục dùng tệp đoạn đã xong; lỗi đọc không lộ đường dẫn hay thông báo kỹ thuật.
- Xuất thử gửi đúng một chương với trial=true, dùng tên tap-1-ban-thu-chuong-N theo định dạng đã chọn; không thay manifest hay tệp xuất cả tập. Mock cũng giữ nguyên cả bản xuất và yêu cầu xuất cả tập đang đợi. Có nghe bản thử rồi xuất cả tập.
- Kiểm chứng: 13 ca FE trong ba suite; toàn bộ hai package Go internal/audio và internal/entry/web; go vet của hai package đều xanh. Có fail-before cho lọc chương chọn, chi tiết lỗi hàng đợi, chốt chương, resume/cache, tên/manifest bản thử và mock không thay yêu cầu xuất đang đợi. Phép đo MAudio 390×844 và chi tiết kiểm tra bổ sung được ghi dưới AudioQueueZ trong /tmp/lm-reports-bug2710.md.
- Không bịa thời gian: dự báo theo chất lượng chỉ hiện khi có số đo công việc tương ứng; chưa có mẫu thì ghi “Chưa ước tính”. Fixture artboard có đủ ba chất lượng để kiểm tra bố cục.

## ListenSyncFix — Đồng bộ tiếng và trình nghe

- Dòng chạy theo cộng thời lượng giọng thật với khoảng lặng sau từng đoạn, gồm khoảng ngừng lời thoại của chế độ một giọng, cùng quy tắc ghép chương của backend. Không lấy số từ thay cho thời lượng đã tạo.
- Trình phát lấy mẫu `HTMLAudioElement.currentTime` mỗi khung hình; không tích luỹ thời gian bằng timer. Đứng tiếng và đổi tốc độ giữ đúng đồng hồ media; dừng vòng lấy mẫu khi tạm dừng, lỗi, đổi nguồn hoặc gỡ màn.
- Nhãn giờ không làm tròn lên giây chưa nghe; sóng đã phát, kim, mốc đánh dấu và thao tác tua dùng cùng thời lượng media khi đã tải metadata. Không đổi bố cục hay hộp xuất.
- Hồi quy kiểm tra khoảng lặng qua mốc 30 giây, lời thoại một giọng, phần làm tròn và 1.800 khung hình có đứng tiếng/đổi tốc độ. Bằng chứng đỏ trước sửa, phép đo browser và lệnh kiểm tra được ghi dưới `ListenSyncFix` trong `/tmp/lm-reports-bug2710.md`.

## Gói Z — Đổi giọng và ghi âm mẫu trên điện thoại

- Ba bảng Giọng đọc / Giọng cho vai / Mẫu giọng mới dùng cùng tài nguyên phân vai và giọng dự án với máy tính. Mỗi lần chỉ phát một mẫu; giọng vừa tải lên được chọn sẵn, không tự đổi phân vai trước khi bấm dùng. Lựa chọn chế độ riêng của truyện thắng mặc định gói ở cả hai giao diện.
- Đổi giọng từng vai có lựa chọn đọc lại các đoạn đã có giọng, giữ âm thanh cũ của vai khác và xếp sau việc đang chạy. Số đoạn tính từ bản đọc đã chốt trong mọi tập; thời gian chỉ ước tính khi máy đọc có số đo RTF.
- MediaRecorder dừng tự động ở 30 giây, báo mẫu ngắn / đủ dài và không dùng mẫu dưới 10 giây. Tệp ghi trình duyệt được chuyển từ tín hiệu thật thành WAV; máy chủ kiểm tra thời lượng 10–30 giây và phần mở rộng mp3 / wav / m4a, lưu vào Tài sản › Giọng, cung cấp mẫu nghe tối đa 5 giây. Không tự gán lỗi nhiễu khi chưa có số đo.
- Đã đo bằng DOM tại 390×844: Giọng đọc 390×816 ở y=28, bảng chọn giọng 390×800 ở y=44; bảng ghi âm 390×639,421875 ở y=204,578125, nút ghi 96 px, hai nút thao tác 174×50 px — trùng phép đo artboard cho các thành phần này. Chi tiết hồi quy và thử ghi / tải mẫu ở mục AudioVoiceZ3 trong báo cáo.

## Gói AA — Xuất lẻ audio trên máy tính

- Hàng chương có nút xuất nhỏ hiện khi rê chuột / lấy nét; chương đang nghe luôn hiện. Bấm mở Chọn chương với đúng chương đó, không kéo theo lựa chọn cũ. Chân trình nghe có Xuất từng chương cạnh Ghép & xuất; không thay lối vào trên điện thoại.
- Hai giao diện dùng lựa chọn chương có giọng, không buộc liền nhau, tính thời lượng đã chọn; hỗ trợ tệp riêng / ZIP, danh sách kết quả và nghe / tải từng chương. Bản xuất cả tập mở rộng danh sách tệp chương thay vì giấu trong một liên kết thư mục.
- Đã đo năm trạng thái AudioExport ở 1440×900 và bốn trạng thái MAudio ở 390×844; số đo DOM lưu tại `/tmp/ExportPrefsAB-measurements.json`. Hộp máy tính 820×754 tại (310,130); bảng từng chương điện thoại cao 674,25 px so với mẫu 674,75 px; kết quả ba chương cùng cao 460 px. Khác tên / thời lượng / số giọng phản ánh dữ liệu thật, không chép số mẫu.

## Gói AB — Tuỳ chọn xuất chung, lưu theo truyện

- Dùng một mô hình, hook tải / lưu và hàm thuần cho cả hai biểu mẫu: MP3 mặc định, M4B, WAV; nhạc mở/kết, đọc tên chương, cân âm lượng, khoảng lặng, mốc chương, phụ đề, cách nhận tệp. Xuất lẻ ẩn nhạc / khoảng lặng / mốc chương; mốc chỉ hiện cho cả tập M4B; cách nhận chỉ hiện khi xuất lẻ. Giữ sở thích khi ẩn nhưng không gửi tuỳ chọn trái phạm vi sang bộ xuất.
- GET/PUT `S/audio/export-prefs` lưu JSON riêng từng truyện; biểu mẫu đọc lại khi mở, lưu trước khi xuất. Đã thao tác M4B + phụ đề trên máy tính, xuất rồi mở điện thoại và xác nhận cả hai được nạp sẵn. Bản xuất thử của Gói Z giữ đường dẫn và manifest riêng.
- Backend thực thi nhạc / tên chương / loudness / khoảng lặng, đúng một khoảng giữa hai chương; mốc M4B, phụ đề SRT của cả tập hoặc cục bộ từng chương, định dạng từng tệp, ZIP thật chứa đúng tệp đã chọn. Kết quả trả thông tin chương, thời lượng, kích thước, URL tải và URL ZIP. Mock hỗ trợ cùng sở thích / quy tắc và ZIP hợp lệ.
- Hồi quy đỏ trước sửa: quy tắc ẩn thiếu hàm, API sở thích trả 404, mutation bỏ chuẩn hoá xuất lẻ cho thời lượng 11 giây thay vì 1 giây. Kiểm cuối: 10 ca FE / 4 suite, Go prefs round-trip + áp dụng tuỳ chọn + bảo toàn bản thử, `go vet` hai package audio/web và `tsc` đều xanh. Không chạy build hay git. Chi tiết tại mục ExportPrefsAB trong `/tmp/lm-reports-bug2710.md`.
- Ranh giới định dạng: xuất cả tập / một phần ghép vẫn giữ thư mục chương phụ MP3 như trước; MP3/M4B/WAV được chọn cho thành phẩm ghép. Chỉ `split:true` áp định dạng đã chọn cho từng chương. Không đổi thẻ tác giả Noverse hay cách nhúng mốc M4B; quy tắc này giữ đúng việc tìm thành phẩm chính và thử lại khi `onExists:"fail"`.

## Gói AC — Đưa quy tắc viết về đúng cấp

- `RulesEditor` dùng chung cho tab **Quy tắc viết** của truyện, mục **Quy tắc viết chung của dự án** và **Tuỳ chỉnh › Quy tắc chung**. Tab truyện có số quy tắc riêng + mục học từ góp ý; dự án chỉ đếm quy tắc riêng. Nhóm kế thừa chỉ xem, kể cả lựa chọn làm rõ; liên kết sửa mở đúng trang dự án hoặc Tuỳ chỉnh.
- Tuỳ chỉnh chỉ sửa **Mọi truyện**, bỏ thanh **Áp dụng cho** riêng ở mục này; các mục còn dùng bộ chọn cấp vẫn giữ nguyên. Thẻ hướng dẫn quay về tab quy tắc của truyện đang mở. Cột phải truyện có thẻ kiểm quy tắc và lối tắt quy tắc không còn trỏ về bộ chọn cấp cũ.
- M4 mở trực tiếp quy tắc truyện, bỏ nút chia ba cấp. Chạm nhóm kế thừa để sửa cấp dự án / Mọi truyện, có nút **Về quy tắc của truyện**. Tab truyện được giữ bằng `Activity`, tránh mất bản nháp khi đổi tab.
- API giữ nguyên. Trình duyệt đã lưu quy tắc riêng bằng `PUT /api/rules/file?scope=story…` + đường áp dụng hiện có, xác nhận không sang truyện thứ hai; quy tắc dự án được xác nhận xuất hiện ở cả hai truyện. Đã xoá các quy tắc thử sau kiểm.
- Đã đo Outline `tab=rules` ở `list`, `understood`, `thinking`, `unsure` và `view=project`; Settings rules ở `list`, `understood`, `adding`, `thinking`, `retry`, `unsure`, `dirty` tại 1440; MMore `rules`, `rulesAdd`, `rulesThinking` tại 390. Bảng đo: `/tmp/rules-ac-measurements.json`; diễn giải chênh lệch theo nội dung và safe-area ở báo cáo RulesTabAC.
- Hồi quy đỏ trước sửa: mục rules còn hiện bộ chọn cấp; hook cho sửa nhóm kế thừa; bộ đếm global không cập nhật khi giữ ngữ cảnh truyện. Sau sửa: 167 ca / 15 suite, kiểm ratchet màu và `tsc --noEmit -p .` xanh. Không chạy build hay git.
- **GAP cần design chốt:** artboard/spec ghi 12 từ, engine có 16. Theo quyết định tích hợp, giữ đủ 16 từ thật cả danh sách lẫn tổng; không đổi backend chỉ để khớp hình.

## GenPickEmpty — Chương chưa có bản đọc vẫn vào hàng đợi

- **Quyết định tích hợp của chủ dự án:** chọn “Tự soạn rồi tạo”, không buộc quay về máy tính. Chương đã nghiệm thu nhưng chưa có bản đọc vẫn chọn được, có nhãn **AI sẽ soạn bản đọc**. Hàng đợi dùng mô hình của truyện để soạn, chốt bản đọc vừa soạn rồi tạo giọng theo thứ tự; lỗi thiếu mô hình hoặc soạn thất bại có lý do tiếng Việt và dùng **Thử lại** của hàng đợi hiện có.
- Không ghi số đoạn, người nói hay thời gian giả cho bản đọc chưa tồn tại. Chương đã có bản đọc giữ số đoạn / người nói / chỗ cần xem thật. ETA chỉ dùng số đo tạo giọng của đúng chất lượng, không lấy thời lượng nghe làm thời gian tạo; chưa có mẫu thì giải thích **Chưa đo tốc độ tạo ở chất lượng này**, bản đọc chưa soạn có thêm **Chưa đo thời gian soạn bản đọc**.
- Chủ sở hữu thực thi và hồi quy: `internal/audio/mobile_queue.go`, `mobile_queue_test.go`, `render.go`; bảng chọn và avatar dùng `web/src/screens/audio/mobile/GenerationSheets.tsx`, fixture `mobileUnprepared` trong `web/mock/audio.ts`. Bằng chứng dữ liệu thật, đỏ trước sửa và kiểm sau sửa ở `/tmp/lm-reports-bug2710.md` › `GenPickEmpty`.
- **GAP cần design:** `MAudio` chưa có artboard cho nhãn **AI sẽ soạn bản đọc** và bước **Đang soạn bản đọc**. Đây là luồng đã được chủ dự án duyệt để không chặn chương chưa có bản đọc; không sửa ngược `18-mobile.md`, `21-dev-prompt.md` hoặc artboard để coi như thiết kế đã có.


## Gói AG — Đổi truyện một cú bấm và mục lục thật

- **Đổi dự án chỉ ở thanh trên** (`ContextCrumb` variant `switcher`): popover `ProjectPopover` với listbox, số truyện/đang viết, **Dự án mới**, context-menu Mở/Mở thư mục/Đổi tên; bỏ mục “Đổi thư mục thư viện” (chỉ còn trong Tuỳ chỉnh › Nâng cao). Bộ chọn dự án trong cột trái đã xoá cùng code chết.
- **Đổi truyện một cú bấm** (F2): thẻ truyện đầu cột trái mở `StoryPopover` — Tổng quan dự án · các truyện · Truyện mới trong dự án này. Cùng popover mở từ nút **Tên truyện ▾** ở crumb thanh trên. Không đóng tập đang mở khi đổi truyện (màn mới tải trạng thái riêng).
- Cột trái: view truyện = **MỤC LỤC · n ch.** (tìm không dấu + theo số, Tới chương đang viết, Thu gọn mọi tập, đầu tập dính, một tập mở một lúc, tập khoá `disabled` “mở khi Tập N-1 xong”, nhớ tập theo truyện ở `localStorage` `ainovel.toc.<p>.<s>`). View dự án = menu Tổng quan/Ý tưởng/Tư liệu/Quy tắc/Não dự án + **TRUYỆN · n** gọn.
- **Lazy >200 chương (§AG §3):** `GET …/outline` thêm `?volume=N` — chỉ kèm `chapters` của tập N; `?volume=0` chỉ header (arcs giữ số chương để đếm/range). Sidebar truyện >200 chỉ tải chương tập đang mở; tích luỹ trong phiên. `arcs[].estimatedChapters` lộ số chương ước lượng của hồi khung. Không kèm tham số = đầy đủ như cũ.
- Bấm chương trong mục lục mở `?ch=N` trên tab Đại cương (`id="och-N"` cuộn tới, mở hàng). Crumb variant `link` giữ như cũ; `switcher` không còn đoạn “/ Tập 1” giả.
- Mock: `/outline` chia tập 40 chương khớp `volumes`, hỗ trợ `?volume=`; thêm truyện `08-dai-240-chuong` (240 chương, 6 tập + tập 7 khung khoá).
- Kiểm: `web/src/screens/project/toc.test.ts` 11 ca (tìm không dấu + số, một tập mở, tập khoá không chọn được, localStorage theo truyện, lazy >200) và `TestStoryReaderOutline_VolumeFilter` (Go). `tsc --noEmit` web xanh; `go build ./internal/host ./internal/entry/web` xanh; ratchet màu đã chốt lại sau khi token hoá mã màu mới.
- **GAP cần design chốt:** artboard len 120 hiện “Tập 3 · 40 ch. · chưa dàn ý” nhưng tải 120 chương có tên — chưa có dữ liệu cho “tập chưa dàn ý có range”; hiện “chưa dàn ý” chỉ khi tập có chương nhưng toàn chương trống tên. Hàng “Tập N+1” giả sau tập cuối đã bung: artboard luôn khoá ở len 8/40 nhưng code cũ cho “viết tiếp” khi tập trước xong — giữ hành vi cũ (mở được = `reopen`).
- **Chưa hoàn tất ở lượt này:** đo trực quan nav × len tại 1440 trên trình duyệt thật (mock 5200 chạy nhưng cần xác nhận màn hình); kiểm fail-before từng test bằng mutation chỉ còn ghi nhận trên module logic (các test viết trước/tồn tại trước tên export mới).

## Gói AH — Kết nối AI: chọn model của từng nhà cung cấp

- **Backend:** `POST /api/config/models/list` (`internal/entry/web/config_model_list.go`) lấy danh sách từ API của nhà cung cấp theo bản nháp: OpenAI-compatible `GET {base}/models` (gồm OpenCode Go, DeepSeek, Qwen, GLM, xAI…), OpenRouter (ngữ cảnh + giá USD/1M), Anthropic (phân trang, `x-api-key`), Gemini (`generateContent`, `inputTokenLimit`), Ollama `/api/tags`; gói thuê bao dùng danh sách OAuth sẵn có. Cache 10 phút theo tên + loại + base URL + băm khoá, `refresh` bỏ cache, lỗi không cache. Lỗi trả trong thân 200: `key_rejected` / `unreachable` / `missing_key` / `not_logged_in` kèm câu tiếng Việt có tên nhà cung cấp; Bedrock `unsupported`. `POST /api/config/test` khi ok thêm `contextWindow` + `contextSource`. `PUT /api/config` (global) trả 409 `model_in_use` kèm nhãn trợ lý khi bỏ model đã lưu mà Mặc định / vai / dự phòng còn dùng; không ghi tệp.
- **FE:** thẻ nhà cung cấp có *Model · n* / *Chọn model* (viền vàng khi 0 model) mở `ModelPicker` trong thẻ: tìm không dấu theo từng từ (“glm 5.1” khớp `glm-5.1`), lọc Đã chọn / Gợi ý / Tất cả, ngữ cảnh, giá, nhãn trợ lý, khoá bỏ chọn model đang dùng, ảo hoá > 50 hàng, *Thêm theo tên* qua `/api/config/test`, trạng thái đang lấy / lỗi / không có API danh sách. Thêm bằng khoá: ô khoá mở sẵn, dán khoá xong bảng tự mở ở *Gợi ý* (đổi khoá chờ ngừng gõ 600 ms mới gọi lại); Ollama mở ngay; gói thuê bao mở khi đăng nhập xong. Trợ lý AI › Model nhóm theo nhà cung cấp, nhóm rỗng có *Chưa chọn model · Chọn* mở đúng bảng. Nâng cao › Tinh chỉnh model bỏ *Thêm model*.
- **Lỗi bản thật sửa kèm:** `configForSave` từng bỏ nhà cung cấp chỉ còn khoá (server giấu khoá → entry rỗng bị tỉa) → thẻ biến mất sau khi tải lại và lần Lưu toàn cục kế tiếp xoá luôn khoá. Nay giữ entry `{}` để server gắn lại khoá. Ô nhập khoá của thẻ vừa thêm bị đóng ngay lúc mount (effect theo `ctx.response`), nay chỉ đóng khi máy chủ trả cấu hình mới.
- **Kiểm:** Go `go test ./internal/entry/web/ ./internal/bootstrap/` xanh (httptest cho từng họ API, cache/refresh, khoá nháp, 403, 409). Web `tsc --noEmit` + toàn bộ vitest xanh; `modelPick.test.ts`, `config.test.ts` (nhà cung cấp chỉ còn khoá). Máy chủ thật cô lập + OpenCode Go thật: 38 model, cache giữ `fetchedAt`, `refresh` lấy mới, khoá hết hạn → `key_rejected`, thiếu khoá → `missing_key`, Bedrock `unsupported`, Ollama tắt → `unreachable`, 409 `Đang dùng cho Người viết…` và tệp không đổi. Trình duyệt 1440: thêm OpenCode Go → dán khoá → bảng mở ở Gợi ý (10 model), tìm, chọn, khoá Mặc định, Thêm theo tên (tên sai → lỗi HTTP 400 bằng lời; tên đúng → báo dùng 200K dự phòng), Lưu, Trợ lý AI nhóm DeepSeek rỗng → *Chọn* mở đúng bảng, khoá DeepSeek còn nguyên sau Lưu.
- **GAP:** OpenAI-compatible không trả ngữ cảnh nên model ngoài danh mục Noverse hiện 200K (`contextSource: default`) cho tới khi quản trị đặt tay; giá chỉ có ở OpenRouter. Màn điện thoại (`more/mobile/Models.tsx`) chưa có bảng chọn model — chưa có artboard.

## Gói AI — Neo góp ý theo trích dẫn + ngữ cảnh, vòng đời đầy đủ

- **Mô hình:** `Comment` thêm `status` (`pending|sent|fixed|lost|unsure|done`), `sentAt/fixedAt/resolvedAt`, `fixedBy` (`agent|quote_gone`), `unfixed`, `moved`, `relinked`, `whole`, `newQuote`, `candidates` (≤3) và `anchor{paragraph,quote,prefix,suffix}` — `paragraph`/`quote` gốc giữ bất biến. Tệp cũ (`paragraph`+`quote`, `appliedAt`) migrate khi đọc: `appliedAt`→`sent`+`sentAt`, không mất dữ liệu, bỏ `appliedAt` khi ghi lại.
- **Gắn lại sau mọi lần ghi chương** (draft/append/edit/commit cả lẫn replay, nhận bản sửa tay `/sync`, nhập truyện): khớp nguyên văn duy nhất → dời; gần đúng duy nhất (Dice trigram ≥0.6) → dời + `moved`; nhiều → `unsure`+`candidates`; không → `lost`. `lost`/`unsure` không tự ẩn. `sent` sau ghi: câu còn → `pending`+`unfixed`; trích mất + đoạn thay → `fixed(quote_gone)`+`newQuote`. Bộ tách đoạn Go (`comments.Paragraphs`) khớp `parseChapterText` của FE (chỉ đếm khối paragraph), normalize giống `textContent+\s+→" "`.
- **Compose gửi id góp ý**; writer trả `fixed_comments` trên `commit_chapter` (đã ghi trong schema + `writer.md`) → `fixed(agent)`. Duyệt lần kế của truyện → `done`+`resolvedAt` (`ResolveAllFixed` sau `/next` thành công). Batches lũy đẳng (chỉ `pending` gửi, chỉ `sent` nhận fixed).
- **PATCH** `…/chapters/:n/comments/:id` `{action: resolve|reopen|relink|resend_whole, paragraph?, quote?}` trả comment cập nhật; lỗi là câu tiếng Việt.
- **SSE** `comments.reanchored` `{chapter,fixed,unfixed,need}`: đường ghi xếp vào `.ainovel/gop-y/_events.json`, gateway rút khi thấy `commit_chapter|draft_chapter|edit_chapter` hoàn tất hay job `sync|import|importsim` xong; `story.changed` kèm part `comments`. `GET …/chapters` thêm `comments{need,pending}` mỗi chương.
- **GAP:** đường đọc (`GET`, `Add`, `Apply`) chỉ giải anchor khi đọc được chính văn (nháp nếu có, không thì cuối); `sent` giữ nguyên trạng thái ở đường đọc — chuyển `fixed`/`pending+unfixed` chỉ xảy ra trên lần ghi chương kế tiếp. `unsure`/`lost` tự hồi về `pending` nếu trích xuất hiện lại ở ghi/đọc sau (theo thiết kế, không phải cờ `relinked`).

## Gói AK · đo lại MVoiceGen/AudioExport (AKMeasure)

- Đã sửa lỗi hai `className` (React bỏ một bên) và thêm test chặn `web/src/styles/duplicateClassName.test.ts`; token hoá toàn bộ mã màu Gói AK (ratchet màu chỉ giảm).
- Đã vá 5 Δ đo được ở lượt 1 (màu nhãn phụ #8E8E99, nền thẻ "Đang tạo" acc-16, ô chất lượng #2A2A2E, nền chân ngăn #18181A, mất nút chính/câu tóm tắt khi mở ngăn bằng deep-link do effect chọn sẵn chỉ chạy theo `[open]`).
- Lượt đo lại (AKMeasure2) đã đóng GAP 1–4: bảng Δ 7 trạng thái `gk` @390 và 3 trạng thái `upd` @1440 nằm ở `/tmp/lm-reports-bug2710.md` › `AKMeasure2`; xem thêm §"Gói AK · đo lại lần 2" bên dưới.

## Gói AK · đo lại lần 2 (AKMeasure2)

- **Δ = 0** cho 7 trạng thái `gk` @390×844 (entry · stale · ready · redo · running · error · allDone) và 3 trạng thái `upd` @1440 (`none` · `stale` · `queued`), trừ hai GAP ghi dưới đây.
- Đã sửa: mock `genRunning/genError` bị tiến job sau khi seed — `activateAudioScenario` gán `state.timer = null` trước khi `clearInterval`, làm interval của `ensureTimer()` mồ côi và tiếp tục tick; giờ chỉ `clearInterval` rồi mới gán `null`.
- Đã sửa: `genRedo` chọn chương đã có giọng (ch3) như artboard, và `VoiceGenSheet` gộp chương `initialRedo` vào ô chọn.
- Đã sửa: thanh `upd` (padding 10 12, bo 12, nền warn-7, viền warn-30, chữ `--kit-warn-bar`, nhịp 12, thêm icon `refresh-2` 16) + nút 32/0 14/bo 9/warn-on/12,5 + nhánh queued acc-6/acc-30/`--kit-acc-bar`; ô "đã sửa" nền warn-9/viền warn-50/chữ warn-soft, meta `--color-fg-4`; ô "chờ cập nhật" nền trong suốt/viền acc-50 **nét đứt**/chữ badge-run, meta `--color-fg-4`; câu queued dùng đúng chữ artboard "sau chương đang tạo".
- Đã sửa: `ChapterTimeline` đo bề rộng dải ngay khi mount (ResizeObserver có thể không chạy ở tab ẩn → trước đây rơi nhầm chế độ "bar"); thẻ trang vào 8px (artboard 0 + scroll 8) và thêm nhãn "Chương" đúng `.ghead` của MAudio (18 30 8 · 12,5 · #7E7E8A · .02em).
- **GAP 5 (dữ liệu, không sửa):** số phút ước lượng lệch artboard. `AudioExport.dc.html` dùng công thức mẫu 0,6 phút/đoạn (5 đoạn → "khoảng 3 phút", 2 đoạn → "1 phút"); app lấy ETA thật từ `timeline`/`qualityEtaSec` nên 5 đoạn hiện "khoảng 2 phút". Giữ số thật theo dữ liệu, không chép công thức mẫu.
- **GAP 6 (hai artboard khác nhau):** danh sách chương sau thẻ ở màn nghe. `MVoiceGen.dc.html` vẽ bản rút gọn chỉ đọc (`.vr` 54px, icon 28, không nút phát); app giữ danh sách phát được của `MAudio.dc.html`/`18-mobile.md` (`.grow` 60px 8 14, nút phát, thời lượng). Không đổi theo MVoiceGen vì sẽ phá màn nghe. Hệ quả: thẻ đầu trang cao 145px so với 162,7px của artboard do app cắt "…" dòng trạng thái theo quy tắc không tràn chữ (artboard để xuống dòng).

## Gói AL · Cách viết theo tầng (29/10/2026) — `Outline.dc.html › tab=style` + `pstyle` + `Settings.dc.html › section=writing`

Đã làm (web/mock + web/src):

- **Tab “Cách viết” của truyện** (`?tab=style`, sau “Quy tắc viết”, trước “Bản xuất”): thanh kế thừa `.st-inh` (+`.own` khi truyện chọn riêng, nút “Dùng theo dự án”), lưới Gói thể loại + Kiểu viết dùng chung với Tuỳ chỉnh (`StylePicker`), thẻ rút gọn “Gói … gồm” (`PackSummaryCard`), khối “Văn phong của bạn · văn mẫu của truyện này” (`StyleSamples` chuyển từ Tuỳ chỉnh sang `web/src/screens/project/tabs/`, tiêu đề + nút “Thêm văn mẫu” + ngăn nhập đúng artboard). Chọn một ô lưu ngay `config.style` cấp truyện (PUT `/api/config?scope=story`, If-Match = version) + toast “Đã đổi văn phong của truyện · áp dụng từ chương N” (N = `effectiveFromChapter` như Quy tắc viết); “Dùng theo dự án” PUT tệp không còn `style` + toast “Truyện dùng lại văn phong của dự án · từ chương N”. “Văn phong của bạn” chỉ chọn được khi truyện đã có văn mẫu; khi chưa có, ô mờ kèm câu “Thêm văn mẫu ở bên dưới để dùng”.
- **Mục “Cách viết mặc định của dự án”** trên trang dự án (`#pstyle`, sau Quy tắc viết dự án) + mục “Cách viết” trong cột trái: hàng văn phong đang dùng (“Riêng dự án này” / “Theo “Mọi truyện”…”), danh sách truyện với chip tím = truyện chọn riêng (đọc `sources.style==='story'` của từng truyện qua `GET /api/config?scope=story` — không có API gộp, gọi từng truyện), nút “Đổi” mở ngăn cùng lưới chọn; chọn ô lưu ngay cấp dự án + toast “Đã đổi văn phong mặc định của dự án · truyện mới sẽ dùng”.
- **Tuỳ chỉnh › Cách viết**: cấp “Mọi truyện” giữ “Văn phong chung · cho truyện mới và truyện chưa chọn riêng” (ô “Văn phong của bạn” mờ, ghi chú văn mẫu thêm theo truyện ở Dự án & truyện); cấp truyện/dự án thay lưới + văn mẫu bằng thẻ chỉ dẫn (icon, tên văn phong + nguồn, nút “Mở Cách viết của truyện/dự án”). “Duyệt từng chương” và “Trọng tài hỏi bạn khi phân vân” giữ nguyên 3 cấp.
- **CSS**: tách các lớp `.st-*` của Cách viết (sec · srow · stx · field · uchip · style · grph · pbadge · lyr · pchip · drawer) + mới `.st-inh` (thanh kế thừa) và `.st-smp` (hàng danh sách) sang `web/src/screens/settings/style.css`; `settings.css` giữ phần riêng Tuỳ chỉnh, `index.tsx` import thêm `style.css`.
- **Liên kết áp dụng**: hàng “Văn phong” trong thẻ “Đang áp dụng” của truyện trỏ `?tab=style`, của dự án trỏ `?section=pstyle` (trước đây đều mở Tuỳ chỉnh).
- **Mock**: gieo 2 văn mẫu cho Trà Xanh (`styleSamples` map — vốn bắt đầu rỗng nên không thể đo trạng thái `samples` của artboard).
- **API**: không đổi — vẫn `config.style` 3 cấp và `…/stories/{s}/style-samples`; số “truyện nào chọn riêng” suy từ `sources.style`, không cần API mới.
- **Kiểm chứng (browser, 1440, mock :5200):**
  | Trạng thái | Đo | Kết quả |
  | --- | --- | --- |
  | wstyle=pack | `.st-inh.own` (12 14 · bo 14 · acc-35 · acc-5), ô gói `.on` (acc-55 · acc-7), pbadge 20px/rgba(166,237,193), grph 12px #A0A0AB hoa .04em, thẻ “Gói … gồm” 3 dòng + ghi chú + link Chất lượng | Δ = 0 |
  | wstyle=builtin | PUT story romance → thanh “Ngôn tình · Riêng truyện này”, ô Ngôn tình `.on` | Δ = 0 |
  | wstyle=inherit | bấm “Dùng theo dự án” → PUT bỏ style, `.st-inh` thường + “Theo dự án “Kênh truyện audio””, ô của cấp kế thừa vẫn `.on` | Δ = 0 |
  | wstyle=samples | danh sách `.st-smp` “1.840 chữ · 26/10”, nút Thêm văn mẫu → ngăn `.st-drawer`, field 34px/textarea 104px, “Lưu văn mẫu”; thẻ tự tạo chọn được | Δ = 0 |
  | wstyle=proj | `#pstyle`: hàng “Ngôn tình · Riêng dự án này”, chip truyện riêng tím acc-12/acc-soft, “Theo dự án” xám; Đổi → ngăn lưới, ô tự tạo mờ | Δ = 0 |
  | Settings scope | `Mọi truyện` = Văn phong chung + ô tự tạo mờ (opacity .55); `Truyện`/`Dự án` = thẻ chỉ dẫn, 0 ô radio; Duyệt/Trọng tài còn nguyên | Δ = 0 |
- **GAP 1 (nhãn đơn vị, không sửa):** hàng văn mẫu artboard ghi “{n} ký tự” nhưng `StyleSample.chars` là số **từ** (mock POST đếm `text.split(whitespace).length`). App hiện “{n} chữ” — đúng ngữ nghĩa dữ liệu + glossary (văn bản đếm theo từ); không đổi API.
- **GAP 2 (dữ liệu artboard):** mục “Cách viết mặc định” của artboard minh hoạ `pw='default'` (Theo “Mọi truyện”) nhưng mock đã seed `CONFIG_PROJECT.style='romance'` (Riêng dự án này) — app hiện theo dữ liệu thật; cả hai nhánh code đều có.
- **GAP 3 (không có trong API):** artboard vẽ nút “Hoàn tác” trong toast xoá văn mẫu, nhưng `GET style-samples` không trả `text` nên không thể khôi phục sau khi xoá; app gửi toast “Đã xoá văn mẫu” không nút. Đổi API mới làm được — giữ nguyên theo ràng buộc.
- **GAP 4 (gấp đôi phát hiện khi đo):** `#/settings` giữ main cũ mount ẩn (`display:none`/w=0) khi đổi cấp → DOM chứa cả hai trạng thái; đo đúng thì phải chọn `main` có width>0. Không phải lỗi hiển thị.

## GAP HostedLibrary — onboarding Docker / Postgres (31/10/2026)

- Thiết kế §10 của `09-auth-onboard.md` chỉ có thư viện thư mục; chưa có biến thể hosted. Biến thể tối thiểu giữ bước **Nơi lưu truyện**, thay bộ chọn bằng thẻ “Truyện được lưu trong cơ sở dữ liệu của máy chủ” và nút **Tiếp tục** hiện có. Không gọi API duyệt thư mục, không ghi cấu hình thư viện file vốn không được Postgres hỗ trợ. Dự án hosted chỉ hỏi tên/mô tả, không hỏi thư mục. Tuỳ chỉnh không hiện thao tác đổi/mở thư mục hoặc sửa tên thư mục trên Postgres.
- `GET /api/library` thêm duy nhất `storageMode: "file" | "postgres" | "container"`: `file` giữ bộ chọn hiện có, `postgres` dùng cơ sở dữ liệu + BlobStore, `container` dùng volume dữ liệu do image quản lý. Trình duyệt cũ/máy chủ chưa có trường này vẫn dùng luồng file.
- Docker file có `LIBRARY_BROWSE_ROOTS` là biến thể file, không phải `container`: các Nơi là đúng các bind mount đã khai báo, gốc đầu tiên là mặc định khi chưa lưu lựa chọn; chặn đường dẫn ngoài gốc, đoạn `..`, symlink thoát gốc ở duyệt/kiểm tra/đổi gốc. Không có allowlist thì image dùng volume `/var/lib/noverse/library` và thẻ thông tin, không yêu cầu chọn đường dẫn trong container. Cài native không thay đổi.

- Nhật ký hosted đọc `Store.Runtime.LoadQueue()` từ `story_events` stream `runtime_queue`, nơi observer lưu payload sự kiện thật. Giữ cùng chuẩn hoá văn bản/ẩn đường dẫn, lọc, đếm, quyền và phân trang chụp mốc theo sequence; không trả 501 hoặc giả danh sách rỗng. Nhật ký file vẫn đọc `tui.log` như trước.
- Thẻ kiểm tra cấu trúc thư mục dự án chỉ thuộc file mode. Hosted không gọi `/validate` và không hiện nút “Kiểm tra lại”/“Sửa tự động” cho filesystem vốn không tồn tại; đây không phải phép kiểm tra nội dung truyện.

- Bảo toàn dữ liệu container: image đặt `NOVERSE_CONTAINER=1`; bộ duyệt không đưa HOME vào Nơi, kể cả không có `LIBRARY_BROWSE_ROOTS`. Chỉ cho đổi sang root thuộc allowlist hoặc volume mặc định. `storageWarning` ở bootstrap/kiểm tra thư mục và log máy chủ cảnh báo “Thư mục này sẽ mất khi cập nhật container — hãy gắn volume.” nếu `/proc/self/mountinfo` không xác nhận mount bền (không tính overlay/tmpfs). Các compose mẫu khai báo mount thư viện rõ ràng.

## Gói AM · Cách viết gộp & chọn nhanh (31/10/2026)

- **Truyện:** chỉ còn tab **Cách viết**; `?tab=rules` được thay bằng `?tab=style`. Thẻ Văn phong gọn, Đổi/Xong mở/đóng lưới trong thẻ, hàng Theo dự án xoá ghi đè; văn mẫu thu gọn theo mặc định. Quy tắc nối ngay bên dưới, không còn số đếm quy tắc trong nội dung tab; chỉ giá trị kế thừa có nhãn nguồn. Thẻ kế thừa dùng tên riêng `.rl-inherited`, viền nét đứt, chỉ xem và có liên kết sửa đúng cấp.
- **Dự án:** một mục **Cách viết của dự án**, chứa Văn phong và Quy tắc; một lối tắt Cách viết bên trái. Thẻ Văn phong chỉ liệt kê truyện chọn văn phong khác giá trị dự án; trạng thái rỗng ghi mọi truyện đang dùng văn phong này.
- **Đang áp dụng:** Văn phong và Người viết có bảng chọn tại chỗ; Duyệt từng chương dùng switch. Văn phong/Người viết ghi riêng cấp truyện, lựa chọn kế thừa xoá khoá ghi đè; switch dự án ghi riêng `advance_mode` của dự án. Tuỳ chỉnh giữ các thiết lập sâu. Chỉ Văn phong có nhãn theo dự án.
- **M4:** Thêm › Cách viết có dòng tóm tắt văn phong và số quy tắc; đổi văn phong bằng danh sách tại chỗ, lựa chọn đầu tiên là Theo dự án. Tuỳ chỉnh truyện chỉ còn duyệt, người viết và ngân sách; xoá logic chọn văn phong cũ ở trang này.
- **Dùng chung với Tạo truyện mới:** các bảng chọn ở `quick-pickers.tsx`; cả hai luồng dùng `writerConfig` để giữ nguyên vai trò khác khi đổi/xoá `roles.writer`. Phần Tạo truyện mới do AmNewStory thực hiện và kiểm chứng riêng; không sửa QuickMode trong phần AmWritingTab.
- **Kiểm chứng AmWritingTab:** `npx tsc -b --noEmit` xanh; 15 tệp/79 test liên quan xanh với `--pool=threads --maxWorkers=2`, gồm ratchet màu và chặn hai className. Ba regression mới (chuyển tab cũ, nhãn nguồn, phạm vi PUT chọn nhanh) đã đỏ trước khi sửa. Baseline màu chỉ giảm: Rules mobile 11→9, RightColumn 32→21.
- **Browser thật, mock :5200:** 1440×900 xác nhận một tab Cách viết, chuyển URL cũ sang style, chọn nhanh Hồi hộp và gpt-5.4 không rời trang, toast văn phong từ chương 4, toast người viết từ chương 4, switch duyệt và toast đúng. Dự án chọn Hồi hộp ghi `target.config.style=suspense` cấp project; switch ghi `advance_mode=review`. 390×844 xác nhận Thêm › Cách viết đổi Hồi hộp tại chỗ, danh sách đóng lại, `document.scrollWidth=390`; Tuỳ chỉnh truyện không còn chọn văn phong.
- **Bẫy căn giữa:** `getComputedStyle`/`getBoundingClientRect` trên thẻ kế thừa đang hiện cho kết quả `textAlign=left`, `alignItems=stretch`, `borderStyle=dashed`; thẻ x=292, nội dung x=309, chênh 17px. Artboard Outline đo cùng chênh 17px (x≈307,94, chữ x≈324,94), không bị lớp `.rule.inh` chi phối.
- **GAP AM-1 (quyết định Main):** §AM mô tả `roles.default`, nhưng runtime ưu tiên `roles.writer`. Main yêu cầu cả Tạo truyện mới và chọn nhanh ghi `roles.writer` (provider/model), chọn Theo dự án xoá `roles.writer`; không đổi backend/hợp đồng trong tài liệu nguồn.
- **GAP AM-2 (dữ liệu):** các model/provider/văn mẫu/tên truyện và mô tả gói lấy từ API, không chép dữ liệu minh hoạ artboard. AmNewStory báo danh sách cấu hình chỉ có gpt-5.4 và claude-sonnet-4, mặc định dự án o4-mini; đã kiểm chứng claude-sonnet-4 trên truyện mới 15 và kế thừa trên truyện 16.
- **GAP AM-3 (token):** `--kit-shadow-pop` có thêm bóng phụ và inset so với bóng đơn của qpop artboard. Giữ token hiện có, không thêm màu/bóng viết cứng hay sửa gói token ngoài phạm vi.
- **GAP AM-4 (xung đột số đếm):** artboard và một đoạn §AM còn mô tả số đếm trong nhóm/mặc định. Yêu cầu triển khai mới nhất nói không có số đếm trong tab; vì vậy bỏ số đếm quy tắc ở nội dung, giữ số đoạn/ký tự văn mẫu và các tham số gói theo yêu cầu riêng.
- **GAP AM-5 (đơn vị API, kế thừa AL-1):** hàng văn mẫu dùng nhãn ký tự theo yêu cầu/artboard, nhưng `StyleSample.chars` của mock đang đếm từ. Không tự tính lại hoặc sửa API trong gói AM.
- **GAP AM-6 (khung dùng chung):** artboard có rail desktop rộng hơn khung app hiện tại và mô phỏng status bar điện thoại; không thay shell toàn cục để bù vị trí tuyệt đối. Bố cục/thẻ nội dung được so theo kích thước viewport và phần thụt nội dung thực tế.
- **GAP AM-7 (Tạo truyện mới trên điện thoại):** không có artboard mobile tương ứng; 18-mobile quy định tạo truyện ở máy tính. AmNewStory đo QuickMode desktop khi thu hẹp 390px, không tạo màn mobile mới.
- Bằng chứng ảnh: `/tmp/AmWritingTab-desktop.png`, `/tmp/AmWritingTab-mobile-writing.png`, `/tmp/AmWritingTab-mobile-list.png`, `/tmp/AmWritingTab-project.png`, `/tmp/AmWritingTab-artboard-mobile.png`. Phần **Xoá truyện** ngoài phạm vi.

## Gói AN · Lưu trữ tệp (31/10/2026)

- Tuỳ chỉnh có **Lưu trữ tệp** ngay sau Thành viên, chỉ quản trị; gồm chế độ thư mục, ổ máy chủ, lưới tám nhà cung cấp, form theo dịch vụ, kiểm tra, kho do máy chủ đặt, thiếu khoá mã hoá, xác nhận chuyển, đang chuyển/tạm dừng/có lỗi, kho đang dùng và mất kết nối. Thanh đầu app dùng chung dữ liệu với mục này: viên tiến độ/cảnh báo, dòng đỏ dưới thanh đầu khi kho lỗi; Thêm trên điện thoại có nhóm Quản trị và trang chỉ xem/kiểm tra.
- Kiểm tra dùng tiến độ NDJSON thật, ráp dòng bị cắt và kiểm tra kiểu dữ liệu trước khi nhận. Luôn sáu dòng theo mã bước, không suy ra từ thứ tự trả về. Trình duyệt tự tải link ký sau kết quả máy chủ; lỗi tải được thử lại bằng chế độ không CORS để phân biệt kho chặn CORS với không tới được kho. Ba mẫu R2/AWS/Garage lấy đúng địa chỉ trang hiện tại, tên header viết thường; lệnh Garage, cấu hình và lệnh tạo khoá chỉ ở khối chi tiết/đoạn chép.
- Quy tắc địa chỉ nội bộ được đối chiếu cùng 23 trường hợp Go, kể cả IPv6 công khai, địa chỉ IPv4 ánh xạ trong IPv6 và địa chỉ không hợp lệ. Đổi bất kỳ đầu vào nào huỷ phép thử cũ và khoá Lưu. Trước xác nhận đọc lại tổng số tệp/dung lượng; chỉ bắt đầu chuyển sau PUT thành công và phép thử đạt, dùng phiên bản PUT trả về. Xung đột hoặc thử thất bại không gọi chuyển.
- Một bộ hẹn giờ dùng chung cho header/trang: 5 giây khi chuyển, 30 giây cho sức khoẻ kho; không chạy khi tab ẩn, không gọi API kho đối với người không phải quản trị. Đủ tệp được chép/đọc lại mới đổi kho; tạm dừng, tiếp tục, huỷ và thử tệp lỗi dùng API thật. Kho cũ chỉ hiện khi đã hoàn tất và chưa dọn; lệnh dọn dùng mã lượt chuyển, xem trước rồi mới xác nhận.
- **GAP-AN-FE1 (bước đồng hồ):** API có thêm `clock`; artboard luôn sáu dòng. Bước đạt không tạo dòng riêng; lệch giờ gắn vào **Ghi thử** với tiêu đề/cách sửa tương ứng artboard, theo quyết định Main.
- **GAP-AN-FE2 (thiếu khoá mã hoá):** artboard cũ nói chưa chạy được bước trình duyệt. Máy chủ hiện dọn link thử bằng bộ hẹn giờ khi chưa có khoá, nên vẫn kiểm tra cả trình duyệt; chỉ **Lưu** bị chặn, theo quyết định Main. Không giả lỗi để khớp minh hoạ.
- **GAP-AN-FE3 (khung và số liệu minh hoạ):** artboard điện thoại mô phỏng status bar; giữ safe-area/header của app thật, không thêm status bar giả. Dữ liệu/tên kho/dung lượng/tốc độ, giờ, môi trường, kết quả thử và tệp lỗi lấy từ máy chủ, không chép số minh hoạ. Thẻ desktop rộng 820px, thẻ điện thoại rộng 358px ở viewport 390; dùng StyleX/token hiện có, không thêm màu hex.

- **Gói AN · rà soát giữ khoá và số liệu:** ngay khi lưu kho thành công, xoá bản nháp và kết quả thử khỏi state, đóng form/hộp xác nhận trước khi chờ bắt đầu chuyển. Nếu bước bắt đầu lỗi, thử lại bằng phiên bản kho đã lưu, không giữ hay gửi lại khoá. Thống kê “Tạo link” gom đúng phép ký link tải mà máy chủ ghi, gồm số lần, lỗi và độ trễ; có kiểm thử component với dữ liệu thật của hợp đồng API.

---

## Gói AO — Đăng nhập chung với Viderse

Đã khép phần web Noverse của Gói AO:

- Menu tài khoản trên máy tính và tab **Thêm** trên điện thoại có nhóm **Ứng dụng**, lấy địa chỉ từ `apps` của `/api/me`. Nhóm ẩn hoàn toàn nếu không có quyền Viderse hoặc chưa cấu hình app kia; kho tệp giữ giao diện cũ. App đang dùng đứng trước, app kia mở thẻ mới.
- Màn hình chưa được cấp quyền dùng `apps` của lỗi 403 để mở Viderse, có lời nhờ quản trị viên có thể chép và nút thử lại.
- Đăng xuất, đổi mật khẩu, thu hồi phiên và các xác nhận đặt lại mật khẩu/khoá nói rõ hệ quả ở cả hai app. Thu hồi các thiết bị khác của chính mình giữ phiên hiện tại.
- Màn đăng nhập giải thích lý do kết thúc phiên theo lỗi 401 `reason`/`app`: tự đăng xuất, đăng xuất ở Viderse, đổi mật khẩu, quản trị thay đổi/khoá tài khoản hoặc không rõ nguyên nhân. Hết hạn giữ khung A1 cũ, không thêm khối lý do AO; trình duyệt sạch không hiện thông báo phiên bị kết thúc. `sharedAccounts` từ `/api/setup` quyết định dòng phụ cho kho tài khoản chung.
- `#/settings/members?user=<tên>` mở đúng tài khoản; tên không tồn tại quay về danh sách, không chọn nhầm người.

Đối chiếu **canvas 130**: dùng `Login`, `Members`, `MMore`, `Phone` trong chính monorepo verse. Mục **Quyền vào ứng dụng** đứng sau **Dự án**; chỉ hiện khung hệ quả sau khi lưu thành công, đủ các trạng thái đổi quyền, tắt Viderse và không còn app nào. Hồ sơ dùng tên, tên đăng nhập, ngày tạo và lần hoạt động từ máy chủ; không thay thời điểm chưa có bằng số liệu minh hoạ. Đổi mật khẩu PostgreSQL thu hồi mọi phiên, kể cả máy hiện tại (`platform/identity/postgres/passwords.go`); kho tệp giữ câu cũ. Menu neo theo trigger với khoảng dọc 9px, lệch phải 4px; chiều cao header không còn phụ thuộc loại kho hay quyền ứng dụng, theo nghiệm thu khung dùng chung bên dưới.

### Ngoại lệ đã chốt với Main

- **(a) Nền vòng tròn logo:** board ghi `#18181B` nhưng chưa có token tương ứng; dùng `--color-surface-3` (`#18181A`) theo Main, không thêm mã màu hay sửa `platform/ui`. Design sẽ quy về token.
- **(b) Địa chỉ app chưa cấu hình:** khi `apps[]` không có app tương ứng, bỏ dòng địa chỉ và giữ thẻ trung tính; không viết cứng hoặc bịa địa chỉ Viderse.
- **(c) Điều hướng trong menu:** chế độ chung ẩn riêng **Tuỳ chỉnh** và **Hướng dẫn lệnh** để khớp `Login.state=menu`; hai mục vẫn mở được từ header/sidebar. Kho tệp giữ nguyên menu cũ.
- **(d) Huy hiệu quản trị:** menu chế độ chung dùng huy hiệu gọn **Quản trị**; board ghi `#CBB992` và nền vàng `.12` nhưng chưa có token. Theo Main dùng tạm `--members-admin-fg` / `--members-admin-bg` (`.14`); kho tệp và các chỗ dùng `RoleBadge` khác giữ nguyên.
- **(e) Status bar điện thoại:** theo **GAP-AN-FE3**, giữ safe-area của máy thật, không vẽ lại `9:41` hay các biểu tượng hệ điều hành trong app. Với `MMore`, so vị trí tương đối từ content top; khung Ứng dụng có cùng toạ độ tương đối, kích thước và khoảng cách với board. Màn đăng nhập chung dùng safe-area thực, không cộng thêm khoảng dự phòng 8px.

Kiểm tra: TypeScript và vitest chọn lọc; lỗi trước sửa cho thông báo quyền, denied dùng app từ máy chủ, bảo toàn kho tệp, thứ tự Dự án/Ứng dụng và menu chung. Đo bằng `getComputedStyle`/`getBoundingClientRect` ở cùng viewport với board; bằng chứng và bảng đo trong báo cáo SsoNoverseWeb. Lượt trước đã kiểm tra backend thật với phiên dùng chung. Không thêm mã màu trực tiếp hay sửa `platform/ui`.

## Gói AP — Thành viên · Quyền vào ứng dụng (canvas 132)

- Phần Thành viên của nhánh đăng nhập chung dùng `Checkbox` chuẩn kit với màu nhấn mặc định; bỏ biến thể đơn sắc. Giữ nguyên luồng lưu quyền, trạng thái chờ/lỗi và quyền kế thừa; không sửa bộ thành phần dùng chung.
- Lưới ứng dụng dùng `repeat(auto-fit,minmax(210px,1fr))`, khoảng cách 10px, tự về một cột khi khung hẹp hơn 430px. Tên miền dùng chữ giao diện 12px, không đơn cách, cắt bằng dấu ba chấm và giữ địa chỉ đầy đủ trong chú giải.
- Hồ sơ hiển thị thời điểm hoạt động tương đối từ `lastSeenAt` bằng bộ định dạng đang dùng ở Tài khoản. Giá trị chưa có vẫn giữ trạng thái chưa đăng nhập; không dùng thời gian minh hoạ. Sai lệch 17.546875px ở viewport 1120px đã được truy tới dòng thông tin này và sửa, không ghi nhận là ngoại lệ.
- Đo đối chiếu với `Members.dc.html` của `9b3214fd`: ở 1440px, hai thẻ rộng 342px/cao 62.390625px, Y=709.328125; ở 1120px, một cột rộng 374px, Y=924.328125 và 996.71875. Ở 1176px, hai cột 210px, tên miền rộng 108px và có cắt tràn. Các kích thước và vị trí đều trùng board; không thêm mã màu trực tiếp.

## Gói AP · Tuỳ chỉnh và thanh bên (31/10/2026)

- Tuỳ chỉnh mặc định mở Trợ lý AI; bỏ mục, route, bản nháp và component Cách viết, cùng toàn bộ phần gắn Quy tắc chung vào màn Tuỳ chỉnh. Cách chạy nằm ngay dưới danh sách trợ lý: truyện giữ lệnh duyệt trực tiếp; dự án/mặc định dùng `advance_mode`, Trọng tài hỏi dùng `ask_user`, lưu qua luồng cấu hình có phiên bản hiện có.
- Thông tin dự án luôn có trong thanh bên; bấm khi đang xem truyện chuyển sang cấp dự án và giữ ngữ cảnh trong URL. Thông tin truyện và lịch sử phiên bản không còn ở Tuỳ chỉnh. Các lối cũ từ audio/kết quả học văn phong chuyển sang tab Cách viết của truyện.
- Thành viên dùng cùng SettingsNav, gồm Lưu trữ tệp, tổng số thành viên và phiên bản máy chủ; Não hệ thống hiển thị số bài học chờ duyệt trong dòng phụ tím nhạt thay vì huy hiệu số. Phần quyền ứng dụng do nhánh SSO thực hiện theo phân công của Main.
- Đo 1440×900: bề rộng/chiều cao danh sách trợ lý 860×550, Cách chạy 860×236.25, thẻ thông tin 820×378, thanh bên 248px, dòng Não hệ thống 227×36; phụ đề 11.5px/14.95px, màu `--kit-badge-run`, khớp artboard canvas 132. Sai lệch thanh đầu −1px đã được sửa tại AppHeader dùng chung, không bù viền ở màn riêng.
- Kiểm tra luồng thật trên máy chủ thư mục tại 8831: bấm Duyệt từng chương ở cấp dự án gửi `PUT /api/config?scope=project&project=smoke-one` với `advance_mode=review` trả 200; GET đọc lại review, tải lại trang vẫn bật. Tắt và lưu gửi `advance_mode=auto` trả 200, GET đọc lại auto; fixture đã trả về trạng thái ban đầu. Bảy tệp Vitest liên quan chạy 34 kiểm thử đều đạt; kiểm thử mới về hai công tắc đã ghi nhận thất bại trước khi thêm Cách chạy.
- 390×844: Settings và Members desktop không được ép vào 142px còn lại như bản desktop artboard thu hẹp; app dùng MMore theo 18-mobile. Không có thanh bên ở điện thoại; chiều rộng nội dung bằng 390px. SysBrain.dc.html tự nó cũng không có thanh bên Tuỳ chỉnh — thay đổi hàng Não hệ thống được đo trong Settings/Members. MMore giữ safe-area thật, không dựng status bar minh hoạ.

### Não dự án

- Bỏ hẳn cột Ngày 7 và chú thích trung vị, kể cả khi API cũ trả số. Khối kiểm hiển thị kết luận và đủ bốn hàng tên phép kiểm, mức, nội dung, gợi ý. Bảng truyện, tần suất từng trục, lọc truyện trùng và soi ô dùng dữ liệu API sổ cái hiện có; số lần chỉ đếm mười truyện trước, không đếm lại truyện mới.
- Bấm “Thợ may” với dữ liệu tương đương artboard: ba ô (truyện mới và hai truyện cũ) cùng viền tím 1,5 px; dải đếm ghi “dùng 2 lần trong 10 truyện gần nhất”. Tiêu đề cột dính khi cuộn. Vũ trụ chung lọc theo loại, mục cần xem, tên; `checks: null` hiện Chưa kiểm và thẻ Chưa có kết quả kiểm. Tìm lọc thực thể/đoạn văn và truyện ngay trên kết quả đã tải.
- Đo cặp canonical canvas 132 và app ở 1440×900 đủ `lview=table|freq|pick|only`, `uni=checked|unchecked`, `sfilter=all|ent|hits`: khối kiểm 1392×292,5; hàng kiểm 1390×41; tiêu đề bảng 1390×54; ô 112,23×26; dải soi 1392×44; hàng tần suất 1390×58,98; bảng vũ trụ 1038×406,25; ô lọc tên 200×30; thẻ tìm thực thể 320×307,625 (Tất cả) hoặc 860×289,625 (Thực thể); thẻ đoạn văn 1058×335,0625 hoặc 860×335,0625. Kích thước khớp từng trạng thái; bằng chứng chi tiết `/tmp/ap-brain-measurements.json`.
- **Đã đóng GAP AP–Não dự án (khung dùng chung):** AppHeader lấy chiều cao tự nhiên, không còn nhánh theo quyền hay màn. Kiểm chứng thật cả kho tệp và PostgreSQL cùng 55px; xem nghiệm thu khung dùng chung bên dưới.
- **GAP AP–Não dự án (điện thoại):** không có artboard ProjBrain điện thoại; router hiện chuyển URL này sang MMore ở 390×844. Đã đo `document.scrollWidth=390`, không cuộn ngang; không dựng một luồng điện thoại mới ngoài thiết kế.


### Gói AP · Tab truyện — Thông tin, Tiền đề, Thế giới

- Tab theo thứ tự Thông tin · Tiền đề · Nhân vật · Thế giới · Dàn ý · Cài cắm · Cách viết · Bản xuất, không biểu tượng. Khung truyện chuyển khỏi Dàn ý sang Thông tin; ô sửa sách dùng GET/PUT `S/meta`, giữ phiên bản sau mỗi lần lưu. Menu Đổi tên mở `?tab=info&edit=title`, lấy tiêu điểm ô tên. Các tab dùng Activity để không mất trạng thái khi chuyển qua lại.
- Tiền đề đọc `GET S/premise`, hợp nhất mục trong tệp với `missing[]` của máy chủ, không suy tầng lập truyện từ số chương; giữ cả mục tuỳ biến. Mục bắt buộc bị xoá hiện dòng vàng và Nhờ bổ sung. Thế giới đọc `GET S/world-rules`, nhóm Xã hội/Công nghệ/Địa lý/Phép thuật/Khác, hiển thị giới hạn. Không còn nhánh snapshot/brain tạm sau khi AP4/AP5 có thật.
- Đã chạy trình duyệt với máy chủ tệp thật tại `8834`, Vite `5338`, tài khoản và bản sao thư viện riêng dưới `/tmp/APStoryTabsWeb` (không sửa thư viện gốc). Xoá thực mục Vùng cấm sáng tác trong premise.md: hiện thiếu 1/10 và nút bổ sung. Xoá thực world_rules.json: thẻ trống Thế giới và dải Nền móng vàng, chữ đo được `rgb(243,217,174)`. Nhờ bổ sung điền đúng tên mục; Nhờ Kiến trúc sư lập điền “Lập quy tắc thế giới từ 8 chương đã viết”; cả hai lấy tiêu điểm `id=steer`. Đã khôi phục các tệp sau đo. Hai lần lưu inline liên tiếp thành công, GET sau lưu vẫn có version/builds.
- Cách viết gắn RulesPanel dùng chung (bao gồm AntiAiCard và thời điểm chạy); sửa cỡ tiêu đề 15px/1,5, bổ sung dòng giải thích văn mẫu còn thiếu, khoảng cách tới Quy tắc đúng 22px. Dòng Đang áp dụng không còn “Mọi truyện”.
- Đo đủ 14 cặp `Outline: tab=info|premise|cast|world|outline|fs|export × found=full|partial` tại 1440×900 bằng canonical canvas 132 của worktree này; JSON chi tiết `/tmp/APStoryTabsWeb/measurements.json`. Bảng dưới là vị trí Y thẻ đầu, tương đối với đầu vùng nội dung; thứ tự số là board/app, full rồi partial:

  | Tab | Full | Partial |
  | --- | --- | --- |
  | Thông tin | 16 / 16 | 16 / 16 |
  | Tiền đề | 74 / 74 | 74 / 74 |
  | Nhân vật | 16 / 16 | 16 / 16 |
  | Thế giới | 93,5 / 93,5 | 74 / 74 |
  | Dàn ý | 62 / 62 | 62 / 62 |
  | Cài cắm | 47,5 / 47,5 | 47,5 / 47,5 |
  | Bản xuất | 16 / 16 | 16 / 16 |

- Tab 34px, chữ 13px/19,5px, đệm ngang 12px, khoảng cách 2px, bo 9px khớp; thẻ Thông tin/Tiền đề/Thế giới bo 16px, Cài cắm 14px. Dải Nền móng cao 68,41px ở cả full/partial, khớp board. Độ dài nội dung của thư viện thật khác mẫu ngắn trên board: 11 quy tắc thay vì 5; 17 cài cắm thay vì 3; tiểu sử/tiền đề đầy đủ dài hơn; danh sách chương và bản xuất thực cũng khác. Không co nội dung thật hay chế dữ liệu để ép chiều cao thẻ khớp.
- **Ngoại lệ đã chốt AP–khung:** board khai báo cột trái 264px nhưng tràn thành 279,94px do min-width:auto; app cố ý giữ 264px/min-width:0. Nội dung vì vậy x=292, rộng 800px thay vì x=307,94, rộng 784,06px; cùng tên truyện vẫn có thể xuống 2 dòng thay vì 3. So geometry tương đối trong cột, không bù bằng số cứng mô phỏng lỗi tràn.
- **GAP AP–tab truyện điện thoại:** chưa có board điện thoại cho Thông tin/Tiền đề/Thế giới (cũng không có bảy biến thể tab truyện). Theo quyết định giữ thiết kế hiện tại, URL truyện tại 390×844 tiếp tục mở MWork, không dựng màn mới. Đã đo scrollWidth=390, không cuộn ngang; không tuyên bố có cặp đo tab điện thoại khi tab không được mount.
- Kiểm tra: `tsc --noEmit` thành công; 11 bài kiểm tra thuộc 5 tệp tab/applied thành công. Đã chứng minh kiểm tra bắt lỗi bằng đột biến mất cờ missing (3 bài thất bại đúng hành vi) và trước khi thêm invalidation (mapping event thiếu premise/world-rules thất bại). Ratchet màu chỉ báo giảm literal, không tăng; Main chốt lại baseline chung sau khi các phần AP nhập xong.

### Gói AP · Quy tắc và chống giọng văn máy

- `RulesPanel` thay `RulesEditor`, dùng chung cho truyện và dự án; `AntiAiCard` dùng cùng giao dịch sửa quy tắc trên máy tính và MMore. Không thêm endpoint hay kho thiết lập. Mỗi lần Nới/Siết cập nhật dòng Giới hạn; về mức nền tự bỏ dòng. Ngưỡng 0 là Giới hạn 0, không thêm vào danh sách cấm không thể gỡ.
- Ô thêm một dòng, hàng Mẫu, thẻ Học từ góp ý riêng; mỗi quy tắc một hàng, một nhãn loại. Năm loại có thật trong API là ban/chr/max/genre/pref; pref hiển thị Lời dặn, không suy đoán Nhân vật/Giọng văn/Cốt truyện từ câu chữ. Nhóm Từ dự án chỉ đọc ở truyện; chỉ còn Chuyển lên dự án. Không có cấp Mọi truyện trong hai màn.
- AP2 dùng `fatigueWords` và `forbiddenDefaults`; `basePerChapter`/`baseSource` giữ đúng mức gói/dự án phía dưới khi ↺. Snapshot lịch sử thiếu nguồn vẫn giữ nguyên số, không đoán huy hiệu nguồn. Huy hiệu vượt so sánh số dùng ở chương gần nhất với ngưỡng đang sửa, không cần chấm lại.
- Nghiệm thu 4 quy tắc truyện + 2 học từ góp ý + 2 dự án: browser 1440×900 hiện **8 đang áp dụng**. Cuộn bình thường tới Quy tắc: heading y=192,25; ô thêm y=260,39 cao 44; thẻ học y=348,39 cao 165,14; danh sách y=525,53 cao 346, đáy=871,53 — đủ tám quy tắc trong màn 900px. Canonical cùng chiều cao ô thêm 44, học 165,17, danh sách 346. Ngoại lệ chiều rộng cột 800/784,06 theo quyết định AP–khung ở trên, không co dữ liệu thật.
- Browser bấm + ở “như một” 3 lần: xuất hiện `“như một” tối đa 4 lần mỗi chương.` ngay trong danh sách; ↺ bỏ dòng, số về 3. Thẻ mở có đủ 16 từ và 4 cụm khoá. Checkbox kiểm tra chương cũ dùng Checkbox kit: nền khi chọn rgb(201,166,245), không dùng nút tick tự vẽ.
- Browser thật (Vite proxy tới file backend riêng, không gọi nhà cung cấp): `/tmp/APRulesWeb-fixture`, truyện `smoke-one/ap-rules`; seed rule/cache chuẩn hoá và `meta/rule_violations.jsonl` cho chương 3 dùng “dường như” 2 lần, ngưỡng 1. UI hiện **vượt ở ch.3**; bấm + lên 2 làm huy hiệu biến mất. Đây là fixture đầu vào, không phải kết quả chạy mô hình; không tuyên bố đã gọi Apply hay chấm lại với nhà cung cấp.
- Đo paired canonical canvas132: Outline `rules=list/understood/thinking/unsure`, `aai=closed/open/edited`, `view=project` lấy từ runtime board (artifact `42835`). App đã thao tác list/understood, aai đóng/mở/sửa và dự án: thẻ đóng 69,5px (khớp), từ 42px (khớp); thẻ mở truyện 583,75px so board 615,75px vì danh sách cụm từ ở cột 800px nằm một hàng 24px, ở cột 784,06px xuống hàng 56px. Dự án danh sách 130px khớp; chiều cao thẻ mở 564,375px so board 585,75px còn khác theo dữ liệu gói/câu chân thẻ. Không thêm dòng giả để ép chiều cao. Trạng thái thinking/unsure giữ luồng cũ và có kiểm tra hook, chưa có cặp browser app tương ứng trong lần đo này.
- MMore 390×900: home group rộng 358, hàng bên trong **356px**, x=17; không có lỗi hàng 364px ở trạng thái đã ổn định, nên không thêm override chiều rộng. Rules/RulesAai: thẻ 358px/x16, header 356px/x17 cao 72px; thẻ mở **1054,9375px khớp canonical**, hàng từ đầu 52px, các hàng thường 53px, hàng có nguồn 53,4375px. Không cuộn ngang. Đã sửa nhãn loại điện thoại từ 22 lên đúng 24px của `.uch`; nguồn văn bản thật dài hơn mẫu vẫn có thể làm hàng học từ góp ý cao hơn.
- Kiểm tra cuối logic: 53 bài thuộc 8 tệp (rules, mobile rules summary/permissions, scope save) đều qua; `tsc --noEmit` qua. Trước sửa đã chặn thử setFatigue và ẩn đối chiếu vi phạm: 3 kiểm tra thất bại đúng hành vi, khôi phục rồi qua. Ảnh browser cục bộ: `/tmp/APRulesWeb-story-rules-1440.png`, `/tmp/APRulesWeb-mobile-home-390.png`, `/tmp/APRulesWeb-mobile-rules-390.png`, `/tmp/APRulesWeb-mobile-rulesAai-390-final.png`. Main cần gate tích hợp cuối sau các commit AP.


### Khung header dùng chung · đo lại canvas 132 (01/11/2026)

- Nguồn quy luật là nội dung, không phải quyền hay màn: ở các artboard app chính, nav cao 38px (mục 30px, lề 3px mỗi phía, viền 1px mỗi phía); header thêm lề dọc 8px mỗi phía và viền dưới 1px. `shell/AppHeader.tsx` bỏ chiều cao cố định/điều kiện `grants` và giữ lề của board. Nếu nội dung cao hơn, khung tự tăng thay vì cắt.
- Đã mở toàn bộ 41 artboard và đo 157 tổ hợp mặc định/tweak liên quan tài khoản, thư viện và màn nền. Các header app Main, Outline, Settings, Members, ProjBrain, SysBrain, Brain, Reader, Quality, Jobs, Transcript, AudioExport, NewStory, Commands, CmdResults, Feedback và Slash đều 55px. Login/LoginQR chuyển giữa khung công khai 58px và khung app 55px theo trạng thái; có/không quyền Viderse không đổi chiều cao. Setup 58px, Onboard 55px, mobile appbar 52px và Phone theo nội dung riêng — không dùng AppHeader desktop.
- Khác biệt có thật trong board cũ: Activity dùng nav 40px và lề dọc 10px nên 61px; Kit gallery cũng 61px; Nav so sánh đặt cố định 54px. Giữ một quy luật chung của app, không thêm nhánh theo tên màn để tái tạo các biến thể minh hoạ này. Bằng chứng đầy đủ `/tmp/ap-header-measurements.json`.
- Máy chủ thật kho tệp và PostgreSQL riêng: đo 17 route ở mỗi chế độ đều 55px, ba vùng con lần lượt tối đa 32/38/32px; mở menu tài khoản không đổi. Trước có tài khoản và sau đăng xuất, khung công khai vẫn 58px. Tăng tạm mục nav từ 30 lên 52px trong browser làm header tự tăng 55→77px, trả mục về 30px thì header về 55px ở cả hai chế độ. Ba ca hồi quy không tài khoản/kho tệp/hosted đỏ trước bản sửa; 15 kiểm thử AppHeader/AccountMenu/AppSwitcher đạt sau sửa.

## Gói AQ — trả lời các chỗ dev đang làm tạm (01/11/2026, canvas 133) <!-- aq-gap-closure -->

| # | Chỗ tạm của dev | Quyết định design | Artboard |
|---|---|---|---|
| 1 | Hộp Dự án mới còn “chép từ Mọi truyện” | **Đổi.** Mặc định *Bắt đầu trống*; tuỳ chọn *Chép từ dự án khác* (cần GAP-AQ1; chưa có thì ẩn). Ô chọn chỉ còn nghĩa model/ngân sách (`shareGlobal`). | `Dialogs` `npcopy` |
| 2 | GAP AP–tab truyện điện thoại: URL truyện ở 390 px mở MWork | **Vẽ M9.** Thôi chuyển hướng khi URL có tab truyện; MWork có nút Truyện. | `MOutline` |
| 3 | GAP AP–Não dự án điện thoại: router chuyển sang MMore | **Vẽ M10** (không bảng 10 cột). | `MProjBrain` |
| 4 | Luồng viết chỉ có mẫu thẻ vòng chung | **Vẽ đủ thẻ theo loại** + lịch sử điện thoại. Bảng `20-settings-rules.md` §10. | `RunCards`, `MRun` |
| 5 / AO (d) | Huy hiệu Quản trị `#CBB992`/.12 chưa có token | **Chốt token** `--members-admin-fg #E3C99E` / `--members-admin-bg rgba(184,154,110,.14)` (đã thêm `tokens.css`); board Login đổi theo. | `Login` `state=menu`, `Members` |
| 6 / AO (c) | Menu chung ẩn Tuỳ chỉnh, Hướng dẫn lệnh | **Xác nhận**, và áp cho cả kho tệp (một menu cho hai chế độ). | `Login` `state=menu` |
| 7 / AO (b) | Không có `apps[]` → bỏ nhóm Ứng dụng | **Xác nhận** (như `viderse=no`). | `Login`, `MMore` |
| 8 / AO (a), (e) | Logo dùng `surface-3`; giữ safe-area thật | **Xác nhận cả hai.** Board đổi `#18181B` → `#18181A` (= `--color-surface-3`); 40 px đầu board điện thoại chỉ là chỗ status bar minh hoạ. | `Login`, `MMore`, `Members` |
| AP–khung | Cột trái board tràn 279,94 px | **Sửa board**: `min-width:0`, đúng 264 px như app. | `Outline` |
| AP–quy tắc | Năm loại thật ban/chr/max/genre/pref; không suy Nhân vật/Giọng văn/Cốt truyện | **Xác nhận** — nhãn ngoài 5 loại trên board chỉ là minh hoạ; app hiện theo dữ liệu. Ngưỡng 0 là Giới hạn 0 (đúng). | `Outline` `tab=style` |
| AP–độ dài thật | Dữ liệu thật dài hơn mẫu | **Xác nhận** — không co dữ liệu để khớp chiều cao board; so bố cục tương đối. | — |

### Gói AQ — lệch khi triển khai

- Luồng viết › tiêu đề vòng: vòng cũ chưa lưu metadata công việc nên không có `title`; chỉ hiển thị số vòng, vai và giờ (“Vòng N · {vai} · hh:mm”). Vòng mới lấy `title` từ trạng thái của host lúc giao việc, không suy từ chữ tự do của điều phối.
- Huy hiệu Quản trị trên `MMore` còn tím (`rgb(220,200,248)` / `rgba(201,166,245,.14)`); app dùng `--members-admin-fg` / `--members-admin-bg` theo mục 5 của Gói AQ cho mọi huy hiệu, kể cả điện thoại. Nhờ design sửa board cho thống nhất.
- M9 `found=partial`: board minh hoạ 2 mục thiếu trong 10 mục, gồm cả “Tính phù hợp với truyện ngắn”. Với fixture tier thực tế, API chỉ báo thiếu “Vùng cấm sáng tác” trong 9 mục; mục còn lại không thuộc danh sách bắt buộc của tier. App giữ danh sách từ API, không tự thêm mục hay tạo lời nhờ cho mục không được máy chủ yêu cầu. Khối tiền đề do đó cao 936,875 px thay vì 1.068,75 px của board ở 390 px. Thanh trạng thái 40 px của board là minh hoạ; app dùng safe-area thật theo mục 8.
- Màu vai Đồng phạm trên M9 chưa có token trong `tokens.css`. Dev thêm `--role-accomplice-fg: #E8A3AA` và `--role-accomplice-bg: rgba(255,154,162,.06)` theo board `MOutline`; nhờ design xác nhận tên token.

## Bỏ truy cập “cùng Wi-Fi / Tailscale” (01/11/2026) <!-- no-lan -->

Quyết định của chủ sản phẩm (01/11/2026): bỏ hẳn tuỳ chọn cũ “cùng mạng LAN / Wi-Fi nhà / Tailscale”. Thành viên vào bằng địa chỉ Noverse mà quản trị viên gửi, từ bất kỳ máy nào có Internet. App **cố ý lệch** các artboard sau; không sửa artboard.

| Artboard | Phần bị bỏ | App làm |
|---|---|---|
| `Login` / `LoginQR` | trạng thái `qrNoLan`, tab “Wi-Fi nhà / Tailscale”, nút “Cho phép thiết bị cùng Wi-Fi” | Một liên kết duy nhất (`PUBLIC_URL`, hoặc địa chỉ trang đang mở); không còn tab và không còn trạng thái “chưa vào được” |
| `Members` | dòng thứ ba “Mở Noverse: Cùng Wi-Fi nhà, hoặc quét QR” | Dòng “Mở Noverse” hiện địa chỉ thật (`location.origin`); “Chép lời nhắn” chép đủ tên đăng nhập, mật khẩu tạm và địa chỉ |
| `Settings` › Nâng cao | hàng “Cho phép thiết bị cùng Wi-Fi” và “Truy cập khi ra ngoài (Tailscale)” | Nhóm còn một hàng “Mã khôi phục quản trị” (đổi tên nhóm thành “Khôi phục quản trị”) |
| `Onboard` | tab “Ở nhà / Ra ngoài” ở bước điện thoại | Một mã QR, không tab |
| `Phone` | chip mạng “Wi-Fi nhà” / “Tailscale” ở đầu màn | Bỏ chip; lời giải thích camera chỉ nói trình duyệt cần kết nối https |

Phía máy chủ: bỏ `PUT /api/system/network`, cờ `--lan`, khoá `lan` trong `serve.json` (tệp cũ còn khoá này vẫn đọc được) và trường `lan` của `GET /api/system`; `POST /api/me/pairings` và `POST /api/users/:u/pairings` trả đúng một liên kết, không dò card mạng. Giới hạn loopback của local SSO giữ nguyên.

## Gói AR — Truyện mới trên điện thoại · trả lời “Gói AQ — lệch khi triển khai” (01/11/2026, canvas 135) <!-- ar-gap-closure -->

| # | Chỗ dev nêu | Quyết định design | Artboard |
|---|---|---|---|
| AM-7 | Không có artboard Tạo truyện mới cho điện thoại; app thu hẹp QuickMode | **Vẽ M11** — bố cục riêng, cùng logic và dữ liệu. | `MNewStory` |
| AQ-lệch 1 | Tiêu đề vòng cũ không có `title` → “Vòng N · {vai} · hh:mm” | **Xác nhận.** Không suy tiêu đề từ chữ tự do. | `RunCards`, `MRun` |
| AQ-lệch 2 | Huy hiệu Quản trị trên `MMore` còn tím | **Sửa board**: `#E3C99E` / `rgba(184,154,110,.14)` như mọi huy hiệu Quản trị. | `MMore` home |
| AQ-lệch 3 | M9 `found=partial`: API chỉ báo thiếu mục bắt buộc của tier | **Xác nhận** — danh sách mục thiếu lấy từ API; board chỉ minh hoạ. Chiều cao khác board là đúng. | `MOutline` |
| AQ-lệch 4 | Màu vai Đồng phạm chưa có token | **Chốt tên** `--role-accomplice-fg: #E8A3AA`, `--role-accomplice-bg: rgba(255,154,162,.06)` (đã thêm `tokens.css`). Board M9 sửa nền 0,08 → 0,06 cho khớp máy tính. | `MOutline`, `Outline` |
| Máy tính · Mô phỏng văn phong | Nút tạo truyện rồi chạy học văn phong khi chưa có văn mẫu → lỗi | **Đổi**: dán văn mẫu trước (hộp thoại máy tính = ngăn M11). | `MNewStory` `sheet=samples` |

### Gói AR — đã làm (01/11/2026)

- M11 `MNewStory` ≤ 640 px (`screens/newstory/mobile/`): Bắt đầu nhanh (ô 16 px, ngăn Văn phong/Người viết, nút đáy dính kèm `disabledReason` chung với `QuickMode`, nâng theo `visualViewport` như ô chat M1, lỗi → *Thử lại*, mất mạng, mở từ ý tưởng), So với truyện trước 2 cột, Đồng sáng tác toàn màn với thẻ *Bản nháp yêu cầu* (bản nháp giữ qua tải lại), ba cách khác (tệp thiết lập gõ tên, bản thảo → import, ngăn dán văn mẫu). Điện thoại không còn thu hẹp `QuickMode` máy tính (đóng AM-7).
- Lối vào: M1 trống có *Tạo truyện mới*; M4 › Dự án & truyện có hàng *Truyện mới* cuối mỗi dự án; Hướng dẫn lệnh › `/start` “Dùng ở Truyện mới” + *Mở Truyện mới*.
- Máy tính › *Mô phỏng văn phong*: mở hộp thoại dán văn mẫu (cùng nội dung ngăn M11) → tạo truyện → gửi từng đoạn → học văn phong; lỗi giữa chừng *Thử lại* không tạo truyện thứ hai. **GAP:** hộp thoại máy tính chưa có artboard — dùng Dialog có sẵn với nội dung ngăn `sheet=samples`.
- Token `--role-accomplice-fg/bg` dùng ở Nhân vật (máy tính) và M9; huy hiệu Quản trị trên M4 dùng `--members-admin-*`.
- Chưa kiểm trên bàn phím ảo thật (chỉ DOM + test); GAP-AR1 (danh sách tệp thiết lập) chưa làm — không bắt buộc.

## Mật khẩu tạm bắt buộc đổi — bỏ “Để sau” (01/11/2026, quyết định của chủ dự án) <!-- temp-password-required -->

- Lệch so với `18-mobile.md` › Thêm › mật khẩu tạm lần đầu, `21-dev-prompt.md` M4 và bước Mật khẩu ở `09-auth-onboard.md`: tài khoản còn mật khẩu tạm **không có “Để sau”**, cả điện thoại lẫn máy tính.
- Lý do: máy chủ trả `403 password_change_required` cho mọi API (kể cả luồng sự kiện) tới khi đổi, nên “Để sau” chỉ đưa người dùng vào một app báo lỗi khắp nơi và băng “Mất kết nối” không tắt.
- App không mở luồng sự kiện khi còn mật khẩu tạm; đổi xong (không phải đăng nhập lại) thì tải lại phiên rồi mới mở.
- Bước Mật khẩu ở onboard vẫn “tùy chọn” với tài khoản đã có mật khẩu riêng.
- Artboard `MMore` (`page=firstPw`) và `Onboard` còn vẽ dòng “Để sau” — cần design xoá ở lần cập nhật canvas tới.
