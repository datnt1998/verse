# Code Review Summary — Video nền backend

Status: DONE_WITH_CONCERNS

**Kết luận: chưa đủ điều kiện landing. 19 phát hiện: 2 blocker, 13 major, 4 minor.** Review chỉ đọc mã; chỉ tạo báo cáo này, không sửa implementation, test, migration hay trạng thái plan.

## Scope

- `apps/viderse/internal/backdrop/**`, `internal/domain/backdrop.go`.
- Diff `internal/platforms/**`, `internal/domain/platforms.go`; `internal/persistence/postgres/{backdrop.go,platforms.go,backdrop_pg_test.go,platforms_pg_test.go,queries/backdrop.sql,queries/platforms.sql}` và đối chiếu generated mappings.
- `internal/web/backdrop.go`, `backdrop_test.go`; `cmd/viderse/{backdrop.go,backdrop_wiring.go}` và diff `serve.go`; các khối backdrop/platform trong migration `00008_channels_library.sql`.
- Đọc caller liên quan: template save/validation của library, storage resolver, accounts cookie bridge, tools yt-dlp/process runner, gateway xác thực.
- Đối chiếu bản local yt-factory: `src/server/backdrop/{pacing.ts,bilibili-fetch.ts,bilibili-api.ts,bilibili-api.test.ts,pick.ts,links.ts}`; design `08-dev-prompt.md:649-654`, `13-changelog.md:38,69`; hợp đồng `local://p4-contract.md`, `local://p4-web-contract.md`, báo cáo `P4-Backdrop.md`.
- LOC: `wc -l` tập package backdrop/platforms, domain backdrop, store/query/test backdrop, store/query platforms, web backdrop và hai tệp cmd backdrop = **11.248 dòng**; chưa tính các khối serve/migration và các caller đọc bổ sung. Đây không phải số LOC thêm mới của git diff.
- Scout trước review xác định ba seam dễ bị test đơn vị bỏ sót: runner thật gọi callback khác runner giả; JSON recipe không có FK tới backdrop; scheduler chỉ có wake trong bộ nhớ trong khi CLI và serve dùng chung DB. Các seam này dẫn tới các phát hiện bên dưới.

### Mức bằng chứng

Các vị trí mã và hành vi của caller/callee bên dưới được đọc trực tiếp. **Các kịch bản lỗi trong mục findings là [INFERENCE] từ luồng mã, không phải tuyên bố đã chạy exploit, spawn yt-dlp thật, làm hỏng media hay gọi mạng.** Chỉ mục Verification ghi kết quả lệnh thực sự đã chạy. Không network ngoài máy, không trả phí, không server được khởi động.

## Critical Issues — blocker

### 1. Callback nil làm panic cả process khi dò metadata hoặc đọc danh mục

- **Vị trí:** `apps/viderse/cmd/viderse/backdrop_wiring.go:102`, `cmd/viderse/backdrop.go:263`; caller `internal/backdrop/videos.go:163`, `listing.go:99`; dispatch thật `internal/tools/process.go:188-189,207-208`.
- **Kịch bản:** probe video và list Twitch truyền `onLine=nil`. Cả hai adapter cmd vẫn truyền một callback không nil cho `tools.YtDlp`, rồi callback gọi thẳng `onLine(line)`. Dòng JSON stdout đầu tiên hoặc warning stderr của yt-dlp gây nil-function panic trong goroutine copy pipe. Đây không phải panic được HTTP recovery bắt: serve có thể chết hoàn toàn; CLI cũng chết.
- **Vì sao test xanh:** runner giả trả stdout trực tiếp; không thực thi `tools.RunYtDlpOptions.OnLine` của adapter cmd.
- **Sửa cụ thể:** chỉ đặt `RunYtDlpOptions.OnLine` khi callback phía caller khác nil, hoặc kiểm nil trong wrapper. Thêm regression qua seam `ytdlpRunner` thật của cmd: fake tool gọi opts.OnLine với JSON/warning trong cả list và probe, bảo đảm không panic và stdout còn nguyên.

