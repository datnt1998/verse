# 10 · Audio: bản đọc (transcript) → tạo giọng đọc → nghe → xuất tập

Artboard: `Transcript.dc.html` (06 · duyệt, nghe thử & chốt — chốt là tự tạo giọng), `AudioExport.dc.html` (07 · tạo giọng chạy nền, nghe cả tập, ghép & xuất). Nav chính thêm mục **Audio** (giữa Trình đọc và Truyện mới).

## 1. Quyết định đã chốt với anh Dat

| Câu hỏi | Chốt |
|---|---|
| “Transcript” là gì | **Bản đọc cho TTS** — AI soạn từ chương đã nghiệm thu: đổi số/ký hiệu thành chữ, tách lời thoại, gán người nói, ngừng nghỉ, sắc thái. Người dùng duyệt/sửa rồi **chốt** từng chương |
| Giọng | Chọn được cho từng truyện: **Phân vai** (mặc định, kiểu audio drama: dẫn truyện + nhân vật chính có giọng riêng từ mẫu giọng trong `tai-san/`, nhân vật phụ dùng giọng chung) hoặc **Một giọng** đọc tất cả |
| Dịch vụ | **Chạy trên máy** (mô hình TTS mở, có clone giọng từ mẫu ngắn) |
| Đơn vị xuất | **Theo tập, ghép từ chương**: audio từng chương để nghe/sửa lẻ → ghép 1 file mỗi tập (nhạc mở/kết, khoảng lặng) vào `xuat-ban/tap-NN/` |

## 2. Luồng — “nghe là giữ, chốt là tự tạo”

Không có bước “tạo audio cả tập” riêng. Mọi đoạn có giọng chỉ được tạo **một lần** và được giữ:

```
Chương nghiệm thu ─▶ AI soạn bản đọc ─▶ Duyệt ở 06 ── nghe thử đoạn nào = tạo & GIỮ đoạn đó
                                              │
                                              ▼  [Chốt & tạo giọng chương N]
                       ainovel tự tạo nốt các đoạn còn lại của chương — CHẠY NỀN (phòng thu 07)
                                              │  (vẫn duyệt chương khác bình thường)
                                              ▼  đủ 8/8 chương có giọng
                       [Nghe & xuất Tập 1] ─▶ Nghe cả tập (07 listen) ─ đọc lại đoạn nào chỉ tạo đoạn đó
                                              ▼
                       [Ghép & xuất Tập 1] ─▶ chỉ ghép file (≈ 1 phút) ─▶ Đã xuất
```

- Sửa chữ một đoạn đã có giọng → chấm cam “cần đọc lại”; chốt lại chỉ tạo đúng các đoạn đổi.
- Chất lượng giọng là **thiết lập của truyện** (chip “Chất lượng: Chuẩn” trên thanh Audio), dùng cho cả nghe thử lẫn tạo nền. Đổi chất lượng hoặc đổi chế độ giọng ⇒ các đoạn đã có sẽ được tạo lại (hỏi xác nhận, nêu thời gian).
- Có thể tắt “Tự tạo khi chốt chương” trong Cài đặt › Giọng đọc; khi tắt, chương đã chốt hiện nút [Tạo giọng chương này].

### A1 · Bản đọc (06, state `review`)
- **Thanh Audio** — 3 bước `Duyệt & chốt (N/8 chương)` → `Tạo giọng (chạy nền)` → `Nghe & xuất`; chip trạng thái audio có vòng nhỏ (“Audio 2/8 chương · đã nghe thử 12 đoạn” / “Đang tạo giọng chương 3 · 38%” + sóng, bấm mở phòng thu); chip **Chất lượng: Chuẩn ▾**; nút [Nghe & xuất Tập 1] (sáng khi 8/8 chương đã chốt).
- **Cột trái** — mỗi chương thêm dòng audio: `xong · 16:40` (mint) · `đang tạo 38%` + thanh chạy · `12/46 đoạn có giọng` · `2 đoạn cần đọc lại` (cam). Trạng thái bản đọc: `Chưa soạn` (vòng nét đứt) · `AI đang soạn…` (spinner + chữ chạy) · `Chờ duyệt · N cần xem` (chấm tím) · `Đã chốt` (khóa mint, kèm thời lượng) · `Sửa sau khi chốt` (cảnh báo cam). Thanh “N/8 đã chốt”. Nút “Soạn bản đọc các chương còn lại”. Chỉ chương đã nghiệm thu mới soạn được.
- **Giữa** — tên chương, “46 đoạn · 5 người nói · ≈ 17 phút đọc”, [So với chương gốc], [Nghe thử cả chương]; lọc: Tất cả · Cần xem · theo người nói. Mỗi **đoạn**: số thứ tự, nút nghe (chấm mint góc nút = **đã có giọng, dùng luôn khi xuất** — bấm phát ngay; chưa có → bấm sẽ tạo rồi giữ: spinner → sóng nhỏ; chấm cam = chữ đã đổi, cần đọc lại), chip người nói, chip sắc thái (Bình thản, Mỉa mai nhẹ, Ngọt, giả lả…), chữ bản đọc (Literata), chip ngừng `⏸ 0,6 s`. Chữ đã chuẩn hoá gạch chân chấm xanh, rê chuột xem gốc (`90 giây → chín mươi giây`, `live → lai`). Hover: đổi người nói · tách đoạn · sửa chữ.
- **Trường hợp cần người quyết** ngay trong đoạn: “Chưa rõ người nói” + gợi ý của AI có xác suất (`Hứa Hoài Châu · 62%`, `Đỗ Mẫn · 30%`, Người khác…); “Câu dài 41 từ — nên ngắt ở dấu ;” + [Tách thành 2 đoạn] [Bỏ qua].
- **Chân** — “4 chỗ cần xem · vẫn chốt được nếu bạn thấy ổn” · [Xem chỗ cần xem tiếp theo `J`] · [Chốt & tạo giọng chương 3].
- **Panel phải** (tabs):
  - *Chế độ giọng* (đầu tab Phân vai, radio 2 ô): **Phân vai** — mỗi nhân vật một giọng · **Một giọng** — một người đọc tất cả. Đổi qua lại không cần soạn lại bản đọc; người nói vẫn được giữ.
  - *Một giọng*: danh sách giọng (mẫu giọng của dự án + giọng có sẵn, nghe thử, chọn 1) + 3 công tắc: **Đổi giọng nhẹ khi là lời thoại** (cao/trầm hơn chút theo nhân vật — bật mặc định) · **Giữ sắc thái từng đoạn** (bật) · **Ngừng lâu hơn trước lời thoại** (+0,3 s, tắt). Khi ở chế độ này: bỏ các mục Cần xem về người nói/vai chưa có giọng, đoạn chưa rõ người nói hiện nhãn trung tính “Lời thoại”, không chặn chốt.
  - *Phân vai · cả truyện*: Dẫn truyện (ngôi thứ nhất → giọng Lâm Tri Hạ) · Lâm Tri Hạ lời thoại “Cùng giọng dẫn truyện” · từng nhân vật + mẫu giọng (`giong-tri-ha.wav · 18 giây`) hoặc giọng có sẵn (`Nam trầm`) + nút nghe mẫu · vai chưa có giọng viền cam [Chọn giọng cho …] · Nhân vật phụ → giọng chung theo giới tính · [Thêm mẫu giọng] (10–30 giây, rõ, ít tạp âm → Tài sản › Giọng của dự án).
  - *Cần xem*: chưa rõ người nói, từ tiếng Anh, câu dài, vai chưa có giọng — bấm nhảy tới đoạn. Ghi chú “Đã tự đổi 12 chỗ số, ngày giờ, ký hiệu…”.
  - *Cách đọc · cả dự án*: từ điển `live → lai`, `show → sô`, `MC → em-xi`… (nghe thử từng từ) + công tắc tự chuẩn hoá: đọc số thành chữ, ngày/giờ/tiền, đọc tên chương ở đầu, bỏ ghi chú trong ngoặc vuông.
  - Chân panel: “Đọc trên máy của bạn · [mô hình đọc] · Đổi” → Cài đặt › Giọng đọc.

