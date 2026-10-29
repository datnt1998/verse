# 14 · Việc chạy nền (dock góc dưới phải) & kết quả sau khi chọn lệnh

| Artboard | Màn |
|---|---|
| `Jobs.dc.html` | 09 · Dock việc chạy nền — thấy ở **mọi màn**. Bấm các tab trên thanh điều hướng của artboard để đổi màn nền: dock vẫn giữ nguyên. Bảng thử (viền đứt, góc trái trên vùng nội dung) dùng để xem animation, **không thuộc giao diện** |
| `CmdResults.dc.html` | 05c · Thứ hiện ra sau khi chọn lệnh: bảng `/model`, `/diag`; thẻ kết quả trong nhật ký cho `/export`, `/sync`, `/simulate` |

---

## 1. Dock việc chạy nền

### 1.1 Vị trí: góc dưới phải

- Neo `right: 16px; bottom: 16px`. Pill, bảng và toast đều canh mép phải và mọc lên/sang trái từ góc đó.
- Ở Workspace, dock **tự thu thành chấm** 52 px nằm dưới cột agent bên phải. Lý do: việc của các trợ lý đã hiện ngay trên màn, và dạng pill sẽ đè ô chat. Bấm chấm để mở bảng. Các việc của agent (viết, lên dàn ý, chấm, trọng tài) không lặp lại trong dock khi đang ở Workspace.
- Lớp: `z-index` trên nội dung, dưới hộp thoại và scrim.

### 1.2 Bốn trạng thái & chuyển đổi

```
ẩn ──(có việc mới)──▶ pill ◀──(Thu gọn / Esc)── bảng
 ▲                     │  ▲                       ▲
 │(xong hết + 4,2 s)   │  └──(bấm chấm)── chấm ───┘(bấm chấm khi ở Workspace)
 └─────────────────────┘        ▲
                       (nút – trên pill/bảng)
```

| Trạng thái | Kích thước | Nội dung |
|---|---|---|
| **ẩn** | — | Không có việc chạy, chờ hoặc lỗi |
| **chấm** | 52 × 52 | 1 vòng tiến độ của việc quan trọng nhất |
| **pill** | 352 × 52, bo 26 | Tối đa 3 vòng tiến độ chồng nhau (+N) · dòng chữ tự chạy · số việc · mũi tên mở · vạch tiến độ gộp 2 px ở đáy |
| **bảng** | 392 × tối đa ~560 | Đầu bảng (Việc chạy nền · đếm · Báo khi xong · –) · nhóm **Cần bạn / Đang chạy / Vừa xong** · chân bảng (Xem nhật ký đầy đủ · ⌥J · Thu gọn) |

Phím tắt `⌥J` (Win/Linux: `Alt+J`) bật/tắt bảng. `Esc` khi bảng mở → thu về pill.

### 1.3 Animation (giá trị dùng trong artboard)

| Khoảnh khắc | Hiệu ứng | Thời lượng · easing |
|---|---|---|
| Dock xuất hiện | từ `translate(10px,14px) scale(.82) blur(6px)` → bình thường, nảy nhẹ, gốc biến hình ở góc dưới phải | 550 ms · `cubic-bezier(.18,.95,.22,1.12)` |
| Dock biến mất (xong hết) | thu về góc + mờ + blur | 380 ms · `cubic-bezier(.5,0,.75,0)` |
| Pill → bảng | **morph bằng `clip-path`**: hé từ đúng khung pill (dưới phải 352×52, bo 26: `inset(calc(100% - 52px) 0 0 calc(100% - 352px) round 26px)`) mở lên thành bảng (bo 20). Chân bảng cao 52, cùng vị trí pill, nên lúc bắt đầu trông như chính pill đang mọc lên | 520 ms · `cubic-bezier(.2,.9,.22,1)` |
| Bảng → pill | morph ngược | 340 ms · `cubic-bezier(.55,0,.7,.2)` |
| Nội dung bảng | từng nhóm, từng dòng trồi lên 10 px, trễ dần 45 ms | 420 ms |
| Pill ⇄ chấm | `width` 352 ↔ 52; chữ mờ trước (140 ms), vòng tiến độ ở lại | 420 ms |
| Dòng chữ trên pill | trượt dọc 36 px mỗi ~4,2 s giữa các việc. Vòng của việc đang hiện sáng, vòng khác 50% | 500 ms |
| Một việc vừa xong | pill **nảy** (scale 1.045) + **vòng sáng mint** lan 14 px | 600 ms / 1 s |
| Toast vào | trồi 18 px + scale .94 + blur → rõ | 500 ms · nảy nhẹ |
| Toast ra | hai lớp: (1) thẻ **rơi vào pill** — trong 65% đầu mờ hẳn, `translateY(34px) scale(.5)`, blur 3 px, gốc ở góc phải; (2) sau 200 ms, khung bọc thu `max-height` và `padding-top` về 0 để toast phía trên **trượt xuống êm**, không giật. Xong thì pill nảy | 420 ms + 320 ms |
| Dòng việc bị huỷ | thu `max-height` → 0 | 300 ms |
| Dấu tick | vẽ nét (`stroke-dashoffset`) | 500 ms |
| Tiến độ không rõ % | vạch chạy qua lại; vòng quay 1,1 s | — |
| Đầu vòng tiến độ | chấm trắng 5 px có quầng màu ở đầu cung, xoay theo % | 300 ms linear |
| Orb agent đang chạy | thở (scale 1 ↔ 1.08) + quầng sáng nhịp theo | 2,6 s lặp |
| `prefers-reduced-motion` | tắt chuyển động, giữ đổi trạng thái | — |

