# 20 · Tuỳ chỉnh, Quy tắc viết & “không lộ kỹ thuật”

Artboard: `Settings.dc.html` (04 · Tuỳ chỉnh), `Outline.dc.html` (02 · Dự án & truyện), `MMore.dc.html` (M4, trang `rules` · `storyset`).

## 1. Nguyên tắc: người dùng không cần biết cấu trúc tệp

Người dùng chỉ thấy **khái niệm đời thường**. Đường dẫn, tên tệp, tên khoá cấu hình, tên công cụ và mã lỗi chỉ xuất hiện ở ngăn **Nâng cao**, và chỉ quản trị mới thấy ngăn này.

| UI hiển thị | Bên dưới (chỉ dev / Nâng cao) |
|---|---|
| Ý tưởng | `<dự án>/y-tuong/*.md` |
| Tư liệu (bìa, nhạc nền, giọng mẫu) | `<dự án>/tai-san/` |
| Truyện | `<dự án>/kich_ban/NN-slug/` + `story.json` |
| Bản xuất | `<truyện>/xuat-ban/tap-NN/…` |
| Quy tắc viết | `.ainovel/rules/*.md` ở 3 cấp |
| Tuỳ chỉnh (văn phong, trợ lý, ngân sách…) | `.ainovel/config.json` ở 3 cấp |
| Tên dự án “Kênh truyện audio” | `project.json.name`; slug thư mục **không** hiện |
| Tên truyện | `story.json.title`; tiền tố `01 ·` **không** hiện |
| Dàn ý · Nhân vật · Chi tiết cài cắm | `output/novel/outline.json` · `characters.json` · `foreshadow_ledger.json` |

**Đã dọn:**
- **Header mọi màn:** tên dự án thay cho slug, bỏ `01 ·`. Nút bánh răng giờ là “Tuỳ chỉnh”.
- **Dự án:** bỏ cột “Thư mục”, “Kiểm tra chuẩn”, mọi `y-tuong/`, `kich_ban/`, `*.json`.
- **Workspace, Nhật ký:** tên công cụ đổi sang lời thường (bảng §5). Khối lưu chương hiện “Kiểu móc / Tuyến chính / Nhân vật mới / Chi tiết cài cắm / Sự kiện chính”, không còn `hook_type`, `dominant_strand`, `cast_intros`, `foreshadow_updates`, `key_events`, `replace_all`, `max_runes`.
- **Trình đọc:**
  - Thẩm duyệt dùng nhãn tiếng Việt (Đạt · Cần gọt · Viết lại; nghiêm trọng · lỗi · lưu ý; đạt · một phần · chưa đạt).
  - Xuất truyện: hiện tên tệp thân thiện (“Trà Xanh Cướp Kịch Bản · Tập 1.epub”), không hiện đường dẫn.
- **Hộp thoại Dự án mới:** bỏ “Tên thư mục”, “Ngôn ngữ” và cây tệp. Thay bằng danh sách “Dự án mới có sẵn: Ý tưởng · Tư liệu · Truyện · Quy tắc viết · Trợ lý AI”. Trùng tên thì báo bằng lời.
- **Thiết lập lần đầu:** bước “Nơi lưu truyện” không còn ô đặt tên thư mục. Ngăn “Chi tiết kỹ thuật” vẫn thu gọn như cũ.
- **Điện thoại:**
  - Nhật ký: lời gọi công cụ hiện tên thường (“Viết bản nháp”); phần tham số chỉ hiện khi mở, nút “Chép chi tiết kỹ thuật”.
  - Dự án: dùng tên hiển thị.

## 2. Tuỳ chỉnh (`Settings.dc.html`)

> **Gói AP (31/10):** Tuỳ chỉnh **bỏ hẳn** mục *Quy tắc chung* và *Cách viết*. Quy tắc viết và văn phong chỉ còn hai cấp — truyện và dự án — sửa ở `Outline` (xem §9). Hai công tắc *Duyệt từng chương* và *Trọng tài hỏi bạn khi phân vân* chuyển xuống thẻ **Cách chạy** trong mục Trợ lý AI. Mục *Thông tin dự án* luôn hiện; bấm khi đang chọn “Truyện này” thì thanh Áp dụng cho chuyển sang “Dự án”. Thông tin truyện sửa ở `Outline` › tab Thông tin. <!-- ap-set -->

Tweak `section`: models · budget · info · connect · storage · export · notify · app · adv (mặc định `models`); `scope`: story · project · global (cấp “Mọi truyện” chỉ còn cho Trợ lý AI và Ngân sách).

### 2.1 Thanh “Áp dụng cho”

Có 3 viên chọn: **Truyện này** (tên truyện) · **Dự án** (tên dự án · số truyện) · **Mọi truyện** (toàn bộ thư viện). Dưới thanh là một câu giải thích kế thừa.

- Mỗi thiết lập có nhãn **“Riêng truyện này”** / “Riêng dự án” (tím) kèm nút *Theo dự án* để đặt lại, hoặc nhãn **“Theo dự án”** / “Theo ‘Mọi truyện’” (xám).
- Nguồn nhãn: `ConfigResponse.sources["roles.writer.model"]`.
- Trong menu trái, mục có chỉnh riêng ở cấp đang chọn có chấm tím.
- Thay đổi gom lại ở **thanh nổi đáy**: “N thay đổi cho truyện này · có hiệu lực từ chương 26” · *Huỷ* · **Áp dụng** (Quy tắc) / **Lưu** (mục khác).

### 2.2 Ma trận thiết lập

| Mục | Truyện | Dự án | Mọi truyện | Ai sửa | Ghi vào |
|---|---|---|---|---|---|
| Quy tắc viết | ✓ | ✓ | — (bỏ ở Gói AP) | có quyền ghi | `PUT /api/rules/file` (§3.3) · sửa ở `Outline` <!-- ap-mx --> |
| Văn phong (Chung · Ngôn tình · Hồi hộp · Huyền huyễn · Kiếm hiệp · Văn phong của bạn) | ✓ | ✓ | — (Gói AP: dự án chưa chọn thì dùng “Chung” của Noverse) <!-- ap-mx2 --> | có quyền ghi | `config.style` (`default · romance · suspense · fantasy · wuxia` + tên tệp tự tạo) |
| Duyệt từng chương | ✓ (đổi ngay = `/review on\|off`) | ✓ (cho truyện mới) | ✓ (cho truyện mới) | có quyền ghi | truyện: lệnh live; dự án/toàn cục: `config.advance_mode` (`review`/`auto`) |
| Trợ lý AI: model + mức suy nghĩ (Tự động · Thấp · Vừa · Cao) | ✓ | ✓ | ✓ | có quyền ghi | `config.roles.<vai>.{provider,model,reasoning_effort}`; Mặc định = `provider/model/reasoning_effort` cấp trên |
| Ngữ cảnh của model (Gói W): Tự nhận / Tự đặt (nghìn) | — | — | ✓ (theo model, mọi trợ lý) | quản trị | `providers.<p>.models[].context_window`; bỏ trống = theo danh mục model, không có thì mức dự phòng |
| Model dự phòng | ✓ | ✓ | ✓ | có quyền ghi | `roles.<vai>.fallbacks` |
| Ngân sách mỗi truyện · báo trước % · khi hết | ✓ | ✓ | ✓ | có quyền ghi | `config.budget.{book_usd,warn_ratio,hard_stop}` |
| Thông tin truyện / dự án | truyện | dự án | — | có quyền ghi | `story.json` / `project.json` |
| Kết nối AI (tài khoản ChatGPT, khoá API) | — | — | ✓ | quản trị | `providers.*`, OAuth (GAP-16) |
| Xuất tự động | — | — | ✓ thư viện | quản trị | `library.json.autoExport` |
| Báo động máy chủ | — | — | ✓ | quản trị | `config.notify.events` (lệnh shell ở Nâng cao) |
| Nâng cao | — | — | ✓ | quản trị | vị trí lưu, tên thư mục, mẫu tên tệp, cửa sổ ngữ cảnh, ngắt khi im lặng, model khi nhập truyện, lệnh báo động |

- Cấp dự án/truyện từ chối `api_key` và `notify` (400) đúng như `11-api-contract.md` §6. UI không hiện các mục này ở hai cấp đó.
- Trọng tài luôn dùng model Mặc định. UI ghi rõ điều này, không cho chọn riêng.

### 2.3 Kết nối AI — chọn model của từng nhà cung cấp (28/10, Gói AH) <!-- ah-conn -->

Tweak `conn` (Settings): list · added · models · search · manual · loading · failed.

- **Thẻ nhà cung cấp:** dòng phụ `Khoá API ••••abcd · 2 model đã chọn`. Nút **Model · n** mở bảng chọn model ngay trong thẻ. Chưa chọn model nào: viền thẻ vàng, dòng phụ “chưa chọn model, trợ lý chưa dùng được”, nút đổi thành **Chọn model** (nền sáng).
- **Thêm nhà cung cấp** (sau khi nhập khoá / đăng nhập): bảng chọn model tự mở ở bộ lọc *Gợi ý*, kèm lời nhắc “Chọn ít nhất một model để các trợ lý dùng được”.
- **Bảng chọn model:**
  - ô tìm (bỏ dấu, theo tên model hoặc tên hiển thị); bộ lọc *Đã chọn · n* · *Gợi ý* · *Tất cả*;
  - mỗi hàng: ô chọn, tên model, mô tả, ngữ cảnh (`131K`), giá vào · ra mỗi 1 triệu token nếu nhà cung cấp trả (gói thuê bao ghi “theo gói”, Ollama ghi “miễn phí”), nhãn trợ lý đang dùng (`Người viết`) hoặc `Gợi ý`;
  - model đang gán cho một trợ lý: không bỏ chọn được, giải thích “Đang dùng cho Người viết. Đổi model của trợ lý đó trước khi bỏ chọn.”;
  - danh sách dài (OpenRouter ~300): model đã chọn lên đầu, hiện một phần, ghi “đang hiện 9 trong 312, gõ tên để tìm”;
  - chân bảng: “Lấy từ {nhà cung cấp} lúc 10:24” + tải lại; **Không thấy model? Thêm theo tên**;
  - **Thêm theo tên:** ô tên + *Kiểm tra & thêm* (gọi thử model) → “trả lời được · 640 ms · ngữ cảnh 164K (tự nhận). Đã thêm và chọn.”; không tra được ngữ cảnh → cảnh báo dùng mức dự phòng 200K, đổi ở Trợ lý AI;
  - đang lấy: “Đang lấy danh sách model từ OpenRouter…”; lỗi: “Không lấy được danh sách model. OpenRouter từ chối khoá API này…” + *Đổi khoá* · *Thử lại* · *Thêm theo tên*;
  - cuối bảng: “Đã chọn n model…” + *Gán cho trợ lý →* (sang Trợ lý AI).