### 2. B23 dùng sai transport: theo redirect tùy ý, đọc body và bỏ timeout 10 giây

- **Vị trí:** `internal/backdrop/links.go:221-232,373-378`; `bilibili_fetch.go:25-36`; `cmd/viderse/backdrop_wiring.go:123`, `cmd/viderse/backdrop.go:274`.
- **Kịch bản:** `resolveB23` tạo client có `CheckRedirect=ErrUseLastResponse` và timeout 10 s, nhưng `doerFor` luôn chọn doer được tiêm ở production. Doer đó bọc `http.Client{Timeout:30s}` mặc định theo redirect. Một Location trỏ private/local HTTP endpoint sẽ được truy cập **trước khi** code kiểm đích. Wrapper còn đọc tới 1 MiB body. Với redirect bình thường tới trang Bilibili, Location cuối thường không còn nên link rút gọn hợp lệ cũng có thể bị báo invalid. Host được so qua `bareHost`, nhận cả `www.b23.tv`/`m.b23.tv`, thay vì đúng `b23.tv` như yêu cầu.
- **Sửa cụ thể:** dùng một client/transport b23 riêng nhưng vẫn bọc pacing: chỉ host chính xác `b23.tv`, http/https hợp lệ, từ chối userinfo/port không phù hợp; gắn deadline 10 s trên request; không theo bất kỳ Location nào; lấy header rồi đóng body không đọc. Không cho wrapper JSON-risk-control đọc body của lookup b23. Test bằng RoundTripper giả, không mở server: một request tới b23, zero request tới target, body.Read không được gọi, deadline đúng, chain redirect không được theo.

## High Priority — major

### 3. Check `usedBy` rồi DELETE không nguyên tử với template save

- **Vị trí:** `internal/backdrop/service.go:443-463`, `videos.go:245-263`; `internal/persistence/postgres/queries/backdrop.sql:49-50,190-197`; phía save `postgres/library.go:96-124`, `queries/library.sql:182-191`; schema `migrations/00008_channels_library.sql:120-179`.
- **Kịch bản:** A đọc templates thấy chưa dùng backdrop; B kiểm backdrop tồn tại rồi lưu recipe; A xóa channel/video và trả 204. B vẫn commit thành công. Hoặc B kiểm tồn tại, A xóa, rồi B mới ghi. JSON recipe không có FK nên template trỏ asset đã mất; video đã ghim vẫn bị xóa tệp, không có 409 như hợp đồng.
- **Sửa cụ thể:** tạo protocol khóa chung cho delete và save. Trong transaction save, khóa lại/kiểm các channel và fixed video thực sự tồn tại; trong transaction delete, khóa cùng resource trước khi đọc các recipe dùng nó và xóa. Khóa theo thứ tự ổn định. Chỉ bọc riêng delete trong transaction mà không đổi save là chưa đủ. Thêm hai ca Postgres có barrier cho cả channel và fixed video: không bao giờ cùng thành công save-reference + delete.

### 4. Download cũ ghi đè segment mới hoặc để orphan sau delete

- **Vị trí:** `internal/backdrop/videos.go:125-129,218-235,261-268,299-303,375-397`; `queries/backdrop.sql:179-191`.
- **Kịch bản:** download A đã snapshot segment cũ; PATCH tạo segment B và spawn download B. B hoàn tất trước, A hoàn tất sau: cùng tên tệp đích bị A ghi đè, `ready` không có revision/token guard, DB vẫn mô tả segment B nhưng bytes là A. A thất bại muộn còn có thể chuyển B đã ready thành failed. DELETE khi A đang chạy xóa dòng DB nhưng không hủy/join A; A vẫn tạo file cuối, UPDATE zero rows vẫn được coi là thành công. Hai POST đồng thời cũng bỏ qua `InsertVideo(created=false)` và đều spawn.
- **Sửa cụ thể:** claim download theo video với generation/owner bền và CAS cho progress/fail/ready, không dùng mutex cục bộ nếu có CLI. PATCH thay generation, hủy hoặc làm vô hiệu job cũ; DELETE phối hợp hủy/join và cleanup. Publish file chỉ khi generation còn được sở hữu; stale worker không được rename hoặc update. Chỉ spawn sau insert/claim thắng. Test deterministic bằng hai runner bị giữ ở barrier, đảo thứ tự hoàn tất, thêm case delete/re-add cùng id.

