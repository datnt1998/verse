# 13 · Lệnh: gợi ý khi gõ `/` & trang hướng dẫn

Hai màn hình giúp người dùng **biết lệnh nào có, gõ thế nào, và lệnh sẽ làm gì** trước khi nhấn Enter.

| Artboard | Màn | Vai trò |
|---|---|---|
| `Slash.dc.html` | 05b · Gõ `/` trong ô chat | Popover ngay trên ô chat Workspace: danh sách lệnh + mô tả, cú pháp, gợi ý cách dùng, gợi ý tham số, kiểm tra lỗi trực tiếp |
| `Commands.dc.html` | 08 · Hướng dẫn lệnh | Trang tra cứu đầy đủ: mọi lệnh, giải thích từng tham số, ví dụ, kết quả, lỗi hay gặp, bộ ghép lệnh thử |

Nguồn sự thật: `internal/commands` (`Spec`/`Param`, `Parse`, `SplitArgs` — TUI, web và host dùng chung), `import.go#parseImportArgs`, `revision.go#startRevisionSync`, `host/exp/exporter.go`, `host.go#Reopen`. Dữ liệu dùng chung của 2 artboard nằm trong `renderVals` (khối `C = {…}`) — **copy nội dung, không copy code**.

---

## 1. Popover `/` (05b)

### 1.1 Bốn chế độ (theo nội dung ô chat)

| Ô chat | Chế độ | Danh sách trái | Bảng phải |
|---|---|---|---|
| `/` | **list** | “Dùng gần đây” (3) + 5 nhóm theo việc | Mô tả · nhãn · cú pháp · 2–3 “Gợi ý cách dùng” (bấm = điền) · mẹo |
| `/ex` | **list (lọc)** | “N lệnh khớp”, phần khớp tô tím, ưu tiên khớp đầu tên → giữa tên → từ khoá tiếng Việt không dấu (`xuat`, `epub`, `doi model`…) | như trên; **ghost text** phần còn lại của tên (`/ex`port) |
| `/export ␠` | **args** | “Có thể thêm”: giá trị gợi ý (đường dẫn gần đây), `from=`, `to=`, `--overwrite`… chỉ những tham số chưa dùng | Cú pháp có **ô đang nhập tô sáng** (`.slot`), tham số đã điền mờ đi; khung “Đang nhập: …” (mô tả + mặc định); “Khi nhấn Enter” = kết quả kiểm tra |
| `/export … from=` | **args (giá trị)** | “Giá trị cho from”: `1 · Từ chương đầu`, `13 · Tiếp sau lần xuất trước` | như trên |
| `/exprt` (không khớp lệnh nào) | **unknown** | “Có phải bạn muốn”: 2 lệnh gần nhất (Levenshtein) | Giải thích; Enter = sửa thành lệnh gợi ý. Composer viền đỏ + chip “Chưa chạy được” |

Văn bản không bắt đầu bằng `/` → không có popover, ô chat trở về font thường.

### 1.2 Bàn phím (đã chạy thật trong artboard)

| Phím | list | args |
|---|---|---|
| ↑ ↓ | đổi dòng chọn (vòng) | đổi gợi ý |
| Tab | điền `/<lệnh> ` | điền gợi ý (thêm dấu cách, trừ khi kết thúc bằng `=`) |
| Enter | lệnh `AutoExecute` & không tham số → chạy ngay; còn lại = như Tab | đang gõ dở gợi ý → điền; hợp lệ → gửi; lỗi → không gửi |
| Esc | đóng popover, xoá `/…` | như list |

Chân popover luôn ghi nhãn Enter theo ngữ cảnh: `chạy` · `điền & gõ tiếp` · `chạy lệnh` · `chưa chạy được` · `sửa thành /export`.

### 1.3 Kiểm tra trực tiếp (client-side, phản chiếu lỗi thật của backend)

| Điều kiện | Thông báo trong popover | Lỗi thật tương ứng |
|---|---|---|
| Thiếu tham số bắt buộc | `Còn thiếu chế độ (on\|off).` | `Cách dùng: /review on\|off` |
| `from > to` | `khoảng chương không hợp lệ: from=30 > to=12` | exporter.go |
| Đuôi ≠ `.txt/.epub` | `Không xuất được đuôi .pdf…` + nút sửa nhanh **Đổi thành .epub** | `không suy ra được định dạng từ đuôi…` |
| `from` > số chương đã xong | `Khoảng 25..30 không có chương hoàn thành…` | exporter.go |
| Đường dẫn có khoảng trắng (trừ `/start`) | “sẽ bị tách làm nhiều phần” | `strings.Fields` tách args → GAP-22 |
| `NeedsIdle` + đang chạy | nhãn “Cần rảnh” cam + khối cảnh báo orb Writer; Enter bị chặn | `Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /x` |
| Hợp lệ | `Đúng cú pháp — nhấn Enter để chạy.` + dòng “sẽ làm gì” | — |

