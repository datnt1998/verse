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

Tweak `section`: rules · writing · models · budget · info · connect · export · notify · app · adv; `scope`: story · project · global; `rules`: list · understood · adding · thinking · unsure · dirty.

### 2.1 Thanh “Áp dụng cho”

Có 3 viên chọn: **Truyện này** (tên truyện) · **Dự án** (tên dự án · số truyện) · **Mọi truyện** (toàn bộ thư viện). Dưới thanh là một câu giải thích kế thừa.

- Mỗi thiết lập có nhãn **“Riêng truyện này”** / “Riêng dự án” (tím) kèm nút *Theo dự án* để đặt lại, hoặc nhãn **“Theo dự án”** / “Theo ‘Mọi truyện’” (xám).
- Nguồn nhãn: `ConfigResponse.sources["roles.writer.model"]`.
- Trong menu trái, mục có chỉnh riêng ở cấp đang chọn có chấm tím.
- Thay đổi gom lại ở **thanh nổi đáy**: “N thay đổi cho truyện này · có hiệu lực từ chương 26” · *Huỷ* · **Áp dụng** (Quy tắc) / **Lưu** (mục khác).

### 2.2 Ma trận thiết lập

| Mục | Truyện | Dự án | Mọi truyện | Ai sửa | Ghi vào |
|---|---|---|---|---|---|
| Quy tắc viết | ✓ | ✓ | ✓ | có quyền ghi | `PUT /api/rules/file` (§3.3) |
| Văn phong (Chung · Ngôn tình · Hồi hộp · Huyền huyễn · Kiếm hiệp · Văn phong của bạn) | ✓ | ✓ | ✓ | có quyền ghi | `config.style` (`default · romance · suspense · fantasy · wuxia` + tên tệp tự tạo) |
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
| Mọi truyện | **Tuỳ chỉnh › Quy tắc chung** — mục này không còn thanh “Áp dụng cho”; đầu trang có thẻ chỉ dẫn “Quy tắc riêng của từng truyện và từng dự án nằm ở tab Dự án” | `Settings` · `section=rules` |

- Tab truyện dùng **cùng thành phần** với Tuỳ chỉnh (ô thêm + chip mẫu, thẻ quy tắc, *Noverse hiểu thế nào?*, học từ góp ý, mặc định chống giọng văn máy, *Khi nào có hiệu lực*). Nhóm kế thừa (dự án, Mọi truyện) hiện viền đứt, chỉ xem, có nút *Sửa ở trang dự án ›* / *Sửa ở Tuỳ chỉnh ›*.
- Cột phải khi mở tab: thẻ **Kiểm tra 3 chương gần nhất** (thay vị trí trong Tuỳ chỉnh). Dòng *Quy tắc viết* trong thẻ “Đang áp dụng cho truyện” mở tab này thay vì sang Tuỳ chỉnh.
- “Khi nào có hiệu lực” lấy theo truyện thật: quy tắc mới có hiệu lực từ chương kế tiếp sau chương đang viết; chương đã viết không bị sửa.
- **Điện thoại** (M4 › Quy tắc viết): mở thẳng cấp truyện, bỏ nút chia 3 cấp. Nhóm kế thừa chạm vào để sửa ở cấp đó; khi đang ở cấp dự án / Mọi truyện có nút *‹ Về quy tắc của truyện*.
- Các mục khác của Tuỳ chỉnh (Trợ lý AI, Ngân sách, và trong Cách viết: Duyệt từng chương, Trọng tài hỏi) vẫn giữ thanh “Áp dụng cho” 3 cấp. **Văn phong** đã chuyển theo cùng cách với quy tắc viết — xem *Cách viết — vị trí* bên dưới (Gói AL). <!-- al-note -->
- Số từ dễ lặp mặc định là **12** (đúng danh sách đang hiện), không phải 16.

**Cách viết — vị trí, chốt 29/10 (Gói AL)** <!-- cach-viet-vi-tri -->

Văn phong (gói thể loại, kiểu viết) và văn mẫu gắn với truyện, nên chọn ở Dự án & truyện, cạnh Quy tắc viết.

| Cấp | Sửa ở đâu (máy tính) | Artboard |
|---|---|---|
| Truyện | **Dự án › truyện › tab Cách viết** (sau Quy tắc viết): thanh kế thừa (“Riêng truyện này” + *Dùng theo dự án* / “Theo dự án …”), lưới Gói thể loại + Kiểu viết, thẻ “Gói … gồm”, **Văn phong của bạn · văn mẫu của truyện này** (danh sách + *Thêm văn mẫu*) | `Outline` · `tab=style` · `wstyle`: pack · builtin · inherit · samples |
| Dự án | **Trang dự án › Cách viết mặc định của dự án** (lối tắt ở thanh trái “Cách viết”): văn phong đang dùng, truyện nào chọn riêng, *Đổi* mở cùng lưới | `Outline` · `wstyle=proj` |
| Mọi truyện | **Tuỳ chỉnh › Cách viết › Văn phong chung** (chỉ hiện khi chọn “Mọi truyện”). Ở cấp truyện/dự án, Tuỳ chỉnh chỉ còn thẻ chỉ dẫn “Văn phong của truyện này: Gói … · riêng truyện này” + *Mở Cách viết của truyện* | `Settings` · `section=writing` · `scope` |

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
