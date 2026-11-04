# Viderse — Hợp đồng dữ liệu

Dữ liệu UI cần, theo từng mục. Mỗi mục ghi **có sẵn** (dùng service/API hiện tại) hay **mới**. Mỗi đợt thiết kế thêm một mục “Bổ sung §X”, không sửa mục cũ; mục bị thay thì ghi “Thay bằng §Y”.

Mọi lỗi trả về giữ dạng hiện tại `{ error, code? }`. UI dịch `code` sang câu tiếng Việt theo [06-copy.md](06-copy.md).

---

## §1 · Trợ lý của một video (mới, chỉ là view-model) <a id="agent-view"></a>

Hàm thuần `agentView(job, steps)` trong `src/lib/` (dùng chung cho client và CLI). Không đổi DB.

```ts
type AgentId = "scout" | "director" | "artist" | "qa";
const AGENT_STEPS: Record<AgentId, CloneStepName[]> = {
  scout: ["download", "backdrop", "transcript"],
  director: ["metadata", "scenes"],
  artist: ["images"],
  qa: ["build", "score", "export"],
};
type AgentState = "waiting" | "running" | "done" | "stopped" | "needs_you";
interface AgentView {
  id: AgentId;
  state: AgentState;
  /** Câu đang làm, lấy từ cùng nguồn với cloneTerminal: “Đang vẽ cảnh 5 — đèn pin soi vào phòng trống” */
  now: string | null;
  /** Một dòng số liệu: “Ảnh đã vẽ 5 / 7 · ChatGPT”, “6 cảnh · 14 tag”, “Độ hoàn thiện 96 / 100” */
  metric: { label: string; value: string } | null;
}
```

Quy tắc trạng thái:
- `running` khi bất kỳ step nào của trợ lý đang chạy.
- `done` khi mọi step đều xong.
- `stopped` khi có step lỗi hoặc đang chờ thử lại.
- `needs_you` chỉ dành cho `qa` khi video ở Chờ duyệt.
- Còn lại là `waiting`.

## §2 · Trạng thái video (đổi ánh xạ, thêm Tạm dừng)

`CloneStageGroup` hiện có → nhóm UI:

| Nhóm UI | Lấy từ |
|---|---|
| `queued` | Mục trong hàng chờ, chưa có job |
| `working` | `preparing`, `images`, `rendering` |
| `review` | `review` |
| `attention` | `attention` |
| `paused` | job có `pausedAt` |
| `ready` | `done` |
| `posted` | `posted` |
| `legacy` | `legacy` |

**Mới — Tạm dừng một video:**
- **DB:** cột `clone_jobs.paused_at` (nullable).
- **API:**
  - `POST /api/clone/<jobId>/pause` — step đang chạy làm xong rồi không chạy step kế; scheduler bỏ qua job; job không giữ luồng.
  - `POST /api/clone/<jobId>/resume`.
- **CLI:** `ytf clone pause|resume <jobId>`.
- Khác `cancel`: `cancel` giữ nguyên nghĩa “dừng hẳn”.

**Mới — dòng phụ cho mọi trạng thái:** `job.subline` (string), tính ở view-model, ví dụ:
- “Hoạ sĩ đang vẽ cảnh 5/6”
- “Chờ hạn mức ảnh · 07:00”
- “7 ảnh · 42 phút trước”
- “Chờ bạn duyệt ảnh” (bước images đỗ `awaiting_review`)
- “Bản dựng đang chờ duyệt” (bước build đỗ `awaiting_review`; ảnh đã duyệt xong). Hai bước kỹ thuật cùng nhóm `review` phải khác dòng phụ; nếu cả hai cùng đỗ thì ưu tiên duyệt ảnh.

## §3 · Hàng chờ (mới: ưu tiên, nhịp, giải thích)

**DB:**
- `clone_queue.priority`: `"pick" | "batch"`, mặc định `"batch"` cho dữ liệu cũ.
- `clone_queue.position`: dùng sẵn có.

