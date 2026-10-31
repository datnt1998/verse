# 11 · Hợp đồng API v1 (chuẩn để implement)

Tài liệu này **chốt** các API mà `06-data-contract.md` và `09-auth-onboard.md` đánh dấu GAP. Backend và frontend phải khớp đúng tên trường dưới đây. Mọi thay đổi hợp đồng phải sửa tài liệu này trước.

Quy ước chung (giữ nguyên từ bản hiện tại):
- JSON, `camelCase` cho trường API mới. Các struct engine đã có `json` tag (ví dụ `ReviewEntry` dùng `snake_case`) được trả **nguyên tag gốc**, ghi rõ bên dưới.
- Lỗi: `{"error": string}` (tiếng Việt cho lỗi người dùng). `429` thêm `retryAfter` (giây).
- Ghi: `POST`/`PUT` bắt buộc `Content-Type: application/json`. Thân rỗng gửi `{}`.
- Mã: 400 dữ liệu sai · 401 chưa đăng nhập · 403 không có quyền (vai trò/quyền dự án) · 404 không có — và **truyện không đọc/ghi được luôn trả 404** để không lộ tồn tại · 409 xung đột (trùng, đang chạy, bị khoá, quản trị viên cuối) · 502 worker lỗi.
- Thời gian: RFC3339. Thời gian rỗng = `null` (không trả `0001-01-01`).

## 0. Cắt chuyển

`/api/books*` **bị xoá**. Thay bằng cây `/api/library` → `/api/projects/:p` → `/api/projects/:p/stories/:s`. Frontend chuyển `#/book/:id` → `#/p/_/s/:id/work`.

## 1. Mô hình thư mục

```
<root>                          ← serve --root (Thư viện)
├── .ainovel/library.json       ← LibraryConfig (GAP-13)
├── <dự án>/                    ← có project.json
│   ├── project.json            ← ProjectFile (GAP-2)
│   ├── .ainovel/config.json    ← cấu hình dự án (GAP-10)
│   ├── .ainovel/rules/*.md     ← quy tắc dự án
│   ├── <dirs.ideas>/*.md       ← ý tưởng (GAP-11)
│   ├── <dirs.assets>/          ← tài sản
│   └── <dirs.stories>/NN-slug/ ← truyện
│       ├── story.json          ← StoryFile (GAP-3)
│       ├── .ainovel/config.json, .ainovel/rules/   (tùy chọn, ưu tiên cao nhất)
│       ├── output/novel/       ← engine (chỉ đọc với UI)
│       └── <dirs.exports>/     ← xuất bản (GAP-4)
└── <thư mục cũ>/output/novel   ← truyện kiểu cũ, gom vào dự án ẩn danh "_"
```

- **Dự án** = thư mục con trực tiếp của root có `project.json`. Id dự án = tên thư mục.
- **Dự án `_`** (ẩn danh, `legacy: true`): gom các thư mục con trực tiếp của root **không** có `project.json` mà có `output/` hoặc `.ainovel/` (cách bố trí cũ). Chỉ tồn tại khi có ít nhất một truyện như vậy. Không sửa được `project.json`, không có ý tưởng/tài sản.
- **Ứng viên** (`candidates`): thư mục con của root không có `project.json` nhưng có thư mục `<dirs.stories>` → có thể “nhận làm dự án” (onboard bước 6, nút Kiểm tra chuẩn).
- **Truyện** = thư mục con trực tiếp của `<dự án>/<dirs.stories>/` (dự án `_`: thư mục con của root). Id truyện đầy đủ = `"<p>/<s>"`.
- Tên thư mục hợp lệ (dự án, truyện, `dirs.*`): `^[a-z0-9][a-z0-9_-]*$`, ≤ 100 ký tự, không trùng `output`, `.ainovel`, `project.json`, `story.json`. Ngoại lệ đọc: truyện/dự án cũ có tên khác vẫn liệt kê được (chỉ không tạo mới với tên sai chuẩn).
- Worker chạy với **cwd = thư mục truyện**; gateway truyền `AINOVEL_LIBRARY_ROOT=<root>` qua env.

## 2. Quyền (webauth)

`User.books[]` chấp nhận: `"*"` · `"<p>/*"` · `"<p>/<s>"`. Mục cũ không có `/` được hiểu là `"_/<tên>"`.
- `CanRead(storyID)`, `CanWrite(storyID)` như hiện tại, khớp theo 3 dạng trên. Admin luôn đủ quyền.
- `CanReadProject(p)` = có ít nhất một truyện đọc được trong `p`, hoặc có `"<p>/*"`/`"*"`.
- Tạo dự án: admin. Sửa `project.json`, thêm ý tưởng: admin hoặc writer có `"<p>/*"`/`"*"`. Tạo truyện trong dự án: admin hoặc writer có `"<p>/*"`/`"*"`; writer chỉ có quyền truyện lẻ thì không tạo được. Người tạo truyện được cấp `"<p>/<s>"` nếu chưa phủ.
- `GET` trên truyện đọc được; mọi ghi trên truyện cần `CanWrite`.

## 3. Thư viện & dự án (GAP-1, 2, 3, 9, 11, 12, 13)

### Kiểu

```ts
type LibraryConfig = {
  version: 1
  dirs: { ideas: string; assets: string; stories: string; exports: string; audio: string } // mặc định y-tuong · tai-san · kich_ban · xuat-ban · audio (audio là thư mục cấp truyện)
  slugWords: number                // 1–12, mặc định 5
  autoExport: AutoExport
}
type AutoExport = {
  onVolume: boolean   // mặc định true
  onBook: boolean     // mặc định true
  onChapter: boolean  // mặc định false
  formats: ('txt' | 'epub')[]      // mặc định ['txt','epub']
  pattern: string     // mặc định 'tap-{tap}/{truyen}.tap-{tap}.{ext}'; biến: {truyen} {tap} {tu} {den} {ngay} {ext}
  onExists: 'replace' | 'keep'     // keep → thêm -{ngay} (YYYYMMDD-HHmmss) trước đuôi
}
type ProjectSummary = {
  id: string; name: string; description: string
  legacy: boolean; storyCount: number; runningCount: number; updatedAt: string | null
}
// `language` đã bị xoá (14 §B.4 — chỉ còn tiếng Việt): project.json cũ có khoá này thì bị bỏ qua khi đọc, không ghi lại.
type ProjectFile = { name: string; description: string; notes: string; createdAt: string | null }
type Project = ProjectSummary & { notes: string; createdAt: string | null; path: string /* tên hiển thị tương đối root, vd "dit-dang-audio" */ }
type StoryStatus = 'idea' | 'writing' | 'paused' | 'done' | 'published'
type StoryFile = { title: string; status: StoryStatus; idea: string; tags: string[]; publishedAt: string | null; createdAt: string | null }
type StorySummary = {
  id: string            // "<p>/<s>"
  project: string; story: string
  folder: string        // tương đối dự án, vd "kich_ban/01-tra-xanh"
  title: string         // story.json.title, nếu rỗng lấy meta engine, nếu rỗng = story
  status: StoryStatus   // story.json; nếu thiếu suy ra: engine complete → done, có chương → writing, còn lại idea
  tags: string[]; idea: string
  phase: string; completedChapters: number; totalChapters: number; totalWords: number
  volumes: number       // số tập (layered) hoặc 1 nếu có đại cương, 0 nếu chưa
  open: boolean; running: boolean
  locked: boolean       // bị tiến trình khác (TUI) giữ — từ .ainovel.lock, không cần mở worker
  updatedAt: string | null
  exportsCount: number
  tempName: boolean     // thư mục còn tên tạm "NN-truyen-moi" → gợi ý đổi tên (GAP-9)
}
type Idea = { id: string /* tên tệp không đuôi */; title: string; excerpt: string; createdAt: string | null; updatedAt: string | null; usedBy: string[] /* story ids có story.json.idea == id */ }
type Asset = { path: string /* tương đối thư mục tài sản */; kind: 'file' | 'dir'; size: number; updatedAt: string | null }
type FolderNode = {
  name: string; path: string /* tương đối dự án */; kind: 'file' | 'dir'
  role: 'project' | 'config' | 'rules' | 'ideas' | 'assets' | 'stories' | 'story' | 'storyMeta' | 'engine' | 'exports' | 'other'
  status: 'ok' | 'engine' | 'optional' | 'missing' | 'invalid'
  note?: string; children?: FolderNode[]
}
type ValidationIssue = { path: string; level: 'error' | 'warn' | 'info'; code: string; message: string; fixable: boolean }
```

### Endpoint

| Method & path | Thân | Trả về | Quyền / lỗi |
|---|---|---|---|
| `GET /api/library` | — | `{configured: boolean /* false = cài mới, chưa chọn nơi lưu truyện (GAP-34) */, rootDisplay: string /* "Nơi › … › Thư mục", "" khi chưa chọn; KHÔNG còn `root` (tên thư mục trần) */, rootPath: string /* admin: tuyệt đối, chỉ cho tooltip/sao chép; người khác và khi chưa chọn: "" */, config: LibraryConfig, projects: ProjectSummary[], candidates: string[] /* chỉ admin, còn lại [] */}` | đăng nhập; chỉ liệt kê dự án đọc được |
| `PUT /api/library` | `LibraryConfig` **hoặc** `{root: string}` | `{ok: true, config: LibraryConfig, version: string}` (dạng cấu hình) / `{ok: true, root: string, rootDisplay: string, config: LibraryConfig, version: string}` (dạng đổi gốc) | admin; 400 tên thư mục/đường dẫn sai; **đổi gốc**: 400 nếu không phải thư mục đọc+ghi được, 409 nếu có worker đang chạy, dừng worker nhàn rỗi, ghi `~/.ainovel/serve.json` và `serve` không có `--root` dùng lại (xem 14 §G2) |
| `POST /api/projects` | `{slug?, name, description?, notes?, shareGlobal?: boolean /* mặc định true */}` (`language` **đã xoá** — 14 §B.4; gửi kèm bị bỏ qua). `slug` **tùy chọn**: bỏ trống thì server tự sinh từ `name` (`Slugify`, thêm `-2`, `-3`… khi trùng thư mục) — UI không hỏi tên thư mục (20-settings-rules.md §1). | 201 `Project` | admin; 400 thiếu cả `slug` lẫn `name` (“Nhập tên dự án”) hoặc `slug` sai; **409 “Đã có dự án cùng tên — đặt tên khác nhé”** khi `name` trùng tên dự án có sẵn (không phân biệt hoa thường), 409 khi `slug` đã có thư mục. Tạo `project.json`, `.ainovel/config.json` (chỉ khi `shareGlobal:false`: ảnh chụp `roles`, `style`, `budget` từ toàn cục, **không** chép providers/khoá), `.ainovel/rules/`, các `dirs.*`. `shareGlobal` = ô “Dùng chung trợ lý AI và quy tắc với “Mọi truyện””. **true (mặc định)**: chép `~/.ainovel/rules/*.md` (gồm `quy-tac.md`, không chép README.txt) vào `.ainovel/rules/` của dự án; **không** ghi roles/style/budget nên trợ lý AI kế thừa “Mọi truyện” qua lớp cấu hình. **false**: dự án không có quy tắc riêng, và `.ainovel/config.json` giữ ảnh chụp roles/style/budget hiện tại (“riêng dự án”). |
| `POST /api/projects/adopt` | `{dir, name?}` | 201 `Project` | admin; `dir` phải nằm trong `candidates`; chỉ ghi `project.json` + tạo thư mục còn thiếu |
| `GET /api/projects/:p` | — | `{project: Project, stories: StorySummary[], ideas: Idea[], assets: Asset[]}` | đọc dự án |
| `PUT /api/projects/:p` | `{name, description, notes}` (`language` **đã xoá**; gửi kèm bị bỏ qua) | `Project` | ghi dự án; 409 với `_` |
| `GET /api/projects/:p/stories` | — | `{stories: StorySummary[]}` | |
| `POST /api/projects/:p/stories` | `{name?: string /* tên làm việc */, idea?: string /* Idea.id */}` | 201 `StorySummary` | ghi dự án. Thư mục `NN-<slug>`: NN = số lớn nhất hiện có + 1 (2 chữ số); slug = `slugify(name, slugWords)` hoặc `truyen-moi` (khi đó `tempName=true`). Ghi `story.json {title: name ?? "", status: "idea", idea, tags: [], createdAt}` |
| `GET /api/projects/:p/stories/:s/meta` | — | `StoryFile` | đọc |
| `PUT /api/projects/:p/stories/:s/meta` | `StoryFile` (không gồm createdAt) | `StoryFile` | ghi |
| `POST /api/projects/:p/stories/:s/rename` | `{slug: string /* phần sau NN- */}` | `StorySummary` (id mới) | ghi; 409 nếu worker đang mở hoặc truyện bị khoá; giữ tiền tố `NN-`; cập nhật `books[]` của mọi tài khoản trỏ tới id cũ |
| `POST /api/projects/:p/stories/:s/close` | `{}` | `{ok:true}` | ghi; dừng worker |
| `GET /api/projects/:p/ideas` | — | `{ideas: Idea[]}` | |
| `GET /api/projects/:p/cocreate?draft=` | — | `{draftId, active, messages: {role, content}[], draft /* chỉ dẫn đã tổng hợp */, ready, suggestions: string[]}`; không có `draft` → state rỗng | ghi dự án; 404 nháp không tồn tại (xem 14 §G8) |
| `POST /api/projects/:p/cocreate/message` | `{message, draft?}` (hoặc `?draft=`) | `{draftId, reply, draft, ready, suggestions}`; thiếu `draft` → tạo id mới + thư mục `<dự án>/.ainovel/drafts/<id>/`, worker chạy với cwd = thư mục nháp | ghi dự án; 400 `message` rỗng |
| `POST /api/projects/:p/cocreate/commit` | `{draft, name?}` | 201 `{story: StorySummary, warning?: string}` — chuyển nháp thành `<dirs.stories>/NN-<slug>` (slug theo `name` hoặc dòng đầu tin nhắn đầu), ghi `story.json`, bắt đầu viết bằng `POST /input` với bản nháp; `warning` khi chưa khởi động được engine | ghi dự án; 409 chưa có phiên/bản nháp rỗng (xem 14 §G8) |
| `POST /api/projects/:p/cocreate/cancel` | `{draft}` | `{ok: true}` — dừng worker nháp, xoá thư mục nháp | ghi dự án; 404 nháp không tồn tại |
| `POST /api/projects/:p/ideas` | `{text: string /* ≤ 20000 */, title?: string}` | 201 `Idea` | ghi dự án; tệp `<dirs.ideas>/<YYYYMMDD-HHmm>-<slug(title‖dòng đầu, 6)>.md`; tiêu đề = `title` hoặc dòng đầu (bỏ `#`) |
| `GET /api/projects/:p/ideas/:id` | — | `Idea & {text: string}` | |
| `GET /api/projects/:p/assets` | — | `{assets: Asset[]}` (đệ quy, tối đa 500 mục) | |
| `POST /api/projects/:p/assets` | multipart `kind=cover\|music` + `file` (≤ 64 MB; cover: jpg/jpeg/png/webp · music: mp3/m4a/wav/ogg/flac) | `201 Asset` (`path` tương đối thư mục tư liệu: `bia/…` · `nhac/…`; trùng tên → thêm hậu tố `-2`, không ghi đè) | quyền quản lý dự án; 400 sai loại/định dạng · 413 quá lớn. Giọng mẫu dùng `POST /api/projects/:p/voices` |
| `GET /api/projects/:p/validate` | — | `{ok: boolean, issues: ValidationIssue[], tree: FolderNode}` | đọc; `tree` gồm cấu trúc chuẩn + thực tế, tối đa độ sâu 3 dưới mỗi truyện (không liệt kê bên trong `output/novel`) |
| `POST /api/projects/:p/validate/fix` | `{codes?: string[]}` | như `validate` sau khi sửa | admin; chỉ sửa an toàn: tạo thư mục thiếu, tạo `project.json`/`story.json` thiếu, chuyển `_chung/*.md` → `.ainovel/rules/`, xoá symlink rules trỏ vào `.ainovel/rules` của dự án |

