# Code Review Summary — Thư viện

## Scope

- Review working tree chưa commit: `internal/library/**`, `internal/domain/library.go`, PostgreSQL `library.go`, `library_pg_test.go`, `queries/library.sql` và generated SQL tương ứng; `internal/web/library.go`, `library_mix.go`, `library_test.go`; `cmd/viderse/library_wiring.go`, `template.go`, `overlay.go`; diff gateway/serve/main và `platform/webapp/server.go`; các khối templates/image_prompts/overlays của migration 00008.
- Đường dẫn mã bên dưới tương đối với `apps/viderse/`, trừ khi ghi rõ đường dẫn root repo.
- 7.600 dòng Go/SQL trong các tệp library/domain/store/web/CLI đã thống kê (gồm test; chưa tính shared gateway/migration và generated code). 12 MP4 nhúng: **10.129.520 byte**, khoảng **9,66 MiB** payload binary trước overhead. Không build binary để suy ra mức tăng thực tế; binary untracked hiện có không phải bằng chứng build của review.
- Chuẩn đối chiếu: `local://p4-contract.md`, root `AGENTS.md`, phase-04 plan, báo cáo P4-Library/P4-Channels; yt-factory `template-schemas.ts`, `template-limits.ts`, `template-recipe.ts`; noverse `projectUpload` precedent.
- Scout trước review: vòng đời seed → file → unique constraint; các tham chiếu prompt trong JSONB không có FK; multipart name/file khác nghĩa; phân giải đường dẫn DB → storage → ServeContent; API/CLI cùng parser nhưng import có decoder riêng.
- Read-only đối với mã/test/plan; chỉ tạo báo cáo này theo yêu cầu. Không gọi model, không gọi mạng nền tảng, không mở server.

## Overall assessment

**Chưa sẵn sàng landing.** CAS recipe ở SQL có điều kiện revision đúng và test đua writer qua được, nhưng parser có nhánh làm mất cấu hình nền khi input sai; seed song song có thể xoá tệp của writer thắng; prompt reference integrity vẫn check-then-write. Suite PostgreSQL chạy thật hiện thất bại ở hai test. Các race ghi nhận dưới đây được chứng minh bằng thứ tự thao tác trong source, không tuyên bố đã chạy thêm regression test cho chúng.

## Blocker

### B1 — Recipe nền sai bị lưu thành null, làm mất cấu hình hiện có

- **Vị trí:** `internal/library/recipe.go:183-185` (bỏ lỗi từ `parseBackdrop`); parser con ở `:265-275`; ghi thật ở `internal/library/library.go:438-450`.
- **Kịch bản:** template đang có backdrop hợp lệ, PATCH revision hiện tại với `{"recipe":{"backdrop":{"clipSec":"60"}}}` hoặc `{"recipe":{"backdrop":{"unexpected":true}}}`. Parser con trả lỗi và nil; caller bỏ lỗi, gán backdrop=nil. `ValidateRecipe` chấp nhận nil, `CheckRefs` bỏ qua, CAS lưu thành công và tăng revision. Client nhận 200 thay vì 400, cấu hình nền cũ mất.
- **Sửa:** trả lỗi `parseBackdrop` nguyên vẹn trước khi gán; không gọi store khi parse thất bại. Regression: backdrop sai kiểu/field lạ/array phải trả invalid_input và giữ nguyên recipe + revision.
- **Bằng chứng:** source trace trực tiếp; test parser hiện chỉ kiểm field lạ trong scenes, không kiểm lỗi decode backdrop.

## Major

### A1 — Seed đồng thời xoá tệp builtin mà transaction thắng đang tham chiếu

