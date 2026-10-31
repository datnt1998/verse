# 07 · Chuỗi giao diện (tiếng Việt)

## Giọng văn

Viết cho người viết truyện, không phải lập trình viên. Nói điều xảy ra và việc cần làm tiếp; chi tiết kỹ thuật (lỗi gốc, lệnh, tệp) đặt trong mục **Chi tiết kỹ thuật** thu gọn.

| Tránh | Dùng |
|---|---|
| localhost, 127.0.0.1, IP, cổng `:8686`, `:1455` | “máy tính này”, “Wi-Fi nhà”, “khi ra ngoài (Tailscale)” |
| `serve`, TUI, terminal | “Noverse”, “cửa sổ dòng lệnh” (chỉ khi bắt buộc) |
| máy chủ | “máy tính cài Noverse” / “thư viện” |
| token, credential, `auth.json`, “tự làm mới” | “giữ kết nối”, “tự động gia hạn — không phải đăng nhập lại” |
| OAuth, callback, state, `access_denied` | “đăng nhập ChatGPT”, “liên kết đăng nhập đã cũ”, “bạn đã huỷ hoặc chưa bấm Cho phép” |
| `config.json`, `web-accounts.json`, JSON | “thiết lập”, “Thiết lập sẽ lưu” |
| HTTP không mã hoá | “khi ở ngoài nhà, dùng Tailscale thay vì Wi-Fi công cộng” |
| `~/Truyen`, `dit-dang-audio/01-…` | tên hiển thị: “Truyen”, “Đít Đang Audio”, “Trà Xanh Cướp Kịch Bản…” (đường dẫn chỉ ở dòng phụ khi cần tìm trong Finder) |
| `/model`, `/next`… như TUI | “trong Cài đặt”, “Gõ / để duyệt chương, xuất bản, viết lại…” |

Giữ nguyên: tên model (`gpt-5.6-terra`), tên nhà cung cấp, “agent” và tên 4 agent, “API Key” trong ô nhập của nhà cung cấp dùng khoá.

Đặt trong `web/src/i18n/vi.ts`. Chuỗi lấy từ TUI giữ nguyên câu chữ.

## Ô nhập (placeholder theo trạng thái)

| Trạng thái | Chuỗi |
|---|---|
| Đang chạy | Nhập can thiệp cốt truyện, ví dụ: đẩy tuyến tình cảm lên chương 4 |
| Đang khởi tạo | Đang khởi tạo sáng tác... |
| Đang tạm dừng | Đang tạm dừng sáng tác... |
| Đã tạm dừng | Sáng tác đã tạm dừng, nhập nội dung bất kỳ để tiếp tục |
| Bị gián đoạn | Sáng tác bị gián đoạn, nhập nội dung bất kỳ để tiếp tục |
| Chờ nghiệm thu | Chờ nghiệm thu: Nhập ý kiến chỉnh sửa, hoặc /next để duyệt chương tiếp |
| Hoàn thành | Truyện đã hoàn thành · Nhập yêu cầu sửa đổi (vd: “viết lại chương 3”), /reopen hoặc /export |
| Khởi tạo thất bại | Khởi tạo thất bại, vui lòng kiểm tra cấu hình hoặc dùng /model để đổi model |
| Truyện mới (nhanh) | Nhập yêu cầu sáng tác truyện, Enter để bắt đầu viết ngay |
| Đồng sáng tác | Tiếp tục bổ sung yêu cầu, Enter để gửi cho AI |
| Dự án (ý tưởng) | Ghi nhanh một ý tưởng truyện… sẽ lưu vào y-tuong/ |
| Trình đọc | Nhập yêu cầu sửa cho chương N… |

## Nhãn trạng thái