Dòng “sẽ làm gì” cho `/export` (mẫu): *Sẽ tạo ebook EPUB trong exports/tra-xanh.epub · Gồm chương 1–24 · bỏ qua 25–30 vì chưa viết xong · Nếu tệp đã có, lệnh sẽ dừng để giữ bản cũ.* Cần `progress.completedChapters` + tên truyện (đã có trong snapshot).

**Backend vẫn là nơi quyết định.** Kiểm tra phía client chỉ để gợi ý; lỗi trả về từ server hiển thị nguyên văn trong nhật ký như hiện nay.

### 1.4 Dữ liệu cần thêm cho popover

- `GET /api/commands` (GAP-23): trả `[{name, aliases, usage, group, autoExecute, needsIdle, only?, params[]}]` sinh từ registry, để web không tự chép danh sách. `params[]` = `{key, token, kind: path|enum|flag|num|text, required, prefix?, flag?, options?[{value,note}]}` — thêm trường `Params` vào `slashCommandSpec` (mô tả tham số hiện chỉ nằm trong `Usage` + code parse).
- Gợi ý giá trị: đường dẫn xuất/nhập gần đây (lưu ở client, theo truyện), `NUMS` lấy từ tiến độ (`1`, `lastExportTo+1`, `completed`, “hết tập”).
- “Dùng gần đây”: 3 lệnh cuối, lưu ở client.
- Nội dung tiếng Việt (tiêu đề, mô tả, mẹo, ví dụ) đặt ở frontend `i18n/commands.vi.ts` — dùng nguyên văn bản trong artboard.

---

## 2. Trang Hướng dẫn lệnh (08)

Route `#/help/commands/:name?` (mặc định `intro`). Lối vào: lệnh `/help`, phím `?` mọi màn, nút **Hướng dẫn** ở header, link “Hướng dẫn đầy đủ ?” trong popover. Phím `?` chỉ bắt khi con trỏ **không** nằm trong ô nhập (gõ `?` trong ô chat vẫn là chữ); trong popover dùng link.

Bố cục 3 cột: **danh sách** 300px · **nội dung** linh hoạt (max 820) · **thử lệnh** 360px.

- **Trái**: tìm kiếm (không dấu, theo tên/tên khác/tiêu đề/từ khoá), mục “Cách dùng lệnh”, 5 nhóm theo việc: *Bắt đầu truyện · Trong lúc viết · Văn phong · Khi truyện đã xong · Cài đặt & trợ giúp*. Nhãn “Cần rảnh” chuyển cam khi đang viết. Tìm không thấy → gợi ý từ khoá.
- **Giữa — Cách dùng lệnh (intro)**: minh hoạ ô chat + 4 bước; “Đọc cú pháp trong 30 giây” (`/lệnh` · `<bắt buộc>` · `[có thể bỏ]` · `a|b` · `--công-tắc` · `tên=giá trị`); 2 loại lệnh (chạy ngay / cần gõ thêm); 4 lý do bị từ chối + cách sửa.
- **Giữa — chi tiết lệnh**: tên + tiêu đề dễ hiểu; nhãn (chạy ngay / cần gõ thêm / cần rảnh / chỉ ở màn chào mừng / chỉ khi đang viết / tên khác); lời dẫn; *Dùng khi* / *Chưa nên dùng khi*; **Cú pháp** + bảng tham số (bắt buộc/có thể bỏ, ý nghĩa, giá trị, mặc định khi bỏ trống); **Ví dụ** (Chép · Thử bên phải); **Sau khi chạy** (các bước + nhật ký mẫu); **Lỗi hay gặp & cách sửa**; Mẹo; Lệnh liên quan.
- **Phải**: *Ghép lệnh thử* — điều khiển theo loại tham số (ô chữ/số, nhóm chọn, công tắc), dòng lệnh tô màu, trạng thái kiểm tra giống popover, nút **Chép** và **Điền vào ô chat Workspace** (`/start` → màn Truyện mới; đang bận + cần rảnh → “Đợi hệ thống rảnh”). Bên dưới: *Phím tắt* (ô chat · web · bản dòng lệnh). Ở intro: “Bạn muốn…” — 8 việc thường gặp → lệnh.

Tweak: `cmd` (intro + 14 lệnh), `busy`, `q`.

---

## 3. Màu token lệnh (dùng chung 2 màn, CLI nên theo)

| Phần | Màu |
|---|---|
| Tên lệnh `/export` | `#C9A6F5` |
| Giá trị / tham số bắt buộc / lựa chọn | `#A6EDC1` |
| Công tắc `--x`, tiền tố `tên=` | `#F6CB8C` |
| Tham số có thể bỏ (trong cú pháp) | `#A0A0AB` |
| Đã điền (trong cú pháp ở popover) | `#5E5E68` |
| Ô đang nhập | nền `rgba(201,166,245,.16)` + gạch chân `#C9A6F5` |