- **Vị trí:** `internal/library/overlays.go:289-305,326-330`; gọi nền ở `cmd/viderse/serve.go:153`.
- **Kịch bản:** hai serve dùng chung DB/root, cổng khác nhau, đều đọc chưa có snow. Cả hai rename vào cùng `library/overlays/snow-loop.mp4`; A insert thành công, B insert đụng unique sha/builtin. B chạy `os.Remove(target)` rồi coi unique là thành công. DB giữ dòng A nhưng file đã mất. Các lần seed sau thấy ExistingBuiltins nên không sửa lại.
- **Sửa:** mỗi attempt có final path riêng do nó sở hữu, hoặc serialize seed bằng lock dùng chung giữa process; chỉ xoá tệp của attempt thua, không unlink shared target. Test barrier hai seeder trên cùng root/DB, xác nhận một dòng và tệp vẫn đọc được; kiểm cả cancellation sau rename.
- **Bằng chứng:** source trace; test seeding hiện chỉ gọi tuần tự.

### A2 — Xoá/đổi kind prompt đua với lưu template tạo tham chiếu hỏng

- **Vị trí:** `internal/library/prompts_service.go:164-172,203-233`; `internal/library/library.go:444-450`; `internal/persistence/postgres/queries/library.sql:64-70,102-107,115-126`.
- **Kịch bản:** DELETE đọc `TemplatesUsingPrompt` rỗng; PATCH template kiểm PromptExists=true; DELETE commit; PATCH CAS commit JSONB chứa ID vừa xoá. Đổi kind cũng có cửa sổ tương tự, khiến scenePromptId trỏ cover. Snapshot sau đó thất bại hoặc phân giải sai kind (resolvePrompt không kiểm kind). FK templates→channel không bảo vệ prompt ID trong JSONB.
- **Sửa:** kiểm reference và ghi template phải trong cùng transaction/locking protocol với delete/kind-change prompt. Ví dụ khoá prompt rows theo thứ tự ổn định trong cả hai đường và giữ tới commit; writer template phải kiểm lại tồn tại/kind dưới khoá. Chỉ bọc DELETE trong transaction mà PATCH vẫn kiểm ngoài transaction là chưa đủ.
- **Bằng chứng:** store hiện chỉ giữ gen.Queries trên pool; không có transaction hoặc khoá xuyên các bước này.

### A3 — Loại media tin vào đuôi tên; probe không bắt buộc có video/ảnh

- **Vị trí:** `internal/library/overlays.go:80,107,145-154`; `internal/library/media.go:59-90,184-205`; HTTP trả MIME này tại `internal/web/library.go:732`.
- **Kịch bản:** tải audio-only MP4 dưới tên `x.png`, hoặc container không thuộc allowlist nhưng ffprobe đọc được với đuôi `.mp4`. ffprobe có thể exit 0, không có stream video hợp lệ; probeFile vẫn trả ProbeInfo rỗng thành công. detectBackground lỗi chỉ trả none. Dòng được insert và file được phục vụ với MIME giả theo tên; không phải overlay dùng được.
- **Sửa:** sniff signature/container bằng nội dung trước probe và đối chiếu allowlist (MOV/MP4/WebM cần nhận diện container phù hợp); bắt buộc stream video với kích thước hợp lệ; suy MIME từ media đã xác nhận, không từ display name. Reject audio-only và sai MIME/container bằng bad_media. Không tuyên bố đây là XSS/SSRF khi chưa có bằng chứng khai thác.
- **Bằng chứng:** source trace; test upload hiện dùng bytes không phải video với fake ffprobe thành công, nên không chứng minh validation MIME.

### A4 — Trường name hiển thị làm hỏng upload file hợp lệ

- **Vị trí:** `internal/web/library.go:658-662`; `internal/library/overlays.go:75-87,107`.
- **Kịch bản:** multipart `file=rain.mp4`, `name=Mưa` là body đúng API đã công bố, nhưng handler bỏ filename và service kiểm MimeForFile("Mưa") → 400. Nếu name có đuôi khác file thật thì MIME/path bị đổi theo tên hiển thị.
- **Sửa:** truyền filename và displayName thành hai trường riêng; validate/sniff file bằng bytes + filename gốc, chỉ dùng name cho nhãn, có giới hạn 1–200 ký tự tương ứng DB. Regression upload tên tiếng Việt không đuôi và tên có dấu chấm.