**Bổ sung G8 · đề xuất có cấu trúc (18-mobile M5):** phản hồi rỗng, GET và message
ở cấp dự án có thêm `schemaVersion:1`, `proposal`, `committed`, `nextChapter` theo
kiểu `CoCreateProposal` trong §5. Commit 201 cũng trả bốn trường này cùng `story`
và `warning` nếu có. Quy trình khởi động trong hàng commit phía trên được thay
bằng **`POST /cocreate/commit {draft}`** trên worker truyện mới, không còn `/input`:
thư mục được chuyển cùng `.ainovel/cocreate/session.json`, worker khôi phục phiên,
chạy `PrepareUserRules`/`StartPrepared`, rồi lưu `committed:true`. Nhờ vậy tải lại
truyện không khôi phục nhầm một phiên đang chờ chốt. Nếu khởi động lỗi hoặc vẫn
đang chạy nền sau 3 giây, 201 trả `committed:false`; GET đồng sáng tác của truyện
cho trạng thái thật khi hoàn tất. Context lượt nền sống tới khi khởi động xong
hoặc hết hạn 5 phút, không bị huỷ khi phản hồi sớm.

`slugify`: bỏ dấu (NFD + bỏ dấu kết hợp, `đ→d`), thường, ký tự khác `[a-z0-9]` thành khoảng trắng, lấy `slugWords` từ đầu, nối `-`. Rỗng → `truyen-moi`.

## 4. Truyện — đọc trực tiếp (không cần worker, chạy được khi bị khoá) (GAP-7, GAP-4)

Gateway phục vụ qua hàm đọc của `host` (không giữ khoá truyện). Tiền tố: `/api/projects/:p/stories/:s`.

| Method & path | Trả về |
|---|---|
| `GET /chapters` | `{chapters: ChapterInfo[], revision: string}` · `ChapterInfo = {chapter, title, completed, words, hasDraft: boolean, volume: number, arc: number}` · `revision` đổi khi bất kỳ chương/bản nháp/thẩm duyệt đổi (mtime+kích thước; xem 14 §G10) |
| `GET /chapters/:n?source=committed\|draft` | `{chapter, title, text, source, words}` · 404 không có |
| `GET /chapters/:n/plan` | `ChapterPlan` (struct engine, trả nguyên tag) · 404 |
| `GET /chapters/:n/facts` | facts chương (struct engine, nguyên tag) · 404 |
| `GET /chapters/:n/revisions` | `{revisions: {time: string|null, kind: string, old: string, new: string, replaceAll: boolean}[]}` (rỗng nếu engine không lưu) |
| `GET /outline` | `Outline` (dưới) |
| `GET /characters` | `{characters: Character[], supporting: CastEntry[]}` (struct engine, nguyên tag) |
| `GET /foreshadow` | `{entries: ForeshadowEntry[]}` (nguyên tag engine); kiểu đầy đủ và evidence do [domain/review.go](../../../internal/domain/review.go) sở hữu, không duy trì DTO rút gọn riêng ở đây |
| `GET /reviews/latest` | `ReviewEntry` mới nhất (nguyên tag engine, gồm checklist và kết quả kiểm lại); kiểu nguồn: [domain/review.go](../../../internal/domain/review.go), [domain/goals.go](../../../internal/domain/goals.go) · 404 |
| `GET /reviews/:chapter` | `{chapter: ReviewEntry\|null, global: ReviewEntry\|null}` |
| `GET /export?format=txt\|epub&from=&to=` | tải về (như cũ, thêm khoảng chương) |
| `GET /exports` | `{dir: string /* tương đối truyện */, files: ExportFile[], matrix: ExportRow[], autoExport: AutoExport}` |
| `POST /exports` | `{format, from?, to?, volume?, overwrite?: boolean}` → `ExportResult` · ghi vào `<dirs.exports>/` theo `pattern` (cần quyền ghi) |
| `GET /exports/file?path=` | tải tệp trong thư mục xuất bản (chặn `..`) |

`ReviewEntry.goal_checks` thuộc [GoalCheck / GoalsFromContract / DeriveContractStatus](../../../internal/domain/goals.go): checklist đóng băng từ contract của chương, trạng thái `pass|fail|unknown`; `unknown` không phải đạt. Quyền tiếp nhận thuộc [SaveReviewTool.Schema / normalizeReviewEntry](../../../internal/tools/save_review.go): scope chapter có mục tiêu phải phủ đúng bộ ID, không thiếu/trùng/lạ; scope khác hoặc không có mục tiêu gửi mảng rỗng. `contract_status` / `contract_misses` được suy ra, không dùng giá trị Editor tự kết luận khi có checklist. Mọi quote được gửi trong checklist và issue phải khớp chính văn của chương liên quan qua [LocateQuote](../../../internal/domain/quote.go); một quote có thật chỉ chứng minh vị trí, không chứng minh suy luận đúng.

`ReviewEntry.recheck` đánh dấu vòng “Kiểm lại”; `recheck_results` thuộc [RecheckResult](../../../internal/domain/review.go), trạng thái `fixed|unfixed|unknown`. `finding_id` dùng ID chapter-local của vòng nguồn lưu trữ ngay trước, không phải index hoặc ID mục hiện tại; UI ghép qua `previous[last].items`. [validateRecheck](../../../internal/tools/save_review.go) sở hữu điều kiện evidence: `unfixed` cần quote hiện tại và issue cùng quote yêu cầu sửa; `fixed` không chỉ dựa vào việc đã viết lại. [ApplyReviewOutcome](../../../internal/store/progress.go) gộp chương chưa sửa được vào hàng đợi hiện có, không thay thế các chương khác. ID thẻ cũ và quyết định “Giữ nguyên” không đổi. Kiểu máy sinh được sở hữu bởi [api_dto.go](../../../internal/entry/web/api_dto.go) và [types.gen.ts](../../../web/src/api/types.gen.ts).

Sổ phục bút dùng các trạng thái `planted|advanced|resolved|abandoned`; `due_by`, `last_touched`, `abandon_reason`, `abandoned_at`, `evidence` thuộc [ForeshadowEntry](../../../internal/domain/review.go). `abandon` nhận `reason` trong `ForeshadowUpdate`. [ApplyForeshadow](../../../internal/domain/foreshadow.go) là máy trạng thái chung cho store và revision replay: thao tác trên tuyến đã đóng không mở lại tuyến; cập nhật kèm quote vẫn có thể bổ sung evidence. `ThreadHealth` là nguồn tính overdue/dormant, không phải quy tắc UI tự phán rằng tuyến phải thu hồi.

Quote tùy chọn của `TimelineEvent`, `StateChange`, `ForeshadowUpdate` được lưu dưới dạng trích dẫn, không lưu offset. [ValidateQuotes](../../../internal/chapterfacts/facts.go) định vị quote không rỗng trên bản nộp `commit_chapter` mới và [phân tích revision](../../../internal/revision/analyze.go); không kiểm lại pending đã đóng băng. Import không mang quote nên không bị yêu cầu bổ sung. Đoạn bằng chứng đọc lại từ chính văn hiện tại và chính sách ngân sách mềm được sở hữu bởi [quản lý ngữ cảnh](../../context-management.md).


```ts
type Outline = {
  layered: boolean; path: string /* "output/novel/outline.json" */
  volumes: { index: number; title: string; summary: string; arcs: { index: number; title: string; summary: string; chapters: number[] }[]; chapters: OutlineChapter[] }[]
  nextVolumeTitle: string; compassDirection: string; compassScale: string
}
type OutlineChapter = { chapter: number; title: string; coreEvent: string; summary: string; completed: boolean; words: number
  status: 'done' | 'writing' | 'pending' | 'rewrite' | 'rewriting'; hasPlan: boolean; hasReview: boolean }
type ExportFile = { path: string; name: string; format: 'txt' | 'epub' | 'audio' | 'other'; size: number; updatedAt: string | null; volume: number | null }
type ExportRow = { volume: number; title: string; from: number; to: number; complete: boolean; txt: ExportFile | null; epub: ExportFile | null; audio: ExportFile[] }
type ExportResult = { path: string; bytes: number; chapters: number; skipped: number[] }
```

Truyện phẳng (không layered) = 1 tập (`index 1`, `title` = tên truyện).

### Góp ý theo đoạn (18-mobile M2)

Tiền tố `S = /api/projects/:p/stories/:s`. Gateway lưu độc lập ở
`<story>/.ainovel/gop-y/<chapter>.json`, dùng khoá riêng; GET/POST/DELETE không
khởi động worker và vẫn hoạt động khi TUI đang giữ khoá truyện.

```ts
type ParagraphComment = {
  id: string; paragraph: number | null; quote: string; chips: string[]; text: string
  author: string; createdAt: string; appliedAt?: string
}
```

`paragraph` là vị trí đoạn bắt đầu từ **0**; `null` = *Góp ý sửa* cả chương.
Đoạn cụ thể phải có `quote`. `chips` chỉ nhận: **Ngắn lại · Thêm thoại · Kịch tính hơn ·
Giữ giọng văn · Chậm nhịp lại**; cần ít nhất một chip hoặc `text` không rỗng.
`author` lấy từ tài khoản đăng nhập, thời gian RFC3339 UTC do máy chủ tạo.

| Method & path | Thân | Trả về / quyền |
|---|---|---|
| `GET S/chapters/:n/comments` | — | `{comments: ParagraphComment[]}` (luôn mảng); đọc |
| `POST S/chapters/:n/comments` | `{paragraph, quote, chips, text}` | 201 `ParagraphComment`; ghi |
| `DELETE S/chapters/:n/comments?id=:id` | hoặc `{id}` | `{ok:true}`; ghi; 404 mã không tồn tại |
| `POST S/next` | `{applyComments:true, chapter?:number}` | `{ok:true, appliedComments:number}`; ghi; 409 nếu cổng duyệt chưa sẵn sàng hoặc TUI giữ khoá |

*Duyệt & viết tiếp* gửi `applyComments:true`; bỏ `chapter` thì chọn chương hoàn thành
lớn nhất. Các góp ý chưa có `appliedAt` được gom thành **một chỉ dẫn tiếng Việt**
cho Người viết: theo số đoạn tăng dần, cùng đoạn theo thời điểm tạo; góp ý cả chương
đặt cuối. Mỗi mục giữ nguyên trích dẫn và ghép chip + ý kiến tự do.
Host ghi chỉ dẫn Writer và giấy phép chương tiếp theo trong cùng một lần ghi
nguyên tử `meta/run.json` **trước khi** khởi động engine; khi khởi động lại chỉ dẫn
được khôi phục, không gọi Arbiter để diễn giải lại. Chỉ sau khi hành động duyệt
được chấp nhận mới đánh dấu `appliedAt`; thất bại giữ nguyên góp ý đang chờ.
`appliedAt` nghĩa là đã giao cho Người viết, không khẳng định LLM đã sửa xong.
Mọi lần thêm/xoá/duyệt thành công phát `story.changed{parts:["comments"]}` qua Hub.
Reader chỉ đọc; writer/admin có quyền truyện mới được góp ý, xoá và duyệt.

## 5. Truyện — qua worker (live)

Tiền tố `/api/projects/:p/stories/:s`. Giữ nguyên hành vi hiện có: `GET /snapshot`, `GET /events/live` (SSE; `/events` dành cho lịch sử GAP-31), `POST /input {text}`, `/resume`, `/abort`, `/next`, `/review {on}`, `/reopen {direction}`. 409 `errBookLocked` khi truyện bị khoá.

Hành động điều khiển engine có thể gọi Arbiter/LLM (`POST /input`, `/resume`, `/reopen`, `/start-file`, `/cocreate/commit`, `/command` `start`/`reopen`, `/diag/actions/:fingerprint`, `/sync/fix`) không giữ HTTP chờ model: lỗi điều kiện tức thời (đang chạy, đang có việc độc quyền, ngân sách…) vẫn trả 409 trong khoảng 250 ms; nếu còn đang xử lý thì trả **202** cùng thân thường lệ và `pending:true`, tiến trình/lỗi muộn đi qua SSE (`event` SYSTEM/ERROR, rồi `snapshot`). Mỗi worker chỉ nhận một hành động như vậy cùng lúc; hành động thứ hai trong lúc đó nhận 409 `Đang xử lý yêu cầu trước, vui lòng chờ`. Gateway không giữ khoá toàn cục khi proxy tới worker; xoá truyện trong lúc truyện còn yêu cầu đang chạy trả 409 thay vì xếp hàng.

Lời gọi model phía host cho các hành động đó (chuẩn hoá quy tắc, phán định khởi động/can thiệp) có hạn **2 phút** mỗi lượt và bị `/abort` huỷ khi engine chưa chạy. Nhà cung cấp đã cấu hình nhưng không kết nối được (từ chối kết nối, DNS, lỗi mạng) không được thử lại: hành động thất bại ngay với lỗi bắt đầu bằng `Không kết nối được nhà cung cấp AI — kiểm tra kết nối và cấu hình nhà cung cấp (…nguyên nhân…)`; chuẩn hoá quy tắc vẫn lùi về chế độ giảm cấp như thiết kế. Lỗi trả lời của nhà cung cấp (429…) vẫn được thử lại trong hạn đó. `snapshot` của worker có thêm `actionPending:true` khi còn hành động đang xử lý; xoá truyện coi đó là đang chạy.

