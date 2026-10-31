# 18 · Bản điện thoại & máy tính bảng (thiết kế riêng, không bóp từ bản máy tính)

Canvas có 2 trang: **Máy tính** và **Điện thoại**. Trang Điện thoại gồm:

- Hàng 1: 4 tab chính (M1–M4) và `Phone.dc.html` (quét QR đăng nhập).
- Hàng 2: các màn phụ M5–M7 và bản máy tính bảng T1.

Mọi artboard đều bấm được. Điện thoại là 390×844, máy tính bảng là 834×1194.

| Artboard | Màn | Tweak |
|---|---|---|
| `MWork.dc.html` | M1 · Viết | `state`: running · gate · idle · paused · done · error · empty · offline · expired · typing · cmd · cmdArgs · jobs · pause |
| `MReader.dc.html` | M2 · Đọc | `state`: gate · read · immersive · comment · commentKb · chapters · chapterMenu · font |
| `MAudio.dc.html` | M3 · Nghe | `state`: list · player · export · exportDone |
| `MMore.dc.html` | M4 · Thêm | `state`: home · rules · rulesAdd · rulesThinking · storyset · accounts · password · firstPwd · guide · guideCmd · projects · model · pick · devices · confirm |
| `MCmd.dc.html` | M5 · Kết quả lệnh | `view`: cocreate · import · importGuide · importRun · diag · syncCheck · syncDone · reopen · reopenBlocked |
| `MLog.dc.html` | M6 · Nhật ký hoạt động | `state`: live · tool · errors · search · scrolled |
| `MNotify.dc.html` | M7 · Thông báo | `view`: inbox · prompt · system · denied · settings · lock · lockActions |
| `MTablet.dc.html` | T1 · Máy tính bảng | `state`: running · gate · cmd · jobs · readFocus |

**Vai trò của điện thoại: theo dõi & điều khiển.**

- Làm được trên điện thoại:
  - xem agent đang làm gì, duyệt chương, góp ý;
  - chạy lệnh và xem kết quả;
  - nghe và xuất audio;
  - đổi truyện, model, tài khoản, mật khẩu; quản lý thiết bị; chỉnh thông báo.
- Để trên máy tính (điện thoại có ghi chú chỉ sang):
  - dàn ý chi tiết;
  - duyệt bản đọc audio từng đoạn;
  - cài đặt nhà cung cấp và thư viện;
  - tạo truyện, học văn phong.

## Khung chung (`_mbase` trong artboard)

| Phần | Thông số |
|---|---|
| Vùng an toàn trên | 40 px (PWA dùng `env(safe-area-inset-top)`; ứng dụng cài trên iOS/iPadOS 26 cộng thêm dải 40 px chống mờ, xem *Dải mờ trên cùng iOS 26*) <!-- aj-row --> |
| Thanh trên | 52 px. Trái: tên truyện hoặc chương (bấm để đổi). Phải: 1–2 nút 44×44. Màn con: nút ‹ 44×44 + tiêu đề (dòng lệnh mono nhỏ ở trên nếu là kết quả lệnh) |
| Vùng chạm | ≥ 44 px; nút chính cao 50, bo 14; chip cao 34 |
| Cỡ chữ | Thân 15–16. Ô nhập 16 để iOS không tự phóng to. Truyện: Literata 18/1.8, chỉnh được |
| Bottom sheet | Thay cho popover, menu và hộp thoại. Thanh kéo 38×5, bo trên 26. Vào: `translateY(100%) → 0`, 440 ms, `cubic-bezier(.2,.9,.25,1.03)`. Ra: 260 ms. Nền mờ 58%. Bấm nền hoặc kéo xuống để đóng |
| Action sheet | Xác nhận kiểu iOS: khối tiêu đề + mô tả + nút hành động, khối **Huỷ** riêng ở dưới |
| Điều hướng con | Đẩy trang sang phải (`push`, 340 ms), nút ‹ quay lại kèm tên trang trước |
| Toast | Ngay trên thanh tab (hoặc trên thanh hành động), rộng gần cả màn, tự tắt sau 2,4–2,8 s, có thể kèm liên kết *Xem* |
| Nút bấm | Thu nhỏ còn .96 khi nhấn (`:active`) |
| Thanh hành động dưới | Màn con không có thanh tab. Nút chính nằm trong dải `padding 16 16 30`, nền gradient từ trong suốt sang `#0B0B0D` |

### Thanh tab nổi (mới)

Thanh tab là một **viên nang kính nổi**, không dính mép dưới.

- **Vị trí:** cách trái/phải 16, cách đáy 20, cao 64, bo 32.
- **Nền:** `rgba(26,26,29,.74)` + `backdrop-filter: blur(26px) saturate(180%)`, viền 1 px trắng 10%, bóng `0 22px 44px -14px`, ánh sáng mép trên (inset 1 px + gradient 45%).
- **Chỉ báo trượt** (`.tind`): viên tím 15%, viền tím 30%, quầng tím nhẹ.
  - Rộng `(100% − 10px) / 4`, `left = 5px + i × (100% − 10px)/4`.
  - Khi đổi tab: chạy `tind`, scale .7 → 1 với độ nảy `cubic-bezier(.2,.9,.25,1.15)`, 550 ms.
- **Tab:** icon 24 + nhãn 10.5. Tab đang chọn: icon bulk tím `#C9A6F5`, chữ `#F3E8FF`, icon nhích lên 1 px và scale 1.06.
- **Chấm cam 8 px:** có việc cần bạn, ví dụ chương chờ duyệt.
- **3 chế độ** (`tabMode`):
  - mặc định;
  - **`mini`**: khi cuộn xuống. Thu còn .92, ẩn nhãn, dịch xuống 6 px; cuộn lên thì hiện lại;
  - **`hide`**: trượt xuống 110 px và mờ đi. Dùng khi bàn phím mở, khi đọc tập trung, khi phiên hết hạn, và ở màn con có thanh hành động riêng.
- **Phần nổi phía trên** (mini player, ô chat, thanh duyệt) đặt ở `bottom: 92px` để không đè lên thanh tab.

## M1 · Viết (`MWork.dc.html`)

- **Thanh trên:**
  - 3 orb + tên truyện + “Tập 1 · 24/30 chương” → M4 để đổi truyện;
  - **vòng việc nền** + số việc → bottom sheet việc nền, thay dock góc phải của bản máy tính;
  - chuông → M7 (chấm tím khi có thông báo chưa đọc).
- **Dải trợ lý** cuộn ngang: 4 thẻ orb. Thẻ đang chạy có quầng màu agent, orb “thở”, ghi bước hiện tại.
- **Thẻ chính đổi theo trạng thái truyện:**