### A5 — Upload lại không phục hồi file bị mất dù API hướng dẫn đúng thao tác này

- **Vị trí:** `internal/library/overlays.go:129-142,183-193`.
- **Kịch bản:** file đã bị xoá khỏi đĩa nhưng DB còn SHA; GET file báo bad_media với recovery “Thêm lại tệp này vào thư viện”. Upload lại cùng bytes gặp ExistingBySHA và return dòng cũ, defer xoá tệp tạm mới tải. GET tiếp theo vẫn hỏng, không có đường tự phục hồi qua API.
- **Sửa:** khi dedupe, kiểm file của dòng cũ; nếu thiếu thì công bố nguyên tử bytes upload đã hash vào đường dẫn của dòng đó, giữ nguyên ID và xử lý concurrent repair an toàn. Không chỉ restore tombstone trong DB. Kiểm cả trường hợp dòng live và tombstone.

### A6 — SQLSTATE của ON DELETE RESTRICT không được map thành in-use

- **Vị trí:** `internal/persistence/postgres/library.go:552-555`; service dựa ErrInUse ở `internal/library/library.go:509-519`.
- **Kịch bản:** template vừa được gắn kênh sau bước kiểm TemplateChannels và trước DELETE. PostgreSQL thực tế trả **23001 restrict_violation**, trong khi map chỉ bắt **23503**. FK giữ dữ liệu an toàn nhưng API trả 500 internal thay vì 409 conflict có recovery.
- **Sửa:** map 23001 cho constraint owner_channel_templates_template_id_fkey (và 23503 khi thích hợp) sang ErrInUse; giữ nguyên lỗi constraint khác, không swallow.
- **Bằng chứng chạy thật:** `TestLibraryTemplateDeleteBlockedWhenChannelUses` thất bại tại library_pg_test.go:147 với SQLSTATE 23001. Không chạy lại để xác nhận.

## Minor

### N1 — ImageCountFor dùng công thức ceil số nguyên cho đầu vào float

- **Vị trí:** `internal/library/library.go:200`; nguồn đúng: yt-factory `template-recipe.ts:107-110`.
- **Kịch bản:** per-minutes=2, duration=121 giây → công thức hiện tại trả 1, trong khi ceil(121/60/2)=2. Với minutes=1, duration=61 giây cũng trả 1 thay vì 2. Giới hạn numeric port đúng không cứu được phép tính này.
- **Sửa:** `math.Ceil(durationSec / 60 / float64(m.Minutes))` rồi clamp trước khi đổi sang int; thêm biên 60/61/120/121 giây. Hiện helper chưa có production caller ngoài test, nên xếp minor thay vì tuyên bố đang tính sai giá GET.

### N2 — Import chấp nhận JSON có rác phía sau và version không hỗ trợ

- **Vị trí:** `internal/library/prompts_service.go:343-356`; ParseRecipe cũng thiếu kiểm EOF ở `internal/library/recipe.go:163-169` cho đường CLI recipe-file.
- **Kịch bản:** import body `{"format":"viderse-image-prompt","version":999,"name":"X","kind":"scene","prompt":"a"} garbage` vẫn được decode lần đầu và tạo prompt; version thiếu/0 cũng được nhận. Khác guard chuẩn DecodeJSONCode vốn bắt buộc EOF.
- **Sửa:** yêu cầu decode thứ hai là io.EOF, chấp nhận đúng version đã xuất (1), báo invalid_input cho unsupported/missing version. Áp cùng kiểm một JSON value cho parser CLI; HTTP outer decoder đã bảo vệ recipe raw trên đường PATCH thông thường.

