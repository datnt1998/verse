# 21 · Prompt cho Dev — bổ sung toàn bộ cập nhật thiết kế (đợt 24–25/10)

> Dán nguyên tệp này cho Claude Code (hoặc dev) làm việc trong repo `ainovel-cli`. Tệp này là **danh sách việc**. Spec chi tiết nằm trong các tài liệu được dẫn; khi có mâu thuẫn, **artboard thắng**, rồi mới đến tài liệu.

---

## 0. Bối cảnh & quy tắc làm việc

**Nguồn sự thật:**
- Canvas Claude Design “Noverse Agentic Studio” (trang **Máy tính** + **Điện thoại**).
- Bản sao mã nguồn từng màn: `docs/design/v1/artboards/*.dc.html`. Mỗi artboard có tweak trong `data-props`; xem bảng tweak ở `artboards/README.md`. Mở từng trạng thái để lấy pixel, nhãn và logic trong `renderVals()`.
- Spec:

  | Tài liệu | Nội dung |
  |---|---|
  | `20-settings-rules.md` | **mới**: Tuỳ chỉnh, Quy tắc viết, Dự án, đổi tên công cụ, đăng nhập không dòng lệnh |
  | `22-brief-2510.md` | **mới (25/10)**: trả lời brief design D1–D12, câu hỏi treo §E, Gói L |
  | `18-mobile.md` | Điện thoại & máy tính bảng |
  | `13-commands.md`, `14-jobs-dock.md`, `17-kit.md` | Lệnh, việc nền, bộ thành phần |
  | `09-auth-onboard.md` | Đăng nhập, thiết lập lần đầu |
  | `06-data-contract.md` | Bảng GAP |
  | `11-api-contract.md`, `16-feedback-refresh.md`, `19-gap-closure.md` | Hợp đồng API đã chốt (của dev) |

**Bất biến** (`README.md`):
1. Không phá engine: `output/novel/` do engine quản lý.
2. Không bịa số: chỗ chưa có dữ liệu hiện `—` hoặc trạng thái rỗng.
3. Mọi chuỗi UI là tiếng Việt, gom vào `web/src/i18n/vi.ts`.
4. **Mới — bất biến #7 “Không lộ kỹ thuật”.** Người dùng chỉ thấy Ý tưởng · Tư liệu · Truyện · Bản xuất, tên dự án/truyện và lời thường. Những thứ sau chỉ được xuất hiện ở ngăn **Nâng cao** (quản trị) hoặc khối “chi tiết kỹ thuật” do người dùng tự mở:
   - đường dẫn, tên tệp (`*.json`, `.ainovel/…`);
   - khoá cấu hình (`api_key`, `advance_mode`…);
   - tên công cụ (`commit_chapter`…), mã lỗi;
   - lệnh `ainovel-cli`.

**Cách làm:**
- Làm theo thứ tự gói ở §1 (P0 trước).
- Mỗi gói:
  1. đọc spec + mở artboard;
  2. sửa frontend (`web/src`) và backend (`internal/…`) nếu có GAP;
  3. viết/điều chỉnh test;
  4. tự đối chiếu 1:1 với artboard ở mọi tweak liệt kê.
- Phần **“Đối chiếu lại”**: code đã có một phần (đã thấy `web/src/mobile`, `web/src/tablet`, `screens/notify`, `shell/jobs`, PWA/push…). Không viết lại; chỉ so với artboard mới nhất và vá chỗ lệch.
- Không đổi định dạng tệp người dùng đã có (`project.json`, `story.json`, `config.json`, `rules/*.md`); chỉ thêm.
- Xong mỗi gói: chạy `go test ./...` và test web. Ghi checklist nghiệm thu ở cuối gói vào PR/commit message (đánh dấu ✓/✗ kèm lý do).

---

## 1. Thứ tự & phạm vi

| Gói | Ưu tiên | Nội dung | Backend |
|---|---|---|---|
| A | P0 | Dọn chữ kỹ thuật toàn app | không |
| B | P0 | Đăng nhập & thiết lập lần đầu không cần dòng lệnh | GAP-33 |
| C | P0 | Màn **Tuỳ chỉnh** (thay Cài đặt) — 3 cấp áp dụng | dùng API config/rules sẵn có |
| D | P0 | **Quy tắc viết** (máy tính + điện thoại) | GAP-32 |
| E | P1 | Màn **Dự án & truyện** không còn cây thư mục | dùng `validate`/`fix` sẵn có |
| F | P1 | Điện thoại & máy tính bảng — đối chiếu bản v2 | GAP-30, GAP-31 |
| G | P2 | Lệnh, việc nền, bộ thành phần — đối chiếu | GAP-22…29 |
| H | P0 | **Bổ sung 25/10:** mã QR quá nhỏ + bước Điện thoại & người cùng viết | không |
| I | P0 | **Bổ sung 25/10:** chọn nơi lưu truyện (bỏ mặc định ngầm `~/Truyen`, không hiện “lib”) | GAP-34 |
| J | P0 | **Bổ sung 25/10:** Việt hoá & tối ưu UI trợ lý AI làm việc (tên trợ lý, công cụ, nhật ký, số & đơn vị) | không |
| K | P1 | **Bổ sung 25/10:** chuyển động cho công tắc, radio, hộp kiểm, nhóm chọn | không |
| L | P0–P1 | **Brief 25/10:** gói thể loại, bản đọc TTS, thẩm duyệt có §, Não truyện W1–W2 | GAP-35…40 |
| M | P0–P2 | **Bổ sung 25/10:** quản trị đầu tiên trên web, Não truyện điện thoại, Chất lượng truyện, Não dự án, Não hệ thống | GAP-41…46 |

---

## Gói A — Dọn chữ kỹ thuật toàn app (P0)

**Spec:** `20-settings-rules.md` §1, §5. **Artboard:** `Main`, `Activity`, `Reader`, `Dialogs` (state `newProject`), `NewStory`, `Onboard` (step `library`), `MLog`, mọi header.

**Việc:**

1. **Tên hiển thị thay cho slug.**
   - Header mọi màn hiện `project.json.name` (ví dụ “Kênh truyện audio”) và `story.json.title`.
   - **Bỏ** tiền tố số thư mục `01 ·` và mọi slug (`dit-dang-audio`, `01-tra-xanh-…`).
   - Danh sách truyện/dự án ở màn quản lý tài khoản (chọn truyện được giao) cũng dùng tên hiển thị.
2. **Thêm `web/src/lib/labels.ts`**, nguồn duy nhất để đổi tên công cụ và trường sang lời thường:
   - Bảng tên công cụ và bảng trường khối lưu chương theo `20-settings-rules.md` §5.
   - Mẫu hiển thị: `{nhãn} · chương {n}`, chữ thường (không dùng font mono).
   - Công cụ lạ → “Công cụ khác”. Tên gốc chỉ hiện trong khối chi tiết khi người dùng mở.
   - Dùng ở: Workspace (dòng tiến độ, khối lưu chương), Nhật ký máy tính, Nhật ký điện thoại (`screens/activity/mobile`).
3. **Khối lưu chương (Workspace):**
   - Thay `hook_type / dominant_strand / feedback / cast_intros / characters / foreshadow_updates / key_events` bằng Kiểu móc / Tuyến chính / Góp ý / Nhân vật mới / Nhân vật xuất hiện / Chi tiết cài cắm / Sự kiện chính, kèm giá trị đã dịch (crisis → khủng hoảng…).
   - Bỏ `replace_all`, `max_runes`, `source`, `from`, `to`, và id tuyến cài cắm (`E9`…).
   - “writer → commit_chapter(Chương 4)” → “Người viết · đang lưu chương 4”.
4. **Trình đọc:**
   - Thẩm duyệt: accept/polish/rewrite → **Đạt / Cần gọt / Viết lại**; critical·error·warning → nghiêm trọng · lỗi · lưu ý; met·partial·missed → đạt · một phần · chưa đạt.
   - Bỏ “· description / evidence / suggestion / summary / type”.
   - Xuất: hiện tên tệp thân thiện (“Trà Xanh Cướp Kịch Bản · Tập 1.epub”, nhãn “Tên tệp · lưu trong mục Bản xuất”). Không hiện đường dẫn tuyệt đối. Nút cạnh “Lần xuất gần nhất” là **Tải xuống**.
   - Chip “/sync nhận bản sửa tay” → “Nhận bản sửa tay”.
5. **Hộp thoại Dự án mới:**
   - Bỏ ô “Tên thư mục” (slug tự sinh, chỉ báo lỗi bằng lời khi trùng), bỏ “Ngôn ngữ”, bỏ cây tệp.
   - Thay bằng danh sách “Dự án mới có sẵn: Ý tưởng · Tư liệu · Truyện · Quy tắc viết (chép từ ‘Mọi truyện’) · Trợ lý AI (theo ‘Mọi truyện’)”.
   - Ô chọn: “Dùng chung trợ lý AI và quy tắc với ‘Mọi truyện’”.
6. **Truyện mới:** “kich_ban/02-{slug}/” → “Lưu vào dự án {tên}”.
7. **Thiết lập lần đầu — bước “Nơi lưu truyện”** (đổi tên từ “Thư viện & cấu trúc thư mục”):
   - Bỏ các ô đặt tên thư mục và dòng `output/novel`.
   - Hiện 4 thẻ Ý tưởng · Tư liệu · Truyện · Bản xuất, kèm câu “Noverse tự sắp xếp tệp, bạn không cần tạo hay đặt tên thư mục”.
   - Cột tóm tắt: “Mỗi dự án có: Ý tưởng · Tư liệu · Truyện · Bản xuất”.
   - Bước Ngôn ngữ: “…chọn riêng sau trong Tuỳ chỉnh” (không nhắc `project.json`).
8. **Nhật ký điện thoại:** chip công cụ hiện nhãn thường; nút “Chép chi tiết kỹ thuật”.
9. Đổi mọi “Cài đặt” trỏ tới màn cấu hình thành **“Tuỳ chỉnh”** (nút bánh răng, aria-label, liên kết).

**Nghiệm thu:**
- [x] Thêm test (ví dụ `web/src/lib/noTech.test.ts`): render các màn chính với dữ liệu mock, kiểm tra text hiển thị **không** khớp `/(~\/|\.ainovel|\.json\b|output\/|kich_ban|y-tuong|tai-san|xuat-ban|ainovel-cli|\b[a-z]+_[a-z_]+\b)/`. Ngoại lệ: Nâng cao, khối chi tiết kỹ thuật, trang OAuth `from=cli`.
- [x] Mọi nhãn công cụ đi qua `labels.ts`, không có chuỗi rời.

---

## Gói B — Đăng nhập & thiết lập lần đầu không cần dòng lệnh (P0) · GAP-33

**Spec:** `20-settings-rules.md` §6, `09-auth-onboard.md` (đã cập nhật). **Artboard:** `Login` (state `password` → Quên mật khẩu? → Dùng mã khôi phục; state `qrNoLan`), `LoginQR`, `Onboard` (step `done`, role admin/writer), `Settings` (section `adv` › Truy cập & khôi phục).

**Frontend:**
1. **Quên mật khẩu:**
   - Người dùng thường thấy: “Nhờ quản trị viên đặt lại mật khẩu cho bạn. Họ sẽ gửi một mật khẩu tạm — lần đầu vào, bạn tự đặt mật khẩu mới.”
   - Nếu tài khoản đang chọn là quản trị: thêm dòng “Bạn là quản trị viên duy nhất? Dùng mã khôi phục…” và nút **Dùng mã khôi phục**.
   - Nút mở form: ô mã (chữ hoa, gợi ý `XXXX-XXXX-XXXX-XXXX`), ô mật khẩu mới (≥ 8), nút **Đặt lại & đăng nhập**, ghi chú “Mỗi mã dùng một lần… Mọi thiết bị của tài khoản sẽ được đăng xuất.”
   - **Xoá** khối lệnh `ainovel-cli serve user passwd …` và nút “Sao chép lệnh” (cả trong `i18n/vi.ts`).
2. **QR — chưa có địa chỉ cho điện thoại** (`qrNoLan`):
   - Tiêu đề “Điện thoại chưa vào được”, mô tả “Noverse đang chỉ mở trên máy tính này…”.
   - Nút **Cho phép thiết bị cùng Wi-Fi** (quản trị) → trạng thái “Đang mở kết nối cho Wi-Fi nhà… khoảng 5 giây” → hiện mã QR với `links`.
   - Người không phải quản trị thấy: “Nhờ quản trị bật ‘Cho phép thiết bị cùng Wi-Fi’ trong Tuỳ chỉnh.”
   - **Xoá** lệnh `ainovel-cli serve --lan` (ở `screens/auth`, `screens/account/PairingDialog.tsx`, `i18n/vi.ts`).
3. **Thiết lập lần đầu:**
   - Bước Hoàn tất (quản trị): thẻ **Mã khôi phục của quản trị**, mã hiện một lần, có *Chép* và *Tải tệp* (.txt).
   - Ngăn “Tệp sẽ ghi (cho quản trị)” chỉ hiện với quản trị. **Xoá** các ghi chú dành cho dev (tên API, số GAP) khỏi UI.
   - Câu ở cột trái: “Đã dùng Noverse trước đây? Thiết lập cũ được giữ nguyên — chỉ cần kiểm tra lại.”
4. **Tuỳ chỉnh › Nâng cao › Truy cập & khôi phục:** Cho phép thiết bị cùng Wi-Fi (Bật/Tắt) · Truy cập khi ra ngoài (Tailscale, hướng dẫn) · Mã khôi phục quản trị (“Đã tạo dd/mm · chưa dùng” + **Tạo mã mới**).

**Backend (GAP-33):**
- `POST /api/me/recovery` (quản trị):
  - tạo mã 16 ký tự `[A-Z0-9]`, nhóm 4, trả về **một lần**;
  - lưu băm (cùng cơ chế băm mật khẩu của `platform/identity`);
  - gọi tự động ở bước Hoàn tất.
- `POST /api/me/recovery/rotate` · `GET /api/me/recovery` → `{createdAt, used}`.
- `POST /api/recover {username, code, newPassword}` (công khai, không cần phiên):
  - chỉ tài khoản quản trị; mã một lần;
  - đặt mật khẩu, thu hồi mọi phiên, cấp phiên mới (cookie như `/api/login`), huỷ mã;
  - giới hạn 5 lần/giờ/IP → 429;
  - lỗi: 401 “Mã khôi phục không đúng hoặc đã dùng”; 400 “Mật khẩu cần ít nhất 8 ký tự”.
- `PUT /api/system/network {lan: boolean}`:
  - quản trị **và** request từ loopback (cùng điều kiện `canReveal`);
  - mở/đóng listener trên mạng nội bộ **không dừng worker**;
  - ghi `~/.ainovel/serve.json` để lần khởi động sau giữ trạng thái;
  - trả `{lan, links[]}` cho QR.
- `GET /api/system` thêm `lan: boolean`.
- Cờ CLI `--lan` / `serve user passwd` vẫn giữ cho người chạy không giao diện; **chỉ nhắc trong `HUONG_DAN_SU_DUNG.md`**.

**Nghiệm thu:**
- [x] Không còn chuỗi `ainovel-cli` nào trong `web/src` (trừ trang OAuth `from=cli`).
- [x] Test Go cho `/api/recover`: đúng mã, sai mã, mã đã dùng, quá giới hạn, tài khoản không phải quản trị.
- [x] Test bật/tắt LAN: không làm mất phiên, không dừng worker đang viết.

**Tạo quản trị đầu tiên trên web:** đã có artboard `Setup` (A0, PO duyệt 25/10). Làm ở Gói M mục 1.

---

## Gói C — Màn “Tuỳ chỉnh” thay Cài đặt (P0)

**Spec:** `20-settings-rules.md` §2. **Artboard:** `Settings.dc.html`, mọi `section` × `scope` (story · project · global).

**Việc** (sửa `web/src/screens/settings/*`):

1. **Thanh “Áp dụng cho”** gồm 3 viên: Truyện này · Dự án · Mọi truyện, mỗi viên có tên và mô tả phụ, kèm câu giải thích kế thừa.
   - Gọi `GET /api/config?scope=&project=&story=`.
   - Nhãn **“Riêng truyện này / Riêng dự án”** (tím, có *Theo dự án* để đặt lại = xoá khoá ở cấp đó) hoặc **“Theo dự án / Theo ‘Mọi truyện’”**, lấy từ `ConfigResponse.sources`.
   - Mục có ghi đè ở cấp đang chọn: chấm tím trong menu trái.
2. **Menu trái:**
   - SÁNG TÁC: Quy tắc viết (số) · Cách viết · Trợ lý AI · Ngân sách · Thông tin truyện/dự án.
   - HỆ THỐNG · QUẢN TRỊ (vương miện, **ẩn với người không phải quản trị**): Kết nối AI · Xuất tự động · Báo động máy chủ · Giao diện · Nâng cao.
3. **Các mục (theo ma trận §2.2):**
   - **Cách viết**: Văn phong dạng thẻ (Chung, Ngôn tình, Hồi hộp, Huyền huyễn, Kiếm hiệp, Văn phong của bạn — từ `GET /api/config/presets` `styles`). Duyệt từng chương: cấp truyện gọi lệnh live `/review on|off`, cấp dự án/toàn cục ghi `advance_mode` “cho truyện mới”.
   - **Trợ lý AI**: 4 hàng (Mặc định, Kiến trúc sư, Người viết, Biên tập viên) gồm model (tên ngắn) + mức suy nghĩ (Tự động · Thấp · Vừa · Cao) + nhãn kế thừa. Ghi rõ Trọng tài dùng Mặc định. Model dự phòng dạng chip.
   - **Ngân sách**: ở cấp truyện có thẻ đã dùng/giới hạn + ước còn bao nhiêu chương; ô giới hạn ($, 0 = không giới hạn); thanh “Báo trước khi chạm” 50–95 %; “Khi hết”: Viết xong bước đang làm rồi dừng / Dừng ngay.
   - **Thông tin**: `story.json` (tên, trạng thái, ngày đăng, thẻ, ý tưởng ban đầu) hoặc `project.json` (tên, mô tả, ghi chú cho trợ lý).
   - **Kết nối AI**: thẻ nhà cung cấp (trạng thái đăng nhập/khoá dạng `••••abcd`, *Thử kết nối*, *Đăng xuất/Đổi khoá*). Không hiện `base_url`, `stream_idle_timeout`, `json_schema` ở đây.
   - **Xuất tự động**: công tắc theo tập/truyện/chương, định dạng, khi đã có bản cũ (Thay/Giữ cả hai), ví dụ tên tệp bằng lời.
   - **Báo động máy chủ**: công tắc theo sự kiện, viết bằng lời.
   - **Nâng cao**: các nhóm thu gọn — Vị trí lưu trên máy chủ (Mở trong Finder / Sửa dạng văn bản) · Tên thư mục & tên tệp xuất · Tinh chỉnh model (cửa sổ ngữ cảnh, ngắt khi im lặng, model khi nhập truyện) · **Truy cập & khôi phục** (gói B) · Lệnh chạy khi có báo động. Toàn bộ mục “Thư viện & cấu trúc thư mục” cũ chuyển vào đây.
4. **Thanh nổi đáy** khi có thay đổi: “N thay đổi cho {cấp} · có hiệu lực từ chương X” · *Huỷ* · **Áp dụng/Lưu**. Ghi bằng `PUT` kèm `If-Match` (theo `16-feedback-refresh.md`); 409 → báo và tải lại.
5. Cấp dự án/truyện **không** hiện `api_key`, `notify` (server từ chối 400).

**Nghiệm thu:**
- [ ] Mọi tweak của `Settings.dc.html` khớp pixel/nhãn. *(25/10: quản trị 1440 đạt; chưa chụp góc nhìn người viết — xem 19 §E)*
- [x] Đổi model Người viết ở cấp truyện → nhãn “Riêng truyện này”; *Theo dự án* xoá khoá và nhãn trở về “Theo dự án”.
- [x] Người viết (không phải quản trị) không thấy nhóm Hệ thống.