| Trạng thái | Nội dung | Nút |
|---|---|---|
| running | Chương, bước x/5, thanh gradient Writer, 3 dòng chữ đang viết + con trỏ | *Tạm dừng* (action sheet: sau bước này / dừng ngay / tiếp tục) · *Xem bản nháp* |
| gate | “Chương 24 chờ bạn duyệt”, trích đoạn, điểm | *Đọc chương 24* · *Duyệt & viết tiếp* |
| idle | “Trợ lý đang chờ bạn”, lần duyệt gần nhất | *Bàn hướng đi* · *Viết chương 25* |
| paused | “Đã tạm dừng” + dừng sau bước nào | *Góp ý trước* · *Chạy tiếp* |
| done | “Truyện đã hoàn thành”, orb nảy nhẹ, số chương/chữ | *Viết tiếp tập mới ›* → `/reopen` (M5) |
| error | “Người viết dừng lại”, lời giải thích dễ hiểu (hết hạn mức…), phần đã viết được giữ | *Đổi model* · *Thử lại* |
| empty | Không có truyện: minh hoạ + lời dẫn sang máy tính | *Mở truyện có sẵn* · *Báo tôi khi có truyện mới* |

- **Mất mạng** (`offline`):
  - Băng cam dưới thanh trên: “Mất kết nối · thử lại sau N s” (đếm ngược) + *Thử ngay*.
  - Nội dung mờ 55% và bớt màu: số liệu cũ, không giả vờ đang chạy.
  - Góp ý gửi lúc mất mạng xếp hàng với nhãn **“Chờ gửi · sẽ tự gửi khi có mạng”**.
- **Phiên hết hạn** (`expired`):
  - Sheet đăng nhập không đóng được, thanh tab ẩn.
  - Có câu nhắc “Góp ý đang gõ dở vẫn được giữ” kèm trích nội dung.
  - Đăng nhập bằng tên + mật khẩu, hoặc *Quét mã QR từ máy tính* (→ `Phone.dc.html`).
- **Bàn phím mở** (`typing`):
  - Ô chat dời lên trên bàn phím (khung giữ chỗ 300 px).
  - Thanh tab và hàng chip ẩn; ô nhập lớn tối đa 4 dòng.
  - Triển khai: `visualViewport` resize → đặt `--kb` cho `bottom`.
- **Cuộn:** cuộn xuống thì hàng chip lệnh thu lại (`max-height 0`) và thanh tab về `mini`. Cuộn lên thì hiện lại.
- **Ô chat** nổi trên thanh tab:
  - nút `/` tím bên trái mở sheet lệnh;
  - hàng chip lệnh nhanh;
  - gõ `/…` rồi gửi cũng mở sheet lệnh.
- **Sheet lệnh** — bản điện thoại của `13-commands.md`:
  - **Tiêu đề ngắn** cho điện thoại (`SHORT`), ví dụ “Xuất TXT / EPUB”, “Nhận chỗ sửa tay”, để mỗi dòng không xuống hàng.
  - Ô tìm gõ tiếng Việt không dấu được; danh sách theo nhóm, có nhãn “Cần rảnh”.
  - Chọn lệnh → trang tham số:
    - mỗi tham số là **chip** (giá trị gợi ý, “Không ghi”, công tắc); nội dung lời thường dùng ô chữ;
    - mục **Ví dụ · chạm để dùng**; hộp mẹo;
    - xem trước dòng lệnh tô màu, trạng thái kiểm tra, nút **Chạy /export** to.
  - Lệnh có màn kết quả riêng (cocreate, import, diag, sync, reopen) → sau khi chạy, toast có liên kết sang M5.
  - Ẩn trên điện thoại: `/config`, `/start`, `/simulate`, `/importsim`. Trang hướng dẫn vẫn liệt kê chúng với nhãn “Chỉ trên máy tính”.
- **Liên kết:**
  - *Xem tất cả* ở “Vừa xong” và *Xem nhật ký đầy đủ* trong sheet việc nền → M6;
  - *Hướng dẫn* trong sheet lệnh → M4 · Hướng dẫn lệnh.

## M2 · Đọc (`MReader.dc.html`)

> **Góp ý theo đoạn (Gói AI):** vòng đời, ngăn Góp ý, gắn lại đoạn — xem `MFeedback` và `05-screens.md` › *03b*. Tiêu đề sheet góp ý đổi “Góp ý đoạn 2” → “Góp ý cho đoạn này” (không hiện số đoạn). <!-- ai-m2 -->

- **Thanh trên:**
  - “Tập 1 · chương 24/30” + tên chương → sheet danh sách chương;
  - `Aa` → sheet hiển thị (cỡ 15–23, có chân / không chân, giãn dòng);
  - tai nghe → M3;
  - vạch tiến độ đọc 2 px.
- **Đọc tập trung** (`immersive`):
  - Cuộn xuống quá 40 px thì thanh trên thu lại (margin âm + mờ dần) và thanh tab ẩn; chỉ còn vạch tiến độ.
  - Cuộn lên hoặc chạm giữa màn thì hiện lại.
- **Cuối chương:**
  - Thẻ **Chương tiếp** gồm tên, số chữ và trạng thái (Đã duyệt / Đang viết có orb / Chờ duyệt). Bấm để sang chương sau.
  - Gợi ý nhỏ “Vuốt sang trái/phải để đổi chương”: vuốt ngang trên vùng chữ để qua chương trước/sau.
- **Chạm một đoạn để chọn:** đoạn sáng lên, các đoạn khác mờ, hiện chip *Góp ý đoạn này* · *Chép*.
- **Sheet góp ý:**
  - trích đoạn, chip nhanh, ô chữ tự do, *Gửi cho Người viết*;
  - khi gõ (`commentKb`), sheet nằm ngay trên bàn phím (`bottom: 300px`), nút *Xong* thu bàn phím.
- **Danh sách chương:**
  - Mỗi dòng có nút **⋯** → action sheet *Đọc chương* · *Nghe chương* · *Nhờ viết lại…* (mở sheet góp ý tiêu đề “Viết lại chương N”) · *Xuất riêng chương này* · *Chép đường dẫn tệp*.
  - Nhấn giữ một dòng cũng mở action sheet này.
- **Thanh duyệt** cố định khi chờ duyệt: *Góp ý sửa* · *Duyệt & viết tiếp*.
- **Triển khai:** `web/src/screens/reader/mobile` dùng chung dữ liệu/neo đoạn/hoàn tác với trình đọc máy tính. Một listener cuộn qua `mobile/runtime.tsx` đồng thời cập nhật tiến độ, lưu vị trí đoạn và đổi chế độ đọc tập trung. Bình luận ngoại tuyến dùng `mobile/outbox.ts`, có nhãn “Chờ gửi · sẽ tự gửi khi có mạng”; nội dung đang gõ dùng bộ nhớ riêng theo tài khoản. Vai trò chỉ đọc không có dấu hay thao tác góp ý; truyện khoá vẫn đọc được. Các thao tác viết lại/xuất bị ẩn khi không có quyền ghi.
- **Mock M2:** `#/p/dit-dang-audio/s/m2-reader/read/24`, chương 23 đã duyệt và truyện `m2-reader-locked` cho trạng thái khoá. Tài khoản `lan` xem cùng nội dung chỉ đọc.

## M3 · Nghe (`MAudio.dc.html`)

> **Tạo giọng (Gói AK, 29/10):** ngăn Tạo giọng · Tập N thay hai sheet cũ (Hàng đợi + chọn chương). Xem `MVoiceGen` và `10-audio.md` › *A8*. <!-- ak-m3 -->