### A2 · Các trạng thái khác của 06
- `locked` — banner mint “Đã chốt lúc 21:40 · đang tạo giọng cho các đoạn còn lại trong nền” + thanh chạy “18/46 đoạn có giọng · 12 đoạn từ lúc nghe thử” + [Xem tiến độ] + [Mở khoá để sửa]; nút chân đổi thành [Sang chương 4]. Các đoạn “chưa rõ” đã được giải quyết.
- `drafting` — chương 5 đang được AI soạn: dòng chữ chạy “AI đang soạn bản đọc chương 5 · đoạn 14/52 · đổi số thành chữ, tách lời thoại, gán người nói…”, các đoạn chưa xong mờ.
- `stale` — chương đã sửa sau khi chốt: banner cam + [Soạn lại 2 đoạn]; đoạn đổi viền/tô vàng; nút “Chốt lại & đọc lại 2 đoạn”. Audio của các đoạn không đổi được giữ.
- `allDone` — 8/8 đã chốt và có giọng → nút **Nghe & xuất Tập 1** sáng lên (trước đó xám, tooltip “Chốt đủ 8 chương để nghe cả tập và xuất”).

### A3 · Ghép & xuất (07, `confirm`) — mở từ nút ở chân trình nghe
Hộp thoại “Ghép & xuất Tập 1”:
- 3 ô: **Giọng đọc đã tạo** 8/8 chương · 372 đoạn · đã nghe & duyệt · không tạo lại · **Độ dài cả tập** 2 giờ 24 phút (kể cả nhạc và khoảng lặng) · **Giọng đọc** 6 giọng (hoặc 1 giọng) + Sửa phân vai.
- Ghi chú: “Bước này chỉ ghép audio đã có của 8 chương thành file để đăng — không đọc lại gì.”
- **Thành phẩm**: theo *Tuỳ chọn xuất dùng chung* bên dưới (Gói AB) — định dạng MP3 · M4B · WAV, nhạc mở đầu & kết thúc, đọc tên chương, cân bằng âm lượng, khoảng lặng 2 giây, mốc chương (chỉ M4B), phụ đề .srt. (Không còn chọn chất lượng ở đây.)
- Lưu vào … · dung lượng. Chân: “Chạy nền khoảng 1 phút — đóng hộp này, việc vẫn tiếp tục” + công tắc **Tự tải về máy này khi xong** · Huỷ · [Ghép & xuất] → hộp đóng ngay, ghép chạy nền (Gói AE). *Thay cho bản cũ: chờ trong hộp rồi sang `exported`.*

**Xuất lẻ từng chương** (28/10, Gói AA) <!-- xuat-le -->

- **Ba lối vào trên máy tính** <!-- xuat-le-loi-vao -->:
  - ở trình nghe (`listen`), mỗi dòng chương bên trái có nút xuất nhỏ, hiện khi rê chuột, khi focus và luôn hiện ở chương đang nghe. Bấm → mở hộp ở chế độ *Chọn chương*, chọn sẵn đúng chương đó;
  - chân trình nghe có nút phụ **Xuất từng chương** cạnh **Ghép & xuất Tập 1** → mở hộp ở chế độ *Chọn chương*;
  - trong hộp Ghép & xuất, đổi *Xuất gì* sang *Chọn chương*.
- **Lối vào trên điện thoại:** nút Xuất → *Từng chương*; hoặc nút ⋯ ở dòng chương → *Xuất riêng chương này*.

- **Hộp Ghép & xuất** có thêm mục **Xuất gì** với hai lựa chọn dạng thẻ:
  - **Cả tập** (như cũ);
  - **Chọn chương**: mỗi chương một tệp riêng (`confirmCh`).
- **Khi chọn chương:**
  - lưới 8 ô có ô chọn, số chương và thời lượng; nút *Chọn cả 8* / *Bỏ chọn*; chọn được chương không liền nhau;
  - tiêu đề hộp đổi thành “Xuất từng chương · Tập 1”;
  - ô thứ hai đổi thành “Độ dài đã chọn · 54 phút · không kèm nhạc mở/kết”;
  - chân hộp: “3 tệp · 54 phút · khoảng 64 MB · xuất mất khoảng 18 giây”; nút **Xuất 3 chương** (một chương thì **Xuất chương N**).
- **Màn đã xuất khi xuất lẻ** (`exportedCh`):
  - “Đã xuất 3 chương · Tập 1”;
  - mỗi tệp một dòng (▶ · tên · thời lượng · dung lượng · *Tải*);
  - nút *Mở thư mục* · *Tải cả 3 tệp*;
  - không hiện mốc chương và danh sách bản xuất cả tập.
- **Màn đã xuất cả tập:** dòng “Từng chương (8 tệp)” có nút *Xem 8 tệp* mở danh sách từng tệp, mỗi tệp có nút tải riêng.
- **Tên tệp:** `{Tên ngắn} · Tập 1 · Chương 02 · {tên chương}.{đuôi}`, máy tính và điện thoại như nhau (Gói AF thay quy tắc cũ `tap-1-chuong-02`); đuôi theo định dạng đã chọn.

**Tuỳ chọn xuất dùng chung — máy tính và điện thoại giống nhau** (28/10, Gói AB) <!-- xuat-chung -->

Trước đây máy tính có MP3/WAV + 4 công tắc âm thanh, điện thoại có M4B/MP3 + mốc chương + .srt. Hợp đồng dữ liệu (GAP-20) có đủ cả hai, nên hai nơi phải hiện **cùng một bộ**; chỉ cách bố trí khác.

| Tuỳ chọn | Giá trị · mặc định | Khi nào hiện |
|---|---|---|
| Định dạng | **MP3** (mở được mọi nơi) · M4B (sách nói, nhớ chỗ, có mốc chương) · WAV (chất lượng gốc, nặng gấp khoảng 8 lần) — mặc định MP3 | luôn |
| Nhạc mở đầu & kết thúc | bật | chỉ *Cả tập* |
| Đọc tên chương ở đầu | bật | luôn |
| Cân bằng âm lượng | bật | luôn |
| Khoảng lặng 2 giây giữa chương | bật | chỉ *Cả tập* |
| Mốc chương trong tệp | bật | chỉ *Cả tập* **và** M4B |
| Kèm phụ đề .srt | bật | luôn |
| Cách nhận tệp | Từng tệp riêng · Gộp .zip | chỉ *Chọn chương / Từng chương* |

- **Máy tính** (`AudioExport` · `confirm` / `confirmCh`): mọi dòng nằm trong thẻ *Thành phẩm*; định dạng là nút chia 3; dòng *Cách nhận tệp* ghi “Tệp lẻ không kèm nhạc mở/kết, khoảng lặng và mốc chương”.
- **Điện thoại** (`MAudio` · `export` / `exportCh`): định dạng là nút chia 3 (tên + một dòng ghi chú); 4 công tắc âm thanh gộp vào dòng mở rộng **Âm thanh khi ghép** (“Đang bật 4/4 · nhạc, tên chương, âm lượng”); mốc chương và .srt nằm ngoài. Sheet cao tối đa gần kín màn và cuộn trong.
- **Màn đã xuất** dùng đúng đuôi theo định dạng; dòng phụ đề chỉ có khi bật .srt; dung lượng ước tính theo định dạng (MP3 ≈ 1,18 MB/phút · M4B ≈ 0,95 · WAV ≈ 10,1).
- **Lựa chọn được nhớ theo truyện, dùng chung giữa máy tính và điện thoại**: xuất trên máy tính bằng M4B thì mở sheet trên điện thoại cũng thấy M4B.

**Ghép & xuất chạy nền + tự tải (28/10, Gói AE)** <!-- ghep-chay-nen -->

Ghép không giữ người dùng trong hộp thoại. Bấm *Ghép & xuất* (hoặc *Xuất N chương*):