Thiếu/sai cấu hình nhà cung cấp không chặn đọc: worker mở Host chỉ đọc dữ liệu thật, không dựng model giả. Snapshot, SSE live, nhật ký, chương và chẩn đoán cục bộ vẫn hoạt động cho mọi vai trò có quyền đọc. Hành động cần LLM trả 409 `{"error":"Chưa có cấu hình mô hình — thiết lập ở Cài đặt › Nhà cung cấp","needsConfig":true}`. `config.changed` theo phạm vi đóng các worker chưa có model; lời gọi kế tiếp nạp lại cấu hình và dựng engine bình thường khi cấu hình đã hợp lệ. Engine đã có model không bị ngắt.

### Trường bổ sung (G3, G4, G5)

- `HostEvent` (mọi event trong SSE `event` và trong danh sách sự kiện) thêm `sessionId: string` — tăng mỗi lần engine khởi chạy một phiên (`start`/`resume`/`continue`/`reopen`/`import`); rỗng với sự kiện phát sinh trước phiên đầu tiên. UI nhóm nhật ký theo `sessionId`.
- Event retry (`retryAt` khác rỗng) thêm `canRetryNow: boolean` — `true` khi vòng chờ thuộc wrapper model có thể đánh thức bằng `POST /retry`; `false` với retry nội bộ agentcore giữa luồng (không đánh thức được). UI chỉ hiện nút “Thử lại ngay” khi `true`.
- `Snapshot` thêm `waitingFor: '' | 'gate' | 'cocreate' | 'confirm' | 'input'` — `gate` = cổng duyệt/advance hold, `cocreate` = đang trong phiên đồng sáng tác, `confirm` = import đang chờ xác nhận, `input` = đang rảnh chờ lệnh. Chip: đang chạy → “Đang chạy”; `waitingFor` khác rỗng → “Đang chờ bạn”.
- `POST /retry {}` → `{ok:true}`: đánh thức ngay vòng chờ backoff của model (sự kiện có `retryAt`). Không có lượt nào đang chờ → 409 `Không có lượt nào đang chờ thử lại`. Quyền ghi.

SSE bổ sung loại sự kiện:
| Event | Data |
|---|---|
| `cocreate` | `{text: string}` — delta trả lời đồng sáng tác |
| `job` | `JobEvent` — tiến trình việc dài (import, simulate, sync, export) |

### Đồng sáng tác (GAP-6)

| Method & path | Thân | Trả về |
|---|---|---|
| `GET /cocreate` | — | `{active: boolean, messages: {role:'user'\|'assistant', content}[], draft: string, ready: boolean, suggestions: string[]}` |
| `POST /cocreate/message` | `{message}` | `{reply, draft, ready, suggestions}` (stream delta qua SSE `cocreate`). Truyện chưa bắt đầu: hội thoại trước khi viết. Truyện đang có: tự `PauseForCoCreate` lần đầu |
| `POST /cocreate/commit` | `{draft?: string}` | `{ok, action: 'start'\|'resume'}` — chưa bắt đầu → bắt đầu viết với bản nháp; đang có → `ResumeFromCoCreate` |
| `POST /cocreate/cancel` | `{}` | `{ok}` |

#### Thẻ đề xuất đồng sáng tác (18-mobile M5)

`GET /cocreate`, `POST /cocreate/message` và `POST /cocreate/commit` bổ sung:

```ts
type CoCreateProposal = {
  schemaVersion: 1;
  title: string;
  revision: number;
  chapters: {schemaVersion: 1; chapter: number; title: string; summary: string}[];
  impacts: string[];
};
// Các trường bổ sung ở cấp phản hồi:
// {schemaVersion:1, proposal:CoCreateProposal|null, committed:boolean, nextChapter:number|null}
```

- Đề xuất do **model thật** trả trong thẻ XML `<proposal>` sau `<suggestions>`: `<title>`, `<chapters><chapter><number>…</number><title>…</title><summary>…</summary></chapter></chapters>`, `<impacts><impact>…</impact></impacts>`. Hai prompt (trước khi viết/theo giai đoạn) đều yêu cầu cùng cấu trúc. Tên, tóm tắt một câu và chip tác động lấy từ XML, không suy diễn từ văn bản trả lời hay tạo nội dung mẫu. Model phải cập nhật `<draft>` nhất quán để giao đúng chỉ dẫn cho engine.
- Chưa đủ dữ kiện, thiếu trường, XML đề xuất lỗi, số chương không dương hoặc không tăng dần → không có đề xuất mới; `proposal:null` nếu chưa từng có đề xuất hợp lệ, nếu không giữ đề xuất hợp lệ gần nhất. Tên/tóm tắt/tác động được chuẩn hoá khoảng trắng thành một dòng; XML entity được giải mã. Phản hồi tự do vẫn giữ hành vi trả lời cũ.
- Phiên tự gán `revision:1` cho đề xuất đầu, tăng một mỗi lần nhận đề xuất hợp lệ tiếp theo sau góp ý (UI: “Đã chỉnh v2”); không tin revision do model gửi. `nextChapter` là số chương đầu trong đề xuất, `null` khi chưa có đề xuất.
- Chốt thành công vẫn chạy `PrepareUserRules`/`StartPrepared` hoặc `ResumeFromCoCreate` thật, trả `committed:true` cùng đề xuất. `GET` sau chốt có `active:false`, giữ hội thoại, bản nháp, đề xuất và revision để tải lại thẻ đã chốt; chốt lần nữa → 409. Tin nhắn mới sau chốt mở phiên mới.
- Lưu nguyên tử tại `<storyDir>/.ainovel/cocreate/session.json` với `schemaVersion:1`, gồm phiên/hội thoại XML gốc/bản nháp/đề xuất/revision và trạng thái đã chốt. Worker đọc lười khi truy cập đầu tiên; phiên theo giai đoạn chưa chốt khôi phục cả trạng thái tạm dừng của Host. Huỷ xoá tệp và đề xuất; `POST /cocreate/cancel` thêm `schemaVersion:1`. Lỗi đọc/ghi/xoá hoặc phiên bản tệp không hỗ trợ trả 500 với `error` tiếng Việt, không giả báo thành công.

### Việc dài & lệnh TUI (GAP-8)

```ts
type JobEvent = { job: string /* id */; kind: 'import' | 'simulate' | 'importsim' | 'sync' | 'export'; stage: string; current: number; total: number;
  message: string; level: 'info' | 'warn' | 'error' | 'success'; retryAt: string | null; done: boolean; error: string; awaiting: '' | 'confirm' | 'storyStatus' }
```

| Method & path | Thân | Trả về |
|---|---|---|
| `POST /import` | `{path: string /* tương đối thư mục truyện hoặc dự án; tuyệt đối phải nằm trong root */, storyStatus?: 'open'\|'closed', continueAfter?: boolean, guide?: string, autoConfirm?: boolean}` | `{job}`; `path` rỗng = tiếp tục lần import dở |
| `POST /import/confirm` | `{accept: boolean, storyStatus?: 'open'\|'closed'}` | `{ok}` |
| `POST /import/preview` | `{path}` (cùng quy tắc sandbox như `/import`) | `{chapters: {n, title, words, firstLine}[], warnings: string[], encoding: 'utf-8'\|'utf-8-bom'\|'gb18030'}` — chỉ đọc, không ghi dữ liệu (G9) |
| `GET /import` | — | `{resumeHint: string, job: JobEvent\|null}` |
| `POST /simulate` | `{dir?: string /* mặc định ./simulate */}` | `{job}` |
| `POST /importsim` | `{path}` | `{job}` |
| `POST /sync` | `{check: boolean}` | `check=true`: `{changed: {chapter,title,kind,added,removed,modifiedAt}[]}` (không ghi chương); `false`: `{job}`. Kết quả từng chương qua `JobEvent.sync`. |
| `GET /diag` | — | `{generatedAt,stats,findings,actions,sections}` — hình dạng GAP-28 bên dưới |
| `POST /start-file` | `{path}` | `{ok, action:'start'}` |
| `GET /models` | — | `{roles: {role: 'default'\|'architect'\|'writer'\|'editor', provider, model, thinking, thinkingOptions: string[]}[], providers: {provider, label /* tên ngắn của preset */, ready /* có key, không cần key, hoặc OAuth đã đăng nhập */, models: {name, contextWindow, thinking: string[] /* mức model hỗ trợ; '' = Mặc định */}[]}[]}` — bảng `/model` (14 §2) dùng `providers` để chỉ hiện mức suy luận model chọn hỗ trợ trước khi lưu |
| `POST /models` | `{role, provider, model}` | `{ok}` (đổi lúc chạy, ghi cấu hình như `/model`) |
| `POST /thinking` | `{role, level}` | `{ok}` |
| `POST /command` | `{line: string /* bắt đầu bằng "/" */}` | `{ok:true,message?,result?:CommandOutcome}` (G13/GAP-26…29). Registry dùng chung (`internal/commands`) kiểm tra cú pháp/quyền trạng thái; lỗi cú pháp 400, đang chạy hoặc sai màn 409. `/model` và `/diag` trả `panel`; việc dài trả `job`; `/sync --check` trả `syncCheck`. |

#### G9 · Nhập truyện trên điện thoại (18-mobile M5)

Bổ sung dưới đây thay thế mô tả preview chỉ quét tiêu đề/không ghi workspace ở bảng cũ: host thật chạy **Kiến trúc sư thật** để chia chương và lưu bản chụp nguồn + kết quả trong `meta/import/`; chưa ghi chương, nhân vật hay tiến độ vào truyện chính thức. Kết quả xem trước và lúc xác nhận dùng **cùng artifact chia chương đã kiểm tra bao phủ**, không gọi model chia lần nữa sau khi bấm đồng ý.

```ts
type ImportPreview = {
  schemaVersion:1; filename:string; size:number; words:number; splitHow:string;
  chapters:{n:number; title:string; words:number; firstLine:string; newlySplit:boolean; warning?:string}[];
  warnings:string[]; encoding:'utf-8'|'utf-8-bom'|'gb18030';
};
type ImportOptions = {schemaVersion:1; storyStatus:'open'|'closed'; continueAfter:boolean; guide:string};
type ImportSummary = {schemaVersion:1; chapters:number; characters:number; foreshadows:number};
type ImportProgress = {
  schemaVersion:1;
  steps:{schemaVersion:1; id:'ingesting'|'segmenting'|'analyzing'|'synthesizing'|'publishing';
    title:string; current:number; total:number; status:'pending'|'running'|'done'}[];
  etaSec?:number; preview?:ImportPreview; summary?:ImportSummary; continued:boolean;
};
```

- `POST /import/preview {path, guide?:string}` → `ImportPreview`. `size` tính byte nguồn, `words` là số từ cách nhau bởi khoảng trắng, `splitHow` là mô tả tiếng Việt. `path` không rỗng vẫn phải qua sandbox; `path:''` dùng bản chụp nguồn đang nhập để chia lại sau khi tải lại trang hoặc tệp gốc đã di chuyển. Chưa có workspace thì phải đưa đường dẫn nguồn. Quá trình dùng chung khoá độc quyền nhập; không chạy song song với engine hay lần nhập khác.
- Preview cần host có mô hình và engine sẵn sàng; host chỉ đọc/chưa cấu hình trả lỗi tiếng Việt yêu cầu thiết lập nhà cung cấp trước khi dùng khoá engine hay gọi Kiến trúc sư, không rơi vào panic hoặc tạo chương mẫu.
- Preview là yêu cầu đồng bộ có giới hạn **20 giây** (tính cả mọi khối chia và lần hỏi lại sửa JSON của Kiến trúc sư); hết hạn hoặc trình duyệt huỷ yêu cầu sẽ huỷ luôn lời gọi model, không để goroutine chạy nền. Mỗi lời gọi model của preview chỉ thử một lần với nhà cung cấp đã chọn, không chuyển fallback, không chờ thử lại; nhà cung cấp không kết nối được hay quá hạn → 400 `Không kết nối được nhà cung cấp AI — …` kèm nguyên nhân gốc. Bản chụp nguồn và `guide` đã lưu vẫn giữ để thử lại bằng `path:''`; không tạo preview giả. Việc nhập nền (`POST /import`, xác nhận) vẫn giữ fallback/thử lại như cũ.
- `guide` là hướng dẫn bằng lời được gửi nguyên nội dung có dấu cách cho Kiến trúc sư, lưu bền cùng nguồn; thay đổi hướng dẫn làm artifact cũ mất hiệu lực và chia lại thật. Cả hai chương con của một chương vừa tách có `newlySplit:true`; các nhãn được giữ khi tải lại hoặc xem lại cùng kết quả. Không tạo chương giả để mô phỏng chia lại.
- Một chương có số từ **≥ 2 × trung bình toàn bộ chương** nhận `warning` tiếng Việt và cùng nội dung trong `warnings`; so sánh bằng tổng/số chương, không làm tròn trung bình.
- `GET /import` bổ sung `{schemaVersion:1,preview:ImportPreview|null,path:string,options:ImportOptions}` bên cạnh `resumeHint`/`job`. `path` là tên tệp để hiển thị; để chia lại nguồn đang nhập, gửi `path:''`. Trạng thái dựng lại từ workspace bền sau khi kết nối lại, không phụ thuộc sự kiện SSE còn trong bộ nhớ.
- `POST /import/confirm` nhận thêm `{continueAfter?:boolean,guide?:string}`. Lựa chọn `storyStatus` của người dùng ưu tiên hơn kết luận open/closed của model và được lưu để khôi phục. `continueAfter:false` được phân biệt với bỏ trống, có thể tắt lựa chọn viết tiếp đã bật; bỏ trống giữ ý định đã lưu. `guide` nếu có phải khớp hướng dẫn của preview đang lưu; khác → 409 `Hãy chia lại và xem trước hướng dẫn mới trước khi xác nhận`, không tự nhận kết quả chưa xem.
- `/import … --story=open|closed [--continue] --guide=<lời hướng dẫn>`: `--guide=` luôn ở cuối, toàn bộ phần còn lại kể cả dấu cách thuộc hướng dẫn, không còn được phân tích thành cờ. `--continue` dùng cơ chế tiếp nối engine thật sẵn có (chế độ duyệt từng chương vẫn chờ người dùng); không bật thì giữ điểm dừng nghiệm thu sau nhập.
- `JobEvent` của import bổ sung `import?:ImportProgress`. Năm bước lần lượt: **Đọc tệp**, **Chia chương**, **Tóm tắt từng chương**, **Dựng nhân vật & bối cảnh**, **Lập dàn ý & phục bút**. Số đếm theo sự kiện thật của pipeline (ví dụ 13/24), không bị xoá khi chờ thử lại. `etaSec` chỉ xuất hiện khi đã đo được tiến độ phân tích chương, không dùng thời gian mẫu. `summary` cuối lấy số chương đã xuất bản, số nhân vật đã lưu và số mục phục bút trong sổ cái thật. `continued` chỉ true khi Host đã khởi động tiếp nối thành công.
- `Job.result.data` giữ cùng `ImportProgress` cả khi đang chờ xác nhận và khi kết thúc, để liên kết việc nền vẫn mở được preview/tổng kết ngay cả khi không còn sự kiện workspace.