### N3 — Test SHA duplicate thực tế chỉ đụng primary key

- **Vị trí:** `internal/persistence/postgres/library_pg_test.go:219-224`.
- **Kịch bản/bằng chứng chạy thật:** test InsertOverlay hai lần nguyên cùng ID; PostgreSQL trả overlays_pkey chứ không phải overlays_sha256_key, nên test fail ở :223 và chưa chứng minh dedupe SHA giữa hai upload độc lập.
- **Sửa:** tạo ID khác, giữ nguyên SHA để thực sự kiểm constraint SHA; kiểm builtin uniqueness bằng trường hợp riêng nếu cần. Không mở rộng production map để coi mọi primary-key collision là SHA dedupe.

### N4 — Template ID sai định dạng bị trả 500 thay vì 404/400

- **Vị trí:** `internal/persistence/postgres/library.go:59-63`; `internal/library/library.go:319-324`; `internal/web/library.go:750-800`.
- **Kịch bản:** GET/PATCH/DELETE `/api/templates/not-a-uuid`: GetTemplate trả sentinel ErrNotFound; service propagate nguyên sentinel, writeLibraryError chỉ map *library.Error nên trả internal 500. Với UUID hợp lệ nhưng không có thì lại 404.
- **Sửa:** thống nhất store not-found về nil hoặc map sentinel sang lỗi domain tại service; regression HTTP với PostgreSQL-backed service cho malformed UUID. Không leak lỗi scan UUID.

### N5 — PATCH kiểm tên đã trim nhưng ghi tên raw

- **Vị trí:** `internal/library/library.go:430-434,450,458`.
- **Kịch bản:** PATCH name="  Mẫu  " được service trả Name="Mẫu" nhưng DB lưu chuỗi có khoảng trắng; HTTP đọc lại trả tên raw. Tên trim dài 200 ký tự cộng padding có thể qua validator rồi đụng CHECK DB → 500.
- **Sửa:** truyền con trỏ tới tên đã normalize cho UpdateTemplate, giữ nil nếu không đổi tên. Test so sánh response/store và boundary 200 ký tự có padding.

### N6 — Các bảng giới hạn/tập giá trị là mutable globals trái hợp đồng

- **Vị trí:** `internal/domain/library.go:22-24,34-66`; `internal/library/recipe.go:43-52`; `internal/library/media.go:184-194`; `internal/library/prompts.go:30`.
- **Kịch bản:** domain.LibraryLimits.FixedImages.Max hoặc ThumbnailStyles có thể bị package khác sửa và đổi validator toàn process; đọc/ghi đồng thời không có đồng bộ. Đây là vi phạm trực tiếp quy tắc không biến toàn cục, không chỉ style nit.
- **Sửa:** numeric limits thành const; accessor trả value/fresh collection cho tập có thứ tự, switch cho MIME/kinds; giá trị regex/metadata bất biến đặt trong service hoặc hàm phù hợp. Embed FS và error sentinels là trường hợp kỹ thuật khác, không yêu cầu xoá để chạy theo style.

### N7 — Chú thích mã còn mang ID gói/design bị cấm

- **Vị trí:** `internal/library/prompts.go:11` ghi “Gói V15”; cùng dấu design-index ở `internal/library/prompts_service.go:14,318` và migration :22.
- **Kịch bản:** source chứa ID gói/phase thay vì chỉ giải thích bất biến, trái AGENTS Stable Code Artifacts và hợp đồng §1. Ví dụ V15 là vi phạm rõ ràng; nguồn port yt-factory trong comment test được phép và không phải mục cần bỏ.
- **Sửa:** bỏ ID gói/plan khỏi comment; mô tả trực tiếp tên duy nhất, prompt builtin chỉ đọc và quan hệ recipe. Không đổi tên migration do controller sở hữu chỉ để sửa style.

### N8 — Copy upload bỏ qua context cancellation