Đang chạy · Đã tạm dừng · Đang tạm dừng · Đang rảnh · Hoàn thành · Thẩm duyệt · Viết lại · Gọt giũa · Can thiệp · Sẵn sàng · Lỗi.
Agent: Đang chạy · Tạm dừng · Chờ lệnh · Lỗi · Gọi khi cần (ARBITER).
Chương: Đã viết lại · Đang viết lại · Chờ viết lại · Đang viết · Hoàn thành.

## Tạo truyện

- H1: Bạn muốn viết câu chuyện nào?
- Phụ đề: Architect quy hoạch · Writer sáng tác · Editor thẩm duyệt — bạn can thiệp bất cứ lúc nào.
- Chế độ: Bắt đầu nhanh (Nhập một câu ý tưởng để bắt đầu viết ngay) · Đồng sáng tác (Trò chuyện làm rõ yêu cầu cùng AI trước khi viết)
- Ví dụ (từ `panels.go`):
  1. Viết truyện đô thị trinh thám hồi hộp 12 chương, nhân vật chính là một nữ giám định pháp y
  2. Sáng tác truyện tiên hiệp dài tập, nhân vật chính từ người phàm tu luyện đến phi thăng
  3. Viết một truyện ngắn khoa học viễn tưởng về tình huống đạo đức khó xử khi AI thức tỉnh
- Đồng sáng tác: “AI đang tổng hợp yêu cầu của bạn...” · “Gợi ý từ AI — bấm hoặc nhấn 1/2/3 để chọn, có thể sửa trước khi gửi” · “Tiếp tục bổ sung, hoặc nhấn Ctrl+S để bắt đầu sáng tác”
- Lối vào khác: Đã có thiết lập / đại cương `/start <đường-dẫn>` · Đã có bản thảo truyện `/import <đường-dẫn>` · Mô phỏng văn phong `/simulate`

## Lệnh (palette) — mô tả từ `commands.go`

| Lệnh | Mô tả |
|---|---|
| `/help` | Xem danh sách và hướng dẫn các lệnh |
| `/model [vai-trò]` | Chuyển đổi Model và mức độ suy luận cho từng vai trò |
| `/config` | Thêm hoặc chỉnh sửa Provider, Model và Cửa sổ ngữ cảnh |
| `/diag` | Bảng chẩn đoán sức khỏe và chất lượng truyện |
| `/review on\|off` | Bật/tắt chế độ nghiệm thu từng chương |
| `/next` | Nghiệm thu và cho phép viết tiếp một chương mới |
| `/start <tệp>` | Tạo truyện mới từ tệp thiết lập hoặc đại cương có sẵn |
| `/import …` | Nhập tiểu thuyết từ bên ngoài theo ngữ nghĩa (không tham số để tiếp tục) |
| `/reopen [hướng]` | Mở lại cuốn sách đã hoàn thành để tiếp tục sáng tác |
| `/cocreate` | Tạm dừng viết để đồng sáng tác quy hoạch giai đoạn tiếp theo |
| `/simulate` | Đọc thư mục ./simulate để tạo hoặc cập nhật hồ sơ mô phỏng văn phong |
| `/importsim <tệp>` | Nhập hồ sơ mô phỏng văn phong có sẵn và gộp theo vân tay ngữ liệu |
| `/sync [--check]` | Kiểm tra hoặc tiếp nhận các chương đã sửa đổi thủ công từ bên ngoài |
| `/export […]` | Xuất các chương đã hoàn thành ra tệp TXT hoặc EPUB |

Lỗi lệnh: “Lệnh không xác định: /x” · “Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /x”.

## Phím tắt

`/` lệnh · `Enter` gửi · `Ctrl J`/`Alt Enter` xuống dòng · `Esc` tạm dừng (khi chạy) / đóng · `Tab` đổi panel (Workspace) / đổi chế độ (Truyện mới) · `↑ ↓` lịch sử nhập / cuộn · `End` về cuối · `Ctrl L` xóa màn hình · `[` `]` ẩn/hiện panel · `⌘/Ctrl K` bảng lệnh · `Ctrl S` bắt đầu (Đồng sáng tác).