- **Thẻ tổng:** vòng 9/12 chương có giọng, thời lượng, phân vai, “Đang tạo chương 10 · 44%”.
- **Danh sách chương:**
  - ▶ để nghe; chương đang nghe có cột sóng;
  - vòng % cho chương đang tạo; vòng nét đứt cho chương đang chờ.
- **Trình phát thu nhỏ** nổi ở `bottom: 92px` (trên thanh tab nổi).
  - Hai nút tách riêng: vùng mở trình phát và nút phát/tạm dừng. Không lồng nút trong nút.
- **Trình phát đầy đủ** (sheet gần kín màn):
  - câu đang đọc kiểu karaoke;
  - dạng sóng 48 vạch làm thanh tua;
  - ⏮ −10 ⏯ +10 ⏭;
  - chip tốc độ, hẹn giờ tắt.
- **Ghép & xuất:**
  - định dạng và tuỳ chọn giống hệt máy tính (xem `10-audio.md` › *Tuỳ chọn xuất dùng chung*, Gói AB): MP3 · M4B · WAV; nhóm thu gọn *Âm thanh khi ghép* (nhạc mở/kết, đọc tên chương, cân âm lượng, khoảng lặng); mốc chương (chỉ M4B); phụ đề .srt;
  - *Xuất 9 chương* / *Xuất khi xong*;
  - khi chạy: vòng % + bước;
  - xong: *Chia sẻ* / *Tải xuống*.

### M3 bổ sung 28/10: quản lý tạo giọng và xuất thử (Gói Z) <!-- m3-taogiong -->

- **Thẻ tổng là nút** (mũi tên ›) mở sheet **Tạo giọng · Tập 1**. Dòng trạng thái trong thẻ:
  - đang chạy: chữ có ánh lướt như bản máy tính;
  - tạm dừng: cam “Đã tạm dừng ở chương 10 · 41%”;
  - lỗi: đỏ “Dừng do lỗi ở chương 10 · bấm để xem”, viền thẻ đổi màu theo.
- **Sheet Tạo giọng** (cao gần kín màn, cuộn trong):
  - **Thẻ đang chạy:** avatar người đang đọc có sóng toả; “Dẫn truyện đang đọc” (ánh lướt); chương, đoạn 23/51, thời gian còn; % lớn; thanh tiến độ. Nút *Tạm dừng* và công tắc chip *Báo khi xong*; dòng “Chạy trên máy chủ ở nhà, có thể đóng app”.
  - **Tạm dừng** (`queuePaused`): thẻ cam “Phần đã tạo được giữ nguyên, tiếp tục từ đoạn 24”, nút *Tiếp tục*.
  - **Lỗi** (`queueError`): thẻ đỏ nói lý do bằng lời thường (“Máy chủ hết bộ nhớ đồ hoạ khi đọc một đoạn dài”), nút *Thử lại* và *Thử với “Nghe thử nhanh”*.
  - **Hàng đợi:**
    - dòng gộp “Chương 1–9 đã có giọng”, mở ra từng chương có *Tạo lại*;
    - chương đang tạo có vòng % + nhãn Đang tạo / Tạm dừng / Lỗi;
    - chương chờ có nhãn Chờ + thời gian ước tính;
    - chương chưa chốt bị mờ, ghi “Chốt bản đọc trên máy tính”.
    - Nút *Làm trước* chỉ hiện khi có từ 2 chương chờ trở lên.
  - **Chất lượng giọng cho chương chưa tạo:** Nghe thử nhanh · Chuẩn · Cao nhất, mỗi mức có thời gian ước tính. Chương đã có giọng giữ nguyên, muốn đổi thì *Tạo lại*.
  - **Công tắc** *Tự tạo giọng khi chốt chương*.
- **Ghép & xuất** thêm nút chữ **Xuất thử chương N trước · khoảng 10 giây** (N = chương đang nghe):
  - xuất đúng một chương với cùng định dạng và tuỳ chọn để nghe kiểm tra;
  - chạy nền như xuất cả tập (Gói AE): bảng đóng, thẻ tiến độ nổi trên trình phát thu nhỏ;
  - xong (`exportTrial`): “Bản thử chương 4 đã sẵn sàng”, tên tệp có “ban-thu”, nút *Nghe bản thử* / *Xuất cả tập* (quay lại form, giữ tuỳ chọn).

### M3 bổ sung 28/10 (5): ghép chạy nền + tự tải (Gói AE) <!-- m3-ghep-nen -->

- Sheet Ghép & xuất có công tắc **Tự tải về khi xong** (“Chạy nền — đóng bảng này vẫn tiếp tục”), mặc định bật, nhớ theo thiết bị.
- Bấm *Xuất 9 chương* / *Xuất N chương* / *Xuất thử*: sheet đóng ngay; **thẻ việc nổi** trên trình phát thu nhỏ (`bottom: 164px`): vòng % · “Đang ghép Tập 1” · “Chương 2/9 · chạy nền, cứ nghe tiếp” · nút × = huỷ. Toast thường nâng lên trên thẻ.
- Xong: “Đã tải về máy · Tập 1” · *Xem tệp* (mở màn đã xuất trong sheet) · *Chia sẻ*. Tắt tự tải: “Đã ghép xong” · **Tải xuống** · *Chia sẻ*. Máy chặn tự tải: thẻ cam + “Bấm Tải xuống, hoặc Chia sẻ để lưu vào Tệp”. Lỗi: thẻ đỏ · **Thử lại** · *Ghép không nhạc*.
- Bỏ `state=exportRun` (vòng % trong sheet). Trạng thái mới: `bgRun` · `bgDone` · `bgManual` · `bgBlocked` · `bgFail`.
- Trình phát thu nhỏ ghi người đọc theo dữ liệu thật (“Dẫn truyện”), không dùng tên giọng mẫu.

### M3 bổ sung 28/10 (2): chọn chương để tạo giọng ngay trên điện thoại <!-- m3-chon-chuong -->

Trên điện thoại giờ làm được trọn vòng: **chọn chương → chốt → tạo giọng → theo dõi → xuất**. Chỉ việc sửa bản đọc chi tiết (người nói, xưng hô, từng đoạn) vẫn làm trên máy tính.

- **Lối vào:**
  - dưới thẻ tổng có hai nút **Tạo giọng** (chính) và **Hàng đợi · N**;
  - mỗi chương chưa có giọng có chip **Tạo giọng** ở cuối dòng (mở sheet chọn sẵn chương đó);
  - trong sheet Tạo giọng · Tập 1: nút **+ Thêm chương** ở đầu hàng đợi;
  - dòng chương chưa chốt có nút **Chốt & tạo**.