- Chọn/bỏ chọn là sửa bản nháp; thanh dưới ghi “n thay đổi ở Kết nối AI · dùng cho cả máy chủ” + *Lưu*.
- **Trợ lý AI › ô Model:** danh sách nhóm theo nhà cung cấp, chỉ gồm model đã chọn. Nhà cung cấp chưa chọn model hiện tiêu đề nhóm + nút “Chưa chọn model · Chọn” (mở thẳng bảng của nhà cung cấp đó). Dòng cuối: “Chọn thêm model hoặc nhà cung cấp ở Kết nối AI”.
- **Nâng cao › Tinh chỉnh model** chỉ còn phần kỹ thuật của model đã chọn (base URL, giao thức, ngữ cảnh tay, json_schema). Thêm/bớt model chỉ làm ở Kết nối AI.

## 3. Quy tắc viết

**Vị trí — chốt 28/10 (Gói AC)** <!-- quy-tac-vi-tri -->

Quy tắc viết gắn với truyện, nên sửa ở chỗ người dùng làm việc với truyện, không phải trong Tuỳ chỉnh.

| Cấp | Sửa ở đâu (máy tính) | Artboard |
|---|---|---|
| Truyện | **Dự án › truyện › tab Quy tắc viết** (cạnh Dàn ý · Nhân vật · Chi tiết cài cắm · Bản xuất; số trên tab = quy tắc riêng + học từ góp ý) | `Outline` · `tab=rules` · `rules`: list · understood · thinking · unsure |
| Dự án | **Trang dự án › Quy tắc viết chung** (mục trong trang tổng quan, có lối tắt ở thanh trái “Quy tắc viết”) | `Outline` · `view=project` |
| ~~Mọi truyện~~ | **Bỏ ở Gói AP (31/10).** Quy tắc chỉ còn cấp truyện và dự án. Quy tắc global đang có trên máy chủ phải được chuyển — xem §9.7 GAP-AP1 <!-- ap-lv --> | — |

- Tab truyện dùng **cùng thành phần** với Tuỳ chỉnh (ô thêm + chip mẫu, thẻ quy tắc, *Noverse hiểu thế nào?*, học từ góp ý, mặc định chống giọng văn máy, *Khi nào có hiệu lực*). Nhóm kế thừa (dự án, Mọi truyện) hiện viền đứt, chỉ xem, có nút *Sửa ở trang dự án ›* / *Sửa ở Tuỳ chỉnh ›*.
- Cột phải khi mở tab: thẻ **Kiểm tra 3 chương gần nhất** (thay vị trí trong Tuỳ chỉnh). Dòng *Quy tắc viết* trong thẻ “Đang áp dụng cho truyện” mở tab này thay vì sang Tuỳ chỉnh.
- “Khi nào có hiệu lực” lấy theo truyện thật: quy tắc mới có hiệu lực từ chương kế tiếp sau chương đang viết; chương đã viết không bị sửa.
- **Điện thoại** (M4 › Quy tắc viết): mở thẳng cấp truyện, bỏ nút chia 3 cấp. Nhóm kế thừa chạm vào để sửa ở cấp đó; khi đang ở cấp dự án / Mọi truyện có nút *‹ Về quy tắc của truyện*.
- Các mục khác của Tuỳ chỉnh (Trợ lý AI, Ngân sách, và trong Cách viết: Duyệt từng chương, Trọng tài hỏi) vẫn giữ thanh “Áp dụng cho” 3 cấp. **Văn phong** đã chuyển theo cùng cách với quy tắc viết — xem *Cách viết — vị trí* bên dưới (Gói AL). <!-- al-note -->
- Số từ dễ lặp mặc định là **12** (đúng danh sách đang hiện), không phải 16.

**Cách viết gọn lại, chốt 31/10 (Gói AM — thay phần chia tab của Gói AL bên dưới)** <!-- am-gon -->

Người dùng không phân biệt được “Quy tắc viết” và “Cách viết”, và bấm *Văn phong* ở thẻ “Đang áp dụng” lại bị đưa sang chỗ khác. Gọn lại theo ba ý:

**Nguyên tắc nhãn nguồn (sửa 31/10):** đang đứng ở truyện thì mọi thứ trên màn đều là *của truyện* — **không** ghi “riêng truyện này”. Chỉ đánh dấu khi giá trị **lấy từ cấp trên**: nhãn xanh “theo dự án” (ở truyện) / “chung mọi truyện” (ở trang dự án); nhóm quy tắc kế thừa ghi “Từ dự án …” / “Chung cho mọi truyện” · chỉ xem. Quy tắc tự dặn ghi “Bạn đã dặn”. Tab Cách viết không có số đếm (dễ hiểu nhầm). <!-- am-label -->

1. **Một tab “Cách viết”** cho truyện (bỏ tab “Quy tắc viết” riêng). Hai mục cùng kiểu tiêu đề, trên xuống:
   - **Văn phong** — một thẻ: tên + “Có chấm” (nếu là gói) + “theo dự án” (nếu kế thừa), một dòng mô tả, nút **Đổi** mở lưới ngay trong thẻ (dòng đầu lưới: **Theo dự án · …** để quay về kế thừa; *Xong* để thu lại). Cuối thẻ là dòng **Văn mẫu · n đoạn · x ký tự** (thu gọn, *Thêm*). Thẻ “Gói … gồm” chỉ hiện khi lưới đang mở.
   - **Quy tắc** — tiêu đề + *Noverse hiểu thế nào?* cùng hàng, một dòng mô tả bên dưới, ô dặn, rồi các nhóm: Bạn đã dặn · Học từ góp ý · Từ dự án · Chung cho mọi truyện · Mặc định của Noverse.
   - Cuối tab: **Khi nào có hiệu lực** (áp cho cả văn phong và quy tắc).
2. **Chọn nhanh ngay ở thẻ “Đang áp dụng”** (cột phải), không chuyển trang:
   - *Văn phong* → bảng chọn nhỏ: Theo dự án · các gói · các kiểu viết. Chọn là lưu, toast “Đã đổi văn phong · áp dụng từ chương 5”.
   - *Người viết* → bảng chọn model (cùng danh sách Trợ lý AI).
   - *Duyệt từng chương* → công tắc ngay trên dòng.
   - *Quy tắc* → mở tab Cách viết (quy tắc cần gõ chữ, không chọn nhanh được). Dòng Văn phong chỉ có nhãn “theo dự án” khi kế thừa; Người viết không gắn nhãn. <!-- am-rail -->
   - Thẻ dự án: *Văn phong* (bảng chọn), *Duyệt cho truyện mới* (công tắc); *Trợ lý AI*, *Ngân sách* vẫn mở Tuỳ chỉnh.
   - Tiêu đề thẻ: “Đang áp dụng” + *Tuỳ chỉnh ›*.
3. **Trang dự án:** một mục **Cách viết của dự án**, bên trong cùng hai phần như tab truyện: **Văn phong** (thẻ + “chung mọi truyện” khi kế thừa Tuỳ chỉnh; cuối thẻ liệt kê *truyện chọn văn phong khác*, hoặc “Mọi truyện trong dự án đang dùng văn phong này”) và **Quy tắc**. Thanh trái chỉ còn một lối tắt “Cách viết”. <!-- am-proj -->

Điện thoại (M4): dòng “Quy tắc viết” đổi thành **Cách viết** (“Nữ tần bảo thù · 9 quy tắc”). Trang Cách viết: một dòng mô tả (không còn tiêu đề “Riêng truyện …”), dòng *Văn phong* (nhãn “theo dự án” khi kế thừa) bấm mở danh sách chọn tại chỗ — dòng đầu **Theo dự án · …** — bên dưới là **Quy tắc** với nhóm “Bạn đã dặn”. <!-- am-m4b --> “Tuỳ chỉnh truyện” chỉ còn Cách chạy (Duyệt từng chương, Người viết) và Ngân sách.

Artboard: `Outline` · `tab=style` · `wstyle` · `quick`: none · style · writer; `MMore` · `state`: rules · rulesStyle · storyset.

**Màn Tạo truyện mới — chọn nhanh văn phong & người viết (Gói AM, 31/10)** <!-- am-new -->

Lỗi bản thật: bấm thẻ *Văn phong* (hoặc tên model) dưới ô nhập yêu cầu thì bị chuyển sang Tuỳ chỉnh, mất mạch đang gõ.

