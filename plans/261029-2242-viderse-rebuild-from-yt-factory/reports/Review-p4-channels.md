# Code Review Summary — Owner channels

Ngày: 2026-10-31. Chế độ read-only: không sửa code, test, migration hay trạng thái plan; chỉ tạo báo cáo này.

## Scope

- Working tree chưa commit, gồm cả tệp untracked: `internal/channels/**`, `internal/domain/channels.go`, `internal/persistence/postgres/{channels.go,channels_pg_test.go,queries/channels.sql}`, `internal/web/channels{,_test}.go`, `cmd/viderse/{channels_wiring.go,channel.go}`, khối `owner_channels*` của migration `00008_channels_library.sql`.
- 6.479 dòng Go/SQL trong các tệp được giao, cộng khối migration liên quan (không phải số dòng diff của cả repo).
- Đọc contract `local://p4-contract.md`, hai báo cáo `P4-Channels.md`/`P4-Library.md`, plan `phase-04-channels-library.md`. Đối chiếu `yt-factory/src/server/channels/branding.ts`, `branding.test.ts` và encoder `src/server/clone/webp.ts`.
- Scout trước review: lần theo HTTP → service → store/SQL, activity thật, vòng đời serve, storage resolver, provider ảnh thật và test doubles. Điểm rủi ro nổi bật: task được nhận lại theo object, snapshot template trước transaction, COUNT nguồn trước khóa hàng kênh, recovery chỉ chạy một lần, provider không bảo đảm pixel theo Request.Width/Height.
- Findings bên dưới dựa trên source và các interleaving cụ thể; **không** khẳng định đã chạy các regression scenario mới. Không thêm test trong đợt read-only.

## Overall assessment

**Chưa nên kết luận DONE / production-ready.** Lease/publish có fence ở SQL, nhưng recovery bỏ sót restart nhanh; activity có thể bị request thua lease xóa; các thao tác nguồn/template chưa serialize đúng; file publication thiếu bảo đảm canvas và containment theo realpath. Các test hiện có đều xanh nhưng không bao phủ những biên này.

## Blocker

Không có finding được xếp blocker. Có **8 major** cần xử lý trước khi landing và **6 minor** có bằng chứng cụ thể.

## Major

### 1. Restart trước khi lease hết hạn khiến kênh kẹt generating vô thời hạn

- **Vị trí:** `internal/channels/channels.go:295-304`; `internal/persistence/postgres/queries/channels.sql:208-224`; `internal/web/channels.go:191-209`.
- **Scenario:** process chết ngay sau heartbeat (TTL 60 giây), app restart sau 5 giây. `Start` gọi `MarkInterruptedIdentities` đúng một lần; SQL chỉ đánh dấu lease đã hết hạn nên bỏ qua dòng này. Khi 55 giây còn lại trôi qua không có timer/reconcile nào chạy lại, GET vẫn ánh xạ `identity_state=running` thành `generating` mãi. Restart một process khác trong khi một owner còn sống cũng có cùng khoảng hở nếu owner đó chết sau startup.
- **Tác động:** không đạt phục hồi crash sang `interrupted`; UI hiện một việc không còn chạy và không giải thích lý do. Không có auto redraw trong code recovery, nhưng trạng thái phục hồi không hoàn chỉnh.
- **Sửa:** sau startup, hẹn một lần tới hạn lease đang tồn tại và tiếp tục reconcile theo mốc cần thiết, không idle polling, không gọi Drawer. SQL vẫn phải fence bằng lease hết hạn để không chạm owner sống. Thêm test restart khi lease còn hạn, tiến đồng hồ qua hạn, mong `failed/interrupted` và 0 lần vẽ.

### 2. Request thua lease xóa activity của run đang thắng; activity cũng không được fence khi mất lease