1. Hộp đóng ngay, quay về trình nghe. Việc “Ghép Tập 1” vào **dock việc chạy nền** (`14-jobs-dock.md`) ở góc dưới phải; trên trang có thanh hành động ở đáy (trình nghe) dock nâng lên trên thanh đó (`bottom: 72px`) để không che nút.
2. Dock: vòng tiến độ màu Xuất (mint) · “Đang ghép Tập 1” · “Chương 3/8 · 34% · còn 46 giây” · nút huỷ. Nút chính ở chân trình nghe đổi thành “Đang ghép · 34%” (không bấm ghép thêm lần nữa). Người dùng nghe, sửa, đổi màn thoải mái; dock gắn ở app shell nên không mất.
3. Xong:
   - **Tự tải bật** (mặc định, nhớ theo thiết bị): toast mint “Đã ghép Tập 1 · đang tải về máy” có vạch tải → “Đã tải về máy này · Tập 1” · tên tệp · dung lượng · *Mở tệp* · *Xem bản xuất* (→ `exported`). Xuất lẻ: “đang tải 1 tệp .zip” hoặc “N tệp”.
   - **Tự tải tắt**: toast “Đã ghép xong Tập 1” · **Tải xuống** · *Xem bản xuất*.
   - **Trình duyệt chặn tự tải**: toast cam “Đã ghép xong · chưa tải được” + lời giải thích, **Tải xuống**; lần bấm này là thao tác của người dùng nên tải được.
   - **Lỗi**: toast đỏ “Ghép Tập 1 dừng ở chương 5” · lý do bằng lời · “Các chương đã ghép được giữ lại” · **Thử lại** (chạy tiếp từ chương lỗi) · *Ghép không nhạc* (khi lỗi do nhạc).
4. Toast tự đóng sau 6 giây (vòng đếm quanh nút ×, rê chuột thì dừng), việc vẫn nằm trong dock mục *Vừa xong* cho tới khi dock tự ẩn.
5. Tab bị ẩn: dùng thông báo của trình duyệt (như *Báo khi xong*); tự tải chỉ chạy khi tab còn mở — đóng tab thì tải sau từ *Xem bản xuất*.

Artboard `AudioExport` · `state`: merging · merged · mergedManual · mergeBlocked · mergeFailed (bấm *Ghép & xuất* ở `confirm` để xem cả luồng chạy thật).

### A4 · Phòng thu — đang tạo (07, `rendering` · `paused` · `renderError` · `background` · `rendered`)

Phòng thu là nơi xem **hàng đợi tạo giọng chạy nền** của các chương đã chốt (mở từ chip trạng thái audio, từ thẻ nổi, hoặc bước 2 trên thanh Audio). Dòng thời gian chương: xong · đang tạo · `chờ` (đã chốt, đang xếp hàng) · `chưa chốt`.

**Thẻ chính (đang tạo)**
- Avatar người đang đọc (chữ cái đầu, viền màu vai) + **2 vòng sóng toả ra** liên tục (1,8 s, lệch pha 0,9 s) + avatar “nói” (phóng 1 → 1,045, 0,42 s) + huy hiệu sóng nhỏ. Một giọng: “Giọng đọc chung · đang đọc lời Tống Nhược Vy”.
- Tiêu đề “Dẫn truyện đang đọc” (`aria-live=polite`), dòng phụ “Chương 4 · Bảy mươi ba giây · còn khoảng N phút cho các chương đã chốt”.
- **Vòng tiến độ** 96 px: viền gradient tím theo % + vầng sáng quay chậm phía sau (3,2 s/vòng); giữa là % (đếm lên) và “còn N ph”.
- **Câu đang đọc** (Literata 19 px): chữ đã đọc sáng, chữ đang đọc có ánh chạy tím, chữ chưa đọc mờ — theo tiến độ tạo đoạn.
- **Sóng âm vẽ dần** 96 vạch, cao 72 px: vạch mới “mọc” từ 15% lên đủ (0,32 s), 3 vạch mới nhất màu tím, vạch đầu bút có quầng sáng; phần chưa tạo là đường chấm mảnh.
- **4 bước của đoạn**: Chuẩn bị câu → Tạo giọng → Làm sạch âm → Ghép vào chương (xong: tick + gạch ngang · đang làm: spinner + chữ chạy + viền tím · chưa: vòng nét đứt) + “đoạn 23/51 · 4,1 giây”.
- Điều khiển: [Tạm dừng]/[Tiếp tục] · [Chạy nền] · [Huỷ, giữ phần đã xong] · “Báo khi xong”.
- Quầng sáng nền thẻ trôi chậm (9 s) — chỉ khi đang chạy.

**Dòng thời gian chương** (sửa 28/10, Gói V) <!-- tl-chuong-dai -->

- **Bề rộng** của mỗi chương theo thời lượng:
  - chương đã tạo dùng số thật;
  - chương chưa tạo dùng **thời lượng ước tính** (số chữ ÷ tốc độ đọc của giọng; chưa có số chữ thì lấy trung bình các chương đã tạo, không có nữa thì chia đều).
  - **Không bao giờ để 0**: lỗi “chương 1 chiếm gần hết, chương 2–16 bẹp” là do chương chưa tạo bị tính 0 giây.
- **Ba chế độ** theo bề rộng trung bình mỗi chương (đo thật, đổi khi co giãn cửa sổ):

  | Bề rộng mỗi chương | Chế độ | Trong ô |
  |---|---|---|
  | ≥ 72 px | Đầy đủ (cao 40, cách 4) | số chương + tick + thời lượng / % / “chờ” / “chưa chốt” |
  | 28–71 px | Chỉ số (cao 40, cách 4) | số chương ở giữa; đã tạo nền xanh nhạt; đang tạo phủ dải màu, không có kim |
  | < 28 px | Dải màu (cao 28, cách 2; > 80 chương cách 1) | không chữ. Đã tạo xanh `rgba(166,237,193,.5)` · đang tạo dải màu chạy · chờ xám 12 % · chưa chốt gạch chéo |

- **Chế độ dải màu** có thêm:
  - thước số bên dưới, chia mỗi 5 chương (≤ 60 chương), mỗi 10 (≤ 150) hoặc mỗi 25 (> 150), luôn có chương đầu và chương cuối;
  - nhãn tím ở chương đang tạo: “Ch. 37 · 45%” (bỏ vạch số chương nằm sát nó);
  - một dòng chú giải: Đã tạo · Đang tạo · Chờ tạo · Chưa chốt.
- **Rê chuột hoặc focus** một chương thì hiện thẻ nhỏ phía trên: “Chương 12 · tên chương” và “đã tạo · 07:12” / “đang tạo 45% · khoảng 18:20” / “chờ tạo · khoảng 16:53” / “chưa chốt · khoảng …”.
- **Bàn phím:** cả dải là một điểm dừng Tab (roving tabindex). ← → chuyển chương, Home/End về đầu/cuối, Enter nghe chương đã tạo.
- **Dòng tổng** ở góc phải: “đã tạo 1 giờ 37 phút / khoảng 5 giờ 00 phút · 5/16 chương”. Tổng dùng thời lượng ước tính cho chương chưa tạo; xong hết thì “đã tạo 5 giờ 02 phút · 16/16 chương”.
- Bấm chương đã xong → nghe ở mini player.

**Theo giọng**: mỗi vai một thanh “đã đọc/tổng đoạn”; vai đang đọc có thanh gradient chạy. **Nhật ký tạo**: mục mới trượt xuống từ trên (0,45 s), mờ dần theo tuổi; có mục “Đoạn 21 bị rè — đã tự đọc lại”.

**Panel phải**: Nghe trong lúc chờ (mini player) · **Máy của bạn**: tốc độ tạo (“1 phút audio ≈ 41 giây”), đang dùng (card đồ hoạ/CPU), đã chạy, dung lượng.