- **Sheet chọn chương** (`genPick`):
  - chip nhanh *Chọn hết chưa có giọng (N)* · *Bỏ chọn*;
  - **Chưa có giọng:** ô chọn + tên + nhãn “Chưa chốt” + số đoạn, số người nói + nhãn cam “2 chỗ cần xem” nếu có + thời gian ước tính. Chương đang tạo hiện mờ ở cuối nhóm (“đã trong hàng đợi”), không chọn được.
  - **Tạo lại chương đã có giọng** (`genPickRedo`): công tắc; bật thì hiện chương 1–9 có ô chọn. Chương có bản đọc sửa sau khi tạo ghi “bản đọc đã sửa sau khi tạo”.
  - **Giọng đọc:** dòng tóm tắt (avatar + “Phân vai · 5 giọng”) mở sheet **Giọng đọc** (`genVoice`):
    - 3 kiểu: Phân vai · Một giọng kể · Giọng của gói;
    - danh sách “Ai đọc ai”, mỗi dòng có nút nghe mẫu;
    - đổi giọng từng nhân vật làm trên máy tính.
  - **Chất lượng:** Nghe thử nhanh · Chuẩn · Cao nhất (phút/chương).
  - **Cảnh báo cam** khi chọn chương chưa chốt: “Tạo giọng sẽ chốt luôn theo bản đọc hiện tại. Chương 11 còn 2 chỗ cần xem (người nói chưa rõ); vẫn tạo được, sửa sau trên máy tính rồi Tạo lại.”
  - **Thanh dưới cố định:** “2 chương · Chuẩn · khoảng 26 phút sau chương đang tạo” + nút **Chốt & tạo giọng 2 chương** (chỉ tạo lại: **Tạo lại N chương**; chưa chọn: mờ).
  - Bấm thì đóng sheet, báo “Đã xếp N chương vào hàng đợi”; nút Hàng đợi và danh sách chương cập nhật ngay (“Chờ tạo · khoảng 13 phút”).

### M3 bổ sung 28/10 (3): đổi giọng trên điện thoại <!-- m3-doi-giong -->

Dữ liệu giọng khớp bản máy tính (Audio › Phân vai):

| Vai | Giọng |
|---|---|
| Dẫn truyện | Mẫu giọng Tri Hạ · 18 giây |
| Lâm Tri Hạ | cùng giọng dẫn truyện |
| Tống Nhược Vy | Mẫu giọng Nhược Vy · 22 giây |
| Hứa Hoài Châu | Nam trầm |
| Đỗ Mẫn | Nữ trung niên |

- **Lối vào sheet Giọng đọc** (`genVoice`):
  - dòng **Giọng đọc** trong sheet chọn chương;
  - dòng **Giọng đọc** trong sheet Tạo giọng · Tập 1.
  - Nút Xong quay về đúng sheet đã mở nó.
- **Sheet Giọng đọc:**
  - 3 kiểu giọng;
  - danh sách “Ai đọc ai · bấm để đổi”: mỗi vai có ▶ nghe mẫu, avatar, giọng đang dùng (tím nếu vừa đổi), chữ **Đổi ›**.
- **Sheet Giọng cho {vai}** (`genVoicePick`):
  - dòng phụ: số đoạn ở chương đã có giọng · giọng đang dùng;
  - **Giọng của bạn:** các mẫu đã tải lên (+ “Cùng giọng dẫn truyện” riêng cho vai người kể) và dòng **Ghi âm mẫu giọng mới**;
  - **Giọng có sẵn:** Nữ trẻ, ấm · Nữ trung niên · Nam trầm · Nam trẻ, sáng · Giọng kể lạnh (gói đề xuất);
  - mỗi dòng có ▶: đang phát thì thành nút tím có sóng; bấm dòng là chọn và phát luôn mẫu.
  - **Thanh dưới:** công tắc **Đọc lại N đoạn cũ của {vai}**.
    - Tắt: đoạn cũ giữ giọng cũ, chương sau dùng giọng mới.
    - Bật: xếp hàng sau chương đang tạo, có thời gian ước tính.
  - Nút **Dùng “{giọng}”**; chưa đổi gì thì nút ghi **Giữ giọng hiện tại**.
- **Ghi âm mẫu** (`genVoiceRec`):
  - đoạn văn gợi ý để đọc; nút ghi đỏ lớn (đang ghi thì có nhịp, icon dừng); đồng hồ 0:16 / 0:30; sóng âm đỏ chạy;
  - gợi ý trạng thái: cần ít nhất 10 giây / đủ dài / quá ngắn;
  - nút **Chọn tệp** (.mp3 · .wav · .m4a) và **Dùng mẫu này** (mờ khi chưa đủ 10 giây);
  - ghi chú: lưu vào Tài sản › Giọng của dự án; chỉ dùng giọng của bạn hoặc người đã đồng ý.
  - Dùng xong thì quay về danh sách, mẫu mới được chọn sẵn.

### M3 bổ sung 28/10 (4): xuất lẻ từng chương <!-- m3-xuat-le -->

- **Dòng chương đã có giọng** có nút **⋯** mở sheet thao tác (`chMenu`):
  - Nghe chương này;
  - **Xuất riêng chương này** (một tệp · dung lượng);
  - Chia sẻ tệp audio;
  - Tạo lại giọng (mở sheet chọn chương, chọn sẵn chương này).
- **Sheet Ghép & xuất** có hàng chọn **Cả tập** (một tệp có mốc chương) / **Từng chương** (mỗi chương một tệp). `exportCh`:
  - lưới số chương 1–9 (5 cột, ô 44 px); ô “10–12” mờ vì chưa có giọng; *Chọn cả 9* / *Bỏ chọn*;
  - định dạng MP3 · M4B · WAV như cả tập; nhóm *Âm thanh khi ghép* chỉ còn đọc tên chương và cân âm lượng (tệp lẻ không kèm nhạc mở/kết, khoảng lặng, mốc chương); giữ phụ đề .srt;
  - **Cách nhận tệp:** *Từng tệp riêng* / *Gộp một tệp .zip*;
  - dòng tóm tắt “3 chương · 54 phút · khoảng 51 MB”; nút **Xuất 3 chương · 3 tệp** (một chương: **Xuất chương 4**).
- **Đang xuất:** “Đang xuất 3 chương…”, bước “Chương 3 · 2/3”.
- **Xong** (`exportChDone`):
  - “Đã xuất 3 chương · 3 tệp .mp3 + .srt” (đuôi theo định dạng đã chọn);
  - danh sách tệp (tên, thời lượng, dung lượng, nút chia sẻ từng tệp);
  - nút **Chia sẻ tất cả** / **Tải tất cả** (hoặc **Tải .zip**).

## M4 · Thêm (`MMore.dc.html`)

> **Ứng dụng (Gói AO, 31/10):** nhóm *Ứng dụng* trên nhóm Tài khoản — Noverse (Đang mở) và Viderse (mở thẻ mới); ẩn cả nhóm khi không có quyền Viderse (`viderse=no`). *Đăng xuất* có dòng phụ “Khỏi cả Noverse và Viderse trên trình duyệt này”. Xem `09-auth-onboard.md` §12. <!-- ao-m4 -->