- **Vị trí:** `internal/library/overlays.go:116`; seed cũng ghi bytes tại :296 trước kiểm ctx tiếp theo trong vòng ngoài.
- **Kịch bản:** cancel ctx khi đang copy file lớn; io.Copy/LimitReader tiếp tục đọc/hash/ghi đến hết hoặc tới cap, sau đó mới gặp store/probe dùng ctx. Reader business không quan sát ctx; hợp đồng yêu cầu mọi việc theo ctx.
- **Sửa:** dùng vòng copy/chunk reader kiểm ctx giữa các lần đọc và kiểm ctx trước rename/publication; vẫn defer đóng/xoá temp. Không cần thêm polling hay goroutine copy không join.

## Edges checked / không ghi thành lỗi

- **CAS:** query UPDATE có WHERE id AND revision và tăng revision nguyên tử; test concurrent writer unit và PostgreSQL qua được trong run này. Không có bằng chứng lost update giữa hai writer cùng revision. CLI rename không tăng revision là quyết định đã công bố, không tự ý đảo quyết định đó trong review.
- **Numeric limits:** các khoảng fixedImages/perMinutes/motion/appearance/backdrop/clip đều khớp yt-factory; style cutout+framing thay cutout-close đúng binding contract. Partial PATCH dùng base hiện tại là chủ ý; không yêu cầu đổi về full-object zod ingestion.
- **Multipart:** MaxBytesReader đặt trước ParseMultipartForm, tổng request cap 256 MiB + 1 MiB overhead, file cap 256 MiB kiểm trước business copy và đọc thêm tối đa một byte. Prompt import cap 64 KiB. Không có đọc body không giới hạn. Giới hạn tổng request khác giới hạn file là hợp lý.
- **Temp cleanup:** business temp được defer Remove/Close trên các đường lỗi bình thường. ParseMultipartForm file spill của request HTTP thật được net/http dọn sau handler; không báo false positive chỉ vì thiếu RemoveAll trong handler. Atomic rename của upload dùng temp cùng thư mục; lỗi DB dọn file UUID riêng. Shared target của seed là ngoại lệ A1.
- **SHA races:** unique SHA có trong migration và map đúng tên constraint SHA. Upload user có final path UUID riêng, nên loser cleanup không xoá file winner; không gộp trường hợp này với seed shared path.
- **Startup/GET:** seed chạy từ composition root với bg context; GET library không probe/spawn/network/tree walk. BuiltinEffects chỉ đọc manifest; asset tree walk chỉ trong helper test. Seed không tự resurrect builtin tombstone ở đường tuần tự.
- **ServeContent:** route chỉ nhận ID; file path lấy từ DB, Resolve của storage từ chối absolute/.. escape, không join URL trực tiếp; MIME/Range/Content-Disposition không mở đường traversal từ request hiện tại. Không báo local symlink attack khi không có remote input tạo symlink. Resolve chỉ lexical, không phải sandbox chống actor đã sửa được filesystem/DB.
- **Auth/CSRF:** route chạy sau xác thực session; app dùng kho máy chia sẻ, không tìm thấy yêu cầu tenant isolation riêng cho từng user. AllowNonJSON chỉ POST đúng hai route, MIME parse chính xác, CrossSiteRequest chạy trước ngoại lệ; khớp noverse projectUpload ở gateway.go:589-605. Không mở miễn JSON toàn app, không bỏ Origin guard.
- **Prompt names:** builtins kiểm bằng lowercase trong business, custom unique lower(name) ở DB, errors 23505 constraint name được map conflict. Import suffix không bypass validation. Không tìm thấy race tạo trùng custom names.
- **Price:** perImage đọc model Artist.Draw đã lưu, không gọi model/network; unknown trả known=false/totalUSD=null. Theo công thức triển khai đã công bố, fixed=5 + cover=1, USD 0,04 → 0,24; none → một cover; per-minutes không có duration → null. Không dùng hằng 7 của màn Agents để thay template count. Việc luôn cộng một cover là quyết định sản phẩm trong báo cáo, không phải lỗi số học chứng minh được khi chưa có clone pipeline; cần lead ghi rõ quyết định design ở mục bên dưới.
- **Query efficiency/layering:** list template gộp channel refs bằng SQL, tên prompt đọc một lượt; không có N+1 GET theo từng template. Không có import web/persistence từ business trong các tệp review. Prompt/overlay usage dùng batch scan; chưa đo p95 trên volume thật, không suy luận đạt SLO.
- **Errors/data exposure:** lỗi nội bộ HTTP qua generic 500; riêng errBadMedia dùng firstLine nguyên lỗi probe và có thể chứa absolute temp path (`internal/library/media.go:64,222-226`). Chưa có bằng chứng lộ secret; nên đưa path vào diag và trả câu lỗi media được duyệt nếu sửa boundary này.