Tất cả việc dài phát `job` qua SSE và kết thúc bằng `done: true`.

### Xuất tự động (GAP-4)

Worker đọc `LibraryConfig.autoExport` (từ `AINOVEL_LIBRARY_ROOT`) khi khởi động và theo dõi engine: nộp chương (`onChapter`), xong một tập (`onVolume`), truyện hoàn thành (`onBook`) → xuất theo `formats`/`pattern`/`onExists` vào `<truyện>/<dirs.exports>/`. Mỗi lần xuất phát sự kiện `SYSTEM` mức `success`: `Đã xuất <tên tệp> (<n> chương)`; lỗi → mức `warn`. TUI chạy ngoài thư viện (không có env) không xuất tự động.

### Việc chạy nền & kết quả lệnh (GAP-26…29)

**GAP-26.** `GET /api/jobs` trả `{jobs:Job[]}` của mọi truyện tài khoản được đọc: gom worker đang chạy (không khởi động worker chỉ để xem dock) và nguồn ở gateway như audio/xuất tệp. `POST /api/jobs/:id/{pause|resume|cancel|retry}` cần quyền ghi truyện, trả `{ok:true}`, **404 khi chỉ có quyền đọc** (ẩn, như việc không tồn tại/không được đọc), 409 thao tác không được nguồn hỗ trợ; `cancel` một việc đã xong chỉ bỏ nó khỏi danh sách. ID gồm bốn phần `<project>/<story>/<kind>/<n>`; phần số là định danh trong tiến trình, không phải số chương.

```ts
type Job = {
  id:string; kind:'write'|'audio'|'export'|'sync'|'simulate'|'import'|'diag'
  title:string; status:'running'|'paused'|'waiting'|'failed'|'done'
  progress:number|null; step?:string; unit?:{done:number,total:number,label:string}
  etaSec?:number; error?:string
  result?:{title:string,sub:string,actions:{label:string,kind?:'download'|'reveal'|'open',href?:string,path?:string,primary?:boolean}[],data?:unknown}
  href:string; startedAt:string; finishedAt?:string
  project:string; story:string; agent?:string
}
type CommandOutcome =
  | {kind:'panel',panel:'model'|'diag',role?:string}
  | {kind:'job',job:string,jobKind:'export'|'sync'|'simulate'|'import'|'importsim',path?:string}
  | {kind:'syncCheck',changed:SyncChanged[]}
  | {kind:'exportExists',exists:ExportConflict} // kèm phản hồi 409
```

Chỉ có **một luồng toàn cục `GET /api/events`** (§10), không có `/api/jobs/events`. `job.created|job.progress|job.waiting|job.failed|job.done` mang toàn bộ `Job`; `job.removed` mang `{id}`. Việc xong giữ 10 phút; import cần xác nhận và cổng duyệt chương mang `waiting`. Luồng workspace theo truyện vẫn phát `JobEvent` để thẻ kết quả giữ dữ liệu chi tiết (`export?`, `sync?`, `simulate?`). `job` trong `CommandOutcome`/`JobEvent` là ID cục bộ `<kind>/<n>`; gateway thêm tiền tố dự án/truyện khi đưa vào dock.

`POST S/abort {afterStep:true}` yêu cầu **tạm dừng sau bước agent hiện tại**: không huỷ model/tool đang thực hiện; dừng trước bước tiếp theo, sau khi lưu kết quả bước hiện tại. `{ok:true,stopped:true}` xác nhận đã nhận yêu cầu; snapshot vẫn đang chạy cho tới khi thực sự dừng. Bỏ `afterStep` hoặc `false` giữ dừng ngay. `write` job `pause` dùng cùng cơ chế; `resume` gọi Host.Resume và xoá yêu cầu dừng sau bước. `S` là tiền tố truyện.

**GAP-27.** `POST S/exports` nhận thêm `onExists:'replace'|'keep'|'fail'` (mặc định `fail`). `replace` ghi đè; `keep` thêm thời gian `-YYYYMMDD-HHmmss` trước phần mở rộng (thêm số nếu vẫn trùng); `fail` không ghi và trả 409:

409 do trùng tệp là xung đột kiểm tra trước, **không tạo việc thất bại và không phát `job.failed`**. Mọi `path` trong xung đột (`path`, `existing.path`, `incoming.path`) tương đối với truyện; lỗi xuất, `Job.error` và kết quả cũng không chứa đường dẫn tuyệt đối máy chủ.

```ts
type ExportConflict = {
  path:string; format:'txt'|'epub'
  existing:{path:string,size:number,modTime:string|null,chapters:number|null,bytes:number,modifiedAt:string|null}
  incoming:{path:string,size:number,chapters:number,skipped:number[],bytesEstimate:number|null}
}
// JSON lỗi: {error,existing,incoming,result:{kind:'exportExists',exists:ExportConflict}}
```

`/command /export [đường-dẫn] [from=N] [to=M] [--overwrite|--keep]` dùng cùng chiến lược (`--overwrite` = `replace`) và chạy thành việc nền. `result.path` là đường dẫn đích tương đối truyện. Kết quả `JobEvent.export` gồm `{path,name,title,format,bytes,chapters,skipped}`; `GET S/export/file?path=` tải tệp trong truyện. `POST S/exports` giữ phản hồi `{path,bytes,chapters,skipped}` và thêm `job` (ID toàn cục); tải tệp đường này qua `S/exports/file?path=`.

**GAP-28.** `GET S/diag` lấy chẩn đoán từ Host, trả `generatedAt`, `stats` (số chương/chữ, giai đoạn, luồng, quy hoạch, số lần/điểm thẩm duyệt, phục bút, thời điểm bắt đầu, tập hiện tại/tổng tập), `findings:[{fingerprint,rule,severity,category,confidence,autoLevel,target,title,evidence,suggestion}]`, `actions:[{fingerprint,sourceRule,kind,severity,summary,message,sent}]` và `sections` cho bảng tổng quan. Fingerprint ổn định theo quy tắc/đích/tiêu đề/bằng chứng. `POST S/diag/actions/:fingerprint {}` gửi thông điệp gợi ý cho trợ lý (can thiệp khi đang chạy, tiếp tục khi dừng); gửi lại trong cùng worker là idempotent. `POST S/diag/export {}` tải Markdown đã ẩn thông tin nhạy cảm (`Content-Disposition: attachment`).

`/command /diag` chỉ mở bảng; GET `/diag` là lượt chẩn đoán duy nhất tạo việc `diag`. `GET S/diag/last` đọc báo cáo đã lưu trong worker mà không chạy lại (404 nếu chưa có). Nút tải dùng được cả GET và POST `/diag/export`. `GET S/simulate/profile` trả hồ sơ tổng quan; `?download=1` tải tệp JSON hồ sơ engine. Mọi đường dẫn xuất/nhập so sánh theo đường dẫn thật, kể cả root là liên kết tượng trưng, và không cho liên kết trong thư viện dẫn tệp ra ngoài.

**GAP-29.** `/sync --check` và `POST S/sync {check:true}` chỉ quét, tạo một việc kiểm tra trong dock và trả `SyncChanged[] = {chapter,title,kind:'edit'|'resume',added,removed,modifiedAt}`. `added`/`removed` tính số từ thêm/bớt so với bản đã tiếp nhận; `modifiedAt` là mtime chương hoặc null. `GET S/sync/diff/:chapter` trả `{chapter,title,before,after}`. Đồng bộ thật giữ kết quả `revision.Result` theo từng chương ở `JobEvent.sync` và `Job.result.data`: `{applied:number[],analyses:{chapter,title,changeSummary,storyChanged,downstreamIssues:{text,chapter:number|null}[]}[]}`. Lỗi không che giấu phần kết quả đã nhận.

**M5 · mở lại, đồng bộ và liên kết việc nền (schemaVersion 1).**

- `GET S/reopen` chỉ đọc, trả `{schemaVersion:1,title,chapters,words,lockedAt,closedAt,allowed,checks:[{id,ok,label,note,chapters:number[]}]}`. `lockedAt`/`closedAt` là ISO-8601 hoặc null nếu truyện cũ chưa ghi thời điểm hoàn thành; không suy đoán từ mtime. Các điều kiện gồm truyện đã hoàn, hệ thống rảnh (không đồng sáng tác/tác vụ độc quyền), và `N/M chương đã duyệt sạch`; `chapters` trong điều kiện chứa liên kết chương còn sửa ngoài app hoặc chờ viết lại. `POST S/reopen` kiểm tra lại cùng điều kiện trước khi mở; các chương cũ không thay đổi. `meta/progress.json` ghi `completion_version:1,locked_at` khi hoàn thành, giữ nguyên thời điểm khi đánh dấu hoàn thành lặp lại và xoá `locked_at` khi mở lại.
- `JobEvent.sync` và `Job.result.data` thêm `schemaVersion:1`; mỗi phần tử `analyses` thêm `schemaVersion:1,classification:'facts'|'wording'|'conflicts',conflictCount`. Có mâu thuẫn → `conflicts`; không có mâu thuẫn và `storyChanged` → `facts`; còn lại → `wording`. Mỗi `downstreamIssues` thêm `issueIndex,sent`; `chapter` chỉ được liên kết khi văn bản phân tích nêu rõ số chương có thật, nếu không là null.
- `POST S/sync/fix {chapter,issueIndex,job?}` gửi “Nhờ sửa” từ mâu thuẫn thật trong kết quả đã giữ; `job` là ID cục bộ `<kind>/<n>` (bỏ tiền tố dự án/truyện), không ghi thì dùng lượt đồng bộ gần nhất. Trả `{schemaVersion:1,ok:true,sent:true,action:'steer'|'continue'}`; gửi lại cùng mâu thuẫn là idempotent trong worker, lỗi gửi không đánh dấu đã gửi. Kết quả việc và sự kiện `job` được cập nhật để tải lại vẫn thấy “Đã gửi”; không tạo lại thời điểm hoàn tất việc. 400 chỉ số sai, 404 kết quả/mâu thuẫn không có, 409 trợ lý chưa nhận được.
- `GET /api/jobs/:id` (URL-encode toàn bộ ID toàn cục) trả `{schemaVersion:1,project,story,job:Job,event:JobEvent|null}`. Chỉ trả việc tài khoản được đọc; không có/không được đọc → 404. Không khởi động worker chỉ để tra việc; `event` là chi tiết đúng ID từ worker còn sống, null nếu nguồn không có sự kiện chi tiết. Các bước và kết quả đã có trong `job.result.data` vẫn xem được khi nguồn chi tiết không còn.
- “Phân tích lại” dùng **GET S/diag** mỗi lần để quét trạng thái hiện tại, không dùng `/diag/last`. Đây là đường đã có của GAP-28, chỉ đọc truyện; việc nền chẩn đoán và cache báo cáo được thay mới, không gọi LLM hay sửa chương.



## 6. Cấu hình & quy tắc (GAP-5, 10, 16)

### Kế thừa (GAP-10)

Chuỗi: `~/.ainovel/config.json` → `<dự án>/.ainovel/config.json` → `<truyện>/.ainovel/config.json`. Dự án = thư mục tổ tiên gần nhất của cwd có `project.json` (dừng ở home hoặc `/`). Quy tắc: `~/.ainovel/rules` → `<dự án>/.ainovel/rules` → `<truyện>/.ainovel/rules`; cùng tên tệp thì cấp gần hơn thắng, còn lại gộp theo tên. Áp dụng cho cả TUI và worker.

### Kiểu