> **Cách viết (Gói AP, 31/10):** bỏ cấp Mọi truyện; nhóm “Của truyện này” · “Học từ góp ý · đang áp dụng” · “Từ dự án …”; mỗi quy tắc một nhãn loại. Thẻ Chống giọng văn máy mở ra 16 từ với − / + (`state=rulesAai`). Xem `20-settings-rules.md` §9.6. <!-- ap-m4 -->

> **Lưu trữ tệp (Gói AN, 31/10):** nhóm Quản trị › hàng *Lưu trữ tệp* (chỉ quản trị) mở trang chỉ xem: trạng thái, đang chuyển, mất kết nối, Kiểm tra kết nối, dung lượng 6 loại, “Đổi kho trên máy tính.” Xem `20-settings-rules.md` §8.6. <!-- an-m4 -->

> **Cách viết (Gói AM, 31/10):** dòng “Quy tắc viết” → **Cách viết** (văn phong + quy tắc). Trang Cách viết: dòng *Văn phong* mở danh sách chọn tại chỗ (`rulesStyle`), dưới là Quy tắc của truyện. “Tuỳ chỉnh truyện” chỉ còn Duyệt từng chương, Người viết, Ngân sách. Xem `20-settings-rules.md` › *Cách viết gọn lại*. <!-- am-m4 -->

- **Quy tắc viết** (`rules`, `rulesAdd`, `rulesThinking`) và **Tuỳ chỉnh truyện** (`storyset`): hai dòng mới trong nhóm “Đang mở”. Chi tiết ở `20-settings-rules.md` §3.2. Công tắc *Duyệt từng chương* chuyển từ trang Thêm vào Tuỳ chỉnh truyện.

- **Trang Thêm:**
  - **Thẻ tài khoản** (bấm để đổi tài khoản).
  - Đang mở: đổi truyện.
  - Trợ lý: model từng trợ lý, *Duyệt từng chương*.
  - Tài khoản: Thiết bị · **Đổi mật khẩu** · **Thông báo** (→ M7).
  - Trợ giúp: **Hướng dẫn lệnh** (14 lệnh).
  - Đăng xuất.
- **Đổi tài khoản** (`accounts`) — bottom sheet “Tài khoản đã dùng trên máy này”. Theo đúng `09-auth-onboard.md`: máy chỉ giữ **một phiên**, danh sách lấy từ `localStorage["ainovel.accounts"]` (chỉ tên/vai trò/lần cuối, không mật khẩu hay cookie).
  - Mỗi dòng: ảnh đại diện, tên · vai trò, lần cuối. Tài khoản đang dùng có ✓ và hạn phiên; các tài khoản khác có nhãn “Đăng nhập”.
  - Chạm tài khoản khác → sheet chuyển sang bước **Đăng nhập Linh** (thẻ tên + tên đăng nhập, *Đổi* để quay lại), ô mật khẩu, *Quét QR* (→ `Phone.dc.html`) và *Đăng nhập*.
  - Đăng nhập xong: toast “Đã đăng nhập Linh · Dat đã đăng xuất”. Thẻ tài khoản đổi tên, màu và huy hiệu vai trò (Quản trị có vương miện, Biên tập có bút).
  - *Thêm tài khoản* → `Phone.dc.html`.
  - Ghi chú dưới sheet: chuyển tài khoản sẽ đăng xuất tài khoản đang dùng; truyện và việc nền vẫn chạy trên máy chủ.
- **Đổi mật khẩu** (`password`):
  - Ba ô: mật khẩu hiện tại, mới, nhập lại; nút mắt để hiện/ẩn.
  - Vạch độ mạnh 4 nấc, cùng công thức với bản máy tính (≥ 8 ký tự).
  - Danh sách điều kiện tick dần: ≥ 8 ký tự · có chữ và số · khác mật khẩu hiện tại · hai ô khớp.
  - Dòng ghi chú: máy này vẫn đăng nhập, các thiết bị khác phải đăng nhập lại (`POST /api/me/password {current,new}` luôn giữ phiên hiện tại và thu hồi phiên khác). Nút chính bị khoá tới khi đủ điều kiện. Lỗi 403 hiện dưới ô mật khẩu hiện tại.
- **Đăng nhập lần đầu bằng mật khẩu tạm** (`firstPwd`, khi `User.mustChangePassword`):
  - Màn toàn phần, không có nút quay lại hay thanh tab: “Chào Linh, đặt mật khẩu của bạn”, kèm lời giải thích ai đã tạo tài khoản và vì sao nên đổi ngay.
  - Ô **Mật khẩu tạm** (API cần `current`; đăng nhập bằng QR thì trình duyệt không biết mật khẩu tạm), mật khẩu mới, nhập lại; điều kiện “Khác mật khẩu tạm”. Nút **Lưu & vào Noverse**.
  - Đúng như `09-auth-onboard.md` (tuỳ chọn nhưng nhắc mạnh): dòng nhỏ “*Để sau* · sẽ nhắc lại mỗi lần mở cho tới khi đổi”.
- **Hướng dẫn lệnh** (`guide`, `guideCmd`) — bản điện thoại của `Commands.dc.html`, dùng chung `_cmddata`:
  - **Danh sách:** ô tìm, nhóm theo GROUPS, mỗi dòng là `/lệnh` mono + tiêu đề ngắn. Ghi chú “Cần hệ thống rảnh” (cam) hoặc “Chỉ trên máy tính” (mờ).
  - **Chi tiết:**
    - `/lệnh` lớn + bí danh + nhãn “Cần hệ thống rảnh”, tiêu đề, lời dẫn;
    - **Cách gõ**: token tô màu, mỗi token không bị cắt giữa chừng;
    - **Tham số**: token · Bắt buộc/Tuỳ chọn · nhãn · mô tả;
    - **Ví dụ**: dòng lệnh tô màu + *Dùng* → M1 điền sẵn;
    - Nên dùng khi (✓) / Tránh khi (✕), Sau khi chạy (đánh số), Lỗi thường gặp (thông báo → cách sửa), mẹo;
    - chip **Liên quan** đẩy sang lệnh khác;
    - nút dưới “Thử /export ở màn Viết”.
- **Dự án & truyện / Model / Thiết bị:** như cũ (action sheet xác nhận đăng xuất thiết bị).

## M5 · Kết quả lệnh (`MCmd.dc.html`)

Màn con đẩy từ M1. Thanh trên: ‹, dòng lệnh mono nhỏ (ví dụ `/sync --check`) + tiêu đề, nút ? → hướng dẫn của lệnh đó. Không có thanh tab.

- **`/cocreate` — bàn hướng đi với Kiến trúc sư:**
  - Dải trạng thái: orb Kiến trúc sư “thở” khi đang trả lời.
  - Luồng chat: tin của bạn (bong bóng phải), trả lời của Kiến trúc sư (orb + văn bản, chữ chạy kèm con trỏ xanh), dòng hệ thống ở giữa.
  - **Thẻ đề xuất**: danh sách chương 25–27 (số + tên + một câu), chip tác động (Sửa dàn ý ch.25–27 · Giữ nguyên 24 chương · Đẩy 1 phục bút), nhãn “Đã chỉnh v2” sau khi bạn góp ý.
    - *Sửa thêm* điền sẵn ô trả lời.
    - **Chốt & viết tiếp** → thẻ chuyển viền xanh với dấu tick vẽ nét; ô trả lời được thay bằng nút “Xem Người viết làm chương 25”.
  - Ô trả lời nổi ở đáy. Gửi → chấm “đang nghĩ” 3 nhịp → chữ chạy; khung chat tự cuộn xuống.