- Hai thẻ dưới ô nhập thành **nút mở bảng chọn tại chỗ** (mũi tên ▾), không còn là liên kết.
- *Văn phong* → bảng “Văn phong cho truyện này”: **Theo dự án** (mặc định, ghi tên văn phong của dự án) · Gói thể loại (kèm nhãn “có chấm”) · Kiểu viết. Không có “Văn phong của bạn” vì truyện chưa có văn mẫu. Chân bảng: *Đổi mặc định của dự án ›* (mở trang dự án › Cách viết của dự án).
- *Người viết* (tên model) → bảng model theo nhà cung cấp, dòng đầu **Theo dự án**. Chân bảng: *Thêm model ở Kết nối AI ›*.
- Thẻ chỉ hiện tên đang chọn, không gắn nhãn nguồn (dấu ✓ trong bảng cho biết đang “Theo dự án” hay chọn khác). <!-- am-ns-tag -->
- Lựa chọn chỉ giữ trên màn này (chưa có truyện để lưu); áp vào truyện lúc bấm *Bắt đầu viết*. Bấm ra ngoài hoặc Esc để đóng bảng.
- Artboard: `NewStory` · `quick`: none · style · writer · picked.

**Xoá truyện (Gói AM, 31/10)** <!-- am-del -->

- Lối vào: nút **⋯** ở đầu trang truyện (Đổi tên truyện · Xuất truyện… · **Xoá truyện…**), cùng các mục với menu ⋯ / chuột phải của thẻ truyện ở trang dự án (đã có trong bản thật).
- Chỉ **chủ dự án** (quản trị tính là chủ) xoá được. Người khác thấy mục mờ + “Chỉ chủ dự án hoặc quản trị xoá được truyện.”
- Hộp xác nhận nói rõ:
  - **Mất**: số chương, số chữ, bản viết lại; dàn ý, nhân vật, quy tắc, văn mẫu, góp ý; sách nói và bản xuất.
  - **Giữ**: ý tưởng, tư liệu, các truyện khác của dự án.
  - Lối *Xuất trước*.
  - Ô gõ **tên ngắn của truyện** (không phân biệt hoa thường) để mở nút đỏ **Xoá vĩnh viễn**. Không bắt gõ tên thư mục.
- **Truyện đang chạy:** máy chủ từ chối xoá (“Truyện đang chạy — dừng trước khi xoá”). Hộp hiện dải vàng “Người viết đang viết lại chương 4. Cần dừng trước khi xoá.” + *Tạm dừng*; nút Xoá vĩnh viễn khoá cho tới khi dừng xong.
- Xoá xong → về trang dự án, thẻ truyện biến mất, số liệu dự án cập nhật, toast “Đã xoá truyện …”.
- Artboard: `Outline` · `del`: none · menu · confirm · typed · running · noPerm · done.

**Cách viết — vị trí, chốt 29/10 (Gói AL)** <!-- cach-viet-vi-tri -->

Văn phong (gói thể loại, kiểu viết) và văn mẫu gắn với truyện, nên chọn ở Dự án & truyện, cạnh Quy tắc viết.

| Cấp | Sửa ở đâu (máy tính) | Artboard |
|---|---|---|
| Truyện | **Dự án › truyện › tab Cách viết** (sau Quy tắc viết): thanh kế thừa (“Riêng truyện này” + *Dùng theo dự án* / “Theo dự án …”), lưới Gói thể loại + Kiểu viết, thẻ “Gói … gồm”, **Văn phong của bạn · văn mẫu của truyện này** (danh sách + *Thêm văn mẫu*) | `Outline` · `tab=style` · `wstyle`: pack · builtin · inherit · samples |
| Dự án | **Trang dự án › Cách viết mặc định của dự án** (lối tắt ở thanh trái “Cách viết”): văn phong đang dùng, truyện nào chọn riêng, *Đổi* mở cùng lưới | `Outline` · `wstyle=proj` |
| ~~Mọi truyện~~ | **Bỏ ở Gói AP** — dự án chưa chọn văn phong thì hiện “Chung · mặc định Noverse”. <!-- ap-lv2 --> Bản cũ: **Tuỳ chỉnh › Cách viết › Văn phong chung** (chỉ hiện khi chọn “Mọi truyện”). Ở cấp truyện/dự án, Tuỳ chỉnh chỉ còn thẻ chỉ dẫn “Văn phong của truyện này: Gói … · riêng truyện này” + *Mở Cách viết của truyện* | `Settings` · `section=writing` · `scope` |

- Chọn một ô là lưu ngay (không qua thanh Lưu), toast “Đã đổi văn phong của truyện · áp dụng từ chương 5”; chương đã viết giữ nguyên.
- “Văn phong của bạn” chỉ chọn được ở cấp truyện và khi truyện có văn mẫu (API văn mẫu theo truyện). Ở Tuỳ chỉnh “Mọi truyện” ô này mờ: “Văn mẫu thêm theo từng truyện ở Dự án & truyện”.
- Dòng *Văn phong* trong thẻ “Đang áp dụng cho truyện” / “Mặc định của dự án” mở tab Cách viết / cuộn tới mục Cách viết của dự án (không sang Tuỳ chỉnh nữa).
- Điện thoại: M4 › “Tuỳ chỉnh truyện” vốn đã nằm trong nhóm “Đang mở” của truyện — giữ nguyên ở đợt này.

### 3.1 Trải nghiệm (máy tính)

- **Ô thêm quy tắc:** viết bằng lời thường. Có chip mẫu: *Không dùng từ… · Giới hạn từ lặp… · Xưng hô… · Nhân vật… · Giọng văn…*. Bấm **Thêm** (⌘/Ctrl+Enter) → thẻ “Noverse đang đọc quy tắc…” chữ lấp lánh 1–2 s → thẻ quy tắc mới loé tím.
- **Thẻ quy tắc:**
  - nguyên văn của bạn + **chip “Noverse hiểu là”**:

    | Chip | Màu |
    |---|---|
    | Không dùng “…” (cụm từ / ký tự cấm) | đỏ nhạt |
    | Tối đa “…” · N lần/chương (từ dễ lặp) | cam |
    | Thể loại | xanh |
    | Lời dặn cho trợ lý (sở thích, không kiểm máy) | tím |

  - Rê chuột hiện *Sửa · Chuyển lên dự án / “Mọi truyện” · Xoá*.
- **Chưa chắc hiểu đúng** (`uncertain`): khung cam “Noverse chưa chắc ‘quá dài’ là bao nhiêu” kèm 2–3 cách hiểu dạng nút và *Giữ như lời dặn*.
- **Nhóm theo nguồn**, theo thứ tự ưu tiên:
  1. **Riêng truyện này**
  2. **Học từ góp ý của bạn**: luật Trọng tài trích từ chat. Orb Trọng tài, dòng “Từ góp ý của bạn ở chương 12”, nút *Giữ làm quy tắc của truyện* / *Bỏ*.
  3. **Từ dự án**: viền đứt, kèm *Sửa ở cấp dự án ›*.
  4. **Từ “Mọi truyện”**
  5. **Mặc định của Noverse**: chống giọng văn máy, 4 cụm từ không dùng + 16 từ dễ lặp, luôn bật, mở ra để xem. Kèm gợi ý cách nới một từ.
- **“Noverse hiểu thế nào?”**: bảng tổng hợp hiệu lực (Thể loại · Không dùng · Giới hạn lặp · Lời dặn) cho truyện hoặc dự án đang chọn. Đây là thứ trợ lý thật sự nhận.
- **Cột phải:**
  - *Kiểm tra 3 chương gần nhất*: vi phạm theo chương, ví dụ “Ch. 24 · ‘dường như’ 2 lần · tối đa 1 — Biên tập viên ghi nhận”.
  - *Khi nào có hiệu lực*: “từ chương 26; chương 25 đang viết dở giữ quy tắc cũ; chương đã viết không bị sửa”. Ô chọn *Kiểm tra lại các chương đã viết theo quy tắc mới (không tự sửa)*.
  - Mẹo viết quy tắc, trong đó có ý: dặn một lần cho chương sau thì nhắn ở Workspace.

### 3.2 Điện thoại (M4 › Quy tắc viết, `rules` · `rulesAdd` · `rulesThinking`)

- Thanh 3 cấp dạng nhóm chọn; ô thêm có chip mẫu cuộn ngang. Khi gõ, thanh tab ẩn.
- Thẻ quy tắc giống máy tính; chạm vào → action sheet *Sửa · Chuyển lên dự án · Xoá* (luật học từ chat: *Giữ làm quy tắc của truyện · Bỏ*).
- Nhóm kế thừa thu thành một dòng “Kế thừa · chạm để sửa ở cấp này”.
- Thanh **Áp dụng** nổi trên thanh tab.
- **Tuỳ chỉnh truyện** (`storyset`): văn phong (radio), Duyệt từng chương, Trợ lý AI › M4 Model, ngân sách (đã dùng + giới hạn ±).

### 3.3 Lưu trữ

- Mỗi cấp, UI quản lý **một tệp** `quy-tac.md` trong `.ainovel/rules/`, mỗi quy tắc một dòng `- …`. Thứ tự dòng = thứ tự hiển thị.
- Các tệp `.md` khác người dùng tự tạo vẫn được engine đọc. UI liệt kê chúng ở *Nâng cao › Thư mục quy tắc* (sửa dạng văn bản), không trộn vào danh sách thẻ.
- Engine đã có (`internal/rules`, `internal/userrules`):
  - nguồn `global → project → story`, cùng tên tệp thì cấp gần hơn thắng;
  - LLM chuẩn hoá thành `structured {genre, forbidden_chars, forbidden_phrases, fatigue_words}` + `preferences` + `uncertain`;
  - lỗi chuẩn hoá → hạ cấp thành lời dặn nguyên văn (`status: degraded`).

### 3.4 Khoảng trống backend — GAP-32

Engine **chốt quy tắc vào truyện lúc bắt đầu viết** (`Host.PrepareUserRules` → `userrules.Service.Build`). Sau đó `GetOrBuild` chỉ đọc lại bản đã chốt; quy tắc mới chỉ được cộng qua `AddRuntimeRule` (Trọng tài). Vì vậy sửa tệp quy tắc giữa chừng hiện **không có tác dụng** với truyện đang viết.