**Trạng thái khác**
- `paused`: mọi hoạt ảnh đứng yên tại chỗ (`animation-play-state: paused`), vòng tiến độ và viền thẻ chuyển cam, tiêu đề “Đã tạm dừng · tiếp tục từ đoạn 23”.
- `renderError`: thẻ cảnh báo “Bộ đọc trên máy đã dừng giữa chừng · Chương 4 · đoạn 23 · lúc 22:31”, lý do bằng lời thường (“Máy hết bộ nhớ đồ hoạ khi đọc đoạn này…”), phần đã xong được giữ; [Thử lại từ đoạn 23] · [Phần còn lại dùng “Nghe thử nhanh”] · [Mở Cài đặt › Giọng đọc]; Chi tiết kỹ thuật thu gọn.
- `background`: rời sang màn khác → **thẻ nổi góc phải dưới** (360 px): vòng tiến độ nhỏ + sóng, “Đang tạo audio Tập 1 · 43%”, người đang đọc, thời gian còn lại, thanh 8 chương; nút mở lại phòng thu. Hiện trên mọi màn tới khi xong; xong thì đổi thành “Audio Tập 1 đã tạo xong · Nghe ngay” + thông báo hệ thống.
- `rendered`: **khoảnh khắc hoàn thành** — vòng mint quay vào đủ (0,8 s), 2 vòng toả nổ (1,5 s), tick tự vẽ, 8 khối chương sáng lần lượt (lệch 0,08 s/khối); “Đã tạo xong audio Tập 1 · 8 chương · 2 giờ 24 phút 31 giây audio · tạo trong 1 giờ 37 phút · 3 đoạn đã tự đọc lại”; [Nghe & duyệt] · [Ghép & xuất ngay] (mở hộp thoại A3).

**Nguyên tắc chuyển động**: mọi chuyển động gắn với tiến độ thật (SSE `audio.render`), không lặp vô nghĩa; giảm chuyển động (`prefers-reduced-motion`) → tắt toàn bộ hoạt ảnh, giữ số và màu. Gradient chỉ dùng ở vùng đang hoạt động (vòng tiến độ, thanh đang chạy, vạch mới) — đúng nguyên tắc 2 của README.

### A5 · Nghe & duyệt (07, `listen`)
- Trái: chương + thời lượng + số cờ.
- Giữa: sóng âm chương (phần đã nghe sáng), cờ cam trên sóng, kim phát; điều khiển: chương trước · lùi 10s · phát/dừng · tới 10s · chương sau · tốc độ (1× → 1,25× → 1,5× → 0,75×) · cờ “Đánh dấu chỗ cần sửa (M)”. Dưới là **bản đọc chạy theo**: mốc thời gian, người nói (màu theo vai), đoạn đang đọc viền tím + sóng nhỏ, đoạn đã qua mờ. Hover đoạn: [Đọc lại đoạn này] · cờ.
- Phải: **Cần nghe lại** — mỗi cờ: lý do, vị trí, người đọc. Mở ra: chỉnh giọng/tốc độ → so sánh **Bản cũ / Bản mới** → [Dùng bản mới] [Giữ bản cũ]. Chỉ tạo lại đúng đoạn đó (vài giây) rồi ghép vào chương.
- Chân: “3 chỗ cần nghe lại · vẫn xuất được nếu bạn thấy ổn” · [Ghép & xuất Tập 1].
- Điện thoại: nút `⋯` riêng từng chương mở thao tác thật theo trạng thái. Chương đã có audio có **Nghe chương này**, **Xuất riêng chương này**, **Chia sẻ tệp audio**, **Tạo lại chương này**; chương cũ có **Cập nhật chương này**. Chương chờ có **Làm trước**, **Tạm dừng**, **Huỷ**; chương lỗi có **Thử lại**. Tạo lại/huỷ phải xác nhận; mở menu hay huỷ xác nhận không tạo việc.
- Xuất/chia sẻ từ menu chỉ chọn tệp audio và phụ đề đúng chương, không tải nhầm ZIP cả tập. Chưa có tệp thì dẫn sang xuất riêng chương đó. Các nút chạm tối thiểu 44 px; dùng tấm trượt và xác nhận chung của ứng dụng.

### A6 · Đã xuất (07, `exported`)
- “Đã xuất audio Tập 1 · 8 chương · 2 giờ 24 phút 31 giây · 6 giọng đọc · xong lúc 23:18”, player cả tập, [Mở thư mục] [Tải xuống] [Sửa & xuất lại].
- **Mốc chương** — dán vào mô tả video (`00:00 Mở đầu`, `00:12 Chương 1 · …`), [Sao chép].
- **Trong thư mục `xuat-ban › tap-01`**: file cả tập (.mp3), từng chương (8 tệp), **phụ đề `.srt`** khớp từng câu (sinh từ thời gian các đoạn), bản đọc `.txt`.

### A7 · Audio ngoài app: màn khoá, thông báo, xe, tệp tải về (28/10, Gói AF) <!-- audio-meta -->

Artboard `AudioMeta` (07c) · `scope`: chapter · volume · trial · `cover`: uploaded · generated · `compare` (so với bản hiện tại).

**Một bộ thông tin** dùng cho trình phát web (màn khoá iPhone, thông báo Android, trình duyệt máy tính, xe/Bluetooth) và cho tệp xuất (MP3, M4B, WAV), để nghe trong app hay mở tệp tải về đều giống nhau:

| Trường | Giá trị |
|---|---|
| Tên | `Chương N · {tên chương}`; bản thử thêm ` (bản thử)`; tệp cả tập: `{Tên ngắn} · Tập N`, khi phát thì đổi theo mốc chương |
| Người đọc (artist) | **Noverse Audio** — luôn như vậy, kể cả chương chưa chọn giọng hay đang tạo |
| Album | `{Tên truyện} · Tập N` |
| Nghệ sĩ album | Noverse Audio |
| Số bài / số đĩa | `N/tổng chương` · tập |
| Thể loại · năm | Sách nói · năm xuất |
| Bìa | bìa truyện (Tư liệu › Bìa truyện), vuông; chưa có thì **bìa tự tạo** (tên truyện + tập + Noverse Audio trên nền thương hiệu) — không dùng chữ viết tắt |
| Mốc chương | tệp cả tập: mỗi chương một mốc có tên |

- **Bản hiện tại sai** (ảnh chụp 28/10): màn khoá ghi “Chưa chọn giọng” ở chỗ người đọc, bìa là chữ “CG”, không có tên truyện; tệp tải về tên “Noverse.mp3”.
- **Tên tệp tải về** (máy tính và điện thoại như nhau): theo chương `{Tên ngắn} · Tập 1 · Chương 04 · {Tên chương}.mp3`; cả tập `{Tên ngắn} · Tập 1.mp3`; nhiều chương gộp `{Tên ngắn} · Tập 1 · 3 chương.zip`; bản thử thêm ` (bản thử)` trước đuôi. Số chương 2 chữ số. Tên ngắn = tên ngắn của truyện, chưa có thì cắt tên truyện ở ranh giới từ, ≤ 32 ký tự; bỏ ký tự cấm `/ \ : * ? " < > |`.
- **Nút trên màn khoá**: lùi/tới 10 giây như trong app; ⏮ ⏭ = chương trước/sau khi nghe theo chương, nhảy mốc chương khi nghe tệp cả tập (nơi nào hệ điều hành cho hiện).
- Đổi chương: cập nhật ngay tên, thời lượng, vị trí phát.

### A8 · Tạo giọng: một ngăn, chương đã sửa (29/10, Gói AK) <!-- ak-spec -->

Artboard `MVoiceGen` (M3b) · `gk`: entry · stale · ready · redo · running · error · allDone. Máy tính: `AudioExport` · `upd`: none · stale · queued (dùng kèm `state=rendered` hoặc `rendering`).

**Thay** hai sheet cũ trên điện thoại (sheet “Tạo giọng · Tập N” có Hàng đợi + nút *Thêm chương*, và sheet chọn chương có công tắc “Tạo lại chương đã có giọng”) bằng **một ngăn**. Trạng thái `MAudio` cũ `genPick` · `genPickRedo` · `queue` · `queuePaused` · `queueError` chỉ còn để tra.

**Trạng thái chương — một bộ cho máy tính và điện thoại**

