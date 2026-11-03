# Nghiệm thu bắt buộc cho từng PR

## Phạm vi và môi trường
- Đây là kế hoạch kiểm thử, không phải báo cáo đã chạy. Không suy kết quả Vitest, API 202 hay fixture thành provider đã sinh ảnh thật.
- Mỗi PR có QA server và DB riêng; không dùng kho production hoặc điều khiển hàng đợi người dùng. Fixture đi qua schema/API thật, media thử có thật; không intercept thành công giả, không sửa DOM/state ứng dụng để dựng ảnh chứng cứ.
- **Trước mọi mutation QA** xác minh tách DSN app, DSN identity, process HOME/config, kho media/blob, browser storage và credentials provider QA. `auth.DefaultStore()` lấy `$HOME/.viderse/auth.json`, không theo DB/human account; provider logout có thể xoá credential thật nếu chỉ đổi DB. Dùng HOME/config riêng có credential QA được duyệt, kiểm đường resolve bằng metadata không đọc secret. Chưa chứng minh isolation thì block logout, paid draw và write smoke; PW_STATE/cookie files ngoài repo và trace không chứa chúng.
- Đọc manifest/lệnh serve hiện tại, kiểm tra port trước khi khởi động. Chỉ khởi động một server app và một server artboard; ghi command/PID/port/cwd. Kết thúc phải đóng tab và dừng process đã tạo. Không tự tạo worktree: chủ máy vừa yêu cầu dọn; chỉ dùng isolation mới nếu workflow yêu cầu và được chấp thuận.
- Reference chạy từ `apps/viderse/docs/design/v1` (yêu cầu đề xuất `npx serve apps/viderse/docs/design/v1`; lúc lập kế hoạch đã mở trực tiếp Main bằng static HTTP server). Không sửa artboard, kit.css hoặc tokens.css để làm reference khớp code.
- Credential chỉ ở kho được duyệt, không trong plan, fixture commit, URL, ảnh hay Playwright trace. Luồng tốn ảnh phải chạy trong tài khoản QA có hạn mức và xác định chi phí; không có thì ghi BLOCKED, không đánh dấu pass. Huỷ trước submit cũng phải chứng minh không có request ghi.

## 1. State manifest: không bỏ sót tweak
Trong PR0 tạo manifest kiểm kê tại `reports/artboard-state-matrix.json` từ `data-props` của các artboard liên quan. Đây là output kiểm kê, không thêm runtime contract.
Mỗi row: artboard, property, enum value/boolean true-false/numeric boundary hoặc giá trị mẫu hợp lệ do README chỉ định, prerequisite, app URL, cách seed qua API/DB QA, expected visible state, PR sở hữu, reference screenshot, app screenshot, control-ledger path, status và lý do blocked.
- Đi hết mọi giá trị enum/boolean của từng tweak. Không nhân chéo mù toàn bộ tổ hợp, nhưng phải có tổ hợp phụ thuộc hợp lệ: Main inspector/state/viewer; Channel tab/tplMenu/autoApprove; New source/step/template; mobile sheet/keyboard/error.
- Giá trị số/text liên tục: ghi domain, giá trị mặc định, min/max, rỗng và nhãn dài khi có ý nghĩa. Không tuyên bố đã đi hết vô hạn giá trị.
- Map trạng thái app qua dữ liệu thật; nếu API không thể biểu diễn tweak, ghi rõ gap và package/backend được phép bổ sung. Không giả trạng thái ready/building bằng CSS.
- Mobile viewer đợt45 là exclusion có tên và lý do, không phải dòng pass. Desktop Editor không bị tự mở rộng thành mobile Editor; V40 giữ đúng danh sách màn trong §6.2.
- State manifest được commit và đối chiếu với `artboards/README.md` cùng footer thực; README cũ không được thắng footer/Workflow.