Cần bổ sung:

| API | Việc |
|---|---|
| `POST /api/rules/understand {text}` → `{structured, preferences, uncertain[], options[], chips, status}` (`options`: 0–3 cách hiểu, mỗi mục là quy tắc đã viết lại rõ ràng — nút trong khung cam “chưa chắc hiểu đúng” §3.1) | Chuẩn hoá thử một quy tắc để hiện chip trước khi lưu (≤ 2 phút như mọi lời gọi model phía host, huỷ được) |
| `GET S/rules/effective` → `{snapshot, items: {text, scope, chips, uncertain, options}[], runtime: {id, text, chapter, createdAt, chips, uncertain, options}[]}` | Bảng “Noverse hiểu thế nào?”, nhóm “Học từ góp ý” |
| `GET /api/rules/effective?scope=global\|project&project=` → `{snapshot, items}` | Như dòng trên nhưng không gắn truyện: không `runtime`, không cấp truyện; liệt kê quy tắc từ tệp của cấp đó và các cấp cha + mặc định. Dùng cho Tuỳ chỉnh › Quy tắc ở cấp “Mọi truyện” / dự án |
| `POST S/rules/apply {recheck?: boolean}` → `{snapshot, effectiveFrom: chapter}` | Dựng lại bản chốt từ tệp, **giữ** các luật runtime (cần lưu ứng viên runtime riêng thay vì chỉ có bản đã trộn). Áp từ chương kế tiếp. `recheck` tạo việc nền `rules.recheck` (SSE `job.*`) báo vi phạm ở chương cũ, không sửa |
| `POST S/rules/runtime/:id/keep` · `DELETE S/rules/runtime/:id` | Chuyển luật học từ chat thành dòng trong `quy-tac.md` của truyện / bỏ |
| `GET S/rules/violations?last=3` | `commit_chapter` đã trả `violations` nhưng chưa lưu lại. Cần lưu theo chương (cùng chỗ thẩm duyệt) để hiện cột “Kiểm tra gần nhất” |

Đã làm (GAP-32): `BuildSnapshot` trộn **hợp có khử trùng** cho `forbidden_phrases` / `forbidden_chars` (một truyện thêm 1 từ cấm không còn làm mất 4 cụm từ mặc định); từ lặp vẫn theo từng từ, cấp gần hơn thắng. `quy-tac.md` cộng dồn cả ba cấp (không áp dụng “cùng tên → cấp gần thắng”). Chi tiết ở `11-api-contract.md` §6.

## 4. Dự án & truyện (`Outline.dc.html`)

Tweak `view`: story · project; `tab`: outline · cast · fs · export; `health`: ok · issues.

- **Thanh trên (Gói AG):** `Tên dự án ▾ / Tên truyện`. Bấm tên dự án → popover đổi dự án: danh sách dự án (tên + “2 truyện · 1 đang viết”), dấu ✓ ở dự án đang mở, *Dự án mới*. Đây là chỗ **duy nhất** đổi dự án; cùng một popover ở mọi màn có breadcrumb. Bấm tên truyện → về Tổng quan truyện. <!-- ag-nav -->
- **Trái (Gói AG, thay cây Dự án → Truyện → Tập → Chương cũ):**
  - **Thẻ truyện** trên cùng: chấm trạng thái + tên truyện + “Đang viết lại · 3 tập · 120 chương”. Bấm → popover “Trong ‘{dự án}’”: *Tổng quan dự án* · các truyện của dự án (tên, trạng thái, ✓ ở truyện đang mở) · *Truyện mới trong dự án này*. Đổi truyện là **một cú bấm**, không phải đóng tập/chương nào.
  - **Khi đang ở Tổng quan dự án:** Tổng quan · Ý tưởng (n) · Tư liệu (n) · Quy tắc viết (n) · Não dự án; dưới là “TRUYỆN · n” dạng danh sách gọn (không bung tập).
  - **Khi đang ở một truyện:** “MỤC LỤC · n ch.” **chỉ của truyện này**, có:
    - ô tìm chương (theo tên, không cần dấu, hoặc gõ số chương);
    - *Tới chương đang viết* (mở đúng tập, cuộn tới, tô sáng chương);
    - *Thu gọn mọi tập*;
    - tiêu đề tập dính trên đầu khi cuộn (`Tập 1 · 40 ch. · đang viết`);
    - **mở một tập thì tập khác tự đóng** (accordion);
    - tập chưa mở khoá: mờ, không bấm được, ghi “mở khi Tập 1 xong”; tập đã có dàn ý ghi “có dàn ý”, chưa có ghi “chưa dàn ý”.
  - đáy: viên tình trạng “Dự án đã sẵn sàng” / “Dự án có 2 việc cần sửa”.
  - Tweak `nav`: none · project (popover dự án) · story (popover truyện) · search (đang tìm “ổ cứng”); `len`: 8 · 40 · 120 (số chương ở cột trái, để xem mục lục dài).
- **Tổng quan dự án:**
  - tên + mô tả + *Sửa* (→ Tuỳ chỉnh › Thông tin dự án);
  - 4 số liệu (Truyện · Đang viết · Tổng số chữ · Bản xuất);
  - thẻ truyện (trạng thái, thanh tiến độ gradient, số tập/chương/chữ);
  - **Ý tưởng** (tiêu đề, dòng đầu, lúc ghi, *Viết thành truyện*);
  - **Tư liệu** (ô thả tệp: Bìa truyện · Nhạc nền · Giọng mẫu);
  - ô “Ghi nhanh một ý tưởng truyện…” neo đáy.
- **Truyện:** chip trạng thái + thẻ, tên truyện, *Mở Workspace*; tab **Dàn ý · Nhân vật · Chi tiết cài cắm · Bản xuất**.
  - Dàn ý: chương mở ra có Tóm tắt · Sự kiện chính · **Mục tiêu chương** (= `plan_chapter`) · **Bắt buộc có** (= `contract`) · *Đọc chương* · *Góp ý cho chương này*.
  - Chi tiết cài cắm: nhãn *Đã gieo · Đẩy tiếp · Thu hồi*, không hiện mã tuyến.
  - Bản xuất: bảng Văn bản · Ebook · Sách nói, câu tóm tắt xuất tự động bằng lời, *Xuất Tập 1 ngay*, *Chọn chương, định dạng…*.
  - Ô “Dặn hướng cốt truyện…” neo đáy + *Bàn kỹ với Kiến trúc sư* (→ /cocreate).
- **Phải:**
  - **Đang áp dụng cho truyện** / **Mặc định của dự án**: Quy tắc viết · Văn phong · Duyệt từng chương · Người viết. Mỗi dòng có nhãn “riêng” và dẫn sang Tuỳ chỉnh đúng cấp. Thêm thanh chi phí truyện.
  - **Tình trạng**, từ `GET /api/projects/:p/validate`:
    - không có vấn đề: “Mọi thứ đã sẵn sàng” + *Kiểm tra lại*;
    - có vấn đề: danh sách bằng lời + **Sửa tự động** (`POST …/validate/fix`, spinner → toast) + câu “Chỉ thêm phần còn thiếu, không xoá hay sửa nội dung truyện”;
    - `<details>` “Chi tiết cho quản trị” (đường dẫn, mã, *Mở thư mục trên máy chủ*).

### Lời cho từng mã kiểm tra

| `code` | Lời hiển thị | Sửa tự động |
|---|---|---|
| `missing-project-json` | Dự án chưa có thông tin — Noverse sẽ tạo mới với tên “{tên thư mục}”. | ✓ |
| `invalid-project-json` | Thông tin dự án bị hỏng, Noverse không đọc được. Nhờ quản trị kiểm tra. | — |
| `missing-dir` | Thiếu mục {Ý tưởng / Tư liệu / Truyện / Quy tắc viết} — sẽ tạo mục trống. | ✓ |
| `missing-story-json` | Truyện “{tên}” chưa có thông tin trạng thái — sẽ tạo mới với trạng thái “Đang viết”. | ✓ |
| `invalid-story-json` | Thông tin của truyện “{tên}” bị hỏng. Nhờ quản trị kiểm tra. | — |
| `invalid-name` | Ẩn với người dùng (chỉ “Chi tiết cho quản trị”) — vẫn dùng được. | — |
| `legacy-rules` | Có quy tắc viết kiểu cũ — sẽ chuyển vào Quy tắc viết của dự án. | ✓ |
| `rules-symlink` | Quy tắc của truyện đang trỏ sang dự án — sẽ gỡ để tránh áp hai lần. | ✓ |

## 5. Tên công cụ → lời thường (Workspace, Nhật ký, điện thoại)

| Công cụ | Nhãn |
|---|---|
| `novel_context` | Đọc bối cảnh |
| `read_chapter` | Đọc chương |
| `draft_chapter` | Viết bản nháp |
| `edit_chapter` | Gọt giũa |
| `check_consistency` | Soát nhất quán |
| `commit_chapter` | Lưu chương |
| `plan_chapter` | Lập kế hoạch chương |
| `save_book` | Lưu thông tin truyện |
| `save_foundation[premise]` / `[outline]` | Lưu tiền đề / Lưu dàn ý |
| `save_review` | Lưu thẩm duyệt |
| `save_arc_summary` | Tóm tắt hồi |
| `audit_foundation` | Soát nền truyện |
| công cụ chưa có trong bảng | “Công cụ khác” + tên gốc chỉ trong phần chi tiết |

Hiển thị theo mẫu `{nhãn} · chương {n}`, chữ thường (không mono). Tham số và kết quả gốc chỉ hiện khi người dùng mở “chi tiết kỹ thuật”.