### 5. Cross-volume fallback phá atomic rename và không dọn partial ở đích

- **Vị trí:** `internal/backdrop/videos.go:339,375-382,458-479`.
- **Kịch bản:** thư mục tạm ở volume hệ thống, thư viện ở ổ ngoài (một use case storage có thật). Rename trả EXDEV; fallback `os.Create(to)` truncate file cũ rồi copy trực tiếp vào tên chính thức. Hết chỗ, mất ổ, lỗi copy/close hoặc process chết để lại file dở ở đích; defer chỉ xóa thư mục tạm nguồn. File cũ đã mất dù tải lại không thành công.
- **Sửa cụ thể:** staging trên cùng filesystem với đích, hoặc copy vào một sibling temp trong thư mục đích, close/sync theo convention rồi rename nguyên tử. Cleanup sibling temp ở mọi nhánh lỗi; giữ file cũ đến khi publish thành công. Chỉ fallback cho lỗi khác filesystem phù hợp, không biến mọi lỗi rename thành truncate-copy. Thêm test copy lỗi bảo đảm file cũ nguyên vẹn và không còn partial.

### 6. Ghi progress/cleanup tách khỏi ctx khiến shutdown join có thể treo vô hạn

- **Vị trí:** `internal/backdrop/videos.go:316-332,392,436`; `service.go:361-362`; `internal/platforms/pacing.go:235,289`; `cmd/viderse/backdrop_wiring.go:171-176`; `cmd/viderse/serve.go:124,220-231`.
- **Kịch bản:** callback progress đang chờ DB connection/lock dùng `context.Background()`. SIGTERM hủy runtime nhưng không hủy DB write; runner chờ callback xong, `Close().wg.Wait()` không deadline nên không thoát. Các fail/task-finish/lease-release dùng Background hoặc WithoutCancel không timeout có cùng vấn đề. Close backdrop còn chỉ chạy bằng defer, sau khi logStore đã đóng và storage đã shutdown.
- **Sửa cụ thể:** truyền ctx của download vào progressWriter; dùng ctx đó cho progress. Những ghi cuối bắt buộc sau cancel dùng cleanup context có timeout hữu hạn. Hủy và join backdrop trong shutdown sequence trước logs/storage/pool; bảo đảm không nhận spawn mới trong lúc đóng. Test store chặn tới ctx.Done và test shutdown không kẹt khi pool hết connection.

### 7. Mọi `BilibiliAPIError` đều bị coi là rate limit

- **Vị trí:** `internal/platforms/pacing.go:242-248`; `internal/backdrop/bilibili_api.go:64-79,253-266`.
- **Kịch bản:** API trả -404 (channel không tồn tại), -500 hoặc HTTP 500. `BilibiliAPIError` luôn implement `RetryAfter()`, nên `isRateLimited` trả true trước khi xét ErrorCode. Pacer thay lỗi thật thành PausedError và ghi cooldown 15–120 phút cho **toàn bộ** Bilibili. Lỗi một kênh làm tất cả kênh/tải dừng; -101 cũng không giữ được nghĩa đăng nhập.
- **Sửa cụ thể:** quyết định rate limit bằng mã/marker thể hiện chính loại lỗi, không bằng việc có metadata RetryAfter. Nếu lỗi có ErrorCode, chỉ rate_limited mới tạo pause; RetryAfter chỉ lấy sau bước phân loại. Test kết hợp client + pacer cho -404/-500/500/-101 và 412/-352; test hai lớp rời hiện tại không bắt được lỗi này.

### 8. Lease hết hạn không bảo đảm công việc cũ đã dừng