## 2. Ảnh đối chiếu và layout
Cho mỗi row hợp lệ, chụp reference và code ở **1440×900** và **390×844**; với màn chỉ có một thiết kế responsive, dùng đúng artboard tương ứng và ghi not-applicable có nguồn cho phía kia, không phóng artboard desktop thành mẫu mobile.
- Tải đủ font và ảnh, chờ finite animations hoàn tất trước ảnh tĩnh. Không tắt animation trong lượt kiểm tra motion. Chỉ lượt geometry mới được settle thời gian.
- Lưu PNG reference, app và ảnh đặt cạnh nhau cùng tỷ lệ, trong `reports/pr-NN/{reference,app,comparison}/`; kèm viewport, DPR, font status, commit thật, seed và trạng thái trong JSON. Không ghi commit tương lai vào gap.
- So: thanh trên/nav, grid/cột, typography/token, khoảng cách, bo góc, icon baseline, avatar tròn không bị clipping, ảnh contain, primary hierarchy, CTA/footer, empty/loading/error/pending/disabled.
- Luật hình bắt buộc: màu/cỡ/bo góc chỉ qua token hiện có, không thêm màu; avatar luôn tròn; không thanh màu chỉ một cạnh; mỗi vùng tối đa một primary. Rà cả CSS/StyleX/Tailwind đang được render, không chỉ component mới.
- Đo không tràn ngang; bounding box lớp phủ nằm trong viewport; mọi touch target interactive trên mobile ≥44px. Kiểm thêm 360/430px, ranh 767/768px, scroll dài, safe-area và mở/đóng bàn phím. Bàn phím CSS không thay chứng cứ Safari thiết bị thật.
- Với Segmented: throttle font mới, đợi document.fonts.ready, đo indicator và nút chọn bằng getBoundingClientRect; sai số từng cạnh ≤2px sau font/resize/nhãn đổi/scroll. Không hard-code offset bù.
- Screenshot giống không thay kiểm hành vi; pixel khác do dữ liệu nguồn/thời gian phải giải thích và vẫn giữ layout/token giống.

## 3. Playwright: mọi control phải có oracle hành vi
Reuse `tests/pc-uikit/playwright.config.ts` và fixture có sẵn; bổ sung suite **mới** `tests/viderse-workflow/playwright.config.ts`, `interaction-audit.pw.ts`, `workflow-parity.pw.ts` vì đây là audit app đa màn, không ép test app vào smoke kit `__kit=1`.
- Từ từng state manifest, lấy mọi button, link và `[role=tab|radio|switch]` visible, kể cả menu/sheet vừa mở. Ghi một control ledger: accessible role/name, source screen, state, enabled/disabled + lý do, action, expected effect, observed effect, API/readback và pass/fail.
- Mỗi control enabled phải map oracle cụ thể: URL/focus đúng, đúng panel/ảnh/version, lựa chọn đổi và lưu qua reload, job được tạo một lần, trạng thái undo phục hồi, tải file đúng hoặc clipboard đọc lại. Chỉ count click, thấy spinner/toast, không throw hay mock được gọi **không đủ**.
- Disabled phải có lý do người dùng hiểu, không bị tính pass như đã thực hiện action. Dead control hoặc không có oracle = FAIL; không whitelist theo selector tuỳ tiện.
- Mutation đợi response rồi kiểm qua API/reload/UI; polling phải ổn định theo ID, không seed response thành công trong browser. Hai tab/requests cùng enqueue được kiểm dedup thật.
- Kiểm keyboard: N ngoài input/contenteditable mới mở Nhập nhanh; Esc đóng đúng lớp; ←/→, R/E/A trong viewer theo quyền và ngữ cảnh; không kích hoạt shortcut xuyên dialog/input.
- Mạng lỗi, mất quyền, quota, partial batch, duplicate: giữ dữ liệu và các mục chưa xong, có đường tiếp tục, không tạo job/thu phí trùng khi double click hoặc thử lại.
- Lưu HTML report, trace đã rà credential, console errors và API errors; report phải **0 control thiếu phản hồi**, không bỏ controls trong state khó seed.

## 4. Animation và feedback
- So UkMotion/kit motion token, không thêm duration/easing tuỳ hứng. Chạy cả normal và prefers-reduced-motion.
- Ghi trace/video cho popover/sheet/dialog, toast/undo, đổi tab/slider, tiến độ vẽ, ảnh mới cross-fade. Kiểm opening → visible → closing → unmounted, rapid-toggle, route change, focus return và scroll restore; không chỉ ảnh cuối.
- Progress/nhật ký dùng event thật, có ai đang làm/việc gì/kết quả bằng tiếng Việt; SSE/polling reconnect không nhân dòng hoặc báo xong giả. Nội dung nguồn và prompt thực gửi giữ nguyên ngôn ngữ gốc theo 06-copy.
- Khi mobile keyboard/visualViewport thay đổi: footer/dock không kẹt lơ lửng sau đóng bàn phím; test Safari thiết bị thật trước kết luận đã sửa bug iPhone, không gọi Chromium emulation là iPhone thật.