```ts
type ConfigScope = 'global' | 'project' | 'story'
// Config = bootstrap.Config theo json tag gốc (providers, roles, style, budget, notify, context_window, advance_mode, ask_user).
// `language` **đã xoá** (14 §B.4): tệp cũ có khoá này → bỏ qua (giá trị zh ghi một slog.Warn), PUT gửi kèm → bỏ qua, không ghi lại.
// Mỗi provider: api_key KHÔNG bao giờ trả về; thay bằng api_key_set: boolean, api_key_hint: string ("sk-…abcd").
// Khi PUT: api_key vắng/rỗng = giữ khoá cũ; clear_api_key: true = xoá.
type ConfigDoc = { scope: ConfigScope; path: string /* hiển thị: admin rút gọn ~; người khác: global dạng ~/.ainovel/…, project/story tương đối gốc thư viện */; exists: boolean; config: Config }
type ConfigResponse = { target: ConfigDoc; effective: Config; chain: ConfigDoc[] /* toàn cục → dự án → truyện, chỉ các cấp áp dụng */;
  sources: Record<string /* đường dẫn khoá dạng "roles.writer.model" */, ConfigScope>;
  accounts: Record<string /* provider id */, {connected, label, email, plan, expiresAt}> /* chỉ admin, còn lại {} — G7, xem 14 */ }
type ProviderPreset = { id: string; label: string; type: string; baseURL: string; keyRequired: boolean; oauth: boolean; deviceOnly: boolean /* oauth chỉ có device-code, vd. xai-oauth */; pasteCode: boolean /* oauth nhận mã dán tay `code#state`/URL chuyển hướng, vd. anthropic-oauth */; api: 'chat' | 'responses' | ''; defaultModel: string /* G6, xem 14 */ }
```

| Method & path | Thân | Trả về | Quyền |
|---|---|---|---|
| `GET /api/config?scope=&project=&story=` | — | `ConfigResponse` | đăng nhập (người không phải admin: providers chỉ còn tên/type/models) |
| `PUT /api/config?scope=&project=&story=` | `Config` (toàn bộ nội dung tệp của cấp đó) | `ConfigResponse` | global: admin; project/story: admin hoặc có quyền ghi. Cấp project/story từ chối khoá `providers.*.api_key`, `tts.api_key`, `notify` (400). **409 `model_in_use`** (chỉ scope global, Gói AH): từ chối khi bản ghi bỏ một model đã có trong `providers.<id>.models` của tệp đang lưu (kể cả xoá cả provider) mà cấu hình mới vẫn tham chiếu — mặc định `provider`+`model`, `roles.<key>` (provider rỗng = provider mặc định), hoặc `roles.<key>.fallbacks[]`. Thân: `{error:"Đang dùng cho {nhãn…}. Đổi model của trợ lý đó trước khi bỏ chọn.", code:"model_in_use", provider, model, roles:[nhãn…]}` với nhãn `Mặc định` / `Kiến trúc sư` / `Người viết` / `Biên tập viên` / tên role khác nguyên văn (kèm ` (dự phòng)` cho fallback); không ghi gì. Model role đang dùng nhưng chưa từng nằm trong `models` đã lưu (cấu hình cũ) không chặn |
| `GET /api/config/presets` | — | `{providers: ProviderPreset[], styles: string[], reasoning: string[]}` (`languages` **đã xoá** — 14 §B.4). `styles` = 5 văn phong có sẵn (`default · romance · suspense · fantasy · wuxia`) + tên tệp `~/.ainovel/style/styles/*.md` do người dùng tự tạo (thẻ “Văn phong của bạn” ở Tuỳ chỉnh › Cách viết), sắp theo tên | đăng nhập |
| `POST /api/config/test` | `{provider, model?, config?: Config /* chưa lưu */}` | `{ok, latencyMs, message, model /* model đã dùng */, detail?, contextWindow?, contextSource?}` — thiếu `model` thì lấy `defaultModel` của preset (theo tên provider, rồi theo `type`). `config.providers[provider].api_key` vắng/rỗng ⇒ dùng khoá đã lưu ở cấp toàn cục (cùng quy ước với PUT). `message` khi `ok:false` luôn tiếng Việt: “Chưa có khoá API cho {nhà cung cấp} — dán khoá API rồi thử lại” · “Cấu hình chưa hợp lệ: …” · “Không kết nối được {nhà cung cấp} tại {scheme://host} — kiểm tra máy chủ đã chạy và địa chỉ đúng” · “{nhà cung cấp} không phản hồi trong 30 giây — …” · “{nhà cung cấp} từ chối khoá API ({phân loại}, HTTP n, provider, model) — kiểm tra lại khoá” · “Kết nối thất bại ({phân loại}, HTTP n, provider, model)”. `detail` (tuỳ chọn, chỉ khi `ok:false`): nguyên văn lỗi từ nhà cung cấp/hệ thống để xem chi tiết — không dùng làm thông báo chính. Khi `ok:true` kèm `contextWindow` + `contextSource` (`model_config` \| `provider` \| `registry` \| `default`): cửa sổ ngữ cảnh phân giải cho provider+model vừa thử theo bản nháp — `context_window` trong `providers.<id>.models` › giá trị provider báo trong danh sách đã cache (`POST /models/list`) › danh mục Noverse › 200000 | admin |
| `GET /api/config/models?provider=` | — | `{models: {name, displayName, contextWindow}[]}` (OAuth provider: từ `codex.ListModels` / `xai.ListModels` / `anthropicoauth.ListModels`; còn lại: models trong cấu hình) | admin |
| `POST /api/config/models/list` | `{provider, config?: Config /* bản nháp chưa lưu, như /config/test */, refresh?: boolean}` | **Luôn 200** — lỗi nằm trong thân: `ProviderModelList {models: ProviderModel[], total, fetchedAt /* RFC3339, cache hit giữ giờ lần lấy đầu; rỗng khi lỗi */, unsupported?: true, error?: 'key_rejected'\|'unreachable'\|'missing_key'\|'not_logged_in', message?: string /* tiếng Việt, luôn có khi error */, detail?: string /* lỗi thô upstream */}`. `ProviderModel {name /* id ghi vào providers.<id>.models[] */, displayName /* = name nếu provider không cho */, description?, contextWindow /* luôn > 0 */, contextSource /* model_config=‹draft› · provider=‹API báo› · registry=‹danh mục Noverse› · default=200000 */, registryWindow /* 0 = danh mục không biết */, price?: {input, output} /* USD/1M token, chỉ khi API trả — OpenRouter */, recommended /* có entry đúng tên trong danh mục Noverse hoặc = defaultModel của preset */}`. Gọi API danh sách của chính provider theo **type hiệu dụng** (kể cả proxy tuỳ chỉnh): kiểu OpenAI `GET {base}/models` Bearer (openai · deepseek · qwen · glm · grok · mimo · minimax · opencode-go · type `openai`); OpenRouter `GET {base}/models` (`context_length`, `pricing` USD/token → ×1e6); Anthropic `GET {base}/v1/models` + `x-api-key`/`anthropic-version: 2023-06-01`, phân trang `has_more`/`last_id`; Gemini `GET {base}/v1beta/models` + `x-goog-api-key`, bỏ tiền tố `models/`, chỉ giữ `supportedGenerationMethods` chứa `generateContent`, `inputTokenLimit`, phân trang `nextPageToken`; Ollama `GET {host}/api/tags` (bỏ `/v1` cuối base). Gói thuê bao (openai-codex · xai-oauth · anthropic-oauth) dùng lại `oauthModelList`, chưa đăng nhập → `not_logged_in`; bedrock → `unsupported:true`. Upstream 401/403 → `key_rejected`; mạng/timeout/lỗi khác → `unreachable`; thiếu khoá bắt buộc → `missing_key`. Cache RAM 10 phút theo (provider · type · base URL · sha256 khoá), không cache lỗi; `refresh:true` bỏ cache. Timeout upstream ~20s, giới hạn kích thước phản hồi | admin (403 `{"error":"Không có quyền"}`) |
| `GET /api/config/account?provider=openai-codex` | — | `{connected, label, email, plan, expiresAt}` | admin |
| `POST /api/config/oauth/start` | `{provider, device: boolean}` | `{id, mode: 'browser'\|'device'\|'paste', url, userCode, verifyURL, expiresAt}` | admin; `device=false` chỉ khi request từ loopback (400 nếu không); provider `deviceOnly` (`xai-oauth`) luôn chạy device-code, bỏ qua `device`; provider `pasteCode` (`anthropic-oauth`) không bao giờ trả 400 vì không loopback: `device=true` hoặc request từ xa ⇒ `mode:'paste'` (`url` = link claude.ai, `userCode` rỗng, không mở cổng 54545), còn lại `mode:'browser'` (callback `localhost:54545` và vẫn nhận mã dán) |
| `GET /api/config/oauth/status?id=` | — | `{state: 'pending'\|'done'\|'failed', mode, account: {label,email,plan}\|null, error: string, models: {name,displayName,contextWindow}[]}` | admin |
| `POST /api/config/oauth/cancel` | `{id}` | `{ok}` | admin |
| `POST /api/config/oauth/complete` | `{id, code}` — `code` là chuỗi `code#state`, URL chuyển hướng đầy đủ hoặc `code=…&state=…` | `{ok}` — 400 (thông báo tiếng Việt) nếu mã trống/sai state/phiên không nhận mã dán (luồng vẫn chờ lần dán tiếp), 404 phiên không tồn tại, 409 phiên đã kết thúc | admin; chỉ phiên của provider `pasteCode` |
| `POST /api/config/oauth/logout` | `{provider}` | `{ok}` | admin |
| `GET /api/rules?scope=&project=&story=` | — | `{chain: {scope, dir: string /* như ConfigDoc.path */, exists: boolean, files: {name, size, updatedAt}[]}[]}` | đăng nhập |
| `GET /api/rules/file?scope=&project=&story=&name=` | — | `{name, text}` | |
| `PUT /api/rules/file` | `{scope, project?, story?, name, text}` | `{name, size, updatedAt}` | như PUT config; `name` `^[\p{L}\p{N}._-]+\.md$` |
| `POST /api/rules/file/delete` | `{scope, project?, story?, name}` | `{ok}` | |
| `POST /api/rules/understand` | `{text, scope?, project?, story?}` (`scope/project/story` chỉ chọn chuỗi cấu hình model, mặc định toàn cục) | `{structured: {genre, forbidden_chars, forbidden_phrases, fatigue_words}, preferences, uncertain: string[], options: string[], chips: Chip[], status: 'ready'\|'degraded'}` — `options` (0–3) là các cách hiểu: mỗi mục là một quy tắc đã viết lại rõ ràng bằng tiếng Việt (vd. “chương đừng quá dài” → `["Khoảng 3.000 chữ mỗi chương", "Ngắn hơn chương trước một chút"]`), chỉ có khi `uncertain` không rỗng, `[]` khi rõ ràng hoặc `degraded` — gọi `Normalizer.Normalize`, **không lưu gì**, timeout 2 phút, dừng khi client ngắt kết nối. Chuẩn hoá lỗi KHÔNG phải lỗi HTTP: trả 200 với `status:'degraded'`, `preferences` = nguyên văn, `chips` chỉ gồm chip `pref`. | quyền ghi quy tắc của cấp đó (như `PUT /api/rules/file`); 400 `Hãy nhập quy tắc cần kiểm tra` (rỗng) / `Quy tắc quá dài (tối đa 2000 ký tự)`; **409 `{error, needsConfig:true}`** khi chưa có model (cùng thông báo `needsModelMessage`); 504 `Mô hình phản hồi quá lâu, hãy thử lại` |
| `GET /api/system` | — | `{version, root /* admin: tuyệt đối ("" khi chưa chọn nơi lưu); người khác: "" */, rootDisplay /* "Nơi › … › Thư mục" */, retries: 7, deadlock: [3,5], compactThreshold: 0.85, promptCache: true, outputDir: "./output/novel", defaultContextWindow: 200000, canReveal: boolean /* chỉ admin, loopback và không có X-Forwarded-For/Forwarded — G1, xem 14 */, lan: boolean /* điện thoại cùng Wi-Fi có vào được không — GAP-33, xem §7 */}` | đăng nhập |
| `GET /api/reveal?target=&project=&story=&scope=` | — | `{path: string /* admin: tuyệt đối; người khác: tương đối gốc thư viện, ví dụ "proj/kich_ban/01-story/xuat-ban" */, canReveal: boolean /* như /api/system */}` — chỉ phân giải đường dẫn, không mở | đọc đối tượng; `target=rules`/`target=config` + `scope=global` và `target=library` cần admin (G1, xem 14) |
| `POST /api/reveal` | `{target: 'library'\|'project'\|'story'\|'exports'\|'activity'\|'audio'\|'rules'\|'config', project?, story?, scope?: 'global'\|'project'\|'story'}` (`config` = tệp `config.json` của cấp `scope`; bỏ `scope` thì suy ra từ project/story, không có cả hai → `global`) | `{ok: true, path}` | chỉ admin (403 `Không có quyền`, không tạo thư mục); chỉ loopback (không qua proxy): khác → 403 `Chỉ mở được thư mục khi dùng ngay trên máy chủ`; 400 target sai; 404 thiếu dự án/truyện (G1, xem 14) |
| `POST /api/system/pick-folder` | `{start?: string}` | `{path}` hoặc `{cancelled: true}` | chỉ admin (403); chỉ loopback (403); 501 khi máy chủ không có trình chọn thư mục (darwin `osascript`, linux `zenity`/`kdialog`) — G2, xem 14 |

`advance_mode` (`"auto"\|"review"`, mặc định `auto`): khoá mới trong `Config`, worker áp dụng khi tạo truyện mới.

`ask_user` (`bool`, mặc định `false`, 27/10): khoá trong `Config`, kế thừa 3 cấp như `advance_mode` (truyện ghi đè dự án ghi đè toàn cục; con trỏ bool nên `false` ở cấp gần cũng là một giá trị). Bật → Trọng tài được dừng hỏi người dùng (quyết định `ask` trong hợp đồng can thiệp). Tắt → prompt bỏ luật `ask`; model vẫn trả `ask` thì máy chủ từ chối, tự chọn phương án `is_default` (không có thì lấy phương án đầu), coi `reply` của nó như lời người dùng rồi phán định lại, và ghi nhật ký "Trọng tài tự chọn: … (vì bạn đã tắt Hỏi khi phân vân)". `snapshot.pendingAsk` không bao giờ được đặt khi cờ tắt.

Trang callback OAuth (`core/auth/oauth.go`): thiết kế A4 (`OAuth.dc.html`), `html/template`, 4 trường hợp `success · denied · stateMismatch · missingCode`, biến thể `from=cli|web` (web: `?from=web` truyền qua `state` phía server, không lộ ra URL). Icon inline của trang so với bản chép `core/auth/testdata/icons`; test `TestCoreOAuthIconsMatchDesignFiles` (`internal/entry/web`) giữ bản chép trùng từng byte với `docs/design/v1/icons`.

### Quy tắc của truyện (GAP-32, 21 §Gói D)

Các endpoint `S/rules/*` chạy **trực tiếp trong gateway** (như audio/xuất bản, không cần worker sống), đọc/ghi `<truyện>/output/novel/meta/*` và `<truyện>/.ainovel/rules/`. Model chuẩn hoá lấy từ chuỗi cấu hình của truyện.

```ts
type Chip = { kind: 'ban' | 'chr' | 'max' | 'genre' | 'pref'; text: string; limit?: number /* chỉ 'max': tối đa N lần/chương */ }
// ban = cụm từ cấm · chr = ký tự cấm · max = từ dễ lặp · genre = thể loại · pref = lời dặn cho trợ lý (không kiểm máy). Thứ tự: genre → ban → chr → max (theo chữ cái) → pref.
type RuleItem = { text: string; scope: 'global'|'project'|'story'; chips: Chip[]; uncertain: string[]; options: string[] /* 0–3 */; status: 'ready'|'degraded' }
type RuntimeRule = { id: string; text: string; chapter: number; createdAt: string; chips: Chip[]; uncertain: string[]; options: string[]; status: 'ready'|'degraded' }
type Violation = { rule: 'forbidden_chars'|'forbidden_phrases'|'fatigue_words'|string; target?: string; limit?: number; actual: number; severity: 'error'|'warning' }
```