| Trạng thái | Khi nào | Nhãn | Thao tác |
|---|---|---|---|
| Đã có giọng | đủ đoạn, khớp bản đọc và chương | thời lượng | *Tạo lại* (đọc lại từ đầu) |
| Cần cập nhật | `chapterChanged` hoặc `scriptChanged` | “Chương đã sửa” / “Bản đọc đã sửa” + “n đoạn đổi” | chọn sẵn |
| Chưa có giọng | chưa chốt / chưa soạn bản đọc | “Chưa chốt”, “Chưa soạn bản đọc”, “n chỗ cần xem” | chọn sẵn |
| Đang tạo / Đang cập nhật | job chạy | % + “đoạn x/y · còn khoảng …” | (tạm dừng ở đầu nhóm) |
| Chờ | trong hàng đợi | “Tiếp theo” / “Chờ · thứ n” | *Làm trước* |
| Dừng do lỗi | job lỗi | lý do bằng lời | *Thử lại* |

**Ngăn Tạo giọng · Tập N**
- Đầu ngăn: dải N ô màu (xanh có giọng, vàng cần cập nhật, tím đang tạo, xám chưa có, đỏ lỗi) + chú thích đếm. Cùng dải ở thẻ “Tạo giọng · Tập N” trên trang Nghe và ở dải chương của Phòng thu (máy tính).
- Nhóm theo thứ tự: **Cần làm** (cần cập nhật + chưa có giọng, **chọn sẵn hết**, có *Bỏ chọn hết*) → **Đang tạo** (*Tạm dừng* / *Tiếp tục*, *Báo khi xong*; hàng có *Làm trước*, *Thử lại*) → **Đã có giọng** (thu gọn, *Xem · tạo lại*; *Tạo lại* thêm chương vào lựa chọn với nhãn “Tạo lại từ đầu”).
- **Cách đọc** (cho chương sắp tạo): giọng đọc (mở sheet giọng như cũ) + **một** ô chọn chất lượng 3 mức (đây cũng là mặc định đã lưu). Chưa đo tốc độ thì mỗi ô ghi “chưa đo”, kèm một câu giải thích — không lặp “Chưa ước tính” ở từng hàng.
- Công tắc “Tự tạo giọng khi chốt chương” ở cuối.
- **Chân ngăn (luôn hiện):** câu tóm tắt + **một** nút chính.

**Một quy tắc cho nút chính** (thẻ trên trang Nghe, chân ngăn, thanh trên Phòng thu dùng cùng chữ):
- Chỉ có chương cần cập nhật → **Cập nhật n chương** (nền vàng, biểu tượng làm mới). Tóm tắt: “n cập nhật · Chuẩn · khoảng x phút”. Mô tả hàng: “chỉ đọc lại đoạn đổi” (bản đọc đã sửa) hoặc “AI soạn lại phần đổi rồi đọc lại” (chương đã sửa).
- Có chương mới hoặc tạo lại → **Tạo giọng n chương** (nền trắng, micro). Tóm tắt liệt kê “2 cập nhật · 2 mới · 1 tạo lại từ đầu”.
- Không chọn gì: còn việc đang chạy → “Không còn chương nào cần làm · n chương đang tạo”, ẩn nút; hết việc → “Cả N chương đã có giọng mới nhất” + *Nghe & xuất cả tập*.
- Không dùng chữ “Chốt & tạo” trên nút nữa. Việc chốt ghi trong tóm tắt: “Chương chưa chốt sẽ được chốt theo bản đọc hiện tại.”
- Bấm → toast “Đã xếp n chương · theo dõi ở mục Đang tạo”, các hàng chuyển sang nhóm Đang tạo.

**Máy tính (Phòng thu, dải chương):** chương cần cập nhật là ô viền vàng “đã sửa”; trên dải hiện thanh “Chương 1 và 4 đã sửa sau khi tạo giọng · Chỉ đọc lại 5 đoạn đã đổi · khoảng 3 phút” + **Cập nhật 2 chương**. Bấm → ô thành “chờ cập nhật”, thanh đổi thành “Đã xếp chương 1 và 4 · chỉ đọc lại 5 đoạn đã đổi”.

## 3. Dữ liệu & thư mục

```
dit-dang-audio/                          (dự án)
├── tai-san/giong/  giong-tri-ha.wav …   mẫu giọng (10–30 s) + giong.json (tên, giới tính, ghi chú)   ← <dirs.assets>
├── tai-san/nhac/   intro-dit-dang.mp3
├── .ainovel/cach-doc.json               từ điển cách đọc cấp dự án
└── kich_ban/01-tra-xanh-cuop-kich-ban/
    ├── audio/                           (mới — UI/host quản lý, engine viết truyện không đụng)
    │   ├── vai.json                     phân vai cả truyện: speaker → voice
    │   ├── ban-doc/tap-01/03.json       bản đọc chương 3 (xem schema)
    │   └── cache/tap-01/03/<hash>.wav   audio từng đoạn (khóa theo nội dung)
    └── xuat-ban/tap-01/
        ├── Trà Xanh Cướp Kịch Bản — Tập 1.mp3
        ├── chuong/01-vai-cua-toi-la-do-canh.mp3 …
        ├── Tập 1.srt · Tập 1.ban-doc.txt · Tập 1.moc-chuong.txt
```
Tên thư mục `audio` cấu hình được cùng các thư mục khác (GAP-13).

Bản đọc chương:
```json
{ "chapter": 3, "volume": 1, "status": "review|locked", "lockedAt": "…",
  "sourceHash": "sha256 chữ chương lúc soạn",          // khác hash hiện tại ⇒ “Sửa sau khi chốt”
  "segments": [
    { "id": "s0007", "speaker": "dan", "text": "… không quá chín mươi giây, …",
      "source": "… không quá 90 giây, …", "pauseMs": 500, "tone": "",
      "normalized": [{ "from": "90 giây", "to": "chín mươi giây", "kind": "number" }],
      "speakerGuess": [{ "who": "hc", "p": 0.62 }],     // chỉ khi chưa rõ
      "flags": ["long_sentence"] } ] }
```
`audio cache key = hash(text + voice + tone + tốc độ + chất lượng + mô hình)` — đổi chế độ giọng sẽ tạo lại toàn bộ, ước tính trong hộp thoại xác nhận cập nhật theo; → sửa một đoạn chỉ tạo lại đoạn đó; đổi giọng một vai chỉ tạo lại đoạn của vai đó.

Thời lượng ước tính trước khi tạo: số âm tiết × tốc độ đọc trung bình; thời gian tạo: đo “tốc độ tạo / thời lượng audio” trên vài đoạn đầu rồi cập nhật liên tục.

## 4. API cần thêm (GAP) — theo quy ước `11-api-contract.md`

Phạm vi: **v1.1 · Audio**, làm sau khi v1 (`12-implementation-plan.md`) xong. Đường dẫn gốc truyện `S = /api/projects/:p/stories/:s`. JSON `camelCase`, lỗi `{"error"}`, ghi cần quyền ghi truyện (người đọc chỉ nghe và tải).