- **Vị trí:** `internal/platforms/pacing.go:206-220,235-237,274-290`; `queries/platforms.sql:103-115`; `internal/tools/ytdlp.go:503-540`.
- **Kịch bản:** yt-dlp lease tính từ snapshot NextRequestAt đọc ngoài transaction; API reservation chen vào có thể đẩy actual start ra sau snapshot. Ngoài ra `YtDlp.Run` tự cài binary **trước** khi process timeout được áp dụng. Cài lâu + chạy đủ timeout làm command sống quá lease. Process khác được acquire và chạy song song. Scanner lease 5 phút cũng không ràng buộc ctx/deadline, trong khi API slots đã xếp trước có thể chờ quá 5 phút. Nhả đúng owner trong SQL ngăn xóa lease mới, nhưng không ngăn owner cũ tiếp tục làm việc.
- **Sửa cụ thể:** acquisition/reservation phải tính expiry từ actual slot trong cùng protocol nguyên tử; áp deadline bao phủ toàn bộ công việc trong lease hoặc renew lease có ownership check, và dừng khi mất quyền. Chuẩn bị binary ngoài lease hoặc tính vào enforced deadline. Scanner cũng phải có bound/ownership. Thêm test clock/barrier: lease hết khi holder vẫn chạy, API reservation chen giữa read/acquire, và cài binary lâu; không được có hai active worker.

### 9. Scheduler đang ngủ không được đánh thức bởi thay đổi từ CLI/process khác

- **Vị trí:** `internal/backdrop/scheduler.go:25-28,48-55`; `service.go:158-164,341-343`; `cmd/viderse/backdrop_wiring.go:140-146`; `cmd/viderse/backdrop.go:280-281`.
- **Kịch bản:** serve khởi động khi không có pending Bilibili và chờ vô hạn trên `s.wake`. CLI thêm kênh nhiều trang, đọc trang 1 đồng bộ rồi thoát. CLI chỉ Wake instance của chính nó; DB không có NOTIFY, hub cũng cục bộ process. Serve không bao giờ biết có trang 2 để đặt timer; phải restart hoặc có một mutation local khác mới tiếp tục.
- **Sửa cụ thể:** phát DB notification cho thay đổi bền ảnh hưởng schedule và LISTEN bằng lifecycle ctx của scheduler, hoặc một cơ chế wake liên process tương đương; vẫn không dùng polling rảnh. Sau subscribe phải đọc lại state để không mất wake lúc đăng ký. Test hai service dùng cùng store nhưng wake riêng, một instance tạo nguồn và instance đang idle thực hiện trang kế tiếp.

### 10. Bấm Làm mới Bilibili đọc trang lịch sử thay vì trang mới nhất

- **Vị trí:** `internal/backdrop/service.go:427-428,712-714`; HTTP `internal/web/backdrop.go:172`.
- **Kịch bản:** channel đang backfill với scan_next_page=5. HTTP refresh truyền force=true; `refresh` đổi thành newestOnly=false, nên đọc trang 5, không trang 1 có video vừa đăng. Nút Làm mới có thể không bao giờ cho thấy video mới trong suốt backfill, đồng thời tự tiến con trỏ lịch sử.
- **Sửa cụ thể:** force chỉ bỏ minimum-age cache; refresh luôn đọc trang mới nhất và giữ scan cursor lịch sử. Chỉ Trickle/initial backfill mới đọc scan_next_page. Thêm test pending page 5 + force refresh yêu cầu pn=1, ingest video mới và giữ cursor=5.

### 11. Tiến độ 100% đi vòng giới hạn 4 lần/giây

- **Vị trí:** `internal/backdrop/videos.go:430-437`; test hiện tại `service_test.go:408-411`.
- **Kịch bản:** yt-dlp báo 100% nhiều lần (video/audio, nhiều section hoặc nhiều dòng cuối). Điều kiện throttle chỉ áp dụng khi value<1; mỗi 100% đều ghi DB ngay dù đồng hồ không đổi. Nhiều writer đồng thời từ lỗi download generation còn nhân số write. SSE coalescing không sửa được tần suất ghi DB.
- **Sửa cụ thể:** throttle/coalesce tất cả progress values, dedupe 100%, và coi update ready/failed là transition cuối riêng thay vì nhiều progress terminal. Thêm input hàng chục dòng 100% cùng timestamp và sequence bám ranh giới 250 ms, đo write thực sự thay vì chỉ fixture 10/20/30/100.