## Kết nối & lỗi

- Mất kết nối máy chủ · tự kết nối lại sau Ns · [Thử ngay]
- Phiên đăng nhập đã hết hạn — Phiên của {tên} trên thiết bị này đã đủ 30 ngày. Đăng nhập lại để tiếp tục. (toàn bộ chuỗi đăng nhập/QR/onboard: `09-auth-onboard.md` §5)
- Truyện đang được mở ở nơi khác — Một tiến trình khác (TUI hoặc phiên web khác) đang giữ truyện này. [Chỉ đọc] [Thử lại]
- Không kết nối được máy chủ. Vui lòng thử lại. (giữ nguyên từ `client.ts`)
- Trống: Không có agent nào đang viết · Không có sự kiện nào thuộc loại này · Không có lệnh khớp “q”

## Bảng từ: trợ lý AI & công cụ (bổ sung 25/10)

Áp dụng ở mọi màn có AI làm việc: Bàn viết, Nhật ký tiến độ, Việc chạy nền, kết quả lệnh, bảng lệnh, điện thoại, máy tính bảng, Thiết lập lần đầu. **Không còn chữ tiếng Anh, không IN HOA tên trợ lý.**

### Tên trợ lý (`agent` / `role` → nhãn)
| Engine | Nhãn | Ghi chú |
|---|---|---|
| `architect`, `architect_long` | **Kiến trúc sư** | lên dàn ý, kế hoạch chương |
| `writer` | **Người viết** | viết chương |
| `editor` | **Biên tập viên** | thẩm duyệt |
| `arbiter` | **Trọng tài** | phân xử can thiệp, phán quyết |
| `system` | **Hệ thống** | |
| `agent` (chữ chung) | **trợ lý** | tab “Agent” → “Trợ lý” |

### Tên công cụ (`tool` → nhãn)
| Engine | Nhãn | Icon |
|---|---|---|
| `novel_context`, `read_context` | Đọc bối cảnh | note-text |
| `read_chapter` | Đọc chương | book-1 |
| `draft_chapter` | Viết bản nháp | edit-2 |
| `edit_chapter` | Gọt giũa | magicpen |
| `check_consistency` | Soát nhất quán | shield |
| `commit_chapter`, `save_chapter` | Lưu chương | folder-open |
| `plan_chapter` | Lập kế hoạch chương | routing-2 |
| `save_book` · `save_foundation[premise]` · `save_foundation[outline]` | Lưu thông tin truyện · Lưu tiền đề · Lưu dàn ý | folder-open |
| `update_outline` | Cập nhật dàn ý | routing-2 |
| `score_chapter` | Chấm điểm chương | chart-2 |
| `save_arc_summary` · `audit_foundation` | Tóm tắt hồi · Soát nền truyện | note-text · shield |
| công cụ chưa có trong bảng | tên engine, dạng `snake_case` → chữ thường có cách | code |

Tham số hiển thị thành dòng “nhãn · giá trị”: `chapter` → Chương, `source: draft` → Bản: nháp, `target_words` → Độ dài mục tiêu (… chữ), `from/to` → Chương N–M, `arc` → Hồi, `foreshadow` → Chi tiết cài cắm. Tham số thô chỉ có trong nút **Chép chi tiết kỹ thuật**.

### Trạng thái, phán quyết, cảnh báo
| Engine | Nhãn |
|---|---|
| `retry · reroute · abort` | thử lại · giao trợ lý khác · dừng hẳn |
| `StopGuard` | Bộ chống lặp |
| `context N% (a/b) · strategy: full summary` | {Trợ lý} đã dùng N% bộ nhớ ngữ cảnh (a/b) · sẽ tóm tắt phần cũ để viết tiếp |
| `model switched: writer → p/m` | Đã đổi model cho Người viết → {nhà cung cấp · model} |
| `429 Too Many Requests` | {Nhà cung cấp} báo vượt hạn mức (mã 429) |
| `ENOSPC` | Ổ đĩa máy chủ đã đầy |
| `plant · advance · resolve` (phục bút) | Đã gieo · Đẩy tiếp · Thu hồi |
| `cache read/write/break` | Đọc bộ nhớ đệm · Ghi bộ nhớ đệm · Lần mất bộ nhớ đệm |