---

## Gói D — Quy tắc viết (P0) · GAP-32

**Spec:** `20-settings-rules.md` §3. **Artboard:** `Settings.dc.html` section `rules` với `rules` = list · understood · adding · thinking · unsure · dirty, và cả 3 `scope`; điện thoại `MMore.dc.html` state `rules`, `rulesAdd`, `rulesThinking`.

**Frontend:**
1. **Ô thêm quy tắc:**
   - Chip mẫu: Không dùng từ… · Giới hạn từ lặp… · Xưng hô… · Nhân vật… · Giọng văn…
   - Thêm bằng nút hoặc ⌘/Ctrl+Enter → thẻ “Noverse đang đọc quy tắc…” (chữ lấp lánh) → thẻ mới loé tím.
2. **Thẻ quy tắc:**
   - Nguyên văn + **chip “Noverse hiểu là”**: Không dùng “…” (đỏ), Tối đa “…” · N lần/chương (cam), Thể loại (xanh), Lời dặn (tím).
   - Rê chuột hiện *Sửa · Chuyển lên dự án / “Mọi truyện” · Xoá*. Điện thoại: chạm → action sheet.
   - `uncertain` → khung cam kèm 2–3 cách hiểu dạng nút + *Giữ như lời dặn*.
3. **Nhóm theo nguồn:** Riêng truyện này → Học từ góp ý của bạn (orb Trọng tài, “Từ góp ý ở chương N”, *Giữ làm quy tắc của truyện* / *Bỏ*) → Từ dự án (viền đứt, *Sửa ở cấp dự án ›*) → Từ “Mọi truyện” → Mặc định của Noverse (luôn bật, mở xem danh sách, kèm gợi ý cách nới).
4. **“Noverse hiểu thế nào?”**: bảng tổng hợp hiệu lực (Thể loại / Không dùng / Giới hạn lặp / Lời dặn).
5. **Cột phải** (cấp truyện):
   - Kiểm tra 3 chương gần nhất.
   - Khi nào có hiệu lực: “từ chương {n+1}; chương đang viết dở giữ luật cũ; chương đã viết không bị sửa”.
   - Ô chọn *Kiểm tra lại các chương đã viết* (mặc định bật).
   - Mẹo viết quy tắc.
6. **Áp dụng** nằm ở thanh nổi đáy (gói C). Điện thoại: thanh Áp dụng nổi trên thanh tab; khi gõ, thanh tab ẩn.
7. **Lưu trữ:**
   - Mỗi cấp, UI quản lý **một** tệp `.ainovel/rules/quy-tac.md`, mỗi quy tắc một dòng `- …`, theo thứ tự hiển thị (`PUT /api/rules/file`).
   - Tệp `.md` khác chỉ liệt kê ở Nâng cao › Thư mục quy tắc (sửa dạng văn bản).

**Backend (GAP-32, `internal/rules`, `internal/userrules`, `internal/host`):**
1. `POST /api/rules/understand {text}` → `{structured, preferences, uncertain[]}`:
   - gọi `Normalizer.Normalize`, không lưu;
   - hạn 2 phút, huỷ được, cùng quy ước lỗi tiếng Việt.
2. Lưu **ứng viên runtime riêng**: hiện `AddRuntimeRule` chỉ lưu bản snapshot đã trộn.
   - Thêm `store.UserRules.Runtime` = danh sách `{id, text, chapter, createdAt, candidate}`.
   - Chuyển mã cũ: snapshot không có danh sách runtime thì coi `Sources` có `runtime_update` là một mục.
3. `GET S/rules/effective` → `{snapshot, items: {text, scope, chips, uncertain}[], runtime[]}`.
4. `POST S/rules/apply {recheck?}`:
   - Dựng lại snapshot = `SystemDefaults` + chuẩn hoá các tệp (cache theo hash nội dung để khỏi gọi model lại) + `startup_prompt` gốc + **các ứng viên runtime**.
   - Áp từ chương kế tiếp. Chương đang viết dở giữ snapshot cũ: đọc snapshot lúc bắt đầu chương.
   - Trả `{snapshot, effectiveFrom}`, phát `story.changed {parts:['rules']}`.
   - `recheck=true` → tạo job `rules.recheck` (SSE `job.*`): chạy `rules.Checker` trên các chương đã lưu, ghi vi phạm, **không sửa** chương.
5. `POST S/rules/runtime/:id/keep` (ghi thêm dòng vào `quy-tac.md` của truyện, bỏ khỏi runtime) · `DELETE S/rules/runtime/:id`.
6. Lưu `violations` của `commit_chapter` theo chương (cạnh thẩm duyệt) + `GET S/rules/violations?last=3`.
7. **Sửa trộn:** `BuildSnapshot` đang **thay cả danh sách** `forbidden_phrases` / `forbidden_chars` khi cấp sau có giá trị. Đổi sang **hợp** (union, bỏ trùng); `fatigue_words` giữ cách gộp hiện tại (cấp gần thắng theo từ). Cập nhật test `snapshot_test.go`.

**Nghiệm thu:**
- [x] Thêm “Không dùng từ ‘thở dài’ quá 2 lần mỗi chương” → chip “Tối đa ‘thở dài’ · 2 lần/chương”. Áp dụng → chương kế tiếp có kiểm tra máy; chương đang viết không đổi luật.
- [x] Luật học từ chat hiện trong nhóm riêng; *Giữ* chuyển vào tệp truyện; *Bỏ* làm nó biến khỏi snapshot ở lần áp dụng sau.
- [x] Thêm một từ cấm ở cấp truyện **không** làm mất 4 cụm từ mặc định.
- [ ] Chuẩn hoá lỗi → thẻ vẫn lưu, hiện như “Lời dặn” (trạng thái `degraded`), không chặn việc viết. *(25/10: đạt với lỗi 401; lỗi 500 chờ tới 2 phút do thử lại — xem 19 §E)*

---

## Gói E — Màn Dự án & truyện không cây thư mục (P1)

**Spec:** `20-settings-rules.md` §4. **Artboard:** `Outline.dc.html` — `view` story/project × `tab` outline/cast/fs/export × `health` ok/issues.

**Việc** (`web/src/screens/project/*`):
1. **Xoá** `FolderPanel` và cột “Thư mục / Kiểm tra chuẩn / đúng chuẩn · engine quản lý · tùy chọn”, cùng mọi đường dẫn trong `ProjectOverview`, `StoryView`, `CandidateProject`.
2. **Cột trái:**
   - nút dự án (tên + số truyện, menu đổi / Dự án mới);
   - Tổng quan · Ý tưởng (n) · Tư liệu (n);
   - TRUYỆN → Tập → chương (✓/orb); Tập 2 khoá; Truyện mới;
   - viên tình trạng ở đáy.
3. **Tổng quan dự án:**
   - 4 số liệu;
   - thẻ truyện (trạng thái, thanh gradient, tóm tắt số liệu);
   - **Ý tưởng** (tiêu đề, dòng đầu, lúc ghi, *Viết thành truyện* → Truyện mới điền sẵn);
   - **Tư liệu** (ô thả tệp Bìa truyện / Nhạc nền / Giọng mẫu);
   - ô “Ghi nhanh một ý tưởng truyện…” neo đáy (`POST /api/projects/:p/ideas`).
4. **Truyện:**
   - tab **Dàn ý · Nhân vật · Chi tiết cài cắm · Bản xuất**;
   - Dàn ý mở chương: Tóm tắt · Sự kiện chính · **Mục tiêu chương** (`plan`) · **Bắt buộc có** (`contract`) · *Đọc chương* · *Góp ý cho chương này*;
   - Chi tiết cài cắm dùng nhãn Đã gieo / Đẩy tiếp / Thu hồi, không hiện id;
   - Bản xuất: bảng Văn bản · Ebook · Sách nói + câu tóm tắt xuất tự động bằng lời + *Xuất Tập N ngay* + *Chọn chương, định dạng…*;
   - ô “Dặn hướng cốt truyện…” + *Bàn kỹ với Kiến trúc sư*.
5. **Cột phải:**
   - **Đang áp dụng cho truyện / Mặc định của dự án**: Quy tắc viết · Văn phong · Duyệt từng chương · Người viết (nhãn “riêng”, dẫn sang Tuỳ chỉnh đúng cấp) + thanh chi phí.
   - **Tình trạng** từ `GET /api/projects/:p/validate`:
     - ok → “Mọi thứ đã sẵn sàng” + *Kiểm tra lại*;
     - có vấn đề → danh sách lời thường theo bảng mã §4 + **Sửa tự động** (`POST …/validate/fix`) + “Chỉ thêm phần còn thiếu, không xoá hay sửa nội dung truyện”;
     - `<details>` “Chi tiết cho quản trị” (đường dẫn, mã, Mở thư mục — chỉ khi `canReveal`).
   - Mã `invalid-name` **không** hiện với người dùng.

**Nghiệm thu:**
- [x] Không còn `y-tuong/`, `kich_ban/`, `*.json`, `output/novel` trên màn Dự án (test gói A).
- [x] Mỗi mã `validate` có câu tiếng Việt đúng bảng; `fixable=false` thì không hiện trong nút sửa.

---

## Gói F — Điện thoại & máy tính bảng (P1) — đối chiếu bản v2 · GAP-30, GAP-31

**Spec:** `18-mobile.md` (giữ nguyên ghi chú triển khai dev đã thêm). **Artboard trang Điện thoại:** `MWork`, `MReader`, `MAudio`, `MMore`, `MCmd`, `MLog`, `MNotify`, `MTablet`, `Phone`.

Code đã có `web/src/mobile`, `web/src/tablet`, `screens/notify`, `pwa.ts`, `outbox.ts`, push. **Đối chiếu từng tweak**, vá chỗ lệch:

1. **Thanh tab nổi:** viên nang kính 16/20/64, bo 32; chỉ báo trượt có độ nảy; chế độ `mini` khi cuộn xuống và `hide` khi bàn phím mở / đọc tập trung / phiên hết hạn / màn con; các phần nổi đặt ở `bottom: 92px`.
2. **M1 Viết:**
   - trạng thái idle · paused · done · error · empty · offline (băng đếm ngược, “Chờ gửi”) · expired (sheet đăng nhập không đóng được, giữ nội dung đang gõ) · typing (`visualViewport`);
   - chip lệnh thu lại khi cuộn;
   - tiêu đề lệnh ngắn (`SHORT`);
   - chuông → M7.
3. **M2 Đọc:** đọc tập trung; thẻ chương tiếp + vuốt ngang đổi chương; sheet góp ý nằm trên bàn phím; nút ⋯ ở mỗi chương (Đọc · Nghe · Nhờ viết lại… · Xuất riêng chương này · Chép đường dẫn tệp — **mục cuối chỉ cho quản trị**, theo gói A).
4. **M4 Thêm:**
   - **mới**: dòng Quy tắc viết và Tuỳ chỉnh truyện (gói C/D, `rules`, `storyset`); công tắc *Duyệt từng chương* chuyển vào Tuỳ chỉnh truyện;
   - đổi tài khoản theo **một phiên** (chọn tài khoản → đăng nhập bằng mật khẩu/QR → tài khoản cũ bị đăng xuất; danh sách từ `localStorage["ainovel.accounts"]`);
   - đổi mật khẩu (luôn thu hồi phiên khác) / mật khẩu tạm lần đầu (có *Để sau*, cần ô “Mật khẩu tạm”);
   - hướng dẫn lệnh (list + chi tiết, dùng chung dữ liệu với `Commands`);
   - tên dự án hiển thị, không slug.
5. **M5 Kết quả lệnh:** /cocreate (chat + thẻ đề xuất + Chốt), /import (chia chương, cảnh báo, Chia lại… với `--guide=`, tiến trình), /diag, /sync (`--check` + khác biệt + mâu thuẫn), /reopen (+ bị chặn).
6. **M6 Nhật ký:**
   - bộ lọc agent / Lỗi / Của bạn, tìm có tô sáng, tiêu đề chương dính;
   - công cụ hiện nhãn thường (gói A), tham số chỉ trong khối mở;
   - “N sự kiện mới”.
   - **GAP-31**: lịch sử có lọc `?chapter=&agent=&level=error&q=&before=` + chi tiết công cụ `{tool, args, result, durationMs, status, retries[]}`. Kiểm tra `11-api-contract.md` đã có danh sách sự kiện chưa; nếu có thì chỉ thêm tham số lọc.
7. **M7 Thông báo:**
   - hộp thư, màn xin quyền trước hộp thoại hệ thống, bị chặn, cài đặt (loại, gộp chương xong, giờ yên lặng, thiết bị, gửi thử), màn khoá có thao tác nhanh;
   - lời nhắc iPhone cần “Thêm vào Màn hình chính”;
   - **GAP-30**: đối chiếu `api/push` đã có với bảng trong `06-data-contract.md` (prefs, hộp thư, payload `{type, story, chapter?, job?, title, body, url, actions[]}`, liên kết sâu §M7).
8. **T1 Máy tính bảng (641–1099 px):** thanh tab nổi giữa thanh trên; Viết + Đọc hai cột; popover lệnh neo ô chat; thanh duyệt ở cột phải; Nghe/Thêm dùng bố cục điện thoại căn giữa 560 px.

**Nghiệm thu:**
- [ ] Mỗi tweak của 9 artboard điện thoại/máy tính bảng: chụp màn 390×844 / 834×1194 so cạnh artboard, không lệch bố cục, không chữ xuống dòng sai. *(25/10: phần lớn đạt; trạng thái còn thiếu liệt kê ở 19 §E)*

---

## Gói G — Lệnh, việc nền, bộ thành phần (P2) — đối chiếu · GAP-22…29

**Spec & artboard:**
- `13-commands.md`: `Slash`, `Commands`;
- `14-jobs-dock.md`: `Jobs`, `CmdResults`;
- `17-kit.md`: `Kit`.

Đã triển khai phần lớn (`web/src/commands`, `shell/jobs`, `screens/cmdresults`). Việc còn lại:
- Đối chiếu tweak mới nhất.
- Áp gói A vào các màn này: header tên dự án, “Tải hồ sơ văn phong” thay “Lưu hồ sơ ra tệp .json”.
- Kiểm tra từng GAP-22…29 đã đóng chưa (`19-gap-closure.md`). GAP nào chưa đóng thì làm theo `06-data-contract.md`.

---

---

## Gói H — Mã QR & bước “Điện thoại & người cùng viết” (bổ sung 25/10 · P0)

**Spec:** `17-kit.md` › *QrCode*. **Artboard:** `Onboard.dc.html` step `mobile` (role admin / writer, bấm nút phóng to), `Login.dc.html` state `qr` · `qrExpired` · `qrNoLan`.

1. **Sửa mã QR quá nhỏ:**
   - Bỏ padding CSS trên ô trắng; để thư viện vẽ lề đúng 4 ô (`margin: 4`).
   - Kích thước ≥ 232 px ở Thiết lập lần đầu, 264–300 px trong hộp thoại; cỡ ô là số nguyên px.
   - Mức sửa lỗi M; URL ghép ngắn.
2. **Không đặt gì đè lên mã.** Nút **Phóng to** nằm cạnh đồng hồ đếm ngược → lớp phủ mã 464 px.
3. **Thẻ QR theo artboard:** tiêu đề + viên đếm ngược + nút phóng to · mã · tab Ở nhà / Ra ngoài · 2 bước hướng dẫn · “Đang chờ quét…” + **Chép đường dẫn**.
4. **Thẻ Mời người cùng viết:** nhãn riêng cho từng ô; bỏ font mono và chữ `mat-khau-tam:`; dropdown “Được viết” dùng tên truyện/dự án (bỏ `demo-audio · tất cả truyện`), mũi tên dạng chevron; có trạng thái đã tạo + chép lời mời.
5. **Câu dẫn** ngắn, không để một chữ rơi xuống dòng cuối.

**Nghiệm thu:**
- [x] Đo bằng DevTools: phần mã (không tính lề) ≥ 200 px.
- [ ] Quét được bằng iPhone và Android ở khoảng cách 40 cm từ màn hình laptop. *(cần thiết bị thật)*
- [x] Không có phần tử nào nằm trong vùng mã.

---

## Gói I — Chọn nơi lưu truyện (bổ sung 25/10 · P0) · GAP-34

**Spec:** `09-auth-onboard.md` §10. **Artboard:** `Onboard.dc.html` step `library` (tweak `lib`: existing · fresh · picking · browse · picked · readonly). `Settings.dc.html` section `adv`, dòng **Nơi lưu truyện**.

1. **Bỏ việc mặc định ngầm `~/Truyen`.** Bước Thư viện là nơi chọn thật:
   - Thư mục đang dùng (nếu có dự án).
   - Gợi ý “Tài liệu › Truyện Noverse”.
   - **Chọn thư mục khác…**
   - Tiếp tục → `PUT /api/library {root}` (đã có, `switchLibraryRoot`).
2. **Không hiện tên thư mục trần** (`lib`, `Truyen`) hay đường dẫn tuyệt đối. Luôn hiện “Nơi › Thư mục” (Thư mục nhà, Tài liệu, Màn hình, iCloud Drive, ổ ngoài). Bảng ở §10.2.
3. **Chọn thư mục khác:**
   - Trên máy chủ: `POST /api/system/pick-folder` (đã có).
   - Từ xa, hoặc khi `501` / `403`: hộp duyệt thư mục trong app (GAP-34) — chỉ thư mục, có huy hiệu `ainovel` / `chỉ đọc`, nút Thư mục mới, nền đặc.
4. **Hộp kiểm tra** dưới danh sách: tìm thấy thư viện / trống / có tệp khác / chỉ đọc. Khi chỉ đọc: khoá Tiếp tục.
5. **Tóm tắt Hoàn tất:** “Lưu ở: {Nơi › Thư mục}”.
6. **Tuỳ chỉnh › Nâng cao:**
   - Đổi “Thư mục thư viện ~/Truyen” thành **Nơi lưu truyện · Đổi thư mục…** (cùng bộ chọn) + **Mở nơi lưu truyện**.
   - Báo rõ truyện cũ không bị di chuyển; `409` khi đang có truyện chạy.

**Nghiệm thu:**
- [x] Cài mới không có `~/Truyen`: bước Thư viện gợi ý “Tài liệu › Truyện Noverse”, tạo đúng thư mục đó khi Tiếp tục. *(`TestResolveServeRootNoImplicitDefault`, `TestUnconfiguredLibraryNoPanicAnd409` — mock đo: gợi ý “Tài liệu › Truyện Noverse” có thẻ Gợi ý)*
- [x] Chọn thư mục iCloud/ổ ngoài từ điện thoại được (không cần hộp chọn của máy). *(`folderPicker.test.tsx`: canReveal=false → bộ duyệt trong app; mock đo: chọn iCloud Drive › Truyện của Dat thành công)*
- [x] Không chỗ nào trên UI hiện “lib”, “~/Truyen” hay `/Users/...`. Đường dẫn đầy đủ chỉ có trong tooltip quản trị. *(grep toàn bộ `rootPath`/đường dẫn thô; mock đo: mọi ô hiện “Nơi › Thư mục”, path chỉ ở `title` của hàng Nâng cao)*
- [x] Thư mục chỉ đọc bị chặn với lời giải thích; `serve.json` chỉ ghi khi `PUT` thành công. *(`TestLibraryInspectEndpoint` readonly · `TestServeJSONWrittenOnlyOnSuccessfulPut` · `blocked` khoá Tiếp tục trong `libraryChoice.ts`)*

---

## Gói J — Việt hoá & tối ưu UI trợ lý AI làm việc (bổ sung 25/10 · P0)