### 1.4 Toast

- Chỉ hiện khi bảng **đang đóng**. Khi bảng mở, dòng việc vừa xong nháy nền mint thay cho toast.
- Hiện tối đa 2 toast. Toast mới nằm sát pill; toast cũ lùi lên với scale .955, mờ 60%, giảm bão hoà. Rê chuột vào chồng toast thì cả hai bung rõ lại.
- **Đếm ngược tự đóng = vòng mảnh quanh nút ×** (SVG, bán kính 12, nét 1,6, màu theo loại toast, `stroke-dashoffset` chạy mượt 160 ms linear), không dùng vạch ở đáy. Toast tự tắt sau 6 s. Rê chuột vào thì dừng đếm: vòng mờ còn 30% và tooltip đổi thành “Đã dừng tự đóng”.
- Toast của việc agent dùng **orb của agent đó** kèm huy hiệu tick mint, và nền loang theo màu agent.
- Ba loại:

| Loại | Màu | Ví dụ | Nút |
|---|---|---|---|
| Xong | mint | Đã xuất EPUB · 24 chương — *Trà Xanh….epub · 1,2 MB* | **Tải xuống** · Mở thư mục |
| Cần bạn | cam | Nhập truyện tra-xanh.txt — *Đã chia 42 chương, chờ bạn xác nhận* | **Xem & xác nhận** |
| Lỗi | đỏ hồng | Tạo giọng chương 7 dừng lại — *Bộ đọc trên máy không phản hồi…* | **Thử lại** · Chi tiết |

- Khi tab trình duyệt bị ẩn và đang bật **Báo khi xong**: gửi thông báo của trình duyệt (Notification API, xin quyền lần đầu bật).

**Bổ sung Gói AE (28/10)** <!-- jobs-ae -->
- Toast *Xong* của việc xuất có biến thể **tự tải**: đang tải (vạch tải mint dưới dòng phụ) → “Đã tải về máy này” · *Mở tệp* · *Xem bản xuất*; và **bị chặn** (cam): “chưa tải được” + **Tải xuống**.
- Trang có thanh hành động ở đáy cùng phía phải (ví dụ trình nghe Audio): dock nâng lên trên thanh (`bottom` = chiều cao thanh + 12 px), không che nút chính.
- Điện thoại: việc chạy nền hiện thành **thẻ nổi** trên trình phát thu nhỏ (cùng nội dung và nút như toast máy tính), không dùng pill.

### 1.5 Dòng việc trong bảng

- **Vòng tiến độ** 36 px (orb Writer cho việc của agent, icon cho việc khác) · tiêu đề · dòng phụ · vạch 4 px + `% · còn ~N phút`.
- Hiện khi rê chuột: Tạm dừng / Chạy tiếp · Mở màn liên quan · Huỷ (việc agent: “Tạm dừng sau bước này”).
- **Cần bạn**: dòng phụ màu kem, có nút hành động chính.
- **Lỗi**: dòng phụ đỏ, có *Thử lại* và *Bỏ qua chương này*.
- **Vừa xong**: tick mint vẽ nét, các nút kết quả. Mục xong được dọn khi dock tự ẩn.

**Nền agent:** mỗi việc gắn với đúng agent làm nó, dùng orb và bảng màu riêng.

| Agent | Orb | Gradient vòng & vạch | Quầng (rgba) |
|---|---|---|---|
| Kiến trúc sư (lên dàn ý) | `o-architect` | `#B9D0F7 → #7C93F3 → #5A57E3` | `124,147,243` |
| Người viết (viết chương) | `o-writer` | `#C9A6F5 → #9A63E3 → #C9457F` | `154,99,227` |
| Biên tập viên (chấm chương) | `o-editor` | `#F6CB8C → #EC9548 → #E25E36` | `236,149,72` |
| Trọng tài (xét góp ý) | `o-arbiter` | `#A6EDC1 → #5DC1B0 → #3E73D6` | `93,193,176` |