Nhật ký dùng chung `lib/labels.ts` để dựng câu lỗi: công cụ “không thành công”, kèm lý do đã dịch nếu nhận ra. Lỗi gốc không nhận ra chỉ ở chi tiết kỹ thuật; nội dung chép vẫn lấy lỗi từ tóm tắt nếu sự kiện cũ không có kết quả riêng.
Tên vai/người lập kế hoạch và lời gọi công cụ nằm trong câu tự do cũng qua cùng bộ nhãn. Dòng giao việc bỏ tên vai lặp và ngoặc bao nhiệm vụ, giữ mẫu “{Trợ lý} · {nhiệm vụ}” trên máy tính và nhãn người gửi riêng trên điện thoại.

### Số, đơn vị, viết tắt
- Thời lượng: `137ms` → **0,1 giây**, `29ms` → 0,03 giây, `15.6s` → 15,6 giây, `1m39s` → 1 phút 39 giây.
- Token: `3.5M` → **3,5 triệu token**, `488.7k` → 488,7 nghìn token. Thanh trạng thái gọn: `4,0 tr vào · 87,0K ra · đệm 75% · $2,64`.
- Dấu thập phân phẩy, nghìn chấm: 58.699 chữ · $2,05 · 77,8 KB.
- `Ch. 4`, `Ch.25` → **Chương 4**. `arc` → **hồi**.
- Điều hướng: `Workspace` → **Bàn viết**. `panel` → **bảng bên**.
- Nút không hiện cú pháp lệnh: “/next Duyệt chương 4” → **Duyệt chương 4**; “/model đổi model” → **Đổi model**. Cú pháp lệnh (`/export`, `--overwrite`, tên vai `writer` trong `/model writer`) chỉ xuất hiện ở bảng lệnh và màn Hướng dẫn lệnh.