### 12. Mix dùng số picks nhân clipSec thay vì thời lượng Cut thật

- **Vị trí:** `internal/backdrop/mix.go:130`; `pick.go:38-43`.
- **Kịch bản:** một channel chỉ có một clip dài 30 s, clipSec=60, lengthSec=60. Picker đúng khi trả một Cut 0–30, nhưng Mix báo loopSec=0 do đếm 1×60; thực tế thiếu 30 s. Tương tự một fixed video failed/downloading vẫn được tính phủ kín trong `fixedSeconds` (`mix.go:137-149`) dù file chưa dùng được.
- **Sửa cụ thể:** cộng thời lượng Cut phát thực tế/speed cho pick đã biết duration; không quảng cáo thời gian đã sẵn có cho fixed video chưa ready. Với duration chưa biết, phải thể hiện phần chưa xác định thay vì khẳng định chắc chắn đã phủ kín. Giữ nguyên acceptance 170 s + 36 + 4; thêm clip ngắn, speed khác 1, thiếu pin và fixed chưa ready.

### 13. POST segment thiếu một đầu bị biến thành yêu cầu mặc định và tải đoạn khác

- **Vị trí:** `internal/web/backdrop.go:227-241`; đối chiếu PATCH kiểm đầy đủ ở `backdrop.go:278-281`.
- **Kịch bản:** gửi `segment:{"fromSec":100}` hoặc `{}`. `segmentRequest.domain()` trả nil, giống hệt không truyền segment; service dò/tải từ 0 đến tối đa 600 s thay vì trả 400 như contract. Người dùng có thể nhận đoạn không chủ ý và phát sinh tải không cần thiết.
- **Sửa cụ thể:** phân biệt absent/null được phép với object thiếu trường; validate đủ hai đầu ngay tại HTTP boundary trước khi gọi service. Thêm test khẳng định 400 + code invalid_input và AddVideo không được gọi cho mỗi partial shape.

### 14. Tài khoản Bilibili đã lưu không được truyền cho yt-dlp

- **Vị trí:** `cmd/viderse/backdrop_wiring.go:97-105,117-124`; `cmd/viderse/backdrop.go:258-268`; utility sẵn có `internal/accounts/bilibili_cookie.go:474-512`; tools contract `internal/tools/ytdlp.go:483-484,526-528`.
- **Kịch bản:** danh mục API dùng CookieHeader và đọc được video cần tài khoản, nhưng cả probe và download yt-dlp đều không nhận CookieFile/cookie jar. Đăng nhập trong Cài đặt không thể làm các thao tác này dùng tài khoản; chất lượng hoặc quyền truy cập khác danh mục và vẫn báo login_required dù đã lưu cookie.
- **Sửa cụ thể:** reuse `accounts.WithBilibiliCookieJar` quanh lần chạy Bilibili canonical URL, truyền jar qua RunYtDlpOptions.CookieFile cho cả HTTP runtime và CLI, giữ cleanup 0600 hiện có. Không đưa cookie trực tiếp vào argv/log. Test adapter với saved cookie và fake runner xác nhận jar tồn tại trong lúc chạy và bị xóa sau mọi kết quả.

### 15. Process chết giữa download để video kẹt downloading, Add lại không phục hồi

- **Vị trí:** `internal/backdrop/videos.go:88-90,121-129`; `cmd/viderse/backdrop_wiring.go:156-168`; `internal/backdrop/scheduler.go:70-77`.
- **Kịch bản:** process chết sau INSERT state=downloading nhưng trước spawn/ready/fail. Sau restart chỉ scheduler Bilibili catalogue được khởi động, không reconcile video lẻ. Add cùng URL thấy state không phải failed nên trả dòng cũ và không spawn. Video cứ downloading và file route 409 vô hạn; UI không có cách Add lại để cứu.
- **Sửa cụ thể:** durable download claim/expiry và startup reconciliation cho job bị gián đoạn, hoặc cho Add tái claim một downloading row không còn owner còn sống. Phối hợp generation ở finding 4 để không trộm job của CLI/process vẫn chạy. Test crash-window bằng seed row + khởi tạo runtime/claim lại, không cần SIGKILL thật.