Trường của khối lưu chương:

| Trường | Nhãn |
|---|---|
| `hook_type` | Kiểu móc (crisis = khủng hoảng, cliffhanger = treo, reveal = hé lộ, question = câu hỏi) |
| `dominant_strand` | Tuyến chính (quest = nhiệm vụ, romance = tình cảm, mystery = bí ẩn) |
| `cast_intros` | Nhân vật mới |
| `foreshadow_updates` | Chi tiết cài cắm (plant = Đã gieo, advance = Đẩy tiếp, resolve = Thu hồi) |
| `key_events` | Sự kiện chính |
| `feedback` | Góp ý (null = chưa có) |

## 6. Đăng nhập & kết nối điện thoại — không bắt chạy dòng lệnh

Trước đây có 2 chỗ trên UI bảo người dùng mở cửa sổ dòng lệnh: quên mật khẩu (`ainovel-cli serve user passwd`) và điện thoại chưa vào được (`ainovel-cli serve --lan`). Cả hai đã được thay bằng thao tác trên giao diện.

| Tình huống | Giao diện mới | Artboard |
|---|---|---|
| Người dùng quên mật khẩu | “Nhờ quản trị viên đặt lại mật khẩu cho bạn. Họ sẽ gửi một mật khẩu tạm — lần đầu vào, bạn tự đặt mật khẩu mới.” | `Login.dc.html` › password › Quên mật khẩu? |
| Quản trị viên duy nhất quên mật khẩu | Nút **Dùng mã khôi phục** → ô mã (`XXXX-XXXX-XXXX-XXXX`) + mật khẩu mới → *Đặt lại & đăng nhập*. Mỗi mã dùng một lần; mọi thiết bị của tài khoản bị đăng xuất | như trên, tài khoản quản trị |
| Lấy mã khôi phục | Bước **Hoàn tất** của thiết lập lần đầu: thẻ “Mã khôi phục của quản trị” (*Chép* · *Tải tệp*, chỉ hiện một lần). Tạo mã mới ở *Tuỳ chỉnh › Nâng cao › Truy cập & khôi phục* | `Onboard.dc.html` › done; `Settings.dc.html` › adv |
| Điện thoại chưa vào được (máy chủ chỉ mở trên máy này) | “Điện thoại chưa vào được” + nút **Cho phép thiết bị cùng Wi-Fi** → “Đang mở kết nối cho Wi-Fi nhà… khoảng 5 giây” → mã QR hiện ra. Tắt lại ở *Tuỳ chỉnh › Nâng cao* | `Login.dc.html` › qrNoLan |
| Ngăn “Tệp sẽ ghi” khi thiết lập lần đầu | Chỉ quản trị thấy. Đã bỏ ghi chú dành cho dev (tên API, số GAP) khỏi UI | `Onboard.dc.html` |

Dòng lệnh vẫn dùng được cho ai chạy Noverse không có giao diện, nhưng chỉ ghi trong tài liệu (`HUONG_DAN_SU_DUNG.md`), không hiện trên UI. Ngoại lệ: trang kết quả OAuth khi đăng nhập **bắt đầu từ dòng lệnh** (`OAuth.dc.html`, `from=cli`). Ở đó người dùng vốn đang ở dòng lệnh, nên giữ gợi ý `ainovel-cli login`.

**GAP-33 — backend cần thêm:**

- `POST /api/recover {username, code, newPassword}`:
  - chỉ áp dụng cho tài khoản quản trị;
  - mã lưu dạng băm, dùng một lần;
  - thành công thì đặt mật khẩu, thu hồi mọi phiên, huỷ mã;
  - giới hạn 5 lần/giờ (429);
  - sai thì báo “Mã khôi phục không đúng hoặc đã dùng”.
- Mã khôi phục:
  - tạo ở bước Hoàn tất (`POST /api/me/recovery`, trả mã một lần);
  - tạo lại ở Nâng cao (`POST /api/me/recovery/rotate`);
  - `GET /api/me/recovery` → `{createdAt, used}`.
- `PUT /api/system/network {lan: boolean}`:
  - chỉ quản trị, và chỉ gọi được từ chính máy chủ (loopback, như `canReveal`);
  - mở/đóng cổng nghe trên mạng nội bộ mà không dừng worker;
  - ghi vào `~/.ainovel/serve.json` để lần khởi động sau giữ nguyên;
  - trả `links[]` cho mã QR.
  - Người không phải quản trị thấy: “Nhờ quản trị bật ‘Cho phép thiết bị cùng Wi-Fi’ trong Tuỳ chỉnh.”

## 7. Cập nhật 25/10 (brief design) <!-- brief-2510 -->

- **Cách viết (D2):** nhóm “Gói thể loại” đứng trước “Kiểu viết” (`Settings.writing=pack · packOpen · builtin`). Gói được chọn qua khoá `style` sẵn có; quy tắc người dùng ghi đè được [CẢNH BÁO] và mục không nhãn, không ghi đè [LỖI].
- **Kiểm tra 3 chương gần nhất (D6):** rail chỉ hiện chỗ lệch “Đo bằng máy theo gói …” với Bắt sửa / Cảnh báo / Đạt; bảng “Xem đủ số đo chương N” có 9 số đo kèm chuẩn (`metrics=numbers`).
- **Quy tắc viết — lỗi 500 khi hiểu quy tắc** (`rules=retry`): thử lại tối đa 3 lần trong ~20 giây (thẻ chờ viền cam, “Máy chủ đang bận, Noverse thử lại lần 2/3…”, nút “Lưu nguyên văn ngay” / “Huỷ”); hết lượt thì lưu nguyên văn, gắn “chưa chắc”, tự thử hiểu lại khi máy chủ ổn. Không chờ 2 phút.
- **Nâng cao:** nhóm “Phiên bản Noverse” (D1) và “Não truyện” với “Mở bằng Obsidian” ×2, không hiện đường dẫn (D12).
- **Dự án & truyện (`Outline`):** tab Dàn ý có thẻ “Khung truyện” và chế độ “Bản đồ giữ chân” (D5); tab Nhân vật có trang nhân vật với bảng xưng hô, lịch sử quan hệ, khoá/sửa/xác nhận (D7–D8); nút “Não truyện” mở màn `Brain` (D10); tab Bản xuất thêm cột “Bản đọc TTS” (D3).

## 8. Lưu trữ tệp — kho S3 (31/10, Gói AN) <!-- an-spec -->

Artboard: `Settings` · `section=storage` · `storage` (14 trạng thái) · `sprov`: r2 · aws · garage · other (nhà cung cấp ở form) · `sfail`: key · bucket · perm · net · clock · cors · reach (khi `storage=testfail`). Điện thoại: `MMore` · `state=storage` · `storage`: active · migrating · down · local.

Đối chiếu bản thật (31/10): `GET/PUT /api/system/storage`, `POST …/test`, `POST …/migration{,/pause,/resume,/cancel}` đã có trong `internal/entry/web/storage_metrics.go` + `internal/bootstrap/blob_*.go`. Bước kiểm tra, mã lỗi, dữ liệu chuyển kho trong thiết kế lấy đúng theo đó. Chỗ thiếu ghi GAP ở cuối mục.

### 8.1 Vị trí

- Tuỳ chỉnh › nhóm **Hệ thống · Quản trị** › **Lưu trữ tệp**, ngay dưới Thành viên. Chỉ quản trị thấy. Dòng phụ ở thanh trái tóm tắt: “Cloudflare R2 · 3,4 GB” / “Ổ máy chủ · 3,4 GB” / “Đang chuyển · 37%” / “Không kết nối được”. Chấm màu: xanh dương khi đang chuyển, vàng khi chuyển có lỗi, đỏ khi mất kết nối.
- Đầu app (chỉ quản trị): khi đang chuyển có **viên nhỏ cạnh nút Tuỳ chỉnh** — vòng tiến độ + “Chuyển kho tệp · 37%” (tạm dừng: xám; có lỗi: vàng “Chuyển kho: 3 tệp lỗi”). Khi mất kết nối: viên đỏ “Kho tệp lỗi” **và** một dòng đỏ dưới thanh trên cùng “Kho tệp không kết nối được — nghe audio, tải bản xuất và ảnh bìa đang lỗi” + *Xem Lưu trữ tệp*. Bấm đều mở mục này.

### 8.2 Trạng thái