| ID | Hạng mục | Endpoint |
|---|---|---|
| GAP-17 | Bản đọc | `GET S/audio/volumes/:v` → `{ chapters: [{ chapter, title, status: 'todo'\|'draft'\|'review'\|'locked'\|'stale', issues, durationSec \| null, lockedAt }] }` · `POST S/audio/volumes/:v/scripts {chapters: number[]}` (AI soạn; tiến độ qua SSE truyện, `category: "AUDIO"`) · `GET/PUT S/audio/volumes/:v/scripts/:ch` (`AudioScript`, §3) · `POST …/:ch/lock {}` · `POST …/:ch/unlock {}` · `POST …/:ch/refresh {}` (soạn lại đoạn đổi khi `stale`) |
| GAP-18 | Phân vai, giọng, cách đọc | `GET/PUT S/audio/cast` → `{ mode: 'cast'\|'single', single: { voiceId, dialogueShift: boolean, keepTones: boolean, dialoguePauseMs: number }, speakers: [{ id, name, role, voiceId \| null, sameAs?: string }] }` (chế độ `single` bỏ qua `speakers[].voiceId` khi tạo, nhưng vẫn giữ để chuyển lại) · `GET /api/projects/:p/voices` → `[{ id, name, file, durationSec, gender, builtIn }]` · `POST /api/projects/:p/voices` (multipart: tệp 10–30 s + tên) · `GET/PUT /api/projects/:p/pronunciations` → `[{ written, spoken }]` · `POST /api/tts/preview { voiceId, text, tone? }` → `audio/wav` |
| GAP-19 | Tạo giọng (hàng đợi nền) | `GET/PUT S/audio/settings { quality: 'fast'\|'standard'\|'high', autoOnLock: boolean }` · khoá chương (`…/:ch/lock`) tự xếp chương vào hàng đợi; `GET S/audio/queue` → `{ jobs: [{ chapter, state, segmentsDone, segments, etaSec }] }` · `POST S/audio/volumes/:v/chapters/:ch/render {}` (khi tắt tự tạo) · `POST S/audio/segments/:id/preview {}` → tạo & **lưu** đoạn, trả `url` (dùng cho nút nghe ở 06) · SSE `audio.render { jobId, chapter, segment, segments, speaker, percent, etaSec, state }` · `POST S/audio/renders/:job/pause\|resume\|cancel {}` · `GET S/audio/volumes/:v/chapters/:ch.mp3` (hỗ trợ `Range`) · `GET …/:ch/peaks` (đỉnh sóng) · `POST S/audio/segments/:id/rerender { voiceId?, speed?, tone? }` → `{ candidateId, url }` · `POST S/audio/segments/:id/accept { candidateId }` · `GET/POST S/audio/volumes/:v/flags` (cờ cần nghe lại) |
| GAP-20 | Ghép & xuất | `POST S/audio/volumes/:v/exports { format: 'm4b'\|'mp3'\|'wav', chapters?: number[], onExists?: 'replace'\|'keep'\|'fail', chapterMarks: boolean, subtitles: boolean, whenReady: boolean, intro: boolean, chapterTitles: boolean, loudness: boolean, gapSec: number }` → `{ files: ExportFile[], chapterMarks: [{ atSec, label }], durationSec: number, pending?: true }`; mặc định các cờ là `false`, `onExists` là `replace`. Bỏ `chapters` hoặc gửi `null`: cả tập; danh sách tường minh: chỉ các chương đó, bỏ trùng và sắp tăng dần. `[]`, số chương không thuộc tập hoặc chương được chọn chưa có đủ giọng → 409 (lỗi nêu số chương còn thiếu). `whenReady:true`: lưu “Xuất khi xong”, trả `{files:[],chapterMarks:[],durationSec:0,pending:true}`; chỉ đợi các chương được chọn, không đợi chương ngoài danh sách. |
| GAP-21 | Bộ đọc trên máy | Cấu hình `tts: { engine, model, device: 'cpu'\|'gpu', sampleRate }` trong `config.json` (kế thừa 3 cấp như GAP-10) · `GET /api/tts/status` → `{ ready, engine, model, device, rtf \| null }` · mục **Cài đặt › Giọng đọc**. Chạy dạng tiến trình phụ nói chuyện qua HTTP nội bộ; ghép, chuẩn hoá âm lượng, MP3 bằng `ffmpeg` (`loudnorm`, khoảng −16 LUFS cho đăng mạng) |

Thư mục làm việc audio của truyện thêm vào `LibraryConfig.dirs` thành `dirs.audio` (mặc định `audio`); mẫu giọng/nhạc nằm trong `<dirs.assets>/giong/`, `<dirs.assets>/nhac/` của dự án; thành phẩm trong `<dirs.exports>/tap-NN/`.

Trạng thái trống và lỗi: `GET …/scripts/:ch` trả 404 khi chưa có bản đọc; giao diện đổi thành **Chưa soạn** và nút **Soạn bản đọc chương N**, không hiện lỗi đỏ. Soạn/soạn lại cần LLM: thiếu hoặc sai cấu hình trả 409 `{error,needsConfig:true}` theo 11 §5. Lỗi kết nối nhà cung cấp phát qua `audio.compose` (khung `{type,category,data}`), hiện ngay trong Bản đọc; quản trị viên có liên kết **Cài đặt › Nhà cung cấp**. Mỗi lượt soạn giới hạn hai phút để nhà cung cấp không phản hồi không giữ trạng thái đang soạn vô hạn. Phân vai GET/PUT chỉ dùng hồ sơ nhân vật và dữ liệu đã lưu, không cần LLM.

Danh sách giọng của màn Audio chỉ lấy từ `GET /api/projects/:p/voices` (máy chủ đã gộp giọng dự án với giọng có sẵn, phân biệt bằng `builtIn`; bộ đọc chưa cài thì chỉ có giọng dự án), nên mỗi giọng hiện một lần và không có lỗi 503 từ `/api/tts/voices` lặp thành cảnh báo “Không tải lại được”. Ở bản máy chủ (hosted) không có bộ đọc cục bộ, “giọng có sẵn” là danh mục thật do máy đọc công bố trong `state.voices` của worker; chưa có danh mục thì chỉ hiện giọng dự án, không bịa giọng. Cài xong bộ đọc thì danh sách tự tải lại. Khi bộ đọc chưa cài chỉ thẻ hướng dẫn giải thích trạng thái này; điện thoại chưa có chương nghe được chỉ hiện thẻ, chương đã có giọng vẫn nghe được. Các lỗi tải tập, phân vai, thiết lập, danh sách giọng vẫn hiện bình thường.

Cờ `no_voice` theo phân vai hiện tại: số “cần xem” của tập và bản đọc tính lại theo phân vai mỗi lần đọc, lượt tạo giọng ghi lại cờ đúng vào tệp bản đọc. Tiến độ hiển thị luôn làm tròn số nguyên (máy chủ có thể gửi % lẻ).

**Ghi chú triển khai:** M4B dùng AAC trong MP4; `chapterMarks:true` nhúng chương bằng ffmetadata (mốc giây có phần lẻ), khác với `chapterTitles` là đọc tên chương thành tiếng. `subtitles:true` ghi `.srt`; tắt cờ sẽ xoá `.srt` cũ, bản đọc `.ban-doc.txt` không thay đổi. `loudness:true` dùng hai lượt `loudnorm`, đo trước rồi áp dụng thông số đo, đích −16 LUFS ±1. Phụ đề/bản đọc không chứa chỉ dẫn cảm xúc: chỉ engine có `Capabilities.EmotionCues` nhận `[cười]`, `[thở dài]`, `[hắng giọng]` trong chữ tổng hợp; chữ này tham gia cache key.

**Xuất một phần (18-mobile §M3):** “Xuất 9 chương” gửi `chapters:[1,2,3,4,5,6,7,8,9]`; mốc chương, phụ đề và bản đọc chỉ chứa các chương đã chọn. Tệp cả phần có hậu tố `-ch1-9`, tập chương rời dùng `_` giữa các khoảng (ví dụ `-ch1-3_5`); phụ đề/bản đọc và thư mục từng chương dùng cùng hậu tố nên không đè thành phẩm cả tập. `onExists:"keep"` giữ tệp cũ, thêm `-YYYYMMDD-HHmmss` trước đuôi theo quy ước xuất hiện có; trùng cùng giây trả 409, `fail` trả 409 nếu tên đã có. POST và `GET …/exports` trả danh sách tệp của lần xuất mới nhất; các lần trước vẫn tải được qua thư mục xuất chung.

Khi gateway bật, quét truyện có `audio/queue.json` ở trạng thái `queued|rendering` để tiếp tục, không chờ mở màn hình. Yêu cầu xuất lưu trong `audio/export-requests.json` và tiếp tục sau khi khởi động lại. Hàng đợi tạo giọng và xuất tự động xuất hiện trong jobs dock (`p/s/audio/v`, `p/s/audio/v/export`); xuất tự động phát `audio.export {volume,state,percent,durationSec,error?}` và `job.waiting|job.progress|job.done|job.failed`. Thành phẩm tiếp tục xuất hiện trong `ExportRow.audio`.