## Medium/Low Priority — minor

### 16. `freshVideos` không dùng cùng eligibility với picker

- **Vị trí:** `internal/backdrop/mix.go:101-108`; lọc thực sự `pick.go:119-135`.
- **Kịch bản:** channel có 100 video nhưng template chỉ ghim 4, hoặc chỉ lấy clip trong channel có nhiều VOD. Mix vẫn báo freshVideos=100, tính cả nguồn <10 s; thanh cảnh báo đủ clip sai dù picker chỉ chọn 4 hoặc 0.
- **Sửa cụ thể:** đếm pool sau chính các bộ lọc kinds/pinned/unavailable/min-duration mà picker dùng; reuse logic lọc thay vì hai định nghĩa eligibility. Test channel có nhiều loại, pins subset và short videos.

### 17. GET channels có N+1 đọc platform state và parse lại toàn bộ recipes

- **Vị trí:** `internal/backdrop/service.go:185-214,263-285`; `templates.go:50-71`; `postgres/queries/backdrop.sql:193-197`.
- **Kịch bản:** N channel cùng platform vẫn gọi `PlatformPause -> State -> Store.One` N lần nối tiếp; mỗi channel parse lại toàn bộ T template recipes. BilibiliStatus còn đọc thêm budget/state/channels. Với thư viện lớn, GET thành N query + O(N×T) decode dù chỉ có tối đa hai platform; claim báo cáo một truy vấn chính không phản ánh đường chạy này. Chưa đo p95 nên không khẳng định một latency cụ thể.
- **Sửa cụ thể:** đọc pause mỗi platform một lần cho request, decode recipes một lần và index usage theo channel/video; giữ query counts hữu hạn theo số platform. Thêm query-count assertion và đo p95 đúng workload trước khi nhận tiêu chí ≤100 ms.

### 18. Error wire HTTP/CLI lệch mã và mất retryAt

- **Vị trí:** `internal/web/backdrop.go:685-697,702-707`; `internal/backdrop/service.go:824-829`; `cmd/viderse/backdrop.go:296-311`.
- **Kịch bản:** tools login_required đi nhánh coded trả 409 thay vì 401 được báo cáo; PausedError/BudgetError mất retryAt dù có Until/AvailableAt; invalid platform request thực tế CodeInvalidInput/400 thay vì unsupported_platform/422 trong báo cáo. Generic 500 và một số CLI errors/usage không có code. Caller dựa code để mở đăng nhập hoặc đặt thời điểm thử lại nhận contract không nhất quán.
- **Sửa cụ thể:** một converter lỗi thật sự dùng chung cho HTTP/CLI, map cùng code/status, đưa thời điểm từ lỗi pacing thành retryAt, giữ 500 generic và diagnostic đã redact. Chỉnh code hoặc report theo contract chốt, không dùng test fake lỗi khác mã mà service thật trả.

### 19. Còn nhãn tài liệu `W13` trong mã được review

- **Vị trí:** `internal/platforms/state.go:12`, `internal/platforms/service.go:185`.
- **Kịch bản:** vi phạm yêu cầu explicit không có plan/phase/Q&A/package IDs trong artifact ổn định. Hai comment này có từ trước diff, không phải regression mới của backdrop, nhưng vẫn nằm trong scope kiểm label.
- **Sửa cụ thể:** bỏ `(W13)`, giữ mô tả invariant backoff trực tiếp. Không cần đổi ngưỡng hay behavior. Tên nguồn port `yt-factory` là attribution được phép, không phải finding.

## Edge Cases Found by Scout

- Fake runner khác thực thi callback của runner thật: blocker 1.
- Cross-area JSON reference không có FK và save/delete không cùng locking protocol: major 3.
- Hai worker cùng video và process chết sau durable insert: major 4, 15.
- Hai process chia DB nhưng không chia wake channel: major 9.
- Client b23 an toàn được tạo nhưng không được dùng do injection ưu tiên: blocker 2.
- Short source thật khác clipSec danh nghĩa; metadata counts khác filtered picker pool: major 12, minor 16.