## Exercised verification

Chạy đúng một lệnh, không rerun sau lỗi:

```sh
lockf -k /tmp/verse-heavy.lock nice -n 10 env GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test go test -race -p 1 ./apps/viderse/internal/library ./apps/viderse/internal/web ./apps/viderse/internal/persistence/postgres -run 'Test(DefaultRecipe|StandardRecipe|ValidateRecipe|ParseRecipe|CheckRefs|ImageCountFor|Builtin|CreateTemplate|CreateStandardTemplate|DuplicateTemplate|Patch|RenameTemplate|DeleteTemplate|PriceEstimate|ListSummary|ListPrompts|CreatePrompt|Prompt|ClassifyBackground|AddOverlay|ListOverlays|RemoveOverlay|SeedBuiltins|Snapshot|Library)'
```

- library: **PASS**, 1,776 s.
- web: **PASS**, 20,279 s (chỉ các test match filter; không tuyên bố toàn package).
- postgres: **FAIL**, 1,536 s; hai lỗi A6 và N3:
  - `TestLibraryTemplateDeleteBlockedWhenChannelUses`, :147, SQLSTATE 23001.
  - `TestLibraryOverlayStoreTombstone`, :223, overlays_pkey, SQLSTATE 23505.
- Không có data race được báo trong những test đã chạy. Logical races A1/A2 không được suite hiện tại ép lịch, nên race detector xanh không bác bỏ chúng.
- Type coverage / test coverage / lint issue count: **không đo**. Không chạy build, formatter, full repo suite, archtest hoặc p95 benchmark.

## Plan / report follow-ups

- Có implementation thực cho template CAS, prompt CRUD/builtins/import/export, overlay storage/upload/startup seed, CLI và wire routes. Không đánh dấu complete trước khi sửa B1/A1–A6 và suite PostgreSQL qua được.
- Preview generation, samples, clone jobs/editor và UI là non-goal đã chốt trong binding contract; không coi thiếu các phần đó là finding. Plan cũ vẫn pending và có scope rộng hơn contract; review không sửa trạng thái plan.
- Báo cáo P4-Library tuyên bố “đủ test/CAS thật/FK” và Status DONE cần bổ sung hai failure có log thật ở trên; seeding “idempotent” mới đúng cho tuần tự, chưa đúng concurrent startup.
- Cần regression deterministic cho B1, A1, A2, A3–A5; giữ test CAS thật đang có. Sau sửa chạy lại focused library/web/Postgres dưới lock, rồi lead thực hiện architecture/GET p95 theo nghiệm thu riêng.
- Cần lead ghi quyết định pricing (template count + cover thay vì ví dụ 7 ảnh; cover upload vẫn tính hay không) vào nguồn design tương ứng; không silently thay công thức trong review. Không có câu hỏi chặn việc sửa các lỗi kỹ thuật đã nêu.

Status: DONE_WITH_CONCERNS — blocker: 1; major: 6; minor: 8.