- **`/import` — xác nhận cách chia chương:**
  - Thẻ tệp (tên, KB, số chữ); số chương lớn; cách chia (“theo dòng «Chương N»”).
  - **Cảnh báo** khi một chương dài bất thường (gấp đôi trung bình).
  - Trạng thái truyện dạng **thẻ radio**: Còn tiếp `--story=open` / Đã hoàn `--story=closed`. Khi còn tiếp có công tắc *Viết tiếp ngay sau khi nhập* (`--continue`).
  - Xem trước 6 chương (dòng cảnh báo tô cam) + *Xem cả N chương*; ô “Lệnh sẽ gửi” hiện dòng lệnh đầy đủ.
  - Nút **Chia lại…** (sheet, `importGuide`):
    - ô mô tả bằng lời + chip gợi ý;
    - ghi chú rằng `--guide=` luôn đặt cuối nên được phép có dấu cách;
    - bấm *Chia lại* → “Đang chia lại…” → 25 chương, hai dòng có nhãn “mới tách”.
  - Nút **Đồng ý (y)**.
  - **Đang nhập** (`importRun`):
    - vòng 148 px gradient Kiến trúc sư → Người viết, % và thời gian còn lại;
    - 5 bước có số đếm (Tóm tắt từng chương 13/24);
    - *Chạy nền*. Xong → tick + “24 chương · 14 nhân vật · 9 phục bút” + *Mở truyện*.
- **`/diag` — chẩn đoán:**
  - Lưới 2 cột 8 chỉ số; “3 đình trệ” tô cam.
  - Chip lọc theo mức (Tất cả · Cảnh báo · Gợi ý, có số).
  - Thẻ phát hiện bấm để mở: bằng chứng + gợi ý (đèn).
  - “Việc nên làm” có nút *Gửi* → *Đã gửi* + toast.
  - *Phân tích lại* → khung xương lấp lánh 1,7 s. Ghi chú “chỉ đọc, không thay đổi gì”.
- **`/sync` — chương sửa bên ngoài:**
  - `--check`: “3 chương đã được sửa bên ngoài Noverse”. Mỗi dòng có +/− và giờ; bấm để mở **khác biệt** (dòng cũ gạch đỏ, dòng mới xanh, chữ mono).
  - Nút *Để sau* / **Tiếp nhận 3 chương**.
  - Sau khi tiếp nhận: mỗi chương có chip (Sự thật cốt truyện cập nhật / Chỉ đổi lời văn / 1 mâu thuẫn phía sau). Thẻ mâu thuẫn có *Mở chương 14* và *Nhờ sửa* (→ *Đã gửi*).
- **`/reopen` — mở lại truyện đã hoàn:**
  - Thẻ sách (khoá từ 21/10); danh sách **điều kiện**, mỗi dòng có ✓ hoặc ✕.
  - Ô “Viết tiếp theo hướng nào?” (không bắt buộc) + chip gợi ý; ghi chú 30 chương cũ giữ nguyên.
  - *Mở lại truyện* → action sheet xác nhận → toast có liên kết.
  - `reopenBlocked`: “28/30 chương đã duyệt sạch” ✕ + “Chương 28 và 29 còn bản sửa chưa duyệt” + liên kết mở chương; ô nhập và nút bị khoá kèm lý do.

## M6 · Nhật ký hoạt động (`MLog.dc.html`)

- **Thanh trên:** chấm xanh nhấp nháy “Đang ghi trực tiếp · N sự kiện”; nút tìm → ô tìm thay tiêu đề + *Huỷ*. Từ khớp được tô vàng (`<mark>`).
- **Chip lọc:** Tất cả · Người viết · Kiến trúc sư · Biên tập · Trọng tài · Lỗi (số) · Của bạn.
- **Nhóm theo chương:**
  - Tiêu đề **dính** khi cuộn, nền mờ. Chương đang viết có orb “thở”; chương khác có icon sách + điểm + số sự kiện.
  - Bấm tiêu đề để thu/mở.
- **Dòng sự kiện:** trục dọc nối các nút. Nút là orb của agent, vòng đỏ ⚠ cho lỗi, chữ “D” cho bạn. Mỗi dòng gồm tên agent màu, giờ mono, nội dung.
  - **Lời gọi công cụ**: nút `tên_công_cụ` + thời gian (spinner nếu đang chạy, chữ “lỗi” đỏ); bấm để mở khối mono (tham số → kết quả) + *Chép*.
  - Lỗi đã tự khắc phục có dòng xanh: “Tự thử lại sau 8 giây · thành công”.
- **Sự kiện mới** trượt vào đầu nhóm và loé tím nhẹ. Nếu bạn đang cuộn xa, hiện viên nổi “↑ N sự kiện mới”.
- *Xem các chương trước* tải thêm.
- API: lịch sử sự kiện lọc theo chương/agent/lỗi/từ khoá và chi tiết lời gọi công cụ — GAP-31.
- Lọc không có kết quả → trạng thái trống có lời gợi ý.

## M7 · Thông báo (`MNotify.dc.html`)

- **Hộp thư** (`inbox`, mở từ chuông M1):
  - Thẻ mời “Nhận thông báo trên điện thoại · Bật” nếu chưa cấp quyền.
  - Nhóm Hôm nay / Hôm qua. Mỗi thông báo: icon màu theo loại, tiêu đề (đậm khi chưa đọc, chấm tím ở mép), nội dung, giờ, **lời dẫn đích đến** (“Mở cổng duyệt ›”).
  - *Đọc hết*, nút cài đặt.
- **Xin quyền** (`prompt`) — màn **trước** hộp thoại hệ thống, để không phí lần hỏi duy nhất:
  - minh hoạ màn khoá có thông báo trượt xuống;
  - 3 cam kết (chỉ báo khi đáng báo);
  - lưu ý iPhone: cần **Thêm vào Màn hình chính** thì Web Push mới chạy (iOS 16.4+);
  - *Bật thông báo* / *Để sau*.
- **Hộp thoại hệ thống** (`system`): mô phỏng để thấy luồng. Cho phép → về hộp thư + toast; Không cho phép → `denied`.
- **Bị chặn** (`denied`): 3 bước mở trong Cài đặt máy, *Tôi đã bật, kiểm tra lại*, và lời nhắc hộp thư vẫn nhận đủ.
- **Cài đặt** (`settings`):
  - Công tắc tổng; khi tắt, các loại mờ đi.
  - **Báo khi…**:
    - Chương chờ duyệt (có nút Duyệt nhanh);
    - Truyện dừng do lỗi (luôn báo, kể cả giờ yên lặng);
    - Việc nền xong;
    - Chương đã xong;
    - Đăng nhập & thiết bị mới (khoá bật).
  - Gộp “Chương đã xong”: Mỗi chương / Mỗi 5 chương / Cuối ngày (21:00).
  - Giờ yên lặng 22:00–07:00.
  - Danh sách thiết bị nhận thông báo, *Gửi thử một thông báo*.