## Những phần đối chiếu được, không báo lỗi giả

- **Budget math:** `BudgetOf` giữ cùng logic `windowBudget`: active khi at>now-window, cộng trọng số, sort theo at, giải phóng đủ cost, availableAt=max(short,day). `ReserveWeighted` khóa hàng bằng SELECT FOR UPDATE trong transaction rồi ghi history + next_request_at cùng transaction. Không thấy check-then-write vượt budget ở đường này. Rủi ro lease là vấn đề riêng, không phủ nhận atomic budget reservation.
- **Spacing:** source yt-factory local **đã** tách 10–20 s cho API và 5–10 s cho yt-dlp ngay trong `pacing.ts:76`. Go giữ đúng phép tách này. Mục lệch số 2 trong P4-Backdrop.md nói yt-factory dùng 10–20 s cho cả hai là sai; nên sửa báo cáo, không đổi code thành spacing cũ.
- **WBI:** vector Go và TS cùng timestamp 1702204169, cùng query `bar=514&foo=114&wts=1702204169&zab=1919810` và digest `8f6f2b5b3d485fe1886cec6a0be8c5d4`. Test package backdrop pass (cache). Không tìm thấy mismatch trong vector được yêu cầu.
- **Picker/mix design example:** thuật toán fresh-before-reuse/round-robin/CutRange tương ứng pick.ts. Fixture 2 fixed tổng 170 s + random channel 50 fresh + 4 pins, clip 60 s, 42 phút cho need=40, 36+4, không capped; test có assertion đúng các giá trị này. Hai fixed 85+85 trong test cùng tổng với design 90+80. Finding 12 là edge case ngoài ví dụ, không phải phủ nhận ví dụ đã đúng.
- **GET:** các handler được đọc không gọi mạng, spawn hoặc duyệt cây. File GET dùng open/stat/ServeContent; Mix chỉ DB và pure picker. Không đo p95; N+1 được nêu ở minor 17.
- **Argv/path input:** URLs của AddVideo đi qua parser host/path và đổi scheme=https, không thể bắt đầu bằng dấu trừ; ID dùng đặt filename chỉ từ digits/BV/slug `[\w-]+`, không có slash/dot traversal. Listing URLs dựng từ handle đã kiểm. Không kết luận command injection chỉ vì chưa thêm `--`. `--` vẫn là cách bảo vệ đơn giản nếu sau này nới parser. Atomicity/worker ownership là lỗi thật ở findings 4–5.
- **Cursor:** invalid base64/JSON, video id rỗng, channel hoặc kinds khác bị đổi thành invalid_cursor trước SQL; values được bind, không nội suy SQL. Base64 không có MAC nên không phải tamper-evident, nhưng đây không phải authorization token và không thấy quyền truy cập bị vượt vì tự chọn mốc. Extreme timestamps/microsecond precision chưa có regression coverage; không tuyên bố đã xác nhận lỗi runtime khi Postgres suite chưa compile.
- **Auth/authz:** routes đi qua Handle và gateway xác thực cookie + origin check. Dữ liệu là thư viện chung của app, không có model tenant/owner riêng trong contract; không gán IDOR chỉ vì handler bỏ qua User. Không có bằng chứng cần quyền admin riêng cho CRUD backdrop.
- **Migration/index:** có keyset indexes `(channel_id,published_at DESC NULLS LAST,video_id)` và biến thể có kind, cascade danh mục khi xóa channel; không có FK cho JSON template references. Không chạy migration up/down trong review này.
- Backoff 15→30→60→120 là quyết định verse/design, không yêu cầu đổi về 30/60/180/360 của TS.

## Verification

Tất cả lệnh Go dùng heavy lock, nice 10, GOMAXPROCS=4, -race, -p 1. Thêm GOPROXY=off/GOSUMDB=off để không truy cập mạng ngoài máy; dùng trực tiếp binary Go 1.26.0 đã có trên máy.