| Method & path | Thân | Trả về | Quyền / lỗi |
|---|---|---|---|
| `GET S/rules/effective` | — | `{snapshot: Snapshot /* bản đã áp dụng; chưa áp dụng lần nào = chỉ system_defaults */, items: RuleItem[] /* từng dòng của mọi `quy-tac.md` (toàn cục → dự án → truyện), chuẩn hoá từng dòng, đọc từ tệp hiện tại nên có thể đi trước snapshot */, runtime: RuntimeRule[] /* luật “Học từ góp ý” do Trọng tài thêm */, timing: {effectiveFrom, inProgress, completed}}` | đọc truyện. Không cần model: dòng chưa chuẩn hoá được thì `status:'degraded'` (chip `pref`). Chuẩn hoá cache theo băm nội dung (`meta/user_rules_cache.json`, tối đa 500 mục, kết quả hạ cấp **không** cache) nên gọi lại không tốn model |
| `GET /api/rules/effective?scope=global\|project&project=` | — | `{snapshot, items: RuleItem[]}` — cùng hình dạng như `S/rules/effective` **bỏ `runtime`, `timing` và cấp truyện**: `items` là từng dòng `quy-tac.md` của cấp đó và các cấp cha (global → project), `snapshot` = mặc định + mọi tệp `.md` của các cấp đó (tính tại chỗ, không lưu). `scope` mặc định `global`; `story` → 400 (dùng `S/rules/effective`) | đăng nhập; `project` cần quyền đọc dự án (404 nếu không). Không bao giờ 409: chưa có model/model lỗi thì dòng `status:'degraded'`, `options:[]`. Đệm theo băm nội dung ở `~/.ainovel/rules-cache.json` nên GET lặp lại không tốn model |
| `POST S/rules/apply` | `{recheck?: boolean}` | `{snapshot, effectiveFrom: number /* chương đầu tiên dùng snapshot mới */, pinnedChapter?: number /* chương đang viết dở, giữ snapshot cũ đến khi xong */, jobId?: string /* khi recheck */}` | ghi truyện; **409 `needsConfig`** khi chưa có model; 504 khi quá 5 phút (không ghi gì); client ngắt → không ghi gì. Dựng lại snapshot = `system_defaults` + các tệp quy tắc (cache) + `startup_prompt` gốc (`RunMeta.StartPrompt`) + luật runtime; **giữ** runtime. Phát `story.changed {parts:['rules']}`. Chương đang viết dở: snapshot trước khi áp dụng được ghim cho chương đó (`meta/user_rules_pin.json`), `Load` trả bản ghim đến khi `in_progress_chapter` đổi; luật Trọng tài học thêm trong lúc đó vẫn áp vào cả bản ghim |
| `POST S/rules/apply {recheck:true}` | | như trên + `jobId` | Mở job `rules.recheck` (SSE `job.*`, `Job.kind='rules.recheck'`, `unit` “Chương n/N”, huỷ được bằng `POST /api/jobs/:id/cancel` — huỷ = gỡ khỏi danh sách) chạy nền: kiểm máy mọi chương đã lưu theo snapshot **mới** và ghi vi phạm từng chương (cùng nơi với `commit_chapter`). **Không bao giờ sửa chương.** Xong: `result.data = {checked, withViolations, violations}`, phát `story.changed {parts:['rules']}` |
| `POST S/rules/runtime/:id/keep` | — | `{ok, text, file: 'quy-tac.md'}` | ghi truyện; 404 `Không tìm thấy quy tắc này`. Nối dòng `- …` vào `<truyện>/.ainovel/rules/quy-tac.md` (bỏ qua nếu đã có dòng y hệt) rồi xoá khỏi runtime. Snapshot không đổi đến lần Áp dụng kế |
| `DELETE S/rules/runtime/:id` | — | `{ok}` | ghi truyện; 404 như trên. Luật biến khỏi snapshot ở lần Áp dụng kế |
| `GET S/rules/violations?last=3` | — | `{chapters: {chapter, at, violations: Violation[]}[]}` — `last` mặc định 3, 1–50 (400 nếu sai); lấy `last` chương gần nhất có bản ghi, **số chương giảm dần**, mỗi chương bản ghi mới nhất (mảng rỗng = đã kiểm và sạch) | đọc truyện |

Ghi chú:
- `quy-tac.md` là tệp duy nhất giao diện quản lý ở mỗi cấp (`PUT /api/rules/file` với `name:'quy-tac.md'` đã dùng được cho cả ba `scope`; mỗi dòng không trống/không bắt đầu bằng `#` là một quy tắc, tiền tố `- `/`* ` được cắt). Khác các tệp `.md` khác (cùng tên → cấp gần thắng), **`quy-tac.md` cộng dồn cả ba cấp** — không thì một dòng ở cấp truyện sẽ vô hiệu hoá toàn bộ quy tắc của dự án và “Mọi truyện”.
- **Gộp snapshot (sửa lỗi)**: `forbidden_phrases` và `forbidden_chars` là **hợp có khử trùng** của mọi nguồn (truyện thêm 1 cụm không còn xoá 4 cụm mặc định); `fatigue_words` vẫn theo từng từ, cấp gần hơn thắng; `genre` cấp gần hơn thắng. Áp dụng cho `BuildSnapshot` và luật Trọng tài (`OverlaySnapshot`).
- `options`: do bộ chuẩn hoá sinh cùng lúc với `uncertain` (schema `options: string[]` bắt buộc, prompt yêu cầu ≤ 3 mục, tự đứng được, không bịa con số). Server khử trùng/cắt còn 3 và bỏ hẳn khi `uncertain` rỗng. Lưu trong đệm chuẩn hoá (phiên bản đệm `v2`, đệm cũ tự bỏ).
- `commit_chapter` lưu vi phạm từng chương vào `meta/rule_violations.jsonl` (bản ghi mới nhất của chương thắng; rewrite sạch ghi mảng rỗng).
- Luật runtime lưu ở `meta/user_rules_runtime.json` `{rules:[{id,text,chapter,createdAt,candidate,legacy?}]}`. Snapshot cũ chưa có tệp này được **di trú**: mỗi `Sources` có `runtime_update` thành một mục (`id:'legacy-N'`, nguyên văn lấy từ các đoạn `## [runtime_update]` của `preferences`, chuẩn hoá lại ở lần Áp dụng kế; mục mất nguyên văn bị bỏ khi áp dụng).
- `story.changed` có thêm phần `rules`, cũng được phát khi `meta/user_rules*.json` hoặc `rule_violations.jsonl` đổi (ví dụ Trọng tài học luật mới trong worker).

## 7. Tài khoản bổ sung (GAP-14, 15)

- `User` thêm `onboardedAt: string | null`, `mustChangePassword: boolean`, `lastSeenAt: string | null` (lấy từ phiên mới nhất), `sessionCount: number` (chỉ trong `GET /api/users`).
- `mustChangePassword = true` khi admin tạo tài khoản trên web hoặc đặt lại mật khẩu cho người khác; xoá khi `POST /api/me/password` thành công. Tài khoản tạo bằng `serve user add` (CLI) = `false`.
- `POST /api/me {name}` → `{user}` (mọi vai trò, tên ≤ 60 ký tự).
- `POST /api/me/onboarded {}` → `{user}`.
- Đặt lại mật khẩu: `POST /api/users/:u {password}` như cũ.
- Frontend tài khoản dùng cache `users`, `sessions`, `me`: `account.changed` cập nhật danh sách tài khoản và thiết bị; `session.revoked {username,session}` làm mới phiên, đưa đúng phiên bị thu hồi về đăng nhập cùng thông báo. QR nhận phiên mới qua cùng luồng sự kiện, không dò thiết bị định kỳ.
- Các lệnh sửa hồ sơ, đổi mật khẩu và sửa tài khoản nhận phiên bản gốc qua `If-Match`; bản nháp chưa lưu không bị sự kiện ghi đè. Xung đột giữ nguyên bản nháp, cho xem/tải bản mới hoặc ghi đè bằng phiên bản mới. Xoá tài khoản/thu hồi thiết bị dùng xác nhận của kit; đăng xuất mọi thiết bị khác yêu cầu giữ nút xác nhận.

### Đăng nhập chung với Viderse (`STORE_DRIVER=postgres`)

Noverse và Viderse đọc cùng kho tài khoản (bảng `users`/`sessions`/`app_grants` của Noverse) và cùng cookie `verse_session`. Hợp đồng dưới đây giống hệt ở hai app; `STORE_DRIVER=file` giữ `ainovel_session` và không có `grants`.

- `User.grants: {noverse?: 'admin'|'member', viderse?: 'admin'|'member'}` là quyền **hiệu lực**: quản trị hệ thống luôn `admin` ở cả hai; thành viên chỉ có app được cấp. Mỗi request đọc lại từ DB, không cache trong phiên.
- Phân loại sau khi đọc cookie:

| Tình huống | Trả về | Client |
|---|---|---|
| Không có cookie, phiên hết hạn hoặc bị thu hồi | 401 | về đăng nhập |
| Phiên hợp lệ nhưng thiếu quyền app | 403 `{code:"app_access_denied", app, error:"Tài khoản của bạn chưa được cấp quyền dùng <App>. Vui lòng liên hệ quản trị viên.", user}` — giữ cookie | màn “chưa được cấp quyền” (Thử lại · Đăng xuất), không mở dữ liệu/SSE |
| Đang dùng mật khẩu tạm | 403 `{code:"password_change_required", mustChangePassword:true, error:"Bạn cần đổi mật khẩu tạm trước khi tiếp tục."}` cho mọi route nghiệp vụ, cả upload và SSE | cổng đổi mật khẩu, không có nút bỏ qua |
| Kho tài khoản không đọc được | 503, giữ cookie | báo lỗi + Thử lại, không giả đăng xuất/thiết lập |

- Thiếu quyền app vẫn dùng được đúng các route của chính mình: `POST /api/logout`, `POST /api/me/password`, `GET /api/me/sessions`, `POST /api/me/sessions/revoke-all`, `POST /api/sessions/:id/revoke` (chỉ phiên của mình, kể cả với admin). `GET /api/me` trả 403 `app_access_denied` kèm `user`.
- `POST /api/login` đúng mật khẩu nhưng thiếu quyền app: vẫn đặt cookie (để vào app được cấp) rồi trả 403 `app_access_denied`; không tính là lần sai.
- `POST /api/me/password` và `POST /api/me/sessions/revoke-all` → `{ok:true, reauthenticate:true}`: thu hồi **mọi** phiên của tài khoản ở cả hai app, kể cả phiên đang dùng, và xoá cookie. Admin đặt lại mật khẩu, khoá tài khoản hay đổi quyền app cũng thu hồi mọi phiên của người đó. Đăng xuất ở app nào thì app kia cũng 401 ở request kế tiếp.
- Quyền app (admin, chỉ ở Noverse): `PUT /api/users/:u/grants/:app {role:"member", reason?}` · `DELETE /api/users/:u/grants/:app {reason?}` → `User`. `role` khác `member` hoặc `app` lạ → 400. Không đổi gì → 200, không ghi nhật ký, không thu hồi phiên. Viderse không có HTTP sửa tài khoản; operator dùng `ainovel-cli serve user grant|revoke <tên> --app noverse|viderse` hoặc `viderse user grant|revoke`.
- `POST /api/users` (admin) luôn tạo kèm quyền `noverse`; hộp *Thêm thành viên* gọi tiếp `PUT`/`DELETE …/grants/:app` theo ô Noverse/Viderse đã chọn (mặc định chỉ Noverse).
- Request ghi (POST/PUT/PATCH/DELETE) bị 403 trước handler khi `Sec-Fetch-Site: cross-site`, hoặc khi `Origin` không phải origin của chính app hay một origin trong `VERSE_APP_ORIGINS` (so khớp chính xác; `null`, lookalike, khác scheme/cổng đều bị chặn). Request không có cả `Origin` lẫn `Sec-Fetch-Site` (CLI, curl) đi qua như trước.

### Khôi phục quản trị & mạng nội bộ (GAP-33)

Không còn câu lệnh nào bắt người dùng mở cửa sổ dòng lệnh (`serve user passwd`, `serve --lan` chỉ còn trong `HUONG_DAN_SU_DUNG.md`).

| Method & path | Thân | Trả về | Quyền & lỗi |
|---|---|---|---|
| `POST /api/me/recovery` | `{}` | `{code, createdAt}` — `code` dạng `XXXX-XXXX-XXXX-XXXX` (16 ký tự `[A-Z0-9]`), **chỉ trả lần này**, `Cache-Control: no-store` | admin (403 `Không có quyền`). Đã có mã **chưa dùng** → 409 `Đã có mã khôi phục chưa dùng — hãy tạo mã mới`: client gọi `rotate`. Chưa có mã hoặc mã cũ đã dùng thì tạo được. Gọi ở bước Hoàn tất. |
| `POST /api/me/recovery/rotate` | `{}` | như trên | admin. Luôn tạo mã mới, mã cũ (dùng rồi hay chưa) vô hiệu ngay. |
| `GET /api/me/recovery` | — | `{createdAt: string \| null, used: boolean}` — `createdAt: null` = chưa từng tạo; không bao giờ trả mã | admin |
| `POST /api/recover` | `{username, code, newPassword}` | như `POST /api/login`: `{user, …}` + cookie phiên mới | **Công khai** (không cần phiên). Cùng luật request với `/api/login`: `Content-Type: application/json` bắt buộc, cookie `HttpOnly; SameSite=Strict`. Mã nhập không phân biệt hoa/thường, bỏ qua `-` và khoảng trắng. 401 `Mã khôi phục không đúng hoặc đã dùng` cho mọi trường hợp sai: mã sai, mã đã dùng, tài khoản không phải admin, bị khoá hay không tồn tại (không lộ tài khoản nào có thật). 400 `Mật khẩu cần ít nhất 8 ký tự` (không tính vào giới hạn, không đốt mã). 429 + `Retry-After` khi quá **5 lần sai mỗi giờ mỗi IP** (lần thứ 6 bị chặn dù mã đúng; thành công xoá bộ đếm). |
| `PUT /api/system/network` | `{lan: boolean}` | `{lan, links: PairingLink[], code?, expiresAt?}` — khi `lan:true` `links` là các liên kết QR cùng dạng `POST /api/me/pairings` (cùng hàm dựng, tạo một mã ghép 5 phút cho admin đang gọi); `lan:false` → `links: []` | admin **và** loopback không qua proxy (như `canReveal`): khác → 403. Thiếu `lan` → 400. Máy chủ chạy `--addr` cố định → 409 `Máy chủ được khởi động với địa chỉ cố định…`. Không tìm thấy địa chỉ mạng nhà → 409 `Chưa tìm thấy mạng Wi-Fi nhà trên máy này`; không bind được cổng → 500. |

- **Mã khôi phục** lưu cùng cơ chế băm mật khẩu (argon2id) trong `web-accounts.json` (`users[].recovery = {codeHash, createdAt, usedAt?}`, thêm trường, không đổi định dạng cũ). Một mã một lần: `POST /api/recover` thành công ghi `usedAt`, đặt mật khẩu mới, xoá cờ `mustChangePassword`, **thu hồi mọi phiên và mã ghép QR đang chờ của tài khoản** (phát `session.revoked` cho từng phiên bị thu hồi), cấp phiên mới như `/api/login`. Hai request cùng một mã: đúng một request thành công.
- **Mạng nội bộ:** `serve` luôn giữ listener loopback (`127.0.0.1:8686`, hoặc `--addr`). `PUT /api/system/network` mở/đóng thêm một `http.Server` thứ hai, cùng handler, cùng cổng, bind từng IPv4 không phải loopback của máy. Đóng nó **không** chạm tới worker, phiên đăng nhập, hay listener loopback. Trạng thái ghi vào `~/.ainovel/serve.json` (thêm trường `lan: boolean`; `root` giữ nguyên) và được áp lại ở lần khởi động sau; cờ `--lan` vẫn bật LAN khi khởi động (không ghi tệp). `serve --addr <không phải loopback>` giữ hành vi cũ: không có Network, `lan` = có địa chỉ công khai.
- `GET /api/system` thêm `lan: boolean` (mọi vai trò đọc được) — người không phải admin dùng nó để hiện “Nhờ quản trị bật…”.