| `storage` | Thấy gì |
|---|---|
| `filemode` | Máy chủ chạy chế độ thư mục (GET trả `mode: "file"`). Chỉ một thẻ thông tin: tệp nằm trong thư mục từng truyện; muốn dùng S3 thì người quản trị máy chủ chuyển sang chế độ cơ sở dữ liệu. Không nút. |
| `local` | Thẻ kho đang dùng “Ổ đĩa máy chủ” + Hoạt động tốt + *Kiểm tra kết nối*; thẻ mời **Chuyển sang kho S3** (*Chọn kho S3*); Dung lượng theo loại; Hoạt động từ lần khởi động (không có dòng Tạo link vì gửi qua máy chủ). |
| `pick` | Lưới 8 thẻ: Garage · Cloudflare R2 · Amazon S3 · Google Cloud Storage · Wasabi · Backblaze B2 · DigitalOcean Spaces · Khác, tương thích S3. Mỗi thẻ: chữ viết tắt đơn sắc, tên, một dòng gợi ý, “Gợi ý: gửi qua máy chủ / gửi trực tiếp”. Kho đang dùng có nhãn “Đang dùng”. Không có MinIO. |
| `form` | “Kết nối {nhà cung cấp}”: thẻ **Thông tin kho** (ô theo nhà cung cấp — 8.3), thẻ **Cách gửi tệp tới trình duyệt**, thẻ **Kiểm tra kết nối** (6 bước, chưa chạy), thanh lưu dưới đáy “Kiểm tra kết nối đạt rồi mới lưu được” + *Huỷ* · *Lưu…* (mờ). |
| `testing` | Đang chạy: 2 bước đạt kèm ms, bước 3 quay, còn lại chấm xám; nút thành “Đang kiểm tra…”. |
| `testok` | Đủ 6 bước xanh + ms; dải xanh “Đạt cả 6 bước · tổng 657 ms · trình duyệt tải thẳng từ kho”; thanh lưu “Kết nối đạt · lưu để dùng Cloudflare R2”, *Lưu…* sáng. |
| `testfail` | Bước lỗi đỏ “lỗi”, các bước sau “bỏ qua” (riêng *Xoá tệp thử* vẫn đạt nếu lỗi ở Tạo link ký / Trình duyệt — máy chủ vẫn chạy bước xoá). Khung đỏ: tiêu đề + lý do bằng lời + “Cách sửa”. Bảy lỗi ở 8.4. |
| `envlocked` | Thẻ kho đang dùng (Garage, Qua máy chủ) — mọi dòng có nhãn “do máy chủ đặt”; khung giải thích + *Chi tiết cho người quản trị máy chủ* (tên biến). Còn *Kiểm tra kết nối*; không *Đổi kho…*, không Kho cũ. |
| `nokey` | Form + khung vàng “Máy chủ chưa có khoá mã hoá nên chưa lưu được khoá bí mật” (giải thích ngắn; lệnh tạo khoá chỉ nằm trong *Chi tiết cho người quản trị máy chủ*). Vẫn kiểm tra được; chọn Trực tiếp thì bước Trình duyệt “chưa chạy được” (bản thật cần lưu tạm cấu hình để tạo link thử) → dải vàng “chưa đạt đủ bước”. *Lưu…* luôn mờ. |
| `migrate-confirm` | Hộp “Chuyển 1.248 tệp · 3,4 GB sang Cloudflare R2?” · “Từ ổ đĩa máy chủ · kết nối đã kiểm tra đạt” · 4 dòng: app vẫn chạy, tệp mới vẫn ghi kho cũ tới khi xong rồi chép nốt · mỗi tệp chép rồi đọc lại, còn lỗi thì chưa đổi · kho cũ không bị xoá · tạm dừng/huỷ lúc nào cũng được. *Huỷ* · *Bắt đầu chuyển*. |
| `migrating` | Thẻ xanh dương đầu mục: “Đang chuyển sang Cloudflare R2” + Đang chạy; “Bắt đầu lúc 14:02…”; hai thanh Số tệp (474 / 1.248 · 37%) và Dung lượng (1,3 / 3,4 GB); “Tốc độ trung bình 18 MB/giây · còn khoảng 2 phút”; “Đã chép và kiểm tra 474 · Lỗi 0”; *Tạm dừng / Tiếp tục* · *Huỷ chuyển* (hỏi lại tại chỗ: “App vẫn dùng ổ máy chủ, không mất tệp nào. 474 tệp đã chép sang Cloudflare R2 vẫn nằm ở đó”). Bên dưới vẫn là thẻ “Ổ đĩa máy chủ · Kho đang dùng”. |
| `migrate-errors` | Thẻ “Chuyển sang Cloudflare R2 · còn 3 tệp lỗi” + Có lỗi, thanh vàng 99%, bảng Loại tệp · Truyện · Lý do, “Còn tệp lỗi thì chưa đổi kho — app vẫn dùng ổ máy chủ, không mất gì.”, *Thử lại các tệp lỗi* · *Huỷ chuyển*. |
| `active` | Thẻ “Cloudflare R2 · Hoạt động tốt · Kiểm tra gần nhất lúc 14:31” + *Kiểm tra kết nối* · *Đổi kho…*; lưới: Tên kho, Vùng, Địa chỉ (chỉ tên miền), Cách gửi tệp, Môi trường (Thử nghiệm/Chạy thử/Chính thức), Khoá truy cập ••••7f3a, Cấu hình (lưu trong app · đã mã hoá / do máy chủ đặt). Dung lượng theo loại (6 loại + tổng), Bộ nhớ đệm trên máy chủ (1,2 / 5,0 GB), Hoạt động từ lần khởi động (Ghi/Đọc/Tạo link/Xoá: số lần, lỗi, dữ liệu, trung bình, chậm nhất; lỗi > 0 tô cam cả dòng), **Kho cũ (ổ máy chủ) vẫn giữ 3,4 GB** + cách dọn. |
| `down` | Khung đỏ đầu mục “Không kết nối được kho tệp Cloudflare R2” + lý do (`health.error`) + “Lần thử gần nhất 14:32 · 40 giây trước” + *Kiểm tra lại*; thẻ kho đổi nhãn “Không kết nối được”; dòng đỏ đầu app + viên đỏ. |

### 8.3 Ô theo nhà cung cấp

| Nhà cung cấp | Ô riêng | Ghi chú |
|---|---|---|
| Cloudflare R2 | Account ID | App dựng địa chỉ `<id>.r2.cloudflarestorage.com`, hiện ngay dưới ô. Vùng cố định `auto`, không hiện. |
| Amazon S3 | Vùng (danh sách) | |
| Wasabi · Backblaze B2 · DigitalOcean Spaces | Vùng (danh sách) | App dựng địa chỉ theo vùng. |
| Google Cloud Storage | — | Địa chỉ cố định. Khoá là khoá HMAC. |
| Garage | Địa chỉ máy chủ nội bộ (`http://garage:3900`), Vùng điền sẵn “garage” (sửa được, khớp `s3_region`) | Kiểu đường dẫn luôn bật (không hiện). Công tắc “Cho phép http (chỉ mạng nội bộ)” bật sẵn + cảnh báo vàng. Khối thu gọn “Lệnh Garage” có 3 lệnh + *Chép*. |
| Khác | Địa chỉ máy chủ, Vùng, công tắc Kiểu đường dẫn, công tắc Cho phép http + cảnh báo | |

Ô chung: Tên kho, Mã khoá truy cập, Khoá bí mật. Mỗi ô có một dòng hướng dẫn và *Lấy ở đâu?* — mở khung “Lấy thông tin ở {nhà cung cấp}” ngay đầu thẻ (4 bước, bước của ô vừa bấm được tô). Thiếu ô bắt buộc → nút kiểm tra mờ + “Điền đủ n ô còn trống rồi kiểm tra”. Khoá đã lưu (chọn lại đúng kho đang dùng ở *Đổi kho…*): Mã khoá hiện “••••7f3a · Đã lưu” + *Đổi khoá*; Khoá bí mật “•••••••• · Đã lưu · đã mã hoá”. *Đổi khoá* xoá trắng cả hai ô.

**Cách gửi tệp tới trình duyệt** — hai lựa chọn, mỗi cái một dòng ưu (xanh) và nhược (vàng), nhãn “Gợi ý” theo nhà cung cấp (Garage → Qua máy chủ; đám mây → Trực tiếp). Chọn Trực tiếp mà địa chỉ kho là nội bộ (http, tên không có dấu chấm, IP riêng — cùng luật `privateEndpoint` của máy chủ) → hiện ô **Địa chỉ kho cho trình duyệt** (`https://s3.aarverse.studio`) + cảnh báo vàng khi trống. Đổi bất kỳ ô nào/cách gửi → kết quả kiểm tra cũ bị xoá, phải kiểm tra lại.

### 8.4 Kiểm tra kết nối — bước và lỗi

Bước (đúng thứ tự máy chủ): Ghi thử (`write`) · Đọc lại (`read`) · Đọc một đoạn — dùng khi tua audio (`range`) · Tạo link ký (`presign`; khi Qua máy chủ hiện “không cần”) · **Trình duyệt tải được (CORS)** (`cors_probe` + trình duyệt tự tải link thử) *hoặc* **Máy chủ gửi được một đoạn cho trình duyệt** (`app_range`, khi Qua máy chủ) · Xoá tệp thử (`delete`).

| `sfail` | Bước | Mã máy chủ | Tiêu đề · cách sửa |
|---|---|---|---|
| key | Ghi thử | `wrong_key` | Kho từ chối khoá truy cập · chép lại cả hai ô; khoá bí mật chỉ hiện một lần lúc tạo |
| bucket | Ghi thử | `no_bucket` | Không có kho tên “noverse-media” · kiểm tra chính tả hoặc tạo kho |
| perm | Xoá tệp thử | `no_delete_permission` (tương tự `no_write_permission`, `no_read_permission` ở bước tương ứng) | Khoá chưa có quyền xoá tệp · đổi quyền thành Đọc và Ghi; tệp thử tự dọn ở lần đạt sau |
| net | Ghi thử | `unreachable_endpoint` | Không kết nối được địa chỉ kho · kiểm tra địa chỉ, cổng, tường lửa; với Garage/Khác thêm dòng “chung mạng Docker hoặc IP nội bộ” |
| clock | Ghi thử | `clock_skew` | Đồng hồ máy chủ bị lệch · bật đồng bộ giờ tự động |
| cors | Trình duyệt tải được | (trình duyệt) | Trình duyệt bị kho chặn (thiếu CORS) · đoạn cấu hình CORS **điền sẵn địa chỉ app** với 3 thẻ Cloudflare R2 / Amazon S3 / Garage (mỗi thẻ ghi dán vào đâu) + *Chép* · “chờ chừng một phút” · *Kiểm tra lại* · *Chuyển sang gửi qua máy chủ* |
| reach | Tạo link ký | `browser_endpoint_required` | Trình duyệt không tới được địa chỉ kho · *Điền địa chỉ cho trình duyệt* (cuộn tới ô) · *Chuyển sang gửi qua máy chủ* |