- **Vị trí:** `internal/channels/identity.go:216-238,492-511`; đối chiếu `internal/activity/activity.go:138-183,209-219` và `internal/persistence/postgres/queries/activity.sql:5-9`.
- **Scenario:** run A đã mở task T và giữ lease. Request B cùng kênh gọi `startIdentityTask` **trước** khi giành lease. Activity.Start không tạo task riêng: cùng kind + object JSON thì adopt T. B không giành được lease rồi gọi `Finish(T)`, xóa task của A. A vẫn đang tiêu thời gian/ảnh nhưng không còn dòng tiến độ. Sau lease takeover, run cũ gọi `failIdentity` cũng Update task vô điều kiện dù FailIdentity SQL đã trả false; nếu task được run mới nhận lại thì owner cũ có thể đánh dấu task mới failed.
- **Tác động:** API 409 có side effect phá trạng thái run hợp lệ; task/status/notification không còn phản ánh runner thật. Test fakeTasks.Start luôn cấp ID mới nên che lỗi contract với activity thật.
- **Sửa:** claim lease trước khi tạo/adopt task, rồi gắn task ID bằng update có token fence (hoặc transaction tương đương). Chỉ owner hiện tại được đổi/kết thúc task; không Finish một task được adopt chỉ vì acquire thất bại. Test hai service dùng activity thật, giữ A ở model gate, gọi B, xác nhận task A còn nguyên; thêm takeover + late failure.

### 3. Rotation template có lost update và có thể tồn tại hai template cùng position

- **Vị trí:** `internal/channels/channels.go:689-699,743-763`; `internal/persistence/postgres/channels.go:419-464`; migration `00008_channels_library.sql:107-115`.
- **Scenario:** hai tab/process cùng đọc danh sách `[A]`, lần lượt add B và C. Nếu store call thứ hai bắt đầu sau commit thứ nhất, snapshot `[A,C]` xóa B vừa thêm. Nếu cả hai store transaction đọc trước khi đối phương commit, mỗi bên giữ tập riêng, cùng chèn B/C ở position 1; schema không UNIQUE(owner_channel_id, position), nên có thể commit `[A@0,B@1,C@1]`. `SetAutoPickTouch` chỉ khóa ở cuối, không bảo vệ read/modify/write.
- **Tác động:** mất thao tác người dùng hoặc thứ tự không xác định; không bảo đảm position liền mạch. Báo cáo implementer thừa nhận không khóa vì một người dùng, nhưng một người vẫn có nhiều tab và HTTP/CLI cùng chạy.
- **Sửa:** đưa thao tác add/remove/move vào transaction khóa hàng kênh **trước khi đọc** danh sách; hoặc CAS revision và trả 409 khi stale. Khóa chỉ bên trong SetTemplates sau khi service đã tính IDs từ snapshot cũ là chưa đủ. Có thể thêm ràng buộc position phù hợp với renumber transaction. Test hai kết nối có gate, bao gồm add/add và move/remove.

### 4. Bỏ đồng thời hai nguồn cuối có thể để auto_pick=true khi không còn nguồn

- **Vị trí:** `internal/persistence/postgres/channels.go:384-411`.
- **Scenario:** kênh có A/B, auto_pick=true. T1 xóa A; T2 xóa B. Trước khi commit, T1 COUNT thấy B chưa commit xóa, T2 COUNT thấy A chưa commit xóa; cả hai đều thấy 1. Sau đó hai UPDATE touch hàng kênh serialize, nhưng không COUNT lại. Cả hai trả paused=false, commit xong không còn nguồn mà auto_pick vẫn bật.
- **Tác động:** sai contract unlink-last auto-pause và body `{autoPickPaused}`. Mutex của fakeStore che interleaving Postgres READ COMMITTED này; test PG chỉ unlink tuần tự.
- **Sửa:** khóa hàng owner_channels ở đầu transaction rồi mới xóa/đếm/quyết định pause; dùng cùng quy tắc khóa cho LinkSource và các source mutations liên quan. Test hai transaction có barrier trước COUNT, mong nguồn rỗng, auto_pick=false, một thao tác báo paused=true.

