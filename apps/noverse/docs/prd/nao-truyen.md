# Não truyện: bộ nhớ liên kết theo cấp truyện, dự án và hệ thống

- **Ngày:** 25/10/2026
- **PO:** Dat Nguyen
- **Trạng thái:** đã chốt hướng (25/10). Màn hình chờ design.
- **Liên quan:** `toi-uu-ainovel-cli-tieng-viet.md` (P1-3, P2), `rules/he-thong-san-xuat-lien-tuc.md` (§1 luật khoảng cách, §4 vũ trụ chung, §5 sổ cái), `docs/adr/0001-nao-truyen-du-lieu-goc-json.md`.

## Bối cảnh

PO muốn mỗi cấp dự án có một "não" kiểu Obsidian: các ghi chú liên kết đầy đủ với nhau, xem được dạng sơ đồ. PO cũng đề nghị xem xét [agentmemory](https://github.com/rohitg00/agentmemory) kết hợp embedding chạy trên máy.

Hiện trạng đo trên truyện thật `01-tra-xanh-cuop-kich-ban` (8 chương):

- **Đã có, nhưng rời rạc:** dữ liệu nhớ của truyện nằm trong `output/novel/`, gồm `characters.json`, `relationship_state.json`, `world_rules.json`, `foreshadow_ledger.json`, `timeline.jsonl`. Engine đã ghi kèm mỗi tệp JSON một bản `.md` sinh một chiều (`internal/store/characters.go:27`, `internal/store/world.go:259`, `:337`).
- **Rất nhỏ:** toàn bộ dữ liệu nhớ khoảng 100 KB, văn bản truyện 78 KB. Ở cấp truyện, mọi thứ vừa ngữ cảnh model. Vấn đề không nằm ở chỗ tìm kiếm.
- **Mất lịch sử quan hệ:** `WorldStore.UpdateRelationships` (`internal/store/world.go:276`) ghi đè theo cặp. `pairKey` (`world.go:492`) không phân biệt chiều A→B và B→A. Truyện thật chỉ còn trạng thái cuối, ghi `"chapter": 8`.
- **Không có chỗ cho xưng hô:** `domain.Character` gồm `name, aliases, role, description, arc, traits`. `domain.RelationshipEntry` gồm `character_a, character_b, relation, chapter`.
- **Cắt ngữ cảnh theo nguyên khối:** khi vượt ngân sách, `trimByBudget` (`internal/tools/novel_context.go:531`) xoá nguyên khối theo thứ tự `timeline`, `recent_state_changes`, `foreshadow_ledger`, `relationship_state`. Chương đông nhân vật có thể mất toàn bộ quan hệ.
- **Cấp dự án gần như trống:** `project.json` chỉ có `name, description, notes, createdAt`. Theo nhịp 5–7 truyện mỗi tuần (§2 hệ thống sản xuất), dự án sẽ có khoảng 300 truyện mỗi năm. Sổ cái truyện và luật khoảng cách (§1, §5) hiện làm tay trên bảng tính.

## Đánh giá agentmemory

**Kết luận:** không dùng agentmemory làm thành phần chạy kèm. Lấy lại các mẫu thiết kế của nó và tự làm trong Go. Lý do ghi ở ADR 0001.

| Điểm | agentmemory | Hệ quả với ainovel |
|---|---|---|
| Mục đích | Bộ nhớ phiên của agent viết code. Ghi lại thao tác qua 12 hook (`PostToolUse`, `SessionStart`…) | Không có khái niệm nhân vật, chương, xưng hô |
| Môi trường chạy | TypeScript, Node ≥ 20, iii-engine khoá ở v0.22.1, mở 4 cổng (3111/3112/3113/49134) | Phá mô hình một tệp chạy Go `serve` qua LAN/Tailscale. Windows phải cài engine tay |
| Embedding trên máy | Mặc định `Xenova/all-MiniLM-L6-v2` | Model huấn luyện chủ yếu bằng tiếng Anh, yếu với tiếng Việt [suy luận, chưa đo] |
| Số liệu công bố | 95,2% có vector so với 86,2% chỉ BM25 trên LongMemEval | Đo trên tiếng Anh. Không áp được cho truyện tiếng Việt |
| Tách từ BM25 | Có xử lý chữ Latin có dấu | Không ghép âm tiết tiếng Việt thành từ |
| Quên theo thời gian (Ebbinghaus) | Lõi của hệ thống | **Sai với truyện**: chi tiết cài ở chương 1 không được "phai" |
| Giấy phép | Apache-2.0 | Được phép chép ý tưởng |

**Lấy lại:**

- Thực thể và quan hệ có kiểu.
- Bản mới thay bản cũ nhưng vẫn giữ chuỗi phiên bản.
- Ghi nguồn gốc: mỗi dữ kiện trỏ về chương sinh ra nó.
- Phát hiện mâu thuẫn và tự kiểm (lint).
- Tìm lai BM25 + vector + đồ thị, gộp bằng RRF (k=60).
- Nguyên tắc "lược đồ là sản phẩm" trong [gist LLM Wiki v2](https://gist.github.com/rohitg00/2067ab416f7bbe447c1977edaaa681e2).

**Không lấy:** hook thu thao tác, cơ chế phai và quên, bốn tầng hợp nhất, chia sẻ nhóm/mesh, 54 công cụ MCP.

## Quyết định đã chốt (25/10)

1. **Dữ liệu gốc vẫn là JSON** do engine ghi. Kho ghi chú Obsidian là bản sinh ra để đọc. Mọi sửa đổi đi qua web/API. Cách này giữ nguyên tắc bất biến số 6 của `docs/design/v1/README.md` ("`output/novel/` do engine quản lý").
2. **Đợt đầu chỉ gồm nhân vật, quan hệ, xưng hô** (W1 + W2).
3. **Làm W0 trước khi viết code tìm kiếm** (đã làm 25/10, xem `evals/nao-w0/results.md`). Kết quả: vector không hơn `bm25` thuần (R@5: bm25 72,9%, bge-m3 72,9%, qwen3 75,0%, tức hơn một câu trên 48). **W4 hoãn**, đo lại khi dự án có từ 10 truyện.
4. **P1-3 (khóa xưng hô) của PRD giao qua Não truyện W1 + W2,** thay cho phương án thêm ma trận vào `characters.json`.
5. **Màn hình do PO trao đổi với design.** Dev không tự vẽ (chỉ thị "không tự chế"). Danh sách nhu cầu ở mục *Cần design*.

## Thuật ngữ

| Từ | Nghĩa |
|---|---|
| Não truyện | Toàn bộ dữ kiện của một truyện hoặc dự án cùng liên kết giữa chúng |
| Dữ kiện gốc | Sự thật trong truyện đã được engine ghi nhận, kèm chương nguồn |
| Thực thể | Một đối tượng có trang riêng: nhân vật, địa điểm, vật, sự kiện, phục bút, quy tắc thế giới, chương |
| Quan hệ | Cạnh có kiểu và có hiệu lực theo chương giữa hai thực thể |
| Xưng hô | Cách A gọi B và cách A tự xưng khi nói với B. **Có chiều**: A→B khác B→A |
| Kho ghi chú | Thư mục Markdown sinh từ dữ kiện gốc, mở được bằng Obsidian |
| Sổ cái truyện | Một dòng cho mỗi truyện của dự án, gồm 10 trục (§5 hệ thống sản xuất) |
| Vũ trụ chung | Nhân vật hoặc địa điểm dùng lại giữa nhiều truyện cùng dự án (§4) |

## Ba cấp

| Cấp | Chứa | Đợt |
|---|---|---|
| Truyện | Dữ kiện gốc: nhân vật, quan hệ, xưng hô, phục bút, sự kiện, quy tắc thế giới, chương | W1, W2 |
| Dự án | Sổ cái truyện, nhân vật vũ trụ chung, tuyển tập, liên kết giữa các truyện, tìm kiếm | W3, W4 |
| Hệ thống (cả thư viện) | Kiến thức cách làm, không chứa dữ kiện truyện: bài học từ các vòng duyệt, lỗi lặp lại theo mục của gói thể loại, sở thích viết của người dùng, số liệu kênh. Liên kết giữa các dự án | Khung chốt ngay; tính năng làm sau W3 |

## Cấp hệ thống: chốt khung ngay, chưa làm tính năng

**Lý do chốt ngay:** bốn điểm dưới đây rất khó đổi khi dữ liệu W1 đã ghi ra đĩa. Làm sai từ đầu, sau này phải chuyển đổi mọi truyện.

**Lý do chưa làm tính năng:** thư viện hiện chỉ có một dự án (`dit-dang-audio`), nên chưa có dữ liệu để đo hay kiểm. Bắt đầu làm khi W3 xong và có ít nhất hai dự án đang chạy.

Khung bắt buộc từ W1:

1. **Mã thực thể ổn định, có phạm vi.**
   - Mã lưu trong truyện là mã **cục bộ**, ví dụ `nv:ky-duong`. Slug bỏ dấu tiếng Việt (`đ` → `d`), chữ thường ASCII, ký tự khác thành `-`; trùng mã thêm `-2`, `-3`.
   - Mã cấp một lần, không đổi khi đổi tên. Tên hiện tại chỉ là thuộc tính. Mã đầy đủ `truyen:dit-dang-audio/01-tra-xanh-cuop-kich-ban:nv:ky-duong` tính từ vị trí khi cần, không ghi vào dữ kiện; chuyển hoặc đổi tên thư mục truyện không phá liên kết.
   - `characters.json` thêm trường `id` tuỳ chọn; truyện cũ vẫn đọc được, nhận mã trong bộ nhớ và lưu mã ở lần lưu hồ sơ tiếp theo, không cần bước chuyển đổi riêng.
2. **Mỗi dữ kiện mang phạm vi và nguồn.**
   - `scope` là một trong `truyen`, `du-an`, `he-thong`.
   - `source` ghi chương hoặc lần duyệt sinh ra dữ kiện.
   - Dữ kiện truyện **không bao giờ** tự chảy sang truyện khác. Thứ duy nhất được dùng chung là thực thể cấp dự án (vũ trụ chung) mà truyện liên kết tới một cách tường minh.
3. **Lọc theo quyền tài khoản ở mọi đường đọc.**
   - Tài khoản đã có quyền theo từng truyện (`Books`: `p/s`, `p/*`, `*` trong `platform/identity/store.go`).
   - Tìm kiếm, ngữ cảnh và đồ thị ở cấp dự án hay hệ thống phải lọc theo `CanRead`, để người viết không thấy truyện ngoài quyền.
4. **Liên kết ghi bằng mã, hiển thị bằng tên.**
   - Kho ghi chú dùng `[[đường-dẫn-từ-gốc-thư-viện|Tên]]`. Nhờ vậy, mở cả thư viện như một vault Obsidian vẫn không lẫn tên trùng giữa các truyện và dự án.

Hệ thống chỉ giữ **kiến thức cách làm**, gồm:
- bài học rút từ kết quả editor;
- mục luật hay bị vi phạm theo từng gói;
- sở thích viết đã học.

Hệ thống không giữ dữ kiện truyện, vì ghép chéo canon giữa các truyện là lỗi chứ không phải tính năng.

Quy tắc viết đã có ba cấp (Truyện · Dự án · Mọi truyện, `docs/design/v1/20-settings-rules.md`). Cấp hệ thống của não đi cùng chuỗi kế thừa đó, không lập chuỗi thứ hai.

## Mô hình dữ liệu (đợt đầu)

Giữ nguyên các tệp hiện có để tương thích ngược. Phần thêm vào:

**`relationship_log.jsonl`**, chỉ ghi thêm, mỗi dòng là một thay đổi:

```json
{"chapter": 5, "from": "nv:lam-uyen", "to": "nv:ky-duong", "relation": "người yêu cũ → đối thủ", "scope": "truyen", "source": "chương 5"}
```

`relationship_state.json` thành bản tính ra từ log: trạng thái mới nhất của mỗi cặp, đúng dạng hiện tại. Người đọc cũ không cần sửa.

**`address_terms.json`**, xưng hô có chiều và có hiệu lực theo chương:

```json
[{"from": "nv:ky-duong", "to": "nv:lam-uyen",
  "calls": "chị", "self": "em", "from_chapter": 1, "until_chapter": 0, "reason": "", "scope": "truyen", "source": "chương 1"}]
```

- `until_chapter = 0` nghĩa là còn hiệu lực.
- Đổi xưng hô hợp lệ phải đóng bản cũ (`until_chapter`) và mở bản mới kèm `reason` (ví dụ "chương 6 lật mặt, hạ xưng hô").
- Thay đổi không có `reason` bị từ chối trước khi đóng băng bản nộp; lời nhắc lỗi yêu cầu agent giải thích sự kiện dẫn đến đổi xưng hô. Nhắc lại cách xưng hô giống hệt là thao tác rỗng.

**Lệch PRD có chủ ý:** P1-3 ghi "thêm ma trận vào `characters.json`". Xưng hô là dữ liệu của **cặp có chiều**, không thuộc về một nhân vật. Tách tệp riêng giữ `characters.json` nguyên dạng và không phải chuyển đổi dữ liệu.

**Nhận diện biệt danh:** "Lâm Uyển", "Uyển Uyển", "chị Uyển" nối về một thực thể qua `aliases` sẵn có và một bảng tất định. Không dùng embedding cho việc này.

Agent ghi `commit_chapter.address_terms: [{from, to, calls, self, reason}]` bằng tên hoặc biệt danh trong `characters.json`, khi hai người lần đầu đối thoại và khi cách xưng hô đổi; chương lấy từ bản nộp. So khớp đúng tên/dấu trước, sau đó không phân biệt hoa thường, không bỏ dấu. Tên lạ trả danh sách tên đã biết để sửa. `save_foundation` với `type=characters` cũng nhận `address_terms` ở cấp trên cùng, hiệu lực từ chương 1. Dữ liệu ghi xuống dùng mã cục bộ. Truyện có trạng thái quan hệ cũ nhưng chưa có log được gieo lịch sử một lần với nguồn `trạng thái cũ`; tu chỉnh chương chỉ thêm khác biệt với nguồn `viết lại chương N`, không xóa log. Xóa dữ kiện quan hệ qua tu chỉnh chương chưa thuộc W1.

W1 ghi xưng hô từ bản nộp chương mới và thiết lập nền. Sửa hồi tố xưng hô khi viết lại chương cũ chưa thuộc W1: cần đưa xưng hô vào hồ sơ sự kiện chương và cơ chế dựng lại lịch sử, không đóng bản hiện tại bằng một mốc chương lùi.

## Chọn ngữ cảnh theo đồ thị (W2)

Thay việc xoá nguyên khối trong `trimByBudget` bằng cách chọn theo đồ thị cho chương N:

1. Nhân vật có mặt trong đại cương chương N.
2. Quan hệ còn hiệu lực ở chương N giữa họ, và quan hệ đi thêm một bước.
3. Xưng hô còn hiệu lực giữa các nhân vật có mặt. **Không bao giờ bị cắt**, vì phần này nhỏ.
4. Phục bút đang mở có dính tới các nhân vật đó.
5. Sự kiện trong K chương gần nhất có các nhân vật đó.

Khi vẫn vượt ngân sách thì cắt **từng mục, từ cũ nhất**, không cắt cả khối. `_trimmed` ghi các mục đã cắt.

## Kiểm bằng máy (W2)

- **Lint xưng hô:** đối chiếu đại từ trong lời thoại với `address_terms.json`. Luật thuần Go, tất định, đúng yêu cầu phi chức năng của PRD.
- **Rủi ro:** xác định ai đang nói chỉ chắc chắn khi có câu dẫn thoại ("Kỷ Dương nói"). Lời thoại không gắn người nói chỉ được báo mức cảnh báo. `check_consistency` vẫn nhận bảng xưng hô làm dữ liệu đối chiếu để editor xét phần còn lại.
- **Lint mâu thuẫn:** đổi quan hệ hoặc xưng hô không có `reason`; nhân vật đã chết vẫn xuất hiện; phục bút được giải trước khi cài.

### Quy tắc W2 đã triển khai

- `novel_context(chapter=N)` bật chọn đồ thị khi có lịch sử W1 và nhận ra nhân vật trong đại cương/kế hoạch. Hai cấu trúc hiện không có danh sách diễn viên riêng: khớp tên/mã/biệt danh trong văn bản, không dùng biệt danh chỉ là đại từ như “tôi”. Không nhận ra ai thì giữ ngữ cảnh cũ, không đoán diễn viên.
- Quan hệ lấy bản mới nhất có `chapter <= N`, chỉ giữ cạnh chạm nhân vật có mặt (một bước). Phục bút lấy từ sổ đầy đủ, lọc mốc cài/giải tại N và tên nhân vật trong mô tả. Timeline lấy K từ `ContextProfile.TimelineWindow`. Xưng hô có hiệu lực giữa các nhân vật có mặt nằm ở `working_memory.address_terms`, không thuộc danh sách được cắt. Nếu phần bắt buộc vượt ngân sách, trả lỗi rõ ràng thay vì cắt xưng hô.
- Chỉ đường đồ thị cắt từng phần tử, cũ nhất trước; trường không có mốc thời gian dùng ưu tiên cũ và thứ tự khóa ổn định. `_trimmed` ghi đường dẫn/mã/mốc của từng phần tử. Truyện không có dữ kiện W1 giữ chính sách cắt cũ.
- `internal/naolint.Address` kiểm lời thoại trong dấu `“…”` hoặc `"…"`, trên cùng một dòng. Tên/biệt danh theo sau bởi động từ dẫn thoại (`nói`, `hỏi`, `đáp`, `trả lời`, `gọi`, `nhắn`, `thì thầm`, `quát`, `cười`, `tiếp lời`, `lên tiếng`, `bảo`, `gật đầu`) ở phần ngoài ngoặc của cùng đoạn là người nói chắc chắn. Chỉ có một tên ở phần ngoài ngoặc, hoặc một đoạn trần thuật ngay trước/sau (bỏ dòng trống, không vượt qua đoạn thoại khác), là suy luận.
- Người nghe chắc chắn chỉ khi được gọi tên ở đầu lời thoại (`Tên,`, `Tên à,`, `Tên ơi,`). Nếu không, chỉ một nhân vật khác trong đoạn/đoạn trần thuật kề mới được suy luận là người nghe. Đoạn sau giới thiệu một tên riêng mới không được dùng làm người nghe. Biệt danh “tôi” chỉ được dùng nhận diện người kể ở phần trần thuật, không nhận diện nhân vật từ lời thoại.
- Chỉ báo `[LỖI]`/`severity=error` (Master §15.3) khi cả hai danh tính chắc chắn; có một bên suy luận thì `warning`. Bỏ qua khi không xác định được hai bên. Không tự chia lượt thoại xen kẽ: thử nghiệm trên truyện mẫu cho thấy đoán lượt tạo nhiều gán nhầm người nói.
- Đại từ tự xưng rõ `tôi/tớ/tao`; đại từ gọi ở vị trí hô cách, đầu mệnh đề mệnh lệnh `đừng/hãy`, hoặc câu có đại từ tự xưng rõ và vị trí hướng người nghe (`nhờ`, `cảm ơn`, `giúp`, `cho`). Đại từ đa nghĩa ở đầu mệnh đề chỉ xem là tự xưng khi cùng lời có cách gọi đã biết và không có tự xưng rõ khác. Bỏ `anh ấy/cô ta/… đó`, danh xưng trước tên riêng, và “mình” (thường là số nhiều bao gồm người nghe). Không kiểm đại từ trong lời kể, thoại không ngoặc/qua nhiều dòng, thoại không rõ người nói, lời diễn kịch không rõ người nghe. Ưu tiên độ chính xác, không tuyên bố bao phủ mọi đổi đại từ.
- Commit thường và viết lại trả vi phạm chung trong `rule_violations`, đồng thời lưu cho editor như các luật khác; không chặn giao thức commit. `check_consistency` luôn nhận `address_terms` tại chương đang kiểm. `naolint.Chapter` và `naolint.All` cho phép kiểm lại chương hoặc toàn bộ bản cuối đã nộp; lỗi đọc dữ kiện được trả rõ, không giả thành kết quả sạch.
- Mâu thuẫn: bản xưng hô đổi mà thiếu `reason`; quan hệ đổi mà thiếu `StateChange(field=relation/relationship/quan_he).Reason` cùng chương/nhân vật (W1 chưa có trường `reason` trong `relationship_log`); phục bút có `resolved_at < planted_at`; nhân vật có trạng thái `dead/deceased/chết/đã chết/tử vong` ở `StateChange(field=status/trang_thai)` và xuất hiện trong danh sách nhân vật của sự kiện sau đó. Không suy ra cái chết/việc xuất hiện chỉ từ nhắc tên trong văn xuôi; hồi tưởng cần người duyệt. Hồ sơ `Character` không có trường chết riêng.

### Đo truyện mẫu W2 (25/10/2026)

Nguồn được sao chép chỉ đọc từ `01-tra-xanh-cuop-kich-ban` sang `/tmp/ainovel-nao-w2-sample`; chỉ bản sao có `address_terms.json`. Đối chiếu 8 bản cuối `output/novel/chapters/01.md`–`08.md`, số dòng tính cả tiêu đề và dòng trống. Bảng gốc có hiệu lực từ chương 1, không tự ghi “lý do” để hợp thức hóa đoạn đổi. Các cặp khác không có đủ chứng cứ tự xưng/gọi thì không được bịa thêm.

| Chiều | Gọi / tự xưng chủ đạo | Dẫn xuất (chương:dòng) |
|---|---|---|
| Lâm Tri Hạ → Tống Nhược Vy | chị / tôi | 01:71,81; 02:35,87,91,107; 03:31,35; 04:103,107,111; 07:69,93,99 |
| Tống Nhược Vy → Lâm Tri Hạ | em / chị | 01:67,75; 02:15,33,85,99,103; 03:39; 04:5,37,65,101; 07:63,67,97; 08:61,65 |
| Lâm Tri Hạ → Phùng Chấn | anh / tôi | 01:11,15,19,27; 05:57,61,65; 06:39,43,51,59; 07:43,83; 08:83 |
| Phùng Chấn → Lâm Tri Hạ | cô / tôi | 01:7,13,25,31; 05:59,63; 06:33,37,41,49,53,57; 07:41 |
| Lâm Tri Hạ → Kỷ Dương | cậu / tôi | 02:45,69,81; 03:3,85,89; 05:39,43; đối chiếu 06:11,27 và 08:139 đổi sang em/chị |
| Kỷ Dương → Lâm Tri Hạ | chị / em | 02:47,65; 03:7,13,19,91; 04:27; 05:37,41,47; 06:9,13,21,25; 08:133 |
| Lâm Tri Hạ → Hứa Hoài Châu | anh / tôi | 03:121,147; 06:81; 08:147 |
| Hứa Hoài Châu → Lâm Tri Hạ | cô / tôi | 05:121; 06:79,87,91; 07:107 |
| Lâm Tri Hạ → Đỗ Mẫn | chị / tôi | 01:91,101; 02:23,127,131; 03:67 |
| Đỗ Mẫn → Lâm Tri Hạ | cô / tôi | 01:99; 02:129; 03:69; đối chiếu 05:107,115 đổi sang em/chị |

Kết quả cuối của `naolint.All`: **2 cảnh báo, cùng một dòng, không có lỗi chắc chắn**. Đây là toàn bộ kết quả, không chỉ trích mẫu:

| Chương | Dòng | Nhân vật | Thấy → bảng | Mức | Trích nguyên lời |
|---|---:|---|---|---|---|
| 7 | 89 | Tống Nhược Vy → Lâm Tri Hạ | mày → em | cảnh báo | “Mày đào chuyện này để hủy tao?” |
| 7 | 89 | Tống Nhược Vy → Lâm Tri Hạ | tao → chị | cảnh báo | “Mày đào chuyện này để hủy tao?” |

Đánh giá bằng người đọc: **0 gán nhầm người nói/người nghe/đại từ trong 2 phát hiện cuối**; cả hai là lệch bảng thật nhưng cơn giận có thể là lý do hợp lệ trong văn cảnh, nên không đồng nghĩa 2 lỗi sáng tác vô cớ. Không có `reason` có cấu trúc cho đoạn này trong bảng kiểm. Nếu coi đổi giọng do giận là có chủ ý thì hai cảnh báo này cần được miễn sau duyệt, không nâng thành `[LỖI]`.

**Chưa đạt tiêu chí “đủ 4 nhân vật”.** Có chứng cứ đọc tay cho đổi `cậu → em`/`tôi → chị` của Tri Hạ khi nói với Kỷ Dương (06:11,27; 08:139), `cô/tôi → em/chị` của Đỗ Mẫn (05:107,115), và `cô/tôi → em/anh` của Phùng Chấn (08:81,85), nhưng các dòng thiếu đủ câu dẫn cùng đoạn hoặc không có vị trí đại từ chắc chắn nên máy bỏ qua. Nhược Vy đổi sang `mày/tao` được bắt ở 07:89; 08:73 thiếu người nói đủ rõ nên bỏ qua. Không khẳng định Kỷ Dương tự đổi cách tự xưng: trong bản cuối đã đọc, `em/chị` của cậu vẫn nhất quán. PRD không cung cấp nhãn chuẩn chỉ rõ bốn người/bốn chỗ; không thể báo tỷ lệ bắt đủ bốn bằng suy đoán hoặc đổi bảng theo từng đoạn. Cần dữ liệu người nói/người nghe theo cảnh hoặc người duyệt đối chiếu bảng qua `check_consistency` để tăng recall mà không đánh đổi precision.


## Kho ghi chú Obsidian (W1)

- **Vị trí:**
  - Cấp truyện: `output/novel/nao/`.
  - Cấp dự án: `<dự án>/nao/` (trang mục lục, nhân vật vũ trụ chung).
  - Không đặt trong thư mục ẩn, vì Obsidian bỏ qua thư mục bắt đầu bằng dấu chấm.
- **Mỗi thực thể một ghi chú,** có frontmatter YAML (loại, biệt danh, chương xuất hiện) và `[[wikilink]]` tới thực thể liên quan.
  - Liên kết `[[nhan-vat/<slug>|Tên]]` tính từ gốc kho `output/novel/nao/`; mở chính thư mục này làm vault Obsidian.
- **Sinh lại toàn bộ sau mỗi lần ghi dữ kiện.** Không đọc ngược từ kho ghi chú. Đầu mỗi ghi chú ghi rõ "tệp sinh tự động, sửa trong ainovel".
- **Giới hạn:** [suy luận] Graph view của Obsidian không phân biệt loại cạnh. Đồ thị có kiểu đầy đủ phải nằm trong màn của ainovel.

## Tìm kiếm cấp dự án (W3, W4)

- **Dùng cho:**
  - so tiền đề và câu đinh với sổ cái để bắt lỗi "giống truyện trước" (§1, §8);
  - tìm nhân vật vũ trụ chung (§4);
  - tìm lại đoạn văn cũ theo nghĩa.
- **Chỉ mục là dữ liệu phụ,** xoá đi dựng lại được từ dữ kiện gốc. Cách lưu (giữ trong bộ nhớ hay SQLite thuần Go) chốt ở W3 sau khi đo. Không thêm cơ sở dữ liệu vector riêng.
  - [suy luận] So cosine vét cạn trên 10⁴–10⁵ đoạn chỉ tốn vài mili-giây trong Go.
- **Tìm theo từ:** giữ nguyên dấu (ma/má/mà khác nghĩa). W0 cho thấy trộn dạng bỏ dấu làm kết quả kém đi (paraphrase từ 56,2% xuống 50,0%), nên chỉ thử dạng bỏ dấu khi truy vấn có dấu không ra kết quả nào.
- **Đồ thị không trộn thứ hạng với từng đoạn văn.** W0 cho thấy gộp thực thể bằng RRF ở mức đoạn kéo R@5 từ 72,9% xuống 58,3%. Đồ thị dùng để trả dữ kiện có cấu trúc (xưng hô, quan hệ, phục bút) và để lọc phạm vi. Đoạn văn xếp hạng bằng `bm25`.
- **Gộp kết quả:** RRF (k=60) chỉ dùng giữa `bm25` và vector, khi W4 được bật.
- **Embedding (W4):** mặc định **tắt**.
  - Khi bật, gọi một endpoint `/v1/embeddings` kiểu OpenAI chạy trên máy (Ollama, LM Studio, llama.cpp server). Không nhúng runtime vào tệp chạy.
  - Vector lưu đệm theo mã băm của từng đoạn.
  - **Ứng viên:**
    - `BAAI/bge-m3`: MIT, hạng 9/32 trên [VN-MTEB](https://leaderboard.mteb.org/models/BAAI/bge-m3).
    - `Qwen3-Embedding-0.6B`: chưa kiểm hạng. Bài [VN-MTEB](https://arxiv.org/abs/2507.21500) ghi nhận nhóm model RoPE nhìn chung hơn nhóm APE.
  - **Loại:** `vietnamese-bi-encoder`, vì cần tách từ kiểu PhoBERT ngoài Go.

## Lộ trình và nghiệm thu

| Đợt | Việc | Nghiệm thu |
|---|---|---|
| W0 · Đo trước ✓ (25/10) | 48 câu hỏi tiếng Việt (16 lexical, 16 paraphrase, 16 entity) trên 150 đoạn của truyện thật; so bm25, bm25 bỏ dấu, bm25 + đồ thị, bge-m3, qwen3-embedding:0.6b, và hybrid. Bộ đo: `evals/nao-w0/` | Đạt. R@5: bm25 72,9%, bge-m3 72,9%, qwen3 75,0%, bm25 + đồ thị 58,3%. Vector không hơn rõ, nên hoãn W4 |
| W1 · Dữ kiện có lịch sử | `relationship_log.jsonl`, `address_terms.json`, công cụ ghi xưng hô cho agent, sinh kho ghi chú | Viết lại truyện mẫu: đủ lịch sử quan hệ theo chương. Mở `output/novel/nao/` bằng Obsidian thấy liên kết đúng. Truyện cũ không có tệp mới vẫn chạy tiếp |
| W2 · Dùng dữ kiện | Chọn ngữ cảnh theo đồ thị, lint xưng hô, lint mâu thuẫn | Truyện mẫu bị báo đủ 4 chỗ lệch xưng hô (P1-3). Ép ngân sách nhỏ: xưng hô của nhân vật có mặt không bị cắt |
| W3 · Não dự án | Sổ cái 10 trục, kiểm luật khoảng cách, nhân vật vũ trụ chung, tìm theo từ + đồ thị | Tạo truyện trùng tổ hợp trục 1+3+4+6 với một trong 10 truyện gần nhất thì bị báo `[LỖI]` |
| W4 · Vector (hoãn) | Endpoint embedding trên máy, đệm vector, RRF với `bm25` | Chỉ làm khi `evals/nao-w0/run.py` chạy trên dự án có ≥ 10 truyện (thêm câu hỏi xuyên truyện) cho vector hơn `bm25` ≥ 5 điểm R@5. Tắt embedding thì mọi tính năng khác vẫn chạy |
| W5 · Não hệ thống (sau W3, khi có ≥ 2 dự án chạy) | Bài học từ kết quả editor, mục luật hay bị vi phạm theo gói, sở thích viết; nạp vào ngữ cảnh theo chuỗi Truyện · Dự án · Mọi truyện | Một lỗi `[LỖI]` lặp ở ≥ 3 truyện của cùng gói được nhắc trước khi viết truyện mới. Người viết không có quyền ở truyện X thì không thấy gì từ X |

**Vị trí trong lộ trình PRD:**

- W0, W1, W2 vào **Sprint 3**, thay P1-3.
- W3, W4, W5 thành **P2-5**, **P2-6** và **P2-7**.
- Khung mã/phạm vi/quyền/liên kết của cấp hệ thống là điều kiện nghiệm thu của W1, không chờ W5.

## Cần design

Dev chưa làm màn hình nào dưới đây khi chưa có artboard. Bản gom cho design (đánh số D7–D15, cùng các nhu cầu khác của PRD) nằm ở `brief-design.md`.

1. **Màn Não truyện (máy tính):** đồ thị thực thể và quan hệ có kiểu; lọc theo loại; thanh chọn chương để xem quan hệ và xưng hô tại chương N; bảng chi tiết khi chọn một thực thể.
2. **Trang nhân vật:**
   - bảng xưng hô (gọi người kia là gì, tự xưng là gì, từ chương nào, lý do đổi);
   - lịch sử quan hệ theo chương;
   - chương xuất hiện.
3. **Sửa dữ kiện trên web:**
   - sửa xưng hô và quan hệ (vì JSON là gốc);
   - trạng thái khoá khi engine đang viết;
   - xác nhận khi sửa làm thay đổi chương đã viết.
4. **Vi phạm xưng hô** trong Nhật ký và kết quả kiểm. Có thể dùng thẻ sẵn có; cần design xác nhận.
5. **Não dự án:** sổ cái truyện (10 trục), cảnh báo luật khoảng cách, nhân vật vũ trụ chung, ô tìm kiếm.
6. **Điện thoại:** có làm đồ thị ở 390 px hay chỉ danh sách thực thể.
7. **"Mở bằng Obsidian" và cài đặt embedding:** đặt ở ngăn Nâng cao (chỉ quản trị), theo nguyên tắc bất biến số 7 không lộ đường dẫn.
8. **Não hệ thống (W5, chưa gấp):** nơi xem và duyệt bài học rút ra (giữ/bỏ, giống luật học từ chat ở `20-settings-rules.md`), và nơi xem các mục luật hay bị vi phạm theo gói.

## Ngoài phạm vi

- Chạy agentmemory hoặc iii-engine.
- Đồng bộ hai chiều từ Obsidian về dữ kiện gốc.
- Cơ chế phai và quên.
- Tính năng của não hệ thống trước khi W3 xong. Khung dữ liệu thì làm từ W1.
- Embedding gọi API trả phí.