Trình duyệt tự phân biệt *thiếu CORS* và *không tới được*: tải `corsProbeUrl` (header `Range: bytes=0-1`); lỗi thì tải lại với `mode: "no-cors"` — được (phản hồi kín) ⇒ thiếu CORS; vẫn lỗi ⇒ địa chỉ cho trình duyệt không mở được (dùng tiêu đề “reach”, lý do “Trình duyệt của bạn không mở được {địa chỉ cho trình duyệt}”). Mã `content_mismatch` (“Nội dung đọc lại không khớp”) và `storage_error` dùng câu `reason` máy chủ trả. Kiểm tra quá dày → 429: “Bạn đã kiểm tra nhiều lần; hãy chờ một phút rồi thử lại.”

### 8.5 Lưu và chuyển kho

1. *Lưu…* chỉ sáng khi: đủ ô · kiểm tra đạt đủ bước · máy chủ có khoá mã hoá · không do máy chủ đặt.
2. Kho cũ còn tệp (lấy `total` của GET) → hộp xác nhận ghi rõ **số tệp và dung lượng**. Không còn tệp → lưu luôn, toast “Đã chuyển sang {nhà cung cấp}”.
3. *Bắt đầu chuyển* → `PUT` (máy chủ kiểm tra lại, trả `needsMigration` + số tệp/dung lượng chuẩn) → `POST …/migration`. *Huỷ* trong hộp **không gọi PUT**, nên không để lại kho chờ.
4. Trong lúc chuyển: hỏi lại GET mỗi 5 giây khi mục đang mở (30 giây cho viên ở đầu app). *Tạm dừng* = `/pause`, *Tiếp tục* và *Thử lại các tệp lỗi* = `/resume`, *Huỷ chuyển* = `/cancel` (hỏi lại tại chỗ).
5. Máy chủ tự đổi kho khi mọi tệp đã chép và kiểm tra → trạng thái `active`, có thẻ Kho cũ. Noverse không tự xoá kho cũ.
6. Mọi lỗi 409 “cấu hình lưu trữ đã thay đổi; vui lòng tải lại” → toast + tải lại mục.

### 8.6 Điện thoại (M4)

Nhóm mới **Quản trị** trên trang Thêm (chỉ quản trị): hàng **Lưu trữ tệp** — “Cloudflare R2 · 3,4 GB” / “Đang chuyển sang Cloudflare R2 · 37%” (chấm xanh) / “Không kết nối được kho” (chữ đỏ, chấm đỏ). Trang chỉ xem: khung đỏ khi mất kết nối · thẻ tiến độ khi đang chuyển (“Tạm dừng hoặc huỷ trên máy tính”) · Kho đang dùng (tên, trạng thái, Tên kho, Cách gửi tệp, Môi trường) · *Kiểm tra kết nối* (kết quả một dòng ngay dưới) · Dung lượng 6 loại + tổng · “Đổi kho trên máy tính.” Mất kết nối thì đầu app trên điện thoại dùng lại dải cảnh báo trên cùng (như dải mất mạng), chữ “Kho tệp không kết nối được”.

### 8.7 GAP (dữ liệu chưa có — thiết kế không bịa)

- **GAP-AN1** GET chưa trả kho chờ (`pending_store`): thẻ chuyển cần tên nhà cung cấp/kho mới (“sang Cloudflare R2”). Thêm `pending: {provider, bucket, endpointHost}` vào View hoặc vào `migration`.
- **GAP-AN2** `bytesPerSecond` là trung bình từ lúc bắt đầu, chưa có tốc độ hiện tại → thiết kế ghi “Tốc độ trung bình”; thời gian còn lại ước từ đó.
- **GAP-AN3** Không có số “đã kiểm tra” riêng: `copiedCount` chỉ đếm tệp đã chép **và** đã xác minh → hiện một số “Đã chép và kiểm tra”.
- **GAP-AN4** `MigrationError.reason` là một câu chung; FE dịch theo `code` (bảng ở `07-copy.md`). `story` là mã truyện → cần `storyTitle` (tệp dùng chung dự án: story rỗng → “Dùng chung của dự án”).
- **GAP-AN5** Sau khi chuyển xong không có thông tin kho cũ riêng → lấy `migration` gần nhất (`completed`: `totalCount`, `totalBytes`). Chưa có lệnh dọn kho cũ (`storage gc` chỉ dọn tệp mồ côi) → cần lệnh bảo trì dọn kho nguồn của một lần chuyển đã xong. Chưa có giờ xong chuyển → không hiện.
- **GAP-AN6** Đổi khoá hoặc cách gửi của **cùng** kho (cùng địa chỉ + tên kho) hiện cũng thành “kho mới” → `needsMigration` và chép lại toàn bộ (đều `matched`). Nên nhận ra cùng kho và đổi tại chỗ, không chuyển.
- **GAP-AN7** `Usage` xếp mọi khoá có tiền tố môi trường: ảnh đại diện (ảnh) rơi vào Bìa, bản sao lưu cơ sở dữ liệu rơi vào Bản thảo nhập. Cần loại avatar/backup khỏi 6 loại (hoặc thêm loại riêng, khi đó cập nhật thiết kế).
- **GAP-AN8** Kiểm tra trên kho đang dùng (*Kiểm tra kết nối* ở thẻ trạng thái, cả khi `envlocked`): chưa có route kiểm tra cấu hình hiện hành không cần gửi khoá — `POST …/test` nhận ứng viên; với khoá rỗng thì giữ khoá đã lưu (`nil credentials retain`) nên FE gửi lại cấu hình hiện hành không kèm khoá. Với `envlocked` cần cho phép test (hiện Candidate dựa trên kho active — kiểm lại).

## 9. Gói AP — Quy tắc gọn, chống giọng văn máy, nền móng truyện, Não dự án (31/10) <!-- ap-spec -->

Canvas bản 132. Artboard: `Settings`, `Outline` (`tab`, `found`, `aai`, `rules`, `view=project`), `MMore` (`state=rules` · `rulesAai`), `ProjBrain` (`lview`, `uni`, `sfilter`), `Members` (thanh bên, ô chọn Quyền vào ứng dụng), `SysBrain` (câu chữ).

### 9.1 Hai cấp, một chỗ sửa

| Cấp | Sửa ở đâu | Ghi chú |
|---|---|---|
| Truyện | `Outline` › truyện › tab **Cách viết** › Quy tắc | Được ưu tiên hơn dự án |
| Dự án | `Outline` › trang dự án › Cách viết của dự án › Quy tắc | Áp cho mọi truyện trong dự án, trừ chỗ truyện dặn khác |

Không còn cấp “Mọi truyện” ở máy tính, điện thoại, cột phải “Đang áp dụng”, “Noverse hiểu thế nào?”. Thao tác *Chuyển lên* chỉ còn truyện → dự án.

### 9.2 Danh sách quy tắc (máy tính)

- Ô thêm **một dòng** + nút *Thêm* (Enter hoặc ⌘/Ctrl+Enter). Dưới ô là hàng **Mẫu** (Không dùng từ… · Giới hạn từ lặp… · Xưng hô… · Nhân vật… · Giọng văn…) chèn câu mẫu vào ô.
- **Học từ góp ý của bạn** tách thành thẻ riêng ở trên (viền xanh lá nhạt, chỉ khi có). Mỗi dòng: câu, nguồn (“Từ góp ý của bạn ở chương 3 · hôm qua”), nhãn loại, *Giữ* · *Bỏ*. Ghi rõ “Đang áp dụng” — giữ là thành quy tắc của truyện, bỏ là Noverse không nhắc trợ lý nữa.
- Một thẻ danh sách, chia nhóm bằng tiêu đề nhỏ: **Của truyện này · n** rồi **Từ dự án “…” · n** (chữ nhạt, chỉ xem, *Sửa ở trang dự án ›*). Ở trang dự án chỉ có **Của dự án · n**.
- Mỗi quy tắc **một dòng** (cao ≥ 44 px): câu quy tắc + **một nhãn loại** ở cuối dòng. Bỏ các chip “Noverse hiểu” lặp lại câu. Nút Sửa · Chuyển lên dự án · Xoá hiện khi rê chuột hoặc focus (luôn giữ chỗ 90 px để nhãn thẳng cột).
- Nhãn loại: Cấm từ · Cấm ký tự · Giới hạn · Thể loại · Giọng văn · Cốt truyện · Nhân vật · Lời dặn. Lấy từ kết quả `understand` (`ban`, `max`, `genre`… ; câu tự do = Lời dặn).
- Quy tắc Noverse chưa chắc cách hiểu: khối vàng ngay dưới dòng, chọn một cách hiểu.
- Tiêu đề: “Quy tắc · n đang áp dụng” (truyện = của truyện + học từ góp ý + từ dự án).

### 9.3 Chống giọng văn máy — xem và chỉnh được

Thẻ thu gọn: “Chống giọng văn máy · Mặc định của Noverse · luôn bật” + “4 cụm từ không dùng · 16 từ dễ lặp có giới hạn mỗi chương [· n từ đã chỉnh cho truyện này]” + nút *Xem & chỉnh*. Mở ra (`aai=open`; `edited` có sẵn một từ đã chỉnh):