- **Màn hình khoá** (`lock`, `lockActions`): 3 thông báo mẫu. Nhấn giữ thông báo chờ duyệt mở thao tác nhanh: *Duyệt & viết tiếp* · *Nhờ viết lại…* · *Tắt loại thông báo này*.

### Nội dung & liên kết sâu

| Loại | Tiêu đề / nội dung | Chạm vào mở | Thao tác nhanh |
|---|---|---|---|
| `gate` | “Chương 25 chờ bạn duyệt” · tên chương · điểm | `/read/:story/25?gate=1` → M2 trạng thái gate | Duyệt & viết tiếp · Nhờ viết lại… |
| `error` | “Truyện dừng do lỗi model” · nguyên nhân dễ hiểu | `/write/:story` → M1 trạng thái error | — |
| `job` | “Đã xuất sách nói Tập 1” · tệp · dung lượng | `/listen/:story` (audio) hoặc liên kết tải (EPUB) | Chia sẻ |
| `done` | “Xong 5 chương (20–24)” · điểm trung bình | `/read/:story/20` | — |
| `security` | “Đăng nhập trên thiết bị mới” · trình duyệt · nơi | `/more/devices` → M4 Thiết bị | Không phải tôi → đăng xuất thiết bị đó |
| `import` / `sync` / `cocreate` | “Đã nhập 24 chương” … | `/cmd/:job` → M5 đúng view | — |

- API: đăng ký Web Push, cài đặt loại thông báo, hộp thư — GAP-30.
- Payload Web Push: `{type, story, chapter?, job?, title, body, url, actions[]}`.
- Service worker: `notificationclick` → `clients.openWindow(url)`, hoặc focus tab đang mở rồi `postMessage` để điều hướng. Nếu phiên hết hạn, mở đăng nhập rồi chuyển tiếp tới `url`.
- **Triển khai (GAP-30, đối chiếu backend):** cột “Chạm vào mở” dùng đúng route của router, mã truyện `p/s` mã hoá thành một đoạn (`/read/p%2Fs/25?gate=1`, `/write/p%2Fs`, `/listen/p%2Fs`, `/cmd/p%2Fs%2Fimport%2F1`); xuất truyện (EPUB…) trỏ thẳng đường tải `/api/…` và service worker mở thẳng (phiên hết hạn thì vào `/notifications` để đăng nhập). `job` (cả sách nói lẫn xuất truyện) có thao tác `share`. Điểm hiển thị thang 10 (`Biên tập viên chấm 8,4`), lỗi model chỉ ghi nguyên nhân dễ hiểu. Chi tiết và bảng `type → url → actions` ở `11-api-contract.md` §Thông báo.

## M8 · Não truyện (`MBrain.dc.html`) — bổ sung 25/10 <!-- goi-m -->

- **Vào từ:** M4 Thêm › “Não truyện” (ngay sau “Tuỳ chỉnh truyện”). Tab bar sáng ở “Thêm”. Nút quay lại trên thanh trên.
- **Tweak:** `state`: list · sheet · filter · search · empty; `ch`: 1–8.
- **Bố cục:**
  - thẻ chương (‹ ›, dải 8 nấc bấm được, dòng cam khi chương đang viết lại);
  - chip lọc loại có số đếm;
  - danh sách nhóm theo loại;
  - bottom sheet chi tiết.
  - Không có đồ thị: dòng chân mời xem đồ thị trên máy tính.
- **Dòng danh sách:** ảnh đại diện màu nhận diện hoặc biểu tượng loại; tên kèm nhãn “mới” / “đổi ở ch. N”; một dòng quan hệ nổi bật và “+N”.
- **Sheet:** vai, quan hệ tại chương N, xưng hô (gọi / tự xưng), diễn biến, ghi chú khoá khi Người viết đang viết, “Mở trang nhân vật”.
- **Dữ liệu:** cùng API GAP-39 với bản máy tính; trạng thái tại chương N tính ở client.
- **Từ trang Đọc** (`MReader.state=person`): tên nhân vật trong chương có gạch chấm; chạm mở sheet xem nhanh (vai, quan hệ, xưng hô hai chiều, không lộ chương sau), nút “Mở Não truyện” mở `MBrain` đúng chương đang đọc, “Đọc tiếp” đóng sheet.
- **Spec:** `22-brief-2510.md` › M8. **Việc dev:** `21-dev-prompt.md` › Gói M mục 2.

## T1 · Máy tính bảng 641–1099 px (`MTablet.dc.html`)

- **Thanh trên 72 px:** trái là orb + tên truyện; **giữa là thanh tab nổi nằm ngang** (kiểu iPadOS 18, 4×94 px, icon + nhãn cùng hàng, chỉ báo trượt như điện thoại); phải là vòng việc nền “2 việc” (popover 340 px) + chuông.
- **Viết = hai cột:**
  - **Trái 372 px:** thẻ agent (orb lớn, %, thanh gradient, chữ đang viết), 3 thẻ agent nhỏ, “Vừa xảy ra” → M6, ô chat + chip ở đáy. Gõ `/` mở **popover lệnh** neo trên ô chat (không phải sheet), dùng được ↑↓/Enter với bàn phím ngoài.
  - **Phải (phần còn lại):** trình đọc chương, cột chữ tối đa 560 px, Literata 19/1.85.
  - Khi chờ duyệt, **thanh duyệt** nổi ở đáy cột phải: tiêu đề + điểm ở trên, hai nút ở dưới. Tab Đọc có chấm cam.
- **Đọc** (`readFocus`): cột trái thu về 0 (450 ms); nút hiện lại cột ở góc trái thanh chương.
- **Nghe, Thêm:** dùng lại bố cục điện thoại, căn giữa trong cột 560 px.
- **Từ 1100 px trở lên:** dùng bản máy tính.


### Dải mờ trên cùng iOS 26 (Gói AJ, 28/10) <!-- aj-edge -->

**Hiện tượng:** mở Noverse từ màn hình chính iPhone, dòng tên truyện ở thanh trên bị nhoè, dòng “Tập 2 · 12/28 chương” bên dưới thì rõ.

**Nguyên nhân:** từ iOS/iPadOS 26, hệ thống vẽ một lớp mờ dần (Liquid Glass) đè lên khoảng ~40 pt ngay dưới thanh trạng thái của ứng dụng web đã cài. Lớp này nằm *trên* trang web, không phải CSS của mình. Thanh trên của mình bắt đầu ngay ở `max(8px, env(safe-area-inset-top))` nên dòng tên truyện rơi vào dải mờ. Chưa có CSS hay thẻ meta nào tắt được lớp này; đổi `black-translucent` sang `black` chưa thấy ai xác nhận là hết.

**Cách xử lý:** chừa dải đó ra, tô cùng màu nền app (`#0B0B0D`) nên lớp mờ đè lên cũng không thấy gì; mọi chữ và nút bắt đầu bên dưới.

