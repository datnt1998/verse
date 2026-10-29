# 16 · Phản hồi thao tác & làm mới dữ liệu

Yêu cầu của chủ dự án: **phản hồi thao tác và dữ liệu luôn mới phải làm kỹ** trên mọi màn. Tài liệu này là quy chuẩn bắt buộc; mọi màn (kể cả Audio, dock, thẻ kết quả lệnh) phải theo.

## 1. Tầng dữ liệu dùng chung (`web/src/data/`)

- **Bộ nhớ đệm theo khoá**: `['library']`, `['project', p]`, `['story', p, s, 'snapshot'|'chapters'|'chapter', n|'outline'|'reviews'|'exports'|'audio'…]`, `['config', scope, …]`, `['me']`, `['users']`, `['sessions']`, `['jobs']`, `['tts']`…
- `useResource(key, fetcher, opts)` → `{data, error, status: 'loading'|'ready'|'error', refreshing, updatedAt, refresh()}`:
  - **stale-while-revalidate**: có dữ liệu cũ thì hiện ngay, tải lại ngầm; **không bao giờ làm trống màn khi tải lại**.
  - gộp yêu cầu trùng đang bay; huỷ (`AbortController`) khi khoá đổi/unmount.
  - tự tải lại khi: cửa sổ lấy lại focus / tab hiện lại (tối đa 1 lần mỗi 15 s mỗi khoá), mạng có lại, luồng sự kiện nối lại, nhận sự kiện làm mới khớp khoá (§2).
  - lỗi khi tải lại: giữ dữ liệu cũ, hiện dòng nhỏ “Không tải lại được · Thử lại” ở đầu panel; lỗi lần đầu: trạng thái lỗi của panel kèm nút Thử lại.
- `invalidate(prefix)` sau mọi thao tác ghi; `setData(key, updater)` cho cập nhật lạc quan; API ghi trả về tài nguyên mới để đặt thẳng vào cache khi có.
- `useAction(fn, {success?, error?, undo?, invalidate?: keys[], optimistic?})` → `{run, pending, error}` dùng chung cho mọi nút.

## 2. Làm mới tức thời (sự kiện)

- **Một luồng toàn cục** `GET /api/events` (SSE, gateway) cho mọi màn: `library.changed` · `project.changed {project}` · `story.changed {project, story, parts[]}` (`parts` ⊂ `snapshot, chapters, outline, reviews, exports, meta, audio, rules`) · `config.changed {scope, project?, story?}` · `account.changed {username}` · `session.revoked` · `job.created|job.progress|job.waiting|job.failed|job.done` (`14-jobs-dock.md`) · `ping` 20 s. Có `id` tăng dần; client nối lại bằng `Last-Event-ID`, backoff 1→2→5→10 s; sau khi nối lại thì `invalidate` toàn bộ khoá đang hiển thị.
- **Theo dõi thay đổi ngoài app**: gateway quét mtime/kích thước các tệp chính (chương, đại cương, thẩm duyệt, xuất bản, `story.json`, `project.json`, cấu hình, quy tắc, audio) của các truyện đang có người xem, 2 s/lần, phát `story.changed`/`project.changed`/`config.changed` — sửa trong TUI, trình soạn thảo, hay máy khác đều hiện lên web không cần F5.
- Luồng live của truyện (SSE worker) vẫn giữ cho Workspace; sự kiện engine (nộp chương, thẩm duyệt, đổi đại cương, xuất) cũng ánh xạ sang `invalidate` khoá tương ứng.
- Mọi ghi từ chính gateway/worker phát sự kiện tương ứng ngay sau khi ghi xong (không đợi lượt quét).

## 3. Phản hồi thao tác

| Tình huống | Quy chuẩn |
|---|---|
| Bấm nút gọi API | Nút vào trạng thái chờ **sau 150 ms** (tránh nháy): spinner trong nút, giữ nguyên bề rộng, `aria-busy`, chặn bấm lặp; đã hiện thì giữ tối thiểu 400 ms |
| Thành công | Thao tác tại chỗ: dấu ✓ trong nút 1,2 s hoặc chip “Đã lưu” cạnh tiêu đề; thao tác có kết quả ở nơi khác: `Toast` (pill đáy giữa) có hành động (Mở, Tải xuống…); việc dài: chuyển sang dock |
| Lỗi | Toast đỏ với **thông báo tiếng Việt từ server** + “Thử lại”; lỗi theo trường hiện ngay dưới trường, focus vào trường lỗi đầu tiên; không bao giờ nuốt lỗi |
| Xoá / thu hồi / gỡ | Ưu tiên **Hoàn tác** 5 s (toast có đếm ngược) với thao tác đảo được; không đảo được → hộp thoại xác nhận nêu rõ hậu quả |
| Bật/tắt, đổi tên, sắp xếp, gán vai | **Lạc quan**: đổi UI ngay, lỗi thì trả lại + toast lỗi |
| Biểu mẫu | Theo dõi thay đổi; SaveBar; `⌘S`/`Ctrl+S` lưu; rời trang/đóng tab khi còn thay đổi → hỏi lại; sau khi lưu đặt dữ liệu server trả về làm mốc mới |
| Tải lần đầu | Khung xương (skeleton) đúng bố cục panel, không spinner toàn trang; > 8 s hiện “Đang chậm hơn bình thường…” |
| Tải lại ngầm | Vạch 2 px chạy ở mép trên panel; tooltip “Cập nhật lúc 14:02” ở tiêu đề panel |
| Mất kết nối | `ConnectionBanner`; nút ghi bị khoá kèm tooltip; tự nối lại và làm mới khi có mạng |
| Ghi đè lẫn nhau | PUT có `If-Match: <version>`; server trả `409 {error:"Đã có thay đổi từ nơi khác", current}` → thanh “Đã có thay đổi từ nơi khác · Xem bản mới · Ghi đè” |
| Truyện bị khoá | `LockedDialog` (409 khoá) như hiện có |
| Thao tác chậm (> 2 s) không phải việc nền | Dòng phụ trong nút/panel nói đang làm gì (“Đang kiểm tra kết nối…”) |
| Phím tắt | Mọi hành động chính có tooltip ghi phím tắt |
| Hỗ trợ tiếp cận | Kết quả thao tác đọc qua `aria-live="polite"`, lỗi `assertive` |

## 4. Backend bổ sung

- `GET /api/events` (gateway, theo quyền người xem; sự kiện của truyện chỉ gửi cho người được xem truyện đó). Gộp luôn `job.*` (không có luồng việc riêng).
- Bộ quét thay đổi tệp như §2.
- **Phiên bản tài nguyên**: `version` (chuỗi, đổi mỗi lần ghi; dùng mtime+size hoặc hash) trong phản hồi GET và PUT của: cấu hình (mỗi scope), quy tắc, `project.json`, `story.json`, thư viện, tài khoản, bản đọc/phân vai/từ điển audio. PUT nhận `If-Match` (hoặc trường `version`) → 409 khi lệch.
- Mọi API ghi trả về tài nguyên sau khi ghi (hoặc `{ok, version}` khi tài nguyên lớn).