**Luật** (xem [ia-flows §5](01-ia-flows.md#5-luật-chạy)):
- Enqueue `pick` → chèn sau mục `pick` cuối cùng.
- Enqueue `batch` → cuối hàng.
- Scheduler:
  1. Lấy mục `pick` đầu tiên trước.
  2. Hết mục `pick` thì luân phiên kênh trong các mục `batch`.
  3. Bỏ qua kênh đã đủ nhịp, hoặc kênh không còn template dùng được.
- **Nhịp** chỉ đếm job bắt đầu từ mục `batch`, kể cả mục do Tự lấy video tạo. Thay cho “giới hạn mỗi ngày” hiện tại, vốn đếm cả video chọn tay đi qua hàng chờ.
- **Thay đổi hành vi:** video chọn tay không còn “bắt đầu thẳng không qua hàng chờ”. Nó vào hàng `pick`, và còn luồng thì scheduler bắt đầu ngay (kích hoạt scheduler ngay khi enqueue, không đợi chu kỳ 1 phút).

**`GET /api/queue`** trả:

```ts
{
  paused: boolean;
  pausedAt: string | null;                             // r31-v27: giờ bấm Tạm dừng → “Đã tạm dừng lúc 00:04”
  workers: { used: number; total: number };
  imageWorkers: number;
  rescanHours: number;
  running: Array<{ jobId; channelId; title; agent: AgentId; now: string; stepIndex: number /* 0–8 */; etaMinutes: number | null }>;
  rotation: {
    order: string[];                                    // channelId theo vòng
    next: string | null;
    skipped: Array<{ channelId; reason: "pace_full" | "no_template" | "paused_channel"; until?: string }>;
  };
  items: Array<{ id; channelId; title; source; sourceUrl: string | null; thumbnailUrl: string | null; templateName; durationSec; priority: "pick" | "batch"; position: number; etaStart: string | null;
    waitReason?: "workers_busy" | "rotation" | "pace_full" | "no_template" | "queue_paused" | "paused_channel" }>;   // r31-v27 (#52); câu ở 06-copy › Hàng chờ
  pace: Array<{ channelId; used: number; limit: number }>;
  imageQuota: Array<{ provider: "chatgpt" | "grok"; used: number; limit: number | null; period: "day" | "week"; resetsAt: string | null; role: "main" | "fallback" }>;
  doneToday: Array<{ jobId; title; at: string; state: "ready" | "review" }>;
}
```

`limit`/`resetsAt` là `null` khi nhà cung cấp không cho biết. Khi đó UI ghi “43 ảnh hôm nay”, không bịa “/50”.

**Hành động:**
- Có sẵn: `pause`, `resume`, `move` (lên đầu, đổi vị trí), `remove`.
- Mới: `PATCH settings { workers, imageWorkers, rescanHours }`. Các khoá này đang nằm ở Cài đặt → Clone video, chỉ đổi nơi gọi.
- Theo dev (#52, #64, đã duyệt): hợp đồng v2 `PATCH /api/queue` nhận `{ paused }` · `{ workers (1–6), imageWorkers (1–8), rescanHours (1–24) }` · `{ id, position }` · `{ id, action: "front" | "remove" | "restore" }`. `id` là UUID. `front` chuyển mục thành `pick`, đặt trước mọi `pick`. `paused_channel` = kênh tắt Tự lấy, chỉ khoá mục tự lấy của kênh đó. SSE `queue.updated`. <!-- r31-v27 -->
- `PATCH { id, action: "remove" }` trả `200` với toàn bộ `CloneQueueView` như GET, thêm `removed: { id: string, position: number, priority: "pick" | "batch" }`. Ghi `clone_queue.removed_at`, không xoá vật lý hoặc tạo job; mục biến mất ngay khỏi `items`, ETA, lượt luân phiên, bộ đếm và lựa chọn scheduler.
- `PATCH { id, action: "restore" }` trả `200` với toàn bộ `CloneQueueView` (không có `removed`), xoá `removed_at` trên **chính dòng cũ**, giữ nguyên `id`, `position`, `priority`, nguồn, tuỳ chọn ảnh và `templateId`/recipe hiện hữu. Không gọi enqueue hoặc chọn lại template. Gọi lại restore trên mục vẫn chờ là an toàn.
- Restore trả `409 { error: string, code: "conflict", recovery: null, retryAt: null }` nếu mục đã bị dọn, đã chạy, hoặc cùng nguồn đã được xếp/chạy thành một mục/job khác. UUID sai/thao tác sai → `400` (`invalid_request`); bỏ/di chuyển mục không còn tồn tại → `404` (`not_found`).
- Mỗi lần remove/restore thành công phát `queue.updated`. GET không có trường `removed`; đó chỉ là biên nhận của remove để client giữ danh tính cho Hoàn tác. Mục đã bỏ không chặn khử trùng hoặc Tự lấy; nếu nguồn được xếp lại thì restore cũ xung đột, không tạo bản trùng.
- Sweep drain định kỳ hiện có dọn vật lý mục có `removed_at` **cũ hơn 24 giờ**, kể cả lúc tạm dừng hoặc hết luồng. Tính tuổi từ lúc bỏ, không từ lúc tạo; mục chưa được sweep vẫn khôi phục được. Lịch sử mục đã chạy/bỏ qua không có `removed_at` tiếp tục giữ 30 ngày. Không thêm timer/goroutine.
- Bớt `workers` không dừng job đang chạy; luồng thừa nghỉ khi job đó xong.
- `GET /api/status` lỗi 2 lần liền hoặc SSE đứt quá 10 giây → viên việc nền “Mất kết nối”, giữ số cũ.

## §4 · Kế hoạch trước khi làm video (mới)

`POST /api/clone/plan` với `{ channelId, urls: string[], priority?: "pick" | "batch", templateId?: string, noImages?: boolean }`. Mặc định `priority` là `pick` khi có 1 link, `batch` khi nhiều hơn. Công tắc “Ưu tiên cả N” gửi `priority: "pick"`.

<!-- r31-v27 --> `templateId` khi chọn khác vòng luân phiên ở chip Template (#53). `noImages` **không** gửi từ Video mới (L14, #58): giữ cho CLI; ngoại lệ ảnh của một video đặt ở Bàn dựng (§14). Trả thêm (#52): `accepted[].imageCalls`, `accepted[].priceUSD` (`null` khi chưa có bảng giá → UI chỉ ghi số ảnh), `viewCount`, `publishedAt`, `rivalLinked`; `duplicates[]` có `title`, `thumbnailUrl`, `durationSec`, `state` (kể cả `"queued"`, `jobId: null`).

Trả:

```ts
{
  accepted: Array<{ url; videoId; title; durationSec; outlier: number | null; hasCaptions: boolean }>;
  duplicates: Array<{ url; jobId; state }>;   // đã làm cho kênh này → UI dẫn tới video đó
  invalid: Array<{ url; reason: "unreadable" | "private" | "not_youtube" }>;
  position: "front" | "back";
  etaStart: string | null;                    // giờ bắt đầu dự kiến của video đầu
  paceLeftToday: number;                      // chỉ có nghĩa với batch
  overflowDays: number;                       // số ngày rải ra theo nhịp
  template: { id; name; fromRotation: boolean };
}
```

Sau đó `POST /api/clone/enqueue` với cùng body (không còn endpoint “start now” riêng cho UI; CLI `clone start` giữ nguyên và tương đương `priority: "pick"`).

## §5 · Đầu trang (mới)

`GET /api/status` (lần tải đầu; sau đó nhận sự kiện đẩy `GET /api/events`; mất kết nối sự kiện thì poll 3 giây — #8), trả: <!-- r32 -->

```ts
{
  running: number;
  queued: number;
  paused: boolean;
  reviewCount: number;
  attention: Array<{ kind: "tool_update" | "login_expired" | "quota_low" | "tool_failed"; text: string; href: string }>;
}
```

Chấm trên bánh răng hiện khi `attention.length > 0`.

## §6 · Bàn dựng

- Danh sách trái: `GET /api/clone?channel=<id>`. Đã có, chỉ cần thêm nhóm `paused` và `subline`. <!-- r5-scope -->
- **Mọi kênh (mới):** `GET /api/clone?channel=all` trả video của mọi kênh, mỗi video có `channelId`. Có thể giới hạn mỗi nhóm: `queued` trả 3 video đầu theo thứ tự Hàng chờ kèm `queuedTotal`, `posted` không trả ở Mọi kênh.
- **Dòng kênh (mới):** `GET /api/channels/summary` trả một dòng cho mỗi kênh:

```ts
Array<{
  channelId: string; name: string; initials: string; color: string;
  today: { done: number; cap: number };      // nhịp kênh hôm nay, chỉ đếm batch/auto (L4)
  waitingYou: number;                        // Chờ duyệt + Cần xử lý + Tạm dừng
  ready: number;                             // Sẵn sàng đăng
  postedThisWeek: number;                    // chỉ dùng trong tooltip
  autoFetch: boolean;
  templateIssue?: { templateId: string; text: string }; // ví dụ “Template Cận cảnh thiếu clip nền”
}>
```

- Bộ đổi kênh dùng chung endpoint này (nhịp hôm nay, chấm Chờ bạn). Dòng “Mọi kênh” cộng tổng `today` và `waitingYou`.
- Video mặc định: hàm thuần `pickDefaultJob(jobs)` theo [ia-flows W5](01-ia-flows.md#w5--theo-dõi-video-đang-làm-bàn-dựng).
- Bảng phải có đúng **Nội dung đăng** và **Thông tin** (mặc định). Thông tin đọc bốn trợ lý, nguồn, template và nền; Nội dung đăng dùng metadata và lịch sử thật (§16).
- Cột trái dùng job thật; chế độ Chọn chỉ đưa video `ready` vào gói ZIP `GET /api/clone/download?ids=...`. Chọn thêm video chưa xong không làm mất lựa chọn; thanh dưới nêu rõ số đã chọn/số đủ điều kiện.
- Ô ảnh **bị chặn**: dùng trạng thái `skipped` của image item có sẵn.
- **Sẵn sàng:** cần `exportBytes` của MP4 để gợi ý xoá sau khi đăng. Có trong `delete-export` (freedBytes); cần đọc trước khi xoá.
- Bản xuất Editor đã công bố có thể nằm trong `steps[export].output.data` (`projectId`, `exportId`, `durationMs`, `size`) mà không có phần tử trong `output.artifacts`. Trình xem thành phẩm đọc bước xuất `succeeded`, không có `removedAt`, và dùng thời lượng của bản xuất thay vì thời lượng nguồn; danh sách artefact rỗng không có nghĩa tệp đã bị xoá. Trường `accepted` chỉ ghi lựa chọn **Vẫn xuất** của chủ, không phải điều kiện tồn tại của tệp; bản tự đạt cổng xuất vẫn phát khi `accepted: false`.
- Lỗi tải/giải mã MP4 không được thông báo thành “tệp đã xoá” nếu đầu ra vẫn còn; hai trạng thái này tách biệt ở cả máy tính lẫn điện thoại.

## §7 · Chọn nhiều trong Kênh

Dùng `POST /api/clone/bulk` có sẵn. UI cần đếm trước số job hợp lệ cho từng thao tác: hàm thuần `bulkEligibility(jobs)` → `{ retry, approve, bundle, markPosted, cancel, delete }`, mỗi mục là danh sách `jobId`.

“Mọi video của 3 kênh” = `GET /api/clone` không có `channel`.

## §8 · Tạo kênh

- `POST /api/owner-channels` đã có. **Đổi:** UI bắt buộc ít nhất 1 đối thủ; CLI giữ tuỳ chọn.
- Ô “Đối thủ mới” trên màn Tạo kênh dùng API theo dõi đối thủ đã có (như Khám phá), rồi đưa id vào danh sách chọn; lỗi handle hiện ngay dưới ô. <!-- r13-create -->
- Ngay sau khi tạo: chạy tạo bộ nhận diện (logic có sẵn khi liên kết đối thủ đầu tiên). Trả `identityJobId` để màn **Xong** theo dõi. <!-- r11-audit -->
- Cùng request đặt luôn “Cách kênh chạy” (template rotation = Mẫu chuẩn, nhịp, `autonomy` — data-contract §20) bằng các API đã có, và `language` = ngôn ngữ đoán từ đối thủ tham chiếu.
- Nhận diện tham chiếu **đối thủ liên kết đầu tiên** (thứ tự `owner_channel_sources.linked_at`). Chi tiết §16.

## §9 · Các API trợ lý đã ngừng (Gói V37)

Không còn ô Nhắn trợ lý, thẻ Kế hoạch hoặc bộ phân loại câu thành intent. Nhóm `/api/assistant/*`, `/api/recap*`, `/api/explain/*`, `/api/learned-rules*` và `/api/undo` không đăng ký route, không có alias hoặc fallback.

Người dùng thao tác trực tiếp trên video, template và ba loại đề xuất ở §18. Ghi chú người dùng trong hồ sơ kênh không phải kế hoạch trợ lý; dữ liệu lịch sử cũ không được tự biến thành tác vụ mới.

## §10 · Truy cập từ điện thoại (đã chốt D1: cách a) <!-- r32 -->

Đã chốt (29/10/2026, #11): **(a)** nghe trên mạng LAN, ghép thiết bị một lần bằng mã QR, sau đó dùng cookie phiên. Hai cách đã cân nhắc:
- **(a)** nghe trên mạng LAN, kèm mã ghép thiết bị một lần và cookie phiên;
- **(b)** chỉ qua Tailscale/VPN của người dùng.

Cả hai đều phải thêm xác thực trước khi mở ra ngoài localhost (theo DESIGN.md › Discovery policy). Chưa chốt thì Gói V10 chỉ làm giao diện responsive và chạy được trên cùng máy.

## §11 · Việc nền & thông báo (mới, Gói V14) <a id="activity"></a>

```ts
interface BackgroundTask {
  id: string;
  kind: "video" | "identity" | "channel_scan" | "backdrop_scan" | "thumbnail_preview" | "template_preview" | "editor_export" | "setup" | "cleanup" | "logs";
  agent?: AgentId;              // orb khi có
  title: string;                // “Tạo bộ nhận diện”
  objectRef: { type: string; id: string; label: string };
  phase: string;                // câu tiếng Việt
  progress: { done: number; total: number } | null;   // null = không xác định
  etaSec: number | null;
  state: "running" | "paused" | "waiting" | "failed";
  action: { label: string; href?: string; op?: "cancel" | "resume" | "retry" };
}
interface Notification {
  id: string;
  kind: "review_ready" | "needs_attention" | "ready_to_post" | "task_done" | "action_result";
  count: number;                // > 1 khi đã gộp
  text: string; href: string; actionLabel: string;
  createdAt: string; readAt: string | null;
}
GET /api/activity → { running: BackgroundTask[]; videoJobs: number; queued: number; unread: Notification[] }
GET /api/notifications?cursor= → lịch sử
POST /api/notifications/read { ids | "all" }
```

- Giữ cả hai (#12): đầu trang đọc `GET /api/status` (§5: `paused`, `reviewCount`, `attention`); popover Việc nền và thông báo đọc `GET /api/activity`. Cả hai làm mới qua sự kiện đẩy; mất kết nối sự kiện thì poll 3 giây. <!-- r32 -->
- Gộp: thông báo cùng `kind` trong 10 giây → tăng `count`, không tạo dòng mới.
- Toast chỉ hiện cho `review_ready`, `needs_attention`, `ready_to_post`, `task_done` của việc người dùng tự bắt đầu, và `action_result`.
<!-- r3-dc -->

## §12 · Khung đăng lặp lại (đổi, Gói V5 + V4) <a id="posting-slots"></a>

`ytf channel posting-times` hiện trả khung hay đăng nhất. UI cần **mọi** khung lặp lại.

```ts
interface PostingSlot {
  weekdays: number[];        // 0 = T2 … 6 = CN, theo múi giờ đang xem
  start: string; end: string; // "06:00", "08:00"
  count: number;             // số video trong 30 video gần nhất rơi vào khung
  weeksHit: number; weeksTotal: number;   // độ đều: 6/6 tuần
  rhythm: "weekly" | "most_weeks" | "biweekly" | "new";
  isTop: boolean;
  competitorIds?: string[];  // chỉ khi gộp nhiều đối thủ (trang Kênh)
}
GET /api/channels/<id>/posting-schedule?zone=vn|ny → { total: 30, perWeek: number, slots: PostingSlot[], offSlot: number, grid: number[7][12] }
GET /api/owner-channels/<id>/posting-schedule?zone=vn|ny → { total: number, slots: PostingSlot[] }   // gộp các đối thủ liên kết
```

**Mỗi tuần** (`perWeek`) <!-- r28-explore -->: (số video có giờ đăng − 1) ÷ độ dài thật của mẫu tính bằng tuần (từ video cũ nhất tới mới nhất), tối thiểu 1 tuần. Hiện: ≥ 1 → số nguyên (“6 video”), < 1 → một chữ số lẻ (“0,4 video”). Ví dụ 3 video/ngày, 30 video trong 9 ngày → ≈ 22.

Tìm video của đối thủ: `GET /api/rivals/<id>/videos?q=<≤200 ký tự>&…` — cộng với lọc và sắp xếp, không phân biệt hoa thường và dấu.

Cách tính khung (một hàm thuần dùng chung cho UI và CLI):
1. Đổi giờ đăng chính xác của 30 video sang múi giờ đang xem (New York tự tính giờ mùa hè/đông; thứ đổi theo khi qua nửa đêm).
2. Gom theo (thứ, khung 1 giờ); gộp hai khung giờ liền kề cùng thứ.
3. Một khung là **lặp lại** khi có ≥ 3 video và rơi vào ≥ 2 tuần khác nhau.
4. Các thứ có cùng khoảng giờ và cùng độ đều được gộp thành một dòng (T3, T5 · 06:00–08:00).
5. Sắp theo `count` giảm dần; video không thuộc khung nào đếm vào `offSlot`.
6. Gộp nhiều đối thủ: hai khung trùng nhau ≥ 50% thời lượng và cùng thứ được gộp, `competitorIds` là hợp các đối thủ.
<!-- r4-slots -->

## §13 · Prompt ảnh (mới, Gói V15) <a id="image-prompts"></a> <!-- r7-prompts -->

```ts
type ImagePrompt = {
  id: string;                         // có sẵn: "builtin:scene" | "builtin:cover-<ThumbnailStyle>"  <!-- r9-assets -->
  name: string;                       // ≤ 60 ký tự, duy nhất
  kind: "scene" | "cover";
  builtIn: boolean;                   // true: chỉ đọc, không xoá
  composition?: ThumbnailStyle;              // kind = cover → recipe.thumbnail ('clean' | 'hook' | 'circle-arrow' | 'hook-arrow' | 'split' | 'closeup')
  prompt: string;                     // ≤ 2.000 ký tự; có sẵn có thể rỗng (dùng house style)
  samples: string[];                  // id ảnh mẫu đã vẽ xong (tối đa 12, mới nhất trước)
  sampleUrls: string[];               // URL ảnh mẫu thật (GET .../samples/{id}/image), cùng thứ tự samples
  usedBy: string[];                   // templateId, tính khi đọc
  updatedAt: string;
};
```

API:
- `GET /api/image-prompts?kind=scene|cover&q=` → `ImagePrompt[]` (có sẵn trước, rồi theo `updatedAt`).
- `POST /api/image-prompts` `{ name, kind, composition?, prompt, fromId? }` (`composition` chỉ khi `kind = cover`) → tạo hoặc nhân bản (`fromId`).
- `PATCH /api/image-prompts/:id` → sửa; `403` với prompt có sẵn; đổi `kind` khi `usedBy` khác rỗng → `409 { reason: "in_use", usedBy }`.
- `DELETE /api/image-prompts/:id` → `409 { reason: "in_use", usedBy }` khi đang dùng; `403` với có sẵn.
- `POST /api/image-prompts/:id/samples` `{ videoId, sceneId? }` → tạo việc nền (§11) vẽ 1 ảnh; xong thì thêm vào `samples`.
- `GET /api/image-prompts/:id/export` · `POST /api/image-prompts/import` (tệp `.json` gồm các trường trên, không có `usedBy`, `samples` tuỳ chọn).

Recipe template:

```ts
recipe.scenes.style: "full" | "cutout";  // giữ nguyên: Cách ghép ảnh vào video (Cận cảnh = cutout + framing "close")
recipe.scenes.framing?: "close";       // mới, cho Cận cảnh
recipe.scenes.scenePromptId: string;   // thay imagePrompt + imagePromptMode
recipe.scenes.coverPromptId: string;   // thay thumbnail
```

Lúc video bắt đầu (L9), app phân giải prompt và ghi vào manifest đúng các trường đang có: `imagePrompt ← prompt`, `imagePromptMode ← builtIn ? "blend" : "custom"`, `thumbnail ← composition`, và trường mới `coverPrompt ← prompt của prompt bìa`. `style` lấy thẳng từ recipe. Nhờ vậy `storyFingerprint`, `buildStoryPrompt`, `thumbnailPrompt` giữ nguyên chữ ký. <!-- r9-assets -->

## §14 · Ngoại lệ ảnh của một video (mới, Gói V16) <a id="prompt-override"></a> <!-- r8-pm --> <!-- r9-assets -->

Hai lớp cho cả video: **template → video** (ia-flows L14). Lớp thứ ba (đợt 33): prompt của **từng ảnh** khi vẽ lại, xem §26. <!-- r33-img -->

Ngoại lệ được lưu cùng manifest ảnh của video (`images/manifest.json` và checkpoint được CAS trong PostgreSQL), không sửa snapshot template. `scenePromptSet` / `coverPromptSet` phân biệt lựa chọn của video với giá trị kế thừa; API nhận `null` để xoá lựa chọn đó và quay về template.

```ts
type VideoImageOverrides = {
  scenePromptSet: boolean;
  scenePromptId?: string;
  coverPromptSet: boolean;
  coverPromptId?: string;
  noImages?: boolean;
  drawProvider?: string;   // nguồn vẽ riêng của video (lệnh /provider): provider id đã kết nối
  drawModel?: string;      // model ảnh của nguồn đó
};
```

- Recipe hiệu lực lấy template trước, rồi đè lựa chọn của video; worker khôi phục manifest đã công bố trước khi reconcile để không làm mất ngoại lệ sau một lượt dựng thành công.
- `drawProvider`/`drawModel` là nguồn vẽ riêng của **một** video (lệnh `/provider`): lần vẽ sau dùng nguồn này trước, rồi tới chuỗi nguồn chung của Hoạ sĩ; ảnh đã vẽ không bị vẽ lại, video khác không đổi.
- `style` (cách ghép) **không** đè được theo video; đổi thì Làm lại từ đầu với template khác.

API:
- `POST /api/clone/:id/prompts/plan` `{ scenePromptId?: string | null, coverPromptId?: string | null, noImages?: boolean }` → `{ redraw: { scenes: number, cover: boolean }, cost: { images: number }, quotaLeft: number, rebuild: boolean, enablesImages: boolean }`. `enablesImages` = video bật lại ảnh cảnh; `redraw.scenes` là số ảnh cảnh sẽ vẽ (template “Không vẽ ảnh”: 6, cắt lại cảnh).
- `PATCH /api/clone/:id/prompts` nhận body của `plan` cùng `expectedRevision: number`; khi `cost.images > 0`, bắt buộc `confirmCount` bằng đúng tổng ảnh trong kế hoạch. Áp dụng và trả về như `plan`; revision hoặc số xác nhận cũ → `409`.
- Với video còn trong Chờ lượt, `:id` là `queueItem.id`; `expectedRevision` lấy từ `queueItem.overrideRevision`. `plan` luôn có `redraw.scenes = 0`, `redraw.cover = false`, `cost.images = 0`, `rebuild = false`; không cần xác nhận chi phí. Lựa chọn được lưu trên hàng chờ và áp dụng khi video bắt đầu, không sửa snapshot template. Nếu mục đã chạy, bị bỏ hoặc revision đã đổi, `PATCH` trả `409`. SSE `queue.updated` báo cập nhật hàng chờ.
- Vẽ lại một ảnh: `POST /api/clone/:id/images/:imageId` `{ op: "regenerate" | "editPrompt", prompt?: string, expectedRevision: number }`. `prompt` là góp ý (`direction`), không nhận `promptId`; ảnh đã duyệt được giữ đến khi duyệt ứng viên mới.
- Duyệt một ảnh dùng cùng route `{ op: "approve", version: number, expectedRevision: number }`; duyệt hàng loạt qua `POST /api/clone/:id/images` `{ op: "approveAll", expectedRevision: number }` ghi ý định chủ duyệt bền vững (không có `accept`). Lần duyệt đầu commit manifest rồi để pipeline dựng lại nếu cần. Khi mọi ảnh đã duyệt và build đang đỗ `awaiting_review`, route mở đúng bản dựng đã công bố qua `PrepareCloneExport` trong cùng transaction (giữ `output_manifest` của build, chỉ reset `score`/`export`), không dựng lại; khi build còn `pending`/`running` thì ý định chờ worker tiêu thụ. Lặp lại khi build đã `succeeded` là no-op, không đổi revision và không thêm lượt dựng. Cổng Độ hoàn thiện vẫn chặn xuất (`accept` chỉ đến từ `POST /export`). Vẽ lại hàng loạt `{ op: "regenerateAll", expectedRevision: number, confirmCount: number, kind?: "scene" | "thumbnail" }` kiểm tra số ảnh ở server trước khi xếp hàng.
- `POST /api/clone/:id/cover?expectedRevision=N` nhận bytes PNG/JPEG/WebP tối đa 20 MB; ảnh thật được decode, chuẩn hoá WebP 1280×720 và duyệt ngay. `GET /api/clone/:id/images/:imageId?variant=cutout` trả PNG tách nền đã công bố; `GET /api/clone/:id/thumbnail` tải JPG.
- `GET /api/clone/:id/backdrop` trả `{ backdrop }`; `POST` nhận `{ op: "reroll" | "swap" | "add" | "remove", revision?: string, itemId?: string, channelId?: string, videoId?: string, startSec?: number, endSec?: number }`. Revision chuỗi cũ → `409`; `reroll` có `itemId` chỉ đổi một slot. Chỉ xoá clip thì dựng lại; clip mới/đổi chạy bước lấy nền trước.
- Nền của timeline giữ `cloneBackdropId`; bản đọc nền có `editorMediaId`, `rebuilding` và `projectRevision` để UI chờ đúng revision dựng xong, không nhầm revision autosave. Ảnh cảnh dùng timeline id `${projectId}-scene-${sceneId}`.
- `POST /api/clone/bulk` nhận `{ action, jobIds, expectedRevisions? }`, tối đa 100 video, trả lỗi riêng từng video; duyệt ảnh hàng loạt cần revision ảnh theo từng video. `GET /api/clone/download?ids=id1,id2` nhận tối đa 50 video đã xuất xong, stream ZIP từ bản xuất đã công bố (MP4, JPG bìa, ảnh cảnh WebP và thông tin đăng); không nhận đường dẫn tệp từ client.
- Mọi thay đổi được công bố cùng CAS checkpoint và invalidation pipeline; SSE `clone.job.updated` cập nhật UI. Vẽ lại đi qua hàng đợi ảnh hiện hữu, D7 và receipt, không gọi một nhánh tạo ảnh riêng.
- Bulk `approve` là hành động **Duyệt cả video & dựng**: ghi ý định chủ duyệt bền vững theo revision ảnh trong cùng giao dịch, worker dựng lại rồi dùng đúng cổng `PrepareCloneExport` khi build tới `awaiting_review` và đã nhả lease. Ý định sống qua khởi động lại; sửa ảnh/nền sau đó huỷ ý định cũ. Không truyền `accept`, nên điểm dưới 90 vẫn bị chặn trước xuất.

Reconcile: prompt cảnh đổi → vẽ lại item cảnh đã vẽ, item chưa vẽ dùng prompt mới; prompt bìa đổi → chỉ item bìa; `noImages` → bỏ item cảnh, dựng từ nền. Video đã dựng: ảnh cảnh đổi → dựng + xuất lại; chỉ bìa đổi → chỉ cập nhật gói đăng.

## §15 · Kết nối AI và model của trợ lý (mới, Gói V17) <a id="ai-providers"></a> <!-- r8-ai -->

```ts
type Capability = "text" | "vision" | "image";
type AiProvider = {
  id: string;
  kind: "chatgpt" | "grok" | "openai" | "gemini" | "openrouter" | "deepseek" | "xai" | "anthropic" | "ollama" | "openai_compat";
  auth: "subscription" | "api_key" | "local";
  label: string;
  keyHint?: string;                   // "••••7c2e"; khoá đầy đủ không bao giờ trả về
  baseUrl?: string;                   // ollama, openai_compat
  status: "ready" | "logged_out" | "login_pending" | "auth_failed" | "unreachable";
  quota?: { used: number; limit: number; period: "day" | "week"; resetAt: string };   // gói đăng nhập
  spendMonth?: number;                // khoá API, USD
  catalogAt?: string;
  catalogCount: number;
  selected: string[];                 // model id đã chọn
};
type AiModel = { id: string; caps: Capability[]; contextK?: number; price?: { inPerM?: number; outPerM?: number; perImage?: number }; recommended?: boolean; manual?: boolean };
type ModelRef = { provider: string; model: string };
type AgentModels = {
  director: { model: ModelRef; effort: "auto" | "low" | "medium" | "high"; fallback: ModelRef[] };
  artist: { promptModel: ModelRef; draw: ModelRef; fallbackOn: boolean; drawFallback: ModelRef[] };
};
```

API:
- `GET /api/ai/providers` → `AiProvider[]`.
- `POST /api/ai/providers` `{ kind, apiKey?, baseUrl? }` → tạo + lấy danh mục; khoá sai → `422 { reason: "auth_failed", status: 401 }` (thẻ vẫn được tạo với `status: "auth_failed"` để sửa khoá).
- `GET /api/ai/providers/:id/models?q=&filter=selected|recommended|all` → `AiModel[]`.
- `PATCH /api/ai/providers/:id` `{ selected?, apiKey?, baseUrl? }`.
- `POST /api/ai/providers/:id/test` → `{ ok, ms }`. `POST /api/ai/providers/:id/models/check` `{ model }` → `{ ok, ms, caps, contextK }` rồi thêm với `manual: true`.
- `DELETE /api/ai/providers/:id` → `409 { reason: "in_use", agents }` khi đang được gán.
- `GET /api/ai/agents` · `PUT /api/ai/agents` `AgentModels` → `422 { reason: "capability_missing", need }` khi model thiếu `text` (director), `vision` (artist.promptModel) hoặc `image` (artist.draw / drawFallback).

Đối chiếu code hiện tại:
- `chatgpt` = Codex CLI (`tools:codex`): `gpt-6.1-sol` (text + vision), `gpt-image-2` (image). `grok` = grok CLI: `grok-imagine-image`.
- `clone.imageProvider` → `artist.draw`; `clone.imageFallback` → `artist.fallbackOn`; `drawFallback` mới (mặc định: nguồn còn lại).
- `metadata.ts`, `branding.ts` (text) và đường viết mô tả ảnh trong `images.ts` (vision) gọi qua adapter theo `AgentModels` thay cho Codex cố định.
- Mỗi step ghi `modelRef` vào job khi bắt đầu; đổi cài đặt không ảnh hưởng step đang chạy.

## §16 · Hồ sơ kênh và nội dung đăng (đợt 9) <a id="channel-profile"></a> <!-- r10-channel -->

**Kênh** (`owner_channels`, thêm cột):

```ts
language: "en" | "vi" | "auto";        // mới; "auto" = theo ngôn ngữ video nguồn (hành vi hiện tại)
styleNote: string | null;              // đã có: Giọng văn
descriptionFooter: string | null;      // mới, ≤ 1.000 ký tự, thêm nguyên văn vào cuối mô tả
alwaysTags: string[];                  // mới, ≤ 10, đứng đầu danh sách tag
```

**Nhận diện** (đã có trong `branding.ts`, chỉ nói rõ cho UI):
- `BrandingState { brief, summary, look, avatar, banner }`: `brief` = Mô tả kênh (sửa được), `summary` = câu tiếng Việt hiện ở “Phong cách nhận diện”, `look` = phong cách nhận diện dùng cho avatar, ảnh bìa và ảnh video ở prompt **Theo nhận diện kênh** (`imagePromptMode = blend`).
- Tạo lại: `POST /api/owner-channels/:id/branding` `{ part: "all" | "name" | "avatar" | "banner", brief, direction? }` (theo `brandingInputSchema`). Avatar hoặc ảnh bìa = 1 ảnh; cả bộ = 2 ảnh.
- Trạng thái: `autoBrandingStatus` → UI `ready | generating | failed`; `failed` kèm `error` và giờ thử lại.
- Tham chiếu = đối thủ liên kết đầu tiên; bỏ liên kết nó thì đối thủ kế tiếp thành tham chiếu cho lần tạo lại sau.
- `part: "all"` gửi `brief` hiện tại (kể cả bản người dùng sửa) làm hướng; có thể trả tên mới. <!-- r11-audit -->

**Bỏ liên kết đối thủ** (`unlinkCompetitor`, đã có): giữ video đã làm và `clone_queue` đã có; nếu không còn đối thủ nào thì đặt `autoPick.enabled = false` và trả `autoPickPaused: true` để UI ghi lý do.

**Nội dung đăng của một video** (`metadata` artifact, `cloneMetadataSchema`):
- Step `metadata` thêm `language`, `descriptionFooter`, `alwaysTags` vào prompt; app (không phải model) nối `descriptionFooter` vào cuối và đặt `alwaysTags` lên đầu, rồi cắt tag của Đạo diễn cho vừa 30 tag / 500 ký tự.
- `PATCH /api/clone/:id/metadata` `{ title?, description?, tags? }` → kiểm theo `cloneMetadataSchema`; ghi `editedBy: "owner"`, `editedAt`; bản trước vào `metadata-history`.
- `POST /api/clone/:id/metadata/rewrite` `{ parts: ("title" | "description" | "tags")[], direction?: string }` → chạy lại step `metadata` cho các phần chọn (không tốn ảnh), giữ phần không chọn.
- Lịch sử: `GET /api/clone/:id/metadata/history` → tối đa 10 bản `{ at, by: "director" | "owner", parts, title, description, tags }`; `POST /api/clone/:id/metadata/restore` `{ at }` đưa bản đó thành bản đang dùng (bản hiện tại vào lịch sử). <!-- r11-audit -->
- Trạng thái cho UI: `pending` (chưa tới step `metadata`), `writing` (step đang chạy), `failed` (step lỗi; video vẫn dựng tiếp, chỉ chặn Tải gói đăng), `ready`.
- Gói đăng (`download-bundle.ts`) đọc bản mới nhất.
- Ảnh bìa của bạn: `POST /api/clone/:id/cover` (đã có, `uploadCloneCover`).

## §17 · Lý do của trợ lý (mới, Gói V18) <a id="reason"></a> <!-- r16-agent -->

```ts
interface Reason {
  agent: AgentId;
  text: string;                       // “Lượt xem/giờ gấp 4,2 lần mức thường của Nocturne Stories”
  facts: { label: string; value: string; source: string }[];   // 2–4 dòng, source = “YouTube · 08:10”, “30 video gần nhất”…
  fix?: { label: string; href: string };                         // “Sửa quy tắc viết”
}
```
- Lý do số liệu còn dùng cho điểm video/ảnh, quyết định lấy nguồn và ba loại đề xuất sống. Sinh bằng code từ dữ liệu đã có, không gọi endpoint giải thích hoặc model chỉ để dựng câu.
- Không còn thêm quy tắc đã học vào prompt metadata/ảnh. Dấu vết đã gửi của bản cũ là provenance lịch sử, không phải chỉ dẫn cho lượt kế tiếp.

## §18 · Ba loại đề xuất dùng chung (Gói V37) <a id="suggestions"></a>

```ts
type SuggestionKind = "hot_video" | "template_failing" | "cleanup";
interface Suggestion {
  id: string; kind: SuggestionKind; agent: AgentId;
  channelId: string | null; channelName: string | null;
  title: string; evidence: string;
  reason: Reason;
  primary: {
    label: string; op: string; params: object;
    cost?: { images?: number }; href?: string;
  };
  createdAt: string; expiresAt: string;
  state: "open" | "accepted" | "dismissed" | "expired";
}
GET  /api/suggestions?channelId=        → Suggestion[] // tối đa5open, xếp theo giá trị
POST /api/suggestions/:id/accept { confirm?: boolean }
POST /api/suggestions/:id/dismiss {}
```

- Hiện thành dòng trong **Chờ bạn**, không phải dock/thẻ độc lập. `primary.href` chỉ điều hướng; không gọi accept cho thao tác này.
- Khi cần xác nhận: `409 { code: "confirm_required", cost: { images } }`. Xác nhận một lần, gửi lại `{ confirm: true }`; dọn MP4 nêu rõ số video/dung lượng và không đụng dự án, nguồn hoặc video chưa đăng.
- Khoá trùng `kind + channelId + đối tượng`; bỏ qua nghỉ7ngày. Không còn `forever` hoặc tắt theo từng loại/kênh; bộ sinh và truy vấn chỉ nhận ba loại sống.
- `GET/PUT /api/notifications/settings`: trường `suggestions` là **boolean**, mặc định `true`; `false` ngừng sinh và không trả đề xuất. Các thiết lập thông báo khác giữ nguyên. Bỏ `recapOnReturn`.
- Migration [`00038_global_suggestions.sql`](../../../internal/persistence/migrations/00038_global_suggestions.sql) đổi dữ liệu cũ trong cùng transaction: cả ba loại sống đều bị tắt toàn cục → `false`; chưa tắt loại sống nào → `true`. Tắt một phần hoặc theo kênh không biểu diễn được bằng một công tắc nên **dừng để chủ dữ liệu chốt**, không tự bật lại. Dữ liệu sai kiểu/loại lạ cũng dừng để quản trị kiểm tra. Đóng đề xuất mở đã bỏ, giữ lịch sử đã giải quyết; gỡ con trỏ học/bản tin và tác vụ kế hoạch cũ.
- Ràng buộc cơ sở dữ liệu chỉ cho ba loại sống có trạng thái `open`; loại đã bỏ được giữ để đọc lịch sử nhưng không được tạo mới hoặc mở lại.

## §19 · Quy tắc đã học đã ngừng (Gói V37) <a id="learned-rules"></a>

Không phát hiện, lưu hay tiêm quy tắc học từ sửa metadata/góp ý ảnh. Không còn endpoint, trường phản hồi `learn`, nút Lưu thành quy tắc hoặc công việc học ngầm. Lịch sử metadata, góp ý và dấu vết prompt đã gửi vẫn là dữ liệu thật của video; không xoá để giả lập prompt mới và không dùng làm chỉ dẫn cho lượt mới.

## §20 · Mức tự chủ và điểm ảnh (mới, Gói V20) <a id="autonomy"></a>

```ts
interface Autonomy {
  preset: "careful" | "balanced" | "auto" | "custom";
  pick: "suggest" | "score2" | "all";               // thay autoPick.enabled
  approve: { mode: "ask" | "score" | "all"; min: number };   // min 70–95, mặc định 85; thay autoApprove
  onError: "stop" | "retry_once";
  belowQuality: "ask" | "fix_then_ask";
}
owner_channels.autonomy  (JSON)        // migration: autoApprove=true → approve.mode="all"; false → preset "balanced"
owner_channels.priority: "steady" | "views"
ai_providers.dailyPaidImageLimit: number | null   // mặc định 50 (D7); 0 = không tự tiêu
image.score: { value: number; parts: { match: number; artifacts: number; identity: number; duplicate: number }; reason: Reason }
```
- Điểm ảnh: Kiểm định chấm sau khi ảnh vẽ xong bằng model đọc ảnh của Hoạ sĩ (§15), không tốn ảnh. `approve.mode="score"` và mọi ảnh ≥ `min` → bỏ qua Chờ duyệt, ghi `action_result` “Tự duyệt theo điểm”.
- `retry_once`: chỉ với mã lỗi trong danh sách tự sửa được (`source_download_failed`, `image_failed`, `render_failed`); thêm `job.autoFix = { at, code, result }`.
- `fix_then_ask`: chạy lại các mục chưa đạt có cách tự sửa (vẽ lại ảnh lỗi, dựng lại phụ đề); đo lại một lần.

## §21 · Số liệu sau đăng và nhận xét (mới, Gói V21) <a id="performance"></a>

```ts
interface PostedStats { jobId: string; youtubeId: string; at24h?: Stat; at7d?: Stat; at28d?: Stat; vsMedian7d?: number }
interface Stat { views: number; likes: number; comments: number; readAt: string }
interface Insight {
  id: string; channelId: string; dimension: "template" | "title_pattern" | "length" | "post_hour";
  text: string;                     // “Tiêu đề dạng câu hỏi: lượt xem 7 ngày gấp 1,8 lần”
  groups: { label: string; n: number; median7d: number }[];
  preliminary: boolean;             // < 20 video
  apply?: { op: "template.rotation"; params: object };
}
POST /api/clone/:id/posted { youtubeUrl? }     // đã có “Đánh dấu đã đăng”, thêm link
GET  /api/channels/:id/performance → { posted: PostedStats[]; insights: Insight[]; needMore: number }
```
- Nguồn số: theo quyết định D5. Đọc ở mốc 24 giờ, 7 ngày, 28 ngày; lỗi đọc không chặn gì, chỉ để trống.

## §22 · Tiến độ thay cho Bản tin và Kế hoạch (Gói V37) <a id="recap"></a>

Tiến độ Bàn dựng đọc trạng thái/bước/nhật ký của job hiện tại và `GET /api/activity`; nhóm công việc không liên quan không được giả làm tiến độ video. Hiện hai dòng nhật ký mới nhất và nút Nhật ký N bước theo dữ liệu thật. Bản tin kể lại hoạt động, parse/plan/run và hoàn tác kế hoạch không còn API; danh sách nhóm endpoint đã ngừng ở §9.

## §23 · Thư mục lưu dự án (mới, Gói V22) <a id="storage"></a> <!-- r17-storage -->

```ts
interface StorageLocation {
  path: string;                    // tuyệt đối, ví dụ "/Volumes/Samsung T7/Viderse Projects"
  volume: { name: string; kind: "internal" | "external" | "network" | "cloud_sync"; fs: string; freeBytes: number };
  usedBytes: number;               // assets dự án ở thư mục này
  online: boolean;                 // false khi ổ bị rút / thư mục biến mất
  previous?: { path: string; bytes: number; deleteAfter: string };   // bản cũ giữ 7 ngày sau khi chuyển
}
GET  /api/storage → { projects: StorageLocation; stayBytes: number; suggestions: { path: string; volume: StorageLocation["volume"] }[] }
POST /api/storage/check { path, mode: "move" | "new_only" }
  → { ok: boolean; createSubdir: boolean; checks: { kind: "space" | "write" | "cloud" | "fs" | "same_volume"; status: "ok" | "warn" | "fail"; text: string }[]; needBytes: number; etaSec: number | null }
POST /api/storage/move { path, mode } → { taskId }       // BackgroundTask kind "storage_move" (§11)
POST /api/storage/move/:taskId/cancel
POST /api/storage/previous/delete
POST /api/storage/use-default-for-new                   // khi ổ bị rút: video mới lưu lại vào máy này
```
- `kind: "storage_move"` thêm vào `BackgroundTask.kind`; tiến độ theo nhóm `{ group: "images" | "editor" | "sources" | "exports", done, total }`.
- `Notification.kind = "needs_attention"` với `objectRef.type = "storage"` khi `online` chuyển sang `false`.

## §23.1 · Lưu trữ khi chạy trên máy chủ (mới, Gói V25) <a id="storage-server"></a> <!-- r29-s3 -->

Chỉ có khi Viderse chạy trên máy chủ. Bản desktop vẫn dùng §23. Phần cấu hình chỉ trả cho quản trị viên; thành viên nhận `usage` và phần dọn của §23.

```ts
type StorageKind = "disk" | "s3";
type S3Provider = "garage" | "r2" | "aws" | "gcs" | "wasabi" | "b2" | "do" | "other";
type Delivery = "proxy" | "redirect";            // Qua máy chủ Viderse | Trực tiếp từ kho (link ký)
type ValueSource = "env" | "app" | "default";    // biến môi trường · lưu trong app · mặc định
type StorageErrorCode = "wrong_key" | "no_bucket" | "no_write_permission" | "no_delete_permission" | "unreachable_endpoint"
  | "clock_skew" | "cors_blocked" | "browser_unreachable" | "storage_error";
interface ServerStorage {
  kind: StorageKind;
  provider?: S3Provider;
  endpointHost?: string;           // chỉ host[:cổng], ví dụ "garage:3900"
  presignEndpoint?: string;        // Địa chỉ kho cho trình duyệt (redirect + địa chỉ nội bộ) — S3_PRESIGN_ENDPOINT
  delivery: Delivery;
  region?: string; bucket?: string; forcePathStyle?: boolean; allowInsecure?: boolean;
  accessKeyLast4?: string;         // "c41d"; khoá bí mật không bao giờ trả về
  sources: Partial<Record<"provider" | "endpoint" | "region" | "bucket" | "keys" | "delivery" | "presignEndpoint" | "forcePathStyle" | "allowInsecure" | "cache", ValueSource>>;
  encryptionKey: { ok: boolean; path: string; issue?: "readonly_dir" | "bad_permissions" | "corrupt" };   // <thư mục cấu hình>/secret.key, 0600; ok false → s3nokey
  online: boolean; checkedAt: string; lastError?: StorageErrorCode;
  disk?: { path: string; freeBytes: number };                          // khi kind = "disk"
  previous?: { kind: StorageKind; label: string; path?: string; files: number; bytes: number };   // chỗ cũ sau khi chuyển; không tự xoá
}
interface UsageGroup { group: "images" | "editor" | "library" | "sources" | "exports"; files: number | null; bytes: number | null }   // library = lớp phủ, video nền, ảnh kênh; nhóm khác null tới khi có luồng dựng video → “chưa có số”
interface OpStat { op: "put" | "get" | "presign" | "delete"; count: number; errors: number; bytes: number; avgMs: number; maxMs: number }   // từ lần khởi động
GET  /api/storage/server → { role: "admin" | "member"; storage?: ServerStorage; usage: UsageGroup[]; workCache?: { usedBytes: number; limitBytes: number }; ops?: OpStat[]; migration?: Migration }

interface S3Config { provider: S3Provider; endpoint?: string; accountId?: string; region?: string; bucket: string;
  accessKeyId?: string; secretAccessKey?: string;   // bỏ trống cả hai = giữ khoá đã lưu
  forcePathStyle?: boolean; allowInsecure?: boolean; delivery: Delivery; presignEndpoint?: string }
POST /api/storage/server/test { config: S3Config }
  → { testId; steps: { id: "write" | "read" | "range" | "presign" | "cors_probe" | "app_range" | "delete"; status: "ok" | "fail" | "skip" | "na" | "pending"; ms?: number }[];
      probeUrl?: string;           // redirect: link ký để TRÌNH DUYỆT tải thử Range bytes=0-1023
      error?: { code: StorageErrorCode; step: string } }
POST /api/storage/server/test/:testId/probe { ok: boolean; reason?: "cors" | "network" } → kết quả đủ 6 bước (server chạy nốt delete)
PUT  /api/storage/server/config { config: S3Config; testId } → { needsMigration: { files: number; bytes: number; groups: UsageGroup[] } | null }
     // 409 khi testId chưa đạt hoặc config khác lúc test; 423 khi encryptionKey = false; 403 khi giá trị do máy chủ đặt
POST /api/storage/server/migrate → { taskId }                      // BackgroundTask kind "storage_migrate" (§11)
POST /api/storage/server/migrate/:taskId/pause | resume | cancel | retry

type MigrationErrorCode = "content_mismatch" | "unreachable_endpoint" | "storage_error" | "wrong_key";
interface Migration {
  taskId: string; state: "running" | "paused" | "error" | "done" | "cancelled";
  from: { kind: StorageKind; label: string }; to: { kind: StorageKind; label: string };
  files: { done: number; total: number }; bytes: { done: number; total: number }; checked: number;
  rateBps: number; startedAt: string; finishedAt?: string;
  groups: { group: UsageGroup["group"]; files: { done: number; total: number }; bytes: { done: number; total: number }; state: "done" | "running" | "waiting" | "error" }[];
  errors: { group: UsageGroup["group"]; jobId?: string; title: string; detail?: string; code: MigrationErrorCode }[];   // FE dịch code (06-copy)
}
```
- **Biến môi trường** (đúng tên Noverse, tệp cấu hình riêng của Viderse; hỏi đáp #49): `STORAGE_DRIVER` (`disk|s3`) · `S3_PROVIDER` · `S3_ENDPOINT` · `S3_REGION` · `S3_BUCKET` · `S3_ACCESS_KEY_ID` · `S3_SECRET_ACCESS_KEY` · `S3_FORCE_PATH_STYLE` · `S3_ALLOW_INSECURE_ENDPOINT` · `S3_PRESIGN_ENDPOINT` · `BLOB_DELIVERY=proxy|redirect` · `BLOB_CACHE_DIR`, `BLOB_CACHE_MAX_BYTES` (bộ nhớ đệm làm việc → `workCache`). Giá trị đặt bằng biến thì `sources[k] = "env"`. Không có biến cho khoá mã hoá; chỉ một bản chạy. <!-- r29b -->
- Huỷ chuyển: tệp đã chép nằm lại kho mới; tệp đặt tên theo nội dung nên lần chuyển sau dùng lại, không chép lại. Dọn bằng lệnh bảo trì.
- Thời gian còn lại = (bytes.total − bytes.done) ÷ `rateBps` (trung bình trượt 30 giây). Chưa đủ 10 giây dữ liệu thì không hiện.
- Trong lúc chuyển: đọc ở chỗ cũ, ghi mới vào chỗ cũ, cuối cùng chép nốt phần ghi mới rồi đổi `kind` trong một bước. Còn `errors` thì không đổi.
- `online` false → `Notification.kind = "needs_attention"`, `objectRef.type = "storage"` (chỉ gửi cho quản trị viên) và `GET /api/status` trả `storageDown: { label: string; since: string }` để vẽ dòng cảnh báo đầu trang. Hold dùng lại `HoldOffline` của V22.
- Dọn (§23) có thêm nhóm: `orphan_objects` (tệp trên kho của video đã xoá), `work_cache` (chỉ quản trị viên; giữ tệp của video đang dựng).

## §25 · Đăng nhập dùng chung với Noverse (Gói V26) <a id="sso"></a> <!-- r30-sso -->

Hợp đồng gốc ở Noverse `11-api-contract.md` › *Đăng nhập chung với Viderse* (cookie `verse_session`, `User.grants`, 401/403/503). Phần dưới là chỗ giao diện cần thêm.

```ts
type AppId = "noverse" | "viderse";
interface AppLink { id: AppId; name: string; origin: string }    // r31-v27: theo dev (#50); nguồn biến VERSE_APP_LINKS
GET /api/me → { user: User; session: { expiresAt: string }; apps: AppLink[] }
// apps = các app người dùng vào được (kể cả app hiện tại), theo grants hiệu lực; quản trị viên luôn đủ hai
403 { code: "app_access_denied", app: "viderse", user: User, apps: AppLink[] }   // apps để màn chưa cấp quyền hiện Mở Noverse
401 { code: "session_ended", reason: "logout" | "password_changed" | "admin_changed" | "locked" | "expired" | "unknown", app?: AppId }   // đã có (#50); locked → câu admin_changed
POST /api/logout → 204; client về #/login?bye=1 để hiện “Đã đăng xuất”
POST /api/me/password → { ok: true, reauthenticate: true }; client về #/login?pw=1
```
- **Mở app kia:** `window.open(app.origin, "_blank", "noopener")` — mở ở thẻ mới để không mất việc đang dở. Quản lý thành viên: `<url Noverse>/#/settings/members` (thêm `?user=<tên>` nếu Noverse nhận).
- **Lý do đăng nhập lại** (`reason`, `app`): `logout` + `app` khác app hiện tại → “Bạn vừa đăng xuất ở …”; `password_changed`; `admin_changed` (đổi quyền app, đặt lại mật khẩu, khoá); `expired`. Thiếu `reason` → câu chung. `?bye=1` và `?pw=1` do chính client đặt, không cần máy chủ.
- `apps` rỗng hoặc chỉ có app hiện tại (chưa đặt `VERSE_APP_LINKS`) → không hiện nhóm Ứng dụng, không hiện *Mở Noverse*. Ở Thành viên (Noverse), thẻ quyền app vẫn hiện tên app và ô quyền, ẩn dòng địa chỉ và biểu tượng mở; không đoán địa chỉ. <!-- r31-v27 -->
- `reason: "unknown"` không có cờ `vv.signedInBefore` trên trình duyệt → coi như lần đầu, không hiện khối lý do (06-copy).
- Thiết bị: `GET /api/me/sessions` → `{ sessions: [{ id, device, browser, lastActiveAt, expiresAt, current }] }` — một phiên mỗi thiết bị, chung cho hai app; `POST /api/sessions/:id/revoke`, `POST /api/me/sessions/revoke-all` (đăng xuất mọi thiết bị khác).

## §24 · Video nền và nền tảng nguồn (mới, Gói V23) <a id="sources"></a> <!-- r22-sources -->

```ts
type PlatformId = "youtube" | "twitch" | "bilibili";   // registry; nền tảng mới thêm giá trị ở đây
interface PlatformStatus {
  id: PlatformId; label: string;
  uses: ("competitor" | "backdrop")[];
  kinds: ("clip" | "highlight" | "vod" | "video")[];
  targetHeight: number;                                   // YouTube 360 · Twitch 480 · Bilibili 1080
  login: { type: "none" | "browser_cookie" | "cookie_string"; saved: boolean; masked?: string; browserProfile?: string };
  state: "ready" | "paused" | "needs_login";
  pausedUntil?: string; pauseReason?: string; failureCount?: number;   // từ backdrop_platform_state
  lastError?: { code: "cookies_failed"; at: string; jobId?: string };   // YouTube “không phải bot”
  usage: { channels: number; videos: number; rivals?: number };
}
GET /api/platforms → PlatformStatus[]

type ParsedSource =
  | { kind: "channel"; platform: PlatformId; id: string; handle: string; url: string }
  | { kind: "video"; platform: PlatformId; id: string; url: string; title?: string; durationSec?: number }
  | { kind: "unsupported"; host: string; label: string }          // “Douyin”
  | { kind: "invalid"; message: string; recovery?: string };
POST /api/sources/parse { input } → ParsedSource                  // b23.tv: một lần đọc chuyển hướng

interface BackdropVideo {                                          // “Video lẻ” trong Thư viện › Video nền
  id: string;                    // "<platform>:<videoId>"
  platform: PlatformId; url: string; title: string; durationSec: number;
  segment: { fromSec: number; toSec: number };                   // toSec - fromSec ≤ 600
  file?: { path: string; height: number; bytes: number };
  state: "downloading" | "ready" | "failed"; progress?: number;
  usedBy: { templateId: string; name: string }[];
}
GET    /api/backdrop/videos → BackdropVideo[]
POST   /api/backdrop/videos { url, segment? } → BackdropVideo     // BackgroundTask kind "backdrop_download"
PATCH  /api/backdrop/videos/:id { segment }
DELETE /api/backdrop/videos/:id                                    // 409 khi usedBy không rỗng
POST   /api/platforms/requests { host }                            // ghi platform_requests, kèm vào gói Báo lỗi
```
- Template: `recipe.backdrop.fixedVideoIds: string[]` thay clip cố định nhúng; `GET /api/templates/:id` trả kèm `BackdropVideo` đã gắn.
- **Xem video của một kênh** (đợt 25) <!-- r25-backdrops -->:
```ts
GET /api/backdrop/channels/:id/videos?kind=clip|highlight|vod|video&sort=newest&cursor=&limit=24
  → { items: { videoId, kind, title, durationSec, publishedAt, thumbnailUrl, url, unavailable?: { reason } , chosenBy: { templateId, name }[] }[],
      counts: Record<Kind, number>, total: number, nextCursor?: string }
GET /api/backdrop/channels/:id → { …BackdropChannel, usedBy: { templateId, name, mode: "random" | "pick", kinds: Kind[], chosenCount?: number }[] }
```
- **Nguồn nền của template** giữ đúng dạng code (`templateBackdropSchema`): `channels: { channelId, kinds, pinned: videoId[] }[]` — `pinned` rỗng = **Ngẫu nhiên từ mọi video**, có phần tử = **Chỉ từ video tôi chọn**; `clipCount?` (bỏ trống = tự phủ kín), `clipSec`. Video lẻ là `fixedVideoIds`.
- **Ước lượng cho thanh Nền của một video:** `GET /api/templates/:id/backdrop/mix?lengthSec=2520` → `{ fixedSec, perChannel: { channelId, clips, freshVideos, repeats: boolean }[], loopSec, need, capped }` — dùng cùng hàm chia của `pickBackdropSources` (vòng tròn, kênh hết video nhường kênh khác, tối đa 40 clip) để số trên thanh khớp video làm ra.
- `BackdropChannel.platform` và `BackdropVideo.platform` dùng cùng `PlatformId`; chip lọc nền tảng ở Thư viện sinh từ `GET /api/platforms` (chỉ các nền tảng có `uses` chứa `backdrop`).

### §24.2 · Editor: việc cần sửa, khớp phụ đề, đo lại (Gói V13, đợt 27) <a id="editor-fix"></a> <!-- r27-editor -->
```ts
// mở rộng CloneScore (src/lib/clone-quality.ts): mỗi mục có fix + vùng lệch
interface QualityItemView { id: CloneQualityItemId; status: "pass" | "fail" | "unknown"; label: string; evidence?: string;
  fix?: { itemId?: string; href?: string };
  ranges?: { cueId: string; startSec: number; endSec: number; text: string; offsetSec: number }[];   // offset < 0: chữ sớm hơn tiếng
  stale?: boolean;                                                                                   // đã sửa sau lần đo gần nhất → “Cần đo lại”
}
GET  /api/editor/:projectId/quality → { total: number; measuredAt: string; items: QualityItemView[]; changedSinceMeasure: number }
POST /api/editor/:projectId/captions/align { cueIds: string[] | "all-drifting" } → { cues: { cueId, offsetSec, words: { w, startSec, endSec }[] }[] }   // khớp mốc từng từ theo giọng gốc
POST /api/clone/:jobId/measure → BackgroundTask (kind "measure")                                    // Đo lại độ hoàn thiện; Kiểm định chạy nền ~1 phút
POST /api/clone/:jobId/export { accept?: true }                                                      // accept = Vẫn xuất (ghi gate.accepted)
POST /api/clone/:jobId/quality { accept: boolean } → { accepted: boolean }                           // “Vẫn xuất” (accept) / “Bật lại kiểm tra Độ hoàn thiện” (không accept)
```
- Mở editor từ Cần xử lý: `?fix=captionAlignment&cue=<id đầu tiên>` → chọn sẵn đoạn, playhead tới `startSec`.
- `captions/align` là một thao tác hoàn tác được (một bước undo cho cả lệnh), lưu project như mọi sửa khác; không tự đo lại.

### §24.1 · Đăng nhập nền tảng bằng mã QR (Gói V24) <a id="qr-login"></a> <!-- r24-biliqr -->

Bản tham chiếu đã chạy: `yt-factory`: `src/server/backdrop/bilibili-qr-login.ts`, `bilibili-login.ts`, `src/app/settings/bilibili-cookie-controls.tsx`. API dưới đây là dạng HTTP cho verse của các server action đó.

```ts
// PlatformStatus.login mở rộng
interface PlatformLogin {
  type: "none" | "browser_cookie" | "cookie_string" | "qr";      // Bilibili: "qr" (vẫn nhận cookie dán tay)
  saved: boolean; masked?: string;                                // "SESSDATA=8f3a••••c21e"
  account?: string;                                               // tên tài khoản phụ, lấy khi kiểm tra cookie
  method: "qr" | "manual" | null;                                 // manual = cookie dán tay
  renewable: boolean;                                             // method = "qr" và có refresh token
  lastRefreshAt?: string; refreshError?: string;                  // refreshError → thẻ “Cần đăng nhập lại”
}
POST   /api/platforms/:id/login/qr        → { qrcodeKey, qrImage /* data:image/svg+xml */, loginUrl, expiresAt }   // mã sống 180 giây
GET    /api/platforms/:id/login/qr/:key   → { status: "waiting" | "scanned" | "expired" | "success" | "rejected"; account? }
POST   /api/platforms/:id/login/refresh   → PlatformLogin & { status: "refreshed" | "current" | "skipped" | "failed" }   // Gia hạn ngay (force)
PUT    /api/platforms/:id/login { cookie } → PlatformLogin        // cookie dán tay; kiểm tra với nền tảng trước khi lưu
DELETE /api/platforms/:id/login
```
- UI hỏi `GET …/qr/:key` mỗi 2 giây khi `waiting | scanned`; dừng khi đóng hộp thoại, khi hết `expiresAt` (tự chuyển `expired`, không cần hỏi server) hoặc khi có kết quả cuối. Lỗi mạng hay lỗi đọc dữ liệu → trạng thái lỗi kết nối.
- `success`: cookie và refresh token lưu cùng lúc (ghi một lần, hỏng thì giữ đăng nhập cũ); **xoá giờ chờ bị chặn** của nền tảng (`pausedUntil`, `failureCount`) nên `GET /api/platforms` trả `state: "ready"` ngay. Không kiểm tra được tên tài khoản vì mạng: vẫn lưu, `account` trống, `refreshError` = “Đã nhận đăng nhập; chưa kiểm tra được tài khoản do kết nối Bilibili…”.
- Tự gia hạn: trước mỗi lần quét kênh, tối đa một lần mỗi 24 giờ (`ensureFresh`); lỗi gia hạn không chặn lần quét (quét bằng cookie đang có). Thất bại → `refreshError`, `PlatformStatus.state = "needs_login"`.
- Một khoá chung cho đăng nhập, hỏi mã và gia hạn (không chạy song song). Cookie và trạng thái lưu tệp quyền 600 trên máy chạy app; API chỉ trả giá trị đã che.

## §26 · Ảnh, phiên bản và prompt đã gửi (mới, Gói V29) <a id="image-versions"></a> <!-- r33-img -->

Mỗi lần vẽ một ảnh (cảnh hoặc bìa) là một **phiên bản**. Máy chủ lưu đúng văn bản đã gửi nhà cung cấp và ảnh tham chiếu đã gửi kèm, để giao diện hiện lại và vẽ lại từ đó.

```ts
type PromptPartKind = "scene" | "style" | "refs" | "learned" | "avoid";
interface PromptPart { kind: PromptPartKind; text: string; source: { type: "artist" | "artist_feedback" | "image_prompt" | "template" | "user"; id?: string; label: string } }
interface ImageRef { id: string; role: "layout" | "character" | "style" | "object"; url: string; source: { type: "rival_frame" | "video_image" | "upload"; label: string; videoId?: string; atSec?: number; imageId?: string } }
interface ImageVersion {
  n: number;                              // 1, 2, 3…
  state: "drawing" | "ready" | "failed" | "cancelled";
  url: string | null; previewUrl: string | null;   // previewUrl: ảnh xem dần khi nhà cung cấp gửi
  width: number; height: number;          // kích thước gốc → giới hạn khung hiển thị
  parts: PromptPart[];                    // theo thứ tự ghép
  wrapper?: { guard: string[]; tool?: string[]; provider?: string };  // trường thêm của dev (xem 10-gap-closure › Gói V29): phần không sửa được — hàng rào an toàn và chỉ dẫn công cụ của đúng nguồn vẽ
  sentText: string;                       // nguyên văn đã gửi: wrapper.guard + các phần + wrapper.tool, mỗi khối cách nhau "\n\n"
  refs: ImageRef[];
  provider: "chatgpt" | "grok" | "gemini" | string; model: string; params: { size: string; quality?: string };
  feedback: string | null;                // góp ý sinh ra bản này
  diffFromPrev: Array<[string, "" | "ins" | "del"]> | null;   // chỉ phần scene
  startedAt: string; finishedAt: string | null; durationSec: number | null; costImages: number;
  error: { reason: "quota" | "filter" | "network" | "provider"; retryAt?: string; message: string } | null;
}
GET  /api/clone/:jobId/images → { images: Array<{ id; kind: "scene" | "cover"; index; label; usedVersion: number; versions: ImageVersion[]; refLimit: number }> }
POST /api/clone/:jobId/images/redraw {
  imageIds: string[];                     // 1 ảnh, hoặc nhiều ảnh (chọn nhiều, Vẽ lại cả N)
  project?: boolean;                     // Editor gửi false: tự thay ảnh và giữ lịch sử hoàn tác; bỏ qua = máy chủ thay project
  baseVersion?: number;                  // một ảnh: phiên bản đang xem làm nền prompt/refs, theo #84 đã chốt
  feedback?: string;                      // Hoạ sĩ sửa phần scene của từng ảnh
  parts?: Partial<Record<PromptPartKind, string>>;   // sửa tay, chỉ khi imageIds.length === 1
  refs?: Array<{ id?: string; upload?: string; videoImageId?: string; rivalFrame?: { videoId: string; atSec: number }; role: ImageRef["role"] }>;
  provider?: string;
} → { versions: Array<{ imageId; n }> }
POST /api/clone/:jobId/images/:imageId/use { n, project?: boolean } → bản đang dùng (project:false chỉ cập nhật manifest/import media; mặc định cập nhật cả project đã dựng)
POST /api/clone/:jobId/images/:imageId/versions/:n/cancel
SSE  image.progress { jobId, imageId, n, phase: "sending" | "drawing" | "receiving" | "done" | "failed", previewUrl?, elapsedSec }
```

- **Phần không sửa được:** hàng rào an toàn (dữ liệu cảnh/nhân vật/ảnh tham chiếu viết từ nguồn không tin được) và chỉ dẫn công cụ của nguồn vẽ nằm trong `wrapper`, không nằm trong `parts`; sửa `parts` không bao giờ chạm tới chúng. Chỉ Codex nhận chỉ dẫn công cụ; Grok và nguồn HTTP nhận hàng rào nhưng không nhận lệnh gọi `image_gen` hay luật đọc tệp.
- **Không sửa template:** `parts` chỉ áp cho ảnh đó. Không gửi `baseVersion` (đặc biệt vẽ nhiều ảnh): giữ chính sách nền từ bản đang dùng của từng ảnh. Gửi `baseVersion` cho một ảnh: giữ phần prompt và ảnh tham chiếu đã lưu của đúng bản đó, chỉ thay các phần/refs được gửi; không tự đổi `usedVersion`. Đây là quyết định #84 đã duyệt, không phải gọi `/use` trước khi sửa.
- **Phiên bản nền rõ ràng:** `baseVersion` là số dương của một bản thuộc chính ảnh duy nhất trong yêu cầu; không chấp nhận một số nền cho nhiều ảnh, bản không tồn tại hoặc chưa có dữ liệu prompt dùng được. Bản đang vẽ/đã huỷ không phải nền hợp lệ; bản lỗi có prompt/refs đã ghi dùng được cho sửa/thử lại. Ảnh còn chuyến vẽ chưa kết thúc từ chối lượt vẽ mới bằng `409 conflict`. Mọi từ chối xảy ra trước khi đổi manifest/revision. Nguồn nền giữ qua checkpoint/worker restart và độc lập với số bản đang dùng để kiểm race khi worker hoàn tất.
- **Giữ dữ liệu không sửa:** phần Cảnh/Phong cách/Tránh không được thay bằng nội dung từ bản đang dùng khác. Không gửi `refs` thì dùng đúng refs của nền đã chọn, kể cả danh sách rỗng; gửi `refs:[]` thì bỏ hết. Cùng ảnh/vai trò giữ nguyên lời tham chiếu và nguồn đã lưu; đổi refs thì dựng lại mô tả theo tệp hiệu lực. Wrapper bảo vệ/chỉ dẫn công cụ dựng lại theo nguồn vẽ; bỏ hướng góp ý cũ từng dính trong Cảnh trước khi ghép hướng mới, không sửa lịch sử hoặc biến toàn bộ `sentText` thành phần Cảnh sửa được.
- `refLimit` theo nguồn vẽ hiệu lực của video (ChatGPT 4, Grok 1; gồm thẻ nguồn riêng của video). Quá giới hạn, kể cả refs kế thừa từ `baseVersion` → 422 `{ code: "too_many_refs", limit }`; không nhận rồi âm thầm bỏ refs trong worker.
- Vẽ xong: bản mới thành `usedVersion` (trừ khi người dùng đã chọn bản khác trong lúc vẽ). Video đã duyệt/dựng: ảnh mới đánh dấu `needsReview`, editor thay media của clip đó (giữ chuyển động, keyframe, thời lượng) và ghi vào lịch sử hoàn tác; Độ hoàn thiện đo lại mục ảnh.
- **Một nơi ghi project:** Editor gửi `project:false` khi vẽ lại và khi dùng bản ảnh; cờ này được giữ từ draft qua chuyến vẽ tới phiên bản, kể cả worker khởi động lại. Worker vẫn lưu ảnh và cập nhật manifest dựng nhưng không ghi đè project đang sửa. Editor áp một lệnh đổi media có hoàn tác rồi lưu bằng `If-Match` đã tải; không tự nhận revision mới để bỏ qua xung đột. Duyệt/Bàn dựng không gửi cờ nên vẫn thay project ở máy chủ.
- **Giữ media đã dùng:** lưu project và lệnh Editor giữ binding metadata, nguồn ảnh và liên kết media cùng giao dịch CAS với project. Đổi bản dựng về sau không làm mất ảnh cũ của project đã lưu hoặc bản có thể hoàn tác.
- **Đổi bản khi lượt vẽ còn giữ lease:** chọn một bản đã sẵn sàng chỉ đổi lựa chọn ảnh, không đánh dấu build/score/export cũ. Quy tắc giống nhau khi `images` còn chạy và ngay sau khi bước đã xong nhưng lease chưa nhả; worker vẫn là nơi ghi checkpoint đang chạy. Nhận bản mới từ SSE không được tự dựng lại project đang sửa. Hoàn tác đọc lại danh sách sau khi đồng bộ để nhãn “đang dùng” khớp canvas.
- **Hai bên nhận cùng bản ảnh:** rebind của worker và Editor dùng CAS của chính bước build, vẫn kiểm generation/hủy/fence/attempt/trạng thái. Worker chuyển bước không tự làm hỏng lượt đổi ảnh của chủ kênh. Chỉ coi xuất bản lặp là đã xong khi checkpoint JSONB đúng hệt nội dung cần ghi và mọi artifact có đúng ID, đường dẫn, digest, bước và metadata đã lưu; payload khác hoặc artifact ngoài video vẫn bị từ chối.
- Lỗi giữ nguyên `parts`, `refs` đã gửi để *Sửa prompt* mở lại đúng nội dung.
- Ghi chú đã học (§19) vẫn chỉ ghi khi người dùng bấm lưu; `parts.learned` hiển thị nhưng sửa ở đây không đổi Ghi chú đã học.