```css
:root{--m-edge:0px}
@supports (-webkit-touch-callout:none){
  @media (display-mode:standalone) and (orientation:portrait){:root{--m-edge:40px}}
  @media (display-mode:standalone) and (min-width:641px) and (min-height:641px){:root{--m-edge:40px}} /* iPad: còn thanh trạng thái cả khi nằm ngang */
}
html[data-standalone]{--m-edge:40px} /* dự phòng: đặt khi navigator.standalone === true */
:root{--m-top:max(8px,calc(var(--m-safe-top) + var(--m-edge)))}
```

- Chỉ áp cho ứng dụng đã cài trên iPhone/iPad. Safari thường, Android, máy tính: `--m-edge = 0`, không đổi gì. iPhone nằm ngang (không có thanh trạng thái): 0.
- Mọi thứ neo ở mép trên dùng `--m-top` (thanh trên, sheet toàn màn, lớp phủ, toast trên, thanh mất mạng). Thay 5 chỗ đang dùng thẳng `env(safe-area-inset-top)`: `tablet.css` (`.tablet-safe`), `phone.css`, `cmdresults/mobile.css` (`.mc-safe-top`), `reader.css` (`.mfb-toast` → `calc(var(--m-top) + 52px)`), `FolderBrowserDialog.tsx`.
- Phần cuộn (`.m-scroll`) luôn nằm dưới thanh trên nên chữ không cuộn vào dải mờ. Nếu có trang cho nội dung cuộn lên sát mép, thêm một dải nền đặc cao `--m-top` cố định trên cùng.
- Artboard vẫn vẽ 40 px thanh trạng thái như cũ; trên máy iOS 26 đã cài, thanh trên thấp xuống thêm 40 px.
- Mở rộng `mobile/safeArea.test.ts`: không component nào dùng thẳng `env(safe-area-inset-top)`, `--m-top` có cộng `--m-edge`.
- Kiểm tra trên iPhone iOS 26 thật (hoặc Simulator iOS 26), mở từ màn hình chính: tên truyện, nút chuông, vòng việc chạy nền đều rõ; cuộn Bàn viết, Đọc, Nghe, Thêm; mở sheet; xoay ngang.

## Ghi chú triển khai

- Cùng route với bản máy tính. Chọn layout bằng `matchMedia`:
  - `(max-width: 640px)` → điện thoại;
  - `(min-width: 641px) and (max-width: 1099px)` → máy tính bảng;
  - còn lại → máy tính.
- **Không** dùng chung component lưới. Mỗi màn có `features/<màn>/mobile/` (máy tính bảng ghép các phần của mobile + reader).
- **Sheet:** dùng `vaul` hoặc tự viết với `touch-action: none`; kéo xuống quá 30% chiều cao thì đóng.
- **Bàn phím:** `window.visualViewport` (`resize`/`scroll`) → `--kb: <px>`. Thanh tab dùng `tabMode = 'hide'` khi `--kb > 0`.
- **Cuộn:** một listener `scroll` (passive) cho mỗi màn. `down = y > lastY && y > 40` → chip thu lại, thanh tab `mini`. Ở trình đọc: `imm = y > 40` thì ẩn thanh trên và thanh tab.
- **Mất mạng:** sự kiện `online`/`offline` + SSE `error` → băng mất kết nối, thử lại theo backoff 1 → 2 → 4 → 5 s. Góp ý gửi lúc mất mạng lưu IndexedDB (`outbox`), gửi lại khi `online`.
- **Phiên hết hạn:** API trả 401 → sheet đăng nhập (giữ nội dung ô chat trong bộ nhớ). Đăng nhập xong thì gọi lại request bị lỗi.
- **PWA:** `display: standalone`, `theme-color #0B0B0D`, `apple-touch-icon`. Web Push (VAPID) — xem GAP-30.
- **Audio:** Media Session API (điều khiển trên màn khoá, ảnh giọng, tên chương), giữ phát khi khoá máy.
- **Đổi tài khoản:** không thêm API. `POST /api/logout` rồi `POST /api/login` (hoặc `/api/pair`), sau đó xoá cache `web/src/data` và tải lại dữ liệu. Ô chat đang gõ dở được giữ trong bộ nhớ theo tài khoản.

## Cập nhật 25/10 (brief design) <!-- brief-2510 -->

- **M2 Đọc · Xuất riêng chương** (`MReader.state=exportMenu`): menu chương mở bước thứ hai “Xuất chương N dưới dạng” — Văn bản .txt · Ebook .epub · Bản đọc TTS; nút dưới đổi “Huỷ” → “Quay lại”; toast “Đã xuất chương 22 · bản đọc TTS”. “Chép đường dẫn tệp” → “Chép liên kết chương”.
- **M6 Nhật ký** (`MLog.state=review`): dòng Biên tập viên có thẻ mục thu gọn (mức · § · tên mục · câu lỗi); vi phạm xưng hô Bắt sửa/Cảnh báo theo D9. Mốc giờ dùng font thường (có chữ “hôm qua”).
- **M3 Nghe** (`MAudio.voice=pack`): “~2 giờ 41 phút · một giọng kể” + badge “theo gói”.
- **Não truyện trên điện thoại:** chỉ danh sách (mẫu `Brain.view=list` + thanh chương dạng nấc + bottom sheet chi tiết), không vẽ đồ thị ở 390 px. Đã vẽ 25/10: **M8** (`MBrain.dc.html`), xem mục M8.
- **Câu hỏi treo:** khung “chưa chắc” dùng lại mẫu bản máy tính (viền cam, hai nút xếp dọc full width); cocreate trống = icon Kiến trúc sư + “Chưa bàn gì · Gõ ý tưởng để bắt đầu bàn với Kiến trúc sư”; dòng hệ thống cocreate có `HH:MM` ở dòng đầu mỗi phút; danh sách push ghi “iPhone · Safari” / “Android · Chrome” / “Máy tính · Chrome” / “Thiết bị khác”; máy tính bảng giữ mốc 641–1099 px, 760 px chỉ là mốc phụ thu cột phụ thành ngăn kéo.

## M9 · Truyện (`MOutline.dc.html`) · M10 · Não dự án (`MProjBrain.dc.html`) · M1b · Luồng viết (`MRun.dc.html`) — Gói AQ, 01/11 <!-- aq-m -->

- **M9** vào từ nút Truyện trên thanh đầu M1 và hàng *Truyện* ở M4 › Đang mở. Tab bar sáng ở Viết. Tweak `tab`, `found`, `sheet`. Chi tiết `20-settings-rules.md` §11.
- **M10** vào từ M4 › Đang mở › *Não dự án*. Không tab bar (trang đẩy). Tweak `tab`, `lview`, `sheet`, `check`, `uni`.
- **M1b** vào từ dòng bước của thẻ Đang viết ở M1 (*Luồng viết ›*). Tweak `state`, `op`. Chi tiết `20-settings-rules.md` §10.
- Khoảng 40 px đầu mọi board điện thoại là chỗ status bar minh hoạ; app dùng safe-area thật (GAP-AN-FE3).