Jobs dock điều khiển xuất qua `POST /api/jobs/:p/:s/audio/:v/export/retry|cancel`: thử lại yêu cầu bị lỗi giữ nguyên lựa chọn chương và tuỳ chọn; huỷ yêu cầu đang chờ/đang xuất/bị lỗi được lưu xuống đĩa, huỷ tiến trình xuất đang chạy nhưng không đổi hàng đợi tạo giọng.

Web dùng chung `web/src/data`: dữ liệu tập, bản đọc, phân vai, thiết lập, hàng đợi, cờ nghe lại và thành phẩm được giữ trong cache theo khoá audio; `story.changed` phần `audio` làm mới các panel đang mở mà không xoá nội dung cũ. Bản đọc/phân vai/thiết lập gửi `If-Match` từ `version`; từ điển cách đọc giữ JSON mảng và lấy phiên bản qua `ETag`. Xung đột hiện thanh **Xem bản mới / Ghi đè**; các công tắc chuẩn hoá lưu trong thiết lập audio của truyện, không lưu lẫn với mục từ dự án. Yêu cầu `pending:true` chỉ hiện **Đã hẹn xuất**, không chuyển sang thẻ **Đã xuất** cho tới khi có thành phẩm; thẻ thành phẩm dùng `durationSec` do server trả. Tiến độ, thông báo và điều khiển nền dùng jobs dock chung, không có thẻ audio nổi riêng.

Lát thi công đề xuất bổ sung vào `12-implementation-plan.md`: **B6 · Audio** (`internal/audio/*`, `internal/entry/web/audio.go`, sidecar TTS, ffmpeg) và **F6 · Audio** (`web/src/screens/audio/*`), phụ thuộc B1 (thư viện), B2 (xuất), F0.

Ứng viên mô hình TTS tiếng Việt chạy trên máy (cần tự đánh giá chất lượng, giấy phép và khả năng clone giọng trước khi chọn): VieNeu-TTS, VietTTS, Valtec TTS, ZipVoice tiếng Việt. UI không phụ thuộc mô hình — chỉ dùng: danh sách giọng, nghe thử, tạo đoạn, tốc độ tạo.

## 5. Component mới

`AudioPipeline` (3 bước, đếm chốt) · `ScriptChapterList` · `ScriptSegment` (chip người nói / sắc thái / ngừng, chữ chuẩn hoá, gợi ý người nói, gợi ý tách câu) · `SegmentPreviewButton` (idle → đang tạo → đang phát) · `CastPanel` / `VoicePicker` / `VoiceUpload` · `IssueList` · `PronunciationTable` · `RenderConfirmDialog` · `QualityPicker` · `RenderProgress` · `ChapterRenderRow` · `MiniPlayer` · `Waveform` (đỉnh tính sẵn phía server, 1 giá trị/≈ 0,5 s) · `FollowAlongScript` · `FlagList` + `RerenderAB` · `ExportResult` (mốc chương, danh sách tệp).

Phím tắt trình nghe: `Space` phát/dừng · `←/→` 10 giây · `Shift ←/→` đoạn trước/sau · `M` đánh dấu · `[`/`]` tốc độ. Trong Bản đọc: `J`/`K` chỗ cần xem kế/trước · `P` nghe đoạn đang chọn · `Enter` sửa chữ.

## 6. Chữ & nguyên tắc
- Theo `07-copy.md` › Giọng văn: không nói “TTS”, “render”, “segment”, “LUFS” trên giao diện — dùng “bản đọc”, “tạo audio”, “đoạn”, “cân bằng âm lượng”.
- Mọi số thời gian có dấu “≈” khi là ước tính; thời gian thật sau khi tạo không có “≈”.
- Không cho tạo audio khi chưa chốt đủ chương; cho phép **xuất** khi còn cờ chưa xử lý (người dùng quyết).
- Agent orb chỉ xuất hiện ở thẻ tiến độ (đang tạo). Người nói dùng chữ cái đầu + viền màu, không dùng gradient.