### 5. Canvas chỉ nằm trong prompt/request, không được bảo đảm trong file công bố

- **Vị trí:** `internal/channels/identity.go:673-712`; `internal/channels/files.go:85-101`. Callee thực tế: `internal/imagegen/api_images.go:153-158,367-374,414-423`.
- **Scenario:** chọn provider Images API kiểu OpenAI. `openAISize(800,800)` gửi 1024x1024; `openAISize(2560,1440)` gửi 1536x1024. Provider lưu PNG nguyên kích thước. Avatar được move nguyên file, banner chỉ `jpeg.Encode(img)` không resize/crop. Kết quả được công bố idle nhưng không phải 800x800 / 2560x1440; banner còn sai tỉ lệ và không đạt kích thước tối thiểu thông thường của YouTube.
- **Regression:** bản gốc `branding.ts:512-528` gọi encode với canvas đích; `clone/webp.ts:15-17,43-48` scale/crop và probe xác nhận kích thước. Đổi PNG/JPEG là lệch đã khai báo, bỏ normalize canvas thì chưa được khai báo/chấp nhận.
- **Sửa:** decode, normalize/crop về canvas đích rồi encode nguyên tử trước publish; kiểm kích thước file cuối, giữ giới hạn 6 MB banner. Test drawer trả ảnh lệch kích thước và assert pixel của file đã công bố, không chỉ assert request.Width/Height.

### 6. Branding endpoint theo symlink ra ngoài thư mục kênh/project

- **Vị trí:** `internal/channels/files.go:27-48`; `internal/web/channels.go:513`; resolver thật `internal/storage/service.go:256-272`.
- **Scenario / prerequisite:** một file đang được DB tham chiếu, có tên UUID hợp lệ, hoặc thư mục cha của nó trong project storage bị thay bằng symlink trỏ tới file ngoài project. Regex và DB membership đều qua; Resolve chỉ kiểm containment theo chuỗi, `os.Stat` theo symlink, ServeFile mở target. GET trả nội dung target với MIME ảnh. Đây **không** phải claim rằng URL `../` tự vượt được regex, và chưa tìm được remote-only đường tạo symlink trong scope.
- **Regression:** `yt-factory/branding.ts:84-90` dùng realpath root/file và kiểm relative chính xác; port bỏ lớp kiểm này.
- **Sửa:** phục vụ từ handle mở dưới root bị chặn symlink escape (ví dụ os.Root và mở file tương đối trong boundary), hoặc ít nhất realpath + containment tương đương bản gốc; không chỉ thêm filepath.Clean. Test file/parent symlink ra ngoài và một file kênh khác, phải 404. Ưu tiên phương án không có khoảng TOCTOU check/open.

### 7. Lỗi nền bypass error map an toàn và được lưu/trả nguyên nội dung nội bộ

- **Vị trí:** `internal/channels/identity.go:492-508,523-536`; `internal/web/channels.go:203-209`. Nguồn lỗi text thực: `core/llm/wire/errors.go:120-166` giữ body/metadata.raw từ upstream.
- **Scenario:** model text trả lỗi upstream chứa raw proxy response, hoặc filesystem publication lỗi chứa absolute path. Nhánh unknown dùng `err.Error()` làm message, ghi vào owner_channels và activity/error event; GET list/detail xuất thẳng `identity.error.message`. Hàm writeChannelInternal có mask/log cho 500 **không chạy** trên đường dữ liệu này.
- **Tác động:** lộ đường dẫn và chi tiết nội bộ; nếu upstream echo credential thì chuỗi đó cũng không được redact. Đây là đường truyền dữ liệu xác nhận từ source, không phải khẳng định đã quan sát secret thật.
- **Sửa:** lỗi lạ luôn lưu câu tiếng Việt trung lập + stable internal code; nguyên nhân chỉ tới diagnostic đã redact. Chỉ giữ message của lớp lỗi nghiệp vụ được bảo đảm an toàn. Test injected error chứa sentinel bí mật/path và assert không có trong GET, task hay error event.