Gradient chỉ xuất hiện ở orb Writer (đang viết) và viền chạy quanh ô chat khi agent hoạt động — đúng quy tắc chung.

## 4. Nội dung từng lệnh (tóm tắt — văn bản đầy đủ trong artboard)

| Lệnh | Tiêu đề hiển thị | Nhãn | Tham số |
|---|---|---|---|
| `/help` | Mở hướng dẫn lệnh | chạy ngay | — |
| `/model [vai-trò]` | Đổi model cho từng trợ lý | chạy ngay | `architect` · `writer` · `editor` |
| `/config` | Thêm hoặc sửa nhà cung cấp AI | chạy ngay | — (có thêm gì → `Cách dùng: /config`) |
| `/diag` | Khám sức khoẻ truyện | chạy ngay · dùng được khi đang viết | — |
| `/review on\|off` | Bật/tắt duyệt từng chương | cần gõ thêm | `on` · `off` (bắt buộc) |
| `/next` | Cho viết chương tiếp theo | chạy ngay · cần rảnh | — |
| `/start <tệp>` | Tạo truyện mới từ tệp có sẵn | chỉ ở màn chào mừng | đường dẫn (được đặt trong ngoặc kép) |
| `/import …` | Nhập truyện có sẵn để viết tiếp | cần rảnh | `<đường-dẫn>` (trống = tiếp tục lần dở) · `--yes` · `--story=open\|closed` · `--continue` · `--guide=…` (luôn cuối) |
| `/reopen [hướng]` | Mở lại truyện đã xong để viết tiếp | cần rảnh | lời thường; cần `/sync` trước nếu có chương sửa tay |
| `/cocreate` (`/plan`) | Cùng bàn hướng đi giai đoạn tới | chạy ngay · chỉ khi đang viết | — |
| `/simulate` | Học văn phong từ văn mẫu | cần rảnh | — (đọc thư mục `simulate`) |
| `/importsim <tệp.json>` | Nhập hồ sơ văn phong có sẵn | cần rảnh | tệp hồ sơ |
| `/sync [--check]` | Nhận chỉnh sửa bạn tự làm | chạy ngay · cần rảnh | `--check` = chỉ liệt kê |
| `/export …` | Xuất truyện ra TXT hoặc EPUB | chạy ngay | `[đường-dẫn]` (đuôi quyết định định dạng; kết thúc `/` = thư mục) · `from=N` · `to=M` · `--overwrite` |

## 5. Khoảng trống (thêm vào `06-data-contract.md`)

| # | Việc | Ghi chú |
|---|---|---|
| GAP-22 | Tham số có khoảng trắng | `parseSlashCommand` dùng `strings.Fields` → `/export "a b.txt"` vỡ. Hỗ trợ ngoặc kép (như `/start`) hoặc web tự chặn như popover |
| GAP-23 | `GET /api/commands` + `Params` trong registry | để popover/hướng dẫn không chép tay danh sách |
| GAP-24 | Việt hoá thông báo của host | `SetAdvanceMode`, `Reopen`, `requireCleanChapters` còn trả tiếng Trung (vd `章节推进模式已切换为…`, `请先执行 /sync`). Văn bản đích nằm trong mục “Nhật ký sẽ hiện” / “Lỗi hay gặp” của artboard |
| GAP-25 | `/next`, `/review` trên web | nút tương đương ở Workspace/Trình đọc gọi cùng API, popover chỉ là đường tắt |

## 6. Cập nhật 25/10 — `/export`, `/simulate` <!-- brief-2510 -->

- **`/export`** khớp registry: `/export [đường-dẫn|txt|epub|narration] [from=N] [to=M] [--overwrite|--keep] [doc-so]`.
  - `narration` = **Bản đọc TTS** (không tiêu đề chương, không in nghiêng/đậm, chương cách nhau hai dòng trống).
  - `--keep` = **Giữ cả hai**: bản mới thêm `-YYYYMMDD-HHmmss` trước đuôi tệp. Dùng chung với `--overwrite` ⇒ lỗi “chỉ dùng một trong --overwrite hoặc --keep”.
  - `doc-so` = **Đọc số thành chữ**, chỉ có tác dụng với bản đọc TTS.
  - Artboard: `Commands.cmd=export`; thẻ kết quả `CmdResults.view=exportDone · exportExists` không hiện đường dẫn, có hàng “Xuất thêm định dạng”.
- **`/simulate`**: mô tả không nhắc thư mục — “Thêm vài đoạn văn bạn thích vào Văn mẫu (Tuỳ chỉnh › Cách viết › Văn phong của bạn)…”. Backend: GAP-35.
- **`/reopen`**: ẩn khỏi gợi ý `/` và bảng lệnh với người không có quyền viết truyện; trang Hướng dẫn vẫn liệt kê, nhãn “Cần quyền viết”; gõ tay ⇒ thẻ lỗi “Bạn chỉ có quyền đọc truyện này”.