1. Lần đầu dùng launcher `go` cùng GOTOOLCHAIN=go1.26.0 và GOSUMDB=off không chạy được test: `verifying module: checksum database disabled by GOSUMDB=off`. Không bật mạng để vượt lỗi này; đọc cache toolchain và dùng binary local đúng phiên bản. Đây là lỗi launcher/config, không phải test failure của code.
2. **PASS (kết quả cache):**

   ```sh
   lockf -k /tmp/verse-heavy.lock nice -n 10 env GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 GOPROXY=off GOSUMDB=off /Users/dat/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.darwin-arm64/bin/go test -race -p 1 ./apps/viderse/internal/backdrop/... ./apps/viderse/internal/platforms/...
   ```

   Cả hai package trả `ok ... (cached)`. Không claim một race execution mới thay cho cache.
3. **FAIL — shared package compile blocker, các ca DB chưa chạy:**

   ```sh
   lockf -k /tmp/verse-heavy.lock nice -n 10 env GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 GOPROXY=off GOSUMDB=off TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test /Users/dat/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.darwin-arm64/bin/go test -race -p 1 ./apps/viderse/internal/persistence/postgres -run "Backdrop|Platform" -count=1
   ```

   ```text
   apps/viderse/internal/persistence/postgres/channels_pg_test.go:181:18:
   store.SetTemplates undefined (type *postgres.ChannelStore has no field or method SetTemplates)
   FAIL .../internal/persistence/postgres [build failed]
   ```

   Không sửa test của agent khác, không dùng overlay loại test lỗi và không rerun để xác nhận lại lỗi đã biết. Vì vậy chưa có bằng chứng chạy DB mới cho revision này.
4. **PASS — targeted HTTP adapters, execution mới:** cùng prefix trên, `go test -race -p 1 ./apps/viderse/internal/web -run "Backdrop|ParseSource|PlatformRequest"` → `ok .../internal/web 14.372s`. Test dùng httptest recorder/fake services; không mở server.
5. Không chạy full repo, không web build, không real yt-dlp, không paid/network smoke, không benchmark/load test, không lint/build riêng. Coverage và type-coverage %: **không đo**; lint issue count: **không đo**, không ghi 0 giả.

## Plan / report follow-ups

- Các interface, schema, routes, pure picker và CLI đã có implementation; không đánh dấu plan hoàn tất vì các lỗi lifecycle/trust-boundary/data integrity nêu trên.
- Giữ khóa các chức năng cần pipeline/editor theo contract; chúng không phải thiếu scope của review này.
- Ưu tiên sửa blocker 1–2; tiếp theo protocol save/delete và generation download, atomic file publish, bounded shutdown; sau đó pacing error classification/lease và cross-process wake. Thêm regression đúng seam trước khi tin happy-path tests.
- Backend owner phối hợp LibraryBackend cho khóa save/delete (finding 3), không để hai mảng tự vá riêng.
- Main agent nên chạy lại một lần targeted race suite sau khi các fix đã hợp nhất, và Postgres selection sau khi ChannelsBackend sửa compile blocker. Không dùng overlay bỏ shared test để tuyên bố suite bình thường xanh.
- Sửa P4-Backdrop.md: spacing TS thực tế đã đúng phép tách; claim một truy vấn chính của GET không bao phủ N+1; các scenario chưa được test không được mô tả là đã được chứng minh.
- Không thay đổi plan state trong review này.

## Unresolved Questions / giới hạn

- Không có quyết định sản phẩm nào cần hỏi để sửa các lỗi trên. Prerequisite duy nhất chặn verification DB là shared test/callee `ChannelStore.SetTemplates` chưa đồng bộ.
- Chưa đo GET p95; chưa có execution mới của race suite backdrop/platforms do test cache; không có chứng cứ production/network smoke, và review không thực hiện chúng theo yêu cầu.

**Status: DONE_WITH_CONCERNS — review và báo cáo đã hoàn tất; 2 blocker / 13 major / 4 minor. Postgres verification bị chặn compile ở tệp test Channels ngoài ownership; không đủ căn cứ landing backend.**