### 8. Close có thể trả xong trước khi request đang chuẩn bị đăng ký goroutine mới

- **Vị trí:** `internal/channels/identity.go:118-127,142-240`; `internal/channels/channels.go:309-325`; integration `cmd/viderse/serve.go:184-205,218`.
- **Scenario:** HTTP request đang ở prepareIdentity/DB khi wg còn 0. Serve bắt đầu Shutdown trong goroutine, rồi gọi channelSvc.Close mà chưa đợi các handler xong. Close cancel baseCtx và wg.Wait trả ngay. Request cũ tiếp tục với request ctx vẫn sống, acquire lease, Add(1), spawn worker **sau Close**. Handler kết thúc, HTTP shutdown trả; worker vẫn có cleanup và FailIdentity dùng context riêng tối đa 10 giây. Close không join worker đó.
- **Tác động:** guarantee shutdown/join không đúng; có goroutine và DB writes sau khi composition root cho rằng runner đã dừng, nguy cơ đụng pool/log đang đóng. Không cần khẳng định race detector hiện tại bắt được: test không phối hợp admission và Close.
- **Sửa:** admission gate/closed flag có mutex, đăng ký mọi accepted run trước khi Close bắt đầu Wait và từ chối run sau closing; hủy cả giai đoạn prepare theo service lifetime. Không chỉ check ctx một lần trước Add. Test gate prepare, gọi Close, nhả prepare, kiểm không thể có worker sống sau Close. Với RunIdentity đồng bộ cũng cần xác định rõ tracking/lifecycle contract.

## Minor

### 9. Mọi đối thủ đều được đánh dấu reference=true

- **Vị trí:** `internal/persistence/postgres/channels.go:202-210`.
- `seen` dùng key channel + rival, mà mỗi cặp là PK duy nhất; mỗi source lần đầu đều có `!seen[key]=true`. Kênh có A/B trả cả hai reference=true, trái hợp đồng chỉ link sớm nhất. Test PG chỉ kiểm source đầu; fakeStore tự gán i==0 nên service test xanh.
- **Sửa:** track seen theo channel ID, không theo rival; test list nhiều kênh, mỗi kênh nhiều nguồn, chỉ phần tử đầu true và reference chuyển khi unlink.

### 10. Bù trừ dùng context đã bị hủy nên bỏ lại Mẫu chuẩn sau canceled create

- **Vị trí:** `internal/channels/channels.go:407-424,743-752`; callee `internal/library/library.go:494-515`.
- Template được commit trước; request timeout/cancel làm CreateChannel transaction thất bại. DeleteTemplate được gọi với chính ctx đã hủy, ngay truy vấn đầu đã không thể chạy, để lại template không ai dùng. Nhánh standard có cùng vấn đề. Fake DeleteTemplate bỏ qua ctx nên test hiện có không chứng minh được bù trừ thật.
- **Sửa:** cleanup bằng context không kế thừa cancellation nhưng có timeout ngắn (WithoutCancel + WithTimeout), vẫn best-effort và không che lỗi gốc; ghi diagnostic nếu cleanup thất bại. Test cancel ngay sau tạo template rồi xác nhận template không còn.

### 11. Delete tìm task bằng label khác task đã lưu

- **Vị trí:** `internal/channels/channels.go:579`; `internal/channels/identity.go:275`; `internal/persistence/postgres/queries/activity.sql:5-9`.
- Task của kênh tên “Tên A” lưu ObjectRef label “Tên A”; Delete gọi channelObject(id, empty) thành label “Kênh”. Activity query so toàn JSON, nên không tìm/xóa được task. Sau đó heartbeat của runner thấy kênh mất và failIdentity có thể giữ task thành failed trỏ tới kênh đã xóa.
- **Sửa:** lấy IdentityTaskID từ view trước khi xóa và Finish đúng ID (có bảo vệ ownership cần thiết), hoặc dùng lookup identity type+id ổn định không phụ thuộc display label. Test named channel + active task + Delete phải không còn task/error orphan.