**Spec:** `07-copy.md` › *Bảng từ: trợ lý AI & công cụ*. **Artboard:**
- `Main.dc.html`: phase rewrite · gate · done, tab Trợ lý · Chi phí.
- `Activity.dc.html`.
- `Jobs.dc.html`.
- `CmdResults.dc.html`, `Slash.dc.html`.
- `MLog.dc.html`: state tool · errors, bấm vào thẻ công cụ.
- `Phone.dc.html` home, `NewStory.dc.html`, `Onboard.dc.html` step model.

1. **Một hàm dịch dùng chung** (vd `web/src/lib/ai-labels.ts`):
   - `agentLabel(role)` và `toolLabel(name)`, `toolIcon(name)`.
   - `formatDuration(ms)`, `formatTokens(n)` (dạng dài và dạng gọn), `formatNumber` theo vi-VN.
   - Mọi màn gọi hàm này; không tự viết chuỗi.
2. **Tên trợ lý:** Kiến trúc sư · Người viết · Biên tập viên · Trọng tài · Hệ thống. Viết thường có dấu, không IN HOA, không giãn chữ. Tab “Agent” → **Trợ lý**.
3. **Dòng nhật ký:**
   - Dòng giao việc: “{Trợ lý} · {nhiệm vụ}”, bỏ ngoặc `（）`.
   - Dòng công cụ: icon theo loại công cụ + nhãn tiếng Việt, **font sans** (font mono đang làm vỡ dấu).
   - Thời lượng dạng “0,1 giây / 1 phút 39 giây”.
4. **Thẻ công cụ trong Luồng viết:**
   - Tiêu đề dùng cùng nhãn với nhật ký: Đọc bối cảnh, Gọt giũa, Đọc chương, Soát nhất quán, Lưu chương. Bỏ “Truy vấn ngữ cảnh”, “Kiểm tra nhất quán”, “Nộp chương”.
   - Icon theo loại công cụ.
5. **Chi tiết công cụ trên điện thoại** (MLog):
   - Bảng “nhãn · giá trị” + một dòng kết quả (xanh = xong, đỏ = lỗi). Lỗi dịch theo bảng (429, ENOSPC…).
   - JSON thô chỉ khi bấm **Chép chi tiết kỹ thuật**.
6. **Phán quyết & cảnh báo** theo bảng (thử lại · giao trợ lý khác · dừng hẳn; Bộ chống lặp; bộ nhớ ngữ cảnh…). Phục bút `plant/advance/resolve` → Đã gieo / Đẩy tiếp / Thu hồi, giữ đúng màu.
7. **Số & đơn vị:**
   - Token “3,5 triệu token”, tiền “$2,05”, số chữ “58.699”.
   - Thanh trạng thái gọn: “4,0 tr vào · 87,0K ra · đệm 75% · $2,64”.
   - “Ch. N” → “Chương N”, `arc` → “hồi”.
8. **Nút không chứa cú pháp lệnh:** “Duyệt chương 4”, “Đổi model”. Nav “Workspace” → **Bàn viết** ở mọi màn.
9. **Số đang chạy cố định chỗ:** đồng hồ “Đang lưu chương 0:49” cập nhật mỗi giây, ô rộng cố định căn phải, tabular-nums. Áp dụng cho %, đếm đoạn, đếm ngược QR. Xem `07-copy.md` › Kiểu chữ.
10. **ThinkingOrb thay ký tự nhảy và sao nhấp nháy:** quả cầu 150 chấm đều, quay chậm, sóng sáng lan trên bề mặt khoảng 1,8 giây/nhịp. Không quầng, không vòng sáng, không vệt đuôi. Canvas 2D, inline 22px sau chữ cuối của Luồng viết. Xem `17-kit.md` › ThinkingOrb.
11. **Nhãn “Chương N” không đặt trong ô rộng cố định:** dùng `min-width` hoặc xếp chồng (rail Kiểm tra 3 chương gần nhất ở Tuỳ chỉnh, danh sách chương trên điện thoại).

**Nghiệm thu:**
- [x] Bật nhật ký tiến độ của một phiên thật: không còn chuỗi `snake_case`, `WRITER/ARCHITECT/EDITOR/ARBITER`, `retry/reroute/abort`, `Ch.` trên UI. *(25/10: bản sao truyện thật `01-tra-xanh`, 368 sự kiện, cuộn hết lịch sử: Bàn viết, Nhật ký, Dự án ở 1440 và Nhật ký ở 390 đều 0 lần. Lần duy nhất khớp chữ “editor” là cụm “Bản editor” trích từ bộ quy tắc của người dùng, không phải mã trợ lý.)*
- [x] Không phần tử chứa chữ tiếng Việt nào dùng font mono. Kiểm tra bằng DevTools: `getComputedStyle(el).fontFamily`. *(Duyệt mọi nút chữ có dấu: 0 lần trên 8 route máy tính, điện thoại và máy tính bảng.)*
- [x] Công cụ mới chưa có trong bảng vẫn hiện được (chữ thường, icon code), không lỗi. *(vitest `labels.test`, `activity/mobile/logic.test`)*
- [x] Quay màn 60 giây khi trợ lý đang chạy: nhãn “Đang lưu chương”, icon ∞ và các chip bước không xê dịch dù chỉ 1px khi số đổi. *(Đo rect trong lúc đồng hồ chạy: đồng hồ, ∞ và 12 chip lệch 0 px sau 3 và 6 giây. Nhãn chỉ đổi khi chuyển bước.)*
- [x] ThinkingOrb: chỉ có chấm (không quầng/ring), nhìn ra khối cầu ở 22px; bật Giảm chuyển động thì đứng yên.
- [x] Tên vai trò tài khoản “Người viết” và trợ lý “Người viết” không đứng cạnh nhau gây nhầm. Ở menu tài khoản, luôn kèm chữ “tài khoản”.

---

## Gói K — Chuyển động cho công tắc, radio, hộp kiểm (bổ sung 25/10 · P1)

**Spec:** `17-kit.md` › *Chuyển động của điều khiển*. **Artboard:**
- `Kit.dc.html` section inputs: nhóm Chọn & bật tắt có đủ 4 loại, nhóm chọn có khối nền trượt.
- `Settings.dc.html`: công tắc, radio Cách viết, hộp kiểm “Kiểm tra lại các chương”.
- `Onboard.dc.html` step model / library / mobile.
- Mọi màn điện thoại (`.msw`).

1. Gom vào 4 component: `Switch`, `Radio`/`RadioCard`, `Checkbox`, `Segmented`. Mọi màn dùng lại, không tự vẽ điều khiển bằng `justify-content` (không animate được).
2. Chép khối CSS `motion:controls` (hoặc viết lại bằng Tailwind/`@layer components`) đúng thời gian và easing trong bảng.
3. `Segmented` có khối nền trượt. Áp cho: Áp dụng cho (3 cấp), Mức suy luận, Nhanh/Đồng sáng tác, Tốc độ/Cỡ chữ ở Trình đọc, bộ lọc Nhật ký.
4. Thay `<input type=checkbox>` gốc bằng `Checkbox`.
5. Hỗ trợ `prefers-reduced-motion`.

**Nghiệm thu:**
- [x] Quay chậm 0.25×: nút tròn công tắc vượt đích nhẹ rồi về; chấm radio nở; dấu ✓ được vẽ; khối nền nhóm chọn trượt. *(Lấy mẫu `getAnimations()`: công tắc vượt đích 14,6 → 14 px trong 300 ms; chấm radio 0 → 1,09 → 1; dấu ✓ đi 28 → 0 trong 260 ms sau 60 ms; khối nền trượt 300 ms, kể cả Nhanh/Đồng sáng tác giữ nguyên nút.)*
- [x] Đồng hồ hoặc SSE chạy nền không làm mục đã chọn nảy lại. *(Đếm ngược QR 05:00 → 04:57, Bàn viết điện thoại chạy 3 giây: giữ nguyên nút, 0 chuyển động mới. Chưa thử SSE của một phiên thật.)*
- [x] Bật “Giảm chuyển động” trên macOS/iOS: không còn trượt hay nảy, vẫn đổi màu. *(Giả lập `reduce` trên Chrome: animation `none`, chỉ còn đổi màu 200 ms. Chưa thử trên máy Mac/iPhone thật.)*
- [x] Space/Enter và phím mũi tên ra cùng chuyển động như chuột. *(Gõ phím thật trên Chrome: Space và Enter đổi công tắc, chạy chuyển động 300 ms; ArrowRight trong nhóm radio chạy `mo-dot` 320 ms.)*

---

## Gói L — Brief design 25/10: gói thể loại, bản đọc TTS, thẩm duyệt có §, Não truyện W1–W2 (P0–P1)

**Spec:** `22-brief-2510.md` (bảng tra nhanh artboard ↔ tweak, mô tả từng D, câu hỏi treo, mục “Dev: gói L”). **Artboard:** `Settings`, `Kit`, `MMore`, `NewStory`, `Reader`, `Outline`, `Onboard`, `CmdResults`, `Commands`, `MReader`, `Main`, `Activity`, `MLog`, `Brain` (mới), `AudioExport`, `Transcript`, `MAudio`, `Dialogs`.

1. **D1 Phiên bản** — chân thanh bên Tuỳ chỉnh, chấm ảnh đại diện + dòng “Có bản mới” trong menu tài khoản, banner, hộp “Có gì mới”, nhóm “Phiên bản Noverse” ở Nâng cao, “Viết bằng phiên bản” ở Thông tin truyện.
2. **D2 Gói thể loại** — nhóm “Gói thể loại” trước “Kiểu viết”, thẻ gói (số chương chuẩn, badge “Có chấm”, “Sắp có”), chi tiết gói (lớp luật Master/Bộ A/Cách đo, số [LỖI]/[CẢNH BÁO], § mở được, số đo, giọng). Quy tắc của người dùng chỉ ghi đè [CẢNH BÁO] và mục không nhãn.
3. **D3 Bản đọc TTS** — `narration` + `doc-so` ở mọi chỗ xuất; bỏ đường dẫn khỏi thẻ `/export`; menu chương điện thoại có bước chọn định dạng; Hướng dẫn lệnh có `--keep`, `doc-so`.
4. **D4 Thẻ thẩm duyệt có §** — một component dùng chung (`ReviewIssue`): mức Bắt sửa/Cảnh báo, chip §, tên mục, câu lỗi, đoạn trích, gợi ý. Có Bắt sửa ⇒ chương tự vào hàng chờ sửa; cửa duyệt chỉ mở khi hết. Bàn viết phase gate thay khung luồng bằng thẻ “Thẩm duyệt chương N”.
5. **D5** — thẻ “Khung truyện” (người nghe §1.2, Điểm khác §17, hạt giống phần hai §19) + chế độ “Bản đồ giữ chân” (dải vị trí, 4 phép kiểm §2/§3/§7/§8, bảng mẫu §21).
6. **D6** — rail chỉ hiện chỗ lệch, “Xem đủ số đo” và Nhật ký hiện đủ số kèm chuẩn.
7. **D7–D9 Xưng hô** — trang nhân vật (bảng có chiều, lịch sử quan hệ, chương xuất hiện), sửa có khoá khi đang viết và hộp xác nhận khi chạm chương đã viết; lint xưng hô dùng thẻ D4 (`ref = "Xưng hô"`).
8. **D10 Não truyện** — `StoryGraph` theo mục D10 “Đồ thị v3” trong `22-brief-2510.md` (thẻ nút, màu nhận diện nhân vật, cạnh cong, nhãn né nhau, đo chữ sau khi font tải xong, cắt “…”). Điện thoại: Gói M mục 2.
9. **D11** — gói `voice: single` ⇒ “Một giọng kể · theo gói” ở Transcript/AudioExport/MAudio; bỏ tên tệp mẫu giọng.
10. **Quyết định 6** — bỏ bước ngôn ngữ ở thiết lập lần đầu (7 bước).
11. **Câu hỏi treo** — Dialogs `locked` (bỏ “TUI”), `recovery` (409), mô tả `/simulate` không nhắc thư mục, Esc một lần (`17-kit.md` › Phím Esc), ẩn `/reopen` với người không có quyền viết, quy tắc viết lỗi 500: thử lại 3 lần/~20 giây rồi lưu nguyên văn (`Settings.rules=retry`).

**Nghiệm thu:**
- [ ] Mở từng artboard với tweak ở bảng tra nhanh của `22-brief-2510.md`, so chữ và trạng thái 1:1.
- [ ] Không có đường dẫn, tên tệp, cú pháp lệnh ngoài Nâng cao và trang Hướng dẫn lệnh; không chữ tiếng Việt trong font mono.
- [ ] Chương có mục Bắt sửa không duyệt được; chấm lại hết Bắt sửa thì cửa duyệt mở.
- [ ] Sửa xưng hô với “từ chương” ≤ chương đã viết luôn hiện hộp xác nhận; khi engine đang viết thì nút sửa bị khoá.
- [ ] Não truyện: đổi chương 1→8 đúng thực thể và kiểu cạnh; chữ không tràn thẻ khi font tải chậm và khi tên dài; Tab/Enter chọn được nút; giảm chuyển động tắt hạt chạy.

---

## Gói M — Phần trước chưa vẽ: quản trị đầu tiên trên web, Não truyện điện thoại, D13–D15 (bổ sung 25/10 · P0–P2) <!-- goi-m -->

**Spec:** `22-brief-2510.md` › *Bổ sung 25/10 — phần trước chưa vẽ*; `09-auth-onboard.md` §11; `18-mobile.md` › M8. **Artboard:** `Setup`, `MBrain`, `Quality`, `ProjBrain`, `SysBrain` (đều mới), `Outline`, `Settings`, `MMore`. **Backend:** GAP-41…46.

1. **A0 Lần chạy đầu (P0 · GAP-41)**
   - Máy chủ khởi động mà chưa có tài khoản thì sinh mã 8 ký tự (bỏ các ký tự dễ nhầm 0/O/1/I/L), in ra stdout và log cùng địa chỉ mở. Mã hết hạn sau 30 phút; khởi động lại thì có mã mới. Bỏ câu hỏi tên/mật khẩu trong terminal.
   - Route `#/setup`. Khi `/api/me` trả 401, app gọi `GET /api/setup`; nếu `needed=true` thì mở A0 thay vì Đăng nhập.
   - Ô mã: 8 ô hiển thị và một input ẩn (`autocomplete="one-time-code"`). Nhận dán có hoặc không có gạch nối. Đủ 8 ký tự thì tự gọi verify.
   - Sai 5 lần khoá 10 phút, tính phía server.
   - Tạo xong: tự đăng nhập → A3 luồng quản trị (bước Hồ sơ điền sẵn tên; mã khôi phục ở bước Hoàn tất như hiện có). Đầu rail A3 có khối “Lần chạy đầu” ✓✓.
   - Mã hết hạn khi đang điền form (`state=tokenExpired`): giữ dữ liệu form, cho nhập mã mới ngay trong form (`POST /api/setup/verify` lần nữa).
   - Hai người cùng thiết lập: người sau nhận 409 → `state=taken`.
2. **M8 Não truyện điện thoại (P1 · dùng GAP-39)**
   - Thêm › Não truyện mở `MBrain`.
   - Không render đồ thị.
   - Bottom sheet dùng mẫu sheet đã có.
   - Trang Đọc (`MReader.state=person`): tên nhân vật là nút riêng (gạch chấm), chạm mở sheet xem nhanh, chỉ dữ liệu tới chương đang đọc. `MBrain` mở từ đây thì nhận chương đang đọc.
   - Dải nấc chương dùng chung cách tính trạng thái tại chương N với bản máy tính.
3. **D13 Chất lượng truyện (P2 · GAP-42)**
   - Nút “Chất lượng” kèm số Bắt sửa còn mở ở đầu trang truyện.
   - Ma trận mục luật × chương.
   - Panel phải dùng `ReviewIssue`.
   - Lọc phía client.
   - Màn đầu “Việc cần làm” (`view=todo`) chia 3 nhóm: Đang chặn chương / Cần bạn xem / Sẽ tự sửa khi viết lại. “Nhờ sửa ở chương này” = thêm mục vào ý kiến cho lần viết lại; “Giữ nguyên…” = ghi `kept` kèm lý do.
   - Truyện dài (`len=40`): cuộn ngang, cột đầu `position: sticky`.
   - Liên kết vào từ Bàn viết (phase gate) và Trình đọc (tab Thẩm duyệt).
4. **D14 Não dự án (P2 · GAP-43…45)**
   - Mục “Não dự án” ở thanh bên dự án, có 3 tab.
   - Luật khoảng cách tính ở server: khi lưu brief và trước khi Kiến trúc sư lên dàn ý.
   - Chặn ở Tạo truyện mới: artboard `NewStory.check` (checking · conflict · fixed), cả Bắt đầu nhanh và Đồng sáng tác. Không lỗi → vào thẳng Bàn viết; chỉ Cảnh báo → vẫn bắt đầu được; có Bắt sửa → khoá nút cho tới khi đổi trục (`POST …/ledger/suggest`) hoặc sửa yêu cầu.
5. **D15 Não hệ thống (P2 · GAP-46 · chỉ quản trị)**
   - Mục “Não hệ thống” ở đầu nhóm Hệ thống của Tuỳ chỉnh, có số chờ duyệt; chấm tím trên nút Tuỳ chỉnh khi có bài học chờ duyệt (chỉ quản trị). Không đặt ở Nâng cao.
   - Thao tác giữ / bỏ / sửa câu chữ / hoàn tác.
   - Giữ: ghi quy tắc vào lớp “Mọi truyện” hoặc lớp gói (`20-settings-rules.md`), áp dụng từ truyện mới tiếp theo.
   - Lời nhắc được đưa vào ngữ cảnh của Kiến trúc sư khi lên dàn ý truyện cùng gói.

**Nghiệm thu:**
- [ ] Lần chạy đầu không cần gõ gì trong terminal. Mã sai, bị khoá, hết hạn và đã có quản trị hiện đúng các trạng thái của `Setup`.
- [ ] Mã không xuất hiện trong phản hồi API, log trình duyệt hay URL. So sánh mã theo hằng thời gian. Khi đã có quản trị, `GET /api/setup` trả `needed=false` và các `POST /api/setup/*` trả 409.
- [ ] `MBrain` đổi chương 1→8 đúng nhãn “mới” và “đổi ở ch. N”; ở 390 px không có đồ thị.
- [ ] Số trong ma trận Chất lượng khớp thẻ thẩm duyệt của từng chương; ô trống là đạt.
- [ ] Sổ cái không hiện truyện người dùng không được xem. `ProjBrain.check=conflict` cho đúng 4 phép kiểm.
- [ ] Não hệ thống ẩn với người không phải quản trị (không có hàng ở Nâng cao, API trả 403). Bài học đã giữ không chứa tên nhân vật hay tình tiết.
- [ ] `NewStory`: yêu cầu trùng 4 trục chính với truyện cũ thì không bắt đầu viết được; đổi trục xong thì được. Chỉ Cảnh báo thì không chặn.
- [ ] Chữ không tràn thẻ khi font tải chậm. Không có chữ tiếng Việt trong font mono, trừ tên tham số trong cú pháp lệnh.

---

## Gói N — Đổi thương hiệu Noverse (bổ sung 26/10 · P0) <!-- goi-n -->

Đọc `23-brand.md` trước. Artboard: `Brand.dc.html` (B0), logo ở thanh trên cùng của `Login`, `Setup`, `Onboard`, `OAuth`, `Phone` và thanh app (`_appheader`).

1. **Biểu tượng.**
   - Chép `docs/design/v1/brand/png/icons/` vào `web/public/icons/`: favicon, `favicon.ico`, `apple-touch-icon.png`, `icon-192/512.png`, `icon-maskable-512.png`, `noverse-og-image.png`, cùng `svg/favicon.svg`.
   - Bỏ `icon-192.svg` và `icon-512.svg` cũ khỏi manifest.