Nguồn tham khảo mô hình: [VieNeu-TTS (PyPI)](https://pypi.org/project/vieneu/) · [VietTTS (Hugging Face)](https://huggingface.co/tinhthe/viettts) · [Valtec TTS (GitHub)](https://github.com/tronghieuit/valtec-tts) · [ViZipVoice (Hugging Face)](https://huggingface.co/anphunl/ViZipvoice)

## 7. Audio theo gói (bổ sung 25/10 · D11) <!-- brief-2510 -->

- Gói có `voice: single` (gói A — Quyết định 3) ⇒ tweak `voice=pack` ở `Transcript`, `AudioExport`, `MAudio`.
- **Transcript › Phân vai:** thay hai thẻ “Phân vai | Một giọng” bằng thẻ khoá **“Một giọng kể · theo gói”** + mô tả + “Đổi gói trong Tuỳ chỉnh ›”; danh sách giọng chọn sẵn “Nữ trẻ, ấm · gói đề xuất”, vẫn đổi được giọng cụ thể.
- **AudioExport:** thẻ Giọng đọc “Một giọng kể” + badge “theo gói”, liên kết “Gói chọn sẵn · đổi gói trong Cách viết”; làn giọng “Giọng kể của gói”.
- **MAudio:** “~2 giờ 41 phút · một giọng kể” + badge “theo gói”.
- Nguyên tắc 7: không hiện tên tệp mẫu giọng (`giong-tri-ha.wav` → “Mẫu giọng Tri Hạ · Bạn tải lên · 18 giây · nữ”). Nhạc nền không làm trong công cụ (Quyết định 10).

## 8. Máy đọc — bản chạy trên máy chủ (Gói AT, 01/11) <!-- at-spec -->

Nguồn: `plans/reports/noverse-tts-autosetup.md` (§4.3 enum trạng thái, §5b yêu cầu). Bộ đọc không chạy trong máy chủ mà trên **máy đọc** (một máy luôn bật, hiện là Mac mini). Máy đọc tự **chuẩn bị bộ đọc** khi chưa có. Bản cài trên máy tính cá nhân dùng **cùng khung**, chỉ khác nguồn số liệu (“Máy này”).

**Từ dùng trên giao diện:** máy đọc · bộ đọc · chuẩn bị bộ đọc · máy đọc chưa kết nối · đang chờ máy đọc. **Không** hiện: worker, container, launchd, uv, python, ffmpeg, đường dẫn, tên tệp, mã lỗi, nhật ký — trừ **Tuỳ chỉnh › Nâng cao** (quản trị).

### 8.1 Năm trạng thái (`Settings` › Giọng đọc, tweak `voice`)

| Trạng thái | Nhãn trạng thái | Mức độ | Nội dung | Hành động |
|---|---|---|---|---|
| `noMachine` | Chưa có máy đọc (bản cá nhân: Chưa chuẩn bị) | — | Máy đọc là gì + 3 bước | Bản máy chủ: *Cách bật máy đọc* → Nâng cao. Bản cá nhân: **Chuẩn bị bộ đọc** |
| `offline` | Chưa kết nối (xám) | **Cảnh báo** chỉ khi có chương đang chờ: “n chương đang chờ” | “Lần cuối kết nối {giờ}. Có thể máy đang ngủ hoặc tắt — không phải lỗi…” | không nút; tự làm mới |
| `provisioning` | Đang chuẩn bị (xanh dương) | — | “Bước i/6 · {bước}” + danh sách 6 bước; bước đo được có thanh + % (Cài thư viện, Tải mẫu giọng: “244 / 581 MB · còn khoảng 4 phút”); bước không đo được: spinner + một dòng | không nút |
| `provisionFailed` | Chuẩn bị chưa xong (đỏ) | **Bắt sửa** | tiêu đề = lý do ngắn; một câu lý do (bảng 8.2) | **Thử lại** (bị thu hồi quyền: *Mở Nâng cao*) |
| `ready` | Sẵn sàng (xanh lá) | — | 4 ô: Máy đọc · Mẫu giọng · Tốc độ tạo (“1 phút audio ≈ 25 giây”) · Dung lượng đang dùng | — |

Sáu bước (`step`): Tải công cụ cài đặt · Chuẩn bị môi trường chạy · Tải bộ đọc · **Cài thư viện (%)** · **Tải mẫu giọng (% + MB)** · Khởi động bộ đọc. % chỉ hiện ở hai bước đo được; không có % giả. Tweak `vstep`: `source` (spinner) · `model` (42 %).

Dưới thẻ: **Chất lượng giọng** (Nghe thử nhanh · Chuẩn · Cao nhất) ghi “Mặc định cho truyện mới; từng truyện chỉnh ở Phòng thu”. Bản máy chủ **ẩn** Thiết bị và Bộ đọc; bản cá nhân giữ “Chạy bằng” (Tự chọn · Bộ xử lý · Card đồ hoạ). **Giọng có sẵn** chỉ hiện khi sẵn sàng; nút Nghe thử theo tweak `vprev` (8.4).

### 8.2 Câu lý do lỗi (tweak `vfail`)

| Mã (chỉ ở Nâng cao) | Tiêu đề | Câu | Hành động |
|---|---|---|---|
| `no_disk` | Máy đọc không đủ chỗ trống | Cần ít nhất 4 GB trống để chuẩn bị bộ đọc, máy đọc chỉ còn {x} GB. Dọn bớt tệp trên máy đọc rồi thử lại. | Thử lại |
| `download_failed` (cả `uv_failed`) | Không tải được bộ đọc | Mạng của máy đọc bị ngắt giữa chừng. Máy đọc tự thử lại sau {n} phút; bấm Thử lại để thử ngay. | Thử lại |
| `hash_mismatch` | Tệp tải về không khớp bản gốc | Tệp vừa tải khác bản Noverse đã kiểm nên đã bị bỏ để giữ an toàn. Máy đọc không tự thử tiếp; thử lại khi mạng ổn định, vẫn lỗi thì báo quản trị. | Thử lại |
| `interrupted` | Lượt chuẩn bị bị gián đoạn | Máy đọc dừng giữa chừng khi chuẩn bị bộ đọc nên chưa rõ kết quả. Phần đã xong được giữ nguyên; bấm Thử lại để làm tiếp. Không tự thử khi chưa xác định được kết quả lượt trước. | Thử lại |
| `deps_failed` | Không chuẩn bị được thư viện | Bước cài thư viện cho bộ đọc dừng giữa chừng, thường do mạng chập chờn. Phần đã tải được giữ lại nên lần sau nhanh hơn. | Thử lại |
| `model_failed` | Không tải được mẫu giọng | Mẫu giọng (khoảng 580 MB) mới tải được một phần. Phần đã tải được giữ, bấm Thử lại để tải tiếp. | Thử lại |
| `start_failed` | Bộ đọc không khởi động được | Bộ đọc đã chuẩn bị xong nhưng không bật lên được, thường do máy đọc thiếu bộ nhớ. Đóng bớt ứng dụng trên máy đọc rồi thử lại. | Thử lại |
| `token_rejected` | Máy đọc đã bị thu hồi quyền | Máy đọc này không còn được nhận việc tạo giọng. Quản trị cấp lại quyền ở Nâng cao. | Mở Nâng cao |

### 8.3 Thẻ đầu màn Nghe / Phòng thu / Bản đọc (tweak `machine`)

Cùng một thẻ cho `Transcript` (Bản đọc), `AudioExport` (Phòng thu, Nghe & xuất), `MAudio` (điện thoại). Biến thể: `noMachine` · `offline` · `provisioning` (thanh % + “vẫn soạn bản đọc được”) · `provisionFailed` (lý do + Thử lại) · `missingFfmpeg` (chỉ máy tính, Cảnh báo: “Chưa ghép và xuất tập được” — chỉ khoá nút **Ghép & xuất**, có ổ khoá + thẻ trong hộp xuất; tạo giọng và nghe vẫn chạy). Bản máy chủ **không** có nút “Cài bộ đọc trên máy”.

Khi máy đọc chưa sẵn sàng mà có chương xếp hàng, Phòng thu thay khung “đang đọc %” bằng khung **“n chương đang chờ máy đọc”**: biểu tượng đồng hồ, danh sách thứ tự với nhãn trung tính *Đang chờ máy đọc*, nút *Báo khi bắt đầu*, câu “Không phải lỗi…”. Dải chương không còn ô “đang tạo”. Cột phải “Máy của bạn” đổi thành **Máy đọc** (dòng đầu: tên máy đọc hoặc trạng thái).

### 8.4 Trả lời câu hỏi của dev

1. **Máy đọc offline:** trạng thái **trung tính** (chấm xám, không đỏ). Chỉ khi có chương đang chờ mới gắn **Cảnh báo** “n chương đang chờ”, vì lúc đó người dùng đang mất việc.
2. **Kiểm tra lại:** **không** có nút; tự làm mới (luồng sự kiện + khi quay lại tab). Điện thoại bỏ nút cũ, dùng kéo để làm mới sẵn có. Chỉ trạng thái lỗi có **Thử lại**.
3. **Đang chuẩn bị lần đầu:** **cho xếp hàng**, hiện “Đang chờ máy đọc” (không khoá nút Tạo giọng).
4. **Thu hồi quyền:** xác nhận **một bước** (hộp nêu hậu quả, nút đỏ “Thu hồi quyền”). Không mất dữ liệu, cấp lại được — gõ tên chỉ dành cho thao tác mất dữ liệu.
5. **Chất lượng / Thiết bị / Bộ đọc ở bản máy chủ:** ẩn **Thiết bị** và **Bộ đọc**; giữ **Chất lượng** với nhãn “Mặc định cho truyện mới; từng truyện chỉnh ở Phòng thu”.
6. **Nghe thử giọng ở bản máy chủ:** **ẩn** cho tới khi máy đọc báo hỗ trợ (`capabilities.preview`); khi có thì hiện, bấm là “Đang nhờ máy đọc…” (tweak `vprev`: hidden · supported · waiting). Bản cá nhân giữ như cũ.

### 8.5 Nâng cao (quản trị) — chỗ duy nhất có chi tiết kỹ thuật

Khối **Máy đọc · chi tiết kỹ thuật** đầu trang Nâng cao: Tên máy đọc · Lần cuối kết nối · Phiên bản bộ đọc · Dung lượng (+ thư mục) · Kênh nối · Quyền · Trạng thái gửi về (giá trị kỹ thuật mono, chú thích tiếng Việt chữ thường) + **6 dòng nhật ký gần nhất** + *Chép nhật ký* + **Thu hồi quyền máy đọc** (viền đỏ). Chưa có máy đọc: hướng dẫn và lệnh bật một lần. Tweak `vadv`: none · confirm · revoked.

### 8.6 Dock & điện thoại

- **Dock** (`Jobs`, tweak `scenario`: prep · prepStart · prepDone · prepErr): hàng **Chuẩn bị bộ đọc** — bước không đo được thì vòng xoay + “Bước 3/6 · Tải bộ đọc…”; bước đo được có % và “Bước 5/6 · Tải mẫu giọng · 299/581 MB”; xong: “Bộ đọc đã sẵn sàng” + *Mở Giọng đọc*; lỗi: lý do + **Thử lại** + **Huỷ**. Khác `audioErr` (lỗi tạo giọng một chương).
- **Điện thoại:** `MAudio` tweak `machine` (noMachine · offline · provisioning · provisionFailed) — thẻ đầu tab Nghe; thẻ tổng “Chương 10 đang chờ máy đọc” (không vòng sóng). Bỏ câu “Cài bộ đọc trên máy tính để tạo giọng”. `MVoiceGen` tweak `gk`: `waiting` · `waitPrep` — nhóm **Đang chờ máy đọc · n** (trung tính, đồng hồ, “không phải lỗi · giữ nguyên thứ tự”), ghi chú máy đọc, nút *Báo khi bắt đầu*; khác hẳn `error` (“Dừng do lỗi”).