### 12. Raw package globals trái quy tắc contract

- **Vị trí:** `internal/channels/identity.go:43,52`; `internal/channels/prompt.go:28`; `internal/channels/files.go:16`.
- Có slice/map mutable toàn cục cho JPEG qualities, host allowlist, answer limits, cộng regexp toàn cục. Contract §1 cấm biến toàn cục; không có lý do nghiệp vụ cho shared mutable state này.
- **Sửa:** qualities dùng giá trị cục bộ cố định; host/limit tra bằng switch hoặc constants; compiled regex thuộc instance dependency được dựng trong New nếu giữ regex. Không cần thêm manager/abstraction mới.

### 13. Chỉ lưu brief đã bị hiểu nhầm là có thiết kế để redraw

- **Vị trí:** `internal/channels/identity.go:163-164,658-670`; đường ghi hợp lệ `internal/channels/channels.go:541-548`, query PatchChannel ghi branding.brief.
- Kênh chưa từng công bố nhận diện, PATCH brief để mô tả chủ đề, rồi POST branding part=avatar hoặc banner. `hasPrevious` true chỉ vì Brief không rỗng, needsText=false, nhưng Avatar/Banner vẫn rỗng; runner thất bại “Thiếu brief ảnh” thay vì tạo phần thiết kế cần thiết. Các part endpoint hợp lệ không chạy được từ trạng thái hợp lệ này.
- **Sửa:** quyết định needsText dựa trên sự tồn tại của design brief cho asset được chọn, không dựa vào channel description. Test fresh channel → PATCH brief → avatar/banner, text chạy một lần, giữ tên và asset còn lại.

### 14. HTTP bắt buộc đối thủ bị vượt qua bằng danh sách toàn chuỗi trắng

- **Vị trí:** `internal/web/channels.go:258`; `internal/channels/channels.go:376`.
- POST `{competitorIds:["  ",""]}` có len>0 nên qua handler; service dedupe/trim thành rỗng, rồi tạo kênh không nguồn, không identityJobId. Sai acceptance ít nhất một đối thủ ở luồng web; CLI được phép rỗng không phải lý do để web chấp nhận.
- **Sửa:** kiểm ít nhất một ID không trắng tại HTTP boundary sau chuẩn hóa, hoặc truyền rõ yêu cầu nguồn bắt buộc vào service mà vẫn giữ CLI tùy chọn. Thêm case blank-only và duplicate/blank xen ID hợp lệ.

## Các biên đã kiểm, không tạo finding thêm

- **Avatar SSRF production path:** `identity.go:605-648` chỉ HTTPS, exact hostname `yt3.googleusercontent.com`/`yt3.ggpht.com`, default client không follow redirect, request ctx timeout 20 giây, LimitReader 8 MiB + 1 rồi loại oversize. Không thấy đường SSRF arbitrary host trong wiring thật (HTTP=nil). Client tiêm ở test không phải bằng chứng production follow redirect.
- **Publish fence:** Acquire/Heartbeat/Publish SQL có token; heartbeat và publish kiểm expiry; publish kiểm expected name và một UPDATE ghi toàn nhận diện. Tên bị sửa/xóa giữa run không được ghi đè/hồi sinh. Lease ownership và activity ownership là hai thứ khác nhau; finding 2 không phủ nhận fence SQL.
- **No auto paid redraw trong recovery:** không có đường Start gọi Drawer. Mỗi run có key chứa token/part; ảnh cũ chỉ bị dọn sau publish thành công. Crash chưa hết TTL là lỗi finding 1, không phải chứng cứ có auto-redraw.
- **Create transaction / library seam:** template tạo trước, kênh + sources + rotation link trong một transaction; rival validation/lock trước insert; RESTRICT bảo vệ template đang dùng. Vấn đề cleanup context ở finding 10 không phủ nhận transaction atomicity.
- **GET/layering/query shape:** list dùng một batch source query và một batch template query, không N+1 cho read; GET scope không spawn process, tree walk hay gọi mạng. Business không import web/persistence. Trong create có vòng query validate/link rival nhưng bị chặn trần 200, không ghi nhận là unbounded N+1.
- **Auth:** các route qua gateway session authentication; schema/contract hiện tại là workspace owner-channels dùng chung, không có owner_user_id hay chính sách per-channel ACL được chỉ định. Không tự suy diễn yêu cầu multi-tenant. Không thấy unauthenticated route bypass trong các mount mới.
- **SSE:** channel mutations qua web.Notifier gộp 250 ms; activity dùng notifier chung của serve. Không chạy p95/load test và không xác nhận trần DB progress-write bằng measurement. Không thấy side-effect init trong scope.
- **Branding compatibility:** giữ all/name/avatar/banner, briefs riêng từng ảnh, canvas/safe-band prompt, image cap và publish all-or-nothing. PNG/JPEG thay WebP và không có rival banner_url đã được implementer khai báo; không đánh thành thiếu scope. Các regression chưa được biện minh là canvas normalization và realpath containment ở finding 5/6. Không yêu cầu port channelLook legacy vì schema mới không có dữ liệu legacy trong scope.