2. **`index.html` và manifest.**
   - Đổi `<title>`, favicon, thẻ `og:*` theo mẫu ở `23-brand.md` §4.
   - Manifest: `name` “Noverse · Xưởng viết truyện”, `short_name` “Noverse”, icon PNG `any` + `maskable`.
3. **Component `BrandMark`** (`size`, `variant`).
   - Logo ở thanh trên cùng: biểu tượng **bản phẳng** 22 px trên máy tính (mọi màn), 20 px trên điện thoại; chữ Noverse Plus Jakarta Sans 16/700, khoảng chữ −0,012 em, rồi “Studio” 13 px `#7E7E8A`.
   - Thanh app: chỉ biểu tượng, thay hàng 4 orb.
   - Tải font theo `&text=Noverse`, không tải cả bộ.
4. **Đổi chữ người dùng thấy** từ `ainovel` sang Noverse:
   - i18n, copy, mock và snapshot test trong `web/src`;
   - trang OAuth do server sinh, thông báo đẩy, chuỗi Go hiện lên web (vd. “Noverse 0.9.3”);
   - thư mục gợi ý “Truyện Noverse”;
   - TUI/CLI: màn chào “N O V E R S E” và mọi chữ hiển thị (không chỉ web);
   - tên tệp tải về: `ma-khoi-phuc-noverse.txt` và nội dung tệp.

   **Giữ nguyên tên kỹ thuật** (danh sách ở `23-brand.md` §5): lệnh, thư mục cấu hình, biến môi trường, khoá `localStorage`.
5. **Nghiệm thu.**
   - `grep -rnw ainovel web/src` chỉ còn tên kỹ thuật.
   - Chụp các màn có logo ở 1440 và 390, so với artboard.
   - Lighthouse nhận icon `maskable`.
   - Favicon 16 px còn đọc được 4 quả cầu.

## Gói O — Sửa lỗi UI design báo ngày 26/10 (P0) <!-- goi-o -->

Kiểm từ ảnh chụp app thật, so với artboard. Mỗi mục ghi: chỗ sai → đúng là gì → tham chiếu. Làm xong ghi kết quả vào `19-gap-closure.md`, mỗi mục kèm ảnh chụp 1440.

### O1 · Bàn viết: nhật ký tiến độ và luồng viết (`Main`, `Activity`)

1. **Nút trạng thái “Khôi phục: giai đoạn lập kế hoạch (premise)”.**
   - Sai: lộ chữ tiếng Anh “premise” (phạm quy tắc Việt hoá, Gói A) và lặp ý với nút “Giai đoạn Tiền đề” bên cạnh.
   - Đúng: ghi bước sẽ chạy tiếp, vd. “Khôi phục · tiếp tục từ Lưu dàn ý”. Mọi tên giai đoạn đi qua bảng dịch nhãn (`06` › bảng dịch nhãn), không in khoá kỹ thuật.
2. **Dòng đầu nhật ký bị cắt nửa** dưới tiêu đề “Nhật ký tiến độ”.
   - Đúng: vùng cuộn có mờ 16 px ở mép trên, hoặc cuộn khớp theo từng dòng. Không để chữ nằm dưới tiêu đề.