## 5. Lệnh có owner đã xác minh
Từ root `/Users/dat/Projects/verse`:
```sh
npm test -w apps/viderse/web -- <test-file-or-pattern-of-touched-module>
npm run typecheck -w apps/viderse/web
npm run build -w apps/viderse/web
npm run size -w apps/viderse/web
npm test -w platform/ui -- <changed-kit-test-pattern>
npm run typecheck -w platform/ui
go test ./apps/viderse/internal/<touched-package>
# Sau khi tạo suite trong PR0/PR1; không tuyên bố đã tồn tại hoặc đã chạy hiện nay:
npx --no-install playwright test --config tests/viderse-workflow/playwright.config.ts
```
Backend public contract hoặc migration đổi: chạy focused trước, sau đó `go test ./apps/viderse/...`, race các package queue/channels/postgres bị đổi với DB QA rỗng. Không dùng DB app hiện có làm TEST_DATABASE_URL; script/runner phải chứng minh isolation trước.
SQLC owner `apps/viderse/sqlc.yaml`: nếu đổi query/schema, chạy `sqlc generate -f apps/viderse/sqlc.yaml` với bản binary theo project; không sửa tay `postgres/gen`.
Không chạy toàn repo nhiều lần trong lúc agent đang sửa. Test output timeout/fail phải giữ nguyên trong report, không thu hẹp rồi gọi full pass.

### 5.1. Công cụ baseline đã có trong PR0
`node tests/viderse-workflow/artboard-state-manifest.mjs` tạo inventory; `--check` kiểm drift. Capture dùng **defaults + context + patch**, không bỏ ngữ cảnh cha của tweak.
```sh
# Env chỉ tới server loopback, fixture/cookie riêng ngoài repo; không dùng phiên production.
WORKFLOW_REVISION=<source-sha> PW_BROWSER_BINARY=<chrome-for-testing> \
  node tests/viderse-workflow/capture-baseline.mjs reference
WORKFLOW_REVISION=<source-sha> PW_BROWSER_BINARY=<chrome-for-testing> \
  WORKFLOW_QA_FIXTURE=<private-fixture.json> WORKFLOW_QA_COOKIE_FILE=<private-cookies.json> \
  node tests/viderse-workflow/capture-baseline.mjs app
```
Lọc board cần `WORKFLOW_EVIDENCE_DIR` khác thư mục canonical, không ghi đè báo cáo đầy đủ. Runner chỉ chụp/read API: không intercept response, không tự bấm việc tốn hạn mức; đây không phải full-control acceptance suite. Receipt có trạng thái running/completed/aborted và ảnh có timestamp, DPR, font, viewport, ngữ cảnh. Xem [baseline và giới hạn](reports/pr-00/baseline.md), [runtime trace index](reports/pr-00/capture-index.json).

## 6. Kiểm tính năng đã bỏ và hợp đồng cũ
- Rà UI production source theo các chữ “luân phiên” trong ngữ cảnh template, “Mức tự chủ”, “Vì sao?”, “Ghi chú đã học”, “YouTube Data API”, “Bản tin”. Phân biệt lịch sử docs, test chứng minh **không còn** UI, prompt nguyên văn cũ và chữ dùng nghĩa khác; không xoá dữ liệu cũ chỉ vì grep.
- Mỗi PR chứng minh các từ/tính năng thuộc gói mình đã mất; chưa tới gói thì liệt kê tồn đọng có PR sở hữu. PR cuối bắt buộc zero UI-visible match cho toàn bộ danh sách.
- Sau cutover V39: không caller/runtime route creation cũ, không request priority/imageWorkers/rescanHours; không xoá GET/PATCH/asset/editor API còn hợp lệ vì cùng prefix /api/clone.
- Xoá tests chỉ đóng đinh wording/source/wiring hoặc tính năng đã loại; thay contract tests cũ bằng hành vi mới có giá trị consumer. Không đổi expectation để che regression.

## 7. Báo cáo, gap và merge gate
Mỗi PR gửi: bảng màn → đã sửa/còn lệch; source/state matrix; ảnh đặt cạnh; control ledger + kết quả click; motion trace; command + exit code; câu hỏi số trong `11-hoi-dap.md`.
Cập nhật đúng dòng V36–V40 trong `10-gap-closure.md`: ngày, commit thật, phạm vi đã kiểm, chỗ lệch và lý do. PR nền/kiểm kê dùng record riêng, không tuyên bố gói sản phẩm hoàn thành.
PR0 chỉ kiểm kê, không cutover UI: chênh lệch code cũ phải ghi owner và pending target, không gắn nghiệm thu gói. Artifact chụp sai ngữ cảnh, còn loading, thiếu provenance hoặc so ảnh sai màn phải sửa trước khi merge kiểm kê; một ảnh route nền không thay cho overlay chưa mở.
Đang block provider/schema/tweak hoặc có câu hỏi chưa được trả lời = chưa nghiệm thu. Không gắn completed/deploy-full vì compile/build xanh. Mỗi PR merge xong và có evidence mới mở PR kế tiếp; release/production deployment cần bước cho phép riêng, không tự deploy từ phiên lập kế hoạch.