## Verification exercised

Chạy đúng một lượt kiểm thử hẹp, có heavy lock, race và test database được chỉ định, từ module root:

```text
lockf -k /tmp/verse-heavy.lock nice -n 10 env GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test go test -race -p 1 ./apps/viderse/internal/channels/... ./apps/viderse/internal/web ./apps/viderse/internal/persistence/postgres -run Test(Identity|Channel|Create|Update|Delete|Link|Unlink|Template|Asset) -count=1
```

Trong shell thực tế biểu thức -run được quote. Kết quả tool:

```text
ok  github.com/datnt1998/verse/apps/viderse/internal/channels             28.378s
ok  github.com/datnt1998/verse/apps/viderse/internal/web                  24.689s
ok  github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres   2.765s
Wall time: 73.69 seconds
```

Không gọi model/image provider thật, không gọi avatar CDN, không mở server. Không chạy build/vet/lint/toàn repo, không sửa test để chứng minh finding. Các ca đang có không bao phủ interleaving, restart-before-expiry, symlink, mismatched image dimensions và activity thật được nêu trên; xanh không phải phủ nhận những lỗi source này.

## Plan follow-ups / recommended actions

1. Sửa lease recovery/lifecycle và task ownership trước; thêm deterministic gate tests (không stress ngẫu nhiên).
2. Chuyển source/template mutations sang concurrency contract ở Postgres; test nhiều connection/process semantics.
3. Khôi phục asset canvas + containment, sanitize persisted errors; fixture-only tests, không paid smoke.
4. Sửa các minor nêu trên, bổ sung assertions PG cho mọi reference flag và activity cleanup thật.
5. Lead giữ nguyên các defer đã được contract chốt (video/jobs/editor/autonomy); không thêm endpoint preview hoặc paid call trong fixes. `phase-04-channels-library.md` hiện pending; không sửa trạng thái plan từ review này. Phần CRUD/routes/CLI và integration seam đã hiện diện, nhưng nhãn DONE của P4-Channels chưa được chứng minh về robustness; không đánh giá DONE cho phần web hay các mảng khác.

## Metrics / unresolved questions

- Type coverage: N/A (Go; không đo phần trăm).
- Test coverage: chưa đo; chỉ có kết quả test selection bên trên.
- Lint issues: chưa đo, không suy từ test pass thành lint sạch.
- GET p95: chưa đo.
- Không có prerequisite chưa lấy được làm chặn review. Findings source-only đã nêu rõ điều kiện; riêng symlink cần khả năng thay nội dung project filesystem, không khẳng định remote exploit không điều kiện.

Status: DONE_WITH_CONCERNS — blocker: 0, major: 8, minor: 6.