- **Từ dễ lặp** — lưới 2 cột, đủ 16 từ của `rules.SystemDefaults.FatigueWords`. Ngưỡng hiện **theo chương** = `per_1000 × 1,5` làm tròn (chương chuẩn 1.500 tiếng, `FatigueRefChapterTieng`): 0,67 → 1 lần · 1,33 → 2 · 2 → 3. Ghi chú “chương dài hơn khoảng 1.500 chữ được tăng theo tỉ lệ”.
- Mỗi từ có nút **−** (Siết) và **+** (Nới), 0–9 lần. Mỗi lần bấm **tạo hoặc sửa một quy tắc** của cấp đang xem (““như một” tối đa 4 lần mỗi chương.”, nhãn Giới hạn) — quy tắc đó hiện ngay trong danh sách trên; về đúng mức nền thì quy tắc tự xoá. Không có kho thiết lập riêng.
- Từ đã chỉnh: số màu tím + nút ↺ “Về mặc định · n lần” (hoặc “Về mức của dự án”). Nguồn khác hiện huy hiệu nhỏ: **góp ý** (đang theo quy tắc học từ góp ý), **dự án** (truyện đang theo quy tắc dự án).
- **vượt ở ch.N** khi chương gần nhất dùng nhiều hơn ngưỡng đang áp (từ `S/rules/violations`); nới ngưỡng thì huy hiệu tự mất.
- **Cụm từ không dùng** — 4 cụm của `ForbiddenPhrases`, có biểu tượng khoá, “gặp là Biên tập viên bắt sửa”. Không có nút tắt (backend chỉ cộng thêm danh sách cấm, xem GAP-AP3). *Thêm cụm của bạn* điền “Không dùng cụm “”” vào ô thêm quy tắc.
- Câu cuối: cách chỉnh tạo quy tắc + “Gói Nữ tần báo thù dùng đúng các ngưỡng mặc định” (gói có `fatigue_words` trong `goi.json` thì ghi các từ gói đổi và huy hiệu **gói**).
- Trang dự án dùng cùng thẻ; chỉnh ở đó tạo quy tắc của dự án.

### 9.4 Tab truyện: Thông tin · Tiền đề · Nhân vật · Thế giới · Dàn ý · Cài cắm · Cách viết · Bản xuất

Tab không còn biểu tượng (8 tab vừa cột giữa 1440 px). Mặc định vẫn mở **Dàn ý**. Tweak `found`: full · partial (tiền đề thiếu 2 mục, chưa có quy tắc thế giới).

**Thông tin** (`tab=info`)
- Dải **Nền móng**: Tiền đề (Đủ 10 mục / 8/10 · thiếu 2) · Nhân vật (6 · 2 chính) · Thế giới (n quy tắc / Chưa có) · Dàn ý (8/8 chương · 1 tập). Dấu tích xanh hoặc cảnh báo vàng; bấm là sang tab đó.
- **Thông tin truyện** — Tên truyện · Tên ngắn (thanh bên, tên tệp xuất; ≤ 32 ký tự) · Trạng thái (Ý tưởng · Đang viết · Tạm ngưng · Hoàn thành · Đã đăng) · Thẻ · Ý tưởng ban đầu: sửa tại chỗ (*Sửa* → *Xong*), ghi `PUT S/meta`. **Tóm tắt** (Kiến trúc sư viết, `meta/book.json`): chỉ *Nhờ sửa*.
- **Khung truyện** (chuyển từ đầu tab Dàn ý): Người nghe · Điểm khác · Hạt giống phần hai, sửa tại chỗ, `GET/PUT S/frame`.
- **Định hướng kết** (compass): Hướng kết · Tuyến còn mở (phải khép hết trước khi hoàn tất) · Quy mô dự kiến — chỉ *Nhờ sửa*.
- **10 trục của truyện** (giá trị của truyện trong sổ cái, *So với các truyện khác ›* sang Não dự án).
- **Viết bằng phiên bản** (chuyển từ Tuỳ chỉnh) — `builds` của `GET S/meta`.
- Menu ⋯ › *Đổi tên truyện* mở tab này ở ô Tên truyện.

**Tiền đề** (`tab=premise`) — các mục cấp 2 của `premise.md` theo `requiredPremiseHeadings` (tầng ngắn: 9 mục chung + Tính phù hợp với truyện ngắn), bảng hai cột tên mục · nội dung. Huy hiệu “Đủ 10 mục” / “Thiếu 2/10 mục”. Mục thiếu: dòng vàng “Chưa có. Thiếu mục này, Biên tập viên không đối chiếu được khi chấm chương.” + *Nhờ bổ sung*.

**Thế giới** (`tab=world`) — `world_rules.json` gom theo `category`: Xã hội · Công nghệ · Địa lý · Phép thuật · Khác, một thẻ, mỗi quy tắc hai dòng (quy tắc · “Giới hạn · {boundary}”). Câu cuối trỏ vũ trụ chung ở Não dự án. Rỗng (`found=partial`): thẻ viền đứt “Truyện này chưa có quy tắc thế giới” + *Nhờ Kiến trúc sư lập* (ẩn nút ở đầu trang).

**Sửa nội dung nền móng:** tiền đề, thế giới, tóm tắt, định hướng **không sửa tay** — mọi nút *Nhờ…* điền sẵn câu vào ô **Dặn hướng cốt truyện** ở đáy (vd. “Bổ sung mục “Vùng cấm sáng tác” cho tiền đề: ”) và đặt con trỏ ở cuối; người dùng viết tiếp rồi gửi cho Kiến trúc sư (`save_foundation`). Đây là quyết định thiết kế để giữ dàn ý và tiền đề khớp nhau.

### 9.5 Não dự án (`ProjBrain`)

- **Bỏ cột Ngày 7** (dữ liệu `day7` luôn `null`, chưa có nguồn lượt nghe). Khi có nguồn thì thêm lại thành cột tuỳ chọn.
- **Kiểm truyện mới**: dòng đầu “Kiểm truyện mới trước khi viết · so với 10 truyện gần nhất”, tên truyện + Brief; bên phải **kết luận** (Chưa đạt · n lỗi phải sửa, m cảnh báo / Đạt · m cảnh báo / Đạt cả 4 phép kiểm). Bốn phép kiểm thành bảng: tên phép (Tổ hợp trục 1 · 3 · 4 · 6 · Khác truyện liền trước · Giá trị dùng lại nhiều · Kiểu mở đầu) · mức · câu · gợi ý sửa.
- Thanh công cụ: **Bảng truyện · Tần suất theo trục** (`lview`), *Chỉ truyện có ô trùng truyện mới* (`lview=only`), đếm “10 truyện đã viết · kiểm theo 10 truyện gần nhất”.
- Bảng: tiêu đề cột dính khi cuộn; ô là nút — bấm một ô **soi** mọi truyện cùng giá trị ở trục đó (viền tím, dải “… dùng n lần trong 10 truyện gần nhất”, *Bỏ chọn*; `lview=pick`). Dòng truyện mới có số ×n cạnh mỗi giá trị.
- **Tần suất theo trục**: mỗi trục một dòng, các giá trị xếp theo số truyện đã dùng; giá trị của truyện mới viền tím; trục có mức tối đa (Nghề · Phản diện · Sân khấu) tô vàng giá trị đã đủ 2/10. Ghi chú luật dưới tên trục. Tính ở client từ `GET /ledger`.
- **Vũ trụ chung**: lọc Tất cả · Nhân vật · Địa điểm, *Chỉ mục cần xem · n*, ô lọc theo tên (client). `uni=unchecked` (`checks: null`): cột Kiểm “Chưa kiểm”, thẻ bên phải “Chưa có kết quả kiểm”.
- **Tìm trong dự án**: lọc Tất cả · Thực thể · Đoạn văn và theo truyện (chip, client, trên kết quả `?q=`).

### 9.6 Điện thoại (M4 › Cách viết)

Bỏ cấp Mọi truyện. Nhóm quy tắc “Của truyện này” / “Học từ góp ý · đang áp dụng” / “Từ dự án …”. Mỗi quy tắc một nhãn loại (bỏ chip lặp). Thẻ **Chống giọng văn máy** mở ra danh sách 16 từ, mỗi dòng nút − / + 36 px; chỉnh tạo quy tắc như máy tính (`state=rulesAai`).

### 9.7 GAP (dữ liệu chưa có — thiết kế không bịa)

| ID | Việc | Ghi chú |
|---|---|---|
| GAP-AP1 | **Chuyển quy tắc global** khi bỏ cấp Mọi truyện | Lần chạy đầu sau bản này: chép các tệp `~/.ainovel/rules/*.md` vào quy tắc của **từng dự án** (ghi nhật ký quản trị), rồi ngừng đọc `SourceGlobal` trong `rules` loader và từ chối `scope=global` ở `/api/rules/*` (400). Không làm cùng đợt thì trợ lý vẫn theo quy tắc người dùng không còn thấy — **bắt buộc đi chung với FE**. |
| GAP-AP2 | Ngưỡng từ dễ lặp hiệu lực theo cấp, có nguồn | `GET /api/rules/effective?scope=story|project` thêm `fatigueWords[{word, per1000, perChapter, source: default\|pack\|project\|story\|learned}]` và `forbiddenDefaults[]`. Nếu đã có trong snapshot thì ghi rõ khoá vào `06-data-contract.md`. |
| GAP-AP3 | Tắt một cụm từ cấm mặc định | Hiện `mergeForbidden` chỉ cộng thêm. UI **không** có nút tắt; làm sau nếu cần. |
| GAP-AP4 | Tiền đề theo mục | `GET S/premise` → `{sections[{heading, text}], missing[], updatedAt}` (dùng `parsePremiseSections` + `requiredPremiseHeadings`). Trước khi có: FE tự tách `snapshot.premise` theo `## `. |
| GAP-AP5 | Quy tắc thế giới | `GET S/world-rules` → `[{category, rule, boundary}]` (hiện chỉ qua `S/brain` entities và plan detail `kind=worldRules`). |
| GAP-AP6 | Định hướng kết đủ trường | Snapshot đã có `compassDirection`, `compassScale`; thêm `openThreads[]`. |
| GAP-AP7 | Tóm tắt sửa tay | `book.json.synopsis` chưa có PUT — UI chỉ *Nhờ sửa*. Không bắt buộc. |
| — | Não dự án | Không đổi API: bỏ Ngày 7, tần suất/lọc/soi đều tính ở client. |