Chip của agent:
- nền `radial-gradient(rgba(quầng,.32) → trong suốt 70%)` trên `#141416`;
- orb có thêm `0 0 12px rgba(quầng,.55)`;
- quầng ngoài nhịp thở khi đang chạy;
- khi tạm dừng thì giảm bão hoà (`saturate(.25)`, mờ 75%).

Dòng phụ ghi tên agent, ví dụ “Kiến trúc sư · Bước 2/4 · Chọn tuyến chính”. Ở Workspace, thẻ agent bên phải dùng cùng orb, quầng và nền loang để đồng bộ.

Màu vòng và vạch cho các việc không phải agent:

| Loại việc | Màu |
|---|---|
| Audio | `#C9A6F5`, kèm cột sóng nhỏ |
| Xuất | `#A6EDC1` |
| Đồng bộ | `#7C93F3` |
| Văn phong | `#F6CB8C`, dạng không rõ % |
| Chờ bạn | `#F6CB8C`, kèm chấm ping trên pill |
| Lỗi | `#FF9FA8` |

### 1.6 Thứ tự ưu tiên

Cần bạn → Lỗi → Đang chạy → Tạm dừng → Xong.

Vòng đầu tiên trên pill và chấm luôn là việc ưu tiên cao nhất.

### 1.7 Dữ liệu (GAP-26)

- `GET /api/jobs`: trả `[{id, kind: write|audio|export|sync|simulate|import|diag, title, status: running|paused|waiting|failed|done, progress: 0..1|null, step?, unit?{done,total,label}, etaSec?, error?, result?{title, sub, actions[]}, href, startedAt, finishedAt?}]`.
- SSE `job.created | job.progress | job.waiting | job.failed | job.done`, đi chung luồng sự kiện hiện có (`web_live.go`).
- `POST /api/jobs/:id/{pause|resume|cancel|retry}`.
- Nguồn việc:
  - engine viết (`IsRunning`, bước hiện tại);
  - audio (`10-audio.md` GAP-19);
  - `exp.Export`;
  - `revision.Sync`;
  - simulate;
  - import (giai đoạn “chờ xác nhận chia chương” = `waiting`);
  - review gate (`waiting`);
  - `/diag` (chạy ngầm khi đóng bảng).
- Dock gắn một lần ở app shell (ngoài router), nên **không mount lại khi đổi màn**. Trạng thái pill/bảng/chấm và “Báo khi xong” lưu `localStorage` theo người dùng.

**Mock / kiểm tra dock:** `POST /api/jobs/demo` nhận `{scenario:"running"|"expanded"|"finishing"|"attention"|"error"|"mini"|"empty"}` hoặc `{action:"add"|"finish"|"attention"|"error"|"finishAll"|"reset"}`. Dữ liệu mô phỏng phát `job.*` trên `/api/events` như máy chủ thật; không có `/api/jobs/events`. Mở bảng bằng `⌥J`/`Alt+J`, thu thành chấm bằng nút `–`; thay màn không khởi tạo lại việc. Store làm mới `GET /api/jobs` khi nối lại luồng hoặc quay về tab, và dùng chung `useJobs` cho dock desktop / sheet mobile.

---

## 2. Sau khi chọn lệnh (05c)