3. **Dòng “Bắt đầu sáng tác (người lập kế hoạch: Kiến trúc sư — [lý do dài của AI]…”.**
   - Sai: in nguyên lý do của AI, cắt “…” và không mở ra được.
   - Đúng theo `Activity` › Khởi động sáng tác: “Bắt đầu sáng tác (Kiến trúc sư lập kế hoạch truyện dài — [lý do ngắn])”.
   - Lý do tối đa khoảng 80 ký tự. Bản đầy đủ hiện khi rê chuột (`title`) hoặc khi bấm mở dòng.
4. **“Kiến trúc sư · Xây dựng kịch bản ở cấp độ dàn...” bị cắt cứng theo số ký tự** dù bên phải còn trống.
   - Đúng: cắt bằng CSS (`text-overflow: ellipsis`) theo chiều rộng thật. Không cắt chuỗi trong code.
5. **Dòng con đang chạy (vd. “Lưu dàn ý”) lệch phải khoảng 12 px** và mất nét nhánh “├”.
   - Đúng: vòng xoay thay vào đúng chỗ icon của bước, giữ nét nhánh. Chữ của bước đang chạy thẳng hàng với các bước cùng cấp.
6. **Khung “Luồng viết trực tiếp” ghi “Người viết” với chấm tím** trong khi trợ lý đang chạy là Kiến trúc sư.
   - Đúng: tên và màu chấm theo trợ lý đang chạy (Kiến trúc sư: xanh dương `.o-architect`).
7. **Thẻ “Thiết lập” màu cam ở cuối khung luồng viết** không nói đang làm gì. Cam là màu của cảnh báo và của Biên tập viên.
   - Đúng: thẻ của bước đang chạy, tiêu đề “Lưu dàn ý · Kiến trúc sư”, màu theo trợ lý.
   - “Chi tiết kỹ thuật” vẫn ẩn trong nút mở như hiện nay.
8. **Thiếu thanh tiến độ mảnh** ở mép trên khung luồng viết (artboard `Main`: vạch gradient chạy theo tiến độ bước).
9. **Khung luồng viết trống gần 600 px suốt giai đoạn lập kế hoạch.**
   - **Đã vẽ (26/10):** làm theo Gói P (`Main`, `phase=plan`). Bỏ cách tạm thời bên dưới.
   - Trong lúc chờ: khi không có chữ đang viết, thu khung về chiều cao tối thiểu và hiện một dòng “Kiến trúc sư đang lưu dàn ý · xem từng bước ở Nhật ký tiến độ”. Không để khoảng đen trống.

### O2 · Danh sách Nhân vật (Bàn viết › Truyện, Dự án & truyện, Não truyện)

10. **Chữ viết tắt trong avatar sai nguồn**: không chữ nào khớp tên, có dấu hiệu lệch hàng. Vd. “TD” của Trần Duy đang nằm ở dòng Hạ Vy, còn dòng Trần Duy là “VÂ”.
    - Đúng: tính từ `name` theo luật ở `04-components.md` › *Avatar nhân vật*. Ví dụ: Lâm An → LA, Lâm Tri Hạ → TH, Luật sư Hàn → LH.
    - Dùng `ini` người dùng tự đặt nếu có.
    - Thêm test: mỗi avatar khớp tên cùng dòng.
11. **Vai bị gộp vào tên trong ngoặc**, vd. “Lâm An (Nữ chính; người kể xưng tôi)”.
    - Đúng: tên đậm 13/500, sau đó vai 12 px `#7E7E8A` cùng dòng, các ý nối bằng “ · ”, vd. “Lâm An · Nữ chính · người kể xưng tôi”.
    - Dữ liệu cũ đang gộp: tách phần trong ngoặc ra làm `role`, đổi “;” thành “ · ”.

### O3 · Logo (nhắc lại Gói N)

12. Logo ở thanh trên cùng dùng **bản phẳng**: 22 px trên máy tính (mọi màn, kể cả Bàn viết, Nhật ký tiến độ, Trình đọc, Tạo truyện mới), 20 px trên điện thoại.
    - Không còn hàng 4 orb cũ ở bất kỳ màn nào.
    - Chữ “Noverse” và “Studio” canh theo đường chân chữ, cách nhau 6 px (`23-brand.md` §4).

### Nghiệm thu

- Chụp 1440 các màn: Bàn viết ở giai đoạn lập kế hoạch và đang viết; Nhật ký tiến độ; danh sách Nhân vật có ít nhất 7 người, gồm tên 2 chữ, tên 3 chữ và tên có danh xưng.
- `grep` không còn chữ tiếng Anh của tên giai đoạn trong chuỗi hiển thị.
- Ghi “Gói O” vào `19-gap-closure.md` với ✓/✗ từng mục 1–12.

## Gói P — Bàn viết: giai đoạn Lập kế hoạch và lối vào nhanh Não truyện (26/10 · P0) <!-- goi-p -->

Artboard `Main.dc.html` (01 · Bàn viết), tweak mới: `phase=plan`, `drawer=brain`, `peek=nv`. Gói này thay mục O9 (khung luồng trống).

### P1 · Giai đoạn Lập kế hoạch (`phase=plan`)

- **Nút trạng thái:** “Giai đoạn **Lập kế hoạch**” chữ xanh `#B9D0F7`; khi khôi phục thêm “Khôi phục · tiếp tục từ {bước}”.
- **Nhật ký và khung kế hoạch:** nhật ký `flex 3`, khung kế hoạch `flex 7`; ở giai đoạn viết vẫn là `4 / 6`.
- **Khung luồng đổi thành “Kế hoạch truyện”:**
  - chấm và thanh tiến độ màu Kiến trúc sư (`.g-architect`);
  - phụ đề “Kiến trúc sư · 4/5 bước”;
  - nhãn chạy theo bước, vd. “Đang lưu dàn ý”.
- **Năm bước, mỗi bước một hàng bấm mở được:** Thông tin truyện · Tiền đề · Nhân vật · Quy tắc thế giới · Dàn ý.
  - Hàng gồm: biểu tượng trạng thái (✓ / vòng xoay / tạm dừng), tên bước, tóm tắt một dòng (cắt “…”), thời lượng, mũi tên.
  - Bước xong hiện ngay tóm tắt, không cần mở.
  - Bước đang chạy mở sẵn, viền xanh nhạt.
  - Nội dung khi mở:
    - Thông tin truyện: các ô khoá–giá trị;
    - Tiền đề: đoạn văn;
    - Nhân vật: chip người, bấm vào mở ngăn kéo Não truyện chọn sẵn người đó;
    - Quy tắc thế giới: danh sách;
    - Dàn ý: lưới 2 cột, chương đã lưu ghi “tên · móc”, chương chưa lưu là thanh chờ nhấp nháy.
  - Dưới cùng: “Lập xong kế hoạch, Người viết bắt đầu chương 1. Muốn đổi gì thì góp ý ở ô bên dưới trước khi viết.”
- **Cột Đại cương:** dòng “Đang lập dàn ý · 5/8” có vòng xoay. Chương đã lưu hiện tên, chương chưa lưu là thanh chờ. “Mới nhất” ghi “Chưa có chương”.
- **Thẻ trợ lý:** Kiến trúc sư “Đang chạy”, dòng việc “Kiến trúc sư · đang lưu dàn ý”.
- **Ô nhập:** placeholder “Góp ý cho kế hoạch, ví dụ: …”.
- **Tab Truyện:** ẩn “Bản nộp mới nhất” và “Đổi ở chương”. Tóm tắt gần đây ghi “Chưa có chương nào…”.
- **Dữ liệu:** dùng sự kiện lưu từng phần sẵn có của Kiến trúc sư (`save_story_info`, `save_premise`, `save_characters`, `save_world_rules`, `save_outline` hoặc tên tương đương). Dàn ý cần đếm “đã lưu n/N chương”. Thiếu trường nào thì ghi vào `19-gap-closure.md`.

### P2 · Tab Truyện: lối vào nhanh

- **Đầu thẻ Nhân vật** có hai lối:
  - nút **Đồ thị** (icon `i-routing`, `title` “Mở Não truyện (phím G)”) mở ngăn kéo;
  - **Tất cả** mở Não truyện ở dạng Danh sách, không còn dẫn về Dự án & truyện.
- **Hàng nhân vật** là nút: bấm thì mở ngăn kéo chọn sẵn người đó.
  - Rê chuột hoặc focus thì hiện thẻ nhỏ đè xuống dưới hàng, rộng bằng thẻ, `role="tooltip"`.
  - Thẻ gồm: “Ở chương N”, tối đa 2 quan hệ (vạch màu, loại, đối tượng, “từ chương …”), một dòng xưng hô, gợi ý “Bấm để mở trên Não truyện”.
  - Tweak `peek=nv` cho thấy trạng thái này.
- **Thẻ mới “Đổi ở chương N”:** quan hệ hoặc xưng hô đổi ở chương đang viết, chip “Mới” và nút “Xem trên đồ thị ›”. Không có gì đổi thì ghi “Chương này không đổi quan hệ hay xưng hô nào.”
- **Giới thiệu tóm tắt:** thêm “Khung truyện ›” (Dự án & truyện › khung truyện).
- **Tóm tắt gần đây:** mỗi dòng là liên kết mở Trình đọc đúng chương.

### P3 · Ngăn kéo Não truyện (`drawer=brain`)

- **Hình thức:** trượt từ phải, rộng `min(1180px, 84vw)` (sửa 28/10, Gói U), lớp phủ mờ phía sau. Luồng viết vẫn chạy.
  - `role="dialog"`, `aria-modal`.
  - **Esc** hoặc bấm lớp phủ để đóng; phím **G** để mở (bỏ qua khi đang gõ trong ô nhập).
- **Đầu ngăn kéo:**
  - “Não truyện” và tên truyện · “Chương N · chỉ xem”;
  - chip “Đang viết · chỉ xem”;
  - “Mở toàn màn ›” dẫn tới màn Não truyện đầy đủ;
  - “Đóng Esc”.
- **Thân, bên trái và bên phải:** xem *Gói U* (28/10). Ngăn kéo dùng **đúng** component đồ thị, dòng thời gian chương và bảng chi tiết của màn Não truyện; mô tả cũ ở đây (đường thẳng, nhãn giữa đường, hàng ô chương) bỏ.
- **Khoá:** đang viết thì chỉ xem, kèm ghi chú “Trợ lý đang viết nên Não truyện chỉ cho xem. Muốn sửa…, tạm dừng rồi mở toàn màn.”
- **Điện thoại:** chạm tên nhân vật ở màn Viết thì mở tấm kéo lên của nhân vật trong `MBrain`, không có ngăn kéo.

### P4 · Khung luồng khi không có trợ lý nào chạy (thay ô “Chưa có trợ lý nào đang viết”)

Không để khung trống với một dòng chữ ở đáy. Khi không có gì đang viết, khung luồng hiện **một thẻ ở giữa** theo trạng thái. Thẻ rộng tối đa 660 px, gồm:
- biểu tượng màu;
- tiêu đề nói đang ở đâu;
- một câu giải thích;
- 2–3 nút;
- một dòng gợi ý dùng ô nhập.

Khi có thẻ này, nhật ký và khung luồng chia `5 / 5`. Tiêu đề khung giữ “Luồng viết trực tiếp”, chấm mờ, phụ đề ghi trạng thái.

| Trạng thái (`phase`) | Phụ đề | Thẻ | Nút |
|---|---|---|---|
| `wait` · Đang chờ bạn | Đang chờ bạn | Viền cam, “Trọng tài cần bạn chọn cách đi tiếp” + câu hỏi thật của Trọng tài | Lựa chọn của Trọng tài (nút chính là phương án mặc định) · “Đọc chương N” |
| `ready` · Kế hoạch xong, chưa viết | Chưa bắt đầu | Viền tím, “Kế hoạch đã xong · sẵn sàng viết chương 1” | Bắt đầu viết chương 1 · Xem kế hoạch · Não truyện |
| `done` · Hoàn thành | Không có trợ lý nào đang chạy | Viền xanh lá, “Truyện đã xong · N chương · X chữ” | Đọc từ đầu · Tạo audio · Bản xuất |
| Tạm dừng | — | Không dùng thẻ: giữ luồng viết đang dở, nút Tiếp tục ở đầu màn | — |

- **Ở trạng thái `wait`:**
  - nút trạng thái “Đang chờ bạn” màu cam;
  - dòng cuối nhật ký là câu hỏi, chữ cam;
  - placeholder ô nhập là “Trả lời Trọng tài, ví dụ: …”;
  - chương đang chờ trong Đại cương được tô cam nhạt.
- **Câu hỏi và các lựa chọn** lấy từ phán quyết của Trọng tài (`ARBITER` cần người dùng). Thiếu trường thì ghi vào `19-gap-closure.md`.
- **Lý do:** ảnh app ngày 26/10 ghi “Đang chờ bạn” nhưng thân khung lại báo “Chưa có trợ lý nào đang viết”, không nói chờ gì và phải làm gì.

### Nghiệm thu

- Chụp 1440: `phase=plan` (đang chạy và tạm dừng), `wait`, `ready`, `done`, tab Truyện có thẻ rê, ngăn kéo mở với 2 nhân vật khác nhau.
- Phím: G mở, Esc đóng, Tab đi qua nút nhân vật và nút trong ngăn kéo theo đúng thứ tự.
- Ghi “Gói P” vào `19-gap-closure.md`.

## Gói Q — Chuyển trang mượt: thanh trên đứng yên, chỉ báo trượt, nội dung hiện dần (27/10 · P1) <!-- goi-q -->

Artboard `Nav.dc.html` (10b), bấm thử được: “Trước” mô phỏng app hiện tại, “Sau” là cách cần làm. Có thêm nút “Xem chậm ×5” và “Trang mới tải chậm”.

### Vì sao đang nhảy

1. Thanh trên được dựng lại ở mỗi trang, mỗi trang lề và khoảng cách hơi khác nhau.
2. Đường dẫn bên trái dài ngắn khác nhau làm thanh điều hướng giữa bị đẩy (flex).
3. Mục đang chọn đổi độ đậm chữ nên nút giãn ra, các mục bên cạnh xê dịch.
4. Trang cũ biến mất trước khi trang mới có dữ liệu, gây chớp trắng hoặc vòng xoay giữa màn.
5. Trang có thanh cuộn và trang không có làm nội dung rộng hẹp khác nhau vài pixel.

### Việc cần làm

1. **Khung app dùng chung.** `AppShell` ở route gốc chứa `AppHeader`, dock việc nền và phần chung khác. Mỗi trang chỉ render phần nội dung trong `<Outlet>`. Cấm trang tự render thanh trên.
2. **Lưới 3 cột:** `grid-template-columns: minmax(0,1fr) auto minmax(0,1fr)`, cao 54 px, lề ngang 16 px.
   - Đường dẫn bên trái `min-width: 0`, cắt “…”.
   - Thanh điều hướng luôn ở chính giữa.
3. **MainNav:**
   - Mọi mục đậm 500, rộng theo chữ cố định. Mục chọn chỉ đổi màu chữ `#EDEDEF` và icon `#C9A6F5`.
   - Một `span` nền duy nhất (`#222225`, bo 7) đặt tuyệt đối, trượt tới mục mới bằng `transform: translateX` + `width`, 280 ms, `cubic-bezier(.3,.9,.3,1)`. Đo vị trí bằng `offsetLeft` / `offsetWidth`, đo lại khi đổi cỡ cửa sổ.
   - Chỉ báo trượt **ngay khi bấm**, không chờ dữ liệu.
4. **Giữ trang cũ tới khi trang mới sẵn sàng:**
   - Điều hướng bọc `startTransition` (React 18), Suspense giữ nội dung cũ.
   - Quá 300 ms thì hiện vạch tiến độ 2 px gradient tím ngay dưới thanh trên. Không có vòng xoay giữa màn, không trang trắng.
5. **Nội dung mới hiện dần:**
   - `opacity 0 → 1` + `translateY(6px) → 0`, 200 ms, `cubic-bezier(.2,.8,.2,1)`. Không có hiệu ứng khi rời trang.
   - Đường dẫn bên trái mờ dần 200 ms.
   - Có thể dùng View Transitions API (`document.startViewTransition`) với `view-transition-name: app-header` để thanh trên đứng yên. Nếu trình duyệt không hỗ trợ thì dùng CSS như trên.
6. **Nạp trước:** rê chuột hoặc focus vào mục điều hướng thì tải sẵn mã trang (`import()` của route) và dữ liệu chính, qua bộ đệm của tầng dữ liệu (`16-feedback-refresh.md`).
7. **Chống xô lệch:**
   - `scrollbar-gutter: stable` trên vùng cuộn chính.
   - Khung tải (skeleton) đúng kích thước khối thật.
   - Nạp sẵn font Be Vietnam Pro 400/500/600 (`<link rel=preload>`).
   - Nhớ vị trí cuộn và trạng thái cột (ẩn/hiện, tab đang chọn) của từng trang khi quay lại.
   - Bàn viết giữ kết nối luồng sự kiện ở cấp app, không ngắt khi rời trang.
8. **Giảm chuyển động** (`prefers-reduced-motion`): bỏ trượt và nhích, chỉ mờ dần 120 ms.

### Nghiệm thu

- Quay video chuyển qua lại 5 mục ở 1440: thanh điều hướng không xê dịch pixel nào (so khung hình), không chớp trắng.
- Chuyển sang trang có dữ liệu chậm (giả lập 3G) thì thấy vạch tiến độ, trang cũ vẫn còn.
- Lighthouse CLS < 0,02 khi chuyển trang.
- Ghi “Gói Q” vào `19-gap-closure.md`.

## Gói R — Tuỳ chỉnh: chọn dự án và truyện ngay ở thanh “Áp dụng cho” (27/10 · P1) <!-- goi-r -->

Artboard `Settings.dc.html`, tweak mới:
- `pick`: none · project · story · confirm;
- `target`: open · other (đang xem dự án “Vân Châu”, truyện “Đám Giỗ Cả Họ”).

**Vấn đề:** thanh “Áp dụng cho” luôn bám truyện đang mở. Muốn sửa quy tắc cho dự án khác, người dùng phải ra thanh đường dẫn đổi truyện rồi mở lại Tuỳ chỉnh.

### R1 · Nút ▾ trên ô “Truyện” và ô “Dự án”

- Mỗi ô gồm hai nút:
  - bấm phần chữ thì chọn cấp như cũ;
  - bấm ▾ (`aria-haspopup="listbox"`, `aria-label` “Chọn dự án khác” / “Chọn truyện khác trong dự án”) thì mở danh sách.
- “Mọi truyện” không có ▾.
- Khi ô đang chọn, viền và nền bọc cả hai nút.
- Nhãn đổi theo lựa chọn:
  - “Truyện này” khi là truyện đang mở, “Truyện” khi là truyện khác;
  - dòng phụ là tên truyện rút gọn, hoặc “Tên dự án · N truyện”.

### R2 · Danh sách chọn (popover rộng 360 px, neo dưới ô)

- **Dự án:**
  - tiêu đề “Chọn dự án” + “N dự án”, ô tìm luôn có;
  - mỗi dòng: icon thư mục, tên, “N truyện · M quy tắc riêng” (hoặc “theo “Mọi truyện”” khi chưa có quy tắc riêng);
  - chip “Đang mở” cho dự án đang mở ở Bàn viết, dấu ✓ cho dự án đang xem.
- **Truyện:**
  - tiêu đề “Truyện trong “{dự án}”” + “N truyện”, ô tìm khi có hơn 5 truyện;
  - dòng gồm tên đầy đủ (cắt “…”) và “M quy tắc riêng” hoặc “theo dự án”;
  - dự án chưa có truyện thì ghi “Dự án này chưa có truyện.”
- **Cuối danh sách**, khi đang xem chỗ khác truyện đang mở: “Quay về truyện đang mở · {tên}”.
- **Đóng:** Esc, bấm ra ngoài, hoặc chọn xong.
- **Phím:** ↑↓ di chuyển, Enter chọn, như `Select` ở Kit.

### R3 · Khi đổi lựa chọn

- Danh sách quy tắc, bộ đếm ở mục điều hướng trái và bản tổng hợp “Noverse hiểu thế nào?” đổi theo dự án/truyện đang xem.
- Nhóm kế thừa ghi “Từ dự án “{tên}””.
- Đổi dự án thì “Truyện” tự chọn truyện đang mở (nếu cùng dự án), không thì chọn truyện đầu tiên.
- Dự án không có truyện thì ô “Truyện” mờ đi và nhảy về cấp “Dự án”.
- Đang xem chỗ khác truyện đang mở thì hiện dải xanh dưới thanh: “Đang xem quy tắc của dự án “Vân Châu”. Truyện đang mở ở Bàn viết vẫn là “…”.” kèm nút “Quay về truyện đang mở”.
- **Không đổi truyện đang mở ở Bàn viết**, thanh đường dẫn giữ nguyên.
- **Còn thay đổi chưa lưu:** hiện hộp cam ngay trong danh sách, “Có N thay đổi chưa lưu cho {đối tượng}. Lưu trước khi chuyển?”, với 3 nút: Lưu rồi chuyển · Bỏ thay đổi · Ở lại.
- Quy tắc mới thêm, chuyển lên dự án, giữ thành quy tắc truyện đều gắn với dự án/truyện **đang xem**.
- Áp dụng cho mọi mục có 3 cấp: Quy tắc viết, Cách viết, Trợ lý AI, Ngân sách, Thông tin.
- **Đường dẫn:** `#/settings/rules?project={id}&story={id}`. Mở lại trang thì giữ lựa chọn; không có tham số thì dùng truyện đang mở.
- **Điện thoại (`MMore`):** chạm vào dòng phạm vi thì mở bottom sheet cùng nội dung danh sách.

### Nghiệm thu

- Chụp 1440: danh sách dự án mở, danh sách truyện của dự án khác, hộp “chưa lưu”, dải “Đang xem…”.
- Chuyển dự án rồi thêm quy tắc: quy tắc lưu đúng vào dự án đang xem (kiểm tra tệp quy tắc của dự án đó).
- Ghi “Gói R” vào `19-gap-closure.md`.

## Gói S — Avatar nhân vật phân biệt theo loại vai (28/10 · P1) <!-- goi-s -->

**Vì sao:** avatar hiện cùng một màu xám, nhìn danh sách không biết ai là chính, ai là phản diện. Artboard: `Main.dc.html` (tab Truyện, `peek`=nv, `phase`=plan, `drawer`=brain), `Brain.dc.html`, `MBrain.dc.html`. Spec đầy đủ: `04-components.md` › *Avatar nhân vật* › *Màu theo loại vai*.

### Việc FE

- Component `CharAvatar` dùng chung, props: `name`, `ini?`, `kind`, `hidden`, `size` (`sm` ≤ 20 · `md` 26 · `lg` 40). Chữ viết tắt giữ luật cũ.
- Màu viền/nền/chữ và huy hiệu góc theo `kind` (Chính tím + sao · Phản diện đỏ + tam giác · Đồng minh xanh + khiên · Phụ xám, không huy hiệu). Huy hiệu ẩn ở `sm`.
- `hidden` = `revealAt != null && chương đang xem < revealAt` → viền nét đứt. Ở Bàn viết “chương đang xem” là chương mới nhất đã nộp; ở Não truyện là chương trên thanh trượt.
- Danh sách Nhân vật sắp theo Chính → Phản diện → Đồng minh → Phụ, có dòng chú giải bên dưới. Thẻ rê ghi nhãn loại bên cạnh vai.
- Ngăn kéo Não truyện: nút đồ thị dùng cùng màu; nút mờ dùng `filter`, nền đặc. Ngăn chi tiết: avatar 40 px + chip loại.
- Não truyện (máy tính và điện thoại): màu nút nhân vật lấy theo loại vai, bỏ bảng màu riêng từng người.
- Dự án › Nhân vật: thêm ô chọn “Loại vai” (Chính · Phản diện · Đồng minh · Phụ) và “Lộ vai ở chương” (để trống nếu không có bí mật).

### Việc backend

- Thêm `kind`, `revealAt` vào nhân vật (xem `06-data-contract.md` › *Bổ sung §S*). Kiến trúc sư điền khi lưu nhân vật; chuyển dữ liệu cũ một lần từ `role`.

### Nghiệm thu

- Chụp 1440 tab Truyện của Bàn viết: 6 nhân vật đủ 4 loại, có ít nhất 1 viền nét đứt; so với artboard.
- Mở ngăn kéo Não truyện, chọn một nút: các nút khác mờ nhưng không thấy cạnh xuyên qua.
- Kiểm tra đọc màn hình: avatar đọc “{tên}, {loại}”.
- Ghi “Gói S” vào `19-gap-closure.md`.

## Gói T — Nút lên đầu cho chỗ đọc và danh sách dài (28/10 · P1) <!-- goi-t -->

**Vì sao:** chương dài và danh sách dài phải cuộn tay rất lâu mới về đầu, nhất là trên điện thoại. Artboard: `ScrollTop.dc.html` (10c) và bản gắn sẵn ở `Reader`, `MReader`, `Outline`, `Transcript`. Spec: `04-components.md` › *Nút lên đầu*.

### Việc FE

- Component `ScrollTopButton` dùng chung, props: `target` (ref vùng cuộn), `focusTarget?` (ref tiêu đề), `label`, `progress?` (bật vòng tiến độ), `size` (`md` 40 · `lg` 44), `hideWithBars?` (điện thoại). Hook `useScrollTop(ref)` trả `{visible, progress}`; theo dõi bằng `scroll` có `passive: true` và gộp theo `requestAnimationFrame`.
- Ngưỡng: chỉ gắn khi `scrollHeight ≥ 3 × clientHeight`; hiện khi `scrollTop > clientHeight`, ẩn khi `< clientHeight / 2`. Artboard dùng 30 % vì bản mẫu ngắn; app dùng đúng số này.
- Bấm: `scrollTo({top: 0, behavior: 'smooth'})`. Xa hơn 6 màn hình thì đặt `scrollTop = clientHeight` trước. `prefers-reduced-motion` thì `behavior: 'auto'`. Sau đó `focus({preventScroll: true})` vào tiêu đề.
- Gắn vào: Trình đọc (có vòng tiến độ), Đọc trên điện thoại (có vòng, ẩn cùng thanh khi cuộn xuống, đổi vị trí theo nút Duyệt), Dự án & truyện (nội dung tab), Audio (danh sách đoạn), Nhật ký tiến độ.
- Ẩn khi bottom sheet, bàn phím hoặc menu đang mở. Không gắn ở luồng Bàn viết và nhật ký điện thoại.
- Đổi chương ở Trình đọc thì vùng cuộn về đầu và nút ẩn ngay, không chạy hiệu ứng.

### Nghiệm thu

- Chương 3.000 chữ ở cỡ chữ 18: cuộn qua 1 màn hình thì nút hiện, vòng tiến độ khớp thanh tiến độ trên cùng (điện thoại); bấm thì về đầu và focus nằm ở tiêu đề.
- Điện thoại: cuộn xuống thì nút ẩn cùng thanh; cuộn lên thì nút hiện trên thanh dưới, không đè nút “Duyệt & viết tiếp”.
- Bật giảm chuyển động: không cuộn mượt, không có hiệu ứng hiện/ẩn.
- Tab: khi nút ẩn, phím Tab không dừng ở nút.
- Ghi “Gói T” vào `19-gap-closure.md`.

## Gói U — Ngăn kéo Não truyện dùng chung đồ thị với màn Não truyện (28/10 · P1) <!-- goi-u -->

**Vì sao:** đồ thị trong ngăn kéo ở Bàn viết đang là một bản vẽ riêng (vòng tròn rỗng, đường thẳng, nhãn to, hàng ô chương), trông kém và khác hẳn màn Não truyện. Dữ liệu cũng lệch: ngăn kéo hiện quan hệ “Đồng phạm · chưa lộ” ở chương 4, trong khi Não truyện chỉ hiện từ chương 5. Lỗi gốc nằm ở artboard cũ; artboard đã sửa. Artboard: `Main.dc.html` (`drawer=brain`) và `Brain.dc.html` giờ dùng chung một bộ vẽ.

### Việc FE

- **Một component `StoryGraph`** cho cả hai nơi; ngăn kéo chỉ là cách dùng gọn (`compact`). Không được có bản vẽ đồ thị thứ hai.
  - Nút: thẻ bo tròn (avatar 30 px + tên + vai; vật/sự kiện/địa điểm: icon + tên).
  - Cạnh: đường cong nối mép thẻ, cong ra xa tâm, màu nhạt dần hai đầu. Nhãn chỉ hiện ở cạnh nối với mục đang chọn hoặc đang rê, có chấm màu, tự né thẻ và né nhau.
  - Mục đang chọn: viền màu loại + quầng sáng; mục không liên quan mờ 50 %, nền thẻ vẫn đặc.
  - Avatar trong thẻ theo Gói S: màu theo loại vai, huy hiệu góc, viền nét đứt khi chưa lộ.
  - Rê để xem quan hệ, bấm để chọn, Enter/Space trên thẻ đang focus.
- **Ngăn kéo `compact`:** không có cột lọc, ô tìm, chế độ danh sách và nút phóng to.
  - Góc trên: “Tại chương N · X thực thể · Y quan hệ”.
  - Góc dưới trái: chú giải loại vai một dòng (Chính · Phản diện · Đồng minh · Phụ · Chưa lộ).
  - Góc dưới phải: “Rê để xem quan hệ · bấm để chọn”.
  - Bỏ hai hàng chú giải cũ ở trên đồ thị.
- **Dòng thời gian chương:** dùng lại dòng thời gian của Não truyện (chấm xanh = đã viết, tím = đang viết, rỗng = chưa viết; nút ‹ ›; tên chương; “Đang viết lại chương này”). Bỏ hàng ô 1…8.
- **Bảng chi tiết (320 px)** dùng lại bảng của Não truyện ở chế độ chỉ xem:
  - avatar 40 px, chip loại vai, “có từ chương N”, mô tả;
  - thẻ “Quan hệ tại chương N” (vạch theo kiểu đường, “mới ở chương N”);
  - thẻ “Xưng hô tại chương N” (có dòng “Đổi ở chương…” khi có);
  - thẻ “Diễn biến” (5 mục, mục của chương sau mờ đi);
  - ghi chú khoá.
  - Bấm vào vật/sự kiện cũng xem được chi tiết.
- **Ngăn kéo rộng** `min(1180px, 84vw)` để đồ thị hiện gần đúng cỡ 1:1.
- **Một nguồn dữ liệu:** ngăn kéo, danh sách Nhân vật, thẻ rê và thẻ “Đổi ở chương N” ở tab Truyện đều đọc từ `GET …/brain` (GAP-39) tại chương đang xem. Không giữ danh sách quan hệ hay xưng hô riêng ở Bàn viết.
  - “Đổi ở chương N” = các quan hệ bắt đầu hoặc đổi ở chương N có dính tới nhân vật (tối đa 3 dòng, “Đổi” trước “Mới”).

### Nghiệm thu

- Chụp 1440 ngăn kéo (chọn Tống Nhược Vy, rồi Phùng Chấn) và màn Não truyện cùng chương 4: thẻ, cạnh, nhãn, màu giống nhau; khác nhau chỉ ở phần khung bỏ đi.
- Chương 4: không còn cạnh “Đồng phạm · chưa lộ” giữa Phùng Chấn và Nhược Vy; Phùng Chấn có viền nét đứt và cạnh “Ép xoá bản live” tới Tri Hạ.
- Tab Truyện: thẻ rê Nhược Vy ghi xưng hô “gọi Tri Hạ “em”, tự xưng “chị””, giống Não truyện.
- Ghi “Gói U” vào `19-gap-closure.md`.

## Gói V — Dòng thời gian chương ở Phòng thu cho tập dài (28/10 · P1) <!-- goi-v -->

**Vì sao:** tập 16 chương chỉ mới tạo chương 1 thì ô chương 1 chiếm gần hết chiều ngang, 15 chương còn lại bẹp thành ô nét đứt, số “10”–“16” bị cắt. Dòng tổng cũng sai (“0 giờ 07 phút / 0 giờ 07 phút”). Nguyên nhân: chương chưa tạo bị tính 0 giây. Artboard `AudioExport.dc.html`, tweak mới `len`: 8 · 16 · 40 · 120. Spec: `10-audio.md` › A4 › *Dòng thời gian chương*.

### Việc FE

- Bề rộng mỗi chương = `sec ?? estSec`, không bao giờ 0.
- Chọn chế độ theo bề rộng trung bình mỗi chương, đo bằng `ResizeObserver`: ≥ 72 px đầy đủ · 28–71 px chỉ số · < 28 px dải màu.
- Chế độ dải màu có thước số (bước 5 / 10 / 25), nhãn tím ở chương đang tạo và một dòng chú giải.
- Thẻ nhỏ khi rê hoặc focus; cả dải là một điểm dừng Tab, ← → Home End để chuyển chương, Enter để nghe.
- Dòng tổng: “đã tạo X / khoảng Y · a/b chương”.

### Việc backend

- Hàng đợi tạo giọng trả `sec`, `estSec`, `state`, `pct` cho từng chương, và `madeSec`, `estTotalSec` (xem `06-data-contract.md` › *Bổ sung §V*).

### Nghiệm thu

- Chụp 1440 với tập 8, 16, 40 và 120 chương; so với artboard (`len`).
- 16 chương, mới xong chương 1: các ô rộng gần bằng nhau, không ô nào bị cắt số.
- Thu cửa sổ còn 1024 px: tự chuyển chế độ, không tràn ngang.
- Ghi “Gói V” vào `19-gap-closure.md`.

## Gói W — Tuỳ chỉnh › Trợ lý AI: ô chọn model và ngữ cảnh (28/10 · P1) <!-- goi-w -->

**Vì sao:**
- Ô chọn model xén chữ giữa ký tự (“gpt-6.1-sol · viết hay nhấ”) vì nút của `KitSelect` không cắt “…”.
- Trang không cho thấy, cũng không cho đặt **ngữ cảnh** của model. Đây là số quyết định khi nào trợ lý phải tóm tắt phần cũ, và hiện chỉ sửa được ở Nâng cao.

Artboard `Settings.dc.html`, `section=models`, tweak mới `ai`:
- `none` · `model` (danh sách model mở);
- `ctx` · `custom` (popover ngữ cảnh, tự nhận / tự đặt);
- `fallback` (model không nhận ra, đang dùng mức dự phòng).

Spec: `04-components.md` › *Ô chọn: chữ dài phải cắt “…”*, `20-settings-rules.md` (dòng Ngữ cảnh).

### Việc FE

- **Sửa `KitSelect`** cho mọi nơi dùng: nhãn cắt “…”, mũi tên không co, `title` đủ nhãn.
- **Mỗi trợ lý một khối hai dòng:**
  - dòng 1: orb, tên, việc làm, chip phạm vi (Theo dự án / Riêng truyện này);
  - dòng 2: ba ô có nhãn nhỏ — **Model** (rộng nhất) · **Mức suy nghĩ** · **Ngữ cảnh** (176 px).
- **Ô Model:** tên model + mô tả xám (chỉ mô tả bị cắt). Danh sách: tên, mô tả, chip ngữ cảnh, tick; cuối có “Thêm nhà cung cấp ở Kết nối AI”. *(Thay bằng Gói AH: nhóm theo nhà cung cấp, dòng cuối “Chọn thêm model hoặc nhà cung cấp ở Kết nối AI”.)* <!-- ah-old -->
- **Ô Ngữ cảnh:** “272K · tự nhận” / “256K · bạn đặt” / “⚠ 128K · dự phòng” (viền cam). Bấm mở popover:
  - mô tả một câu;
  - cảnh báo cam khi đang dùng mức dự phòng;
  - hai lựa chọn: **Tự nhận · N** (hoặc **Mức dự phòng · N**) và **Tự đặt** (ô số đơn vị “nghìn”, chip 32K · 64K · 128K · 200K · 1M, kiểm tra 8–2.000);
  - dòng “Trợ lý sẽ tự tóm tắt phần cũ khi dùng tới khoảng X” (X = ngữ cảnh − phần chừa sẵn);
  - chân: “Áp dụng cho model này ở mọi trợ lý và mọi truyện. Chỉ quản trị đổi được.” + Huỷ / Lưu.
  - Người không phải quản trị: popover chỉ xem, không có Lưu.
- Popover và danh sách ở hàng gần đáy thì mở lên trên. Esc đóng.
- Đổi model thì ô Ngữ cảnh cập nhật theo model mới.

### Việc backend

- Danh mục model trả `contextWindow`, `contextSource`, `registryWindow`; cho quản trị ghi `context_window` theo model (xem `06-data-contract.md` › *Bổ sung §W*). Ô ở Nâng cao vẫn giữ, cùng một giá trị.

### Nghiệm thu

- Chụp 1440 bốn trạng thái `ai` (`none`, `model`, `custom`, `fallback`); không ô nào xén chữ.
- Đặt 256K cho một model: mọi trợ lý dùng model đó hiện “256K · bạn đặt”; nhật ký ngữ cảnh của trợ lý báo cửa sổ mới.
- Chọn “Tự nhận” thì xoá giá trị riêng, quay về số của danh mục.
- Ghi “Gói W” vào `19-gap-closure.md`.

### Bổ sung: cùng ô Ngữ cảnh ở bảng `/model` và trên điện thoại <!-- goi-w2 -->

Ngữ cảnh phải chỉnh được ở **cả ba nơi** chọn model, cùng một giá trị (theo model):

- **Tuỳ chỉnh › Trợ lý AI** (máy tính): như trên.
- **Bảng `/model`** (artboard `CmdResults`, `view=modelCtx`): mục “Ngữ cảnh của {model}” nằm giữa “Model” và “Mức suy luận”.
  - Có hai nút **Tự nhận · 400K** / **Tự đặt**. Tự đặt hiện ô số “nghìn” và các mức nhanh 32K · 64K · 128K · 200K · 1M, kèm dòng mốc tự tóm tắt.
  - Thay đổi chờ đến khi bấm “Lưu thay đổi”; chân bảng ghi “Ngữ cảnh 1 model có thay đổi chưa lưu”.
  - Chip “ngữ cảnh …” trên từng dòng model đổi theo.
- **Điện thoại › Thêm › Model trợ lý › {trợ lý}** (artboard `MMore`, `state=pickCtx`): nhóm “Ngữ cảnh của {model}” trước “Mức suy luận”.
  - Hai dòng chọn: Tự nhận · N / Tự đặt (dòng phụ “Đang dùng 256K”).
  - Chọn Tự đặt thì hiện ô số + hàng chip cuộn ngang. Lưu ngay như các lựa chọn khác trên điện thoại.
- **Số liệu thống nhất** từ danh mục model: gpt-6.1-sol 400K · gpt-6-luna 400K · gpt-5.6-terra 200K · claude-3.5-sonnet 200K · gemini 1M · deepseek-chat 128K. Viết hoa “K”, không viết “400k”.

## Gói X — Sửa Nhật ký hoạt động trên điện thoại (28/10 · P0) <!-- goi-x -->

Design rà ảnh chụp thật (tập 1, chương 38–41) so với artboard `MLog.dc.html` (`state`: live · tool · review · scrolled).

| # | Lỗi | Sửa |
|---|---|---|
| X1 | Hàng chương đã thu gọn (“Chương 40 · Mỗi giọng…”) bị lệch: icon sách trôi giữa màn, tên bị đẩy sang phải và cắt sớm, bên trái bỏ trống. | Bố cục như artboard: icon 18 px sát lề trái (16 px) · khối tên (`flex: 1; min-width: 0`) · mũi tên sát phải. Tên một dòng cắt “…”, dòng phụ dưới. |
| X2 | “Chương 38” đang thu gọn nhưng mũi tên chỉ lên. | Mũi tên xuống = đang gọn, lên = đang mở; `aria-expanded` đúng theo trạng thái. |
| X3 | Câu sự kiện in thẳng tên công cụ và tham số: “gọi Đọc bối cảnh, Lưu thẩm duyệt dùng scope=arc, chapter=40”. | Câu chính chỉ nói việc: “Thẩm duyệt cung 2 của tập 1 · chương 33–40”. Mỗi công cụ là một thẻ gập riêng như artboard (“Đọc bối cảnh · 1,2 giây”, “Lưu thẩm duyệt”). Tham số chỉ hiện khi mở thẻ, bằng nhãn tiếng Việt (“Phạm vi: cả cung · Đến chương: 40”). Không có `key=value` ngoài Nâng cao (nguyên tắc 7). |
| X4 | Dải chương dùng gạch nối “33-40”. | Dùng gạch ngang “33–40”, như mọi chỗ khác. |
| X5 | Nhóm “Chương 41 · Đang viết” chứa “Viết lại chương 40”, “Viết lại chương 39”, “Viết chương 40” và thẩm duyệt cung 33–40, nên người đọc tưởng đó là việc của chương 41. | Xếp sự kiện vào đúng chương nó nói tới (trường `chapter` của sự kiện). Việc nhiều chương (thẩm duyệt cung) có nhóm riêng “Cung 2 · chương 33–40”, đặt theo thời gian. Nhóm đang chạy lấy theo việc đang làm thật: “Chương 40 · Đang viết lại”. |
| X6 | Đầu nhóm đang chạy chỉ ghi “Chương 41”, thiếu tên. | “Chương N · tên chương” như artboard. Chưa có tên thì “Chương 41 · chưa đặt tên”. |
| X7 | Tiêu đề “Nhật ký hoạt động” bị mờ nhoè (hiệu ứng làm mờ áp lên chữ). | Chỉ làm mờ nền thanh trên (`backdrop-filter` trên nền), chữ tiêu đề luôn nét. |
| X8 | “8 điểm · 24 sự kiện” không rõ điểm gì. | “Biên tập 8,0 · 24 sự kiện”: luôn một chữ số thập phân, có chữ “Biên tập”. Chương chưa chấm thì bỏ phần điểm. |
| X9 | Đầu nhóm dính trên cùng cắt ngang dòng chữ bên dưới. | Đầu nhóm dính có nền đặc `#0B0B0D`, viền dưới 1 px, và dải mờ dần 12 px phía dưới. |
| X10 | Hàng bộ lọc bị cắt ngang ở mép phải (“Biên tập”) mà không có dấu hiệu cuộn được. | Mép phải hàng chip có dải mờ dần 24 px. Chip đang chọn tự cuộn vào giữa. |

### Nghiệm thu

- Chụp 390 px cùng dữ liệu tập 1 (chương 38–41), so với artboard `MLog`.
- Tìm “=” trong toàn bộ nhật ký (trừ Nâng cao): không còn kết quả.
- Ghi “Gói X” vào `19-gap-closure.md`.

## Gói Y — Thẻ trợ lý trên màn Viết (điện thoại): làm rõ ai đang chạy (28/10 · P1) <!-- goi-y -->

**Vì sao:** trên hàng thẻ trợ lý, thẻ đang chạy chỉ khác thẻ đang chờ ở một viền mờ. Khi hai trợ lý chạy song song (Người viết viết chương mới, Biên tập viên soát chương trước), rất khó nhìn ra. Artboard `MWork.dc.html`: `state=running` (2 trợ lý chạy), `gate` (chờ bạn duyệt), `idle`.

### Chốt (28/10, bản 3) <!-- goi-y-v3 -->

**Giữ thẻ như thiết kế ban đầu:**
- viền màu trợ lý 35 % và nền ánh màu từ trái ở thẻ đang chạy;
- các thẻ khác giữ viền xám;
- chữ trạng thái của thẻ đang chạy theo màu trợ lý.

Bỏ cả hai bản thử trước: bản viền xoay, vạch nhịp, chữ ánh chạy, và bản vệt sáng dưới chữ.

**Chỉ thêm chuyển động cho orb của trợ lý đang chạy** (cùng nhịp 3,2 s):

| Lớp | Chuyển động |
|---|---|
| Mặt cầu | thở + nổi nhẹ: `scale 1 → 1,09`, `translateY 0 → −1,5 px`, `ease-in-out` |
| Quầng sau orb | vùng sáng mờ màu trợ lý, rộng hơn orb 7 px mỗi phía, `opacity .35 → .85`, `scale .9 → 1,08` |
| Điểm sáng trên mặt cầu | đốm trắng mờ ở góc trên trái, lắc qua lại quanh tâm −25° → 35° (6 s), như ánh đèn trượt trên quả cầu |

- **Chữ hành động** (dòng dưới tên, vd. “Soát nhất quán”) của trợ lý đang chạy có ánh sáng lướt qua giống nhãn đang chạy ở Bàn viết bản máy tính (`.shimmer`). <!-- goi-y-chu -->
  - Cùng nhịp 2,2 s, chạy từ trái sang phải, `background-size: 250%`, đoạn sáng trắng 45–55 %.
  - Nền chữ là màu trợ lý pha 28 % trắng (`color-mix(in srgb, rgb(var(--rgb)) 72%, #fff)`), nên vẫn đọc ra màu của trợ lý.
  - Đổi bước thì đổi chữ ngay, ánh sáng chạy tiếp, không reset.
  - Giảm chuyển động: chữ đứng yên, giữ màu trợ lý.
- Thẻ rảnh: orb đứng yên như cũ.
- Chỉ animate `transform` và `opacity`.
- Giảm chuyển động: orb đứng yên, quầng giữ ở 50 %.
- Đọc màn hình: `aria-label` “Người viết · Viết bản nháp · đang chạy”.
- Máy tính: thẻ trợ lý ở Bàn viết giữ kiểu đã chốt.

### Nghiệm thu

- Quay màn hình 5 giây ở `running`: chỉ orb của trợ lý đang chạy chuyển động (thở, nổi, quầng, đốm sáng); thẻ không có hiệu ứng khác.
- Bật giảm chuyển động: không còn gì chuyển động trên hàng thẻ.
- Ghi “Gói Y” vào `19-gap-closure.md`.

## Gói Z — Điện thoại › Nghe: quản lý tạo giọng và xuất thử (28/10 · P1) <!-- goi-z -->

**Vì sao:** trên điện thoại chỉ thấy “Đang tạo chương 10 · 46%”, không tạm dừng, thử lại, đổi chất lượng hay xem hàng đợi được. Ghép & xuất cũng không có cách kiểm tra trước khi xuất cả tập. Artboard `MAudio.dc.html`, `state` mới: `queue` · `queuePaused` · `queueError` · `exportRun` · `exportTrial`. Spec: `18-mobile.md` › *M3 bổ sung 28/10*.

### Việc FE

- Thẻ tổng thành nút mở sheet **Tạo giọng**. Dòng trạng thái đổi theo chạy / tạm dừng / lỗi.
- **Sheet Tạo giọng:**
  - thẻ đang chạy (avatar có sóng, chữ ánh lướt, % và thanh tiến độ);
  - *Tạm dừng* / *Tiếp tục* / *Thử lại* / *Thử với “Nghe thử nhanh”*; chip *Báo khi xong*;
  - hàng đợi theo chương (đã có giọng gộp lại, có *Tạo lại*; đang tạo; chờ; chưa chốt);
  - chọn chất lượng cho chương chưa tạo;
  - công tắc *Tự tạo giọng khi chốt chương*.
- **Ghép & xuất:** nút *Xuất thử chương N trước*. Bản thử gọi cùng API xuất với `chapters: [N]`, tên tệp thêm `ban-thu`. Màn xong có *Nghe bản thử* / *Xuất cả tập*.
- Lý do lỗi viết bằng lời thường, không in mã lỗi hay đường dẫn (nguyên tắc 7).

### Việc backend (đã có trong GAP-19/20, kiểm tra đủ trường)

- `GET S/audio/queue`: mỗi chương `state` (`done` · `rendering` · `queued` · `unlocked` · `error`), `pct`, `segment`, `segments`, `etaSec`, `quality`; lỗi kèm `reason` (câu tiếng Việt), `at`.
- Hàng đợi: `POST …/pause`, `…/resume`, `…/retry {quality?}`, `…/chapters/:n/rerender {quality}`, `…/chapters/:n/prioritize`.
- `PUT S/audio/settings { quality, autoOnLock }`.
- `notify` khi xong theo người dùng.
- Xuất: `POST …/exports { chapters: [n], trial: true }` → tên tệp `tap-1-ban-thu-chuong-n.<đuôi>`; không ghi đè bản xuất cả tập.

### Nghiệm thu

- Chụp 390 px các trạng thái mới, so với artboard.
- Tắt app khi đang tạo rồi mở lại: % tiếp tục đúng, không tạo lại đoạn đã xong.
- Xuất thử chương 4 rồi *Xuất cả tập*: form giữ định dạng và tuỳ chọn vừa chọn.
- Ghi “Gói Z” vào `19-gap-closure.md`.

### Bổ sung Gói Z: chọn chương để tạo giọng trên điện thoại <!-- goi-z2 -->

Artboard `MAudio`, `state` mới: `genPick` · `genPickRedo` · `genVoice`. Spec: `18-mobile.md` › *M3 bổ sung 28/10 (2)*.

- **Lối vào:** nút **Tạo giọng** / **Hàng đợi · N** dưới thẻ tổng; chip **Tạo giọng** ở dòng chương chưa có giọng; **+ Thêm chương** và **Chốt & tạo** trong sheet hàng đợi.
- **Sheet chọn chương:** chọn chương chưa có giọng (kèm nhãn chưa chốt, chỗ cần xem); công tắc tạo lại chương đã có giọng; kiểu giọng (sheet con 3 kiểu + nghe mẫu từng giọng); chất lượng; cảnh báo chốt; thanh dưới có tổng thời gian và nút **Chốt & tạo giọng N chương**.
- **Backend:**
  - `POST S/audio/queue { chapters: [n], quality, voiceMode, lock: true }`: chốt các chương chưa chốt theo bản đọc hiện tại rồi xếp hàng; trả về danh sách chương đã xếp và `etaSec`.
  - Chương có chỗ cần xem vẫn cho chốt; trả `warnings[]` để FE hiện.
  - `GET S/audio/voices?mode=` trả “ai đọc ai” và URL mẫu giọng ngắn (≤ 5 giây) để nghe thử.
  - Tạo lại chương đã có giọng dùng cùng API, không cần `lock`.
- **Nghiệm thu:** trên điện thoại, chọn chương 11–12 → Chốt & tạo → hai chương hiện “Chờ” trong hàng đợi và danh sách; máy tính thấy hai chương đã chốt.

### Bổ sung Gói Z: đổi giọng trên điện thoại <!-- goi-z3 -->

Artboard `MAudio`: `genVoice` · `genVoicePick` · `genVoiceRec`. Spec: `18-mobile.md` › *M3 bổ sung (3)*.

- **FE:**
  - sheet Giọng đọc mở được từ sheet chọn chương và sheet hàng đợi;
  - bấm một vai → danh sách giọng (của bạn / có sẵn), nghe mẫu từng giọng;
  - công tắc đọc lại đoạn cũ; nút “Dùng …”;
  - ghi âm mẫu 10–30 giây hoặc chọn tệp.
  - Dữ liệu giọng phải khớp tab Phân vai bản máy tính (cùng một nguồn).
- **Backend:**
  - `GET S/audio/voices` trả giọng của dự án (mẫu đã tải) và giọng có sẵn, mỗi giọng có `sampleUrl` ngắn;
  - `PUT S/audio/cast/:role { voiceId, rerenderOld: boolean }`: `rerenderOld` thì xếp các đoạn cũ của vai đó vào hàng đợi;
  - `POST …/voices` nhận tệp ghi âm (10–30 giây, `.mp3/.wav/.m4a`), lưu vào Tài sản › Giọng của dự án;
  - lỗi trả câu tiếng Việt (quá ngắn, quá ồn, sai định dạng).
- **Nghiệm thu:** đổi Nhược Vy sang “Nữ trẻ, ấm” trên điện thoại → máy tính thấy cùng giọng ở tab Phân vai. Bật đọc lại thì 7 đoạn cũ vào hàng đợi.

## Gói AA — Xuất lẻ audio từng chương (máy tính + điện thoại) (28/10 · P1) <!-- goi-aa -->

**Vì sao:** người dùng cần gửi duyệt, đăng lẻ hoặc dựng video theo từng chương, không phải lúc nào cũng xuất cả tập. Artboard:
- `AudioExport.dc.html`: `state` mới `confirmCh` · `exportedCh`; `exported` có *Xem 8 tệp*.
- `MAudio.dc.html`: `state` mới `chMenu` · `exportCh` · `exportChDone`.

Spec: `10-audio.md` › *Xuất lẻ từng chương*, `18-mobile.md` › *M3 bổ sung (4)*, `06-data-contract.md` › *Bổ sung §AA*.

### Việc FE

- **Máy tính:**
  - hộp Ghép & xuất thêm mục “Xuất gì” (Cả tập / Chọn chương) và lưới chọn chương;
  - tiêu đề, ô độ dài, chân hộp và nút đổi theo lựa chọn;
  - màn đã xuất lẻ liệt kê từng tệp (▶ · Tải) và *Tải cả N tệp*;
  - màn đã xuất cả tập có *Xem 8 tệp*.
  - lối vào nhanh ở trình nghe <!-- aa-loi-vao -->: nút xuất nhỏ ở từng dòng chương (hiện khi rê chuột/focus, luôn hiện ở chương đang nghe) mở hộp *Chọn chương* với đúng chương đó; nút phụ **Xuất từng chương** ở chân trình nghe;
- **Điện thoại:**
  - nút ⋯ ở dòng chương đã có giọng (Nghe · Xuất riêng · Chia sẻ · Tạo lại);
  - sheet xuất có Cả tập / Từng chương, lưới số chương, Từng tệp riêng / Gộp .zip;
  - màn xong có danh sách tệp, chia sẻ từng tệp hoặc tất cả.
- Chỉ chọn được chương đã có giọng. Không chọn chương nào thì nút mờ.

### Việc backend

- `exports` nhận `split`, `pack`, `chapters`; trả `files[]` (+ `zipUrl`). Xuất cả tập trả thêm `chapterFiles[]`.
- Tệp lẻ không kèm nhạc mở/kết và khoảng lặng; vẫn chuẩn hoá âm lượng và đọc tên chương nếu bật.

### Nghiệm thu

- Máy tính: chọn chương 2, 3, 4 → ra 3 tệp đúng tên, đúng thời lượng.
- Điện thoại: ⋯ ở chương 4 → Xuất riêng → một tệp đúng định dạng đã chọn (mặc định .mp3), chia sẻ được qua bảng chia sẻ của máy.
- Gộp .zip trả một tệp chứa đúng các chương đã chọn.
- Ghi “Gói AA” vào `19-gap-closure.md`.

## Gói AB — Một bộ tuỳ chọn xuất audio cho máy tính và điện thoại (28/10 · P1) <!-- goi-ab -->

**Vì sao:** máy tính có MP3/WAV + 4 công tắc âm thanh, điện thoại lại có M4B/MP3 + mốc chương + .srt. Hợp đồng GAP-20 có đủ cả hai, nên hai nơi phải giống nhau. Artboard: `AudioExport.dc.html` (`confirm`, `confirmCh`, `exported`, `exportedCh`) và `MAudio.dc.html` (`export`, `exportCh`, `exportDone`, `exportChDone`).

Spec: `10-audio.md` › *Tuỳ chọn xuất dùng chung*; `18-mobile.md` › M3; `06-data-contract.md` › *Bổ sung §AB*.

### Việc FE

- Một nguồn dữ liệu tuỳ chọn dùng chung cho hai nơi (định dạng, 6 công tắc, cách nhận tệp; cùng thứ tự, cùng nhãn, cùng mặc định).
- Định dạng: MP3 · M4B · WAV (mặc định MP3) ở cả hai nơi.
- Quy tắc ẩn giống nhau: nhạc mở/kết và khoảng lặng chỉ khi xuất cả tập; mốc chương chỉ khi cả tập + M4B; cách nhận tệp chỉ khi xuất lẻ.
- Máy tính: thêm M4B, mốc chương, .srt, *Cách nhận tệp*; màn đã xuất đổi đuôi, dung lượng theo định dạng; dòng .srt chỉ hiện khi bật.
- Điện thoại: thêm WAV và 4 công tắc âm thanh (gộp trong dòng mở rộng *Âm thanh khi ghép*); sheet cuộn trong khi cao.
- Mở form thì nạp `export-prefs`; bấm xuất thì lưu lại.

### Việc backend

- `GET/PUT S/audio/export-prefs` theo §AB; xuất bằng `POST …/exports` vẫn nhận đủ cờ như GAP-20.

### Nghiệm thu

- Chọn M4B + tắt .srt trên máy tính, xuất → mở sheet trên điện thoại thấy M4B và .srt tắt.
- Đổi sang WAV: không thấy *Mốc chương*; dung lượng ước tính tăng khoảng 8 lần.
- Xuất lẻ: không thấy nhạc mở/kết, khoảng lặng, mốc chương ở cả hai nơi.
- Ghi “Gói AB” vào `19-gap-closure.md`.

## Gói AC — Chuyển Quy tắc viết của truyện / dự án sang tab Dự án (28/10 · P1) <!-- goi-ac -->

**Vì sao:** quy tắc viết gắn với từng truyện; để trong Tuỳ chỉnh (kèm thanh chọn 3 cấp) người dùng phải tìm và dễ sửa nhầm cấp. Artboard: `Outline.dc.html` (`tab=rules`, `rules`: list · understood · thinking · unsure; `view=project`), `Settings.dc.html` (`section=rules`), `MMore.dc.html` (`rules`).

Spec: `20-settings-rules.md` › *3. Quy tắc viết* › *Vị trí — chốt 28/10*.

### Việc FE

- Trang truyện ở tab Dự án: thêm tab **Quy tắc viết** giữa *Chi tiết cài cắm* và *Bản xuất*; số trên tab = quy tắc riêng + học từ góp ý.
- Tách khối quy tắc trong Tuỳ chỉnh thành thành phần dùng chung `RulesEditor` (props: cấp sửa được, danh sách kế thừa, lối sửa cấp trên) và dùng lại ở: tab truyện, trang dự án, Tuỳ chỉnh.
- Trang dự án: mục **Quy tắc viết chung của dự án** (sau mục Truyện) + mục *Quy tắc viết* ở thanh trái cuộn tới đó.
- Tuỳ chỉnh › **Quy tắc chung**: bỏ thanh “Áp dụng cho” ở mục này, chỉ sửa cấp Mọi truyện; thêm thẻ chỉ dẫn sang tab Dự án. Các mục khác giữ 3 cấp.
- Cột phải trang truyện: dòng *Quy tắc viết* mở tab; khi ở tab thì có thẻ *Kiểm tra 3 chương gần nhất*.
- Điện thoại M4 › Quy tắc viết: bỏ nút chia 3, mở thẳng cấp truyện; nhóm kế thừa chạm để sửa cấp đó; nút *Về quy tắc của truyện*.

### Việc backend

- Không đổi API: vẫn `GET/PUT /api/rules/file` theo cấp và `GET /api/rules/effective`.

### Nghiệm thu

- Từ trang truyện, thêm quy tắc ở tab Quy tắc viết → có trong *Noverse hiểu thế nào?* và trong `effective` của truyện; không xuất hiện ở truyện khác.
- *Sửa ở trang dự án ›* mở trang dự án, thêm quy tắc → mọi truyện trong dự án thấy ở nhóm kế thừa.
- Tuỳ chỉnh › Quy tắc chung không còn chọn cấp; thẻ chỉ dẫn mở đúng trang truyện đang mở.
- Ghi “Gói AC” vào `19-gap-closure.md`.

## Gói AD — Thành viên: vai trò theo dự án, thiết bị, nhật ký quản trị (28/10 · P1) <!-- goi-ad -->

**Vì sao:** hộp thoại “Quản lý tài khoản” chỉ có 3 vai trò chung và danh sách truyện; không phù hợp khi quy tắc, cách viết đã theo dự án, và tên vai trò “Người viết” trùng agent. Artboard: `Members.dc.html` (04c); `Settings.dc.html` (mục *Thành viên* ở thanh trái); `Login.dc.html` (menu tài khoản › *Thành viên*, bỏ `state=users`).

Spec: `09-auth-onboard.md` › *F5b. Thành viên*; `06-data-contract.md` › *Bổ sung §AD*.

### Việc FE

- Trang Tuỳ chỉnh › Thành viên (chỉ quản trị): danh sách + chi tiết; nhật ký quản trị ở nút chia *Thành viên | Nhật ký quản trị*.
- Ô chọn vai trò dự án dùng menu của kit (4 mục có mô tả); bảng quyền mở/đóng tại chỗ.
- Đổi quyền gom vào thanh nổi Lưu/Huỷ; khoá/xoá/đặt lại dùng xác nhận của kit, có ô lý do.
- Hộp Thêm thành viên: gợi ý tên đăng nhập bỏ dấu, kiểm trùng khi gõ (debounce), mật khẩu tạm sinh sẵn; màn Đã tạo hiện mật khẩu một lần + *Chép lời nhắn*.
- Đổi tên vai trò ở mọi nơi: “Người viết · Người đọc” (tài khoản) → “Thành viên” + vai trò dự án. Menu chuyển tài khoản hiện “Thành viên · cần nhập mật khẩu”.
- Ẩn/khóa nút theo vai trò dự án ở các màn (ví dụ *Chỉ xem*: không có ô chat Bàn viết, không có Tạo giọng, tab Quy tắc viết chỉ đọc).

### Việc backend

- Mô hình `role` + `projects` và di trú như §AD; kiểm quyền theo từng request.
- Nhật ký quản trị chỉ ghi thêm, API đọc có lọc và phân trang.
- `revoke-all` cho một người; đặt lại mật khẩu thu hồi mọi phiên.

### Nghiệm thu

- Đổi Linh ở “Vân Châu” từ Không vào → Chỉ xem, Lưu: Linh thấy dự án ngay (không đăng nhập lại), mở được truyện nhưng không có ô chat và nút Tạo giọng; nhật ký có dòng “Không vào → Chỉ xem”.
- Không bỏ được quyền / khoá / xoá quản trị cuối cùng (UI tắt + server 409).
- Tạo “Ngọc” trùng tên đăng nhập “linh” → báo lỗi và gợi ý; tạo đúng → mật khẩu tạm hiện một lần, lần đầu vào bắt đổi.
- Di trú dữ liệu cũ: writer có 1 truyện trong “Kênh truyện audio” → Cộng tác ở dự án đó, có dòng nhật ký di trú.
- Ghi “Gói AD” vào `19-gap-closure.md`.

## Gói AE — Ghép & xuất chạy nền, tự tải khi xong (28/10 · P1) <!-- goi-ae -->

**Vì sao:** bản hiện tại giữ người dùng trong hộp “Ghép & xuất” với nút “Đang ghép…” tới khi xong (1–2 phút), không làm gì khác được. Artboard: `AudioExport` (`state`: merging · merged · mergedManual · mergeBlocked · mergeFailed); `MAudio` (`state`: bgRun · bgDone · bgManual · bgBlocked · bgFail; bỏ `exportRun`).

Spec: `10-audio.md` › *Ghép & xuất chạy nền + tự tải*; `18-mobile.md` › *M3 bổ sung (5)*; `14-jobs-dock.md` › *Bổ sung Gói AE*; `06-data-contract.md` › *Bổ sung §AE*.

### Việc FE
- Bấm Ghép & xuất: đóng hộp ngay, tạo việc trong dock (máy tính) / thẻ nổi (điện thoại). Không còn spinner “Đang ghép…” trong hộp.
- Công tắc *Tự tải về máy này khi xong* ở chân hộp (máy tính) và trong sheet (điện thoại), nhớ theo thiết bị.
- Nút chính ở chân trình nghe đổi thành “Đang ghép · N%” khi đang có việc ghép tập này.
- Toast/thẻ kết quả: tự tải (vạch tải → đã tải), tải tay, bị chặn, lỗi — đúng chữ và nút như artboard. Dock nâng lên trên thanh hành động đáy ở trình nghe.
- Điện thoại: trình phát thu nhỏ ghi “Dẫn truyện” thay tên giọng mẫu.

### Việc backend
- `exports` trả `{jobId}` ngay; tiến độ qua SSE `job.*`; kết quả `files[]`/`zipUrl`; lỗi có `chapter`, `retryable`, lời người dùng; `retry` chạy tiếp từ chương lỗi; 409 khi đã có việc ghép cùng tập.

### Nghiệm thu
- Bấm Ghép & xuất → hộp đóng trong < 300 ms, dock hiện “Đang ghép Tập 1”, nghe và đổi màn vẫn chạy.
- Xong với tự tải bật: tệp vào thư mục Tải về của máy đang mở tab, toast “Đã tải về máy này”. Tắt tự tải: toast có **Tải xuống**.
- Trình duyệt chặn tự tải → toast cam, bấm Tải xuống tải được.
- Xoá nhạc mở đầu rồi ghép → dừng ở chương lỗi, Thử lại chạy tiếp, *Ghép không nhạc* xong được.
- Bấm Ghép & xuất lần 2 khi đang ghép → không tạo việc mới.
- Ghi “Gói AE” vào `19-gap-closure.md`.

## Gói AF — Metadata audio ngoài app & tên tệp tải về (28/10 · P1) <!-- goi-af -->

**Vì sao:** bản thật hiện “Chưa chọn giọng” ở chỗ người đọc và bìa chữ “CG” trên màn khoá; tệp tải về tên “Noverse.mp3”, không biết là truyện nào, chương nào. Artboard: `AudioMeta` (07c, mới); `AudioExport`, `MAudio` (tên tệp theo quy tắc mới).

Spec: `10-audio.md` › *A7 · Audio ngoài app*; `06-data-contract.md` › *Bổ sung §AF*.

### Việc FE
- Trình phát web (máy tính, điện thoại): đặt Media Session đủ tên, Noverse Audio, album, bìa nhiều cỡ; cập nhật khi đổi chương; vị trí phát; nút 10 giây và chương trước/sau.
- Mọi chỗ hiện tên tệp dùng quy tắc mới (đã cập nhật trong `AudioExport`, `MAudio`).

### Việc backend
- Bìa: `GET S/cover?size=` trả bìa đã tải hoặc bìa tự tạo.
- Gắn thẻ metadata + bìa + mốc chương vào MP3/M4B/WAV khi xuất.
- `Content-Disposition` có `filename*` UTF-8 và tên dự phòng không dấu; `shortTitle` cho truyện.

### Nghiệm thu
- Nghe chương 4 trên iPhone, khoá máy: “Chương 4 · Bảy mươi ba giây” · “Noverse Audio” · bìa truyện; chương chưa chọn giọng vẫn ghi Noverse Audio.
- Android: thông báo có tên truyện · tập, ⏮ ⏭ đổi chương.
- Truyện chưa có bìa: hiện bìa tự tạo, không có chữ viết tắt.
- Tải chương 4 trên iPhone và máy tính: tệp tên `Trà Xanh Cướp Kịch Bản · Tập 1 · Chương 04 · Bảy mươi ba giây.mp3`; mở trong ứng dụng nhạc thấy đúng tên, người đọc, album, bìa.
- Tệp cả tập M4B mở trong app sách nói có mốc chương.
- Ghi “Gói AF” vào `19-gap-closure.md`.

## Gói AG — Điều hướng màn Dự án: đổi dự án ở thanh trên, đổi truyện ở cột trái (28/10 · P1) <!-- goi-ag -->

**Vì sao:** cột trái đang là cây Dự án → Truyện → Tập → Chương. Truyện nhiều tập, nhiều chương thì cây rất dài, muốn sang truyện khác phải đóng hết. Artboard: `Outline` (`nav`: none · project · story · search; `len`: 8 · 40 · 120).

Spec: `20-settings-rules.md` › *§4 Dự án & truyện*; `02-ia-flows.md` › *F2*; `06-data-contract.md` › *Bổ sung §AG*.

### Việc FE
- Thanh trên: breadcrumb `Tên dự án ▾ / Tên truyện`; popover dự án (danh sách, ✓, Dự án mới). Bỏ nút chọn dự án ở cột trái.
- Cột trái: thẻ truyện + popover truyện (Tổng quan dự án · truyện · Truyện mới). Ở Tổng quan dự án: menu dự án + danh sách truyện gọn. Ở truyện: mục lục của riêng truyện đó.
- Mục lục: tìm chương (bỏ dấu, số chương), *Tới chương đang viết*, *Thu gọn mọi tập*, tiêu đề tập dính, một tập mở một lúc, tập khoá không bấm được, nhớ tập đang mở theo truyện.
- Popover đóng bằng Esc / bấm ra ngoài; điều hướng được bằng phím mũi tên; `aria-expanded` trên nút mở.

### Việc backend
- Không đổi API. Kiểm tra các trường ở §AG đã có; truyện dài thì cho tải chương theo tập.

### Nghiệm thu
- Đang ở chương 37 của truyện 120 chương, bấm thẻ truyện → chọn truyện thứ hai: sang ngay, không cần đóng tập nào.
- Đổi dự án chỉ ở thanh trên; dự án mới mở ở Tổng quan dự án.
- Gõ “o cung” → thấy “Chiếc vỏ ổ cứng”; gõ “37” → chương 37. Không có kết quả → “Không có chương nào khớp “…””.
- Cuộn mục lục Tập 1 dài: tiêu đề Tập 1 dính trên đầu; mở Tập 2 thì Tập 1 tự đóng.
- *Tới chương đang viết* mở đúng tập và tô sáng chương.
- Ghi “Gói AG” vào `19-gap-closure.md`.

## Gói AH — Kết nối AI: chọn model của từng nhà cung cấp (28/10 · P0, lỗi bản thật) <!-- goi-ah -->

**Vì sao:** bản thật thêm nhà cung cấp xong không có cách thêm model. `providers.tsx › addProvider` lưu `models: []`; chỗ thêm model duy nhất là Nâng cao › Tinh chỉnh model › Nhà cung cấp › Đổi › *Thêm model* và phải gõ tay tên (`ProviderEditor.tsx` chỉ hiện combobox khi danh mục > 6 model). Backend `configModels` chỉ lấy danh sách từ xa cho nhà cung cấp gói thuê bao; nhà cung cấp dùng khoá API trả rỗng. Kết quả: ô Model ở Trợ lý AI không có model của nhà cung cấp mới. Artboard: `Settings` (`conn`: list · added · models · search · manual · loading · failed; `section=models`, `ai=model` cho danh sách nhóm theo nhà cung cấp).

Spec: `20-settings-rules.md` › *2.3 Kết nối AI — chọn model*; `06-data-contract.md` › *Bổ sung §AH*.

### Việc FE
- Thẻ nhà cung cấp: nút *Model · n* / *Chọn model*, trạng thái cảnh báo khi 0 model, bảng chọn model trong thẻ (tìm, lọc, ô chọn, ngữ cảnh, giá, nhãn trợ lý đang dùng, khoá bỏ chọn khi đang dùng).
- Thêm nhà cung cấp xong: tự mở bảng ở *Gợi ý*.
- *Thêm theo tên*: gọi `/api/config/test` với `model`, ok thì thêm và chọn.
- Đang lấy / lỗi (*Đổi khoá* · *Thử lại* · *Thêm theo tên*) / không có API danh sách.
- Chọn/bỏ ghi vào bản nháp `providers.<id>.models`; thanh Lưu chung ghi “ở Kết nối AI · dùng cho cả máy chủ”.
- Trợ lý AI: ô Model nhóm theo nhà cung cấp; nhóm rỗng có “Chưa chọn model · Chọn” mở đúng bảng.
- Nâng cao › Tinh chỉnh model: bỏ *Thêm model*, chỉ sửa phần kỹ thuật của model đã chọn.
- Danh sách > 50 model: chỉ render phần nhìn thấy (ảo hoá), tìm ở máy khách.

### Việc backend
- `POST /api/config/models/list` theo §AH (dùng khoá trong bản nháp, cache, lỗi bằng lời, `unsupported`).
- 409 khi lưu cấu hình bỏ model đang được trợ lý dùng.

### Nghiệm thu
- Thêm OpenRouter, dán khoá → bảng tự mở ở *Gợi ý*; chọn 2 model → *Lưu* → Trợ lý AI › Người viết thấy nhóm OpenRouter với đúng 2 model.
- Gõ “qwen” chỉ còn model qwen; gõ tên không có → “Không có model nào khớp …”.
- Khoá sai → báo từ chối khoá + *Đổi khoá*; *Thêm theo tên* vẫn dùng được.
- Bỏ chọn model đang dùng cho Người viết → không bỏ được, có lời giải thích.
- Ollama: danh sách lấy từ máy chủ, cột giá ghi “miễn phí”. ChatGPT (gói thuê bao) vẫn lấy danh sách như cũ, giá ghi “theo gói”.
- Nhà cung cấp mới chưa chọn model: viền vàng + “trợ lý chưa dùng được”; Trợ lý AI hiện nhóm rỗng với nút *Chọn*.
- Ghi “Gói AH” vào `19-gap-closure.md`.

## Gói AI — Vòng đời góp ý theo đoạn & góp ý mất vị trí (28/10 · P1) <!-- goi-ai -->

**Vì sao:** sau khi Người viết viết lại, góp ý không biết đã được sửa chưa, còn nằm đúng chỗ không; câu trích cũ mất thì góp ý trôi. Bản thật còn hiện “Đoạn N” cho người dùng. Artboard: `Feedback` (03b, `fb`: pending · sent · after · fixed · unsure · relink · history), `MFeedback` (M2b, `fb`: pending · sheet · sent · after · afterSheet · fixed · unsure · relink).

Spec: `05-screens.md` › *03b · Góp ý theo đoạn*; `06-data-contract.md` › *Bổ sung §AI*.

### Việc FE
- Máy tính: tab **Góp ý** ở bảng phải (Thẩm duyệt · Góp ý · Xuất), thay khối “Nhận xét” trong tab Thẩm duyệt; nhóm Cần gắn lại → Chờ gửi → Đã gửi → Đã sửa → Đã xử lý.
- Điện thoại: nút nổi “Góp ý · n” trên thanh tab, mở sheet cùng nội dung.
- Đánh dấu trong bài theo trạng thái (gạch chân + chấm lề), “đã dời” nét đứt; bấm chỗ đánh dấu mở đúng thẻ.
- Thẻ Đã sửa: Trước/Sau, *Ổn* · *Chưa ổn*. Thẻ Mất vị trí: *Gắn lại* · *Gửi lại* · *Xoá* (hoàn tác 5 giây như hiện có). Thẻ Không chắc vị trí: gợi ý + *Gắn vào đây*.
- Chế độ gắn lại (dải vàng, chọn đoạn hoặc câu).
- Dải báo sau khi viết lại + toast; số trên tab, nút nổi, danh sách chương.
- Bỏ mọi “Đoạn N”: `ReaderComments` (“@tác giả · Đoạn N”, “Đang nhận xét đoạn N”), `CommentSheet` (“Góp ý đoạn N”).

### Việc backend
- Trường và luật gắn lại ở §AI; chạy sau mỗi lần ghi chương (viết lại, gọt giũa, sửa tay).
- `Compose` gửi kèm id góp ý; công cụ lưu chương của Người viết nhận `fixed_comments`.
- `PATCH` thao tác góp ý; tự `fixed→done` ở lần duyệt kế tiếp; sự kiện `comments.reanchored`.

### Nghiệm thu
- Góp ý 5 chỗ → Duyệt & gửi → thẻ Đã gửi, bài mờ + dải “đang sửa” → xong: dải “2 đã sửa · 1 chưa sửa · 2 cần gắn lại”, toast nếu đang mở chương.
- Bấm “2 cần gắn lại” mở đúng nhóm; máy tính và điện thoại hiện cùng góp ý ở cùng đoạn.
- Không chắc vị trí: mở thẻ thấy 2 đoạn viền vàng “Gợi ý 1/2”; *Gắn vào đây* → Chờ gửi, “Bạn đã gắn lại”.
- Mất vị trí → *Gắn lại* → chọn câu → Chờ gửi; *Gửi lại* → gửi cho cả chương kèm câu cũ.
- Đã sửa → *Chưa ổn* → Chờ gửi với nội dung cũ; không bấm gì → duyệt chương 9 thì thành Đã xử lý.
- Không còn chữ “Đoạn N” ở bất kỳ đâu.
- Ghi “Gói AI” vào `19-gap-closure.md`.

## Gói AJ — iOS 26: thanh trên bị nhoè khi mở từ màn hình chính (28/10 · P1, lỗi bản thật) <!-- goi-aj -->

**Vì sao:** iOS/iPadOS 26 vẽ lớp mờ dần ~40 pt dưới thanh trạng thái của ứng dụng web đã cài; tên truyện ở thanh trên nằm trong dải đó nên bị nhoè. Không tắt được bằng CSS/meta, chỉ né được.

Spec: `18-mobile.md` › *Dải mờ trên cùng iOS 26*.

### Việc FE
- Thêm `--m-edge` (40px khi là ứng dụng đã cài trên iOS, dọc; iPad cả ngang) và cộng vào `--m-top`.
- Đổi 5 chỗ dùng thẳng `env(safe-area-inset-top)` sang `--m-top`.
- Đặt `html[data-standalone]` khi `navigator.standalone` là true (dự phòng khi `display-mode` không khớp).
- Mở rộng `safeArea.test.ts`.

### Việc backend
- Không có.

### Nghiệm thu
- iPhone iOS 26, mở từ màn hình chính: tên truyện, chuông, vòng việc chạy nền rõ nét ở Bàn viết, Đọc, Nghe, Thêm, Thông báo, các sheet toàn màn.
- Safari thường và Android: thanh trên ở vị trí cũ.
- iPhone nằm ngang: không có khoảng trống thừa.
- Ghi “Gói AJ” vào `19-gap-closure.md`.

## Gói AK — Tạo giọng: một ngăn, cập nhật chương đã sửa (29/10 · P1, lỗi bản thật) <!-- goi-ak -->

**Vì sao:** bản thật có hai sheet chồng việc (Hàng đợi + *Thêm chương* → sheet chọn chương có công tắc tạo lại), hai kiểu chọn chất lượng, thẻ cảnh báo “Chương này đã được sửa sau khi chốt” không có nút, “Chưa ước tính” lặp 3 lần, nút ở trang Nghe và nút trong sheet khác chữ. Artboard: `MVoiceGen` (M3b, `gk`), `AudioExport` (`upd`).

Spec: `10-audio.md` › *A8*; `06-data-contract.md` › *Bổ sung §AK*.

### Việc FE
- Điện thoại: gộp `GenerationSheet` + `ChapterPickSheet` thành một ngăn: dải trạng thái → Cần làm (chọn sẵn) → Đang tạo → Đã có giọng (thu gọn) → Cách đọc (một ô chất lượng) → công tắc tự tạo → chân ngăn một nút.
- Một hàm đặt chữ nút chính dùng chung cho thẻ trang Nghe, chân ngăn và thanh Phòng thu (*Cập nhật n chương* / *Tạo giọng n chương*). Bỏ chữ “Chốt & tạo” trên nút.
- Thẻ cảnh báo chương đã sửa → hàng trong nhóm Cần làm, chọn sẵn.
- Máy tính: ô “đã sửa” và thanh *Cập nhật n chương* trên dải chương Phòng thu.

### Việc backend
- `redo?: number[]`; cập nhật chỉ đọc lại đoạn đổi; `updateEtaSec`; job `kind`, `position` (§AK).

### Nghiệm thu
- Sửa chương 1 trên Bàn viết, sửa bản đọc chương 4 trên máy tính → trang Nghe hiện “Cập nhật 2 chương”; mở ngăn thấy 2 hàng vàng chọn sẵn, nút cùng chữ; bấm → 2 chương vào Đang tạo, nhãn “Đang cập nhật”, chỉ đọc lại đoạn đổi.
- Có thêm chương chưa có giọng → nút thành “Tạo giọng 4 chương”, tóm tắt “2 cập nhật · 2 mới”.
- Đã có giọng → *Tạo lại* → hàng có nhãn “Tạo lại từ đầu”, tóm tắt thêm “1 tạo lại từ đầu”.
- Lỗi → hàng đỏ + *Thử lại*; hàng chờ có *Làm trước*.
- Máy tính: ô chương đã sửa viền vàng, thanh *Cập nhật n chương*; bấm → “chờ cập nhật”.
- Ghi “Gói AK” vào `19-gap-closure.md`.

## Gói AL — Cách viết chuyển vào Dự án & truyện (29/10 · P1) <!-- goi-al -->

**Vì sao:** văn phong và văn mẫu là của từng truyện, nhưng đang nằm ở Tuỳ chỉnh › Cách viết với thanh 3 cấp — giống lý do đã chuyển Quy tắc viết (Gói AC). Artboard: `Outline` (`tab=style`, `wstyle`: pack · builtin · inherit · samples · proj), `Settings` (`section=writing`, `scope`).

Spec: `20-settings-rules.md` › *Cách viết — vị trí*; `06-data-contract.md` › *Bổ sung §AL*.

### Việc FE
- Tab **Cách viết** của truyện (sau Quy tắc viết): thanh kế thừa + *Dùng theo dự án*, lưới Gói thể loại + Kiểu viết (dùng lại thành phần của `writing.tsx`), thẻ “Gói … gồm”, khối văn mẫu (chuyển `StyleSamples.tsx` sang đây).
- Trang dự án: mục **Cách viết mặc định của dự án** + lối tắt ở thanh trái; liệt kê truyện chọn riêng.
- Chọn là lưu ngay + toast “áp dụng từ chương N”.
- Thẻ “Đang áp dụng”: dòng Văn phong mở tab/mục mới.
- Tuỳ chỉnh › Cách viết: cấp “Mọi truyện” giữ lưới (“Văn phong chung”), ô “Văn phong của bạn” mờ; cấp truyện/dự án thay lưới + văn mẫu bằng thẻ chỉ dẫn. Duyệt từng chương và Trọng tài hỏi giữ nguyên.

### Việc backend
- Không đổi API (xem §AL).

### Nghiệm thu
- Truyện “Khi sếp học vibecode” › Cách viết: chọn Hồi hộp → toast, thẻ Đang áp dụng đổi, Tuỳ chỉnh cấp truyện hiện thẻ chỉ dẫn “Hồi hộp · riêng truyện này”.
- *Dùng theo dự án* → về văn phong của dự án.
- Thêm văn mẫu trong tab → ô “Văn phong của bạn” chọn được.
- Trang dự án › Cách viết mặc định › Đổi → truyện “Theo dự án” đổi theo, truyện chọn riêng giữ nguyên.
- Tuỳ chỉnh “Mọi truyện” vẫn đổi được văn phong chung.
- Ghi “Gói AL” vào `19-gap-closure.md`.

## Gói AM — Cách viết gọn một chỗ, chọn nhanh, xoá truyện (31/10 · P1) <!-- goi-am -->

**Vì sao:** “Quy tắc viết” và “Cách viết” là hai tab cạnh nhau làm người dùng phân vân; bấm *Văn phong* ở thẻ Đang áp dụng lại chuyển trang; xoá truyện chỉ có ở menu chuột phải trang dự án và bắt gõ tên thư mục. Artboard: `Outline` (`tab=style`, `wstyle`, `quick`, `del`), `MMore` (`rules`, `rulesStyle`, `storyset`), `Settings` (chỉ sửa chữ).

Spec: `20-settings-rules.md` › *Cách viết gọn lại* và *Xoá truyện*; `06-data-contract.md` › *Bổ sung §AM*.

### Việc FE
- **Nhãn nguồn:** trong truyện không ghi “riêng truyện này”; chỉ gắn “theo dự án” / “chung mọi truyện” cho giá trị kế thừa (xem *Nguyên tắc nhãn nguồn*). Lưu ý CSS: đừng đặt class trùng tên với `.rule.inh` (bản mẫu từng bị thẻ quy tắc kế thừa căn giữa vì trùng class). <!-- am-label-fe -->
- Gộp tab Quy tắc viết + Cách viết thành **Cách viết**: thẻ Văn phong gọn (Đổi văn phong mở lưới tại chỗ) → Quy tắc của truyện → Văn mẫu thu gọn. `?tab=rules` cũ chuyển sang `style`.
- Thẻ “Đang áp dụng”: bảng chọn nhanh Văn phong + Người viết, công tắc Duyệt từng chương; Quy tắc viết mở tab. Thẻ dự án: Văn phong (bảng chọn), Duyệt cho truyện mới (công tắc).
- Trang dự án: gộp “Cách viết mặc định” + “Quy tắc của dự án” thành một mục **Cách viết của dự án**; một lối tắt ở thanh trái.
- Nút ⋯ ở đầu trang truyện (Đổi tên · Xuất · Xoá). Hộp xoá: danh sách mất/giữ, *Xuất trước*, ô gõ tên ngắn của truyện, dải “cần dừng trước khi xoá” + *Tạm dừng* khi đang chạy. Dùng lại `confirmDeleteStory`, nhưng đổi `phrase` từ mã thư mục sang tên ngắn hiển thị (nguyên tắc #7). Áp cùng hộp cho menu ở thẻ truyện trang dự án.
- Điện thoại: M4 “Cách viết” gộp văn phong + quy tắc; Tuỳ chỉnh truyện bỏ phần văn phong.
- Tuỳ chỉnh: sửa câu chỉ dẫn (“Mở truyện › tab Cách viết, hoặc trang dự án › Cách viết của dự án”).
- **Màn Tạo truyện mới (`QuickMode.tsx`, lỗi bản thật):** `ConfigChip` *Văn phong* và *model* đang là `<a href=settings…>` → đổi thành nút mở bảng chọn tại chỗ (Theo dự án · Gói thể loại · Kiểu viết; model theo nhà cung cấp), nhãn “riêng” khi khác dự án; lúc bắt đầu thì `createStory` → `putConfig` cấp truyện → chạy (§AM · màn Tạo truyện). Artboard `NewStory` · `quick`. <!-- am-new-fe -->

### Việc backend
- Không có (xem §AM).

### Nghiệm thu
- Truyện có một tab Cách viết; liên kết cũ `?tab=rules` vẫn mở đúng.
- Thẻ Đang áp dụng: đổi văn phong, người viết, duyệt từng chương mà không rời trang; toast đúng.
- Chủ dự án: ⋯ › Xoá truyện → gõ sai tên thì nút khoá, gõ đúng (hoa thường tuỳ ý) → xoá, về trang dự án. Truyện đang chạy → dải vàng, bấm Tạm dừng rồi mới xoá được. Thành viên khác: mục mờ + lý do.
- Điện thoại: Thêm › Cách viết đổi văn phong tại chỗ.
- Màn Tạo truyện mới: bấm thẻ Văn phong không rời trang; chọn Hồi hộp + gpt-6-luna → bắt đầu → truyện mới có văn phong Hồi hộp, người viết gpt-6-luna (Đang áp dụng ghi “riêng”); chọn “Theo dự án” → truyện theo dự án. <!-- am-new-ok -->
- Ghi “Gói AM” vào `19-gap-closure.md`.

## Gói AN — Tuỳ chỉnh › Lưu trữ tệp (S3) (31/10 · P1) <!-- goi-an -->

**Vì sao:** kho tệp chỉ đặt được bằng biến môi trường; quản trị cần chọn kho S3 (Garage trên Mac mini, R2, AWS…), kiểm tra, chuyển tệp an toàn và theo dõi ngay trong app. Lỗi dễ gặp nhất là gửi trực tiếp mà thiếu CORS hoặc địa chỉ kho chỉ dùng trong mạng nội bộ. Artboard: `Settings` (`section=storage`, `storage`, `sprov`, `sfail`), `MMore` (`state=storage`, `storage`).

Spec: `20-settings-rules.md` §8; câu chữ `07-copy.md` › *Lưu trữ tệp*; dữ liệu `06-data-contract.md` › *Bổ sung §AN*.

### Việc FE
- Mục **Lưu trữ tệp** (sau Thành viên) với 14 trạng thái ở §8.2; viên ở đầu app + dòng đỏ khi mất kết nối (§8.1).
- Lưới nhà cung cấp → form theo nhà cung cấp (§8.3), *Lấy ở đâu?* từng bước, khối lệnh Garage, ô Địa chỉ kho cho trình duyệt khi Trực tiếp + địa chỉ nội bộ (cùng luật `privateEndpoint`).
- Kiểm tra 6 bước theo `steps` máy chủ; bước trình duyệt tự tải `corsProbeUrl` rồi phân biệt thiếu CORS / không tới được bằng `no-cors` (§8.4). Đoạn CORS điền sẵn origin hiện tại, 3 thẻ R2/AWS/Garage, *Chép*.
- Lưu → xác nhận số tệp/dung lượng → `PUT` → `POST …/migration` (§8.5); thẻ chuyển với tạm dừng/tiếp tục/huỷ/thử lại; hỏi lại 5 giây khi đang chuyển.
- `envlocked` chỉ đọc + nhãn; `nokey` chặn lưu, lệnh tạo khoá chỉ trong khối chi tiết.
- Điện thoại: nhóm Quản trị + trang chỉ xem (§8.6).
- Không hiện tên biến, mã lỗi hay `key=value` ngoài khối “Chi tiết cho người quản trị máy chủ” và đoạn cấu hình để dán.

### Việc backend (GAP)
- GAP-AN1 trả kho chờ; GAP-AN4 `storyTitle`; GAP-AN5 lệnh dọn kho cũ sau chuyển; GAP-AN6 đổi khoá/cách gửi cùng kho không chuyển; GAP-AN7 loại avatar/backup khỏi Usage; GAP-AN8 kiểm tra kho đang dùng (kể cả `envlocked`). GAP-AN2/AN3 thiết kế đã theo dữ liệu hiện có.

### Nghiệm thu
- Ổ máy chủ → *Chọn kho S3* → Cloudflare R2 → điền → Kiểm tra: đủ 6 bước có ms → *Lưu…* → hộp “Chuyển 1.248 tệp · 3,4 GB sang Cloudflare R2?” → *Bắt đầu chuyển* → thẻ tiến độ + viên ở đầu app; tạm dừng/tiếp tục được; xong → R2 Hoạt động tốt + thẻ Kho cũ.
- R2 chưa đặt CORS → bước Trình duyệt lỗi, đoạn CORS có đúng địa chỉ app; dán, chờ, *Kiểm tra lại* → đạt. *Chuyển sang gửi qua máy chủ* → bước đổi thành “Máy chủ gửi được một đoạn…”.
- Garage `http://garage:3900` + Trực tiếp, để trống Địa chỉ cho trình duyệt → lỗi ở Tạo link ký với hai lối sửa; chọn Qua máy chủ → đạt.
- Sai khoá / sai tên kho / khoá thiếu quyền xoá / sai địa chỉ / lệch giờ → đúng tiêu đề và cách sửa ở §8.4, không lộ mã lỗi.
- Cấu hình bằng biến môi trường → chỉ đọc, nhãn “do máy chủ đặt”, vẫn kiểm tra được, không *Đổi kho…*.
- Thiếu khoá mã hoá → không lưu được, giải thích ngắn, lệnh chỉ trong khối chi tiết.
- Một tệp lỗi khi chuyển → bảng lỗi, chưa đổi kho, *Thử lại các tệp lỗi* chạy tiếp.
- Ngắt kho → khung đỏ + dòng đỏ đầu app (máy tính và điện thoại).
- Điện thoại (quản trị): hàng Lưu trữ tệp đúng trạng thái; trang chỉ xem; không có nút đổi kho.
- Ghi “Gói AN” vào `19-gap-closure.md`.

## 2. Tổng hợp GAP backend của đợt này

| GAP | Gói | Việc |
|---|---|---|
| GAP-30 | F | Web Push + prefs + hộp thư (đối chiếu bản đã có) |
| GAP-31 | F | Lịch sử sự kiện có lọc + chi tiết công cụ |
| GAP-32 | D | understand · effective · apply (giữ runtime, áp từ chương sau, recheck) · runtime keep/delete · lưu violations · trộn hợp danh sách cấm |
| GAP-33 | B | Mã khôi phục quản trị + `POST /api/recover` · bật/tắt LAN từ UI |
| GAP-34 | I | `fs/places` · `fs/list` · `fs/mkdir` · `library/inspect` cho bộ chọn thư mục khi không có hộp chọn của máy |
| GAP-35…40 | L | Văn mẫu trên web · mục thẩm duyệt có `level/ref/quote/fix` · khung truyện & bản đồ giữ chân · xưng hô/quan hệ (khoá, 409 kèm chương bị chạm) · thực thể & cạnh có `phases` · `narration`/`doc-so`/`--keep` qua API (chi tiết `06-data-contract.md`) |
| GAP-41…46 | M | Mã thiết lập & tạo quản trị đầu tiên · ma trận chất lượng · sổ cái 10 trục & luật khoảng cách · vũ trụ chung · tìm trong dự án · bài học hệ thống & mục hay vấp theo gói (chi tiết `06-data-contract.md`) |
| GAP-22…29 | G | Kiểm tra đã đóng chưa |

## 3. Quyết định sản phẩm đã chốt (đừng đổi)

- Tên hiển thị dự án/truyện thay cho slug ở mọi nơi. Slug chỉ ở Nâng cao.
- Máy chỉ giữ **một phiên**; đổi tài khoản = đăng nhập lại.
- Đổi mật khẩu luôn thu hồi các phiên khác.
- Mật khẩu tạm lần đầu: nhắc mạnh nhưng cho *Để sau*.
- Quy tắc mới áp dụng **từ chương kế tiếp**, không sửa chương đã viết; kiểm tra lại chương cũ chỉ báo, không tự sửa.
- Nhóm Hệ thống trong Tuỳ chỉnh và ngăn Nâng cao chỉ dành cho quản trị.
- Không có lệnh `ainovel-cli` nào trên UI. Ngoại lệ duy nhất: trang OAuth `from=cli`.
- Tên trợ lý AI luôn bằng tiếng Việt: Kiến trúc sư · Người viết · Biên tập viên · Trọng tài. Chữ tiếng Việt không dùng font mono.
- Nơi lưu truyện do người dùng chọn; UI không bao giờ hiện tên thư mục trần hay đường dẫn tuyệt đối (trừ tooltip quản trị).

- Bỏ tiếng Trung: chỉ viết tiếng Việt, không còn bước chọn ngôn ngữ (Quyết định 6).
- Mục [LỖI] = Bắt sửa: chương tự vào hàng chờ sửa. Mục [CẢNH BÁO] không chặn chương.
- Esc bấm một lần là đóng lớp trên cùng; chỉ khi không còn lớp nào mới là Tạm dừng.
- Não truyện trên điện thoại chỉ có danh sách, không vẽ đồ thị ở 390 px.
- Quản trị viên đầu tiên tạo trên web bằng mã thiết lập in lúc khởi động. Terminal không hỏi tên hay mật khẩu nữa.
- Bài học của Não hệ thống chỉ thành quy tắc khi quản trị bấm Giữ, và áp dụng từ truyện mới tiếp theo.
- Phép kiểm Bắt sửa của Não dự án chặn bắt đầu viết truyện mới, ngay ở màn Tạo truyện mới.

## 4. Khi xong

Cập nhật `19-gap-closure.md` (GAP nào đã đóng, ở đâu). Mỗi gói ghi một đoạn “Triển khai” ngắn vào tài liệu spec tương ứng, như đã làm ở `18-mobile.md`. Báo lại danh sách ✓/✗ của mọi checklist nghiệm thu.