## 8. Frontend lưu cục bộ

| Khoá | Nơi | Nội dung |
|---|---|---|
| `ainovel.accounts` | localStorage | `[{username, name, role, lastLoginAt, device}]` (không mật khẩu/cookie) |
| `ainovel.next` | sessionStorage | route quay lại sau đăng nhập |
| `ainovel.panels` | localStorage | `{left: boolean, right: boolean}` Workspace |
| `ainovel.readerSize` | localStorage | 14–24 |
| `ainovel.theme`, `ainovel.readerFont` | localStorage | Cài đặt › Giao diện ứng dụng |
| `ainovel.lastRoute.<username>` | localStorage | route cuối |

## 9. Audio (GAP-17…20) — trỏ tới 10-audio.md §4

**Contract chính là [10-audio.md §4](10-audio.md#4-api-cần-thêm-gap--theo-quy-ước-11-api-contractmd):** `GET/POST/PUT S/audio/*`, `GET/PUT /api/projects/:p/{voices,pronunciations}`, SSE `GET S/audio/events`. Xuất hỗ trợ `m4b|mp3|wav`, nhúng `chapterMarks`, bật/tắt `.srt` bằng `subtitles`; `chapters?:number[]` chọn phần cần xuất (bỏ trùng, tăng dần; thiếu giọng/không thuộc tập → 409 nêu chương; `[]` → 409). `whenReady` chỉ đợi các chương được chọn; bỏ `chapters`/gửi `null` là cả tập. Tên phần có hậu tố `-ch1-9`/`-ch1-3_5`, không đè tệp cả tập; `onExists?:replace|keep|fail` dùng quy ước hậu tố thời gian của xuất bản (audio mặc định `replace`). Phản hồi có `durationSec`, yêu cầu chờ có `pending:true` (chi tiết và khởi động lại ở §4). Mục này chỉ ghi những hình dạng nằm ngoài spec chính:

| Endpoint | Ghi chú |
|---|---|
| `GET S/audio/ready` | `{ffmpeg:boolean, engine: {ready,engine,model,device,state,error,note?,rtf}, ready:boolean, message:string}` — thẻ "chưa có ffmpeg / bộ đọc" (15 §3). |
| `GET S/audio/volumes/:v/exports` | `{volume, dir, totalBytes, files:[ExportFile], chapterMarks:[{atSec,label}], exportedAt?, durationSec?, voices?, chapterCount?, format?, peaks?:[0..1 ≈2/s]}` — trạng thái "đã xuất"; `files[].path` tương đối thư mục xuất (tải qua `S/exports/file?path=`). Bản đọc/phân vai/cài đặt audio trả `version` (+ETag), từ điển chỉ ETag; PUT nhận `If-Match` → 409 `{error, conflict:true, version, current:<cùng hình dạng GET>}`. Ghi thành công và cập nhật nền phát `story.changed{parts:["audio"]}`. |
| `DELETE S/audio/flags/:id` | `{ok:true}` — xoá cờ. GET là `{flags:[Flag]}` (luôn mảng). |
| `POST S/audio/segments/:id/accept` + `/reject` | `{candidateId}` → `{ok:true}` — cặp của `rerender` (chọn bản mới/cũ). |
| `GET S/audio/segments/:id/audio` | WAV của đoạn (Range); đây là `url` mà `preview`/`script` trả về. |
| `GET S/audio/candidates/:id` | WAV của bản đọc lại (chưa chốt). |
| `GET S/audio/volumes/:v/chapters/:ch/peaks` | `{peaks:[0..1], durationSec}` (≈2 điểm/giây). |
| `GET S/audio/queue` | `{jobs:[Job]}` — Job gồm `jobId, chapter, volume, state, segmentsDone, segments, etaSec, speaker, currentText?, segmentId?, error?, errorAt?, percent, createdAt, updatedAt`. |
| `POST /api/jobs/:p/:s/audio/:v/export/:op` | `op=retry\|cancel`: thử lại xuất bị lỗi với lựa chọn đã lưu; huỷ cả yêu cầu đang chờ lẫn đang chạy (không huỷ tạo giọng). `audio.export` phát `state:"canceled"` và dòng xuất biến mất khỏi dock. |

**Segment id trong URL:** `:id` = `<chapter>-<segId>` (vd `3-s0007`), `?volume=` mặc định 1; trong script JSON `id` vẫn là `s0007`.

**Hình dạng JSON audio:** `GET S/audio/volumes/:v` chỉ trả `{chapters:[…]}` (không có `volume`/`title` ở cấp gốc). `PUT …/scripts/:ch` trả cùng tài nguyên bản đọc như GET, không phải `{ok:true}`. Trong đoạn, `source` có thể vắng; API luôn trả `normalized:[]` và `flags:[]` khi không có mục (không trả null), kể cả tệp lưu cũ bỏ các trường đó. Cờ nghe lại có `segmentId?`, `speaker?`, `durationSec?` và `createdAt`; không mặc định các trường tuỳ chọn luôn có. Audio chương chỉ có đường `.mp3`; WAV là đường đoạn/ứng viên. `GET /api/tts/voices` trả `{voices:[…]}` (`file?`, `durationSec?`), còn `GET /api/tts/engines` trả `{engines:[…],active}`; không phải mảng trần. `POST /api/tts/install` trả 202 `{job:"install",state:"running"}` khi bắt đầu, 200 `{job:"",state:"done",message}` khi đã cài và 409 khi đang cài.

**Sự kiện SSE** (`GET S/audio/events`, mỗi frame `event: <type>` + `data: {type, category:"AUDIO", data:{…}}`): `audio.render` `{jobId,chapter,segment,segments,segmentId,speaker,percent,etaSec,state}` (`state: queued|rendering|paused|error|done`), `audio.compose` `{chapter,state:drafting|done|error,segments,percent,error?}`, `audio.queue` `{jobs:[Job]}`, `audio.export` `{volume,percent,state,durationSec}`.

**Script GET trả thêm mỗi đoạn:** `changed:boolean` (chữ đổi sau khi tạo giọng) và `audio:{url,state:"none"|"ready"|"stale",durationSec}`; script root thêm `stale:boolean` (chapter nguồn đổi sau khi chốt). `PUT settings` chấp nhận thêm `normalize:{numbers,dates,money,symbols,abbreviations,chapterTitles,dropBrackets}` (mặc định mở hết). `PUT /api/projects/:p/pronunciations` chấp nhận cả `[{written,spoken}]` lẫn `{entries:[…]}`; GET luôn trả mảng trần.

**Quyền & lỗi:** ghi cần quyền viết story (voices/pronunciations ở cấp dự án cần quyền quản lý dự án); đọc (nghe/tải) chỉ cần quyền đọc. Lỗi `{"error":"<tiếng Việt>"}`: 404 chưa có bản đọc/đoạn; 409 chưa chốt, chương chưa nghiệm thu, chưa đủ giọng để nghe/ghép; 503 khi thiếu ffmpeg hoặc bộ đọc (soạn/duyệt vẫn dùng được). Cache đoạn = `sha256(text+voice+tone+speed+pitch+quality+engine+model)` — pitch là trường thêm cho "đổi giọng nhẹ khi là lời thoại".

## 10. Sự kiện toàn cục & phiên bản

- `GET /api/events?watch=p/s,p` là SSE duy nhất ở gateway. `hello {subscriber}` xác nhận đăng ký; `POST /api/events/watch {keys:string[], subscriber?:string}` thay toàn bộ phạm vi theo dõi của kết nối đó (bỏ `subscriber` để cập nhật mọi kết nối của tài khoản). Khoá dự án/truyện luôn được lọc theo quyền.
- Các sự kiện: `library.changed {}`, `project.changed {project}`, `story.changed {project,story,parts}` (`snapshot|chapters|outline|reviews|exports|meta|audio|rules`), `config.changed {scope,project?,story?}`, `account.changed {username}`, `session.revoked {username,session}` và `job.*`. `session` là ID phiên bị thu hồi, không phải yêu cầu đăng xuất các phiên khác. Sự kiện tài khoản chỉ tới chủ tài khoản và quản trị viên; truyện chỉ tới người được đọc. SSE có ID tăng dần, nối lại bằng `Last-Event-ID`, `reset` khi lịch sử đã hết, `ping` mỗi 20 giây.
- Gateway phát sự kiện ngay sau ghi thành công; các công cụ engine nộp chương/thẩm duyệt/đại cương và việc nhập/đồng bộ/xuất hoàn tất cũng phát `story.changed`. Các tệp thuộc khoá đang xem được quét mtime+kích thước 2 giây/lần, gộp thay đổi theo truyện trong mỗi lượt; gồm cấu hình và thư mục quy tắc toàn cục/dự án/truyện.
- GET/ghi cấu hình, tệp quy tắc, dự án, siêu dữ liệu truyện, thư viện, tài khoản trả `version:string` và `ETag`. `GET /api/projects/:p` còn đặt `project.version`; `/api/me` còn đặt `user.version`; `/api/users` còn đặt phiên bản riêng ở mỗi `users[]`. `config.version` trong cấu hình thư viện vẫn là số phiên bản định dạng `1`; PUT thư viện trả `{ok:true,config,version}` (đổi gốc trả thêm `root`).
- Ghi nhận `If-Match: <version>` (có thể có dấu ngoặc kép) hoặc trường JSON `version:string`; nếu lệch trả `409 {error:"Đã có thay đổi từ nơi khác",conflict:true,version,current}`. `current` là tài nguyên mới nhất theo quyền đọc, không chứa khoá API/mật khẩu. Không gửi điều kiện thì ghi đè theo lần ghi cuối. Hồ sơ và tài khoản nhận PUT bên cạnh POST hiện có; các thao tác đặt mật khẩu/thu hồi phiên vẫn là POST.


### Xoá truyện vĩnh viễn

`DELETE /api/projects/:p/stories/:s` nhận JSON `{confirm:"<s>"}`; xác nhận phải khớp chính xác tên thư mục truyện trong URL (ví dụ `01-tra-xanh`, không phải ID ghép `p/s`). Thành công trả `{ok:true}`.

- Quản trị viên hoặc người viết được giao truyện có quyền xoá; người chỉ đọc nhận 404 (ẩn). Truyện không tồn tại/không được xem cũng nhận 404; xác nhận sai nhận 400.
- Engine đang chạy, worker chưa xác định được là rảnh, hoặc tiến trình ngoài đang giữ khoá truyện: 409 `{"error":"Truyện đang chạy — dừng trước khi xoá"}`; không xoá dữ liệu và không dừng worker đang chạy.
- Worker rảnh được đóng trước; gateway giữ khoá truyện trong lúc xoá toàn bộ thư mục, gồm audio, góp ý và bản nháp đồng sáng tác thuộc truyện. Các bản nháp cấp dự án chưa gắn với truyện khác được giữ nguyên.
- Dọn khoá theo dõi, việc nền, bộ nhớ audio và quyền tài khoản trỏ chính xác đến truyện; giữ các quyền `*`/`p/*`. Phát `library.changed {}` và `project.changed {project:p}` ngay sau thành công.


### Thông báo tiến độ đăng nhập OAuth

Mỗi luồng phát `config.changed {scope:"oauth",flow:"<id>",status:"pending"|"done"|"failed"}` **chỉ cho tài khoản đã bắt đầu luồng**, kể cả khi có quản trị viên khác đang trực tuyến. Sự kiện xuất hiện khi tạo luồng, khi URL trình duyệt/mã thiết bị sẵn sàng (vẫn `pending`), khi callback thành công, lỗi nhà cung cấp, huỷ hoặc hết hạn (đều `failed`). Không đưa URL đăng nhập, mã thiết bị hoặc token vào SSE; nhận sự kiện thì đọc lại `GET /api/config/oauth/status?id=<flow>`. `state` của phản hồi này vẫn dùng ba trạng thái trên; hết hạn có lỗi `Đã hết thời gian đăng nhập`.


## Thông báo (GAP-30)

Tất cả đường dẫn cần cookie phiên đăng nhập. Dữ liệu và đăng ký thiết bị thuộc tài khoản; người nhận chỉ nhận truyện được phép đọc. `notify.changed {unread}` chỉ phát cho chủ hộp thư sau thay đổi, để tải lại hộp thư/cài đặt.

| Đường dẫn | Nhận | Trả |
|---|---|---|
| `GET /api/push/vapid` | — | `{publicKey}` |
| `GET /api/push/subscriptions` | — | `{subscriptions:[{endpoint,keys:{p256dh,auth},device,session,createdAt}]}` |
| `POST /api/push/subscriptions` | `{endpoint,keys:{p256dh,auth},device}` | `{ok:true}`; gắn với phiên hiện tại, không nhận phiên từ client |
| `DELETE /api/push/subscriptions` | `{endpoint}` | `{ok:true}`; chỉ gỡ đăng ký của tài khoản hiện tại |
| `GET /api/notify/prefs` | — | `NotifyPrefs` bên dưới, kèm `ETag` |
| `PUT /api/notify/prefs` | `NotifyPrefs`, `If-Match` | `NotifyPrefs`; phiên bản lệch: `409 {error,conflict:true,version,current}` |
| `GET /api/notifications?before=<id>` | — | `{notifications:InboxNotification[],unread,nextBefore}`; tối đa 50/lượt, `nextBefore` rỗng khi hết |
| `POST /api/notifications/read` | `{ids:string[]}` hoặc `{all:true}` | `{ok:true,unread}` |
| `POST /api/notifications/:id/action` | `{action:"approve"|"rewrite"|"mute"|"revoke"|"share"}` | `{ok:true,url?}`; chỉ nhận thao tác được cung cấp trong mục hộp thư; kiểm tra quyền/cookie, không nhận session đích từ client |
| `POST /api/notify/test` | `{}` | `{ok:true}`; thêm hộp thư và xếp hàng gửi thử |

`NotifyPrefs = {enabled,types:{gate,error,job,done,security},digest:"each"|"five"|"day",quiet:{from,to},version:string}`. Giờ dạng `HH:mm`, mặc định `22:00–07:00`; `{from:"",to:""}` tắt giờ yên lặng. `security` và `error` luôn bật và bỏ qua giờ yên lặng/công tắc tổng. Các thiết lập còn lại chỉ lọc Web Push; hộp thư vẫn lưu đủ. Loại `import/sync/cocreate` dùng công tắc `job`.

`InboxNotification = {id,type,story,chapter?,job?,title,body,url,actions:[{action,title,url?}],createdAt,read,score?}`. `score` là điểm trung bình các chiều thẩm định (0–100) khi đã có dữ liệu; nhóm chương tính trung bình các chương có điểm. `story` là `p/s`, rỗng cho an toàn tài khoản. Hộp thư bền vững giữ tối đa 500 mục/tài khoản, mới nhất trước; `unread` đếm toàn bộ mục còn được phép đọc, không chỉ trang hiện tại.

Nguồn Hub: `gate.waiting`, `engine.stopped.error`, `job.done`, `chapter.committed`, `security.login`. Nội dung tiếng Việt và liên kết sâu theo `18-mobile.md` §M7. Chương hoàn thành gộp theo từng truyện: mỗi chương, đủ 5 chương, hoặc 21:00 giờ máy chủ (nhóm cuối ngày còn tồn tại sau khởi động lại). Duyệt nhanh gọi cùng `/next {applyComments:true,chapter}`; nhờ viết lại mở URL; tắt loại cập nhật prefs; “Không phải tôi” chỉ thu hồi phiên được lưu trong thông báo an toàn.

Khoá VAPID tạo một lần tại `~/.ainovel/push/vapid.json`, quyền `0600`; hộp thư, nhóm gộp, prefs và đăng ký ở `state.json` cùng thư mục. Gửi mã hoá bất đồng bộ qua hàng đợi giới hạn; `404/410` gỡ endpoint. Đăng xuất/thu hồi phiên gỡ các đăng ký của phiên; kiểm tra phiên và quyền lần nữa trước khi gửi. Hàng đợi đầy không làm mất mục hộp thư.

**Liên kết sâu & nội dung (đối chiếu `18-mobile.md` §M7).** `url` luôn là đường dẫn cùng nguồn; mã truyện `p/s` được mã hoá thành một đoạn (`%2F`) vì router tách theo “/” rồi mới giải mã (`web/src/mobile/deepLinks.tsx` tra truyện từ thư viện):

| `type` | `url` | `actions[].action` |
|---|---|---|
| `gate` | `/read/p%2Fs/25?gate=1` | `approve` · `rewrite` (`url` = `/read/p%2Fs/25?rewrite=1`) · `mute`; người chỉ đọc truyện không nhận `approve`/`rewrite` |
| `error` | `/write/p%2Fs` | — |
| `job` (sách nói) | `/listen/p%2Fs` | `share` |
| `job` (xuất truyện) | `/api/projects/:p/stories/:s/exports/file?path=…` (đường dẫn tải, không phải route SPA; service worker mở thẳng) | `share` |
| `done` | `/read/p%2Fs/20` (nhóm gộp trỏ chương đầu) | — |
| `security` | `/more/devices` | `revoke` |
| `import` · `sync` · `cocreate` | `/cmd/<job mã hoá>` (vd `/cmd/p%2Fs%2Fimport%2F1`) | — |

Nội dung: cổng duyệt và chương xong ghi `<tên chương> · Biên tập viên chấm 8,4`; nhóm gộp ghi `Điểm trung bình 8,2` (điểm 0–100 của `score` hiển thị theo thang 10, dấu phẩy, như màn Đọc). Nhóm chỉ có một chương giữ nguyên tiêu đề “Xong chương N”. Thông báo lỗi chỉ ghi nguyên nhân dễ hiểu (hết hạn mức · khoá API · quá chậm · quá tải · mất kết nối · chung), không lộ mã trạng thái hay tên công cụ.

Chống trùng (phát lại sự kiện khi worker khởi động lại): cùng `gate`/`done` + truyện + chương, hoặc cùng mã việc nền, hoặc cùng phiên đăng nhập bị chặn trong 10 phút; cổng duyệt còn chưa đọc chặn trùng bất kể thời gian. Đổi `digest` sang `each` trả ngay các chương đang chờ gộp thành một mục hộp thư. `POST /api/push/subscriptions` không gửi `device` thì dùng nhãn suy ra từ User-Agent.


## Nhật ký (GAP-31)

`GET /api/projects/:p/stories/:s/events?chapter=&agent=&level=error&mine=1&q=&before=&limit=` trả JSON `{events,nextCursor,chapters,counts}`. Mọi tài khoản được đọc truyện đều đọc được nhật ký; không được xem hoặc không tồn tại trả 404. Đọc trực tiếp `output/novel/logs/tui.log`, không khởi động worker hay lấy khoá truyện; hoạt động cả khi TUI giữ khoá.

- `events`: mới nhất trước, mỗi lời gọi chỉ một dòng (hợp nhất bắt đầu/kết thúc theo ID). Giữ các trường `HostEvent` (`id,time,finishedAt,failed,category,agent,summary,kind,level,depth,durationNs,retryAt,sessionId,canRetryNow`), thêm `chapter` (0 = hệ thống), `username` chỉ cho nhập/lệnh của người dùng.
- Công cụ thêm `{tool,args,result,durationMs,status:"running"|"ok"|"error",retries:[{atSec,error,waitSec,ok}]}`. `args` giới hạn 400 ký tự Unicode, `result` 800, thêm dấu `…` khi cắt. `atSec` tính từ lúc bắt đầu gọi; `waitSec` là thời gian chờ thực tế do tầng retry báo; `ok` cho biết lần gọi đã hồi phục hay chưa. Không giả tạo tham số/kết quả khi đọc log cũ.
- `chapters:[{chapter,title,score?,eventCount}]` mô tả các nhóm trong toàn bộ kết quả lọc (không riêng trang). `score` là trung bình các điểm chiều đánh giá chương nếu có, thang 0–100 (giao diện chia 10 để hiện 8,1).
- `counts:{total,byAgent:{[agent]:number},errors,mine}` tính sau lọc chương/từ khoá, trước các chip agent/lỗi/của bạn. `agent=architect` và `byAgent.architect` gộp `architect_short/architect_long`. Lọc lỗi và số lỗi gồm cả lỗi đã tự hồi phục có `retries[]`. `mine=1` chỉ chọn sự kiện đầu vào/lệnh được gắn đúng tên đăng nhập hiện tại, không chọn phản hồi của engine; TUI gắn tên người dùng hệ điều hành. Chữ tìm kiếm khớp cả tham số/kết quả/tên người dùng, không phân biệt hoa/thường hoặc dấu tiếng Việt (kể cả đ/d).
- `limit` mặc định 100, hợp lệ 1–500. `nextCursor` là chuỗi mờ hoặc `null`; gửi nguyên chuỗi vào `before` với cùng bộ lọc. Cursor đóng băng chiều dài log ở trang đầu nên sự kiện append sau đó không làm lặp/mất trang cũ. Cursor/bộ lọc sai trả 400 với lời nhắc tải lại; phản hồi `Cache-Control: no-store`.
- Luồng trực tiếp ở `GET /api/projects/:p/stories/:s/events/live` (thay đường `/events` dành cho JSON). Sự kiện SSE `event` dùng cùng trường bổ sung và ID để thay thế lời gọi đang chạy. Envelope JSON `event_data` được thêm vào từng dòng slog hiện hữu; reader vẫn hiểu log text cũ.
- Khi log được ghi thêm, watcher phát `story.changed {project,story,parts:["activity","snapshot"]}` cho người đang theo dõi truyện để tải lại lịch sử/counts; hoạt động với cả writer TUI bên ngoài gateway.

## Tải bản xuất với tên thân thiện

`GET /api/projects/:p/stories/:s/exports/download?file=<đường dẫn>` tải tệp trong thư mục xuất bản của truyện về trình duyệt. Dùng cho nút **Tải xuống** của Trình đọc và thẻ kết quả `/export`; `exports/file?path=` (tên đĩa) vẫn dành cho tải/nghe tệp audio.

- Quyền: đọc truyện (`CanRead`); tài khoản không có quyền hoặc đã khoá → `404` (ẩn tồn tại truyện).
- `file` là đường dẫn tương đối thư mục xuất bản (đúng `ExportFile.path`) hoặc tương đối truyện (`<dirs.exports>/…`, đúng `ExportResult.path`). Đường dẫn rỗng, tuyệt đối, chứa `..` hoặc thoát khỏi thư mục xuất bản → `400 {error:"đường dẫn không hợp lệ"}`; tệp không tồn tại hoặc là thư mục → `404`.
- Với `.txt`/`.epub`, `Content-Disposition: attachment; filename*=UTF-8''<tên thân thiện>`, tên thân thiện = `<tên truyện> · Tập N.<đuôi>`. Tên truyện bỏ `/\:*?"<>|` và ký tự điều khiển, gộp khoảng trắng; lấy từ `story.json.title`, rồi tên sách của engine, rồi `Truyện`. Không suy được tập: truyện một tập → `Tập 1`, nhiều tập → bỏ ` · Tập N`. Slug trên đĩa không bao giờ xuất hiện ở tên tải về; định dạng khác giữ tên tệp gốc.
- `Content-Type` theo đuôi tệp (`application/epub+zip`, `text/plain`, …); hỗ trợ `Range` (`http.ServeContent`).
- Web dựng cùng tên bằng `friendlyExportName` (`web/src/lib/exportName.ts`) để hiện ở “Tên tệp · lưu trong mục Bản xuất” và trên thẻ kết quả `/export`; giao diện không hiện đường dẫn tệp xuất (chỉ quản trị có nút “Chép đường dẫn”).

## Chọn nơi lưu truyện (GAP-34 · 09 §10)

Bộ duyệt thư mục trong app, chỉ quản trị (403 `Không có quyền`). Mọi đường dẫn bị **giới hạn trong các Nơi** (`library.DiscoverPlaces`): `filepath.EvalSymlinks` rồi so khớp; `..`, symlink thoát Nơi, đường dẫn ngoài Nơi → `403 {error:"Không duyệt được thư mục này"}`; không tồn tại → 404.

| Method & path | Thân | Trả về | Ghi chú |
|---|---|---|---|
| `GET /api/fs/places` | — | `[{id, label, path, icon}]` | Chỉ Nơi đang tồn tại, theo thứ tự: Thư mục nhà, Tài liệu, Màn hình, iCloud Drive, mỗi `~/Library/CloudStorage/*` (Google Drive, OneDrive… — nhãn không chứa tài khoản), mỗi `/Volumes/*` (ổ ngoài; bỏ ổ hệ thống trỏ về `/`). Linux: `XDG_DOCUMENTS_DIR`/`XDG_DESKTOP_DIR` + `/media/$USER/*`, `/run/media/$USER/*`, `/mnt/*`; Windows: Documents, Desktop, ổ `C:\…Z:\`. `icon` = tên icon của kit (`user`, `document-text`, `monitor`, `cloud-connection`, `box`). `path` chỉ để duyệt/tooltip. |
| `GET /api/fs/list?path=` | — | `[{name, path, kind: 'dir'\|'library'\|'readonly', items, writable, projects?}]` | Chỉ thư mục, sắp theo tên; ẩn tên bắt đầu bằng `.`, thư mục hệ thống (`lost+found`, `$RECYCLE.BIN`, `Windows`…, và `Library`/`Applications` trong Thư mục nhà trên macOS) và symlink trỏ ra ngoài Nơi. `library` = có ≥ 1 dự án Noverse (cùng phép dò với `GET /api/library`, kèm `projects`); `readonly` = hệ điều hành báo không ghi được (`access(W_OK)`, không tạo gì trên đĩa). `items` không đếm tệp chấm, chặn ở 999. |
| `POST /api/fs/mkdir` | `{parent, name}` | 201 `{name, path, display}` | `parent` bị giới hạn như trên. `name` 1–100 ký tự, không bắt đầu/kết thúc bằng `.`, không chứa `/ \ : * ? " < > \|` hay ký tự điều khiển → 400 `Tên thư mục không hợp lệ`. Đã tồn tại → 409 `Đã có thư mục tên này` (dùng `os.Mkdir`, không bao giờ dùng lại thư mục cũ). |
| `GET /api/library/inspect?path=` | — | `{kind: 'found'\|'empty'\|'other'\|'readonly'\|'missing', projects, stories, items, place, display, name, projectName?}` | Không giới hạn trong Nơi (hộp chọn của máy có thể trả thư mục bất kỳ) nhưng chỉ trả số đếm + chuỗi hiển thị. `readonly` dùng đúng phép thử của `library.CheckRoot` (tạo rồi xoá thư mục tạm). `found` = đã là thư viện Noverse (kèm `projects`, `stories`, `projectName` của dự án đầu tiên); `empty` = không có mục nào ngoài tệp chấm; `other` = có tệp khác (vẫn dùng được); `missing` = chưa tồn tại hoặc không phải thư mục. `display` = `Nơi › … › Thư mục` (tối đa 3 đoạn: đoạn đầu › … › cuối); thư mục ngoài mọi Nơi hiện `Máy chủ › …`; không bao giờ lộ đường dẫn tuyệt đối khi thuộc một Nơi. |

### Khởi động không có thư viện

`serve` chọn gốc theo thứ tự: `--root` → `~/.ainovel/serve.json` → `~/Truyen` **nếu đã tồn tại** → (không có) **chưa chọn nơi lưu truyện**. Không còn mặc định ngầm vào `~/Truyen` hay thư mục hiện tại; `serve.json` chỉ được ghi khi `PUT /api/library {root}` thành công.

Khi chưa chọn: `GET /api/library` → 200 `{configured: false, rootDisplay: "", rootPath: "", config: <mặc định>, projects: [], candidates: []}`; `/api/projects…`, `/api/reveal` (trừ `target=rules&scope=global` và `target=config&scope=global`) và các đường cấu hình/quy tắc theo dự án → **409 `Chưa chọn nơi lưu truyện`** (không panic, không đọc thư mục hiện tại). `PUT /api/library {root}` vẫn chạy để chọn lần đầu; `PUT` cấu hình khi chưa chọn → 409.

Luồng bước 5 (Onboard): gợi ý “Tài liệu › Truyện Noverse” → bấm Tiếp tục: `POST /api/fs/mkdir {parent: <Tài liệu>, name: "Truyện Noverse"}` (409 coi như đã có) rồi `PUT /api/library {root}`. Đang có truyện chạy → 409 `Đang có truyện chạy — hãy đóng tất cả trước khi đổi thư viện`.