| Lệnh | Hiện gì | Artboard / trạng thái |
|---|---|---|
| `/help` | Trang Hướng dẫn lệnh | `Commands.dc.html` |
| `/config` | Cài đặt › Nhà cung cấp AI | `Settings.dc.html` |
| `/model [vai]` | **Bảng Model cho từng trợ lý**: trái là 4 vai (Mặc định · Kiến trúc sư · Người viết · Biên tập viên) kèm model và mức đang dùng; phải là Nhà cung cấp → Model (ngữ cảnh, ghi chú) → Mức suy luận (chỉ các mức model hỗ trợ: Mặc định/Tắt/Thấp/Vừa/Cao/Rất cao/Tối đa). Vai có thay đổi chưa lưu có chấm tím; Lưu báo “Đã lưu: Người viết → …” | `CmdResults` `view=model` |
| `/diag` | **Bảng Sức khoẻ truyện**. Lúc đầu chạy ngầm: 4 orb quay, dòng “Đang phân tích…”, khung chờ, ghi chú “có thể đóng, vẫn chạy ngầm” → dock. Báo cáo gồm: 8 ô tổng quan theo `diag.Stats`, lọc Nghiêm trọng/Cảnh báo/Gợi ý, thẻ phát hiện mở ra xem *Bằng chứng / Nên làm* kèm nhóm (Quy trình/Chất lượng/Đại cương/Bối cảnh) và độ tin cậy, mục Hành động đề xuất có nút “Gửi cho trợ lý”, nút *Xuất chẩn đoán ẩn danh* và *Phân tích lại* | `diagLoading` → `diag` |
| `/export …` | Thẻ trong nhật ký: **đang xuất** (lưới 24 ô chương sáng dần, %, chương đang ghép) → **xong** (bìa EPUB có tick, tên tệp, số chương và dung lượng, ghi chú bỏ qua chương chưa xong, nút Tải xuống / Mở thư mục / Chép đường dẫn / Xuất với tuỳ chọn khác) | `exportRun` → `exportDone` |
| `/export` gặp tệp trùng | Thẻ cam “Đã có tệp trùng tên — chưa xuất gì”, so sánh bản đang có và bản mới. **Thay bản cũ** (= `--overwrite`, `OnExists=replace`) · **Giữ cả hai** (`OnExists=keep`, thêm giờ vào tên) · Huỷ | `exportExists` |
| `/sync --check` | Thẻ liệt kê chương bị sửa ngoài app (+chữ/−chữ, giờ sửa, Xem khác biệt) và nút **Tiếp nhận cả 3 chương** | `syncCheck` |
| `/sync` | Thẻ theo từng chương, nhãn *Sự thật cốt truyện đã cập nhật* / *Chỉ đổi lời văn* / *N mâu thuẫn phía sau*. Mâu thuẫn có *Mở chương 14* / *Nhờ sửa chương 14* | `syncDone` |
| `/simulate` | Thẻ 3 bước có spinner và vạch không rõ % → thẻ hồ sơ: 4 đặc điểm, câu tiêu biểu, “Đang dùng”, Lưu .json | `simulateRun` → `simulateDone` |
| `/import` | Hộp thoại nhập truyện có sẵn | `Dialogs.dc.html` `state=import` |
| `/reopen` | Hộp thoại có sẵn | `Dialogs.dc.html` `state=reopen` |
| `/review on/off`, `/next` | Chip chế độ ở Workspace + trạng thái chờ duyệt | `Main.dc.html` `phase=gate`; dock: “Chương 24 chờ bạn duyệt” |
| `/cocreate` | Chế độ Đồng sáng tác của ô chat | `NewStory` `mode=cocreate` / Workspace |
| `/start` | Màn Truyện mới | `NewStory.dc.html` |

Mọi thẻ kết quả đều có chip lệnh vừa gửi ở phía trên (`/export exports/tra-xanh.epub`), trồi lên 14 px khi xuất hiện. Việc dài cũng hiện trong dock, nên người dùng đổi màn mà không mất theo dõi.

Các thẻ kết quả dùng cùng bộ điều khiển ở Workspace và Nhật ký; đổi giữa hai màn không xoá thẻ hay gửi lại lệnh. Tiến trình và kết quả lấy từ SSE `job` (`export`, `sync`, `simulate`), không chạy đồng hồ giả trong giao diện. Bảng model dùng cache cấu hình của truyện; báo cáo chẩn đoán giữ bản cũ khi tải lại, có nút thử lại khi lỗi, và chỉ chạy phân tích mới khi chọn `/diag` hoặc **Phân tích lại**.

Máy chủ mock phục vụ tệp xuất theo đúng tên tệp và MIME được yêu cầu: TXT UTF-8 hoặc EPUB có gói ZIP, mục lục và các chương XHTML thật; tải EPUB không đổi tên một tệp TXT.

**Lưu ý:** các ví dụ trong báo cáo `/diag` và hồ sơ văn phong là **dữ liệu mẫu**. Tiêu đề phát hiện thật đến từ các rule `internal/diag/rules_*.go` (hiện còn tiếng Trung → GAP-24).

## 3. Khoảng trống mới

| # | Việc | Ghi chú |
|---|---|---|
| GAP-26 | API việc chạy nền | `GET /api/jobs`, SSE `job.*`, pause/resume/cancel/retry — §1.7 |
| GAP-27 | `/export` trên web chọn cách xử lý tệp trùng | đưa `OnExists` = `replace` / `keep` ra API (exporter đã hỗ trợ) |
| GAP-28 | `/diag` trả JSON | `Host.Diagnose()` → `GET /api/diag` (Stats, Findings, Actions) + `POST /api/diag/actions/:fingerprint` |
| GAP-29 | `/sync` trả chi tiết | `revision.Result` (Applied, Analyses: ChangeSummary, StoryChanged, DownstreamIssues) + số chữ thêm/bớt cho `--check` |