### Kiểu chữ
- **Chữ tiếng Việt không bao giờ dùng font mono.** Font mono làm vỡ dấu (“kế´hoạch”, “đề`”). Mono chỉ dùng cho giờ (21:52:47), tên model (`gpt-6.1-sol`), cú pháp lệnh và chi tiết kỹ thuật.
- Dòng công cụ trong nhật ký: `├` + icon công cụ 15px (#7E7E8A) + nhãn sans 13px + thời lượng nghiêng bên phải.
- Dòng giao việc: tên trợ lý đậm theo màu trợ lý + “ · nhiệm vụ” (bỏ ngoặc toàn khổ `（）`).
- **Nhãn “Chương N” không đặt trong ô rộng cố định theo px.** Bản cũ là “Ch. 24” trong ô 46px; sau khi Việt hoá thì chữ tràn đè lên nội dung. Dùng `min-width` + `white-space: nowrap`, hoặc xếp chồng nhãn chương lên trên dòng nội dung khi cột hẹp (rail Kiểm tra chương, danh sách chương trên điện thoại).
- **Số đang chạy không được xô chữ xung quanh.** Đồng hồ, phần trăm, bộ đếm:
  - Đặt trong ô rộng cố định (`min-width` đủ cho giá trị dài nhất, vd `5ch` cho m:ss), `font-variant-numeric: tabular-nums`, căn phải.
  - Cập nhật tối đa 1 lần/giây.
  - Đồng hồ đếm lên dạng `0:49` (`role="timer"`, aria-label “Đã chạy 49 giây”), không dùng số lẻ “50,0 giây”.
  - Icon đầu các bước cùng một cỡ (15px), để khi đổi trạng thái chữ không bị đẩy.

**Triển khai (gói J):**
- Một module nhãn duy nhất `web/src/lib/labels.ts`: `agentLabel`, `toolLabel` (công cụ lạ → chữ thường dễ đọc), `toolIcon` (lạ → `code`), `errorLabel` (429, ENOSPC…), `decisionLabel`, `warningLabel`, `foreshadowActionLabel`, `chapterRangeLabel`.
- Số & đơn vị ở `web/src/lib/format.ts` (vi-VN): `formatDuration(ms)`, `formatTimer(ms)`, `formatTokens(n, 'long'|'short')`, `formatNumber`, `formatUSD`, `chapterLabel`. Bản en-US cũ trong `screens/workspace/format.ts` đã bỏ.
- Icon tra theo từng artboard: MLog dùng `shield-security` cho công cụ soát (`mlogToolIcon` ở `screens/activity/mobile/logic.ts`), còn Main/Activity dùng `shield-tick`.
- Đồng hồ luồng viết là `role="timer"`, ô `min-width:5ch` căn phải, tabular-nums, 1 nhịp/giây. Đo trên mock: lệch 0 px sau 3 và 6 giây.

## Bảng từ: brief 25/10 <!-- brief-2510 -->

| Khái niệm | Chữ hiện trên UI | Không dùng |
|---|---|---|
| Định dạng xuất | Văn bản .txt · Ebook .epub · **Bản đọc TTS** (mô tả “cho giọng đọc AI”) | TXT/EPUB viết hoa trần, narration |
| `doc-so` | Đọc số thành chữ | doc-so |
| `--keep` / `--overwrite` | Giữ cả hai (thêm giờ vào tên) / Thay bản cũ | ghi đè, overwrite |
| [LỖI] / [CẢNH BÁO] | **Bắt sửa** / **Cảnh báo** | Lỗi, Error, Warning |
| Mục luật | Master §7 · Bộ A §4.3 · Xưng hô · Quy tắc của bạn | rule id, mã luật |
| Kết luận thẩm duyệt | Cần gọt giũa · Đạt · còn N cảnh báo · Đã vào hàng chờ sửa | pass/fail |
| Kiểu móc cuối chương (Master §7) | Lật thông tin · Đe dọa mới · Quyết định treo · Câu thoại nổ | khủng hoảng, cliffhanger |
| Khung truyện | Người nghe · Điểm khác · Hạt giống phần hai | audience, USP |
| Bản đồ | Bản đồ giữ chân · Câu hỏi đang treo · Lời hứa · đếm ngược · Trả thưởng · cú lật | retention map |
| Gói thể loại | Gói Nữ tần báo thù · có chấm · Sắp có | pack, style key |
| Giọng đọc theo gói | Một giọng kể · theo gói | single voice |
| Não truyện | Não truyện · Thực thể · Quan hệ · Xưng hô (có chiều) · Ẩn, chưa lộ · MỚI | graph, node, edge |
| Khoá khi đang viết | “Người viết đang viết lại chương 4, nên xưng hô và quan hệ đang khoá…” · Tạm dừng để sửa | locked, 423 |

## Bảng từ: bổ sung 25/10 (gói M) <!-- goi-m -->

| Khái niệm | Chữ hiện trên UI | Không dùng |
|---|---|---|
| Mã cho lần chạy đầu | **Mã thiết lập** (8 ký tự) | setup token, OTP, mã kích hoạt |
| Người quản lý máy chủ | Quản trị viên · tài khoản quản trị | admin, root |
| Trạng thái mục thẩm duyệt | Còn mở · Đã sửa · Giữ nguyên | open/fixed/kept, bỏ qua |
| Màn chất lượng | Chất lượng truyện · Mục hay vi phạm nhất · Chương đạt | QA, báo cáo lỗi |
| Não cấp dự án | Não dự án · Sổ cái · 10 trục · Vũ trụ chung | ledger, universe |
| Não cấp hệ thống | Não hệ thống · Bài học · Chờ duyệt / Đã giữ / Đã bỏ · Mục luật hay vấp · Lời nhắc trước khi viết | lessons, insight, memory |
| Lượt nghe | Ngày 7 (lượt nghe ngày thứ 7) | D7, retention |
| Nhóm việc ở Chất lượng | Đang chặn chương · Cần bạn xem · Sẽ tự sửa khi viết lại | backlog, todo |
| Hành động với cảnh báo | Nhờ sửa ở chương này · Giữ nguyên… | fix, ignore, bỏ qua |
| Kiểm trùng khi tạo truyện | So với truyện trước · Truyện này giống một truyện bạn đã đăng · Nhờ Kiến trúc sư đổi trục · Được bắt đầu | duplicate, conflict, trùng lặp |
| Mức trong Não dự án / hệ thống | Bắt sửa · Cảnh báo · Đạt | [LỖI], [CẢNH BÁO] |

## Bảng từ: Lưu trữ tệp (Gói AN, 31/10) <!-- an-copy -->

| Chỗ | Dùng | Không dùng |
|---|---|---|
| Tên mục | Lưu trữ tệp | Blob storage, Object storage |
| Nhóm nhà cung cấp | Kho S3 · Ổ đĩa máy chủ | bucket provider, local driver |
| Tên ô | Tên kho · Mã khoá truy cập · Khoá bí mật · Địa chỉ máy chủ · Địa chỉ máy chủ nội bộ · Vùng · Account ID (R2) · Địa chỉ kho cho trình duyệt | bucket, access key id, secret, endpoint, presign endpoint, region |
| Cách gửi | Trực tiếp từ kho · Qua máy chủ Noverse | redirect, proxy, presigned URL |
| Công tắc | Kiểu đường dẫn · Cho phép http (chỉ mạng nội bộ) | force path style, allow insecure endpoint |
| Môi trường | Thử nghiệm · Chạy thử · Chính thức | dev, staging, prod |
| Nguồn cấu hình | Lưu trong app · khoá đã mã hoá · do máy chủ đặt | env, database |
| Bước kiểm tra | Ghi thử · Đọc lại · Đọc một đoạn · Tạo link ký · Trình duyệt tải được (CORS) · Máy chủ gửi được một đoạn cho trình duyệt · Xoá tệp thử | PUT, GET, Range, presign, cors_probe, app_range, DELETE |
| Kết quả bước | đạt (ms) · lỗi · bỏ qua · không cần · chưa chạy | OK, FAIL, skipped, N/A |
| Hoạt động | Ghi · Đọc · Tạo link · Xoá — Số lần · Lỗi · Dữ liệu · Trung bình · Chậm nhất | put, open, presign, delete, p50, max |
| Dung lượng | Âm thanh · Bản xuất · Bìa · Nhạc nền · Giọng mẫu · Bản thảo nhập | audio, export, cover, music, voiceSample, import |
| Chuyển kho | Đang chuyển sang … · Tạm dừng · Có lỗi · Đã chép và kiểm tra · Thử lại các tệp lỗi · Huỷ chuyển · Kho cũ | migration, copied, verified, pending store |

**Mã lỗi kiểm tra → lời:** `wrong_key` Kho từ chối khoá truy cập · `no_bucket` Không có kho tên “…” · `no_write_permission` Khoá chưa có quyền ghi tệp · `no_read_permission` Khoá chưa có quyền đọc tệp · `no_delete_permission` Khoá chưa có quyền xoá tệp · `unreachable_endpoint` Không kết nối được địa chỉ kho · `clock_skew` Đồng hồ máy chủ bị lệch · `browser_endpoint_required` Trình duyệt không tới được địa chỉ kho · (trình duyệt) Trình duyệt bị kho chặn (thiếu CORS) · `content_mismatch` Nội dung đọc lại không khớp · `storage_error` Kho chưa hoàn thành được bước kiểm tra · 429 Bạn đã kiểm tra nhiều lần; hãy chờ một phút rồi thử lại.

**Mã lỗi chuyển tệp → lời (bảng lỗi):** `content_mismatch` Chép xong nhưng đọc lại không khớp — thường do mạng chập chờn, thử lại là được · `unreachable_endpoint` Mất kết nối tới kho mới giữa chừng — thử lại khi mạng ổn định · `storage_error` Kho mới chưa nhận tệp này — kiểm tra quyền của khoá và kết nối, rồi thử lại · `wrong_key` Kho mới từ chối khoá — khoá vừa bị thu hồi hoặc đổi · `no_read_permission` Khoá của kho mới thiếu quyền · `clock_skew` Đồng hồ máy chủ lệch giờ · `no_bucket` Không còn thấy kho mới — kho bị xoá hoặc đổi tên.

**Câu cố định:** “Chưa đổi gì cho tới khi bạn kiểm tra kết nối, lưu và xác nhận chuyển tệp.” · “Kiểm tra kết nối đạt rồi mới lưu được” · “Chuyển {n} tệp · {dung lượng} sang {nhà cung cấp}?” · “App vẫn chạy bình thường trong lúc chuyển. Tệp mới vẫn ghi vào kho cũ cho tới khi chuyển xong, rồi được chép nốt.” · “Còn tệp lỗi thì chưa đổi kho — app vẫn dùng ổ máy chủ, không mất gì.” · “Kho cũ ({nơi}) vẫn giữ {dung lượng}” · “Kho tệp do máy chủ đặt sẵn. Ở đây chỉ xem và kiểm tra kết nối được.” · “Máy chủ chưa có khoá mã hoá nên chưa lưu được khoá bí mật” · “Đổi kho trên máy tính.”

## Bảng từ: Đăng nhập chung với Viderse (Gói AO, 31/10) <!-- goi-ao -->

| Chỗ | Dùng | Không dùng |
|---|---|---|
| Nhóm trong menu tài khoản, tab Thêm | Ứng dụng · Đang mở | Apps, Chuyển app, Workspace |
| Dòng app | Viderse · `vid.aarverse.studio` (mở ở thẻ mới) | “Sang Viderse”, link trần |
| Đăng nhập (dòng phụ) | Một tài khoản dùng cho Noverse và Viderse | SSO, đăng nhập một lần |
| Đăng xuất | Khỏi cả Noverse và Viderse trên trình duyệt này | Đăng xuất toàn hệ thống, revoke |
| Chuyển tài khoản | Đăng xuất cả hai app rồi nhập mật khẩu của … | — |
| Đổi mật khẩu | Đổi xong, mọi thiết bị phải đăng nhập lại ở cả Noverse và Viderse, kể cả máy này. | thu hồi phiên |
| Thiết bị | Mỗi thiết bị một phiên chung cho cả hai app. · Thiết bị bị đăng xuất phải đăng nhập lại ở cả Noverse và Viderse. | session |
| Thành viên › Quyền vào ứng dụng | … sẽ bị đăng xuất khỏi mọi thiết bị ở cả Noverse và Viderse và phải đăng nhập lại. · … vẫn vào Noverse được; mở Viderse sẽ thấy thông báo chưa được cấp quyền. | grant, revoke, app_access |
| Chưa được cấp quyền | Chưa được cấp quyền vào Noverse · Nhờ quản trị viên bật ô Noverse cho bạn ở Thành viên, rồi bấm Thử lại. · Chép lời nhờ quản trị viên cấp quyền | 403, access denied |

Câu “vì sao phải đăng nhập lại” ở `09-auth-onboard.md` §12.3.
